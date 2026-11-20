package guard

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Windows programs that start another command (RT126SWEEP3 F4): wt and conhost run the words after their options,
// schtasks /tr, wmic's call create, forfiles /c and runas a command line, explorer and rundll32 whatever Windows starts
// for the file they open, and Invoke-CimMethod's (or Invoke-WmiMethod's) Win32_Process Create a command line too. Each
// is read in every reading.

// launchMark starts the command that stands for a program started where the guard can't read what runs: a script
// explorer or rundll32 opens, a process WMI creates. The command's own words follow it.
const launchMark = "\x00launch"

// launchRule is the rule a launchMark's command is refused by.
const launchRule = "a program started where the guard can't read what it runs"

// launcherMost is the most words a launcher's own words are read from: past it, the command is refused as longer than
// the guard reads (each plain word may start the command, so each is read).
const launcherMost = 64

// windowsLaunchers are the programs windowsLaunched reads.
var windowsLaunchers = map[string]bool{"wt": true, "conhost": true, "schtasks": true, "wmic": true, "forfiles": true,
	"runas": true, "explorer": true, "rundll32": true, "invoke-cimmethod": true, "icim": true, "invoke-wmimethod": true,
	"iwmi": true}

// windowsLaunched is what a Windows launcher (windowsLaunchers) runs.
func windowsLaunched(name string, args []string) []inner {
	if len(args) > launcherMost {
		return []inner{{words: simple{"\x00long", name}}}
	}
	folded := make([]string, len(args))
	for i, a := range args {
		folded[i] = strings.ToLower(withoutSubs(a))
	}
	// lineAfter is the word after each of switches as a line cmd reads (CreateProcess's command line).
	lineAfter := func(switches ...string) []inner {
		var out []inner
		for i, a := range folded[:max(len(folded)-1, 0)] {
			for _, s := range switches {
				if a == "/"+s || a == "-"+s {
					out = append(out, inner{line: args[i+1], ps: true})
				}
			}
		}
		return out
	}
	switch name {
	case "wt", "conhost":
		// Windows Terminal (wt [-w 0] [new-tab|nt|split-pane|sp …] [options] command, its tabs split at a ;) and the
		// console host (conhost [--headless] command): each plain word may be an option's value or the command's start,
		// so each is read as the start.
		var out []inner
		for i, a := range args {
			if !strings.HasPrefix(a, "-") {
				out = append(out, inner{words: args[i:]})
			}
		}
		return out
	case "schtasks":
		// schtasks /create … /tr "command" (or /change): the task's command line.
		return lineAfter("tr")
	case "forfiles":
		// forfiles /c "command": cmd's line, with 0xHH written as its character, and forfiles' @file, @path and the
		// rest as variables cmd fills in (%file%): one used as the command is a word the guard can't read.
		var out []inner
		for _, in := range lineAfter("c") {
			out = append(out, in)
			if line := forfilesLine(in.line); line != in.line {
				out = append(out, inner{line: line, ps: true})
			}
		}
		return out
	case "wmic":
		// wmic [/node:x] process call create "command"[,"folder"]: the command line, whole and before its comma.
		for i := 0; i+2 < len(folded); i++ {
			if folded[i] == "call" && folded[i+1] == "create" {
				line := strings.Join(args[i+2:], " ")
				out := []inner{{line: line, ps: true}}
				if first, _, ok := strings.Cut(line, ","); ok {
					out = append(out, inner{line: first, ps: true})
				}
				return out
			}
		}
	case "runas":
		// runas [/noprofile] [/env] /user:name "command": its words that aren't switches are the command line. With /?
		// it shows its help and runs nothing (RT126SWEEP3FIXP2 O5).
		if slices.ContainsFunc(folded, func(a string) bool { return strings.TrimPrefix(a, unreadable) == "/?" }) {
			return nil
		}
		var out []inner
		for i, a := range args {
			if !strings.HasPrefix(a, "/") {
				out = append(out, inner{line: args[i], ps: true})
			}
		}
		return out
	case "explorer", "rundll32":
		// explorer file, rundll32 url.dll,FileProtocolHandler file (or shell32.dll,ShellExec_RunDLL file …): Windows
		// starts what the file is: a program, read by its name, or a script, whose lines the guard can't read.
		var out []inner
		for i, a := range args {
			if strings.HasPrefix(a, "/") || name == "rundll32" && i == 0 {
				continue
			}
			if opensScript(a) {
				return []inner{{words: append(simple{launchMark, name}, args...)}}
			}
			out = append(out, inner{words: args[i:]})
		}
		return out
	case "invoke-cimmethod", "icim", "invoke-wmimethod", "iwmi":
		// Win32_Process's Create starts a command line (or a class read from a variable, whose Create does too), by
		// Invoke-CimMethod and by the older Invoke-WmiMethod alike (RT126SWEEP3FIXP2 O6).
		piped := len(args) > 0 && args[len(args)-1] == pipedMark
		if piped {
			args, folded = args[:len(args)-1], folded[:len(folded)-1]
		}
		for _, a := range folded {
			if strings.Contains(a, "win32_process") || a == "create" || strings.HasSuffix(a, ":create") {
				return []inner{{words: append(simple{launchMark, name}, args...)}}
			}
		}
		// A class or method the guard can't read (a variable, a $( ), a call): it may be Win32_Process's Create.
		// So may one a splat gives (icim @p), or one not written out at all (icim ( … ) -Arguments $a ( … ): the
		// expressions are read as commands of their own), unless a pipeline feeds the call its object
		// (RT126SWEEP3FIXP2R1 N4).
		if wmiNameUnread(args, folded) || slices.ContainsFunc(args, isSplat) || !piped && wmiClassUnwritten(args, folded) {
			return []inner{{words: append(simple{unreadable + name}, args...)}}
		}
	}
	return nil
}

// wmiMethods are the cmdlets that call a WMI class's method, and their aliases.
var wmiMethods = map[string]bool{"invoke-cimmethod": true, "icim": true, "invoke-wmimethod": true, "iwmi": true}

// isSplat is whether a word is PowerShell's splat, @ then a variable's name (@p, @args): its parameters, named or not,
// are a hashtable's or an array's, which the guard can't read. @{ … } and @( … ) are values, not splats.
func isSplat(word string) bool {
	w := strings.TrimPrefix(word, unreadable)
	return len(w) > 1 && w[0] == '@' && ('a' <= w[1]|0x20 && w[1]|0x20 <= 'z')
}

// wmiValues are Invoke-CimMethod's and Invoke-WmiMethod's parameters whose values name no class and no method; every
// other parameter's value, and a word given by position, may name one (wmiNameUnread). A parameter is written as any
// start of its name (-Arg), as PowerShell takes it.
var (
	wmiValues = []string{"arguments", "argumentlist", "computername", "cimsession", "namespace", "operationtimeoutsec",
		"querydialect", "credential", "impersonation", "authentication", "locale", "authority", "throttlelimit",
		"resourceuri"}
	wmiNames = []string{"classname", "class", "cimclass", "methodname", "name", "inputobject", "query", "path"}
	// wmiClasses are the naming parameters that give the class, or the object whose class it is (wmiClassUnwritten).
	wmiClasses = []string{"classname", "class", "cimclass", "inputobject", "query", "path"}
)

// wmiClassUnwritten is whether an Invoke-CimMethod's or Invoke-WmiMethod's class is written nowhere in its words: no
// parameter that gives it (wmiClasses, or any start of one, as PowerShell takes it), and no first word by position, or
// that word not written out ($k, a $( )). Its class then comes from what the guard doesn't read.
func wmiClassUnwritten(args, folded []string) bool {
	for i := 0; i < len(args); i++ {
		if a := folded[i]; strings.HasPrefix(a, "-") && len(a) > 1 {
			p, _, glued := strings.Cut(a[1:], ":")
			if slices.ContainsFunc(wmiClasses, func(n string) bool { return strings.HasPrefix(n, p) }) {
				return false // its value is wmiNameUnread's
			}
			if !glued {
				i++ // another parameter's value, or the word after a switch: no class by position here
			}
			continue
		}
		a := args[i]
		return strings.HasPrefix(a, unreadable) || strings.HasPrefix(a, "\x00") || strings.ContainsAny(a, "$"+string(subMark))
	}
	return true
}

// wmiNameUnread is whether an Invoke-CimMethod's or Invoke-WmiMethod's class or method is named where the guard can't
// read it: a value that is a variable or a $( ) (-MethodName $m, -Class $k), or a parameter with no word after it,
// whose value an expression gives (-MethodName (Get-X), read as a command of its own). A parameter's start that
// could be a naming one too (-n: -Name or -Namespace) counts as naming.
func wmiNameUnread(args, folded []string) bool {
	starts := func(names []string, p string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return strings.HasPrefix(n, p) })
	}
	for i := 0; i < len(args); i++ {
		value := args[i]
		if a := folded[i]; strings.HasPrefix(a, "-") && len(a) > 1 {
			p, _, ok := strings.Cut(a[1:], ":")
			naming := starts(wmiNames, p) || !starts(wmiValues, p)
			switch {
			case ok && naming:
				_, value, _ = strings.Cut(args[i], ":") // -MethodName:$m
			case !ok && naming && starts(wmiNames, p) && i == len(args)-1:
				return true
			case !ok && !naming:
				i++ // the value of a parameter that names nothing
				continue
			default:
				continue // a switch (-AsJob), or a naming parameter whose value is the next word
			}
		}
		if strings.HasPrefix(value, unreadable) || strings.ContainsAny(value, "$"+string(subMark)) {
			return true
		}
	}
	return false
}

// scriptKinds are the files Windows runs a script or a shortcut for when one is opened.
var scriptKinds = map[string]bool{".bat": true, ".cmd": true, ".ps1": true, ".psm1": true, ".vbs": true, ".vbe": true,
	".js": true, ".jse": true, ".wsf": true, ".wsh": true, ".hta": true, ".sh": true, ".lnk": true, ".url": true,
	".scr": true, ".pif": true, ".com": true, ".msi": true, ".appref-ms": true}

// opensScript is whether a file explorer or rundll32 is given is a script or a shortcut (scriptKinds).
func opensScript(word string) bool {
	w := strings.ToLower(strings.Trim(withoutSubs(strings.TrimPrefix(word, unreadable)), `"' `))
	w = strings.TrimRight(strings.ReplaceAll(w, `\`, "/"), "/.")
	if scheme, _, ok := strings.Cut(w, "://"); ok && scheme != "file" {
		return false // a web page, opened in the browser (https://example.com)
	}
	return scriptKinds[path.Ext(w)]
}

var (
	forfilesHex = regexp.MustCompile(`0[xX][0-9a-fA-F]{2}`)
	forfilesVar = regexp.MustCompile(`(?i)@(relpath|fname|file|path|ext|isdir|fsize|fdate|ftime)`)
)

// forfilesLine is a forfiles /c line as cmd is given it: 0xHH as its character, and each @variable as cmd's %variable%.
func forfilesLine(line string) string {
	line = forfilesHex.ReplaceAllStringFunc(line, func(hex string) string {
		n, _ := strconv.ParseUint(hex[2:], 16, 8)
		return string(rune(n))
	})
	return forfilesVar.ReplaceAllString(line, "%$1%")
}

// cmdVariable matches a word that is cmd's %name% or !name! (delayed expansion) in its last part, or a for loop's
// variable, which cmd fills in each time round (RT126SWEEP3FIXP2 N4): % or %%, any ~ and modifier letters, with a
// $NAME: (%~$PATH:i, the first match on PATH), then one character but %, a blank, a quote or a {. cmd takes nearly
// any character as the variable (%i, %%#, %@, %1, RT126SWEEP3FIXP2R1 N3), so only a last part that starts with it is
// one: a % inside a name (100%done.sh, tool%v1) isn't (O3). %{ is PowerShell's ForEach-Object and its script block
// (1|%{echo $_}); in a cmd line, read as PowerShell's, a { ends the word there anyway.
var cmdVariable = regexp.MustCompile(`%[^%\s]+%|![^!\s]+!|^%%?(?:~[A-Za-z]*(?:\$[^%\s"':]*:)?)?[^%\s"'{]`)

// spelledAtRunTime is whether a command word's last part (the program's name) is filled in when it runs: a $name,
// ${…}, $env:name or a $( ) (PowerShell's & and ., the shells' dot), or cmd's %name% or !name!. The guard can't read
// such a name (RT126SWEEP3 F4): one written out after the variable's folders ($env:ProgramFiles\Git\bin\git.exe) it
// reads as any other.
func spelledAtRunTime(word string) bool {
	last, ok := programPart(word)
	return ok && (strings.ContainsAny(last, "$"+string(subMark)) || cmdVariable.MatchString(last))
}

// namedByCmdVariable is whether a command word's last part is cmd's %name% or !name!, which cmd fills in.
func namedByCmdVariable(word string) bool {
	last, ok := programPart(word)
	return ok && cmdVariable.MatchString(last)
}

// programPart is a command word's last part, the program's name; false when the word is a whole line read as one
// quoted word (cmdLines' quoted reading), which names no program: its last part holds a blank (cmd /c "set c=1&& echo
// %c%"), or the word is a whole for loop, whose last part is only an argument of its command (cmd /c "for %f in
// (*.md) do copy %f backup\%f", RT126SWEEP3FIXP2R2 O2). cmdLines' plain reading reads that loop's own command. A
// folder's name with a blank in it doesn't make a word a line: "C:\Program Files\$c" still has its last part read.
func programPart(word string) (string, bool) {
	w := strings.TrimPrefix(word, unreadable)
	last := w[strings.LastIndexAny(w, `/\`)+1:]
	return last, last != "" && !strings.ContainsAny(last, " \t") && !cmdForLoop.MatchString(w)
}

// cmdForLoop is a word that is a whole cmd for loop, in any case: for, its switches and variable, in (…), then do and
// its command (cmd /c "for /r %f in (*.md) do copy %f backup\%~nxf").
var cmdForLoop = regexp.MustCompile(`(?is)^for\s.*\sin\s*\(.*\)\s*do[\s(]`)

// calledAtRunTime is a wrapper's words with the program unreadable when its name is spelledAtRunTime: refused as a
// command name the guard can't read.
func calledAtRunTime(words []string) []string {
	if len(words) == 0 || !spelledAtRunTime(words[0]) {
		return words
	}
	return append([]string{unreadable + words[0]}, words[1:]...)
}

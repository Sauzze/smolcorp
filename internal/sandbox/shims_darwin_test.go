package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SA-T2 on this Mac, with stand-ins only (owner Q5 (a)), on M5's work desk (escape2_darwin_test.go): the real
// /usr/bin/sandbox-exec and the real profile, with the shims in an install folder the desk's Policy denies, as it
// denies the app (shims_test.go shows the app's own is). The shims are copied into a plain folder: nothing runs from
// a folder named .app on this Mac (_rules.md).
//
// The normal-work rows (§A4.3 "Still works"): swift build, and swift test where SMOLCORP_TEST_SWIFT_TEST=1, through
// the shim with no flag typed; xcodebuild through the shim where SMOLCORP_TEST_XCODEBUILD=1 (CI's Mac sandbox job and
// the runner: Xcode writes in the real home's Library whatever HOME says, so never on a Mac someone uses).

// shimDesk is a work desk whose Policy denies an install folder holding the shims, and the folder. The install sits in
// a folder of its own (apps), as the app sits in /Applications.
func shimDesk(t *testing.T) (*workDesk, string) {
	t.Helper()
	w := newWorkDesk(t, IOKitClasses())
	install := filepath.Join(w.base, "apps", "inst")
	folder := shimFolder(t, install)
	w.pl.InstallRoots = append(w.pl.InstallRoots, install)
	w.prepareWith(t, nil, w.iokit)
	return w, folder
}

func TestOnAMacTheShimsRunFirstInADeskAndNoDeskChangesThem(t *testing.T) {
	w, folder := shimDesk(t)
	tools := filepath.Join(w.base, "tools")
	for _, name := range shimNames {
		mustWrite(t, filepath.Join(tools, name), printsItsWords)
		if err := os.Chmod(filepath.Join(tools, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := []string{"PATH=" + folder + ":" + tools + ":/usr/bin:/bin"}

	// Called by name, as a desk's agent and its scripts call them: the shim runs first, and hands on with its flag.
	for name, want := range map[string]string{"swift": "build\n--disable-sandbox\n", "xcodebuild": "build\nENABLE_USER_SCRIPT_SANDBOXING=NO\n"} {
		r := w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", name+" build")
		if r.status != 0 || r.stdout != filepath.Join(tools, name)+"\n"+want {
			t.Errorf("%s build in the desk: %+v, want %q", name, r, filepath.Join(tools, name)+"\n"+want)
		}
	}

	// No change to them: written, emptied, removed, moved, linked over, their mode, their folder moved, a tool added
	// to it that PATH would take first, a hard link to a shim made in the desk's own place (to change it through), the
	// install folder moved, and the folder that holds the install moved (RT126SAT2 A1, B1, B2). The same changes in the
	// desk's own place work (the control: own/apps/inst/shims, its own copy).
	// $F is the shims' folder, $I the install folder that holds it, $A the folder that holds the install.
	hardLink := filepath.Join(w.root, "hl")
	spelled := func(shims string) *strings.Replacer {
		return strings.NewReplacer("$F", shims, "$I", filepath.Dir(shims), "$A", filepath.Dir(filepath.Dir(shims)), "$HL", hardLink)
	}
	// The ones that would take the folder or a shim away come last, so a change let through can't hide the rest.
	for _, change := range []string{
		"ln $F/swift $HL && echo 'exec /bin/sh' >> $HL", "echo 'exec /bin/sh' >> $F/swift", ": > $F/swift",
		"chmod 777 $F/swift", "printf '#!/bin/sh\\n' > $F/git", "ln -s /bin/sh $F/node",
		"ln -sf /bin/sh $F/swift", "rm -f $F/swift", "mv $F/swift $F/swift.x", "mv $F $F.x", "mv $I $I.x", "mv $A $A.x",
	} {
		if try := spelled(folder).Replace(change); w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", try).status == 0 {
			t.Errorf("a desk changed the shims: %q passed", try)
		}
		for _, gone := range []string{filepath.Join(w.root, "own"), hardLink} {
			if err := os.RemoveAll(gone); err != nil {
				t.Fatal(err)
			}
		}
		control := spelled(shimFolder(t, filepath.Join(w.root, "own", "apps", "inst"))).Replace(change)
		if r := w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", control); r.status != 0 {
			t.Errorf("the control, %q in the desk's own place, failed: %+v", control, r)
		}
	}
	for _, name := range shimNames {
		repo, _ := os.ReadFile(filepath.Join("..", "..", "Resources", "shims", name))
		if got, err := os.ReadFile(filepath.Join(folder, name)); err != nil || string(got) != string(repo) {
			t.Errorf("%s isn't as shipped after the desk's tries: %v", name, err)
		}
	}
	for _, added := range []string{"git", "node"} {
		if _, err := os.Lstat(filepath.Join(folder, added)); err == nil {
			t.Errorf("the desk added %s to the shims' folder", added)
		}
	}
}

// RT126SAT2 B1, probe D, under the real profile: a desk plants a swift or an xcodebuild in a folder the search takes
// before the real one (a stand-in ~/.local/bin, and a Homebrew bin outside the home folder), by a copy, a link, a hard
// link, a rename, or a folder of its own brought in whole. Each is refused, and the next desk that calls the tool by
// name (the GM, say: here the same desk again) runs the real one through its shim. The desk's Policy holds the names
// as the core's start does (ResolvePrograms over the search). The control: the same plant of a name no one holds
// works, so the folders are the desk's to write and only the names are held.
func TestOnAMacNoDeskPlantsWhatAnotherDesksShimRuns(t *testing.T) {
	w := newWorkDesk(t, IOKitClasses())
	folder := shimFolder(t, filepath.Join(w.base, "apps", "inst"))
	local, brew, tools := filepath.Join(w.home, ".local", "bin"), filepath.Join(w.base, "brew", "bin"), filepath.Join(w.base, "tools")
	for _, name := range shimNames {
		mustWrite(t, filepath.Join(tools, name), printsItsWords)
		if err := os.Chmod(filepath.Join(tools, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(brew, 0o700); err != nil {
		t.Fatal(err)
	}
	onlyPrograms(t, shimTools()...)
	ps, err := ResolvePrograms(Finding{GOOS: "darwin", Home: w.home, Search: []string{local, brew, tools}, DeveloperDir: filepath.Join(w.base, "no-developer-tools")})
	if err != nil {
		t.Fatal(err)
	}
	w.pl.InstallRoots = append(w.pl.InstallRoots, filepath.Join(w.base, "apps", "inst"))
	w.pl.InstallRoots = append(w.pl.InstallRoots, ps.InstallRoots()...)
	w.prepareWith(t, nil, w.iokit)
	path := []string{"PATH=" + strings.Join([]string{folder, local, brew, tools, "/usr/bin", "/bin"}, ":")}
	plant := filepath.Join(w.root, "plant")
	mustWrite(t, plant, "#!/bin/sh\necho PLANTED \"$@\"\n")
	if err := os.Chmod(plant, 0o755); err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(w.root, "own")
	for _, name := range shimNames {
		for _, at := range []string{local, brew} {
			target := filepath.Join(at, name)
			for _, try := range []string{
				"cp " + plant + " " + target,
				"ln -s " + plant + " " + target,
				"ln " + plant + " " + target,
				"cp " + plant + " " + own + " && mv " + own + " " + target,
				// The folder brought in whole: the search's folder moved away, and one of the desk's own put in its place.
				"mkdir -p " + own + " && cp " + plant + " " + own + "/" + name + " && mv " + at + " " + at + ".x && mv " + own + " " + at,
				// The same, a folder higher: ~/.local or brew, with a bin of the desk's own in it.
				"mkdir -p " + own + "/bin && cp " + plant + " " + own + "/bin/" + name + " && mv " + filepath.Dir(at) + " " + filepath.Dir(at) + ".x && mv " + own + " " + filepath.Dir(at),
			} {
				if r := w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", try); r.status == 0 {
					t.Errorf("a desk planted %s: %q passed", name, try)
				}
				if _, err := os.Lstat(target); err == nil {
					t.Errorf("a desk planted %s at %s, by %q", name, target, try)
				}
				if err := os.RemoveAll(own); err != nil {
					t.Fatal(err)
				}
			}
		}
		// The next desk's build, by name: the real tool, through the shim, with its flag.
		want := map[string]string{"swift": "build\n--disable-sandbox\n", "xcodebuild": "build\nENABLE_USER_SCRIPT_SANDBOXING=NO\n"}[name]
		if r := w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", name+" build"); r.status != 0 || r.stdout != filepath.Join(tools, name)+"\n"+want {
			t.Errorf("%s build in the next desk: %+v, want %q", name, r, filepath.Join(tools, name)+"\n"+want)
		}
	}
	// The control: a program no one holds, put in the same folders by the same desk.
	for _, at := range []string{local, brew} {
		if r := w.work(t, "", w.root, 30*time.Second, path, "/bin/sh", "-c", "cp "+plant+" "+at+"/jq && jq"); r.status != 0 || r.stdout != "PLANTED\n" {
			t.Errorf("the control, jq installed in %s, failed: %+v", at, r)
		}
	}
}

func TestOnAMacSwiftAndXcodebuildWorkInADeskThroughTheShims(t *testing.T) {
	t.Run("swift build and test, through the shim, with no flag typed", func(t *testing.T) {
		swift := tool(t, "swift")
		w, folder := shimDesk(t)
		pkg := func(name string) string {
			dir := filepath.Join(w.root, name)
			mustWrite(t, filepath.Join(dir, "Package.swift"), "// swift-tools-version:5.9\nimport PackageDescription\n"+
				"let package = Package(name: \"M\", targets: [.target(name: \"M\"), .testTarget(name: \"MTests\", dependencies: [\"M\"])])\n")
			mustWrite(t, filepath.Join(dir, "Sources", "M", "m.swift"), "public func inADesk() -> Bool { true }\n")
			mustWrite(t, filepath.Join(dir, "Tests", "MTests", "MTests.swift"), "import XCTest\n@testable import M\n"+
				"final class MTests: XCTestCase { func testInADesk() { XCTAssertTrue(inADesk()) } }\n")
			return dir
		}
		// The control: swift as it is, no shim, in the same desk. SwiftPM's own sandbox doesn't start inside the desk's
		// (P10), which is why the shim is there; said here, not required, should a later SwiftPM nest.
		bare := w.work(t, "", pkg("bare"), 5*time.Minute, []string{pathOf(swift)}, "/bin/sh", "-c", "swift build")
		t.Logf("CONTROL: swift build with no shim and no flag, in the desk: status %d (%s)", bare.status, tail(bare.stderr+bare.stdout))

		dir := pkg("pkg")
		env := []string{"PATH=" + folder + ":" + filepath.Dir(swift) + ":/usr/bin:/bin:/usr/sbin:/sbin"}
		worked(t, "swift build, through the shim", w.work(t, "", dir, 5*time.Minute, env, "/bin/sh", "-c", "swift build"))
		// No `swift test` on a Mac someone uses until CLIP1 lands (_rules.md): the runner and CI set SMOLCORP_TEST_SWIFT_TEST.
		if os.Getenv("SMOLCORP_TEST_SWIFT_TEST") != "1" {
			t.Log("swift test: not here (SMOLCORP_TEST_SWIFT_TEST=1 runs it: the runner and CI)")
			return
		}
		worked(t, "swift test, through the shim", w.work(t, "", dir, 10*time.Minute, env, "/bin/sh", "-c", "swift test"))
	})

	t.Run("xcodebuild, through the shim, with a script phase", func(t *testing.T) {
		if os.Getenv("SMOLCORP_TEST_XCODEBUILD") != "1" {
			// Not turned into a failure by SMOLCORP_SANDBOX_REQUIRED: it may run only on a Mac nobody uses.
			t.Skip("SKIPPED, xcodebuild writes in the real home's Library whatever HOME says: SMOLCORP_TEST_XCODEBUILD=1 runs it (CI and the runner)")
		}
		xcodebuild := tool(t, "xcodebuild")
		w, folder := shimDesk(t)
		dir := filepath.Join(w.root, "xc")
		mustWrite(t, filepath.Join(dir, "InADesk.xcodeproj", "project.pbxproj"), scriptPhaseProject)
		build := func(profile string, path []string) ran {
			out := filepath.Join(dir, "in-a-desk")
			_ = os.Remove(out)
			r := w.work(t, profile, dir, 10*time.Minute, path, "/bin/sh", "-c", "xcodebuild -project InADesk.xcodeproj -target InADesk "+
				"-configuration Release SYMROOT="+filepath.Join(dir, "build")+" OBJROOT="+filepath.Join(dir, "obj")+" build")
			if said, err := os.ReadFile(out); r.status == 0 && (err != nil || string(said) != "in a desk\n") {
				r.status = -1
				r.stderr += "\nthe script phase didn't write its file: " + string(said)
			}
			return r
		}
		bare := build("", []string{pathOf(xcodebuild)})
		t.Logf("CONTROL: xcodebuild with no shim, in the desk: status %d (%s)", bare.status, tail(bare.stderr+bare.stdout))

		shimmed := []string{"PATH=" + folder + ":" + filepath.Dir(xcodebuild) + ":/usr/bin:/bin:/usr/sbin:/sbin"}
		if r := build("", shimmed); r.status == 0 {
			return
		} else {
			t.Logf("xcodebuild through the shim, under the desk's profile: status %d (%s)", r.status, tail(r.stderr+r.stdout))
		}
		// It didn't build. As M5's Chrome row: the same profile with every Mach service allowed tells whether the list
		// is all that stops it. If so, what Xcode needs is off the list on purpose (said, never added); if not, the shim
		// or something else of the profile's stops it, and that fails here.
		if r := build(w.appended(t, "every-mach-service.sb", "(allow mach-lookup) ; a probe: is the Mach list all that stops it?"), shimmed); r.status != 0 {
			t.Errorf("xcodebuild fails in a desk through the shim for more than the Mach list: status %d (%s)", r.status, tail(r.stderr+r.stdout))
		} else {
			t.Log("COST: xcodebuild here needs Mach services that stay off the list; it builds in a desk only once they are allowed, " +
				"so it isn't on the normal-work list yet: for SIERRA")
		}
	})
}

// scriptPhaseProject is the smallest Xcode project with a script phase: one aggregate target that writes a file, with
// Xcode's script sandbox on, as new projects have it.
const scriptPhaseProject = `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 56;
	objects = {
		5A0000000000000000000001 = {
			isa = PBXProject;
			buildConfigurationList = 5A0000000000000000000002;
			compatibilityVersion = "Xcode 14.0";
			developmentRegion = en;
			hasScannedForEncodings = 0;
			knownRegions = (
				en,
			);
			mainGroup = 5A0000000000000000000003;
			projectDirPath = "";
			projectRoot = "";
			targets = (
				5A0000000000000000000004,
			);
		};
		5A0000000000000000000002 = {
			isa = XCConfigurationList;
			buildConfigurations = (
				5A0000000000000000000005,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
		5A0000000000000000000003 = {
			isa = PBXGroup;
			children = (
			);
			sourceTree = "<group>";
		};
		5A0000000000000000000004 = {
			isa = PBXAggregateTarget;
			buildConfigurationList = 5A0000000000000000000006;
			buildPhases = (
				5A0000000000000000000007,
			);
			dependencies = (
			);
			name = InADesk;
			productName = InADesk;
		};
		5A0000000000000000000005 = {
			isa = XCBuildConfiguration;
			buildSettings = {
				SDKROOT = macosx;
			};
			name = Release;
		};
		5A0000000000000000000006 = {
			isa = XCConfigurationList;
			buildConfigurations = (
				5A0000000000000000000008,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
		5A0000000000000000000007 = {
			isa = PBXShellScriptBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			inputPaths = (
			);
			outputPaths = (
				"$(SRCROOT)/in-a-desk",
			);
			runOnlyForDeploymentPostprocessing = 0;
			shellPath = /bin/sh;
			shellScript = "echo in a desk > \"$SCRIPT_OUTPUT_FILE_0\"\n";
		};
		5A0000000000000000000008 = {
			isa = XCBuildConfiguration;
			buildSettings = {
				ENABLE_USER_SCRIPT_SANDBOXING = YES;
			};
			name = Release;
		};
	};
	rootObject = 5A0000000000000000000001;
}
`

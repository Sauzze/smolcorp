package desktop

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Role is one the office can hire, as the Mac app's Role: its name, the
// agent it is and how that agent starts (its model, and its effort where it
// has one), what it asks you before it starts, the instructions it works on
// once you've answered, the badge it wears in the office and what its agents
// wear there. Plain text you
// own, kept on this computer: nothing runs until you start one with
// `smolcorp run --role`. All of it is the agent's first message, there to
// read in its terminal, and nothing changes what the agent may do without
// asking.
type Role struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Agent string `json:"agent"`
	// Empty for the agent's own default.
	Model string `json:"model"`
	// Empty for the agent's own, or one of effortLevels for an agent with an
	// effort setting.
	Effort string `json:"effort"`
	// What it asks you before it starts, after which project to work on.
	Questions    []string `json:"questions"`
	Instructions string   `json:"instructions"`
	Badge        string   `json:"badge"`
	// What its agents wear in the office, one of a team member's outfits, or
	// empty for none: a team member's own outfit still comes first.
	Outfit string `json:"outfit"`
	// Where its brief comes from, as the Mac app's: nil as kept before roles
	// had briefs, which checkedRoles gives one.
	Brief *RoleBrief `json:"brief,omitempty"`
}

// RoleBrief is where a role's brief comes from, as the Mac app's Brief: its
// own instructions ("text"), one of the briefs that come with the Mac app
// ("library", by its id), or a Markdown file picked in setup, copied in whole
// ("file", with the file's name kept to show). The library, and writing a
// brief on hire, are the Mac app's only for now: here a role starts on its
// instructions whatever its brief, and its brief is kept as it is, so a
// config goes back and forth without losing it.
type RoleBrief struct {
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Text     string `json:"text,omitempty"`
	FileName string `json:"fileName,omitempty"`
}

const (
	roleNameLimit         = 24
	roleModelLimit        = 64
	roleInstructionsLimit = 4000
	// The most questions a role asks, and the longest each can be.
	roleQuestionsMost = 8
	roleQuestionLimit = 200
	// The most a picked brief file can hold, in bytes, and how long its name can be.
	roleBriefFileLimit     = 32 * 1024
	roleBriefFileNameLimit = 100
)

var (
	// The briefs that come with the Mac app, by id: the shipped roles' own,
	// then those for roles of your own.
	briefLibraryIDs = []string{"reviewer", "tester", "bughunter", "docs", "refactorer", "artdirector", "lawyer", "therapist", "plumber",
		"gm", "janitor", "firefighter", "pilot", "postman", "accountant", "marketer", "chef", "thinker",
		"builder", "security-auditor", "perf-auditor", "designer", "pixel-artist", "planner", "fixer", "study"}
	// A shipped role whose brief has an id of its own: the Security guard's is the security auditor's.
	briefLibraryOfRole = map[string]string{"securityguard": "security-auditor"}
	// What a role can wear in the office: the shipped roles' own, then some
	// for yours. office.js draws them.
	roleBadges = []string{"glasses", "clipboard", "magnifier", "quill", "wrench", "star", "bulb", "headset", "flag", "shield",
		"rocket", "coffee", "heart", "palette", "scales", "speech", "pipe", "crown",
		"broom", "flame", "plane", "envelope", "coins", "megaphone", "lock", "toque"}
	// The roles that come with the app. Each asks what to work on before it
	// starts, then reports and asks before it changes anything.
	shippedRoles = []Role{
		{ID: "reviewer", Name: "Reviewer", Agent: "claude", Model: "opus", Effort: "high", Badge: "glasses",
			Questions: []string{
				"Which changes should you review: a branch, a pull request, or a range of commits?",
				"What should they be compared with, for example main, or the commit before?",
				"What matters most this time: bugs, security, performance, style, or all of them?",
			},
			Instructions: "Review the changes I point you to. List the problems you find, most serious first, each with its file and line " +
				"and why it matters. Don't edit any files: report, and wait for me to say which to fix."},
		{ID: "tester", Name: "Tester", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "clipboard",
			Questions: []string{
				"Which tests should you run: all of them, a folder, or a single file?",
				"Is there anything to set up or start first, such as a database, a server or environment variables?",
			},
			Instructions: "Run the tests I point you to. If any fail, find out why, and explain each failure with the fix you'd make. " +
				"Don't change any code until I say so."},
		{ID: "bughunter", Name: "Bug hunter", Agent: "claude", Model: "opus", Effort: "high", Badge: "magnifier", Outfit: "detective",
			Questions: []string{"Which area or feature should you focus on, or should you look everywhere?"},
			Instructions: "Look for bugs where I point you to: wrong results, crashes, edge cases that aren't handled. Report each one " +
				"with how to reproduce it and how sure you are. Don't fix anything until I say which."},
		{ID: "docs", Name: "Docs writer", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "quill", Outfit: "librarian",
			Questions: []string{"Which docs should you check: the README, a docs folder, or comments in the code?"},
			Instructions: "Check the docs I point you to against what the code actually does. List what's missing, wrong or out of date, " +
				"with the changes you'd make. Wait for my OK before editing any file."},
		{ID: "refactorer", Name: "Refactorer", Agent: "claude", Model: "opus", Effort: "high", Badge: "wrench", Outfit: "hardhat",
			Questions: []string{
				"Which area or files should you work on?",
				"Any limits: anything you mustn't change, such as public APIs, file names or behaviour?",
			},
			Instructions: "In the area I point you to, find the code that's hardest to follow or change. Propose one refactor at a time: " +
				"what you'd change, why, and what could break, within the limits I've given. Wait for my OK before editing, and " +
				"keep each change small."},
		{ID: "artdirector", Name: "Art Director", Agent: "claude", Model: "opus", Effort: "high", Badge: "palette", Outfit: "artdirector",
			Questions: []string{
				"What should you look at: assets, screenshots, a page or a screen of the app?",
				"Is there a style or brief it has to match?",
			},
			Instructions: "Review the visual work I point you to. Say whether each piece belongs with the rest and reads well at the size " +
				"it's shown, and list the fixes you'd make, most important first. Don't redraw or edit anything until I say " +
				"which."},
		{ID: "lawyer", Name: "Lawyer", Agent: "claude", Model: "opus", Effort: "high", Badge: "scales", Outfit: "lawyer",
			Questions: []string{
				"What should you read: the licence, terms, a privacy policy, or the dependencies' licences?",
				"Where will the project be used or sold, and by whom?",
			},
			Instructions: "Read the documents and licences I point you to. Flag the risks and gaps you find, each with where it is and " +
				"why it matters. Say plainly that this isn't legal advice, and when a real lawyer should look. Don't change any " +
				"files."},
		{ID: "therapist", Name: "Therapist", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "speech", Outfit: "therapist",
			Questions: []string{"What are you stuck on: a problem, a decision, or where to start?"},
			Instructions: "Be a calm sounding board. Ask one question at a time to help me untangle it, then sum up the options with what " +
				"each costs. Don't edit any code. Keep it light and practical; you're no substitute for real care, so say so if " +
				"we go beyond work."},
		{ID: "plumber", Name: "Plumber", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "pipe", Outfit: "plumber",
			Questions: []string{
				"Which pipeline should you look at: the build, CI, deploy scripts, or a data job?",
				"Is there anything you mustn't run, such as a deploy or a job that writes data?",
			},
			Instructions: "Run and watch the pipeline I point you to, and find where it leaks or clogs: failures, slow steps, flaky ones. " +
				"Report each with the fix you'd make, and wait for my OK before making it. Never deploy or push unless I ask."},
		{ID: "gm", Name: "General Manager", Agent: "claude", Model: "opus", Effort: "high", Badge: "crown", Outfit: "gmexec",
			Questions: []string{
				"What's the goal, and which project or projects does it cover?",
				"What may the team spend (plans, API keys, cloud time), and which agents and accounts may it use?",
				"What do you want to approve yourself, beyond push, publish, deploy, spending and deleting?",
			},
			Instructions: "Run a team of agents toward the goal. First plan it: tasks one agent can finish and prove in one sitting, and " +
				"wait for my OK on the plan. Then hire one agent per task, each in its own worktree or folder, branch and port " +
				"range, with a brief that says what done means and what proof to show. Check that each hire really started, " +
				"check their proof yourself, and have a second agent review any change before it merges. Decide the team and " +
				"process calls yourself; ask me before anything is pushed, merged to main, published, deployed, released, " +
				"deleted or paid for, or reaches a real person. Never type into another window, never ask an employee to do " +
				"what your own checks refused, and when an employee is done, end its session, make sure it's gone, and close " +
				"its window."},
		{ID: "janitor", Name: "Janitor", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "broom", Outfit: "janitor",
			Questions: []string{
				"Which project or area should you tidy up?",
				"Is there anything that must stay, even if it looks unused, such as old branches or public APIs?",
			},
			Instructions: "Look for what's left lying around: dead code, unused files and dependencies, stale branches, old TODOs. List " +
				"each with why you think it's unused and how sure you are. Don't delete anything until I say which."},
		{ID: "firefighter", Name: "Firefighter", Agent: "claude", Model: "opus", Effort: "high", Badge: "flame", Outfit: "firefighter",
			Questions: []string{
				"What's on fire: what's broken, since when, and who is affected?",
				"Is there an error, a log or a failing request you can look at?",
			},
			Instructions: "Triage first: find out what's broken, how badly, and the most likely cause, and tell me. Then propose the " +
				"smallest safe fix, with how to undo it. Don't change, deploy or restart anything until I say so."},
		{ID: "pilot", Name: "Pilot", Agent: "claude", Model: "opus", Effort: "high", Badge: "plane", Outfit: "pilot",
			Questions: []string{
				"What's going out: which project, which version, and to where?",
				"Is there a release checklist or a deploy script you should follow?",
			},
			Instructions: "Prepare the release I point you to and run through the checklist: tests, version, changelog, migrations, " +
				"config. Report each item as ready or not, with what's missing, and wait for my OK before changing anything. " +
				"Never deploy or publish unless I tell you to."},
		{ID: "postman", Name: "Postman", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "envelope", Outfit: "postman",
			Questions: []string{
				"Which notifications or webhooks should you look at: sending, receiving, or both?",
				"Where do they go, and is there a test endpoint you can use?",
			},
			Instructions: "Check how the notifications and webhooks I point you to are wired: what's sent, the payloads, signatures, " +
				"retries and what happens when delivery fails. Report what's wrong or missing with the fix you'd make, and wait " +
				"for my OK before changing anything."},
		{ID: "accountant", Name: "Accountant", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "coins", Outfit: "accountant",
			Questions: []string{
				"Which costs should you look at: API calls, usage, billing code, or budgets?",
				"Is there a budget or a limit it has to stay within?",
			},
			Instructions: "Find where the project spends money: API calls, usage, billing and budgets in the code. Estimate what each " +
				"costs, flag waste and missing limits, and list the savings you'd make. Don't change anything until I say " +
				"which."},
		{ID: "marketer", Name: "Marketer", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "megaphone", Outfit: "marketer",
			Questions: []string{
				"What should you write: release notes, landing copy, or an announcement?",
				"Who is it for, and what tone should it have?",
			},
			Instructions: "Read what changed and write the copy I asked for: clear, short, and true to what the code actually does. Show " +
				"me a draft before editing any file, and don't publish or post anything."},
		{ID: "securityguard", Name: "Security guard", Agent: "claude", Model: "opus", Effort: "high", Badge: "lock", Outfit: "securityguard",
			Questions: []string{
				"Which project or area should you check?",
				"Anything you mustn't touch or run, such as production keys or live servers?",
			},
			Instructions: "Review the code I point you to for security problems: leaked secrets, weak auth, injection, unsafe " +
				"dependencies. Report each with where it is, how serious it is and the fix you'd make, most serious first. " +
				"Don't fix anything until I say which, and never print a secret in full."},
		{ID: "chef", Name: "Chef", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "toque", Outfit: "chef",
			Questions: []string{
				"What should you cook up: a scaffold, a prototype, or a new feature's first draft?",
				"What's the recipe: the stack, the files, and anything it has to include?",
			},
			Instructions: "Follow the recipe I give you to cook up a scaffold or prototype. First list what you'll make, file by file, " +
				"and wait for my OK. Then build it in small steps, and keep it out of the way of existing code."},
		{ID: "thinker", Name: "Thinker", Agent: "claude", Model: "opus", Effort: "max", Badge: "bulb", Outfit: "professor",
			Questions: []string{
				"What should you think about: a question about this project, or a broader one?",
				"What should your answer help me decide or do?",
				"Anything to read first, any limits, or ideas already ruled out?",
			},
			Instructions: "Think the question I give you through before doing anything else. Read what matters in this project, and " +
				"on the web if it helps. Write your thinking to a file: the options, their trade-offs, what you recommend " +
				"and why, the open questions, and your sources. Don't change any code: report, and wait for my OK on any " +
				"next step."},
	}
	// The shipped roles as 1.7.1 had them, which assumed what to work on. A
	// role still exactly so is brought up to date as it loads; one you've
	// changed is left as you have it.
	shippedIn171 = []Role{
		{ID: "reviewer", Name: "Reviewer", Agent: "claude", Model: "opus", Effort: "high", Badge: "glasses",
			Instructions: "Review the changes on this branch against main. List the problems you find, most serious first, each with " +
				"its file and line and why it matters. Don't edit any files: report, and wait for me to say which to fix."},
		{ID: "tester", Name: "Tester", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "clipboard",
			Instructions: "Run this project's tests. If any fail, find out why, and explain each failure with the fix you'd make. " +
				"Don't change any code until I say so."},
		{ID: "bughunter", Name: "Bug hunter", Agent: "claude", Model: "opus", Effort: "high", Badge: "magnifier",
			Instructions: "Look through this project for bugs: wrong results, crashes, edge cases that aren't handled. Report each " +
				"one with how to reproduce it and how sure you are. Don't fix anything until I say which."},
		{ID: "docs", Name: "Docs writer", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "quill",
			Instructions: "Check the README and the docs against what the code actually does. List what's missing, wrong or out of " +
				"date, with the changes you'd make. Wait for my OK before editing any file."},
		{ID: "refactorer", Name: "Refactorer", Agent: "claude", Model: "opus", Effort: "high", Badge: "wrench",
			Instructions: "Find the code in this project that's hardest to follow or change. Propose one refactor at a time: what " +
				"you'd change, why, and what could break. Wait for my OK before editing, and keep each change small."},
	}
)

// rolesOfferedBefore are the shipped roles a config kept before rolesOffered
// was given: the first five, before the Art Director and the rest came. A
// config that has seen a version's roles isn't given them again, so a role
// you deleted stays deleted while a new one is added (see loadConfig).
var rolesOfferedBefore = []string{"reviewer", "tester", "bughunter", "docs", "refactorer"}

// shippedBeforeOutfits are the Bug hunter, the Docs writer and the
// Refactorer as they were before they had an outfit: a role still exactly so
// is given its outfit as it loads, as shippedIn171 brings a role up to date.
func shippedBeforeOutfits() []Role {
	var old []Role
	for _, role := range shippedRoles {
		if role.ID == "bughunter" || role.ID == "docs" || role.ID == "refactorer" {
			role.Outfit = ""
			role.Questions = slices.Clone(role.Questions)
			old = append(old, role)
		}
	}
	return old
}

// shippedBeforeTeams is the General Manager as it came before it ran a team,
// when it only planned: one still exactly so, its brief aside, is brought up
// to date as it loads.
func shippedBeforeTeams() []Role {
	return []Role{
		{ID: "gm", Name: "General Manager", Agent: "claude", Model: "opus", Effort: "high", Badge: "crown", Outfit: "gmexec",
			Questions: []string{
				"What's the goal, and which project or projects does it cover?",
				"Who or what should do the work: other agents, roles, or you alone?",
			},
			Instructions: "Plan the work toward the goal: split it into tasks small enough for one agent each, say who should take each, " +
				"and keep a list of what's done and what's left. Ask before starting, handing out or merging anything."},
	}
}

// shippedRoleIDs are the ids of the roles that come with the app, in order.
func shippedRoleIDs() []string {
	ids := make([]string, len(shippedRoles))
	for i, role := range shippedRoles {
		ids[i] = role.ID
	}
	return ids
}

// UnmarshalJSON reads a role field by field: anything missing or of the
// wrong kind is empty, and checked decides what's kept.
func (r *Role) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	text := func(key string) string {
		var s string
		json.Unmarshal(raw[key], &s)
		return s
	}
	*r = Role{ID: text("id"), Name: text("name"), Agent: text("agent"), Model: text("model"), Effort: text("effort"),
		Instructions: text("instructions"), Badge: text("badge"), Outfit: text("outfit")}
	// Missing from a role kept before roles asked questions; text among them is kept.
	var questions []json.RawMessage
	json.Unmarshal(raw["questions"], &questions)
	for _, question := range questions {
		var s string
		if json.Unmarshal(question, &s) == nil {
			r.Questions = append(r.Questions, s)
		}
	}
	// Missing from a role kept before roles had briefs; an unknown kind isn't one.
	var brief map[string]json.RawMessage
	if json.Unmarshal(raw["brief"], &brief) == nil {
		field := func(key string) string {
			var s string
			json.Unmarshal(brief[key], &s)
			return s
		}
		if kind := field("kind"); kind == "text" || kind == "library" || kind == "file" {
			r.Brief = &RoleBrief{Kind: kind, ID: field("id"), Text: field("text"), FileName: field("fileName")}
		}
	}
	return nil
}

// checkedBriefFile is a picked brief as kept, as the Mac app keeps it: its
// text without a byte-order mark, with plain line ends and no control
// characters but tabs, and its name without a folder; false if it's empty,
// too long, not text, or not a .md, .markdown or .txt file. Its
// placeholders are checked by the Mac app, which fills them.
func checkedBriefFile(text, name string) (RoleBrief, bool) {
	if strings.ContainsAny(text, "\x00\uFFFD") {
		return RoleBrief{}, false
	}
	text = strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(text, "\uFEFF"), "\r\n", "\n"), "\r", "\n")
	text = strings.Map(func(c rune) rune {
		if c != '\n' && c != '\t' && unicode.IsControl(c) {
			return -1
		}
		return c
	}, text)
	parts := strings.FieldsFunc(name, func(c rune) bool { return c == '/' || c == '\\' })
	fileName := ""
	if len(parts) > 0 {
		fileName = strings.TrimSpace(strings.Map(func(c rune) rune {
			if unicode.IsControl(c) {
				return -1
			}
			return c
		}, parts[len(parts)-1]))
	}
	extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	if strings.TrimSpace(text) == "" || len(text) > roleBriefFileLimit || fileName == "" || len([]rune(fileName)) > roleBriefFileNameLimit ||
		!slices.Contains([]string{"md", "markdown", "txt"}, extension) {
		return RoleBrief{}, false
	}
	return RoleBrief{Kind: "file", Text: text, FileName: fileName}, true
}

// defaultBrief is the brief a role kept without one is given, as the Mac
// app gives it: a shipped role still on its shipped instructions, its own
// from the library; any other, its instructions, so an edit of yours stays yours.
func (r Role) defaultBrief() *RoleBrief {
	library, ok := briefLibraryOfRole[r.ID]
	if !ok && slices.Contains(briefLibraryIDs, r.ID) {
		library, ok = r.ID, true
	}
	if ok && slices.ContainsFunc(shippedRoles, func(s Role) bool { return s.ID == r.ID && s.Instructions == r.Instructions }) {
		return &RoleBrief{Kind: "library", ID: library}
	}
	return &RoleBrief{Kind: "text", Text: r.Instructions}
}

// checked is the role as kept, and false without an id or a name. An
// unknown agent is Claude Code; a model with anything but a model name's
// characters, an effort the agent hasn't, an unknown badge or an outfit the
// office doesn't know is none.
func (r Role) checked() (Role, bool) {
	id := strings.TrimSpace(r.ID)
	name := strings.TrimSpace(strings.Join(strings.FieldsFunc(r.Name, func(c rune) bool { return c == '\n' || c == '\r' }), " "))
	name = strings.TrimSpace(string([]rune(name)[:min(len([]rune(name)), roleNameLimit)]))
	if len(id) < 1 || len(id) > 32 || strings.ContainsFunc(id, func(c rune) bool { return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') }) || name == "" {
		return Role{}, false
	}
	agent := Agent(r.Agent)
	if !slices.Contains(agents, agent) {
		agent = Claude
	}
	model := strings.TrimSpace(r.Model)
	if len(model) > roleModelLimit || strings.ContainsFunc(model, func(c rune) bool {
		return !(c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c) || strings.ContainsRune("-._/:[]", c)))
	}) {
		model = ""
	}
	effort := ""
	if agent.HasEffortDial() && slices.Contains(effortLevels, r.Effort) {
		effort = r.Effort
	}
	// Lines and tabs stay; other control characters go.
	instructions := strings.TrimSpace(strings.Map(func(c rune) rune {
		if c != '\n' && c != '\t' && unicode.IsControl(c) {
			return -1
		}
		return c
	}, r.Instructions))
	instructions = string([]rune(instructions)[:min(len([]rune(instructions)), roleInstructionsLimit)])
	badge := ""
	if slices.Contains(roleBadges, r.Badge) {
		badge = r.Badge
	}
	outfit := ""
	if r.Outfit != "auto" && slices.Contains(teamChoices["outfit"], r.Outfit) {
		outfit = r.Outfit
	}
	// Each question one line, without control characters; empty ones go.
	questions := []string{}
	for _, question := range r.Questions {
		question = strings.TrimSpace(strings.Map(func(c rune) rune {
			if unicode.IsControl(c) {
				return -1
			}
			return c
		}, question))
		question = strings.TrimSpace(string([]rune(question)[:min(len([]rune(question)), roleQuestionLimit)]))
		if question != "" && len(questions) < roleQuestionsMost {
			questions = append(questions, question)
		}
	}
	// A brief that can't be one is none. One on its own instructions has them as its text, or gives them its own.
	var brief *RoleBrief
	if r.Brief != nil {
		switch r.Brief.Kind {
		case "library":
			if slices.Contains(briefLibraryIDs, r.Brief.ID) {
				brief = &RoleBrief{Kind: "library", ID: r.Brief.ID}
			}
		case "file":
			if file, ok := checkedBriefFile(r.Brief.Text, r.Brief.FileName); ok {
				brief = &file
			}
		case "text":
			if instructions == "" {
				given, _ := Role{ID: id, Name: name, Instructions: r.Brief.Text}.checked()
				instructions = given.Instructions
			}
			brief = &RoleBrief{Kind: "text", Text: instructions}
		}
	}
	return Role{ID: id, Name: name, Agent: string(agent), Model: model, Effort: effort, Questions: questions, Instructions: instructions,
		Badge: badge, Outfit: outfit, Brief: brief}, true
}

// checkedRoles is roles as kept: each checked, each id once, and every one of
// your own, in their order. None is an empty list. A
// shipped role still as 1.7.1 had it is brought up to date, and one still as
// it was before it had an outfit gets it. Each has a brief: one kept without
// is given its defaultBrief.
func checkedRoles(roles []Role) []Role {
	kept := []Role{}
	for _, given := range roles {
		role, ok := given.checked()
		if !ok || slices.ContainsFunc(kept, func(r Role) bool { return r.ID == role.ID }) {
			continue
		}
		if role.Brief == nil && (slices.ContainsFunc(shippedIn171, role.sameAs) || slices.ContainsFunc(shippedBeforeOutfits(), role.sameAs)) {
			role = shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == role.ID })]
		}
		// The General Manager from before it ran a team, whatever brief it was kept with: sameAs leaves briefs aside.
		if slices.ContainsFunc(shippedBeforeTeams(), role.sameAs) {
			role = shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == role.ID })]
		}
		if role.Brief == nil {
			role.Brief = role.defaultBrief()
		}
		kept = append(kept, role)
	}
	return kept
}

// syncedRoles is roles as setup sends them, with a brief on their own
// instructions kept the same text as those, as the Mac app keeps it:
// whichever of the two changed from saved wins, the brief's text if both did.
// A library role whose instructions changed is on them from then on, so an
// edit is never ignored.
func syncedRoles(given, saved []Role) []Role {
	synced := slices.Clone(given)
	for i, role := range synced {
		j := slices.IndexFunc(saved, func(s Role) bool { return s.ID == role.ID })
		if role.Brief != nil && role.Brief.Kind == "library" && j >= 0 && role.Instructions != saved[j].Instructions {
			synced[i].Brief = &RoleBrief{Kind: "text", Text: role.Instructions}
			continue
		}
		if role.Brief == nil || role.Brief.Kind != "text" {
			continue
		}
		before := role.Instructions
		if j >= 0 {
			before = saved[j].Instructions
		}
		if role.Brief.Text != "" && role.Brief.Text != before {
			synced[i].Instructions = role.Brief.Text
		}
		synced[i].Brief = &RoleBrief{Kind: "text", Text: synced[i].Instructions}
	}
	return synced
}

// sameAs is whether two roles are the same in every part but their briefs.
func (r Role) sameAs(other Role) bool {
	return r.ID == other.ID && r.Name == other.Name && r.Agent == other.Agent && r.Model == other.Model && r.Effort == other.Effort &&
		slices.Equal(r.Questions, other.Questions) && r.Instructions == other.Instructions && r.Badge == other.Badge &&
		r.Outfit == other.Outfit
}

// roleStart is how a role's agent came to start where it is.
type roleStart int

const (
	// From a terminal with `smolcorp run --role`, in the folder you started it in.
	startedHere roleStart = iota
	// Hired from the office at a desk pinned to a project, in that project.
	hiredAtPinnedDesk
	// Hired from the office into the folder set up for new hires, such as
	// ~/Projects, rather than a project.
	hiredIntoGeneralFolder
)

// firstMessage is what the agent is first told: to only ask, in its first
// reply, which project to work on and the role's own questions, and to wait
// for the answers; then the role's instructions. Started where you chose,
// from a terminal, or at a pinned project's desk, it asks you to confirm
// that folder instead. A hire's words are the Mac app's.
func (r Role) firstMessage(folder string, how roleStart) string {
	if homeDir != "" {
		if rest, err := filepath.Rel(homeDir, folder); err == nil && !strings.HasPrefix(rest, "..") {
			folder = filepath.Join("~", rest)
		}
	}
	project := map[roleStart]string{
		startedHere:            "You've started in " + folder + ". Is that the one to work on?",
		hiredAtPinnedDesk:      "You've started in " + folder + ", this desk's project. Is that the one to work on?",
		hiredIntoGeneralFolder: "Which project should you work on: a folder on this computer, or a GitHub repository?",
	}[how]
	asked := []string{"1. " + project}
	for i, question := range r.Questions {
		asked = append(asked, fmt.Sprintf("%d. %s", i+2, question))
	}
	then := "Once I've answered, ask me what to do."
	if r.Instructions != "" {
		then = "Once I've answered, as my " + r.Name + ": " + r.Instructions
	}
	general := ""
	if how == hiredIntoGeneralFolder {
		general = "You've started in " + folder + ", a general folder, not a project. "
	}
	return general + "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n" + strings.Join(asked, "\n") + "\n\n" + then
}

// arguments are how the role's agent starts: its model, its effort, and
// message as its first. Each option is written whole, with its value after
// `=`, and the message after `--` where it stands alone, so text starting
// with a dash is never read as an option. Grok starts as it is: how it takes
// these hasn't been checked.
func (r Role) arguments(message string) []string {
	var args []string
	if r.Model != "" && Agent(r.Agent) != Grok {
		args = append(args, "--model="+r.Model)
	}
	switch Agent(r.Agent) {
	case Claude:
		if r.Effort != "" {
			args = append(args, "--effort="+r.Effort)
		}
		return append(args, "--", message)
	case Codex:
		return append(args, "--", message)
	case Gemini:
		return append(args, "--prompt-interactive="+message)
	case OpenCode:
		return append(args, "--prompt="+message)
	}
	return nil
}

// roleAgents are the agents a role can be, for the setup page: whether each
// has an effort, and whether it takes a first message.
func roleAgents() []map[string]any {
	list := []map[string]any{}
	for _, agent := range agents {
		list = append(list, map[string]any{"id": agent, "name": agent.Name(), "effort": agent.HasEffortDial(), "starts": agent != Grok})
	}
	return list
}

// forPhone is what the phone is told of a role: never its instructions.
func (r Role) forPhone() map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name, "badge": r.Badge, "outfit": r.Outfit}
}

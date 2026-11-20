package desktop

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// redactGoldenCorpus is text every kind of line redactText reads: each secret shape and label, with and without a
// transcript's label, a private key across lines and on one, and lines that look near one and aren't. Keys are put
// together here, so no file holds one whole.
func redactGoldenCorpus() []string {
	anthropic := "sk-" + "ant-" + "api03-" + "FAKEfakeFAKEfake"
	jwt := "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJ" + "zdWIiOiIxMjM0In0" + "." + "c2lnbmF0dXJl"
	aws := "wJalrXUtnFEMI/" + "K7MDENG/bPxRfiCY" + "EXAMPLEKEY"
	return []string{
		"Plain words, nothing to hide.",
		"USER: fix the elevator please",
		"never put a token in a brief",
		"USER: never put a token in a brief",
		"the key to this is patience",
		"keyboard shortcuts and a monkey",
		"monkey: banana",
		`"apiKey": "abc"`,
		"API_KEY=" + "fake123",
		"CLAUDE: set API_KEY = something",
		"TOOL Bash: export GITHUB_TOKEN=" + "fake",
		"TOOL Bash export with no colon password",
		"TOOL Read: /etc/hosts",
		"RESULT: " + anthropic,
		"ERROR: " + "re_" + "Fake123",
		"pre_fix re_ is not after a letter: are_you",
		"task-sk-is-a-name",
		"ghp_" + "FAKEFAKEFAKE",
		"use gho_" + "x and github_pat_" + "y",
		"xoxb-" + "1-2-3",
		"AKIA" + "FAKEFAKEFAKEFAKE",
		"STRIPE=" + "sk_" + "test_" + "FAKEfake",
		"pk_" + "live_" + "Fake",
		"jwt " + jwt,
		"DATABASE_URL=postgres://app:" + "hunter2" + "@db.local:5432/app",
		"REDIS_URL=redis://:" + "hunter2" + "@cache:6379",
		"see https://example.com/a:b@c",
		"Authorization: Bearer " + "abcdef",
		"bearer\tx",
		"my Passwd is here",
		"mot de passe oublié",
		"Secretary of state",
		"tokenizer settings",
		"aws " + aws,
		"commit 8d2f1c9a4b7e6d5c3b2a1f0e9d8c7b6a5f4e3d2c",
		"-----BEGIN " + "RSA PRIVATE KEY-----",
		"MIIEowIBAAKCAQEA",
		"USER: inside the key",
		"-----END " + "RSA PRIVATE KEY-----",
		"after the key",
		"one line -----BEGIN " + "PRIVATE KEY----- abc -----END " + "PRIVATE KEY-----",
		"after the one-line key",
		"CLAUDE: -----BEGIN " + "EC PRIVATE KEY-----",
		"still inside",
		"-----END " + "EC PRIVATE KEY----- trailing",
		"",
		"PEER desk-2: a token here",
		"REPORT from hire: all good",
		"déployer en production, puis fusionner",
		"emoji 🙂 and accents éàç",
		"x" + strings.Repeat("y", 300),
		"last line",
	}
}

// redactGolden is redactText's output on redactGoldenCorpus, as the code before MEM2's split gave it (bd6b196b),
// line for line.
var redactGolden = []string{
	"Plain words, nothing to hide.",
	"USER: fix the elevator please",
	"[redacted]",
	"USER: [redacted]",
	"the key to this is patience",
	"keyboard shortcuts and a monkey",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"CLAUDE: [redacted]",
	"TOOL Bash: [redacted]",
	"[redacted]",
	"TOOL Read: /etc/hosts",
	"RESULT: [redacted]",
	"ERROR: [redacted]",
	"pre_fix re_ is not after a letter: are_you",
	"task-sk-is-a-name",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"see https://example.com/a:b@c",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"mot de passe oublié",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"commit 8d2f1c9a4b7e6d5c3b2a1f0e9d8c7b6a5f4e3d2c",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"[redacted]",
	"after the key",
	"[redacted]",
	"after the one-line key",
	"CLAUDE: [redacted]",
	"[redacted]",
	"[redacted]",
	"",
	"[redacted]",
	"REPORT from hire: all good",
	"déployer en production, puis fusionner",
	"emoji 🙂 and accents éàç",
	"x" + strings.Repeat("y", 300),
	"last line",
}

// MEM2 split secretLine into shapes and labels; redactText's output stays byte for byte what it was.
func TestRedactTextGivesExactlyWhatItGaveBeforeTheSplit(t *testing.T) {
	got := strings.Split(redactText(strings.Join(redactGoldenCorpus(), "\n")), "\n")
	if !slices.Equal(got, redactGolden) {
		for i := range max(len(got), len(redactGolden)) {
			var g, w string
			if i < len(got) {
				g = got[i]
			}
			if i < len(redactGolden) {
				w = redactGolden[i]
			}
			if g != w {
				t.Errorf("line %d: got %q, want %q", i, g, w)
			}
		}
	}
}

// secretLine is the two lists, whole: every pattern of each, once, and no other.
func TestSecretLineIsItsShapesAndItsLabels(t *testing.T) {
	if len(secretLine) != len(secretShapes)+len(secretLabels) {
		t.Fatalf("%d patterns, %d shapes and %d labels", len(secretLine), len(secretShapes), len(secretLabels))
	}
	for _, pattern := range slices.Concat(secretShapes, secretLabels) {
		if !slices.Contains(secretLine, pattern) {
			t.Errorf("%s isn't in secretLine", pattern)
		}
	}
	// A label is a word; a shape is a value's look, and never matches a word alone.
	for _, word := range []string{"token", "password", "secret", "Bearer x", "key:"} {
		if slices.ContainsFunc(secretShapes, func(p *regexp.Regexp) bool { return p.MatchString(word) }) {
			t.Errorf("a shape matches the word %q", word)
		}
		if !slices.ContainsFunc(secretLabels, func(p *regexp.Regexp) bool { return p.MatchString(word) }) {
			t.Errorf("no label matches %q", word)
		}
	}
}

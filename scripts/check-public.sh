#!/bin/bash
# Fails if what's committed holds something that must never be public: an
# Anthropic API key, an AWS access key ID or secret key, a name or path from the
# maintainer's own machine, the folders of notes and briefs a personal build
# keeps beside the code, or, in a line added since the release base, a name of
# the team that builds it (in a clone without that base, in any line of its code,
# pages, docs and workflows). A test that needs a key-shaped string puts it
# together when it runs ("AKIA" + "FAKE…"), so no file holds one whole.
#
#   scripts/check-public.sh
#
# Run by CI on every push and pull request, and by scripts/release.sh before
# it builds. The names it looks for are listed below only as SHA-256s, so this
# file doesn't publish them itself: every word of every committed text file,
# and every run of two and three words, lowercased, is hashed and looked up.
# To add one, append its hash:
#
#   printf '%s' 'smolcorp-public-check:<words, lowercase, one space apart>' | shasum -a 256

set -euo pipefail

cd "$(dirname "$0")/.."

git ls-files -z | python3 -I -c '
import hashlib, os, re, subprocess, sys

SALT = "smolcorp-public-check:"
FORBIDDEN = set("""
e6d8d81f84a31d71edfd5e999d98eca71aff916b9ae9f6c565f7ccb515503388
ede2ea7e57bd6c7a2aebf26392ae47ee5154d9a458678ba56cd58e83cd34e612
d7107f80909815f004dcc0fc3d2d5afe81c11cadd3de8ca219c4421e391a3773
afb6624fe1564cf9033ef2882fe0ba9f7fb39a51195d761eef41bc6a4918a3a5
10c1a3bfd02475eedfa600d3d9c4c02ca12d6d4a8cf43847e9be82dd62ebd86c
afba467e7ee0124623e88c46fb19fb685f74b58192376d0120382badf1446927
973123f3a7d1fca007f2f4489ed5a8bc51ced70effe5896ee8b3efdbec1135d5
ac2a5281a028645588b6fe84ec903bbd010bf0133504b34b6a2091517d8b313e
5ee05ea622e89460d508538a9778fb7d1598146bd5ec64f91195aeed85fee9f6
d08fdd6c8317af72d063646597525f2e3692283dd53b0ddbae838a1eb2833a6e
fb36c01f9162bfea681239be759e230044db1b96716d70199f7eef0841225988
d044bae576b96f50b7789053f1a97b5d5d486e08e56673710ba65b96b7ffb330
3482da3a05fa4781ee5d333fff10a9792a7ed6c4b0f8c8279c147d2200231943
c055bd4fc4d84f1284c8b400788c2eeeb4f5418ebc67681987640f6a3609bb97
0e5eafde6f171f96179c699df4f700e758ba7b36eb43d4978da2bfea9042b612
e290a13d9b0938cdb13f608e06d05dfa56e2aa5b65b9ceee28c17289d241227d
ac01e98442e2a52ac7f4447bd031d3ad97562aba7a6b22a85ad378f41e18843d
8d2771434ff75671e3fb3cf45057f631715badc93a3274675ec9a7449f6282e5
be749ba2d8f48610a3c3367bff24e9b260d3b8393e73a2c5b6804562e598f1bb
910c9a93fab838619c264becbef6d189e8304d541718355fb76b33466e15e7d5
51bd3863dcd03c134a31cd60c13f179934a3fc1c8c25b47a9e85cd19b62c34cb
6828a5f9092bab4d80dfb88766d987e614eca27181e8d2e7bbd943fcadeab7c0
""".split())
API_KEY = re.compile(rb"sk-ant-[A-Za-z0-9_-]{8,}")
# An AWS access key ID (AKIA… for a long-lived key, ASIA… for a temporary one), and a
# secret key: a run of 40 of its characters with capitals, small letters and digits, on
# a line that also says aws, secret, AKIA or ASIA. (A git hash is all lowercase.)
AWS_KEY_ID = re.compile(rb"(?<![A-Za-z0-9])(AKIA|ASIA)[A-Z0-9]{16}(?![A-Za-z0-9])")
AWS_SECRET = re.compile(rb"(?<![A-Za-z0-9/+])[A-Za-z0-9/+]{40}(?![A-Za-z0-9/+=])")  # KEY=… is one
AWS_NEAR = re.compile(rb"[Aa][Ww][Ss]|[Ss][Ee][Cc][Rr][Ee][Tt]|AKIA|ASIA")

def aws_secret(data):
    for line in data.split(b"\n"):
        if AWS_NEAR.search(line):
            for run in AWS_SECRET.findall(line):
                if re.search(rb"[A-Z]", run) and re.search(rb"[a-z]", run) and re.search(rb"[0-9]", run):
                    return True
    return False

KEPT_OUT = ("notes-out/", ".team/", "security/")

problems = []
all_names = [name for name in sys.stdin.buffer.read().decode("utf-8").split("\0") if name]
for name in all_names:
    if name.startswith(KEPT_OUT):
        problems.append(f"{name}: this folder is never committed")
        continue
    try:
        data = open(name, "rb").read()
    except OSError:
        continue
    if b"\0" in data[:8192]:
        continue  # an image or a sound
    if API_KEY.search(data):
        problems.append(f"{name}: what looks like an Anthropic API key")
    if AWS_KEY_ID.search(data):
        problems.append(f"{name}: what looks like an AWS access key ID")
    if aws_secret(data):
        problems.append(f"{name}: what looks like an AWS secret key")
    words = re.findall(r"[a-z0-9]+", data.decode("utf-8", "replace").lower())
    found = False
    for size in (1, 2, 3):
        for i in range(len(words) - size + 1):
            if hashlib.sha256((SALT + " ".join(words[i:i + size])).encode()).hexdigest() in FORBIDDEN:
                problems.append(f"{name}: a personal name or path (words {i + 1}-{i + size})")
                found = True
                break
        if found:
            break

# The Mac app logs nothing, so nothing it holds, as the Summary key, can end up
# in a log: no print, NSLog, os_log or Logger outside #if DEBUG.
LOGGING = re.compile(r"\b(print|NSLog|os_log|Logger)\(")
# The answer of --setup is its checklist, printed on the terminal that ran it:
# not a log, and it holds no key (Sources/AgentDeck/SetupCommand.swift, only).
TERMINAL = ("Sources/AgentDeck/SetupCommand.swift",)
for name in [n for n in all_names if n.startswith("Sources/") and n.endswith(".swift") and n not in TERMINAL]:
    # Each open #if: "debug" (only a debug build compiles what follows), "release" (only
    # another), or "other". What a #else follows flips debug and release.
    stack = []
    for number, line in enumerate(open(name, encoding="utf-8"), 1):
        code = line.split("//", 1)[0]
        if re.match(r"\s*#if\s+DEBUG\b", code):
            stack.append("debug")
        elif re.match(r"\s*#if\s+!\s*DEBUG\b", code):
            stack.append("release")
        elif re.match(r"\s*#if\b", code):
            stack.append("other")
        elif re.match(r"\s*#(else|elseif)\b", code) and stack:
            stack[-1] = {"debug": "release", "release": "debug"}.get(stack[-1], "other")
        elif re.match(r"\s*#endif\b", code) and stack:
            stack.pop()
        elif "debug" not in stack and LOGGING.search(code):
            problems.append(f"{name}:{number}: logging outside #if DEBUG")

# Nor does the Windows and Linux app log anything from Summary, which holds the key.
GO_LOGGING = re.compile(r"\b(log\.|fmt\.(Print|Fprint))")
for name in [n for n in all_names if re.match(r"desktop/summary[^/]*\.go$", n) and not n.endswith("_test.go")]:
    for number, line in enumerate(open(name, encoding="utf-8"), 1):
        if GO_LOGGING.search(line.split("//", 1)[0]):
            problems.append(f"{name}:{number}: logging in Summary")

# Nor does a line added since the release base name the team that builds Smolcorp: each name is a whole word in
# capitals (a word in small letters, or one inside a longer name, is another word), listed only as its SHA-256 with
# its own prefix, as the names above are. The base is a tree (SMOLCORP_RELEASE_BASE, or the one below: the signed
# 1.26 base), and an added line is one the diff from it to the files here adds. A clone without that tree (a shallow
# one, or a tree loaded on its own) checks every line of its .go, .html, .js, .swift, .md and .yml files instead, and
# says so. It lets through only a line the base itself had, kept or moved: scripts/check-public-base-lines.txt lists
# each one, trimmed, as the SHA-256 of its text with its own prefix (TestCheckPublicsBaseLinesAreTheReleaseBases
# checks the list, and remakes it, against the default base).
TEAM_SALT = "smolcorp-team-check:"
TEAM = set("""
1145fd2c3199bfa9ce66190ba34516d4009f3315073546c0eb25bf21b663c4c9
355350f2e61ebe0de36fb892524a7f4f8571d90edb9269a2a7e2e8deab9287c9
018eb287160bf04c8d1f88895da0225e6612135c64cfb1d345f067a1f6e80eb4
b7d10a7ebddba0209e7aa2e61313175930487c7aa27b73547e7396766ed3ff49
2f44115804f65bfe999c5a3add3d5401305a02e6eba40934155eac5d149cb69f
7e3c4b6a9c022d8ed2402029bcb4438c4585727cdf7ef1b72c9fe0c1c3cf6c08
""".split())
RELEASE_BASE = os.environ.get("SMOLCORP_RELEASE_BASE") or "cf9905c004436821c52bbb41cb47093a9f124d4f"
TEAM_CHECKED = (".go", ".html", ".js", ".swift", ".md", ".yml")
BASE_LINE_SALT = "smolcorp-team-line:"
try:
    BASE_LINES = {line.strip() for line in open("scripts/check-public-base-lines.txt", encoding="utf-8")
                  if line.strip() and not line.startswith("#")}
except OSError:
    BASE_LINES = set()

def names_team(text):
    return any(word.isupper() and word.isalpha() and hashlib.sha256((TEAM_SALT + word).encode()).hexdigest() in TEAM
               for word in re.findall(r"\w+", text, re.ASCII))

notes = []
if subprocess.run(["git", "cat-file", "-e", RELEASE_BASE + "^{tree}"], stderr=subprocess.DEVNULL).returncode != 0:
    notes.append(f"Note: the release base {RELEASE_BASE} is not in this clone, so every line of its "
                 + ", ".join(TEAM_CHECKED) + " files was checked for the team names, but for the lines the base had.")
    for name in [n for n in all_names if n.endswith(TEAM_CHECKED) and not n.startswith(KEPT_OUT)]:
        try:
            text = open(name, "rb").read().decode("utf-8", "replace")
        except OSError:
            continue
        for number, line in enumerate(text.split("\n"), 1):
            if names_team(line) and hashlib.sha256((BASE_LINE_SALT + line.strip()).encode()).hexdigest() not in BASE_LINES:
                problems.append(f"{name}:{number}: a name of the team, in a clone without the release base")
else:
    diff = subprocess.run(["git", "-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-color",
                           "--src-prefix=a/", "--dst-prefix=b/", "-M", "-U0", RELEASE_BASE, "--"],
                          stdout=subprocess.PIPE, check=True).stdout.decode("utf-8", "replace")
    name, number, hunk = "", 0, False
    for line in diff.split("\n"):
        if line.startswith("diff --git "):
            hunk = False
        elif not hunk and line.startswith("+++ "):
            name = line[6:] if line.startswith("+++ b/") else line[4:]
        elif line.startswith("@@ "):
            number, hunk = int(re.match(r"@@ -\S+ \+(\d+)", line).group(1)), True
        elif hunk and line.startswith("+"):
            if names_team(line[1:]):
                problems.append(f"{name}:{number}: a name of the team in a line added since the release base")
            number += 1

for note in notes:
    print(note)
if problems:
    print("Not for a public repository:")
    for problem in problems:
        print("  " + problem)
    sys.exit(1)
print("Nothing personal in what is committed.")
'

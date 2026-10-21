#!/bin/bash
# Fails if what's committed holds something that must never be public: an
# Anthropic API key, a name or path from the maintainer's own machine, or the
# folders of notes and briefs a personal build keeps beside the code.
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
import hashlib, re, sys

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
""".split())
API_KEY = re.compile(rb"sk-ant-[A-Za-z0-9_-]{8,}")
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
for name in [n for n in all_names if n.startswith("Sources/") and n.endswith(".swift")]:
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

if problems:
    print("Not for a public repository:")
    for problem in problems:
        print("  " + problem)
    sys.exit(1)
print("Nothing personal in what is committed.")
'

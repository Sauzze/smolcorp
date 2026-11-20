# Writes the Go tables for the Mac's name fold (policy_fold_tables.go; run from internal/sandbox:
# python3 -I testdata/foldtables.py > policy_fold_tables.go) from the Unicode Character Database files pinned in
# testdata/ucd-17.0.0, the Unicode this Mac's volume compares names by (RT126POLICYFOLD measured it: the volume takes
# exactly the code points Unicode 17.0 assigns). Each file must be the one pinned below, by its SHA-256, and say its
# version, so another Unicode can't write the tables without a word. Python's own unicodedata isn't used.
#
# The files come from the Unicode Consortium's own, as published at https://www.unicode.org/Public/17.0.0/ucd/
# (fetched 17 Nov 2026 for FXPOLICYFOLDR1): UnicodeData.txt, CaseFolding.txt, CompositionExclusions.txt and
# DerivedNormalizationProps.txt, each pinned below by its SHA-256 as published (OFFICIAL). What is kept here is only
# the data this reads, without the characters' names and the comments (check-public.sh reads every word of a committed
# file): `python3 -I testdata/foldtables.py --strip <folder of the published files>` checks the published files and
# writes testdata/ucd-17.0.0 again, byte for byte.
#
# The tables: every code point's full canonical decomposition (Hangul's syllables left out: Unicode decomposes them by
# rule), every canonical combining class that isn't 0, as ranges, every code point's full case folding (CaseFolding's
# C and F rows), itself decomposed, for the code points a decomposition can give, every primary composite, and the
# code points Unicode 17.0 assigns, as ranges. Checks what the Go code takes for granted.
import hashlib, os, sys

VERSION = "17.0.0"
HERE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "ucd-" + VERSION)
OFFICIAL = {
    "UnicodeData.txt": "2e1efc1dcb59c575eedf5ccae60f95229f706ee6d031835247d843c11d96470c",
    "CaseFolding.txt": "ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183",
    "CompositionExclusions.txt": "2f239196ef3b5b61db5cc476e9bd80f534d15aa1b74e1be1dea5d042a344c85f",
    "DerivedNormalizationProps.txt": "71fd6a206a2c0cdd41feb6b7f656aa31091db45e9cedc926985d718397f9e488",
}
PINNED = {
    "UnicodeData.txt": "06c83a21c3a680c714f3407b5e24740b3082e30198ad5fd16541b0df48ce56e4",
    "CaseFolding.txt": "b1742f7451cc4993c74293d1fa9f988c78ce5ed02f9a7a591b919291eb5ce5ba",
    "CompositionExclusions.txt": "07f66bd23b781759962753c60b4bfe968c7ea4845952e7f6c0499f5201dcd7e7",
    "DerivedNormalizationProps.txt": "31271f32f448a4398a73627838b77339993a13d32c66f523e23f852ff4f57161",
}
FORBIDDEN = set("/*?[\\\x00|")
SBASE, LBASE, VBASE, TBASE, LCOUNT, VCOUNT, TCOUNT = 0xAC00, 0x1100, 0x1161, 0x11A7, 19, 21, 28
HANGUL = range(SBASE, SBASE + LCOUNT * VCOUNT * TCOUNT)


def rows(text):
    for line in text.split("\n"):
        line = line.split("#", 1)[0].strip()
        if line:
            yield [f.strip() for f in line.split(";")]


def strip(folder):
    # The published files, checked, cut down to what this reads: UnicodeData.txt's code point, range mark, category,
    # combining class and canonical (untagged) decomposition; CaseFolding.txt's and CompositionExclusions.txt's rows;
    # DerivedNormalizationProps.txt's Full_Composition_Exclusion rows.
    for name, want in OFFICIAL.items():
        data = open(os.path.join(folder, name), "rb").read()
        got = hashlib.sha256(data).hexdigest()
        assert got == want, "%s isn't the published Unicode %s file: sha256 %s" % (name, VERSION, got)
        text = data.decode("utf-8")
        if name != "UnicodeData.txt":  # which has no header line; its hash pins it
            first = text.split("\n", 1)[0]
            assert first == "# %s-%s.txt" % (name[:-4], VERSION), "%s says %r" % (name, first)
        out = ["# %s-%s.txt" % (name[:-4], VERSION),
               "# Only the data testdata/foldtables.py reads, by its --strip, from the published file (SHA-256 %s)." % want]
        for f in rows(text):
            if name == "UnicodeData.txt":
                mark = "<First>" if f[1].endswith(", First>") else "<Last>" if f[1].endswith(", Last>") else ""
                out.append(";".join([f[0], mark, f[2], f[3], "", "" if f[5].startswith("<") else f[5]]))
            elif name == "CaseFolding.txt":
                out.append("; ".join(f[:3]) + ";")
            elif name == "CompositionExclusions.txt":
                out.append(f[0])
            elif f[1] == "Full_Composition_Exclusion":
                out.append("; ".join(f[:2]))
        os.makedirs(HERE, exist_ok=True)
        open(os.path.join(HERE, name), "w", encoding="utf-8", newline="\n").write("\n".join(out) + "\n")


if sys.argv[1:2] == ["--strip"]:
    strip(sys.argv[2])
    sys.exit(0)


def read(name):
    data = open(os.path.join(HERE, name), "rb").read()
    got = hashlib.sha256(data).hexdigest()
    assert got == PINNED[name], "%s isn't the pinned Unicode %s data: sha256 %s" % (name, VERSION, got)
    text = data.decode("utf-8")
    first = text.split("\n", 1)[0]
    assert first == "# %s-%s.txt" % (name[:-4], VERSION), "%s says %r" % (name, first)
    return text


def cps(field):
    return [int(x, 16) for x in field.split()]


def span(field):
    a, _, b = field.partition("..")
    return range(int(a, 16), int(b or a, 16) + 1)


# UnicodeData.txt: each assigned code point (a range's first and last name it once), its category, its canonical
# combining class and its canonical decomposition mapping (one level; a <tagged> one is a compatibility mapping).
category, ccc, mapping = {}, {}, {}
first = None
for f in rows(read("UnicodeData.txt")):
    cp = int(f[0], 16)
    if f[1] == "<First>":
        first = cp
        continue
    for c in range(first, cp + 1) if f[1] == "<Last>" else [cp]:
        category[c] = f[2]
        if int(f[3]):
            ccc[c] = int(f[3])
        if f[5] and not f[5].startswith("<"):
            mapping[c] = cps(f[5])
    first = None
ASSIGNED = sorted(category)

# CaseFolding.txt: the full case folding is the C and F rows (S is the simple one, T Turkic only).
casefold = {}
for f in rows(read("CaseFolding.txt")):
    if f[1] in ("C", "F"):
        assert int(f[0], 16) not in casefold, f
        casefold[int(f[0], 16)] = cps(f[2])

# The full composition exclusions: CompositionExclusions.txt's, the singletons, and the non-starter decompositions;
# DerivedNormalizationProps.txt's Full_Composition_Exclusion must say the same.
excluded = {c for f in rows(read("CompositionExclusions.txt")) for c in span(f[0])}
excluded |= {c for c, m in mapping.items() if len(m) == 1 or ccc.get(c) or ccc.get(m[0])}
derived = {c for f in rows(read("DerivedNormalizationProps.txt")) if f[1] == "Full_Composition_Exclusion" for c in span(f[0])}
assert excluded == derived, sorted(excluded ^ derived)[:20]


def decompose(c):
    if c in HANGUL:
        si = c - SBASE
        out = [LBASE + si // (VCOUNT * TCOUNT), VBASE + si % (VCOUNT * TCOUNT) // TCOUNT]
        return out + [TBASE + si % TCOUNT] if si % TCOUNT else out
    if c in mapping:
        return [x for m in mapping[c] for x in decompose(m)]
    return [c]


def nfd(s):
    out = [x for c in s for x in decompose(c)]
    for i in range(1, len(out)):  # canonical order: each run of marks sorted by class, stably
        j = i
        while j > 0 and ccc.get(out[j], 0) and ccc.get(out[j - 1], 0) > ccc.get(out[j], 0):
            out[j - 1], out[j] = out[j], out[j - 1]
            j -= 1
    return out


def esc(cps):
    out = []
    for cp in cps:
        if 0x20 < cp < 0x7F and chr(cp) not in '"\\' or cp >= 0xA0 and category.get(cp, "Cn")[0] in "LNPS":
            out.append(chr(cp))
        elif cp <= 0xFFFF:
            out.append("\\u%04x" % cp)
        else:
            out.append("\\U%08x" % cp)
    return "".join(out)


def wrap(tokens):
    lines, line = [], ""
    for t in tokens:
        if len(line) + len(t) > 116:
            lines.append(line)
            line = ""
        line += t
    lines.append(line)
    return " +\n\t".join('"%s"' % l for l in lines)


def records(pairs):
    # One record per code point: the code point, what it maps to, then "|". Lines of about 110 bytes.
    for cp, to in pairs:
        assert not (FORBIDDEN & {chr(x) for x in [cp] + to}), hex(cp)
    return wrap([esc([cp] + to) + "|" for cp, to in pairs])


def ranges(points):
    out = []
    for cp, v in points:
        if out and out[-1][1] == cp - 1 and out[-1][2] == v:
            out[-1][1] = cp
        else:
            out.append([cp, cp, v])
    return out


decomp = [(cp, decompose(cp)) for cp in ASSIGNED if cp not in HANGUL and decompose(cp) != [cp]]
cccs = ranges((cp, ccc[cp]) for cp in sorted(ccc))
# What a decomposition can give: a code point that NFD leaves as it is.
folds = []
for cp in ASSIGNED:
    if decompose(cp) != [cp]:
        continue
    f = nfd([x for c in [cp] for x in casefold.get(c, [c])])
    if f != [cp]:
        folds.append((cp, f))
comps = [(mapping[cp][0], mapping[cp][1], cp) for cp in sorted(mapping) if len(mapping[cp]) == 2 and cp not in excluded]
# Surrogates can't be in a Go string, nor in a name.
assigned = ranges((cp, 0) for cp in ASSIGNED if category[cp] != "Cs")

# What the Go code takes for granted: a fold of a decomposed code point is decomposed already (so NFD of it is it),
# a full decomposition is in canonical order, no fold or decomposition makes a Hangul syllable or names a code point
# Unicode doesn't assign, and every composite's letter and mark are themselves decomposed or composites.
for cp, f in folds:
    assert nfd(f) == f, hex(cp)
for cp, d in decomp:
    assert nfd(d) == d, hex(cp)
for cp, to in folds + decomp:
    assert all(x not in HANGUL and x in category for x in to), hex(cp)

out = sys.stdout
out.write("""package sandbox

// Generated from the Unicode %s Character Database (testdata/ucd-%s, pinned by SHA-256) by testdata/foldtables.py;
// checked against the Mac's volume, name by name, by TestTheMacsFoldIsUnicodesCanonicalCaselessMatch and
// TestTheMacsVolumeKnowsNoCodePointTheFoldDoesnt. Don't edit by hand.

// foldUnicode is the Unicode version the tables are from: the one this Mac's volume compares names by.
const foldUnicode = "%s"

// decompositionTable is every code point's full canonical decomposition (NFD) where it isn't the code point itself,
// Hangul's syllables left out: the code point, what it decomposes to, then "|".
const decompositionTable = "" +
\t%s

// combiningTable is every canonical combining class that isn't 0, as ranges: the first code point, the last, the class.
const combiningTable = "" +
\t%s

// caseFoldTable is Unicode's full case folding (CaseFolding.txt's C and F rows) of every code point NFD leaves as it
// is, where it isn't the code point itself, decomposed: the code point, its fold, then "|".
const caseFoldTable = "" +
\t%s

// compositionTable is every canonical primary composite but Hangul's, as triples: the letter, the mark, what they
// compose to. Each code point whose decomposition is two code points with no tag, unless Unicode excludes it from
// composition (DerivedNormalizationProps.txt's Full_Composition_Exclusion).
const compositionTable = "" +
\t%s

// assignedTable is every code point Unicode assigns (any category but Cn, surrogates left out), as ranges: the first
// code point, the last. The Mac's volume refuses a name with any other (EILSEQ), as far as its own Unicode goes.
const assignedTable = "" +
\t%s
""" % (VERSION, VERSION, VERSION, records(decomp), wrap([esc([a, b]) + "\\u%04x" % c for a, b, c in cccs]), records(folds),
       wrap([esc([a, b, c]) for a, b, c in comps]),
       wrap(["".join("\\u%04x" % x if x <= 0xFFFF else "\\U%08x" % x for x in (a, b)) for a, b, _ in assigned])))
print("Unicode %s: decompositions %d, combining ranges %d, folds %d, compositions %d, assigned ranges %d" % (
    VERSION, len(decomp), len(cccs), len(folds), len(comps), len(assigned)), file=sys.stderr)

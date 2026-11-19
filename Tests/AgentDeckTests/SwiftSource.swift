import Foundation

/// A Swift file as the spawn and AppleScript checks read it (SpawnSitesTests, AppleScriptFixedTests): its code, with
/// every comment and the text of every string literal blanked (spaces; newlines kept, so offsets and line numbers
/// stay), its string literals and what each one interpolates, and the functions and closure-valued properties that
/// hold the code. It knows Swift's comments (nested too) and strings (raw, multi-line, interpolated), not macros or
/// regex literals: it reads honest code, not code written to slip past it.
struct SwiftSource {
    struct Literal {
        let range: Range<Int>
        /// The code of each `\(…)`.
        let interpolations: [Range<Int>]
    }

    struct Declaration {
        let name: String
        let body: Range<Int>
    }

    let name: String
    let bytes: [UInt8]
    private(set) var code: [UInt8]
    private(set) var literals: [Literal] = []
    private(set) var declarations: [Declaration] = []

    init(name: String, text: String) {
        self.name = name
        bytes = Array(text.utf8)
        code = bytes
        var i = 0
        _ = scanCode(&i, untilParen: false)
        // Only code is left, and it's ASCII from here: a regex's UTF-16 offsets are byte offsets.
        for k in code.indices where code[k] >= 0x80 { code[k] = 0x20 }
        declarations = findDeclarations()
    }

    /// The code as text, for regexes.
    var codeText: String { String(decoding: code, as: UTF8.self) }

    /// The source's own text from `range`.
    func text(_ range: Range<Int>) -> String { String(decoding: bytes[range], as: UTF8.self) }

    func line(of offset: Int) -> Int { bytes[..<offset].reduce(1) { $1 == 0x0A ? $0 + 1 : $0 } }

    /// The innermost function or closure-valued property whose body holds `offset`, "" at the top level.
    func enclosing(_ offset: Int) -> String {
        declarations.filter { $0.body.contains(offset) }.min { $0.body.count < $1.body.count }?.name ?? ""
    }

    /// Each match of `pattern` in the code: its range and capture groups (as the source's own text).
    func matches(_ pattern: String) -> [(range: Range<Int>, groups: [String])] {
        let regex = try! NSRegularExpression(pattern: pattern)
        let text = codeText
        return regex.matches(in: text, range: NSRange(location: 0, length: (text as NSString).length)).map { match in
            let groups = (1..<max(match.numberOfRanges, 1)).map { n -> String in
                let r = match.range(at: n)
                return r.location == NSNotFound ? "" : self.text(r.location..<(r.location + r.length))
            }
            return (match.range.location..<(match.range.location + match.range.length), groups)
        }
    }

    /// The arguments of the call whose "(" is at `open`, each as the source's own text, trimmed.
    func arguments(at open: Int) -> [String] {
        var depth = 0, start = open + 1, found: [String] = []
        for k in open..<code.count {
            switch code[k] {
            case UInt8(ascii: "("), UInt8(ascii: "["), UInt8(ascii: "{"): depth += 1
            case UInt8(ascii: ")"), UInt8(ascii: "]"), UInt8(ascii: "}"):
                depth -= 1
                if depth == 0 {
                    found.append(text(start..<k).trimmingCharacters(in: .whitespacesAndNewlines))
                    return found.filter { !$0.isEmpty }
                }
            case UInt8(ascii: ",") where depth == 1:
                found.append(text(start..<k).trimmingCharacters(in: .whitespacesAndNewlines))
                start = k + 1
            default: break
            }
        }
        return found
    }

    // MARK: Reading

    private func at(_ i: Int, _ text: String) -> Bool {
        let wanted = Array(text.utf8)
        return i + wanted.count <= bytes.count && Array(bytes[i..<(i + wanted.count)]) == wanted
    }

    private mutating func blank(_ range: Range<Int>) {
        for k in range where k < code.count && bytes[k] != 0x0A { code[k] = 0x20 }
    }

    /// Code from `i`, to the end, or to the ")" closing it (`untilParen`, for an interpolation), where `i` is left.
    private mutating func scanCode(_ i: inout Int, untilParen: Bool) -> Bool {
        var depth = 0
        while i < bytes.count {
            if at(i, "//") {
                let start = i
                while i < bytes.count && bytes[i] != 0x0A { i += 1 }
                blank(start..<i)
                continue
            }
            if at(i, "/*") {
                let start = i
                var nested = 0
                repeat {
                    if at(i, "/*") { nested += 1; i += 2 } else if at(i, "*/") { nested -= 1; i += 2 } else { i += 1 }
                } while nested > 0 && i < bytes.count
                blank(start..<i)
                continue
            }
            var hashes = 0
            while at(i + hashes, "#") { hashes += 1 }
            if at(i + hashes, "\"") {
                scanString(&i, hashes: hashes)
                continue
            }
            if untilParen {
                if bytes[i] == UInt8(ascii: "(") { depth += 1 }
                if bytes[i] == UInt8(ascii: ")") {
                    if depth == 0 { return true }
                    depth -= 1
                }
            }
            i += max(hashes, 1)
        }
        return false
    }

    /// The string literal starting at `i` (its `hashes` first), blanked but for what it interpolates.
    private mutating func scanString(_ i: inout Int, hashes: Int) {
        let start = i
        i += hashes
        let multiline = at(i, "\"\"\"")
        i += multiline ? 3 : 1
        blank(start..<i)
        let close = (multiline ? "\"\"\"" : "\"") + String(repeating: "#", count: hashes)
        let escape = "\\" + String(repeating: "#", count: hashes)
        var interpolations: [Range<Int>] = []
        while i < bytes.count {
            if at(i, close) {
                blank(i..<(i + close.utf8.count))
                i += close.utf8.count
                break
            }
            if at(i, escape) {
                let after = i + escape.utf8.count
                if at(after, "(") {
                    blank(i..<(after + 1))
                    var j = after + 1
                    let codeStart = j
                    _ = scanCode(&j, untilParen: true)
                    interpolations.append(codeStart..<j)
                    blank(j..<(j + 1))
                    i = j + 1
                } else {
                    blank(i..<(after + 1))
                    i = after + 1
                }
                continue
            }
            if !multiline && bytes[i] == 0x0A { break } // an unclosed string: leave it at the line's end
            blank(i..<(i + 1))
            i += 1
        }
        literals.append(Literal(range: start..<i, interpolations: interpolations))
    }

    /// Functions (`func name(…) {`, `init(…) {`) and properties whose value or getter is a closure
    /// (`let name: T = {`, `var name: T {`), with their bodies.
    private func findDeclarations() -> [Declaration] {
        var found: [Declaration] = []
        for match in matches(#"\b(?:func\s+([A-Za-z_]\w*)|(init))\s*[?!]?\s*(?:<[^>{]*>)?\s*\("#) {
            let open = match.range.upperBound - 1
            guard let close = matching(from: open, "(", ")"), let brace = firstBrace(after: close + 1),
                  let end = matching(from: brace, "{", "}") else { continue }
            found.append(Declaration(name: match.groups[0].isEmpty ? match.groups[1] : match.groups[0], body: brace..<(end + 1)))
        }
        // A type or "=" is needed: `if let saved {` is no declaration.
        for match in matches(#"\b(?:let|var)\s+([A-Za-z_]\w*)\s*(?::[^=\n{]*\{|(?::[^=\n{]*)?=\s*\{)"#) {
            let brace = match.range.upperBound - 1
            guard let end = matching(from: brace, "{", "}") else { continue }
            found.append(Declaration(name: match.groups[0], body: brace..<(end + 1)))
        }
        return found
    }

    /// The "{" opening a function's body, after its parameters: none if another declaration comes first (a
    /// protocol's requirement has no body).
    private func firstBrace(after start: Int) -> Int? {
        var k = start
        while k < code.count {
            if code[k] == UInt8(ascii: "{") { return k }
            if code[k] == UInt8(ascii: "}") { return nil }
            if code[k] == 0x0A {
                let rest = String(decoding: code[(k + 1)..<min(code.count, k + 80)], as: UTF8.self).trimmingCharacters(in: .whitespaces)
                if ["func ", "var ", "let ", "static ", "private ", "case ", "init", "@", "}"].contains(where: rest.hasPrefix) { return nil }
            }
            k += 1
        }
        return nil
    }

    private func matching(from open: Int, _ opening: Character, _ closing: Character) -> Int? {
        let o = UInt8(ascii: opening.unicodeScalars.first!), c = UInt8(ascii: closing.unicodeScalars.first!)
        var depth = 0
        for k in open..<code.count {
            if code[k] == o { depth += 1 }
            if code[k] == c {
                depth -= 1
                if depth == 0 { return k }
            }
        }
        return nil
    }
}

extension SwiftSource {
    /// The app's own Swift files (Sources/AgentDeck), as read here.
    static func app() throws -> [SwiftSource] {
        let folder = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../../Sources/AgentDeck").standardized
        return try FileManager.default.contentsOfDirectory(atPath: folder.path).filter { $0.hasSuffix(".swift") }.sorted().map {
            SwiftSource(name: $0, text: try String(contentsOf: folder.appendingPathComponent($0), encoding: .utf8))
        }
    }
}

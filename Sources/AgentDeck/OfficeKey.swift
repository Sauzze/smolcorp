import AppKit
import Carbon.HIToolbox

/// The keys that show and hide the office's window from anywhere: ⌥⌘O unless
/// you choose others in Settings, or none. Kept in config.json as the setup
/// page records them, "alt+cmd+O": the modifiers in the Mac's order (⌃⌥⇧⌘),
/// then the key, named by where it sits on a US keyboard, as Carbon's key
/// codes are, so they keep working if the layout changes; "off" for none, and
/// no entry for the default. They're shown as the layout in use prints them
/// (`Labels`: ⌥⌘A on a French AZERTY Mac for the key a US one has Q on), and
/// the Mac's own keys are refused by what the key types there. The
/// Linux/Windows app checks them the same way (desktop/officekey.go), and both
/// engines' tests read Tests/fixtures/officekey.json.
struct OfficeKey: Equatable {
    /// Each key's name as the layout in use prints it, by its US name, for those that differ: ["Q": "A", "A": "Q",
    /// …] on a French AZERTY Mac, none on a US one.
    typealias Labels = [String: String]

    /// ctrl, alt, shift and cmd, each once, in that order.
    let modifiers: [String]
    let key: String

    static let standard = OfficeKey(modifiers: ["alt", "cmd"], key: "O")
    static let off = "off"
    static let modifierOrder = ["ctrl", "alt", "shift", "cmd"]

    /// The keys Smolcorp can hold, by name, with their Carbon key codes.
    static let codes: [String: Int] = [
        "A": kVK_ANSI_A, "B": kVK_ANSI_B, "C": kVK_ANSI_C, "D": kVK_ANSI_D, "E": kVK_ANSI_E, "F": kVK_ANSI_F,
        "G": kVK_ANSI_G, "H": kVK_ANSI_H, "I": kVK_ANSI_I, "J": kVK_ANSI_J, "K": kVK_ANSI_K, "L": kVK_ANSI_L,
        "M": kVK_ANSI_M, "N": kVK_ANSI_N, "O": kVK_ANSI_O, "P": kVK_ANSI_P, "Q": kVK_ANSI_Q, "R": kVK_ANSI_R,
        "S": kVK_ANSI_S, "T": kVK_ANSI_T, "U": kVK_ANSI_U, "V": kVK_ANSI_V, "W": kVK_ANSI_W, "X": kVK_ANSI_X,
        "Y": kVK_ANSI_Y, "Z": kVK_ANSI_Z,
        "0": kVK_ANSI_0, "1": kVK_ANSI_1, "2": kVK_ANSI_2, "3": kVK_ANSI_3, "4": kVK_ANSI_4,
        "5": kVK_ANSI_5, "6": kVK_ANSI_6, "7": kVK_ANSI_7, "8": kVK_ANSI_8, "9": kVK_ANSI_9,
        "F1": kVK_F1, "F2": kVK_F2, "F3": kVK_F3, "F4": kVK_F4, "F5": kVK_F5, "F6": kVK_F6,
        "F7": kVK_F7, "F8": kVK_F8, "F9": kVK_F9, "F10": kVK_F10, "F11": kVK_F11, "F12": kVK_F12,
        "Space": kVK_Space, "Tab": kVK_Tab,
        ",": kVK_ANSI_Comma, ".": kVK_ANSI_Period, "/": kVK_ANSI_Slash, ";": kVK_ANSI_Semicolon, "'": kVK_ANSI_Quote,
        "[": kVK_ANSI_LeftBracket, "]": kVK_ANSI_RightBracket, "-": kVK_ANSI_Minus, "=": kVK_ANSI_Equal,
        "\\": kVK_ANSI_Backslash, "`": kVK_ANSI_Grave,
    ]

    /// The Mac's own keys in every app, never taken from it, by what they type: the system's, then the editing and
    /// file keys every app has, then Log Out and Lock Screen.
    static let reserved = ["cmd+Tab", "cmd+Space", "cmd+Q", "cmd+W", "cmd+H", "cmd+M", "cmd+,",
                           "cmd+C", "cmd+V", "cmd+X", "cmd+Z", "shift+cmd+Z", "cmd+A", "cmd+S", "cmd+F", "cmd+T", "cmd+N",
                           "cmd+P", "cmd+O", "shift+cmd+Q", "ctrl+cmd+Q"]

    /// As config.json and the page write it.
    var text: String { (modifiers + [key]).joined(separator: "+") }

    /// As the Mac's menus show it on a US layout: ⌥⌘O.
    var symbols: String { symbols([:]) }

    /// As the Mac's menus show it on the layout in use: ⌥⌘A for "alt+cmd+Q" on AZERTY.
    func symbols(_ labels: Labels) -> String {
        let marks = ["ctrl": "⌃", "alt": "⌥", "shift": "⇧", "cmd": "⌘"]
        return modifiers.compactMap { marks[$0] }.joined() + (labels[key] ?? key)
    }

    /// The keys as apps read them on the layout in use, which is how macOS matches its menus' keys: the key named
    /// by what it types there when that's a key Smolcorp knows by name, else by where it sits (a layout that types
    /// another script gives the US keys to ⌘, as macOS does).
    func typed(_ labels: Labels) -> OfficeKey {
        guard let label = labels[key], OfficeKey.codes[label] != nil else { return self }
        return OfficeKey(modifiers: modifiers, key: label)
    }

    static let shapeRefusal = "That isn't a shortcut Smolcorp can use: a letter, a digit, F1 to F12, Space, Tab or a punctuation key, with ⌘, ⌥, ⌃ or ⇧."
    static let modifierRefusal = "Hold ⌘, ⌥ or ⌃ too: a key alone, or with ⇧ only, would stop it typing anywhere."
    static func reservedRefusal(_ key: OfficeKey, _ labels: Labels = [:]) -> String {
        "\(key.symbols(labels)) is the Mac's own, in every app: choose another."
    }
    /// What Carbon can say: the keys are in macOS's own shortcuts, switched on, or it wouldn't hold them. Another
    /// app's menu keys can't be seen, so they're never claimed.
    static func takenRefusal(_ key: OfficeKey, _ labels: Labels = [:]) -> String {
        "\(key.symbols(labels)) is taken: macOS has it switched on, or won't let Smolcorp hold it. Choose another."
    }

    /// Keys as written, in any order of modifiers, or nil.
    static func parse(_ text: String) -> OfficeKey? {
        var parts = text.components(separatedBy: "+")
        guard parts.count >= 1, let key = parts.popLast(), codes[key] != nil, Set(parts).count == parts.count,
              parts.allSatisfy(modifierOrder.contains) else { return nil }
        return OfficeKey(modifiers: modifierOrder.filter(parts.contains), key: key)
    }

    /// What the setup page asks for: "default" for ⌥⌘O, "off" for none (nil), or keys, checked on the layout in
    /// use; or why not.
    static func choice(_ text: String, labels: Labels = [:]) -> Result<OfficeKey?, Refusal> {
        if text == "default" { return .success(standard) }
        if text == off { return .success(nil) }
        guard let key = parse(text) else { return .failure(Refusal(shapeRefusal)) }
        guard !Set(key.modifiers).isDisjoint(with: ["ctrl", "alt", "cmd"]) else { return .failure(Refusal(modifierRefusal)) }
        guard !reserved.contains(key.typed(labels).text) else { return .failure(Refusal(reservedRefusal(key, labels))) }
        return .success(key)
    }

    struct Refusal: Error, Equatable {
        let message: String
        init(_ message: String) { self.message = message }
    }

    /// The keys a config holds: the default when it has none, nil when they're off. A hand-edited entry that
    /// isn't fit to hold counts as none set, so the office keeps a key. Checked on the layout in use, as the page's
    /// ask was: AZERTY's ⌘; ("cmd+,") stays, and keys that are the Mac's own on it don't.
    static func chosen(_ config: Config, labels: Labels = [:]) -> OfficeKey? {
        guard let text = config.officeKey else { return standard }
        guard case .success(let key) = choice(text, labels: labels) else { return standard }
        return key
    }

    /// What config.json keeps for `key`: no entry for the default, "off" for none.
    static func saved(_ key: OfficeKey?) -> String? {
        guard let key else { return off }
        return key == standard ? nil : key.text
    }

    // MARK: On this Mac

    var keyCode: UInt32 { UInt32(OfficeKey.codes[key] ?? kVK_ANSI_O) }

    var carbonModifiers: UInt32 {
        let flags = ["ctrl": controlKey, "alt": optionKey, "shift": shiftKey, "cmd": cmdKey]
        return UInt32(modifiers.reduce(0) { $0 | (flags[$1] ?? 0) })
    }

    /// For a menu item on a US layout: its key equivalent and modifiers.
    var menuKey: (key: String, modifiers: NSEvent.ModifierFlags) { menuKey([:]) }

    /// For a menu item on the layout in use: the key as printed there, which is what macOS matches a menu's key by.
    func menuKey(_ labels: Labels) -> (key: String, modifiers: NSEvent.ModifierFlags) {
        let flags: [String: NSEvent.ModifierFlags] = ["ctrl": .control, "alt": .option, "shift": .shift, "cmd": .command]
        let mask = modifiers.reduce(into: NSEvent.ModifierFlags()) { $0.insert(flags[$1] ?? []) }
        if key.count > 1, key.hasPrefix("F"), let number = Int(key.dropFirst()) {
            return (String(UnicodeScalar(UInt16(NSF1FunctionKey + number - 1)).map(Character.init) ?? " "), mask)
        }
        let named = ["Space": " ", "Tab": "\t"]
        return (named[key] ?? (labels[key] ?? key).lowercased(), mask)
    }

    /// Each key's name as a layout prints it, from what `typing` says it types there without and with ⇧: a key
    /// that gives a digit only with ⇧ (AZERTY's top row: é and 2) by its digit, as ⌘ reads it; a letter as a
    /// capital, and any other sign as it is. Only the names that differ from the US ones; a key that types
    /// nothing keeps its US name.
    static func labels(typing: (_ code: Int, _ shift: Bool) -> String?) -> Labels {
        let digit = { (text: String) in text.count == 1 && text.unicodeScalars.allSatisfy(CharacterSet.decimalDigits.contains) }
        var labels: Labels = [:]
        for (name, code) in codes where name.count == 1 {
            guard let plain = typing(code, false), plain.count == 1, let scalar = plain.unicodeScalars.first,
                  !CharacterSet.controlCharacters.union(.whitespaces).contains(scalar) else { continue }
            var label = plain.uppercased()
            if !digit(plain), let shifted = typing(code, true), digit(shifted) { label = shifted }
            if label != name { labels[name] = label }
        }
        return labels
    }

    /// The layout in use on this Mac, read with Text Input Sources and `UCKeyTranslate`: read only, no permission
    /// asked. On the main queue. None when it can't be read (the US names are shown then).
    static func currentLabels() -> Labels {
        guard let source = TISCopyCurrentKeyboardLayoutInputSource()?.takeRetainedValue(),
              let property = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData) else { return [:] }
        let data = Unmanaged<CFData>.fromOpaque(property).takeUnretainedValue() as Data
        return data.withUnsafeBytes { raw -> Labels in
            guard let layout = raw.baseAddress?.assumingMemoryBound(to: UCKeyboardLayout.self) else { return [:] }
            return labels { code, shift in
                var dead: UInt32 = 0
                var length = 0
                var chars = [UniChar](repeating: 0, count: 4)
                let state = shift ? UInt32(shiftKey >> 8) & 0xFF : 0
                let status = UCKeyTranslate(layout, UInt16(code), UInt16(kUCKeyActionDisplay), state, UInt32(LMGetKbdType()),
                                            OptionBits(kUCKeyTranslateNoDeadKeysMask), &dead, chars.count, &length, &chars)
                return status == noErr && length > 0 ? String(utf16CodeUnits: chars, count: length) : nil
            }
        }
    }

    /// Whether macOS's own shortcuts (Keyboard Shortcuts in System Settings) have these keys switched on, from
    /// the list `CopySymbolicHotKeys` gives: read only, no permission asked.
    static func systemUses(_ key: OfficeKey, among list: [[String: Any]]) -> Bool {
        let mask = UInt32(controlKey | optionKey | shiftKey | cmdKey)
        return list.contains { item in
            guard item["kHISymbolicHotKeyEnabled"] as? Bool == true,
                  let code = (item["kHISymbolicHotKeyCode"] as? NSNumber)?.uint32Value,
                  let modifiers = (item["kHISymbolicHotKeyModifiers"] as? NSNumber)?.uint32Value else { return false }
            return code == key.keyCode && modifiers & mask == key.carbonModifiers
        }
    }

    static func systemShortcuts() -> [[String: Any]] {
        var list: Unmanaged<CFArray>?
        guard CopySymbolicHotKeys(&list) == noErr, let array = list?.takeRetainedValue() as? [[String: Any]] else { return [] }
        return array
    }
}

/// The office's keys, held from anywhere and changed at once from Settings, no restart. `register` holds keys
/// until what it gives back is let go, or gives nil when they're taken: Carbon's hot keys in the app (`HotKey`,
/// which needs no permission), a stand-in in the tests, which never hold a real key. A test copy holds none at
/// all (`testCopy`): a hidden copy taking ⌥⌘O would toggle its own office onto the owner's screen. Its choice is
/// still saved by the callers, and `held` says it isn't held.
final class OfficeKeyHolder {
    typealias Register = (_ key: OfficeKey, _ action: @escaping () -> Void) -> AnyObject?
    private let register: Register
    /// The layout in use, to name the keys in what it says.
    private let labels: () -> OfficeKey.Labels
    private let action: () -> Void
    private let testCopy: Bool
    private var held: AnyObject?
    /// The keys held now: nil when off, or when the keys asked for couldn't be held.
    private(set) var current: OfficeKey?

    init(register: @escaping Register = OfficeKeyHolder.carbon, labels: @escaping () -> OfficeKey.Labels = OfficeKey.currentLabels,
         testCopy: Bool = TestCopy.isOn, action: @escaping () -> Void) {
        self.register = register
        self.testCopy = testCopy
        self.labels = labels
        self.action = action
    }

    /// Holds `key` instead (nil: none). Nil when done; else why not, and the keys held before are held again, or
    /// said lost too when they can't be. In a test copy: nothing is held, and that's done, not a refusal.
    @discardableResult
    func hold(_ key: OfficeKey?) -> String? {
        guard !testCopy else { return nil }
        let before = current
        // Let go first: the same keys asked for again are held afresh, not refused as taken.
        held = nil
        current = nil
        guard let key else { return nil }
        if let holding = register(key, action) {
            held = holding
            current = key
            return nil
        }
        let labels = labels()
        let refusal = OfficeKey.takenRefusal(key, labels)
        guard let before else { return refusal }
        guard let holding = register(before, action) else {
            return refusal + " \(before.symbols(labels)) couldn't be held again either, so the office has no shortcut now."
        }
        held = holding
        current = before
        return refusal
    }

    /// Whether the keys chosen are held now, for setup. A test copy holds none, even with the keys off.
    static func isHeld(_ holding: OfficeKey?, chosen: OfficeKey?, testCopy: Bool = TestCopy.isOn) -> Bool {
        !testCopy && holding == chosen
    }

    /// Carbon's hot key, unless macOS's own shortcuts have these keys on.
    static func carbon(_ key: OfficeKey, _ action: @escaping () -> Void) -> AnyObject? {
        guard !OfficeKey.systemUses(key, among: OfficeKey.systemShortcuts()) else { return nil }
        return HotKey(keyCode: key.keyCode, modifiers: key.carbonModifiers, id: 1, action: action)
    }
}

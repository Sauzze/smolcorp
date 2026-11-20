import Carbon.HIToolbox
import JavaScriptCore
import XCTest
@testable import AgentDeck

/// The keys that show and hide the office from anywhere (owner, 11 Nov): checked as the shared fixture says
/// (desktop/officekey_test.go reads it too), recorded by the setup page's own script, held and changed at once by
/// a stand-in registrar (no test ever holds a real key), and set only from the page.
final class OfficeKeyTests: XCTestCase {
    private static let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func fixture() throws -> [String: Any] {
        let data = try Data(contentsOf: Self.root.appendingPathComponent("Tests/fixtures/officekey.json"))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    /// The fixture's keyboard layouts, by name: each key's name as that layout prints it, where it differs.
    private func layouts() throws -> [String: OfficeKey.Labels] {
        try XCTUnwrap(fixture()["layouts"] as? [String: OfficeKey.Labels])
    }

    func testTheKeysAreTheFixturesAndTheDefaultIsOptionCommandO() throws {
        XCTAssertEqual(Set(OfficeKey.codes.keys), Set(try XCTUnwrap(fixture()["names"] as? [String])))
        XCTAssertEqual(OfficeKey.standard.text, "alt+cmd+O")
        XCTAssertEqual(OfficeKey.standard.keyCode, UInt32(kVK_ANSI_O))
        XCTAssertEqual(OfficeKey.standard.carbonModifiers, UInt32(cmdKey | optionKey), "the same keys as before")
        XCTAssertEqual(OfficeKey.standard.symbols, "⌥⌘O")
        XCTAssertEqual(Set(OfficeKey.codes.values).count, OfficeKey.codes.count, "one key code each")
    }

    /// Each of the page's asks: the keys held and what config.json keeps, or the refusal, word for word; on a
    /// French AZERTY layout too (FXKSC B2), where the Mac's own keys are refused by what the key types there and
    /// the keys are shown as printed.
    func testEachAskIsCheckedAsTheSharedFixtureSays() throws {
        let layouts = try layouts()
        for one in try XCTUnwrap(fixture()["cases"] as? [[String: Any]]) {
            let keys = try XCTUnwrap(one["keys"] as? String)
            let labels = try XCTUnwrap(layouts[one["layout"] as? String ?? "us"])
            switch OfficeKey.choice(keys, labels: labels) {
            case .success(let key):
                XCTAssertEqual(one["status"] as? Int, 200, keys)
                XCTAssertEqual(key?.text ?? OfficeKey.off, one["held"] as? String, keys)
                XCTAssertEqual(OfficeKey.saved(key), one["saved"] as? String, keys)
                if let symbols = one["symbols"] as? String { XCTAssertEqual(key?.symbols(labels), symbols, keys) }
            case .failure(let refusal):
                XCTAssertEqual(one["status"] as? Int, 400, keys)
                XCTAssertEqual(refusal.message, one["refusal"] as? String, keys)
            }
        }
    }

    /// What config.json holds: nothing is ⌥⌘O, "off" is none, and a hand-edited entry unfit to hold is ⌥⌘O.
    func testTheConfigsKeysAndAFileRoundTrip() throws {
        let config = Config()
        XCTAssertEqual(OfficeKey.chosen(config), .standard)
        config.officeKey = "off"
        XCTAssertNil(OfficeKey.chosen(config))
        for unfit in ["cmd+Q", "K", "nonsense", ""] {
            config.officeKey = unfit
            XCTAssertEqual(OfficeKey.chosen(config), .standard, unfit)
        }
        // On AZERTY, ⌘ on the US , key is its ⌘;, and stays; ⌘ on the US A key is its ⌘Q, and doesn't.
        config.officeKey = "cmd+,"
        XCTAssertEqual(OfficeKey.chosen(config, labels: [",": ";"])?.text, "cmd+,")
        XCTAssertEqual(OfficeKey.chosen(config), .standard)
        config.officeKey = "cmd+A"
        XCTAssertEqual(OfficeKey.chosen(config, labels: ["A": "Q"]), .standard)
        config.officeKey = "ctrl+alt+K"
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("officekey-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).officeKey, "ctrl+alt+K")
        config.officeKey = nil
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertNil(saved["officeKey"], "the default has no entry")
    }

    /// Each key named as the layout in use prints it (FXKSC B2), from what it types there: a stand-in for
    /// `UCKeyTranslate` with a US and a French AZERTY table, never the Mac's own layout. Only the names that differ
    /// are kept; a digit key that types a sign is named by its digit, as printed beside it.
    func testEachKeyIsNamedAsTheLayoutPrintsIt() throws {
        // What each key types on a US keyboard, without and with ⇧.
        var us: [Int: (String, String)] = [:]
        for (name, code) in OfficeKey.codes where name.count == 1 {
            let shifted = ["1": "!", "2": "@", "3": "#", "4": "$", "5": "%", "6": "^", "7": "&", "8": "*", "9": "(", "0": ")",
                           ",": "<", ".": ">", "/": "?", ";": ":", "'": "\"", "[": "{", "]": "}", "-": "_", "=": "+", "\\": "|", "`": "~"]
            us[code] = (name.lowercased(), shifted[name] ?? name)
        }
        for (name, code) in OfficeKey.codes where name.count > 1 { us[code] = (name == "Space" ? " " : name == "Tab" ? "\t" : "\u{10}", "") }
        // A French AZERTY Mac's, where it differs.
        var azerty = us
        let french: [Int: (String, String)] = [
            kVK_ANSI_Q: ("a", "A"), kVK_ANSI_W: ("z", "Z"), kVK_ANSI_A: ("q", "Q"), kVK_ANSI_Z: ("w", "W"),
            kVK_ANSI_Semicolon: ("m", "M"), kVK_ANSI_M: (",", "?"), kVK_ANSI_Comma: (";", "."), kVK_ANSI_Period: (":", "/"),
            kVK_ANSI_Slash: ("=", "+"), kVK_ANSI_Quote: ("ù", "%"), kVK_ANSI_LeftBracket: ("^", "¨"), kVK_ANSI_RightBracket: ("$", "*"),
            kVK_ANSI_Minus: (")", "°"), kVK_ANSI_Equal: ("-", "_"), kVK_ANSI_Backslash: ("`", "£"), kVK_ANSI_Grave: ("<", ">"),
            kVK_ANSI_1: ("&", "1"), kVK_ANSI_2: ("é", "2"), kVK_ANSI_3: ("\"", "3"), kVK_ANSI_4: ("'", "4"), kVK_ANSI_5: ("(", "5"),
            kVK_ANSI_6: ("§", "6"), kVK_ANSI_7: ("è", "7"), kVK_ANSI_8: ("!", "8"), kVK_ANSI_9: ("ç", "9"), kVK_ANSI_0: ("à", "0"),
        ]
        azerty.merge(french) { _, new in new }
        let layouts = try layouts()
        for (name, table) in [("us", us), ("azerty", azerty)] {
            let labels = OfficeKey.labels { code, shift in table[code].map { shift ? $0.1 : $0.0 } }
            XCTAssertEqual(labels, layouts[name], name)
        }
        XCTAssertEqual(OfficeKey.labels { _, _ in nil }, [:], "a layout that types nothing: the US names")
        XCTAssertEqual(OfficeKey.labels { code, _ in code == kVK_ANSI_K ? "к" : nil }, ["K": "К"], "printed, whatever the script")
    }

    func testTheMenusShowTheKeysChosen() {
        XCTAssertEqual(OfficeKey.standard.menuKey.key, "o")
        XCTAssertEqual(OfficeKey.standard.menuKey.modifiers, [.option, .command])
        let f5 = OfficeKey(modifiers: ["ctrl"], key: "F5").menuKey
        XCTAssertEqual(f5.key, String(Character(UnicodeScalar(UInt16(NSF5FunctionKey))!)))
        XCTAssertEqual(f5.modifiers, [.control])
        XCTAssertEqual(OfficeKey(modifiers: ["alt"], key: "Space").menuKey.key, " ")
        XCTAssertEqual(OfficeKey(modifiers: ["shift", "cmd"], key: ",").menuKey.key, ",")
        // On AZERTY the menu gives the key as printed there, which is the key macOS matches (FXKSC B2).
        let azerty: OfficeKey.Labels = ["Q": "A", ",": ";", "'": "Ù"]
        XCTAssertEqual(OfficeKey(modifiers: ["alt", "cmd"], key: "Q").menuKey(azerty).key, "a")
        XCTAssertEqual(OfficeKey(modifiers: ["cmd"], key: ",").menuKey(azerty).key, ";")
        XCTAssertEqual(OfficeKey(modifiers: ["ctrl", "alt"], key: "'").menuKey(azerty).key, "ù")
        XCTAssertEqual(OfficeKey(modifiers: ["ctrl"], key: "F5").menuKey(azerty).key, f5.key)
        XCTAssertEqual(OfficeKey.standard.menuKey(azerty).key, "o")
    }

    /// macOS's own shortcuts, as `CopySymbolicHotKeys` lists them: only those switched on count, whatever else
    /// their modifiers carry.
    func testKeysMacOSHasOnAreTaken() {
        let spotlight: [String: Any] = ["kHISymbolicHotKeyEnabled": true, "kHISymbolicHotKeyCode": NSNumber(value: kVK_Space),
                                        "kHISymbolicHotKeyModifiers": NSNumber(value: cmdKey | 0x800000)]
        let offOne: [String: Any] = ["kHISymbolicHotKeyEnabled": false, "kHISymbolicHotKeyCode": NSNumber(value: kVK_ANSI_K),
                                     "kHISymbolicHotKeyModifiers": NSNumber(value: controlKey | optionKey)]
        let list = [spotlight, offOne]
        XCTAssertTrue(OfficeKey.systemUses(OfficeKey(modifiers: ["cmd"], key: "Space"), among: list))
        XCTAssertFalse(OfficeKey.systemUses(OfficeKey(modifiers: ["ctrl", "alt"], key: "K"), among: list), "switched off")
        XCTAssertFalse(OfficeKey.systemUses(OfficeKey(modifiers: ["alt", "cmd"], key: "Space"), among: list), "other modifiers")
        XCTAssertFalse(OfficeKey.systemUses(.standard, among: [["kHISymbolicHotKeyEnabled": true]]), "an entry it can't read")
    }

    // MARK: Held at once

    /// What the stand-in registrar holds: let go when it's released, as Carbon's hot key is on deinit.
    private final class Held {
        let text: String
        let released: (String) -> Void
        init(_ text: String, released: @escaping (String) -> Void) { self.text = text; self.released = released }
        deinit { released(text) }
    }

    func testTheHolderChangesTheKeysAtOnceAndKeepsTheOldOnesWhenTaken() {
        var log: [String] = []
        var pressed = 0
        let holder = OfficeKeyHolder(register: { key, action in
            log.append("hold \(key.text)")
            guard key.text != "ctrl+alt+T" else { return nil } // macOS's
            action()
            return Held(key.text) { log.append("let go \($0)") }
        }, labels: { [:] }, action: { pressed += 1 })
        XCTAssertNil(holder.hold(.standard))
        XCTAssertEqual(holder.current, .standard)
        let k = OfficeKey(modifiers: ["ctrl", "alt"], key: "K")
        XCTAssertNil(holder.hold(k))
        XCTAssertEqual(holder.current, k)
        let taken = OfficeKey(modifiers: ["ctrl", "alt"], key: "T")
        XCTAssertEqual(holder.hold(taken), "⌃⌥T is taken: macOS has it switched on, or won't let Smolcorp hold it. Choose another.")
        XCTAssertEqual(holder.current, k, "the keys before are held again")
        XCTAssertNil(holder.hold(k), "the same keys again: let go, then held afresh")
        XCTAssertNil(holder.hold(nil))
        XCTAssertNil(holder.current)
        XCTAssertEqual(log, ["hold alt+cmd+O", "let go alt+cmd+O", "hold ctrl+alt+K", "let go ctrl+alt+K", "hold ctrl+alt+T",
                             "hold ctrl+alt+K", "let go ctrl+alt+K", "hold ctrl+alt+K", "let go ctrl+alt+K"])
        XCTAssertEqual(pressed, 4, "each held keys' action is the office's")
    }

    /// The keys before can't be held again either (FXKSC N3): both are said, and none is held. Named as the layout
    /// in use prints them.
    func testTheHolderSaysWhenTheKeysBeforeAreLostToo() {
        var free = true
        let holder = OfficeKeyHolder(register: { _, _ in free ? NSObject() : nil }, labels: { ["Q": "A"] }, action: {})
        XCTAssertNil(holder.hold(OfficeKey(modifiers: ["alt", "cmd"], key: "Q")))
        free = false
        XCTAssertEqual(holder.hold(OfficeKey(modifiers: ["ctrl", "alt"], key: "T")),
                       "⌃⌥T is taken: macOS has it switched on, or won't let Smolcorp hold it. Choose another. "
                       + "⌥⌘A couldn't be held again either, so the office has no shortcut now.")
        XCTAssertNil(holder.current, "nothing held")
    }

    /// A test copy holds no office shortcut (TKEY): the holder never asks its register (a stand-in: no real hot key is
    /// touched), says done, holds nothing, and setup's `held` is false, even with the keys off.
    func testATestCopyHoldsNoShortcutAndSetupSaysSo() {
        var asked: [String] = []
        let holder = OfficeKeyHolder(register: { key, _ in asked.append(key.text); return NSObject() }, labels: { [:] }, testCopy: true, action: {})
        XCTAssertNil(holder.hold(.standard))
        XCTAssertNil(holder.hold(OfficeKey(modifiers: ["ctrl", "alt"], key: "K")))
        XCTAssertNil(holder.hold(nil))
        XCTAssertNil(holder.current)
        XCTAssertEqual(asked, [], "no hot key asked for")
        XCTAssertFalse(OfficeKeyHolder.isHeld(nil, chosen: nil, testCopy: true), "off is not held either")
        XCTAssertFalse(OfficeKeyHolder.isHeld(.standard, chosen: .standard, testCopy: true))
        XCTAssertTrue(OfficeKeyHolder.isHeld(.standard, chosen: .standard, testCopy: false))
        XCTAssertTrue(OfficeKeyHolder.isHeld(nil, chosen: nil, testCopy: false), "the real app: off is as asked")
        XCTAssertFalse(OfficeKeyHolder.isHeld(nil, chosen: .standard, testCopy: false))
    }

    /// Every way the office comes forward goes through the test-copy check (the shortcut, the menu and the
    /// bridge all end in `show()`), and the router says held through `isHeld`.
    func testTheOfficeWindowShowsOnlyAfterTheTestCopyCheck() throws {
        let sources = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck")
        let window = try String(contentsOf: sources.appendingPathComponent("OfficeWindow.swift"), encoding: .utf8)
        let show = try XCTUnwrap(window.range(of: "    func show() {"))
        let body = String(window[show.upperBound...].prefix(900))
        let check = try XCTUnwrap(body.range(of: "guard TestCopy.mayComeForward(\"the office window\") else { return }"))
        XCTAssertLessThan(check.lowerBound, try XCTUnwrap(body.range(of: "orderFrontRegardless")).lowerBound)
        XCTAssertLessThan(check.lowerBound, try XCTUnwrap(body.range(of: "makePanel()")).lowerBound)
    }

    // MARK: The page's recorder

    /// The setup page's officeKey block, run in JavaScriptCore.
    private func pageBlock() throws -> JSContext {
        let page = try String(contentsOf: Self.root.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        let begin = try XCTUnwrap(page.range(of: "// officeKey: begin"))
        let end = try XCTUnwrap(page.range(of: "// officeKey: end"))
        let context = try XCTUnwrap(JSContext())
        context.exceptionHandler = { _, error in XCTFail("the page's script: \(error?.toString() ?? "?")") }
        context.evaluateScript(String(page[begin.upperBound..<end.lowerBound]))
        return context
    }

    /// Each key pressed in the fixture gives its keys and the page's own refusal, the same words the app says, on
    /// the layout it's pressed on (named as printed there, FXKSC B2), and the page's word on ⌘ keys (N1); and the
    /// page's tables are the app's.
    func testThePagesRecorderGivesTheKeysPressed() throws {
        let context = try pageBlock()
        context.evaluateScript("""
            function pressed(e, labels) {
              const keys = officeKeyFrom(e);
              return keys && { keys: keys, refusal: officeKeyRefusal(keys, labels), symbols: officeKeySymbols(keys, labels),
                warning: officeKeyWarning(keys, labels) };
            }
            """)
        let layouts = try layouts()
        for one in try XCTUnwrap(fixture()["pressed"] as? [[String: Any]]) {
            let event = try XCTUnwrap(one["event"])
            let labels = try XCTUnwrap(layouts[one["layout"] as? String ?? "us"])
            let answer = try XCTUnwrap(context.objectForKeyedSubscript("pressed").call(withArguments: [event, labels]))
            let name = "\(event)"
            guard let keys = one["keys"] as? String else {
                XCTAssertTrue(answer.isNull, name)
                continue
            }
            let said = try XCTUnwrap(answer.toDictionary())
            XCTAssertEqual(said["keys"] as? String, keys, name)
            XCTAssertEqual(said["refusal"] as? String, one["refusal"] as? String, name)
            XCTAssertEqual(said["warning"] as? String, one["warning"] as? String, name)
            if let symbols = one["symbols"] as? String { XCTAssertEqual(said["symbols"] as? String, symbols, name) }
            // The page refuses what the app refuses, in its words.
            switch OfficeKey.choice(keys, labels: labels) {
            case .failure(let refusal): XCTAssertEqual(refusal.message, said["refusal"] as? String, name)
            case .success(let key):
                XCTAssertNil(said["refusal"] as? String, name)
                XCTAssertEqual(key?.symbols(labels), said["symbols"] as? String, name)
            }
        }
        XCTAssertEqual(context.evaluateScript("officeKeyReserved").toArray() as? [String], OfficeKey.reserved)
        // Every key the page can record is one the app can hold, and the other way round.
        let codes = (65...90).map { "Key\(Character(UnicodeScalar($0)!))" } + (0...9).map { "Digit\($0)" } + (1...12).map { "F\($0)" }
            + (context.evaluateScript("Object.keys(officeKeyCodes)").toArray() as? [String] ?? [])
        let recorded = codes.compactMap { context.objectForKeyedSubscript("officeKeyFrom").call(withArguments: [["code": $0, "metaKey": true]]).toString() }
            .map { String($0.dropFirst("cmd+".count)) }
        XCTAssertEqual(Set(recorded), Set(OfficeKey.codes.keys))
        XCTAssertEqual(context.objectForKeyedSubscript("officeKeySymbols").call(withArguments: ["ctrl+shift+cmd+F5", [:]]).toString(), "⌃⇧⌘F5")
    }

    /// The recorder in the Mac's Settings window (FXKSC B1): WebKit leaves a button it's clicked unfocused, so its
    /// keys go to the page, never the button. On a DOM stand-in: the click starts it and focuses the button, the
    /// keys are heard on the document while it records (capture, so nothing else on the page takes them) and sent
    /// to be saved, and the listener goes once it stops: a key pressed, Esc, leaving the button or a second click.
    func testTheRecorderHearsTheKeysAfterAClickAsWebKitLeavesTheButtonUnfocused() throws {
        let context = try pageBlock()
        context.evaluateScript("""
            function standIn() {
              const listeners = {};
              return {
                focused: false,
                focus() { this.focused = true; },
                addEventListener(type, f, capture) { (listeners[type] = listeners[type] || []).push({ f: f, capture: !!capture }); },
                removeEventListener(type, f, capture) {
                  listeners[type] = (listeners[type] || []).filter((l) => l.f !== f || l.capture !== !!capture);
                },
                count(type, capture) { return (listeners[type] || []).filter((l) => l.capture === capture).length; },
                fire(type, e) {
                  const event = Object.assign({ prevented: false, preventDefault() { this.prevented = true; }, stopPropagation() {} }, e);
                  for (const l of (listeners[type] || []).slice()) l.f(event);
                  return event;
                },
              };
            }
            const button = standIn(), doc = standIn(), log = [];
            officeKeyRecorder(button, doc, () => ({ Q: 'A', A: 'Q' }), (keys) => log.push('save ' + keys),
              (why) => log.push('refuse ' + why), (on) => log.push(on ? 'recording' : 'stopped'));
            const click = () => button.fire('click', { detail: 1 });
            const key = (e) => doc.fire('keydown', e);
            const seen = [];
            click();
            seen.push(button.focused, doc.count('keydown', true));
            seen.push(key({ code: 'ControlLeft', key: 'Control', ctrlKey: true }).prevented);
            seen.push(key({ code: 'KeyK', key: 'k', ctrlKey: true, altKey: true }).prevented, doc.count('keydown', true));
            key({ code: 'KeyJ', key: 'j', ctrlKey: true, altKey: true });
            click();
            key({ code: 'KeyA', key: 'q', metaKey: true });
            click();
            key({ code: 'Escape', key: 'Escape' });
            click();
            button.fire('blur', {});
            click();
            click();
            seen.push(doc.count('keydown', true));
            """)
        XCTAssertEqual(context.evaluateScript("seen").toArray() as? [AnyHashable], [true, 1, true, true, 0, 0] as [AnyHashable],
                       "focused; heard on the document; a modifier alone waits; the keys taken; let go")
        XCTAssertEqual(context.evaluateScript("log").toArray() as? [String], [
            "recording", "stopped", "save ctrl+alt+K",
            "recording", "stopped", "refuse ⌘Q is the Mac's own, in every app: choose another.",
            "recording", "stopped",
            "recording", "stopped",
            "recording", "stopped",
        ], "a key pressed after it stopped isn't heard; on AZERTY ⌘ on the US A key is ⌘Q")
    }

    /// The page posts only from its recorder and its two buttons, and says a refusal under the step.
    func testThePageSetsTheKeysFromTheRecorderAndItsButtons() throws {
        let page = try String(contentsOf: Self.root.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("const res = await post(`/setup/officekey?keys=${encodeURIComponent(keys)}`);"))
        XCTAssertTrue(page.contains("if (!res.ok) officeKeyProblem(await res.text());"))
        XCTAssertTrue(page.contains("$('officekey-default').onclick = () => setOfficeKey('default');"))
        XCTAssertTrue(page.contains("$('officekey-off').onclick = () => setOfficeKey('off');"))
        XCTAssertTrue(page.contains("officeKeyRecorder($('officekey'), document, officeKeyLabels, setOfficeKey, officeKeyProblem,"),
                      "the recorder saves, or says why not at once")
        XCTAssertEqual(page.components(separatedBy: "/setup/officekey").count - 1, 1, "asked from one place")
    }
}

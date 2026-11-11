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

    func testTheKeysAreTheFixturesAndTheDefaultIsOptionCommandO() throws {
        XCTAssertEqual(Set(OfficeKey.codes.keys), Set(try XCTUnwrap(fixture()["names"] as? [String])))
        XCTAssertEqual(OfficeKey.standard.text, "alt+cmd+O")
        XCTAssertEqual(OfficeKey.standard.keyCode, UInt32(kVK_ANSI_O))
        XCTAssertEqual(OfficeKey.standard.carbonModifiers, UInt32(cmdKey | optionKey), "the same keys as before")
        XCTAssertEqual(OfficeKey.standard.symbols, "⌥⌘O")
        XCTAssertEqual(Set(OfficeKey.codes.values).count, OfficeKey.codes.count, "one key code each")
    }

    /// Each of the page's asks: the keys held and what config.json keeps, or the refusal, word for word.
    func testEachAskIsCheckedAsTheSharedFixtureSays() throws {
        for one in try XCTUnwrap(fixture()["cases"] as? [[String: Any]]) {
            let keys = try XCTUnwrap(one["keys"] as? String)
            switch OfficeKey.choice(keys) {
            case .success(let key):
                XCTAssertEqual(one["status"] as? Int, 200, keys)
                XCTAssertEqual(key?.text ?? OfficeKey.off, one["held"] as? String, keys)
                XCTAssertEqual(OfficeKey.saved(key), one["saved"] as? String, keys)
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

    func testTheMenusShowTheKeysChosen() {
        XCTAssertEqual(OfficeKey.standard.menuKey.key, "o")
        XCTAssertEqual(OfficeKey.standard.menuKey.modifiers, [.option, .command])
        let f5 = OfficeKey(modifiers: ["ctrl"], key: "F5").menuKey
        XCTAssertEqual(f5.key, String(Character(UnicodeScalar(UInt16(NSF5FunctionKey))!)))
        XCTAssertEqual(f5.modifiers, [.control])
        XCTAssertEqual(OfficeKey(modifiers: ["alt"], key: "Space").menuKey.key, " ")
        XCTAssertEqual(OfficeKey(modifiers: ["shift", "cmd"], key: ",").menuKey.key, ",")
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
            guard key.text != "ctrl+alt+T" else { return nil } // another app's
            action()
            return Held(key.text) { log.append("let go \($0)") }
        }, action: { pressed += 1 })
        XCTAssertNil(holder.hold(.standard))
        XCTAssertEqual(holder.current, .standard)
        let k = OfficeKey(modifiers: ["ctrl", "alt"], key: "K")
        XCTAssertNil(holder.hold(k))
        XCTAssertEqual(holder.current, k)
        let taken = OfficeKey(modifiers: ["ctrl", "alt"], key: "T")
        XCTAssertEqual(holder.hold(taken), "⌃⌥T is already used by macOS or another app: choose another.")
        XCTAssertEqual(holder.current, k, "the keys before are held again")
        XCTAssertNil(holder.hold(k), "the same keys again: let go, then held afresh")
        XCTAssertNil(holder.hold(nil))
        XCTAssertNil(holder.current)
        XCTAssertEqual(log, ["hold alt+cmd+O", "let go alt+cmd+O", "hold ctrl+alt+K", "let go ctrl+alt+K", "hold ctrl+alt+T",
                             "hold ctrl+alt+K", "let go ctrl+alt+K", "hold ctrl+alt+K", "let go ctrl+alt+K"])
        XCTAssertEqual(pressed, 4, "each held keys' action is the office's")
    }

    // MARK: The route

    /// POST /setup/officekey, from the page: checked first (400, the holder not asked), held at once, then kept;
    /// keys the holder can't hold are 409 and nothing is kept. Never a GM, a hire or a tool.
    func testTheRouteHoldsTheKeysThenKeepsThemFromThePageOnly() throws {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/officekey")?.callers, [.page])
        let config = Config.shared
        let was = config.officeKey
        defer { config.officeKey = was; config.save() }
        let key = SummaryKey.store
        SummaryKey.store = .init(read: { nil }, hint: { nil }, write: { _, _ in false }, delete: {})
        SummaryKey.forgetKnown()
        let tailnet = Tailnet.ask
        Tailnet.ask = { nil }
        let providers = SummaryProviders.shared.preset([:])
        defer {
            providers()
            SummaryKey.store = key
            SummaryKey.forgetKnown()
            Tailnet.ask = tailnet
            Tailnet.forget()
        }
        config.officeKey = nil
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        let router = DeckRouter(deck: deck)
        var asked: [String] = []
        router.officeKeyChanged = { key in
            asked.append(key?.text ?? "off")
            return key?.text == "ctrl+alt+T" ? key.map(OfficeKey.takenRefusal) : nil
        }
        let setup = deck.access.setupSessions
        let cookie = "\(DeckRouter.setupCookie)=\(try XCTUnwrap(setup.open(link: setup.newLink())))"
        func ask(_ keys: String) throws -> (Int, String, [String: Any]?) {
            var got: Response?
            let answered = expectation(description: keys)
            router.handle(Request.addressed(method: "POST", path: "/setup/officekey", query: ["keys": keys],
                                            headers: [DeckRouter.setupHeader: "1", "host": "127.0.0.1:7710", "cookie": cookie],
                                            body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")) { reply in
                if case .response(let response) = reply { got = response }
                answered.fulfill()
            }
            wait(for: [answered], timeout: 10)
            let response = try XCTUnwrap(got)
            return (response.status, String(decoding: response.body, as: UTF8.self),
                    (try? JSONSerialization.jsonObject(with: response.body)) as? [String: Any])
        }
        let held = try ask("ctrl+alt+K")
        XCTAssertEqual(held.0, 200)
        XCTAssertEqual((held.2?["officeKey"] as? [String: Any])?["keys"] as? String, "ctrl+alt+K")
        XCTAssertEqual((held.2?["officeKey"] as? [String: Any])?["default"] as? String, "alt+cmd+O")
        XCTAssertEqual(config.officeKey, "ctrl+alt+K")

        let taken = try ask("ctrl+alt+T")
        XCTAssertEqual(taken.0, 409)
        XCTAssertEqual(taken.1, "⌃⌥T is already used by macOS or another app: choose another.")
        XCTAssertEqual(config.officeKey, "ctrl+alt+K", "nothing kept")

        let reserved = try ask("cmd+W")
        XCTAssertEqual(reserved.0, 400)
        XCTAssertEqual(reserved.1, "⌘W is the Mac's own, in every app: choose another.")

        XCTAssertEqual(try ask("off").0, 200)
        XCTAssertEqual(config.officeKey, "off")
        XCTAssertEqual(try ask("default").0, 200)
        XCTAssertNil(config.officeKey, "back to ⌥⌘O: no entry")
        XCTAssertEqual(asked, ["ctrl+alt+K", "ctrl+alt+T", "off", "alt+cmd+O"], "a refused ask never reaches the holder")
    }

    // MARK: The page's recorder

    /// The setup page's officeKey block, run in JavaScriptCore: each key pressed in the fixture gives its keys and
    /// the page's own refusal, the same words the app says; and its tables are the app's.
    func testThePagesRecorderGivesTheKeysPressed() throws {
        let page = try String(contentsOf: Self.root.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        let begin = try XCTUnwrap(page.range(of: "// officeKey: begin"))
        let end = try XCTUnwrap(page.range(of: "// officeKey: end"))
        let context = try XCTUnwrap(JSContext())
        var thrown: String?
        context.exceptionHandler = { _, error in thrown = error?.toString() }
        context.evaluateScript(String(page[begin.upperBound..<end.lowerBound]))
        context.evaluateScript("""
            function pressed(e) { const keys = officeKeyFrom(e); return { keys: keys, refusal: keys && officeKeyRefusal(keys) }; }
            """)
        XCTAssertNil(thrown)
        for one in try XCTUnwrap(fixture()["pressed"] as? [[String: Any]]) {
            let event = try XCTUnwrap(one["event"])
            let said = try XCTUnwrap(context.objectForKeyedSubscript("pressed").call(withArguments: [event]).toDictionary())
            let name = "\(event)"
            XCTAssertEqual(said["keys"] as? String, one["keys"] as? String, name)
            guard let keys = one["keys"] as? String else { continue }
            XCTAssertEqual(said["refusal"] as? String, one["refusal"] as? String, name)
            // The page refuses what the app refuses, in its words.
            if case .failure(let refusal) = OfficeKey.choice(keys) { XCTAssertEqual(refusal.message, said["refusal"] as? String, name) }
        }
        XCTAssertEqual(context.evaluateScript("officeKeyReserved").toArray() as? [String], OfficeKey.reserved)
        // Every key the page can record is one the app can hold, and the other way round.
        let codes = (65...90).map { "Key\(Character(UnicodeScalar($0)!))" } + (0...9).map { "Digit\($0)" } + (1...12).map { "F\($0)" }
            + (context.evaluateScript("Object.keys(officeKeyCodes)").toArray() as? [String] ?? [])
        let recorded = codes.compactMap { context.objectForKeyedSubscript("officeKeyFrom").call(withArguments: [["code": $0, "metaKey": true]]).toString() }
            .map { String($0.dropFirst("cmd+".count)) }
        XCTAssertEqual(Set(recorded), Set(OfficeKey.codes.keys))
        XCTAssertEqual(context.objectForKeyedSubscript("officeKeySymbols").call(withArguments: ["ctrl+shift+cmd+F5"]).toString(), "⌃⇧⌘F5")
    }

    /// The page posts only from its recorder and its two buttons, and says a refusal under the step.
    func testThePageSetsTheKeysFromTheRecorderAndItsButtons() throws {
        let page = try String(contentsOf: Self.root.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("const res = await post(`/setup/officekey?keys=${encodeURIComponent(keys)}`);"))
        XCTAssertTrue(page.contains("if (!res.ok) officeKeyProblem(await res.text());"))
        XCTAssertTrue(page.contains("$('officekey-default').onclick = () => setOfficeKey('default');"))
        XCTAssertTrue(page.contains("$('officekey-off').onclick = () => setOfficeKey('off');"))
        XCTAssertTrue(page.contains("if (refusal) return officeKeyProblem(refusal);"), "refused at once, nothing asked")
        XCTAssertEqual(page.components(separatedBy: "/setup/officekey").count - 1, 1, "asked from one place")
    }
}

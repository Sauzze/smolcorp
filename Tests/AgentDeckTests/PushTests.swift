import CryptoKit
import XCTest
@testable import AgentDeck

/// iPhone app (personal build): notifications through Apple. The provider
/// token is signed with a key made here for the test, never the real one;
/// the Keychain and the network are stubbed: nothing is read or sent.
final class PushTests: XCTestCase {
    private let token = String(repeating: "ab", count: 32)

    private func phone(_ id: String, _ push: PushTarget?) -> PairedPhone {
        PairedPhone(id: id, name: "iPhone app", pairedAt: Date(), lastSeen: Date(), keyHash: id, push: push)
    }

    private func decode(_ part: Substring) throws -> [String: Any] {
        var text = part.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
        while text.count % 4 != 0 { text += "=" }
        let data = try XCTUnwrap(Data(base64Encoded: text))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    // MARK: The provider token

    /// The key's comment names the team it signs for and the app it pushes to; anything else is no key at all.
    func testTheKeysTeamAndAppComeFromItsComment() {
        XCTAssertEqual(PushKey.parsed(keyID: "TESTKEY123", comment: "ABCDE12345 com.example.smolcorp"),
                       PushKey.Account(keyID: "TESTKEY123", teamID: "ABCDE12345", topic: "com.example.smolcorp"))
        XCTAssertEqual(PushKey.parsed(keyID: "K", comment: "  ABCDE12345\tcom.example.app ")?.topic, "com.example.app")
        for comment in [nil, "", "ABCDE12345", "abcde12345 com.example.app", "ABCDE1234 com.example.app", "ABCDE12345 noperiod",
                        "ABCDE12345 com.example.app extra", "ABCDE12345 com.exa mple"] {
            XCTAssertNil(PushKey.parsed(keyID: "K", comment: comment), comment ?? "nil")
        }
        XCTAssertNil(PushKey.parsed(keyID: "", comment: "ABCDE12345 com.example.app"))
    }

    func testTheProviderTokenIsAnES256JWTThatTheKeysPublicHalfVerifies() throws {
        let key = P256.Signing.PrivateKey()
        let at = Date(timeIntervalSince1970: 1_790_000_000)
        let jwt = try APNsJWT.make(key: key, keyID: "TESTKEY123", teamID: "ABCDE12345", issuedAt: at)
        let parts = jwt.split(separator: ".")
        XCTAssertEqual(parts.count, 3)
        XCTAssertFalse(jwt.contains("=") || jwt.contains("+") || jwt.contains("/"), "base64url, unpadded")
        let header = try decode(parts[0]), claims = try decode(parts[1])
        XCTAssertEqual(header["alg"] as? String, "ES256")
        XCTAssertEqual(header["kid"] as? String, "TESTKEY123")
        XCTAssertEqual(claims["iss"] as? String, "ABCDE12345")
        XCTAssertEqual(claims["iat"] as? Int, 1_790_000_000)
        XCTAssertEqual(claims.count, 2)

        var text = String(parts[2]).replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
        while text.count % 4 != 0 { text += "=" }
        let raw = try XCTUnwrap(Data(base64Encoded: text))
        XCTAssertEqual(raw.count, 64, "r and s, 32 bytes each")
        let signature = try P256.Signing.ECDSASignature(rawRepresentation: raw)
        XCTAssertTrue(key.publicKey.isValidSignature(signature, for: Data("\(parts[0]).\(parts[1])".utf8)))
        XCTAssertFalse(P256.Signing.PrivateKey().publicKey.isValidSignature(signature, for: Data("\(parts[0]).\(parts[1])".utf8)))
    }

    func testTheKeyIsReadFromAP8FilesTextOrItsBodyAlone() throws {
        let key = P256.Signing.PrivateKey()
        let pem = key.pemRepresentation
        XCTAssertTrue(pem.hasPrefix("-----BEGIN PRIVATE KEY-----"), "a .p8 is PKCS#8, as this is")
        XCTAssertEqual(APNsJWT.privateKey(Data(pem.utf8))?.rawRepresentation, key.rawRepresentation)
        XCTAssertEqual(APNsJWT.privateKey(Data((pem + "\n").utf8))?.rawRepresentation, key.rawRepresentation)
        let body = pem.split(separator: "\n").filter { !$0.hasPrefix("-----") }.joined(separator: "\n")
        XCTAssertEqual(APNsJWT.privateKey(Data(body.utf8))?.rawRepresentation, key.rawRepresentation)
        XCTAssertNil(APNsJWT.privateKey(Data("not a key".utf8)))
        XCTAssertNil(APNsJWT.privateKey(Data()))
    }

    // MARK: What's news, and how often

    func testNewsIsANewWaitOrANewFinishAndWhatsRunningAtTheStartIsnt() {
        let pushes = Pushes(keyID: { nil }, topic: { "com.example.smolcorp" }, sign: { _, _ in nil })
        XCTAssertEqual(pushes.news([("a", "acmebill", .asking)]), [], "the first look is where it starts from")
        XCTAssertEqual(pushes.news([("a", "acmebill", .asking)]), [], "still asking isn't news")
        XCTAssertEqual(pushes.news([("a", "acmebill", .thinking), ("b", "smolcorp", .thinking)]), [])
        XCTAssertEqual(pushes.news([("a", "acmebill", .unread), ("b", "smolcorp", .asking)]), [
            .desk("acmebill", .finished, session: "a"), .desk("smolcorp", .waiting, session: "b"),
        ])
        XCTAssertEqual(pushes.news([("a", "acmebill", .idle), ("b", "smolcorp", .error)]), [], "seen, or an error: no news")
    }

    func testTheSameNewsAboutTheSameSessionWaitsAMinute() {
        var now = Date(timeIntervalSince1970: 1_000)
        let pushes = Pushes(keyID: { nil }, topic: { "com.example.smolcorp" }, sign: { _, _ in nil }, now: { now })
        _ = pushes.news([("a", "acmebill", .thinking)])
        XCTAssertEqual(pushes.news([("a", "acmebill", .unread)]).count, 1)
        now += 20
        _ = pushes.news([("a", "acmebill", .thinking)])
        XCTAssertEqual(pushes.news([("a", "acmebill", .unread)]), [], "finished again 20 s later: not again")
        // Other news about it, or the same about another, isn't held back.
        XCTAssertEqual(pushes.news([("a", "acmebill", .asking)]).map(\.event), [.waiting])
        _ = pushes.news([("b", "smolcorp", .thinking), ("a", "acmebill", .thinking)])
        XCTAssertEqual(pushes.news([("b", "smolcorp", .unread), ("a", "acmebill", .thinking)]).map(\.session), ["b"])
        now += 41
        _ = pushes.news([("a", "acmebill", .thinking)])
        XCTAssertEqual(pushes.news([("a", "acmebill", .unread)]).count, 1, "a minute on, it goes")
    }

    // MARK: What goes through Apple

    func testANotificationSaysOnlyTheDeskAndWhatHappened() throws {
        let note = PushNote.desk("acmebill", .waiting, session: "s-1")
        XCTAssertEqual(note.title, "acmebill")
        XCTAssertEqual(note.body, "needs you")
        XCTAssertEqual(PushNote.desk("acmebill", .finished, session: "s-1").body, "finished")
        let payload = try XCTUnwrap(JSONSerialization.jsonObject(with: note.payload) as? [String: Any])
        XCTAssertEqual(Set(payload.keys), ["aps", "event", "session"])
        XCTAssertEqual(payload["session"] as? String, "s-1")
        let aps = try XCTUnwrap(payload["aps"] as? [String: Any])
        XCTAssertEqual(Set(aps.keys), ["alert", "sound", "thread-id"])
        XCTAssertEqual(aps["alert"] as? [String: String], ["title": "acmebill", "body": "needs you"])
    }

    func testTheGMsMessageIsShortOneLineAndRedacted() throws {
        let note = try XCTUnwrap(PushNote.gm(title: "Audit\ndone", body: String(repeating: "z", count: 500)))
        XCTAssertEqual(note.title, "Audit done")
        XCTAssertEqual(note.body.count, PushNote.bodyMost)
        XCTAssertNil(note.session)
        XCTAssertEqual(PushNote.gm(title: "Build", body: "the token is sk-ant-123")?.body, Redact.placeholder)
        XCTAssertNil(PushNote.gm(title: " ", body: ""))
        XCTAssertEqual(PushNote.gm(title: "", body: "hi")?.title, "Smolcorp")
    }

    func testTheRequestIsApplesHTTP2PushWithTheTopicAndTheSandboxForADevelopmentBuild() throws {
        let note = PushNote.desk("acmebill", .finished, session: "s-1")
        let sandbox = try XCTUnwrap(Pushes.request(note, to: PushTarget(token: token), provider: "jwt", topic: "com.example.smolcorp"))
        XCTAssertEqual(sandbox.url?.absoluteString, "https://api.sandbox.push.apple.com/3/device/\(token)")
        XCTAssertEqual(sandbox.httpMethod, "POST")
        XCTAssertEqual(sandbox.value(forHTTPHeaderField: "authorization"), "bearer jwt")
        XCTAssertEqual(sandbox.value(forHTTPHeaderField: "apns-topic"), "com.example.smolcorp")
        XCTAssertEqual(sandbox.value(forHTTPHeaderField: "apns-push-type"), "alert")
        XCTAssertEqual(sandbox.value(forHTTPHeaderField: "apns-collapse-id"), "finished-s-1")
        XCTAssertEqual(sandbox.httpBody, note.payload)
        var release = PushTarget(token: token)
        release.sandbox = false
        XCTAssertEqual(Pushes.request(note, to: release, provider: "jwt", topic: "com.example.smolcorp")?.url?.host, "api.push.apple.com")
        XCTAssertNil(Pushes.request(note, to: PushTarget(token: "../../x"), provider: "jwt", topic: "com.example.smolcorp"), "only a token goes in the path")
    }

    func testOnlyHexTokensAreTaken() {
        XCTAssertTrue(PushTarget.isToken(token))
        for wrong in ["", "abc", String(repeating: "AB", count: 32), String(repeating: "zz", count: 32), String(repeating: "a", count: 65),
                      String(repeating: "ab", count: 101), token + "/"] {
            XCTAssertFalse(PushTarget.isToken(wrong), wrong)
        }
    }

    // MARK: Sending

    func testItSendsToEachPhoneThatWantsItAndForgetsATokenAppleRefuses() throws {
        var sent: [URLRequest] = []
        var signed = 0
        let pushes = Pushes(transport: { request, done in
            sent.append(request)
            done(request.url!.path.hasSuffix("cd" + String(repeating: "cd", count: 31)) ? Data(#"{"reason":"Unregistered"}"#.utf8) : nil,
                 request.url!.path.hasSuffix("cd" + String(repeating: "cd", count: 31)) ? 410 : 200, nil)
        }, keyID: { "TESTKEY123" }, topic: { "com.example.smolcorp" }, sign: { id, _ in signed += 1; return "jwt-\(id)" })
        var noFinish = PushTarget(token: String(repeating: "ef", count: 32))
        noFinish.finished = false
        let phones = [phone("p1", PushTarget(token: token)), phone("p2", PushTarget(token: String(repeating: "cd", count: 32))),
                      phone("p3", noFinish), phone("p4", nil)]
        var forgotten: [String] = []
        let delivered = expectation(description: "sent")
        pushes.send(.desk("acmebill", .finished, session: "s-1"), to: phones, forget: { phone, token in
            forgotten.append(phone)
            XCTAssertEqual(token, String(repeating: "cd", count: 32), "the token Apple refused, to forget only it")
        }) { count in
            XCTAssertEqual(count, 1)
            delivered.fulfill()
        }
        wait(for: [delivered], timeout: 5)
        XCTAssertEqual(sent.count, 2, "p3 doesn't want it, p4 has no token")
        XCTAssertEqual(sent.map { $0.value(forHTTPHeaderField: "authorization") }, ["bearer jwt-TESTKEY123", "bearer jwt-TESTKEY123"])
        XCTAssertEqual(signed, 1)
        let forgot = expectation(description: "forgotten")
        DispatchQueue.main.async { forgot.fulfill() }
        wait(for: [forgot], timeout: 5)
        XCTAssertEqual(forgotten, ["p2"])

        // The provider token is kept for the next, not signed again.
        let again = expectation(description: "again")
        pushes.send(.desk("acmebill", .waiting, session: "s-1"), to: phones) { _ in again.fulfill() }
        wait(for: [again], timeout: 5)
        XCTAssertEqual(signed, 1)
    }

    func testPushesSentTogetherShareOneProviderToken() {
        var signed = 0
        var tokens: Set<String> = []
        let pushes = Pushes(transport: { request, done in
            tokens.insert(request.value(forHTTPHeaderField: "authorization") ?? "")
            done(nil, 200, nil)
        }, keyID: { "TESTKEY123" }, topic: { "com.example.smolcorp" }, sign: { _, _ in signed += 1; return "jwt-\(signed)" })
        let phones = [phone("p1", PushTarget(token: token))]
        let all = expectation(description: "all sent")
        all.expectedFulfillmentCount = 3
        // As one change's news goes: one after another, before any has been signed for.
        for event in [PushEvent.finished, .waiting, .finished] {
            pushes.send(.desk("acmebill", event, session: "s-\(event)"), to: phones) { _ in all.fulfill() }
        }
        wait(for: [all], timeout: 5)
        XCTAssertEqual(signed, 1)
        XCTAssertEqual(tokens, ["bearer jwt-1"])
    }

    func testOnlyAnExpiredProviderTokenIsSignedAgain() {
        var signed = 0
        var answer: (Int, String) = (403, #"{"reason":"InvalidProviderToken"}"#)
        let pushes = Pushes(transport: { _, done in done(Data(answer.1.utf8), answer.0, nil) }, keyID: { "TESTKEY123" }, topic: { "com.example.smolcorp" },
                            sign: { _, _ in signed += 1; return "jwt-\(signed)" })
        let phones = [phone("p1", PushTarget(token: token))]
        func send() {
            let sent = expectation(description: "sent")
            pushes.send(.desk("acmebill", .finished, session: "s-1"), to: phones) { _ in sent.fulfill() }
            wait(for: [sent], timeout: 5)
            // The push queue's own follow-up, done before the next send there.
            let settled = expectation(description: "settled")
            DispatchQueue.global().asyncAfter(deadline: .now() + 0.1) { settled.fulfill() }
            wait(for: [settled], timeout: 5)
        }
        send()
        send()
        XCTAssertEqual(signed, 1, "another reason: signing again wouldn't help, and Apple counts new tokens")
        answer = (403, #"{"reason":"ExpiredProviderToken"}"#)
        send()
        answer = (200, "")
        send()
        XCTAssertEqual(signed, 2, "expired: one new token, then kept")
        send()
        XCTAssertEqual(signed, 2)
        XCTAssertTrue(Pushes.isExpiredProvider(status: 403, body: Data(#"{"reason":"ExpiredProviderToken"}"#.utf8)))
        XCTAssertFalse(Pushes.isExpiredProvider(status: 400, body: Data(#"{"reason":"ExpiredProviderToken"}"#.utf8)))
    }

    // MARK: A phone's token on the Mac

    private func phonesFile() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: folder) }
        return folder.appendingPathComponent("phones.json")
    }

    func testAPhoneUnpairingItselfClearsItsTokenOnTheMac() throws {
        let file = try phonesFile()
        let phones = Phones(file: file)
        _ = phones.pair(name: "iPhone app")
        let id = try XCTUnwrap(phones.all.first?.id)
        phones.setPush(DeckRouter.pushTarget(["token": token, "gm": false]), for: id)
        XCTAssertEqual(phones.all.first?.push?.token, token)
        XCTAssertEqual(phones.all.first?.push?.gm, false)
        // What the phone sends as it unpairs: no token.
        XCTAssertNil(DeckRouter.pushTarget([:]))
        XCTAssertNil(DeckRouter.pushTarget(["token": NSNull()]))
        XCTAssertNil(DeckRouter.pushTarget(["token": "not hex"]))
        phones.setPush(DeckRouter.pushTarget([:]), for: id)
        XCTAssertNil(phones.all.first?.push)
        XCTAssertNil(Phones(file: file).all.first?.push, "and it's saved so")
        // Nothing to send to it any more.
        var sent = 0
        let pushes = Pushes(transport: { _, done in sent += 1; done(nil, 200, nil) }, keyID: { "K" }, topic: { "com.example.smolcorp" }, sign: { _, _ in "jwt" })
        let done = expectation(description: "done")
        pushes.send(.gm(title: "GM", body: "hi")!, to: phones.all) { count in
            XCTAssertEqual(count, 0)
            done.fulfill()
        }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(sent, 0)
    }

    func testALate410ForgetsOnlyTheTokenItWasAbout() throws {
        let phones = Phones(file: try phonesFile())
        _ = phones.pair(name: "iPhone app")
        let id = try XCTUnwrap(phones.all.first?.id)
        let old = String(repeating: "ab", count: 32), new = String(repeating: "cd", count: 32)
        phones.setPush(PushTarget(token: new), for: id)
        phones.forgetPush(token: old, for: id)
        XCTAssertEqual(phones.all.first?.push?.token, new, "a new token since: kept")
        phones.forgetPush(token: new, for: id)
        XCTAssertNil(phones.all.first?.push)
    }

    func testWithoutAKeyNothingIsSent() {
        var sent = 0
        let pushes = Pushes(transport: { _, done in sent += 1; done(nil, 200, nil) }, keyID: { nil }, topic: { "com.example.smolcorp" }, sign: { _, _ in "jwt" })
        XCTAssertFalse(pushes.hasKey)
        let done = expectation(description: "done")
        pushes.send(.desk("acmebill", .finished, session: "s-1"), to: [phone("p1", PushTarget(token: token))]) { count in
            XCTAssertEqual(count, 0)
            done.fulfill()
        }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(sent, 0)
    }

    func testAKeychainThatWaitsNeverHoldsUpTheDeck() {
        // The Keychain asking for a password: the lookup waits till it's answered.
        let answered = DispatchSemaphore(value: 0)
        let pushes = Pushes(keyID: { answered.wait(); return "K" }, topic: { "com.example.smolcorp" }, sign: { _, _ in "jwt" })
        let started = Date()
        XCTAssertFalse(pushes.hasKey, "not known yet: no")
        XCTAssertLessThan(Date().timeIntervalSince(started), 0.5, "answered at once, not after the Keychain")
        answered.signal()
        let known = expectation(description: "known")
        func look() { pushes.hasKey ? known.fulfill() : DispatchQueue.main.asyncAfter(deadline: .now() + 0.05, execute: look) }
        look()
        wait(for: [known], timeout: 5)
    }

    func testAppleSaysWhichTokensAreDead() {
        XCTAssertTrue(Pushes.isDeadToken(status: 410, body: nil))
        XCTAssertTrue(Pushes.isDeadToken(status: 400, body: Data(#"{"reason":"BadDeviceToken"}"#.utf8)))
        XCTAssertFalse(Pushes.isDeadToken(status: 400, body: Data(#"{"reason":"PayloadTooLarge"}"#.utf8)))
        XCTAssertFalse(Pushes.isDeadToken(status: 403, body: Data(#"{"reason":"ExpiredProviderToken"}"#.utf8)))
        XCTAssertFalse(Pushes.isDeadToken(status: 200, body: nil))
    }

    func testAPhonesTokenIsKeptBesideItsPairingAndAnOldFileStillLoads() throws {
        let old = #"[{"id":"p1","name":"iPhone","pairedAt":"2026-10-21T10:00:00Z","lastSeen":"2026-10-21T10:00:00Z","keyHash":"h"}]"#
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let phones = try decoder.decode([PairedPhone].self, from: Data(old.utf8))
        XCTAssertNil(phones[0].push)
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        let withPush = try decoder.decode([PairedPhone].self, from: encoder.encode([phone("p1", PushTarget(token: token))]))
        XCTAssertEqual(withPush[0].push, PushTarget(token: token))
    }

    func testThePhonesSettingsSayWhetherTheMacHasAKey() {
        XCTAssertEqual(DeckRouter.notifications(nil, hasKey: false)["key"] as? Bool, false)
        var push = PushTarget(token: token)
        push.gm = false
        let state = DeckRouter.notifications(push, hasKey: true)
        XCTAssertEqual(state["key"] as? Bool, true)
        XCTAssertEqual(state["registered"] as? Bool, true)
        XCTAssertEqual(state["gm"] as? Bool, false)
        XCTAssertEqual(state["finished"] as? Bool, true)
        XCTAssertNil(state["token"], "the token never goes back")
    }
}

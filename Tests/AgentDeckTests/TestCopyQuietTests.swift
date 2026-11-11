import Foundation
import XCTest
@testable import AgentDeck

/// A test copy can never disturb the owner (TCH): it never comes forward or
/// shows a window or an alert by itself, only logs it, and listens on this Mac
/// only unless SMOLCORP_TEST_LISTEN=lan asks for the network.
final class TestCopyQuietTests: XCTestCase {
    // MARK: Nothing in front by itself

    func testATestCopyLogsInsteadOfComingForward() {
        var logged: [String] = []
        XCTAssertFalse(TestCopy.mayComeForward("first-run Settings", testCopy: true) { logged.append($0) })
        XCTAssertEqual(logged, ["Smolcorp Test: first-run Settings not shown, a test copy never comes forward by itself"])
    }

    func testTheRealAppComesForwardAndLogsNothing() {
        var logged: [String] = []
        XCTAssertTrue(TestCopy.mayComeForward("first-run Settings", testCopy: false) { logged.append($0) })
        XCTAssertEqual(logged, [])
    }

    /// Every activation of the app goes through `Foreground.activate`, so through the check: it's the
    /// one `NSApp.activate` left in the app's code.
    func testEveryActivationGoesThroughTheCheck() throws {
        var found: [String] = []
        for name in try FileManager.default.contentsOfDirectory(atPath: sources.path) where name.hasSuffix(".swift") {
            let text = try source(name)
            for line in text.components(separatedBy: "\n") where line.contains("NSApp.activate(") {
                found.append(name + ": " + line.trimmingCharacters(in: .whitespaces))
            }
        }
        XCTAssertEqual(found, ["App.swift: NSApp.activate(ignoringOtherApps: true)"])
        let app = try source("App.swift")
        XCTAssertTrue(app.contains("""
                guard TestCopy.mayComeForward(what) else { return }
                NSApp.activate(ignoringOtherApps: true)
        """))
    }

    /// What either shell would show by itself: first-run Settings (both), the Go engine's stopped
    /// alert, setup's Open the office (both) and the folder panel (a page's, through the deck or the bridge).
    func testWhatWouldShowByItselfIsCheckedFirst() throws {
        let app = try source("App.swift"), shell = try source("GoShell.swift"), deck = try source("Deck.swift")
        XCTAssertTrue(app.contains("if !Config.shared.hasCompletedSetup, TestCopy.mayComeForward(\"first-run Settings\") {"))
        XCTAssertTrue(shell.contains("if !Config.shared.hasCompletedSetup, TestCopy.mayComeForward(\"first-run Settings\") { settings?.show() }"))
        XCTAssertTrue(app.contains("if TestCopy.mayComeForward(\"the office (from setup)\") { office.show() }"))
        XCTAssertTrue(shell.contains("if TestCopy.mayComeForward(\"the office (from setup)\") { self?.window?.show() }"))
        XCTAssertTrue(deck.contains("guard TestCopy.mayComeForward(\"the folder panel\") else { return done(nil) }"))

        // The engine-stopped alert: checked before it's made, and its windows closed either way.
        let stopped = try XCTUnwrap(shell.range(of: "private func coreStopped")).upperBound
        let body = String(shell[stopped...].prefix(1200))
        let check = try XCTUnwrap(body.range(of: "guard TestCopy.mayComeForward(\"the alert “\\(said)”\") else { return }"))
        XCTAssertLessThan(try XCTUnwrap(body.range(of: "settings = nil")).lowerBound, check.lowerBound, "windows closed first")
        XCTAssertLessThan(check.lowerBound, try XCTUnwrap(body.range(of: "NSAlert()")).lowerBound)
    }

    // MARK: This Mac only

    func testATestCopyListensOnThisMacUnlessTheLANIsAskedFor() {
        XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: [:]), .thisMac)
        XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: ["SMOLCORP_TEST_LISTEN": "lan"]), .everywhere)
        XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: ["SMOLCORP_TEST_LISTEN": "LAN"]), .everywhere)
        XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: ["SMOLCORP_TEST_LISTEN": "all"]), .thisMac, "only lan")
        XCTAssertEqual(TestCopy.listenScope(testCopy: false, environment: [:]), .everywhere, "the real app, as always")
        XCTAssertEqual(TestCopy.listenScope(testCopy: false, environment: ["SMOLCORP_TEST_LISTEN": "lan"]), .everywhere)
        XCTAssertEqual(TestCopy.listensOn, .thisMac, "the suite's stand-in servers stay on this Mac")
    }

    /// At the socket: the server's listening sockets are bound to 127.0.0.1 (and ::1), never *, and it answers there.
    func testTheServerIsBoundToLoopbackOnly() throws {
        let port = UInt16.random(in: 47_100...47_900)
        let server = HTTPServer(port: port, router: Answer())
        XCTAssertEqual(server.scope, .thisMac, "the default in the suite")
        let ready = expectation(description: "listening")
        server.onReady = { ready.fulfill() }
        server.start()
        defer { server.stop() }
        wait(for: [ready], timeout: 10)
        // ::1 is started beside it: a moment for it too.
        RunLoop.main.run(until: Date().addingTimeInterval(0.3))

        let bound = try listening(on: port)
        XCTAssertTrue(bound.contains("127.0.0.1:\(port)"), "\(bound)")
        XCTAssertEqual(bound.subtracting(["127.0.0.1:\(port)", "[::1]:\(port)"]), [], "nothing but loopback: \(bound)")

        let answered = expectation(description: "answered")
        var status = 0
        URLSession.shared.dataTask(with: URL(string: "http://127.0.0.1:\(port)/")!) { _, response, _ in
            status = (response as? HTTPURLResponse)?.statusCode ?? 0
            answered.fulfill()
        }.resume()
        wait(for: [answered], timeout: 10)
        XCTAssertEqual(status, 200)
    }

    // MARK: Helpers

    private var sources: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck")
    }

    private func source(_ name: String) throws -> String {
        try String(contentsOf: sources.appendingPathComponent(name), encoding: .utf8)
    }

    /// The addresses this process listens on at `port`, as lsof names them ("127.0.0.1:47123", "[::1]:47123", "*:47123").
    private func listening(on port: UInt16) throws -> Set<String> {
        let lsof = Process()
        lsof.executableURL = URL(fileURLWithPath: "/usr/sbin/lsof")
        lsof.arguments = ["-nP", "-a", "-p", String(getpid()), "-iTCP:\(port)", "-sTCP:LISTEN", "-Fn"]
        let out = Pipe()
        lsof.standardOutput = out
        lsof.standardError = FileHandle.nullDevice
        try lsof.run()
        let data = out.fileHandleForReading.readDataToEndOfFile()
        lsof.waitUntilExit()
        let lines = String(decoding: data, as: UTF8.self).components(separatedBy: "\n")
        return Set(lines.filter { $0.hasPrefix("n") }.map { String($0.dropFirst()) })
    }

    private final class Answer: Router {
        func handle(_ request: Request, reply: @escaping (Reply) -> Void) {
            reply(.response(.status(200, "ok")))
        }
    }
}

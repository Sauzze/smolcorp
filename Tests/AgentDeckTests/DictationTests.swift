import XCTest
@testable import AgentDeck

/// Dictation's press and release, with a stand-in for the microphone.
final class DictationTests: XCTestCase {
    /// A recording that takes `ready` seconds to be up, says `text`, and is noted when finished.
    final class StandIn: DictationRecording {
        let text: String
        var finished = false
        init(text: String) { self.text = text }
        func finish() async -> String { finished = true; return text }
    }

    private var made: [StandIn] = []
    private var saved: (TimeInterval, (String) async throws -> Dictation.Recording)!

    override func setUp() {
        saved = (Dictation.longest, Dictation.makeRecording)
        made = []
    }

    override func tearDown() {
        (Dictation.longest, Dictation.makeRecording) = saved
        // Whatever a test left running ends, so the next starts idle.
        let ended = expectation(description: "idle")
        Dictation.stop { _, _ in ended.fulfill() }
        wait(for: [ended], timeout: 2)
    }

    private func standIns(ready: TimeInterval, text: String = "ajoute un test") {
        Dictation.makeRecording = { [unowned self] _ in
            try await Task.sleep(for: .seconds(ready))
            let recording = StandIn(text: text)
            await MainActor.run { self.made.append(recording) }
            return recording
        }
    }

    private func start(phone: String = "p1", deliver: @escaping (String, String) -> Void = { _, _ in }) -> Bool {
        let answered = expectation(description: "started")
        var started = false
        Dictation.start(language: "fr_FR", for: "tty:/dev/ttys003", phone: phone, deliver: deliver) { started = $0; answered.fulfill() }
        wait(for: [answered], timeout: 3)
        return started
    }

    func testAQuickTapLeavesNothingRecording() {
        standIns(ready: 0.3)
        let answered = expectation(description: "started")
        Dictation.start(language: "fr_FR", for: "tty:/dev/ttys003", phone: "p1", deliver: { _, _ in }) { _ in answered.fulfill() }
        // Let go before the microphone is up.
        let stopped = expectation(description: "stopped")
        Dictation.stop { text, tab in
            XCTAssertEqual(text, "")
            XCTAssertEqual(tab, "tty:/dev/ttys003")
            stopped.fulfill()
        }
        wait(for: [stopped, answered], timeout: 3)
        XCTAssertFalse(Dictation.isActive, "the microphone isn't left on")
        XCTAssertEqual(made.count, 1)
        let done = expectation(description: "finished")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { done.fulfill() }
        wait(for: [done], timeout: 1)
        XCTAssertTrue(made[0].finished, "the recording that came up after the release was stopped")
        XCTAssertTrue(start(), "the next press starts")
    }

    func testStopAlwaysAnswers() {
        let answered = expectation(description: "answered")
        Dictation.stop { text, _ in XCTAssertEqual(text, ""); answered.fulfill() }
        wait(for: [answered], timeout: 1)
    }

    func testARecordingStopsByItselfAtTheCap() {
        standIns(ready: 0)
        Dictation.longest = 0.3
        let delivered = expectation(description: "delivered")
        XCTAssertTrue(start { text, tab in
            XCTAssertEqual(text, "ajoute un test")
            XCTAssertEqual(tab, "tty:/dev/ttys003")
            delivered.fulfill()
        })
        wait(for: [delivered], timeout: 3)
        XCTAssertFalse(Dictation.isActive)
    }

    func testItStopsWhenItsPhoneGoes() {
        standIns(ready: 0)
        let delivered = expectation(description: "delivered")
        XCTAssertTrue(start(phone: "p1") { text, _ in XCTAssertEqual(text, "ajoute un test"); delivered.fulfill() })
        Dictation.phoneLeft("p2")
        XCTAssertTrue(Dictation.isActive, "another phone going changes nothing")
        Dictation.phoneLeft("p1")
        wait(for: [delivered], timeout: 3)
        XCTAssertFalse(Dictation.isActive)
    }

    func testOneRecordingAtATime() {
        standIns(ready: 0)
        XCTAssertTrue(start())
        XCTAssertFalse(start(phone: "p2"))
    }
}

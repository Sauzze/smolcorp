import XCTest
@testable import AgentDeck

/// The jukebox's volume bar sends a level, and the device only ever gets one from 0 to 100.
final class MusicVolumeTests: XCTestCase {
    func testALevelIsClampedBeforeItReachesTheDevice() {
        XCTAssertEqual(Music.clampedVolume(-3), 0)
        XCTAssertEqual(Music.clampedVolume(55), 55)
        XCTAssertEqual(Music.clampedVolume(250), 100)
    }
}

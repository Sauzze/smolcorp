import XCTest
@testable import AgentDeck

/// The jukebox's volume bar sends a level from 0 to 100, and nothing else gets through.
final class MusicVolumeTests: XCTestCase {
    func testALevelIsAWholeNumberFrom0To100() {
        XCTAssertEqual(Music.volumeLevel("0"), 0)
        XCTAssertEqual(Music.volumeLevel("42"), 42)
        XCTAssertEqual(Music.volumeLevel("100"), 100)
        for bad in [nil, "", "-1", "101", "4.5", "loud", " 5"] {
            XCTAssertNil(Music.volumeLevel(bad), "\(bad ?? "nil")")
        }
    }

    func testALevelIsClampedBeforeItReachesTheDevice() {
        XCTAssertEqual(Music.clampedVolume(-3), 0)
        XCTAssertEqual(Music.clampedVolume(55), 55)
        XCTAssertEqual(Music.clampedVolume(250), 100)
    }
}

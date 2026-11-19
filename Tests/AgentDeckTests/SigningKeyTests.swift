import Security
import XCTest
@testable import AgentDeck

/// Setup's advice on the key Smolcorp is signed with (SD1): asked through the gate, never with a prompt, and never
/// under the tests, which only ever give it a list of keychains of their own (here, none).
final class SigningKeyTests: XCTestCase {
    /// Shut, the gate asks nothing and says nothing: not said, rather than "not unlocked".
    func testTheGateShutSaysNothing() {
        XCTAssertNil(SigningKey.unlocked(Data([1, 2, 3]), in: [], open: false))
    }

    /// No unlocked keychain to look in: no, without a query.
    func testNoKeychainIsNo() {
        XCTAssertEqual(SigningKey.unlocked(Data([1, 2, 3]), in: [], open: true), false)
    }

    /// Under the tests the fact is null, whatever the app's signature: the owner's keychains are never looked at.
    func testTheTestsLeaveTheKeychainAlone() {
        XCTAssertTrue(SigningKey.fact(testCopy: false) is NSNull)
        XCTAssertTrue(SigningKey.fact(testCopy: true) is NSNull)
    }
}

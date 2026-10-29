import Network
import XCTest
@testable import AgentDeck

/// The iPhone app (personal build): how setup names it, how the deck says
/// it's on the Wi-Fi, and what `/glossary` gives it. Offline.
final class PhoneAppTests: XCTestCase {
    func testSetupNamesTheAppApartFromSafari() {
        XCTAssertEqual(Phones.name(forUserAgent: "Smolcorp iOS"), "iPhone app")
        XCTAssertEqual(Phones.name(forUserAgent: "Smolcorp iOS CFNetwork/3860 Darwin/27.2.0"), "iPhone app")
        XCTAssertEqual(Phones.name(forUserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 27_2 like Mac OS X)"), "iPhone")
    }

    func testTheDeckIsAdvertisedAsSmolcorpWithItsAPIOnly() {
        let service = PhoneApp.service
        XCTAssertEqual(service.type, "_smolcorp._tcp")
        guard let txt = service.txtRecordObject else { return XCTFail("no TXT record") }
        XCTAssertEqual(txt.dictionary, ["api": String(PhoneApp.api)])
    }

    func testTheGlossaryIsSetupsThenTheDesks() {
        let answer = PhoneApp.glossary(desks: ["moonbase", "Smolcorp", "new-project"])
        let names = answer["glossary"] as? [String] ?? []
        XCTAssertEqual(Array(names.prefix(Config.shared.dictationGlossary.count)), Config.shared.dictationGlossary)
        XCTAssertTrue(names.contains("new-project"))
        // Already there, whatever its case: not twice.
        XCTAssertEqual(names.filter { $0.lowercased() == "smolcorp" }.count, 1)
        XCTAssertEqual(answer["language"] as? String, Config.shared.dictationLanguage)
    }
}

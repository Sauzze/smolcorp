import Foundation
import Network

/// Personal build: what the iPhone app (ios/) needs of the deck beyond what
/// the deck's page has. It pairs and reads the deck as the page does, with
/// the same key; it finds the deck on the Wi-Fi by Bonjour, and asks for the
/// names its own speech recognition should listen for.
enum PhoneApp {
    /// The app's User-Agent starts so, for setup to name it.
    static let userAgent = "Smolcorp iOS"
    static let serviceType = "_smolcorp._tcp"
    /// What the app reads, raised when that changes.
    static let api = 1

    /// The deck on the Wi-Fi: this Mac's name, or a test copy's own, so the
    /// app tells them apart. Only says it's here and which API it speaks.
    static var service: NWListener.Service {
        NWListener.Service(name: TestCopy.isOn ? "Smolcorp Test" : nil, type: serviceType, domain: nil,
                           txtRecord: NWTXTRecord(["api": String(api)]))
    }

    /// The names the phone's speech recognition is told to expect: setup's
    /// glossary, then the desks' project names, as dictation's clean-up has
    /// them; and dictation's language, the mic's FR/EN switch.
    static func glossary(desks: [String]) -> [String: Any] {
        ["glossary": DictationCleanup.glossary(configured: Config.shared.dictationGlossary, desks: desks),
         "language": Config.shared.dictationLanguage, "api": api]
    }
}

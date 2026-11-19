import Foundation

/// What the Mac app keeps of its hire desk for the Go core, which hires now:
/// the folder as the bridge checks it (`physical`), and the API-key account's
/// words and gate, which only this app can answer, as it holds the Keychain
/// (`HostBridge`'s api-key actions).
enum HireDesk {
    /// Why a hire on the API key was refused, with the status it answers.
    struct Refusal: Error {
        let status: Int
        let message: String
    }

    /// What setup says beside the API-key choice.
    static let apiKeyNotice = "Billed to your Anthropic API key (the one saved under Summaries), not your Claude subscription. The hire can read this key."
    /// The same in a test copy, where the key is the copy's own file and the Keychain is never asked.
    static let testCopyApiKeyNotice = "Billed to the Anthropic API key in this test copy's \(TestCopySecrets.apiKeyName) file (or the one pasted under Summaries), not your Claude subscription. The hire can read this key."
    static func apiKeyNoticeText(testCopy: Bool = TestCopy.isOn) -> String { testCopy ? testCopyApiKeyNotice : apiKeyNotice }

    static let keyRefusal = Refusal(status: 409, message: "Smolcorp couldn't read your Anthropic API key from the Keychain. Save it again under Summaries, or hire on a Claude account.")

    /// How a test copy is told to put a fake key in its own file: the Keychain isn't mentioned, as the copy never asks it.
    static func testCopyKeyHelp(file: URL?) -> String {
        let named = file.map { "\($0.path)" } ?? TestCopySecrets.apiKeyName
        return "Write a fake key (sk-ant-… on one line, mode 600) into \(named), or paste one under Summaries. No restart is needed."
    }

    static func keyRefusalText(testCopy: Bool = TestCopy.isOn, file: URL? = TestCopy.support?.appendingPathComponent(TestCopySecrets.apiKeyName)) -> Refusal {
        guard testCopy else { return keyRefusal }
        return Refusal(status: 409, message: "Smolcorp couldn't read an Anthropic API key from this test copy's \(TestCopySecrets.apiKeyName) file. \(testCopyKeyHelp(file: file))")
    }

    /// Whether the choice is offered: only with a key in the Keychain (not just the environment).
    static func apiKeyOffered() -> Bool { SummaryKey.shown()?.source == .keychain }

    /// Why the API-key account can't be used for a hire, or nil.
    static func apiKeyRefusal(picked: Bool, offered: Bool, testCopy: Bool = TestCopy.isOn,
                              file: URL? = TestCopy.support?.appendingPathComponent(TestCopySecrets.apiKeyName)) -> Refusal? {
        guard picked && !offered else { return nil }
        if testCopy {
            return Refusal(status: 409, message: "There's no Anthropic API key in this test copy's \(TestCopySecrets.apiKeyName) file. \(testCopyKeyHelp(file: file))")
        }
        return Refusal(status: 409, message: "There's no Anthropic API key in your Keychain. Save one under Summaries, or hire on a Claude account.")
    }

    /// `path` with symlinks resolved, /tmp as /private/tmp: how Claude Code knows a folder.
    static func physical(_ path: String) -> String {
        guard let resolved = realpath(path, nil) else { return path }
        defer { free(resolved) }
        return String(cString: resolved)
    }
}

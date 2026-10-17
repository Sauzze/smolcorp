import AppKit

/// The Mac's music, for the office's jukeboxes: play, pause and skip with
/// the keyboard's media keys, which macOS sends to whatever is playing (Spotify,
/// Music, a video in a browser), and whether Spotify or Music is playing,
/// the two that say so when asked. Pressing the key needs Accessibility, as
/// the deck's other keys do; asking an app needs your OK once, for each.
enum Music {
    /// What a jukebox's keys do, as the office names them, and each one's
    /// code among the keyboard's special keys.
    static let keys = ["playpause": 16, "next": 17, "previous": 18]

    /// Presses one of `keys`, down then up, as the keyboard's media key does.
    static func press(_ name: String) {
        guard let key = keys[name] else { return }
        for down in [true, false] {
            let flags = down ? 0xa00 : 0xb00
            let event = NSEvent.otherEvent(with: .systemDefined, location: .zero, modifierFlags: NSEvent.ModifierFlags(rawValue: UInt(flags)),
                                           timestamp: 0, windowNumber: 0, context: nil, subtype: 8,
                                           data1: (key << 16) | flags, data2: -1)
            event?.cgEvent?.post(tap: .cghidEventTap)
        }
    }

    /// True while Spotify or Music is playing, nil otherwise: paused, or
    /// playing somewhere that doesn't say, such as a browser. An app that
    /// isn't open isn't asked, so it isn't opened.
    static func isPlaying() -> Bool? {
        for app in ["Spotify", "Music"] where NSRunningApplication.runningApplications(withBundleIdentifier: bundle(of: app)).count > 0 {
            if ask("tell application \"\(app)\" to return player state as text") == "playing" { return true }
        }
        return nil
    }

    private static func bundle(of app: String) -> String { app == "Spotify" ? "com.spotify.client" : "com.apple.Music" }

    private static func ask(_ script: String) -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        process.arguments = ["-e", script]
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else { return nil }
        return String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

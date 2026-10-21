import AppKit
import AudioToolbox
import CoreAudio

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

/// The Mac's output volume, for the jukebox's volume bar: the default output
/// device's, as the keyboard's volume keys set it. Unlike the media keys this
/// needs no Accessibility; CoreAudio lets any app set it.
extension Music {
    /// The volume, 0 to 100, and whether the device is muted, or nil where
    /// the device has no volume to set (some HDMI and USB outputs).
    static func volume() -> (level: Int, muted: Bool)? {
        guard let device = outputDevice(), let scalar = volumeScalar(of: device) else { return nil }
        return (Int((scalar * 100).rounded()), isMuted(device))
    }

    /// Sets the volume, 0 to 100. A level above 0 unmutes, as the volume keys
    /// do; 0 is simply 0, and leaves mute as it is.
    static func setVolume(_ level: Int) {
        guard let device = outputDevice() else { return }
        var scalar = Float32(clampedVolume(level)) / 100
        var address = volumeAddress
        guard AudioObjectHasProperty(device, &address) else { return }
        AudioObjectSetPropertyData(device, &address, 0, nil, UInt32(MemoryLayout<Float32>.size), &scalar)
        if level > 0 {
            var mute: UInt32 = 0
            var muteAddress = self.muteAddress
            if AudioObjectHasProperty(device, &muteAddress) {
                AudioObjectSetPropertyData(device, &muteAddress, 0, nil, UInt32(MemoryLayout<UInt32>.size), &mute)
            }
        }
    }

    /// A level from the office, read as an integer from 0 to 100, or nil for
    /// anything else.
    static func volumeLevel(_ text: String?) -> Int? {
        guard let text, let level = Int(text), (0...100).contains(level) else { return nil }
        return level
    }

    static func clampedVolume(_ level: Int) -> Int { min(max(level, 0), 100) }

    /// Calls `onChange` on the main queue when the volume or mute changes,
    /// from the keyboard or anywhere else, or when the output moves to another
    /// device. A burst (holding a volume key) comes as one call.
    static func watchVolume(_ onChange: @escaping () -> Void) {
        guard watcher == nil else { return }
        let watcher = VolumeWatcher(onChange: onChange)
        self.watcher = watcher
        watcher.start()
    }

    private static var watcher: VolumeWatcher?

    fileprivate static var volumeAddress = AudioObjectPropertyAddress(
        mSelector: kAudioHardwareServiceDeviceProperty_VirtualMainVolume,
        mScope: kAudioDevicePropertyScopeOutput, mElement: kAudioObjectPropertyElementMain)
    fileprivate static var muteAddress = AudioObjectPropertyAddress(
        mSelector: kAudioDevicePropertyMute,
        mScope: kAudioDevicePropertyScopeOutput, mElement: kAudioObjectPropertyElementMain)
    fileprivate static var defaultOutputAddress = AudioObjectPropertyAddress(
        mSelector: kAudioHardwarePropertyDefaultOutputDevice,
        mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)

    fileprivate static func outputDevice() -> AudioObjectID? {
        var device = AudioObjectID(kAudioObjectUnknown)
        var size = UInt32(MemoryLayout<AudioObjectID>.size)
        var address = defaultOutputAddress
        let status = AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &address, 0, nil, &size, &device)
        return status == noErr && device != kAudioObjectUnknown ? device : nil
    }

    /// The device's volume, 0 to 1, only where it can also be set: a bar
    /// that moves nothing would be worse than none.
    private static func volumeScalar(of device: AudioObjectID) -> Float32? {
        var address = volumeAddress
        var settable: DarwinBoolean = false
        guard AudioObjectHasProperty(device, &address),
              AudioObjectIsPropertySettable(device, &address, &settable) == noErr, settable.boolValue else { return nil }
        var scalar: Float32 = 0
        var size = UInt32(MemoryLayout<Float32>.size)
        guard AudioObjectGetPropertyData(device, &address, 0, nil, &size, &scalar) == noErr else { return nil }
        return min(max(scalar, 0), 1)
    }

    private static func isMuted(_ device: AudioObjectID) -> Bool {
        var address = muteAddress
        guard AudioObjectHasProperty(device, &address) else { return false }
        var mute: UInt32 = 0
        var size = UInt32(MemoryLayout<UInt32>.size)
        guard AudioObjectGetPropertyData(device, &address, 0, nil, &size, &mute) == noErr else { return false }
        return mute != 0
    }
}

/// Listens to the default output device's volume and mute, and to which
/// device is the default, moving the first two along when it changes.
private final class VolumeWatcher {
    private let onChange: () -> Void
    private var device: AudioObjectID?
    private var pending = false
    private lazy var listener: AudioObjectPropertyListenerBlock = { [weak self] _, _ in self?.heard() }
    private lazy var deviceListener: AudioObjectPropertyListenerBlock = { [weak self] _, _ in
        self?.follow()
        self?.heard()
    }

    init(onChange: @escaping () -> Void) { self.onChange = onChange }

    func start() {
        var address = Music.defaultOutputAddress
        AudioObjectAddPropertyListenerBlock(AudioObjectID(kAudioObjectSystemObject), &address, .main, deviceListener)
        follow()
    }

    /// Moves the volume and mute listeners to whatever device is the default now.
    private func follow() {
        if let device {
            var volume = Music.volumeAddress, mute = Music.muteAddress
            AudioObjectRemovePropertyListenerBlock(device, &volume, .main, listener)
            AudioObjectRemovePropertyListenerBlock(device, &mute, .main, listener)
        }
        device = Music.outputDevice()
        guard let device else { return }
        var volume = Music.volumeAddress, mute = Music.muteAddress
        if AudioObjectHasProperty(device, &volume) { AudioObjectAddPropertyListenerBlock(device, &volume, .main, listener) }
        if AudioObjectHasProperty(device, &mute) { AudioObjectAddPropertyListenerBlock(device, &mute, .main, listener) }
    }

    /// Holding a volume key sends a change every few milliseconds, and a
    /// drag on the bar does too: the deck hears about them once, a moment later.
    private func heard() {
        guard !pending else { return }
        pending = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) { [weak self] in
            self?.pending = false
            self?.onChange()
        }
    }
}

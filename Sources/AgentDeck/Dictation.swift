import AVFoundation
import Speech

/// Dictation for agents with no voice mode of their own: while the phone's
/// mic is held, the Mac records from its own microphone and turns what you
/// say into text, on this Mac only, with Apple's speech engine (macOS 26 and
/// later). Let go, and the text is typed into the agent's tab without Return,
/// for you to read before sending. Nothing is kept: no audio, no text.
enum Dictation {
    /// The languages it takes, as setup and the phone's quick switch name them.
    static let languages = ["fr_FR", "en_US"]

    /// Whether this Mac can dictate at all: macOS 26 and its speech engine.
    static var isAvailable: Bool {
        if #available(macOS 26, *) { return SpeechTranscriber.isAvailable }
        return false
    }

    static var microphoneAllowed: Bool { AVCaptureDevice.authorizationStatus(for: .audio) == .authorized }

    /// Whether dictation is on and can run: turned on in setup, available,
    /// and allowed the microphone.
    static var isOn: Bool { Config.shared.dictationEnabled && isAvailable && microphoneAllowed }

    /// The language the Mac's own settings suggest, before one is chosen.
    static var suggestedLanguage: String {
        Locale.current.language.languageCode?.identifier == "fr" ? "fr_FR" : "en_US"
    }

    /// Asks for the microphone, as setup's button does: never from the phone,
    /// as the Mac may be locked with nobody at it to answer.
    static func askForMicrophone(done: @escaping (Bool) -> Void) {
        AVCaptureDevice.requestAccess(for: .audio) { allowed in DispatchQueue.main.async { done(allowed) } }
    }

    /// Gets Apple's model for `language` ready, downloading it the first
    /// time, and says how that went.
    static func prepare(_ language: String, done: @escaping (String?) -> Void) {
        guard #available(macOS 26, *) else { return done("Dictation needs macOS 26 or later.") }
        Task {
            let problem = await Recorder.install(language)
            await MainActor.run { done(problem) }
        }
    }

    /// The longest a recording runs: held longer, it stops and its text goes in.
    static var longest: TimeInterval = 60

    /// A recording, as Dictation drives it: Apple's own, or a stand-in in tests.
    typealias Recording = AnyObject & DictationRecording

    /// Makes the recording for a language. Settable for tests.
    static var makeRecording: (String) async throws -> Recording = { language in
        guard #available(macOS 26, *) else { throw CocoaError(.featureUnsupported) }
        return try await Recorder(language: language)
    }

    private enum State {
        case idle
        /// Getting the microphone and the model ready. Let go meanwhile, it stops as soon as it's up, with nothing typed.
        case starting(tab: String, phone: String, cancelled: Bool)
        case recording(Recording, tab: String, phone: String)
    }

    private static var state = State.idle
    /// Where the text goes when the recording stops by itself: at the cap, or as its phone goes away.
    private static var deliver: (String, String) -> Void = { _, _ in }
    private static var cap: DispatchWorkItem?

    /// Starts recording for the agent in `tab`, pressed on `phone`; `deliver`
    /// takes the text and tab if it stops by itself. `done` says whether it's
    /// starting: false if a recording is already under way. Main thread.
    static func start(language: String, for tab: String, phone: String, deliver: @escaping (String, String) -> Void,
                      done: @escaping (Bool) -> Void) {
        guard case .idle = state else { return done(false) }
        state = .starting(tab: tab, phone: phone, cancelled: false)
        Dictation.deliver = deliver
        Task {
            let recording = try? await makeRecording(language)
            await MainActor.run {
                guard case .starting(let tab, let phone, let cancelled) = state else { return }
                guard let recording, !cancelled else {
                    // Let go before it was up, or it couldn't start: nothing is typed.
                    state = .idle
                    if let recording { Task { _ = await recording.finish() } }
                    return
                }
                state = .recording(recording, tab: tab, phone: phone)
                let stop = DispatchWorkItem { Dictation.stop(done: Dictation.deliver) }
                cap = stop
                DispatchQueue.main.asyncAfter(deadline: .now() + longest, execute: stop)
            }
            await MainActor.run { done(recording != nil) }
        }
    }

    /// Stops recording and gives what was said and the tab it's for: always,
    /// with "" when nothing was, or it was let go before it was up. Main thread.
    static func stop(done: @escaping (_ text: String, _ tab: String) -> Void) {
        cap?.cancel()
        cap = nil
        switch state {
        case .idle:
            done("", "")
        case .starting(let tab, let phone, _):
            state = .starting(tab: tab, phone: phone, cancelled: true)
            done("", tab)
        case .recording(let recording, let tab, _):
            state = .idle
            Task {
                let text = await recording.finish()
                await MainActor.run { done(text, tab) }
            }
        }
    }

    /// The phone that pressed has gone, asleep or off the Wi-Fi: what it said goes in, and the microphone stops.
    static func phoneLeft(_ phone: String) {
        switch state {
        case .starting(_, let pressed, _) where pressed == phone, .recording(_, _, let pressed) where pressed == phone:
            stop(done: deliver)
        default:
            break
        }
    }

    /// Whether a recording is starting or under way.
    static var isActive: Bool {
        if case .idle = state { return false }
        return true
    }
}

/// What Dictation needs of a recording: to stop, and give the text.
protocol DictationRecording {
    func finish() async -> String
}

/// One recording: the Mac's microphone into Apple's on-device transcriber.
@available(macOS 26, *)
private final class Recorder: DictationRecording {
    private let engine = AVAudioEngine()
    private let analyzer: SpeechAnalyzer
    private let feed: AsyncStream<AnalyzerInput>.Continuation
    private let reading: Task<String, Never>

    /// The model for `language`, downloaded if it isn't on this Mac yet:
    /// nil if all went well, otherwise what went wrong.
    static func install(_ language: String) async -> String? {
        let transcriber = SpeechTranscriber(locale: Locale(identifier: language), transcriptionOptions: [], reportingOptions: [], attributeOptions: [])
        do {
            if let request = try await AssetInventory.assetInstallationRequest(supporting: [transcriber]) {
                try await request.downloadAndInstall()
            }
            return nil
        } catch {
            return "Couldn't get the \(language) speech model: \(error.localizedDescription)"
        }
    }

    init(language: String) async throws {
        let transcriber = SpeechTranscriber(locale: Locale(identifier: language), transcriptionOptions: [], reportingOptions: [], attributeOptions: [])
        analyzer = SpeechAnalyzer(modules: [transcriber])
        guard let format = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber]) else { throw CocoaError(.featureUnsupported) }
        let (input, feed) = AsyncStream<AnalyzerInput>.makeStream()
        self.feed = feed
        reading = Task {
            var text = ""
            do {
                for try await result in transcriber.results where result.isFinal { text += String(result.text.characters) }
            } catch {}
            return text
        }
        try await analyzer.prepareToAnalyze(in: format)
        try await analyzer.start(inputSequence: input)

        let source = engine.inputNode.outputFormat(forBus: 0)
        guard let converter = AVAudioConverter(from: source, to: format) else { throw CocoaError(.featureUnsupported) }
        engine.inputNode.installTap(onBus: 0, bufferSize: 4096, format: source) { buffer, _ in
            let capacity = AVAudioFrameCount(Double(buffer.frameLength) * format.sampleRate / source.sampleRate) + 1
            guard let converted = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: capacity) else { return }
            var given = false
            converter.convert(to: converted, error: nil) { _, status in
                if given { status.pointee = .noDataNow; return nil }
                given = true
                status.pointee = .haveData
                return buffer
            }
            feed.yield(AnalyzerInput(buffer: converted))
        }
        try engine.start()
    }

    /// Stops the microphone and gives the text, once the last of it is read.
    func finish() async -> String {
        engine.inputNode.removeTap(onBus: 0)
        engine.stop()
        feed.finish()
        try? await analyzer.finalizeAndFinishThroughEndOfInput()
        return await reading.value.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

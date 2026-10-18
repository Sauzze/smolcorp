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

    /// The recording under way, if any, and the tab its text goes to.
    private static var current: (recorder: AnyObject, tab: String)?

    /// Starts recording for the agent in `tab`. False if it couldn't start.
    static func start(language: String, for tab: String, done: @escaping (Bool) -> Void) {
        guard #available(macOS 26, *), isOn, current == nil else { return done(false) }
        Task {
            let recorder = try? await Recorder(language: language)
            await MainActor.run {
                if let recorder { current = (recorder, tab) }
                done(recorder != nil)
            }
        }
    }

    /// Stops recording and gives what was said, and the tab it's for.
    static func stop(done: @escaping (_ text: String, _ tab: String) -> Void) {
        guard #available(macOS 26, *), let (recorder, tab) = current, let recording = recorder as? Recorder else { return }
        current = nil
        Task {
            let text = await recording.finish()
            await MainActor.run { done(text, tab) }
        }
    }

    static var isRecording: Bool { current != nil }
}

/// One recording: the Mac's microphone into Apple's on-device transcriber.
@available(macOS 26, *)
private final class Recorder {
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

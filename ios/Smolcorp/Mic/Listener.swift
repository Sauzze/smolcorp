@preconcurrency import AVFoundation
import Foundation
import Observation
import Speech

/// The phone's own ears: the microphone, heard on this iPhone by Apple's
/// dictation model, told the names to expect (the Mac's glossary). Nothing
/// is recorded or kept; only the text leaves, and only once you say so.
@MainActor
@Observable
final class Listener {
    /// What's been heard so far: settled words, then the guess at the rest.
    private(set) var text = ""
    private(set) var listening = false

    private var engine: AVAudioEngine?
    private var analyzer: SpeechAnalyzer?
    private var input: AsyncStream<AnalyzerInput>.Continuation?
    private var results: Task<Void, Never>?
    private var settled = ""
    private var guess = ""

    enum Problem: Error, CustomStringConvertible {
        case microphoneRefused, unsupported(String), failed(String)
        var description: String {
            switch self {
            case .microphoneRefused: return "The microphone is off for Smolcorp: turn it on in Settings › Apps › Smolcorp."
            case .unsupported(let language): return "This iPhone can't take dictation in \(language)."
            case .failed(let why): return "The mic stopped: \(why)"
            }
        }
    }

    /// Asked once, ever: iOS remembers the answer.
    static func microphoneAllowed() async -> Bool {
        switch AVAudioApplication.shared.recordPermission {
        case .granted: return true
        case .denied: return false
        default: return await AVAudioApplication.requestRecordPermission()
        }
    }

    /// Starts listening in `language` (`fr_FR`, `en_US`), expecting `names`.
    /// `heard` gets the text as it grows.
    func start(language: String, names: [String], heard: @escaping (String) -> Void) async throws {
        cancel()
        guard await Listener.microphoneAllowed() else { throw Problem.microphoneRefused }
        let wanted = Locale(identifier: language)
        guard let locale = await DictationTranscriber.supportedLocale(equivalentTo: wanted) else {
            throw Problem.unsupported(wanted.localizedString(forIdentifier: language) ?? language)
        }
        let transcriber = DictationTranscriber(locale: locale, preset: .progressiveLongDictation)
        if let install = try await AssetInventory.assetInstallationRequest(supporting: [transcriber]) {
            try await install.downloadAndInstall()
        }
        let analyzer = SpeechAnalyzer(modules: [transcriber])
        let context = AnalysisContext()
        context.contextualStrings[.general] = names
        try await analyzer.setContext(context)
        guard let format = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber]) else {
            throw Problem.failed("no audio format")
        }

        let session = AVAudioSession.sharedInstance()
        try session.setCategory(.playAndRecord, mode: .measurement, options: [.mixWithOthers, .defaultToSpeaker, .allowBluetoothHFP])
        try session.setActive(true)

        let (stream, input) = AsyncStream<AnalyzerInput>.makeStream()
        let engine = AVAudioEngine()
        let node = engine.inputNode
        let natural = node.outputFormat(forBus: 0)
        guard let converter = AVAudioConverter(from: natural, to: format) else { throw Problem.failed("no converter") }
        node.installTap(onBus: 0, bufferSize: 4096, format: natural) { buffer, _ in
            guard let converted = Listener.convert(buffer, with: converter, to: format) else { return }
            input.yield(AnalyzerInput(buffer: converted))
        }

        settled = ""
        guess = ""
        text = ""
        results = Task { [weak self] in
            do {
                for try await result in transcriber.results {
                    guard let self else { return }
                    let words = String(result.text.characters)
                    if result.isFinal {
                        self.settled += words
                        self.guess = ""
                    } else {
                        self.guess = words
                    }
                    self.text = (self.settled + self.guess).trimmingCharacters(in: .whitespaces)
                    heard(self.text)
                }
            } catch {}
        }

        engine.prepare()
        try engine.start()
        try await analyzer.start(inputSequence: stream)
        self.engine = engine
        self.analyzer = analyzer
        self.input = input
        listening = true
    }

    /// Let go: the last words come in, then the whole text.
    func finish() async -> String {
        guard listening else { return text }
        listening = false
        engine?.inputNode.removeTap(onBus: 0)
        engine?.stop()
        input?.finish()
        try? await analyzer?.finalizeAndFinishThroughEndOfInput()
        await results?.value
        release()
        return text
    }

    /// Dropped without a word kept.
    func cancel() {
        listening = false
        engine?.inputNode.removeTap(onBus: 0)
        engine?.stop()
        input?.finish()
        results?.cancel()
        if let analyzer { Task { await analyzer.cancelAndFinishNow() } }
        release()
        text = ""
    }

    private func release() {
        engine = nil
        analyzer = nil
        input = nil
        results = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }

    nonisolated private static func convert(_ buffer: AVAudioPCMBuffer, with converter: AVAudioConverter,
                                            to format: AVAudioFormat) -> AVAudioPCMBuffer? {
        if buffer.format == format { return buffer }
        let ratio = format.sampleRate / buffer.format.sampleRate
        let capacity = AVAudioFrameCount((Double(buffer.frameLength) * ratio).rounded(.up)) + 16
        guard let out = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: capacity) else { return nil }
        var given = false
        var error: NSError?
        converter.convert(to: out, error: &error) { _, status in
            if given {
                status.pointee = .noDataNow
                return nil
            }
            given = true
            status.pointee = .haveData
            return buffer
        }
        return error == nil ? out : nil
    }
}

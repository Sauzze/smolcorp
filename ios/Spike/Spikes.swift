import AVFoundation
import Foundation
import Network
import Observation
import Speech
import UIKit
import WebKit

/// Phase 0: the risky parts, tried on the phone itself against the mock deck
/// (ios/scripts/mock-deck.py), each result kept in Documents/spike.json for
/// the Mac to copy off. Started with launch arguments:
///
///   -spikeHost 192.168.x.y -spikeLocal mac.local -spikePin <sha256> -spikeCode 123456
@MainActor
@Observable
final class Spikes {
    struct Result: Codable, Identifiable {
        var id: String { name }
        let name: String
        let ok: Bool
        let detail: String
        let seconds: Double
    }

    private(set) var results: [Result] = []
    private(set) var running = ""
    let office = OfficeWeb(extraScripts: [Spikes.errorCatcher])

    private let defaults = UserDefaults.standard
    private var host: String { defaults.string(forKey: "spikeHost") ?? "192.168.11.122" }
    private var local: String { defaults.string(forKey: "spikeLocal") ?? "your-mac.local" }
    private var pin: String { (defaults.string(forKey: "spikePin") ?? "").lowercased() }
    private var code: String { defaults.string(forKey: "spikeCode") ?? "123456" }
    private var only: Set<String> { Set((defaults.string(forKey: "spikeOnly") ?? "").split(separator: ",").map(String.init)) }
    private let port = 7791, tlsPort = 7792
    private var key = ""
    private var snapshots = 0
    private var latest: String?
    private var stream: URLSessionDataTask?
    private var jsErrors: [String] = []

    static let errorCatcher = """
    addEventListener('error', (e) => webkit.messageHandlers.spike.postMessage({ error: String(e.message) + ' @' + e.lineno }));
    addEventListener('unhandledrejection', (e) => webkit.messageHandlers.spike.postMessage({ error: 'rejection: ' + String(e.reason) }));
    """

    func run() async {
        office.onSpike = { [weak self] body in
            if let error = (body as? [String: Any])?["error"] as? String { self?.jsErrors.append(error) }
        }
        office.onWantsEvents = { [weak self] in
            if let latest = self?.latest { self?.office.deliver(latest) }
        }
        office.perform = { [weak self] path, query, body in
            guard let self else { return nil }
            return await self.post(path + query, body: body)
        }
        office.load()

        await step("pair") { try await self.pair() }
        await step("ats-ip") { try await self.ping("http://\(self.host):\(self.port)/ping") }
        await step("ats-local") { try await self.ping("http://\(self.local):\(self.port)/ping") }
        await step("bonjour") { try await self.bonjour() }
        await step("sse-http") { try await self.sse(URL(string: "http://\(self.host):\(self.port)/events")!, for: 14) }
        await step("tls-pin") { try await self.ping("https://\(self.host):\(self.tlsPort)/ping") }
        await step("tls-wrong-pin") { try await self.wrongPin() }
        await step("tls-sse") { try await self.sse(URL(string: "https://\(self.host):\(self.tlsPort)/events")!, for: 8) }
        await step("office") { try await self.officeCheck() }
        await step("speech") { try await self.speech() }
        await step("speech-lm") { try await self.speechWithModel() }
        running = "done"
        save()
    }

    // MARK: Steps

    private func step(_ name: String, _ work: () async throws -> String) async {
        guard only.isEmpty || only.contains(name) else { return }
        running = name
        let start = Date()
        do {
            let detail = try await work()
            results.append(Result(name: name, ok: true, detail: detail, seconds: Date().timeIntervalSince(start)))
        } catch {
            results.append(Result(name: name, ok: false, detail: "\(error)", seconds: Date().timeIntervalSince(start)))
        }
        save()
    }

    private func pair() async throws -> String {
        DeckSession.shared.delegate.pins = [host: pin]
        var request = URLRequest(url: URL(string: "http://\(host):\(port)/pair")!)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["code": code])
        let (data, response) = try await DeckSession.shared.session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard status == 200, let key = (try JSONSerialization.jsonObject(with: data) as? [String: Any])?["key"] as? String else {
            throw Problem("pair answered \(status): \(String(decoding: data, as: UTF8.self))")
        }
        guard Keychain.write(Data(key.utf8), for: "spike"), Keychain.read("spike") == Data(key.utf8) else {
            throw Problem("keychain round trip failed")
        }
        self.key = key
        return "key \(key.prefix(6))…, kept and read back from the Keychain"
    }

    private func ping(_ address: String) async throws -> String {
        let (data, response) = try await DeckSession.shared.session.data(from: URL(string: address)!)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard status == 200 else { throw Problem("status \(status)") }
        return "\(address) → \(String(decoding: data, as: UTF8.self))"
    }

    private func wrongPin() async throws -> String {
        let right = DeckSession.shared.delegate.pins
        DeckSession.shared.delegate.pins = [host: String(repeating: "0", count: 64)]
        defer { DeckSession.shared.delegate.pins = right }
        do {
            _ = try await DeckSession.shared.session.data(from: URL(string: "https://\(host):\(tlsPort)/ping")!)
        } catch {
            return "refused as it should be: \((error as? URLError)?.code.rawValue ?? 0)"
        }
        throw Problem("a wrong pin was let through")
    }

    private func bonjour() async throws -> String {
        let browser = NWBrowser(for: .bonjourWithTXTRecord(type: "_smolcorp._tcp", domain: nil), using: .tcp)
        var found: [String] = []
        var states: [String] = []
        browser.stateUpdateHandler = { states.append("\($0)") }
        browser.browseResultsChangedHandler = { results, _ in
            found = results.map { result in
                var text = "\(result.endpoint)"
                if case let .bonjour(txt) = result.metadata { text += " \(txt.dictionary)" }
                return text
            }
        }
        browser.start(queue: .main)
        for _ in 0..<40 where found.isEmpty { try await Task.sleep(for: .milliseconds(250)) }
        guard let first = browser.browseResults.first(where: { "\($0.endpoint)".contains("mock") }) ?? browser.browseResults.first else {
            browser.cancel()
            throw Problem("nothing found; states \(states)")
        }
        // Where it is: connect, and read the address the connection reached.
        let connection = NWConnection(to: first.endpoint, using: .tcp)
        let reached: String = await withCheckedContinuation { done in
            var finished = false
            connection.stateUpdateHandler = { state in
                guard !finished else { return }
                switch state {
                case .ready:
                    finished = true
                    done.resume(returning: "\(connection.currentPath?.remoteEndpoint.map { "\($0)" } ?? "?")")
                case .failed(let error), .waiting(let error):
                    finished = true
                    done.resume(returning: "not reached: \(error)")
                default: break
                }
            }
            connection.start(queue: .main)
        }
        connection.cancel()
        browser.cancel()
        return "found \(found); resolved to \(reached)"
    }

    private func sse(_ url: URL, for seconds: Double) async throws -> String {
        var request = URLRequest(url: url)
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        let start = Date()
        var first: Double?
        var messages = 0, bytes = 0
        var ended: String?
        stream?.cancel()
        stream = DeckSession.shared.stream(request, onEvent: { [weak self] event in
            if event.name == "message" {
                messages += 1
                bytes = event.data.utf8.count
                first = first ?? Date().timeIntervalSince(start)
                self?.latest = event.data
                self?.office.deliver(event.data)
            } else {
                self?.office.ping()
            }
        }, onEnd: { status, error in
            ended = "status \(status.map(String.init) ?? "-") error \(error.map { "\($0)" } ?? "-")"
        })
        try await Task.sleep(for: .seconds(seconds))
        if let ended { throw Problem("ended early: \(ended), after \(messages) messages") }
        guard messages >= 2 else { throw Problem("only \(messages) messages in \(seconds) s") }
        return String(format: "%d snapshots in %.0f s, first after %.2f s, last %d bytes; still open", messages, seconds, first ?? -1, bytes)
    }

    private func officeCheck() async throws -> String {
        // Fed by the https stream, still open from the last step.
        try await Task.sleep(for: .seconds(2))
        let facts = await office.run("""
        localStorage.setItem('spike', 'kept');
        const canvas = document.querySelector('canvas');
        return JSON.stringify({ href: location.href, secure: isSecureContext, storage: localStorage.getItem('spike'),
          office: !!window.AgentOffice, canvas: canvas ? [canvas.width, canvas.height] : null,
          status: document.getElementById('status').textContent, dpr: devicePixelRatio,
          size: [innerWidth, innerHeight], liteKept: localStorage.getItem('office-lite') });
        """) as? String ?? "nil"
        let posted = await office.run("""
        const r = await fetch('/act?t=&session=s1&action=up', { method: 'POST' });
        return r.status;
        """)
        let meter = """
        return await new Promise((done) => {
          let frames = 0, slow = 0, last = performance.now();
          const start = last;
          function frame(now) {
            frames++;
            if (now - last > 25) slow++;
            last = now;
            if (now - start < 6000) requestAnimationFrame(frame);
            else done(JSON.stringify({ fps: Math.round(frames / ((now - start) / 1000)), slowFrames: slow }));
          }
          requestAnimationFrame(frame);
        });
        """
        _ = await office.run("office.setLite(false); return 1")
        let full = await office.run(meter) as? String ?? "nil"
        await shot("office-full")
        _ = await office.run("office.setLite(true); return 1")
        let lite = await office.run(meter) as? String ?? "nil"
        await shot("office-lite")
        let thermal = ProcessInfo.processInfo.thermalState.rawValue
        return "facts \(facts); bridge POST → \(posted.map { "\($0)" } ?? "nil"); full \(full); lite \(lite); thermal \(thermal); js errors \(jsErrors)"
    }

    private func shot(_ name: String) async {
        let configuration = WKSnapshotConfiguration()
        guard let image = try? await office.webView.takeSnapshot(configuration: configuration),
              let png = image.pngData() else { return }
        try? png.write(to: Spikes.documents.appendingPathComponent("\(name).png"))
    }

    // MARK: Speech

    static let glossary = ["Smolcorp", "Claude", "Claude Code", "Codex", "Gemini", "OpenCode", "Grok", "GitHub", "Tailscale", "Cloudflare", "Vercel", "Supabase"]

    private func speech() async throws -> String {
        var lines = ["auth before: \(SFSpeechRecognizer.authorizationStatus().rawValue)"]
        lines.append("speech available: \(SpeechTranscriber.isAvailable)")
        for (file, id) in [("en", "en_US"), ("fr", "fr_FR")] {
            guard let url = Bundle.main.url(forResource: file, withExtension: "wav") else { throw Problem("no \(file).wav") }
            let locale = Locale(identifier: id)
            for (label, dictation, context) in [("speech", false, false), ("speech+ctx", false, true), ("dictation+ctx", true, true)] {
                let start = Date()
                do {
                    let text = try await transcribe(url, locale: locale, dictation: dictation, context: context ? Spikes.glossary : nil)
                    let hits = Spikes.glossary.filter { text.localizedCaseInsensitiveContains($0) }
                    lines.append(String(format: "%@ %@ (%.1f s) %d/%d [%@]: %@", file, label, Date().timeIntervalSince(start),
                                        hits.count, Spikes.glossary.count, hits.joined(separator: ","), text))
                } catch {
                    lines.append("\(file) \(label) failed: \(error)")
                }
                save(extra: lines)
            }
        }
        lines.append("auth after: \(SFSpeechRecognizer.authorizationStatus().rawValue)")
        return lines.joined(separator: "\n")
    }

    private func transcribe(_ url: URL, locale: Locale, dictation: Bool, context: [String]?) async throws -> String {
        let file = try AVAudioFile(forReading: url)
        if dictation {
            let module = DictationTranscriber(locale: locale, preset: .longDictation)
            try await install(module)
            let analyzer = SpeechAnalyzer(modules: [module])
            if let context { try await analyzer.setContext(Spikes.context(context)) }
            async let text: String = module.results.reduce(into: "") { text, result in
                if result.isFinal { text += String(result.text.characters) }
            }
            try await analyzer.start(inputAudioFile: file, finishAfterFile: true)
            return try await text
        }
        let module = SpeechTranscriber(locale: locale, preset: .transcription)
        try await install(module)
        let analyzer = SpeechAnalyzer(modules: [module])
        if let context { try await analyzer.setContext(Spikes.context(context)) }
        async let text: String = module.results.reduce(into: "") { text, result in
            if result.isFinal { text += String(result.text.characters) }
        }
        try await analyzer.start(inputAudioFile: file, finishAfterFile: true)
        return try await text
    }

    /// Dictation with a custom language model made on the phone from the glossary, and the glossary as context.
    private func speechWithModel() async throws -> String {
        var lines: [String] = []
        for (file, id) in [("en", "en_US"), ("fr", "fr_FR")] {
            guard let url = Bundle.main.url(forResource: file, withExtension: "wav") else { throw Problem("no \(file).wav") }
            let locale = Locale(identifier: id)
            let start = Date()
            let configuration = try await Spikes.model(for: locale, names: Spikes.glossary)
            let built = Date().timeIntervalSince(start)
            for weight in [nil, 0.6] as [Double?] {
                let started = Date()
                let module = DictationTranscriber(locale: locale, contentHints: [.customizedLanguage(modelConfiguration: weight.map {
                    SFSpeechLanguageModel.Configuration(languageModel: configuration.languageModel, vocabulary: configuration.vocabulary,
                                                        weight: NSNumber(value: $0))
                } ?? configuration)], transcriptionOptions: [.punctuation], reportingOptions: [], attributeOptions: [])
                try await install(module)
                let analyzer = SpeechAnalyzer(modules: [module])
                try await analyzer.setContext(Spikes.context(Spikes.glossary))
                async let text: String = module.results.reduce(into: "") { text, result in
                    if result.isFinal { text += String(result.text.characters) }
                }
                try await analyzer.start(inputAudioFile: try AVAudioFile(forReading: url), finishAfterFile: true)
                let heard = try await text
                let hits = Spikes.glossary.filter { heard.contains($0) }
                lines.append(String(format: "%@ lm weight %@ (built %.1f s, ran %.1f s) %d/%d [%@]: %@", file, weight.map { "\($0)" } ?? "default",
                                    built, Date().timeIntervalSince(started), hits.count, Spikes.glossary.count, hits.joined(separator: ","), heard))
                save(extra: lines)
            }
        }
        return lines.joined(separator: "\n")
    }

    static func model(for locale: Locale, names: [String]) async throws -> SFSpeechLanguageModel.Configuration {
        let folder = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0].appendingPathComponent("lm-\(locale.identifier)")
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let french = locale.identifier.hasPrefix("fr")
        let verbs = french ? ["demande à", "ouvre", "regarde", "vérifie", "dans", "avec", "via", "sur"]
                           : ["ask", "open", "check", "tell", "in", "with", "over", "on"]
        let data = SFCustomLanguageModelData(locale: locale, identifier: "com.sauzze.smolcorp.names", version: "1") {
            SFCustomLanguageModelData.PhraseCountsFromTemplates(classes: ["name": names, "verb": verbs]) {
                SFCustomLanguageModelData.TemplatePhraseCountGenerator.Template("<verb> <name>", count: 400)
                SFCustomLanguageModelData.TemplatePhraseCountGenerator.Template("<name>", count: 400)
            }
        }
        let asset = folder.appendingPathComponent("names.bin")
        try await data.export(to: asset)
        let configuration = SFSpeechLanguageModel.Configuration(languageModel: folder.appendingPathComponent("lm"),
                                                                vocabulary: folder.appendingPathComponent("vocab"))
        try await SFSpeechLanguageModel.prepareCustomLanguageModel(for: asset, configuration: configuration)
        return configuration
    }

    private static func context(_ strings: [String]) -> AnalysisContext {
        let context = AnalysisContext()
        context.contextualStrings[.general] = strings
        return context
    }

    private func install(_ module: any SpeechModule) async throws {
        if let request = try await AssetInventory.assetInstallationRequest(supporting: [module]) {
            try await request.downloadAndInstall()
        }
    }

    // MARK: Results

    static let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
    private var extra: [String] = []

    private func save(extra: [String]? = nil) {
        if let extra { self.extra = extra }
        let report: [String: Any] = [
            "at": ISO8601DateFormatter().string(from: Date()),
            "running": running,
            "results": results.map { ["name": $0.name, "ok": $0.ok, "detail": $0.detail, "seconds": $0.seconds] },
            "partial": self.extra,
            "device": UIDevice.current.systemVersion,
        ]
        if let data = try? JSONSerialization.data(withJSONObject: report, options: [.prettyPrinted, .sortedKeys]) {
            try? data.write(to: Spikes.documents.appendingPathComponent("spike.json"))
        }
    }

    private func post(_ pathAndQuery: String, body: String?) async -> (status: Int, body: String, type: String)? {
        var request = URLRequest(url: URL(string: "https://\(host):\(tlsPort)" + pathAndQuery)!)
        request.httpMethod = "POST"
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        if let body { request.httpBody = Data(body.utf8) }
        guard let (data, response) = try? await DeckSession.shared.session.data(for: request),
              let http = response as? HTTPURLResponse else { return nil }
        return (http.statusCode, String(decoding: data, as: UTF8.self), http.value(forHTTPHeaderField: "Content-Type") ?? "")
    }
}

struct Problem: Error, CustomStringConvertible {
    let description: String
    init(_ description: String) { self.description = description }
}

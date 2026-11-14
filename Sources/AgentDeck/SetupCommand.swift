import AppKit

/// `--setup`: setup's checklist from a terminal, for you or for an agent you
/// ask to run it, as `smolcorp setup` on Windows and Linux. It opens Smolcorp
/// if it isn't running, then prints the checklist and ends with 0 when nothing
/// is in the way, 1 when something is. It changes nothing: it holds only the
/// check-only key, which opens the checklist alone, so what it finds is put
/// right in Settings (1.26, SK2).
enum SetupCommand {
    static func run(arguments: [String]) -> Int32 {
        let checkOnly = arguments.contains("--check")
        let base = "http://127.0.0.1:\(Deck.port)"
        if !answers(base) {
            if checkOnly {
                print("Smolcorp isn't running. Open it, or run this without --check to start it.")
                return 1
            }
            print("Starting Smolcorp…")
            let open = Process()
            open.executableURL = URL(fileURLWithPath: "/usr/bin/open")
            open.arguments = [Bundle.main.bundleURL.path]
            try? open.run()
            open.waitUntilExit()
            guard waitUntilItAnswers(base, seconds: 30) else {
                print("Smolcorp didn't answer within 30 s. Open it from Applications, then try again.")
                return 1
            }
        }
        guard let report = selfTest(base) else {
            print("Setup can't be read: is this the Mac Smolcorp runs on, as you, and is it up to date?")
            return 1
        }
        let marks = ["ok": "✓", "fix": "✗", "todo": "•", "info": "i"]
        print("")
        for check in report.checks {
            let status = check["status"] as? String ?? ""
            print("\(marks[status] ?? " ") \(check["title"] as? String ?? "")\n    \(check["detail"] as? String ?? "")")
            if status != "ok", let label = check["label"] as? String { print("    In Settings: \(label).") }
        }
        print(report.ready ? "\nReady. Steps marked • are in Smolcorp's Settings." : "\nNot ready: see the lines marked ✗.")
        if arguments.contains("--open") { NSWorkspace.shared.open(URL(string: "\(base)/setup")!) }
        return report.ready ? 0 : 1
    }

    private static func answers(_ base: String) -> Bool {
        send(base, "GET", "/pair", timeout: 2).status != nil
    }

    private static func waitUntilItAnswers(_ base: String, seconds: Double) -> Bool {
        let deadline = Date().addingTimeInterval(seconds)
        while Date() < deadline {
            if answers(base) { return true }
            Thread.sleep(forTimeInterval: 0.5)
        }
        return false
    }

    private static func selfTest(_ base: String) -> (checks: [[String: Any]], ready: Bool)? {
        let answer = send(base, "GET", "/setup/selftest")
        guard answer.status == 200, let body = answer.body,
              let object = try? JSONSerialization.jsonObject(with: body) as? [String: Any],
              let checks = object["checks"] as? [[String: Any]] else { return nil }
        return (checks, object["ready"] as? Bool ?? false)
    }

    /// The header the check-only key goes in: it opens GET /setup/selftest and nothing else.
    static let selftestKeyHeader = "x-smolcorp-selftest-key"

    /// A request to setup's checklist with the check-only key the running app wrote for this Mac's terminal.
    private static func send(_ base: String, _ method: String, _ path: String,
                             timeout: TimeInterval = 120) -> (status: Int?, body: Data?, problem: String?) {
        var request = URLRequest(url: URL(string: base + path)!, timeoutInterval: timeout)
        request.httpMethod = method
        if let key = try? String(contentsOf: Paths.selftestKey, encoding: .utf8) {
            request.setValue(key.trimmingCharacters(in: .whitespacesAndNewlines), forHTTPHeaderField: selftestKeyHeader)
        }
        var result: (Int?, Data?, String?) = (nil, nil, nil)
        let done = DispatchSemaphore(value: 0)
        URLSession.shared.dataTask(with: request) { data, response, error in
            let status = (response as? HTTPURLResponse)?.statusCode
            let text = data.flatMap { String(data: $0, encoding: .utf8) }?.trimmingCharacters(in: .whitespacesAndNewlines)
            result = (status, data, status.map { $0 >= 400 ? (text ?? "HTTP \($0)") : nil } ?? error?.localizedDescription)
            done.signal()
        }.resume()
        done.wait()
        return result
    }
}

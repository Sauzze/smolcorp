import Foundation

/// The OpenCode installed, for the plugin it can load: OpenCode 1 loads 1.7.2's
/// named export, OpenCode 2 only a default with an id and setup(). Where it
/// is and when it last changed say cheaply whether it might have been
/// updated; its version is asked only then.
enum OpenCodeInstall {
    /// Where OpenCode is installed besides the app's PATH, which, opened from
    /// Finder, is the system's alone: the PATH your login shell sets, then
    /// OpenCode's own installer's folder, Homebrew's, and those of npm, Bun,
    /// pnpm and the Node version managers (nvm, Volta, fnm, asdf, mise).
    private static let places: [String] = {
        let home = Paths.home.path
        let nvm = (try? FileManager.default.contentsOfDirectory(atPath: "\(home)/.nvm/versions/node"))?.sorted().reversed()
            .map { "\(home)/.nvm/versions/node/\($0)/bin" } ?? []
        return loginShellPath() + ["\(home)/.opencode/bin", "/opt/homebrew/bin", "/usr/local/bin", "\(home)/.local/bin", "\(home)/.bun/bin",
                                   "\(home)/.npm-global/bin", "\(home)/Library/pnpm", "\(home)/.volta/bin",
                                   "\(home)/Library/Application Support/fnm/aliases/default/bin", "\(home)/.local/share/fnm/aliases/default/bin",
                                   "\(home)/.asdf/shims", "\(home)/.local/share/mise/shims"] + nvm
    }()

    /// The PATH your login shell sets, asked once, never on the main thread:
    /// empty if it doesn't say within a few seconds.
    private static func loginShellPath() -> [String] {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh")
        // A marker before it, as a shell's start-up files can print things of their own
        process.arguments = ["-lic", #"printf '\n<path>%s' "$PATH""#]
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        process.standardInput = FileHandle.nullDevice
        guard (try? process.run()) != nil else { return [] }
        let timeout = DispatchWorkItem { process.terminate() }
        DispatchQueue.global().asyncAfter(deadline: .now() + 5, execute: timeout)
        let text = String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        process.waitUntilExit()
        timeout.cancel()
        guard let marker = text.range(of: "<path>", options: .backwards) else { return [] }
        return text[marker.upperBound...].trimmingCharacters(in: .whitespacesAndNewlines).split(separator: ":").map(String.init)
    }

    /// The opencode program, with every link resolved, or nil if there's none.
    /// It reads the login shell's PATH the first time: never on the main thread.
    static func program() -> URL? {
        let path = (ProcessInfo.processInfo.environment["PATH"] ?? "").split(separator: ":").map(String.init)
        for folder in path + places {
            let candidate = URL(fileURLWithPath: folder).appendingPathComponent("opencode")
            if FileManager.default.isExecutableFile(atPath: candidate.path) { return candidate.resolvingSymlinksInPath() }
        }
        return nil
    }

    /// Where the program is and when it last changed, or nil without one.
    static func stamp() -> String? {
        guard let program = program(),
              let changed = (try? FileManager.default.attributesOfItem(atPath: program.path))?[.modificationDate] as? Date else { return nil }
        return "\(program.path) \(changed.timeIntervalSince1970)"
    }

    /// OpenCode's major version, as `opencode --version` says it, or nil if
    /// there's no OpenCode or it doesn't say within a few seconds.
    static func major() -> Int? {
        guard let program = program() else { return nil }
        let process = Process()
        process.executableURL = program
        process.arguments = ["--version"]
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let timeout = DispatchWorkItem { process.terminate() }
        DispatchQueue.global().asyncAfter(deadline: .now() + 5, execute: timeout)
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        timeout.cancel()
        return majorVersion(in: String(decoding: data, as: UTF8.self))
    }

    /// The major version in what `--version` prints, as "opencode v2.0.32" or "1.14.0".
    static func majorVersion(in text: String) -> Int? {
        guard let range = text.range(of: #"\d+(?=\.\d)"#, options: .regularExpression) else { return nil }
        return Int(text[range])
    }
}

import Foundation

/// The `smolcorp` command that comes inside the app (Contents/Resources/bin),
/// and the link that puts it in your Terminal: `~/.local/bin/smolcorp`, made
/// from setup, asked first, and removed by the same button. Nothing else on
/// this Mac is touched: no sudo, no shell profile, and a file of yours with
/// that name is never replaced.
enum TerminalCommand {
    static let name = "smolcorp"

    /// Where the link goes: in a folder many shells already search.
    static var link: URL { Paths.home.appendingPathComponent(".local/bin/\(name)") }

    enum State: Equatable {
        /// No link, and nothing else with its name.
        case absent
        /// Our link, to the command in the app it points at.
        case linked
        /// Something else is there: a file or a link of yours. Left alone.
        case occupied
    }

    enum Problem: LocalizedError, Equatable {
        case noCommand, occupied, testCopy, failed(String)

        var errorDescription: String? {
            switch self {
            case .noCommand: return "This build of Smolcorp has no command inside it. Build it again with scripts/build-app.sh."
            case .occupied: return "There is already a file called smolcorp in ~/.local/bin that isn't Smolcorp's. It is left as it is."
            case .testCopy: return "This is a test copy of Smolcorp: it leaves your Terminal's commands to the copy in use."
            case .failed(let why): return "Couldn't change ~/.local/bin/smolcorp: \(why)"
            }
        }
    }

    /// Whether the link at `link` is ours: a symbolic link to a `smolcorp` that
    /// sits in an app's Contents/Resources/bin. Pointing at an app that has since
    /// moved still counts, so it can be mended or removed.
    static func state(link: URL = TerminalCommand.link, fileManager: FileManager = .default) -> State {
        guard let destination = try? fileManager.destinationOfSymbolicLink(atPath: link.path) else {
            return fileManager.fileExists(atPath: link.path) || isDangling(link, fileManager) ? .occupied : .absent
        }
        return destination.hasSuffix("/Contents/Resources/bin/\(name)") ? .linked : .occupied
    }

    private static func isDangling(_ url: URL, _ fileManager: FileManager) -> Bool {
        (try? fileManager.attributesOfItem(atPath: url.path)) != nil
    }

    /// Whether `directory` is one of the folders in `path` (a PATH string).
    static func isOnPath(_ directory: URL, path: String? = ProcessInfo.processInfo.environment["PATH"], home: URL = Paths.home) -> Bool {
        let wanted = directory.standardizedFileURL.path
        return (path ?? "").split(separator: ":").contains { entry in
            let text = String(entry)
            let expanded = text == "~" || text.hasPrefix("~/") ? home.path + text.dropFirst() : text
            return URL(fileURLWithPath: expanded).standardizedFileURL.path == wanted
        }
    }

    /// The line that puts ~/.local/bin on PATH for a login shell, and the file
    /// it goes in: zsh reads ~/.zprofile, bash ~/.bash_profile, and any other
    /// shell gets the plain line with no file named. Smolcorp only shows it;
    /// the owner pastes it, so no shell file of theirs is ever written.
    static func pathLine(shell: String?) -> (command: String, file: String?) {
        let export = "export PATH=\"$HOME/.local/bin:$PATH\""
        let file: String?
        switch URL(fileURLWithPath: shell ?? "").lastPathComponent {
        case "zsh": file = "~/.zprofile"
        case "bash": file = "~/.bash_profile"
        default: file = nil
        }
        guard let file else { return (export, nil) }
        return ("echo '\(export)' >> \(file)", file)
    }

    /// The owner's login shell: the account's, as `$SHELL` says it, else the user record's.
    static func loginShell(environment: [String: String] = ProcessInfo.processInfo.environment) -> String? {
        if let shell = environment["SHELL"], !shell.isEmpty { return shell }
        guard let entry = getpwuid(getuid()), let shell = entry.pointee.pw_shell else { return nil }
        return String(cString: shell)
    }

    /// Makes the link to `command`. Says why not, and changes nothing, if the
    /// command isn't there or the name is taken by something that isn't our link.
    static func add(link: URL = TerminalCommand.link, to command: URL = Paths.bundledCLI, fileManager: FileManager = .default) throws {
        guard fileManager.isExecutableFile(atPath: command.path) else { throw Problem.noCommand }
        switch state(link: link, fileManager: fileManager) {
        case .occupied: throw Problem.occupied
        case .linked: try remove(link: link, fileManager: fileManager) // pointing at where the app was: made again
        case .absent: break
        }
        do {
            try fileManager.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
            try fileManager.createSymbolicLink(at: link, withDestinationURL: command)
        } catch {
            throw Problem.failed(error.localizedDescription)
        }
    }

    /// Takes our link away. A file or link of yours with that name stays.
    static func remove(link: URL = TerminalCommand.link, fileManager: FileManager = .default) throws {
        switch state(link: link, fileManager: fileManager) {
        case .absent: return
        case .occupied: throw Problem.occupied
        case .linked:
            do { try fileManager.removeItem(at: link) } catch { throw Problem.failed(error.localizedDescription) }
        }
    }

    /// What the setup page is told.
    static func json(link: URL = TerminalCommand.link, command: URL = Paths.bundledCLI, path: String? = ProcessInfo.processInfo.environment["PATH"],
                     shell: String? = TerminalCommand.loginShell()) -> [String: Any] {
        let state = state(link: link)
        let line = pathLine(shell: shell)
        return [
            "available": FileManager.default.isExecutableFile(atPath: command.path),
            "state": state == .linked ? "linked" : state == .occupied ? "occupied" : "absent",
            "link": "~/.local/bin/\(name)",
            "onPath": isOnPath(link.deletingLastPathComponent(), path: path),
            "pathLine": line.command,
            "pathFile": line.file ?? "",
            "testCopy": TestCopy.isOn,
        ]
    }
}

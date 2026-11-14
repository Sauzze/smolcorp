import Foundation
import os

/// Programs the app starts on their own account, so they hold none of Smolcorp's permissions (SANDBOX-PLAN §3.5, ST1s).
///
/// macOS charges a program to the app that started it, and Smolcorp.app holds Accessibility, the microphone and
/// screen access. A hire can replace a program the app runs (tmux, kitten, wezterm, opencode, or what your login
/// shell's start-up files run), and that program's next run would read Setup's pairing link and click Allow as
/// Smolcorp. So every program the app starts is made by `process`, which has the bundle's bin/smolcorp-spawn
/// (packaging/macos/smolcorp-spawn.c) become it with responsibility disclaimed: macOS charges it to itself. It is
/// the app's half of the core's disclaim.Cmd (internal/disclaim).
///
/// The only programs the app starts without it, each by its full path: Apple's /usr/bin/osascript with the app's
/// own fixed scripts (Music, Terminal and Tiler need Automation and Accessibility as Smolcorp), /bin/ps,
/// /usr/bin/open, and the core (`GoCore.start`), which is Smolcorp. Apps and links opened with NSWorkspace are
/// started by macOS on their own account, as /usr/bin/open's are. SpawnSitesTests holds that list, and fails on
/// any other start.
enum Disclaimed {
    /// smolcorp-spawn's file name, beside bin/smolcorp in the app.
    static let spawnName = "smolcorp-spawn"

    /// Where smolcorp-spawn should be, and whether this is Smolcorp.app, which must then have it.
    struct Place: Equatable {
        var spawn: URL
        var inApp: Bool
    }

    /// This app's: beside the core in the bundle's bin folder. Only an app counts as one: `swift test` and
    /// `swift run` aren't.
    static let app = Place(spawn: Paths.bundledCLI.deletingLastPathComponent().appendingPathComponent(spawnName),
                           inApp: Bundle.main.bundleURL.pathExtension == "app")

    /// Why a program wasn't started, in Smolcorp's words.
    struct Refusal: LocalizedError, Equatable {
        let message: String
        var errorDescription: String? { message }
    }

    /// A Process that runs `program` with `arguments` on its own account: smolcorp-spawn, given
    /// `PROGRAM PROGRAM ARGS...`. Everything else (environment, folder, pipes, `terminate`) is set on it as on any
    /// Process, and works as before: smolcorp-spawn becomes the program in place, so the pid Smolcorp sees and
    /// signals, and the exit status it gets, are the program's own.
    ///
    /// In Smolcorp.app without its smolcorp-spawn, it throws the refusal, in words, and nothing starts: the app never
    /// starts a program charged to itself. (The app logs nothing in a release build; the core's seal check, which
    /// looks at smolcorp-spawn too, is what tells you in Setup: "Smolcorp's own files changed. Reinstall Smolcorp.")
    /// Outside an app (`swift test`, `swift run`), with no smolcorp-spawn, the Process runs the program as it is:
    /// there's no Smolcorp.app to be charged to. A program that isn't there, or can't be run, throws, as
    /// `Process.run` would.
    static func process(_ program: URL, _ arguments: [String], place: Place = Disclaimed.app) throws -> Process {
        let path = program.path
        guard runnable(path) else {
            throw Refusal(message: "Smolcorp couldn't start \(program.lastPathComponent): there's no program to run at \(path).")
        }
        let process = Process()
        if runnable(place.spawn.path) {
            process.executableURL = place.spawn
            process.arguments = [path, path] + arguments
        } else if place.inApp {
            let refusal = Refusal(message: "Smolcorp didn't start \(program.lastPathComponent): its \(spawnName) is missing, "
                                  + "so it would have held Smolcorp's permissions. Reinstall Smolcorp.")
            #if DEBUG
            Logger(subsystem: "io.github.sauzze.agentdeck", category: "spawn").error("\(refusal.message, privacy: .public)")
            #endif
            throw refusal
        } else {
            process.executableURL = program
            process.arguments = arguments
        }
        return process
    }

    /// Whether `path` is a file with an execute bit: not a folder, not missing.
    private static func runnable(_ path: String) -> Bool {
        var folder: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &folder) && !folder.boolValue
            && FileManager.default.isExecutableFile(atPath: path)
    }
}

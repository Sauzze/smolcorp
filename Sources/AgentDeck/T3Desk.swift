import AppKit

/// The T3 Code desk's link to a hire's thread, which the bridge checks a `t3-link` against
/// before it opens one (`HostBridge`); the Go core runs the desk itself (desktop/t3desk.go).
enum T3Desk {
    /// T3 Code's own link to a thread: `t3code://threads/<env>/<thread id, URL-encoded>`, the env being the first part
    /// of `link`'s path after /threads/ (https://app.t3.codes/threads/<env>/<thread>). Nil without both.
    static func deepLink(thread: String, link: String) -> URL? {
        guard !thread.isEmpty, let url = URL(string: link) else { return nil }
        let parts = url.pathComponents
        guard let at = parts.firstIndex(of: "threads"), at + 1 < parts.count else { return nil }
        let env = parts[at + 1]
        guard !env.isEmpty, env.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
        var allowed = CharacterSet.alphanumerics.intersection(CharacterSet(charactersIn: Unicode.Scalar(0)...Unicode.Scalar(127)))
        allowed.insert(charactersIn: "-._~")
        guard let encoded = thread.addingPercentEncoding(withAllowedCharacters: allowed) else { return nil }
        return URL(string: "t3code://threads/\(env)/\(encoded)")
    }
}

extension Processes {
    /// The processes working in `folder` (or a folder inside it), or given it with `--add-dir`, as T3 Code's agent is
    /// (T797a): read from the kernel by PID, nothing run.
    static func working(in folder: String) -> [pid_t] {
        holding(folder, session: "")?.map(\.pid) ?? []
    }

    /// Why a process holds a folder or a session, as `holding` says it.
    enum Hold: String {
        /// Its current folder is the folder, or one inside it.
        case folder
        /// It was given the folder with `--add-dir`.
        case addDir = "add-dir"
        /// One of its arguments names the Claude Code session (Take over's check, desktop/t3takeover.go).
        case session
    }

    /// The processes working in `folder` as `working(in:)` finds them, and, with a `session`, those whose arguments
    /// name it, each with why: read from the kernel by PID, nothing run, nothing signalled. Nil when it can't look
    /// (no full path, or the kernel listed no process). The Go core on this Mac asks it over the bridge.
    static func holding(_ folder: String, session: String) -> [(pid: pid_t, why: Hold)]? {
        let wanted = HireDesk.physical(folder)
        guard wanted.hasPrefix("/") else { return nil }
        let pids = allPIDs()
        guard !pids.isEmpty else { return nil }
        return pids.compactMap { pid in
            if let here = workingDirectory(of: pid), here == wanted || here.hasPrefix(wanted + "/") { return (pid, .folder) }
            let words = arguments(of: pid)
            if zip(words, words.dropFirst() + [""]).contains(where: { word, next in
                word == "--add-dir=\(wanted)" || word == "--add-dir=\(folder)" || (word == "--add-dir" && (next == wanted || next == folder))
            }) { return (pid, .addDir) }
            if !session.isEmpty, words.contains(where: { $0.contains(session) }) { return (pid, .session) }
            return nil
        }
    }

    /// A process's name as the kernel keeps it (zsh, node…), or "".
    static func name(of pid: pid_t) -> String {
        var info = proc_bsdinfo()
        let size = Int32(MemoryLayout<proc_bsdinfo>.size)
        guard proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, size) == size else { return "" }
        return withUnsafeBytes(of: info.pbi_comm) { String(decoding: $0.prefix { $0 != 0 }, as: UTF8.self) }
    }

    /// A process's arguments, from the kernel (KERN_PROCARGS2); empty when they can't be read.
    static func arguments(of pid: pid_t) -> [String] {
        var mib: [Int32] = [CTL_KERN, KERN_PROCARGS2, pid]
        var size = 0
        guard sysctl(&mib, 3, nil, &size, nil, 0) == 0, size > MemoryLayout<Int32>.size else { return [] }
        var buffer = [UInt8](repeating: 0, count: size)
        guard sysctl(&mib, 3, &buffer, &size, nil, 0) == 0, size > MemoryLayout<Int32>.size else { return [] }
        let count = buffer.withUnsafeBytes { $0.loadUnaligned(as: Int32.self) }
        var i = MemoryLayout<Int32>.size
        // The program's path, then the NULs that pad it, then the arguments.
        while i < size, buffer[i] != 0 { i += 1 }
        while i < size, buffer[i] == 0 { i += 1 }
        var words: [String] = []
        while words.count < Int(count), i < size {
            let start = i
            while i < size, buffer[i] != 0 { i += 1 }
            words.append(String(decoding: buffer[start..<i], as: UTF8.self))
            i += 1
        }
        return words
    }
}

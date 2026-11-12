import XCTest

extension XCTestCase {
    /// Whether `condition` comes true within `seconds`, looked at every few
    /// milliseconds: for what happens a moment later (a process starting, a
    /// file written, the main queue's next turn). It returns as soon as it
    /// does, so a busy Mac only makes it wait longer, never fail. On the main
    /// thread, the main queue keeps running meanwhile.
    func eventually(within seconds: TimeInterval = 10, _ condition: () -> Bool) -> Bool {
        let deadline = Date().addingTimeInterval(seconds)
        while !condition() {
            guard Date() < deadline else { return false }
            if Thread.isMainThread {
                RunLoop.current.run(until: Date().addingTimeInterval(0.02))
            } else {
                Thread.sleep(forTimeInterval: 0.02)
            }
        }
        return true
    }
}

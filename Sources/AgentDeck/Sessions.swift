import Foundation

/// What a session is doing, as the keys show it.
enum SessionState: String, CaseIterable {
    case idle      // white: nothing running, or finished and already seen
    case thinking  // blue: a turn is running
    case asking    // orange: waiting on you — a permission prompt or a question
    case unread    // green: finished, not looked at yet
    case error     // red: the turn ended on an API error
}

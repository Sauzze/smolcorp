import Foundation

/// The GM's mail's limits as config.json keeps them (`Config.mailMaxBytes`,
/// `mailPerHour`, `mailWaitHours`) when they aren't set: the Go core routes the mail.
enum MailRouter {
    static let defaultMaxBytes = 16 * 1024
    static let defaultPerHour = 200
    /// How long a message waits for a busy hire (setting `mailWaitHours`).
    static let defaultWaitHours = 24.0
}

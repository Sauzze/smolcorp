import Foundation
import Observation
import UIKit
import UserNotifications

/// Notifications from the Mac, through Apple, while the app is closed too:
/// an agent finished its turn, one needs you, or a message from the GM
/// session. The Mac sends them itself (its push key), so this phone tells it
/// its device token, through Tailscale only, with which of the three it
/// wants: the switches are kept here and on the Mac. A tap on one opens the
/// office on that agent's desk.
@MainActor
@Observable
final class Notifications {
    static let shared = Notifications()

    /// What the Mac said, at the last look.
    enum Mac: Equatable {
        case unknown
        /// No push key in its Keychain: nothing is sent.
        case noKey
        case ready(registered: Bool)
    }

    var finished = Notifications.flag("notifyFinished") { didSet { changed("notifyFinished", finished) } }
    var waiting = Notifications.flag("notifyWaiting") { didSet { changed("notifyWaiting", waiting) } }
    var gm = Notifications.flag("notifyGM") { didSet { changed("notifyGM", gm) } }

    private(set) var mac = Mac.unknown
    private(set) var allowed: UNAuthorizationStatus = .notDetermined
    /// Why the token didn't reach the Mac, if it didn't.
    private(set) var problem: String?
    /// The session a tapped notification is about, till the office has picked its desk.
    var opened: String?

    private var token = UserDefaults.standard.string(forKey: "pushToken")
    /// What the Mac was last told, so it's told again only when that changes.
    private var told: String?
    private weak var link: DeckLink?
    private var telling: Task<Void, Never>?

    private static func flag(_ name: String) -> Bool {
        UserDefaults.standard.object(forKey: name) as? Bool ?? true
    }

    private func changed(_ name: String, _ on: Bool) {
        UserDefaults.standard.set(on, forKey: name)
        if on { Task { await ask() } }
        tell()
    }

    var anyOn: Bool { finished || waiting || gm }

    /// Whether this build's token is for Apple's sandbox: as its signing says, the aps-environment of the
    /// provisioning profile it carries, whatever the build's configuration. The Simulator's tokens are the
    /// sandbox's; a build with no profile was signed for the App Store's push service.
    static let sandbox: Bool = {
        #if targetEnvironment(simulator)
        return true
        #else
        guard let environment = apsEnvironment(Bundle.main.url(forResource: "embedded", withExtension: "mobileprovision")
            .flatMap { try? Data(contentsOf: $0) }) else { return false }
        return environment == "development"
        #endif
    }()

    /// The aps-environment in a provisioning profile: a plist inside its signature.
    static func apsEnvironment(_ profile: Data?) -> String? {
        guard let profile, let start = profile.range(of: Data("<?xml".utf8)),
              let end = profile.range(of: Data("</plist>".utf8), in: start.lowerBound..<profile.endIndex),
              let plist = try? PropertyListSerialization.propertyList(from: profile[start.lowerBound..<end.upperBound], format: nil)
                as? [String: Any] else { return nil }
        return (plist["Entitlements"] as? [String: Any])?["aps-environment"] as? String
    }

    /// Unpairing: the Mac told to send nothing more here (through Tailscale; if it can't be reached, unpairing
    /// the phone in its setup does it), and the token forgotten here and by iOS. A new pairing asks for a new one.
    func forget(_ link: DeckLink) async {
        telling?.cancel()
        _ = await link.post("/notifications", body: "{}", over: .tailnet, timeout: 5)
        token = nil
        told = nil
        UserDefaults.standard.removeObject(forKey: "pushToken")
        UIApplication.shared.unregisterForRemoteNotifications()
        mac = .unknown
        self.link = nil
    }

    /// The office is up with `link`: the permission asked for once, the token
    /// asked of iOS, and the Mac told.
    func start(_ link: DeckLink) {
        self.link = link
        told = nil
        Task {
            await ask()
            await look()
        }
    }

    /// The one prompt, while any switch is on; and the token, once allowed.
    func ask() async {
        let center = UNUserNotificationCenter.current()
        var status = await center.notificationSettings().authorizationStatus
        if status == .notDetermined, anyOn {
            _ = try? await center.requestAuthorization(options: [.alert, .sound, .badge])
            status = await center.notificationSettings().authorizationStatus
        }
        allowed = status
        if status == .authorized || status == .provisional || status == .ephemeral {
            UIApplication.shared.registerForRemoteNotifications()
        }
    }

    /// iOS gave the device token.
    func received(_ data: Data) {
        // One that comes back after unpairing is no one's.
        guard link != nil else { return }
        let hex = data.map { String(format: "%02x", $0) }.joined()
        if hex != token {
            token = hex
            UserDefaults.standard.set(hex, forKey: "pushToken")
            told = nil
        }
        tell()
    }

    func failed(_ error: Error) {
        problem = "iOS gave no push token: \(error.localizedDescription)"
    }

    /// What the Mac can do and has of this phone, over whichever route.
    func look() async {
        guard let link, let answer = await link.fetch("/notifications", timeout: 6), answer.status == 200,
              let object = try? JSONSerialization.jsonObject(with: answer.data) as? [String: Any] else { return tell() }
        mac = object["key"] as? Bool == true ? .ready(registered: object["registered"] as? Bool == true) : .noKey
        if object["registered"] as? Bool != true { told = nil }
        tell()
    }

    /// The Mac told the token and the switches, through Tailscale only, once
    /// for each change: the connection coming up tries again.
    func tell() {
        guard let link, let token else { return }
        let body: [String: Any] = ["token": token, "sandbox": Notifications.sandbox, "finished": finished, "waiting": waiting, "gm": gm]
        guard let data = try? JSONSerialization.data(withJSONObject: body, options: [.sortedKeys]) else { return }
        let text = String(decoding: data, as: UTF8.self)
        guard text != told, telling == nil else { return }
        telling = Task {
            defer { telling = nil }
            guard let answer = await link.post("/notifications", body: text, over: .tailnet) else {
                problem = "Turn on Tailscale for your Mac to learn this iPhone's notifications"
                return
            }
            guard answer.status == 200 else {
                problem = answer.status == 403 ? "Your Mac takes it only with the phone mic on in its setup" : "Your Mac answered \(answer.status)"
                return
            }
            problem = nil
            told = text
            if let object = try? JSONSerialization.jsonObject(with: Data(answer.body.utf8)) as? [String: Any] {
                mac = object["key"] as? Bool == true ? .ready(registered: true) : .noKey
            }
        }
    }
}

/// The app's delegate, for what only one has: the device token, and the
/// notifications shown and tapped.
final class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        return true
    }

    func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken token: Data) {
        MainActor.assumeIsolated { Notifications.shared.received(token) }
    }

    func application(_ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
        MainActor.assumeIsolated { Notifications.shared.failed(error) }
    }

    // The completion handlers, not the async forms: UIKit wants a tap's answered on the main queue, and an async
    // one ends elsewhere, which aborts the app as a tap opens it.

    /// In front, it shows as a banner all the same: the office may be on another desk.
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification,
                                withCompletionHandler done: @escaping (UNNotificationPresentationOptions) -> Void) {
        done([.banner, .list, .sound])
    }

    /// Tapped: the office opens on that agent's desk.
    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
                                withCompletionHandler done: @escaping () -> Void) {
        let session = response.notification.request.content.userInfo["session"] as? String
        DispatchQueue.main.async {
            if let session, !session.isEmpty { Notifications.shared.opened = session }
            done()
        }
    }
}

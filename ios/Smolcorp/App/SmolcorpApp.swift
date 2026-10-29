import SwiftUI

@main
struct SmolcorpApp: App {
    /// The device token for notifications, and their taps.
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate

    var body: some Scene {
        WindowGroup {
            Group {
                if UserDefaults.standard.string(forKey: "spikeHost") != nil {
                    SpikeView()
                } else {
                    RootView()
                }
            }
            .onAppear {
                UIApplication.shared.isIdleTimerDisabled = true
                // Development: turned on its side without a hand on it (-devOrientation landscape).
                if UserDefaults.standard.string(forKey: "devOrientation") == "landscape",
                   let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene {
                    scene.requestGeometryUpdate(.iOS(interfaceOrientations: .landscapeRight)) { _ in }
                }
            }
        }
    }
}

/// Paired, the office; otherwise, pairing.
struct RootView: View {
    @State private var link: DeckLink? = RootView.saved()

    var body: some View {
        if let link, link.status != .unpaired {
            DeckView(link: link) {
                Task {
                    // Notifications first, while the key still opens the Mac: it forgets this iPhone's token.
                    await Notifications.shared.forget(link)
                    link.stop()
                    Mac.forgetKey()
                    self.link = nil
                }
            }
            .id(ObjectIdentifier(link))
            .statusBarHidden()
        } else {
            PairingView { mac, key in link = DeckLink(mac: mac, key: key) }
        }
    }

    private static func saved() -> DeckLink? {
        // Development: the mock deck, paired from a launch argument (-devCode 123456).
        if let mac = Mac.development {
            let defaults = UserDefaults.standard
            if let key = defaults.string(forKey: "devKey") { return DeckLink(mac: mac, key: key) }
        }
        return Mac.load().map { DeckLink(mac: $0.mac, key: $0.key) }
    }
}

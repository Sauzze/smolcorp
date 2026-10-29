import SwiftUI

/// Phase 0: the office, fed by the mock deck, with what each check found over it.
struct SpikeView: View {
    @State private var spikes = Spikes()

    var body: some View {
        ZStack(alignment: .bottom) {
            OfficeView(office: spikes.office)
                .ignoresSafeArea()
            VStack(alignment: .leading, spacing: 2) {
                Text("spike: \(spikes.running)").bold()
                ForEach(spikes.results) { result in
                    Text("\(result.ok ? "✓" : "✗") \(result.name) \(String(format: "%.1f", result.seconds)) s")
                }
            }
            .font(.system(size: 10, design: .monospaced))
            .foregroundStyle(.white)
            .padding(6)
            .background(.black.opacity(0.6))
            .allowsHitTesting(false)
        }
        .task { await spikes.run() }
    }
}

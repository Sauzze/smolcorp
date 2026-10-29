import SwiftUI

/// The office's own look, for the app's panels over it: the speech bubble's
/// paper and the frame's ink (office.js's PALETTE), its keys, dark with a
/// darker lip, their rounded labels, and corners stepped a pixel in, as the
/// office draws its keys and bubbles. Send is the deck page's own blue.
enum Pixel {
    static let paper = Color(hex: 0xf4f1ea)
    static let ink = Color(hex: 0x1b1e2a)
    static let key = Color(hex: 0x3a3d47)
    static let keyLip = Color(hex: 0x2a2d35)
    static let keyText = Color(hex: 0xf4f1ea)
    static let send = Color(hex: 0x2563eb)
    static let sendLip = Color(hex: 0x1d4fbd)
    static let field = Color.white
    static let dim = Color(hex: 0x6e6a62)
    static let warn = Color(hex: 0xa3540f)
    static let terminal = Color(hex: 0x111418)
    static let terminalText = Color(hex: 0xd8dde3)

    /// One pixel of the office's art, on a panel: the corners' step, the borders' width.
    static let unit: CGFloat = 2

    static func font(_ size: CGFloat, _ weight: Font.Weight = .semibold) -> Font {
        .system(size: size, weight: weight, design: .rounded)
    }
}

extension Color {
    init(hex: UInt32) {
        self.init(red: Double((hex >> 16) & 0xff) / 255, green: Double((hex >> 8) & 0xff) / 255, blue: Double(hex & 0xff) / 255)
    }
}

/// A rectangle with each corner stepped in, as the office's keys are.
struct PixelBox: Shape {
    var step: CGFloat = Pixel.unit * 2

    func path(in r: CGRect) -> Path {
        let s = min(step, r.width / 2, r.height / 2)
        var p = Path()
        p.move(to: CGPoint(x: r.minX + s, y: r.minY))
        p.addLine(to: CGPoint(x: r.maxX - s, y: r.minY))
        p.addLine(to: CGPoint(x: r.maxX - s, y: r.minY + s))
        p.addLine(to: CGPoint(x: r.maxX, y: r.minY + s))
        p.addLine(to: CGPoint(x: r.maxX, y: r.maxY - s))
        p.addLine(to: CGPoint(x: r.maxX - s, y: r.maxY - s))
        p.addLine(to: CGPoint(x: r.maxX - s, y: r.maxY))
        p.addLine(to: CGPoint(x: r.minX + s, y: r.maxY))
        p.addLine(to: CGPoint(x: r.minX + s, y: r.maxY - s))
        p.addLine(to: CGPoint(x: r.minX, y: r.maxY - s))
        p.addLine(to: CGPoint(x: r.minX, y: r.minY + s))
        p.addLine(to: CGPoint(x: r.minX + s, y: r.minY + s))
        p.closeSubpath()
        return p
    }
}

extension View {
    /// Filled and outlined as the office's art is, with its hard shadow under it.
    func pixelFrame(_ fill: Color, border: Color = Pixel.ink, step: CGFloat = Pixel.unit * 2, shadow: CGFloat = 0) -> some View {
        background {
            ZStack {
                if shadow > 0 { PixelBox(step: step).fill(Pixel.ink).offset(y: shadow) }
                PixelBox(step: step).fill(fill)
                PixelBox(step: step).strokeBorder(border, lineWidth: Pixel.unit)
            }
        }
    }
}

extension PixelBox: InsettableShape {
    func inset(by amount: CGFloat) -> some InsettableShape { Inset(base: self, amount: amount) }

    struct Inset: InsettableShape {
        let base: PixelBox
        let amount: CGFloat
        func path(in rect: CGRect) -> Path { base.path(in: rect.insetBy(dx: amount, dy: amount)) }
        func inset(by more: CGFloat) -> Inset { Inset(base: base, amount: amount + more) }
    }
}

/// A key as the office draws them, dark with a darker lip that it sinks into
/// as it's pressed; Send in the deck's blue; or a plain one on the paper.
struct PixelKeyStyle: ButtonStyle {
    enum Kind { case key, send, plain }
    var kind = Kind.key
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        let down = configuration.isPressed
        let (face, lip, text): (Color, Color, Color) = switch kind {
        case .key: (Pixel.key, Pixel.keyLip, Pixel.keyText)
        case .send: (Pixel.send, Pixel.sendLip, .white)
        case .plain: (Pixel.paper, Color(hex: 0xd9d3c5), Pixel.ink)
        }
        return configuration.label
            .font(Pixel.font(16))
            .foregroundStyle(text)
            .frame(minHeight: 44)
            .padding(.horizontal, 12)
            .background {
                ZStack {
                    PixelBox().fill(Pixel.ink).offset(y: down ? 0 : 3)
                    PixelBox().fill(lip).padding(Pixel.unit).offset(y: down ? 0 : 3)
                    PixelBox().fill(face)
                    PixelBox().strokeBorder(Pixel.ink, lineWidth: Pixel.unit)
                }
            }
            .offset(y: down ? 3 : 0)
            .padding(.bottom, 3)
            .opacity(enabled ? 1 : 0.45)
            .contentShape(Rectangle())
    }
}

/// An option's number, small, on the key: as the office's own badges sit on a desk.
struct PixelBadge: View {
    let text: String

    var body: some View {
        Text(text)
            .font(Pixel.font(13, .bold))
            .foregroundStyle(Pixel.ink)
            .frame(minWidth: 24, minHeight: 24)
            .pixelFrame(Pixel.paper, step: Pixel.unit)
    }
}

/// A panel over the office, as its speech bubbles are: paper, ink, a hard
/// shadow. At the foot of the screen upright, down the right on its side,
/// over a light shade, so the office stays in sight round it; above the
/// keyboard when it's up.
struct PixelPanel<Content: View>: View {
    /// A tap on the shade round it, or nil where only the panel's own keys close it.
    let dismiss: (() -> Void)?
    /// As tall as the screen allows, for a panel to read in: the blackboard, settings.
    var tall = false
    @ViewBuilder let content: Content

    init(dismiss: (() -> Void)?, tall: Bool = false, @ViewBuilder content: () -> Content) {
        self.dismiss = dismiss
        self.tall = tall
        self.content = content()
    }

    var body: some View {
        GeometryReader { space in
            let wide = space.size.width > space.size.height
            ZStack(alignment: wide ? .trailing : .bottom) {
                Color.black.opacity(tall ? 0.45 : 0.3)
                    .ignoresSafeArea()
                    .onTapGesture { dismiss?() }
                content
                    .padding(wide ? 12 : 14)
                    .frame(maxWidth: wide ? min(tall ? 560 : 420, space.size.width * (tall ? 0.7 : 0.55)) : 560,
                           maxHeight: tall ? (wide ? space.size.height - 16 : space.size.height * 0.86) : nil)
                    .pixelFrame(Pixel.paper, step: Pixel.unit * 3, shadow: 4)
                    .padding(.horizontal, 10)
                    .padding(.bottom, wide ? 0 : 8)
                    .padding(.vertical, wide ? 8 : 0)
            }
            .frame(width: space.size.width, height: space.size.height)
        }
    }
}

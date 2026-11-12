import AppKit

/// The menu bar item's logo, as config.json's `logo` chooses it, with a dot while the portable office is on
/// (`GoShellDelegate`).
enum StatusMenu {
    /// Both are drawn as templates, so they follow the menu bar's colour.
    static func icon(for logo: Logo) -> NSImage {
        switch logo {
        case .smolcorp: return smolcorpIcon
        case .keys: return keysIcon
        }
    }

    private static var dottedIcons: [Logo: NSImage] = [:]

    /// The logo with a small dot in its corner, a hole cut around it so it
    /// reads on the logo too. Still a template.
    static func dotted(_ logo: Logo) -> NSImage {
        if let made = dottedIcons[logo] { return made }
        let base = icon(for: logo)
        let size = base.size
        let image = NSImage(size: size, flipped: false) { rect in
            base.draw(in: rect)
            guard let context = NSGraphicsContext.current?.cgContext else { return false }
            let dot = CGRect(x: size.width - 6, y: 0, width: 6, height: 6)
            context.setBlendMode(.clear)
            context.fillEllipse(in: dot.insetBy(dx: -1.5, dy: -1.5))
            context.setBlendMode(.normal)
            context.fillEllipse(in: dot)
            return true
        }
        image.isTemplate = true
        dottedIcons[logo] = image
        return image
    }

    /// The hire, a little person in a suit and tie, a point to each cell of
    /// its grid, as the logo kit draws it. A template is tinted white in a
    /// dark menu bar, where the light-ground drawing's hollow face would turn
    /// into a helmet, so a dark one gets the drawing made for dark grounds.
    private static let hireOnLight = [
        ".....######.....", "....########....", "....##....##....", "....#......#....",
        "....#.#..#.#....", "....#.#..#.#....", "....#......#....", ".....######.....",
        "................", "...##..##..##...", "...###.##.###...", "...####..####...",
        "...####..####...", "...##########...", "....###..###....", "....###..###....",
    ]
    private static let hireOnDark = [
        ".....######.....", "....########....", "................", "....########....",
        "....##.##.##....", "....##.##.##....", "....########....", ".....######.....",
        "................", "...##.#..#.##...", "...###.##.###...", "...####..####...",
        "...####..####...", "...##########...", "....###..###....", "....###..###....",
    ]
    private static let smolcorpIcon: NSImage = {
        let image = NSImage(size: NSSize(width: 18, height: 18), flipped: true) { _ in
            guard let context = NSGraphicsContext.current?.cgContext else { return false }
            // Whole cells only, or the edges blur
            context.setShouldAntialias(false)
            let dark = NSAppearance.currentDrawing().bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
            for (row, cells) in (dark ? hireOnDark : hireOnLight).enumerated() {
                for (column, cell) in cells.enumerated() where cell == "#" {
                    context.fill(CGRect(x: 1 + column, y: 1 + row, width: 1, height: 1))
                }
            }
            return true
        }
        image.isTemplate = true
        return image
    }()

    /// Six rounded keys, 3 × 2.
    private static let keysIcon: NSImage = {
        let image = NSImage(size: NSSize(width: 18, height: 14), flipped: false) { _ in
            let side: CGFloat = 4.6, gap: CGFloat = 1.6
            let top: CGFloat = (14 - (2 * side + gap)) / 2
            for row in 0..<2 {
                for col in 0..<3 {
                    let origin = NSPoint(x: 1 + CGFloat(col) * (side + gap), y: top + CGFloat(row) * (side + gap))
                    let rect = NSRect(origin: origin, size: NSSize(width: side, height: side))
                    NSBezierPath(roundedRect: rect, xRadius: 1.2, yRadius: 1.2).fill()
                }
            }
            return true
        }
        image.isTemplate = true
        return image
    }()
}

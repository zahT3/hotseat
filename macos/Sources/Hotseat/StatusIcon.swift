import AppKit

enum StatusIcon {
    /// Two bars for the current account's 5-hour and weekly use, drawn as a
    /// template image so it follows the menu bar's appearance.
    static func meter(five: Double?, week: Double?) -> NSImage {
        let image = NSImage(size: NSSize(width: 18, height: 18), flipped: false) { _ in
            func bar(y: CGFloat, _ fraction: Double?) {
                let full = NSRect(x: 1, y: y, width: 16, height: 4)
                NSColor.black.withAlphaComponent(0.3).setFill()
                NSBezierPath(roundedRect: full, xRadius: 2, yRadius: 2).fill()
                if let f = fraction, f > 0 {
                    var used = full
                    used.size.width = max(4, full.width * CGFloat(min(f, 1)))
                    NSColor.black.setFill()
                    NSBezierPath(roundedRect: used, xRadius: 2, yRadius: 2).fill()
                }
            }
            bar(y: 10, five)
            bar(y: 4, week)
            return true
        }
        image.isTemplate = true
        return image
    }

    static let switching: NSImage? = {
        let image = NSImage(systemSymbolName: "arrow.triangle.2.circlepath", accessibilityDescription: "Switching")
        image?.isTemplate = true
        return image
    }()

    /// The current account's avatar for the menu bar: its picked symbol,
    /// its emoji, or its initial cut out of a circle.
    static func avatar(icon: String?, name: String) -> NSImage {
        let config = NSImage.SymbolConfiguration(pointSize: 14, weight: .semibold)
        if let icon, !icon.isEmpty {
            if let symbol = NSImage(systemSymbolName: icon, accessibilityDescription: name)?.withSymbolConfiguration(config) {
                symbol.isTemplate = true
                return symbol
            }
            return NSImage(size: NSSize(width: 18, height: 18), flipped: false) { rect in
                (icon as NSString).draw(in: rect.insetBy(dx: 0, dy: 1), withAttributes: [.font: NSFont.systemFont(ofSize: 13)])
                return true
            }
        }
        let initial = name.first.map { String($0).uppercased() } ?? "?"
        let image = NSImage(size: NSSize(width: 18, height: 18), flipped: false) { rect in
            NSColor.black.setFill()
            NSBezierPath(ovalIn: rect.insetBy(dx: 1, dy: 1)).fill()
            let font = NSFont.systemFont(ofSize: 10.5, weight: .bold)
            let text = initial as NSString
            let size = text.size(withAttributes: [.font: font])
            NSGraphicsContext.current?.compositingOperation = .destinationOut
            text.draw(at: NSPoint(x: rect.midX - size.width / 2, y: rect.midY - size.height / 2),
                      withAttributes: [.font: font, .foregroundColor: NSColor.black])
            return true
        }
        image.isTemplate = true
        return image
    }
}

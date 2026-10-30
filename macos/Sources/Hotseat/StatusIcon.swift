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
}

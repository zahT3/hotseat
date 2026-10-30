import AppKit
import SwiftUI

/// Holds an account card in the menu and draws the highlight while the
/// menu item is highlighted, which needs no tracking areas of its own.
final class MenuCardView: NSView {
    private let host: NSHostingView<AccountCard>

    init(_ card: AccountCard) {
        host = NSHostingView(rootView: card)
        super.init(frame: NSRect(origin: .zero, size: host.fittingSize))
        host.frame = bounds
        host.autoresizingMask = [.width, .height]
        addSubview(host)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func update(_ card: AccountCard) {
        host.rootView = card
        let size = host.fittingSize
        if size != frame.size {
            setFrameSize(size)
        }
        needsDisplay = true
    }

    override func draw(_ dirtyRect: NSRect) {
        guard enclosingMenuItem?.isHighlighted == true else { return }
        NSColor.labelColor.withAlphaComponent(0.08).setFill()
        NSBezierPath(roundedRect: bounds.insetBy(dx: 5, dy: 1), xRadius: 8, yRadius: 8).fill()
    }
}

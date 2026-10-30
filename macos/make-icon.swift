// Draws Hotseat's app icon: the menu bar meter (two usage bars) on an
// indigo rounded square. Run: swift make-icon.swift Resources/AppIcon.icns
import AppKit

let out = URL(fileURLWithPath: CommandLine.arguments[1])

func draw(_ px: Int) -> Data {
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8,
                               samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                               bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    let s = CGFloat(px) / 1024
    let tile = NSRect(x: 100 * s, y: 100 * s, width: 824 * s, height: 824 * s)
    let shape = NSBezierPath(roundedRect: tile, xRadius: 185 * s, yRadius: 185 * s)
    NSGraphicsContext.current?.saveGraphicsState()
    let shadow = NSShadow()
    shadow.shadowColor = NSColor.black.withAlphaComponent(0.35)
    shadow.shadowBlurRadius = 24 * s
    shadow.shadowOffset = NSSize(width: 0, height: -10 * s)
    shadow.set()
    NSColor(srgbRed: 0.28, green: 0.25, blue: 0.70, alpha: 1).setFill()
    shape.fill()
    NSGraphicsContext.current?.restoreGraphicsState()
    NSGradient(starting: NSColor(srgbRed: 0.40, green: 0.42, blue: 0.96, alpha: 1),
               ending: NSColor(srgbRed: 0.24, green: 0.18, blue: 0.62, alpha: 1))!.draw(in: shape, angle: -90)
    func bar(y: CGFloat, _ fraction: CGFloat) {
        let track = NSRect(x: 232 * s, y: y * s, width: 560 * s, height: 104 * s)
        NSColor.white.withAlphaComponent(0.25).setFill()
        NSBezierPath(roundedRect: track, xRadius: 52 * s, yRadius: 52 * s).fill()
        var used = track
        used.size.width *= fraction
        NSColor.white.setFill()
        NSBezierPath(roundedRect: used, xRadius: 52 * s, yRadius: 52 * s).fill()
    }
    bar(y: 560, 0.72)
    bar(y: 360, 0.42)
    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])!
}

let set = FileManager.default.temporaryDirectory.appendingPathComponent("AppIcon-\(UUID().uuidString).iconset")
try FileManager.default.createDirectory(at: set, withIntermediateDirectories: true)
for size in [16, 32, 128, 256, 512] {
    try draw(size).write(to: set.appendingPathComponent("icon_\(size)x\(size).png"))
    try draw(size * 2).write(to: set.appendingPathComponent("icon_\(size)x\(size)@2x.png"))
}
let p = Process()
p.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
p.arguments = ["-c", "icns", set.path, "-o", out.path]
try p.run()
p.waitUntilExit()
exit(p.terminationStatus)

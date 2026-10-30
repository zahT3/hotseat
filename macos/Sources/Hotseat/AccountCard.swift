import HotseatKit
import SwiftUI

/// One account in the menu. Clicking the card moves both the CLI and
/// Claude Desktop to it; the two buttons at the bottom move one side only.
/// No hover tracking or tooltips here: tracking areas in menu views kept
/// AppKit and WindowServer busy long after the menu closed. MenuCardView
/// draws the highlight from the menu item's own state instead.
struct AccountCard: View {
    let info: AccountInfo
    let recommended: Bool
    let now: Date
    let shortcut: Int?
    /// The account a switch is in progress to, if any.
    let switching: String?
    let onTap: () -> Void
    let onCLIOnly: () -> Void
    let onDesktopOnly: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            HStack(spacing: 10) {
                Avatar(info: info, size: 30)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 6) {
                        Text(info.displayName)
                            .font(.system(size: 14, weight: .semibold))
                            .lineLimit(1).truncationMode(.tail)
                        if recommended {
                            Badge(text: "Suggested", color: .secondary)
                        }
                    }
                    if let detail = info.detail {
                        Text(detail).font(.system(size: 11)).foregroundStyle(.secondary)
                            .lineLimit(1).truncationMode(.middle)
                    }
                }
                Spacer(minLength: 4)
                if switching == info.account {
                    ProgressView().controlSize(.small)
                } else if let shortcut {
                    Text("⌘\(shortcut)").font(.system(size: 11)).foregroundStyle(.tertiary)
                }
            }
            VStack(alignment: .leading, spacing: 5) {
                meter("5h", info.fiveHour)
                meter("7d", info.sevenDay)
                if let problem = info.loginProblem, !problem.isEmpty {
                    warning("CLI login: " + problem, symbol: "person.crop.circle.badge.exclamationmark", color: .red)
                }
                if info.isLimited(now: now) {
                    warning(Format.limit(info.limit, now: now), symbol: "exclamationmark.octagon.fill", color: .red)
                }
                if let problem = info.desktopProblem, !problem.isEmpty {
                    warning("Desktop: " + problem, symbol: "exclamationmark.triangle.fill", color: .orange)
                }
                if let error = info.error, !error.isEmpty {
                    warning(error, symbol: "xmark.octagon.fill", color: .red)
                }
                Text(Format.age(info.observedAt, now: now))
                    .font(.system(size: 11)).foregroundStyle(.secondary)
            }
            .padding(.leading, 40)
            HStack(spacing: 8) {
                SideButton(title: "CLI", symbol: "terminal", on: info.current,
                           enabled: switching == nil, action: onCLIOnly)
                SideButton(title: info.desktop == "none" ? "Desktop not set up" : "Desktop", symbol: "macwindow",
                           on: info.desktop == "active", enabled: switching == nil && info.desktop != "none",
                           action: onDesktopOnly)
            }
            .padding(.leading, 40)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        .frame(width: 350, alignment: .leading)
        .opacity(switching != nil && switching != info.account ? 0.5 : 1)
        .contentShape(Rectangle())
        .onTapGesture { if switching == nil { onTap() } }
    }

    private func meter(_ label: String, _ w: UsageWindow?) -> some View {
        HStack(spacing: 8) {
            Text(label).font(.system(size: 12)).foregroundStyle(.secondary)
                .frame(width: 20, alignment: .leading)
            UsageBar(fraction: w.map { $0.percent(now: now) / 100 })
            Text(Format.value(w, now: now)).font(.system(size: 12)).monospacedDigit()
                .frame(width: 118, alignment: .trailing)
        }
    }

    private func warning(_ text: String, symbol: String, color: Color) -> some View {
        Label(text, systemImage: symbol)
            .font(.system(size: 12))
            .foregroundStyle(color)
            .lineLimit(2)
    }
}

/// Moves one side (CLI or Desktop) to the account. Filled when that side
/// already uses it.
struct SideButton: View {
    let title: String
    let symbol: String
    let on: Bool
    let enabled: Bool
    let action: () -> Void

    var body: some View {
        Button(action: { if enabled && !on { action() } }) {
            HStack(spacing: 5) {
                Image(systemName: on ? "checkmark.circle.fill" : symbol)
                Text(title).lineLimit(1)
            }
            .font(.system(size: 12, weight: .medium))
            .frame(maxWidth: .infinity)
            .frame(height: 26)
            .foregroundStyle(on ? Color.white : enabled ? Color.primary : Color.secondary)
            .background(RoundedRectangle(cornerRadius: 7).fill(on ? Color.accentColor : Color.primary.opacity(0.08)))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

/// The account's picked symbol or emoji, else its initial.
struct Avatar: View {
    let info: AccountInfo
    let size: CGFloat

    var body: some View {
        ZStack {
            Circle().fill(info.current ? Color.accentColor : Color.secondary.opacity(0.22))
            switch AvatarKind(info) {
            case .symbol(let name):
                Image(systemName: name).font(.system(size: size * 0.45, weight: .semibold))
                    .foregroundStyle(info.current ? Color.white : Color.primary)
            case .text(let text):
                Text(text).font(.system(size: size * 0.45, weight: .semibold))
                    .foregroundStyle(info.current ? Color.white : Color.primary)
            }
        }
        .frame(width: size, height: size)
    }
}

enum AvatarKind {
    case symbol(String)
    case text(String)

    init(_ info: AccountInfo) {
        if let icon = info.icon, !icon.isEmpty {
            self = NSImage(systemSymbolName: icon, accessibilityDescription: nil) != nil ? .symbol(icon) : .text(icon)
        } else {
            self = .text(info.displayName.first.map { String($0).uppercased() } ?? "?")
        }
    }
}

struct Badge: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.system(size: 10, weight: .semibold))
            .lineLimit(1)
            .fixedSize()
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .foregroundStyle(color)
            .background(Capsule().fill(color.opacity(0.16)))
    }
}

/// A thin bar that stays empty at 0% and when the number is unknown
/// (ProgressView draws a dot at 0, which reads as some usage).
struct UsageBar: View {
    let fraction: Double?

    var body: some View {
        GeometryReader { g in
            ZStack(alignment: .leading) {
                Capsule().fill(Color.primary.opacity(0.12))
                if let f = fraction, f > 0 {
                    Capsule().fill(Color.accentColor).frame(width: max(6, g.size.width * min(f, 1)))
                }
            }
        }
        .frame(height: 6)
    }
}

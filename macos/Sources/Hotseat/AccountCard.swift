import HotseatKit
import SwiftUI

/// One account in the menu. The whole card is the switch, like a network
/// in the Wi-Fi menu: one account is active at a time.
struct AccountCard: View {
    let info: AccountInfo
    let recommended: Bool
    let now: Date
    let shortcut: Int?
    /// The account a switch is in progress to, if any.
    let switching: String?
    /// Click on the card: CLI and Desktop together.
    let onTap: () -> Void
    let onCLIOnly: () -> Void
    let onDesktopOnly: () -> Void
    let hover = Hover()

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            HStack(spacing: 10) {
                Avatar(name: info.displayName, highlighted: info.current)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 6) {
                        Text(info.displayName)
                            .font(.system(size: 14, weight: .semibold))
                            .lineLimit(1).truncationMode(.middle)
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
                }
                Pill(text: "CLI", on: info.current, enabled: switching == nil,
                     help: info.current ? "New terminal sessions use this account" : "Use this account for new terminal sessions only",
                     action: onCLIOnly)
                if info.desktop != "none" {
                    Pill(text: "Desktop", on: info.desktop == "active", enabled: switching == nil,
                         help: info.desktop == "active" ? "Claude Desktop runs as this account" : "Move only Claude Desktop to this account",
                         action: onDesktopOnly)
                }
                if let shortcut {
                    Text("⌘\(shortcut)").font(.system(size: 11)).foregroundStyle(.tertiary)
                }
            }
            VStack(alignment: .leading, spacing: 5) {
                meter("5h", info.fiveHour)
                meter("7d", info.sevenDay)
                if let problem = info.loginProblem, !problem.isEmpty {
                    warning("CLI login: " + problem, symbol: "person.crop.circle.badge.exclamationmark", color: .red)
                }
                if info.limitActive {
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
            .padding(.leading, 38)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        .frame(width: 350, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(hover.on && switching == nil ? Color.primary.opacity(0.08) : .clear)
                .padding(.horizontal, 5))
        .opacity(switching != nil && switching != info.account ? 0.5 : 1)
        .contentShape(Rectangle())
        .onHover { hover.on = $0 }
        .onTapGesture { if switching == nil { onTap() } }
        .help(info.current && info.desktop != "parked" ? "In use" : "Click to switch to this account")
    }

    private func meter(_ label: String, _ w: UsageWindow?) -> some View {
        HStack(spacing: 8) {
            Text(label).font(.system(size: 12)).foregroundStyle(.secondary)
                .frame(width: 20, alignment: .leading)
            UsageBar(fraction: w.map { $0.reset ? 0 : $0.usedPercent / 100 })
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

struct Avatar: View {
    let name: String
    let highlighted: Bool

    var body: some View {
        ZStack {
            Circle().fill(highlighted ? Color.accentColor : Color.secondary.opacity(0.22))
            Text(name.first.map { String($0).uppercased() } ?? "?")
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(highlighted ? Color.white : Color.primary)
        }
        .frame(width: 28, height: 28)
    }
}

/// Where an account is in use. A filled pill is in use here; an outlined
/// one moves just that side (CLI or Desktop) to this account.
struct Pill: View {
    let text: String
    let on: Bool
    let enabled: Bool
    let help: String
    let action: () -> Void

    var body: some View {
        Button(action: { if !on && enabled { action() } }) {
            Text(text)
                .font(.system(size: 10, weight: .semibold))
                .padding(.horizontal, 7)
                .padding(.vertical, 2)
                .foregroundStyle(on ? Color.white : Color.secondary)
                .background(Capsule().fill(on ? Color.accentColor : Color.clear))
                .overlay(Capsule().strokeBorder(on ? Color.clear : Color.secondary.opacity(0.5), lineWidth: 1))
        }
        .buttonStyle(.plain)
        .help(help)
    }
}

struct Badge: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.system(size: 10, weight: .semibold))
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

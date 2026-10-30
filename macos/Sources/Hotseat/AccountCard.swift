import HotseatKit
import SwiftUI

/// One account in the menu: who it is, where it is in use, its two usage
/// windows, and the actions for it.
struct AccountCard: View {
    let info: AccountInfo
    let recommended: Bool
    let now: Date
    /// The account a switch is in progress to, if any.
    let switching: String?
    let onSwitch: () -> Void
    let onEditNote: () -> Void

    /// Switching does something unless this account already runs both the
    /// CLI and (where it has a profile) Desktop.
    private var canSwitch: Bool { !info.current || info.desktop == "parked" }

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
                if info.current {
                    Badge(text: "CLI", color: .accentColor)
                }
                if info.desktop == "active" {
                    Badge(text: "Desktop", color: .accentColor)
                }
            }
            VStack(alignment: .leading, spacing: 5) {
                meter("5h", info.fiveHour)
                meter("7d", info.sevenDay)
                if info.limitActive {
                    warning(Format.limit(info.limit, now: now), symbol: "exclamationmark.octagon.fill", color: .red)
                }
                if let problem = info.desktopProblem, !problem.isEmpty {
                    warning("Desktop: " + problem, symbol: "exclamationmark.triangle.fill", color: .orange)
                }
                if let error = info.error, !error.isEmpty {
                    warning(error, symbol: "xmark.octagon.fill", color: .red)
                }
                HStack(spacing: 8) {
                    Text(Format.age(info.observedAt, now: now))
                        .font(.system(size: 11)).foregroundStyle(.secondary)
                    Spacer()
                    Button(action: onEditNote) {
                        Image(systemName: "pencil")
                    }
                    .buttonStyle(.borderless)
                    .help("Edit note")
                    .disabled(switching != nil)
                    if switching == info.account {
                        ProgressView().controlSize(.small)
                        Text("Switching…").font(.system(size: 12)).foregroundStyle(.secondary)
                    } else if canSwitch {
                        Button(info.current ? "Switch Desktop" : "Switch", action: onSwitch)
                            .buttonStyle(.borderedProminent)
                            .controlSize(.small)
                            .disabled(switching != nil)
                    }
                }
            }
            .padding(.leading, 38)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        .frame(width: 350, alignment: .leading)
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

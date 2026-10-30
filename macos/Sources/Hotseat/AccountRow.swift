import HotseatKit
import Observation
import SwiftUI

// The Command Line Tools ship no SwiftUIMacros plugin, so @State cannot be
// used here; hover state lives in an @Observable object instead.
@Observable
final class Hover {
    var on = false
}

/// One account in the menu: usage bars, reset times and how old the data is.
struct AccountRow: View {
    let info: AccountInfo
    let recommended: Bool
    let now: Date
    let disabled: Bool
    let onTap: () -> Void
    let hover = Hover()

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 5) {
                Image(systemName: "checkmark")
                    .font(.system(size: 10, weight: .bold))
                    .frame(width: 12)
                    .opacity(info.current ? 1 : 0)
                Text(info.account).fontWeight(info.current ? .semibold : .regular)
                if recommended {
                    Text("suggested").font(.caption).foregroundStyle(.secondary)
                }
                Spacer(minLength: 8)
                if info.desktop == "active" {
                    Text("Desktop").font(.caption).foregroundStyle(.secondary)
                }
                if info.limitActive {
                    Text("LIMIT until " + (info.limit.flatMap { $0.resetsAt > 0 ? Format.when($0.resetsAt, now: now) : nil } ?? "unknown"))
                        .font(.caption).foregroundStyle(.red)
                }
            }
            VStack(alignment: .leading, spacing: 3) {
                bar("5h", info.fiveHour)
                bar("7d", info.sevenDay)
                Text(Format.age(info.observedAt, now: now)).font(.caption).foregroundStyle(.secondary)
                if let problem = info.desktopProblem {
                    Text("Desktop: " + problem).font(.caption).foregroundStyle(.orange)
                }
                if let error = info.error, !error.isEmpty {
                    Text(error).font(.caption).foregroundStyle(.red)
                }
            }
            .padding(.leading, 17)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 5)
        .frame(width: 320, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 5).fill(hover.on && !disabled ? Color.primary.opacity(0.08) : .clear).padding(.horizontal, 5))
        .opacity(disabled ? 0.5 : 1)
        .contentShape(Rectangle())
        .onHover { hover.on = $0 }
        .onTapGesture { if !disabled { onTap() } }
    }

    private func bar(_ label: String, _ w: UsageWindow?) -> some View {
        HStack(spacing: 6) {
            Text(label).font(.caption).foregroundStyle(.secondary).frame(width: 18, alignment: .leading)
            UsageBar(fraction: w.map { $0.reset ? 0 : $0.usedPercent / 100 })
            Text(Format.value(w, now: now)).font(.caption).monospacedDigit()
                .frame(width: 112, alignment: .trailing)
        }
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
                    Capsule().fill(Color.accentColor).frame(width: max(5, g.size.width * min(f, 1)))
                }
            }
        }
        .frame(height: 5)
    }
}

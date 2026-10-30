import HotseatKit
import Observation
import SwiftUI

@MainActor
@Observable
final class SetupModel {
    var status: SetupStatus?
    var usingBundled = false
    var bundledVersion = ""
    /// The step being run, if any.
    var running: String?
    var errors: [String: String] = [:]
    var loadError: String?

    var steps: [SetupStep] {
        status?.steps(bundledVersion: bundledVersion, usingBundled: usingBundled) ?? []
    }
}

struct SetupView: View {
    let model: SetupModel
    let run: (SetupStep) -> Void
    let reload: () -> Void
    let close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 4) {
                Text("Set up Hotseat").font(.system(size: 20, weight: .semibold))
                Text("Each step says what it changes. Nothing runs until you click Run.")
                    .font(.system(size: 12)).foregroundStyle(.secondary)
            }
            if let error = model.loadError {
                Label(error, systemImage: "exclamationmark.triangle.fill").foregroundStyle(.red).font(.system(size: 12))
            }
            VStack(spacing: 8) {
                ForEach(model.steps) { step in
                    row(step)
                }
            }
            Spacer(minLength: 0)
            HStack {
                Button("Check Again", action: reload)
                Spacer()
                Button("Done", action: close).keyboardShortcut(.defaultAction)
            }
        }
        .padding(20)
        .frame(width: 560, height: 520)
    }

    private func row(_ step: SetupStep) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: step.done ? "checkmark.circle.fill" : "circle")
                .font(.system(size: 18))
                .foregroundStyle(step.done ? Color.green : Color.secondary)
            VStack(alignment: .leading, spacing: 3) {
                Text(step.title).font(.system(size: 13, weight: .semibold))
                Text(step.detail).font(.system(size: 12)).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                if let error = model.errors[step.id] {
                    Text(error).font(.system(size: 11)).foregroundStyle(.red)
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                }
            }
            Spacer(minLength: 8)
            if model.running == step.id {
                ProgressView().controlSize(.small)
            } else if !step.done {
                Button("Run") { run(step) }
                    .disabled(model.running != nil || (step.id == "desktop" && model.status?.desktop == "absent"))
            }
        }
        .padding(10)
        .background(RoundedRectangle(cornerRadius: 8).fill(Color.primary.opacity(0.04)))
    }
}

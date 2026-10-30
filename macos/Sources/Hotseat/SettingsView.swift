import HotseatKit
import SwiftUI

struct SettingsActions {
    let saveNote: (String, String) -> Void
    let addAccount: () -> Void
    let setUpDesktop: (String) -> Void
    let setLaunchAtLogin: (Bool) -> Void
    let setAskBeforeDesktopRestart: (Bool) -> Void
}

struct SettingsView: View {
    let model: AppModel
    let actions: SettingsActions

    var body: some View {
        TabView {
            AccountsSettings(model: model, actions: actions)
                .tabItem { Label("Accounts", systemImage: "person.2") }
            GeneralSettings(model: model, actions: actions)
                .tabItem { Label("General", systemImage: "gearshape") }
        }
        .padding(16)
        .frame(width: 600, height: 460)
    }
}

struct AccountsSettings: View {
    let model: AppModel
    let actions: SettingsActions

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ScrollView {
                VStack(spacing: 10) {
                    ForEach(model.snapshot?.accounts ?? []) { a in
                        AccountSettingsRow(info: a, model: model, actions: actions)
                    }
                }
            }
            HStack {
                Button(action: actions.addAccount) {
                    Label("Add Account…", systemImage: "plus")
                }
                Spacer()
                if let error = model.lastError {
                    Label(error.message, systemImage: "exclamationmark.triangle")
                        .font(.system(size: 11)).foregroundStyle(.red).lineLimit(2)
                }
            }
        }
    }
}

struct AccountSettingsRow: View {
    let info: AccountInfo
    let model: AppModel
    let actions: SettingsActions

    private var draft: Binding<String> {
        Binding(
            get: { model.noteDrafts[info.account] ?? info.note ?? "" },
            set: { model.noteDrafts[info.account] = $0 })
    }

    private var edited: Bool {
        guard let d = model.noteDrafts[info.account] else { return false }
        return d.trimmingCharacters(in: .whitespaces) != (info.note ?? "")
    }

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Avatar(name: info.displayName, highlighted: info.current)
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    TextField(info.email ?? info.account, text: draft)
                        .textFieldStyle(.roundedBorder)
                        .font(.system(size: 13))
                        .onSubmit { actions.saveNote(info.account, draft.wrappedValue) }
                    if edited {
                        Button("Save") { actions.saveNote(info.account, draft.wrappedValue) }
                    }
                }
                Grid(alignment: .leading, horizontalSpacing: 10, verticalSpacing: 3) {
                    row("Email", info.email ?? "—")
                    row("Account", info.account)
                    row("Config", (info.configDir ?? "—").replacingOccurrences(of: NSHomeDirectory(), with: "~"))
                    GridRow {
                        Text("Desktop").foregroundStyle(.secondary)
                        HStack(spacing: 8) {
                            Text(desktopText)
                            if info.desktop == "none" {
                                Button("Set Up Desktop…") { actions.setUpDesktop(info.account) }
                                    .controlSize(.small)
                            }
                        }
                    }
                }
                .font(.system(size: 12))
            }
            Spacer(minLength: 0)
            VStack(alignment: .trailing, spacing: 4) {
                if info.current { Badge(text: "CLI", color: .accentColor) }
                if info.desktop == "active" { Badge(text: "Desktop", color: .accentColor) }
            }
        }
        .padding(12)
        .background(RoundedRectangle(cornerRadius: 10).fill(Color.primary.opacity(0.05)))
    }

    private var desktopText: String {
        if let p = info.desktopProblem, !p.isEmpty {
            return p == "not logged in yet" ? "Set up — switch to it and sign in in Claude Desktop" : p
        }
        switch info.desktop {
        case "active": return "In use"
        case "parked": return "Ready"
        default: return "Not set up"
        }
    }

    private func row(_ label: String, _ value: String) -> some View {
        GridRow {
            Text(label).foregroundStyle(.secondary)
            Text(value).textSelection(.enabled).lineLimit(1).truncationMode(.middle)
        }
    }
}

struct GeneralSettings: View {
    let model: AppModel
    let actions: SettingsActions

    var body: some View {
        Form {
            Toggle("Launch at Login", isOn: Binding(
                get: { model.launchAtLogin }, set: { actions.setLaunchAtLogin($0) }))
            Toggle("Ask before restarting Claude Desktop", isOn: Binding(
                get: { model.askBeforeDesktopRestart }, set: { actions.setAskBeforeDesktopRestart($0) }))
            LabeledContent("hotseat CLI", value: model.cliPath.isEmpty ? "not found" : model.cliPath)
            LabeledContent("Version", value: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev")
        }
        .formStyle(.grouped)
    }
}

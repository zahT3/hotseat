import Foundation

/// Adding an account needs the official login in a terminal, so the app
/// hands `hotseat add` to Terminal as a .command script.
public enum AddAccount {
    /// The names `hotseat add` accepts.
    public static func isValidName(_ name: String) -> Bool {
        name.range(of: #"^[A-Za-z0-9][A-Za-z0-9_.-]*$"#, options: .regularExpression) != nil
    }

    /// An interactive login shell, so PATH (and the real claude binary
    /// that `hotseat add` looks for) is the one the user's terminals see.
    public static func script(hotseat: URL, name: String) -> String {
        """
        #!/bin/zsh -il
        \(quote(hotseat.path)) add \(quote(name))
        echo
        echo "Done. You can close this window."

        """
    }

    static func quote(_ s: String) -> String {
        "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
}

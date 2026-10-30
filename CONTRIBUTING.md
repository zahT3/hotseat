# Contributing

Thanks for helping. Hotseat is small on purpose; please open an issue before a large change.

## Setup

- Go 1.26 and the Xcode Command Line Tools (Xcode is not needed).
- `go test ./...` runs the CLI tests. They only ever touch a temporary HOME.
- `macos/test.sh` runs the Swift tests. Use it instead of `swift test`: with only the Command Line Tools, plain `swift test` sometimes cannot find the swift-testing macro plugin.
- `macos/build-app.sh --install` builds the app into `~/Applications`.
- `macos/demo/hotseat-demo` prints made-up accounts. To show them in the app, run `defaults write io.github.zahT3.hotseat hotseatPath "$PWD/macos/demo/hotseat-demo"`. Use it for screenshots.

## Rules that do not bend

- Never read, copy, store or forward tokens or credentials (`.credentials.json`, the keychain, Desktop's cookies and storage, `oauth:tokenCache*`).
- Never switch accounts without the user asking. No rotation, account pools or proxies.
- Never delete user data. Move it into `~/.config/hotseat/backups/` instead.
- The app does not touch Claude's files itself. Everything goes through the `hotseat` CLI, and every command supports `--json`.

## Code

- Write a failing test first. Bug fixes come with the test that reproduces the bug.
- Keep changes focused. Comments explain why the code does something, not what changed.
- SwiftUI in this repo avoids `@State`, because the Command Line Tools ship no SwiftUIMacros plugin; use `@Observable` classes instead. Menu views avoid `.onHover` and `.help`: their tracking areas keep the app busy while idle.

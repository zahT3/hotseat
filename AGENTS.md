# AGENTS.md

Instructions for AI coding agents (and humans) working on Hotseat. Read CONTRIBUTING.md too.

## What this is

Hotseat switches Claude Code and Claude Desktop between several Claude accounts on one Mac while keeping one shared workspace. It has two parts:

- `cmd/hotseat`, `internal/`: the Go CLI. It does all the work: accounts, sharing, shim, status line, limit handoff, Desktop switching, setup and uninstall.
- `macos/`: the menu bar app (Swift, AppKit `NSStatusItem` with SwiftUI views). It only displays and clicks; every action runs the CLI (`hotseat panel --json`, `hotseat use …`).

## Commands

```sh
go vet ./... && go test ./...          # CLI tests (temporary HOME only)
macos/test.sh                          # Swift tests — never plain `swift test`
macos/build-app.sh [--install]         # build (and install) Hotseat.app, CLI bundled
go build -o ~/.local/bin/hotseat ./cmd/hotseat
defaults write io.github.zahT3.hotseat hotseatPath "$PWD/macos/demo/hotseat-demo"   # app on made-up data
```

Toolchain: Go 1.26, Swift 6 with only the Xcode Command Line Tools (CI uses an older Xcode — keep code compatible).

## Rules that never bend

- Never read, copy, store, print or forward credentials: `.credentials.json`, the keychain, `CLAUDE_CODE_OAUTH_TOKEN`, Claude Desktop's `Cookies`/`Local Storage`/`IndexedDB`, `config.json` values other than `lastKnownAccountUuid`. From `.claude.json` only the account and organization IDs, `mcpServers` and the email address may be read.
- Never switch accounts without the user asking. No rotation, account pools or proxies.
- Never delete user data: move it into `~/.config/hotseat/backups/`.
- Tests run only against a temporary HOME (`testutil.FakeHome`); never touch the real `~/.claude*` or Desktop data.
- Every CLI command supports `--json`. The app reads nothing of Claude's itself.

## Conventions

- TDD: write the failing test first and watch it fail; bug fixes come with the test that reproduces the bug.
- Keep changes small and focused; comments explain why, not what changed.
- SwiftUI: no `@State` (the CLT ship no SwiftUIMacros plugin) — keep view state in `@Observable` classes.
- Menu views: no `.onHover` or `.help` (their tracking areas kept the app busy while idle); `MenuCardView` draws the highlight.
- Modal alerts run through `later {}` on the main queue, never inside a Swift `Task` (that crashed the app).
- Claude Desktop's on-disk format is not a public interface: check identities before moving data, and make `hotseat doctor` report what it does not recognise.

## Releasing

Bump `const Version` in `internal/cli/cli.go`, update CHANGELOG.md, tag `vX.Y.Z` and push the tag: `.github/workflows/release.yml` builds the universal CLI and app and publishes the release. Then update `url` and `sha256` in `packaging/homebrew/hotseat.rb` and the tap.

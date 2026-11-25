# Changelog

## 0.1.1

- Limit messages that go on with a sentence after the time zone, such as model-scoped limits ("You've hit your Fable limit · resets Nov 29 at 11am (Asia/Shanghai). Switch to another model …"), now get their reset time instead of an open-ended limit.
- A Claude Desktop usage sample without a 5-hour value no longer shows as 5h 0%: the 5-hour window stays unknown, or keeps the last status-line reading. Desktop often records only the weekly number.
- A config directory whose login changed is no longer registered under a second name. `hotseat init` skips it with a warning and registers the rest; `hotseat remove` refuses to sign out or move a directory that now holds another login, or that two entries share. New: `hotseat remove <account> --forget` drops an account from Hotseat and leaves its directory and login alone.
- `hotseat share rollback` keeps a copy of the MCP server definitions it replaces, so a server added after the apply is not lost.
- A Claude Desktop switch that fails halfway leaves Desktop closed and points to `hotseat desktop recover`, instead of reopening Desktop on an empty profile that recovery could not sort out.
- Switching only Desktop from the menu asks before restarting a Desktop that was started after the menu was last drawn. New: `hotseat desktop switch <account> --if-idle` exits 3 instead of quitting a running Desktop.
- Usage read from Claude Desktop survives a Desktop switch: readings in parked profiles are still shown. A Desktop reading older than its window (5 hours, 7 days) counts as unknown.
- The menu bar app says so when an account was removed but Claude Code could not sign it out.
- `hotseat desktop adopt` and `hotseat desktop add` honour `--json`.

## 0.1.0

First public release.

- CLI: registers accounts, shares work data, adds a `claude` shim, records usage from the status line, hands off at limits, switches Claude Desktop, and adds, removes, notes and icons accounts. Includes `setup status`, `uninstall` and `doctor`.
- Menu bar app: account cards with usage, one-click switching (both sides, CLI only or Desktop only), Settings, and a first-run Setup window. It carries its own CLI.
- Release builds: universal CLI and app, and a Homebrew formula that builds from source.

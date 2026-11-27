# Changelog

## 0.1.2

- Linux: an experimental top-bar app for GNOME on X11, in `linux/` (`hotseat-indicator`), maintained by the community. It mirrors the menu bar app (account cards, Settings, Setup) and, like it, only runs the `hotseat` CLI.
- Linux: the CLI runs on Linux, including Claude Desktop for Linux. It finds Desktop through /proc, quits it with SIGTERM (never a force-kill: a Desktop that does not exit in time is reported with its PIDs), and starts it with `systemd-run --user gtk-launch`, which needs a systemd user session. Desktop's data is looked for under `XDG_CONFIG_HOME` when set. The shim and aliases go to `~/.bashrc`, or `~/.zshrc` when zsh is the login shell, and the limit handoff asks with zenity.
- `hotseat desktop switch --resume` refuses a session ID that is not a UUID before anything is quit or moved.
- When a Desktop switch succeeds but Desktop cannot be reopened, Hotseat says so instead of reporting a plain failure or success.
- A limit on one model ("You've hit your Fable limit …") is recorded as a limit on that model, not on the account: the account is no longer shown as LIMIT or skipped as a handoff target while its other models still work. `hotseat usage` and the account's card show "Fable limit until …", and a handoff away from a model limit skips accounts limited on the same model. A limit named after the plan ("Max plan", "Pro") stays an account-wide limit, and a model name with a version ("Opus 5.5") is read whole. The handoff dialog and notice name the model too ("hit its Fable limit").
- Hotseat no longer needs an account that lives in `~/.claude`. When every login has its own config dir and `~/.claude` is only the shared workspace, `hotseat share plan`, `share apply` and `doctor` work instead of failing with "no default account"; MCP conflicts are then settled by the first registered account. `hotseat init` no longer moves an account back to `~/.claude` because an old login of the same account is still recorded there.

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

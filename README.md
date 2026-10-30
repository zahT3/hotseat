# Hotseat

Use several personal Claude accounts on one Mac without your work changing when you switch. Accounts are pluggable; sessions, settings, skills and history stay in one place.

Status: early. macOS only. Covers the Claude Code CLI, Claude Desktop and a menu bar app.

## How it works

Claude Code officially supports one config dir per account via `CLAUDE_CONFIG_DIR`, and keeps each dir's login in its own keychain entry. Hotseat builds on that:

- `~/.claude` holds the single real copy of your work data (`projects/`, `history.jsonl`, `file-history/`, `skills/`, `settings.json`, ...).
- Every other account dir (`~/.claude2`, `~/.claude-work`, ...) links those entries back to `~/.claude` and keeps only its own login and caches.
- A small `claude` shim follows the account you pick with `hotseat use`, so `claude --resume` continues the same session on another account's quota.

## What Hotseat never does

- It never reads, copies or stores tokens or credentials: not `.credentials.json`, not the keychain, not `CLAUDE_CODE_OAUTH_TOKEN`. From `.claude.json` it reads only the account and organization IDs, the `mcpServers` key, and the email address (shown in the menu bar app to tell accounts apart). Logins always go through Claude's own `claude auth login`.
- It never switches accounts on its own. Every switch is a command you run.
- It never deletes your data. Anything it replaces is moved into a backup you can roll back.

## Install

```sh
go build -o ~/.local/bin/hotseat ./cmd/hotseat
```

## Use

```sh
hotseat init                 # find existing ~/.claude* logins and register them
hotseat accounts             # list them; * marks the current account
hotseat share plan           # show what would be linked or merged (read-only)
hotseat share apply          # do it; refuses while any claude process is running
hotseat share backups        # list backups
hotseat share rollback <id>  # undo an apply
hotseat shim install         # put the claude shim first on PATH via ~/.zshrc
hotseat use claude2          # new claude sessions now use claude2
hotseat add work             # new config dir + official login, linked to shared work data
hotseat doctor               # report drift, unknown entries, and setup problems
hotseat note claude2 Work     # your own label for an account, shown in the menu bar app
```

`share apply` merges each account's `history.jsonl` into the shared one and copies entries that only exist in an account dir into `~/.claude`. When both sides have a different version of the same file, the shared copy wins and the account's copy stays in the backup; `apply` lists those paths.

Roll back with every claude session closed (it refuses otherwise, like apply). A rollback puts each file back as it was before the apply. Files that changed since then, such as `history.jsonl` with prompts typed after the apply, are moved into `~/.config/hotseat/backups/<id>/rollback-removed/` rather than overwritten, and rollback prints where each one went. For `.claude.json` only the `mcpServers` key is restored; everything Claude Code changed since stays. If a rollback stops on an error, fix the cause and run it again; it continues where it stopped.

## Usage and limits

```sh
hotseat statusline install  # adds a status line to every claude session; keeps yours, shown first
hotseat hook install        # registers a StopFailure hook for rate limits
hotseat usage               # 5-hour and 7-day usage, reset times and data age per account
```

The status line shows `claude2 · 5h 16% · 7d 12%` and records those numbers. They come only from what Claude Code hands to the status line and from Claude Desktop's local usage history, so an account you have not used for a while shows its last known numbers and how old they are; a window whose reset time has passed shows 0%.

When a CLI session stops on a usage limit, the hook records it and opens a dialog offering to continue the same session as the account with the most quota left. Nothing switches unless you click Continue; then that account becomes current and a new Terminal window runs `claude --resume <session>`. Don't keep typing in the old window afterwards. The first message on the new account reads the whole conversation again without cache, and the dialog says roughly how big that is.

## Claude Desktop

Claude Desktop holds one login at a time. Hotseat gives every account its own complete Desktop data dir: the current account's sits at the real path `~/Library/Application Support/Claude`, the others are parked in `~/.config/hotseat/desktop/profiles/<account>/`. Opening Desktop from the Dock, Spotlight or a link, or after an auto-update, therefore always opens the current account.

```sh
hotseat desktop adopt                     # manage the Desktop data that is there now
hotseat desktop adopt work ~/path/to/dir  # park a data dir that is logged in to `work`
hotseat desktop add work                  # empty profile for `work`; log in once in the Desktop that opens
hotseat desktop switch work --resume current
hotseat desktop status                    # which profile is where, and whether its login matches
hotseat desktop recover                   # finish or undo a switch that was interrupted
hotseat use work --yes                    # CLI and Desktop together
```

A switch quits Desktop (it waits; it never force-kills), carries Desktop's own work data over to the target profile, swaps the two dirs, and reopens Desktop on the session you were in through Claude's `claude://resume` link. What is carried over: the Code session list (merged across all accounts, newest copy wins, deleted sessions stay deleted), MCP servers and preferences, extensions, session presets and scratch work. What is not: the login (Cookies, `config.json`, Local Storage), which is exactly what makes each profile a different account. Hotseat reads only `lastKnownAccountUuid` from `config.json` to check which account a profile is logged in to.

Without `--yes`, `hotseat use` leaves a running Desktop alone and tells you; a closed Desktop is switched quietly and opens as the new account next time. When a Desktop session hits a usage limit, the dialog offers to switch Desktop to another account and continue that session there.

Don't use Log out inside Claude Desktop to change accounts: it signs that account out on Anthropic's side, and its Hotseat profile has to log in again. Switch with Hotseat instead.

Hotseat does not use symlinks or `--user-data-dir` for this: symlinked data breaks Cowork's file sharing and recent Desktop versions refuse to save sessions into a symlinked folder, and `--user-data-dir` is lost whenever Desktop relaunches itself after an update.

MCP servers are synced as a union: every account gets every server, so a server removed from only one account comes back on the next apply.

Every command accepts `--json` for scripts and the menu bar app.

Entries Hotseat does not recognise are left alone and reported by `hotseat doctor`. Classify one with `hotseat share set <name> shared|local`. `plugins/` and `sessions/` stay per account for now.

## Menu bar app

`macos/` holds a small menu bar app. It shows every account's 5-hour and 7-day usage, when each resets, how old that data is, which account is current and which one Claude Desktop is on. Click an account to switch to it (the same as `hotseat use <account> --yes`). If Desktop is running and has to restart, it asks first.

```sh
macos/build-app.sh            # builds and ad-hoc signs macos/build/Hotseat.app
macos/build-app.sh --install  # also replaces ~/Applications/Hotseat.app and opens it
macos/test.sh                 # Swift tests
```

The Command Line Tools are enough; Xcode is not needed. The app reads nothing itself and makes no network requests: all of its data comes from `hotseat panel --json`, and switching runs `hotseat use`. It looks for `hotseat` in `~/.local/bin`, `/opt/homebrew/bin` and `/usr/local/bin`; point it elsewhere with `defaults write io.github.zahT3.hotseat hotseatPath /path/to/hotseat`. Turn on Launch at Login from its menu.

## Files

- `~/.config/hotseat/accounts.json`: registered accounts (override the dir with `HOTSEAT_HOME`)
- `~/.config/hotseat/current`: the current account
- `~/.config/hotseat/backups/`: one dir per apply, with a `manifest.json` used by rollback
- `~/.config/hotseat/bin/claude`: the shim, a symlink to `hotseat`

## License

MIT

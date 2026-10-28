# Roster

Use several personal Claude accounts on one Mac without your work changing when you switch. Accounts are pluggable; sessions, settings, skills and history stay in one place.

Status: early. macOS only. This first part covers the Claude Code CLI. Desktop support and a menu bar app come later.

## How it works

Claude Code officially supports one config dir per account via `CLAUDE_CONFIG_DIR`, and keeps each dir's login in its own keychain entry. Roster builds on that:

- `~/.claude` holds the single real copy of your work data (`projects/`, `history.jsonl`, `file-history/`, `skills/`, `settings.json`, ...).
- Every other account dir (`~/.claude2`, `~/.claude-work`, ...) links those entries back to `~/.claude` and keeps only its own login and caches.
- A small `claude` shim follows the account you pick with `roster use`, so `claude --resume` continues the same session on another account's quota.

## What Roster never does

- It never reads, copies or stores tokens or credentials: not `.credentials.json`, not the keychain, not `CLAUDE_CODE_OAUTH_TOKEN`. From `.claude.json` it reads only the account and organization IDs and the `mcpServers` key. Logins always go through Claude's own `claude auth login`.
- It never switches accounts on its own. Every switch is a command you run.
- It never deletes your data. Anything it replaces is moved into a backup you can roll back.

## Install

```sh
go build -o ~/.local/bin/roster ./cmd/roster
```

## Use

```sh
roster init                 # find existing ~/.claude* logins and register them
roster accounts             # list them; * marks the current account
roster share plan           # show what would be linked or merged (read-only)
roster share apply          # do it; refuses while any claude process is running
roster share backups        # list backups
roster share rollback <id>  # undo an apply
roster shim install         # put the claude shim first on PATH via ~/.zshrc
roster use claude2          # new claude sessions now use claude2
roster add work             # new config dir + official login, linked to shared work data
roster doctor               # report drift, unknown entries, and setup problems
```

`share apply` merges each account's `history.jsonl` into the shared one and copies entries that only exist in an account dir into `~/.claude`. When both sides have a different version of the same file, the shared copy wins and the account's copy stays in the backup; `apply` lists those paths.

Roll back with every claude session closed (it refuses otherwise, like apply). A rollback puts each file back as it was before the apply. Files that changed since then, such as `history.jsonl` with prompts typed after the apply, are moved into `~/.config/roster/backups/<id>/rollback-removed/` rather than overwritten, and rollback prints where each one went. For `.claude.json` only the `mcpServers` key is restored; everything Claude Code changed since stays. If a rollback stops on an error, fix the cause and run it again; it continues where it stopped.

Entries Roster does not recognise are left alone and reported by `roster doctor`. Classify one with `roster share set <name> shared|local`. `plugins/` and `sessions/` stay per account for now.

## Files

- `~/.config/roster/accounts.json`: registered accounts (override the dir with `ROSTER_HOME`)
- `~/.config/roster/current`: the current account
- `~/.config/roster/backups/`: one dir per apply, with a `manifest.json` used by rollback
- `~/.config/roster/bin/claude`: the shim, a symlink to `roster`

## License

MIT

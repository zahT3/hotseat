# Security

Hotseat runs with your user's permissions. It edits `~/.zshrc`, `~/.claude/settings.json`, `~/.claude*` and Claude Desktop's data dir. By design, it never reads credentials.

Please report vulnerabilities privately through GitHub's **Report a vulnerability** (Security → Advisories) on this repository, not in a public issue. Include the version (`hotseat version`), what you ran and what happened. Expect a first reply within a week.

Things we especially want to hear about:
- any code path that reads, copies or prints a token or credential;
- data loss, or a switch that leaves Claude Desktop logged in to the wrong account;
- command injection through account names, notes or paths.

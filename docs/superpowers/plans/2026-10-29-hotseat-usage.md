# Hotseat 用量与额度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Hotseat 知道每个账号的 5 小时 / 7 天用量和重置时间（`hotseat usage`），并在 CLI 会话撞到额度上限时弹出对话框，用户点一下就用另一个账号在新终端里接着同一个会话。

**Architecture:** 两个由 Claude Code 调用的入口：`hotseat statusline`（statusLine 命令，记录 `rate_limits` 并输出一行状态）和 `hotseat hook stop-failure`（`StopFailure` hook，matcher `rate_limit`）。hook 立刻返回，另起一个脱离会话的 `hotseat handoff` 进程弹 AppleScript 对话框，用户确认后才切换当前账号并在 Terminal 新窗口里 `claude --resume`。数据都存在 `~/.config/hotseat/usage/<账号>.json`。

**Tech Stack:** Go 1.26 标准库；macOS `osascript`。

**Spec:** `docs/superpowers/specs/2026-10-28-roster-design.md`（项目已改名 Hotseat）。本计划覆盖 6.3 的 statusLine、6.6、6.7（Desktop 的 `plan-usage-history.json` 只读接入；套餐类型留到菜单栏计划），以及第 9 节验收 3、5。

## Global Constraints

- 沿用计划 1 的全部约束：只支持 macOS；只用标准库；状态目录 `$HOTSEAT_HOME`（默认 `~/.config/hotseat`）；不碰任何 token 或凭证；不自动切换账号；不删除用户数据；测试只在临时 HOME 里跑；所有命令支持 `--json`；commit 末尾加 `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- 不发任何网络请求，不调用 `claude` 去查额度（spec 4.1）。用量只来自 Claude Code 自己交给 statusLine / hook 的数据、对话文件里的限额报错文本、Desktop 写在本地的 `plan-usage-history.json`。
- Desktop 会话的识别：环境变量 `CLAUDE_CODE_ENTRYPOINT=claude-desktop`（已验证：本机 4 个 Desktop 进程都带这个值，CLI 进程没有）。本计划对 Desktop 会话只输出 statusLine 的串联部分，不记录用量、不弹接续对话框（Desktop 归计划 3）。
- 改 `~/.claude/settings.json`（三个账号共用）时：只动 `statusLine` 键和 `hooks.StopFailure` 里 Hotseat 自己的条目；其他顶层键按原文字节保留；先备份到 `backups/settings-<时间>.json`；先写临时文件再 rename；如果它是 symlink 就写到真实文件。
- 用户的终端是 Apple Terminal（已验证：`TERM_PROGRAM=Apple_Terminal`）。接续只支持 Terminal。
- 界面文案（对话框、statusLine）用英文，与 CLI 一致。

## Review Focus

1. **statusLine 绝不能拖慢或弄坏状态栏**：stdin 不是 JSON、注册表损坏、usage 目录不可写、串联命令超时，都要在 1 秒内输出一行（至少是账号名或空行）并以 0 退出；不得启动 `claude`、不得联网。（Task 3）
2. **Desktop 会话的用量不能记到默认账号上**：`CLAUDE_CODE_ENTRYPOINT=claude-desktop` 且 `CLAUDE_CONFIG_DIR` 未设置时，`account.FromEnv` 必须返回 false。（Task 2）
3. **改 settings.json 不能丢用户的东西**：已有的 `PreToolUse`、`UserPromptSubmit` hook、其他顶层键和它们的原文都保留；装两次只有一条 Hotseat 条目；卸载后 `statusLine` 恢复成安装前的值（之前没有就删掉）。（Task 4）
4. **hook 不能卡住会话，也不能替用户切号**：`hook stop-failure` 必须在启动 handoff 子进程后立刻返回；只有用户在对话框里点了继续，`current` 才会改变、终端才会打开。（Task 6、Task 8）
5. **接续命令的 shell 引号**：cwd 含空格、单引号、`$`、反引号时，Terminal 里执行的命令仍然只做 `cd` + `claude --resume`，不会执行路径里的内容。（Task 8）

---

## File Structure

```
internal/paths/paths.go           + UsageDir()、ChainFile()
internal/account/env.go           FromEnv
internal/usage/usage.go           Record/Window/LimitHit、Load/Save、ObserveRateLimits、Effective
internal/usage/recommend.go       Recommend
internal/usage/desktop.go         ReadDesktopSamples
internal/settings/settings.go     SetStatusLine/RemoveStatusLine、AddHook/RemoveHook（保留原文）
internal/transcript/transcript.go LastLimitMessage、ParseResetTime、ContextTokens
internal/handoff/handoff.go       Offer（对话框 + 打开 Terminal），Runner 可注入
internal/cli/statusline.go        statusline / statusline install|uninstall
internal/cli/hook.go              hook stop-failure / hook install|uninstall
internal/cli/handoff.go           handoff（hook 起的子进程入口）
internal/cli/usage.go             usage
```

---

### Task 1: 用量存储

**Files:**
- Modify: `internal/paths/paths.go`
- Create: `internal/usage/usage.go`
- Test: `internal/usage/usage_test.go`

**Interfaces:**
- Produces:
  - `(paths.Paths) UsageDir() string` → `<StateDir>/usage`；`(paths.Paths) ChainFile() string` → `<StateDir>/statusline-chain.json`
  - `usage.Window{UsedPercent float64; ResetsAt int64; ObservedAt int64; Source string}`（JSON：`usedPercent`、`resetsAt`、`observedAt`、`source`；时间都是 Unix 秒；`ResetsAt==0` 表示未知）
  - `usage.LimitHit{Message string; ResetsAt int64; ObservedAt int64; SessionID, Cwd string}`
  - `usage.Record{Account string; FiveHour, SevenDay *Window; Limit *LimitHit}`（JSON：`account`、`fiveHour`、`sevenDay`、`limit`）
  - `usage.Load(p, name) (Record, error)`（文件不存在返回 `Record{Account:name}`）、`usage.Save(p, Record) error`（原子写，0600）
  - `usage.RateLimits{FiveHour, SevenDay *struct{UsedPercentage float64 \`json:"used_percentage"\`; ResetsAt int64 \`json:"resets_at"\`}}`（对应 statusLine JSON 的 `rate_limits`）
  - `usage.ObserveRateLimits(p, name string, rl RateLimits, now time.Time) error`：只覆盖 rl 里出现的窗口，`Source="statusline"`；两个窗口的百分比和重置时间都没变时不写文件；看到某窗口的新数据时，如果 `Limit` 的重置时间已过，清掉 `Limit`
  - `usage.Effective(w *Window, now time.Time) (pct float64, known, reset bool)`：`w==nil` → known=false；`ResetsAt>0 && ResetsAt<=now` → pct=0、reset=true

- [ ] **Step 1: 写失败测试**：`TestObserveWritesOnlyOnChange`（同样的数据第二次调用后文件 mtime 不变）、`TestObserveKeepsMissingWindow`（只给 five_hour 时 SevenDay 保留旧值）、`TestEffectiveAfterReset`（resetsAt 已过 → 0、reset=true）、`TestLoadMissingReturnsEmpty`、`TestObserveClearsExpiredLimit`。
- [ ] **Step 2: 运行 `go test ./internal/usage/`，预期失败。**
- [ ] **Step 3: 实现。**
- [ ] **Step 4: 运行 `go test ./...`，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: add per-account usage store`

---

### Task 2: 从环境判断是哪个账号

**Files:**
- Create: `internal/account/env.go`
- Test: `internal/account/env_test.go`

**Interfaces:**
- Produces: `account.FromEnv(reg Registry, env []string) (Account, bool)`：
  - `CLAUDE_CONFIG_DIR` 非空 → 在注册表里找 ConfigDir 相同的账号（先 `filepath.Clean`，再比较 `EvalSymlinks` 结果）；找不到返回 false。
  - 为空或未设置 → 若 `CLAUDE_CODE_ENTRYPOINT=claude-desktop` 返回 false；否则返回默认账号。

- [ ] **Step 1: 写失败测试**：`TestFromEnvExplicitDir`、`TestFromEnvDefault`、`TestFromEnvDesktopIsUnknown`、`TestFromEnvUnregisteredDir`、`TestFromEnvSymlinkedDir`。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: resolve the account of a claude process from its env`

---

### Task 3: `hotseat statusline`

**Files:**
- Create: `internal/cli/statusline.go`
- Test: `internal/cli/statusline_test.go`

**Interfaces:**
- Consumes: Task 1、Task 2
- Produces: 命令 `hotseat statusline`，从 stdin 读 statusLine JSON（只用 `rate_limits`），环境取自 `os.Environ()`。为了测试，`cli.Run` 的 stdin 用包变量 `var stdin io.Reader = os.Stdin`。

行为：
1. 读完 stdin（最多 1 MiB），解析失败当作没有 `rate_limits`。
2. `FromEnv` 得到账号 → `ObserveRateLimits`；任何错误都忽略。
3. 如果 `ChainFile()` 里有 `{"command": "..."}`，用 `sh -c` 运行它，stdin 喂同一份原始字节，超时 800ms，取输出第一行。
4. 输出一行：`<串联输出> | <自己的部分>`（没有串联就只有自己的部分）。自己的部分：`<账号名> · 5h <pct>% · 7d <pct>%`，百分比取整，未知的窗口省略；Desktop 会话或识别不出账号时自己的部分为空。
5. 永远以 0 退出。

- [ ] **Step 1: 写失败测试**：`TestStatuslineRecordsAndPrints`（claude2 环境，输入 `{"rate_limits":{"five_hour":{"used_percentage":16.4,"resets_at":2000000000},"seven_day":{"used_percentage":12,"resets_at":2000300000}}}` → 输出 `claude2 · 5h 16% · 7d 12%`，usage 文件里 FiveHour.ResetsAt=2000000000）、`TestStatuslineGarbageInput`（输入 `not json` → 退出 0，输出 `claude2`）、`TestStatuslineDesktopRecordsNothing`、`TestStatuslineChainsPrevious`（ChainFile 里 `echo prev` → 输出以 `prev | ` 开头）、`TestStatuslineChainTimeout`（`sleep 5` → 1 秒内返回，退出 0）、`TestStatuslineCorruptRegistry`（退出 0，输出空行）。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: record rate limits from the statusLine`

---

### Task 4: 编辑 settings.json，安装 statusLine 和 hook

**Files:**
- Create: `internal/settings/settings.go`, `internal/cli/hook.go`（本任务只做 install/uninstall）
- Modify: `internal/cli/statusline.go`（加 install/uninstall）
- Test: `internal/settings/settings_test.go`, `internal/cli/hook_test.go`

**Interfaces:**
- Produces（settings 包，`file` 都是 settings.json 路径）：
  - `settings.TopLevel(b []byte) ([]Member, error)`，`Member{Key string; ValueStart, ValueEnd int}`：用 `json.Decoder` 逐个读顶层键，记下每个值在原文里的字节范围
  - `settings.SetStatusLine(file, command string) (previous json.RawMessage, err error)`：值写成 `{"type":"command","command":<command>}`；有旧值时只替换那段字节，没有时插在最后一个顶层成员之后
  - `settings.RestoreStatusLine(file string, previous json.RawMessage) error`：previous 为空就删掉整个键（连同前面的逗号）
  - `settings.AddHook(file, event, matcher, command string) error`、`settings.RemoveHook(file, event, command string) error`：只重写 `hooks` 这一个值（解析成 `map[string]json.RawMessage` 后重新编码，缩进与顶层一致）；同一 command 不重复添加；删掉后某事件为空就删掉该事件键
  - 所有写入：先把原文件复制到调用方给的备份路径，再原子写到 `throughLink(file)`
- CLI：
  - `hotseat statusline install`：把现有 `statusLine` 里不是 Hotseat 的命令存进 `ChainFile()`（`{"command": ...}`），再设置为 `<hotseat 绝对路径> statusline`；`uninstall` 恢复（Chain 里有就恢复成那个命令，否则删键）并删掉 ChainFile
  - `hotseat hook install`：`AddHook(..., "StopFailure", "rate_limit", "<hotseat 绝对路径> hook stop-failure")`；`uninstall` 对应删除
  - 判断"是不是 Hotseat 自己的命令"：命令以 ` statusline` / ` hook stop-failure` 结尾，且第一个词的 basename 是 `hotseat`

- [ ] **Step 1: 写失败测试**（settings 包，用本机 settings.json 的结构做夹具：顶层有 `env`、`permissions`、`hooks{PreToolUse,UserPromptSubmit}`、`enabledPlugins` 等）：
  - `TestSetStatusLineKeepsOtherBytes`：除 `statusLine` 那段外，其他顶层成员的原文字节完全一致
  - `TestRestoreStatusLineRemovesKey`、`TestRestoreStatusLinePrevious`
  - `TestAddHookKeepsExisting`：PreToolUse、UserPromptSubmit 的内容解析后与原来相等；StopFailure 里有一条 matcher `rate_limit`
  - `TestAddHookIdempotent`、`TestRemoveHookRemovesEmptyEvent`
  - `TestWritesThroughSymlink`、`TestBackupWritten`
  - CLI：`TestStatuslineInstallChainsExisting`（原有 `statusLine.command="echo prev"` → ChainFile 记下它，uninstall 后 settings 里又是 `echo prev`）
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: install the statusLine and StopFailure hook into settings.json`

---

### Task 5: 读对话文件

**Files:**
- Create: `internal/transcript/transcript.go`
- Test: `internal/transcript/transcript_test.go`

**Interfaces:**
- Produces:
  - `transcript.LastLimitMessage(path string) (string, bool)`：从文件末尾往前（最多读最后 256 KiB），找最后一条 `type=="assistant"` 且正文包含 `limit` 和 `resets` 的文本
  - `transcript.ParseResetTime(msg string, now time.Time) (time.Time, bool)`：认 `resets <时间>[ (<IANA 时区>)]`，`<时间>` 支持 `Nov 1 at 6pm`、`Nov 1 at 6:30pm`、`6pm`、`6:30pm`、`18:00`；没给日期时取 now 之后最近的那个时刻；没给时区用 `time.Local`；给了无法识别的时区就返回 false
  - `transcript.ContextTokens(path string) int`：最后一条带 `message.usage` 的 assistant 记录里 `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`；读不到返回 0

- [ ] **Step 1: 写失败测试**：`TestParseResetObservedFormat`（`You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)`，now=2026-10-28 → 2026-11-01 18:00 +08:00；这是本机实际出现过的原文）、`TestParseResetTimeOnlyRollsForward`（now 19:00、`resets 6pm` → 次日 18:00）、`TestParseResetUnknownZone`、`TestLastLimitMessageFindsLast`、`TestContextTokens`、`TestTranscriptMissingFile`。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: read limit messages and context size from transcripts`

---

### Task 6: 推荐接手的账号

**Files:**
- Create: `internal/usage/recommend.go`
- Test: `internal/usage/recommend_test.go`

**Interfaces:**
- Produces: `usage.Recommend(reg account.Registry, records map[string]Record, exclude string, now time.Time) (string, bool)`：候选排除 `exclude` 和 `Limit` 仍有效（`ResetsAt>now`，或 `ResetsAt==0` 且 `ObservedAt` 在 5 小时内）的账号；分数 = 两个窗口 `Effective` 百分比的较大值（未知按 0）；取分数最低者，同分按注册表顺序。

- [ ] **Step 1: 写失败测试**：`TestRecommendLowestUsage`、`TestRecommendSkipsLimited`、`TestRecommendResetWindowCountsAsZero`、`TestRecommendNoneAvailable`。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: recommend which account takes over`

---

### Task 7: `hotseat hook stop-failure`

**Files:**
- Modify: `internal/cli/hook.go`
- Test: 扩充 `internal/cli/hook_test.go`

**Interfaces:**
- Consumes: Task 1、2、5、6
- Produces: 命令 `hotseat hook stop-failure`。包变量 `var spawnHandoff = func(args []string) error`（默认：以 `Setsid` 启动 `<自身> handoff <args...>`，stdin/stdout/stderr 都接 `/dev/null`，`Start` 后 `Release`，不等待）。

行为：
1. 读 stdin JSON：`session_id`、`transcript_path`、`cwd`，以及若存在的 `error`/`message` 字符串字段（官方没写清楚，有就用）。
2. `FromEnv` 识别账号；识别不出（含 Desktop）→ 直接退出 0。
3. 限额文本：优先用 stdin 里的字段，否则 `LastLimitMessage(transcript_path)`；`ParseResetTime` 得到重置时间（失败记 0）。
4. 写 `Record.Limit`。
5. `Recommend` 得到接手账号 B（可能没有）。
6. 调 `spawnHandoff(["--from", A, "--to", B, "--session", id, "--cwd", cwd, "--tokens", N, "--resets", unix, "--message", text])`；没有 B 时传 `--to ""`（handoff 只提示，不提供继续）。
7. 立即以 0 退出，不往 stdout 写任何东西。

- [ ] **Step 1: 写失败测试**：`TestStopFailureRecordsLimitAndSpawns`（假 transcript 含上面那条原文；断言 usage 文件里 Limit.ResetsAt 正确、spawnHandoff 收到 `--to claude3`）、`TestStopFailureDesktopDoesNothing`、`TestStopFailureNoAlternative`（只有一个账号 → `--to ""`）、`TestStopFailureBadInput`（stdin 不是 JSON → 退出 0，不调用 spawn）。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: handle StopFailure rate limits`

---

### Task 8: `hotseat handoff`（对话框 + 接续）

**Files:**
- Create: `internal/handoff/handoff.go`, `internal/cli/handoff.go`
- Test: `internal/handoff/handoff_test.go`

**Interfaces:**
- Produces:
  - `handoff.Request{From, To, SessionID, Cwd, Message string; Tokens int; ResetsAt int64}`
  - `handoff.Runner` 接口：`Dialog(text string, buttons []string, defaultButton string) (pressed string, err error)`、`Notify(title, text string) error`、`OpenTerminal(command string) error`；默认实现 `handoff.OSA{}` 用 `osascript`（`display dialog ... giving up after 600`、`display notification`、`tell application "Terminal" to do script ... / activate`），AppleScript 字符串里的 `\` 和 `"` 要转义
  - `handoff.Offer(p paths.Paths, reg account.Registry, r Request, run Runner) error`
  - `handoff.ResumeCommand(a account.Account, cwd, sessionID string) string`

`Offer` 行为：
- `To==""`：只 `Notify("<From> hit its limit", "resets <本地时间>; no other account has quota left")`，结束。
- 否则对话框正文：`<From> hit its usage limit (<Message>).` 换行 `Continue this session as <To>?` 换行 `Context is about <Tokens/1000>k tokens; the first message on <To> re-reads it without cache.` 换行 `Do not keep typing in the old window.` 按钮 `["Not now", "Continue as <To>"]`，默认第二个。
- 按下继续：`account.SetCurrent(p, To)`，然后 `OpenTerminal(ResumeCommand(...))`。其他按钮或超时：什么都不改。

`ResumeCommand`：`cd <sq(cwd)> && CLAUDE_CONFIG_DIR=<sq(dir)> claude --resume <sq(session)>`；默认账号用 `env -u CLAUDE_CONFIG_DIR claude --resume ...`。`sq` 是 POSIX 单引号转义（`'` → `'\''`）。

- [ ] **Step 1: 写失败测试**（假 Runner 记录调用）：`TestOfferContinueSwitchesAndOpens`、`TestOfferDeclineChangesNothing`、`TestOfferNoAlternativeNotifies`、`TestResumeCommandQuotesHostilePath`（cwd=`/tmp/a b/it's $(rm -rf x)` → 用 `sh -n` 校验语法，并用 `sh -c` 在把 `claude` 换成 `echo` 的版本里执行，断言只打印出参数、没有执行替换）、`TestResumeCommandDefaultUnsetsDir`、`TestAppleScriptEscaping`。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: offer to continue a limited session on another account`

---

### Task 9: `hotseat usage`

**Files:**
- Create: `internal/usage/desktop.go`, `internal/cli/usage.go`
- Test: `internal/usage/desktop_test.go`, `internal/cli/usage_test.go`

**Interfaces:**
- Produces:
  - `usage.ReadDesktopSamples(file string) (map[string]Sample, error)`，`Sample{T int64; FiveHour, SevenDay float64}`：读 `plan-usage-history.json`，按 `org` 取最新一条；`t` 是毫秒，转成秒；文件不存在返回空 map
  - `(paths.Paths) DesktopDataDir() string` → `$HOME/Library/Application Support/Claude`
  - 命令 `hotseat usage [--json]`：每个账号一行 `* claude2  5h 16% (resets 15:20)  7d 12% (resets Sat 18:00)  updated 3m ago`；有效的 Limit 显示 `LIMIT until <时间>`；某个窗口只有 Desktop 数据时标 `(desktop)`。Desktop 样本按 `OrgUUID` 对应账号，比 statusLine 数据新时才采用其百分比，重置时间沿用 statusLine 的（如果仍在将来）。

- [ ] **Step 1: 写失败测试**：`TestReadDesktopSamplesLatestPerOrg`、`TestUsageMergesDesktopWhenNewer`、`TestUsageShowsResetAsZero`、`TestUsageJSON`。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat: show usage for every account`

---

### Task 10: 本机安装与验收

**Files:**
- Modify: `README.md`

- [ ] **Step 1: README** 补上 `usage`、`statusline install`、`hook install` 的用法，以及"接续只在你点按钮时发生"、"数据来自 statusLine，闲置账号只显示上次的数"两条说明。
- [ ] **Step 2: 运行 `go test ./...`，预期全部 PASS。Commit**：`docs: usage and handoff`
- [ ] **Step 3: 本机安装**（会改三个账号共用的 `~/.claude/settings.json`，正在跑的会话会自动重新加载设置，状态栏会出现）：`go build -o ~/.local/bin/hotseat ./cmd/hotseat && hotseat statusline install && hotseat hook install`。贴出 settings.json 里 `statusLine` 和 `hooks.StopFailure` 两段，以及 `PreToolUse`/`UserPromptSubmit` 仍在的证据。
- [ ] **Step 4: 用真实会话验收 statusLine**：等任意一个 CLI 会话完成一次回复后，`hotseat usage` 里该账号出现 5h/7d 数据。
- [ ] **Step 5: 接续流程演练**：用一个临时目录里的假 transcript（含本机出现过的限额原文）构造 stdin，运行 `CLAUDE_CONFIG_DIR=~/.claude2 hotseat hook stop-failure`，请用户在弹出的对话框里点 "Continue as …"，确认 Terminal 新窗口打开、命令是 `claude --resume <假 id>`（会报找不到会话，属预期），然后 `hotseat use claude` 切回。

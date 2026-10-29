# Hotseat Desktop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `hotseat use B` 之后，Claude Desktop 重启就是账号 B 的登录，Code 会话列表、Desktop 的 MCP / 偏好 / 扩展都和切换前一样，刚才那条会话自动打开；从 Dock、Spotlight 打开 Desktop 也永远是当前账号。Desktop 里撞到额度时，接续对话框能直接把 Desktop 切到 B 接着做。

**Architecture:** 每个账号一份完整的 Desktop 数据目录。当前账号那份始终在真实路径 `~/Library/Application Support/Claude`（不用 symlink），其余的停放在 `~/.config/hotseat/desktop/profiles/<账号>/`。切换 = 确认 Desktop 全部退出 → 把 Desktop 自己那部分"工作数据"从旧目录同步进新目录 → 记日志的两步改名交换目录 → 用官方链接 `claude://resume?session=<id>` 打开刚才的会话。登录凭证始终由 Desktop 自己保管在各自目录里，Hotseat 不读、不复制。

**Tech Stack:** Go 1.26 标准库；macOS `open`、`osascript`、`ps`。

**Spec:** `docs/superpowers/specs/2026-10-28-roster-design.md` 6.4、6.5（项目已改名 Hotseat）。与 spec 的差异（用户 2026-10-29 已同意）：用"真实路径上交换目录"取代 spec 里的 `--user-data-dir` 多实例 + symlink 设想；会话列表采用完全一致（spec 6.4 的 b 方案）；"打开刚才的会话"用官方 resume 链接。

## 已验证的事实（2026-10-29 本机实测，Desktop 4.29 / Claude Code 2.1.310）

- `claude://resume?session=<CLI 会话 UUID>` 会被正在运行的 Desktop 接收，调用 `importCliSession`：在当前登录账号的 `claude-code-sessions/<acct>/<org>/` 下建 `local_<UUID>.json`，打开会话，续聊用当前账号额度，新内容写回 `~/.claude/projects/` 的同一份 transcript（`entrypoint: claude-desktop`）。Desktop 未运行时 `open` 这个 URL 会先启动它。
- 把另一账号的完整数据目录放到 `~/Library/Application Support/Claude`（真目录，非 symlink），从 Dock 启动即是该账号，会话可正常续聊。
- `CLAUDE_USER_DATA_DIR`（launchd 环境变量）在 4.29 正式版不生效；symlink 方案有两个独立的社区报告说坏（Cowork VirtioFS；Desktop 2026-09 起拒绝写入 symlink 的会话目录）——都不采用。
- Desktop 的 Code 模式用默认配置目录 `~/.claude`，所以对话内容、CLAUDE.md、skills、hooks 本来就共享。
- Pro/Max 自 2026-10-06 起 Cowork 只在云端运行，本地 VM（`vm_bundles`）不再需要同步。
- 数据目录里与账号无关、需要同步的：`claude-code-sessions/`（会话索引，按 `<acct>/<org>/` 分文件夹，Desktop 只读当前账号那个文件夹，且只在启动时读）、`scratch-workspaces/`（部分会话的 cwd 指向这里）、`claude_desktop_config.json`（`mcpServers`、`preferences`，其中 `*ByAccount` 键按账号 UUID 存值）、`Claude Extensions/`、`Claude Extensions Settings/`、`extensions-installations.json`、`ccd-session-presets.json`（`byAccount`）。
- 账号身份：`config.json` 顶层的 `lastKnownAccountUuid`。该文件同时含 `oauth:tokenCache*`，只能流式读取这一个键（见 Global Constraints）。

## Global Constraints

- 沿用计划 1、2 的全部约束（macOS、标准库、`$HOTSEAT_HOME`、测试只在临时 HOME、所有命令 `--json`、不删用户数据、commit trailer）。
- **凭证红线**：不读取、不复制、不比较 `Cookies*`、`Local Storage/`、`Session Storage/`、`IndexedDB/`、`config.json` 里除 `lastKnownAccountUuid` 之外的任何值。读 `config.json` 只能用 `json.Decoder` 逐个 token 扫顶层键，遇到其他键用 `dec.Decode(&json.RawMessage{})` 跳过且不保存。整个目录随改名移动是允许的（文件内容不经过 Hotseat）。
- **只在 Desktop 完全退出时**交换或同步：`/Applications/Claude.app/Contents/` 下除 `Helpers/chrome-native-host` 外的任何进程，或可执行文件位于 `~/Library/Application Support/Claude/` 下的任何进程，都算"在运行"。（已验证：`chrome-native-host` 不打开数据目录里的文件。）
- 不用 symlink；不用 `CLAUDE_USER_DATA_DIR`；不修改 Claude.app。
- 被替换或移走的东西进 `$HOTSEAT_HOME/backups/desktop-<时间>/`，不删除。
- 与真实 Desktop 交互（进程列表、退出、打开 URL、启动）全部经过 `desktop.Driver` 接口，测试用假实现；只有 Task 10 碰真机。

## Review Focus

1. **交换过程任何一步中断都能恢复，且默认路径永远不会悄悄变成别的账号**：日志先写后做；`desktop recover` 能把半完成的交换做完或撤回；交换后、启动前核对默认路径下的 `lastKnownAccountUuid` 是目标账号（空目录的新账号除外）。（Task 5、7）
2. **会话索引合并只增不减**：同名索引取 `lastActivityAt` 更新的；`deleted_<id>` 墓碑会传播且被墓碑的会话不再复制；transcript 已不存在的索引不传播；目标里原有而将被替换的文件先进备份。（Task 3）
3. **`*ByAccount` / `byAccount` 是重映射不是照抄**：目标账号的键取源账号的值，其他账号的键原样保留。（Task 4）
4. **凭证文件不被读、不被复制**：同步清单是白名单；测试断言假 `Cookies`、`Local Storage`、`config.json` 里的假 token 字符串从未出现在目标目录的新文件、日志、备份清单和任何输出中。（Task 1、4）
5. **用户在某个 profile 里登录了别的账号时不会被悄悄改标签**：`desktop status`/`switch` 发现 `lastKnownAccountUuid` 与该 profile 记录的账号不符时报错并说明，不自动改。（Task 1、7）

---

## File Structure

```
internal/paths/paths.go             + DesktopStoreDir()、DesktopProfileDir(name)、DesktopStateFile()、DesktopJournal()
internal/desktop/identity.go        AccountUUID(dir)（流式读 lastKnownAccountUuid）
internal/desktop/state.go           State{Active string; Profiles []string}、LoadState/SaveState
internal/desktop/driver.go          Driver 接口、Proc、MacDriver（ps / osascript / open）
internal/desktop/sessions.go        SyncSessions
internal/desktop/settings.go        SyncSettings（config、扩展、presets、scratch）
internal/desktop/swap.go            Swap、Recover（日志）
internal/desktop/switch.go          Switch、CurrentSession
internal/cli/desktop.go             desktop status|adopt|add|switch|recover
internal/cli/use.go                 use 同时切 Desktop
internal/cli/statusline.go / stopfailure.go / handoff 扩展：Desktop 会话归属与接续
```

---

### Task 1: 身份、状态与 `desktop status`

**Files:** Create `internal/desktop/identity.go`, `internal/desktop/state.go`, `internal/cli/desktop.go`；Modify `internal/paths/paths.go`；Test `internal/desktop/identity_test.go`, `internal/cli/desktop_test.go`

**Interfaces:**
- `paths`：`DesktopStoreDir()` → `<StateDir>/desktop`；`DesktopProfileDir(name)` → `<StateDir>/desktop/profiles/<name>`；`DesktopStateFile()` → `<StateDir>/desktop/state.json`；`DesktopJournal()` → `<StateDir>/desktop/swap.json`
- `desktop.AccountUUID(dataDir string) (string, error)`：流式扫 `config.json` 顶层，只返回 `lastKnownAccountUuid`；文件不存在或无此键返回 `""`、nil
- `desktop.State{Active string \`json:"active"\`; Profiles map[string]string \`json:"profiles"\`}`（Profiles：账号名 → 该 profile 最近一次观察到的 accountUuid，空串表示还没登录）、`LoadState(p)`、`SaveState(p, s)`
- 命令 `hotseat desktop status [--json]`：列出 active、各 profile（是否存在、记录的 uuid、实际 uuid、是否一致）、Desktop 是否在运行（Task 2 之后接上 Driver，本任务先不显示运行状态）

- [ ] **Step 1: 写失败测试**：`TestAccountUUIDReadsOnlyThatKey`（config.json 含 `"oauth:tokenCache":"SECRET-TOKEN"` 和 `lastKnownAccountUuid`，返回值只等于 uuid）、`TestAccountUUIDMissing`、`TestStateRoundTrip`、`TestDesktopStatusFlagsMismatch`（profile 记录 a、实际 b → 输出含 `mismatch`，退出码 1）、`TestDesktopStatusNeverPrintsTokens`（假 token 不出现在 stdout/`--json`）。
- [ ] **Step 2–4: 失败 → 实现 → `go test ./...` PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): identity, state and status`

---

### Task 2: Desktop 进程检测与退出（Driver）

**Files:** Create `internal/desktop/driver.go`；Test `internal/desktop/driver_test.go`

**Interfaces:**
- `desktop.Proc{PID int; Exe string}`
- `desktop.Driver` 接口：`Procs() ([]Proc, error)`、`Quit() error`、`OpenURL(url string) error`、`Launch() error`
- `desktop.Running(procs []Proc, home string) []Proc`：按 Global Constraints 的规则筛出"算在运行"的进程（纯函数，便于测试）
- `desktop.QuitAndWait(d Driver, home string, timeout time.Duration) error`：已退出直接返回；否则 `Quit()` 后每 500ms 检查，超时返回 `ErrStillRunning`（含 pid 列表）。**不强杀。**
- `desktop.MacDriver{}`：`Procs` 用 `ps -axo pid=,comm=`（comm 给完整可执行路径）；`Quit` 用 `osascript -e 'tell application id "com.anthropic.claudefordesktop" to quit'`；`OpenURL` 用 `open <url>`；`Launch` 用 `open -b com.anthropic.claudefordesktop`

- [ ] **Step 1: 写失败测试**：`TestRunningIgnoresChromeNativeHost`、`TestRunningCountsHelpersAndBundledCLI`（`/Applications/Claude.app/Contents/Frameworks/Claude Helper.app/...`、`$HOME/Library/Application Support/Claude/claude-code/2.1.309/claude` 都算）、`TestRunningIgnoresTerminalClaude`（`$HOME/.local/share/claude/versions/2.1.310` 不算）、`TestQuitAndWaitTimesOut`（假 Driver 一直有进程 → 超时错误含 pid，且 Quit 只调一次）、`TestQuitAndWaitSucceeds`。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): detect and quit Claude Desktop`

---

### Task 3: 会话索引合并

**Files:** Create `internal/desktop/sessions.go`；Test `internal/desktop/sessions_test.go`

**Interfaces:**
- `desktop.SyncSessions(src, dst string, dstAcct, dstOrg string, transcripts map[string]bool, backup string) (SyncReport, error)`：
  - 来源：`src` 和 `dst` 里 `claude-code-sessions/*/*/` 下所有 `local_*.json` 与 `deleted_*`
  - 目标：`dst/claude-code-sessions/<dstAcct>/<dstOrg>/`（不存在就建）
  - 规则：墓碑集合 = 所有 `deleted_<id>`；墓碑传播到目标文件夹；目标里已有但被墓碑的 `local_<id>` 移进 `backup`；其余 `local_<id>` 取 `lastActivityAt` 最大的那份（解析失败当 0）写入目标（目标原文件较旧时先进 `backup`）；`cliSessionId` 不在 `transcripts` 里的不复制
  - 只用 rename/原子写，不删除
- `desktop.SyncReport{Copied, Updated, Tombstoned, SkippedDead int}`
- `desktop.TranscriptIDs(projectsDir string) (map[string]bool, error)`：扫 `~/.claude/projects/*/*.jsonl` 文件名（不读内容）

- [ ] **Step 1: 写失败测试**：`TestSyncSessionsUnionIntoTargetFolder`、`TestSyncSessionsNewerWins`（两份同 id，目标旧 → 被替换且旧的进 backup；目标新 → 不动）、`TestSyncSessionsTombstoneWins`、`TestSyncSessionsSkipsDeadTranscript`、`TestSyncSessionsIdempotent`（连跑两次第二次 report 全 0）。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): union-sync Code session indexes`

---

### Task 4: 设置、扩展、presets、scratch 同步

**Files:** Create `internal/desktop/settings.go`；Test `internal/desktop/settings_test.go`

**Interfaces:**
- `desktop.SyncSettings(src, dst, srcAcct, dstAcct, backup string) error`，白名单：
  - `claude_desktop_config.json`：以 `src` 为准写入 `dst`；对 `preferences` 里所有以 `ByAccount` 结尾的对象，`dst[...][dstAcct] = src[...][srcAcct]`（src 没有则保留 dst 原值），其余账号键保留 dst 原值
  - `ccd-session-presets.json`：同样处理 `byAccount`
  - `Claude Extensions/`、`Claude Extensions Settings/`、`extensions-installations.json`：`dst` 原有的先进 `backup`，再从 `src` 复制（目录用 copyTree）
  - `scratch-workspaces/`：并集复制（只补缺失文件，不覆盖）
- 不在白名单里的一律不碰

- [ ] **Step 1: 写失败测试**：`TestSyncSettingsRemapsByAccount`、`TestSyncSettingsCopiesMCP`、`TestSyncSettingsMirrorsExtensionsWithBackup`、`TestSyncSettingsScratchUnion`、`TestSyncSettingsNeverTouchesCredentials`（src 有假 `Cookies`、`Local Storage/x`、`config.json` 含 `SECRET-TOKEN` → dst 里这些文件内容不变或不存在，backup 里也没有）。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): sync Desktop settings, extensions and scratch work`

---

### Task 5: 记日志的目录交换与恢复

**Files:** Create `internal/desktop/swap.go`；Test `internal/desktop/swap_test.go`

**Interfaces:**
- `desktop.Journal{From, To, Step string}`（Step ∈ `start`、`parked`、`done`）
- `desktop.Swap(p paths.Paths, from, to string) error`：前置：`from` 的 profile 目录不存在、`to` 的 profile 目录存在（新账号可为空目录）；写日志 `start` → rename 默认路径 → `profiles/<from>`，写 `parked` → rename `profiles/<to>` → 默认路径，写 `done` 后删除日志文件（日志是 Hotseat 自己的状态文件，可删）
- `desktop.Recover(p paths.Paths) (string, error)`：无日志 → 无事可做；`start` 且默认路径仍在 → 撤销（删日志）；`parked` → 若 `profiles/<to>` 还在则完成第二步，否则说明情况并报错；返回说明文字
- 测试钩子 `var beforeSwapStep func(step string)`

- [ ] **Step 1: 写失败测试**：`TestSwapMovesDirs`、`TestSwapRefusesWhenTargetMissing`、`TestRecoverAfterCrashAfterPark`（钩子在第二步前 panic/返回错误 → Recover 完成交换）、`TestRecoverAfterCrashAtStart`、`TestRecoverNothingToDo`。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): journaled profile swap with recovery`

---

### Task 6: `desktop adopt`

**Files:** Modify `internal/cli/desktop.go`；Test 扩充 `internal/cli/desktop_test.go`

**Interfaces:**
- `hotseat desktop adopt`：把默认路径当前的目录登记为 `AccountUUID` 对应的注册账号，设为 active（找不到对应账号 → 报错，提示先 `hotseat init`/`add`）
- `hotseat desktop adopt <账号> <目录>`：把一个现成的 Desktop 数据目录移进 `profiles/<账号>`；要求该目录的 `AccountUUID` 等于注册表里该账号的 UUID（否则报错）；Desktop 必须未运行

- [ ] **Step 1: 写失败测试**：`TestAdoptActive`、`TestAdoptActiveUnknownAccount`、`TestAdoptDirMovesIntoStore`、`TestAdoptDirWrongAccountRefused`。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): adopt existing Desktop data`

---

### Task 7: `desktop switch` 与 `desktop add`

**Files:** Create `internal/desktop/switch.go`；Modify `internal/cli/desktop.go`；Test `internal/desktop/switch_test.go`, `internal/cli/desktop_test.go`

**Interfaces:**
- `desktop.CurrentSession(dataDir, acct, org string) (string, bool)`：当前账号文件夹里 `lastFocusedAt` 最大的索引的 `cliSessionId`
- `desktop.Switch(p, reg, d Driver, to string, opts SwitchOptions) (SwitchResult, error)`，`SwitchOptions{Resume string /* "" | "current" | 会话 UUID */; Timeout time.Duration}`：
  1. `Recover` 若有残留日志先处理；`to` 等于 active 且无 Resume → 无事可做
  2. 记下 `current` 会话（Resume=="current"）
  3. `QuitAndWait`；失败则返回错误，什么都不改
  4. 核对默认路径的 `AccountUUID`：State 里 active 记录的 uuid 非空时必须相等；为空（`desktop add` 后刚登录）时必须等于注册表里该账号的 UUID，相等则写进 State。不符一律报错（Review Focus 5）
  5. `SyncSessions`、`SyncSettings`（src=默认路径，dst=`profiles/<to>`，dst 账号/组织取注册表）
  6. `Swap`；更新 State
  7. 有 Resume → `OpenURL("claude://resume?session=" + id)`，否则 `Launch()`
- `desktop add <账号>`：建空 `profiles/<账号>`，执行 Switch（不带 Resume），Desktop 打开到登录页；提示用户登录；下一次 `status`/`switch` 时把观察到的 uuid 记进 State，若与注册表不符则报错
- CLI：`hotseat desktop switch <账号> [--resume current|<id>] [--json]`、`hotseat desktop add <账号>`

- [ ] **Step 1: 写失败测试**（假 Driver + 临时 HOME 里的假数据目录）：`TestSwitchSyncsSwapsAndResumes`（会话、MCP 进了新 profile；默认路径换成目标；OpenURL 收到 `claude://resume?session=<current 的 cliSessionId>`）、`TestSwitchRefusesWhileRunning`（QuitAndWait 超时 → 默认路径、profiles 都没变）、`TestSwitchRefusesIdentityMismatch`、`TestSwitchNoopSameAccount`、`TestAddCreatesEmptyProfileAndLaunches`。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat(desktop): switch Desktop between accounts`

---

### Task 8: `hotseat use` 同时切 Desktop

**Files:** Modify `internal/cli/use.go`；Test 扩充

**Interfaces:**
- `hotseat use <账号> [--cli-only] [--yes]`：先切 CLI 当前账号（原行为）；若 Desktop 已被接管（State 里有 active）且 `<账号>` 有 profile：Desktop 在运行时，没有 `--yes` 就在 stdout 说明"将退出 Desktop 并以 <账号> 重开，继续当前会话"并要求加 `--yes` 重试（退出码 3）；有 `--yes` 则 `Switch(..., Resume:"current")`。`<账号>` 没有 profile 时提示 `hotseat desktop add <账号>`，CLI 部分照常完成

- [ ] **Step 1: 写失败测试**：`TestUseSwitchesDesktopWithYes`、`TestUseAsksBeforeQuittingDesktop`、`TestUseCliOnly`、`TestUseNoDesktopProfileHint`。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat: hotseat use switches Claude Desktop too`

---

### Task 9: Desktop 会话的用量与接续

**Files:** Modify `internal/account/env.go`（或 cli 层包装）、`internal/cli/statusline.go`、`internal/cli/stopfailure.go`、`internal/handoff/handoff.go`；Test 扩充

**Interfaces:**
- `CLAUDE_CODE_ENTRYPOINT=claude-desktop` 且 `CLAUDE_CONFIG_DIR` 未设置时：若 Desktop 已被接管，账号 = State.Active，且仅当默认路径 `AccountUUID` 与之相符才算数；否则仍按"未知"处理（计划 2 的行为）
- Desktop 会话撞额度：handoff 的 Request 增加 `Desktop bool`；对话框按钮改为 `Switch Desktop to <B>`；确认后执行 `hotseat desktop switch <B> --resume <session_id>`（复用 Switch；在 handoff 进程内调用）

- [ ] **Step 1: 写失败测试**：`TestStatuslineAttributesDesktopToActive`、`TestStatuslineDesktopUnknownWhenMismatch`、`TestStopFailureDesktopOffersDesktopSwitch`、`TestOfferDesktopSwitchRunsSwitch`（假 Driver 收到 resume URL）。
- [ ] **Step 2–4: 失败 → 实现 → PASS。**
- [ ] **Step 5: Commit**：`feat: usage and limit handoff for Desktop sessions`

---

### Task 10: 本机接管与验收（需要用户在场）

- [ ] **Step 1**：README 增加 Desktop 一节（接管、新增账号、切换、恢复、"不要在 Desktop 里点退出登录"、"不用 symlink 的原因"）。Commit。
- [ ] **Step 2**：用户退出 Desktop 后：`hotseat desktop adopt`（默认路径 = claude）；`hotseat desktop adopt claude3 ~/Developer/_Bench/desktop-spike-2026-10-29/userdata-claude3`（实验时登录过 claude3 的目录）；`hotseat desktop status`。
- [ ] **Step 3**：`hotseat desktop add claude2`，用户在打开的 Desktop 里登录 claude2；`hotseat desktop status` 三个 profile 都一致。
- [ ] **Step 4**：在 claude2 的 Desktop 里打开任一会话，`hotseat use claude3 --yes`：Desktop 退出、重开为 claude3、自动打开同一会话；左侧会话数与账号 1 相同（≈926）；MCP 服务器数一致。用户从 Dock 退出再打开，仍是 claude3。
- [ ] **Step 5**：`hotseat use claude --yes` 切回账号 1；`hotseat desktop status` 正常。把结果写进 ledger。

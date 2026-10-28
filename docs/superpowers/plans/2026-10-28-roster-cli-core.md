# Roster CLI 核心 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用一个 Go 命令行程序 `roster`，把用户手工搭的多账号 CLI 配置自动化：收编现有账号、共享"工作"数据、切换"当前账号"、`claude` 包装命令、`doctor` 自检、新增账号。

**Architecture:** 单个 Go 二进制。以 `argv[0]` 区分两种模式：叫 `roster` 时是管理命令，叫 `claude` 时是包装命令（busybox 方式，通过 `~/.config/roster/bin/claude` 这个 symlink 进入）。所有路径都从 `$HOME` 和 `$ROSTER_HOME` 推出，测试时用临时目录替换。

**Tech Stack:** Go 1.26，只用标准库。

**Spec:** `docs/superpowers/specs/2026-10-28-roster-design.md`。本计划覆盖其中 6.1（CLI 部分）、6.2、6.3（不含 statusLine）、6.8，以及 9 节的验收 1、2（CLI 部分）、6、7、8。

**后续计划（本计划不做）：**
- 计划 2：用量和额度。statusLine 采集、`StopFailure` 通知加一键接续，对应设计 6.3 的 statusLine、6.6、6.7。
- 计划 3：Desktop。先做设计第 8 节的实验，再做 6.4、6.5。
- 计划 4：SwiftUI 菜单栏面板。

## Global Constraints

- 只支持 macOS。Go 1.26，不引入第三方依赖。模块路径 `github.com/zahT3/roster`，公开发布前如果要换 owner，统一改。
- Roster 状态目录：`$ROSTER_HOME`，未设置时用 `$HOME/.config/roster`。核心目录（"工作"真身）：`$HOME/.claude`。默认账号是 `ConfigDir == $HOME/.claude` 的那个，它的 `.claude.json` 在 `$HOME/.claude.json`，其他账号的在 `<ConfigDir>/.claude.json`。
- 凭证红线：不读、不写、不复制 `.credentials.json`、钥匙串、任何 token。`.claude.json` 只允许读 `oauthAccount.accountUuid`、`oauthAccount.organizationUuid` 和 `mcpServers`；写入时只改 `mcpServers`，其他键保持原值。
- 不自动切换账号。所有切换都由用户执行的命令触发。
- 不删除用户数据。要替换的东西一律移进备份目录 `$ROSTER_HOME/backups/<id>/`，回滚时移回。
- 本计划不修改 `settings.json`，也不动已有的 hook 条目。
- 测试一律在临时 HOME 里进行：每个测试用 `t.Setenv("HOME", t.TempDir())` 和 `t.Setenv("ROSTER_HOME", …)`，不得触碰真实的 `~/.claude*`。
- 命令行输出用英文，`--json` 输出供以后的菜单栏 app 使用（设计者决定）。
- commit message 末尾加 `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。

## Review Focus

1. **默认账号绝不能被设置 `CLAUDE_CONFIG_DIR`。** 官方说明钥匙串条目按配置目录区分，给默认账号显式设成 `~/.claude` 可能会读到另一个条目，表现为"被登出"。包装命令对默认账号只能删除这个变量，不能设置它。（Task 3）
2. **包装命令出任何错都不能拦住 claude**：registry 损坏、`current` 文件指向不存在的账号、状态目录不存在，都要原样启动真正的 claude，只在 stderr 打一行警告。（Task 3）
3. **包装命令不能执行到自己。** PATH 里可能有重复项（本机 `which -a claude` 列出 3 次 `~/.local/bin/claude`），shim 目录也可能经由 symlink 出现。找真 claude 时，要跳过所有解析后等于 shim 目录或等于 roster 自身的候选。（Task 3）
4. **有 claude 进程在跑时，`share apply` 必须拒绝**，除非加 `--force`。施工到一半出错也必须能回滚，所以每完成一步就写一次 manifest。（Task 6）
5. **合并 `history.jsonl` 不能丢行**：坏行、没有 `timestamp` 的行、最后一行没有换行符，都要保留下来；合并结果以换行符结尾。（Task 6）

---

## File Structure

```
go.mod
cmd/roster/main.go              入口：argv[0]=="claude" 走 shim.Run，否则走 cli.Run
internal/paths/paths.go         Paths 和所有路径推导
internal/account/account.go     Account、Registry、current、ReadIdentity
internal/account/discover.go    Discover、ParseAliases
internal/shim/shim.go           ResolveEnv、FindRealClaude、Run
internal/share/classify.go      分类表、Classifier、覆盖规则
internal/share/plan.go          MakePlan
internal/share/apply.go         Apply、Rollback、manifest
internal/share/merge.go         mergeJSONL、mergeTree、copyTree
internal/share/mcp.go           MCP 服务器的合并和写回
internal/share/proc.go          RunningClaude
internal/doctor/doctor.go       Check
internal/cli/*.go               每个子命令一个文件，入口 Run(args, stdout, stderr) int
internal/testutil/fixture.go    假 HOME 和假账号目录的构造工具
```

---

### Task 1: 骨架、路径、账号注册表、身份读取

**Files:**
- Create: `go.mod`, `cmd/roster/main.go`, `internal/paths/paths.go`, `internal/account/account.go`, `internal/testutil/fixture.go`, `internal/cli/cli.go`
- Test: `internal/paths/paths_test.go`, `internal/account/account_test.go`

**Interfaces:**
- Produces:
  - `paths.Paths{Home, RosterDir string}`、`paths.FromEnv() (paths.Paths, error)`
  - 方法 `CoreDir()`、`DefaultClaudeJSON()`、`RegistryFile()`（`accounts.json`）、`CurrentFile()`（`current`）、`OverridesFile()`（`share-overrides.json`）、`BackupsDir()`、`ShimDir()`（`bin`）、`Zshrc()`，返回值都是 `string`
  - `account.Account{Name, ConfigDir string; IsDefault bool; AccountUUID, OrgUUID string}`，JSON 键为 `name`、`configDir`、`isDefault`、`accountUuid`、`orgUuid`
  - `account.Registry{Accounts []Account}`、`LoadRegistry(paths.Paths) (Registry, error)`（文件不存在时返回空表）、`SaveRegistry(paths.Paths, Registry) error`（先写临时文件再 rename，权限 0600）
  - `(Registry) Find(name string) (Account, bool)`、`(Registry) Default() (Account, bool)`
  - `account.ClaudeJSONPath(paths.Paths, Account) string`
  - `account.Identity{AccountUUID, OrgUUID string}`、`account.ReadIdentity(claudeJSON string) (Identity, error)`，没有 `oauthAccount` 时返回 `account.ErrNoIdentity`
  - `account.CurrentName(paths.Paths) (string, error)`（文件不存在时返回 `""`）、`account.SetCurrent(paths.Paths, name string) error`
  - `testutil.FakeHome(t) paths.Paths`、`testutil.AddAccountDir(t, p, dir, acctUUID, orgUUID string, mcp map[string]any)`。传入 `dir == p.CoreDir()` 时，`.claude.json` 写到 `p.DefaultClaudeJSON()`。
  - `cli.Run(args []string, stdout, stderr io.Writer) int`，本任务只支持 `version` 子命令

- [ ] **Step 1: 初始化模块。** 运行 `go mod init github.com/zahT3/roster`，把 `go.mod` 的 go 版本设为 `1.26`。
- [ ] **Step 2: 写失败测试**

```go
func TestFromEnvDefaultsRosterDir(t *testing.T) {
    t.Setenv("HOME", "/tmp/h"); t.Setenv("ROSTER_HOME", "")
    p, _ := paths.FromEnv()
    // p.RosterDir == "/tmp/h/.config/roster"; p.CoreDir() == "/tmp/h/.claude"; p.DefaultClaudeJSON() == "/tmp/h/.claude.json"
}
func TestReadIdentityReadsOnlyOauthUUIDs(t *testing.T) {
    // .claude.json 内容: {"oauthAccount":{"accountUuid":"a1","organizationUuid":"o1","emailAddress":"x@y"},"mcpServers":{}}
    // ReadIdentity → Identity{"a1","o1"}, nil
}
func TestReadIdentityMissingOauth(t *testing.T)   // {"numStartups":3} → errors.Is(err, account.ErrNoIdentity)
func TestRegistryRoundTrip(t *testing.T)          // Save 后 Load 得到相同内容；文件权限 0600
func TestClaudeJSONPathDefaultVsOther(t *testing.T) // 默认账号 → $HOME/.claude.json；其他账号 → <dir>/.claude.json
func TestCurrentMissingIsEmpty(t *testing.T)      // CurrentName → "", nil
```

- [ ] **Step 3: 运行 `go test ./...`，预期失败**（包或函数不存在）。
- [ ] **Step 4: 实现以上接口。** `main.go` 根据 `filepath.Base(os.Args[0])` 分流，本任务里 shim 分支先留成 `os.Exit(cli.Run(...))` 的占位，Task 3 再接上。`roster version` 打印 `roster 0.1.0`。
- [ ] **Step 5: 运行 `go test ./...`，预期 PASS。**
- [ ] **Step 6: Commit**：`feat: scaffold roster with paths, registry and identity`

---

### Task 2: 发现现有账号，以及 `init`、`accounts`、`use`、`current` 命令

**Files:**
- Create: `internal/account/discover.go`, `internal/cli/init.go`, `internal/cli/accounts.go`, `internal/cli/use.go`
- Test: `internal/account/discover_test.go`, `internal/cli/init_test.go`

**Interfaces:**
- Consumes: Task 1 的全部接口
- Produces:
  - `account.ParseAliases(zshrc string) map[string]string`：把**绝对目录**映射到别名。能识别 `alias NAME='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'`，路径可以带 `$HOME`、`${HOME}`、`~`，可以加引号也可以不加。
  - `account.Discover(paths.Paths) ([]Account, error)`：候选目录是 `$HOME/.claude`，加上 `$HOME` 下所有名字以 `.claude` 开头的目录。只收录 `ReadIdentity` 成功的目录。命名规则：默认账号叫 `claude`；有别名的用别名；其余用去掉开头点号的目录名（`.claude2` → `claude2`）。排序：默认账号在前，其余按名字排。

- [ ] **Step 1: 写失败测试**

```go
func TestParseAliases(t *testing.T) {
    // 输入两行: alias claude2='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'
    //           alias work=CLAUDE_CONFIG_DIR=~/.claude-work\ claude   ← 不要求支持转义空格，这一行可以不识别，但不能 panic
    // 期望 map[home+"/.claude2"] == "claude2"
}
func TestDiscoverSkipsDirsWithoutIdentity(t *testing.T) {
    // 构造: .claude（a1）、.claude2（b2，并有别名 claude2）、.claude-deepseek（.claude.json 里没有 oauthAccount）
    // 期望得到 [claude(default,a1), claude2(b2)]，不包含 deepseek
}
func TestInitIsIdempotentAndSetsCurrent(t *testing.T) {
    // cli.Run(["init"]) 两次 → registry 里 2 个账号，不重复；CurrentName == "claude"
    // 第二次 init 前，用户已执行 `use claude2` → current 仍是 claude2
}
func TestUseUnknownAccountFails(t *testing.T) // `use nope` 退出码 1，stderr 包含 `unknown account "nope"`
```

- [ ] **Step 2: 运行 `go test ./...`，预期失败。**
- [ ] **Step 3: 实现。** `init`：调用 Discover，和已有 registry 合并（同一个 `AccountUUID` 保留原来的名字）后保存；`current` 为空时设为默认账号。`accounts`：每行格式 `* claude   ~/.claude   <acct-1>`，当前账号前面加 `*`，`--json` 输出 `[]Account` 和 `current`。`use <name>`、`current`。
- [ ] **Step 4: 运行 `go test ./...`，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: discover existing claude accounts and track current account`

---

### Task 3: `claude` 包装命令（shim）

**Files:**
- Create: `internal/shim/shim.go`
- Modify: `cmd/roster/main.go`（shim 分支调用 `shim.Run(os.Args)`）
- Test: `internal/shim/shim_test.go`

**Interfaces:**
- Consumes: `account.LoadRegistry`、`account.CurrentName`、`paths.FromEnv`
- Produces:
  - `shim.ResolveEnv(env []string, reg account.Registry, current string) (out []string, warning string)`。规则按顺序判断：`CLAUDE_CONFIG_DIR` 已经非空 → 原样返回；`current == ""` → 原样返回；`current` 不在注册表里 → 原样返回，并给出警告 `roster: current account "X" not found; starting claude unchanged`；是默认账号 → 删除 `CLAUDE_CONFIG_DIR`；否则 → 设置 `CLAUDE_CONFIG_DIR=<ConfigDir>`。
  - `shim.FindRealClaude(pathEnv, self, shimDir string) (string, error)`：按 PATH 顺序找名为 `claude` 的可执行普通文件。目录经 `EvalSymlinks` 后等于 shimDir 的跳过；候选经 `EvalSymlinks` 后等于 self 的跳过。
  - `shim.Run(args []string)`：加载路径、注册表和当前账号，任何一步出错都按"原样启动"处理，把警告写到 stderr；然后用 `syscall.Exec(real, args, env)` 启动，`args[0]` 保持 `claude`。找不到真 claude 时打印 `roster: real claude not found in PATH` 并以 127 退出。

- [ ] **Step 1: 写失败测试**

```go
func TestResolveEnvDefaultAccountUnsets(t *testing.T)   // env 里有 CLAUDE_CONFIG_DIR=""（空串），current=默认账号 → 输出里没有 CLAUDE_CONFIG_DIR
func TestResolveEnvSetsNonDefault(t *testing.T)         // current=claude2 → 输出包含 "CLAUDE_CONFIG_DIR=<home>/.claude2"，且只出现一次
func TestResolveEnvRespectsExplicitDir(t *testing.T)    // env 已有 CLAUDE_CONFIG_DIR=/x、current=claude2 → 原样返回，warning 为空
func TestResolveEnvUnknownCurrentWarns(t *testing.T)    // current="gone" → 原样返回，warning 包含 `"gone" not found`
func TestFindRealClaudeSkipsShimAndSelf(t *testing.T) {
    // PATH = shimDir : linkToShimDir（指向 shimDir 的 symlink 目录） : realDir : realDir
    // shimDir/claude → self；realDir/claude 是可执行脚本 → 返回 realDir/claude
}
func TestRunFallsBackOnCorruptRegistry(t *testing.T) {
    // 把 accounts.json 写成 "{"；用测试 hook 把 exec 函数替换掉，捕获它收到的 env
    // → 传给 exec 的 env 与原始 env 相同；stderr 有一行以 "roster:" 开头
}
```

为了能测 `Run`，在包里留一个变量 `var execFn = syscall.Exec`，测试中替换它。

- [ ] **Step 2: 运行 `go test ./internal/shim/`，预期失败。**
- [ ] **Step 3: 实现以上三个函数，并接上 `main.go` 的 shim 分支。**
- [ ] **Step 4: 运行 `go test ./...`，预期 PASS。**
- [ ] **Step 5: 端到端检查**：`go build -o /tmp/rt/roster ./cmd/roster && ln -sf /tmp/rt/roster /tmp/rt/claude`，再用一个临时 HOME，运行 `HOME=$tmp ROSTER_HOME=$tmp/r /tmp/rt/claude --version`。预期输出真 claude 的版本号 `2.1.310 (Claude Code)`；因为没有注册表，不应有警告。
- [ ] **Step 6: Commit**：`feat: add claude shim that follows the current account`

---

### Task 4: 分类表

**Files:**
- Create: `internal/share/classify.go`, `internal/cli/share.go`（本任务只做 `share set`）
- Test: `internal/share/classify_test.go`

**Interfaces:**
- Produces:
  - `share.Class` 类型，取值 `share.Shared`（"shared"）、`share.Local`（"local"）、`share.Pending`（"pending"）、`share.Unknown`（"unknown"）
  - `share.SharedItems() []string`：内置的共享清单，排序后返回
  - `share.LoadClassifier(paths.Paths) (Classifier, error)`、`(Classifier) Classify(name string) Class`：用户覆盖规则优先于内置表
  - `share.SetOverride(paths.Paths, name string, c Class) error`：只接受 Shared 或 Local，其他值返回错误

内置表（照抄设计 6.2，再补上本机出现过的条目）：
- Shared：`CLAUDE.md settings.json projects history.jsonl file-history tasks plans paste-cache skills commands agents output-styles rules workflows agent-memory keybindings.json themes`
- Local：`.credentials.json .claude.json .claude.json.backup stats-cache.json remote-settings.json policy-limits.json backups shell-snapshots session-env statsig cache debug image-cache todos logs ide chrome mcp-needs-auth-cache.json .last-cleanup .last-update-result.json .DS_Store usage-data feedback feedback-bundles settings.local.json`
- Pending：`plugins sessions jobs daemon uploads`
- 不在表里的都是 Unknown。

- [ ] **Step 1: 写失败测试**：`TestClassifyBuiltins`（`projects`→Shared，`.credentials.json`→Local，`plugins`→Pending，`scheduled-tasks`→Unknown）；`TestOverrideWins`（`share set scheduled-tasks shared` 后 Classify 得到 Shared）；`TestOverrideRejectsPending`。
- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现。** 覆盖规则存在 `share-overrides.json`，格式 `{"scheduled-tasks":"shared"}`。CLI 用法：`roster share set <item> shared|local`。
- [ ] **Step 4: 运行，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: add work/account classification table with overrides`

---

### Task 5: 共享计划（只读）

**Files:**
- Create: `internal/share/plan.go`, `internal/share/mcp.go`（本任务只做比较）
- Modify: `internal/cli/share.go`（增加 `share plan [--json]`）
- Test: `internal/share/plan_test.go`

**Interfaces:**
- Consumes: Task 1、Task 4
- Produces:
  - `share.ActionKind` 类型，取值 `Link`、`ReplaceWithLink`、`MergeJSONL`、`MergeDir`、`Adopt`、`Conflict`、`SyncMCP`，JSON 中为 kebab-case 字符串
  - `share.Action{Kind ActionKind; Account, Item, Path, Target, Detail string}`
  - `share.Plan{Actions []Action}`、`share.MakePlan(paths.Paths, account.Registry, Classifier) (Plan, error)`
  - `share.mergedMCP(files []string, defaultFile string) (merged map[string]json.RawMessage, conflicts []string, err error)`：从默认账号的 `mcpServers` 出发，其他账号有而它没有的服务器补进来。同名但内容不同（比较紧凑化后的 JSON）时，保留默认账号的版本，把服务器名记进 conflicts。

规则：对每个**非默认**账号，逐个检查 `SharedItems()` 加上被覆盖为 Shared 的条目。下面 core 指 `<CoreDir>/<item>`，acct 指 `<ConfigDir>/<item>`。

| core | acct | 动作 |
|---|---|---|
| 不存在 | 不存在 | 无 |
| 不存在 | 真实文件或目录 | `Adopt` |
| 不存在 | symlink | `Conflict` |
| 存在 | 不存在 | `Link` |
| 存在 | symlink，`EvalSymlinks` 后等于 core 解析后的路径 | 无 |
| 存在 | symlink，指向别处或已断开 | `Conflict` |
| 文件 | 文件，`item == "history.jsonl"` | `MergeJSONL` |
| 文件 | 文件，内容完全相同 | `ReplaceWithLink` |
| 文件 | 文件，内容不同 | `Conflict` |
| 目录 | 目录 | `MergeDir` |
| 一边是文件、一边是目录 | | `Conflict` |

所有账号的 `mcpServers` 合并结果如果和任何一个账号现有的不同，就额外生成一个 `SyncMCP` 动作，`Detail` 写明涉及的服务器名和冲突。

- [ ] **Step 1: 写失败测试**：表格里每一行一个用例，名字形如 `TestPlanCoreMissingAcctReal_Adopt`。另外加 `TestPlanTreatsEquivalentSymlinkAsLinked`：acct 是一个 symlink，它指向一个本身又指向 core 的 symlink，期望不产生动作。`TestPlanSkipsDefaultAccount`。`TestMergedMCPConflictKeepsDefault`：默认账号有 `{"a":{"command":"x"}}`，另一个账号有 `{"a":{"command":"y"},"b":{}}`，期望 merged 的键是 a、b，`a` 的值是 x，conflicts 为 `["a"]`。
- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现。** `share plan` 输出每个动作一行，形如 `claude2  history.jsonl  merge-jsonl`，末尾附一句 `N conflicts will be skipped`。
- [ ] **Step 4: 运行，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: compute share plan for account dirs`

---

### Task 6: 执行共享计划，备份和回滚

**Files:**
- Create: `internal/share/apply.go`, `internal/share/merge.go`, `internal/share/proc.go`
- Modify: `internal/share/mcp.go`（写回）、`internal/cli/share.go`（增加 `apply [--force] [--json]`、`rollback <id>`、`backups`）
- Test: `internal/share/apply_test.go`, `internal/share/merge_test.go`

**Interfaces:**
- Consumes: Task 5 的 `Plan`、`Action`
- Produces:
  - `share.ApplyOptions{Force bool; Running func() ([]int, error)}`，`Running` 为 nil 时用 `RunningClaude`
  - `share.RunningClaude() ([]int, error)`：执行 `pgrep -x claude`。pgrep 退出码为 1 表示没有匹配进程，返回空切片和 nil
  - `share.ErrClaudeRunning`
  - `share.Apply(paths.Paths, account.Registry, Plan, ApplyOptions) (Manifest, error)`
  - `share.Manifest{ID string; Steps []Step}`，ID 格式 `20061028-150405`
  - `share.Step{Action Action; BackupPath, CoreBackupPath string; AddedToCore, Conflicts []string; Created string; Adopted bool}`
  - `share.Rollback(paths.Paths, id string) error`、`share.ListBackups(paths.Paths) ([]string, error)`

执行规则：
- 开始前检查：`!Force` 且有 claude 进程在跑 → 返回 `ErrClaudeRunning`。CLI 打印进程号，提示关掉所有 claude 会话后再试，或者加 `--force`，退出码 2。
- `Conflict` 动作一律跳过。每完成一步，就把 manifest 写到 `backups/<id>/manifest.json`。
- `Link`：创建绝对路径的 symlink `acct → core`。
- `ReplaceWithLink`：把 acct 移进 `backups/<id>/<account>/<item>`，然后创建 symlink。
- `MergeJSONL`：先把 core 原件复制到 `backups/<id>/core/<item>`，把合并结果先写临时文件再 rename 覆盖 core；然后把 acct 移进备份，创建 symlink。
- `MergeDir`：`mergeTree(acct, core)` 只往 core 里复制 core 没有的条目，复制的路径记进 AddedToCore；两边都是文件但内容不同时保留 core 的版本，把路径记进 Conflicts。然后把 acct 移进备份，创建 symlink。
- `Adopt`：把 acct 直接 `os.Rename` 到 core 的位置，再创建 symlink，标记 `Adopted`。
- `SyncMCP`：对每个 `.claude.json` 先复制一份到备份，再用 `map[string]json.RawMessage` 读出、只替换 `mcpServers` 后写回，缩进两个空格。
- 回滚：按倒序处理每一步。只删除自己创建的 symlink（先确认它确实是 symlink）；把备份里的东西移回原位；AddedToCore 里的路径移进 `backups/<id>/rollback-removed/`，不直接删除；`Adopted` 的步骤把 core 移回 acct。

`mergeJSONL(core, acct []byte) []byte`：先 core 后 acct，把两边的行合在一起；每行去掉结尾的 `\r`，空行丢弃；按去掉首尾空白后的内容去重，保留第一次出现的那行；按解析出来的 `timestamp`（整数）做稳定排序，解析失败或没有 `timestamp` 的行当作 0；结果以 `\n` 结尾。

- [ ] **Step 1: 写失败测试**

```go
func TestMergeJSONLKeepsMalformedLines(t *testing.T) {
    // core: `{"display":"a","timestamp":2}` + "\n" + `not json`        （没有结尾换行）
    // acct: `{"display":"b","timestamp":1}\r\n{"display":"a","timestamp":2}\n{"display":"c"}\n`
    // 结果按顺序是: not json / {"display":"c"} / b / a（一行）；共 4 行，以 \n 结尾
}
func TestApplyRefusesWhenClaudeRunning(t *testing.T) // Running 返回 [123] → errors.Is(err, ErrClaudeRunning)，文件系统没有任何变化
func TestApplyThenRollbackRestoresBytes(t *testing.T) {
    // 构造：core 和 acct 各有 history.jsonl、skills/x/SKILL.md（两边内容不同）、file-history/s1/；acct 里没有 tasks；两边的 mcpServers 不同
    // Apply 后：acct 下各条目都是指向 core 的 symlink；core 的 history 含两边所有行；core 的 skills/x/SKILL.md 保持 core 的版本，并记为冲突
    // Rollback 后：acct 和 core 下每个文件的字节内容、两个 .claude.json，都和 Apply 前快照一致
}
func TestRollbackAfterPartialFailure(t *testing.T) // 在第 2 个动作前让 core 目录变成只读，制造失败 → Apply 返回错误；manifest 里只有第 1 步；Rollback 后与快照一致
func TestSyncMCPPreservesOtherKeys(t *testing.T)   // 写回后，把两个文件解析成 map[string]any 比较，除 mcpServers 外的键和值不变
```

- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现 `merge.go`**（`mergeJSONL`、`mergeTree`、`copyTree`。`copyTree` 保留文件权限，symlink 按 symlink 复制，不跟随）。
- [ ] **Step 4: 实现 `apply.go`、`proc.go`、`mcp.go` 的写回部分，以及三个 CLI 子命令。**
- [ ] **Step 5: 运行 `go test ./...`，预期 PASS。**
- [ ] **Step 6: Commit**：`feat: apply share plan with backups and rollback`

---

### Task 7: doctor

**Files:**
- Create: `internal/doctor/doctor.go`, `internal/cli/doctor.go`
- Test: `internal/doctor/doctor_test.go`

**Interfaces:**
- Consumes: `share.MakePlan`、`share.LoadClassifier`、`account.ReadIdentity`。判断 `shim-inactive` 时，在 env 的 PATH 里找第一个 `claude`，经 `EvalSymlinks` 解析后和 `<ShimDir>/claude` 解析后的路径比较。
- Produces:
  - `doctor.Finding{Level string /* "error" | "warn" | "info" */; Code, Account, Path, Message string}`
  - `doctor.Check(p paths.Paths, reg account.Registry, env []string) ([]doctor.Finding, error)`

各检查项的 Code 和级别：

| 检查 | Code | 级别 |
|---|---|---|
| 账号目录不存在 | `account-dir-missing` | error |
| `.claude.json` 的身份和注册表不一致 | `identity-mismatch` | error |
| 没有身份（未登录） | `identity-missing` | warn |
| 计划里有 `Link` | `shared-missing` | warn |
| 计划里有 `ReplaceWithLink`、`MergeJSONL`、`MergeDir`、`Adopt` | `shared-diverged` | error |
| 计划里有 `Conflict` | `shared-conflict` | error |
| 计划里有 `SyncMCP` | `mcp-diverged` | warn |
| 账号目录里有 Unknown 条目 | `unknown-entry` | warn，Message 提示 `roster share set <item> shared|local` |
| 账号目录里有 Pending 条目 | `pending-entry` | info |
| env 或核心 `settings.json` 的 `env` 块里有 `CLAUDE_CODE_OAUTH_TOKEN` | `oauth-token-env` | warn，只报告存在与否，**不输出变量的值** |
| PATH 上的 `claude` 解析后不是 shim | `shim-inactive` | info |

- [ ] **Step 1: 写失败测试**：每个 Code 至少一个用例，名字形如 `TestDoctorIdentityMismatch`。另外 `TestDoctorOAuthTokenValueNotPrinted` 断言 `"sk-ant-oat01-SECRET"` 不出现在任何 Finding 里。还有 `TestDoctorCleanSetupHasNoErrors`。
- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现。** CLI 输出每条一行，格式 `[error] claude2 shared-diverged history.jsonl: ...`；`--json` 输出 Finding 数组；只要有 error 级别的条目，退出码就是 1。
- [ ] **Step 4: 运行，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: add doctor checks`

---

### Task 8: `add` 新增账号

**Files:**
- Create: `internal/cli/add.go`
- Test: `internal/cli/add_test.go`

**Interfaces:**
- Consumes: `shim.FindRealClaude`、`account.ReadIdentity`、`share.MakePlan`、`share.Apply`
- Produces: 命令 `roster add <name> [--dir <path>]`，默认目录是 `$HOME/.claude-<name>`

流程：
1. name 已存在，或目录存在且非空 → 退出码 1。
2. 创建目录，权限 0700。
3. 用真 claude 运行 `auth login`，环境变量加上 `CLAUDE_CONFIG_DIR=<dir>`，stdin、stdout、stderr 直接继承。
4. 用 `ReadIdentity` 读身份。读不到 → 输出 `login did not complete`，退出码 1，目录保留。身份和已有账号相同 → 输出 `this Claude account is already registered as "X"`，退出码 1。
5. 写入注册表。
6. 只对这个账号执行 `Link` 动作，调用 `Apply` 时设 `Force: true`，因为只是在新目录里建 symlink，不碰任何已有文件，不需要等其他 claude 会话关闭。新目录只会产生 `Link`，如果出现其他动作，说明有异常，应报错而不是执行。
7. 提示用户：MCP 服务器要等运行 `roster share apply` 后才同步。

- [ ] **Step 1: 写失败测试。** 用一个假的 claude 脚本放在 PATH 最前面。脚本收到 `auth login` 时，往 `$CLAUDE_CONFIG_DIR/.claude.json` 写 `{"oauthAccount":{"accountUuid":"n1","organizationUuid":"o9"}}`。用例：`TestAddRegistersAndLinks`（注册表多了 `n1`，`projects`、`history.jsonl` 都是指向 core 的 symlink）、`TestAddDuplicateAccount`、`TestAddLoginNotCompleted`（假脚本什么都不写）。
- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现。**
- [ ] **Step 4: 运行，预期 PASS。**
- [ ] **Step 5: Commit**：`feat: add account via official claude login`

---

### Task 9: 安装 shim，在本机验收

**Files:**
- Create: `internal/cli/shim.go`, `README.md`
- Test: `internal/cli/shim_test.go`

**Interfaces:**
- Produces: 命令 `roster shim install` 和 `roster shim uninstall`

`install` 做三件事：
- 创建 symlink `<ShimDir>/claude`，指向 `os.Executable()` 解析后的路径。
- 把 `~/.zshrc` 备份到 `backups/zshrc-<id>`。
- 在文件**末尾**写入下面这段；已有同名段落时就地替换，不重复追加：

```
# >>> roster >>>
export PATH="<ShimDir>:$PATH"
# <<< roster <<<
```

`uninstall` 删除这段和那个 symlink。

- [ ] **Step 1: 写失败测试**：`TestShimInstallIdempotent`（执行两次后，zshrc 里只有一段 roster 段落，原有内容不变）、`TestShimUninstallRestores`。
- [ ] **Step 2: 运行，预期失败。**
- [ ] **Step 3: 实现。** README 写安装、`init`、`share plan/apply/rollback`、`use`、`doctor`、`add`，以及凭证红线（不读 token）的说明。
- [ ] **Step 4: 运行 `go test ./...`，预期全部 PASS。**
- [ ] **Step 5: Commit**：`feat: install claude shim into zsh PATH`
- [ ] **Step 6: 本机只读验收。** 运行 `go build -o ~/.local/bin/roster ./cmd/roster`，然后依次运行 `roster init`、`roster accounts`、`roster share plan`、`roster doctor`。预期：
  - 识别出 `claude`、`claude2`、`claude3` 三个账号，不包含 `.claude-deepseek`；
  - claude2 和 claude3 各有一条 `history.jsonl merge-jsonl`、`skills merge-dir` 和 `file-history merge-dir`；各账号的 MCP 服务器不一致时，还会有一条 `sync-mcp`；
  - `projects`、`settings.json`、`CLAUDE.md` 没有动作，因为它们已经共享。

  把这四条命令的输出贴给用户。
- [ ] **Step 7: 停下来，等用户确认后再动真实数据。** 请用户关闭所有 claude 会话（包括执行这份计划的会话），在普通终端里运行 `roster share apply`、`roster shim install`，然后新开一个终端，逐项验证：
  - `claude auth status` 显示当前账号；
  - `roster use claude2` 之后新开终端，`claude auth status` 的 orgId 变成 claude2 的；
  - `claude2 --resume` 能看到所有会话，上箭头能调出原来 claude2 的历史；
  - `roster doctor` 没有 error。

  如果有问题，运行 `roster share rollback <id>`，并确认能恢复原样。

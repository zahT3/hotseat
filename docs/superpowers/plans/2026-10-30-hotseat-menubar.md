# Hotseat Menu Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 菜单栏常驻一个 Hotseat 图标，点开看到每个账号的 5 小时 / 7 天用量、重置时间、数据新旧、当前账号和 Desktop 状态；点一个账号就完成切换（CLI 和 Desktop 一起切）。

**Architecture:** 菜单栏 app 只负责显示和点击，不碰任何账号数据。数据由新加的 `hotseat panel --json` 一次给齐，切换调用已有的 `hotseat use <账号> --yes --json`。Swift 端分成可测的 `HotseatKit`（JSON 模型、文字格式、调用 CLI）和不做单测的 `Hotseat` app（AppKit `NSStatusItem` + `NSMenu`，每个账号一行，行是 `NSHostingView` 包的 SwiftUI 视图）。

**Tech Stack:** Go 1.26 标准库；Swift 6.4 + SwiftPM，只用 Command Line Tools（不需要 Xcode）；AppKit、SwiftUI、ServiceManagement；Swift Testing。

**Spec:** `docs/superpowers/specs/2026-10-28-roster-design.md` 6.7、6.9、验收第 3、4 条（项目已改名 Hotseat）。与 spec 的差异：
- 不显示套餐类型。本机三个账号里只有一个的 `.claude.json` 有 `organizationType`，其余没有合规的本地来源（已验证：2026-10-30 只查了键名和这几个字段的值）。
- 技术栈用 AppKit `NSStatusItem` 承载 SwiftUI 行，不用纯 `MenuBarExtra`（用户 2026-10-30 选的 A 方案）。

## 已验证的事实（2026-10-30 本机）

- 只装 CLT 时，`swift build -c release` 能编出带 SwiftUI 行的状态栏 app，耗时 61 s；手工拼 `.app`（`Info.plist` 设 `LSUIElement`）加 `codesign --force -s -` 后能启动。
- 只装 CLT 时 `swift test` 能跑 Swift Testing（`import Testing`）。第一次编译报错，原因没查；重跑后通过。执行时如果复现，先 `swift package clean` 再试。
- `hotseat use <账号> --yes --json` 在 Desktop 运行时会退出 Desktop 并在新账号上重开刚才的会话；Desktop 没运行时只交换目录。退出码：0 成功，1 失败（stderr 是原因）。
- GUI app 拿不到 shell 的 PATH，所以要用绝对路径找 `hotseat`。

## Global Constraints

- 沿用计划 1–3 的全部约束：macOS 14+；Go 只用标准库；测试只在临时 HOME 里跑；不删用户数据；不读写任何凭证；commit 带 trailer。
- app 不读任何 Claude 的文件，所有数据只来自 `hotseat panel --json` 和 `hotseat use … --json`。
- app 不加任何网络请求、遥测或第三方 Swift 包。
- 界面文字用英文，和 CLI 一致。
- Bundle ID 是 `io.github.zahT3.hotseat`，app 名是 `Hotseat`，版本取 `hotseat version` 的输出（现在是 `0.1.0`）。
- 主线程上不跑子进程；同一时间只允许一次切换。

## Review Focus

1. **找不到或运行不了 `hotseat`**（没装、路径改了、没有执行权限）：菜单显示一行错误，写明试过的路径。不能出现空白菜单，也不能崩溃。（Task 3 的测试）
2. **`hotseat` 卡住**（比如 Desktop 退不掉）：界面不冻。`panel` 超过 30 s、`use` 超过 120 s 就杀掉进程并显示超时；切换进行中再点账号不会发出第二次切换。（Task 3、4）
3. **数据缺失或过期**：没有数据的账号显示 `no data yet`；已过重置时间的窗口显示 `0% (reset)`；每行都显示数据时间，不把旧数据当成实时的。（Task 2）
4. **CLI 的 JSON 变了**：多出来的字段要忽略，可选字段为 null 或缺失也能解码；解码失败时显示错误和输出的前 200 个字符，不崩溃。（Task 2）
5. **Desktop 的异常状态**：profile 没登录或登错账号时，那一行要显示 `problem`；有中断的切换时，菜单提示运行 `hotseat desktop recover`；Desktop 正在运行时切换要先确认。（Task 1、4）

---

## File Structure

```
internal/cli/panel.go                 cmdPanel：一次输出菜单栏需要的全部数据
internal/cli/panel_test.go
internal/cli/cli.go                   注册 "panel"
macos/Package.swift                   HotseatKit（library）、Hotseat（executable）、HotseatKitTests
macos/Sources/HotseatKit/Snapshot.swift    panel JSON 的 Decodable 模型
macos/Sources/HotseatKit/Format.swift      百分比、重置时间、数据时间、状态栏标题
macos/Sources/HotseatKit/CLI.swift         找 hotseat、异步运行、超时、错误
macos/Sources/Hotseat/main.swift           启动 NSApplication（accessory）
macos/Sources/Hotseat/AppDelegate.swift    状态栏、菜单、刷新、切换、登录启动
macos/Sources/Hotseat/AccountRow.swift     SwiftUI 账号行
macos/Tests/HotseatKitTests/*.swift
macos/build-app.sh                    编译、拼 .app、ad-hoc 签名，可选安装到 ~/Applications
.gitignore                            macos/.build/、macos/build/
README.md                             菜单栏一节
```

---

### Task 1: `hotseat panel --json`

**Files:**
- Create: `internal/cli/panel.go`, `internal/cli/panel_test.go`
- Modify: `internal/cli/cli.go`（命令表、usage 文字）

**Interfaces:**
- Consumes: `usageRows(p, now)`、`loadRecords(p, reg)`、`usage.Recommend(reg, records, exclude, now)`、`desktopStatus(p)`、`desktop.LoadState(p)`、`desktopDriver.Procs()`、`desktop.Running(procs, home)`、`p.DesktopJournal()`。
- Produces: 下面这份 JSON，`--json` 时输出；不带 `--json` 时输出一行英文提示 `use --json; this command feeds the menu bar app`。只要 registry 能读，退出码就是 0，账号和 Desktop 的问题都写在数据里。

```json
{
  "generatedAt": 1793328100,
  "current": "claude3",
  "recommend": "claude2",
  "desktop": {"managed": true, "active": "claude3", "running": false, "runningKnown": true, "pendingRecovery": false},
  "accounts": [
    {"account": "claude2", "current": false, "fiveHour": {...}, "sevenDay": {...},
     "limit": null, "limitActive": false, "error": "",
     "desktop": "parked", "desktopProblem": "not logged in yet"}
  ]
}
```

`accounts[]` 用的是现有的 `usageRow`（嵌入），再加两个字段。`desktop` 的取值：`active`、`parked`（在 `desktopStatus` 里但不是 active）、`none`（不在里面）；`desktopProblem` 原样取 `profileStatus.Problem`。`recommend` 是 `usage.Recommend(reg, records, current, now)` 的结果，没有可推荐的账号时省略。`Procs()` 出错时 `running:false, runningKnown:false`。账号顺序和 registry 一致。

- [ ] **Step 1: 写失败的测试**（复用 `init_test.go` 的 `run` 和 `desktop_test.go` 的 `fakeDesktop`，沿用那些测试搭临时 HOME 的方式）
  - `TestPanelJSONShape`：三个账号，其中 claude2 有 statusline 记录。断言 `current`、`accounts` 的顺序、claude2 的 `fiveHour.usedPercent`、`desktop.managed == false`，没 adopt 时每个账号都是 `"desktop":"none"`。
  - `TestPanelDesktopStates`：adopt 后 add 一个空 profile。断言 active 账号是 `"active"`；空 profile 那个是 `"parked"`，且 `desktopProblem == "not logged in yet"`；`fakeDesktop` 报有进程在跑时 `running == true`。
  - `TestPanelPendingRecovery`：手写一个 journal 文件，断言 `pendingRecovery == true`，退出码为 0。
  - `TestPanelRecommendSkipsCurrentAndLimited`：current 是 claude，claude2 有生效的 limit，断言 `recommend == "claude3"`。
- [ ] **Step 2:** `go test ./internal/cli -run TestPanel` → FAIL（unknown command）
- [ ] **Step 3: 实现 `cmdPanel(p paths.Paths, args []string, stdout, stderr io.Writer) int`**，在 `cli.go` 注册
- [ ] **Step 4:** `go test ./... ` → 全部 PASS；`go vet ./...` 无输出
- [ ] **Step 5:** `go build -o ~/.local/bin/hotseat ./cmd/hotseat && hotseat panel --json` 在真机上能输出，人工核对和 `hotseat usage` 一致
- [ ] **Step 6: Commit** `feat: panel snapshot for the menu bar app`

---

### Task 2: Swift 包、JSON 模型和格式化

**Files:**
- Create: `macos/Package.swift`, `macos/Sources/HotseatKit/Snapshot.swift`, `macos/Sources/HotseatKit/Format.swift`, `macos/Tests/HotseatKitTests/SnapshotTests.swift`, `macos/Tests/HotseatKitTests/FormatTests.swift`, `.gitignore`
- 为了让 Task 2 能编译，先放一个最小的 `macos/Sources/Hotseat/main.swift`（只有 `import AppKit`），Task 4 再替换

**Interfaces:**
- Produces（全部 `public`，`Sendable`）：
  - `struct Snapshot: Decodable { generatedAt: Int64; current: String; recommend: String?; desktop: DesktopInfo; accounts: [AccountInfo] }`
  - `struct DesktopInfo: Decodable { managed, running, runningKnown, pendingRecovery: Bool; active: String? }`
  - `struct AccountInfo: Decodable, Identifiable { account: String (id); current: Bool; fiveHour, sevenDay: UsageWindow?; limit: LimitHit?; limitActive: Bool; error: String?; desktop: String; desktopProblem: String? }`
  - `struct UsageWindow: Decodable { usedPercent: Double; resetsAt: Int64; observedAt: Int64; source: String; reset: Bool }`
  - `struct LimitHit: Decodable { message: String; resetsAt: Int64; observedAt: Int64 }`（其余字段忽略）
  - `static func Snapshot.decode(_ data: Data) throws -> Snapshot`。失败时抛 `HotseatError.badOutput(String)`，参数是输出的前 200 个字符。
  - `enum Format`（所有函数都带 `now: Date`，并带 `calendar: Calendar = .current`，方便测试）：
    - `window(_ label: String, _ w: UsageWindow?, now:) -> String`：`"5h ?"`、`"5h 0% (reset)"`、`"5h 20% · resets 14:30"`
    - `when(_ unix: Int64, now:) -> String`：规则照抄 Go 的 `fmtWhen`。同一天 `HH:mm`；6 天以内 `EEE HH:mm`；再往后 `MMM d HH:mm`。用 `en_US_POSIX` locale。
    - `age(_ observedAt: Int64?, now:) -> String`：`nil` 或 0 → `"no data yet"`；不到 1 分钟 → `"updated just now"`；不到 60 分钟 → `"updated N min ago"`；不到 48 小时 → `"updated N h ago"`；再往后 → `"updated N d ago"`。一行取 5h 和 7d 两个窗口里较新的 `observedAt`。
    - `title(_ s: Snapshot?, failed: Bool) -> String`：状态栏文字。`failed` 时是 `"Hotseat !"`；当前账号的 limit 生效时是 `"claude3 LIMIT"`；有 5h 数据时是 `"claude3 20%"`；否则只显示账号名。

- [ ] **Step 1: 写失败的测试**
  - `SnapshotTests.decodesRealShape`：用 Task 1 里那份 JSON 做 fixture，额外加一个未知字段 `"futureField": 1`，再让一个账号的 `fiveHour` 为 `null`。断言能解码，取值正确。
  - `SnapshotTests.badOutputKeepsPrefix`：输入 300 个 `x`，断言抛出 `.badOutput`，参数长度是 200。
  - `FormatTests`：固定 `now = 2026-10-30 10:00 Asia/Shanghai`，日历用 `Asia/Shanghai`。逐条断言上面列出的每个输出字符串，包括 `when` 的三档，以及 `age` 在 59 秒、59 分钟、47 小时、3 天这几个边界上的结果。
- [ ] **Step 2:** `cd macos && swift test` → FAIL（类型不存在）
- [ ] **Step 3: 实现** `Snapshot.swift`、`Format.swift`
- [ ] **Step 4:** `swift test` → PASS
- [ ] **Step 5: Commit** `feat(macos): HotseatKit snapshot model and formatting`

---

### Task 3: 调用 CLI

**Files:**
- Create: `macos/Sources/HotseatKit/CLI.swift`, `macos/Tests/HotseatKitTests/CLITests.swift`

**Interfaces:**
- Produces:
  - `public enum HotseatError: Error, Equatable { case notFound(tried: [String]); case failed(code: Int32, stderr: String); case timedOut(seconds: Int); case badOutput(String) }`，另有 `public var message: String`，给菜单直接显示。
  - `public struct HotseatCLI: Sendable`
    - `init(executable: URL)`
    - `static func locate(override: String?, home: URL, isExecutable: (String) -> Bool = FileManager.default.isExecutableFile) throws -> HotseatCLI`：按顺序试 `override`（来自 UserDefaults 的 `hotseatPath`）、`~/.local/bin/hotseat`、`/opt/homebrew/bin/hotseat`、`/usr/local/bin/hotseat`。都不行就抛 `.notFound`，列出试过的全部路径。
    - `func run(_ args: [String], timeout: Duration) async throws -> Data`：在后台跑 `Process`，同时读完 stdout 和 stderr，避免管道写满卡死；超时先 `terminate()`，再抛 `.timedOut`；退出码不是 0 就抛 `.failed`，带上去掉首尾空白的 stderr。
    - `func snapshot() async throws -> Snapshot`：`run(["panel","--json"], timeout: .seconds(30))`
    - `func use(_ account: String) async throws`：`run(["use", account, "--yes", "--json"], timeout: .seconds(120))`

- [ ] **Step 1: 写失败的测试**。测试在临时目录里写可执行的 shell 脚本来冒充 `hotseat`。
  - `locateOrder`：`isExecutable` 只对第三个路径返回 true，断言选中它。
  - `locateNotFound`：断言 `.notFound(tried:)` 里有 4 个路径。如果 override 是 nil，就是另外 3 个路径。
  - `snapshotParses`：脚本 `cat` 一份 fixture JSON。
  - `failureCarriesStderr`：脚本 `echo boom >&2; exit 1`，断言是 `.failed(code: 1, stderr: "boom")`。
  - `timeoutKills`：脚本 `sleep 30`，timeout 设 1 秒。断言 2 秒内抛 `.timedOut`。
  - `largeOutputDoesNotDeadlock`：脚本先往 stdout 输出 1 MB，再输出 fixture。用 `run` 直接断言拿到的字节数。
- [ ] **Step 2:** `swift test` → FAIL
- [ ] **Step 3: 实现** `CLI.swift`
- [ ] **Step 4:** `swift test` → PASS
- [ ] **Step 5: Commit** `feat(macos): run the hotseat CLI with timeouts`

---

### Task 4: 菜单栏 app

**Files:**
- Create/Replace: `macos/Sources/Hotseat/main.swift`, `AppDelegate.swift`, `AccountRow.swift`

**Interfaces:**
- Consumes: Task 2、3 的全部公开类型。

这一个 task 不写单测，验证放在 Step 2 和 Task 6。行为如下：

- 启动：`NSApp.setActivationPolicy(.accessory)`；建 `NSStatusItem(withLength: .variableLength)`，标题用 `Format.title`；启动时刷新一次，之后用 `Timer` 每 60 s 刷新一次，`menuWillOpen` 时也刷新。刷新时先显示缓存的数据，结果回来后再替换菜单项。
- 菜单从上到下：
  1. 每个账号一行（`NSMenuItem` 的 `view = NSHostingView(rootView: AccountRow(...))`，宽 300）。行里依次是：
     - 账号名。当前账号前面加 `●`，推荐账号后面加 `suggested`。
     - `Desktop`（这个账号是 Desktop active 时）或 `LIMIT until <when>`（红色）。
     - 两条 `ProgressView`，标签用 `Format.window`。
     - 一行灰字 `Format.age`；有 `desktopProblem` 时再加一行 `Desktop: <problem>`；有 `error` 时显示 error。
  2. `pendingRecovery` 为 true 时，加一个不可点的项：`Desktop switch was interrupted — run: hotseat desktop recover`。
  3. 刷新出错时，加一个不可点的项，显示 `HotseatError.message`。如果有上次的数据，账号行照常显示上次的。
  4. 分隔线，然后是 `Refresh`（⌘R）、`Launch at Login`（勾选状态取 `SMAppService.mainApp.status == .enabled`，点击时 register 或 unregister，失败就弹 `NSAlert` 显示错误）、`Quit Hotseat`（⌘Q）。
- 点账号行：行视图里用 `.contentShape(Rectangle()).onTapGesture`，先 `menu.cancelTracking()`，再执行切换。点的是当前账号，而且 Desktop 没有落后（`desktop.active` 是 nil 或等于这个账号）时，什么都不做。
- 切换前的确认：`desktop.running && desktop.active != 目标账号`，并且 UserDefaults 的 `skipDesktopConfirm` 不是 true 时，弹 `NSAlert`：
  - 标题：`Switch to <name>?`
  - 正文：`Claude Desktop will quit and reopen as <name> on the session you were in. Anything running in Desktop right now will stop.`
  - 按钮：`Switch`、`Cancel`。
  - 带 suppression checkbox `Don't ask again`，勾上后写入 `skipDesktopConfirm`。
- 切换时：设 `switching = true`，标题改成 `→ <name>…`，所有行变灰且不响应点击。完成后刷新。失败时弹 `NSAlert`，正文是 `HotseatError.message`。无论成败，最后都要把 `switching` 设回 false。
- 刷新时找 CLI 用 `HotseatCLI.locate(override: UserDefaults.standard.string(forKey: "hotseatPath"), home: FileManager.default.homeDirectoryForCurrentUser)`。

- [ ] **Step 1: 实现**三个文件
- [ ] **Step 2:** `cd macos && swift build -c release` 通过，没有 Swift 6 并发警告。`swift test` 仍然 PASS。
- [ ] **Step 3: Commit** `feat(macos): menu bar app`

---

### Task 5: 打包脚本和文档

**Files:**
- Create: `macos/build-app.sh`
- Modify: `README.md`（加 "Menu bar app" 一节：怎么构建、怎么安装、`hotseatPath` 怎么覆盖、登录启动、只装 CLT 就能构建）

`build-app.sh [--install]` 的步骤：
1. `swift build -c release --package-path macos`
2. 拼出 `macos/build/Hotseat.app/Contents/{MacOS/Hotseat,Info.plist}`。`Info.plist` 里要有：
   - `CFBundleIdentifier`、`CFBundleName`、`CFBundleExecutable=Hotseat`、`CFBundlePackageType=APPL`
   - `CFBundleShortVersionString` 和 `CFBundleVersion`，取自 `hotseat version` 的最后一个词；取不到时用 `0.0.0`
   - `LSUIElement=true`、`LSMinimumSystemVersion=14.0`
3. `codesign --force --sign - macos/build/Hotseat.app`，再跑 `codesign --verify` 检查。
4. 带 `--install` 时：先退出正在运行的 Hotseat（`osascript -e 'quit app "Hotseat"'`，失败也继续）；如果 `~/Applications/Hotseat.app` 已经存在，用 `trash` 移到废纸篓；然后 `ditto` 复制进去，再 `open` 它。

- [ ] **Step 1:** 写脚本，`chmod +x`
- [ ] **Step 2:** `macos/build-app.sh` → 输出 `.app` 路径，且 `codesign --verify` 通过；`plutil -p macos/build/Hotseat.app/Contents/Info.plist` 里能看到 `LSUIElement => true` 和版本号 `0.1.0`
- [ ] **Step 3: Commit** `build(macos): package Hotseat.app`

---

### Task 6: 真机验收（和用户一起，放在最终 review 之后）

先跑 `macos/build-app.sh --install`。这一步会在 `~/Applications` 装一个 app，先问用户。然后逐条确认：

1. 菜单栏标题和 `hotseat current` 一致，带 5h 百分比。
2. 每一行的数字和重置时间都和 `hotseat usage` 一致；claude 显示 `LIMIT until Sun 18:00`；claude2 显示 `Desktop: not logged in yet`。
3. Desktop 没运行时点 claude2：标题变成 `claude2 …`，`hotseat current` 输出 claude2，`hotseat desktop status` 里 claude2 是 active。验完切回 claude3。
4. Desktop 运行时点另一个账号：先出确认框；确认后 Desktop 重启，打开的是刚才的会话。
5. 执行 `defaults write io.github.zahT3.hotseat hotseatPath /usr/bin/false` 后刷新，菜单显示失败错误，账号行保留上次的数据。验完执行 `defaults delete io.github.zahT3.hotseat hotseatPath`。找不到 CLI 的情况由 Task 3 的单测覆盖，不改名真机上的 `hotseat`，因为 shim 和 statusLine 都依赖它。
6. 打开 Launch at Login 后，系统设置的登录项里出现 Hotseat。这一条未验证：ad-hoc 签名的 app 能不能用 `SMAppService.mainApp` 注册。如果不行，去掉这个开关，README 改成手动添加登录项。

然后按 superpowers:finishing-a-development-branch 合并。push 前问用户。

# Hotseat Open-Source Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让第一次接触 Hotseat 的 Mac 用户，能免费装好、在 app 里按引导配置完成、能一键卸载。同时把项目包装成一个可以公开的开源项目：CI、发布包、Homebrew、README、截图。

**Architecture:**
- CLI 新增两个命令：`hotseat setup status --json`，报告每个安装步骤是否完成；`hotseat uninstall`，撤掉所有集成。
- app 包内自带一份 `hotseat`，并新增一个 Setup 窗口，逐步调用 CLI 完成配置。app 本身仍然不碰任何账号数据。
- 发布全部用免费方式：
  - GitHub Actions 打出 zip。浏览器下载的文件会被隔离，README 写明怎么解除。
  - Homebrew formula 在用户本机从源码编译。本地编出来的 app 不会被打上隔离标记，所以不会被 Gatekeeper 拦下（推测：这是 macOS 的通常行为，没有在干净的机器上验证过）。

**Tech Stack:** Go 1.26 标准库；Swift 6 + SwiftPM，只用 CLT；GitHub Actions（macOS runner）；Homebrew formula。

**Spec:** `docs/superpowers/specs/2026-10-28-roster-design.md`，第 6.9 节"分发方式先用 GitHub Releases，Homebrew 以后再说"。以下是 2026-10-30 用户在对话里定下的：
- 不花钱，不做 Apple 公证。
- 名字保留 Hotseat。仓库描述和 topics 里要写上 "Claude account switcher"，方便被搜到。
- 宣传口径是"换账号不换工作现场"。不要说成绕过额度限制。
- 仓库先保持私有，等本计划做完再公开。

## Global Constraints

- 沿用计划 1–4 的全部约束：不读写凭证、不自动轮换、不删用户数据、测试只在临时 HOME 里跑、所有命令支持 `--json`、commit 带 trailer。
- Swift 测试一律用 `macos/test.sh` 跑。原因见计划 4：只装 CLT 时直接 `swift test` 会随机失败。
- app 的 SwiftUI 代码里不能用 `@State`（CLT 没有 SwiftUIMacros 插件），状态放在 `@Observable` 类里。
- 菜单里的视图不要加 `.onHover` 和 `.help`，加了会让 app 空闲时也一直占 CPU。
- 弹框只能在 `later {}` 里弹，也就是主队列上，不要放进 `Task`。
- 对外发布的东西只能出现示例数据：`example.com` 的邮箱、虚构的账号名。不能出现真实邮箱、UUID，也不能出现 `/Users/<真实用户名>`。
- 以下操作都要先问用户：仓库改成公开、推 tag、新建 tap 仓库、在 Mac Studio 上安装。

## Review Focus

1. **完全空白的机器**：没有任何 `~/.claude*` 登录，也没有 Desktop。`setup status` 要能正常输出，Setup 窗口要提示先用 `claude` 登录一次，app 不能崩，也不能显示空白。（Task 1、4）
2. **执行 `share apply` 时还有 claude 在运行**：apply 会拒绝执行，Setup 窗口要把 CLI 的原话显示出来，那一步仍然标成未完成。（Task 4）
3. **安装后用户改过自己的 statusLine 或 zshrc**：uninstall 不能覆盖用户改过的内容，要原样保留，并在输出里说明。（Task 2）
4. **`~/.local/bin/hotseat` 比 app 自带的版本旧**：Setup 要显示"有更新"并能一键更新，菜单不能因为新旧 JSON 字段不同而出错。（Task 3、4）
5. **从浏览器下载的 zip 被隔离**：README 要写出 `xattr -dr com.apple.quarantine` 这条命令，release 页面也要写。（Task 6、7）

---

## File Structure

```
internal/cli/setup.go          setup status：各步骤的检测
internal/cli/uninstall.go      uninstall：撤掉 shim、statusLine、hook
macos/Sources/HotseatKit/Setup.swift      SetupStatus 模型
macos/Sources/HotseatKit/CLI.swift        + setupStatus()、run step、bundled fallback
macos/Sources/Hotseat/SetupView.swift     Setup 窗口
macos/Sources/Hotseat/AppDelegate.swift   首次启动时打开 Setup，菜单里加 "Setup…"
macos/build-app.sh             把 hotseat 打进 Contents/Resources/bin/
macos/demo/hotseat-demo        输出示例 JSON 的假 CLI，截图用
.github/workflows/ci.yml       测试
.github/workflows/release.yml  v* tag 时构建 universal 二进制和 app zip，并上传到 Release
packaging/homebrew/hotseat.rb  从源码编译的 formula
README.md, CONTRIBUTING.md, SECURITY.md, CHANGELOG.md, .github/ISSUE_TEMPLATE/*.md, docs/images/*
```

---

### Task 1: `hotseat setup status --json`

**Files:** Create `internal/cli/setup.go`, `internal/cli/setup_test.go`；Modify `internal/cli/cli.go`

**Interfaces:**
- Produces: 输出下面的 JSON，退出码永远是 0。不带 `--json` 时，每个步骤输出一行文字，格式是 `✓/· 名称 说明`。

```json
{"version":"0.1.0","accounts":2,"loginsFound":2,
 "shareActions":0,"shim":true,"statusline":true,"hook":true,
 "desktop":"adopted"}
```

- 各字段的含义和取值：
  - `accounts`：registry 里登记的账号数。
  - `loginsFound`：`account.Discover` 找到的登录数。
  - `shareActions`：`loadPlan` 算出来的待执行动作数；registry 为空时是 0。
  - `shim`：zshrc 里有 Hotseat 的那一段，并且 `p.ShimDir()/claude` 存在。
  - `statusline`、`hook`：`settings.json` 里的命令是 Hotseat 的，用 `isOurs` 判断。
  - `desktop` 有三个取值：`adopted`（`State.Active != ""`）、`available`（`p.DesktopDataDir()` 存在但还没收编）、`absent`。

- [ ] **Step 1：写失败的测试**
  - `TestSetupStatusBlankMachine`：只用 `FakeHome`，什么都没装。期望 `accounts==0`、`loginsFound==0`，`shim`、`statusline`、`hook` 都是 false，`desktop=="absent"`，退出码 0。
  - `TestSetupStatusAfterSteps`：依次跑 `twoAccounts`、`init`、`shim install`、`statusline install`、`hook install`，再建一个假的 Desktop 数据目录。期望 `accounts==2`、`shim`、`statusline`、`hook` 都是 true，`desktop=="available"`。
- [ ] **Step 2：** `go test ./internal/cli -run TestSetupStatus` → FAIL（unknown command）
- [ ] **Step 3：实现** `cmdSetup`（只有 `status` 这个子命令），注册到 `cli.go`
- [ ] **Step 4：** `go test ./...` → PASS
- [ ] **Step 5：Commit** `feat: setup status for first-run onboarding`

### Task 2: `hotseat uninstall`

**Files:** Create `internal/cli/uninstall.go`, `internal/cli/uninstall_test.go`；Modify `internal/cli/cli.go`、`README.md`（Uninstall 一节）

**Interfaces:**
- Consumes：`shimUninstall(p)`；statusline 和 hook 的卸载逻辑，从 `cmdStatuslineInstall` / `cmdHook` 里抽成 `statuslineUninstall(p) error` 和 `hookUninstall(p) error`，原有命令改为调用这两个函数。
- Produces：`hotseat uninstall [--yes] [--json]`
  - 不带 `--yes`：列出将撤掉的三项，以及会保留的东西，然后退出码 1。会保留的是：`~/.claude*` 之间的共享链接（撤销要用 `hotseat share rollback <id>`）、`~/.config/hotseat`（备份和停放的 Desktop profile 都在这里）、Hotseat.app。
  - 带 `--yes`：依次执行三项卸载。某一项失败（比如 statusLine 已经被用户改过）时记下原因、继续下一项，最后输出每一项的结果。

- [ ] **Step 1：写失败的测试**
  - `TestUninstallNeedsYes`：不带 `--yes` 时退出码 1，输出里包含 `share rollback`，zshrc 没有被改。
  - `TestUninstallRemovesIntegrations`：先装好三项，再执行 `uninstall --yes`。期望 zshrc 里没有 `>>> hotseat >>>`，`settings.json` 里不再有 `isOurs` 认得的命令，用户原来的 statusLine 恢复成 `echo mine`。
  - `TestUninstallKeepsChangedStatusLine`：装好后，用户把 statusLine 改成 `echo changed`。执行 `uninstall --yes` 后，statusLine 仍然是 `echo changed`，输出里写明保留了它，退出码 0。
- [ ] **Step 2：** 运行这三个测试 → FAIL
- [ ] **Step 3：实现**，原有的 `statusline uninstall`、`hook uninstall` 测试仍然要过
- [ ] **Step 4：** `go test ./...` → PASS
- [ ] **Step 5：Commit** `feat: hotseat uninstall`

### Task 3: app 自带 CLI

**Files:** Modify `macos/build-app.sh`、`macos/Sources/HotseatKit/CLI.swift`、`macos/Tests/HotseatKitTests/CLITests.swift`

**Interfaces:**
- `HotseatCLI.locate(override:home:bundled:isExecutable:)`：新增参数 `bundled: String?`。查找顺序是 override → `~/.local/bin` → `/opt/homebrew/bin` → `/usr/local/bin` → bundled。只有普通文件才算找到，目录不算（顺带修掉计划 4 的 minor）。
- `HotseatCLI.installBundled(from: URL, to: URL) throws`：先复制到 `to.tmp`，chmod 755，再 rename 成 `to`。
- `build-app.sh`：
  - 先 `go build -o macos/build/hotseat ../cmd/hotseat`。如果设了环境变量 `HOTSEAT_BIN`，就改用它指向的文件（release 时传 universal 二进制）。
  - 把它复制到 `Contents/Resources/bin/hotseat`，然后再签名。
  - 退出 Hotseat 改用 bundle id：`quit app id "io.github.zahT3.hotseat"`（计划 4 的 minor）。

- [ ] **Step 1：写失败的测试**
  - `locateFallsBackToBundled`：前四个路径都不可执行时，返回 bundled 路径。
  - `locateSkipsDirectories`：`isExecutable` 对 override 返回 true，但 override 是一个目录时，要跳过它。
  - `installBundledCopies`：在临时目录里装一次，内容一致，权限是 0o755。
- [ ] **Step 2：** `macos/test.sh` → FAIL
- [ ] **Step 3：实现**；app 调 `locate` 时传入 `Bundle.main.resourceURL/bin/hotseat`
- [ ] **Step 4：** `macos/test.sh` → PASS；跑 `macos/build-app.sh` 后，`macos/build/Hotseat.app/Contents/Resources/bin/hotseat version` 输出 `hotseat 0.1.0`
- [ ] **Step 5：Commit** `feat(macos): ship the CLI inside the app`

### Task 4: Setup 窗口

**Files:** Create `macos/Sources/HotseatKit/Setup.swift`、`macos/Sources/Hotseat/SetupView.swift`、`macos/Tests/HotseatKitTests/SetupTests.swift`；Modify `CLI.swift`、`AppDelegate.swift`

**Interfaces:**
- `struct SetupStatus: Decodable`：字段和 Task 1 的 JSON 一一对应。另有 `func steps(bundledVersion: String, usingBundled: Bool) -> [SetupStep]`，按固定顺序给出：
  - `cli`（完成的条件：`!usingBundled && version == bundledVersion`；`usingBundled` 表示 locate 只找到了 app 自带的那份）
  - `accounts`（`accounts > 0`）
  - `share`（`shareActions == 0`）
  - `shim`
  - `tracking`（`statusline && hook`）
  - `desktop`（`adopted`，或 `absent` 时视为完成）
- `struct SetupStep { id: String; title: String; done: Bool; detail: String; command: [String]? }`。`command` 是这一步要执行的 hotseat 参数：
  - `["init"]`
  - `["share","apply","--json"]`
  - `["shim","install"]`
  - `["statusline","install"]` 和 `["hook","install"]`，这两条依次执行
  - `["desktop","adopt"]`
- `HotseatCLI.setupStatus() async throws -> SetupStatus`，以及 `HotseatCLI.runStep(_ args: [String]) async throws`。
- `title` 和 `detail` 的文案用下面的英文原文，`loginsFound == 0` 时用专门的说明：

| id | title | detail（未完成时） |
|---|---|---|
| cli | Command line tool | Installs `hotseat` to ~/.local/bin |
| accounts | Find your Claude logins | Registers the ~/.claude* dirs you are logged in to. None yet: run `claude` once and log in. |
| share | Share your work data | Links sessions, settings and skills of every account to ~/.claude. Close running claude sessions first. |
| shim | Follow the current account | New terminals start `claude` as the account you pick |
| tracking | Usage and limit handoff | Records usage from Claude Code's status line; offers another account when one runs out |
| desktop | Claude Desktop | Lets Hotseat switch Desktop's account |

- app 的行为：
  - 启动时调用 `setupStatus()`。只要有一步没完成，就自动打开 Setup 窗口（每个 app 版本只自动打开一次，记在 UserDefaults 的 `setupShownForVersion`）。菜单里另外加一个 "Setup…" 项，随时可以打开。
  - 窗口里每一步一行：左边是 ✓ 或圆点，然后是 title 和 detail；没完成的步骤右边有一个 "Run" 按钮。
  - 点 Run 后，执行时显示转圈。失败时把 `HotseatError.message` 用红字显示在那一行下面。执行完重新拉一次状态。
  - `cli` 这一步调用 Task 3 的 `installBundled`。
- [ ] **Step 1：写失败的测试**
  - `setupStepsFromStatus`：用一份 JSON fixture（只有 `accounts` 和 `shim` 没完成），断言 `steps.map(\.done)` 和每一步的 `command`。
  - `blankMachineAccountsDetail`：`loginsFound == 0` 时，`accounts` 这一步的 detail 里包含 `run \`claude\` once`。
  - `cliStepNeedsMatchingVersion`：`usingBundled: true` 时，或者版本和 `bundledVersion` 不同时，`cli` 这一步都是未完成。
- [ ] **Step 2：** `macos/test.sh` → FAIL
- [ ] **Step 3：实现**模型、CLI 方法、窗口和 AppDelegate 的接线
- [ ] **Step 4：** `macos/test.sh` → PASS；`swift build -c release` 没有警告
- [ ] **Step 5：Commit** `feat(macos): first-run setup window`

### Task 5: 计划 4 遗留的小问题

**Files:** `macos/Sources/HotseatKit/{Format,CLI}.swift`、`internal/cli/panel.go`、`README.md`，以及对应的测试

- 窗口的重置时间已经过了、但 snapshot 是旧的：`Format.value` 显示 `0% (reset)`，`Format.limit` 和卡片上的 LIMIT 都按 `now` 判断。测试 `staleWindowPastReset`。
- 超时后进程还在：先 `terminate()`；3 秒后还没退出就 `kill(pid, SIGKILL)`。测试 `timeoutKillsStubbornProcess`：脚本里 `trap '' TERM; sleep 30`，要求 5 秒内抛出 `.timedOut`。
- `panel` 读了两遍 registry 和 usage：改成 `usageRows` 直接用已经读好的 records。行为不变，`go test` 通过即可。
- README 里写明：app 不会继承 shell 里设的 `HOTSEAT_HOME`。
- [ ] 每条按"先让测试失败、再修好、再跑全套测试"的顺序做；最后一起 commit：`fix: leftovers from the menu bar review`

### Task 6: CI、发布包和 Homebrew formula

**Files:** `.github/workflows/ci.yml`、`.github/workflows/release.yml`、`packaging/homebrew/hotseat.rb`、`macos/test.sh`（Xcode runner 下的兼容处理）

- `ci.yml`：在 push 和 pull_request 时，于 `macos-latest` 上跑 `go vet ./...`、`go test ./...`、`macos/test.sh`、`macos/build-app.sh`。
- `release.yml`：push `v*` tag 时运行。
  - 分别以 `GOARCH=arm64` 和 `amd64` 编译，再用 `lipo` 合成 `hotseat` universal 二进制。
  - 运行 `HOTSEAT_BIN=... macos/build-app.sh`。
  - 打包 `Hotseat-<ver>-macos.zip`（里面是 app，app 里自带 CLI）和 `hotseat-<ver>-darwin-universal.tar.gz`。
  - 用 `gh release create` 上传，release 说明里要包含隔离解除命令 `xattr -dr com.apple.quarantine ~/Applications/Hotseat.app`。
- `hotseat.rb`：
  - `url` 指向 GitHub 自动生成的 tag 源码包，`sha256` 在发布时填写。
  - `depends_on "go" => :build`；`install` 里编译 `bin/hotseat`，并用 `macos/build-app.sh` 编出 `prefix/"Hotseat.app"`。
  - `caveats` 里写 `cp -R #{opt_prefix}/Hotseat.app ~/Applications/`。
  - `test do` 里跑 `assert_match "hotseat", shell_output("#{bin}/hotseat version")`。
- [ ] **Step 1：** 用 `actionlint` 检查两个 workflow（没有装就跳过，并在 ledger 里记一笔），同时检查 YAML 语法
- [ ] **Step 2：** 在本地模拟 release 的打包流程：`GOARCH=amd64` 和 `arm64` 各编一份，`lipo -info` 输出 `x86_64 arm64`；zip 解压后 `codesign --verify` 通过
- [ ] **Step 3：** `brew style packaging/homebrew/hotseat.rb` 没有错误
- [ ] **Step 4：Commit** `ci: tests, release packaging and a Homebrew formula`

### Task 7: README、社区文件和截图

**Files:** `README.md`（重写）、`CONTRIBUTING.md`、`SECURITY.md`、`CHANGELOG.md`、`.github/ISSUE_TEMPLATE/{bug_report,feature_request}.md`、`macos/demo/hotseat-demo`、`docs/images/{menu,settings,setup,statusline,handoff}.png`

- README 结构：
  1. 标题和一句话介绍："Claude account switcher for macOS — switch accounts, keep your place."，后面紧跟截图。
  2. Why：额度用完要换账号，但不想丢掉对话、设置和 Desktop 的会话列表。
  3. Features：每一项配截图。
  4. How it works：每个账号一个配置目录，共享数据都链接到 `~/.claude`；Desktop 的数据目录整体交换。
  5. What Hotseat never does：沿用现有那一节。
  6. Install：Homebrew 和 zip 两种方式，zip 方式写明隔离解除命令。
  7. Quick start：打开 app，跟着 Setup 走。
  8. CLI reference。
  9. Uninstall。
  10. FAQ，至少回答五个问题：会不会被封号（条款原文链接加风险说明）、Desktop 升级以后还能不能用、和其他切换器有什么区别、为什么需要每个账号一个目录、支不支持 Windows。
  11. License。
- `hotseat-demo` 只回应 `panel --json` 和 `setup status --json`，其他调用一律 `exit 0`。输出的示例数据：
  - 三个账号：`Work`（用户名 sam，`sam@example.com`，图标 `briefcase.fill`）、`Personal`、`Side project`；
  - 其中一个 LIMIT，一个 Desktop 未登录。
- 截图流程，需要用户配合：
  1. 运行 `defaults write io.github.zahT3.hotseat hotseatPath <demo 的路径>`；
  2. 用户自己打开菜单、设置窗口和 Setup 窗口；
  3. 用 `screencapture -i docs/images/<名字>.png` 截图；
  4. 截完运行 `defaults delete` 恢复。
- 终端状态栏和额度用完提示框的截图，也要用示例账号截。
- [ ] **Step 1：** 写好全部文件和 demo 脚本
- [ ] **Step 2：** 和用户一起截图；运行 `git grep -nIE "@gmail|/Users/[a-z0-9]+" -- README.md docs/images` 应无输出（只检查文字部分）
- [ ] **Step 3：Commit** `docs: README, community files and screenshots`

### Task 8: 公开发布（逐项先问用户）

1. 在 Mac Studio（用 `studio` skill）上从零装一遍：按 README 的 zip 方式安装，跟着 Setup 走完。记下所有卡住的地方，修好后再继续。
2. 把仓库改成公开：`gh repo edit --visibility public --accept-visibility-change-consequences`，再设置 description 和 topics：`claude-account-switcher`、`claude-code`、`claude-desktop`、`multiple-accounts`、`macos`、`menu-bar`。
3. 推 `v0.1.0` tag，等 release workflow 跑完，下载产物核对。
4. 新建 `zahT3/homebrew-tap` 仓库，放入填好 sha256 的 formula。然后在 Studio 上跑 `brew install zahT3/tap/hotseat` 验证。
5. 宣传文案草稿交给用户，由用户自己发。

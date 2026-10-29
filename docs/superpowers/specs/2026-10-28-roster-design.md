# Roster 设计文档

> 2026-10-29 起项目改名为 **Hotseat**：文中的 Roster / `roster` 即 Hotseat / `hotseat`，状态目录 `~/.config/roster` 即 `~/.config/hotseat`。

日期：2026-10-28　状态：待用户确认　工作名：Roster（可改）

本文给拿到文档就要开工的人看。凡是"已验证"的条目，都在 2026-10-28 这台机器上实测过或查过官方文档；"推测"的条目在第 8 节列为待实测。标"用户决定"的是用户在讨论里定的，标"设计者决定"的是写文档的人替用户定的，用户可以推翻。

## 1. 意图

同一台 Mac 上轮流使用多个个人 Claude 订阅账号的人，现在每换一次账号，工作环境就跟着变：Desktop 要注销、收验证码、重新登录，登录后 Code 会话列表、Desktop 设置、agent 配置全变成另一套，原账号的会话"死"在那里。

Roster 的原则：**账号可插拔，工作不动**。会话、设置、agent 配置、项目属于"工作"，只有一份；账号只决定这一刻花谁的额度。从 A 换到 B，除了额度来源，什么都不应该变。

用户自己已经在 CLI 上手工做到了大半（见 5.2），第一版的及格线是把这套手工配置做成工具，并把 Desktop 补上。

## 2. 用户和场景

目标用户：手里有多个账号、在本机跑大项目、需要大量 token 的人。第一个用户是本机用户（三个 Claude Pro 账号）。Chat 标签页的聊天不在范围内，这类用户在意的是本地 Code 会话。

核心场景：A 的额度用完 → 点开 Roster 菜单栏面板 → 看到每个账号的用量和重置时间 → 点 B → Desktop 自动重启成 B，Code 列表和之前一样 → 点开刚才那条会话接着做，花的是 B 的额度。CLI 里同理：新开的终端自动用 B，额度用完的那条会话可以一键用 B 接上。

用户原话："我点开咱们的账号管理工具，选中我要切过去的账号，点击一下，我则可以继续我的工作（如果能无感在 desktop 里继续是最好的，自动重启 app 也可以的）。"

## 3. 目标、非目标、反目标

### 目标（第一版）

1. 收编用户现有的多账号配置，不破坏现有用法。
2. 新增账号：一条命令或一次点击，登录走 Anthropic 自己的流程，CLI 和 Desktop 各登录一次，之后不再登录。
3. 用量面板：每个账号的 5 小时和每周用量、重置时间、数据新旧、当前账号。
4. 一键切换：CLI 的"当前账号"和 Desktop 同时切换；Desktop 自动重启，Code 会话列表在所有账号间一致。
5. 额度用完时接续：CLI 会话被限额打断时发系统通知，点一下用另一个账号在同目录 resume 同一条会话。
6. 工作数据在账号间完整共享（见 6.2 的分类）。

### 非目标（第一版不做，以后可能做）

- Windows、Linux。（用户决定：先只支持 macOS，手边也没有 Windows 机器。）
- ChatGPT、Kimi 等其他家账号。（用户决定：先作为 Claude 专用工具。长期愿景是管多家账号，但第一版不写跨厂商的抽象层。）
- 多个 Desktop 实例同时运行。第一版同一时刻只运行一个 Desktop 实例，切换等于"退出当前实例、启动目标实例"。（设计者决定：避免两个实例同时写同一份索引和对话文件。CLI 的多账号并行不受限制。）
- 按项目自动选账号（类似 claude-profile-manager 的 `.claude-profile`）。用户的模型是"这段时间用 A"，不是"这个项目用 A"。
- Chat 聊天记录、claude.ai 服务器端设置（个人偏好说明、connectors 等）的同步。

### 反目标（做了就算失败，哪怕功能能用）

- **自动轮换账号**：快到限额就自动换号。（用户决定，理由是条款风险：Consumer Terms 禁止绕过"systems or protective measures"。只能用户手动点。）
- **读取、保存、复制、转发任何 token 或凭证**：包括各配置目录的 `.credentials.json`、钥匙串里 Claude Code 的条目、Desktop 数据目录的 `config.json` 中的 `oauth:tokenCache*`、`Cookies`。也不调用 `/api/oauth/usage` 这类需要 token 的接口。（设计者决定，依据见 4.1。）
- 代理、账号池、请求转发。
- 修改 Claude.app 程序包（改 bundle id、重签名等）。
- 破坏用户现有配置：任何迁移都要先备份、能回滚。

## 4. 约束

### 4.1 条款（已验证：官方文档原文）

- Claude Code [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)："developers may not collect, store, or intermediate Claude.ai credentials or session tokens — sign-in to a Claude account must complete through Anthropic's own flow."
- 同页：不能把 "Claude Code" 或 Anthropic 名称用作产品名的一部分。所以产品名里不出现 Claude。
- 官方支持的多账号方式（[Authentication · Log in with multiple accounts](https://code.claude.com/docs/en/authentication)）：每个账号一个 `CLAUDE_CONFIG_DIR`；macOS 钥匙串条目按配置目录区分。Roster 的 CLI 部分完全建立在这个机制上。

### 4.2 本机约定（来自用户的 维护者的全局 agent 规则 和 维护者本机的目录约定文档）

- 项目在 项目目录。删除用 `trash`，不用 `rm`；移动或重命名用 维护者本机的迁移脚本 里的 `man_mv` / `man_rename`。
- 和 app 绑定的目录：真目录放在 Developer，原位置留隐藏的 symlink，不擅自改 plist。动 `~/Library/Application Support/Claude` 之前先读 CURRENT.md。
- 现有 hook 的脚本在 一个单独的 hooks 仓库。`~/.claude/settings.json` 是手写的，直接引用这些脚本，有 `PreToolUse`（Bash，删除保护）和 `UserPromptSubmit`（附加发送时间）两条。它不是那个 hooks 仓库生成的（已验证），所以 Roster 往里追加 hook 条目即可，不能删改已有条目。
- 开发和测试一律用临时 HOME 下的假目录，不对真实的 `~/.claude*` 和 Desktop 数据目录做破坏性测试。

## 5. 已经查清的事实

### 5.1 现有项目（调研于 2026-10-28）

| 类别 | 代表 | 借鉴 | 避开 |
|---|---|---|---|
| 换 token 的切换器 | realiti4/claude-swap（★4166）、uwuclxdy/clauth（★267）、XueshiQiao/CCSwitcher（★199） | claude-swap 的用量面板：拿不到新数据时显示上次的数，并标出数据多旧 | 复制 token、自动轮换 |
| 配置目录方案 | habibtalib/claude-profiles、JakubKontra/claude-profile-manager、willkelly 的 gist | 共享项用 symlink、MCP 配置同步、"有些项允许单个账号独立" | 都是小脚本，没有 Desktop 支持和用量面板 |
| 用量监控 | steipete/CodexBar（★22860，Swift） | 菜单栏交互、Swift app 加 CLI 的结构 | 读 token 调私有接口 |
| 服务商切换 | farion1231/cc-switch（★149527，Rust/Tauri） | 以后扩展到多家时的参考 | 不是多账号工具 |

没有任何现有项目处理 Desktop 的 Code 会话列表。这是 Roster 和它们的主要区别。

### 5.2 本机现状（已验证）

- Claude Code 2.1.310；Claude Desktop 4.29.0，自带 Claude Code 2.1.309。
- 三个配置目录：`~/.claude`（命令 `claude`）、`~/.claude2`（`claude2`）、`~/.claude3`（`claude3`）。后两个的别名在 `~/.zshrc` 第 46、51 行：`alias claude2='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'`。
- `.claude2` 和 `.claude3` 里，`CLAUDE.md`、`settings.json`、`projects/` 是指向 `~/.claude` 的 symlink。`CLAUDE.md` 的内容是 一行 `@` 导入全局规则文件。Claude Code 写 `settings.json` 时不会破坏 symlink。
- 各账号仍然独立的有：`history.jsonl`、`plugins/`、`skills/`、`file-history/`、`.claude.json`（含 MCP 服务器、项目信任状态）、`stats-cache.json`、`sessions/`、`backups/`。`~/.claude` 里还有 `commands/`，其他两个目录没有。
- 没有配置 statusLine。
- 账号身份：`CLAUDE_CONFIG_DIR=<目录> claude auth status --json` 返回 `email`、`orgId`、`subscriptionType`。`<目录>/.claude.json`（默认目录是 `~/.claude.json`）的 `oauthAccount.accountUuid`、`organizationUuid` 是 Desktop 用来分文件夹的那组 ID。这两处都不含 token。
- 映射：`claude` 对应 `<acct-1>…`，`claude2` 对应 `<acct-2>…`，`claude3` 对应 `<acct-3>…`。

### 5.3 Desktop 的存储结构（已验证：只读查看加实验）

数据目录 `~/Library/Application Support/Claude/`。

- **Code 会话列表**：`claude-code-sessions/<accountUuid>/<orgUuid>/local_<uuid>.json`，每个会话一个索引文件。字段包括 `sessionId`（`local_…`）、`cliSessionId`、`cwd`、`model`、`title`、`isArchived`、`createdAt`、`lastActivityAt`、`permissionMode`、`completedTurns` 等 40 多个。
- **对话内容**：在 `~/.claude/projects/` 里，和 CLI 共用。Desktop 自带的 Claude Code 用的是默认配置目录 `~/.claude`，不管 Desktop 登录的是哪个账号。实验中，账号 3 的 Desktop 实例把新消息写进了 `~/.claude/projects/...`，`entrypoint` 字段是 `claude-desktop`。
- **按账号分文件夹的还有**：`scratch-workspaces/`、`local-agent-mode-sessions/`（Cowork）、`pending-uploads/`、`blob_storage/`。
- **设置**：`claude_desktop_config.json` 有 `mcpServers` 和 `preferences`。主实例有 27 项偏好、1 个 MCP 服务器，新实例只有 15 项、0 个；其中 `bypassPermissionsGateByAccount`、`bypassPermissionsOptInByAccount`、`coworkModelAutoFallbackByAccount` 按账号存值。此外还有 `Claude Extensions/`、`Claude Extensions Settings/`、`extensions-installations.json`，以及按账号存值的 `ccd-session-presets.json`（`byAccount`）。
- **用量记录**：`plan-usage-history.json`，格式是 `{"version", "samples": [{"t": 毫秒时间戳, "org": orgUuid, "u": {"fh": 5小时百分比, "sd": 7天百分比}}]}`。没有重置时间。主实例 30 天里记了 112 条；新实例首次启动时立即记了 1 条（账号 3：`fh 16, sd 12`）。`fh`/`sd` 的含义是根据数值推测的：账号 1 周额度用完时，记录正好是 `sd: 100`。
- **凭证**：`config.json` 里的 `oauth:tokenCache`、`oauth:tokenCacheV2`，还有 `Cookies`。Roster 不碰这些。
- 主实例里还有两个旧账号的会话文件夹：`<old-acct-a>…`（297 条）和 `<old-acct-b>…`（593 条），不属于当前三个 CLI 配置目录。

### 5.4 Desktop 实验结果（已验证，2026-10-28）

做法：用 `open -n -a /Applications/Claude.app --args --user-data-dir=<目录>` 开临时实例，实验文件在 一个临时实验目录。

| 问题 | 结果 |
|---|---|
| 同一账号换一个数据目录，Code 列表还在吗 | 不在，是空的 |
| 把账号 1 的索引文件复制进另一个实例的账号文件夹 | 重启后出现在列表里，历史完整显示 |
| 复制到另一个账号（账号 3）的文件夹后能续聊吗 | 能。账号 1 当时周额度已用完，续聊成功，说明用的是账号 3 的额度 |
| 工具自己生成的索引（指向一个纯 CLI 开的会话）能用吗 | 能。所以 CLI 会话也可以出现在 Desktop 列表里 |
| 多开的实例重启后还保持登录吗 | 保持 |
| Desktop 什么时候读索引 | 启动时会读。运行中放进去的文件能不能实时出现，没测到 |
| 续聊用哪个模型 | 用索引里的 `model` 字段（实验会话在 CLI 里是用 haiku 创建的，Desktop 续聊用的是索引里的 opus） |

### 5.5 CLI 侧可用的官方接口（已验证：官方文档）

- **statusLine** 的 stdin JSON 有 `rate_limits.five_hour.used_percentage`、`rate_limits.five_hour.resets_at`（Unix 秒），`seven_day` 同理；只对 Pro/Max 账号提供，并且要等会话里第一次 API 响应之后才有。还有 `session_id`、`transcript_path`。statusLine 命令继承 claude 进程的环境变量，所以能从 `CLAUDE_CONFIG_DIR` 知道是哪个账号（推测，依据是子进程继承环境）。
- **`StopFailure` hook**：一轮因 API 错误结束时触发，matcher 可以填 `rate_limit`。输入至少有 `session_id`、`transcript_path`；有没有重置时间字段，没查到。
- 限额报错会以文本形式写进对话文件，例如 `You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)`，可以从中解析重置时间。
- `claude -p "/usage"` 只输出用量构成，没有额度百分比和重置时间。没有不消耗额度就能查额度的官方命令。
- 官方说明里，`CLAUDE_CODE_OAUTH_TOKEN`（由 `claude setup-token` 生成）的优先级高于 `/login` 的登录。Roster 运行时如果发现这个环境变量存在，要警告用户，否则切换会悄悄失效。

## 6. 已定的做法

### 6.1 账号模型

- 一个账号对应：一个 CLI 配置目录、一个 Desktop 数据目录（第一次在 Desktop 里用这个账号时创建）、一个显示名、accountUuid/orgUuid（从 `.claude.json` 读，只读这两个字段和 `auth status` 的输出）。
- 收编现有账号时，**沿用现有路径**：`~/.claude`、`~/.claude2`、`~/.claude3` 原地保留，主 Desktop 数据目录也原地保留，作为它当前登录账号的实例。新账号的目录放在哪里由实现者决定。（设计者决定：不搬已有目录，`claude2` 这类别名照样能用。）
- 登录：CLI 用 `CLAUDE_CONFIG_DIR=<新目录> claude auth login`；Desktop 由 Roster 用 `--user-data-dir` 启动一个新实例，用户在里面登录。两种登录都是 Anthropic 自己的流程，Roster 只负责启动。Desktop 登录前，Roster 要先退出其他 Desktop 实例，防止登录回调跳错窗口（这条来自社区经验，未验证）。

### 6.2 共享规则

"工作"只有一份，真身放在 `~/.claude`；其他账号目录里对应的位置都是 symlink。"账号"相关的部分每个目录各自一份。Roster 内置一张分类表，`doctor` 命令检查实际情况和表是否一致，遇到不认识的新条目就报告，交给用户归类。不认识的条目默认不共享。

| 归"工作"，共享 | 归"账号"，各自一份 | 需要实测再定 |
|---|---|---|
| `CLAUDE.md`、`settings.json`、`projects/`（已在共享）、`history.jsonl`、`file-history/`、`tasks/`、`plans/`、`paste-cache/`、`skills/`、`commands/`、`agents/`、`output-styles/`、`rules/`、`workflows/`、`agent-memory/`、`keybindings.json`、`themes/` | `.credentials.json`、`.claude.json`、`stats-cache.json`、`remote-settings.json`、`policy-limits.json`、`backups/`、`shell-snapshots/`、`session-env/`、`statsig/` 等缓存 | `plugins/`（`installed_plugins.json` 里可能有绝对路径）、`sessions/`（共享后能不能让 Claude Code 察觉跨账号的并发）、`jobs/`、`daemon/`、`uploads/` |

`.claude.json` 不能整个共享，因为里面有账号信息；但其中的 `mcpServers`（用户级 MCP 服务器）要在各账号间保持一致，由 Roster 同步。同步方向和冲突处理由实现者决定，规则要写在 `doctor` 的输出里。

把原来独立的文件合并成共享时（比如三份 `history.jsonl`），要先合并内容再建 symlink，不能直接丢掉其他账号的那份。

### 6.3 当前账号和 CLI

- Roster 维护一个"当前账号"。
- 提供一个 `claude` 包装命令，放在 PATH 中真正的 `claude` 之前。它读取当前账号，设置 `CLAUDE_CONFIG_DIR` 后用 exec 启动真正的 claude，参数原样传递。用户已经设置的 `CLAUDE_CONFIG_DIR` 优先，这样 `claude2` 之类的别名和临时指定账号仍然有效。
- 切换只影响之后新启动的 claude，已经在跑的会话继续用原账号，直到退出。
- 在共享的 `settings.json` 里配置 statusLine，命令是 `roster statusline`。它做两件事：把 `rate_limits` 写进 Roster 的用量存储；输出一行短状态，例如 `账号 B · 5h 16% · 7d 12%`。如果用户以后自己配置了 statusLine，要能串起来，不能覆盖。（设计者决定：顺便解决"这个终端到底在用哪个账号"的问题。）

### 6.4 Desktop

- 每个账号一个 Desktop 实例，即一个独立的 `--user-data-dir`，各自保持登录。同一时刻只运行一个。
- **会话列表一致**：所有账号文件夹里的会话索引合并成一份，每个实例看到的都一样。合并范围默认包括 5.3 提到的两个旧账号的 890 条会话，让它们"复活"；同时提供开关，可以把某个账号的历史会话排除在外。（设计者决定：符合"工作不跟账号走"，代价是列表会变长。）还要给只在 CLI 里出现过的会话生成索引，让它们也出现在 Desktop 列表里（实验已证明可行）。生成的索引字段以真实索引为模板，`cliSessionId`、`cwd`、`title`、时间戳、`model` 按会话填写。
- 实现方式由实现者选：
  - (a) 各实例的账号文件夹 symlink 到同一个共享文件夹，列表天然一致；
  - (b) 切换时双向复制，同一 `sessionId` 冲突时以 `lastActivityAt` 新的为准。

  先用实验验证 (a) 的 symlink 会不会被 Desktop 写坏，可以的话优先用 (a)。
- **设置一致**：`claude_desktop_config.json` 的 `mcpServers` 和 `preferences`、`Claude Extensions` 相关文件、`ccd-session-presets.json`，在实例之间保持一致。按账号存值的键（`…ByAccount`、`byAccount`）要把值映射到目标账号的键下，而不是原样复制。
- **从 Dock 或聚焦搜索直接打开 Claude** 时，打开的是默认数据目录，而不是当前账号的实例。处理办法由实现者在实测后选择，例如让默认目录位置成为指向当前实例的 symlink（这属于"改应用路径"，动手前先读 CURRENT.md，并征求用户同意）。实在做不到时，至少在面板里提示"请从 Roster 打开 Desktop"。

### 6.5 一键切换流程

1. 用户在面板里点账号 B。
2. Roster 检查：Desktop 里有没有正在运行的 Code 会话（`claude-code/<版本>/` 下的进程）；最近使用的那条会话有多大。大小从对话文件里最后一次请求的 `input_tokens + cache_creation_input_tokens + cache_read_input_tokens` 估算，这个数字决定 B 续聊时要无缓存重读多少。有需要提醒的就弹窗确认。
3. 把当前账号设为 B。
4. 同步 Desktop 的会话索引和设置到 B 的实例。
5. 正常退出当前 Desktop 实例，启动 B 的实例。如果 Desktop 支持打开指定会话的链接，就直接打开刚才那条；不支持的话，那条会话也在列表最上面（链接是否存在未验证）。
6. 如果 B 还没有 Desktop 实例，就走 6.1 的登录流程。

目标是从点击到能继续工作只用几秒。无法做到不重启就换号，因为那需要往运行中的 Desktop 换 token，属于反目标。

### 6.6 CLI 额度用完时接续

- 在共享 `settings.json` 里加 `StopFailure` hook，matcher 为 `rate_limit`，命令是 `roster hook stop-failure`。
- 触发后：记录该账号的限额状态和重置时间（从 hook 输入或对话文件里的报错文本解析），然后发 macOS 通知，例如"账号 A 额度用完，11 月 1 日 18:00 重置。点击用 B 继续"。B 由 Roster 推荐，规则是剩余额度最多的账号，用户可以在面板里改选。
- 点击通知后：把当前账号设为 B，在原会话的 `cwd` 开一个新终端窗口或标签，运行 `claude --resume <session_id>`。用哪个终端 app 由实现者决定，至少支持系统自带的 Terminal。
- 这一步必须用户点击才发生，不自动执行。

### 6.7 用量面板

- 每个账号一行：显示名、套餐类型、5 小时已用百分比和重置倒计时、7 天已用百分比和重置时间、数据更新于多久之前、当前账号标记、"切换"按钮。
- 数据来源都是官方程序写在本地的东西：
  - statusLine 采集（有百分比和重置时间）；
  - 各 Desktop 实例的 `plan-usage-history.json`（只有百分比，按 `org` 对应账号）；
  - `StopFailure` 和对话文件里的限额报错（有重置时间）。

  同一个窗口取最新的一条观测。
- 闲置账号只有上次的数据。已经过了 `resets_at` 的窗口显示"已重置"（推算）。每周窗口的重置时刻是固定周期，知道一次就能往后推算（推测）。
- 显示数据有多旧，不伪装成实时数据。

### 6.8 收编现有配置（第一次运行）

1. 只读扫描，输出计划：发现哪些配置目录和别名、Desktop 实例、哪些条目要改成共享、哪些文件要合并。
2. 用户确认后，先把要动的东西备份到 Roster 的备份目录，再执行。
3. 执行后自检：每个账号运行 `claude auth status` 仍然是原账号；`claude2 --resume` 能看到所有会话；`doctor` 无报错。
4. 提供回滚命令，按备份恢复。

### 6.9 技术栈（设计者决定）

- **核心：Go 1.26（本机已装）**，单个二进制，命令名 `roster`。所有子命令支持 `--json`。负责文件系统操作、分类表、索引同步、启动和退出 Desktop（`open -n -a`、AppleScript quit）、hook 和 statusLine 处理、用量存储。理由：启动快，适合 statusLine 频繁调用；以后做 Windows 可以复用。
- **菜单栏面板：SwiftUI（本机 Swift 6.2）**，只做显示和点击，所有操作调用 `roster … --json`。系统通知也可以由面板发。
- **测试**：Go 的单元测试和集成测试都在临时 HOME 里构造假的 `.claude*` 目录和假的 Desktop 数据目录。涉及真实 Desktop 的行为，写成手动验收清单。
- **许可证**：MIT。分发方式先用 GitHub Releases，Homebrew 以后再说。

## 7. 留给实现者决定的

- Roster 自己的配置、状态、备份放在哪个目录，存储用什么格式。
- 新账号的 CLI 配置目录和 Desktop 数据目录放在哪里、怎么命名。
- 6.4 的 (a)/(b) 选哪个，以及合并冲突的细节。
- `mcpServers` 同步的方向和冲突规则。
- 推荐"用哪个账号继续"的具体排序。
- 移除账号的流程。底线是：不删除任何共享的工作数据；该账号的配置目录和 Desktop 实例目录用 `trash` 移走；登出这一步由用户自己在官方界面里完成。
- 某个 Desktop 实例里登录的账号和预期不一致时怎么处理（比如用户在实例里换了账号）。可以通过实例 `config.json` 的 `lastKnownAccountUuid` 检测，这个字段不是凭证。
- 面板的视觉设计、通知文案、终端 app 支持哪些。
- Go 和 Swift 两部分的代码结构，Go 的 CLI、TUI 库选型。
- 最低支持的 macOS 版本。

## 8. 风险和待实测项

动手写对应部分之前，先做小实验确认：

1. Desktop 的账号文件夹换成 symlink 后，Desktop 读写是否正常，升级后是否被替换掉。（决定 6.4 选 a 还是 b）
2. Desktop 运行中放进新索引，会不会实时出现。（决定切换时是否必须重启）
3. Desktop 有没有打开指定 Code 会话的链接。
4. 除了第一次启动，Desktop 每次启动是否都会写一条 `plan-usage-history`。如果是，"后台启动一下某账号的实例"就可以作为刷新闲置账号用量的手段。这一项可选，确认可行再做。
5. `plugins/`、`sessions/`、`jobs/`、`daemon/` 能否共享。
6. `StopFailure` 的输入里有没有重置时间。
7. 两个账号的 CLI 同时 resume 同一条会话时，对话文件会怎样（推测会分叉）。Roster 至少要在切换和接续时提醒。
8. 从 Dock 打开 Desktop 时，如何让它进入当前账号的实例。
9. 依赖 Desktop 未公开的文件格式，Desktop 升级可能导致失效。要让 `doctor` 能检测到格式变化（比如索引字段缺失、文件夹结构变了），并在面板里提示，而不是静默出错。

## 9. 第一版验收标准

1. 在本机运行收编流程后，`claude`、`claude2`、`claude3` 照常可用；任何一个账号都能 resume 到所有会话；有备份，回滚命令可用。
2. 新增一个账号，CLI 和 Desktop 各登录一次后，不再需要登录。
3. 面板显示每个账号的 5 小时和 7 天用量、重置时间、数据时间、当前账号。
4. 在面板点另一个账号：新开终端里的 `claude` 用新账号；Desktop 自动重启成新账号的实例，Code 列表和切换前一样（包括旧账号的会话，以及只在 CLI 里出现过的会话），点开就能用新账号的额度续聊。
5. CLI 会话被限额打断时收到通知，点击后在新终端用另一个账号接续同一条会话。
6. `roster doctor` 能报告断掉的 symlink、分叉的文件、分类表里没有的新条目、Desktop 格式变化、意外存在的 `CLAUDE_CODE_OAUTH_TOKEN`。
7. 全程不读、不写、不复制 5.3 和第 3 节列出的任何凭证文件和钥匙串条目。代码审查时逐项检查。
8. 自动化测试只在临时 HOME 里运行。

## 10. 参考

- 官方：[Authentication](https://code.claude.com/docs/en/authentication)、[Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)、[statusLine](https://code.claude.com/docs/en/statusline)、[hooks](https://code.claude.com/docs/en/hooks)、[.claude 目录](https://code.claude.com/docs/en/claude-directory)
- 同类项目：[claude-swap](https://github.com/realiti4/claude-swap)、[clauth](https://github.com/uwuclxdy/clauth)、[CCSwitcher](https://github.com/XueshiQiao/CCSwitcher)、[claude-profiles](https://github.com/habibtalib/claude-profiles)、[claude-profile-manager](https://github.com/JakubKontra/claude-profile-manager)、[CodexBar](https://github.com/steipete/CodexBar)、[cc-switch](https://github.com/farion1231/cc-switch)
- 官方多账号功能的需求：[anthropics/claude-code#18435](https://github.com/anthropics/claude-code/issues/18435)（Desktop）、#20131
- 本次实验文件：一个临时实验目录

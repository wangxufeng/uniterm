# 更新日志

## v2026.06.16-alpha

- **bugfix** 修复关闭本地终端 tab 导致程序崩溃。根因：多个 goroutine 并发调用 `ConPTY.Close()`，Windows 上 `ClosePseudoConsole` 被重复调用触发 OS 级访问违规，Go 的 `recover()` 无法捕获。修复：用 `sync.Once` 包裹完整 `Disconnect()` 体。
- **bugfix** 修复切换 tab 后后台终端输出丢失。KeepAlive 停用期间数据被 sessionStore 缓存，但切回时从未回放。追踪已写入 chunk 数，`onActivated` 中补写缺失数据。
- **bugfix** 修复后台 tab 中 SSH 断开后按 Enter 无法重连。`session:status` 事件被 `isActive` 守卫丢弃，`retryOnEnter` 从未设置。切回时从 `sessionStore.getStatus()` 同步。
- **bugfix** 修复重连后智能提示框定位在左上角。`terminalInput` 持有已销毁终端引用，光标追踪返回 `{0,0}`。sessionId 变更时重建 `terminalInput`。
- **bugfix** 修复重连后终端内容被清空。release + acquire 创建了全新 xterm.js 实例。改用 `transferTerminal()` 迁移终端条目到新 sessionId，保留 scrollback。
- **bugfix** 修复 vim 中按 Ctrl+G / Shift+G / PageDown 出现渲染残留。`\x1b[2J` 替换为 scrollClear 在交替屏幕中也会执行，破坏 vim 的屏幕状态。现在仅主屏生效。
- **bugfix** 修复 sidebar 分隔栏挡住连接列表滚动条。激活区域通过负 `right` 偏移移到 sidebar 外部。
- **improve** 文本高亮全面优化：
  - 高亮结束只用 `\x1b[39m` 重置前景色，不取消 vim 的反转视频
  - 有显示属性（反转视频、加粗等）的行跳过，避免颜色混叠
  - 文件路径正则匹配支持无扩展名文件和目录，用 `(^|\s)` 锚定避免词内误匹配
  - 新增日期格式：`HH:MM`（无秒）、ISO 8601 `Z` 后缀、syslog `Mon DD HH:MM:SS`、`Wed Jan 21 HH:MM:SS YYYY`
  - 颜色提取为命名常量；数字色 145→152，符号色 147→223，vim 反选下可辨
  - 本地终端不启用文本高亮
  - 智能提示框关闭时方向键不再触发弹出
- **refactor** 合并 `WorkspaceTabItem` 到 `TabItem`，消除约 300 行重复代码。所有 tab 类型统一由单一组件处理。
- **improve** Tab 关闭按钮替换为 Lucide `X` SVG 图标，调整尺寸和间距避免切 tab 时文字偏移。AI 锁按钮始终可见。本地终端隐藏 SFTP/监控菜单项。
- **bugfix** 修复面板从 workspace 拖出后 AI 锁定状态被清除。

## v2026.06.14-alpha

- **new** SSH 隧道（本地端口转发）。任何连接可选择已有 SSH 连接作为跳板，自动分配本地端口，通过隧道访问目标。VNC 自动处理 libvirt 端口偏移。
- **new** FTP/FTPS 文件传输。新增文件传输大类，支持 FTP 和 FTPS（显式 TLS），被动/主动模式、字符编码可选。复用 SFTP 两栏文件管理器 UI，Go 后端统一 fileTransferSession 接口。
- **new** SFTP 最大并发传输数配置（SSH 连接设置，默认 5），semaphore 控制同时传输文件数，避免带宽打满或触发服务器 MaxSessions 限制。
- **new** 连接表单分类调整为四类：终端 / 文件传输 / 远程桌面 / 数据库。SSH 标注为 SSH (SFTP)，同时出现在终端和文件传输下。
- **improve** 所有通知消息增加关闭按钮，5 秒自动消失。统一 `services/message.ts` 包装器。
- **improve** KeepAlive 缓存扩展至全部标签页组件（Settings/SFTP/RDP/VNC/SPICE），切标签不再重建组件。
- **improve** 字体改为系统原生字体栈，移除 Google Fonts CDN 依赖。UI 用系统界面字体，等宽用系统自带等宽字体，中文 fallback 覆盖 Windows/macOS/Linux。
- **bugfix** 修复 KeepAlive 下 SFTP 缓存实例监听全局拖拽事件导致文件误上传至其他连接的 bug。改由 onActivated/onDeactivated 管理事件。
- **bugfix** 修复快速新建连接时上次编辑的残留数据泄漏到新表单的问题。
- **bugfix** 修复并发传输时同一纳秒时间戳导致任务 ID 重复、进度条混乱的 bug。改用原子计数器。
- **bugfix** 修复连接端口 min 限制为 1 导致无法输入 0 的问题。
- **bugfix** 修复 body 4px padding 导致标题栏不贴顶的问题。
- **bugfix** 修复"本地终端"子菜单与主菜单之间 4px 缝隙导致鼠标划入子菜单消失的问题。
- **bugfix** 修复 AI 确认级别下拉按钮缺少 ChevronDown 图标 import 的问题。
- **bugfix** 修复切换连接类型时远程桌面类型间不更新默认端口的问题（如 RDP 3389 切到 VNC 仍为 3389）。

## v2026.06.13-alpha.1

- **new** 更新检查。设置关于页支持手动检查 + 后台自动检查 GitHub Releases，发现新版本弹出通知并可直接跳转查看详情。
- **fix** macOS 无边框窗口圆角改为原生实现。使用 Wails TitleBarHiddenInset，移除 CSS 圆角 hack，修复之前方形外框问题。
- **fix** macOS 窗口控制按钮改用系统原生红绿灯，移除自定义模拟按钮。

## v2026.06.13-alpha

- **new** AI 终端工具链。新增 5 个工具：start_command（启动后台命令）、capture_terminal（读取终端屏幕）、collect_output（被动等待输出）、send_terminal_key（发送终端输入）、interrupt_command（中断命令）。execute_command 新增超时和输出截断参数，AI 可自主控制等待时长。
- **new** AI SSE 流式响应。Go 后端转发 Anthropic SSE 事件，前端 ai:token 实时渲染 token 输出。
- **new** AI 上下文管理优化。系统提示词分层（静态缓存 + 动态注入），token 感知的上下文窗口管理，提升 prompt cache 命中率。
- **new** AI 对话 IN 框按工具类型解析展示。头部显示工具中文名和超时 `[xxs]`，体部按类型展示命令/参数，不再显示原始 JSON。
- **new** AI 侧边栏搜索。支持高亮匹配文本、上下导航（Enter / Shift+Enter）、匹配计数，自动滚动到当前匹配。
- **bugfix** 修复文本搜索菜单在所有终端窗口同时弹出搜索框的问题。事件细化到当前面板。
- **improve** AI 系统提示词重写。增加超时指南、超时决策树、交互式提示处理说明，禁止清屏命令。

## v2026.06.12-alpha

- **new** Zmodem 文件传输（rz/sz）。支持在 SSH 终端中使用 `rz -be` 上传（含直接拖拽文件到终端）、`sz` 下载文件，带实时进度条。
- **new** SSH 面板头部新增"..."菜单、标签右键菜单新增，包含复制会话、连接 SFTP、服务器监控、文本搜索。
- **improve** 重构终端实例管理，工作区面板和独立标签页拖拽合并/分离后不再重建 xterm 实例，消除切换过程中可能出现的乱码。
- **bugfix** 修复双击选中文字不复制到剪贴板。改用 xterm 原生 onSelectionChange 事件。

## v2026.06.10-alpha

- **new** AI 模型列表同步。在 AI 模型编辑弹窗中可一键从服务端拉取可用模型列表，模型输入框带下拉建议。
- **new** 侧边栏搜索支持按连接类型过滤（终端/远程桌面/数据库等），与文本搜索联合使用。
- **new** 多语言支持。支持简体中文、繁体中文、英文、日文、韩文、德文、西班牙文、法文、俄文 9 种语言，设置中切换实时生效。
- **improve** AI 命令标记简化，移除 `u='...'` 前缀仅保留 echo，移除自检提示词，运行确认面板默认展开。
- **improve** AI 写操作确认按钮配色修正为 primary 风格。
- **improve** 窗口标题去版本号，仅显示 uniTerm。
- **bugfix** 修复粘贴后终端失焦导致光标不显示的问题。
- **bugfix** 修复切换标签时 CSI 响应序列被 bash 回显为乱码的问题。

## v2026.06.08-alpha

- **new** SPICE 远程桌面协议支持。
- **new** 面板复制、重命名、拖拽图像预览、标题同步。
- **new** 将活动终端标签拖拽到相邻标签并合并工作区。
- **improve** SSH keepalive 从全局请求改为 session channel 请求（`keepalive@openssh.com`），对齐 OpenSSH `ServerAliveInterval` 行为；间隔从 30s 调整为 60s，最大失败次数从 2 调整为 3。
- **improve** Windows 上优先使用 Git Bash 而非 WSL，修复 WSL bash 参数传递问题。
- **improve** 多面板工作区中建议弹出框位置修复；SFTP 滚动行为优化。
- **bugfix** 修复终端重连后尺寸未更新。新 session 默认以 80×24 创建 PTY；现在重连后强制发送当前终端尺寸进行 `SessionResize`，vim/k9s 等全屏应用显示正确。
- **bugfix** 修复 `clear` 命令清除 scrollback 历史的问题。将 ED2（清屏）替换为换行滚动+归位，清屏前先将 viewport 内容推入 scrollback。
- **bugfix** 修复切换标签后文本高亮消失。恢复历史时根据当前 `highlightEnabled` 设置重新应用高亮。
- **bugfix** 修复选中复制在切换面板或从其他应用返回时误覆盖剪贴板。现在只有鼠标确实在本 terminal 内开始选择时才触发复制。
- **bugfix** 修复某些布局下将面板/标签拖放到空标签栏区域不生效的问题。

## v2026.06.02-alpha

- **new** Telnet 和 Mosh 连接协议支持。Telnet 提供 IAC 协商（二进制模式、终端类型、窗口大小）；Mosh 基于 UDP 的 SSP 协议实现低延迟移动连接。
- **new** 终端文本高亮。自动高亮终端输出中的时间戳、IP 地址、URL、文件路径、关键词（ERROR/WARN/INFO）、引号字符串、数字、括号等符号。设置中可开关，含 ESC 的行自动跳过避免干扰 TUI 应用。
- **new** xterm.js Unicode11 插件，正确渲染 emoji 等宽字符（如 k9s 小狗图标）。
- **improve** 标签栏与标题栏合并为一行，节省约 40px 垂直空间。按钮全部图标化，新建连接与本地终端合并为 `+` 下拉，窗口控制按钮风格统一。
- **improve** 新建/编辑连接界面重构为两级分类结构（终端 / 远程桌面 / 数据库），子级用 radio-button toggle 切换协议。
- **improve** 全界面控件风格统一：高 28px、字号 12px、圆角统一、边框和底色一致，涵盖 el-input、el-button、el-select、el-radio-button、el-switch、el-checkbox 等。
- **improve** 智能提示 UX 修复：提示框智能上下翻转避免遮挡输入行；鼠标静止时不会误选中提示项；密码隐藏输入不记入历史、不弹出提示。
- **improve** 终端标签改为按钮风格，活跃态有 accent 边框 + 底色，AI 锁定与选中效果叠加。
- **improve** AI 侧边栏默认新建会话，空会话不保存，最多保留 15 个会话。
- **bugfix** 修复终端历史记录读取逻辑。从可视区域末行扫描替代 cursorY，解决 buffer 滚动后无法读取 prompt 行命令的问题。
- **bugfix** 修复提示框 Enter 后未关闭的竞态条件（debounce 计时器取消 + 空 token 检查）。
- **bugfix** 修复 Windows 11 多进程 WebView2 冲突导致无法输入。UserDataFolder 改用进程 PID 隔离路径。
- **bugfix** 修复 Tab 切换时终端出现乱码。xterm.js allowProposedApi 开启后 OSC 颜色查询经 onData 被发往服务端回显为乱码，增加 OSC 过滤解决。

## v2026.05.29-alpha

- **new** 终端智能补全。SSH 终端输入时实时弹出历史命令和 AI 转写建议。设置页面新增历史命令管理栏目，支持搜索、全选、批量删除。
- **new** 服务器监控。实时监看已连接服务器的运行状态。支持 CPU/内存/磁盘/网络性能指标、进程列表及详情、监听端口、磁盘用量与挂载信息、网卡列表及 bond/bridge 识别。
- **new** SSH 登录后脚本执行。支持配置连接成功后自动执行脚本，支持空闲检测避免在用户手动操作时误执行。
- **new** SSH 保活机制防止空闲断开。定时发送保活包，连接断开时显示重连提示。
- **improve** 侧边栏分割条可见性和终端滚动条对比度优化，操作更便捷。
- **bugfix** 修复数据库 MySQL 多库查询竞态条件问题。
- **bugfix** 统一侧边栏分组与非分组视图中的连接类型标签渲染。

## v2026.05.27-alpha

- **new** 数据库连接与查询。支持 MySQL、PostgreSQL、rqlite 三种数据库，提供 SQL 查询执行、表结构浏览、数据行增删改查、数据库/表树形导航等功能。
- **new** 终端搜索栏。Ctrl+F 打开搜索栏，基于 @xterm/addon-search 实现匹配高亮和结果计数。
- **new** 连接侧边栏和 AI 侧边栏显示状态持久化到 localStorage，重启后保持上次的展开/收起状态。
- **improve** 滚动条宽度从 5px 增加到 8px，更容易抓取操作。
- **improve** AI 侧边栏最大化按钮在展开时显示缩回图标（Shrink），更直观。
- **improve** 窗口边缘增加透明内边距，终端填充边缘时仍可拖拽调整窗口大小。
- **improve** 工作区标签页增加 LayoutDashboard 图标。
- **bugfix** 修复标签页切换时终端出现乱码的问题。会话数据缓冲区裁剪点可能落在转义序列中间（DA2、OSC 颜色查询等），残缺片段缺少 \x1b 前缀被 xterm.js 渲染为乱码。修复方案：在第一个 \n 前扫描 \x1b 确定安全重启边界。

## v2026.05.25-alpha

- **bugfix** 编辑连接保存后，其他连接的密码信息丢失。修复 Go 后端 Save 方法底层数组共享导致的密码/APIKey 被意外清空的问题。

## v2026.05.24-alpha

- **new** 云端配置同步。基于 GitHub、GitLab、Gitee 私有仓库构建专属私人云同步仓库，所有配置（连接信息、AI 模型密钥、应用设置）经 AES-256-GCM 加密后保存至远端，支持自动同步、冲突解决、主密码修改和仓库绑定管理。

## v2026.05.23-alpha

- **new** VNC 远程桌面。支持通过 noVNC 连接 VNC 服务器（TigerVNC、TightVNC、QEMU 等），内置 WebSocket↔TCP 代理桥接；标签页切换时 DOM 保活实现零延迟恢复画面，支持自动缩放开关、剪贴板双向共享（Ctrl+Shift+V 粘贴本地剪贴板）。
- **new** 本地终端支持。可直接打开本地 shell（Windows PowerShell/CMD、macOS/Linux bash/zsh），无需 SSH 连接。
- **new** AI Shell 感知。AI 可以感知当前终端的 shell 类型，生成更准确的命令。
- **improve** 连接列表显示端口。Sidebar 中连接条目从 `user@host` 改为 `user@host:port`，用户名为空时仅显示 `host:port`。
- **improve** VNC 新建连接时隐藏用户名字段（VNC 认证只需要密码）。
- **improve** 图标统一。
- **bugfix** RDP 窗口在弹出菜单、对话框、窗口拖拽时正确隐藏/显示，避免遮挡。
- **bugfix** 设置页状态持久化问题修复。
- **bugfix** 密码输入框可见性切换修复。
- **bugfix** Windows 11 安全对话框抑制修复。

## v2026.05.22-alpha

- **new** RDP 远程桌面。支持通过 Microsoft RDP ActiveX 控件连接 Windows 远程桌面，原生安全对话框已完全抑制，RDP 窗口无缝嵌入 uniTerm 标签页，窗口拖拽和缩放时平滑跟随。
- **new** AI 边栏最大化按钮。可将 AI 助理窗口最大化至整个主区域，再次点击恢复原始宽度。
- **new** AI 消息复制功能。工具调用 IN/OUT 展开框旁增加复制按钮，点击后显示对勾反馈；消息右上角增加"复制为 Markdown"按钮，hover 后展开显示文字。
- **new** 广播输入功能。工作区面板标题栏增加广播按钮，可将键盘输入同时发送到工作区内所有终端。
- **new** 设置页关于栏目，显示应用版本号。
- **new** AI 命令执行确认模式新增"写操作确认"级别。原有三级（关闭/仅危险/全部）基础上增加"危险+写操作"选项，对 rm、mv 等写操作命令也需确认，填补危险与全部之间的安全粒度空缺。
- **new** 连接分组右键菜单增加"新建连接"，新建时自动预选该分组。
- **improve** 侧边栏选中状态统一。将 `selectedId` 与 `multiSelectedIds` 合并为单一 `selectedIds` 集合，选中高亮逻辑一致，修复右键选中时之前选中未清除的问题。
- **improve** 右键单击连接时，若该连接未在选中集合中，自动取消其他选中并仅选中当前连接。
- **improve** 标签栏优化：支持鼠标滚轮横向滚动，下拉菜单选中后自动滚动到可见位置。
- **improve** 系统提示消息角色从 `assistant` 改为 `tool`，避免污染 LLM 对话上下文。
- **improve** AI 会话存储从 localStorage 迁移至 Go 后端文件存储。
- **improve** README 重新设计：新增中文版本、介绍网站首页、功能分类展示和截图轮播。
- **bugfix** 修复标签页切换时终端出现乱码的问题（剥离不完整的转义序列）。
- **bugfix** 修复工作区中面板激活状态未同步终端焦点的问题。
- **bugfix** 修复选中单个连接时编辑按钮错误置灰的问题。
- **bugfix** 修复搜索无匹配结果时未自动选中"新建连接..."虚拟条目的问题。
- **bugfix** 修复标签标题显示带主机后缀的问题（应仅显示连接名称）。

## v2026.05.17-alpha

- **new** 连接分组功能。支持创建、重命名、删除连接分组，连接可按分组折叠展示，支持拖拽调整连接所属分组，无分组连接自动归入"(无分组)"虚拟分组。
- **new** 连接批量选择与操作。支持 Ctrl+点击多选、Shift+点击范围选择，右键菜单可批量连接、批量连接 SFTP、批量复制、批量删除选中的连接，回车键同样支持批量打开。
- **new** 搜索栏输入时列表底部显示"新建连接..."虚拟条目，双击或回车可将搜索内容预填到主机字段并打开新建连接窗口。
- **new** 删除连接前弹出确认提示，显示待删除数量。
- **change** 浅色主题重新设计为 Windows 风格中性灰色调，使用蓝色强调色替代原来的暖黄色。
- **improve** AI 会话名称、工作区名称、AI 锁定按钮提示等支持中英文国际化。
- **bugfix** 修复 AI 助理设置按钮跳转到基础设置而非 AI 设置的问题。
- **bugfix** 修复 SFTP 文件列表中".."返回上级目录行弹出右键菜单的问题。
- **bugfix** 修复多选连接时选中数量总是少一个的问题。

## v2026.05.16-alpha

- **new** SFTP 文件管理器，支持双栏浏览本地与远程文件，可进行文件上传、下载、重命名、删除、权限修改等操作，传输任务按标签页独立跟踪。

## v2026.05.15-alpha

- **new** 工作区面板系统。支持将多个终端标签页合并为工作区，在同一个窗口内左右或上下分屏显示，拖拽面板标题栏可自由调整面板位置和大小，拖拽标签页到面板边缘自动创建新的分屏。
- **new** 自定义窗口标题栏，适配 Windows 和 macOS 平台窗口控制按钮。
- **new** 终端内 http/https 链接自动识别并显示下划线，鼠标悬停提示，Ctrl+点击在默认浏览器中打开。
- **new** Windows 安装包增加运行中进程检测，安装前提示关闭正在运行的程序。
- **improve** 暗色主题对比度提升，默认和深蓝两套配色背景层次更分明，文字更易读。
- **bugfix** 选中文字自动复制到剪贴板现在即使鼠标在终端外松开也能生效。
- **bugfix** 修复 AI 多轮对话中 tool 响应丢失导致的错误。
- **bugfix** 修复面板分屏时子面板出现空白区域、分割条无法拖动的问题。
- **bugfix** 修复窗口或面板尺寸变化时终端内容显示异常。
- **bugfix** 修复 Ctrl+滚轮导致整个窗口意外缩放的问题。

## v2026.05.13-alpha

- **new** 支持自由分屏功能。窗口可以左右或上下拆分，同时打开多个终端，拖拽边界调整面板大小，标签页也可跨面板拖拽。
- **new** 支持AI窗口锁定按钮。每个终端标签页上的 "AI" 按钮可将 AI 锁定到该终端，锁定后 AI 执行命令只针对该终端，切换其他标签也不会跑错位置。
- **improve** 增加 AI 单次对话最多连续交互 20 轮（原来是 10 轮），达到上限后会出现 "继续" 按钮，点击可接着之前的话题继续聊；增加控制历史消息长度，防止上下文过长导致卡顿。
- **new** 左侧连接列表可用上下箭头切换选中项，按回车直接连接，不用鼠标双击。
- **change** Windows 只提供安装包（.exe），不再额外提供压缩包。macOS 只提供 DMG 镜像，不再额外提供压缩包。
- **improve** 终端默认保留的历史行数从 5000 行降到 2500 行，减少内存占用。
- **bugfix** 修复窗口或面板缩小时终端内容不跟随调整的问题。

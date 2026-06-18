# 快捷命令管理 — 设计文档

## 概述

为 uniTerm 新增快捷命令（Quick Commands）功能。用户可以保存常用 shell 命令，设置可选的显示名称，按分组整理，点击即可发送到当前活跃终端。

## 数据模型

```typescript
interface QuickCommand {
  id: string
  name?: string       // 显示名称，可选。为空时直接显示命令文本
  command: string     // 要发送的命令，如 "df -h"
  groupId?: string    // 所属分组（undefined = 未分组）
  sortOrder: number   // 排序
}

interface QuickCommandGroup {
  id: string
  name: string        // 分组名称，如 "系统监控"
  sortOrder: number
}
```

### 持久化

新增 Pinia store：`quickCommandStore`（`stores/quickCommandStore.ts`）。

存储方式与 `settingsStore` 一致：
- 后端导出：`SaveQuickCommands(data)` / `LoadQuickCommands()`
- Go 端在应用数据目录读写 JSON 文件
- 通过 version 字段支持向前兼容

```typescript
interface QuickCommandData {
  version: number     // 当前为 1
  groups: QuickCommandGroup[]
  commands: QuickCommand[]
}
```

### 数据同步

快捷命令数据纳入已有的数据同步机制，与其他数据（连接配置、设置）一起同步。修改 `app.go` 中同步数据结构和 `sync` 包，将 `QuickCommandData` 加入同步负载。

## 侧边栏改造

改造现有 `Sidebar.vue`，在 sidebar 头部（搜索框上方）增加横向切换标签。

### 布局

```
┌──────────────────────────┐
│ [连接列表] [快捷命令]     │  ← 顶部切换标签
├──────────────────────────┤
│                          │
│  面板内容                │
│  搜索 + 连接列表         │
│  或 快捷命令面板          │
│                          │
└──────────────────────────┘
```

- **切换标签**（内联于 Sidebar.vue）：
  - 两个横向排列的标签按钮
  - 连接列表 — 现有搜索 + 连接（默认选中）
  - 快捷命令 — 新的快捷命令面板
  - 激活标签底部高亮指示条

- **面板区**：根据选中的标签渲染对应面板

### 状态

- `activeView: 'connections' | 'quickCommands'` — Sidebar.vue 内的本地 ref
- 不持久化（每次启动默认显示连接列表）

## 快捷命令面板

新组件：`QuickCommandsPanel.vue`

### 布局（上到下）

```
┌─ [+ 分组] [+ 命令] ─────────┐
│                              │
│ ▼ 系统监控 (3)              │  ← 分组标题，可折叠
│     查看磁盘                  │  ← 命令项（有名称：名称在上）
│     df -h                     │  ←         命令在下，浅色
│     free -m                   │  ← 命令项（无名称：只显示命令，浅色）
│     uptime                    │
│                              │
│ ▼ 日志 (1)                  │
│     应用日志                  │
│     tail -f /var/log/app.log │
│                              │
│ (未分组)                     │
│     echo hello               │
│                              │
└──────────────────────────────┘
```

### 行为

| 操作 | 触发方式 | 效果 |
|------|---------|------|
| 选中命令 | 单击命令项 | 高亮该项 |
| 显示按钮 | 鼠标 hover 命令项，或单击选中 | 显示 Run / Paste 按钮 |
| 执行命令 | 双击命令项，或点击 [Run] 按钮 | 发送到活跃终端，命令末尾无换行则自动追加 `\n` |
| 粘贴命令 | 点击 [Paste] 按钮 | 原样发送到活跃终端，不追加 `\n` |
| 添加命令 | 点击 [+ 命令] | 打开编辑弹窗 |
| 编辑命令 | 右键命令项 → 编辑 | 打开编辑弹窗 |
| 删除命令 | 右键命令项 → 删除 | 确认后删除 |
| 添加分组 | 点击 [+ 分组] | 打开分组名称弹窗 |
| 重命名分组 | 右键分组 → 重命名 | 打开分组名称弹窗 |
| 删除分组 | 右键分组 → 删除 | 确认后弹出选择：命令移至未分组，或同时删除组内命令 |
| 折叠/展开 | 点击分组标题 | 切换分组开关 |

### 命令项显示

- 有名称时：上方显示名称（主文本），下方显示命令（浅色）
- 无名称时：仅显示命令（浅色）
- 长文本截断加省略号
- 选中态：高亮背景，显示 [Run] [Paste] 按钮
- hover 态：显示 [Run] [Paste] 按钮
- 未选中未 hover：按钮隐藏，简洁显示

## 命令编辑弹窗

新组件：`QuickCommandEditDialog.vue`

### 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 名称 | 文本输入框 | 否 | 占位文案："可选，为空时显示命令文本" |
| 分组 | 下拉选择框 | 否 | 列出所有分组 + "无分组" 选项 |
| 命令 | 多行文本框 | **是** | 等宽字体，占位文案："如 df -h" |

### 校验

- 命令字段不能为空 — 禁用保存按钮并给出提示

## 发送机制

两种模式将命令发送到活跃终端：

### Run 模式（双击或点击 [Run] 按钮）

1. 从 `tabStore` 获取当前 workspace 的 `activePanelId`
2. 从 `panelStore` 获取面板的 `sessionId`
3. 确定发送文本：如 `command` 已以 `\n` 结尾则原样发送，否则追加 `\n`
4. 按 `\n` 拆分，过滤空行
5. 逐行调用 `SessionWrite(sessionId, line + '\n')`，行间 100ms 延迟

### Paste 模式（点击 [Paste] 按钮）

1. 同 Run 模式获取 sessionId
2. 按 `command` 原文发送，不追加 `\n`，不拆行
3. 一次调用 `SessionWrite(sessionId, command)`

无活跃终端时（如当前为设置页），不做任何操作。

### 错误处理

- 无活跃终端 → 不操作（静默）
- 会话已断开 → 仍尝试发送（后端队列处理），不显示错误

## 实现范围

### 新增文件
| 文件 | 说明 |
|------|------|
| `stores/quickCommandStore.ts` | Pinia store，数据 + 持久化 |
| `components/QuickCommandsPanel.vue` | 侧边栏面板：命令列表 + 分组 |
| `components/QuickCommandEditDialog.vue` | 添加/编辑弹窗 |

### 修改文件
| 文件 | 修改内容 |
|------|---------|
| `components/Sidebar.vue` | 重构布局：左侧切换按钮栏 + 条件渲染面板 |
| `i18n/locales/*.json`（9 个） | 新增 `quickCommands.*` 翻译 key |
| `app.go`（Go 端） | 新增 `SaveQuickCommands` / `LoadQuickCommands` 接口 |

### 非功能需求

- **状态隔离**：`activeView` 切换状态仅限 Sidebar 内部，不影响其他组件
- **性能**：命令数量少（预计 < 100），无需虚拟滚动
- **拖拽排序**：v1 不做，后续迭代支持

## i18n 翻译 Keys

```
quickCommands.title          快捷命令
quickCommands.addGroup       添加分组
quickCommands.addCommand     添加命令
quickCommands.editGroup      编辑分组
quickCommands.deleteGroup    删除分组
quickCommands.groupName      分组名称
quickCommands.renameGroup    重命名分组
quickCommands.deleteConfirm  确定删除此命令？
quickCommands.noGroup        无分组
quickCommands.command        命令
quickCommands.name           名称
quickCommands.namePlaceholder 可选，为空时显示命令文本
quickCommands.commandPlaceholder 如 df -h
quickCommands.run            执行
quickCommands.paste          粘贴
quickCommands.save           保存
quickCommands.cancel         取消
quickCommands.noActiveTerminal 无活跃终端
```

## 验证点

1. 创建分组 → 确认面板中显示
2. 在分组中创建命令 → 确认名称和命令显示正确
3. 双击命令（Run）→ 确认发送到活跃终端并执行
4. 点击 Paste → 确认原样粘贴，不追加回车
5. 将命令名称清空 → 确认显示命令文本代替
6. 删除命令 → 确认已移除
7. 重启应用 → 确认命令和分组持久化
8. 无终端时点击命令 → 确认无报错
9. 侧边栏切换连接/快捷命令 → 确认面板切换正确

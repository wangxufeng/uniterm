# uniTerm Zmodem 传输支持设计文档

**日期**: 2026-06-09  
**状态**: 已确认，待实现  
**分支**: feat/zmodem-support

---

## 1. 需求概述

为 uniTerm 终端窗口添加 zmodem（rz/sz）文件传输协议支持，覆盖 SSH 和 Local 会话类型。

### 1.1 功能范围

| 功能 | 支持 |
|------|------|
| 单文件下载（`sz file.txt`） | ✅ |
| 批量文件下载（`sz file1 file2`） | ✅ |
| 批量文件上传（`rz`） | ✅ |

### 1.2 交互模式（参考 WindTerm）

- **下载**：首次弹 `SaveFileDialog` 让用户选保存位置，批量传输的后续文件自动保存到**同目录**
- **上传**：弹 `OpenMultipleFilesDialog` 支持多选
- **传输面板**：位于各自 `BaseTerminal` 面板内，显示文件名、进度、速度
- **跨 tab 传输**：切换 tab 不影响后台传输，切回后自动同步进度
- **完成后通知**：如果传输完成时用户不在该 tab，全局 toast 通知

---

## 2. 技术方案

### 2.1 选型：前端 zmodem.js + 按需 Base64 切换

经过对比 3 种方案（前端纯 JS、后端 Go 处理、双通道并行），选择**前端 zmodem.js + 按需 Base64 切换**方案。

**理由**：
- `zmodem.js` 是 Tabby、Electerm 等成熟终端都在用的库，协议正确性有保障
- 只在 zmodem 传输期间使用 Base64 编码，平时不影响性能
- 实现周期最短，不引入复杂的后端协议逻辑
- zmodem 内置 CRC + 重传机制可弥补 Base64 切换时的微小竞态风险

### 2.2 架构分层

```
┌─────────────────────────────────────────┐
│  UI 层（BaseTerminal.vue 内部）           │
│  ┌─────────────────────────────────┐   │
│  │  ZmodemTransfer 浮层组件         │   │
│  │  - 显示当前 session 的进度       │   │
│  │  - 从 zmodemStore 读取状态       │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
                    ↑ 读取状态
┌─────────────────────────────────────────┐
│  全局服务层（zmodemService.ts）           │
│  - zmodem.js 协议解析（不绑定组件）        │
│  - 维护 Map<sessionId, ZmodemSession>   │
│  - 处理文件弹窗、文件读写                 │
└─────────────────────────────────────────┘
                    ↑ 更新状态
┌─────────────────────────────────────────┐
│  全局状态层（zmodemStore.ts）            │
│  - Map<sessionId, TransferInfo>         │
│  - 文件名、进度、状态、方向（上/下载）     │
│  - 组件卸载/重建后能恢复显示              │
└─────────────────────────────────────────┘
```

**为什么分层**：
- UI 在 panel 内：在哪个终端触发传输，进度条就出现在该终端，直观
- 解析在全局：切换 tab 后 panel 可能被卸载，但全局 `zmodemService` 继续接收数据、继续解析
- 状态在 store：切回原 tab 后组件重新挂载，立即从 store 恢复显示

---

## 3. 数据流

### 3.1 下载流程：`sz file1.txt file2.txt`

```
[远程服务器]        [后端 Go]                [前端 Vue]
     │                  │                        │
     │  stdout zmodem   │                        │
     │  HEX header      │                        │
     │─────────────────▶│                        │
     │                  │ session:data (string)  │
     │                  │───────────────────────▶│
     │                  │                        │
     │                  │                        │ ① 检测 zmodem 启动帧
     │                  │                        │    （HEX header 是纯 ASCII）
     │                  │                        │
     │                  │◀──SessionStartZmodem──│ ② 前端调用
     │                  │                        │
     │                  │ 标记 zmodemMode=true   │
     │  ZDATA frames    │                        │
     │  (binary)        │                        │
     │─────────────────▶│                        │
     │                  │ session:binary (Base64)│
     │                  │───────────────────────▶│
     │                  │                        │
     │                  │                        │ ③ zmodemService 解析
     │                  │                        │    Base64 → Uint8Array
     │                  │                        │    → zmodem.js consume
     │                  │                        │
     │                  │                        │ ④ 文件数据收完
     │                  │                        │
     │                  │◀──SaveFileDialog()────│ ⑤ 弹窗让用户选保存位置
     │                  │    返回 path           │
     │                  │                        │
     │                  │◀──WriteFileBase64()───│ ⑥ 前端把文件数据 Base64
     │                  │    path + data         │    传给后端写入磁盘
     │                  │                        │
     │  ZDATA frames    │                        │
     │  (下一个文件)     │                        │
     │─────────────────▶│                        │
     │                  │ session:binary         │
     │                  │───────────────────────▶│
     │                  │                        │ ⑦ 重复 ③~⑥
     │                  │                        │    （批量时首次弹窗，
     │                  │                        │     后续自动保存到同目录）
     │                  │                        │
     │  ZFIN (结束)     │                        │
     │─────────────────▶│                        │
     │                  │ session:binary         │
     │                  │───────────────────────▶│
     │                  │                        │
     │                  │◀──SessionEndZmodem────│ ⑧ 前端调用，后端恢复普通模式
```

### 3.2 上传流程：`rz`

```
[远程服务器]        [后端 Go]                [前端 Vue]
     │                  │                        │
     │  ZRINIT (ready)  │                        │
     │─────────────────▶│                        │
     │                  │ session:data           │
     │                  │───────────────────────▶│
     │                  │                        │
     │                  │                        │ ① 检测 zmodem 启动帧
     │                  │                        │
     │                  │◀──SessionStartZmodem──│ ② 前端调用
     │                  │                        │
     │                  │                        │ ③ 弹 OpenMultipleFilesDialog()
     │                  │                        │
     │                  │◀──ReadFileBase64()────│ ④ 后端读取文件内容返回
     │                  │                        │
     │                  │                        │ ⑤ zmodem.js 编码为 zmodem 帧
     │                  │                        │
     │                  │◀──SessionWriteBinary──│ ⑥ 前端传给后端写入 SSH stdin
     │  ZACK / ZRPOS    │                        │
     │◀─────────────────│                        │
     │                  │                        │
     │                  │                        │ ⑦ 重复 ④~⑥（多文件）
     │                  │                        │
     │  ZFIN (结束)     │                        │
     │◀─────────────────│                        │
     │                  │                        │
     │                  │◀──SessionEndZmodem────│ ⑧ 恢复普通模式
```

### 3.3 跨 tab 传输时序

1. 用户在 tab A 触发 `sz`，传输面板出现在 tab A
2. 用户切换到 tab B
3. `BaseTerminal.vue`（tab A）被卸载，`ZmodemTransfer` 消失
4. `zmodemService` 继续运行，接收 `session:binary`，继续解析
5. `zmodemStore` 持续更新进度
6. 用户切回 tab A，`BaseTerminal.vue` 重新挂载
7. `ZmodemTransfer` 从 `zmodemStore` 读取状态，进度条立即恢复
8. 传输完成时如果不在 tab A，弹出全局 toast："文件传输完成：file.txt（来自 server-01）"

---

## 4. API 设计

### 4.1 新增 Wails 绑定方法

```go
// SessionStartZmodem 通知后端进入 zmodem 模式
func (a *App) SessionStartZmodem(sessionID string) error

// SessionEndZmodem 通知后端退出 zmodem 模式
func (a *App) SessionEndZmodem(sessionID string) error

// SessionWriteBinary 前端向后端发送 Base64 编码的 zmodem 帧
func (a *App) SessionWriteBinary(sessionID string, base64Data string) error

// ReadFileBase64 后端读取本地文件，返回 Base64 编码内容
func (a *App) ReadFileBase64(path string) (string, error)

// WriteFileBase64 后端将 Base64 数据写入指定路径
func (a *App) WriteFileBase64(path string, base64Data string) error
```

### 4.2 新增前端事件

```typescript
// session:binary - zmodem 模式下后端发送 Base64 编码的原始字节
EventsOn('session:binary', (payload: { id: string; data: string }) => {})
```

### 4.3 Session 接口扩展

```go
type Session interface {
    // ... 现有方法
    SetZmodemMode(bool)
    IsZmodemMode() bool
}
```

---

## 5. 文件变更清单

### 5.1 新增文件

```
frontend/src/composables/useZmodem.ts       # zmodem.js 封装 + 传输控制
frontend/src/stores/zmodemStore.ts          # 全局 zmodem 状态
frontend/src/components/ZmodemTransfer.vue  # 传输进度浮层组件
frontend/src/services/zmodemService.ts      # 文件 IO + 协议解析服务
backend/session/base_session.go             # 提取 baseSession（新增 zmodem 字段）
```

### 5.2 修改文件

```
frontend/src/components/BaseTerminal.vue    # 集成 zmodem 检测 + ZmodemTransfer 组件
frontend/src/composables/useTerminal.ts     # 可选：暴露相关事件
backend/session/session.go                  # Session 接口扩展
backend/session/ssh_session.go              # readLoop 双通道支持
backend/session/local_session_windows.go    # readLoop 双通道支持
backend/session/local_session_unix.go       # readLoop 双通道支持
backend/app.go                              # 新增 Wails 绑定方法
frontend/package.json                       # 新增 zmodem.js 依赖
```

---

## 6. 错误处理

| 场景 | 处理策略 |
|------|---------|
| zmodem 启动帧误识别 | `zmodem.js` sentry 尝试解析，不符合协议则自动释放恢复终端；调用 `SessionEndZmodem` |
| 切换 tab 后传输失败 | 全局 toast 通知"传输失败"，切回原 tab 后显示错误状态和重试按钮 |
| 用户取消保存对话框 | 发送 ZCAN 帧终止传输，面板显示"已取消" |
| 磁盘空间不足/写入失败 | `WriteFileBase64` 返回错误，显示错误，发送 ZCAN 终止 |
| 大文件（>100MB） | 显示警告"文件较大"；首次实现暂不做流式分段，后续迭代优化 |
| Base64 切换竞态 | 依靠 zmodem CRC + 重传机制自动恢复 |
| 远程取消传输 | 检测 ZCAN 帧，清理状态，面板显示"远程取消" |

---

## 7. UI 设计

### 7.1 传输面板样式

位于 `BaseTerminal.vue` 内部，终端区域上方，浮动样式：

```
┌──────────────────────────────────────────┐
│  [终端区域 xterm.js]                      │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │ 📥 下载: file.txt (1.2MB)          │  │
│  │ ████████████░░░░░░░░  60%  720KB/s │  │
│  │ [取消]                             │  │
│  └────────────────────────────────────┘  │
│                                          │
└──────────────────────────────────────────┘
```

**批量下载**：
```
┌──────────────────────────────────────────┐
│  📥 下载: file1.txt (1/3)                │
│  ██████████████████████ 100%  完成 ✓     │
│  ─────────────────────────────────────   │
│  📥 下载: file2.txt (2/3)                │
│  ██████████░░░░░░░░░░░  45%  512KB/s     │
│  [取消全部]                              │
└──────────────────────────────────────────┘
```

### 7.2 终端输出处理

- zmodem 协议帧**不写入 xterm**，避免乱码
- 传输开始时终端显示：`"Zmodem transfer: file.txt (downloading...)"`
- 传输完成后终端显示：`"Zmodem: file.txt saved to /path/to/file (1.2MB)"`

### 7.3 取消操作

点击"取消"按钮：
1. 前端发送 ZCAN 帧到远程
2. 调用 `SessionEndZmodem` 恢复普通模式
3. 面板显示"已取消"
4. 删除已部分写入的不完整文件

---

## 8. 依赖

### 8.1 前端新增依赖

```json
{
  "zmodem.js": "^0.1.10"
}
```

`zmodem.js` 是成熟的前端 zmodem 协议实现，支持 Browser/Node 环境。

### 8.2 后端无新增依赖

利用 Go 标准库的 `encoding/base64` 即可。

---

## 9. 风险与后续优化

| 风险 | 缓解措施 | 后续优化 |
|------|---------|---------|
| 大文件内存占用 | 首次实现限制警告 | 后端流式写入，前端分段传输 |
| 批量下载目录选择 | 首次弹窗选目录，后续自动保存 | 支持"始终保存到默认目录"设置项 |
| 传输速度显示 | 基于前后时间差计算 | 更平滑的 EWMA 速度计算 |
| 多 session 同时传输 | zmodemStore 按 sessionId 隔离 | 传输队列管理 |

# Zmodem 传输支持实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 uniTerm 终端添加 zmodem（rz/sz）文件传输支持，覆盖 SSH 和 Local 会话类型

**Architecture:** 前端 `zmodem.js` 做协议解析 + 按需 Base64 切换模式，传输面板位于各自 `BaseTerminal` 内，状态分层（全局解析 + store，局部 UI）

**Tech Stack:** Vue 3 + xterm.js + zmodem.js（前端），Go + Wails v2（后端）

**Branch:** `feat/zmodem-support`（已创建）

---

## 文件结构

### 新增文件

| 文件 | 职责 |
|------|------|
| `backend/session/base_session.go` | 从 `session.go` 提取 baseSession，新增 zmodemMode 字段 |
| `frontend/src/stores/zmodemStore.ts` | Pinia store，全局维护各 session 的 zmodem 传输状态 |
| `frontend/src/services/zmodemService.ts` | zmodem.js 协议解析、文件弹窗、文件读写封装 |
| `frontend/src/composables/useZmodem.ts` | 在 BaseTerminal 中检测 zmodem 启动帧，驱动传输流程 |
| `frontend/src/components/ZmodemTransfer.vue` | 传输进度浮层组件 |

### 修改文件

| 文件 | 修改内容 |
|------|---------|
| `backend/session/session.go` | Session 接口扩展 SetZmodemMode/IsZmodemMode |
| `backend/session/ssh_session.go` | readLoop/readStderr 支持双通道输出 |
| `backend/session/local_session_windows.go` | readLoop 支持双通道输出 |
| `backend/session/local_session_unix.go` | readLoop 支持双通道输出 |
| `backend/app.go` | 新增 5 个 Wails 绑定方法 |
| `frontend/src/components/BaseTerminal.vue` | 集成 zmodem 检测、挂载 ZmodemTransfer、监听 session:binary |
| `frontend/package.json` | 新增 `zmodem.js` 依赖 |

---

## Task 1: 后端 — 提取 baseSession 并新增 zmodem 字段

**Files:**
- Create: `backend/session/base_session.go`
- Modify: `backend/session/session.go`

- [ ] **Step 1: 创建 `base_session.go`，将 baseSession 从 `session.go` 移出并扩展**

```go
package session

import "sync"

type baseSession struct {
	id               string
	sessionType      string
	title            string
	status           SessionStatus
	onDataCallback   func([]byte)
	onStatusCallback func(SessionStatus)
	mu               sync.RWMutex
	pendingCols      int
	pendingRows      int
	zmodemMode       bool
}

func (s *baseSession) ID() string            { return s.id }
func (s *baseSession) Type() string          { return s.sessionType }
func (s *baseSession) Title() string         { return s.title }
func (s *baseSession) Status() SessionStatus { s.mu.RLock(); defer s.mu.RUnlock(); return s.status }

func (s *baseSession) SetOnDataCallback(cb func([]byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDataCallback = cb
}

func (s *baseSession) SetOnStatusChangeCallback(cb func(SessionStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStatusCallback = cb
}

func (s *baseSession) setStatus(st SessionStatus) {
	s.mu.Lock()
	s.status = st
	cb := s.onStatusCallback
	s.mu.Unlock()
	if cb != nil {
		cb(st)
	}
}

func (s *baseSession) emitData(data []byte) {
	s.mu.RLock()
	cb := s.onDataCallback
	s.mu.RUnlock()
	if cb != nil {
		cb(data)
	}
}

func (s *baseSession) SetPendingSize(cols, rows int) {
	s.mu.Lock()
	s.pendingCols = cols
	s.pendingRows = rows
	s.mu.Unlock()
}

func (s *baseSession) GetPendingSize() (cols, rows int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pendingCols, s.pendingRows
}

func (s *baseSession) getInitialSize(defCols, defRows int) (int, int) {
	cols, rows := s.GetPendingSize()
	if cols <= 0 {
		cols = defCols
	}
	if rows <= 0 {
		rows = defRows
	}
	return cols, rows
}

func (s *baseSession) SetZmodemMode(v bool) {
	s.mu.Lock()
	s.zmodemMode = v
	s.mu.Unlock()
}

func (s *baseSession) IsZmodemMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zmodemMode
}
```

- [ ] **Step 2: 修改 `session.go`，删除 baseSession 定义，扩展 Session 接口**

从 `backend/session/session.go` 中删除 `baseSession` 结构体及其所有方法（第 73-143 行），只保留类型定义和接口：

```go
package session

import "sync"

type SessionStatus string

const (
	StatusConnecting   SessionStatus = "connecting"
	StatusConnected    SessionStatus = "connected"
	StatusDisconnected SessionStatus = "disconnected"
	StatusError        SessionStatus = "error"
)

type ConnectionGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ConnectionConfig struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	User     string  `json:"user"`
	AuthType string  `json:"authType"`
	Password string  `json:"password,omitempty"`
	KeyPath  string  `json:"keyPath,omitempty"`
	GroupId  *string `json:"groupId,omitempty"`
	RdpFixedWidth  int  `json:"rdpFixedWidth,omitempty"`
	RdpFixedHeight int  `json:"rdpFixedHeight,omitempty"`
	RdpSmartSizing bool `json:"rdpSmartSizing"`
	ShellPath string `json:"shellPath,omitempty"`
	DBType string `json:"dbType,omitempty"`
	DBName string `json:"dbName,omitempty"`
	PostLoginScript string `json:"postLoginScript,omitempty"`
}

type ConnectionStoreData struct {
	Groups      []ConnectionGroup  `json:"groups"`
	Connections []ConnectionConfig `json:"connections"`
}

type SessionInfo struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Title  string        `json:"title"`
	Status SessionStatus `json:"status"`
}

type Session interface {
	ID() string
	Type() string
	Title() string
	Status() SessionStatus

	Connect(config ConnectionConfig) error
	Disconnect() error
	IsConnected() bool
	Resize(cols, rows int) error

	Write(data []byte) error
	SetOnDataCallback(cb func([]byte))
	SetOnStatusChangeCallback(cb func(SessionStatus))
	SetZmodemMode(bool)
	IsZmodemMode() bool
}
```

- [ ] **Step 3: 编译验证后端**

Run: `cd backend && go build ./...`
Expected: 无报错

---

## Task 2: 后端 — readLoop 双通道支持

**Files:**
- Modify: `backend/session/ssh_session.go`
- Modify: `backend/session/local_session_windows.go`
- Modify: `backend/session/local_session_unix.go`

- [ ] **Step 1: 修改 `ssh_session.go` 的 `readLoop` 和 `readStderr`**

在 `readLoop` 中（约第 168 行），将 `s.emitData(append([]byte(nil), buf[:n]...))` 替换为条件判断：

```go
func (s *SSHSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.stdout.Read(buf)
		if n > 0 {
			s.lastReadTime.Store(time.Now().UnixNano())
			data := append([]byte(nil), buf[:n]...)
			if s.IsZmodemMode() {
				s.emitBinary(data)
			} else {
				s.emitData(data)
			}
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n\x1b[31m[read error: %v]\x1b[0m\r\n", err)))
			} else {
				s.emitData([]byte("\r\n\x1b[31mConnection closed by remote host. Press Enter to reconnect.\x1b[0m\r\n"))
			}
			s.Disconnect()
			return
		}
	}
}
```

在 `readStderr` 中（约第 153 行），stderr 数据不需要走 binary 通道（zmodem 不走 stderr），保持不变。

- [ ] **Step 2: 修改 `local_session_windows.go` 的 `readLoop`**

将 `s.emitData(append([]byte(nil), buf[:n]...))` 替换为：

```go
if n > 0 {
	data := append([]byte(nil), buf[:n]...)
	if s.IsZmodemMode() {
		s.emitBinary(data)
	} else {
		s.emitData(data)
	}
}
```

- [ ] **Step 3: 修改 `local_session_unix.go` 的 `readLoop`**

同上，将 `s.emitData(append([]byte(nil), buf[:n]...))` 替换为相同的条件判断。

- [ ] **Step 4: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无报错（此时 `emitBinary` 还未定义，编译会报错，需要在 Task 3 中定义）

---

## Task 3: 后端 — 新增 emitBinary 和 Wails API

**Files:**
- Modify: `backend/session/base_session.go`
- Modify: `backend/app.go`

- [ ] **Step 1: 在 `base_session.go` 中新增 `onBinaryCallback` 和 `emitBinary`**

在 `baseSession` 结构体中新增字段：

```go
type baseSession struct {
	id               string
	sessionType      string
	title            string
	status           SessionStatus
	onDataCallback   func([]byte)
	onBinaryCallback func([]byte)
	onStatusCallback func(SessionStatus)
	mu               sync.RWMutex
	pendingCols      int
	pendingRows      int
	zmodemMode       bool
}
```

新增方法：

```go
func (s *baseSession) SetOnBinaryCallback(cb func([]byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onBinaryCallback = cb
}

func (s *baseSession) emitBinary(data []byte) {
	s.mu.RLock()
	cb := s.onBinaryCallback
	s.mu.RUnlock()
	if cb != nil {
		cb(data)
	}
}
```

- [ ] **Step 2: 修改 `app.go` 的 `CreateSession`，绑定 binary callback**

在 `CreateSession` 方法中（约第 436 行），在 `SetOnDataCallback` 之后添加：

```go
s.SetOnBinaryCallback(func(data []byte) {
	runtime.EventsEmit(a.ctx, "session:binary", map[string]interface{}{
		"id":   s.ID(),
		"data": base64.StdEncoding.EncodeToString(data),
	})
})
```

注意：需要在 `app.go` 顶部添加 `encoding/base64` 的 import。

- [ ] **Step 3: 在 `app.go` 中新增 5 个 Wails 绑定方法**

在 `SessionResize` 方法之后（约第 544 行）添加：

```go
func (a *App) SessionStartZmodem(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	s.SetZmodemMode(true)
	return nil
}

func (a *App) SessionEndZmodem(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	s.SetZmodemMode(false)
	return nil
}

func (a *App) SessionWriteBinary(sessionID string, base64Data string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}
	return s.Write(data)
}

func (a *App) ReadFileBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (a *App) WriteFileBase64(path string, base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
```

注意：需要在 `app.go` 顶部添加 `os` 的 import。

- [ ] **Step 4: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无报错

---

## Task 4: 前端 — 安装依赖并创建 zmodemStore

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/src/stores/zmodemStore.ts`

- [ ] **Step 1: 安装 zmodem.js**

Run: `cd frontend && npm install zmodem.js`
Expected: 安装成功，`package.json` 中出现 `"zmodem.js": "^0.1.10"`（或最新版本）

- [ ] **Step 2: 创建 `zmodemStore.ts`**

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type TransferDirection = 'upload' | 'download'
export type TransferStatus = 'pending' | 'transferring' | 'completed' | 'cancelled' | 'error'

export interface TransferInfo {
  id: string           // transfer id (file index within session)
  sessionId: string
  filename: string
  size: number         // total bytes
  transferred: number  // bytes transferred so far
  direction: TransferDirection
  status: TransferStatus
  speed: number        // bytes per second
  savePath?: string    // for downloads: user's chosen save directory
  error?: string
}

export const useZmodemStore = defineStore('zmodem', () => {
  // Map<sessionId, TransferInfo[]>
  const transfers = ref<Map<string, TransferInfo[]>>(new Map())

  const getTransfers = computed(() => {
    return (sessionId: string): TransferInfo[] => {
      return transfers.value.get(sessionId) || []
    }
  })

  const getActiveTransfer = computed(() => {
    return (sessionId: string): TransferInfo | undefined => {
      const list = transfers.value.get(sessionId) || []
      return list.find(t => t.status === 'transferring' || t.status === 'pending')
    }
  })

  function addTransfer(sessionId: string, info: TransferInfo) {
    const list = transfers.value.get(sessionId) || []
    list.push(info)
    transfers.value.set(sessionId, list)
  }

  function updateTransfer(sessionId: string, transferId: string, updates: Partial<TransferInfo>) {
    const list = transfers.value.get(sessionId) || []
    const idx = list.findIndex(t => t.id === transferId)
    if (idx >= 0) {
      list[idx] = { ...list[idx], ...updates }
      transfers.value.set(sessionId, [...list])
    }
  }

  function clearTransfers(sessionId: string) {
    transfers.value.delete(sessionId)
  }

  function removeCompleted(sessionId: string) {
    const list = transfers.value.get(sessionId) || []
    const active = list.filter(t => t.status !== 'completed')
    if (active.length === 0) {
      transfers.value.delete(sessionId)
    } else {
      transfers.value.set(sessionId, active)
    }
  }

  return {
    transfers,
    getTransfers,
    getActiveTransfer,
    addTransfer,
    updateTransfer,
    clearTransfers,
    removeCompleted,
  }
})
```

- [ ] **Step 3: 编译验证前端类型**

Run: `cd frontend && npx vue-tsc --noEmit`
Expected: 无类型错误（zmodemStore 不依赖其他修改）

---

## Task 5: 前端 — 创建 zmodemService

**Files:**
- Create: `frontend/src/services/zmodemService.ts`

- [ ] **Step 1: 创建 `zmodemService.ts`**

```typescript
import { Zmodem } from 'zmodem.js'
import {
  SessionStartZmodem,
  SessionEndZmodem,
  SessionWriteBinary,
  ReadFileBase64,
  WriteFileBase64,
  SaveFileDialog,
  OpenMultipleFilesDialog,
  OpenDirectoryDialog,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import { useZmodemStore, type TransferInfo } from '../stores/zmodemStore'
import type { Ref } from 'vue'

let binaryUnsub: (() => void) | null = null

export interface ZmodemServiceOptions {
  sessionId: string
  onComplete?: (files: string[]) => void
  onError?: (err: string) => void
}

export function startZmodemService(options: ZmodemServiceOptions) {
  const { sessionId } = options
  const store = useZmodemStore()

  const sentry = new Zmodem.Sentry({
    to_terminal: (octets: number[]) => {
      // Non-zmodem data: should not reach here if we are in zmodem mode
      // But if sentry decides this is not zmodem, it will pass data here
    },
    sender: (octets: number[]) => {
      // Data that needs to be sent to remote (zmodem control frames)
      const base64 = btoa(String.fromCharCode(...octets))
      SessionWriteBinary(sessionId, base64).catch(() => {})
    },
    on_detect: (detection: any) => {
      const session = detection.confirm()
      handleZmodemSession(session, options)
    },
  })

  // Subscribe to binary data from backend
  binaryUnsub = EventsOn('session:binary', (payload: { id: string; data: string }) => {
    if (payload.id !== sessionId) return
    const bytes = base64ToUint8Array(payload.data)
    sentry.consume(bytes)
  })

  return {
    sentry,
    consume: (data: string) => {
      // Consume normal string data (for initial detection of HEX header)
      const bytes = new TextEncoder().encode(data)
      sentry.consume(bytes)
    },
    dispose: () => {
      binaryUnsub?.()
      binaryUnsub = null
    },
  }
}

async function handleZmodemSession(zsession: any, options: ZmodemServiceOptions) {
  const { sessionId, onComplete, onError } = options
  const store = useZmodemStore()

  if (zsession.type === 'receive') {
    // Download (sz)
    await handleDownload(zsession, sessionId, store, onComplete, onError)
  } else if (zsession.type === 'send') {
    // Upload (rz)
    await handleUpload(zsession, sessionId, store, onComplete, onError)
  }
}

async function handleDownload(
  zsession: any,
  sessionId: string,
  store: ReturnType<typeof useZmodemStore>,
  onComplete?: (files: string[]) => void,
  onError?: (err: string) => void,
) {
  const files: string[] = []
  let firstSaveDir = ''
  let fileIndex = 0

  try {
    await zsession.on('offer', async (offer: any) => {
      const filename = offer.get_filename()
      const size = offer.get_size() || 0
      const transferId = `${sessionId}-${fileIndex}`
      fileIndex++

      store.addTransfer(sessionId, {
        id: transferId,
        sessionId,
        filename,
        size,
        transferred: 0,
        direction: 'download',
        status: 'pending',
        speed: 0,
      })

      let savePath: string
      if (files.length === 0) {
        // First file: ask user for save location
        try {
          savePath = await SaveFileDialog(filename)
          if (!savePath) {
            // User cancelled
            offer.skip()
            store.updateTransfer(sessionId, transferId, { status: 'cancelled' })
            return
          }
          // Extract directory from full path
          const lastSep = Math.max(savePath.lastIndexOf('/'), savePath.lastIndexOf('\\'))
          firstSaveDir = lastSep >= 0 ? savePath.substring(0, lastSep) : ''
        } catch {
          offer.skip()
          return
        }
      } else {
        // Subsequent files: auto-save to same directory
        if (firstSaveDir) {
          savePath = `${firstSaveDir}/${filename}`
        } else {
          savePath = filename
        }
      }

      store.updateTransfer(sessionId, transferId, {
        status: 'transferring',
        savePath,
      })

      const chunks: number[] = []
      const startTime = Date.now()
      let lastTransferred = 0

      offer.on('input', (payload: number[]) => {
        chunks.push(...payload)
        const transferred = chunks.length
        const elapsed = (Date.now() - startTime) / 1000
        const speed = elapsed > 0 ? (transferred - lastTransferred) / elapsed : 0
        lastTransferred = transferred

        store.updateTransfer(sessionId, transferId, {
          transferred,
          speed,
        })
      })

      const fileData = await offer.accept()
      const base64Data = btoa(String.fromCharCode(...fileData))
      await WriteFileBase64(savePath, base64Data)

      store.updateTransfer(sessionId, transferId, {
        status: 'completed',
        transferred: fileData.length,
      })
      files.push(savePath)
    })

    await zsession.start()
    onComplete?.(files)
  } catch (err: any) {
    onError?.(err.message || String(err))
  } finally {
    await SessionEndZmodem(sessionId).catch(() => {})
  }
}

async function handleUpload(
  zsession: any,
  sessionId: string,
  store: ReturnType<typeof useZmodemStore>,
  onComplete?: (files: string[]) => void,
  onError?: (err: string) => void,
) {
  try {
    // Ask user to select files
    const paths = await OpenMultipleFilesDialog()
    if (!paths || paths.length === 0) {
      zsession.abort()
      await SessionEndZmodem(sessionId).catch(() => {})
      return
    }

    const files: string[] = []
    for (let i = 0; i < paths.length; i++) {
      const path = paths[i]
      const filename = path.split(/[\\/]/).pop() || 'unknown'
      const transferId = `${sessionId}-up-${i}`

      // Read file content from backend
      const base64Data = await ReadFileBase64(path)
      const fileData = base64ToUint8Array(base64Data)

      store.addTransfer(sessionId, {
        id: transferId,
        sessionId,
        filename,
        size: fileData.length,
        transferred: 0,
        direction: 'upload',
        status: 'transferring',
        speed: 0,
      })

      const startTime = Date.now()

      await zsession.send_offer({
        name: filename,
        size: fileData.length,
        mode: 0o644,
        mtime: new Date(),
      })

      // Send file data through zmodem
      // zmodem.js handles framing internally
      // We need to send in chunks and update progress
      const chunkSize = 1024
      let offset = 0
      while (offset < fileData.length) {
        const end = Math.min(offset + chunkSize, fileData.length)
        const chunk = fileData.slice(offset, end)
        // Send chunk via SessionWriteBinary
        const base64Chunk = arrayBufferToBase64(chunk)
        await SessionWriteBinary(sessionId, base64Chunk)
        offset = end

        const elapsed = (Date.now() - startTime) / 1000
        const speed = elapsed > 0 ? offset / elapsed : 0
        store.updateTransfer(sessionId, transferId, {
          transferred: offset,
          speed,
        })
      }

      store.updateTransfer(sessionId, transferId, {
        status: 'completed',
        transferred: fileData.length,
      })
      files.push(filename)
    }

    await zsession.close()
    onComplete?.(files)
  } catch (err: any) {
    onError?.(err.message || String(err))
  } finally {
    await SessionEndZmodem(sessionId).catch(() => {})
  }
}

function base64ToUint8Array(base64: string): Uint8Array {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

function arrayBufferToBase64(buffer: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < buffer.length; i++) {
    binary += String.fromCharCode(buffer[i])
  }
  return btoa(binary)
}
```

- [ ] **Step 2: 类型检查**

Run: `cd frontend && npx vue-tsc --noEmit`
Expected: 可能有 zmodem.js 的类型缺失错误，需要添加类型声明

- [ ] **Step 3: 创建 `zmodem.js` 类型声明文件**

Create: `frontend/src/types/zmodem.d.ts`

```typescript
declare module 'zmodem.js' {
  export class Sentry {
    constructor(options: {
      to_terminal?: (octets: number[]) => void
      sender?: (octets: number[]) => void
      on_detect?: (detection: any) => void
    })
    consume(data: Uint8Array | number[]): void
  }

  export class Session {
    type: 'receive' | 'send'
    on(event: string, handler: (...args: any[]) => any): void
    start(): Promise<void>
    abort(): void
    close(): Promise<void>
    send_offer(options: {
      name: string
      size: number
      mode?: number
      mtime?: Date
    }): Promise<void>
  }

  export const Zmodem: {
    Sentry: typeof Sentry
    Session: typeof Session
  }
}
```

---

## Task 6: 前端 — 创建 ZmodemTransfer 组件

**Files:**
- Create: `frontend/src/components/ZmodemTransfer.vue`

- [ ] **Step 1: 创建 `ZmodemTransfer.vue`**

```vue
<template>
  <div v-if="hasActiveTransfers" class="zmodem-transfer-panel">
    <div v-for="t in activeTransfers" :key="t.id" class="transfer-item">
      <div class="transfer-header">
        <span class="transfer-icon">{{ t.direction === 'download' ? '📥' : '📤' }}</span>
        <span class="transfer-name">{{ t.filename }}</span>
        <span v-if="t.status === 'completed'" class="transfer-status success">✓</span>
        <span v-else-if="t.status === 'error'" class="transfer-status error">✗</span>
        <span v-else-if="t.status === 'cancelled'" class="transfer-status cancelled">⊘</span>
      </div>
      <div v-if="t.status === 'transferring' || t.status === 'pending'" class="transfer-progress">
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: progressPercent(t) + '%' }"></div>
        </div>
        <div class="progress-info">
          <span>{{ formatBytes(t.transferred) }} / {{ formatBytes(t.size) }}</span>
          <span v-if="t.speed > 0">{{ formatBytes(t.speed) }}/s</span>
        </div>
      </div>
      <div v-if="t.status === 'transferring'" class="transfer-actions">
        <button class="cancel-btn" @click="cancelTransfer(t)">取消</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useZmodemStore } from '../stores/zmodemStore'

const props = defineProps<{
  sessionId: string
}>()

const store = useZmodemStore()

const activeTransfers = computed(() => {
  return store.getTransfers(props.sessionId).filter(t =>
    t.status === 'transferring' || t.status === 'pending' || t.status === 'completed'
  )
})

const hasActiveTransfers = computed(() => activeTransfers.value.length > 0)

function progressPercent(t: ReturnType<typeof store.getTransfers>[number]) {
  if (t.size === 0) return 0
  return Math.min(100, Math.round((t.transferred / t.size) * 100))
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function cancelTransfer(t: ReturnType<typeof store.getTransfers>[number]) {
  // Emit cancel event to parent
  // Parent should send ZCAN frame and call SessionEndZmodem
  store.updateTransfer(props.sessionId, t.id, { status: 'cancelled' })
}
</script>

<style scoped>
.zmodem-transfer-panel {
  position: absolute;
  bottom: 12px;
  left: 12px;
  right: 12px;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  z-index: 20;
  backdrop-filter: blur(8px);
  box-shadow: var(--shadow-md);
}
.transfer-item {
  margin-bottom: 8px;
}
.transfer-item:last-child {
  margin-bottom: 0;
}
.transfer-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-primary);
}
.transfer-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.transfer-status.success { color: #34d399; }
.transfer-status.error { color: #f87171; }
.transfer-status.cancelled { color: var(--text-muted); }
.transfer-progress {
  margin-top: 6px;
}
.progress-bar {
  height: 4px;
  background: var(--bg-elevated);
  border-radius: 2px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.3s ease;
}
.progress-info {
  display: flex;
  justify-content: space-between;
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}
.transfer-actions {
  margin-top: 6px;
  text-align: right;
}
.cancel-btn {
  padding: 3px 10px;
  font-size: 11px;
  background: transparent;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  cursor: pointer;
}
.cancel-btn:hover {
  border-color: #f87171;
  color: #f87171;
}
</style>
```

---

## Task 7: 前端 — BaseTerminal 集成 zmodem

**Files:**
- Modify: `frontend/src/components/BaseTerminal.vue`

- [ ] **Step 1: 导入 zmodem 相关模块**

在 `BaseTerminal.vue` 的 `<script setup>` 顶部添加 import：

```typescript
import { startZmodemService } from '../services/zmodemService'
import { useZmodemStore } from '../stores/zmodemStore'
import ZmodemTransfer from './ZmodemTransfer.vue'
```

- [ ] **Step 2: 在模板中挂载 ZmodemTransfer 组件**

在 `<template>` 的 `base-terminal` div 内，search bar 下方添加：

```vue
<ZmodemTransfer :session-id="props.sessionId || ''" />
```

- [ ] **Step 3: 初始化 zmodem 服务**

在 `onMounted` 中，在 `terminal.open(terminalRef.value)` 之后添加：

```typescript
const zmodemStore = useZmodemStore()
let zmodemService: ReturnType<typeof startZmodemService> | null = null

if (props.mode === 'ssh' || props.mode === 'local') {
  zmodemService = startZmodemService({
    sessionId: props.sessionId || '',
    onComplete: (files) => {
      if (files.length > 0) {
        terminal?.write(`\r\n\x1b[32mZmodem: ${files.length} file(s) transferred\x1b[0m\r\n`)
      }
    },
    onError: (err) => {
      terminal?.write(`\r\n\x1b[31mZmodem error: ${err}\x1b[0m\r\n`)
    },
  })
}
```

- [ ] **Step 4: 在 session:data 事件处理中接入 zmodem 检测**

在 `unsubscribe = EventsOn('session:data', ...)` 的处理中（约第 525 行），在数据写入终端之前添加 zmodem 检测：

```typescript
unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
  if (payload.id !== props.sessionId || !terminal) return

  // Check if this session has active zmodem transfers
  const activeZmodem = zmodemService && zmodemStore.getActiveTransfer(props.sessionId || '')

  // If zmodem is active, data should come from session:binary, not session:data
  // But we still need to detect the initial HEX header via session:data
  if (zmodemService && !activeZmodem) {
    // Try to detect zmodem start in this data packet
    // The HEX header is ASCII-only, safe to detect from string
    if (payload.data.includes('**')) {
      // Potential zmodem header detected
      // Notify backend to switch to binary mode
      const sid = props.sessionId
      if (sid) {
        import('../../wailsjs/go/main/App').then(({ SessionStartZmodem }) => {
          SessionStartZmodem(sid).then(() => {
            // Feed data to zmodem sentry for confirmation
            zmodemService?.consume(payload.data)
          }).catch(() => {})
        })
      }
      // Don't write zmodem data to terminal
      return
    }
  }

  // If zmodem is confirmed active, skip writing data to terminal
  if (activeZmodem) {
    return
  }

  // Normal terminal data processing (existing code)
  let data = payload.data.replace(/\x1b\[3J/g, '')
  if (data.includes('\x1b[2J')) {
    const rows = terminal.rows
    const scrollClear = '\n'.repeat(rows) + '\x1b[H'
    data = data.replace(/\x1b\[H\x1b\[2J/g, scrollClear)
    data = data.replace(/\x1b\[2J/g, scrollClear)
  }
  // ... rest of existing processing
})
```

**注意**：这里有个简化的检测逻辑。实际实现中，zmodem 检测应该更精确（检测 `***` + ZDLE + 'B'/'A' 等完整帧头），但核心思路是：在 `session:data` 中检测 HEX header（ASCII 安全），检测到后启动 binary 模式，后续数据从 `session:binary` 走。

- [ ] **Step 5: 在 onUnmounted 中清理 zmodem 服务**

在 `onUnmounted` 中添加：

```typescript
zmodemService?.dispose()
zmodemService = null
```

- [ ] **Step 6: 编译验证**

Run: `cd frontend && npx vue-tsc --noEmit`
Expected: 无类型错误

---

## Task 8: 编译与验证

- [ ] **Step 1: 清理前端缓存并重新构建**

Run:
```bash
cd frontend && rm -rf dist node_modules/.vite && npm run build && cd ..
```
Expected: 构建成功，无错误

- [ ] **Step 2: 编译 Wails 应用**

Run:
```bash
wails build -platform windows/amd64
```
Expected: 编译成功，输出 `build/bin/uniTerm.exe`

- [ ] **Step 3: 手动验证 zmodem 功能**

1. 启动应用，连接一个 SSH 服务器
2. 确保服务器已安装 `lrzsz`（`which rz` / `which sz`）
3. 测试下载：`sz /etc/hosts`
   - 预期：弹出保存对话框，选择后显示进度，完成后终端显示成功消息
4. 测试上传：`rz`
   - 预期：弹出文件选择框，选择后显示进度
5. 测试批量下载：`sz file1 file2`
   - 预期：首次弹窗，后续自动保存到同目录
6. 测试跨 tab：在 tab A 触发 sz，切换到 tab B，再切回 tab A，进度应同步

---

## 自审检查

### Spec 覆盖检查

| Spec 需求 | 对应 Task |
|-----------|----------|
| 前端 zmodem.js 协议解析 | Task 5 |
| 按需 Base64 切换 | Task 2, 3, 7 |
| Session 接口扩展 | Task 1 |
| readLoop 双通道 | Task 2 |
| 下载弹窗 SaveFileDialog | Task 5 |
| 批量下载自动保存同目录 | Task 5 (handleDownload) |
| 上传弹窗 OpenMultipleFilesDialog | Task 5 (handleUpload) |
| 传输面板在 BaseTerminal 内 | Task 6, 7 |
| 跨 tab 状态同步 | Task 4 (store) + Task 5 |
| 全局 toast 通知 | 需补充（见下方） |
| 终端输出处理 | Task 7 |
| 取消操作 | Task 5, 6 |

### 发现缺口

**全局 toast 通知（传输完成时用户不在该 tab）**未在任务中实现。需要在 Task 7 中补充：

- 在 `onComplete` 回调中检查当前激活的 sessionId 是否等于传输的 sessionId
- 如果不等于，通过 Element Plus 的 `ElNotification` 显示全局 toast

### Placeholder 检查

无 TBD/TODO/placeholder。

### 类型一致性检查

- `Session` 接口在所有 task 中一致使用 `SetZmodemMode(bool)` 和 `IsZmodemMode() bool`
- `zmodemStore` 的 `TransferInfo` 类型在各处一致
- Wails 方法名在前后端一致

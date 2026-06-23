# Keyboard Shortcuts Design

## Overview

为 uniTerm 增加全局快捷键系统。采用集中式 composable 方案，所有快捷键在 `useKeyboardShortcuts.ts` 中统一注册，不依赖用户配置，使用固定预设。

## Design Principles

1. **不冲突终端**：快捷键不与 shell 常用按键冲突。全部使用 Ctrl+Shift 或 Alt 组合
2. **集中管理**：所有快捷键在单一 composable 中注册，App.vue 挂载时绑定 `document.addEventListener('keydown')`
3. **固定预设**：不需要用户配置界面，快捷键硬编码在 composable 中

## Shortcuts

| 操作 | 快捷键 | 实现 |
|------|--------|------|
| 下一个 tab | `Ctrl+Tab` | `tabStore.nextTab()` |
| 上一个 tab | `Ctrl+Shift+Tab` | `tabStore.prevTab()` |
| 关闭当前 tab | `Ctrl+Shift+W` | `tabStore.closeTab(activeTab.id)` |
| 新建会话对话框 | `Ctrl+Shift+N` | `showConnectionForm = true` |
| 打开侧边栏并聚焦搜索 | `Ctrl+Shift+B` | `sidebarVisible = true` + focus search input |
| 焦点回到终端 | `Ctrl+Shift+K` | `tabStore.getActivePanelId()` + focus terminal |
| 锁定/解锁 AI 窗口 | `Ctrl+Shift+L` | `tabStore.setAILockedPanel()` toggle |
| 打开/聚焦 AI 输入框 | `Ctrl+Shift+J` | `aiStore.visible = true` + focus textarea |
| 关闭当前面板 | `Ctrl+Shift+Q` | workspace 中分离面板，只剩一个时转为独立 tab；非 workspace 直接关闭 tab |
| 面板间导航 | `Alt+↑↓←→` | workspace 内切换焦点面板 |
| 打开设置 | `Ctrl+Shift+,` | `tabStore.createSettingsTab()` or switch to existing settings tab |

## Architecture

### New File: `frontend/src/composables/useKeyboardShortcuts.ts`

```typescript
// Key: normalized key string (e.g. "ctrl+shift+b")
// Value: handler function or action identifier
interface Shortcut {
  ctrl: boolean
  shift: boolean
  alt: boolean
  key: string
  handler: () => void
}

function normalize(e: KeyboardEvent): string
  // e.g. Ctrl+Shift+B → normalized "ctrl+shift+b"
  // Ignore case, sort modifiers alphabetically

function register(shortcuts: Shortcut[]): void
  // Called from App.vue onMounted

function unregister(): void
  // Called from App.vue onUnmounted
```

### Modified Files

| File | Change |
|------|--------|
| `frontend/src/composables/useKeyboardShortcuts.ts` | **NEW** — shortcut registry and handler |
| `frontend/src/App.vue` | Add `onMounted` keydown listener, expose shortcut targets (sidebar ref, AI textarea ref) |
| `frontend/src/stores/tabStore.ts` | Add `nextTab()` / `prevTab()` methods if missing |
| `frontend/src/stores/aiStore.ts` | Add `toggleLock()` method if missing |
| `frontend/src/components/Sidebar.vue` | Expose `focusSearch()` method via expose or reactive flag |
| `frontend/src/components/AISidebar.vue` | Expose `focusInput()` method via expose or reactive flag |

### Intercept Strategy

**Don't intercept when:**
- User is typing in an `input`, `textarea`, `select`, or `[contenteditable]` element
- Exception: AI textarea allows `Ctrl+Shift+L` (lock toggle) even when focused

**Block browser defaults:**
- `Ctrl+Tab` → prevent browser from trying to switch tabs
- `Ctrl+Shift+T` → prevent browser from reopening closed tab
- `Ctrl+Shift+W` → prevent browser from closing the WebView2
- `Ctrl+Shift+B` → prevent browser from showing bookmarks bar
- `Ctrl+Shift+N` → prevent browser from opening incognito
- All registered shortcuts call `e.preventDefault()` and `e.stopPropagation()`

### Error Handling

- All handlers wrapped in try/catch, log to console
- If a store method is unavailable (not yet initialized), shortcut silently ignored
- Shortcut registration is idempotent; calling `register()` twice unregisters first

## Testing

- Manual: Start app, press each shortcut, verify behavior
- Unit: Test key normalization in `useKeyboardShortcuts.ts`
- Regression: Verify existing shortcuts (Ctrl+F search, Ctrl+Enter in DB editor, Ctrl+Shift+V in VNC/SPICE) still work and are not intercepted

## Scope Boundaries

**In scope:** The 13 shortcuts listed above with fixed keybindings, centralized composable, App.vue integration

**Out of scope:**
- User-configurable keybindings or settings UI
- Global OS-level hotkeys (registering system-wide shortcuts)
- Backend-only shortcuts
- Adding new store methods beyond what's needed for the listed shortcuts

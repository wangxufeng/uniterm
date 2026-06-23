# Keyboard Shortcuts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 11 global keyboard shortcuts to uniTerm via a centralized composable, avoiding conflicts with terminal shell input.

**Architecture:** Single `useKeyboardShortcuts.ts` composable registers a `document.addEventListener('keydown')` listener in App.vue. All shortcuts use Ctrl+Shift or Alt modifiers. The listener does not intercept when focus is in input/textarea/select elements (except AI lock toggle).

**Tech Stack:** Vue 3 Composition API, Pinia stores, TypeScript

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `frontend/src/composables/useKeyboardShortcuts.ts` | **CREATE** | Shortcut registry, key normalization, global keydown handler |
| `frontend/src/App.vue` | MODIFY | Integrate composable, expose handlers |
| `frontend/src/stores/tabStore.ts` | MODIFY | Add `nextTab()`, `prevTab()` helpers |
| `frontend/src/components/Sidebar.vue` | MODIFY | Add template ref to search input, expose `focusSearch()` |
| `frontend/src/components/AISidebar.vue` | MODIFY | Add template ref to textarea, expose `focusInput()` |

---

### Task 1: Add `nextTab()` and `prevTab()` to tabStore

**Files:**
- Modify: `frontend/src/stores/tabStore.ts`

- [ ] **Step 1: Add nextTab()**

After `setActiveTab` (around line 213), add:

```typescript
function nextTab() {
  const idx = tabState.tabs.findIndex(t => t.id === tabState.activeTabId)
  if (idx < 0) return
  const next = tabState.tabs[(idx + 1) % tabState.tabs.length]
  tabState.activeTabId = next.id
}

function prevTab() {
  const idx = tabState.tabs.findIndex(t => t.id === tabState.activeTabId)
  if (idx < 0) return
  const prev = tabState.tabs[(idx - 1 + tabState.tabs.length) % tabState.tabs.length]
  tabState.activeTabId = prev.id
}
```

- [ ] **Step 2: Export nextTab and prevTab**

In the return object (around line 505), add:

```typescript
nextTab,
prevTab,
```

- [ ] **Step 3: Verify build**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npx vue-tsc --noEmit --project tsconfig.json 2>&1 | head -20
```

---

### Task 2: Add `focusSearch()` ref to Sidebar.vue

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: Add template ref to search input**

Find the `<el-input>` for search (around line 18, has `v-model="searchQuery"`). Add a ref:

```html
<el-input
  ref="searchInputRef"
  v-model="searchQuery"
```

- [ ] **Step 2: Add ref variable and expose**

In the `<script setup>` section, near the other refs (around line 410), add:

```typescript
const searchInputRef = ref<InstanceType<typeof ElInput>>()
```

At the end of `<script setup>`, add defineExpose before the closing `</script>` tag (before `<style>`):

```typescript
defineExpose({ focusSearch })
```

- [ ] **Step 3: Add focusSearch function**

After the searchInputRef declaration, add:

```typescript
function focusSearch() {
  nextTick(() => {
    const el = searchInputRef.value?.$el?.querySelector('input')
    if (el instanceof HTMLInputElement) {
      el.focus()
      el.select()
    }
  })
}
```

Make sure `nextTick` is imported from `vue` (check existing import at top of `<script setup>`).

- [ ] **Step 4: Verify build**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npx vue-tsc --noEmit --project tsconfig.json 2>&1 | grep -i "sidebar" | head -10
```

---

### Task 3: Add `focusInput()` ref to AISidebar.vue

**Files:**
- Modify: `frontend/src/components/AISidebar.vue`

- [ ] **Step 1: Add template ref to chat textarea**

Find the `<el-input type="textarea">` (around line 89, has `v-model="input"`). Add a ref:

```html
<el-input
  ref="chatInputRef"
  v-model="input"
  type="textarea"
```

- [ ] **Step 2: Add ref variable and expose**

In `<script setup>`, find the ref declarations and add:

```typescript
const chatInputRef = ref<InstanceType<typeof ElInput>>()
```

At the end of `<script setup>`, add:

```typescript
defineExpose({ focusInput })
```

- [ ] **Step 3: Add focusInput function**

After chatInputRef:

```typescript
function focusInput() {
  nextTick(() => {
    const el = chatInputRef.value?.$el?.querySelector('textarea')
    if (el instanceof HTMLTextAreaElement) {
      el.focus()
    }
  })
}
```

Verify `nextTick` is in the import from `vue`.

- [ ] **Step 4: Verify build**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npx vue-tsc --noEmit --project tsconfig.json 2>&1 | grep -i "aisidebar" | head -10
```

---

### Task 4: Create `useKeyboardShortcuts.ts` composable

**Files:**
- Create: `frontend/src/composables/useKeyboardShortcuts.ts`

- [ ] **Step 1: Write the composable**

```typescript
interface ShortcutDef {
  ctrl: boolean
  shift: boolean
  alt: boolean
  key: string
  handler: () => void
}

function normalize(e: KeyboardEvent): string {
  const parts: string[] = []
  if (e.ctrlKey || e.metaKey) parts.push('ctrl')
  if (e.shiftKey) parts.push('shift')
  if (e.altKey) parts.push('alt')
  parts.push(e.key.toLowerCase())
  return parts.join('+')
}

function buildKey(s: ShortcutDef): string {
  let k = ''
  if (s.ctrl) k += 'ctrl+'
  if (s.shift) k += 'shift+'
  if (s.alt) k += 'alt+'
  k += s.key.toLowerCase()
  return k
}

export function useKeyboardShortcuts(shortcuts: ShortcutDef[]) {
  const map = new Map<string, () => void>()
  for (const s of shortcuts) {
    map.set(buildKey(s), s.handler)
  }

  function onKeydown(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement)?.tagName
    const isEditing = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
      || (e.target as HTMLElement)?.isContentEditable

    const normalized = normalize(e)
    const handler = map.get(normalized)
    if (!handler) return

    // Ctrl+Shift+L (lock AI) works even when focus is in AI textarea
    if (isEditing && normalized !== 'ctrl+shift+l') return

    e.preventDefault()
    e.stopPropagation()
    handler()
  }

  function register() {
    document.addEventListener('keydown', onKeydown)
  }

  function unregister() {
    document.removeEventListener('keydown', onKeydown)
  }

  return { register, unregister }
}
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npx vue-tsc --noEmit --project tsconfig.json 2>&1 | grep "useKeyboardShortcuts" | head -5
```

---

### Task 5: Wire shortcuts in App.vue

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Add imports and refs**

Add import (near other composable imports, around line 116):

```typescript
import { useKeyboardShortcuts } from './composables/useKeyboardShortcuts'
```

Add template refs for sidebar and AI sidebar (near other refs, around line 198):

```typescript
const sidebarRef = ref<InstanceType<typeof Sidebar>>()
const aiSidebarRef = ref<InstanceType<typeof AISidebar>>()
```

Import the component types for InstanceType — these should be inferred from existing imports.

- [ ] **Step 2: Add ref binding in template**

Bind `sidebarRef` to Sidebar (line 12):

```html
<Sidebar ref="sidebarRef" :visible="sidebarVisible" @toggle="sidebarVisible = !sidebarVisible" ... />
```

Bind `aiSidebarRef` to AISidebar (line 71):

```html
<AISidebar ref="aiSidebarRef" />
```

- [ ] **Step 3: Build shortcuts array and register**

In `onMounted` (around line 282), add after the existing code:

```typescript
const shortcuts = [
  // Tab navigation
  { ctrl: true, shift: false, alt: false, key: 'tab', handler: () => tabStore.nextTab() },
  { ctrl: true, shift: true, alt: false, key: 'tab', handler: () => tabStore.prevTab() },
  { ctrl: true, shift: true, alt: false, key: 'w', handler: () => {
    const t = tabStore.activeTab
    if (t) closeTab(t.id)
  }},
  // Connection
  { ctrl: true, shift: true, alt: false, key: 'n', handler: () => { showConnectionForm.value = true }},
  // Sidebar
  { ctrl: true, shift: true, alt: false, key: 'b', handler: () => {
    sidebarVisible.value = true
    nextTick(() => sidebarRef.value?.focusSearch())
  }},
  // AI
  { ctrl: true, shift: true, alt: false, key: 'j', handler: () => {
    aiStore.visible = true
    nextTick(() => aiSidebarRef.value?.focusInput())
  }},
  { ctrl: true, shift: true, alt: false, key: 'k', handler: () => {
    // Focus back to active terminal
    const t = tabStore.activeTab
    if (!t) return
    if (t.type === 'workspace') {
      const panelId = t.activePanelId || t.panelIds[0]
      if (panelId) focusPanelTerminal(panelId)
    } else if (t.type === 'terminal') {
      focusPanelTerminal(t.panelId)
    }
  }},
  { ctrl: true, shift: true, alt: false, key: 'l', handler: () => {
    const t = tabStore.activeTab
    if (!t) return
    let panelId: string | null = null
    if (t.type === 'workspace') {
      panelId = t.activePanelId || t.panelIds[0] || null
    } else if (t.type === 'terminal') {
      panelId = t.panelId
    }
    if (panelId) onToggleAiLock(panelId)
  }},
  // Panel
  { ctrl: true, shift: true, alt: false, key: 'q', handler: () => {
    const t = tabStore.activeTab
    if (!t) return
    if (t.type === 'workspace' && t.panelIds.length > 1) {
      const panelId = t.activePanelId || t.panelIds[t.panelIds.length - 1]
      tabStore.removePanelFromWorkspaceTab(t.id, panelId)
    } else if (t.type === 'workspace' && t.panelIds.length === 1) {
      const panelId = t.panelIds[0]
      tabStore.removePanelFromWorkspaceTab(t.id, panelId)
    } else {
      closeTab(t.id)
    }
  }},
  // Panel navigation
  { ctrl: false, shift: false, alt: true, key: 'arrowleft', handler: () => navigatePanel('left') },
  { ctrl: false, shift: false, alt: true, key: 'arrowright', handler: () => navigatePanel('right') },
  { ctrl: false, shift: false, alt: true, key: 'arrowup', handler: () => navigatePanel('up') },
  { ctrl: false, shift: false, alt: true, key: 'arrowdown', handler: () => navigatePanel('down') },
  // Settings
  { ctrl: true, shift: true, alt: false, key: ',', handler: () => openSettings() },
]

const { register, unregister } = useKeyboardShortcuts(shortcuts)
register()
```

- [ ] **Step 5: Unregister in onUnmounted**

In the `onUnmounted` callback (around line 324), add at the end:

```typescript
unregister()
```

Add after the shortcuts definition (before the closing of onMounted):

```typescript
function focusPanelTerminal(panelId: string) {
  nextTick(() => {
    const el = document.querySelector(`[data-panel-id="${panelId}"] .xterm-helper-textarea`)
    if (el instanceof HTMLTextAreaElement) {
      el.focus()
    }
  })
}

function navigatePanel(direction: 'left' | 'right' | 'up' | 'down') {
  const t = tabStore.activeTab
  if (!t || t.type !== 'workspace') return
  const panels = t.panelIds
  if (panels.length <= 1) return
  const current = t.activePanelId || panels[0]
  const idx = panels.indexOf(current)
  if (idx < 0) return

  // Simple linear navigation: left/up = prev, right/down = next
  let nextIdx: number
  if (direction === 'left' || direction === 'up') {
    nextIdx = (idx - 1 + panels.length) % panels.length
  } else {
    nextIdx = (idx + 1) % panels.length
  }
  tabStore.setActivePanel(t.id, panels[nextIdx])
}
```

---

### Task 6: Full build and smoke test

- [ ] **Step 1: Build frontend + Wails**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && rm -rf dist node_modules/.vite && npm run build && cd .. && wails build -platform windows/amd64
```

- [ ] **Step 2: Manual smoke test checklist**

Launch `build/bin/uniTerm.exe` and verify each shortcut:

| Shortcut | Expected |
|----------|----------|
| Ctrl+Tab | Switch to next tab |
| Ctrl+Shift+Tab | Switch to previous tab |
| Ctrl+Shift+W | Close current tab |
| Ctrl+Shift+N | Open connection dialog |
| Ctrl+Shift+B | Open sidebar + focus search |
| Ctrl+Shift+J | Open AI sidebar + focus input |
| Ctrl+Shift+K | Focus back to active terminal |
| Ctrl+Shift+L | Lock/unlock AI panel |
| Ctrl+Shift+Q | Close current panel (workspace) or tab |
| Alt+←→ | Navigate workspace panels |
| Ctrl+Shift+, | Open settings tab |

Also verify:
- Typing Ctrl+C in terminal still works (sends SIGINT)
- Typing Ctrl+W in bash/zsh still deletes word
- Ctrl+F in terminal still opens search bar
- Typing in input fields (connection form, etc.) does not trigger shortcuts
- Ctrl+Shift+L in AI textarea still toggles lock

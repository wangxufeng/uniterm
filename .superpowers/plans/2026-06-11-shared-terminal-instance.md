# Shared Terminal Instance Refactor Implementation Plan (v2)

> **Status:** Revised — fixed 3 critical gaps in original plan.

## Motivation

When a terminal tab is dragged into a workspace panel (or vice versa), the terminal instance is destroyed and recreated. This causes:
1. **Scrollback buffer loss** — the entire terminal history is gone
2. **Garbage on restore** — restoring from `sessionStore.getData()` may contain zmodem binary residue and CSI escape sequences
3. **Visual flicker** — the user sees a clear-and-refill cycle

**Goal:** Share the same xterm.js `Terminal` instance across component mounts. When drag-and-drop happens, only the DOM element moves — the terminal instance survives.

---

## Design

### TerminalManager owns the terminal lifecycle

```
Map<sessionId, ManagedTerminal>
                    │
          ┌─────────┼─────────┐
          │         │         │
     Terminal    FitAddon   SearchAddon
     (shared)    (shared)   (shared)
          │
     refs: Set<string>   ← reference count per component
     disposeTimer        ← delayed cleanup
```

**Delayed dispose:** When `refs.size` drops to 0, the terminal is NOT immediately disposed. A 500ms timer starts. If `acquireTerminal` is called within that window, the timer is cancelled and the terminal is reused. This solves the Vue lifecycle race (old component unmounts before new component mounts).

### BaseTerminal owns per-component resources

| Resource | Lifecycle | Owner |
|----------|-----------|-------|
| `terminal.onData()` | Per mount/unmount → **must dispose** | BaseTerminal |
| `terminal.attachCustomKeyEventHandler()` | Per mount/unmount → **must dispose** | BaseTerminal |
| `WebLinksAddon` (custom callbacks) | Per mount/unmount | BaseTerminal |
| `EventsOn('session:data')` | Per mount/unmount → unsubscribe | BaseTerminal |
| `ResizeObserver` / `IntersectionObserver` | Per mount/unmount | BaseTerminal |
| Terminal instance | Session lifetime | TerminalManager |
| FitAddon / SearchAddon / Unicode11Addon | Session lifetime | TerminalManager |

### WebLinksAddon stays in BaseTerminal

WebLinksAddon has custom callbacks (Ctrl+Click to open, hover tooltip). These are DOM-level behaviors tied to the component, not shared state. Keep it in BaseTerminal, created after acquiring the terminal from the manager.

---

## Task 1: Create TerminalManager Service

**File:** `frontend/src/services/terminalManager.ts` (new)

### Step 1: Imports and interfaces

```typescript
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { SearchAddon } from '@xterm/addon-search'
import { getXtermTheme } from '../composables/useTerminal'

export interface TerminalOptions {
  fontSize?: number
  fontFamily?: string
  themeName?: string
  scrollback?: number
}

export interface ManagedTerminal {
  terminal: Terminal
  fitAddon: FitAddon
  searchAddon: SearchAddon
  unicodeAddon: Unicode11Addon
  container: HTMLElement | null
  refs: Set<string>
  options: TerminalOptions
  disposeTimer: ReturnType<typeof setTimeout> | null
}

const terminals = new Map<string, ManagedTerminal>()
```

Note: `WebLinksAddon` is intentionally NOT in the manager — it has per-component callbacks.

### Step 2: Implement `acquireTerminal`

- If a dispose timer is pending, cancel it and reuse the existing managed terminal
- On first creation, set up terminal + shared addons (FitAddon, SearchAddon, Unicode11Addon)
- Add ref and return the terminal

```typescript
export function acquireTerminal(
  sessionId: string,
  ref: string,
  options: TerminalOptions
): Terminal {
  let managed = terminals.get(sessionId)

  if (managed) {
    // Cancel any pending disposal — terminal is still needed
    if (managed.disposeTimer) {
      clearTimeout(managed.disposeTimer)
      managed.disposeTimer = null
    }
  } else {
    const terminal = new Terminal({
      fontSize: options.fontSize ?? 13,
      fontFamily: options.fontFamily ?? 'Consolas, "Courier New", monospace',
      theme: getXtermTheme(options.themeName ?? 'dark'),
      cursorBlink: true,
      rightClickSelectsWord: false,
      scrollback: options.scrollback ?? 2500,
      allowProposedApi: true,
    })

    const fitAddon = new FitAddon()
    const searchAddon = new SearchAddon()
    const unicodeAddon = new Unicode11Addon()

    terminal.loadAddon(fitAddon)
    terminal.loadAddon(searchAddon)
    terminal.loadAddon(unicodeAddon)

    managed = {
      terminal,
      fitAddon,
      searchAddon,
      unicodeAddon,
      container: null,
      refs: new Set(),
      options,
      disposeTimer: null,
    }
    terminals.set(sessionId, managed)
  }

  managed.refs.add(ref)
  return managed.terminal
}
```

### Step 3: Implement `releaseTerminal` — delayed dispose

When refs drop to 0, schedule a 500ms delayed disposal instead of immediate dispose. This handles the Vue lifecycle race during drag-and-drop.

```typescript
export function releaseTerminal(sessionId: string, ref: string): void {
  const managed = terminals.get(sessionId)
  if (!managed) return

  managed.refs.delete(ref)

  if (managed.refs.size === 0) {
    // Delay disposal to survive drag-and-drop lifecycle race.
    // If acquireTerminal is called within 500ms, the timer is cancelled.
    managed.disposeTimer = setTimeout(() => {
      managed.terminal.dispose()
      terminals.delete(sessionId)
    }, 500)
  }
}

export function disposeTerminal(sessionId: string): void {
  const managed = terminals.get(sessionId)
  if (!managed) return
  if (managed.disposeTimer) {
    clearTimeout(managed.disposeTimer)
  }
  managed.terminal.dispose()
  terminals.delete(sessionId)
}
```

### Step 4: Implement `attachTerminal`

```typescript
export function attachTerminal(sessionId: string, container: HTMLElement): void {
  const managed = terminals.get(sessionId)
  if (!managed) return
  if (managed.container === container) return

  managed.container = container

  if (!managed.terminal.element) {
    managed.terminal.open(container)
  } else {
    container.appendChild(managed.terminal.element)
  }

  requestAnimationFrame(() => {
    managed.fitAddon.fit()
  })
}
```

### Step 5: Implement `detachTerminal`

```typescript
export function detachTerminal(sessionId: string): void {
  const managed = terminals.get(sessionId)
  if (!managed || !managed.terminal.element) return
  managed.terminal.element.remove()
  managed.container = null
}
```

### Step 6: Implement helpers

```typescript
export function getTerminal(sessionId: string): Terminal | undefined {
  return terminals.get(sessionId)?.terminal
}

export function getManagedTerminal(sessionId: string): ManagedTerminal | undefined {
  return terminals.get(sessionId)
}
```

---

## Task 2: Refactor BaseTerminal to Use TerminalManager

**File:** `frontend/src/components/BaseTerminal.vue`

### Step 1: Update imports

Add:
```typescript
import {
  acquireTerminal,
  releaseTerminal,
  attachTerminal,
  detachTerminal,
  getManagedTerminal,
} from '../services/terminalManager'
```

Remove these imports (no longer directly constructed in BaseTerminal):
```typescript
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { SearchAddon } from '@xterm/addon-search'
```

Keep:
```typescript
import type { Terminal } from '@xterm/xterm'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { getXtermTheme } from '../composables/useTerminal'
```

### Step 2: Replace local addon variables

Remove `fitAddon` and `searchAddon` variables. Add disposer tracking for per-component listeners:

```typescript
let terminal: Terminal | null = null
let onDataDispose: { dispose(): void } | null = null
let keyHandlerDispose: { dispose(): void } | null = null
```

Remove:
```typescript
let fitAddon: FitAddon | null = null
let searchAddon: SearchAddon | null = null
```

### Step 3: Add helper accessors

```typescript
function getFitAddon() {
  return props.sessionId ? getManagedTerminal(props.sessionId)?.fitAddon : undefined
}

function getSearchAddon() {
  return props.sessionId ? getManagedTerminal(props.sessionId)?.searchAddon : undefined
}
```

### Step 4: Rewrite `getTerminalOptions` to return plain options

```typescript
function getTerminalOptions() {
  const ts = settingsStore.settings.terminal
  return {
    fontSize: ts.fontSize || 13,
    fontFamily: ts.fontFamily || 'Consolas, "Courier New", monospace',
    themeName: ts.theme || 'dark',
    scrollback: ts.maxHistoryLines || 2500,
  }
}
```

### Step 5: Rewrite terminal setup in `onMounted`

Replace the block that does `new Terminal(...)` → `loadAddon(...)` → `terminal.open(...)`:

```typescript
// Acquire shared terminal from manager
const opts = getTerminalOptions()
const refId = props.panelId || `tab-${props.sessionId}`
terminal = acquireTerminal(props.sessionId || '', refId, opts)

// Load WebLinksAddon per-component (has custom callbacks)
let hoverEl: HTMLDivElement | null = null
const webLinksAddon = new WebLinksAddon(
  (event, uri) => {
    if (event.ctrlKey || event.metaKey) {
      BrowserOpenURL(uri)
    }
  },
  {
    hover(event, _text, _location) {
      if (!hoverEl) {
        hoverEl = document.createElement('div')
        hoverEl.className = 'xterm-link-tooltip'
        terminal!.element!.appendChild(hoverEl)
      }
      const rect = terminal!.element!.getBoundingClientRect()
      hoverEl.textContent = 'Ctrl + Click to open'
      hoverEl.style.left = (event.clientX - rect.left + 12) + 'px'
      hoverEl.style.top = (event.clientY - rect.top - 28) + 'px'
      hoverEl.style.display = 'block'
    },
    leave() {
      if (hoverEl) {
        hoverEl.style.display = 'none'
      }
    }
  }
)
terminal.loadAddon(webLinksAddon)

// Attach to DOM
attachTerminal(props.sessionId || '', terminalRef.value)

// Set up search results handler
const managed = getManagedTerminal(props.sessionId || '')
if (managed) {
  managed.searchAddon.onDidChangeResults((e) => {
    searchResultIndex.value = e.resultIndex
    searchResultCount.value = e.resultCount
  })
}
```

### Step 6: Track `onData` disposer

Surround the existing `terminal.onData(...)` call (the large block) with disposer tracking:

```typescript
onDataDispose = terminal.onData((data) => {
  // ... existing onData handler code (unchanged) ...
})
```

### Step 7: Track key handler disposer

Wrap `terminal.attachCustomKeyEventHandler(...)`:

```typescript
keyHandlerDispose = terminal.attachCustomKeyEventHandler((e) => {
  // ... existing key handler code (unchanged) ...
})
```

### Step 8: Update `resize()` to use manager's FitAddon

Replace direct `fitAddon` references with `getFitAddon()`:

```typescript
function resize() {
  if (props.mode === 'ssh' || props.mode === 'local') {
    const sid = props.sessionId
    if (!terminal || !sid) return
    const fitAddon = getFitAddon()
    if (!fitAddon) return
    const el = terminalRef.value
    if (!el) return
    // ... rest unchanged ...
  } else {
    getFitAddon()?.fit()
  }
}
```

### Step 9: Update search functions

Replace direct `searchAddon` references with `getSearchAddon()`:

```typescript
function openSearch() {
  searchVisible.value = true
  nextTick(() => {
    searchInputRef.value?.focus()
    if (searchText.value) {
      searchInputRef.value?.select()
      getSearchAddon()?.findNext(searchText.value, { decorations: searchDecoOptions })
    }
  })
}

function closeSearch() {
  searchVisible.value = false
  searchText.value = ''
  searchResultIndex.value = 0
  searchResultCount.value = 0
  getSearchAddon()?.clearDecorations()
}

function onSearchInput() {
  if (!searchText.value) {
    searchResultIndex.value = 0
    searchResultCount.value = 0
    getSearchAddon()?.clearDecorations()
    return
  }
  getSearchAddon()?.findNext(searchText.value, { incremental: true, decorations: searchDecoOptions })
}

function onSearchNext() {
  if (!searchText.value) return
  getSearchAddon()?.findNext(searchText.value, { decorations: searchDecoOptions })
}

function onSearchPrev() {
  if (!searchText.value) return
  getSearchAddon()?.findPrevious(searchText.value, { decorations: searchDecoOptions })
}
```

### Step 10: Update `onUnmounted`

Replace `terminal?.dispose()` with per-component cleanup + detach + release:

```typescript
onUnmounted(() => {
  resizeObserver?.disconnect()
  intersectionObserver?.disconnect()

  // Dispose per-component listeners BEFORE releasing terminal
  onDataDispose?.dispose()
  onDataDispose = null
  keyHandlerDispose?.dispose()
  keyHandlerDispose = null

  // Detach and release (delayed dispose if no other refs)
  if (props.sessionId) {
    detachTerminal(props.sessionId)
    releaseTerminal(props.sessionId, refId)
  }

  unsubscribe?.()
  statusUnsubscribe?.()
  // ... rest of cleanup (unchanged) ...
})
```

### Step 11: Update `watch sessionId`

When sessionId changes, release old terminal and acquire new one:

```typescript
watch(() => props.sessionId, (newId, oldId) => {
  if (oldId && oldId !== newId) {
    const oldRef = props.panelId || `tab-${oldId}`
    detachTerminal(oldId)
    releaseTerminal(oldId, oldRef)
    disposeZmodemService(oldId)
  }
  if (newId && terminal && (props.mode === 'ssh' || props.mode === 'local')) {
    initZmodemService(newId)
    const newRef = props.panelId || `tab-${newId}`
    terminal = acquireTerminal(newId, newRef, getTerminalOptions())
    if (terminalRef.value) {
      attachTerminal(newId, terminalRef.value)
    }
    const delays = [200, 400, 600, 800, 1000, 1500, 2000]
    delays.forEach((delay) => {
      setTimeout(() => resize(), delay)
    })
  }
})
```

### Step 12: Settings watcher

Adjust to use `getFitAddon()`:

```typescript
watch(() => settingsStore.settings.terminal, (ts) => {
  if (!terminal) return
  if (ts.fontSize) terminal.options.fontSize = ts.fontSize
  if (ts.fontFamily) terminal.options.fontFamily = ts.fontFamily
  if (ts.maxHistoryLines) terminal.options.scrollback = ts.maxHistoryLines
  if (ts.theme) terminal.options.theme = getXtermTheme(ts.theme)
  resize()
}, { deep: true })
```

### Step 13: Keep history restoration for first mount only

The terminal is shared so its buffer persists. But on FIRST mount (new session), the history from sessionStore is still needed as fallback. We can detect this: if `acquireTerminal` created a new terminal (no pre-existing managed instance), restore history.

Actually, simpler approach: keep the existing sessionStore restoration code. If the terminal is reused, writing the same history again would just add duplicate content — not ideal. So wrap it:

```typescript
// Only restore history if this is a fresh terminal (no existing buffer)
const managed = getManagedTerminal(props.sessionId || '')
const isNewTerminal = managed && managed.refs.size === 1  // we just added ourselves
if (isNewTerminal && sid) {
  const history = sanitizeTerminalHistory(sessionStore.getData(sid))
  if (history) {
    const hlOn = settingsStore.settings.terminal.highlightEnabled ?? true
    terminal.write(hlOn ? highlight(history) : history)
  }
}
```

---

## Task 3: Ensure Stable Containers

**Files:** `Panel.vue`, `TerminalTabContent.vue`

Review only — verify that `BaseTerminal` is rendered inside a stable container element that survives across component mounts. The existing `terminalRef` on `.terminal-area` div should be sufficient.

No changes expected unless testing reveals issues.

---

## Task 4: Drag-and-Drop — No Code Changes Expected

The drag-and-drop flow:

1. User drags tab → workspace
2. `tabStore.addPanelToWorkspaceTab()` creates new panel, removes old tab
3. Vue renders new `BaseTerminal` in panel → `onMounted` → `acquireTerminal`
4. Old `BaseTerminal` unmounts → `onUnmounted` → `releaseTerminal`

With delayed dispose (500ms), the timeline is safe regardless of Vue's ordering:

```
Timeline (worst case: unmount before mount):
  0ms:  Old unmounts → release → refs=0 → starts 500ms timer
  ~1ms: New mounts    → acquire → cancels timer → refs=1
 500ms: (timer cancelled, nothing happens)

Timeline (best case: mount before unmount):
  0ms:  New mounts    → acquire → refs=2
  ~1ms: Old unmounts  → release → refs=1
 500ms: (timer never started, nothing happens)
```

---

## Task 5: Build and Verify

```bash
cd frontend && rm -rf dist node_modules/.vite && npm run build
```

Expected: no TypeScript errors.

---

## Self-Review

1. **onData listener leak:** Fixed — `onDataDispose?.dispose()` in `onUnmounted`
2. **release/acquire race:** Fixed — delayed dispose with 500ms timer cancelled on acquire
3. **WebLinksAddon customization:** Fixed — stays in BaseTerminal, not shared
4. **keyHandler accumulation:** Fixed — `keyHandlerDispose?.dispose()` in `onUnmounted`
5. **History restoration:** Safe-guarded — only restored when terminal is freshly created (refs.size === 1)

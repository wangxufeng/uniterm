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
  /** Whether this terminal was newly created (not reused via timer cancellation). */
  isNew: boolean
}

const terminals = new Map<string, ManagedTerminal>()

// Hidden holding containers to keep terminal elements alive when no
// component is actively displaying them. detachTerminal moves elements
// here; attachTerminal picks them up regardless of where they are.
const holding = new Map<string, HTMLDivElement>()
function getHolding(sessionId: string): HTMLDivElement {
  let el = holding.get(sessionId)
  if (!el) {
    el = document.createElement('div')
    el.style.display = 'none'
    holding.set(sessionId, el)
  }
  return el
}

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
    managed.isNew = false
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
      isNew: true,
    }
    terminals.set(sessionId, managed)
  }

  managed.refs.add(ref)
  return managed.terminal
}

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

  requestAnimationFrame(() => managed.fitAddon.fit())
}

export function detachTerminal(sessionId: string, container: HTMLElement): void {
  const managed = terminals.get(sessionId)
  if (!managed) return
  // Move element to a holding container so it survives component destruction.
  // The next attachTerminal picks it up from there.
  if (managed.terminal.element?.parentElement === container) {
    getHolding(sessionId).appendChild(managed.terminal.element)
  }
  if (managed.container === container) {
    managed.container = null
  }
}

export function getTerminal(sessionId: string): Terminal | undefined {
  return terminals.get(sessionId)?.terminal
}

export function getManagedTerminal(sessionId: string): ManagedTerminal | undefined {
  return terminals.get(sessionId)
}

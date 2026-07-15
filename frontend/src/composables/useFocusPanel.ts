// Focus the xterm textarea of a panel, retrying briefly if the DOM
// hasn't caught up (e.g. right after RDPShow's timeout or a re-render).
export function focusPanelTerminal(panelId: string, attempt = 0) {
  if (attempt > 10) return
  const el = document.querySelector(`[data-panel-id="${panelId}"] .xterm-helper-textarea`)
  if (el instanceof HTMLTextAreaElement) {
    el.focus()
  } else {
    setTimeout(() => focusPanelTerminal(panelId, attempt + 1), 100)
  }
}

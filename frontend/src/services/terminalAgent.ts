import { EventsOn } from '../../wailsjs/runtime'
import { SessionWrite } from '../../wailsjs/go/main/App'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'

export interface ExecuteResult {
  output: string
  exitCode: number
  timedOut?: boolean
}

export interface WatchResult {
  output: string
  timedOut: boolean
}

export function watchOutput(
  sessionId: string,
  marker: string,
  timeoutMs: number
): { promise: Promise<WatchResult>; cleanup: () => void } {
  let timeoutId: ReturnType<typeof setTimeout>
  let unsubscribe: (() => void) | null = null
  let resolved = false
  let output = ''
  let lastScanPos = 0
  let markerSeen = false

  const cleanup = () => {
    clearTimeout(timeoutId)
    unsubscribe?.()
    resolved = true
  }

  const promise = new Promise<WatchResult>((resolve) => {
    unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      if (payload.id !== sessionId || resolved) return

      output += payload.data
      const clean = stripAnsi(output)

      const scanStart = Math.max(0, lastScanPos - marker.length)
      lastScanPos = clean.length
      let searchIdx = scanStart
      while ((searchIdx = clean.indexOf(marker, searchIdx)) !== -1) {
        searchIdx += marker.length
        if (!markerSeen) {
          markerSeen = true
          continue
        }
        cleanup()
        const result = clean.slice(0, searchIdx - marker.length).trim()
        resolve({ output: result, timedOut: false })
        return
      }
    })

    timeoutId = setTimeout(() => {
      cleanup()
      resolve({
        output: stripAnsi(output).trim(),
        timedOut: true,
      })
    }, timeoutMs)
  })

  return { promise, cleanup }
}

export function truncateOutput(
  text: string,
  headLines: number,
  tailLines: number
): string {
  const lines = text.split('\n')
  const total = lines.length
  const threshold = headLines + tailLines
  if (total <= threshold) return text

  const head = lines.slice(0, headLines).join('\n')
  const tail = lines.slice(total - tailLines).join('\n')
  const omitted = total - headLines - tailLines
  return `${head}\n\n─────── [截断: 共 ${total} 行, 已省略 ${omitted} 行] ────────\n调整 head_lines / tail_lines 参数可查看更多内容。\n\n${tail}`
}

export async function executeCommand(
  command: string,
  timeoutMs: number = 60000,
  headLines: number = 50,
  tailLines: number = 150
): Promise<ExecuteResult> {
  const tabStore = useTabStore()
  const panelStore = usePanelStore()

  const lockedPanelId = tabStore.getAILockedPanel()
  let panel = lockedPanelId ? panelStore.getPanel(lockedPanelId) : null

  if (!panel) {
    const activeTab = tabStore.activeTab
    if (activeTab?.type === 'terminal' || activeTab?.type === 'settings') {
      panel = panelStore.getPanel(activeTab.panelId)
    } else if (activeTab?.type === 'workspace' && activeTab.activePanelId) {
      panel = panelStore.getPanel(activeTab.activePanelId)
    }
  }

  if (!panel || !panel.sessionId) {
    throw new Error('No active terminal session')
  }

  const sessionId = panel.sessionId
  const marker = `__AI_DONE_${Date.now()}_${Math.random().toString(36).slice(2, 8)}__`
  const shellPath = panel.config?.shellPath
  const fullCommand = buildCommand(command, marker, shellPath)

  const lowerShell = (shellPath || '').toLowerCase()
  let newline: string
  if (lowerShell.includes('powershell') || lowerShell.includes('pwsh')) {
    newline = '\r'
  } else if (lowerShell.includes('cmd')) {
    newline = '\r\n'
  } else if (lowerShell.includes('bash') || lowerShell.includes('sh')) {
    newline = '\r\n'
  } else {
    newline = '\n'
  }

  await SessionWrite(sessionId, fullCommand + newline)

  const { promise } = watchOutput(sessionId, marker, timeoutMs)
  const result = await promise

  if (result.timedOut) {
    const truncated = truncateOutput(result.output, headLines, tailLines)
    const timeoutSec = Math.round(timeoutMs / 1000)
    return {
      output: truncated
        + `\n\n⚠️ 命令在 ${timeoutSec}s 内未完成，可能仍在运行中。\n`
        + `请勿重复发送相同命令。\n`
        + `• 如果输出显示进度（百分比、文件名滚动等）→ 使用 collect_output 继续等待\n`
        + `• 如果输出显示密码/确认提示 → 使用 send_terminal_key 响应\n`
        + `• 如果命令卡住无响应 → 使用 interrupt_command 取消`,
      exitCode: -1,
      timedOut: true,
    }
  }

  return {
    output: truncateOutput(result.output, headLines, tailLines),
    exitCode: 0,
    timedOut: false,
  }
}

export interface StartResult {
  output: string
  started: boolean
}

export async function startCommand(command: string): Promise<StartResult> {
  const tabStore = useTabStore()
  const panelStore = usePanelStore()

  const lockedPanelId = tabStore.getAILockedPanel()
  let panel = lockedPanelId ? panelStore.getPanel(lockedPanelId) : null

  if (!panel) {
    const activeTab = tabStore.activeTab
    if (activeTab?.type === 'terminal' || activeTab?.type === 'settings') {
      panel = panelStore.getPanel(activeTab.panelId)
    } else if (activeTab?.type === 'workspace' && activeTab.activePanelId) {
      panel = panelStore.getPanel(activeTab.activePanelId)
    }
  }

  if (!panel || !panel.sessionId) {
    throw new Error('No active terminal session')
  }

  const sessionId = panel.sessionId
  const shellPath = panel.config?.shellPath

  const lowerShell = (shellPath || '').toLowerCase()
  let newline: string
  if (lowerShell.includes('powershell') || lowerShell.includes('pwsh')) {
    newline = '\r'
  } else if (lowerShell.includes('cmd')) {
    newline = '\r\n'
  } else if (lowerShell.includes('bash') || lowerShell.includes('sh')) {
    newline = '\r\n'
  } else {
    newline = '\n'
  }

  await SessionWrite(sessionId, command + newline)

  // Collect output for 3 seconds, then return
  return new Promise((resolve) => {
    let output = ''
    const unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      if (payload.id !== sessionId) return
      output += payload.data
    })

    setTimeout(() => {
      unsubscribe()
      resolve({
        output: stripAnsi(output).trim(),
        started: true,
      })
    }, 3000)
  })
}

function buildCommand(command: string, marker: string, shellPath?: string): string {
  const lower = (shellPath || '').toLowerCase()
  if (lower.includes('powershell') || lower.includes('pwsh')) {
    // PowerShell syntax
    return `${command};Write-Output "${marker}"`
  }
  if (lower.includes('cmd')) {
    // CMD syntax
    return `${command}&echo ${marker}`
  }
  // Default: bash / sh / zsh / fish
  return ` ${command};echo "${marker}"`
}

// Simple ANSI stripper for extracting readable text from terminal output
function stripAnsi(str: string): string {
  return str
    .replace(/\x1B\[[0-9;?]*[A-Za-z]/g, '')
    .replace(/\x1B][0-9;]*(?:\x07|\x1B\\)/g, '')
    .replace(/\x1B[()[\]#\^%@>=]/g, '')
    .replace(/\x1B[/!_]./g, '')
    .replace(/\x1B./g, '')
}

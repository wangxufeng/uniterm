# AI Conversation Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable streaming SSE responses, maximize Anthropic prompt-cache hit rate, and implement token-aware context window management.

**Architecture:** Go backend streams Anthropic SSE events via Wails runtime events; frontend restructures system prompt into static (cached) + dynamic (appended to user message) layers; token-budget-based context trimming replaces fixed message count.

**Tech Stack:** Go (backend), Vue 3 + TypeScript (frontend), Wails v2 runtime, Anthropic Messages API with prompt caching

**Design spec:** `.superpowers/specs/2026-06-12-ai-stream-cache-optimization-design.md`

---

## File Structure

| File | Role | Change |
|------|------|--------|
| `app.go` | Backend LLM proxy | Rewrite `ChatCompletion` to stream SSE + emit events; add `CancelChatStream` |
| `frontend/src/services/llm.ts` | API request construction | Add `cache_control` breakpoints, `stream: true`, beta header |
| `frontend/src/services/agent.ts` | Agent loop + system prompt | Split static/dynamic prompt; wire stream events; wire cancel |
| `frontend/src/stores/aiStore.ts` | Conversation state | Add `estimateTokens`; rewrite `conversation` with token budget |
| `frontend/wailsjs/go/main/App.js` | Auto-generated Wails binding | Regenerated after Go signature changes |
| `frontend/wailsjs/go/main/App.d.ts` | Auto-generated Wails types | Regenerated after Go signature changes |

---

### Task 1: Go Backend — Streaming SSE + Cancellation

**Files:**
- Modify: `app.go:781-811` (ChatCompletion)
- Modify: `app.go:28-43` (App struct)
- Modify: `app.go:1-20` (imports)
- Auto-generated: `frontend/wailsjs/go/main/App.js`, `App.d.ts`

**Background:** The current `ChatCompletion` reads the full response with `io.ReadAll` before returning. We replace it with SSE streaming that emits Wails events per token while still returning the complete message JSON at the end (backward-compatible return type). The Anthropic streaming API sends `text/event-stream` with `data:` prefixed JSON lines. We also need the `prompt-caching-2024-07-31` beta header for `cache_control` support.

- [ ] **Step 1: Add imports for streaming**

Read `app.go` lines 1-20. Add `bufio`, `encoding/json`, `sync` to imports. `strings` and `bytes` are already imported.

```go
import (
    "bufio"
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "runtime/debug"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
    goruntime "runtime"

    // ... existing wails/log/store/sync imports
)
```

- [ ] **Step 2: Add stream cancellation fields to App struct**

Read `app.go` lines 29-43 (App struct). Add `chatCancel` and `chatCancelMu` fields.

```go
type App struct {
    ctx                  context.Context
    sessionManager       *session.SessionManager
    connectionStore      *store.ConnectionStore
    aiSessionStore       *store.AISessionStore
    settingsStore        *store.SettingsStore
    terminalHistoryStore *store.TerminalHistoryStore
    syncService          *sync.SyncService
    mainHwnd            uintptr
    originalWndProc     uintptr
    wndProcCb           uintptr
    inSizeMove          bool
    webviewDataPath     string
    chatCancel          context.CancelFunc  // active stream cancellation
    chatCancelMu        sync.Mutex          // guards chatCancel
}
```

- [ ] **Step 3: Rewrite ChatCompletion to stream SSE with event emission**

Replace the existing `ChatCompletion` function (lines 781-811) with the streaming version below. The function signature stays the same: `func (a *App) ChatCompletion(apiKey, baseURL, model string, requestJSON string, protocol string) (string, error)`. It returns the full message JSON when streaming completes, maintaining backward compatibility.

```go
// ChatCompletion streams the Anthropic API response via SSE, emitting Wails
// events for each token while collecting the full message. It returns the
// complete message JSON when the stream ends (backward-compatible).
func (a *App) ChatCompletion(apiKey, baseURL, model string, requestJSON string, protocol string) (string, error) {
    // Inject stream: true into the request body
    var reqBody map[string]interface{}
    if err := json.Unmarshal([]byte(requestJSON), &reqBody); err != nil {
        return "", fmt.Errorf("invalid request JSON: %w", err)
    }
    reqBody["stream"] = true

    modifiedJSON, err := json.Marshal(reqBody)
    if err != nil {
        return "", fmt.Errorf("marshal modified request: %w", err)
    }

    url := strings.TrimRight(baseURL, "/") + "/messages"

    ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
    defer cancel()

    // Store cancel for frontend stop
    a.chatCancelMu.Lock()
    a.chatCancel = cancel
    a.chatCancelMu.Unlock()
    defer func() {
        a.chatCancelMu.Lock()
        a.chatCancel = nil
        a.chatCancelMu.Unlock()
    }()

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(modifiedJSON))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
    req.Header.Set("User-Agent", "uniTerm")

    client := &http.Client{Timeout: 0} // no timeout; context handles it
    res, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(res.Body)
        return "", fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
    }

    // Accumulated state from SSE events
    var contentBlocks []map[string]interface{}
    var currentBlock map[string]interface{}
    var messageRole string
    var usage map[string]interface{}
    var currentBlockIndex int = -1

    scanner := bufio.NewScanner(res.Body)
    scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // max 1MB per line

    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        dataStr := line[6:]

        var event map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
            continue
        }

        eventType, _ := event["type"].(string)

        switch eventType {
        case "message_start":
            if msg, ok := event["message"].(map[string]interface{}); ok {
                messageRole, _ = msg["role"].(string)
            }

        case "content_block_start":
            currentBlockIndex++
            if block, ok := event["content_block"].(map[string]interface{}); ok {
                currentBlock = block
                block["index"] = float64(currentBlockIndex)
                runtime.EventsEmit(a.ctx, "ai:block_start", map[string]interface{}{
                    "index":        currentBlockIndex,
                    "content_block": block,
                })
            }

        case "content_block_delta":
            delta, _ := event["delta"].(map[string]interface{})
            deltaType, _ := delta["type"].(string)

            if deltaType == "text_delta" {
                text, _ := delta["text"].(string)
                runtime.EventsEmit(a.ctx, "ai:token", map[string]interface{}{
                    "text":  text,
                    "index": currentBlockIndex,
                })
            }
            if deltaType == "input_json_delta" && currentBlock != nil {
                partial, _ := delta["partial_json"].(string)
                if currentBlock["input"] == nil {
                    currentBlock["input"] = ""
                }
                currentBlock["input"] = currentBlock["input"].(string) + partial
            }

        case "content_block_stop":
            if currentBlock != nil {
                contentBlocks = append(contentBlocks, currentBlock)
                currentBlock = nil
            }

        case "message_delta":
            if u, ok := event["usage"].(map[string]interface{}); ok {
                usage = u
            }
            if stopReason, ok := event["delta"].(map[string]interface{})["stop_reason"].(string); ok {
                runtime.EventsEmit(a.ctx, "ai:done", map[string]interface{}{
                    "message": map[string]interface{}{
                        "role":    messageRole,
                        "content": contentBlocks,
                    },
                    "usage":       usage,
                    "stop_reason": stopReason,
                })
            }

        case "message_stop":
            fullMessage := map[string]interface{}{
                "role":    messageRole,
                "content": contentBlocks,
            }
            resultJSON, err := json.Marshal(fullMessage)
            if err != nil {
                return "", fmt.Errorf("marshal full message: %w", err)
            }
            return string(resultJSON), nil

        case "error":
            errData, _ := event["error"].(map[string]interface{})
            errMsg, _ := errData["message"].(string)
            return "", fmt.Errorf("stream error: %s", errMsg)
        }
    }

    if err := scanner.Err(); err != nil {
        return "", err
    }

    // If we got here, the stream ended without message_stop — return what we have
    if len(contentBlocks) > 0 {
        fullMessage := map[string]interface{}{
            "role":    messageRole,
            "content": contentBlocks,
        }
        resultJSON, _ := json.Marshal(fullMessage)
        return string(resultJSON), nil
    }

    return "", fmt.Errorf("stream ended without message_stop")
}
```

- [ ] **Step 4: Add CancelChatStream method**

Add after the `ChatCompletion` function in `app.go`:

```go
// CancelChatStream cancels the currently active ChatCompletion stream.
// Called from the frontend when the user clicks Stop.
func (a *App) CancelChatStream() {
    a.chatCancelMu.Lock()
    defer a.chatCancelMu.Unlock()
    if a.chatCancel != nil {
        a.chatCancel()
    }
}
```

- [ ] **Step 5: Regenerate Wails bindings**

Run `wails build` (or `wails dev` for dev mode) to regenerate `frontend/wailsjs/go/main/App.js` and `App.d.ts` with the new `CancelChatStream` binding.

```bash
cd frontend && npm run build  # or: wails build
```

- [ ] **Step 6: Verify Go code compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```

Expected: compilation succeeds with no errors.

- [ ] **Step 7: Commit**

```bash
git add app.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "feat(ai): add streaming SSE support and cancel to ChatCompletion"
```

---

### Task 2: Token Estimation Utility

**File:**
- Modify: `frontend/src/stores/aiStore.ts:1-10` (add utility function)
- Modify: `frontend/src/stores/aiStore.ts:8-44` (replace SYSTEM_PROMPT with static-only SYSTEM_RULES)

**Background:** We need a lightweight token estimator without pulling in a tiktoken dependency. The heuristic is: ASCII text ~3.5 chars/token, CJK/non-ASCII ~1.8 chars/token. This is accurate to within ~15% for typical mixed content.

We also replace the old `SYSTEM_PROMPT` constant (which mixed static rules with dynamic shell context) with a pure static `SYSTEM_RULES` constant. This is the single source of truth for the system prompt — `agent.ts` reads it via `store.systemPrompt`.

- [ ] **Step 1: Add estimateTokens utility function**

Add at the top of `aiStore.ts`, before the `SYSTEM_PROMPT` constant (around line 8, after imports):

```typescript
/**
 * Estimate token count for a string using character-based heuristics.
 * ASCII/English: ~3.5 chars per token. CJK/non-ASCII: ~1.8 chars per token.
 * Accurate to within ~15% for typical mixed content.
 */
function estimateTokens(text: string): number {
  let asciiChars = 0
  let nonAsciiChars = 0
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) <= 0x7f) {
      asciiChars++
    } else {
      nonAsciiChars++
    }
  }
  return Math.ceil(asciiChars / 3.5 + nonAsciiChars / 1.8)
}

/**
 * Estimate tokens for an AIMessage, including its content, tool_calls, and
 * serialized _rawApiMsg.
 */
function estimateMessageTokens(msg: AIMessage): number {
  let total = estimateTokens(msg.content)
  if (msg.tool_calls) {
    for (const tc of msg.tool_calls) {
      total += estimateTokens(tc.function.name)
      total += estimateTokens(tc.function.arguments)
    }
  }
  if (msg._rawApiMsg) {
    total += estimateTokens(JSON.stringify(msg._rawApiMsg))
  }
  return total
}
```

- [ ] **Step 2: Replace SYSTEM_PROMPT constant with static-only SYSTEM_RULES**

Replace the existing `SYSTEM_PROMPT` constant (lines 8-44) with the new static `SYSTEM_RULES`. The old constant included dynamic shell context instructions — the new one omits them since dynamic context is now injected into the user message. Read the existing constant first, then replace:

```typescript
/**
 * Static AI system rules — immutable per app version, always cacheable.
 * Dynamic shell/panel context is injected into the latest user message instead.
 */
const SYSTEM_RULES = `You are an AI assistant inside uniTerm, a terminal emulator. You can execute shell commands in the user's active terminal to help them complete tasks.

When you need to run a command, use the execute_command tool. The command will be executed in the active terminal session and you will receive its stdout/stderr output.

CRITICAL RULES:
- You can only send ONE execute_command tool call at a time. Never send multiple tool calls in a single response.
- Always explain what you are about to do before executing commands.
- If a command might be destructive, warn the user.
- Chain multiple commands with && or ; when appropriate.
- If the output is too long, summarize the key findings.
- Commands have a 60-second timeout.
- At the START of EVERY response, read the shell/panel context in the user's message. IGNORE any memory of what the previous shell was.
- The user may switch terminal tabs at any time. Each terminal is an independent environment. ALWAYS reassess before proceeding.
- When the terminal type changes, switch to the NEW shell's command syntax immediately. NEVER mix commands from different shell types.
- Do NOT invoke a different shell executable from within the current terminal. ALWAYS use the native syntax of the CURRENT shell only.

RISK CLASSIFICATION:
Every execute_command call MUST include a "risk" field:
- "read": only inspects/views data, no modifications at all
- "write": modifies or creates data, but not system-destructive
- "dangerous": potentially destructive or system-altering
For chained commands, classify based on the MOST risky operation in the chain.

--- NEGATIVE EXAMPLES (STRICTLY FORBIDDEN) ---
❌ In Git Bash, do NOT run: Get-CimInstance Win32_LogicalDisk
❌ In PowerShell, do NOT run: ls -la /mnt/c/
❌ In CMD, do NOT run: df -h
❌ In Git Bash, do NOT run: powershell.exe -Command "..."
❌ In PowerShell, do NOT run: bash -c "..."
Use ONLY the current shell's native syntax.`
```

Update the `systemPrompt` computed to use the new constant:

```typescript
const systemPrompt = computed(() => SYSTEM_RULES)
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

Expected: no new type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/stores/aiStore.ts
git commit -m "feat(ai): add token estimation utility functions"
```

---

### Task 3: Restructure System Prompt

**File:**
- Modify: `frontend/src/services/agent.ts:70-122` (buildSystemPrompt → split into static + dynamic)
- Modify: `frontend/src/services/agent.ts:124-140` (runAgent, prepend dynamic context to user message)

**Background:** The current `buildSystemPrompt()` concatenates dynamic shell/panel info with static rules into a single string used as the system prompt. This breaks cache because the dynamic part changes every request. We extract the dynamic portion into `buildDynamicContext()` which returns a header prepended to the latest user message. The static `SYSTEM_RULES` in the store (updated in Task 2) is now the single source of truth.

- [ ] **Step 1: Create buildDynamicContext() and refactor buildSystemPrompt()**

Replace the existing `buildSystemPrompt()` function (lines 70-122) with the two functions below. The new `buildSystemPrompt()` reads `store.systemPrompt` (which is now the static `SYSTEM_RULES` from Task 2) and appends lightweight shell guidance. `buildDynamicContext()` returns the shell banner + switch notice header for injection into the user message:

```typescript
/**
 * Build the dynamic context header injected into the latest user message.
 * This carries current shell/terminal state WITHOUT polluting the system prompt.
 */
function buildDynamicContext(): string {
  const store = useAIStore()
  const activePanel = getActivePanel()

  if (!activePanel) return ''

  const parts: string[] = []
  const shellPath = activePanel.config?.shellPath

  // Build visible shell banner
  const shellName = shellPath
    ? getShellName(shellPath)
    : (activePanel.type === 'ssh' ? 'SSH (Unix-like)' : 'Unknown')

  const isWindowsShell = !!shellPath && (
    shellPath.toLowerCase().includes('powershell') ||
    shellPath.toLowerCase().includes('pwsh') ||
    shellPath.toLowerCase().includes('cmd')
  )

  const syntaxStyle = isWindowsShell
    ? (shellPath!.toLowerCase().includes('cmd')
      ? 'Windows CMD (dir, cd, type, wmic)'
      : 'Windows PowerShell (cmdlets like Get-ChildItem, Set-Location)')
    : 'Unix (ls, cat, grep, df, find)'

  parts.push(`========================================`)
  parts.push(`CURRENT SHELL: ${shellName}`)
  parts.push(`SYNTAX: ${syntaxStyle}`)
  parts.push(`PANEL: ${activePanel.title} (id: ${activePanel.id})`)
  parts.push(`========================================`)

  if (activePanel.type === 'ssh' && activePanel.config) {
    parts.push(`Connected to: ${activePanel.config.user}@${activePanel.config.host}:${activePanel.config.port}`)
  }

  // Detect terminal switch and inject explicit notice
  const lastCtx = store.lastPanelContext
  if (lastCtx && lastCtx.panelId !== activePanel.id) {
    const prev = lastCtx.shellPath ? getShellName(lastCtx.shellPath) : 'another terminal'
    const curr = shellPath ? getShellName(shellPath) : (activePanel.type === 'ssh' ? 'SSH' : 'local terminal')
    parts.push(`\n【TERMINAL SWITCHED】The user has switched from "${prev}" to "${curr}". This is a DIFFERENT terminal environment. Reassess the environment from scratch.`)
  }

  return parts.join('\n')
}

/**
 * Build the system prompt: static rules from store + lightweight shell guidance.
 * The dynamic context (shell banner, switch notices) goes into the user message.
 */
function buildSystemPrompt(): string {
  const store = useAIStore()
  const activePanel = getActivePanel()
  const shellPath = activePanel?.config?.shellPath
  const isWindowsShell = !!shellPath && (
    shellPath.toLowerCase().includes('powershell') ||
    shellPath.toLowerCase().includes('pwsh') ||
    shellPath.toLowerCase().includes('cmd')
  )
  return store.systemPrompt + getShellGuidance(shellPath, isWindowsShell)
}
```

Remove the old shell-specific suffix code from `buildSystemPrompt()` (lines 100-121: the `isWindowsShell` check, and the "IMPORTANT:" prefix concatenation that was in the old code). These are now handled by `getShellGuidance()`.

- [ ] **Step 2: Inject dynamic context into user message in runAgent()**

In `runAgent()`, modify the place where user messages are added (around line 163-169). Prepend the dynamic context to the user's input:

```typescript
// In runAgent(), replace the user message addition (around line 163-169):
if (userInput) {
  const dynamicCtx = buildDynamicContext()
  const fullContent = dynamicCtx ? dynamicCtx + '\n\n' + userInput : userInput
  store.addMessage({
    id: `msg-${Date.now()}`,
    role: 'user',
    content: fullContent
  })
}
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

Expected: no new type errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/services/agent.ts
git commit -m "feat(ai): restructure system prompt — static rules constant, dynamic context in user message"
```

---

### Task 4: Cache Control Breakpoints in API Layer

**File:**
- Modify: `frontend/src/services/llm.ts:32-97` (chat function)
- Modify: `frontend/src/services/llm.ts:99-119` (AVAILABLE_TOOLS, add cache_control)

**Background:** The Anthropic API supports `cache_control: {type: "ephemeral"}` on content blocks within system and tools. We add breakpoints at: (1) system text block, (2) last tool definition. The stable message prefix breakpoint is added in Task 6 (aiStore conversation computed). Requires the `anthropic-beta: prompt-caching-2024-07-31` header.

- [ ] **Step 1: Add cache_control to system blocks and tools**

Replace the `chat()` function signature and body (lines 32-97) with the version below. The key changes: system is now an array of content blocks (not a string), tools have cache_control on the last one, and the request includes `stream: true`.

```typescript
export async function chat(options: ChatOptions): Promise<void> {
  const settingsStore = useSettingsStore()
  const activeModel = settingsStore.activeModel

  const apiKey = activeModel?.apiKey || ''
  const baseURL = activeModel?.baseURL || ''
  const model = activeModel?.model || ''

  if (!apiKey) throw new Error('API key not configured')

  // Format system as content block array with cache_control on the last block
  const systemBlock: Record<string, unknown> = {
    type: 'text',
    text: options.system,
    cache_control: { type: 'ephemeral' }
  }

  // Attach cache_control to the last tool definition
  const tools = options.tools && options.tools.length > 0
    ? options.tools.map((t, i) =>
        i === options.tools.length - 1
          ? { ...t, cache_control: { type: 'ephemeral' } }
          : t
      )
    : options.tools

  const requestBody: Record<string, unknown> = {
    model,
    max_tokens: 4096,
    system: [systemBlock],
    messages: options.messages,
    tools,
  }

  const requestJSON = JSON.stringify(requestBody)

  let responseText: string
  try {
    responseText = await ChatCompletion(apiKey, baseURL, model, requestJSON, 'anthropic')
  } catch (e: any) {
    throw new Error(formatAPIError(e?.message || String(e)))
  }

  // Parse the full message returned after stream completes
  let json: any
  try {
    json = JSON.parse(responseText)
  } catch (e: any) {
    throw new Error(`Failed to parse LLM response: ${e.message}`)
  }

  if (json.error) {
    const errMsg = json.error.message || JSON.stringify(json.error)
    throw new Error(`LLM API error: ${errMsg}`)
  }

  const rawContent = json.content
  if (!Array.isArray(rawContent)) {
    throw new Error('Unexpected Anthropic response: content is not an array')
  }

  // Store raw message for history preservation
  ;(options as any)._rawApiMsg = {
    role: json.role,
    content: rawContent
  }

  // Dispatch text and tool_use blocks (for non-streaming fallback;
  // normally the stream events handle real-time display)
  for (const block of rawContent) {
    switch (block.type) {
      case 'text':
        options.onChunk?.(block.text || '')
        break
      case 'tool_use':
        options.onToolUse?.({
          id: block.id,
          name: block.name,
          input: block.input || {}
        })
        break
    }
  }
}
```

- [ ] **Step 2: Update AVAILABLE_TOOLS with cache_control on last tool**

Since there's only one tool (`execute_command`), add `cache_control` directly to it:

```typescript
export const AVAILABLE_TOOLS = [
  {
    name: 'execute_command',
    description: 'Execute a shell command in the active terminal session and return its output. You MUST classify every command with a risk level.',
    input_schema: {
      type: 'object',
      properties: {
        command: {
          type: 'string',
          description: 'The shell command to execute. Use syntax appropriate for the current shell (provided in context).'
        },
        risk: {
          type: 'string',
          enum: ['read', 'write', 'dangerous'],
          description: 'The risk level of this command:\n- "read": only inspects/views data, absolutely no modifications\n- "write": modifies or creates data but not system-destructive\n- "dangerous": potentially destructive or system-altering'
        }
      },
      required: ['command', 'risk']
    },
    cache_control: { type: 'ephemeral' }
  }
]
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

Expected: no new type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/llm.ts
git commit -m "feat(ai): add cache_control breakpoints to system and tools"
```

---

### Task 5: Wire Stream Events in Agent Loop

**File:**
- Modify: `frontend/src/services/agent.ts` (runAgent function, add event listeners)
- Modify: `frontend/src/services/agent.ts` (imports)

**Background:** The Go backend now emits `ai:token` events during streaming. The frontend needs to subscribe and update the assistant message in real-time. The existing agent loop's `onChunk` callback is preserved as a fallback (for the final dispatch in `chat()`), but the primary real-time display comes from events.

- [ ] **Step 1: Add imports for event listening**

In `agent.ts`, add `EventsOn` import at the top:

```typescript
import { chat, AVAILABLE_TOOLS } from './llm'
import { executeCommand } from './terminalAgent'
import { useAIStore } from '../stores/aiStore'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'
import { EventsOn } from '../../wailsjs/runtime'
import { CancelChatStream } from '../../wailsjs/go/main/App'
```

- [ ] **Step 2: Register stream event listeners in runAgent**

In `runAgent()`, between the `store.resetStop()` and `store.isRunning = true` lines (around line 154), register one-shot event listeners that update the current assistant message in real-time. Use a cleanup variable to unsubscribe on stop/error:

```typescript
// In runAgent(), after store.resetStop() and store.isRunning = true (around line 155):

store.resetStop()
store.isRunning = true

// Track active listeners for cleanup
let currentAssistantMsg: AIMessage | null = null
let cleanupTokenListener: (() => void) | null = null

// Register stream event listeners (these fire from Go backend SSE events)
cleanupTokenListener = EventsOn('ai:token', (data: any) => {
  if (currentAssistantMsg && data.text) {
    currentAssistantMsg.content += data.text
  }
})
```

- [ ] **Step 3: Wire assistant message to event listener**

In the agent loop, after creating the assistant message (the `store.addMessage({ id, role: 'assistant', content: '' })` call around line 182), assign it to `currentAssistantMsg`:

```typescript
// After creating the assistant message (around line 182-186):
const assistantMsg = store.addMessage({
  id: `msg-${Date.now()}`,
  role: 'assistant',
  content: ''
})
currentAssistantMsg = assistantMsg
```

- [ ] **Step 4: Clean up listeners on stop/error/exit**

Add cleanup at all exit points in `runAgent()`:

```typescript
// Define a cleanup helper at the start of runAgent (after store.isRunning = true):
function cleanupStreamListeners() {
  if (cleanupTokenListener) {
    cleanupTokenListener()
    cleanupTokenListener = null
  }
  currentAssistantMsg = null
}

// Call cleanup at each exit point:
// 1. In the stopRequested check (around line 177-179):
if (store.stopRequested) {
  store.isRunning = false
  cleanupStreamListeners()
  return
}

// 2. In the error catch block (around line 220-222):
store.setDebugInfo(store.conversation, errMsg)
store.isRunning = false
cleanupStreamListeners()
return

// 3. At the normal exit (after tool execution loop, around line 269):
store.isRunning = false
store.doSave()
cleanupStreamListeners()
return
```

- [ ] **Step 5: Wire Stop button to CancelChatStream**

In the `AISidebar.vue` `onStop()` function (around line 358), add a call to `CancelChatStream()` to abort the backend stream:

Read `frontend/src/components/AISidebar.vue` line 358-372. In the `onStop` function:

```typescript
// In AISidebar.vue, add import:
import { CancelChatStream } from '../../wailsjs/go/main/App'

// In onStop(), before aiStore.stop():
async function onStop() {
  if (aiStore.pendingCommand) {
    // ... existing pending command handling
    return
  }
  // Cancel the backend stream first
  try { await CancelChatStream() } catch { /* ignore */ }
  aiStore.stop()
}
```

- [ ] **Step 6: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

Expected: no new type errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/services/agent.ts frontend/src/components/AISidebar.vue
git commit -m "feat(ai): wire stream events for real-time token display and cancel support"
```

---

### Task 6: Token-Aware Context Trimming

**File:**
- Modify: `frontend/src/stores/aiStore.ts:305-450` (conversation computed, replace MAX_CTX_MSGS)

**Background:** The current `conversation` computed uses `MAX_CTX_MSGS = 50` for trimming. We replace this with a token-budget approach: 160K token budget (~80% of Claude's 200K window), walk backwards accumulating estimated tokens, stop when budget exceeded. The stable prefix (everything except the last 3 messages) gets `cache_control` on its last content block. The complex tool_use/tool_result pairing logic is preserved from the current implementation.

- [ ] **Step 1: Define token budget and replace MAX_CTX_MSGS**

In `aiStore.ts`, remove `const MAX_CTX_MSGS = 50` from the `conversation` computed (around line 306) and replace with token budget constants:

```typescript
const conversation = computed(() => {
  // Token budget: 80% of Claude's 200K context window
  const MAX_CONTEXT_TOKENS = 160000
  const UNCACHED_TURNS = 3  // most recent N turns stay uncached

  // Estimate system prompt + tools tokens (cached, counted once)
  const systemTokens = estimateTokens(SYSTEM_RULES)
  const toolsTokens = estimateTokens(JSON.stringify(AVAILABLE_TOOLS))
  let tokenCount = systemTokens + toolsTokens
```

- [ ] **Step 2: Walk backwards, accumulate token estimates**

Replace the `recentMsgs.slice(-MAX_CTX_MSGS)` line (around line 309) with a backwards-walking loop:

```typescript
  // Walk backwards through messages, accumulate token estimates.
  // Stop when we exceed the budget.
  const kept: typeof messages.value = []
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const msg = messages.value[i]
    const msgTokens = estimateMessageTokens(msg)
    if (tokenCount + msgTokens > MAX_CONTEXT_TOKENS) break
    tokenCount += msgTokens
    kept.unshift(msg)
  }

  // Don't start with orphaned tool_result whose matching tool_use was trimmed
  while (kept.length > 0 && kept[0].role === 'tool') {
    kept.shift()
  }

  let recentMsgs = kept
```

- [ ] **Step 3: Add cache_control breakpoint to stable prefix boundary**

After the existing result array building and dedup (the `cleaned`/`deduped` arrays at the end of the computed), add a post-processing step. This is cleaner than weaving cache_control into the existing complex message-filtering loop.

Insert the following code at the end of the `conversation` computed, just before the `return deduped` statement (around line 449):

```typescript
  // Add cache_control breakpoint at the stable prefix boundary.
  // Everything except the last UNCACHED_TURNS messages is cacheable.
  // cache_control goes on the last content block of the boundary message.
  const cacheBoundaryIdx = Math.max(0, deduped.length - UNCACHED_TURNS - 1)
  
  for (let msgIdx = 0; msgIdx < deduped.length; msgIdx++) {
    const msg = deduped[msgIdx]
    const isCacheBoundary = (msgIdx === cacheBoundaryIdx)
    if (!isCacheBoundary) continue
    
    const content = msg.content
    if (Array.isArray(content) && content.length > 0) {
      let lastBlock = content[content.length - 1] as Record<string, unknown>
      // Don't put cache_control on tool_use or tool_result blocks;
      // use the second-to-last text block instead
      if ((lastBlock.type === 'tool_use' || lastBlock.type === 'tool_result') && content.length > 1) {
        const secondLast = content[content.length - 2] as Record<string, unknown>
        secondLast.cache_control = { type: 'ephemeral' }
      } else if (lastBlock.type === 'text') {
        lastBlock.cache_control = { type: 'ephemeral' }
      }
    } else if (typeof msg.content === 'string' && msg.content) {
      // Convert string content to content block array with cache_control
      msg.content = [
        { type: 'text', text: msg.content, cache_control: { type: 'ephemeral' } }
      ]
    }
  }

  return deduped
```

The rest of the existing message-filtering logic (dangling tool_use removal, tool_result pairing, consecutive user merging) remains unchanged.
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

Expected: no new type errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/aiStore.ts
git commit -m "feat(ai): token-aware context trimming with cache_control on stable prefix"
```

---

### Task 7: Build Verification

**Files:**
- No source changes — verify build only.

- [ ] **Step 1: Build Go backend**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build -o /dev/null ./...
```

Expected: build succeeds.

- [ ] **Step 2: Build frontend**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npm run build 2>&1 | tail -10
```

Expected: build succeeds.

- [ ] **Step 3: Verify Wails dev mode starts**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && wails dev 2>&1 &
# Wait ~5s, check for startup errors, then kill
```

Expected: app compiles and starts without errors.

- [ ] **Step 4: Manual smoke test checklist**

Launch dev mode and verify:
1. Open AI sidebar → type a message → response streams token by token (visible incremental display)
2. Click Stop during streaming → stream stops, no errors in log
3. Open browser DevTools → Network → check request headers include `anthropic-beta: prompt-caching-2024-07-31`
4. Check response `usage` fields: `cache_read_input_tokens` > 0 on second+ request
5. Long conversation (20+ turns) → context trimming works, no API errors about context length
6. Switch terminal tabs mid-conversation → shell context updates correctly in next user message

- [ ] **Step 5: Commit any remaining changes**

```bash
git status
# If any auto-generated files changed:
git add frontend/wailsjs/
git commit -m "chore: update Wails bindings after streaming refactor"
```

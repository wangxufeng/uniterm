# AI Terminal Tools Design Spec

**Date:** 2026-06-13
**Branch:** `fix/ai-conversation`
**Status:** Draft

---

## Goal

解决 AI 在终端中执行命令时的两个核心问题：
1. 命令进入交互式等待（sudo 密码、y/n 确认）时，AI 干等 60s 超时然后重复发命令
2. 长任务（npm install、docker build）被 60s 硬超时误判，AI 不知道命令还在跑

方案：不给前端硬编码规则，而是给 AI 一套工具，让 AI **自己决定**等待多久、何时输入密码、何时取消命令。

---

## Tool Index

7 tools, grouped by operation type:

### Command Execution

| Tool | Sends command | Waits for result | Returns |
|------|:---:|:---:|---|
| `execute_command` | Yes | Until marker or timeout | Full or partial output |
| `start_command` | Yes | 3s snapshot only | Startup output |

### Terminal Reading (no input sent)

| Tool | Mechanism | Returns |
|------|-----------|---------|
| `capture_terminal` | Instant snapshot from xterm.js buffer | Last N lines of visible content |
| `collect_output` | Listen to session:data for N seconds | New output accumulated during wait |
| `get_terminal_state` | Query session metadata | `{ pwd, user, shell, cols, rows }` |

### Terminal Control (input only, no wait)

| Tool | What it does |
|------|--------------|
| `send_terminal_key` | Send text or control character to terminal |
| `interrupt_command` | Send Ctrl+C (`\x03`) |

---

## Tool Definitions

### `execute_command`

Execute a shell command and wait for it to complete or timeout.

```
Input:
  command:    string   — Shell command
  risk:       "read" | "write" | "dangerous"
  timeout:    number   — Max wait seconds, default 60, min 5, max 300
  head_lines: number   — Head lines in truncated output, default 50
  tail_lines: number   — Tail lines in truncated output, default 150

Returns:
  output:   string   — Command output (possibly truncated)
  exitCode: number   — 0 on success, -1 on timeout or error
  timedOut: boolean  — true if timeout was reached
```

Truncation: when total lines > `head_lines + tail_lines`, keep only head and tail with a truncation notice showing the omitted line count.

Timeout return format:
```
<output collected so far>

⚠️ Command did not complete within Ns. The command may still be running.
Do NOT re-send the same command.
• If output shows progress → use collect_output to keep waiting.
• If output shows a prompt → use send_terminal_key to respond.
• If command is stuck → use interrupt_command to cancel.
```

### `start_command`

Start a command without waiting for completion. Returns initial output only.

```
Input:
  command: string — Shell command

Returns:
  output:  string — First 3s of output
  started: true
```

Use cases: `npm run dev`, `redis-server`, `python -m http.server`.

### `capture_terminal`

Read terminal visible content from xterm.js buffer. Instant, no waiting.

```
Input:
  head_lines: number — Head lines from buffer, default 0
  tail_lines: number — Tail lines from buffer, default 50

Returns:
  output: string — Requested lines, newest at bottom
```

**Implementation:** Read from `xterm.buffer.active` (normal screen) or `xterm.buffer.normal` (if alternate screen is active). Each line retrieved via `line.translateToString()`.

Use cases: check what's on screen after command completes, verify shell prompt is back, read error messages visible in viewport.

### `collect_output`

Wait and collect new terminal output, without sending anything to the terminal.

```
Input:
  timeout:    number — Wait seconds, default 30, min 5, max 120
  head_lines: number — Head lines, default 50
  tail_lines: number — Tail lines, default 150

Returns:
  output:   string  — Output accumulated during the wait
  timedOut: boolean — true if wait period expired
```

**Implementation:** Uses the same `watchOutput` primitive with a fresh marker queued by writing `echo "__AI_COLLECT_xxx__"`. When the marker appears, the shell prompt is available and collection completes. If marker doesn't appear within timeout, return whatever was collected.

Use case: keep waiting for a long-running command without sending a new command.

### `get_terminal_state`

Query session metadata without running commands.

```
Input: none

Returns:
  pwd:   string — Current working directory
  user:  string — Current user
  shell: string — Shell type (bash, zsh, powershell, cmd, etc.)
  cols:  number — Terminal columns
  rows:  number — Terminal rows
```

**Implementation:** Two approaches:

1. **Preferred (Go backend):** The Go session manager tracks the last known CWD from OSC 7 escape sequences or similar. If available, return cached state.

2. **Fallback:** Return `{ shell, cols, rows }` from panel config (already available in frontend). pwd/user require backend support — for initial release, these can be null with a note that future Go-side work will populate them.

For v1, minimum viable: return shell, cols, rows from panel config. pwd/user left as future enhancement.

### `send_terminal_key`

Send text or a control character to the terminal.

```
Input (one of):
  input:  string                     — Text to send (e.g., password, "y", "n")
  control: "ctrl_c" | "ctrl_d" | "enter" — Control character

Returns:
  output: string — Brief snapshot of output after sending (5s window)
```

**Restrictions:**
- No `ctrl_l` (clear screen) — user must always be able to see AI's actions
- After sending, a short marker echo captures immediate response

### `interrupt_command`

Send Ctrl+C to cancel the currently running command.

```
Input: none

Returns:
  output: string — Output captured after interrupt
```

**Implementation:** Calls same path as `send_terminal_key(undefined, 'ctrl_c')`.

---

## Architecture

### Core Primitive: `watchOutput`

All tools that need to observe terminal output share a single `watchOutput` primitive:

```
watchOutput(sessionId, marker, timeoutMs) → { promise, cleanup }
```

```
promise resolves to:
  { output: string, timedOut: boolean }
```

**Flow:**
1. Subscribe to `session:data` events (filtered by sessionId)
2. Accumulate output in buffer, strip ANSI codes
3. Scan buffer for marker string
4. First occurrence = command echo → skip
5. Second occurrence = marker executed → command complete → resolve
6. Timeout fires → resolve with whatever output was collected + `timedOut: true`
7. Cleanup: clear timeout + unsubscribe from `session:data`

### Tool Composition

```
execute_command(cmd, timeout, head, tail)
  → SessionWrite(cmd + "echo 'MARKER'" + newline)
  → watchOutput(sessionId, marker, timeout)
  → truncate(output, head, tail)
  → return { output, exitCode, timedOut }

start_command(cmd)
  → SessionWrite(cmd + "echo 'MARKER'" + newline)
  → watchOutput(sessionId, marker, 3000)
  → return { output, started: true }

collect_output(timeout, head, tail)
  → SessionWrite("echo 'MARKER'" + newline)   // light marker, queues after running cmd
  → watchOutput(sessionId, marker, timeout)
  → truncate(output, head, tail)
  → return { output, timedOut }

send_terminal_key(input?, control?)
  → resolve to data string
  → SessionWrite(data)
  → SessionWrite("echo 'MARKER'" + newline)
  → watchOutput(sessionId, marker, 5000)
  → return { output }

interrupt_command()
  → same as send_terminal_key(undefined, 'ctrl_c')

capture_terminal(head, tail)
  → direct read from xterm.js buffer
  → no watchOutput, no network call

get_terminal_state()
  → read from panel config (shell, cols, rows)
  → no watchOutput, no network call
```

---

## System Prompt Changes

Update `SYSTEM_RULES` in `aiStore.ts`:

1. Replace hardcoded "60-second timeout" with tool descriptions
2. Add "TIMEOUT GUIDELINES" section with recommended timeout values
3. Add "HANDLING TIMEOUTS" section with decision tree
4. Add "INTERACTIVE PROMPTS" section
5. Add incremental output guidance (collect_output vs capture_terminal)
6. Add prohibition on explicit clear/ls commands (let user see history)
7. Add max_lines usage guidance

### AI Behavior Rules (non-exhaustive)

```
TIMEOUT GUIDELINES:
- 5-10s: quick commands (ls, cat, pwd, whoami)
- 15-30s: moderate commands (grep, find, df, systemctl status)
- 60-120s: build/install tasks (npm install, pip install, apt-get)
- 120-300s: very long tasks (docker build, large git clone, full compilation)

HANDLING TIMEOUTS (decision tree):
1. Read the output, especially the last 10 lines
2. Progress visible (percentages, file names scrolling)? → collect_output
3. Password/prompt visible? → ask user, then send_terminal_key
4. No output or garbage? → interrupt_command, then reassess
5. NEVER re-send execute_command after timeout — use collect_output.

INTERACTIVE PROMPTS:
- When you see a password prompt → ask user (don't guess)
- When you see y/n → use send_terminal_key(input: "y")
- When you see [sudo] password → ask user for sudo password

OUTPUT READING:
- If command returned but shell prompt hasn't appeared, use capture_terminal
- collect_output queues a marker and waits — only works when command is in progress
```

---

## Output Truncation Format

When output exceeds `head_lines + tail_lines`:

```
<head 1>
<head 2>
...
<head N>

─────── [截断: 共 X 行, 已省略 Y 行] ────────
调整 head_lines / tail_lines 参数可查看更多内容。  

<tail 1>
<tail 2>
...
<tail N>
```

When total lines ≤ threshold, output is returned verbatim (no truncation marker).

---

## File Changes

| File | Change |
|------|--------|
| `frontend/src/services/terminalAgent.ts` | Add `watchOutput`, rewrite `executeCommand`, add `startCommand`, `collectOutput`, `sendTerminalKey` |
| `frontend/src/services/llm.ts` | Add 6 new tool definitions to `AVAILABLE_TOOLS`, update `execute_command` schema |
| `frontend/src/services/agent.ts` | Handle tool calls for new tools in `runAgent()`, update `getRisk()` if needed |
| `frontend/src/stores/aiStore.ts` | Rewrite `SYSTEM_RULES` |
| `frontend/src/types/ai.ts` | Add types if needed for tool results |

No Go backend changes in scope (v1 uses frontend-only mechanisms). `get_terminal_state` pwd/user fields deferred to future backend work.

---

## Test Scenarios

| Scenario | Expected AI Behavior |
|----------|---------------------|
| `sudo systemctl restart nginx` | `execute_command` times out → AI sees `[sudo] password` → asks user → `send_terminal_key` |
| `npm install` | `execute_command(timeout=120)` → may timeout → `collect_output(timeout=60)` → complete |
| `ssh user@host` | Times out → AI sees password prompt → asks user |
| `sleep 100` | Times out with no output → `capture_terminal` confirms idle → AI reports to user |
| `cat /var/log/syslog` | Output truncated at 200 lines → truncation marker shown → AI can re-run with larger limits |
| `npm run dev` | `start_command` → returns startup output immediately → AI confirms server started |

---

## Non-Goals (explicitly excluded)

- AI guessing passwords (must ask user)
- `send_terminal_key` with `ctrl_l` (clear screen — user must see history)
- `get_terminal_state` pwd/user via command execution (must be passive, no side effects)
- Executing `clear`/`cls` commands via `execute_command` (enforced in system prompt)
- AI writing to terminal in background (no async/promise-based write tools)

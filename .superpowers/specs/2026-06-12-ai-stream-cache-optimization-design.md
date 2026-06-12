# AI Conversation Optimization: Streaming + Cache + Context Management

**Goal:** Enable true token-by-token streaming responses, maximize Anthropic prompt-cache hit rate, and implement token-aware context window management for stable long conversations.

**Motivation:**
- Current backend reads full response before returning — no real streaming.
- Dynamic content (shell type, panel ID, switch notices) is baked into system prompt, making the entire system block uncacheable.
- Context trimming uses fixed message count (50) with no token awareness, risking context overflow for long tool outputs.

---

## 1. Prompt Structure Reorganization

### Problem

`buildSystemPrompt()` in `agent.ts` concatenates static rules + dynamic shell banner + terminal switch notices into a single system prompt. Since the banner changes per panel and notices vary per turn, the entire system block changes every request — cache hit rate is near zero.

### Solution

Split into three layers with `cache_control` breakpoints:

```
Request body:
┌──────────────────────────────────────────────┐
│ system: [{ text: <static rules>,              │
│            cache_control: {type:"ephemeral"}}]│ ← BP1: static, never changes
├──────────────────────────────────────────────┤
│ messages[0..K-3]: stable conversation prefix  │
│   last block with cache_control               │ ← BP2: stable history tail
├──────────────────────────────────────────────┤
│ messages[K-2..K-1]: recent 2 turns            │ ← uncached
├──────────────────────────────────────────────┤
│ latest user message:                          │
│   [dynamic context header] + user input        │ ← uncached, carries current state
├──────────────────────────────────────────────┤
│ tools: [{...}, {..., cache_control}]           │ ← BP3: static (on last tool)
└──────────────────────────────────────────────┘
```

**Dynamic context** moves from system prompt into the latest user message as a plain-text header:

```
========================================
CURRENT SHELL: Git Bash (C:\Program Files\Git\bin\bash.exe)
PANEL ID: panel-1234567890
========================================
(Below will be full user input)
```

If terminal switch detected, prepend an extra notice line. This keeps the system prompt immutable while still giving the model current terminal context.

### Cache hit expectation

- **System rules** (~3 KB): hit on every request after first
- **Tools** (~1 KB): hit on every request after first
- **Stable history prefix** (N-3 messages): hit when conversation length is stable
- **Estimated savings**: ~70-90% fewer input tokens read on cache hits for multi-turn conversations

---

## 2. Streaming Response

### Problem

`ChatCompletion` in `app.go` uses `io.ReadAll` — the entire response is buffered before returning. The frontend `onChunk` callback fires just once with the full text, so there's no visible streaming.

### Solution

Go backend streams Anthropic SSE events to frontend via Wails runtime events:

```
Go Backend                          Frontend
─────────                          ─────────
POST /messages (stream: true)
  request header:
    x-api-key, anthropic-version, 
    Accept: text/event-stream,
    anthropic-beta: prompt-caching-2024-07-31

  response (text/event-stream):
  event: message_start       ──→ (ignored)
  event: content_block_delta ──→ runtime:EventsEmit("ai:token", {text})
  event: content_block_start ──→ runtime:EventsEmit("ai:tool_use_start", {id, name})
  event: content_block_delta ──→ (accumulated in backend for tool_use args)
  event: content_block_stop  ──→ runtime:EventsEmit("ai:content_block_stop", {index})
  event: message_delta       ──→ runtime:EventsEmit("ai:usage", {input_tokens, 
                                   output_tokens, cache_read_tokens, 
                                   cache_creation_tokens})
  event: message_stop        ──→ runtime:EventsEmit("ai:done", {full_message})
  event: ping                ──→ (heartbeat, keep-alive)
```

### Backend: `ChatCompletion` rewrite

```go
func (a *App) ChatCompletionStream(apiKey, baseURL, model string, 
    requestJSON string, protocol string) error {
    
    // Parse request to add stream: true if not present
    // Remove any frontend-only fields before sending
    
    req, _ := http.NewRequest("POST", url, bodyReader)
    req.Header.Set("Accept", "text/event-stream")
    req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
    
    // Stream with context for cancellation on stop
    ctx, cancel := context.WithCancel(a.ctx)
    req = req.WithContext(ctx)
    
    resp, _ := client.Do(req)
    scanner := bufio.NewScanner(resp.Body)
    
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            data := line[6:]
            var event map[string]interface{}
            json.Unmarshal([]byte(data), &event)
            
            switch event["type"] {
            case "content_block_delta":
                delta := event["delta"].(map[string]interface{})
                runtime.EventsEmit(a.ctx, "ai:token", delta)
            case "message_stop":
                runtime.EventsEmit(a.ctx, "ai:done", event)
            // ... etc
            }
        }
    }
}
```

### Frontend: `agent.ts` stream event listener

```typescript
// Register event listeners once
EventsOn("ai:token", (delta) => {
  if (delta.type === "text_delta") {
    assistantMsg.content += delta.text
  }
})

EventsOn("ai:done", (event) => {
  // Extract full message (tool_use blocks, etc.)
  // Continue agent loop if tool calls present
})
```

- Stop button cancels via context propagation to Go backend
- Timeout: 120s for initial response, 30s idle between events

---

## 3. Token-Aware Context Management

### Problem

Current trimming: `MAX_CTX_MSGS = 50`, purely message-count based. A single tool output could be thousands of tokens. No awareness of actual context window pressure.

### Solution

Replace fixed message count with token-budget-based trimming:

```typescript
// Token estimation (no tiktoken dependency — character-based heuristic)
function estimateTokens(text: string): number {
  // English/ASCII: ~4 chars/token
  // CJK: ~2 chars/token
  let tokens = 0
  for (const ch of text) {
    tokens += (ch.charCodeAt(0) > 0x7f) ? 0.5 : 0.25
  }
  return Math.ceil(tokens)
}

// Context budget: 80% of model's context window
const CONTEXT_BUDGET = 160_000 // Claude 200K * 0.8

function trimConversation(messages: AIMessage[]): Array<Record<string, unknown>> {
  let tokenCount = estimateSystemPrompt() + estimateToolsDef()
  
  // Walk backwards, accumulate tokens
  const kept: AIMessage[] = []
  for (let i = messages.length - 1; i >= 0; i--) {
    const msgTokens = estimateMessageTokens(messages[i])
    if (tokenCount + msgTokens > CONTEXT_BUDGET) break
    tokenCount += msgTokens
    kept.unshift(messages[i])
  }
  
  // Safety: ensure we don't start with orphaned tool_result
  while (kept.length > 0 && kept[0].role === 'tool') {
    kept.shift()
  }
  
  // Insert cache_control breakpoint at stable prefix boundary
  // ...
}
```

### Key parameters

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Context budget | 80% of model window | Reserve 20% for response tokens |
| Min recent turns | 2 | Always keep last 2 user-assistant-tool cycles |
| Tool output max | 8000 chars | Truncate extremely long outputs |
| cache_control BP | N-3 messages | Keep recent 3 turns uncached; cache the rest |

### Anthropic cache_control mechanics

Per [Anthropic docs](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching):
- Minimum cacheable size: 1024 tokens (text) or 1024 tokens (tools)
- Maximum: 4 cache breakpoints per request
- Cache TTL: 5 minutes (reset on each use)
- Billing: cache write = base price × 1.25, cache read = base price × 0.10

We use 3 breakpoints (system + stable prefix + tools), keeping 1 slot for future use. Anthropic allows a maximum of 4 cache breakpoints per request.

---

## 4. Files Changed

### Backend

| File | Change |
|------|--------|
| `app.go` | Replace `ChatCompletion` with `ChatCompletionStream`; add `CancelChatStream`; add `ChatCompletion` fallback (non-stream, for tool-only calls) |
| `app.go` | Add `streamKey` map to track active stream contexts for cancellation |

### Frontend

| File | Change |
|------|--------|
| `src/services/agent.ts` | Split `buildSystemPrompt()` into static `SYSTEM_PROMPT` + `buildDynamicContext()`; listen to stream events; add cache_control to request body |
| `src/services/llm.ts` | Rewrite `chat()` to construct Anthropic-native request with cache_control breakpoints; remove `onChunk`/`onToolUse` callbacks (events replace them) |
| `src/stores/aiStore.ts` | Add `estimateTokens()`; rewrite `conversation` computed with token-budget trimming; add cache_control breakpoints to messages; remove `MAX_CTX_MSGS` constant |
| `src/types/ai.ts` | Add `cache_control` field types if needed |

### No changes

- `src/components/AISidebar.vue` — UI already handles chunked updates via reactive `assistantMsg.content`
- `src/components/AIMessage.vue` — renders `v-html` from reactive content, no changes needed
- All other session/terminal components — no impact

---

## 5. Edge Cases

- **Stop during stream**: Cancel Go context → HTTP request aborted → `ai:done` emits with partial content
- **Network error mid-stream**: Frontend sets error on assistant message, shows debug info
- **Cache miss on long idle**: Model still works, just slower/ more expensive; no user-visible difference
- **Very long tool output**: Truncate at 8000 chars, append `...[truncated]`
- **Context budget exceeded by single message**: Keep the message but flag with warning
- **Multiple rapid stops/starts**: Context cancellation is idempotent; new request gets fresh context

---

## 6. Testing Strategy

- **Unit**: Token estimation accuracy against known Claude tokenizer output
- **Integration**: Mock SSE server, verify event parsing
- **Manual**: Real Anthropic API call, verify cache hit via response `usage` fields
- **Edge**: Stop mid-stream, resume after stop, tool output truncation, terminal switch mid-conversation

# AI Assistant

uniTerm has a built-in autonomous AI Agent that can independently plan and execute multiple rounds of Shell commands in the terminal.

![AI Assistant](/imgs/ai_assistant_light.webp)

## Core Capabilities

### Autonomous Multi-Round Execution

The AI Agent will:

1. **Understand Intent** — Analyze the natural language requirements you input
2. **Make a Plan** — Break it down into executable Shell command steps
3. **Execute and Observe** — Run commands in the terminal and observe the output
4. **Iterate and Adjust** — Revise the plan based on results and continue until the task is completed

### LLM Provider Configuration

Supports Anthropic and OpenAI-compatible APIs. You can add multiple models and switch between them at any time.

![AI Model Settings](/imgs/ai_model_light.webp)

**Model Configuration Fields:**

| Field | Description |
|------|------|
| Name | Custom display name used to distinguish configurations in the model list |
| Protocol | Choose **Anthropic** or **OpenAI** protocol, determining the request format |
| Base URL | API endpoint address. OpenAI defaults to `https://api.openai.com/v1`, Anthropic defaults to `https://api.anthropic.com` |
| User-Agent | Optional. Custom User-Agent header identifier. Presets include uniTerm, Claude Code, Cursor, and manual input is also supported |
| API Key | API authentication key, stored as a password |
| Model | Model name (e.g. `gpt-4o`, `claude-sonnet-4-20250514`). Use the "Fetch Model List" button to pull available models from the API |
| Test Connection | After filling in the API Key and model, click "Test Connection" to verify the configuration |

**Multi-Model Management:**

- Supports adding multiple model configurations, each with independent protocol, address, and key settings
- Switch the active model from the dropdown menu at the top of the AI sidebar
- Edit or delete existing model configurations in Settings

### Execution Mode

| Mode | Description |
|------|------|
| Skip | All commands execute directly without confirmation |
| Dangerous Only | Potentially dangerous commands (rm, sudo, etc.) require confirmation |
| Dangerous + Write | Dangerous commands and file write operations require confirmation |
| Confirm All | Every command requires manual confirmation |

### Terminal Integration

The AI Assistant is deeply integrated with the terminal — commands run directly in terminal tabs without manual copy-paste.

**Execution Flow:**

1. Enter a natural language request in the AI dialog (e.g. "Check disk usage for me")
2. The AI analyzes the request and generates an execution plan
3. Depending on the current execution mode, you may need to confirm each command
4. Commands are executed one by one in the target terminal with real-time output
5. The AI observes the output and automatically adjusts subsequent steps

**Selecting the Target Terminal:**

- **Follow Active Tab** (default) — AI commands execute in the currently active terminal tab. When you switch tabs, subsequent commands automatically switch to the new tab
- **Pin to a Specific Tab** — Click the pin button at the top of the AI panel and select an open terminal tab. All subsequent AI commands will execute in that tab regardless of tab switching

**Split-Screen Collaboration:**

Drag the AI conversation panel to the right area to form a left-right split with the terminal. View the AI's analysis and plan on one side while observing execution results in the terminal on the other — neither side obstructs the other.

::: tip Related
- [Smart Suggestions](/en/features/smart-suggest) — AI-driven command completion
:::

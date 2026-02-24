# ADK Scion Agent Example

An example [ADK (Agent Development Kit)](https://google.github.io/adk-docs/) agent that integrates with scion's lifecycle management. The agent reports its status through scion's `sciontool` so it can be orchestrated alongside other agents in a grove.

## Prerequisites

- Python 3.11+
- `google-adk` package (`pip install google-adk`)
- A Google AI API key or Vertex AI credentials

## Quick Start (Standalone)

```bash
# From the repository root:
cp examples/adk_scion_agent/.env.example examples/adk_scion_agent/.env
# Edit .env and set GOOGLE_API_KEY

cd examples
adk run adk_scion_agent
```

The agent starts an interactive session. Type a task and the agent will work through it, using `file_write` to create files and `sciontool_status` to signal lifecycle events.

When running outside a scion container, `sciontool` won't be on PATH — the agent works normally but status reporting is silently skipped.

## Running Inside a Scion Container

When scion launches this agent inside a container:

1. **sciontool** runs as PID 1 and supervises the agent process.
2. The agent writes transient status updates (`THINKING`, `EXECUTING`, `IDLE`) to `$HOME/agent-info.json` via ADK callbacks.
3. Sticky status transitions (`WAITING_FOR_INPUT`, `COMPLETED`) go through `sciontool status` which also reports to the scion Hub.
4. **Message delivery** works natively: `scion message` sends text via tmux `send-keys` into ADK's `input()` loop.

### Status Lifecycle

```
User sends message
    │
    ▼
THINKING          ← before_agent_callback
    │
    ├──► EXECUTING    ← before_tool_callback (file_write, etc.)
    │        │
    │        ▼
    │    THINKING     ← after_tool_callback
    │        │
    │   (more tools...)
    │
    ▼
IDLE              ← after_agent_callback

If agent calls sciontool_status("task_completed", ...):
    → COMPLETED (sticky — survives subsequent transient updates)

If agent calls sciontool_status("ask_user", ...):
    → WAITING_FOR_INPUT (sticky — cleared when user responds)
```

## Auth Bridging

Scion's Gemini harness sets `GEMINI_API_KEY`. ADK requires `GOOGLE_API_KEY`. The agent bridges this automatically at import time — if `GOOGLE_API_KEY` is unset but `GEMINI_API_KEY` is available, it copies the value over.

For Vertex AI, set `GOOGLE_GENAI_USE_VERTEXAI=true` and configure Application Default Credentials. See `.env.example` for all options.

## Tools

| Tool | Purpose |
|---|---|
| `file_write(file_path, content)` | Write a file to the workspace. Paths are resolved relative to `/workspace` (or CWD). Enforces workspace boundary. |
| `sciontool_status(status_type, message)` | Signal `task_completed` or `ask_user` to scion. |

## Project Structure

```
adk_scion_agent/
├── __init__.py        # ADK package entry point
├── agent.py           # root_agent definition, auth bridging, model config
├── tools.py           # file_write and sciontool_status tools
├── callbacks.py       # ADK callbacks → scion status updates
├── sciontool.py       # Low-level sciontool subprocess wrapper
├── .env.example       # Environment variable template
└── README.md          # This file
```

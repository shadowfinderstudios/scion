"""Root agent definition for the ADK scion example.

This module defines an ADK LlmAgent that integrates with scion's lifecycle
management. It bridges auth environment variables, wires up status-reporting
callbacks, and exposes file_write and sciontool_status as callable tools.

When run with `adk run`, the agent operates as an interactive coding assistant
that reports its status to scion throughout its lifecycle.
"""

import os

from google.adk import Agent

from . import callbacks, tools

# ---------------------------------------------------------------------------
# Auth bridging
# ---------------------------------------------------------------------------
# ADK requires GOOGLE_API_KEY for Gemini API access. Scion's Gemini harness
# provides GEMINI_API_KEY instead. Bridge the gap at import time.
if not os.environ.get("GOOGLE_API_KEY") and os.environ.get("GEMINI_API_KEY"):
    os.environ["GOOGLE_API_KEY"] = os.environ["GEMINI_API_KEY"]

# ---------------------------------------------------------------------------
# Model configuration
# ---------------------------------------------------------------------------
MODEL = os.environ.get("ADK_MODEL", "gemini-2.5-flash")

# ---------------------------------------------------------------------------
# Agent instruction
# ---------------------------------------------------------------------------
AGENT_INSTRUCTION = """\
You are a coding assistant running inside a scion-managed container.

Your workspace is mounted at /workspace (or the current working directory if
running outside a container). You can read, create, and modify files there.

## Available tools

- **file_write(file_path, content)**: Create or overwrite a file in the
  workspace. Paths are relative to the workspace root unless absolute.

- **sciontool_status(status_type, message)**: Signal lifecycle events to scion.
  - Call `sciontool_status("ask_user", "<your question>")` **before** you ask
    the user a question. This lets scion know you are waiting for input.
  - Call `sciontool_status("task_completed", "<summary>")` when you have
    finished the user's task. Summarize what you did.

## Workflow

1. When you receive a task, work through it step by step.
2. Use file_write to create or modify files as needed.
3. If you need clarification, call sciontool_status("ask_user", ...) first,
   then ask your question.
4. When the task is complete, call sciontool_status("task_completed", ...)
   with a brief summary of what you accomplished.
"""

# ---------------------------------------------------------------------------
# Agent construction
# ---------------------------------------------------------------------------
root_agent = Agent(
    model=MODEL,
    name="scion_agent",
    instruction=AGENT_INSTRUCTION,
    tools=[tools.file_write, tools.sciontool_status],
    before_agent_callback=callbacks.before_agent_callback,
    before_tool_callback=callbacks.before_tool_callback,
    after_tool_callback=callbacks.after_tool_callback,
    after_agent_callback=callbacks.after_agent_callback,
)

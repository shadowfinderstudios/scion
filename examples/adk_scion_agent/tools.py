"""Agent-callable tools for file operations and scion status signaling.

These tools are exposed to the LLM agent so it can:
  - Write files to the workspace (file_write)
  - Signal lifecycle events to scion (sciontool_status)
"""

import logging
import os
from pathlib import Path

from . import sciontool

logger = logging.getLogger(__name__)

# Workspace root: /workspace inside scion containers, otherwise CWD.
_WORKSPACE_ROOT = Path(os.environ.get("WORKSPACE_ROOT", "/workspace"))
if not _WORKSPACE_ROOT.exists():
    _WORKSPACE_ROOT = Path.cwd()


def file_write(file_path: str, content: str) -> dict:
    """Write content to a file in the workspace.

    Creates parent directories as needed. The file path is resolved relative
    to the workspace root and must stay within it.

    Args:
        file_path: Path to the file (absolute or relative to workspace root).
        content: The content to write to the file.

    Returns:
        A dict with status, path, and message.
    """
    try:
        target = Path(file_path)
        if not target.is_absolute():
            target = _WORKSPACE_ROOT / target
        target = target.resolve()

        # Safety check: ensure the resolved path is within the workspace.
        workspace_resolved = _WORKSPACE_ROOT.resolve()
        try:
            target.relative_to(workspace_resolved)
        except ValueError:
            return {
                "status": "error",
                "path": str(target),
                "message": (
                    f"Path escapes workspace boundary ({workspace_resolved}). "
                    "File writes are restricted to the workspace directory."
                ),
            }

        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)

        return {
            "status": "success",
            "path": str(target),
            "message": f"Wrote {len(content)} bytes to {target}",
        }
    except Exception as e:
        logger.warning("file_write failed for %s: %s", file_path, e)
        return {
            "status": "error",
            "path": file_path,
            "message": str(e),
        }


def sciontool_status(status_type: str, message: str) -> dict:
    """Signal a lifecycle event to scion's orchestration layer.

    Args:
        status_type: Either "task_completed" or "ask_user".
        message: A description of the event (task summary or question).

    Returns:
        A dict confirming the status update.
    """
    valid_types = {"task_completed", "ask_user"}
    if status_type not in valid_types:
        return {
            "status": "error",
            "message": (
                f"Invalid status_type '{status_type}'. "
                f"Must be one of: {', '.join(sorted(valid_types))}"
            ),
        }

    sciontool.run_status(status_type, message)

    return {
        "status": "success",
        "message": f"Reported {status_type}: {message}",
    }

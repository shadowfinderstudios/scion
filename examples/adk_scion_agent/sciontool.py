"""Low-level wrapper for sciontool — scion's container-side status management CLI.

This module provides two mechanisms for reporting agent status:

1. write_agent_status() — writes directly to $HOME/agent-info.json for transient
   states (THINKING, EXECUTING, IDLE). Uses atomic rename to prevent corruption.

2. run_status() — invokes `sciontool status <type> <message>` for sticky states
   (ask_user → WAITING_FOR_INPUT, task_completed → COMPLETED). The CLI handles
   hub reporting and logging in addition to the local file update.

All functions degrade gracefully when running outside a scion container (i.e.,
when sciontool is not on PATH). Failures are logged but never raised.
"""

import json
import logging
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

logger = logging.getLogger(__name__)

_sciontool_path: str | None = None
_sciontool_searched: bool = False

# Sticky statuses that should not be overwritten by transient updates.
_STICKY_STATUSES = frozenset({"WAITING_FOR_INPUT", "COMPLETED", "LIMITS_EXCEEDED"})


def _find_sciontool() -> str | None:
    """Locate the sciontool binary, caching the result."""
    global _sciontool_path, _sciontool_searched
    if not _sciontool_searched:
        _sciontool_path = shutil.which("sciontool")
        _sciontool_searched = True
        if _sciontool_path:
            logger.debug("Found sciontool at %s", _sciontool_path)
        else:
            logger.debug("sciontool not found on PATH — status reporting disabled")
    return _sciontool_path


def _agent_info_path() -> Path:
    """Return the path to agent-info.json."""
    return Path(os.environ.get("HOME", "/home/scion")) / "agent-info.json"


def _read_current_status() -> str | None:
    """Read the current status from agent-info.json, or None if unavailable."""
    try:
        path = _agent_info_path()
        if path.exists():
            data = json.loads(path.read_text())
            return data.get("status")
    except Exception:
        pass
    return None


def write_agent_status(status: str) -> None:
    """Write a transient status to agent-info.json via atomic rename.

    Respects sticky status semantics — will not overwrite WAITING_FOR_INPUT,
    COMPLETED, or LIMITS_EXCEEDED.

    Args:
        status: One of THINKING, EXECUTING, IDLE (or other transient states).
    """
    try:
        current = _read_current_status()
        if current in _STICKY_STATUSES:
            logger.debug(
                "Skipping transient status %s — current sticky status is %s",
                status,
                current,
            )
            return

        info_path = _agent_info_path()

        # Preserve existing fields in the file.
        existing: dict = {}
        try:
            if info_path.exists():
                existing = json.loads(info_path.read_text())
        except Exception:
            pass

        existing["status"] = status
        # Clean up legacy field if present.
        existing.pop("sessionStatus", None)

        # Atomic write: write to temp file in the same directory, then rename.
        fd, tmp_path = tempfile.mkstemp(
            dir=str(info_path.parent), suffix=".tmp", prefix="agent-info-"
        )
        try:
            with os.fdopen(fd, "w") as f:
                json.dump(existing, f)
            os.rename(tmp_path, str(info_path))
        except Exception:
            # Clean up temp file on failure.
            try:
                os.unlink(tmp_path)
            except OSError:
                pass
            raise

        logger.debug("Updated agent-info.json status to %s", status)
    except Exception:
        logger.warning("Failed to write agent status %s", status, exc_info=True)


def run_status(status_type: str, message: str) -> None:
    """Invoke `sciontool status <type> <message>` for sticky state transitions.

    This is used for states that require hub reporting and logging beyond the
    local agent-info.json update (ask_user, task_completed, limits_exceeded).

    Args:
        status_type: One of "ask_user", "task_completed", "limits_exceeded".
        message: A human-readable message describing the status change.
    """
    binary = _find_sciontool()
    if not binary:
        logger.debug(
            "sciontool not available — skipping status %s: %s", status_type, message
        )
        return

    try:
        result = subprocess.run(
            [binary, "status", status_type, message],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            logger.warning(
                "sciontool status %s exited %d: %s",
                status_type,
                result.returncode,
                result.stderr.strip(),
            )
        else:
            logger.debug("sciontool status %s: %s", status_type, message)
    except Exception:
        logger.warning(
            "Failed to run sciontool status %s", status_type, exc_info=True
        )

#!/bin/sh

set -eu

payload="${1-}"
if [ -z "$payload" ]; then
  payload="$(cat)"
fi

if [ -z "$payload" ]; then
  exit 0
fi

event="$(printf '%s' "$payload" | sed -n 's/.*"type"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
if [ -z "$event" ]; then
  event="$(printf '%s' "$payload" | sed -n 's/.*"event"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
fi

if [ "$event" = "agent-turn-complete" ]; then
  autoc="${SCION_CODEX_NOTIFY_AUTO_COMPLETE-true}"
  if [ "$autoc" = "false" ] || [ "$autoc" = "0" ] || [ "$autoc" = "no" ]; then
    exit 0
  fi

  title="$(printf '%s' "$payload" | sed -n 's/.*"title"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [ -z "$title" ]; then
    title="Codex turn completed"
  fi

  sciontool status task_completed "$title" >/dev/null 2>&1 || true
fi

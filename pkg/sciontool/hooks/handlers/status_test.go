/*
Copyright 2025 The Scion Authors.
*/

package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusHandler_UpdateStatus(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "agent-info.json")

	h := &StatusHandler{
		StatusPath: statusPath,
	}

	// Test updating status
	err := h.UpdateStatus(hooks.StateThinking, false)
	require.NoError(t, err)

	// Verify file contents
	data, err := os.ReadFile(statusPath)
	require.NoError(t, err)

	var info AgentInfo
	err = json.Unmarshal(data, &info)
	require.NoError(t, err)
	assert.Equal(t, "THINKING", info.Status)
	assert.Empty(t, info.SessionStatus)

	// Test updating session status
	err = h.UpdateStatus(hooks.StateWaitingForInput, true)
	require.NoError(t, err)

	data, err = os.ReadFile(statusPath)
	require.NoError(t, err)
	err = json.Unmarshal(data, &info)
	require.NoError(t, err)
	assert.Equal(t, "THINKING", info.Status) // Previous status preserved
	assert.Equal(t, "WAITING_FOR_INPUT", info.SessionStatus)
}

func TestStatusHandler_Handle(t *testing.T) {
	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "agent-info.json")

	h := &StatusHandler{
		StatusPath: statusPath,
	}

	tests := []struct {
		name       string
		event      *hooks.Event
		wantStatus hooks.AgentState
	}{
		{
			name:       "SessionStart sets STARTING",
			event:      &hooks.Event{Name: hooks.EventSessionStart},
			wantStatus: hooks.StateStarting,
		},
		{
			name:       "PreStart sets INITIALIZING",
			event:      &hooks.Event{Name: hooks.EventPreStart},
			wantStatus: hooks.StateInitializing,
		},
		{
			name:       "PostStart sets IDLE",
			event:      &hooks.Event{Name: hooks.EventPostStart},
			wantStatus: hooks.StateIdle,
		},
		{
			name:       "PreStop sets SHUTTING_DOWN",
			event:      &hooks.Event{Name: hooks.EventPreStop},
			wantStatus: hooks.StateShuttingDown,
		},
		{
			name:       "PromptSubmit sets THINKING",
			event:      &hooks.Event{Name: hooks.EventPromptSubmit},
			wantStatus: hooks.StateThinking,
		},
		{
			name:       "ToolStart sets EXECUTING",
			event:      &hooks.Event{Name: hooks.EventToolStart, Data: hooks.EventData{ToolName: "Bash"}},
			wantStatus: hooks.StateExecuting,
		},
		{
			name:       "ToolEnd sets IDLE",
			event:      &hooks.Event{Name: hooks.EventToolEnd},
			wantStatus: hooks.StateIdle,
		},
		{
			name:       "AgentEnd sets IDLE",
			event:      &hooks.Event{Name: hooks.EventAgentEnd},
			wantStatus: hooks.StateIdle,
		},
		{
			name:       "SessionEnd sets EXITED",
			event:      &hooks.Event{Name: hooks.EventSessionEnd},
			wantStatus: hooks.StateExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Handle(tt.event)
			require.NoError(t, err)

			data, err := os.ReadFile(statusPath)
			require.NoError(t, err)

			var info AgentInfo
			err = json.Unmarshal(data, &info)
			require.NoError(t, err)
			assert.Equal(t, string(tt.wantStatus), info.Status)
		})
	}
}

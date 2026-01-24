/*
Copyright 2025 The Scion Authors.
*/

// Package handlers provides hook handler implementations.
package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
)

// StatusHandler manages agent status in a JSON file.
// It replicates the functionality of scion_tool.py's update_status function.
type StatusHandler struct {
	// StatusPath is the path to the agent-info.json file.
	StatusPath string

	mu sync.Mutex
}

// AgentInfo represents the agent status JSON structure.
type AgentInfo struct {
	Status        string `json:"status,omitempty"`
	SessionStatus string `json:"sessionStatus,omitempty"`
}

// NewStatusHandler creates a new status handler.
func NewStatusHandler() *StatusHandler {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/scion"
	}
	return &StatusHandler{
		StatusPath: filepath.Join(home, "agent-info.json"),
	}
}

// Handle processes an event and updates the agent status.
func (h *StatusHandler) Handle(event *hooks.Event) error {
	state := h.eventToState(event)
	if state == "" {
		return nil // Event doesn't trigger a state change
	}

	return h.UpdateStatus(state, false)
}

// UpdateStatus writes the status to the agent-info.json file atomically.
func (h *StatusHandler) UpdateStatus(status hooks.AgentState, sessionStatus bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Read existing data
	info := &AgentInfo{}
	if data, err := os.ReadFile(h.StatusPath); err == nil {
		_ = json.Unmarshal(data, info)
	}

	// Update the appropriate field
	if sessionStatus {
		info.SessionStatus = string(status)
	} else {
		info.Status = string(status)
	}

	// Write atomically
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}

	dir := filepath.Dir(h.StatusPath)
	tmpFile, err := os.CreateTemp(dir, "agent-info-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, h.StatusPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

// eventToState maps normalized events to agent states.
func (h *StatusHandler) eventToState(event *hooks.Event) hooks.AgentState {
	switch event.Name {
	case hooks.EventSessionStart:
		return hooks.StateStarting

	case hooks.EventPreStart:
		return hooks.StateInitializing

	case hooks.EventPostStart:
		return hooks.StateIdle

	case hooks.EventPreStop:
		return hooks.StateShuttingDown

	case hooks.EventPromptSubmit, hooks.EventAgentStart:
		return hooks.StateThinking

	case hooks.EventModelStart:
		return hooks.StateThinking

	case hooks.EventModelEnd:
		return hooks.StateIdle

	case hooks.EventToolStart:
		// Include tool name in state if available
		if event.Data.ToolName != "" {
			// Return a dynamic state - caller should handle formatting
			return hooks.StateExecuting
		}
		return hooks.StateExecuting

	case hooks.EventToolEnd, hooks.EventAgentEnd:
		return hooks.StateIdle

	case hooks.EventNotification:
		return hooks.StateWaitingForInput

	case hooks.EventSessionEnd:
		return hooks.StateExited

	default:
		return "" // No state change
	}
}

// GetFormattedState returns the state with tool name if applicable.
func (h *StatusHandler) GetFormattedState(event *hooks.Event) string {
	state := h.eventToState(event)
	if state == hooks.StateExecuting && event.Data.ToolName != "" {
		return fmt.Sprintf("%s (%s)", state, event.Data.ToolName)
	}
	return string(state)
}

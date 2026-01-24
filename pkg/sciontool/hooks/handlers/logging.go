/*
Copyright 2025 The Scion Authors.
*/

package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
)

// LoggingHandler logs hook events to a file.
// It replicates the functionality of scion_tool.py's log_event function.
type LoggingHandler struct {
	// LogPath is the path to the agent.log file.
	LogPath string

	mu sync.Mutex
}

// NewLoggingHandler creates a new logging handler.
func NewLoggingHandler() *LoggingHandler {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/scion"
	}
	return &LoggingHandler{
		LogPath: filepath.Join(home, "agent.log"),
	}
}

// Handle logs an event to the log file.
func (h *LoggingHandler) Handle(event *hooks.Event) error {
	state := h.eventToState(event)
	message := h.formatLogMessage(event)

	return h.LogEvent(state, message)
}

// LogEvent writes a log entry to the agent log file.
func (h *LoggingHandler) LogEvent(state hooks.AgentState, message string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("%s [%s] %s\n", timestamp, state, message)

	f, err := os.OpenFile(h.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("writing log entry: %w", err)
	}

	return nil
}

// eventToState maps normalized events to agent states for logging.
func (h *LoggingHandler) eventToState(event *hooks.Event) hooks.AgentState {
	switch event.Name {
	case hooks.EventSessionStart:
		return hooks.StateStarting
	case hooks.EventPromptSubmit, hooks.EventAgentStart:
		return hooks.StateThinking
	case hooks.EventModelStart:
		return hooks.StateThinking
	case hooks.EventModelEnd:
		return hooks.StateIdle
	case hooks.EventToolStart:
		return hooks.StateExecuting
	case hooks.EventToolEnd, hooks.EventAgentEnd:
		return hooks.StateIdle
	case hooks.EventNotification:
		return hooks.StateWaitingForInput
	case hooks.EventSessionEnd:
		return hooks.StateExited
	case hooks.EventPreStart:
		return hooks.StateInitializing
	case hooks.EventPostStart:
		return hooks.StateIdle
	case hooks.EventPreStop:
		return hooks.StateShuttingDown
	default:
		return hooks.StateIdle
	}
}

// formatLogMessage creates a human-readable log message for an event.
func (h *LoggingHandler) formatLogMessage(event *hooks.Event) string {
	switch event.Name {
	case hooks.EventSessionStart:
		if event.Data.Source != "" {
			return fmt.Sprintf("Session started (source: %s)", event.Data.Source)
		}
		return "Session started"

	case hooks.EventSessionEnd:
		if event.Data.Reason != "" {
			return fmt.Sprintf("Session ended (reason: %s)", event.Data.Reason)
		}
		return "Session ended"

	case hooks.EventPromptSubmit:
		if event.Data.Prompt != "" {
			prompt := event.Data.Prompt
			if len(prompt) > 100 {
				prompt = prompt[:100] + "..."
			}
			return fmt.Sprintf("User prompt: %s", prompt)
		}
		return "User prompt submitted"

	case hooks.EventAgentStart:
		return "Agent turn started"

	case hooks.EventAgentEnd:
		return "Agent turn completed"

	case hooks.EventToolStart:
		if event.Data.ToolName != "" {
			return fmt.Sprintf("Running tool: %s", event.Data.ToolName)
		}
		return "Tool execution started"

	case hooks.EventToolEnd:
		if event.Data.ToolName != "" {
			return fmt.Sprintf("Tool %s completed", event.Data.ToolName)
		}
		return "Tool execution completed"

	case hooks.EventModelStart:
		return "LLM call started"

	case hooks.EventModelEnd:
		return "LLM call completed"

	case hooks.EventNotification:
		if event.Data.Message != "" {
			return fmt.Sprintf("Notification: %s", event.Data.Message)
		}
		return "Notification received"

	case hooks.EventPreStart:
		return "Container initializing"

	case hooks.EventPostStart:
		return "Container ready"

	case hooks.EventPreStop:
		return "Container shutting down (received termination signal)"

	default:
		return fmt.Sprintf("Event: %s", event.RawName)
	}
}

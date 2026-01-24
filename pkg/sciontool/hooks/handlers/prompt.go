/*
Copyright 2025 The Scion Authors.
*/

package handlers

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
)

// PromptHandler saves the initial user prompt to a file.
// This replicates the functionality from scion_tool.py that saves
// the first prompt to ~/prompt.md.
type PromptHandler struct {
	// PromptPath is the path to save the prompt.
	PromptPath string

	// saved tracks if a prompt has already been saved this session.
	saved bool
	mu    sync.Mutex
}

// NewPromptHandler creates a new prompt handler.
func NewPromptHandler() *PromptHandler {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/scion"
	}
	return &PromptHandler{
		PromptPath: filepath.Join(home, "prompt.md"),
	}
}

// Handle saves the prompt if this is the first prompt of the session.
func (h *PromptHandler) Handle(event *hooks.Event) error {
	// Only handle prompt events
	if event.Name != hooks.EventPromptSubmit && event.Name != hooks.EventAgentStart {
		return nil
	}

	// No prompt data
	if event.Data.Prompt == "" {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if we've already saved a prompt
	if h.saved {
		return nil
	}

	// Check if file exists and has content
	if info, err := os.Stat(h.PromptPath); err == nil && info.Size() > 0 {
		h.saved = true
		return nil
	}

	// Save the prompt
	if err := os.WriteFile(h.PromptPath, []byte(event.Data.Prompt), 0644); err != nil {
		return err
	}

	h.saved = true
	return nil
}

// Reset clears the saved state for a new session.
func (h *PromptHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.saved = false
}

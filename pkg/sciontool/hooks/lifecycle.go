/*
Copyright 2025 The Scion Authors.
*/

package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LifecycleManager handles Scion lifecycle hooks.
// These are container-level events managed by sciontool init.
type LifecycleManager struct {
	// HooksDir is the directory containing hook scripts.
	// Defaults to /etc/scion/hooks or $SCION_HOOKS_DIR
	HooksDir string

	// Handlers are the registered handlers for lifecycle events.
	Handlers map[string][]Handler
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager() *LifecycleManager {
	hooksDir := os.Getenv("SCION_HOOKS_DIR")
	if hooksDir == "" {
		hooksDir = "/etc/scion/hooks"
	}

	return &LifecycleManager{
		HooksDir: hooksDir,
		Handlers: make(map[string][]Handler),
	}
}

// RegisterHandler adds a handler for a lifecycle event.
func (m *LifecycleManager) RegisterHandler(eventName string, handler Handler) {
	m.Handlers[eventName] = append(m.Handlers[eventName], handler)
}

// RunPreStart executes pre-start lifecycle hooks.
// Called after container setup but before the child process starts.
func (m *LifecycleManager) RunPreStart() error {
	event := &Event{
		Name: EventPreStart,
	}
	return m.runHooks(event)
}

// RunPostStart executes post-start lifecycle hooks.
// Called after the child process is confirmed running.
func (m *LifecycleManager) RunPostStart() error {
	event := &Event{
		Name: EventPostStart,
	}
	return m.runHooks(event)
}

// RunPreStop executes pre-stop lifecycle hooks.
// Called when a termination signal (SIGTERM/SIGINT) is received,
// before starting the graceful shutdown process.
func (m *LifecycleManager) RunPreStop() error {
	event := &Event{
		Name: EventPreStop,
	}
	return m.runHooks(event)
}

// RunSessionEnd executes session-end lifecycle hooks.
// Called on graceful shutdown before child termination.
func (m *LifecycleManager) RunSessionEnd() error {
	event := &Event{
		Name: EventSessionEnd,
	}
	return m.runHooks(event)
}

// runHooks executes both script-based and registered handlers for an event.
func (m *LifecycleManager) runHooks(event *Event) error {
	var errs []string

	// Run script-based hooks
	if err := m.runScriptHooks(event.Name); err != nil {
		errs = append(errs, fmt.Sprintf("script hooks: %v", err))
	}

	// Run registered handlers
	if handlers, ok := m.Handlers[event.Name]; ok {
		for _, handler := range handlers {
			if err := handler(event); err != nil {
				errs = append(errs, fmt.Sprintf("handler: %v", err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("hook errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// runScriptHooks looks for and executes script files in the hooks directory.
func (m *LifecycleManager) runScriptHooks(eventName string) error {
	// Check if hooks directory exists
	if _, err := os.Stat(m.HooksDir); os.IsNotExist(err) {
		return nil // No hooks directory, skip
	}

	// Look for hook scripts (e.g., pre-start, pre-start.sh, pre-start.d/*)
	patterns := []string{
		filepath.Join(m.HooksDir, eventName),
		filepath.Join(m.HooksDir, eventName+".sh"),
	}

	for _, pattern := range patterns {
		if info, err := os.Stat(pattern); err == nil && !info.IsDir() {
			if err := m.executeScript(pattern); err != nil {
				return fmt.Errorf("script %s: %w", pattern, err)
			}
		}
	}

	// Check for .d directory with multiple scripts
	dirPath := filepath.Join(m.HooksDir, eventName+".d")
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return fmt.Errorf("reading hooks dir %s: %w", dirPath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			scriptPath := filepath.Join(dirPath, entry.Name())
			if err := m.executeScript(scriptPath); err != nil {
				return fmt.Errorf("script %s: %w", scriptPath, err)
			}
		}
	}

	return nil
}

// executeScript runs a hook script.
func (m *LifecycleManager) executeScript(path string) error {
	// Check if executable
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		// Not executable, skip with warning
		fmt.Fprintf(os.Stderr, "[sciontool] Warning: hook script %s is not executable, skipping\n", path)
		return nil
	}

	cmd := exec.Command(path)
	cmd.Stdout = os.Stderr // Redirect hook output to stderr
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	return nil
}

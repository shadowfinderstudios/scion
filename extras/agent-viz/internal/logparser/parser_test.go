package logparser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLogFile(t *testing.T) {
	// Create a test log file
	entries := []GCPLogEntry{
		{
			InsertID:  "1",
			Timestamp: "2026-03-22T16:30:00.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-agents",
			Labels: map[string]string{
				"agent_id":      "agent-1",
				"scion.harness": "gemini",
			},
			JSONPayload: map[string]any{
				"message": "agent.lifecycle.pre_start",
			},
		},
		{
			InsertID:  "2",
			Timestamp: "2026-03-22T16:30:01.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-agents",
			Labels: map[string]string{
				"agent_id":      "agent-1",
				"scion.harness": "gemini",
			},
			JSONPayload: map[string]any{
				"message": "agent.session.start",
			},
		},
		{
			InsertID:  "3",
			Timestamp: "2026-03-22T16:30:05.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-agents",
			Labels: map[string]string{
				"agent_id":      "agent-1",
				"scion.harness": "gemini",
			},
			JSONPayload: map[string]any{
				"message":   "agent.tool.call",
				"tool_name": "write_file",
				"file_path": "/workspace/src/main.go",
			},
		},
		{
			InsertID:  "4",
			Timestamp: "2026-03-22T16:30:10.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-messages",
			Labels: map[string]string{
				"sender":       "agent:alpha",
				"sender_id":    "agent-1",
				"recipient":    "agent:beta",
				"recipient_id": "agent-2",
				"msg_type":     "instruction",
				"grove_id":     "grove-1",
			},
			JSONPayload: map[string]any{
				"sender":          "agent:alpha",
				"recipient":       "agent:beta",
				"msg_type":        "instruction",
				"message_content": "do something",
				"message":         "message dispatched",
			},
		},
		{
			InsertID:  "5",
			Timestamp: "2026-03-22T16:30:15.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-agents",
			Labels: map[string]string{
				"agent_id":      "agent-2",
				"scion.harness": "claude",
			},
			JSONPayload: map[string]any{
				"message": "agent.session.start",
			},
		},
		{
			InsertID:  "6",
			Timestamp: "2026-03-22T16:31:00.000Z",
			Severity:  "INFO",
			LogName:   "projects/test/logs/scion-agents",
			Labels: map[string]string{
				"agent_id":      "agent-1",
				"scion.harness": "gemini",
			},
			JSONPayload: map[string]any{
				"message": "agent.session.end",
			},
		},
	}

	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test-logs.json")
	if err := os.WriteFile(logPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ParseLogFile(logPath)
	if err != nil {
		t.Fatalf("ParseLogFile failed: %v", err)
	}

	// Verify manifest
	if result.Manifest.Type != "manifest" {
		t.Errorf("expected manifest type, got %s", result.Manifest.Type)
	}

	// Verify agents found
	if len(result.Manifest.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(result.Manifest.Agents))
	}

	// Verify agent names resolved from messages
	agentNames := map[string]bool{}
	for _, a := range result.Manifest.Agents {
		agentNames[a.Name] = true
	}
	if !agentNames["alpha"] {
		t.Error("expected agent name 'alpha' from message sender")
	}

	// Verify files extracted
	fileIDs := map[string]bool{}
	for _, f := range result.Manifest.Files {
		fileIDs[f.ID] = true
	}
	if !fileIDs["src/main.go"] {
		t.Error("expected file 'src/main.go' from write_file tool call")
	}
	if !fileIDs["src"] {
		t.Error("expected directory 'src' from file tree")
	}

	// Verify events
	if len(result.Events) == 0 {
		t.Fatal("expected events, got none")
	}

	// Count event types
	typeCounts := map[string]int{}
	for _, e := range result.Events {
		typeCounts[e.Type]++
	}
	if typeCounts["agent_state"] == 0 {
		t.Error("expected agent_state events")
	}
	if typeCounts["message"] != 1 {
		t.Errorf("expected 1 message event, got %d", typeCounts["message"])
	}
	if typeCounts["file_edit"] != 1 {
		t.Errorf("expected 1 file_edit event, got %d", typeCounts["file_edit"])
	}

	// Verify time range
	if result.Manifest.TimeRange.Start != "2026-03-22T16:30:00.000Z" {
		t.Errorf("unexpected start time: %s", result.Manifest.TimeRange.Start)
	}
}

func TestIsFileEditTool(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"write_file", true},
		{"create_file", true},
		{"Write", true},
		{"edit_file", true},
		{"Edit", true},
		{"patch_file", true},
		{"read_file", false},
		{"Read", false},
		{"run_shell_command", false},
		{"Bash", false},
	}

	for _, tt := range tests {
		if got := isFileEditTool(tt.name); got != tt.expected {
			t.Errorf("isFileEditTool(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestTimestampToTime(t *testing.T) {
	ts := "2026-03-22T16:30:00.123456789Z"
	tm, err := TimestampToTime(ts)
	if err != nil {
		t.Fatal(err)
	}
	if tm.Year() != 2026 || tm.Month() != 3 || tm.Day() != 22 {
		t.Errorf("unexpected parsed time: %v", tm)
	}
}

func TestExtractFilesPlaceholder(t *testing.T) {
	// When no file tool calls, should create placeholder structure
	entries := []GCPLogEntry{
		{
			LogName: "projects/test/logs/scion-agents",
			Labels:  map[string]string{"agent_id": "a1"},
			JSONPayload: map[string]any{
				"message": "agent.session.start",
			},
		},
	}
	files := extractFiles(entries)
	if len(files) == 0 {
		t.Error("expected placeholder files when no tool calls found")
	}
}

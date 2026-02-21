// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeInjectAgentInstructions(t *testing.T) {
	agentHome := t.TempDir()
	o := &OpenCode{}
	content := []byte("# Agent Instructions\nDo good work.")

	if err := o.InjectAgentInstructions(agentHome, content); err != nil {
		t.Fatalf("InjectAgentInstructions failed: %v", err)
	}

	target := filepath.Join(agentHome, "AGENTS.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected file at %s: %v", target, err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestOpenCodeRequiredEnvKeys(t *testing.T) {
	o := &OpenCode{}

	got := o.RequiredEnvKeys("")
	if len(got) != 1 || got[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("RequiredEnvKeys() = %v, want [ANTHROPIC_API_KEY]", got)
	}
}

func TestOpenCodeInjectSystemPrompt(t *testing.T) {
	agentHome := t.TempDir()
	o := &OpenCode{}

	// First inject agent instructions
	agentContent := []byte("# Existing Instructions\nDo things.")
	if err := o.InjectAgentInstructions(agentHome, agentContent); err != nil {
		t.Fatalf("InjectAgentInstructions failed: %v", err)
	}

	// Now inject system prompt (should prepend)
	sysContent := []byte("You are a helpful assistant.")
	if err := o.InjectSystemPrompt(agentHome, sysContent); err != nil {
		t.Fatalf("InjectSystemPrompt failed: %v", err)
	}

	target := filepath.Join(agentHome, "AGENTS.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected file at %s: %v", target, err)
	}

	content := string(data)
	if !strings.Contains(content, "# System Prompt") {
		t.Error("expected system prompt header in merged content")
	}
	if !strings.Contains(content, "You are a helpful assistant.") {
		t.Error("expected system prompt content in merged file")
	}
	if !strings.Contains(content, "# Existing Instructions") {
		t.Error("expected original agent instructions to be preserved")
	}
}

func TestOpenCodeInjectSystemPrompt_NoExistingInstructions(t *testing.T) {
	agentHome := t.TempDir()
	o := &OpenCode{}

	sysContent := []byte("You are a helpful assistant.")
	if err := o.InjectSystemPrompt(agentHome, sysContent); err != nil {
		t.Fatalf("InjectSystemPrompt failed: %v", err)
	}

	target := filepath.Join(agentHome, "AGENTS.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected file at %s: %v", target, err)
	}

	content := string(data)
	if !strings.Contains(content, "# System Prompt") {
		t.Error("expected system prompt header")
	}
	if !strings.Contains(content, "You are a helpful assistant.") {
		t.Error("expected system prompt content")
	}
}

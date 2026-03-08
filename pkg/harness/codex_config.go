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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ptone/scion-agent/pkg/api"
)

const codexNotifyCommand = "sh ~/.codex/scion_notify.sh"

func (c *Codex) reconcileConfig(agentHome string, telemetry *api.TelemetryConfig, env map[string]string) error {
	codexDir := filepath.Join(agentHome, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return fmt.Errorf("failed to create .codex directory: %w", err)
	}

	configPath := filepath.Join(codexDir, "config.toml")
	content := ""
	if data, err := os.ReadFile(configPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read codex config: %w", err)
	}

	// Always wire Scion's notify bridge.
	content = upsertTOMLKey(content, "", "notify", `"`+codexNotifyCommand+`"`)

	// Reconcile [otel] only when telemetry is enabled.
	if telemetry != nil && (telemetry.Enabled == nil || *telemetry.Enabled) {
		endpoint := resolveCodexOTELEndpoint(telemetry, env)
		protocol := resolveCodexOTELProtocol(telemetry, env)

		content = upsertTOMLKey(content, "otel", "enabled", "true")
		content = upsertTOMLKey(content, "otel", "exporter", `"otlp"`)
		content = upsertTOMLKey(content, "otel", "endpoint", `"`+endpoint+`"`)
		content = upsertTOMLKey(content, "otel", "protocol", `"`+protocol+`"`)

		// Preserve Scion prompt-privacy defaults in Codex when an explicit event
		// filter includes/excludes agent.user.prompt.
		if telemetry.Filter != nil && telemetry.Filter.Events != nil {
			if listContains(telemetry.Filter.Events.Exclude, "agent.user.prompt") {
				content = upsertTOMLKey(content, "otel", "log_user_prompts", "false")
			}
			if listContains(telemetry.Filter.Events.Include, "agent.user.prompt") {
				content = upsertTOMLKey(content, "otel", "log_user_prompts", "true")
			}
		}
	}

	return os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0644)
}

func resolveCodexOTELEndpoint(telemetry *api.TelemetryConfig, env map[string]string) string {
	if v := firstNonEmpty(
		resolveEnv("SCION_CODEX_OTEL_ENDPOINT", env),
		resolveEnv("SCION_OTEL_ENDPOINT", env),
	); v != "" {
		return v
	}
	if telemetry != nil && telemetry.Cloud != nil && telemetry.Cloud.Endpoint != "" {
		return telemetry.Cloud.Endpoint
	}
	return "localhost:4317"
}

func resolveCodexOTELProtocol(telemetry *api.TelemetryConfig, env map[string]string) string {
	if v := firstNonEmpty(
		resolveEnv("SCION_CODEX_OTEL_PROTOCOL", env),
		resolveEnv("SCION_OTEL_PROTOCOL", env),
	); v != "" {
		return v
	}
	if telemetry != nil && telemetry.Cloud != nil && telemetry.Cloud.Protocol != "" {
		return telemetry.Cloud.Protocol
	}
	return "grpc"
}

func resolveEnv(key string, env map[string]string) string {
	if env != nil {
		if v := strings.TrimSpace(env[key]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv(key))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func listContains(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func upsertTOMLKey(content, section, key, value string) string {
	lines := strings.Split(content, "\n")
	targetSection := strings.TrimSpace(section)

	sectionStart := 0
	sectionEnd := len(lines)
	currentSection := ""
	foundSection := targetSection == ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			if currentSection == targetSection {
				sectionEnd = i
				break
			}
			currentSection = sectionName
			if sectionName == targetSection {
				foundSection = true
				sectionStart = i + 1
				sectionEnd = len(lines)
			}
		}
	}

	if targetSection == "" {
		sectionStart = 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				sectionEnd = i
				break
			}
		}
	}

	if !foundSection && targetSection != "" {
		if strings.TrimSpace(content) != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if strings.TrimSpace(content) != "" {
			content += "\n"
		}
		content += "[" + targetSection + "]\n" + key + " = " + value + "\n"
		return content
	}

	for i := sectionStart; i < sectionEnd; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, key+" ") || strings.HasPrefix(line, key+"=") {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
	}

	insertAt := sectionEnd
	newLine := key + " = " + value
	lines = append(lines[:insertAt], append([]string{newLine}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n")
}

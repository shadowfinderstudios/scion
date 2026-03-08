/*
Copyright 2025 The Scion Authors.
*/

package dialects

import "github.com/ptone/scion-agent/pkg/sciontool/hooks"

// CodexDialect parses Codex notify payloads.
// This provides a minimal registration point while Codex payloads evolve.
type CodexDialect struct{}

func NewCodexDialect() *CodexDialect {
	return &CodexDialect{}
}

func (d *CodexDialect) Name() string {
	return "codex"
}

func (d *CodexDialect) Parse(data map[string]interface{}) (*hooks.Event, error) {
	rawName := getString(data, "type")
	if rawName == "" {
		rawName = getString(data, "event")
	}

	event := &hooks.Event{
		Name:    d.normalizeEventName(rawName),
		RawName: rawName,
		Dialect: "codex",
		Data: hooks.EventData{
			Message:  firstNonEmptyString(getString(data, "title"), getString(data, "message")),
			ToolName: getString(data, "tool_name"),
			Raw:      data,
		},
	}
	return event, nil
}

func (d *CodexDialect) normalizeEventName(name string) string {
	switch name {
	case "agent-turn-complete":
		return hooks.EventResponseComplete
	case "notification":
		return hooks.EventNotification
	default:
		return name
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

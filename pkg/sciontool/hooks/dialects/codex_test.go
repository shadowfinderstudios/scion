/*
Copyright 2025 The Scion Authors.
*/

package dialects

import (
	"testing"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexDialectParse_TurnComplete(t *testing.T) {
	d := NewCodexDialect()
	event, err := d.Parse(map[string]interface{}{
		"type":  "agent-turn-complete",
		"title": "Done",
	})
	require.NoError(t, err)
	assert.Equal(t, hooks.EventResponseComplete, event.Name)
	assert.Equal(t, "Done", event.Data.Message)
	assert.Equal(t, "codex", event.Dialect)
}

func TestCodexDialectParse_FallbackEventField(t *testing.T) {
	d := NewCodexDialect()
	event, err := d.Parse(map[string]interface{}{
		"event":   "notification",
		"message": "Need approval",
	})
	require.NoError(t, err)
	assert.Equal(t, hooks.EventNotification, event.Name)
	assert.Equal(t, "Need approval", event.Data.Message)
}

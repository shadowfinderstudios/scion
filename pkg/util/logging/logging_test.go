package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCPHandler(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := NewGCPHandler(&buf, opts, "test-component")
	logger := slog.New(handler)

	logger.Info("test message", "key", "value")

	var data map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &data)
	assert.NoError(t, err)

	assert.Equal(t, "INFO", data[GCPKeySeverity])
	assert.Equal(t, "test message", data[GCPKeyMessage])
	assert.NotNil(t, data[GCPKeyTimestamp])
	assert.Equal(t, "value", data["key"])

	labels := data[GCPKeyLabels].(map[string]interface{})
	assert.Equal(t, "test-component", labels["component"])
	
	hostname, _ := os.Hostname()
	if hostname != "" {
		assert.Equal(t, hostname, labels["hostname"])
	}
}

func TestGCPHandlerSourceLocation(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo, AddSource: true}
	handler := NewGCPHandler(&buf, opts, "test-component")
	logger := slog.New(handler)

	logger.Info("test message")

	var data map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &data)
	assert.NoError(t, err)

	source := data[GCPKeySourceLocation].(map[string]interface{})
	assert.Contains(t, source["file"], "logging_test.go")
	assert.NotEmpty(t, source["line"])
	assert.Contains(t, source["function"], "TestGCPHandlerSourceLocation")
}

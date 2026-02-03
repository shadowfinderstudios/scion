package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ptone/scion-agent/pkg/store"
	"github.com/ptone/scion-agent/pkg/store/sqlite"
	"github.com/ptone/scion-agent/pkg/transfer"
)

// testWorkspaceDevToken is the development token used for workspace testing.
const testWorkspaceDevToken = "scion_dev_workspace_test_token_1234567890"

// testWorkspaceServer creates a test server for workspace handler tests.
func testWorkspaceServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test store: %v", err)
	}

	cfg := DefaultServerConfig()
	cfg.DevAuthToken = testWorkspaceDevToken
	srv := New(cfg, s)
	return srv, s
}

// createTestGrove creates a grove for tests that need to create agents.
func createTestGrove(t *testing.T, s store.Store, groveID string) {
	t.Helper()
	grove := &store.Grove{
		ID:        groveID,
		Slug:      "test-grove",
		Name:      "Test Grove",
		GitRemote: "https://github.com/test/repo",
		Created:   time.Now(),
		Updated:   time.Now(),
	}
	if err := s.CreateGrove(context.Background(), grove); err != nil {
		t.Fatalf("failed to create grove: %v", err)
	}
}

func TestWorkspaceRoutesParsing(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedID     string
		expectedAction string
	}{
		{
			name:           "workspace status",
			url:            "/api/v1/agents/agent-123/workspace",
			expectedID:     "agent-123",
			expectedAction: "workspace",
		},
		{
			name:           "workspace sync-from",
			url:            "/api/v1/agents/agent-123/workspace/sync-from",
			expectedID:     "agent-123",
			expectedAction: "workspace/sync-from",
		},
		{
			name:           "workspace sync-to",
			url:            "/api/v1/agents/agent-123/workspace/sync-to",
			expectedID:     "agent-123",
			expectedAction: "workspace/sync-to",
		},
		{
			name:           "workspace sync-to finalize",
			url:            "/api/v1/agents/agent-123/workspace/sync-to/finalize",
			expectedID:     "agent-123",
			expectedAction: "workspace/sync-to/finalize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			id, action := extractAction(req, "/api/v1/agents")

			if id != tt.expectedID {
				t.Errorf("extractAction() id = %q, want %q", id, tt.expectedID)
			}
			if action != tt.expectedAction {
				t.Errorf("extractAction() action = %q, want %q", action, tt.expectedAction)
			}
		})
	}
}

func TestWorkspaceStatusHandler(t *testing.T) {
	srv, s := testWorkspaceServer(t)
	ctx := context.Background()

	now := time.Now()

	// Create the grove first (foreign key dependency)
	createTestGrove(t, s, "grove_test_1")

	// Create a test agent
	agent := &store.Agent{
		ID:           "agent_workspace_test_1",
		AgentID:      "workspace-test-agent",
		Name:         "test-agent",
		GroveID:      "grove_test_1",
		Status:       store.AgentStatusRunning,
		StateVersion: 1,
		Created:      now,
		Updated:      now,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Test workspace status endpoint
	req := httptest.NewRequest("GET", "/api/v1/agents/agent_workspace_test_1/workspace", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkspaceDevToken)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("workspace status returned status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp WorkspaceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AgentID != "agent_workspace_test_1" {
		t.Errorf("response AgentID = %q, want %q", resp.AgentID, "agent_workspace_test_1")
	}
	if resp.GroveID != "grove_test_1" {
		t.Errorf("response GroveID = %q, want %q", resp.GroveID, "grove_test_1")
	}
}

func TestWorkspaceStatusHandler_AgentNotFound(t *testing.T) {
	srv, _ := testWorkspaceServer(t)

	req := httptest.NewRequest("GET", "/api/v1/agents/nonexistent/workspace", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkspaceDevToken)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("workspace status for nonexistent agent returned status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWorkspaceSyncFromHandler_AgentNotRunning(t *testing.T) {
	srv, s := testWorkspaceServer(t)
	ctx := context.Background()
	now := time.Now()

	// Create the grove first
	createTestGrove(t, s, "grove_test")

	// Create a stopped agent
	agent := &store.Agent{
		ID:           "agent_stopped_1",
		AgentID:      "stopped-agent",
		Name:         "stopped-agent",
		GroveID:      "grove_test",
		Status:       store.AgentStatusStopped,
		StateVersion: 1,
		Created:      now,
		Updated:      now,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/agents/agent_stopped_1/workspace/sync-from", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkspaceDevToken)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	// Should return 409 Conflict because agent is not running
	if rec.Code != http.StatusConflict {
		t.Errorf("sync-from for stopped agent returned status %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestWorkspaceSyncToHandler_EmptyFiles(t *testing.T) {
	srv, s := testWorkspaceServer(t)
	ctx := context.Background()
	now := time.Now()

	// Create the grove first
	createTestGrove(t, s, "grove_syncto")

	agent := &store.Agent{
		ID:           "agent_syncto_test",
		AgentID:      "sync-to-test-agent",
		Name:         "test-agent",
		GroveID:      "grove_syncto",
		Status:       store.AgentStatusRunning,
		StateVersion: 1,
		Created:      now,
		Updated:      now,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Send request with empty files list
	body := `{"files": []}`
	req := httptest.NewRequest("POST", "/api/v1/agents/agent_syncto_test/workspace/sync-to", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testWorkspaceDevToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	// Should return 400 Bad Request because files list is required
	if rec.Code != http.StatusBadRequest {
		t.Errorf("sync-to with empty files returned status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWorkspaceSyncToFinalizeHandler_MissingManifest(t *testing.T) {
	srv, s := testWorkspaceServer(t)
	ctx := context.Background()
	now := time.Now()

	// Create the grove first
	createTestGrove(t, s, "grove_finalize")

	agent := &store.Agent{
		ID:           "agent_finalize_test",
		AgentID:      "finalize-test-agent",
		Name:         "test-agent",
		GroveID:      "grove_finalize",
		Status:       store.AgentStatusRunning,
		StateVersion: 1,
		Created:      now,
		Updated:      now,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Send request without manifest
	body := `{}`
	req := httptest.NewRequest("POST", "/api/v1/agents/agent_finalize_test/workspace/sync-to/finalize", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testWorkspaceDevToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	// Should return 400 Bad Request because manifest is required
	if rec.Code != http.StatusBadRequest {
		t.Errorf("finalize without manifest returned status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWorkspaceRoutesRequireAuth(t *testing.T) {
	srv, s := testWorkspaceServer(t)
	ctx := context.Background()
	now := time.Now()

	// Create the grove first
	createTestGrove(t, s, "grove_auth")

	agent := &store.Agent{
		ID:           "agent_auth_test",
		AgentID:      "auth-test-agent",
		Name:         "test-agent",
		GroveID:      "grove_auth",
		Status:       store.AgentStatusRunning,
		StateVersion: 1,
		Created:      now,
		Updated:      now,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	tests := []struct {
		name   string
		method string
		url    string
	}{
		{"workspace status", "GET", "/api/v1/agents/agent_auth_test/workspace"},
		{"sync-from", "POST", "/api/v1/agents/agent_auth_test/workspace/sync-from"},
		{"sync-to", "POST", "/api/v1/agents/agent_auth_test/workspace/sync-to"},
		{"sync-to finalize", "POST", "/api/v1/agents/agent_auth_test/workspace/sync-to/finalize"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			// No authorization header
			rec := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rec, req)

			// Should return 401 Unauthorized (no auth token provided)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s without auth returned status %d, want %d", tt.name, rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestSyncFromResponse_JSONSerialization(t *testing.T) {
	resp := SyncFromResponse{
		Manifest: &transfer.Manifest{
			Version:     "1.0",
			ContentHash: "sha256:abc123",
			Files: []transfer.FileInfo{
				{Path: "src/main.go", Size: 1024, Hash: "sha256:def456"},
			},
		},
		DownloadURLs: []transfer.DownloadURLInfo{
			{Path: "src/main.go", URL: "https://storage.example.com/file", Size: 1024, Hash: "sha256:def456"},
		},
		Expires: time.Date(2026, 2, 3, 10, 45, 0, 0, time.UTC),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal SyncFromResponse: %v", err)
	}

	var parsed SyncFromResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal SyncFromResponse: %v", err)
	}

	if parsed.Manifest.Version != "1.0" {
		t.Errorf("manifest version = %q, want %q", parsed.Manifest.Version, "1.0")
	}
	if len(parsed.DownloadURLs) != 1 {
		t.Errorf("download URLs count = %d, want 1", len(parsed.DownloadURLs))
	}
}

func TestSyncToResponse_JSONSerialization(t *testing.T) {
	resp := SyncToResponse{
		UploadURLs: []transfer.UploadURLInfo{
			{
				Path:   "src/main.go",
				URL:    "https://storage.example.com/upload",
				Method: "PUT",
				Headers: map[string]string{
					"Content-Type": "application/octet-stream",
				},
			},
		},
		ExistingFiles: []string{"README.md"},
		Expires:       time.Date(2026, 2, 3, 10, 45, 0, 0, time.UTC),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal SyncToResponse: %v", err)
	}

	var parsed SyncToResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal SyncToResponse: %v", err)
	}

	if len(parsed.UploadURLs) != 1 {
		t.Errorf("upload URLs count = %d, want 1", len(parsed.UploadURLs))
	}
	if len(parsed.ExistingFiles) != 1 || parsed.ExistingFiles[0] != "README.md" {
		t.Errorf("existing files = %v, want [README.md]", parsed.ExistingFiles)
	}
}

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

//go:build !no_sqlite

package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

// ============================================================================
// GitHub App Config Endpoints
// ============================================================================

func TestHandleGetGitHubApp(t *testing.T) {
	srv, _ := testServer(t)

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/github-app", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp GitHubAppConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Configured {
		t.Error("expected configured=false for default config")
	}
}

func TestHandleUpdateGitHubApp(t *testing.T) {
	srv, _ := testServer(t)

	appID := int64(12345)
	apiURL := "https://github.example.com/api/v3"
	webhooksEnabled := true

	rec := doRequest(t, srv, http.MethodPut, "/api/v1/github-app", GitHubAppConfigUpdateRequest{
		AppID:           &appID,
		APIBaseURL:      &apiURL,
		WebhooksEnabled: &webhooksEnabled,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp GitHubAppConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AppID != 12345 {
		t.Errorf("expected app_id 12345, got %d", resp.AppID)
	}
	if resp.APIBaseURL != apiURL {
		t.Errorf("expected api_base_url %s, got %s", apiURL, resp.APIBaseURL)
	}
	if !resp.WebhooksEnabled {
		t.Error("expected webhooks_enabled true")
	}
}

func TestHandleGitHubApp_MethodNotAllowed(t *testing.T) {
	srv, _ := testServer(t)

	rec := doRequest(t, srv, http.MethodDelete, "/api/v1/github-app", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// ============================================================================
// GitHub App Installation Endpoints
// ============================================================================

func TestHandleGitHubAppInstallations_CRUD(t *testing.T) {
	srv, _ := testServer(t)

	// Create installation
	rec := doRequest(t, srv, http.MethodPost, "/api/v1/github-app/installations", map[string]interface{}{
		"installation_id": 12345,
		"account_login":   "acme-org",
		"account_type":    "Organization",
		"app_id":          42,
		"repositories":    []string{"widgets", "api"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List installations
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/github-app/installations", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		Installations []store.GitHubInstallation `json:"installations"`
		Total         int                        `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 {
		t.Errorf("expected 1 installation, got %d", listResp.Total)
	}
	if listResp.Installations[0].AccountLogin != "acme-org" {
		t.Errorf("expected acme-org, got %s", listResp.Installations[0].AccountLogin)
	}

	// Get by ID
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/github-app/installations/12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var inst store.GitHubInstallation
	if err := json.NewDecoder(rec.Body).Decode(&inst); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if inst.InstallationID != 12345 {
		t.Errorf("expected installation_id 12345, got %d", inst.InstallationID)
	}

	// Delete
	rec = doRequest(t, srv, http.MethodDelete, "/api/v1/github-app/installations/12345", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/github-app/installations/12345", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestHandleGitHubAppInstallations_ValidationErrors(t *testing.T) {
	srv, _ := testServer(t)

	// Missing installation_id
	rec := doRequest(t, srv, http.MethodPost, "/api/v1/github-app/installations", map[string]interface{}{
		"account_login": "acme",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing installation_id, got %d", rec.Code)
	}

	// Missing account_login
	rec = doRequest(t, srv, http.MethodPost, "/api/v1/github-app/installations", map[string]interface{}{
		"installation_id": 123,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing account_login, got %d", rec.Code)
	}
}

// ============================================================================
// Grove GitHub Installation Association
// ============================================================================

func TestHandleGroveGitHubInstallation(t *testing.T) {
	srv, s := testServer(t)
	ctx := context.Background()

	// Create a grove
	grove := &store.Grove{
		ID:         "grove_gh_test",
		Slug:       "gh-test-grove",
		Name:       "GH Test Grove",
		GitRemote:  "https://github.com/acme/widgets",
		Created:    time.Now(),
		Updated:    time.Now(),
		Visibility: "private",
	}
	if err := s.CreateGrove(ctx, grove); err != nil {
		t.Fatalf("failed to create grove: %v", err)
	}

	// Create an installation
	inst := &store.GitHubInstallation{
		InstallationID: 54321,
		AccountLogin:   "acme",
		AccountType:    "Organization",
		AppID:          42,
		Status:         store.GitHubInstallationStatusActive,
	}
	if err := s.CreateGitHubInstallation(ctx, inst); err != nil {
		t.Fatalf("failed to create installation: %v", err)
	}

	// Associate grove with installation
	rec := doRequest(t, srv, http.MethodPut, "/api/v1/groves/grove_gh_test/github-installation", map[string]interface{}{
		"installation_id": 54321,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify grove has installation ID
	updatedGrove, err := s.GetGrove(ctx, "grove_gh_test")
	if err != nil {
		t.Fatalf("failed to get grove: %v", err)
	}
	if updatedGrove.GitHubInstallationID == nil || *updatedGrove.GitHubInstallationID != 54321 {
		t.Errorf("expected installation_id 54321, got %v", updatedGrove.GitHubInstallationID)
	}
	if updatedGrove.GitHubAppStatus == nil || updatedGrove.GitHubAppStatus.State != store.GitHubAppStateUnchecked {
		t.Error("expected unchecked status after association")
	}

	// Get status
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/groves/grove_gh_test/github-status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Remove association
	rec = doRequest(t, srv, http.MethodDelete, "/api/v1/groves/grove_gh_test/github-installation", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify removed
	clearedGrove, err := s.GetGrove(ctx, "grove_gh_test")
	if err != nil {
		t.Fatalf("failed to get grove: %v", err)
	}
	if clearedGrove.GitHubInstallationID != nil {
		t.Error("expected nil installation_id after removal")
	}
}

func TestHandleGroveGitHubInstallation_NotFoundInstallation(t *testing.T) {
	srv, s := testServer(t)
	ctx := context.Background()

	grove := &store.Grove{
		ID: "grove_gh_notfound", Slug: "gh-nf", Name: "GH NF",
		Created: time.Now(), Updated: time.Now(), Visibility: "private",
	}
	if err := s.CreateGrove(ctx, grove); err != nil {
		t.Fatalf("failed to create grove: %v", err)
	}

	rec := doRequest(t, srv, http.MethodPut, "/api/v1/groves/grove_gh_notfound/github-installation", map[string]interface{}{
		"installation_id": 99999,
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent installation, got %d", rec.Code)
	}
}

// ============================================================================
// Grove GitHub Permissions
// ============================================================================

func TestHandleGroveGitHubPermissions(t *testing.T) {
	srv, s := testServer(t)
	ctx := context.Background()

	grove := &store.Grove{
		ID: "grove_gh_perms", Slug: "gh-perms", Name: "GH Perms",
		Created: time.Now(), Updated: time.Now(), Visibility: "private",
	}
	if err := s.CreateGrove(ctx, grove); err != nil {
		t.Fatalf("failed to create grove: %v", err)
	}

	// Get defaults
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/groves/grove_gh_perms/github-permissions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var perms store.GitHubTokenPermissions
	if err := json.NewDecoder(rec.Body).Decode(&perms); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if perms.Contents != "write" {
		t.Errorf("expected default contents:write, got %s", perms.Contents)
	}

	// Set custom permissions
	rec = doRequest(t, srv, http.MethodPut, "/api/v1/groves/grove_gh_perms/github-permissions", map[string]interface{}{
		"contents": "read",
		"metadata": "read",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify stored
	updatedGrove, err := s.GetGrove(ctx, "grove_gh_perms")
	if err != nil {
		t.Fatalf("failed to get grove: %v", err)
	}
	if updatedGrove.GitHubPermissions == nil || updatedGrove.GitHubPermissions.Contents != "read" {
		t.Error("expected custom contents:read permission")
	}

	// Reset to defaults
	rec = doRequest(t, srv, http.MethodDelete, "/api/v1/groves/grove_gh_perms/github-permissions", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	clearedGrove, err := s.GetGrove(ctx, "grove_gh_perms")
	if err != nil {
		t.Fatalf("failed to get grove: %v", err)
	}
	if clearedGrove.GitHubPermissions != nil {
		t.Error("expected nil permissions after reset")
	}
}

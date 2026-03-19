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

package hub

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

// GitHubAppConfigResponse is the API response for GitHub App configuration.
// Sensitive fields (private key, webhook secret) are never returned.
type GitHubAppConfigResponse struct {
	AppID           int64  `json:"app_id"`
	APIBaseURL      string `json:"api_base_url,omitempty"`
	WebhooksEnabled bool   `json:"webhooks_enabled"`
	Configured      bool   `json:"configured"`
}

// GitHubAppConfigUpdateRequest is the API request to update GitHub App configuration.
type GitHubAppConfigUpdateRequest struct {
	AppID           *int64  `json:"app_id,omitempty"`
	PrivateKey      *string `json:"private_key,omitempty"`
	WebhookSecret   *string `json:"webhook_secret,omitempty"`
	APIBaseURL      *string `json:"api_base_url,omitempty"`
	WebhooksEnabled *bool   `json:"webhooks_enabled,omitempty"`
}

// handleGitHubApp handles GET and PUT /api/v1/github-app.
func (s *Server) handleGitHubApp(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetGitHubApp(w, r)
	case http.MethodPut:
		s.handleUpdateGitHubApp(w, r)
	default:
		MethodNotAllowed(w)
	}
}

func (s *Server) handleGetGitHubApp(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config.GitHubAppConfig
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, GitHubAppConfigResponse{
		AppID:           cfg.AppID,
		APIBaseURL:      cfg.APIBaseURL,
		WebhooksEnabled: cfg.WebhooksEnabled,
		Configured:      cfg.AppID != 0 && (cfg.PrivateKeyPath != "" || cfg.PrivateKey != ""),
	})
}

func (s *Server) handleUpdateGitHubApp(w http.ResponseWriter, r *http.Request) {
	var req GitHubAppConfigUpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid request body", nil)
		return
	}

	s.mu.Lock()
	if req.AppID != nil {
		s.config.GitHubAppConfig.AppID = *req.AppID
	}
	if req.PrivateKey != nil {
		s.config.GitHubAppConfig.PrivateKey = *req.PrivateKey
	}
	if req.WebhookSecret != nil {
		s.config.GitHubAppConfig.WebhookSecret = *req.WebhookSecret
	}
	if req.APIBaseURL != nil {
		s.config.GitHubAppConfig.APIBaseURL = *req.APIBaseURL
	}
	if req.WebhooksEnabled != nil {
		s.config.GitHubAppConfig.WebhooksEnabled = *req.WebhooksEnabled
	}
	cfg := s.config.GitHubAppConfig
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, GitHubAppConfigResponse{
		AppID:           cfg.AppID,
		APIBaseURL:      cfg.APIBaseURL,
		WebhooksEnabled: cfg.WebhooksEnabled,
		Configured:      cfg.AppID != 0 && (cfg.PrivateKeyPath != "" || cfg.PrivateKey != ""),
	})
}

// handleGitHubAppInstallations handles GET and POST /api/v1/github-app/installations.
func (s *Server) handleGitHubAppInstallations(w http.ResponseWriter, r *http.Request) {
	// Check if this is a sub-route (e.g., /api/v1/github-app/installations/{id})
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/github-app/installations")
	if path != "" && path != "/" {
		subPath := strings.TrimPrefix(path, "/")
		subPath = strings.TrimSuffix(subPath, "/")

		// Handle /discover sub-route
		if subPath == "discover" {
			s.handleGitHubAppDiscover(w, r)
			return
		}

		// Sub-route: /api/v1/github-app/installations/{id}
		installationID, err := strconv.ParseInt(subPath, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid installation ID", nil)
			return
		}
		s.handleGitHubAppInstallationByID(w, r, installationID)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleListGitHubAppInstallations(w, r)
	case http.MethodPost:
		s.handleCreateGitHubAppInstallation(w, r)
	default:
		MethodNotAllowed(w)
	}
}

func (s *Server) handleListGitHubAppInstallations(w http.ResponseWriter, r *http.Request) {
	filter := store.GitHubInstallationFilter{
		AccountLogin: r.URL.Query().Get("account_login"),
		Status:       r.URL.Query().Get("status"),
	}

	installations, err := s.store.ListGitHubInstallations(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list installations", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"installations": installations,
		"total":         len(installations),
	})
}

func (s *Server) handleCreateGitHubAppInstallation(w http.ResponseWriter, r *http.Request) {
	var installation store.GitHubInstallation
	if err := readJSON(r, &installation); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid request body", nil)
		return
	}

	if installation.InstallationID == 0 {
		writeError(w, http.StatusBadRequest, ErrCodeValidationError, "installation_id is required", nil)
		return
	}
	if installation.AccountLogin == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidationError, "account_login is required", nil)
		return
	}

	// Set app_id from server config if not provided
	if installation.AppID == 0 {
		s.mu.RLock()
		installation.AppID = s.config.GitHubAppConfig.AppID
		s.mu.RUnlock()
	}

	if err := s.store.CreateGitHubInstallation(r.Context(), &installation); err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create installation", nil)
		return
	}

	writeJSON(w, http.StatusCreated, installation)
}

func (s *Server) handleGitHubAppInstallationByID(w http.ResponseWriter, r *http.Request, installationID int64) {
	switch r.Method {
	case http.MethodGet:
		installation, err := s.store.GetGitHubInstallation(r.Context(), installationID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "installation not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get installation", nil)
			return
		}
		writeJSON(w, http.StatusOK, installation)

	case http.MethodPut:
		var installation store.GitHubInstallation
		if err := readJSON(r, &installation); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid request body", nil)
			return
		}
		installation.InstallationID = installationID

		if err := s.store.UpdateGitHubInstallation(r.Context(), &installation); err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "installation not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update installation", nil)
			return
		}
		writeJSON(w, http.StatusOK, installation)

	case http.MethodDelete:
		if err := s.store.DeleteGitHubInstallation(r.Context(), installationID); err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "installation not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete installation", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		MethodNotAllowed(w)
	}
}

// handleGroveGitHubInstallation handles PUT and DELETE /api/v1/groves/{id}/github-installation.
func (s *Server) handleGroveGitHubInstallation(w http.ResponseWriter, r *http.Request, groveID string) {
	switch r.Method {
	case http.MethodPut:
		var req struct {
			InstallationID int64 `json:"installation_id"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid request body", nil)
			return
		}
		if req.InstallationID == 0 {
			writeError(w, http.StatusBadRequest, ErrCodeValidationError, "installation_id is required", nil)
			return
		}

		// Verify installation exists
		if _, err := s.store.GetGitHubInstallation(r.Context(), req.InstallationID); err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "installation not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to verify installation", nil)
			return
		}

		grove, err := s.store.GetGrove(r.Context(), groveID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
			return
		}

		grove.GitHubInstallationID = &req.InstallationID
		grove.GitHubAppStatus = &store.GitHubAppGroveStatus{
			State:       store.GitHubAppStateUnchecked,
			LastChecked: timeNow(),
		}

		if err := s.store.UpdateGrove(r.Context(), grove); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update grove", nil)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"grove_id":        groveID,
			"installation_id": req.InstallationID,
			"status":          "associated",
		})

	case http.MethodDelete:
		grove, err := s.store.GetGrove(r.Context(), groveID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
			return
		}

		grove.GitHubInstallationID = nil
		grove.GitHubAppStatus = nil

		if err := s.store.UpdateGrove(r.Context(), grove); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update grove", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		MethodNotAllowed(w)
	}
}

// handleGroveGitHubStatus handles GET /api/v1/groves/{id}/github-status.
func (s *Server) handleGroveGitHubStatus(w http.ResponseWriter, r *http.Request, groveID string) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w)
		return
	}

	grove, err := s.store.GetGrove(r.Context(), groveID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
		return
	}

	resp := map[string]interface{}{
		"grove_id":        groveID,
		"installation_id": grove.GitHubInstallationID,
		"status":          grove.GitHubAppStatus,
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGroveGitHubPermissions handles GET, PUT, DELETE /api/v1/groves/{id}/github-permissions.
func (s *Server) handleGroveGitHubPermissions(w http.ResponseWriter, r *http.Request, groveID string) {
	switch r.Method {
	case http.MethodGet:
		grove, err := s.store.GetGrove(r.Context(), groveID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
			return
		}

		perms := grove.GitHubPermissions
		if perms == nil {
			// Return defaults
			perms = &store.GitHubTokenPermissions{
				Contents:     "write",
				PullRequests: "write",
				Metadata:     "read",
			}
		}
		writeJSON(w, http.StatusOK, perms)

	case http.MethodPut:
		var perms store.GitHubTokenPermissions
		if err := readJSON(r, &perms); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "invalid request body", nil)
			return
		}

		grove, err := s.store.GetGrove(r.Context(), groveID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
			return
		}

		grove.GitHubPermissions = &perms
		if err := s.store.UpdateGrove(r.Context(), grove); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update grove", nil)
			return
		}

		writeJSON(w, http.StatusOK, perms)

	case http.MethodDelete:
		grove, err := s.store.GetGrove(r.Context(), groveID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "grove not found", nil)
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get grove", nil)
			return
		}

		grove.GitHubPermissions = nil
		if err := s.store.UpdateGrove(r.Context(), grove); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update grove", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		MethodNotAllowed(w)
	}
}

// timeNow is a helper that returns the current time. Can be overridden in tests.
var timeNow = func() time.Time { return time.Now() }

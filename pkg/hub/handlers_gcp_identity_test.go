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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestGroveForSA(t *testing.T, srv *Server, s store.Store) string {
	t.Helper()
	rec := doRequest(t, srv, http.MethodPost, "/api/v1/groves", map[string]string{
		"name": "test-grove-sa",
	})
	require.Equal(t, http.StatusCreated, rec.Code, "create grove: %s", rec.Body.String())
	var grove store.Grove
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&grove))
	return grove.ID
}

func TestCreateGCPServiceAccount_Success(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	body := map[string]string{
		"email":      "agent@my-project.iam.gserviceaccount.com",
		"project_id": "my-project",
	}

	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var sa store.GCPServiceAccount
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&sa))
	assert.Equal(t, "agent@my-project.iam.gserviceaccount.com", sa.Email)
	assert.Equal(t, "my-project", sa.ProjectID)
	assert.NotEmpty(t, sa.ID)
}

func TestCreateGCPServiceAccount_MissingEmail(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	body := map[string]string{
		"project_id": "my-project",
	}

	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, ErrCodeInvalidRequest, errResp.Error.Code)
	assert.Contains(t, errResp.Error.Message, "email")
}

func TestCreateGCPServiceAccount_ProjectIDAutoExtracted(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	body := map[string]string{
		"email": "agent@my-project.iam.gserviceaccount.com",
	}

	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var sa store.GCPServiceAccount
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&sa))
	assert.Equal(t, "agent@my-project.iam.gserviceaccount.com", sa.Email)
	assert.Equal(t, "my-project", sa.ProjectID, "project_id should be auto-extracted from email")
}

func TestCreateGCPServiceAccount_ProjectIDExplicit(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	body := map[string]string{
		"email":      "agent@my-project.iam.gserviceaccount.com",
		"project_id": "override-project",
	}

	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var sa store.GCPServiceAccount
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&sa))
	assert.Equal(t, "override-project", sa.ProjectID, "explicit project_id should take precedence")
}

func TestCreateGCPServiceAccount_EmptyBody(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), map[string]string{})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, ErrCodeInvalidRequest, errResp.Error.Code)
	assert.Contains(t, errResp.Error.Message, "email")
}

func TestCreateGCPServiceAccount_InvalidJSON(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	rec := doRequestRaw(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID),
		[]byte("not-json"), "application/json")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, ErrCodeInvalidRequest, errResp.Error.Code)
	assert.Contains(t, errResp.Error.Message, "invalid request body")
}

func TestCreateGCPServiceAccount_GroveNotFound(t *testing.T) {
	srv, _ := testServer(t)

	body := map[string]string{
		"email":      "agent@my-project.iam.gserviceaccount.com",
		"project_id": "my-project",
	}

	rec := doRequest(t, srv, http.MethodPost,
		"/api/v1/groves/nonexistent-grove-id/gcp-service-accounts", body)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateGCPServiceAccount_Duplicate(t *testing.T) {
	srv, s := testServer(t)
	groveID := createTestGroveForSA(t, srv, s)

	body := map[string]string{
		"email":      "agent@my-project.iam.gserviceaccount.com",
		"project_id": "my-project",
	}

	// First create should succeed
	rec := doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusCreated, rec.Code, "first create: %s", rec.Body.String())

	// Second create with same email should conflict
	rec = doRequest(t, srv, http.MethodPost,
		fmt.Sprintf("/api/v1/groves/%s/gcp-service-accounts", groveID), body)
	require.Equal(t, http.StatusConflict, rec.Code)

	var errResp ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, ErrCodeConflict, errResp.Error.Code)
}

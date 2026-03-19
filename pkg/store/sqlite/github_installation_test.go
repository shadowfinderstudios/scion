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

package sqlite

import (
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

func TestGitHubInstallation_CRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	inst := &store.GitHubInstallation{
		InstallationID: 12345,
		AccountLogin:   "acme-org",
		AccountType:    "Organization",
		AppID:          42,
		Repositories:   []string{"widgets", "api"},
		Status:         store.GitHubInstallationStatusActive,
	}

	// Create
	if err := s.CreateGitHubInstallation(ctx, inst); err != nil {
		t.Fatalf("CreateGitHubInstallation failed: %v", err)
	}

	// Get
	got, err := s.GetGitHubInstallation(ctx, 12345)
	if err != nil {
		t.Fatalf("GetGitHubInstallation failed: %v", err)
	}
	if got.AccountLogin != "acme-org" {
		t.Errorf("expected account_login acme-org, got %s", got.AccountLogin)
	}
	if got.AccountType != "Organization" {
		t.Errorf("expected account_type Organization, got %s", got.AccountType)
	}
	if got.AppID != 42 {
		t.Errorf("expected app_id 42, got %d", got.AppID)
	}
	if len(got.Repositories) != 2 || got.Repositories[0] != "widgets" {
		t.Errorf("expected repos [widgets, api], got %v", got.Repositories)
	}
	if got.Status != store.GitHubInstallationStatusActive {
		t.Errorf("expected status active, got %s", got.Status)
	}

	// Update
	got.Status = store.GitHubInstallationStatusSuspended
	got.Repositories = []string{"widgets"}
	if err := s.UpdateGitHubInstallation(ctx, got); err != nil {
		t.Fatalf("UpdateGitHubInstallation failed: %v", err)
	}

	updated, err := s.GetGitHubInstallation(ctx, 12345)
	if err != nil {
		t.Fatalf("GetGitHubInstallation after update failed: %v", err)
	}
	if updated.Status != store.GitHubInstallationStatusSuspended {
		t.Errorf("expected status suspended, got %s", updated.Status)
	}
	if len(updated.Repositories) != 1 {
		t.Errorf("expected 1 repo, got %d", len(updated.Repositories))
	}

	// Delete
	if err := s.DeleteGitHubInstallation(ctx, 12345); err != nil {
		t.Fatalf("DeleteGitHubInstallation failed: %v", err)
	}

	_, err = s.GetGitHubInstallation(ctx, 12345)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGitHubInstallation_CreateIdempotent(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	inst := &store.GitHubInstallation{
		InstallationID: 11111,
		AccountLogin:   "alice",
		AccountType:    "User",
		AppID:          42,
		Status:         store.GitHubInstallationStatusActive,
	}

	if err := s.CreateGitHubInstallation(ctx, inst); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Second create should be a no-op (INSERT OR IGNORE)
	if err := s.CreateGitHubInstallation(ctx, inst); err != nil {
		t.Fatalf("second create should not fail: %v", err)
	}
}

func TestGitHubInstallation_List(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create several installations
	for i, login := range []string{"org-a", "org-b", "user-c"} {
		inst := &store.GitHubInstallation{
			InstallationID: int64(100 + i),
			AccountLogin:   login,
			AccountType:    "Organization",
			AppID:          42,
			Status:         store.GitHubInstallationStatusActive,
		}
		if login == "user-c" {
			inst.AccountType = "User"
			inst.Status = store.GitHubInstallationStatusSuspended
		}
		if err := s.CreateGitHubInstallation(ctx, inst); err != nil {
			t.Fatalf("CreateGitHubInstallation failed for %s: %v", login, err)
		}
	}

	// List all
	all, err := s.ListGitHubInstallations(ctx, store.GitHubInstallationFilter{})
	if err != nil {
		t.Fatalf("ListGitHubInstallations failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 installations, got %d", len(all))
	}

	// Filter by status
	active, err := s.ListGitHubInstallations(ctx, store.GitHubInstallationFilter{Status: "active"})
	if err != nil {
		t.Fatalf("ListGitHubInstallations with status filter failed: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active installations, got %d", len(active))
	}

	// Filter by account login
	byAccount, err := s.ListGitHubInstallations(ctx, store.GitHubInstallationFilter{AccountLogin: "org-a"})
	if err != nil {
		t.Fatalf("ListGitHubInstallations with account filter failed: %v", err)
	}
	if len(byAccount) != 1 {
		t.Errorf("expected 1 installation for org-a, got %d", len(byAccount))
	}
}

func TestGitHubInstallation_NotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	_, err := s.GetGitHubInstallation(ctx, 99999)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = s.UpdateGitHubInstallation(ctx, &store.GitHubInstallation{InstallationID: 99999})
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound on update, got %v", err)
	}

	err = s.DeleteGitHubInstallation(ctx, 99999)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound on delete, got %v", err)
	}
}

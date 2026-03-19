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

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

func (s *SQLiteStore) CreateGitHubInstallation(ctx context.Context, installation *store.GitHubInstallation) error {
	if installation.CreatedAt.IsZero() {
		installation.CreatedAt = time.Now()
	}
	if installation.UpdatedAt.IsZero() {
		installation.UpdatedAt = installation.CreatedAt
	}
	if installation.Status == "" {
		installation.Status = store.GitHubInstallationStatusActive
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO github_installations (installation_id, account_login, account_type, app_id, repositories, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		installation.InstallationID, installation.AccountLogin, installation.AccountType,
		installation.AppID, marshalJSON(installation.Repositories),
		installation.Status, installation.CreatedAt, installation.UpdatedAt,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) GetGitHubInstallation(ctx context.Context, installationID int64) (*store.GitHubInstallation, error) {
	var inst store.GitHubInstallation
	var repos string

	err := s.db.QueryRowContext(ctx, `
		SELECT installation_id, account_login, account_type, app_id, repositories, status, created_at, updated_at
		FROM github_installations WHERE installation_id = ?`, installationID,
	).Scan(&inst.InstallationID, &inst.AccountLogin, &inst.AccountType,
		&inst.AppID, &repos, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	unmarshalJSON(repos, &inst.Repositories)
	return &inst, nil
}

func (s *SQLiteStore) UpdateGitHubInstallation(ctx context.Context, installation *store.GitHubInstallation) error {
	installation.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(ctx, `
		UPDATE github_installations SET
			account_login = ?, account_type = ?, app_id = ?,
			repositories = ?, status = ?, updated_at = ?
		WHERE installation_id = ?`,
		installation.AccountLogin, installation.AccountType, installation.AppID,
		marshalJSON(installation.Repositories), installation.Status, installation.UpdatedAt,
		installation.InstallationID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteGitHubInstallation(ctx context.Context, installationID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM github_installations WHERE installation_id = ?`, installationID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListGitHubInstallations(ctx context.Context, filter store.GitHubInstallationFilter) ([]store.GitHubInstallation, error) {
	query := "SELECT installation_id, account_login, account_type, app_id, repositories, status, created_at, updated_at FROM github_installations WHERE 1=1"
	var args []interface{}

	if filter.AccountLogin != "" {
		query += " AND account_login = ?"
		args = append(args, filter.AccountLogin)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.AppID != 0 {
		query += " AND app_id = ?"
		args = append(args, filter.AppID)
	}

	query += " ORDER BY created_at ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.GitHubInstallation
	for rows.Next() {
		var inst store.GitHubInstallation
		var repos string

		if err := rows.Scan(&inst.InstallationID, &inst.AccountLogin, &inst.AccountType,
			&inst.AppID, &repos, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, err
		}

		unmarshalJSON(repos, &inst.Repositories)
		results = append(results, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Ensure we never return nil slice (return empty slice instead)
	if results == nil {
		results = []store.GitHubInstallation{}
	}

	return results, nil
}

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

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

// ============================================================================
// Grove Sync State Operations
// ============================================================================

// UpsertGroveSyncState creates or updates sync state for a grove.
func (s *SQLiteStore) UpsertGroveSyncState(ctx context.Context, state *store.GroveSyncState) error {
	if state.GroveID == "" {
		return store.ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO grove_sync_state (grove_id, broker_id, last_sync_time, last_commit_sha, file_count, total_bytes)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(grove_id, broker_id) DO UPDATE SET
			last_sync_time = excluded.last_sync_time,
			last_commit_sha = excluded.last_commit_sha,
			file_count = excluded.file_count,
			total_bytes = excluded.total_bytes
	`, state.GroveID, state.BrokerID,
		nullableTimePtr(state.LastSyncTime),
		nullableString(state.LastCommitSHA),
		state.FileCount, state.TotalBytes,
	)
	return err
}

// GetGroveSyncState retrieves sync state for a grove and optional broker.
func (s *SQLiteStore) GetGroveSyncState(ctx context.Context, groveID, brokerID string) (*store.GroveSyncState, error) {
	state := &store.GroveSyncState{}
	var lastSyncTime sql.NullTime
	var lastCommitSHA sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT grove_id, broker_id, last_sync_time, last_commit_sha, file_count, total_bytes
		FROM grove_sync_state
		WHERE grove_id = ? AND broker_id = ?
	`, groveID, brokerID).Scan(
		&state.GroveID, &state.BrokerID,
		&lastSyncTime, &lastCommitSHA,
		&state.FileCount, &state.TotalBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if lastSyncTime.Valid {
		state.LastSyncTime = &lastSyncTime.Time
	}
	if lastCommitSHA.Valid {
		state.LastCommitSHA = lastCommitSHA.String
	}

	return state, nil
}

// ListGroveSyncStates returns all sync states for a grove.
func (s *SQLiteStore) ListGroveSyncStates(ctx context.Context, groveID string) ([]store.GroveSyncState, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT grove_id, broker_id, last_sync_time, last_commit_sha, file_count, total_bytes
		FROM grove_sync_state
		WHERE grove_id = ?
		ORDER BY broker_id
	`, groveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []store.GroveSyncState
	for rows.Next() {
		var state store.GroveSyncState
		var lastSyncTime sql.NullTime
		var lastCommitSHA sql.NullString

		if err := rows.Scan(
			&state.GroveID, &state.BrokerID,
			&lastSyncTime, &lastCommitSHA,
			&state.FileCount, &state.TotalBytes,
		); err != nil {
			return nil, err
		}

		if lastSyncTime.Valid {
			state.LastSyncTime = &lastSyncTime.Time
		}
		if lastCommitSHA.Valid {
			state.LastCommitSHA = lastCommitSHA.String
		}

		states = append(states, state)
	}

	if states == nil {
		states = []store.GroveSyncState{}
	}
	return states, rows.Err()
}

// DeleteGroveSyncState removes sync state for a grove and optional broker.
func (s *SQLiteStore) DeleteGroveSyncState(ctx context.Context, groveID, brokerID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM grove_sync_state WHERE grove_id = ? AND broker_id = ?
	`, groveID, brokerID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

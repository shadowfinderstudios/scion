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
	"context"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"golang.org/x/net/webdav"
)

// syncExcludePatterns defines directory/file prefixes excluded from WebDAV sync.
// These are matched against the first path component.
var syncExcludePatterns = []string{
	".git",
	".scion",
	"node_modules",
}

// syncExcludeExtensions defines file extensions excluded from WebDAV sync.
var syncExcludeExtensions = []string{
	".env",
}

// handleGroveWebDAV serves a WebDAV endpoint for grove workspace file sync.
// It mounts at /api/v1/groves/{groveId}/dav/ and serves the grove's workspace
// directory with file exclusion filters applied.
func (s *Server) handleGroveWebDAV(w http.ResponseWriter, r *http.Request, groveID, davPath string) {
	ctx := r.Context()

	grove, err := s.store.GetGrove(ctx, groveID)
	if err != nil {
		writeErrorFromErr(w, err, "")
		return
	}

	// Only hub-native groves and shared-workspace git groves have a managed workspace
	if grove.GitRemote != "" && !grove.IsSharedWorkspace() {
		Conflict(w, "Workspace sync is only available for hub-native and shared-workspace groves")
		return
	}

	workspacePath, err := hubNativeGrovePath(grove.Slug)
	if err != nil {
		InternalError(w)
		return
	}

	// Ensure the workspace directory exists
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		slog.Error("failed to create grove workspace directory", "grove_id", groveID, "error", err)
		InternalError(w)
		return
	}

	// Build the prefix that the WebDAV handler should strip.
	// The full URL path is /api/v1/groves/{groveId}/dav/...
	// We need to strip everything up to and including /dav
	prefix := "/api/v1/groves/" + r.URL.Path[len("/api/v1/groves/"):len("/api/v1/groves/")+strings.Index(r.URL.Path[len("/api/v1/groves/"):], "/dav/")+len("/dav/")]

	// Simpler approach: reconstruct the prefix from the grove ID raw portion
	prefixEnd := strings.Index(r.URL.Path, "/dav/")
	if prefixEnd == -1 {
		prefixEnd = strings.Index(r.URL.Path, "/dav")
	}
	if prefixEnd == -1 {
		NotFound(w, "WebDAV endpoint")
		return
	}
	prefix = r.URL.Path[:prefixEnd+len("/dav")]

	handler := &webdav.Handler{
		Prefix:     prefix,
		FileSystem: &filteredFS{root: webdav.Dir(workspacePath)},
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				slog.Debug("webdav operation", "method", r.Method, "path", r.URL.Path, "error", err)
			}
		},
	}

	handler.ServeHTTP(w, r)

	// Update sync state after successful write operations
	if r.Method == "PUT" || r.Method == "DELETE" || r.Method == "MKCOL" || r.Method == "MOVE" {
		go s.updateGroveSyncState(grove.ID, workspacePath)
	}
}

// updateGroveSyncState recalculates and persists file count and total bytes for a grove.
func (s *Server) updateGroveSyncState(groveID, workspacePath string) {
	var fileCount int
	var totalBytes int64

	_ = walkFilteredDir(workspacePath, func(relPath string, info os.FileInfo) {
		fileCount++
		totalBytes += info.Size()
	})

	now := time.Now()
	state := &store.GroveSyncState{
		GroveID:      groveID,
		BrokerID:     "", // hub-native
		LastSyncTime: &now,
		FileCount:    fileCount,
		TotalBytes:   totalBytes,
	}

	if err := s.store.UpsertGroveSyncState(context.Background(), state); err != nil {
		slog.Warn("failed to update grove sync state", "grove_id", groveID, "error", err)
	}
}

// walkFilteredDir walks a directory, calling fn for each non-excluded file.
func walkFilteredDir(root string, fn func(relPath string, info os.FileInfo)) error {
	return walkFilteredDirRecursive(root, "", fn)
}

func walkFilteredDirRecursive(root, prefix string, fn func(relPath string, info os.FileInfo)) error {
	fullDir := root
	if prefix != "" {
		fullDir = root + "/" + prefix
	}

	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		relPath := name
		if prefix != "" {
			relPath = prefix + "/" + name
		}

		if isExcluded(relPath) {
			continue
		}

		if entry.IsDir() {
			walkFilteredDirRecursive(root, relPath, fn)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		fn(relPath, info)
	}
	return nil
}

// filteredFS wraps a webdav.FileSystem to exclude sync-excluded paths.
type filteredFS struct {
	root webdav.FileSystem
}

func (fs *filteredFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if isExcluded(name) {
		return os.ErrPermission
	}
	return fs.root.Mkdir(ctx, name, perm)
}

func (fs *filteredFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if isExcluded(name) {
		return nil, os.ErrNotExist
	}

	f, err := fs.root.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return f, err
	}

	// If this is a directory being opened for reading, wrap to filter children
	info, statErr := f.Stat()
	if statErr == nil && info.IsDir() {
		return &filteredDir{File: f, dirName: name}, nil
	}

	return f, nil
}

func (fs *filteredFS) RemoveAll(ctx context.Context, name string) error {
	if isExcluded(name) {
		return os.ErrPermission
	}
	return fs.root.RemoveAll(ctx, name)
}

func (fs *filteredFS) Rename(ctx context.Context, oldName, newName string) error {
	if isExcluded(oldName) || isExcluded(newName) {
		return os.ErrPermission
	}
	return fs.root.Rename(ctx, oldName, newName)
}

func (fs *filteredFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if isExcluded(name) {
		return nil, os.ErrNotExist
	}
	return fs.root.Stat(ctx, name)
}

// filteredDir wraps a webdav.File (directory) to exclude entries from Readdir.
type filteredDir struct {
	webdav.File
	dirName string
}

func (d *filteredDir) Readdir(count int) ([]os.FileInfo, error) {
	entries, err := d.File.Readdir(count)
	if err != nil {
		return entries, err
	}

	filtered := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		childPath := path.Join(d.dirName, entry.Name())
		if !isExcluded(childPath) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

// isExcluded returns true if a path should be excluded from sync.
// name is a slash-separated path relative to the workspace root (may have a leading /).
func isExcluded(name string) bool {
	// Normalize: strip leading slash
	clean := strings.TrimPrefix(path.Clean(name), "/")
	if clean == "" || clean == "." {
		return false
	}

	// Get the top-level component
	topLevel := clean
	if idx := strings.IndexByte(clean, '/'); idx >= 0 {
		topLevel = clean[:idx]
	}

	// Check directory prefix exclusions
	for _, pattern := range syncExcludePatterns {
		if topLevel == pattern {
			return true
		}
	}

	// Check extension exclusions (on the full filename, not just top-level)
	baseName := path.Base(clean)
	for _, ext := range syncExcludeExtensions {
		if strings.HasSuffix(baseName, ext) {
			return true
		}
	}

	return false
}

// GroveSyncStatusResponse is the response for the sync status endpoint.
type GroveSyncStatusResponse struct {
	GroveID    string                 `json:"groveId"`
	States     []store.GroveSyncState `json:"states"`
	TotalFiles int                    `json:"totalFiles"`
	TotalBytes int64                  `json:"totalBytes"`
}

// handleGroveSyncStatus returns the sync status for a grove.
func (s *Server) handleGroveSyncStatus(w http.ResponseWriter, r *http.Request, groveID string) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w)
		return
	}

	ctx := r.Context()

	// Verify grove exists
	_, err := s.store.GetGrove(ctx, groveID)
	if err != nil {
		writeErrorFromErr(w, err, "")
		return
	}

	states, err := s.store.ListGroveSyncStates(ctx, groveID)
	if err != nil {
		InternalError(w)
		return
	}

	var totalFiles int
	var totalBytes int64
	for _, st := range states {
		totalFiles += st.FileCount
		totalBytes += st.TotalBytes
	}

	writeJSON(w, http.StatusOK, GroveSyncStatusResponse{
		GroveID:    groveID,
		States:     states,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	})
}

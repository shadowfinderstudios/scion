package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetGlobalDir(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	dir, err := GetGlobalDir()
	if err != nil {
		t.Fatalf("GetGlobalDir failed: %v", err)
	}
	expected := filepath.Join(tmpHome, GlobalDir)
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestGetGroveName(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		path string
		want string
	}{
		{filepath.Join(tmpDir, "My Project", ".scion"), "my-project"},
		{filepath.Join(tmpDir, "simple", ".scion"), "simple"},
		{filepath.Join(tmpDir, "CamelCase", ".scion"), "camelcase"},
	}

	for _, tt := range tests {
		if err := os.MkdirAll(tt.path, 0755); err != nil {
			t.Fatal(err)
		}
		if got := GetGroveName(tt.path); got != tt.want {
			t.Errorf("GetGroveName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestGetResolvedProjectDir(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	globalDir := filepath.Join(tmpHome, GlobalDir)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		explicit string
		want     string
	}{
		{"home", globalDir},
		{"global", globalDir},
		{tmpHome, tmpHome},
	}

	for _, tt := range tests {
		got, err := GetResolvedProjectDir(tt.explicit)
		if err != nil {
			t.Errorf("GetResolvedProjectDir(%q) error: %v", tt.explicit, err)
			continue
		}
		
		evalGot, _ := filepath.EvalSymlinks(got)
		evalWant, _ := filepath.EvalSymlinks(tt.want)

		if evalGot != evalWant {
			t.Errorf("GetResolvedProjectDir(%q) = %q, want %q", tt.explicit, evalGot, evalWant)
		}
	}
}

func TestGetResolvedProjectDir_WalkUp(t *testing.T) {
	// Create structure:
	// /tmp/grove/.scion
	// /tmp/grove/subdir/deep

	tmpGrove := t.TempDir()
	scionDir := filepath.Join(tmpGrove, ".scion")
	if err := os.Mkdir(scionDir, 0755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpGrove, "subdir", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Set HOME to a clean temp dir so we don't fall back to real global .scion
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	// Expect to find the .scion dir in the parent
	got, err := GetResolvedProjectDir("")
	if err != nil {
		t.Fatalf("GetResolvedProjectDir failed: %v", err)
	}

	evalGot, _ := filepath.EvalSymlinks(got)
	evalScion, _ := filepath.EvalSymlinks(scionDir)

	if evalGot != evalScion {
		t.Errorf("Expected %q, got %q", evalScion, evalGot)
	}
}

func TestRequireGrovePath_ExplicitGlobal(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	globalDir := filepath.Join(tmpHome, GlobalDir)

	// Test "global" path
	got, isGlobal, err := RequireGrovePath("global")
	if err != nil {
		t.Fatalf("RequireGrovePath(global) error: %v", err)
	}
	if !isGlobal {
		t.Error("expected isGlobal=true")
	}
	if got != globalDir {
		t.Errorf("expected %q, got %q", globalDir, got)
	}

	// Test "home" path
	got, isGlobal, err = RequireGrovePath("home")
	if err != nil {
		t.Fatalf("RequireGrovePath(home) error: %v", err)
	}
	if !isGlobal {
		t.Error("expected isGlobal=true")
	}
	if got != globalDir {
		t.Errorf("expected %q, got %q", globalDir, got)
	}
}

func TestRequireGrovePath_NoProjectError(t *testing.T) {
	// Create a clean temp dir with no .scion
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Set HOME to a clean temp dir
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Should error when no project found and no explicit path
	_, _, err := RequireGrovePath("")
	if err == nil {
		t.Fatal("expected error when no project found, got nil")
	}

	// Error message should suggest --global
	if !containsSubstring(err.Error(), "--global") && !containsSubstring(err.Error(), "global") {
		t.Errorf("error should suggest using --global, got: %v", err)
	}
}

func TestRequireGrovePath_ProjectExists(t *testing.T) {
	// Create a project with .scion
	tmpGrove := t.TempDir()
	scionDir := filepath.Join(tmpGrove, ".scion")
	if err := os.Mkdir(scionDir, 0755); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Set HOME to a clean temp dir
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := os.Chdir(tmpGrove); err != nil {
		t.Fatal(err)
	}

	// Should succeed when project found
	got, isGlobal, err := RequireGrovePath("")
	if err != nil {
		t.Fatalf("RequireGrovePath failed: %v", err)
	}
	if isGlobal {
		t.Error("expected isGlobal=false for project grove")
	}

	evalGot, _ := filepath.EvalSymlinks(got)
	evalScion, _ := filepath.EvalSymlinks(scionDir)
	if evalGot != evalScion {
		t.Errorf("expected %q, got %q", evalScion, evalGot)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
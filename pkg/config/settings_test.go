package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSettings(t *testing.T) {
	// Create temporary directories for global and grove settings
	tmpDir := t.TempDir()

	// Mock HOME for global settings
	// We can't easily mock GetGlobalDir() as it likely uses os.UserHomeDir
	// but we can set HOME env var.
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	groveDir := filepath.Join(tmpDir, "my-grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Test defaults
	s, err := LoadSettings(groveDir)
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	if s.DefaultRuntime != "docker" {
		t.Errorf("expected default runtime 'docker', got '%s'", s.DefaultRuntime)
	}

	// 2. Test Global overrides
	globalSettings := `{
		"default_runtime": "kubernetes",
		"kubernetes": {
			"default_namespace": "scion-global"
		}
	}`
	globalScionDir := filepath.Join(tmpDir, ".scion")
	if err := os.MkdirAll(globalScionDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalScionDir, "settings.json"), []byte(globalSettings), 0644); err != nil {
		t.Fatal(err)
	}

	s, err = LoadSettings(groveDir)
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	if s.DefaultRuntime != "kubernetes" {
		t.Errorf("expected global override runtime 'kubernetes', got '%s'", s.DefaultRuntime)
	}
	if s.Kubernetes.DefaultNamespace != "scion-global" {
		t.Errorf("expected global override namespace 'scion-global', got '%s'", s.Kubernetes.DefaultNamespace)
	}

	// 3. Test Grove overrides
	groveSettings := `{
		"default_runtime": "docker",
		"kubernetes": {
			"default_context": "gke-dev"
		}
	}`
	groveScionDir := filepath.Join(groveDir, ".scion")
	if err := os.MkdirAll(groveScionDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groveScionDir, "settings.json"), []byte(groveSettings), 0644); err != nil {
		t.Fatal(err)
	}

	s, err = LoadSettings(groveDir)
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	// Should be back to docker because grove overrides global
	if s.DefaultRuntime != "docker" {
		t.Errorf("expected grove override runtime 'docker', got '%s'", s.DefaultRuntime)
	}
	// Namespace should still be global
	if s.Kubernetes.DefaultNamespace != "scion-global" {
		t.Errorf("expected inherited global namespace 'scion-global', got '%s'", s.Kubernetes.DefaultNamespace)
	}
	// Context should be grove
	if s.Kubernetes.DefaultContext != "gke-dev" {
		t.Errorf("expected grove context 'gke-dev', got '%s'", s.Kubernetes.DefaultContext)
	}
}

func TestUpdateSetting(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	groveDir := filepath.Join(tmpDir, "my-grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Update local setting
	err := UpdateSetting(groveDir, "default_runtime", "kubernetes", false)
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	// Verify file content
	content, err := os.ReadFile(filepath.Join(groveDir, ".scion", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"default_runtime": "kubernetes"`) {
		t.Errorf("expected file to contain default_runtime: kubernetes, got %s", string(content))
	}

	// Update global setting
	err = UpdateSetting(groveDir, "docker.host", "tcp://localhost:2375", true)
	if err != nil {
		t.Fatalf("UpdateSetting global failed: %v", err)
	}

	// Verify global file content
	content, err = os.ReadFile(filepath.Join(tmpDir, ".scion", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"host": "tcp://localhost:2375"`) {
		t.Errorf("expected global file to contain host: tcp://localhost:2375, got %s", string(content))
	}
}

func TestValidateRuntime(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"docker", true},
		{"kubernetes", true},
		{"local", true},
		{"container", true},
		{"invalid", false},
		{"", false},
	}

	// We can't access private validateRuntime directly if it is not exported.
	// But UpdateSetting calls it.

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	groveDir := filepath.Join(tmpDir, "my-grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		err := UpdateSetting(groveDir, "default_runtime", tt.input, false)
		if tt.want {
			if err != nil {
				t.Errorf("UpdateSetting(%q) failed: %v", tt.input, err)
			}
		} else {
			if err == nil {
				t.Errorf("UpdateSetting(%q) should have failed", tt.input)
			}
		}
	}
}

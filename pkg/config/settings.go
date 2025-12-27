package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type KubernetesSettings struct {
	DefaultContext   string `json:"default_context,omitempty"`
	DefaultNamespace string `json:"default_namespace,omitempty"`
}

type DockerSettings struct {
	Host string `json:"host,omitempty"`
}

type Settings struct {
	DefaultRuntime string             `json:"default_runtime,omitempty"`
	Kubernetes     KubernetesSettings `json:"kubernetes,omitempty"`
	Docker         DockerSettings     `json:"docker,omitempty"`
}

// LoadSettings loads and merges settings from the hierarchy.
// Priority: Grove > Global > Defaults
func LoadSettings(grovePath string) (*Settings, error) {
	// 1. Start with App Defaults
	settings := &Settings{
		DefaultRuntime: "docker",
	}

	// 2. Merge Global (~/.scion/settings.json)
	globalDir, err := GetGlobalDir()
	if err == nil {
		globalSettingsPath := filepath.Join(globalDir, "settings.json")
		if err := mergeSettingsFromFile(settings, globalSettingsPath); err != nil {
			// Log warning or ignore if file just doesn't exist?
			// For now, we ignore if it doesn't exist, but report parse errors
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load global settings: %w", err)
			}
		}
	}

	// 3. Merge Grove (.scion/settings.json)
	if grovePath != "" {
		groveSettingsPath := filepath.Join(grovePath, ".scion", "settings.json")

		if err := mergeSettingsFromFile(settings, groveSettingsPath); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load grove settings: %w", err)
			}
		}
	}

	return settings, nil
}

func mergeSettingsFromFile(base *Settings, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var override Settings
	if err := json.Unmarshal(data, &override); err != nil {
		return err
	}

	// Manual merge of top-level fields
	if override.DefaultRuntime != "" {
		base.DefaultRuntime = override.DefaultRuntime
	}

	if override.Kubernetes.DefaultContext != "" {
		base.Kubernetes.DefaultContext = override.Kubernetes.DefaultContext
	}
	if override.Kubernetes.DefaultNamespace != "" {
		base.Kubernetes.DefaultNamespace = override.Kubernetes.DefaultNamespace
	}

	if override.Docker.Host != "" {
		base.Docker.Host = override.Docker.Host
	}

	return nil
}

// SaveSettings saves the settings to the specified location.
func SaveSettings(grovePath string, settings *Settings, global bool) error {
	var targetPath string
	if global {
		globalDir, err := GetGlobalDir()
		if err != nil {
			return err
		}
		targetPath = filepath.Join(globalDir, "settings.json")
	} else {
		if grovePath == "" {
			return fmt.Errorf("grove path required for local settings")
		}
		targetPath = filepath.Join(grovePath, ".scion", "settings.json")
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, data, 0644)
}

// UpdateSetting updates a specific setting key in the specified scope (global or local).
func UpdateSetting(grovePath string, key string, value string, global bool) error {
	var targetPath string
	if global {
		globalDir, err := GetGlobalDir()
		if err != nil {
			return err
		}
		targetPath = filepath.Join(globalDir, "settings.json")
	} else {
		if grovePath == "" {
			return fmt.Errorf("grove path required for local settings")
		}
		targetPath = filepath.Join(grovePath, ".scion", "settings.json")
	}

	// Load existing file specifically (not merged)
	var current Settings
	data, err := os.ReadFile(targetPath)
	if err == nil {
		if err := json.Unmarshal(data, &current); err != nil {
			// If corrupt, maybe start fresh? Or error?
			return fmt.Errorf("failed to parse existing settings at %s: %w", targetPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// Update the field
	switch key {
	case "default_runtime":
		if err := validateRuntime(value); err != nil {
			return err
		}
		current.DefaultRuntime = value
	case "kubernetes.default_context":
		current.Kubernetes.DefaultContext = value
	case "kubernetes.default_namespace":
		current.Kubernetes.DefaultNamespace = value
	case "docker.host":
		current.Docker.Host = value
	default:
		return fmt.Errorf("unknown setting key: %s", key)
	}

	// Save
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	newData, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, newData, 0644)
}

func GetSettingValue(s *Settings, key string) (string, error) {
	switch key {
	case "default_runtime":
		return s.DefaultRuntime, nil
	case "kubernetes.default_context":
		return s.Kubernetes.DefaultContext, nil
	case "kubernetes.default_namespace":
		return s.Kubernetes.DefaultNamespace, nil
	case "docker.host":
		return s.Docker.Host, nil
	}
	return "", fmt.Errorf("unknown setting key: %s", key)
}

// Helper to inspect struct fields if needed for "list"
func GetSettingsMap(s *Settings) map[string]string {
	m := make(map[string]string)
	m["default_runtime"] = s.DefaultRuntime
	m["kubernetes.default_context"] = s.Kubernetes.DefaultContext
	m["kubernetes.default_namespace"] = s.Kubernetes.DefaultNamespace
	m["docker.host"] = s.Docker.Host
	return m
}

func validateRuntime(r string) error {
	switch r {
	case "docker", "kubernetes", "local", "container":
		return nil
	default:
		return fmt.Errorf("invalid runtime '%s'. Supported values: docker, kubernetes, local", r)
	}
}

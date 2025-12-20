package runtime

import (
	"os"
	"os/exec"
)

func GetRuntime() Runtime {
	sandbox := os.Getenv("GEMINI_SANDBOX")
	if sandbox == "container" {
		return NewAppleContainerRuntime()
	}
	if sandbox == "docker" {
		return NewDockerRuntime()
	}

	// Auto-detection on macOS
	if _, err := exec.LookPath("container"); err == nil {
		return NewAppleContainerRuntime()
	}

	// Fallback to Docker if present
	if _, err := exec.LookPath("docker"); err == nil {
		return NewDockerRuntime()
	}

	// Default to Apple Container (might fail at call time if missing)
	return NewAppleContainerRuntime()
}

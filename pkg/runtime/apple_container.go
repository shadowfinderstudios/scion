package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type AppleContainerRuntime struct {
	Command string // usually "container"
}

func NewAppleContainerRuntime() *AppleContainerRuntime {
	return &AppleContainerRuntime{
		Command: "container",
	}
}

func (r *AppleContainerRuntime) RunDetached(ctx context.Context, config RunConfig) (string, error) {
	args := []string{"run", "-d", "-t", "--name", config.Name}

	// container CLI doesn't support --init
	
	if config.HomeDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/home/gemini", config.HomeDir))
	}
	if config.Workspace != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/workspace", config.Workspace))
	}

	for _, e := range config.Env {
		args = append(args, "-e", e)
	}

	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, config.Image)

	cmd := exec.CommandContext(ctx, r.Command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("container run failed: %w (output: %s)", err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}

func (r *AppleContainerRuntime) Stop(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.Command, "stop", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("container stop failed: %w (output: %s)", err, string(out))
	}

	cmdRm := exec.CommandContext(ctx, r.Command, "rm", id)
	outRm, err := cmdRm.CombinedOutput()
	if err != nil {
		return fmt.Errorf("container rm failed: %w (output: %s)", err, string(outRm))
	}

	return nil
}

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type containerListOutput struct {
	ID     string            `json:"id"`
	Names  []string          `json:"names"`
	Status string            `json:"status"`
	Image  string            `json:"image"`
	Labels map[string]string `json:"labels"`
}

func (r *AppleContainerRuntime) List(ctx context.Context, labelFilter map[string]string) ([]AgentInfo, error) {
	args := []string{"list", "-a", "--format", "json"}

	cmd := exec.CommandContext(ctx, r.Command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("container list failed: %w (output: %s)", err, string(out))
	}

	var raw []containerListOutput
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse container list output: %w", err)
	}

	var agents []AgentInfo
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}

		// Filter by labels if requested
		if len(labelFilter) > 0 {
			match := true
			for k, v := range labelFilter {
				if lv, ok := c.Labels[k]; !ok || lv != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		agents = append(agents, AgentInfo{
			ID:     c.ID,
			Name:   name,
			Status: c.Status,
			Image:  c.Image,
		})
	}

	return agents, nil
}

func (r *AppleContainerRuntime) GetLogs(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, r.Command, "logs", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("container logs failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

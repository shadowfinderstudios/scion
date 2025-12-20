package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type DockerRuntime struct {
	Command string
}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{Command: "docker"}
}

func (r *DockerRuntime) RunDetached(ctx context.Context, config RunConfig) (string, error) {
	args := []string{"run", "-d", "-t", "--init", "--name", config.Name}

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
		return "", fmt.Errorf("docker run failed: %w (output: %s)", err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}

func (r *DockerRuntime) Stop(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.Command, "stop", id)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop failed: %w (output: %s)", err, string(out))
	}
	cmdRm := exec.CommandContext(ctx, r.Command, "rm", id)
	if out, err := cmdRm.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm failed: %w (output: %s)", err, string(out))
	}
	return nil
}

func (r *DockerRuntime) List(ctx context.Context, labelFilter map[string]string) ([]AgentInfo, error) {
	// Docker implementation of list using --format {{json .}}
	args := []string{"ps", "-a", "--format", "{{json .}}"}
	cmd := exec.CommandContext(ctx, r.Command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w (output: %s)", err, string(out))
	}

	var agents []AgentInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var data struct {
			ID     string `json:"ID"`
			Names  string `json:"Names"`
			Status string `json:"Status"`
			Image  string `json:"Image"`
			Labels string `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}

		// Docker labels in json format are a comma separated string in some versions,
		// or we might need to be more careful. 
		// For now, let's keep it simple.

		agents = append(agents, AgentInfo{
			ID:     data.ID,
			Name:   data.Names,
			Status: data.Status,
			Image:  data.Image,
		})
	}
	return agents, nil
}

func (r *DockerRuntime) GetLogs(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, r.Command, "logs", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

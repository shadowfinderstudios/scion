---
title: Runtime Development
description: Implementing new execution backends for Scion agents.
---

Scion abstracts the execution environment through the `Runtime` interface. This allows the orchestrator to run agents in diverse environments, from local Docker containers to remote Kubernetes Pods, while maintaining a consistent management API.

## The Runtime Interface

Every runtime implementation must satisfy the `Runtime` interface defined in `pkg/runtime/interface.go`:

```go
type Runtime interface {
	// Name returns a unique identifier for this runtime (e.g., "docker", "kubernetes").
	Name() string

	// Run starts a new agent instance based on the provided configuration.
	// It returns the unique container/pod ID.
	Run(ctx context.Context, config RunConfig) (string, error)

	// Stop gracefully terminates a running agent.
	Stop(ctx context.Context, id string) error

	// Delete removes all resources associated with an agent (containers, ephemeral storage).
	Delete(ctx context.Context, id string) error

	// List returns a list of agents managed by this runtime, optionally filtered by labels.
	List(ctx context.Context, labelFilter map[string]string) ([]api.AgentInfo, error)

	// GetLogs retrieves the standard output/error logs from the agent.
	GetLogs(ctx context.Context, id string) (string, error)

	// Attach provides an interactive terminal stream to the agent's main session.
	Attach(ctx context.Context, id string) error

	// ImageExists checks if the required container image is available locally.
	ImageExists(ctx context.Context, image string) (bool, error)

	// PullImage downloads the container image from a registry.
	PullImage(ctx context.Context, image string) error

	// Sync synchronizes files between the host and the agent's workspace.
	Sync(ctx context.Context, id string, direction SyncDirection) error

	// Exec runs a one-off command inside the agent's environment.
	Exec(ctx context.Context, id string, cmd []string) (string, error)

	// GetWorkspacePath returns the host path to the container's /workspace mount.
	GetWorkspacePath(ctx context.Context, id string) (string, error)
}
```

## Implementing a New Runtime

### 1. Define the Struct
Create a new file in `pkg/runtime/` (e.g., `my_runtime.go`) and define your runtime struct.

```go
type MyRuntime struct {
    // Add configuration fields (e.g., API client, namespace)
}

func NewMyRuntime(config MyConfig) *MyRuntime {
    return &MyRuntime{ ... }
}
```

### 2. Implement the Methods
Follow these guidelines for key methods:

- **Run**: This is the most complex method. You must handle mounting the agent's home directory and workspace, injecting environment variables, and setting up the entrypoint (usually wrapping the harness command in `tmux`).
- **Stop vs. Delete**: `Stop` should be non-destructive (e.g., `docker stop`), while `Delete` should clean up all ephemeral resources (e.g., `docker rm -v`).
- **Attach**: Use a PTY-compatible streaming mechanism. For local runtimes, this often involves `os/exec` with a PTY. For remote runtimes like Kubernetes, use the SPDY/WebSocket `exec` subprotocol.

### 3. Register the Runtime
Add your new runtime to the `GetRuntime` factory in `pkg/runtime/factory.go`. This allows users to select it via the `profiles` section in `settings.yaml`.

## Key Considerations

### Workspace Isolation
The runtime is responsible for ensuring that the agent's `/workspace` is isolated. For local runtimes, this is typically done via a bind mount to a dedicated git worktree. For remote runtimes, you may need to implement a sync strategy using `rsync`, tar snapshots, or a shared volume.

### Credential Injection
Secrets and credentials provided in `RunConfig.ResolvedSecrets` must be injected into the container.
- **Environment Secrets**: Injected as environment variables.
- **File Secrets**: Written to a secure, ephemeral location and mounted into the container.

### Resource Limits
Respect the `api.ResourceSpec` provided in the `RunConfig`. This includes CPU and memory requests/limits, and optionally disk quotas.

### Labels and Annotations
Always apply the labels provided in `RunConfig.Labels` to the underlying container/pod. Scion uses these labels for internal tracking, cleanup, and status reporting.

## Testing Your Runtime
Each runtime should include a comprehensive test suite (e.g., `my_runtime_test.go`). You can use the `mock.go` in `pkg/runtime/` or existing tests like `docker_test.go` as a reference for integration testing.

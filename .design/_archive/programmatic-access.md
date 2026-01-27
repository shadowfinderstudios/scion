# Programmatic Control & External Integration Design

## 1. Overview

The current Scion architecture is primarily designed for a human user interacting via a CLI on their local machine. To scale Scion to team workflows, we need to support **automation triggers** (CI/CD pipelines) and **interactive bots** (Slack/Discord).

This design document outlines the architecture for decoupling the core agent lifecycle logic from the CLI implementation. It specifically targets enabling external Go modules (like `scion-slack` or `scion-ci`) to consume Scion's core logic without inheriting heavy CLI dependencies or side effects.

## 2. Problem Statement

Currently, Scion's logic is embedded in `cmd/` packages (e.g., `cmd/start.go`, `cmd/common.go`), making it difficult to import and reuse in other Go applications.

**Key Limitations:**
*   **CLI Coupling**: Logic relies on global flags (`startCmd.Flags()`) and direct `fmt.Printf` to stdout.
*   **Mixed Concerns**: `pkg/api` currently mixes configuration, runtime interfaces, and internal helpers.
*   **Filesystem Assumptions**: Hardcoded reliance on local `.scion` directories and `os.Getwd()` for context.
*   **Lack of Feedback Loop**: External programs cannot easily subscribe to agent events (errors, "waiting for input", completion) without parsing logs or polling files.

## 3. Core Architecture

We will refactor the codebase to separate **Data Types**, **Business Logic (Core)**, and the **User Interface (CLI)**.

### 3.1. Package Structure Strategy

To support lightweight external consumers, we will enforce a strict separation of concerns:

*   **`pkg/api` (Pure Types)**: This package will contain **pure data structures** with zero dependencies on runtimes or heavy logic. This allows a Slack bot or CI tool to import Scion types without pulling in Docker or Kubernetes libraries.
    *   Contains: `AgentInfo`, `StatusEvent`, `StartOptions`.
*   **`pkg/agent` (Logic Core)**: This package houses the business logic and implements the `Manager` interface. It depends on `pkg/runtime` and `pkg/config`.

### 3.2. The `Manager` Interface

The `Manager` is the primary entry point for programmatic control.

```go
// pkg/agent/manager.go

type Manager interface {
    // Start launches a new agent with the given configuration
    Start(ctx context.Context, opts StartOptions) (*Agent, error)

    // Stop terminates an agent
    Stop(ctx context.Context, agentID string) error

    // List returns active agents
    List(ctx context.Context, filter Filter) ([]*Agent, error)

    // Watch returns a channel of status updates for an agent
    Watch(ctx context.Context, agentID string) (<-chan StatusEvent, error)
}
```

#### Configuration & Secrets
To decouple from CLI flags, we introduce `StartOptions`. To ensure security, secrets should not be passed as raw strings in structs that might be logged.

```go
type StartOptions struct {
    Name        string
    Task        string
    Template    string
    Image       string
    GrovePath   string
    Env         map[string]string
    Detached    bool
    
    // AuthProvider abstracts secret retrieval (e.g., from Env, K8s Secrets, or Vault)
    Auth        AuthProvider 
}
```

### 3.3. Status & Event Stream

To support remote bots, the `Manager` provides a `Watch` method emitting typed events.

*   **Serialization**: `StatusEvent` structs must be strictly defined with JSON tags in `pkg/api` to support remote wire transmission (e.g., gRPC/WebSocket wrappers).
*   **Streams**:
    *   `StatusChanged`: High-level state changes (Starting -> Thinking -> Waiting).
    *   `OutputReceived`: Stream of logs/thoughts (kept distinct to avoid blocking control flow).
    *   `ErrorOccurred`: Structured error reporting.

### 3.4. Dependency Injection & Runtime

External consumers may have different runtime requirements (e.g., a pure K8s operator vs. a local Docker developer tool).

*   **Injection**: The `Manager` should be initialized with a specific `Runtime` implementation (or factory).
*   **Benefit**: This prevents `scion-slack` from needing to compile Docker libraries if it only uses Kubernetes.

## 4. Supporting Projects (External)

These projects will reside in separate repositories (or separate modules) but depend on `scion/pkg/...`.

### 4.1. Scion Slack Bot (`scion-slack`)

A long-running service that listens to chat events and manages agents.

**Architecture:**
1.  **Listener**: Receives slash commands or mentions.
2.  **Context Resolution**: Maps a Slack Channel ID to a Scion Grove.
3.  **Execution**: Calls `manager.Start(opts)` and starts a goroutine to `Watch()` the agent.
4.  **Feedback**: Posts buttons (Approve/Reject) on `WaitingForInput` events.

### 4.2. CI/CD Automation (`scion-action`)

A GitHub Action / GitLab Step to autonomously fix or review code.

**Workflow:**
1.  **Trigger**: Pull Request opened.
2.  **Execution**: Invokes `scion` programmatically to "Review this PR".
3.  **Output**: Parses `pkg/api` structures to create structured PR comments.

## 5. Implementation Roadmap

1.  **Step 0: Type Cleanup**: Refactor `pkg/api` to be a pure type library. Move any logic or interfaces with heavy dependencies out.
2.  **Step 1: The Manager**: Create `pkg/agent` and define the `Manager` interface and `StartOptions` struct.
3.  **Step 2: Logic Migration**: Port `ProvisionAgent` logic to `pkg/agent/provision.go` and `RunAgent` logic to `pkg/agent/run.go`. Ensure all direct `stdout` printing is removed/replaced with event emissions or return values.
4.  **Step 3: Runtime Injection**: Update `Manager` constructor to accept a `Runtime` interface.
5.  **Step 4: CLI Refactor**: Update `cmd/*` to become thin wrappers around the `pkg/agent` Manager.

## 6. Challenges & Considerations

*   **Authentication**: CLI relies on local user login. Bots need Service Account injection. The `AuthProvider` interface must support both.
*   **Concurrency**: The `Manager` must be thread-safe to handle multiple agents in a server context.
*   **Persistence**: Future iterations should allow the `Manager` to swap the file-based "Grove" for a database backend.
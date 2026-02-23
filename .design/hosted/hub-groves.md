# Hub-Native Groves: Filesystem-Based Workspaces on the Hub

**Created:** 2026-02-23
**Status:** Draft / Discussion
**Related:** `git-groves.md`, `sync-design.md`, `hosted-architecture.md`

---

## 1. Overview

Today, groves are created in two ways:

1. **Local-first**: Run `scion grove init` inside a local git checkout, optionally register with the Hub.
2. **Git-URL-first**: Create a grove on the Hub from a remote git URL (per `git-groves.md`). Agents clone the repo at startup.

This document proposes a third mode: **Hub-native groves** — groves whose workspace is a plain directory on the Hub server's filesystem, with no backing git repository. These groves are created entirely through the web interface (or Hub API), enabling users to spin up agent workspaces without any local machine, CLI, or git hosting involvement.

### Motivation

- **Zero-infrastructure onboarding**: A user with only a browser and a Hub account can create a grove and start agents.
- **Scratch/ephemeral workspaces**: Useful for experimentation, prototyping, or one-off tasks that don't warrant a git repo.
- **Hub-as-IDE foundation**: Lays groundwork for a fully web-based workflow where code lives on the Hub.

### Goals

1. Allow grove creation entirely via the web UI or Hub API — no CLI or git URL required.
2. Workspace directories are managed on the Hub server's local filesystem under `~/.scion/groves/`.
3. Agents provisioned against these groves receive a functional workspace without git clone.
4. The resulting grove is a first-class Hub grove — visible in the web UI, supports agents, templates, and all existing grove operations.

### Non-Goals

- Replacing git-based groves for repositories that already have a remote.
- Providing a full web-based code editor (though this could build toward one).
- Multi-Hub replication or cross-Hub grove sharing.
- Workspace persistence guarantees beyond the Hub server's own storage.

---

## 2. Filesystem Layout

Hub-native groves are stored under the global Scion directory on the Hub server:

```
~/.scion/groves/
  ├── my-project/
  │   └── .scion/
  │       ├── settings.yaml
  │       ├── templates/
  │       └── agents/
  ├── experiment-alpha/
  │   └── .scion/
  │       └── ...
  └── scratch-2026-02/
      └── .scion/
          └── ...
```

Each grove directory is equivalent to what `scion grove init` produces in a local project — a `.scion/` subdirectory with settings, templates, and agent metadata. The parent directory (`my-project/`) acts as the workspace root.

### 2.1 Directory Naming

The grove directory name is derived from the grove slug. Since slugs are already URL-safe and unique per Hub, they map directly to filesystem directories.

**Open question:** See [Section 6 — Open Questions](#6-open-questions) for collision and naming concerns.

---

## 3. Creation Flow

### 3.1 Conceptual Steps

Creating a hub-native grove is equivalent to:

```bash
mkdir -p ~/.scion/groves/<slug>
cd ~/.scion/groves/<slug>
scion grove init            # seeds .scion/ directory structure
scion hub enable --hub <this-hub-url>   # links grove to this Hub
```

But executed server-side by the Hub process itself, not by a CLI invocation.

### 3.2 API-Level Flow

1. **User submits** grove creation request via web UI or API with no `gitRemote` field.
2. **Hub server**:
   a. Creates the grove record in the database (existing `createGrove` handler).
   b. Creates the filesystem directory: `~/.scion/groves/<slug>/`.
   c. Runs the equivalent of `config.InitProject()` to seed `.scion/` structure.
   d. Writes grove settings linking to this Hub instance (hub endpoint, grove ID).
   e. Records the filesystem path on the grove record (or a label).
3. **Response** returns the grove object, same as any other grove creation.

### 3.3 Approach Alternatives for Creation

#### Approach A: Hub calls `InitProject()` directly (library call)

The Hub server imports `pkg/config` and calls `InitProject(targetDir, harnesses)` directly, then writes the hub settings into the seeded `.scion/settings.yaml`.

**Pros:**
- No subprocess overhead.
- No dependency on `scion` binary being available in PATH on the Hub server.
- Type-safe; errors are Go errors.

**Cons:**
- Couples the Hub server code to `pkg/config` initialization logic more tightly.
- Any init-time side effects (e.g., git detection) need to be accounted for in a non-git context.

#### Approach B: Hub shells out to `scion` CLI

The Hub server execs `scion grove init` and `scion hub enable` as subprocesses.

**Pros:**
- Exact same code path as local CLI usage — no divergence risk.
- Automatically picks up any future init logic changes.

**Cons:**
- Requires the `scion` binary to be on the Hub server's PATH (it already is in most deployments, since the Hub is a `scion` subcommand).
- Subprocess error handling is less ergonomic.
- Interactive prompts in `grove init` would need to be bypassed (require `--non-interactive` flag or similar).

#### Approach C: New dedicated Hub handler with minimal init

A new handler purpose-built for hub-native groves that only creates the minimal required structure (directory + `settings.yaml`) without the full `InitProject` flow. Templates and harness configs are pulled from the Hub's own template store rather than from embedded defaults.

**Pros:**
- Cleanest separation — Hub groves don't pretend to be local groves.
- Can evolve independently from local init logic.
- Template management is already centralized on the Hub.

**Cons:**
- Parallel code paths that could diverge from local behavior.
- Need to define what "minimal" means — risk of missing something agents expect.

**Recommendation:** Approach A is likely the right starting point. The Hub server already imports `pkg/config` for other purposes, and `InitProject()` is a pure filesystem operation with no interactive prompts. Approach C could be a later evolution once hub-native groves have distinct requirements.

---

## 4. Agent Workspace Provisioning

When an agent is created against a hub-native grove, the workspace must be made available to the agent container. There are several strategies:

### 4.1 Option: Direct bind-mount (Hub-colocated broker only)

If the Runtime Broker is colocated with the Hub (same machine), the grove directory is bind-mounted into the agent container as `/workspace`, the same way local solo-mode groves work.

```
Container /workspace  →  ~/.scion/groves/<slug>/
```

**Pros:**
- Simple, no file transfer needed.
- Agents see changes immediately (no sync lag).
- Existing worktree isolation logic (`../.scion_worktrees/`) can apply.

**Cons:**
- Only works when broker runs on the same host as the Hub.
- Multiple agents writing to the same directory need worktree isolation or some other mechanism.

### 4.2 Option: Workspace sync via GCS (remote brokers)

For remote Runtime Brokers, the Hub uploads the workspace contents to GCS (using the existing sync-design pattern), and the broker downloads them at agent startup — identical to the flow described in `sync-design.md`.

**Pros:**
- Works for any broker topology.
- Reuses existing infrastructure.

**Cons:**
- Requires GCS bucket configuration.
- Adds latency at agent startup.
- Workspace changes require explicit sync operations.

### 4.3 Option: Hub serves workspace over HTTP (tar download)

The Hub exposes an endpoint that streams the workspace directory as a tar archive. The Runtime Broker downloads and extracts it during agent provisioning.

**Pros:**
- No external storage dependency (no GCS).
- Works for remote brokers without cloud storage.

**Cons:**
- Hub is in the data path (bandwidth and latency).
- No incremental sync — full download each time.
- Doesn't scale well for large workspaces.

### 4.4 Workspace Isolation Between Agents

Regardless of provisioning strategy, multiple agents in the same grove need workspace isolation to avoid conflicts. Options:

**A. Git-init + worktree:** `git init` the grove directory so agents can use the existing worktree strategy. This adds git semantics to what was described as a non-git workspace, but provides proven isolation.

**B. Copy-on-create:** Each agent gets a filesystem copy of the grove directory. Simple but storage-heavy.

**C. Overlay filesystem:** Use overlayfs so agents share a read-only base with per-agent writable layers. Efficient but platform-specific.

**D. Single-agent default:** Hub-native groves initially support only one agent at a time, sidestepping the isolation question. Simplest starting point.

**Recommendation:** Start with the Hub-colocated bind-mount (4.1) paired with git-init for worktree isolation (4.4.A). This matches existing solo-mode behavior and requires minimal new infrastructure. GCS sync (4.2) can be added later for remote broker support using the existing sync-design infrastructure.

---

## 5. Data Model Changes

### 5.1 Grove Record

The existing `store.Grove` model needs to distinguish hub-native groves from git-anchored groves. Options:

**A. Label-based (no schema change):**

```go
grove.Labels["scion.dev/workspace-type"] = "hub-native"
grove.Labels["scion.dev/workspace-path"] = "/home/user/.scion/groves/my-project"
```

**B. New field on Grove:**

```go
type Grove struct {
    // ... existing fields ...
    WorkspacePath string `json:"workspacePath,omitempty"` // Hub-local filesystem path
    WorkspaceType string `json:"workspaceType,omitempty"` // "git", "hub-native", "synced"
}
```

**C. Infer from absence of GitRemote:**

If `GitRemote == ""`, the grove is hub-native. The workspace path is derived conventionally from `~/.scion/groves/<slug>`. No new fields needed.

**Recommendation:** Option C for the initial implementation — it requires no schema changes, and the convention-based path derivation is straightforward. Labels (Option A) can be added for metadata if needed without a migration.

### 5.2 Grove ID for Hub-Native Groves

Git-anchored groves use a deterministic hash of the normalized git URL. Hub-native groves have no URL to hash, so they should use a generated UUID — which is already the fallback in `GenerateGroveID()` when no git remote is found.

### 5.3 populateAgentConfig Changes

The existing `populateAgentConfig()` in `pkg/hub/handlers.go:4397` populates `GitClone` config when `grove.GitRemote != ""`. For hub-native groves, it should instead set the `Workspace` field to the grove's filesystem path (for colocated brokers) or set `WorkspaceStoragePath` (for remote brokers after sync upload).

---

## 6. Open Questions

### Q1: Should hub-native groves auto-initialize a git repo?

Running `git init` inside the grove directory would:
- Enable worktree-based agent isolation (proven pattern).
- Allow agents to commit their work.
- Enable `git diff` for reviewing agent changes.
- Make the grove easily promotable to a full git-remote grove later.

**Trade-off:** Adds git overhead to what's meant to be a "no-git" experience. But agents already expect git — most harness instructions reference `git commit`, `git diff`, etc. A grove without git may confuse agents.

**Leaning:** Yes, auto-init git. It's a single `git init` call, agents work better with it, and it doesn't require a remote.

### Q2: How should the Hub server discover its own URL for `hub enable`?

When seeding grove settings, the Hub needs to write its own endpoint URL. Options:
- Use the Hub's configured `--address` / `SCION_HUB_ENDPOINT`.
- Use the `Origin` or `Host` header from the web UI request that triggered creation.
- Require it as a Hub server configuration value.

### Q3: What happens when a hub-native grove is deleted?

Options:
- **Soft delete only:** Grove record is marked deleted, filesystem is preserved. User can manually clean up.
- **Full delete:** Grove record deleted AND filesystem directory removed. Destructive but clean.
- **Archive:** Move directory to `~/.scion/groves/.archive/<slug>-<timestamp>` before deleting the record.

**Leaning:** Soft delete in the database (existing behavior) with filesystem preserved. A separate `grove purge` operation can clean up the filesystem.

### Q4: Should hub-native groves be promotable to git-remote groves?

A natural flow: user starts with a hub-native grove for prototyping, then wants to push it to GitHub and convert it to a git-anchored grove.

This would involve:
1. `git init` (if not already done).
2. `git remote add origin <url>`.
3. `git push`.
4. Update the grove record with `GitRemote`.

This seems valuable but could be deferred. If Q1 is answered "yes" (auto-init git), promotion becomes much simpler.

### Q5: Can multiple Hubs serve the same hub-native grove?

Probably not — the grove lives on one Hub's filesystem. This is a single-Hub feature by nature. Remote brokers can serve agents for it via sync, but the source of truth is one Hub's disk.

### Q6: Storage limits and quotas

Should there be limits on:
- Number of hub-native groves per user?
- Total disk usage per grove?
- Number of agents per hub-native grove?

This may not matter for initial implementation but becomes important for any multi-tenant deployment.

### Q7: Workspace seeding / initial content

When creating a hub-native grove, should the user be able to:
- Start with an empty workspace?
- Upload files via the web UI?
- Choose from a set of starter templates (e.g., "Python project", "Node.js app")?
- Paste a git URL that gets cloned once (but not tracked as a git-remote grove)?

Starting empty is simplest, but the ability to seed content makes the feature much more useful for onboarding.

---

## 7. Web UI Considerations

### 7.1 Grove Creation Form

The existing grove creation flow (web UI) would need a mode selector or would infer the type based on input:

- **If git URL is provided** → git-anchored grove (existing flow).
- **If no git URL** → hub-native grove (new flow).

The form should allow the user to specify:
- Grove name (required).
- Slug (auto-derived, optionally overridden).
- Visibility (private/team/public).
- Initial content strategy (empty, upload, template — see Q7).

### 7.2 Grove Detail View

Hub-native groves should be visually distinguishable from git-anchored groves. The detail view should show:
- Workspace type indicator (e.g., "Hub Workspace" vs "Git Repository").
- Filesystem path (admin-facing, may be hidden from non-admin users).
- Disk usage (if quotas are implemented).
- No git remote / clone URL section.

### 7.3 File Browser (Future)

A natural follow-on feature: a web-based file browser for hub-native grove workspaces. This would allow users to view and edit files without attaching to an agent. Out of scope for this design but worth noting as a motivating use case.

---

## 8. Security Considerations

### 8.1 Filesystem Access

The Hub process creates and manages directories under `~/.scion/groves/`. This means:
- The Hub process user must have write access to this path.
- Groves from different users share the same filesystem namespace — slug uniqueness prevents collisions, but filesystem permissions should ensure user A cannot access user B's grove directory through OS-level paths.
- In multi-tenant deployments, consider per-user subdirectories: `~/.scion/groves/<user-id>/<slug>/`.

### 8.2 Path Traversal

The grove slug is used as a directory name. The slug derivation (`api.Slugify()`) must guarantee no path traversal characters (`..`, `/`, etc.) can appear in the slug. The existing `Slugify` implementation should be audited for this.

### 8.3 Agent Container Isolation

When bind-mounting grove directories into agent containers, standard container isolation applies. Agents should not be able to escape their mount to access other groves' directories.

---

## 9. Implementation Phases

### Phase 1: Minimal Hub-Native Grove

- Hub API accepts grove creation with no `gitRemote`.
- Hub creates `~/.scion/groves/<slug>/` directory with `InitProject()`.
- `git init` inside the grove directory for worktree support.
- Writes hub settings into `.scion/settings.yaml`.
- Colocated broker bind-mounts the grove directory for agents.
- Web UI allows creating groves without a git URL.

### Phase 2: Workspace Content Seeding

- Web UI allows uploading initial files into a hub-native grove.
- Hub API endpoint for uploading files to a grove's workspace.
- Optional starter templates for common project types.

### Phase 3: Remote Broker Support

- Hub uploads workspace to GCS for remote broker provisioning.
- Reuses `sync-design.md` infrastructure.
- Workspace sync back from agents to Hub filesystem.

### Phase 4: Grove Promotion

- `scion hub grove promote <slug> --git-remote <url>` converts a hub-native grove to a git-anchored grove.
- Pushes existing content to the remote.
- Updates grove record with `GitRemote`.

---

## 10. Relationship to Existing Designs

| Design | Relationship |
|--------|-------------|
| `git-groves.md` | Complementary — git groves use clone, hub-native groves use local filesystem. Same grove API, different workspace strategy. |
| `sync-design.md` | Hub-native groves can use workspace sync for remote broker support (Phase 3). |
| `hosted-architecture.md` | Hub-native groves fit the grove-centric model. The Hub is both state server and workspace host. |
| `secrets.md` | Hub-native groves use the same secret management. No git tokens needed, but API keys and other secrets still apply. |

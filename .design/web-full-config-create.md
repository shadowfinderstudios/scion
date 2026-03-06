# Advanced Agent Creation Form (Web UI)

**Status:** Draft
**Created:** 2026-03-06
**Related:** [jit-agent-config.md](./jit-agent-config.md), [web-frontend-design.md](./hosted/web-frontend-design.md)

---

## 1. Overview

### Goal

Expose the full `ScionConfig` inline configuration capability (from the JIT agent config design) as a rich web form in the Hub UI. This gives users a way to customize all agent configuration options without needing to create templates or config files.

### Approach: Two-Phase Creation

The existing simple form remains the common starting point. A new **"Create & Edit"** button creates the agent in a provisioned-but-not-started state (`provisionOnly: true`), then navigates to an advanced configuration form where the user can review and modify the full `ScionConfig` before starting.

The provisioning step includes rendering all settings and performing environment variable gathering so that the advanced form is pre-populated with resolved values when presented to the user.

**Flow:**

```
Simple Form (existing)
  |
  |-- [Create & Start] --> Agent starts immediately (existing behavior)
  |
  |-- [Create & Edit]  --> Agent created with provisionOnly: true
                            --> Navigate to Advanced Config Form
                              |-- [Save]   --> Persist config changes
                              |-- [Start]  --> Save + start agent
                              |-- [Delete] --> Delete the provisioned agent
```

---

## 2. Simple Form Changes

### Button Layout Changes

The form actions area gets a second button to the left of the existing "Create & Start" button:

```
┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────┐
│  Create & Edit           │  │  Create & Start Agent    │  │  Cancel   │
└──────────────────────────┘  └──────────────────────────┘  └──────────┘
   Advanced options
```

- **Create & Edit** — `variant="default"` button with a small subtext line ("Advanced options") rendered below the button. Uses the `sliders` icon. Creates the agent with `provisionOnly: true`, then navigates to the advanced form.
- **Create & Start Agent** — remains `variant="primary"` (existing behavior, unchanged).

### Field Migration

- **Telemetry checkbox** — Moved to the advanced form. It will be set via the `ScionConfig.telemetry.enabled` field in the advanced form, with the default populated from the global settings (same as today).
- **Notify checkbox** — Remains on the simple form.

---

## 3. Advanced Configuration Form

### 3.1 Page Structure

The advanced form is a **new page** at route `/agents/:agentId/configure`. It loads the agent's current state (including `appliedConfig`) and presents an editable form for the full `ScionConfig`.

**Page layout:**

```
<- Back to Agents

Configure Agent: <agent-name>
Status: Created (not started)

┌─────────────────────────────────────────────────────┐
│  [Tab bar]                                           │
│                                                      │
│  General | Task & Prompts | Limits & Resources |     │
│  Environment                                         │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │  Tab content (form fields)                      │ │
│  │  ...                                            │ │
│  └─────────────────────────────────────────────────┘ │
│                                                      │
│  ┌────────┐  ┌────────────┐         ┌──────────┐    │
│  │  Save  │  │  Start     │         │  Delete  │    │
│  └────────┘  └────────────┘         └──────────┘    │
└─────────────────────────────────────────────────────┘
```

### 3.2 Form Sections and Fields

Fields are organized into **4 tabs**. The tab layout keeps the form manageable while reducing the number of sections from the original 6 by consolidating Limits + Resources and folding Telemetry into General.

#### Tab: General

| Field | Type | Maps To | Notes |
|-------|------|---------|-------|
| Model | `sl-input` (text) | `model` | e.g., `claude-opus-4-6`, `gemini-2.5-pro` |
| Image | `sl-input` (text) | `image` | Container image override |
| Branch | `sl-input` (text) | `branch` | Git branch for the agent. Auto-populated with agent slug. Only shown for git-based groves. |
| Container User | `sl-input` (text) | `user` | Unix user inside container |
| Auth Method | `sl-select` | `auth_selectedType` | Same options as simple form's harness auth |
| Harness Config | `sl-input` (text, readonly) | `harness_config` | Display-only, set at creation time |
| Enable Telemetry | `sl-checkbox` | `telemetry.enabled` | Default from global settings |

#### Tab: Task & Prompts

| Field | Type | Maps To | Notes |
|-------|------|---------|-------|
| Task | `sl-textarea` | `task` | Initial task/prompt (if not set on simple form) |
| System Prompt | `sl-textarea` | `system_prompt` | Inline content or `file://` URI |
| Agent Instructions | `sl-textarea` | `agent_instructions` | Inline content or `file://` URI |

These text areas should be tall (8-12 rows) since prompts can be lengthy.

#### Tab: Limits & Resources

| Field | Type | Maps To | Notes |
|-------|------|---------|-------|
| Max Turns | `sl-input` (number) | `max_turns` | 0 = unlimited |
| Max Model Calls | `sl-input` (number) | `max_model_calls` | 0 = unlimited |
| Max Duration | `sl-input` (text) | `max_duration` | Go duration string, e.g. `30m`, `2h` |
| CPU Request | `sl-input` (text) | `resources.requests.cpu` | e.g., `"2"`, `"500m"` |
| Memory Request | `sl-input` (text) | `resources.requests.memory` | e.g., `"4Gi"` |
| CPU Limit | `sl-input` (text) | `resources.limits.cpu` | |
| Memory Limit | `sl-input` (text) | `resources.limits.memory` | |
| Disk | `sl-input` (text) | `resources.disk` | e.g., `"20Gi"` |

#### Tab: Environment

A dynamic key-value editor for `env`:

```
┌─────────────────────────────────────────────┐
│  KEY                    VALUE          [X]   │
│  ┌──────────────┐  ┌──────────────┐         │
│  │ LOG_LEVEL    │  │ debug        │   [X]   │
│  └──────────────┘  └──────────────┘         │
│  ┌──────────────┐  ┌──────────────┐         │
│  │ GITHUB_TOKEN │  │              │   [X]   │  <-- Required (not gathered)
│  └──────────────┘  └──────────────┘         │
│                                              │
│  [+ Add Variable]                            │
└─────────────────────────────────────────────┘
```

- Pre-populated with any env vars from the template's config and values resolved during env gathering.
- **Required variables** that the env-gather process was not able to fulfill are displayed with a clear visual indicator (e.g., red border, "Required" badge) marking them as blocking.
- The **Start** action validates that all required environment variables have values before proceeding. If any are missing, the Environment tab is activated and the missing fields are highlighted with validation messages.

### 3.3 Fields NOT Exposed

Some `ScionConfig` fields are intentionally omitted from the form:

| Field | Reason |
|-------|--------|
| `harness` | Set implicitly by harness-config selection |
| `config_dir` | Internal path, not user-facing |
| `detached` | Web-created agents are always detached |
| `command_args` | Advanced/internal, rarely needed |
| `task_flag` | Harness-specific internal flag |
| `volumes` | Complex structure, better handled via templates |
| `services` | Complex structure (sidecar definitions), better handled via templates |
| `kubernetes` | K8s-specific config, better handled via templates/settings |
| `secrets` | Managed via Hub secrets UI, not inline config |
| `hub` | Internal hub connection config |
| `default_harness_config` | Template-level concern |

### 3.4 Action Buttons

The form footer has three actions:

- **Save** (`variant="default"`) — Persists the current config to the agent without starting it. User can close and come back later.
- **Start** (`variant="primary"`) — Validates required env vars, saves the config, and starts the agent. Navigates to the agent detail page.
- **Delete** (`variant="danger"`, right-aligned) — Deletes the provisioned agent. Confirms with an `sl-dialog` before proceeding. Navigates back to the agents list.

There is no Cancel button because the agent already exists at this point.


---

## 4. API Requirements

### 4.1 Update Agent Config (Extend Existing PUT)

The existing `PUT /api/v1/agents/:id` only supports updating `name`, `labels`, `annotations`, and `taskSummary`. Extend it to support updating the agent's `ScionConfig` / `AppliedConfig` **before the agent has been started**.

Add a `config` field to the update request body:

```go
var updates struct {
    Name         string            `json:"name,omitempty"`
    Labels       map[string]string `json:"labels,omitempty"`
    Config       *api.ScionConfig  `json:"config,omitempty"`    // NEW
    // ...existing fields...
}
```

Guard: Only allow `config` updates when agent phase is `created` (provisioned but not started). Return `409 Conflict` if the agent is already running/stopped.

> **Future consideration:** Allowing config edits for stopped agents (e.g., to reconfigure and restart) is a natural extension but deferred for now. This would require careful handling of state reset and is better addressed alongside a broader "agent info/edit" feature.

### 4.2 Read Agent Config

The existing `GET /api/v1/agents/:id` returns the agent with `appliedConfig` which includes `inlineConfig` (the full `ScionConfig`). This is sufficient for the form to load initial values.

The form populates fields from resolved values. When an agent is created from a template, the template's resolved values are shown as populated, editable defaults. Empty fields mean "use harness/system default."

Field population priority:
1. `agent.appliedConfig.inlineConfig` — if present (agent was created with `--config` or inline config)
2. `agent.appliedConfig.*` top-level fields — for fields like `image`, `model`, `task` that are extracted into applied config
3. Template defaults — if the agent was created from a template, the template's config provides defaults

### 4.3 Start Agent (Existing)

`POST /api/v1/agents/:id/start` — already exists and works for agents in `created` phase.

### 4.4 Delete Agent (Existing)

`DELETE /api/v1/agents/:id` — already exists.

---

## 5. Component Architecture

### New Components

| Component | File | Purpose |
|-----------|------|---------|
| `scion-page-agent-configure` | `web/src/components/pages/agent-configure.ts` | Advanced config form page |
| `scion-env-editor` | `web/src/components/shared/env-editor.ts` | Reusable key-value pair editor for environment variables |

### Routing

Add route in `web/src/client/main.ts`:

```
/agents/:agentId/configure -> scion-page-agent-configure
```

### Navigation Flow

1. Simple form -> "Create & Edit" -> `POST /api/v1/agents` with `provisionOnly: true` -> navigate to `/agents/:agentId/configure`
2. Configure form -> "Start" -> validate required env vars -> `PUT /api/v1/agents/:id` (save config) -> `POST /api/v1/agents/:id/start` -> navigate to `/agents/:agentId`
3. Configure form -> "Save" -> `PUT /api/v1/agents/:id` (save config) -> stay on page with success toast
4. Configure form -> "Delete" -> confirm dialog -> `DELETE /api/v1/agents/:id` -> navigate to `/agents`

### Re-entry

If a user navigates away and comes back to a `created`-phase agent, the agent detail page should show a "Configure" link/button that navigates to `/agents/:agentId/configure`. This provides a way back to the advanced form for agents that were provisioned but not started.

---

## 6. Implementation Plan

### Phase 1: Backend — Extend Agent Update API

- Extend `PUT /api/v1/agents/:id` in `pkg/hub/handlers.go` to accept a `config` field (`*api.ScionConfig`)
- Guard: only allow config updates when `agent.Phase == "created"`
- Merge the provided config into `agent.AppliedConfig.InlineConfig`
- Update relevant `AppliedConfig` top-level fields (image, model, task, etc.) from the config

### Phase 2: Frontend — Simple Form Button

- Add "Create & Edit" button to `agent-create.ts` with `sliders` icon
- Add "Advanced options" subtext below the button
- Move telemetry checkbox to advanced form
- Wire button to create with `provisionOnly: true` and navigate to configure page

### Phase 3: Frontend — Advanced Config Form

- Create `agent-configure.ts` page component
- Implement 4-tab layout (General, Task & Prompts, Limits & Resources, Environment)
- Create `env-editor.ts` shared component for environment variable editing
  - Support required/blocking validation with visual indicators
- Add route in `main.ts`
- Wire Save/Start/Delete actions
- Validate required env vars on Start

### Phase 4: Polish

- Add re-entry link on agent detail page for `created`-phase agents
- Add loading/error states
- Add success toast on save
- Verify dark mode styling

---

## 7. Decisions

The following decisions have been confirmed:

| Topic | Decision |
|-------|----------|
| **Section layout** | Tabs (4 tabs: General, Task & Prompts, Limits & Resources, Environment) |
| **Template-resolved values** | Show resolved values as populated, editable defaults |
| **Config merge semantics** | Replace — the form always sends the complete config object. Empty/zero fields are omitted via `omitempty`. |
| **Post-start access** | Deferred. A read-only config view for running/stopped agents will be addressed as a broader "agent info" display available for all agents, regardless of how they were started. |
| **"Create & Edit" icon** | `sliders` — verify it's in the `USED_ICONS` array |

---

## 8. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| User creates agent with "Create & Edit" then abandons without starting | Stale `created`-phase agents accumulate. Mitigate: add TTL/cleanup for agents in `created` phase beyond a threshold (separate concern, not blocking). |
| Form complexity overwhelms users | Tabs keep content organized. Most fields are optional. General and Task & Prompts tabs cover the primary use cases. |
| Config update races with agent start | Phase guard on the API: config updates only allowed for `created`-phase agents. If the agent transitions to another phase concurrently, the update returns 409. |
| Template provides values the user doesn't realize are set | Show resolved template values in the form. Consider a visual indicator (muted text, "(from template)") for values inherited from the template. |
| Required env vars missing on start | JS validation on the Start action checks all required env vars have values before proceeding. Missing vars are highlighted and the Environment tab is activated. |

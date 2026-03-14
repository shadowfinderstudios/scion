# Grove-Level Template Discovery and Hub Registration

## Status
**Exploration / Design Draft**

## 1. Problem Statement

Groves can contain their own templates in `.scion/templates/`, but these templates are not automatically known to the Hub. This creates several gaps:

1. **Linked groves**: Templates live on the broker's local filesystem and may vary across brokers serving the same grove. The Hub has no visibility into what templates a grove defines.
2. **Git groves**: Templates exist in the repository's `.scion/templates/` directory, but are only accessible inside an agent container after cloning. The Hub and web UI cannot see or offer these templates.
3. **CLI UX**: `scion templates push/sync` already supports grove-scoped uploads, but users must manually invoke it. There is no automatic discovery or suggestion flow.

### What We Want
- Grove-defined templates should be discoverable and usable from the Hub and web UI
- The experience should feel natural whether working from the CLI (where you're "inside" the grove) or the web (where you're looking at a grove from the outside)
- Multi-broker consistency for linked groves should be addressed or at least understood

---

## 2. Current State

### 2.1 Local Template Storage

Templates are stored as directories:
```
~/.scion/templates/<name>/          # Global scope
<project>/.scion/templates/<name>/  # Grove scope (in-repo or external config dir)
```

Each template directory contains `scion-agent.yaml`, `system-prompt.md`, and optional files (skills/, agents.md, etc.).

For git groves, the recent externalization change (d0507b1) moved settings and templates to the external grove-config directory (`~/.scion/grove-configs/<slug>__<uuid>/.scion/`), making them structurally consistent with non-git external groves.

### 2.2 Hub Template Storage

Templates on the Hub have a `scope` field (`"global"`, `"grove"`, `"user"`) and a `scopeID` (grove ID for grove-scoped). Storage is organized:
```
gs://bucket/templates/global/<slug>/
gs://bucket/templates/groves/<groveId>/<slug>/
gs://bucket/templates/users/<userId>/<slug>/
```

### 2.3 Template Sync (Current)

`scion templates sync <name>` uploads a local template to the Hub:
- Default scope: **grove** (uses current grove ID)
- `--global` flag: **global** scope
- Performs incremental upload via signed URLs
- Detects harness from config or template name

### 2.4 Template Resolution for Agents

When creating an agent, template resolution checks (in order):
1. Hub grove scope
2. Hub user scope
3. Hub global scope
4. Local grove filesystem
5. Local global filesystem

If a template is found only locally, the user is prompted to upload it (or `--upload-template` auto-uploads).

---

## 3. Selected Approach

After evaluating several approaches (see [Appendix: Rejected Approaches](#appendix-rejected-approaches)), the selected design uses **two complementary strategies**:

- **CLI path**: Auto-sync on grove link (Approach B)
- **Web path**: In-container template sync via dummy agent (Approach C)

### 3.1 CLI: Auto-Sync on Grove Link

**Concept**: When a grove is linked to the Hub (`scion hub link`), automatically detect and sync grove-local templates to the Hub at grove scope.

**Flow**:
1. `scion hub link` detects templates in `.scion/templates/`
2. **Interactive mode**: User is prompted to confirm syncing discovered templates
3. **Non-interactive mode** (`--non-interactive`): Template sync is skipped entirely
4. Each confirmed template is uploaded to Hub at grove scope (same as `scion templates sync`)
5. For linked groves: first broker to link seeds the Hub; subsequent brokers can pull from Hub
6. For git groves: templates are synced when the grove is created from a git URL (requires cloning to discover templates)

**Conflict handling**: If a grove-scoped template with the same name already exists on the Hub, the CLI warns the user and requires a `--force` flag to overwrite.

**Bulk sync**: `scion templates sync --all` is available for explicit bulk upload at any time.

### 3.2 Web: In-Container Template Sync

**Concept**: For all grove types, the web UI uses the existing agent infrastructure to discover and sync templates. This provides a universal mechanism that works regardless of grove type.

**Flow**:
1. From the grove settings page, a "Load Templates" button launches a short-lived dummy agent in the grove
2. The Hub execs into the agent container and runs `scion templates sync --all` via bash
3. The synced templates become available on the Hub at grove scope
4. The grove settings page displays a **read-only list** of loaded templates
5. The agent creation form is **populated by available grove templates**

**Why this works universally**:
- The agent container has the grove's filesystem mounted (whether linked, git-cloned, or hub-native)
- The `scion` CLI inside the container has access to all local templates
- No special git access or broker APIs needed from the Hub

**Hub-native groves**: These are the simplest case — templates are managed entirely on the Hub. The web UI can create/edit templates at grove scope directly without needing the in-container approach. Hub-native groves should take advantage of this direct path.

---

## 4. Challenges by Grove Type

### 4.1 Linked Groves

**Core challenge**: Multiple brokers may link the same grove but have different local templates.

**Scenarios**:
- Broker A has `code-reviewer` template, Broker B does not
- Both brokers have `code-reviewer` but with different content
- A user pushes a template from Broker A; later Broker B updates its local copy

**Resolution strategy**: Warn on name conflicts and require `--force` to overwrite. The first broker to sync sets the Hub version; subsequent syncs from other brokers will see a conflict warning if their version differs.

### 4.2 Git Groves

**Core challenge**: Templates live in git but are only materialized after a clone.

**Scenarios**:
- Templates are committed to `.scion/templates/` in the repo
- Templates are in the externalized grove-config dir (not in git, broker-local)
- Mix of both: some templates in repo, some external

**Key insight**: The recent externalization change (d0507b1) means git grove templates now live in `~/.scion/grove-configs/<slug>__<uuid>/.scion/templates/` on the broker, **not** in the repo. This means:
- Git grove templates behave like linked grove templates from the broker's perspective
- Agents running in git groves create their worktree from the repo clone, but template resolution uses the external config dir

**Resolution**: The in-container sync approach (Section 3.2) works universally here — the dummy agent has access to both in-repo and externalized templates, and `scion templates sync --all` uploads everything it finds.

### 4.3 Hub-Native Groves

**Simplest case**: Templates are managed entirely on the Hub. The web UI creates/edits templates at grove scope directly. No filesystem variance to reconcile, no need for the in-container sync flow.

---

## 5. CLI UX Design

### 5.1 Auto-Sync on `hub link` (Interactive)

```
$ scion hub link
Grove linked to Hub: my-project (id: abc123)

Found 3 grove templates not yet synced to Hub:
  - code-reviewer
  - security-auditor
  - docs-writer

Sync these templates to the Hub? [Y/n] y

Syncing grove templates to Hub...
  code-reviewer:    uploaded (3 files, 2.1KB)
  security-auditor: uploaded (4 files, 3.4KB)
  docs-writer:      uploaded (3 files, 1.8KB)
3 templates synced to grove scope.
```

In non-interactive mode, sync is skipped:
```
$ scion hub link --non-interactive
Grove linked to Hub: my-project (id: abc123)
Skipping template sync (non-interactive mode).
Run 'scion templates sync --all' to upload grove templates.
```

### 5.2 Bulk Sync

```
$ scion templates sync --all
Syncing grove templates to Hub...
  code-reviewer:    uploaded (3 files, 2.1KB)
  security-auditor: uploaded (4 files, 3.4KB)
  docs-writer:      uploaded (3 files, 1.8KB)
3 templates synced to grove scope.
```

### 5.3 Conflict Detection

```
$ scion templates sync code-reviewer
Warning: template 'code-reviewer' already exists at grove scope on the Hub
  (content hash mismatch: local=abc123, hub=def456)
Use --force to overwrite the existing template.
```

### 5.4 Status Command

```
$ scion templates status
Grove: my-project (abc123)

Template            Local    Hub      Status
code-reviewer       yes      yes      synced (hash match)
security-auditor    yes      yes      out of date (local newer)
docs-writer         yes      no       local only
default             yes      yes      synced (global)
custom-gemini       no       yes      hub only
```

### 5.5 Web UI Considerations

The web UI needs to:
1. Show a read-only list of loaded templates in the grove settings page
2. Provide a "Load Templates" button that triggers in-container sync
3. Populate the agent creation form with available grove templates
4. For hub-native groves, support direct template creation/editing inline

---

## 6. Implementation Plan

### Phase 1: CLI Improvements (low effort, high value)
- Add `scion templates sync --all` for bulk grove template sync
- Add `scion templates status` to show sync state between local and Hub
- Add auto-sync prompt during `scion hub link` (opt-out in interactive, skipped in non-interactive)
- Add conflict detection with `--force` flag for overwrites

### Phase 2: Web Template Loading (medium effort)
- Implement "Load Templates" button in grove settings page
- Launch dummy agent, exec `scion templates sync --all` in container
- Display read-only template list in grove settings
- Populate agent creation form with available grove-scoped templates

### Phase 3: Hub-Native Template Editing (medium effort)
- Web UI support for creating/editing grove templates directly for hub-native groves
- Template content editor with file management
- No container needed — direct Hub storage operations

---

## 7. Open Questions

### Architectural
1. **Source of truth**: After a template is synced to the Hub, which version is authoritative - Hub or local filesystem? Should the Hub version be pushed back to brokers?
2. **Sync direction**: Is sync unidirectional (local -> Hub) or bidirectional? If bidirectional, how to handle conflicts?

### UX
3. **Web-first template creation**: Should the web UI support creating grove templates directly (Hub-native) for non-hub-native groves, or is the CLI the primary authoring tool?
4. **Template visibility in agent creation**: When creating an agent from the web UI, should users only see Hub-synced templates, or also indicate that unsynced local templates may exist?

### Technical
5. **Dummy agent lifecycle**: How long does the dummy agent for web-based template sync live? What cleanup is needed?
6. **Content hash stability**: Are content hashes computed the same way on broker and Hub? (Currently yes - SHA-256 of concatenated file hashes)
7. **Externalized git grove templates**: Since templates are now in the external config dir (not in-repo), what is the expected flow for a brand-new broker linking a git grove? The external config dir starts empty - how do templates get there?

---

## Appendix: Rejected Approaches

### Approach A: Broker-Reported Template Inventory (Rejected - too complex)

Brokers would periodically scan their local grove template directories and report the inventory to the Hub via heartbeat. Rejected due to excessive complexity — requires extending the GroveProvider model, adds multiple states to track, and doesn't solve the fundamental problem of getting template content to the Hub.

### Approach D: Template Declaration in Grove Metadata (Not selected)

Groves would declare template requirements in metadata by name/version. Rejected because it adds a two-step process (declare + sync) without solving the core sync problem. Declaration can drift from actual template availability.

### Approach E: Hybrid Broker Inventory + On-Demand Promotion (Not selected)

Combined broker inventory reporting with lazy on-demand sync. Not selected in favor of the simpler B+C combination, which achieves the same goals with less operational complexity.

---

## 8. Related Design Documents

- [Hosted Templates](hosted/hosted-templates.md) - Hub template storage and management
- [Decoupling Templates](decouple-templates.md) - Template/harness separation analysis
- [Git Groves](hosted/git-groves.md) - Git grove architecture
- [Hub Groves](hosted/hub-groves.md) - Hub-native grove design
- [Settings Externalization](../commit d0507b1) - Recent change moving git grove config external
- [Agnostic Template Design](agnostic-template-design.md) - Harness-agnostic template system

# Principals Design: Users, Agents, and Groups

## Status
**Proposed**

## 1. Overview

This document defines the principal model for the Scion Hub's authorization system. A **principal** is any identity that can be referenced in access control policies. This design extends the existing user and agent identity model with a unified principal abstraction and a rich group system that supports both explicit membership and dynamic, system-derived membership.

This document is a prerequisite for the permissions system described in `permissions-design.md`. It establishes the identities and groupings that serve as the subjects in authorization policy evaluation.

### Goals

1. **Unified principal model** - Users, agents, and groups share a common principal interface for policy binding
2. **Human users** - First-class user identities authenticated via OAuth or API keys
3. **Agent identities** - Agents as principals that can be referenced in policies
4. **Agent-as-delegate** - Agents inherit authorization context from the human user who created them
5. **Explicit groups** - Manually curated membership groups of users, agents, and other groups
6. **Dynamic grove groups** - Automatically maintained groups representing all agents in a given grove
7. **Ent-based persistence** - All principal and group data modeled and persisted using [entgo.io](https://entgo.io/)

### Non-Goals

- Policy definition and evaluation (see `permissions-design.md`)
- Authentication mechanisms (see `auth-overview.md`, `server-auth-design.md`)
- Runtime Broker identity as a policy principal (brokers use a separate HMAC trust model)
- Cross-hub principal federation

### Relationship to Existing Design

The `permissions-design.md` document defines groups and policy bindings at a high level. This document refines and extends those definitions with:

- Agent principals (not covered in `permissions-design.md`)
- Agent delegation semantics
- Dynamic grove groups
- Detailed Ent schema definitions for all principal types
- Migration strategy from current SQLite-backed store models

---

## 2. Principal Types

A principal is anything that can appear as a subject in a policy binding. The system defines three principal types:

### 2.1 User

A human user authenticated through OAuth, API key, or dev token. Users are the foundational principal - all authorization decisions ultimately trace back to a human user (either directly or through delegation).

```go
// Existing identity, carried forward with minimal changes.
type User struct {
    ID          string    // UUID primary key
    Email       string    // Canonical identifier (unique)
    DisplayName string
    AvatarURL   string
    Role        string    // System-level: "admin", "member", "viewer"
    Status      string    // "active", "suspended"
    Created     time.Time
    LastLogin   time.Time
}
```

**As a principal:**
- Can be a direct member of explicit groups
- Can be bound directly to policies
- Has a system-level `Role` that may grant administrative bypass
- Owns resources (groves, agents, templates, groups)

### 2.2 Agent

An LLM agent running in a container. Agents authenticate to the Hub via scoped JWT tokens and are always associated with a grove.

```go
// Extended from the existing Agent model with principal-relevant fields.
type Agent struct {
    ID                string    // UUID primary key
    Slug              string    // URL-safe identifier (unique per grove)
    GroveID           string    // FK to Grove - every agent belongs to exactly one grove
    CreatedBy         string    // FK to User - the human who created this agent
    OwnerID           string    // FK to User - current owner (may differ from creator)
    DelegationEnabled bool      // Default: false. When true, creator relationship is policy-addressable.
    Status            string    // provisioning, running, stopped, error
    // ... other operational fields unchanged
}
```

**As a principal:**
- Automatically included in the dynamic grove group for its grove
- Can be an explicit member of other groups
- Can be bound directly to policies (for agent-specific permissions)
- Optionally a **delegate** of its creating user (see Section 3) - off by default, must be explicitly enabled

### 2.3 Group

A named collection of principals. Groups are the primary mechanism for scalable access control. There are two subtypes:

| Group Type | Membership Model | Lifecycle |
|------------|-----------------|-----------|
| **Explicit** | Manually managed - members are added and removed by group admins | Created/deleted by users |
| **Dynamic (Grove)** | Automatically maintained - membership reflects all agents currently in a grove | Created when a grove is created; updated as agents are added/removed |

Both types are first-class groups that can appear in policy bindings. The distinction is in how membership is managed, not in how policies reference them.

---

## 3. Agent Delegation Model

When a user creates an agent, that agent may need to be granted access to resources based on its relationship to that user. Rather than requiring policies to enumerate every individual agent, the system supports a delegation model that makes an agent's creator-relationship addressable in policy.

### 3.1 What Delegation Is (and Is Not)

Delegation **does not** mean the agent "acts as" the user. An agent with delegation enabled is not impersonating its creator and does not automatically inherit the creator's permissions. Instead, delegation establishes a **queryable relationship** between agent and creator that policies can reference.

**Delegation means:** policies can be written that target "agents created by user X" or "agents whose creator is in group Y" as a principal selector. The agent remains a distinct principal with its own identity.

**Delegation does not mean:** the agent silently gains all of the creator's permissions. Access must still be explicitly granted through policies that reference the delegation relationship.

### 3.2 Delegation Flag

Delegation is controlled by a per-agent boolean flag, **off by default**:

```go
type Agent struct {
    // ...existing fields...
    DelegationEnabled bool   // Default: false. When true, creator relationship is policy-addressable.
    CreatedBy         string // FK to User - always set regardless of delegation flag
}
```

- **`DelegationEnabled: false`** (default) - The agent is an independent principal. Its `CreatedBy` field is tracked for audit purposes but is not used during policy evaluation.
- **`DelegationEnabled: true`** - The agent's creator relationship becomes a policy-addressable attribute. Policies can grant or deny access based on "agents delegated from user X."

The flag is set at agent creation time and can be modified by the agent's owner.

### 3.3 Policy Expressions Using Delegation

With delegation enabled, policies can reference agents through their creator:

```json
{
  "name": "alice-agents-grove-read",
  "description": "Alice's delegated agents can read grove resources",
  "scopeType": "grove",
  "scopeId": "grove-uuid",
  "resourceType": "*",
  "actions": ["read"],
  "effect": "allow",
  "conditions": {
    "delegatedFrom": {
      "principalType": "user",
      "principalId": "alice-uuid"
    }
  }
}
```

Or more broadly, targeting delegated agents from any member of a group:

```json
{
  "name": "team-agents-access",
  "description": "Delegated agents from platform-team members can read templates",
  "scopeType": "grove",
  "scopeId": "grove-uuid",
  "resourceType": "template",
  "actions": ["read"],
  "effect": "allow",
  "conditions": {
    "delegatedFromGroup": "platform-team-uuid"
  }
}
```

### 3.4 Access Mechanisms Comparison

| Mechanism | Use Case | Example |
|-----------|----------|---------|
| **Delegation policy** | Grant access to agents based on who created them | "Alice's agents can read grove secrets" |
| **Direct policy** | Agent needs specific, bounded access | Agent can only update its own status |
| **Grove group** | All agents in a grove share access | All agents in `my-project` grove can read project templates |
| **Agent scope (JWT)** | Transport-level access control | Agent token has `agent:status:update` scope |

### 3.5 Delegation Resolution During Policy Evaluation

```
Agent (principal)
  │
  ├── Check: Does agent have direct policy granting access?
  │     └── Yes → Allow (subject to effect)
  │
  ├── Check: Is agent in a group with policy granting access?
  │     └── Yes → Allow (subject to effect)
  │
  └── Check: Is agent.DelegationEnabled == true?
        │
        ├── No → No further checks
        │
        └── Yes → Check: Do any policies with delegation conditions match
              │   this agent's creator (or creator's groups)?
              └── Yes → Allow, but constrained by agent's JWT scopes
```

Direct agent policies and group memberships take precedence over delegation-based policies.

### 3.6 Delegation Constraints

- **Off by default**: Agents must explicitly opt in to delegation at creation time. This prevents unintended privilege inheritance.
- **JWT scope gating**: Even if a delegation policy would grant access, the agent's JWT scopes must include the relevant scope. An agent with only `agent:status:update` scope cannot leverage delegation to read grove secrets.
- **Not transitive through agents**: If Agent A creates Agent B, Agent B's delegation (if enabled) references Agent A's `CreatedBy` user, not Agent A itself. Agents cannot delegate to other agents.
- **Owner transfer**: If an agent's `OwnerID` is transferred to a different user, delegation still follows `CreatedBy`. Ownership determines resource-level control; delegation determines the creator relationship.
- **Suspended users**: If the creating user is suspended, delegation-based policies matching that user do not grant access.

---

## 4. Group Types

### 4.1 Explicit Groups

Explicit groups are manually managed collections of principals. They support:

- **User members** - Human users as direct members
- **Agent members** - Agents as direct members (useful for granting specific agents access to cross-grove resources)
- **Group members** - Nested groups for hierarchical organization (with cycle detection)
- **Membership roles** - Members have a role within the group: `member`, `admin`, or `owner`

```
engineering-team (explicit group)
├── alice (user, role: owner)
├── bob (user, role: admin)
├── charlie (user, role: member)
├── agent-code-reviewer (agent, role: member)
└── platform-subteam (group, role: member)
    ├── dave (user, role: member)
    └── agent-deploy-bot (agent, role: member)
```

**Membership roles within a group:**

| Role | Capabilities |
|------|-------------|
| `member` | Included in group for policy evaluation |
| `admin` | Can add/remove members, update group metadata |
| `owner` | Full control: delete group, transfer ownership, manage admins |

### 4.2 Dynamic Grove Groups

Every grove automatically has an associated dynamic group. This group's membership is the set of all agents currently belonging to that grove.

**Properties:**
- **One-to-one with grove** - Each grove has exactly one dynamic grove group
- **Automatic membership** - When an agent is created in a grove, it is automatically a member; when removed, it is automatically excluded
- **No manual membership changes** - Members cannot be manually added or removed
- **Policy-bindable** - Can be referenced in policy bindings like any other group
- **Naming convention** - Slug follows pattern `grove:<grove-slug>:agents` (e.g., `grove:my-project:agents`)

**Use cases:**
- Grant all agents in a grove read access to grove-scoped templates
- Allow all agents in a grove to read grove-level environment variables
- Restrict all agents in a grove from accessing other groves

**Dynamic membership resolution:**

Rather than materializing membership records, grove group membership is resolved at query time:

```go
// Pseudo-query: "Is agent X a member of grove group G?"
// Resolved as: "Does agent X belong to the grove associated with group G?"
agent.GroveID == groveGroup.GroveID
```

This avoids the need to synchronize membership records when agents are created or destroyed.

### 4.3 Group Type Comparison

| Aspect | Explicit Group | Dynamic Grove Group |
|--------|---------------|-------------------|
| Creation | Manual (API/UI) | Automatic (when grove is created) |
| Membership | Manual add/remove | Automatic (agents in grove) |
| Member types | Users, agents, groups | Agents only |
| Nested groups | Yes | No |
| Membership roles | member, admin, owner | N/A (all members equivalent) |
| Deletable | Yes | Only when grove is deleted |
| Mutable metadata | Yes (name, description, labels) | Limited (description, labels) |

---

## 5. Data Model (Ent Schemas)

The following Ent schemas define the persistence layer for principals and groups. These schemas supersede the current `store.Group` and `store.GroupMember` models defined in `pkg/store/models.go`.

### 5.1 Principal (Edge-Type Pattern)

Rather than creating a separate `Principal` entity, the system uses Ent's polymorphic edge pattern. Policy bindings reference principals through typed edges:

```go
// PolicyBinding uses separate edges for each principal type.
// This avoids the need for a discriminator column and leverages
// Ent's native type safety.
edge.To("user", User.Type).Unique()
edge.To("group", Group.Type).Unique()
edge.To("agent", Agent.Type).Unique()
```

Only one of these edges is populated per binding. The `PrincipalType` field serves as a discriminator for API serialization and query filtering.

### 5.2 Ent Schema: User

Extends the current `store.User` with group and policy edges.

```go
package schema

import (
    "time"

    "entgo.io/ent"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
    "github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
    ent.Schema
}

func (User) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(uuid.New),
        field.String("email").NotEmpty().Unique(),
        field.String("display_name").NotEmpty(),
        field.String("avatar_url").Optional(),
        field.Enum("role").Values("admin", "member", "viewer").Default("member"),
        field.Enum("status").Values("active", "suspended").Default("active"),
        field.JSON("preferences", &UserPreferences{}).Optional(),
        field.Time("created").Default(time.Now),
        field.Time("last_login").Optional(),
    }
}

func (User) Edges() []ent.Edge {
    return []ent.Edge{
        // Groups this user belongs to (explicit membership)
        edge.From("groups", Group.Type).Ref("user_members"),

        // Policies bound directly to this user
        edge.From("policies", PolicyBinding.Type).Ref("user"),

        // Groups owned by this user
        edge.From("owned_groups", Group.Type).Ref("owner"),

        // Agents created by this user (delegation chain)
        edge.To("created_agents", Agent.Type),

        // Agents owned by this user
        edge.To("owned_agents", Agent.Type),
    }
}
```

### 5.3 Ent Schema: Agent

Extends the current `store.Agent` with principal-relevant edges.

```go
// Agent holds the schema definition for the Agent entity.
type Agent struct {
    ent.Schema
}

func (Agent) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(uuid.New),
        field.String("slug").NotEmpty(),
        field.String("name").NotEmpty(),
        field.String("template").Optional(),

        // Grove association
        field.UUID("grove_id", uuid.UUID{}),

        // Status fields
        field.Enum("status").Values("provisioning", "running", "stopped", "error", "pending").
            Default("pending"),

        // Delegation: who created this agent and whether delegation is active
        field.UUID("created_by", uuid.UUID{}).Optional(),
        field.UUID("owner_id", uuid.UUID{}).Optional(),
        field.Bool("delegation_enabled").Default(false),

        field.String("visibility").Default("private"),

        field.Time("created").Default(time.Now),
        field.Time("updated").Default(time.Now).UpdateDefault(time.Now),

        // ... other operational fields
    }
}

func (Agent) Edges() []ent.Edge {
    return []ent.Edge{
        // The grove this agent belongs to
        edge.From("grove", Grove.Type).Ref("agents").Unique().Required(),

        // The user who created this agent (delegation source)
        edge.From("creator", User.Type).Ref("created_agents").Unique(),

        // The user who owns this agent
        edge.From("owner", User.Type).Ref("owned_agents").Unique(),

        // Explicit group memberships (agent can be in explicit groups)
        edge.From("groups", Group.Type).Ref("agent_members"),

        // Policies bound directly to this agent
        edge.From("policies", PolicyBinding.Type).Ref("agent"),
    }
}
```

### 5.4 Ent Schema: Group

```go
// Group holds the schema definition for the Group entity.
type Group struct {
    ent.Schema
}

func (Group) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(uuid.New),
        field.String("name").NotEmpty(),
        field.String("slug").NotEmpty().Unique(),
        field.String("description").Optional(),

        // Group type discriminator
        field.Enum("group_type").Values("explicit", "grove_agents").Default("explicit"),

        // For grove_agents groups: the associated grove
        field.UUID("grove_id", uuid.UUID{}).Optional().Nillable(),

        // Metadata
        field.JSON("labels", map[string]string{}).Optional(),
        field.JSON("annotations", map[string]string{}).Optional(),

        // Timestamps
        field.Time("created").Default(time.Now),
        field.Time("updated").Default(time.Now).UpdateDefault(time.Now),

        // Ownership
        field.UUID("created_by", uuid.UUID{}).Optional(),
        field.UUID("owner_id", uuid.UUID{}).Optional(),
    }
}

func (Group) Edges() []ent.Edge {
    return []ent.Edge{
        // User members of this group
        edge.To("user_members", User.Type),

        // Agent members of this group (explicit membership only)
        edge.To("agent_members", Agent.Type),

        // Child groups (groups contained within this group)
        edge.To("child_groups", Group.Type),

        // Parent groups (groups that contain this group)
        edge.From("parent_groups", Group.Type).Ref("child_groups"),

        // Owner of this group
        edge.To("owner", User.Type).Unique(),

        // For grove_agents groups: the associated grove
        edge.To("grove", Grove.Type).Unique(),

        // Policies bound to this group as a principal
        edge.From("policies", PolicyBinding.Type).Ref("group"),
    }
}

// Indexes of the Group.
func (Group) Indexes() []ent.Index {
    return []ent.Index{
        // Grove groups are unique per grove
        index.Fields("grove_id").Unique().Where(
            sqljson.ValueEQ("group_type", "grove_agents"),
        ),
    }
}
```

### 5.5 Ent Schema: GroupMembership (Edge Schema)

For explicit groups, membership carries metadata (role, timestamp, who added). Ent supports edge schemas for this purpose.

```go
// GroupMembership holds metadata for group membership edges.
// This is used as an edge schema on the user_members and agent_members edges.
type GroupMembership struct {
    ent.Schema
}

func (GroupMembership) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(uuid.New),

        // Membership role within the group
        field.Enum("role").Values("member", "admin", "owner").Default("member"),

        // Who added this member
        field.UUID("added_by", uuid.UUID{}).Optional(),

        // When the member was added
        field.Time("added_at").Default(time.Now),
    }
}

func (GroupMembership) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("group", Group.Type).Unique().Required(),
        edge.To("user", User.Type).Unique(),
        edge.To("agent", Agent.Type).Unique(),
    }
}
```

### 5.6 Ent Schema: PolicyBinding

Policy bindings link principals to policies. Each binding references exactly one principal (user, agent, or group).

```go
// PolicyBinding holds the schema definition for linking principals to policies.
type PolicyBinding struct {
    ent.Schema
}

func (PolicyBinding) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(uuid.New),

        // Discriminator for API serialization
        field.Enum("principal_type").Values("user", "agent", "group"),

        field.Time("created").Default(time.Now),
        field.UUID("created_by", uuid.UUID{}).Optional(),
    }
}

func (PolicyBinding) Edges() []ent.Edge {
    return []ent.Edge{
        // The policy this binding belongs to
        edge.From("policy", Policy.Type).Ref("bindings").Unique().Required(),

        // Principal edges (exactly one is populated)
        edge.To("user", User.Type).Unique(),
        edge.To("group", Group.Type).Unique(),
        edge.To("agent", Agent.Type).Unique(),
    }
}
```

---

## 6. Principal Resolution for Policy Evaluation

When the authorization system needs to determine whether a principal has access to a resource, it must resolve the principal's **effective group memberships**. This section describes the resolution algorithm.

### 6.1 Effective Groups for a User

```go
func getEffectiveGroups(ctx context.Context, client *ent.Client, userID uuid.UUID) ([]uuid.UUID, error) {
    // 1. Get all explicit groups the user is a direct member of
    directGroups, err := client.User.Query().
        Where(user.ID(userID)).
        QueryGroups().
        IDs(ctx)

    // 2. Recursively expand parent groups (transitive membership)
    allGroups := expandParentGroups(ctx, client, directGroups)

    return allGroups, nil
}

func expandParentGroups(ctx context.Context, client *ent.Client, groupIDs []uuid.UUID) []uuid.UUID {
    visited := make(map[uuid.UUID]bool)
    queue := append([]uuid.UUID{}, groupIDs...)

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]

        if visited[current] {
            continue
        }
        visited[current] = true

        // Find all groups that contain this group
        parents, _ := client.Group.Query().
            Where(group.ID(current)).
            QueryParentGroups().
            IDs(ctx)

        queue = append(queue, parents...)
    }

    result := make([]uuid.UUID, 0, len(visited))
    for id := range visited {
        result = append(result, id)
    }
    return result
}
```

### 6.2 Effective Groups for an Agent

```go
func getEffectiveGroupsForAgent(ctx context.Context, client *ent.Client, agentID uuid.UUID) ([]uuid.UUID, error) {
    agent, err := client.Agent.Get(ctx, agentID)
    if err != nil {
        return nil, err
    }

    var allGroups []uuid.UUID

    // 1. Get the grove's dynamic group
    groveGroup, err := client.Group.Query().
        Where(
            group.GroupTypeEQ(group.GroupTypeGroveAgents),
            group.GroveID(agent.GroveID),
        ).
        Only(ctx)
    if err == nil {
        allGroups = append(allGroups, groveGroup.ID)
    }

    // 2. Get explicit groups the agent is a direct member of
    explicitGroups, err := client.Agent.Query().
        Where(agent_ent.ID(agentID)).
        QueryGroups().
        IDs(ctx)

    allGroups = append(allGroups, explicitGroups...)

    // 3. Expand parent groups transitively
    allGroups = expandParentGroups(ctx, client, allGroups)

    return allGroups, nil
}
```

### 6.3 Delegation-Based Policy Matching

When an agent has `DelegationEnabled == true`, the policy engine checks for policies
with delegation conditions that match the agent's creator.

```go
func findDelegationPolicies(
    ctx context.Context,
    client *ent.Client,
    agentID uuid.UUID,
    resource Resource,
    action Action,
) ([]*Policy, error) {
    // Get the agent with delegation flag and creator
    agent, err := client.Agent.Query().
        Where(agent_ent.ID(agentID)).
        WithCreator().
        Only(ctx)
    if err != nil {
        return nil, err
    }

    // Delegation must be explicitly enabled
    if !agent.DelegationEnabled {
        return nil, nil
    }

    creator := agent.Edges.Creator
    if creator == nil {
        return nil, nil // No delegation possible without a creator
    }

    // Suspended creators cannot be delegation sources
    if creator.Status == user.StatusSuspended {
        return nil, nil
    }

    // Find policies with delegation conditions matching this creator
    // This checks both direct "delegatedFrom" user matches and
    // "delegatedFromGroup" matches against the creator's effective groups
    creatorGroups, err := getEffectiveGroups(ctx, client, creator.ID)
    if err != nil {
        return nil, err
    }

    // Query policies that have delegation conditions matching either
    // the creator's ID or any of the creator's groups
    policies, err := findPoliciesWithDelegationConditions(
        ctx, client, creator.ID, creatorGroups, resource, action,
    )

    return policies, err
}
```

---

## 7. API Endpoints

### 7.1 Group Management

These extend the existing group endpoints in `handlers_groups.go` with support for the new group types and agent members.

#### Create Group
```
POST /api/v1/groups
```

**Request Body:**
```json
{
  "name": "Platform Team",
  "slug": "platform-team",
  "description": "Platform engineering team",
  "labels": {"department": "engineering"},
  "groupType": "explicit"
}
```

`groupType` defaults to `"explicit"`. Creating `grove_agents` groups directly is not allowed - they are created automatically when a grove is registered.

#### List Group Members
```
GET /api/v1/groups/{groupId}/members
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `expand` | bool | Include transitive members from nested groups (default: false) |
| `type` | string | Filter by member type: `user`, `agent`, `group` |

**Response:**
```json
{
  "members": [
    {
      "principalType": "user",
      "principalId": "user-uuid",
      "displayName": "Alice",
      "role": "admin",
      "addedAt": "2026-02-01T00:00:00Z",
      "addedBy": "owner-uuid"
    },
    {
      "principalType": "agent",
      "principalId": "agent-uuid",
      "displayName": "code-reviewer",
      "role": "member",
      "addedAt": "2026-02-01T00:00:00Z",
      "addedBy": "owner-uuid"
    }
  ]
}
```

For dynamic grove groups, the response lists all agents in the grove. The `role` field is omitted (or set to `"member"`) and `addedBy` is `"system"`.

#### Add Group Member
```
POST /api/v1/groups/{groupId}/members
```

**Request Body:**
```json
{
  "principalType": "agent",
  "principalId": "agent-uuid",
  "role": "member"
}
```

- `principalType` accepts `"user"`, `"agent"`, or `"group"`
- Returns `409 Conflict` if the membership already exists
- Returns `400 Bad Request` if the target group is a `grove_agents` group (dynamic groups do not accept manual membership changes)
- Returns `400 Bad Request` if adding a group member would create a cycle

#### Remove Group Member
```
DELETE /api/v1/groups/{groupId}/members/{principalType}/{principalId}
```

Returns `400 Bad Request` if the target group is a `grove_agents` group.

### 7.2 Principal Query Endpoints

#### Get My Groups
```
GET /api/v1/users/me/groups
```

Returns all groups the authenticated user belongs to (direct and transitive).

#### Get Agent's Groups
```
GET /api/v1/agents/{agentId}/groups
```

Returns all groups the agent belongs to (explicit memberships + grove group).

#### Resolve Principal
```
GET /api/v1/principals/{principalType}/{principalId}
```

Returns summary information about a principal and their group memberships. Useful for debugging and UI display.

**Response:**
```json
{
  "principal": {
    "type": "agent",
    "id": "agent-uuid",
    "displayName": "code-reviewer",
    "groveId": "grove-uuid"
  },
  "directGroups": ["grove:my-project:agents", "platform-team"],
  "effectiveGroups": ["grove:my-project:agents", "platform-team", "engineering"],
  "delegatesFrom": {
    "type": "user",
    "id": "user-uuid",
    "displayName": "Alice"
  }
}
```

---

## 8. Lifecycle Events

### 8.1 Grove Created

When a new grove is registered in the Hub:

1. Create a dynamic grove group with:
   - `name`: `"<grove-name> Agents"`
   - `slug`: `"grove:<grove-slug>:agents"`
   - `group_type`: `"grove_agents"`
   - `grove_id`: the grove's ID
   - `owner_id`: the grove's owner

### 8.2 Grove Deleted

When a grove is deleted:

1. Delete the associated dynamic grove group
2. Remove any policy bindings that referenced the grove group
3. Log the cascading deletions in the audit trail

### 8.3 Agent Created

When an agent is provisioned:

1. Agent is automatically visible as a member of the grove's dynamic group (no explicit membership record needed)
2. Set `CreatedBy` to the authenticated user's ID
3. Set `DelegationEnabled` based on the creation request (defaults to `false`)
4. If the agent's template specifies default group memberships, add those explicit memberships

### 8.4 Agent Deleted

When an agent is removed:

1. Agent is automatically excluded from the grove's dynamic group
2. Remove any explicit group memberships for the agent
3. Remove any direct policy bindings for the agent

### 8.5 User Suspended

When a user is suspended:

1. The user's direct access is blocked (handled by auth middleware)
2. Delegation from this user to their agents is suspended
3. The user remains a member of groups but cannot exercise permissions through them
4. Group memberships are preserved for reactivation

---

## 9. Cycle Detection

Groups can contain other groups, which creates the possibility of cycles. The system prevents cycles at write time.

### 9.1 Algorithm

Before adding group B as a member of group A, check whether A is reachable from B by traversing B's child_groups edges:

```go
func wouldCreateCycle(ctx context.Context, client *ent.Client, parentID, childID uuid.UUID) (bool, error) {
    if parentID == childID {
        return true, nil
    }

    // BFS from childID downward through child_groups
    visited := make(map[uuid.UUID]bool)
    queue := []uuid.UUID{childID}

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]

        if visited[current] {
            continue
        }
        visited[current] = true

        children, err := client.Group.Query().
            Where(group.ID(current)).
            QueryChildGroups().
            IDs(ctx)
        if err != nil {
            return false, err
        }

        for _, id := range children {
            if id == parentID {
                return true, nil // Cycle detected
            }
            queue = append(queue, id)
        }
    }

    return false, nil
}
```

### 9.2 Depth Limit

To bound traversal cost, the system enforces a maximum group nesting depth of **10 levels**. Attempts to create nesting deeper than this limit are rejected with a `400 Bad Request`.

---

## 10. Ent Integration Strategy

### 10.1 Migration from Current Store

The current system uses a hand-written `Store` interface (`pkg/store/store.go`) backed by SQLite. The Ent integration follows a phased approach:

**Phase 1: Parallel schemas** - Define Ent schemas for User, Agent, Group, GroupMembership, and PolicyBinding. Generate Ent code. Run Ent alongside the existing store for group/membership operations only.

**Phase 2: Group operations on Ent** - Migrate `GroupStore` interface implementation from SQLite to Ent-generated client. The existing `Store` interface methods (`CreateGroup`, `GetGroup`, etc.) are implemented by delegating to the Ent client internally.

**Phase 3: Principal resolution on Ent** - Implement `GetEffectiveGroups` and delegation resolution using Ent graph traversals. These are new capabilities not present in the current store.

**Phase 4: Full migration** - Migrate remaining entity operations (User, Agent, Grove, etc.) to Ent, replacing the SQLite store entirely. This is a broader effort beyond the scope of this document.

### 10.2 Ent Code Generation

```
pkg/ent/
├── schema/
│   ├── user.go
│   ├── agent.go
│   ├── group.go
│   ├── groupmembership.go
│   ├── policybinding.go
│   ├── policy.go
│   └── grove.go
├── generate.go          // go:generate entc generate ./schema
├── client.go            // Generated
├── user.go              // Generated
├── agent.go             // Generated
├── group.go             // Generated
└── ...
```

### 10.3 Database Backend

Ent supports multiple database backends. For development and solo mode, SQLite continues to be used. For production hosted deployments, PostgreSQL is the target:

| Context | Database | Notes |
|---------|----------|-------|
| Development / Solo | SQLite | Single-file, zero-config |
| Testing | SQLite (in-memory) | Fast, isolated test runs |
| Production (Hosted) | PostgreSQL | Concurrent access, robust transactions |

Ent handles the abstraction. Schema definitions are database-agnostic.

---

## 11. Security Considerations

### 11.1 Principal Impersonation

- Agents authenticate with scoped JWTs; their principal identity is derived from the token's `Subject` claim
- The `CreatedBy` field that establishes delegation cannot be modified after agent creation
- Admin users cannot impersonate other users through the delegation mechanism

### 11.2 Group Membership Escalation

- Adding members to a group requires `admin` role within the group or the `add_member` action via policy
- Dynamic grove groups cannot have members added or removed manually, preventing escalation through grove groups
- Cycle detection prevents infinite loops in group resolution

### 11.3 Cascading Deletes

| Deleted Entity | Impact on Principals/Groups |
|----------------|---------------------------|
| User | Remove from all explicit groups; delegation suspended for created agents; direct policy bindings removed |
| Agent | Remove from all explicit groups; automatically excluded from grove group; direct policy bindings removed |
| Group | Remove all memberships (both as parent and child); remove all policy bindings referencing this group |
| Grove | Delete associated grove group; cascading to grove group's policy bindings |

### 11.4 Audit Trail

All principal and group operations are logged:

- Group create/update/delete
- Member add/remove
- Policy binding create/remove
- Delegation resolution decisions (at DEBUG level)

---

## 12. Alternative Approaches Considered

### 12.1 Unified Principal Table vs. Type-Specific Edges

| Approach | Pros | Cons |
|----------|------|------|
| **Unified `principals` table** with type discriminator | Single FK in policy bindings; simpler queries | Loses Ent type safety; polymorphic queries are less ergonomic in Ent |
| **Type-specific edges** (chosen) | Full Ent type safety; natural Go types; clear schema | Multiple nullable FKs in policy binding; slightly more complex binding queries |

**Decision:** Type-specific edges align better with Ent's design philosophy and provide compile-time guarantees that principals reference valid entities.

### 12.2 Materialized vs. Query-Time Grove Group Membership

| Approach | Pros | Cons |
|----------|------|------|
| **Materialized** (write membership records) | Uniform query pattern for all group types; simpler membership listing | Must synchronize on every agent create/delete; risk of stale data |
| **Query-time** (chosen) | No synchronization needed; always consistent; no extra writes | Different query path for grove groups; slightly more complex resolution |

**Decision:** Query-time resolution avoids synchronization bugs and is simpler overall. The query (`WHERE grove_id = ?`) is trivially indexed.

### 12.3 Agent Delegation vs. Separate Service Account Model

| Approach | Pros | Cons |
|----------|------|------|
| **Delegation** (chosen) | Leverages existing user permissions; no separate identity management | Tight coupling to creator; requires careful scope gating |
| **Service accounts** | Independently managed identities; clear separation | Additional identity lifecycle; more policies to manage; UX complexity |

**Decision:** Delegation is more natural for Scion's model where agents are ephemeral extensions of human users. Service accounts may be added later for long-lived automated agents.

---

## 13. Implementation Phases

### Phase 1: Ent Schema Foundation
- [x] Initialize Ent project structure in `pkg/ent/`
- [x] Define schemas for User, Agent, Group, GroupMembership, PolicyBinding, Grove
- [x] Run code generation and verify schema compilation
- [x] Write Ent client initialization (SQLite for dev, PostgreSQL for prod)
- [x] Implement database migration via Ent auto-migration

### Phase 2: Group Operations
- [x] Implement explicit group CRUD using Ent client
- [x] Implement group membership add/remove with cycle detection
- [x] Implement agent-as-member support in group membership
- [x] Add `group_type` discriminator and validation (reject manual membership changes on grove groups)
- [x] Write adapter layer bridging `store.GroupStore` interface to Ent client
- [x] Update `handlers_groups.go` to support `principalType: "agent"`

### Phase 3: Dynamic Grove Groups
- [ ] Implement automatic grove group creation on grove registration
- [ ] Implement grove group deletion on grove deletion
- [ ] Implement query-time membership resolution for grove groups
- [ ] Add grove group to effective group expansion

### Phase 4: Principal Resolution
- [ ] Implement `GetEffectiveGroups` for users (transitive group expansion)
- [ ] Implement `GetEffectiveGroupsForAgent` (grove group + explicit groups + transitive expansion)
- [ ] Implement delegation resolution (`checkDelegatedAccess`)
- [ ] Add principal query endpoints (`/users/me/groups`, `/agents/{id}/groups`, `/principals/{type}/{id}`)

### Phase 5: Integration with Permissions System
- [ ] Wire principal resolution into the policy evaluation algorithm from `permissions-design.md`
- [ ] Implement `PolicyBinding` CRUD using Ent
- [ ] Add delegation check as final fallback in access resolution
- [ ] End-to-end integration tests covering all principal types

---

## 14. Resolved Questions

The following questions were raised during the design process and have been resolved.

### 14.1 Agent Group Membership Persistence

**Question:** Should agents that are explicitly added to groups retain that membership across agent restarts (stop/start cycles)?

**Decision:** Yes. Explicit group memberships are tied to the agent's identity (UUID), not its runtime lifecycle. An agent that is stopped and restarted retains its memberships. Only deletion removes memberships.

### 14.2 Delegation Model

**Question:** Should users be able to control delegation for agents they create?

**Decision:** Per-agent flag, off by default. Delegation does not mean the agent "acts as" the user as a principal. It means the agent is an elevated type of principal whose creator-relationship is addressable in policy — e.g., "John's agents are allowed to X." See Section 3 for the full design.

### 14.3 Grove Group Visibility in Group Listings

**Question:** Should grove groups appear in the general `/api/v1/groups` listing, or should they be filtered to grove-specific endpoints?

**Decision:** They appear in the general listing with `groupType: "grove_agents"` so clients can filter. A `groupType` query parameter on the list endpoint enables filtering.

### 14.4 Cross-Grove Agent Membership

**Question:** Can an agent in Grove A be an explicit member of a group used in policies for Grove B?

**Decision:** Yes. Explicit group membership is not scoped to a grove. This enables cross-grove collaboration patterns (e.g., a shared "CI agents" group). However, the agent's JWT scopes still limit what it can actually do.

### 14.5 Maximum Group Size

**Question:** Should there be a limit on the number of members in a single group?

**Decision:** No hard limit initially. Monitor performance of group expansion queries and add limits if needed. The depth limit (10 levels) bounds transitive expansion cost, but breadth is unconstrained.

---

## 15. References

- **Permissions System Design:** `permissions-design.md`
- **Authentication Overview:** `auth-overview.md`
- **Server Authentication Design:** `server-auth-design.md`
- **Agent Token Service:** `sciontool-auth.md`
- **Implementation Milestones:** `auth-milestones.md`
- **Ent Documentation:** https://entgo.io/
- **Ent Edge Schemas:** https://entgo.io/docs/schema-edges/

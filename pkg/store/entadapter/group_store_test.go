// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !no_sqlite

package entadapter

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/ptone/scion-agent/pkg/ent/entc"
	"github.com/ptone/scion-agent/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGroupStore(t *testing.T) *GroupStore {
	t.Helper()
	client, err := entc.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, entc.AutoMigrate(context.Background(), client))

	// Create a test user for membership tests
	_, err = client.User.Create().
		SetID(testUserUID).
		SetEmail("test@example.com").
		SetDisplayName("Test User").
		Save(context.Background())
	require.NoError(t, err)

	// Create a grove (required FK for agent)
	grove, err := client.Grove.Create().
		SetID(testGroveUID).
		SetName("test-grove").
		SetSlug("test-grove").
		Save(context.Background())
	require.NoError(t, err)

	// Create a test agent for membership tests
	_, err = client.Agent.Create().
		SetID(testAgentUID).
		SetName("test-agent").
		SetSlug("test-agent").
		SetGrove(grove).
		Save(context.Background())
	require.NoError(t, err)

	return NewGroupStore(client)
}

var (
	testUserUID  = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	testAgentUID = uuid.MustParse("20000000-0000-0000-0000-000000000001")
	testGroveUID = uuid.MustParse("30000000-0000-0000-0000-000000000001")
)

func TestCreateGroup(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:          uuid.New().String(),
		Name:        "Engineering",
		Slug:        "engineering",
		Description: "Engineering team",
	}

	err := gs.CreateGroup(ctx, g)
	require.NoError(t, err)
	assert.False(t, g.Created.IsZero())
	assert.False(t, g.Updated.IsZero())
	assert.Equal(t, store.GroupTypeExplicit, g.GroupType)
}

func TestCreateGroupDuplicate(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Engineering",
		Slug: "engineering",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	g2 := &store.Group{
		ID:   uuid.New().String(),
		Name: "Engineering 2",
		Slug: "engineering", // same slug
	}
	err := gs.CreateGroup(ctx, g2)
	assert.ErrorIs(t, err, store.ErrAlreadyExists)
}

func TestGetGroup(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	g := &store.Group{
		ID:          id,
		Name:        "Platform",
		Slug:        "platform",
		Description: "Platform team",
		Labels:      map[string]string{"dept": "eng"},
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	got, err := gs.GetGroup(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "Platform", got.Name)
	assert.Equal(t, "platform", got.Slug)
	assert.Equal(t, "Platform team", got.Description)
	assert.Equal(t, "eng", got.Labels["dept"])
	assert.Equal(t, store.GroupTypeExplicit, got.GroupType)
}

func TestGetGroupNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	_, err := gs.GetGroup(ctx, uuid.New().String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetGroupBySlug(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Ops Team",
		Slug: "ops-team",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	got, err := gs.GetGroupBySlug(ctx, "ops-team")
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
	assert.Equal(t, "Ops Team", got.Name)
}

func TestGetGroupBySlugNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	_, err := gs.GetGroupBySlug(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestUpdateGroup(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Old Name",
		Slug: "old-name",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	g.Name = "New Name"
	g.Description = "Updated"
	err := gs.UpdateGroup(ctx, g)
	require.NoError(t, err)

	got, err := gs.GetGroup(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
	assert.Equal(t, "Updated", got.Description)
}

func TestUpdateGroupNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Ghost",
		Slug: "ghost",
	}
	err := gs.UpdateGroup(ctx, g)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteGroup(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Delete Me",
		Slug: "delete-me",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	err := gs.DeleteGroup(ctx, g.ID)
	require.NoError(t, err)

	_, err = gs.GetGroup(ctx, g.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteGroupNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	err := gs.DeleteGroup(ctx, uuid.New().String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestListGroups(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Group " + string(rune('A'+i)),
			Slug: "group-" + string(rune('a'+i)),
		}
		require.NoError(t, gs.CreateGroup(ctx, g))
	}

	result, err := gs.ListGroups(ctx, store.GroupFilter{}, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, result.Items, 3)
}

func TestListGroupsWithGroupTypeFilter(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g1 := &store.Group{
		ID:        uuid.New().String(),
		Name:      "Explicit Group",
		Slug:      "explicit-group",
		GroupType: store.GroupTypeExplicit,
	}
	require.NoError(t, gs.CreateGroup(ctx, g1))

	// For grove_agents, we create directly to bypass the API guard
	g2 := &store.Group{
		ID:        uuid.New().String(),
		Name:      "Grove Group",
		Slug:      "grove-group",
		GroupType: store.GroupTypeGroveAgents,
	}
	require.NoError(t, gs.CreateGroup(ctx, g2))

	result, err := gs.ListGroups(ctx, store.GroupFilter{GroupType: store.GroupTypeExplicit}, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, store.GroupTypeExplicit, result.Items[0].GroupType)

	result, err = gs.ListGroups(ctx, store.GroupFilter{GroupType: store.GroupTypeGroveAgents}, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, store.GroupTypeGroveAgents, result.Items[0].GroupType)
}

func TestListGroupsWithLimit(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Group " + string(rune('A'+i)),
			Slug: "group-" + string(rune('a'+i)),
		}
		require.NoError(t, gs.CreateGroup(ctx, g))
	}

	result, err := gs.ListGroups(ctx, store.GroupFilter{}, store.ListOptions{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, result.TotalCount)
	assert.Len(t, result.Items, 2)
}

func TestAddGroupMemberUser(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}

	err := gs.AddGroupMember(ctx, member)
	require.NoError(t, err)
	assert.False(t, member.AddedAt.IsZero())
}

func TestAddGroupMemberAgent(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group-agent",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeAgent,
		MemberID:   testAgentUID.String(),
		Role:       store.GroupMemberRoleMember,
	}

	err := gs.AddGroupMember(ctx, member)
	require.NoError(t, err)
	assert.False(t, member.AddedAt.IsZero())

	// Verify we can get the membership back
	members, err := gs.GetGroupMembers(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, store.GroupMemberTypeAgent, members[0].MemberType)
	assert.Equal(t, testAgentUID.String(), members[0].MemberID)
}

func TestAddGroupMemberDuplicate(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group-dup",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}

	require.NoError(t, gs.AddGroupMember(ctx, member))

	err := gs.AddGroupMember(ctx, member)
	assert.ErrorIs(t, err, store.ErrAlreadyExists)
}

func TestAddGroupMemberGroupNesting(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	parent := &store.Group{
		ID:   uuid.New().String(),
		Name: "Parent",
		Slug: "parent",
	}
	child := &store.Group{
		ID:   uuid.New().String(),
		Name: "Child",
		Slug: "child",
	}
	require.NoError(t, gs.CreateGroup(ctx, parent))
	require.NoError(t, gs.CreateGroup(ctx, child))

	member := &store.GroupMember{
		GroupID:    parent.ID,
		MemberType: store.GroupMemberTypeGroup,
		MemberID:   child.ID,
		Role:       store.GroupMemberRoleMember,
	}

	err := gs.AddGroupMember(ctx, member)
	require.NoError(t, err)

	// Verify child shows up in members
	members, err := gs.GetGroupMembers(ctx, parent.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, store.GroupMemberTypeGroup, members[0].MemberType)
	assert.Equal(t, child.ID, members[0].MemberID)
}

func TestRemoveGroupMemberUser(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group-rm",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}
	require.NoError(t, gs.AddGroupMember(ctx, member))

	err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	require.NoError(t, err)

	_, err = gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestRemoveGroupMemberAgent(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group-rm-agent",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeAgent,
		MemberID:   testAgentUID.String(),
		Role:       store.GroupMemberRoleMember,
	}
	require.NoError(t, gs.AddGroupMember(ctx, member))

	err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String())
	require.NoError(t, err)

	_, err = gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestRemoveGroupMemberNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test Group",
		Slug: "test-group-rm-nf",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetGroupMembers(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Mixed Group",
		Slug: "mixed-group",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	child := &store.Group{
		ID:   uuid.New().String(),
		Name: "Child Group",
		Slug: "child-group",
	}
	require.NoError(t, gs.CreateGroup(ctx, child))

	// Add user member
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}))

	// Add agent member
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeAgent,
		MemberID:   testAgentUID.String(),
		Role:       store.GroupMemberRoleMember,
	}))

	// Add group member
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeGroup,
		MemberID:   child.ID,
		Role:       store.GroupMemberRoleMember,
	}))

	members, err := gs.GetGroupMembers(ctx, g.ID)
	require.NoError(t, err)
	assert.Len(t, members, 3)

	// Count by type
	typeCounts := map[string]int{}
	for _, m := range members {
		typeCounts[m.MemberType]++
	}
	assert.Equal(t, 1, typeCounts[store.GroupMemberTypeUser])
	assert.Equal(t, 1, typeCounts[store.GroupMemberTypeAgent])
	assert.Equal(t, 1, typeCounts[store.GroupMemberTypeGroup])
}

func TestGetUserGroups(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g1 := &store.Group{
		ID:   uuid.New().String(),
		Name: "Group 1",
		Slug: "group-1",
	}
	g2 := &store.Group{
		ID:   uuid.New().String(),
		Name: "Group 2",
		Slug: "group-2",
	}
	require.NoError(t, gs.CreateGroup(ctx, g1))
	require.NoError(t, gs.CreateGroup(ctx, g2))

	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g1.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}))
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g2.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleAdmin,
	}))

	groups, err := gs.GetUserGroups(ctx, testUserUID.String())
	require.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestGetGroupMembershipUser(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test",
		Slug: "test-membership",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleAdmin,
	}))

	m, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	require.NoError(t, err)
	assert.Equal(t, store.GroupMemberRoleAdmin, m.Role)
	assert.Equal(t, store.GroupMemberTypeUser, m.MemberType)
	assert.Equal(t, testUserUID.String(), m.MemberID)
}

func TestGetGroupMembershipGroup(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	parent := &store.Group{
		ID:   uuid.New().String(),
		Name: "Parent",
		Slug: "parent-gm",
	}
	child := &store.Group{
		ID:   uuid.New().String(),
		Name: "Child",
		Slug: "child-gm",
	}
	require.NoError(t, gs.CreateGroup(ctx, parent))
	require.NoError(t, gs.CreateGroup(ctx, child))

	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    parent.ID,
		MemberType: store.GroupMemberTypeGroup,
		MemberID:   child.ID,
		Role:       store.GroupMemberRoleMember,
	}))

	m, err := gs.GetGroupMembership(ctx, parent.ID, store.GroupMemberTypeGroup, child.ID)
	require.NoError(t, err)
	assert.Equal(t, store.GroupMemberTypeGroup, m.MemberType)
	assert.Equal(t, child.ID, m.MemberID)
}

func TestGetGroupMembershipNotFound(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Test",
		Slug: "test-gm-nf",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	_, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestWouldCreateCycleSelf(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Self",
		Slug: "self-cycle",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	wouldCycle, err := gs.WouldCreateCycle(ctx, g.ID, g.ID)
	require.NoError(t, err)
	assert.True(t, wouldCycle)
}

func TestWouldCreateCycleDirect(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	a := &store.Group{
		ID:   uuid.New().String(),
		Name: "A",
		Slug: "cycle-a",
	}
	b := &store.Group{
		ID:   uuid.New().String(),
		Name: "B",
		Slug: "cycle-b",
	}
	require.NoError(t, gs.CreateGroup(ctx, a))
	require.NoError(t, gs.CreateGroup(ctx, b))

	// A contains B
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    a.ID,
		MemberType: store.GroupMemberTypeGroup,
		MemberID:   b.ID,
		Role:       store.GroupMemberRoleMember,
	}))

	// Would B containing A create a cycle?
	wouldCycle, err := gs.WouldCreateCycle(ctx, b.ID, a.ID)
	require.NoError(t, err)
	assert.True(t, wouldCycle)
}

func TestWouldCreateCycleTransitive(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "trans-a"}
	b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "trans-b"}
	c := &store.Group{ID: uuid.New().String(), Name: "C", Slug: "trans-c"}
	require.NoError(t, gs.CreateGroup(ctx, a))
	require.NoError(t, gs.CreateGroup(ctx, b))
	require.NoError(t, gs.CreateGroup(ctx, c))

	// A contains B, B contains C
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID: a.ID, MemberType: store.GroupMemberTypeGroup, MemberID: b.ID, Role: store.GroupMemberRoleMember,
	}))
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID: b.ID, MemberType: store.GroupMemberTypeGroup, MemberID: c.ID, Role: store.GroupMemberRoleMember,
	}))

	// Would C containing A create a cycle?
	wouldCycle, err := gs.WouldCreateCycle(ctx, c.ID, a.ID)
	require.NoError(t, err)
	assert.True(t, wouldCycle)

	// A containing C should NOT create a cycle (C is already in A, but it's not circular)
	wouldCycle, err = gs.WouldCreateCycle(ctx, a.ID, c.ID)
	require.NoError(t, err)
	assert.False(t, wouldCycle) // C doesn't contain A anywhere
}

func TestWouldCreateCycleNoCycle(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "nc-a"}
	b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "nc-b"}
	require.NoError(t, gs.CreateGroup(ctx, a))
	require.NoError(t, gs.CreateGroup(ctx, b))

	// Neither contains the other
	wouldCycle, err := gs.WouldCreateCycle(ctx, a.ID, b.ID)
	require.NoError(t, err)
	assert.False(t, wouldCycle)
}

func TestGroveGroupGuardAddMember(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:        uuid.New().String(),
		Name:      "Grove Group",
		Slug:      "grove-guard",
		GroupType: store.GroupTypeGroveAgents,
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	member := &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}
	err := gs.AddGroupMember(ctx, member)
	assert.ErrorIs(t, err, store.ErrInvalidInput)
}

func TestGroveGroupGuardRemoveMember(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:        uuid.New().String(),
		Name:      "Grove Group",
		Slug:      "grove-guard-rm",
		GroupType: store.GroupTypeGroveAgents,
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
	assert.ErrorIs(t, err, store.ErrInvalidInput)
}

func TestGetEffectiveGroups(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	// Create a group hierarchy: A contains B, B contains C
	a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "eff-a"}
	b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "eff-b"}
	c := &store.Group{ID: uuid.New().String(), Name: "C", Slug: "eff-c"}
	require.NoError(t, gs.CreateGroup(ctx, a))
	require.NoError(t, gs.CreateGroup(ctx, b))
	require.NoError(t, gs.CreateGroup(ctx, c))

	// B is a child of A (A contains B)
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID: a.ID, MemberType: store.GroupMemberTypeGroup, MemberID: b.ID, Role: store.GroupMemberRoleMember,
	}))
	// C is a child of B (B contains C)
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID: b.ID, MemberType: store.GroupMemberTypeGroup, MemberID: c.ID, Role: store.GroupMemberRoleMember,
	}))

	// User is member of C
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID: c.ID, MemberType: store.GroupMemberTypeUser, MemberID: testUserUID.String(), Role: store.GroupMemberRoleMember,
	}))

	effective, err := gs.GetEffectiveGroups(ctx, testUserUID.String())
	require.NoError(t, err)

	// User should be in C, and also in B and A through transitive parent_groups expansion
	assert.Len(t, effective, 3)

	found := make(map[string]bool)
	for _, gid := range effective {
		found[gid] = true
	}
	assert.True(t, found[a.ID], "expected group A")
	assert.True(t, found[b.ID], "expected group B")
	assert.True(t, found[c.ID], "expected group C")
}

func TestGetEffectiveGroupsNoMemberships(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	effective, err := gs.GetEffectiveGroups(ctx, testUserUID.String())
	require.NoError(t, err)
	assert.Empty(t, effective)
}

func TestCompositeStoreDelegation(t *testing.T) {
	// Verify the CompositeStore properly delegates group operations
	client, err := entc.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, entc.AutoMigrate(context.Background(), client))

	// We use nil as the base store since we're only testing group methods
	// and they should all go to the Ent adapter.
	composite := NewCompositeStore(nil, client)

	ctx := context.Background()
	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Composite Test",
		Slug: "composite-test",
	}

	err = composite.CreateGroup(ctx, g)
	require.NoError(t, err)

	got, err := composite.GetGroup(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Composite Test", got.Name)

	got, err = composite.GetGroupBySlug(ctx, "composite-test")
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
}

func TestDeleteGroupCascadesMemberships(t *testing.T) {
	gs := newTestGroupStore(t)
	ctx := context.Background()

	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Group With Members",
		Slug: "group-cascade",
	}
	require.NoError(t, gs.CreateGroup(ctx, g))

	// Add a user member
	require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
		GroupID:    g.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   testUserUID.String(),
		Role:       store.GroupMemberRoleMember,
	}))

	// Delete group
	err := gs.DeleteGroup(ctx, g.ID)
	require.NoError(t, err)

	// Group should be gone
	_, err = gs.GetGroup(ctx, g.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

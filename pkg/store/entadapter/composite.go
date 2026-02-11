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

package entadapter

import (
	"context"

	"github.com/ptone/scion-agent/pkg/ent"
	"github.com/ptone/scion-agent/pkg/store"
)

// CompositeStore wraps an existing store.Store and overrides group operations
// with an Ent-backed GroupStore implementation.
type CompositeStore struct {
	store.Store
	groups *GroupStore
	client *ent.Client
}

// NewCompositeStore creates a CompositeStore that delegates group operations to
// the Ent-backed GroupStore while forwarding all other operations to the
// underlying store.
func NewCompositeStore(base store.Store, client *ent.Client) *CompositeStore {
	return &CompositeStore{
		Store:  base,
		groups: NewGroupStore(client),
		client: client,
	}
}

// Close closes both the Ent client and the underlying store.
func (c *CompositeStore) Close() error {
	if err := c.client.Close(); err != nil {
		_ = c.Store.Close()
		return err
	}
	return c.Store.Close()
}

// GroupStore method overrides — delegate to Ent-backed GroupStore.

func (c *CompositeStore) CreateGroup(ctx context.Context, group *store.Group) error {
	return c.groups.CreateGroup(ctx, group)
}

func (c *CompositeStore) GetGroup(ctx context.Context, id string) (*store.Group, error) {
	return c.groups.GetGroup(ctx, id)
}

func (c *CompositeStore) GetGroupBySlug(ctx context.Context, slug string) (*store.Group, error) {
	return c.groups.GetGroupBySlug(ctx, slug)
}

func (c *CompositeStore) UpdateGroup(ctx context.Context, group *store.Group) error {
	return c.groups.UpdateGroup(ctx, group)
}

func (c *CompositeStore) DeleteGroup(ctx context.Context, id string) error {
	return c.groups.DeleteGroup(ctx, id)
}

func (c *CompositeStore) ListGroups(ctx context.Context, filter store.GroupFilter, opts store.ListOptions) (*store.ListResult[store.Group], error) {
	return c.groups.ListGroups(ctx, filter, opts)
}

func (c *CompositeStore) AddGroupMember(ctx context.Context, member *store.GroupMember) error {
	return c.groups.AddGroupMember(ctx, member)
}

func (c *CompositeStore) RemoveGroupMember(ctx context.Context, groupID, memberType, memberID string) error {
	return c.groups.RemoveGroupMember(ctx, groupID, memberType, memberID)
}

func (c *CompositeStore) GetGroupMembers(ctx context.Context, groupID string) ([]store.GroupMember, error) {
	return c.groups.GetGroupMembers(ctx, groupID)
}

func (c *CompositeStore) GetUserGroups(ctx context.Context, userID string) ([]store.GroupMember, error) {
	return c.groups.GetUserGroups(ctx, userID)
}

func (c *CompositeStore) GetGroupMembership(ctx context.Context, groupID, memberType, memberID string) (*store.GroupMember, error) {
	return c.groups.GetGroupMembership(ctx, groupID, memberType, memberID)
}

func (c *CompositeStore) WouldCreateCycle(ctx context.Context, groupID, memberGroupID string) (bool, error) {
	return c.groups.WouldCreateCycle(ctx, groupID, memberGroupID)
}

func (c *CompositeStore) GetEffectiveGroups(ctx context.Context, userID string) ([]string, error) {
	return c.groups.GetEffectiveGroups(ctx, userID)
}

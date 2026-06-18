package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SyncService owns per-user workspace RPC behavior above the repository layer.
type SyncService struct {
	store *Store
}

// NewSyncService builds the server-side workspace sync service.
func NewSyncService(store *Store) *SyncService {
	return &SyncService{store: store}
}

// List returns workspaces strictly scoped to the authenticated user.
func (s *SyncService) List(ctx context.Context, includeDeleted bool) ([]Workspace, error) {
	user, err := syncUser(ctx)
	if err != nil {
		return nil, err
	}
	workspaces, err := s.store.ListWorkspaces(user.ID, includeDeleted)
	if err != nil {
		return nil, fmt.Errorf("server sync: list workspaces: %w", err)
	}
	return workspaces, nil
}

// Get returns one workspace strictly scoped to the authenticated user.
func (s *SyncService) Get(ctx context.Context, workspaceID string) (Workspace, bool, error) {
	user, err := syncUser(ctx)
	if err != nil {
		return Workspace{}, false, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return Workspace{}, false, status.Error(codes.InvalidArgument, "workspace id is required")
	}
	workspace, ok, err := s.store.GetWorkspace(user.ID, workspaceID)
	if err != nil {
		return Workspace{}, false, fmt.Errorf("server sync: get workspace: %w", err)
	}
	return workspace, ok, nil
}

// Delete writes a user-scoped workspace tombstone.
func (s *SyncService) Delete(ctx context.Context, workspaceID string, updatedAt time.Time, deviceID string) (bool, error) {
	user, err := syncUser(ctx)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return false, status.Error(codes.InvalidArgument, "workspace id is required")
	}
	deleted, err := s.store.SoftDeleteWorkspace(user.ID, workspaceID, updatedAt, deviceID)
	if err != nil {
		return false, fmt.Errorf("server sync: delete workspace: %w", err)
	}
	return deleted, nil
}

func syncUser(ctx context.Context) (AuthUser, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return AuthUser{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	return user, nil
}

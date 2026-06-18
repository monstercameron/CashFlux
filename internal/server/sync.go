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

// PutWorkspaceResult reports the server-side result of a LWW workspace put.
type PutWorkspaceResult struct {
	Accepted  bool
	Workspace Workspace
	Version   int64
	UpdatedAt time.Time
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

// PutWorkspace applies last-write-wins workspace updates scoped to the authenticated user.
func (s *SyncService) PutWorkspace(ctx context.Context, workspace Workspace, clientUpdatedAt time.Time, force bool, serverNow time.Time) (PutWorkspaceResult, error) {
	user, err := syncUser(ctx)
	if err != nil {
		return PutWorkspaceResult{}, err
	}
	if strings.TrimSpace(workspace.ID) == "" || strings.TrimSpace(workspace.Name) == "" {
		return PutWorkspaceResult{}, status.Error(codes.InvalidArgument, "workspace id and name are required")
	}
	owner, owned, err := s.store.WorkspaceOwner(workspace.ID)
	if err != nil {
		return PutWorkspaceResult{}, fmt.Errorf("server sync: workspace owner: %w", err)
	}
	if owned && owner != user.ID {
		return PutWorkspaceResult{}, status.Error(codes.NotFound, "workspace not found")
	}
	current, exists, err := s.store.GetWorkspace(user.ID, workspace.ID)
	if err != nil {
		return PutWorkspaceResult{}, fmt.Errorf("server sync: get current workspace: %w", err)
	}
	if exists && !force && clientUpdatedAt.Before(current.UpdatedAt) {
		return PutWorkspaceResult{
			Accepted:  false,
			Workspace: current,
			Version:   current.Version,
			UpdatedAt: current.UpdatedAt,
		}, nil
	}
	if serverNow.IsZero() {
		serverNow = time.Now().UTC()
	}
	workspace.UserID = user.ID
	workspace.UpdatedAt = serverNow.UTC()
	if exists {
		workspace.Version = current.Version + 1
	} else {
		workspace.Version = 1
	}
	if err := s.store.PutWorkspace(workspace); err != nil {
		return PutWorkspaceResult{}, fmt.Errorf("server sync: put workspace: %w", err)
	}
	return PutWorkspaceResult{
		Accepted:  true,
		Workspace: workspace,
		Version:   workspace.Version,
		UpdatedAt: workspace.UpdatedAt,
	}, nil
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

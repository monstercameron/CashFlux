// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxWorkspaceIDLength     = 128
	maxWorkspaceNameLength   = 160
	maxWorkspaceColorLength  = 64
	maxWorkspaceDeviceLength = 128
)

// SyncService owns per-user workspace RPC behavior above the repository layer.
type SyncService struct {
	store             *Store
	maxStreamsPerUser int
	metrics           *Metrics
	watchMu           sync.Mutex
	watches           map[string]map[chan backendrpc.WatchWorkspacesResponse]struct{}
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
	return &SyncService{store: store, watches: map[string]map[chan backendrpc.WatchWorkspacesResponse]struct{}{}}
}

func NewSyncServiceWithLimits(store *Store, maxStreamsPerUser int, metrics ...*Metrics) *SyncService {
	service := NewSyncService(store)
	service.maxStreamsPerUser = maxStreamsPerUser
	if len(metrics) > 0 {
		service.metrics = metrics[0]
	}
	return service
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
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return Workspace{}, false, status.Error(codes.InvalidArgument, "workspace id is required")
	}
	if len(workspaceID) > maxWorkspaceIDLength {
		return Workspace{}, false, status.Error(codes.InvalidArgument, "workspace id is too long")
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
	if err := validateWorkspaceFields(workspace); err != nil {
		return PutWorkspaceResult{}, err
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
	if err := s.ensureUser(user); err != nil {
		return PutWorkspaceResult{}, err
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
	if len(workspaceID) > maxWorkspaceIDLength || len(deviceID) > maxWorkspaceDeviceLength {
		return false, status.Error(codes.InvalidArgument, "workspace id or device id is too long")
	}
	deleted, err := s.store.SoftDeleteWorkspace(user.ID, workspaceID, updatedAt, deviceID)
	if err != nil {
		return false, fmt.Errorf("server sync: delete workspace: %w", err)
	}
	if deleted {
		if workspace, ok, err := s.store.GetWorkspace(user.ID, workspaceID); err == nil && ok {
			s.publishWorkspace(user.ID, workspace)
		}
	}
	return deleted, nil
}

func validateWorkspaceFields(workspace Workspace) error {
	switch {
	case len(workspace.ID) > maxWorkspaceIDLength:
		return status.Error(codes.InvalidArgument, "workspace id is too long")
	case len(workspace.Name) > maxWorkspaceNameLength:
		return status.Error(codes.InvalidArgument, "workspace name is too long")
	case len(workspace.Color) > maxWorkspaceColorLength:
		return status.Error(codes.InvalidArgument, "workspace color is too long")
	case len(workspace.DeviceID) > maxWorkspaceDeviceLength:
		return status.Error(codes.InvalidArgument, "workspace device id is too long")
	default:
		return nil
	}
}

func (s *SyncService) subscribeWorkspaces(userID string) (chan backendrpc.WatchWorkspacesResponse, func(), error) {
	ch := make(chan backendrpc.WatchWorkspacesResponse, 16)
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.watches == nil {
		s.watches = map[string]map[chan backendrpc.WatchWorkspacesResponse]struct{}{}
	}
	if s.watches[userID] == nil {
		s.watches[userID] = map[chan backendrpc.WatchWorkspacesResponse]struct{}{}
	}
	if s.maxStreamsPerUser > 0 && len(s.watches[userID]) >= s.maxStreamsPerUser {
		close(ch)
		return nil, nil, status.Error(codes.ResourceExhausted, "too many workspace streams")
	}
	s.watches[userID][ch] = struct{}{}
	if s.metrics != nil {
		s.metrics.IncActiveStream()
		s.updateWatchQueueDepthLocked()
	}
	return ch, func() {
		s.watchMu.Lock()
		if watchers := s.watches[userID]; watchers != nil {
			delete(watchers, ch)
			if len(watchers) == 0 {
				delete(s.watches, userID)
			}
		}
		s.watchMu.Unlock()
		if s.metrics != nil {
			s.metrics.DecActiveStream()
		}
		s.updateWatchQueueDepth()
		close(ch)
	}, nil
}

func (s *SyncService) publishWorkspace(userID string, workspace Workspace) {
	resp := backendrpc.WatchWorkspacesResponse{Workspace: rpcWorkspace(workspace)}
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	for ch := range s.watches[userID] {
		select {
		case ch <- resp:
		default:
		}
	}
	s.updateWatchQueueDepthLocked()
}

func (s *SyncService) updateWatchQueueDepth() {
	if s == nil || s.metrics == nil {
		return
	}
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	s.updateWatchQueueDepthLocked()
}

func (s *SyncService) updateWatchQueueDepthLocked() {
	if s == nil || s.metrics == nil {
		return
	}
	var depth int64
	for _, watchers := range s.watches {
		for ch := range watchers {
			depth += int64(len(ch))
		}
	}
	s.metrics.SetQueueDepth("workspace_watch", depth)
}

func syncUser(ctx context.Context) (AuthUser, error) {
	user, ok := AuthUserFromContext(ctx)
	if !ok || strings.TrimSpace(user.ID) == "" {
		return AuthUser{}, status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	return user, nil
}

func (s *SyncService) ensureUser(user AuthUser) error {
	if s == nil || s.store == nil {
		return status.Error(codes.FailedPrecondition, "sync service store is not configured")
	}
	var existing string
	err := s.store.db.QueryRow(`SELECT id FROM users WHERE id = ?`, user.ID).Scan(&existing)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("server sync: find user: %w", err)
	}
	if err := s.store.UpsertUser(User{ID: user.ID, Provider: "token", Subject: user.ID, CreatedAt: time.Now().UTC()}); err != nil {
		return fmt.Errorf("server sync: upsert user: %w", err)
	}
	return nil
}

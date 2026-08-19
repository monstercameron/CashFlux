// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/datasetguard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultSnapshotMaxBytes     = 16 << 20
	defaultSnapshotHistoryLimit = 10
)

func RegisterSyncServiceServer(s grpc.ServiceRegistrar, srv *SyncService) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.SyncService",
		HandlerType: (*syncServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "ListWorkspaces", Handler: syncListWorkspacesHandler},
			{MethodName: "GetWorkspace", Handler: syncGetWorkspaceHandler},
			{MethodName: "PutWorkspace", Handler: syncPutWorkspaceHandler},
			{MethodName: "DeleteWorkspace", Handler: syncDeleteWorkspaceHandler},
		},
		Streams: []grpc.StreamDesc{
			{StreamName: "WatchWorkspaces", Handler: syncWatchWorkspacesHandler, ServerStreams: true},
		},
		Metadata: "cashflux/v1/sync.proto",
	}, srv)
}

type syncServiceServer interface {
	ListWorkspacesRPC(context.Context, backendrpc.ListWorkspacesRequest) (backendrpc.ListWorkspacesResponse, error)
	GetWorkspaceRPC(context.Context, backendrpc.GetWorkspaceRequest) (backendrpc.GetWorkspaceResponse, error)
	PutWorkspaceRPC(context.Context, backendrpc.PutWorkspaceRequest) (backendrpc.PutWorkspaceResponse, error)
	DeleteWorkspaceRPC(context.Context, backendrpc.DeleteWorkspaceRequest) (backendrpc.DeleteWorkspaceResponse, error)
	WatchWorkspacesRPC(backendrpc.WatchWorkspacesRequest, grpc.ServerStream) error
}

func (s *SyncService) ListWorkspacesRPC(ctx context.Context, req backendrpc.ListWorkspacesRequest) (backendrpc.ListWorkspacesResponse, error) {
	workspaces, err := s.List(ctx, req.IncludeDeleted)
	if err != nil {
		return backendrpc.ListWorkspacesResponse{}, err
	}
	out := make([]backendrpc.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		out = append(out, rpcWorkspace(workspace))
	}
	s.metrics.ObserveSyncPull("list")
	return backendrpc.ListWorkspacesResponse{Workspaces: out}, nil
}

func (s *SyncService) GetWorkspaceRPC(ctx context.Context, req backendrpc.GetWorkspaceRequest) (backendrpc.GetWorkspaceResponse, error) {
	SetLogScope(ctx, LogScope{WorkspaceID: req.ID})
	workspace, found, err := s.Get(ctx, req.ID)
	if err != nil {
		return backendrpc.GetWorkspaceResponse{}, err
	}
	if !found {
		s.metrics.ObserveSyncPull("missing")
		return backendrpc.GetWorkspaceResponse{Found: false}, nil
	}
	etag := workspaceETag(workspace)
	resp := backendrpc.GetWorkspaceResponse{Found: true, ETag: etag, Workspace: rpcWorkspace(workspace)}
	if strings.TrimSpace(req.IfNoneMatch) == etag {
		resp.NotModified = true
		s.metrics.ObserveSyncPull("not_modified")
		return resp, nil
	}
	if found {
		user, _ := AuthUserFromContext(ctx)
		snapshot, ok, err := s.store.GetSnapshotForUser(user.ID, workspace.ID)
		if err != nil {
			return backendrpc.GetWorkspaceResponse{}, err
		}
		if ok {
			resp.Dataset = snapshot.Dataset
		}
	}
	s.metrics.ObserveSyncPull("found")
	return resp, nil
}

func (s *SyncService) PutWorkspaceRPC(ctx context.Context, req backendrpc.PutWorkspaceRequest) (backendrpc.PutWorkspaceResponse, error) {
	SetLogScope(ctx, LogScope{WorkspaceID: req.Workspace.ID, DeviceID: req.Workspace.DeviceID})
	clientUpdatedAt, err := parseOptionalRPCTime(req.ClientUpdatedAt)
	if err != nil {
		return backendrpc.PutWorkspaceResponse{}, err
	}
	// Before anything is written: would this dataset gut the copy already
	// stored? Last-write-wins orders concurrent edits; it does not decide whether
	// a replacement is sane, and on 2026-08-19 that gap let a stale dataset from
	// another account overwrite a household's records with a merely newer
	// timestamp. Checked server-side, so it holds whatever the client believes —
	// including an old client, or one with the bug still in it.
	//
	// Force is the escape hatch: a user who really is deleting most of their
	// records confirms it in the client and the same call comes back with
	// Force set. The guard exists to make that a decision rather than an
	// accident.
	if !req.Force && len(req.Dataset) > 0 {
		if err := s.guardDestructiveWrite(ctx, req.Workspace.ID, req.Dataset); err != nil {
			return backendrpc.PutWorkspaceResponse{}, err
		}
	}
	result, err := s.PutWorkspace(ctx, serverWorkspace(req.Workspace), clientUpdatedAt, req.Force, time.Now().UTC())
	if err != nil {
		return backendrpc.PutWorkspaceResponse{}, err
	}
	var dataset []byte
	if result.Accepted && len(req.Dataset) > 0 {
		if err := s.store.PutSnapshot(Snapshot{
			UserID:      result.Workspace.UserID,
			WorkspaceID: result.Workspace.ID,
			Dataset:     req.Dataset,
			Version:     result.Version,
			UpdatedAt:   result.UpdatedAt,
		}, defaultSnapshotMaxBytes, defaultSnapshotHistoryLimit); err != nil {
			if errors.Is(err, errPayloadTooLarge) {
				return backendrpc.PutWorkspaceResponse{}, status.Error(codes.ResourceExhausted, "sync dataset is too large")
			}
			return backendrpc.PutWorkspaceResponse{}, err
		}
		dataset = req.Dataset
	} else if !result.Accepted {
		user, _ := AuthUserFromContext(ctx)
		snapshot, ok, err := s.store.GetSnapshotForUser(user.ID, result.Workspace.ID)
		if err != nil {
			return backendrpc.PutWorkspaceResponse{}, err
		}
		if ok {
			dataset = snapshot.Dataset
		}
		s.metrics.ObserveSyncPush("lww_rejected")
		s.metrics.ObserveSyncLWWReject()
	}
	if result.Accepted {
		if user, ok := AuthUserFromContext(ctx); ok {
			s.publishWorkspace(user.ID, result.Workspace)
		}
		auditFromContext(ctx, s.store, "workspace.put", "workspace", result.Workspace.ID)
		s.metrics.ObserveSyncPush("accepted")
	}
	return backendrpc.PutWorkspaceResponse{
		Accepted:  result.Accepted,
		Workspace: rpcWorkspace(result.Workspace),
		Dataset:   dataset,
		Version:   result.Version,
		UpdatedAt: formatRPCTime(result.UpdatedAt),
	}, nil
}

func (s *SyncService) DeleteWorkspaceRPC(ctx context.Context, req backendrpc.DeleteWorkspaceRequest) (backendrpc.DeleteWorkspaceResponse, error) {
	SetLogScope(ctx, LogScope{WorkspaceID: req.ID, DeviceID: req.DeviceID})
	updatedAt, err := parseOptionalRPCTime(req.UpdatedAt)
	if err != nil {
		return backendrpc.DeleteWorkspaceResponse{}, err
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	deleted, err := s.Delete(ctx, req.ID, updatedAt, req.DeviceID)
	if err != nil {
		return backendrpc.DeleteWorkspaceResponse{}, err
	}
	if deleted {
		auditFromContext(ctx, s.store, "workspace.delete", "workspace", req.ID)
		s.metrics.ObserveSyncPush("deleted")
	} else {
		s.metrics.ObserveSyncPush("delete_missing")
	}
	return backendrpc.DeleteWorkspaceResponse{Deleted: deleted}, nil
}

func (s *SyncService) WatchWorkspacesRPC(req backendrpc.WatchWorkspacesRequest, stream grpc.ServerStream) error {
	start := time.Now()
	statusCode := codes.OK.String()
	defer func() {
		if s.metrics != nil {
			s.metrics.ObserveStreamDuration(backendrpc.MethodSyncWatchWorkspaces, statusCode, time.Since(start))
		}
	}()
	user, err := syncUser(stream.Context())
	if err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	ch, unsubscribe, err := s.subscribeWorkspaces(user.ID)
	if err != nil {
		statusCode = status.Code(err).String()
		return err
	}
	defer unsubscribe()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case resp, ok := <-ch:
			if !ok {
				return nil
			}
			if resp.Workspace.Deleted && !req.IncludeDeleted {
				continue
			}
			if err := stream.SendMsg(&resp); err != nil {
				statusCode = status.Code(err).String()
				return err
			}
			// No per-delivery depth refresh: the gauge is maintained by the
			// (throttled) publish path and by subscribe/unsubscribe. Walking
			// the full watch registry here cost O(all watchers) per DELIVERED
			// event — the same hot-path scan the publish throttle removed.
		}
	}
}

func rpcWorkspace(workspace Workspace) backendrpc.Workspace {
	return backendrpc.Workspace{
		ID:        workspace.ID,
		Name:      workspace.Name,
		Color:     workspace.Color,
		Sort:      workspace.Sort,
		Deleted:   workspace.Deleted,
		Version:   workspace.Version,
		UpdatedAt: formatRPCTime(workspace.UpdatedAt),
		DeviceID:  workspace.DeviceID,
	}
}

func serverWorkspace(workspace backendrpc.Workspace) Workspace {
	updatedAt, _ := parseOptionalRPCTime(workspace.UpdatedAt)
	return Workspace{
		ID:        workspace.ID,
		Name:      workspace.Name,
		Color:     workspace.Color,
		Sort:      workspace.Sort,
		Deleted:   workspace.Deleted,
		Version:   workspace.Version,
		UpdatedAt: updatedAt,
		DeviceID:  workspace.DeviceID,
	}
}

func workspaceETag(workspace Workspace) string {
	return fmt.Sprintf(`"%s-%d"`, workspace.ID, workspace.Version)
}

func parseOptionalRPCTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, status.Error(codes.InvalidArgument, "invalid timestamp")
	}
	return t.UTC(), nil
}

func formatRPCTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func syncListWorkspacesHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.ListWorkspacesRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(syncServiceServer).ListWorkspacesRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodSyncListWorkspaces}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(syncServiceServer).ListWorkspacesRPC(ctx, req.(backendrpc.ListWorkspacesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func syncGetWorkspaceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.GetWorkspaceRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(syncServiceServer).GetWorkspaceRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodSyncGetWorkspace}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(syncServiceServer).GetWorkspaceRPC(ctx, req.(backendrpc.GetWorkspaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func syncPutWorkspaceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.PutWorkspaceRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(syncServiceServer).PutWorkspaceRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodSyncPutWorkspace}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(syncServiceServer).PutWorkspaceRPC(ctx, req.(backendrpc.PutWorkspaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func syncDeleteWorkspaceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var in backendrpc.DeleteWorkspaceRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(syncServiceServer).DeleteWorkspaceRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: backendrpc.MethodSyncDeleteWorkspace}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(syncServiceServer).DeleteWorkspaceRPC(ctx, req.(backendrpc.DeleteWorkspaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func syncWatchWorkspacesHandler(srv any, stream grpc.ServerStream) error {
	var in backendrpc.WatchWorkspacesRequest
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return srv.(syncServiceServer).WatchWorkspacesRPC(in, stream)
}

// guardDestructiveWrite refuses a snapshot that would destroy most of what is
// already stored for this workspace.
//
// It answers a question last-write-wins cannot: not "which of these is newer"
// but "is this a replacement anyone meant". A client pushing a dataset it
// inherited from another account looks, to a timestamp comparison, exactly like
// a client pushing legitimate new work.
//
// It is deliberately narrow:
//
//   - Only an EXISTING stored snapshot can be protected; a first write has
//     nothing to lose and is always allowed.
//   - Either side being unreadable (an App Lock envelope, which the server
//     cannot decrypt by design) means no verdict is possible, and the write
//     proceeds. Refusing what it cannot read would break encrypted sync
//     entirely, which is a worse failure than the one being guarded.
//   - Only losses past the policy threshold are refused, and Force overrides.
func (s *SyncService) guardDestructiveWrite(ctx context.Context, workspaceID string, incoming []byte) error {
	user, err := syncUser(ctx)
	if err != nil {
		return err
	}
	stored, ok, err := s.store.GetSnapshotForUser(user.ID, strings.TrimSpace(workspaceID))
	if err != nil {
		return fmt.Errorf("server sync: read stored snapshot: %w", err)
	}
	if !ok || len(stored.Dataset) == 0 {
		return nil // nothing stored yet — no loss is possible
	}
	prev, prevOK := datasetguard.Inspect(stored.Dataset)
	next, nextOK := datasetguard.Inspect(incoming)
	if !prevOK || !nextOK {
		return nil // encrypted or unparseable: no honest verdict available
	}
	verdict := datasetguard.Check(prev, next, datasetguard.DefaultPolicy)
	if !verdict.Destructive {
		return nil
	}
	if s.metrics != nil {
		s.metrics.ObserveDestructiveWriteBlocked()
	}
	// FailedPrecondition, not InvalidArgument: the request is well-formed and the
	// caller may legitimately retry it with Force once a person has confirmed.
	return status.Errorf(codes.FailedPrecondition,
		"refusing a destructive sync: %s. Confirm in the app to replace it anyway.", verdict.Reason)
}

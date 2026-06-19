package server

import (
	"context"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		Streams:  []grpc.StreamDesc{},
		Metadata: "cashflux/v1/sync.proto",
	}, srv)
}

type syncServiceServer interface {
	ListWorkspacesRPC(context.Context, backendrpc.ListWorkspacesRequest) (backendrpc.ListWorkspacesResponse, error)
	GetWorkspaceRPC(context.Context, backendrpc.GetWorkspaceRequest) (backendrpc.GetWorkspaceResponse, error)
	PutWorkspaceRPC(context.Context, backendrpc.PutWorkspaceRequest) (backendrpc.PutWorkspaceResponse, error)
	DeleteWorkspaceRPC(context.Context, backendrpc.DeleteWorkspaceRequest) (backendrpc.DeleteWorkspaceResponse, error)
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
	return backendrpc.ListWorkspacesResponse{Workspaces: out}, nil
}

func (s *SyncService) GetWorkspaceRPC(ctx context.Context, req backendrpc.GetWorkspaceRequest) (backendrpc.GetWorkspaceResponse, error) {
	workspace, found, err := s.Get(ctx, req.ID)
	if err != nil {
		return backendrpc.GetWorkspaceResponse{}, err
	}
	return backendrpc.GetWorkspaceResponse{Found: found, Workspace: rpcWorkspace(workspace)}, nil
}

func (s *SyncService) PutWorkspaceRPC(ctx context.Context, req backendrpc.PutWorkspaceRequest) (backendrpc.PutWorkspaceResponse, error) {
	clientUpdatedAt, err := parseOptionalRPCTime(req.ClientUpdatedAt)
	if err != nil {
		return backendrpc.PutWorkspaceResponse{}, err
	}
	result, err := s.PutWorkspace(ctx, serverWorkspace(req.Workspace), clientUpdatedAt, req.Force, time.Now().UTC())
	if err != nil {
		return backendrpc.PutWorkspaceResponse{}, err
	}
	return backendrpc.PutWorkspaceResponse{
		Accepted:  result.Accepted,
		Workspace: rpcWorkspace(result.Workspace),
		Version:   result.Version,
		UpdatedAt: formatRPCTime(result.UpdatedAt),
	}, nil
}

func (s *SyncService) DeleteWorkspaceRPC(ctx context.Context, req backendrpc.DeleteWorkspaceRequest) (backendrpc.DeleteWorkspaceResponse, error) {
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
	return backendrpc.DeleteWorkspaceResponse{Deleted: deleted}, nil
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

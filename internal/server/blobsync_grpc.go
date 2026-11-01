// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"context"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type blobSyncServer interface {
	AppendChanges(context.Context, backendrpc.AppendChangesRequest) (backendrpc.AppendChangesResponse, error)
	ListChanges(context.Context, backendrpc.ListChangesRequest) (backendrpc.ListChangesResponse, error)
	PutSnapshot(context.Context, backendrpc.PutSnapshotRequest) (backendrpc.PutSnapshotResponse, error)
	GetSnapshot(context.Context, backendrpc.GetSnapshotRequest) (backendrpc.GetSnapshotResponse, error)
	GetKeyring(context.Context, backendrpc.GetKeyringRequest) (backendrpc.GetKeyringResponse, error)
	PutKeyring(context.Context, backendrpc.PutKeyringRequest) (backendrpc.PutKeyringResponse, error)
	DeleteWorkspace(context.Context, backendrpc.BlobSyncDeleteWorkspaceRequest) (backendrpc.BlobSyncDeleteWorkspaceResponse, error)
	CreatePairing(context.Context, backendrpc.CreatePairingRequest) (backendrpc.CreatePairingResponse, error)
	ClaimPairing(context.Context, backendrpc.ClaimPairingRequest) (backendrpc.ClaimPairingResponse, error)
	Watch(backendrpc.WatchRequest, grpc.ServerStream) error
}

// RegisterBlobSyncServiceServer installs the hand-written v2 service descriptor.
func RegisterBlobSyncServiceServer(reg grpc.ServiceRegistrar, srv *BlobSyncService) {
	reg.RegisterService(&grpc.ServiceDesc{ServiceName: "cashflux.v2.BlobSync", HandlerType: (*blobSyncServer)(nil), Methods: []grpc.MethodDesc{
		{MethodName: "AppendChanges", Handler: blobSyncAppendHandler}, {MethodName: "ListChanges", Handler: blobSyncListHandler}, {MethodName: "PutSnapshot", Handler: blobSyncPutSnapshotHandler}, {MethodName: "GetSnapshot", Handler: blobSyncGetSnapshotHandler}, {MethodName: "GetKeyring", Handler: blobSyncGetKeyringHandler}, {MethodName: "PutKeyring", Handler: blobSyncPutKeyringHandler}, {MethodName: "DeleteWorkspace", Handler: blobSyncDeleteHandler}, {MethodName: "CreatePairing", Handler: blobSyncCreatePairHandler}, {MethodName: "ClaimPairing", Handler: blobSyncClaimPairHandler}}, Streams: []grpc.StreamDesc{{StreamName: "Watch", Handler: blobSyncWatchHandler, ServerStreams: true}}, Metadata: "cashflux.proto"}, srv)
}

func blobSyncUnary[T any, R any](srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor, method string, call func(context.Context, T) (R, error)) (any, error) {
	var req T
	if err := dec(&req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return call(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: method}
	handler := func(ctx context.Context, in any) (any, error) { return call(ctx, in.(T)) }
	return interceptor(ctx, req, info, handler)
}
func blobSyncAppendHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.AppendChangesRequest, backendrpc.AppendChangesResponse](s, c, d, i, backendrpc.MethodBlobSyncAppendChanges, s.(blobSyncServer).AppendChanges)
}
func blobSyncListHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.ListChangesRequest, backendrpc.ListChangesResponse](s, c, d, i, backendrpc.MethodBlobSyncListChanges, s.(blobSyncServer).ListChanges)
}
func blobSyncPutSnapshotHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.PutSnapshotRequest, backendrpc.PutSnapshotResponse](s, c, d, i, backendrpc.MethodBlobSyncPutSnapshot, s.(blobSyncServer).PutSnapshot)
}
func blobSyncGetSnapshotHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.GetSnapshotRequest, backendrpc.GetSnapshotResponse](s, c, d, i, backendrpc.MethodBlobSyncGetSnapshot, s.(blobSyncServer).GetSnapshot)
}
func blobSyncGetKeyringHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.GetKeyringRequest, backendrpc.GetKeyringResponse](s, c, d, i, backendrpc.MethodBlobSyncGetKeyring, s.(blobSyncServer).GetKeyring)
}
func blobSyncPutKeyringHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.PutKeyringRequest, backendrpc.PutKeyringResponse](s, c, d, i, backendrpc.MethodBlobSyncPutKeyring, s.(blobSyncServer).PutKeyring)
}
func blobSyncDeleteHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.BlobSyncDeleteWorkspaceRequest, backendrpc.BlobSyncDeleteWorkspaceResponse](s, c, d, i, backendrpc.MethodBlobSyncDelete, s.(blobSyncServer).DeleteWorkspace)
}
func blobSyncCreatePairHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.CreatePairingRequest, backendrpc.CreatePairingResponse](s, c, d, i, backendrpc.MethodBlobSyncCreatePair, s.(blobSyncServer).CreatePairing)
}
func blobSyncClaimPairHandler(s any, c context.Context, d func(any) error, i grpc.UnaryServerInterceptor) (any, error) {
	return blobSyncUnary[backendrpc.ClaimPairingRequest, backendrpc.ClaimPairingResponse](s, c, d, i, backendrpc.MethodBlobSyncClaimPair, s.(blobSyncServer).ClaimPairing)
}

func blobSyncWatchHandler(s any, stream grpc.ServerStream) error {
	var req backendrpc.WatchRequest
	if err := stream.RecvMsg(&req); err != nil {
		return err
	}
	return s.(blobSyncServer).Watch(req, stream)
}

// Watch emits the current head and then polls for changes until the client disconnects.
func (s *BlobSyncService) Watch(req backendrpc.WatchRequest, stream grpc.ServerStream) error {
	user, err := s.caller(stream.Context())
	if err != nil {
		return err
	}
	if req.Workspace == "" {
		return status.Error(codes.InvalidArgument, "workspace is required")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	cursor := req.Since
	for {
		_, head, err := s.store.listBlobChanges(stream.Context(), user.ID, req.Workspace, cursor, 1)
		if err != nil {
			return mapBlobSyncError(err)
		}
		if head > cursor {
			if err := stream.SendMsg(&backendrpc.WatchEvent{Workspace: req.Workspace, Head: head}); err != nil {
				return err
			}
			cursor = head
		}
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

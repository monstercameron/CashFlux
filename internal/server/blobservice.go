// SPDX-License-Identifier: MIT

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterBlobServiceServer registers the hand-rolled BlobService ServiceDesc
// against s, following the same JSON-codec pattern as
// SyncService/AIService/AuthService. UploadBlob is client-streaming
// (ClientStreams: true) and DownloadBlob is server-streaming
// (ServerStreams: true), matching the proto shapes.
func RegisterBlobServiceServer(s grpc.ServiceRegistrar, srv BlobServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "cashflux.v1.BlobService",
		HandlerType: (*BlobServiceServer)(nil),
		Streams: []grpc.StreamDesc{
			{StreamName: "UploadBlob", Handler: blobUploadBlobHandler, ClientStreams: true},
			{StreamName: "DownloadBlob", Handler: blobDownloadBlobHandler, ServerStreams: true},
		},
		Metadata: "cashflux/v1/cashflux.proto",
	}, srv)
}

// BlobServiceServer is the server-side contract for BlobService (TODOS.md
// C426 — moves artifact/blob transfer off the REST /v1/blobs/{hash} PUT/GET
// endpoints onto this authenticated gRPC-streaming tunnel).
type BlobServiceServer interface {
	UploadBlob(grpc.ServerStream) error
	DownloadBlob(backendrpc.DownloadBlobRequest, grpc.ServerStream) error
}

// blobDownloadChunkBytes is the size of each DownloadBlob server-stream
// message. Small enough to keep memory bounded on both ends, large enough
// to keep the per-message overhead of the JSON codec low.
const blobDownloadChunkBytes = 64 << 10

// blobServer implements BlobServiceServer. It ports the existing
// withinStorageQuota check (blob_http.go) — dedup via Store.UserBlobLinked,
// running total via Store.UserBlobBytes against Config.StorageMaxBytes/
// StorageWarnBytes — onto this streaming shape: a soft pre-check against the
// declared size in the first UploadBlobChunk, and a hard check against actual
// bytes received at commit (TODOS.md C434). blob_http.go itself is left
// untouched — it is retired once BlobService is proven.
type blobServer struct {
	store *Store
	cfg   Config
}

func newBlobService(store *Store, cfg Config) *blobServer {
	return &blobServer{store: store, cfg: cfg}
}

// UploadBlob receives a content-addressed blob as a client stream: a header
// chunk (hash + declared size) followed by data chunks. It streams the
// incoming bytes to a temp file under <blobRoot>/partials so an interrupted
// upload leaves a reapable partial artifact (blobcleanup.go) rather than
// silent in-memory loss, hash-verifies the assembled bytes against the
// declared hash, and only then commits them via Store.PutBlobContext —
// rolling back (removing the temp file, never persisting) if the actual size
// blows the hard quota cap even though the declared-size soft pre-check
// passed.
func (s *blobServer) UploadBlob(stream grpc.ServerStream) error {
	if s == nil || s.store == nil {
		return status.Error(codes.FailedPrecondition, "store is not configured")
	}
	user, ok := AuthUserFromContext(stream.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		return status.Error(codes.Unauthenticated, "authenticated user is required")
	}

	var first backendrpc.UploadBlobChunk
	if err := stream.RecvMsg(&first); err != nil {
		if err == io.EOF {
			return status.Error(codes.InvalidArgument, "upload stream ended before a header was sent")
		}
		return status.Error(codes.Internal, "upload stream read failed")
	}
	if first.Header == nil {
		return status.Error(codes.InvalidArgument, "first upload message must carry a header")
	}
	hash := strings.ToLower(strings.TrimSpace(first.Header.Hash))
	if !validBlobHash(hash) {
		return status.Error(codes.InvalidArgument, "invalid blob hash")
	}
	// WorkspaceID identifies the caller's workspace this blob is being
	// attached to (see backendrpc.UploadBlobHeader's doc comment) — verified
	// here, before any bytes are received, exactly like blob_http.go's
	// handlePutBlob/authorizedBlobWorkspace does for the REST PUT route.
	// Without this check the blob is never attributed to anyone (storage
	// quota bypass) and, on the download side, never scoped to a workspace
	// (cross-tenant read of any blob on the server by hash).
	workspaceID := strings.TrimSpace(first.Header.WorkspaceID)
	if workspaceID == "" {
		return status.Error(codes.InvalidArgument, "workspace id is required")
	}
	if _, ok, err := s.store.GetWorkspace(user.ID, workspaceID); err != nil {
		return status.Error(codes.Internal, "workspace lookup failed")
	} else if !ok {
		return status.Error(codes.NotFound, "workspace not found")
	}
	declaredSize := first.Header.DeclaredSizeBytes
	if declaredSize < 0 {
		return status.Error(codes.InvalidArgument, "declared size must not be negative")
	}
	if s.cfg.BlobMaxBytes > 0 && declaredSize > s.cfg.BlobMaxBytes {
		return status.Errorf(codes.ResourceExhausted, "blob is %d bytes, exceeds limit %d", declaredSize, s.cfg.BlobMaxBytes)
	}
	// Soft pre-check (C434): reject before receiving any bytes when the
	// DECLARED size alone would already blow the account's storage quota.
	within, err := blobWithinStorageQuota(s.store, user.ID, hash, declaredSize, s.cfg)
	if err != nil {
		return status.Error(codes.Internal, "storage quota check failed")
	}
	if !within {
		return status.Error(codes.ResourceExhausted, "storage quota exceeded")
	}

	root := blobRoot(s.cfg)
	partialPath, cleanup, err := createBlobPartial(root)
	if err != nil {
		return status.Error(codes.Internal, "blob staging failed")
	}
	defer cleanup()

	hasher := sha256.New()
	var total int64
	writeErr := func() error {
		f, err := os.OpenFile(partialPath, os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		for {
			var chunk backendrpc.UploadBlobChunk
			err := stream.RecvMsg(&chunk)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if len(chunk.Data) == 0 {
				continue
			}
			total += int64(len(chunk.Data))
			if s.cfg.BlobMaxBytes > 0 && total > s.cfg.BlobMaxBytes {
				return fmt.Errorf("blob exceeds limit %d bytes", s.cfg.BlobMaxBytes)
			}
			if _, err := f.Write(chunk.Data); err != nil {
				return err
			}
			hasher.Write(chunk.Data)
		}
	}()
	if writeErr != nil {
		return status.Error(codes.ResourceExhausted, "blob upload exceeded the size limit")
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != hash {
		return status.Error(codes.InvalidArgument, "blob hash mismatch")
	}

	// Hard quota check (C434) against the ACTUAL bytes received — the soft
	// pre-check only ever saw the caller's declared size, which the caller
	// could understate.
	within, err = blobWithinStorageQuota(s.store, user.ID, hash, total, s.cfg)
	if err != nil {
		return status.Error(codes.Internal, "storage quota check failed")
	}
	if !within {
		// Rollback: the temp file is removed by the deferred cleanup and
		// nothing is ever persisted via PutBlobContext.
		return status.Error(codes.ResourceExhausted, "storage quota exceeded")
	}

	data, err := os.ReadFile(partialPath)
	if err != nil {
		return status.Error(codes.Internal, "blob staging read failed")
	}
	mime := safeBlobMIMEGRPC(strings.TrimSpace(first.Header.Mime), data)
	blob, err := s.store.PutBlobContext(stream.Context(), root, data, mime, strings.TrimSpace(first.Header.Name), s.cfg.BlobMaxBytes)
	if err != nil {
		return status.Error(codes.Internal, "blob write failed")
	}
	if err := s.store.LinkWorkspaceBlob(workspaceID, blob.Hash); err != nil {
		return status.Error(codes.Internal, "blob link failed")
	}
	if s.cfg.Metrics != nil {
		s.cfg.Metrics.ObserveBlobStored(blob.Size)
	}
	return stream.SendMsg(&backendrpc.UploadBlobResponse{Hash: blob.Hash, Size: blob.Size})
}

// DownloadBlob streams a content-addressed blob's bytes back to the caller in
// fixed-size chunks.
func (s *blobServer) DownloadBlob(req backendrpc.DownloadBlobRequest, stream grpc.ServerStream) error {
	if s == nil || s.store == nil {
		return status.Error(codes.FailedPrecondition, "store is not configured")
	}
	user, ok := AuthUserFromContext(stream.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		return status.Error(codes.Unauthenticated, "authenticated user is required")
	}
	hash := strings.ToLower(strings.TrimSpace(req.Hash))
	if !validBlobHash(hash) {
		return status.Error(codes.InvalidArgument, "invalid blob hash")
	}
	// Enforce the same per-user tenant isolation as the REST GET
	// /v1/blobs/{hash}?workspaceId=... route (blob_http.go's
	// authorizedBlobWorkspace/UserWorkspaceBlob, requireLink=true): blobs are
	// stored content-addressed and globally deduplicated, so a bare
	// GetBlob(hash) existence check alone would let ANY authenticated caller
	// download ANY blob on the server just by knowing its hash. WorkspaceID
	// must belong to the caller AND be linked to hash before bytes are served.
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return status.Error(codes.InvalidArgument, "workspace id is required")
	}
	if _, ok, err := s.store.GetWorkspace(user.ID, workspaceID); err != nil {
		return status.Error(codes.Internal, "workspace lookup failed")
	} else if !ok {
		return status.Error(codes.NotFound, "workspace not found")
	}
	if linked, err := s.store.UserWorkspaceBlob(user.ID, workspaceID, hash); err != nil {
		return status.Error(codes.Internal, "blob link lookup failed")
	} else if !linked {
		return status.Error(codes.NotFound, "blob not found")
	}
	if _, ok, err := s.store.GetBlob(hash); err != nil {
		return status.Error(codes.Internal, "blob lookup failed")
	} else if !ok {
		return status.Error(codes.NotFound, "blob not found")
	}
	data, err := s.store.ReadBlobContext(stream.Context(), blobRoot(s.cfg), hash)
	if err != nil {
		return status.Error(codes.Internal, "blob read failed")
	}
	if s.cfg.Metrics != nil {
		s.cfg.Metrics.ObserveBlobTransferred(int64(len(data)))
	}
	for offset := 0; offset < len(data); offset += blobDownloadChunkBytes {
		end := offset + blobDownloadChunkBytes
		if end > len(data) {
			end = len(data)
		}
		if err := stream.SendMsg(&backendrpc.DownloadBlobChunk{Data: data[offset:end]}); err != nil {
			return status.Error(codes.Internal, "blob stream send failed")
		}
	}
	if len(data) == 0 {
		// Send a single empty chunk so the client's stream sees at least one
		// message for a legitimately empty blob.
		if err := stream.SendMsg(&backendrpc.DownloadBlobChunk{Data: nil}); err != nil {
			return status.Error(codes.Internal, "blob stream send failed")
		}
	}
	return nil
}

// blobWithinStorageQuota mirrors blob_http.go's withinStorageQuota exactly
// (same fields, same Store calls) but returns a (bool, error) instead of
// writing an HTTP response, since this transport is gRPC. It is the single
// source of truth both handlers (HTTP PUT and this streaming RPC) consult —
// deliberately not forked business logic, just a different result shape.
func blobWithinStorageQuota(store *Store, userID, hash string, size int64, cfg Config) (bool, error) {
	limit := storageLimitForUser(store, userID, cfg)
	if limit <= 0 && cfg.StorageWarnBytes <= 0 {
		return true, nil
	}
	linked, err := store.UserBlobLinked(userID, hash)
	if err != nil {
		return false, err
	}
	if linked {
		return true, nil
	}
	current, err := store.UserBlobBytes(userID)
	if err != nil {
		return false, err
	}
	next := current + size
	if limit > 0 && next > limit {
		return false, nil
	}
	return true, nil
}

// safeBlobMIMEGRPC is the streaming-transport equivalent of blob_http.go's
// safeBlobMIME: it applies the identical forbidden-MIME rules but has no
// http.ResponseWriter to report a rejection through, so a declared or
// sniffed dangerous type (text/html, image/svg+xml, …) is silently
// downgraded to application/octet-stream rather than rejecting the upload —
// the same effect (the blob can never be served back as active content).
func safeBlobMIMEGRPC(declared string, data []byte) string {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	if declared != "" && !forbiddenBlobMIME(declared) && declared != "application/octet-stream" {
		return declared
	}
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if sniffed == "" || forbiddenBlobMIME(sniffed) {
		return "application/octet-stream"
	}
	return sniffed
}

// createBlobPartial makes a fresh, empty temp file under
// <root>/partials to stage an in-flight upload's bytes, and returns a cleanup
// func that removes it. Callers that successfully commit the upload should
// still call cleanup (it is a no-op once the file no longer exists) —
// blobcleanup.go independently reaps any partials cleanup never reached
// (crashed process, killed connection) once they age past its threshold.
func createBlobPartial(root string) (path string, cleanup func(), err error) {
	dir := filepath.Join(root, "partials")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, fmt.Errorf("server store: blob partial mkdir: %w", err)
	}
	f, err := os.CreateTemp(dir, "*.partial")
	if err != nil {
		return "", nil, fmt.Errorf("server store: blob partial create: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	return name, func() { _ = os.Remove(name) }, nil
}

func blobUploadBlobHandler(srv any, stream grpc.ServerStream) error {
	return srv.(BlobServiceServer).UploadBlob(stream)
}

func blobDownloadBlobHandler(srv any, stream grpc.ServerStream) error {
	var in backendrpc.DownloadBlobRequest
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return srv.(BlobServiceServer).DownloadBlob(in, stream)
}

// blobPartialCutoff is exported for blobcleanup.go's default threshold.
const blobPartialCutoff = time.Hour

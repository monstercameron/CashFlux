// SPDX-License-Identifier: MIT

package backendrpc

// Method name constants for BlobService — artifact transfer moved off REST
// onto gRPC streaming (TODOS.md C426). These must stay in lockstep with the
// generated backendrpcpb.BlobService_*_FullMethodName constants;
// proto_contract_test.go asserts the correspondence.
const (
	MethodBlobUploadBlob   = "/cashflux.v1.BlobService/UploadBlob"
	MethodBlobDownloadBlob = "/cashflux.v1.BlobService/DownloadBlob"
)

// UploadBlobHeader is the first message on an UploadBlob client stream: it
// declares the content-addressed hash the caller believes the bytes hash to
// and the size it intends to send, so the server can soft pre-check quota
// before receiving any bytes (TODOS.md C434). WorkspaceID identifies the
// caller's workspace the blob is being attached to — the server verifies the
// caller owns it and links the committed blob to it, exactly like the REST
// PUT /v1/blobs/{hash}?workspaceId=... route did (blob_http.go's
// handlePutBlob/LinkWorkspaceBlob). Without this the blob is never attributed
// to the uploader for storage-quota accounting or ownership checks.
type UploadBlobHeader struct {
	Hash              string `json:"hash"`
	DeclaredSizeBytes int64  `json:"declaredSizeBytes"`
	Mime              string `json:"mime,omitempty"`
	Name              string `json:"name,omitempty"`
	WorkspaceID       string `json:"workspaceId"`
}

// UploadBlobChunk is one message on an UploadBlob client stream. Exactly one
// of Header (first message only) or Data (every subsequent message) is set —
// the hand-written JSON codec's equivalent of the proto `oneof payload`.
type UploadBlobChunk struct {
	Header *UploadBlobHeader `json:"header,omitempty"`
	Data   []byte            `json:"data,omitempty"`
}

// UploadBlobResponse is returned once the client half-closes the UploadBlob stream.
type UploadBlobResponse struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// DownloadBlobRequest asks the server to stream back a content-addressed
// blob. WorkspaceID is the caller's workspace the blob must be linked to —
// the server enforces the same per-user tenant isolation as the REST
// GET /v1/blobs/{hash}?workspaceId=... route (blob_http.go's
// authorizedBlobWorkspace/UserWorkspaceBlob): a blob not linked to this
// caller's workspace is reported NotFound, not served.
type DownloadBlobRequest struct {
	Hash        string `json:"hash"`
	WorkspaceID string `json:"workspaceId"`
}

// DownloadBlobChunk is one message on a DownloadBlob server stream.
type DownloadBlobChunk struct {
	Data []byte `json:"data"`
}

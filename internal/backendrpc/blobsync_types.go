// SPDX-License-Identifier: MIT

package backendrpc

// BlobSync method names identify the v2 service methods on the JSON gRPC wire.
const (
	MethodBlobSyncAppendChanges = "/cashflux.v2.BlobSync/AppendChanges"
	MethodBlobSyncListChanges   = "/cashflux.v2.BlobSync/ListChanges"
	MethodBlobSyncPutSnapshot   = "/cashflux.v2.BlobSync/PutSnapshot"
	MethodBlobSyncGetSnapshot   = "/cashflux.v2.BlobSync/GetSnapshot"
	MethodBlobSyncGetKeyring    = "/cashflux.v2.BlobSync/GetKeyring"
	MethodBlobSyncPutKeyring    = "/cashflux.v2.BlobSync/PutKeyring"
	MethodBlobSyncDelete        = "/cashflux.v2.BlobSync/DeleteWorkspace"
	MethodBlobSyncCreatePair    = "/cashflux.v2.BlobSync/CreatePairing"
	MethodBlobSyncClaimPair     = "/cashflux.v2.BlobSync/ClaimPairing"
	MethodBlobSyncWatch         = "/cashflux.v2.BlobSync/Watch"
)

// BlobChange carries one opaque encrypted change and its server-assigned sequence.
type BlobChange struct {
	ChangeID string `json:"changeId"`
	DeviceID string `json:"deviceId"`
	Blob     []byte `json:"blob"`
	Seq      int64  `json:"seq,omitempty"`
}

// AppendChangesRequest appends one encrypted batch to a workspace log.
type AppendChangesRequest struct {
	Workspace string       `json:"workspace"`
	Changes   []BlobChange `json:"changes"`
}

// AppendChangesResponse returns the stable sequence for each requested change.
type AppendChangesResponse struct {
	Seqs []int64 `json:"seqs"`
}

// ListChangesRequest pages the change log after a sequence cursor.
type ListChangesRequest struct {
	Workspace string `json:"workspace"`
	Since     int64  `json:"since"`
	Limit     int    `json:"limit"`
}

// ListChangesResponse returns changes and the current workspace head.
type ListChangesResponse struct {
	Changes []BlobChange `json:"changes"`
	Head    int64        `json:"head"`
}

// PutSnapshotRequest stores an opaque snapshot covering a log sequence.
type PutSnapshotRequest struct {
	Workspace string `json:"workspace"`
	CoversSeq int64  `json:"coversSeq"`
	Blob      []byte `json:"blob"`
}

// PutSnapshotResponse reports whether the snapshot advanced the stored cursor.
type PutSnapshotResponse struct {
	Accepted bool `json:"accepted"`
}

// GetSnapshotRequest asks for the newest snapshot in a workspace.
type GetSnapshotRequest struct {
	Workspace string `json:"workspace"`
}

// GetSnapshotResponse contains the newest snapshot when one exists.
type GetSnapshotResponse struct {
	Found     bool   `json:"found"`
	CoversSeq int64  `json:"coversSeq"`
	Blob      []byte `json:"blob,omitempty"`
}

// GetKeyringRequest asks for a workspace's opaque wrapped-key blob.
type GetKeyringRequest struct {
	Workspace string `json:"workspace"`
}

// GetKeyringResponse returns the stored wrapped-key version when present.
type GetKeyringResponse struct {
	Found   bool   `json:"found"`
	Version int64  `json:"version"`
	Blob    []byte `json:"blob,omitempty"`
}

// PutKeyringRequest updates a wrapped-key blob with compare-and-swap.
type PutKeyringRequest struct {
	Workspace       string `json:"workspace"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Blob            []byte `json:"blob"`
}

// PutKeyringResponse reports the observed version and whether it advanced.
type PutKeyringResponse struct {
	Version int64 `json:"version"`
	Updated bool  `json:"updated"`
}

// BlobSyncDeleteWorkspaceRequest tombstones a workspace log.
type BlobSyncDeleteWorkspaceRequest struct {
	Workspace string `json:"workspace"`
}

// BlobSyncDeleteWorkspaceResponse reports whether a tombstone was written.
type BlobSyncDeleteWorkspaceResponse struct {
	Deleted bool `json:"deleted"`
}

// CreatePairingRequest places an opaque device-pairing envelope in the relay.
type CreatePairingRequest struct {
	ID         string `json:"id"`
	Blob       []byte `json:"blob"`
	TTLSeconds int64  `json:"ttlSeconds"`
}

// CreatePairingResponse returns the relay's expiration time.
type CreatePairingResponse struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expiresAt"`
}

// ClaimPairingRequest consumes a relay envelope by its id.
type ClaimPairingRequest struct {
	ID string `json:"id"`
}

// ClaimPairingResponse contains the opaque relay envelope when found.
type ClaimPairingResponse struct {
	Found bool   `json:"found"`
	Blob  []byte `json:"blob,omitempty"`
}

// WatchRequest subscribes to a workspace head beginning at a cursor.
type WatchRequest struct {
	Workspace string `json:"workspace"`
	Since     int64  `json:"since"`
}

// WatchEvent announces a newer workspace head sequence.
type WatchEvent struct {
	Workspace string `json:"workspace"`
	Head      int64  `json:"head"`
}

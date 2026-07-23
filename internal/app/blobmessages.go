// SPDX-License-Identifier: MIT

package app

import "github.com/monstercameron/CashFlux/internal/backendrpc"

// buildUploadBlobHeader constructs the header chunk uploadBlobStream sends
// first on a BlobService.UploadBlob client stream. Pulled out as a pure,
// no-build-tag function (backend.go is js&&wasm-gated, so its own logic
// can't be unit-tested on native Go) specifically so WorkspaceID's presence
// is a regression-tested invariant, not something that can silently regress
// to the zero value again the way it did before this function existed: the
// server fails closed on an empty workspace id (blobservice.go), so a
// dropped WorkspaceID here doesn't leak data, but it does mean every upload
// from the real app hard-fails — the exact bug a security scan caught after
// the fact once, that this test exists to make impossible to reintroduce
// silently.
func buildUploadBlobHeader(workspaceID, hash string, declaredSizeBytes int64, mime string) *backendrpc.UploadBlobHeader {
	return &backendrpc.UploadBlobHeader{
		Hash:              hash,
		DeclaredSizeBytes: declaredSizeBytes,
		Mime:              mime,
		WorkspaceID:       workspaceID,
	}
}

// buildDownloadBlobRequest constructs the request downloadBlobStream sends
// to open a BlobService.DownloadBlob server stream. See
// buildUploadBlobHeader's doc comment — same reasoning, same regression.
func buildDownloadBlobRequest(workspaceID, hash string) *backendrpc.DownloadBlobRequest {
	return &backendrpc.DownloadBlobRequest{
		Hash:        hash,
		WorkspaceID: workspaceID,
	}
}

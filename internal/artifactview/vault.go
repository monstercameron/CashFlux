// SPDX-License-Identifier: MIT

// Package artifactview prepares immutable facts for the artifact vault screen.
// It is platform-independent; source acquisition and revision caching belong to
// the screen controller.
package artifactview

import (
	"fmt"
	"math"

	"github.com/monstercameron/CashFlux/internal/store"
)

// Summary contains the counts and storage figures rendered by the artifact
// vault. Reference maps include dangling IDs so callers can explain references
// even when an artifact record is missing. The maps are owned by the result.
type Summary struct {
	TransactionReferences   map[string]int
	PageReferences          map[string]int
	ArtifactCount           int
	ImageCount              int
	PDFCount                int
	OtherCount              int
	ReferencedArtifactCount int
	PageBoundArtifactCount  int
	DatasetBytes            int64
	BlobBytes               int64
	TotalBytes              int64
	StorageMeterBytes       int64
}

// Build prepares the artifact-vault facts from a consistent dataset snapshot.
// DatasetBytes preserves the current storage meter's exact semantics: the byte
// length of store.Export after clearing the device-only OpenAI key. TotalBytes
// adds blob-store usage to that serialized JSON length. Export bytes are
// temporary and are never retained in Summary.
func Build(dataset store.Dataset, blobBytes int64) (Summary, error) {
	if blobBytes < 0 {
		return Summary{}, fmt.Errorf("artifactview: negative blob storage usage: %d", blobBytes)
	}
	result := Summary{
		TransactionReferences: make(map[string]int),
		PageReferences:        make(map[string]int),
		ArtifactCount:         len(dataset.Artifacts),
		BlobBytes:             blobBytes,
	}
	for _, transaction := range dataset.Transactions {
		for _, attachment := range transaction.Attachments {
			result.TransactionReferences[attachment.ArtifactID]++
		}
	}
	for _, page := range dataset.CustomPages {
		for _, widget := range page.Widgets {
			if id := widget.Binding.ArtifactID; id != "" {
				result.PageReferences[id]++
			}
		}
	}
	for _, artifact := range dataset.Artifacts {
		switch artifact.Kind {
		case "image":
			result.ImageCount++
		case "pdf":
			result.PDFCount++
		default:
			// The existing vault labels every other artifact kind as CSV.
			result.OtherCount++
		}
	}
	for id, count := range result.TransactionReferences {
		if id != "" && count > 0 {
			result.ReferencedArtifactCount++
		}
	}
	for _, count := range result.PageReferences {
		if count > 0 {
			result.PageBoundArtifactCount++
		}
	}

	// store.Export stamps the current schema and prunes stale transfer receipts;
	// copy the dataset value and redact only the copied Settings value to match
	// appstate.DatasetBytesWithBlobs without modifying caller-owned data.
	exportDataset := dataset
	exportDataset.Settings.OpenAIKey = ""
	data, err := store.Export(exportDataset)
	if err != nil {
		return Summary{}, fmt.Errorf("artifactview: measure dataset storage: %w", err)
	}
	result.DatasetBytes = int64(len(data))
	result.TotalBytes, err = checkedTotalBytes(result.DatasetBytes, blobBytes)
	if err != nil {
		return Summary{}, err
	}
	result.StorageMeterBytes = result.TotalBytes
	return result, nil
}

func checkedTotalBytes(datasetBytes, blobBytes int64) (int64, error) {
	if datasetBytes < 0 || blobBytes < 0 {
		return 0, fmt.Errorf("artifactview: storage byte counts must be nonnegative")
	}
	if blobBytes > math.MaxInt64-datasetBytes {
		return 0, fmt.Errorf("artifactview: combined storage size overflows int64")
	}
	return datasetBytes + blobBytes, nil
}

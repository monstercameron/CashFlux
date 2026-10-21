// SPDX-License-Identifier: MIT

package artifactview

import (
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestBuildEmptyDataset(t *testing.T) {
	got, err := Build(store.Dataset{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtifactCount != 0 || got.ImageCount != 0 || got.PDFCount != 0 || got.OtherCount != 0 {
		t.Fatalf("empty artifact counts = %#v", got)
	}
	if len(got.TransactionReferences) != 0 || len(got.PageReferences) != 0 || got.ReferencedArtifactCount != 0 || got.PageBoundArtifactCount != 0 {
		t.Fatalf("empty reference counts = %#v", got)
	}
	if got.DatasetBytes <= 0 || got.TotalBytes != got.DatasetBytes {
		t.Fatalf("empty storage bytes = dataset %d total %d", got.DatasetBytes, got.TotalBytes)
	}
}

func TestBuildReferenceAndKindParity(t *testing.T) {
	dataset := store.Dataset{
		Artifacts: []domain.Artifact{
			{ID: "image", Kind: "image"},
			{ID: "pdf", Kind: "pdf"},
			{ID: "csv", Kind: "csv"},
			{ID: "json", Kind: "json"},
		},
		Transactions: []domain.Transaction{{Attachments: []domain.AttachmentRef{
			{ArtifactID: "image"}, {ArtifactID: "image"}, {ArtifactID: "dangling"}, {ArtifactID: ""},
		}}},
		CustomPages: []domain.CustomPage{
			{Widgets: []domain.PageWidget{
				{Binding: domain.WidgetBinding{ArtifactID: "pdf"}},
				{Binding: domain.WidgetBinding{ArtifactID: "pdf"}},
				{Binding: domain.WidgetBinding{ArtifactID: "dangling"}},
				{Binding: domain.WidgetBinding{ArtifactID: ""}},
			}},
			{Widgets: []domain.PageWidget{{Binding: domain.WidgetBinding{ArtifactID: "csv"}}}},
		},
	}

	got, err := Build(dataset, 1234)
	if err != nil {
		t.Fatal(err)
	}
	wantTxnRefs := map[string]int{"image": 2, "dangling": 1, "": 1}
	wantPageRefs := map[string]int{"pdf": 2, "dangling": 1, "csv": 1}
	if !reflect.DeepEqual(got.TransactionReferences, wantTxnRefs) {
		t.Errorf("transaction refs = %#v, want %#v", got.TransactionReferences, wantTxnRefs)
	}
	if !reflect.DeepEqual(got.PageReferences, wantPageRefs) {
		t.Errorf("page refs = %#v, want %#v", got.PageReferences, wantPageRefs)
	}
	if got.ArtifactCount != 4 || got.ImageCount != 1 || got.PDFCount != 1 || got.OtherCount != 2 {
		t.Errorf("kind counts = total %d image %d pdf %d other %d", got.ArtifactCount, got.ImageCount, got.PDFCount, got.OtherCount)
	}
	if got.ReferencedArtifactCount != 2 || got.PageBoundArtifactCount != 3 {
		t.Errorf("unique referenced counts = transaction %d page %d, want 2 and 3", got.ReferencedArtifactCount, got.PageBoundArtifactCount)
	}
	if got.BlobBytes != 1234 || got.TotalBytes != got.DatasetBytes+1234 {
		t.Errorf("storage sizes = dataset %d blobs %d total %d", got.DatasetBytes, got.BlobBytes, got.TotalBytes)
	}
	if got.StorageMeterBytes != got.TotalBytes {
		t.Errorf("meter bytes = %d, want combined total %d", got.StorageMeterBytes, got.TotalBytes)
	}
}

func TestBuildPreservesExportSizeSemanticsAndInput(t *testing.T) {
	dataset := store.Dataset{
		SchemaVersion: 1,
		Settings:      store.Settings{OpenAIKey: "device-secret", BaseCurrency: "USD"},
		Artifacts:     []domain.Artifact{{ID: "a", Kind: "image", Bytes: []byte{1, 2, 3}}},
		Transactions:  []domain.Transaction{{ID: "live"}},
		TransferOperations: []store.TransferOperationReceipt{
			{ID: "live-receipt", Transactions: []domain.Transaction{{ID: "live"}}},
			{ID: "stale-receipt", Transactions: []domain.Transaction{{ID: "deleted"}}},
		},
	}
	before := store.Dataset{
		SchemaVersion:      dataset.SchemaVersion,
		Settings:           dataset.Settings,
		Artifacts:          append([]domain.Artifact(nil), dataset.Artifacts...),
		Transactions:       append([]domain.Transaction(nil), dataset.Transactions...),
		TransferOperations: append([]store.TransferOperationReceipt(nil), dataset.TransferOperations...),
	}
	got, err := Build(dataset, 2_000_000)
	if err != nil {
		t.Fatal(err)
	}
	copy := dataset
	copy.Settings.OpenAIKey = ""
	exported, err := store.Export(copy)
	if err != nil {
		t.Fatal(err)
	}
	if got.DatasetBytes != int64(len(exported)) || got.TotalBytes != int64(len(exported))+2_000_000 {
		t.Errorf("size = dataset %d total %d; export parity = %d plus blobs", got.DatasetBytes, got.TotalBytes, len(exported))
	}
	if got.StorageMeterBytes != got.TotalBytes {
		t.Errorf("meter bytes = %d, want combined total %d", got.StorageMeterBytes, got.TotalBytes)
	}
	if dataset.Settings.OpenAIKey != "device-secret" || dataset.SchemaVersion != before.SchemaVersion || !reflect.DeepEqual(dataset, before) {
		t.Fatal("Build mutated caller-owned dataset")
	}
}

func TestBuildStorageArithmeticWideAndChecked(t *testing.T) {
	wide := int64(math.MaxInt32) + 1
	got, err := Build(store.Dataset{}, wide)
	if err != nil {
		t.Fatal(err)
	}
	if got.BlobBytes != wide || got.TotalBytes != got.DatasetBytes+wide || got.StorageMeterBytes != got.TotalBytes {
		t.Fatalf("wide storage = dataset %d blob %d total %d meter %d", got.DatasetBytes, got.BlobBytes, got.TotalBytes, got.StorageMeterBytes)
	}
	if _, err := Build(store.Dataset{}, -1); err == nil {
		t.Fatal("Build accepted negative blob usage")
	}

	if total, err := checkedTotalBytes(math.MaxInt64-1, 1); err != nil || total != math.MaxInt64 {
		t.Fatalf("checked boundary total = %d, %v", total, err)
	}
	if _, err := checkedTotalBytes(math.MaxInt64-1, 2); err == nil {
		t.Fatal("checked total accepted int64 overflow")
	}
}

func TestBuildReturnsExportError(t *testing.T) {
	_, err := Build(store.Dataset{Tasks: []domain.Task{{Custom: map[string]any{"bad": func() {}}}}}, 0)
	if err == nil {
		t.Fatal("Build accepted a dataset that store.Export cannot encode")
	}
}

func BenchmarkVaultPrepared(b *testing.B) {
	dataset := vaultFixture()
	const blobUsage int64 = 18 << 20
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := Build(dataset, blobUsage)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSink = int(result.TotalBytes) + result.ReferencedArtifactCount + result.PageBoundArtifactCount
	}
}

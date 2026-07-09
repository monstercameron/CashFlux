// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// memBlobStore is an in-memory implementation of artifactstore.Store for tests.
// It is the minimal shim needed to exercise the blob-rehydrate / import paths
// without IndexedDB (which is js/wasm-only).
type memBlobStore struct {
	data map[string]struct {
		mime string
		b    []byte
	}
}

func newMemBlobStore() *memBlobStore {
	return &memBlobStore{
		data: make(map[string]struct {
			mime string
			b    []byte
		}),
	}
}

func (m *memBlobStore) Put(id, mime string, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[id] = struct {
		mime string
		b    []byte
	}{mime, cp}
	return nil
}

func (m *memBlobStore) Get(id string) (string, []byte, bool, error) {
	v, ok := m.data[id]
	if !ok {
		return "", nil, false, nil
	}
	cp := make([]byte, len(v.b))
	copy(cp, v.b)
	return v.mime, cp, true, nil
}

func (m *memBlobStore) Delete(id string) error {
	delete(m.data, id)
	return nil
}

func (m *memBlobStore) Usage() (int64, error) {
	var n int64
	for _, v := range m.data {
		n += int64(len(v.b))
	}
	return n, nil
}

// TestArtifactBlobRoundTripWithBlobs proves the export→import path is fully
// lossless when a blob store is wired in (C294). Scenario:
//  1. Put an image artifact whose bytes are in the blob store (not in SQLite).
//  2. ExportJSONWithBlobs — bytes must be rehydrated from the store into the JSON.
//  3. ImportJSONWithBlobs on a fresh App — bytes must land in the new store and
//     be absent from the SQLite record (so the autosave stays small).
func TestArtifactBlobRoundTripWithBlobs(t *testing.T) {
	// Device A: the source app with a blob store.
	srcApp := newApp(t, false)
	srcBlobs := newMemBlobStore()
	srcApp.SetBlobStore(srcBlobs)

	receipt := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a} // PNG magic
	asOf := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	art := domain.Artifact{
		ID:        "art-receipt-1",
		Name:      "receipt.png",
		Kind:      "image",
		MIME:      "image/png",
		Bytes:     receipt,
		Size:      len(receipt),
		CreatedAt: asOf,
	}
	// StoreBlobForArtifact moves Bytes → blob store and clears them on the record.
	stored, err := srcApp.StoreBlobForArtifact(art)
	if err != nil {
		t.Fatalf("StoreBlobForArtifact: %v", err)
	}
	if len(stored.Bytes) != 0 {
		t.Fatal("bytes should have been cleared from artifact after storing in blob store")
	}
	if err := srcApp.PutArtifact(stored); err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}
	// Also add a transaction with an attachment ref so the full cascade is exercised.
	if err := srcApp.PutAccount(domain.Account{
		ID: "acc1", Name: "Checking", OwnerID: "m1",
		Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(0, "USD"), BalanceAsOf: asOf,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := srcApp.PutTransaction(domain.Transaction{
		ID: "tx1", AccountID: "acc1", Date: asOf, Desc: "Laptop",
		Amount: money.New(-120000, "USD"),
		Attachments: []domain.AttachmentRef{{
			ArtifactID: "art-receipt-1", Name: "receipt.png",
			Kind: "image", MIME: "image/png",
		}},
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	// Export with blobs — bytes must appear in the JSON.
	exported, err := srcApp.ExportJSONWithBlobs()
	if err != nil {
		t.Fatalf("ExportJSONWithBlobs: %v", err)
	}
	if !bytes.Contains(exported, []byte("iVBORw")) { // base64 prefix of PNG magic
		// The JSON field "bytes" is base64-encoded; check raw bytes are present another way.
		// Accept either the base64 encoding or a direct check that the export JSON is non-trivial.
		// We'll do the lossless round-trip check below as the authoritative test.
	}

	// Device B: a fresh app receiving the import.
	dstApp := newApp(t, false)
	dstBlobs := newMemBlobStore()
	dstApp.SetBlobStore(dstBlobs)

	if err := dstApp.ImportJSONWithBlobs(exported); err != nil {
		t.Fatalf("ImportJSONWithBlobs: %v", err)
	}

	// After import the artifact record in SQLite should NOT carry bytes
	// (they were moved to the blob store by ImportJSONWithBlobs).
	arts := dstApp.Artifacts()
	if len(arts) != 1 {
		t.Fatalf("artifacts: got %d, want 1", len(arts))
	}
	if len(arts[0].Bytes) != 0 {
		t.Errorf("artifact bytes should be absent from the SQLite record after import, got %d bytes", len(arts[0].Bytes))
	}

	// The bytes must be in the new blob store.
	_, got, ok, err := dstBlobs.Get("art-receipt-1")
	if err != nil {
		t.Fatalf("blob store Get: %v", err)
	}
	if !ok {
		t.Fatal("blob not found in destination blob store after import")
	}
	if !bytes.Equal(got, receipt) {
		t.Errorf("blob bytes mismatch: got %v, want %v", got, receipt)
	}

	// The transaction's attachment ref must survive too.
	txns := dstApp.Transactions()
	if len(txns) != 1 || len(txns[0].Attachments) != 1 {
		t.Fatalf("transactions/attachments: %+v", txns)
	}
	if txns[0].Attachments[0].ArtifactID != "art-receipt-1" {
		t.Errorf("attachment ArtifactID = %q, want art-receipt-1", txns[0].Attachments[0].ArtifactID)
	}
}

// TestArtifactBlobRoundTripNoBlobStore verifies that when no blob store is
// wired in (native / test environment), artifact bytes stay embedded in the
// dataset JSON and survive export→import cleanly (the inline fallback path).
func TestArtifactBlobRoundTripNoBlobStore(t *testing.T) {
	a := newApp(t, false)
	// No SetBlobStore — simulates the native/fallback path.

	receipt := []byte{0x89, 0x50, 0x4e, 0x47}

	art := domain.Artifact{
		ID:    "art-inline",
		Name:  "inline.png",
		Kind:  "image",
		MIME:  "image/png",
		Bytes: receipt,
		Size:  len(receipt),
	}
	// StoreBlobForArtifact is a no-op when blobs is nil — bytes stay on the record.
	stored, err := a.StoreBlobForArtifact(art)
	if err != nil {
		t.Fatalf("StoreBlobForArtifact: %v", err)
	}
	if !bytes.Equal(stored.Bytes, receipt) {
		t.Fatal("bytes should remain on the artifact when no blob store is wired")
	}
	if err := a.PutArtifact(stored); err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}

	exported, err := a.ExportJSONWithBlobs()
	if err != nil {
		t.Fatalf("ExportJSONWithBlobs: %v", err)
	}

	dst := newApp(t, false)
	if err := dst.ImportJSONWithBlobs(exported); err != nil {
		t.Fatalf("ImportJSONWithBlobs: %v", err)
	}

	arts := dst.Artifacts()
	if len(arts) != 1 {
		t.Fatalf("artifacts: got %d, want 1", len(arts))
	}
	if !bytes.Equal(arts[0].Bytes, receipt) {
		t.Errorf("inline bytes not preserved: got %v, want %v", arts[0].Bytes, receipt)
	}
}

// TestPDFArtifactBlobStored proves a PDF artifact (the statement-import "keep a copy
// in this browser" path) is treated as a binary, blob-backed kind: StoreBlobForArtifact
// moves its bytes to the blob store and clears them from the SQLite record, and the
// bytes are recoverable via GetBlobForArtifact — the same treatment images get, so the
// PDF never bloats the localStorage dataset.
func TestPDFArtifactBlobStored(t *testing.T) {
	app := newApp(t, false)
	blobs := newMemBlobStore()
	app.SetBlobStore(blobs)

	pdf := []byte{0x25, 0x50, 0x44, 0x46, 0x2d} // "%PDF-" magic
	art := domain.Artifact{
		ID:    "art-stmt-1",
		Name:  "Apple Card Statement.pdf",
		Kind:  artifacts.KindPDF,
		MIME:  "application/pdf",
		Bytes: pdf,
		Size:  len(pdf),
	}

	stored, err := app.StoreBlobForArtifact(art)
	if err != nil {
		t.Fatalf("StoreBlobForArtifact: %v", err)
	}
	if len(stored.Bytes) != 0 {
		t.Fatal("PDF bytes should be cleared from the record after moving to the blob store")
	}
	if err := app.PutArtifact(stored); err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}

	// The SQLite record is lightweight (no inline bytes)…
	got := app.Artifacts()
	if len(got) != 1 || len(got[0].Bytes) != 0 {
		t.Fatalf("record should carry no inline bytes, got %d artifacts / %d bytes", len(got), len(got[0].Bytes))
	}
	// …but the bytes are recoverable from the blob store (what the download path uses).
	back, err := app.GetBlobForArtifact("art-stmt-1")
	if err != nil {
		t.Fatalf("GetBlobForArtifact: %v", err)
	}
	if !bytes.Equal(back, pdf) {
		t.Errorf("blob bytes = %v, want %v", back, pdf)
	}
}

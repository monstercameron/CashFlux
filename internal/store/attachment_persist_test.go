// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestAttachmentRoundTrip verifies an image Artifact's bytes and a transaction's
// AttachmentRef survive export→import losslessly (L29 — receipts survive backup).
func TestAttachmentRoundTrip(t *testing.T) {
	bytesIn := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a} // PNG magic
	ds := Dataset{
		Artifacts: []domain.Artifact{{
			ID:        "art-receipt-1",
			Name:      "laptop-receipt.png",
			Kind:      "image",
			MIME:      "image/png",
			Bytes:     bytesIn,
			Size:      len(bytesIn),
			CreatedAt: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		}},
		Transactions: []domain.Transaction{{
			ID:     "tx-laptop",
			Desc:   "Laptop",
			Amount: money.New(-120000, "USD"),
			Date:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			Attachments: []domain.AttachmentRef{{
				ArtifactID: "art-receipt-1",
				Name:       "laptop-receipt.png",
				Kind:       "image",
				MIME:       "image/png",
			}},
		}},
	}

	exported, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(exported)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if len(imported.Artifacts) != 1 || !bytes.Equal(imported.Artifacts[0].Bytes, bytesIn) {
		t.Fatalf("artifact bytes not preserved: got %#v", imported.Artifacts)
	}
	if len(imported.Transactions) != 1 || len(imported.Transactions[0].Attachments) != 1 {
		t.Fatalf("attachment ref not preserved: got %#v", imported.Transactions)
	}
	if got := imported.Transactions[0].Attachments[0].ArtifactID; got != "art-receipt-1" {
		t.Fatalf("attachment artifactID = %q, want art-receipt-1", got)
	}
}

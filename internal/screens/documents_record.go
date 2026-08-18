// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

// recordDocumentAs writes the import-history record for one run, under an id the
// caller already minted.
//
// The id must be decided BEFORE the import so every row it writes can carry it
// (domain.Transaction.SourceDocID). That linkage is the whole point: without it,
// what a run reported adding and what the ledger holds today are two unconnected
// numbers, and the difference between them has no explanation (C687).
//
// duplicates and failed are recorded separately as well as summed into
// SkippedCount, which older readers still use. They are genuinely different
// facts — a duplicate is the safeguard working, a parse failure is money missing
// from the ledger that nothing else will report.
func recordDocumentAs(app *appstate.App, docID string, kind domain.DocumentKind, accountID string,
	rows []extract.Row, rowCount, skipped, duplicates, failed int, cpID string) {
	_ = app.PutDocument(domain.Document{
		ID: docID, Kind: kind, UploadedAt: time.Now(), AccountID: accountID,
		Status: domain.DocImported, Extracted: toDocumentRows(rows), RowCount: rowCount,
		SkippedCount: skipped, DuplicateCount: duplicates, FailedCount: failed,
		CheckpointID: cpID,
	})
}

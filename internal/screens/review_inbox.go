// SPDX-License-Identifier: MIT

//go:build js && wasm

// Transaction Review inbox (CG-S2) — the WRITE PATH.
//
// This file used to hold a whole second review UI, ReviewInboxBody: a
// one-at-a-time triage flow mounted from the shell root. C500's dual-mode review
// surface replaced it, ReviewInboxHost has rendered screens.ReviewSurfaceBody
// ever since, and nothing referenced ReviewInboxBody again — but it stayed, and
// kept attracting fixes. Two separate sessions have since "fixed" bugs in code
// that could never run: the C553 don't-advance-on-a-failed-write guards and the
// C600 queue-reason relabelling were both applied here first, to closures with no
// caller, and had to be applied again in review_surface.go to reach a user. It is
// deleted (C-review cleanup), along with the seven helpers only it used.
//
// What remains is what the live surface calls: the functions that actually write a
// review decision to the store. Categorizing a charge clears its #needs-review tag
// (that is what resolves it), captures an undo point, and persists — an in-memory
// write that never reaches the dataset is undone by the next reload. The pure
// queue selection lives in internal/reviewqueue.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// removeReviewTag returns tags without the review flag.
func removeReviewTag(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != reviewqueue.ReviewTag {
			out = append(out, t)
		}
	}
	return out
}

// assignReviewCategory sets one transaction's category (clearing the review flag,
// since categorizing resolves it) and persists. A failed write (e.g. a read-only
// Viewer identity) is surfaced as a notice — swallowing it left the inbox frozen
// on the same item with zero feedback (QA CF-02). Returns whether the write
// landed, so callers can offer an undo toast only for real changes.
func assignReviewCategory(app *appstate.App, txnID, catID string) bool {
	for _, t := range app.Transactions() {
		if t.ID == txnID {
			t.CategoryID = catID
			t.Tags = removeReviewTag(t.Tags)
			// C617: a confirmed decision IS the review. Clearing only the queue's
			// inputs (category + tag) left `Reviewed` false, so the row vanished
			// from the inbox while the ledger still badged it "Needs review" —
			// the app disagreeing with itself about the same transaction.
			t.Reviewed = true
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(err.Error(), true)
				uistate.BumpDataRevision()
				return false
			}
			// Learn from the confirmation, so the next charge from this merchant
			// is suggested locally instead of costing a SMART+ call (C513).
			rememberReviewChoice(t, catID)
			// C553: PutTransaction writes the in-memory store; RequestPersist is
			// what puts it in the dataset. Without this the card advanced and the
			// queue count dropped, but a reload brought the transaction back
			// Uncategorized — the write looked like it landed and had not. Same
			// defect as C543 on the categories page, in a second surface.
			uistate.RequestPersist()
			uistate.BumpDataRevision()
			return true
		}
	}
	return false
}

// assignReviewByMerchant categorizes every queued transaction sharing the merchant
// key (the current one included) in one pass, so a repeated charge clears in a
// single action. Returns how many transactions were written (0 when the write
// failed), so callers can offer an undo toast only for real changes.
func assignReviewByMerchant(app *appstate.App, key, catID string) int {
	var writeErr error
	n := 0
	app.BulkMutate(func() {
		for _, t := range app.Transactions() {
			if !reviewqueue.Needs(t) {
				continue
			}
			if reviewqueue.MerchantKey(t) == key {
				t.CategoryID = catID
				t.Tags = removeReviewTag(t.Tags)
				t.Reviewed = true // C617: same rule as the single path — confirming IS reviewing.
				if err := app.PutTransaction(t); err != nil {
					if writeErr == nil {
						writeErr = err
					}
					continue
				}
				rememberReviewChoice(t, catID)
				n++
			}
		}
	})
	if writeErr != nil {
		uistate.PostNotice(writeErr.Error(), true)
	}
	// C553: persist the batch for the same reason the single path does — an
	// in-memory write that never reaches the dataset is undone by a reload.
	if n > 0 {
		uistate.RequestPersist()
	}
	uistate.BumpDataRevision()
	return n
}

// postCategorizedUndo captures an undo point for the categorization that just
// landed and shows an undoable toast naming the category, so a slip in the
// review flow is reversible in one click (report item: review loop needs undo).
func postCategorizedUndo(app *appstate.App, catID string, batch int) {
	auditview.CaptureNow()
	name := reviewCatName(app, catID)
	if batch > 1 {
		uistate.PostUndoable(uistate.T("review.categorizedBatchUndo", batch, name))
		return
	}
	uistate.PostUndoable(uistate.T("review.categorizedUndo", name))
}

func reviewCatName(app *appstate.App, id string) string {
	if id == "" {
		return uistate.T("review.uncategorized")
	}
	for _, c := range app.Categories() {
		if c.ID == id {
			return c.Name
		}
	}
	return uistate.T("review.uncategorized")
}

func reviewAcctName(app *appstate.App, id string) string {
	for _, a := range app.Accounts() {
		if a.ID == id {
			return a.Name
		}
	}
	return ""
}

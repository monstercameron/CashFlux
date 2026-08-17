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
// What remains is what the live surface calls: the function that actually writes a
// review decision to the store. Categorizing a charge clears its #needs-review tag
// (that is what resolves it), captures an undo point, and persists — an in-memory
// write that never reaches the dataset is undone by the next reload. The pure
// queue selection lives in internal/reviewqueue, and the pure "how many charges
// does this click change?" decision in internal/reviewscope.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catname"
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

// assignReviewToCharges categorizes exactly the listed transactions — clearing
// the review flag, since categorizing resolves it — in one pass, and persists.
// It returns how many were written (0 when nothing matched or every write
// failed), so callers can offer an undo toast only for real changes.
//
// The caller names the charges. This used to be two functions, one keyed on a
// transaction id and one that re-derived a merchant key and swept the whole
// ledger for matches — and the sweep is where C616 lived (a raw payee passed
// where a normalized reviewqueue.MerchantKey was required matched zero rows) and
// where C653 lived (a card depicting ONE charge quietly wrote 122). Taking the id
// list the surface already built to draw and count the card makes both impossible:
// the number on the button and the number written come from the same slice.
//
// A failed write (e.g. a read-only Viewer identity) is surfaced as a notice —
// swallowing it left the inbox frozen on the same item with zero feedback
// (QA CF-02). Charges that have left the queue since the card was drawn are
// skipped rather than silently re-categorized.
func assignReviewToCharges(app *appstate.App, ids []string, catID string) int {
	if app == nil || catID == "" || len(ids) == 0 {
		return 0
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return 0
	}
	var writeErr error
	n := 0
	app.BulkMutate(func() {
		for _, t := range app.Transactions() {
			if !want[t.ID] || !reviewqueue.Needs(t) {
				continue
			}
			t.CategoryID = catID
			t.Tags = removeReviewTag(t.Tags)
			// C617: a confirmed decision IS the review. Clearing only the queue's
			// inputs (category + tag) left `Reviewed` false, so the row vanished
			// from the inbox while the ledger still badged it "Needs review" —
			// the app disagreeing with itself about the same transaction.
			t.Reviewed = true
			if err := app.PutTransaction(t); err != nil {
				if writeErr == nil {
					writeErr = err
				}
				continue
			}
			// Learn from the confirmation, so the next charge from this merchant
			// is suggested locally instead of costing a SMART+ call (C513).
			rememberReviewChoice(t, catID)
			n++
		}
	})
	if writeErr != nil {
		uistate.PostNotice(writeErr.Error(), true)
	}
	// The caller showed a count before the click — "Categorize all 122" — and that
	// count came from a memoized snapshot. If anything resolved one of these charges
	// in between (a SMART+ scan callback landing, a rule firing, another surface),
	// fewer are written than were promised, and the user has no way to notice: the
	// dialog they read said 122 and the queue simply drops by 119. Saying so is the
	// difference between a count that was wrong and a count that corrected itself.
	if n > 0 && n < len(want) && writeErr == nil {
		uistate.PostNotice(uistate.T("review.wroteFewer", n, len(want)), false)
	}
	// C553: PutTransaction writes the in-memory store; RequestPersist is what puts
	// it in the dataset. Without this the card advanced and the queue count
	// dropped, but a reload brought the transaction back Uncategorized — the write
	// looked like it landed and had not. Same defect as C543 on the categories
	// page, in a second surface.
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

// reviewCatName names a category the way the picker the user chose from does.
//
// It used to return the bare leaf while `categorySelectNodes` renders every
// option through `catname.Label` — so a confirmation reading "as Home
// maintenance" and an undo toast reading "Categorized 122 transactions as Home
// maintenance" both dropped the parent the user had actually navigated, at
// exactly the moment (a 122-charge batch) where knowing WHICH "Home maintenance"
// matters most. C619's rule is one naming convention app-wide, and catname.Label
// qualifies only when the leaf is genuinely ambiguous, so this costs nothing when
// there is nothing to disambiguate.
func reviewCatName(app *appstate.App, id string) string {
	if id == "" {
		return uistate.T("review.uncategorized")
	}
	cats := app.Categories()
	for _, c := range cats {
		if c.ID == id {
			return catname.Label(cats, c)
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

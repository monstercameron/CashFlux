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
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnreview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

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
	assignments := make(map[string]string, len(ids))
	for _, id := range ids {
		assignments[id] = catID
	}
	return commitReviewAssignments(app, assignments, nil)
}

// commitReviewAssignments confirms exactly the live queued rows in one atomic,
// undoable write. Completion, learning and success copy wait for strict durability.
func commitReviewAssignments(app *appstate.App, assignments map[string]string, done func(int, error)) int {
	if app == nil {
		return 0
	}
	var updates []domain.Transaction
	var ids []string
	splitSkipped := false
	for _, txn := range app.Transactions() {
		category := assignments[txn.ID]
		if category == "" || !reviewqueue.Needs(txn) {
			continue
		}
		if reviewqueue.ReasonFor(txn) == reviewqueue.ReasonSplitUnbalanced {
			splitSkipped = true
			continue
		}
		updates = append(updates, txnreview.Confirm(txn, category))
		ids = append(ids, txn.ID)
	}
	if splitSkipped && appstate.Default == app {
		uistate.PostNotice(uistate.T("review.splitNeeded"), false)
	}
	if len(updates) == 0 {
		return 0
	}
	label := uistate.T("review.bulkApplied", len(updates))
	err := CommitUndoable(app, label, ids, func() error { return app.PutTransactionsAtomic(updates) }, func(err error) {
		if appstate.Default == app {
			if err == nil {
				for _, txn := range updates {
					rememberReviewChoice(txn, txn.CategoryID)
				}
				if len(updates) < len(assignments) {
					uistate.PostNotice(uistate.T("review.wroteFewer", len(updates), len(assignments)), false)
				}
			} else {
				uistate.PostNotice(uistate.T("review.commitFailed"), true)
			}
		}
		if done != nil {
			done(len(updates), err)
		}
	})
	if err != nil {
		return 0
	}
	return len(updates)
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

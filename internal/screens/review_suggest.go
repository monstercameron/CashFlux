// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// reviewSuggestion resolves the best LOCAL (SMART) category proposal for a
// transaction: a rule the user wrote, what they did with this charge or this
// merchant before, or the shipped merchant dictionary — ranked by
// internal/catsuggest. No model is consulted and nothing leaves the device; the
// SMART+ path is separate and explicit.
//
// The rules engine runs here (via AutoCategorizeTransaction, which is rules-only)
// and its answer is handed to the resolver, so catsuggest stays free of that
// dependency.
func reviewSuggestion(app *appstate.App, t domain.Transaction) (catsuggest.Suggestion, bool) {
	if app == nil {
		return catsuggest.Suggestion{}, false
	}
	return reviewSuggestionWith(app, t, uistate.LoadLearnTally(), uistate.LoadLearnThreshold(), app.Categories())
}

// reviewSuggestionWith is reviewSuggestion with its expensive inputs hoisted out.
//
// The tally and threshold each come from a settings-KV read plus a full JSON
// unmarshal. Resolving a queue merchant-by-merchant through reviewSuggestion
// therefore re-parsed the ENTIRE tally once per merchant, which is what made
// opening the surface block for a second on a real dataset. Callers that resolve
// more than one charge must load once and pass the values in.
func reviewSuggestionWith(
	app *appstate.App, t domain.Transaction,
	tally learntally.Tally, threshold int, cats []domain.Category,
) (catsuggest.Suggestion, bool) {
	if app == nil {
		return catsuggest.Suggestion{}, false
	}
	ruleCat := ""
	if r := app.AutoCategorizeTransaction(t); r.CategoryID != t.CategoryID {
		ruleCat = r.CategoryID
	}
	return catsuggest.Resolve(catsuggest.Input{
		Payee:          t.Payee,
		Desc:           t.Desc,
		AmountMinor:    t.Amount.Amount,
		RuleCategoryID: ruleCat,
		Tally:          tally,
		Threshold:      threshold,
		Categories:     cats,
	})
}

// reviewWhy renders the evidence behind a suggestion in plain English, so the
// user can judge it rather than trust it (the no-black-boxes rule). Returns ""
// when there is nothing worth saying.
func reviewWhy(s catsuggest.Suggestion) string {
	switch s.Source {
	case catsuggest.SourceRule:
		return uistate.T("review.whyRule")
	case catsuggest.SourceHistoryExact:
		return uistate.T("review.whyHistoryExact", s.Count, s.Total)
	case catsuggest.SourceHistoryMerchant:
		return uistate.T("review.whyHistoryMerchant", s.Count, s.Total)
	case catsuggest.SourceDictionary:
		if s.Merchant == "" {
			return ""
		}
		return uistate.T("review.whyDictionary", s.Merchant)
	}
	return ""
}

// applyReviewChoice is the ONE path every apply action in the review inbox takes
// — confirm, the SMART suggestion chip, and the SMART+ button alike.
//
// It exists because the batch flag used to be read by only one of the three
// (C496): checking "also apply to N others" and then clicking a different button
// silently categorized a single transaction while looking like a batch. Routing
// every caller through here means a fourth action cannot forget the flag.
// applyReviewChoice reports whether the categorization actually LANDED.
//
// C553: it used to return nothing, so the caller advanced to the next card
// unconditionally — including when the write had failed. A queue that moves on
// from an item it did not save is worse than one that refuses to move, because
// the user has no way to tell the two apart. The boolean is what lets the caller
// hold position and show the reason instead.
func applyReviewChoice(app *appstate.App, cur domain.Transaction, categoryID string, batch bool) bool {
	if app == nil || categoryID == "" {
		return false
	}
	if batch {
		// C616: the key MUST be reviewqueue.MerchantKey, not the raw payee.
		// assignReviewByMerchant matches on `reviewqueue.MerchantKey(t) == key`,
		// and MerchantKey is payeealias.Normalize + lowercase — so passing the raw
		// descriptor ("VENMO PAYMENT 1042778120") could never equal the normalized
		// key of the very transaction it came from. Every single-mode confirm
		// therefore matched ZERO rows, returned 0, and (since C553 made a failed
		// write honest) reported "That didn't save" while the card sat still. Bulk
		// worked the whole time because it passes the group's own MerchantKey.
		// This is C498's raw-vs-cleaned mismatch, reintroduced on one call path.
		n := assignReviewByMerchant(app, reviewqueue.MerchantKey(cur), categoryID)
		if n > 0 {
			postCategorizedUndo(app, categoryID, n)
		}
		return n > 0
	}
	if assignReviewCategory(app, cur.ID, categoryID) {
		postCategorizedUndo(app, categoryID, 1)
		return true
	}
	return false
}

// rememberReviewChoice records a confirmed categorization so the next charge
// from the same merchant can be suggested without a model. Recording under both
// the exact descriptor and the merchant stem is what lets a payee that stamps a
// per-charge reference ever reach the suggestion threshold (C513).
func rememberReviewChoice(t domain.Transaction, categoryID string) {
	payee := rawPayeeOf(t)
	if payee == "" || categoryID == "" {
		return
	}
	uistate.RecordLearnTally(payee, categoryID)
}

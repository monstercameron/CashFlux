// SPDX-License-Identifier: MIT

//go:build js && wasm

// Inline "categorize this" row affordance (SM-2). An uncategorized charge whose
// merchant the app already recognizes — from a rule the user wrote, from what they
// filed the same merchant under before, or from the shipped merchant dictionary —
// gets a one-click chip in its category cell instead of a trip through the edit
// modal. Everything here is the FREE tier: internal/catsuggest resolves it locally
// and nothing leaves the device. The Smart+ ask lives in the kebab's modal, where
// its cost can be stated before it is spent.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	"github.com/monstercameron/CashFlux/internal/txnreview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The two catalog codes this feature spans, and the reason there are two.
//
// The gate must be the FREE code. AI-tier features default to OFF (the cost is
// real, so opting in is the honest default), and gating a local, free, no-key
// suggestion on an AI code would have hidden it from everyone who never turned
// Smart+ on — which is most people. SMART-T1F is the deterministic tier and
// defaults on; SMART-T1 gates only the model ask inside the modal. Same pairing
// as SMART-T3F / SMART-T3 for natural-language search.
const (
	catSuggestFreeCode = "SMART-T1F"
	catSuggestAICode   = "SMART-T1"
)

// txnCatSuggestChipProps carries the row's transaction id. The chip resolves the
// suggestion itself rather than taking one as a prop, so the table's row builder
// does not pay for a resolve on every row it renders — only mounted chips resolve.
type txnCatSuggestChipProps struct {
	TxnID    string
	OnChoose func(ui.Event)
}

type txnCatSuggestInputs struct {
	byID        map[string]domain.Transaction
	suggestions map[string]txnCatSuggestion
	categories  []domain.Category
	tally       learntally.Tally
	threshold   int
}

type txnCatSuggestion struct {
	value catsuggest.Suggestion
	ok    bool
}

type txnCatSuggestInputsCache struct {
	value smartcatview.RevisionCache[*appstate.App, txnCatSuggestInputs]
}

func (c *txnCatSuggestInputsCache) get(app *appstate.App, revision int, build func() txnCatSuggestInputs) txnCatSuggestInputs {
	var storeRevision uint64
	if app != nil {
		storeRevision = app.Rev()
	}
	return c.value.Get(app, revision, storeRevision, build)
}

var txnCatSuggestCache txnCatSuggestInputsCache

func loadTxnCatSuggestInputs(app *appstate.App) txnCatSuggestInputs {
	inputs := txnCatSuggestInputs{
		byID:        make(map[string]domain.Transaction),
		suggestions: make(map[string]txnCatSuggestion),
	}
	if app == nil {
		return inputs
	}
	txns := app.Transactions()
	inputs.byID = smartcatview.IndexTransactions(txns)
	inputs.categories = app.Categories()
	inputs.tally = uistate.LoadLearnTally()
	inputs.threshold = uistate.LoadLearnThreshold()
	return inputs
}

func (inputs *txnCatSuggestInputs) suggestion(app *appstate.App, txn domain.Transaction) (catsuggest.Suggestion, bool) {
	result := smartcatview.Memo(inputs.suggestions, txn.ID, func() txnCatSuggestion {
		suggestion, ok := reviewSuggestionWith(app, txn, inputs.tally, inputs.threshold, inputs.categories)
		return txnCatSuggestion{value: suggestion, ok: ok}
	})
	return result.value, result.ok
}

func txnCatSuggestInputsFor(app *appstate.App, revision int) txnCatSuggestInputs {
	if app != nil {
		revision = int(app.Rev())
	}
	return txnCatSuggestCache.get(app, revision, func() txnCatSuggestInputs { return loadTxnCatSuggestInputs(app) })
}

// txnCatSuggestChip renders the one-click suggestion for an uncategorized charge.
// It is its own component because it owns a click hook, and a per-row hook inside
// the table's variable-length row loop is the framework's cardinal sin.
func txnCatSuggestChip(props txnCatSuggestChipProps) ui.Node {
	revision := uistate.UseDataRevision().Get()
	app := appstate.Default

	// The click hook is registered unconditionally (stable hook order) before any
	// early return; it re-reads the transaction at click time so it can never act
	// on a stale row.
	apply := ui.UseEvent(func() {
		a := appstate.Default
		if a == nil {
			return
		}
		current := txnCatSuggestInputsFor(a, uistate.CurrentDataRevision())
		txn, ok := current.byID[props.TxnID]
		if !ok {
			return
		}
		s, ok := current.suggestion(a, txn)
		if !ok || s.CategoryID == "" || !categorySuggestionMatchesDirection(txn, s.CategoryID, current.categories) {
			return
		}
		applyCatSuggestion(a, txn, s.CategoryID)
	})
	choose := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnChoose != nil {
			props.OnChoose(e)
		}
	})
	applyClick := OnClick(apply)
	chooseClick := OnClick(choose)

	if app == nil || !uistate.LoadSmartSettings().IsEnabled(catSuggestFreeCode) {
		return Fragment()
	}
	inputs := txnCatSuggestInputsFor(app, revision)
	txn, ok := inputs.byID[props.TxnID]
	if !smartcatview.EligibleTransaction(txn, ok) {
		return Fragment()
	}
	s, ok := inputs.suggestion(app, txn)
	if !smartcatview.ChipVisible(txn, s, ok, inputs.categories) {
		return Fragment()
	}
	cats := inputs.categories
	name := catname.LabelByID(cats, s.CategoryID)
	return Fragment(Button(css.Class("txn-catsuggest"), Type("button"),
		Attr("data-testid", "txn-catsuggest-chip"),
		Attr("aria-label", uistate.T("r1b.categorySuggestionAria", name)),
		Title(uistate.T("catSuggest.chipHint")),
		applyClick,
		uistate.T("r10w.suggestionInline", name),
	), Button(ClassStr("txn-cat-choose"), Type("button"),
		Attr("data-testid", "txn-category-choose"),
		Attr("aria-label", uistate.T("r10w.chooseCategory")),
		chooseClick, uistate.T("r10w.choose")))
}

// categorySuggestionMatchesDirection prevents a rule with a mismatched category
// kind from appearing as a suggestion on a transaction in the opposite direction.
func categorySuggestionMatchesDirection(txn domain.Transaction, categoryID string, categories []domain.Category) bool {
	return smartcatview.MatchesDirection(txn, categoryID, categories)
}

// applyCatSuggestion files ONE charge under a category and makes it undoable.
//
// It deliberately does not route through the review inbox's apply path. That path
// carries queue semantics this affordance does not have — merchant-wide batching,
// card advancement, queue-position bookkeeping — and a chip on a ledger row means
// exactly one charge. Reusing it would import behavior that has to be suppressed
// argument by argument, which is how a "reuse" ends up costlier than the four
// lines it replaced.
//
// The order matters: capture the undo point BEFORE the write, persist after it
// (the in-memory store is not the dataset — a write that skips RequestPersist
// looks like it landed and is gone after a reload), and record the choice so the
// next charge from this merchant is suggested without a model.
func applyCatSuggestion(app *appstate.App, txn domain.Transaction, categoryID string) {
	if app == nil || categoryID == "" || txn.ID == "" {
		return
	}
	live, ok := app.TransactionByID(txn.ID)
	if !ok || live.IsTransfer() {
		return
	}
	confirmed := txnreview.Confirm(live, categoryID)
	_ = CommitUndoable(app, uistate.T("catSuggest.filedUndo", catname.LabelByID(app.Categories(), categoryID)), []string{txn.ID}, func() error {
		if err := app.PutTransactionsAtomic([]domain.Transaction{confirmed}); err != nil {
			return err
		}
		rememberReviewChoice(live, categoryID)
		return nil
	}, nil)
}

// txnByID finds a transaction by id in the live app state.
func txnByID(app *appstate.App, id string) (domain.Transaction, bool) {
	if app == nil || id == "" {
		return domain.Transaction{}, false
	}
	return app.TransactionByID(id)
}

// categoryByID finds a category by id in a category slice.
func categoryByID(cats []domain.Category, id string) (domain.Category, bool) {
	return smartcatview.CategoryByID(cats, id)
}

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
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
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
	app      *appstate.App
	revision int
	inputs   txnCatSuggestInputs
	valid    bool
}

func (c *txnCatSuggestInputsCache) get(app *appstate.App, revision int, build func() txnCatSuggestInputs) txnCatSuggestInputs {
	if !c.valid || c.app != app || c.revision != revision {
		c.app, c.revision, c.inputs, c.valid = app, revision, build(), true
	}
	return c.inputs
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
	inputs.byID = make(map[string]domain.Transaction, len(txns))
	for _, txn := range txns {
		inputs.byID[txn.ID] = txn
	}
	inputs.categories = app.Categories()
	inputs.tally = uistate.LoadLearnTally()
	inputs.threshold = uistate.LoadLearnThreshold()
	return inputs
}

func (inputs *txnCatSuggestInputs) suggestion(app *appstate.App, txn domain.Transaction) (catsuggest.Suggestion, bool) {
	if cached, ok := inputs.suggestions[txn.ID]; ok {
		return cached.value, cached.ok
	}
	suggestion, ok := reviewSuggestionWith(app, txn, inputs.tally, inputs.threshold, inputs.categories)
	inputs.suggestions[txn.ID] = txnCatSuggestion{value: suggestion, ok: ok}
	return suggestion, ok
}

func txnCatSuggestInputsFor(app *appstate.App, revision int) txnCatSuggestInputs {
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

	if app == nil || !uistate.LoadSmartSettings().IsEnabled(catSuggestFreeCode) {
		return Fragment()
	}
	inputs := txnCatSuggestInputsFor(app, revision)
	txn, ok := inputs.byID[props.TxnID]
	if !ok || txn.CategoryID != "" || txn.IsTransfer() {
		return Fragment()
	}
	s, ok := inputs.suggestion(app, txn)
	if !ok || s.CategoryID == "" || !categorySuggestionMatchesDirection(txn, s.CategoryID, inputs.categories) {
		return Fragment()
	}
	// A coin-flip history is a signal, not an answer: offering a one-click apply for
	// it would train the user to accept without reading. Those charges keep the
	// kebab's modal (where the evidence is spelled out) as their only path.
	if s.Confidence < catsuggest.ConfMedium {
		return Fragment()
	}
	cats := inputs.categories
	if _, ok := categoryByID(cats, s.CategoryID); !ok {
		return Fragment()
	}
	name := catname.LabelByID(cats, s.CategoryID)

	return Fragment(Button(css.Class("txn-catsuggest"), Type("button"),
		Attr("data-testid", "txn-catsuggest-chip"),
		Attr("aria-label", uistate.T("r1b.categorySuggestionAria", name)),
		Title(uistate.T("catSuggest.chipHint")),
		OnClick(apply),
		uistate.T("r10w.suggestionInline", name),
	), Button(ClassStr("txn-cat-choose"), Type("button"),
		Attr("data-testid", "txn-category-choose"),
		Attr("aria-label", uistate.T("r10w.chooseCategory")),
		OnClick(choose), uistate.T("r10w.choose")))
}

// categorySuggestionMatchesDirection prevents a rule with a mismatched category
// kind from appearing as a suggestion on a transaction in the opposite direction.
func categorySuggestionMatchesDirection(txn domain.Transaction, categoryID string, categories []domain.Category) bool {
	category, ok := categoryByID(categories, categoryID)
	if !ok {
		return false
	}
	want := domain.KindExpense
	if txn.Amount.IsPositive() {
		want = domain.KindIncome
	}
	return category.Kind == want
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
	auditview.CaptureNow()
	t := txn
	t.CategoryID = categoryID
	if err := app.PutTransaction(t); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	rememberReviewChoice(txn, categoryID)
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	uistate.PostUndoable(uistate.T("catSuggest.filedUndo", catname.LabelByID(app.Categories(), categoryID)))
}

// txnByID finds a transaction by id in the live app state.
func txnByID(app *appstate.App, id string) (domain.Transaction, bool) {
	if app == nil || id == "" {
		return domain.Transaction{}, false
	}
	for _, t := range app.Transactions() {
		if t.ID == id {
			return t, true
		}
	}
	return domain.Transaction{}, false
}

// categoryByID finds a category by id in a category slice.
func categoryByID(cats []domain.Category, id string) (domain.Category, bool) {
	if id == "" {
		return domain.Category{}, false
	}
	for _, c := range cats {
		if c.ID == id {
			return c, true
		}
	}
	return domain.Category{}, false
}

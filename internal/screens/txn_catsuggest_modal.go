// SPDX-License-Identifier: MIT

//go:build js && wasm

// "Categorize this" flip modal (SM-2), opened from a transaction row's kebab. The
// row chip (txn_catsuggest_chip.go) is the one-click path for a confident local
// suggestion; this is the fuller one — it states the EVIDENCE behind the
// suggestion, offers the whole category list when the suggestion is wrong, lets
// one decision cover every charge from the merchant, and is the only place the
// paid Smart+ ask is offered, where its cost can be stated before it is spent.
package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// CatSuggestFormID is shared by the modal's body <form> and the FlipPanel
// footer's Save button, so the pinned footer submits while the body scrolls.
const CatSuggestFormID = "catsuggest-form"

// CatSuggestBody is the body of the categorize flip modal (mounted at the shell
// root by app.CatSuggestHost).
func CatSuggestBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseCatSuggest()
	txnID := openAtom.Get()
	pr := uistate.UsePrefs().Get()

	// Hooks first, unconditionally, before any early return.
	chosen := ui.UseState("")
	batch := ui.UseState(false)
	loading := ui.UseState(false)
	errText := ui.UseState("")
	aiNote := ui.UseState("")
	seeded := ui.UseState("")

	if app == nil || txnID == "" {
		return Fragment()
	}
	txn, found := txnByID(app, txnID)
	if !found {
		return Fragment()
	}
	cats := app.Categories()

	// The local suggestion and the evidence for it. Resolved every render (it is a
	// cheap in-memory read) so a correction made elsewhere is reflected on reopen.
	sugg, hasSugg := reviewSuggestion(app, txn)
	why := ""
	if hasSugg {
		why = reviewWhy(sugg)
	}

	// Seed the picker from the suggestion the first time this charge's modal opens
	// (re-seed when reopened for a different charge). An already-categorized charge
	// seeds from its own category, so reopening confirms what is set rather than
	// silently proposing a change.
	if seeded.Get() != txnID {
		switch {
		case txn.CategoryID != "":
			chosen.Set(txn.CategoryID)
		case hasSugg:
			chosen.Set(sugg.CategoryID)
		default:
			chosen.Set("")
		}
		batch.Set(false)
		errText.Set("")
		aiNote.Set("")
		seeded.Set(txnID)
	}

	// How many OTHER charges share this merchant — the batch offer's scope. It uses
	// reviewqueue.MerchantKey (payee-alias normalized, lower-cased), never the raw
	// descriptor: a merchant that stamps a per-charge reference would otherwise
	// match nothing, including the very charge the key came from (C616).
	key := reviewqueue.MerchantKey(txn)
	others := 0
	if key != "" {
		for _, t := range app.Transactions() {
			if t.ID != txn.ID && t.CategoryID == "" && !t.IsTransfer() && reviewqueue.MerchantKey(t) == key {
				others++
			}
		}
	}

	// Smart+ availability: a configured provider plus the AI half enabled. The
	// deterministic half above needed neither.
	backendAI := pr.Normalize().BackendActive()
	aiEnabled := aiProviderConfigured(app, backendAI) && uistate.LoadSmartSettings().IsEnabled(catSuggestAICode)
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	onPick := func(v string) { chosen.Set(v); errText.Set("") }
	onBatch := ui.UseEvent(func() { batch.Set(!batch.Get()) })

	askAI := ui.UseEvent(Prevent(func() {
		if loading.Get() {
			return
		}
		loading.Set(true)
		errText.Set("")
		aiNote.Set("")
		catalog := smartCatalog(cats)
		runSmartAI(aiConn, smartai.Categorize(txnLabelOf(txn)+" "+fmtMoney(txn.Amount), catalog.Prompt()),
			func(text string) {
				loading.Set(false)
				answer := strings.TrimSpace(text)
				// The model's answer is resolved against the household's REAL
				// categories and dropped when it does not match. It may not invent
				// one here any more than it may in T16.
				if hit, ok := catalog.Lookup(answer); ok {
					chosen.Set(hit.ID)
					aiNote.Set(uistate.T("catSuggest.aiSource"))
					return
				}
				aiNote.Set(uistate.T("catSuggest.aiUnmatched", answer))
			},
			func(e string) { loading.Set(false); errText.Set(e) })
	}))

	save := ui.UseEvent(Prevent(func() {
		catID := chosen.Get()
		if catID == "" {
			errText.Set(uistate.T("catSuggest.needCategory"))
			return
		}
		a := appstate.Default
		if a == nil {
			return
		}
		cur, ok := txnByID(a, txnID)
		if !ok {
			return
		}
		if batch.Get() && others > 0 {
			applyCatSuggestionToMerchant(a, cur, catID)
		} else {
			applyCatSuggestion(a, cur, catID)
		}
		uistate.CloseCatSuggest()
	}))

	// The suggestion block: the proposal plus WHY, so the user judges it rather
	// than trusts it. Absent when nothing local matched — which is exactly the
	// case Smart+ exists for, so the copy says so instead of showing an empty row.
	var suggestNode ui.Node
	if hasSugg {
		suggestNode = Div(css.Class("csug-suggest"), Attr("data-testid", "catsuggest-suggestion"),
			Div(css.Class(tw.Text12, tw.TextDim), uistate.T("catSuggest.suggestedLabel")),
			Div(css.Class("csug-suggest-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
				smartGlyph(false, tw.Fold(tw.W35, tw.H35)),
				Span(css.Class("csug-suggest-name"), catname.LabelByID(cats, sugg.CategoryID)),
			),
			If(why != "", P(css.Class("csug-why", tw.Text12, tw.TextDim), why)),
		)
	} else {
		suggestNode = P(css.Class("csug-none", tw.Text13, tw.TextDim),
			Attr("data-testid", "catsuggest-none"), uistate.T("catSuggest.noSuggestion"))
	}

	var aiBtn ui.Node = Fragment()
	if aiEnabled {
		label := uistate.T("catSuggest.suggestAI")
		if loading.Get() {
			label = uistate.T("catSuggest.asking")
		}
		aiBtn = Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "catsuggest-ai"), Attr("aria-disabled", ariaBool(loading.Get())), OnClick(askAI),
			smartGlyph(true, tw.Fold(tw.W35, tw.H35)), Span(label))
	}

	kind := domain.KindExpense
	if txn.Amount.Amount > 0 {
		kind = domain.KindIncome
	}

	return Form(css.Class("csug", tw.FlexCol, tw.Gap3),
		Attr("id", CatSuggestFormID), Attr("data-testid", "catsuggest-modal"), OnSubmit(save),
		// The charge itself, read-only, so it is clear what is being filed.
		Div(css.Class("csug-txn"),
			Div(css.Class(tw.Text12, tw.TextDim), uistate.T("catSuggest.txnLabel")),
			Div(css.Class("csug-txn-val"),
				Span(css.Class("csug-txn-payee"), txnLabelOf(txn)),
				Span(css.Class("csug-txn-amt"), fmtMoney(txn.Amount)),
			),
		),
		suggestNode,
		// The full list, so a wrong suggestion costs one dropdown rather than a
		// trip to the edit modal.
		uiw.FormField(uistate.T("catSuggest.pickLabel"),
			Div(css.Class("csug-pick-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
				uiw.SelectInput(uiw.SelectInputProps{
					AriaLabel: uistate.T("catSuggest.pickLabel"),
					Selected:  chosen.Get(),
					Options:   catSuggestOptions(cats, kind),
					OnChange:  onPick,
					TestID:    "catsuggest-pick",
				}),
				aiBtn,
			),
		),
		If(aiNote.Get() != "", P(css.Class("csug-ainote", tw.Text12, tw.TextDim),
			Attr("data-testid", "catsuggest-ainote"), aiNote.Get())),
		// One decision, every charge from this merchant. Offered only when there
		// ARE others, so it is never a no-op dressed as a choice.
		If(others > 0, Label(css.Class("csug-batch", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Style(map[string]string{"cursor": "pointer"}),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"),
				Attr("data-testid", "catsuggest-batch"), OnChange(onBatch)}, checkedAttr(batch.Get())...)...),
			Span(css.Class(tw.Text13), uistate.T("catSuggest.batchLabel", others)))),
		If(errText.Get() != "", P(css.Class("err"), Attr("role", "alert"), errText.Get())),
	)
}

// catSuggestOptions builds the picker's options: a "choose" placeholder, then
// every category of the charge's own kind as a qualified label. Filtering by kind
// is what stops an expense being handed an income category — the same sign rule
// catsuggest and the Smart+ parsers enforce.
func catSuggestOptions(cats []domain.Category, kind domain.CategoryKind) []uiw.SelectOption {
	out := []uiw.SelectOption{{Value: "", Label: uistate.T("review.choose")}}
	for _, o := range categoryOptions(cats, kind) {
		out = append(out, uiw.SelectOption{Value: o.ID, Label: o.Path})
	}
	return out
}

// applyCatSuggestionToMerchant files every UNCATEGORIZED charge from the same
// merchant, plus the one in hand, under a category — one undo point for the lot.
//
// It deliberately leaves already-categorized charges alone. "Also file the other
// N" is an offer to finish the job, not to overwrite decisions the user already
// made, and the count in the label is built from the same predicate so the number
// they agreed to is the number that moves.
func applyCatSuggestionToMerchant(app *appstate.App, txn domain.Transaction, categoryID string) {
	if app == nil || categoryID == "" {
		return
	}
	key := reviewqueue.MerchantKey(txn)
	if key == "" {
		applyCatSuggestion(app, txn, categoryID)
		return
	}
	auditview.CaptureNow()
	n := 0
	for _, t := range app.Transactions() {
		// The charge in hand always moves — it is the one the user was looking at.
		// Every OTHER charge must match the merchant AND still be uncategorized,
		// which is exactly the predicate the count in the checkbox label was built
		// from, so the number agreed to is the number that moves.
		if t.ID != txn.ID {
			if t.CategoryID != "" || t.IsTransfer() || reviewqueue.MerchantKey(t) != key {
				continue
			}
		}
		u := t
		u.CategoryID = categoryID
		if err := app.PutTransaction(u); err != nil {
			uistate.PostNotice(err.Error(), true)
			continue
		}
		n++
	}
	if n == 0 {
		return
	}
	rememberReviewChoice(txn, categoryID)
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	name := catname.LabelByID(app.Categories(), categoryID)
	if n > 1 {
		uistate.PostUndoable(uistate.T("catSuggest.filedBatchUndo", n, name))
		return
	}
	uistate.PostUndoable(uistate.T("catSuggest.filedUndo", name))
}

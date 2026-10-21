// SPDX-License-Identifier: MIT

//go:build js && wasm

// "Categorize this" flip modal, opened from a transaction row's kebab. The
// row chip (txn_catsuggest_chip.go) is the one-click path for a confident local
// suggestion; this is the fuller one — it states the EVIDENCE behind the
// suggestion, offers the whole category list when the suggestion is wrong, lets
// one decision cover every charge from the merchant, and is the only place the
// paid Smart+ ask is offered, where its cost can be stated before it is spent.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// CatSuggestFormID links the form to the modal's pinned Save button.
const CatSuggestFormID = "catsuggest-form"

// CatSuggestBody hosts the category suggestion modal's state and app effects.
func CatSuggestBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	open := uistate.UseCatSuggest()
	txnID := open.Get()
	pr := uistate.UsePrefs().Get()
	chosen := ui.UseState("")
	pickerOpen := ui.UseState(false)
	batch := ui.UseState(false)
	loading := ui.UseState(false)
	errText := ui.UseState("")
	aiNote := ui.UseState("")
	seeded := ui.UseState("")
	var txn domain.Transaction
	var cats []domain.Category
	var txns []domain.Transaction
	found := false
	if app != nil && txnID != "" {
		txn, found = txnByID(app, txnID)
		if found {
			cats = app.Categories()
			txns = app.Transactions()
		}
	}
	var suggestion catsuggest.Suggestion
	hasSuggestion := false
	if found {
		suggestion, hasSuggestion = reviewSuggestion(app, txn)
	}
	why := ""
	if hasSuggestion {
		why = reviewWhy(suggestion)
	}
	if found && seeded.Get() != txnID {
		chosen.Set(smartcatview.SeedCategory(txn, suggestion, hasSuggestion))
		batch.Set(false)
		errText.Set("")
		aiNote.Set("")
		seeded.Set(txnID)
	}
	others := smartcatview.MerchantOthers(txn, txns)
	backend := uistate.BackendAIActive(pr)
	aiEnabled := app != nil && aiProviderConfigured(app, backend) && uistate.LoadSmartSettings().IsEnabled(catSuggestAICode)
	var conn smartAIConn
	if app != nil {
		conn = resolveAIConn(app, backend, pr.ServerURL, pr.ServerToken)
	}
	ask := func() {
		if loading.Get() {
			return
		}
		loading.Set(true)
		errText.Set("")
		aiNote.Set("")
		runSmartAI(conn, smartcatview.CategorizeRequest(txn, cats), func(text string) {
			loading.Set(false)
			id, key, answer := smartcatview.CategoryAnswer(text, cats)
			if id != "" {
				chosen.Set(id)
				aiNote.Set(uistate.T(key))
				return
			}
			aiNote.Set(uistate.T(key, answer))
		}, func(message string) { loading.Set(false); errText.Set(message) })
	}
	save := func() {
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
	}
	if !found {
		return Fragment()
	}
	return ui.CreateElement(catSuggestForm, catSuggestFormProps{
		Txn: txn, Categories: cats, Suggestion: suggestion, HasSuggestion: hasSuggestion, Why: why,
		Chosen: chosen.Get(), PickerOpen: pickerOpen.Get(), Batch: batch.Get(), Others: others,
		AIEnabled: aiEnabled, Loading: loading.Get(), AINote: aiNote.Get(), Error: errText.Get(),
		OnSave: save, OnAsk: ask, OnPick: func(id string) { chosen.Set(id); errText.Set("") },
		OnNewCategory: func() { pickerOpen.Set(true) }, OnBatch: func() { batch.Set(!batch.Get()) },
		OnPickerPick:  func(id string) { chosen.Set(id); errText.Set(""); pickerOpen.Set(false) },
		OnPickerClose: func() { pickerOpen.Set(false) },
	})
}

type catSuggestFormProps struct {
	Txn                                                  domain.Transaction
	Categories                                           []domain.Category
	Suggestion                                           catsuggest.Suggestion
	HasSuggestion                                        bool
	Why, Chosen, AINote, Error                           string
	PickerOpen, Batch, AIEnabled, Loading                bool
	Others                                               int
	OnSave, OnAsk, OnNewCategory, OnBatch, OnPickerClose func()
	OnPick, OnPickerPick                                 func(string)
}

func catSuggestForm(p catSuggestFormProps) ui.Node {
	save := ui.UseEvent(Prevent(p.OnSave))
	newCategory := ui.UseEvent(p.OnNewCategory)
	kind := smartcatview.CategoryKind(p.Txn)
	picker := Fragment()
	if p.PickerOpen {
		picker = ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind: kind, Selected: p.Chosen, OnPick: p.OnPickerPick, OnClose: p.OnPickerClose,
		})
	}
	return Fragment(Form(css.Class("csug", tw.FlexCol, tw.Gap3),
		Attr("id", CatSuggestFormID), Attr("data-testid", "catsuggest-modal"), OnSubmit(save),
		Div(css.Class("csug-txn"),
			Div(css.Class(tw.Text12, tw.TextDim), uistate.T("catSuggest.txnLabel")),
			Div(css.Class("csug-txn-val"),
				Span(css.Class("csug-txn-payee"), smartcatview.TransactionLabel(p.Txn)),
				Span(css.Class("csug-txn-amt"), smartcatview.TransactionAmount(p.Txn)),
			),
		),
		ui.CreateElement(catSuggestEvidence, catSuggestEvidenceProps{
			Available: p.HasSuggestion, Name: catname.LabelByID(p.Categories, p.Suggestion.CategoryID), Why: p.Why,
		}),
		uiw.FormField(uistate.T("catSuggest.pickLabel"),
			Div(css.Class("csug-pick-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
				uiw.SelectInput(uiw.SelectInputProps{
					AriaLabel: uistate.T("catSuggest.pickLabel"), Selected: p.Chosen,
					Options: catSuggestOptions(p.Categories, kind), OnChange: p.OnPick, TestID: "catsuggest-pick",
				}),
				TxnNewCategoryAction("catsuggest-new-category", p.PickerOpen, newCategory),
				ui.CreateElement(catSuggestAIAction, catSuggestAIActionProps{Enabled: p.AIEnabled, Loading: p.Loading, OnAsk: p.OnAsk}),
			),
		),
		If(p.AINote != "", P(css.Class("csug-ainote", tw.Text12, tw.TextDim), Attr("data-testid", "catsuggest-ainote"), p.AINote)),
		ui.CreateElement(catSuggestBatch, catSuggestBatchProps{Others: p.Others, Checked: p.Batch, OnChange: p.OnBatch}),
		If(p.Error != "", P(css.Class("err"), Attr("role", "alert"), p.Error)),
	), picker)
}

type catSuggestEvidenceProps struct {
	Available bool
	Name, Why string
}

func catSuggestEvidence(p catSuggestEvidenceProps) ui.Node {
	if !p.Available {
		return P(css.Class("csug-none", tw.Text13, tw.TextDim), Attr("data-testid", "catsuggest-none"), uistate.T("catSuggest.noSuggestion"))
	}
	return Div(css.Class("csug-suggest"), Attr("data-testid", "catsuggest-suggestion"),
		Div(css.Class(tw.Text12, tw.TextDim), uistate.T("catSuggest.suggestedLabel")),
		Div(css.Class("csug-suggest-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
			smartGlyph(false, tw.Fold(tw.W35, tw.H35)), Span(css.Class("csug-suggest-name"), p.Name),
		),
		If(p.Why != "", P(css.Class("csug-why", tw.Text12, tw.TextDim), p.Why)),
	)
}

type catSuggestAIActionProps struct {
	Enabled, Loading bool
	OnAsk            func()
}

func catSuggestAIAction(p catSuggestAIActionProps) ui.Node {
	ask := ui.UseEvent(Prevent(p.OnAsk))
	click := OnClick(ask)
	if !p.Enabled {
		return Fragment()
	}
	label := uistate.T(smartcatview.CategoryAskKey(p.Loading))
	return Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "catsuggest-ai"), Attr("aria-disabled", ariaBool(p.Loading)), click,
		smartGlyph(true, tw.Fold(tw.W35, tw.H35)), Span(label))
}

type catSuggestBatchProps struct {
	Others   int
	Checked  bool
	OnChange func()
}

func catSuggestBatch(p catSuggestBatchProps) ui.Node {
	change := ui.UseEvent(p.OnChange)
	option := OnChange(change)
	return If(p.Others > 0, Label(css.Class("csug-batch", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Style(map[string]string{"cursor": "pointer"}),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "catsuggest-batch"), option}, checkedAttr(p.Checked)...)...),
		Span(css.Class(tw.Text13), uistate.T("catSuggest.batchLabel", p.Others))))
}

// catSuggestOptions adapts the pure option values to the framework's props.
func catSuggestOptions(cats []domain.Category, kind domain.CategoryKind) []uiw.SelectOption {
	options := smartcatview.CategoryOptions(cats, kind, uistate.T("review.choose"))
	out := make([]uiw.SelectOption, 0, len(options))
	for _, o := range options {
		out = append(out, uiw.SelectOption{Value: o.Value, Label: o.Label})
	}
	return out
}

// applyCatSuggestionToMerchant persists one undoable, merchant-scoped confirmation.
func applyCatSuggestionToMerchant(app *appstate.App, txn domain.Transaction, categoryID string) {
	if app == nil || categoryID == "" {
		return
	}
	if reviewqueue.MerchantKey(txn) == "" {
		applyCatSuggestion(app, txn, categoryID)
		return
	}
	writes, ids := smartcatview.MerchantWrites(txn, app.Transactions(), categoryID)
	if len(writes) == 0 {
		return
	}
	name := catname.LabelByID(app.Categories(), categoryID)
	label := smartcatview.FiledLabel(len(writes), name, uistate.T)
	_ = CommitUndoable(app, label, ids, func() error {
		if err := app.PutTransactionsAtomic(writes); err != nil {
			return err
		}
		rememberReviewChoice(txn, categoryID)
		return nil
	}, nil)
}

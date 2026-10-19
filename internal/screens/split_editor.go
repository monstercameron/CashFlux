// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// SplitModalFormID is the id shared by the split modal's body <form> and the
// FlipPanel footer's Save (type=submit form=…) button, so the pinned footer drives
// the editor's save. Only used by the modal host (TxnSplitHost); the inline uses
// leave FooterFormID empty and keep their own buttons.
const SplitModalFormID = "txn-split-form"

const splitNewCategoryOption = "__split_new_category__"
const splitCategoryResetOption = "__split_category_reset__"

type splitEditorProps struct {
	Txn        domain.Transaction
	Categories []domain.Category
	// Members is the household roster used to populate the optional per-line owner
	// picker. When empty (single-member household), no owner UI is shown and every
	// line stays attributed to the transaction's payer.
	Members []domain.Member
	// OnSave persists the transaction with its Splits set (empty slice clears the
	// breakdown). The parent (transactions screen) wires it to PutTransaction.
	OnSave func(domain.Transaction)
	// Pending holds Save/Clear while the parent awaits its durable result.
	Pending bool
	// OnReceiptSplit opens the receipt-assisted path from this editor, keeping
	// manual and receipt-based splits under one ledger action.
	OnReceiptSplit func(txnID string)
	// FooterFormID, when set, renders the editor as a modal body <form> with that id
	// and its own pinned Cancel/Save bar, dropping the inline layout's title and card
	// border — the modal chrome supplies those. Empty (the default) keeps the
	// self-contained inline layout for the edit-form and classic-table uses.
	//
	// C566: the editor owns this footer rather than borrowing the FlipPanel's
	// standard one. Whether the draft can be saved is a fact only the editor holds
	// (are all the lines finished? does the money add up?), and the panel's footer had
	// no way to read it — so an unfinished split showed a live Save that could only
	// fail. One component now owns both the verdict and the button that acts on it.
	FooterFormID string
	// OnCancel dismisses the modal from the editor's own footer. Only used with
	// FooterFormID; the inline layout has no Cancel (it is not a modal).
	OnCancel func()
	// DraftGeneration changes only when a caller accepts a new proposal. Ordinary
	// parent renders keep the user's in-progress draft; a generation change reseeds it.
	DraftGeneration int
}

// SplitEditor wires the pure splitdraft state to its host's persistence callback.
// Child components own the controls; each splitRow owns its own input hooks.
func SplitEditor(props splitEditorProps) ui.Node {
	draft := ui.UseState(splitdraft.New(props.Txn, props.DraftGeneration))
	_ = uistate.UseDataRevision().Get()
	ui.UseEffect(func() func() {
		if draft.Get().Generation != props.DraftGeneration {
			draft.Set(draft.Get().Reseed(props.Txn, props.DraftGeneration))
		}
		return nil
	}, props.DraftGeneration)
	change := func(action splitdraft.Action) { draft.Set(draft.Get().Apply(action, props.Txn)) }
	save := func() {
		next, transaction, ok := draft.Get().AttemptSave(props.Txn, props.Pending)
		draft.Set(next)
		if ok && props.OnSave != nil {
			props.OnSave(transaction)
		}
	}
	clear := func() {
		next, transaction, ok := draft.Get().Clear(props.Txn, props.Pending)
		draft.Set(next)
		if ok && props.OnSave != nil {
			props.OnSave(transaction)
		}
	}
	cancel := func() {
		if props.OnCancel != nil {
			props.OnCancel()
		}
	}
	return ui.CreateElement(splitEditorContent, splitEditorContentProps{
		Editor: props, Draft: draft.Get(), Change: change, Save: save, Clear: clear, Cancel: cancel,
	})
}

type splitEditorContentProps struct {
	Editor              splitEditorProps
	Draft               splitdraft.State
	Change              func(splitdraft.Action)
	Save, Clear, Cancel func()
}

func splitEditorContent(props splitEditorContentProps) ui.Node {
	p, draft := props.Editor, props.Draft
	view := draft.View(p.Txn)
	categories := p.Categories
	if app := appstate.Default; app != nil {
		categories = app.Categories()
	}
	catOpts := append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}},
		CategoryPickOptions(splitdraft.CategoriesForKind(categories, splitdraft.CategoryKind(p.Txn)))...)
	catOpts = append(catOpts, uiw.SelectOption{Value: splitNewCategoryOption, Label: uistate.T("v5form.splitNewCategory")})
	ownerOpts := splitOwnerOptions(p.Members)
	showOwner := splitdraft.ShowOwner(p.Members)
	setCat := func(i int, v string) {
		props.Change(splitdraft.Action{Kind: splitdraft.SetCategory, Index: i, Value: v})
	}
	setAmt := func(i int, v string) { props.Change(splitdraft.Action{Kind: splitdraft.SetAmount, Index: i, Value: v}) }
	setOwner := func(i int, v string) { props.Change(splitdraft.Action{Kind: splitdraft.SetOwner, Index: i, Value: v}) }
	openPicker := func(i int) { props.Change(splitdraft.Action{Kind: splitdraft.OpenPicker, Index: i}) }
	remove := func(i int) { props.Change(splitdraft.Action{Kind: splitdraft.Remove, Index: i}) }
	var rows []ui.Node
	for i, line := range draft.Lines {
		rows = append(rows, ui.CreateElement(splitRow, splitRowProps{
			Index: i, Cat: line.Cat, Amt: line.Amt, Owner: line.Owner, PickerOpen: draft.PickerLine == i,
			CatOpts: catOpts, OwnerOpts: ownerOpts, ShowOwner: showOwner, Dec: currency.Decimals(p.Txn.Amount.Currency),
			Percent: draft.Percent, OnCat: setCat, OnNewCategory: openPicker, OnAmt: setAmt, OnOwner: setOwner, OnRemove: remove,
		}))
	}
	picker := Fragment()
	if view.PickerOpen {
		picker = ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind: splitdraft.CategoryKind(p.Txn), Selected: view.PickerCategory,
			OnPick: func(id string) {
				props.Change(splitdraft.Action{Kind: splitdraft.PickCategory, Index: draft.PickerLine, Value: id})
			},
			OnClose: func() { props.Change(splitdraft.Action{Kind: splitdraft.ClosePicker}) },
		})
	}
	receipt := ui.UseEvent(func() {
		if p.OnReceiptSplit != nil {
			p.OnReceiptSplit(p.Txn.ID)
		}
	})
	layout := splitEditorLayoutProps{
		Editor: p, Hint: P(css.Class("muted"), Style(map[string]string{"margin-bottom": "0.5rem"}), uistate.T("splitEditor.hint", view.Hint)),
		ReceiptAction: ui.CreateElement(splitReceiptAction, splitReceiptActionProps{Show: p.OnReceiptSplit != nil, OnClick: receipt}),
		ModeToggle:    ui.CreateElement(splitModeToggle, splitModeToggleProps{Percent: draft.Percent, Change: props.Change}),
		Rows:          Div(css.Class("split-rows"), rows),
		AddRow:        ui.CreateElement(splitDraftStatus, splitDraftStatusProps{View: view, Change: props.Change}),
		Error:         splitErrorNode(draft.ErrorKey), Save: props.Save, Clear: props.Clear, Cancel: props.Cancel, ShowOwner: showOwner, Picker: picker,
	}
	if p.FooterFormID != "" {
		return ui.CreateElement(splitEditorModal, layout)
	}
	return ui.CreateElement(splitEditorInline, layout)
}

func splitOwnerOptions(members []domain.Member) []uiw.SelectOption {
	if !splitdraft.ShowOwner(members) {
		return nil
	}
	options := uiw.OptionsFrom(members, func(m domain.Member) string { return m.ID }, func(m domain.Member) string { return m.Name }, "")
	return append([]uiw.SelectOption{{Value: "", Label: uistate.T("splitEditor.ownerSameAsTxn")}}, options...)
}

func splitErrorNode(key string) ui.Node {
	if key == "" {
		return Fragment()
	}
	return P(css.Class("muted", "neg"), Attr("role", "alert"), uistate.T(key))
}

type splitModeToggleProps struct {
	Percent bool
	Change  func(splitdraft.Action)
}

func splitModeToggle(props splitModeToggleProps) ui.Node {
	amounts := ui.UseEvent(func() { props.Change(splitdraft.Action{Kind: splitdraft.ToAmounts}) })
	percents := ui.UseEvent(func() { props.Change(splitdraft.Action{Kind: splitdraft.ToPercents}) })
	return Div(Style(map[string]string{"display": "flex", "gap": "0.35rem", "margin-bottom": "0.5rem"}),
		Button(ClassStr(splitdraft.ModeClass(!props.Percent)), Type("button"), Attr("data-testid", "split-mode-amount"),
			Attr("aria-pressed", ariaBool(!props.Percent)), OnClick(amounts), uistate.T("splitEditor.modeAmounts")),
		Button(ClassStr(splitdraft.ModeClass(props.Percent)), Type("button"), Attr("data-testid", "split-mode-percent"),
			Attr("aria-pressed", ariaBool(props.Percent)), OnClick(percents), uistate.T("splitEditor.modePercents")),
	)
}

type splitDraftStatusProps struct {
	View   splitdraft.View
	Change func(splitdraft.Action)
}

func splitDraftStatus(props splitDraftStatusProps) ui.Node {
	add := ui.UseEvent(func() { props.Change(splitdraft.Action{Kind: splitdraft.Add}) })
	var text string
	if props.View.Value != "" {
		text = uistate.T(props.View.Key, props.View.Value)
	} else {
		text = uistate.T(props.View.Key)
	}
	return Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem", "align-items": "center", "flex-wrap": "wrap"}),
		Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-add"), OnClick(add), uistate.T("splitEditor.add")),
		Span(ClassStr("hero-stat-sub "+props.View.Tone), Attr("data-testid", "split-remainder"), Attr("role", "status"), text),
	)
}

type splitEditorLayoutProps struct {
	Editor        splitEditorProps
	Hint          ui.Node
	ReceiptAction ui.Node
	ModeToggle    ui.Node
	Rows          ui.Node
	AddRow        ui.Node
	Error         ui.Node
	Save          func()
	Clear         func()
	Cancel        func()
	ShowOwner     bool
	Picker        ui.Node
}

func splitEditorModal(props splitEditorLayoutProps) ui.Node {
	save := ui.UseEvent(Prevent(props.Save))
	clearAction := ui.UseEvent(props.Clear)
	cancel := ui.UseEvent(Prevent(props.Cancel))
	p := props.Editor
	var inert any
	if p.Pending {
		inert = Attr("inert", "")
	}
	clear := ui.CreateElement(splitClearButton, splitClearButtonProps{Show: p.Txn.HasSplits(), Pending: p.Pending, OnClick: clearAction})
	return Fragment(Form(css.Class("split-editor split-editor-modal"), inert, Attr("id", p.FooterFormID),
		Attr("data-testid", "split-editor"), Attr("aria-busy", strconv.FormatBool(p.Pending)),
		If(p.Pending, P(Attr("role", "status"), uistate.T("common.saving"))), OnSubmit(save),
		Div(css.Class("modal-scroll"), props.Hint, props.ReceiptAction, props.ModeToggle,
			splitColumnLabels(props.ShowOwner), props.Rows, props.AddRow, props.Error,
			If(p.Txn.HasSplits(), Div(css.Class("split-editor-clear"),
				Style(map[string]string{"margin-top": "0.75rem", "padding-top": "0.6rem", "border-top": "1px solid var(--border)"}), clear))),
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "split-cancel"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "split-save"), Disabled(p.Pending), uistate.T("splitEditor.save"))),
	), props.Picker)
}

func splitEditorInline(props splitEditorLayoutProps) ui.Node {
	saveAction := ui.UseEvent(Prevent(props.Save))
	clearAction := ui.UseEvent(props.Clear)
	p := props.Editor
	var inert any
	if p.Pending {
		inert = Attr("inert", "")
	}
	save := Button(css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "split-save"), Disabled(p.Pending), OnClick(saveAction), uistate.T("splitEditor.save"))
	clear := ui.CreateElement(splitClearButton, splitClearButtonProps{Show: p.Txn.HasSplits(), Pending: p.Pending, OnClick: clearAction})
	return Fragment(Div(css.Class("split-editor"), inert, Attr("data-testid", "split-editor"), Attr("aria-busy", strconv.FormatBool(p.Pending)),
		If(p.Pending, P(Attr("role", "status"), uistate.T("common.saving"))),
		Style(map[string]string{"margin-top": "0.75rem", "padding": "0.75rem", "border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("hero-flanker-label"), Attr("data-testid", "split-inline-title"), Style(map[string]string{"margin-bottom": "0.4rem", "text-align": "left"}), uistate.T("splitEditor.title")),
		props.Hint, props.ReceiptAction, props.ModeToggle, splitColumnLabels(props.ShowOwner), props.Rows, props.AddRow, props.Error,
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem"}), save, clear),
	), props.Picker)
}

func splitColumnLabels(showOwner bool) ui.Node {
	owner := Fragment()
	className := "split-column-labels"
	if showOwner {
		className += " has-owner"
		owner = P(Attr("data-testid", "split-label-who"), uistate.T("splitEditor.ownerColumn"))
	}
	return Div(css.Class(className), Attr("data-testid", "split-labels"),
		Style(map[string]string{"display": "grid", "gap": "0.5rem", "margin-bottom": "0.35rem", "font-size": "0.8rem", "font-weight": "600", "color": "var(--muted)"}),
		P(Attr("data-testid", "split-label-category"), uistate.T("splitEditor.category")),
		owner,
		P(Attr("data-testid", "split-label-amount"), uistate.T("splitEditor.amount")),
		Span(Attr("aria-hidden", "true")),
	)
}

type splitReceiptActionProps struct {
	Show    bool
	OnClick ui.Handler
}

func splitReceiptAction(props splitReceiptActionProps) ui.Node {
	click := OnClick(props.OnClick)
	if !props.Show {
		return Fragment()
	}
	return Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "txn-receipt-split-open"),
		click, uistate.T("receiptsplit.menuAction"))
}

type splitClearButtonProps struct {
	Show    bool
	Pending bool
	OnClick ui.Handler
}

func splitClearButton(props splitClearButtonProps) ui.Node {
	click := OnClick(props.OnClick)
	if !props.Show {
		return Fragment()
	}
	return Button(css.Class("btn", "btn-sm", "btn-ghost"), Type("button"), Attr("data-testid", "split-clear"),
		Disabled(props.Pending), click, uistate.T("splitEditor.clear"))
}

// Compatibility adapters retain existing callers while decisions live in splitdraft.
func splitColumns(showOwner bool) string { return splitdraft.Columns(showOwner) }

func splitRemainderTone(saveAttempted bool) string { return splitdraft.RemainderTone(saveAttempted) }

// splitCellLabel translates the pure summary for existing transaction-row callers.
func splitCellLabel(tx domain.Transaction) (label, tooltip string) {
	count, tooltip := splitdraft.CellSummary(tx)
	if count == 0 {
		return "", ""
	}
	return uistate.T("splitEditor.cellLabel", count), tooltip
}

type splitRowProps struct {
	Index     int
	Cat       string
	Amt       string
	Owner     string
	CatOpts   []uiw.SelectOption
	OwnerOpts []uiw.SelectOption
	ShowOwner bool
	Dec       int
	// Percent renders the value input as a percentage entry (percent mode).
	Percent       bool
	PickerOpen    bool
	OnCat         func(int, string)
	OnNewCategory func(int)
	OnAmt         func(int, string)
	OnOwner       func(int, string)
	OnRemove      func(int)
}

// splitRow is one editable split line (category + amount + remove). It is its own
// component so its OnChange/OnInput/OnClick hooks live at stable positions per row
// instead of inside the parent's variable-length loop (the framework gotcha).
func splitRow(props splitRowProps) ui.Node {
	selectedCategory := ui.UseState(props.Cat)
	ui.UseEffect(func() func() {
		selectedCategory.Set(props.Cat)
		return nil
	}, props.Cat, props.PickerOpen)
	onCat := ui.UseEvent(func(e ui.Event) {
		v := e.GetValue()
		if v == splitNewCategoryOption {
			// The sentinel opens the picker; it is not the split category. Move the
			// controlled select away from the sentinel, then the picker-open render
			// restores the current category before the overlay is dismissed.
			selectedCategory.Set(splitCategoryResetOption)
			props.OnNewCategory(props.Index)
			return
		}
		selectedCategory.Set(v)
		props.OnCat(props.Index, v)
	})
	onAmt := ui.UseEvent(func(v string) { props.OnAmt(props.Index, v) })
	// TX16: on blur, evaluate an arithmetic entry ("12+8", "45.99*3") and replace
	// it with the result; a plain number or a parse failure is left untouched.
	onAmtBlur := ui.UseEvent(func(e ui.Event) {
		if s, ok := EvalAmountField(e.GetValue()); ok {
			props.OnAmt(props.Index, s)
		}
	})
	onOwner := func(v string) { props.OnOwner(props.Index, v) }
	onRemove := ui.UseEvent(func() { props.OnRemove(props.Index) })
	// A hairline + padding under each row visually separates the splits so multiple category
	// breakdowns don't blur together (coworker feedback).
	rowClass := "split-row"
	if props.ShowOwner {
		rowClass += " has-owner"
	}
	return Div(css.Class(rowClass), Attr("data-testid", "split-row"),
		Div(css.Class("split-category-control"),
			Select(append([]any{
				css.Class("field"),
				Value(selectedCategory.Get()),
				Attr("aria-label", uistate.T("splitEditor.category")),
				Attr("data-testid", "split-cat-"+strconv.Itoa(props.Index)),
				OnChange(onCat),
			}, splitCategoryOptionNodes(props.CatOpts, selectedCategory.Get())...)...),
		),
		// XC10: optional per-line owner. Only rendered for multi-member households;
		// "Same as transaction" (empty) keeps the pre-XC10 payer attribution.
		If(props.ShowOwner, Div(Style(map[string]string{"min-width": "0"}),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.OwnerOpts,
				Selected:  props.Owner,
				AriaLabel: uistate.T("splitEditor.owner"),
				TestID:    "split-owner-" + strconv.Itoa(props.Index),
				OnChange:  onOwner,
			}))),
		Div(css.Class("split-amount-control"),
			Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"),
				Attr("aria-label", uistate.T(splitAmtKey(props.Percent))), Attr("data-testid", "split-amt-"+strconv.Itoa(props.Index)),
				Placeholder(uistate.T(splitAmtKey(props.Percent))), OnInput(onAmt), OnBlur(onAmtBlur), uiw.FieldValue(props.Amt)),
			If(props.Percent, Span(css.Class("muted"), Attr("aria-hidden", "true"), "%")),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("splitEditor.remove")),
			Title(uistate.T("splitEditor.remove")), Attr("data-testid", "split-remove-"+strconv.Itoa(props.Index)),
			OnClick(onRemove), "✕"),
	)
}

func splitCategoryOptionNodes(options []uiw.SelectOption, selected string) []any {
	nodes := make([]any, 0, len(options))
	for _, option := range options {
		nodes = append(nodes, Option(Value(option.Value), SelectedIf(selected == option.Value), option.Label))
	}
	return nodes
}

// splitAmtKey translates the pure input-label decision for existing callers.
func splitAmtKey(percent bool) string { return splitdraft.AmountKey(percent) }

// absMinor retains the shared legacy magnitude helper for other screens.
func absMinor(v int64) int64 { return splitdraft.Magnitude(v) }

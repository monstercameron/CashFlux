// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// splitDraft is one editable line of a transaction's category breakdown: a
// category id and the amount (major-unit string, sign-agnostic — the parent
// applies the transaction's sign on save).
type splitDraft struct {
	Cat string
	Amt string
}

type splitEditorProps struct {
	Txn        domain.Transaction
	Categories []domain.Category
	// OnSave persists the transaction with its Splits set (empty slice clears the
	// breakdown). The parent (transactions screen) wires it to PutTransaction.
	OnSave func(domain.Transaction)
}

// SplitEditor (C58) is the split-transaction UI: it lets a single transaction be
// broken into per-category amounts (e.g. a Costco receipt → groceries + shopping).
// The domain model (domain.CategorySplit, SplitsTotal/SplitsReconcile), persistence
// (store round-trip), and sample data already existed; this is the missing thin
// shell over them. It is its own component so its hooks never sit inside the
// variable-length split-row loop, and each row is a further child component
// (splitRow) that owns its per-row input hooks.
func SplitEditor(props splitEditorProps) ui.Node {
	dec := currency.Decimals(props.Txn.Amount.Currency)

	seed := func() []splitDraft {
		if props.Txn.HasSplits() {
			out := make([]splitDraft, 0, len(props.Txn.Splits))
			for _, s := range props.Txn.Splits {
				out = append(out, splitDraft{Cat: s.CategoryID, Amt: money.FormatMinor(absMinor(s.Amount.Amount), dec)})
			}
			return out
		}
		// No splits yet: start with the whole amount on the transaction's current
		// category, plus a blank line to split off, so the common "carve a piece out"
		// flow is one tap away.
		return []splitDraft{
			{Cat: props.Txn.CategoryID, Amt: money.FormatMinor(absMinor(props.Txn.Amount.Amount), dec)},
			{Cat: "", Amt: ""},
		}
	}

	splits := ui.UseState(seed())
	errMsg := ui.UseState("")

	setCat := func(i int, v string) {
		cur := append([]splitDraft(nil), splits.Get()...)
		if i >= 0 && i < len(cur) {
			cur[i].Cat = v
			splits.Set(cur)
		}
	}
	setAmt := func(i int, v string) {
		cur := append([]splitDraft(nil), splits.Get()...)
		if i >= 0 && i < len(cur) {
			cur[i].Amt = v
			splits.Set(cur)
		}
	}
	addRow := ui.UseEvent(func() { splits.Set(append(append([]splitDraft(nil), splits.Get()...), splitDraft{})) })
	removeRow := func(i int) {
		cur := splits.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		out := append(append([]splitDraft(nil), cur[:i]...), cur[i+1:]...)
		splits.Set(out)
	}

	// Live total of the entered split amounts (minor units, unsigned) and the
	// transaction's own amount, so the remainder line tells the user how much is
	// still unallocated before they can save.
	var total int64
	parseErr := false
	for _, d := range splits.Get() {
		if d.Amt == "" {
			continue
		}
		v, err := money.ParseMinor(d.Amt, dec)
		if err != nil {
			parseErr = true
			continue
		}
		total += absMinor(v)
	}
	txnAbs := absMinor(props.Txn.Amount.Amount)
	remainder := txnAbs - total

	save := ui.UseEvent(func() {
		cur := splits.Get()
		built := make([]domain.CategorySplit, 0, len(cur))
		var sum int64
		for _, d := range cur {
			if d.Cat == "" || d.Amt == "" {
				continue
			}
			v, err := money.ParseMinor(d.Amt, dec)
			if err != nil || v <= 0 {
				errMsg.Set(uistate.T("splitEditor.badAmount"))
				return
			}
			signed := v
			if props.Txn.Amount.IsNegative() {
				signed = -v
			}
			built = append(built, domain.CategorySplit{CategoryID: d.Cat, Amount: money.New(signed, props.Txn.Amount.Currency)})
			sum += v
		}
		if len(built) < 2 {
			errMsg.Set(uistate.T("splitEditor.needTwo"))
			return
		}
		if sum != txnAbs {
			errMsg.Set(uistate.T("splitEditor.mustBalance"))
			return
		}
		t := props.Txn
		t.Splits = built
		errMsg.Set("")
		if props.OnSave != nil {
			props.OnSave(t)
		}
	})

	clear := ui.UseEvent(func() {
		t := props.Txn
		t.Splits = nil
		errMsg.Set("")
		if props.OnSave != nil {
			props.OnSave(t)
		}
	})

	catOpts := uiw.OptionsFrom(props.Categories,
		func(c domain.Category) string { return c.ID },
		func(c domain.Category) string { return c.Name },
		"")
	catOpts = append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}}, catOpts...)

	var rows []ui.Node
	for i, d := range splits.Get() {
		rows = append(rows, ui.CreateElement(splitRow, splitRowProps{
			Index:    i,
			Cat:      d.Cat,
			Amt:      d.Amt,
			CatOpts:  catOpts,
			Dec:      dec,
			OnCat:    setCat,
			OnAmt:    setAmt,
			OnRemove: removeRow,
		}))
	}

	// Remainder phrasing: balanced (green), or "$X left"/"$X over" so the user knows
	// exactly what to adjust. Save is gated on a true balance, so this is the guide.
	remTone, remText := "pos", uistate.T("splitEditor.balanced")
	switch {
	case parseErr:
		remTone, remText = "neg", uistate.T("splitEditor.badAmount")
	case remainder > 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.left", money.FormatMinor(remainder, dec))
	case remainder < 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.over", money.FormatMinor(-remainder, dec))
	}

	return Div(css.Class("split-editor"), Attr("data-testid", "split-editor"),
		Style(map[string]string{"margin-top": "0.75rem", "padding": "0.75rem", "border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("hero-flanker-label"), Style(map[string]string{"margin-bottom": "0.4rem"}), uistate.T("splitEditor.title")),
		P(css.Class("muted"), Style(map[string]string{"margin-bottom": "0.5rem"}), uistate.T("splitEditor.hint", money.FormatMinor(txnAbs, dec))),
		Div(css.Class("split-rows"), rows),
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem", "align-items": "center", "flex-wrap": "wrap"}),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-add"), OnClick(addRow), uistate.T("splitEditor.add")),
			Span(ClassStr("hero-stat-sub "+remTone), Attr("data-testid", "split-remainder"), remText),
		),
		If(errMsg.Get() != "", P(css.Class("muted", "neg"), Attr("role", "alert"), errMsg.Get())),
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem"}),
			Button(css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "split-save"), OnClick(save), uistate.T("splitEditor.save")),
			If(props.Txn.HasSplits(), Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-clear"), OnClick(clear), uistate.T("splitEditor.clear"))),
		),
	)
}

type splitRowProps struct {
	Index    int
	Cat      string
	Amt      string
	CatOpts  []uiw.SelectOption
	Dec      int
	OnCat    func(int, string)
	OnAmt    func(int, string)
	OnRemove func(int)
}

// splitRow is one editable split line (category + amount + remove). It is its own
// component so its OnChange/OnInput/OnClick hooks live at stable positions per row
// instead of inside the parent's variable-length loop (the framework gotcha).
func splitRow(props splitRowProps) ui.Node {
	onCat := func(v string) { props.OnCat(props.Index, v) }
	onAmt := ui.UseEvent(func(v string) { props.OnAmt(props.Index, v) })
	onRemove := ui.UseEvent(func() { props.OnRemove(props.Index) })
	return Div(css.Class("split-row"), Attr("data-testid", "split-row"),
		Style(map[string]string{"display": "flex", "gap": "0.5rem", "align-items": "center", "margin-bottom": "0.4rem"}),
		Div(Style(map[string]string{"flex": "1 1 auto"}),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.CatOpts,
				Selected:  props.Cat,
				AriaLabel: uistate.T("splitEditor.category"),
				TestID:    "split-cat-" + strconv.Itoa(props.Index),
				OnChange:  onCat,
			})),
		Input(css.Class("field"), Type("number"), Step("0.01"), Style(map[string]string{"max-width": "8rem"}),
			Attr("aria-label", uistate.T("splitEditor.amount")), Attr("data-testid", "split-amt-"+strconv.Itoa(props.Index)),
			Placeholder(uistate.T("splitEditor.amount")), Value(props.Amt), OnInput(onAmt)),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("splitEditor.remove")),
			Title(uistate.T("splitEditor.remove")), Attr("data-testid", "split-remove-"+strconv.Itoa(props.Index)),
			OnClick(onRemove), "✕"),
	)
}

// splitToggleKey picks the toggle-button label: open vs closed, and "Edit" vs
// "Split" depending on whether the transaction already has a breakdown.
func splitToggleKey(open, hasSplits bool) string {
	switch {
	case open:
		return "splitEditor.hide"
	case hasSplits:
		return "splitEditor.edit"
	default:
		return "splitEditor.toggle"
	}
}

// absMinor returns the absolute value of a minor-unit amount.
func absMinor(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

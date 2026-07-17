// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// splitDraft is one editable line of a transaction's category breakdown: a
// category id and the amount (major-unit string, sign-agnostic — the parent
// applies the transaction's sign on save).
type splitDraft struct {
	Cat string
	Amt string
	// Owner is the member id this line is attributed to (XC10), or "" for "same as
	// the transaction" — the parent applies the fallback to the txn's payer.
	Owner string
}

// SplitModalFormID is the id shared by the split modal's body <form> and the
// FlipPanel footer's Save (type=submit form=…) button, so the pinned footer drives
// the editor's save. Only used by the modal host (TxnSplitHost); the inline uses
// leave FooterFormID empty and keep their own buttons.
const SplitModalFormID = "txn-split-form"

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
	// FooterFormID, when set, renders the editor as a modal body <form> with that id
	// (submitted by the FlipPanel's pinned Save footer) and drops the editor's own
	// title, card border, and inline Save button — the modal chrome supplies them.
	// Empty (the default) keeps the self-contained inline layout for the edit-form
	// and classic-table uses.
	FooterFormID string
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
				out = append(out, splitDraft{Cat: s.CategoryID, Amt: money.FormatMinor(absMinor(s.Amount.Amount), dec), Owner: s.MemberID})
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
	// Entry mode: exact amounts (the default) or percentages of the whole. Rows
	// keep one Amt string; the mode decides how it is read. Percentages are
	// fixed-point basis points (split.PercentScale), never floats.
	pctMode := ui.UseState(false)

	txnAbsEarly := absMinor(props.Txn.Amount.Amount)
	// Switching modes converts each parseable row in place — amounts become the
	// percentage they represent, percentages become their share of the total — so
	// the draft carries over instead of resetting.
	toAmounts := ui.UseEvent(func() {
		if !pctMode.Get() {
			return
		}
		cur := append([]splitDraft(nil), splits.Get()...)
		for i, d := range cur {
			bp, err := money.ParseMinor(d.Amt, 2)
			if err != nil || bp <= 0 || txnAbsEarly == 0 {
				continue
			}
			amt := txnAbsEarly * bp / split.PercentScale
			cur[i].Amt = money.FormatMinor(amt, dec)
		}
		splits.Set(cur)
		pctMode.Set(false)
	})
	toPercents := ui.UseEvent(func() {
		if pctMode.Get() {
			return
		}
		cur := append([]splitDraft(nil), splits.Get()...)
		for i, d := range cur {
			v, err := money.ParseMinor(d.Amt, dec)
			if err != nil || v <= 0 || txnAbsEarly == 0 {
				continue
			}
			bp := (absMinor(v)*split.PercentScale + txnAbsEarly/2) / txnAbsEarly
			cur[i].Amt = money.FormatMinor(bp, 2)
		}
		splits.Set(cur)
		pctMode.Set(true)
	})

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
	setOwner := func(i int, v string) {
		cur := append([]splitDraft(nil), splits.Get()...)
		if i >= 0 && i < len(cur) {
			cur[i].Owner = v
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

	// Live total of the entered split values and the target they must reach —
	// minor units against the transaction amount in amounts mode, basis points
	// against 100% in percent mode — so the remainder line tells the user how
	// much is still unallocated before they can save.
	inPct := pctMode.Get()
	valDec := dec
	if inPct {
		valDec = 2 // percent entries: two decimals = basis points
	}
	var total int64
	parseErr := false
	for _, d := range splits.Get() {
		if d.Amt == "" {
			continue
		}
		v, err := money.ParseMinor(d.Amt, valDec)
		if err != nil {
			parseErr = true
			continue
		}
		total += absMinor(v)
	}
	txnAbs := absMinor(props.Txn.Amount.Amount)
	target := txnAbs
	if inPct {
		target = split.PercentScale
	}
	remainder := target - total

	save := ui.UseEvent(Prevent(func() {
		cur := splits.Get()
		type line struct {
			cat, owner string
			val        int64 // minor units (amounts mode) or basis points (percent mode)
		}
		lines := make([]line, 0, len(cur))
		usePct := pctMode.Get()
		parseDec := dec
		if usePct {
			parseDec = 2
		}
		for _, d := range cur {
			if d.Cat == "" || d.Amt == "" {
				continue
			}
			v, err := money.ParseMinor(d.Amt, parseDec)
			if err != nil || v <= 0 {
				if usePct {
					errMsg.Set(uistate.T("splitEditor.badPercent"))
				} else {
					errMsg.Set(uistate.T("splitEditor.badAmount"))
				}
				return
			}
			lines = append(lines, line{cat: d.Cat, owner: d.Owner, val: v})
		}
		if len(lines) < 2 {
			errMsg.Set(uistate.T("splitEditor.needTwo"))
			return
		}
		amounts := make([]int64, len(lines))
		if usePct {
			bps := make([]int64, len(lines))
			for i, l := range lines {
				bps[i] = l.val
			}
			shares, err := split.ByPercents(txnAbs, bps)
			if err != nil {
				errMsg.Set(uistate.T("splitEditor.pctMustBalance"))
				return
			}
			for _, s := range shares {
				if s == 0 {
					errMsg.Set(uistate.T("splitEditor.pctTooSmall"))
					return
				}
			}
			amounts = shares
		} else {
			var sum int64
			for i, l := range lines {
				amounts[i] = l.val
				sum += l.val
			}
			if sum != txnAbs {
				errMsg.Set(uistate.T("splitEditor.mustBalance"))
				return
			}
		}
		built := make([]domain.CategorySplit, 0, len(lines))
		for i, l := range lines {
			signed := amounts[i]
			if props.Txn.Amount.IsNegative() {
				signed = -signed
			}
			built = append(built, domain.CategorySplit{CategoryID: l.cat, Amount: money.New(signed, props.Txn.Amount.Currency), MemberID: l.owner})
		}
		t := props.Txn
		t.Splits = built
		errMsg.Set("")
		if props.OnSave != nil {
			props.OnSave(t)
		}
	}))

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

	// Owner picker options (XC10): only when the household actually has members.
	// The first option is "Same as transaction" (empty value → falls back to the
	// txn's payer at attribution time).
	var ownerOpts []uiw.SelectOption
	showOwner := len(props.Members) > 0
	if showOwner {
		ownerOpts = uiw.OptionsFrom(props.Members,
			func(m domain.Member) string { return m.ID },
			func(m domain.Member) string { return m.Name },
			"")
		ownerOpts = append([]uiw.SelectOption{{Value: "", Label: uistate.T("splitEditor.ownerSameAsTxn")}}, ownerOpts...)
	}

	var rows []ui.Node
	for i, d := range splits.Get() {
		rows = append(rows, ui.CreateElement(splitRow, splitRowProps{
			Index:     i,
			Cat:       d.Cat,
			Amt:       d.Amt,
			Owner:     d.Owner,
			CatOpts:   catOpts,
			OwnerOpts: ownerOpts,
			ShowOwner: showOwner,
			Dec:       dec,
			Percent:   inPct,
			OnCat:     setCat,
			OnAmt:     setAmt,
			OnOwner:   setOwner,
			OnRemove:  removeRow,
		}))
	}

	// Remainder phrasing: balanced (green), or "$X left"/"$X over" — "X% left" in
	// percent mode — so the user knows exactly what to adjust. Save is gated on a
	// true balance, so this is the guide.
	remTone, remText := "pos", uistate.T("splitEditor.balanced")
	switch {
	case parseErr && inPct:
		remTone, remText = "neg", uistate.T("splitEditor.badPercent")
	case parseErr:
		remTone, remText = "neg", uistate.T("splitEditor.badAmount")
	case remainder > 0 && inPct:
		remTone = "neg"
		remText = uistate.T("splitEditor.pctLeft", money.FormatMinor(remainder, 2))
	case remainder > 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.left", money.FormatMinor(remainder, dec))
	case remainder < 0 && inPct:
		remTone = "neg"
		remText = uistate.T("splitEditor.pctOver", money.FormatMinor(-remainder, 2))
	case remainder < 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.over", money.FormatMinor(-remainder, dec))
	}

	// The shared body: the hint, the split rows, and the "Add split" + live-remainder
	// line, plus any validation error. Both layouts render these.
	hint := P(css.Class("muted"), Style(map[string]string{"margin-bottom": "0.5rem"}),
		uistate.T("splitEditor.hint", fmtMoney(money.New(txnAbs, props.Txn.Amount.Currency))))
	// Amounts / Percentages entry-mode toggle (aria-pressed segmented pair).
	modeBtn := func(testid, key string, active bool, on ui.Handler) ui.Node {
		cls := "btn btn-sm"
		if active {
			cls += " btn-primary"
		}
		return Button(ClassStr(cls), Type("button"), Attr("data-testid", testid),
			Attr("aria-pressed", ariaBool(active)), OnClick(on), uistate.T(key))
	}
	modeToggle := Div(Style(map[string]string{"display": "flex", "gap": "0.35rem", "margin-bottom": "0.5rem"}),
		modeBtn("split-mode-amount", "splitEditor.modeAmounts", !inPct, toAmounts),
		modeBtn("split-mode-percent", "splitEditor.modePercents", inPct, toPercents),
	)
	rowsNode := Div(css.Class("split-rows"), rows)
	addRow2 := Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem", "align-items": "center", "flex-wrap": "wrap"}),
		Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-add"), OnClick(addRow), uistate.T("splitEditor.add")),
		Span(ClassStr("hero-stat-sub "+remTone), Attr("data-testid", "split-remainder"), remText),
	)
	errNode := If(errMsg.Get() != "", P(css.Class("muted", "neg"), Attr("role", "alert"), errMsg.Get()))

	// Modal layout: a body <form> whose id the FlipPanel's pinned Save footer submits.
	// No inner title/border (the panel's chrome supplies them) and no inline Save
	// button (the footer owns it); "Clear split" stays as a quiet body action since
	// the standard footer is only Cancel + Save.
	if props.FooterFormID != "" {
		return Form(css.Class("split-editor split-editor-modal"), Attr("id", props.FooterFormID),
			Attr("data-testid", "split-editor"), OnSubmit(save),
			hint,
			modeToggle,
			rowsNode,
			addRow2,
			errNode,
			If(props.Txn.HasSplits(),
				Div(css.Class("split-editor-clear"),
					Style(map[string]string{"margin-top": "0.75rem", "padding-top": "0.6rem", "border-top": "1px solid var(--border)"}),
					Button(css.Class("btn", "btn-sm", "btn-ghost"), Type("button"), Attr("data-testid", "split-clear"),
						OnClick(clear), uistate.T("splitEditor.clear")))),
		)
	}

	// Inline layout (edit-form / classic table): the self-contained bordered card with
	// its own title and Save/Clear buttons.
	return Div(css.Class("split-editor"), Attr("data-testid", "split-editor"),
		Style(map[string]string{"margin-top": "0.75rem", "padding": "0.75rem", "border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("hero-flanker-label"), Style(map[string]string{"margin-bottom": "0.4rem"}), uistate.T("splitEditor.title")),
		hint,
		modeToggle,
		rowsNode,
		addRow2,
		errNode,
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem"}),
			Button(css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "split-save"), OnClick(save), uistate.T("splitEditor.save")),
			If(props.Txn.HasSplits(), Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-clear"), OnClick(clear), uistate.T("splitEditor.clear"))),
		),
	)
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
	Percent  bool
	OnCat    func(int, string)
	OnAmt    func(int, string)
	OnOwner  func(int, string)
	OnRemove func(int)
}

// splitRow is one editable split line (category + amount + remove). It is its own
// component so its OnChange/OnInput/OnClick hooks live at stable positions per row
// instead of inside the parent's variable-length loop (the framework gotcha).
func splitRow(props splitRowProps) ui.Node {
	onCat := func(v string) { props.OnCat(props.Index, v) }
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
	return Div(css.Class("split-row"), Attr("data-testid", "split-row"),
		Style(map[string]string{"display": "flex", "gap": "0.5rem", "align-items": "center", "margin-bottom": "0.4rem", "flex-wrap": "wrap"}),
		Div(Style(map[string]string{"flex": "1 1 auto"}),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.CatOpts,
				Selected:  props.Cat,
				AriaLabel: uistate.T("splitEditor.category"),
				TestID:    "split-cat-" + strconv.Itoa(props.Index),
				OnChange:  onCat,
			})),
		// XC10: optional per-line owner. Only rendered for multi-member households;
		// "Same as transaction" (empty) keeps the pre-XC10 payer attribution.
		If(props.ShowOwner, Div(Style(map[string]string{"flex": "1 1 auto"}),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.OwnerOpts,
				Selected:  props.Owner,
				AriaLabel: uistate.T("splitEditor.owner"),
				TestID:    "split-owner-" + strconv.Itoa(props.Index),
				OnChange:  onOwner,
			}))),
		Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"), Style(map[string]string{"max-width": "8rem"}),
			Attr("aria-label", uistate.T(splitAmtKey(props.Percent))), Attr("data-testid", "split-amt-"+strconv.Itoa(props.Index)),
			Placeholder(uistate.T(splitAmtKey(props.Percent))), Value(props.Amt), OnInput(onAmt), OnBlur(onAmtBlur)),
		If(props.Percent, Span(css.Class("muted"), Attr("aria-hidden", "true"), "%")),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("splitEditor.remove")),
			Title(uistate.T("splitEditor.remove")), Attr("data-testid", "split-remove-"+strconv.Itoa(props.Index)),
			OnClick(onRemove), "✕"),
	)
}

// splitAmtKey picks the value input's label: percent-mode rows read "Percent",
// amounts-mode rows read "Amount".
func splitAmtKey(percent bool) string {
	if percent {
		return "splitEditor.percent"
	}
	return "splitEditor.amount"
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

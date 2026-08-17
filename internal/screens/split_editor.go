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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
	//
	// C632: converting each row on its own with plain integer division truncates
	// PER ROW, so a draft that summed exactly to the parent before the toggle was
	// short by a cent or more after it — the editor told the user their split no
	// longer balanced when the only thing they had changed was the display mode.
	// When the draft IS exactly balanced in the source mode, convert the whole set
	// together through the largest-remainder helper (the same Hamilton
	// apportionment split.ByPercents/ByWeights use), which sums to the destination
	// target exactly. An already-unbalanced draft has no exact answer to preserve,
	// so it keeps the cheap per-row conversion and stays unbalanced.
	convertRows := func(cur []splitDraft, srcDec int, srcTotal int64, dstDec int, dstTotal int64) {
		// Read every row that carries a parseable positive value; blanks are left
		// alone, and anything unparseable means the draft cannot be balanced.
		idx := make([]int, 0, len(cur))
		weights := make([]split.WeightedMember, 0, len(cur))
		var sum int64
		exact := true
		for i, d := range cur {
			if d.Amt == "" {
				continue
			}
			v, err := money.ParseMinor(d.Amt, srcDec)
			if err != nil || v <= 0 {
				exact = false
				continue
			}
			idx = append(idx, i)
			weights = append(weights, split.WeightedMember{MemberID: strconv.Itoa(i), Weight: absMinor(v)})
			sum += absMinor(v)
		}
		if len(idx) == 0 || srcTotal == 0 {
			return
		}
		if exact && sum == srcTotal {
			if shares := split.ByWeights(dstTotal, weights); shares != nil {
				for k, i := range idx {
					cur[i].Amt = money.FormatMinor(shares[k].Amount, dstDec)
				}
				return
			}
		}
		// Fallback: proportional per-row conversion for a draft that does not
		// balance, so the numbers still carry over between modes.
		for k, i := range idx {
			cur[i].Amt = money.FormatMinor((weights[k].Weight*dstTotal+srcTotal/2)/srcTotal, dstDec)
		}
	}
	toAmounts := ui.UseEvent(func() {
		if !pctMode.Get() || txnAbsEarly == 0 {
			return
		}
		cur := append([]splitDraft(nil), splits.Get()...)
		convertRows(cur, 2, split.PercentScale, dec, txnAbsEarly)
		splits.Set(cur)
		pctMode.Set(false)
	})
	toPercents := ui.UseEvent(func() {
		if pctMode.Get() || txnAbsEarly == 0 {
			return
		}
		cur := append([]splitDraft(nil), splits.Get()...)
		convertRows(cur, dec, txnAbsEarly, 2, split.PercentScale)
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
		// C634: a split line has to be a positive share of the parent. Taking the
		// magnitude here let a negative entry ADD to the running total, so a draft
		// containing "-5" could reach the target and read "Balanced" with Save
		// enabled — and then fail on the click, because save rejects it. A value
		// that save will refuse must fail the same check the footer and the Save
		// button read, not a friendlier one.
		if v <= 0 {
			parseErr = true
			continue
		}
		total += v
	}
	txnAbs := absMinor(props.Txn.Amount.Amount)
	target := txnAbs
	if inPct {
		target = split.PercentScale
	}
	remainder := target - total

	// C566: a line is only a real split line when it carries BOTH a category and an
	// amount. The editor seeds a blank second line so "carve a piece out" is one tap
	// away, and the amount total ignored it — so the footer read "Balanced" and offered
	// Save while the split could not actually be saved (save then failed with "needs at
	// least two lines"). Both the footer text and the Save state read ONE verdict from
	// split.Classify/Saveable (pure, table-driven-tested), so they cannot disagree.
	draftLines := make([]split.Line, 0, len(splits.Get()))
	for _, d := range splits.Get() {
		draftLines = append(draftLines, split.Line{CategoryID: d.Cat, Value: d.Amt})
	}
	shape := split.Classify(draftLines)
	canSave := shape.Saveable(!parseErr, remainder == 0)

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
	// Registered unconditionally even though only the modal layout renders it: the
	// two layouts are branches of ONE component, so a hook created inside either
	// branch would shift the hook order between them (the GWC hooks rule).
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnCancel != nil {
			props.OnCancel()
		}
	}))

	// C570: qualified paths ("Housing > Mortgage"), not bare leaf names — a split is
	// exactly where two categories called "Gas" have to be told apart.
	catOpts := append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}},
		CategoryPickOptions(props.Categories)...)

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
	//
	// C566: the unfinished-line cases are tested FIRST, because they are the ones the
	// arithmetic cannot see. A blank line contributes nothing to the total, so without
	// these branches a half-written split reports itself as balanced.
	remTone, remText := "pos", uistate.T("splitEditor.balanced")
	switch {
	case parseErr && inPct:
		remTone, remText = "neg", uistate.T("splitEditor.badPercent")
	case parseErr:
		remTone, remText = "neg", uistate.T("splitEditor.badAmount")
	case shape.Incomplete > 0:
		remTone, remText = "neg", uistate.T("splitEditor.incomplete")
	case remainder == 0 && shape.Complete < split.MinSplitLines:
		remTone, remText = "neg", uistate.T("splitEditor.needTwoShort")
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
		Span(ClassStr("hero-stat-sub "+remTone), Attr("data-testid", "split-remainder"), Attr("role", "status"), remText),
	)
	// The seeded blank line is named as the draft it is, so it reads as "yours to fill
	// in" rather than as a row the editor forgot about.
	draftNote := If(shape.Blank > 0 && shape.Incomplete == 0,
		P(css.Class("muted", tw.Text13), Attr("data-testid", "split-draft-note"),
			Style(map[string]string{"margin": "0.35rem 0 0"}), uistate.T("splitEditor.draftRow")))
	errNode := If(errMsg.Get() != "", P(css.Class("muted", "neg"), Attr("role", "alert"), errMsg.Get()))

	// Modal layout: a body <form> whose id the FlipPanel's pinned Save footer submits.
	// No inner title/border (the panel's chrome supplies them) and no inline Save
	// button (the footer owns it); "Clear split" stays as a quiet body action since
	// the standard footer is only Cancel + Save.
	if props.FooterFormID != "" {
		modalSaveArgs := []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "split-save"), uistate.T("splitEditor.save")}
		if !canSave {
			modalSaveArgs = append(modalSaveArgs, Disabled(true), Attr("aria-disabled", "true"),
				Title(uistate.T("splitEditor.cannotSaveYet")))
		}
		return Form(css.Class("split-editor split-editor-modal"), Attr("id", props.FooterFormID),
			Attr("data-testid", "split-editor"), OnSubmit(save),
			Div(css.Class("modal-scroll"),
				hint,
				modeToggle,
				rowsNode,
				addRow2,
				draftNote,
				errNode,
				If(props.Txn.HasSplits(),
					Div(css.Class("split-editor-clear"),
						Style(map[string]string{"margin-top": "0.75rem", "padding-top": "0.6rem", "border-top": "1px solid var(--border)"}),
						Button(css.Class("btn", "btn-sm", "btn-ghost"), Type("button"), Attr("data-testid", "split-clear"),
							OnClick(clear), uistate.T("splitEditor.clear"))))),
			Div(css.Class("modal-sticky-foot"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "split-cancel"),
					OnClick(cancel), uistate.T("action.cancel")),
				Button(modalSaveArgs...)),
		)
	}

	// Inline layout (edit-form / classic table): the self-contained bordered card with
	// its own title and Save/Clear buttons.
	saveArgs := []any{css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "split-save"), OnClick(save), uistate.T("splitEditor.save")}
	if !canSave {
		saveArgs = append(saveArgs, Disabled(true), Attr("aria-disabled", "true"),
			Title(uistate.T("splitEditor.cannotSaveYet")))
	}
	return Div(css.Class("split-editor"), Attr("data-testid", "split-editor"),
		Style(map[string]string{"margin-top": "0.75rem", "padding": "0.75rem", "border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("hero-flanker-label"), Style(map[string]string{"margin-bottom": "0.4rem"}), uistate.T("splitEditor.title")),
		hint,
		modeToggle,
		rowsNode,
		addRow2,
		draftNote,
		errNode,
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem"}),
			Button(saveArgs...),
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
	// A hairline + padding under each row visually separates the splits so multiple category
	// breakdowns don't blur together (coworker feedback).
	return Div(css.Class("split-row"), Attr("data-testid", "split-row"),
		Style(map[string]string{"display": "flex", "gap": "0.5rem", "align-items": "center", "padding-bottom": "0.5rem", "margin-bottom": "0.5rem", "border-bottom": "1px solid var(--border)", "flex-wrap": "wrap"}),
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
			Placeholder(uistate.T(splitAmtKey(props.Percent))), OnInput(onAmt), OnBlur(onAmtBlur), uiw.FieldValue(props.Amt)),
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

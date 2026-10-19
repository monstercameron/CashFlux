// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
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
			{Cat: props.Txn.CategoryID, Amt: ""},
			{Cat: "", Amt: ""},
		}
	}

	splits := ui.UseState(seed())
	pickerLine := ui.UseState(-1)
	_ = uistate.UseDataRevision().Get()
	errMsg := ui.UseState("")
	saveAttempted := ui.UseState(false)
	// Entry mode: exact amounts (the default) or percentages of the whole. Rows
	// keep one Amt string; the mode decides how it is read. Percentages are
	// fixed-point basis points (split.PercentScale), never floats.
	pctMode := ui.UseState(false)
	draftGeneration := ui.UseState(props.DraftGeneration)
	ui.UseEffect(func() func() {
		if draftGeneration.Get() != props.DraftGeneration {
			splits.Set(seed())
			errMsg.Set("")
			pctMode.Set(false)
			draftGeneration.Set(props.DraftGeneration)
		}
		return nil
	}, props.DraftGeneration)

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
	openCategoryPicker := func(i int) { pickerLine.Set(i) }
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
	draftLines := make([]split.Line, 0, len(splits.Get()))
	for _, d := range splits.Get() {
		draftLines = append(draftLines, split.Line{CategoryID: d.Cat, Value: d.Amt})
	}
	shape := split.Classify(draftLines)
	canSave := shape.Saveable(!parseErr, remainder == 0)

	save := ui.UseEvent(Prevent(func() {
		saveAttempted.Set(true)
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
	categories := props.Categories
	if app := appstate.Default; app != nil {
		categories = app.Categories()
	}
	catOpts := append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}},
		CategoryPickOptions(categoriesForKind(categories, splitCategoryKind(props.Txn)))...)

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
			Index:         i,
			Cat:           d.Cat,
			Amt:           d.Amt,
			Owner:         d.Owner,
			CatOpts:       catOpts,
			OwnerOpts:     ownerOpts,
			ShowOwner:     showOwner,
			Dec:           dec,
			Percent:       inPct,
			OnCat:         setCat,
			OnNewCategory: openCategoryPicker,
			OnAmt:         setAmt,
			OnOwner:       setOwner,
			OnRemove:      removeRow,
		}))
	}
	pickerNode := Fragment()
	if i := pickerLine.Get(); i >= 0 && i < len(splits.Get()) {
		kind := splitCategoryKind(props.Txn)
		pickerNode = ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind: kind, Selected: splits.Get()[i].Cat,
			OnPick:  func(categoryID string) { setCat(i, categoryID); pickerLine.Set(-1) },
			OnClose: func() { pickerLine.Set(-1) },
		})
	}

	// Remainder phrasing stays arithmetic while the user edits. Validation details
	// appear only after Save is attempted.
	//
	// A blank seeded line is a draft, not an error. Show the arithmetic remainder
	// while the user is editing; validation messages appear only after Save.
	remTone, remText := "pos", uistate.T("splitEditor.balanced")
	switch {
	case saveAttempted.Get() && parseErr && inPct:
		remTone, remText = "neg", uistate.T("splitEditor.badPercent")
	case saveAttempted.Get() && parseErr:
		remTone, remText = "neg", uistate.T("splitEditor.badAmount")
	case saveAttempted.Get() && remainder == 0 && !canSave && shape.Incomplete > 0:
		remTone, remText = "neg", uistate.T("splitEditor.incomplete")
	case saveAttempted.Get() && remainder == 0 && !canSave:
		remTone, remText = "neg", uistate.T("splitEditor.needTwoShort")
	case remainder > 0 && inPct:
		remTone = "neg"
		remText = uistate.T("splitEditor.pctLeft", money.FormatMinor(remainder, 2))
	case remainder > 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.left", fmtMoney(money.New(remainder, props.Txn.Amount.Currency)))
	case remainder < 0 && inPct:
		remTone = "neg"
		remText = uistate.T("splitEditor.pctOver", money.FormatMinor(-remainder, 2))
	case remainder < 0:
		remTone = "neg"
		remText = uistate.T("splitEditor.over", fmtMoney(money.New(-remainder, props.Txn.Amount.Currency)))
	}

	// The shared body: the hint, the split rows, and the "Add split" + live-remainder
	// line, plus any validation error. Both layouts render these.
	hint := P(css.Class("muted"), Style(map[string]string{"margin-bottom": "0.5rem"}),
		uistate.T("splitEditor.hint", fmtMoney(money.New(txnAbs, props.Txn.Amount.Currency))))
	receiptSplit := ui.UseEvent(func() {
		if props.OnReceiptSplit != nil {
			props.OnReceiptSplit(props.Txn.ID)
		}
	})
	receiptAction := If(props.OnReceiptSplit != nil,
		Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "txn-receipt-split-open"),
			OnClick(receiptSplit), uistate.T("receiptsplit.menuAction")))
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
	errNode := If(errMsg.Get() != "", P(css.Class("muted", "neg"), Attr("role", "alert"), errMsg.Get()))

	// Modal layout: a body <form> whose id the FlipPanel's pinned Save footer submits.
	// No inner title/border (the panel's chrome supplies them) and no inline Save
	// button (the footer owns it); "Clear split" stays as a quiet body action since
	// the standard footer is only Cancel + Save.
	if props.FooterFormID != "" {
		modalSaveArgs := []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "split-save"), uistate.T("splitEditor.save")}
		return Fragment(Form(css.Class("split-editor split-editor-modal"), Attr("id", props.FooterFormID),
			Attr("data-testid", "split-editor"), OnSubmit(save),
			Div(css.Class("modal-scroll"),
				hint,
				receiptAction,
				modeToggle,
				splitColumnLabels(showOwner),
				rowsNode,
				addRow2,
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
		), pickerNode)
	}

	// Inline layout (edit-form / classic table): the self-contained bordered card with
	// its own title and Save/Clear buttons.
	saveArgs := []any{css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "split-save"), OnClick(save), uistate.T("splitEditor.save")}
	return Fragment(Div(css.Class("split-editor"), Attr("data-testid", "split-editor"),
		Style(map[string]string{"margin-top": "0.75rem", "padding": "0.75rem", "border": "1px solid var(--border)", "border-radius": "8px"}),
		P(css.Class("hero-flanker-label"), Style(map[string]string{"margin-bottom": "0.4rem", "text-align": "left"}), uistate.T("splitEditor.title")),
		hint,
		receiptAction,
		modeToggle,
		splitColumnLabels(showOwner),
		rowsNode,
		addRow2,
		errNode,
		Div(Style(map[string]string{"margin-top": "0.5rem", "display": "flex", "gap": "0.5rem"}),
			Button(saveArgs...),
			If(props.Txn.HasSplits(), Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "split-clear"), OnClick(clear), uistate.T("splitEditor.clear"))),
		),
	), pickerNode)
}

func splitColumnLabels(showOwner bool) ui.Node {
	owner := Fragment()
	if showOwner {
		owner = P(Attr("data-testid", "split-label-who"), uistate.T("splitEditor.ownerColumn"))
	}
	return Div(css.Class("split-column-labels"), Attr("data-testid", "split-labels"),
		Style(map[string]string{"display": "grid", "grid-template-columns": splitColumns(showOwner), "gap": "0.5rem", "margin-bottom": "0.35rem", "font-size": "0.8rem", "font-weight": "600", "color": "var(--muted)"}),
		P(Attr("data-testid", "split-label-category"), uistate.T("splitEditor.category")),
		owner,
		P(Attr("data-testid", "split-label-amount"), uistate.T("splitEditor.amount")),
		Span(Attr("aria-hidden", "true")),
	)
}

func splitColumns(showOwner bool) string {
	if showOwner {
		return "minmax(0, 1fr) minmax(0, 1fr) 8rem 2rem"
	}
	return "minmax(0, 1fr) 8rem 2rem"
}

func splitCategoryKind(txn domain.Transaction) domain.CategoryKind {
	if txn.Amount.Amount > 0 {
		return domain.KindIncome
	}
	return domain.KindExpense
}

func categoriesForKind(categories []domain.Category, kind domain.CategoryKind) []domain.Category {
	filtered := make([]domain.Category, 0, len(categories))
	for _, category := range categories {
		if category.Kind == kind {
			filtered = append(filtered, category)
		}
	}
	return filtered
}

// splitCellLabel returns the compact category-cell label and the split amounts
// for its tooltip. The transaction row can keep its normal category display for
// unsplit transactions.
func splitCellLabel(tx domain.Transaction) (label, tooltip string) {
	if !tx.HasSplits() {
		return "", ""
	}
	categoryIDs := make(map[string]struct{}, len(tx.Splits))
	amounts := make([]string, 0, len(tx.Splits))
	for _, line := range tx.Splits {
		categoryIDs[line.CategoryID] = struct{}{}
		amounts = append(amounts, fmtMoney(line.Amount))
	}
	return uistate.T("splitEditor.cellLabel", len(categoryIDs)), strings.Join(amounts, ", ")
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
	onCat := func(v string) { props.OnCat(props.Index, v) }
	onNewCategory := ui.UseEvent(func() { props.OnNewCategory(props.Index) })
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
		Style(map[string]string{"display": "grid", "grid-template-columns": splitColumns(props.ShowOwner), "gap": "0.5rem", "align-items": "center", "padding-bottom": "0.5rem", "margin-bottom": "0.5rem", "border-bottom": "1px solid var(--border)"}),
		Div(Style(map[string]string{"min-width": "0", "display": "flex", "gap": "0.35rem", "align-items": "center"}),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.CatOpts,
				Selected:  props.Cat,
				AriaLabel: uistate.T("splitEditor.category"),
				TestID:    "split-cat-" + strconv.Itoa(props.Index),
				OnChange:  onCat,
			}),
			Button(css.Class("btn", "btn-tool", "txn-cat-new"), Type("button"),
				Attr("data-testid", "split-new-category-"+strconv.Itoa(props.Index)),
				Attr("aria-label", uistate.T("transactions.newCategory")), OnClick(onNewCategory), uistate.T("splitEditor.newCategory"))),
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
		Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"), Style(map[string]string{"width": "100%", "min-width": "0"}),
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

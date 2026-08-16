// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/catmerge"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// categoryMapGrid renders the at-a-glance "Category map" as a wrapping grid of
// group chips (a parent name with its sub-categories as pills) instead of the old
// mermaid flowchart. A flowchart laid every top-level category out as an isolated
// node, so dagre stacked them in a single tall column that wasted ~75% of the
// horizontal space; a wrapping grid fills the width and stays glanceable (GI2).
//
// Every chip is a jump link into the ledger below (C360). The sweep read the map
// as duplicating the list, and it was right while the chips were inert text: the
// same forty names, twice, with the second copy carrying all the information. As
// links they do a job the trees cannot — get you to one category in a taxonomy
// of forty without scrolling — and the duplication becomes navigation. Anchors,
// not buttons, deliberately: a per-chip click handler would be a hook inside a
// variable-length loop.
func categoryMapGrid(roots []categorytree.Node) ui.Node {
	if len(roots) == 0 {
		return Fragment()
	}
	groups := make([]any, 0, len(roots)+1)
	groups = append(groups, css.Class("cat-map"))
	for _, r := range roots {
		items := []any{css.Class("cat-map-group")}
		items = append(items, A(css.Class("cat-map-chip"), Href("#cat-row-"+r.Category.ID), r.Category.Name))
		for _, ch := range r.Children {
			items = append(items, A(css.Class("cat-map-sub"), Href("#cat-row-"+ch.Category.ID), ch.Category.Name))
			// one level of grandchildren keeps the map readable without nesting noise
			for _, gc := range ch.Children {
				items = append(items, A(css.Class("cat-map-sub", "cat-map-sub2"), Href("#cat-row-"+gc.Category.ID), gc.Category.Name))
			}
		}
		groups = append(groups, Div(items...))
	}
	return Div(groups...)
}

// Categories manages income and expense categories, presented in the
// Understand-surface language: a hero tile (this period's filed spending, the
// taxonomy figure chips, and a plain-English takeaway naming the leading
// category), the at-a-glance map, then the two tree ledgers whose rows carry
// this-period figures with category-tinted share bars. Add, inline edit,
// collapse, drill-to-transactions, and reassign-before-delete all preserved.
func Categories() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// The render body must READ this atom, not merely hold it: a state.UseAtom only
	// re-renders the components that read it during render. It was written but
	// never read, which made every bump() in this file a no-op — deleting a
	// category, reassigning one before deleting it, and (until this was found)
	// opening the merge panel all changed state that the page never repainted to
	// show. The page happened to look correct whenever some OTHER subscriber
	// (the data revision below) fired at the same moment, which is what made it
	// look intermittent rather than broken.
	rev := state.UseAtom("rev:categories", 0)
	_ = rev.Get()
	// Every mutation on this page must ALSO ask for a persist. Writing through the
	// store updates memory; RequestPersist is what puts it in the dataset. Without
	// it a deleted or merged category came back on the next reload, which looks
	// like the action silently failed rather than like a save that never happened.
	bump := func() {
		rev.Set(rev.Get() + 1)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}
	_ = uistate.UseDataRevision().Get()

	errMsg := ui.UseState("")
	// The move panel's selection lives in ATOMS, not component state. This page
	// remounts while its dataset settles (the first visit after a sample load is
	// the reliable way to see it), and a remount resets ui.UseState — so a panel
	// opened a moment too early vanished with no trace, which reads as a dead
	// button. Atoms survive the remount, which is why every other modal on this
	// page (the category editor, the Smart+ modal) is already driven by one.
	reassignID := state.UseAtom("categories:moveFrom", "") // awaiting reassignment before delete, or merge
	reassignTo := state.UseAtom("categories:moveTo", "")
	panelMergeAtom := state.UseAtom("categories:moveIsMerge", false)
	// Whether this panel was opened from the page (both pickers) or from a row
	// (target only). Kept so the source picker does not disappear the moment a
	// source is chosen, which both tore the element out of the DOM mid-interaction
	// and left no way to change your mind short of cancelling.
	pickedFromPage := state.UseAtom("categories:moveFromPage", false)
	// C523: merging reuses the reassign panel rather than adding a second
	// move-your-data UI. The only differences are the verb, the explanation, and
	// whether the source category survives — so the mode is one flag, not a fork.
	panelMerge := panelMergeAtom
	collapsed := ui.UseState(map[string]bool{}) // id → collapsed; session state
	sortByUsage := ui.UseState(false)           // sort-by-usage toggle (GI2)
	// In-context add (G17 §1): an "+ Add category" header button on each kind card,
	// so Tomás isn't forced to discover the command-palette / global "+".
	addCategory := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("category") }))
	toggleSort := ui.UseEvent(Prevent(func() { sortByUsage.Set(!sortByUsage.Get()) }))
	// Open the Smart+ categorization modal (shared with /transactions) — from here it
	// leads with the "Suggest new categories" scan, the natural fit for this page.
	smartCatOpen := uistate.UseTxnSmartCatOpen()
	openSmartCat := ui.UseEvent(Prevent(func() { smartCatOpen.Set(true) }))
	addCatBtn := func() ui.Node {
		return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "categories-add"), Title(uistate.T("categories.add")), OnClick(addCategory),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("categories.addCategory")))
	}
	smartCatBtn := func() ui.Node {
		return Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "categories-smartcat"), Title(uistate.T("smartcat.title")), OnClick(openSmartCat),
			smartGlyph(false, tw.Fold(tw.W4, tw.H4)),
			Span(uistate.T("categories.smartBtn")))
	}

	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })
	// C549: the name for a merge target that does not exist yet.
	mergeNewName := state.UseAtom("categories:mergeNewName", "")
	onMergeNewName := ui.UseEvent(func(e ui.Event) { mergeNewName.Set(e.GetValue()) })

	categoryUsage := func(catID string) int {
		used := 0
		for _, t := range app.Transactions() {
			if t.CategoryID == catID {
				used++
			}
		}
		for _, b := range app.Budgets() {
			if b.CategoryID == catID {
				used++
			}
		}
		return used
	}

	// txnByCat counts transactions per category in one pass, for the per-row usage
	// badge (C63). Budgets are excluded here: the badge drills into Transactions, so
	// it counts exactly the thing it links to.
	txnByCat := map[string]int{}
	for _, t := range app.Transactions() {
		txnByCat[t.CategoryID]++
	}
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTxns := func(catID string) {
		f := uistate.TxFilter{Category: catID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	// catNameNow resolves a category name from a FRESH store read. The closures
	// below run at click time, long after any snapshot taken during render.
	catNameNow := func(id string) string {
		for _, c := range app.Categories() {
			if c.ID == id {
				return c.Name
			}
		}
		return id
	}

	mergeCat := func(catID string) {
		pickedFromPage.Set(false)
		panelMerge.Set(true)
		reassignID.Set(catID)
		reassignTo.Set("")
		errMsg.Set("")
	}
	// Page-level merge: opens the same panel with no source chosen yet, so the
	// panel offers both pickers. Merging is a two-category act; choosing both in
	// one place reads better than starting from a row and hunting for the other.
	openMerge := ui.UseEvent(Prevent(func() {
		pickedFromPage.Set(true)
		panelMerge.Set(true)
		reassignID.Set(mergeAnySentinel)
		reassignTo.Set("")
		errMsg.Set("")
		bump()
	}))
	onMergeFrom := ui.UseEvent(func(e ui.Event) {
		reassignID.Set(e.GetValue())
		reassignTo.Set("")
	})
	mergeBtn := func() ui.Node {
		return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "categories-merge"), Title(uistate.T("categories.mergeMenuTitle")),
			OnClick(openMerge), Span(uistate.T("categories.mergeOpen")))
	}

	deleteCat := func(catID string) {
		// If in use, open the reassign panel instead of deleting; otherwise delete now.
		if categoryUsage(catID) > 0 {
			panelMerge.Set(false)
			reassignID.Set(catID)
			reassignTo.Set("")
			errMsg.Set("")
			return
		}
		if err := app.DeleteCategory(catID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	cancelReassign := ui.UseEvent(Prevent(func() { reassignID.Set("") }))
	confirmReassign := ui.UseEvent(Prevent(func() {
		from := reassignID.Get()
		to := reassignTo.Get()
		if from == mergeAnySentinel {
			errMsg.Set(uistate.T("categories.mergeChooseSource"))
			return
		}
		if to == "" || to == from {
			errMsg.Set(uistate.T("categories.pickDifferent"))
			return
		}
		// C549: merging into a category that does not exist yet. One call, so a
		// failed name cannot leave the data half-moved — the target is created
		// first and a rejected name aborts before anything merges.
		if panelMerge.Get() && to == mergeNewSentinel {
			fromName := catNameNow(from)
			name := strings.TrimSpace(mergeNewName.Get())
			_, counts, err := app.MergeCategoriesIntoNew(name, from)
			if err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.PostNotice(uistate.T("categories.mergedToast", fromName, name, counts.Total()), false)
			panelMerge.Set(false)
			reassignID.Set("")
			reassignTo.Set("")
			mergeNewName.Set("")
			errMsg.Set("")
			bump()
			return
		}
		if panelMerge.Get() {
			// Capture the names BEFORE the merge: the source stops existing as part
			// of it, so resolving afterwards yields a raw id in the confirmation.
			fromName, toName := catNameNow(from), catNameNow(to)
			// One call: the sweep, the retire and the count all come from the same
			// code that produced the preview, so what was promised is what happens.
			counts, err := app.MergeCategories(from, to)
			if err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.PostNotice(uistate.T("categories.mergedToast",
				fromName, toName, counts.Total()), false)
			panelMerge.Set(false)
			reassignID.Set("")
			errMsg.Set("")
			bump()
			return
		}
		if _, err := app.ReassignCategory(from, to); err != nil {
			errMsg.Set(err.Error())
			return
		}
		if err := app.DeleteCategory(from); err != nil {
			errMsg.Set(err.Error())
			return
		}
		reassignID.Set("")
		errMsg.Set("")
		bump()
	}))

	cats := app.Categories()
	var incomeList, expenseList []domain.Category
	catByID := make(map[string]domain.Category, len(cats))
	for _, c := range cats {
		catByID[c.ID] = c
		if c.Kind == domain.KindIncome {
			incomeList = append(incomeList, c)
		} else {
			expenseList = append(expenseList, c)
		}
	}
	// hasChildrenSet: set of category IDs that have at least one child in the full
	// category list, used to decide whether to show a collapse toggle.
	hasChildrenSet := make(map[string]bool, len(cats))
	for _, c := range cats {
		if c.ParentID != "" {
			hasChildrenSet[c.ParentID] = true
		}
	}

	toggleCollapse := func(id string) {
		cur := collapsed.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[id] = !cur[id]
		collapsed.Set(next)
	}

	// ── This period's figures — same computation paths as /reports. ────────────
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	periodStart, periodEnd := uistate.UsePeriod().Get().Range()
	pr := uistate.UsePrefs().Get()

	spendByCat := map[string]int64{}
	var totalSpend, unfiledSpend, maxSpend int64
	var topSpendID string
	if rows, err := reports.SpendingByCategory(app.Transactions(), periodStart, periodEnd, false, time.Time{}, time.Time{}, rates); err == nil {
		for _, r := range rows {
			spendByCat[r.CategoryID] = r.Amount
			totalSpend += r.Amount
			if r.CategoryID == "" {
				unfiledSpend = r.Amount
				continue
			}
			if r.Amount > maxSpend {
				maxSpend, topSpendID = r.Amount, r.CategoryID
			}
		}
	}
	incomeByCat := map[string]int64{}
	var maxIncome int64
	if rows, err := reports.IncomeByCategory(app.Transactions(), periodStart, periodEnd, rates); err == nil {
		for _, r := range rows {
			incomeByCat[r.CategoryID] = r.Amount
			if r.CategoryID != "" && r.Amount > maxIncome {
				maxIncome = r.Amount
			}
		}
	}

	deductibleCount := 0
	for _, c := range cats {
		if c.Deductible {
			deductibleCount++
		}
	}

	// ── Hero: filed spending, taxonomy chips, and the takeaway. ────────────────
	eyebrow := uistate.T("categories.countWord", len(cats)) + " · " +
		pr.FormatDate(periodStart) + " – " + pr.FormatDate(periodEnd)
	chips := []ui.Node{
		rptChip(uistate.T("categories.chipExpense"), fmt.Sprintf("%d", len(expenseList)), ""),
		rptChip(uistate.T("categories.chipIncome"), fmt.Sprintf("%d", len(incomeList)), ""),
	}
	if deductibleCount > 0 {
		chips = append(chips, rptChip(uistate.T("categories.chipDeduct"), fmt.Sprintf("%d", deductibleCount), ""))
	}
	if unfiledSpend > 0 {
		chips = append(chips, rptChip(uistate.T("categories.chipUnfiled"), fmtMoney(money.New(unfiledSpend, base)), rptToneCls("neg")))
	}

	takeaway := uistate.T("cats.quietTake")
	if totalSpend > 0 {
		if top, ok := catByID[topSpendID]; ok {
			takeaway = uistate.T("cats.leadTake", top.Name)
		} else {
			takeaway = ""
		}
		if unfiledSpend > 0 {
			takeaway = strings.TrimSpace(takeaway + " " + uistate.T("cats.unfiledClause", fmtMoney(money.New(unfiledSpend, base))))
		} else if takeaway != "" {
			takeaway = takeaway + " " + uistate.T("cats.filedClause")
		}
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-cats-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), eyebrow),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("categories.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), Attr("data-countup", ""), fmtMoney(money.New(totalSpend, base))),
			),
		),
		Div(css.Class("debt-chips"), chips),
		If(takeaway != "", P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "cats-takeaway"), takeaway)),
	)
	heroTile := rptTile("cats-hero", "1 / span 4", rptSection("", uistate.T("categories.heroTitle"), nil, heroBody))

	renderFlat := func(f categorytree.Flat) ui.Node {
		catID := f.Category.ID
		amt, hasAmt := spendByCat[catID], false
		maxAmt := maxSpend
		sub := uistate.T("categories.spentSub")
		if f.Category.Kind == domain.KindIncome {
			amt, maxAmt, sub = incomeByCat[catID], maxIncome, uistate.T("categories.earnedSub")
		}
		hasAmt = amt > 0
		pct := 0
		if maxAmt > 0 {
			pct = int(amt * 100 / maxAmt)
		}
		return ui.CreateElement(CategoryRow, categoryRowProps{
			Category:    f.Category,
			Depth:       f.Depth,
			TxnCount:    txnByCat[catID],
			HasChildren: hasChildrenSet[catID],
			Collapsed:   collapsed.Get()[catID],
			IsChild:     f.Depth > 0,
			IsZeroUsage: txnByCat[catID] == 0,
			Amount:      money.New(amt, base),
			AmountSub:   sub,
			HasAmount:   hasAmt,
			SharePct:    pct,
			OnView:      viewTxns,
			OnDelete:    deleteCat,
			OnMerge:     mergeCat,
			OnToggle:    toggleCollapse,
		})
	}
	// flattenSortedByUsage produces a flat list sorted by descending transaction
	// count (ties broken by name). Used when the sort-by-usage toggle is on.
	flattenSortedByUsage := func(list []domain.Category) []categorytree.Flat {
		flats := make([]categorytree.Flat, len(list))
		for i, c := range list {
			flats[i] = categorytree.Flat{Category: c, Depth: 0}
		}
		sort.SliceStable(flats, func(i, j int) bool {
			ci, cj := txnByCat[flats[i].Category.ID], txnByCat[flats[j].Category.ID]
			if ci != cj {
				return ci > cj
			}
			return flats[i].Category.Name < flats[j].Category.Name
		})
		return flats
	}
	// sortToggleBtn renders the sort-by-usage toggle in a section header (GI2).
	sortToggleBtn := func() ui.Node {
		label := "Sort by usage"
		if sortByUsage.Get() {
			label = "Sort: alphabetical"
		}
		return Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Title(label), OnClick(toggleSort), Span(label))
	}
	flatKey := func(f categorytree.Flat) any { return f.Category.ID }

	// Reassign-before-delete panel, shown when a used category is being deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		// fromPage: opened via the page-level Merge button rather than from a row,
		// so the panel owns the source choice for as long as it is open.
		fromPage := panelMerge.Get() && (rid == mergeAnySentinel || pickedFromPage.Get())
		pickingSource := rid == mergeAnySentinel
		target := catByID[rid]
		opts := []ui.Node{Option(Value(""), SelectedIf(reassignTo.Get() == ""), uistate.T("categories.chooseCategory"))}
		if panelMerge.Get() && !pickingSource {
			// Merge only. Reassign-on-delete is about finding an existing home for
			// data; offering to invent an empty one there would be a different
			// operation wearing the same button.
			opts = append(opts, Option(Value(mergeNewSentinel),
				SelectedIf(reassignTo.Get() == mergeNewSentinel), uistate.T("categories.mergeIntoNew")))
		}
		for _, c := range cats {
			// Only offer same-kind targets: reassigning an expense category's data to
			// an income category (or vice versa) is semantically wrong and a
			// data-integrity hazard (C63). Skip the category being retired. Until a
			// source is chosen there is no kind to match, so everything is offered.
			if c.ID == rid || (!pickingSource && c.Kind != target.Kind) {
				continue
			}
			opts = append(opts, Option(Value(c.ID), SelectedIf(reassignTo.Get() == c.ID), c.Name))
		}
		// Source picker, shown only when the panel was opened from the page rather
		// than from a specific category's row.
		var sourcePicker ui.Node = Fragment()
		if fromPage {
			sopts := []ui.Node{Option(Value(mergeAnySentinel), SelectedIf(pickingSource), uistate.T("categories.mergeChooseSource"))}
			for _, c := range cats {
				sopts = append(sopts, Option(Value(c.ID), SelectedIf(rid == c.ID), c.Name))
			}
			sourcePicker = Select(css.Class("field"), Attr("data-testid", "cats-merge-source"),
				Attr("aria-label", uistate.T("categories.mergeChooseSource")), OnChange(onMergeFrom), sopts)
		}
		merging := panelMerge.Get()
		title := uistate.T("common.reassignTitle")
		desc := uistate.T("categories.reassignDesc", target.Name, categoryUsage(rid))
		confirmLabel := uistate.T("common.moveAndDelete")
		if merging {
			title = uistate.T("categories.mergeTitle", target.Name)
			if pickingSource {
				title = uistate.T("categories.mergeTitleAny")
			}
			desc = uistate.T("categories.mergeDesc")
			confirmLabel = uistate.T("categories.mergeConfirm")
		}
		// The preview is computed by the SAME sweep that will run, so a promise of
		// "128 transactions" cannot be contradicted by an apply that moves 131.
		var preview ui.Node = Fragment()
		if merging && reassignTo.Get() != "" && reassignTo.Get() != mergeNewSentinel && !pickingSource {
			c := app.PlanCategoryMerge(rid, reassignTo.Get())
			preview = P(css.Class("muted", tw.Text13), Attr("data-testid", "cats-merge-preview"),
				uistate.T("categories.mergePreview", c.Total(), catByID[reassignTo.Get()].Name)+" "+
					mergePreviewParts(c))
		}
		reassignPanel = Div(css.Class("rpt-headsup", tw.Mb2), Attr("data-testid", "cats-move-panel"),
			Attr("data-mode", panelMode(merging)),
			H3(css.Class(tw.Mb1), title),
			P(css.Class("muted"), desc),
			sourcePicker,
			// The consequence is stated ABOVE the button that causes it. Below, it
			// is a receipt for a decision already made.
			preview,
			Form(css.Class("form-grid"), OnSubmit(confirmReassign),
				Select(css.Class("field"), Attr("aria-label", title), Attr("data-testid", "cats-move-target"), OnChange(onReassignTo), opts),
				// C549: name the target when it does not exist yet. Kind and parent
				// are inherited from the source, so there is nothing else to ask.
				If(reassignTo.Get() == mergeNewSentinel,
					Input(css.Class("field"), Type("text"), Value(mergeNewName.Get()),
						Attr("data-testid", "cats-merge-newname"),
						Attr("aria-label", uistate.T("categories.mergeNewNameLabel")),
						Placeholder(uistate.T("categories.mergeNewNamePlaceholder")),
						OnInput(onMergeNewName))),
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "cats-move-confirm"), confirmLabel),
				Button(css.Class("btn"), Type("button"), OnClick(cancelReassign), uistate.T("action.cancel")),
			),
		)
	}

	// Resolve the current flat lists once (respects sort-by-usage toggle).
	var expenseFlats, incomeFlats []categorytree.Flat
	if sortByUsage.Get() {
		expenseFlats = flattenSortedByUsage(expenseList)
		incomeFlats = flattenSortedByUsage(incomeList)
	} else {
		expenseFlats = visibleFlats(categorytree.Flatten(expenseList), categorytree.VisibleUnderCollapsed(expenseList, collapsed.Get()))
		incomeFlats = visibleFlats(categorytree.Flatten(incomeList), categorytree.VisibleUnderCollapsed(incomeList, collapsed.Get()))
	}

	// catTreeBody adapts a keyed tree list to a section body: the EmptyState CTA
	// when the kind has no categories yet, else the .rows ledger.
	catTreeBody := func(flats []categorytree.Flat, emptyMsg, emptyCTA string) ui.Node {
		if len(flats) == 0 {
			return ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: emptyMsg, CTALabel: emptyCTA, AddTarget: "category"})
		}
		return hhRowsList(MapKeyed(flats, flatKey, renderFlat))
	}

	tiles := []any{css.Class("bento bento-cats"),
		heroTile,
		If(reassignID.Get() != "", rptTile("cats-reassign", "1 / span 4", reassignPanel)),
		If(errMsg.Get() != "", rptTile("cats-err", "1 / span 4", P(css.Class("notice-danger"), errMsg.Get()))),
		// Visual category map (GI2): visible on arrival without scrolling past the
		// full expense/income ledgers (C70/C63 tree view).
		If(len(cats) > 0, rptTile("cats-map", "1 / span 4",
			rptSection("sec-cats-map", uistate.T("categories.mapTitle"), nil, Fragment(
				P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), uistate.T("categories.mapTake")),
				categoryMapGrid(categorytree.Build(cats)),
			)))),
		rptTile("cats-expense", "1 / span 4",
			rptSection("sec-cats-expense", uistate.T("categories.expenseTitle"),
				Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter), spendReportLink(), smartCatBtn(), mergeBtn(), sortToggleBtn(), addCatBtn()),
				catTreeBody(expenseFlats, uistate.T("categories.expenseEmpty"), uistate.T("categories.addFirstExpense")))),
		rptTile("cats-income", "1 / span 4",
			rptSection("sec-cats-income", uistate.T("categories.incomeTitle"),
				addCatBtn(),
				catTreeBody(incomeFlats, uistate.T("categories.incomeEmpty"), uistate.T("categories.addFirstIncome")))),
	}
	return Div(tiles...)
}

type categoryRowProps struct {
	Category    domain.Category
	Depth       int
	TxnCount    int  // transactions filed under this category
	HasChildren bool // true when this category has at least one child
	Collapsed   bool // true when this category's children are hidden
	IsChild     bool // true when depth > 0 (sub-category nesting cue, GI2)
	IsZeroUsage bool // true when TxnCount == 0 (dim treatment, GI2)
	// This-period figure: spend for expense categories, income for income ones.
	Amount    money.Money
	AmountSub string       // "spent this period" / "earned this period"
	HasAmount bool         // false hides the figure column (nothing this period)
	SharePct  int          // share of the largest same-kind category (0–100)
	OnView    func(string) // drill into Transactions filtered by category
	OnDelete  func(string)
	OnMerge   func(string)    // fold this category into another one
	OnToggle  func(id string) // toggle collapse/expand for this category
}

// visibleFlats filters a pre-flattened category list to only those entries whose
// IDs appear in the visible set (as produced by categorytree.VisibleUnderCollapsed).
// This keeps the filter logic out of the render closure while preserving the
// DFS pre-order produced by Flatten.
func visibleFlats(flats []categorytree.Flat, visible map[string]bool) []categorytree.Flat {
	out := make([]categorytree.Flat, 0, len(flats))
	for _, f := range flats {
		if visible[f.Category.ID] {
			out = append(out, f)
		}
	}
	return out
}

// CategoryRow is a per-category ledger row: swatch + collapse toggle + name
// (indented by depth) with the usage drill and deductible tag, a this-period
// figure with a category-tinted share bar, then a visible Edit and the ⋯ menu
// (view transactions / delete). Edit opens the shell-root flip modal
// (CategoryEditHost).
func CategoryRow(props categoryRowProps) ui.Node {
	c := props.Category
	del := ui.UseEvent(Prevent(func() { props.OnDelete(c.ID) }))
	merge := ui.UseEvent(Prevent(func() { props.OnMerge(c.ID) }))
	view := ui.UseEvent(func() {
		if props.OnView != nil {
			props.OnView(c.ID)
		}
	})
	toggle := ui.UseEvent(Prevent(func() {
		if props.OnToggle != nil {
			props.OnToggle(c.ID)
		}
	}))
	startEdit := ui.UseEvent(Prevent(func() { uistate.SetCategoryEdit(c.ID) }))

	// Sub-categories nest with real left padding (a guide line via border) rather
	// than literal "— " prefixes, for a cleaner hierarchy (C63). Depth 0 is flush.
	descStyle := map[string]string{}
	if props.Depth > 0 {
		descStyle["padding-left"] = uiw.IndentPx(props.Depth)
		descStyle["border-left"] = "2px solid var(--border, #2a2a2a)"
		descStyle["margin-left"] = "2px"
	}
	// Chevron toggle: shown for parent categories; a spacer aligns leaf rows.
	var toggleBtn ui.Node
	if props.HasChildren {
		chevronIcon := icon.ChevronDown
		if props.Collapsed {
			chevronIcon = icon.ChevronRight
		}
		ariaLabel := uistate.T("categories.collapseTitle", c.Name)
		if props.Collapsed {
			ariaLabel = uistate.T("categories.expandTitle", c.Name)
		}
		ariaExpanded := "true"
		if props.Collapsed {
			ariaExpanded = "false"
		}
		toggleBtn = Button(
			css.Class("btn", tw.ShrinkO),
			Type("button"),
			Attr("aria-label", ariaLabel),
			Attr("aria-expanded", ariaExpanded),
			Attr("data-testid", "cat-toggle-"+c.ID),
			OnClick(toggle),
			uiw.Icon(chevronIcon, css.Class(tw.W4, tw.H4)),
		)
	} else {
		// Spacer keeps name-column aligned with parent rows that do have a toggle.
		toggleBtn = Span(Style(map[string]string{"display": "inline-block", "width": "1.5rem", "flex-shrink": "0"}))
	}

	// The this-period share bar, tinted with the category's own color so the
	// ledger doubles as a legend for the charts that use these hues.
	var bar ui.Node = Fragment()
	if props.HasAmount && props.SharePct > 0 {
		bar = Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{
				"width":      fmt.Sprintf("%d%%", props.SharePct),
				"background": catColor(c.Color),
			})))
	}
	var figure ui.Node = Fragment()
	if props.HasAmount {
		figure = Div(css.Class("cat-figure"),
			Span(css.Class("amount"), fmtMoney(props.Amount)),
			Span(css.Class("cat-figure-sub"), props.AmountSub),
		)
	}

	// The usage drill + deductible tag as the quiet meta line.
	metaBits := []any{css.Class("row-meta")}
	if props.TxnCount > 0 {
		metaBits = append(metaBits, Button(css.Class("btn-link cat-usage"), Type("button"), Title(uistate.T("categories.viewTxnsTitle")), OnClick(view), Text(plural(props.TxnCount, "transaction"))))
	} else {
		metaBits = append(metaBits, Span(css.Class(tw.TextFaint), Text(uistate.T("categories.noTransactions"))))
	}
	if c.Deductible {
		metaBits = append(metaBits, Span(css.Class("cat-tag"), uistate.T("categories.deductTag")))
	}

	// The ⋯ overflow menu: view transactions + delete (reassign guard intact).
	menuItems := []ui.Node{}
	if props.TxnCount > 0 {
		menuItems = append(menuItems, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "cat-view-"+c.ID), OnClick(view), uistate.T("categories.viewTxnsTitle")))
	}
	// Merge sits above Delete and outside the danger group: it MOVES data rather
	// than losing it, which is the whole reason it exists as an alternative to
	// deleting a duplicate.
	menuItems = append(menuItems, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
		Attr("data-testid", "cat-merge-"+c.ID), Title(uistate.T("categories.mergeMenuTitle")),
		OnClick(merge), uistate.T("categories.mergeMenu")))
	menuItems = append(menuItems, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
		Attr("data-testid", "cat-delete-"+c.ID), Attr("aria-label", uistate.T("categories.deleteTitle")),
		Title(uistate.T("categories.deleteTitle")), OnClick(del), uistate.T("categories.deleteTitle")))

	// Build row class: base "row" + optional child/zero-usage modifiers (GI2).
	rowClass := "row"
	if props.IsChild {
		rowClass += " cat-child-row"
	}
	if props.IsZeroUsage {
		rowClass += " cat-zero-usage"
	}
	return Div(css.Class(rowClass),
		// C360: the anchor the category map's chips jump to. Shipped without this
		// in the first pass, so every chip was a link to an id that did not exist —
		// the map looked like navigation and did nothing.
		Attr("id", "cat-row-"+c.ID),
		Span(css.Class("cat-swatch"), Style(map[string]string{"background": catColor(c.Color)})),
		toggleBtn,
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), Style(descStyle), c.Name),
			Span(metaBits...),
			bar,
		),
		figure,
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("categories.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "cat-menu-" + c.ID,
			AriaLabel:    uistate.T("categories.menuAria"),
			ToggleTestID: "cat-menu-btn-" + c.ID,
			Items:        menuItems,
		}),
	)
}

// catColor returns a category's color, falling back to a neutral default when
// it has none set (older categories created before colors existed).
func catColor(c string) string {
	if strings.TrimSpace(c) == "" {
		return "#7c83ff"
	}
	return c
}

// panelMode is the data-mode the move panel exposes, so a test can tell a merge
// from a reassign-before-delete without reading copy.
func panelMode(merging bool) string {
	if merging {
		return "merge"
	}
	return "reassign"
}

// mergePreviewParts spells out WHAT moves, not just how much. "Moves 12 things"
// is not a number a person can check; "8 transactions, 2 budgets, 1 rule" is.
func mergePreviewParts(c catmerge.Counts) string {
	var parts []string
	add := func(n int, key string) {
		if n > 0 {
			parts = append(parts, uistate.T(key, n))
		}
	}
	add(c.Transactions, "categories.mergePartTxns")
	add(c.Splits, "categories.mergePartSplits")
	add(c.Budgets, "categories.mergePartBudgets")
	add(c.Goals, "categories.mergePartGoals")
	add(c.Rules, "categories.mergePartRules")
	add(c.Recurring, "categories.mergePartRecurring")
	add(c.Children, "categories.mergePartChildren")
	if len(parts) == 0 {
		return uistate.T("categories.mergePartNothing")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// mergeAnySentinel marks the merge panel as "opened from the page, no source
// chosen yet", which is what makes the source picker appear.
const mergeAnySentinel = "__pick__"

// mergeNewSentinel is the target-picker entry that means "make one" (C549).
// Cam's ask was "merge 2 categories into a new category"; the picker only
// offered targets that already existed, so the phrasing had no path at all.
const mergeNewSentinel = "__new__"

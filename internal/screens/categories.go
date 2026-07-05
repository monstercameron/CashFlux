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
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// categoryMapGrid renders the at-a-glance "Category map" as a wrapping grid of
// group chips (a parent name with its sub-categories as pills) instead of the old
// mermaid flowchart. A flowchart laid every top-level category out as an isolated
// node, so dagre stacked them in a single tall column that wasted ~75% of the
// horizontal space; a wrapping grid fills the width and stays glanceable (GI2).
func categoryMapGrid(roots []categorytree.Node) ui.Node {
	if len(roots) == 0 {
		return Fragment()
	}
	groups := make([]any, 0, len(roots)+1)
	groups = append(groups, css.Class("cat-map"))
	for _, r := range roots {
		items := []any{css.Class("cat-map-group")}
		items = append(items, Span(css.Class("cat-map-chip"), r.Category.Name))
		for _, ch := range r.Children {
			items = append(items, Span(css.Class("cat-map-sub"), ch.Category.Name))
			// one level of grandchildren keeps the map readable without nesting noise
			for _, gc := range ch.Children {
				items = append(items, Span(css.Class("cat-map-sub", "cat-map-sub2"), gc.Category.Name))
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

	rev := state.UseAtom("rev:categories", 0)
	bump := func() { rev.Set(rev.Get() + 1) }
	_ = uistate.UseDataRevision().Get()

	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // category awaiting reassignment before delete
	reassignTo := ui.UseState("")
	collapsed := ui.UseState(map[string]bool{}) // id → collapsed; session state
	sortByUsage := ui.UseState(false)           // sort-by-usage toggle (GI2)
	// In-context add (G17 §1): an "+ Add category" header button on each kind card,
	// so Tomás isn't forced to discover the command-palette / global "+".
	addCategory := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("category") }))
	toggleSort := ui.UseEvent(Prevent(func() { sortByUsage.Set(!sortByUsage.Get()) }))
	addCatBtn := func() ui.Node {
		return Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "categories-add"), Title(uistate.T("categories.add")), OnClick(addCategory),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("categories.addCategory")))
	}

	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })

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

	deleteCat := func(catID string) {
		// If in use, open the reassign panel instead of deleting; otherwise delete now.
		if categoryUsage(catID) > 0 {
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
		if to == "" || to == from {
			errMsg.Set(uistate.T("categories.pickDifferent"))
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
		target := catByID[rid]
		opts := []ui.Node{Option(Value(""), SelectedIf(reassignTo.Get() == ""), uistate.T("categories.chooseCategory"))}
		for _, c := range cats {
			// Only offer same-kind targets: reassigning an expense category's data to
			// an income category (or vice versa) is semantically wrong and a
			// data-integrity hazard (C63). Skip the category being deleted.
			if c.ID == rid || c.Kind != target.Kind {
				continue
			}
			opts = append(opts, Option(Value(c.ID), SelectedIf(reassignTo.Get() == c.ID), c.Name))
		}
		reassignPanel = Div(css.Class("rpt-headsup", tw.Mb2),
			H3(css.Class(tw.Mb1), uistate.T("common.reassignTitle")),
			P(css.Class("muted"), uistate.T("categories.reassignDesc", target.Name, categoryUsage(rid))),
			Form(css.Class("form-grid"), OnSubmit(confirmReassign),
				Select(css.Class("field"), Attr("aria-label", uistate.T("common.reassignTitle")), OnChange(onReassignTo), opts),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("common.moveAndDelete")),
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
				Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter), sortToggleBtn(), addCatBtn()),
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

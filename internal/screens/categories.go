//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Categories manages income and expense categories: add, list (grouped by kind),
// and per-row delete.
func Categories() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:categories", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

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
	saveCat := func(id, newName, kind, parent, color string, deductible bool) {
		for _, c := range app.Categories() {
			if c.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				c.Name = n
			}
			if k := domain.CategoryKind(kind); k.Valid() {
				c.Kind = k
			}
			c.ParentID = parent
			c.Color = color
			c.Deductible = deductible
			if err := app.PutCategory(c); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
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

	renderFlat := func(f categorytree.Flat) ui.Node {
		return ui.CreateElement(CategoryRow, categoryRowProps{
			Category:      f.Category,
			Depth:         f.Depth,
			AllCategories: cats,
			TxnCount:      txnByCat[f.Category.ID],
			HasChildren:   hasChildrenSet[f.Category.ID],
			Collapsed:     collapsed.Get()[f.Category.ID],
			IsChild:       f.Depth > 0,
			IsZeroUsage:   txnByCat[f.Category.ID] == 0,
			OnView:        viewTxns,
			OnDelete:      deleteCat,
			OnSave:        saveCat,
			OnToggle:      toggleCollapse,
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
	// sortToggleBtn renders the sort-by-usage toggle in a card header (GI2).
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
		reassignPanel = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("common.reassignTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("categories.reassignDesc", target.Name, categoryUsage(rid))),
				Form(css.Class("form-grid"), OnSubmit(confirmReassign),
					Select(css.Class("field"), Attr("aria-label", uistate.T("common.reassignTitle")), OnChange(onReassignTo), opts),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("common.moveAndDelete")),
					Button(css.Class("btn"), Type("button"), OnClick(cancelReassign), uistate.T("action.cancel")),
				),
			),
		})
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

	return Div(
		reassignPanel,
		// Visual category map (GI2): moved first so it's visible on arrival
		// without scrolling past the full expense/income lists (C70/C63 tree view).
		If(len(cats) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: "Category map",
			Body:  uiw.Mermaid(uiw.MermaidProps{Source: mermaid.FromCategories(cats), Label: "Category hierarchy diagram"}),
		})),
		Section(css.Class("card"),
			Div(css.Class("card-head"),
				H2(css.Class("card-title"), uistate.T("categories.expenseTitle")),
				sortToggleBtn(),
				addCatBtn(),
			),
			IfElse(len(expenseList) == 0, ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("categories.expenseEmpty"), CTALabel: uistate.T("categories.addFirstExpense"), AddTarget: "category"}), Div(css.Class("rows"), MapKeyed(expenseFlats, flatKey, renderFlat))),
		),
		Section(css.Class("card"),
			Div(css.Class("card-head"),
				H2(css.Class("card-title"), uistate.T("categories.incomeTitle")),
				addCatBtn(),
			),
			IfElse(len(incomeList) == 0, ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("categories.incomeEmpty"), CTALabel: uistate.T("categories.addFirstIncome"), AddTarget: "category"}), Div(css.Class("rows"), MapKeyed(incomeFlats, flatKey, renderFlat))),
		),
	)
}

type categoryRowProps struct {
	Category      domain.Category
	Depth         int
	AllCategories []domain.Category // for the inline parent picker
	TxnCount      int               // transactions filed under this category
	HasChildren   bool              // true when this category has at least one child
	Collapsed     bool              // true when this category's children are hidden
	IsChild       bool              // true when depth > 0 (sub-category nesting cue, GI2)
	IsZeroUsage   bool              // true when TxnCount == 0 (dim treatment, GI2)
	OnView        func(string)      // drill into Transactions filtered by category
	OnDelete      func(string)
	OnSave        func(id, name, kind, parent, color string, deductible bool)
	OnToggle      func(id string) // toggle collapse/expand for this category
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

// CategoryRow is a per-category row. It can be edited inline (name + kind). All
// hooks are declared unconditionally so the edit toggle never reorders them.
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
	editing := ui.UseState(false)
	nameS := ui.UseState(c.Name)
	kindS := ui.UseState(string(c.Kind))
	parentS := ui.UseState(c.ParentID)
	colorS := ui.UseState(catColor(c.Color))
	deductibleS := ui.UseState(c.Deductible)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	// onKind/onParent hook slots kept for stable hook ordering; SelectInput owns the
	// change event internally.
	ui.UseEvent(func(e ui.Event) {
		kindS.Set(e.GetValue())
		parentS.Set("")
	})
	ui.UseEvent(func(e ui.Event) { parentS.Set(e.GetValue()) })
	onDeductible := ui.UseEvent(func(e ui.Event) { deductibleS.Set(e.IsChecked()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(c.Name)
		kindS.Set(string(c.Kind))
		parentS.Set(c.ParentID)
		colorS.Set(catColor(c.Color))
		deductibleS.Set(c.Deductible)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(c.ID, nameS.Get(), kindS.Get(), parentS.Get(), colorS.Get(), deductibleS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("cat-edit-" + c.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		// Parent options: same-kind categories except this one (prevents self-parenting).
		var sameKind []domain.Category
		for _, cc := range props.AllCategories {
			if string(cc.Kind) == kindS.Get() && cc.ID != c.ID {
				sameKind = append(sameKind, cc)
			}
		}
		parentOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("categories.noParent")}}
		for _, f := range categorytree.Flatten(sameKind) {
			parentOpts = append(parentOpts, uiw.SelectOption{Value: f.Category.ID, Label: uiw.IndentLabel(f.Depth) + f.Category.Name})
		}
		kindOpts := []uiw.SelectOption{
			{Value: string(domain.KindExpense), Label: uistate.T("category.expense")},
			{Value: string(domain.KindIncome), Label: uistate.T("category.income")},
		}
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				Input(css.Class("field"), Attr("id", "cat-edit-"+c.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   kindOpts,
					Selected:  kindS.Get(),
					OnChange:  func(v string) { kindS.Set(v); parentS.Set("") },
					AriaLabel: "Category type",
				}),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   parentOpts,
					Selected:  parentS.Get(),
					OnChange:  func(v string) { parentS.Set(v) },
					AriaLabel: "Parent category",
				}),
				Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(colorS.Get()), OnInput(onColor)),
				Label(css.Class("checkbox-label"), Attr("title", uistate.T("categories.deductibleTitle")),
					Input(Type("checkbox"), Attr("id", "cat-edit-deductible-"+c.ID), Attr("aria-label", uistate.T("categories.deductible")), Attr("data-testid", "cat-deductible-"+c.ID), CheckedIf(deductibleS.Get()), OnChange(onDeductible)),
					Text(" "+uistate.T("categories.deductible")),
				),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	// Sub-categories nest with real left padding (a guide line via border) rather
	// than literal "— " prefixes, for a cleaner hierarchy (C63). Depth 0 is flush.
	descStyle := map[string]string{}
	if props.Depth > 0 {
		descStyle["padding-left"] = uiw.IndentPx(props.Depth)
		descStyle["border-left"] = "2px solid var(--border, #2a2a2a)"
		descStyle["margin-left"] = "2px"
	}
	kindLabel := uistate.T("category.expense")
	if c.Kind == domain.KindIncome {
		kindLabel = uistate.T("category.income")
	}
	// Chevron toggle: shown for parent categories; a spacer aligns leaf rows.
	var toggleBtn ui.Node
	if props.HasChildren {
		chevronIcon := icon.ChevronRight
		if props.Collapsed {
			chevronIcon = icon.ChevronRight
		} else {
			chevronIcon = icon.ChevronDown
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
			Span(css.Class("row-meta"),
				Text(kindLabel),
				Text(" · "),
				// Per-row usage (C63): show how many transactions are filed under
				// this category, and drill into Transactions filtered by it when
				// there are any (matches the Accounts/Members drill pattern).
				IfElse(props.TxnCount > 0,
					Button(css.Class("btn-link cat-usage"), Type("button"), Title("View these transactions"), OnClick(view), Text(plural(props.TxnCount, "transaction"))),
					Span(css.Class(tw.TextFaint), Text("No transactions"))),
			),
		),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("categories.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("categories.deleteTitle")), Title(uistate.T("categories.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
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

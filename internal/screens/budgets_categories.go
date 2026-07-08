// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// BudgetCategoriesBody is the "tracked categories" flip modal (mounted at the shell root
// by app.BudgetCategoriesHost). It lets a budget track 1..n expense categories: check
// the categories this budget should count, and its spend becomes their combined total
// (each still rolls up its own sub-categories). Overlap is allowed — a category already
// tracked by another budget shows a soft "also in …" note. Nothing is written until Save.
func BudgetCategoriesBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetCategoriesEdit()
	budgetID := openAtom.Get()

	var budget domain.Budget
	found := false
	if app != nil {
		for _, b := range app.Budgets() {
			if b.ID == budgetID {
				budget, found = b, true
				break
			}
		}
	}

	// Seed the checklist from the budget's current tracked set.
	seed := make(map[string]bool)
	for _, id := range budget.TrackedCategoryIDs() {
		seed[id] = true
	}
	picked := ui.UseState(seed)
	toggle := func(id string) {
		m := picked.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[id] = !nm[id]
		picked.Set(nm)
	}

	onCancel := ui.UseEvent(Prevent(func() { openAtom.Set("") }))
	onSave := ui.UseEvent(Prevent(func() {
		var sel []string
		for _, c := range app.Categories() {
			if c.Kind == domain.KindExpense && picked.Get()[c.ID] {
				sel = append(sel, c.ID)
			}
		}
		if len(sel) == 0 {
			return // Save is disabled in this state; guard anyway.
		}
		b := budget
		// Store a single category in the historical shape; only reach for CategoryIDs when
		// tracking more than one, so single-category budgets stay unchanged.
		b.CategoryID = sel[0]
		if len(sel) > 1 {
			b.CategoryIDs = sel
		} else {
			b.CategoryIDs = nil
		}
		if err := app.PutBudget(b); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.PostNotice(uistate.T("budgets.catsSaved"), false)
		uistate.BumpDataRevision()
		openAtom.Set("")
	}))

	if !found {
		return Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted"), uistate.T("common.notReady")),
			Div(css.Class("autobudget-footer"),
				Button(css.Class("btn"), Type("button"), OnClick(onCancel), uistate.T("action.close"))))
	}

	// Which OTHER budgets track each category (for the soft overlap note).
	otherBudget := make(map[string]string)
	for _, b := range app.Budgets() {
		if b.ID == budgetID {
			continue
		}
		for _, cid := range b.TrackedCategoryIDs() {
			if otherBudget[cid] == "" {
				otherBudget[cid] = firstNonEmpty(b.Name, "")
			}
		}
	}

	sel := picked.Get()
	nSel := 0
	for _, c := range app.Categories() {
		if c.Kind == domain.KindExpense && sel[c.ID] {
			nSel++
		}
	}

	var expenseCats []domain.Category
	for _, c := range app.Categories() {
		if c.Kind == domain.KindExpense {
			expenseCats = append(expenseCats, c)
		}
	}
	keyOf := func(c domain.Category) any { return c.ID }
	rows := MapKeyed(expenseCats, keyOf, func(c domain.Category) ui.Node {
		return ui.CreateElement(budgetCatRow, budgetCatRowProps{
			CategoryID: c.ID, CategoryName: c.Name, Checked: sel[c.ID],
			AlsoIn: otherBudget[c.ID], OnToggle: toggle,
		})
	})

	return Div(css.Class(tw.FlexCol, tw.Gap3),
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), Attr("data-testid", "budgetcats-intro"),
			uistate.T("budgets.catsIntro")),
		Div(css.Class("autobudget-rows"), Attr("data-testid", "budgetcats-rows"), rows),
		Div(css.Class("autobudget-footer"),
			Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "budgetcats-count"),
				uistate.T("budgets.catsCount", plural(nSel, "category"))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgetcats-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			buttonWithDisabled(nSel == 0, []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "budgetcats-save"), OnClick(onSave)},
				uistate.T("budgets.catsSave"))))
}

// budgetCatRowProps drives one selectable category in the tracked-categories modal.
type budgetCatRowProps struct {
	CategoryID   string
	CategoryName string
	Checked      bool
	AlsoIn       string // name of another budget already tracking this category ("" = none)
	OnToggle     func(id string)
}

// budgetCatRow is one category checklist row (its own component so its checkbox hook is
// never registered inside the results loop).
func budgetCatRow(props budgetCatRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.CategoryID) })
	return Label(css.Class("row", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer", "padding": ".35rem .3rem"}),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgetcat-pick-"+props.CategoryID), OnChange(onToggle)}, checkedAttr(props.Checked)...)...),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.CategoryName),
			If(props.AlsoIn != "", Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.catsAlsoIn", props.AlsoIn)))),
	)
}

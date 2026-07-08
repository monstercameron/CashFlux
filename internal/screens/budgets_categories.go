// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

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

	nSel := 0
	for id, on := range picked.Get() {
		if on && id != "" {
			nSel++
		}
	}

	return Div(css.Class(tw.FlexCol, tw.Gap2),
		ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{
			Picked: picked.Get(), OnToggle: toggle, ExcludeBudgetID: budgetID,
		}),
		Div(css.Class("autobudget-footer"),
			Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "budgetcats-count"),
				uistate.T("budgets.catsCount", plural(nSel, "category"))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgetcats-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			buttonWithDisabled(nSel == 0, []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "budgetcats-save"), OnClick(onSave)},
				uistate.T("budgets.catsSave"))))
}

// budgetCategoryPickerProps configures the reusable multi-category picker (used by the
// tracked-categories modal and the add/edit budget forms).
type budgetCategoryPickerProps struct {
	Picked          map[string]bool
	OnToggle        func(id string)
	ExcludeBudgetID string // a budget id whose own tracking is ignored in the overlap note ("" = none)
}

// budgetCategoryPicker is the reusable "which categories does this budget track"
// control: a search box that filters a clean one-line checklist of expense categories.
// It owns only its search-query state; the parent owns the picked set. Keeping it a
// component means its search hook stays at a stable position wherever it's embedded.
func budgetCategoryPicker(props budgetCategoryPickerProps) ui.Node {
	app := appstate.Default
	query := ui.UseState("")
	onQuery := ui.UseEvent(func(v string) { query.Set(v) })

	// Which OTHER budgets track each category — for the soft overlap tag.
	otherBudget := make(map[string]string)
	if app != nil {
		for _, b := range app.Budgets() {
			if b.ID == props.ExcludeBudgetID {
				continue
			}
			for _, cid := range b.TrackedCategoryIDs() {
				if otherBudget[cid] == "" {
					otherBudget[cid] = b.Name
				}
			}
		}
	}

	q := strings.ToLower(strings.TrimSpace(query.Get()))
	var shown []domain.Category
	if app != nil {
		for _, c := range app.Categories() {
			if c.Kind != domain.KindExpense {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
				continue
			}
			shown = append(shown, c)
		}
	}

	keyOf := func(c domain.Category) any { return c.ID }
	rows := MapKeyed(shown, keyOf, func(c domain.Category) ui.Node {
		return ui.CreateElement(budgetCatRow, budgetCatRowProps{
			CategoryID: c.ID, CategoryName: c.Name, Checked: props.Picked[c.ID],
			AlsoIn: otherBudget[c.ID], OnToggle: props.OnToggle,
		})
	})

	var list ui.Node
	if len(shown) == 0 {
		list = P(css.Class("muted", tw.Text13), Attr("data-testid", "budgetcats-none"), uistate.T("budgets.catsNoMatch"))
	} else {
		list = Div(css.Class("budgetcats-list"), Attr("data-testid", "budgetcats-rows"), rows)
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Input(css.Class("field"), Type("search"), Attr("data-testid", "budgetcats-search"),
			Attr("aria-label", uistate.T("budgets.catsSearch")), Placeholder(uistate.T("budgets.catsSearch")),
			Value(query.Get()), OnInput(onQuery)),
		list,
	)
}

// budgetCatRowProps drives one selectable category in the picker.
type budgetCatRowProps struct {
	CategoryID   string
	CategoryName string
	Checked      bool
	AlsoIn       string // name of another budget already tracking this category ("" = none)
	OnToggle     func(id string)
}

// budgetCatRow is one clean, one-line category checklist row: checkbox + name, with a
// subtle right-aligned "in <budget>" tag only when the category is already budgeted
// elsewhere. Its own component so its checkbox hook is never registered in a loop.
func budgetCatRow(props budgetCatRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle(props.CategoryID) })
	rowCls := "budgetcat-row"
	if props.Checked {
		rowCls += " is-on"
	}
	return Label(ClassStr(rowCls),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgetcat-pick-"+props.CategoryID), OnChange(onToggle)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("budgetcat-name"), props.CategoryName),
		If(props.AlsoIn != "", Span(css.Class("budgetcat-also"), uistate.T("budgets.catsAlsoIn", props.AlsoIn))),
	)
}

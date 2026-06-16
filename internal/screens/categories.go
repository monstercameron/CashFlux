//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Categories manages income and expense categories: add, list (grouped by kind),
// and per-row delete.
func Categories() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:categories", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	name := ui.UseState("")
	kind := ui.UseState(string(domain.KindExpense))
	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // category awaiting reassignment before delete
	reassignTo := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kind.Set(e.GetValue()) })
	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set("Enter a category name.")
			return
		}
		c := domain.Category{ID: id.New(), Name: n, Kind: domain.CategoryKind(kind.Get())}
		if err := app.PutCategory(c); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		errMsg.Set("")
		bump()
	}))

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
			errMsg.Set("Pick a different category to move these into.")
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

	kindOptions := []ui.Node{
		Option(Value(string(domain.KindExpense)), SelectedIf(kind.Get() == string(domain.KindExpense)), "Expense"),
		Option(Value(string(domain.KindIncome)), SelectedIf(kind.Get() == string(domain.KindIncome)), "Income"),
	}

	form := Section(Class("card"),
		H2(Class("card-title"), "Add category"),
		Form(Class("form-grid"), OnSubmit(add),
			Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
			Select(Class("field"), OnChange(onKind), kindOptions),
			Button(Class("btn btn-primary"), Type("submit"), "Add"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
	)

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
	saveCat := func(id, newName, kind string) {
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
			if err := app.PutCategory(c); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}
	renderRow := func(c domain.Category) ui.Node {
		return ui.CreateElement(CategoryRow, categoryRowProps{Category: c, OnDelete: deleteCat, OnSave: saveCat})
	}
	keyOf := func(c domain.Category) any { return c.ID }

	// Reassign-before-delete panel, shown when a used category is being deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		target := catByID[rid]
		opts := []ui.Node{Option(Value(""), SelectedIf(reassignTo.Get() == ""), "— Choose category —")}
		for _, c := range cats {
			if c.ID == rid {
				continue
			}
			opts = append(opts, Option(Value(c.ID), SelectedIf(reassignTo.Get() == c.ID), c.Name))
		}
		reassignPanel = Section(Class("card"),
			H2(Class("card-title"), "Reassign before deleting"),
			P(Class("muted"), fmt.Sprintf("%q is used by %d transaction(s) or budget(s). Move them to another category, then it will be deleted.", target.Name, categoryUsage(rid))),
			Form(Class("form-grid"), OnSubmit(confirmReassign),
				Select(Class("field"), OnChange(onReassignTo), opts),
				Button(Class("btn btn-primary"), Type("submit"), "Move and delete"),
				Button(Class("btn"), Type("button"), OnClick(cancelReassign), "Cancel"),
			),
		)
	}

	return Div(
		form,
		reassignPanel,
		Section(Class("card"),
			H2(Class("card-title"), "Expense categories"),
			IfElse(len(expenseList) == 0, P(Class("empty"), "No expense categories yet."), Div(Class("rows"), MapKeyed(expenseList, keyOf, renderRow))),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Income categories"),
			IfElse(len(incomeList) == 0, P(Class("empty"), "No income categories yet."), Div(Class("rows"), MapKeyed(incomeList, keyOf, renderRow))),
		),
	)
}

type categoryRowProps struct {
	Category domain.Category
	OnDelete func(string)
	OnSave   func(id, name, kind string)
}

// CategoryRow is a per-category row. It can be edited inline (name + kind). All
// hooks are declared unconditionally so the edit toggle never reorders them.
func CategoryRow(props categoryRowProps) ui.Node {
	c := props.Category
	del := ui.UseEvent(Prevent(func() { props.OnDelete(c.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(c.Name)
	kindS := ui.UseState(string(c.Kind))
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kindS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(c.Name)
		kindS.Set(string(c.Kind))
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(c.ID, nameS.Get(), kindS.Get())
		editing.Set(false)
	}))

	if editing.Get() {
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(nameS.Get()), OnInput(onName)),
				Select(Class("field"), OnChange(onKind),
					Option(Value(string(domain.KindExpense)), SelectedIf(kindS.Get() == string(domain.KindExpense)), "Expense"),
					Option(Value(string(domain.KindIncome)), SelectedIf(kindS.Get() == string(domain.KindIncome)), "Income"),
				),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), c.Name),
			Span(Class("row-meta"), humanizeType(string(c.Kind))),
		),
		Button(Class("btn"), Type("button"), Title("Edit category"), OnClick(startEdit), "Edit"),
		Button(Class("btn-del"), Type("button"), Title("Delete category"), OnClick(del), "✕"),
	)
}

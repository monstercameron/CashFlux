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

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) { kind.Set(e.GetValue()) })

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

	deleteCat := func(catID string) {
		// Guard: don't orphan transactions/budgets that reference this category.
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
		if used > 0 {
			errMsg.Set(fmt.Sprintf("This category is used by %d transaction(s) or budget(s). Recategorize those first.", used))
			return
		}
		if err := app.DeleteCategory(catID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

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

	var incomeList, expenseList []domain.Category
	for _, c := range app.Categories() {
		if c.Kind == domain.KindIncome {
			incomeList = append(incomeList, c)
		} else {
			expenseList = append(expenseList, c)
		}
	}
	renderRow := func(c domain.Category) ui.Node {
		return ui.CreateElement(CategoryRow, categoryRowProps{Category: c, OnDelete: deleteCat})
	}
	keyOf := func(c domain.Category) any { return c.ID }

	return Div(
		form,
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
}

// CategoryRow is a per-category row with a stable delete-handler hook.
func CategoryRow(props categoryRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Category.ID) }))
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Category.Name),
			Span(Class("row-meta"), humanizeType(string(props.Category.Kind))),
		),
		Button(Class("btn-del"), Type("button"), Title("Delete category"), OnClick(del), "✕"),
	)
}

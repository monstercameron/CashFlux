//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Categories manages income and expense categories: add, list (grouped by kind),
// and per-row delete.
func Categories() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:categories", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	name := ui.UseState("")
	kind := ui.UseState(string(domain.KindExpense))
	parentID := ui.UseState("")
	color := ui.UseState("#7c83ff")
	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // category awaiting reassignment before delete
	reassignTo := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) {
		kind.Set(e.GetValue())
		parentID.Set("") // a parent must share the new kind; clear the stale choice
	})
	onParent := ui.UseEvent(func(e ui.Event) { parentID.Set(e.GetValue()) })
	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set(uistate.T("categories.nameRequired"))
			return
		}
		c := domain.Category{ID: id.New(), Name: n, Kind: domain.CategoryKind(kind.Get()), ParentID: parentID.Get(), Color: color.Get()}
		if err := app.PutCategory(c); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		parentID.Set("")
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

	kindOptions := []ui.Node{
		Option(Value(string(domain.KindExpense)), SelectedIf(kind.Get() == string(domain.KindExpense)), uistate.T("category.expense")),
		Option(Value(string(domain.KindIncome)), SelectedIf(kind.Get() == string(domain.KindIncome)), uistate.T("category.income")),
	}

	// Parent options: existing categories of the chosen kind, indented by depth.
	var kindCats []domain.Category
	for _, c := range app.Categories() {
		if string(c.Kind) == kind.Get() {
			kindCats = append(kindCats, c)
		}
	}
	parentOpts := []ui.Node{Option(Value(""), SelectedIf(parentID.Get() == ""), uistate.T("categories.noParentTop"))}
	for _, f := range categorytree.Flatten(kindCats) {
		parentOpts = append(parentOpts, Option(Value(f.Category.ID), SelectedIf(parentID.Get() == f.Category.ID), indentLabel(f.Depth)+f.Category.Name))
	}

	form := Section(Class("card"),
		H2(Class("card-title"), uistate.T("categories.add")),
		Form(Class("form-grid"), OnSubmit(add),
			Input(append([]any{Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("cat-err", errMsg.Get())...)...),
			Select(Class("field"), OnChange(onKind), kindOptions),
			Select(Class("field"), Title(uistate.T("categories.parentOptional")), OnChange(onParent), parentOpts),
			Input(Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(color.Get()), OnInput(onColor)),
			Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		),
		errText("cat-err", errMsg.Get()),
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
	saveCat := func(id, newName, kind, parent, color string) {
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
			if err := app.PutCategory(c); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}
	renderFlat := func(f categorytree.Flat) ui.Node {
		return ui.CreateElement(CategoryRow, categoryRowProps{Category: f.Category, Depth: f.Depth, AllCategories: cats, OnDelete: deleteCat, OnSave: saveCat})
	}
	flatKey := func(f categorytree.Flat) any { return f.Category.ID }

	// Reassign-before-delete panel, shown when a used category is being deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		target := catByID[rid]
		opts := []ui.Node{Option(Value(""), SelectedIf(reassignTo.Get() == ""), uistate.T("categories.chooseCategory"))}
		for _, c := range cats {
			if c.ID == rid {
				continue
			}
			opts = append(opts, Option(Value(c.ID), SelectedIf(reassignTo.Get() == c.ID), c.Name))
		}
		reassignPanel = Section(Class("card"),
			H2(Class("card-title"), uistate.T("common.reassignTitle")),
			P(Class("muted"), uistate.T("categories.reassignDesc", target.Name, categoryUsage(rid))),
			Form(Class("form-grid"), OnSubmit(confirmReassign),
				Select(Class("field"), OnChange(onReassignTo), opts),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("common.moveAndDelete")),
				Button(Class("btn"), Type("button"), OnClick(cancelReassign), uistate.T("action.cancel")),
			),
		)
	}

	return Div(
		form,
		reassignPanel,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("categories.expenseTitle")),
			IfElse(len(expenseList) == 0, P(Class("empty"), uistate.T("categories.expenseEmpty")), Div(Class("rows"), MapKeyed(categorytree.Flatten(expenseList), flatKey, renderFlat))),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("categories.incomeTitle")),
			IfElse(len(incomeList) == 0, P(Class("empty"), uistate.T("categories.incomeEmpty")), Div(Class("rows"), MapKeyed(categorytree.Flatten(incomeList), flatKey, renderFlat))),
		),
	)
}

type categoryRowProps struct {
	Category      domain.Category
	Depth         int
	AllCategories []domain.Category // for the inline parent picker
	OnDelete      func(string)
	OnSave        func(id, name, kind, parent, color string)
}

// indentLabel returns a depth-proportional prefix for nested category labels.
func indentLabel(depth int) string {
	return strings.Repeat("— ", depth)
}

// CategoryRow is a per-category row. It can be edited inline (name + kind). All
// hooks are declared unconditionally so the edit toggle never reorders them.
func CategoryRow(props categoryRowProps) ui.Node {
	c := props.Category
	del := ui.UseEvent(Prevent(func() { props.OnDelete(c.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(c.Name)
	kindS := ui.UseState(string(c.Kind))
	parentS := ui.UseState(c.ParentID)
	colorS := ui.UseState(catColor(c.Color))
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) {
		kindS.Set(e.GetValue())
		parentS.Set("") // parent must share the kind
	})
	onParent := ui.UseEvent(func(e ui.Event) { parentS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(c.Name)
		kindS.Set(string(c.Kind))
		parentS.Set(c.ParentID)
		colorS.Set(catColor(c.Color))
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(c.ID, nameS.Get(), kindS.Get(), parentS.Get(), colorS.Get())
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
		parentOpts := []ui.Node{Option(Value(""), SelectedIf(parentS.Get() == ""), uistate.T("categories.noParent"))}
		for _, f := range categorytree.Flatten(sameKind) {
			parentOpts = append(parentOpts, Option(Value(f.Category.ID), SelectedIf(parentS.Get() == f.Category.ID), indentLabel(f.Depth)+f.Category.Name))
		}
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", "cat-edit-"+c.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)),
				Select(Class("field"), OnChange(onKind),
					Option(Value(string(domain.KindExpense)), SelectedIf(kindS.Get() == string(domain.KindExpense)), uistate.T("category.expense")),
					Option(Value(string(domain.KindIncome)), SelectedIf(kindS.Get() == string(domain.KindIncome)), uistate.T("category.income")),
				),
				Select(Class("field"), Title(uistate.T("categories.parent")), OnChange(onParent), parentOpts),
				Input(Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(colorS.Get()), OnInput(onColor)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	desc := c.Name
	if props.Depth > 0 {
		desc = indentLabel(props.Depth) + c.Name // visually nest sub-categories
	}
	kindLabel := uistate.T("category.expense")
	if c.Kind == domain.KindIncome {
		kindLabel = uistate.T("category.income")
	}
	return Div(Class("row"),
		Span(Class("cat-swatch"), Style(map[string]string{"background": catColor(c.Color)})),
		Div(Class("row-main"),
			Span(Class("row-desc"), desc),
			Span(Class("row-meta"), kindLabel),
		),
		Button(Class("btn"), Type("button"), Title(uistate.T("categories.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
		Button(Class("btn-del"), Type("button"), Title(uistate.T("categories.deleteTitle")), OnClick(del), "✕"),
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

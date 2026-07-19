// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// CategoryAddFormProps configures the CategoryAddForm component.
type CategoryAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// CategoryAddForm is the standalone add-a-category form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Categories() for use in the AddHost modal.
func CategoryAddForm(props CategoryAddFormProps) ui.Node {
	return ui.CreateElement(categoryAddForm, props)
}

func categoryAddForm(props CategoryAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	name := ui.UseState("")
	kind := ui.UseState(string(domain.KindExpense))
	parentID := ui.UseState("")
	color := ui.UseState("#7c83ff")
	deductible := ui.UseState(false)
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })
	// onKind and onParent hook slots kept for stable hook ordering; SelectInput owns
	// the change event internally.
	ui.UseEvent(func(e ui.Event) {
		kind.Set(e.GetValue())
		parentID.Set("")
	})
	ui.UseEvent(func(e ui.Event) { parentID.Set(e.GetValue()) })
	onDeductible := ui.UseEvent(func(e ui.Event) { deductible.Set(e.IsChecked()) })

	add := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(name.Get())
		if n == "" {
			errMsg.Set(uistate.T("categories.nameRequired"))
			return
		}
		c := domain.Category{ID: id.New(), Name: n, Kind: domain.CategoryKind(kind.Get()), ParentID: parentID.Get(), Color: color.Get(), Deductible: deductible.Get()}
		if err := app.PutCategory(c); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields (including kind — so subsequent adds start as Expense, L42).
		name.Set("")
		kind.Set(string(domain.KindExpense))
		parentID.Set("")
		color.Set("#7c83ff")
		deductible.Set(false)
		errMsg.Set("")
		uistate.PostNotice(uistate.T("categories.addedToast", n), false)
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	kindOpts := []uiw.SelectOption{
		{Value: string(domain.KindExpense), Label: uistate.T("category.expense")},
		{Value: string(domain.KindIncome), Label: uistate.T("category.income")},
	}

	// Parent options: existing categories of the chosen kind, indented by depth.
	var kindCats []domain.Category
	for _, c := range app.Categories() {
		if string(c.Kind) == kind.Get() {
			kindCats = append(kindCats, c)
		}
	}
	parentOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("categories.noParentTop")}}
	for _, f := range categorytree.Flatten(kindCats) {
		parentOpts = append(parentOpts, uiw.SelectOption{Value: f.Category.ID, Label: uiw.IndentLabel(f.Depth) + f.Category.Name})
	}

	return Form(css.Class("form-grid"), Attr("id", "category-add-form"), Attr("data-testid", "category-add-form"), OnSubmit(add),
		Input(append([]any{css.Class("field"), Attr("id", "cat-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("cat-err", errMsg.Get())...)...),
		uiw.FormField("Category type",
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   kindOpts,
				Selected:  kind.Get(),
				OnChange:  func(v string) { kind.Set(v); parentID.Set("") },
				AriaLabel: uistate.T("categories.kindAria"),
			})),
		uiw.FormField(uistate.T("categories.parentOptional"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   parentOpts,
				Selected:  parentID.Get(),
				OnChange:  func(v string) { parentID.Set(v) },
				AriaLabel: uistate.T("categories.parentOptional"),
			})),
		Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(color.Get()), OnInput(onColor)),
		Label(css.Class("checkbox-label"), Attr("title", uistate.T("categories.deductibleTitle")),
			Input(Type("checkbox"), Attr("id", "cat-add-deductible"), Attr("aria-label", uistate.T("categories.deductible")), CheckedIf(deductible.Get()), OnChange(onDeductible)),
			Text(" "+uistate.T("categories.deductible")),
		),
		errText("cat-err", errMsg.Get()),
	)
}

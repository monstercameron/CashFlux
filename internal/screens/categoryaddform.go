//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
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
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onColor := ui.UseEvent(func(v string) { color.Set(v) })
	onKind := ui.UseEvent(func(e ui.Event) {
		kind.Set(e.GetValue())
		parentID.Set("") // a parent must share the new kind; clear the stale choice
	})
	onParent := ui.UseEvent(func(e ui.Event) { parentID.Set(e.GetValue()) })

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
		// Reset fields.
		name.Set("")
		parentID.Set("")
		color.Set("#7c83ff")
		errMsg.Set("")
		uistate.PostNotice(uistate.T("categories.addedToast", n), false)
		if props.OnDone != nil {
			props.OnDone()
		}
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

	return Form(css.Class("form-grid"), Attr("data-testid", "category-add-form"), OnSubmit(add),
		Input(append([]any{css.Class("field"), Attr("id", "cat-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("cat-err", errMsg.Get())...)...),
		Select(css.Class("field"), Attr("aria-label", "Category type"), OnChange(onKind), kindOptions),
		Select(css.Class("field"), Attr("aria-label", "Parent category (optional)"), Title(uistate.T("categories.parentOptional")), OnChange(onParent), parentOpts),
		Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(color.Get()), OnInput(onColor)),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		errText("cat-err", errMsg.Get()),
	)
}

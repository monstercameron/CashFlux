// SPDX-License-Identifier: MIT

//go:build js && wasm

// Shared category picker (C499). Inline "quick-add a category" was copy-pasted
// into four forms — transaction edit, budget add, the Smart+ modal and the chat
// agent — and every one of them built domain.Category{ID, Name, Kind} with NO
// ParentID, so every category ever created inline was top level. This is the one
// implementation, and it can create a child.
package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// catPathSep is the separator shown between a parent and its child. The option
// TEXT must carry the parent, not just the optgroup label: a CLOSED select shows
// only the option, and a bare "Gas" is ambiguous once Auto and Utilities both
// have one — the same collision C489 fixes in the AI contract.
const catPathSep = " › "

// catOption is one row of the category list: a real category with its display
// path and depth.
type catOption struct {
	ID    string
	Path  string
	Depth int
	Kind  domain.CategoryKind
}

// categoryOptions flattens the tree into display order (parent before children),
// filtered to kind. Sorted deterministically by categorytree.Flatten.
func categoryOptions(cats []domain.Category, kind domain.CategoryKind) []catOption {
	nameByID := make(map[string]string, len(cats))
	for _, c := range cats {
		nameByID[c.ID] = c.Name
	}
	out := make([]catOption, 0, len(cats))
	for _, f := range categorytree.Flatten(cats) {
		if f.Category.Kind != kind {
			continue
		}
		path := f.Category.Name
		if p, ok := nameByID[f.Category.ParentID]; ok && f.Category.ParentID != f.Category.ID {
			path = p + catPathSep + f.Category.Name
		}
		out = append(out, catOption{ID: f.Category.ID, Path: path, Depth: f.Depth, Kind: f.Category.Kind})
	}
	return out
}

// catSelectNew is the signal a row sends to open the picker. It is NOT an option
// inside the select: a <select> is a controlled input the browser also mutates,
// so a sentinel option could be committed as the element's live value and left
// displayed as though it were a category. The escape hatch is a separate button,
// which cannot be selected by accident.
const catSelectNew = "__new__"

// categorySelectNodes renders the <option> set for an inline category select:
// every category as a full path, then the create escape hatch. Used by the bulk
// rows, where switching a category must not cost a trip through a modal.
func categorySelectNodes(cats []domain.Category, kind domain.CategoryKind, selected string) []any {
	out := []any{Option(Value(""), SelectedIf(selected == ""), uistate.T("review.choose"))}
	for _, o := range categoryOptions(cats, kind) {
		out = append(out, Option(Value(o.ID), SelectedIf(selected == o.ID), o.Path))
	}
	return out
}

// CreateCategory makes a category, optionally nested under an existing parent,
// and returns its id. Kind is inherited from the parent when there is one, so a
// child can never contradict its parent (the tree validation would reject it).
// Errors are surfaced as a notice and reported by ok=false.
func CreateCategory(app *appstate.App, name, parentID string, kind domain.CategoryKind) (string, bool) {
	name = strings.TrimSpace(name)
	if app == nil || name == "" {
		return "", false
	}
	if parentID != "" {
		for _, p := range app.Categories() {
			if p.ID == parentID {
				kind = p.Kind
				break
			}
		}
	}
	c := domain.Category{ID: id.New(), Name: name, Kind: kind, ParentID: parentID}
	if err := app.PutCategory(c); err != nil {
		uistate.PostNotice(err.Error(), true)
		return "", false
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	uistate.PostNotice(uistate.T("categories.addedToast", name), false)
	return c.ID, true
}

// CategoryPickerProps drives the shared picker overlay.
type CategoryPickerProps struct {
	// Kind filters the list and is the kind a newly created top-level category
	// gets — inferred by the caller from the transaction's sign.
	Kind domain.CategoryKind
	// Selected is the currently chosen category id, if any.
	Selected string
	// OnPick receives the chosen (or newly created) category id.
	OnPick func(categoryID string)
	// OnClose dismisses the picker without choosing.
	OnClose func()
}

// CategoryPicker is a search-and-create overlay over the category tree. It is
// mounted by whichever surface needs it and dismissed via OnClose.
func CategoryPicker(props CategoryPickerProps) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()

	query := ui.UseState("")
	parentID := ui.UseState("")

	onQuery := ui.UseEvent(func(v string) { query.Set(v) })
	onParent := ui.UseEvent(func(e ui.Event) { parentID.Set(e.GetValue()) })
	closePicker := ui.UseEvent(func() {
		if props.OnClose != nil {
			props.OnClose()
		}
	})
	// doCreate is a plain closure so both the button and the Enter key run it
	// without Enter submitting any enclosing form.
	doCreate := func() {
		if newID, ok := CreateCategory(app, query.Get(), parentID.Get(), props.Kind); ok {
			if props.OnPick != nil {
				props.OnPick(newID)
			}
			query.Set("")
			parentID.Set("")
		}
	}
	create := ui.UseEvent(Prevent(func() { doCreate() }))
	onKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		switch e.GetKey() {
		case "Enter":
			e.PreventDefault()
			doCreate()
		case "Escape":
			e.PreventDefault()
			if props.OnClose != nil {
				props.OnClose()
			}
		}
	})

	if app == nil {
		return Fragment()
	}
	q := strings.ToLower(strings.TrimSpace(query.Get()))
	all := categoryOptions(app.Categories(), props.Kind)
	matches := make([]catOption, 0, len(all))
	for _, o := range all {
		if q == "" || strings.Contains(strings.ToLower(o.Path), q) {
			matches = append(matches, o)
		}
	}

	// Parent choices for the create block: top-level categories of this kind, so
	// the tree stays two deep and legible.
	parents := make([]domain.Category, 0)
	for _, c := range app.Categories() {
		if c.ParentID == "" && c.Kind == props.Kind {
			parents = append(parents, c)
		}
	}
	sort.SliceStable(parents, func(i, j int) bool { return parents[i].Name < parents[j].Name })
	parentOpts := []any{css.Class("field"), Attr("data-testid", "catpick-parent"),
		Attr("aria-label", uistate.T("catpick.parentLabel")), OnChange(onParent),
		Option(Value(""), SelectedIf(parentID.Get() == ""), uistate.T("catpick.topLevel"))}
	for _, p := range parents {
		parentOpts = append(parentOpts, Option(Value(p.ID), SelectedIf(parentID.Get() == p.ID),
			uistate.T("catpick.inside", p.Name)))
	}

	typed := strings.TrimSpace(query.Get())
	return Div(css.Class("catpick-back"), Attr("data-testid", "catpick"),
		Div(css.Class("catpick"), Attr("role", "dialog"), Attr("aria-label", uistate.T("catpick.title")),
			Div(css.Class("catpick-search"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "catpick-search"),
					Attr("aria-label", uistate.T("catpick.searchPlaceholder")),
					Attr("placeholder", uistate.T("catpick.searchPlaceholder")),
					Value(query.Get()), OnInput(onQuery), OnKeyDown(onKey)),
			),
			Div(css.Class("catpick-list"),
				If(len(matches) == 0, P(css.Class("catpick-empty"), uistate.T("catpick.noMatch", typed))),
				MapKeyed(matches, func(o catOption) any { return o.ID }, func(o catOption) ui.Node {
					// Own component: its OnClick hook must never be registered
					// inside a variable-length loop (the framework's top gotcha).
					return ui.CreateElement(catPickRow, catPickRowProps{
						ID: o.ID, Path: o.Path, Depth: o.Depth,
						Selected: o.ID == props.Selected, OnPick: props.OnPick,
					})
				}),
			),
			If(typed != "",
				Div(css.Class("catpick-make"), Attr("data-testid", "catpick-make"),
					Div(css.Class("catpick-make-t"), uistate.T("catpick.makeTitle", typed)),
					Div(css.Class("catpick-make-row"),
						Select(parentOpts...),
						Button(css.Class("btn btn-primary btn-sm"), Type("button"),
							Attr("data-testid", "catpick-create"), OnClick(create),
							uistate.T("catpick.create")),
					),
				)),
			Div(css.Class("catpick-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "catpick-close"),
					OnClick(closePicker), uistate.T("catpick.close")),
			),
		),
	)
}

// catPickRowProps drives one selectable category row.
type catPickRowProps struct {
	ID       string
	Path     string
	Depth    int
	Selected bool
	OnPick   func(string)
}

// catPickRow is one row of the picker list, its own component so the click
// handler is registered at a stable hook position.
func catPickRow(props catPickRowProps) ui.Node {
	pick := ui.UseEvent(func() {
		if props.OnPick != nil {
			props.OnPick(props.ID)
		}
	})
	cls := "catpick-opt"
	if props.Depth > 0 {
		cls += " is-child"
	}
	if props.Selected {
		cls += " is-sel"
	}
	return Button(css.Class(cls), Type("button"), Attr("data-testid", "catpick-opt-"+props.ID),
		OnClick(pick), props.Path)
}

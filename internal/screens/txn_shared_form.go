// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// TxnFormMode identifies which transaction flow is using the shared form.
type TxnFormMode string

const (
	TxnFormAdd  TxnFormMode = "add"
	TxnFormEdit TxnFormMode = "edit"
)

// TxnFormModel contains the shared fields and their caller-owned controls.
// TxnSharedForm owns their labels, order, and grid placement.
type TxnFormModel struct {
	Mode                                              TxnFormMode
	Amount, Direction, Payee, Category, Date, Account ui.Node
	More                                              []ui.Node
	AddQuickFill, AddTemplates                        ui.Node
	EditTools, EditSummary                            ui.Node
	MoreID                                            string
	OpenMore                                          bool
}

// TxnWhoFieldProps configures the shared owner picker used by Add and Edit.
type TxnWhoFieldProps struct {
	ID       string
	Owners   []string
	Options  []uiw.PickerOption
	Colors   map[string]string
	OnToggle func(string)
}

// TxnWhoField renders the selected owner stack and the accessible multi-picker.
func TxnWhoField(props TxnWhoFieldProps) ui.Node {
	label := uistate.T("who2.label")
	selected := make([]string, 0, len(props.Owners))
	byID := make(map[string]string, len(props.Options))
	for _, option := range props.Options {
		byID[option.Value] = option.Label
	}
	avatars := make([]uiw.MemberAvatarProps, 0, len(props.Owners))
	for _, id := range props.Owners {
		if id == "" {
			continue
		}
		name := byID[id]
		if name == "" {
			name = id
		}
		selected = append(selected, id)
		avatars = append(avatars, uiw.MemberAvatarProps{Name: name, Color: props.Colors[id]})
	}
	accessible := uistate.T("who2.unassigned")
	var summary ui.Node = uiw.MemberUnassigned(accessible, props.ID+"-unassigned")
	if len(avatars) > 0 {
		names := make([]string, 0, len(avatars))
		for _, avatar := range avatars {
			names = append(names, avatar.Name)
		}
		accessible = names[0]
		if len(names) > 1 {
			accessible = uistate.T("who2.sharedBy", whoNames(names))
		}
		summary = uiw.AvatarStack(uiw.AvatarStackProps{Members: avatars, Label: accessible, Title: accessible, TestID: props.ID + "-avatars"})
	}
	return uiw.FormField(label, Div(css.Class("txn-who-field"), summary,
		uiw.MultiPicker(uiw.MultiPickerProps{ID: props.ID, Label: label, Placeholder: accessible, Options: props.Options, Selected: selected, OnToggle: props.OnToggle, TestID: props.ID + "-picker"}),
	))
}

func whoNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func toggleTxnOwner(owners []string, owner string) []string {
	for i, current := range owners {
		if current == owner {
			return append(append([]string(nil), owners[:i]...), owners[i+1:]...)
		}
	}
	return append(append([]string(nil), owners...), owner)
}

// TxnDirectionOption is one choice in the shared Add/Edit direction toggle.
type TxnDirectionOption struct {
	Value    string
	Label    string
	TestID   string
	Disabled bool
	Title    string
}

// TxnDirectionToggleProps describes the direction choices shown by a transaction form.
type TxnDirectionToggleProps struct {
	Label    string
	Value    string
	Options  []TxnDirectionOption
	OnSelect func(string)
}

// TxnDirectionToggle renders the same sized, accessible direction control in Add and Edit.
func TxnDirectionToggle(props TxnDirectionToggleProps) ui.Node {
	return ui.CreateElement(txnDirectionToggle, props)
}

func txnDirectionToggle(props TxnDirectionToggleProps) ui.Node {
	hintID := ui.UseId()
	hint := ""
	for _, option := range props.Options {
		if option.Disabled && option.Title != "" {
			hint = option.Title
			break
		}
	}
	group := []any{css.Class("btn-group", "txn-dir-toggle"), Attr("role", "group"), Attr("aria-label", props.Label)}
	if hint != "" {
		group = append(group, Attr("aria-describedby", hintID))
	}
	group = append(group,
		MapKeyed(props.Options, func(option TxnDirectionOption) any { return option.Value }, func(option TxnDirectionOption) ui.Node {
			return ui.CreateElement(txnDirectionOption, txnDirectionOptionProps{
				Option: option, Selected: props.Value == option.Value, OnSelect: props.OnSelect,
			})
		}),
	)
	return uiw.FormField(props.Label, Div(
		Div(group...),
		If(hint != "", Span(ID(hintID), css.Class("txn-form-help"), hint)),
	))
}

type txnDirectionOptionProps struct {
	Option   TxnDirectionOption
	Selected bool
	OnSelect func(string)
}

func txnDirectionOption(props txnDirectionOptionProps) ui.Node {
	onClick := ui.UseEvent(func(ui.Event) {
		if props.OnSelect != nil {
			props.OnSelect(props.Option.Value)
		}
	})
	className := "btn txn-dir-option"
	if props.Selected {
		className += " is-pressed"
	}
	args := []any{css.Class(className), Type("button"), Attr("data-testid", props.Option.TestID),
		Attr("aria-label", props.Option.Label), Attr("aria-pressed", ariaBool(props.Selected)),
		OnClick(onClick), props.Option.Label}
	if props.Option.Disabled {
		args = append(args, Attr("disabled", "true"), Attr("aria-disabled", "true"), Attr("title", props.Option.Title))
	}
	return Button(args...)
}

// TxnNewCategoryAction renders the compact accessible New category icon used by Add and Edit.
func TxnNewCategoryAction(testID string, expanded bool, onClick ui.Handler) ui.Node {
	return Button(css.Class("btn btn-tool txn-cat-new", "txn-new-category-action"), Type("button"),
		Attr("data-testid", testID), Attr("aria-label", uistate.T("transactions.newCategory")),
		Attr("aria-expanded", ariaBool(expanded)), Title(uistate.T("transactions.newCategory")), OnClick(onClick),
		uiw.Icon(icon.Plus, css.Class("w-4", "h-4")))
}

// TxnSharedForm renders the common Add/Edit field hierarchy: everyday fields
// first, followed by a single disclosure for supporting details.
func TxnSharedForm(model TxnFormModel) ui.Node {
	css.Inject("cf-transaction-form", txnFormCSS)
	moreID := model.MoreID
	if moreID == "" {
		moreID = "txn-add-more"
	}
	detailsArgs := []any{css.Class("csv-help txn-form-details"), Attr("id", moreID), Attr("data-testid", moreID)}
	if model.OpenMore {
		detailsArgs = append(detailsArgs, Attr("open", "open"))
	}
	detailsArgs = append(detailsArgs,
		Summary(Span(uistate.T("r1e.moreDetails")), Span(css.Class("txn-form-optional"), uistate.T("sol1.optional"))),
		Div(css.Class("txn-form-details-grid"), model.More),
	)
	return Div(css.Class("form-grid", "qa-grid", "txn-form"),
		If(model.AddQuickFill != nil, model.AddQuickFill),
		Div(css.Class("txn-shared-row", "txn-shared-row-amount"), Attr("id", "txn-shared-row-amount"), model.Amount, model.Direction),
		Div(css.Class("txn-shared-row", "txn-shared-row-payee"), Attr("id", "txn-shared-row-payee"), model.Payee),
		Div(css.Class("txn-shared-row", "txn-shared-row-category-date"), Attr("id", "txn-shared-row-category-date"), model.Category, model.Date),
		Div(css.Class("txn-shared-row", "txn-shared-row-account"), Attr("id", "txn-shared-row-account"), model.Account),
		Details(detailsArgs...),
		model.EditTools,
		model.EditSummary,
		If(model.AddTemplates != nil, model.AddTemplates),
	)
}

// The shared form owns layout; callers supply controlled fields and action slots.
const txnFormCSS = `
.txn-form.qa-grid { display: flex; flex-direction: column; gap: var(--space-4); }
.txn-form .txn-shared-row { gap: var(--space-4); align-items: start; }
.txn-form .txn-shared-row-payee { grid-template-columns: minmax(0, 1fr); }
.txn-form .txn-shared-row > *, .txn-form .cf-field { min-width: 0; }
.txn-form .txn-shared-row-amount .field { font-size: var(--type-24); font-weight: 600; font-variant-numeric: tabular-nums; }
.txn-form .field, .txn-form .txn-dir-option { min-height: calc(var(--space-5) + var(--space-4)); }
.txn-form .txn-edit-category-label { margin: 0; min-height: auto; }
.txn-form .txn-edit-category-field { display: flex; flex-direction: column; gap: var(--space-1); }
.txn-form .txn-cat-row { display: flex; align-items: start; gap: var(--space-2); }
.txn-form .txn-cat-row > :first-child { flex: 1; min-width: 0; }
.txn-form .txn-cat-picker { display: flex; flex-direction: column; gap: var(--space-2); }
.txn-form .cf-pill { align-self: start; white-space: normal; text-align: start; }
.txn-form .txn-form-details { margin: 0; padding-block: var(--space-4); border-block: 1px solid var(--border); }
.txn-form-details > summary { cursor: pointer; font-weight: 600; }
.txn-form-optional { margin-inline-start: var(--space-2); font-weight: 400; color: var(--text-dim); }
.txn-form-details-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4); padding-block-start: var(--space-4); }
.txn-form-details-grid > .txn-classify-field, .txn-form-details-grid > .txn-exclude-field,
.txn-form-details-grid > .txn-form-detail-group { grid-column: 1 / -1; }
.txn-form-detail-group { display: flex; flex-direction: column; gap: var(--space-4); }
.txn-form-check-field { display: flex; flex-direction: column; gap: var(--space-1); }
.txn-form-check { display: flex; align-items: center; gap: var(--space-2); }
.txn-form-help, .txn-form .cf-field-hint { color: var(--text-dim); font-size: var(--type-12); line-height: 1.5; }
.txn-form .txn-dir-toggle + .txn-form-help { display: block; margin-block-start: var(--space-1); }
.txn-form .cf-field-label { font-size: var(--type-12); color: var(--text-dim); }
.txn-form .cf-field-error { color: var(--danger); font-size: var(--type-12); }
.txn-form-root { display: flex; flex-direction: column; gap: var(--space-4); }
.txn-form-record { display: flex; align-items: start; justify-content: space-between; gap: var(--space-4); padding-block-end: var(--space-4); border-bottom: 1px solid var(--border); }
.txn-form-record-name { font-weight: 600; overflow-wrap: anywhere; }
.txn-form-record-meta { display: flex; flex-direction: column; gap: var(--space-1); min-width: 0; }
.txn-form-record-amount { font-size: var(--type-24); font-weight: 600; white-space: nowrap; }
.txn-form-footer { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); width: 100%; flex-wrap: wrap; }
.txn-form-footer-actions { display: flex; gap: var(--space-2); margin-inline-start: auto; flex-wrap: wrap; justify-content: flex-end; }
@media (max-width: 540px) {
 .txn-form .txn-shared-row, .txn-form-details-grid { grid-template-columns: minmax(0, 1fr); }
 .txn-form .txn-dir-toggle { width: 100%; }
 .txn-form-record { flex-wrap: wrap; }
}
`

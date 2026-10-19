// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
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

// TxnSharedFormProps remains as a compatibility alias for callers compiled during migration.
type TxnSharedFormProps struct {
	Primary               []ui.Node
	More                  []ui.Node
	BeforeMore, AfterMore []ui.Node
	OpenMore              bool
	MoreID                string
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
	return uiw.FormField(props.Label, Div(css.Class("btn-group", "txn-dir-toggle"),
		MapKeyed(props.Options, func(option TxnDirectionOption) any { return option.Value }, func(option TxnDirectionOption) ui.Node {
			return ui.CreateElement(txnDirectionOption, txnDirectionOptionProps{
				Option: option, Selected: props.Value == option.Value, OnSelect: props.OnSelect,
			})
		}),
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
func TxnSharedForm(input any) ui.Node {
	model, legacy := input.(TxnFormModel)
	if !legacy {
		old, ok := input.(TxnSharedFormProps)
		if ok {
			model = TxnFormModel{More: old.More, MoreID: old.MoreID, OpenMore: old.OpenMore,
				EditSummary: Fragment()}
			return legacyTxnSharedForm(old)
		}
		return Fragment()
	}
	moreID := model.MoreID
	if moreID == "" {
		moreID = "txn-add-more"
	}
	detailsArgs := []any{css.Class("csv-help"), Attr("id", moreID), Attr("data-testid", moreID)}
	if model.OpenMore {
		detailsArgs = append(detailsArgs, Attr("open", "open"))
	}
	detailsArgs = append(detailsArgs,
		Summary(uistate.T("r1e.moreDetails")),
		Div(css.Class("form-grid"), model.More),
	)
	return Div(css.Class("form-grid", "qa-grid"),
		If(model.AddQuickFill != nil, model.AddQuickFill),
		Div(css.Class("txn-shared-row", "txn-shared-row-amount"), Attr("id", "txn-shared-row-amount"), model.Amount, model.Direction),
		Div(css.Class("txn-shared-row", "txn-shared-row-payee"), Attr("id", "txn-shared-row-payee"), model.Payee),
		Div(css.Class("txn-shared-row", "txn-shared-row-category-date"), Attr("id", "txn-shared-row-category-date"), model.Category, model.Date),
		Div(css.Class("txn-shared-row", "txn-shared-row-account"), Attr("id", "txn-shared-row-account"), model.Account),
		model.EditTools,
		Details(detailsArgs...),
		model.EditSummary,
		If(model.AddTemplates != nil, model.AddTemplates),
	)
}

func legacyTxnSharedForm(props TxnSharedFormProps) ui.Node {
	moreID := props.MoreID
	if moreID == "" {
		moreID = "txn-add-more"
	}
	details := []any{css.Class("csv-help"), Attr("id", moreID), Attr("data-testid", moreID), Summary(uistate.T("r1e.moreDetails")), Div(css.Class("form-grid"), props.More)}
	if props.OpenMore {
		details = append(details[:3], append([]any{Attr("open", "open")}, details[3:]...)...)
	}
	return Div(css.Class("form-grid", "qa-grid"), props.Primary, props.BeforeMore, Details(details...), props.AfterMore)
}

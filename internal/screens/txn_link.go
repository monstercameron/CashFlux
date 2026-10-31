// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/txnlink"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TxnLinkBody is the payment-link modal's state and persistence shell. Both links
// remain pending across tab changes and are saved together only on Save.
func TxnLinkBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	linkAtom := uistate.UseTxnLinkTarget()
	target := linkAtom.Get()
	model := loadTxnLinkModel(app, target.TxnID)
	// Keep all state hooks unconditional when the payment disappears mid-edit.
	mode := ui.UseState(target.Mode)
	draft := ui.UseState(model.Initial)
	close := func() { linkAtom.Set(uistate.TxnLinkTarget{}) }
	if !model.Found {
		return ui.CreateElement(txnLinkMissing, txnLinkMissingProps{Close: close})
	}
	pending := draft.Get()
	decision := pending.Evaluate(model.Transaction)
	summary := txnlink.Summarize(model.Transaction, model.AccountNames, uistate.LoadPrefs(), uistate.T("transactions.uncategorized"))
	var picker ui.Node
	if mode.Get() == uistate.TxnLinkModeSub {
		picker = ui.CreateElement(txnLinkSubscriptionPicker, txnLinkSubscriptionPickerProps{
			Options: model.Subscriptions, Selected: pending.SubscriptionName,
			Change: func(v string) {
				draft.Update(func(d txnlink.Draft) txnlink.Draft { return d.WithSubscription(v) })
			},
		})
	} else {
		picker = ui.CreateElement(txnLinkBillPicker, txnLinkBillPickerProps{
			Options: model.Bills, Selected: pending.BillAccountID, AutoLink: pending.AutoLink,
			ShowAutoLink: decision.HasBill,
			Merchant:     txnlink.Merchant(model.Transaction, uistate.T("transactions.uncategorized")),
			Change: func(v string) {
				draft.Update(func(d txnlink.Draft) txnlink.Draft { return d.WithBill(v) })
			},
			ToggleAutoLink: func() {
				draft.Update(txnlink.Draft.ToggleAutoLink)
			},
		})
	}
	return Div(css.Class(tw.FlexCol, tw.Gap3),
		ui.CreateElement(txnLinkSummary, summary),
		ui.CreateElement(txnLinkMode, txnLinkModeProps{Selected: mode.Get(), Change: func(v string) { mode.Set(v) }}),
		picker,
		ui.CreateElement(txnLinkPreview, txnLinkPreviewProps{Decision: decision, AccountNames: model.AccountNames}),
		ui.CreateElement(txnLinkFooter, txnLinkFooterProps{Decision: decision, Cancel: close, Save: func() {
			saveTxnLink(app, model.Transaction, draft.Get(), close)
		}}),
	)
}

// loadTxnLinkModel only reads platform state; dataset decisions belong to txnlink.
func loadTxnLinkModel(app *appstate.App, txnID string) txnlink.Model {
	if app == nil {
		return txnlink.Prepare(txnlink.Input{}, txnID)
	}
	settings := app.Settings()
	prefs := uistate.LoadSubsDetectPrefs()
	return txnlink.Prepare(txnlink.Input{
		Transactions: app.Transactions(), Accounts: app.Accounts(), Categories: app.Categories(),
		BaseCurrency: settings.BaseCurrency, FXRates: settings.FXRates,
		IgnoredCategoryIDs: prefs.IgnoredCategoryIDs, IgnoredAccountTypes: prefs.IgnoredAccountTypes,
		MinOccurrences: prefs.MinOccurrencesOrDefault(), Confirmed: loadConfirmedSubs(), Ignored: app.IgnoredSubscriptions(),
	}, txnID)
}

// saveTxnLink applies the pure decision through the existing persistence path.
func saveTxnLink(app *appstate.App, txn domain.Transaction, draft txnlink.Draft, close func()) {
	decision := draft.Evaluate(txn)
	if !decision.Changed {
		uistate.PostNotice(uistate.T("txnlink.nothingToSave"), true)
		return
	}
	// A produced rule is saved in the same commit as the transaction.
	rule, ruleMade := txnlink.BillRule(app.Rules(), txn, draft, id.New(), app.NextRuleOrder())
	var err error
	if ruleMade {
		err = app.PutTransactionWithRule(decision.Transaction, rule)
	} else {
		err = app.PutTransaction(decision.Transaction)
	}
	if err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.PostNotice(uistate.T(decision.NoticeKey(ruleMade)), false)
	uistate.BumpDataRevision()
	close()
}

type txnLinkMissingProps struct{ Close func() }

func txnLinkMissing(p txnLinkMissingProps) ui.Node {
	closeClick := OnClick(ui.UseEvent(Prevent(p.Close)))
	return Div(css.Class(tw.FlexCol, tw.Gap3),
		P(css.Class("muted"), Attr("data-testid", "txnlink-missing"), uistate.T("txnlink.missing")),
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), closeClick, uistate.T("action.close"))))
}

func txnLinkSummary(p txnlink.Summary) ui.Node {
	return Div(css.Class("txnlink-summary"), Attr("data-testid", "txnlink-summary"),
		Div(css.Class("txnlink-summary-main"),
			Span(css.Class("txnlink-summary-desc"), p.Description),
			Span(css.Class("txnlink-summary-meta", tw.TextDim), p.AccountLine)),
		Span(css.Class("txnlink-summary-amount", tw.FontDisplay, tw.ColorClass(p.Tone)), p.Amount))
}

type txnLinkModeProps struct {
	Selected string
	Change   func(string)
}

func txnLinkMode(p txnLinkModeProps) ui.Node {
	return uiw.Segmented(uiw.SegmentedProps{
		Label: uistate.T("txnlink.modeLabel"), Selected: p.Selected,
		Options: []uiw.SegOption{
			{Value: uistate.TxnLinkModeBill, Label: uistate.T("txnlink.modeBill"), TestID: "txnlink-mode-bill"},
			{Value: uistate.TxnLinkModeSub, Label: uistate.T("txnlink.modeSub"), TestID: "txnlink-mode-sub"},
		}, OnSelect: p.Change,
	})
}

type txnLinkSubscriptionPickerProps struct {
	Options  []txnlink.Option
	Selected string
	Change   func(string)
}

func txnLinkSubscriptionPicker(p txnLinkSubscriptionPickerProps) ui.Node {
	if len(p.Options) == 0 {
		return P(css.Class("muted"), Attr("data-testid", "txnlink-no-subs"), uistate.T("txnlink.noSubs"))
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Label(css.Class(tw.Text13, tw.TextDim), uistate.T("txnlink.subLabel")),
		uiw.SelectInput(uiw.SelectInputProps{
			Options: txnLinkOptions(p.Options, "txnlink.noneSub"), Selected: p.Selected, OnChange: p.Change,
			AriaLabel: uistate.T("txnlink.subLabel"), TestID: "txnlink-sub-select",
		}),
		P(css.Class("muted", tw.Text13, "txn-hint-flush"), uistate.T("txnlink.subHint")))
}

type txnLinkBillPickerProps struct {
	Options            []txnlink.Option
	Selected, Merchant string
	AutoLink           bool
	ShowAutoLink       bool
	Change             func(string)
	ToggleAutoLink     func()
}

func txnLinkBillPicker(p txnLinkBillPickerProps) ui.Node {
	// Register the checkbox hook before any empty-state or selection branch.
	autoLinkChange := OnChange(ui.UseEvent(p.ToggleAutoLink))
	if len(p.Options) == 0 {
		return P(css.Class("muted"), Attr("data-testid", "txnlink-no-debts"), uistate.T("txnlink.noDebts"))
	}
	return Div(css.Class(tw.FlexCol, tw.Gap15),
		Label(css.Class(tw.Text13, tw.TextDim), uistate.T("txnlink.billLabel")),
		uiw.SelectInput(uiw.SelectInputProps{
			Options: txnLinkOptions(p.Options, "txnlink.noneBill"), Selected: p.Selected, OnChange: p.Change,
			AriaLabel: uistate.T("txnlink.billLabel"), TestID: "txnlink-bill-select",
		}),
		P(css.Class("muted", tw.Text13, "txn-hint-flush"), uistate.T("txnlink.billHint")),
		If(p.ShowAutoLink, Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "txnlink-autolink"), autoLinkChange}, checkedAttr(p.AutoLink)...)...),
			Div(css.Class("row-main"),
				Span(uistate.T("txnlink.autoLink", p.Merchant)),
				Span(css.Class("row-meta", tw.TextDim), uistate.T("txnlink.autoLinkHint"))))))
}

// txnLinkOptions maps pure options to the UI's option nodes without changing order.
func txnLinkOptions(options []txnlink.Option, noneKey string) []uiw.SelectOption {
	out := []uiw.SelectOption{{Value: "", Label: uistate.T(noneKey)}}
	for _, option := range options {
		out = append(out, uiw.SelectOption{Value: option.Value, Label: option.Label})
	}
	return out
}

type txnLinkPreviewProps struct {
	Decision     txnlink.Decision
	AccountNames map[string]string
}

func txnLinkPreview(p txnLinkPreviewProps) ui.Node {
	if !p.Decision.Linked {
		return Fragment()
	}
	txn := p.Decision.Transaction
	args := []any{css.Class("txnlink-preview"), Attr("data-testid", "txnlink-preview"),
		Span(css.Class("txnlink-preview-label", tw.TextDim), uistate.T("txnlink.previewLabel"))}
	if p.Decision.HasBill {
		args = append(args, Span(css.Class("txnlink-chip"), uistate.T("txnlink.chipBill", p.AccountNames[txn.BillAccountID])))
	}
	if p.Decision.HasSubscription {
		args = append(args, Span(css.Class("txnlink-chip"), uistate.T("txnlink.chipSub", txn.SubscriptionName)))
	}
	return Div(args...)
}

type txnLinkFooterProps struct {
	Decision     txnlink.Decision
	Save, Cancel func()
}

func txnLinkFooter(p txnLinkFooterProps) ui.Node {
	cancelClick := OnClick(ui.UseEvent(Prevent(p.Cancel)))
	saveClick := OnClick(ui.UseEvent(Prevent(p.Save)))
	hint := Fragment()
	if p.Decision.HintKey != "" {
		hint = P(css.Class("muted", tw.Text13, "txn-hint-flush"), Attr("data-testid", p.Decision.HintTestID),
			uistate.T(p.Decision.HintKey))
	}
	args := []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txnlink-save"), saveClick, uistate.T(p.Decision.SaveKey)}
	if !p.Decision.Changed {
		args = append(args, Disabled(true), Attr("aria-disabled", "true"))
	}
	return Fragment(hint, Div(css.Class("modal-sticky-foot"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "txnlink-cancel"), cancelClick, uistate.T("action.cancel")),
		Button(args...)))
}

// txnLinkDesc is retained for the refund dialog; its display decision is pure.
func txnLinkDesc(t domain.Transaction) string {
	return txnlink.Description(t, uistate.T("transactions.uncategorized"))
}

// Keep private helper names used by the unchanged R2Q wasm regression tests.
var (
	bestTxnSubscription            = txnlink.BestSubscription
	txnLinkSubscriptionNames       = txnlink.SubscriptionNames
	txnLinkFilterSubscriptionKinds = txnlink.FilterSubscriptionKinds
)

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// ensureBillRule persists a rule that auto-links future transactions matching `phrase`
// (payee/description, contains) as bill payments toward accountID, unless an equivalent
// rule already exists. Returns whether a new rule was created.
func ensureBillRule(app *appstate.App, phrase, accountID string) bool {
	phrase = strings.TrimSpace(phrase)
	if app == nil || phrase == "" || accountID == "" {
		return false
	}
	for _, r := range app.Rules() {
		if r.SetBillAccountID == accountID && strings.EqualFold(strings.TrimSpace(r.Match), phrase) {
			return false // an equivalent rule already covers this merchant → account
		}
	}
	if err := app.PutRule(rules.Rule{ID: id.New(), Match: phrase, SetBillAccountID: accountID, Order: app.NextRuleOrder()}); err != nil {
		uistate.PostNotice(err.Error(), true)
		return false
	}
	return true
}

// TxnLinkBody is the body of the payment-link flip modal (mounted at the shell root
// by app.TxnLinkHost). It links one transaction to a liability (as a recurring BILL
// payment) and/or to a subscription (as a subscription payment), via a Bill /
// Subscription toggle. The row ⋯ menu opens it to the chosen mode; both links save
// together so the modal reads as "what this payment is for". Saving is the write
// step — nothing changes until the user clicks Save.
func TxnLinkBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	linkAtom := uistate.UseTxnLinkTarget()
	target := linkAtom.Get()

	// Resolve the target transaction. If it's gone (deleted underneath us), show a
	// gentle empty state rather than a broken form.
	var txn domain.Transaction
	found := false
	if app != nil {
		for _, t := range app.Transactions() {
			if t.ID == target.TxnID {
				txn, found = t, true
				break
			}
		}
	}

	// All hooks are called unconditionally, before any early return, so the hook order
	// is identical on every render (the GWC hooks rule) even if the transaction vanishes
	// underneath us mid-edit.
	mode := ui.UseState(target.Mode)
	billChoice := ui.UseState(txn.BillAccountID)
	subChoice := ui.UseState(txn.SubscriptionName)
	autoLink := ui.UseState(false)
	onSave := ui.UseEvent(Prevent(func() {
		t := txn
		t.BillAccountID = billChoice.Get()
		t.SubscriptionName = subChoice.Get()
		if err := app.PutTransaction(t); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		// "Auto-link future payments like this" — persist a rule matching this merchant
		// (payee, contains) that sets the bill account, so future/imported payments tie
		// to it automatically. Created once per (merchant, account) pair.
		ruleMade := false
		if autoLink.Get() && billChoice.Get() != "" {
			ruleMade = ensureBillRule(app, firstNonEmpty(txn.Payee, txn.Desc), billChoice.Get())
		}
		switch {
		case ruleMade:
			uistate.PostNotice(uistate.T("txnlink.savedWithRule"), false)
		case t.BillAccountID == "" && t.SubscriptionName == "":
			uistate.PostNotice(uistate.T("txnlink.cleared"), false)
		default:
			uistate.PostNotice(uistate.T("txnlink.saved"), false)
		}
		uistate.BumpDataRevision()
		linkAtom.Set(uistate.TxnLinkTarget{})
	}))
	onCancel := ui.UseEvent(Prevent(func() { linkAtom.Set(uistate.TxnLinkTarget{}) }))
	onToggleAutoLink := ui.UseEvent(func() { autoLink.Set(!autoLink.Get()) })

	if !found {
		return Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted"), Attr("data-testid", "txnlink-missing"), uistate.T("txnlink.missing")),
			Div(css.Class("modal-sticky-foot"),
				Button(css.Class("btn"), Type("button"), OnClick(onCancel), uistate.T("action.close"))))
	}

	// Bill-payment targets: ANY non-archived account (not just liabilities) — a bill can
	// be paid toward any account the user tracks. Also build an id→name map for the
	// summary's account line and the effect preview.
	var billAccts []domain.Account
	acctName := make(map[string]string)
	for _, ac := range app.Accounts() {
		acctName[ac.ID] = ac.Name
		if !ac.Archived {
			billAccts = append(billAccts, ac)
		}
	}

	// Detected subscriptions (by name) — the subscription-payment targets. Detection
	// mirrors the /subscriptions page so the names line up with what the user sees.
	base := "USD"
	if b := app.Settings().BaseCurrency; b != "" {
		base = b
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	detected, _ := subscriptions.Detect(app.Transactions(), rates, uistate.LoadSubsDetectPrefs().MinOccurrencesOrDefault())
	// De-dupe names (a subscription can surface more than once across currencies) while
	// preserving detection order.
	seenSub := map[string]bool{}
	var subNames []string
	for _, s := range detected {
		if s.Name == "" || seenSub[s.Name] {
			continue
		}
		seenSub[s.Name] = true
		subNames = append(subNames, s.Name)
	}
	// If the txn is already linked to a subscription that no longer detects (e.g. too
	// few occurrences), keep its name selectable so the link is still editable.
	if txn.SubscriptionName != "" && !seenSub[txn.SubscriptionName] {
		subNames = append([]string{txn.SubscriptionName}, subNames...)
	}

	// Mode toggle: Bill payment ↔ Subscription. Both links persist on Save; the toggle
	// just chooses which picker is shown.
	seg := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("txnlink.modeLabel"),
		Selected: mode.Get(),
		Options: []uiw.SegOption{
			{Value: uistate.TxnLinkModeBill, Label: uistate.T("txnlink.modeBill"), TestID: "txnlink-mode-bill"},
			{Value: uistate.TxnLinkModeSub, Label: uistate.T("txnlink.modeSub"), TestID: "txnlink-mode-sub"},
		},
		OnSelect: func(m string) { mode.Set(m) },
	})

	// Transaction summary — the payment is the hero: description + date/account on the
	// left, the amount as a display figure on the right, so the user sees exactly what
	// they're linking at a glance.
	acctLine := txn.Date.Format("Jan 2, 2006")
	if an := acctName[txn.AccountID]; an != "" {
		acctLine += " · " + an
	}
	summary := Div(css.Class("txnlink-summary"), Attr("data-testid", "txnlink-summary"),
		Div(css.Class("txnlink-summary-main"),
			Span(css.Class("txnlink-summary-desc"), txnLinkDesc(txn)),
			Span(css.Class("txnlink-summary-meta", tw.TextDim), acctLine)),
		Span(css.Class("txnlink-summary-amount", tw.FontDisplay, tw.ColorClass(figTone(txn.Amount))), fmtMoney(txn.Amount)))

	var picker ui.Node
	if mode.Get() == uistate.TxnLinkModeSub {
		if len(subNames) == 0 {
			picker = P(css.Class("muted"), Attr("data-testid", "txnlink-no-subs"), uistate.T("txnlink.noSubs"))
		} else {
			opts := []uiw.SelectOption{{Value: "", Label: uistate.T("txnlink.noneSub")}}
			for _, n := range subNames {
				opts = append(opts, uiw.SelectOption{Value: n, Label: n})
			}
			picker = Div(css.Class(tw.FlexCol, tw.Gap15),
				Label(css.Class(tw.Text13, tw.TextDim), uistate.T("txnlink.subLabel")),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: opts, Selected: subChoice.Get(),
					OnChange:  func(v string) { subChoice.Set(v) },
					AriaLabel: uistate.T("txnlink.subLabel"), TestID: "txnlink-sub-select",
				}),
				P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), uistate.T("txnlink.subHint")))
		}
	} else {
		if len(billAccts) == 0 {
			picker = P(css.Class("muted"), Attr("data-testid", "txnlink-no-debts"), uistate.T("txnlink.noDebts"))
		} else {
			opts := []uiw.SelectOption{{Value: "", Label: uistate.T("txnlink.noneBill")}}
			for _, a := range billAccts {
				opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
			}
			picker = Div(css.Class(tw.FlexCol, tw.Gap15),
				Label(css.Class(tw.Text13, tw.TextDim), uistate.T("txnlink.billLabel")),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: opts, Selected: billChoice.Get(),
					OnChange:  func(v string) { billChoice.Set(v) },
					AriaLabel: uistate.T("txnlink.billLabel"), TestID: "txnlink-bill-select",
				}),
				P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), uistate.T("txnlink.billHint")),
				// Offer to remember this as a rule once a debt is chosen, so future
				// payments to the same merchant auto-link (also applied on import).
				If(billChoice.Get() != "", Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
					Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "txnlink-autolink"), OnChange(onToggleAutoLink)}, checkedAttr(autoLink.Get())...)...),
					Div(css.Class("row-main"),
						Span(uistate.T("txnlink.autoLink", txnLinkMerchant(txn))),
						Span(css.Class("row-meta", tw.TextDim), uistate.T("txnlink.autoLinkHint"))))))
		}
	}

	// Live "will link to" preview — both a bill and a subscription link save together,
	// so echo every pending link as a chip regardless of the active tab. This makes the
	// commit transparent: the user sees exactly what Save will do.
	preview := Fragment()
	previewArgs := []any{css.Class("txnlink-preview"), Attr("data-testid", "txnlink-preview"),
		Span(css.Class("txnlink-preview-label", tw.TextDim), uistate.T("txnlink.previewLabel"))}
	linked := false
	if aid := billChoice.Get(); aid != "" {
		previewArgs = append(previewArgs, Span(css.Class("txnlink-chip"), uistate.T("txnlink.chipBill", acctName[aid])))
		linked = true
	}
	if name := subChoice.Get(); name != "" {
		previewArgs = append(previewArgs, Span(css.Class("txnlink-chip"), uistate.T("txnlink.chipSub", name)))
		linked = true
	}
	if linked {
		preview = Div(previewArgs...)
	}

	return Div(css.Class(tw.FlexCol, tw.Gap3),
		summary,
		seg,
		picker,
		preview,
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txnlink-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txnlink-save"), OnClick(onSave), uistate.T("txnlink.save"))),
	)
}

// txnLinkDesc is the transaction's display description for the link modal summary,
// preferring the description and falling back to the payee.
func txnLinkDesc(t domain.Transaction) string {
	if d := t.Desc; d != "" {
		return d
	}
	if t.Payee != "" {
		return t.Payee
	}
	return uistate.T("transactions.uncategorized")
}

// txnLinkMerchant is the phrase the auto-link rule matches on — the payee, falling back
// to the description. Shown in the toggle label so the user sees exactly what "like
// this" means.
func txnLinkMerchant(t domain.Transaction) string {
	if m := strings.TrimSpace(firstNonEmpty(t.Payee, t.Desc)); m != "" {
		return m
	}
	return uistate.T("transactions.uncategorized")
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// RefundPairBody is the body of the refund-pairing flip modal (mounted at the
// shell root by app.RefundPairHost). It pairs a positive (money-in) transaction
// to the original purchase it refunds (XC2): pick a candidate original — same
// payee, at least the refund amount, within 90 days — and Save writes a
// refund-pair link. Budgets and reports then net the refund in the purchase's
// month; the ledger keeps both transactions untouched.
func RefundPairBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	targetAtom := uistate.UseRefundPairTarget()
	refundID := targetAtom.Get()

	// Resolve the refund transaction (hooks run before any early return).
	var refund domain.Transaction
	found := false
	var all []domain.TxnLink
	var txns []domain.Transaction
	if app != nil {
		txns = app.Transactions()
		all = app.TxnLinks()
		for _, t := range txns {
			if t.ID == refundID {
				refund, found = t, true
				break
			}
		}
	}

	candidates := txnlinks.RefundCandidates(refund, txns, all)
	choice := ui.UseState("")
	// Effective selection defaults to the top-ranked candidate so a single click
	// confirms the obvious match (without mutating state during render).
	effective := choice.Get()
	if effective == "" && len(candidates) > 0 {
		effective = candidates[0].ID
	}

	onCancel := ui.UseEvent(Prevent(func() { targetAtom.Set("") }))
	onSave := ui.UseEvent(Prevent(func() {
		origID := choice.Get()
		if origID == "" && len(candidates) > 0 {
			origID = candidates[0].ID
		}
		if origID == "" {
			return
		}
		link := domain.TxnLink{
			Kind:   domain.TxnLinkRefundPair,
			TxnIDs: []string{origID, refund.ID},
			Amount: refund.Amount.Abs(), // the refund's value nets against the original
		}
		if err := app.PutTxnLink(link); err != nil {
			uistate.PostNotice(uistate.T("txnlinks.pairErr", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("txnlinks.paired"), false)
		uistate.BumpDataRevision()
		targetAtom.Set("")
	}))

	if !found {
		return Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted"), Attr("data-testid", "refundpair-missing"), uistate.T("txnlinks.pairMissing")),
			Div(css.Class("modal-sticky-foot"),
				Button(css.Class("btn"), Type("button"), OnClick(onCancel), uistate.T("action.close"))))
	}

	acctName := map[string]string{}
	for _, ac := range app.Accounts() {
		acctName[ac.ID] = ac.Name
	}

	// The refund is the hero: description + date on the left, the amount on the right.
	acctLine := refund.Date.Format("Jan 2, 2006")
	if an := acctName[refund.AccountID]; an != "" {
		acctLine += " · " + an
	}
	summary := Div(css.Class("txnlink-summary"), Attr("data-testid", "refundpair-summary"),
		Div(css.Class("txnlink-summary-main"),
			Span(css.Class("txnlink-summary-desc"), txnLinkDesc(refund)),
			Span(css.Class("txnlink-summary-meta", tw.TextDim), acctLine)),
		Span(css.Class("txnlink-summary-amount", tw.FontDisplay, tw.ColorClass(figTone(refund.Amount))), fmtMoney(refund.Amount)))

	var body ui.Node
	if len(candidates) == 0 {
		body = P(css.Class("muted"), Attr("data-testid", "refundpair-none"), uistate.T("txnlinks.pairNoneFound"))
	} else {
		opts := make([]uiw.SelectOption, 0, len(candidates))
		for _, c := range candidates {
			opts = append(opts, uiw.SelectOption{Value: c.ID, Label: refundCandidateLabel(c)})
		}
		body = Div(css.Class(tw.FlexCol, tw.Gap15),
			Label(css.Class(tw.Text13, tw.TextDim), uistate.T("txnlinks.pairChoose")),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: opts, Selected: effective,
				OnChange:  func(v string) { choice.Set(v) },
				AriaLabel: uistate.T("txnlinks.pairChoose"), TestID: "refundpair-select",
			}))
	}

	return Div(css.Class(tw.FlexCol, tw.Gap3),
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), uistate.T("txnlinks.pairIntro")),
		summary,
		body,
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "refundpair-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			If(len(candidates) > 0, Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "refundpair-save"), OnClick(onSave), uistate.T("txnlinks.pairConfirm")))),
	)
}

// refundCandidateLabel is a candidate original purchase's picker label: its
// description (or payee) plus date and amount, so the user recognizes the buy.
func refundCandidateLabel(t domain.Transaction) string {
	return txnLinkDesc(t) + " · " + t.Date.Format("Jan 2") + " · " + fmtMoney(t.Amount)
}

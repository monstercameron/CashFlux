// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
	"github.com/monstercameron/CashFlux/internal/ordermatch"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
	"strings"
)

// orderImportCardProps carries nothing yet — the card reads appstate.Default like
// the other Documents cards. It is a struct so the component signature matches the
// framework's CreateElement(component, props) shape.
type orderImportCardProps struct{}

// OrderImportCard is the "Amazon order history" import card on /documents (TX4).
// It takes a privacy-export CSV (file) or a paste from the orders page, parses the
// orders locally (no network), matches each order to the card transaction(s) that
// paid for it, and lists the results with a per-order Apply:
//   - a single-charge match offers the receiptsplit-built proposed split (opens the
//     shipped split editor pre-filled from the order's line items);
//   - a multi-charge match proposes an XC1 order group over the matched charges.
//
// Unmatched orders and gift-card/promo drift are stated plainly on the row, never
// hidden. All parsing is local — the copy says so.
func OrderImportCard(_ orderImportCardProps) ui.Node {
	app := appstate.Default
	pasteText := ui.UseState("")
	matches := ui.UseState([]ordermatch.ReviewMatch(nil))
	msg := ui.UseState("")

	base := "USD"
	if app != nil && app.Settings().BaseCurrency != "" {
		base = app.Settings().BaseCurrency
	}

	runMatch := func(orders []orderimport.Order) {
		if app == nil {
			return
		}
		if len(orders) == 0 {
			matches.Set(nil)
			msg.Set(uistate.T("orderimport.noneParsed"))
			return
		}
		ms := orderimport.MatchOrders(orders, orderChargesFromLedger(app))
		matches.Set(ordermatch.ForReview(ms, app.Transactions()))
		msg.Set(uistate.T("orderimport.parsed", len(orders)))
	}

	onFindPaste := func() {
		if strings.TrimSpace(pasteText.Get()) == "" {
			matches.Set(nil)
			msg.Set(uistate.T("orderimport.emptyPaste"))
			return
		}
		runMatch(orderimport.ParseOrdersPaste(pasteText.Get(), base))
	}
	onPasteInput := func(v string) { pasteText.Set(v) }
	onChooseFile := func() {
		pickFileLimited(".csv,text/csv", maxImportCSVBytes, func(_, limit int) {
			msg.Set(uistate.T("impc.fileTooLarge", "CSV", fileLimitLabel(limit)))
		}, func(_ string, _ string, data []byte) {
			if err := validateCSVRowLimit(data, maxImportRows); err != nil {
				msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
				return
			}
			orders, err := orderimport.ParseRetailCSV(string(data), base)
			if err != nil {
				msg.Set(err.Error())
				return
			}
			runMatch(orders)
		})
	}

	ms := matches.Get()
	return Div(css.Class("doc-form-body"), Attr("data-testid", "order-import-card"),
		ui.CreateElement(orderImportInput, orderImportInputProps{onChooseFile: onChooseFile, onFindPaste: onFindPaste, onPasteInput: onPasteInput}),
		P(css.Class("muted", tw.Text12), uistate.T("orderimport.localNote")),
		If(msg.Get() != "", P(css.Class("muted"), Attr("data-testid", "order-import-msg"), msg.Get())),
		If(len(ms) > 0,
			Div(css.Class(tw.Mt2), Attr("role", "list"),
				MapKeyed(ms,
					func(m ordermatch.ReviewMatch) any { return m.Match.Order.ID },
					func(m ordermatch.ReviewMatch) ui.Node {
						return ui.CreateElement(orderImportRow, orderImportRowProps{
							Review: m, Base: base,
							OnApply: func() { applyOrderMatch(app, m.Match) },
						})
					},
				),
			),
		),
	)
}

// findTxnByID returns the ledger transaction with the given id.
func findTxnByID(app *appstate.App, id string) (domain.Transaction, bool) {
	return importdraft.FindTransaction(app.Transactions(), id)
}

// orderChargesFromLedger builds the matcher's candidate charges from the ledger's
// expenses (transfers and income excluded).
func orderChargesFromLedger(app *appstate.App) []orderimport.Charge {
	return importdraft.OrderCharges(app.Transactions())
}

// applyOrderMatch runs the per-order Apply: a single-charge match opens the split
// editor pre-filled from the order items; a multi-charge match creates an XC1 order
// group over the matched charges (preview-then-approve on the ledger).
func applyOrderMatch(app *appstate.App, m orderimport.Match) {
	if app == nil {
		return
	}
	switch m.Kind {
	case orderimport.MatchSingle:
		if len(m.TxnIDs) != 1 {
			return
		}
		txn, ok := findTxnByID(app, m.TxnIDs[0])
		if !ok {
			uistate.PostNotice(uistate.T("orderimport.txnGone"), true)
			return
		}
		proposeSplitFromOrder(app, txn, m.Order)
	case orderimport.MatchMulti:
		link := domain.TxnLink{
			Kind:         domain.TxnLinkOrderGroup,
			TxnIDs:       m.TxnIDs,
			EnteredTotal: money.New(m.Order.TotalMinor, m.Order.Currency),
			Note:         m.Order.ID,
		}
		if err := app.PutTxnLink(link); err != nil {
			uistate.PostNotice(uistate.T("orderimport.groupErr", err.Error()), true)
			return
		}
		uistate.PostNotice(uistate.T("orderimport.grouped", len(m.TxnIDs)), false)
		uistate.BumpDataRevision()
	default:
		uistate.PostNotice(uistate.T("orderimport.noMatch"), true)
	}
}

// proposeSplitFromOrder turns an order's line items into a proposed category split
// for the matched transaction (reusing receiptsplit.Propose) and opens the split
// editor pre-filled. Categories come from the auto-categorization rules on each
// item name; tax/shipping/gift-card drift auto-balances onto the txn's own category.
func proposeSplitFromOrder(app *appstate.App, txn domain.Transaction, o orderimport.Order) {
	proposal, key := importdraft.OrderSplit(txn, o, app.Categories(), app.Rules())
	if key != "" {
		uistate.PostNotice(uistate.T(key), true)
		return
	}
	uistate.SetTxnSplitProposal(txn.ID, proposal.Splits, proposal.Note)
}

// orderImportRowProps drives one matched-order row. OnApply is a plain callback so
// the row (its own component) owns the click hook — never an On* option in a loop.
type orderImportRowProps struct {
	Review  ordermatch.ReviewMatch
	Base    string
	OnApply func()
}

// orderImportRow renders one parsed order: its id, total, the match verdict
// (single / multi-shipment / unmatched), any amount difference stated without guessing,
// and an Apply button (enabled only when a match exists).
func orderImportRow(props orderImportRowProps) ui.Node {
	review := props.Review
	m := review.Match
	cur := m.Order.Currency
	if cur == "" {
		cur = props.Base
	}
	verdict, chargeLabel, canApply := importdraft.OrderText(review, uistate.T)
	items := make([]ui.Node, 0, len(m.Order.Items))
	for _, item := range m.Order.Items {
		items = append(items, Div(css.Class("muted", tw.Text12),
			Span(importdraft.OrderItemLabel(item, cur, uistate.T)),
		))
	}
	return Div(css.Class("doc-order-row", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
		Attr("role", "listitem"), Attr("data-testid", "order-row-"+m.Order.ID),
		Div(css.Class(tw.Flex1),
			Div(Strong(m.Order.ID), Span(css.Class("muted"), " "+fmtMoney(money.New(m.Order.TotalMinor, cur)))),
			Div(css.Class("muted", tw.Text12),
				Span(verdict),
				If(m.Kind != orderimport.MatchNone && m.DriftMinor != 0,
					Span(". "+uistate.T("orderimport.amountDifference", importdraft.MagnitudeText(m.DriftMinor, cur)))),
			),
			If(len(review.MissingTxnIDs) > 0,
				P(css.Class("muted", tw.Text12), uistate.T("orderimport.chargeMissing"))),
			If(len(review.Charges) > 0,
				Div(css.Class(tw.Mt1),
					Strong(uistate.T(chargeLabel)),
					Div(Map(review.Charges, func(txn domain.Transaction) ui.Node {
						payee := importdraft.ChargePayee(txn)
						return Div(css.Class("muted", tw.Text12),
							Span(txn.Date.Format("Jan 2, 2006")), Span(" — "), Span(payee), Span(" — "), Span(fmtMoney(txn.Amount)))
					})),
				),
			),
			If(m.Kind != orderimport.MatchNone,
				Div(css.Class(tw.Mt1),
					Strong(uistate.T("orderimport.itemsLabel")),
					IfElse(len(items) > 0, Div(items), P(css.Class("muted", tw.Text12), uistate.T("orderimport.noOrderItems"))),
				),
			),
		),
		If(canApply,
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "order-apply-"+m.Order.ID),
				OnClick(props.OnApply), uistate.T("orderimport.apply")),
		),
	)
}

type orderImportInputProps struct {
	onChooseFile, onFindPaste func()
	onPasteInput              func(string)
}

// orderImportInput owns the local order file and paste controls.
func orderImportInput(props orderImportInputProps) ui.Node {
	return Fragment(Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "order-file-picker"),
			OnClick(props.onChooseFile), uistate.T("orderimport.chooseFile")),
		Span(css.Class("muted", tw.Text13), uistate.T("orderimport.fileOrPaste")),
	),
		Form(Attr("data-testid", "order-paste-form"), OnSubmit(Prevent(props.onFindPaste)),
			Div(css.Class("form-grid"), Style(map[string]string{"margin-bottom": "0.5rem"}),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("orderimport.findOrders")),
			),
			Textarea(css.Class("field field-wide"), Attr("rows", "5"),
				Attr("id", "order-paste"),
				Attr("data-testid", "order-paste"),
				Placeholder(uistate.T("orderimport.pastePlaceholder")),
				OnInput(props.onPasteInput),
			),
		))
}

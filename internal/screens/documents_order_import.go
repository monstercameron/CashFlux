// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
	"github.com/monstercameron/CashFlux/internal/receiptsplit"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	matches := ui.UseState([]orderimport.Match(nil))
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
		matches.Set(ms)
		msg.Set(uistate.T("orderimport.parsed", len(orders)))
	}

	onFindPaste := ui.UseEvent(Prevent(func() {
		runMatch(orderimport.ParseOrdersPaste(pasteText.Get(), base))
	}))
	onPasteInput := ui.UseEvent(func(v string) { pasteText.Set(v) })
	onChooseFile := ui.UseEvent(func() {
		pickFile(".csv,text/csv", func(_ string, _ string, data []byte) {
			orders, err := orderimport.ParseRetailCSV(string(data), base)
			if err != nil {
				msg.Set(err.Error())
				return
			}
			runMatch(orders)
		})
	})

	ms := matches.Get()
	return Div(css.Class("doc-form-body"), Attr("data-testid", "order-import-card"),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "order-file-picker"),
				OnClick(onChooseFile), uistate.T("orderimport.chooseFile")),
			Span(css.Class("muted", tw.Text13), uistate.T("orderimport.fileOrPaste")),
		),
		Form(OnSubmit(onFindPaste),
			Div(css.Class("form-grid"), Style(map[string]string{"margin-bottom": "0.5rem"}),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("orderimport.findOrders")),
			),
			Textarea(css.Class("field field-wide"), Attr("rows", "5"),
				Attr("data-testid", "order-paste"),
				Placeholder(uistate.T("orderimport.pastePlaceholder")),
				OnInput(onPasteInput),
			),
		),
		P(css.Class("muted", tw.Text12), uistate.T("orderimport.localNote")),
		If(msg.Get() != "", P(css.Class("muted"), Attr("data-testid", "order-import-msg"), msg.Get())),
		If(len(ms) > 0,
			Div(css.Class(tw.Mt2), Attr("role", "list"),
				MapKeyed(ms,
					func(m orderimport.Match) any { return m.Order.ID },
					func(m orderimport.Match) ui.Node {
						return ui.CreateElement(orderImportRow, orderImportRowProps{
							Match: m, Base: base,
							OnApply: func() { applyOrderMatch(app, m) },
						})
					},
				),
			),
		),
	)
}

// findTxnByID returns the ledger transaction with the given id.
func findTxnByID(app *appstate.App, id string) (domain.Transaction, bool) {
	for _, t := range app.Transactions() {
		if t.ID == id {
			return t, true
		}
	}
	return domain.Transaction{}, false
}

// orderChargesFromLedger builds the matcher's candidate charges from the ledger's
// expenses (transfers and income excluded).
func orderChargesFromLedger(app *appstate.App) []orderimport.Charge {
	var out []orderimport.Charge
	for _, t := range app.Transactions() {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		out = append(out, orderimport.Charge{
			TxnID: t.ID, Date: t.Date, AmountMinor: t.Amount.Amount, Currency: t.Amount.Currency,
		})
	}
	return out
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
	cur := txn.Amount.Currency
	if cur == "" {
		cur = o.Currency
	}
	cats := app.Categories()
	appRules := app.Rules()

	lineCat := make(map[string]string, len(o.Items))
	items := make([]receiptsplit.LineItem, 0, len(o.Items))
	for _, it := range o.Items {
		if it.Name == "" {
			continue
		}
		cid := resolveExpenseCategoryName(cats, it.Name)
		if cid == "" {
			if mrule := rules.FirstMatch(appRules, it.Name); mrule != nil {
				cid = mrule.SetCategoryID
			}
		}
		lineCat[it.Name] = cid
		items = append(items, receiptsplit.LineItem{Name: it.Name, Amount: money.New(it.LineTotalMinor(), cur)})
	}
	if len(items) == 0 {
		uistate.PostNotice(uistate.T("orderimport.noItems"), true)
		return
	}
	match := func(name string) string { return lineCat[name] }
	proposal, ok := receiptsplit.Propose(items, receiptsplit.Target{
		Amount: txn.Amount, CategoryID: txn.CategoryID,
	}, match)
	if !ok {
		uistate.PostNotice(uistate.T("receiptsplit.noProposal"), true)
		return
	}
	uistate.SetTxnSplitProposal(txn.ID, proposal.Splits, proposal.Note)
}

// orderImportRowProps drives one matched-order row. OnApply is a plain callback so
// the row (its own component) owns the click hook — never an On* option in a loop.
type orderImportRowProps struct {
	Match   orderimport.Match
	Base    string
	OnApply func()
}

// orderImportRow renders one parsed order: its id, total, the match verdict
// (single / multi-shipment / unmatched), any gift-card/promo drift stated plainly,
// and an Apply button (enabled only when a match exists).
func orderImportRow(props orderImportRowProps) ui.Node {
	m := props.Match
	cur := m.Order.Currency
	if cur == "" {
		cur = props.Base
	}
	var verdict string
	switch m.Kind {
	case orderimport.MatchSingle:
		verdict = uistate.T("orderimport.matchedSingle")
	case orderimport.MatchMulti:
		verdict = uistate.T("orderimport.matchedMulti", len(m.TxnIDs))
	default:
		verdict = uistate.T("orderimport.unmatched")
	}
	drift := m.DriftMinor
	if drift < 0 {
		drift = -drift
	}
	return Div(css.Class("doc-order-row", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
		Attr("role", "listitem"), Attr("data-testid", "order-row-"+m.Order.ID),
		Div(css.Class(tw.Flex1),
			Div(Strong(m.Order.ID), Span(css.Class("muted"), " "+fmtMoney(money.New(m.Order.TotalMinor, cur)))),
			Div(css.Class("muted", tw.Text12),
				Span(verdict),
				If(m.Kind != orderimport.MatchNone && m.DriftMinor != 0,
					Span(" · "+uistate.T("orderimport.drift", fmtMoney(money.New(drift, cur))))),
			),
		),
		If(m.Kind != orderimport.MatchNone,
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "order-apply-"+m.Order.ID),
				OnClick(props.OnApply), uistate.T("orderimport.apply")),
		),
	)
}

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Split is a self-contained "split a shared expense" calculator (B24): enter an
// amount, pick who's sharing it, and it shows each member's even share; pick who
// paid and it shows who owes them what. Backed by the pure internal/split core.
// (The transaction-level split + persisted settle-up build on the same core.)
func Split() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	members := app.Members()

	amountS := ui.UseState("")
	selected := ui.UseState(map[string]bool{})
	payerS := ui.UseState("")
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onPayer := ui.UseEvent(func(e ui.Event) { payerS.Set(e.GetValue()) })
	toggle := func(id string) {
		cur := selected.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[id] = !next[id]
		selected.Set(next)
	}

	amt, _ := money.ParseMinor(strings.TrimSpace(amountS.Get()), dec)
	var ids []string
	for _, m := range members {
		if selected.Get()[m.ID] {
			ids = append(ids, m.ID)
		}
	}
	shareByID := map[string]int64{}
	for _, s := range split.Equal(amt, ids) {
		shareByID[s.MemberID] = s.Amount
	}
	nameByID := map[string]string{}
	for _, m := range members {
		nameByID[m.ID] = m.Name
	}

	memberRows := MapKeyed(members,
		func(m domain.Member) any { return m.ID },
		func(m domain.Member) ui.Node {
			on := selected.Get()[m.ID]
			share := Fragment()
			if on && amt > 0 {
				share = Span(Class("budget-amount"), fmtMoney(money.New(shareByID[m.ID], base)))
			}
			id := m.ID
			return Div(Class("row"),
				uiw.ToggleRow(uiw.ToggleRowProps{Label: m.Name, On: on, OnChange: func(bool) { toggle(id) }}),
				share,
			)
		},
	)

	payerOpts := []ui.Node{Option(Value(""), SelectedIf(payerS.Get() == ""), uistate.T("split.noPayer"))}
	for _, m := range members {
		payerOpts = append(payerOpts, Option(Value(m.ID), SelectedIf(payerS.Get() == m.ID), m.Name))
	}

	// Settle-up: each sharer who isn't the payer owes the payer their share.
	var owes []ui.Node
	if payer := payerS.Get(); payer != "" && amt > 0 {
		for _, id := range ids {
			if id == payer {
				continue
			}
			owes = append(owes, Div(Class("row"),
				Span(Class("row-desc"), uistate.T("split.owes", nameByID[id], nameByID[payer])),
				Span(Class("budget-amount"), fmtMoney(money.New(shareByID[id], base))),
			))
		}
	}

	var memberBody ui.Node
	if len(members) == 0 {
		memberBody = P(Class("empty"), uistate.T("split.noMembers"))
	} else {
		memberBody = Div(Class("rows"), memberRows)
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.split")),
			P(Class("muted"), uistate.T("split.hint")),
			Div(Class("form-grid"),
				Input(Class("field"), Type("number"), Attr("aria-label", uistate.T("split.amount")), Placeholder(uistate.T("split.amount")), Value(amountS.Get()), Step("0.01"), OnInput(onAmount)),
				Select(Class("field"), Attr("aria-label", uistate.T("split.payer")), Title(uistate.T("split.payer")), OnChange(onPayer), payerOpts),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.members")),
			memberBody,
		),
		If(len(owes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.settleUp")),
			Div(Class("rows"), owes),
		)),
	)
}

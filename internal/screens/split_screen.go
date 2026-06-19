//go:build js && wasm

package screens

import (
	"strconv"
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
	weighted := ui.UseState(false)
	weights := ui.UseState(map[string]string{})
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
	setWeight := func(id, v string) {
		cur := weights.Get()
		next := make(map[string]string, len(cur)+1)
		for k, val := range cur {
			next[k] = val
		}
		next[id] = v
		weights.Set(next)
	}

	amt, _ := money.ParseMinor(strings.TrimSpace(amountS.Get()), dec)
	var ids []string
	for _, m := range members {
		if selected.Get()[m.ID] {
			ids = append(ids, m.ID)
		}
	}
	shareByID := map[string]int64{}
	if weighted.Get() {
		// Proportional split: a blank or invalid weight defaults to 1 (so a fresh
		// proportional split behaves like an even one until weights are set); an
		// explicit 0 excludes that member from the share.
		wm := make([]split.WeightedMember, 0, len(ids))
		for _, id := range ids {
			w := int64(1)
			if s := strings.TrimSpace(weights.Get()[id]); s != "" {
				if v, err := strconv.Atoi(s); err == nil {
					w = int64(v)
				}
			}
			wm = append(wm, split.WeightedMember{MemberID: id, Weight: w})
		}
		for _, s := range split.ByWeights(amt, wm) {
			shareByID[s.MemberID] = s.Amount
		}
	} else {
		for _, s := range split.Equal(amt, ids) {
			shareByID[s.MemberID] = s.Amount
		}
	}
	nameByID := map[string]string{}
	for _, m := range members {
		nameByID[m.ID] = m.Name
	}

	memberRows := MapKeyed(members,
		func(m domain.Member) any { return m.ID },
		func(m domain.Member) ui.Node {
			on := selected.Get()[m.ID]
			share := ""
			if on && amt > 0 {
				share = fmtMoney(money.New(shareByID[m.ID], base))
			}
			return ui.CreateElement(SplitMemberRow, splitMemberRowProps{
				Member: m, On: on, Weighted: weighted.Get(), Weight: weights.Get()[m.ID],
				Share: share, OnToggle: toggle, OnWeight: setWeight,
			})
		},
	)

	payerOpts := []ui.Node{Option(Value(""), SelectedIf(payerS.Get() == ""), uistate.T("split.noPayer"))}
	for _, m := range members {
		payerOpts = append(payerOpts, Option(Value(m.ID), SelectedIf(payerS.Get() == m.ID), m.Name))
	}

	// Settle-up: each sharer who isn't the payer owes the payer their share.
	var owes []ui.Node
	var transfers []split.Transfer
	if payer := payerS.Get(); payer != "" && amt > 0 {
		for _, id := range ids {
			if id == payer {
				continue
			}
			owes = append(owes, Div(Class("row"),
				Span(Class("row-desc"), uistate.T("split.owes", nameByID[id], nameByID[payer])),
				Span(Class("budget-amount"), fmtMoney(money.New(shareByID[id], base))),
			))
			transfers = append(transfers, split.Transfer{From: id, To: payer, Amount: shareByID[id]})
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
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("split.byWeight"), On: weighted.Get(), OnChange: func(v bool) { weighted.Set(v) }}),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.members")),
			memberBody,
		),
		If(len(owes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.settleUp")),
			Div(Class("rows"), owes),
			Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("split.downloadCsvTitle")), OnClick(func() {
					nm := func(id string) string { return nameByID[id] }
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("settle-up.csv", "text/csv", split.CSV(transfers, nm, csvAmount))
				}), uistate.T("split.downloadCsv")),
			),
		)),
	)
}

type splitMemberRowProps struct {
	Member   domain.Member
	On       bool
	Weighted bool
	Weight   string // current weight input value (proportional mode)
	Share    string // pre-formatted share; "" hides the amount
	OnToggle func(id string)
	OnWeight func(id, value string)
}

// SplitMemberRow renders one member in the split picker: an include toggle, a
// weight input (only in proportional mode while included), and the computed
// share. It owns its weight-input hook (per the no-hooks-in-loops rule), so the
// member list can render many rows safely.
func SplitMemberRow(props splitMemberRowProps) ui.Node {
	m := props.Member
	onWeight := ui.UseEvent(func(v string) { props.OnWeight(m.ID, v) })

	weightField := Fragment()
	if props.Weighted && props.On {
		weightField = Input(Class("field"), Type("number"), Attr("aria-label", uistate.T("split.weight")),
			Placeholder(uistate.T("split.weight")), Value(props.Weight), Step("1"), OnInput(onWeight))
	}
	share := Fragment()
	if props.Share != "" {
		share = Span(Class("budget-amount"), props.Share)
	}
	return Div(Class("row"),
		uiw.ToggleRow(uiw.ToggleRowProps{Label: m.Name, On: props.On, OnChange: func(bool) { props.OnToggle(m.ID) }}),
		weightField,
		share,
	)
}

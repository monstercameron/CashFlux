// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Split is a self-contained "split a shared expense" calculator (B24): enter an
// amount, pick who's sharing it, and it shows each member's even share; pick who
// paid and it shows who owes them what. Backed by the pure internal/split core.
// (The transaction-level split + persisted settle-up build on the same core.)
func Split() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	members := app.Members()

	amountS := ui.UseState("")
	descS := ui.UseState("")
	selected := ui.UseState(map[string]bool{})
	// Seed the payer from the "View as" member (if one is active) so Priya
	// doesn't have to select herself separately (G12 §7, item 18).
	activeMember := uistate.UseActiveMember()
	initialPayer := activeMember.Get() // "" == everyone; a member ID seeds the payer
	payerS := ui.UseState(initialPayer)
	weighted := ui.UseState(false)
	weights := ui.UseState(map[string]string{})
	errS := ui.UseState("")
	rev := ui.UseState(0) // bumped after save/record so the settle-up ledger re-reads
	bump := func() { rev.Set(rev.Get() + 1) }
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
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
	selectAll := func() {
		all := make(map[string]bool, len(members))
		for _, m := range members {
			all[m.ID] = true
		}
		selected.Set(all)
	}
	clearAll := func() { selected.Set(map[string]bool{}) }
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

	// saveSplit records the current forward split as a persisted SharedExpense so it
	// joins the running settle-up ledger below.
	saveSplit := func() {
		if amt <= 0 || payerS.Get() == "" || len(ids) == 0 {
			errS.Set("Enter an amount, pick who paid, and who's sharing.")
			return
		}
		shares := make([]domain.SharedExpenseShare, 0, len(ids))
		for _, mid := range ids {
			shares = append(shares, domain.SharedExpenseShare{MemberID: mid, Amount: money.New(shareByID[mid], base)})
		}
		e := domain.SharedExpense{ID: id.New(), Desc: strings.TrimSpace(descS.Get()), Date: time.Now(), PayerID: payerS.Get(), Shares: shares}
		if err := app.PutSharedExpense(e); err != nil {
			errS.Set(err.Error())
			return
		}
		amountS.Set("")
		descS.Set("")
		selected.Set(map[string]bool{})
		payerS.Set("")
		errS.Set("")
		bump()
	}

	// recordSettlement marks a suggested transfer as paid, persisting a Settlement
	// so the ledger re-balances.
	recordSettlement := func(from, to string, amount money.Money) {
		if err := app.RecordSettlement(domain.Settlement{ID: id.New(), FromID: from, ToID: to, Amount: amount, Date: time.Now()}); err != nil {
			errS.Set(err.Error())
			return
		}
		errS.Set("")
		bump()
	}

	// The persisted settle-up ledger across every saved shared expense.
	net, ledger := app.SettleUp(base)

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
		for _, mid := range ids {
			if mid == payer {
				continue
			}
			owes = append(owes, Div(css.Class("row"),
				Span(css.Class("row-desc"), uistate.T("split.owes", nameByID[mid], nameByID[payer])),
				Span(css.Class("budget-amount"), fmtMoney(money.New(shareByID[mid], base))),
			))
			transfers = append(transfers, split.Transfer{From: mid, To: payer, Amount: shareByID[mid]})
		}
	}

	// The persisted ledger: each member's running net, then the minimal payments.
	var netRows []ui.Node
	for _, m := range members {
		bal := net[m.ID]
		if bal.IsZero() {
			continue
		}
		label := m.Name + " is owed " + fmtMoney(bal)
		amtCls := "budget-amount"
		if bal.IsNegative() {
			label = m.Name + " owes " + fmtMoney(bal.Neg())
			amtCls = "budget-amount text-down"
		}
		netRows = append(netRows, Div(css.Class("row"),
			Span(css.Class("row-desc"), label),
			Span(ClassStr(amtCls), fmtMoney(bal.Abs())),
		))
	}
	var ledgerRows []ui.Node
	for _, tr := range ledger {
		ledgerRows = append(ledgerRows, ui.CreateElement(settleTransferRow, settleTransferRowProps{
			From: tr.From, To: tr.To, FromName: nameByID[tr.From], ToName: nameByID[tr.To],
			Amount: fmtMoney(tr.Amount), AmountRaw: tr.Amount, OnRecord: recordSettlement,
		}))
	}

	var memberBody ui.Node
	if len(members) == 0 {
		memberBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:  uistate.T("split.noMembers"),
			CTALabel: uistate.T("split.goToMembers"),
			Href:     uistate.RoutePath("/members"),
		})
	} else {
		memberBody = Div(css.Class("rows"), memberRows)
	}

	// Summary so the math is legible at a glance: amount, how many share it, and the
	// even per-person figure with any rounding remainder the core hands the first
	// sharer (C58). For a weighted split the per-person figure doesn't apply.
	var splitSummary ui.Node = Fragment()
	if n := len(ids); n > 0 && amt > 0 {
		if weighted.Get() {
			// The summary is the key computed output — highlight it, not muted (G12 §4).
			splitSummary = P(css.Class("split-summary", tw.FontDisplay), fmt.Sprintf("%s split among %d (weighted)", fmtMoney(money.New(amt, base)), n))
		} else {
			each := amt / int64(n)
			rem := amt - each*int64(n)
			s := fmt.Sprintf("%s split among %d → %s each", fmtMoney(money.New(amt, base)), n, fmtMoney(money.New(each, base)))
			if rem > 0 {
				s += fmt.Sprintf(" (+%s remainder to the first)", fmtMoney(money.New(rem, base)))
			}
			splitSummary = P(css.Class("split-summary", tw.FontDisplay), s)
		}
	}
	// Select-all / clear for households with several members.
	var memberControls ui.Node = Fragment()
	if len(members) > 1 {
		memberControls = Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(css.Class("btn"), Type("button"), OnClick(selectAll), uistate.T("split.selectAll")),
			// "Clear" is the destructive (deselect-all) counterpart to the additive
			// "Select all" — a ghost-danger style distinguishes the two (G12 §2).
			Button(css.Class("btn btn-ghost-danger"), Type("button"), OnClick(clearAll), uistate.T("split.clear")),
		)
	}

	return Div(
		rptSection("sec-split-calc", uistate.T("nav.split"), nil,
			Fragment(
				P(css.Class("muted"), uistate.T("split.hint")),
				Div(css.Class("form-grid"),
					Input(css.Class("field"), Type("number"), Attr("aria-label", uistate.T("split.amount")), Placeholder(uistate.T("split.amount")), Value(amountS.Get()), Step("0.01"), OnInput(onAmount)),
					Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("split.whatForLabel")), Placeholder(uistate.T("split.whatForLabel")), Value(descS.Get()), OnInput(onDesc)),
					Select(css.Class("field"), Attr("aria-label", uistate.T("split.payer")), Title(uistate.T("split.payer")), OnChange(onPayer), payerOpts),
				),
				uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("split.byWeight"), On: weighted.Get(), OnChange: func(v bool) { weighted.Set(v) }}),
				errText("split-err", errS.Get()),
			),
		),
		rptSection("sec-split-members", uistate.T("split.members"), nil,
			Fragment(
				memberControls,
				memberBody,
				splitSummary,
			),
		),
		// Forward hint: show a placeholder prompt when the user has entered an amount +
		// selected members but hasn't chosen who paid — so Priya sees where the result
		// will appear before she completes the form (G12 §5).
		If(len(ids) > 0 && amt > 0 && payerS.Get() == "",
			P(css.Class("muted"), uistate.T("split.pickPayerHint")),
		),
		// "This split" — ephemeral card; only visible when a payer + members + amount are
		// all set. Title distinguishes it from the persisted "Running balance" card (G12 §7).
		If(len(owes) > 0, rptSection("sec-split-this", uistate.T("split.thisSplit"), nil,
			Fragment(
				Div(css.Class("rows"), owes),
				// Who-owes-whom as a Mermaid digraph (C70): debtor → payer, labelled.
				uiw.Mermaid(uiw.MermaidProps{
					Source: mermaid.FromSettleUp(transfers,
						func(id string) string { return nameByID[id] },
						func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }),
					Label: uistate.T("split.whoOwesWhom"),
					Class: tw.Fold(tw.Mt2),
				}),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
					Button(css.Class("btn btn-primary"), Type("button"), Title(uistate.T("split.saveSplitTitle")), OnClick(saveSplit), uistate.T("split.saveSplit")),
					Button(css.Class("btn"), Type("button"), Title(uistate.T("split.downloadCsvTitle")), OnClick(func() {
						nm := func(id string) string { return nameByID[id] }
						csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
						downloadBytes("settle-up.csv", "text/csv", split.CSV(transfers, nm, csvAmount))
					}), uistate.T("split.downloadCsv")),
				),
			),
		)),
		// Persisted running balance — always shown (with empty-state when no splits yet),
		// titled "Running balance" to distinguish it from the ephemeral "This split" card
		// above (G12 §7 item 19; G12 §1 item 3 empty-state).
		rptSection("sec-split-balance", uistate.T("split.runningBalance"), nil,
			Fragment(
				P(css.Class("muted"), uistate.T("split.runningBalanceHint")),
				If(len(net) == 0,
					P(css.Class("muted"), uistate.T("split.runningBalanceEmpty")),
				),
				If(len(net) > 0, Fragment(
					Div(css.Class("rows"), netRows),
					If(len(netRows) > 0 && len(ledgerRows) > 0, Div(
						P(css.Class("budget-sub"), uistate.T("split.squareUpHint")),
						Div(css.Class("rows"), ledgerRows),
					)),
					If(len(netRows) == 0, P(css.Class("muted"), uistate.T("split.allSettled"))),
				)),
			),
		),
	)
}

type settleTransferRowProps struct {
	From, To         string // member IDs
	FromName, ToName string
	Amount           string      // pre-formatted
	AmountRaw        money.Money // for recording the settlement
	OnRecord         func(from, to string, amount money.Money)
}

// settleTransferRow renders one suggested "X pays Y $Z" payment with a button to
// record it as settled. Own component (per the no-hooks-in-loops rule) so the
// ledger can list many rows safely.
func settleTransferRow(props settleTransferRowProps) ui.Node {
	onRec := ui.UseEvent(Prevent(func() { props.OnRecord(props.From, props.To, props.AmountRaw) }))
	// Button names the specific transfer so Priya can't click the wrong one when
	// several are listed (G12 §7 item 20).
	btnLabel := "Record: " + props.FromName + " pays " + props.ToName + " " + props.Amount
	return Div(css.Class("row"),
		Span(css.Class("row-desc"), props.FromName+" pays "+props.ToName),
		Span(css.Class("budget-amount"), props.Amount),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("split.recordSettledTitle")), OnClick(onRec), btnLabel),
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
		weightField = Input(css.Class("field"), Type("number"), Attr("aria-label", uistate.T("split.weight")),
			Placeholder(uistate.T("split.weight")), Value(props.Weight), Step("1"), OnInput(onWeight))
	}
	share := Fragment()
	if props.Share != "" {
		share = Span(css.Class("budget-amount"), props.Share)
	}
	return Div(css.Class("row", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Div(css.Class(tw.Flex1),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: m.Name, On: props.On, OnChange: func(bool) { props.OnToggle(m.ID) }}),
		),
		weightField,
		share,
	)
}

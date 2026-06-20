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
	descS := ui.UseState("")
	selected := ui.UseState(map[string]bool{})
	payerS := ui.UseState("")
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
			owes = append(owes, Div(Class("row"),
				Span(Class("row-desc"), uistate.T("split.owes", nameByID[mid], nameByID[payer])),
				Span(Class("budget-amount"), fmtMoney(money.New(shareByID[mid], base))),
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
		netRows = append(netRows, Div(Class("row"),
			Span(Class("row-desc"), label),
			Span(Class(amtCls), fmtMoney(bal.Abs())),
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
		memberBody = P(Class("empty"), uistate.T("split.noMembers"))
	} else {
		memberBody = Div(Class("rows"), memberRows)
	}

	// Summary so the math is legible at a glance: amount, how many share it, and the
	// even per-person figure with any rounding remainder the core hands the first
	// sharer (C58). For a weighted split the per-person figure doesn't apply.
	var splitSummary ui.Node = Fragment()
	if n := len(ids); n > 0 && amt > 0 {
		if weighted.Get() {
			splitSummary = P(Class("muted"), fmt.Sprintf("%s split among %d (weighted)", fmtMoney(money.New(amt, base)), n))
		} else {
			each := amt / int64(n)
			rem := amt - each*int64(n)
			s := fmt.Sprintf("%s split among %d → %s each", fmtMoney(money.New(amt, base)), n, fmtMoney(money.New(each, base)))
			if rem > 0 {
				s += fmt.Sprintf(" (+%s remainder to the first)", fmtMoney(money.New(rem, base)))
			}
			splitSummary = P(Class("muted"), s)
		}
	}
	// Select-all / clear for households with several members.
	var memberControls ui.Node = Fragment()
	if len(members) > 1 {
		memberControls = Div(Class("flex flex-wrap gap-2 items-center"), Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(Class("btn"), Type("button"), OnClick(selectAll), "Select all"),
			Button(Class("btn"), Type("button"), OnClick(clearAll), "Clear"),
		)
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.split")),
			P(Class("muted"), uistate.T("split.hint")),
			Div(Class("form-grid"),
				Input(Class("field"), Type("number"), Attr("aria-label", uistate.T("split.amount")), Placeholder(uistate.T("split.amount")), Value(amountS.Get()), Step("0.01"), OnInput(onAmount)),
				Input(Class("field"), Type("text"), Attr("aria-label", "What was it for? (optional)"), Placeholder("What was it for? (optional)"), Value(descS.Get()), OnInput(onDesc)),
				Select(Class("field"), Attr("aria-label", uistate.T("split.payer")), Title(uistate.T("split.payer")), OnChange(onPayer), payerOpts),
			),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("split.byWeight"), On: weighted.Get(), OnChange: func(v bool) { weighted.Set(v) }}),
			errText("split-err", errS.Get()),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.members")),
			memberControls,
			memberBody,
			splitSummary,
		),
		If(len(owes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("split.settleUp")),
			Div(Class("rows"), owes),
			// Who-owes-whom as a Mermaid digraph (C70): debtor → payer, labelled.
			uiw.Mermaid(uiw.MermaidProps{
				Source: mermaid.FromSettleUp(transfers,
					func(id string) string { return nameByID[id] },
					func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }),
				Label: "Who owes whom",
				Class: "mt-2",
			}),
			Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn btn-primary"), Type("button"), Title("Save this split to the settle-up ledger below"), OnClick(saveSplit), "Save split"),
				Button(Class("btn"), Type("button"), Title(uistate.T("split.downloadCsvTitle")), OnClick(func() {
					nm := func(id string) string { return nameByID[id] }
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("settle-up.csv", "text/csv", split.CSV(transfers, nm, csvAmount))
				}), uistate.T("split.downloadCsv")),
			),
		)),
		If(len(net) > 0, Section(Class("card"),
			H2(Class("card-title"), "Settle up"),
			P(Class("muted"), "Running balance across every saved split."),
			Div(Class("rows"), netRows),
			If(len(netRows) > 0 && len(ledgerRows) > 0, Div(
				P(Class("budget-sub"), "Simplest way to square up:"),
				Div(Class("rows"), ledgerRows),
			)),
			If(len(netRows) == 0, P(Class("muted"), "All settled up — nobody owes anybody.")),
		)),
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
	return Div(Class("row"),
		Span(Class("row-desc"), props.FromName+" pays "+props.ToName),
		Span(Class("budget-amount"), props.Amount),
		Button(Class("btn"), Type("button"), Title("Record this payment as settled"), OnClick(onRec), "Record settlement"),
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

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalAllocateFormProps drives the virtual-allocation ("earmark") modal.
type GoalAllocateFormProps struct {
	GoalID string
	OnDone func()
}

// GoalAllocateForm is the body of the shell-root flip modal for VIRTUAL ALLOCATION: it
// lets the user reserve part of specific accounts' EXISTING balances toward the goal
// without posting any transaction. A master toggle turns earmarking on; below it every
// account is a selectable row with its own amount, each capped at that account's free
// balance (its balance minus what other goals already earmark against it). Coverage =
// committed savings + earmarks, so a goal can read "funded" purely by reservation.
func GoalAllocateForm(props GoalAllocateFormProps) ui.Node {
	return ui.CreateElement(goalAllocateForm, props)
}

func goalAllocateForm(props GoalAllocateFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var g domain.Goal
	found := false
	if app != nil {
		for _, gg := range app.Goals() {
			if gg.ID == props.GoalID {
				g, found = gg, true
				break
			}
		}
	}

	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = base
	}
	dec := currency.Decimals(cur)

	// Seed the toggle + per-account selection/amount from the goal's existing earmarks.
	seedSel := map[string]bool{}
	seedAmt := map[string]string{}
	for _, al := range g.Allocations {
		seedSel[al.AccountID] = true
		seedAmt[al.AccountID] = money.FormatMinor(al.Amount.Amount, dec)
	}
	// Default the smart-split target to what still needs covering (target − committed savings).
	gapMinor := g.TargetAmount.Amount - g.CurrentAmount.Amount
	if gapMinor < 0 {
		gapMinor = 0
	}
	enabledS := ui.UseState(len(g.Allocations) > 0)
	selS := ui.UseState(seedSel)
	amountsS := ui.UseState(seedAmt)
	totalS := ui.UseState(money.FormatMinor(gapMinor, dec))
	errS := ui.UseState("")

	onToggleEnabled := ui.UseEvent(func(e ui.Event) { enabledS.Set(e.IsChecked()) })
	onTotal := ui.UseEvent(func(v string) { totalS.Set(v) })
	// Plain closures (not hooks) passed to each hook-owning row.
	onToggleAcct := func(acctID string) { selS.Set(toggleInSet(selS.Get(), acctID)) }
	onAmount := func(acctID, v string) {
		m := amountsS.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[acctID] = v
		amountsS.Set(nm)
	}
	cancel := ui.UseEvent(Prevent(func() { done() }))

	rates := currency.Rates{Base: base}
	var txns []domain.Transaction
	var accounts []domain.Account
	if app != nil {
		rates.Rates = app.Settings().FXRates
		txns = app.Transactions()
		accounts = app.Accounts()
	}
	// You earmark from money you can actually MOVE — only liquid cash accounts (checking /
	// debit / savings / cash), never liabilities, illiquid holdings (retirement, property,
	// vehicle, brokerage estimates), or archived accounts. Showing a 401(k) or home equity
	// as "free" to reserve toward a savings goal would be misleading. The picker and the
	// save loop both iterate this eligible set.
	eligible := make([]domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if !a.Archived && earmarkEligibleType(a.Type) {
			eligible = append(eligible, a)
		}
	}

	// availMinor returns an account's free-to-earmark balance in the goal currency.
	availMinor := func(acctID string) int64 {
		ac, ok := domain.AccountByID(accounts, acctID)
		if !ok {
			return 0
		}
		bal, _ := ledger.Balance(ac, txns)
		inGoal := bal.Amount
		if bal.Currency != cur {
			if conv, err := rates.Convert(bal, cur); err == nil {
				inGoal = conv.Amount
			}
		}
		return goalsvc.AvailableToEarmarkMinor(app.Goals(), acctID, inGoal, g.ID)
	}

	// doSplit auto-fills the per-account amounts from the "total to earmark" field, spread
	// across the SELECTED accounts (or all eligible when none are picked) by the given mode.
	// Even = as-equal-as-possible with a waterfall past capped accounts; proportional = by
	// each account's free-balance share. The math (goals.SplitEarmark) guarantees no account
	// is asked for more than its available headroom and the parts sum to the target (or all
	// capacity when the target exceeds it).
	doSplit := func(mode goalsvc.SplitMode) {
		totalMinor, perr := money.ParseMinor(strings.TrimSpace(totalS.Get()), dec)
		if perr != nil || totalMinor <= 0 {
			errS.Set(uistate.T("goals.allocSplitBad"))
			return
		}
		errS.Set("")
		targets := make([]domain.Account, 0, len(eligible))
		for _, a := range eligible {
			if selS.Get()[a.ID] {
				targets = append(targets, a)
			}
		}
		if len(targets) == 0 {
			targets = eligible
		}
		avails := make([]int64, len(targets))
		for i, a := range targets {
			avails[i] = availMinor(a.ID)
		}
		parts := goalsvc.SplitEarmark(totalMinor, avails, mode)
		newSel := map[string]bool{}
		newAmt := map[string]string{}
		for i, a := range targets {
			if parts[i] > 0 {
				newSel[a.ID] = true
				newAmt[a.ID] = money.FormatMinor(parts[i], dec)
			}
		}
		selS.Set(newSel)
		amountsS.Set(newAmt)
		if !enabledS.Get() {
			enabledS.Set(true)
		}
	}
	onSplitEven := ui.UseEvent(Prevent(func() { doSplit(goalsvc.SplitEven) }))
	onSplitProp := ui.UseEvent(Prevent(func() { doSplit(goalsvc.SplitProportional) }))

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		var allocs []domain.GoalAllocation
		if enabledS.Get() {
			for _, ac := range eligible {
				if !selS.Get()[ac.ID] {
					continue
				}
				minor, perr := money.ParseMinor(strings.TrimSpace(amountsS.Get()[ac.ID]), dec)
				if perr != nil || minor <= 0 {
					continue
				}
				if avail := availMinor(ac.ID); minor > avail {
					minor = avail // cap at the account's free balance (never earmark more than exists)
				}
				if minor <= 0 {
					continue
				}
				allocs = append(allocs, domain.GoalAllocation{AccountID: ac.ID, Amount: money.New(minor, cur)})
			}
		}
		if err := app.SetGoalAllocations(g.ID, allocs); err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("goals.allocateFooter", fmtMoney(money.New(sumAllocMinor(allocs), cur)), fmtMoney(g.TargetAmount)), false)
		done()
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	enabled := enabledS.Get()

	// Live earmarked total from the current selection/inputs (for the footer).
	var earmarked int64
	if enabled {
		for _, ac := range eligible {
			if !selS.Get()[ac.ID] {
				continue
			}
			if minor, perr := money.ParseMinor(strings.TrimSpace(amountsS.Get()[ac.ID]), dec); perr == nil && minor > 0 {
				earmarked += minor
			}
		}
	}
	coverPct := goalsvc.CoveragePercent(domain.Goal{TargetAmount: g.TargetAmount, CurrentAmount: g.CurrentAmount, Allocations: []domain.GoalAllocation{{Amount: money.New(earmarked, cur)}}})

	// The account picker (only when earmarking is on): every account is a selectable row
	// with its own amount, disabled until picked.
	var picker ui.Node = Fragment()
	if enabled {
		if len(eligible) == 0 {
			picker = P(css.Class("budget-sub"), uistate.T("goals.allocateNoAccts"))
		} else {
			picker = Div(css.Class("goal-alloc-list"),
				MapKeyed(eligible, func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
					return ui.CreateElement(goalAllocateRow, goalAllocateRowProps{
						AccountID:   a.ID,
						AccountName: a.Name,
						AvailStr:    uistate.T("goals.allocateAvail", fmtMoney(money.New(availMinor(a.ID), cur))),
						Value:       amountsS.Get()[a.ID],
						Selected:    selS.Get()[a.ID],
						OnToggle:    onToggleAcct,
						OnChange:    onAmount,
					})
				}),
			)
		}
	}

	return Form(css.Class("acct-edit-form", "goal-allocate"), OnSubmit(save),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", "muted"), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("goals.allocateIntro")),
			// Master toggle: turn earmarking on/off for this goal.
			Label(css.Class("goal-alloc-enable"),
				Input(append([]any{Type("checkbox"), Attr("data-testid", "goal-alloc-toggle"), OnChange(onToggleEnabled)}, checkedAttr(enabled)...)...),
				Span(css.Class("goal-alloc-enable-txt"),
					Span(css.Class("goal-alloc-enable-title"), uistate.T("goals.allocateEnable")),
					Span(css.Class("budget-sub"), uistate.T("goals.allocateEnableHint")),
				),
			),
			// Smart split: enter a total and auto-distribute it across the picked accounts
			// (or all of them) — evenly, or in proportion to each account's free balance.
			If(enabled, Div(css.Class("goal-alloc-split"), Attr("data-testid", "goal-alloc-split"),
				Div(css.Class("goal-alloc-split-row"),
					Div(css.Class("goal-alloc-split-field"),
						Span(css.Class("t-caption", tw.TextDim), uistate.T("goals.allocSplitLabel")),
						Input(css.Class("field", "goal-alloc-total"), Type("number"), Attr("data-testid", "goal-alloc-total"),
							Attr("min", "0"), Step("0.01"), Value(totalS.Get()), OnInput(onTotal)),
					),
					Div(css.Class("goal-alloc-split-btns"),
						Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-alloc-split-even"), OnClick(onSplitEven), uistate.T("goals.allocSplitEven")),
						Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-alloc-split-prop"), OnClick(onSplitProp), uistate.T("goals.allocSplitProp")),
					),
				),
				Span(css.Class("budget-sub"), uistate.T("goals.allocSplitHint")),
			)),
			picker,
			If(enabled, Div(css.Class("goal-alloc-summary"), Attr("data-testid", "goal-alloc-summary"),
				Span(uistate.T("goals.allocateFooter", fmtMoney(money.New(earmarked, cur)), fmtMoney(g.TargetAmount))),
				Span(css.Class("goal-alloc-cover"), uistate.T("goals.coverageChip", coverPct)),
			)),
			errLine,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "goal-alloc-save"), uistate.T("goals.allocateSave")),
		),
	)
}

// earmarkEligibleType reports whether an account type holds spendable cash you can
// reserve toward a goal. Only truly liquid types qualify — not brokerage/retirement
// (locked or market-risked) or property/vehicle (estimated value, not cash).
func earmarkEligibleType(t domain.AccountType) bool {
	switch t {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return true
	default:
		return false
	}
}

// sumAllocMinor totals a set of allocations in minor units (all share the goal currency).
func sumAllocMinor(allocs []domain.GoalAllocation) int64 {
	var s int64
	for _, a := range allocs {
		s += a.Amount.Amount
	}
	return s
}

// goalAllocateRowProps drives one account's selectable earmark row.
type goalAllocateRowProps struct {
	AccountID, AccountName, AvailStr, Value string
	Selected                                bool
	OnToggle                                func(string)         // select/deselect the account
	OnChange                                func(string, string) // amount change (accountID, value)
}

// goalAllocateRow is one account row in the allocate modal: a select checkbox + the
// account name/free-balance, then an amount input that's disabled until the account is
// picked. Its own component so the checkbox + input hooks stay at stable call-sites.
func goalAllocateRow(props goalAllocateRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.AccountID) })
	onInput := ui.UseEvent(func(v string) { props.OnChange(props.AccountID, v) })
	rowCls := "goal-alloc-row"
	if props.Selected {
		rowCls += " is-on"
	}
	amtArgs := []any{css.Class("field", "goal-alloc-input"), Type("number"),
		Attr("data-testid", "goal-alloc-"+props.AccountID), Placeholder("0"),
		Value(props.Value), Step("0.01"), Attr("min", "0"), OnInput(onInput)}
	if !props.Selected {
		amtArgs = append(amtArgs, Attr("disabled", ""))
	}
	return Div(css.Class(rowCls),
		Label(css.Class("goal-alloc-pick"),
			Input(append([]any{Type("checkbox"), Attr("data-testid", "goal-alloc-pick-"+props.AccountID), OnChange(toggle)}, checkedAttr(props.Selected)...)...),
			Div(css.Class("goal-alloc-row-main"),
				Span(css.Class("goal-alloc-acct"), props.AccountName),
				Span(css.Class("goal-alloc-avail", tw.TextDim), props.AvailStr),
			),
		),
		Input(amtArgs...),
	)
}

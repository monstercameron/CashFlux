// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type goalEarmarksProps struct{ App *appstate.App }

// GoalEarmarksManager is the "Earmarks" tab of the goals page: a full-CRUD view of every
// virtual allocation across all goals. It shows per-account exposure (how much of each
// account is reserved vs still free, flagged when over-earmarked), then each goal's
// earmarks grouped together with per-row Remove and a Manage button that opens the goal's
// allocate modal (where amounts are added/edited via the smart-split control). Read here,
// create/update via Manage, delete per row.
func GoalEarmarksManager(props goalEarmarksProps) ui.Node {
	return ui.CreateElement(goalEarmarksManager, props)
}

func goalEarmarksManager(props goalEarmarksProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	toBase := func(m money.Money) int64 {
		if m.Currency == base || m.Currency == "" {
			return m.Amount
		}
		if conv, err := rates.Convert(m, base); err == nil {
			return conv.Amount
		}
		return m.Amount
	}
	accounts := app.Accounts()
	txns := app.Transactions()
	goalsList := app.Goals()

	// Delete one earmark (goal + account) by persisting the goal's remaining allocations.
	onDelete := func(goalID, acctID string) {
		for _, g := range goalsList {
			if g.ID != goalID {
				continue
			}
			kept := make([]domain.GoalAllocation, 0, len(g.Allocations))
			for _, al := range g.Allocations {
				if al.AccountID != acctID {
					kept = append(kept, al)
				}
			}
			if err := app.SetGoalAllocations(goalID, kept); err == nil {
				uistate.BumpDataRevision()
				uistate.PostNotice(uistate.T("goals.earmarkRemoved"), false)
			}
			return
		}
	}

	// Per-account earmarked totals (base), and which goals have any earmarks.
	earmarkByAcct := map[string]int64{}
	var grandTotal int64
	var goalsWithEarmarks []domain.Goal
	for _, g := range goalsList {
		if g.AllocatedMinor() <= 0 {
			continue
		}
		goalsWithEarmarks = append(goalsWithEarmarks, g)
		for _, al := range g.Allocations {
			b := toBase(al.Amount)
			earmarkByAcct[al.AccountID] += b
			grandTotal += b
		}
	}

	if len(goalsWithEarmarks) == 0 {
		return uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("data-testid", "earmarks-empty")},
			Body: Div(css.Class("ea-empty"),
				uiw.Icon(icon.Goals, css.Class(tw.W5, tw.H5, tw.TextDim)),
				P(css.Class("ea-empty-title"), uistate.T("goals.earmarksEmpty")),
				P(css.Class("budget-sub"), uistate.T("goals.earmarksEmptyHint")),
			),
		})
	}

	// Account exposure rows (only accounts that carry an earmark), with a free figure and an
	// over-earmarked flag when the live balance no longer backs the reservations.
	var exposureRows []ui.Node
	for _, a := range accounts {
		em := earmarkByAcct[a.ID]
		if em <= 0 {
			continue
		}
		bal, _ := ledger.Balance(a, txns)
		balBase := toBase(bal)
		free := balBase - em
		over := free < 0
		if free < 0 {
			free = 0
		}
		freeCls := "ea-exp-free"
		if over {
			freeCls += " " + tw.Fold(tw.TextWarn)
		}
		exposureRows = append(exposureRows, Div(css.Class("ea-exp-row"), Attr("data-testid", "ea-exp-"+a.ID),
			Span(css.Class("ea-exp-name"), a.Name),
			Span(css.Class("ea-exp-earmarked"), fmtMoney(money.New(em, base))),
			Span(css.Class(freeCls), uistate.T("goals.earmarksFreeOf", fmtMoney(money.New(free, base)), fmtMoney(money.New(balBase, base)))),
		))
	}

	exposureCard := uiw.Card(uiw.CardProps{
		Header: H2(css.Class("card-title"), uistate.T("goals.earmarksExposure"),
			Span(css.Class("budget-sub"), uistate.T("goals.earmarksTotal", fmtMoney(money.New(grandTotal, base)), plural(len(goalsWithEarmarks), "goal")))),
		Body: Div(css.Class("ea-exp-list"),
			Div(css.Class("ea-exp-row ea-exp-head"),
				Span(css.Class("ea-exp-name"), uistate.T("common.name")),
				Span(css.Class("ea-exp-earmarked"), uistate.T("goals.earmarksColEarmarked")),
				Span(css.Class("ea-exp-free"), uistate.T("goals.earmarksColFree")),
			),
			exposureRows,
		),
	})

	overbooked := overbookedGoals(app)
	groups := MapKeyed(goalsWithEarmarks, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
		return ui.CreateElement(goalEarmarkGroup, goalEarmarkGroupProps{
			Goal: g, Accounts: accounts, Overbooked: overbooked[g.ID], OnDelete: onDelete,
		})
	})

	return Div(css.Class("earmarks-mgr"),
		exposureCard,
		uiw.Card(uiw.CardProps{
			Header: H2(css.Class("card-title"), uistate.T("goals.earmarksByGoal")),
			Body:   Div(css.Class("ea-goals"), groups),
		}),
	)
}

// goalEarmarkGroupProps drives one goal's block in the earmarks manager.
type goalEarmarkGroupProps struct {
	Goal       domain.Goal
	Accounts   []domain.Account
	Overbooked bool
	OnDelete   func(goalID, acctID string)
}

// goalEarmarkGroup renders one goal's earmarks: a header (name + coverage + a Manage button
// that opens its allocate modal) and a row per earmarked account. Its own component so the
// Manage hook sits at a stable position (never inside the goals loop).
func goalEarmarkGroup(props goalEarmarkGroupProps) ui.Node {
	g := props.Goal
	manage := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeAllocate})
	}))
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = g.CurrentAmount.Currency
	}
	var warn ui.Node = Fragment()
	if props.Overbooked {
		warn = Span(ClassStr("pace-badge earmark-partial"), Attr("data-testid", "ea-over-"+g.ID), uistate.T("goals.earmarksOverbooked"))
	}
	rows := MapKeyed(g.Allocations, func(al domain.GoalAllocation) any { return al.AccountID }, func(al domain.GoalAllocation) ui.Node {
		name := accountName(props.Accounts, al.AccountID)
		if name == "" {
			name = al.AccountID
		}
		return ui.CreateElement(goalEarmarkRow, goalEarmarkRowProps{
			GoalID: g.ID, AccountID: al.AccountID, AccountName: name,
			AmountStr: fmtMoney(al.Amount), OnDelete: props.OnDelete,
		})
	})
	return Div(css.Class("ea-goal"), Attr("data-testid", "ea-goal-"+g.ID),
		Div(css.Class("ea-goal-head"),
			Span(css.Class("ea-goal-name"), g.Name),
			Span(css.Class("goal-alloc-cover"), uistate.T("goals.coverageChip", goalsvc.CoveragePercent(g))),
			warn,
			Button(css.Class("btn btn-sm", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "ea-manage-"+g.ID), OnClick(manage),
				uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("goals.earmarksManage"))),
		),
		Div(css.Class("ea-goal-rows"), rows),
	)
}

// goalEarmarkRowProps drives one account earmark row inside a goal's block.
type goalEarmarkRowProps struct {
	GoalID, AccountID, AccountName, AmountStr string
	OnDelete                                  func(goalID, acctID string)
}

// goalEarmarkRow is one account · amount · Remove row. Its own component so the delete hook
// stays at a stable call-site (never inside the allocations loop).
func goalEarmarkRow(props goalEarmarkRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.GoalID, props.AccountID) }))
	return Div(css.Class("ea-row"),
		Span(css.Class("ea-row-acct"), props.AccountName),
		Span(css.Class("ea-row-amt"), props.AmountStr),
		Button(css.Class("ea-row-del"), Type("button"), Attr("data-testid", "ea-del-"+props.GoalID+"-"+props.AccountID),
			Attr("aria-label", uistate.T("goals.earmarksDelete")), Title(uistate.T("goals.earmarksDelete")), OnClick(del),
			uiw.Icon(icon.Close, css.Class(tw.W35, tw.H35))),
	)
}

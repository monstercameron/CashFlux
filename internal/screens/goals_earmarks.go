// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
	funding := goalFunding(app)
	v := funding.Earmarks
	accounts := app.Accounts()
	goalsList := app.Goals()
	base := v.TotalBalance.Currency

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

	goalsWithEarmarks := v.Goals
	grandTotal, totalBal, totalFree, heldAssets := v.TotalReserved.Amount, v.TotalBalance.Amount, v.Free.Amount, v.Held.Amount
	freeMod := ""
	if totalFree < 0 {
		freeMod = " " + tw.Fold(tw.TextWarn)
	}
	earmarkedShare := v.Percent
	moneyMap := uiw.Card(uiw.CardProps{
		Attrs:  []any{Attr("data-testid", "earmarks-moneymap")},
		Header: H2(css.Class("card-title"), uistate.T("goals.mapTitle")),
		Body: Div(css.Class("ea-map"),
			Div(css.Class("ea-map-figs"),
				goalFig(uistate.T("goals.mapTotal"), fmtMoney(money.New(totalBal, base))),
				goalFig(uistate.T("goals.mapEarmarked"), fmtMoney(money.New(grandTotal, base))),
				Div(css.Class("goal-fig"),
					Span(css.Class("goal-fig-k"), uistate.T("goals.mapFree")),
					Span(ClassStr("goal-fig-v"+freeMod), fmtMoney(money.New(totalFree, base))),
				),
			),
			// A single stacked bar: the earmarked share solid, the free remainder quiet.
			Div(css.Class("ea-map-bar"), Attr("role", "img"),
				Attr("aria-label", uistate.T("goals.mapBarLabel", fmtMoney(money.New(grandTotal, base)), fmtMoney(money.New(totalBal, base)))),
				Div(css.Class("ea-map-bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", earmarkedShare))),
			),
			If(heldAssets > 0, P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "earmarks-held-note"),
				uistate.T("goals.mapHeldNote", fmtMoney(money.New(heldAssets, base))))),
		),
	})

	// G3: the manager can START an earmark, not just trim one — a chip per financial,
	// non-archived goal that has none yet, each opening its Set-aside modal directly.
	unearmarked := v.Unearmarked
	var startStrip ui.Node = Fragment()
	if len(unearmarked) > 0 {
		chips := MapKeyed(unearmarked, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return ui.CreateElement(earmarkStartChip, earmarkStartChipProps{GoalID: g.ID, GoalName: g.Name})
		})
		startStrip = Div(css.Class("ea-start"), Attr("data-testid", "earmarks-start-strip"),
			Span(css.Class("budget-sub"), uistate.T("goals.earmarksStartHint")),
			Div(css.Class("ea-start-chips"), chips),
		)
	}

	if len(goalsWithEarmarks) == 0 {
		return uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("data-testid", "earmarks-empty")},
			Body: Div(css.Class("ea-empty"),
				uiw.Icon(icon.Goals, css.Class(tw.W5, tw.H5, tw.TextDim)),
				P(css.Class("ea-empty-title"), uistate.T("goals.earmarksEmpty")),
				P(css.Class("budget-sub"), uistate.T("goals.earmarksEmptyHint")),
				startStrip,
			),
		})
	}

	// Account exposure cards (only accounts that carry an earmark): a name + earmarked/free
	// figures over a coverage BAR that fills to the earmarked share of the balance — so the
	// reserved-vs-free split reads at a glance. When the live balance no longer backs the
	// reservations (over-earmarked), the bar fills fully in the danger tone and the free
	// figure is flagged.
	var exposureRows []ui.Node
	for _, exposure := range v.Accounts {
		a := exposure.Account
		em, balBase, free := exposure.Reserved.Amount, exposure.Balance.Amount, exposure.Free.Amount
		over, coverPct := exposure.Overbooked, exposure.Percent
		freeCls, barCls := "ea-exp-free", "ea-bar-fill"
		if over {
			freeCls += " " + tw.Fold(tw.TextWarn)
			barCls += " is-over"
		}
		exposureRows = append(exposureRows, Div(css.Class("ea-acct"), Attr("data-testid", "ea-exp-"+a.ID),
			Div(css.Class("ea-acct-top"),
				Span(css.Class("ea-acct-name"), a.Name),
				Span(css.Class("ea-acct-figs"),
					Span(css.Class("ea-acct-earmarked"), fmtMoney(money.New(em, base))),
					Span(css.Class(freeCls), uistate.T("goals.earmarksFreeOf", fmtMoney(money.New(free, base)), fmtMoney(money.New(balBase, base)))),
				),
			),
			Div(css.Class("ea-acct-bar"), Attr("role", "img"),
				Attr("aria-label", uistate.T("goalsredesign.earmarkBarLabel", a.Name, fmtMoney(money.New(em, base)), fmtMoney(money.New(balBase, base)))),
				Div(ClassStr(barCls), Attr("style", fmt.Sprintf("width:%d%%", coverPct))),
			),
		))
	}

	exposureCard := uiw.Card(uiw.CardProps{
		Header: H2(css.Class("card-title"), uistate.T("goals.earmarksExposure"),
			Span(css.Class("budget-sub"), uistate.T("goals.earmarksTotal", fmtMoney(money.New(grandTotal, base)), plural(len(goalsWithEarmarks), "goal")))),
		Body: Div(css.Class("ea-exp-list"), exposureRows),
	})

	overbooked := funding.Overbooked
	groups := MapKeyed(goalsWithEarmarks, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
		return ui.CreateElement(goalEarmarkGroup, goalEarmarkGroupProps{
			Goal: g, Accounts: accounts, Overbooked: overbooked[g.ID], OnDelete: onDelete,
		})
	})

	return Div(css.Class("earmarks-mgr"),
		If(v.Incomplete, P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "earmarks-excluded"),
			uistate.T("goals.earmarksIncomplete"),
			If(len(v.ExcludedCurrencies) > 0, Span(uistate.T("goals.earmarksExcludedCurrencies", strings.Join(v.ExcludedCurrencies, ", ")))))),
		moneyMap,
		exposureCard,
		uiw.Card(uiw.CardProps{
			Header: H2(css.Class("card-title"), uistate.T("goals.earmarksByGoal")),
			Body:   Fragment(Div(css.Class("ea-goals"), groups), startStrip),
		}),
	)
}

// earmarkStartChipProps drives one "Set aside for <goal>" starter chip.
type earmarkStartChipProps struct {
	GoalID, GoalName string
}

// earmarkStartChip opens a goal's Set-aside modal straight from the Earmarks tab, so
// the manager can start a reservation, not only trim existing ones. Its own component
// so the click hook sits at a stable call-site (no On* in the map loop).
func earmarkStartChip(props earmarkStartChipProps) ui.Node {
	open := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: props.GoalID, Mode: uistate.GoalEditModeAllocate})
	}))
	return Button(css.Class("ea-start-chip"), Type("button"),
		Attr("data-testid", "earmark-start-"+props.GoalID),
		Title(uistate.T("goals.allocateTitle")), OnClick(open),
		uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(uistate.T("goals.earmarksStartChip", props.GoalName)),
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
	cov := goalsvc.CoveragePercent(g)
	covFill := cov
	if covFill > 100 {
		covFill = 100
	}
	return Div(css.Class("ea-goal"), Attr("data-testid", "ea-goal-"+g.ID),
		Div(css.Class("ea-goal-head"),
			Span(css.Class("ea-goal-name"), g.Name),
			// Coverage as a mini bar beside the % chip, so how-close-to-covered reads at a glance.
			Div(css.Class("ea-cover"), Attr("role", "img"), Attr("aria-label", uistate.T("goalsredesign.coverageBarLabel", cov)),
				Div(css.Class("ea-cover-fill"), Attr("style", fmt.Sprintf("width:%d%%", covFill)))),
			Span(css.Class("goal-alloc-cover"), uistate.T("goals.coverageChip", cov)),
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

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/preflight"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// paydayPreflightCard (XC9) is the dismissible payday pre-flight ritual surfaced at
// the top of /recurring at each pay-cycle boundary. It composes the bill-schedule
// outputs into a glanceable checklist — bills due this cycle, the projected low
// point (the one emphasized figure), and any account running below the keep floor
// — with a one-tap "mark planned" per bill and a move-money link. It regenerates
// once per pay cycle and stays hidden after dismissal until the next payday.
func paydayPreflightCard() ui.Node {
	app := appstate.Default
	if app == nil {
		return nil
	}
	uistate.UseDataRevision() // re-render on data / dismissal changes
	nav := router.UseNavigate()

	now := time.Now()
	bd := liveBillsSmartHorizon(app, 60)
	nextPay := nextPaydayAfter(bd.Paydays, now)
	if nextPay.IsZero() {
		return nil // no configured pay cycle — nothing to anchor the ritual to
	}
	cycleKey := nextPay.Format("2006-01-02")

	// Session-instant dismissal that also persists per cycle.
	dismissed := ui.UseState(uistate.PreflightDismissedCycle() == cycleKey)
	planned := ui.UseState(map[string]bool{})
	if dismissed.Get() {
		return nil
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Bills due this cycle.
	var items []preflight.BillItem
	for _, b := range bills.OccurrencesWithin(app.Accounts(), app.Recurring(), now, nextPay) {
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		items = append(items, preflight.BillItem{
			ID: billItemID(b), Name: b.Name, AmountMinor: abs64i(amt.Amount),
			Due: b.DueDate, Autopay: b.Autopay, AccountID: b.AccountID, Currency: base,
		})
	}

	// Projected low over the cycle (smart schedule) + account balances.
	plan := computeBillsSmart(app, now, nextPay)
	var accts []preflight.AccountBalance
	if bals, err := ledger.Balances(app.Accounts(), app.Transactions()); err == nil {
		for _, a := range app.Accounts() {
			if a.Archived || a.Class == domain.ClassLiability {
				continue
			}
			if m, ok := bals[a.ID]; ok {
				accts = append(accts, preflight.AccountBalance{
					ID: a.ID, Name: a.Name,
					BalanceMinor: convertMinorScreen(m.Amount, a.Currency, base, rates),
				})
			}
		}
	}

	// TX9: drop bills already settled by a matched transaction. A recurring bill's
	// occurrence carries a durable bill-match link when a real payment matched it;
	// treat those as done rather than "still due this cycle".
	paid := map[string]bool{}
	for _, b := range bills.OccurrencesWithin(app.Accounts(), app.Recurring(), now, nextPay) {
		if rid, ok := recurringIDFromBillAccount(b.AccountID); ok {
			if _, matched := app.BillMatchForOccurrence(rid, b.DueDate); matched {
				paid[billItemID(b)] = true
			}
		}
	}

	cl := preflight.Build(preflight.Input{
		Now: now, CycleStart: nextPaydayOnOrBefore(bd.Paydays, now), NextPayday: nextPay,
		Bills: items, ProjectedLowMinor: plan.Res.Smart.Low, ProjectedLowDate: plan.Res.Smart.LowDate,
		KeepFloorMinor: bd.MinKeepMinor, Accounts: accts, Paid: paid,
	})
	if !cl.HasItems() {
		return nil
	}

	onDismiss := ui.UseEvent(func() {
		uistate.DismissPreflightCycle(cycleKey)
		dismissed.Set(true)
	})
	onMove := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/accounts")) })

	markPlanned := func(row preflight.BillRow) {
		task := domain.Task{
			ID: id.New(), Title: uistate.T("paydayPre.payTask", row.Name), Status: domain.StatusOpen,
			Priority: domain.PriorityMedium, Source: domain.SourceNudge,
		}
		r := preflight.ResolveForBill(row)
		task.Resolve = &r
		if err := app.PutTask(task); err == nil {
			planned.Update(func(prev map[string]bool) map[string]bool {
				next := map[string]bool{}
				for k, v := range prev {
					next[k] = v
				}
				next[row.ID] = true
				return next
			})
		}
	}

	// Bill rows (each its own component so its button hook stays stable).
	billRows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt2), Attr("data-testid", "preflight-bills")}
	for _, r := range cl.Bills {
		row := r
		billRows = append(billRows, ui.CreateElement(preflightBillRow, preflightBillRowProps{
			Name: row.Name, Amount: fmtMoney(money.New(row.AmountMinor, row.Currency)),
			Due: row.Due.Format("Jan 2"), Autopay: row.Autopay,
			Planned: planned.Get()[row.ID], OnPlan: func() { markPlanned(row) },
		}))
	}

	// The low-point line is the single emphasized figure.
	lowTone := tw.TextDim
	lowMsg := uistate.T("preflight.lowPointOk")
	if cl.BelowFloor {
		lowTone = tw.TextDown
		lowMsg = uistate.T("preflight.lowPointFloor", fmtMoney(money.New(cl.FloorMinor, base)))
	}

	dipRows := []any{}
	for _, dsp := range cl.DippingAccounts {
		dipRows = append(dipRows, P(css.Class("t-caption", tw.TextDim),
			uistate.T("preflight.dippingItem", dsp.Name, fmtMoney(money.New(dsp.BalanceMinor, base)))))
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "complementary"),
		Attr("data-testid", "payday-preflight"),
		Attr("aria-label", uistate.T("preflight.title")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "🗓️"),
				Div(css.Class("catchup-card-text"),
					Strong(uistate.T("preflight.title")),
					P(uistate.T("preflight.subtitle", nextPay.Format("Jan 2"))),
				),
			),
			// Emphasized low-point figure.
			P(css.Class("t-body", lowTone), Attr("role", "status"), Attr("data-testid", "preflight-lowpoint"),
				uistate.T("preflight.lowPoint", fmtMoney(money.New(cl.LowPointMinor, base)), cl.LowPointDate.Format("Jan 2"))+" "+lowMsg),
			If(len(cl.Bills) > 0, Fragment(
				P(css.Class("t-caption", tw.TextDim, tw.Mt3), uistate.T("preflight.billsHeading")),
				Div(billRows...),
			)),
			If(len(dipRows) > 0, Fragment(
				P(css.Class("t-caption", tw.TextDim, tw.Mt3), uistate.T("preflight.dippingHeading")),
				Div(append([]any{css.Class(tw.Flex, tw.FlexCol, tw.Gap1)}, dipRows...)...),
			)),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-sm btn-primary"), Type("button"),
					Attr("data-testid", "preflight-move"), OnClick(onMove), uistate.T("preflight.transfer")),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "preflight-dismiss"), OnClick(onDismiss), uistate.T("preflight.dismiss")),
			),
		),
	)
}

type preflightBillRowProps struct {
	Name    string
	Amount  string
	Due     string
	Autopay bool
	Planned bool
	OnPlan  func()
}

// preflightBillRow is one checklist-quiet bill line as its own component so the
// "mark planned" click handler hook stays at a stable render position.
func preflightBillRow(props preflightBillRowProps) ui.Node {
	onPlan := ui.UseEvent(func() {
		if props.OnPlan != nil {
			props.OnPlan()
		}
	})
	right := Button(css.Class("btn btn-tool btn-sm"), Type("button"),
		Attr("aria-label", uistate.T("preflight.markPlanned")+" "+props.Name),
		OnClick(onPlan), uistate.T("preflight.markPlanned"))
	if props.Planned {
		right = Span(css.Class("t-caption", tw.TextUp), uistate.T("preflight.planned"))
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		Span(css.Class("t-body"), props.Name),
		If(props.Autopay, Span(css.Class("chip"), uistate.T("preflight.autopay"))),
		Span(css.Class(tw.MlAuto, "t-body", tw.TextDim), props.Amount),
		Span(css.Class("t-caption", tw.TextDim), props.Due),
		right,
	)
}

// nextPaydayAfter returns the earliest payday strictly after t (zero if none).
func nextPaydayAfter(paydays []time.Time, t time.Time) time.Time {
	var best time.Time
	for _, p := range paydays {
		if p.After(t) && (best.IsZero() || p.Before(best)) {
			best = p
		}
	}
	return best
}

// nextPaydayOnOrBefore returns the latest payday on or before t (zero if none).
func nextPaydayOnOrBefore(paydays []time.Time, t time.Time) time.Time {
	var best time.Time
	for _, p := range paydays {
		if !p.After(t) && (best.IsZero() || p.After(best)) {
			best = p
		}
	}
	return best
}

// abs64i returns the absolute value of an int64.
func abs64i(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// convertMinorScreen converts minor units between currencies for screen assembly.
func convertMinorScreen(minor int64, from, to string, rates currency.Rates) int64 {
	if from == "" || from == to {
		return minor
	}
	if v, err := currency.ConvertBetween(minor, from, to, rates); err == nil {
		return v
	}
	return minor
}

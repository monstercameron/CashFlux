// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// DebtStrategyPanelProps configures a DebtStrategyPanel.
// No external props are required; the panel reads appstate.Default directly.
type DebtStrategyPanelProps struct{}

// DebtStrategyPanel renders the snowball-vs-avalanche debt-payoff block as a
// registered component so it can be shared between /planning and /debt without
// duplicating hook state or violating the GWC per-row hook rule. All hooks are
// declared unconditionally at the top; the component owns its own revision
// counter and extra-payment input state so mutations stay isolated to this panel.
func DebtStrategyPanel(props DebtStrategyPanelProps) ui.Node {
	// All hooks must be declared unconditionally — even when app is nil.
	rev := ui.UseState(0)
	dsExtra := ui.UseState("")
	onDsExtra := ui.UseEvent(func(v string) { dsExtra.Set(v) })

	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	if app == nil {
		return Fragment()
	}

	// Reading rev establishes the render dependency; Set() calls in callbacks
	// will trigger a re-render of this panel.
	_ = rev.Get()

	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// C195: FX-convert each included debt to the base currency. A EUR balance
	// must not be summed into a USD plan as raw cents. AggregateDebts handles
	// IncludedInPayoff + Abs + conversion, and reports currencies missing a rate
	// (those debts are excluded rather than miscounted).
	debts, missingDebtRates := payoff.AggregateDebts(app.Accounts(), txns, base, rates)

	// Payoff progress vs a stored baseline (L5 gap 5): "paid off $X of $Y".
	var currentOwed int64
	for _, d := range debts {
		currentOwed += d.Balance
	}
	prog, since, tracking := app.PayoffProgress(currentOwed)
	var progressNode ui.Node = Fragment()
	if tracking {
		w := prog.Percent
		if w > 100 {
			w = 100
		}
		progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
			P(css.Class("budget-sub", tw.FontDisplay), "Paid off "+fmtMoney(money.New(prog.PaidOff, base))+" of "+fmtMoney(money.New(prog.Baseline, base))+" ("+strconv.Itoa(prog.Percent)+"%) since "+since.Format("Jan 2, 2006")+"."),
			Div(css.Class("bar"), Div(css.Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", w)))),
			Button(css.Class("btn"), Type("button"), Style(map[string]string{"margin-top": "0.4rem"}), OnClick(func() { _ = app.ClearPayoffTracking(); rev.Set(rev.Get() + 1) }), "Reset progress"),
		)
	} else if len(debts) > 0 {
		owed := currentOwed
		progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
			Button(css.Class("btn"), Type("button"), Title(uistate.T("planning.snapshotTitle")),
				OnClick(func() { _ = app.StartPayoffTracking(owed, base); rev.Set(rev.Get() + 1) }), "Start tracking progress"),
		)
	}

	// C195: if any liability is in a currency with no FX rate, it can't be summed
	// into the base-currency plan, so AggregateDebts excluded it. Say so plainly
	// rather than silently undercounting the debt total.
	var rateWarn ui.Node = Fragment()
	if len(missingDebtRates) > 0 {
		rateWarn = P(css.Class("budget-sub"), Style(map[string]string{"margin-top": "0.6rem"}),
			uistate.T("planning.debtMissingRate", strings.Join(missingDebtRates, ", ")))
	}

	// Per-debt inclusion and APR/minimum editing now live on the payoff-ladder cards
	// (the in-plan toggle + Edit), so the strategy panel no longer duplicates them — it
	// stays focused on the snowball-vs-avalanche decision.

	var body ui.Node
	switch {
	case len(debts) == 0:
		body = P(css.Class("empty"), uistate.T("planning.debtStrategyEmpty"))
	default:
		extra, _ := money.ParseMinor(strings.TrimSpace(dsExtra.Get()), currency.Decimals(base))
		if extra < 0 {
			extra = 0
		}
		snow, okS := payoff.BuildPlan(debts, extra, payoff.Snowball)
		aval, okA := payoff.BuildPlan(debts, extra, payoff.Avalanche)
		if !okS || !okA {
			body = P(css.Class("err"), Attr("role", "alert"), uistate.T("planning.strategyNotViable"))
		} else {
			rec := Fragment()
			if saved := snow.TotalInterest - aval.TotalInterest; saved > 0 {
				rec = P(css.Class("muted"), uistate.T("planning.strategyRecommend", fmtMoney(money.New(saved, base))))
			}
			// C197: surface the *time* difference, not just the interest. The two
			// strategies pay the same amount each month, but the faster one clears
			// the whole balance in fewer months — a concrete "X months sooner".
			if dm := snow.Months - aval.Months; dm != 0 {
				fasterLabel := uistate.T("planning.avalanche")
				months := dm
				if dm < 0 {
					fasterLabel = uistate.T("planning.snowball")
					months = -dm
				}
				dur := strconv.Itoa(months) + " month"
				if months != 1 {
					dur += "s"
				}
				rec = Fragment(rec, P(css.Class("muted"), uistate.T("planning.strategyTimeSaved", fasterLabel, dur)))
			}
			// When the two strategies are truly identical (typically at $0 extra,
			// or a single debt) the side-by-side is meaningless — explain why (L5).
			explain := Fragment()
			if snow.Months == aval.Months && snow.TotalInterest == aval.TotalInterest {
				explain = P(css.Class("budget-sub"), "Snowball and avalanche match here — add an extra monthly amount above to see them diverge.")
			}
			// A calendar debt-free date reads better than a bare month count
			// (L5), plus a "cleared by <month>" beside each debt in the order.
			now := time.Now()
			snowDate := payoff.DebtFreeMonth(now, snow.Months).Format("Jan 2006")
			avalDate := payoff.DebtFreeMonth(now, aval.Months).Format("Jan 2006")
			orderParts := make([]string, len(aval.Order))
			for i, n := range aval.Order {
				if i < len(aval.ClearedMonths) {
					orderParts[i] = n + " (" + payoff.DebtFreeMonth(now, aval.ClearedMonths[i]).Format("Jan 2006") + ")"
				} else {
					orderParts[i] = n
				}
			}
			// Burn-down chart (L5 gap 4): the remaining total balance falling to
			// zero from the full starting balance. C199: overlay BOTH strategies
			// as lines so snowball and avalanche can be compared, not just
			// avalanche shown alone. Each series is anchored at the full starting
			// balance (month 0) then follows its own schedule down to zero.
			burnChart := Fragment()
			if len(aval.Schedule) > 0 {
				var startTotal int64
				for _, d := range debts {
					startTotal += d.Balance
				}
				mkBurnPts := func(schedule []int64) []chartspec.Point {
					pts := make([]chartspec.Point, 0, len(schedule)+1)
					// C203: label each month with its calendar date so the x-axis reads
					// as real months, not bare indices (0,1,2…).
					pts = append(pts, chartspec.Point{X: 0, Y: currency.MajorFromMinor(startTotal, base), Label: payoff.DebtFreeMonth(now, 1).Format("Jan 2006")})
					for i, b := range schedule {
						pts = append(pts, chartspec.Point{X: float64(i + 1), Y: currency.MajorFromMinor(b, base), Label: payoff.DebtFreeMonth(now, i+2).Format("Jan 2006")})
					}
					return pts
				}
				yFmt := ".3~s"
				if currency.Symbol(base) == "$" {
					yFmt = "$.3~s"
				}
				burnChart = Div(Style(map[string]string{"margin-top": "0.6rem"}),
					P(css.Class("budget-sub"), "Balance burn-down to zero:"),
					uiw.Chart(uiw.ChartProps{
						Spec: chartspec.Spec{Kind: chartspec.Line, Series: []chartspec.Series{
							{Name: uistate.T("planning.avalanche"), Points: mkBurnPts(aval.Schedule)},
							{Name: uistate.T("planning.snowball"), Points: mkBurnPts(snow.Schedule)},
						}, Y: chartspec.Axis{Format: yFmt}},
						Height: "150px",
						Label:  "Debt balance falling to zero — avalanche vs snowball over " + strconv.Itoa(aval.Months) + " months",
					}),
				)
			}
			// Which method wins: fewer months, then less interest. At $0 extra they tie
			// (neither wins), so no card is badged and the "add an extra amount" hint shows.
			snowWins := snow.Months < aval.Months || (snow.Months == aval.Months && snow.TotalInterest < aval.TotalInterest)
			avalWins := aval.Months < snow.Months || (aval.Months == snow.Months && aval.TotalInterest < snow.TotalInterest)
			body = Div(
				// The snowball-vs-avalanche decision, side by side: months (the headline),
				// total interest, and debt-free date, with the better method badged.
				Div(css.Class("strat-compare"),
					strategyCard(uistate.T("planning.snowball"), snow.Months, money.New(snow.TotalInterest, base), snowDate, snowWins),
					strategyCard(uistate.T("planning.avalanche"), aval.Months, money.New(aval.TotalInterest, base), avalDate, avalWins),
				),
				rec,
				explain,
				If(len(orderParts) > 0, Div(css.Class("strat-order"),
					Span(css.Class("strat-order-label", tw.TextDim), uistate.T("debt.payoffOrderLabel")),
					P(css.Class("strat-order-seq"), strings.Join(orderParts, "  →  ")),
				)),
				burnChart,
			)
		}
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("planning.debtStrategyTitle"),
		// Allocate owns where spare money goes each month, which is exactly what the extra
		// payment here feeds — link there.
		HeaderAction: debtOwnerLink("/allocate", uistate.T("debt.linkAllocate")),
		// C200: an HTML id anchor so the debt planner is directly linkable
		// (/planning#debt) and the dedicated /debt route can scroll to it.
		Attrs: []any{Attr("id", "debt")},
		Body: Fragment(
			P(css.Class("muted"), uistate.T("planning.debtStrategyHint")),
			// The one control that drives the whole comparison: an optional extra monthly
			// payment, with a one-click "sensible amount" suggestion beside it (shown only
			// while the field is empty, so the plans can be made to diverge in one tap).
			Div(css.Class("strat-extra"),
				labeledField(uistate.T("planning.debtStrategyExtra", base),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Attr("data-testid", "strat-extra"), Value(dsExtra.Get()), Step("0.01"), OnInput(onDsExtra))),
				If(strings.TrimSpace(dsExtra.Get()) == "" && len(debts) > 1 && payoff.SuggestedExtra(debts) > 0,
					Button(css.Class("btn strat-try"), Type("button"), Title(uistate.T("planning.fillSensibleTitle")),
						OnClick(func() { dsExtra.Set(money.FormatMinor(payoff.SuggestedExtra(debts), currency.Decimals(base))) }),
						"Try "+fmtMoney(money.New(payoff.SuggestedExtra(debts), base))+"/mo")),
			),
			body,
			rateWarn,
			progressNode,
		),
	})
}

// strategyCard renders one side of the snowball-vs-avalanche comparison: the method name
// (with a "Recommended" badge on the winner), the months-to-clear headline in the display
// serif, and the total interest + debt-free date beneath. The winner card is accent-tinted
// so the better method is obvious at a glance.
func strategyCard(name string, months int, interest money.Money, freeDate string, winner bool) ui.Node {
	cls := "strat-card"
	if winner {
		cls += " is-winner"
	}
	return Div(ClassStr(cls),
		Div(css.Class("strat-card-head"),
			Span(css.Class("strat-card-name"), name),
			If(winner, Span(css.Class("strat-badge"), uistate.T("debt.recommended"))),
		),
		Div(css.Class("strat-card-months", tw.FontDisplay), uistate.T("planning.strategyMonths", months)),
		Div(css.Class("strat-card-stats"),
			Div(css.Class("strat-card-stat"),
				Span(css.Class("strat-card-stat-label", tw.TextDim), uistate.T("debt.totalInterestLabel")),
				Span(css.Class("strat-card-stat-value", tw.FontDisplay), fmtMoney(interest)),
			),
			Div(css.Class("strat-card-stat"),
				Span(css.Class("strat-card-stat-label", tw.TextDim), uistate.T("debt.debtFreeLabel")),
				Span(css.Class("strat-card-stat-value", tw.FontDisplay), freeDate),
			),
		),
	)
}

// DebtPlanner now lives in debt_widget.go as a widgetized surface host; the shared
// DebtStrategyPanel above is reused by both /planning and the /debt strategy tile.

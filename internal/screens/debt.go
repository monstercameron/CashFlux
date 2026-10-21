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
	"github.com/monstercameron/CashFlux/internal/debtview"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// DebtStrategyPanelProps configures a DebtStrategyPanel.
type DebtStrategyPanelProps struct {
	// HideExtraInput drops the panel's own extra-payment field when another tile
	// owns the canonical DebtConfig control.
	HideExtraInput bool
	App            *appstate.App
	Base           string
	Prepared       *debtview.Result
}

// DebtStrategyPanel renders prepared snowball-vs-avalanche facts. All hooks are
// declared unconditionally at the top; the component owns its revision counter
// and optional extra-payment input state.
func DebtStrategyPanel(props DebtStrategyPanelProps) ui.Node {
	app := props.App
	if app == nil {
		app = appstate.Default
	}
	base := props.Base
	if base == "" {
		base = "USD"
		if app != nil && app.Settings().BaseCurrency != "" {
			base = app.Settings().BaseCurrency
		}
	}
	dec := currency.Decimals(base)
	var currentOwed int64
	canTrack := false
	var suggestedExtra int64
	canSuggestExtra := false

	// All hooks must be declared unconditionally — even when app is nil.
	rev := ui.UseState(0)
	onResetTracking := ui.UseEvent(func() {
		if app != nil {
			_ = app.ClearPayoffTracking()
			rev.Set(rev.Get() + 1)
		}
	})
	onStartTracking := ui.UseEvent(func() {
		if app != nil && canTrack {
			_ = app.StartPayoffTracking(currentOwed, base)
			rev.Set(rev.Get() + 1)
		}
	})
	onSuggestExtra := ui.UseEvent(func() {
		if !canSuggestExtra {
			return
		}
		c := uistate.DebtConfigGet()
		c.DefaultExtraMinor = suggestedExtra
		uistate.SetDebtConfig(c)
		uistate.BumpDataRevision()
	})

	// Subscribe to the shared data revision so the comparison redraws when the /debt
	// tuner changes the plan (they share one canonical extra-payment value).
	_ = uistate.UseDataRevision().Get()
	// The extra monthly payment is the ONE canonical value in DebtConfig — the same
	// figure the /debt tuner writes and the ladder + hero read — so this comparison
	// never disagrees with the rest of the page about which scenario is active.
	// Committed on change (not per keystroke) to avoid a full-page recompute mid-type.
	onExtra := ui.UseEvent(func(v string) {
		minor, err := money.ParseMinor(strings.TrimSpace(v), dec)
		if err != nil {
			return
		}
		if minor < 0 {
			minor = 0
		}
		c := uistate.DebtConfigGet()
		c.DefaultExtraMinor = minor
		uistate.SetDebtConfig(c)
		uistate.BumpDataRevision()
	})

	if app == nil {
		return Fragment()
	}

	// Reading rev establishes the render dependency; Set() calls in callbacks
	// will trigger a re-render of this panel.
	_ = rev.Get()

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	prepared := debtview.Result{}
	if props.Prepared != nil {
		prepared = *props.Prepared
	} else {
		cfg := uistate.DebtConfigGet()
		prepared = debtview.Prepare(app.Accounts(), app.Transactions(), rates, base, debtview.Config{
			ExtraMinor: cfg.DefaultExtraMinor, Strategy: strategyFromConfig(cfg), CompareStrategies: true,
		}, time.Now())
	}
	debts := prepared.Debts

	// Payoff progress vs a stored baseline (L5 gap 5): "paid off $X of $Y".
	if prepared.PlanKnown {
		currentOwed = prepared.PlanOwed.Amount
		canTrack = true
	}
	suggestedExtra, canSuggestExtra = prepared.SuggestedExtraMinor, prepared.SuggestedExtraKnown && prepared.SuggestedExtraMinor > 0
	var prog payoff.Progress
	var since time.Time
	tracking := false
	if canTrack {
		prog, since, tracking = app.PayoffProgress(currentOwed)
	}
	var progressNode ui.Node = Fragment()
	if tracking {
		w := prog.Percent
		if w > 100 {
			w = 100
		}
		progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
			P(css.Class("budget-sub", tw.FontDisplay), uistate.T("debt.paidOffSince", fmtMoney(money.New(prog.PaidOff, base)), fmtMoney(money.New(prog.Baseline, base)), prog.Percent, since.Format("Jan 2, 2006"))),
			Div(css.Class("bar"), Div(css.Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", w)))),
			Button(css.Class("btn"), Type("button"), Style(map[string]string{"margin-top": "0.4rem"}), OnClick(onResetTracking), uistate.T("debt.resetProgress")),
		)
	} else if prepared.PlanKnown && len(debts) > 0 {
		progressNode = Div(Style(map[string]string{"margin-top": "0.6rem"}),
			Button(css.Class("btn"), Type("button"), Title(uistate.T("planning.snapshotTitle")),
				OnClick(onStartTracking), uistate.T("debt.startTracking")),
		)
	}

	// A missing included-debt rate makes the whole comparison unavailable. Show
	// the affected currencies from the checked preparation result.
	var rateWarn ui.Node = Fragment()
	if len(prepared.PlanMissingRates) > 0 {
		rateWarn = P(css.Class("budget-sub"), Style(map[string]string{"margin-top": "0.6rem"}),
			uistate.T("planning.debtMissingRate", strings.Join(prepared.PlanMissingRates, ", ")))
	}

	// Per-debt inclusion and APR/minimum editing now live on the payoff-ladder cards
	// (the in-plan toggle + Edit), so the strategy panel no longer duplicates them — it
	// stays focused on the snowball-vs-avalanche decision.

	var body ui.Node
	switch {
	case !prepared.PlanKnown:
		message := "debt plan unavailable"
		if prepared.PlanErr != nil {
			message = prepared.PlanErr.Error()
		}
		body = P(css.Class("err"), Attr("role", "alert"), message)
	case len(debts) == 0:
		body = P(css.Class("empty"), uistate.T("planning.debtStrategyEmpty"))
	case !prepared.Comparison.Known:
		message := uistate.T("planning.strategyNotViable")
		if prepared.Comparison.Err != nil {
			message = prepared.Comparison.Err.Error()
		}
		body = P(css.Class("err"), Attr("role", "alert"), message)
	default:
		comparison := prepared.Comparison
		snow, aval := comparison.SnowballPlan, comparison.AvalanchePlan
		rec := Fragment()
		if comparison.HasInterestSaving && comparison.InterestSavedBy == payoff.Avalanche {
			rec = P(css.Class("muted"), uistate.T("planning.strategyRecommend", fmtMoney(comparison.InterestSaved)))
		}
		if comparison.HasTimeSaving {
			fasterLabel := uistate.T("planning.avalanche")
			if comparison.MonthsSavedBy == payoff.Snowball {
				fasterLabel = uistate.T("planning.snowball")
			}
			dur := strconv.Itoa(comparison.MonthsSaved) + " month"
			if comparison.MonthsSaved != 1 {
				dur += "s"
			}
			rec = Fragment(rec, P(css.Class("muted"), uistate.T("planning.strategyTimeSaved", fasterLabel, dur)))
		}
		explain := Fragment()
		if comparison.Match {
			explain = P(css.Class("budget-sub"), uistate.T("debt.strategiesMatch"))
		}
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
		burnChart := Fragment()
		if len(aval.Schedule) > 0 {
			startTotal := prepared.PlanOwed.Amount
			mkBurnPts := func(schedule []int64) []chartspec.Point {
				pts := make([]chartspec.Point, 0, len(schedule)+1)
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
				P(css.Class("budget-sub"), uistate.T("debt.burnDownHeading")),
				uiw.Chart(uiw.ChartProps{
					Spec: chartspec.Spec{Kind: chartspec.Line, Series: []chartspec.Series{
						{Name: uistate.T("planning.avalanche"), Points: mkBurnPts(aval.Schedule)},
						{Name: uistate.T("planning.snowball"), Points: mkBurnPts(snow.Schedule)},
					}, Y: chartspec.Axis{Format: yFmt}},
					Height: "150px",
					Label:  uistate.T("debt.burnDownChartLabel", aval.Months),
				}),
			)
		}
		snowWins := comparison.HasRecommendation && comparison.RecommendedStrategy == payoff.Snowball
		avalWins := comparison.HasRecommendation && comparison.RecommendedStrategy == payoff.Avalanche
		body = Div(
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
			// The extra monthly payment — the SAME canonical value the tuner sets. Editing
			// it here updates the tuner, ladder, and hero too. Committed on change so a
			// full-page recompute doesn't fire on every keystroke. On /debt the tuner owns
			// this control, so the field is hidden and a note points at it instead.
			IfElse(props.HideExtraInput,
				P(css.Class("budget-sub"), Attr("data-testid", "strat-extra-note"),
					uistate.T("debt.compareAtNote", fmtMoney(money.New(uistate.DebtConfigGet().DefaultExtraMinor, base)))),
				Div(css.Class("strat-extra"),
					labeledField(uistate.T("planning.debtStrategyExtra", base),
						Input(css.Class("field"), Type("number"), Attr("min", "0"), Attr("data-testid", "strat-extra"), Value(money.FormatMinor(uistate.DebtConfigGet().DefaultExtraMinor, dec)), Step("0.01"), OnChange(onExtra))),
					If(uistate.DebtConfigGet().DefaultExtraMinor == 0 && len(debts) > 1 && canSuggestExtra,
						Button(css.Class("btn strat-try"), Type("button"), Title(uistate.T("planning.fillSensibleTitle")),
							OnClick(onSuggestExtra),
							uistate.T("debt.tryExtra", fmtMoney(money.New(suggestedExtra, base))))),
				),
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

// DebtPlanner now lives in debt_widget.go as a widgetized surface host.

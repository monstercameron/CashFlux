// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
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
	var payoffLiabs []domain.Account // liabilities with a balance, for the include toggles
	for _, a := range app.Accounts() {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		if bal.Abs().Amount <= 0 {
			continue
		}
		payoffLiabs = append(payoffLiabs, a)
	}
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

	// Per-liability include/exclude toggles (each ToggleRow is its own component,
	// so the per-row hook is safe inside this loop).
	var includeToggles []ui.Node
	for _, a := range payoffLiabs {
		acc := a
		includeToggles = append(includeToggles, uiw.ToggleRow(uiw.ToggleRowProps{
			Label: acc.Name,
			On:    acc.IncludedInPayoff(),
			OnChange: func(on bool) {
				next := acc
				v := on
				next.IncludeInPayoff = &v
				if err := app.PutAccount(next); err != nil {
					return
				}
				rev.Set(rev.Get() + 1)
			},
		}))
	}

	// C201: per-liability APR + minimum-payment editors, so the plan's key
	// inputs can be tuned right here instead of detouring to /accounts. Each
	// row is its own component (ui.CreateElement), so the per-row hooks are
	// isolated even though one is rendered per liability in this loop.
	var rateEditRows []ui.Node
	for _, a := range payoffLiabs {
		acc := a
		rateEditRows = append(rateEditRows, ui.CreateElement(debtRateRow, debtRateRowProps{
			Acc: acc,
			OnSave: func(next domain.Account) {
				if err := app.PutAccount(next); err != nil {
					return
				}
				rev.Set(rev.Get() + 1)
			},
		}))
	}

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
			// C196: a per-debt detail table so the balances, APRs, and minimum
			// payments feeding the plan are visible — not hidden inside the
			// totals. Display-only rows (no per-row handlers), so a loop is safe.
			debtRows := make([]ui.Node, 0, len(debts))
			for _, d := range debts {
				apr := "—"
				if d.AprPercent > 0 {
					apr = fmt.Sprintf("%.2f%%", d.AprPercent)
				}
				minPay := "—"
				if d.MinPayment > 0 {
					minPay = fmtMoney(money.New(d.MinPayment, base))
				}
				debtRows = append(debtRows, Tr(css.Class(tw.BorderB, tw.BorderLine70),
					Td(css.Class(tw.Py25), d.Name),
					Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)), fmtMoney(money.New(d.Balance, base))),
					Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)), apr),
					Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)), minPay),
				))
			}
			debtTable := Div(Style(map[string]string{"margin-top": "0.6rem", "overflow-x": "auto"}),
				Table(css.Class("t-body", tw.WFull),
					Thead(Tr(css.Class(tw.BorderB, tw.BorderLine70),
						Th(css.Class(tw.TextLeft, tw.Py25, tw.TextDim), uistate.T("planning.debtColName")),
						Th(css.Class(tw.TextRight, tw.Py25, tw.TextDim), uistate.T("planning.debtColBalance")),
						Th(css.Class(tw.TextRight, tw.Py25, tw.TextDim), uistate.T("planning.debtColApr")),
						Th(css.Class(tw.TextRight, tw.Py25, tw.TextDim), uistate.T("planning.debtColMin")),
					)),
					Tbody(debtRows),
				),
			)
			body = Div(
				Div(css.Class("stat-grid"),
					stat(uistate.T("planning.snowball"), uistate.T("planning.strategyMonths", snow.Months), ""),
					stat(uistate.T("planning.avalanche"), uistate.T("planning.strategyMonths", aval.Months), ""),
				),
				debtTable,
				P(css.Class("budget-sub", tw.FontDisplay), "Debt-free by "+snowDate+" (snowball) · "+avalDate+" (avalanche)."),
				P(css.Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.snowball"), fmtMoney(money.New(snow.TotalInterest, base)))),
				P(css.Class("muted"), uistate.T("planning.strategyInterest", uistate.T("planning.avalanche"), fmtMoney(money.New(aval.TotalInterest, base)))),
				P(css.Class("muted"), "Payoff order: "+strings.Join(orderParts, " → ")),
				rec,
				explain,
				burnChart,
			)
		}
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("planning.debtStrategyTitle"),
		// C200: an HTML id anchor so the debt planner is directly linkable
		// (/planning#debt) and the dedicated /debt route can scroll to it.
		Attrs: []any{Attr("id", "debt")},
		Body: Fragment(
			P(css.Class("muted"), uistate.T("planning.debtStrategyHint")),
			Form(css.Class("form-grid"),
				labeledField(uistate.T("planning.debtStrategyExtra", base), Input(css.Class("field"), Type("number"), Attr("min", "0"), Value(dsExtra.Get()), Step("0.01"), OnInput(onDsExtra))),
			),
			// C202: the default ($0 extra) state ties snowball and avalanche, which
			// reads as "broken" unless explained. With 2+ debts, always say why
			// they match and how to make them diverge; offer a one-click suggested
			// extra when one is available. (A single debt ties inherently — no hint.)
			If(strings.TrimSpace(dsExtra.Get()) == "" && len(debts) > 1,
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
					Span(css.Class("muted"), uistate.T("planning.debtTieHint")),
					If(payoff.SuggestedExtra(debts) > 0,
						Button(css.Class("btn"), Type("button"), Title(uistate.T("planning.fillSensibleTitle")),
							OnClick(func() { dsExtra.Set(money.FormatMinor(payoff.SuggestedExtra(debts), currency.Decimals(base))) }),
							"Try "+fmtMoney(money.New(payoff.SuggestedExtra(debts), base))+"/mo"),
					),
				),
			),
			body,
			rateWarn,
			progressNode,
			If(len(includeToggles) > 0, Div(Style(map[string]string{"margin-top": "0.6rem"}),
				P(css.Class("budget-sub"), "Include in payoff plan (a mortgage is excluded by default):"),
				Div(includeToggles),
			)),
			If(len(rateEditRows) > 0, Div(Style(map[string]string{"margin-top": "0.6rem"}),
				P(css.Class("budget-sub"), uistate.T("planning.debtEditHeading")),
				Div(rateEditRows),
			)),
		),
	})
}

// DebtPlanner is the real /debt route — a focused "What you owe" screen showing
// a total-owed hero with a debt-free projection date, a compact read-only
// liability list, and the shared snowball-vs-avalanche strategy panel. It no
// longer aliases Planning(); it renders only debt content (FEATURE_MAP §5.7a).
func DebtPlanner() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	base := "USD"
	if b := app.Settings().BaseCurrency; b != "" {
		base = b
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	// Build the liability list, sorted by absolute balance descending so the
	// heaviest debt leads — same ordering convention as /accounts (G3 §7).
	var liabList []domain.Account
	for _, ac := range app.Accounts() {
		if !ac.Archived && ac.Class == domain.ClassLiability {
			liabList = append(liabList, ac)
		}
	}
	convBal := func(ac domain.Account) int64 {
		bal, _ := ledger.Balance(ac, txns)
		abs := bal.Abs()
		if c, err := rates.Convert(abs, base); err == nil {
			return c.Amount
		}
		return abs.Amount
	}
	sort.SliceStable(liabList, func(i, j int) bool { return convBal(liabList[i]) > convBal(liabList[j]) })

	// Empty state: the user has no liabilities — celebrate, don't nag.
	if len(liabList) == 0 {
		return Div(
			uiw.EntityListSection(uiw.EntityListSectionProps{
				Title: uistate.T("debt.whatYouOwe"),
				Body:  P(css.Class("empty"), uistate.T("debt.noDebts")),
			}),
		)
	}

	// Sum all liability absolute balances in base currency for the hero figure.
	// Every account's balance is FX-converted; those with missing rates contribute
	// zero rather than skewing the total (same policy as AggregateDebts).
	var totalOwedMinor int64
	for _, ac := range liabList {
		totalOwedMinor += convBal(ac)
	}
	totalOwed := money.New(totalOwedMinor, base)

	// Debt-free projection: use the avalanche plan at $0 extra (most optimistic
	// viable scenario). Only shown when the plan is actually viable (i.e. minimums
	// outpace interest). Uses current IncludedInPayoff settings, same as the panel.
	debts, _ := payoff.AggregateDebts(app.Accounts(), txns, base, rates)
	var debtFreeDate string
	if len(debts) > 0 {
		if aval, ok := payoff.BuildPlan(debts, 0, payoff.Avalanche); ok {
			debtFreeDate = payoff.DebtFreeMonth(time.Now(), aval.Months).Format("January 2006")
		}
	}
	var heroSub ui.Node = Fragment()
	if debtFreeDate != "" {
		heroSub = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("planning.debtFreeBy")+" "+debtFreeDate+" at current minimums.")
	}

	// Compact read-only liability rows: name, type badge, balance, APR if set,
	// utilization if a credit card with a known limit. No per-row interactive
	// elements here — editing stays on /accounts; no hooks needed.
	liabRows := make([]ui.Node, 0, len(liabList))
	var hasCreditCards bool
	var hasInstallmentLoans bool
	for _, ac := range liabList {
		bal, _ := ledger.Balance(ac, txns)
		balAbs := bal.Abs()
		typeLabel := uistate.T("acctType." + string(ac.Type))

		// Determine which optional sections to show below.
		if ac.Type == domain.TypeCreditCard {
			hasCreditCards = true
		}
		if isInstallmentLoan(ac.Type) {
			hasInstallmentLoans = true
		}

		var aprBadge ui.Node = Fragment()
		if ac.InterestRateAPR > 0 {
			aprBadge = Span(css.Class("muted"), fmt.Sprintf("%.2f%% APR", ac.InterestRateAPR))
		}

		var utilBadge ui.Node = Fragment()
		if ac.Type == domain.TypeCreditCard && ac.CreditLimit.Amount > 0 {
			util := int(balAbs.Amount * 100 / ac.CreditLimit.Amount)
			utilBadge = Span(css.Class("muted"), fmt.Sprintf("%d%% util.", util))
		}

		liabRows = append(liabRows, Div(css.Class("row"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("row-name"), ac.Name),
				Span(css.Class("t-caption", tw.TextDim), typeLabel),
			),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				aprBadge,
				utilBadge,
				Span(ClassStr("fig neg "+tw.Fold(tw.FontDisplay)), fmtMoney(balAbs)),
			),
		))
	}

	// The credit and loans panels are registered components (ui.CreateElement),
	// so their hooks are isolated and safe to use inside conditional If() calls.
	// Never inline hook-bearing code here — the panels own their own hook slots.
	return Div(
		// Hero: total owed across all liabilities + best-case debt-free date.
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("debt.whatYouOwe"),
			Body: Fragment(
				Div(css.Class("stat-grid"),
					stat(uistate.T("debt.totalOwed"), fmtMoney(totalOwed), "neg"),
				),
				heroSub,
			),
		}),
		// Compact liability account list — read-only; full edit lives on /accounts.
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("dashboard.liabilities"),
			HeaderAction: A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
				uistate.T("debt.manageAccounts")),
			Rows: liabRows,
		}),
		// Shared snowball-vs-avalanche strategy panel (also embedded in /planning).
		ui.CreateElement(DebtStrategyPanel, DebtStrategyPanelProps{}),
		// Credit health section — shown only when at least one credit card exists.
		// CreditHealthPanel is a registered component and owns its own hooks.
		If(hasCreditCards,
			uiw.EntityListSection(uiw.EntityListSectionProps{
				Title: uistate.T("nav.credit"),
				Body: Fragment(
					P(css.Class("muted"), uistate.T("screen.creditSub")),
					ui.CreateElement(CreditHealthPanel, CreditHealthPanelProps{}),
				),
			}),
		),
		// Loans section — shown only when at least one installment loan exists.
		// LoansPanel is a registered component and owns its own hooks.
		If(hasInstallmentLoans,
			uiw.EntityListSection(uiw.EntityListSectionProps{
				Title: uistate.T("nav.loans"),
				Body: Fragment(
					P(css.Class("muted"), uistate.T("screen.loansSub")),
					ui.CreateElement(LoansPanel, LoansPanelProps{}),
				),
			}),
		),
		// Manual single-debt payoff what-if (moved off /planning — FEATURE_MAP §5.3).
		// A registered component so its four input hooks stay in their own scope.
		ui.CreateElement(PayoffCalculatorPanel, PayoffCalculatorPanelProps{}),
	)
}

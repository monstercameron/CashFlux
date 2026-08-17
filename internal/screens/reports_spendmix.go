// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendmix"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T3b: how much of the month was a choice, and how far plans landed ────
//
// The report already said what was spent and on what. Neither answers what
// someone looking at a bad month is actually asking — how much of this could I
// have done anything about. A $4,200 month that is 80% commitments and the same
// $4,200 at 30% are completely different situations, and the category chart
// renders them identically.

// spendMixProps carries the reporting window, so this section can never describe
// a different period from the report it sits in.
type spendMixProps struct {
	From, To time.Time
}

// spendMixSection renders the commitment/discretionary split and the
// budget-variance table.
func spendMixSection(props spendMixProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	fmtM := func(v int64) string { return fmtMoney(money.New(v, base)) }

	mix := spendmix.Summarize(app.Transactions(), app.Categories(), props.From, props.To, rates)

	// Budget limits and spend come from `budgeting` — the same functions the
	// budgets screen uses — so this table can never disagree with that screen
	// about what was spent. Two independent computations of one figure eventually
	// differ, and the reader has no way to tell which is lying.
	limits := map[string]int64{}
	spends := map[string]int64{}
	labels := map[string]string{}
	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	for _, bg := range app.Budgets() {
		sp, err := budgeting.Spent(bg, app.Transactions(), props.From, props.To, rates)
		if err != nil {
			continue
		}
		limits[bg.ID] = bg.Limit.Amount
		spends[bg.ID] = sp.Amount
		if n := catName[bg.CategoryID]; n != "" {
			labels[bg.ID] = n
		} else {
			labels[bg.ID] = uistate.T("reports.uncategorized")
		}
	}
	variance := spendmix.Compare(limits, spends, labels, map[string]string{})

	return rptaSection("rpta-mix", "13", uistate.T("mix.secTitle"), "neutral",
		uistate.T("mix.secSub"), "", Fragment(
			spendMixBlock(mix, fmtM),
			budgetVarianceBlock(variance, fmtM),
		))
}

// spendMixBlock renders the committed-versus-chosen split.
func spendMixBlock(m spendmix.Mix, fmtM func(int64) string) ui.Node {
	if m.TotalMinor <= 0 {
		return Div(css.Class("tax-block"),
			rptaSubG("◑", "neutral", uistate.T("mix.splitTitle"), nil),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "mix-empty"),
				uistate.T("mix.empty")))
	}
	// The headline is the SHARE, not the amount. "You spent $1,300 on things you
	// chose" is a number; "31% of what you spent was a choice" is the answer to
	// the question, and it is the one that is comparable month to month.
	var headline ui.Node = Fragment()
	if pct, ok := m.FlexSharePct(); ok {
		headline = P(css.Class("t-figure", tw.FontDisplay), Attr("data-testid", "mix-share"),
			uistate.T("mix.share", pct, fmtM(m.FlexMinor)))
	}
	var unknown ui.Node = Fragment()
	if m.UncategorizedMinor > 0 {
		// Named, not folded into the discretionary bucket: calling unknown spending
		// "a choice" is the flattering guess, and it tells the reader they had more
		// room than they did.
		unknown = P(ClassStr("t-caption "+tw.ColorClass("text-down")), Attr("data-testid", "mix-unknown"),
			uistate.T("mix.unknown", fmtM(m.UncategorizedMinor)))
	}
	return Div(css.Class("tax-block"),
		rptaSubG("◑", "neutral", uistate.T("mix.splitTitle"), nil),
		headline,
		Div(css.Class("stat-grid"),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("mix.fixed")),
				Div(css.Class("stat-value"), Attr("data-testid", "mix-fixed"), fmtM(m.FixedMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("mix.nonMonthly")),
				Div(css.Class("stat-value"), Attr("data-testid", "mix-nonmonthly"), fmtM(m.NonMonthlyMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("mix.flex")),
				Div(css.Class("stat-value"), Attr("data-testid", "mix-flex"), fmtM(m.FlexMinor))),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("mix.committed")),
				Div(css.Class("stat-value"), Attr("data-testid", "mix-committed"), fmtM(m.CommittedMinor()))),
		),
		P(css.Class("t-caption", tw.TextDim), uistate.T("mix.note")),
		unknown,
	)
}

// budgetVarianceBlock renders how far each budget landed from plan.
func budgetVarianceBlock(r spendmix.Report, fmtM func(int64) string) ui.Node {
	if len(r.Rows) == 0 {
		return Div(css.Class("tax-block"),
			rptaSubG("⇅", "neutral", uistate.T("mix.varTitle"), nil),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "var-empty"),
				uistate.T("mix.varEmpty")))
	}
	// Over and under are stated separately and never netted. Being $300 over on
	// groceries and $300 under on travel is not a month that went to plan, and a
	// net of zero would say it was.
	return Div(css.Class("tax-block"),
		rptaSubG("⇅", "neutral", uistate.T("mix.varTitle"), nil),
		P(css.Class("t-body"), Attr("data-testid", "var-headline"),
			uistate.T("mix.varHeadline", fmtM(r.OverMinor), fmtM(r.UnderMinor))),
		Table(css.Class("tax-table"), Attr("data-testid", "var-table"),
			Thead(Tr(Th(uistate.T("mix.colBudget")), Th(uistate.T("mix.colPlanned")),
				Th(uistate.T("mix.colActual")), Th(uistate.T("mix.colDiff")))),
			Tbody(MapKeyed(r.Rows,
				func(v spendmix.Variance) any { return v.BudgetID },
				func(v spendmix.Variance) ui.Node {
					// A near miss reads as on plan rather than as a failure. Flagging a
					// $2 overspend on a $400 budget trains people to ignore the flag.
					word := uistate.T("mix.under", fmtM(-v.DeltaMinor))
					tone := "text-up"
					switch {
					case v.OnPlan():
						word, tone = uistate.T("mix.onPlan"), "text-dim"
					case v.Over():
						word, tone = uistate.T("mix.over", fmtM(v.DeltaMinor)), "text-down"
					}
					return Tr(Attr("data-testid", "var-row-"+v.BudgetID),
						Td(v.Label),
						Td(fmtM(v.LimitMinor)),
						Td(fmtM(v.SpentMinor)),
						Td(ClassStr(tw.ColorClass(tone)), word))
				})),
		),
	)
}

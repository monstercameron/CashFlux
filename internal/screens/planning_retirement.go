// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/retirement"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// retirementTile is the long-horizon projection card on /planning (FP-T1a,
// FP-T1b).
//
// Retirement was an account type with no arithmetic behind it. This is the
// surface over `internal/retirement`: how much the pot reaches by a chosen age,
// how long it then lasts, and the FIRE number the same spending implies.
//
// Every figure leads with TODAY's money. A projection of $2.1M in 2056 is
// technically correct and communicates nothing, because nobody can price 2056
// groceries; the nominal figure is available beside it, not in front of it.
//
// The assumptions are on screen and adjustable, not buried. A projection is
// arithmetic over guesses, and a guess the reader cannot see is a guess they
// will mistake for a measurement.
func retirementTile(base string) ui.Node {
	_ = uistate.UseDataRevision().Get()
	cfg := uistate.LoadRetirementPlan()

	onYears := ui.UseEvent(func(v string) {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= retirement.MaxYears {
			cur := uistate.LoadRetirementPlan()
			cur.YearsToRetirement = n
			uistate.SaveRetirementPlan(cur)
		}
	})
	onReturn := ui.UseEvent(func(v string) { saveRetirementFloat(v, func(c *uistate.RetirementPlan, f float64) { c.ReturnPct = f }) })
	onInflation := ui.UseEvent(func(v string) {
		saveRetirementFloat(v, func(c *uistate.RetirementPlan, f float64) { c.InflationPct = f })
	})
	onContribution := ui.UseEvent(func(v string) {
		saveRetirementMinor(v, base, func(c *uistate.RetirementPlan, m int64) { c.AnnualContributionMinor = m })
	})
	onSpend := ui.UseEvent(func(v string) {
		saveRetirementMinor(v, base, func(c *uistate.RetirementPlan, m int64) { c.AnnualSpendMinor = m })
	})

	// The starting pot is the household's own retirement accounts, not a typed
	// number: asking someone to re-enter a balance the app already holds is how a
	// projection comes to disagree with the accounts page it sits beside.
	startMinor, accounts := retirementStartMinor(base)

	a := retirement.Assumptions{ReturnPct: cfg.ReturnPct, InflationPct: cfg.InflationPct}
	proj := retirement.Project(startMinor, cfg.AnnualContributionMinor, cfg.YearsToRetirement, a)

	var body ui.Node
	switch {
	case accounts == 0:
		// Nothing to project from. Saying WHICH accounts would feed it beats an
		// empty card that looks broken.
		body = P(css.Class("muted"), Attr("data-testid", "plan-retire-none"),
			uistate.T("retire.noAccounts"))
	case !proj.Known:
		body = P(css.Class("muted"), Attr("data-testid", "plan-retire-unset"),
			uistate.T("retire.needsHorizon"))
	default:
		body = retirementResults(proj, cfg, a, base)
	}

	return planTile("plan-retirement", planSection("plan-retirement-sec",
		uistate.T("retire.title"), Fragment(),
		Fragment(
			P(css.Class("muted"), Attr("data-testid", "plan-retire-basis"),
				uistate.T("retire.basis", accounts, fmtMoney(money.New(startMinor, base)))),
			retirementInputs(cfg, base, onYears, onReturn, onInflation, onContribution, onSpend),
			// FP-T2d: the inflation field is the household's one assumption, not this
			// card's. Saying so keeps a reader from setting it here, seeing the
			// forecast move, and taking that for a bug.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "plan-retire-shared"),
				uistate.T("retire.inflationShared")),
			body,
		)))
}

// retirementResults renders the projection, the drawdown and the FIRE number.
func retirementResults(proj retirement.Projection, cfg uistate.RetirementPlan,
	a retirement.Assumptions, base string) ui.Node {

	// Real first. The nominal figure is beside it, deliberately smaller, because
	// it is the one that flatters and the one that means less.
	real := money.New(proj.FinalRealMinor, base)
	nominal := money.New(proj.FinalNominalMinor, base)

	var drawNode ui.Node = Fragment()
	if cfg.AnnualSpendMinor > 0 {
		// The drawdown horizon is deliberately generous: the question is "does it
		// last", and a horizon that ends before the money does would answer it by
		// running out of chart rather than out of money.
		d := retirement.RunDrawdown(proj.FinalNominalMinor, cfg.AnnualSpendMinor, retirementDrawdownYears, a)
		switch {
		case !d.Known:
			drawNode = Fragment()
		case d.Depleted:
			drawNode = P(ClassStr("retire-read retire-short"), Attr("data-testid", "plan-retire-lasts"),
				uistate.T("retire.lastsYears", d.LastsYears))
		default:
			drawNode = P(ClassStr("retire-read retire-ok"), Attr("data-testid", "plan-retire-lasts"),
				uistate.T("retire.lastsBeyond", retirementDrawdownYears,
					fmtMoney(money.New(d.EndingRealMinor, base))))
		}
	}

	var fireNode ui.Node = Fragment()
	if n, ok := retirement.FIRENumber(cfg.AnnualSpendMinor, retirement.DefaultSWRPct); ok {
		years, reachable := retirement.YearsToFI(
			proj.Years[0].NominalMinor, cfg.AnnualContributionMinor, n, a.RealReturnPct())
		line := uistate.T("retire.fireUnreachable", fmtMoney(money.New(n, base)))
		if reachable {
			line = uistate.T("retire.fireYears", fmtMoney(money.New(n, base)), years)
		}
		fireNode = Fragment(
			P(css.Class("retire-read"), Attr("data-testid", "plan-retire-fire"), line),
			// Say where the 4% came from. A rule of thumb about one historical
			// period, presented as arithmetic, is the kind of number people plan a
			// decade around.
			P(css.Class("muted", tw.Text12), uistate.T("retire.fireSource", retirement.DefaultSWRPct)),
		)
	}

	return Fragment(
		P(css.Class("retire-headline"), Attr("data-testid", "plan-retire-pot"),
			uistate.T("retire.potReal", fmtMoney(real), cfg.YearsToRetirement)),
		P(css.Class("muted", tw.Text12), Attr("data-testid", "plan-retire-nominal"),
			uistate.T("retire.potNominal", fmtMoney(nominal))),
		P(css.Class("muted", tw.Text12), Attr("data-testid", "plan-retire-split"),
			uistate.T("retire.split",
				fmtMoney(money.New(proj.ContributedMinor, base)),
				fmtMoney(money.New(proj.GrowthMinor(), base)))),
		drawNode,
		fireNode,
		// The assumptions, restated under the figures they produced.
		P(css.Class("muted", tw.Text12), Attr("data-testid", "plan-retire-assumptions"),
			uistate.T("retire.assumptions", a.ReturnPct, a.InflationPct, a.RealReturnPct())),
	)
}

// retirementDrawdownYears is how far the "will it last" run looks.
//
// Forty years, so a retirement at 60 is covered to 100. A shorter horizon would
// answer "does it last" by running out of chart rather than out of money, which
// reads as a pass.
const retirementDrawdownYears = 40

// retirementInputs renders the adjustable assumptions.
func retirementInputs(cfg uistate.RetirementPlan, base string,
	onYears, onReturn, onInflation, onContribution, onSpend ui.Handler) ui.Node {

	num := func(label, testid, val string, h ui.Handler, step string) ui.Node {
		return Label(css.Class("retire-field"),
			Span(css.Class("t-caption"), label),
			Input(css.Class("field"), Type("number"), Attr("step", step),
				Attr("data-testid", testid), Attr("aria-label", label),
				uiw.FieldValue(val), OnChange(h)))
	}
	return Div(css.Class("retire-inputs"),
		num(uistate.T("retire.years"), "plan-retire-years", strconv.Itoa(cfg.YearsToRetirement), onYears, "1"),
		num(uistate.T("retire.contribution", base), "plan-retire-contrib",
			majorString(cfg.AnnualContributionMinor, base), onContribution, "100"),
		num(uistate.T("retire.spend", base), "plan-retire-spend",
			majorString(cfg.AnnualSpendMinor, base), onSpend, "100"),
		num(uistate.T("retire.return"), "plan-retire-return",
			strconv.FormatFloat(cfg.ReturnPct, 'f', -1, 64), onReturn, "0.1"),
		num(uistate.T("retire.inflation"), "plan-retire-inflation",
			strconv.FormatFloat(cfg.InflationPct, 'f', -1, 64), onInflation, "0.1"),
	)
}

// retirementStartMinor sums the household's retirement-type accounts, and
// reports how many fed the figure.
//
// Reading the accounts rather than asking for a number is the point: a typed
// starting balance is one more thing to keep in sync, and the moment it drifts
// the projection disagrees with the accounts page beside it.
func retirementStartMinor(base string) (int64, int) {
	app := appstate.Default
	if app == nil {
		return 0, 0
	}
	txns := app.Transactions()
	var total int64
	var n int
	for _, a := range app.Accounts() {
		if a.Archived || a.Type != domain.TypeRetirement {
			continue
		}
		n++
		bal := a.OpeningBalance.Amount
		for _, t := range txns {
			if t.AccountID == a.ID {
				bal += t.Amount.Amount
			}
		}
		total += bal
	}
	return total, n
}

// saveRetirementFloat parses a percent field and persists it. A value that does
// not parse is IGNORED rather than zeroed — half-typed input passes through
// every keystroke, and zeroing on the way would wipe the field as the user types.
func saveRetirementFloat(v string, set func(*uistate.RetirementPlan, float64)) {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return
	}
	cur := uistate.LoadRetirementPlan()
	set(&cur, f)
	uistate.SaveRetirementPlan(cur)
}

// saveRetirementMinor parses a major-unit money field into minor units.
func saveRetirementMinor(v, base string, set func(*uistate.RetirementPlan, int64)) {
	m, ok := parseMajorMinor(v, base)
	if !ok {
		return
	}
	cur := uistate.LoadRetirementPlan()
	set(&cur, m)
	uistate.SaveRetirementPlan(cur)
}

// majorString renders minor units as a plain major-unit number for a numeric
// input — no symbol, no grouping, because a number input rejects both.
func majorString(minor int64, base string) string {
	return strconv.FormatFloat(currency.MajorFromMinor(minor, base), 'f', -1, 64)
}

// parseMajorMinor reads a major-unit number into minor units, reporting failure
// rather than zero so a half-typed value does not wipe the stored one.
func parseMajorMinor(v, base string) (int64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return currency.MinorFromMajor(f, base), true
}

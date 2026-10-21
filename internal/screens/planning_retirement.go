// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/planningmodel"
	"github.com/monstercameron/CashFlux/internal/retirement"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
	facts := cachedRetirementFacts(appstate.Default, cfg, base)

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

	var body ui.Node
	switch {
	case !facts.StartingPot.Available:
		body = P(css.Class("muted"), Attr("data-testid", "plan-retire-unavailable"),
			uistate.T("retire.balancesUnavailable"))
	case facts.AccountCount == 0:
		// Nothing to project from. Saying WHICH accounts would feed it beats an
		// empty card that looks broken.
		body = P(css.Class("muted"), Attr("data-testid", "plan-retire-none"),
			uistate.T("retire.noAccounts"))
	case !facts.Projection.Available:
		body = P(css.Class("muted"), Attr("data-testid", "plan-retire-unset"),
			uistate.T("retire.needsHorizon"))
	default:
		body = retirementResults(facts, cfg, base)
	}
	var basis ui.Node
	if facts.StartingPot.Available {
		basis = P(css.Class("muted"), Attr("data-testid", "plan-retire-basis"),
			uistate.T("retire.basis", facts.AccountCount, fmtMoney(facts.StartingPot.Value)))
	}

	return planTile("plan-retirement", planSection("plan-retirement-sec",
		uistate.T("retire.title"), Fragment(),
		Fragment(
			basis,
			retirementInputs(cfg, base, onYears, onReturn, onInflation, onContribution, onSpend),
			// FP-T2d: the inflation field is the household's one assumption, not this
			// card's. Saying so keeps a reader from setting it here, seeing the
			// forecast move, and taking that for a bug.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "plan-retire-shared"),
				uistate.T("retire.inflationShared")),
			body,
		)))
}

var retirementFactsCache = newViewCache[planningmodel.RetirementFacts](2)

func cachedRetirementFacts(app *appstate.App, cfg uistate.RetirementPlan, base string) planningmodel.RetirementFacts {
	accounts, accountsErr := app.AccountsChecked()
	txns, txnsErr := app.TransactionsChecked()
	settings, settingsErr := app.SettingsChecked()
	settingsBase := strings.TrimSpace(settings.BaseCurrency)
	if settingsErr == nil && settingsBase != "" && !strings.EqualFold(settingsBase, strings.TrimSpace(base)) {
		settingsErr = errors.Join(settingsErr, errors.New("planning: base currency changed while preparing retirement facts"))
	}

	snapshot := planningmodel.Snapshot{
		Accounts: accounts, AccountsErr: accountsErr,
		Transactions: txns, TransactionsErr: txnsErr,
	}
	inputs := planningmodel.RetirementInputs{
		Base: base, FXRates: settings.FXRates, SettingsErr: settingsErr,
		YearsToRetirement:       cfg.YearsToRetirement,
		AnnualContributionMinor: cfg.AnnualContributionMinor,
		AnnualSpendMinor:        cfg.AnnualSpendMinor,
		ReturnPct:               cfg.ReturnPct, InflationPct: cfg.InflationPct,
	}
	if errors.Join(accountsErr, txnsErr, settingsErr) != nil {
		return planningmodel.PrepareRetirement(snapshot, inputs)
	}

	key := retirementFactsKey(app, cfg, base, settings.BaseCurrency, settings.FXRates)
	return memoByRev(retirementFactsCache, key, func() planningmodel.RetirementFacts {
		return planningmodel.PrepareRetirement(snapshot, inputs)
	})
}

// retirementFactsKey identifies every mutable input to the deterministic
// retirement preparation: app/store and UI revisions, exact plan assumptions,
// the selected currency, and the full FX table. The bounded cache can therefore
// reuse completed simulation output without retaining a result for a partial key.
func retirementFactsKey(app *appstate.App, cfg uistate.RetirementPlan, base, settingsBase string, rates map[string]float64) string {
	var b strings.Builder
	b.WriteString(revKey(app))
	b.WriteString("|base=")
	b.WriteString(strconv.Quote(base))
	b.WriteString("|settings-base=")
	b.WriteString(strconv.Quote(settingsBase))
	b.WriteString("|years=")
	b.WriteString(strconv.Itoa(cfg.YearsToRetirement))
	b.WriteString("|contribution=")
	b.WriteString(strconv.FormatInt(cfg.AnnualContributionMinor, 10))
	b.WriteString("|spend=")
	b.WriteString(strconv.FormatInt(cfg.AnnualSpendMinor, 10))
	b.WriteString("|return=")
	b.WriteString(strconv.FormatUint(math.Float64bits(cfg.ReturnPct), 16))
	b.WriteString("|inflation=")
	b.WriteString(strconv.FormatUint(math.Float64bits(cfg.InflationPct), 16))

	codes := make([]string, 0, len(rates))
	for code := range rates {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		b.WriteString("|fx=")
		b.WriteString(strconv.Quote(code))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(math.Float64bits(rates[code]), 16))
	}
	return b.String()
}

// retirementResults renders the projection, the drawdown and the FIRE number.
func retirementResults(facts planningmodel.RetirementFacts, cfg uistate.RetirementPlan, base string) ui.Node {
	proj := facts.Projection.Value
	a := facts.Assumptions

	// Real first. The nominal figure is beside it, deliberately smaller, because
	// it is the one that flatters and the one that means less.
	real := money.New(proj.FinalRealMinor, base)
	nominal := money.New(proj.FinalNominalMinor, base)

	var drawNode ui.Node = Fragment()
	if facts.Drawdown.Available {
		// The drawdown horizon is deliberately generous: the question is "does it
		// last", and a horizon that ends before the money does would answer it by
		// running out of chart rather than out of money.
		d := facts.Drawdown.Value
		switch {
		case d.Depleted:
			drawNode = P(ClassStr("retire-read retire-short"), Attr("data-testid", "plan-retire-lasts"),
				uistate.T("retire.lastsYears", d.LastsYears))
		default:
			drawNode = P(ClassStr("retire-read retire-ok"), Attr("data-testid", "plan-retire-lasts"),
				uistate.T("retire.lastsBeyond", planningmodel.RetirementDrawdownYears,
					fmtMoney(money.New(d.EndingRealMinor, base))))
		}
	}

	// FP-T3d: the same drawdown as a PROBABILITY. The line above is one future at
	// a fixed rate; this is how often the money lasted across two thousand of them.
	// Sequence-of-returns risk is invisible to a single-rate projection — the same
	// average with the bad years first can exhaust a portfolio the same years at
	// the end would have left intact.
	var mcNode ui.Node = Fragment()
	if facts.MonteCarlo.Available {
		mc := facts.MonteCarlo.Value
		tone := "retire-ok"
		if mc.SuccessRatePct < 80 {
			tone = "retire-short"
		}
		var failNode ui.Node = Fragment()
		if mc.Depleted {
			// "92% success" with the failures in year 4 is a different situation
			// from "92%" with them in year 28, and the bare percentage hides which.
			failNode = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "plan-retire-mc-fail"),
				uistate.T("retire.mcFail", mc.WorstDepletionYear))
		}
		mcNode = Fragment(
			P(ClassStr("retire-read "+tone), Attr("data-testid", "plan-retire-mc"),
				uistate.T("retire.mc", mc.SuccessRatePct, planningmodel.RetirementDrawdownYears)),
			failNode,
			// The method is stated, not implied. A probability with no method is
			// a number people plan a decade around and cannot check.
			P(css.Class("muted", tw.Text12), Attr("data-testid", "plan-retire-mc-method"),
				uistate.T("retire.mcMethod", mc.Iterations, mc.Seed)),
		)
	}

	var fireNode ui.Node = Fragment()
	if facts.FI.Available {
		fi := facts.FI.Value
		line := uistate.T("retire.fireUnreachable", fmtMoney(money.New(fi.TargetMinor, base)))
		if fi.Reachable {
			line = uistate.T("retire.fireYears", fmtMoney(money.New(fi.TargetMinor, base)), fi.Years)
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
		mcNode,
		fireNode,
		// The assumptions, restated under the figures they produced.
		P(css.Class("muted", tw.Text12), Attr("data-testid", "plan-retire-assumptions"),
			uistate.T("retire.assumptions", a.ReturnPct, a.InflationPct, a.RealReturnPct())),
	)
}

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

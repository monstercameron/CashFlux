// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/vitals"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// rptaVitalsSection renders "00 · Where you stand" — the position snapshot that
// opens the Annual Review before the year's story. Three ruled ledger columns
// (cash flow, the cushion, debt & credit) of vital rows: small-caps label, a
// toned display figure, and a one-line plain-English reading that states the
// figure's own basis. Bounded vitals (savings rate, coverage, payment share,
// utilization) carry a hairline meter whose TARGET TICK marks the published
// threshold, so distance-to-target reads as structure, not prose. Every row is
// applicability-gated — a household with no debts sees one quiet sentence, not
// a column of fake zeros. Pure rendering over the tested internal/vitals core.
// hhChip is the "Household-wide" tag shown in the section head while a report
// scope is active (a balance sheet has no scope); pass Fragment() when not.
func rptaVitalsSection(vt vitals.Result, monthsAvgd int, fixedMinor, essSpendMinor int64, fmtMinor func(int64) string, hhChip ui.Node) ui.Node {
	pctStr := func(p int) string { return fmt.Sprintf("%d%%", p) }
	moStr := func(tenths int64) string {
		return uistate.T("rpta.standMonthsVal", fmt.Sprintf("%d.%d", tenths/10, tenths%10))
	}

	// ── Cash flow ──
	var flowRows []ui.Node
	if vt.HasIncome {
		flowRows = append(flowRows,
			rptaVital("income", uistate.T("rpta.standIncomeK"), fmtMinor(vt.IncomeMonthlyMinor), vitals.ToneNone, "", nil),
			rptaVital("spend", uistate.T("rpta.standSpendK"), fmtMinor(vt.ExpenseMonthlyMinor), vitals.ToneNone, "", nil),
			rptaVital("kept", uistate.T("rpta.standKeptK"), fmtMinor(vt.SurplusMonthlyMinor), vt.SurplusTone,
				uistate.T("rpta.standKeptR", fmtMinor(vt.SurplusAnnualMinor)), nil),
			rptaVital("rate", uistate.T("rpta.standRateK"), pctStr(vt.SavingsRatePct), vt.SavingsTone,
				uistate.T("rpta.standRateR", vitals.SavingsTargetPct),
				rptaVitalMeter(vt.SavingsRatePct, 40, vitals.SavingsTargetPct, vt.SavingsTone,
					uistate.T("rpta.standMeterTitle", pctStr(vt.SavingsRatePct), pctStr(vitals.SavingsTargetPct)))),
			rptaVital("free", uistate.T("rpta.standFreeK"), fmtMinor(vt.DiscretionaryMinor), vt.DiscretionaryTone,
				uistate.T("rpta.standFreeR"), nil),
		)
	} else {
		flowRows = append(flowRows, P(css.Class("rpta-vital-empty"), uistate.T("rpta.standNoIncome")))
	}

	// ── The cushion ──
	var cushRows []ui.Node
	if vt.HasCushion {
		essReading := uistate.T("rpta.standEssR", fmtMinor(fixedMinor), fmtMinor(essSpendMinor))
		switch {
		case essSpendMinor <= 0:
			essReading = uistate.T("rpta.standEssRFixed")
		case fixedMinor <= 0:
			essReading = uistate.T("rpta.standEssRSpend")
		}
		cushRows = append(cushRows,
			rptaVital("essential", uistate.T("rpta.standEssK"), fmtMinor(vt.EssentialMonthlyMinor), vitals.ToneNone,
				essReading, nil),
			rptaVital("liquid", uistate.T("rpta.standLiquidK"), fmtMinor(vt.LiquidMinor), vitals.ToneNone, "", nil),
			rptaVital("coverage", uistate.T("rpta.standCoverK"), moStr(vt.CoverageMonthsTenths), vt.CoverageTone,
				uistate.T("rpta.standCoverR"),
				rptaVitalMeter(int(vt.CoverageMonthsTenths), 80, vitals.CoverageTargetTenths, vt.CoverageTone,
					uistate.T("rpta.standMeterTitle", moStr(vt.CoverageMonthsTenths), moStr(vitals.CoverageTargetTenths)))),
			rptaVital("fund", uistate.T("rpta.standFundK", vt.FundMonths), fmtMinor(vt.FundTargetMinor), vitals.ToneNone,
				rptaFundGapReading(vt.FundGapMinor, fmtMinor), nil),
		)
		if vt.RunwayAfterDebtTenths >= 0 {
			cushRows = append(cushRows,
				rptaVital("runway", uistate.T("rpta.standRunwayK"), moStr(vt.RunwayAfterDebtTenths), vt.RunwayTone,
					uistate.T("rpta.standRunwayR"), nil))
		}
	} else {
		cushRows = append(cushRows, P(css.Class("rpta-vital-empty"), uistate.T("rpta.standNoCushion")))
	}

	// ── Debt & credit ──
	var debtRows []ui.Node
	if vt.HasDebts {
		mortReading := ""
		if vt.HasMortgage {
			mortReading = uistate.T("rpta.standDebtMortR", fmtMinor(vt.ExMortgageMinor))
		}
		debtRows = append(debtRows,
			rptaVital("debt", uistate.T("rpta.standDebtK"), fmtMinor(vt.TotalDebtMinor), vitals.ToneNone, mortReading, nil),
		)
		if vt.MinPaymentsMinor > 0 {
			debtRows = append(debtRows,
				rptaVital("mins", uistate.T("rpta.standMinsK"), fmtMinor(vt.MinPaymentsMinor), vitals.ToneNone,
					uistate.T("rpta.standMinsR", fmtMinor(vt.AnnualDebtServiceMinor)), nil))
		}
		if vt.PaymentShareTone != vitals.ToneNone {
			debtRows = append(debtRows,
				rptaVital("dti", uistate.T("rpta.standDtiK"), pctStr(vt.PaymentShareOfIncomePct), vt.PaymentShareTone,
					uistate.T("rpta.standDtiR", vitals.PaymentShareTargetPct),
					rptaVitalMeter(vt.PaymentShareOfIncomePct, 60, vitals.PaymentShareTargetPct, vt.PaymentShareTone,
						uistate.T("rpta.standMeterTitle", pctStr(vt.PaymentShareOfIncomePct), pctStr(vitals.PaymentShareTargetPct)))))
		}
		if vt.WeightedAprTone != vitals.ToneNone {
			debtRows = append(debtRows,
				rptaVital("apr", uistate.T("rpta.standAprK"), fmt.Sprintf("%.1f%%", vt.WeightedAprPercent), vt.WeightedAprTone,
					uistate.T("rpta.standAprR", fmtMinor(vt.InterestDragMonthlyMinor)), nil))
		}
		if vt.PayoffApplicable {
			val, reading := rptaPayoffReadout(vt)
			debtRows = append(debtRows, rptaVital("payoff", uistate.T("rpta.standPayoffK"), val, vt.PayoffTone, reading, nil))
		}
	} else {
		debtRows = append(debtRows, P(css.Class("rpta-vital-empty"), uistate.T("rpta.standNoDebt")))
	}
	if vt.HasCards {
		if vt.HasUtilization {
			debtRows = append(debtRows,
				rptaVital("util", uistate.T("rpta.standUtilK"), pctStr(vt.UtilizationPct), vt.UtilizationTone,
					uistate.T("rpta.standUtilR", fmtMinor(vt.CardBalanceMinor), fmtMinor(vt.CardLimitMinor), fmtMinor(vt.CardAvailableMinor)),
					rptaVitalMeter(vt.UtilizationPct, 100, vitals.UtilizationTargetPct, vt.UtilizationTone,
						uistate.T("rpta.standMeterTitle", pctStr(vt.UtilizationPct), pctStr(vitals.UtilizationTargetPct)))))
		} else {
			debtRows = append(debtRows, P(css.Class("rpta-vital-empty"), uistate.T("rpta.standUtilNoLim")))
		}
	}

	col := func(headKey, testid string, rows []ui.Node) ui.Node {
		return Div(css.Class("rpta-vit-col"), Attr("data-testid", testid),
			Div(css.Class("rpta-vit-head"), uistate.T(headKey)),
			Fragment(anyify(rows)...),
		)
	}

	ask := rptaVitalsAsk(vt, fmtMinor)
	basisLine := ""
	if vt.HasIncome && monthsAvgd > 0 {
		basisLine = uistate.T("rpta.standBasis", monthsAvgd)
	}
	return rptaSectionWithAction("rpta-00", "00", uistate.T("rpta.secStand"), "neutral", uistate.T("rpta.secStandSub"), ask, hhChip, Fragment(
		Div(css.Class("rpta-vitals"), Attr("data-testid", "rpta-vitals"),
			col("rpta.standColFlow", "rpta-vit-flow", flowRows),
			col("rpta.standColCushion", "rpta-vit-cushion", cushRows),
			col("rpta.standColDebt", "rpta-vit-debt", debtRows),
		),
		If(basisLine != "", P(css.Class("rpta-muted", "rpta-vitals-basis"), basisLine)),
	))
}

// rptaVital is one vital row: small-caps label, toned display figure, muted
// one-line reading, and an optional target-tick meter.
func rptaVital(key, label, value string, tone vitals.Tone, reading string, meter ui.Node) ui.Node {
	vCls := "rpta-vital-v " + tw.Fold(tw.FontDisplay)
	if tone != vitals.ToneNone {
		vCls += " rpta-tone-" + string(tone)
	}
	if meter == nil {
		meter = Fragment()
	}
	return Div(css.Class("rpta-vital"), Attr("data-testid", "rpta-vital-"+key),
		Span(css.Class("rpta-vital-k"), label),
		Span(ClassStr(vCls), value),
		meter,
		If(reading != "", Span(css.Class("rpta-vital-r"), reading)),
	)
}

// rptaVitalMeter is the section's signature device: a hairline track whose fill
// is the metric's toned value and whose single tick marks the published target,
// so "how far from the line am I?" reads spatially. value/target share the
// metric's own unit; scale is the track's honest full width in that unit.
func rptaVitalMeter(value, scale, target int, tone vitals.Tone, title string) ui.Node {
	if scale <= 0 {
		return Fragment()
	}
	fill := min(max(value*100/scale, 0), 100)
	tick := min(target*100/scale, 100)
	return Div(css.Class("rpta-vital-meter"), Attr("aria-hidden", "true"), Title(title),
		Div(ClassStr("rpta-vital-fill rpta-fill-"+string(tone)), Style(map[string]string{"width": fmt.Sprintf("%d%%", fill)})),
		Div(css.Class("rpta-vital-tick"), Style(map[string]string{"left": fmt.Sprintf("%d%%", tick)})),
	)
}

// rptaFundGapReading phrases the fund row's gap: short of, past, or exactly at
// the target.
func rptaFundGapReading(gapMinor int64, fmtMinor func(int64) string) string {
	switch {
	case gapMinor > 0:
		return uistate.T("rpta.standFundShort", fmtMinor(gapMinor))
	case gapMinor < 0:
		return uistate.T("rpta.standFundMet", fmtMinor(-gapMinor))
	default:
		return uistate.T("rpta.standFundExact")
	}
}

// rptaPayoffReadout renders the debt-free horizon: "N yr M mo" at minimums, or
// the honest never-clears verdict.
func rptaPayoffReadout(vt vitals.Result) (value, reading string) {
	if vt.PayoffNeverClears {
		return uistate.T("rpta.standPayoffNever"), uistate.T("rpta.standPayoffBadR")
	}
	m := vt.PayoffMonths
	if m >= 24 {
		value = uistate.T("rpta.standPayoffYrMo", m/12, m%12)
	} else {
		value = uistate.T("rpta.standPayoffMo", m)
	}
	reading = uistate.T("rpta.standPayoffR")
	if vt.HasMortgage {
		reading += uistate.T("rpta.standPayoffXMort")
	}
	return value, reading
}

// rptaVitalsAsk composes the section's assistant seed from the live figures.
func rptaVitalsAsk(vt vitals.Result, fmtMinor func(int64) string) string {
	ask := ""
	if vt.HasIncome {
		ask += fmt.Sprintf("keeping %s a month (savings rate %d%%), %s free after debt minimums; ",
			fmtMinor(vt.SurplusMonthlyMinor), vt.SavingsRatePct, fmtMinor(vt.DiscretionaryMinor))
	}
	if vt.HasCushion {
		ask += fmt.Sprintf("liquid cash covers %d.%d months of essentials (target %d); ",
			vt.CoverageMonthsTenths/10, vt.CoverageMonthsTenths%10, vt.FundMonths)
	}
	if vt.HasDebts {
		ask += fmt.Sprintf("total debt %s with %s/mo required (%d%% of income) at a blended %.1f%%; ",
			fmtMinor(vt.TotalDebtMinor), fmtMinor(vt.MinPaymentsMinor), vt.PaymentShareOfIncomePct, vt.WeightedAprPercent)
		if vt.PayoffNeverClears {
			ask += "minimum payments never clear the debt; "
		} else if vt.PayoffApplicable {
			ask += fmt.Sprintf("debt-free in ~%d months at minimums; ", vt.PayoffMonths)
		}
	}
	if vt.HasUtilization {
		ask += fmt.Sprintf("card utilization %d%%", vt.UtilizationPct)
	}
	return ask
}

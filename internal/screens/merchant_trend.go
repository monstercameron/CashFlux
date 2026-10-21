// SPDX-License-Identifier: MIT

//go:build js && wasm

// Merchant spending context for the transaction editor. The shared story combines
// recent charges, typical spending, visit frequency, and the current month's total.
package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/merchantstats"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// toBaseMag converts a money amount to a positive magnitude in the base currency.
func toBaseMag(app *appstate.App, m money.Money, base string) (int64, bool) {
	if m.Currency == "" || m.Currency == base {
		return absMinor(m.Amount), true
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	v, err := currency.ConvertBetween(m.Amount, m.Currency, base, rates)
	if err != nil {
		return 0, false
	}
	return absMinor(v), true
}

// computeMerchantStats gathers this merchant's expense charges (FX-converted to base)
// and computes its merchantstats. Returns (stats, base, ok) where ok is false for a
// one-off merchant with too little history to be worth showing.
func computeMerchantStats(app *appstate.App, merchant string) (merchantstats.Stats, string, bool) {
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return merchantstats.Stats{}, "", false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	resolver := app.PayeeResolver()
	target := strings.ToLower(merchant)

	toBase := func(m money.Money) (int64, bool) {
		if m.Currency == "" || m.Currency == base {
			return absMinor(m.Amount), true
		}
		v, err := currency.ConvertBetween(m.Amount, m.Currency, base, rates)
		if err != nil {
			return 0, false
		}
		return absMinor(v), true
	}

	var charges []merchantstats.Charge
	for _, t := range app.Transactions() {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if strings.ToLower(strings.TrimSpace(resolver.Resolve(firstNonEmpty(t.Payee, t.Desc)))) != target {
			continue
		}
		mag, ok := toBase(t.Amount)
		if !ok {
			continue
		}
		charges = append(charges, merchantstats.Charge{Date: t.Date, Minor: mag})
	}
	stats := merchantstats.Compute(charges, time.Now(), uistate.LoadPrefs().WeekStartWeekday())
	return stats, base, stats.Enough
}

// merchantStoryNodes renders the shared merchant "story" content — the title + this-vs-
// typical delta, visit frequency, month-vs-typical line, and the recent-charge
// sparkline — used by BOTH the edit-modal panel and the row trend popover. thisMag is
// this transaction's magnitude in base (for the delta); hasThis is false when there is
// no single charge to compare (the popover always has one, the panel usually does).
func merchantStoryNodes(stats merchantstats.Stats, merchant, base string, thisMag int64, hasThis bool) []ui.Node {
	// The delta gets its own colour-coded treatment (a real figure, not a tag word), so
	// it never borrows the uppercase/letter-spaced `.badge` pill built for SPLIT/SMART.
	var deltaLine ui.Node = Fragment()
	if hasThis {
		delta := stats.DeltaVsTypical(thisMag)
		switch {
		case delta > 0:
			deltaLine = Span(css.Class("mtrend-delta is-up"), uistate.T("merchantPanel.aboveUsual", "+"+fmtMoney(money.New(delta, base))))
		case delta < 0:
			deltaLine = Span(css.Class("mtrend-delta is-down"), uistate.T("merchantPanel.belowUsual", "-"+fmtMoney(money.New(-delta, base))))
		default:
			deltaLine = Span(css.Class("mtrend-delta is-flat"), uistate.T("merchantPanel.atUsual"))
		}
	}

	// Each line is a block (Div), so the layout never depends on the container being a
	// flex column — the visit and month lines can't run together onto one line.
	var visitLine ui.Node = Fragment()
	if stats.VisitsThisWeek > 0 {
		freq := uistate.T("merchantPanel.visitThisWeek", ordinalDay(stats.VisitsThisWeek))
		if stats.VisitsThisMonth > 0 {
			freq += " · " + uistate.T("merchantPanel.visitsThisMonth", stats.VisitsThisMonth)
		}
		visitLine = Div(css.Class("muted", tw.Text12), freq)
	} else if stats.VisitsThisMonth > 0 {
		visitLine = Div(css.Class("muted", tw.Text12), uistate.T("merchantPanel.visitsThisMonth", stats.VisitsThisMonth))
	}

	var monthLine string
	switch {
	case stats.VisitsThisMonth == 0 && stats.TypicalMonth > 0:
		// No charge in the current calendar month yet. A bare "$0.00 this month" reads
		// as alarming for a recurring merchant that simply hasn't posted this month —
		// say so plainly, and anchor it to the typical monthly figure.
		monthLine = uistate.T("merchantPanel.noneThisMonth", fmtMoney(money.New(stats.TypicalMonth, base)))
	case stats.TypicalMonth > 0:
		monthLine = uistate.T("merchantPanel.monthVsTypical",
			fmtMoney(money.New(stats.SpentThisMonth, base)), fmtMoney(money.New(stats.TypicalMonth, base)))
	default:
		monthLine = uistate.T("merchantPanel.monthSpentOnly", fmtMoney(money.New(stats.SpentThisMonth, base)))
	}

	return []ui.Node{
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2),
			Strong(uistate.T("merchantPanel.title", merchant)),
			deltaLine,
		),
		visitLine,
		Div(css.Class("muted", tw.Text12), monthLine),
		merchantSparkline(stats.Last12, stats.Last12Dates, base),
	}
}

// merchantSparkline wraps the bare recent-charge polyline with the scale metadata a
// reader needs to make sense of it: a y-axis legend (the high + low charge), an x-axis
// time span (oldest → newest charge date), and a caption with the count + latest
// charge. Value text lives in HTML around the SVG (the polyline stretches with
// preserveAspectRatio="none", which would distort any text inside it).
func merchantSparkline(mags []int64, dates []time.Time, base string) ui.Node {
	if len(mags) < 2 || len(dates) != len(mags) {
		return sparklineSVG(mags, uistate.T("merchantPanel.sparklineAlt")) // fall back to the bare line if dates are unavailable
	}
	lo, hi := mags[0], mags[0]
	for _, v := range mags {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	latest := mags[len(mags)-1]
	oldest, newest := dates[0], dates[len(dates)-1]
	return Div(css.Class("mtrend-spark"),
		Div(css.Class("mtrend-spark-row"),
			sparklineSVG(mags, uistate.T("merchantPanel.sparklineAlt")),
			// y-axis legend: highest charge at the top, lowest at the bottom.
			Div(css.Class("mtrend-spark-yaxis"),
				Span(css.Class("mtrend-spark-hi"), fmtMoney(money.New(hi, base))),
				Span(css.Class("mtrend-spark-lo"), fmtMoney(money.New(lo, base))),
			),
		),
		// x-axis: the span the line covers (oldest → newest charge).
		Div(css.Class("mtrend-spark-xaxis"),
			Span(oldest.Format("Jan '06")),
			Span(newest.Format("Jan '06")),
		),
		Div(css.Class("mtrend-spark-meta"),
			uistate.T("merchantTrend.sparkMeta", len(mags), fmtMoney(money.New(latest, base)))),
	)
}

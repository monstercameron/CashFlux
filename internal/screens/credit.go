// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// buildCreditInputs derives the credithealth.Inputs from live app data.
// It computes current balances for every credit-card account via ledger.Balance
// (matching the same pattern used by buildHealthInputs in health.go).
func buildCreditInputs(app *appstate.App, now time.Time) credithealth.Inputs {
	accounts := app.Accounts()
	txns := app.Transactions()

	// Build the per-card balance map expected by credithealth.Evaluate.
	balances := make(map[string]int64, len(accounts))
	for _, a := range accounts {
		if a.Type != domain.TypeCreditCard || a.Archived {
			continue
		}
		// ledger.Balance computes the running balance from transactions.
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		balances[a.ID] = bal.Amount
	}

	return credithealth.Inputs{
		Accounts:     accounts,
		Balances:     balances,
		Transactions: txns,
		Now:          now,
	}
}

// creditBandTone maps a credithealth.Band to a semantic tone class.
func creditBandTone(b credithealth.Band) string {
	switch b {
	case credithealth.BandExcellent:
		return "text-up"
	case credithealth.BandGood:
		return "text-up"
	case credithealth.BandFair:
		return "text-warn"
	default:
		return "text-down"
	}
}

// creditUtilBandTone maps a per-card utilization band to a semantic tone class.
func creditUtilBandTone(b credithealth.UtilBand) string {
	switch b {
	case credithealth.BandUtilBest:
		return "text-up"
	case credithealth.BandUtilGood:
		return "text-up"
	case credithealth.BandUtilFair:
		return "text-warn"
	case credithealth.BandUtilPoor, credithealth.BandUtilWorst:
		return "text-down"
	default:
		return "text-dim"
	}
}

// creditUtilBarTone maps a per-card utilization band to a progress-bar tone.
func creditUtilBarTone(b credithealth.UtilBand) string {
	switch b {
	case credithealth.BandUtilBest, credithealth.BandUtilGood:
		return "bg-up"
	case credithealth.BandUtilFair:
		return "bg-warn"
	default:
		return "bg-down"
	}
}

// creditHue maps a 0–100 proxy score to a continuous red→green hue (HSL),
// mirroring the approach in healthHue.
func creditHue(score int) int { return score * 13 / 10 }

// creditProxyColor returns the ring/figure stroke for a proxy score.
func creditProxyColor(r credithealth.Result) string {
	return fmt.Sprintf("hsl(%d, 64%%, 52%%)", creditHue(r.ProxyScore))
}

// creditScoreRing renders the circular proxy-score gauge as an SVG, matching
// the design of healthRing in health.go. size is the outer pixel diameter.
func creditScoreRing(r credithealth.Result, size int) ui.Node {
	const radius = 52.0
	const circ = 2 * 3.141592653589793 * radius
	pct := float64(r.ProxyScore)
	offset := circ * (1 - pct/100)
	color := creditProxyColor(r)
	figure := fmt.Sprintf("%d", r.ProxyScore)
	px := fmt.Sprintf("%dpx", size)

	ring := Svg(
		Attr("viewBox", "0 0 120 120"),
		Attr("width", px), Attr("height", px),
		Attr("aria-hidden", "true"),
		// Faint full track.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", "var(--line, #2a2a2d)"), Attr("stroke-width", "10")),
		// Score arc — starts at 12 o'clock (rotate -90), rounded cap.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", color), Attr("stroke-width", "10"),
			Attr("stroke-linecap", "round"),
			Attr("stroke-dasharray", fmt.Sprintf("%.2f", circ)),
			Attr("stroke-dashoffset", fmt.Sprintf("%.2f", offset)),
			Attr("transform", "rotate(-90 60 60)"),
			Style(map[string]string{"transition": "stroke-dashoffset .9s cubic-bezier(.22,1,.36,1), stroke .6s ease"})),
	)

	overlay := Div(
		Style(map[string]string{
			"position": "absolute", "inset": "0",
			"display": "flex", "flex-direction": "column",
			"align-items": "center", "justify-content": "center",
		}),
		Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(creditBandTone(r.Band))),
			Style(map[string]string{"font-size": fmt.Sprintf("%dpx", size/3)}), figure),
		Div(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "2px"}), uistate.T("credit.outOf100")),
	)

	return Div(
		Style(map[string]string{"position": "relative", "width": px, "height": px, "flex": "0 0 " + px}),
		ring, overlay,
	)
}

// creditCardRow renders one credit card's utilization detail: name, balance,
// limit, utilization bar, and the actionable "pay $X to reach 30%" nudge.
// This is a plain (non-interactive) display row, so MapKeyed is safe to use.
func creditCardRow(cu credithealth.CardUtil, base, baseCur string) ui.Node {
	// Format balance: cards carry a negative balance (money owed); show owed amount.
	owed := cu.BalanceMinor
	if owed < 0 {
		owed = -owed
	}
	dec := currency.Decimals(baseCur)
	sym := currency.Symbol(baseCur)

	balStr := sym + fmtMinorAmount(owed, dec)
	limitStr := sym + fmtMinorAmount(cu.LimitMinor, dec)
	_ = base

	// Utilization label.
	var utilLabel string
	var utilPct int
	if cu.UtilPct < 0 {
		utilLabel = uistate.T("credit.noLimit")
		utilPct = 0
	} else {
		utilLabel = fmt.Sprintf("%d%%", cu.UtilPct)
		utilPct = cu.UtilPct
	}

	// Progress bar: cap at 100 for display.
	barPct := utilPct
	if barPct > 100 {
		barPct = 100
	}

	head := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
		Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), cu.Name),
		Span(ClassStr("t-body "+tw.ColorClass(creditUtilBandTone(cu.Band))), utilLabel),
	)

	bar := uiw.ProgressBar(uiw.ProgressBarProps{
		Percent: barPct,
		Tone:    creditUtilBarTone(cu.Band),
	})

	meta := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mt1),
		Span(css.Class("t-caption", tw.TextFaint),
			fmt.Sprintf(uistate.T("credit.balanceOfLimit"), balStr, limitStr)),
		Span(ClassStr("t-caption "+tw.ColorClass(creditUtilBandTone(cu.Band))),
			string(cu.Band)),
	)

	// Actionable nudge: pay $X to reach 30% utilization.
	var nudge ui.Node = Fragment()
	if cu.Target30Minor > 0 {
		payStr := sym + fmtMinorAmount(cu.Target30Minor, dec)
		nudge = Div(css.Class("t-caption", tw.TextDim, tw.Mt1),
			uistate.T("credit.payToReach30Prefix"),
			Span(ClassStr("t-caption "+tw.Fold(tw.FontMedium)), payStr),
			uistate.T("credit.payToReach30Suffix"),
		)
	}

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), head, bar, meta, nudge)
}

// fmtMinorAmount formats minor-unit integer cents into a decimal display string
// without a symbol (the symbol is prepended by callers).
func fmtMinorAmount(minor int64, decimals int) string {
	if decimals == 0 {
		return fmt.Sprintf("%d", minor)
	}
	div := int64(1)
	for i := 0; i < decimals; i++ {
		div *= 10
	}
	whole := minor / div
	frac := minor % div
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%0*d", whole, decimals, frac)
}

// CreditScreen is the full /credit page: an overall credit-health proxy score
// ring (C208 — local, privacy-friendly, no bureau), then a per-card
// utilization breakdown with actionable "pay $X to reach 30%" nudges (C209).
// C210 (utilization history/trend) is out of scope — that needs stored
// snapshots which don't exist yet.
func CreditScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	in := buildCreditInputs(app, now)
	r := credithealth.Evaluate(in)

	// Empty state: no credit cards at all.
	if len(r.Cards) == 0 {
		empty := uiw.Card(uiw.CardProps{
			Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("credit.emptyTitle")),
				P(css.Class("t-caption", tw.TextDim), uistate.T("credit.emptyBody")),
			),
		})
		disclaimer := P(css.Class("t-caption", tw.TextFaint), r.Disclaimer)
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5), empty, disclaimer)
	}

	// Hero: ring + band + aggregate utilization.
	aggLabel := fmt.Sprintf("%d%%", r.Agg.UtilPct)
	if r.Agg.UtilPct < 0 {
		aggLabel = uistate.T("credit.noLimit")
	}

	hero := uiw.Card(uiw.CardProps{Body: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5),
		creditScoreRing(r, 150),
		Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
			Div(ClassStr("t-figure-lg "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(creditBandTone(r.Band))),
				string(r.Band)),
			Div(css.Class("t-caption", tw.TextDim, tw.Mt1),
				fmt.Sprintf(uistate.T("credit.aggUtil"), aggLabel)),
		),
	)})

	// Per-card breakdown.
	cardRows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap4)}
	for _, cu := range r.Cards {
		cardRows = append(cardRows, creditCardRow(cu, base, base))
	}
	breakdown := uiw.Card(uiw.CardProps{
		Title: uistate.T("credit.breakdownTitle"),
		Body:  Div(cardRows...),
	})

	// Missing-limit note (if any cards lack a limit).
	var missingNote ui.Node = Fragment()
	if r.Agg.CardsMissingLimit > 0 {
		missingNote = uiw.Card(uiw.CardProps{
			Body: P(css.Class("t-caption", tw.TextDim),
				fmt.Sprintf(uistate.T("credit.missingLimitNote"), r.Agg.CardsMissingLimit)),
		})
	}

	// Privacy + disclaimer note.
	disclaimer := P(css.Class("t-caption", tw.TextFaint), r.Disclaimer)

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		hero, breakdown, missingNote, disclaimer)
}

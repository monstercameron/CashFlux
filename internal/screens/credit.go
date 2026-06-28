// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// utilTrendPoint holds one dated utilization reading derived from a balance snapshot.
type utilTrendPoint struct {
	date string // short formatted date e.g. "Jan 2"
	pct  int    // 0–100+ utilization percent
	id   string // snapshot ID for keying
}

// buildUtilTrend fetches balance history for a card and converts it to a series
// of utilization points. Only snapshots where the limit is known (limitMinor > 0)
// produce a point. Returns nil when there are fewer than 2 points.
func buildUtilTrend(app *appstate.App, accountID string, limitMinor int64) []utilTrendPoint {
	if limitMinor <= 0 {
		return nil
	}
	snaps := app.BalanceHistory(accountID)
	if len(snaps) < 2 {
		return nil
	}
	// Cap to last 8 snapshots.
	start := 0
	if len(snaps) > 8 {
		start = len(snaps) - 8
	}
	recent := snaps[start:]

	pts := make([]utilTrendPoint, 0, len(recent))
	for _, s := range recent {
		bal := s.BalanceMinor
		if bal < 0 {
			bal = -bal
		}
		pct := int(bal * 100 / limitMinor)
		pts = append(pts, utilTrendPoint{
			date: s.AsOf.Format("Jan 2"),
			pct:  pct,
			id:   s.ID,
		})
	}
	return pts
}

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

// creditScoreRing renders the circular proxy-score gauge as an SVG, delegating
// the shared geometry to scoreRingNode (FEATURE_MAP §5.7c). This wrapper is
// responsible only for deriving the credit-specific color, figure text, and
// R52/R64 aria label.
func creditScoreRing(r credithealth.Result, size int) ui.Node {
	pct := float64(r.ProxyScore)
	color := creditProxyColor(r)
	figure := fmt.Sprintf("%d", r.ProxyScore)
	// R52/R64 a11y: label the ring (role=img + one-sentence name) rather than
	// hiding it; the overlay number below is aria-hidden so the score isn't read
	// twice. Mirrors the healthRing fix.
	ringLabel := uistate.T("credit.ringLabel", r.ProxyScore, string(r.Band))
	centerLabel := Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(creditBandTone(r.Band))),
		Style(map[string]string{"font-size": fmt.Sprintf("%dpx", size/3)}), figure)
	subLabel := Div(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "2px"}), uistate.T("credit.outOf100"))
	return scoreRingNode(pct, color, size, ringLabel, centerLabel, subLabel)
}

// creditTrendBarTone returns the bar tone for a utilization pct.
func creditTrendBarTone(pct int) string {
	switch {
	case pct <= 10:
		return "bg-up"
	case pct <= 30:
		return "bg-up"
	case pct <= 50:
		return "bg-warn"
	default:
		return "bg-down"
	}
}

// creditCardRow renders one credit card's utilization detail: name, balance,
// limit, utilization bar, the actionable "pay $X to reach 30%" nudge, and
// (when ≥2 balance snapshots exist) a compact chronological utilization trend.
// This is a plain (non-interactive) display row, so MapKeyed is safe to use.
func creditCardRow(cu credithealth.CardUtil, base, baseCur, acctCur string, onSaveLimit func(string, int64), trend []utilTrendPoint) ui.Node {
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

	// Utilization trend (C210): shown only when ≥2 balance snapshots exist.
	var trendPanel ui.Node = Fragment()
	if len(trend) >= 2 {
		keyOfPt := func(p utilTrendPoint) any { return p.id }
		renderPt := func(p utilTrendPoint) ui.Node {
			pct := p.pct
			if pct > 100 {
				pct = 100
			}
			barWidth := fmt.Sprintf("%d%%", pct)
			tone := creditTrendBarTone(p.pct)
			return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("t-caption", tw.TextFaint, "util-trend-date"),
					Style(map[string]string{"min-width": "4.5rem"}), p.date),
				Div(css.Class("util-trend-track"),
					Style(map[string]string{
						"flex": "1", "background": "var(--line, #2a2a2d)",
						"border-radius": "2px", "height": "6px", "overflow": "hidden",
					}),
					Div(css.Class(tone, "util-trend-bar"),
						Style(map[string]string{
							"width": barWidth, "height": "100%",
							"border-radius": "2px",
						})),
				),
				Span(css.Class("t-caption", tw.TextFaint),
					Style(map[string]string{"min-width": "2.5rem", "text-align": "right"}),
					fmt.Sprintf("%d%%", p.pct)),
			)
		}
		trendPanel = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Style(map[string]string{"margin-top": "0.5rem"}),
			P(css.Class("t-caption", tw.TextDim), uistate.T("credit.trendTitle")),
			MapKeyed(trend, keyOfPt, renderPt),
		)
	} else if len(trend) == 0 && cu.LimitMinor > 0 {
		// Card has a limit but no snapshots yet — show a muted nudge.
		trendPanel = P(css.Class("t-caption", tw.TextFaint),
			Style(map[string]string{"margin-top": "0.5rem"}),
			uistate.T("credit.trendNoHistory"))
	}

	// C211: inline credit-limit editor so the limit (which drives utilization) can
	// be corrected right here, instead of only in the account edit form. Its own
	// component so its input hook sits at a stable position (rows render in a loop).
	limitEditor := ui.CreateElement(creditLimitEditor, creditLimitEditorProps{
		AccountID: cu.AccountID, Currency: acctCur, LimitMinor: cu.LimitMinor, OnSave: onSaveLimit,
	})

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), head, bar, meta, nudge, trendPanel, limitEditor)
}

// creditLimitEditorProps drives the inline credit-limit editor (C211).
type creditLimitEditorProps struct {
	AccountID  string
	Currency   string
	LimitMinor int64
	OnSave     func(accountID string, limitMinor int64)
}

// creditLimitEditor is a small inline editor for a card's credit limit. It is its
// own component so its UseState/UseEvent hooks sit at a stable render position —
// credit-card rows render in a variable-length loop (the framework gotcha).
func creditLimitEditor(props creditLimitEditorProps) ui.Node {
	dec := currency.Decimals(props.Currency)
	init := ""
	if props.LimitMinor > 0 {
		init = money.FormatMinor(props.LimitMinor, dec)
	}
	val := ui.UseState(init)
	saved := ui.UseState(false)
	onInput := ui.UseEvent(func(v string) { val.Set(v); saved.Set(false) })
	commit := func() {
		m, err := money.ParseMinor(strings.TrimSpace(val.Get()), dec)
		if err == nil && m >= 0 && props.OnSave != nil {
			props.OnSave(props.AccountID, m)
			saved.Set(true)
		}
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
		Span(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"min-width": "5rem"}), uistate.T("credit.limitLabel")),
		Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("0.01"),
			Attr("data-testid", "credit-limit-edit"),
			Attr("aria-label", uistate.T("credit.limitEditAria", props.AccountID)),
			Placeholder(uistate.T("credit.limitPlaceholder")),
			Value(val.Get()), OnInput(onInput), OnBlur(func() { commit() })),
		If(saved.Get(), Span(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("role", "status"), uistate.T("credit.limitSaved"))),
	)
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
// utilization breakdown with actionable "pay $X to reach 30%" nudges (C209),
// and per-card utilization trend bars derived from balance snapshots (C210).
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
	accByID := map[string]domain.Account{}
	for _, a := range app.Accounts() {
		accByID[a.ID] = a
	}
	// C211: persist an edited credit limit (in the account's own currency), then the
	// data-revision bump re-renders utilization with the new denominator.
	saveLimit := func(id string, limitMinor int64) {
		acc, ok := accByID[id]
		if !ok {
			return
		}
		acc.CreditLimit = money.New(limitMinor, acc.Currency)
		if err := app.PutAccount(acc); err != nil {
			app.Log().Error("credit-limit save failed", "account", id, "err", err)
			return
		}
		// PutAccount is the logic layer and doesn't touch the UI; bump the shared
		// data revision so the screen re-renders and utilization recomputes.
		uistate.BumpDataRevision()
	}
	cardRows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap4)}
	for _, cu := range r.Cards {
		trend := buildUtilTrend(app, cu.AccountID, cu.LimitMinor)
		acctCur := base
		if a, ok := accByID[cu.AccountID]; ok && a.Currency != "" {
			acctCur = a.Currency
		}
		cardRows = append(cardRows, creditCardRow(cu, base, base, acctCur, saveLimit, trend))
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

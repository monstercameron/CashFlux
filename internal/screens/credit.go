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
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
// (matching the same pattern used by liveHealthInputs in health.go).
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
	// A card's balance and limit are in its OWN currency — format with the account's
	// symbol/decimals, not the household base (a EUR card must read €, not $). The
	// utilization percent is currency-independent, so no conversion is needed here.
	dec := currency.Decimals(acctCur)
	sym := currency.Symbol(acctCur)

	balStr := sym + fmtMinorAmount(owed, dec)
	limitStr := sym + fmtMinorAmount(cu.LimitMinor, dec)
	_ = base
	_ = baseCur

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

	// The severity band sits BESIDE the percentage it describes (review: an
	// orphaned chip three lines below forced a zigzag to reconnect them).
	head := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
		Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), cu.Name),
		Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(ClassStr("t-body "+tw.ColorClass(creditUtilBandTone(cu.Band))), utilLabel),
			Span(ClassStr("t-caption "+tw.ColorClass(creditUtilBandTone(cu.Band))), string(cu.Band)),
		),
	)

	bar := uiw.ProgressBar(uiw.ProgressBarProps{
		Percent: barPct,
		Tone:    creditUtilBarTone(cu.Band),
	})

	meta := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mt1),
		Span(css.Class("t-caption", tw.TextFaint),
			fmt.Sprintf(uistate.T("credit.balanceOfLimit"), balStr, limitStr)),
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
						"flex": "1", "background": "var(--border)",
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
	// Rare-use maintenance UI folds behind a disclosure (review: a permanently
	// open full-width input carried the same weight as the read-only data). The
	// c211 flow is intact — open the disclosure, edit, blur.
	limitEditor := Details(css.Class("hlt-curve"),
		Summary(uistate.T("credit.editLimit")),
		ui.CreateElement(creditLimitEditor, creditLimitEditorProps{
			AccountID: cu.AccountID, Currency: acctCur, LimitMinor: cu.LimitMinor, OnSave: onSaveLimit,
		}),
	)

	return Div(css.Class("credit-card-item "+tw.Fold(tw.Flex, tw.FlexCol, tw.Gap1)), head, bar, meta, nudge, trendPanel, limitEditor)
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
			Attr("data-testid", "credit-limit-edit-"+props.AccountID),
			Attr("aria-label", uistate.T("credit.limitEditAria", props.AccountID)),
			Placeholder(uistate.T("credit.limitPlaceholder")),
			Value(val.Get()), OnInput(onInput), OnBlur(func() { commit() })),
		If(saved.Get(), Span(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("role", "status"), uistate.T("credit.limitSaved"))),
	)
}

// fmtMinorAmount formats minor-unit integer cents into a comma-grouped decimal
// display string without a symbol (the symbol is prepended by callers) — the
// same grouping every other money figure gets via money.FormatAccounting, so
// /credit, /investments, /loans, and /duplicates figures no longer render
// "$33720.00" beside a "$4,590.56" from the shared formatter (C337).
func fmtMinorAmount(minor int64, decimals int) string {
	if decimals == 0 {
		return money.Group(fmt.Sprintf("%d", minor))
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
	return money.Group(fmt.Sprintf("%d.%0*d", whole, decimals, frac))
}

// CreditHealthPanelProps configures CreditHealthPanel. No external props are
// required; the panel reads appstate.Default directly.
type CreditHealthPanelProps struct{}

// CreditHealthPanel renders the credit-health proxy score ring (C208), per-card
// utilization breakdown with actionable nudges (C209), utilization trend bars
// (C210), and inline credit-limit editors (C211) as a registered component.
// It owns its UseDataRevision hook so it can be embedded at two call sites
// (/credit and /debt) without duplicating state or violating GWC hook rules.
func CreditHealthPanel(props CreditHealthPanelProps) ui.Node {
	// Hook declared unconditionally before any conditional return (GWC rule).
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

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
			// Name the figure as a CashFlux estimate right at the ring — this embedded
			// panel carries its full disclaimer only at the very bottom, so the score
			// otherwise reads as a bureau number until the reader scrolls past it.
			Div(css.Class("t-caption", tw.TextFaint, tw.Mt1), Attr("data-testid", "credit-estimate-note"),
				uistate.T("detail6.creditScoreLabel")),
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

	// Demerits — the factors dragging the score down, most costly first (C-credit demerits).
	var demeritsCard ui.Node = Fragment()
	if len(r.Demerits) > 0 {
		items := []any{css.Class("credit-list")}
		for _, d := range r.Demerits {
			var chip ui.Node = Fragment()
			if d.PointsLost > 0 {
				chip = Span(css.Class("credit-pts credit-pts-down"), uistate.T("credit.ptsDown", d.PointsLost))
			}
			items = append(items, Div(css.Class("credit-item"),
				Span(css.Class("credit-item-icon is-down"), Attr("aria-hidden", "true"),
					uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				Span(css.Class("credit-item-text"), creditDemeritText(d)),
				chip,
			))
		}
		demeritsCard = uiw.Card(uiw.CardProps{Title: uistate.T("credit.demeritsTitle"),
			TestID: "credit-demerits", Body: Div(items...)})
	}

	// Advice — the clearest, prioritized actions to raise the score, biggest impact first.
	var adviceCard ui.Node = Fragment()
	if len(r.Advice) > 0 {
		items := []any{css.Class("credit-list")}
		for _, a := range r.Advice {
			var chip ui.Node = Fragment()
			if a.ImpactPts > 0 {
				chip = Span(css.Class("credit-pts credit-pts-up"), uistate.T("credit.ptsUp", a.ImpactPts))
			}
			items = append(items, Div(css.Class("credit-item"),
				Span(css.Class("credit-item-icon is-up"), Attr("aria-hidden", "true"),
					uiw.Icon(icon.ArrowUpCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				Span(css.Class("credit-item-text"), creditAdviceText(a, base)),
				chip,
			))
		}
		adviceCard = uiw.Card(uiw.CardProps{Title: uistate.T("credit.improveTitle"),
			TestID: "credit-advice", Body: Div(items...)})
	}

	// Optional Smart+ AI analysis (SMART-A11) — a deeper, personalized read, shown only when
	// the feature is enabled and an inference provider is configured (see creditAINode).
	aiCard := creditAINode()

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
		hero, demeritsCard, adviceCard, aiCard, breakdown, missingNote, disclaimer)
}

// creditDemeritText renders one demerit as a plain-English sentence (i18n).
func creditDemeritText(d credithealth.Demerit) string {
	switch d.Kind {
	case credithealth.DemeritUtilization:
		return uistate.T("credit.demeritUtil", d.Pct)
	case credithealth.DemeritCardUtil:
		return uistate.T("credit.demeritCardUtil", d.Name, d.Pct)
	case credithealth.DemeritOnTime:
		return uistate.T("credit.demeritOnTime")
	case credithealth.DemeritAge:
		return uistate.T("credit.demeritAge")
	case credithealth.DemeritOverLimit:
		return uistate.T("credit.demeritOverLimit", d.Name, d.Pct)
	case credithealth.DemeritMissingLimit:
		return uistate.T("credit.demeritMissingLimit", d.Pct)
	}
	return ""
}

// creditAdviceText renders one advice item as an imperative sentence (i18n), formatting any
// suggested payment in the base currency.
func creditAdviceText(a credithealth.Advice, base string) string {
	switch a.Kind {
	case credithealth.AdviceLowerUtilization:
		return uistate.T("credit.adviceLowerUtil", fmtMoney(money.New(a.PayMinor, base)), a.Name, a.TargetPct)
	case credithealth.AdviceOnTime:
		return uistate.T("credit.adviceOnTime")
	case credithealth.AdviceAddLimit:
		return uistate.T("credit.adviceAddLimit", a.Name)
	}
	return ""
}

// creditContextString builds the compact, hook-free snapshot the SMART-A11 AI analysis is
// grounded in: the proxy score/band, per-card utilization, the demerits, and the suggested
// actions — the same deterministic figures shown on screen, so the model only personalizes
// and prioritizes them (never invents numbers).
func creditContextString(app *appstate.App) string {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	r := credithealth.Evaluate(buildCreditInputs(app, time.Now()))
	var b strings.Builder
	fmt.Fprintf(&b, "Overall score: %d/100 (%s)\n", r.ProxyScore, r.Band)
	if r.Agg.UtilPct >= 0 {
		fmt.Fprintf(&b, "Overall utilization: %d%%\n", r.Agg.UtilPct)
	}
	if len(r.Cards) > 0 {
		b.WriteString("Cards:\n")
		for _, c := range r.Cards {
			if c.UtilPct >= 0 {
				fmt.Fprintf(&b, "- %s: %d%% utilization\n", c.Name, c.UtilPct)
			} else {
				fmt.Fprintf(&b, "- %s: no limit set\n", c.Name)
			}
		}
	}
	if len(r.Demerits) > 0 {
		b.WriteString("Dragging it down:\n")
		for _, d := range r.Demerits {
			fmt.Fprintf(&b, "- %s\n", creditDemeritText(d))
		}
	}
	if len(r.Advice) > 0 {
		b.WriteString("Suggested actions:\n")
		for _, a := range r.Advice {
			fmt.Fprintf(&b, "- %s\n", creditAdviceText(a, base))
		}
	}
	return b.String()
}

// creditAINode renders the opt-in Smart+ AI credit analysis (SMART-A11): a deeper,
// personalized read of the demerits + advice. It appears only when the feature is enabled
// and an inference provider is configured; enabled-without-provider shows a quiet hint, and
// off shows nothing (opt-in, never a dead control). UsePrefs is called unconditionally so
// hook order is stable if the feature is toggled.
func creditAINode() ui.Node {
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	const code = "SMART-A11"
	settings := uistate.LoadSmartSettings()
	if !settings.IsEnabled(code) || settings.IsMuted(code) {
		return Fragment()
	}
	backendAI := pr.Normalize().BackendActive()
	if !aiProviderConfigured(app, backendAI) {
		return uiw.Card(uiw.CardProps{Title: uistate.T("credit.aiTitle"), TestID: "credit-ai",
			Body: P(css.Class("t-caption", tw.TextDim), uistate.T("smart.aiNeedsProvider"))})
	}
	f, ok := smart.ByCode(code)
	if !ok {
		return Fragment()
	}
	conn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	return uiw.Card(uiw.CardProps{Title: uistate.T("credit.aiTitle"), TestID: "credit-ai",
		Body: smartAIFeatureNode(f, conn)})
}

// CreditScreen is the /credit route — a thin shell rendering CreditHealthPanel.
// The panel owns all hooks and state so it can also be embedded in /debt.
// creditFactorTileProps drives one proxy-factor tile (on-time / age): the
// score, its exact normalized weight, and the copy keys. Its own component so
// any future interaction hooks sit at stable positions.
type creditFactorTileProps struct {
	Key     string // "ontime" | "age"
	Title   string
	Score   int // -1 = not enough data
	Weight  float64
	VarName string
}

// creditFactorTile renders one supporting proxy factor in the /health tile
// style: the meter as the only score visual, a why-paragraph, and the exact
// score/weight/variable identity folded behind "How it's scored".
func creditFactorTile(p creditFactorTileProps) ui.Node {
	if p.Score < 0 {
		return hltSection("sec-cf-"+p.Key, p.Title, nil, Fragment(
			P(css.Class("empty"), Attr("data-testid", "cf-na-"+p.Key), uistate.T("credit.na."+p.Key)),
			P(css.Class("muted"), uistate.T("credit.f."+p.Key+".why")),
		))
	}
	meterPct := p.Score
	if meterPct == 0 {
		meterPct = 2 // a zero still renders a visible sliver (scored, not broken)
	}
	var statusLine ui.Node
	if p.Score >= 90 {
		statusLine = Span(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "cf-met-"+p.Key),
			uistate.T("credit.f."+p.Key+".good"))
	} else {
		statusLine = Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cf-unmet-"+p.Key),
			uistate.T("credit.f."+p.Key+".room"))
	}
	return hltSection("sec-cf-"+p.Key, p.Title, nil, Fragment(
		Div(css.Class("hlt-factor-head"),
			// The unit rides with the value (review: a bare "57" on the age tile
			// could read as months; the hero's own score says "out of 100").
			Span(
				Span(ClassStr("hlt-factor-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(healthBandForScore(p.Score)))), fmt.Sprintf("%d", p.Score)),
				Span(css.Class("t-caption", tw.TextFaint), " "+uistate.T("credit.outOf100")),
			),
			statusLine,
		),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: meterPct, Tone: healthBarTone(p.Score)}),
		P(css.Class("muted", tw.Mt2), uistate.T("credit.f."+p.Key+".why")),
		Details(css.Class("hlt-curve"),
			Summary(uistate.T("health.curveSummary")),
			P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.f."+p.Key+".curve")),
			P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", p.Score, int(p.Weight*100+0.5))),
			Div(css.Class("hlt-varchip"), Attr("data-testid", "cf-var-"+p.Key),
				Title(uistate.T("health.varChipTitle")),
				Code(p.VarName), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", p.Score)),
			),
		),
	))
}

// creditListItems renders the demerit/advice item rows (icons + point chips)
// shared with the embedded panel's cards.
func creditListItems(r credithealth.Result, base string, up bool) []any {
	items := []any{css.Class("credit-list")}
	if up {
		for _, a := range r.Advice {
			var chip ui.Node = Fragment()
			if a.ImpactPts > 0 {
				// Show the destination, not just the delta (review: "+24 pts" carries
				// no scale context; "→ 79" makes the payoff concrete).
				projected := r.ProxyScore + a.ImpactPts
				if projected > 100 {
					projected = 100
				}
				chip = Span(css.Class("credit-pts credit-pts-up"), uistate.T("credit.ptsUpTo", a.ImpactPts, projected))
			}
			items = append(items, Div(css.Class("credit-item"),
				Span(css.Class("credit-item-icon is-up"), Attr("aria-hidden", "true"),
					uiw.Icon(icon.ArrowUpCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				Span(css.Class("credit-item-text"), creditAdviceText(a, base)),
				chip,
			))
		}
		return items
	}
	for _, d := range r.Demerits {
		var chip ui.Node = Fragment()
		if d.PointsLost > 0 {
			chip = Span(css.Class("credit-pts credit-pts-down"), uistate.T("credit.ptsDown", d.PointsLost))
		}
		items = append(items, Div(css.Class("credit-item"),
			Span(css.Class("credit-item-icon is-down"), Attr("aria-hidden", "true"),
				uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("credit-item-text"), creditDemeritText(d)),
			chip,
		))
	}
	return items
}

// CreditScreen is the full /credit page, a widgetized bento surface where the
// proxy score IS a formula: a hero tile (score ring + band + the folded
// credit_proxy molecule — auditable, referenceable, re-weightable under
// Formulas), a full-width utilization tile (the dominant 55% factor: the
// aggregate meter with a met/unmet target line plus every card's detail rows
// with limit editing and pay-down targets), the on-time and account-age
// factor tiles, the what's-dragging-it-down / how-to-raise-it pair, the
// optional Smart+ AI read, and an opt-in FormulaBuilder seeded with
// credit_proxy. The embedded summary panel (CreditHealthPanel, used on /debt)
// is unchanged.
func CreditScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))

	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	in := buildCreditInputs(app, now)
	r := credithealth.Evaluate(in)

	if len(r.Cards) == 0 {
		return Fragment(
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{
				Message:   uistate.T("credit.emptyBody"),
				CTALabel:  uistate.T("accounts.addFirst"),
				AddTarget: "account",
			}),
			P(css.Class("t-caption", tw.TextFaint), r.Disclaimer),
		)
	}

	// The proxy's formula identity (the persisted molecule — a user edit under
	// Formulas travels here too).
	scoreFormula := ""
	for _, m := range app.Molecules() {
		if m.Name == "credit_proxy" {
			scoreFormula = m.Formula
			break
		}
	}

	// ── Hero: ring + band + disclaimer + the folded formula. ────────────────────
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("credit.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("credit.metricsHide")
	}
	metricsBtn := Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
		Attr("data-testid", "credit-toggle-formulas"), Title(uistate.T("credit.metricsTitle")),
		OnClick(toggleFormulas), Text(metricsLabel))
	aggLabel := fmt.Sprintf("%d%%", r.Agg.UtilPct)
	if r.Agg.UtilPct < 0 {
		aggLabel = uistate.T("credit.noLimit")
	}
	// The hero carries ONLY the score story (review: "Overall utilization: 58%"
	// beside "55 / Good" read as the same metric restated — utilization lives on
	// its own tile below). The not-a-FICO line gets chip weight, not filler weight.
	hero := hltTile("crd-hero", "1 / span 4", hltSection("sec-credit-hero", uistate.T("credit.pageTitle"), metricsBtn,
		Fragment(
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5, tw.FlexWrap),
				creditScoreRing(r, 150),
				Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter),
					Div(ClassStr("t-figure-lg "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(creditBandTone(r.Band))), string(r.Band)),
					Div(css.Class("crd-disclaimer", tw.Mt2), Attr("data-testid", "credit-disclaimer"),
						uiw.Icon(icon.HelpCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
						Span(r.Disclaimer)),
				),
			),
			If(scoreFormula != "", Details(css.Class("hlt-formula"), Attr("data-testid", "credit-formula"),
				Summary(css.Class("t-caption", tw.TextDim), uistate.T("health.formulaTitle")),
				Code(css.Class("hlt-formula-code"), "credit_proxy = "+scoreFormula),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.formulaNote")),
			)),
		)))

	// ── Utilization: the dominant factor, with every card's detail. ─────────────
	accByID := map[string]domain.Account{}
	for _, a := range app.Accounts() {
		accByID[a.ID] = a
	}
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
		uistate.BumpDataRevision()
	}
	cardRows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap4, tw.Mt3)}
	for _, cu := range r.Cards {
		trend := buildUtilTrend(app, cu.AccountID, cu.LimitMinor)
		acctCur := base
		if a, ok := accByID[cu.AccountID]; ok && a.Currency != "" {
			acctCur = a.Currency
		}
		cardRows = append(cardRows, creditCardRow(cu, base, base, acctCur, saveLimit, trend))
	}
	utilScoreVal := credithealth.UtilScore(r.Agg.UtilPct)
	utilMeter := r.Agg.UtilPct
	if utilMeter < 0 {
		utilMeter = 0
	}
	if utilMeter > 100 {
		utilMeter = 100
	}
	var utilStatus ui.Node
	if r.Agg.UtilPct >= 0 && r.Agg.UtilPct < 30 {
		utilStatus = Span(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "cf-met-util"),
			uistate.T("health.onTarget", uistate.T("credit.utilTarget")))
	} else {
		utilStatus = Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cf-unmet-util"),
			uistate.T("health.target", uistate.T("credit.utilTarget")))
	}
	utilTile := hltTile("crd-util", "1 / span 4", hltSection("sec-cf-util", uistate.T("credit.utilTitle"), nil,
		Fragment(
			Div(css.Class("hlt-factor-head"),
				Span(ClassStr("hlt-factor-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(creditUtilBandTone(r.Agg.Band))), aggLabel),
				utilStatus,
			),
			// The meter shows the utilization VALUE (how much of the limit is in
			// use), matching the headline — with the 30% target rendered ON the
			// bar as a tick so value-vs-target is visual, not just prose.
			Div(css.Class("crd-meter-wrap"),
				uiw.ProgressBar(uiw.ProgressBarProps{Percent: utilMeter, Tone: creditTrendBarTone(r.Agg.UtilPct)}),
				Div(css.Class("crd-target-tick"), Attr("title", uistate.T("credit.utilTarget")), Attr("aria-hidden", "true")),
			),
			P(css.Class("muted", tw.Mt2), uistate.T("credit.f.util.why")),
			Details(css.Class("hlt-curve"),
				Summary(uistate.T("health.curveSummary")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.f.util.curve")),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", utilScoreVal, int(r.Weights.Util*100+0.5))),
				Div(css.Class("hlt-varchip"), Attr("data-testid", "cf-var-util"),
					Title(uistate.T("health.varChipTitle")),
					Code("credit_util_score"), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", utilScoreVal)),
				),
			),
			Div(cardRows...),
			If(r.Agg.CardsMissingLimit > 0, P(css.Class("muted", tw.Mt2),
				fmt.Sprintf(uistate.T("credit.missingLimitNote"), r.Agg.CardsMissingLimit))),
		)))

	// ── Supporting factors + the down/up pair. ───────────────────────────────────
	onTimeTile := hltTile("crd-ontime", "span 2", ui.CreateElement(creditFactorTile, creditFactorTileProps{
		Key: "ontime", Title: uistate.T("credit.ontimeTitle"), Score: r.OnTimeScore, Weight: r.Weights.OnTime, VarName: "credit_ontime_score"}))
	ageTile := hltTile("crd-age", "span 2", ui.CreateElement(creditFactorTile, creditFactorTileProps{
		Key: "age", Title: uistate.T("credit.ageTitle"), Score: r.AgeScore, Weight: r.Weights.Age, VarName: "credit_age_score"}))

	var downBody ui.Node = P(css.Class("empty"), uistate.T("credit.demeritsEmpty"))
	if len(r.Demerits) > 0 {
		downBody = Div(creditListItems(r, base, false)...)
	}
	var upBody ui.Node = P(css.Class("empty"), uistate.T("credit.adviceEmpty"))
	if len(r.Advice) > 0 {
		upBody = Div(creditListItems(r, base, true)...)
	}
	downTile := hltTile("crd-down", "span 2", hltSection("sec-credit-down", uistate.T("credit.demeritsTitle"), nil, downBody))
	upTile := hltTile("crd-up", "span 2", hltSection("sec-credit-up", uistate.T("credit.improveTitle"), nil, upBody))

	tiles := []ui.Node{hero, utilTile, onTimeTile, ageTile, downTile, upTile}

	// Optional Smart+ AI read renders its own card chrome → a bare grid child.
	tiles = append(tiles, Div(Style(map[string]string{"grid-column": "1 / span 4"}), creditAINode()))

	if showFormulas.Get() {
		tiles = append(tiles, hltTile("crd-formula", "1 / span 4", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("credit.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("credit.metricsShow"), Initial: "credit_proxy", ShowSaved: true}),
		)))
	}
	return Div(css.Class("bento bento-credit"), tiles)
}

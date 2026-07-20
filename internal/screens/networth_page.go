// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// The /networth surface.
//
// A from-scratch balance-sheet page (vocabulary `nws-`, styles in
// internal/styles/rules_networth.go), deliberately NOT the bento tile kit. It
// ships two readings of ONE computation, mirroring the Reports
// Summary | Full report pattern:
//
//	GLANCE  — the hero figure, THE BRIDGE (what actually moved net worth, with
//	          the residual shown), TWO SIDES (composition legible inside the
//	          trend), one plain-English takeaway, and the ratios WITH their
//	          interpretation. Everything the page is for, in one screen.
//	DETAIL  — the full balance sheet as a numbered document with a chip index,
//	          enumerating what Glance summarizes.
//
// Both views read the same nwsView, which is the mechanical reason no figure
// can disagree between them.

// nwsWindowLabel names a window length the way a person would say it.
func nwsWindowLabel(months int) string {
	switch months {
	case nwsAllTime:
		return uistate.T("nws.winAll")
	case 1:
		return uistate.T("nws.winMonth")
	case 24:
		return uistate.T("nw.horizon24")
	}
	return uistate.T("nws.winMonths", months)
}

// nwsWindowSpan phrases the window as a PHRASE that fits inside a sentence,
// which is a different job from naming it on a button.
//
// The period control is captioned "This month" / "6 months" / "All time",
// because that is how you label a choice. Dropping those captions into a
// sentence slot produced "$4,678.21 over This month" and "Up $169,788.68 over
// All time" — the same defect already fixed once on this page, where a window
// LENGTH was interpolated into a point-in-time slot ("6 months ago"). A label
// is not a phrase, and no generic template makes it one, so each period gets
// its own wording and carries its own preposition.
func nwsWindowSpan(months int) string {
	switch months {
	case nwsAllTime:
		return uistate.T("nws.spanAll")
	case 1:
		return uistate.T("nws.spanMonth")
	case 24:
		return uistate.T("nws.spanYears")
	}
	return uistate.T("nws.spanMonths", months)
}

// nwsWindowAgo phrases the window's far edge as a point in time, so a sentence
// can say "where you stood 6 months ago" rather than "where you stood 6 months".
func nwsWindowAgo(months int) string {
	switch months {
	case nwsAllTime:
		return uistate.T("nws.agoAll")
	case 1:
		return uistate.T("nws.agoMonth")
	case 24:
		return uistate.T("nws.agoYears")
	}
	return uistate.T("nws.agoMonths", months)
}

// nwsPctText prints a ratio, or says plainly that it cannot be computed rather
// than printing a fake 0%.
func nwsPctText(r balancesheet.Ratio) string {
	if !r.OK {
		return uistate.T("nws.ratioUnknown")
	}
	return fmt.Sprintf("%d%%", r.Pct)
}

// nwsRunwayText prints months of expenses to one decimal, from the engine's
// integer tenths.
func nwsRunwayText(h balancesheet.Health) string {
	if !h.RunwayOK {
		return uistate.T("nws.ratioUnknown")
	}
	return fmt.Sprintf("%d.%d", h.RunwayTenths/10, h.RunwayTenths%10)
}

// nwsBandCls maps an interpretation band to its tone class. Only `alarm` is
// allowed the danger colour: debt is structure, not an emergency.
func nwsBandCls(b balancesheet.Band) string {
	switch b {
	case balancesheet.BandStrong:
		return " is-strong"
	case balancesheet.BandWatch:
		return " is-watch"
	case balancesheet.BandAlarm:
		return " is-alarm"
	}
	return ""
}

// nwsBandKey suffixes an i18n key with the band, so each band gets its OWN
// sentence rather than one sentence with an adjective swapped in.
func nwsBandKey(b balancesheet.Band) string {
	switch b {
	case balancesheet.BandStrong:
		return "Strong"
	case balancesheet.BandWatch:
		return "Watch"
	case balancesheet.BandAlarm:
		return "Alarm"
	}
	return "Ok"
}

// nwsRatioCard renders one ratio as figure + plain-English reading. A ratio
// never ships as a bare number the reader is left to judge alone.
func nwsRatioCard(testid, label, value, read string, band balancesheet.Band) ui.Node {
	return Div(ClassStr("nws-ratio"+nwsBandCls(band)), Attr("data-testid", testid),
		Span(css.Class("nws-ratio-label"), label),
		Span(ClassStr("nws-ratio-value "+tw.Fold(tw.FontDisplay)), value),
		Span(css.Class("nws-ratio-read"), read),
	)
}

// nwsRatioCards is the interpreted ratio set shared by BOTH views — which is
// also why the two views can never phrase the same ratio differently.
func nwsRatioCards(v nwsView) ui.Node {
	h := v.Health
	liquidRead := uistate.T("nws.ratioUnknownRead")
	if h.LiquidShare.OK {
		liquidRead = uistate.T("nws.readLiquid"+nwsBandKey(h.LiquidShare.Band),
			fmtMoney(money.New(v.CashMinor, v.Base)))
	}
	runwayRead := uistate.T("nws.readRunwayUnknown")
	if h.RunwayOK {
		runwayRead = uistate.T("nws.readRunway"+nwsBandKey(h.RunwayBand),
			fmtMoney(money.New(v.MonthlyExpenseMinor, v.Base)))
	}
	debtRead := uistate.T("nws.ratioUnknownRead")
	if h.DebtToAsset.OK {
		debtRead = uistate.T("nws.readDebt" + nwsBandKey(h.DebtToAsset.Band))
	}
	return Div(css.Class("nws-ratios"), Attr("data-testid", "nws-ratios"),
		nwsRatioCard("nws-ratio-liquid", uistate.T("nws.ratioLiquid"),
			nwsPctText(h.LiquidShare), liquidRead, h.LiquidShare.Band),
		nwsRatioCard("nws-ratio-runway", uistate.T("nws.ratioRunway"),
			uistate.T("nws.runwayValue", nwsRunwayText(h)), runwayRead, h.RunwayBand),
		nwsRatioCard("nws-ratio-debt", uistate.T("nws.ratioDebt"),
			nwsPctText(h.DebtToAsset), debtRead, h.DebtToAsset.Band),
	)
}

// nwsTakeaway is the window's story in one sentence, built from THE BRIDGE
// rather than from the headline: "up $3,818" is not a story; "most of this
// month's gain came from paying down debt, not from saving" is. The largest leg
// by magnitude names the cause. The residual is never offered as an
// explanation — it is an honesty disclosure, not an answer.
func nwsTakeaway(v nwsView) string {
	b := v.Bridge
	delta := b.DeltaMinor()
	window := nwsWindowSpan(v.Months)
	if delta == 0 {
		return uistate.T("nws.takeFlat", fmtMoney(money.New(b.EndMinor, v.Base)), window)
	}
	var top attribution.LegKind
	var topAbs int64
	for _, k := range attribution.BridgeLegOrder {
		if k == attribution.LegResidual {
			continue
		}
		if a := absMinor(b.Leg(k)); a > topAbs {
			top, topAbs = k, a
		}
	}
	mag := fmtMoney(money.New(absMinor(delta), v.Base))
	dir := "nws.takeUp"
	if delta < 0 {
		dir = "nws.takeDown"
	}
	if topAbs == 0 {
		return uistate.T(dir+"Plain", mag, window)
	}
	// The share is of everything that MOVED, not of the net change. Against the
	// net change a leg can read as "99% of the move" while two other legs of
	// £6,700 each quietly cancel out — arithmetically true and materially
	// misleading, because it implies the others were negligible when they were
	// not. Gross movement is the honest denominator for "how much of what
	// happened was this".
	var gross int64
	for _, k := range attribution.BridgeLegOrder {
		if k != attribution.LegResidual {
			gross += absMinor(b.Leg(k))
		}
	}
	share := topAbs * 100 / nwsAtLeastOne(gross)
	return uistate.T(dir, mag, window, nwsLegCause(top), share)
}

// nwsLegCause phrases a leg as the CAUSE of a movement ("paying down debt"),
// not as a column heading.
func nwsLegCause(k attribution.LegKind) string {
	switch k {
	case attribution.LegMoneyKept:
		return uistate.T("nws.causeMoneyKept")
	case attribution.LegMarketMovement:
		return uistate.T("nws.causeMarket")
	case attribution.LegDebtPaidDown:
		return uistate.T("nws.causeDebtPaid")
	case attribution.LegNewDebt:
		return uistate.T("nws.causeNewDebt")
	}
	return uistate.T("nws.causeRevaluation")
}

// nwsAtLeastOne guards a share from dividing by zero.
func nwsAtLeastOne(v int64) int64 {
	if v < 1 {
		return 1
	}
	return v
}

// nwsWindowBtnProps drives one period option.
type nwsWindowBtnProps struct {
	Months   int
	Selected string
	OnSelect func(string)
}

// nwsWindowBtn is one period option. Its own component so the click hook sits at
// a stable call-site per option rather than inside a loop.
func nwsWindowBtn(p nwsWindowBtnProps) ui.Node {
	val := strconv.Itoa(p.Months)
	on := p.Selected == val
	click := ui.UseEvent(Prevent(func() { p.OnSelect(val) }))
	return Button(ClassStr("nws-view"+If2(on, " is-on", "")), Type("button"),
		Attr("data-testid", "nws-win-"+val), Attr("aria-pressed", boolStr(on)),
		OnClick(click), nwsWindowLabel(p.Months))
}

// nwsSection is the page's section chrome: a titled card filling the content
// column. flush drops the card frame for the hero, which draws its own field.
func nwsSection(id, title, note string, action, body ui.Node, flush bool) ui.Node {
	return nwsSectionCls(id, "", title, note, action, body, flush)
}

// nwsSectionCls is nwsSection with an extra modifier class, which is how a
// section states its place in the Glance grid (a full-width row, the narrow
// interpretation column) without the layout leaking into the page function.
func nwsSectionCls(id, mod, title, note string, action, body ui.Node, flush bool) ui.Node {
	cls := "nws-section"
	if mod != "" {
		cls += " " + mod
	}
	if flush {
		cls += " nws-flush"
	}
	args := []any{ClassStr(cls)}
	if id != "" {
		args = append(args, Attr("id", id))
	}
	if title != "" || action != nil {
		args = append(args, Div(css.Class("nws-sec-head"),
			If(title != "", H2(css.Class("nws-sec-title"), title)),
			If(action != nil, action),
		))
	}
	if note != "" {
		args = append(args, P(css.Class("nws-sec-note"), note))
	}
	args = append(args, body)
	return Div(args...)
}

// nwsSlot reserves a deferred section's position in the page stack. It is
// display:contents, so it costs no box and no gap while empty; its only job is
// to exist in the DOM from the first paint so a late mount lands here rather
// than being appended after the last child. It carries no data-testid —
// deferral scaffolding is not a control.
func nwsSlot(body ui.Node) ui.Node { return Div(css.Class("nws-slot"), body) }

// nwsHero is the calm headline: the net figure in the display serif, the signed
// movement over the selected window, and the two side totals. No chart junk.
func nwsHero(v nwsView) ui.Node {
	p := v.Latest()
	delta := v.Bridge.DeltaMinor()
	valCls := "nws-hero-value " + tw.Fold(tw.FontDisplay)
	// A negative net worth is the one balance-sheet state that is unambiguously
	// alarming, so it is the only one that turns the headline red.
	if p.NetMinor < 0 {
		valCls += " is-negative"
	}
	var deltaNode ui.Node = Fragment()
	if delta != 0 {
		arrow, tone := "▲", " "+tw.ColorClass("text-up")
		if delta < 0 {
			arrow, tone = "▼", ""
		}
		deltaNode = Span(ClassStr("nws-hero-delta"+tone), Attr("data-testid", "nw-delta"),
			Attr("title", uistate.T("nws.deltaTitle", nwsWindowSpan(v.Months))),
			arrow+" "+fmtMoney(money.New(absMinor(delta), v.Base))+" "+nwsWindowSpan(v.Months))
	}

	// Exclusions are disclosed, never silently folded into the figure.
	var missingNote, byChoiceNote ui.Node = Fragment(), Fragment()
	if len(v.Snapshot.MissingCurrencies) > 0 {
		missingNote = P(css.Class("err"), Attr("role", "alert"),
			uistate.T("accounts.nwExcludes", plural(len(v.Snapshot.ExcludedAccounts), "account"),
				strings.Join(v.Snapshot.MissingCurrencies, ", ")))
	}
	if n := len(v.Snapshot.ExcludedByChoice); n > 0 {
		byChoiceNote = P(css.Class("t-caption", tw.TextDim), Attr("role", "status"),
			Attr("data-testid", "nw-excludes-by-choice"), uistate.T(excludesByChoiceKey(n), n))
	}

	return nwsSection("", "", "", nil, Div(css.Class("nws-hero"),
		Div(
			ui.CreateElement(nwsQuality, nwsQualityProps{
				Q: nwsAssessQuality(v), Base: v.Base, App: appstate.Default,
				AsOfLine: uistate.T("nw.asOf", uistate.LoadPrefs().FormatDate(v.Now)) + " ",
			}),
			Div(ClassStr(valCls), Attr("data-countup", ""), Attr("data-testid", "nw-hero-value"),
				fmtMoney(money.New(p.NetMinor, v.Base))),
			deltaNode,
			missingNote,
			byChoiceNote,
		),
		Div(css.Class("nws-hero-sides"),
			Div(css.Class("nws-side", "is-assets"),
				Span(css.Class("nws-side-label"), uistate.T("accounts.assets")),
				Span(ClassStr("nws-side-value "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "nws-assets"),
					fmtMoney(money.New(p.AssetsMinor, v.Base))),
			),
			Div(css.Class("nws-side", "is-liabilities"),
				Span(css.Class("nws-side-label"), uistate.T("dashboard.liabilities")),
				Span(ClassStr("nws-side-value "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "nws-liabilities"),
					fmtMoney(money.New(p.LiabilitiesMinor, v.Base))),
			),
		),
	), true)
}

// NetWorth is the /networth screen.
func NetWorth() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	_ = uistate.UsePrefs().Get() // re-render when the accent/theme changes

	cfg := uistate.NetWorthConfigGet()
	window := ui.UseState(fmt.Sprintf("%d", cfg.TrendMonths))
	view := ui.UseState(uistate.NetWorthViewGet())
	showFormulas := ui.UseState(false)

	setWindow := func(val string) {
		window.Set(val)
		months := 6
		fmt.Sscanf(val, "%d", &months)
		uistate.SetNetWorthConfig(uistate.NetWorthConfig{TrendMonths: months})
	}
	onGlance := ui.UseEvent(Prevent(func() {
		view.Set(uistate.NetWorthViewGlance)
		uistate.NetWorthViewSet(uistate.NetWorthViewGlance)
	}))
	onDetail := ui.UseEvent(Prevent(func() {
		view.Set(uistate.NetWorthViewDetail)
		uistate.NetWorthViewSet(uistate.NetWorthViewDetail)
	}))
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))

	// The hero and THE BRIDGE paint immediately; the mirrored chart, the
	// interpreted ratios and the Detail document's long tables arrive once the
	// route has settled, so mount never blocks on the history math.
	settled := useAfterSettle("networth")
	// The scroll-spy runs only in Detail, and its hook stays unconditional.
	nwsPublishNavOffset(view.Get())
	nwsScrollSpy(view.Get() == uistate.NetWorthViewDetail)

	if len(app.Accounts()) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:   uistate.T("reports.emptyNetWorth"),
			CTALabel:  uistate.T("accounts.addFirst"),
			AddTarget: "account",
		})
	}

	months := 6
	fmt.Sscanf(window.Get(), "%d", &months)
	v := computeNwsView(app, months, time.Now())

	isGlance := view.Get() == uistate.NetWorthViewGlance
	toggle := Div(css.Class("nws-views"),
		Div(css.Class("nws-viewset"), Attr("role", "group"), Attr("aria-label", uistate.T("nws.viewAria")),
			Button(ClassStr("nws-view"+If2(isGlance, " is-on", "")), Type("button"),
				Attr("data-testid", "nws-view-glance"), Attr("aria-pressed", boolStr(isGlance)),
				Title(uistate.T("nws.viewGlanceTitle")), OnClick(onGlance), uistate.T("nws.viewGlance")),
			Button(ClassStr("nws-view"+If2(!isGlance, " is-on", "")), Type("button"),
				Attr("data-testid", "nws-view-detail"), Attr("aria-pressed", boolStr(!isGlance)),
				Title(uistate.T("nws.viewDetailTitle")), OnClick(onDetail), uistate.T("nws.viewDetail")),
		),
		// The period control is built from the page's own buttons rather than the
		// shared Segmented: every control on this page needs a data-testid for the
		// coverage ratchet, and adding testids inside the shared component would
		// change the control inventory of every other route that uses it.
		Div(css.Class("nws-window", "nws-viewset"), Attr("role", "group"),
			Attr("aria-label", uistate.T("nws.windowLabel")),
			ui.CreateElement(nwsWindowBtn, nwsWindowBtnProps{Months: nwsWindowMonths[0], Selected: window.Get(), OnSelect: setWindow}),
			ui.CreateElement(nwsWindowBtn, nwsWindowBtnProps{Months: nwsWindowMonths[1], Selected: window.Get(), OnSelect: setWindow}),
			ui.CreateElement(nwsWindowBtn, nwsWindowBtnProps{Months: nwsWindowMonths[2], Selected: window.Get(), OnSelect: setWindow}),
			ui.CreateElement(nwsWindowBtn, nwsWindowBtnProps{Months: nwsWindowMonths[3], Selected: window.Get(), OnSelect: setWindow}),
			ui.CreateElement(nwsWindowBtn, nwsWindowBtnProps{Months: nwsWindowMonths[4], Selected: window.Get(), OnSelect: setWindow}),
		),
	)

	// ── Glance ───────────────────────────────────────────────────────────────
	// A two-column editorial layout, not a stack: the interpretation belongs
	// BESIDE the evidence, not below it. Stacked, a reader met three charts
	// before a single sentence telling them what to make of them — and on a
	// common desktop viewport the sentence was below the fold entirely, which
	// for a view called Glance is a contradiction in terms.
	glance := Div(css.Class("nws-glance"),
		nwsSection("sec-nw-bridge", uistate.T("nws.bridgeTitle"),
			uistate.T("nws.bridgeNote", nwsWindowAgo(v.Months)), nwsBridgeExplain(), nwsBridge(v), false),
		nwsSlot(If(settled, nwsSectionCls("sec-nw-read", "nws-read", uistate.T("nws.readTitle"), "", nil, Fragment(
			P(ClassStr("nws-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "nw-takeaway"), nwsTakeaway(v)),
			Div(Style(map[string]string{"margin-top": "0.9rem"}), nwsRatioCards(v)),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2), Style(map[string]string{"margin-top": "0.9rem"}),
				A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/accounts")),
					Attr("data-testid", "nw-accounts-link"), uistate.T("reports.viewAccounts")),
				A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/debt")),
					Attr("data-testid", "nw-debt-link"), uistate.T("nw.viewDebts")),
			),
		), false))),
		nwsSlot(If(settled, nwsSectionCls("sec-nw-sides", "nws-wide", uistate.T("nws.sidesTitle"),
			uistate.T("nws.sidesNote"), nwsSidesExplain(), nwsSides(v), false))),
	)

	// ── Detail ───────────────────────────────────────────────────────────────
	detail := Fragment(
		nwsIndex(onGlance),
		nwsStandSection(v),
		nwsChangedSection(v),
		nwsSlot(If(settled, nwsSideSection(v, true))),
		nwsSlot(If(settled, nwsSideSection(v, false))),
		nwsSlot(If(settled, nwsHistorySection(v))),
		nwsSlot(If(settled, nwsHealthSection(v))),
	)

	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("nw.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("nw.metricsHide")
	}

	// data-settled states whether the deferred sections have arrived: a deferred
	// section may legitimately render nothing, so "has content" cannot tell
	// "nothing to show" apart from "not here yet".
	return Div(css.Class("nws"), Attr("data-settled", ariaBool(settled)),
		nwsHero(v),
		toggle,
		If(isGlance, glance),
		If(!isGlance, detail),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
			Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
				Attr("data-testid", "nw-toggle-formulas"), Title(uistate.T("nw.metricsTitle")),
				OnClick(toggleFormulas), Text(metricsLabel)),
		),
		If(showFormulas.Get(), nwsSection("", "", uistate.T("nw.formulaHint"), nil,
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("nw.metricsShow"), ShowSaved: true}),
			false)),
	)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// liveSmartCounts summarizes the persisted Smart settings by tier for the
// smart_* engine variables (and this surface's hero) — the single derivation
// both read, so the hero figure and the formula variable can never disagree.
func liveSmartCounts() engineenv.SmartCounts {
	settings := uistate.LoadSmartSettings()
	var c engineenv.SmartCounts
	for _, code := range settings.EnabledCodes() {
		switch {
		case smartengine.HasEngine(code):
			c.FreeOn++
		case smartai.Implemented(code):
			c.AIOn++
		}
	}
	return c
}

// smartHeroVoice picks the hero's agent-voiced line for the current posture.
func smartHeroVoice(counts engineenv.SmartCounts, openInsights int) string {
	switch {
	case counts.FreeOn+counts.AIOn == 0:
		return uistate.T("smart.heroVoiceOff")
	case openInsights > 0:
		return uistate.T("smart.heroVoiceFindings", openInsights)
	default:
		return uistate.T("smart.heroVoiceQuiet")
	}
}

// SmartSurface is the redesigned Smart panel (the /assistant Smart tab and the
// /smart route): ONE flattened bento surface instead of nested tabs — an
// agent-voiced hero tile (how many features are watching, split by tier with
// honest cost chips, the same counts the smart_* engine variables expose),
// then the live insight feed, the AI features and digest pair, and the full
// opt-in catalog — all visible on one scroll, each section keeping its proven
// internals (toggles, pager, cadence, density dial).
func SmartSurface() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get() // re-render on data or settings change

	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()
	backendAI := pr.Normalize().BackendActive()
	hasProvider := aiProviderConfigured(app, backendAI)
	conn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	settings := uistate.LoadSmartSettings()
	insights := runSmart(app, weekStart, settings)
	counts := liveSmartCounts()
	density := settings.DensityOrDefault()

	// The hero counts what the FEED actually shows (per-rule capped, same as
	// smartInsightsSection) so the voice line and the list can't disagree.
	smart.SortInsights(insights)
	findings := len(smart.CapPerRule(insights, 3))

	// ── Hero: what the agent FOUND leads (review: nobody opens a findings feed
	// to admire how many rules are running); the watcher count is a chip. ────────
	heroTone := ""
	if findings > 0 {
		heroTone = " " + tw.ColorClass("text-warn")
	}
	chips := []ui.Node{
		rptChip(uistate.T("smart.chipWatching"), fmt.Sprintf("%d", counts.FreeOn+counts.AIOn), ""),
		rptChip(uistate.T("smart.chipAI"), fmt.Sprintf("%d", counts.AIOn), ""),
		rptChip(uistate.T("smart.chipDensity"), uistate.T("smart.density."+string(density)), ""),
	}
	hero := astTile("smt-hero", "1 / span 4", astSection("sec-smart-hero", uistate.T("smart.heroTitle"), nil,
		Div(css.Class("rpt-hero"),
			P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("smart.heroEyebrow")),
			Div(css.Class("rpt-hero-main"),
				Div(
					Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)+heroTone), Attr("data-testid", "smt-hero-count"),
						fmt.Sprintf("%d", findings)),
					Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("smart.heroLabel")),
				),
			),
			P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "smt-hero-voice"),
				smartHeroVoice(counts, findings)),
			Div(css.Class("debt-chips"), chips),
		)))

	// ── The proven sections, flattened onto one surface as bento children. ──────
	var freeEnabled int
	for _, code := range settings.EnabledCodes() {
		if smartengine.HasEngine(code) {
			freeEnabled++
		}
	}
	span := func(col string, n ui.Node) ui.Node {
		return Div(Style(map[string]string{"grid-column": col}), n)
	}
	tiles := []ui.Node{
		hero,
		span("1 / span 4", smartInsightsSection(insights, freeEnabled, counts.FreeOn+counts.AIOn > 0)),
	}
	// AI feature outputs are content — they follow the feed; the digest is
	// config, so it rides with the catalog at the bottom (review: a scheduling
	// widget wedged between content and content read as an orphan).
	if counts.AIOn > 0 {
		tiles = append(tiles, span("1 / span 4", smartAISection(settings, conn, hasProvider)))
	}
	tiles = append(tiles,
		span("1 / span 4", smartManageSection(settings, hasProvider)),
		span("1 / span 4", SmartDigestSection(settings)),
	)

	return Div(css.Class("bento bento-smart"), Attr("data-testid", "smart-hub"), tiles)
}

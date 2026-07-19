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
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	// Defer the below-the-fold opt-in catalog + digest config off the initial mount;
	// the findings feed (the reason people open this page) paints immediately.
	catalogReady := useAfterSettle("smart-hub")

	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()
	backendAI := pr.Normalize().BackendActive()
	hasProvider := aiProviderConfigured(app, backendAI)
	conn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	settings := uistate.LoadSmartSettings()
	insights := runSmart(app, weekStart, settings)
	// Reconcile overlapping findings: two engines can reach the same conclusion,
	// and showing it twice reads as noise. Dedupe once, up front, so both the hero
	// count and every downstream section work off the same reconciled set.
	insights = smart.DedupeInsights(insights)
	counts := liveSmartCounts()
	density := settings.DensityOrDefault()

	// The hero leads with what NEEDS the user (Warn + Alert), not the full catalog
	// tally — a "108 findings" headline reads as homework, not help. The count is
	// taken from the per-rule-capped, deduped set so it agrees with the default
	// "Needs you" feed the sections render below.
	smart.SortInsights(insights)
	capped := smart.CapPerRule(insights, 3)
	findings := 0
	for _, ins := range capped {
		if smart.NeedsAttention(ins) {
			findings++
		}
	}

	// ── Bespoke masthead (built from scratch — NO bento tile, NO astSection): a
	// serif kicker, the big FINDINGS count leading (nobody opens a findings feed to
	// admire how many rules run), the agent's voice line, quiet inline posture
	// metrics, and the on-device promise as fine print. Keeps #sec-smart-hero and
	// the smt-hero-* testids so the count/voice/metrics stay addressable. ─────────
	heroTone := ""
	if findings > 0 {
		heroTone = " " + tw.ColorClass("text-warn")
	}
	// Label BEFORE value so the metric reads "Watching 66" in the DOM.
	metric := func(label, value string) ui.Node {
		return Div(css.Class("smt-metric"),
			Span(css.Class("smt-metric-label"), label),
			Span(css.Class("smt-metric-value", tw.FontDisplay), value),
		)
	}
	masthead := Div(css.Class("smt-masthead"), Attr("id", "sec-smart-hero"),
		Span(css.Class("smt-kicker"), uistate.T("smart.heroTitle")),
		Div(css.Class("smt-headline"),
			Div(ClassStr("smt-count "+tw.Fold(tw.FontDisplay)+heroTone), Attr("data-testid", "smt-hero-count"),
				fmt.Sprintf("%d", findings)),
			Div(css.Class("smt-count-label", tw.TextDim), uistate.T("smart.heroLabel")),
		),
		P(ClassStr("smt-voice "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "smt-hero-voice"),
			smartHeroVoice(counts, findings)),
		Div(css.Class("smt-metrics"),
			metric(uistate.T("smart.chipWatching"), fmt.Sprintf("%d", counts.FreeOn+counts.AIOn)),
			metric(uistate.T("smart.chipAI"), fmt.Sprintf("%d", counts.AIOn)),
			metric(uistate.T("smart.chipDensity"), uistate.T("smart.density."+string(density))),
		),
		P(css.Class("smt-fine", tw.TextDim), uistate.T("smart.heroEyebrow")),
	)

	// ── The proven sections, stacked as bespoke blocks on one editorial surface.
	// They still return their EntityListSection/Card internals (toggles, pager,
	// cadence, density dial intact); the .smt-deck scoped CSS dissolves the card
	// chrome so they read as bespoke sections, not stacked tiles. ────────────────
	var freeEnabled int
	for _, code := range settings.EnabledCodes() {
		if smartengine.HasEngine(code) {
			freeEnabled++
		}
	}
	// Cap the feed above the fold on mount; the rest of the findings render after
	// first paint (catalogReady). The masthead count still reflects the full set.
	feedInsights := insights
	if !catalogReady && len(feedInsights) > 5 {
		feedInsights = feedInsights[:5]
	}
	blocks := []ui.Node{
		masthead,
		smartInsightsSection(feedInsights, freeEnabled, counts.FreeOn+counts.AIOn > 0),
	}
	// AI feature outputs are content — they follow the feed; the digest is config,
	// so it rides with the catalog at the bottom.
	if counts.AIOn > 0 {
		blocks = append(blocks, smartAISection(settings, conn, hasProvider))
	}
	// The opt-in catalog + digest config are the page's heaviest DOM (every feature
	// row) and sit at the bottom — deferred to a child that mounts after first paint,
	// so they never block the feed's initial render.
	if catalogReady {
		blocks = append(blocks, ui.CreateElement(smartCatalogDeferred, smartCatalogDeferredProps{
			Settings: settings, HasProvider: hasProvider,
		}))
	}

	return Div(css.Class("smt-deck"), Attr("data-testid", "smart-hub"), blocks)
}

// smartCatalogDeferredProps carries what the deferred catalog needs.
type smartCatalogDeferredProps struct {
	Settings    smart.Settings
	HasProvider bool
}

// smartCatalogDeferred renders the opt-in catalog + digest config as an isolated
// child component, so mounting it a beat after the page's first paint keeps its
// heavy DOM off the mount critical path without touching the parent's hook order.
func smartCatalogDeferred(props smartCatalogDeferredProps) ui.Node {
	return Fragment(
		smartManageSection(props.Settings, props.HasProvider),
		SmartDigestSection(props.Settings),
	)
}

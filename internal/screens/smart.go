// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
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

// SmartHub is the Smart screen (/smart): a single glanceable home for the
// optional per-page intelligence layer. It shows the active insights from every
// enabled Free feature, plus a "Manage" catalog of opt-in toggles that is honest
// about cost — each feature is tagged Free (on-device, $0) or AI (needs an
// inference provider, billed per call). Everything is opt-in; nothing here runs
// or costs anything until the user turns it on.
func SmartHub() ui.Node {
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

	// Count what's enabled, split by tier, to choose the right empty states.
	var freeEnabled, aiEnabled int
	for _, code := range settings.EnabledCodes() {
		if smartengine.HasEngine(code) {
			freeEnabled++
		} else if smartai.Implemented(code) {
			aiEnabled++
		}
	}

	return Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol, tw.Gap4)),
		Attr("data-testid", "smart-hub"),

		// Insights section — the glanceable payoff from the Free engines.
		smartInsightsSection(insights, freeEnabled, freeEnabled+aiEnabled > 0),

		// Interactive AI features (e.g. account Q&A), gated on a configured provider.
		smartAISection(settings, conn, hasProvider),

		// Manage section — the opt-in catalog with honest cost labels.
		smartManageSection(settings, hasProvider),
	)
}

// smartInsightsSection renders the active Free-engine insights, or a calm
// empty/onboarding state. anyEnabled covers AI features too, so the onboarding
// copy only shows when nothing at all is on; when only AI features are enabled
// the section steps aside (the AI section carries the value).
func smartInsightsSection(insights []smart.Insight, freeEnabled int, anyEnabled bool) ui.Node {
	var body ui.Node
	switch {
	case !anyEnabled:
		body = P(ClassStr(tw.Fold(tw.Text14, tw.TextDim)), uistate.T("smart.onboard"))
	case freeEnabled == 0:
		return Fragment() // only AI features on — nothing to show here
	case len(insights) == 0:
		body = P(ClassStr(tw.Fold(tw.Text14, tw.TextDim)), uistate.T("smart.allClear"))
	default:
		body = smartInsightList(insights)
	}
	return uiw.Card(uiw.CardProps{
		Header: smartBrandHeader(uistate.T("smart.insightsTitle"), false, nil),
		TestID: "smart-insights",
		Body:   body,
	})
}

// implementedFeaturesForPage returns the page's features that have a working
// engine today, so the manage list never offers a dead toggle.
func implementedFeaturesForPage(p smart.Page) []smart.Feature {
	var out []smart.Feature
	for _, f := range smart.FeaturesForPage(p) {
		if smartengine.HasEngine(f.Code) || smartai.Implemented(f.Code) {
			out = append(out, f)
		}
	}
	return out
}

// pagesWithImplemented returns the pages that have at least one shipped feature.
func pagesWithImplemented() []smart.Page {
	var out []smart.Page
	for _, p := range smart.Pages() {
		if len(implementedFeaturesForPage(p)) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// smartManageSection renders the opt-in catalog grouped by page. Only features
// with a working engine are listed, so every toggle has a real effect.
func smartManageSection(settings smart.Settings, hasProvider bool) ui.Node {
	return uiw.Card(uiw.CardProps{
		Header: smartBrandHeader(uistate.T("smart.manageTitle"), false, nil),
		TestID: "smart-manage",
		Body: Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap2)),
			P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)), uistate.T("smart.manageHint")),
			Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap3)),
				MapKeyed(pagesWithImplemented(),
					func(p smart.Page) any { return string(p) },
					func(p smart.Page) ui.Node { return smartPageGroup(p, settings, hasProvider) },
				),
			),
		),
	})
}

// smartPageGroup renders one page's heading and its feature toggle rows.
func smartPageGroup(page smart.Page, settings smart.Settings, hasProvider bool) ui.Node {
	feats := implementedFeaturesForPage(page)
	return Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap1)),
		H3(ClassStr(tw.Fold(tw.Text12, tw.FontSemibold, tw.TextFaint, tw.Mt2)), page.Label()),
		Div(ClassStr(tw.Fold(tw.FlexCol)),
			MapKeyed(feats,
				func(f smart.Feature) any { return f.Code },
				func(f smart.Feature) ui.Node {
					return ui.CreateElement(smartFeatureRow, smartRowProps{F: f, On: settings.IsEnabled(f.Code), HasProvider: hasProvider})
				},
			),
		),
	)
}

// smartRowProps carries one catalog feature and its current opt-in state.
type smartRowProps struct {
	F           smart.Feature
	On          bool
	HasProvider bool
}

// tierBadge renders the Free/AI cost pill for a feature — the heart of the
// cost-transparency promise.
func tierBadge(f smart.Feature, hasProvider bool) ui.Node {
	if f.Tier == smart.TierFree {
		return Span(ClassStr(tw.Fold(tw.Text11, tw.FontMedium, tw.TextUp, tw.BgUp, tw.Px1, tw.Py05, tw.Rounded)),
			uistate.T("smart.tierFree"),
		)
	}
	cost := f.EstimateCost(false)
	label := uistate.T("smart.tierAI") + " · " + smart.FormatCents(cost.Cents) + uistate.T("smart.perUse")
	if !hasProvider {
		label += " · " + uistate.T("smart.needsProvider")
	}
	return Span(ClassStr(tw.Fold(tw.Text11, tw.FontMedium, tw.BgSky15, tw.Px1, tw.Py05, tw.Rounded)),
		label,
	)
}

// smartFeatureRow is one opt-in toggle row: the feature name + summary, its
// Free/AI cost badge, and a switch. Its own component so the toggle's hook sits
// at a stable position (the On*-hooks-in-loops rule).
func smartFeatureRow(props smartRowProps) ui.Node {
	rev := uistate.UseDataRevision()
	f := props.F
	onChange := func(on bool) {
		uistate.SetSmartFeatureEnabled(f.Code, on)
		rev.Set(rev.Get() + 1)
	}
	return Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.Py2, tw.BorderB, tw.BorderLine)),
		Attr("data-testid", "smart-feature-"+f.Code),
		Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap1, tw.MinW0)),
			Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
				Span(ClassStr(tw.Fold(tw.Text14, tw.FontMedium)), f.Title),
				tierBadge(f, props.HasProvider),
			),
			Span(ClassStr(tw.Fold(tw.Text12, tw.TextDim)), f.Summary),
		),
		uiw.Toggle(uiw.ToggleProps{On: props.On, OnChange: onChange, Label: f.Title}),
	)
}

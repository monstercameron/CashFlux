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
// with a working engine are listed, so every toggle has a real effect. Its
// header carries the global controls: the density dial + bulk enable/disable.
func smartManageSection(settings smart.Settings, hasProvider bool) ui.Node {
	return uiw.Card(uiw.CardProps{
		Header: smartBrandHeader(uistate.T("smart.manageTitle"), false, ui.CreateElement(smartManageControls, struct{}{})),
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

// smartManageControls renders the global SMART controls in the manage header: the
// density dial (how much smart weaves into the app) plus Enable all / Disable all.
// Its own component so the On* hooks sit at stable positions.
func smartManageControls(_ struct{}) ui.Node {
	rev := uistate.UseDataRevision()
	settings := uistate.LoadSmartSettings()
	density := settings.DensityOrDefault()
	enabledCount := settings.EnabledCount()

	onDensity := ui.UseEvent(func(v string) {
		uistate.SetSmartDensity(smart.Density(v))
		rev.Set(rev.Get() + 1)
	})
	onEnableAll := ui.UseEvent(func() {
		uistate.EnableAllSmart()
		rev.Set(rev.Get() + 1)
	})
	onDisableAll := ui.UseEvent(func() {
		uistate.DisableAllSmart()
		rev.Set(rev.Get() + 1)
	})

	disableAllBtn := Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
		Attr("data-testid", "smart-disable-all"),
		Attr("aria-label", uistate.T("smart.disableAll")),
		OnClick(onDisableAll),
		uistate.T("smart.disableAll"),
	)
	if enabledCount == 0 {
		disableAllBtn = Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-disable-all"), Attr("disabled", "true"),
			uistate.T("smart.disableAll"),
		)
	}

	return Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
		Span(ClassStr(tw.Fold(tw.Text12, tw.TextFaint)), uistate.T("smart.densityLabel")),
		Select(ClassStr("field "+tw.Fold(tw.Text12)),
			Attr("data-testid", "smart-density"),
			Attr("aria-label", uistate.T("smart.densityLabel")),
			OnChange(onDensity),
			MapKeyed(smart.AllDensities(),
				func(d smart.Density) any { return string(d) },
				func(d smart.Density) ui.Node {
					return Option(Value(string(d)), SelectedIf(d == density), d.Label())
				},
			),
		),
		Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-enable-all"),
			Attr("aria-label", uistate.T("smart.enableAll")),
			OnClick(onEnableAll),
			uistate.T("smart.enableAll"),
		),
		disableAllBtn,
	)
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

// smartFeatureRow is one feature's management row: the name + cost badge, and —
// when enabled — its run controls (a cadence/schedule picker for AI features, a
// mute/snooze button) plus the on/off switch. Its own component so the On* hooks
// sit at stable positions (the On*-hooks-in-loops rule).
func smartFeatureRow(props smartRowProps) ui.Node {
	rev := uistate.UseDataRevision()
	f := props.F
	settings := uistate.LoadSmartSettings()
	muted := settings.IsMuted(f.Code)
	cad := settings.CadenceFor(f.Code)

	onChange := func(on bool) {
		uistate.SetSmartFeatureEnabled(f.Code, on)
		rev.Set(rev.Get() + 1)
	}
	onMute := ui.UseEvent(func() {
		uistate.SetSmartMuted(f.Code, !muted)
		rev.Set(rev.Get() + 1)
	})
	onCadence := ui.UseEvent(func(v string) {
		uistate.SetSmartCadence(f.Code, smart.Cadence(v))
		rev.Set(rev.Get() + 1)
	})

	// Run controls — only meaningful when the feature is on.
	var controls ui.Node = Fragment()
	if props.On {
		// Cadence/schedule picker: AI only (Free features are free + instant, so
		// they always run live — scheduling them would be a no-op control).
		var cadencePicker ui.Node = Fragment()
		if f.Tier == smart.TierAI {
			cadencePicker = Select(ClassStr("field "+tw.Fold(tw.Text12)),
				Attr("data-testid", "smart-cadence-"+f.Code),
				Attr("aria-label", uistate.T("smart.schedule")),
				OnChange(onCadence),
				MapKeyed(smart.AllCadences(),
					func(c smart.Cadence) any { return string(c) },
					func(c smart.Cadence) ui.Node {
						return Option(Value(string(c)), SelectedIf(c == cad), c.Label())
					},
				),
			)
		}
		muteLabel := uistate.T("smart.mute")
		if muted {
			muteLabel = uistate.T("smart.muted")
		}
		controls = Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
			cadencePicker,
			Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
				Attr("data-testid", "smart-mute-"+f.Code),
				Attr("aria-pressed", boolAttrStr(muted)),
				OnClick(onMute),
				muteLabel,
			),
		)
	}

	leftCls := tw.Fold(tw.FlexCol, tw.Gap1, tw.MinW0)
	if muted {
		leftCls = tw.Fold(tw.FlexCol, tw.Gap1, tw.MinW0, tw.Opacity60)
	}
	return Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.Py2, tw.BorderB, tw.BorderLine)),
		Attr("data-testid", "smart-feature-"+f.Code),
		Div(ClassStr(leftCls),
			Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
				Span(ClassStr(tw.Fold(tw.Text14, tw.FontMedium)), f.Title),
				tierBadge(f, props.HasProvider),
			),
			Span(ClassStr(tw.Fold(tw.Text12, tw.TextDim)), f.Summary),
		),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap3)),
			controls,
			uiw.Toggle(uiw.ToggleProps{On: props.On, OnChange: onChange, Label: f.Title}),
		),
	)
}

// boolAttrStr renders a bool as an aria attribute value.
func boolAttrStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

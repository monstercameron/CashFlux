// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

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

// SmartHub is the Smart screen (/smart): a tabbed home for the optional
// per-page intelligence layer. The "Insights" tab (default) shows the active
// insights from every enabled Free feature, the AI feature panel, and the
// digest controls. The "Manage" tab shows the full opt-in catalog with honest
// cost labels — Free (on-device, $0) or AI (billed per call). Everything is
// opt-in; nothing runs or costs anything until the user turns it on.
func SmartHub() ui.Node {
	// The redesigned surface: one flattened bento (hero + feed + AI/digest +
	// catalog) — no nested tabs. Kept as the registered name both the /smart
	// route and the /assistant Smart tab reference.
	return SmartSurface()
}

// insightsPagerProps carries the capped insight list into the pager component.
type insightsPagerProps struct {
	Insights []smart.Insight
}

// insightsPageSize is the number of insights shown per page in the Insights tab.
const insightsPageSize = 10

// smartInsightsPager renders the paginated insight list. It is its own component
// so the pagination On* hooks sit at stable positions (no hooks in loops rule).
func smartInsightsPager(props insightsPagerProps) ui.Node {
	page := ui.UseState(0) // 0-based current page

	all := props.Insights
	total := len(all)
	totalPages := (total + insightsPageSize - 1) / insightsPageSize
	if totalPages < 1 {
		totalPages = 1
	}

	cur := page.Get()
	if cur >= totalPages {
		cur = totalPages - 1
		page.Set(cur)
	}

	start := cur * insightsPageSize
	end := start + insightsPageSize
	if end > total {
		end = total
	}
	pageInsights := all[start:end]

	onPrev := ui.UseEvent(func() {
		if p := page.Get(); p > 0 {
			page.Set(p - 1)
		}
	})
	onNext := ui.UseEvent(func() {
		if p := page.Get(); p < totalPages-1 {
			page.Set(p + 1)
		}
	})

	var pager ui.Node = Fragment()
	if totalPages > 1 {
		prevBtn := Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-insights-prev"),
			Attr("aria-label", uistate.T("smart.prevPage")),
			OnClick(onPrev),
			uistate.T("smart.prevPage"),
		)
		if cur == 0 {
			prevBtn = Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
				Attr("data-testid", "smart-insights-prev"),
				Attr("disabled", "true"),
				uistate.T("smart.prevPage"),
			)
		}
		nextBtn := Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-insights-next"),
			Attr("aria-label", uistate.T("smart.nextPage")),
			OnClick(onNext),
			uistate.T("smart.nextPage"),
		)
		if cur >= totalPages-1 {
			nextBtn = Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
				Attr("data-testid", "smart-insights-next"),
				Attr("disabled", "true"),
				uistate.T("smart.nextPage"),
			)
		}
		pager = Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Mt2)),
			Attr("data-testid", "smart-insights-pager"),
			prevBtn,
			Span(ClassStr(tw.Fold(tw.Text12, tw.TextDim)),
				uistate.T("smart.pageOf", cur+1, totalPages),
			),
			nextBtn,
		)
	}

	return Fragment(
		smartInsightList(pageInsights),
		pager,
	)
}

// smartInsightsSection renders the active Free-engine insights, or a calm
// empty/onboarding state. anyEnabled covers AI features too, so the onboarding
// copy only shows when nothing at all is on; when only AI features are enabled
// the section steps aside (the AI section carries the value). Insights are
// capped to 3 per rule (highest-severity kept) and then paginated.
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
		smart.SortInsights(insights)
		capped := smart.CapPerRule(insights, 3)
		// Wrapped in a plain Div: a bare component as the card's direct child
		// mounts AFTER its header sibling in the DOM (GWC component-sibling
		// ordering quirk), which rendered the "Your insights" title at the
		// card's bottom.
		body = Div(ui.CreateElement(smartInsightsPager, insightsPagerProps{Insights: capped}))
	}
	return uiw.Card(uiw.CardProps{
		// "Findings" — deliberately NOT "insights" (review: the AI-narrative
		// pinned insights on the other tabs are a different concept; one word
		// per concept).
		Header: smartBrandHeader(uistate.T("smart.findingsTitle"), false, nil),
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
			Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap1)),
				// Each page's features fold behind an accordion header carrying its
				// enabled/total count — sixty-plus toggles as one flat wall
				// overwhelmed the page (review P1). Controlled per-group components
				// (not <details>) so a toggle's re-render can't snap groups shut.
				MapKeyed(pagesWithImplemented(),
					func(p smart.Page) any { return string(p) },
					func(p smart.Page) ui.Node {
						feats := implementedFeaturesForPage(p)
						enabled := 0
						for _, f := range feats {
							if settings.IsEnabled(f.Code) {
								enabled++
							}
						}
						return ui.CreateElement(smartCatalogGroup, smartCatalogGroupProps{
							Page: p, Enabled: enabled, Total: len(feats),
							Settings: settings, HasProvider: hasProvider,
						})
					},
				),
			),
		),
	})
}

type smartCatalogGroupProps struct {
	Page        smart.Page
	Enabled     int
	Total       int
	Settings    smart.Settings
	HasProvider bool
}

// smartCatalogOpen remembers which catalog groups are expanded for the whole
// session (a package-level map, not component state — toggling a feature bumps
// the data revision and can remount the surface, which would otherwise snap
// every group shut mid-interaction).
var smartCatalogOpen = map[string]bool{}

// smartCatalogGroup renders one page's feature group behind a collapsible
// header (page name + enabled/total). Its own component so the toggle hook
// sits at a stable position; the open state lives in smartCatalogOpen.
func smartCatalogGroup(p smartCatalogGroupProps) ui.Node {
	rev := ui.UseState(0)
	open := smartCatalogOpen[string(p.Page)]
	tog := ui.UseEvent(Prevent(func() {
		smartCatalogOpen[string(p.Page)] = !smartCatalogOpen[string(p.Page)]
		rev.Set(rev.Get() + 1)
	}))
	caret := "▸"
	if open {
		caret = "▾"
	}
	return Div(
		Button(css.Class("fb-pal-head"), Type("button"),
			Attr("aria-expanded", ariaBool(open)),
			Attr("data-testid", "smart-group-"+string(p.Page)),
			OnClick(tog),
			Span(css.Class("fb-pal-caret"), caret),
			Span(css.Class("fb-pal-title"), p.Page.Label()),
			Span(css.Class("fb-pal-count"), fmt.Sprintf("%d / %d", p.Enabled, p.Total)),
		),
		If(open, smartPageGroup(p.Page, p.Settings, p.HasProvider)),
	)
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
	onEnableFree := ui.UseEvent(func() {
		uistate.EnableFreeSmart()
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
			Attr("data-testid", "smart-enable-free"),
			Attr("aria-label", uistate.T("smart.enableFreeOnly")),
			OnClick(onEnableFree),
			uistate.T("smart.enableFreeOnly"),
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
		return Span(ClassStr(tw.Fold(tw.Text11, tw.FontMedium, tw.TextUp, tw.BgUp15, tw.Px1, tw.Py05, tw.Rounded)),
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

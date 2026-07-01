// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// This file holds the reusable "smart affordance" toolkit — the small components
// that weave the SMART layer into the fabric of the app (rows, figures, sections)
// beyond the /smart hub and the per-page strip. Every one is gated: it renders
// only when its feature is enabled + not muted AND the global density permits its
// kind (smart.Settings.ShowsAffordance). So the app is "riddled with smart" only
// as much as the user's density dial allows.

// insightsByEntity indexes insights by the entity their action targets
// (Action.RelatedID), so a row can pick out the insight(s) about its own record
// without re-running any engine per row.
func insightsByEntity(insights []smart.Insight) map[string][]smart.Insight {
	out := map[string][]smart.Insight{}
	for _, ins := range insights {
		if ins.Action != nil && ins.Action.RelatedID != "" {
			out[ins.Action.RelatedID] = append(out[ins.Action.RelatedID], ins)
		}
	}
	return out
}

// --- SmartBadge: a quiet severity dot on a row/figure ---------------------

type smartBadgeProps struct {
	Ins smart.Insight
}

// smartBadge renders a tiny clickable severity dot carrying the insight's title
// as its tooltip; clicking opens the Smart hub. Its own component for the click
// hook. Callers should use smartBadgeFor, which applies the density/enable gate.
func smartBadge(props smartBadgeProps) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/smart")) })
	tone := severityTone(props.Ins.Severity)
	return Button(ClassStr("btn-icon-bare "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)), Type("button"),
		Attr("data-testid", "smart-badge-"+props.Ins.Feature),
		Attr("aria-label", props.Ins.Title),
		Attr("title", props.Ins.Title),
		OnClick(open),
		Span(ClassStr(tw.Fold(tw.Text12)+" "+tw.ColorClass(tone)), "✦"),
	)
}

// smartBadgeFor returns a severity badge for an entity when an enabled, non-muted
// feature has an insight targeting it and the density permits badges; otherwise
// it renders nothing. `byEntity` comes from insightsByEntity over the page's
// insights. The highest-severity qualifying insight wins.
func smartBadgeFor(settings smart.Settings, byEntity map[string][]smart.Insight, relatedID string) ui.Node {
	var best *smart.Insight
	for i := range byEntity[relatedID] {
		ins := byEntity[relatedID][i]
		if !settings.ShowsAffordance(ins.Feature, smart.AffordanceBadge) {
			continue
		}
		if best == nil || ins.Severity > best.Severity {
			b := ins
			best = &b
		}
	}
	if best == nil {
		return Fragment()
	}
	return ui.CreateElement(smartBadge, smartBadgeProps{Ins: *best})
}

// --- SmartTooltip: an opt-in explainer popover on a figure/control --------

type smartTooltipProps struct {
	// ID uniquely identifies this tooltip's popover wrapper on the page.
	ID string
	// Title is the short heading of the explainer.
	Title string
	// Text is the plain-English explanation (Free, templated — no AI).
	Text string
}

// smartTooltip renders a small "ⓘ"-style info button that toggles an accessible
// explainer popover (click to open, Escape/outside-click to close via
// DismissPopover). Its own component for the open-state + dismiss hooks. Callers
// use smartTooltipFor, which applies the density gate.
func smartTooltip(props smartTooltipProps) ui.Node {
	open := ui.UseState(false)
	wrapID := "smarttip-" + props.ID
	uiw.DismissPopover(open.Get(), wrapID, func() { open.Set(false) })
	// Reuse the .add-menu overlay look but position it as a FIXED popover so it floats
	// over content — never expanding the stat/loader box it lives in, never clipped by
	// that box's overflow:hidden, and with a z-index respected against the whole page. It
	// stays viewport-aware (flips above / clamps horizontally) regardless of the ⓘ's side.
	uiw.AnchorFixedPopover(open.Get(), wrapID)

	var pop ui.Node = Fragment()
	if open.Get() {
		pop = Div(ClassStr("add-menu smart-tip-pop"),
			Attr("role", "tooltip"),
			Attr("data-testid", "smart-tip-pop"),
			Div(ClassStr(tw.Fold(tw.FontSemibold, tw.Text13)), props.Title),
			P(ClassStr(tw.Fold(tw.Text12, tw.TextDim, tw.Mt1)), props.Text),
		)
	}
	return Span(ClassStr("smart-tip-wrap add-wrap "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)), Attr("id", wrapID),
		Attr("data-testid", "smart-tip-"+props.ID),
		uiw.IconButton(uiw.IconButtonProps{
			Icon:    icon.HelpCircle,
			Label:   props.Title,
			OnClick: func() { open.Set(!open.Get()) },
			Class:   "btn-icon-bare " + tw.Fold(tw.TextFaint),
		}),
		pop,
	)
}

// smartTooltipFor returns the explainer when the density permits tooltips, else
// nothing. Tooltips are general help (not tied to one feature's opt-in), so they
// are gated by density alone.
func smartTooltipFor(settings smart.Settings, id, title, text string) ui.Node {
	if !settings.DensityOrDefault().Shows(smart.AffordanceTooltip) {
		return Fragment()
	}
	return ui.CreateElement(smartTooltip, smartTooltipProps{ID: id, Title: title, Text: text})
}

// --- SmartSectionAction: a sparkle quick-access in a toolbar/section ------

// smartSectionAction renders a compact sparkle button that jumps to the Smart hub
// — quick access to the page's smart features (run, schedule, manage) from a
// section toolbar. Gated by the section-action density level. Its own component
// for the click hook.
func smartSectionAction(settings smart.Settings) ui.Node {
	if !settings.DensityOrDefault().Shows(smart.AffordanceSectionAction) {
		return Fragment()
	}
	return ui.CreateElement(smartSectionActionBtn, struct{}{})
}

func smartSectionActionBtn(_ struct{}) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/smart")) })
	return Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
		Attr("data-testid", "smart-section-action"),
		Attr("aria-label", uistate.T("smart.stripTitle")),
		OnClick(open),
		smartGlyph(false, tw.Fold(tw.W4, tw.H4)),
		Span(ClassStr(tw.Fold(tw.Ml1)), uistate.T("smart.stripTitle")),
	)
}

// --- SmartFieldAssist: inline suggestion chip inside a form field ----------

// SmartFieldAssistProps configures a field-assist chip.
type SmartFieldAssistProps struct {
	// Settings is the current SMART settings (for density gate).
	Settings smart.Settings
	// ID is a stable string used to build data-testid attributes. Must be unique
	// per form/field combination on the page (e.g. "qa-desc", "goal-wish").
	ID string
	// Suggestion is the cleaned/computed value to offer. An empty suggestion means
	// nothing to suggest; the chip renders nothing.
	Suggestion string
	// OnApply is called when the user clicks the chip to accept the suggestion.
	OnApply func()
}

// smartFieldAssistInner is the inner component that holds the click hook.  It
// must be its own component (not inlined) so UseEvent is called at a stable
// render position — never inside a variable-length loop.
func smartFieldAssistInner(props SmartFieldAssistProps) ui.Node {
	apply := ui.UseEvent(func() {
		if props.OnApply != nil {
			props.OnApply()
		}
	})
	return Span(ClassStr("smart-assist-wrap "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.Mt1)),
		Attr("data-testid", "smart-assist-"+props.ID),
		smartGlyph(false, tw.Fold(tw.W3, tw.H3, tw.ShrinkO)),
		Button(ClassStr("btn-link "+tw.Fold(tw.Text12, tw.TextDim)),
			Type("button"),
			Attr("data-testid", "smart-assist-"+props.ID+"-apply"),
			Attr("aria-label", "Use suggestion: "+props.Suggestion),
			OnClick(apply),
			Text(`Use "`+props.Suggestion+`"`),
		),
	)
}

// SmartFieldAssist renders a compact inline suggestion chip ("✦ Use "…"") below
// a form field. Clicking the chip calls OnApply so the caller can fill the field.
//
// Renders nothing (Fragment) when:
//   - The density setting does not permit AffordanceFieldAssist, OR
//   - Suggestion is empty.
//
// The chip is its own component so its UseEvent hook is always at a stable position.
func SmartFieldAssist(settings smart.Settings, id, suggestion string, onApply func()) ui.Node {
	if suggestion == "" {
		return Fragment()
	}
	if !settings.DensityOrDefault().Shows(smart.AffordanceFieldAssist) {
		return Fragment()
	}
	return ui.CreateElement(smartFieldAssistInner, SmartFieldAssistProps{
		Settings:   settings,
		ID:         id,
		Suggestion: suggestion,
		OnApply:    onApply,
	})
}

// smartFieldAssist is the package-internal shorthand for SmartFieldAssist,
// kept for use within the screens package.
func smartFieldAssist(settings smart.Settings, id, suggestion string, onApply func()) ui.Node {
	return SmartFieldAssist(settings, id, suggestion, onApply)
}

// --- SmartEmptyState: "set this up with help" helper in an empty section ----

// smartEmptyStateProps carries the parameters for an empty-state smart helper.
type smartEmptyStateProps struct {
	// Insights is the capped list of insights to surface (caller already caps to 1).
	Insights []smart.Insight
	// Page is used to form the data-testid.
	Page smart.Page
}

// smartEmptyStateInner renders the branded empty-state insight list.
// Its own component so any hook inside smartInsightList sits at stable positions.
func smartEmptyStateInner(props smartEmptyStateProps) ui.Node {
	if len(props.Insights) == 0 {
		return Fragment()
	}
	return Div(ClassStr("smart-emptystate "+tw.Fold(tw.FlexCol, tw.Gap2, tw.Mt2)),
		Attr("data-testid", "smart-emptystate-"+string(props.Page)),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Text13, tw.TextDim)),
			smartGlyph(false, tw.Fold(tw.W3, tw.H3, tw.ShrinkO)),
			Span(uistate.T("smart.emptyHint")),
		),
		smartInsightList(props.Insights),
	)
}

// smartEmptyStateFor returns a smart helper block for an empty section when:
// (a) the density permits AffordanceEmptyState, (b) the given page has at least
// one enabled + active insight. Capped to 1 insight so it stays lightweight.
// Returns nothing when the gate fails or there are no insights.
func smartEmptyStateFor(settings smart.Settings, page smart.Page, in smartengine.Input) ui.Node {
	if !settings.DensityOrDefault().Shows(smart.AffordanceEmptyState) {
		return Fragment()
	}
	insights := smartengine.RunPage(in, settings, page)
	if len(insights) == 0 {
		return Fragment()
	}
	if len(insights) > 1 {
		insights = insights[:1]
	}
	return ui.CreateElement(smartEmptyStateInner, smartEmptyStateProps{Insights: insights, Page: page})
}

// --- SmartOverlay: deep entity coach/insights popover (Everywhere density) --

// smartOverlayProps carries the data for one entity's insight overlay.
type smartOverlayProps struct {
	// ID is the entity's stable identifier, used for data-testid and popover wrap id.
	ID string
	// Insights is the list of insights targeting this entity.
	Insights []smart.Insight
}

// smartOverlay renders a toggleable popover that surfaces all insights for one
// entity. It opens via a sparkle trigger button and closes via DismissPopover
// (Escape / outside click). Its own component so UseState + UseEvent hooks sit
// at a stable position — never inside a variable-length loop.
func smartOverlay(props smartOverlayProps) ui.Node {
	open := ui.UseState(false)
	wrapID := "smart-overlay-wrap-" + props.ID
	uiw.DismissPopover(open.Get(), wrapID, func() { open.Set(false) })
	toggle := ui.UseEvent(func() { open.Set(!open.Get()) })

	var pop ui.Node = Fragment()
	if open.Get() {
		pop = Div(ClassStr("smart-overlay-pop "+tw.Fold(tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
			Attr("role", "dialog"),
			Attr("aria-modal", "true"),
			Attr("data-testid", "smart-overlay-"+props.ID),
			Div(ClassStr(tw.Fold(tw.FontSemibold, tw.Text13, tw.Mb2)),
				smartGlyph(false, tw.Fold(tw.W3, tw.H3, tw.ShrinkO)),
				Span(ClassStr(tw.Fold(tw.Ml1)), uistate.T("smart.overlayTitle")),
			),
			smartInsightList(props.Insights),
		)
	}
	return Span(ClassStr("smart-overlay-wrap "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)),
		Attr("id", wrapID),
		Button(ClassStr("btn-icon-bare "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)),
			Type("button"),
			Attr("data-testid", "smart-overlay-trigger-"+props.ID),
			Attr("aria-label", uistate.T("smart.overlayLabel")),
			Attr("title", uistate.T("smart.overlayLabel")),
			OnClick(toggle),
			smartGlyph(false, tw.Fold(tw.W4, tw.H4)),
		),
		pop,
	)
}

// smartOverlayFor returns an entity coach overlay when:
// (a) density permits AffordanceOverlay (Everywhere only), (b) byEntity has
// at least one insight for the given relatedID, (c) every insight passes the
// ShowsAffordance gate (enabled + not muted). Returns nothing when any gate fails.
func smartOverlayFor(settings smart.Settings, byEntity map[string][]smart.Insight, relatedID string) ui.Node {
	if !settings.DensityOrDefault().Shows(smart.AffordanceOverlay) {
		return Fragment()
	}
	raw := byEntity[relatedID]
	var qualifying []smart.Insight
	for _, ins := range raw {
		if settings.ShowsAffordance(ins.Feature, smart.AffordanceOverlay) {
			qualifying = append(qualifying, ins)
		}
	}
	if len(qualifying) == 0 {
		return Fragment()
	}
	return ui.CreateElement(smartOverlay, smartOverlayProps{ID: relatedID, Insights: qualifying})
}

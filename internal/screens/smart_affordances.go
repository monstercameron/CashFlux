// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/smart"
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

	var pop ui.Node = Fragment()
	if open.Get() {
		pop = Div(ClassStr("smart-tip-pop "+tw.Fold(tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
			Attr("role", "tooltip"),
			Attr("data-testid", "smart-tip-pop"),
			Div(ClassStr(tw.Fold(tw.FontSemibold, tw.Text13)), props.Title),
			P(ClassStr(tw.Fold(tw.Text12, tw.TextDim, tw.Mt1)), props.Text),
		)
	}
	return Span(ClassStr("smart-tip-wrap "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)), Attr("id", wrapID),
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

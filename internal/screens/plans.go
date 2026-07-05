// SPDX-License-Identifier: MIT

//go:build js && wasm

// Plans is the canonical pricing/comparison surface (R31-plans). It shows the
// Free (local, always) vs Cloud (optional, paid) tiers with REAL prices, plain
// language, no dark patterns, and an easy path back to free. It replaces the
// one-shot UpgradeSheet as the durable discover-pricing destination.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Plans renders a Free vs Cloud comparison surface with honest pricing.
// It is the permanent, re-discoverable entry point for the upgrade decision
// (R31-reengage). No dark patterns: both prices shown, easy navigation away.
func Plans() ui.Node {
	priceAnnual := uistate.T("settings.cloudPriceAnnual")
	priceMonthly := uistate.T("settings.cloudPriceMonthly")
	return Div(css.Class("plans-page", tw.Flex, tw.FlexCol, tw.Gap4, tw.MxAuto, tw.Py2),
		Attr("aria-label", uistate.T("plans.pageTitle")),

		// Current-plan chip (R31-chip): surfaces free/local status prominently.
		Div(css.Class("plan-chip", tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.Px3, tw.Py1, tw.RoundedFull, tw.Text13, tw.FontMedium),
			Span(css.Class("plan-chip-dot")),
			uistate.T("plans.currentPlan"),
		),

		// Two-column (or stacked) comparison cards.
		Div(css.Class("plans-grid"),

			// ── Free card ──────────────────────────────────────────────
			planCard(
				uistate.T("plans.freeTitle"),
				uistate.T("plans.freeTagline"),
				uistate.T("plans.freePrice"),
				"",
				[]string{
					uistate.T("plans.freeFeature1"),
					uistate.T("plans.freeFeature2"),
					uistate.T("plans.freeFeature3"),
					uistate.T("plans.freeFeature4"),
				},
				Fragment(),
				false,
			),

			// ── Cloud card ─────────────────────────────────────────────
			planCard(
				uistate.T("plans.cloudTitle"),
				uistate.T("plans.cloudTagline"),
				priceAnnual,
				"or "+priceMonthly+" billed monthly",
				[]string{
					uistate.T("plans.cloudFeature1"),
					uistate.T("plans.cloudFeature2"),
					uistate.T("plans.cloudFeature3"),
					uistate.T("plans.cloudFeature4"),
				},
				// CTA: opens Settings (the Cloud tab holds Stripe Checkout, same as
				// UpgradeSheet). Settings is a modal, not a route — a /settings href
				// silently landed on the dashboard.
				Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
					Button(
						Type("button"),
						css.Class("btn btn-primary", tw.WFull, tw.TextCenter),
						OnClick(Prevent(func() { uistate.OpenGlobalSettings() })),
						uistate.T("plans.startTrial"),
					),
					P(css.Class(tw.Text12, tw.TextDim, tw.TextCenter), uistate.T("plans.cloudTrial")),
				),
				true,
			),
		),

		// Trust / cancellation note — shown below both cards, not gated.
		P(css.Class("plans-trust", tw.Text12, tw.TextFaint, tw.TextCenter, tw.Mx3),
			uistate.T("plans.cloudTrust"),
		),

		// Self-host note.
		P(css.Class("plans-selfhost", tw.Text12, tw.TextFaint, tw.TextCenter, tw.Mx3),
			uistate.T("plans.orSelfHost"),
		),
	)
}

// planCard renders one pricing tier card.
func planCard(title, tagline, primaryPrice, secondaryPrice string, features []string, cta ui.Node, highlighted bool) ui.Node {
	cardClass := "plan-card"
	if highlighted {
		cardClass = "plan-card plan-card--cloud"
	}
	featureNodes := []any{css.Class("plan-features", tw.Flex, tw.FlexCol, tw.Gap1, tw.My2)}
	for _, f := range features {
		featureNodes = append(featureNodes,
			Div(css.Class("plan-feature-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("plan-check"), "✓"),
				Span(css.Class(tw.Text13), f),
			),
		)
	}
	priceEl := []any{css.Class("plan-price", tw.FontSemibold, tw.Text18), primaryPrice}
	if secondaryPrice != "" {
		priceEl = append(priceEl, Span(css.Class(tw.Text12, tw.TextDim, tw.Block), secondaryPrice))
	}
	return Div(css.Class(cardClass),
		Attr("role", "region"),
		Attr("aria-label", title),
		P(css.Class("plan-title", tw.FontSemibold, tw.TextLg), title),
		P(css.Class("plan-tagline", tw.Text13, tw.TextDim, tw.Mt1), tagline),
		Div(append([]any{}, priceEl...)...),
		Div(featureNodes...),
		cta,
	)
}

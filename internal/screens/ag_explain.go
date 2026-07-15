// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: on the assistant chat host's mount, consume the seed and submit it
// as the opening user turn — a one-liner:
//
//	if seed, ok := uistate.ConsumeExplainSeed(); ok { submit(seed) }
//
// The Explain affordance below (ExplainChip) sets the seed and navigates here.
// This file adds NO tools; it is the front door for explain-anything (AG7).

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/explainseed"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// explainSeedFor builds the pre-seeded "explain this figure" prompt for a named
// engine variable, grounding it in the live derivation (engineenv.Explain over the
// live vars). Returns ok=false when the name isn't a known engine variable. The
// value formatter is formatFormulaValue, the same one the assistant's
// evaluate_formula tool uses, so the figure reads identically everywhere.
func explainSeedFor(app *appstate.App, varName string) (string, bool) {
	vars := liveEngineVars(app)
	d, ok := engineenv.Explain(varName, vars, app.Molecules())
	if !ok {
		return "", false
	}
	return explainseed.SeedText(d, func(v float64) string { return formatFormulaValue(v) }), true
}

// seedExplainAndNav stores the explain seed for varName and navigates to the
// assistant, where the chat host consumes it on mount. A no-op when the variable
// is unknown (so a mislabeled affordance never sends an empty prompt).
func seedExplainAndNav(app *appstate.App, varName string) {
	if seed, ok := explainSeedFor(app, varName); ok {
		uistate.SeedExplain(seed)
		router.Navigate(uistate.RoutePath("/assistant"))
	}
}

// ExplainChipProps configures an Explain affordance. VarName is the engine
// variable behind the figure (e.g. "net_worth", "health_score", a
// "budget_<slug>_remaining"); Label overrides the accessible label when the raw
// name isn't friendly.
type ExplainChipProps struct {
	VarName string
	Label   string
}

// ExplainChip renders the small "Explain" affordance for a KPI/figure surface
// (AG7): a kebab-free, single-tap button that opens the assistant pre-seeded with
// the figure's derivation. It is its own component so its click hook stays at a
// stable render position when dropped into a variable-length tile/figure list.
// Drop it beside any figure whose engine variable is known — dashboard KPI tiles,
// /health factors, budget numbers — passing that variable's name.
func ExplainChip(p ExplainChipProps) ui.Node { return ui.CreateElement(explainChipComp, p) }

func explainChipComp(p ExplainChipProps) ui.Node {
	app := appstate.Default
	label := uistate.T("explain.label")
	aria := label
	if name := explainLabel(p); name != "" {
		aria = uistate.T("explain.aria", name)
	}
	onClick := ui.UseEvent(func() {
		if app != nil {
			seedExplainAndNav(app, p.VarName)
		}
	})
	return Button(
		css.Class("explain-chip", tw.InlineFlex, tw.ItemsCenter, tw.Gap1),
		Type("button"),
		Attr("data-testid", "explain-chip"),
		Attr("aria-label", aria),
		Title(aria),
		OnClick(onClick),
		uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
		Span(label),
	)
}

// explainLabel returns the friendly figure name for the affordance's a11y label.
func explainLabel(p ExplainChipProps) string {
	if p.Label != "" {
		return p.Label
	}
	return explainseed.Label(p.VarName)
}

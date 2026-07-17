// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetengine"
	"github.com/monstercameron/CashFlux/internal/widgetregistry"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// kpiScope builds the engine evaluation context for a KPI tile: the numeric variable
// surface plus the non-numeric tokens a sub-label template can reference ("period",
// "base"). Money formats against the base currency.
func kpiScope(c widgetrender.RenderCtx) widgetengine.Scope {
	return widgetengine.Scope{
		Vars: c.Vars,
		Strs: map[string]string{"period": c.PeriodLabel, "base": c.Base},
		Base: c.Base,
	}
}

// kpiPresentation carries the per-tile presentation a declarative KPI spec does not
// encode in its data binding: its fixed grid cell, figure tone, and whether to use
// the larger hero figure. The DATA (formula/format/sub) is fully in the spec and
// hydrated by widgetengine; only this layout/tone styling is code. A Tone of "sign"
// colors by the value's sign (green ≥ 0, red < 0). See §7.7.
var kpiPresentation = map[string]struct {
	Col, Row, Tone string
	Big            bool
}{
	"kpi-networth":    {"1", "2", "sign", true},
	"kpi-income":      {"2", "2", "text-up", false},
	"kpi-spending":    {"3", "2", "text-down", false},
	"kpi-liabilities": {"4", "2", "text-down", false}, // debt reads in the same red as liability balances elsewhere (Accounts card), not a one-off amber
	"kpi-assets":      {"1", "3", "text-up", false},
	"kpi-safetospend": {"2", "3", "sign", false},
}

// kpiSubOverride supplies the genuinely contextual sub-labels a var template can't
// faithfully express: net worth's delta line (already computed with its FX-exclusion
// disclosure) and safe-to-spend's sign-driven copy. It returns the sub text, its
// tone, and whether an override applies; everything else uses the spec's templated
// sub. This is presentation layering, not data computation — the figure is still
// fully engine-hydrated from the spec.
func kpiSubOverride(id string, c widgetrender.RenderCtx, val float64) (text, tone string, ok bool) {
	switch id {
	case "kpi-networth":
		return c.NWSub, c.NWTone, true
	case "kpi-safetospend":
		if val < 0 {
			return uistate.T("dashboard.safeToSpendOver"), "text-dim", true
		}
		return uistate.T("dashboard.safeToSpendSub"), "text-dim", true
	}
	return "", "", false
}

// renderKPISpec is the generic Kind==KPI renderer: it hydrates the spec's scalar
// binding through the engine and paints the tile from the result — title from the
// registry, figure/sub from the engine, grid/tone from kpiPresentation — with NO
// per-widget Go body. This is the "spec → engine → hydrated widget" path; a
// user-edited formula/format in the tile settings overrides the seeded binding so
// the tile stays programmable.
func renderKPISpec(spec domain.WidgetSpec, c widgetrender.RenderCtx) ui.Node {
	expr, format, sub := "0", widgetspec.FormatCurrency, ""
	if spec.Scalar != nil {
		expr, format, sub = spec.Scalar.Expr, spec.Scalar.Format, spec.Scalar.Sub
	}
	if sch, ok := widgetcfg.SchemaFor(spec.ID); ok {
		if f, ok := sch.FieldByKey("formula"); ok {
			if s := f.Str(spec.Settings); s != "" {
				expr = s
			}
		}
		if f, ok := sch.FieldByKey("format"); ok {
			if s := f.Str(spec.Settings); s != "" {
				format = s
			}
		}
	}
	view, err := widgetengine.HydrateKPI(&domain.ScalarBind{Expr: expr, Format: format, Sub: sub}, kpiScope(c))
	fig := view.Text
	if err != nil {
		fig = "—"
	}
	// savings is a declarative KPI (scalar binding "savings_rate") but rendered as a
	// gauge rather than a plain figure tile — a presentation variant of Kind==KPI.
	if spec.ID == "savings" {
		return savingsRateWidget(view.Value, spec.Settings)
	}
	pres := kpiPresentation[spec.ID]
	tone := pres.Tone
	if tone == "sign" {
		tone = "text-up"
		if view.Value < 0 {
			tone = "text-down"
		}
	}
	subText, subTone := view.Sub, "text-dim"
	if t, tn, ok := kpiSubOverride(spec.ID, c, view.Value); ok {
		subText, subTone = t, tn
	}
	title := spec.Title
	if title == "" {
		if d, ok := widgetregistry.Get(spec.ID); ok {
			title = uistate.T(d.NameKey)
		}
	}
	var body ui.Node
	switch {
	case spec.ID == "kpi-networth" && c.HHNWSub != "":
		// Scoped net worth shows a muted "vs household total" second sub-label.
		body = Div(
			Div(ClassStr("fig t-figure-lg "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(tone)),
				Attr("data-countup", ""), fig),
			Div(ClassStr("t-caption "+tw.Fold(tw.Pt15, tw.Truncate)+" "+tw.ColorClass(subTone)), Attr("title", subText), subText),
			Div(ClassStr("t-caption "+tw.Fold(tw.Pt15, tw.Truncate)+" text-dim"), Attr("title", c.HHNWSub), c.HHNWSub),
		)
	case pres.Big:
		body = kpiBodyHero(fig, tone, subText, subTone)
	default:
		body = kpiBody(fig, tone, subText, subTone)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: spec.ID, Title: title, Draggable: !c.Preview, Resizable: !c.Preview, Preview: c.Preview,
		GridColumn: pres.Col, GridRow: pres.Row,
		BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
		Body:      body,
	})
}

// init registers every built-in dashboard widget body with the engine's render
// registry, keyed by its NativeID. The dashboard no longer keeps a local closure
// map — it builds a widgetrender.RenderCtx once per render and dispatches each
// validated placement through widgetrender.Render(spec.NativeID, ctx). Bodies read
// their inputs from the ctx rather than closing over surface locals, which is what
// makes them registry-dispatchable (docs/UNIFIED_WIDGET_API.md §6).
func init() {
	R := widgetrender.Register

	// The dashboard welcome/hero, now a widget: the greeting + headline figures +
	// quick actions, wrapped in the shared shell with ChromeHover so the card surface
	// and controls fade in only on hover. dashboardHero is its own component, so its
	// hooks stay at stable positions inside the tile body.
	R("hero", func(c widgetrender.RenderCtx) ui.Node {
		return uiw.Widget(uiw.WidgetProps{
			ID: "hero", Title: uistate.T("dashboard.heroTitle"), ChromeHover: true,
			Body: ui.CreateElement(dashboardHero),
		})
	})

	R("attention", func(c widgetrender.RenderCtx) ui.Node {
		return attentionWidget(c.App, c.Txns, c.Rates, c.Start, c.End, c.Dismissals, c.Spec.Settings, c.AttnCol, c.AttnRow)
	})

	// All six KPI tiles (net worth, income, spending, liabilities, assets, safe to
	// spend) are now declarative Kind==KPI specs: their scalar binding (formula +
	// format + templated sub-label) is seeded in widgetregistry, hydrated by
	// widgetengine over the engine variable surface, and painted by renderKPISpec —
	// the spec→engine→widget path, with NO per-widget Go closures here.

	// budgets, accounts, and trend are declarative Kind==List/Chart specs: their
	// Pipeline (seeded in widgetregistry) is resolved into a Frame by the engine and
	// painted by these registered FrameRenderers — the data path is engine-driven, the
	// bespoke visualization stays here.
	RF := widgetrender.RegisterFrame
	RF("budgets", budgetsFrame)
	RF("accounts", accountsFrame)
	RF("trend", trendFrame)

	// recent, bills, cashflow, and breakdown are declarative Kind==List/Chart specs
	// hydrated by the engine into Frames; savings is a declarative Kind==KPI
	// (savings_rate) rendered as a gauge by renderKPISpec.
	RF("recent", recentFrame)
	RF("bills", billsFrame)
	RF("cashflow", cashFlowFrame)
	RF("breakdown", breakdownFrame)

	R("goals", func(c widgetrender.RenderCtx) ui.Node { return goalsWidget(c.App, c.Spec.Settings) })
	R("goal-states", func(c widgetrender.RenderCtx) ui.Node { return goalStatesWidget(c.App) })
	R("todo", func(c widgetrender.RenderCtx) ui.Node { return todoWidget(c.App, c.Spec.Settings) })
	R("health", func(c widgetrender.RenderCtx) ui.Node { return ui.CreateElement(healthWidgetNode, struct{}{}) })
	R("freshness", func(c widgetrender.RenderCtx) ui.Node {
		return freshnessWidget(c.Accounts, c.App.FreshnessWindows(), c.Dismissals, c.RemindToUpdate, c.DismissFreshness)
	})
	R("highlight", func(c widgetrender.RenderCtx) ui.Node {
		return topHighlightWidget(c.ScopedTxns, c.App.Categories(), c.Rates)
	})
	R("monthly-recap", monthlyRecapWidget)
	// 30/60/90-day available-cash forecast from per-account recurring projections.
	R("forecast", forecastWidget)
	R("smart-digest", func(c widgetrender.RenderCtx) ui.Node { return smartDigestWidget(c.App) })
	R("anomaly-hub", func(c widgetrender.RenderCtx) ui.Node { return ui.CreateElement(anomalyHubWidget, c.App) })
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attention"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetengine"
	"github.com/monstercameron/CashFlux/internal/widgetregistry"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// engineVarsTTL bounds how long a memoized engine variable surface is reused when
// its key (data revision · period · scope) is unchanged. A data edit bumps the
// revision and invalidates immediately; the TTL only caps staleness of the few
// wall-clock-relative figures (bills_due/goal_needs), which don't move minute to
// minute. 60s is a sensible balance between freshness and avoiding recompute on
// every re-render.
const engineVarsTTL = 60 * time.Second

// engineVarsMemo caches the last computed variable surface (single dashboard
// surface; wasm is single-threaded, so no lock is needed).
var engineVarsMemo struct {
	key  string
	at   time.Time
	vars map[string]float64
}

// memoEngineVars returns the cached surface when the key matches and the TTL has
// not elapsed, otherwise computes, caches, and returns a fresh one.
func memoEngineVars(key string, compute func() map[string]float64) map[string]float64 {
	now := time.Now()
	if engineVarsMemo.vars != nil && engineVarsMemo.key == key && now.Sub(engineVarsMemo.at) < engineVarsTTL {
		return engineVarsMemo.vars
	}
	v := compute()
	engineVarsMemo.key, engineVarsMemo.at, engineVarsMemo.vars = key, now, v
	return v
}

// safeRender dispatches a widget through the render registry inside a recover, so a
// panic in one tile (a nil deref, a divide-by-zero, a bad slice index in its body)
// renders a contained "unavailable" tile instead of taking down the whole
// dashboard. This is the per-widget error boundary that keeps the engine stable
// across all widgets (§13.3). The fallback reuses the standard tile shell so the
// grid stays intact.
func safeRender(id string, ctx widgetrender.RenderCtx) (node ui.Node, ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("dashboard widget render panicked", "id", id, "recover", rec)
			node, ok = uiw.Widget(uiw.WidgetProps{
				ID: id, Title: id,
				Body: P(css.Class("empty", tw.TextDim), uistate.T("dashboard.widgetLoadFailed")),
			}), true
		}
	}()
	return widgetrender.Render(id, ctx)
}

// safeRenderSpec dispatches a placement's spec by Kind, inside the same per-widget
// error boundary as safeRender. A declarative Kind==KPI spec is hydrated through the
// engine and painted generically (renderKPISpec) with no per-widget Go body; every
// other kind falls through to the Native render registry keyed by NativeID. This is
// the seam that lets fully-composable widgets ("spec → engine → widget") and the
// irreducible Native tiles coexist on one surface.
func safeRenderSpec(spec domain.WidgetSpec, ctx widgetrender.RenderCtx) (node ui.Node, ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("dashboard widget render panicked", "id", spec.ID, "recover", rec)
			node, ok = uiw.Widget(uiw.WidgetProps{
				ID: spec.ID, Title: spec.ID,
				Body: P(css.Class("empty", tw.TextDim), uistate.T("dashboard.widgetLoadFailed")),
			}), true
		}
	}()
	// A custom intra-tile content layout (compound widget: hand-placed text/figure/
	// icon/divider/dataview blocks) is rendered by the content-layout engine,
	// regardless of Kind (§7.5).
	if spec.Content.Mode == domain.LayoutCustom {
		return renderCustomContent(spec, ctx), true
	}
	switch spec.Kind {
	case domain.KindKPI:
		return renderKPISpec(spec, ctx), true
	case domain.KindList, domain.KindTable, domain.KindChart:
		return hydrateAndRenderFrame(spec, ctx)
	case domain.KindSpacer:
		return uiw.Widget(uiw.WidgetProps{ID: spec.ID, ChromeHover: true, Preview: ctx.Preview, Body: Fragment()}), true
	}
	return widgetrender.Render(spec.NativeID, ctx)
}

// frameDataCtx builds the engine DataCtx for a spec's Pipeline over the SCOPED data
// (member/institution scope), so collection/chart widgets and embedded data-view
// blocks show the same figures as the scoped KPI tiles.
func frameDataCtx(ctx widgetrender.RenderCtx) widgetengine.DataCtx {
	return widgetengine.DataCtx{
		Vars: ctx.Vars, Base: ctx.Base,
		Accounts:     ctx.ScopedAccounts,
		Transactions: ctx.ScopedTxns,
		Budgets:      ctx.App.Budgets(),
		Categories:   ctx.App.Categories(),
		Recurring:    ctx.App.Recurring(),
		Rates:        ctx.Rates,
		Start:        ctx.Start, End: ctx.End, Now: time.Now(),
		// The per-month variable surface behind "formula" series charts —
		// scoped like the KPI tiles, re-windowed per month.
		MonthVars: func(s, e time.Time) map[string]float64 {
			return engineenv.Vars(engineenv.Data{
				Accounts: ctx.ScopedAccounts, Transactions: ctx.ScopedTxns,
				Members: ctx.App.Members(), Budgets: ctx.App.Budgets(), Goals: ctx.App.Goals(), Tasks: ctx.App.Tasks(),
				Recurring: ctx.App.Recurring(), Categories: ctx.App.Categories(), Rates: ctx.Rates,
				Now: time.Now(), PeriodStart: s, PeriodEnd: e,
				CustomDefs: ctx.App.CustomFieldDefs(), Molecules: ctx.App.Molecules(),
			})
		},
	}
}

// hydrateAndRenderFrame resolves a List/Table/Chart spec's Pipeline into a Frame
// through the engine, then dispatches to the widget's registered FrameRenderer. The
// effective pipeline merges the spec's declared Source with the tile's user settings
// (limit, at-risk, cleared, window) so the data path is engine-driven end to end.
func hydrateAndRenderFrame(spec domain.WidgetSpec, ctx widgetrender.RenderCtx) (ui.Node, bool) {
	pipe := effectivePipeline(spec, ctx)
	frame, err := widgetengine.HydrateFrame(&pipe, frameDataCtx(ctx))
	if err != nil {
		slog.Warn("dashboard: hydrate frame failed", "id", spec.ID, "err", err)
		return nil, false
	}
	// Built-in tiles have a bespoke FrameRenderer keyed by id; a Studio-designed
	// widget has none, so it falls back to the generic renderer that paints any Frame
	// (list rows / chart) straight from its typed columns — no per-id code.
	if node, ok := widgetrender.RenderFrame(spec.ID, frame, ctx); ok {
		return node, true
	}
	return renderGenericFrame(spec, frame, ctx), true
}

// renderGenericFrame paints any Frame for a Studio-designed List/Table/Chart widget,
// driven entirely by the Frame's typed columns (no hardcoded per-widget layout): a
// chart projects an x/value series; a list shows a label/value row per record.
func renderGenericFrame(spec domain.WidgetSpec, fr domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
	title := spec.Title
	if title == "" {
		title = "Widget"
	}
	if spec.Kind == domain.KindChart {
		return uiw.Widget(uiw.WidgetProps{
			ID: spec.ID, Title: title, Draggable: !ctx.Preview, Resizable: !ctx.Preview, Preview: ctx.Preview,
			BodyClass: tw.Fold(tw.Flex, tw.FlexCol, tw.MinH0),
			Body:      genericChartBody(fr, ctx.Base),
		})
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: spec.ID, Title: title, Draggable: !ctx.Preview, Resizable: !ctx.Preview, Preview: ctx.Preview,
		BodyClass: tw.Fold(tw.OverflowHidden, tw.Flex, tw.FlexCol, tw.MinH0),
		Body:      ui.CreateElement(genericListWidget, genericListProps{Spec: spec, Frame: fr, Base: ctx.Base}),
	})
}

// frameLabelCol / frameValueCol pick sensible columns for generic rendering: the
// first string column is the label; the value prefers money, then percent, then a
// number column (the last such column, so a series' "value" beats its "t").
func frameLabelCol(fr domain.Frame) (domain.Field, bool) {
	for _, f := range fr.Fields {
		if f.Type == domain.FieldString {
			return f, true
		}
	}
	return domain.Field{}, false
}

func frameValueCol(fr domain.Frame) (domain.Field, bool) {
	var out domain.Field
	found := false
	for _, f := range fr.Fields {
		switch f.Type {
		case domain.FieldMoney, domain.FieldPercent, domain.FieldNumber:
			out, found = f, true
		}
	}
	return out, found
}

// genericListBody renders a Frame as label/value rows.
func genericListBody(fr domain.Frame, base string) ui.Node {
	if fr.Rows == 0 {
		return P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noDataYet"))
	}
	labelCol, hasLabel := frameLabelCol(fr)
	valCol, _ := frameValueCol(fr)
	rows := make([]ui.Node, 0, fr.Rows)
	for i := 0; i < fr.Rows; i++ {
		label := fmt.Sprintf("%d", i+1)
		if hasLabel {
			label = labelCol.Str(i)
		}
		rows = append(rows, Div(css.Class("t-body", tw.Flex, tw.JustifyBetween, tw.Py25, tw.BorderB, tw.BorderLine70),
			Span(css.Class(tw.TextDim), label),
			Span(css.Class("fig", tw.FontDisplay), dataViewValue(valCol, i, base)),
		))
	}
	return Div(css.Class("t-body"), rows)
}

// genericChartBody projects a Frame into an area chart (time series: a "t" column +
// a value) or a bar chart (categorical: a label column + a value). Falls back to a
// list when neither shape is present.
func genericChartBody(fr domain.Frame, base string) ui.Node {
	if fr.Rows == 0 {
		return P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noDataYet"))
	}
	valCol, hasVal := frameValueCol(fr)
	if !hasVal {
		return genericListBody(fr, base)
	}
	div := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		div *= 10
	}
	scale := func(v float64) float64 {
		switch valCol.Type {
		case domain.FieldMoney:
			return v / div
		case domain.FieldPercent:
			// Percent columns carry 0–100 (the convention every source and the
			// row formatter use); the axis's d3 "~%" format expects a fraction —
			// without this a 60% savings rate charted as "6000%".
			return v / 100
		}
		return v
	}
	tCol, isSeries := fr.Column("t")
	pts := make([]chartspec.Point, fr.Rows)
	kind := chartspec.Bar
	if isSeries {
		kind = chartspec.Area
		for i := 0; i < fr.Rows; i++ {
			// .UTC(): series timestamps are UTC calendar boundaries; local-zone
			// reconstruction shifts a month-start label to the previous month
			// west of UTC (C339).
			pts[i] = chartspec.Point{X: float64(i), Y: scale(valCol.Num(i)), Label: time.Unix(int64(tCol.Num(i)), 0).UTC().Format("Jan '06")}
		}
	} else {
		labelCol, hasLabel := frameLabelCol(fr)
		for i := 0; i < fr.Rows; i++ {
			lbl := fmt.Sprintf("%d", i+1)
			if hasLabel {
				lbl = labelCol.Str(i)
			}
			pts[i] = chartspec.Point{X: float64(i), Y: scale(valCol.Num(i)), Label: lbl}
		}
	}
	yFmt := ".3~s"
	if valCol.Type == domain.FieldMoney && currency.Symbol(base) == "$" {
		yFmt = "$.3~s"
	} else if valCol.Type == domain.FieldPercent {
		yFmt = "~%"
	}
	spec := chartspec.Spec{
		Kind:   kind,
		Series: []chartspec.Series{{Name: uistate.T("dashboard.seriesValue"), Points: pts}},
		Y:      chartspec.Axis{Format: yFmt},
	}
	return uiw.Chart(uiw.ChartProps{Spec: spec, Height: "100%", Class: "trend-chart"})
}

// userSpecPrefix namespaces a Studio-designed widget's id (and its dashboard-layout
// Item.ID) so the dashboard renders it from its persisted WidgetSpec rather than the
// built-in registry. Distinct from vbCardPrefix ("wb:", cardgraph cards).
const userSpecPrefix = "us:"

// hydrateStoredSpec overlays a persisted spec's user-customizable parts onto the
// registry default so saved widget configs act as hydration: a custom content layout,
// the tile Style, and an overridden Title win when stored. Structural bindings
// (Kind / Scalar / Pipeline / NativeID) stay from the default so built-ins keep
// evolving with the code; only the configurable surface is restored from storage.
func hydrateStoredSpec(def, stored domain.WidgetSpec) domain.WidgetSpec {
	if stored.Content.Mode == domain.LayoutCustom && len(stored.Content.Blocks) > 0 {
		def.Content = stored.Content
	}
	if !stored.Style.Empty() {
		def.Style = stored.Style
	}
	if stored.Title != "" {
		def.Title = stored.Title
	}
	return def
}

// renderCustomContent is the intra-tile content-layout engine (§7.5): it composes a
// compound widget body from the spec's hand-placed Blocks (text / figure / icon /
// divider / spacer / dataview), each hydrated by widgetengine and styled by the
// per-block Style merged over the tile Style. Blocks stack in order in a wrapping
// flex row; a Block.ColSpan (1–4 of an internal 4-col content grid) sets its width
// so figures can sit side by side. The tile carries the spec's token-first Style.
func renderCustomContent(spec domain.WidgetSpec, ctx widgetrender.RenderCtx) ui.Node {
	sc := kpiScope(ctx)
	blocks := make([]ui.Node, 0, len(spec.Content.Blocks))
	for _, b := range spec.Content.Blocks {
		blocks = append(blocks, renderBlock(spec, b, ctx, sc))
	}
	title := spec.Title
	if title == "" {
		if d, ok := widgetregistry.Get(spec.ID); ok {
			title = uistate.T(d.NameKey)
		}
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: spec.ID, Title: title, Draggable: !ctx.Preview, Resizable: !ctx.Preview, Preview: ctx.Preview,
		Style: spec.Style.CSS(),
		// Content model: a 4-column CSS grid (NOT the surface packer) — blocks stack in
		// order at intrinsic height, each spanning Block.ColSpan columns so figures can
		// sit side by side. Grid gap is subtracted from track widths, so spans never
		// overflow the row (a flex basis + gap would wrap; §7.5).
		Body: Div(Style(map[string]string{
			"display":               "grid",
			"grid-template-columns": "repeat(4, minmax(0, 1fr))",
			"gap":                   "4px 10px",
			"align-items":           "baseline",
		}), blocks),
	})
}

// blockWidthStyle sets a block's column span on the internal 4-column content grid
// (ColSpan 0 or ≥4 → full row); min-width:0 lets long text truncate inside the cell.
func blockWidthStyle(colSpan int) map[string]string {
	span := colSpan
	if span <= 0 || span > 4 {
		span = 4
	}
	return map[string]string{"grid-column": fmt.Sprintf("span %d", span), "min-width": "0"}
}

// blockTypographyCSS applies only color / font-weight / text-align to a block,
// inheriting the tile Style per-property and overriding with the block's own. The
// tile owns background/border/accent (§7.5), so those are never emitted per-block.
func blockTypographyCSS(tile, block domain.Style) map[string]string {
	out := map[string]string{}
	pick := func(b, t string) string {
		if b != "" {
			return b
		}
		return t
	}
	if v := pick(block.Text, tile.Text); v != "" {
		out["color"] = v
	}
	if v := pick(block.FontWeight, tile.FontWeight); v != "" {
		out["font-weight"] = v
	}
	if v := pick(block.Align, tile.Align); v != "" {
		out["text-align"] = v
	}
	return out
}

// renderBlock renders one content Block to a node, applying its width + typography.
func renderBlock(spec domain.WidgetSpec, b domain.Block, ctx widgetrender.RenderCtx, sc widgetengine.Scope) ui.Node {
	style := blockWidthStyle(b.ColSpan)
	for k, v := range blockTypographyCSS(spec.Style, b.Style) {
		style[k] = v
	}
	switch b.Kind {
	case domain.BlockText:
		return Div(css.Class("t-body"), Style(style), Text(widgetengine.HydrateBlock(b, sc)))
	case domain.BlockFigure:
		return Div(ClassStr("fig t-figure "+tw.Fold(tw.FontDisplay, tw.LeadingTight)), Style(style), Text(widgetengine.HydrateBlock(b, sc)))
	case domain.BlockIcon:
		name := b.Bind
		if name == "" {
			name = b.Text
		}
		return Div(Style(style), uiw.Icon(icon.Name(name), css.Class(tw.W5, tw.H5)))
	case domain.BlockDivider:
		return Div(Style(map[string]string{"grid-column": "1 / -1", "height": "1px", "margin": "6px 0", "background": "var(--line)"}))
	case domain.BlockSpacer:
		return Div(Style(map[string]string{"grid-column": fmt.Sprintf("span %d", max(b.ColSpan, 1))}))
	case domain.BlockDataView:
		return renderBlockDataView(spec, ctx, style)
	}
	return Fragment()
}

// renderBlockDataView embeds the widget's own Pipeline Frame as a compact label/value
// list inside a custom layout (the "table beneath a caption" case, §7.5). Renders the
// first column as the label and the last as the value, up to six rows.
func renderBlockDataView(spec domain.WidgetSpec, ctx widgetrender.RenderCtx, style map[string]string) ui.Node {
	style["grid-column"] = "1 / -1"
	if spec.Pipeline == nil {
		return Div(Style(style))
	}
	fr, err := widgetengine.HydrateFrame(spec.Pipeline, frameDataCtx(ctx))
	if err != nil || fr.Rows == 0 || len(fr.Fields) == 0 {
		return Div(css.Class("t-caption", tw.TextDim), Style(style), "No data yet.")
	}
	labelCol := fr.Fields[0]
	valCol := fr.Fields[len(fr.Fields)-1]
	n := fr.Rows
	if n > 6 {
		n = 6
	}
	rows := make([]ui.Node, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, Div(css.Class("t-body", tw.Flex, tw.JustifyBetween),
			Span(css.Class(tw.TextDim), labelCol.Str(i)),
			Span(css.Class("fig", tw.FontDisplay), dataViewValue(valCol, i, ctx.Base)),
		))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Style(style), rows)
}

// dataViewValue formats a frame cell for a data-view row by column type.
func dataViewValue(f domain.Field, i int, base string) string {
	switch f.Type {
	case domain.FieldMoney:
		return fmtMoney(money.New(f.Int64(i), base))
	case domain.FieldPercent:
		return fmt.Sprintf("%d%%", int(f.Num(i)))
	case domain.FieldNumber:
		return fmt.Sprintf("%g", f.Num(i))
	}
	return f.Str(i)
}

// effectivePipeline merges a spec's declared Pipeline with the tile's user settings,
// turning settings into Source parameters (cleared, window) and Transforms (at-risk
// filter, row limit) so a user's configuration drives the engine query rather than
// being applied ad hoc in the renderer.
func effectivePipeline(spec domain.WidgetSpec, ctx widgetrender.RenderCtx) domain.Pipeline {
	var p domain.Pipeline
	if spec.Pipeline != nil {
		p = *spec.Pipeline
	}
	sch, ok := widgetcfg.SchemaFor(spec.ID)
	if !ok {
		return p
	}
	switch spec.ID {
	case "budgets":
		limit, atRisk := 6, false
		if f, ok := sch.FieldByKey("count"); ok {
			limit = f.Int(spec.Settings)
		}
		if f, ok := sch.FieldByKey("atRisk"); ok {
			atRisk = f.Bool(spec.Settings)
		}
		if atRisk {
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformFilter, Arg: "atrisk"})
		}
		// In scroll/page mode, fetch a generous set so the tile can scroll/page through
		// more than the cap; "cap" keeps the top N.
		limit = listFetchLimit(spec, "scroll", limit, 30)
		if limit > 0 {
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: limit})
		}
	case "accounts":
		limit := 6
		if f, ok := sch.FieldByKey("count"); ok {
			limit = f.Int(spec.Settings)
		}
		if f, ok := sch.FieldByKey("cleared"); ok {
			p.Source.Cleared = f.Bool(spec.Settings)
		}
		limit = listFetchLimit(spec, "scroll", limit, 30)
		if limit > 0 {
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: limit})
		}
	case "trend":
		months := 6
		if f, ok := sch.FieldByKey("months"); ok {
			months = f.Int(spec.Settings)
		}
		p.Source.Series.Months = months
	case "recent":
		limit := 6
		if f, ok := sch.FieldByKey("count"); ok {
			limit = f.Int(spec.Settings)
		}
		// Recent defaults to "page": fetch a generous set so there are several pages
		// (or a long scroll); "cap" keeps just the count.
		limit = listFetchLimit(spec, "page", limit, 60)
		if limit > 0 {
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: limit})
		}
	case "bills":
		// Default "scroll": show the upcoming run, scrollable; "cap" keeps the next four.
		p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: listFetchLimit(spec, "scroll", 4, 16)})
	}
	return p
}

// listFetchLimit decides how many rows to fetch for a list tile given its display
// behavior. In "cap" mode it returns capN (the top-N the user sees). In "scroll"
// or "page" mode it returns the larger fetchN so the tile has enough rows to
// scroll through or page across. def is the tile's default display mode.
func listFetchLimit(spec domain.WidgetSpec, def string, capN, fetchN int) int {
	if widgetDisplay(spec, def) == "cap" {
		return capN
	}
	return fetchN
}

// bentoFlipDriver is a zero-output component that owns the bento FLIP reflow trigger
// and the drag-preview subscription, deliberately split out of Dashboard(). Dashboard
// recomputes every widget's data frame on each render; if it subscribed to the live
// drag-preview atom (as it used to via flipSig), every dragover re-ran all of that —
// over a large ledger that pinned the main thread for hundreds of ms and made the drag
// feel frozen. This component subscribes to the layout + drag atoms instead and renders
// nothing, so a dragover re-renders only the cheap per-tile shells and this driver. It
// runs uiw.FlipBento() on any arrangement change (drag preview, drop, resize, auto-mode)
// and registers the render-captured drag-atom reset the coordinator's safety net calls.
func bentoFlipDriver() ui.Node {
	layoutItems := uistate.UseLayoutItems().Get()
	layoutMode := uistate.UseLayoutMode().Get()
	flipSig := string(layoutMode)
	for _, it := range layoutItems {
		flipSig += fmt.Sprintf("|%s:%dx%d:%d", it.ID, it.ColSpan, it.RowSpan, it.Importance)
	}
	// Keyed on the layout signature only — NOT on any drag atom. The drag is now
	// coordinator/DOM-driven (no atom writes mid-drag), so the FLIP fires on real
	// arrangement changes (a drop reorder, resize, or auto-mode switch), which is
	// exactly when tiles actually move.
	ui.UseEffect(func() func() {
		uiw.FlipBento()
		return nil
	}, flipSig)
	return Fragment()
}

// Dashboard shows headline metrics in the candidate-C bento grid, driven by the
// live store and the shared time-resolution window.
func Dashboard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Div(css.Class("bento"), Div(css.Class("w"), Div(css.Class("wbody"), P(css.Class("empty"), uistate.T("common.notReady")))))
	}
	_ = uistate.UseDataRevision().Get() // re-render after import / load-sample / wipe

	// The drag-preview FLIP trigger + drag-atom reads live in a separate tiny
	// component (bentoFlipDriver), NOT here. Dashboard() recomputes every widget's
	// frame over the full dataset, so subscribing it to the live drag-preview atom
	// re-ran all of that on every dragover — over a large ledger that blocked the main
	// thread for hundreds of ms and made dragging feel frozen. The driver subscribes
	// to the drag atoms instead and renders nothing, so a dragover re-renders only the
	// cheap tile shells + the driver, never the data frames. (B2 / perf)
	layoutItems := uistate.UseLayoutItems().Get()

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// MIA-extend (#445-8): apply the active report scope to accounts and
	// transactions so KPI tiles, spending totals, and the net-worth tile all
	// reflect the user's chosen scope (member / institution / type / account).
	// UseActiveScope is a hook (state.UseAtom) and occupies the same stable slot
	// as the former UseActiveMember call it replaces.
	sc := uistate.UseActiveScope().Get()
	instOf := func(a domain.Account) string { return a.Institution }
	ids := scope.ResolveScope(accounts, sc, instOf)
	kpiTxns := scope.ApplyScopeToTxns(txns, ids)
	scopedAccounts := scope.ApplyScopeToAccounts(accounts, ids)
	scopeSig := fmt.Sprintf("%v|%v|%v|%v", sc.Owners, sc.Institutions, sc.Types, sc.AccountIDs)

	// Memoized via state.UseComputed keyed on app.Rev() — recomputes only when the
	// dataset/FX actually changes, not on every re-render (§1.6).
	nw := useNetWorth(app, scopedAccounts, txns, rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities
	w := uistate.UsePeriod().Get()
	widgetCfgs := uistate.UseWidgetConfigs().Get()
	start, end := w.Range()
	// Memoized (§1.6): keyed on app.Rev() + the period + the full scope sig,
	// so it recomputes only when one of those changes.
	income, expense := usePeriodTotals(app, kpiTxns, start, end, rates, scopeSig)

	// W-15: trigger count-up animation on the KPI hero figures whenever the
	// underlying values change. The sig is keyed on the four headline amounts so
	// the effect fires exactly on mount and on genuine data changes — not on every
	// re-render that leaves the numbers unchanged. cashfluxCountUpScan (countup.js)
	// tracks per-element last-animated values so it skips elements whose text
	// hasn't changed and always restores the exact original string at end-of-tween.
	kpiSig := fmt.Sprintf("%d|%d|%d|%d", net.Amount, income.Amount, expense.Amount, liabilities.Amount)
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, kpiSig)

	// Cash flow = income − spending for the period (G1 §7): the surplus/deficit Elena
	// wants in one line. Shown as a signed sub-line on the Income tile so "what
	// changed?" is answerable above the fold without mental arithmetic.
	cashFlow := money.New(income.Amount-expense.Amount, income.Currency)
	cashFlowSub := "cash flow −" + fmtMoney(money.New(-cashFlow.Amount, income.Currency))
	if cashFlow.Amount >= 0 {
		cashFlowSub = "cash flow +" + fmtMoney(cashFlow)
	}
	periodLabel := w.FromLabel()
	if w.ToLabel() != w.FromLabel() {
		periodLabel += " – " + w.ToLabel()
	}

	incCount, expCount := 0, 0
	for _, t := range kpiTxns {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			incCount++
		case t.IsExpense():
			expCount++
		}
	}

	// scopedAccounts is already non-archived (ResolveScope excludes archived IDs).
	active := len(scopedAccounts)

	// "Remind me" on the freshness nudge → create a to-do and jump to the list.
	nav := router.UseNavigate()
	noticeAtom := uistate.UseNotice()
	freshnessDismissals := uistate.UseFreshnessDismissals()
	remindToUpdate := ui.UseEvent(func() {
		if _, err := app.CreateFreshnessReminderTask(uistate.T("dashboard.staleTaskTitle")); err != nil {
			noticeAtom.Set(noticeAtom.Get().With(uistate.T("dashboard.reminderErr", err.Error()), true))
			return
		}
		nav.Navigate(uistate.RoutePath("/todo"))
	})
	dismissFreshness := ui.UseEvent(func() {
		stale := freshness.StaleAccounts(accounts, app.FreshnessWindows(), time.Now())
		next := freshnessDismissals.Get().Dismiss(stale, time.Now())
		freshnessDismissals.Set(next)
		uistate.PersistFreshnessDismissals(next)
	})

	// Net-worth change since the start of this month (end of last month).
	// MIA-extend (#445-8): use scopedAccounts so the scoped NW drives the tile.
	nwSub, nwTone := uistate.T("dashboard.assetsSub", fmtMoney(assets)), "text-dim"
	if prev, _ := ledger.NetWorthSeries(scopedAccounts, txns, []time.Time{dateutil.MonthStart(time.Now())}, rates); len(prev) == 1 {
		if d, ok := ledger.PercentChange(net.Amount, prev[0].Amount); ok {
			// A flat 0% reads as "nothing moved" rather than "income == spending";
			// say so plainly with the absolute delta instead of a misleading "▲ 0%"
			// (G1 §7).
			delta := money.New(net.Amount-prev[0].Amount, net.Currency)
			switch {
			case d < 0:
				nwTone, nwSub = "text-down", fmt.Sprintf("▼ %d%% (%s) this month", -d, fmtMoney(delta))
			case d > 0:
				nwTone, nwSub = "text-up", fmt.Sprintf("▲ %d%% (+%s) this month", d, fmtMoney(delta))
			case delta.Amount != 0:
				nwTone, nwSub = "text-dim", fmt.Sprintf("%s this month", fmtMoney(delta))
			default:
				nwTone, nwSub = "text-dim", "No change this month"
			}
		}
	}
	// A missing FX rate excludes accounts from the total (L4) — say so on the tile,
	// rather than letting net worth silently collapse.
	if len(nw.MissingCurrencies) > 0 {
		nwTone = "text-down"
		nwSub = "excludes " + plural(len(nw.ExcludedAccounts), "account") + " — no " + strings.Join(nw.MissingCurrencies, ", ") + " rate"
	} else {
		// C82: when the total folds in non-base-currency accounts, disclose that a
		// conversion happened so the figure isn't read as a raw same-currency sum.
		// MIA-extend (#445-8): check scopedAccounts for the FX-disclosure signal.
		for _, ac := range scopedAccounts {
			if ac.Currency != "" && ac.Currency != base {
				nwSub += " · " + uistate.T("dashboard.netWorthConverted", base)
				break
			}
		}
	}

	// MIA-extend (#445-8): when a scope is active, show "vs household total: $X"
	// as a muted second sub-label on the net-worth tile so the user can compare
	// their scoped view to the full household at a glance.
	// Use ledger.NetWorthExplained directly (not via the hook) to avoid
	// registering an extra state.UseComputed slot.
	hhNWSub := ""
	if !sc.IsAll() {
		if hhResult, err := ledger.NetWorthExplained(accounts, txns, rates); err == nil {
			hhNWSub = uistate.T("dashboard.householdTotal", fmtMoney(hhResult.Net))
		}
	}

	attnCol, attnRow := spanOf(layoutItems, "attention")

	// The engine variable surface: atoms (scoped accounts/transactions over the
	// active period + recurring/goals/custom fields) composed into molecules via the
	// DB-stored formulas. KPI tiles evaluate their configurable formulas against
	// this, so every figure is derived from a fundamental source and FX/period/scope
	// consistent. Memoized with a TTL keyed on (data revision, period, scope): a data
	// edit (rev change) or a period/scope switch recomputes immediately; otherwise
	// repeated renders within the TTL reuse the cached surface (the figures depend on
	// the wall clock only through bills_due/goal_needs, which don't move minute to
	// minute).
	varsKey := fmt.Sprintf("%d|%v|%s", app.Rev(), w, scopeSig)
	vars := memoEngineVars(varsKey, func() map[string]float64 {
		return engineenv.Vars(engineenv.Data{
			Accounts: scopedAccounts, Transactions: kpiTxns,
			Members: app.Members(), Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(),
			Recurring: app.Recurring(), Rates: rates, Now: time.Now(),
			PeriodStart: start, PeriodEnd: end,
			CustomDefs: app.CustomFieldDefs(),
			Molecules:  app.Molecules(),
		})
	})

	// Build the render context once: the live data + callbacks every tile needs.
	// Each tile body is a renderer registered with the engine (internal/widgetrender,
	// seeded in dashboard_widgets.go) and dispatched by NativeID in the loop below —
	// there is no local closure map. Bodies read from rctx, not surface locals (§6).
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: scopedAccounts, ScopedTxns: kpiTxns,
		Rates: rates, Base: base,
		Start: start, End: end,
		Net: net, Assets: assets, Liabilities: liabilities, Income: income, Expense: expense,
		PeriodLabel: periodLabel, CashFlowSub: cashFlowSub,
		IncCount: incCount, ExpCount: expCount, ActiveAccounts: active,
		NWSub: nwSub, NWTone: nwTone, HHNWSub: hhNWSub,
		AttnCol: attnCol, AttnRow: attnRow,
		Vars:           vars,
		Dismissals:     freshnessDismissals.Get(),
		RemindToUpdate: remindToUpdate, DismissFreshness: dismissFreshness,
	}

	hidden := uistate.UseHiddenWidgets().Get()
	tiles := make([]any, 0, len(layoutItems)+1)
	// no-touch-chrome: lets the CSS agent hide drag/resize affordances (.grip, .rz
	// buttons) under @media (hover:none) so they don't show on touch screens (L33).
	tiles = append(tiles, css.Class("bento no-touch-chrome"))
	// Renders nothing; owns the FLIP reflow trigger (keyed on layout, not drag state).
	tiles = append(tiles, ui.CreateElement(bentoFlipDriver))
	// The dashboard surface's full placement set (built-ins + hidden + their saved
	// settings/style), persisted to SQLite as first-class rows below so the layout
	// travels with export and is the engine's canonical representation (§8).
	// Hydration source: the persisted placement specs for this surface. A saved tile's
	// user-customizable parts (custom content layout, tile style, title) overlay the
	// registry default so stored config DRIVES rendering — the default is only the
	// fallback for not-yet-customized tiles (same defaults-merge pattern as molecules).
	stored := map[string]domain.WidgetSpec{}
	if app != nil {
		for _, p := range app.Placements("dashboard") {
			stored[p.ID] = p.Spec
		}
	}
	placements := make([]domain.Placement, 0, len(layoutItems))
	for _, it := range layoutItems {
		// Each built-in tile is a validated domain.Placement: its spec comes from the
		// widget registry (the single source of truth) and carries this instance's
		// saved settings/style (widgetcfg, incl. the reserved style keys); its body is
		// dispatched through the render registry by NativeID. A "wb:" id is a
		// user-published Widget Builder card, rendered from its saved graph.
		if spec, ok := widgetregistry.DefaultSpec(it.ID); ok {
			spec.Settings = widgetCfgs.For(it.ID)
			if sp, ok := stored[it.ID]; ok {
				spec = hydrateStoredSpec(spec, sp)
			}
			pl := domain.Placement{
				SchemaVersion: domain.WidgetSpecVersion,
				ID:            it.ID,
				Surface:       "dashboard",
				Spec:          spec,
				Layout:        it,
				Hidden:        hidden.IsHidden(it.ID),
			}
			if pl.Validate() != nil {
				continue
			}
			placements = append(placements, pl)
			if pl.Hidden {
				continue
			}
			rctx.Spec = pl.Spec
			if node, ok := safeRenderSpec(pl.Spec, rctx); ok {
				tiles = append(tiles, node)
			}
		} else if strings.HasPrefix(it.ID, userSpecPrefix) {
			// A Studio-designed widget: its full WidgetSpec is the persisted placement
			// (formula / pipeline / content layout / style). Render it through the same
			// engine path as a built-in — pure hydration, no Go body.
			sp, have := stored[it.ID]
			if !have {
				continue
			}
			pl := domain.Placement{
				SchemaVersion: domain.WidgetSpecVersion,
				ID:            it.ID,
				Surface:       "dashboard",
				Spec:          sp,
				Layout:        it,
				Hidden:        hidden.IsHidden(it.ID),
			}
			if pl.Validate() != nil {
				continue
			}
			placements = append(placements, pl)
			if pl.Hidden {
				continue
			}
			rctx.Spec = pl.Spec
			if node, ok := safeRenderSpec(pl.Spec, rctx); ok {
				tiles = append(tiles, node)
			}
		} else if strings.HasPrefix(it.ID, vbCardPrefix) {
			if hidden.IsHidden(it.ID) {
				continue
			}
			if w := vbPublishedWidget(strings.TrimPrefix(it.ID, vbCardPrefix), it.ColSpan, it.RowSpan); w != nil {
				tiles = append(tiles, w)
			}
		}
	}

	// The dashboard welcome/hero is a widget too: a full-width placement rendered
	// above the bento (not a packed grid cell) with ChromeHover, so it reads as clean
	// content but is a configurable, persisted widget like any tile.
	heroNode := ui.CreateElement(dashboardHero)
	if hspec, ok := widgetregistry.DefaultSpec("hero"); ok {
		hpl := domain.Placement{
			SchemaVersion: domain.WidgetSpecVersion,
			ID:            "hero",
			Surface:       "dashboard",
			Spec:          hspec,
			Layout:        dashlayout.Item{ID: "hero", ColSpan: 4, RowSpan: 2},
		}
		if hpl.Validate() == nil {
			placements = append(placements, hpl)
			rctx.Spec = hpl.Spec
			if n, ok := safeRender("hero", rctx); ok {
				heroNode = n
			}
		}
	}

	// Persist the dashboard's placements to SQLite as first-class rows so the layout
	// (and each tile's saved settings/style) travels with export and is the engine's
	// canonical representation (§8). Keyed on a signature of the placements, the
	// effect runs only when they actually change — never on a plain re-render — so it
	// can't loop with the data-revision re-render. The live edit surface remains the
	// layout/config atoms; this mirrors them into placement rows.
	plSig, _ := json.Marshal(placements)
	placementsToPersist := placements
	ui.UseEffect(func() func() {
		if app != nil {
			if err := app.PutPlacements(placementsToPersist); err != nil {
				slog.Warn("dashboard: persist placements failed", "err", err)
			}
		}
		return nil
	}, string(plSig))

	return Fragment(
		// Optional decorative banner band (B20) — shown only when the user picks a
		// banner; driven entirely by CSS vars/attribute set by uistate.ApplyBanner,
		// so it needs no state here. Decorative, hence aria-hidden.
		Div(css.Class("app-banner"), Attr("aria-hidden", "true")),
		// Home band (EC4): glanceable greeting + net-worth hero + this-month stats +
		// quick actions — now a ChromeHover widget (engine-rendered above the bento).
		heroNode,
		// C329: first-run onboarding callout — a dismissible setup checklist with a
		// link to the help center. Self-hides once setup is complete or dismissed.
		ui.CreateElement(dashOnboardCard),
		// C271: "While you were away" catch-up card — shown when new notifications
		// have arrived since the last time the user opened the Notification Center.
		// Dismissed per session (the atom resets on reload). Only shown when
		// lastSeen > 0 (not the very first open) and newCount > 0.
		ui.CreateElement(dashCatchUpCard),
		// C319: the "Customize" entry point now lives in the top bar (DashCustomizeButton),
		// grouped with the other page actions, instead of a floating bar above the bento.
		// C8: on a genuinely empty workspace (no accounts and no transactions) the
		// bento KPI grid is just a wall of $0 tiles with no hierarchy — suppress it
		// and let the welcome hero + onboarding checklist own the empty state. The
		// grid returns the moment there's any real data to summarise.
		If(len(accounts) > 0 || len(txns) > 0, Div(tiles...)),
	)
}

// dashCatchUpCard is a dismissible "While you were away" bento-adjacent card
// (C271). It appears above the bento grid when new notifications have arrived
// since the last time the user opened the Notification Center, giving a
// glanceable count with a direct link to the center. Dismissed per session
// (the dismissed state resets on reload); lastSeen is read from the KV store
// so the count is exactly what the Notification Center would show as "new".
func dashCatchUpCard() ui.Node {
	dismissedAtom := ui.UseState(false)
	nav := router.UseNavigate()

	feed := uistate.UseNotifyFeed().Get()
	now := time.Now().Unix()
	visible := uistate.VisibleFeed(feed, now)
	lastSeen := loadLastSeen()
	newCount := len(uistate.NewSinceLastSeen(visible, lastSeen))

	// Hide when: dismissed this session; first-ever open (lastSeen==0); no new items.
	if dismissedAtom.Get() || lastSeen == 0 || newCount == 0 {
		return nil
	}

	body := uistate.T("dashboard.catchUpBodyOne")
	if newCount > 1 {
		body = uistate.T("dashboard.catchUpBody", newCount)
	}

	onView := ui.UseEvent(func() {
		nav.Navigate(uistate.RoutePath("/notifications"))
	})
	onDismiss := ui.UseEvent(func() {
		dismissedAtom.Set(true)
	})

	return Div(
		css.Class("catchup-card"),
		Attr("data-testid", "dash-catchup-card"),
		Attr("role", "complementary"),
		Attr("aria-label", uistate.T("dashboard.catchUpTitle")),
		Div(css.Class("catchup-card-body"),
			Span(css.Class("catchup-card-icon"), "🔔"),
			Div(css.Class("catchup-card-text"),
				Strong(uistate.T("dashboard.catchUpTitle")),
				P(body),
			),
		),
		Div(css.Class("catchup-card-actions"),
			Button(css.Class("btn", "btn-primary"), Type("button"), OnClick(onView),
				uistate.T("dashboard.catchUpLink")),
			Button(css.Class("btn"), Type("button"), OnClick(onDismiss),
				uistate.T("notifications.catchUpDismiss")),
		),
	)
}

// freshnessWidget is the full-width Freshness nudge: a friendly reminder of which
// account balances look stale (via internal/freshness), with how long since each
// was last updated.
// humanizeStaleDays renders an account's days-since-update as a calm, capped
// label rather than a raw day count: a four-year-old sample balance reads "4y+",
// not an alarming "1460d". Days stay literal up to two months, then collapse to
// months and finally years so the freshness chips never shout a giant number.
func humanizeStaleDays(days int) string {
	switch {
	case days < 1:
		return "today"
	case days < 60:
		return fmt.Sprintf("%dd", days)
	case days < 365:
		return fmt.Sprintf("%dmo", days/30)
	default:
		return fmt.Sprintf("%dy+", days/365)
	}
}

func freshnessWidget(accounts []domain.Account, windows freshness.Windows, dismissals freshness.Dismissals, onRemind, onDismiss ui.Handler) ui.Node {
	now := time.Now()
	stale := freshness.VisibleStaleAccounts(accounts, windows, dismissals, now)
	var body ui.Node
	if len(stale) == 0 {
		body = P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.allFresh"))
	} else {
		// Cap the visible chips so a long stale list never overflows the tile; the
		// remainder collapses into a "+N more" chip (the full count is in the heading).
		const maxChips = 8
		shown, extra := stale, 0
		if len(shown) > maxChips {
			extra = len(shown) - maxChips
			shown = shown[:maxChips]
		}
		chips := make([]ui.Node, 0, len(shown)+1)
		for _, a := range shown {
			chips = append(chips, Span(css.Class("member-chip"),
				Span(a.Name),
				Span(css.Class("fig", tw.TextWarn), "· "+humanizeStaleDays(freshness.DaysSinceUpdate(a, now))),
			))
		}
		if extra > 0 {
			chips = append(chips, Span(css.Class("member-chip", tw.TextDim), fmt.Sprintf("+%d more", extra)))
		}
		body = Div(
			P(css.Class("t-body", tw.TextDim, tw.Mb2), uistate.T("dashboard.staleCount", len(stale))),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), chips),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("dashboard.remindTitle")), OnClick(onRemind), uistate.T("dashboard.remind")),
				Button(css.Class("btn"), Type("button"), OnClick(onDismiss), uistate.T("action.dismiss")),
			),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "freshness", Title: uistate.T("dashboard.freshness"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "8", Body: body,
	})
}

// upcomingBillsWidget is the 2×1 Upcoming bills widget: the next due date and
// minimum payment for each liability account that has them, soonest first.
// billsFrame is the FrameRenderer for the Upcoming bills widget (Kind==List): the
// next charges painted from the Frame the engine hydrated (columns
// name/due(unix)/days/amount(money)). The four-row cap is an engine-side limit
// transform; this only presents the rows. Due dates within a week are toned amber.
func billsFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	pr := uistate.UsePrefs().Get()
	nameCol, _ := fr.Column("name")
	dueCol, _ := fr.Column("due")
	daysCol, _ := fr.Column("days")
	amtCol, _ := fr.Column("amount")
	curCol, _ := fr.Column("currency")
	var body ui.Node
	if fr.Rows == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noUpcomingBills"))
	} else {
		rows := make([]ui.Node, 0, fr.Rows)
		for i := 0; i < fr.Rows; i++ {
			dueTone := "text-faint"
			if daysCol.Num(i) <= 7 {
				dueTone = "text-warn"
			}
			amt := money.New(amtCol.Int64(i), curCol.Str(i))
			// .UTC(): due dates are UTC-midnight calendar dates; local-zone
			// reconstruction shows them a day early west of UTC (C339).
			when := time.Unix(int64(dueCol.Num(i)), 0).UTC()
			rows = append(rows, Div(css.Class(tw.Flex, tw.JustifyBetween),
				Span(nameCol.Str(i)),
				Span(ClassStr(dueTone), pr.FormatDate(when)),
				Span(css.Class("fig", tw.FontDisplay, tw.TextDown, tw.W24, tw.TextRight), fmtMoney(amt.Neg())),
			))
		}
		body = Div(css.Class("t-body", tw.SpaceY25), rows)
	}
	bodyCls := ""
	if fr.Rows > 0 && widgetDisplay(c.Spec, "scroll") != "cap" {
		bodyCls = tw.Fold(tw.OverflowYAuto)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "bills", Title: uistate.T("dashboard.upcomingBills"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "6",
		BodyClass: bodyCls, Body: body,
	})
}

// breakdownFrame is the FrameRenderer for the Spending breakdown widget
// (Kind==Chart): a segmented bar of the period's expenses by category, painted from
// the Frame the engine hydrated (columns name/amount(money)/percent — the full root
// rollup, ranked). The top-N visible segments plus a collapsed "Other" are the
// widget's presentation choice; the engine supplies the ranked data.
func breakdownFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	topN := 3
	if sch, ok := widgetcfg.SchemaFor("breakdown"); ok {
		if f, ok := sch.FieldByKey("topN"); ok {
			topN = f.Int(c.Spec.Settings)
		}
	}
	if fr.Rows == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
			Body: P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noSpending")),
		})
	}
	nameCol, _ := fr.Column("name")
	pctCol, _ := fr.Column("percent")
	type seg struct {
		name string
		pct  int
	}
	// Build segments from the RAW float percents, summing the tail into "Other".
	// Display percents are then assigned so the bar fills exactly 100% — the last
	// segment absorbs the rounding remainder — rather than losing ~8% to per-segment
	// int truncation (which left the bar visibly underfilled and "Other" wrong).
	segs := make([]seg, 0, topN+1)
	var otherRaw float64
	for i := 0; i < fr.Rows; i++ {
		if i < topN {
			name := nameCol.Str(i)
			if name == "" {
				name = uistate.T("dashboard.uncategorized")
			}
			segs = append(segs, seg{name: name, pct: int(pctCol.Num(i) + 0.5)})
		} else {
			otherRaw += pctCol.Num(i)
		}
	}
	if otherRaw > 0 {
		segs = append(segs, seg{name: uistate.T("dashboard.other"), pct: int(otherRaw + 0.5)})
	}
	// Force the displayed percents to total exactly 100 so the segmented bar always
	// spans the full container width without over/underflow: distribute the rounding
	// difference onto the largest segment (largest-remainder, robust to either sign).
	if len(segs) > 0 {
		sum, largest := 0, 0
		for i := range segs {
			sum += segs[i].pct
			if segs[i].pct > segs[largest].pct {
				largest = i
			}
		}
		if diff := 100 - sum; diff != 0 {
			segs[largest].pct += diff
		}
	}

	tones := []string{"bg-up", "bg-warn", "bg-dim", "bg-down"}
	barParts := make([]ui.Node, 0, len(segs))
	legend := make([]ui.Node, 0, len(segs))
	for i, s := range segs {
		tone := tones[i%len(tones)]
		barParts = append(barParts, Div(ClassStr(tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", s.pct)})))
		legend = append(legend, Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap15),
			Span(ClassStr(tw.Fold(tw.W2, tw.H2, tw.RoundedFull)+" "+tw.ColorClass(tone))),
			Textf("%s %d%%", s.name, s.pct),
		))
	}

	body := Div(
		Div(css.Class(tw.H25, tw.RoundedFull, tw.OverflowHidden, tw.Flex), barParts),
		Div(css.Class("t-caption", tw.Flex, tw.FlexWrap, tw.GapX4, tw.GapY1, tw.Mt3, tw.TextDim), legend),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
		Body: body,
	})
}

// savingsRateWidget is the 2×1 Savings rate widget: the share of the period's
// income that wasn't spent, as a big figure and a bar.
func savingsRateWidget(ratePct float64, cfg widgetcfg.Config) ui.Node {
	// ratePct is the tile's configurable formula evaluated over the engine surface
	// (default (income−expense)/income·100). Truncate toward zero to match the hero's
	// whole-percent reading (ledger.SavingsRate convention).
	pct := int(ratePct)

	// Widget settings (gear → flip): target savings rate and whether to show the bar.
	target, showBar := 20, true
	if sch, ok := widgetcfg.SchemaFor("savings"); ok {
		if f, ok := sch.FieldByKey("target"); ok {
			target = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("showBar"); ok {
			showBar = f.Bool(cfg)
		}
	}

	// Tone reflects performance against the user's target: at/above = good,
	// positive-but-short = warning, negative = bad.
	tone, bar := "text-up", "bg-up"
	switch {
	case pct < 0:
		tone, bar = "text-down", "bg-down"
	case pct < target:
		tone, bar = "text-warn", "bg-warn"
	}

	left := Div(
		Div(ClassStr("fig t-figure-lg "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(tone)), fmt.Sprintf("%d%%", pct)),
		Div(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("dashboard.savingsSub", target)),
	)
	var right ui.Node = Fragment()
	if showBar {
		right = Div(css.Class(tw.Flex1),
			uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: bar}),
			Div(css.Class("t-caption", tw.TextFaint, tw.Mt2), uistate.T("dashboard.thisPeriod")),
		)
	}
	body := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5), left, right)
	return uiw.Widget(uiw.WidgetProps{
		ID: "savings", Title: uistate.T("dashboard.savingsRate"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "7",
		Body: body,
	})
}

// topHighlightWidget surfaces the single most significant spending change this
// month (via the shared anomaly detection) as a one-line plain-English highlight,
// or a calm "nothing notable" message when there's nothing to flag. It links the
// dashboard to the fuller Spending highlights card on the Insights screen.
func topHighlightWidget(txns []domain.Transaction, categories []domain.Category, rates currency.Rates) ui.Node {
	anomalies := detectSpendingAnomalies(txns, categories, rates)
	var body ui.Node
	if len(anomalies) == 0 {
		body = P(css.Class("t-body", tw.TextDim), uistate.T("dashboard.noHighlights"))
	} else {
		a := anomalies[0]
		body = Div(css.Class(tw.Flex, tw.ItemsStart, tw.Gap2),
			Span(ClassStr("insight-dot "+highlightTone(a)), uiw.Icon(highlightArrow(a), css.Class(tw.W4, tw.H4))),
			Span(css.Class("t-body"), highlightText(a, rates.Base)),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "highlight", Title: uistate.T("dashboard.highlight"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "9", Body: body,
	})
}

// cashFlowWidget is the 2×1 Cash flow widget: income (up) vs expense (down) bars
// for the last four months, scaled to the largest bar, with the current month's
// net to the right. Totals via ledger.PeriodTotals.
// cashFlowFrame is the FrameRenderer for the Cash flow widget (Kind==Chart): income
// (up) vs expense (down) bars per month, painted from the series Frame the engine
// hydrated (columns t(unix month start)/income(money)/expense(money), base currency).
// No totals are computed here.
func cashFlowFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	type monthBar struct {
		label           string
		income, expense int64
	}
	tCol, _ := fr.Column("t")
	incCol, _ := fr.Column("income")
	expCol, _ := fr.Column("expense")
	rates := c.Rates
	months := make([]monthBar, 0, fr.Rows)
	var maxv int64 = 1
	for i := 0; i < fr.Rows; i++ {
		mb := monthBar{
			// .UTC(): month-bucket boundaries are UTC; local-zone reconstruction
			// mislabels the month west of UTC (C339).
			label:   time.Unix(int64(tCol.Num(i)), 0).UTC().Format("Jan"),
			income:  incCol.Int64(i),
			expense: expCol.Int64(i),
		}
		if mb.income > maxv {
			maxv = mb.income
		}
		if mb.expense > maxv {
			maxv = mb.expense
		}
		months = append(months, mb)
	}
	if len(months) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "cashflow", Title: uistate.T("dashboard.cashFlow"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "6",
			Body: P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noTransactions")),
		})
	}

	bars := make([]ui.Node, 0, len(months))
	for i, mb := range months {
		labelTone := "text-faint"
		if i == len(months)-1 {
			labelTone = "text-fg"
		}
		bars = append(bars, Div(css.Class(tw.Flex, tw.FlexCol, tw.ItemsCenter, tw.Gap15),
			Div(css.Class(tw.Flex, tw.ItemsEnd, tw.Gap1, tw.H14),
				Div(css.Class(tw.W3, tw.BgUp), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.income*100/maxv))})),
				Div(css.Class(tw.W3, tw.BgDown), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.expense*100/maxv))})),
			),
			Span(ClassStr("t-caption "+labelTone), mb.label),
		))
	}

	last := months[len(months)-1]
	netMoney := money.New(last.income-last.expense, rates.Base)
	netTone := "text-up"
	if last.income-last.expense < 0 {
		netTone = "text-down"
	}
	netBlock := Div(css.Class(tw.MlAuto, tw.TextRight),
		Div(css.Class("t-caption", tw.TextFaint), "net · "+last.label),
		Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.TextLg)+" "+tw.ColorClass(netTone)), fmtMoney(netMoney)),
	)

	// R52(a): a one-sentence plain-English takeaway under the bars, so the widget
	// states what the income-vs-expense bars mean instead of leaving an unlabeled
	// mini-chart to interpret. Toned to match the net (kept = up, short = down).
	netAmt := last.income - last.expense
	var caption ui.Node
	switch {
	case netAmt > 0:
		caption = P(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowKept", fmtMoney(netMoney), last.label))
	case netAmt < 0:
		caption = P(ClassStr("t-caption "+tw.ColorClass("text-down")), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowShort", fmtMoney(money.New(-netAmt, rates.Base)), last.label))
	default:
		caption = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowEven", last.label))
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "cashflow", Title: uistate.T("dashboard.cashFlow"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "6",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			Div(css.Class(tw.Flex, tw.ItemsEnd, tw.Gap5), bars, netBlock),
			caption,
		),
	})
}

// netWorthTrendWidget is the 1×2 Net worth trend widget: the current figure over
// a six-month end-of-month area chart (via ledger.NetWorthSeries + the chart
// geometry helpers).
// trendFrame is the FrameRenderer for the Net worth trend (Kind==Chart): it paints
// the headline figure + a six-month area chart from the series Frame the engine
// hydrated (columns t(unix)/value(money)). The figure currency is the base; window
// length and axis visibility come from the tile settings. No data is computed here.
func trendFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	net := c.Net
	months := 6
	showXAxis := true
	if sch, ok := widgetcfg.SchemaFor("trend"); ok {
		if f, ok := sch.FieldByKey("months"); ok {
			months = f.Int(c.Spec.Settings)
		}
		if f, ok := sch.FieldByKey("showXAxis"); ok {
			showXAxis = f.Bool(c.Spec.Settings)
		}
	}
	start := dateutil.MonthStart(time.Now())
	valCol, _ := fr.Column("value")
	tCol, _ := fr.Column("t")
	series := make([]money.Money, fr.Rows)
	for i := 0; i < fr.Rows; i++ {
		series[i] = money.New(valCol.Int64(i), net.Currency)
	}
	deltaLabel, deltaTone := "No change", "text-dim"
	rangeLabel := ""
	if len(series) > 0 {
		first := series[0]
		last := series[len(series)-1]
		delta := money.New(last.Amount-first.Amount, last.Currency)
		switch {
		case delta.IsPositive():
			deltaLabel = "Up " + fmtMoney(delta)
			deltaTone = "text-up"
		case delta.IsNegative():
			deltaLabel = "Down " + fmtMoney(delta.Abs())
			deltaTone = "text-down"
		}
		low, high := first, first
		for _, m := range series[1:] {
			if m.Amount < low.Amount {
				low = m
			}
			if m.Amount > high.Amount {
				high = m
			}
		}
		rangeLabel = fmt.Sprintf("%s - %s", fmtMoney(low), fmtMoney(high))
	}
	// Plot in major units (dollars), not raw minor units (cents): feeding cents
	// made the Y axis read "2,000,000 / 1,500,000 …" and clip in the narrow
	// widget (C16). The Y-axis format hint renders ticks as compact currency
	// ("$20k"); see web/chart.js.
	div := 1.0
	for i := 0; i < currency.Decimals(net.Currency); i++ {
		div *= 10
	}
	pts := make([]chartspec.Point, len(series))
	for i, m := range series {
		// Labels come from the Frame's t column (unix seconds per cutoff).
		label := ""
		if ts := int64(tCol.Num(i)); ts != 0 {
			// .UTC(): cutoffs are UTC month boundaries (C339).
			label = trendPointLabel(time.Unix(ts, 0).UTC(), months)
		}
		// C215: the final cutoff is next-month-start, so it captures the current
		// month's data "so far" — label it as the current month + "(so far)" instead
		// of the next month's name, which read as a confusing unlabeled partial point.
		if i == len(series)-1 {
			label = trendPointLabel(start, months) + " " + uistate.T("dashboard.trendSoFar")
		}
		pts[i] = chartspec.Point{X: float64(i), Y: float64(m.Amount) / div, Label: label}
	}
	yFmt := ".3~s" // compact SI w/ enough precision to keep narrow-range ticks distinct, e.g. "21.4k"
	if currency.Symbol(net.Currency) == "$" {
		yFmt = "$.3~s" // "$21.4k" for dollar currencies
	}
	spec := chartspec.Spec{
		Kind:   chartspec.Area,
		Series: []chartspec.Series{{Name: uistate.T("dashboard.netWorth"), Points: pts}}, // empty Color → theme accent
		X:      chartspec.Axis{Label: uistate.T("dashboard.axisTime")},
		Y:      chartspec.Axis{Format: yFmt},
	}
	if !showXAxis {
		spec.X.Format = "hidden"
	}
	body := Div(css.Class("trend-body"),
		Div(css.Class("trend-head"),
			Div(css.Class("trend-figure fig t-figure", tw.FontDisplay), fmtMoney(net)),
			Div(css.Class("trend-standard t-caption", tw.TextDim), trendWindowLabel(months)),
		),
		Div(css.Class("trend-expanded"),
			Div(css.Class("trend-stat"),
				Span(css.Class("t-caption", tw.TextFaint), uistate.T("dashboard.trendChange")),
				Span(ClassStr("fig t-body "+deltaTone), deltaLabel),
			),
			Div(css.Class("trend-stat"),
				Span(css.Class("t-caption", tw.TextFaint), uistate.T("dashboard.trendRange")),
				Span(css.Class("fig t-body", tw.TextDim), rangeLabel),
			),
		),
		uiw.Chart(uiw.ChartProps{
			Spec:   spec,
			Height: "100%",
			Class:  "trend-chart",
			Label:  uistate.T("dashboard.netWorthChartLabel", fmtMoney(net)),
		}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "trend", Title: uistate.T("dashboard.netWorthTrend"), Draggable: true, Resizable: true, GridColumn: "4", GridRow: "3 / span 2",
		BodyClass: tw.Fold(tw.Flex, tw.FlexCol, tw.MinH0), Body: body,
	})
}

// accountsWidget is the 2×1 Accounts widget: a small grid of active account
// balances (accounting figures, negatives toned red) via ledger.Balance. How
// many accounts to show, and whether to show only cleared balances, are
// configurable.
type emptyAddProps struct {
	Message string
	Label   string
	Path    string
}

// emptyAddCTA renders a dashboard widget's empty state with an in-context "add"
// button that routes to the relevant screen, so a user can create data from the
// dashboard instead of hunting for the screen (C23). Its own component so the
// navigate hook stays at a stable position.
func emptyAddCTA(props emptyAddProps) ui.Node {
	nav := router.UseNavigate()
	path := props.Path
	return Div(css.Class("empty t-body", tw.TextDim, tw.Flex, tw.FlexCol, tw.ItemsStart, tw.Gap2),
		Span(props.Message),
		Button(css.Class("btn btn-primary"), Type("button"), OnClick(func() { nav.Navigate(uistate.RoutePath(path)) }), props.Label),
	)
}

func trendWindowLabel(months int) string {
	if months >= 24 && months%12 == 0 {
		return fmt.Sprintf("%d years", months/12)
	}
	if months == 12 {
		return "1 year"
	}
	return fmt.Sprintf("%d months", months)
}

func trendPointLabel(t time.Time, months int) string {
	if months > 36 {
		if t.Month() == time.January {
			return t.Format("2006")
		}
		return t.Format("Jan '06")
	}
	return t.Format("Jan '06")
}

// accountsFrame is the FrameRenderer for the Accounts widget (Kind==List): a grid of
// account balances painted from the Frame the engine hydrated (columns
// name/balance(money)/currency/tone). Cleared-balance selection and the row cap are
// engine-side (Source.Cleared + a limit transform); this only presents the columns.
func accountsFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	nameCol, _ := fr.Column("name")
	balCol, _ := fr.Column("balance")
	curCol, _ := fr.Column("currency")
	toneCol, _ := fr.Column("tone")
	cells := make([]ui.Node, 0, fr.Rows)
	for i := 0; i < fr.Rows; i++ {
		bal := money.New(balCol.Int64(i), curCol.Str(i))
		tone := ""
		if toneCol.Str(i) == "down" {
			tone = "text-down"
		}
		cells = append(cells, Div(
			Div(css.Class(tw.TextDim), nameCol.Str(i)),
			Div(ClassStr("fig t-body "+tw.Fold(tw.FontDisplay, tw.Mt05)+" "+tw.ColorClass(tone)), fmtMoney(bal)),
		))
	}
	var body ui.Node
	bodyCls := ""
	if len(cells) == 0 {
		body = ui.CreateElement(emptyAddCTA, emptyAddProps{Message: uistate.T("dashboard.noAccountsYet"), Label: uistate.T("dashboard.addAccount"), Path: "/accounts"})
	} else {
		body = Div(css.Class("t-body", tw.Grid, tw.GridCols3, tw.Gap4), cells)
		if widgetDisplay(c.Spec, "scroll") != "cap" {
			bodyCls = tw.Fold(tw.OverflowYAuto)
		}
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "accounts", Title: uistate.T("nav.accounts"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "5",
		BodyClass: bodyCls, Body: body,
	})
}

// todoWidget is the 1×1 To-do widget: the next few open tasks (how many is
// configurable, default 3), dot-toned by priority (high = amber, others =
// dim/faint).
func todoWidget(app *appstate.App, cfg widgetcfg.Config) ui.Node {
	count, sortMode, showCompleted := 3, tasksort.ModeSmart, false
	if sch, ok := widgetcfg.SchemaFor("todo"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			count = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("sort"); ok {
			sortMode = tasksort.ParseMode(f.Str(cfg))
		}
		if f, ok := sch.FieldByKey("showCompleted"); ok {
			showCompleted = f.Bool(cfg)
		}
	}

	all := app.Tasks()
	var openTasks, doneTasks []domain.Task
	for _, t := range all {
		if t.Status == domain.StatusDone {
			doneTasks = append(doneTasks, t)
		} else {
			openTasks = append(openTasks, t)
		}
	}
	// Overdue first, then the chosen order — an overdue cue belongs at the top (C52).
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	overdue := func(t domain.Task) bool { return !t.Due.IsZero() && t.Due.Before(today) }
	ordered := tasksort.OrderBy(openTasks, sortMode)
	var od, rest []domain.Task
	for _, t := range ordered {
		if overdue(t) {
			od = append(od, t)
		} else {
			rest = append(rest, t)
		}
	}
	openOrdered := append(od, rest...)

	if len(openOrdered) == 0 && !(showCompleted && len(doneTasks) > 0) {
		return uiw.Widget(uiw.WidgetProps{
			ID: "todo", Title: uistate.T("nav.todo"), Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
			Body: ui.CreateElement(emptyAddCTA, emptyAddProps{Message: uistate.T("dashboard.nothingToDo"), Label: uistate.T("dashboard.addTodo"), Path: "/todo"}),
		})
	}

	shown := openOrdered
	truncated := 0
	if len(shown) > count {
		truncated = len(shown) - count
		shown = shown[:count]
	}

	rows := make([]ui.Node, 0, len(shown)+len(doneTasks)+2)
	for _, t := range shown {
		rows = append(rows, ui.CreateElement(dashTaskRow, dashTaskRowProps{Task: t, Overdue: overdue(t)}))
	}
	if showCompleted {
		done := tasksort.OrderBy(doneTasks, sortMode)
		if len(done) > count {
			done = done[:count]
		}
		for _, t := range done {
			rows = append(rows, ui.CreateElement(dashTaskRow, dashTaskRowProps{Task: t}))
		}
	}
	if truncated > 0 {
		rows = append(rows, ui.CreateElement(todoMoreLink, todoMoreProps{N: truncated}))
	}

	progress := uistate.T("dashboard.todoProgress", len(openOrdered), len(doneTasks))
	body := Div(
		P(css.Class("t-caption", tw.TextDim, tw.Mb2), progress),
		Div(css.Class("t-body", tw.SpaceY15), rows),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo", Title: uistate.T("nav.todo"), Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
		Body: body,
	})
}

type dashTaskRowProps struct {
	Task    domain.Task
	Overdue bool
}

// dashTaskRow renders one dashboard To-do row with an inline complete checkbox and
// a title that drills into /todo. Its own component so the toggle/nav hooks stay
// at stable positions across the list (the On* loop gotcha). Toggling completion
// writes the task and bumps the data revision (content change, not layout — the
// bento FLIP signature is undisturbed).
func dashTaskRow(props dashTaskRowProps) ui.Node {
	t := props.Task
	nav := router.UseNavigate()
	app := appstate.Default
	rev := uistate.UseDataRevision()
	done := t.Status == domain.StatusDone

	toggle := ui.UseEvent(func() {
		if app == nil {
			return
		}
		nt := t
		if done {
			nt.Status = domain.StatusOpen
		} else {
			nt.Status = domain.StatusDone
		}
		if err := app.PutTask(nt); err == nil {
			rev.Set(rev.Get() + 1)
		}
	})
	openTodo := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/todo")) })

	dotTone, prio := "text-faint", "Low priority"
	var dotContent any = "○"
	switch t.Priority {
	case domain.PriorityHigh:
		dotTone, dotContent, prio = "text-warn", uiw.Icon(icon.AlertTriangle, css.Class(tw.W4, tw.H4, tw.ShrinkO)), "High priority"
	case domain.PriorityMedium:
		dotTone, dotContent, prio = "text-dim", "●", "Medium priority"
	}
	titleCls := tw.Fold(tw.Flex1, tw.TextLeft, tw.Truncate)
	if done {
		titleCls += " " + tw.Fold(tw.LineThrough, tw.TextFaint)
	} else if props.Overdue {
		titleCls += " " + tw.Fold(tw.TextDown)
	}
	checkLabel := uistate.T("dashboard.todoComplete", t.Title)
	return Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter),
		Button(css.Class("dash-check"), Type("button"), Attr("role", "checkbox"), Attr("aria-checked", boolStr(done)),
			Attr("aria-label", checkLabel), Attr("title", checkLabel), OnClick(toggle),
			Text(checkGlyph(done))),
		Span(ClassStr(dotTone), Attr("title", prio), Attr("aria-label", prio), dotContent),
		Button(ClassStr("dash-task "+titleCls), Type("button"), OnClick(openTodo), t.Title),
	)
}

func checkGlyph(done bool) string {
	if done {
		return "☑"
	}
	return "☐"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

type todoMoreProps struct{ N int }

// todoMoreLink is the "+N more →" footer linking to the full To-do screen (no
// silent truncation). Its own component for a stable nav hook.
func todoMoreLink(props todoMoreProps) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/todo")) })
	return Button(css.Class("t-caption", tw.TextDim, tw.HoverTextFg, tw.Mt1), Type("button"), OnClick(open),
		uistate.T("dashboard.todoMore", props.N))
}

// goalsWidget is the 1×1 Goals widget: one goal's progress (% + saved / target)
// via internal/goals. By default it features the first goal; configurably it can
// feature the goal nearest completion, and the target-date caption is optional.
func goalsWidget(app *appstate.App, cfg widgetcfg.Config) ui.Node {
	pr := uistate.UsePrefs().Get()
	byProgress, showDate := false, true
	if sch, ok := widgetcfg.SchemaFor("goals"); ok {
		if f, ok := sch.FieldByKey("byProgress"); ok {
			byProgress = f.Bool(cfg)
		}
		if f, ok := sch.FieldByKey("showDate"); ok {
			showDate = f.Bool(cfg)
		}
	}
	list := app.Goals()
	if len(list) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goals", Title: uistate.T("nav.goals"), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
			Body: ui.CreateElement(emptyAddCTA, emptyAddProps{Message: uistate.T("dashboard.noGoalsYet"), Label: uistate.T("dashboard.addGoal"), Path: "/goals"}),
		})
	}
	g := list[0]
	if byProgress {
		// Feature the goal nearest completion (highest percent; first wins ties).
		best := goals.Percent(g)
		for _, cand := range list[1:] {
			if p := goals.Percent(cand); p > best {
				best, g = p, cand
			}
		}
	}
	pct := goals.Percent(g)
	caption := fmt.Sprintf("%d%%", pct)
	if showDate && !g.TargetDate.IsZero() {
		caption += " · by " + pr.FormatDate(g.TargetDate)
	}
	body := Div(
		Div(css.Class("t-body", tw.Flex, tw.JustifyBetween),
			Span(css.Class(tw.TextDim), "saved"),
			Span(css.Class("fig t-body", tw.FontDisplay), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
		),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: "bg-up", Class: "mt-2"}),
		Div(css.Class("t-caption", tw.TextDim, tw.Mt15), caption),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goals", Title: uistate.T("dashboard.goalPrefix", g.Name), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
		Body: body,
	})
}

// budgetsWidget is the 1×2 Budgets widget: spend vs limit per budget with an
// ok/near/over progress bar (via internal/budgeting). It evaluates the shared
// dashboard period window (start/end) so it stays in sync with the top-bar
// time selector. Over-budget rows are clickable links to the Budgets screen.
// budgetsFrame is the FrameRenderer for the Budgets widget (Kind==List): one
// progress row per budget, colored by the state tone, painted from the Frame the
// engine hydrated (columns name/percent/state). At-risk filtering and the row cap
// are engine-side transforms; this only presents the rows.
func budgetsFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	app := c.App
	var body ui.Node
	if fr.Rows == 0 {
		if len(app.Budgets()) == 0 {
			// Genuinely no budgets — offer to add one in context.
			body = ui.CreateElement(emptyAddCTA, emptyAddProps{Message: uistate.T("dashboard.noBudgetsYet"), Label: uistate.T("dashboard.addBudget"), Path: "/budgets"})
		} else {
			// Budgets exist but none match the at-risk filter — not an add case.
			body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noBudgetAlerts"))
		}
	} else {
		nameCol, _ := fr.Column("name")
		pctCol, _ := fr.Column("percent")
		stateCol, _ := fr.Column("state")
		rows := make([]ui.Node, 0, fr.Rows)
		for i := 0; i < fr.Rows; i++ {
			state := stateCol.Str(i)
			tone, bar := "text-dim", "bg-up"
			switch state {
			case "near":
				tone, bar = "text-warn", "bg-warn"
			case "over":
				tone, bar = "text-down", "bg-down"
			}
			rows = append(rows, ui.CreateElement(dashBudgetRow, dashBudgetRowProps{
				Label:   nameCol.Str(i),
				Percent: int(pctCol.Num(i)),
				Tone:    tone,
				Bar:     bar,
				Over:    state == "over",
			}))
		}
		body = Div(css.Class("t-body", tw.SpaceY4), rows)
	}
	bodyCls := ""
	if fr.Rows > 0 && widgetDisplay(c.Spec, "scroll") != "cap" {
		bodyCls = tw.Fold(tw.OverflowYAuto)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budgets", Title: uistate.T("nav.budgets"), Draggable: true, Resizable: true,
		GridColumn: "3", GridRow: "3 / span 2", BodyClass: bodyCls, Body: body,
	})
}

// recentFrame is the FrameRenderer for the Recent transactions widget (Kind==List):
// a compact table painted from the Frame the engine hydrated (columns
// date(unix)/desc/amount(money)/currency). The newest-first order and the row cap
// are engine-side (the resolver sorts; a limit transform caps). Dates render in a
// compact "Jan 2" form so the column never wraps to two lines.
func recentFrame(fr domain.Frame, c widgetrender.RenderCtx) ui.Node {
	dateCol, _ := fr.Column("date")
	descCol, _ := fr.Column("desc")
	amtCol, _ := fr.Column("amount")
	curCol, _ := fr.Column("currency")
	display := widgetDisplay(c.Spec, "page")
	pageSize := atoiOr(c.Spec.Settings["count"], 6)
	var body ui.Node
	bodyCls := tw.Fold(tw.OverflowHidden)
	if fr.Rows == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noTransactions"))
	} else {
		rows := make([]ui.Node, 0, fr.Rows)
		for i := 0; i < fr.Rows; i++ {
			amt := money.New(amtCol.Int64(i), curCol.Str(i))
			// .UTC(): txn dates are UTC-midnight calendar dates; local-zone
			// reconstruction rendered them a day early west of UTC (C339).
			when := time.Unix(int64(dateCol.Num(i)), 0).UTC()
			rows = append(rows, Tr(css.Class(tw.BorderB, tw.BorderLine70),
				Td(css.Class("fig", tw.Py25, tw.TextDim, tw.W16, tw.Truncate), when.Format("Jan 2")),
				Td(css.Class(tw.Py25), descCol.Str(i)),
				Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)+" "+tw.ColorClass(figTone(amt))), fmtMoney(amt)),
			))
		}
		switch display {
		case "page":
			body = ui.CreateElement(pagedList, pagedListProps{Rows: rows, PageSize: pageSize, AsTable: true})
		case "scroll":
			body = Table(css.Class("t-body", tw.WFull), Tbody(rows))
			bodyCls = tw.Fold(tw.OverflowYAuto)
		default: // cap
			body = Table(css.Class("t-body", tw.WFull), Tbody(rows))
		}
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "recent", Title: uistate.T("dashboard.recent"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "3 / span 2", BodyClass: bodyCls,
		Body: body,
	})
}

// DashboardLayoutControls renders the dashboard layout manager — the Custom/Auto
// mode selector (C24) and a Reset-layout action — for the Settings modal. Custom
// keeps your hand-arranged order; the auto modes reorder the tiles (sizes stay as
// you set them). Switching to Custom bakes the current auto order in so nothing
// jumps. It lived in a wasted full-width header cell on the dashboard; it now
// lives in Settings so the canvas is all widgets.
func DashboardLayoutControls() ui.Node {
	layoutAtom := uistate.UseLayoutItems()
	modeAtom := uistate.UseLayoutMode()
	reset := func() {
		d := dashlayout.DefaultLayoutItems()
		layoutAtom.Set(d)
		uistate.PersistItems(d)
	}
	onMode := ui.UseEvent(func(e ui.Event) {
		m := dashlayout.Mode(e.GetValue())
		if !m.Valid() {
			return
		}
		if m == dashlayout.ModeCustom {
			baked := dashlayout.Arrange(layoutAtom.Get(), modeAtom.Get())
			layoutAtom.Set(baked)
			uistate.PersistItems(baked)
		}
		modeAtom.Set(m)
		uistate.PersistLayoutMode(m)
	})
	mode := modeAtom.Get()
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3, tw.FlexWrap),
		Select(css.Class("rstep t-caption"), Attr("title", uistate.T("dashboard.layoutMode")), OnChange(onMode),
			Option(Value(string(dashlayout.ModeCustom)), SelectedIf(mode == dashlayout.ModeCustom), uistate.T("dashboard.layoutCustom")),
			Option(Value(string(dashlayout.ModeAutoDefault)), SelectedIf(mode == dashlayout.ModeAutoDefault), uistate.T("dashboard.layoutAutoDefault")),
			Option(Value(string(dashlayout.ModeAutoImportance)), SelectedIf(mode == dashlayout.ModeAutoImportance), uistate.T("dashboard.layoutAutoImportance")),
		),
		Button(css.Class("data-btn"), Type("button"), OnClick(reset), uistate.T("dashboard.reset")),
	)
}

// spanOf returns the intrinsic column/row span of the widget with the given id
// in the current layout, defaulting to 1×1 when absent. The attention widget uses
// it to choose how much detail to render (responsive-by-span).
func spanOf(items []dashlayout.Item, id string) (col, row int) {
	for _, it := range items {
		if it.ID == id {
			c, r := it.ColSpan, it.RowSpan
			if c < 1 {
				c = 1
			}
			if r < 1 {
				r = 1
			}
			return c, r
		}
	}
	return 1, 1
}

// attentionWidget is the headline "Needs attention" digest: the urgent, act-now
// signals (bills due soon, near/over budgets, stale balances, overdue &
// high-priority to-dos, the biggest spending spike), ranked by the pure
// internal/attention package under the widget's gear/flip settings. It is
// responsive-by-span: at 1×1 it shows the single most-urgent item plus a count;
// wider/taller it shows more. Default placement is 4×1 at the top of the grid.
func attentionWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time, dismissals freshness.Dismissals, cfg widgetcfg.Config, spanCol, spanRow int) ui.Node {
	now := time.Now()

	// Budget statuses (near/over are what the digest keeps), rolled up like the
	// Budgets widget so parent budgets include sub-category spend.
	cats := app.Categories()
	statuses := make([]budgeting.Status, 0, len(app.Budgets()))
	bs, be := dateutil.MonthRange(now)
	for _, b := range app.Budgets() {
		if st, err := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); err == nil {
			statuses = append(statuses, st)
		}
	}

	var anomalyPtr *insights.Anomaly
	if anomalies := detectSpendingAnomalies(txns, cats, rates); len(anomalies) > 0 {
		anomalyPtr = &anomalies[0]
	}

	items := attention.Rank(attention.Inputs{
		Now:     now,
		Bills:   bills.UpcomingAll(app.Accounts(), app.Recurring(), now),
		Budgets: statuses,
		Stale:   freshness.VisibleStaleAccounts(app.Accounts(), app.FreshnessWindows(), dismissals, now),
		Tasks:   app.Tasks(),
		Anomaly: anomalyPtr,
	}, attentionConfig(cfg))

	base := rates.Base
	var body ui.Node
	switch {
	case len(items) == 0:
		body = P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.attentionClear"))
	case spanCol < 2 && spanRow < 2:
		// Compact 1×1: the single most-urgent item, plus a count of the rest.
		rows := []ui.Node{ui.CreateElement(attentionRow, attentionRowProps{Item: items[0], Base: base})}
		if crit, warn := attention.Counts(items); crit+warn > 1 {
			rows = append(rows, P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("dashboard.attentionMore", crit+warn-boolToInt(items[0].Severity >= attention.SeverityWarning))))
		}
		body = Div(css.Class("attention-list"), rows)
	default:
		rows := make([]ui.Node, 0, len(items))
		for _, it := range items {
			rows = append(rows, ui.CreateElement(attentionRow, attentionRowProps{Item: it, Base: base}))
		}
		// Wide-and-short (e.g. the default 4×1) flows items as wrapping chips; any
		// layout with height stacks them as a list.
		cls := "attention-list"
		if spanRow < 2 {
			cls = "attention-chips"
		}
		body = Div(ClassStr(cls), rows)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "attention", Title: uistate.T("dashboard.attention"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "1", Body: body,
	})
}

// attentionConfig maps the widget's stored gear settings to a typed
// attention.Config, falling back to the schema defaults.
func attentionConfig(cfg widgetcfg.Config) attention.Config {
	out := attention.DefaultConfig()
	sch, ok := widgetcfg.SchemaFor("attention")
	if !ok {
		return out
	}
	boolField := func(key string, dst *bool) {
		if f, ok := sch.FieldByKey(key); ok {
			*dst = f.Bool(cfg)
		}
	}
	boolField("bills", &out.Bills)
	boolField("budgets", &out.Budgets)
	boolField("stale", &out.Stale)
	boolField("tasks", &out.Tasks)
	boolField("spending", &out.Spending)
	if f, ok := sch.FieldByKey("billsDays"); ok {
		out.BillsWindowDays = f.Int(cfg)
	}
	if f, ok := sch.FieldByKey("maxItems"); ok {
		out.MaxItems = f.Int(cfg)
	}
	if f, ok := sch.FieldByKey("minSeverity"); ok {
		switch f.Str(cfg) {
		case "warn":
			out.MinSeverity = attention.SeverityWarning
		case "critical":
			out.MinSeverity = attention.SeverityCritical
		default:
			out.MinSeverity = attention.SeverityInfo
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// digestCap is the most cross-page insights the digest widget shows inline.
// Kept small so the widget is glanceable, not a wall — the full set lives on /smart.
const digestCap = 3

// smartDigestWidget is the "Smart digest" dashboard widget: a compact, cross-page
// glance at the top active insights from all enabled Free engines. Gated by
// AffordanceWidget (Standard density), so it only appears when the user's density
// dial permits dashboard widgets. Strictly additive: if no features are enabled or
// no insights are active, it renders the neutral empty hint rather than nothing,
// so the tile still makes sense in the widget manager. Returns a full bento tile.
func smartDigestWidget(app *appstate.App) ui.Node {
	pr := uistate.UsePrefs().Get()
	settings := uistate.LoadSmartSettings()

	const widgetID = "smart-digest"

	if !settings.DensityOrDefault().Shows(smart.AffordanceWidget) {
		return uiw.Widget(uiw.WidgetProps{
			ID: widgetID, Title: uistate.T("smart.digestTitle"), Draggable: true, Resizable: true,
			GridColumn: "1 / span 2", GridRow: "10",
			Body: P(css.Class("empty t-body", tw.TextDim), uistate.T("smart.digestEmpty")),
		})
	}

	in := buildSmartInput(app, pr.WeekStartWeekday())
	all := smartengine.Run(in, settings)
	if len(all) > digestCap {
		all = all[:digestCap]
	}

	var body ui.Node
	if len(all) == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("smart.digestEmpty"))
	} else {
		body = Div(css.Class("t-body", tw.Flex, tw.FlexCol),
			Attr("data-testid", "smart-digest-list"),
			smartDigestList(all),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: widgetID, Title: uistate.T("smart.digestTitle"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "10",
		// Scroll within the tile if the digest has more insights than fit, so content
		// never overflows the bento cell.
		BodyClass: tw.Fold(tw.OverflowYAuto),
		Body:      body,
	})
}

// attentionRowProps configures one attention digest row.
type attentionRowProps struct {
	Item attention.Item
	Base string
}

// attentionRow renders one urgent item as a clickable line — a severity dot, the
// plain-English detail, and (when one exists) a deep link that navigates to the
// item's screen and scrolls to it. Its own component so the navigate hook stays
// at a stable position across the list.
func attentionRow(props attentionRowProps) ui.Node {
	nav := router.UseNavigate()
	it := props.Item
	open := func() {
		if it.Route == "" {
			return
		}
		nav.Navigate(uistate.RoutePath(it.Route))
		if it.AnchorID != "" {
			scrollToID(it.AnchorID)
		}
	}
	return Button(ClassStr("attention-item "+attentionTone(it.Severity)), Type("button"), OnClick(open),
		Attr("title", uistate.T("dashboard.attentionOpen")),
		Span(css.Class("attention-dot"), Attr("aria-hidden", "true"), attentionGlyph(it.Severity)),
		Span(css.Class("attention-text"), attentionText(it, props.Base)),
	)
}

// attentionText renders the plain-English line for an item from its structured
// fields, localizing at the edge.
func attentionText(it attention.Item, base string) string {
	switch it.Kind {
	case attention.KindBill:
		when := uistate.T("dashboard.attentionDueToday")
		switch {
		case it.Days == 1:
			when = uistate.T("dashboard.attentionDueTomorrow")
		case it.Days > 1:
			when = uistate.T("dashboard.attentionDueInDays", it.Days)
		}
		return uistate.T("dashboard.attentionBill", it.Label, when, fmtMoney(it.Amount))
	case attention.KindBudget:
		if it.Severity >= attention.SeverityCritical {
			return uistate.T("dashboard.attentionBudgetOver", it.Label, it.Pct)
		}
		return uistate.T("dashboard.attentionBudgetNear", it.Label, it.Pct)
	case attention.KindStale:
		return uistate.T("dashboard.attentionStale", it.Label, it.Days)
	case attention.KindTask:
		if it.Severity >= attention.SeverityCritical {
			return uistate.T("dashboard.attentionTaskOverdue", it.Label, it.Days)
		}
		return uistate.T("dashboard.attentionTaskHigh", it.Label)
	case attention.KindSpending:
		if it.Anomaly != nil {
			return highlightText(*it.Anomaly, base)
		}
	}
	return it.Label
}

func attentionTone(s attention.Severity) string {
	switch s {
	case attention.SeverityCritical:
		return "is-critical"
	case attention.SeverityWarning:
		return "is-warning"
	default:
		return "is-info"
	}
}

func attentionGlyph(s attention.Severity) ui.Node {
	switch s {
	case attention.SeverityCritical:
		return uiw.Icon(icon.AlertTriangle, css.Class(tw.W4, tw.H4, tw.ShrinkO))
	case attention.SeverityWarning:
		return Text("●")
	default:
		return Text("○")
	}
}

// plural renders a count with a singular/plural noun, e.g. "1 deposit" or
// "3 deposits".
func plural(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// kpiBody renders a KPI tile's body: a large accounting figure with a small
// subline. figTone/subTone are color classes (e.g. "text-up", "text-dim").
func kpiBody(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(ClassStr("fig t-figure "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(figTone)),
			Attr("data-countup", ""), figure),
		// Keep the sub-label to one line (ellipsis if too narrow) so a long caption
		// never wraps and breaks the KPI baseline across the tile row.
		Div(ClassStr("t-caption "+tw.Fold(tw.Pt15, tw.Truncate)+" "+tw.ColorClass(subTone)), Attr("title", subline), subline),
	)
}

// kpiBodyHero renders the visual-hero variant of a KPI tile body: a larger
// figure (t-figure-lg) so the headline number draws the eye first. Used for
// Net Worth, the most important single number on the dashboard (L33).
func kpiBodyHero(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(ClassStr("fig t-figure-lg "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(figTone)),
			Attr("data-countup", ""), figure),
		// One-line sub (ellipsis if too narrow; full text on hover), matching kpiBody.
		Div(ClassStr("t-caption "+tw.Fold(tw.Pt15, tw.Truncate)+" "+tw.ColorClass(subTone)), Attr("title", subline), subline),
	)
}

// dashBudgetRowProps configures one budget row in the dashboard budgets widget.
type dashBudgetRowProps struct {
	Label   string
	Percent int
	Tone    string // color class for the figure, e.g. "text-down"
	Bar     string // progress bar tone class, e.g. "bg-down"
	Over    bool   // true when the budget is over-limit (drives the drill link)
}

// dashBudgetRow renders one budget progress row in the dashboard budgets
// widget. When the budget is over its limit the row is a button that navigates
// to /budgets so the user can act immediately. Its own component so the
// navigate hook stays at a stable position across the variable-length list
// (the On* loop gotcha).
func dashBudgetRow(props dashBudgetRowProps) ui.Node {
	nav := router.UseNavigate()
	openBudgets := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/budgets")) })

	header := Div(css.Class(tw.Flex, tw.JustifyBetween),
		Span(props.Label),
		Span(ClassStr("fig "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(props.Tone)), fmt.Sprintf("%d%%", props.Percent)),
	)
	bar := uiw.ProgressBar(uiw.ProgressBarProps{Percent: props.Percent, Tone: props.Bar, Class: "mt-1.5"})

	if props.Over {
		// Over-budget rows are actionable: clicking opens the Budgets screen.
		return Button(css.Class("budget-over-row", tw.WFull, tw.TextLeft),
			Type("button"),
			Attr("aria-label", uistate.T("dashboard.budgetDrillTitle")),
			Attr("title", uistate.T("dashboard.budgetDrillTitle")),
			OnClick(openBudgets),
			header, bar,
		)
	}
	return Div(header, bar)
}

// anomalyHubRowProps carries one SMART anomaly finding to its per-row component.
type anomalyHubRowProps struct {
	Insight smart.Insight
	Route   string
	OnClick func()
}

// anomalyHubRow renders one flagged-activity row on the dashboard anomaly-hub
// widget. It is its own component so OnClick registers at a stable hook position
// across the list (no On* in loops). Reuses the same visual treatment as
// SmartAnomalyInsightRow on /insights.
func anomalyHubRow(p anomalyHubRowProps) ui.Node {
	navigate := ui.UseEvent(func() { p.OnClick() })
	iconName := icon.AlertTriangle
	if p.Insight.Severity == smart.SeverityInfo {
		iconName = icon.AlertCircle
	}
	return Button(
		css.Class("insight-row insight-row-action"),
		Type("button"),
		Attr("aria-label", p.Insight.Title),
		OnClick(navigate),
		Span(ClassStr("insight-dot text-down"), uiw.Icon(iconName, css.Class(tw.W4, tw.H4))),
		// WFull + default stretch (no ItemsStart) so the Truncate/LineClamp
		// children fill the row and don't overflow the card.
		Div(css.Class(tw.Flex, tw.FlexCol, tw.MinW0, tw.WFull),
			Span(css.Class(tw.Text14, tw.FontMedium, tw.Truncate), p.Insight.Title),
			Span(css.Class("muted", tw.Text13, tw.LineClamp2), p.Insight.Detail),
		),
	)
}

// anomalyHubViewAllProps carries the navigation callback to the drill-through button.
type anomalyHubViewAllProps struct {
	OnClick func()
}

// anomalyHubViewAll is the "View full analysis" link at the bottom of the widget.
// Its own component keeps the navigate hook at a stable position outside any loop.
func anomalyHubViewAll(p anomalyHubViewAllProps) ui.Node {
	open := ui.UseEvent(func() { p.OnClick() })
	return Button(
		css.Class("btn-link t-caption", tw.Mt2, tw.SelfStart),
		Type("button"),
		Attr("aria-label", uistate.T("dashboard.anomalyHubViewAllAria")),
		OnClick(open),
		uistate.T("dashboard.anomalyHubViewAll"),
	)
}

// anomalyHubWidget is the R25 "Flagged activity" dashboard tile. It runs the four
// anomaly-type SMART detectors (SMART-A1 balance, SMART-T2 duplicates, SMART-T6
// spending spikes, SMART-T7 missing transaction) unconditionally — no Smart opt-in
// gate — and surfaces the top 1–3 findings as a compact bento widget. Drill-through
// navigates to /insights for the full analysis. Returns a full bento tile.
func anomalyHubWidget(app *appstate.App) ui.Node {
	nav := router.UseNavigate()

	const widgetID = "anomaly-hub"
	const maxRows = 3

	pr := uistate.UsePrefs().Get()
	flagged := runAnomalyDetectors(app, pr.WeekStartWeekday())

	// Cap at maxRows so the widget stays glanceable.
	if len(flagged) > maxRows {
		flagged = flagged[:maxRows]
	}

	toInsights := func() { nav.Navigate(uistate.RoutePath("/insights")) }

	var body ui.Node
	if len(flagged) == 0 {
		body = Div(
			P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.anomalyHubClear")),
			ui.CreateElement(anomalyHubViewAll, anomalyHubViewAllProps{OnClick: toInsights}),
		)
	} else {
		rows := make([]ui.Node, 0, len(flagged))
		for _, ins := range flagged {
			route := "/transactions"
			if ins.Page == smart.PageAccounts {
				route = "/accounts"
			}
			capturedIns := ins
			capturedRoute := route
			rows = append(rows, ui.CreateElement(anomalyHubRow, anomalyHubRowProps{
				Insight: capturedIns,
				Route:   capturedRoute,
				OnClick: func() { nav.Navigate(uistate.RoutePath(capturedRoute)) },
			}))
		}
		body = Div(
			P(css.Class("t-caption", tw.TextDim, tw.Mb2), uistate.T("dashboard.anomalyHubHint")),
			Div(css.Class("insight-list"), rows),
			ui.CreateElement(anomalyHubViewAll, anomalyHubViewAllProps{OnClick: toInsights}),
		)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: widgetID, Title: uistate.T("dashboard.anomalyHubTitle"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "11",
		Body: body,
	})
}

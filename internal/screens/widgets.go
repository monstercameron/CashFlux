// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetstyle"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Bento cell geometry, mirrored from the dashboard grid (.bento --cell + gap) so
// the builder stage previews a tile at its true on-dashboard proportions.
const dashCellPx, dashGapPx = 152, 10

// WidgetBuilder is the widget-creation screen. It is laid out top-to-bottom: a
// stage rendering a live preview tile at its true bento proportions, an n8n-style
// pipeline canvas (data source → transform → visualization), and a size control
// (width 1–4, height 1–3). Per-step configuration and real source/transform/viz
// values land in later phases; for now the stage shows a sample tile and the
// pipeline nodes carry placeholder summaries.
func WidgetBuilder() ui.Node {
	col := ui.UseState(1)
	row := ui.UseState(1)
	c, r := col.Get(), row.Get()

	// Which pipeline step is selected for configuration (its panel comes later).
	active := ui.UseState(wbStepSource)

	// Faithful tile footprint: N cells plus the gaps between them.
	span := func(n int) string {
		return strconv.Itoa(n*dashCellPx+(n-1)*dashGapPx) + "px"
	}
	setCol := func(n int) { col.Set(clampSpan(n, dashMaxColSpan)) }
	setRow := func(n int) { row.Set(clampSpan(n, dashMaxRowSpan)) }

	return Div(css.Class("wb"),
		uiw.Card(uiw.CardProps{
			Header: H3(css.Class("card-title"), uistate.T("widgetBuilder.stageTitle")),
			Body: Div(css.Class("wb-stage"),
				Div(css.Class("w wb-tile"), Style(map[string]string{"width": span(c), "height": span(r)}),
					Div(css.Class("wh"),
						Span(css.Class("grip"), Attr("aria-hidden", "true"), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
						H3(uistate.T("widgetBuilder.sampleTitle")),
					),
					Div(css.Class("wbody"),
						Div(css.Class("fig t-figure", tw.FontDisplay), "$12,480"),
						P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("widgetBuilder.sampleSub")),
					),
				),
			),
		}),
		uiw.Card(uiw.CardProps{
			Header: H3(css.Class("card-title"), uistate.T("widgetBuilder.pipelineTitle")),
			Body: Fragment(
				P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("widgetBuilder.pipelineHint")),
				wbPipeline(active.Get(), func(step string) { active.Set(step) }),
			),
		}),
		uiw.Card(uiw.CardProps{
			Header: H3(css.Class("card-title"), uistate.T("widgetBuilder.sizeTitle")),
			Body: Fragment(
				P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("widgetBuilder.sizeHint")),
				Div(css.Class("wb-size"),
					wmStepper("W", c, uistate.T("widget.narrower"), uistate.T("widget.wider"),
						func() { setCol(c - 1) }, func() { setCol(c + 1) }),
					wmStepper("H", r, uistate.T("widget.shorter"), uistate.T("widget.taller"),
						func() { setRow(r - 1) }, func() { setRow(r + 1) }),
				),
			),
		}),
	)
}

// Pipeline step ids, in left-to-right flow order. Each step's output feeds the
// next: a data source → an optional transform → a visualization.
const (
	wbStepSource    = "source"
	wbStepTransform = "transform"
	wbStepVisualize = "visualize"
)

// wbPipeline renders the n8n-style node canvas: a horizontal flow of step nodes
// joined by directed connectors. Clicking a node selects it (active) so its
// configuration panel can open; the per-step config + real source/transform/viz
// values land in later phases — for now the nodes show placeholder summaries.
func wbPipeline(active string, onSelect func(string)) ui.Node {
	steps := []wbNodeProps{
		{Step: wbStepSource, Title: uistate.T("widgetBuilder.nodeSource"), Value: uistate.T("widgetBuilder.nodeSourceVal")},
		{Step: wbStepTransform, Title: uistate.T("widgetBuilder.nodeTransform"), Value: uistate.T("widgetBuilder.nodeTransformVal")},
		{Step: wbStepVisualize, Title: uistate.T("widgetBuilder.nodeVisualize"), Value: uistate.T("widgetBuilder.nodeVisualizeVal")},
	}

	flow := make([]ui.Node, 0, len(steps)*2-1)
	for i, s := range steps {
		if i > 0 {
			flow = append(flow, Div(css.Class("wb-edge"), Attr("aria-hidden", "true")))
		}
		s := s
		s.Active = s.Step == active
		s.OnSelect = onSelect
		flow = append(flow, ui.CreateElement(wbPipelineNode, s))
	}
	return Div(css.Class("wb-canvas"), Attr("role", "list"), flow)
}

type wbNodeProps struct {
	Step     string
	Title    string
	Value    string
	Active   bool
	OnSelect func(string)
}

// wbPipelineNode is one pipeline step card with an inbound/outbound port. It is
// its own component so its click hook stays at a stable position (the On*-hooks
// rule), mirroring the manager's row components.
func wbPipelineNode(p wbNodeProps) ui.Node {
	cls := "wb-node"
	if p.Active {
		cls += " is-active"
	}
	step := p.Step
	on := ui.UseEvent(func() {
		if p.OnSelect != nil {
			p.OnSelect(step)
		}
	})
	return Button(ClassStr(cls), Type("button"), Attr("role", "listitem"),
		Attr("aria-pressed", boolAttr(p.Active)), OnClick(on),
		Span(css.Class("wb-port wb-port-in"), Attr("aria-hidden", "true")),
		Div(css.Class("wb-node-body"),
			Span(css.Class("wb-node-kind"), p.Title),
			Span(css.Class("wb-node-val"), p.Value),
		),
		Span(css.Class("wb-port wb-port-out"), Attr("aria-hidden", "true")),
	)
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

const dashMaxColSpan, dashMaxRowSpan = 4, 3

// wmanFlash scrolls the ledger row for a widget into view and flashes it — the
// landing action for the board-map tiles, so a click on the map always answers
// "where is this widget's row?".
func wmanFlash(id string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	el := doc.Call("querySelector", `[data-wmrow="`+id+`"]`)
	if !el.Truthy() {
		return
	}
	el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "center"})
	el.Get("classList").Call("add", "is-flash")
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		el.Get("classList").Call("remove", "is-flash")
		return nil
	})
	js.Global().Call("setTimeout", cb, 1400)
}

// WidgetManager is the Studio "Manage widgets" surface (and /widget-manager):
// a from-scratch arrangement deck — the widget ledger (visibility, size, order)
// beside a live, true-to-span board map of the dashboard, with the tile style
// studio beneath. Every change writes the same shared atoms the dashboard reads
// (layout items + hidden set + per-widget config), so edits apply live.
func WidgetManager() ui.Node {
	itemsAtom := uistate.UseLayoutItems()
	hiddenAtom := uistate.UseHiddenWidgets()
	list := itemsAtom.Get()
	hidden := hiddenAtom.Get()

	setItems := func(next []dashlayout.Item) {
		itemsAtom.Set(next)
		uistate.PersistItems(next)
	}
	setHidden := func(next widgetvis.Set) {
		hiddenAtom.Set(next)
		uistate.PersistHiddenWidgets(next)
	}

	showAll := ui.UseEvent(func() { setHidden(widgetvis.Set{}) })
	hideAll := ui.UseEvent(func() {
		next := widgetvis.Set{}
		for _, it := range list {
			next = next.With(it.ID, true)
		}
		setHidden(next)
	})

	hiddenCount := 0
	for _, it := range list {
		if hidden.IsHidden(it.ID) {
			hiddenCount++
		}
	}

	// The ledger always reads in live dashboard order — the order IS the thing
	// being edited, so a sortable table view would only obscure it.
	rows := MapKeyed(list,
		func(it dashlayout.Item) any { return it.ID },
		func(it dashlayout.Item) ui.Node {
			idx := 0
			for i := range list {
				if list[i].ID == it.ID {
					idx = i
					break
				}
			}
			return ui.CreateElement(widgetManagerRow, widgetManagerRowProps{
				Item:        it,
				Index:       idx,
				Total:       len(list),
				Hidden:      hidden.IsHidden(it.ID),
				OnToggleVis: func() { setHidden(hidden.Toggle(it.ID)) },
				OnUp:        func() { setItems(dashlayout.Move(list, it.ID, idx-1)) },
				OnDown:      func() { setItems(dashlayout.Move(list, it.ID, idx+1)) },
				OnResize:    func(col, row int) { setItems(dashlayout.ResizeItem(list, it.ID, col, row)) },
			})
		},
	)

	// The board map: every widget at its true span on a 4-column grid, in live
	// order — hidden tiles ghost out. Clicking a tile finds its ledger row.
	mapTiles := MapKeyed(list,
		func(it dashlayout.Item) any { return it.ID },
		func(it dashlayout.Item) ui.Node {
			return ui.CreateElement(wmanMapTile, wmanMapTileProps{
				Item: it, Name: widgetDisplayName(it.ID), Hidden: hidden.IsHidden(it.ID),
			})
		},
	)

	masthead := Div(css.Class("wman-head"),
		Span(css.Class("studio-eyebrow"), uistate.T("wman.eyebrow")),
		H2(css.Class("studio-design-title"), uistate.T("wman.title")),
		P(css.Class("studio-design-sub"), uistate.T("wman.lede")),
	)

	toolbar := Div(css.Class("wman-toolbar"),
		DashboardLayoutControls(),
		Button(css.Class("data-btn"), Type("button"), OnClick(showAll), uistate.T("widgetManager.showAll")),
		Button(css.Class("data-btn"), Type("button"), OnClick(hideAll), uistate.T("widgetManager.hideAll")),
		Span(css.Class("wman-count"), uistate.T("wman.visibleCount", len(list)-hiddenCount, len(list))),
	)

	board := Div(css.Class("wman-aside"),
		Span(css.Class("wman-aside-label"), uistate.T("wman.mapLabel")),
		Div(css.Class("wman-map"), Attr("role", "list"), Attr("aria-label", uistate.T("wman.mapLabel")), mapTiles),
		P(css.Class("wman-map-hint"), uistate.T("wman.mapHint")),
	)

	return Div(css.Class("wman"),
		masthead,
		Div(css.Class("wman-grid"),
			Div(css.Class("wman-main"), toolbar, Div(css.Class("wman-ledger"), rows)),
			board,
		),
		Div(css.Class("wman-section"),
			H3(css.Class("wman-section-title"), uistate.T("widgetManager.styleTitle")),
			P(css.Class("wman-section-lede"), uistate.T("widgetManager.styleHint")),
			ui.CreateElement(tileStyleEditor, struct{}{}),
		),
	)
}

type wmanMapTileProps struct {
	Item   dashlayout.Item
	Name   string
	Hidden bool
}

// wmanMapTile is one board-map tile: a button spanning the widget's true
// columns/rows that jumps to (and flashes) the widget's ledger row. Its own
// component so the click hook is stable per tile.
func wmanMapTile(p wmanMapTileProps) ui.Node {
	itID := p.Item.ID
	jump := ui.UseEvent(Prevent(func() { wmanFlash(itID) }))
	c, r := clampSpan(p.Item.ColSpan, dashMaxColSpan), clampSpan(p.Item.RowSpan, dashMaxRowSpan)
	cls := "wman-map-tile"
	if p.Hidden {
		cls += " is-hidden"
	}
	return Button(ClassStr(cls), Type("button"), Attr("role", "listitem"),
		Attr("title", p.Name), Attr("aria-label", uistate.T("wman.jumpTo", p.Name)),
		Style(map[string]string{
			"grid-column": "span " + strconv.Itoa(c),
			"grid-row":    "span " + strconv.Itoa(r),
		}),
		OnClick(jump),
		Span(css.Class("wman-map-name"), p.Name),
	)
}

// tileStyleEditor styles tiles: pick a target ("All widgets" for the global tile
// default, or one widget to override it), tweak colors / font / weight / shape /
// border / shadow, and watch a live preview tile. Saves to the same per-widget
// config store the dashboard reads, so the real tiles update live.
func tileStyleEditor(struct{}) ui.Node {
	cfgAtom := uistate.UseWidgetConfigs()
	all := cfgAtom.Get()
	target := ui.UseState(widgetstyle.GlobalID)
	tid := target.Get()
	cfg := all.For(tid)

	set := func(key, val string) {
		next := all.WithField(tid, key, val)
		cfgAtom.Set(next)
		uistate.PersistWidgetConfigs(next)
	}
	reset := ui.UseEvent(func() {
		next := all
		for _, k := range widgetstyle.Keys {
			next = next.WithField(tid, k, "")
		}
		cfgAtom.Set(next)
		uistate.PersistWidgetConfigs(next)
	})
	onTarget := ui.UseEvent(func(e ui.Event) { target.Set(e.GetValue()) })

	// Preview = the effective style: the global default, overlaid by this widget's
	// overrides when a specific widget is selected.
	eff := cfg
	if tid != widgetstyle.GlobalID {
		eff = widgetstyle.Effective(all.For(widgetstyle.GlobalID), cfg)
	}
	preview := widgetstyle.InlineStyle(eff)

	targetOpts := []ui.Node{Option(Value(widgetstyle.GlobalID), SelectedIf(tid == widgetstyle.GlobalID), uistate.T("widgetManager.allWidgets"))}
	for _, it := range uistate.UseLayoutItems().Get() {
		targetOpts = append(targetOpts, Option(Value(it.ID), SelectedIf(tid == it.ID), widgetDisplayName(it.ID)))
	}

	color := func(label, key string) ui.Node {
		return ui.CreateElement(styleColorRow, styleColorProps{Label: label, Value: cfg[key], OnSet: func(v string) { set(key, v) }})
	}
	sel := func(label, key string, opts []styleOpt) ui.Node {
		return ui.CreateElement(styleSelectRow, styleSelectProps{Label: label, Value: cfg[key], Options: opts, OnSet: func(v string) { set(key, v) }})
	}

	return Div(css.Class("wm-style"),
		Div(css.Class("wm-style-left"),
			Div(css.Class("wm-style-row"),
				Span(css.Class("wm-style-label"), uistate.T("widgetManager.styleTarget")),
				Select(css.Class("set-input"), Attr("aria-label", uistate.T("widgetManager.styleTarget")), OnChange(onTarget), targetOpts),
			),
			Div(css.Class("wm-style-grid"),
				color(uistate.T("widgetManager.styleBg"), widgetstyle.KeyBg),
				color(uistate.T("widgetManager.styleText"), widgetstyle.KeyText),
				color(uistate.T("widgetManager.styleBorderColor"), widgetstyle.KeyBorder),
				color(uistate.T("widgetManager.styleAccent"), widgetstyle.KeyAccent),
				sel(uistate.T("widgetManager.styleBorderW"), widgetstyle.KeyBorderW, borderWidthOpts()),
				sel(uistate.T("widgetManager.styleRadius"), widgetstyle.KeyRadius, radiusOpts()),
				sel(uistate.T("widgetManager.styleFont"), widgetstyle.KeyFont, fontOpts()),
				sel(uistate.T("widgetManager.styleWeight"), widgetstyle.KeyWeight, weightOpts()),
				sel(uistate.T("widgetManager.styleShadow"), widgetstyle.KeyShadow, shadowOpts()),
			),
			Button(css.Class("data-btn", tw.Mt3), Type("button"), OnClick(reset), uistate.T("widgetManager.resetStyle")),
		),
		Div(css.Class("wm-style-preview"),
			Span(css.Class("wm-preview-label"), uistate.T("widgetManager.preview")),
			Div(css.Class("w wm-preview-tile"), Style(preview),
				Div(css.Class("wh"), Span(css.Class("wtitle"), uistate.T("widgetManager.previewTitle"))),
				Div(css.Class("wbody"),
					Div(css.Class("fig t-figure", tw.FontDisplay), "$12,480"),
					P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("widgetManager.previewSub")),
				),
			),
		),
	)
}

type styleOpt struct{ Value, Label string }

func borderWidthOpts() []styleOpt {
	return []styleOpt{{"", uistate.T("widgetManager.styleDefault")}, {"0", uistate.T("widgetManager.styleNone")}, {"1", "1px"}, {"2", "2px"}, {"3", "3px"}}
}
func radiusOpts() []styleOpt {
	return []styleOpt{{"", uistate.T("widgetManager.styleDefault")}, {"0", uistate.T("widgetManager.shapeSquare")}, {"6", uistate.T("widgetManager.shapeSmall")}, {"10", uistate.T("widgetManager.shapeMedium")}, {"16", uistate.T("widgetManager.shapeLarge")}, {"24", uistate.T("widgetManager.shapeRound")}}
}
func fontOpts() []styleOpt {
	return []styleOpt{{"", uistate.T("widgetManager.styleDefault")}, {"sans", uistate.T("widgetManager.fontSans")}, {"display", uistate.T("widgetManager.fontDisplay")}, {"mono", uistate.T("widgetManager.fontMono")}}
}
func weightOpts() []styleOpt {
	return []styleOpt{{"", uistate.T("widgetManager.styleDefault")}, {"400", uistate.T("widgetManager.weightRegular")}, {"500", uistate.T("widgetManager.weightMedium")}, {"600", uistate.T("widgetManager.weightSemibold")}, {"700", uistate.T("widgetManager.weightBold")}}
}
func shadowOpts() []styleOpt {
	return []styleOpt{{"", uistate.T("widgetManager.styleDefault")}, {"none", uistate.T("widgetManager.styleNone")}, {"soft", uistate.T("widgetManager.shadowSoft")}, {"strong", uistate.T("widgetManager.shadowStrong")}}
}

type styleColorProps struct {
	Label string
	Value string
	OnSet func(string)
}

// styleColorRow is a labeled color picker with a clear-to-theme button. Its own
// component so the color input's change hook stays at a stable position.
func styleColorRow(p styleColorProps) ui.Node {
	on := ui.UseEvent(func(e ui.Event) {
		if p.OnSet != nil {
			p.OnSet(e.GetValue())
		}
	})
	val := p.Value
	if val == "" {
		val = "#888888"
	}
	return Div(css.Class("wm-style-row"),
		Span(css.Class("wm-style-label"), p.Label),
		Div(css.Class("wm-style-color"),
			Input(Type("color"), css.Class("wm-color"), Value(val), Attr("aria-label", p.Label), OnChange(on)),
			If(p.Value != "", Button(css.Class("wm-clear"), Type("button"), Attr("aria-label", uistate.T("widgetManager.clearStyle")), OnClick(func() {
				if p.OnSet != nil {
					p.OnSet("")
				}
			}), "×")),
		),
	)
}

type styleSelectProps struct {
	Label   string
	Value   string
	Options []styleOpt
	OnSet   func(string)
}

// styleSelectRow is a labeled select. Its own component so the select's change
// hook stays at a stable position across the editor's fixed control set.
func styleSelectRow(p styleSelectProps) ui.Node {
	on := ui.UseEvent(func(e ui.Event) {
		if p.OnSet != nil {
			p.OnSet(e.GetValue())
		}
	})
	opts := make([]ui.Node, 0, len(p.Options))
	for _, o := range p.Options {
		opts = append(opts, Option(Value(o.Value), SelectedIf(p.Value == o.Value), o.Label))
	}
	return Div(css.Class("wm-style-row"),
		Span(css.Class("wm-style-label"), p.Label),
		Select(css.Class("set-input wm-style-select"), Attr("aria-label", p.Label), OnChange(on), opts),
	)
}

type widgetManagerRowProps struct {
	Item        dashlayout.Item
	Index       int
	Total       int
	Hidden      bool
	OnToggleVis func()
	OnUp        func()
	OnDown      func()
	OnResize    func(col, row int)
}

// widgetManagerRow is one widget's ledger row: live order number, name (+ a
// Hidden tag), size steppers (value at rest, steppers on hover/focus — §8.4
// row-action density), reorder arrows, and the visibility switch. Its own
// component so the several event hooks stay at stable positions across the
// list (the On* loop gotcha). Carries data-wmrow so the board map can find it.
func widgetManagerRow(props widgetManagerRowProps) ui.Node {
	it := props.Item
	col, row := it.ColSpan, it.RowSpan
	if col < 1 {
		col = 1
	}
	if row < 1 {
		row = 1
	}

	resize := func(c, r int) {
		c = clampSpan(c, dashMaxColSpan)
		r = clampSpan(r, dashMaxRowSpan)
		if props.OnResize != nil {
			props.OnResize(c, r)
		}
	}

	rowClass := "wm-row wman-row"
	if props.Hidden {
		rowClass += " is-hidden"
	}

	return Div(ClassStr(rowClass), Attr("data-wmrow", it.ID), Attr("tabindex", "-1"),
		Span(css.Class("wman-ord"), Textf("%d", props.Index+1)),
		Div(css.Class("wman-id"),
			Span(css.Class("wm-name"), widgetDisplayName(it.ID)),
			If(props.Hidden, Span(css.Class("wman-hidden-tag"), uistate.T("wman.hiddenTag"))),
		),
		Div(css.Class("wm-col-size"),
			Div(css.Class("wm-stack"),
				Span(css.Class("wm-static"), Attr("aria-hidden", "true"), Textf("%d×%d", col, row)),
				Div(css.Class("wm-size"),
					wmStepper("W", col, uistate.T("widget.narrower"), uistate.T("widget.wider"),
						func() { resize(col-1, row) }, func() { resize(col+1, row) }),
					wmStepper("H", row, uistate.T("widget.shorter"), uistate.T("widget.taller"),
						func() { resize(col, row-1) }, func() { resize(col, row+1) }),
				),
			),
		),
		Div(css.Class("wm-reorder wman-reorder"),
			Button(css.Class("wm-arrow"), Type("button"), Attr("aria-label", uistate.T("widgetManager.moveUp")),
				DisabledIf(props.Index == 0), OnClick(func() {
					if props.OnUp != nil {
						props.OnUp()
					}
				}), "↑"),
			Button(css.Class("wm-arrow"), Type("button"), Attr("aria-label", uistate.T("widgetManager.moveDown")),
				DisabledIf(props.Index >= props.Total-1), OnClick(func() {
					if props.OnDown != nil {
						props.OnDown()
					}
				}), "↓"),
		),
		uiw.Toggle(uiw.ToggleProps{
			On:    !props.Hidden,
			Label: uistate.T("widgetManager.visible"),
			OnChange: func(bool) {
				if props.OnToggleVis != nil {
					props.OnToggleVis()
				}
			},
		}),
	)
}

// wmStepper renders a compact bordered −/value/+ size control (e.g. "− W 4 +"),
// far tighter than the full-width period StepperPill which looked stretched here.
func wmStepper(axis string, value int, prevLabel, nextLabel string, onPrev, onNext func()) ui.Node {
	return Div(css.Class("wm-step"),
		Button(css.Class("wm-step-btn"), Type("button"), Attr("aria-label", prevLabel), OnClick(onPrev), "−"),
		Span(css.Class("wm-step-val"), axis+" "+strconv.Itoa(value)),
		Button(css.Class("wm-step-btn"), Type("button"), Attr("aria-label", nextLabel), OnClick(onNext), "+"),
	)
}

func clampSpan(v, max int) int {
	if v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
}

// widgetManagerTitleKeys maps each built-in widget id to the i18n key for its
// display name, so the manager labels match the dashboard's tile titles.
var widgetManagerTitleKeys = map[string]string{
	"attention":       "dashboard.attention",
	"kpi-networth":    "dashboard.netWorth",
	"kpi-income":      "dashboard.income",
	"kpi-spending":    "dashboard.spending",
	"kpi-liabilities": "dashboard.liabilities",
	"kpi-assets":      "dashboard.assets",
	"recent":          "dashboard.recent",
	"budgets":         "nav.budgets",
	"goals":           "nav.goals",
	"todo":            "nav.todo",
	"accounts":        "nav.accounts",
	"trend":           "dashboard.netWorthTrend",
	"cashflow":        "dashboard.cashFlow",
	"savings":         "dashboard.savingsRate",
	"health":          "dashboard.healthScore",
	"breakdown":       "dashboard.breakdown",
	"bills":           "dashboard.upcomingBills",
	"freshness":       "dashboard.freshness",
	"highlight":       "dashboard.highlight",
	"kpi-safetospend": "dashboard.safeToSpend",
}

// widgetDisplayName resolves a widget id to its human title: the built-in
// registry first, then a published builder card's own name ("wb:<name>"), then
// a Studio-designed widget's spec title ("us:<id>"), falling back to the raw
// id only for something truly unknown.
func widgetDisplayName(id string) string {
	if key, ok := widgetManagerTitleKeys[id]; ok {
		return uistate.T(key)
	}
	if name, ok := strings.CutPrefix(id, vbCardPrefix); ok {
		return name
	}
	if strings.HasPrefix(id, userSpecPrefix) && appstate.Default != nil {
		for _, p := range appstate.Default.Placements("dashboard") {
			if p.ID == id && p.Spec.Title != "" {
				return p.Spec.Title
			}
		}
	}
	return id
}

//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetstyle"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// WidgetBuilder is the (placeholder) widget-creation screen: a future surface for
// composing a dashboard widget from a data source, transform, and visualization.
// Blank for now — routing + rail entry only.
func WidgetBuilder() ui.Node {
	return Section(ClassStr("card"),
		H3(ClassStr("card-title"), uistate.T("widgetBuilder.title")),
		P(ClassStr("empty"), uistate.T("widgetBuilder.empty")),
	)
}

const dashMaxColSpan, dashMaxRowSpan = 4, 3

// WidgetManager governs the dashboard's widgets in one place: show/hide each tile,
// resize it, reorder it, and control the overall arrangement. Every change writes
// the same shared atoms the dashboard reads (layout items + hidden set), so the
// dashboard reflects edits live. Styling, presets, and duplication land in later
// phases.
func WidgetManager() ui.Node {
	itemsAtom := uistate.UseLayoutItems()
	hiddenAtom := uistate.UseHiddenWidgets()
	list := itemsAtom.Get()
	hidden := hiddenAtom.Get()

	// Table sort state — defaults to the live dashboard order so reorder reads
	// naturally; sorting by another column is a view aid (it doesn't change the
	// layout, which the up/down controls own).
	sortKey := ui.UseState("order")
	sortDir := ui.UseState("asc")
	onSort := func(key string) {
		if sortKey.Get() == key {
			if sortDir.Get() == "asc" {
				sortDir.Set("desc")
			} else {
				sortDir.Set("asc")
			}
			return
		}
		sortKey.Set(key)
		sortDir.Set("asc")
	}

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

	// Build view models carrying each widget's true layout index, then sort a copy
	// for display. Reorder/resize always act on the layout index, not the row's
	// position in a sorted view.
	type rowVM struct {
		Item dashlayout.Item
		Idx  int
	}
	vms := make([]rowVM, len(list))
	for i, it := range list {
		vms[i] = rowVM{Item: it, Idx: i}
	}
	sk, dir := sortKey.Get(), sortDir.Get()
	sort.SliceStable(vms, func(a, b int) bool {
		c := 0
		switch sk {
		case "name":
			c = strings.Compare(strings.ToLower(widgetDisplayName(vms[a].Item.ID)), strings.ToLower(widgetDisplayName(vms[b].Item.ID)))
		case "visible":
			c = boolKey(hidden.IsHidden(vms[a].Item.ID)) - boolKey(hidden.IsHidden(vms[b].Item.ID)) // visible first
		case "size":
			c = spanArea(vms[a].Item) - spanArea(vms[b].Item)
		default: // "order"
			c = vms[a].Idx - vms[b].Idx
		}
		if dir == "desc" {
			return c > 0
		}
		return c < 0
	})

	rows := MapKeyed(vms,
		func(v rowVM) any { return v.Item.ID },
		func(v rowVM) ui.Node {
			it, idx := v.Item, v.Idx
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

	return Div(ClassStr("wm"),
		Section(ClassStr("card"),
			H3(ClassStr("card-title"), uistate.T("widgetManager.layoutTitle")),
			P(ClassStr("text-dim t-body mb-3"), uistate.T("widgetManager.layoutHint")),
			Div(ClassStr("wm-toolbar"),
				DashboardLayoutControls(),
				Span(ClassStr("wm-sep"), Attr("aria-hidden", "true")),
				Button(ClassStr("data-btn"), Type("button"), OnClick(showAll), uistate.T("widgetManager.showAll")),
				Button(ClassStr("data-btn"), Type("button"), OnClick(hideAll), uistate.T("widgetManager.hideAll")),
			),
		),
		Section(ClassStr("card"),
			H3(ClassStr("card-title"), uistate.T("widgetManager.widgetsTitle")),
			uiw.DataTable(uiw.DataTableProps{
				Class: "wm-table",
				Columns: []uiw.Column{
					{Label: uistate.T("widgetManager.colWidget"), SortKey: "name"},
					{Label: uistate.T("widgetManager.visible"), SortKey: "visible", Class: "wm-col-vis"},
					{Label: uistate.T("widgetManager.colSize"), SortKey: "size", Class: "wm-col-size"},
					{Label: uistate.T("widgetManager.colOrder"), SortKey: "order", Class: "wm-col-order"},
				},
				Body:   rows,
				Sort:   sk,
				Dir:    dir,
				OnSort: onSort,
			}),
		),
		Section(ClassStr("card"),
			H3(ClassStr("card-title"), uistate.T("widgetManager.styleTitle")),
			P(ClassStr("text-dim t-body mb-3"), uistate.T("widgetManager.styleHint")),
			ui.CreateElement(tileStyleEditor, struct{}{}),
		),
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

	return Div(ClassStr("wm-style"),
		Div(ClassStr("wm-style-left"),
			Div(ClassStr("wm-style-row"),
				Span(ClassStr("wm-style-label"), uistate.T("widgetManager.styleTarget")),
				Select(ClassStr("set-input"), Attr("aria-label", uistate.T("widgetManager.styleTarget")), OnChange(onTarget), targetOpts),
			),
			Div(ClassStr("wm-style-grid"),
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
			Button(ClassStr("data-btn mt-3"), Type("button"), OnClick(reset), uistate.T("widgetManager.resetStyle")),
		),
		Div(ClassStr("wm-style-preview"),
			Span(ClassStr("wm-preview-label"), uistate.T("widgetManager.preview")),
			Div(ClassStr("w wm-preview-tile"), Style(preview),
				Div(ClassStr("wh"), Span(ClassStr("wtitle"), uistate.T("widgetManager.previewTitle"))),
				Div(ClassStr("wbody"),
					Div(ClassStr("font-display fig t-figure"), "$12,480"),
					P(ClassStr("t-caption text-dim mt-1"), uistate.T("widgetManager.previewSub")),
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
	return Div(ClassStr("wm-style-row"),
		Span(ClassStr("wm-style-label"), p.Label),
		Div(ClassStr("wm-style-color"),
			Input(Type("color"), ClassStr("wm-color"), Value(val), Attr("aria-label", p.Label), OnChange(on)),
			If(p.Value != "", Button(ClassStr("wm-clear"), Type("button"), Attr("aria-label", uistate.T("widgetManager.clearStyle")), OnClick(func() {
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
	return Div(ClassStr("wm-style-row"),
		Span(ClassStr("wm-style-label"), p.Label),
		Select(ClassStr("set-input wm-style-select"), Attr("aria-label", p.Label), OnChange(on), opts),
	)
}

func boolKey(b bool) int {
	if b {
		return 1
	}
	return 0
}

func spanArea(it dashlayout.Item) int {
	c, r := it.ColSpan, it.RowSpan
	if c < 1 {
		c = 1
	}
	if r < 1 {
		r = 1
	}
	return c * r
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

// widgetManagerRow is one widget's table row: name, a visibility switch, size
// steppers, and reorder controls. Its own component so the several event hooks
// stay at stable positions across the list (the On* loop gotcha).
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

	nameClass := "wm-name"
	if props.Hidden {
		nameClass += " is-hidden"
	}

	rowClass := "wm-row"
	if props.Hidden {
		rowClass += " is-hidden"
	}

	return Tr(ClassStr(rowClass),
		Td(ClassStr("wm-cell-name"), Span(ClassStr(nameClass), widgetDisplayName(it.ID))),
		Td(ClassStr("wm-col-vis"),
			uiw.Toggle(uiw.ToggleProps{
				On:    !props.Hidden,
				Label: uistate.T("widgetManager.visible"),
				OnChange: func(bool) {
					if props.OnToggleVis != nil {
						props.OnToggleVis()
					}
				},
			}),
		),
		Td(ClassStr("wm-col-size"),
			Div(ClassStr("wm-size"),
				wmStepper("W", col, uistate.T("widget.narrower"), uistate.T("widget.wider"),
					func() { resize(col-1, row) }, func() { resize(col+1, row) }),
				wmStepper("H", row, uistate.T("widget.shorter"), uistate.T("widget.taller"),
					func() { resize(col, row-1) }, func() { resize(col, row+1) }),
			),
		),
		Td(ClassStr("wm-col-order"),
			Div(ClassStr("wm-reorder"),
				Button(ClassStr("wm-arrow"), Type("button"), Attr("aria-label", uistate.T("widgetManager.moveUp")),
					DisabledIf(props.Index == 0), OnClick(func() {
						if props.OnUp != nil {
							props.OnUp()
						}
					}), "↑"),
				Button(ClassStr("wm-arrow"), Type("button"), Attr("aria-label", uistate.T("widgetManager.moveDown")),
					DisabledIf(props.Index >= props.Total-1), OnClick(func() {
						if props.OnDown != nil {
							props.OnDown()
						}
					}), "↓"),
			),
		),
	)
}

// wmStepper renders a compact bordered −/value/+ size control (e.g. "− W 4 +"),
// far tighter than the full-width period StepperPill which looked stretched here.
func wmStepper(axis string, value int, prevLabel, nextLabel string, onPrev, onNext func()) ui.Node {
	return Div(ClassStr("wm-step"),
		Button(ClassStr("wm-step-btn"), Type("button"), Attr("aria-label", prevLabel), OnClick(onPrev), "−"),
		Span(ClassStr("wm-step-val"), axis+" "+strconv.Itoa(value)),
		Button(ClassStr("wm-step-btn"), Type("button"), Attr("aria-label", nextLabel), OnClick(onNext), "+"),
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
	"recent":          "dashboard.recent",
	"budgets":         "nav.budgets",
	"goals":           "nav.goals",
	"todo":            "nav.todo",
	"accounts":        "nav.accounts",
	"trend":           "dashboard.netWorthTrend",
	"cashflow":        "dashboard.cashFlow",
	"savings":         "dashboard.savingsRate",
	"breakdown":       "dashboard.breakdown",
	"bills":           "dashboard.upcomingBills",
	"freshness":       "dashboard.freshness",
	"highlight":       "dashboard.highlight",
}

// widgetDisplayName resolves a widget id to its human title, falling back to the
// raw id for anything unmapped (e.g. a future duplicated instance).
func widgetDisplayName(id string) string {
	if key, ok := widgetManagerTitleKeys[id]; ok {
		return uistate.T(key)
	}
	return id
}

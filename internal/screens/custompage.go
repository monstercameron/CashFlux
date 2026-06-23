//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/pages"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetdata"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// pageCtx is the precomputed data every widget on a page draws from: the engine
// variable surface plus the raw entity slices and currency context. Built once
// per render and passed to each tile, so widgets stay cheap and consistent.
type pageCtx struct {
	Vars  map[string]float64
	App   *appstate.App
	Rates currency.Rates
	Base  string
}

// CustomPage renders a user-authored page resolved by its slug: a toolbar to add
// widgets, then the page's bento grid of custom widgets bound to the app engine.
// A slug with no matching page shows a friendly not-found message.
func CustomPage(slug string) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	page, ok := pages.BySlug(app.CustomPages(), slug)
	if !ok {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("pages.notFound"))})
	}

	// A version counter forces a re-render after a mutation (add/delete/resize/
	// reorder widget) that doesn't change the route.
	version := ui.UseState(0)
	_ = version.Get()
	refresh := func() { version.Set(version.Get() + 1) }

	// dragSrc holds the id of the widget being dragged, so a drop on another tile
	// can reorder the page's layout. Held here so it survives across the tiles.
	dragSrc := ui.UseState("")

	// reorderWidget moves the dragged widget in front of the drop target in the
	// page's layout, then persists. resizeWidget cycles a widget's width/height.
	reorderWidget := func(targetID string) {
		src := dragSrc.Get()
		dragSrc.Set("")
		if src == "" || src == targetID {
			return
		}
		pg, ok := pages.ByID(app.CustomPages(), page.ID)
		if !ok {
			return
		}
		pg.Layout = ensureLayout(pg)
		pg.Layout = dashlayout.Move(pg.Layout, src, layoutIndex(pg.Layout, targetID))
		_ = app.PutCustomPage(pg)
		refresh()
	}
	resizeWidget := func(id string, widthAxis bool) {
		pg, ok := pages.ByID(app.CustomPages(), page.ID)
		if !ok {
			return
		}
		pg.Layout = ensureLayout(pg)
		for _, it := range pg.Layout {
			if it.ID != id {
				continue
			}
			col, row := it.ColSpan, it.RowSpan
			if widthAxis {
				col = dashlayout.CycleSpan(col, 4, false)
			} else {
				row = dashlayout.CycleSpan(row, 3, false)
			}
			pg.Layout = dashlayout.ResizeItem(pg.Layout, id, col, row)
			break
		}
		_ = app.PutCustomPage(pg)
		refresh()
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	ctx := pageCtx{
		Vars: engineenv.Vars(engineenv.Data{
			Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
			Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Rates: rates, Now: time.Now(),
		}),
		App: app, Rates: rates, Base: base,
	}

	toolbar := ui.CreateElement(addWidgetBar, addWidgetBarProps{PageID: page.ID, Refresh: refresh})

	if len(page.Widgets) == 0 {
		return Div(
			toolbar,
			uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("pages.empty"))}),
		)
	}

	// Place widgets on the 4-column grid from the page's saved layout (falling back
	// to a default size for any widget missing a layout entry).
	items := page.Layout
	if len(items) == 0 {
		for _, w := range page.Widgets {
			items = append(items, dashlayout.Item{ID: w.ID, ColSpan: 1, RowSpan: 1})
		}
	}
	layout := dashlayout.Pack(items, 4)

	tiles := make([]ui.Node, 0, len(page.Widgets))
	for _, w := range page.Widgets {
		w := w
		p, has := layout.Get(w.ID)
		col, row := "auto", "auto"
		if has {
			col, row = p.GridColumn(), p.GridRow()
		}
		id := w.ID
		tiles = append(tiles, ui.CreateElement(customTile, customTileProps{
			PageID: page.ID, Widget: w, Ctx: ctx, GridColumn: col, GridRow: row, Refresh: refresh,
			OnDragStart: func() { dragSrc.Set(id) },
			OnDrop:      func() { reorderWidget(id) },
			OnResizeW:   func() { resizeWidget(id, true) },
			OnResizeH:   func() { resizeWidget(id, false) },
		}))
	}

	return Div(
		toolbar,
		Div(css.Class("bento"), tiles),
	)
}

type customTileProps struct {
	PageID      string
	Widget      domain.PageWidget
	Ctx         pageCtx
	GridColumn  string
	GridRow     string
	Refresh     func()
	OnDragStart func()
	OnDrop      func()
	OnResizeW   func()
	OnResizeH   func()
}

// customTile renders one widget instance as a bento cell. Its header is a drag
// handle (drop onto another tile to reorder), with width/height resize buttons, an
// edit toggle, and a delete button; the body is the type's renderer, or an inline
// edit form while editing. It's its own component so its hooks (edit state, click
// handlers) stay stable across the widget list.
func customTile(props customTileProps) ui.Node {
	w := props.Widget
	pageID := props.PageID
	editing := ui.UseState(false)

	del := func() {
		app := appstate.Default
		if app == nil {
			return
		}
		page, ok := pages.ByID(app.CustomPages(), pageID)
		if !ok {
			return
		}
		page.Widgets = removeWidget(page.Widgets, w.ID)
		page.Layout = removeLayoutItem(page.Layout, w.ID)
		_ = app.PutCustomPage(page)
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	title := w.Title
	if title == "" {
		title = widgetTypeLabel(w.Type)
	}

	// The header doubles as the drag handle for reordering.
	header := Div(css.Class("wh"),
		Attr("draggable", "true"),
		OnDragStart(func() {
			if props.OnDragStart != nil {
				props.OnDragStart()
			}
		}),
		OnDragOver(Prevent(func() {})),
		OnDrop(Prevent(func() {
			if props.OnDrop != nil {
				props.OnDrop()
			}
		})),
		Span(css.Class("grip", tw.CursorGrab), Attr("aria-label", "Drag to reorder"), Attr("role", "button"), "⠿"),
		H2(title),
		Button(css.Class("gear-inline"), Type("button"), Title(uistate.T("pages.resizeWidth")), Attr("aria-label", uistate.T("pages.resizeWidth")),
			OnClick(func() {
				if props.OnResizeW != nil {
					props.OnResizeW()
				}
			}), "↔"),
		Button(css.Class("gear-inline"), Type("button"), Title(uistate.T("pages.resizeHeight")), Attr("aria-label", uistate.T("pages.resizeHeight")),
			OnClick(func() {
				if props.OnResizeH != nil {
					props.OnResizeH()
				}
			}), "↕"),
		Button(css.Class("gear-inline"), Type("button"), Attr("aria-label", uistate.T("pages.editWidget")), Title(uistate.T("pages.editWidget")),
			OnClick(func() { editing.Set(!editing.Get()) }), uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
		Button(css.Class("gear-inline"), Type("button"), Attr("aria-label", uistate.T("pages.deleteWidget")), Title(uistate.T("pages.deleteWidget")),
			OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)

	var body ui.Node
	if editing.Get() {
		body = ui.CreateElement(editWidgetForm, editWidgetFormProps{
			PageID: pageID, Widget: w, Ctx: props.Ctx,
			OnDone: func() {
				editing.Set(false)
				if props.Refresh != nil {
					props.Refresh()
				}
			},
		})
	} else {
		body = widgetBody(w, props.Ctx)
	}

	return Div(css.Class("w"),
		Attr("style", "grid-column:"+props.GridColumn+";grid-row:"+props.GridRow),
		header,
		Div(css.Class("wbody"), body),
	)
}

// layoutIndex returns the position of id in items, or 0 if absent (so a drop falls
// to the front rather than erroring).
func layoutIndex(items []dashlayout.Item, id string) int {
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return 0
}

// ensureLayout returns the page's layout, synthesizing a default 1×1 entry for any
// widget missing one (older pages or after an add), so reorder/resize always have
// a layout to operate on.
func ensureLayout(p domain.CustomPage) []dashlayout.Item {
	have := map[string]bool{}
	out := append([]dashlayout.Item(nil), p.Layout...)
	for _, it := range out {
		have[it.ID] = true
	}
	for _, w := range p.Widgets {
		if !have[w.ID] {
			out = append(out, dashlayout.Item{ID: w.ID, ColSpan: 1, RowSpan: 1})
		}
	}
	return out
}

type editWidgetFormProps struct {
	PageID string
	Widget domain.PageWidget
	Ctx    pageCtx
	OnDone func()
}

// editWidgetForm is the inline editor shown in a tile while editing: it edits the
// widget's title and its one binding (KPI formula + format, list source, text,
// or artifact), then saves the change back into the page. Its own component so its
// field hooks are stable; all hooks run unconditionally (the per-type control just
// picks which states to show).
func editWidgetForm(props editWidgetFormProps) ui.Node {
	w := props.Widget
	title := ui.UseState(w.Title)
	expr := ui.UseState(w.Binding.Expr)
	format := ui.UseState(w.Config["format"])
	source := ui.UseState(w.Binding.Source)
	text := ui.UseState(w.Config["text"])
	artifact := ui.UseState(w.Binding.ArtifactID)

	onTitle := ui.UseEvent(func(v string) { title.Set(v) })
	onExpr := ui.UseEvent(func(v string) { expr.Set(v) })
	onText := ui.UseEvent(func(v string) { text.Set(v) })
	onFormat := ui.UseEvent(func(e ui.Event) { format.Set(e.GetValue()) })
	onSource := ui.UseEvent(func(e ui.Event) { source.Set(e.GetValue()) })
	onArtifact := ui.UseEvent(func(e ui.Event) { artifact.Set(e.GetValue()) })

	// The form mounts only while editing, so focus the title field on mount so the
	// cursor lands there without a click (§6.7). The stable dep runs it just once.
	ui.UseEffect(func() func() {
		focusByID("widget-edit-" + w.ID)
		return nil
	}, w.ID)

	save := func() {
		app := appstate.Default
		if app == nil {
			return
		}
		pg, ok := pages.ByID(app.CustomPages(), props.PageID)
		if !ok {
			return
		}
		for i := range pg.Widgets {
			if pg.Widgets[i].ID != w.ID {
				continue
			}
			nw := pg.Widgets[i]
			nw.Title = title.Get()
			if nw.Config == nil {
				nw.Config = map[string]string{}
			}
			switch nw.Type {
			case widgetspec.TypeKPI:
				nw.Binding.Expr = expr.Get()
				nw.Config["format"] = format.Get()
			case widgetspec.TypeList:
				nw.Binding.Source = source.Get()
			case widgetspec.TypeText:
				nw.Config["text"] = text.Get()
			case widgetspec.TypeImage, widgetspec.TypeTable:
				nw.Binding.ArtifactID = artifact.Get()
			}
			pg.Widgets[i] = nw
			break
		}
		_ = app.PutCustomPage(pg)
		if props.OnDone != nil {
			props.OnDone()
		}
	}

	// The binding control depends on the widget type.
	var bindCtl ui.Node
	switch w.Type {
	case widgetspec.TypeKPI:
		bindCtl = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Input(css.Class("field"), Attr("placeholder", uistate.T("pages.kpiFormula")), Value(expr.Get()), OnInput(onExpr)),
			Select(css.Class("field"), OnChange(onFormat),
				Option(Value("number"), SelectedIf(format.Get() == "number"), "number"),
				Option(Value("percent"), SelectedIf(format.Get() == "percent"), "percent"),
				Option(Value("currency"), SelectedIf(format.Get() == "currency"), "currency"),
			),
		)
	case widgetspec.TypeList:
		opts := make([]ui.Node, 0)
		for _, d := range widgetspec.ListSources() {
			opts = append(opts, Option(Value(d.Type), SelectedIf(source.Get() == d.Type), d.Label))
		}
		bindCtl = Select(css.Class("field"), OnChange(onSource), opts)
	case widgetspec.TypeText:
		bindCtl = Input(css.Class("field"), Attr("placeholder", uistate.T("pages.textContent")), Value(text.Get()), OnInput(onText))
	case widgetspec.TypeImage, widgetspec.TypeTable:
		opts := []ui.Node{Option(Value(""), uistate.T("pages.chooseArtifact"))}
		if appstate.Default != nil {
			for _, a := range appstate.Default.Artifacts() {
				if w.Type == widgetspec.TypeImage && a.Kind != artifacts.KindImage {
					continue
				}
				if w.Type == widgetspec.TypeTable && a.Kind == artifacts.KindImage {
					continue
				}
				opts = append(opts, Option(Value(a.ID), SelectedIf(artifact.Get() == a.ID), a.Name))
			}
		}
		bindCtl = Select(css.Class("field"), OnChange(onArtifact), opts)
	default:
		bindCtl = P(css.Class("muted"), uistate.T("pages.chartDefault"))
	}

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		Input(css.Class("field"), Attr("id", "widget-edit-"+w.ID), Attr("placeholder", uistate.T("pages.widgetTitle")), Value(title.Get()), OnInput(onTitle)),
		bindCtl,
		Div(css.Class(tw.Flex, tw.Gap2),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(save), uistate.T("action.save")),
			Button(css.Class("btn"), Type("button"), OnClick(func() {
				if props.OnDone != nil {
					props.OnDone()
				}
			}), uistate.T("action.cancel")),
		),
	)
}

// widgetBody dispatches to the renderer for a widget's type.
func widgetBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	switch w.Type {
	case widgetspec.TypeKPI:
		return cpKPIBody(w, ctx)
	case widgetspec.TypeList:
		return listBody(w, ctx)
	case widgetspec.TypeChart:
		return chartBody(ctx)
	case widgetspec.TypeText:
		return textBody(w)
	case widgetspec.TypeImage:
		return cpImageBody(w, ctx)
	case widgetspec.TypeTable:
		return cpTableBody(w, ctx)
	default:
		return P(css.Class("empty"), uistate.T("pages.unknownWidget"))
	}
}

// cpImageBody renders an image artifact bound by ID.
func cpImageBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	art, ok := findArtifact(ctx.App.Artifacts(), w.Binding.ArtifactID)
	if !ok || art.Kind != artifacts.KindImage || len(art.Bytes) == 0 {
		return P(css.Class("empty"), uistate.T("pages.pickArtifact"))
	}
	return Img(Attr("src", artifacts.DataURL(art.MIME, art.Bytes)),
		Attr("alt", art.Name), css.Class(tw.MaxWFull, tw.MaxHFull, tw.ObjectContain, tw.MAuto))
}

// cpTableBody renders a dataset artifact (columns + first rows) bound by ID.
func cpTableBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	art, ok := findArtifact(ctx.App.Artifacts(), w.Binding.ArtifactID)
	if !ok || len(art.Columns) == 0 {
		return P(css.Class("empty"), uistate.T("pages.pickArtifact"))
	}
	head := make([]ui.Node, 0, len(art.Columns))
	for _, c := range art.Columns {
		head = append(head, Th(css.Class(tw.TextLeft, tw.Pr3, tw.TextFaint, tw.FontMedium), c))
	}
	bodyRows := make([]ui.Node, 0)
	for i, r := range art.Rows {
		if i >= 8 { // keep tiles compact; the artifact keeps the full data
			break
		}
		cells := make([]ui.Node, 0, len(r))
		for _, cell := range r {
			cells = append(cells, Td(css.Class(tw.Pr3, tw.Py05, tw.Truncate), cell))
		}
		bodyRows = append(bodyRows, Tr(cells))
	}
	return Table(css.Class("fig", tw.WFull, tw.Text12),
		Thead(Tr(head)),
		Tbody(bodyRows),
	)
}

func findArtifact(arts []domain.Artifact, id string) (domain.Artifact, bool) {
	for _, a := range arts {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Artifact{}, false
}

// cpKPIBody evaluates the widget's formula over the engine variables and shows the
// result, formatted per the widget's "format" config (number/percent/currency).
func cpKPIBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	val, err := widgetspec.EvalKPI(w.Binding.Expr, ctx.Vars)
	if err != nil {
		// Show a friendly muted placeholder rather than the raw error string, so a
		// KPI added without a formula doesn't alarm the user with developer text.
		return P(css.Class("empty"), uistate.T("pages.kpiNoFormula"))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.JustifyCenter, tw.HFull),
		Div(css.Class("fig", tw.FontDisplay, tw.Text28), widgetdata.KPIText(val, w.Config["format"], ctx.Base)),
	)
}

// listBody shows up to N rows from the widget's data source. The row data
// (source selection, newest-first ordering, formatting, cap) is computed by the
// pure internal/widgetdata package; this just renders the rows.
func listBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	rows, ok := widgetdata.ListRows(w.Binding.Source, widgetdata.Data{
		Transactions: ctx.App.Transactions(), Accounts: ctx.App.Accounts(),
		Budgets: ctx.App.Budgets(), Goals: ctx.App.Goals(), Tasks: ctx.App.Tasks(),
		Recurring: ctx.App.Recurring(), Rates: ctx.Rates, Now: time.Now(),
	}, widgetdata.DefaultListRows)
	if !ok {
		return P(css.Class("empty"), uistate.T("pages.pickSource"))
	}
	if len(rows) == 0 {
		return P(css.Class("empty"), uistate.T("pages.noData"))
	}
	nodes := make([]ui.Node, 0, len(rows))
	for _, r := range rows {
		nodes = append(nodes, Div(css.Class("row"),
			Span(css.Class("row-desc", tw.Truncate), r.Label),
			Span(css.Class("amount fig"), r.Value),
		))
	}
	return Div(css.Class("rows"), nodes)
}

// chartBody renders the net-worth trend over the last six months — a sensible
// default chart for a custom page. (A configurable series follows.)
func chartBody(ctx pageCtx) ui.Node {
	app := ctx.App
	accounts, txns := app.Accounts(), app.Transactions()
	net, _, _, _ := ledger.NetWorth(accounts, txns, ctx.Rates)
	cutoffs := widgetdata.ChartWindow(time.Now(), 6)
	series, _ := ledger.NetWorthSeries(accounts, txns, cutoffs, ctx.Rates)
	div := minorPerMajor(net.Currency)
	pts := make([]chartspec.Point, len(series))
	for i, m := range series {
		pts[i] = chartspec.Point{X: float64(i), Y: float64(m.Amount) / div}
	}
	yFmt := ".2~s"
	if currency.Symbol(net.Currency) == "$" {
		yFmt = "$.2~s"
	}
	spec := chartspec.Spec{
		Kind:   chartspec.Area,
		Series: []chartspec.Series{{Name: "Net worth", Points: pts}},
		Y:      chartspec.Axis{Format: yFmt},
	}
	return uiw.Chart(uiw.ChartProps{Spec: spec, Height: "140px", Label: uistate.T("dashboard.netWorthTrend")})
}

// textBody renders the widget's authored text.
// textBody renders the text widget's content as Markdown (C66/C32) so a note
// can carry headings, lists, emphasis, and links — not just a flat paragraph.
// The framework's Markdown escapes raw HTML and drops active URL schemes, so
// even imported page content can't smuggle an executable href.
func textBody(w domain.PageWidget) ui.Node {
	t := w.Config["text"]
	if t == "" {
		return P(css.Class("empty"), uistate.T("pages.emptyText"))
	}
	return Div(css.Class("md md-widget muted"),
		Markdown(t, MarkdownRenderOptions{LinkTarget: "_blank", LinkRel: "noopener noreferrer"}))
}

// --- small helpers ---

func widgetTypeLabel(t string) string {
	for _, d := range widgetspec.Catalog() {
		if d.Type == t {
			return d.Label
		}
	}
	return t
}

func minorPerMajor(cur string) float64 {
	div := 1.0
	for i := 0; i < currency.Decimals(cur); i++ {
		div *= 10
	}
	return div
}

func removeWidget(ws []domain.PageWidget, id string) []domain.PageWidget {
	out := ws[:0:0]
	for _, w := range ws {
		if w.ID != id {
			out = append(out, w)
		}
	}
	return out
}

func removeLayoutItem(items []dashlayout.Item, id string) []dashlayout.Item {
	out := items[:0:0]
	for _, it := range items {
		if it.ID != id {
			out = append(out, it)
		}
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func itoaPct(p int) string {
	return strconv.Itoa(p) + "%"
}

type addWidgetBarProps struct {
	PageID  string
	Refresh func()
}

// addWidgetBar is the page toolbar: an "Add widget" button that reveals a small
// form to pick a type, name it, and set its one binding (KPI formula, list
// source, or text), then appends the widget to the page. It's a single stable
// component instance so its form hooks don't run inside a loop.
func addWidgetBar(props addWidgetBarProps) ui.Node {
	open := ui.UseState(false)
	wtype := ui.UseState(widgetspec.TypeKPI)
	title := ui.UseState("")
	bind := ui.UseState("") // formula (KPI), source (List), or text (Text)

	onType := ui.UseEvent(func(v string) { wtype.Set(v) })
	onTitle := ui.UseEvent(func(v string) { title.Set(v) })
	onBind := ui.UseEvent(func(v string) { bind.Set(v) })

	addWidget := func() {
		app := appstate.Default
		if app == nil {
			return
		}
		page, ok := pages.ByID(app.CustomPages(), props.PageID)
		if !ok {
			return
		}
		w := domain.PageWidget{ID: id.New(), Type: wtype.Get(), Title: title.Get(), Config: map[string]string{}}
		switch wtype.Get() {
		case widgetspec.TypeKPI:
			w.Binding.Expr = bind.Get()
			w.Config["format"] = "number"
		case widgetspec.TypeList:
			w.Binding.Source = firstNonEmpty(bind.Get(), widgetspec.SourceTransactions)
		case widgetspec.TypeText:
			w.Config["text"] = bind.Get()
		case widgetspec.TypeImage, widgetspec.TypeTable:
			w.Binding.ArtifactID = bind.Get()
		}
		span := 1
		if wtype.Get() == widgetspec.TypeChart || wtype.Get() == widgetspec.TypeList {
			span = 2
		}
		page.Widgets = append(page.Widgets, w)
		page.Layout = append(page.Layout, dashlayout.Item{ID: w.ID, ColSpan: span, RowSpan: 1})
		if err := app.PutCustomPage(page); err != nil {
			return
		}
		title.Set("")
		bind.Set("")
		open.Set(false)
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	if !open.Get() {
		return Div(css.Class(tw.Mb3),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(func() { open.Set(true) }),
				uistate.T("pages.addWidget")),
		)
	}

	// Type selector.
	typeOpts := make([]ui.Node, 0)
	for _, d := range widgetspec.Catalog() {
		typeOpts = append(typeOpts, Option(Value(d.Type), SelectedIf(wtype.Get() == d.Type), d.Label))
	}

	// The binding control depends on the chosen type.
	var bindControl ui.Node
	switch wtype.Get() {
	case widgetspec.TypeKPI:
		bindControl = Input(css.Class("field"), Attr("placeholder", uistate.T("pages.kpiFormula")),
			Value(bind.Get()), OnInput(onBind))
	case widgetspec.TypeList:
		srcOpts := make([]ui.Node, 0)
		for _, d := range widgetspec.ListSources() {
			srcOpts = append(srcOpts, Option(Value(d.Type), SelectedIf(bind.Get() == d.Type), d.Label))
		}
		bindControl = Select(css.Class("field"), OnChange(onBind), srcOpts)
	case widgetspec.TypeText:
		bindControl = Input(css.Class("field"), Attr("placeholder", uistate.T("pages.textContent")),
			Value(bind.Get()), OnInput(onBind))
	case widgetspec.TypeImage, widgetspec.TypeTable:
		arts := appstate.Default.Artifacts()
		artOpts := []ui.Node{Option(Value(""), uistate.T("pages.chooseArtifact"))}
		for _, a := range arts {
			// Image widgets list image artifacts; Table widgets list datasets.
			if wtype.Get() == widgetspec.TypeImage && a.Kind != artifacts.KindImage {
				continue
			}
			if wtype.Get() == widgetspec.TypeTable && a.Kind == artifacts.KindImage {
				continue
			}
			artOpts = append(artOpts, Option(Value(a.ID), SelectedIf(bind.Get() == a.ID), a.Name))
		}
		bindControl = Select(css.Class("field"), OnChange(onBind), artOpts)
	default: // Chart needs no binding in Phase B
		bindControl = P(css.Class("muted"), uistate.T("pages.chartDefault"))
	}

	return Section(css.Class("card", tw.Mb3),
		Div(css.Class("form-grid"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("pages.labelType")), OnChange(onType), typeOpts),
			Input(css.Class("field"), Attr("aria-label", uistate.T("pages.widgetTitle")), Attr("placeholder", uistate.T("pages.widgetTitle")),
				Value(title.Get()), OnInput(onTitle)),
			bindControl,
		),
		Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(addWidget), uistate.T("action.add")),
			Button(css.Class("btn"), Type("button"), OnClick(func() { open.Set(false) }), uistate.T("action.cancel")),
		),
	)
}

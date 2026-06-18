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
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/pages"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetdata"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
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
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	page, ok := pages.BySlug(app.CustomPages(), slug)
	if !ok {
		return Section(Class("card"), P(Class("empty"), uistate.T("pages.notFound")))
	}

	// A version counter forces a re-render after a mutation (add/delete widget)
	// that doesn't change the route.
	version := ui.UseState(0)
	_ = version.Get()
	refresh := func() { version.Set(version.Get() + 1) }

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
			Section(Class("card"), P(Class("empty"), uistate.T("pages.empty"))),
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
		tiles = append(tiles, ui.CreateElement(customTile, customTileProps{
			PageID: page.ID, Widget: w, Ctx: ctx, GridColumn: col, GridRow: row, Refresh: refresh,
		}))
	}

	return Div(
		toolbar,
		Div(Class("bento"), tiles),
	)
}

type customTileProps struct {
	PageID     string
	Widget     domain.PageWidget
	Ctx        pageCtx
	GridColumn string
	GridRow    string
	Refresh    func()
}

// customTile renders one widget instance as a bento cell: a header with the title
// and a delete button, and a body produced by the type's renderer. It's its own
// component so the delete-click hook stays stable across the widget list.
func customTile(props customTileProps) ui.Node {
	w := props.Widget
	pageID := props.PageID
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

	return Div(Class("w"),
		Attr("style", "grid-column:"+props.GridColumn+";grid-row:"+props.GridRow),
		Div(Class("wh"),
			Span(Class("grip"), ""),
			H3(title),
			Button(Class("gear-inline"), Type("button"), Title(uistate.T("pages.deleteWidget")),
				OnClick(del), "✕"),
		),
		Div(Class("wbody"), widgetBody(w, props.Ctx)),
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
		return P(Class("empty"), uistate.T("pages.unknownWidget"))
	}
}

// cpImageBody renders an image artifact bound by ID.
func cpImageBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	art, ok := findArtifact(ctx.App.Artifacts(), w.Binding.ArtifactID)
	if !ok || art.Kind != artifacts.KindImage || len(art.Bytes) == 0 {
		return P(Class("empty"), uistate.T("pages.pickArtifact"))
	}
	return Img(Attr("src", artifacts.DataURL(art.MIME, art.Bytes)),
		Attr("alt", art.Name), Class("max-w-full max-h-full object-contain m-auto"))
}

// cpTableBody renders a dataset artifact (columns + first rows) bound by ID.
func cpTableBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	art, ok := findArtifact(ctx.App.Artifacts(), w.Binding.ArtifactID)
	if !ok || len(art.Columns) == 0 {
		return P(Class("empty"), uistate.T("pages.pickArtifact"))
	}
	head := make([]ui.Node, 0, len(art.Columns))
	for _, c := range art.Columns {
		head = append(head, Th(Class("text-left pr-3 text-faint font-medium"), c))
	}
	bodyRows := make([]ui.Node, 0)
	for i, r := range art.Rows {
		if i >= 8 { // keep tiles compact; the artifact keeps the full data
			break
		}
		cells := make([]ui.Node, 0, len(r))
		for _, cell := range r {
			cells = append(cells, Td(Class("pr-3 py-0.5 truncate"), cell))
		}
		bodyRows = append(bodyRows, Tr(cells))
	}
	return Table(Class("w-full text-[12px] fig"),
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
		return P(Class("err"), Attr("role", "alert"), err.Error())
	}
	return Div(Class("flex flex-col justify-center h-full"),
		Div(Class("font-display fig text-[28px]"), widgetdata.KPIText(val, w.Config["format"], ctx.Base)),
	)
}

// listBody shows up to N rows from the widget's data source. The row data
// (source selection, newest-first ordering, formatting, cap) is computed by the
// pure internal/widgetdata package; this just renders the rows.
func listBody(w domain.PageWidget, ctx pageCtx) ui.Node {
	rows, ok := widgetdata.ListRows(w.Binding.Source, widgetdata.Data{
		Transactions: ctx.App.Transactions(), Accounts: ctx.App.Accounts(),
		Budgets: ctx.App.Budgets(), Goals: ctx.App.Goals(), Tasks: ctx.App.Tasks(), Rates: ctx.Rates,
	}, widgetdata.DefaultListRows)
	if !ok {
		return P(Class("empty"), uistate.T("pages.pickSource"))
	}
	if len(rows) == 0 {
		return P(Class("empty"), uistate.T("pages.noData"))
	}
	nodes := make([]ui.Node, 0, len(rows))
	for _, r := range rows {
		nodes = append(nodes, Div(Class("row"),
			Span(Class("row-desc truncate"), r.Label),
			Span(Class("amount fig"), r.Value),
		))
	}
	return Div(Class("rows"), nodes)
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
func textBody(w domain.PageWidget) ui.Node {
	t := w.Config["text"]
	if t == "" {
		return P(Class("empty"), uistate.T("pages.emptyText"))
	}
	return P(Class("muted"), t)
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
		return Div(Class("mb-3"),
			Button(Class("btn btn-primary"), Type("button"), OnClick(func() { open.Set(true) }),
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
		bindControl = Input(Class("field"), Attr("placeholder", uistate.T("pages.kpiFormula")),
			Value(bind.Get()), OnInput(onBind))
	case widgetspec.TypeList:
		srcOpts := make([]ui.Node, 0)
		for _, d := range widgetspec.ListSources() {
			srcOpts = append(srcOpts, Option(Value(d.Type), SelectedIf(bind.Get() == d.Type), d.Label))
		}
		bindControl = Select(Class("field"), OnChange(onBind), srcOpts)
	case widgetspec.TypeText:
		bindControl = Input(Class("field"), Attr("placeholder", uistate.T("pages.textContent")),
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
		bindControl = Select(Class("field"), OnChange(onBind), artOpts)
	default: // Chart needs no binding in Phase B
		bindControl = P(Class("muted"), uistate.T("pages.chartDefault"))
	}

	return Section(Class("card mb-3"),
		Div(Class("form-grid"),
			Select(Class("field"), OnChange(onType), typeOpts),
			Input(Class("field"), Attr("placeholder", uistate.T("pages.widgetTitle")),
				Value(title.Get()), OnInput(onTitle)),
			bindControl,
		),
		Div(Class("flex gap-2 mt-2"),
			Button(Class("btn btn-primary"), Type("button"), OnClick(addWidget), uistate.T("action.add")),
			Button(Class("btn"), Type("button"), OnClick(func() { open.Set(false) }), uistate.T("action.cancel")),
		),
	)
}

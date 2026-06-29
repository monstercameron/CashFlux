// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcatalog"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// studioPreviewCtx assembles a RenderCtx over the current month + whole household so
// the designer can render a widget spec through the EXACT dashboard engine path —
// what you design is what you'll get. Pure read of appstate; no persistence.
func studioPreviewCtx(app *appstate.App) widgetrender.RenderCtx {
	now := time.Now()
	start, end := dateutil.MonthRange(now)
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()
	vars := engineenv.Vars(engineenv.Data{
		Accounts: accounts, Transactions: txns, Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
		Rates: rates, Now: now, PeriodStart: start, PeriodEnd: end,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
	})
	net := money.New(0, base)
	if nw, err := ledger.NetWorthExplained(accounts, txns, rates); err == nil {
		net = nw.Net
	}
	return widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: accounts, ScopedTxns: txns,
		Rates: rates, Base: base, Start: start, End: end,
		PeriodLabel: now.Format("Jan 2006"), Vars: vars, Net: net,
	}
}

type studioDesignerPanelProps struct{}

// studioDesignerPanel is the spec-based widget DESIGNER: a casual user picks a kind
// and a metric (or writes a formula), composes a layout, and sees a LIVE preview
// rendered by the real engine, then publishes the widget to their dashboard. Every
// picker is populated from widgetcatalog (no hardcoded option lists), and the result
// is a domain.WidgetSpec persisted as a placement (pure hydration). Hooks: UseState
// per field + the layout atom — all leaf inputs/buttons are isolated components, so
// the form can show kind-specific fields without disturbing this hook chain.
func studioDesignerPanel(_ studioDesignerPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	kind := ui.UseState("kpi")
	title := ui.UseState("My widget")
	formula := ui.UseState("net_worth")
	format := ui.UseState("currency")
	sub := ui.UseState("")
	collection := ui.UseState("transactions")
	series := ui.UseState("networth")
	limit := ui.UseState("6")
	listDisplay := ui.UseState("cap") // cap | scroll | page
	listLink := ui.UseState(true)     // add a "view all" link to the source screen
	sortBy := ui.UseState("")         // list sort column ("" = source's natural order)
	sortDir := ui.UseState("desc")    // desc | asc (engine arg gets a "-" prefix for desc)
	chartIsSeries := ui.UseState(true)
	blocks := ui.UseState(widgetcatalog.IncomeVsSpendingBlocks())
	cols := ui.UseState("2")
	rows := ui.UseState("1")
	status := ui.UseState("")
	published := ui.UseState(false)
	advanced := ui.UseState(false)
	activeStarter := ui.UseState("")
	layoutAtom := uistate.UseLayoutItems()
	nav := router.UseNavigate()

	spec := studioBuildSpec(studioSpecInput{
		kind: kind.Get(), title: title.Get(), formula: formula.Get(), format: format.Get(), sub: sub.Get(),
		collection: collection.Get(), series: series.Get(), chartIsSeries: chartIsSeries.Get(),
		limit: atoiOr(limit.Get(), 6), listDisplay: listDisplay.Get(), listLink: listLink.Get(),
		sortBy: sortBy.Get(), sortDir: sortDir.Get(), blocks: blocks.Get(),
	})

	publishFn := func() {
		s := spec
		if err := s.Validate(); err != nil {
			published.Set(false)
			status.Set(studioFriendlyError(err))
			return
		}
		wid := userSpecPrefix + id.New()
		s.ID = wid
		item := dashlayout.Item{ID: wid, ColSpan: clampSpan(atoiOr(cols.Get(), 2), 4), RowSpan: clampSpan(atoiOr(rows.Get(), 1), 4)}
		pl := domain.Placement{SchemaVersion: domain.WidgetSpecVersion, ID: wid, Surface: "dashboard", Spec: s, Layout: item}
		if err := app.PutPlacements([]domain.Placement{pl}); err != nil {
			published.Set(false)
			status.Set("Couldn't save — " + err.Error())
			return
		}
		next := append([]dashlayout.Item(nil), layoutAtom.Get()...)
		next = append(next, item)
		layoutAtom.Set(next)
		uistate.PersistItems(next)
		published.Set(true)
		status.Set("“" + title.Get() + "” is on your dashboard.")
	}

	// applyStarter pre-fills the form from a preset so the user never starts blank.
	applyStarter := func(st widgetcatalog.Starter) {
		kind.Set(st.Kind)
		title.Set(st.Title)
		sub.Set(st.Sub)    // always reset the caption so a starter never inherits stale text
		sortBy.Set("")     // a preset uses the source's natural order until the user sorts
		sortDir.Set("desc")
		if st.Formula != "" {
			formula.Set(st.Formula)
		}
		if st.Format != "" {
			format.Set(st.Format)
		}
		if st.Collection != "" {
			collection.Set(st.Collection)
			chartIsSeries.Set(false)
		}
		if st.Series != "" {
			series.Set(st.Series)
			chartIsSeries.Set(true)
		}
		if len(st.Blocks) > 0 {
			blocks.Set(append([]domain.Block(nil), st.Blocks...))
		}
		activeStarter.Set(st.Label)
		status.Set("")
		published.Set(false)
	}

	// LIVE preview through the real engine.
	var preview ui.Node
	if err := spec.Validate(); err == nil {
		ctx := studioPreviewCtx(app)
		ctx.Spec = spec
		if node, ok := safeRenderSpec(spec, ctx); ok {
			preview = node
		} else {
			preview = P(css.Class("empty t-body", tw.TextDim), "Nothing to preview yet.")
		}
	} else {
		preview = P(css.Class("empty t-body", tw.TextDim), studioFriendlyError(err))
	}

	form := studioDesignerForm(studioDesignerFormState{
		kind: kind.Get(), title: title.Get(), formula: formula.Get(), format: format.Get(), sub: sub.Get(),
		collection: collection.Get(), series: series.Get(), chartIsSeries: chartIsSeries.Get(),
		limit: limit.Get(), cols: cols.Get(), rows: rows.Get(), blocks: blocks.Get(), defs: app.CustomFieldDefs(),
		molecules:   app.Molecules(),
		advanced:    advanced.Get(),
		listDisplay: listDisplay.Get(), listLink: listLink.Get(),
		sortBy: sortBy.Get(), sortDir: sortDir.Get(),
		setListDisplay: listDisplay.Set,
		setListLink:    func(v string) { listLink.Set(v == "yes") },
		// Picking a column also sets a sensible default direction for its type
		// (numbers High→Low, text A→Z); the user can still flip it.
		setSortBy: func(v string) {
			sortBy.Set(v)
			if v == "" {
				return
			}
			dir := "asc"
			for _, sfld := range widgetcatalog.SortFields(collection.Get()) {
				if sfld.Column == v && sfld.Numeric {
					dir = "desc"
				}
			}
			sortDir.Set(dir)
		},
		setSortDir: sortDir.Set,
		setKind:  func(v string) { kind.Set(v); activeStarter.Set("") }, setTitle: title.Set, setFormula: formula.Set, setFormat: format.Set, setSub: sub.Set,
		// Changing the data source clears the sort column — its columns differ, so a
		// stale choice could reference a column the new source doesn't have.
		setCollection: func(v string) { collection.Set(v); sortBy.Set("") }, setSeries: series.Set,
		setChartSrc:   func(v string) { chartIsSeries.Set(v == "series") },
		setLimit:      limit.Set, setCols: cols.Set, setRows: rows.Set,
		toggleAdvanced: func() {
			next := !advanced.Get()
			mols := app.Molecules()
			if next {
				// Revealing the formula: expand a molecule reference into its actual
				// atom-built expression (net_worth → "assets - liabilities") so the user
				// edits the real formula, not the molecule's name.
				if f := moleculeFormula(formula.Get(), mols); f != "" {
					formula.Set(f)
				}
			} else {
				// Going back to the picker: if the formula is exactly a molecule's
				// definition, collapse it back to that molecule so the picker matches.
				if name := moleculeForFormula(formula.Get(), mols); name != "" {
					formula.Set(name)
				}
			}
			advanced.Set(next)
		},
		addBlock: func() {
			blocks.Set(append(append([]domain.Block(nil), blocks.Get()...), domain.Block{Kind: domain.BlockText, Text: "New text"}))
		},
		changeBlock: func(i int, b domain.Block) {
			next := append([]domain.Block(nil), blocks.Get()...)
			if i >= 0 && i < len(next) {
				next[i] = b
				blocks.Set(next)
			}
		},
		removeBlock: func(i int) {
			cur := blocks.Get()
			next := make([]domain.Block, 0, len(cur))
			for j, b := range cur {
				if j != i {
					next = append(next, b)
				}
			}
			blocks.Set(next)
		},
		moveBlock: func(i, dir int) {
			cur := blocks.Get()
			j := i + dir
			if i < 0 || i >= len(cur) || j < 0 || j >= len(cur) {
				return
			}
			next := append([]domain.Block(nil), cur...)
			next[i], next[j] = next[j], next[i]
			blocks.Set(next)
		},
	})

	// Starter presets row.
	starterChips := make([]ui.Node, 0)
	for _, st := range widgetcatalog.Starters() {
		st := st
		cls := "studio-starter"
		if activeStarter.Get() == st.Label {
			cls = "studio-starter is-active"
		}
		starterChips = append(starterChips, ui.CreateElement(studioButton, studioButtonProps{
			Label: st.Label, Class: cls, OnClick: func() { applyStarter(st) },
		}))
	}

	// Status, with a direct link to the dashboard once published.
	statusNode := Fragment()
	if status.Get() != "" {
		kids := []ui.Node{Span(status.Get())}
		if published.Get() {
			kids = append(kids, ui.CreateElement(studioButton, studioButtonProps{
				Label: "Open dashboard →", Class: "btn btn-sm", OnClick: func() { nav.Navigate(uistate.RoutePath("/")) },
			}))
		}
		statusNode = Div(css.Class("studio-status"), kids)
	}

	// Width/height-aware preview: place the tile on a 4-col stage so its span reads true.
	pcols := clampSpan(atoiOr(cols.Get(), 2), 4)
	prows := clampSpan(atoiOr(rows.Get(), 1), 4)
	// Faithful mini-dashboard: a real 4-column bento (the dashboard's fixed column
	// count) where the tile occupies exactly its width×height SLOTS — so the preview
	// shows the true footprint, capped at 4 columns, never larger than the grid allows.
	stage := Div(css.Class("studio-stage"),
		Div(css.Class("studio-stage-inner"),
			Div(css.Class("studio-stage-cell"), Style(map[string]string{
				"grid-column": fmt.Sprintf("span %d", pcols),
				"grid-row":    fmt.Sprintf("span %d", prows),
			}), preview),
		),
	)

	return Div(css.Class("studio-design"),
		Div(css.Class("studio-design-head"),
			Span(css.Class("studio-eyebrow"), "Studio"),
			H2(css.Class("studio-design-title"), "Design a widget"),
			P(css.Class("studio-design-sub"), "Start from a preset or build your own — pick what to measure, shape how it looks, then drop it on your dashboard."),
			Div(css.Class("studio-starter-row"), starterChips),
		),
		Div(css.Class("studio-design-grid"),
			Div(css.Class("studio-config"), form),
			Div(css.Class("studio-stage-wrap"),
				Div(css.Class("studio-stage-head"),
					Span(css.Class("studio-eyebrow"), "Live preview"),
					Span(css.Class("studio-stage-hint"), "Updates as you design"),
				),
				stage,
				ui.CreateElement(studioButton, studioButtonProps{Label: "Publish to dashboard →", Class: "btn btn-primary studio-publish", OnClick: publishFn}),
				statusNode,
			),
		),
	)
}

// studioFriendlyError turns an engine validation error into casual, plain-English
// guidance instead of leaking Go internals.
func studioFriendlyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "scalar"):
		return "Pick a metric to show."
	case strings.Contains(msg, "pipeline"):
		return "Pick what this widget should show."
	case strings.Contains(msg, "block"):
		return "Add at least one block to your layout."
	}
	return "Almost there — finish setting up this widget."
}

// studioSpecInput is the designer's form state, passed to studioBuildSpec.
type studioSpecInput struct {
	kind, title, formula, format, sub string
	collection, series                string
	chartIsSeries                     bool
	limit                             int
	listDisplay                       string // cap | scroll | page
	listLink                          bool
	sortBy                            string // list sort column ("" = natural order)
	sortDir                           string // desc | asc
	blocks                            []domain.Block
}

// listSourceCap bounds how many rows a scroll/page list pulls, so the Frame never
// balloons even when the user wants "all" rows.
const listSourceCap = 100

// studioBuildSpec composes a domain.WidgetSpec from the designer's form state. Pure;
// reused by the live preview and by publish so they never drift.
func studioBuildSpec(in studioSpecInput) domain.WidgetSpec {
	spec := domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, Title: in.title}
	switch in.kind {
	case "kpi":
		spec.Kind = domain.KindKPI
		spec.Scalar = &domain.ScalarBind{Expr: strings.TrimSpace(in.formula), Format: in.format, Sub: strings.TrimSpace(in.sub)}
	case "list":
		spec.Kind = domain.KindList
		p := domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: in.collection}}
		// Sort first (over the full set) so a following limit/cap keeps the right rows.
		// Engine arg: a leading "-" sorts descending; bare name ascending.
		if in.sortBy != "" {
			arg := in.sortBy
			if in.sortDir != "asc" {
				arg = "-" + in.sortBy
			}
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformSort, Arg: arg})
		}
		// "cap" trims to N at the engine; "scroll"/"page" pull up to a safety cap and
		// the renderer scrolls/pages within it.
		switch in.listDisplay {
		case "scroll", "page":
			p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: listSourceCap})
		default: // cap
			if in.limit > 0 {
				p.Transform = append(p.Transform, domain.Transform{Kind: domain.TransformLimit, N: in.limit})
			}
		}
		spec.Pipeline = &p
		// Carry the list display options on the spec so the renderer (and a reload)
		// honor them — pure hydration data, no per-widget code.
		display := in.listDisplay
		if display == "" {
			display = "cap"
		}
		spec.Settings = widgetcfg.Config{
			"display": display,
			"count":   fmt.Sprintf("%d", in.limit),
			"viewall": boolStr(in.listLink),
		}
	case "chart":
		spec.Kind = domain.KindChart
		if in.chartIsSeries {
			spec.Pipeline = &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: in.series}}}
		} else {
			spec.Pipeline = &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: in.collection}}
		}
	case "compound":
		spec.Kind = domain.KindText
		spec.Content = domain.ContentLayout{Mode: domain.LayoutCustom, Blocks: append([]domain.Block(nil), in.blocks...)}
	}
	return spec
}

func atoiOr(s string, def int) int {
	n, read := 0, false
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return def
		}
		n, read = n*10+int(r-'0'), true
	}
	if !read {
		return def
	}
	return n
}

// ─── Leaf input components (own their On* hooks so conditional rendering is safe) ──

type studioTextFieldProps struct {
	Label, Value, Placeholder, Hint string
	Big, Compact                    bool
	OnChange                        func(string)
}

func studioTextField(p studioTextFieldProps) ui.Node {
	h := ui.UseEvent(func(v string) {
		if p.OnChange != nil {
			p.OnChange(v)
		}
	})
	inputCls := "field studio-field"
	if p.Big {
		inputCls = "field studio-field studio-field-lg"
	}
	input := Input(css.Class(inputCls), Type("text"), Value(p.Value), Placeholder(p.Placeholder), Attr("aria-label", p.Label), OnInput(h))
	if p.Compact {
		// Label-less (aria only) for table-style rows where a column header names the field.
		return Label(css.Class("field-label field-compact"), input)
	}
	kids := []ui.Node{Span(css.Class("studio-label"), p.Label), input}
	if p.Hint != "" {
		kids = append(kids, Span(css.Class("studio-hint"), p.Hint))
	}
	return Label(css.Class("field-label"), kids)
}

type studioSelectFieldProps struct {
	Label, Value string
	Options      []widgetcatalog.Option
	Compact      bool
	OnChange     func(string)
}

func studioSelectField(p studioSelectFieldProps) ui.Node {
	so := make([]uiw.SelectOption, len(p.Options))
	for i, o := range p.Options {
		so[i] = uiw.SelectOption{Value: o.Value, Label: o.Label}
	}
	sel := uiw.SelectInput(uiw.SelectInputProps{Options: so, Selected: p.Value, OnChange: p.OnChange, AriaLabel: p.Label})
	if p.Compact {
		return Label(css.Class("field-label field-compact"), sel)
	}
	return Label(css.Class("field-label"), Span(css.Class("studio-label"), p.Label), sel)
}

type studioSegFieldProps struct {
	Label, Value string
	Options      []uiw.SegOption
	OnSelect     func(string)
}

// studioSegField is a labelled segmented control — used for short, mutually-exclusive
// choices (format, chart kind, size) so the option set is visible at a glance rather
// than hidden in a dropdown.
func studioSegField(p studioSegFieldProps) ui.Node {
	return Label(css.Class("field-label"),
		Span(css.Class("studio-label"), p.Label),
		uiw.Segmented(uiw.SegmentedProps{Label: p.Label, Selected: p.Value, OnSelect: p.OnSelect, Options: p.Options}),
	)
}

type studioTypeCardProps struct {
	Value, Label, Desc string
	Icon               icon.Name
	Selected           bool
	OnSelect           func(string)
}

// studioTypeCard is one selectable widget-type tile (icon + label + blurb). Its own
// component so its OnClick hook is isolated; the selected card carries an accent ring.
func studioTypeCard(p studioTypeCardProps) ui.Node {
	h := ui.UseEvent(func() {
		if p.OnSelect != nil {
			p.OnSelect(p.Value)
		}
	})
	cls := "studio-type-card"
	if p.Selected {
		cls = "studio-type-card is-selected"
	}
	return Button(css.Class(cls), Type("button"), Attr("aria-pressed", boolStr(p.Selected)), OnClick(h),
		Span(css.Class("studio-type-icon"), uiw.Icon(p.Icon, css.Class(tw.W5, tw.H5))),
		Span(css.Class("studio-type-label"), p.Label),
		Span(css.Class("studio-type-desc"), p.Desc),
	)
}

type studioButtonProps struct {
	Label, Class string
	OnClick      func()
}

func studioButton(p studioButtonProps) ui.Node {
	h := ui.UseEvent(func() {
		if p.OnClick != nil {
			p.OnClick()
		}
	})
	cls := p.Class
	if cls == "" {
		cls = "btn"
	}
	return Button(css.Class(cls), Type("button"), OnClick(h), p.Label)
}

// ─── Form ─────────────────────────────────────────────────────────────────────

type studioDesignerFormState struct {
	kind, title, formula, format, sub string
	collection, series                string
	chartIsSeries                     bool
	limit, cols, rows                 string
	blocks                            []domain.Block
	defs                              []customfields.Def
	molecules                         []domain.Molecule
	advanced                          bool
	listDisplay                       string
	listLink                          bool
	sortBy, sortDir                   string

	setKind, setTitle, setFormula, setFormat, setSub func(string)
	setCollection, setSeries, setChartSrc            func(string)
	setLimit, setCols, setRows                       func(string)
	setListDisplay, setListLink                      func(string)
	setSortBy, setSortDir                            func(string)
	toggleAdvanced                                   func()
	addBlock                                         func()
	changeBlock                                      func(int, domain.Block)
	removeBlock                                      func(int)
	moveBlock                                        func(int, int)
}

// studioDesignerForm renders the left-hand config as ordered sections (Name → Type →
// Data → Size). Plain function (no hooks); every field/card/button is an isolated
// component, so kind-specific fields can appear/disappear without disturbing any hook
// chain. The numbered eyebrows encode the real authoring sequence.
func studioDesignerForm(s studioDesignerFormState) ui.Node {
	tf := func(label, value, placeholder, hint string, on func(string)) ui.Node {
		return ui.CreateElement(studioTextField, studioTextFieldProps{Label: label, Value: value, Placeholder: placeholder, Hint: hint, OnChange: on})
	}
	sf := func(label, value string, opts []widgetcatalog.Option, on func(string)) ui.Node {
		return ui.CreateElement(studioSelectField, studioSelectFieldProps{Label: label, Value: value, Options: opts, OnChange: on})
	}
	seg := func(label, value string, opts []uiw.SegOption, on func(string)) ui.Node {
		return ui.CreateElement(studioSegField, studioSegFieldProps{Label: label, Value: value, Options: opts, OnSelect: on})
	}

	// Section 1 — Type, as selectable cards (the primary choice).
	typeCards := make([]ui.Node, 0, len(widgetcatalog.Kinds()))
	for _, k := range widgetcatalog.Kinds() {
		typeCards = append(typeCards, ui.CreateElement(studioTypeCard, studioTypeCardProps{
			Value: k.Value, Label: k.Label, Desc: kindBlurb(k.Value), Icon: kindIcon(k.Value),
			Selected: s.kind == k.Value, OnSelect: s.setKind,
		}))
	}

	// Section 2 — Data (kind-specific).
	var dataTitle string
	var dataFields []ui.Node
	switch s.kind {
	case "kpi":
		dataTitle = "The figure"
		dataFields = []ui.Node{}
		// Simple mode = pick a metric (primary, casual path); advanced mode = edit the
		// raw formula. Showing only one at a time avoids the picker/formula mismatch.
		advLabel := "▸ Edit the formula directly"
		if s.advanced {
			advLabel = "▾ Use the metric picker instead"
			dataFields = append(dataFields,
				tf("Formula", s.formula, "assets - liabilities", "Combine metrics with + − × ÷. Functions: sum, avg, min, max, round, clamp, safediv.", s.setFormula))
		} else {
			dataFields = append(dataFields,
				ui.CreateElement(studioMetricPicker, studioMetricPickerProps{Defs: s.defs, Molecules: s.molecules, Selected: s.formula, OnPick: s.setFormula}))
		}
		dataFields = append(dataFields,
			seg("Show as", s.format, segFrom(widgetcatalog.Formats()), s.setFormat),
			tf("Caption (optional)", s.sub, "e.g. across all accounts", "Plain text. Insert a live value with a metric, e.g. {{accounts}}.", s.setSub),
			ui.CreateElement(studioButton, studioButtonProps{Label: advLabel, Class: "btn btn-sm studio-toggle", OnClick: s.toggleAdvanced}))
	case "list":
		dataTitle = "The list"
		dataFields = []ui.Node{
			sf("Show", s.collection, widgetcatalog.Collections(), s.setCollection),
			seg("Display", s.listDisplay, segFrom(widgetcatalog.ListDisplays()), s.setListDisplay),
		}
		// Row count applies to "cap" (top N) and "page" (rows per page); scrolling
		// shows everything, so the count is irrelevant there.
		if s.listDisplay != "scroll" {
			rowLabel := "Rows to show"
			if s.listDisplay == "page" {
				rowLabel = "Rows per page"
			}
			dataFields = append(dataFields, seg(rowLabel, s.limit, segFrom(widgetcatalog.RowCounts()), s.setLimit))
		}
		// Scroll/page pull up to a safety cap; say so plainly rather than silently
		// truncating a large dataset.
		if s.listDisplay != "cap" {
			dataFields = append(dataFields, Span(css.Class("studio-hint"), fmt.Sprintf("Up to %d rows are loaded.", listSourceCap)))
		}
		// Offer a "view all" link only when the source has a full-data screen.
		if route, lbl := widgetcatalog.CollectionRoute(s.collection); route != "" {
			dataFields = append(dataFields, seg("Add a link: “"+lbl+"”", boolToYesNo(s.listLink),
				[]uiw.SegOption{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}}, s.setListLink))
		}
		// Sort control — order rows by one of the source's columns. "Default order"
		// keeps the source's natural order (e.g. newest transactions first). The
		// direction labels adapt to the column type: High↔Low for numbers, A↔Z for text.
		if sortFields := widgetcatalog.SortFields(s.collection); len(sortFields) > 0 {
			sortOpts := []widgetcatalog.Option{{Value: "", Label: "Default order"}}
			selNumeric := true
			for _, sfld := range sortFields {
				sortOpts = append(sortOpts, widgetcatalog.Option{Value: sfld.Column, Label: sfld.Label})
				if sfld.Column == s.sortBy {
					selNumeric = sfld.Numeric
				}
			}
			dataFields = append(dataFields, sf("Sort by", s.sortBy, sortOpts, s.setSortBy))
			if s.sortBy != "" {
				dataFields = append(dataFields, seg("Order", s.sortDir, segFrom(widgetcatalog.SortDirections(selNumeric)), s.setSortDir))
			}
		}
	case "chart":
		dataTitle = "The chart"
		src := "series"
		if !s.chartIsSeries {
			src = "collection"
		}
		dataFields = []ui.Node{seg("Chart kind", src, segFrom(widgetcatalog.ChartSourceTypes()), s.setChartSrc)}
		if s.chartIsSeries {
			dataFields = append(dataFields, sf("Track", s.series, widgetcatalog.SeriesMetrics(), s.setSeries))
		} else {
			dataFields = append(dataFields, sf("Break down", s.collection, widgetcatalog.Collections(), s.setCollection))
		}
	case "compound":
		dataTitle = "The layout"
		dataFields = []ui.Node{studioBlocksEditor(s)}
	}

	sizeSeg := []ui.Node{
		Div(css.Class("studio-size-row"),
			seg("Width", s.cols, spanSegOptions(), s.setCols),
			seg("Height", s.rows, spanSegOptions(), s.setRows),
		),
	}

	return Div(css.Class("studio-form"),
		ui.CreateElement(studioTextField, studioTextFieldProps{Label: "Name", Value: s.title, Placeholder: "Name your widget", OnChange: s.setTitle, Big: true}),
		studioSection("Type", []ui.Node{Div(css.Class("studio-type-grid"), typeCards)}),
		studioSection(dataTitle, dataFields),
		studioSection("Size on dashboard", sizeSeg),
	)
}

// studioSection wraps a titled group of fields with an eyebrow heading.
func studioSection(title string, body []ui.Node) ui.Node {
	return Div(css.Class("studio-section"),
		Span(css.Class("studio-section-title"), title),
		Div(css.Class("studio-section-body"), body),
	)
}

// kindBlurb / kindIcon give each widget type a one-line description and a glyph for
// the type-card selector.
func kindBlurb(kind string) string {
	switch kind {
	case "kpi":
		return "One headline number"
	case "compound":
		return "Mix figures, text & icons"
	case "list":
		return "Rows from your data"
	case "chart":
		return "A trend or breakdown"
	}
	return ""
}

func kindIcon(kind string) icon.Name {
	switch kind {
	case "kpi":
		return icon.Reports
	case "compound":
		return icon.Box
	case "list":
		return icon.List
	case "chart":
		return icon.TrendingUp
	}
	return icon.Box
}

// segFrom adapts catalog options to segmented options.
func segFrom(opts []widgetcatalog.Option) []uiw.SegOption {
	out := make([]uiw.SegOption, len(opts))
	for i, o := range opts {
		out[i] = uiw.SegOption{Value: o.Value, Label: o.Label}
	}
	return out
}

// boolToYesNo maps a bool to the segmented "yes"/"no" value.
func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// spanSegOptions are the 1–4 grid-span choices for width/height.
func spanSegOptions() []uiw.SegOption {
	return []uiw.SegOption{{Value: "1", Label: "1"}, {Value: "2", Label: "2"}, {Value: "3", Label: "3"}, {Value: "4", Label: "4"}}
}

type studioMetricPickerProps struct {
	Defs      []customfields.Def
	Molecules []domain.Molecule
	Selected  string
	OnPick    func(string)
}

// studioMetricPicker is a grouped dropdown of every available metric (engine vars +
// custom fields), so a casual user picks "the figure they care about" by name; the
// chosen metric becomes the formula and its plain-English description shows beneath.
// When the metric is a molecule (a compound figure), the picker also shows how it is
// built from atoms — e.g. "Built from: assets − liabilities" — so every figure is
// auditable. Its own component for an isolated hook.
func studioMetricPicker(p studioMetricPickerProps) ui.Node {
	ms := widgetcatalog.Metrics(p.Defs, p.Molecules)
	opts := make([]uiw.SelectOption, 0, len(ms))
	var sel widgetcatalog.Metric
	for _, m := range ms {
		opts = append(opts, uiw.SelectOption{Value: m.Name, Label: string(m.Group) + " · " + m.Label})
		if m.Name == p.Selected {
			sel = m
		}
	}
	kids := []ui.Node{
		Span(css.Class("studio-label"), "Metric"),
		uiw.SelectInput(uiw.SelectInputProps{Options: opts, Selected: p.Selected, OnChange: p.OnPick, AriaLabel: "Metric"}),
	}
	if sel.Doc != "" {
		kids = append(kids, Span(css.Class("studio-hint"), sel.Doc))
	}
	if sel.Molecule && sel.Formula != "" {
		kids = append(kids, Span(css.Class("studio-formula"), "Built from atoms:  "+prettyFormula(sel.Formula)))
	}
	return Label(css.Class("field-label"), kids)
}

// prettyFormula makes a stored formula readable in the UI (ASCII operators → math
// glyphs) without changing its meaning.
func prettyFormula(f string) string {
	r := strings.NewReplacer(" - ", " − ", " * ", " × ", " / ", " ÷ ")
	return r.Replace(f)
}

// moleculeFormula returns the atom-built formula for a molecule referenced by name
// (e.g. "net_worth" → "assets - liabilities"), or "" if name is not a molecule.
func moleculeFormula(name string, molecules []domain.Molecule) string {
	name = strings.TrimSpace(name)
	for _, m := range molecules {
		if m.Name == name {
			return m.Formula
		}
	}
	return ""
}

// moleculeForFormula is the inverse: the molecule name whose definition equals the
// given formula, or "" if none — used to collapse an expanded formula back to its
// molecule when the user returns to the picker.
func moleculeForFormula(formula string, molecules []domain.Molecule) string {
	formula = strings.TrimSpace(formula)
	for _, m := range molecules {
		if strings.TrimSpace(m.Formula) == formula {
			return m.Name
		}
	}
	return ""
}

// studioBlocksEditor renders the compound widget's block list with add/remove and a
// per-row editor (each row is its own component so its hooks stay stable).
func studioBlocksEditor(s studioDesignerFormState) ui.Node {
	rowNodes := make([]ui.Node, 0, len(s.blocks))
	for i, b := range s.blocks {
		i, b := i, b
		rowNodes = append(rowNodes, ui.CreateElement(studioBlockRow, studioBlockRowProps{
			Index: i, Count: len(s.blocks), Block: b, Defs: s.defs,
			OnChange: func(nb domain.Block) { s.changeBlock(i, nb) },
			OnRemove: func() { s.removeBlock(i) },
			OnMoveUp: func() { s.moveBlock(i, -1) },
			OnMoveDn: func() { s.moveBlock(i, 1) },
		}))
	}
	header := Div(css.Class("studio-block-head"),
		Span(""), Span("Block"), Span("Shows"), Span("Width"), Span(""),
	)
	return Div(css.Class("studio-blocks"),
		Span(css.Class("studio-hint"), "Stack blocks top to bottom. Set a width to place figures side by side."),
		header,
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2), rowNodes),
		ui.CreateElement(studioButton, studioButtonProps{Label: "+ Add block", Class: "btn btn-sm studio-addblock", OnClick: s.addBlock}),
	)
}

type studioBlockRowProps struct {
	Index, Count int
	Block        domain.Block
	Defs         []customfields.Def
	OnChange     func(domain.Block)
	OnRemove     func()
	OnMoveUp     func()
	OnMoveDn     func()
}

// blockWidthOptions are the per-block width choices (Full or a 1–4 column span).
func blockWidthOptions() []widgetcatalog.Option {
	return []widgetcatalog.Option{{Value: "0", Label: "Full"}, {Value: "1", Label: "1"}, {Value: "2", Label: "2"}, {Value: "3", Label: "3"}, {Value: "4", Label: "4"}}
}

// metricSelectOptions are the metrics as compact value/label options (no description),
// for inline pickers like a figure block.
func metricSelectOptions(defs []customfields.Def) []widgetcatalog.Option {
	ms := widgetcatalog.Metrics(defs, nil)
	out := make([]widgetcatalog.Option, len(ms))
	for i, m := range ms {
		out[i] = widgetcatalog.Option{Value: m.Name, Label: m.Label}
	}
	return out
}

// studioBlockRow edits one content block. It composes leaf input components (each
// owns its hook), so it holds no element hooks itself and can render kind-specific
// fields freely. A Figure block is two guided pickers (metric + format), never a
// raw "metric|verb" DSL.
func studioBlockRow(props studioBlockRowProps) ui.Node {
	b := props.Block
	setKind := func(v string) {
		nb := b
		nb.Kind = domain.BlockKind(v)
		props.OnChange(nb)
	}
	setSpan := func(v string) {
		nb := b
		nb.ColSpan = atoiOr(v, 0)
		props.OnChange(nb)
	}
	span := "0"
	if b.ColSpan > 0 {
		span = fmt.Sprintf("%d", b.ColSpan)
	}

	// "Shows" column — the kind-specific control(s).
	var shows []ui.Node
	switch b.Kind {
	case domain.BlockFigure:
		metric, verb := b.Bind, "currency"
		if k := strings.LastIndex(b.Bind, "|"); k >= 0 {
			metric, verb = b.Bind[:k], b.Bind[k+1:]
		}
		setMetric := func(v string) { nb := b; nb.Bind = v + "|" + verb; props.OnChange(nb) }
		setVerb := func(v string) { nb := b; nb.Bind = metric + "|" + v; props.OnChange(nb) }
		// Two side-by-side controls get tiny sub-labels so the user never has to infer
		// which dropdown is the metric vs the format.
		shows = []ui.Node{
			Div(css.Class("studio-microfield"),
				Span(css.Class("studio-microlabel"), "Metric"),
				ui.CreateElement(studioSelectField, studioSelectFieldProps{Label: "Metric", Compact: true, Value: metric, Options: metricSelectOptions(props.Defs), OnChange: setMetric}),
			),
			Div(css.Class("studio-microfield"),
				Span(css.Class("studio-microlabel"), "Format"),
				ui.CreateElement(studioSelectField, studioSelectFieldProps{Label: "Format", Compact: true, Value: verb, Options: widgetcatalog.FigureFormats(), OnChange: setVerb}),
			),
		}
	case domain.BlockText:
		setText := func(v string) { nb := b; nb.Text = v; props.OnChange(nb) }
		shows = []ui.Node{ui.CreateElement(studioTextField, studioTextFieldProps{Label: "Caption text", Compact: true, Value: b.Text, Placeholder: "Caption", OnChange: setText})}
	case domain.BlockIcon:
		setIcon := func(v string) { nb := b; nb.Bind = v; props.OnChange(nb) }
		shows = []ui.Node{ui.CreateElement(studioTextField, studioTextFieldProps{Label: "Icon name", Compact: true, Value: b.Bind, Placeholder: "sparkles", OnChange: setIcon})}
	default:
		shows = []ui.Node{Span(css.Class("studio-hint"), "—")}
	}

	moveCtrls := []ui.Node{}
	if props.Index > 0 {
		moveCtrls = append(moveCtrls, ui.CreateElement(studioButton, studioButtonProps{Label: "↑", Class: "btn-icon studio-block-move", OnClick: props.OnMoveUp}))
	}
	if props.Index < props.Count-1 {
		moveCtrls = append(moveCtrls, ui.CreateElement(studioButton, studioButtonProps{Label: "↓", Class: "btn-icon studio-block-move", OnClick: props.OnMoveDn}))
	}

	return Div(css.Class("studio-block-row"),
		Span(css.Class("studio-block-num"), fmt.Sprintf("%d", props.Index+1)),
		ui.CreateElement(studioSelectField, studioSelectFieldProps{Label: "Block type", Compact: true, Value: string(b.Kind), Options: widgetcatalog.BlockKinds(), OnChange: setKind}),
		Div(css.Class("studio-block-shows"), shows),
		ui.CreateElement(studioSelectField, studioSelectFieldProps{Label: "Width", Compact: true, Value: span, Options: blockWidthOptions(), OnChange: setSpan}),
		Div(css.Class("studio-block-actions"),
			Div(css.Class("studio-block-move-group"), moveCtrls),
			ui.CreateElement(studioButton, studioButtonProps{Label: "✕", Class: "btn-del studio-block-del", OnClick: props.OnRemove}),
		),
	)
}

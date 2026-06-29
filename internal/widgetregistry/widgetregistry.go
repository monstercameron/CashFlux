// SPDX-License-Identifier: MIT

// Package widgetregistry is the single, platform-independent source of truth for
// the app's built-in widgets: each widget's id, display-name key, icon id,
// class, and default grid size. It replaces the scattered per-concern maps
// (dashboard renderer keys, manager title keys, route/icon maps) with one
// descriptor per widget, and produces the seed domain.WidgetSpec that drives a
// placement. Pure Go, no syscall/js — unit-tested on native Go. The wasm render
// layer (internal/widgetrender) resolves a descriptor's id to a Go renderer.
//
// See docs/UNIFIED_WIDGET_API.md §6 (the registry split that protects testability).
package widgetregistry

import (
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Class distinguishes a widget whose body is fully defined by its spec
// (DataDriven) from one whose body is Go code keyed by id (Native). Most built-in
// dashboard tiles are Native — they compute via domain packages (ledger,
// budgeting, …) that a data pipeline can't express. See §4.
type Class string

const (
	// ClassNative is a Go-rendered widget body, keyed by id.
	ClassNative Class = "native"
	// ClassDataDriven is a widget fully defined by its spec (KPI/List/Table/Chart).
	ClassDataDriven Class = "data"
)

// Descriptor is the registered metadata for one widget id.
type Descriptor struct {
	ID         string // stable widget id (== NativeID for Native widgets)
	NameKey    string // i18n key for the display name
	IconID     string // icon key, resolved to a glyph in the wasm layer
	Class      Class
	DefaultCol int // default ColSpan on the 4-column bento
	DefaultRow int // default RowSpan
}

var (
	registry = map[string]Descriptor{}
	order    []string
)

// Register adds or replaces a descriptor, preserving first-seen order.
func Register(d Descriptor) {
	if _, ok := registry[d.ID]; !ok {
		order = append(order, d.ID)
	}
	registry[d.ID] = d
}

// Get returns the descriptor for an id.
func Get(id string) (Descriptor, bool) {
	d, ok := registry[id]
	return d, ok
}

// Catalog returns all descriptors in registration order — drives the "add widget"
// picker and the widget manager.
func Catalog() []Descriptor {
	out := make([]Descriptor, 0, len(order))
	for _, id := range order {
		out = append(out, registry[id])
	}
	return out
}

// DefaultSpec returns the seed WidgetSpec for a registered id. Native widgets get
// a Native spec keyed by id; the display title is resolved in the UI from NameKey.
func DefaultSpec(id string) (domain.WidgetSpec, bool) {
	d, ok := registry[id]
	if !ok {
		return domain.WidgetSpec{}, false
	}
	spec := domain.WidgetSpec{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            id,
		Kind:          domain.KindNative,
		NativeID:      id,
	}
	if d.Class == ClassDataDriven {
		// A DataDriven KPI is fully described by its spec: a scalar binding (formula +
		// format + templated sub-label) the engine hydrates, with no Go body keyed by
		// id. The wasm layer dispatches on Kind==KPI rather than NativeID.
		if sb, ok := kpiBindings[id]; ok {
			b := sb // copy so the returned spec doesn't alias the registry table
			spec.Kind = domain.KindKPI
			spec.NativeID = ""
			spec.Scalar = &b
		}
		// A DataDriven collection/chart is described by a Pipeline: a Source the engine
		// resolves into a Frame (via widgetsource), optionally refined by the tile's
		// settings at render time. The wasm layer dispatches on Kind in {List,Chart}.
		if pb, ok := pipelineBindings[id]; ok {
			p := pb.pipe // copy so the returned spec doesn't alias the registry table
			spec.Kind = pb.kind
			spec.NativeID = ""
			spec.Pipeline = &p
		}
		// A DataDriven compound is described by a custom ContentLayout (blocks) + a
		// token-first Style; the content-layout engine renders it with no Go body.
		if cb, ok := contentBindings[id]; ok {
			spec.Kind = cb.kind
			spec.NativeID = ""
			spec.Style = cb.style
			// Deep-copy the blocks so the returned spec never aliases the shared table.
			blocks := make([]domain.Block, len(cb.content.Blocks))
			copy(blocks, cb.content.Blocks)
			spec.Content = domain.ContentLayout{Mode: cb.content.Mode, Blocks: blocks}
		}
	}
	return spec, true
}

// DefaultPlacement builds a validated placement for a widget id on a surface,
// using the registry's default size. Returns false if the id is unknown or the
// resulting spec is invalid.
func DefaultPlacement(id, surface string) (domain.Placement, bool) {
	spec, ok := DefaultSpec(id)
	if !ok {
		return domain.Placement{}, false
	}
	d, _ := registry[id]
	pl := domain.Placement{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            id,
		Surface:       surface,
		Spec:          spec,
		Layout:        dashlayout.Item{ID: id, ColSpan: d.DefaultCol, RowSpan: d.DefaultRow},
	}
	if pl.Validate() != nil {
		return domain.Placement{}, false
	}
	return pl, true
}

// nameKeys maps each built-in id to its display-name i18n key.
var nameKeys = map[string]string{
	"attention":       "dashboard.attention",
	"kpi-networth":    "dashboard.netWorth",
	"kpi-income":      "dashboard.income",
	"kpi-spending":    "dashboard.spending",
	"kpi-liabilities": "dashboard.liabilities",
	"kpi-assets":      "dashboard.assets",
	"kpi-safetospend": "dashboard.safeToSpend",
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
	"smart-digest":    "dashboard.smartDigest",
	"anomaly-hub":     "dashboard.anomalyHub",
	"spotlight":       "dashboard.spotlight",
}

// iconIDs maps each built-in id to its header glyph id (resolved in the UI).
var iconIDs = map[string]string{
	"kpi-networth":    "accounts",
	"kpi-liabilities": "creditCard",
	"kpi-income":      "arrowDownCircle",
	"kpi-spending":    "arrowUpCircle",
	"accounts":        "landmark",
	"trend":           "trendingUp",
	"cashflow":        "trendingUp",
	"bills":           "bills",
	"freshness":       "clock",
	"recent":          "receipt",
	"savings":         "reports",
	"health":          "insights",
	"breakdown":       "budgets",
	"budgets":         "budgets",
	"goals":           "goals",
	"todo":            "todo",
	"highlight":       "insights",
	"spotlight":       "sparkles",
}

// kpiBindings holds the declarative scalar binding for each DataDriven KPI: the
// default formula (over the engine variable surface), display format, and a
// templated sub-label. Seeding these here — rather than a Go render closure — is
// what makes the tile fully composable from its spec. The same defaults are
// mirrored as editable widgetcfg settings so a user can still rewrite the formula.
var kpiBindings = map[string]domain.ScalarBind{
	"kpi-networth":    {Expr: "net_worth", Format: "currency"}, // sub is contextual (delta + FX disclosure), supplied at render
	"kpi-assets":      {Expr: "assets", Format: "currency", Sub: "{{asset_accounts|plural:account}}"},
	"kpi-liabilities": {Expr: "liabilities", Format: "currency", Sub: "{{liability_accounts|plural:account}}"},
	"kpi-income":      {Expr: "income", Format: "currency", Sub: "{{period}} · {{income_count|plural:deposit}} · cash flow {{cashflow_net|signed}}"},
	"kpi-spending":    {Expr: "expense", Format: "currency", Sub: "{{period}} · {{expense_count|plural:expense}}"},
	"kpi-safetospend": {Expr: "safe_to_spend", Format: "currency"}, // sub is sign-driven copy, supplied at render
	"savings":         {Expr: "savings_rate", Format: "percent"},   // rendered as a gauge by renderKPISpec
}

// pipelineBindings holds the declarative Pipeline for each DataDriven collection /
// chart widget: a Source the engine resolves into a Frame. The tile's user settings
// (limit, at-risk, cleared, window) refine it at render time. Seeding these here —
// rather than a Go render closure that computes inline — is what makes the data path
// declarative; the bespoke visualization is a registered FrameRenderer.
var pipelineBindings = map[string]struct {
	kind domain.WidgetKind
	pipe domain.Pipeline
}{
	"budgets":   {domain.KindList, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "budgets"}}},
	"accounts":  {domain.KindList, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "accounts"}}},
	"recent":    {domain.KindList, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}}},
	"bills":     {domain.KindList, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "bills"}}},
	"breakdown": {domain.KindChart, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "spending-breakdown"}}},
	"trend":     {domain.KindChart, domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "networth"}}}},
	"cashflow":  {domain.KindChart, domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "cashflow"}}}},
}

// contentBindings holds the declarative intra-tile CONTENT LAYOUT for compound
// widgets (§7.5): a custom arrangement of text/figure/icon/divider blocks the
// content-layout engine renders, with a token-first tile Style. This is what makes a
// fully custom compound widget composable from a spec — no Go body. The "spotlight"
// built-in demonstrates the engine: a glyph, a heading, two side-by-side money
// figures (income/expense, toned), a divider, and a templated caption.
var contentBindings = map[string]struct {
	kind    domain.WidgetKind
	style   domain.Style
	content domain.ContentLayout
}{
	"spotlight": {
		kind: domain.KindText, // carries no single data binding — it composes blocks
		content: domain.ContentLayout{
			Mode: domain.LayoutCustom,
			Blocks: []domain.Block{
				{Kind: domain.BlockIcon, Bind: "sparkles"},
				{Kind: domain.BlockText, Text: "This month", Style: domain.Style{FontWeight: "600"}},
				{Kind: domain.BlockDivider},
				{Kind: domain.BlockFigure, Bind: "income|currency", ColSpan: 2, Style: domain.Style{Text: "var(--up)"}},
				{Kind: domain.BlockFigure, Bind: "expense|currency", ColSpan: 2, Style: domain.Style{Text: "var(--down)"}},
				{Kind: domain.BlockText, Text: "Net {{cashflow_net|signed}} · {{floor(savings_rate)|number}}% saved", Style: domain.Style{Text: "var(--text-dim)"}},
			},
		},
	},
}

// init seeds the registry from dashlayout.DefaultItems() — the existing source of
// the built-in widget set and default sizes — so sizes stay DRY with the packer.
func init() {
	for _, it := range dashlayout.DefaultItems() {
		class := ClassNative
		if _, ok := kpiBindings[it.ID]; ok {
			class = ClassDataDriven
		}
		if _, ok := pipelineBindings[it.ID]; ok {
			class = ClassDataDriven
		}
		if _, ok := contentBindings[it.ID]; ok {
			class = ClassDataDriven
		}
		Register(Descriptor{
			ID:         it.ID,
			NameKey:    nameKeys[it.ID],
			IconID:     iconIDs[it.ID],
			Class:      class,
			DefaultCol: it.ColSpan,
			DefaultRow: it.RowSpan,
		})
	}
	// The dashboard welcome/hero is a full-width Native widget rendered above the
	// bento (not a packed grid cell), so it isn't in DefaultItems; register it
	// explicitly so it has a spec and participates in the engine like any other tile.
	Register(Descriptor{ID: "hero", NameKey: "dashboard.heroTitle", Class: ClassNative, DefaultCol: 4, DefaultRow: 2})
}

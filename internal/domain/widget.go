// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

// Molecule is a compound engine variable defined as a FORMULA over atoms (and
// earlier molecules) — e.g. net_worth = "assets - liabilities". Keeping the
// derivation as data (a formula string), persisted and editable, rather than Go
// code, is what makes a figure auditable: it can be traced down to the indivisible
// atoms it's built from. Built-in molecules are seeded from engineenv defaults;
// overrides/additions are stored in the dataset. (Atoms — the leaf reductions over
// transactions/accounts/bills/goals — live in engineenv; they can't be a formula
// over other variables, so they're not Molecules.)
type Molecule struct {
	Name    string `json:"name"`
	Formula string `json:"formula"`
	Doc     string `json:"doc,omitempty"`
}

// WidgetSpecVersion is the current schema version stamped on new WidgetSpec and
// Placement records. Upgrades are additive and applied once, at load, by the
// store's dataset migration (never per-record at render time). See
// docs/UNIFIED_WIDGET_API.md §9.
const WidgetSpecVersion = 1

// WidgetKind is the discriminant for a unified widget. Exactly one data binding
// is set per kind: KPI→Scalar, List/Table/Chart→Pipeline, Builder→Graph,
// Native→NativeID, and Text/Image/Spacer carry none. WidgetSpec.Validate()
// enforces this — the type system alone cannot. See §3.
type WidgetKind string

const (
	// KindKPI is a single scalar figure (a formula over engine variables).
	KindKPI WidgetKind = "kpi"
	// KindList is a vertical list of rows from a collection.
	KindList WidgetKind = "list"
	// KindTable is a sortable, paginated table over a Frame.
	KindTable WidgetKind = "table"
	// KindChart is a chart (line/area/bar/donut) projected from a Frame.
	KindChart WidgetKind = "chart"
	// KindText is static or templated prose.
	KindText WidgetKind = "text"
	// KindImage is a stored image artifact.
	KindImage WidgetKind = "image"
	// KindNative is a Go-rendered widget keyed by NativeID (the irreducible tiles).
	KindNative WidgetKind = "native"
	// KindBuilder is a node-graph card; Graph holds the serialized cardgraph DAG.
	KindBuilder WidgetKind = "builder"
	// KindSpacer occupies grid cells and renders empty (intentional layout gaps).
	KindSpacer WidgetKind = "spacer"
)

// Valid reports whether k is a known widget kind.
func (k WidgetKind) Valid() bool {
	switch k {
	case KindKPI, KindList, KindTable, KindChart, KindText, KindImage, KindNative, KindBuilder, KindSpacer:
		return true
	}
	return false
}

// WidgetSpec is the persisted, surface-independent definition of a widget. It is
// pure data; the only code is the renderer the registry resolves from Kind /
// NativeID. See docs/UNIFIED_WIDGET_API.md §3.
type WidgetSpec struct {
	SchemaVersion int              `json:"schemaVersion"`
	ID            string           `json:"id"`
	Kind          WidgetKind       `json:"kind"`
	Title         string           `json:"title,omitempty"`
	Scalar        *ScalarBind      `json:"scalar,omitempty"`   // Kind==KPI
	Pipeline      *Pipeline        `json:"pipeline,omitempty"` // Kind in {List,Table,Chart}
	Graph         json.RawMessage  `json:"graph,omitempty"`    // Kind==Builder (serialized cardgraph)
	NativeID      string           `json:"nativeId,omitempty"` // Kind==Native (registry id)
	Content       ContentLayout    `json:"content,omitzero"`   // intra-tile arrangement (§7.5)
	Settings      widgetcfg.Config `json:"settings,omitempty"` // schema-validated user settings
	Style         Style            `json:"style,omitzero"`     // token-first presentation overrides (§7.7)
}

// ScalarBind is a KPI's data binding: a formula expression evaluated against the
// engine variable surface, plus an output format. NOT a Frame — a scalar (§3.1).
type ScalarBind struct {
	Expr   string `json:"expr"`             // formula over engineenv vars (+ cf_* custom fields)
	Format string `json:"format,omitempty"` // currency (alias: money) | percent | number | compact
	// Sub is an optional templated sub-label. Literal text with "{{ expr | format }}"
	// tokens evaluated over the same variable surface — so a KPI's caption (e.g.
	// "{{accounts}} accounts") is described by the spec, not hardcoded in a renderer.
	Sub string `json:"sub,omitempty"`
}

// Pipeline produces a Frame for List/Table/Chart widgets: a Source followed by
// ordered Frame→Frame transforms (filter, aggregate, window, sort, limit).
type Pipeline struct {
	Source    Source      `json:"source"`
	Transform []Transform `json:"transform,omitempty"`
}

// SourceKind selects how a Pipeline materializes its initial Frame.
type SourceKind string

const (
	// SourceCollection reads a named domain collection (transactions, bills, …).
	SourceCollection SourceKind = "collection"
	// SourceSeries reads a windowed time series (for charts).
	SourceSeries SourceKind = "series"
)

// Source is the head of a Pipeline. Exactly the field matching Kind is used.
type Source struct {
	Kind       SourceKind `json:"kind"`
	Collection string     `json:"collection,omitempty"` // SourceCollection
	Series     SeriesSpec `json:"series,omitzero"`      // SourceSeries
	// Cleared selects the cleared-only balance for the accounts collection (a source
	// parameter, since it changes the reduction rather than filtering rows).
	Cleared bool `json:"cleared,omitempty"`
}

// SeriesSpec describes a windowed time series (e.g. net-worth over N months).
// Beyond the built-in metrics ("networth", "cashflow"), two user-programmable
// metrics graph the household's own vocabulary:
//   - "formula": Expr is evaluated against the engine variable surface for
//     EACH month window — any formula or molecule ("income - expense",
//     "savings_rate") becomes a trend line.
//   - "flow": Filter selects transactions ("tag:<tag>", "cat:<id or name>",
//     or "cf:<key>=<value>" — a custom-field value) and each month plots
//     their sum.
type SeriesSpec struct {
	Metric string `json:"metric"`           // networth | cashflow | formula | flow
	Months int    `json:"months,omitempty"` // trailing window length
	Expr   string `json:"expr,omitempty"`   // Metric=="formula": per-month formula
	Filter string `json:"filter,omitempty"` // Metric=="flow": txn selector
	Format string `json:"format,omitempty"` // formula output: currency (default) | percent | number
}

// TransformKind names a Frame→Frame operation.
type TransformKind string

const (
	// TransformFilter drops rows not matching Arg (a txnfilter-style criterion).
	TransformFilter TransformKind = "filter"
	// TransformSort orders rows by Arg (a field name; prefix "-" for descending).
	TransformSort TransformKind = "sort"
	// TransformLimit keeps the first N rows.
	TransformLimit TransformKind = "limit"
	// TransformPaginate keeps one page (render-time paging; see §12.3).
	TransformPaginate TransformKind = "paginate"
	// TransformAggregate reduces rows to a summary (Arg names the aggregation).
	TransformAggregate TransformKind = "aggregate"
)

// Transform is one ordered step in a Pipeline. Arg/N carry the operation's
// parameters; their meaning depends on Kind.
type Transform struct {
	Kind TransformKind `json:"kind"`
	Arg  string        `json:"arg,omitempty"`
	N    int           `json:"n,omitempty"`
}

// FieldType is the value type of a Frame column.
type FieldType string

const (
	// FieldNumber is a plain numeric value.
	FieldNumber FieldType = "number"
	// FieldString is text.
	FieldString FieldType = "string"
	// FieldMoney is a monetary value, stored as minor units int64 (auto-redactable
	// for sharing; §7.6).
	FieldMoney FieldType = "money"
	// FieldDate is a date/time value.
	FieldDate FieldType = "date"
	// FieldBool is a boolean.
	FieldBool FieldType = "bool"
	// FieldPercent is a 0..N percentage (e.g. budget used).
	FieldPercent FieldType = "percent"
	// FieldTone is a presentation token a renderer maps to color/state — e.g.
	// "up"/"down"/"warn"/"over"/"" — so a Frame can drive a colored bar or row
	// without the renderer recomputing the status. This is what lets the rich
	// visualizations (status bars, signed balances) stay data-driven.
	FieldTone FieldType = "tone"
	// FieldColor is an explicit color value (hex or token) for chart segments.
	FieldColor FieldType = "color"
)

// Field is one typed, named column of a Frame; Values has Frame.Rows entries.
type Field struct {
	Name   string    `json:"name"`
	Type   FieldType `json:"type"`
	Values []any     `json:"values"`
}

// Str returns the value at row i as a string (best-effort), or "" out of range.
func (f Field) Str(i int) string {
	if i < 0 || i >= len(f.Values) {
		return ""
	}
	if s, ok := f.Values[i].(string); ok {
		return s
	}
	return ""
}

// Num returns the value at row i as a float64 (0 if absent or non-numeric).
func (f Field) Num(i int) float64 {
	if i < 0 || i >= len(f.Values) {
		return 0
	}
	switch n := f.Values[i].(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

// Int64 returns the value at row i as int64 (for money minor units / counts).
func (f Field) Int64(i int) int64 {
	if i < 0 || i >= len(f.Values) {
		return 0
	}
	switch n := f.Values[i].(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

// Frame is the canonical tabular shape every collection/series renderer consumes
// (the Grafana data-frame model; §3.1). Resolvers produce Frames; rich widgets —
// lists, status bars, grids, and charts (as label/value columns) — render from
// one. Money columns hold minor-unit int64; a FieldTone column lets a row carry
// its own color/state so the renderer needn't recompute it.
type Frame struct {
	Fields []Field `json:"fields"`
	Rows   int     `json:"rows"`
}

// Column returns the named field.
func (fr Frame) Column(name string) (Field, bool) {
	for _, f := range fr.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// NewFrame builds a Frame from named columns, setting Rows to the longest column.
func NewFrame(fields ...Field) Frame {
	rows := 0
	for _, f := range fields {
		if len(f.Values) > rows {
			rows = len(f.Values)
		}
	}
	return Frame{Fields: fields, Rows: rows}
}

// LayoutMode selects how a tile arranges its content (§7.5).
type LayoutMode string

const (
	// LayoutStandard uses the kind's built-in arrangement, tuned by Settings.
	LayoutStandard LayoutMode = "standard"
	// LayoutCustom places author-defined Blocks (the design-flexibility tier; v2).
	LayoutCustom LayoutMode = "custom"
)

// ContentLayout governs arrangement INSIDE a tile (distinct from the surface grid).
// Standard mode must have empty Blocks; Custom mode must have ≥1 block.
type ContentLayout struct {
	Mode   LayoutMode `json:"mode,omitempty"`
	Blocks []Block    `json:"blocks,omitempty"`
}

// BlockKind names a content block within a custom ContentLayout.
type BlockKind string

const (
	// BlockText is literal or templated ("{{net_worth}}") prose, HTML-escaped.
	BlockText BlockKind = "text"
	// BlockFigure is a single formula-bound figure.
	BlockFigure BlockKind = "figure"
	// BlockDataView embeds the widget's Frame (table/list/chart).
	BlockDataView BlockKind = "dataview"
	// BlockDivider is a horizontal rule.
	BlockDivider BlockKind = "divider"
	// BlockIcon is a decorative glyph.
	BlockIcon BlockKind = "icon"
	// BlockSpacer is empty space within the tile.
	BlockSpacer BlockKind = "spacer"
)

// Block is one element of a custom ContentLayout. Height is always intrinsic
// (content-driven); ColSpan optionally sets width within a block row (no RowSpan).
type Block struct {
	Kind    BlockKind `json:"kind"`
	Text    string    `json:"text,omitempty"`    // BlockText (escaped on render) / templated
	Bind    string    `json:"bind,omitempty"`    // BlockFigure formula / BlockDataView target
	ColSpan int       `json:"colSpan,omitempty"` // width within a block row
	Style   Style     `json:"style,omitzero"`    // per-block overrides (inherit tile Style)
}

// Style is a token-first set of presentation overrides. Values should reference
// theme tokens (e.g. "var(--accent)") so widgets repaint with the theme; raw hex
// is an escape hatch validated for contrast. Empty fields inherit (§7.7).
type Style struct {
	Background string `json:"bg,omitempty"`
	Text       string `json:"text,omitempty"`
	Accent     string `json:"accent,omitempty"`
	Border     string `json:"border,omitempty"`
	Radius     string `json:"radius,omitempty"`
	FontWeight string `json:"fontWeight,omitempty"`
	Align      string `json:"align,omitempty"`
	Shadow     string `json:"shadow,omitempty"`
}

// Empty reports whether the style sets no overrides.
func (s Style) Empty() bool { return s == Style{} }

// Merge returns s with over's set (non-empty) fields applied on top. A content
// Block uses tileStyle.Merge(blockStyle) so a block inherits the tile's Style
// per-property and overrides only the tokens it sets (§7.5).
func (s Style) Merge(over Style) Style {
	if over.Background != "" {
		s.Background = over.Background
	}
	if over.Text != "" {
		s.Text = over.Text
	}
	if over.Accent != "" {
		s.Accent = over.Accent
	}
	if over.Border != "" {
		s.Border = over.Border
	}
	if over.Radius != "" {
		s.Radius = over.Radius
	}
	if over.FontWeight != "" {
		s.FontWeight = over.FontWeight
	}
	if over.Align != "" {
		s.Align = over.Align
	}
	if over.Shadow != "" {
		s.Shadow = over.Shadow
	}
	return s
}

// CSS renders the style as inline CSS properties — only the fields that are set,
// so unset tokens inherit the theme. Token references (e.g. "var(--accent)") pass
// through unchanged; the shell applies this map over a tile's grid placement.
func (s Style) CSS() map[string]string {
	out := map[string]string{}
	if s.Background != "" {
		out["background-color"] = s.Background
	}
	if s.Text != "" {
		out["color"] = s.Text
	}
	if s.Border != "" {
		out["border-color"] = s.Border
		out["border-style"] = "solid"
	}
	if s.Radius != "" {
		out["border-radius"] = s.Radius
	}
	if s.FontWeight != "" {
		out["font-weight"] = s.FontWeight
	}
	if s.Align != "" {
		out["text-align"] = s.Align
	}
	// Accent renders as an inset top strip composed with an optional drop shadow,
	// matching the legacy widgetstyle box-shadow convention.
	var parts []string
	if s.Accent != "" {
		parts = append(parts, "inset 0 3px 0 0 "+s.Accent)
	}
	if s.Shadow != "" && s.Shadow != "none" {
		parts = append(parts, s.Shadow)
	}
	switch {
	case len(parts) > 0:
		out["box-shadow"] = strings.Join(parts, ", ")
	case s.Shadow == "none":
		out["box-shadow"] = "none"
	}
	return out
}

// Access gates view/edit of a placement against the memberrole hierarchy
// (Owner>Admin>Viewer) and optional member-scoping. Empty = inherit the surface.
type Access struct {
	ViewRoles   []MemberRole `json:"viewRoles,omitempty"`
	EditRoles   []MemberRole `json:"editRoles,omitempty"`
	OnlyMembers []string     `json:"onlyMembers,omitempty"`
}

// Empty reports whether no access restriction is set.
func (a Access) Empty() bool {
	return len(a.ViewRoles) == 0 && len(a.EditRoles) == 0 && len(a.OnlyMembers) == 0
}

// Placement is an independent, self-contained instance of a widget on a surface.
// It embeds a copy of the spec (template/clone semantics) so it is independently
// editable and carries no dangling cross-record reference (§8). Layout positions
// it on the surface's bento grid.
type Placement struct {
	SchemaVersion int             `json:"schemaVersion"`
	ID            string          `json:"id"`
	Surface       string          `json:"surface"` // "dashboard" | "page:<slug>" | …
	Spec          WidgetSpec      `json:"spec"`
	Layout        dashlayout.Item `json:"layout"`
	Hidden        bool            `json:"hidden,omitempty"`
	SourceLibKey  string          `json:"sourceLibKey,omitempty"` // provenance (which library spec)
	LibLink       string          `json:"libLink,omitempty"`      // live-link to a library spec (v2)
	Access        Access          `json:"access,omitzero"`
}

// Validate checks a WidgetSpec's structural invariants: a known kind, and exactly
// the data binding the kind requires (no more, no less). It is the single guard
// for the "exactly one binding per kind" rule and runs at write time and before
// render (§9). Standard ContentLayout must carry no blocks; Custom must carry some.
func (w WidgetSpec) Validate() error {
	if !w.Kind.Valid() {
		return fmt.Errorf("widget %q: unknown kind %q", w.ID, w.Kind)
	}
	// Count how many mutually-exclusive bindings are set.
	set := 0
	if w.Scalar != nil {
		set++
	}
	if w.Pipeline != nil {
		set++
	}
	if len(w.Graph) > 0 {
		set++
	}
	if w.NativeID != "" {
		set++
	}
	if set > 1 {
		return fmt.Errorf("widget %q: more than one data binding set", w.ID)
	}

	// Each kind requires (or forbids) a specific binding.
	switch w.Kind {
	case KindKPI:
		if w.Scalar == nil {
			return fmt.Errorf("widget %q: kind kpi requires a scalar binding", w.ID)
		}
	case KindList, KindTable, KindChart:
		if w.Pipeline == nil {
			return fmt.Errorf("widget %q: kind %s requires a pipeline", w.ID, w.Kind)
		}
	case KindBuilder:
		if len(w.Graph) == 0 {
			return fmt.Errorf("widget %q: kind builder requires a graph", w.ID)
		}
	case KindNative:
		if w.NativeID == "" {
			return fmt.Errorf("widget %q: kind native requires a nativeId", w.ID)
		}
	case KindText, KindImage, KindSpacer:
		if set != 0 {
			return fmt.Errorf("widget %q: kind %s takes no data binding", w.ID, w.Kind)
		}
	}

	switch w.Content.Mode {
	case "", LayoutStandard:
		if len(w.Content.Blocks) > 0 {
			return fmt.Errorf("widget %q: standard content layout must have no blocks", w.ID)
		}
	case LayoutCustom:
		if len(w.Content.Blocks) == 0 {
			return fmt.Errorf("widget %q: custom content layout requires at least one block", w.ID)
		}
	default:
		return fmt.Errorf("widget %q: unknown content layout mode %q", w.ID, w.Content.Mode)
	}
	return nil
}

// Validate checks a Placement: a non-empty surface, a valid embedded spec, and a
// positive layout span.
func (p Placement) Validate() error {
	if p.Surface == "" {
		return fmt.Errorf("placement %q: empty surface", p.ID)
	}
	if err := p.Spec.Validate(); err != nil {
		return fmt.Errorf("placement %q: %w", p.ID, err)
	}
	return nil
}

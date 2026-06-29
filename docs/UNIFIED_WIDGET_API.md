# Unified Widget System — API Design

> **Status:** design / spec-in-progress (no feature code yet). Per the project process
> rule, we agree the spec before building. This doc is that spec. Hardened through **3
> rounds of adversarial review** against the actual codebase (findings folded in inline,
> tagged "round-N finding"). v1 core assessed buildable; Custom content layout + Share
> phased to v2 (§10).
>
> **Author:** Claude (Opus), 2026-06-28. **Owner to review:** Cam.
>
> **Supersedes:** the three current widget subsystems (dashboard closures,
> custom-page `PageWidget`, Widget Builder cards). Reconciles with
> [`WIDGET_BUILDER_DESIGN.md`](./WIDGET_BUILDER_DESIGN.md) — the node-graph builder
> becomes one *authoring front-end* that emits unified specs (see §11).

## 1. Goal & the one-sentence model

Make a widget a **typed, validated, persisted spec** that any surface renders through
**one shell**, fed by **one data pipeline**, so widgets can be defined once, placed
anywhere, and adjusted per placement — with no hardcoding and a contract stable enough
to evolve for years without breaking saved data.

> **A widget is data. A placement is where a copy of that data lives. A renderer is the
> only code.**

### Design decisions already locked (by owner)
- **Persistence: SQLite, fresh start.** No migration of the legacy `appkv` layout blobs.
  (Revisit per §10 risk R2 — a read-convert is cheap and avoids resetting users.)
- **Big-bang refactor**, internally ordered so each step compiles + tests green.

### Non-goals
- Not a third-party plugin marketplace. Widgets are first-party Go; this is an internal
  contract, not an external ABI. (The contract is *designed* additively so a plugin
  surface is possible later, but that is out of scope.)
- Not arbitrary code execution. Data logic stays inside the sandboxed `formula`/`rules`
  engines — a total, terminating dataflow, never a Turing tarpit.
- Not a layout-engine rewrite. We unify *onto* the existing `dashlayout.Pack` model.

## 2. What this fixes (the failures in today's three systems)

| Problem today | Unified fix |
|---|---|
| Dashboard widgets are hardcoded Go closures across **4 files** | One **registry**, one registration per type (§6) |
| Two tile shells (`ui.Widget` vs `customTile`) drift | **One shell** takes `(WidgetSpec, Placement, surface)` (§7) |
| Builder cards not persisted; lost on `wipe` | All specs/placements in **SQLite**, travel with export (§8) |
| `vbDatasets` "bills" bug (expenses ≠ upcoming bills) | One **Source resolver**; one correct definition per source (§5) |
| Dashboard tiles compute data differently from page widgets | One **data pipeline** to a canonical frame (§5) |
| `widgetstyle` piggybacks `_`-keys on the config map, untested | Style is a **typed sibling**, validated, tested (§4, §9) |

## 3. The core types (`internal/domain`)

Per CLAUDE.md rule 7 (strong core schema; flexibility via *validated* fields, not loose
maps), the spec is **strongly typed with a discriminated union per widget type**. The
loose `map[string]string` config survives only as the *user-tunable settings* layer,
which is gated by a `widgetcfg.Schema` validator — not as the structural model.

```go
// WidgetSpec is the persisted, surface-independent definition of a widget.
type WidgetSpec struct {
    SchemaVersion int         // for additive evolution + migration (§9)
    ID            string      // stable id (library catalog key)
    Kind          WidgetKind  // discriminant: KPI | List | Table | Chart | Text | Image | Native
    Title         string
    Scalar        *ScalarBind // when Kind==KPI: direct engineenv/formula eval (NOT a Frame; §3.1)
    Pipeline      *Pipeline   // when Kind in {List,Table,Chart}: collection/series → Frame (§3.1)
    Graph         json.RawMessage // when Kind==Builder: serialized cardgraph DAG (see §11 — NOT a cardgraph import)
    Content       ContentLayout // intra-tile content arrangement: Standard | Custom (§7.5)
    Settings      Settings    // user-tunable, schema-validated (rows, window, format, accent…)
    Style         Style       // typed, token-first presentation overrides (theme tie-in §7.7; was widgetstyle's _-keys)
    NativeID      string      // when Kind==Native: registry id of the Go renderer (e.g. "smart-digest")
}
// WidgetKind discriminant: KPI | List | Table | Chart | Text | Image | Native | Builder | Spacer.
// Exactly one of Scalar / Pipeline / Graph / NativeID is set, per Kind. Spacer has none.
// This invariant is NOT enforced by the type system — WidgetSpec.Validate() checks it
// explicitly ("more than one data binding set" / "binding missing for kind") and runs at
// write time + before render (§9). A comment alone is not the guarantee.

// Placement is an independent, self-contained instance on a surface.
// It EMBEDS a copy of the spec (template/clone semantics) so it is independently
// editable and carries no dangling cross-record reference. See §8 rationale.
type Placement struct {
    SchemaVersion int
    ID            string          // placement id
    Surface       SurfaceID       // "dashboard" | "page:<slug>" | future surfaces
    Spec          WidgetSpec      // the cloned, owned definition
    Layout        dashlayout.Item // ColSpan/RowSpan/Importance + order
    Hidden        bool
    SourceLibKey  string          // optional: which library spec this was cloned from (provenance)
    Access        Access          // per-placement ACL (roles/members); empty = inherit surface (§13.2)
    LibLink       string          // optional live-link to a widgetlibrary spec (v2, §8)
}
// Access gates view/edit against the existing memberrole model (Owner>Admin>Viewer).
type Access struct {
    ViewRoles []domain.MemberRole // empty = all; else min role to view
    EditRoles []domain.MemberRole // empty = inherit; else min role to edit
    OnlyMembers []string          // optional: restrict to specific member IDs (scoped widgets)
}
```

### 3.1 Two data shapes, not one — scalar vs. frame

> **Revised after adversarial review.** The v1 claim ("one canonical Frame for
> everything") was wrong: a KPI is a *scalar*, and forcing it through a 1×1 Frame +
> "aggregate transform" is indirection, not composition. We keep **two** data shapes,
> each minimal, sharing the shell but not the data model.

**Scalar shape (KPI)** — resolves directly through the unchanged engine chain
(`engineenv.Vars → formula.Eval`); no Frame:
```go
type ScalarBind struct {
    Expr   string // formula over engineenv vars (+ entity-scoped custom fields, §5.2)
    Format string // money | percent | number | compact
}
```

**Frame shape (List / Table / Chart)** — the Grafana data-frame model, genuinely useful
when there *are* rows/series:
```go
type Pipeline struct {
    Source    Source       // produces a Frame
    Transform []Transform  // ordered, frame → frame (filter, aggregate, window, sort, limit)
}
type Source struct {
    Kind       SourceKind  // Collection | Series
    Collection string      // Collection: a domain collection id (transactions, bills…)
    Series     SeriesSpec  // Series: windowed time-series (for charts)
}
type Frame struct {
    Fields []Field        // columns, each typed + named
    Rows   int
}
type Field struct {
    Name   string
    Type   FieldType      // Number | String | Money | Date | Bool
    Values []any          // len == Frame.Rows
}
```

Renderer mapping:
- **KPI** → `ScalarBind` (direct eval). Rich sub-labels (e.g. networth's "▲ N% this
  month") are domain computations that don't live in scalar/frame space → such KPIs are
  **Native** (see §4), not DataDriven.
- **List / Table** → rows of the `Frame`; columns from `Settings`.
- **Chart** → `Frame` → `chartspec.Spec` via a declared field→axis mapping in `Settings`.
- **Text / Image** → no data binding (static/templated).
- **Native** → registered Go renderer (the irreducible widgets).
- **Builder** → the node-graph DAG, rendered by the builder dispatcher (§11).

> **What "unified" actually means** (honest scope): the unification is real and complete
> at the **shell, registry, persistence, placement, style, and config** layers — that is
> the bulk of the value. The *data* layer is deliberately plural (scalar / frame / native
> / graph) because the domain is plural. We do not claim one binding model fits all.

## 4. Widget classes — the honest audit

An actual audit of the 22 dashboard renderers (`dashboard.go:221-315`) against the
criterion *"fully expressible by spec, zero domain-package code"* — not the optimistic
guess in v1:

| Class | Widgets | Why |
|---|---|---|
| **DataDriven** (spec only) | `recent` (List). KPI *bodies* of networth/income/spending/liabilities/assets via `ScalarBind` | Genuinely a formula/collection over engineenv + a resolver |
| **Native** (Go renderer) | `attention`, `smart-digest`, `anomaly-hub`, `health`, `freshness`, `highlight`, `safetospend`, `accounts`, `budgets`, `goals`, `todo`, `trend`, `cashflow`, `savings`, `breakdown`, `bills`, **and the KPI sub-labels** | Need `ledger`/`budgeting`/`goals`/`tasksort`/`categorytree`/`safespend`/`bills`/`freshness` domain logic, time-windowing, or row-level interactivity (§7.1) |

**This is the central honest correction to v1.** The DataDriven class is *small* (~recent
+ five KPI bodies). Pushing the rest through a Frame would force the resolver layer to
re-import and re-implement nearly every domain package — that's not simplification, it's
relocation. So:

- **DataDriven** (`Kind in {KPI,List,Table,Chart,Text,Image}`): defined by the spec.
  Config-based, no code to add an instance. The growth area for *user-authored* widgets.
- **Native** (`Kind==Native`): a registered Go renderer keyed by `NativeID`. Carries
  `Title`/`Settings`/`Style`/`Layout`, so it places, styles, hides, configures (via its
  `Schema`), and persists *identically* — only its body is code.

**The win is the platform, not the pipeline.** Unification is real and complete at the
**shell, registry, persistence, placement, style, config, and visibility** layers — every
widget, Native or DataDriven, becomes placeable-anywhere, config-driven, and
SQLite-persisted. The data layer stays plural because the domain is plural. We do **not**
force `smart-digest` into a formula string, and we do **not** oversell the pipeline.

## 5. Data sources & the engines (no engine rewrites)

### 5.1 Source resolver
A single `internal/widgetsource` package owns the one true definition of each collection
source → `Frame`. `SourceBills` = `bills.UpcomingAll()` *everywhere* (this is where the
current `vbDatasets` bug dies). Resolvers are pure and table-tested.

### 5.2 Formula + custom fields (scoped, entity-namespaced)
- Scalar (KPI) binds resolve through the existing chain unchanged:
  `engineenv.Vars(Data) → formula.Eval(expr, Env{Vars})`.
- **Custom fields → formulas (bounded v1):** today `engineenv.Vars` exposes 11 global
  floats; per-entity custom fields are *not* exposed. `customfields.Def` carries an
  `EntityType`, and the *same key* can exist on different entity types (a `priority` field
  on both transactions and accounts), so a bare `cf_<key>` is ambiguous. v1 therefore:
  - exposes **numeric custom fields only**, **namespaced by entity type**:
    `cf_txn_<key>`, `cf_acct_<key>`, `cf_budget_<key>`, `cf_goal_<key>`, `cf_member_<key>`
    — no cross-entity collision possible;
  - aggregates with a **single default (`sum` over that entity type's collection)**, scope
    therefore unambiguous;
  - adds the 11 reserved engineenv names **and** the `cf_` prefix to
    `customfields.ReservedKey()` so a user can't define a field that shadows a system var.
  - Text/date/select fields and alternative aggregations (avg/min/max/count) are explicitly
    deferred (§14).

### 5.3 Charting
`Chart` widgets map a `Frame` to `chartspec.Spec` (`Kind ∈ line|area|bar|donut`) via a
declared field→axis binding in `Settings`; rendering uses the existing D3 shim. No change
to `chartspec`.

## 6. The registry — split to protect native testability

> **Revised after adversarial review.** A single descriptor holding a
> `func(RenderCtx) ui.Node` would force the registry package to import `ui` (a
> `//go:build js && wasm` type), making it un-importable in native test binaries and
> violating CLAUDE.md rule 2. We split into **two registries joined by the ID string**.

**`internal/widgetregistry` — platform-independent (native-testable):**
```go
type Descriptor struct {
    ID      string            // kind id or native id
    Name    string            // human label (replaces widgetManagerTitleKeys)
    IconID  string            // icon key (resolved to a glyph in the wasm layer)
    Class   Class             // DataDriven | Native
    Schema  widgetcfg.Schema  // settings schema for the config gear + validation
    Default func() WidgetSpec // seed spec for the catalog/library
}
func Register(d Descriptor)
func Get(id string) (Descriptor, bool)
func Catalog() []Descriptor   // drives the "add widget" picker on every surface
```
No `ui`/`icon`/`syscall/js` imports → unit-tested on native Go like every other logic
package.

**`internal/widgetrender` — wasm-only (`//go:build js && wasm`):**
```go
func Register(id string, render func(RenderCtx) ui.Node) // Native bodies only
func Render(id string, ctx RenderCtx) (ui.Node, bool)
```
DataDriven kinds use the shared kind renderers (KPI/List/Table/Chart) and need no entry
here; only Native widgets register a body. The two registries share the `ID` as the join
key, and a startup assertion verifies every Native `Descriptor` has a matching
`widgetrender` entry (catches drift at boot, not in production).

This still collapses the four scattered maps (`dashboard.go` renderers,
`widgetManagerTitleKeys`, `ui/widget.go` route+icon, `widgetcfg/builtins.go`) into **one
descriptor (+ one render fn for Native)** per widget. Adding a widget = one `Register`
call (two for Native). No Open/Closed violation; testability preserved.

## 7. One shell (`internal/ui/widget.go`, rebuilt)

A single component renders any placement:

```go
func Widget(p domain.Placement, surface SurfaceCtx) ui.Node
```

It owns (merged from `ui.Widget` + `customTile`): the bento tile *chrome* — drag/resize,
roving-tabindex keyboard model (APG grid), the config gear (flip panel from
`Descriptor.Schema`), the style overlay (`Style.Effective`), and body dispatch. Custom-page
tiles gain every chrome feature they currently lack, for free. `SurfaceCtx` carries the
surface-varying behavior (drag enabled, resize enabled, layout/mode atoms, persist
callbacks) — this is *parameterization* of one component, replacing two divergent
components; it is not claimed to be fewer fields, only one code path.

### 7.1 Body interactivity & the On*-in-loop gotcha (BLOCKER-class constraint)
CLAUDE.md's critical rule: **never call `On*` prop options inside a variable-length loop.**
The todo checkbox, bills "pay now", freshness dismiss, and attention per-item dismiss are
all row-level side-effects. Therefore:
- The shell **never** iterates data rows and attaches `On*` itself.
- DataDriven List/Table renderers that need a row action wrap each row in its own
  component: `ui.CreateElement(Row, rowProps)` with plain `func` callbacks passed as props
  (the row owns its handler hook). This is the only compliant pattern.
- Any widget whose body is *primarily* interactive state (todo, attention) is **Native** —
  its registered renderer owns the per-row component boundary directly. This is why §4
  classifies `todo`/`attention` as Native, not DataDriven.

There is **no** "interactive Frame": side-effects are not data, and the Frame stays a pure
read-model. A future declarative `Effects` layer (trigger→action) is noted in §14 but is
out of v1 scope; until then, interactivity = Native + per-row component.

### 7.2 One layout engine for every surface (owner requirement)
Every surface — dashboard **and** every custom page — renders through the **same**
`dashlayout` bento grid. But the round-2 review proved the current functions are
**dashboard-hardcoded** and must be parameterized first; this section specifies those
changes (they are prerequisites, not assumptions).

```
items    := placementsToItems(placements)               // []dashlayout.Item
arranged := dashlayout.Arrange(items, mode, defaults)    // defaults per surface (see below)
layout   := dashlayout.Pack(arranged, surface.Cols)      // iOS-style first-fit reflow
```

**Required signature changes (today these hardcode the dashboard):**
- `Reconcile(saved, defaults []Item)` — currently `reconcile.go` calls `DefaultItems()`
  internally, so running it on a page **injects all 22 dashboard tiles** and drops the
  page's own UUID items (they lack a `":"` so `IsCustomID` rejects them). Fix: pass
  `defaults` per surface — dashboard passes `DefaultItems()`, **pages pass `nil`**. The
  `nil` path must **keep every saved item unconditionally** (they came from the DB → they're
  real) — *not* fall through to the current `known || IsCustomID` filter, which would drop
  any page widget whose id lacks a `":"` (round-3 finding). So page placement ids need no
  special format, and Reconcile-nil splices nothing and drops nothing. Touches every call site.
- `Arrange(items, mode, defaults)` — `ModeAutoDefault` ranks by `DefaultItems()` position,
  so off-dashboard it makes **every item rank-equal → a silent no-op**. Fix: canonical order
  comes from the passed `defaults`; for pages, `ModeAutoDefault` is hidden/disabled and only
  `Custom`/`Importance` are offered.
- **Column count** is a `SurfaceCtx.Cols` field, not the current `const gridCols = 4`
  (`ui/widget.go:91`) / literal `4` (`custompage.go:135`). The shell threads `surface.Cols`
  into `Pack`. Dashboard default 4; a narrow page can set 2.

This kills the custom-page `customTile` ad-hoc flow (§Appendix). The grid is **reflowing**
(`Pack` is first-fit row-major), so hiding/removing a widget closes the gap automatically —
the "iOS-style" behavior the owner described.

### 7.2.1 Surface-scoped state (the global-atom trap)
The current shell reads **global** atoms (`uistate.UseLayoutItems()` → `"dashboard:layout"`,
plus `current-tile`/`grabbed-tile`/`drag-source`/`drag-preview`). If the unified shell kept
reading these, **every surface would share the dashboard's layout and drag state** — a page
tile would pack against the 22 dashboard items, and a drag on one surface would register on
another rendered concurrently. So:
- Layout/mode/drag/grab/preview atoms are **namespaced by `SurfaceCtx.ID`**
  (`<surfaceID>:layout`, `<surfaceID>:grabbed-tile`, …) via `state.UseAtom(surfaceID+":"+key, …)`.
  Each surface gets an isolated state namespace; concurrent surfaces never collide.
- Alternatively (and preferred for read-only embeds) the shell takes the precomputed
  `Layout` + interaction state through `SurfaceCtx` props and does **not** read atoms at all.
  Editable surfaces use namespaced atoms; static embeds use props. This is a prerequisite of
  the "one shell" claim, solved at the state layer before any shell code is written.

### 7.3 Standard sizes + spacer blocks
**Standard sizes.** Widgets snap to a small, named set of grid spans rather than arbitrary
col/row integers — consistent rhythm and a simpler resize UX:
```go
type SizePreset struct { Name string; ColSpan, RowSpan int }
// e.g. Small 1×1, Wide 2×1, Tall 1×2, Large 2×2, Banner 4×1
var StandardSizes = []SizePreset{ ... }
```
`Placement.Layout.ColSpan/RowSpan` are constrained to a preset at write time. Each
`Descriptor` may declare `MinSize`/`DefaultSize`/`AllowedSizes` so, e.g., a chart can't be
placed 1×1. `Pack` already clamps spans to the column count, so presets compose with reflow
safely.

> **Resize must go through a preset-aware path (round-2 finding).** Three existing APIs
> write free integer spans — `Resize`/`ResizeItem` and `CycleSpan(cur, max, shrink)` (used
> by `custompage.go:96`, which cycles 1→2→3→4 and would produce the non-preset span 3).
> The "presets only" guarantee requires a new `CyclePreset(cur, allowed)` that steps through
> `AllowedSizes`, and the free-span APIs are either gated behind it or deprecated. Patching
> only the shell is insufficient; all write paths must funnel through preset validation.

**Spacer blocks.** Because the grid reflows, intentional gaps need a real occupant. Add a
first-class `Kind == Spacer`: a `WidgetSpec` with no data binding and no chrome that simply
**occupies grid cells** (any standard size), rendering empty (a faint placeholder only in
edit mode). It participates in `Pack` like any tile, so users can shape the layout.

> **Spacer IDs must survive `Reconcile` (round-2 finding).** Reconcile keeps an item only if
> it's a default or `IsCustomID` (contains `":"`). A spacer id like `"spacer-1"` is **silently
> dropped on reload**. Convention: spacer ids are `"spacer:<uuid>"` so `IsCustomID` keeps
> them; the shell distinguishes spacers from `"wb:"` builder cards by the `"spacer:"` prefix
> (and `Spec.Kind==Spacer`). On read-only surfaces spacers render as nothing.

### 7.4 Locking movement per surface (owner requirement)
`SurfaceCtx` carries an explicit capability set so non-custom surfaces can be frozen:
```go
type SurfaceCtx struct {
    ID        SurfaceID
    Editable  bool   // master switch: drag, resize, add, delete, gear
    AllowMove bool   // drag/reorder
    AllowResize bool // size-preset cycling
    AllowAddRemove bool
    // …layout/mode atoms, persist callbacks
}
```
- **Custom pages:** fully `Editable` (move/resize/add/spacers).
- **Dashboard:** editable by default, toggleable.
- **System/non-custom surfaces** (e.g. a fixed report page embedding widgets): `Editable:
  false` → the shell renders tiles with **no grips, no resize handles, no gear, no
  drag listeners, and spacers collapse to nothing**. Same component, capabilities gated —
  not a separate read-only renderer. The capability check lives in one place (the shell),
  so "lock this surface" is a single flag, not scattered conditionals.

### 7.5 Intra-widget layout — content layout engine (owner requirement)
Layout is **two-tier**, and the tiers must not be confused:
- **Tier 1 — surface grid (§7.2):** where *tiles* sit on a surface (`dashlayout.Pack`).
- **Tier 2 — content layout (this section):** how *content* (figures, text, the data view)
  is arranged *inside* a single tile, to give each widget flavor and design flexibility.

Every `WidgetSpec` carries a `ContentLayout` describing tier 2:
```go
type ContentLayout struct {
    Mode   LayoutMode // Standard | Custom
    Blocks []Block    // Custom mode only
}
type Block struct {
    Kind    BlockKind // Text | Figure | DataView | Divider | Icon | Spacer
    Text    string    // Text: literal or templated ("{{net_worth}} this month"), HTML-escaped
    Bind    string    // Figure: a scalar formula expr; DataView: which frame/list to embed
    ColSpan int       // optional width within a block row (e.g. 2 figures side by side); NO RowSpan
    Style   Style     // per-block typography/color/align (inherits tile Style per-property)
}
```

**Standard layout (`Mode==Standard`, the default).** The renderer's built-in arrangement
for that `Kind` — e.g. KPI = title row / big figure / sub-label; List = title / rows;
Chart = title / chart / legend. Tuned by `Settings` (show title, alignment, density), not
by hand-placed blocks. Keeps the common case simple and consistent with the standard tile
sizes (§7.3). Most widgets never leave Standard.

**Fully custom layout (`Mode==Custom`).** The author places `Block`s for design control —
headings, captions, multiple figures, a divider, an icon, and the data view. This is the
"flavor" tier. **Standard mode must have empty `Blocks`; Custom mode must have ≥1 block —
`WidgetSpec.Validate()` enforces this** (a Standard spec with stray blocks is rejected, not
silently ignored).

> **Correction (round-3 finding): content layout is NOT `dashlayout.Pack`.** Pack is an
> integer grid bin-packer that assumes **uniform cell heights** — correct for tiles (CSS
> grid rows are a fixed design rhythm), **wrong for content**. A `Text` block has *intrinsic,
> width-dependent* height (a caption wraps to 1–3 lines depending on the tile's `ColSpan`),
> which the author cannot know at spec time and Pack cannot compute (it's tile-width
> independent). Forcing blocks through Pack produces clipped or empty cells. So Custom
> `ContentLayout` uses a **responsive flex/CSS-auto-flow column** model: blocks stack in
> order, each sized to its content (`grid-auto-rows: auto` / flex-column). `Block.Cell` keeps
> an optional `ColSpan` for *width* within a block row (e.g. two figures side by side) but
> **no RowSpan / no bin-packing** — height is always intrinsic. Pack stays on the surface
> grid (§7.2) only.

**Block.Style vs WidgetSpec.Style.** `Block.Style` inherits from `WidgetSpec.Style`
per-property (a block overrides only the tokens it sets; unset tokens fall through to the
tile style). Background is owned by the tile (`WidgetSpec.Style`); blocks set typography/
color/align only — keeps one background per tile.

**Templating is new, small, and escaped (round-3 finding).** Figure `Bind` reuses
`formula.Eval` (it's an arithmetic expr → value). But Text's `{{var}}` is **not** a formula
expression — it's a separate ~60-line scanner (find `{{`/`}}`, look up the name in the
engineenv/frame namespace, format, splice). It is new code with its own tests and rules,
not "reuse": **all interpolated values are HTML-escaped** before splice (String vars are
untrusted → XSS guard); an **unknown var renders as an empty span with a visible validation
warning** (not silent); an **unterminated `{{` is a validation error** surfaced at write
time. The doc no longer pretends this is free.

**Where it connects:** the Widget Builder's `viz.stack` composite nodes (§11) naturally
emit a `Custom` `ContentLayout`; the "add KPI/Table" pickers emit `Standard`. Native
widgets own their body and ignore `ContentLayout` (they *are* custom code). `Block.Kind ==
DataView` is how a custom layout embeds the widget's actual Frame/Table/Chart among the
decorative blocks — so a Table can sit beneath a hand-written caption and a KPI figure in
one tile.

### 7.6 Shareable widget snapshot API (owner requirement)
Any widget can be captured as an image for social sharing. Because the unified shell renders
every widget, the share affordance lives in **one place** and works for all kinds for free.

**Capability + entry point.** `Descriptor.Shareable bool` (default true; Native widgets can
opt out) gates a share action in the tile's overflow menu (`ui.OverflowMenu` already exists).
The shell exposes:
```go
// wasm layer (internal/widgetshare, //go:build js && wasm)
func Capture(el dom.Element, opts ShareOptions) (Blob, error) // DOM tile → PNG
func Share(blob Blob, meta ShareMeta) error                   // Web Share API → fallback
```

**Capture path.** The tile mixes HTML (KPI/text/list) and D3 **SVG** (charts). This needs a
**new** JS shim `cashfluxCaptureWidget(el, opts) → PNG blob` — it does **not** exist today
and is *not* a trivial copy of `cashfluxRenderChart` (round-3 finding). It is ~200-400 lines
that must handle the known-hard problems explicitly: **font preflight** (await
`document.fonts.ready` so self-hosted Inter/Fraunces rasterize, not fallbacks); **animation
settle** (wait for the chart's `data-cf-drawn` / an rAF before capture so SVG line/area
draw-on animations aren't frozen mid-stroke); **CSS-variable + class inlining** for the SVG;
and **tainted-canvas avoidance** (all assets are same-origin/local, which helps). It is a
scoped sub-task with its own tests, registered in `web/index.html` like the other shims.

**Share path.** `navigator.canShare({files})` → `navigator.share` (native share sheet on
mobile/desktop where supported); **fallbacks**: download the PNG, or copy to clipboard
(`navigator.clipboard.write`). No server, no upload — capture is fully client-side, matching
the local-first model.

**Privacy guard (non-negotiable for a finance app).** Sharing a tile can leak balances,
net worth, account names. So:
- `ShareOptions.Redact` (default **on**) replaces monetary values with a blurred/masked
  glyph (`••••`) and can hide account/member names — the *shape* of the chart/trend shares,
  the *figures* don't. The user explicitly toggles redaction off per share, with a clear
  "this reveals exact amounts" confirm.
- Redaction works by a concrete DOM marker, since the capture shim is generic and can't
  infer semantics (round-3 finding). Every sensitive node carries **`data-cf-redact`** (a
  data attribute) and the shim masks the text content of marked nodes before rasterizing.
  - **DataDriven** widgets emit it automatically: the Frame renderer tags any cell whose
    `Field.Type==Money` (and configured name fields) with `data-cf-redact`.
  - **Native** widgets (16 of 22 — the majority) must each add `data-cf-redact` to their
    sensitive DOM nodes, **including D3 chart axis `<text>`** (the chart shim adds it to
    money-formatted tick labels). This is an explicit task in the §10 plan, not handwaving —
    it touches all Native renderers + the chart shim.
- This honors CLAUDE.md's privacy posture: nothing leaves the device without an explicit,
  informed action, and the safe default doesn't expose numbers.

**Flavor / branding.** Capture composes the tile into a shareable **card frame**: optional
app watermark, the widget title, and a subtle background — so a shared KPI looks like a
designed social card, not a cropped screenshot. The frame reuses the `ContentLayout`/`Style`
tokens (§7.5) so it inherits the app's visual language. Frame style is a `ShareOptions` field.

**Testability.** The compose/redaction *policy* (which fields mask, what the card frame
contains) is a pure, native-testable function over the `Frame` + `ShareOptions`; only the
rasterization + Web Share calls are wasm/JS.

### 7.7 Theming engine tie-ins (owner requirement)
CashFlux has a real design-token engine — `internal/theme` (pure: colors, `Radius`, fonts,
`Scale`, `Density`, `IconStroke`; WCAG-AA contrast-validated; presets; JSON-shareable) →
`CSSVars()` applied to `:root` by `uistate.ApplyTheme`. The widget system **binds to this
token engine**; it does not invent its own colors. This is v1 — getting it wrong means
widgets that break in dark mode or ignore the user's theme.

**1. Style is token-first, not hex-first.** `Style` overrides are expressed as **theme
tokens** (`var(--accent)`, `var(--bg-card)`, `var(--text)`, `var(--text-dim)`, `var(--border)`,
`var(--up)`/`var(--down)`, `var(--radius)`), so a widget repaints automatically when the user
switches theme/light-dark/accent — no per-widget logic. A raw hex value is an **escape hatch
only**, and `Style.Validate()` runs it through the same `contrast` check `theme` uses, so a
user can't make an illegible tile.

**2. One cascade, theme at the root.** Resolution order (each layer sets only deltas):
`theme tokens (:root)` → `density` → surface → **global tile `Style`** (§8) → **per-widget
`Style`** → **per-block `Style`** (§7.5). `Style.Effective` merges the tile layers; the theme
sits beneath all of them as CSS-variable inheritance, so there's no copying of theme values
into specs (which would go stale when the theme changes).

**3. Density drives the chrome.** The shell reads `theme.Density` (Comfortable/Compact) for
tile padding, row height, and the standard-size rhythm (§7.3) — so density is a first-class
input to the layout, not a per-widget setting.

**4. Charts repaint with the theme.** The D3 shim already resolves CSS custom properties at
draw time (`chart.js` reads `--accent`/`--up`/`--down`/`--border`). So `Frame → chartspec`
emits **token references** (`var(--up)` for inflow, `var(--down)` for outflow,
`var(--accent)` for series) rather than baked hex — a theme change repaints every chart with
no re-resolve.

**5. Conditional formatting uses semantic tokens.** The threshold `Rules` (§13.4) emit style
deltas in semantic tokens (`var(--down)` for "over budget", `var(--up)` for "on track"), so
red/green always match the active theme's palette and stay contrast-valid.

**6. Share frame + per-widget accent inherit the theme.** The snapshot card frame (§7.6)
themes from the same tokens (the capture shim bakes computed `:root` vars), and the existing
per-tile accent (`widgetstyle.AccentKey`/`_accent`) maps onto `--accent`. A custom per-widget
accent is a token override, theme-aware.

**7. Themes are shareable + editable.** Because `theme` already round-trips JSON and has a
theme editor, the widget **portability** artifact (§13.6) can optionally bundle a theme, and
imported dashboards can carry their look — contrast-validated on import.

## 8. Persistence (`internal/store`)

Follows the confirmed JSON-doc pattern (`(id, data)` rows, `putJSON`, self-creating
`CREATE TABLE IF NOT EXISTS`):

- **`placements` table** + `Placements []domain.Placement` on `Dataset`. Each placement
  is **self-contained** (embeds its spec), so:
  - no foreign keys / dangling-reference bug class (the JSON-doc store can't enforce FKs);
  - export/import stays lossless and per-record;
  - editing one placement never spooks another.
- **`widgetlibrary` table** (optional, additive) — named *template* specs the user can
  clone from. Placing = clone library spec → new `Placement`. This delivers "define once,
  reuse" as a **template** (the semantics users actually want) without live-linking.
- Dashboard contents become the set of `Placements` with `Surface=="dashboard"`, replacing
  the `appkv:"cashflux:layout"` blob. Custom pages keep `[]Placement` on the page entity.
- **Global tile style** lives in `store.Settings` as a typed `Style` (was
  `widgetstyle._all`). `Style.Effective(settings.GlobalTileStyle, placement.Spec.Style)`
  is then fully defined — the merge has a real, single home.
- **No store-level `SchemaVersion` bump** — new tables self-create. Spec evolution is
  handled by the **single dataset-level migration path** (§9), not a second per-record one.

> **Cost & staleness tradeoff (acknowledged, not dismissed).** Embedding a full spec per
> placement costs ~1 KB/placement (~22 KB for a dashboard) in the export blob — acceptable
> for a local-first app, and worth it to keep export per-record and dangling-ref-free. The
> real cost is **staleness**: clone a chart to 8 placements, later fix its axis label →
> 8 edits. "Change everywhere" *is* a genuine user need beyond single-use tiles. We meet it
> with an explicit, additive opt-in rather than denying it:
> `Placement.LibLink string` — when set, the placement renders from the `widgetlibrary`
> spec (live) and its embedded `Spec` becomes an override layer (settings/style only).
> v1 ships the embedded/template path; `LibLink` is the planned answer for power users and
> is designed-for now so it's a field add, not a refactor.

## 9. Evolution & longevity (the "lasts for years" part)

The contract is built to evolve **additively** (the API-evolution literature's core rule):

1. **One migration path — dataset-level.** Spec/placement upgrades run inside the existing
   `store.migrate(Dataset)` step (keyed on the dataset `SchemaVersion`), *not* a second
   per-record mechanism. v1's adversarial review showed a per-record `SchemaVersion` +
   render-time `UpgradeSpec` would (a) risk an import cycle (`store.migrate`→`domain`), and
   (b) open a window where a stored record renders with stale semantics until first view.
   Records may *carry* a version int for diagnostics/forward-tolerance, but the **upgrade
   happens once, at load, in `migrate`** — the established pattern.
2. **Additive-only changes.** New fields ship with **safe defaults**; new `WidgetKind` /
   `SourceKind` / `Transform` values are new enum members, never reinterpretations of old
   ones. Old saved records keep rendering. (Caveat: a new field whose zero value is
   *invalid* must get its default assigned in `migrate`, not relied upon from JSON — the
   validation pass (#4) catches misses.)
3. **Unknown-kind tolerance.** A placement whose `Kind`/`NativeID` isn't registered renders
   a labelled "unavailable widget" tile instead of crashing (graceful degradation, like
   `cardgraph`'s `Issue` collection). Forward-compatible across app versions.
4. **Validation pass.** `widgetcfg.Validate(schema, settings)` + `Style.Validate()` +
   `Pipeline.Validate()`/`ScalarBind.Validate()` run at write time and before render;
   invalid config surfaces a visible, debuggable error (the schema-driven-UI failure mode
   is silent typos — we make them loud).

## 10. Migration plan & phasing

Round-3 finding: the full feature set (grid + content layout + sizes + spacers + lock +
struct widgets + share + builder + library + custom fields) is too much for one v1. Split
into **v1 (the coherent core)** and **v2 (the heavier additive features)**. Everything is
designed so v2 is *additive* — no v1 rework.

### v1 — the buildable core (big-bang, SDLC order; each step compiles + `go test ./...` green)
1. **Data model** — `domain.WidgetSpec/Placement/ScalarBind/Pipeline/Source/Frame`; `Graph`
   is `json.RawMessage`; `WidgetSpec.Validate()` (binding mutual-exclusivity, §3). Pure, tested.
2. **Services** — `widgetsource` resolvers (incl. correct bills), `Frame` transforms,
   frame→`chartspec` projection, entity-namespaced custom-field extension to `engineenv`.
   Pure, tested.
3. **Persistence** — `placements` (+ optional `widgetlibrary`) tables, global `Style` in
   `Settings`, `Dataset` wiring, CRUD, `migrate` upgrade step, export/import round-trip.
4. **dashlayout parameterization** — `Reconcile(saved, defaults)` (nil = keep-all),
   `Arrange(items, mode, defaults)` (canonical order from defaults; `ModeAutoDefault`
   dashboard-only), `Pack(items, cols)` threaded from `SurfaceCtx.Cols`, `CyclePreset`
   replacing free-span `CycleSpan`/`Resize`. Pure, tested. **Prerequisite for the shell.**
5. **Registry + surface-scoped state** — `widgetregistry` (pure) + `widgetrender` (wasm);
   namespaced per-surface atoms (§7.2.1); port all 22 dashboard widgets (§4 audit) + 6 page
   kinds; surface-level `WidgetDataCache` (§10.1).
6. **Shell (UI last)** — rebuild `ui.Widget(placement, SurfaceCtx)`: one grid for all
   surfaces (§7.2), **Standard** content layout only, standard sizes + spacers (§7.3),
   movement lock (§7.4), struct-driven Table/List/Chart over existing `DataTable`/`EntityRow`/
   `Chart` (§12). Route dashboard, custom pages, Widget Manager, Builder through it.
7. **Delete old paths** — `dashboard.go` closure map, `customTile`, `widgetManagerTitleKeys`,
   `ui/widget.go` route+icon maps, `appkv` layout blobs, `widgetstyle` `_`-key piggyback.

### v2 — additive, each its own sub-spec before code
- **Custom `ContentLayout`** (§7.5 `Mode==Custom`) — the flex/auto-flow block renderer + the
  `{{var}}` templater (scanner, HTML-escaping, validation). Standard mode ships in v1; Custom
  is the design-flexibility tier and needs its own spec.
- **Share / snapshot** (§7.6) — the new `cashfluxCaptureWidget` JS shim (font preflight,
  animation settle, SVG inlining) + `data-cf-redact` rollout across the 16 Native renderers
  and the chart shim. Non-trivial new JS + a privacy surface → its own spec + security review.
- **`LibLink` live-linked specs** (§8) — the field exists in v1 (designed-for); the
  edit-once-change-everywhere behavior is built in v2.
- **Page-aware resolvers** (§12.3) and **alt custom-field aggregations** (§5.2).

### 10.1 Memoization (don't recompute N×M)
Each placement resolving its own Source on every render would recompute shared inputs
(all KPIs read the same accounts/transactions) once per tile per data change. Mirror the
current `state.UseComputed`/`app.Rev()` pattern (`screens/selectors.go`): a **surface-level
`WidgetDataCache` atom** maps `Source → Frame`, keyed on **`(app.Rev(), filterContext)`** —
the filter context (period + scope, §13.1) is part of the key from v1 so that cross-filtering
(v2) is a cache *input*, not a cache rewrite. Placements look it up. Placement-only writes
(style/settings edits) must **not** bump the data revision, or every tile re-resolves — route
them through a separate UI-state path, not `mutationRev`.

### Risks
- **R1 — Native data richness.** Some dashboard tiles need data beyond `widgetsource`.
  Mitigation: they're `Native`; their Go renderer computes freely. Don't formula-ify them.
- **R2 — Fresh-start = user-visible reset.** "SQLite fresh start" wipes existing dashboard
  arrangement/accents (real `appkv` data; app is shipped). A one-time read-convert of the
  layout blob into seeded placements is cheap and avoids it. **Recommend reconsidering.**
- **R3 — Big-bang vs. concurrent agents.** This repo has multiple agents + stale worktrees
  touching the exact widget files. Big-bang is the highest-collision strategy. Mitigation:
  land on a single branch, sequence the v1 steps, never revert others' WIP.
- **R5 — Scope.** Three rounds of additions risked a v1 too large to land. Mitigation: the
  v1/v2 split (§10) — v1 is the coherent core; Custom content layout and Share are v2 with
  their own sub-specs. Everything v2 is additive (no v1 rework).
- **R4 — Two sources of truth during cutover.** The live layout atom vs. the new
  placements table. Mitigation: step 4 moves the atom's seed onto placements in one change;
  no dual-write window.

## 11. Reconciliation with the Widget Builder

> **Revised after adversarial review.** v1 claimed the node graph "compiles to a
> Pipeline." That is **false**: `cardgraph` is a typed *DAG* (multiple source nodes,
> `logic.compare`/`logic.branch`, `viz.stack`, `style.*` nodes; row-keyed `Collection`,
> `PortType` incl. `Color`/`Viz`). Our `Pipeline` is a *linear* chain with a different,
> column-keyed `Frame`. There is no mechanical compilation between them — pretending so
> would break the shipped Widget Builder.

Instead, the builder is a **first-class `Kind == Builder`**: `WidgetSpec.Graph` holds the
serialized graph as **`json.RawMessage`**, *not* a `*cardgraph.Graph`. This is deliberate
(round-2 finding): putting a `cardgraph` type in `domain` would invert the build-order
dependency (domain is the base layer; cardgraph is a feature evaluator that may itself need
`domain.Transaction` for source nodes → latent import cycle). With raw JSON, `domain` stays
dependency-free and the spec serializes cleanly; the **wasm builder layer** unmarshals the
bytes to `cardgraph.Graph`, evaluates with the builder's own evaluator, and draws via a
renderer registered in `widgetrender` (a Native-class body). The node graph stays its own
dataflow language; we do **not** force it into the Pipeline IR.

What unification *does* deliver for the builder — and this is the real win:
- builder cards become persisted `WidgetSpec`s in SQLite (today they're unpersisted), so
  they survive `wipe` and travel with export;
- they place on **any** surface through the one shell, with the same drag/style/gear/hide
  chrome as every other widget;
- they appear in the same `Catalog()` picker.

The builder stops being a separate, unpersisted island — by sharing the *envelope*
(`WidgetSpec` + `Placement` + shell), not by collapsing its *language* into the Pipeline.

## 12. Standard struct-driven widgets (Table, pagination, primitives)

> **Goal (owner request):** "send a struct to a widget and it renders." This is exactly
> what the `Frame` shape is for — and the good news from the codebase audit is that the
> *rendering* primitives already exist and are reusable. Standard widgets are therefore a
> thin **`Frame` → existing-component adapter** layer, not new UI.

### 12.1 The struct you send *is* the Frame
A DataDriven widget's body is a pure function `(Frame, Settings) → ui.Node`. The `Frame`
(typed columns + rows, §3.1) is the "struct" the owner wants to send. A handful of
registered **standard renderers** map a `Frame` onto components that already ship:

| Standard widget | Frame → component | Reuses (file) |
|---|---|---|
| **Table** (sortable, paginated) | `Frame` columns → `ui.Column`s; rows → `<tr>` body | `ui.DataTable` (`internal/ui/datatable.go`) — already has `aria-sort` headers + `dtPager` |
| **Paginated list** | `Frame` rows → `EntityRow`s | `ui.EntityListSection` + `ui.EntityRow` (`internal/ui/primitives.go`) |
| **Compact list** | `Frame` → `widgetdata.Row{Label,Value}` | `listBody` pattern (`custompage.go:498`) |
| **Stat grid / KPI row** | `Frame` numeric fields → `Stat`s | `ui.StatGrid` (`internal/ui/primitives.go`) |
| **Chart** | `Frame` → `chartspec.Spec` → D3 | `ui.Chart` (`internal/ui/chartd3.go`) |
| **Meter / progress** | `Frame` ratio field → bar | `ui.MeterBar` / `ui.ProgressBar` |

Adding a new standard widget = one `Frame→ui.Node` adapter registered in `widgetrender`
(+ its `Descriptor` in `widgetregistry`). No new chrome, no new persistence — it inherits
all of it from the shell.

### 12.2 Table: column config in Settings, not code
The `Table` widget derives its `ui.Column`s from the `Frame.Fields` plus a `Settings`
overlay (visible columns, order, label override, alignment, `SortKey`). Because
`ui.DataTable` already owns sort carets and the pager, the Table widget is essentially:
```
cols    := columnsFromFrame(frame, settings)
body    := rowsFromFrame(frame, settings)          // each row a component (no On* in loop)
return ui.DataTable(ui.DataTableProps{Columns: cols, Body: body,
    Sort: settings.Sort, Dir: settings.Dir, OnSort: setSort,
    Page: settings.Page, Total: frame.Rows, PageSize: settings.PageSize,
    PageSizes: pagination.DefaultSizes, OnPage: setPage, OnPageSize: setPageSize})
```

### 12.3 Pagination: reuse the pure package, two modes
`internal/pagination` (`TotalPages/Clamp/Bounds/Slice/Window`) is the engine. Two modes,
chosen per widget in `Settings`:
- **Render-time paging (default):** the full `Frame` is in memory (local-first); the Table
  renderer slices with `pagination.Slice(rows, page, size)` and `ui.DataTable`'s `dtPager`
  draws "1–50 of 312" via `pagination.Window`. Zero new logic.
- **Overflow scroll (owner request):** for inner lists/tables that exceed the tile's fixed
  height (standard sizes, §7.3), `Settings.Overflow` chooses how content beyond the bounds is
  handled — `Clip` (truncate, the dashboard default), `Scroll` (the body becomes an
  `overflow:auto` scroll region with the header pinned), or `Paginate` (the `dtPager` path
  above). `Scroll` is the right mode for a tall list inside a small tile when the author wants
  all rows reachable without growing the tile. It applies to any DataView/List/Table body and
  is purely a CSS/`Style` concern — no new logic. (Spacer/KPI ignore it.)
- **Source-limited paging:** for genuinely large collections, the **resolver itself must be
  page-aware** — a `paginate` Transform late in a linear chain can't save memory, because the
  source already materialized every row before the transform runs (round-2 finding). True
  paging means the resolver fetches only `rows[from:to]` (SQL `LIMIT/OFFSET`, or slice bounds
  for in-memory sources) — a different resolver implementation, not just a transform. v1 ships
  render-time paging only; page-aware resolvers are added per-source if/when a collection
  outgrows memory (transactions is the likely first). (`PageSizeAll = -1` carries over.)

`PageSize`/`Sort`/`Dir`/`Page` are **per-placement `Settings`**, so two placements of the
same Table spec page and sort independently — the per-placement override model (§3, §8)
makes this fall out for free.

### 12.4 Sorting & filtering for free
`ui.DataTable` sort headers are built in; the Table widget maps `OnSort(key)` → a
`Settings.Sort/Dir` write. For filtering, the screen-agnostic `ui.FilterToolbar`
(`internal/ui/filtertoolbar.go`, today wired only to transactions) becomes an optional
Table header: filter predicates are `Frame` `filter` Transforms, surfaced as chips. This
generalizes the transactions toolbar to *any* Frame — a concrete second consumer that also
validates the abstraction.

### 12.5 Why this is low-risk
Every standard widget reuses a **tested, shipping** component (`DataTable`, `EntityRow`,
`StatGrid`, `Chart`, `pagination`). The only new code is pure `Frame`-shaping adapters and
the column/sort `Settings` schemas — both native-testable. Interactivity rule (§7.1): a row
only needs to be its own component **when it has `On*` handlers**. Read-only Frame rows are
plain inline `Tr(cells…)` nodes (as `cpTableBody` already does) — no wasted component
boundary; the per-row component is reserved for rows with actions.

## 13. Enterprise capabilities

A grounded audit of CashFlux shows most enterprise *infrastructure* already exists — it's
just not wired into a widget contract. This section adds the widget-level APIs. Foundational
data-model fields (`Placement.Access`, filter-aware cache key) ship in **v1 so later phases
are additive, not migrations**; the heavier behavior is phased.

| Concern | Already in CashFlux | What the unified widget API adds | Phase |
|---|---|---|---|
| Shared filters | `uistate.UsePeriod` + `UseActiveScope` (global atoms) | a per-surface **`FilterContext`** every widget binds to (§13.1) | v1 (bind) |
| Cross-widget filtering / drill-down | only screen-nav drill-down | widgets **emit** filter events → surface `FilterContext` → siblings re-resolve (§13.1) | v2 |
| Access control | `memberrole` Owner/Admin/Viewer model | per-placement **`Access`** (view/edit roles, member-scoping) enforced in the shell (§13.2) | v1 (field) / v2 (enforce) |
| Lifecycle + resilience | `Skeleton`, `freshness`, ad-hoc empties | a **widget lifecycle contract** + per-widget **error boundary** in the shell (§13.3) | v1 |
| Thresholds / conditional format | system thresholds (budget 80%, attention severity) | per-widget user **`Rules`** → data-driven `Style` (§13.4) | v2 |
| Programmatic / AI authoring | AI read-tools (`aitools`), no write | a pure **`widgetauthor`** API to build/place specs → an AI write-tool (§13.5) | v2 |
| Portability / templates | pages travel only with full dataset export | **single placement / whole-surface** portable versioned JSON ↔ `widgetlibrary` (§13.6) | v2 |
| Locale formatting | `i18n` infra, currency symbol/decimals | format through a **locale `FormatContext`** (separators, dates) (§13.7) | v2 |
| Large data | pagination, overflow-scroll | **virtualized** Table/List window for big Frames (§13.8) | v2/v3 |
| Observability | `auditlog`, server audit, OTel, slog | widget render errors + usage feed the existing `auditlog`; no new system (§13.3) | v1 |

### 13.1 Filter context (shared + cross-widget)
A surface owns a `FilterContext` (initially the existing `period` + `scope` atoms). v1:
every DataDriven widget resolves its `Source` *through* the context, so changing the
surface period/scope re-resolves all widgets (already true for the dashboard — now uniform).
v2: a widget can **emit** a filter (click a chart bar / a category row) that writes the
surface `FilterContext`; siblings re-resolve via the filter-aware cache key (§10.1). Native
widgets opt in by reading the context. This is the single biggest enterprise-dashboard
feature and the cache was keyed for it from day one.

### 13.2 Access control (per-placement ACL)
`Placement.Access` (in the type model) gates view/edit against the `memberrole` hierarchy
and optional member-scoping. The **shell** is the one enforcement point: a viewer sees no
gear/drag and hidden-to-role placements don't render; `OnlyMembers` narrows a widget's data
scope to specific members (a "just my budget" tile). Empty `Access` inherits the surface.
Field ships v1; enforcement v2.

### 13.3 Lifecycle contract + per-widget error boundary (resilience)
The shell wraps **every** widget body so one widget can't take down the surface — a panic
or resolver error renders a contained **error tile** (the §9 "unavailable" pattern) and is
logged to the existing `auditlog`/slog. Each widget exposes a uniform lifecycle:
**Loading** (`ui.Skeleton`), **Empty** (existing empty-state), **Error** (contained tile),
**Stale** (a "last updated" badge driven by `freshness`/data rev), **Ready**. Today these
are ad-hoc in two screens; this makes them a contract for all widgets. v1 — it's robustness,
and cheap given the primitives exist.

### 13.4 Conditional formatting / thresholds
Per-widget `Settings.Rules`: a small list of `{when: formula, then: styleDelta}` evaluated
against the widget's value/Frame (reusing `formula.Eval`), e.g. *value > budget → accent
red*. Drives `Style` from data — the data-driven analog of today's static per-tile color.
Reuses the `attention` severity vocabulary. v2.

### 13.5 Programmatic + AI authoring
A pure `internal/widgetauthor` package: `NewKPI/NewTable/NewChart(...) WidgetSpec`,
`Place(spec, surface, size) Placement`, `Validate`. The Widget Builder and pickers use it;
crucially it gives the **AI agent a write-tool** (`create_widget`/`build_dashboard`) to
complement today's read-tools (`aitools`) — "make me a dashboard for groceries" becomes real.
Pure and testable; no UI. v2.

### 13.6 Portability / templates
Export/import a **single placement** or a **whole surface** as a portable, `SchemaVersion`-
stamped JSON artifact, independent of the full dataset — the unit of sharing for the
`widgetlibrary` (§8) and a future template gallery. Built on the same `UpgradeSpec` path so
an imported older template upcasts cleanly. v2.

### 13.7 Locale-aware formatting
A `FormatContext{Locale, Currency}` threaded into the Frame/Scalar formatters so money/date/
number follow locale conventions (separators, date order), not just currency symbol/decimals.
The `i18n` infra and `currency` table already exist; this wires widgets to them. v2.

### 13.8 Virtualization
For Frames beyond a row threshold, the Table/List renderer switches to a **windowed**
(virtualized) body — reusing the existing `IntersectionObserver` seam (`app/pageenter.go`) —
so a 5,000-row source stays smooth. Pagination/overflow-scroll (§12) cover the common case;
virtualization is the large-data escape hatch. v2/v3.

## 14. Open questions (resolve before/while building)
1. **Custom-field aggregation** beyond `sum` (§5.2) — needed for v1, or deferred?
2. **Template library UI** (§8) — ship the `widgetlibrary` table in v1, or specs-per-
   placement only at first?
3. **R2** — fresh start vs. read-convert the existing layout. (Owner call.)
4. **Native-widget settings** — do code-backed widgets get full schema-driven config, or a
   curated subset?
5. **Declarative `Effects`** (§7.1) — keep interactivity Native-only, or design a
   trigger→action layer so DataDriven widgets can have row actions? (Deferred past v1.)
6. **Enterprise v1 line** — confirm the v1-vs-v2 split in §13 (filter-bind, `Access` field,
   lifecycle/error-boundary in v1; the rest v2). Pull anything forward?

## Appendix A — current → unified mapping

| Today | Unified |
|---|---|
| `dashboard.go` renderer closures (22) | `WidgetSpec` (KPIs DataDriven) / `Native` descriptors |
| `domain.PageWidget` | `Placement` (Surface `page:<slug>`) |
| Widget Builder card (localStorage) | `WidgetSpec` (persisted) authored by the node graph |
| `widgetspec.Type*` consts | `domain.WidgetKind` |
| `widgetcfg.Config` + `Schema` | `Settings` + `Descriptor.Schema` (validated) |
| `widgetstyle` `_`-keys on config map | typed `Style` sibling |
| `widgetvis.Set` + Widget Manager | `Placement.Hidden` + manager over placements |
| `widgetdata.ListRows` | `widgetsource` resolvers → `Frame` |
| `dashboard.go` inline compute | `Pipeline` (DataDriven) or `Native` renderer |
| `appkv:"cashflux:layout"` blob | `placements` rows (Surface `dashboard`) |
| `customTile` ad-hoc page layout | same `dashlayout.Pack` grid as dashboard (§7.2) |
| hand-rolled tables (`cpTableBody`, etc.) | `Table` standard widget over `ui.DataTable` (§12) |
| arbitrary col/row spans | `StandardSizes` presets per `Descriptor` (§7.3) |
| (none) | `Kind==Spacer` grid filler (§7.3); `SurfaceCtx` movement lock (§7.4) |
| fixed per-tile markup | `ContentLayout` Standard/Custom intra-tile engine (§7.5) |
| (none) | shareable widget snapshot w/ redaction (§7.6) |
| `widgetstyle` raw hex per widget | token-first `Style` bound to `internal/theme` engine (§7.7) |
| (none) | per-placement `Access` ACL; surface `FilterContext`; widget error boundary (§13) |

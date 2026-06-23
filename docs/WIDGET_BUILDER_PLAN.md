# Widget Builder v2 — Canvas-first node dataflow (the real plan)

> Supersedes the phased UI in WIDGET_BUILDER_DESIGN.md. The pure `internal/cardgraph`
> engine stays and is extended; the UI is rebuilt so the **canvas is the product**.

## 0. The thesis (what we're actually building)

A **node-based visual program** whose output is a dashboard widget. The canvas is the
whole interface. You **generate data from the app**, pipe it through **transform/logic
nodes**, shape its **design with nodes**, and terminate in a **display node** that *is*
the rendered widget. Formulas and design are both edited as part of the graph — not in a
form bolted next to it.

```
[ app data ] → [ transform / logic ] → [ design ] → [ display widget ]  =  the card
   sources         filter/agg/formula     color/format    kpi/list/chart/…
```

Three hard principles:
1. **The canvas is primary.** Pan/zoom surface, a node palette, drag-to-wire. Everything
   else (preview, inspector, toolbar) docks around it.
2. **Everything is a node, every wire is typed.** Data, math, logic, *and design* are
   nodes. The widget is just the graph's output node rendered.
3. **Pure engine, reactive eval.** All behavior lives in `cardgraph` (native-testable);
   the graph re-evaluates against live app data and renders.

---

## 1. The type system (extended lattice)

Current: Number, Text, Bool, Viz. Add the types real widgets need:

| Type | Carries | Produced by | Consumed by |
|---|---|---|---|
| **Number / Money** | scalar | figure, aggregate, formula, literal | kpi, progress, formula, compare |
| **Text** | string | literal, field, format | text, badge, list label |
| **Bool** | true/false | compare, literal, logic | branch, filter predicate |
| **Date** | instant | literal, period bounds | filter, series bucket |
| **Color** | css color | palette, colorScale, threshold | any display node's `accent` port |
| **Collection** | rows × typed columns | dataset (txns/accounts/…) | filter, sort, aggregate, list, table |
| **Series** | ordered (label→number) | aggregate(group-by), bucketByTime | chart.line/bar/donut, sparkline |
| **Enum** | a fixed choice (e.g. format) | literal/select node | display format/tone ports |
| **Viz** | a renderable block | display nodes, stack | the card Root |
| **Event / Action** | interactivity (Phase F) | button/toggle | action sink → workflow Effect |

Coercions stay minimal & safe (Bool→Number, Number/Bool→Text, Number→Color only via an
explicit colorScale — never implicit). Incompatible wiring is refused with a reason.

**Design-as-dataflow:** display nodes expose **style input ports** (`accent: Color`,
`format: Enum`, `tone: Enum`, …) with sane defaults when unwired. You customize design by
wiring a **design node** into a style port — e.g. `threshold → colorScale → kpi.accent`.
That's "customization of designs via widgets."

---

## 2. Node catalog (the language)

Grouped as the palette will be. ⬛ = already built, 🔲 = to build.

### Data sources — "generate data from the app"
- ⬛ `source.figure` — a named engine scalar (net worth, income, savings rate, counts…).
- 🔲 `source.dataset` — a **Collection** of `transactions | accounts | budgets | goals |
  tasks | bills`, with typed columns (amount, date, payee, category, account, …).
- 🔲 `source.period` — a **Date** range (this month / last N days / quarter / custom),
  honoring week-start; feeds filters and time-bucketing.
- ⬛ `literal.number | text | bool` 🔲 `literal.date | color`.

### Transforms — reshape data
- 🔲 `txn.filter` — Collection→Collection by a predicate (field compare, amount range,
  keyword, or a **rule** node / formula). Reuses `internal/rules`.
- 🔲 `agg` — Collection→Number (sum/avg/count/min/max of a column); with **group-by** →
  **Series**.
- 🔲 `sort` + `limit` — Collection→Collection ("top 5 expenses").
- 🔲 `bucketByTime` — Collection + Period → **Series** (trend lines).
- ⬛ `formula` — Number(s)→Number, sandboxed (`internal/formula`), inputs as variables.
- 🔲 `field` — Collection→Series/Text (pluck a column).

### Logic
- ⬛ `logic.compare` (→Bool) ⬛ `logic.branch` 🔲 `logic.and/or/not`
- 🔲 `threshold` — Number + cutoffs → Enum tone (good/warn/bad).

### Design — "customization of designs via widgets"
- 🔲 `design.colorScale` — Number (+domain) → **Color**.
- 🔲 `design.threshold` — Number + cutoffs → tone Enum (drives color/badge).
- 🔲 `design.format` — Enum literal (number/percent/currency/compact).
- 🔲 `design.palette` — pick a theme/brand Color.

### Display widgets — the card (sinks → Viz)
- ⬛ `viz.kpi` ⬛ `viz.text` ⬛ `viz.progress` ⬛ `viz.badge`
- 🔲 `viz.stat` (value + previous → Δ% with arrow) 🔲 `viz.gauge`
- 🔲 `viz.list` (Collection + columns) 🔲 `viz.table` (sortable)
- 🔲 `viz.chart.line | bar | donut` (Series) 🔲 `viz.sparkline`
- 🔲 `viz.stack` / `viz.grid` — compose 2–4 child Viz into one rich tile (this is the Root
  for multi-block cards, e.g. a KPI header above a list — like the real dashboard tiles).

### Interactivity (Phase F)
- 🔲 `ui.button | ui.toggle | ui.rowAction` → **Event** → `act.*` (createTask, toggleTask,
  setCategory…) applied via `appstate`, reusing `internal/workflow` Effects. Target: the
  To-do tile's inline-complete, rebuilt as a graph.

---

## 3. The interface (canvas-first)

```
┌───────────────────────────────────────────────────────────────┐
│ toolbar:  name │ tile size W×H │ Undo/Redo │ Save │ Export ▼   │
├──────────┬────────────────────────────────────────┬───────────┤
│ PALETTE  │              C A N V A S                │ INSPECTOR │
│ (search) │   pan / zoom / grid                     │ selected  │
│ ▸ Data   │   ┌────┐      ┌─────────┐   ┌────────┐   │ node's    │
│ ▸ Transf │   │txns│─►────│ filter  │─►─│ agg sum│─► │ params:   │
│ ▸ Logic  │   └────┘      └─────────┘   └────────┘ \ │ dropdowns │
│ ▸ Design │                                          ►│ formula  │
│ ▸ Display│                              ┌──────────┐ │ rule b.  │
│          │                              │ KPI (out)│ │ color pk │
├──────────┴──────────────────────────────┴──────────┴───────────┤
│ PREVIEW (docked): the Output node rendered at the chosen size   │
└─────────────────────────────────────────────────────────────────┘
```

- **Palette** (left): searchable, categorized node library. Click/drag to add.
- **Canvas** (center, primary): pan (drag bg), zoom (wheel + buttons), snap grid.
  Nodes are titled boxes with **typed ports** (inputs left, output right). Drag from an
  output port to a compatible input → wire (bezier). Type-checked; incompatible refused
  with a tooltip. Delete wire/node. One node is the **Output** (★) = the card.
- **Inspector** (right): the selected node's full parameters with real controls —
  dropdowns, number/text inputs, **formula editor with variable autocomplete**, **rule
  builder**, **color picker**, column chooser. This is where formulas & designs are
  customized. (A couple of "primary" params may also edit inline on the node.)
- **Preview** (docked bottom or as a pinned node): live render of the Output node's Viz at
  the tile size; reactive to data + graph edits; shows a friendly unfinished/error state.
- **Toolbar**: name, tile size (1–4 × 1–3), undo/redo, **Save/Publish to dashboard**,
  **Export/Import JSON**.

Accessibility: nodes keyboard-focusable; arrow-move; ports reachable; inspector is the
keyboard path for wiring-averse users (every input port has an "input source" dropdown as
an alternative to dragging a wire).

---

## 4. Engine & persistence

- **Eval**: `cardgraph` already does typed DAG eval with cycle detection and graceful
  degradation. Extend with the new value types (Collection/Series/Color/Date/Enum) and
  the new node specs. Memoize per node; recompute on the data-revision atom (reactive).
- **Data access**: a `cardgraph.Context` carrying the engine var surface **plus dataset
  accessors** (collections built from `appstate` by the wasm layer and passed in, keeping
  the core pure/testable).
- **Persistence**: serialize `Graph` (+ node positions + tile size + name) as a new
  `domain.CardSpec`; save through `appstate` → `internal/store` with JSON/CSV round-trip
  tests. **Publish** registers a dashboard layout item rendered by a thin `cardgraph`→
  `ui.Node` renderer, so the Widget Manager governs it like any tile.

---

## 5. Build phases (each: pure logic + tests → canvas UI → e2e)

- **Phase A — Canvas foundation (replaces today's fixed 3-box UI).** Pan/zoom canvas,
  searchable palette, add/move/delete nodes, **drag-to-wire with type checks**, selection
  + **Inspector**, docked live **Preview**, the **Output node** concept, undo/redo. Ships
  with the *existing* node kinds so it's immediately usable. This is the big one.
- **Phase B — Data generation.** `source.dataset` (+ Collection type), `source.period`,
  `agg`, `txn.filter`, `sort`, `limit`. Now cards can do "top 5 expenses this month".
- **Phase C — Display widgets.** list, table, chart.line/bar/donut, stat, gauge,
  sparkline, and `stack`/`grid` compose. Series type + a small chart renderer (reuse the
  existing `chartspec` + D3 shim).
- **Phase D — Design nodes.** colorScale, threshold, format, palette → wired into display
  style ports. Live restyle.
- **Phase E — Save / publish.** `domain.CardSpec`, persistence, JSON export/import, publish
  to dashboard + Widget Manager integration.
- **Phase F — Interactivity.** button/toggle/rowAction → Event → workflow Effect via
  `appstate`. Rebuild the To-do tile as a graph to prove it.

---

## 6. Migration from what exists

The current `widget_builder.go` (fixed source→transform→viz boxes + config cards) is
replaced by the canvas in Phase A. Kept & reused: the whole `internal/cardgraph` engine
(node kinds, eval, types), the drag/preview plumbing patterns, the inline-styling +
Go-injected-shim approach (revert-proof, since a parallel effort keeps reverting
`widgets.go`/`index.html`). New code lives in my own files (`widget_builder.go` +
`cardgraph/`), route stays `View: VisualBuilder`.

---

## 7b. Decisions LOCKED (Cam, 2026-06-23)

- **Phase A scope = canvas + one full vertical.** First delivery: the canvas (pan/zoom,
  palette, drag-to-wire, inspector, live preview) PLUS a complete pipeline wired through
  `dataset → filter → aggregate → chart`, to validate the whole dataflow end-to-end.
  Then fill in breadth.
- **Display breadth = everything.** line/area chart, list, table, stat-with-delta,
  bar/donut/gauge, **plus interactive buttons, text inputs, all useful primitives**, and
  **events + fonts/design** nodes. Build comprehensive; nothing's a throwaway.
- Inspector = right dock + a few inline params; wiring = drag-to-wire + per-port "input
  source" dropdown (keyboard path). Both, as recommended.

## 7. Decisions for Cam (the forks that change the build)

1. **Inspector vs all-inline.** Recommend a **right-dock Inspector** for full params +
   a couple of primary params inline on the node. (Pure-inline gets cramped for formulas/
   rules/colors.)
2. **Wiring UX.** Drag-to-wire on ports **and** an "input source" dropdown per input port
   in the Inspector (keyboard/no-drag path). Recommend both.
3. **First display widgets to nail** (Phase C order). Recommend: **chart.line, list, stat**
   first (highest "wow" + cover the dashboard's real tiles), then table/donut/gauge.
4. **Scope of Phase A demo.** Recommend Phase A ships free-form wiring with the *current*
   node set (figure/literal/formula/logic/kpi/text/progress/badge) so you can see the real
   canvas immediately, before B/C broaden data + display.

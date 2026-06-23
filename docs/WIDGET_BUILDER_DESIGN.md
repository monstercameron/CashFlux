# Widget Builder — Visual Programming System (design)

> **Status:** design / spec-in-progress (no feature code beyond the existing stage +
> pipeline scaffold in `internal/screens/widgets.go`). Per the project process rule,
> we agree the spec before building. This doc is that spec.
>
> **Author:** Claude (taking over per `/goal`, 2026-06-23, overnight).
> **Owner to review:** Cam.

## 1. The vision (restated)

Build a **visual scripting system, n8n-style**, inside CashFlux's Widget Builder. The
user wires a graph of **nodes** on a canvas. Nodes hold and produce different things —
literal **values**, **formulas**, **rules**, dataset queries, transforms, logic — and the
graph evaluates to a **full-fledged dashboard card**. The card can carry **basic
interactivity** (like the dashboard To-do tile's inline "complete" checkbox).

It should feel *almost like its own programming language* — flexible, composable —
but the **visual medium is the constraint** that keeps it safe and finite (a total,
terminating, no-arbitrary-code dataflow language, not a Turing tarpit).

This reuses CashFlux's existing pure engines rather than inventing new ones:

| Engine | Package | Role in the builder |
|---|---|---|
| Expression sandbox | `internal/formula` | the **Formula node** (math/logic over inputs) |
| Auto-categorization | `internal/rules` | the **Rule node** (match/filter/categorize a collection) |
| Automation (trigger→cond→actions) | `internal/workflow` | the **interactivity plane** (events → Effects) |
| Variable surface | `internal/engineenv` | scalar **Source nodes** (`net_worth`, `income`, counts…) |
| Widget catalog + KPI eval | `internal/widgetspec` | the **Visualize nodes** (KPI/List/Chart/Text) |

---

## 2. Refining questions (10) — with my interim answers

Each answer is a default I'm building toward; flag any you disagree with.

**Q1. Card shape — one output, or a layout of several?**
A *card* is a single bento tile. Does it render exactly one visualization (one KPI, or
one list), or can it stack several blocks (a KPI header *and* a list beneath, like the
real dashboard tiles)?
→ **My call:** A card has **one root Output node**, but there's a **Layout/Stack
container node** that arranges 2–4 child viz blocks vertically. v1 ships single-block
cards; the Stack node lands in phase 3 so multi-block (KPI + list) is designed-for, not
bolted-on. This matches how `kpi-*`, `recent`, `todo` tiles already compose.

**Q2. Where do built cards live, and do they replace custom pages?**
→ **My call:** A built card becomes a **first-class dashboard widget** (an entry in the
layout-items atom with a stable id), sitting alongside built-ins in the Widget Manager
(show/hide/resize/reorder/style all work for free). Storage is a new
`domain.CardSpec` (the serialized graph) persisted via the `appstate` seam + SQLite +
JSON/CSV round-trip — modeled on the existing `domain.PageWidget` path. Custom pages
(`custompage.go`/`widgetspec`) stay; the builder is the richer superset and may later
generate `PageWidget`s too.

**Q3. Typed ports or untyped "anything goes"?**
→ **My call:** **Strongly typed ports** with a small type lattice and explicit
coercions. This *is* the "programming language" backbone, and typing is exactly the
constraint that keeps visual wiring from producing nonsense. Types (§4). Incompatible
ports refuse to connect (with a tooltip explaining why); safe coercions (bool→number,
number→string for display) are automatic.

**Q4. How far does interactivity go in v1?**
→ **My call:** **Bounded to the existing workflow Effect set.** Interaction nodes
(Button, Checkbox/Toggle, Row-action) fire **Events** that map to `workflow.Action`s
(create task, toggle task done, set category, add tag, notify, post recurring…). No
arbitrary mutation — interactivity reuses `workflow.Plan → Effect → apply` so it's
dry-runnable and explainable. The To-do tile's inline complete is the canonical target.

**Q5. Reactive (live) or manual "Run"?**
→ **My call:** **Reactive dataflow.** The data/render plane is pure and recomputes
whenever the shared data-revision atom bumps (same mechanism the dashboard uses). No
"run" button for rendering. The *action* plane fires only on user events. This keeps
cards always-fresh and the language side-effect-free except at explicit Action sinks.

**Q6. Loops / cycles — allowed?**
→ **My call:** **No cycles in the data plane — it's a DAG** (topological eval + cycle
detection; a cycle is a graph error shown inline). Iteration is *data-parallel*, via
collection nodes (Filter/Map/GroupBy/Aggregate/Sort/Limit), never explicit `while`.
This guarantees every card terminates — the central safety property.

**Q7. Formula node ↔ existing `formula` engine?**
→ **My call:** **Reuse `internal/formula` verbatim.** A Formula node names its input
ports as variables and evaluates the expression via `formula.Eval(expr, Env{Vars,Strs})`.
Same sandbox, same functions (`sum/avg/min/max/if/contains/…`), zero new eval surface.

**Q8. Rule node behavior?**
→ **My call:** A **Rule node** wraps `rules.Condition` (all/any keywords, account scope,
amount range) as a **collection filter**, and a second mode wraps `rules.Category/Tags`
to **derive** a category/tag column. Existing saved rules are selectable as presets.

**Q9. Half-built / broken graphs — what does the user see?**
→ **My call:** **Never crash; degrade gracefully.** Every node carries a status
(`ok` / `needs-input` / `error`) with an inline chip. The stage renders a best-effort
preview (resolved subgraph) plus a friendly "this card isn't finished" state when the
root can't evaluate. Errors are plain-English (reusing the engines' error strings).

**Q10. Reusable subgraphs ("functions")?**
→ **My call:** **Not in v1, but architected for.** The graph model supports a future
"group these nodes into a reusable Component with typed in/out ports" — the language's
function abstraction. v1 keeps a flat graph; the node/edge model leaves room (a Component
is just a node whose body is another graph).

---

## 3. The 20 user stories

Format: *As a CashFlux user, I want … so that …* + key acceptance criteria (AC).
Grouped by capability and roughly ordered by build phase.

### A. Canvas & graph fundamentals

**US-1 — Place nodes on a canvas.**
I want to add nodes from a palette onto an infinite, pannable/zoomable canvas, so that I
can lay out my card's logic visually.
*AC:* palette grouped by category; drag or click-to-add; canvas pans (drag bg) and zooms;
node positions persist.

**US-2 — Connect nodes with wires.**
I want to drag from a node's output port to another node's input port, so that data flows
between steps.
*AC:* wires snap to ports; a wire shows the value type; an input accepts one wire, an
output fans out to many; clicking a wire deletes it.

**US-3 — Type-safe connections.**
I want incompatible ports to refuse connection with an explanation, so that I can't build
a nonsensical card.
*AC:* a Collection output won't connect to a Scalar input; rejected attempts show a
tooltip ("expects a number, got a list"); safe coercions connect silently.

**US-4 — Live preview stage.**
I want the card preview above the canvas to update as I wire nodes, so that I see the
result immediately.
*AC:* preview re-renders on any graph change and on dataset changes; shows the card at its
true bento size; shows an "unfinished" state until the root Output resolves.

**US-5 — Cycle & error guards.**
I want the builder to catch loops and broken nodes, so that a card never hangs or crashes.
*AC:* creating a cycle is blocked with a message; nodes show ok/needs-input/error chips;
the rest of the graph still previews around a broken node.

### B. Source nodes (the "values")

**US-6 — Literal value node.**
I want nodes that hold a fixed number, money amount, text, or date, so that I can feed
constants into formulas and labels.
*AC:* one node per literal type; money carries a currency; values are editable inline.

**US-7 — Scalar figure source.**
I want to pull a live figure (net worth, income this month, account count, …) from my
data, so that my card reflects reality.
*AC:* exposes the `engineenv` variable surface; updates when data changes; labelled with
its plain-English name + current value.

**US-8 — Dataset source.**
I want to start from a collection (transactions, accounts, budgets, goals, tasks, bills),
so that I can build lists, tables, and aggregates.
*AC:* one node per `widgetspec` list source; outputs a typed Collection of rows; shows row
count.

**US-9 — Period / date-range node.**
I want to scope data to a period (this month, last 30 days, custom), so that figures and
lists are time-bounded.
*AC:* reuses `dateutil` period ranges; feeds Filter/Aggregate nodes; respects the user's
week-start preference.

### C. Transform & logic nodes (the "language")

**US-10 — Filter node.**
I want to keep only rows matching a condition, so that my card shows the relevant subset.
*AC:* predicate built from a Rule node, a Formula, or field comparisons; outputs a smaller
Collection; shows "N of M kept".

**US-11 — Aggregate node.**
I want to reduce a collection to a number (sum/avg/count/min/max), optionally grouped by a
field, so that I can compute KPIs and chart series.
*AC:* reuses formula aggregation semantics; grouped mode outputs a Series; ungrouped
outputs a Scalar.

**US-12 — Formula node.**
I want a node that computes an expression over its named inputs, so that I can do custom
math and logic.
*AC:* inputs become variables; evaluated by `internal/formula`; supports the full function
set; errors show the engine's plain message.

**US-13 — Rule node.**
I want to match/categorize transactions using my rules, so that the card reflects how I
file money.
*AC:* wraps `rules.Condition`; selectable from saved rules; can filter or derive a
category/tag column.

**US-14 — Sort & limit node.**
I want to order a collection and take the top N, so that I can show "biggest 5 expenses".
*AC:* sort by any field asc/desc; limit with a count; stable ordering.

**US-15 — Branch / if node.**
I want conditional output (show X when a condition holds, else Y), so that a card adapts
to state (e.g. a "you're over budget" warning).
*AC:* boolean input selects between two value inputs of the same type; integrates with
Formula comparisons.

### D. Visualization / output nodes (the "card")

**US-16 — KPI output.**
I want to render a single big figure with a label and tone (up/down), so that I get a
clean stat tile.
*AC:* takes a Scalar; format number/percent/currency via `widgetspec`; tone color from a
threshold or sign.

**US-17 — List & table output.**
I want to render a collection as a list or a sortable table, so that I can show recent
transactions, top payees, etc.
*AC:* choose columns from the collection's fields; reuses the `DataTable` component;
empty-state message.

**US-18 — Chart output.**
I want to render a Series as a line/bar/donut chart, so that I can show trends and
breakdowns.
*AC:* takes a Series; reuses the `chartspec` + D3 shim; legend + axis labels.

**US-19 — Layout/stack container.**
I want to combine 2–4 blocks into one card (a KPI header above a list), so that a single
tile can be rich like the built-ins.
*AC:* a container node with ordered child slots; renders blocks vertically within the
tile; respects tile size.

### E. Interactivity & lifecycle

**US-20 — Interactive controls wired to actions.**
I want to add a button/checkbox/row-action that performs an action (complete a task, post
recurring, add a tag), so that my card *does* things, like the To-do tile.
*AC:* Interaction nodes emit Events; Events map to `workflow.Action`s applied through the
`appstate` seam (dry-runnable); the To-do "inline complete" is reproducible end-to-end.

> **Plus two lifecycle stories that fall out of the above (US-2x bonus):**
> - **Save & publish a card** to the dashboard as a managed widget (Widget Manager
>   controls apply); **duplicate / edit / delete**.
> - **Export/import a card** as JSON (lossless, no lock-in) so cards are shareable —
>   matching the app's "the export *is* the sync payload" principle.

---

## 4. System architecture

### 4.1 Two planes

- **Data/render plane** — a **pure DAG**. Source → Transform/Logic → Visualize → root
  Output. Recomputed reactively on the data-revision atom. No side effects. Memoized per
  node by (inputs hash). Topologically evaluated; cycles are a graph error.
- **Event/action plane** — Interaction nodes emit **Events** on user input; Events resolve
  to `workflow.Action`s and apply through `appstate` (the single write seam). This is the
  *only* place state changes, mirroring `workflow.Plan → Effect → apply`.

### 4.2 The type lattice (port types)

```
Scalar:   Number | Money | Text | Bool | Date
Series:   ordered (key → Number)         // for charts / grouped aggregates
Collection<Row>: rows with named typed fields
Viz:      a renderable block (KPI/List/Table/Chart/Text)
Event:    a UI interaction occurrence
Action:   a planned workflow effect
```

Coercions (auto): `Bool→Number` (1/0), `Number/Money/Date→Text` (for display). Everything
else requires an explicit node. Strong typing = the visual constraint that keeps cards
valid.

### 4.3 Node model (pure, native-testable)

A new pure package, proposed `internal/cardgraph` (no `syscall/js`), holding:

```go
type NodeID string
type PortID string                 // "nodeID:portName"

type Node struct {
    ID    NodeID
    Kind  string                   // "source.dataset", "transform.filter", "viz.kpi", …
    Pos   Point                    // canvas position (UI only)
    Props map[string]string        // node config: literal value, formula text, rule id, source key…
}

type Edge struct{ From, To PortID }

type Graph struct {
    Nodes []Node
    Edges []Edge
    Root  NodeID                    // the Output node
}
```

- **Pure functions:** `Validate(Graph) []Issue`, `TopoOrder(Graph) ([]NodeID, error)`
  (cycle detection), and `Eval(Graph, engineenv.Data) (Render, []Issue)` producing a
  serializable **Render** (a tree of Viz blocks + bound Events) that the wasm layer turns
  into `ui.Node`s. All table-tested on native Go — the whole language is exercisable
  without a browser, exactly per the project's clean-architecture rule.
- **Node kinds register a spec**: input ports (name+type), output type, a `props` schema,
  and an evaluator `func(inputs, props, data) (Value, error)`. Adding a node = registering
  a spec; the palette and type-checker are derived from the registry (no hardcoded lists).

### 4.4 Evaluation sketch

```
Eval(graph, data):
  order = TopoOrder(graph)            // error → cycle issue
  values = {}
  for id in order:
     node = graph[id]
     inputs = gather wired input values (typed, coerced)
     spec = registry[node.Kind]
     values[id], err = spec.Eval(inputs, node.Props, data)
     record issue on err (node continues as "error", downstream degrades)
  return RenderFrom(values[graph.Root])
```

`data` is `engineenv.Data` (datasets + FX rates + now), so figures/collections resolve
deterministically — same input → same card.

### 4.5 Persistence

`domain.CardSpec { ID, Name, Graph, Size{Col,Row} }`, stored through `appstate` →
`internal/store` (SQLite) with JSON/CSV import-export (lossless round-trip test). A
published card registers a dashboard layout item with a stable id so the Widget Manager
governs it like any tile.

### 4.6 Rendering & interactivity in the wasm layer

- A thin `internal/screens` renderer walks the pure `Render` tree → `ui.Node`s, reusing
  `Widget`, `DataTable`, `Chart`, KPI bodies, `Toggle`, etc.
- Interaction nodes bind handlers that build `workflow.Action`s and apply via `appstate`
  (with the same notice/undo behavior as elsewhere). The To-do inline-complete pattern is
  the reference implementation.

---

## 5. Build phases

1. **Graph core (pure):** `internal/cardgraph` — model, validate, topo/cycle, eval, node
   kinds + full tests. *No UI.* **✅ DONE (2026-06-23).** Shipped: typed ports
   (number/text/bool/viz) with safe coercions; `Graph`/`Node`/`Edge`; `Validate`;
   `TopoOrder` (Kahn + cycle detection); pure `Eval` that degrades around broken nodes
   (collects `Issue`s, never panics); node kinds `literal.number/text/bool`,
   `source.scalar` (reads the engineenv var surface), `formula` (reuses
   `internal/formula`), `logic.compare`, `logic.branch.number`, `viz.kpi`. Table tests
   green (`go test ./internal/cardgraph/`), gofmt + `go vet` clean, wasm build green. A
   "is net worth positive?" card (source→compare→branch→kpi) is proven end-to-end —
   the visual *language* (sources + logic + branching + output) works in the core.
   The wasm layer builds `cardgraph.Context{Vars,Strs}` from `engineenv.Vars(data)`.
2. **Canvas UI:** palette, place/connect/delete, pan/zoom, type-checked wiring, live stage
   bound to the pure eval. (US-1..US-5, US-6/7, US-16)
   **🟦 IN PROGRESS — first working slice shipped (2026-06-23).** The Widget Builder
   screen (`internal/screens/widgets.go`) now renders a **live KPI card evaluated through
   `cardgraph` against the real app figures** (`engineenv.Vars`): the stage shows your
   actual data and updates as you configure. A per-step **config panel** drives the graph —
   **Source** (pick a live figure), **Transform** (an optional `formula` over the source
   value, exposed as `a`), **Visualize** (title + number/percent/currency format, with
   currency rendered as money at the edge). The n8n pipeline nodes show the live graph
   summary; the graph is built by `wbBuildGraph` (source → [formula] → kpi) as the single
   source of truth. Broken/empty graphs show a friendly "unfinished" state.
   **E2E TESTED:** `e2e/widget_builder_check.mjs` drives it in a headless browser —
   selects a source figure, applies the `a / 2` transform (asserts the previewed figure
   halves — proving the formula engine runs live), switches to currency format (asserts a
   money symbol), and resizes via the stepper. Passes against the served wasm. Still TODO
   for this phase: a free-form drag-to-wire canvas with arbitrary nodes (current shape is
   the fixed source→transform→viz chain), and node placement/deletion.
3. **Data & transforms:** dataset source, period, filter, aggregate, formula, rule, sort/
   limit, branch. (US-8..US-15)
4. **Rich viz:** list/table, chart, layout/stack container. (US-17..US-19)
5. **Interactivity + lifecycle:** interaction nodes → actions; save/publish/duplicate/
   export-import. (US-20 + bonus)

Each phase: data model → tested pure logic → persistence → state → UI last; one feature
per commit; CHANGELOG + DEVLOG updated.

---

## 6. Open risks / decisions for Cam

- **Canvas tech:** pure-Go/WASM SVG canvas vs. a tiny JS shim (like `chart.js`/`flip.js`).
  Leaning **SVG via the Go DSL** to keep "no JS supply chain", with a minimal pan/zoom
  shim only if perf demands. (Confirm.)
- **How much of the type lattice ships in v1** vs. starts Scalar+Collection only.
- **Node count for v1 palette** — I've scoped ~16 kinds across phases; trim if you want a
  smaller first cut.
- Whether published cards should *also* be expressible as existing `PageWidget`s for custom
  pages, or stay dashboard-only at first.

I'll proceed down the phases starting with the **pure `cardgraph` core** (safe, testable,
no conflict with the UI other agents may touch), unless you redirect.

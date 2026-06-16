# CashFlux — Master Feature Backlog

Single source of truth, **ordered top-to-bottom by implementation priority**. Work in order;
within a section earlier items unblock later ones. Build **bottom-up** per the SDLC rule
(data model → services/logic with tests → persistence → state → UI last). See [`SPEC.md`](./SPEC.md)
for product detail and [`CLAUDE.md`](./CLAUDE.md) for the rules.

**Legend:** `[ ]` todo · `[x]` done · `[~]` in progress · `(P#)` phase · `★` critical path.
**Discipline:** one feature per commit; update `CHANGELOG.md` + `DEVLOG.md` each commit; pure logic
packages have no `syscall/js` and ship with table-driven tests.

---

## B. Bug fixes (active, high priority) ★

### B1. Deep-link refresh 404 on non-root paths ★

**Symptom:** visiting/refreshing a non-root URL (e.g. `http://127.0.0.1:8080/accounts`) returns a
404 instead of routing to the screen.
**Root cause:** the app uses `router.NewHistoryRouter` (clean pushState URLs; `internal/app/app.go`).
The client-side `*` fallback (`app.go`) only runs *after* the wasm app boots. A hard refresh / direct
visit to `/accounts` makes the browser request `/accounts` from the server, which has no such file and
404s before `index.html` (and thus the SPA) loads. The service worker (`web/sw.js`) only falls back to
cache on a thrown network error — not on a non-ok response — and `/accounts` isn't cached, so the 404
passes through. It's a server/SW history-fallback gap, not a router bug.
**Fix (layered; clean paths must keep working — no hash router):**
- [x] Service worker: for navigation requests (`event.request.mode === "navigate"`), serve the cached
      app shell (`./index.html`) when the network returns non-ok or throws, so deep-link refresh works
      on repeat / installed / offline visits. (`web/sw.js`, CACHE bumped to v2)
- [ ] Server (dev): make `gwc dev` serve `index.html` for unknown non-asset paths (SPA history
      fallback). Resolve the known `gwc dev -html` issue (see §0) — framework-side change.
      _(2026-06-16: confirmed empirically — `gwc dev` returns 404 for `/`, `/index.html`, **and**
      `/accounts` while `/bin/main.wasm` serves 200, so the HTML shell isn't served at any route. Both
      the HTML-resolution bug and the missing SPA fallback live in the GoWebComponents dev tool, not
      this repo. README now documents the caveat + workaround.)_
- [x] Server (prod/static hosting): document the SPA rewrite rule (all non-asset routes → `index.html`)
      — README "Hosting (SPA history fallback)" covers GitHub Pages (404.html), Netlify, Vercel, nginx, Caddy.
- [ ] Verify: hard-refresh at `/accounts`, `/transactions`, `/budgets`, … lands on the correct screen
      online and offline; the `*` route still catches genuinely unknown paths.

### B2. Dashboard drag should reflow like an iOS app grid (respect multi-cell tiles) ★

**Symptom:** dragging a dashboard widget swaps it 1:1 with the drop target instead of inserting it and
letting the other tiles reflow; multi-cell (multi-span) widgets aren't handled and can overlap.
**Root cause:** `ui.Widget` (`internal/ui/widget.go`) handles `OnDrop` by calling
`dashlayout.Layout.Swap(src, target)`, which exchanges the two widgets' absolute `Col/Row` **and**
spans. So (a) only the two tiles move — the rest don't reflow; (b) no live displacement during the
drag (acts only on drop); (c) swapping spans between differently-sized tiles overlaps neighbors and
corrupts the bento packing. The model is absolute-placement + pairwise-swap; iOS-grid behavior needs
ordered reflow + size-aware packing.
**Fix (bottom-up per SDLC):**
- [ ] Model: change `internal/dashlayout` from absolute placement+`Swap` to an **ordered sequence** of
      tiles (`ID` + `ColSpan` + `RowSpan`) plus a pure **`Pack`** that flows tiles into the N-column
      grid (first-fit/dense bin-packing, top→bottom, no overlap, honoring each tile's spans) and derives
      `Col/Row`. Keep the `GridColumn()/GridRow()` rendering.
- [ ] Ops: replace `Swap` with `Move(id, toIndex)` (reorder the sequence) then re-`Pack`; keep
      `Resize` (re-`Pack` after a span change). **Table tests:** mixed-span packing has no overlaps,
      wraps to the next row, `Move` reflows the rest, determinism, no-mutation, clamp oversized spans
      to the column count.
- [ ] State: persist the ordered sequence + spans; migrate the existing localStorage layout and
      tolerate the old absolute format.
- [ ] UI: live reflow — on drag-over compute the insertion index and re-pack a preview (CSS-transition
      animate the shifts); commit on drop. Prefer pointer events over HTML5 DnD for smooth movement +
      touch support. Respect the On*-hooks-in-loops rule (the cell component owns its handlers).
- [ ] **Animate reorder**: tiles that shift during a reflow move smoothly, iOS-home-screen style.
      CSS-grid placement changes don't transition natively → use a FLIP technique (measure old/new
      rects, transform from old→new, transition the transform to zero) keyed by widget id.
- [ ] **Animate resize**: growing/shrinking a tile's span scales smoothly rather than snapping
      (transition the cell, FLIP the neighbors that reflow around it). Pairs with the reorder FLIP.
- [x] **Resize handles only while holding Shift**: `.rz` hidden by default, revealed when the root has
      `data-resize` (toggled by a global Shift keydown/keyup listener + window-blur clear in
      `internal/app/resizereveal.go`), with an opacity fade. Keeps the bento visually calm.
- [ ] Verify: dragging a 1×1 into a row of 2×2s reflows cleanly; multi-cell tiles never overlap;
      resize re-packs; reorder/resize animate smoothly; layout persists across reload.

### B3. Routing sometimes duplicates the whole page ★

**Symptom:** navigating between screens sometimes renders the page twice (two sidebars / top bars /
screens stacked).
**Root cause (from GoWebComponents router source — live DOM scan was unavailable, see note):** the
framework router is a **nested-layout router** (`router/doc.go`: "Nested layout routes with explicit
outlets"; a layout renders chrome and places `router.GetOutlet()` where the active child goes). For a
path like `/accounts`, `expandPathPrefixes` returns `["/", "/accounts"]`, so `resolveRouteStack`
builds the stack `[exact "/", exact "/accounts"]` and renders `/` as the **parent layout** that wraps
`/accounts` through the outlet. But `internal/app/app.go` registers **every** route — including `/` —
as a full `Shell` page, and no `Shell` calls `router.GetOutlet()`. So any non-root navigation renders
two full Shells (the `/` Dashboard Shell as the parent + the target screen's Shell as the unplaced
child), duplicating the chrome/page. (The `*` route is *not* the cause: `Register("*", …)` is the
router's dedicated not-found factory, not a stacking pattern.)
**Fix (framework-intended layout + outlet structure):**
- [ ] Register `/` as a **layout** component that renders the Shell chrome **once** and places
      `router.GetOutlet()` for the active child — the layout must NOT itself be the Dashboard.
- [ ] Register each screen as a **child route** that renders only its screen content (drop the
      per-screen `Shell` wrapper in `app.go`); the layout supplies the chrome.
- [ ] Make the Dashboard an **index child** of the layout (its own route) so home content also lands
      in the outlet, rather than `/` doubling as both the universal parent layout and the dashboard.
- [ ] Keep `*` as the not-found registration (already correct).
- [ ] Verify (ideally with the browser oracle once Playwright is installed — see §0): navigating and
      hard-refreshing every route renders exactly one Shell; no stacked/duplicated chrome.
- _Note:_ couldn't scan the live DOM this session — `gwc probe` reports `playwright unavailable` and
  the `gwc` MCP server isn't connected. Diagnosis is from the router source, which is definitive here.
  Installing the Playwright driver (§0) would let `gwc probe`/MCP confirm the DOM directly.

### B4. Settings is duplicated — consolidate into the household-card panel ★

**Symptom:** the "Settings" item in the menu list opens what looks like a duplicate of the settings
you get from the **Your household** card at the bottom of the rail. The household card should be the
single, primary settings panel.
**Root cause:** there are two settings surfaces. (1) The **Settings** nav item → `/settings` route →
`screens.Settings()`, which only shows a *read-only* Household summary (base currency + member/account/
category counts) plus the Debug log — so it reads as an emptier duplicate. (2) The **household card**
(`app.HouseholdCard`, rail bottom) → the global settings flip panel (`globalSettingsForm` in
`internal/app/settings.go`), which holds all the real editing: members, base currency + FX rates, AI
key/model, appearance (theme/accent/density/week-start/date), data export/import/sample/wipe, freshness
overrides, module-visibility toggles.
**Fix (make the household-card panel the one primary settings surface):** — done.
- [x] Moved the **Debug log viewer** (+Refresh, last 25 entries) into the global panel
      (`globalSettingsForm`). Satisfies §1.18 "Debug: open log viewer".
- [x] Removed the "Settings" nav item + `/settings` route and deleted `screens.Settings()`. Single
      entry point = the household card.
- [x] Updated the module-visibility locked set (only `/` now) + the modules tests.
- [x] Members/Categories remain their own nav screens; the panel's manage-links are unchanged.
- [~] Verify: one settings entry point, debug log in the panel, nothing regresses (full `go test ./...`
      + wasm green; browser spot-check pending).

### B5. Collapsed rail should reveal labels on hover ★

**Symptom / want:** the left menu should collapse to icons-only, and hovering an icon should show a
text label ("text highlight") for quick reference.
**Current state:** the rail already collapses to a 58px icon-only mode (`.collapsed`, shared
`rail:collapsed` atom; `internal/app/shell.go`), which hides each item's label `Span`. What's missing
is the hover affordance — collapsed, there's no quick way to see what an icon is.
**Fix:**
- [x] `title` attr on every nav item (via `navItem`) **and** the household card — native tooltip +
      accessible name when collapsed.
- [x] CSS flyout: in `.collapsed`, `.nv:hover/:focus-visible/:focus-within > span` reveals the label as
      an absolutely-positioned pill to the right (overlays content, doesn't widen the rail). Covers all
      nav groups (primary/Tools/System/My pages) since they share the `.nv` class.
- [x] Respects `prefers-reduced-motion` (fade-in gated); keyboard focus reveals via `:focus-visible`/
      `:focus-within`.
- [~] Verify: hover/focus reveals the label without expanding the rail (wasm green; browser spot-check pending).

### B6. Add a UI / font-size scale setting ★

**Want:** fonts and buttons feel ~30% too large for some users (e.g. on `/accounts`), though others
find them fine — add a setting to scale the whole interface up or down.
**Approach (analysis):** the design is px-heavy (Tailwind arbitrary px like `text-[13px]`), so a
rem-based root-font scale would NOT resize buttons/spacing. Use a **whole-UI zoom**: a `--ui-scale`
CSS variable applied via `zoom` on `#app` (Chromium target; `zoom` reflows and scales fonts + buttons
+ spacing together).
- [x] `internal/prefs`: `Scale` percent field (range 70–130, default 100) + `Normalize` clamp (0/unset
      → 100) + `ScaleFraction()`; table-tested.
- [x] `uistate.ApplyPrefs`: sets `--ui-scale` from the scale; CSS `#app { zoom: var(--ui-scale, 1); }`.
- [x] Settings → Appearance: a "Display scale" select (70%–130%, 100% marked default); persists with
      prefs (reload-persistent, like theme/accent/density).
- [~] Verify: changing scale resizes the whole UI (wasm build green; browser spot-check pending); 100% == current.

### B7. Menu is missing main-line features ★

**Symptom:** the sidebar lists fewer items than the app implements. Primary nav has Dashboard /
Accounts / Transactions / Budgets / Goals / To-do; System has Members / Categories / Settings. But
`screens.All()` also routes five Phase-2 screens that are **not in the rail** — reachable only by
typing the URL: **Planning** (`/planning`), **Allocate** (`/allocate`), **Insights** (`/insights`),
**Documents** (`/documents`), **Customize** (`/customize`).
**Fix:**
- [x] Add the five missing screens to the sidebar — a "Tools" nav group (`shell.go` `toolsNav()`/
      `Sidebar`) with new icons (planning/allocate/insights/customize; documents reuses `page`).
- [x] They respect the module-visibility set (filtered by `hidden.IsHidden(path)` like primary nav).
- [ ] Optional hardening: derive nav groups from `screens.All()` (or a Group field on Route) so a new
      routed screen can't silently miss the menu again.
- [~] Verify: every routed main-line screen now has a menu entry (wasm build green; browser spot-check
      pending). Module toggles cover them via the hidden-path filter.

### B8. Sidebar menu management: reorder, drop "My pages", visibility settings ★

Three related sidebar changes (relates to B5 collapsed-hover, B7 missing items):
- [ ] **Shift+drag reorder.** Holding Shift turns nav items draggable; drag-reorder the menu (same
      Shift-gating idea as the dashboard resize handles). Persist the order to localStorage (a
      `uistate` nav-order atom) and apply it when rendering `primaryNav`/System. Reuse the
      `internal/dashlayout` ordered-sequence/`Move` approach or a small pure `navorder` helper +
      table tests. Respect the On*-hooks-in-loops rule (each item already its own `navItem` component).
- [x] **Remove the "My pages" segment.** Dropped the `myPages()` example section + "New page"
      affordance (and the dead `customPage`/`myPages` code) from the rail — the rail is now just the
      real screens.
- [x] **Menu visibility settings.** `hideableScreens` (Settings → Screens toggles) now covers every
      routed main-line screen — primary nav, the Tools group (Planning/Allocate/Insights/Documents/
      Customize), and System (Members/Categories/Rules). Dashboard stays locked.
- [~] Verify: no "My pages" group (done, wasm green). Shift+drag reorder still pending.

### B9. Clickable breadcrumb in the top bar ★

**Want:** an easy-to-read, clickable breadcrumb on the right side of the top-level panel so users can
see where they are and step backwards.
**Context:** the top bar (`internal/app/shell.go` `TopBar`) shows the page title on the left and the
resolution control + "+ Add" on the right (`ml-auto`). Routing is **flat** — Dashboard, Accounts,
Transactions, … are siblings with no nesting (`screens.All()`), so there's no natural multi-level
trail yet.
**Open decision (resolve before building) — what does the trail contain?**
  1. *Home-rooted* (simplest, recommended): `Dashboard / {Current Page}`, with "Dashboard" clickable to
     go home. Static, derived from the current route — no history needed.
  2. *Visited history*: last N visited pages as crumbs (browser-like back trail). Needs a small
     nav-history atom.
  3. *Logical hierarchy*: e.g. `Dashboard / Accounts / {account} transactions` once drill-downs carry
     context (account→ledger filter already exists). Richest but needs per-drill-down context.
**Fix — implemented option 1 (home-rooted):**
- [x] Derived from `router.InspectCurrentRoute().Path`: a `Dashboard › {page}` breadcrumb in the top
      bar (replaced the plain title); last crumb non-link, marked `aria-current="page"`. On the
      dashboard route only the title shows.
- [x] The Dashboard crumb navigates via the existing `nav` (router.UseNavigate); muted styling + hover.
- [x] Keyboard-accessible (a real `<button>` crumb) + `<nav aria-label="Breadcrumb">`; theme-agnostic
      utility classes.
- [~] Verify: trail correct per screen; clicking returns home (wasm green; browser spot-check pending).

### B10. Rethink the time-resolution control (drastic UX improvement) ★

**Current control** (`internal/app/shell.go` `ResolutionControl`, driven by `internal/period.Window`):
a `Week | Month | Quarter` segmented toggle + **two** independent stepper pills (`From` ‹ › and
`To` ‹ ›) joined by an em-dash, where each pill steps one unit and the pair defines a range.

**Why it's confusing (deep analysis):**
1. **Two steppers for a range is the wrong default.** The overwhelmingly common need is *one* period
   ("this month"). Presenting two anchors forces every user to reason about a range they rarely want,
   and it's unclear the two pills are even related.
2. **Redundant when From == To.** In the common single-period case the control reads "Jun 2026 – Jun
   2026", which looks broken/duplicated rather than "Jun 2026".
3. **No way back to "now".** Once you step away there's no "This month"/Today reset — you must count
   clicks back. There's no visual cue that you've left the current period.
4. **No presets.** The most common selections (This/Last month, This quarter, Year to date, Last 30
   days) all require stepping; there are no one-tap presets.
5. **Granularity ↔ range coupling is invisible.** Switching Week→Quarter re-snaps the anchors
   (correct, via `SetResolution`) but nothing explains the jump.
6. **Width / competition.** Segmented + two steppers is wide and will crowd the top bar next to "+ Add"
   and the planned breadcrumb (B9); no responsive/narrow behavior.
7. **Discoverability of range mode.** Power users *do* want custom ranges, but that shouldn't tax the
   90% single-period case.

**Proposed redesign (drastic):**
- **Primary = a single period stepper** ‹ `Jun 2026` › for the common case (From==To), reading as one
  label. A small **granularity affordance** (Week/Month/Quarter) stays but compact (e.g. a dropdown or
  the segmented shown only when the period menu is open).
- **A presets dropdown** ("This month ▾"): This month, Last month, This quarter, Year to date, Last 30
  days, Custom range… — plus the Week/Month/Quarter choice. One tap for the common ranges.
- **A "This {period}" / Today reset** that re-anchors to now, with a subtle indicator when the view is
  off the current period.
- **Custom range** reveals the From/To steppers (today's behavior) only when chosen — advanced, not
  default. Show "From – To" only in range mode; a single label otherwise.

**Bottom-up plan (pure logic first):**
- [x] `internal/period` preset constructors (pure, table-tested): `Previous`, `YearToDate`, plus
      `Window.Shift` (page the whole window) and `Window.IsCurrent` (off-now predicate). `ThisPeriod` =
      `NewWindow`. `LastNDays` dropped — arbitrary day ranges don't fit the unit-based Window model
      (would need a different representation if wanted).
- [ ] `Window`: a single-period helper (set both anchors to one unit) + a clear "is single period" check
      so the label collapses correctly. Tests.
- [ ] UI: rebuild `ResolutionControl` as single-stepper + presets dropdown + reset, with Custom-range
      revealing the dual steppers. Keep persisting resolution (and now presets/range) per B existing
      reload-persistence.
- [ ] Responsive: collapse gracefully in a narrow top bar.
- [ ] Verify: single-period is one tap and reads cleanly; presets work; "this period" resets to now;
      custom range still does everything today's control can; persists across reload.
- _Decision to confirm:_ how far to simplify — keep the full From/To range power behind "Custom range"
  (recommended), or drop ranges entirely for a single-period-only control.

### B12. Wire per-widget flip-panel settings to content (persisted) ★

**Goal:** clicking a dashboard widget's gear opens its *own* settings in the flip panel (e.g. Savings
rate → savings settings), with values persisted, and the widget renders accordingly.
**Done (foundation, committed):** pure `internal/widgetcfg` — typed `Field`/`Schema`/`Config` +
registry + accessors; savings rate schema (target rate %, show-bar toggle). Table-tested.
**Remaining (the wiring — was started then deferred back to planning):**
- [x] `uistate` persisted atom `WidgetConfigs` (`map[widgetID]widgetcfg.Config`) backed by
      localStorage (load/persist + a copy-on-write `WithField` setter), mirroring the layout/filter atoms.
- [x] Rebuilt `app.widgetSettingsForm` schema-driven: ID threaded from `SettingsHost`, looks up
      `widgetcfg.SchemaFor(id)`, renders a control per field (toggle/number/select) via a dedicated
      `widgetFieldRow` component bound to the persisted config; placeholder for widgets with no schema.
- [x] Savings widget consumes its config: reads target rate + show-bar from `WidgetConfigs.For("savings")`
      — tone reflects performance vs target (green/amber/red), subline shows the target, bar hides when off.
- [ ] Register feasible schemas for the other widgets incrementally (recent-transactions count,
      budgets scope, trend range, etc.) — "any feasible settings exposed and persisted".
- [ ] Verify: gear opens widget-specific settings; changes persist across reload; savings reflects its
      target.

### B11. "+ Add" opens a flip-panel of add actions ★

**Want:** the top-bar "+ Add" button should open a centered flip panel (the same lift-to-center +
`rotateY` animation as settings) offering the kinds of things you can add — new transaction, bills to
scan, docs to scan, custom workflows, etc. — instead of jumping straight to `/transactions`.
**Context / reuse:** the flip animation + centered panel already exist as `ui.FlipPanel`, driven by
the `uistate.UseSettings()` atom and rendered by `app.SettingsHost` (kinds: "global" / "widget"). The
cleanest path is to **reuse that mechanism** rather than build a parallel overlay.
**Fix:**
- [ ] Add an "add" target to the settings/overlay atom (e.g. `uistate.AddPanel()` → `SettingsTarget`
      kind "add") and render it in `SettingsHost` as a `ui.FlipPanel` whose back face is a new
      `addMenu` component.
- [ ] `addMenu`: a grid of large, labelled, icon'd action cards. Initial set:
      - **New transaction** → opens the transaction add flow (today's `/transactions` add form).
      - **Scan a bill** → the Documents image/vision import (`ai.BuildVisionRequest` + extract review).
      - **Scan a document** → Documents CSV/image import.
      - **Custom workflow** → (see decision below).
      - extensible — easy to add more cards later.
- [ ] Repoint the "+ Add" button (`TopBar`) to open the panel instead of `nav.Navigate("/transactions")`.
- [ ] Each card either navigates to the relevant screen with the right mode or opens its own sub-flow;
      keyboard-accessible, labelled, light/dark.
- [ ] Verify: "+ Add" flips open the panel; each action starts the right flow; Esc/✕ closes.
- _Decision to confirm:_ what "custom workflows" means here — map to the existing Customize screen
  (custom fields + formula builder), or a new "workflow" concept? Need scope before building that card.

### B13. Integrate Lucide icons behind a strong Go interface ★

**Goal:** replace the hand-rolled icon set with [Lucide](https://lucide.dev) glyphs, exposed through a
**type-safe** Go API (no stringly-typed names).
**Current:** `internal/ui.Icon(name string, …)` switches on a string and emits inline 24×24 stroked
`currentColor` SVGs — already Lucide's exact format, so this is a clean swap, not a rewrite.
**Proposed strong Go interface (pure `internal/icon`):**
```go
package icon
// Name is a Lucide icon id; only the constants below are valid (compile-checked).
type Name string
const (
    Dashboard    Name = "layout-dashboard"
    Wallet       Name = "wallet"
    Transactions Name = "arrow-left-right"
    TrendingUp   Name = "trending-up"
    // … the curated set the app actually uses
)
// Inner returns the icon's inner SVG markup (Lucide path data); "" if unknown.
func (n Name) Inner() string
func (n Name) Valid() bool
```
Then `internal/ui.Icon(n icon.Name, extra ...PropOption) ui.Node` renders Lucide's paths with the
existing viewBox/stroke/currentColor defaults (size + tint still via caller classes).
**Approach decision (flag before building):**
  - **Embed at build time (recommended):** a small generator pulls the curated icons' SVG path data
    from the Lucide package into a generated Go file (`internal/icon/icons_gen.go`). No runtime JS
    dependency → robust with the vdom and works offline (PWA). Re-run to add icons.
  - *vs.* CDN + `lucide.createIcons()` rewriting `<i data-lucide>` — simpler but fights the framework's
    vdom and needs network; **not** recommended for this wasm/offline app.
**Bottom-up plan:**
- [ ] `internal/icon`: `Name` + curated constants + embedded Lucide path data; `Inner`/`Valid`; tests
      (every constant resolves to non-empty data; `Valid` correct). Pure, no `syscall/js`.
- [ ] Generator/script to fetch Lucide SVGs for the set and write the Go file (documented, repeatable).
- [ ] Rewire `ui.Icon` to take `icon.Name`; migrate call sites (sidebar nav string names → Lucide ids).
- [ ] Verify: all existing icons render identically/closely; unknown-name path is gone (compile-checked).

### B14. Integrate D3 charting behind a strong Go interface ★

**Goal:** richer, interactive charts via [D3](https://d3js.org), exposed through a **declarative,
typed** Go spec — the Go side describes a chart; the D3/JS is hidden.
**Current:** charts are pure-Go SVG (`ui.AreaChart` + the `chart` helper) — works, but limited (no
axes/ticks/tooltips/transitions).
**Proposed strong Go interface:**
```go
// Pure, testable spec — no syscall/js (internal/chartspec).
package chartspec
type Kind string
const ( Line Kind = "line"; Area Kind = "area"; Bar Kind = "bar"; Donut Kind = "donut" )
type Point  struct { X, Y float64; Label string }
type Series struct { Name, Color string; Points []Point }
type Axis   struct { Label, Format string }
type Spec   struct { Kind Kind; Series []Series; X, Y Axis; Stacked, Legend bool }
func (s Spec) Validate() error
func (s Spec) Extent() (minX, maxX, minY, maxY float64) // pure scale helpers, tested
```
Then `internal/ui.Chart(spec chartspec.Spec, extra ...PropOption) ui.Node` renders it via D3.
**Integration approach (the hard part — D3 mutates the DOM, the framework owns a vdom):**
  - Render a **managed container** the framework creates but doesn't draw into; in a `UseEffect` keyed
    on a hash of the spec, call a thin JS shim `cashfluxRenderChart(el, specJSON)` that runs D3 to draw
    into it; redraw on spec change; clean up on unmount (the ref/portal pattern).
  - Load D3 via a pinned CDN `<script>` in `index.html`; **add it to the service-worker `CORE` cache**
    so charts work offline (PWA).
**Decision to confirm (significant):** D3 is a large dependency and the pure-Go SVG charts already
work. Adopt D3 for the richer/interactive charts (accepting the JS dep + offline caching + the
vdom-portal complexity), **or** keep growing the pure-Go SVG helpers (no dep, fully testable)? If D3:
which chart kinds first (line/area/bar/donut)?
**Bottom-up plan (assuming D3 is approved):**
- [ ] `internal/chartspec`: the typed spec + `Validate` + scale/extent helpers; table tests. Pure.
- [ ] JS shim (`web/`) `cashfluxRenderChart(el, specJSON)` building each Kind with D3; pin D3 version;
      SW-cache it.
- [ ] `ui.Chart`: managed container + effect that drives the shim; cleanup; theme-aware (reads CSS vars).
- [ ] Migrate one widget (e.g. net-worth trend) to `ui.Chart` as the proof; keep others until parity.
- [ ] Verify: chart renders + updates on data change, survives hot-reload, works offline, matches theme.

### B15. App-wide accessibility — spike + program ★

**Goal:** make CashFlux usable with a keyboard and a screen reader, at high zoom, and without relying
on color — to WCAG 2.1 AA as the bar. This is large and cross-cutting, so it starts as a **spike**
(time-boxed audit → prioritized plan) before the implementation tasks it spawns. Supersedes the
one-line a11y item in §1.20.

**B15.0 — Spike (do first):**
- [ ] Audit current state: run an automated pass (axe-core via the `gwc` browser oracle / Playwright
      once installed — see §0), plus a manual keyboard-only pass and a screen-reader pass (NVDA on
      Windows / VoiceOver). Inventory concrete gaps per screen + shared component.
- [ ] Catalogue what the framework already provides: GoWebComponents a11y primitives (CLAUDE.md says
      "use the framework a11y primitives") — which roles/focus/live-region helpers exist and how to
      apply them, so we build on them rather than hand-rolling ARIA.
- [ ] Decide reusable patterns: dialog/focus-trap for `FlipPanel`, ARIA for each custom control,
      chart alt-text strategy, focus-on-route-change, a contrast-checked token set.
- [ ] Output: a findings note + prioritized follow-up tasks (the checklist below becomes concrete,
      assigned items). Spike is done when the plan is actionable, not when a11y is "finished".

**Deep analysis — the areas the program must cover (becomes tasks after the spike):**
- [~] **Semantics & landmarks:** the sidebar `<nav>` is now labelled "Main navigation" (distinct from
      the breadcrumb nav); `<main>`/`<aside>`/`<nav>` provide implicit landmarks. Still TODO: a single
      `<h1>` per screen + heading order, a "skip to content" link, `banner`/`contentinfo` roles.
- [ ] **Keyboard:** everything reachable + operable in a logical tab order; **the bento drag/resize is
      pointer-only — needs a keyboard alternative** (move/resize via arrows; Shift+drag in B2/B8 is
      pointer-only); inline-edit rows manage focus on enter/exit.
- [ ] **Dialogs (`FlipPanel`, the B11 add panel, confirms):** `role="dialog"` + `aria-modal`, focus
      trap, Esc to close, restore focus to the trigger on close.
- [~] **Custom controls → correct ARIA:** Segmented = `role="radiogroup"`/`role="radio"`/`aria-checked`;
      Toggle/ToggleRow = `role="switch"` + `aria-checked` + name; StepperPill ‹/› have `aria-label`s;
      SwatchPicker = labelled `role="radiogroup"` of `role="radio"` chips. Still TODO: the gear/menu/grip
      icon-buttons' `aria-label`s, and real keyboard operability for the div-based Toggle/Swatch.
- [ ] **Focus visibility:** clear `:focus-visible` rings in both themes (custom styling must not
      suppress outlines).
- [ ] **Screen-reader / live regions:** announce dynamic changes — filtered-result counts, balance
      updates, toast errors (toast is `role=status` polite; consider assertive for errors); associate
      form errors via `aria-describedby`; mark required fields.
- [ ] **Color is never the only cue:** up/down, over-budget, stale, cleared must carry a non-color
      signal (arrows/sign/parentheses/text) — audit every color-coded state.
- [ ] **Contrast:** verify AA (4.5:1 text / 3:1 large+UI) for both themes — `text-faint` and accent-on-
      surface are suspect; bake checked values into the token set.
- [ ] **Motion:** `prefers-reduced-motion` for the dashboard reorder/resize animations (B2) and the
      flip panel (boot loader + settle already do).
- [ ] **Zoom / reflow:** usable at 200% browser zoom and with enlarged browser font sizes — the
      px-heavy styling (see B6) is the risk; pairs with the B6 UI-scale work.
- [ ] **Forms:** correct input types (number/date), every field labelled, inline validation announced.
- [ ] **Route changes (SPA):** update `document.title` and move focus to the new screen's `<h1>`/main on
      navigation (intersects B3 routing + B9 breadcrumb).
- [~] **Charts:** `ui.AreaChart` is now `role="img"` + `aria-label` with a live-figure summary (net-worth
      trend, forecast). Remaining: the div-based bar charts + any future D3 charts (B14).
- [ ] **Touch targets:** ≥44×44px hit areas (watch compact density + small icon buttons).
- [ ] **i18n:** route `aria-label`s/announcements through the language store (B i18n) so they translate.
- [ ] **Tooling:** wire an automated a11y check into CI (axe via the browser lane) once Playwright is in.

### B16. End-to-end test stories — every feature, UX + correctness ★

**Goal:** a *trustworthy* app: dozens of E2E "stories" (scripted user journeys) covering every feature's
standard path so it's provably flawless and regression-guarded. Each story asserts **both** UX (the
standard path is smooth — controls reachable, feedback shown, no dead ends) **and** correctness (the
resulting data, persisted state, and derived figures match expectations). Canonical example: *add a
transaction* — open the form, fill it the standard way, save, see it appear in the ledger, see balances
and dashboard KPIs update, and confirm it survives reload.
**Tooling:** browser E2E needs the framework's wasm/browser lane (`gwc test -lane wasm -lane browser`)
which requires **Playwright + Chromium (§0, not yet installed)**; `gwc export-test` can also generate a
testkit Go test from a recorded live session. Until that's installed, author/queue the stories and keep
relying on the (already extensive) pure-logic unit tests. Run the suite in CI once available.
**Story backlog (each = happy path + key edge cases; expand to "dozens"):**
- [ ] Transactions: add expense / add income / add transfer (paired, excluded from totals) / inline-edit
      / delete (+transfer-pair removal) / duplicate / repeat-last / tags.
- [ ] Transactions: filter (member/account/category/text/date) + sort + clear + **persist across reload**;
      CSV export of the filtered view; filtered summary line correctness.
- [ ] Reconcile: toggle cleared, cleared-status filter, bulk clear/unclear, cleared balance, update-balance
      adjustment txn.
- [ ] Accounts: add asset / add liability (+sub-forms) / archive+restore / inline-edit / mark-all-updated /
      drill to filtered ledger; net-worth header correctness.
- [ ] Budgets: create / period switch (week/month/quarter) / near+over indicators / summary totals.
- [ ] Goals: create / contribute / pace + projection.
- [ ] Members: add / set default / inline-edit / reassign-on-delete. Categories: add / sub-category /
      reassign-on-delete.
- [ ] To-do: add / complete-toggle / inline-edit / ordering / hide-done; create-from-insight + from-nudge.
- [ ] Settings: theme/accent/density/week-start/date-format **persist across reload**; export→import
      round-trip; load sample; wipe (with confirm).
- [ ] Dashboard: resolution control (presets/range), KPIs match the data, drill-downs, widget settings (B12),
      drag/resize (B2).
- [ ] Documents: CSV import; image vision import review + dedupe. AI: insight/Q&A → save-as-task.
- [ ] Allocate / Planning / Customize(formula) happy paths.
- [ ] Cross-cutting: reload persistence, offline (PWA), deep-link routing (B1/B3), accessibility journeys (B15).
- [ ] Organize as story files; gate CI on them once the browser lane is available; aim for full
      standard-path coverage of every feature.

---

## 0. Foundation & tooling (Phase 0)

- [x] Install toolchain (Go 1.26.4, Git, GitHub CLI) on PATH
- [x] Init repo, name project, git on `main`
- [x] Consume GoWebComponents as a versioned Go module (no local replace)
- [x] WASM entrypoint builds + serves
- [x] `gwc` runner + MCP server wired (`.tools/gwc.exe`, `.mcp.json`)
- [x] Init framework `GoGRPCBridge` submodule
- [x] Spec, CLAUDE rules, CHANGELOG, DEVLOG, framework notes, this backlog
- [x] Routed app shell + nav + stub screens, served on live view
- [x] Clean standard layout (`main.go`, `internal/`, `web/`, `docs/`)
- [x] ★ `.gitattributes` (normalize LF; mark `*.wasm` binary) — fixes CRLF warnings
- [x] Create GitHub repo `monstercameron/CashFlux` + push (remote `origin`; `main` tracks `origin/main`)
- [x] CI: GitHub Actions — `go vet` + `go test` (logic pkgs) + wasm build on push/PR (`.github/workflows/ci.yml`)
- [~] **README.md** — what CashFlux is, the stack (Go→wasm on GoWebComponents), local dev (`gwc dev`),
      build/test commands, the local-first + BYO-AI-key model, badges, a **Live demo** link to the
      GitHub Pages build, a License section, and pointers to SPEC/DEVLOG/TODOS — all present.
      - [ ] Still TODO: screenshots/GIF (needs a browser capture + image assets; do deliberately).
- [~] **MIT licensing.** Set the project up under the MIT license.
      - [x] Top-level `LICENSE` file (standard MIT text, 2026, copyright holder `monstercameron`).
      - [x] Establish the lightweight convention: one-line `// SPDX-License-Identifier: MIT` in the
            `main.go` entrypoint (above the `//go:build` constraint; wasm build verified unaffected).
      - [ ] Optional: sweep the SPDX one-liner across the remaining Go files (deferred — mechanical,
            and fragile around build-tagged files; do deliberately).
      - [x] Note the license in `README.md` ("License" section + MIT badge) — done with the README pass.
- [x] **Host the app on GitHub Pages.** Done via Actions instead of a committed `/docs` folder:
      `.github/workflows/deploy-pages.yml` builds the wasm site on every push to `main` and deploys it
      as a Pages artifact (`upload-pages-artifact` + `deploy-pages`) — relative asset paths (already
      `./…`) work under the `/CashFlux/` subpath, and a `404.html` shell is generated for deep-link
      routing (static-host side of B1). No committed build artifacts, no commit loops.
  - [ ] **One-time:** set repo Settings → Pages → Source = "GitHub Actions" (or via `gh api`), then the
        live URL is https://monstercameron.github.io/CashFlux/.
- [ ] Fix framework `gwc dev -html` resolution (commit in GoWebComponents, rebuild + recopy `gwc`)
- [ ] `playwrightgo`-tagged `gwc` + Chromium for automated DOM verification (optional)
- [ ] Install Claude Code design skills (`frontend-design`, `playground`) — user action
- [ ] Decide native test command (logic pkgs only; js/wasm pkgs excluded) + document it

---

## 1. Phase 1 — Local household core

### 1.1 Domain types — `internal/domain` ★ (pure, no build tags)

- [x] ★ `Member{ID, Name, Color, IsDefault}`
- [x] ★ `Account` core fields: `ID, Name, OwnerID, Scope(individual|shared), Class(asset|liability), Type, Currency, OpeningBalance, BalanceAsOf, Archived`
- [x] ★ Account liability fields: `CreditLimit, InterestRateAPR, MinPayment, DueDayOfMonth, Lender`
- [x] ★ Account allocation fields: `ExpectedReturnAPR, LiquidityScore, StabilityScore, LockUntil`
      (LockUntil set on add + inline edit; excludes locked accounts from allocation)
- [x] ★ `Category{ID, Name, Kind(income|expense), Color, ParentID}`
- [x] ★ `Transaction{ID, AccountID, Date, Payee, Desc, CategoryID, Amount(Money), TransferAccountID, Cleared, Tags, MemberID, SourceDocID}`
- [x] ★ `Budget{ID, Name, Scope(individual|group), OwnerID, CategoryID, Period(monthly), Limit(Money)}`
- [x] ★ `Goal{ID, Name, Scope, OwnerID, TargetAmount, CurrentAmount, TargetDate, AccountID}`
- [x] ★ `Task{ID, Title, Notes, Due, Status(open|done), Priority(low|med|high), RelatedType, RelatedID, MemberID, Source(manual|ai|nudge)}`
- [x] Enums + `Valid()`/`String()` for `AccountClass`, `AccountType`, `CategoryKind`, `Scope`, `TaskStatus`, `TaskPriority`, `RelatedType`
- [x] `custom map[string]any` field on every entity (for custom fields)
- [x] Doc comments on every exported type/field; package doc
- [x] Unit tests: enum `Valid()`/`String()`, zero-value sanity

### 1.2 Money & currency — ★

- [x] ★ `internal/money`: `Money{Amount int64, Currency}`; `Add/Sub/Neg/Abs/Cmp/Equal/Sum`; tests
- [~] Money formatting per currency: `FormatMinor` (plain decimal) done; symbol/grouping/locale = UI layer
- [x] Money parsing: `ParseMinor` (strict decimal → minor units, validation, round-trip) + tests; grouping input later
- [x] ★ `internal/currency`: registry (code, symbol, decimals, name) + `Rates` table type
- [x] ★ `Rates.Convert` / `ToBase` rounding to target minor units (nearest; float-rate caveat noted)
- [x] Missing-rate + non-positive-rate error handling; tests for cross-currency + rounding
- [x] Helper: format a `Money` in a target/base currency for display — `Rates.FormatAccounting` +
      `Rates.FormatInBase` (`internal/currency/format.go`), table-tested

### 1.3 Pure logic services — ★ (each in its own `internal/*` pkg, table-driven tests)

- [x] ★ `internal/id`: stable, collision-safe ID generation (seedable for tests)
- [x] `internal/dateutil`: month boundaries, fiscal-month start, week-start, period ranges
- [x] ★ `internal/ledger`: account balance from opening balance + transactions
- [x] `internal/ledger`: running balance series for an account
- [x] `internal/ledger`: cleared balance (opening + cleared txns) for reconciliation
- [x] `internal/ledger`: income/expense totals for a period (exclude transfers)
- [x] `internal/ledger`: net worth (assets − liabilities) with multi-currency → base
- [x] `internal/ledger`: per-member and group rollups
- [x] `internal/budgeting`: spent vs limit per budget (individual + group scope)
- [x] `internal/budgeting`: near/over-limit threshold evaluation
- [x] `internal/goals`: progress %, remaining, projected completion (read-only estimate)
- [x] ★ `internal/freshness`: per-type staleness windows + `IsStale(balanceAsOf, type, now)`; recurring-bill exemption
- [x] ★ `internal/validate`: per-entity validation (required, positive amounts, valid refs, currency match)
- [x] Tests for every service above (edge cases, multi-currency, rounding, boundaries)

### 1.4 Persistence — `internal/store` (pure-Go in-memory SQLite via `ncruces/go-sqlite3`) ★

- [x] ★ In-memory SQLite store (`NewMemory`) with clean `Load`/`Snapshot` dataset ingress/egress (builds for js/wasm + native)
- [x] Schema + schema-version constant; migration scaffold (in `Import`) + version bump test
- [x] Object store per entity (members, accounts, categories, transactions, budgets, goals, tasks)
- [x] CRUD per entity (create/get/list/update/delete)
- [x] Query helpers: by account, by member, by date range, by category, by status
- [x] Settings store (base currency, FX rates, freshness overrides, prefs, OpenAI key) — `Get/PutSettings`
- [x] ★ Export entire dataset → versioned JSON (entities + settings + custom fields)
- [x] ★ Import dataset from JSON (version-migrate; rejects newer schema)
- [x] ★ Lossless export→import round-trip test
- [x] CSV export for transactions (stable columns)
- [x] CSV import for transactions (header-name column mapping, error rows; UI preview later)
- [x] Sample dataset (`SampleDataset`) + `Wipe` (data layer; UI "load sample"/"wipe" actions later)
- [x] Tests: pure store logic, query helpers, import/export round-trip, migration

### 1.5 Logging — `internal/logging`

- [x] `log/slog` custom `slog.Handler` → `io.Writer` (browser console writer wired in the app)
- [x] In-app ring buffer sink (bounded) for a debug log viewer
- [x] Level config + contextual fields (`slog.With`/`WithGroup`)
- [x] Debug log viewer panel (in the Settings screen, newest-first + Refresh)
- [x] Tests for the handler/ring buffer (pure parts)

### 1.6 State wiring — `internal/appstate`

- [x] `internal/appstate` seam: in-memory store + slog logger, typed read accessors, validated
      write-through (`Put*`/`Delete*`), JSON export/import; `Init`/`Default` for screens
- [x] Boot hydration: `appstate.Init` loads sample data on boot (wired into `app.Run`)
- [x] Single persist path: every write goes through validated `appstate.Put*` → store (+ slog)
- [x] Reactive refresh per screen (`state.UseAtom` revision bumped after `appstate.Put*`) — Accounts add form
- [ ] Derived/computed selectors (net worth, totals, budget health) via `state.UseComputed` — with screens
- [x] Error/toast surface for failed persistence — `uistate.Notice` atom + `app.Toast` (auto-dismiss);
      all screen write sites routed (ledger bulk + paired-transfer delete, Accounts mark-all-updated,
      dashboard nudge reminder)

### 1.7 Design system / UI primitives — `internal/ui`

- [ ] Tokens: colors, spacing, typography scale (extend `web/index.html` styles or a CSS file)
- [ ] Button (variants: primary/secondary/ghost/danger; sizes)
- [ ] Input, NumberInput, MoneyInput (currency-aware), TextArea
- [ ] Select / Dropdown, Combobox
- [ ] Field wrapper (label, hint, error) + form validation pattern
- [ ] Modal / Dialog, ConfirmDialog
- [ ] Toast / notification system
- [ ] Badge, Tag/Chip, ProgressBar, Meter
- [ ] Card, Section, StatCard
- [ ] EmptyState, Skeleton/Loading, ErrorState
- [ ] Table/List with row-component pattern (respect On*-hooks-in-loops)
- [ ] Color picker (members/categories), DatePicker, Icon set
- [ ] Responsive: mobile nav (drawer/hamburger), content widths

### 1.7c Dashboard UI & design system — selected design: `design/candidate-c.html` ★

The chosen visual direction is **candidate C** (flat neutral-dark · Fraunces serif headings + accounting
figures · bento grid · per-widget grip/title/gear · drag-reorder + resize · gear→flip settings ·
collapsible icon sidebar · global-settings flip). The static reference mockup is
[`design/candidate-c.html`](./design/candidate-c.html) (open via the dev server at
`/design/candidate-c.html`). Every item below is a Go/`html/shorthand` component to port from it.
Drag/resize/flip need pointer/drag events via `syscall/js`/`interop`; keep computation in the tested
logic packages, persist layout/settings to the store `Settings`.

**Reusability (required):** build these as generic, props-driven components shared across the whole
app — not per-widget bespoke markup. In particular: one `Widget` shell (grip/title/gear header slots
+ body slot), one `FlipPanel` primitive reused by **both** per-widget and global settings, one
settings-form renderer driven by a field schema, and shared primitives (`Toggle`, `Segmented`,
`StepperPill`, `Swatch`, `Chip`, `ProgressBar`, `Icon` set, and SVG `Chart` helpers). Every widget is
`Widget`-shell + content; every screen composes these. Mark each item below `(reuse)` where a single
component should serve many call sites.

Design tokens & foundation:
- [x] `internal/ui` tokens (mirror mockup `<style>`): palette + radii — Tailwind config + design-system CSS in host page; legacy screens retargeted to match
- [x] Fonts: Fraunces (display headings + figures) + Inter (UI); `.fig` tabular lining figures helper
- [x] Accounting money display in UI (`$` + thousands + 2dp, **negatives in parentheses**, red/green) — `money.FormatAccounting` + `fmtAccounting`/`figTone`
- [x] Dark modern scrollbar styling for the scroll pane (`main.cf-scroll`)

App shell & navigation:
- [x] App shell: fixed left rail + independently scrolling `main`; sticky top bar
- [x] Sidebar rail: brand header; nav items each with an SVG icon — `internal/ui.Icon` + `navItem`
- [x] "My pages" section: example custom pages (+ colored page icons) and a "New page" action
- [x] Collapsible rail: toggle → 58px icon-only mode (shared `rail:collapsed` atom); reload-persist later
- [x] Household card (rail bottom) → opens global settings
- [x] Top bar: menu toggle, page title, time-resolution control, `+ Add`

Time-resolution control (top bar):
- [x] Segmented **Week / Month / Quarter** toggle (`ui.Segmented`)
- [x] **From / To** stepper pills that relabel per resolution; clamp From ≤ To (`period.Window`)
- [x] Drive dashboard period from this control (`uistate` window → `ledger.PeriodTotals`)
- [x] Persist the chosen resolution across reloads (`uistate.PersistResolution` localStorage; re-anchor
      to the current period on load — From/To stepping stays transient by design)

Bento grid system:
- [x] Grid engine: base cell unit `--cell` (152px), equal columns, uniform gap, integer cell spans
- [x] Visible squared cell borders; full-width header cell (1×N)
- [x] Widget shell: unified header — **grip · title · gear** + body (`ui.Widget`)
- [x] Drag-to-reorder / swap widgets (HTML5 DnD), keyed by widget id (`dashlayout.Swap`)
- [x] Resize: right/bottom handles → change col/row span (`dashlayout.Resize`; click-cycle for now, pointer-drag later)
- [~] Persist per-user layout — order + spans saved to `localStorage`; hidden/per-page + store persistence later

Per-widget settings (gear → flip):
- [x] Flip primitive: card lifts to center, dim/blur backdrop, 3D `rotateY` (`ui.FlipPanel`, reused for global)
- [x] Settings back: centered title + right ✕ close; scrollable body; dark Save/Cancel footer
- [~] Settings fields: editable Title + behavior toggles done; accent swatches/default size/refresh/Remove + persistence later

Widget catalog (each backed by tested logic; see mockup):
- [x] KPI tile — Net worth / Income / Spending / Liabilities (figure + subline)
- [x] Recent transactions (table, accounting amounts)
- [x] Budgets (progress bars, ok/near/over) — `internal/budgeting`
- [x] Net worth trend (SVG area chart) — `ledger.NetWorthSeries` + `chart`/`ui.AreaChart`
- [x] Goals (progress) — `internal/goals`
- [x] To-do (task list)
- [x] Accounts (mini balances)
- [x] Cash flow (in/out bar chart per period) — `ledger.PeriodTotals`
- [x] Upcoming bills (from liabilities' due day + min payment)
- [x] Savings rate (figure + bar)
- [x] Spending breakdown (segmented bar + legend by category)
- [~] Reusable SVG chart helpers — area/sparkline (`chart` + `ui.AreaChart`) done; bars are div-based; donut later

Global settings (household card → large flip panel):
- [x] Large centered flip panel (2-column scrollable body), dark Save/Cancel
- [x] Household members (chips + add); Base currency; editable FX rate rows (live reads)
- [x] AI (OpenAI BYO key toggle + key + model); Appearance (theme seg + accent + density) — UI (local state)
- [x] Data: export JSON/CSV, import, load sample, wipe (confirm) — wired via `appstate`

Shared control components (from mockup):
- [x] Switch/toggle, swatch picker, segmented control, stepper pill, member chip, data buttons, dashed "add" button (`internal/ui` + settings)

### 1.8 Members / Household

- [x] List members; add/delete; set default; color; inline edit (name + color)
- [x] Ownership assignment UI (individual vs group) — set at creation everywhere and editable inline
      on accounts, budgets, and goals (shared `ownerSelectOptions` owner picker)
- [~] Member switcher / filter — per-member "Transactions" drill-down filters the ledger by member;
      global cross-screen member scope deferred (ambiguous semantics)
- [x] Member delete: reassign owned accounts/budgets/goals (+ transactions) to another owner via
      `appstate.ReassignOwner` + Members reassign panel, then delete
- [ ] Tests: member logic, ownership rules

### 1.9 Accounts (assets + liabilities) ★

- [x] ★ Accounts list grouped by class (assets / liabilities) with per-account balance
- [x] ★ Add + delete + archive/restore + inline edit account (name, opening balance, type attributes)
- [x] Liability sub-form (credit limit, APR, min payment, due day, lender) — shown for liability types
- [x] Allocation attributes sub-form (expected return, liquidity, stability, lock-until) on add + edit
- [~] Per-account ledger view — account row "Transactions" button filters the ledger to that account
      and navigates; dedicated running-balance view optional later
- [x] "Update balance" action → cleared adjustment txn for the difference + set `BalanceAsOf`
- [~] Credit utilization indicator done (on liability rows); due-date reminder via Upcoming bills widget
- [x] Net-worth summary header (assets, liabilities, net) in base currency
- [x] Per-account staleness indicator (Stale badge) + per-row "Mark updated" + bulk "Mark all updated"
- [ ] Tests already in services; add UI-state tests where logic leaks

### 1.10 Categories

- [x] List + add + delete + inline edit (name + kind); income vs expense
- [x] Sub-categories (parentId): engine + parent picker (add & inline edit) + indented lists +
      dashboard breakdown rolls sub-category spend up to the top-level parent
- [~] Default scheme + reset; methodology-aware presets (envelope/zero-based) — pure
      `internal/catscheme.Default()` (starter income/expense set + sub-categories), table-tested; the
      reset action (apply via appstate) + methodology presets remain
- [x] Reassign transactions on category delete (pick replacement) — `appstate.ReassignCategory` +
      Categories reassign panel (moves transactions and budgets, then deletes)
- [~] Tests: tree building, reassignment — reassignment tested; category tree building N/A (flat list)

### 1.11 Transactions (+ transfers, filters) ★

- [x] ★ Ledger list (newest first); virtualization for large sets later
- [x] ★ Add transaction (desc, amount, income/expense, category, account, date, member)
- [x] ★ Delete + inline edit transaction (desc, amount, category, date; non-transfers; sign preserved)
- [x] ★ Transfers between accounts (paired entries; excluded from income/expense); deleting one leg removes both
- [x] Tags input + tag display (income/expense); search matches tags
- [x] Filters: member, account, category, text, date range + sort (combine + clear) + persist last
      filter across reloads (`uistate.UseTxFilter` localStorage atom)
- [x] Sort options (date, amount, payee)
- [x] Export the filtered/sorted view to CSV (`applyTxFilter` shared with the list)
- [x] Filtered summary line: count + net total (base currency) of the shown set
- [x] Row component for actions; inline edit (incl. category) — `TransactionRow` edit mode
- [x] Bulk select + bulk delete (transfer-aware) + bulk recategorize + bulk mark cleared/uncleared
- [x] Repeat-last helper (pre-fills form from newest txn) + per-row duplicate (copies row to today)
- [x] Cleared/reconciled toggle per transaction + cleared-status filter (both/not/cleared, persisted)
- [x] Tests: filter + sort logic (`internal/txnfilter`, table-tested); signed amounts/transfer pairing in `ledger`

### 1.12 Budgets (individual + group)

- [x] List budgets with spent vs limit + progress bar (current month)
- [x] Add + delete + inline edit budget (name, limit, period) + weekly/monthly/quarterly periods
      (engine `PeriodRange` + selector + per-budget evaluation honoring week-start)
- [x] Near/over-limit indicators (gentle, colored bar) + summary header (spent/budgeted/left)
- [x] Period selector (month stepper) — view any month
- [x] Tests: spent/remaining, scope aggregation, thresholds (in `internal/budgeting`)

### 1.13 Goals

- [x] List with progress bar (% + remaining) + pace guidance + combined progress header (saved/target/%)
- [x] Add + delete + inline edit goal (name, target, target date, linked account) + contribute
- [~] Contribute-to-goal action done (prompt); auto-progress from linked account later
- [x] Tests: progress + projection (in `internal/goals`)

### 1.14 To-do (budgeting tasks)

- [x] List (open/done) with due + priority
- [x] Add + complete-toggle + delete + inline edit (title, priority, due, notes); linking later
- [~] Sort (open first, then due, then title) + hide-done filter done; more filters later
- [x] Create-from-insight (Insights "Save as task" → AI task) + create-from-nudge (freshness
      "Remind me" → nudge task) hooks
- [~] Tests: ordering (pure `internal/tasksort` — Order/Visible, table-tested); status transitions still UI

### 1.15 Freshness & friendly nudges

- [~] Dashboard nudge widget ("N balances could use a refresh") done; dismissible + one-tap update later
- [ ] One-tap "update balance" from nudge
- [ ] Per-account staleness badges
- [ ] Configurable windows in settings; recurring-bill exemption respected
- [ ] Tests already in `internal/freshness`; add dismissal-state tests

### 1.16 Custom fields (extensibility)

- [x] `CustomFieldDef{ID, EntityType, Key, Label, Type, Options, Required}` + store CRUD
      — `customfields.Def` (pure) + `customfielddefs` table CRUD + `CustomFieldDefsByEntity`; appstate accessors
- [x] Validate `custom{}` map against defs for the entity type — `customfields.Validate`, table-tested
- [x] Forms render core + custom fields by type (text/number/date/bool/select) — `CustomFieldInput`
      on all five entity forms (accounts, transactions, budgets, goals, members)
- [x] Custom field management UI (per entity type) — `CustomFieldsManager` on the Customize screen
- [x] Export/import round-trips custom field defs — dataset + Export/Import covered by tests
- [x] Tests: validation (value + Def), round-trip (store CRUD, dataset, export/import), save-path enforcement

### 1.17 Dashboard

- [x] Net worth + per-member/group rollups (Members screen "Net worth by member")
- [~] This-month income/expense (done); balance trend snapshot (later)
- [ ] Budget health summary; next goal; overdue tasks
- [ ] Freshness nudges block
- [x] Recent activity list
- [ ] Placeholder slots for AI insight + formula results (wired P2)

### 1.18 Settings

- [ ] Members management entry
- [ ] Base currency selector + editable FX rate table (add/edit/remove rate)
- [ ] Category management entry
- [x] Freshness window overrides editor — per-type day inputs in Settings writing
      `Settings.FreshnessOverrides`, applied via `appstate.FreshnessWindows`
- [x] OpenAI key + model fields persist to Settings (global panel) — used by Insights
- [ ] Data: export JSON, export CSV, import JSON, import CSV, load sample, wipe (confirm)
- [~] Preferences: theme/density, week-start, fiscal-month start, number/date formats
      — theme (dark/light/system) + accent + density + week-start + date format all complete &
        reload-persistent (engine + atom + Settings UI + `ApplyPrefs` + light/dark skins);
        only fiscal-month start remains
- [ ] Budgeting methodology selector (envelope / zero-based / simple tracking)
- [x] Module visibility toggles (show/hide screens) — end-to-end: pure `internal/modules` +
      localStorage atom + sidebar filter + Settings per-screen toggles, reload-persistent
- [ ] Debug: open log viewer

### 1.19 Configuration & modalities

- [ ] Layered config resolution: defaults → household → member → screen
- [ ] Config persisted + included in export/import
- [ ] Methodology changes adjust UI affordances (e.g. envelope view)
- [ ] Per-member preferences (formatting, default account/member)
- [ ] Tests: config layering/resolution

#### Localization (i18n) — central language store
- [x] Pure `internal/i18n`: dot-namespaced key catalog, `T(lang, key, args…)` with en fallback,
      `MissingKeys` coverage, whole-bundle JSON export/import, English source seed — table-tested
- [x] Live bundle + active-language in `uistate` — shared `i18n.DefaultBundle()`, imported languages
      persisted to localStorage and merged on boot, active language persisted + loaded at boot
      (`ActiveLanguage`/`SetActiveLanguage` reload-applies)
- [x] `T(key, args…)` helper for screens/shell — `uistate.T`, hook-free (safe in loops), resolves
      against the active language
- [x] Language selector in the household settings panel — Settings → Languages "Display language"
      `<select>` over `uistate.Languages()`; switching persists + reloads to re-resolve all strings
- [x] Export/Import language bundle buttons in settings — Settings → Languages; `uistate.ExportLanguages`/
      `ImportLanguages` (merge + persist to localStorage, seeded on boot)
- [x] **Migrate all page verbiage onto `T`** — done across the shell chrome and every screen/component
      (todo, members, categories, goals, budgets, insights, planning, customize, documents, allocate,
      dashboard, settings screen + global panel, accounts, transactions, custom-fields). Intentional
      literals: `humanizeType` account-type names, currency/AI-model display names, date-format
      examples, OpenAI prompt instructions.
- [x] Tests: CI guard for the English catalog (`TestDefaultCatalogQuality`) — every key is dot-namespaced
      with no whitespace and maps to a non-empty string, so a blank/malformed entry fails `go test` in CI
      (ci.yml runs `go test ./...`). Values' trimming/format-verbs intentionally unconstrained (legit
      leading-space suffixes + literal `%`).

### 1.20 Phase 1 hardening

- [ ] Accessibility pass (labels, focus order, keyboard nav, ARIA) via framework a11y — **see B15**
      (app-wide a11y spike + program; this line is subsumed there)
- [ ] Empty/error/loading states on every screen
- [ ] Plain-English copy review (labels, nudges, errors, confirmations)
- [ ] Performance: large dataset (10k+ txns) virtualization + memoization
- [ ] Usage docs + screenshots; update framework notes if APIs learned
- [ ] Phase 1 release via `gwc release`; verify compressed sizes (`gwc wasm measure`)

---

## 2. Phase 2 — Intelligence & power tools (OpenAI, client-side)

### 2.1 OpenAI client — `internal/ai`

- [x] Client over `fetch` with user key from settings; base URL configurable — `ai.SendChat`
- [x] Chat/Responses call with JSON-schema **structured outputs** → Go structs — codec
      (`ai.BuildStructuredRequest`/`BuildStructuredVisionRequest`), transport (`SendStructuredVisionChat`),
      and document image extraction switched onto a strict `transactions` schema. Round-trip tested.
- [~] Vision input support (images/PDF pages) for document parsing — `ai.BuildVisionRequest` (pure) done
- [x] Model selection; token + cost surfacing; "AI off until key set" state — cost surfacing
      (`ai.EstimateCostUSD`/`FormatCostUSD` + `Usage` through the transport, shown in Insights), a
      Settings model picker covering the priced models, and an explicit "AI off until key set" hint.
- [x] Error handling: auth, rate limit, network, CORS — plain-English messages via pure, table-tested
      `ai.ErrorMessage(status, body)` + an HTTP-status check in the fetch transport (network/CORS
      handled in the catch). Inherited by Insights + Documents with no screen changes.
- [x] Retry/backoff; request cancellation — transient failures (429/5xx/network) retry up to 3× with
      exponential backoff (pure, tested `ai.IsRetryable`/`ai.RetryDelayMS`), and the Send* funcs return
      a cancel handle (AbortController + retry-timer clear) wired to a Cancel button in Insights.
- [x] Request build + response decode (pure codec, round-trip tested) — `internal/ai`

### 2.2 Documents — AI import

- [~] Upload UI (CSV paste + image picker) done; PDF + drag-drop later
- [x] Local CSV parse → import transactions (no AI needed) — Documents screen paste-and-import
- [x] Send PDF/image to vision model → structured transactions — `ai.BuildVisionRequest` +
      `ai.SendVisionChat` + `extract.ParseRows` + Documents image-import UI (choose → read → review → import)
- [x] `Document{ID, Filename, Kind, UploadedAt, AccountID, MemberID, Status, Extracted[]}` lifecycle —
      end-to-end: model + persistence (`domain.Document`, `documents` table, store CRUD, dataset
      round-trip, validated appstate accessors, table-tested), CSV/image imports record a `DocImported`
      document (image carries the rows), and an **Import history** card on the Documents screen lists
      and removes them.
- [x] Review screen: list + per-row edit + per-row remove → import to ledger (account-pick) +
      dedupe vs existing (skip same date+amount in account, reported)
- [ ] Monthly-spend extraction summary view
- [~] Tests: CSV parsing (store) + extraction parsing/dedupe (`extract`) done; extraction→txn mapping is UI

### 2.3 Insights & NL query

- [x] "Explain my month" generated narrative (Insights screen)
- [~] Natural-language query over data → answer (Insights "Ask about your money"); richer data context later
- [x] Trend/anomaly highlights — done end-to-end: pure engine (`internal/insights.Detect`,
      current-vs-trailing-average w/ noise floor + threshold, explainable + table-tested), feeder
      (`ledger.CategorySpendSeries`, FX-aware, table-tested), and the offline **Spending highlights**
      card in Insights (`screens.spendingHighlights`, last 4 months, plain-English ↑/↓ rows, no AI key).
      AI-generated "advice cards" remain a separate later enhancement.
- [x] Pin/save insights; show top insight on dashboard — top insight on dashboard
      (`screens.topHighlightWidget`); pinned-insight storage (`domain.SavedInsight` + store/state,
      table-tested) **and** a Pin button + "Pinned insights" list (remove) on the Insights screen.
- [x] Guardrails: scope data sent — insights prompts build from a pure `ai.FinancialContext` that by
      construction carries only aggregates (no payees/account numbers/per-transaction detail), so the
      privacy scope is explicit and reviewable in one place.
- [x] Tests: prompt assembly, data-context selection (pure parts) — `ai.FinancialContext.Line()` is
      table-tested (incl. a no-leak assertion).

### 2.4 Auto-categorization & Rules

- [x] `Rule{ID, Match, SetCategoryID, SetTags}` store + management UI — store/state (`rules` table +
      CRUD, dataset export/import, validated `appstate.Rules/PutRule/DeleteRule`, table-tested) **and**
      the `/rules` management screen (add/list/inline-edit/delete + nav entry, localized validation).
- [x] Rule matching engine (pure) + tests — `internal/rules` (Category/Tags/FirstMatch)
- [x] Rule-based category suggestion on entry — saved rules (priority) + implicit category-name match
      auto-fill category **and tags** as you type the description, never overriding a manual choice
- [x] Rules from history (review + accept) — pure deterministic suggester (`internal/rulesuggest`,
      payee/desc → consistent category, support-ranked, skips covered keys; table-tested) **and** a
      "Suggested rules" card on the Rules screen with one-click Add (accepted rules drop off the list).
      (Optional later: AI proposals for fuzzier patterns beyond exact payee/desc keys.)
- [x] Conflict handling — `rules.Conflicts` flags rules that never fire (shadowed by an earlier
      substring-matching rule, or empty-phrase), surfaced as a per-row warning on the Rules screen.
      Table-tested.
- [x] Apply rules on import/entry — entry (add form) + image import run rows through `autoRules`
      (first-match-wins, explicit category beats inferred), and a retroactive `appstate.ApplyRules`
      with an "Apply to existing" button on the Rules screen covers the CSV-paste path and any
      pre-existing uncategorized transactions. (Conflict handling beyond first-match is still open.)

### 2.5 Formula builder + sandboxed engine — `internal/formula`

- [x] ★ Tokenizer (numbers, strings, idents, operators, parens, commas) — `internal/formula.Tokenize`
- [x] ★ Parser → AST (precedence, unary, function calls) — `internal/formula.Parse`
- [x] ★ Evaluator with allow-list functions (`sum/avg/min/max/count/if/round/abs`) + arithmetic/compare — `internal/formula.Eval`
- [~] Variable resolution: live figures (net worth/income/expense/counts) done via `Env`; custom fields + filtered aggregates later
- [~] Typed results (number/bool/text) done; money/percent typing + formatting later
- [x] `Formula{ID, Name, Expr, Enabled}` store + CRUD — model + persistence (`domain.Formula`,
      `formulas` table, store CRUD, dataset round-trip, validated appstate accessors; table-tested)
      **and** save/list/edit/delete UI on Customize (live result per saved formula). Target/ResultType/
      Format deferred.
- [~] Builder UI: live preview + error messages + example chips done (Customize); guided insert later
- [ ] Surface results on dashboard / relevant entities
- [x] ★ Extensive tests: tokenizer, parser, evaluator, errors, security (no escape), edge cases —
      `eval_security_test.go` (sandbox rejects host/non-allowlisted fns, scalar-only results, unknown
      vars error, deep nesting, determinism, numeric edge cases, malformed→error) + existing token/
      parser/eval tests.

### 2.6 Planning + Forecast

- [x] `Recurring{Label, Amount, Cadence, NextDue, AccountID, CategoryID, Autopost}` + CRUD — model +
      persistence (`domain.Recurring` w/ `Cadence.Next`/`Advance`/`MonthlyEquivalent`, store CRUD,
      dataset round-trip, validated appstate accessors; table-tested), a "Recurring cash flows" card on
      Planning (add w/ account/category/autopost, list, delete, net-monthly total), and **autoposting**
      due ones into transactions (`appstate.PostDueRecurring` + "Post due now" button; table-tested).
      Optional later: feed recurring into the forecast (needs a no-double-count design vs. actuals).
- [ ] `Plan{ID, Name, HorizonMonths, BaseScenario, Assumptions[]}` + `PlanItem{...}` + CRUD
- [~] ★ Forecast engine (pure): `internal/forecast.Project` over horizon from start + recurring + one-time items done; actuals-derived recurring later
- [x] Debt payoff math (`internal/payoff.Project`) + tests + extra-payment scenario (months/interest saved)
- [~] What-if scenarios: extra debt payment + trim-spending forecast done; add-recurring/rate-change later
- [ ] Planning screen: build scenario, compare vs actuals, push to forecast
- [~] Forecast visualization (net-worth curve) done on Planning; scenario comparison later
- [x] ★ Tests: forecast projection, payoff math — forecast (recurring/one-time/flat + out-of-horizon
      ignored, same-month sum, negative horizon, negative balances) and payoff (zero/interest payoff,
      payment-too-small, single-month clear, payment==interest boundary, negative balance, TotalPaid
      invariant). Scenario application (extra-payment/trim what-ifs) is exercised via the Planning UI.

### 2.7 Capital-allocation engine — `internal/allocate`

- [~] ★ Criterion scorers: returns, stability, liquidity, debt reduction done (`internal/allocate`); goal-progress criterion later
- [x] ★ Weighted combination by profile; normalization; deterministic (`Score`/`Rank`)
- [x] `AllocationProfile{ID, Name, Weights}` + CRUD — model + persistence (`domain.AllocationProfile`,
      `allocprofiles` table, store CRUD, dataset round-trip, validated appstate accessors; table-tested)
      **and** an Allocate picker with editable criterion weights, preset/saved-profile loading, and
      save/delete. Constraints/CustomCriteria on the profile are a later extension.
- [~] Constraints: emergency buffer, max-per-destination, exclusions — applied/clamped
      — exclusions complete (engine + UI); emergency buffer + max-per-destination implemented in the
        `Distribute` split engine (tested); amount-split UI next
- [x] Candidate set assembly (asset accounts + high-interest liabilities + unfinished goals)
- [x] Ranked output with per-criterion breakdown (no black box)
- [x] Allocate screen: profile select → ranked suggestions + exclude/restore + amount-split input
      (amount + emergency buffer → per-destination dollar amounts via `Distribute`, with kept-back note)
- [x] Optional AI narrative ("Explain with AI" on the Allocate screen)
- [x] ★ Extensive tests: scoring, weighting, constraints, determinism — scoring/weights/constraints/
      Distribute (proportional/reserve/cap/even/edge) plus explicit determinism (Rank+Distribute stable
      across runs), tie-stability, and breakdown clamping. (Custom-criteria scoring lands with the
      formula-backed criteria.)

---

## 3. Phase 3 — Sync & PWA

### 3.1 Sync server (Go)

- [ ] HTTP service sharing client domain structs
- [ ] Household account/auth model
- [ ] Endpoints: pull deltas, push deltas, full snapshot, health
- [ ] Conflict resolution strategy (last-write-wins + vector/seq) + tests
- [ ] Storage backend (sqlite/file) for the household dataset

### 3.2 Client sync

- [ ] Sync client in wasm app; background sync + status UI
- [ ] Offline mutation queue + replay
- [ ] Settings toggle + endpoint/credentials
- [ ] End-to-end sync tests

### 3.3 PWA / offline

- [~] Web manifest done (`manifest.webmanifest` + theme-color/apple meta); icons later
- [x] Service worker (`sw.js`): network-first cache of shell + assets, offline fallback
- [~] Installability prompt done (beforeinstallprompt button); offline read works (sw); update flow later
- [ ] Verify via framework `pwa` package

---

## 4. Cross-cutting (continuous)

- [ ] Keep logic packages pure + table-driven tested as features land
- [ ] One feature per commit; CHANGELOG + DEVLOG updated every commit
- [ ] Grow the design system rather than one-off styles
- [ ] Accessibility + plain-English copy on every new screen
- [ ] Keep `docs/GOWEBCOMPONENTS.md`, `CLAUDE.md`, `SPEC.md`, `TODOS.md` current
- [ ] CI green (tests + wasm build) before merge
- [ ] Periodic bundle-size check (`gwc wasm measure`)
- [ ] Security review before any data leaves the device (AI calls): scope + redaction

---

## 5. Future / nice-to-have (post-core)

Lower-priority items to pick up **only after the core product (Phases 0–3) is complete**. These are
enhancements, not part of the core spec; sequence them after the Phase 3 / sync work.

### 5.1 Standalone desktop app via Electron

Wrap the existing WASM/PWA build as a native, installable desktop app (Windows/macOS/Linux) so
CashFlux can be distributed and launched outside the browser while reusing the exact same Go→wasm
bundle and `web/` shell. Local-first; no behavior change — just a native window + installer.

- [ ] Decide the wrapper: Electron shell loading the existing `web/` build (vs. evaluate a lighter
      alternative like Tauri/Wails) — record the choice and trade-offs in DEVLOG
- [ ] Electron scaffold: `main` process that serves/loads `index.html` + `bin/main.wasm` +
      `wasm_exec.js` + `sw.js` + `manifest` (correct MIME for `.wasm`; relative asset paths)
- [ ] Reuse the production `web/` build as the renderer payload — no separate UI codebase; keep the
      wasm bundle the single source of truth
- [ ] App window chrome: title, icon, sensible default size, native menu (minimal)
- [ ] Packaging/installers per OS (e.g. `electron-builder`): Windows installer, macOS `.dmg`, Linux
      AppImage/deb
- [ ] Build script / CI job to produce the desktop artifacts from the same wasm build (don't hand-copy)
- [ ] Verify: app installs and launches natively, loads offline, and matches the PWA behavior

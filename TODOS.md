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
- [x] Model: `internal/dashlayout/pack.go` — ordered `Item` sequence + `Pack` (first-fit, no overlap,
      honors spans, clamps oversized), table-tested.
- [x] Ops: `Move(id, toIndex)` (reorder → re-`Pack`) replaces `Swap`; `ResizeItem` + re-`Pack`. Tested.
- [x] State: persist the ordered `[]Item` (`uistate.PersistItems`); the old `[]Placement` localStorage
      migrates for free (unmarshal into `Item` ignores col/row).
- [x] UI wired: `widget.go` renders via `Pack(items,4)` (header row offset), drag-drop calls `Move` +
      reflow, resize calls `ResizeItem` + reflow. Verified in-browser: default arrangement pixel-identical.
- [ ] UI polish (deferred): live drag-over reflow PREVIEW (currently reflows on drop, not during drag);
      prefer pointer events over HTML5 DnD for touch.
- [ ] **Animate reorder**: tiles that shift during a reflow move smoothly, iOS-home-screen style.
      CSS-grid placement changes don't transition natively → use a FLIP technique (measure old/new
      rects, transform from old→new, transition the transform to zero) keyed by widget id.
- [ ] **Animate resize**: growing/shrinking a tile's span scales smoothly rather than snapping
      (transition the cell, FLIP the neighbors that reflow around it). Pairs with the reorder FLIP.
- [x] **Resize handles only while holding Shift**: `.rz` hidden by default, revealed when the root has
      `data-resize` (toggled by a global Shift keydown/keyup listener + window-blur clear in
      `internal/app/resizereveal.go`), with an opacity fade. Keeps the bento visually calm.
- [~] Verify: multi-cell tiles never overlap + resize re-packs — **done** (Pack model + render verified
      in-browser). Still TODO: smooth FLIP animations and a live drag-over preview (reflow is on drop).

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
- [x] **Drag reorder.** DONE — the primary nav items are drag-reorderable: drop one onto another to
      move it, persisted to localStorage (`cashflux:nav-order`) via a new pure `internal/navorder` helper
      (`Move`/`Apply`, table-tested) + `uistate.UseNavOrder`/`PersistNavOrder`; `Apply` layers the saved
      order over the live, hidden-filtered list (new screens append, hidden ones drop). Each item is its
      own `navItem` component so the drag hooks stay stable. Verified live: dragging Accounts onto
      Dashboard reorders to `[Accounts, Dashboard, …]` and persists. **Deviation:** implemented
      *always-draggable* rather than Shift-gated — click still navigates (separate event); Shift-gating
      would need a reactive shift-held atom (the resize-reveal uses a non-reactive DOM attribute), left as
      a later refinement.
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
- [x] `Window`: `IsSinglePeriod()` + `Single()` (collapse to one unit) + a combined `Label()` that
      collapses to one unit label when single, else "from – to". Table-tested.
- [x] UI: `ResolutionControl` rebuilt — single-period stepper (pages the window, one label), a
      "This period" reset that shows only when off the current period, a "Custom range" toggle that
      reveals the dual From/To steppers (collapsing back on exit), and a **"Jump to…" presets dropdown**
      (This/Last period, This quarter, Year to date). Resolution still persists.
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
- [x] Register feasible schemas for the other widgets incrementally — done across the board:
      recent-transactions count, trend range, breakdown top-N, to-do tasks-to-show, accounts (count +
      cleared-balance toggle), budgets (count + at-risk-only), **goals (nearest-completion + show-date)**.
      Every widget with feasible settings now exposes them.
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
- [x] A quick-add overlay exists: a `uistate.UseQuickAdd()` bool atom + `app.QuickAddHost` render a
      `ui.FlipPanel`. (Implemented as its own atom/host rather than a "kind" on the settings atom, to
      keep the two concerns separate.)
- [~] Back face: instead of a menu of cards, it goes straight to the **New transaction** flow inline
      (account / expense-income / amount / description / category / date → `PutTransaction`, toast).
      Still TODO if a menu is wanted: **Scan a bill** / **Scan a document** (Documents import) /
      **Custom workflow** cards.
- [x] Repoint the "+ Add" button (`TopBar`) to open the panel instead of `nav.Navigate("/transactions")`.
- [~] Keyboard-accessible, labelled, light/dark — inherits FlipPanel's chrome and the focus-visible
      rings; a `role="dialog"`/`aria-modal`/focus-trap pass is tracked under the dialogs a11y item.
- [x] Verify: "+ Add" flips open the panel; saving logs the transaction and toasts; ✕/Cancel closes.
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
- [x] `internal/icon`: `Name` + curated constants (16 icons) + inner SVG markup (lifted from the
      hand-rolled set), `Inner`/`Valid`/`All`; table-tested (every constant resolves to non-empty
      inner-only markup, unknowns invalid/empty, `All` sorted). Pure, no `syscall/js`. Kept existing
      names (not Lucide ids) so the rewire stays mechanical and glyphs identical.
- [ ] Generator/script to fetch Lucide SVGs for the set and write the Go file (documented, repeatable).
- [x] Rewire `ui.Icon` to take `icon.Name`; migrated all call sites (railItem/navItemProps Icon fields
      + the settings/menu icons). Renders the same typed shapes (no framework raw-SVG inject primitive
      exists to consume `icon.Inner()` strings — kept for a future Lucide-string renderer). Glyphs
      identical; the stringly-typed unknown-name path is gone (compile-checked).
- [x] Verify: all existing icons render identically (typed shapes unchanged); unknown-name path is now
      a build error, not a blank SVG. wasm + native suite green.
- [ ] Optional refinement: a generator to fetch real Lucide path data for the curated set (current
      glyphs are already Lucide-format stroked SVGs, so this is polish, not a blocker).

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
- [x] `internal/chartspec`: the typed spec (`Kind`/`Point`/`Series`/`Axis`/`Spec`) + `Validate`
      (sentinel errors) + `Extent` (min/max with ok flag); table tests. Pure, no `syscall/js`. Built
      decision-independent — drives either a D3 or pure-SVG renderer.
- _Decision still open (renderer):_ D3 (JS dep + offline cache + vdom portal) vs. keep pure-Go SVG.
      `ui.Chart` waits on this; the chartspec foundation is useful either way.
- [x] JS shim `web/chart.js` `cashfluxRenderChart(el, specJSON)` building line/area/bar/donut with D3
      (theme-aware via CSS vars); pinned D3 v7.9.0 in index.html; both added to the SW CORE cache (v3).
      chartspec JSON-tagged. (D3 render needs an in-browser check.)
- [x] `ui.Chart` (`internal/ui/chartd3.go`): managed container (stable `UseId`) + `UseEffect` keyed on
      the serialized spec that calls `cashfluxRenderChart`; clears on unmount; theme-aware via the shim.
      `role="img"`+label. (D3 render needs a browser check.)
- [x] Migrated the dashboard **net-worth trend** widget to `ui.Chart` (Area spec) as the proof; the
      pure-SVG `AreaChart` still renders the planning forecast + plan sparklines (kept until parity).
- [ ] **Verify in a browser** (the one step I can't do here): the D3 trend chart renders + updates on
      data change, survives hot-reload, works offline (SW cache), and matches light/dark. Then migrate
      the remaining charts and retire `AreaChart`.

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
- [~] **Semantics & landmarks:** sidebar `<nav>` labelled "Main navigation"; `<main id=main tabindex=-1>`
      + a **skip-to-content** link; the top bar's page title is now the screen's single `<h1>` (dashboard
      in-canvas header demoted to `<h2>`). Still TODO: `banner`/`contentinfo` roles.
- [~] **Keyboard:** the div-based **toggle switch** and **accent swatches** are now focusable + operable
      (tabindex=0 + Space/Enter via the framework's `OnKeyDown`; focus ring via the existing
      `:focus-visible`). Segmented = real buttons (already operable). Still pointer-only: **the bento
      drag/resize** (needs an arrow-key move/resize alternative) and the Shift+drag nav reorder (B8);
      inline-edit rows' focus-on-enter/exit also still TODO.
- [x] **Dialogs (`FlipPanel`, the B11 add panel, confirms):** `role="dialog"` + `aria-modal="true"` +
      an accessible label, **Esc to close**, a **focus trap** (Tab/Shift+Tab cycle within), **initial
      focus** into the dialog, and **focus restore** to the trigger on close — all done in one shared
      effect covering every overlay.
- [~] **Custom controls → correct ARIA:** Segmented = `role="radiogroup"`/`role="radio"`/`aria-checked`;
      Toggle/ToggleRow = `role="switch"` + `aria-checked` + name; StepperPill ‹/› have `aria-label`s;
      SwatchPicker = labelled `role="radiogroup"` of `role="radio"` chips. Still TODO: the gear/menu/grip
      icon-buttons' `aria-label`s, and real keyboard operability for the div-based Toggle/Swatch.
- [x] **Focus visibility:** a global `:focus-visible` ring (accent, 2px offset) on every interactive
      element/role in both themes.
- [~] **Screen-reader / live regions:** the toast notice is now a persistent live region (idle region
      stays mounted; errors are `assertive`/`role=alert`, normal notices polite) so async outcomes are
      announced. Still TODO: announce filtered-result counts / balance updates inline; associate form
      errors via `aria-describedby`; mark required fields.
- [x] **Color is never the only cue:** audited every color-coded state. Budget bars carry
      "On track/Near limit/Over budget" text, net-worth/highlights use ▲/▼ arrows, stale accounts show
      a "Stale" badge, cleared shows a ✓; the one offender — the To-do widget's priority dots (high vs
      medium were both `●`) — now uses distinct shapes ▲/●/○ plus accessible names.
- [~] **Contrast:** built `internal/contrast` (table-tested) and **audited** the tokens with it.
      Fixed: `text-faint` now meets AA-normal (4.5) on both surfaces in both themes (dark→#888890,
      light→#686870). The appearance settings now **show the selected accent's contrast ratio** vs the
      theme surface and warn when it's low (uses `internal/contrast`) — so users see when an accent is
      hard to read. Still TODO (brand decision): pick a light-theme-safe default accent (or per-theme
      accent) since the default green is ~2.3:1 on light.
- [~] **Motion:** `prefers-reduced-motion` now also covers the flip-panel, toast slide-in, and rail
      width (boot + rail flyout were already handled). Still TODO once they exist: the dashboard
      reorder/resize animations (B2). Original note below:
- [ ] (orig) **Motion:** `prefers-reduced-motion` for the dashboard reorder/resize animations (B2) and the
      flip panel (boot loader + settle already do).
- [ ] **Zoom / reflow:** usable at 200% browser zoom and with enlarged browser font sizes — the
      px-heavy styling (see B6) is the risk; pairs with the B6 UI-scale work.
- [~] **Forms:** correct input types (number/date) in use; **inline validation is announced** —
      every `.err` message is `role="alert"`; **required fields carry `aria-required`** across every add
      form (accounts, categories, budgets, goals, members, rules, to-do, transactions, quick-add,
      plans). Still TODO: per-field `aria-describedby` association tying each error to its input.
- [x] **Route changes (SPA):** focus moves to `<main>` on navigation (skips the initial load so the
      first Tab still reaches the skip link) and `document.title` is set to "<Screen> · CashFlux".
- [~] **Charts:** `ui.AreaChart` is now `role="img"` + `aria-label` with a live-figure summary (net-worth
      trend, forecast). Remaining: the div-based bar charts + any future D3 charts (B14).
- [~] **Touch targets:** small icon-only buttons (delete/toast-x/rstep/set-close) now meet the WCAG
      2.5.8 AA 24×24 minimum (centered glyph). 44×44 (AAA) left aspirational given the dense desktop UI.
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

## C. Live UI/UX review findings — 2026-06-16 (sample data) ★

Captured by driving the running app (`http://127.0.0.1:8080`) in a real headless Chromium via the
now-installed Playwright driver and screenshotting all 14 routes (Dashboard, Accounts, Transactions,
Budgets, Goals, To-do, Planning, Allocate, Insights, Documents, Customize, Members, Categories,
Rules). Screenshots + rendered text are in `.review-screenshots/` (git-ignore this). Items are
ordered correctness-first, then cross-cutting chrome, then per-screen polish.

### C1. Dashboard "Income" shows $0.00 despite a $4,200 salary in-period ★ (correctness)
**Symptom:** with sample data, the Dashboard Income KPI reads **$0.00 · 0 deposits** for Jun 2026,
but `tx-1` Salary (+$4,200, income, cleared, **2026-06-01**) is clearly in June. Spending ($1,800.75,
3 txns on Jun 2/3/5) is correct.
**Root cause (verified in code):** `period.Range`/`Truncate` compute the window start with
`dateutil.MonthStart(t)` which **preserves the browser-local timezone** (`t.Location()`), while sample
transaction dates are stored at **UTC midnight** (`time.Date(2026,6,1,0,0,0,0,time.UTC)`). In any
timezone *behind* UTC, the local month-start (e.g. `Jun 1 00:00 −05:00` = `Jun 1 05:00Z`) falls
*after* the `Jun 1 00:00Z` salary, so `dateutil.InRange` (`!Before(start) && Before(end)`) drops it.
Jun 2–5 expenses survive because they're a day later. This silently drops any first-of-period,
UTC-dated transaction.
- [x] Canonical convention chosen: **dates are timezone-free calendar dates stored at UTC midnight**
      (`ParseDate` already parses in UTC; transaction input round-trips through it). The boundary builders
      in `dateutil` (`midnight`/`MonthStart`/`FiscalMonthRange`/`NextMonthlyDue`, and `WeekStart` via
      `midnight`) plus `period.quarterStart` now take the calendar date from the reference instant but
      emit the boundary at **UTC midnight**, so windows compare cleanly against UTC-dated transactions.
- [x] Table test `TestPeriodBoundariesAreUTCRegardlessOfZone`: a `00:00Z` first-of-month transaction is
      counted for "now" evaluated in UTC-5, UTC-11, and UTC+13 zones (was dropped behind UTC).
- [x] Verified live: Dashboard Income shows **$4,200.00** (was $0.00); KPIs read net $20,749.25 / income
      $4,200.00 / expense $1,800.75 / liabilities $850.00.

### C2. Money formatting is inconsistent across screens ★ (correctness/polish)
The CLAUDE.md standard is accounting format — thousands separators + **parentheses** for negatives
(`money.FormatAccounting`). It's applied on the **Dashboard** and the **Transactions list summary**
(`$20,749.25`, `($1,500.00)`) but bypassed elsewhere, producing ugly/locale-naive output:
- [x] **Grouping** — fixed in one place: `fmtMoney` now formats with thousands grouping, so
      Accounts/Budgets/Goals/Allocate/etc. show `$20,749.25` not `$20749.25`.
- [x] **Negative style** — unified. `fmtMoney` now renders accounting-style (parentheses + grouping),
      identical to the old `fmtAccounting`, so Transactions rows now show `($60.20)` like the Dashboard.
      The two formatters were collapsed into one canonical `fmtMoney`. Confirmed safe: `fmtMoney` is
      display-only (no `Value(fmtMoney(...))` anywhere — inputs use `money.FormatMinor`/`ParseMinor`), so
      parentheses can't reach an editable value. Verified live (Dashboard figures unchanged).

### C3. "Your household" card (rail bottom) is visually broken on every page ★
**Symptom:** the bottom-left household card overlaps and clips its own text — the avatar bubble (which
oddly reads **"GWC"**, not the member's initial) sits on top of "Your household", and the second line
shows cut-off fragments ("…ember · USD base ·" / "…tings"). Present on all 14 screens.
- [x] Resolved by the redesign: the card is now a flex Button (gear icon + two text lines), no avatar
      bubble and no overlap/clipping (the "GWC" avatar was from the old mockup). Tidied the summary to
      drop the redundant "· Settings". (Re-verified in current code.)

### C4. Global top-bar chrome appears on screens where it's meaningless ★ (UX)
The **time-resolution control** (Week/Month/Quarter + Jump-to + ‹Jun 2026› + Custom range) and the
**+ Add** button render on *every* route, including ones with no period concept — Members, Categories,
Rules, Customize, Allocate, Documents, To-do, Goals. A period stepper on Categories does nothing.
- [x] The resolution control now shows only on period-aware screens (Dashboard, Transactions, Budgets,
      Planning, Insights), gated by a `periodAware` set on the current route in `TopBar`.
- [x] "+ Add" left visible everywhere — logging a transaction is a valid action on any screen, so it has
      an obvious target (no-op by that reasoning).

### C5. Dashboard ships a duplicate "Net worth" widget (default layout)
The default bento has the **Net worth KPI** (`$20,749.25 ▼7% this month`) *and* a second standalone
**Net worth** tile (`$20,749.25`) lower in the grid — redundant out of the box.
- [x] Differentiated rather than removed: the lower tile is the net-worth **trend chart** and is now
      titled "Net worth trend" (distinct from the net-worth KPI), so they no longer read as duplicates.

### C6. Allocate criterion weights are five unlabeled "1" inputs ★ (UX)
Under "CRITERION WEIGHTS" there are five number fields all defaulting to `1` with **no labels**, so
you can't tell which is returns / stability / liquidity / debt-reduction / goal-progress.
- [x] Each weight input is labelled (Title + Placeholder: "Returns weight" … "Goal-progress weight").
- [x] Zero-score candidates (no allocation attributes set) are now hidden from the ranked list (and the
      amount split); when that empties the list, a hint prompts setting expected return / stability /
      liquidity on the accounts.

### C7. Budgets — duplicate "Food · Food" label + double period control
- [x] The budget row reads **"Food · Food"** — fixed: `BudgetRow` now shows one label when name ==
      category (case-insensitive), just the category when unnamed, and "name · category" only when they
      differ.
- [x] Consolidated to one control. The Budgets card's own `‹ January 2006 ›` month stepper is removed;
      the screen now reads the viewed period from the shared top-bar resolution control
      (`uistate.UsePeriod().Get().From`), so there's a single period control and one format. (Dropped the
      now-unused `monthOffset` state + `dateutil`/`time` imports.)

### C8. Members — color picker renders as a bare line
**Symptom:** the Add-member form's color field shows only a thin horizontal line between Name and the
Add button — no visible swatch/label, looks broken.
- [x] Fixed: the native color input now uses a dedicated `.color-input` class (renders as a proper
      clickable swatch, not a bare line) with a "Member color" label, in both the Add and Edit forms.
      Kept the native picker (full color choice) over the fixed-palette SwatchPicker.

### C9. Smaller polish
- [x] **Accounts** add/edit row: shortened the asset placeholders ("Return %"/"Liquidity"/"Stability")
      and added full-label `title`s with the range, so they no longer clip. (The `.form-grid` already
      wraps the ~9 inputs.)
- [ ] **Accounts** rows expose 6 actions each (Transactions / Update balance / Mark updated / Edit /
      Archive / ✕) — visually busy; consider an overflow menu for secondary actions.
- [x] **Goals** add form's current-amount field — already labelled with a "Saved so far" placeholder
      (stale report; verified in current code).
- [x] **Categories** now show their color (an 11px swatch on each row) and let you set it — the Add
      form and inline Edit row have a color picker; `Color` is threaded through `saveCat`/`OnSave`.
- [x] **Insights** without a key: the offline Spending-highlights card already showed; now the "Ask
      about your money" box is also always visible (disabled preview + key hint when no key), so the
      screen advertises its features instead of looking bare.

### C10. No responsive / mobile layout at all ★ (UX, severe)
**Symptom (verified at 390×844):** on every screen the left rail stays full-width and fixed, the
content is pushed off-screen to the right, and the page scrolls **horizontally** to reach it. The
dashboard bento keeps its desktop cell size so tiles are clipped; forms (Add account/transaction) run
off the right edge. The app is effectively unusable on a phone.
- [x] Below 768px the rail collapses to its icon-only width and the main content takes full width (CSS
      `@media (max-width:767px)`). The `.form-grid`/`.stat-grid` already auto-fit.
- [x] Bento reflows to a single column on narrow screens (tiles' inline grid placement overridden).
- [x] Verified at 390px in a headless browser: no horizontal scroll (scrollWidth == clientWidth), rail
      56px. Follow-ups: a slide-in drawer (vs icon rail) and the top-bar resolution control's mobile
      overflow.

### C11. Widget gear opens an empty "Save"-able panel for widgets with no settings (UX)
**Symptom:** clicking the gear on a no-schema widget (e.g. Net worth) opens the flip panel reading
"This widget doesn't have any settings yet." — yet it still shows a **Save** button, implying there's
something to save.
- [x] Replaced Cancel/Save with a single **Close** when a widget's settings panel is empty —
      `FlipPanel.CloseOnly` set via `!widgetcfg.Has(id)` in `SettingsHost`. (C11)

### C12. Settings panel: "Display scale" row is clipped by the footer
**Symptom:** in the global Settings flip panel, the last Appearance row ("Display scale") is cut off
where the two-column scrollable body meets the sticky Cancel/Save footer (label renders as "Display
sale"). The body doesn't scroll far enough to clear the footer.
- [x] Added bottom padding to `.set-body` (1rem → 1.5rem) so the last row clears the sticky footer fold
      and scrolls fully into view.

### C13. Quick-add panel is transaction-only with large empty space (UX) — DONE (height)
The "+ Add" flip panel jumped straight to a tall "Add a transaction" form with lots of unused vertical
space and no other add actions. **Fixed (panel height):** the panel is now sized to its content (420px,
body scrolls if it overflows) so it no longer floats in a tall empty card — verified live in a headless
browser (opens at 420px on "+ Add"). The *additional add-actions* part (scan bill / scan document /
custom workflow cards) remains tracked as the open part of **B11**.

### C14. Dashboard grid resize is broken in practice + can't shrink ★ (UX — this grid templates custom pages)
**Reported:** Shift+click resize "doesn't work" and there's no simple way to shrink a tile. Confirmed
empirically by driving the live app (held Shift, clicked the right edge handle, watched the inline
style):
- **It fires once, then self-destructs.** The first Shift+click correctly grew `kpi-networth` from
  `grid-area: 2 / 1` to `2 / 1 / auto / span 2`. But the layout is **absolute placement with no
  packing** (`internal/dashlayout` Default + `Resize` set spans without reflow), so the now-2-wide tile
  **overlaps** `kpi-income` (col 2, row 2). The overlapping neighbor paints over the resize handle, so
  the **second** click is intercepted (Playwright click times out). To the user this looks like "resize
  doesn't work." This is the same root cause as **B2** (absolute placement + pairwise `Swap`/`Resize`,
  no `Pack`).
- **No way to shrink — by design.** Each handle (`internal/ui/widget.go` `.rz` `OnClick`) only
  *increments* the span and *wraps* at the max (`maxColSpan 4`, `maxRowSpan 3`): a 2-wide tile shrinks
  only via `3 → 4 → 1`. No drag, no shrink handle; the handle tooltips say only "Widen"/"Taller", so the
  wrap-to-shrink is invisible.
- **Poor affordance/discoverability.** Resize needs *holding Shift* while clicking a 3px bar inside an
  11px edge strip, with no on-screen hint that Shift is the trigger. It's click-to-cycle, not the drag
  gesture users expect (`widget.go` comment: "click-cycle for now, pointer-drag later").
- **Silent no-op for off-layout widgets.** `Resize`/`Swap` return the layout unchanged for any id not in
  the layout (`indexOf == -1`), and the grid then falls back to the props defaults — so a widget whose
  id isn't in `Default()`/the persisted layout can't be resized or reordered at all (relevant once this
  grid backs custom pages with arbitrary widget ids).
**Fix:** this is the B2 work — do **B2 first** (ordered sequence + pure `Pack` bin-packing so spans
never overlap + `Move`/re-pack + pointer drag-resize with an explicit shrink direction + FLIP animation)
before reusing this grid as the custom-pages template. Add table tests that a grow never produces
overlapping cells and that shrink is reachable in one gesture.

### C15. Collapsed rail loses all navigation ★ (bug)
**Symptom (verified live):** clicking the top-bar menu toggle collapses the rail to 58px, but it then
shows only the "C" brand mark and the active item's highlight box — **no nav icons at all**, so you
can't navigate while collapsed (and B5's hover-flyout has nothing to reveal).
**Likely cause:** `web/index.html` `aside.rail.collapsed nav > div { display:none }` (intended to hide
the "TOOLS"/"SYSTEM" section headers) also hides every nav item, because the framework wraps each
`uic.CreateElement(navItem, …)` output in a `<div>` — so `nav > div` matches the items too.
- [x] Scoped the rule to the section-header element: `railHeader` now carries a `rail-section` class and
      the collapsed rule targets `nav .rail-section` instead of `nav > div`, so the wrapped nav items stay
      visible. Applied the same scoping to the `<768px` mobile rail (it had the identical bug). Verified
      live: collapsed rail (58px) shows all 14 nav icons with both section headers hidden. SW cache v4→v5.

### C16. Net-worth trend chart plots cents — Y-axis is wrong & unreadable ★ (correctness)
**Symptom (verified live):** the Net worth trend chart's Y-axis labels read "000,000 / 500,000 /
000,000 / 500,000" — non-monotonic and clipped.
**Root cause (verified in code):** `dashboard.go:459` feeds `Y: float64(m.Amount)` — the **raw minor
units (cents)** — into the chart spec, so the axis ticks are cent values (2,000,000 / 1,500,000 / …)
truncated to 7 chars in the narrow widget. The figure above the chart is correct ($20,749.25) only
because it uses `fmtAccounting`.
- [x] Convert minor units to major units before plotting, and format axis ticks as compact currency.
      `dashboard.go` now divides by the currency's decimal factor and sets `Y.Format` (`$.2~s`); the D3
      shim (`web/chart.js`) honors the per-axis `format` hint. Y-axis now reads `$0 / $5k / $10k / $15k /
      $20k` — verified live in a headless browser. SW cache bumped v3→v4. Audited the other chart feeds:
      `customize.go` already plots major units; the planning `AreaChart` is an axis-less sparkline
      (normalized path, no numeric labels) so it was never affected. No other cents-vs-dollars feed.

### C17. Custom range shows a redundant "Jun 2026 – Jun 2026" (live confirmation of B10 #2)
**Symptom:** toggling "Custom range" reveals two steppers that read "Jun 2026 – Jun 2026" when From==To
— the exact redundancy called out in **B10**. (Logged here as a live repro; fix under B10's
single-period collapse.)

### C18. Inline-edit layout is inconsistent across screens (UX)
**Symptom (verified live):** **Budgets** inline-edit lays its fields out horizontally, matching the Add
form — good. But **Transactions** and **Accounts** inline-edit stack every field vertically in a narrow
left-hand column (very tall, lots of empty space to the right), looking unfinished and inconsistent.
- [x] Done. The Transactions/Accounts edit forms were already `form-grid` but wrapped in the flex `.row`,
      which shrink-wrapped them to a single column. Wrapped them in a new full-width `.row-edit` block
      instead (Budgets already used a block, `.budget`), so the grid expands like the Add form. Verified
      in-browser: `form-grid` yields 3 columns at 600px in `.row-edit` vs 1 in `.row`. SW cache v5→v6.

### C19. Responsive breakage specifics (extends C10) ★
Captured at 768px (tablet):
- [x] **Top-bar controls overflow off-screen:** fixed. At ≤1024px the top bar now grows to two rows —
      breadcrumb on row 1, the control cluster (resolution segmented + jump + stepper + custom range +
      Add) wraps onto a full-width row below (`flex: 1 0 100%`, `height:auto !important` to beat
      Tailwind's `h-14`, and the resolution control wraps internally). Verified live: at 768px the bar is
      ~175px with the breadcrumb readable (96px) and no control past the viewport; at 390px all controls
      are reachable too. No horizontal page scroll at either width. SW cache v6→v7.
- [x] **Transaction rows break:** fixed. `.row` now wraps (`flex-wrap: wrap`) at ≤1024px, so `.row-main`
      (flex:1) takes the first line and the action buttons (Mark cleared / Edit / Duplicate / ✕) flow
      underneath instead of overlapping. A no-op on rows that still fit. Shared by every list screen.
      Verified the mechanism by injecting a representative row at 360px: it wraps (height ~204px) with 0
      of 5 buttons overlapping the text. SW cache v9→v10.
- [x] **KPI tile figures clip** (e.g. "$20,749.2", "$1,800.7$") when the bento is squeezed — fixed.
      Between the phone breakpoint and the desktop, the 4-column bento squeezed tiles to ~153px and a
      figure clipped. Added a tablet bento (`768–1024px`) that flows the tiles into **2 columns** (header
      kept full-width via `:first-child`). Verified live at 900px: 0 clipped figures, KPI tiles ~315px
      (was 153), header full-width, no horizontal page scroll. SW cache v8→v9.
- _Good:_ the Add/filter `form-grid`s do reflow to two columns cleanly — the pattern works; the rail,
  top bar, bento, and list rows are the parts that don't. (Pairs with C10.)

### C20. Collapsible side panel reads as "missing" — toggle is misplaced and collapse is broken ★
**Reported:** no collapsible left panel and no toggle button. **Reality (verified):** a menu-toggle
button *does* exist (28×28, with the `icon.Menu` glyph) and clicking it collapses the rail — but:
- [ ] The toggle lives in the **top bar** (inside the scrolling main pane, ~x=260), not **on the
      panel** where a collapse control is conventionally expected — so it doesn't read as "the panel's
      collapse button." Add an on-panel collapse affordance (e.g. a chevron at the rail's edge/footer).
      _(Remaining — a placement/design choice; the working top-bar toggle stands in the meantime.)_
- [x] Collapsing **empties the rail** — resolved by **C15** (the collapsed rail now keeps its nav icons).
- [x] Persist the collapsed state across reloads: moved the atom into `uistate.UseRailCollapsed()` seeded
      from localStorage, with `PersistRailCollapsed` written on every toggle (mirrors the resolution-pref
      pattern). Verified live: toggling writes `cashflux:rail-collapsed` = `1`/`0` and the rail goes
      58px↔240px. (The load path matches the proven `loadResolution` pattern; the oracle uses a fresh
      profile per launch so cross-reload couldn't be exercised end-to-end there.)
- [~] Verify: collapse → usable icon rail (C15 ✓) and persists (✓). An on-panel toggle is the open part.

### C21. Per-tile dashboard settings are incomplete + the gear is easy to miss ★
**Reported:** per-tile settings don't exist. **Reality (verified):** the gear *does* open real,
persisted settings for **8 widgets** (savings, recent, trend, breakdown, todo, accounts, budgets,
goals). But:
- [x] Chose "hide the gear where there's nothing to configure": `ui.widget` now renders the gear only
      when `widgetcfg.Has(id)` (or an explicit `OnGear`); no-schema tiles get an inert, equal-width slot
      so the header stays balanced. The empty "no settings yet" panel is no longer reachable from a gear.
- [x] Strengthened the affordance: the gear brightens on `.w:hover/:focus-within` (with a color
      transition), so it's discoverable on configurable tiles without being loud.
- [x] Verified live: 16 tiles → 8 real gears (recent, budgets, goals, todo, accounts, trend, savings,
      breakdown) and 8 inert slots (the 4 KPIs + cashflow/bills/freshness); the net-worth KPI gear is a
      hidden span, not a button. SW cache v7→v8.

### C22. Layout engine does not reflow on move or on resize ★ (= B2 / C14, with fresh evidence)
**Reported:** moving tiles doesn't reflow; scaling tiles up/down doesn't reflow. **Verified live:**
dragging `kpi-income` onto `kpi-liabilities` changed only those two tiles' `grid-area` (income→`2/4`,
liabilities→`2/3`) — **no other tile moved**, and the result even mis-placed a tile (not a clean swap).
Resize overlaps neighbors (C14). Root cause: absolute placement + pairwise `Swap`/`Resize`, no packing.
- [x] Resolved by the **B2 / C14** Pack migration. `internal/ui/widget.go` now renders via
      `dashlayout.Pack`, drag-drop calls `dashlayout.Move` (reorder → re-Pack), and resize calls
      `dashlayout.ResizeItem` (re-Pack) — there is no `Swap` anywhere. So moving a tile reflows the rest
      and growing/shrinking re-packs without overlap, by construction (the Pack/Move/ResizeItem ops are
      table-tested for no-overlap, and the default arrangement was verified pixel-identical in-browser).
- [x] Shrink is reachable: the resize handles cycle the span and wrap at the max back to 1 (so the
      "wrap" is how you shrink); tooltips say "cycles 1→4" / "1→3".
- [~] Verify: move/resize reflow is structural (✓, via Pack + the unit tests + the pixel-identical render
      check). The only open piece is the live drag-over **preview** (reflow currently lands on drop, not
      during the drag) — tracked as the remaining B2 UI-polish item, not a correctness gap.

### C23. No way to add data beyond a single transaction ★
**Reported:** no way to add new data. **Reality:** the top-bar "+ Add" opens a quick-add **transaction**
form only; every other entity (account, budget, goal, category, member, rule, recurring, plan) can be
added **only** by navigating to its own screen — there's no global/dashboard add affordance for them.
- [x] Turn "+ Add" into a real add menu (the open part of **B11**): the new `app.AddMenu` component makes
      "+ Add" a popover — New transaction (inline quick-add) · New account · New budget · New goal · Scan
      a document — the entity items route to their screen via the router. Always-rendered + CSS-toggled so
      the On* hooks stay stable. Verified live: opens with 5 items, "New transaction" opens the quick-add
      panel, the menu closes on select. SW cache v10→v11.
- [x] Per-widget "add" affordances on the dashboard — DONE. A reusable `emptyAddCTA` component renders an
      empty Accounts / Goals / Budgets / To-do tile's empty state with an in-context "Add a …" button that
      routes to the relevant screen via `router.Navigate`. The Budgets tile distinguishes genuinely-empty
      (no budgets → CTA) from "nothing near/over the at-risk filter" (no CTA). Verified the navigation
      mechanism live (nav → /goals renders the Goals screen); the sample data populates all tiles so the
      CTA isn't shown by default, but the empty branch + nav are confirmed.
- [x] Verify: from the dashboard alone a user can create each core entity type — the menu reaches
      transaction/account/budget/goal/document from anywhere. (Category/member/rule are still reachable
      via their screens; could be added to the menu later if wanted.)

### C24. Proposal: auto-layout engine with two modalities (importance vs default) ★ (design)
**Request:** an optional auto-layout with two modes — (1) **user-defined importance sorting** and (2) a
**default sort order** — so tiles arrange themselves instead of being hand-placed. Analysis + plan:
- [x] **Confirmed with user (2026-06-17):** importance is set **per-tile via the gear panel**; tile
      **size stays user-set** (auto-layout only reorders, never resizes). Resolves the C21 tension by
      making importance a universal per-tile setting, so a gear panel is never empty (the gear can show
      on every tile in importance mode without reintroducing C21's empty panel).
- [x] **Model + tests (done):** pure `dashlayout.Arrange(items, mode) []Item` reorders by `Mode`, then
      the existing `Pack` derives positions (no manual Col/Row). `Item` gained an additive `Importance`
      field. Custom = no-op; Auto-default = canonical `DefaultItems` order; Auto-importance = importance
      desc, canonical-order tiebreak. Table-tested: determinism, stability, no-overlap-after-Pack, no
      input mutation, spans preserved, unknown ids sort last.
- [x] **State (done):** `uistate.UseLayoutMode()` / `PersistLayoutMode` / `loadLayoutMode` persist the
      mode (default Custom). A manual drag bakes the current arrangement into the sequence and flips to
      Custom.
- [x] **UI — selector + render (done):** `ui.widget` applies `Arrange(items, mode)` before `Pack`; the
      dashboard header has a Custom · Auto: default · Auto: importance selector (switching to Custom bakes
      the current auto order so tiles don't jump). Verified live: selector persists, dashboard re-renders.
- [x] **UI — importance editing (done):** the per-tile gear panel now has an Importance control
      (Highest/High/Normal/Low → 2/1/0/−1) that writes the layout items via `dashlayout.SetImportance`
      (pure + tested). The gear shows on **every** tile while in Auto-importance mode; importance is a
      universal setting, so the panel is never empty (respects C21). End-to-end verified live: ranking
      the bottom freshness tile "Highest" moved it from grid-row 8 → row 2, and persisted. **C24 done.**
- [x] _Decision to confirm with user:_ resolved above (per-tile gear; size user-set). (per the
      "agree the spec first" rule).

### C25. Default UI is too "fat/chunky" — tighten the density tokens ★ (UX)
**Reported:** the UI (incl. the Add-transaction modal) feels too fat/chunky on every screen.
**Measured live (1440px, scale 100%) — concrete weights:**
- body **16px** / 24px line-height (Tailwind default; heavy for a dense finance app)
- form `.field` inputs **40px tall**, 16px text, 8×9.6px padding
- buttons up to **~60px tall** (12px padding); primary actions read oversized
- widget `.wbody` padding **~15×16px**; widget title 16px; nav items **36px** tall
- the "+ Add" → **Add a transaction** modal body is ~360px but its fields use only ~150px — large dead
  space below the form (also **C13**)
**Analysis:** chunkiness is global because it comes from shared tokens (base font, `.field`, button,
`.wbody` padding), so adjusting the tokens fixes all 14 screens + modals at once. Two existing levers
already exist but don't fix the *default*: the **Compact density** toggle and the **Display scale**
zoom (**B6**) — the complaint is that the out-of-the-box weight is too high.
- [x] Lowered the **default** base font to **14.5px** (from 16) with line-height 1.45; the Fraunces
      display figures keep their explicit sizes, so the data accents stay prominent.
- [x] Tightened `.field` (padding 0.5/0.6→0.4/0.55rem, radius 8→6px — now ~34px tall), `.btn` (0.55/0.9→
      0.4/0.8rem, radius 8→6px), and `.wbody` (0.85→0.7rem) padding.
- [x] The quick-add flip panel already sizes to its content (C13); the dead space is gone there.
- [~] Re-check at the new density: verified live on the dashboard + the quick-add form — body 14.5px,
      fields 34px with no text clipping, KPI figures still fit (0 clipped). The other screens are
      route-gated in the static oracle but use the same shared tokens, so the effect is uniform; nothing
      reduced below the existing **24px** B15 touch-target minimum (fields 34px, buttons ~30px).
- [x] _Decision (made, per the user's "just pick" steer):_ **rebalanced the default density down** rather
      than shipping new Cozy/Compact presets — simpler and lower-risk, and the existing Compact toggle +
      Display scale remain as further levers on top.

### C26. Make text size configurable for low-vision users ★ (accessibility)
**Reported:** font size should be configurable for visually impaired folks. **Current state:** B6 added
a **Display scale** (70–130%) implemented as a whole-UI **`zoom`** on `#app`. That helps but isn't a
true text-resize control: it tops out at 130%, scales layout (not just text), and `zoom` can break the
non-responsive layout (**C10**) at large values.
- [x] Raised the scale range to **200%** (`prefs.ScaleMax` 130→200; table test updated so 200 is valid
      and 250 clamps to 200). Now covers WCAG 2.1 SC 1.4.4.
- [x] **Chose option (b):** keep the `zoom`-on-`#app` mechanism — now viable because **C10/C19
      responsiveness is fixed**. Verified empirically: at `--ui-scale: 2` on a 1280px window the page
      reflows to the effective ~640px width (phone rules engage) with **no horizontal scroll**
      (`bodyScrollW == viewport`). The rem-migration (option a) is not needed for the accessibility goal.
- [x] Composes with C25 density: density rebalances the base tokens; this scale is a `zoom` multiplier on
      `#app` on top — independent, so Compact + 150% (etc.) stack.
- [x] Persists with the other prefs (already wired). Relabeled the control **"Text & display size"** so
      it reads as an accessibility control. (A live sample preview is a nice-to-have, not done.)
- [~] Verify at 200%: confirmed no horizontal scroll / reflow on the dashboard (root). The other 13
      screens are route-gated in the static oracle, but they share the same responsive rules + zoom
      mechanism, so the reflow behavior is uniform.

### C27. AI features — live test results with a real OpenAI key (2026-06-17)
Tested by driving the app with the key from `.env` (entered in Settings → AI; key persists on input).
Direct browser→`api.openai.com` calls **succeed — no CORS problem** (all returned HTTP 200 on
`gpt-4o-mini`). **Working well:**
- [x] **Insights → Explain my month** — returns a coherent narrative from live figures.
- [x] **Insights → Ask about your money** — answered using **income $4,200 / spending $1,800.75**, with
      token + cost surfaced ("Used 166 tokens · about $0.0001"). _Note: the AI context computes income
      **$4,200 correctly**, while the Dashboard Income KPI shows **$0** — independent corroboration that
      **C1** is a dashboard/period bug, not a data problem._
- [x] **Allocate → Explain with AI** — returns a sensible ranking rationale.
- [x] **Documents → Read with AI (vision)** — read the test receipt accurately: Coffee −4.50, Sandwich
      −8.25, Cookie −2.00 (2026-06-10), review screen + monthly summary "out $14.75 · net ($14.75)".

**Not working / rough edges found (fix):**
- [x] **CSV import rejects the documented format** ★ — FIXED. `TransactionsFromCSV(data, defaultCurrency)`
      now defaults the currency to the household base when the column is absent (only amount stays
      required), and reads `account`/`category`/`member` (the documented friendly names) as well as the
      export's `*_id` headers — appstate resolves any **names** to ids case-insensitively. The UI strips
      the `store:` prefix from import errors. `documents.csvDesc` updated (currency optional, names or IDs).
      New table tests: default-currency + friendly columns, and id-wins-over-name.
- [x] **Vision category names don't match the app's categories** — improved with a fuzzy fallback in the
      import: after the exact (case-insensitive) name match fails, accept a substring match either way
      ("Food & Drink" ↔ "Food", min length 3, categories scanned in order for determinism) before falling
      through to the auto-rules. Handles the reported near-name case. (Constraining the vision prompt to
      the user's category list, or a per-row picker, remain as possible further hardening.)
- [x] **Review-row amounts use a minus sign** — FIXED. The draft review rows now format the amount
      through `fmtMoney` (the unified accounting formatter, parentheses for negatives) in the chosen import
      account's currency (falling back to base), with a raw-string fallback while the value is unparseable.
      Matches the rest of the app (C2). The summary line already used `fmtMoney`.
- [x] Harden the AI key flow — DONE. (1) **Local dataset persistence (2026-06-18):** the dataset
      autosaves to localStorage and hydrates on boot, so data survives a reload; the OpenAI key is
      **redacted** from that autosave (stays session-only). (2) **Opt-in key persistence (2026-06-18):**
      a `prefs.RememberAIKey` toggle in Settings → AI ("Remember my key on this device", off by default).
      When on, the key is written to its own `cashflux:openai-key` entry and restored on boot
      (`hydrateAIKey`); when off, it's cleared. Secure-by-default, with a plain-English unencrypted-storage
      note. Verified live: toggling on persists the key, off clears it.
- [x] **Insights "Save as task"** — verified: the AI answer becomes a To-do (body carries the full
      answer; savings-rate math 2399.25/4200 = 57.14% correct). **Rough edge FIXED:** the task title is
      now the **question** (for a Q&A) or a short generic label ("Money insight") for "Explain my month",
      with the full answer kept in the notes — no more whole-sentence titles.
- [x] **Insights "Pin"** — verified: pins to the "Pinned insights" list.
- [x] Re-confirmed on re-test (2026-06-17): OpenAI calls 200, vision works. The **CSV documented-format
      failure is now fixed** (see the currency-default + name-resolution item above).
- [ ] Not yet exercised (queue for the browser E2E lane): cancel/abort mid-call, retry/backoff on
      429/5xx, and the error message shown on a bad/empty key.

---

## D. Cross-component E2E workstream stories — budgeting · planning · finances ★

Each story below is a **workstream**: one real user journey followed end-to-end, asserting that every
component it crosses stays correct *and* coherent — the persisted data, the derived figures, and the
UX all agree. Unlike B16 (per-feature happy paths), these are organized by **concept** and deliberately
**span components** so a change in one place is proven not to break the figures somewhere else.

**How to run:** browser E2E needs the Playwright lane (§0 — the driver is now installed locally, so
`gwc probe` / the screenshot harness in `.review-screenshots/` can drive these manually today; wire
`gwc test -lane browser` into CI when ready). For every *computational* assertion, also add/strengthen
a pure table-driven test in the owning logic package (`ledger`/`budgeting`/`forecast`/… ) so the
invariant is guarded without a browser. **Discipline:** when a story surfaces a defect, file it (or
extend a B/C item) and check the story's "fix:" box only when both the unit test and the journey pass.
Known live findings are cross-linked inline.

### Budgeting workstreams

#### D1. Paycheck → spend → budget → dashboard, one period ★
**Workstream:** add an income paycheck and a couple of category expenses, then confirm they flow into
the ledger, the period totals, the matching budget's spent/left, the savings-rate widget, and the
dashboard KPIs — all scoped to the same period.
**Touches:** Transactions · `ledger.PeriodTotals` · `budgeting` · Budgets · Dashboard (Income/Spending/
Savings KPIs) · `period.Window`.
- [ ] Add an income txn dated the **1st** of the current month; assert Dashboard Income KPI rises by it
      and the deposit count increments. **(Currently fails — C1 timezone boundary drops day-1 income.)**
- [ ] Add a Food expense; assert Budgets "spent" rises, "left" falls, and the threshold tone updates.
- [ ] Assert the same expense shows in the Dashboard Spending KPI and the savings-rate recomputes.
- [ ] Switch resolution Week→Month→Quarter; assert the budget window and all KPIs re-window together.
- [ ] Reload; assert every figure persists and still agrees across screens.
- [ ] fix: any cross-screen disagreement — correct it in the shared `ledger`/`period` path, not per screen.

#### D2. Budget near/over-limit lifecycle ★
**Workstream:** drive a budget from under → near → over and back, watching indicators everywhere.
**Touches:** `budgeting` (threshold eval) · Budgets (bar + summary) · Dashboard Budgets widget · a11y (color-not-only).
- [ ] Add spend to cross the "near" threshold; assert bar tone + "Near limit" text on Budgets and widget.
- [ ] Cross "over"; assert "Over budget" text + tone, and the summary "left" goes negative correctly.
- [ ] Delete/adjust the txn back under; assert all indicators revert.
- [ ] Assert the state is conveyed by **text + shape**, not color alone (B15 color-cue rule).
- [x] unit: `budgeting` threshold table test covers exact boundary (==limit, ==near%) values — added
      `TestClassifyBoundaries`/`TestClassifyZeroLimit`/`TestPercentBoundaries` (12 cases): `==limit` is
      Over, `==near%` is Near, one-cent-below is OK, plus the zero-limit guards (no divide-by-zero).

#### D3. Category reassign-on-delete ripples into budgets & ledger ★
**Workstream:** delete a category that has both transactions and a budget; reassign to a replacement.
**Touches:** Categories · `appstate.ReassignCategory` · Transactions · Budgets · store · Dashboard breakdown.
- [ ] Delete a used category, pick a replacement; assert all its transactions move to the replacement.
- [ ] Assert the budget on the deleted category moves/points to the replacement (no orphan budget).
- [ ] Assert spending breakdown + budget "spent" recompute against the new category.
- [ ] Reload; assert no dangling `CategoryID` anywhere and totals unchanged.
- [ ] fix: any orphaned reference; unit test `ReassignCategory` for txns **and** budgets.

#### D4. Individual vs group budget scope aggregation
**Workstream:** create one individual (member-owned) and one group budget on the same category; verify scope-correct spend.
**Touches:** Members · Budgets (scope/owner) · `budgeting` (scope aggregation) · `ledger` per-member rollup.
- [ ] Add expenses by different members; assert the individual budget counts only its owner's spend.
- [ ] Assert the group budget counts the household's spend.
- [ ] Edit a budget's owner inline; assert spend recomputes for the new scope.
- [ ] unit: `budgeting` scope-aggregation test (individual vs group, mixed members).

#### D5. Sub-category rollup into parent budget & breakdown
**Workstream:** add a sub-category under a parent, spend on the sub, and confirm rollup.
**Touches:** Categories (parentId tree) · `categorytree` · Dashboard breakdown (rolls sub→parent) · Budgets.
- [ ] Create a sub-category; add spend on it; assert the dashboard **breakdown** rolls it up to the
      parent. _(The spending-breakdown widget rollup is still pending; the budget rollup below is done.)_
- [x] **Parent-category budget includes sub-category spend** — DONE. New `categorytree.Descendants`
      (rootID + all nested ids, cycle-safe) feeds a new `budgeting.EvaluateRollup` (the budget counts
      spend in its category or any descendant, still respecting period + owner scope). Both the Budgets
      screen and the dashboard Budgets widget now evaluate with rollup.
- [x] Reassign the sub's parent; rollup follows — `Descendants` recomputes from the live `ParentID`, so a
      reparented sub-category rolls up under its new parent (covered by `TestDescendantsReparent`).
- [x] unit: `categorytree` rollup test (multi-level, reparent) — `TestDescendantsMultiLevel`/
      `TestDescendantsReparent`/`TestDescendantsEdgeCases` + `budgeting` `TestEvaluateRollup*` (3 cases:
      descendants counted, empty covers = own category, scope respected).

#### D6. Budget methodology selector (envelope / zero-based / simple) — gap
**Workstream:** pick a methodology and confirm the UI affordances and presets adapt.
**Touches:** Settings (methodology — **not yet built**, §1.18/1.19) · Categories presets (`catscheme`) · Budgets.
- [x] **Methodology selector + persisted config built.** `budgeting.Methodology` (simple/zero-based/
      envelope) + `ParseMethodology`/`ToAssign` (pure, table-tested); `store.Settings.BudgetMethodology`
      (household config, persists with the dataset); a Settings → household selector (Simple · Zero-based).
- [x] Apply "zero-based": the Budgets screen surfaces an "assign every dollar" banner — income for the
      month minus total budgeted ("$X left to assign" / "Every dollar is assigned" / "Over-assigned by
      $X"). Verified live: switching to zero-based and visiting Budgets shows "$3,600.00 left to assign".
- [ ] Apply "envelope": envelope-style carry-forward view — deferred. `MethodEnvelope` exists in the type
      but isn't offered in the selector yet (behaves like simple); needs the carry-forward computation.
- [ ] unit: config-layering test (defaults→household→member). Methodology is household-only today; the
      per-member layering is a future refinement.

#### D7. Month-boundary rollover correctness ★
**Workstream:** step the period across a month/quarter/week boundary with transactions on the edges.
**Touches:** `period.Window`/`Range`/`Truncate` · `dateutil` · `ledger.PeriodTotals` · Budgets · Dashboard.
- [x] Place txns on the **first** and **last** day of a month; assert each lands in exactly one period
      (no drop, no double-count) — `ledger.TestPeriodTotalsMonthBoundary` (May 31 / Jun 1 / Jun 30 / Jul
      1 across three consecutive windows; their sum equals every amount once).
- [x] Repeat for week (honoring week-start) and quarter boundaries —
      `TestPeriodTotalsWeekBoundary`/`TestPeriodTotalsQuarterBoundary` (half-open window: start day in,
      next start day out).
- [x] fix: single UTC-calendar-date convention across `period`/`dateutil`/`ledger` — done in C1, with
      `dateutil.TestPeriodBoundariesAreUTCRegardlessOfZone` exercising membership under non-UTC zones.

### Planning workstreams

#### D8. Recurring cash flow → autopost → ledger → forecast (no double-count) ★
**Workstream:** add a recurring bill + paycheck, autopost the due ones, and project the forecast.
**Touches:** Planning (Recurring) · `domain.Recurring.Cadence` · `appstate.PostDueRecurring` · Transactions · `forecast.Project` · Dashboard.
- [ ] Add a monthly recurring expense + income; assert net-monthly total is correct.
- [ ] "Post due now"; assert exactly the due occurrences become transactions (none missed/duplicated).
- [ ] Assert the forecast projects from start + recurring **without double-counting** already-posted actuals.
- [ ] Advance the period and re-post; assert idempotence (no duplicate posts for the same due date).
- [ ] unit: `Cadence.Next/Advance` + a forecast-vs-actuals no-double-count test.

#### D9. Debt payoff scenario → allocate → balances ★
**Workstream:** model a credit-card payoff, then allocate extra cash toward it and watch the liability fall.
**Touches:** Planning (`payoff.Project`) · Accounts (liability, APR, min payment) · Allocate (`allocate` debt scorer + `Distribute`) · `ledger` net worth.
- [ ] Enter balance/APR/min payment; assert months-to-clear + total interest match `payoff`.
- [ ] Add an extra payment; assert months & interest saved recompute.
- [ ] On Allocate, assert the card ranks high under the debt-reduction criterion and `Distribute` honors
      the emergency buffer + max-per-destination.
- [ ] Post a payment; assert the liability balance and net worth update consistently.
- [ ] unit: `payoff` boundary tests (payment==interest, payoff month) + `allocate.Distribute` reserve/cap.

#### D10. What-if trim-spending → forecast curve vs actuals
**Workstream:** apply a "trim monthly spending by X" what-if and compare the projected net-worth curve.
**Touches:** Planning (trim what-if) · `forecast` · `ledger.NetWorthSeries` · chart (`ui.Chart`).
- [ ] Enter a trim amount; assert the projected end balance shifts by the right delta.
- [ ] Assert the chart redraws and the axis is **in dollars, not cents** (**C16**).
- [ ] Compare scenario vs actual baseline side by side (build the comparison — §2.6 gap).
- [ ] unit: `forecast.Project` with a spending delta over the horizon.

#### D11. Plan (start balance + monthly) projection → dashboard surfacing
**Workstream:** create a savings/spending plan and see its projection.
**Touches:** Planning (`planning.Project`/`EndBalance`) · store (`plans`) · Dashboard (formula/plan slot — §1.17 gap).
- [ ] Create a plan (name/horizon/start/monthly); assert projected end balance matches `planning.EndBalance`.
- [ ] Add a one-time item in a future month; assert the curve bends at that month.
- [ ] Reload; assert the plan persists and re-projects identically.
- [ ] unit: `planning.Project`/`MonthlyNet`/`EndBalance` with one-time items.

#### D12. Goal pace → linked-account contributions → allocate
**Workstream:** create a goal linked to an account, contribute, and see pace + allocation interplay.
**Touches:** Goals (`goals` pace/projection) · Accounts (linked) · Allocate (goal-progress criterion) · Dashboard goal widget.
- [ ] Create a goal with a target date + linked account; assert monthly-needed + projected completion.
- [ ] Contribute; assert progress %, remaining, and the dashboard goal widget update.
- [ ] On Allocate, assert "Finish goals" preset feeds `GoalProgress` and ranks the goal sensibly.
- [ ] unit: `goals.MonthlyNeeded`/projection + allocate goal-progress scorer.

#### D13. Net-worth forecast horizon correctness ★
**Workstream:** project net worth over the horizon from recurring + one-time items and validate edges.
**Touches:** `forecast.Project` · `ledger.NetWorthSeries` · Planning chart · Dashboard trend widget.
- [ ] Assert out-of-horizon items are ignored; same-month items sum; negative balances allowed.
- [ ] Assert the dashboard trend widget and the planning curve agree for overlapping months.
- [ ] Assert chart values are dollars (**C16**) and labels are readable at the widget's width.
- [ ] unit: `forecast` horizon/edge tests (already partial — extend for net-worth feed).

### Finances workstreams

#### D14. Transfer between accounts (paired, excluded from totals) ★
**Workstream:** transfer money between two accounts and confirm it's balance-neutral to income/expense.
**Touches:** Transactions (transfer) · `domain.IsTransfer` · `ledger` (Balance, PeriodTotals exclude transfers) · Dashboard · net worth.
- [ ] Create a transfer; assert both account balances move and net worth is unchanged.
- [ ] Assert Income/Spending KPIs and budgets are **not** affected by the transfer.
- [ ] Delete one leg; assert the paired leg is removed too.
- [ ] unit: `ledger.PeriodTotals`/Balance transfer-exclusion + paired-delete.

#### D15. Reconciliation: clear → cleared balance → update-balance adjustment ★
**Workstream:** clear transactions, reconcile against a real balance, and let the app post an adjustment.
**Touches:** Transactions (cleared toggle + filter) · `ledger.ClearedBalance` · Accounts ("Update balance") · `freshness` (BalanceAsOf).
- [ ] Toggle cleared on several txns; assert cleared balance = opening + cleared only.
- [ ] Use "Update balance" with a different real balance; assert a cleared adjustment txn for the diff is
      created and `BalanceAsOf` is set.
- [ ] Assert the staleness badge clears after the update (ties D17).
- [ ] unit: `ledger.ClearedBalance` + adjustment-amount math.

#### D16. Multi-currency FX across every aggregate ★
**Workstream:** add a foreign-currency account + txns and confirm base-currency conversion everywhere.
**Touches:** Settings (base currency + FX rates) · `currency.Rates.Convert/ToBase` · `ledger` (net worth, totals) · Budgets · `forecast` · displays.
- [ ] Add a non-base account + foreign txns; assert net worth, period totals, and budgets convert to base.
- [ ] Edit an FX rate; assert every aggregate re-converts live.
- [ ] Assert a missing/zero rate surfaces a clear error, not a silent wrong total.
- [ ] Assert rounding is to target minor units and is stable (no drift on re-render).
- [ ] unit: `currency` cross-rate + rounding + missing-rate tests (extend existing).

#### D17. Staleness → nudge → task ★
**Workstream:** let an account go stale, get nudged, and turn the nudge into a to-do.
**Touches:** `freshness.IsStale` · Accounts (Stale badge, Mark updated) · Dashboard freshness widget · To-do (create-from-nudge).
- [ ] Age a balance past its window; assert the Stale badge + dashboard "N balances need a refresh".
- [ ] "Remind me"; assert a nudge task is created in To-do.
- [ ] "Mark updated" / update balance; assert staleness clears and the nudge count drops.
- [ ] Assert recurring-bill exemption is respected.
- [ ] unit: `freshness.IsStale` windows + exemption; **1.15** dismissal-state test (gap).

#### D18. Net-worth assembly across members & group ★
**Workstream:** mix individual and shared assets/liabilities and verify the net-worth breakdown.
**Touches:** Accounts (scope/owner/class) · `ledger.NetWorth` + per-member/group rollups · Members ("Net worth by member") · Dashboard.
- [ ] Assert net worth = assets − liabilities in base currency, matching the Accounts header and KPI.
- [ ] Assert per-member rollup sums to the household total (individual + group).
- [ ] Archive an account; assert it drops out of net worth but is restorable.
- [ ] unit: `ledger.NetWorth` + rollup tests (multi-member, multi-currency, archived).

#### D19. Member add/reassign/delete ripples ★
**Workstream:** add a member, reassign ownership, then delete a member with owned entities.
**Touches:** Members · `appstate.ReassignOwner` · Accounts/Budgets/Goals/Transactions (owner) · net worth rollups.
- [ ] Add a member + set default; assert default-member behavior in new forms.
- [ ] Reassign owned accounts/budgets/goals/txns to another owner; assert all move.
- [ ] Delete the member; assert no orphaned `OwnerID`/`MemberID` and rollups recompute.
- [ ] unit: `ReassignOwner` across all four entity types.

#### D20. Rules auto-categorize on entry & import ★
**Workstream:** define rules, then add/import transactions and confirm category/tags are applied (and conflicts handled).
**Touches:** Rules (`rules` engine, conflicts) · `rulesuggest` · Transactions (entry auto-fill) · Documents (import) · `appstate.ApplyRules` · Budgets/breakdown impact.
- [ ] Add a rule; type a matching description; assert category + tags auto-fill without overriding a manual pick.
- [ ] Import a CSV/image; assert rows are categorized by first-match rule; assert budget/breakdown reflect it.
- [ ] "Apply to existing"; assert pre-existing uncategorized txns get categorized.
- [ ] Assert a shadowed/never-fires rule shows the conflict warning.
- [ ] unit: `rules.FirstMatch`/`Conflicts` + `ApplyRules` retroactive path.

#### D21. Document import → review → dedupe → ledger → derived figures ★
**Workstream:** import via CSV and via image (vision), review, dedupe, import to ledger, and verify downstream.
**Touches:** Documents (CSV + image) · `extract.ParseRows` · `ai` vision codec · dedupe · store (`documents`) · Transactions · Dashboard/Budgets/net worth · `spendsummary`.
- [ ] Paste a CSV with a header; assert rows map by column name and import to the chosen account.
- [ ] Import the same rows again; assert same-date+amount dedupe skips them and reports the count.
- [ ] (Image path, key set) assert vision extraction → review edits → import; assert an Import-history entry.
- [ ] Assert imported txns update Spending KPI, budgets, and the monthly-spend summary.
- [ ] unit: `extract` parsing/dedupe + CSV column mapping.

#### D22. Custom fields + formula over live figures
**Workstream:** define a custom field, fill it on an entity, and reference live figures in a saved formula.
**Touches:** Customize (custom fields + formula) · `customfields.Validate` · `formula` (Tokenize/Parse/Eval, `Env`) · store round-trip.
- [ ] Add a custom field to an entity; assert it renders on that entity's add/edit form and validates by type.
- [ ] Build a formula (e.g. `round((income-expense)/income*100)`); assert the live result matches the figures.
- [ ] Save the formula; reload; assert it persists and re-evaluates.
- [ ] Assert sandbox safety: a non-allowlisted function / unknown var errors cleanly (no escape).
- [ ] unit: `formula` eval + security + `customfields.Validate` round-trip.

#### D23. Accounting money display consistency on every surface ★
**Workstream:** the same money value renders identically (grouped thousands, parentheses for negatives) everywhere it appears.
**Touches:** `money.FormatAccounting` · Dashboard · Accounts · Budgets · Goals · Transactions · Planning · charts.
- [ ] Pick one negative and one large value; assert identical formatting on every screen that shows it.
- [ ] **(Currently fails — C2:** Accounts/Budgets/Goals drop grouping; Transactions use `-` not parentheses.)**
- [ ] Assert chart axes/labels use major units + currency formatting (**C16**).
- [ ] fix: route every money render through `money.FormatAccounting`; add a guard test/shared helper so
      new surfaces can't bypass it.

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
- [x] Monthly-spend extraction summary view — `internal/spendsummary.Summarize` (per-month out/in/net,
      tolerant parsing, table-tested) rendered on the Documents screen as a per-month card over the
      draft rows (account-currency amounts; undated rows surfaced).
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
- [x] `Plan{ID, Name, HorizonMonths, StartBalance, Items[]}` + `PlanItem{ID,Label,Kind,Amount,Month}`
      + CRUD — across all layers: `domain` data, `internal/planning` engine (`Project`/`MonthlyNet`/
      `EndBalance`, table-tested), persistence (`plans` table, store CRUD, dataset round-trip, validated
      appstate accessors), and a Planning-screen **Plans card** (create name/horizon/start/monthly,
      list with projected end balance, delete) — now also captures an optional **one-time item**
      (amount + in-horizon month) on create. Later: a full per-plan item add/remove editor for existing
      plans; scenario-vs-actuals.
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

- [x] ★ Criterion scorers: returns, stability, liquidity, debt reduction, **goal progress** — all done
      (`internal/allocate`, tested), persisted on saved profiles (`AllocationProfile.GoalProgress`,
      round-trip-tested), and wired through the Allocate UI (weight input, "Finish goals" preset,
      `GoalProgress` populated from goal pace, breakdown "· goal N%" note).
- [x] ★ Weighted combination by profile; normalization; deterministic (`Score`/`Rank`)
- [x] `AllocationProfile{ID, Name, Weights}` + CRUD — model + persistence (`domain.AllocationProfile`,
      `allocprofiles` table, store CRUD, dataset round-trip, validated appstate accessors; table-tested)
      **and** an Allocate picker with editable criterion weights, preset/saved-profile loading, and
      save/delete. Constraints/CustomCriteria on the profile are a later extension.
- [x] Constraints: emergency buffer, max-per-destination, exclusions — applied/clamped across engine
      and UI. Exclusions (engine + UI), emergency buffer (reserve input), and **max-per-destination**
      (new amount-split input → `SplitOptions.MaxPer`) are all wired; overflow falls into the kept-back
      note. (Persisting reserve/max-per onto a saved profile is a possible later extension.)
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

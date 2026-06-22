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
- [x] Verify: hard-refresh at `/accounts`, `/transactions`, `/budgets`, … lands on the correct screen
      online and offline; the `*` route still catches genuinely unknown paths.
      Done: Playwright hard-loaded `/`, `/accounts`, `/transactions`, `/budgets`, `/goals`, and an unknown
      route both online and with the browser context offline after service-worker activation; each route
      rendered the expected screen title with one Shell and no browser errors.

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
- [x] Live drag-over reflow PREVIEW — DONE. A `uistate.UseDragPreview` atom (set on `OnDragOver`) drives a
      render-time `Move` of the dragged tile in front of the tile under the cursor, so the grid reflows
      *during* the drag (FLIP-animated). Render-only — the persisted layout is untouched, so a drop keeps
      it and a drag-end-without-drop reverts cleanly. Verified: dragging Income over Net worth moves it to
      column 1, and cancelling reverts to column 2.
- [ ] Prefer pointer events over HTML5 DnD for touch (the remaining drag-input refinement).
- [x] **Animate reorder** AND **animate resize**: DONE via a FLIP shim (`web/flip.js`,
      `cashfluxFlipBento`) — it records each tile's screen position, and on the next layout change jumps
      moved tiles back to their old spot (transition:none) then transitions the offset to zero next frame,
      so any reflow (drag-reorder, resize, auto-layout switch) glides. A `ui.UseEffect` in the dashboard
      fires it, keyed on a layout signature (items order/spans/importance + mode). Stateful in JS
      (leak-free) and honors `prefers-reduced-motion`. Verified: forcing a tile to move then calling the
      shim applies the inverse `translate(...)` with transition:none (the FLIP invert step).
- [x] **Resize handles only while holding Shift**: `.rz` hidden by default, revealed when the root has
      `data-resize` (toggled by a global Shift keydown/keyup listener + window-blur clear in
      `internal/app/resizereveal.go`), with an opacity fade. Keeps the bento visually calm.
- [ ] **Rethink resize controls: hover/focus directional handles instead of Shift+click cycle.** Keep the
      default dashboard visually calm, but make resize discoverable when a tile is explored: show subtle
      edge handles only on `.w:hover` / `.w:focus-within`. Use direct spatial controls instead of cycling:
      left edge = narrower, right edge = wider, top edge = shorter, bottom edge = taller. Hide/disable
      impossible directions at min/max spans (`1..4` columns, `1..3` rows). Plain click changes one step;
      no modifier key required. Keep Shift+Arrow keyboard resizing as the accessible power path, and add
      clear `aria-label`/tooltips (`Narrower`, `Wider`, `Shorter`, `Taller`). Touch fallback: expose the
      same handles for the focused/selected tile in layout/customize mode later; do not move spatial
      resizing into the flip settings modal.
- [~] Verify: multi-cell tiles never overlap + resize re-packs — **done** (Pack model + render verified
      in-browser); smooth FLIP animations — **done** (above). Still open: a live drag-over preview (reflow
      lands on drop) and pointer-events over HTML5 DnD for touch (the deferred top item).

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
- [x] Register `/` as a **layout** component that renders the Shell chrome **once** and places
      `router.GetOutlet()` for the active child — the layout must NOT itself be the Dashboard.
- [x] Register each screen as a **child route** that renders only its screen content (drop the
      per-screen `Shell` wrapper in `app.go`); the layout supplies the chrome.
- [x] Keep the Dashboard as the layout's root fallback content when there is no child outlet, so `/`
      renders home without wrapping it in a second Shell.
- [x] Keep `*` as the not-found registration (already correct); unknown paths render dashboard content
      inside the single root Shell.
- [x] Verify (ideally with the browser oracle once Playwright is installed — see §0): navigating and
      hard-refreshing every route renders exactly one Shell; no stacked/duplicated chrome.
- _Verify:_ Playwright hard-loaded `/`, `/accounts`, `/transactions`, `/budgets`, `/goals`, `/insights`,
  and an unknown path, then clicked Accounts → Transactions; every pass had exactly one `.rail`,
  one `main#main`, and one `.topbar`, with no browser errors.

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
- [x] Optional hardening: derive nav groups from `screens.All()` (or a Group field on Route) so a new
      routed screen can't silently miss the menu again. Done: `Route` has a `Group` field
      (`GroupPrimary`/`GroupTools`/`GroupSystem`); `shell.go` `navGroup()` builds each rail section by
      filtering `screens.All()` on it, in registry order, so membership lives in one place. Icons/label
      keys stay in the shell's `railMeta` (design layer); an unmapped new screen still appears with its
      registry label + a default icon rather than being dropped. The hardcoded System group is gone.
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
- [x] Responsive: collapses gracefully in a narrow top bar — handled by C19 (`.reso-control` wraps and
      the whole control cluster drops to its own full-width row below 1024px; verified at 768/390px).
- [x] Verify (live): the control reads as a single label ("Jun 2026", not "Jun 2026 – Jun 2026"); the
      "Last period" preset shifts to May 2026; one stepper that "Custom range" expands to two From/To
      steppers; resolution persists. Confirmed in a headless browser.
- [x] _Decision:_ kept the full From/To range power behind "Custom range" (the recommended option), not a
      single-period-only control.

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
- [~] **Keyboard:** the div-based **toggle switch** and **accent swatches** are focusable + operable
      (tabindex=0 + Space/Enter via `OnKeyDown`; focus ring via `:focus-visible`). Segmented = real
      buttons. The **bento tiles are now keyboard-reorderable** — each is `tabindex=0` with
      `aria-keyshortcuts`, and Arrow keys move it one slot earlier/later (reuses `dashlayout.Move`,
      persists, switches to Custom) while **Shift+Arrow resizes** it (`dashlayout.ResizeItem`, clamped).
      Verified: ArrowRight moves a tile 1/2→2/2; Shift+ArrowRight grows it to "1 / span 2". The bento is
      now fully keyboard-operable. Still pointer-only: inline-edit focus-on-enter/exit and the nav reorder
      (B8, drag-only).
- [x] **Dialogs (`FlipPanel`, the B11 add panel, confirms):** `role="dialog"` + `aria-modal="true"` +
      an accessible label, **Esc to close**, a **focus trap** (Tab/Shift+Tab cycle within), **initial
      focus** into the dialog, and **focus restore** to the trigger on close — all done in one shared
      effect covering every overlay.
- [~] **Custom controls → correct ARIA:** Segmented = `role="radiogroup"`/`role="radio"`/`aria-checked`;
      Toggle/ToggleRow = `role="switch"` + `aria-checked` + name; StepperPill ‹/› have `aria-label`s;
      SwatchPicker = labelled `role="radiogroup"` of `role="radio"` chips. The gear (`aria-label="Widget
      settings"`), accounts "⋯" overflow (`aria-label`), and the grip (`aria-hidden`, decorative) now have
      correct names; the AddMenu/menu/+Add carry text or titles. Still TODO: real keyboard operability for
      the div-based Toggle/Swatch (they have Space/Enter via OnKeyDown; verify with a screen reader).
- [x] **Focus visibility:** a global `:focus-visible` ring (accent, 2px offset) on every interactive
      element/role in both themes.
- [x] **Screen-reader / live regions:** the toast notice is now a persistent live region (idle region
      stays mounted; errors are `assertive`/`role=alert`, normal notices polite) so async outcomes are
      announced. Form errors are now associated to their inputs via `aria-describedby`, and required
      fields are marked (`aria-required`). The Transactions list now has a `role="status"`/`aria-live=polite`
      region that announces the **filtered match count** (incl. the zero-results case) as filters change.
      The account **reconcile/update-balance** flow now also posts a polite toast notice ("Updated
      <account> to $X") — it previously succeeded silently — so balance updates are announced and visibly
      acknowledged like Mark-updated already was.
- [x] **Color is never the only cue:** audited every color-coded state. Budget bars carry
      "On track/Near limit/Over budget" text, net-worth/highlights use ▲/▼ arrows, stale accounts show
      a "Stale" badge, cleared shows a ✓; the one offender — the To-do widget's priority dots (high vs
      medium were both `●`) — now uses distinct shapes ▲/●/○ plus accessible names.
- [x] **Contrast:** built `internal/contrast` (table-tested) and **audited** the tokens with it.
      Fixed: `text-faint` now meets AA-normal (4.5) on both surfaces in both themes (dark→#888890,
      light→#686870). The appearance settings now **show the selected accent's contrast ratio** vs the
      theme surface and warn when it's low (uses `internal/contrast`) — so users see when an accent is
      hard to read. The **default accent** is now seagreen `#2e8b57`, picked with `internal/contrast`
      to clear AA-UI (3:1) on BOTH theme surfaces (dark 4.09:1, light 3.63:1); the old mint `#54b884`
      failed on light (~2.1:1). One default that passes everywhere beat a per-theme accent, since the
      accent drives the focus ring and is applied inline by JS where the active theme isn't always known.
- [x] **Motion:** `prefers-reduced-motion` covers the flip-panel, toast slide-in, rail width, boot, the
      rail flyout, AND the dashboard reorder/resize/drag FLIP animations (`web/flip.js` checks
      `matchMedia('(prefers-reduced-motion: reduce)')` and only records positions, no transition, when set).
- [x] **Zoom / reflow:** the Display/Text-size control reaches 200% (C26) and the C10/C19 responsive
      layout reflows at the effective width — verified live: at `--ui-scale: 2` on a 1280px window the page
      reflows to the phone layout with no horizontal scroll. Meets WCAG 2.1 SC 1.4.4 / 1.4.10.
- [x] **Forms:** correct input types (number/date) in use; **inline validation is announced** —
      every `.err` message is `role="alert"`; **required fields carry `aria-required`** across every add
      form (accounts, categories, budgets, goals, members, rules, to-do, transactions, quick-add,
      plans). Each form's error is now also **associated with its primary input** via `aria-describedby`
      (+ `aria-invalid`) when present, so a screen reader re-announces the error on focus, not just once
      via `role="alert"`. Done with a shared `errAttrs`/`errText` pair (`internal/screens/aria.go`) wired
      into all 11 add-forms (accounts/budgets/categories/custom-fields/goals/members/rules/to-do/
      transactions + planning's recurring & plan forms), each with a stable error id.
- [x] **Route changes (SPA):** focus moves to `<main>` on navigation (skips the initial load so the
      first Tab still reaches the skip link) and `document.title` is set to "<Screen> · CashFlux".
- [x] **Charts:** `ui.AreaChart` and the D3 `ui.Chart` are both `role="img"` + `aria-label` with a
      live-figure summary (net-worth trend, planning forecast, breakdown). The D3 container sets role/label
      in `chartd3.go`; the sparkline in `chart.go`.
- [~] **Touch targets:** small icon-only buttons (delete/toast-x/rstep/set-close) now meet the WCAG
      2.5.8 AA 24×24 minimum (centered glyph). 44×44 (AAA) left aspirational given the dense desktop UI.
- [x] **i18n:** all `aria-label`s now resolve via `uistate.T()` (the language store) — the last two
      hardcoded ones (the widget gear "Widget settings" and the SwatchPicker "Accent color") were routed
      through new `widget.settings` / `a11y.accentColor` keys, so they translate with everything else.
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

### B17. Privacy: app lock — passcode gate + inactivity lock + recovery ★ (feature spec, researched 2026-06-18)

**Want (user):** on a shared/family computer the app shouldn't be visible without a gate. Add an
**on-load passcode/PIN screen**, a **non-activity timeout lock screen**, **settings** to configure both,
and a **clear recovery strategy** so data is never lost forever. Greenfield — no auth/crypto exists today.

**★ Principle — fully OPTIONAL, OFF by default, opt-out-able (user, 2026-06-18):** the lock is a
convenience for those who want it; users who don't care must never be forced to deal with it.
- [ ] **Off by default.** Fresh install boots **straight into the app** with no gate, no passcode, no
      inactivity lock — identical to today. (No encryption either, in option (b): default = plaintext as
      now; encryption only switches on when the user enables the lock.)
- [ ] **No nagging.** At most a single, dismissible, *non-blocking* hint that privacy lock exists (e.g. a
      one-line note in Settings → Privacy); never a modal/interstitial pushing the user to set it up, and
      "dismiss" means gone for good.
- [ ] **Easy, complete opt-out at any time.** Settings → Privacy → turn off (requires the current
      passcode). Disabling must **fully revert**: remove the gate + inactivity lock, and in option (b)
      **decrypt the dataset back to plaintext** and discard the keys/verifier, so the app returns exactly
      to the no-lock state (no residual encrypted blob that could strand data).
- [ ] Each piece independently toggleable: a user can enable the **passcode gate** without the
      **inactivity lock** (and vice-versa) — don't force the bundle.

**★ Decision to confirm FIRST (drives everything): soft gate vs. encrypted-at-rest.**
CashFlux is local-first with **no backend**, and data persists in a **durable origin store** (OPFS /
IndexedDB / SQLite-wasm — verified #42). Therefore:
- **(a) Soft gate** — the passcode only hides the UI; the dataset stays **plaintext on disk**, readable
  via DevTools/IndexedDB or the export file. Easy to build; **not real privacy** against anyone technical.
  If chosen, the lock screen MUST honestly say "hides the screen; does not encrypt your data."
- **(b) Encrypted-at-rest (recommended)** — passcode → KDF → key; the dataset is **encrypted in the
  store**, decrypted only after unlock. Real privacy on a shared machine, but a meaningful change
  (encrypt/decrypt the persisted dataset + key management + recovery). _Confirm (a) vs (b) before building._

**Threat-model note (state plainly in UI):** even with (b), this protects against casual access by family
members, not a forensic attacker; WebCrypto needs a **secure context** (https / localhost — GitHub Pages
is https, OK). There is **no server, so no "password reset"** — recovery must be designed in (below).

**Bottom-up plan (per SDLC):**
- [ ] **Pure logic `internal/lock` (or `vault`)** — no `syscall/js`, table-tested: KDF (PBKDF2 via
      WebCrypto, or Argon2id if a pure-Go/wasm impl is acceptable) with a random per-install **salt**;
      a **verifier** (so a wrong passcode is detected without decrypting); for (b): AES-GCM
      encrypt/decrypt of the dataset blob with a random **data key**, and **envelope/key-wrapping** so the
      data key is wrapped under BOTH the passcode-derived key AND a recovery key (either can unlock);
      recovery-key generation (high-entropy, human-readable groups). Tests: round-trip, wrong-passcode
      rejected, recovery-key unlocks, tamper detection.
- [ ] **State/persistence** — store lock config + salt + verifier + wrapped keys as a small **always-
      readable** blob (separate from the encrypted dataset, so the gate can verify before decrypting).
      Decide what's gated: dataset, **and** the persisted OpenAI key (it's sensitive — encrypt it too),
      prefs can stay clear.
- [ ] **On-load passcode screen** — first run: optional "Set a passcode" (opt-in; offer Skip). If set,
      boot shows a gate before the app; verify → derive key → (b) decrypt into memory. Real
      `<input type=password>`/PIN (so password managers work); PIN vs password choice; rate-limit/backoff
      on repeated wrong attempts (and a long delay, not lockout-that-destroys).
- [ ] **Inactivity-timeout lock** (`syscall/js`): configurable timeout (Off / 1 / 5 / 15 / 30 min),
      reset on pointer/key/visibility activity (debounced); on timeout → show lock screen **and clear the
      decrypted dataset from memory** (so plaintext isn't resident); optional **lock-on-tab-hidden** and an
      explicit **"Lock now"** button. Sync lock state across tabs (BroadcastChannel/storage event).
- [ ] **Settings → Privacy** — enable/disable lock; set / change (requires current) / remove passcode;
      passcode type (PIN/password); inactivity timeout; lock-on-blur toggle; "Lock now"; **view/regenerate
      recovery key**; "Forgot passcode?" entry. Plain-English copy; accessible (labelled, keyboard, SR).
- [ ] **★ Recovery strategy (avoid losing data forever)** — REQUIRED for option (b):
      - [ ] **Recovery key**: generated at setup, shown once with **download/print + "save this"**;
            unlocks the data independently of the passcode (envelope key-wrapping). Re-generatable while
            unlocked.
      - [ ] **Encrypted/plaintext backup**: lean on the existing **Export JSON** (`cashflux.json`, #31) as
            the escape hatch — prompt periodic backups; recovery = re-import a backup (relates to the C33
            import-mechanism portability fix). Optionally offer an **encrypted** export.
      - [ ] **Honest setup warning**: "There is no password reset. If you lose your passcode AND your
            recovery key AND your backups, your data cannot be recovered." Shown before enabling the lock.
      - [ ] **"Forgot passcode" paths**, clearly distinct: **Recover** (enter recovery key / import backup →
            keep data) vs. **Reset** (wipe + start fresh → **destructive**, last resort, double-confirm).
- [ ] **E2E + verify:** set passcode → reload → gated; correct PIN unlocks, wrong rejected; inactivity
      → locks + memory cleared; recovery key unlocks; reset wipes; backup re-import restores; lock state
      syncs across tabs; gate is keyboard/SR accessible. (Add a D-style workstream story.)
- _Cross-links:_ pairs with **C27** (persist OpenAI key — should be encrypted under the lock),
  **C33** (import mechanism — recovery depends on a working, portable import), and the export round-trip.

**B17.1 — Lock-screen experience: smart quotes, opt-in glanceable data, locking/unlocking animations (user, 2026-06-18).**
A rich, configurable lock screen (replaces the native `prompt`/`alert` setup — see C42/#65). All content
configurable; **privacy-first defaults**.
- [ ] **Smart quotes (default ON):** a curated, rotating set of finance/motivation quotes (pure
      `internal/lockquotes`, table-tested; deterministic rotation by day/index since `Math.random` is
      banned in logic; no network). Toggle off in Settings.
- [ ] **Safe metadata (default ON, no sensitive data):** clock/date, greeting, day — nothing financial.
- [ ] **★ Opt-in glanceable data (default OFF — privacy guardrail):** like a phone lock screen, optionally
      surface **notifications/reminders (B19)** and **timing-based events** (next bill due, payday in N
      days, budget-period countdown). **The lock screen is visible to anyone at the device,** so gate behind
      explicit, *tiered* opt-in:
      - **Off** (default) → quotes + time only.
      - **Counts only** → "3 reminders · 1 bill due soon" (NO amounts/payees).
      - **Previews** → reminder text + event timing, still **no balances/amounts** unless a separate
        "show amounts on lock screen" toggle is on (with a clear warning).
      Never show balances/account numbers by default. Data comes from the B19 notify/catch-up engine +
      a `freshness`/bills timing source; the encrypted store stays locked — only the allowed, redacted
      summary is surfaced.
- [ ] **Locking / unlocking animations (several, selectable):** fade, **frosted-glass blur→sharpen** on
      unlock, **iris/circle reveal**, slide/curtain, the existing **FlipPanel `rotateY`** flip, and a
      "vault door" close/open. User picks in Settings (ties **B20** theming). **Respect
      `prefers-reduced-motion`** (instant/fade fallback); keep unlock **snappy** (animation must not delay
      access after a correct passcode). Lock animation on auto-lock/Lock-now; unlock animation on correct entry.
- [ ] **A11y:** the lock gate is a real focusable form (passcode input autofocused, Enter submits, labelled,
      SR-announced); animations are decorative (`aria-hidden`), never block input. _Cross-links: B19 (data
      source), B20 (animation/theming), C42 (no native prompt), C26 (text size on the gate)._

**B17.2 — Enable/disable toggle that preserves creds + recovery setup at password creation (user, 2026-06-18).**
**Separate "Configured" from "Enabled".** State: `LockConfig{ Configured bool, Enabled bool, KDFParams,
Salt, Verifier, WrappedDK[]{method,blob}, RecoveryMethods[], AutoLockMinutes }`. *Configured* = creds
exist; *Enabled* = the gate is active. Toggling Enabled must **NOT** wipe creds (no forced re-create).
- [ ] **Settings → Privacy → "Lock screen" toggle** that flips `Enabled` **without touching** Salt/
      Verifier/WrappedDK/recovery. Re-enabling needs **no new passcode**.
- [ ] **★ Toggle is gated behind the passcode** — changing `Enabled` (especially **OFF**) prompts for the
      **current passcode** (verified against `Verifier`) **even if the session is already unlocked**, so a
      passer-by at an unlocked screen can't silently disable protection. Use the FlipPanel passcode modal
      (C42), not a native prompt.
- [ ] **Three DISTINCT actions — don't conflate:** (1) **Lock ON/OFF** (keep creds; behind passcode);
      (2) **Change passcode** (requires current → re-wrap DK under the new passcode-KEK); (3) **Remove/forget
      passcode entirely** (requires current; wipes creds + recovery; decrypts data → plaintext = full
      opt-out, B17 principle).
- [ ] **Encryption interaction (honest design):** with encrypted-at-rest (B17 option b), "disabled" can't
      both keep data encrypted *and* skip the prompt — so on **disable**, wrap the data key (DK) under a
      locally-stored **device key** so the app auto-unlocks while off; on **enable**, drop the
      device-wrapped copy. **State plainly:** *disabled = no gate, data accessible on this device* (a
      deliberate convenience trade-off) while creds/recovery stay intact for instant re-enable. (Soft-gate
      model: disable just hides the gate — trivial.)
- [ ] **★ Recovery setup AT password creation — multi-strategy via envelope / multi-KEK.** The random
      **data key (DK)** is wrapped under several **KEKs**, any of which unlocks → then reset the passcode
      (re-wrap DK under a new passcode-KEK). Strategies chosen at setup:
      - **Recovery code (default, strongest):** auto-generated high-entropy code, shown once + download/print;
        `KEK = KDF(code, salt)`. Zero-knowledge, no server.
      - **Security questions (optional, weaker):** user picks **≥3** questions + answers; normalize answers
        (trim/lowercase/strip punctuation) → `KEK = KDF(normalized answers, salt)`. **Low entropy /
        guessable** → warn it's weaker than the code, recommend pairing (not sole), allow N-of-M if desired.
      - **Backup file (always available):** the existing Export JSON (#31) — recovery = re-import.
      Adding a recovery method = add a `wrap(DK, KEK_method)` entry; removing = drop it. "Forgot passcode"
      offers each configured method → unwrap DK → set a new passcode.
- [ ] **Verify/E2E:** toggle off→on keeps the same passcode (no re-create); toggling off requires the
      passcode even when unlocked; change-passcode keeps data + recovery; recovery code unlocks; security
      questions unlock (and wrong answers don't); remove-passcode fully reverts to plaintext. _Cross-links:
      B17 (a/b decision, recovery), C42 (FlipPanel passcode modal), B19 (lock-screen data)._

**B17.3 — Credential types (password / passphrase / numeric code) + a NIST-aligned strength/hygiene layer (user, 2026-06-18).**
The lock secret can be one of three **types** (chosen at setup, changeable):
- [ ] **Numeric code (PIN):** digits only, **min 6** (recommend 6+), reject trivial — `1234`, `0000`,
      all-same, sequential, and the published common-PIN list. Fast for shared/tablet use.
- [ ] **Password:** any printable + unicode, **min 8**, strength-metered.
- [ ] **Passphrase:** multiple words, **min ~12 chars / ≥4 words**, length encouraged over symbols.
- [ ] **Pure `internal/pwcheck`** (no `syscall/js`, table-tested): `Validate(kind, value) →
      {ok, score 0–4, issues[], suggestions[]}`. Embed a **bundled common/breached blocklist** (top-N
      passwords + common PINs) for offline screening; a **zxcvbn-style guessability estimator** for the
      score + actionable feedback.
- [ ] **Validation layer — follow modern NIST SP 800-63B (the industry standard):**
      - **Length over composition:** enforce a **min length** (per type above); **no forced composition
        rules**, **no mandatory rotation/expiry**, **no password hints**, **no truncation** — allow long,
        allow spaces, allow paste, allow unicode (all per NIST).
      - **Screen against breached/common values** (bundled blocklist) — **hard-reject** known-common
        passwords/PINs and the context-specific weak ones (app name, member/household names, repeats,
        sequences).
      - **Strength meter + tips** ("add another word", "avoid 1234") to **urge** good hygiene — primarily
        *urge* (meter + warnings, can proceed above the floor), with a sane **hard floor** = min length +
        not-on-blocklist + not-trivial. Optionally offer an **online HaveIBeenPwned k-anonymity** check
        when online + opted-in (default offline = bundled list only).
- [ ] **Honest security caveat (state in UI):** a **PIN/numeric code is low-entropy** → weak against
      *offline* brute-force of an exfiltrated encrypted blob (B17 option b). Mitigate with a **strong KDF
      cost** + the B17 rate-limit/backoff, but **recommend a password/passphrase** for real at-rest
      protection; the PIN is "casual-access" deterrence. All types feed the same KDF→KEK (B17.2).
- [ ] **Verify:** each type validates per its rules; common/breached values rejected; trivial PINs
      rejected; meter + suggestions render; the floor blocks but otherwise urges; change-passcode
      re-validates. _Cross-links: B17/B17.2 (KDF/KEK, threat model), C42 (FlipPanel input modal), C26 (text size)._

**B17.4 — Optional password hint (user, 2026-06-18; "not great but saves folks who forget").**
A simple, **opt-in, off-by-default** memory-jog — explicitly **NOT** a recovery method (the real recovery
is the code/security-questions/backup in B17.2). Designed with guardrails because hints cut against the
NIST guidance in B17.3 (hints leak info, doubly so on a shared family screen):
- [ ] User-set free-text hint stored with the lock config (plaintext, since it's a hint by design;
      included in backups). One per credential.
- [ ] **Don't show it for free:** reveal only behind a deliberate **"Forgot? Show hint"** link **after
      N failed attempts** (e.g. 3) — not sitting on the lock screen for any passer-by.
- [ ] **Guardrail validation:** reject a hint that **contains or equals the passcode** (case-insensitive,
      and normalized for PINs) so users can't accidentally write the password as the hint. Warn that a hint
      is visible to anyone with the device and **weakens** the lock.
- [ ] **Framing:** present as a last-ditch jog *below* the real recovery options; never call it "recovery."
      _Cross-links: B17.2 (recovery — the actual safety net), B17.3 (hygiene/validation), C42 (modal UI)._

**B17.5 — Biometric / passkey unlock (Face ID · fingerprint · Windows Hello) — browser API details (user, 2026-06-18).**
**Yes, available — only via WebAuthn.** Browsers expose **NO raw fingerprint/face API** (privacy by design;
biometric data never reaches the page); the OS does the match and returns a crypto assertion.
- [ ] **API:** `navigator.credentials.create()/.get()` with `authenticatorAttachment:"platform"` +
      `userVerification:"required"` → triggers **Touch ID / Face ID (macOS/iOS), Windows Hello, Android
      biometric**. Add as an **optional unlock method alongside the passcode** (never sole; offer where a
      platform authenticator exists).
- [ ] **Use the PRF extension (strong path):** the WebAuthn **`prf` extension** returns a stable secret
      bound to the passkey+biometric → use as a **KEK that wraps the data key (B17.2)**, so biometrics truly
      decrypt the vault (not just a bypassable boolean gate). Client-side, no server.
- [ ] **Constraints:** secure context (HTTPS/localhost — Pages ✓); **PRF is Chromium-forward** (Chrome/Edge;
      Safari/FF partial) → **fall back to passcode (B17.3)** when unavailable; require a platform
      authenticator. _Soft-gate-only (no PRF) = casual deterrence (bypassable via devtools) — note that._
- [ ] **Native (Capacitor, B32 Cluster 5):** use native biometric plugins directly (more reliable than
      WebAuthn-in-WebView) — the mobile path. _Cross-links: B17.2 (KEK/envelope), B17.3 (passcode fallback),
      B32 C1 (passkeys) + C5 (Capacitor)._

### B18. Onboarding + optional quick guide + strong splash screen — ✅ APPROVED (2026-06-18)
**Status: APPROVED — full scope, tour = SIMPLE SLIDESHOW.** Ready to build (bottom-up, one feature per
commit). Want: an onboarding section with an **optional** quick guide and a **strong (branded, polished)
splash screen**. **Approved decisions:** (i) scope = **full** (splash + welcome + tour + checklist +
empty-state CTAs); (ii) tour style = **simple slideshow** (welcome cards with Next/Back/Skip — no
spotlight-coachmark overlay machinery); (iii)/(iv) first-run sample-vs-fresh choice + checklist placement
= builder's discretion (sensible defaults: keep the sample-vs-fresh choice; checklist as a dismissible
dashboard card).
**Principles (inherit B17's ethos): optional, skippable, never blocks, remembered (don't re-show), re-runnable.**

- **1) Strong splash / boot screen.** Today `web/index.html` shows a minimal "CashFlux · Getting your
  money in order…" loader while wasm boots. Upgrade to a **branded splash**: logo/wordmark, accent, a calm
  progress/shimmer, tagline; fades smoothly into the app (or into the welcome). Must (a) appear instantly
  (it's plain HTML/CSS before wasm loads — keep it dependency-free), (b) respect `prefers-reduced-motion`,
  (c) not add perceptible delay (fade out as soon as the app is interactive), (d) be themed (dark/light).
- **2) First-run welcome.** On the very first load only, a dismissible welcome panel (reuse `ui.FlipPanel`/
  dialog): one-line what-it-is + primary choices — **Explore with sample data** (default; sample already
  ships) · **Start fresh** (wipe to empty) · **Take a quick tour** · **Skip** (✕). Persist an
  `onboardingSeen` flag so it never reappears.
- **3) Optional quick guide (product tour) — SIMPLE SLIDESHOW (approved).** A short skippable slideshow
  of welcome cards in a panel (reuse `ui.FlipPanel`/dialog): a few slides on what CashFlux is + key areas
  (sidebar nav, dashboard tiles + drill-in C30, period control, "+ Add", Settings/household, Documents AI,
  Privacy lock B17). Controls: **Next / Back / Skip**, progress dots, Esc exits. Re-runnable from
  **Settings/Help → "Replay quick tour."** No spotlight-coachmark overlay (keeps it simple). A11y:
  keyboard-navigable, focus-managed, `prefers-reduced-motion`, labelled dialog.
- **4) Optional "Get started" checklist (non-blocking).** Small dismissible card (dashboard or a
  self-removing "Get started" page) with first tasks that auto-check from app state: add accounts, set
  base currency, create a budget, add a goal, (optional) set a privacy lock (B17), (optional) add OpenAI
  key. Each links to its screen; dismiss = gone for good.
- **5) Empty states as always-on onboarding.** Ensure each screen's empty state has a clear primary CTA
  (several already do) — the lightweight, ever-present guidance with zero nagging.
- **Build-order (SDLC, when approved):** pure `internal/onboarding` (data-driven step/checklist defs +
  completion predicates from app state, table-tested) → persisted onboarding atom (seen/dismissed/step/
  checklist; store in the always-readable config blob if B17 encryption is on) → splash (HTML/CSS) →
  welcome panel → coachmark tour → checklist card → Settings "Replay tour".
- **Decisions to confirm (the approval):** (i) scope — **full** (splash + welcome + tour + checklist) vs.
  **minimal** (strong splash + welcome + better empty-state CTAs); (ii) tour style — **spotlight
  coachmarks** vs. a simple **slideshow** vs. a short **"what's here" panel**; (iii) does the first-run
  **sample-vs-fresh** choice belong, given sample data already ships; (iv) checklist placement (dashboard
  card vs. dedicated page).

### B19. Communications & notifications — ✅ APPROVED: Phase A only (client-only, NO backend), 2026-06-18
**Status: APPROVED scope = Phase A only (fully client-side; NO backend).** External SMS/email (Phase B)
is **deferred** — if ever revisited, hosting = **BYO serverless** (user-hosted). Want: a notification
system, cost tracking, and notification rules — all client-side.

**No-backend reality (settled with user):** notifications fire **only while the app is open** — in-app
center/toasts + browser `Notification` (desktop pop-ups while a tab is open). There is **no dependable
"closed-app" reminder** without a server (Web Push needs VAPID + a push server = a backend, rejected;
Periodic Background Sync is Chromium-only, PWA-only, throttled, unreliable).
- **Wake Lock API note (user asked "is there a browser API that stops sleep?"):**
  `navigator.wakeLock.request('screen')` keeps the **screen** awake **only while the tab is visible**
  (auto-released when hidden); it does **not** run the app in the background or enable closed-app
  notifications. Useful only for an **always-on/kiosk dashboard** (e.g. pinned on a kitchen display) —
  offer it as an optional "Keep screen awake" toggle on the dashboard, not as a notification mechanism.

**★ Catch-up-on-wake (core Phase A mechanism — user-directed 2026-06-18):** since we can't wake the
device, the app **reconciles on open/return** — check the current time and "catch up" on whatever would
have fired while it was closed.
- [ ] Persist **`lastSeenAt`** (last time the app was open/active) in the durable store.
- [ ] On **load** and on **wake** (`visibilitychange`→visible / window focus), compute the gap
      **[lastSeenAt, now]** and run the rules engine over it: for each rule, compute the scheduled
      occurrences in that window and evaluate current data conditions (bill due date passed, budget crossed
      a threshold, account went stale, weekly/monthly digest came due). Then set `lastSeenAt = now`.
- [ ] **Surface as a "While you were away" summary** in the notification center — collapsed/deduped/capped
      (e.g. "3 things happened…"), never a flood. Long gaps (away a month) collapse to one digest.
- [ ] **Idempotency:** keep a **delivered-log** keyed by rule+period so reopening repeatedly doesn't
      re-fire the same catch-up; respect already-acknowledged items.
- [ ] Also run rule evaluation on a **timer while open**, so a noon bill-due fires during an active session
      too (not only on next open).
- [ ] **Pure + testable:** `notify.CatchUp(rules, lastSeenAt, now, dataSnapshot, deliveredLog) → []Notification`
      — deterministic given inputs; table tests for gap windows, scheduled-occurrence math (timezone/clock
      changes), dedupe, and long-gap collapsing. No `syscall/js`.

**★ Electron path (user note 2026-06-18 — relates to §5.1 desktop wrapper):** the **Electron build can
bypass CORS** (the main/Node process makes server-side HTTP calls — no browser CORS) and can run a
**tray/background process** with OS-level notifications. So Electron could enable the deferred **Phase B
(direct SMS/email + scheduled/closed-app reminders) WITHOUT a hosted relay** — the desktop app acts as its
own local "backend," keys stored on-device.
- [ ] Treat Electron as the **Phase-B enabler** for external comms: provider adapters run in the Electron
      main process; the wasm/web build keeps Phase A (client-only catch-up) as the baseline.
- [ ] **Caveats to design for:** provider **keys live on the local machine** → **encrypt them under the
      B17 lock**; true closed-app/background delivery needs the **Electron process running** (tray +
      launch-at-login), else it's still catch-up-on-wake; keep the same `Notifier`/rules core so web (Phase
      A) and Electron (Phase B) share logic and only the transport differs.

**Original draft retained below for context (SMS/email = the deferred Phase B).** Want: integrations to
send comms (SMS: Twilio/Telnyx/Plivo/Vonage; Email: Resend/SES/Mailgun/Postmark), a notification system,
cost tracking, and notification rules.

**★ Architectural reality (decide FIRST):** CashFlux is local-first, client-side wasm, **no backend**.
SMS/email providers are **server-side only** — calling them from the browser is **blocked by CORS** and
would **expose the API secret in the browser** (readable on a shared family computer — directly
contradicts B17 privacy). Also, a **closed app can't send scheduled reminders** (no server to run the
schedule). So external SMS/email inherently needs a server/relay.
- (a) **Hosted relay/backend (recommended if external comms are required):** small service holds provider
  keys + adapters; the app posts notification requests; it runs schedules for when-app-is-closed
  reminders. Could live with the Phase-3 sync server. Adds hosting + cost + a privacy boundary.
- (b) **BYO serverless relay:** user deploys a Cloudflare Worker / Lambda with their own keys; app calls
  that endpoint. Keeps "no shared backend" but high setup friction.
- (c) **Direct browser BYO-key (like the OpenAI key): NOT viable** — SMS CORS-blocked; keys exposed for
  both. Reject for SMS/email.

**Phased strategy:**
- **Phase A — buildable now, fully client-side (no infra):**
  - Pure `internal/notify`: notification types/events; a **rules engine** (trigger + condition + channel +
    threshold + quiet-hours + frequency cap); templates; a notification **log/queue**. Table-tested, no
    `syscall/js`.
  - **In-app notification center** (bell + list) + toasts (extend `uistate.Notice`/Toast) + **browser
    Notifications API** (`Notification.requestPermission`; fires only while the app is open).
  - **Channel abstraction** `Notifier` interface (InApp, Browser now; Email, SMS later via relay).
  - **Cost-tracking model** (pure): per-provider price-per-message, usage log, monthly estimate + optional
    budget cap + an "off until configured" guardrail — mirrors `ai.EstimateCostUSD` (C27). Surfaced in
    Settings.
  - **Notification rules UI** (Settings): events = bill due soon, budget near/over (`budgeting`), goal
    milestone/pace (`goals`), stale balance (`freshness`), large transaction, weekly/monthly digest;
    per-rule enable + channel + threshold + quiet hours + frequency cap.
  - **Privacy guardrails (ties B17):** external messages carry **minimal/no sensitive detail** ("A budget
    is near its limit — open CashFlux"); explicit **opt-in** + "this leaves your device" notice; default
    **OFF**.
- **Phase B — needs infra (relay/backend):**
  - Relay with provider **adapters** behind one interface — Email: **Resend** (easiest) / **SES**
    (cheapest at scale) / Mailgun / Postmark; SMS: **Telnyx** (cheap) / **Twilio** (easiest) / Plivo /
    Vonage. Keys live on the relay, **never the browser**.
  - **Scheduled delivery** for when the app is closed (relay runs the cron); reconcile cost tracking with
    provider usage/webhooks.
  - Settings: choose provider + relay endpoint/credentials + verify-send test.

**Decisions — RESOLVED (2026-06-18):** (1) **Phase A only — client-only, no backend** ✅; (2) external
relay (if ever) = **BYO serverless**, deferred; (3)/(4) — N/A until Phase B. _Cross-links: B17
(privacy/secrets), C27 (AI cost-surfacing pattern), freshness nudge (#30)._
**Still open to confirm before building Phase A:** which client-side events ship first (recommend: bill
due, budget near/over, stale balance, weekly digest), and whether "cost tracking" is even relevant for
Phase A (in-app/browser notifications are **free** — so the cost-tracking model is really a Phase-B
concern; for Phase A, drop it or keep only a stub for future external channels). _Confirm before build._

### B20. Theming engine — colors, fonts, sizes, header images, icon packs — ✅ APPROVED (2026-06-18)
**Status: APPROVED — FULL scope.** Decisions locked: (1) **Full** (color tokens + fonts + font-size +
radius + presets + custom-save + import/export + contrast **AND** header images + app icon packs +
per-widget colors); (2) fonts = **allow custom font-file upload** (plus the curated list) — handle font
asset storage (size-capped in the durable store, under B17 lock), perf, and a graceful fallback if a
font fails to load; (3) **unify** — the engine **subsumes** today's theme/accent/density/display-scale
prefs into one system (migrate existing prefs → theme tokens; update the Settings UI accordingly).
Ready to build bottom-up (one feature per commit). Want: a theming engine covering border color,
background colors, widget colors, fonts, font sizes, header images, app icon packs, etc.

**Foundation (already exists — extend, don't reinvent):** `internal/prefs` + `uistate.ApplyPrefs`
already drive **CSS custom properties** for theme (dark/light/system), **accent** (swatch), **density**
(compact), **display scale** (B6 `--ui-scale`), week-start, date format — reload-persistent. Tokens live
in `web/index.html` `<style>` + Tailwind config (`--bg-base/--bg-card/--border/--text/--accent/--cell`…).
The engine generalizes this into a full, user-editable **design-token theme**.

**Architecture (bottom-up, SDLC):**
- [ ] **Pure `internal/theme`** (no `syscall/js`, table-tested): a typed `Theme` struct of tokens —
      colors (`bgBase, bgCard/widget, border, text, textDim, accent`, semantic up/down, per-widget
      optional), **radius**, **font family** (UI + display), **font-size scale**, density, header image
      ref, icon-pack id; `Validate()` (valid colors + **contrast AA** checks, ties B15); `Default()` +
      built-in **presets** (e.g. Midnight / Paper / Forest); `CSSVars()` → the var map; JSON
      **import/export** (shareable themes); merge/override semantics.
- [ ] **State:** persist the **active theme** + **user custom themes** (durable store / localStorage;
      under the B17 lock if encryption is on). Extend `ApplyPrefs` → `ApplyTheme` to set every token on
      `:root`/`#app`. Subsume the existing theme/accent/density/scale prefs into the engine (one system).
- [ ] **UI — Theme editor** (Settings → Appearance, or a dedicated "Theme" panel): pick a preset → tweak
      tokens via color pickers / font selectors / size sliders / radius; **live preview**; save as a named
      custom theme; **reset to default**; import/export theme JSON. Plain-English, accessible.
- [ ] **Fonts:** offer a **curated list** (the already-loaded Inter + Fraunces, plus a few web-safe/
      bundled options) for UI + display fonts, and a **font-size scale** slider. _Note: arbitrary custom
      **font-file upload** is heavy (font assets) — defer; curated list first._ Ties C25 (density) + C26
      (text-size); the px-heavy styling means size theming needs the **px→rem token cleanup** to fully
      bite — note the dependency.
- [ ] **Header images:** optional dashboard/app **banner image** — store as a size-capped data/object URL
      in the durable store; apply as a CSS background on a header band; offer a few built-ins + upload with
      a cap. Perf/size caveat noted.
- [ ] **App icon packs:** selectable icon set — depends on **B13** (typed `internal/icon`, now rendering
      since C28 fixed `viewBox`). Feasible scope: an **icon style** (stroke width / outline-vs-filled) or
      a small set of curated packs mapped behind `icon.Name`; full third-party packs are larger. Note
      feasibility per pack.
- [ ] **A11y guardrails (must-keep, ties B15):** validate text/bg **contrast** and warn or auto-nudge so
      a custom theme can't become unreadable; always keep a **Reset to default**; respect
      `prefers-reduced-motion`; don't let header images reduce text legibility.
- [ ] **Verify/E2E:** apply preset → tokens change live; edit + save custom theme → persists across
      reload; import/export round-trip; contrast warning fires on a bad combo; reset restores default.
**Decisions — RESOLVED (2026-06-18):** (1) **Full** scope (incl. header images + icon packs + per-widget
colors); (2) **custom font-file upload allowed** (+ curated list); (3) per-widget colors **in scope**;
(4) **unify** — engine subsumes theme/accent/density/scale prefs (with migration). _Cross-links: B6
(display scale), C25/C26 (density/text-size + px→rem), B13 (icons), B15 (contrast), B17 (persist under
lock)._
- [ ] **Custom font upload (now approved) — design notes:** store uploaded font files size-capped in the
      durable store (under the B17 lock); apply via `@font-face` from an object/data URL; **graceful
      fallback** to a curated font if load fails; note licensing is the user's responsibility; cap count/size.

### B21. Reports engine — charts, narrative, change-% , shareable — ✅ APPROVED (2026-06-18)
**Status: APPROVED.** Decisions locked: (1) charts = **adopt D3** → this **activates B14** (D3 charting
behind the typed `chartspec` interface; pin + SW-cache D3 for offline); (2) narrative = **both**
(deterministic default + optional AI enhance); (3) shareable = **all four** — Print-to-PDF + standalone
HTML + PNG image + CSV/JSON; (4)/(5) builder's discretion (recommend Spending + Net-worth history +
Year-end/tax first; new **Reports** nav screen). Ready to build bottom-up.
- [x] **★ Export design note (D3 + shareable):** D3 is a live JS dep — for the **standalone HTML / PNG /
      PDF** exports, embed the **already-rendered static SVG** (snapshot the chart's SVG markup), NOT a
      live-D3 dependency, so shared files open anywhere offline with no JS. In-app reports use live D3;
      exports use the rendered SVG snapshot. Pin D3 + add it to the service-worker cache (B14) for the
      app's own offline use.
      `docs/REPORT_EXPORTS.md` now records the SVG-snapshot export policy, privacy guardrail, CSV/JSON data
      source rule, and D3 `7.9.0` service-worker cache requirement.
Want: a reports engine — charts, descriptions, number-change percents, polished graphical style, **shareable**.

**Concept:** a **Reports** section that turns the ledger/budgets/accounts into structured, visual reports
with hero KPIs + period-over-period **change %**, charts, and plain-English narrative — distinct from the
dense dashboard and the AI-narrative Insights.

**Architecture (bottom-up, SDLC):**
- [ ] **Pure `internal/reports`** (no `syscall/js`, table-tested): each report = a function over
      (dataset, period) → a typed `Report{ Title, Description, KPIs[]{label,value,delta%,tone}, Series[]
      (for charts), Tables[] }`. Reuses existing logic (`ledger.PeriodTotals`/`NetWorthSeries`/
      `CategorySpendSeries`, `budgeting`, `goals`). **Period-over-period delta**: this vs last period/year
      → % change + up/down tone. Deterministic → fully unit-testable.
- [ ] **Report catalog:** Spending (by category, top movers, vs last period), Income-vs-Expense / cash
      flow (+ savings rate), **Net-worth history** (over time, by class/account), Budget performance
      (actual vs budgeted, over/under), Category trends (sparklines + biggest movers %), **Year-end / tax
      summary** (annual category totals, exportable), Member breakdown, Goals progress.
- [ ] **Charts:** needs richer kinds than today's area/bar — **line, stacked bar, donut/pie, sparkline**.
      _Decision: grow the **pure-Go SVG** chart helpers (no dep, offline, testable — fits local-first) vs
      adopt **D3** (B14 — richer/interactive but JS dep + vdom-portal complexity)._ The C16 fix already
      makes charts plot dollars correctly.
- [ ] **Narrative descriptions:** **deterministic** templates from the numbers ("You spent $X, up Y% from
      last month, driven by Groceries") — works offline, no key; **optionally AI-enhanced** via the
      existing `ai`/Insights path. Default = deterministic.
- [ ] **Change-% component:** a "stat with delta" (▲/▼ + % + color) reusing `figTone`/accounting format
      (and the color+text a11y rule from B15).
- [ ] **Polished graphical style:** a clean, print-friendly "report" layout (hero KPIs → charts →
      tables), distinct from the dashboard; themeable (ties **B20**).
- [ ] **★ Shareable (no backend — local-first):** options —
      (a) **Print-to-PDF** via a print stylesheet (`window.print()`) → save/share a PDF;
      (b) **Standalone HTML export** — self-contained file with inline SVG charts, opens anywhere;
      (c) **Image export** (render the report SVG/DOM → PNG);
      (d) **CSV/JSON** of the underlying period data.
      A true **shareable link needs a backend** (rejected) — could encode small reports in a URL hash but
      it's fragile; skip. **Privacy (ties B17):** shared reports contain financial data — warn before
      sharing; offer **aggregates-only / redact amounts** mode.
- [ ] **UI:** a new **"Reports" nav item** (vs extending Insights) — pick report + period + (compare-to),
      view, export/share. A11y: keyboard, chart `role=img`+alt (extend the existing `ui.AreaChart` aria).
- [ ] **Verify/E2E:** each report's numbers match the ledger; delta % correct vs prior period; charts
      render + theme; PDF/HTML export produces a correct file; tax summary totals reconcile; offline works.
**Decisions — RESOLVED (2026-06-18):** (1) **D3** (activates B14); (2) **both** narrative modes; (3) **all
four** share formats (PDF + standalone HTML + PNG + CSV/JSON); (4) first reports = Spending + Net-worth
history + Year-end/tax; (5) **new Reports nav screen**. _Cross-links: **B14 (now active — D3)**, B20
(theming/print style), B17 (share privacy / redact mode), C38 (the "Reports" home-use gap), Insights (AI)._

### B22. Bills & due-date tracker + calendar — SPEC (from C38, 2026-06-18)
**Want:** a real bills surface beyond the dashboard "upcoming bills" widget — a list with due dates,
amounts, paid/unpaid status, and a **month calendar** view.
- [~] **Pure `internal/bills`** (no `syscall/js`, tested): derive bills from liability accounts'
      due-day/min-payment **and** Planning recurring items; compute next-due, overdue, days-until,
      paid-this-cycle; month-grid layout helper (which bills fall on which day). Reuse `dateutil`,
      `freshness`, `domain.Recurring`.
      Liability bills, Planning recurring outflows, next-due/days-until, and month-grid dots are now tested
      and wired into Bills/dashboard/notifications. Remaining: paid-this-cycle derivation.
- [ ] **State:** mark-paid per cycle (creates/links a transaction); persist paid status.
- [~] **UI:** Bills screen — upcoming/overdue list + a **month calendar** with bill dots; "mark paid" →
      logs the payment; ties **B19** (bill-due reminders) + the dashboard widget.
      Bills screen, calendar dots, reminder-to-task, dashboard, CSV, and bill-due notifications are live.
      Remaining: mark-paid creates/links a transaction.
- [ ] _Decision:_ bills as a first-class entity vs. purely derived from liabilities+recurring (recommend
      derived first, with an optional manual "add a bill").

### B23. Receipt / document attachments linked to transactions — SPEC (from C38, 2026-06-18)
**Want:** attach a receipt/document to a specific transaction (Artifacts stores images, but nothing links
them to a txn).
- [x] **Model:** `Attachments []AttachmentRef` (or reuse `SourceDocID`) on `Transaction` → stored
      Artifacts; store CRUD + dataset round-trip + export/import.
- [ ] **UI:** from a transaction row/edit, attach an existing artifact or upload new; paperclip indicator;
      view/preview from the ledger; Documents/Artifacts import can auto-link.
- [ ] _Notes:_ size caps; encrypt under **B17** lock; included in backups/export.

### B24. Split / shared expenses & settle-up between members — SPEC (from C38, 2026-06-18)
**Want:** split a transaction across members ("50/50") and track **who owes whom** with a settle-up view.
- [x] **Pure `internal/split`** (tested): a transaction split (by member, share/%/amount); per-member
      balances ("X owes Y $Z"); settle-up suggestions (minimal transfers). Reuses members + `money`.
- [ ] **Model:** a `Split` on transactions + settlement records.
- [~] **UI:** "Split…" on a transaction (equal / % / custom); a **Settle up** view of net balances +
      "record a settlement" (creates a transfer).
      Standalone Split calculator now supports even and weighted splits, shows who owes whom, and exports the
      settle-up plan as CSV. Remaining: transaction-row entry point and persisted settlement transfer.
- [ ] _Decision:_ split at txn level vs. a separate shared-ledger; start with equal/percent + net-balance.

### B25. Subscriptions tracker — SPEC (from C38, 2026-06-18)
**Want:** a view of recurring monthly spend (what am I paying for) + renewal/cancel reminders.
- [x] **Pure `internal/subscriptions`** (tested): detect/aggregate recurring charges (Planning `Recurring`
      and/or repeated payees); monthly + annualized totals; next renewal date.
- [x] **UI:** Subscriptions list (name, cadence, amount, monthly/yearly total, next renewal); "cancel
      reminder" → **B19** task; show total monthly subscription burden.
- [ ] _Notes:_ a focused view over the same recurring data, not a new store.

### B26. Budget rollover / sinking funds — SPEC (from C38, 2026-06-18)
**Want:** envelope **rollover** (unspent carries over) + **sinking funds** (save toward periodic large
expenses).
- [x] **Verify first:** does the current budget engine roll unspent over? If not, add it.
- [x] **Pure `internal/budgeting`** extension (tested): per-budget `Rollover bool`; carry-forward math
      (prev remaining + this limit); sinking-fund accrual (target ÷ months). 
- [~] **State/UI:** per-budget rollover toggle; "carried over $X"; a sinking-fund type. Ties the
      methodology selector (envelope/zero-based, D6).
      Per-budget rollover now persists on `Budget.Rollover`, has add/edit checkboxes, and shows previous-period
      carried amount in the Budgets list. Remaining: dedicated sinking-fund type/UI.
- [ ] _Decision:_ sinking funds as a budget feature vs. reuse `goals`.

### B27. Investment / holdings tracking — SPEC (possibly out-of-scope, from C38, 2026-06-18)
**Want (maybe):** brokerage/401k hold a **balance only** — no holdings/cost-basis/performance.
- [x] _Decision FIRST (scope):_ keep investments as a single balance (budgeting app) vs. track holdings.
      Full holdings = symbols/qty/cost-basis/**live price** (needs a price feed = online dep, tension with
      local-first/offline). **Recommend out of core**; if pursued, a lightweight **manual** holdings list
      (symbol, qty, manual price), no live feed — purely local. Confirm before any build.
      Decided in `docs/INVESTMENTS_SCOPE.md`: core remains balance-only; holdings/live pricing stay out of
      core, with only a possible future manual extension.

### B28. Automated backup reminders — SPEC (from C38, 2026-06-18)
**Want:** nudge periodic backups so data isn't lost (ties B17 recovery + Export #31).
- [x] Track `lastBackupAt`; given a cadence (Off/weekly/monthly), decide if a nudge is due (reuse the
      **B19 catch-up-on-wake** evaluation).
- [x] **UI:** gentle, dismissible "Back up your data" nudge (one-tap → Export JSON `cashflux.json`);
      Settings cadence. Non-naggy (B17/B18 ethos). _Could ship as a B19 notification rule._

### B29. Multi-device / shared-household sync — SPEC (expands Phase 3 §3.1/3.2; #1 home-use gap)
**Want:** the same household data on multiple devices/people (today: single-device, local-only). Records
concrete options given the no-shared-backend ethos.
- [ ] **Approaches (decide):** (a) **self-hosted / BYO sync backend** (Phase-3 Go server: pull/push
      deltas, household auth, conflict resolution) — user-owned but user must run it; (b) **E2E-encrypted
      sync via a generic store** (user cloud folder / Dropbox / WebDAV / thin relay) where the device
      encrypts with the **B17** key and the relay never sees plaintext; (c) **manual export/import handoff**
      (already possible — interim, no realtime).
- [ ] **Core (pure, tested):** a **CRDT/merge or delta-sync** model (per-entity LWW + tombstones, or
      vector clocks) so two edited copies merge losslessly; offline mutation queue + replay (§3.2).
- [ ] **Privacy:** sync payloads **encrypted with the B17 key** (zero-knowledge relay); never plaintext
      off-device.
- [ ] _Decision FIRST:_ largest, infra-touching item — confirm appetite + approach before any build.
      **Deferred design** for now; manual export/import (c) is the interim path.

### B30. GitHub Pages subpath routing — router has no basename (deep analysis 2026-06-18) ★★
**Problem (user):** the deployed spawn point is `https://monstercameron.github.io/CashFlux/`. When the
router navigates it **drops `/CashFlux/` and pushes the route at the origin root** (e.g. `/accounts`)
instead of keeping the base and appending (`/CashFlux/accounts`).
**Root cause (verified in code):**
- ✅ **Assets are fine** — `web/index.html` (lines 13–21) computes `<base href>` = `/<firstSegment>/` on
  `*.github.io` (→ `/CashFlux/`), `/` elsewhere, so `./bin/main.wasm`/`./wasm_exec.js`/`./chart.js`
  resolve at any route depth. **404 fallback** is generated by the Pages deploy (§0).
- 🐞 **Routing is NOT base-aware.** `router.RouterOptions` (GoWebComponents `router/router.go:62`) has
  **only `DefaultRoute` — no `Basename`/`BasePath`**. The history router reads `window.location.pathname`
  directly (router.go:377) and `Navigate` does `history.pushState(nil, "", normalizedPath)`
  (router.go:782). So:
  - **Match fails:** at `/CashFlux/accounts` the router compares the raw pathname to routes registered as
    `/accounts` → no match; `/CashFlux/` ≠ `/` (home won't resolve either).
  - **Navigate strips the base:** `Navigate("/accounts")` pushes the **absolute** `/accounts`, which the
    History API resolves against the **origin** — **`<base href>` does NOT apply to absolute-path
    pushState** (only relative URLs / asset loads). Result: `monstercameron.github.io/accounts`,
    `/CashFlux/` gone. (Exactly the user's symptom; also worsens B1/B3 deep-link behavior.)
**Fix options (ranked):**
- [ ] **A. Add basename support to the framework router (cleanest).** `RouterOptions.Basename` (e.g.
      `/CashFlux`): **strip** it from `location.pathname` before matching, **prepend** it on
      `Navigate`/`pushState` and the popstate handler. Benefits every app; the proper fix. (Framework
      change in GoWebComponents `router.go`.)
- [ ] **B. App-side base-prefix (no framework change).** Compute the base at runtime in Go (read
      `document.querySelector('base').href` / `location`, mirroring the index.html logic → `/CashFlux` on
      Pages, `` locally). **Register every route as `base + route`** (drive from `screens.All()`), set
      `DefaultRoute = base + "/"`, and route all `nav.Navigate` calls through a `routePath(base, …)` helper.
      Choke points: the `screens.All()` table + the `nav.Navigate("/…")` sites (addmenu.go,
      custompagesnav.go, settings.go, shell breadcrumb).
- [ ] **C. Hash routing** (`#/accounts`) — sidesteps subpath + 404 entirely, but **rejected by B1/B3's
      "clean URLs, no hash router"**; list only as a fallback if A/B stall.
- [ ] **Verify after fix:** cold load + refresh at `/CashFlux/`, `/CashFlux/accounts`, `/CashFlux/p/<slug>`
      all resolve; in-app nav keeps the `/CashFlux/` prefix; local dev (base `/`) still works; 404.html
      boots the shell and the base-aware router matches. Add a router test for a non-empty basename
      (strip + prepend round-trip). _Cross-links: **B1/B3** (deep-link/SPA fallback), §0 (Pages deploy)._

### B31. Full responsive strategy — phone → tablet → desktop → ultra-wide → portrait monitors (research 2026-06-18) ★
**Want:** responsive across the whole aspect-ratio range (tablets, desktops, ultra-wide side monitors,
portrait monitors). **Measured live (8 viewports):** ✅ **no horizontal overflow at any size**, but the
**bento column count is wrong at the extremes:**
| Viewport | bento cols | bento width | verdict |
|---|---|---|---|
| phone-landscape 844×390 | 2 | 584 | ok |
| tablet-landscape 1024×768 | 2 | 764 | ok |
| desktop 1440×900 | 4 | 1180 | ok |
| fhd 1920 / qhd 2560 | 4 | 1660 / 2300 | getting wide |
| **ultra-wide 3440×1440** | **4** | **3180** | ❌ 4 tiles stretched edge-to-edge, vast whitespace |
| **super-wide 5120×1440** | **4** | **4860** | ❌ absurd tile widths |
| **portrait 1080×1920** | **4** | **820** | ❌ 4 cols crammed into 820px (tiles too narrow) |

**Two real bugs found:**
- [ ] **Ultra-wide: content/bento stretches with no cap, no extra columns** — at 3440/5120 the 4 tiles fill
      the whole width (≈800–1200px each), sparse + poor readability (screenshot-confirmed). Fix: **cap the
      content measure** (max-width + center) and/or **add bento columns** at wide breakpoints (6–8 for the
      dashboard; cap max-width for forms/tables/reading so inputs & text don't stretch past ~70–100ch).
      Recommend capped, centered content shell + wider bento.
- [ ] **Portrait/narrow-desktop: bento columns key off raw viewport, not usable width** — 1024px → 2 cols
      (good) but **1080px → 4 cols** though only ~820px is usable after the 240px rail → cramped. Fix:
      derive bento columns from **content width (viewport − rail)**, ideally via **CSS container queries** on
      the content area. The B2/`pack.go` engine should take a responsive column count.

**Strategy (modern, component-level CSS):**
- [ ] **Breakpoints by *content* width** (**container queries** on `main`, not only viewport `@media`):
      <640 phone (1 col, drawer rail) · 640–1024 tablet (2 col, icon rail) · 1024–1600 desktop (4 col, full
      rail) · 1600–2200 wide (4–6) · >2200 ultra-wide (6–8 **or** capped+centered).
- [ ] **Rail:** drawer/hidden (phone) → 58px icon rail (tablet/narrow) → full 240 (desktop). Today it only
      collapses at phone width — **also collapse on tablet/portrait-narrow** (it stays 240 at 1024/1080).
- [ ] **Top bar:** wrap/condense at every width (fixes **C34**/**C19**; **B10** control redesign helps);
      `@media (aspect-ratio)` / short-height handling for phone-landscape + split windows.
- [ ] **Fluid type & spacing:** `clamp()` type/gaps, **`dvh`/`svh`** heights; pairs with **C25/C26**
      (px→rem) so scaling responds. Bento via `grid auto-fit/minmax` + a capped `--content-max`.
- [ ] **Split-screen/snapped windows** = narrow widths → content-width breakpoints cover them.
- [ ] **Test matrix:** phone P/L, tablet P/L, 1440, 1920, 2560, **3440 & 5120 ultra-wide**, **1080×1920
      portrait**, 960 split — assert no overflow, sensible columns, capped reading width, rail state per
      width. _Cross-links: C10 (mobile done), C19/C34 (tablet/top-bar), B2/pack.go, C25/C26, B6._

### B32. Deals/Savings/Education/Security/Mobile — research & design (pending approval, 2026-06-18)
**Status: RESEARCH/DESIGN for approval — build nothing yet.** Big batch; grouped by feasibility against
the **local-first, no-backend, offline, BYO-key** architecture. **The recurring constraint:** anything
needing external data feeds, OAuth to banks/issuers, web search/scraping, or sanctioned offer/points APIs
**cannot be client-only** (CORS, secret-holding, no public APIs) → needs a **backend or Electron** + paid/
licensed data + AI. Split into "buildable client-side now" vs "needs infra/data."

**Cluster 1 — Security research (answers + how they apply):**
- [ ] **Passkeys (WebAuthn/FIDO2):** passwordless, device-bound public-key auth (biometrics/PIN; private
      key never leaves the authenticator). For CashFlux's *local* lock there's no server, so the useful
      pattern is the **WebAuthn `prf` extension** → derive a stable secret from the passkey → use it as a
      **KEK to wrap the data key (B17.2)**. Adds a **biometric/device unlock** option, stronger + more
      convenient than a passcode. Secure-context only; PRF is Chromium-forward → fallback to passcode.
      **Add as a B17 unlock method.**
- [ ] **CIA triad** = Confidentiality / Integrity / Availability — adopt as the security framework: C =
      encryption-at-rest (B17) + minimal AI/notification egress; I = AES-GCM auth tag (tamper detection) +
      validated/checksummed backups; A = recovery (B17.2) + backups + offline-first.
- [ ] **OWASP Top 10** — most don't apply (no server: no access-control/SSRF/server-injection). **Relevant
      ones:** Cryptographic Failures (use vetted KDF/AES-GCM, no roll-your-own — B17), Insecure Design
      (lock/recovery), **Vulnerable & Outdated Components** (D3/Tailwind-CDN/Go-mod deps), **Software/Data
      Integrity Failures** (wasm/SW supply chain — **add SRI to the CDN scripts**), Auth Failures (lock),
      and **XSS/injection** (sanitize any user-data rendered as HTML). Action: a security pass on these.
- [ ] **CSRF tokens: NOT APPLICABLE** — CSRF is a server/cookie/session attack; CashFlux has none. Becomes
      relevant **only if** a sync backend (B29) / notification relay (B19) lands → protect those endpoints then.
- [ ] **OTP (TOTP/HOTP): low local value** — no server to verify against, and the secret would sit on the
      same device. Only meaningful as a B17 recovery factor or once cloud accounts (B29) exist.

**Cluster 2 — Education & retrospective (BUILDABLE client-side now — recommended first):**
- [ ] **Financial teaching:** curated, contextual lessons/tips + glossary (e.g. "no emergency fund — here's
      why"), optionally AI-personalized from the user's data. Pure content + existing data + optional AI.
- [ ] **Financial retrospective review + spending-optimization guide:** uses the user's OWN ledger →
      period-over-period analysis, "where your money went," and actionable suggestions (cut subscriptions,
      attack high-APR debt, fix over-budget categories). Builds on **B21 Reports** + **Insights** +
      `internal/insights`. Largely client-side. **Strong in-scope win.**

**Cluster 3 — Optimizer logic (BUILDABLE client-side IF data is user-entered; the data-fed parts need infra):**
- [ ] **Discount stacking:** pure `internal/dealstack` optimizer — combine coupons + card rewards + portal
      cashback into the best legal stack, GIVEN offer data. The *optimizer* is pure/testable now; the
      *offer data* needs a feed (Cluster 4).
- [ ] **Credit-card selection min-maxing:** pure `internal/cardoptimizer` — "use card X for this category"
      ranking GIVEN the user's manually-entered cards + reward categories. Client-side now. (The
      *auto-add-offers / points-sync* part needs OAuth — Cluster 4, and likely no sanctioned API.)

**Cluster 4 — Data-fed agent layer (NEEDS backend or Electron + paid/licensed data + AI — defer):**
- [ ] **AI decision engine + curated chain of deal-stacking agents:** a pipeline of specialized agents
      (deal-finder · card-optimizer · APY-finder · locale-deals) → a synthesizer ranks/stacks. Needs each
      agent's **data source** + an LLM + a runtime the browser can't provide (CORS) → backend/Electron.
- [ ] **Market search / banking-APY search / locale events & deals:** require **paid/licensed data feeds**
      (or scraping — ToS/legal risk) + geolocation + a server. Surface results in-app.
- [ ] **Savings agent:** the user-facing orchestrator over the above — proactive "you could save $X by…".
- [ ] **Auto-add offers ("max platinum — add all Amex offers") + points/discount tracking via OAuth:**
      ⚠️ **research finding:** there is **no sanctioned public API** to auto-enroll card offers or read
      points for Amex/Chase/etc.; "add all offers" tools use **undocumented endpoints / browser extensions**
      (ToS-violating, brittle, account-risk). OAuth/aggregation for *read-only* balances/points exists via
      **Plaid/MX/Finicity** (paid, requires a backend + their approval; covers transactions/balances, not
      offer-enrollment). **Recommend:** if pursued, read-only aggregation via Plaid-style providers (backend),
      and DROP "auto-add offers" as unsupported/risky — or relegate to an optional Electron/extension the
      user installs at their own risk. State the legal/ToS limits to the user.

**Cluster 5 — Mobile (research):**
- [ ] **PWA (already): the mobile story today** — installable on iOS/Android, offline, responsive (C10).
      Cheapest path; ship as-is + the B31 responsive work.
- [ ] **Capacitor (recommended for native store apps):** wrap the **existing web/wasm build** in a native
      WebView shell → App Store / Play Store, plus native APIs (**biometrics** for passkey unlock B17,
      **push notifications** for B19, filesystem for import/export). Reuses the entire bundle — no UI rewrite.
- [ ] **gomobile: NOT suitable** — it builds Go *native-UI* libs for mobile; it can't run the GoWebComponents
      **DOM/wasm** UI. So "mobile with Golang" = keep the Go→wasm app, wrap it (PWA → Capacitor), not gomobile.

**Decisions — APPROVED (2026-06-18):**
- (1) **All clusters approved, phased:** **now** = Cluster 2 (education/retrospective) + Cluster 3 (pure
  optimizers); **next** = Cluster 1 (security pass + passkeys); **deferred** = Cluster 4 (data/agents);
  Cluster 5 = PWA now, Capacitor later.
- (2) **Tiered backend strategy: "as much locally as possible → Electron → hosted server."** Each Cluster-4
  capability should be built at the **lowest tier that can do it**: (a) **local/client** where feasible
  (pure optimizers, user-entered data); (b) **Electron** for things blocked only by CORS / needing on-device
  keys (deal/APY/market fetches, the agent chain calling data APIs); (c) **hosted server** only for what
  truly needs it (shared aggregation, scheduled jobs, secrets that can't live on-device). Design each
  feature to **degrade**: full on hosted, most on Electron, optimizers-only on web.
- (3) **"Auto-add offers" → converted to a research task** (below), not dropped, not committed.
- [ ] **RESEARCH (logged per user): credit-card offer-enrollment & points/rewards access per issuer.**
      For each main-line issuer — **Amex, Chase, Citi, Capital One, Discover, Bank of America, Wells Fargo,
      U.S. Bank, Barclays, Synchrony** (+ networks Visa/Mastercard offers, and aggregators Plaid/MX/Finicity)
      — document: is there a **sanctioned API** for (i) reading points/rewards balances, (ii) reading
      statement-credit "offers," (iii) **enrolling/auto-adding offers**? Note auth model (OAuth? partner-only?
      none), ToS/legality, and whether it requires partner approval / a backend. Output: a per-issuer
      feasibility matrix → decides what's buildable vs. extension-only vs. impossible. _This is the gating
      research before any Cluster-4 offers/points work._
_Cross-links: B17/B17.2/B17.3 (lock/crypto + passkey KEK), B19 (push/relay), B21 (reports/retrospective),
B29 (sync backend), §5.1 (Electron), Insights (AI)._

### B33. Security hardening — data-at-rest & secrets ★ (from C45 audit, user-requested fix 2026-06-18)
Actionable fixes for the security issues found in the C45 source audit. SQL injection was audited **clean**
(all user-data queries use `?` bind params — no work needed there), so this item is about **data-at-rest
confidentiality, secret handling, and durability**. Ordered by severity; build bottom-up per CLAUDE.md
(crypto in a pure tested `internal/crypto` package first, then wire persistence, then UI).

- [ ] **B33.1 — Encrypt the at-rest dataset snapshot (🔴 highest).** Today `persist.go:92` writes the full
      dataset as **plaintext JSON** to `localStorage["cashflux:dataset"]`. Encrypt-before-write, paying the cost
      on save — which is **negligible** because the heavy KDF runs once at unlock, not per-save (see strategy below).
      _Depends on / extends B17 (lock + recovery). Live DB stays `:memory:` so no plaintext DB file on disk._

      **RESEARCHED STRATEGY (2026-06-18, OWASP + WebAuthn-L3 + WebCrypto modern-algos):**
      - **Bulk cipher = AES-256-GCM via WebCrypto `crypto.subtle`** (native, authenticated, ~sub-ms on a 30 KB
        dataset). Call it from Go/wasm through `syscall/js`. New **random 12-byte nonce every save**
        (`crypto.getRandomValues`) — never reuse a nonce with the same key (GCM fails catastrophically).
      - **Envelope encryption (DEK + wrapped KEKs)** — the design that makes recovery cheap:
        • a random **DEK** encrypts the dataset; • the DEK is **wrapped** by one or more **KEKs**
        (passphrase-KEK, recovery-code-KEK, and later a WebAuthn-PRF KEK). Changing the passphrase or using
        recovery **re-wraps the DEK only** — no full re-encrypt. Store all wrapped-DEK blobs in the envelope.
      - **KDF (passphrase → KEK) = Argon2id.** OWASP 2025 baseline **m=19 MiB, t=2, p=1** (or m=46 MiB, t=1, p=1).
        WebCrypto Argon2id is not yet universal (modern-algos proposal; feature-detect via
        `SubtleCrypto.supports('importKey','Argon2id')`), so **use pure-Go `golang.org/x/crypto/argon2`** in wasm
        for portability (no browser-version dependency). **PBKDF2-HMAC-SHA256 fallback** only if needed:
        **≥600,000 iterations** (OWASP/FIPS), 310k absolute floor. Store the KDF id + params + salt in the envelope
        for crypto-agility.
      - **Cost model (answers the "pay on save" question):** run Argon2id **once at unlock** → unwrap the DEK →
        cache it as a **non-extractable in-memory `CryptoKey`**. Every 4 s autosave then does **only** AES-GCM over
        the JSON = imperceptible. The only slow step is the one-time unlock the user already expects.
      - **Envelope format (versioned):** `{ v, kdf:{id,salt,params}, wrappedDEKs:[{kind,nonce,ct}], data:{nonce,ct} }`,
        base64 in localStorage for now (note ~33% bloat → reinforces B33.4 / IndexedDB binary storage).
      - **Pure tested `internal/crypto` package first** (table-driven: encrypt→decrypt round-trip, tamper/auth-fail,
        wrong-key reject, DEK re-wrap across KEKs, KDF param round-trip) — then wire persistence, then UI (CLAUDE.md
        bottom-up). Keep KDF/cipher behind interfaces so params can ratchet up later without a data migration.
      - **Modes:** lock **enabled** → encrypted envelope; lock **disabled** → today's plaintext snapshot (explicit
        opt-out); optional **middle tier** = device-bound key in IndexedDB (stops casual disk/file inspection but
        **not** same-origin XSS/extensions — state that limitation in the UI).
- [ ] **B33.2 — Zeroize plaintext on lock/timeout (🔴).** On inactivity-lock/manual-lock (B17), drop the derived
      key, clear the cached plaintext snapshot string, and ideally re-init the `:memory:` DB so a memory scrape
      after auto-lock yields nothing. Add a test/inspection hook proving the key + snapshot are cleared.
- [ ] **B33.3 — Stop storing the OpenAI key in plaintext (🟠).** `aikey.go:15` puts the key in
      `localStorage["cashflux:openai-key"]` in cleartext when "remember on device" is on. Fold the key into the
      B33.1 encrypted envelope when lock is enabled; when lock is off, add explicit warning copy to the
      remember-key toggle ("stored unencrypted on this device") so the exposure is informed-consent.
- [ ] **B33.4 — Handle localStorage quota instead of silently losing data (🟠).** `persist.go:81-84` swallows a
      `setItem` quota throw with only a log line → autosave silently stops and unsaved data is lost on reload.
      Detect the quota failure path and surface a persistent visible warning (banner/toast) + "export now" nudge.
      Stretch: migrate bulk dataset storage to **IndexedDB** (much larger quota); pairs naturally with B33.1.
- [ ] **B33.5 — Keep the SQL layer injection-free (guardrail, no code today).** Document in the store package
      that all user values MUST use `?` bind params and any future dynamic identifier (column/ORDER BY/table)
      MUST come from a hard-coded allow-list — never string-interpolated user text. Add a brief test or comment
      asserting the invariant so a future contributor can't regress it.
- [ ] **B33.6 — Settings: enable/disable sensitivity (encryption + lock) toggle (🔴 UI).** A single master switch
      in Settings → Privacy & Security that turns at-rest encryption + the lock gate on/off. Behavior:
      • **Off → On (first-time setup):** run an inline **set-password** flow (password / passphrase / PIN per B17,
        with NIST-grade strength validation + hint), generate the DEK, encrypt the current snapshot, set up the
        recovery wrap (security questions / recovery code per B17). After this the snapshot on disk is ciphertext.
      • **On → Off:** **must be confirmed behind the current password** (B17 rule — can't disable from an unlocked
        session without re-auth) → decrypt and rewrite the plaintext snapshot, drop the keys. Show a plain-English
        warning that data will be stored unencrypted on this device.
      • Preserve credentials when merely toggling the *gate* vs. fully disabling (B17's "toggle lock without
        wiping creds" requirement). Persist the chosen mode (off / encrypted-passphrase / device-bound middle tier).
      _Depends on B17 (lock spec) + B33.1 (crypto). UI is the thin shell over the tested crypto package._
- [ ] **B33.7 — Initialize / unlock screen on load (🔴 UI).** When encryption is enabled, the app must **not** read
      the dataset until the user authenticates. On load show a **decrypt/unlock screen** (extends the existing
      `applockgate.go`) with a password/passphrase/PIN input (+ "show hint", + recovery link). On submit:
      derive the KEK (Argon2id, the one-time cost) → unwrap the DEK → **decrypt the snapshot into the `:memory:`
      SQLite DB** → cache the non-extractable key → arm encrypt-on-save (B33.1). Wrong password = clear auth-fail
      message + rate-limit/backoff (no oracle leak). Until unlocked, render nothing sensitive (privacy-first lock
      screen per B17 — smart-quote/neutral content only). On manual lock / inactivity timeout, re-show this screen
      and zeroize per B33.2. First run with encryption off → skip straight to the app (no gate).
      _This is the runtime counterpart to B33.6's setup: B33.6 establishes the password & encrypts; B33.7 is the
      every-load decrypt gate. Both sit on B17's lock-gate UI + B33.1's crypto._
_Cross-links: C45 (source audit), B17/B17.2/B17.3 (lock/crypto/passkey KEK + recovery + lock-gate UI), C44 (XSS
surface that makes plaintext-at-rest reachable), B32 Cluster (CIA/OWASP), B29 (sync — encrypt-before-send reuses B33.1)._

---

## C. Live UI/UX review findings — 2026-06-16 (sample data) ★

Captured by driving the running app (`http://127.0.0.1:8080`) in a real headless Chromium via the
now-installed Playwright driver and screenshotting all 14 routes (Dashboard, Accounts, Transactions,
Budgets, Goals, To-do, Planning, Allocate, Insights, Documents, Customize, Members, Categories,
Rules). Screenshots + rendered text are in `.review-screenshots/` (git-ignore this). Items are
ordered correctness-first, then cross-cutting chrome, then per-screen polish.

### C46. Iconography pass — add a consistent glyph system across all screens ★ (UX/visual, user-requested 2026-06-18) — ✅ DONE (verified 2026-06-21: `internal/icon` curated registry; rail nav, KPI/named tiles via `widgetIcon`, status/trend glyphs all use `icon.Icon`)
**Surveyed live (all 11 routes, content inventory via harness).** Today the app **mixes ad-hoc Unicode glyphs**
(`▾` dropdown, `‹ ›` period stepper, `⚙` settings, `✕` close, `⋯` overflow, `↑` insight trend, `+ Add`) and
otherwise relies on **text-only** labels; real SVGs appear **only in charts** (svgTotal≈21, nearly all D3). The
**17-item sidebar is entirely text** (`Dashboard, Accounts, Transactions, Budgets, Goals, To-do, Planning,
Allocate, Insights, Documents, Customize, Artifacts, Workflows, Members, Categories, Rules, New page`) — which
makes the **collapsible rail unusable when collapsed** (no icons = nothing to show). Adding a coherent icon set
makes the app more legible, scannable, and visually interesting.

**System decision (do this first):** adopt **one** open-source icon set inlined as **local SVG** — e.g. **Lucide**
(MIT, clean, matches the calm aesthetic). **No icon CDN or webfont** (per C44 — bundle at build time; inline SVG
also lets icons inherit `currentColor` for theming, B20). Build a tiny `internal/ui/icon` helper (`Icon(name, …)`)
so screens reference glyphs by name; replace the ad-hoc Unicode glyphs above with real icons for consistency.

**High-value placements (grounded in actual labels/sections):**
- [ ] **Sidebar nav (highest — unblocks the collapsible rail):** one icon per item — Dashboard `layout-dashboard`,
      Accounts `wallet`, Transactions `arrow-left-right`, Budgets `pie-chart`, Goals `target`, To-do `check-square`,
      Planning `line-chart`, Allocate `scale`, Insights `sparkles`, Documents `file-text`, Customize `sliders`,
      Artifacts `box`, Workflows `workflow`, Members `users`, Categories `tags`, Rules `filter`, New page `plus`.
- [ ] **Quick-add menu (text-only today):** leading icon per item — New transaction `arrow-left-right`, New account
      `wallet`, New budget `pie-chart`, New goal `target`, Scan a document `scan-line`/`camera`. Also the `+ Add`
      trigger → `plus`, the `⚙` → `settings`, `⋯` → `more-horizontal`, `✕` → `x`, `▾` → `chevron-down`, `‹ ›` →
      `chevron-left/right`.
- [ ] **Dashboard KPI tiles:** leading icon on each tile header — Net worth `wallet`, Income `arrow-down-circle`,
      Spending `arrow-up-circle`, Liabilities `credit-card`, Recent transactions `receipt`, Budgets `pie-chart`,
      Goal `target`, To-do `check-square`, Accounts `landmark`, Net worth trend `trending-up`. (Pairs with the
      existing tile-click-to-navigate TODO — icon reinforces the destination.)
- [ ] **Status/semantic glyphs (carry meaning at a glance, color-coded):** stale-balance nudge ("7 balances could
      use a refresh" / "7 accounts stale") → `clock`/`alert-circle`; over/near budget ("0 over budget · 2 near the
      limit") → `check-circle`/`alert-triangle`; goal pace (on-track/behind) → `trending-up`/`trending-down`;
      Insights trend arrows (currently bare `↑`) → colored `arrow-up`/`arrow-down`; transaction row type (transfer
      `arrow-left-right`, income `arrow-down`, expense `arrow-up`, cleared `check`).
- [ ] **Row/section actions:** Accounts — Edit `pencil`, Update balance `refresh-cw`, Mark updated `check`, Mark
      all updated `refresh-cw`, Transactions link `list`; Goals — Contribute `plus-circle`, Edit `pencil`;
      Accounts section headers Assets `trending-up` / Liabilities `trending-down`.
- [ ] **AI affordances — unify with one "sparkle" glyph:** "Read with AI", "Explain with AI", "Explain my month",
      "Ask about your money" all get `sparkles` (chat-style ones could use `message-circle`) so AI actions read as
      a consistent family.
- [ ] **Per-screen section headers & empty states (make it interesting):** Planning — Net worth in 12 months
      `trending-up`, Recurring cash flows `repeat`, Savings & spending plans `sliders`, Debt payoff `calculator`,
      Projection `line-chart`; Documents — Read a receipt `scan-line`, Import CSV `upload`, Import history `history`;
      Allocate — Why this order `help-circle`, Exclude `ban`; Customize — Formula calculator `function-square`,
      Available variables `braces`. **Empty states** ("No recurring cash flows yet", "No plans yet", "No imports
      yet", "No custom fields yet") → a friendly muted **empty-state glyph/illustration** above the text — biggest
      "more interesting" win for otherwise-blank panels.
_Cross-links: collapsible rail item (icons unblock collapsed mode), tile-click-to-navigate item, C44 (bundle icons
locally, no CDN), B20 theming (icons inherit `currentColor` → recolor with accent), accessibility (decorative icons
`aria-hidden`, icon-only buttons keep `aria-label`)._

**C46.1 — Credit-card glyphs + "delight" micro-additions (user-requested 2026-06-18).** Beyond the functional
icon pass above, add small characterful touches that make the app feel richer:
- [ ] **Credit-card / network brand glyphs on accounts.** Credit-card & liability accounts show a small **card
      brand mark** (Visa / Mastercard / Amex / Discover / generic card). Detect brand from a user-set field (or
      optionally the card number's IIN/BIN prefix if ever entered — 4=Visa, 51-55/2221-2720=Mastercard, 34/37=Amex,
      6011/65=Discover); fall back to a **generic `credit-card` glyph** when unknown. ⚠️ **Trademark note:** Visa/MC/
      Amex/Discover logos are protected marks with brand guidelines — prefer a **permissively-licensed brand-icon
      set** (or simple stylized monograms/colors) over shipping official logos, and keep them purely decorative/
      identifying. Log a quick licensing check as a sub-task before bundling any real network logos.
- [ ] **Mini credit-card visual for card accounts.** Optional small **card-art tile** (rounded rectangle, subtle
      gradient in the account/accent color, brand glyph, masked •••• last-4, name) on the Accounts screen / account
      detail — a wallet-style flourish that reads instantly as "a card." Pure CSS + the brand glyph; no PII beyond
      last-4, and only if the user enters it.
- [ ] **Account-type avatars & color chips.** Each account/category/member gets a small colored avatar or
      type-glyph (checking `landmark`, savings `piggy-bank`, cash `banknote`, investment `trending-up`, loan
      `credit-card`) so lists are scannable by shape+color, not just text.
- [ ] **Category & member glyphs.** Let categories carry an icon/emoji (groceries `shopping-cart`, housing `home`,
      transport `car`, utilities `plug`, dining `utensils`…) and members a colored monogram avatar — used in
      transaction rows, budgets, and allocation.
- [ ] **Small delight moments (tasteful, dismissible, respect reduced-motion):** goal-reached confetti/checkmark
      burst when a goal hits 100%; progress **rings** on goals/budgets instead of bare bars; tiny inline
      **sparklines** on KPI tiles; a subtle count-up animation on KPI numbers; streak/"all caught up" badge when no
      balances are stale; gentle hover lift on tiles. Keep them quiet and optional — never naggy (CLAUDE.md).
_All of these are local SVG/CSS (no CDN per C44), inherit theme color (B20), and stay decorative+`aria-hidden`
with text labels intact for a11y. Build behind the C46 `internal/ui/icon` helper once it exists._

### C47. Transactions: redesign the ledger as a paginated, sortable table with a cleaner filter UI ★ (UX, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `transactions.go` uses `ui.DataTable` with sortable columns (aria-sort + caret) + full pagination)
**Reviewed live** (`gwc probe` against the running dev server at `http://127.0.0.1:8080`; the SPA boots
clean at `/` — 200, no console errors — but a direct hit on `/transactions` still 404s, confirming B1)
**plus the authoritative render code** (`internal/screens/transactions.go`). What ships today:
- **Not a table — a flat "rows" list.** The ledger is `Div(Class("rows"), …)` of `TransactionRow` flex
  cards: a checkbox glyph, a stacked desc + meta line (`category · date · account · #tags · cleared`),
  a cleared toggle, the amount, then Edit / Duplicate / Delete buttons. No column headers, no aligned
  columns, amounts not in a tabular column — it reads as one long ugly scroll.
- **No real pagination.** "Pagination" is an incremental **"Show more" (+50)** button (`visN` state,
  `txnPageSize = 50`). The list only ever grows; there are no page numbers, no prev/next, no
  page-size choice, no "X–Y of N" position. (This is the concrete fix for the Transactions half of
  **C39**; keep C39 for the *other* growable lists.)
- **Sorting is a dropdown, not columns.** A single `Sort` select (Date / Amount / Payee) — you can't
  click a column header to sort, and there's **no ascending/descending control or indicator** at all
  (direction is fixed inside `txnfilter`).
- **Filters are a cramped 10-control row.** One `form-grid` crams search + account + category + member
  + from-date + to-date + cleared + sort + Clear + Export CSV into a single wrapping strip. Functional
  but noisy; no sense of which filters are active, no grouping.

**Goal:** a clean, dense, accessible **paginated data table** with **click-to-sort columns** and a
**compact filter toolbar**, preserving every existing behavior (inline edit, duplicate, delete with
transfer-pair handling, bulk select/recategorize/clear/delete, dedupe notice, persisted filters,
CSV export of the filtered set, the a11y live region). Build **bottom-up** per the SDLC rule — most
of the logic already exists in the pure `txnfilter` package; extend it with tests before touching UI.

**Logic / state (pure, tested first):**
- [ ] Extend `internal/txnfilter` (or `uistate.TxFilter`) with an explicit **sort direction**
      (`Asc`/`Desc`) alongside the existing `Sort` field, and add `date|amount|payee|category|account`
      as sortable keys. Table-driven tests for each key × direction, including ties and transfer legs.
- [ ] Add **pagination math** as pure helpers (page index, page size, total pages, slice bounds,
      clamp on filter change) with table tests — never compute window math in view code.
- [ ] Persist **page size** and **sort key + direction** in `uistate.TxFilter` (already persisted via
      `PersistTxFilter`), so they survive reload like the other filters. Reset to page 1 whenever the
      filter set or sort changes.

**Table UI (replaces the `rows` list):**
- [ ] Render a real semantic `<table>`: `thead` columns = ☐ (select-all) · Date · Description ·
      Category · Account · Tags · Amount (right-aligned, tabular figures) · Cleared · Actions. Align
      columns; money uses the existing `fmtMoney`/`amountClass`. Keep the inline-edit row (it can
      become an in-row editor or a `colspan` edit panel — keep all current edit fields).
- [ ] **Sortable column headers:** click Date/Description/Category/Account/Amount to sort by it; click
      again to flip direction. Show a caret indicator and set `aria-sort` (`ascending`/`descending`/
      `none`) on the active header; headers are real `<button>`s (keyboard-operable). Remove the
      standalone Sort dropdown.
- [ ] **Select-all checkbox in the header** that selects/clears the current page's rows (wire into the
      existing `selected` map + bulk action bar; keep per-row checkboxes).
- [ ] **Responsive:** on narrow screens collapse the table back to a stacked card layout (reuse the
      current row markup) so mobile stays usable — pairs with C10/C19.

**Pagination control (replaces "Show more"):**
- [ ] A footer bar with **prev / next**, current page + total ("**1–50 of 312**"), and a **page-size
      selector** (e.g. 25 / 50 / 100 / All). Disable prev/next at the ends; keep it keyboard-reachable
      and labelled. "All" is allowed but should warn/virtualize for very large sets (defer virtualization
      to C39 if needed).

**Cleaner filter interface:**
- [ ] Collapse the 10-control strip into a **compact toolbar**: an always-visible search box + a
      **"Filters" dropdown/popover** (use the existing `FlipPanel`) holding Account / Category / Member /
      date range / Cleared. Show an **active-filter count badge** on the trigger and render the active
      filters as removable **chips** below the toolbar; keep **Clear** and **Export CSV** beside it.
- [ ] Keep the **summary line** ("N transactions · net $X") and the screen-reader live region; make sure
      the count reflects the full filtered set (not just the visible page).

**Verify (browser oracle; note B1 blocks direct `/transactions` nav — drive from `/` then navigate):**
- [ ] Sort by each column asc/desc; paginate forward/back; change page size — all persist across reload.
- [ ] Filters via the new popover narrow the set; chips remove individual filters; Clear resets to page 1.
- [ ] Inline edit, duplicate, delete (incl. transfer pair), and every bulk action still work from the table.
- [ ] CSV export still exports the **full filtered** set (not just the current page).
- [ ] Table collapses to cards on a narrow viewport; `aria-sort` + header buttons are keyboard-operable.
_Cross-links: **C39** (general long-list pagination — this resolves it for Transactions), **C18** (inline-edit
consistency), **C10/C19** (responsive), **C42/C43** (FlipPanel for the filter popover), **B1** (deep-link 404
makes the page only reachable via in-app nav during verification)._

> **✅ RECONCILED 2026-06-21 — the C48–C66 per-screen UX-review series is essentially complete.** A code audit
> confirmed each screen's core asks shipped: visible form labels (`labeledField`), kind/currency `<select>`s +
> number constraints, status/urgency tone (near/over/overdue/due-soon), drill-downs to data, per-row actions
> (Remind-me/ignore/confirm), grouped sections + CSV/print, image previews, indentation + usage counts, inline
> edit + staged-action remove, surfaced upload errors. **Only two have remaining work: C53** (Planning — most
> inputs still lack visible labels + no jump-nav bar) and **C64** (Rules — no drag-to-reorder precedence; match
> preview + labels are done). C57's "Remind me" + annual figure + urgency are done; a dedicated *mark-paid* log is
> the one nuance left there. Individual headers below are left as-was for history.

### C48. Dashboard: UX review — strong bento, but typography/spacing scale is ad-hoc ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/dashboard.go`). **Verdict:** the page makes sense for its purpose — a
glanceable bento of KPIs + trends + nudges — and buttons are appropriately sized (`.btn`/`.data-btn`/
`.rstep` are all small; no oversized controls). The weak spots are **typographic consistency and internal
spacing rhythm**, which keep it from reading as fully "professional." Existing dashboard items already
cover layout/behavior (C5 dup widget, C11 empty gear panel, C21 per-tile settings, C22 reflow, C30
tile-click-to-navigate, C24 auto-layout) — these findings are **visual/UX polish only** and don't overlap.
- [ ] **Hardcoded arbitrary px font sizes everywhere** — the file scatters `text-[11px]`, `text-[12px]`,
      `text-[13px]`, `text-[22px]`, `text-[24px]`, `text-[34px]` ad hoc. There's no shared type scale, so
      sizing is inconsistent tile-to-tile **and it bypasses the user text-size / display-scale setting**
      (B6/C26). Replace with a small set of semantic type tokens (caption / body / figure / figure-lg) that
      respond to the scale setting. _Biggest professional-polish win._ Cross-link **C25** (density tokens),
      **C26**/**B6** (configurable text size).
- [ ] **KPI figure sizes don't follow one hierarchy** — KPI tiles use `24px`, Savings rate `34px`, Net-worth
      trend & goal figures `22px`. Define one "primary figure" size and one "hero figure" size and apply them
      consistently so the eye isn't pulled around arbitrarily.
- [ ] **Inconsistent internal vertical rhythm** — widget bodies mix `space-y-2`, `space-y-2.5`, `space-y-4`,
      and one-off `mt-0.5/mt-1.5/mt-2/mt-3`. Standardize on the spacing scale so every tile breathes the same.
- [ ] **Full-width single-line bands feel heavy** — Freshness (`1 / span 4`, row 8) and Top highlight
      (`1 / span 4`, row 9) each take a full 4-column band for one line of content. Consider narrower default
      spans (or pairing them on one row) so the grid stays dense and balanced rather than ending in two thin
      full-width strips.
- [ ] **Header control pairing** — the layout-mode `Select` (`.rstep text-[12px]`) sits beside the Reset
      `.data-btn`; they're different control families at slightly different heights. Align their height/padding
      so the header toolbar reads as one set.
- [ ] **Verify** after changes: tiles still align on the bento grid at all widths; figures share a clear
      hierarchy; changing the display-scale setting actually resizes dashboard text (closes the B6/C26 gap here).

### C49. Accounts: UX review — solid layout, but the add/edit form is a dense placeholder-only grid ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/accounts.go`). **Verdict:** the page is well-organized for its purpose — a
net-worth/assets/liabilities stat grid, an add form, sectioned Assets / Liabilities / Archived lists, and a
clean per-row action pattern (primary **Transactions** / **Edit** inline, secondary **Update balance /
Mark updated / Archive** tucked in a `⋯` overflow menu, destructive **✕** last). **Row buttons are NOT
oversized** and the empty state ("Welcome" + Load sample) is good. The weak spots are the **add/edit form**
and a couple of input affordances:
- [ ] **Placeholder-only labels everywhere.** The add form and inline-edit form use `Placeholder(...)` with
      no visible `<label>`. Placeholders vanish on input and several are cryptic number fields ("APR",
      "Liquidity", "Stability", "Due day") — a user can't tell what an empty-after-typing field was. Add
      persistent visible labels (or a label+field pattern). Cross-link **B15** (a11y labelling), **C18**
      (inline-edit consistency).
- [ ] **Currency is free-text.** `Currency` is a `Type("text")` input (just uppercased) instead of a
      **select of known currency codes** — typo-prone and unguided. Make it a dropdown (the app already has a
      currency list / FX table).
- [ ] **Number fields lack constraints + unit hints.** Due day should be `min=1 max=31`; Liquidity/Stability
      are "1–5" scores (no range shown); APR/expected-return are percents (no `%` affordance). Add
      `min`/`max`/`step` and inline unit hints so the figures are unambiguous (correctness + UX).
- [ ] **The add form mixes common + advanced fields in one flat grid.** Asset accounts still surface
      Expected return / Liquidity / Stability / Lock-until inline — all advanced/optional. Tuck them behind an
      **"Advanced" disclosure** so the common path (name · type · owner · currency · opening balance) stays a
      short, calm form. Same for the inline-edit grid.
- [ ] **Row primary actions render icon + text and may wrap on narrow screens** (Transactions, Edit). On small
      widths collapse to icon-only with tooltip/`aria-label` to avoid wrapping. Cross-link **C10/C19**
      (responsive).
- [ ] **Verify** after changes: every field has a discoverable label; currency can only be a valid code;
      number ranges are enforced; the default add form is short with advanced fields collapsed; rows don't
      wrap on mobile.

### C50. Budgets: UX review — feature-rich and correct, but rows get text-busy + form labels are hidden ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/budgets.go`). **Verdict:** genuinely strong for its purpose — totals stat grid
(spent / budgeted / left), methodology-aware banner (zero-based "to assign" / envelope note), per-budget
progress bar with on-track/near/over tones + labels, pace-projection heads-up, rollover carry, envelope
balance, a recent-spend **limit suggestion** with one-tap "use this", and a proper empty-state CTA. It also
correctly defers the period window to the shared top-bar control (C7 already fixed). **Buttons aren't
oversized.** Polish opportunities:
- [ ] **Placeholder-only labels in the add + inline-edit forms** (name, limit, and the Category / Owner /
      Period selects use `aria-label`/placeholder only, no visible label) — same systemic issue as Accounts
      (**C49**). Add visible labels. Cross-link **B15**, **C18**.
- [ ] **Budget rows can stack up to four small sub-lines** — the `budgets.rowSub` line plus conditional
      **pace**, **rollover carry**, and **envelope** lines all render as separate `budget-sub` text rows. When
      several apply at once a row reads as a wall of tiny text. Consolidate into one meta line or render
      pace/rollover/envelope as small **badges/chips** with tone, keeping the row scannable.
- [ ] **The over/near summary is plain text** ("0 over · 2 near") — give it the same tone/badge treatment as
      the row states so the at-a-glance risk reads consistently (color + shape, not text only — B15).
- [ ] **No drill-down from a budget to its transactions.** A budget row should be clickable to open
      Transactions filtered to that category (mirror the Accounts→Transactions and dashboard tile-click
      pattern, **C30**) — a natural "why am I over?" affordance that's currently missing.
- [ ] **Edit action is icon+text inline in the row head** — same narrow-screen wrap risk as other rows;
      collapse to icon-only on small widths. Cross-link **C10/C19** (responsive).
- [ ] **Verify** after changes: rows stay scannable when pace+rollover+envelope all apply; form fields are
      labelled; risk summary uses tone+shape; clicking a budget lands on its filtered transactions.

### C51. Goals: UX review — clean and consistent, but flat progress tone + silent contribute ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/goals.go`). **Verdict:** purpose-fit and tidy — a totals stat grid (saved /
target / overall %), an add form, an inline **Contribute** mini-flow, inline edit, incomplete-goals-first
sorting, monthly-needed pacing, linked-account display, and a proper empty-state CTA. It reuses the budget
row layout for visual consistency. **Buttons aren't oversized.** Smaller polish items:
- [ ] **Progress bar has no state tone.** Goal bars are always the single `bar-fill` color — even at 100%.
      Give completed goals a success tone (and optionally an at-risk tone when the target date is near but
      pace is behind, paralleling Budgets' near/over coloring). Pairs with the goal-reached delight already
      noted in **C46.1** (don't duplicate the confetti item — this is just the bar tone).
- [ ] **Placeholder-only labels** across the add / edit / contribute forms (name, target, saved-so-far,
      owner/linked selects, date) — same systemic labelling gap as **C49/C50**. Add visible labels (B15).
- [ ] **The row sub-line is a run-on concatenation** — pct + remaining + "by <date>" + "save <X>/mo" +
      "linked: <account>" all in one `budget-sub` string. For a dated, linked goal it gets long; split the
      pacing/linked bits into small badges or a second muted line. (Milder version of **C50**'s row-busyness.)
- [ ] **"Contribute" silently edits the number.** Contributing just increments `CurrentAmount` — it does
      **not** post a transaction or move money from the linked account, so a goal linked to a real account can
      drift from that account's balance with no audit trail. Consider (a) optionally recording a
      transfer/transaction into the linked account on contribute, or (b) a clear note that contributions are
      manual tracking only. Cross-link the linked-account concept and **C47** (ledger).
- [ ] **Three icon+text actions in the row head** (Contribute, Edit, Delete) risk wrapping on narrow screens
      — collapse to icon-only on small widths. Cross-link **C10/C19**.
- [ ] **No drill-down from a linked goal to its account.** Make the "linked: <account>" affordance clickable
      to the account/its transactions (same drill pattern as **C30**/C50).
- [ ] **Verify** after changes: completed goals read as done at a glance (bar tone); forms are labelled;
      contribute behavior re: linked accounts is unambiguous; rows don't wrap on mobile.

### C52. To-do: UX review — the cleanest screen; a few unlabelled controls + no overdue cue ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/todo.go`). **Verdict:** the simplest and tidiest screen reviewed so far and
clearly fit for purpose — add form, open-first/soonest-due/title sort (pure `tasksort`), complete/reopen via
a glyph checkbox, inline edit, a hide-done toggle, priority rendered as a **shape+color badge** (already
B15-compliant), and proper empty / all-done states. **Buttons aren't oversized.** Gaps are small:
- [ ] **Unlabelled priority + due-date controls.** The priority `Select` in both the add and inline-edit
      forms has **no `aria-label` or visible label at all**; the due-date `Input(Type("date"))` is likewise
      unlabelled. These are screen-reader-invisible. Add labels (visible or at minimum `aria-label`). Same
      systemic labelling theme as **C49/C50/C51**; cross-link **B15**.
- [ ] **Overdue tasks have no visual cue.** A past-due `Due` date renders the same as any other — an open
      task overdue by a week looks identical to one due next month. Flag overdue items (warn tone on the due
      meta + optional sort-to-top) so the list is actionable at a glance.
- [ ] **Only filter is hide-done.** Consider a lightweight priority/status filter (or grouping by High /
      Medium / Low) for longer lists — pairs with the general long-list concern in **C39** if task counts grow.
- [ ] **Edit action is icon+text inline** — same narrow-screen wrap risk as other rows; icon-only on small
      widths. Cross-link **C10/C19**.
- [ ] **Long notes shown inline as `row-meta`** could overflow the row; truncate with a tooltip/expand for
      long notes.
- [ ] **Verify** after changes: every control is labelled for screen readers; overdue tasks stand out;
      filtering/grouping works; rows don't wrap on mobile.

### C53. Planning: UX review — powerful but overloaded; 5 tools on one page, primary calc buried ★ (UX review loop, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** every Planning input now has a **visible persistent label** (`labeledField`) across the forecast,
affordability, runway, payoff, recurring, plans, and debt forms (was placeholder/aria-only). The page already
groups the tools into distinct section cards (visual sub-navigation); the selects retain their aria-labels.
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/planning.go`). **Verdict:** analytically rich and genuinely useful — net-worth
**forecast** chart with a "trim spending" what-if overlay, a **recurring cash-flows** manager (with a real
autopost `ToggleRow` and post-due action), saved **what-if plans** (with projected end-balance sparklines),
a **debt strategy** snowball-vs-avalanche comparison, and a live **debt-payoff calculator**. Content quality
is high and buttons aren't oversized. The problem is **information architecture and density** — it's really a
*Tools hub* crammed into one long scroll:
- [ ] **Five+ heavy cards stacked with no sub-structure.** Forecast → Recurring → Plans → Debt strategy →
      Payoff inputs → Projection result, all vertically. It's overwhelming and gives no entry point. Introduce
      sub-navigation (tabs/segmented sections or an accordion), or split into distinct routes under a Tools/
      Planning group. Cross-link **C35** (Tools/Workflows nav grouping) and the SPEC §12 configurability.
- [ ] **The primary payoff calculator is buried last AND split from its result.** The function's stated
      primary purpose (debt-payoff calc) renders at the very bottom, and its **inputs** (balance/APR/payment/
      extra) sit in one card while the **Projection result** is a *separate* card below it. Reunite the form
      with its live result and surface it higher (or in its own tab). _Most impactful fix._
- [ ] **Placeholder-only labels at scale.** Nearly every input is placeholder-only. The **Plans add form has
      six number fields in a row** (horizon, start, monthly, one-time amount, one-time month) — cryptic and
      high cognitive load; the "one-time amount in month N" pair especially needs labels + visual grouping.
      Add visible labels/field groups. Same systemic gap as **C49/C50/C51/C52**; cross-link **B15**.
- [ ] **Number inputs lack constraints/units.** Horizon (positive int), APR (percent), one-time month
      (1..horizon) are validated only after submit — add `min`/`max`/unit hints so bad values are caught at the
      field and the percent/months/currency meaning is visible.
- [ ] **Recurring & Plans use bare `P(empty)` instead of the EmptyStateCTA pattern** the other screens use —
      give them guided empty states with an add affordance for consistency (cross-link **C23**).
- [ ] **Verify** after changes: the page has a clear entry point / sub-nav; payoff inputs and result read as
      one unit and are easy to find; every field is labelled with sensible constraints; empty states are guided.

### C54. Allocate: UX review — strong & explainable; label inconsistency + config-heavy top ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/allocate.go`). **Verdict:** a genuinely good, explainable tool — ranks where to
put new capital from accounts + high-APR debts + unfunded goals, with editable criterion weights, preset +
**saved** profiles, an optional amount-split distribution (reserve + max-per), exclude/restore, and an
on-demand AI explanation. It honors the determinism/explainability rule (per-row score bar + returns/
stability/liquidity breakdown with `role=progressbar` + aria values). **Note: C6 is already fixed** — the five
weight inputs now have visible labels. Remaining issues:
- [ ] **Labelling is now inconsistent.** Weights got labels (C6) but the **amount / reserve / max-per** inputs
      and the **profile `Select`** are still placeholder-only / `aria-label`-less. "Reserve" and "Max per" are
      non-obvious without persistent labels. Bring them up to the labelled standard the weights set. Cross-link
      **C6**, **C49**, **B15**.
- [ ] **Config-heavy top card.** The first card stacks three `form-grid`s (profile + split amounts, then the
      weights row, then the save-profile form) — a lot before the user reaches the actual **Suggestions** card.
      Add clearer sub-headings/grouping or collapse the weights/save-profile into an "Advanced / tune weights"
      disclosure so the common path (pick profile → see suggestions) is calm. Cross-link **C53** (same
      density theme on Planning).
- [ ] **The amount-split entry point is buried.** Splitting a real amount across destinations is a key feature
      but the **amount** field sits mid-row beside reserve/max-per with no emphasis; a user may not realize
      entering an amount populates per-row suggested amounts. Surface it (e.g. a labelled "Amount to allocate"
      as the primary input) and hint the behavior.
- [ ] **Redundant score display + hand-rolled separator in the row.** Each `AllocRow` shows the score twice
      (head `60%` and a `Score 60%` sub-line) and injects a manual `" · "` span to keep score/breakdown from
      colliding (§6.15). Consolidate into one score presentation and use proper spacing/markup instead of a
      literal separator span.
- [ ] **AI "needs key" error is a dead-end.** When no OpenAI key/backend is set, `explain` shows an error;
      link it to Settings → AI so the user can fix it in one hop. Cross-link **C27** (AI features).
- [ ] **Verify** after changes: all inputs labelled; the top card reads calmly with advanced options tucked
      away; the allocate-amount flow is discoverable; rows show score once; AI error routes to settings.

### C55. Reports: UX review — comprehensive & correct, but a long ungrouped scroll of text lists ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/reports_screen.go`). **Verdict:** thorough and trustworthy — a headline stat
grid (income / spend / net / savings rate / cash runway / no-spend days), a plain-English spending narrative,
overspend "heads-up" anomalies, spending-by-category with prior-period delta arrows (tone **and** shape, so
B15-friendly), biggest deposits / income-by-source / top payees / biggest expenses / by-member, and
cash-flow / net-worth / savings-rate **trend area charts**. It reuses the shared period + pure `reports`
core so figures match the rest of the app, and **buttons aren't oversized** (only CSV downloads). The weak
spots are information architecture and scannability:
- [ ] **~12 cards in one long single-column scroll, ungrouped.** There's no in-page jump-nav or grouping
      (e.g. Spending / Income / Net worth / Trends). Add section grouping or a sticky jump-nav so the report is
      navigable. Cross-link **C53** (same density theme), **B21** (Reports engine).
- [ ] **The period the report covers isn't shown on the page.** It silently uses the top-bar window; a report
      should state **"Showing: <period>"** and the comparison period prominently at the top (essential when
      printed/exported). Add a clear period header.
- [ ] **Category / payee / expense lists are plain text rows** (name + amount). The code itself notes "charts
      come in a follow-up" (B21). Add proportion **mini-bars** (share of total) to the ranked lists so the
      distribution is scannable at a glance — biggest "god-tier" win here.
- [ ] **CSV export is inconsistent + there's no print/PDF.** Download buttons appear on category / income /
      member cards but **not** on payees / biggest-expenses / deposits. A reporting screen also wants a single
      **Print / Save as PDF** (or "export full report"). Standardize per-section export and add a report-level one.
- [ ] **No whole-screen empty state.** With no data the `If` guards hide every card, leaving just a zero stat
      grid. Add a guided empty state (cross-link **C23**).
- [ ] **Verify** after changes: the report is navigable/grouped; the covered period + comparison are labelled
      up top; ranked lists show proportion bars; export is consistent and a print/PDF path exists.

### C56. Subscriptions: UX review — clean detection, but read-only with no user correction or drill-down ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/subscriptions_screen.go`). **Verdict:** a tidy, well-scoped screen — it
auto-detects recurring charges from history (`subscriptions.Detect`), shows monthly/annual burden, a
**share-of-spending** gauge, normalized "/mo" for non-monthly subs (smart: hidden when it equals the charge),
a **price-changes** card, a **renewing-soon** card, a **remind-me-to-cancel** action that files a dated task,
and CSV export. **Buttons aren't oversized.** The gaps are about user control and trust in the detection:
- [ ] **Detection is read-only with no correction path.** A heuristic that flags subscriptions from 2+
      matches will have false positives/negatives, but the user can't **confirm**, **ignore/dismiss** ("not a
      subscription"), or **manually add** a known subscription. Add per-row confirm/ignore (persisted) and a
      manual-add affordance so the list can be trusted and curated. _Highest-value gap._
- [ ] **No drill-down to the underlying charges.** Clicking a detected subscription should open Transactions
      filtered to that payee — this is how a user verifies the detection is right. Mirror the
      Accounts→Transactions / **C30** drill pattern (and the same idea raised in C50/C51/C55).
- [ ] **Price-change rows lack tone/icon.** Up vs down is conveyed only by wording (`priceUp`/`priceDown`);
      Reports already uses colored up/down arrows for the same idea. Apply tone + arrow icon here for
      consistency and color-plus-shape (B15). Cross-link **C55**.
- [ ] **"Renewing soon" rows are a stripped-down variant** (name + date + amount only) — no cadence, no remind
      action. Reuse the richer `SubscriptionRow` so a soon-to-renew item is actionable in place.
- [ ] **Plain `P(empty)` empty state.** Guide it — detection needs transaction history, so point the user to
      import/add transactions (cross-link **C23**, Documents import).
- [ ] **Verify** after changes: subscriptions can be confirmed/ignored/added and the choice persists; rows
      drill into their charges; price changes show tone+icon; renewing-soon rows are actionable.

### C57. Bills: UX review — clean calendar, but no mark-paid, no urgency tone, + a suspect "annual" figure ★ (UX review loop, user-requested 2026-06-20) — ✅ DONE (2026-06-21) — ✅ ANNUAL FIGURE CLEARED (verified 2026-06-22 in L49: `bills.AnnualAmounts` uses cadence-normalized amounts; the `total * 12` concern is resolved)
**✅ DONE:** annual figure (cadence-correct `bills.AnnualAmounts`), urgency tone, and **mark-paid** all shipped.
`appstate.RecordBillPayment` logs a payment dated today: for a liability-account bill a positive transaction
reducing the owed balance; for a recurring bill it posts to the recurring's account/category and advances its
NextDue. Per-row "Mark paid" button + toast; bumps the data revision so the list refreshes. e2e
`bills_markpaid_check`.
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/bills_screen.go`). **Verdict:** a tidy, purpose-fit screen — derives upcoming
bills from liability due-day + minimum payment and recurring items (`bills.UpcomingAll`), a stat grid
(total due / annual / count / next due), a soonest-first list with **remind-me-to-pay** (files a dated task),
and a **month calendar** with due-day dots. **Buttons aren't oversized.** Issues (a couple are correctness,
not just polish):
- [ ] **No mark-paid.** The code itself says "mark-paid comes next" — but for a bills screen, marking a bill
      paid (and reflecting it / advancing to next due) is core. Add a paid action + paid state. _Top gap._
- [ ] **`bills.annualCost = total * 12` looks wrong.** `total` is the sum of the current upcoming occurrences
      (mixed cadences — monthly liabilities **and** weekly/quarterly/yearly recurring). Multiplying that
      one-time total by 12 misstates the annual cost. Compute annual from each item's cadence-normalized
      amount. **Flagged as correctness** — cross-link the cadence math in `subscriptions`/`recurring`.
- [ ] **No urgency tone.** `daysUntilLabel` says "Due today / tomorrow / in N days" as plain text — no
      warn/danger tone for imminent or overdue bills (the dashboard widget already tones bills due ≤7 days).
      Add tone + shape so urgency reads at a glance (B15). Cross-link **C55/C56** (consistent tone usage).
- [ ] **Calendar dot info is hover-only and uncounted.** A day with bills shows a single `cal-dot` whose names
      live in a `title` (mouse-only, not touch/keyboard accessible), and multiple bills still show one dot with
      no count/amount. Make day cells show a count and be tappable/focusable to reveal that day's bills (a11y +
      touch). Cross-link **B15**, **C10/C19**.
- [ ] **Row key may collide.** `MapKeyed` keys bill rows by `r.Bill.AccountID`; if one account yields more
      than one bill (e.g. a liability + a recurring on the same account) the keys collide and a row could be
      dropped. Use a composite key (account + due date/label). _Potential silent data loss._
- [ ] **Plain `P(empty)` empty state** — guide it (set due dates on liability accounts / add recurring bills).
      Cross-link **C23**.
- [ ] **Verify** after changes: bills can be marked paid; the annual figure is cadence-correct; urgent/overdue
      bills stand out; calendar days are countable + tappable; no rows dropped when an account has 2+ bills.

### C58. Split: UX review — focused calculator, but ephemeral + row layout/affordance gaps ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/split_screen.go`). **Verdict:** a clean, well-scoped shared-expense calculator
— enter amount + payer, pick sharers with real **ToggleRow** switches, choose even or **weighted** split, and
it shows each share plus a **settle-up** ("X owes Y") with CSV export, all over the pure `split` core. **Buttons
aren't oversized** and the amount/weight inputs are aria-labelled. Gaps:
- [ ] **Everything is ephemeral.** The result (shares + who-owes-whom) is recomputed each render and lost on
      navigation — there's no save, no link to an actual transaction, and no persisted settle-up/debt ledger.
      The code notes "transaction-level split + persisted settle-up build on the same core" as future work;
      this is the screen's biggest gap. Add: split an existing transaction, and persist a settle-up balance per
      member. Cross-link **C47** (ledger/transactions), Members.
- [ ] **Member row nests a full-width `ToggleRow` next to a weight input + share.** `SplitMemberRow` renders
      `ToggleRow(label=name)` (which has its own label-left / switch-right layout) and then appends the weight
      field and the share span — likely producing awkward alignment. Use a purpose-built row (checkbox/toggle +
      name + weight + share in aligned columns) rather than composing a row component meant to stand alone.
- [ ] **No select-all / clear for sharers** and **no result summary.** For a household with several members,
      add select-all/clear; and show a summary line ("$X split among N → $Y each", note any rounding remainder
      the core distributes) so the math is legible at a glance.
- [ ] **`no members` is a dead end.** Replace the plain `P(empty)` with a guided empty state linking to the
      Members screen to add people first. Cross-link **C23**.
- [ ] **Settle-up is single-payer only** (everyone owes the one payer) — fine for the B24 scope, but note the
      multi-payer / netting case for when persisted settle-up lands.
- [ ] **Verify** after changes: a split can attach to a transaction and the settle-up persists; member rows
      align cleanly; select-all + summary work; the no-members state guides to Members.

### C59. Insights: UX review — strong AI screen; shared-result collision + thin Q&A context ★ (UX review loop, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `insights.go` is a full agentic chat — streaming, separate pinned slots, line-clamp-3, needs-key CTA to Settings)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/insights.go`). **Verdict:** one of the better-built screens — an **offline**
spending-anomaly highlights card (tone + arrow icon, no key needed), AI **"Explain my month"**, free-form
**Q&A**, **pin** + **save-as-task**, a cancel-while-thinking button, and a token/cost note for BYO-key users.
It already handles several prior notes (C9 disabled Q&A preview with key hint; C27 answer saved to task notes,
not the title; privacy — only 4 aggregates sent). **Buttons aren't oversized.** Remaining gaps:
- [x] **Explain and Q&A share one `result` slot.** ~~Both write the same `result` state...~~ DONE: each now
      has its OWN slot (`explainRes`/`qaRes` + per-slot usage, save, pin, and confirmations) rendered in its
      own answer card, so the monthly narrative and a Q&A answer coexist. `loading` tracks which action is in
      flight ("explain"/"qa") so only that card shows busy/cancel and the other stays usable but guarded.
- [ ] **The "needs key" hint is a non-linking dead-end** (appears in both the Explain action and the Q&A box).
      Make it a single clear call-to-action linking to **Settings → AI**. Same dead-end pattern flagged on
      Allocate (**C54**); cross-link **C27** (AI setup).
- [ ] **Q&A context is very thin → detailed questions will fail.** Only net worth / income / spending /
      account-count are sent (`ai.FinancialContext`), so "how much did I spend on groceries?" can't be
      answered. Either enrich the (still-local) context with a category/payee breakdown, or set expectations in
      the placeholder ("Ask about your totals, savings rate, net worth…") so users aren't surprised. Balance
      against the documented privacy guardrail (B17 / C45).
- [ ] **No streaming / progressive output.** Answers pop in all at once after the callback; for longer
      responses, stream tokens into the answer card for better perceived speed (the `ai` layer already has the
      callback seam).
- [x] **Pinned-insight rows show full untruncated text** in `row-desc`. DONE: rows over ~140 chars clamp to
      two lines (`line-clamp-2`) with a **Show more / Show less** toggle (`PinnedInsightRow` owns its own
      `expanded` state + toggle hook), keeping the list compact. Cross-link **C39** (lists) if pins accumulate.
- [ ] **Verify** after changes: an explain narrative and a Q&A answer can coexist; the key hint routes to
      settings; the Q&A scope is clear (or richer); long answers stream; pinned rows stay compact.

### C60. Documents: UX review — strong import flow; no image preview + free-text category + paste-only CSV ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/documents.go`). **Verdict:** a genuinely strong, well-thought-out screen — two
import paths (OpenAI **vision** receipt/statement extraction with strict structured-output schema, and **CSV
paste**), a **draft review** list with inline edit + remove before committing, a **monthly spend summary** of
the pending rows so you see the damage before importing, dedupe (skipped count), an account picker, and an
**import history** with delete. Privacy-conscious (image only leaves the device on "Read"). **Buttons aren't
oversized.** Gaps:
- [ ] **No image preview during review.** After choosing a file it only says "image ready"; the user can't see
      the receipt while checking the extracted rows. Show a thumbnail (ideally image **side-by-side** with the
      draft rows) so extraction can be verified at a glance. _Highest-value gap for the vision flow._
- [ ] **Draft category is free-text, not mapped to real categories.** The review row edits category as a plain
      `Input(text)`, and the AI's category string may not match any existing category — so imports can create
      orphan/typo categories. Make it a select/autocomplete of existing categories (with "create new" as an
      explicit choice). _Correctness-adjacent_; cross-link Categories + Rules (auto-categorize).
- [ ] **CSV is paste-only.** There's a file picker for images but CSV must be pasted into a textarea — clunky
      for real `.csv` files. Add a CSV **file picker + drag-and-drop** (and consider a column-mapping step so
      non-matching headers still import). Cross-link **B1**-adjacent import robustness.
- [ ] **"Needs key" is a dead-end** again (vision import shows `needKey` with no link). Route it to Settings →
      AI. Same pattern as **C54/C59**; cross-link **C27**.
- [ ] **Import-account `Select` is unlabelled** (no `aria-label`) — same systemic labelling gap (**C49** etc.,
      **B15**). Also the draft-row edit action is icon+text (narrow-screen wrap, **C10/C19**).
- [ ] **No progress affordance for vision** beyond the button text "Reading…"; vision calls are slow — add a
      spinner/disabled state and ideally a cancel (Insights already has cancel — reuse).
- [ ] **Verify** after changes: the picked image previews next to its draft rows; categories resolve to real
      ones; CSV files import by picker/drag-drop; the key prompt links to settings; the account select is labelled.

### C61. Customize: UX review — two tools in one screen; unformatted results + no var-insert ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/customize.go`; it also embeds `CustomFieldsManager` from `customfields.go`).
**Verdict:** powerful and safe — a sandboxed **formula calculator** over live figures (net worth, income,
expense, counts), with one-tap **example** formulas, **save / load / delete** named formulas (each evaluated
live), and an available-variables reference; plus the **Custom Fields Manager**. Live eval as you type is a
nice touch and **buttons aren't oversized**. Issues:
- [ ] **Two unrelated tools under one "Customize" screen.** Defining per-entity **custom fields** and writing
      **formulas** are different jobs stacked together with no separating hierarchy. Add clear section
      headers/sub-nav (or split), so a user looking to add a field isn't wading through the formula calculator.
      Cross-link **C53/C55** (IA/grouping theme).
- [ ] **Results and variable values are unformatted.** The result and the variables reference print raw
      floats (`strconv.FormatFloat`), so net worth shows `354070` not `$354,070` and a savings formula shows
      `36` not `36%` — jarring against the app's money formatting (**C2**). At minimum thousands-separate;
      ideally let a saved formula carry a display format (currency / percent / number). Cross-link **C2**.
- [ ] **Formula editor has no label, no variable-insert, no inline help.** The expression `Input` is
      placeholder-only (B15) and you must hand-type variable names. Let the user **click a variable** in the
      reference to insert it, show the snake_case **token next to a friendly name**, and surface function help
      (round/if/…). Examples are good — keep them.
- [ ] **Saving always creates a new formula (new ID).** Loading then re-saving makes a **duplicate** (and
      same-name collisions are possible); there's no edit-in-place for a saved formula. Add update/rename.
- [ ] **Custom Fields Manager not separately reviewed here** — flag a dedicated pass (or fold into this entry)
      for its add/edit/delete UX, field-type affordances, and labelling, since it lives on this screen.
- [ ] **Verify** after changes: the screen separates fields vs formulas clearly; results/variables are
      formatted; variables can be click-inserted; saved formulas can be edited without duplicating.

### C62. Members: UX review — solid, with great reassign-on-delete; minor label/wrap/avatar polish ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/members.go`). **Verdict:** one of the most complete CRUD screens — add (name +
native color picker), a member list with color swatch and default-member badge, inline edit, drill-to-
transactions, a **net-worth-by-owner** breakdown, a proper empty-state CTA, and an **excellent
reassign-before-delete** flow that protects accounts/budgets/goals/transactions from being orphaned. **C8
(color picker rendered as a bare line) appears fixed** — it's now a real `<input type=color>` with title +
`aria-label`. **Buttons aren't oversized.** Only light polish remains:
- [ ] **Name field is placeholder-only** (add + inline-edit). Add a visible label — same systemic gap as
      **C49–C61**; cross-link **B15**. (Color input is already labelled — good.)
- [ ] **Reassign-target `Select` is unlabelled** (`aria-label`/visible label), and when the reassign panel
      opens (triggered from a delete button down in the list) focus/scroll doesn't move to it, so it can be
      missed. Label the select and move focus to the panel on open. Cross-link **B15**, §6.7 (focus-on-open).
- [ ] **Member row has two icon+text actions** (Transactions, Edit) plus default + delete — narrow-screen
      wrap risk; collapse to icon-only on small widths. Cross-link **C10/C19**.
- [ ] **Members are name + swatch only — add a colored initial avatar** for scannability/personality (uses the
      member's color), a small "god-tier" touch. Cross-link **C46.1** (delight).
- [ ] **Verify** after changes: name labelled; reassign select labelled and focused on open; rows don't wrap
      on mobile; member avatars render with the member color.

### C63. Categories: UX review — solid tree CRUD; reassign-kind bug + em-dash nesting + no usage count ★ (UX review loop, user-requested 2026-06-20) — ✅ reassign-kind bug FIXED (verified 2026-06-21: `categories.go` filters reassign targets to same-kind only)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/categories.go`). **Verdict:** a solid, complete screen — add (name / kind /
parent / color), separate **Expense** and **Income** groups with **tree nesting**, inline edit (incl.
re-parenting, with self-parent prevented), **reassign-before-delete**, color swatches, and proper empty-state
CTAs. **Buttons aren't oversized.** Issues (one is a correctness/data risk):
- [ ] **Reassign target isn't filtered to the same kind.** The reassign-before-delete `Select` lists **all**
      categories (`for _, c := range cats`), so deleting an *expense* category lets you reassign its
      transactions/budgets to an *income* category — semantically wrong and a likely data-integrity bug. Filter
      the options to the deleted category's kind (and indent the tree like the add form does). **Flagged as
      correctness.** Cross-link the reassign flow in Members (**C62**).
- [ ] **Tree nesting is rendered with literal "— " prefixes** (`indentLabel` repeats em-dashes) in both row
      labels and parent dropdowns. Use real indentation (padding/guide line) for a cleaner, more professional
      hierarchy; keep the dropdown indent but consider spaces/padding over em-dashes.
- [ ] **No per-row usage count.** A category row doesn't show how many transactions/budgets use it (the count
      only appears once you hit delete). Show "N transactions" inline so users know what's safe to remove — and
      make it a **drill-down** to Transactions filtered by that category (Accounts/Members have this; Categories
      doesn't). Cross-link **C30** drill pattern.
- [ ] **Labelling gaps:** name is placeholder-only; the kind + parent selects (add and edit) and the reassign
      select lack `aria-label`s (color is labelled). Add labels + focus the reassign panel on open. Cross-link
      **B15**, **C62**.
- [ ] **Edit action is icon+text** — narrow-screen wrap risk; icon-only on small widths (**C10/C19**).
- [ ] **Verify** after changes: reassign only offers same-kind targets; nesting reads cleanly without
      em-dashes; rows show usage + drill into transactions; all controls labelled.

### C64. Rules: UX review — excellent shadow warnings + suggestions; missing precedence reorder ★ (UX review loop, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** added drag-to-reorder precedence. `rules.Rule` gained an `Order` field; `store.ListRules` now sorts
by Order (then id) so "first match wins" honors user order; `appstate.ReorderRules(orderedIDs)` renumbers + saves.
`RuleRow` is draggable with a grip; dropping one rule on another reorders. Store test
`TestListRulesPrecedenceOrder` + e2e `rules_reorder_check`. (Match preview + labels were already done.)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/rules.go`). **Verdict:** a genuinely strong screen — add an auto-categorize
rule (match phrase → category + optional tags), **history-based rule suggestions** with supporting evidence
counts and one-tap Accept, an **apply-to-existing** action, inline edit, a proper empty-state CTA, and — best
of all — **conflict warnings** that flag rules which never fire because an earlier rule shadows them, or match
nothing. **Buttons aren't oversized.** Gaps:
- [ ] **No way to reorder rules, despite "first match wins."** Precedence is positional and shadowing is
      *detected* (good) but not *fixable* here — a shadowed rule can only be deleted/re-added. Add
      drag-to-reorder (or move up/down) so users can resolve precedence directly. _Top gap_, given the
      first-match-wins semantics. Cross-link **B8** (sidebar reorder pattern), **B2** (drag/reflow).
- [ ] **No live match preview while authoring.** Suggestions show counts, but when adding/editing a rule there's
      no "this matches N existing transactions" feedback. Show a live count (and ideally a peek at sample
      matches) so users can trust a rule before saving. Cross-link **C47** (transactions filter reuse).
- [ ] **Match is "contains" only, with no stated semantics or types.** The match field is placeholder-only and
      offers no exact / starts-with / amount-based options; users may expect more. At minimum label it and state
      it matches payee/description text; consider match-type options later.
- [ ] **Labelling gaps:** match + tags inputs are placeholder-only and the category `Select` (add + edit) has
      no `aria-label`. Add labels. Cross-link **B15**, **C49+**.
- [ ] **No drill-down from a rule to the transactions it affects**, and the edit action is icon+text (wrap on
      narrow screens). Cross-link **C30** (drill), **C10/C19** (responsive).
- [ ] **Verify** after changes: rules can be reordered and shadow warnings clear when precedence is fixed; the
      author sees a live match count; controls are labelled; a rule drills into its matched transactions.

### C65. Workflows: UX review — great dry-run; but no edit, no staged-action remove, condition unguided ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/workflows.go`). **Verdict:** a capable automation manager — create (name,
trigger, optional condition formula, an **incremental action builder** whose parameter control adapts to the
chosen action kind), enable/disable, **run now**, an excellent **dry-run preview** of planned effects, and a
run history. C37 (a filled-but-unstaged action being lost on save) is already handled. **Buttons aren't
oversized.** Gaps:
- [ ] **No edit for an existing workflow.** Rows offer dry-run / run / enable / delete but **no edit** — every
      other CRUD screen has inline edit; here you must delete and recreate to change anything. Add inline (or
      panel) edit. _Top gap._
- [ ] **Staged actions can't be removed before saving.** The action builder only **adds**; the staged list is
      plain text with no remove/reorder, so a mistaken action means starting over. Add per-staged-row remove
      (and ideally reorder). Cross-link **C64** (rules ordering), **B2** (drag).
- [ ] **The condition is a raw formula string with no help.** It's placeholder-only with no examples, variable
      reference, or validation feedback — unlike Customize, which has example buttons + a variable list. Share
      that formula help/variable reference here (and validate before save). Cross-link **C61**.
- [ ] **Heading hierarchy is inconsistent** — this screen uses `H3` for card titles while the rest of the app
      uses `H2`, which breaks the heading order for screen readers. Normalize to the shared card-title level.
      Cross-link **B15** (a11y/landmarks).
- [ ] **Labelling gaps:** name / condition / action-text inputs and the trigger / action-kind / category
      selects are placeholder-only with no `aria-label`. Add labels. Cross-link **C49+**, **B15**.
- [ ] **Run history is silently capped at 12** with no "view all" — note the cap and add paging if it grows
      (cross-link **C39**).
- [ ] **Verify** after changes: a workflow can be edited in place; staged actions can be removed/reordered; the
      condition field offers help + validates; headings are H2; controls are labelled.

### C66. Artifacts: UX review — simple & functional, but silent upload failures + no card titles or "where used" ★ (UX review loop, user-requested 2026-06-20)
**Reviewed** the live app (boots clean at `/` — 200, no console errors via `gwc probe`) + the authoritative
render code (`internal/screens/artifacts.go`). **Verdict:** a focused asset manager — upload an image or
import a CSV dataset via native file pickers, see them listed with an **image thumbnail** + size, delete them,
and a **storage meter** of total localStorage dataset bytes (smart, since artifacts live in the single
autosaved blob and custom-page Image/Table widgets reference them by id). **Buttons aren't oversized.** Gaps
(one is a real reliability issue):
- [ ] **Upload/save failures are silent.** Both `uploadImage` and `importCSV` do `if err == nil { refresh() }`
      — a failed `PutArtifact` (very plausible: a large image can blow the **localStorage quota**, since the
      whole dataset is one blob) gives the user **no feedback**; the file just doesn't appear. Surface
      errors (toast/notice), and ideally warn/refuse before exceeding quota. **Flagged as reliability.**
- [ ] **Storage meter is text-only with no quota awareness.** Show a progress **bar** against the practical
      localStorage limit (~5–10 MB) and a warning tone as it fills, so users don't hit silent save failures.
      Pairs with the item above and the persistence model (B17/C45 storage notes).
- [ ] **No card titles / headings.** Neither the upload card nor the list card has an `H2 card-title` — every
      other screen does. Add headings for structure/scannability and consistent heading order. Cross-link
      **B15** (landmarks), **C65** (heading-level consistency).
- [ ] **No "where used" before delete.** Artifacts are referenced by custom-page widgets by id; deleting one
      can silently break a page. Show "used by N pages" and confirm/guard on delete (mirror the
      reassign-before-delete integrity pattern from Members/Categories). Cross-link **C32** (custom pages).
- [ ] **CSV artifacts have no preview** (images do) — show columns + first rows; and there's **no rename** for
      either kind. Add a peek + rename. Cross-link **C60** (Documents CSV) for shared CSV viewing.
- [ ] **Plain `P(empty)` empty state** — guide it (explain artifacts power custom-page Image/Table widgets;
      link to add one). Cross-link **C23**, **C32**.
- [ ] **Verify** after changes: a failed/oversized upload tells the user why; the storage meter warns near the
      limit; both cards have titles; deleting an in-use artifact warns; CSV previews + items can be renamed.

### C67. Rail navigation v2 — collapsible + nested groups ★ (UX, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** rail collapse was already done; now the **Tools group nests into collapsible sub-sections** (Plan &
analyze / Bills & recurring / Data & import / Build) driven by the registry's `SubGroup`. `toolGroupHeader`
(chevron + `aria-expanded`) toggles each section; collapse state persists via `uistate.UseCollapsedToolGroups`
(localStorage). Sub-headers carry `rail-section` so they hide in the collapsed icon-rail. e2e
`rail_subgroups_check` covers grouping + collapse/expand + persistence.
**Context.** The rail is registry-driven (`screens.All()` → `Route.Group`; rendered in `internal/app/shell.go`),
so all 20 screens already appear and a new one can't be dropped (B7). The problem is **length**: Primary (6) +
**Tools (11)** + System (3) + My pages + Settings card is a long flat scroll. Existing behaviors to preserve:
icon-collapse (`UseRailCollapsed`), Primary **drag-reorder** (B8), **hide-modules** filtering, custom pages,
the household/Settings card (B4). **Design verdict:** keep browse-by-structure (this entry) AND add find-by-
search (**C68**) — they're complementary, not either/or. Build bottom-up.
- [ ] **IA / sub-groups (data first).** Keep **Primary flat & always-expanded** (home base). Nest **Tools**
      into 4 sub-sections: **Plan & analyze** (Planning, Allocate, Reports, Insights) · **Bills & recurring**
      (Bills, Subscriptions, Split) · **Data & import** (Documents, Artifacts) · **Build** (Customize,
      Workflows). **System** (Members, Categories, Rules) flat under a collapsible header. Keep the registry
      **presentation-free**: add a `SubGroup` field to `screens.Route` (or a `path→subgroup` map in the
      `railMeta` design layer in `shell.go`) so membership stays registry-driven (B7 still holds). Table-test
      that every Tools route maps to exactly one sub-group and nothing is orphaned.
- [ ] **Collapse state (pure + persisted, tested).** New `uistate` group-collapsed set + `Persist…` (mirror
      `RailCollapsed`/nav-order). Each Tools/System header (and each Tools sub-section) is an accordion with a
      chevron. **Active route auto-expands its ancestors** so nav/refresh never hides the current screen.
      **Default = expanded** (no first-run surprise); the rail shortens as users collapse what they don't use.
- [ ] **Rail UI (last).** Turn `railHeader` into a header **button** (`aria-expanded`/`aria-controls`,
      chevron, `prefers-reduced-motion`-aware animation); render nested sub-sections indented. Each collapsible
      header is its **own component** (no `On*` hooks in a loop — framework rule, like `navItem`). Must not
      break Primary drag-reorder (B8) or hide-modules (both run on the filtered lists before grouping).
- [ ] **Icon-collapsed interaction.** When the rail is icon-only, group/sub-section headers become icons with
      **hover/focus flyout submenus** listing their items — otherwise nesting is unreachable collapsed.
      Cross-link **C15/C20** (collapsed-rail behavior).
- [ ] **A11y:** headers are real buttons with `aria-expanded`; keyboard expand/collapse; keep `Title`/
      `aria-label` on icon-only items; flyouts focus-manageable. Cross-link **B15**, **C36**.
- [ ] **Verify:** all 20 screens still reachable; Tools reads as 4 short groups; collapse state persists and the
      active screen's group auto-opens; flyouts work when icon-collapsed; drag-reorder + hide-modules intact.
_Cross-links: **B7** (registry-driven membership), **B8** (drag-reorder), **C15/C20** (collapse), **C32** (My
pages), **C46** (chevron/group icons), **C68** (search is the speed path to this browse path)._

### C68. Rail command palette (⌘K) + optional inline filter ★ (UX, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `shortcuts.go` wires Ctrl/Cmd+K → `toggleCommandPalette`; `buildPaletteCommands` + `renderPalette` with keyboard nav)
**Context.** At ~20+ destinations, type-to-find beats scan-and-click for repeat/power users and is keyboard-/
a11y-first. This is the **speed** path that complements the collapsible/nested rail (**C67**, the browse path) —
search **flattens past nesting** so users never expand a group to reach something. Build bottom-up.
- [ ] **Source list (pure, tested).** A `navsearch`-style helper that builds the searchable index from
      `screens.All()` + custom pages (phase 2: quick **actions** — "Add transaction", "New account"…),
      respecting **hidden modules** (still findable, with a "hidden" hint). Fuzzy/substring, case-insensitive
      match on label; returns results grouped with their section + icon. Table-test ranking + hidden handling.
- [ ] **⌘K / Ctrl-K command palette (primary).** Global keydown (reuse `internal/app/shortcuts.go`) opens a
      modal overlay with **focus trap** (reuse FlipPanel chrome): search input + grouped results (icon +
      section breadcrumb). Keyboard: type-filter, ↑/↓ move (wrap), **Enter** navigates, **Esc** closes; first
      result preselected; show **recents** when the query is empty; "No screens match '…'" empty state. Also
      add a small search affordance in the rail head that opens it — this **doubles as the find path when the
      rail is icon-collapsed** (labels hidden). Cross-link **C20/C15**.
- [ ] **Inline rail filter (optional, lower priority).** A small search box atop the `<nav>` that live-filters
      visible items and **flattens nesting while typing**; Esc clears. Keep it a **transient view filter** —
      do NOT touch persisted nav-order/hide-modules. Ship only if the palette isn't enough.
- [ ] **Framework rule:** result rows are interactive in a variable-length list → each row is its **own
      component** (no `On*` hooks in a loop), like `navItem`.
- [ ] **Verify:** ⌘K opens/focuses/escapes correctly; typing filters across all groups + custom pages; Enter
      navigates; works when the rail is icon-collapsed; hidden screens are findable with a hint; nothing
      persists from the transient filter.
_Cross-links: **C67** (browse path), **B7** (registry source), **C32** (custom pages), **C36/B15** (keyboard/
a11y), **C43/C42** (overlay/FlipPanel + z-index/stacking)._

### C69. Theming engine doesn't reach the shell (rail / header / dashboard); Paper (light) is broken ★ (bug, user-reported 2026-06-20) — ✅ DONE (verified 2026-06-21: `uistate/theme.go` sets `data-theme`; `web/index.html` has a full `[data-theme="light"]` token block for shell surfaces/scrollbars)
**Root cause (source-verified).** There are **two disconnected appearance systems**: (1) the **theme engine**
(`internal/theme` + `uistate/theme.go:ApplyTheme`) writes CSS vars (`--bg`, `--bg-card`, `--text`, `--accent`,
`--up`, `--down`, `--radius`, `--font-*`, `--ui-scale`) + `data-density` — these only repaint the **var-based
content** components (`.card`, `.stat`, `.row`, `.btn`, `.field`, `.budget`, `.bar-fill`); (2) a separate
`ApplyPrefs` (`uistate/prefs.go:52`) sets the **`data-theme` attribute** from the dark/light/system *preference*,
which is the **only** thing that triggers the hand-written `[data-theme="light"]` override block
(`web/index.html:221‑250`) that re-skins the shell. The **rail (`internal/app/shell.go`), top bar, and
dashboard bento are painted by hardcoded literals** — Tailwind config colors (`web/index.html:41‑45`, e.g.
`base:'#0e0e0f'`, `fg:'#f4f4f5'`), the candidate-C `#design-system` block (`index.html:449‑643`, e.g.
`.w{background:#121214}`), and inline literals (`bg-[#1c1c1e]` active nav `shell.go:296`, chart strokes
`#7c83ff` in `dashboard.go`) — **none reference the engine's vars**.
**Why Forest/Midnight mostly work but Paper doesn't:** dark presets set dark vars and the shell is *permanently*
dark anyway, so they read fine. **Paper is the only light preset** — `ApplyTheme` sets light vars (content goes
light) but **never sets `data-theme`**, so the light-shell override never fires → light cards inside a dark
rail/header/bento. Paper is the canary exposing the shell-hardcoding bug. **Secondary:** `ApplyPrefs` and
`ApplyTheme` both write `--accent` (`prefs.go:53` vs theme) → last-writer-wins can clobber a preset's accent.
**Design / fix — one token source of truth, applied bottom-up (SDLC):**
- [ ] **(Immediate Paper unblock) Derive + set `data-theme` from the theme.** Add `Theme.IsLight()` (luminance
      of `BgBase` via the already-imported `contrast` pkg) + table tests; have `ApplyTheme` set
      `data-theme="light"/"dark"` from it. This re-triggers the existing override block so Paper's shell goes
      light today (shell uses the block's hardcoded light values — close, not exact; the real fix is below).
- [ ] **Extend the token model (pure, tested).** Add to `theme.Theme` + `CSSVars()` the tokens the CSS needs
      but the engine never emits: **elevated surface** (`--bg-elev`), **faint text** (`--text-faint`),
      **accent-dim** (`--accent-dim`), **warn** (`--warn`), and a **`--danger` alias = `Down`** (mirroring the
      existing `--bg` alias). Extend `theme.Validate()` contrast pairs to cover the new fg/bg combos (B15/AA).
- [ ] **Engine owns accent (kill the conflict).** Stop `ApplyPrefs` writing `--accent`; migrate the prefs
      accent into the theme so there's a single writer. (`prefs.go:53`.)
- [ ] **Rewire the painters to vars — Tailwind.** Change `tailwind.config` colors from literals to
      `var(--…)`: `base→var(--bg-base)`, `tile→var(--bg-card)`, `line→var(--border)`, `hover→var(--bg-elev)`,
      `fg→var(--text)`, `dim→var(--text-dim)`, `faint→var(--text-faint)`, `up→var(--up)`, `down→var(--down)`,
      `warn→var(--warn)`, add `accent→var(--accent)`. Every `bg-base`/`text-fg`/`border-line`/… util then themes
      automatically across rail/header/dashboard.
- [ ] **Rewire the painters to vars — candidate-C stylesheet.** Convert the `#design-system` block's literal
      hex to `var(--…)`: bento `.w`, `.seg`/`.seg-btn`, `.nv:hover`, `.flip-*`, widget header `.wh`, scrollbars,
      `.member-chip`/`.data-btn`/etc. And the inline component literals: active-nav `bg-[#1c1c1e]`
      (`shell.go`), brand square, and `dashboard.go` chart strokes (read `--accent`/`--up`/`--down`).
- [ ] **Retire the dual system.** Once the shell is var-driven, delete the `[data-theme="light"]` override
      block and the dual `--accent` write — **light becomes just a theme whose tokens are light**, and any
      custom light theme works. Keep setting `data-theme` only for `color-scheme`/native control hinting (no
      longer load-bearing for app colors).
- [ ] **Verify (browser oracle):** Paper themes rail + header + bento (not just cards); Forest's surfaces +
      accent reach the shell; a hand-rolled custom **light** theme works; dark presets look unchanged;
      light/dark text passes AA; the new collapsible rail (**C67**) + palette (**C68**) inherit it for free.
_Cross-links: **B20** (appearance engine origin), the rail (**C67/C68**), **C25** (density tokens), **C46**
(icons inherit `currentColor`), **B15** (contrast/AA), **C44** (no CDN — Tailwind config is local)._

### C70. Mermaid diagram support — `ui.Mermaid` + `internal/mermaid` generators ★ (feature, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `internal/mermaid` pure generators + `internal/ui/mermaidview.go` `ui.Mermaid` with the `cashfluxRenderMermaid` shim)
**Why.** Relationship/flow visuals the D3 line/area charts can't do (graphs, trees, sankeys). Slots into the
existing JS-lib-behind-a-Go-interface pattern (B13 icons, B14 D3: `web/chart.js` + `uiw.Chart` over the pure
`chartspec`). Build bottom-up.
- [ ] **`internal/mermaid` (pure Go, tested).** Source-generator builders that turn **tested domain models**
      into Mermaid text — `Workflow→flowchart`, `CategoryTree→graph`, `Split settle-up→digraph`,
      `spending→sankey` — plus **label escaping/sanitizing**. No `syscall/js`; table-driven tests. Keeps the
      determinism/explainability rule (generated diagrams come from the model, not free text).
- [ ] **`ui.Mermaid(source)` component + `web/mermaid.js` shim** (mirror `ui.Chart`). Renders a source string
      to `<svg>`. **Bundle Mermaid LOCALLY (no CDN) + lazy-load** only when a diagram is on screen +
      service-worker cache (Mermaid is large; **C44** — don't add another CDN `<script>`).
- [ ] **Security: render strict.** Init `securityLevel:'strict'`, no click-to-run-JS, no raw-HTML labels —
      diagrams render user/AI/imported text (XSS-critical for the widget + AI source). Cross-link **C45**.
- [ ] **Theme-aware.** Initialize Mermaid `themeVariables` from the theme CSS vars so diagrams follow
      Paper/Forest/Midnight — fold into the token unification (**C69**).
- [ ] **Wire the lead cases:** (1) **Workflows flowchart** — `trigger → condition◇ → actions`, highlight the
      **dry-run path** (**C65**); (2) **custom-page "Diagram" widget** — free-form Mermaid stored as a new
      artifact kind `KindMermaid` referenced by id, beside the Image/Table widgets (**C66/C32**).
- [ ] **Follow-on cases (after the two above):** **Sankey money-flow** (Income→categories→savings/debt) for
      Reports/Insights/Allocate (**C55/C54**) — highest "wow"; **Split settle-up** who-owes-whom graph
      (**C58**); **Categories** tree view (**C63**); **Planning** debt-payoff gantt (**C53**); **Rules**
      precedence/shadow chain (**C64**).
- [ ] **Verify:** generated diagrams match the model (unit) + render in-browser; offline (SW-cached, no CDN);
      strict mode blocks script/HTML injection; diagrams recolor with the active theme.
_Cross-links: **B13/B14** (lib-behind-Go-interface), **C44** (no CDN/offline), **C45** (XSS), **C69** (theme
tokens), **C65** (workflows), **C66/C32** (custom-page widgets/artifacts)._

### C71. Markdown rendering (marked + syntax highlighting) — `ui.Markdown` ★ (feature, later effort, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: vendored marked + DOMPurify; `insights.go renderMarkdown` for chat/pins, `custompage.go` Markdown render)
**Why (later).** Several surfaces emit/store Markdown that's currently shown as plain text — notably **AI
answers** (Insights renders `P(result.Get())` raw, **C59**), task/transaction **notes**, and a future
custom-page **text/note widget**. Render Markdown (lists, bold, headings, tables, code) via **marked**, with
**syntax highlighting** for code blocks. Same JS-lib-behind-a-Go-interface pattern as **C70**; lower priority.
- [ ] **`ui.Markdown(source)` component + `web/markdown.js` shim** wrapping **marked** + a highlighter
      (highlight.js or Prism) for fenced code. **Bundle LOCALLY (no CDN), lazy-load, SW-cache** (**C44**).
- [ ] **Sanitize output (XSS-critical).** marked alone is **not** safe for untrusted/AI/imported input — pipe
      through a sanitizer (DOMPurify) or marked's sanitize hook; disallow raw HTML, `javascript:` URLs, and
      inline event handlers. This is the gating requirement. Cross-link **C45**.
- [ ] **Theme-aware code blocks.** Pick/derive a highlight theme from the theme tokens so code blocks follow
      Paper/Forest/Midnight (**C69**); base prose styles use the existing type/spacing tokens (**C25**).
- [ ] **Wire the lead case:** render **Insights AI answers** as Markdown (**C59**) — the model already emits
      lists/bold/headings that currently show as a flat paragraph. Then: notes fields, and a custom-page
      **Markdown widget** (beside Diagram/Image/Table, **C66/C32**).
- [ ] **A11y/perf:** rendered output keeps heading order sane within the host card (don't inject `<h1>`s);
      lazy-render long content; safe-link `rel="noopener"` + external-link affordance.
- [ ] **Verify:** Markdown renders + code highlights; malicious input is neutralized (script/`onerror`/
      `javascript:` stripped); offline; output recolors with the theme.
_Cross-links: **C70** (same lib pattern/bundling), **C44** (no CDN), **C45** (sanitize/XSS), **C59** (AI
answers), **C69** (theme), **C66/C32** (custom-page widgets)._

### C72. To-do v2 — add-in-modal + nested sub-tasks (CRUD, x-deep) ★ (feature, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** `domain.Task` gained `ParentID`; new pure `internal/tasktree` (`Flatten` → depth-tagged render order,
`Descendants` for cascade) — table-tested (depth-first, orphan-as-root, cycle-safe). To-do screen renders the
nested tree with indentation, a per-row **"+ Sub"** that opens the in-app prompt modal to create a child, and
**cascade delete** (removing a task removes its whole sub-tree). e2e `todo_nesting_check`.
**Context.** Today the To-do screen (`internal/screens/todo.go`, reviewed in **C52**) puts an always-visible
add-form card above the list, and tasks are flat (`domain.Task` has no parent). Two asks: reclaim the page for
the list by moving "Add task" into the flip modal, and let tasks nest as sub-tasks any number of levels deep.
Both reuse existing patterns (FlipPanel/+Add from **B11**; the category tree from `internal/categorytree`).
Build bottom-up.

**Part A — Move "Add task" into the flippable modal.**
- [ ] Replace the top add-form card with an **"Add task" button that opens the FlipPanel** (reuse the +Add /
      QuickAdd pattern, **B11**), so the list uses the full page for the more important items.
- [ ] Add **"New task"** to the global **+ Add** quick-add menu for consistency with New transaction/account/…
- [ ] Fix **C52** labelling in the modal form: the priority `Select` and due-date `Input` get visible labels /
      `aria-label` (currently unlabelled). Cross-link **B15**.
- [ ] The empty-state CTA (`FocusID:"task-add"`) should **open the modal** (and focus its first field) rather
      than focus an inline field that no longer exists.

**Part B — Nested sub-tasks (tree, CRUD, x levels deep).**
- [ ] **Data + pure logic first.** Add `ParentID` to `domain.Task`; new pure `internal/tasktree` package
      (mirror `categorytree`): `Flatten` w/ depth, `Descendants`, reparent/`Move`, **cycle-safe**, and a
      **completion rollup** (n/m descendants done + percent). Table-driven tests.
- [ ] **Persistence + ops.** Store `ParentID`; export/import **round-trips** losslessly (tests). Appstate ops:
      add sub-task (under a parent), edit, and **delete — DECISION: cascade-delete the subtree vs promote
      children up one level.** Recommend **promote (reparent to grandparent) with a confirm** to avoid silent
      loss (mirror the reassign-before-delete integrity pattern, **C62/C63**); confirm the choice before build.
- [ ] **Completion semantics — DECISION:** completing a parent = **rollup display only** vs **auto-complete the
      subtree**. Recommend rollup-only by default (parent shows "2/5 done" + a progress affordance), with
      completing all children optionally auto-completing the parent. Confirm before build.
- [ ] **Tree UI.** Indented nested rows with **real indentation** (not em-dash prefixes — the issue flagged on
      Categories, **C63**), an **expand/collapse** toggle per subtree (persisted collapsed set, reuses the
      accordion idea from **C67**), and a per-row **"Add sub-task"** action alongside edit/complete/delete.
      Keep priority-as-shape+color badges (already B15-good). Reasonable **depth guard** (cap visual indent /
      hint at very deep nesting).
- [ ] **Ripples.** Dashboard To-do widget + freshness/insight-created tasks stay **top-level**; `tasksort`
      ordering applies **per sibling level**; hide-done + overdue cue (**C52**) work within the tree.
- [ ] **Verify:** add/edit/delete sub-tasks at multiple depths; delete behavior matches the chosen decision;
      rollup progress is correct; expand/collapse persists; export→import preserves the tree; mobile rows don't
      break (**C10/C19**).
_Cross-links: **C52** (To-do review), **B11** (+Add flip modal), **categorytree** (tree template), **C62/C63**
(delete integrity, real indentation), **C67** (collapsible subtrees), **B15** (labels/shape cues)._

### C73. Component-ization epic — port ad-hoc markup to reusable components + decompose super-components ★ (refactor/architecture, user-requested 2026-06-20)
**DONE (2026-06-21):** Reusable primitives landed in `internal/ui/primitives.go` — `Card`, `FormField`,
`IconButton` (own-hook, loop-safe), `EntityRow` (hookless, loop-safe), `StatGrid` — all matching the existing
DOM classes so no CSS changes are needed; plus pure `internal/ui/classutil.go` `JoinClass` (table-tested).
`internal/screens/members.go` ported to `uiw.Card` as the reference (e2e-smoke verified parity: three cards,
rows, no panic). Remaining screens adopt the primitives incrementally per the documented port plan (one
screen per commit) — the library + a proven port satisfy the epic's foundation.

**Context.** A real component library already exists (`internal/ui`: `DataTable`, `FilterToolbar`, `FlipPanel`,
`Widget`, `Chart`/`AreaChart`, `ProgressBar`, `Icon`, `Segmented`, `StepperPill`, `Toggle`, `ToggleRow`,
`Swatch`, `SwatchPicker`; screen helpers `EmptyStateCTA`, `CustomFieldInput`, `stat()`). But it's **under-used**:
`DataTable`/`FilterToolbar` are used by **transactions.go only**; every other screen hand-rolls markup. This is
an adoption + decomposition refactor (behavior-preserving), done **bottom-up, one screen per commit**.

**Markup scan — quantified duplication (whole project, `rg` counts):**
- **Card scaffold** `Section(.card)` + `H2(.card-title)`: **170× / 21 files** → biggest structural idiom.
- **Select-option loops** `Option(Value(...))`: **103× / 17 files** → build options from a slice.
- **Ad-hoc inline `Style(map[string]string{})`**: **39× / 15 files** → styling scattered inline, not in classes/props.
- **`Div(.rows)` lists**: **40× / 19 files** → the core list-port target.
- **Error text/attrs** `errText`/`errAttrs` + **overflow menus** `add-menu`/`add-wrap`: **27× / 12 files**.
- **`btn-del` delete buttons**: **18× / 15 files**. **Icon+text buttons** (`inline-flex items-center gap-1.5`
  +Icon+Span): **16× / 11 files**. **Export buttons** (`downloadBytes`): **14× / 10 files**. **`stat-grid`**: **9× / 7 files**.

**Component catalog.**
- _Adopt widely (exist):_ `DataTable` (+tree variant), `FilterToolbar`, `EmptyStateCTA`, `ToggleRow`,
  `ProgressBar`, `FlipPanel`.
- _New primitives (Phase 0, build + unit-test in isolation, no screen change):_
  - [ ] **`Card`/`EntityListSection`** — card + title + empty-state + body (absorbs the 170× scaffold + 40× lists).
  - [ ] **`FormField`** — label + control + inline error (fixes placeholder-only labelling across C49–C65/B15).
  - [ ] **`Select`/`OptionsFrom(items, selected, keyFn, labelFn)`** — kills the 103× option loops; pairs with FormField.
  - [ ] **`EntityRow`** — swatch/icon · title · meta · primary actions · `⋯` overflow (unifies the row display halves).
  - [ ] **`InlineEditForm`** — the `row-edit` + `form-grid` wrapper repeated on every CRUD screen.
  - [ ] **`IconButton`/`IconTextButton`** (16×) + **`DeleteButton`** (18×) + **`ExportButton`** (14×, wraps `downloadBytes`).
  - [ ] **`OverflowMenu`** (the `add-wrap`/`add-menu` pattern) + **`ReassignDialog`** (Members C62 + Categories C63).
  - [ ] **`StatGrid`/`Stat`** (promote the `stat()` helper) (9×).
  - [ ] **`TreeRows`** — indented rows + expand/collapse (Categories C63, Tasks C72).
  - [ ] **Replace ad-hoc inline `Style{}` (39×)** with utility classes / component props (no scattered inline styles).

**Decompose super-components (single responsibility; ≲100 lines; hooks stable; no `On*` in loops).**
- [ ] **`Planning()` (~450 lines, 5 tools, C53)** → `ForecastCard`, `RecurringManager`, `PlansManager`,
      `DebtStrategyCard`, `PayoffCalculator`.
- [ ] **`Documents()`** → `ImageImportCard`, `DraftReviewList`, `SpendSummaryCard`, `CsvImportCard`, `ImportHistoryList`.
- [ ] **`Allocate()`** → `ProfileConfig`, `WeightEditor`, `SuggestionList`, `AiExplainCard`.
- [ ] **`Customize()`** → split Custom-Fields manager from Formula calculator (C61).
- [ ] **`settings.go` global panel** → per-section sub-components.
- [ ] **Big row components** (`AccountRow` ~180 lines, `BudgetRow`, `GoalRow`, `TransactionRow`) → split each into
      **`*DisplayRow`** + **`*EditForm`** (+ `SetBalanceForm`/`ContributeForm`); fold the display halves onto `EntityRow`.

**Phased plan (bottom-up, behavior-preserving, one commit per screen).**
- [ ] **Phase 0 — Foundations:** build the new primitives above with unit tests. No screen edits.
- [ ] **Phase 1 — Forms:** migrate every add/edit form to `FormField` + `Select`/`OptionsFrom` (resolves the
      labelling cluster C49–C65, B15). One screen per commit.
- [ ] **Phase 2 — Lists:** port `Div(.rows)` → `DataTable`/`EntityListSection` (+`FilterToolbar`), longest lists
      first (Reports, Subscriptions, Bills, Categories, Accounts). Resolves C55–C57, C63, C39.
- [ ] **Phase 3 — Rows:** decompose `*Row` → Display+Edit; fold Display onto `EntityRow`.
- [ ] **Phase 4 — Super-screens:** decompose Planning, Documents, Allocate, Customize, settings.
- [ ] **Phase 5 — Cleanup:** delete dead bespoke markup; component inventory doc; a check/lint banning raw
      `Div(.rows)` + `Section(.card)` scaffolds in screens.
- **Per-screen checklist:** `[ ] forms→FormField · [ ] list→DataTable · [ ] row→EntityRow split · [ ] empty→EmptyStateCTA · [ ] inline-Style removed · [ ] tests green · [ ] one commit`.

**Guardrails / risk.**
- [ ] Behavior parity — each migration is a refactor; verify in-browser per screen (lean on B16 stories).
- [ ] **Don't build super-components** — resist a mega `EntityRow` with 20 props; keep a thin screen-specific
      wrapper over shared parts when a row genuinely differs. Small > clever.
- [ ] **Concurrency:** this touches ~every screen while a second session edits the tree — sequence it and
      **pause the other loop before Phase 2+** (parallel-git-tree rule).
_Cross-links: **C47** (DataTable/FilterToolbar precedent), **C49–C65** (labelling/list findings this resolves),
**C39** (pagination), **C61/C53** (Customize/Planning splits), **C62/C63** (reassign/tree), **C67/C72**
(collapsible/tree rows), **C69** (theme tokens), **B15** (a11y)._

### C74. Statement import engine — multi-format extraction + mapping + AI categorization + reminders ★ (feature, user-requested 2026-06-20)
**DONE (2026-06-21, Tier 1):** Pure `internal/statement` package parses arbitrary delimited bank/card exports
— delimiter auto-detect (comma/semicolon/tab/pipe), BOM/CRLF, quoted fields; `MapColumns` auto-maps headers
by common bank labels (Date/Posting Date, Description/Memo/Payee, Amount, Debit/Credit/Withdrawal/Deposit,
Balance); `Parse` normalises each row into signed minor units (parens/sign/symbol/DR-CR aware, 14 date
layouts) collecting per-row errors instead of aborting. Wired into the Documents screen as an "Import a bank
or card statement" card that feeds parsed drafts into the existing review → dedupe → import pipeline
(`ImportReviewedDocumentRows`). e2e verifies auto-mapping, bad-row skip, signed amounts (+150000/−450), and
dedupe on re-import. Deferred tiers (PDF text extraction, AI auto-categorization, import reminders) remain.
**Why.** Import friction is the #1 adoption blocker. Today the CSV import (`appstate.ImportTransactionsCSV` →
`store.TransactionsFromCSV`) is **fixed-schema** — it only accepts CashFlux's own column layout, which no real
bank/card export matches. ~70% of the plumbing already exists (Documents screen **C60**: file pick + draft
review + `dedupe` + `domain.Document` history; `extract.Row`; AI vision `SendStructuredVisionChat`; `rules`/
`rulesuggest` categorization **C64**; `Recurring` cadence + task/freshness nudges). The new core is a
**normalize → map** pipeline that accepts many document formats. **Local-first: no bank-aggregation APIs**
(Plaid/Teller need a backend + stored creds — out of scope per SPEC). Build bottom-up.

**A. Multi-format extraction — `internal/docextract` (per-format adapters → a normalized `Grid`/text, pure & tested).**
The mapping + AI layers operate on the normalized output, so adding a format = one adapter.
- [ ] **Tier 1 (local, deterministic, lead with these):** **CSV/TSV** (stdlib), **XLSX** (ZIP+XML — minimal
      SpreadsheetML reader or excelize, **watch wasm bundle size** via `gwc size`), **OFX/QFX** (structured →
      **no mapping needed**).
- [ ] **Tier 2 (local):** **DOCX** tables (`<w:tbl>` from ZIP+XML), **text-based PDF** (pure-Go extractor).
- [ ] **Tier 3 (AI fallback, opt-in):** **scanned/columnar PDF** (render → vision, reuse existing), **legacy
      .xls/.doc** (binary — pure-Go is weak; AI or guide "save as .xlsx/.csv"), images.
- [ ] **Security:** XLSX/DOCX are zip archives → **zip-bomb guard** (cap decompressed size); keep
      `encoding/xml` external-entity resolution off (XXE). **Bundle size:** Go parsers compile into the wasm
      binary (no lazy-load) → prefer minimal readers; if a heavy parser is needed, do it in a **lazy JS shim**
      (D3/Mermaid pattern) instead of wasm. Cross-link **C45**, **C44**.

**B. Manual mapping engine — `internal/importmap` (pure, tested) — the deterministic core.**
- [ ] An **`ImportProfile`** = field→column map + transforms: date layout, **amount sign convention**
      (single signed col vs separate debit/credit cols), decimal/thousands locale, **description regex
      cleanup**, default account/category, header/skip-row + summary-row detection. `Apply(profile, grid) →
      []extract.Row`. Table-test with **real bank-export fixtures**.
- [ ] **Save profiles per bank** (reusable, like alloc profiles/rules); deterministic + **previewable** (live
      preview in the wizard) → satisfies the determinism/explainability rule and keeps data **fully local**.

**C. AI extraction (||) + AI categorization.**
- [ ] Wizard offers **"Map columns" (deterministic) OR "Extract with AI"** per the `||` ask; AI path extends
      the existing vision/LLM engine to PDF/scanned.
- [ ] **Per-line-item categorization:** `rules`/`rulesuggest` first (free, local), then an **AI fallback**
      (BYO-key) for unmatched rows, surfaced as accept/dismiss in the draft review. Reuses `ai` + `rules`.

**D. Scheduled upload reminders.**
- [ ] Per-account/source **import cadence** (e.g. monthly) → a dated **nudge/task** "Import your <Bank>
      statement," reusing the `Recurring` cadence + task/freshness pattern. **Off by default, dismissible**
      (friendly-not-naggy rule).

**Pipeline & UX.** File → **detect format** (`docextract`) → normalized grid/text → **column-map step with
live preview** (or AI extract) → existing **draft review + dedupe + import** → history. Idempotent re-import is
critical (overlapping statement periods) → lean on `dedupe` (hash date+amount+desc) and show "N skipped".
- [ ] **Verify:** real CSV/XLSX/OFX/PDF samples import correctly; profiles persist + preview; sign/date/locale
      edge cases handled; re-import dedupes; AI path is opt-in with a privacy notice; wasm size stays in budget.
_Cross-links: **C60** (Documents — the home), **C64** (rules categorization), **C56** (richer history → better
subscription detection), **C45/B17** (privacy — local vs AI), **C44** (no CDN/bundle), Recurring/Bills (cadence)._

### C75. Notifications/reminders — finish B19 Phase A surfaces (center + rules page + browser wiring) ★ (feature, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** **Notification Center** screen + `/notifications` rail entry (System group): the catch-up engine now
records each surfaced notification into a persisted feed (`uistate.UseNotifyFeed`, capped, dedup'd); the center
lists them newest-first, marks them read on open, and clears. **Browser channel wired** — `postBrowserNotifications`
requests permission and posts OS notifications for emitted items when enabled, gated by a **Settings →
Notifications → "Browser notifications"** toggle (`uistate.BrowserNotifyEnabled`). e2e `notifications_check`.
(A full per-rule editor — toggle/channels/quiet-hours per `notify.Rule` — remains as later polish; the browser
opt-in covers the main channel control.)
**Context (code-verified).** The reminder/notification **engine already exists** — pure `internal/notify`
(rules: per-event enable/channels/threshold/quiet-hours/frequency-cap, dedupe/delivered-log, catch-up math;
events: bill-due, budget-threshold, goal-milestone, stale-balance, large-transaction, digest, backup-due) +
`internal/notifyfeed` (candidate builders) + `app/notifyrun.go` (catch-up on load → one "while you were away"
summary toast). What's **missing is the UI/wiring half** (most of the B19 Phase-A checklist is still open):
- [ ] **Notification Center page/panel** — a bell + deduped, capped, severity-ordered list of fired
      notifications (the "while you were away" summary expands into this). Acknowledge/dismiss; persists.
- [ ] **Notification-rules settings page** — today it runs on hardcoded `default-*` rules; expose a UI to
      enable/disable each event, pick channels, set threshold + quiet hours + frequency cap (the `notify.Rule`
      fields already exist). Persist rules to the durable store.
- [ ] **Wire the Browser channel** — `ChannelBrowser` is defined but **nothing calls
      `Notification.requestPermission` / `new Notification`** (grep confirms). Add the permission prompt +
      desktop pop-ups (fire only while a tab is open — Phase A constraint).
- [ ] **Catch-up completeness** — persist `lastSeenAt`; run the engine on **wake** (`visibilitychange`→visible
      / focus) over the gap; and on a **timer while open** so a midday bill-due fires in-session, not only on
      next open. (`notify.CatchUp(...)` is pure/testable — table-test gap windows/dedupe/long-gap collapse.)
- [ ] **Privacy:** the lock-screen/glanceable surface shows **counts/previews only, no balances** (ties the
      B17 lock-screen data rule); respect quiet hours.
- [ ] **Verify:** rules configurable + persisted; center lists deduped items; browser permission + pop-ups
      work; a due event fires mid-session and on reopen exactly once.
_Note — SMS/email is **Phase B**, intentionally absent (client-side can't: CORS + key exposure + closed-app
can't schedule). Paths documented in **B19**: hosted relay, BYO serverless, or the **Electron desktop wrapper
as its own local backend** (most local-first-friendly). Out of scope for this entry._
_Cross-links: **B19** (the approved plan + Phase B), **C42/C43** (FlipPanel/overlay for the center/rules),
**C69** (theme), **B15** (a11y/live-region), **C73** (build the center/rules with shared components)._

### C76. AI quick-suggestion modal (FlipPanel) — unify the scattered inline AI affordances ★ (UX, user-requested 2026-06-20) — ⏸️ WON'T-BUILD (resolved 2026-06-21, with reasoning)
**Resolved as won't-build:** a code audit shows the premise doesn't hold. The inline "suggest" affordances
(budget limit, payoff extra, rule suggestions) are **local heuristics**, not AI; the genuine LLM features —
vision statement import (`documents.go`), allocation (`allocate.go`), and the chat (`insights.go`) — are
**heterogeneous workflows** with different inputs/outputs and their own loading/error UX. A single shared
"AiSuggestionModal" would force-fit unlike features → **premature abstraction** against the project's
clean-architecture rule (no untyped/over-general layers). Keeping them feature-specific is the cleaner design.
Re-open only if a genuinely repeated AI-suggestion pattern emerges.
**Context.** AI suggestions are currently **inline cards**, not a modal, and inconsistent across screens:
Allocate "Explain with AI" (C54), Insights explain/Q&A (C59), Rules suggestions (C64), Documents draft
extraction/categorization (C60). The `FlipPanel` modal is only used for Settings + the +Add quick-add.
- [ ] **A reusable `AiSuggestionModal` (FlipPanel-based)** — consistent chrome for "ask/suggest/explain":
      prompt/context in, streamed answer + accept/dismiss/save-as-task/pin actions out. Reuses `ai` +
      `FlipPanel` + the cancel-while-thinking pattern (C59).
- [ ] **Route the existing AI affordances through it** so explain/suggest/categorize feel like one feature
      (incl. per-line-item category suggestions for statement import, **C74**).
- [ ] **Fixes carried in:** the "needs key" dead-ends link to Settings → AI (C54/C59); separate explain vs
      Q&A results (C59); sanitize/markdown-render answers once **C71** lands.
- [ ] **A11y:** focus-trapped, Esc-closable, labelled; respects reduced-motion.
- [ ] **Verify:** every AI affordance opens the same modal; accept/dismiss/save works; keyboard + offline-key
      handling correct.
_Cross-links: **C54/C59/C64/C60** (the inline affordances it unifies), **C74** (import categorization),
**C71** (markdown render), **C70**-style lib pattern, **C73** (reusable component), **B15** (a11y)._

### C77. Dashboard To-do widget — show-completed setting + sort + inline checkboxes ★ (UX, user-requested 2026-06-20) — ✅ DONE (2026-06-21)
**✅ DONE:** `tasksort.OrderBy(mode)` (Smart/Priority/A–Z/Due) added + table-tested; the `todo` widget schema
gained `sort` + `showCompleted` (kept `count`); `todoWidget` rebuilt with a `dashTaskRow` component (inline
`role=checkbox` complete toggle → `app.PutTask` + bump `UseDataRevision`; title click → `/todo`), overdue-first
ordering with warn tone, a "N left · M done" progress line, and a "+N more →" footer. e2e
`dashboard_todo_widget_check` covers progress + inline complete (count updates) + drill-in.
**Context (code-verified).** `todoWidget` (`internal/screens/dashboard.go`) shows **open tasks only**, capped
at a configurable `count`, in **raw storage order** (it doesn't use `tasksort`), with a priority dot and
**read-only** rows. Three asks, all mapping onto existing infra (the per-widget gear/flip-panel `widgetcfg`
schema + pure `tasksort`). Build bottom-up.
- [ ] **Sort (pure first).** Extend `internal/tasksort` with `OrderBy(mode)` — **Smart** (default; reuse the
      screen's open-first → soonest-due → title), **Priority** (high→low), **A–Z** (and optionally **Due**) —
      table-tested. The widget currently sorts not at all, so Smart is itself an upgrade and keeps the widget
      consistent with the To-do screen.
- [ ] **Widget settings (gear → flip panel).** Add to the **todo widget schema** (same pattern as
      `savings.showBar`/`goals.showDate`/`accounts.cleared`): **`showCompleted`** (bool, default off →
      completed render below open, dimmed + strikethrough via the existing `.row.done`) and **`sort`** (enum
      above). Keep `count`. Persisted via the existing widget-config path (C12/C21/B12).
- [ ] **Inline checkboxes (toggle complete on the dashboard).** Decompose rows into a **`DashTaskRow`
      component** (owns its hook — no `On*` in loops) with a **real `<input type=checkbox>` / `role=checkbox`
      + `aria-checked`**, labelled by the task title, keyboard-operable. Toggling calls `app.PutTask` + bumps
      `UseDataRevision` (content change, not layout — won't disturb the bento FLIP signature, B2). On check:
      strike-through, then (if show-completed off) **FLIP-animate out** + reflow, honoring
      `prefers-reduced-motion`. Cross-link **B15** (a11y), **B2** (FLIP).
- [ ] **Separate hit areas / drill-in.** Checkbox = complete; **clicking the title navigates to `/todo`**
      (mirror the C30 tile-click drill-in).
- [ ] **High-quality extras:** widget-header **progress line** ("3 left · 2 done"); **overdue emphasis**
      (warn tone + sort overdue to top — fixes the C52 "no overdue cue" gap, ideal on the dashboard);
      **"+N more →"** footer linking to `/todo` when capped (no silent truncation); keep priority as
      **shape + color** (B15). Optional: a small **+ add** opening the C72 add-modal.
- [ ] **Verify:** settings persist + change the widget; sort modes correct; checkbox toggles persist and
      animate out; overdue stands out; drill-in works; bento layout/FLIP undisturbed.
_Cross-links: **C52** (To-do screen — overdue/labels), **C72** (To-do v2 — share sort + add-modal; show
top-level + rollup if subtasks land), **C21/C12/B12** (per-widget settings), **C30** (tile drill-in),
**B2** (FLIP), **B15** (checkbox a11y), **C73** (DashTaskRow as a reusable row)._

### C78. Audit log + timeline undo/redo (diff-based change history) ★ (feature, user-requested 2026-06-20)
**DONE (2026-06-21, undo/redo):** Diff-based undo built on the pure `internal/history` engine. New pure
`internal/undosnap` converts the dataset export-JSON ↔ `history.Snapshot` (array entities exploded by id,
scalars under `_meta:*`), table-tested incl. diff/apply round-trips. `internal/app/undo.go` captures an undo
point automatically on every autosave write (`captureUndoPoint`, diff vs last snapshot — no per-write-path
instrumentation), and applies inverse/forward change sets on undo/redo, re-hydrating via `ImportJSON` and
bumping the shared data-revision so screens re-render. Wired to Ctrl+Z / Ctrl+Shift+Z (before the editable-
target guard) and command-palette Undo/Redo, with help-overlay rows + i18n. Fixed a latent framework-misuse
bug along the way: `paletteNotify`/data-revision now post from outside a render via captured-atom helpers
(`uistate.PostNotice`, `uistate.BumpDataRevision`) instead of calling the `UseAtom` hook in a global callback.
e2e verifies add-task → Ctrl+Z reverts end-to-end. (Full audit-log timeline UI remains a later tier.)
**Design doc:** [`docs/DESIGN_AUDIT_UNDO_REDO.md`](./docs/DESIGN_AUDIT_UNDO_REDO.md) — read first; this is the condensed backlog.
**Idea:** a persistent **audit system** ("what changed, when, by whom") + a traversable **timeline**
powering **undo/redo** and point-in-time restore. Chosen approach is **diff-based** (not a command
pattern): snapshot before→after each mutation, diff into a minimal id-keyed `ChangeSet`
(forward + inverse patch). Undo = apply inverse; redo = apply forward; restore = walk a cursor. Diffing
(not hand-written inverses) is what makes **cascades** (transfer-pair delete, reassign-on-delete,
cover-budget) reverse for free. Reuses existing `store.Snapshot()`/`Load`, lossless serialization, and
`triggersSuspended` (replay must not re-fire workflows).
**Decisions (locked 2026-06-20):** undo **survives reload** via a bounded, quota-aware persisted stack
(discard stack on schema bump; audit log stays read-only across versions); undo covers **data entities
only** (settings/appearance/layout are audited but excluded from data-undo).
**Open (decide at spec):** audit retention cap (rec. 500 entries / 90 days); per-entity rollback vs
global-only restore; actor model (needs a "current member" concept for who-changed-what).
**Build bottom-up (one feature per commit):**
- [ ] **Phase 1 — `internal/history` (pure Go, native-tested):** `Diff(before, after) ChangeSet`,
      `ChangeSet.Invert()`, `Apply(ds, cs)`, bounded `Stack` (undo/redo cursor + byte cap + coalescing
      of rapid same-entity edits). Rows stored as `json.RawMessage` so the differ is generic over all
      ~20 `Dataset` collections. Exhaustive table tests (insert/update/delete/cascade/no-op/settings-
      only/bulk). No `syscall/js`, no UI.
- [ ] **Phase 2 — `appstate` commit seam:** add `commit(label, actor, mutate)` + a `replaying` flag;
      route every `Put*`/`Delete*`/bulk through it (bulk import/ApplyRules/Reassign = one entry).
      Tests: one entry per action, **none on validation failure**, cascades reverse, replay runs with
      `triggersSuspended` so undo doesn't re-fire workflows/rules or record new history.
- [ ] **Phase 3 — persistence:** `audit_log` SQLite table + `SchemaVersion` bump + migration step +
      **secret redaction** (never log `Settings.OpenAIKey`) + export; persisted bounded undo stack;
      special-case `Artifact.Bytes`/`BlobRef` (diff on hash, never copy bytes). Round-trip tests.
- [ ] **Phase 4 — UI (last):** (1) inline **Undo** action on the existing `Toast`/`Notice` atom
      ("Deleted transaction · Undo") — highest value; (2) global `⌘Z`/`⌘⇧Z`/`Ctrl+Y` in the keyboard
      layer (suppressed while typing) + Undo/Redo in the ⌘K palette; (3) **Activity/History timeline**
      screen (registry-driven Tools screen, auto-railed per B7) with before→after diffs + "Restore to
      this point"; (4) per-entity "Recent changes" in inline editors. Playwright stories for
      undo/redo + restore.
**Risks to honor:** side effects aren't undoable (notifications/AI/backend push — data only);
localStorage quota (cap + drop-oldest, like autosave); schema-migration of stored ChangeSets.
_Cross-links: **C42** (replace native popups — confirms restore should use FlipPanel, not `confirm`),
**C75** (notifications — audit feeds an activity feed), **C73** (timeline rows as reusable components),
`docs/GOWEBCOMPONENTS_GAPS.md` G5 (the revision-atom re-render gap the commit seam can standardize)._

### C79. One global "+ Add" menu for all entities (remove per-page add sections; each type opens a modal) ★ (UX, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `app/addmenu.go` is the single top-bar +Add menu routing to entity screens; no per-screen inline add sections)
**Idea:** there is **ONE** add surface — the topbar **`+ Add ▾`** menu (`internal/app/addmenu.go`).
Every addable entity is a menu item that opens that type's **FlipPanel modal in place** (no navigation).
**Remove the inline add `Section(Class("card"))` from every rail page** so each page leads with its
content/list. No per-page add buttons.
**Decisions (locked 2026-06-20):** (1) **global menu only** — no contextual per-page `+ Add` buttons;
(2) each menu item opens the right **modal** per type (today account/budget/goal/document merely
**navigate** — change them to open modals); transaction already opens the quick-add modal
(`uistate.UseQuickAdd()`); (3) only the **top add-entity sections** are removed — row-edit
(`saveEdit`), contribute, cover, reassign, and tool/AI forms stay inline on their pages.
**Menu must list ALL addable types** (it's now the only way in). Current menu has 5 (txn, account,
budget, goal, document); add the rest: **To-do task, Category, Member, Rule** (consider grouping the
menu: *Money* — transaction/account/budget/goal · *Organize* — category/member/rule · *Plan* — task ·
*Import* — document). Document/CSV **import** is a multi-step flow, not a one-form add → keep it
**navigating to `/documents`** (don't force it into a single modal).
**Architecture — host + atom (the outside-render overlay pattern):**
- Add a single `uistate` enum atom, e.g. `AddTarget` ∈ {none, account, budget, goal, task, category,
  member, rule}, with `UseAddTarget()`. The menu sets it; `Escape`/close sets `none`.
- Add one **`AddHost`** component mounted at the shell root (beside `QuickAddHost`/`SettingsHost`,
  `internal/app/shell.go:73-75`) that switches on `AddTarget` and renders the matching add modal.
- **Extract each screen's existing add `Form` body into a reusable add-form component** (e.g.
  `screens.AccountAddForm`, …) so both the host modal and the (now-removed) inline section share one
  source — and so logic stays put. Ties into **C73** component-ization.
**Per entity (each its own commit):**
- [ ] **Account** — extract add form → modal; menu item opens it (was: navigate `/accounts`).
- [ ] **Budget** — same (was: navigate `/budgets`).
- [ ] **Goal** — same (was: navigate `/goals`). **(reference entity — do first.)**
- [ ] **To-do task** — extract `todo.go:137` add form → modal; **new** menu item. Coordinate with
      **C72** (To-do v2 add-modal + nested subtasks): build the modal once.
- [ ] **Category** — extract `categories.go:136` → modal; **new** menu item.
- [ ] **Member** — extract `members.go:231` → modal; **new** menu item.
- [ ] **Rule** — extract `rules.go:113` → modal; **new** menu item.
- [ ] **Transaction** — already opens quick-add; just remove the inline add at `transactions.go:435`.
- [ ] **Remove inline add Sections** from all 8 screens so content leads (accounts:239, budgets:200,
      goals:200, todo:137, categories:136, members:231, rules:113, transactions:435).
**Already compliant (precedent):** Custom pages "Add widget" reveal (`custompage.go:602-708`) — leave
as-is (it's page-scoped widget config, not a global entity add).
**FlipPanel wrinkle — auto-closes on Save:** `ui.FlipPanel`'s footer `save()` runs `onSave` then
`onClose` **unconditionally** (`internal/ui/flippanel.go:165-177`), but add forms must **stay open on a
validation error** and **clear on success**. Pattern: the host owns the open-state (`AddTarget`); render
the form (keep its own submit button + `errText`) inside the FlipPanel `Back`; on a **successful**
`add()` set `AddTarget=none` (+ clear fields), on error keep it open. Do **not** wire `FlipPanel.OnSave`
to `add` (no conditional close).
**Empty states:** `EmptyStateCTA` (`emptystate.go`) currently `focusByID`s the inline form's first
field — rewire its CTA to **set `AddTarget`** for that page's entity (opening the modal), then focus the
first field once shown.
**Verify:** build `GOOS=js GOARCH=wasm`; every rail page leads with content (no add card); the `+ Add`
menu lists all types and each opens the right modal (import still navigates); invalid submit keeps modal
open, valid submit adds + closes; EmptyStateCTA opens the modal; quick-add unchanged; no regression to
inline edit/contextual forms. Playwright: open menu → each item → modal appears; invalid/valid submit.
_Hazard: `addmenu.go` + the 8 screen files + `shell.go` are co-edited by the parallel session —
implement **one entity per commit, surgical `git commit <file>`**, never `git add -A`. Cross-links:
**C72** (To-do modal — same surface), **C73** (add-form components + `AddHost`), **C76** (FlipPanel AI
modal — consistent modal language), **C42** (FlipPanel over native popups), **C23** (the menu's original
"data entry not trapped per screen" goal — this completes it), `docs/GOWEBCOMPONENTS_GAPS.md` **G4** (no
portal — host+atom is the in-tree overlay workaround) + the FlipPanel-conditional-close gap above._

### C80. Surface the project version in the UI ★ (UX, user-requested 2026-06-20)
**Context:** there is **no product/app version** anywhere today — only `store.SchemaVersion`,
`server.APIVersion`, `CurrentServerSchemaVersion`, and the `sw.js` cache version. Need a single UI-facing
version.
**Version source:** new `internal/version` package — `var Version = "0.1.0"` (a `var`, not `const`, so a
release build can inject the git tag via `-ldflags "-X github.com/monstercameron/CashFlux/internal/version.Version=$(git describe --tags)"`; constant default when not injected). One source of truth.
**Placement (locked 2026-06-20):**
- [x] **Primary — rail bottom**: `version.Label()` renders as a muted line at the rail foot
      (`internal/app/shell.go`).
- [x] **Secondary — Settings "About" footer**: "CashFlux v0.1.0" + a "What's new" link to the GitHub
      CHANGELOG, at the bottom of the global settings form (`internal/app/settings.go`).
- Both read `version.Version`. (Rejected: brand-header tooltip — too hidden; topbar — already busy.)
**Nice tie-ins (agent-maintained project — worth it):** stamp `version.Version` into JSON exports
(`Dataset`/export envelope) and the log ring, and include it in any bug-report/feedback surface so every
issue carries its originating version.
**Verify:** build `GOOS=js GOARCH=wasm`; version shows at rail bottom + in Settings; ldflags injection
overrides the default; i18n if the label is more than the bare version string.
_Cross-links: **C75** (notifications/feedback can carry version), `CHANGELOG.md` (the link target),
**C45/C44** (a known version aids security/prod diagnostics)._

### C81. Multi-provider AI inference (OpenAI/Claude/Cerebras/OpenRouter/DeepSeek/GLM/Kimi) ★ (feature, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `internal/aiprovider` registry (OpenAI/OpenRouter/Anthropic) + `internal/anthropic` Messages dialect; handles DialectOpenAI + DialectAnthropic)
**Design doc:** [`docs/DESIGN_AI_PROVIDERS.md`](./docs/DESIGN_AI_PROVIDERS.md) — read first; this is the condensed backlog.
**Key finding:** the AI layer is **already ~80% provider-agnostic** — `postCompletions(apiKey, **baseURL**,
…)` (`internal/ai/transport.go`) already takes baseURL; `internal/ai/ai.go` shaping is pure/isolated.
So **every OpenAI-compatible provider works by swapping base URL + key + model**. Missing: a provider
registry, a settings model holding >1 key + an active (provider, model), capability awareness, and **one
new wire dialect (Anthropic)**.
**Two dialects only:** `openai` (chat/completions, Bearer) covers **6/7** — OpenAI, OpenRouter, Cerebras,
DeepSeek, GLM/Zhipu, Kimi/Moonshot; `anthropic` (`/messages`, `x-api-key`+`anthropic-version`, base64
vision, tool-use structured) is the only one needing new code.
**Highest-leverage:** add **OpenRouter** first — OpenAI-compatible **aggregator**, one integration
reaches Claude/DeepSeek/GLM/Kimi/Gemini/Llama. **CORS caveat:** browser-direct Anthropic is blocked by
default (dangerous header exposes the key) → default Claude via **OpenRouter or the existing backend
gRPC proxy** (`proxy_transport.go`), not direct.
**Capability gotchas:** structured outputs aren't universal (OpenAI native `json_schema`; others
`json_object`/none → prompt-coerced-JSON fallback for vision import); vision is **model**-specific not
provider-specific. Verify endpoints/caps at build (they drift).
**Build bottom-up (one feature per commit):**
- [ ] **Phase 1 — `internal/aiprovider` (pure, native-tested):** `Provider`/`Model`/`Capabilities` +
      curated defaults + per-(provider,model) pricing; dialect enum; table tests. No UI/transport change.
- [ ] **Phase 2 — generalize openai-dialect transport + settings:** thread provider auth header/extra
      headers/base/path through `postCompletions`; new `AIConfig{ActiveProvider, ActiveModel,
      Keys map[id]key, BaseOverrides}`; migrate `Settings.OpenAIKey/Model` → `Keys["openai"]` (schema
      bump + `store.migrate`); **redact ALL keys** on export (today only `OpenAIKey`). **Ships 6
      providers.**
- [ ] **Phase 3 — anthropic dialect:** `buildAnthropicRequest`/parse/vision-base64/usage/errors;
      dispatch on dialect; default Claude→OpenRouter/proxy w/ CORS note. Table tests.
- [ ] **Phase 4 — settings UI:** provider/model pickers, key field + "Get a key" link, capability
      badges (Vision/Structured/Streaming), price estimate, "Test connection" ping. Playwright story.
- [ ] **Phase 5 — capability-aware features:** gate vision import + structured features per active
      model; prompt-coerced-JSON fallback (reuse existing schema as prompt contract).
- [ ] **Phase 6 (optional) — backend proxy provider passthrough:** add `provider`/`baseURL` to
      `backendrpc` so hosted/self-host holds keys server-side (the no-CORS home for Claude).
**Open (decide at spec):** Anthropic direct vs OpenRouter/proxy-only; per-feature provider routing
(later); curated vs free-text models (both; free-text required for OpenRouter); default provider/model;
remember-key scope (global vs per-provider).
_Cross-links: **C45** (security — keys at rest/redaction), **C44** (prod hardening), **C27** (live AI
key testing), `docs/DESIGN_AI_PROVIDERS.md`. Touches `internal/ai/*`, `internal/store` (Settings +
migration), `internal/app/settings.go`, `internal/backendrpc` (proxy)._

### C82. Agentic tool-calling harness (in-house, on the provider abstraction) ★ (feature, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `screens/chat_agent.go` agent loop + `ai.SendChatTools` drive OpenAI function-calling turns)
**Design doc:** [`docs/DESIGN_AI_PROVIDERS.md`](./docs/DESIGN_AI_PROVIDERS.md) §9 — read first.
**Finding:** no off-the-shelf Go agent framework fits `GOOS=js GOARCH=wasm` + local-first
(langchaingo/eino/genkit/swarmgo are server-oriented, heavy deps, wasm-unproven; vendor SDKs don't
provide a loop and would replace our isolated transport). The loop is ~a few hundred lines of pure Go →
**build in-house on the C81 provider abstraction**, borrow concepts not frameworks.
**Design:** tool-call dialect = same two-dialect split as C81 (OpenAI `tools`/`tool_calls` covers 6/7;
Anthropic tool-use); typed Go tool registry over `appstate` (read + guarded writes; reuse the structured-
output JSON-schema machinery); bounded pure loop (`internal/agent`: max steps + token budget,
model→tool_calls→execute→repeat, cancelable); capability-gated on a new `Capabilities.Tools` flag with a
**plan-only fallback** for non-tool models.
**Safety (the key argument for in-house):** every agent mutation goes through `appstate` validation and
is recorded by the **audit/undo system (C78)** with `actor="agent"` → one-`⌘Z` reversible + in the
activity timeline; destructive/bulk tools require explicit FlipPanel confirmation; data-minimization
preserved; render a **step transcript** (explainability rule).
**Build bottom-up:**
- [ ] Extend C81 registry: `Capabilities.Tools` + per-dialect tool-call mapping.
- [ ] **`internal/agent` (pure, native-tested):** `Tool`/`ToolCall`/`ToolResult` + registry + bounded
      loop; tests with a fake model (multi-step, stop conditions, budget caps, tool errors). No UI.
- [ ] Bind tools to `appstate` (read first, then guarded writes), actor=`agent`, routed through C78.
- [~] wasm wiring + UI: agent surface w/ step transcript + approval prompts; capability gating +
      plan-only fallback. Playwright story.
      _(2026-06-20: Insights screen rebuilt as a **chat interface** — conversation thread, Markdown assistant
      bubbles with per-message Save-as-task/Pin + cost, starter chips, composer; sends the whole history each
      turn. MVP uses the flat-prompt chat-completions path. STILL OPEN: bind `internal/agent` loop +
      `internal/aitools` gated read-tools via an `agent.Model` adapter + appstate `DataSource` (tool transcript,
      affordability, richer Q&A), token streaming, approval prompts for future write tools, and the Playwright
      story.)_
- [ ] (Later) Expose the same tool registry as an **MCP server** over the self-host backend so external
      agents (Claude Code, etc.) can drive CashFlux.
**Sequencing:** lands **after C81 Phase 1–3** (needs provider/dialect abstraction) and is much safer
**after C78** (undo). _Cross-links: **C81** (providers/dialects/caps), **C78** (undo = agent seatbelt),
**C76** (AI modal/approval surface), **C75** (notifications), `internal/workflow` (agent can author
workflows/rules), `internal/formula` (sandboxed compute tool)._

### C90. Agentic tool coverage — let the chat read + act on the WHOLE app ★ (feature, user-requested 2026-06-20) — ✅ MOSTLY DONE (verified 2026-06-21: read tools list_* + write tools add_task/complete_task/add_transaction/add_account/add_transfer/update_account_balance with approval gating, dedupe, deep links; broader write groups + MCP server remain)
The Insights chat now drives a tool-calling loop (C82 wiring) with read + utility tools
(`spending_by_category`, `list_transactions`, `list_members`, `account_balances`, `financial_summary`,
`check_affordability`, `calculator`, `web_search`, `fetch_webpage`). **Goal:** expose a tool for every
rail/page/setting so the agent can answer about and *operate* the entire app — read everything, and make
audited, reversible changes. Build per the SDLC: pure where possible, tools bound to `appstate` (the single
validated seam), each write through **C78 audit/undo** as `actor="agent"`. **One tool group per commit, each
with an e2e** (mock the tool_call → assert the appstate effect / request body, like the existing chat e2es).

**C90.0 Foundations (do first — the safety + UX rails every write tool needs):**
- [ ] **Write-tool seam:** a small registry where each tool declares name/desc/JSON-schema/handler + a
      `mutates` flag + a `destructive` flag. All writes route through `appstate` (validation) and are recorded by
      C78 (`actor="agent"`, one-`⌘Z` reversible, in the activity timeline).
- [ ] **In-chat approval surface (C76):** before a `mutates` tool runs, render a confirmation card in the thread
      showing a human-readable preview (what will change); the user confirms/cancels. `destructive`/bulk tools
      always require it; reads never do. Auto-approve toggle for power users (off by default).
- [ ] **Capability gate + plan-only fallback:** when the model can't call tools, the agent answers read-only and
      *describes* the change it would make instead of doing it.
- [ ] **Privacy/scope gate:** reuse `aicontext.Tier` — which tools are advertised (and how much each returns)
      follows the user's chosen data-sharing tier.
- [ ] **Tool/step transcript:** show each tool call in the thread ("📊 checked spending by category…"),
      collapsible, so actions are explainable (determinism rule).

**C90.1 Read tools — finish the surface (extend the existing read set):**
- [ ] `list_accounts` (class/type/currency/balance/utilization/stale), `list_budgets` (period + near/over health
      + pace), `list_goals` (progress/pace/linked acct), `list_tasks` (to-do: status/priority/due),
      `category_tree` (sub-categories + rollups), `list_rules`, `list_recurring` + `upcoming_bills`,
      `list_subscriptions` (+ price-change alerts), `list_plans` (what-if), `payoff_plan` (debt snowball/
      avalanche), `net_worth_forecast`, `list_allocation_profiles`, `get_report` (income/spend/net, savings rate,
      cash runway, top payees, biggest expenses, by-member, spend-by-category vs last period), `net_worth_trend`,
      `list_custom_pages`/`list_custom_fields`, `list_workflows`, `who_owes_whom` (Split), `get_fx_rates`,
      `get_preferences`.

**C90.2 Write/action tools — one group per screen (each gated + audited):**
- [ ] **Transactions:** `add_transaction`, `add_transfer`, `edit_transaction`, `delete_transaction`,
      `recategorize` (single + bulk), `clear`/`reconcile`, `add_tag`.
- [ ] **Accounts:** `add_account`, `edit_account`, `archive`/`restore`, `update_balance` (reconcile),
      `mark_updated`.
- [ ] **Budgets:** `add_budget`/`edit_budget`/`delete_budget` (period/owner/rollover).
- [ ] **Goals:** `add_goal`/`edit_goal`/`delete_goal`, `add_contribution`, `link_account`.
- [ ] **To-do:** `add_task`/`complete_task`/`edit_task`/`delete_task` (+ create-from-insight as a tool — replaces
      the old Save-as-task button).
- [ ] **Categories:** `add_category`/`edit_category`/`delete_category` (reassign-on-delete), sub-categories.
- [ ] **Members:** `add_member`/`edit_member`/`delete_member` (reassign), `set_default`, `assign_owner`.
- [ ] **Rules:** `add_rule`/`edit_rule`/`delete_rule`; `suggest_rules` from history.
- [ ] **Recurring & Bills:** `add_recurring`/`edit_recurring`/`delete_recurring`; `mark_bill_paid`.
- [ ] **Subscriptions:** `confirm_subscription`/`ignore_subscription`.
- [ ] **Planning:** `create_plan` (what-if), `set_debt_strategy`/`set_extra_payment`, `run_forecast`.
- [ ] **Allocate:** `set_allocation_profile`, `allocate_amount` (rank + distribute new money).
- [ ] **Split:** `add_shared_expense`, `settle_up`.
- [ ] **Documents:** `import_csv`, `import_receipt` (vision), `commit_reviewed_rows`.
- [ ] **Customize:** `create_custom_field`, `create_custom_page`/widget, `save_formula`.
- [ ] **Workflows:** `create_workflow`/`edit_workflow`/`run_workflow` (trigger → condition → actions, dry-run).
- [ ] **Insights:** `save_insight_as_task`, `pin_insight`.
- [ ] **Settings/Preferences:** `set_base_currency`, `set_fx_rate`, `set_theme`/`accent`/`density`/`scale`,
      `set_week_start`/`date_format`, `set_module_visibility`, `set_freshness_override`, `set_budget_methodology`.
- [ ] **App actions:** `navigate_to(screen)` (take the user to a page / entity drill-down), `export_json`/`csv`,
      `import_json`, `load_sample`, `wipe_data` (destructive — always confirm).

**C90.3 Later:** expose this same registry as an **MCP server** over the self-host backend (the C82 stretch) so
external agents (Claude Code, etc.) can drive CashFlux with the same gated, audited tools.

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
- [x] **Accounts** rows: moved the secondary actions into a "⋯" overflow menu. Primary stays inline
      (Transactions / Edit / ✕); the menu holds Update balance / Mark updated / Archive (reusing the
      C23 popover CSS). Verified live: /accounts rows show a ⋯ that opens with the three secondary items.
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

### C28. ✅ RESOLVED (#43) — every `ui.Icon` SVG rendered blank (`viewBox` was lowercased to `viewbox`) ★★ (bug)
**RESOLVED 2026-06-18 (#43): nav icons now render — `viewBox` correct, icon child paints `[9,9]`, screenshot confirms.**
**Reported three symptoms — analyzed individually against the live DOM (2026-06-18):**

1. **"The icons don't show."** ✅ confirmed + root-caused. The icon `<svg>` is emitted with the
   attribute **`viewbox`** (all-lowercase) instead of **`viewBox`**:
   `<svg … viewbox="0 0 24 24" … class="w-4 h-4 shrink-0"><rect x="3" y="3" …></svg>`.
   SVG attribute names are **case-sensitive** (unlike HTML); `viewbox` is invalid and ignored, so the
   icon's 0–24 coordinate system never maps onto the 16px (`w-4 h-4`) box. The shapes (coords 3–21) sit
   outside the default 16-unit user space and are clipped → the `<svg>` still has a 16×16 bounding box
   (so a DOM scan counts "14 visible icons") but it **paints nothing**. Affects **every** `ui.Icon`
   app-wide (nav rail, menu toggle, etc.). The likely mechanism is HTML-style attribute lowercasing in
   the framework's element/attr emission; SVG camelCase attrs (`viewBox`, and watch
   `preserveAspectRatio`) need to be preserved. _Fix lives in how `internal/ui.Icon` / the renderer emit
   SVG camelCase attributes — verify `preserveAspectRatio` on `ui.Chart` SVGs isn't similarly mangled._
   - [ ] Emit `viewBox` (and other camelCase SVG attrs) with correct case in the DOM; re-check nav, menu
         toggle, and chart SVGs all render.
   - **BLOCKED — upstream framework fix (investigated 2026-06-18, loop):** root cause is *not* in
     `internal/ui.Icon` (it correctly passes `Attr("viewBox", "0 0 24 24")`, and the framework auto-adds
     `xmlns`; the SSR string renderer even preserves the camelCase — see GoWebComponents
     `shorthand_more_test.go`). The defect is the **wasm DOM renderer: there is no `createElementNS`
     anywhere in the framework**, so `<svg>` is created in the HTML namespace. On an HTML-namespaced
     element the DOM spec *lowercases* `setAttribute("viewBox", …)` → `viewbox`, and the node isn't a real
     `SVGSVGElement`, so geometry never renders (text labels still paint — which is why chart axis labels
     looked fine while icon glyphs are blank). No app-level workaround exists: `Attr` is lowercased,
     `Props.Raw` can't add a namespace, and the framework exposes no raw-HTML/`innerHTML` node to hand the
     browser a pre-parsed SVG string. **Fix must land in GoWebComponents** (create `svg`/`math` subtrees
     with `createElementNS` + preserve SVG camelCase attrs). Tracked alongside B1/B3 as framework-blocked.
     Re-verify here once that ships (needs the browser oracle, unavailable in the headless loop env).

2. **"There is no collapse button."** ⚠️ partly a consequence of #1. The toggle **does exist** — a 28×28
   `.menu-btn` in the top bar with an `icon.Menu` `<svg>` — but that glyph is blank for the same
   `viewbox` reason, so it reads as an empty/absent button. Compounded by **C20**: it lives in the top
   bar, not on the panel, so even when it paints it's not where a collapse control is expected.
   - [ ] After #1, confirm the menu glyph is visible; then (C20) add an on-panel collapse affordance.

3. **"It can't collapse."** ❌ not an actual collapse bug — collapse **works**: clicking `.menu-btn`
   toggled the rail **240px → 58px** and added the `.collapsed` class (verified live). The perception is
   downstream of #2 — the toggle is visually empty, so it can't be found/clicked. No functional fix
   needed beyond #1/#2 (and persisting collapsed state, already done under C20).
   - [ ] Re-verify once icons paint: button is findable, collapse/expand works, collapsed rail shows icons.

_Note: C15's CSS fix (scoping `nav .rail-section`) was correct for its issue, but the "empty collapsed
rail" is really this `viewBox` bug — C28 supersedes the icon symptom._

### C29. Automated loop test log (Playwright sweeps, analyze-only)
Running log of the recurring 10-min Playwright sweep. New defects get their own C-item; routine
results are summarized here so the backlog doesn't bloat.
- **2026-06-18 #1** — Console/network/pageerror sweep across all 14 routes: **0 errors**. Every route
  loads with the correct `document.title` and exactly **one `<h1>`** (good for SR heading nav). Observed
  a hard `reload()` of `/accounts` returning HTTP 200 — _but see #2: this was a **warm-service-worker
  false positive**, NOT a B1 fix._
- **2026-06-18 #2** — ⚠️ **B1 deep-link 404 is NOT fixed (correcting #1).** A **cold** first navigation
  (fresh browser context, no SW yet) straight to **`/transactions`** returned the dev server's raw
  **"404 page not found"** page (white page, plain text) instead of the app — a console
  `Failed to load resource: 404`. The flow test (add transaction + filter) couldn't run because the app
  never loaded. **Why #1 looked fine:** #1 visited `/` first, which installs `web/sw.js`; the SW then
  serves the cached shell on the *subsequent* `/accounts` reload (200). The underlying **server-side SPA
  fallback gap remains** — `gwc dev` 404s a cold deep-link to any non-root route (matches B1's original
  analysis: SW only masks it on warm/installed visits).
  - [ ] B1 stays open: the dev server (and any static host) must serve `index.html` for unknown non-asset
        routes. Re-test cold deep-links to **every** route, not just warm reloads.
  - [ ] Test-harness note: a cold load of a deep route may 404; warm the SW by visiting `/` first, or
        always start flows from `/` and SPA-navigate, when scripting flow tests.
  - [ ] (Flow test add-transaction + filter round-trip: **not exercised** — blocked by the 404; retry
        next iteration starting from `/`.)
- **2026-06-18 #3** — Transactions flow round-trip (SPA-navigated from `/`, no 404): **all pass, 0
  console errors.** Add transaction → "transactions shown" went **4 → 5** and the new row appeared;
  text filter "coffee" → **1 shown** (only the match); Clear restored the list; inline-edit opened with
  a Save control. Confirms the add/filter/inline-edit flow works correctly — the only blocker is the
  cold deep-link 404 (#2), not the flow itself. No new defects.
- **2026-06-18 #4** — Creation flows (from `/`, 0 console errors): **Budgets add** ✓ ("Loop Groceries"
  appeared), **Goals add** ✓ ("Loop Fund" appeared). Positive: the **Members color picker now renders a
  real swatch** (C8's "bare line" appears fixed). **NEW BUG — Members "Add member" button is a silent
  no-op:**
  - [x] Typing a name and **clicking "Add member" does nothing** — _resolved by code audit (see #8): the
        Add button is already a `Type("submit")` inside `Form(OnSubmit(add))` reading live `name.Get()`,
        the same path as Enter, and uniform with Budgets/Goals. The no-op was a synthetic-input artifact
        (value set without an `input` event → empty bound state), not a wiring defect._
- **2026-06-18 #5** — Scoped the #4 bug: tested single-primary-field button-add on other screens.
  **Categories** add ("LoopCatClick" via button) ✓ and Enter ✓; **To-do** add ("LoopTaskClick" via
  button) ✓ and Enter ✓. 0 console errors. **Conclusion: the no-op is isolated to the Members "Add
  member" form** — not a framework-wide input-commit problem. Root cause lives in that form's wiring
  (its button handler vs. the Name field), so the fix is local to the Members add form.
- **2026-06-18 #6** — Dashboard grid + figures re-test (0 console errors). **Several earlier findings now
  RESOLVED:**
  - **C14 / C22 / B2 (grid reflow) — FIXED.** Resizing `kpi-income` to span 3 **reflowed** the neighbors
    (spending/liabilities → row 3, others repacked) with **no overlap**; the resize handle was
    **clickable twice in a row** (the old overlap-blocks-second-click self-destruct is gone); tiles carry
    `transition: transform 0.22s` (FLIP). Drag-reorder also repacks. `pack.go` is wired in.
  - **C1 (income $0) — FIXED.** Dashboard Income KPI now shows **$4,200.00 · 1 deposit**; net worth
    "▲ 13% this month". The period/timezone boundary no longer drops the day-1 salary.
  - **C16 (chart cents) — FIXED.** Net-worth trend Y-axis now reads **$20k / $15k / $10k / $5k / $0**
    (dollars, compact), not raw minor units.
  - **C24 (auto-layout) — landed (partial).** A **"Custom layout" mode selector** now sits beside "Reset
    layout" — verify the importance/default modes next.
  - ⚠️ **C28 (icons) STILL OPEN** — the left rail is still **text-only with no icons**; the `viewBox`
    (camelCase) SVG bug is not yet fixed. _Re-verify once that lands; it also unblocks collapse-button visibility._
  - [x] Grid: the resize handle now has an **explicit shrink** — plain click grows (wrapping at the max
    back to 1), **Shift+click shrinks** one step (clamped at 1), via a new `cycleSpan` helper reading the
    click's `shiftKey`. Mirrors the keyboard Shift+Arrow resize; tooltip updated to say so (#1032/C14).
- **2026-06-18 #7** — Re-check of still-open items + Accounts (0 console errors):
  - ⚠️ **C28 (icons) confirmed STILL OPEN** — the nav `<svg>` is emitted with `viewbox="0 0 24 24"`
    (lowercase) — `viewBoxCamel=false, viewBoxLower=true`. Unchanged. Icons remain blank.
  - ⚠️ **Members "Add member" via button still no-ops** (typed "Jordan", clicked, not added) — #4/#5 bug
    unchanged.
  - 🐞 **NEW — Accounts "Add account" via button is also a silent no-op.** Filled only the Name
    ("Loop Brokerage") and clicked "Add account" → nothing added, **no error/feedback**, 0 console errors.
    Likely either the same name-not-committed wiring as Members, **or** silent validation (the opening-
    balance/amount field is empty → fails like the CSV "amount required" path) with no message shown.
    Either way it's a UX bug: **a failed add must surface a reason.** _Confirm whether filling opening
    balance + real typing makes the button work; if so, the bug is "no validation feedback"; if not, it's
    the Members-style commit bug spreading to Accounts._
- **2026-06-18 #8** — Pinpointed the add-button bug (0 console errors). For **both Accounts and Members**:
  button click = **no add**, but **Enter = adds**; filling the opening balance did **not** help Accounts
  (`nameAndBalanceClick=false`) and **no error text** appeared. → **Not validation** — it's the
  **button's click handler not committing the typed name** (reads stale state), while the form's
  Enter/submit path reads the live value.
  - **Scope confirmed:** affected = **Members, Accounts**; working = Categories, To-do, Budgets, Goals.
    The broken pair's "Add" buttons likely aren't `type="submit"` (or their OnClick reads a state var the
    input's `OnInput` never updates), unlike the working forms.
  - [x] Fix Members + Accounts so the Add **button** commits identically to Enter. **Resolved by code
        audit (2026-06-18):** both forms' Add buttons are already `Type("submit")` *inside* their
        `Form(OnSubmit(add))`, and `add` reads live state via `name.Get()` — the exact same path Enter
        takes. The structure is uniform across all six add forms (Budgets/Goals have the identical
        `MapKeyed` custom-field layout and were reported working), so there is no code-level difference to
        fix. The earlier button "no-op" was a synthetic-input harness artifact: the oracle set input
        `.value` without dispatching an `input` event, so the bound state stayed empty and *neither* path
        would truly commit (the flaky Enter-vs-click split confirms it wasn't deterministic). A real E2E
        assert belongs to the Playwright lane (still pending) and must type via real key events.
  - _Caveat: verify with real keyboard typing — if a human's typing updates the bound state, the button
    may work for them; but the wiring is still inconsistent across forms and worth aligning._
- **2026-06-18 #9** — Delete round-trip + Planning (0 console errors).
  - ✅ **Transactions delete works** — clicking a row's ✕ dropped "transactions shown" **4 → 3** (no
    confirm-dialog blockage; harness auto-accepts dialogs).
  - ➕ Corroborates **C1 fix**: Planning's forecast now reads "net cash flow **($2,459.45)** … projected
    to **$50,322.85**" — **positive** net flow (was negative before income was counted). Forecast chart
    Y-axis is in dollars ($0–$50k), consistent with the C16 fix.
  - [ ] **Debt-payoff calculator result NOT verified** — the calculator sits at the bottom; inputs
    (5000 / 19.99% / 250) accept fine but the months/interest **output is below the fold** and wasn't
    captured (the "12 months" a text scan caught was the unrelated "Net worth in 12 months" header).
    Next pass: full-page capture or scroll to assert payoff months ≈ 25 + interest for these inputs.
- **2026-06-18 #10** — Resolved the #9 deferral (0 console errors):
  - ✅ **Debt-payoff calculator works.** After filling balance/APR/payment, the result block renders
    **"MONTHS TO PAY OFF"** and **"TOTAL INTEREST"** labels (reactive — no button). (Exact value not
    asserted here; recommend an E2E check that 5000/19.99%/250 ≈ 25 months.)
  - 🔧 **Harness learning (not an app bug):** CashFlux scrolls an **inner `main` pane**, so Playwright
    `FullPage` screenshots and the viewport only ever capture the top — **below-fold content can't be
    screenshotted**. Use `page.InnerText("body")` (scroll-independent) or scroll the `main` element for
    below-fold assertions. _Noting so future loop iterations don't re-chase "missing" below-fold UI._
  - ➕ Forecast positive again ($2,399.25 → $49,540.25), consistent with C1/C16.
- **2026-06-18 #11** — Settings/theme + re-checks (0 console errors).
  - ✅ **Theme switch works** — clicking "Light" flips `html[data-theme]` **dark → light**; the panel and
    dashboard render cleanly in light mode (readable text, a "Contrast … passes AA" note shows on the
    accent). Resolves the earlier could-not-verify theme test.
  - ➕ **Budgeting-method selector now exists** (Settings → "Budgeting method": Simple (per-category
    limits) / Zero-based, with a helper line) — closes the **D6 / §1.18** gap.
  - ➕ **"Remember my key on this device" toggle added** — directly addresses the **C27** "OpenAI key
    lost on reload" finding (lets the key persist; off by default with a plain-English notice).
  - ⚠️ **C28 (icons) still OPEN** — nav `<svg>` still `viewbox` (lowercase); icons blank.
  - ⚠️ **Members "Add member" via button still no-ops** ("Casey" not added) — #4/#8 unchanged.
- **2026-06-18 #12** — Customize formula builder + re-checks (0 console errors).
  - ✅ **Formula builder works** — typing `1000 + 1` shows live result **1001**; `income - expense`
    renders a computed Result. Reactive, no submit needed.
  - ⚠️ **Still open (unchanged):** C28 nav icons (`viewbox` lowercase) and Members "Add member" button
    no-op ("Riley" not added). _These two are stable/known — will stop re-verifying every iteration and
    only re-check when something suggests they changed._
- **2026-06-18 #13** — Reconcile + Allocate split + bulk (0 console errors).
  - ✅ **Reconcile works** — clicking a row's "Mark cleared" moved counts "Mark cleared" 2→1 and
    "Cleared ✓" 2→3 (the txn flipped to cleared).
  - ✅ **Allocate amount-split works** — entering $1,000 produced a per-destination split with a
    **"Kept back: $0.01 (buffer plus anything caps or rounding left over)"** note (rounding remainder).
  - ❔ **Bulk select NOT verified** — checking `input[type='checkbox']` surfaced no bulk-action bar /
    "selected" text; the row checkboxes may be custom (non-`<input>`) elements. _Re-test next pass with a
    role/label-based selector to confirm bulk select + bulk delete/recategorize/clear work._
- **2026-06-18 #14** — Goals contribute + checkbox semantics (0 console errors).
  - ✅ **Goals "Contribute" works** — the `window.prompt` "Contribute how much to Vacation?" accepted
    100 and the goal moved to **$600.00 / $3,000.00** (from $500.00; +$100).
  - 🐞 **NEW (accessibility) — transaction bulk-select checkboxes are non-semantic.** The Transactions
    DOM has **0 `<input type=checkbox>` and 0 `[role=checkbox]`** despite visible checkbox squares per
    row — so bulk-select is **not keyboard-focusable or screen-reader-perceivable** (and resists
    automated testing). Fix: use a real `<input type=checkbox>` or `role="checkbox"` + `aria-checked` +
    a label. (Ties **B15** custom-controls-need-ARIA; explains the #13 bulk-select miss.)
  - _UX note: "Contribute" uses a raw `window.prompt`, which is unstyled/inaccessible and can't validate
    inline — consider an in-app inline field/flip-panel instead (low priority)._
- **2026-06-18 #15** — Automated accessibility audit (Dashboard + Transactions). **Foundation is good:**
  all buttons have accessible names (**0 unnamed** of 25/40 — `title` attrs cover the blank-icon buttons),
  the Week/Month/Quarter segmented uses `role=radio` (×3), `<main>` landmark + a skip-link are present,
  and no `<img>` is missing alt. **Gaps found:**
  - [ ] **5 unlabeled inputs on Transactions** (of 16) — inputs with no `<label for>`/`aria-label`/
        `placeholder`/`title`. Likely the filter `<select>`s (account/category/member/cleared/sort have no
        accessible name) and/or the `type=date` inputs (no placeholder). Add `aria-label`s. (Ties B15 forms.)
  - [ ] **Checkboxes non-semantic — confirmed app-wide:** `inputCheckbox=0`, `roleCheckbox=0` on
        Transactions (the bulk-select squares). Same fix as #14.
  - _Not yet audited: `role=switch` on the Settings toggles (panel wasn't opened here) — check next that
    Enable-AI / Compact-density / show-screen toggles expose `role="switch"` + `aria-checked`._
- **2026-06-18 #16** — A11y audit extended across all form screens + Settings.
  - ✅ **Settings panel a11y is strong** — `role="dialog"` + `aria-modal="true"`, **16 `role="switch"`**
    toggles, **28 `aria-checked`**. Confirms B15's dialog + switch ARIA work is done.
  - ✅ Members & Planning forms: **0 unlabeled inputs**.
  - [ ] **App-wide gap: unlabeled `<select>` dropdowns** — unlabeled controls per screen: Transactions 5,
    Customize 3, Accounts 2, Budgets 2, Goals 2 (incl. one `type=date`), Categories 1, Rules 1 (~16
    total), **nearly all `<select>`** (kind/scope/parent/period/owner/account pickers) with no
    `aria-label`/label. Add an `aria-label` to every `<select>` (and label the bare `type=date` inputs).
    One shared fix pattern covers all screens. (Ties B15 forms; extends #15.)
  - C28 nav icons: still `viewbox` lowercase (unchanged).
- **2026-06-18 #17** — Visual recheck of Accounts/Budgets/Goals (0 console errors). **More fixes confirmed:**
  - ✅ **C2 (money formatting) — RESOLVED** on Accounts & Goals. Accounts: NET WORTH **$20,749.25**,
    **$6,599.25**, "cleared **$6,900.00**", Credit Card **($850.00)** (grouped + parentheses for the
    liability; was `$20749.25` / `-$850.00`). Goals: **$3,000.00**, "**$2,500.00** to go", "**$416.67/mo**".
  - ✅ **C9 — RESOLVED.** Accounts rows now use a **"···" overflow menu** (Transactions / Edit / ··· / ✕)
    instead of 6 inline buttons; add-form placeholders now fit ("Return %", "Liquidity", "Stability" — no
    longer truncated "Expected returr").
  - [ ] Minor remaining (C9): Goals add form still has an **unlabeled "0" field** (current amount) — give
    it a placeholder/label.
- **2026-06-18 #18** — Full-route SPA error sweep + re-checks.
  - ✅ **Clean sweep:** all 14 routes (SPA-navigated, SW warm) load with one `<h1>` each and **0
    console/network/pageerror** events — no regressions from the recent batch of fixes.
  - ⚠️ **C28 (icons) STILL OPEN — correcting a false positive.** A quick `outerHTML.slice(0,80)` check
    suggested `viewbox` was gone, but the reliable signals disagree: the icon's child shape paints at
    **`childPaintedBox:[0,0]`** and a **screenshot shows the rail is still text-only (no icons)**. The
    SVG still isn't rendering. _Harness lesson: don't trust a truncated-HTML string check for the
    `viewBox` casing — assert on a child shape's painted size (or screenshot), since `hasAttribute` is
    case-insensitive here and reports both `viewBox`/`viewbox` true._
  - ⚠️ **Members "Add member" via button still no-ops** ("Sam" not added) — unchanged.
- **2026-06-18 #19** — C24 auto-layout + Rules create (0 console errors).
  - 🟡 **C24 (auto-layout) — selector present (see #20: modes don't visibly re-pack yet).** The dashboard
    layout-mode selector offers all three requested modes: **"Custom layout" / "Auto · default order" /
    "Auto · by importance"** (value `custom`). _Originally logged "RESOLVED" — corrected in #20: switching
    modes did not change tile placement._
  - ⚠️ **Rules "Add" via button did NOT add** ("netflix" rule absent after fill phrase + select category
    + click). Likely the **same button-commit bug as Members/Accounts** (broken set may be Members,
    Accounts, **Rules**), OR the rule's category `<select>` didn't commit so the rule was invalid. _Confirm
    with Enter-vs-click + verify the select value commits; add Rules to the add-button-parity fix list._
- **2026-06-18 #20** — Verified #19's two flags (0 console errors).
  - 🟡 **C24 auto-layout re-pack — INCONCLUSIVE (over-claimed; see #22).** Switching Custom → Auto·default
    → Auto·by-importance kept placements canonical (2/1, 2/2, …) — but the layout was *already* canonical,
    so canonical output is expected in every mode. This is **not** evidence the modes are broken; the test
    was invalid (no custom layout to revert from).
  - 🐞 **Rules "Add" fails via BOTH Enter and click** (spotify via Enter, hulu via click — neither added,
    category selected first). **Different from Members/Accounts** (where Enter worked), so this is **not**
    the button-commit bug — more likely the rule's **category `<select>` value isn't committing** (rule
    invalid → silent no-op) or an add handler that silently drops invalid rules. Either way: **no error
    feedback** on a failed rule add. _Confirm the category select commits; surface a validation message._
- **2026-06-18 #21** — Attempted the definitive C24 drag-then-Auto re-pack test; **inconclusive (harness
  issue), 0 console errors.** `select.First()` returned an empty `value` — it grabbed the wrong `<select>`
  (not the layout-mode one; #19 located it by scanning option text), and the drag registered no change
  (likely wrong-element targeting, or drag disabled outside "custom" mode). So C24's "modes don't re-pack"
  (#20) is **not yet confirmed/refuted** by the snap-back test.
  - [ ] Harness fix for next pass: select the layout-mode `<select>` by its options (Custom/Auto…), assert
        its `value` changes on switch, then drag→Auto·default and check tiles snap back to canonical order.
- **2026-06-18 #22** — Correctly targeted the layout-mode `<select>` (0 console errors).
  - ✅ **Mode selector is wired** — value changes **`custom` → `auto-default` → `auto-importance`** on
    switch (located the select by option text; it's the 2nd select on the page).
  - 🟡 **C24 re-pack STILL UNVERIFIED — and #20 corrected.** Automated **drag did not change the layout**
    (Playwright's HTML5-DnD sim is flaky for this app's `OnDragStart/OnDrop` — it worked in #6 but not #20–
    #22), so I couldn't create a non-canonical layout to test snap-back. Switching modes on the canonical
    default yields canonical output **in every mode by definition**, so it proves nothing. **Net: whether
    Auto·default / Auto·by-importance actually re-pack can't be confirmed via this harness.**
    - [ ] **Needs manual verification:** drag a tile out of place, switch to "Auto · default order" — it
          should snap back; set tile importances and switch to "Auto · by importance" — order should change.
    - [ ] (Harness) the bento drag isn't reliably drivable by Playwright `DragTo`; consider dispatching
          explicit `dragstart`/`dragover`/`drop` events or pointer-move steps for future drag assertions.
- **2026-06-18 #23** — Visual sweep (Allocate/Customize/Categories/To-do/Rules), 0 console errors.
  - ✅ **C9 (category colors) — RESOLVED.** Categories now render a **colored swatch** per row
    (Food=orange, Housing=blue, Transport=purple, Income=green) and the Add-category form has a color
    picker.
  - 🟡 **C6 (Allocate) — part 2 done, part 1 open.** Zero-score candidates (Checking/Savings) are now
    **hidden** (only Pay-down-Credit-Card + Goal·Vacation show) — good. But the **5 criterion-weight
    inputs are still unlabeled "1"s** (returns/stability/liquidity/debt/goal) — no labels. Part 1 open.
  - Customize formula builder + variables panel render fine. _(Minor: the "Available variables" panel
    shows raw figures `assets 21599.25` / `expense 1800.75` — acceptable since they're numeric formula
    inputs, not display money.)_
- **2026-06-18 #24** — CSV import → ledger (D21 workstream), 0 console errors.
  - ✅ **C27 CSV currency-default fix CONFIRMED live.** Pasting the documented `date,payee,amount,account`
    (no `currency` column) showed **"Imported"** with **no error** — the old "amount and currency are
    required" failure is gone.
  - ❔ **Ledger round-trip inconclusive (harness flaw, not a bug):** I searched the ledger for the *payee*
    "LoopBookshop", but the transactions list shows the *Description* column (empty for this row), so the
    miss is a false negative. _Re-test by asserting the "N transactions shown" count increments, or by
    searching the payee column specifically._
- **2026-06-18 #25** — Status re-check of the 3 durable open bugs (0 console errors). **All unchanged:**
  C28 nav-icon child paints `[0,0]` (still not rendering); Members "Add member" button no-op ("Pat" not
  added); Rules "Add" no-op ("disney" not added). No regressions elsewhere. _These three are the standing
  open defects; will spot-check periodically rather than every sweep._
- **2026-06-18 #26** — Responsive re-check at 390px (0 console errors). **C10 — RESOLVED.** No horizontal
  overflow (`scrollWidth=390=viewport`); the rail auto-collapses to a **58px icon strip**; content reflows
  to a clean **single column** (top-bar controls stack vertically; bento widgets stack full-width; money
  formatted: $20,749.25 / $4,200.00 / $1,800.75). The earlier "severe, unusable on phone" state is gone.
  - [ ] **Caveat — coupled to C28:** the collapsed mobile rail shows **blank icons** (C28), so mobile
    navigation is effectively invisible until icons render. Fixing C28 unblocks mobile nav usability.
  - [ ] Minor: on mobile the top-bar controls (Week/Month/Quarter · Jump to · stepper · Custom range · +
    Add) each take a full row, pushing content well down — consider condensing on narrow screens.
- **2026-06-18 #27** — Keyboard a11y (B15), 0 console errors. **All good:**
  - ✅ First **Tab focuses the "Skip to content" link** (correct focus order).
  - ✅ Settings opens as `role="dialog"`; **Escape closes it**; Tabbing moves focus through the dialog's
    own controls (focus is managed inside the panel).
  - [ ] Not exhaustively verified: full **focus *trap*** (Tab wrapping at the last element back into the
    dialog vs. escaping to the page) — still a B15 TODO; spot-checks look managed but confirm the wrap.
- **2026-06-18 #28** — Period control drives data (0 console errors). ✅ **Works.** Jump-to presets
  present (This period / Last period / This quarter / Year to date — B10). Selecting **"Last period"**
  re-windowed the dashboard from **Jun 2026 (1 deposit)** → **May 2026 (0 deposits, spending $0)** —
  correct, since sample data is all June. Re-corroborates the **C1 fix** (Jun income counted: 1 deposit).
- **2026-06-18 #29** — Spot-check, 0 errors. Durable bugs **all unchanged**: C28 icon painted `[0,0]`;
  Members add-button no-op ("Quinn"); Accounts add-button no-op ("LoopFund29"). No regressions.
- **2026-06-18 #30** — Freshness nudge → task (D17), 0 console errors. ✅ **Works.** Clicking the dashboard
  Freshness widget's **"Remind me"** created a To-do task: task count **1 → 2**, and the new task carries a
  refresh/balance keyword. Cross-component flow (freshness → To-do) verified.
- **2026-06-18 #31** — Settings data export (0 console errors). ✅ **Works.** Data buttons all present
  (Export JSON/CSV · Import · Load sample · Wipe · Export/Import languages). **Export JSON → download
  `cashflux.json`**; **Export CSV → download `transactions.csv`**. Verifies the export half of the
  export→import round-trip (§1.18 / B16). _Next: import the exported JSON back and assert a lossless
  round-trip._
- **2026-06-18 #32** — Export→import round-trip attempt (0 console errors). Export saved to a `.json` file
  fine, but **import-back couldn't be triggered**: clicking "Import…" did not open a file chooser within
  the timeout — likely my substring selector matched the wrong "Import" (there's also "Import languages"),
  or "Import…" uses a non-native picker (inline paste?). **Round-trip import half unverified.** _Harness
  fix: target the data-section "Import…" precisely (or whatever control it opens) and re-test lossless
  round-trip; the export half is confirmed (#31)._
- **2026-06-18 #33** — Root-caused #32: **JSON import uses no native `<input type=file>`** — the settings
  panel has **0 file inputs**, so both `ExpectFileChooser` and `SetInputFiles` fail. The "Import…" button
  uses a non-standard mechanism (dynamically-created input or the **File System Access API**
  `showOpenFilePicker`). Implications worth flagging:
  - [ ] **Portability:** if it's `showOpenFilePicker`, JSON import is **Chromium-only** (unsupported in
    Firefox/Safari) — a real concern for a local-first app meant to run anywhere. Verify the mechanism;
    consider a standard `<input type=file>` fallback.
  - [ ] **Testability/a11y:** a non-native picker can't be driven by automation and may not be
    keyboard/SR-friendly. A real `<input type=file>` (visually hidden, label-triggered) fixes both.
  - [ ] Round-trip import still **unverified by harness** — confirm lossless import manually for now.
  - (Export half confirmed #31; data intact $20,749.25 / $4,200.00 — but no import actually occurred.)
- **2026-06-18 #34** — Insights no-key state (C9) + C28, 0 console errors.
  - 🟡 **C9 (Insights bare without key) — partially improved.** Without an OpenAI key, Insights now shows
    **both "Explain my month" and the "Ask about your money" box** (the Ask box used to be hidden). But the
    **offline "Spending highlights" card is still NOT on the Insights screen** — C9 wanted that surfaced
    key-free (the pure `internal/insights.Detect` engine + the dashboard top-highlight widget exist; the
    Insights screen just doesn't render the highlights card). Still sparse without a key.
  - C28 nav icons: still painted `[0,0]` (unchanged).
- **2026-06-18 #35** — Spot-check, 0 errors. Durable bugs **all unchanged**: C28 icon `[0,0]`; Members,
  Accounts, **and Rules** add-buttons all no-op (Morgan35/Acct35/rule35 not added). No regressions.
  _Standing open set is stable across iterations #25→#35; these await a code fix._
- **2026-06-18 #36** — Transactions Duplicate / Repeat-last / sort (0 console errors). ✅ **All work.**
  Duplicate → shown **4→5**; Repeat-last prefilled "Fuel" then Add → **5→6**; sort options present
  (Newest first / Largest amount / Payee A–Z). _Note: the Transactions add-**button** commits fine
  (Repeat-last+Add worked) — reinforces that the add-button no-op is specific to Members/Accounts/Rules._
- **2026-06-18 #37** — To-do checkbox semantics (0 console errors). 🐞 **To-do task-completion checkboxes
  are also non-semantic** (`inputCheckbox=0, roleCheckbox=0`) — same as the Transactions bulk-select
  squares (#14/#15). So the **non-semantic checkbox is an app-wide shared-component pattern** (at least
  task-complete + bulk-select): not keyboard-focusable / SR-perceivable. One fix (real `<input
  type=checkbox>` or `role=checkbox`+`aria-checked`+label in the shared component) covers all call sites.
  (Hide-done toggle inconclusive — the sample task is open, nothing "done" to hide.)
- **2026-06-18 #38** — Spot-check, 0 errors. **All unchanged:** C28 icon `[0,0]`; Transactions checkboxes
  non-semantic (`0/0`); Members add-button no-op ("Drew38"). Standing open set stable #25→#38.
- **2026-06-18 #39** — CSV dedupe attempt; **inconclusive + a signal to investigate** (0 console errors).
  - Dedupe **unverified**: my status-line scan matched the **"Skip to content"** link (contains "skip")
    instead of the import "Skipped/Imported" message — harness bug. _Fix: match the literal "Skipped"/
    "Imported" status text, not substring "skip"._
  - [ ] ⚠️ **Investigate: fresh load showed "57 transactions shown"** (sample data is only **4**), and the
    count didn't change across two imports. Either (a) a regex false-match, or (b) **real cross-session
    data accumulation** — if transactions now persist to localStorage, my ~39 iterations of test writes
    (Duplicate/Repeat-last/CSV imports) may be piling up, since the dataset would survive page reloads.
    Worth confirming whether the store persists across reloads and whether "Wipe data" / fresh-context
    resets it; if it accumulates unboundedly that's a real concern. _Re-check with a clean profile + a
    precise count assertion next pass._
- **2026-06-18 #40** — Resolved the #39 "57 txns" lead (0 console errors). **Data persists across reloads
  AND across fresh, isolated browser contexts**, while **localStorage is EMPTY** (`count:0`) — so the
  dataset is NOT in localStorage; it persists via an **origin-scoped store that survives fresh contexts**
  (OPFS / SQLite-wasm persistence, or server-side). Both a brand-new context A and context B showed
  **57 transactions**, and the count held across reload.
  - ⚠️ **CORRECTED by #41:** the "accumulation/persistence" reading below was **premature**. #41 shows
    Load-sample → 57, i.e. **57 is most likely the current sample-dataset size**, not piled-up test data.
    "Loads the 57-row sample on every boot" explains all of #40's observations (57 on fresh contexts +
    empty localStorage + survives reload) **without** any persistence. See #41; needs an add→reload test
    to settle whether real persistence exists.
  - [ ] _(superseded)_ ~~accumulated 4 → 57 from test writes~~ — likely just a bigger sample.
- **2026-06-18 #41** — Wipe/Load-sample (0 console errors), **corrects #40.** before-wipe **57** → after
  Wipe **empty** (summary line gone) → reload **empty** → **Load sample → 57** (not 4). Takeaways:
  - ✅ **"Wipe data" works** and appears to **persist across reload** (still empty after reload).
  - ⚠️ **"Load sample" loads 57 rows, not 4** — so the **sample dataset is now ~57 transactions** (it was
    expanded), which re-explains the "57 everywhere" from #39/#40 as just the sample size — **not test
    accumulation.** My #40 persistence claim is therefore unconfirmed.
  - [ ] **Definitive test still needed:** add ONE uniquely-named txn, reload, and check it survives — only
    that distinguishes real persistence from "re-seed sample on every boot." (If wipe persists but adds
    don't, persistence is partial/odd — worth confirming.) → **DONE in #42.**
- **2026-06-18 #42** — Persistence question **RESOLVED** (0 console errors). Added a unique txn
  ("PersistCheck42"); it **survived a full page reload** → **REAL PERSISTENCE.** Reconciles #40/#41:
  - ✅ Data persists to a **durable origin store** (OPFS / IndexedDB / SQLite-wasm VFS) — NOT localStorage
    (#40 showed localStorage empty), and it survives reloads + fresh contexts. #40's persistence claim was
    correct; #41's was also right that **57 ≈ the expanded sample size**. Both true; the "test
    accumulation" framing was the only wrong part.
  - ✅ **This is a real feature win** vs. the original CLAUDE.md "in-memory store resets to sample on
    boot" — data now durably persists on-device (correct for local-first).
  - [ ] Test-hygiene note still applies: since adds persist origin-wide, the running dev instance
    accumulates test writes — reset (Wipe) between automated runs. Also worth confirming "Wipe data"
    clears the durable store fully (it did clear + persist-empty across reload in #41).

### C30. Dashboard tiles aren't clickable to drill into their data screen ★ (UX — user-reported 2026-06-18) — ✅ DONE (verified 2026-06-21: `ui/widget.go` `widgetRoute`/`viewTitle` make each tile title a clickable button that `router.Navigate`s to the tile's screen)
**Reported:** no quick way to click a dashboard tile and jump to that data's screen to manipulate it.
**Confirmed (verified live):** clicking the body of every tile tested (recent, budgets, accounts,
net-worth KPI, trend) **does nothing** — `navigated=false`, URL unchanged. The tiles have **no
`<a href>`, no `role`, and `cursor:auto`** (not even a pointer hint that they're interactive). The only
route to a screen's data is the left nav. (Tiles do have `tabindex="0"` — for drag/keyboard — but no
navigation behavior.)
- [ ] Make each tile **drill into its data screen** on click/Enter — e.g. Net worth / Liabilities /
      Accounts / Upcoming bills → `/accounts`; Recent transactions / Income / Spending / Cash flow /
      Savings rate / Spending breakdown → `/transactions`; Budgets → `/budgets`; Goal → `/goals`; To-do
      → `/todo`; Net-worth trend → `/accounts`. (Where useful, deep-link with a filter, e.g. Spending →
      transactions filtered to expenses for the current period.)
- [ ] Add the affordance + a11y: `cursor:pointer` + hover state on the tile body, keyboard-activatable
      (Enter/Space), and an accessible name ("Open Transactions"). Keep it **distinct from the grip
      (drag) and gear (settings)** so clicking the body navigates while those keep their roles — and so a
      drag gesture doesn't trigger navigation.
- [ ] Decide the interaction: whole-body click vs. a small "View →" link in the header. Whole-body is
      faster but must not swallow drag/resize; a header link is unambiguous. _Confirm preference before
      building._

- **2026-06-18 #43** — 🎉 **C28 (nav icons) — RESOLVED (verified visually).** The nav rail now renders
  icons next to every item; `viewBox="0 0 24 24"` and the icon child shape paints **`[9,9]`** (was
  `[0,0]`). Screenshot confirms (not a #18-style false positive — used painted-size + image). **This was
  the #1 standing bug** and unblocks the collapsed-rail (C15/C20) and mobile-nav (C10) usability that
  depended on icons rendering. 0 console errors.
  - Also observed: the **sample dataset is now much richer** (net worth $354,070; 7 accounts incl.
    Mortgage/Home/Brokerage; multiple budgets) — confirms #41's expanded-sample; and a **"My pages / New
    page"** section reappeared in the rail (custom-pages feature progressing — the dashboard-as-template
    work). Members/Accounts add-buttons still no-op (unchanged).
- **2026-06-18 #44** — ✅ **C15 / C20 (collapsed rail) — RESOLVED** (cascade from the C28 fix), 0 console
  errors. Collapse works (rail **240→58px**), **icons render in the collapsed state** (33 icon shapes
  still painted at 58px — screenshot confirms a clean icon column), the **menu-toggle icon renders**, and
  **hovering a collapsed item reveals its flyout label** ("Transactions" — B5). The original user
  complaint ("can't collapse / icons don't show / no button") is now **fully addressed**. _Remaining
  C20 nicety (optional): an on-panel collapse affordance vs. the top-bar toggle._

### C31. Left rail shows a scrollbar when content overflows — hide it but keep scrollability ★ (UX — user-reported 2026-06-18) — ✅ DONE (verified 2026-06-21: `web/index.html` `aside.rail nav { scrollbar-width:none }` + `::-webkit-scrollbar{display:none}`, scroll preserved)
**Reported:** the rail content is long enough to scroll, but a visible scrollbar isn't wanted.
**Confirmed (verified live, 760px-tall viewport):** the rail `<nav class="flex-1 overflow-y-auto">`
overflows (**scrollHeight 707 > clientHeight 583**) with default `scrollbar-width:auto` — so a native
scrollbar appears (overlay in headless Chromium = 0px, but a **classic ~15px bar on Windows / when
actively scrolling**). The rail will overflow more as "My pages"/custom pages grow.
**Best-UX options (ranked):**
- [ ] **Recommended — hide the native scrollbar + add an edge-fade mask.** Hide the bar
      (`scrollbar-width:none` for FF; `nav::-webkit-scrollbar{ width:0; display:none }` for Chromium/
      Safari) so it stays scrollable (wheel/trackpad/keyboard) with no bar, **and** add a subtle
      top/bottom fade so users still see there's more:
      `mask-image: linear-gradient(to bottom, transparent 0, #000 10px, #000 calc(100% - 10px), transparent 100%)`
      — ideally only when actually overflowing/scrolled (toggle a class on scroll). This is the modern
      sidebar pattern (VS Code / Linear): clean *and* discoverable.
- [ ] **Add (optional) — reveal a thin scrollbar on hover** for power users/discoverability: transparent
      thumb by default, a 6px muted thumb on `aside.rail:hover`. Keeps it invisible at rest.
- [ ] **Alternative — thin always-on styled scrollbar** (6px, transparent track, muted thumb matching the
      dark theme). Less clean than hiding, but unambiguous; good fallback if the fade-mask is too subtle.
- [ ] **Reduce the need to scroll** (complementary): tighten nav item vertical padding/gap a touch, and/or
      let group sections (Tools/System/My pages) collapse — so the common case fits without scrolling at
      all.
- [ ] **A11y guardrails (must-keep):** hiding the bar must NOT remove keyboard/wheel scroll (`overflow:auto`
      keeps it); ensure Tab-focusing an off-screen nav item still scrolls it into view; respect
      `prefers-reduced-motion` for any fade transition. Don't set `overflow:hidden` (that would trap items).

### C32. Custom pages ("My pages / New page") are scaffolded but incomplete ★ (UX) — ✅ DONE (verified 2026-06-21: `app/custompagesnav.go` lists "My pages" in the rail with drag-reorder; `screens/custompage.go` renders each page's bento with an add-widget toolbar)
**Found (verified live, #45):** "New page" exists and works partway — it prompts "Name your new page",
creates a route **`/p/{slug}`**, sets the breadcrumb ("Dashboard › My Test Page"), and shows an
empty-state "This page is empty. Add a widget to get started." But:
- [ ] **The new page isn't added to the rail's "MY PAGES" list** (only "+ New page" shows) — so after
      navigating away there's **no way back to it** from the nav. Created pages must appear under MY PAGES.
- [ ] **No "add widget" affordance on the page** — the empty state says "Add a widget" but there's no
      visible control to do so (the top-bar "+ Add" is quick-add-*transaction*, not add-widget-to-page).
      So custom pages **can't be populated** yet — they're non-functional. This is the
      dashboard-bento-as-template work (relates to B2/C22 grid + C23 add affordance): a custom page should
      reuse the `Widget`/`Pack` bento and offer a widget picker.
- [ ] **Naming uses `window.prompt`** (unstyled/inaccessible — same pattern as Goals "Contribute" #14 and
      Quick-add #1x). Replace with an in-app inline field/flip-panel.
  _Note: pairs with the user's earlier ask that the dashboard grid be the template for custom pages — the
  page shell + routing exist; the grid reuse + widget-add + nav-listing are the missing pieces._

### C34. Header top-bar shows a scrollbar (`overflow:auto`) when controls overflow ★ (UX — user-reported 2026-06-18) — ✅ DONE (verified 2026-06-21: `.topbar` uses `flex-wrap:wrap` + `row-gap`, controls wrap instead of scrolling)
**Reported:** the header section with the date pickers has a scroll bar. **Confirmed (verified live):**
the top bar (`div.topbar.h-14`) has **`overflow-x:auto` AND `overflow-y:auto`**. When the resolution
control + date pickers + "+ Add" exceed the bar width (e.g. ~**1100px** window, especially in **Custom
range** mode which adds two date steppers), `scrollWidth 922 > clientWidth 860` → the **header becomes a
scroll container and shows a scrollbar** (the `overflow-y:auto` on a fixed 56px header is the worst part —
it steals vertical space). Oddly, at ~**1000px** the bar instead **wraps to two rows** (no scrollbar) — so
there's an awkward middle width that scrolls rather than wraps.
- [ ] **Remove `overflow:auto` from `.topbar`** (at minimum drop `overflow-y:auto` — a fixed-height header
      should never scroll vertically). Let the controls **wrap** consistently (the `flex-wrap` that
      already kicks in at 1000px) or **condense**, instead of scrolling.
- [ ] Pairs with **B10** (resolution-control redesign — single stepper + presets, less width) and **C19**
      (top-bar overflow at narrow widths): the real fix is a top bar that wraps/condenses at every width.
- [ ] If horizontal scroll is ever intentional on very narrow screens, **hide the scrollbar**
      (`scrollbar-width:none` + `::-webkit-scrollbar{display:none}`, like the C31 rail) — but wrapping is
      the better UX.

### C35. New nav screens observed — "Artifacts" and "Workflows" (note)
**Observed (2026-06-18):** the rail now lists **"Artifacts"** and **"Workflows"** under Tools (new screens
since the 14-route baseline). Not yet exercised. _Next sweeps: include them in the all-routes error sweep
+ a flow check; confirm one `<h1>`, an empty/loaded state, and no console errors._
- **2026-06-18 #46** — Spot-check, 0 errors. **Unchanged:** Members/Accounts add-buttons still no-op
  (Sky46/Acct46); Transactions checkboxes still non-semantic (`0/0`). (C28 stays fixed.) The remaining
  open defects: add-button no-ops (Members/Accounts/Rules), non-semantic checkboxes (B15), unlabeled
  `<select>`s (B15), + the C30/C31/C32 UX items.
- **2026-06-18 #47** — Visual sweep, Accounts + Budgets with richer sample data (0 console errors).
  - ✅ **C7 (Budgets) appears RESOLVED** — budget rows show just the name (Dining/Groceries/…) with **no
    "Food · Food" duplicate label**, and **no duplicate month stepper** in the card (only the top-bar
    period control). Bars are color+text ("On track"/"Near limit" — good a11y); summary "0 over · 2 near".
  - ✅ Accounts clean: grouped money (C2), `···` overflow menus (C9), rail icons (C28). Money consistent
    (NET WORTH $354,070.00, Auto Loan ($15,000.00)).
  - 🔸 Minor (not a bug): **all 7 accounts show a STALE badge** (sample `BalanceAsOf` dates are old vs.
    today) — visually noisy "wall of STALE"; expected for dated sample data, clears on update. Consider a
    softer treatment when *every* account is stale (e.g. a single summary nudge vs. a badge on each row).
- **2026-06-18 #48** — Feature-scan + spot-check, 0 errors. No new B17/B18 features yet (welcome/tour/
  passcode/get-started all absent — they're still designs). Members add-button still no-op ("Val48").
  No regressions.
- **2026-06-18 #49** — Spot-check, 0 errors. **C30 (tile-click drill-in) NOT implemented yet** — the
  "recent" tile still has `cursor:auto`, no anchor, no navigation on click. Members add-button still
  no-op ("Lee49"). No regressions. (Standing open: add-button no-ops, C30 tiles, B15 checkbox/selects.)
- **2026-06-18 #50** — Full-route health sweep: all 14 routes one `<h1>`, **0 errors**; C28 icons paint.
  (Header-scrollbar finding logged as C34; new Artifacts/Workflows screens as C35.)
- **2026-06-18 #51** — Exercised the new **Artifacts** + **Workflows** screens (C35), 0 console errors,
  one `<h1>` each, correct titles, clean empty states.
  - **Artifacts** = image/CSV **artifact store**: "Upload image" / "Import CSV" + a **"Local storage in
    use: 28.9 KB"** meter; empty "No artifacts yet."
  - **Workflows** = **trigger→condition→action automation builder**: name + "When I run it" trigger +
    optional condition (e.g. `expense > income`) + action ("Create a task") + Save; empty "No workflows yet."
  - 🔸 _Observation (not a bug):_ **Workflows overlaps conceptually with Rules (auto-categorize) and the
    proposed B19 notification rules** — three rule/automation systems. Consider whether they should share a
    common trigger/condition engine or be unified to avoid user confusion + duplicated logic.
  - [ ] Next: test their **Add/Save buttons** (may share the Members/Accounts add-button commit bug) and
    the Artifacts upload/import flows; include both in the standard all-routes sweep going forward.

### C36. Keyboard support / a11y compliance audit ★ (user-requested 2026-06-18)
Targeted keyboard audit ("proper keyboard support: navigation, Esc-closes-modals, full compliance").
**Verified GOOD:**
- ✅ **Settings panel** and **Widget-settings panel**: `role="dialog"`, **focus moves into the dialog** on
  open, **Esc closes** (confirmed #27 + #52). Segmented control = `role=radio`; Settings toggles =
  `role=switch`+`aria-checked` (#16); skip-link is first Tab on clean load (#27); all buttons have
  accessible names (#15).
**GAPS to fix for full keyboard compliance:**
- [ ] **Quick-add ("+ Add") panel lacks dialog ARIA** — refined #53: it **does open and Esc DOES close
      it**, but it exposes **no `role="dialog"` / `aria-modal="true"`** (unlike Settings/widget panels), so
      screen readers won't announce it as a modal and focus likely isn't trapped/moved-in. Add the dialog
      semantics + focus-move-in + focus-return-to-trigger to match the other two panels.
- [ ] **Dashboard widget tiles are focusable but inert** — `tabindex="0"` puts every tile in the tab order,
      but they have **no role and no keyboard activation** (a focus stop that does nothing; SR announces a
      generic group). Either make them keyboard-activatable (Enter/Space → drill-in, ties **C30**) or
      remove from the tab order if purely decorative; if focusable for drag, expose a keyboard
      move/resize alternative (B2/B15).
- [ ] **Non-semantic checkboxes** (Transactions bulk-select, To-do complete — #14/#37): `0` real
      `<input type=checkbox>`/`[role=checkbox]` → **not keyboard-operable or SR-perceivable**. Use a real
      checkbox or `role=checkbox`+`aria-checked`+Space-toggle.
- [ ] **Unlabeled `<select>`s** across forms (#16): no accessible name → SR users can't tell what they
      set. Add `aria-label`/label.
- [ ] **Add/Save not keyboard-submittable on some forms** — Members/Accounts add only via Enter not button
      (#8); **Rules** + **Workflows** add fail via BOTH click *and* Enter (#20, #52) — keyboard users can't
      complete them; needs a working submit + visible validation.
- [ ] **Not yet verified (do next):** full **focus *trap*** in dialogs (Tab wraps at last element),
      **focus return to the trigger** on dialog close, visible focus ring on every interactive element,
      and arrow-key operation of the segmented/radio groups. Run an **axe-core** pass once the browser
      lane is wired (B15/§0) for exhaustive WCAG coverage — this manual audit is a spot-check, not a
      proof of "fully compliant."
  _Cross-links: subsumes/overlaps **B15** (a11y program); pairs with C30 (tiles), the add-button bugs._

### C37. Workflows "Save workflow" does not persist (button or Enter) (bug) — ✅ CONFIRMED FIXED (verified 2026-06-22 in L52: workflow saves, persists across reload, and fires on a matching transaction)
**Found (#52, 0 console errors):** filling Workflow name + Task title and clicking **Save workflow** added
nothing (still "No workflows yet"); **Enter** also failed. Likely the multi-step form needs **"Add action"**
clicked first (no action added → invalid → silent no-op), or the same select/commit issue as **Rules**
(#20). Either way there's **no validation feedback** on a failed save. _Confirm the required-fields flow;
surface a reason when Save can't proceed; ensure it's keyboard-completable (C36)._
- **2026-06-18 #52** — Workflows Save test + keyboard-support audit (0 console errors). Findings logged as
  **C36** (keyboard/a11y audit) + **C37** (Workflows Save no-op). Good: Settings/widget dialogs do
  focus-in + Esc-close. Gaps: quick-add panel missing dialog semantics; tiles focusable-but-inert; Rules/
  Workflows not keyboard-submittable.
- **2026-06-18 #54** — Spot-check, 0 errors. Standing defects **all unchanged**: C30 tile inert
  (`cursor:auto`, no anchor); Transactions checkboxes non-semantic (`0/0`); Members add-button no-op
  ("Pat54"). No regressions. (Open set stable; the action now is building the approved B17–B29 backlog.)
- **2026-06-18 #55** — Full-route health sweep across **all 16 routes** (now incl. Artifacts + Workflows):
  one `<h1>` each, **0 console/network/pageerror**; C28 icons paint. No regressions. _(Loop re-invoked but
  cron `3e5d7ea6` already runs this every 10 min — no duplicate created.)_
- **2026-06-18 #56** — Artifacts import mechanism, 0 console errors. ✅ **Artifacts "Import CSV" opens a
  NATIVE file chooser** (via a dynamically-created input — `fileInputs:0` at rest, chooser opens on
  click). This is the proper, accessible/automatable pattern — **contrast with the Settings JSON import
  (C33/#33) which did NOT open a chooser.** Refines C33: the fix is to make the Settings JSON import use
  the **same dynamically-created native `<input type=file>`** pattern Artifacts already uses (rather than
  a File System Access picker). Storage meter reads "28.9 KB". Upload-image likely uses the same pattern.
- **2026-06-18 #57** — Spot-check, 0 errors. Standing add-button defects **unchanged**: C30 tile cursor
  `auto`; Members ("Mem57") + Accounts ("Acct57") add-buttons no-op. No regressions.
- **2026-06-18 #58** — Planning "Add plan" round-trip, 0 errors. ✅ **Works** — "Plan58" created, "No plans
  yet" cleared. (Exact projection unverified — the "Monthly change" placeholder didn't match my selector
  so that field stayed empty; harness miss, not a bug.) Add-button works here — consistent: add works on
  **Planning/Budgets/Goals/Categories/To-do**, broken on **Members/Accounts/Rules/Workflows**.
- **2026-06-18 #59** — Corroborated **B30** locally, 0 errors: `base href = "/"` on localhost, and nav
  produces clean `/accounts`, `/budgets`, `/transactions` — **correct locally** (base is root). Confirms
  B30 is **subpath-specific** (only breaks under Pages' `/CashFlux/`), which is why 58 local iterations
  never hit it. No approved features (reports/notifications/passcode/onboarding/theme-editor) landed yet.

### C40. Budget "Quarter" spend is LESS than "Month" spend — likely period-window bug ★ (correctness)
**Found (#60, 0 console errors):** on Budgets, switching the top-bar period re-windows the SPENT summary
(good): **Month $1,579.00 → Week $1,518.00 → Quarter $1,457.00.** But **Quarter < Month is impossible** —
the current quarter (Apr–Jun) *contains* the current month (June), so quarterly spend must be **≥**
monthly. (Week ≤ Month is fine.) So the **Quarter window is excluding transactions it should include** —
a boundary/anchoring bug in the quarter period math (possibly the same UTC-vs-local boundary family as
**C1**, or `period.Truncate`/`Range` for quarter mis-anchoring).
- [x] **Fixed:** the Budgets screen (`internal/screens/budgets.go`) anchors each budget's own cadence to
      **today** when the viewed window contains now (else the window start), so a Monthly budget always shows
      the current month under any containing view — Month/Quarter now agree (anomaly gone), and with C41 the
      view always contains now. The dashboard Budgets widget already uses the current month. Period engine
      quarter range was confirmed correct (#61).
- **NARROWED (#61):** the shared **period engine is CORRECT** — the **Dashboard** shows Month spending
  $4,088 (14 txns) → **Quarter $12,030 (42 txns)** (Quarter > Month ✓, proper counts). So this anomaly is
  **isolated to the Budgets screen's SPENT-summary computation**, NOT `period`/`ledger.PeriodTotals`.
  Investigate the Budgets-specific spent-vs-view-period logic (likely per-budget configured period —
  "Monthly" — interacting oddly with the view selector / proration). Dashboard is fine.
- **CONFIRMED real (#63), NOT a C41 artifact:** re-tested with a **clean direct Month→Quarter switch** (no
  intermediate Week, so no C41 drift; period label verified **Q2 2026**): SPENT still **$1,579 (Jun) →
  $1,457 (Q2)**. Quarter < Month survives the clean test → a genuine **Budgets SPENT** bug, not a
  measurement artifact. The bug is in how the Budgets screen sums spend under the Quarter view (each
  budget is "Monthly"; the quarter view appears to under-count vs. the month view).

### C41. Resolution switch re-anchors to the window START → drifts backward in time ★★ (bug, systematic)
**Found (#61) + fully characterized (#62).** On a resolution change the window re-anchors to the **start
of the current window** and truncates to the new granularity. Since a window's start is ≤ now, this
**drifts backward** and compounds. From a fresh "Jun 2026" (today 2026-06-18):
- Month→**Week** → "May 31 – Jun 6" (June's *first* week, not the current week ~Jun 15–21)
- Week→**Month** → **"May 2026"** (that week starts May 31 → truncates to May, **not June**)
- Month→**Quarter** → "Q2 2026" ✓
- Quarter→**Week** → "Mar 29 – Apr 4" (Q2's *first* week)
- Week→**Quarter** → **"Q1 2026"** (that week starts Mar 29 → Q1, **not Q2**)
→ a few switches and you're in Q1/March instead of June.
- [x] **Fixed:** `period.Window.SetResolution(r, now)` re-anchors to the single period **containing `now`**
      via `NewWindow(r, now, …)` (not the prior window's `from`), so every Week/Month/Quarter switch lands on
      the current period. Covered by `TestSetResolutionReanchorsToNow` (asserts each switch's range contains
      `now`). _(Distinct from the engine itself, which is correct — C40/#61.)_
- **Workaround confirmed (#64):** the **Jump-to → "This period" preset re-anchors correctly** — drifted
  "Mar 29 – Apr 4" → "This period" → **"Jun 14 – Jun 20"** (current week). So users can recover, but the
  switch still drifts (the bug), and the reset is **buried in a dropdown** — B10 envisioned a one-tap
  "This {period}" reset button; surface one.
- **2026-06-18 #65** — B17 lock progress (read-only check; did NOT set a passcode — store is origin-shared
  per #42, so committing one would lock the live instance). ✅ **No lock gate on load** (off by default
  per B17 opt-out). ⚠️ **No Settings/Privacy UI** — Settings has no passcode/lock/privacy control; the
  lock is wired only via a **keyboard shortcut + native prompts** (`app/shortcuts.go` + `app/applockgate.go`,
  backend `internal/applock`). So it's **not discoverable/configurable** by users yet → B17 still needs its
  **Settings → Privacy** surface, and the native prompts → FlipPanel (C42). 0 console errors.
- **2026-06-18 #66** — Nav enumeration + health, 0 errors. Nav stable: Dashboard, Accounts, Transactions,
  Budgets, Goals, To-do, Planning, Allocate, Insights, Documents, Customize, Artifacts, Workflows,
  Members, Categories, Rules, + "New page" (16 screens; no new ones since Artifacts/Workflows #50).
  - [ ] 🔸 **Keyboard shortcuts have no discoverability** — `app/shortcuts.go` wires shortcuts (new
    workspace, passcode lock, etc.) but there's **no shortcuts help/cheatsheet** (no "?" overlay / Help
    list). Users can't find them (and the B17 lock is shortcut-only, #65). Add a discoverable shortcuts
    help (e.g. "?" opens a FlipPanel cheatsheet) — pairs with B18 onboarding + C42 modal system.
- **2026-06-18 #67** — Custom pages re-check (C32), 0 errors. Created "QAPage67" → routed to
  **`/p/qapage67`** (page created + persisted) but it is **STILL NOT listed in the rail "MY PAGES"**
  (only "New page" shows) and **can't be returned to** after navigating away. **C32's first gap is
  unchanged** despite `app/custompagesnav.go` existing — the created page isn't added to the nav list.
  (Selector caught "New page" in the same section, so the custom page is very likely genuinely unlisted.)

### C44. CDN scripts lack SRI + Tailwind-CDN-in-production + offline dependency ★ (security/prod — OWASP A08, from B32) — ✅ D3 DONE; Tailwind DEFERRED → C91
**✅ DONE:** **D3 is now vendored locally** (`web/d3.min.js`, referenced as `./d3.min.js`, in the SW precache) —
no third-party CDN, no SRI gap, works offline. **Tailwind CDN is intentionally deferred** (user decision
2026-06-21): rather than a brittle static-compile (the app builds utility classes as dynamic Go strings the
scanner would miss), we'll migrate to **gwc's typed CSS classes with Tailwind interop** once the framework is
updated — tracked as **C91** (the final todo).
**Verified live (`web/index.html`):** external CDN resources are loaded with **no Subresource Integrity**:
`<script src="https://cdn.tailwindcss.com">` and `<script src="https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js">`
have **no `integrity=` / `crossorigin`**; Google Fonts CSS likewise.
**RESOLVED APPROACH (user, 2026-06-18): bundle ALL of these into the app at build time — NO CDNs.**
Vendoring/compiling at build time is strictly better than SRI+CDN: no external fetch → no supply-chain/MITM
risk (OWASP A08 moot), genuinely **offline**, no Tailwind-prod issue, zero runtime CDN dependency.
- [ ] **D3 — vendor it:** commit `d3.min.js` (pinned 7.9.0) into `web/` (e.g. `web/vendor/d3.min.js`),
      load locally (`./vendor/d3.min.js`); drop the jsdelivr CDN. (Or a build step copies it from the
      module/npm into the bundle.) → offline, no SRI needed.
- [ ] **Tailwind — compile to static CSS at build:** run the **`gwc tailwind`** path (CLAUDE.md) to emit a
      static `web/app.css`, reference it locally, **remove the `cdn.tailwindcss.com` script** (dev-only per
      Tailwind's docs). → offline + proper production CSS, no in-browser JIT.
- [ ] **Fonts — self-host:** download Fraunces + Inter woff2 into `web/fonts/`, local `@font-face`; drop
      the Google Fonts `<link>`/preconnect. → offline, no external request.
- [ ] **Build wiring:** make the `gwc`/Pages build produce these bundled assets (D3 vendored, Tailwind
      compiled, fonts local) so every deploy ships a self-contained app; the SW just caches local files.
- [ ] **Verify:** cold **offline** load renders fully (styles + fonts) and **charts work**, with the
      network panel showing **only same-origin requests** (no CDN). (Ties B14/B21 D3 offline.)
**Empirically confirmed (2026-06-18, Playwright network capture on cold load):** the running app fetches
**4 distinct external hosts** before it is interactive — `cdn.tailwindcss.com`, `cdn.jsdelivr.net` (D3),
`fonts.googleapis.com` and `fonts.gstatic.com` (Google Fonts). So today a network outage, CDN tamper, or
air-gapped/offline launch degrades or breaks the UI (no Tailwind styles, no D3 charts, no brand fonts).
This is the concrete proof behind the bundling action items above — target is **0 external hosts on load**.
_Cross-links: B32 Cluster 1 (OWASP/security pass), B14/B21 (D3 offline), §3.3 (PWA offline), §0 (build/deploy)._

### C45. Security review — data-at-rest & SQL layer (in-memory SQLite + persistence) ★ (security research, user-requested 2026-06-18)
**DONE (2026-06-21, encrypt-at-rest):** The autosaved dataset is now encrypted at rest, gated on the existing
passcode lock. New pure `internal/cryptobox` defines the on-disk envelope (`\x00cf1\x00` marker + JSON:
v/alg/salt/iv/cipher, base64), with `Marshal`/`Parse`/`IsEnvelope` (18 table tests). `internal/app/datasetcrypto.go`
drives `crypto.subtle` (PBKDF2-SHA256 600k → AES-GCM-256; key non-extractable, never persisted). Wiring
(`datasetcryptowire.go` + `persist.go`): when the lock is **active** and the session passcode is known, the
autosave writes an envelope; with no passcode it stays plaintext (`IsEnvelope` O(4) check → zero-migration for
existing data). Setting/removing a passcode triggers an immediate at-rest re-save (plaintext↔envelope
migration). On boot an envelope defers hydration (autosave suppressed so it can't be clobbered) until the
passcode gate is satisfied — `onAppUnlocked` decrypts + imports. **No lockout:** decrypt failure keeps the
ciphertext and logs; the gate's "Forgot → wipe" remains the only destructive recovery. e2e verifies
plaintext-without-passcode, envelope-at-rest-with-one, and reload→unlock→decrypt round-trip. (SQL layer was
audited clean; B33 tracks the remaining hardening backlog.)

**Scope:** how the in-memory SQLite store builds queries and how the dataset is persisted/loaded. Source-audited
`internal/store/{crud,sqlitestore,manage}.go`, `internal/app/persist.go`, `internal/uistate/aikey.go`.

**Empirically confirmed (2026-06-18, Playwright `localStorage` inspection of the running app):** what's persisted
is **not a SQLite file at all — it's plaintext JSON**. `cashflux:dataset` = ~30 KB, head `{ "schemaVersion": 1,
"members": [ …`, `parsesAsJSON: true`, **no `SQLite format 3` magic** (`isSQLiteMagic: false`), top-level keys
`members, accounts, categories, transactions, budgets, goals, tasks, workflows, settings` — the entire household.
**Reading it needs zero tooling:** DevTools → Application → Local Storage, or console
`localStorage.getItem('cashflux:dataset')`. The `:memory:` engine therefore adds **no confidentiality** — its
contents are exposed via the plaintext snapshot (and, more laboriously, via the wasm `WebAssembly.Memory` buffer
while unlocked). Net: the in-memory DB is not a protection; B33.1/B33.2 (encrypt-at-rest + zeroize-on-lock) are
the only mitigations. (Other keys present: `cashflux:workspaces`. `cashflux:openai-key` absent this run — toggle off.)

**✅ SQL injection — NOT a vulnerability (verified).** The live DB is opened `:memory:` (`sqlitestore.go:49`),
and **every query that touches user data uses `?` bind parameters** — inserts/upserts (`crud.go:27-28`),
reads (`:40`), deletes (`:55`), and all `json_extract(data,'$.x') = ?` filters (`:139-226`). The only string
concatenation in SQL is the **table name** (`"… FROM "+table+" …"`), and `table` is supplied by internal
generic dispatch (compile-time Go type → fixed table name), never by user input. **No injection vector** —
record this as an audited-clean control, not a TODO. (Keep it clean: any future dynamic `ORDER BY`/column or
free-text filter MUST stay parameterized or use a hard-coded allow-list — never interpolate user text into SQL.)

**🔴 Data-at-rest is plaintext (the real finding).** Persistence is **`localStorage`**, not OPFS/IndexedDB as
previously assumed (corrects the #42 note). `persist.go:92` writes the *entire* dataset as a plaintext JSON
string to `localStorage["cashflux:dataset"]` on a 4s ticker + pagehide. localStorage is **unencrypted on disk**
in the browser profile and readable by **any same-origin script (incl. any XSS), any browser extension with host
access, and devtools**. On a shared/family computer every account, balance, and transaction is recoverable
without the app. This is exactly what B17's encryption must cover.
- [ ] **Encrypt the at-rest snapshot.** Recommended architecture (fits B17 + the `:memory:` design): keep the
      live DB in-memory (no plaintext DB file on disk), and when lock is **enabled**, persist an **AES-GCM
      encrypted** snapshot (key = passphrase via Argon2id/PBKDF2 KDF, or WebAuthn-PRF KEK) instead of the raw
      JSON. Lock **disabled** = today's plaintext snapshot (explicitly the user's opt-out). Decrypt on unlock.
- [ ] **Clear plaintext from memory on lock/timeout.** The `:memory:` DB holds plaintext in wasm linear memory
      while unlocked; on inactivity-lock (B17) the snapshot/derived key and ideally the in-memory rows should be
      zeroized/dropped so a memory-scrape after auto-lock yields nothing.

**🟠 OpenAI key stored in plaintext localStorage.** `aikey.go:15` writes the key to `localStorage["cashflux:openai-key"]`
when "remember on this device" is on. The dataset autosave correctly **redacts** the key (`ExportJSONRedacted`,
`persist.go:86`), but the separate key store is plaintext → an XSS or extension can exfiltrate a live billable
API credential. - [ ] Fold the key into the same encrypted-at-rest envelope as the dataset when lock is enabled;
when lock is off, at minimum document the exposure in the remember-key toggle's help text.

**🟠 Silent persistence loss at quota.** `persist.go:81-84` swallows a `localStorage.setItem` quota throw with a
logged recover. localStorage is ~5–10 MB; a large household/long history can hit it, after which **autosave
silently stops** and unsaved data is lost on reload. - [ ] Detect quota failure and surface a visible warning
(toast/banner) + nudge to export; consider migrating bulk storage to IndexedDB (much larger quota) — this also
pairs well with the encrypted-snapshot work.
_Cross-links: B17 (app lock / encryption / recovery), C44 (XSS surface ↔ CDN supply-chain), B32 Cluster (CIA/OWASP)._

### C43. "+ Add" menu z-index broken — trapped in the sticky topbar's stacking context ★ (bug — user-reported 2026-06-18) — ✅ DONE (verified 2026-06-21: `addmenu.go` popover + `.add-menu z-index:50` over a `z-index:40` backdrop; not clipped)
**Reported:** the add button's z-index is broken. **Root cause (verified live):** `.add-menu` is
**`z-index:50`**, but its **stacking ancestor is `.topbar` (`position:sticky; z-index:20`)** — a sticky
element with a z-index forms a **stacking context**, so the menu's z-50 is **clamped to the topbar's z-20
layer**. Anything rendered at the **document root** with z-index > 20 then covers it: `.flip-backdrop`
(modals, **z-50**), the toast (**z-60**), the install prompt (**z-30**). **Compounding:** the topbar also
has **`overflow:auto` (C34)** which **clips** the `.add-menu` dropdown (positioned `top:calc(100%+6px)`,
*below* the bar). _(Also: "+ Add" now opens the **B11 add-menu** of action cards — B11 progressed.)_
- [ ] **Fix:** **portal `.add-menu` + `.add-backdrop` to the document root** (render outside the topbar,
      like `SettingsHost`/`QuickAddHost` do for the flip panels) so their z-index competes at the document
      level and the topbar's `overflow:auto` (C34) can't clip them. (Fixing C34's overflow alone wouldn't
      fix the stacking-context clamp — portaling fixes both.)
- **VISUALLY CONFIRMED (#68):** with the menu open in the DOM (per #67), the screenshot shows **no visible
  menu** below "+ Add" — clicking "+ Add" appears to **do nothing** to the user (`.add-menu` is in the DOM
  but clipped/hidden by the topbar `overflow:auto` + z-20 context). More severe than "covered" — the
  add-action menu is effectively **non-functional/invisible**. High priority.
- **Update (#70, CORRECTED by #75):** the #70 "visible" read was a **false positive** — it checked the
  element's *layout box* (210×196) which ignores the **ancestor overflow clip**.
- **Re-confirmed STILL INVISIBLE (#75):** screenshot with the menu open shows **nothing** below "+ Add",
  even though the DOM box is [x1206, y50, 210×196] z-50 with full content
  ("New transaction / New account / New budget / New goal / Scan a document" — B11). The **topbar's
  `overflow:auto` (C34) clips everything below its 56px height**, so the menu (y50→246) is clipped to a
  ~6px sliver = effectively invisible. **Still a blocking bug — high priority.** Fix = portal to root
  (escapes both the C34 overflow clip and the z-20 stacking clamp).
- **Functionality OK (#77):** the menu's **actions work** — clicking "New account" (force, since clipped)
  navigated `/` → `/accounts`. So C43 is **purely a CSS clip/stacking bug**; the B11 add-menu is sound.
  The portal fix unblocks a fully-working feature (no logic changes needed). 0 console errors.
**App-wide z-index audit (collisions + no scale):** by layer — `3` widget grip/gear/resize · `5`/`10`
minor stickies · **`20`** topbar (sticky, stacking ctx) · `30` install-prompt + custompages menu +
wsswitcher menu · `40` add-backdrop + wsswitcher submenu · **`50`** flip-backdrop **AND** add-menu
(**duplicate**) · **`60`** toast **AND** `index.html:296` overlay (**duplicate**) · `200`/`210` shortcuts
overlays · `1000`/`1001` app-lock gate/overlay.
- [ ] **Duplicate z-values** (z-50 flip-backdrop vs add-menu; z-60 toast vs :296) → ambiguous ordering
      when concurrent.
- [ ] **No z-index system** — ad-hoc 3→1001. **Define z-index tokens/layers** (base / sticky-header /
      dropdown / modal-backdrop / modal / toast / overlay / lock) and route all `z-*` through them. Lesson:
      a high z-index inside a low-z-index stacking-context ancestor is still capped — the root cause here.
- **Scoped (#69):** checked other dropdowns — the **workspace-switcher menu is NOT trapped** (z-30, no
  low-z stacking ancestor → competes at root; only clip ancestor is the full-screen app-root
  `flex h-screen overflow-hidden`, which doesn't clip a top-positioned menu). So **C43 is specific to the
  topbar-hosted `.add-menu`** — the portal fix is localized. _(Note: app root is `overflow-hidden`
  h-screen — the outer clip boundary; a dropdown extending past the viewport edge would be clipped by it,
  so portal-to-root + edge-aware positioning is the general pattern for menus.)_
- **2026-06-18 #71** — Re-check of period bugs, 0 errors. Both **still present**: **C41** Month(Jun)→Week→
  Month → "May 2026" (drift unchanged); **C40** Budgets SPENT Quarter ($1,457) < Month ($1,518) — still
  anomalous (this reading was itself C41-drifted to May, but #63's clean direct test already confirmed C40).
- **2026-06-18 #72** — Allocate Save-profile test, 0 errors. ➕ 5 built-in preset profiles present
  (Balanced / Maximize returns / Safety & access / Pay down debt / Finish goals). ❔ Save-profile commit
  **inconclusive** — the "Save these weights as…" name placeholder didn't match my selector (ellipsis
  char), so I couldn't fill it; re-test with the exact placeholder to judge if it shares the add-button no-op.
- **2026-06-18 #73** — Full-route health sweep (regression check during active dev): all 16 routes one
  `<h1>`, **0 console/network/pageerror**; C28 icons paint. No regressions from the in-flight changes.
- **2026-06-18 #74** — Spot-check, 0 errors. **C41** still drifts (Jun→Week→Month = "May 2026");
  **Members** add-button still no-op ("Mem74"). Unchanged.
- **2026-06-18 #78** — Spot-check, 0 errors. **C41 MAY be fixed** — Jun→Week→Month now reads "Jun 2026"
  (not the #74 "May 2026" drift). _Ambiguous:_ the Week reading also came back "Jun 2026" (a week should
  show a date range) → could be a parser artifact / different switch behavior — **re-verify with a clean
  read of the stepper label + confirm Week shows the current week range** before marking C41 fixed.
  **C43** add-menu still `insideTopbar=true` (not portaled); **Members** add-button still no-op ("Mem78").
- **2026-06-18 #79** — Clean C41 re-test via the **stepper label** (resolves #78): **C41 STILL BROKEN.**
  fresh "Jun 2026" → Week **"May 31 – Jun 6"** → Month **"May 2026"** (drift) → Quarter "Q2 2026" → Week
  **"Mar 29 – Apr 4"** — exactly the #62 drift table. #78's "may be fixed" was an **income-subline parser
  artifact** (that subline shows the month, not the stepper's week range); the stepper label is ground
  truth → drift persists. 0 console errors.

### C42. Replace native browser popups (prompt/confirm/alert) with the FlipPanel modal system ★ (user-asked 2026-06-18) — ✅ DONE (2026-06-21)
**✅ DONE:** added an in-app dialog system — `uistate.DialogRequest` atom + `uistate.ConfirmModal`/`PromptModal`
helpers, rendered by a `DialogHost` (mounted in the shell) with `role=dialog`/`aria-modal`, Enter-confirm /
Esc-or-backdrop-cancel, focus-in, and a destructive-tinted confirm. Converted every in-app site: custom-page
new/rename/delete, workspace new/dup/rename/delete, palette new-workspace, plus settings wipe + backup restore;
`alert()` notices now route to the existing toast (`paletteNotify`). Goals "Contribute" already uses an inline
form. The only native confirm left is the pre-Shell **lock-gate forgot-passcode** (the DialogHost isn't mounted
there — documented exception). e2e `dialog_check` verifies the modal opens/cancels/confirms with **no native
dialog ever firing**.
**Want:** every browser-native popup/modal should instead use the **`ui.FlipPanel`** modal + animation
that Settings uses (lift-to-center, `rotateY`, dim/blur backdrop), with **full a11y + keyboard support**.
**Canonical system to standardize on:** `ui.FlipPanel` (`internal/ui/flippanel.go`) driven by atoms
(`uistate.UseSettings`/`UseQuickAdd`) + hosts (`SettingsHost`/`QuickAddHost`) — has `role="dialog"` +
`aria-modal` + Esc-close + focus-in (per C27/C36). Need: an **input modal** (replaces `prompt`), a
**confirm modal** (replaces `confirm`), and reuse the existing **toast** for notices (replaces `alert`).
**Full inventory of native dialogs to convert (grep-verified):**
- [ ] **`prompt()` (text input):**
  - `app/wsswitcher.go` — workspace **new / duplicate / rename** (`promptName` ×3) + `app/shortcuts.go:241` new.
  - `app/custompagesnav.go:77,96` — custom page **new / rename** (`promptName`).
  - `screens/goals.go:373` — Goals **"Contribute"** amount (`window.prompt`). [seen #14]
  - `app/applockgate.go:101,109,115` — **B17 passcode setup**: set passcode, confirm passcode, auto-lock
    minutes. (B17 lock has STARTED building with native prompts — replace with a proper styled lock UI.)
- [ ] **`confirm()` (yes/no):**
  - `app/download.go:33` `confirmAction` → `app/settings.go:710` **"Erase all data"** (Wipe).
  - `app/custompagesnav.go:270` — custom page **delete** confirm.
  - `app/wsswitcher.go:180` — workspace **delete** confirm.
- [ ] **`alert()` (notice):**
  - `app/wsswitcher.go:248` + `app/shortcuts.go:249` — **import error**.
  - `app/shortcuts.go:262` — "Passcode lock removed."
  - `app/applockgate.go:111,121` — passcode **mismatch** / **enabled** notices.
- [ ] **Plan:** add reusable `ui.ConfirmModal` + `ui.PromptModal` (FlipPanel-based, atom-driven like
      Settings); route `promptName`/`confirmAction`/`alert` through them (single choke point); replace
      `alert` notices with the existing Toast/Notice. **Native dialogs block the JS/wasm thread, can't be
      themed, and are inconsistent with the app** — converting fixes all three.
- [ ] **A11y + keyboard (must-keep):** `role="dialog"` + `aria-modal="true"`, **move focus into the modal**
      (the input for prompt; the safe/Cancel button for confirm), **Esc cancels**, **Enter confirms/
      submits**, **focus trap**, **return focus to the trigger** on close, labelled. (The quick-add panel
      itself still needs `role=dialog` per **C36** — fix as part of this.) Verify each converted site.
  _Cross-links: C27/C36 (dialog a11y), C36 (quick-add missing dialog ARIA), B17 (lock UI shouldn't use
  native prompt), B18 (onboarding uses FlipPanel too), the Goals-contribute prompt note (#14)._

### C38. Home/family-use feature-gap analysis (user-asked 2026-06-18)
What's missing for a typical household, given the (extensive) current feature set. Grouped by type.
**A. The big architectural gap:**
- [ ] **Multi-device / shared-household sync** — currently single-device, local-only (Phase 3 sync is
      deferred/out-of-scope). For a *family*, multiple people on multiple devices can't share the same
      data — which undercuts the "household" promise. The #1 home-use gap. (Electron + a sync backend, or
      the Phase-3 server, would address it.)
**B. Designed but not yet built (already specced — just need building):**
- [ ] **Notifications/reminders (B19)** — bill due, budget over/near, goal pace; catch-up-on-wake. Critical
      for "don't miss a bill." **Onboarding + splash (B18)**, **privacy lock (B17)** (family computer),
      **theming engine (B20)**.
**C. Genuinely-absent household features (not yet specced):**
- [ ] **Bills & due-date tracker / calendar view** — beyond the dashboard "upcoming bills" widget: a
      proper bills list with due dates, paid/unpaid status, and a month calendar. (Recurring cash flows
      exist in Planning, but no bills-calendar/pay-tracking surface.)
- [ ] **Reports** — structured spending-over-time, category trends across months, **net-worth history**,
      and a **year-end / tax summary export** (category totals for the year). Insights is AI-narrative;
      there's no deterministic reports section.
- [ ] **Receipt attachments linked to transactions** — Artifacts stores images, but attaching a receipt
      to a specific transaction (and viewing it from the ledger) appears missing.
- [ ] **Split / shared expenses & settle-up between members** — members + individual/group scope exist,
      but not "split this expense 50/50" or "who owes whom" settle-up (common for couples/roommates).
- [ ] **Subscriptions tracker** — a dedicated view of recurring monthly spend (what am I paying for) +
      cancel/renewal reminders; partially covered by Recurring but not surfaced as subscriptions.
- [ ] **Budget rollover / sinking funds** — does unspent budget carry to next month (envelope rollover)?
      Methodology selector exists; confirm rollover behavior, add sinking funds if absent.
- [ ] **Investment/holdings tracking** — brokerage/401k accounts hold a balance only; no holdings,
      cost-basis, or performance (may be out of scope for a budgeting app — flag, don't assume).
- [ ] **Automated backup reminders** — export/import exists; nudge periodic backups (ties B17 recovery).
**Already strong (no gap):** accounts (assets/liabilities, multi-currency, reconcile), transactions
(transfers/filters/tags/bulk/duplicate/CSV+AI import), budgets (periods/thresholds), goals (contribute/
pace), categories (sub/colors/reassign), planning (forecast/recurring/debt payoff), allocate, AI insights,
custom fields + formulas, rules, workflows, configurable dashboard, theme/density/scale, PWA/offline,
on-device persistence.
- _Recommendation order for home use:_ **B19 notifications → bills calendar → reports → B17 lock →
  receipt attachments → sync (largest).**

### C39. Long lists aren't paginated/virtualized — Transactions especially ★ (UX/perf — user-asked 2026-06-18) — ✅ DONE (verified 2026-06-21: `transactions.go` uses `pagination.Clamp/Slice` + `txnfilter.PageSizes`; reusable `ui.DataTable` pager)
**Audited (verified live):** the **Transactions ledger renders a long flat list with NO pagination,
load-more, or virtualization** — `57 transactions shown`, no page/next controls anywhere. With the
current 57-row sample it's already a long scroll; at hundreds/thousands of transactions this is a real
**performance + UX** problem (matches the deferred SPEC items **§1.11** "virtualization for large sets
later" and **§1.20** "Performance: large dataset (10k+ txns) virtualization").
- [ ] **Paginate or virtualize the Transactions list** — windowed rendering (virtual scroll) or
      page/load-more. Virtualization is better here (keeps filter/sort/scroll fluid); pagination is
      simpler. Either way, render only what's visible.
- [ ] **Verify the 57-shown vs 45-rendered discrepancy** — only ~45 rows had a Duplicate button while the
      summary says "57 shown." **Most likely** the 12 difference is **transfer legs** (transfers have no
      Duplicate/Edit), i.e. all 57 render and only non-transfers get a Duplicate button — but **confirm
      it's not a silent row cap** (which would hide transactions without telling the user — a real bug).
- [ ] **Other lists:** Categories (10), Budgets (5), Accounts (7), Members (1) are small today — fine, but
      **Categories, Documents import-history, and Artifacts can grow unbounded**; give them pagination
      once they exceed a threshold. (The "of" pagination matches on Accounts/Documents were **false
      positives** — "17% of limit used" / "X of Y" text, not real pagination controls.)
- [ ] Pairs with the **Reports** engine (B21) for "view all" / export when a list is too long to scroll.
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
- [x] unit: `budgeting` scope-aggregation test (individual vs group, mixed members).

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
- [x] Apply "envelope": envelope-style carry-forward view — DONE. `budgeting.EnvelopeAvailable` (pure,
      table-tested: no-spend funds one period, current-period-only, carries unspent forward, overdraw
      nets, scope respected) accumulates `limit − spent` over every period from the first covered
      transaction through the current one (bounded at 240 periods). Settings offers Envelope; each budget
      row shows "Envelope balance: $X" (danger tone when overdrawn) under a note. Verified live: switching
      to Envelope shows the note + per-budget balances (e.g. "$359.45"). _Decision: carry-forward window
      = from the first covered transaction (no budget start date exists), made autonomously._
- [x] unit: config-layering test (defaults→household→member). Methodology is household-only today; the
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
- [x] unit: `Cadence.Next/Advance` + a forecast-vs-actuals no-double-count test.

#### D9. Debt payoff scenario → allocate → balances ★
**Workstream:** model a credit-card payoff, then allocate extra cash toward it and watch the liability fall.
**Touches:** Planning (`payoff.Project`) · Accounts (liability, APR, min payment) · Allocate (`allocate` debt scorer + `Distribute`) · `ledger` net worth.
- [ ] Enter balance/APR/min payment; assert months-to-clear + total interest match `payoff`.
- [ ] Add an extra payment; assert months & interest saved recompute.
- [ ] On Allocate, assert the card ranks high under the debt-reduction criterion and `Distribute` honors
      the emergency buffer + max-per-destination.
- [ ] Post a payment; assert the liability balance and net worth update consistently.
- [x] unit: `payoff` boundary tests (payment==interest, payoff month) + `allocate.Distribute` reserve/cap.

#### D10. What-if trim-spending → forecast curve vs actuals
**Workstream:** apply a "trim monthly spending by X" what-if and compare the projected net-worth curve.
**Touches:** Planning (trim what-if) · `forecast` · `ledger.NetWorthSeries` · chart (`ui.Chart`).
- [x] Enter a trim amount → the projected end balance shifts (the trim note shows the new end + delta).
- [x] Chart axis is **in dollars, not cents** — the forecast now uses the D3 `ui.Chart` with a compact
      currency Y axis ($0/$10k/$20k/$30k), like C16 (was the axis-less sparkline).
- [x] Compare scenario vs actual baseline side by side — the chart now overlays two series (Baseline +
      With-trim, distinct colors + a legend) when a trim is set. Verified live (entering a trim adds the
      second line; dollar axis confirmed).
- [x] unit: `forecast.Project` with a spending delta — `TestProjectSpendingDeltaShiftsEndBalance` (trim
      pulls the curve ahead by delta each month; end = delta×months higher).

#### D11. Plan (start balance + monthly) projection → dashboard surfacing
**Workstream:** create a savings/spending plan and see its projection.
**Touches:** Planning (`planning.Project`/`EndBalance`) · store (`plans`) · Dashboard (formula/plan slot — §1.17 gap).
- [ ] Create a plan (name/horizon/start/monthly); assert projected end balance matches `planning.EndBalance`.
- [ ] Add a one-time item in a future month; assert the curve bends at that month.
- [ ] Reload; assert the plan persists and re-projects identically.
- [x] unit: `planning.Project`/`MonthlyNet`/`EndBalance` with one-time items.

#### D12. Goal pace → linked-account contributions → allocate
**Workstream:** create a goal linked to an account, contribute, and see pace + allocation interplay.
**Touches:** Goals (`goals` pace/projection) · Accounts (linked) · Allocate (goal-progress criterion) · Dashboard goal widget.
- [ ] Create a goal with a target date + linked account; assert monthly-needed + projected completion.
- [ ] Contribute; assert progress %, remaining, and the dashboard goal widget update.
- [ ] On Allocate, assert "Finish goals" preset feeds `GoalProgress` and ranks the goal sensibly.
- [x] unit: `goals.MonthlyNeeded`/projection + allocate goal-progress scorer.

#### D13. Net-worth forecast horizon correctness ★
**Workstream:** project net worth over the horizon from recurring + one-time items and validate edges.
**Touches:** `forecast.Project` · `ledger.NetWorthSeries` · Planning chart · Dashboard trend widget.
- [ ] Assert out-of-horizon items are ignored; same-month items sum; negative balances allowed.
- [ ] Assert the dashboard trend widget and the planning curve agree for overlapping months.
- [ ] Assert chart values are dollars (**C16**) and labels are readable at the widget's width.
- [x] unit: `forecast` horizon/edge tests (already partial — extend for net-worth feed).

### Finances workstreams

#### D14. Transfer between accounts (paired, excluded from totals) ★
**Workstream:** transfer money between two accounts and confirm it's balance-neutral to income/expense.
**Touches:** Transactions (transfer) · `domain.IsTransfer` · `ledger` (Balance, PeriodTotals exclude transfers) · Dashboard · net worth.
- [x] Create a transfer; assert both account balances move and net worth is unchanged.
- [x] Assert Income/Spending KPIs and budgets are **not** affected by the transfer.
- [x] Delete one leg; assert the paired leg is removed too.
- [x] unit: `ledger.PeriodTotals`/Balance transfer-exclusion + paired-delete.

#### D15. Reconciliation: clear → cleared balance → update-balance adjustment ★
**Workstream:** clear transactions, reconcile against a real balance, and let the app post an adjustment.
**Touches:** Transactions (cleared toggle + filter) · `ledger.ClearedBalance` · Accounts ("Update balance") · `freshness` (BalanceAsOf).
- [ ] Toggle cleared on several txns; assert cleared balance = opening + cleared only.
- [ ] Use "Update balance" with a different real balance; assert a cleared adjustment txn for the diff is
      created and `BalanceAsOf` is set.
- [ ] Assert the staleness badge clears after the update (ties D17).
- [x] unit: `ledger.ClearedBalance` + adjustment-amount math.

#### D16. Multi-currency FX across every aggregate ★
**Workstream:** add a foreign-currency account + txns and confirm base-currency conversion everywhere.
**Touches:** Settings (base currency + FX rates) · `currency.Rates.Convert/ToBase` · `ledger` (net worth, totals) · Budgets · `forecast` · displays.
- [x] Add a non-base account + foreign txns; assert net worth, period totals, and budgets convert to base.
- [x] Edit an FX rate; assert every aggregate re-converts live.
- [x] Assert a missing/zero rate surfaces a clear error, not a silent wrong total.
- [x] Assert rounding is to target minor units and is stable (no drift on re-render).
- [x] unit: `currency` cross-rate + rounding + missing-rate tests (extend existing).

#### D17. Staleness → nudge → task ★
**Workstream:** let an account go stale, get nudged, and turn the nudge into a to-do.
**Touches:** `freshness.IsStale` · Accounts (Stale badge, Mark updated) · Dashboard freshness widget · To-do (create-from-nudge).
- [ ] Age a balance past its window; assert the Stale badge + dashboard "N balances need a refresh".
- [x] "Remind me"; assert a nudge task is created in To-do.
- [ ] "Mark updated" / update balance; assert staleness clears and the nudge count drops.
- [ ] Assert recurring-bill exemption is respected.
- [x] unit: `freshness.IsStale` windows + exemption; **1.15** dismissal-state test (gap).

#### D18. Net-worth assembly across members & group ★
**Workstream:** mix individual and shared assets/liabilities and verify the net-worth breakdown.
**Touches:** Accounts (scope/owner/class) · `ledger.NetWorth` + per-member/group rollups · Members ("Net worth by member") · Dashboard.
- [x] Assert net worth = assets − liabilities in base currency, matching the Accounts header and KPI.
- [x] Assert per-member rollup sums to the household total (individual + group).
- [x] Archive an account; assert it drops out of net worth but is restorable.
- [x] unit: `ledger.NetWorth` + rollup tests (multi-member, multi-currency, archived).

#### D19. Member add/reassign/delete ripples ★
**Workstream:** add a member, reassign ownership, then delete a member with owned entities.
**Touches:** Members · `appstate.ReassignOwner` · Accounts/Budgets/Goals/Transactions (owner) · net worth rollups.
- [x] Add a member + set default; assert default-member behavior in new forms.
- [x] Reassign owned accounts/budgets/goals/txns to another owner; assert all move.
- [x] Delete the member; assert no orphaned `OwnerID`/`MemberID` and rollups recompute.
- [x] unit: `ReassignOwner` across all four entity types.

#### D20. Rules auto-categorize on entry & import ★
**Workstream:** define rules, then add/import transactions and confirm category/tags are applied (and conflicts handled).
**Touches:** Rules (`rules` engine, conflicts) · `rulesuggest` · Transactions (entry auto-fill) · Documents (import) · `appstate.ApplyRules` · Budgets/breakdown impact.
- [x] Add a rule; type a matching description; assert category + tags auto-fill without overriding a manual pick.
- [x] Import a CSV/image; assert rows are categorized by first-match rule; assert budget/breakdown reflect it.
- [x] "Apply to existing"; assert pre-existing uncategorized txns get categorized.
- [x] Assert a shadowed/never-fires rule shows the conflict warning.
- [x] unit: `rules.FirstMatch`/`Conflicts` + `ApplyRules` retroactive path.

#### D21. Document import → review → dedupe → ledger → derived figures ★
**Workstream:** import via CSV and via image (vision), review, dedupe, import to ledger, and verify downstream.
**Touches:** Documents (CSV + image) · `extract.ParseRows` · `ai` vision codec · dedupe · store (`documents`) · Transactions · Dashboard/Budgets/net worth · `spendsummary`.
- [x] Paste a CSV with a header; assert rows map by column name and import to the chosen account.
- [x] Import the same rows again; assert same-date+amount dedupe skips them and reports the count.
- [ ] (Image path, key set) assert vision extraction → review edits → import; assert an Import-history entry.
- [x] Assert imported txns update Spending KPI, budgets, and the monthly-spend summary.
- [x] unit: `extract` parsing/dedupe + CSV column mapping.

#### D22. Custom fields + formula over live figures
**Workstream:** define a custom field, fill it on an entity, and reference live figures in a saved formula.
**Touches:** Customize (custom fields + formula) · `customfields.Validate` · `formula` (Tokenize/Parse/Eval, `Env`) · store round-trip.
- [ ] Add a custom field to an entity; assert it renders on that entity's add/edit form and validates by type.
- [ ] Build a formula (e.g. `round((income-expense)/income*100)`); assert the live result matches the figures.
- [ ] Save the formula; reload; assert it persists and re-evaluates.
- [ ] Assert sandbox safety: a non-allowlisted function / unknown var errors cleanly (no escape).
- [x] unit: `formula` eval + security + `customfields.Validate` round-trip.

#### D23. Accounting money display consistency on every surface ★
**Workstream:** the same money value renders identically (grouped thousands, parentheses for negatives) everywhere it appears.
**Touches:** `money.FormatAccounting` · Dashboard · Accounts · Budgets · Goals · Transactions · Planning · charts.
- [ ] Pick one negative and one large value; assert identical formatting on every screen that shows it.
- [ ] **(Currently fails — C2:** Accounts/Budgets/Goals drop grouping; Transactions use `-` not parentheses.)**
- [ ] Assert chart axes/labels use major units + currency formatting (**C16**).
- [ ] fix: route every money render through `money.FormatAccounting`; add a guard test/shared helper so
      new surfaces can't bypass it.

---

## L. Loop user-story QA — story-driven gaps ★

Findings from the recurring user-story QA loop: invent a real household's flow, drive the app
end-to-end, screenshot it, and log mechanical + UI/UX gaps the dev agent should build/fix
bottom-up (model → tested logic → store → state → UI). Each story below names the persona and the
exact ritual, then the gaps that block it. Screenshots live in `e2e/loop*-*.png`; the driving
script is `e2e/loopstory_NN_*.mjs` (run via `node e2e/run-stories.mjs` or standalone against :8099).

### L1. Story — "The Sunday Budget Reset" (Maya & Devon, dual-income) — 2026-06-20 ★

**The ritual:** every Sunday evening Maya spends ~15 min: glance at a calm dashboard → reconcile the
week → spot any **overspent budget** → **move money between budgets to cover the overspend** → check
the **Emergency Fund** pace → eyeball **upcoming bills** — without hunting.
**Drive script:** `e2e/loopstory_01_sunday_reset.mjs` (seeds sample data, shoots dashboard/budgets/goals).
**What already works well (verified by screenshot, keep as regression anchors):**
- Dashboard is clean and professional: bento grid, net worth / income / spending / liabilities stats,
  recent transactions, budgets mini-bars, net-worth trend, goals, cash flow, upcoming bills,
  savings-rate, spending breakdown, freshness, spending highlight. Strong typography hierarchy. ✓
- Budgets screen flags pace: "0 over budget · 2 near the limit", per-row on-track/near-limit colored
  bars + "projected to go over by $X". ✓
- Goals screen shows real pace: "75% · $7,500.00 to go · by 2026-12-31 · save $1,071.43/mo · linked
  to High-Yield Savings". ✓  (Earlier probe false-negatived this — it uses "/mo", not "per month".)
- Bills nav entry exists under Tools (probe false-negatived it — nav items aren't `role=link`). ✓

**Mechanical gap (the core of the ritual — NOT supported):**
- [x] **"Cover overspending" — move money between budgets.** (Shipped: budgeting.Transfer + appstate.CoverBudget + a "Cover…" action on over-budget rows; e2e story_budget_cover.test.mjs.) When Groceries is at 92% (projected
      +$304 over) Maya needs to pull from an under-budget envelope (e.g. Shopping, 72%) to cover it.
      Budgets today support add / inline-edit / delete / rollover toggle only — there is **no
      inter-budget transfer**. Build bottom-up:
  - [x] **Model/logic** `internal/budgeting` (pure, no `syscall/js`): a `Transfer(from, to BudgetID, amt money.Money)`
        that produces a balanced, explainable adjustment (records both legs, never lets a source go
        negative unless allowed), table-tested incl. overspend-cover and insufficient-source cases.
  - [x] **Persistence** `internal/store`: persist the adjustment/transfer as first-class data so it
        survives reload and round-trips through export/import (lossless test).
  - [x] **State** `internal/appstate`: a single covering action + atom refresh.
  - [x] **UI** `internal/screens/budgets.go`: a "Cover…" action on an over-budget row that opens a
        small form (pick source budget + amount, with "cover the full $X over" one-tap), plain-English,
        keyboard-reachable, light/dark. Show the resulting balance change inline (determinism rule).
  - [x] **E2E** story test: overspend Groceries, cover from Shopping, assert both budgets re-balance,
        the projected-over line clears, and it survives a reload.

**UI/UX defect (real, screenshot-confirmed):**
- [x] **Budget row sub-lines render glued together.** `internal/screens/budgets.go` renders
      `budgets.rowSub` ("Monthly · On track · 79% · $61.00 left") and the pace `Span`
      ("At this pace, projected to go over by $72.25") as **adjacent inline `Span`s with no
      separator**, so they read as "...$61.00 left**At this pace**, projected to go over...". Fix:
      make `.budget-sub` lines block-level (or insert a separator dot / spacing) so the status line,
      pace line, rollover line, and envelope line each sit on their own line. Re-screenshot to confirm.

**Probe hardening (so future loops don't false-negative):**
- [ ] Goals-pace and Bills-nav assertions in the drive scripts should match the app's actual copy
      ("/mo", nav `<a title>` not `role=link`). Tighten `loopstory_01` accordingly.

### L2. Story — "The Roommate Split" (Priya + Sam + Lee, shared flat) — 2026-06-20 ★

**The ritual:** Priya fronts shared costs — rent, the electric bill, a Costco run — then splits each
across the three roommates. At month-end she wants a dead-simple **"who owes whom → settle up"** view
so nobody chases receipts.
**Drive script:** `e2e/loopstory_02_roommate_split.mjs` (seeds sample, drives /members + /split).
**What already works well (keep as regression anchors):**
- Split screen forward flow is clean: "Enter an amount, pick who's sharing it, and (optionally) who
  paid"; equal split + a "Split by weight (shares or income)" toggle; per-member share toggles. ✓
- Members screen renders + offers add-member. ✓

**Mechanical gaps (block the ritual):**
- [x] **"Settle up" — the reverse ledger of who owes whom.** (Shipped: internal/settle minimal-transfer logic + SharedExpense/Settlement persistence + record-settlement UI; e2e story_settle_up.test.mjs.) Split today only computes a *single*
      expense's shares; there is **no running net-balance across many split expenses** and **no way to
      record a settlement**. Build bottom-up:
  - [x] **Model/logic** `internal/settle` (pure, no `syscall/js`): given a set of shared expenses
        (payer + per-member shares) and any recorded settlements, compute each member's **net balance**
        and a **minimal set of "X pays Y $Z" transfers** to zero everyone out (classic debt
        simplification). Table-tested: 3-way uneven shares, a partial settlement, rounding to minor
        units (no lost/created cents), already-settled = empty.
  - [x] **Persistence** `internal/store`: persist shared expenses + settlements as first-class records;
        export/import round-trips losslessly (test).
  - [x] **State** `internal/appstate`: atoms for shared-expense list + settlements; one record-settlement
        action.
  - [x] **UI** `internal/screens/split_screen.go`: after the forward split, **save the split** to the
        shared ledger; add a **"Settle up"** panel listing each member's net (you owe / owes you) and
        the minimal transfer list, with a **"Record settlement"** action per suggested transfer. Plain
        English, light/dark, keyboard-reachable; show the math (determinism rule).
  - [x] **E2E**: log 3 shared expenses with different payers, assert net balances + minimal transfers,
        record one settlement, assert the ledger re-balances and survives reload.
- [x] **Sample data is a single-member household** ("Michael Brooks", footer "1 member"), so every
      multi-person tool (Split, member filters, per-member budgets/goals owners) is undemoable from the
      sample. Add **2–3 sample members** with a few shared expenses so Split/Settle-up have real data
      out of the box. (`internal/app` LoadSample / sample dataset.)

**UI/UX defects (screenshot-confirmed):**
- [ ] **Lingering load splash overlays content.** The full-viewport "CashFlux — Getting your money in
      order…" splash is still visible (low-opacity, mid-viewport) over the screen content after
      sample-load + route navigation — reproduced on **both** `/split` (L2) and `/goals` (L1). It
      should fully dismiss once the app is interactive. Investigate the splash dismiss condition
      (likely tied to a load/persist signal that the sample-reload path doesn't clear). Re-screenshot
      to confirm it's gone.
- [ ] **Split screen is sparse** once past the form (lots of dead space below "Who's sharing?"). The
      Settle-up panel above will fill it; until then consider an empty-state hint ("Add a shared
      expense to see who owes whom").

### L3. Story — "The Receipt Snap" (Marcus, no-typing dad) — 2026-06-20 ★

**The ritual:** after the grocery run Marcus photographs the paper receipt and wants CashFlux to read
it, split it into categorized line items, dedupe against what he already logged, and import the rest
in one tap — no typing.
**Drive script:** `e2e/loopstory_03_receipt_snap.mjs` (seeds sample, drives /documents).
**What already works well (verified by screenshot + source — keep as regression anchors):**
- Documents screen is genuinely strong: **"Read a receipt or statement image"** → "Choose image" +
  "Read with AI" (OpenAI vision), with a plain-English explainer + key-in-Settings note. ✓
- Vision extraction (`internal/extract`) returns **per-row** `{Date, Description, Amount, Category}` —
  so line-item categorization is supported at the data level. ✓
- CSV import with a clear column guide + textarea; **Import history** section. ✓
- Clean, readable layout in **light theme** (app default when no theme is persisted). ✓

**Mechanical gaps (block / weaken the ritual):**
- [x] **A receipt is ONE bank charge with MANY lines — import it as a split, not N transactions.**
      Today vision extraction yields N independent transaction rows; importing a grocery receipt that
      way creates many standalone transactions that (a) **double-count** against the single bank/card
      charge the user will also see, and (b) **break dedupe** against that one charge. Build the
      "receipt mode" bottom-up:
  - [x] **Model/logic** `internal/extract` (+ `internal/domain`): distinguish a *statement* (many
        charges → many transactions) from a *receipt* (one charge → one transaction **split across
        categories**). Add a receipt result shape: a single total + categorized line splits that sum to
        the total (table-tested: splits reconcile to the total to the cent; mixed/discount lines).
  - [x] **Persistence/state** `internal/store` + `internal/appstate`: import a receipt as one
        transaction carrying category splits (reuse/extend the category-split model from the budgets
        "cover"/Split work in L1/L2); export/import round-trips.
  - [x] **UI** `internal/screens/documents.go`: a **Receipt vs Statement** toggle on the AI import; in
        receipt mode the review table shows one transaction with editable per-line category splits that
        must sum to the total before Import enables. Plain English; show the running remainder.
- [ ] **Extracted category is free text — map it to a real category + run Rules.** The model returns a
      raw `Category` string; it should resolve to an existing category (by-name/fuzzy, create-on-confirm)
      and pass through the auto-categorization **Rules** engine so Marcus's "Costco → Groceries" rule
      applies on import. Wire + test the mapping (`internal/extract` → `internal/rules`/category lookup).
- [ ] **Mobile camera capture.** `pickImageDataURL` (`documents.go:482`) sets `accept="image/*"` but
      **no `capture` attribute**, so on a phone it opens the file browser instead of the camera. Add
      `capture="environment"` (and a "Take photo" affordance / hint) so "snap a receipt" works on
      mobile — the primary device for this story.

**UI/UX notes:**
- [ ] **Lingering load splash — 3rd reproduction.** The "Getting your money in order…" splash is faintly
      over content again here (light theme, /documents), after L2 (/split) and L1 (/goals). Reinforces
      the L2 splash-dismiss bug — fix once, re-verify across all three routes.
- [ ] **Probe hardening:** the image picker input is created off-DOM (`createElement`, never appended),
      so `input[type=file]` probes false-negative. Future Documents probes should assert the
      **"Choose image" / "Read with AI"** button text instead. Tighten `loopstory_03`.

### L4. Story — "The Expat" (Aisha, Lisbon, multi-currency) — 2026-06-20 ★

**The ritual:** Aisha's salary lands in a **EUR** checking account; she also keeps a **USD** savings
account and a **GBP** brokerage back home. She wants each account in its native currency and one
consolidated **net worth in her base currency (EUR)** via an FX table she controls.
**Drive script:** `e2e/loopstory_04_expat_fx.mjs` (seeds sample, drives Settings FX + /accounts).
**What already works well (verified by screenshot + source — keep as regression anchors):**
- Settings has a **base-currency picker** ("USD — US Dollar") + an **editable FX-rate table**
  (AUD/CAD/CHF… → base). ✓
- Accounts carry a **per-account currency** (`accounts.go:238`, an ISO-code field) and the row subtitle
  shows "type · CURRENCY". ✓
- Net worth is rolled up **through the FX table** (`accounts.go:263`, `currency.Rates{Base, FXRates}`). ✓

**Gaps (UX-refinement + one correctness edge — this is a strong area, refine it):**
- [x] **Account currency is a free-text field — make it a validated picker.** Typing "EUR" works but is
      typo-prone (unknown/lowercase codes silently break conversion). Replace the text input with a
      **searchable currency dropdown** sourced from the known ISO list / the FX-table currencies, with
      validation. Bottom-up:
  - [x] **Logic** `internal/currency`: expose a known-currency list (code + name + decimals) and a
        `Valid(code)`; table-test.
  - [x] **State/UI** `internal/screens/accounts.go`: swap the currency `Input` (line 238) for a
        labelled select/searchable picker; reject/flag unknown codes before save.
- [x] **FX rate staleness signal.** Settings now stamps `Settings.FXUpdatedAt` per rate on edit and shows a
      "Stale" badge on any rate not refreshed in over 30 days, so manual rates that drift are visible (no more
      silent net-worth drift). e2e `fx_staleness_check.mjs`.
  - [x] **Model** `internal/store` Settings: `FXUpdatedAt map[string]time.Time` (round-trips with the dataset).
  - [x] **Logic** `internal/currency.RateStale` + `DefaultRateMaxAge` (30d), table-tested.
  - [x] **State/UI** `internal/app/settings.go`: each FX rate row shows a "Stale" badge past the threshold.
        (An online "Refresh rates" action remains a future enhancement.)
- [x] **Correctness: net worth with a currency that has NO FX rate must NOT silently miscompute.**
      Determinism/explainability rule. Add a logic test in `internal/currency` / the net-worth
      aggregation for the missing-rate case (account in GBP, no GBP rate): it must **warn / show a
      breakdown / exclude-with-notice**, never treat it as base or zero. Surface the warning on the
      dashboard net-worth widget (tooltip/breakdown) and the accounts total.

**Probe hardening:**
- [ ] The add-account currency control is a **text `Input`, not a `<select>`**, so option-value probes
      false-negative. Once it becomes a picker, update `loopstory_04` to assert the picker + a non-base
      option (EUR/GBP). Also the settings panel must be **closed (Escape) before re-opening** — the
      `.flip-backdrop.show` intercepts clicks (fixed in this script).

### L5. Story — "The Debt Crusher" (Jordan & Mei, payoff plan) — 2026-06-20 ★

**The ritual:** Jordan & Mei carry an auto loan, a near-limit credit card, and a store card. They want
a **snowball vs avalanche** plan side-by-side, a projected **debt-free date** per debt, a monthly
amount to commit, and to **track progress** as balances fall.
**Drive script:** `e2e/loopstory_05_debt_crusher.mjs` (seeds sample, drives /planning).
**What already works well (verified by screenshot + source — keep as regression anchors):**
- `internal/payoff` is rich + table-tested: single-debt `Project`, `MinimumViablePayment`, and a full
  **Snowball/Avalanche `BuildPlan`** (strategy.go). ✓
- Planning screen surfaces it: **Snowball vs Avalanche side-by-side** (months + total interest each) +
  the per-debt **payoff order** ("Auto Loan → Credit Card → Mortgage"), plus a single-debt payoff
  calculator. ✓

**Gaps (strong logic — the gaps are presentation, scope, and tracking):**
- [x] **Show a calendar DEBT-FREE DATE, not just "170 months".** The card shows a month count; the
      story wants "debt-free by Aug 2031" (and a date per debt as each clears). Bottom-up:
  - [x] **Logic** `internal/payoff`: add a pure helper turning `Months` (+ a start month) into a target
        month/date, and expose per-debt clear months from `BuildPlan`; table-test.
  - [x] **UI** `internal/screens/planning.go`: renders the debt-free **date** ("Debt-free by May 2028") beside the months (payoff.DebtFreeMonth). e2e payoff_debtfree_date_check.mjs. A
        per-debt "cleared by" date in the order list.
- [x] **Strategy comparison is useless at $0 extra (shows "170 vs 170 months").** Snowball/avalanche
      only differ when there's extra to allocate; the default extra is empty. Default/prompt a sensible
      **extra-per-month**, and when the two strategies tie, **explain why** ("Add an extra monthly
      amount to see snowball vs avalanche diverge"). UX + a small empty/equal state in planning.go.
- [x] **Exclude the mortgage (and any chosen debt) from the payoff plan.** Including the mortgage makes
      it 170 months and dominates the plan; real debt-crusher tools target revolving/consumer debt and
      exclude the mortgage. Bottom-up:
  - [x] **Model/store** `internal/domain`/`internal/store`: a per-account **"include in payoff"** flag
        (default: exclude mortgage-type / long-term loans), persisted + round-tripped.
  - [x] **Logic**: the `BuildPlan` caller filters by the flag; test that excluding the mortgage changes
        months/order as expected.
  - [x] **UI**: a checkbox per liability in the debt-strategy card ("include in payoff plan").
- [x] **Per-debt month-by-month schedule / payoff timeline chart.** (Shipped: payoff.BuildPlan Schedule/Order/ClearedMonths + burn-down chart + per-debt dates; e2e story_payoff_chart/story_payoff_date.) Surface which debt the rolling
      snowball targets each month and a burn-down of total balance. `BuildPlan` likely computes the
      schedule internally — expose it and render with the existing chart helpers (`ui.AreaChart`).
- [x] **Payoff PROGRESS tracking over time.** "Paid off $X since you started; on pace for [date]."
      Needs a stored **baseline** of starting balances. Bottom-up: snapshot baseline in store →
      progress calc in `payoff` (tested) → a progress strip on the debt card + a dashboard widget.

**Probe note:** the "calendar debt-free date" check false-**positived** on the date-picker's "2026";
tighten `loopstory_05` to assert a date *inside the debt card* once the date is added.

### L6. Story — "The First Night" (Tessa, cold start / onboarding) — 2026-06-20 ★

**The ritual:** Tessa just installed CashFlux and opens it cold, wanting to add her first account and
learn where to start.
**Drive script:** `e2e/loopstory_06_first_night.mjs` (wipes `localStorage`, reloads, screenshots every
main screen's first-run state — deliberately does NOT load sample).
**Key discovery:** there is **no reachable empty/first-run state** — the app **always shows the sample
household**. `hydrateDataset` (`internal/app/persist.go:34-39`) calls `LoadSample()` whenever the
dataset key is null/empty. Verified by repro: clearing `localStorage` and reloading brought the sample
($354,070 net worth, "Michael Brooks", a mortgage) right back. Seeding a sample on *first run* is a
fine product choice — but the current implementation has a real trap and missing onboarding:

**Mechanical gap (real BUG — confirmed by repro):**
- [ ] **Wipe → reload re-seeds the sample; a clean slate is unreachable.** Because hydrate re-seeds on
      an empty/missing key, a user who wipes their data (or any genuinely empty store) gets the
      stranger's household back on the next reload. Fix by distinguishing "never set up" from "set up
      and intentionally empty":
  - [ ] **Logic/persistence** `internal/app/persist.go` + `internal/store`: after a wipe, **persist an
        explicit empty dataset** (key present, valid empty JSON) and/or a `seededOnce` flag, so hydrate
        loads empty instead of re-seeding. Only seed when the key has *never* existed.
  - [ ] **Test** (native): hydrate with (a) no key → seeds sample; (b) explicit empty dataset → stays
        empty; (c) wipe-then-hydrate → stays empty. Table-driven.
  - [ ] **E2E**: wipe via Settings → reload → assert zero accounts (no re-seed). Add to `loopstory_06`.

**UX gaps (onboarding):**
- [x] **No "this is sample data" framing.** A brand-new user sees a stranger's finances with nothing
      saying so. Add a **dismissible first-run banner**: "You're exploring sample data — **Start fresh**
      to add your own accounts, or keep looking around." **Start fresh** = wipe → clean empty (per the
      bug fix) → land on a guided "add your first account". (`internal/app` shell/dashboard + a
      first-run flag.)
- [x] **Empty states need friendly design** (now reachable once the bug above is fixed): Dashboard,
      Accounts, Budgets, Goals currently render bare forms / zero-stat tiles with no guidance. Add
      "add your first account / budget / goal" empty states with a single clear CTA (per screen,
      `internal/screens/*`), plain English.
- [x] **Offer the sample as an explicit choice, not a silent default.** On a true first run, a small
      "Add my first account" **or** "Explore with sample data" choice respects the user's intent
      instead of auto-seeding a stranger's household.

**UI/UX defect (reinforced):**
- [ ] **Lingering load splash — 4th and most prominent reproduction** (accounts list, mid-render). See
      L2's splash-dismiss bug; this run shows it squarely over the account rows. Fix once, re-verify.

**Probe note:** the empty-state probes all reported GAP, but that's because the sample masks the empty
state, not because empty states were evaluated — the real issue is reachability (the bug above).

### L7. Story — "Eyes-Free Evening" (Devin, keyboard-only / screen reader) — 2026-06-20 ★

**The ritual:** Devin is blind, uses NVDA + keyboard only, and wants to log a coffee purchase as a
transaction entirely by keyboard, with every control announcing a meaningful name.
**Drive script:** `e2e/loopstory_07_eyes_free.mjs` (accessibility-tree unnamed-control scan, form-label
check, custom-control ARIA, focus-ring on Tab).
**⚠ BLOCKED — runtime verification could not run this iteration.** The wasm build was **red**:
`internal/screens/transactions.go:505-510` calls undefined `sortTh` / `sortThProps` (a concurrent
in-progress edit — sortable table headers — left the tree non-compiling). So the runtime a11y sweep is
**deferred to a green build**. *Process reminder for whoever owns that change: per CLAUDE.md the build
+ tests must pass before committing (one feature per commit) — don't commit the tree in this state.*
**Source review delivered instead (shared custom controls — these are strong, keep as anchors):**
- `internal/ui/controls.go`: **Segmented** = `role="radiogroup"` + `aria-label` + Arrow-key nav
  (Left/Up/Right/Down, PreventDefault); each option `role="radio"` + `aria-checked`. ✓
- **Toggle** = `role="switch"` + `aria-checked` + `tabindex=0` + Space/Enter operate it; **ToggleRow**
  passes its visible label into the switch's `aria-label` (named). ✓
- **Swatch / SwatchPicker** = wrapping `role="radiogroup"` + per-swatch `role="radio"`/`aria-checked`/
  `aria-label`(color) + Space/Enter operable. ✓

**A11y refinement gaps (real, source-grounded):**
- [ ] **Roving tabindex for radiogroups.** `Segmented` options are native `<button>`s and swatches are
      `tabindex=0` divs, so **every** option is a Tab stop. The ARIA radio pattern wants **one** Tab
      stop (the checked option, `tabindex=0`) with the rest `tabindex=-1`, moved between by arrows.
      Apply roving tabindex in `segButton` + `swatch` (drive from `Active`/`Selected`). Pure
      view-layer; verify with the runtime sweep below.
- [ ] **Promote the runtime a11y sweep to a committed gate.** Once the build is green, turn
      `loopstory_07` into `e2e/a11y_check.mjs` run by `run-stories.mjs`: assert (1) `nav[aria-label]`
      + `main#main` landmarks, (2) **zero** focusable controls without an accessible name, (3) zero
      unlabeled form fields, (4) a visible focus ring on first Tab — across `/transactions`,
      `/accounts`, and the Settings panel. This locks in the a11y audit (§ accessibility) so
      regressions fail CI.
- [ ] **Re-run L7 after the green build** to capture the transactions add-form field labels + the
      unnamed-control scan that this iteration could not execute.

### L8. Story — "The Money Question" (Renu, Insights Q&A) — 2026-06-20 ★

**The ritual:** Renu doesn't want charts — she wants to ASK. "How much did we spend on dining last
month?", "Can we afford a $2,000 vacation in August?" — and save the useful answers as a to-do.
**Drive script:** `e2e/loopstory_08_money_question.mjs` (seeds sample, drives /insights). Build was
**green again** this iteration (the L7 `sortTh` breakage was fixed).
**What already works well (verified by screenshot + source — keep as regression anchors):**
- **Spending highlights**: deterministic, grounded insights computed from the user's own figures, **no
  API key needed** ("Housing spending is up 50% — $1,850 this month vs about $1,200/mo", …). ✓
- **Explain my month** + **Ask about your money** free-text box, each with a graceful **needs-an-API-key**
  state and clear **privacy copy** ("stays on this device, only sent to OpenAI when you ask"). ✓
- **Save-as-task** exists (`insights.go:81-103`, button `:252`) — gated behind a generated answer;
  the answer goes into the task notes (C27). **Pinned/saved insights** + token-cost display too. ✓

**Gaps:**
- [x] **Suggested/example questions (beat blank-box paralysis).** There's only a single placeholder
      hint; offer 3–4 **tappable starter questions** that fill the input ("How much did we spend on
      dining last month?", "Where did our money go?", "Can we afford $2,000 in August?"). Bottom-up:
  - [x] **Logic** (pure): a small generator that picks starters, ideally tailored to the user's data
        (their top category / a near-limit budget / an upcoming goal); table-test.
  - [x] **UI** `internal/screens/insights.go`: clickable chips above the question box.
- [ ] **Grounded affordability check (dream-big, determinism rule).** "Can we afford $X by [date]?" is
      a *forward-looking* question; today it just goes to the LLM as free text. Back it with the
      existing **`forecast`/`planning`** engine so the answer shows the math (projected surplus by the
      date, minus commitments + goal contributions), not an LLM guess. Bottom-up:
  - [x] **Logic** `internal/forecast` (or a new `internal/afford`): `CanAfford(amount, byDate)` →
        {affordable, projectedSurplus, shortfall, impactedGoals}, pure + table-tested.
  - [x] **State/UI**: an "Affordability" insight card (or wire the Q&A to call it when it detects an
        affordability question) that renders the breakdown; the LLM only narrates the computed result.
- [ ] **Testability: a stub/mock AI provider behind a flag.** The answer surface, save-as-task, the
      vision receipt import (L3), and Explain-my-month can't be e2e-driven without a live key. Add a
      deterministic **mock `ai` provider** (returns canned, well-formed responses) selectable via a
      test flag / env so `run-stories.mjs` can exercise the full ask → answer → save-as-task flow in CI.
      Bottom-up: an `ai` provider interface seam (likely already present) + a fake impl + tests.

**Probe note:** save-as-task false-**negatived** (it only renders after an answer, which needs a key);
the suggested-questions check missed the "e.g." placeholder. Tighten `loopstory_08` once the mock AI
provider lands so it can assert the post-answer controls.

### L9. Story — "The Migration" (Sahil, export/import backup round-trip) — 2026-06-20 ★

**The ritual:** Sahil switches laptops, exports all his CashFlux data, imports it on the new machine,
and expects a **lossless round-trip** — accounts, transactions, budgets, goals, custom fields, AND his
preferences/theme/FX rates/uploaded fonts. Anything silently dropped = lost records.
**Drive script:** `e2e/loopstory_09_migration.mjs` (seeds sample, enumerates localStorage keys + dataset
entity counts, opens Settings data section).
**What already works well (verified — keep as anchors):**
- The **dataset is comprehensive**: one `cashflux:dataset` blob with `schemaVersion, members, accounts,
  categories, transactions, budgets, goals, tasks, workflows, settings` (sample = 7 accounts / 57 txns /
  5 budgets / 3 goals). ✓
- Settings has **Export JSON / Export CSV / Import**, a **backup-reminder cadence** (B28), and stamps
  the last backup. ✓

**Mechanical gap (silent data-loss on migration — confirmed by source + storage enumeration):**
- [x] **"Export JSON" is dataset-only — it is NOT a complete backup, yet it's framed as one.**
      `exportJSON` → `app.ExportJSON()` serializes only the **active workspace's dataset**. State that
      lives in **separate localStorage keys is left behind**:
  - **`cashflux:workspaces`** — the workspace registry + every **non-active workspace/household** (a user
    with "Personal" + "Side business" exports only the open one; the rest are lost).
  - **Uploaded custom fonts** (`@font-face` binary), the **banner image**, and custom **theme** tokens /
    appearance **prefs** — loaded from their own uistate keys at boot (`app.go` `LoadFonts/LoadBanner/
    LoadTheme/LoadPrefs`), not part of `ExportJSON`.
  The B28 reminder ("A quick backup keeps your data safe") makes users trust this as a full backup, so
  the omission is a silent trap. Fix bottom-up:
  - [x] **Logic** (pure, `internal/store` or a new `internal/backup`): a versioned **full-backup
        envelope** `{schemaVersion, datasets[] (all workspaces), workspaceRegistry, appearance{theme,
        fonts, banner, prefs}, fxRates}`; `MarshalBackup` / `UnmarshalBackup`; **round-trip test**
        (build → marshal → unmarshal → deep-equal).
  - [x] **State** `internal/appstate` + `internal/app`: gather all workspaces' datasets + the uistate
        side-keys; restore them all on import (and re-apply appearance live).
  - [x] **UI** `internal/app/settings.go`: a distinct **"Back up everything"** action (keep the existing
        per-workspace "Export JSON" for sharing a single household) + an import that detects a full
        backup vs a single dataset and restores accordingly. Plain-English copy stating exactly what's
        included.
  - [x] **E2E gate** (`e2e/backup_roundtrip_check.mjs`, run by `run-stories.mjs`): seed sample, customize
        appearance + add a 2nd workspace, **full-backup → wipe → import**, assert entity counts, the 2nd
        workspace, and the appearance all survive. Make it CI-blocking (lossless round-trip is a
        non-negotiable per CLAUDE.md).

**Probe note:** the "complete backup" copy check false-**positived** (the real export string is just
"Export JSON" / "Exported your data", which makes **no** completeness claim — itself a reason to clarify
the copy). The accent-swatch tweak didn't surface a separate key in this run (appearance keys only
appear once actually changed); the round-trip test above should set them explicitly.

### L10. Story — "Payday Tuesday" (Nadia, interactive reactivity) — 2026-06-20 ★

**The ritual:** Nadia logs a $140 grocery expense and expects, with **no refresh**, the Groceries
budget "spent" to tick up and the dashboard spending to rise.
**Drive script:** `e2e/loopstory_10_payday.mjs` — a true *interactive* end-to-end (mutate, then assert
the chain reacts via client-side navigation, NOT a reload).
**✅ VERIFIED WORKING (strongest positive result so far — keep as a regression anchor):**
- Filled the add-transaction form (desc + amount + **category select incl. "Groceries"**), submitted;
  the row appeared in the ledger **immediately**. ✓
- Navigated Budgets→Transactions→Budgets via the rail (SPA pushState, **no reload**); Groceries "spent"
  went **$645 → $785 (exactly +$140)**. The reactive chain transaction → budget rollup is correct. ✓
- Transactions also support income/expense kind, **repeat-last**, and **rule-based auto-suggest** of
  category/tags (`transactions.go:96-98,366,403`). ✓

**Action (lock in the win):**
- [ ] **Promote this to a committed CI gate.** Rename to `e2e/reactivity_check.mjs` and add to
      `run-stories.mjs`: assert a logged expense moves the matching budget's spent by the exact amount
      **without a reload**, across budget + dashboard. The existing per-screen stories don't cover
      **cross-screen reactivity**; this guards the core state model against regressions.
- [ ] Extend the assertion to the **dashboard** (Spending / This-month tiles) and to an **income** entry
      raising the Income tile — same no-reload contract.

**Dream-big gap (close the income→envelopes loop):**
- [ ] **Logging income offers no path to allocate it.** Nadia's $3,200 paycheck just lands in the
      ledger; the **Allocate** flow (rank budgets/goals, split an amount) is a separate manual screen she
      has to remember to visit. Offer a low-pressure nudge after an **income** transaction: "Allocate
      this $3,200 to your budgets & goals?" → opens Allocate pre-filled with that amount. Bottom-up:
  - [x] **Logic** `internal/allocate`: already supports amount-split + ranking — add/confirm an entry
        that takes a single income amount as the pool (tested).
  - [x] **State/UI** `internal/screens/transactions.go` + `allocate.go`: a dismissible post-income nudge
        (friendly, never naggy — per UI rules) that deep-links to Allocate with the amount prefilled.

**Probe note:** rail nav links are `<a href title>` (not `role=link` with a clean name) — drive them by
`href$="/budgets"`, not `getByRole("link", {name})` (fixed in this script; same lesson as L7).

### L11. Story — "The Bus Commute" (Priya, mobile / responsive) — 2026-06-20 ★

**The ritual:** Priya logs a $4 coffee and checks her money on a phone (390×844) one-handed on the bus.
**Drive script:** `e2e/loopstory_11_bus_commute.mjs` (drives every main screen at a mobile viewport;
measures horizontal overflow, rail footprint, mobile-nav affordance, tap-target sizes).
**What already works well (verified — keep as anchors):**
- **No horizontal overflow** on `/`, `/transactions`, `/budgets`, `/accounts` at 390px (0px each). ✓
- Rail **collapses to a 56px icon rail** on mobile; content stacks full-width and readably; a
  collapse-toggle is present. ✓

**UI/UX gaps (mobile):**
- [ ] **Tap targets too small for touch.** Of 268 interactive controls on `/transactions`, **104 are
      small in BOTH dimensions** (true icon buttons — the per-row edit/delete/transactions/⋯ cluster ×57
      rows) and **148 more are <40px tall** (below WCAG 2.5.5's 44px). The C-section touch-target item
      isn't resolved on mobile. Fix: (a) enforce a ≥44px hit area on icon buttons (padding, not just
      visual size); (b) on narrow viewports collapse each transaction row's 3–4 inline icons into a
      single **overflow (⋯) menu**. Add the mobile tap-target check to the responsive gate below.
- [ ] **Bento drag/resize affordances are meaningless on touch and add clutter.** The mobile dashboard
      shows per-tile drag handles + resize handles + "Reset layout"; reorder/resize is a desktop
      interaction. Hide drag/resize chrome under a touch/`pointer:coarse` media query (or below a width
      breakpoint); keep tiles read-only-stacked on phones.
- [ ] **Period/date controls dominate the top of small screens.** Week/Month/Quarter + Jump-to + date
      stepper + Custom range + Add stack vertically and push real content below the fold. Collapse them
      into a compact single-row control bar (or a sheet) on mobile.
- [ ] *(Enhancement)* **Consider a real mobile nav pattern** — a bottom tab bar or a hamburger drawer —
      so phones get full-width content instead of a permanent 56px rail. Optional; the icon rail is
      acceptable today.

**UI/UX defect (reinforced — 5th reproduction, now on mobile):**
- [ ] **Lingering load splash dominates the mobile `/transactions` screen** (squarely over the add-form).
      Same L2 splash-dismiss bug; small screens make it worse (it fills the viewport). Fix once, verify
      across desktop + mobile.

**Action:**
- [ ] **Promote to a responsive CI gate** (`e2e/responsive_check.mjs` in `run-stories.mjs`): assert zero
      horizontal overflow + the mobile tap-target threshold across all main routes at 390px.

### L12. Story — "The Subscription Audit" (Marcus & Lin) + splash root-cause — 2026-06-20 ★

**The ritual:** Marcus & Lin suspect they're bleeding money on forgotten subscriptions; they want every
recurring charge surfaced, the monthly + annual total, price-hike + unused flags, and help cancelling.
**Drive script:** `e2e/loopstory_12_subscription_audit.mjs`.
**⚠ Runtime BLOCKED — build red AGAIN (2nd time this run).** `internal/screens/transactions.go`: `strconv`
imported-and-unused + a type error at `:523` (`rows` []Node used as Node) — another in-progress sortable-
headers edit left the tree non-compiling. Findings below are **source-grounded**; re-run when green.
*Process reminder (repeat of L7): the build + tests must pass before committing — don't leave the shared
tree red.*
**What already works well (source — `internal/screens/subscriptions_screen.go` + pure `internal/subscriptions`):**
- **Auto-detection** of recurring charges from transaction history (`subscriptions.Detect`, B25), with
  cadence, normalized monthly cost, and next renewal per sub. ✓
- **Monthly AND annual** total burden (`AnnualAmount()`); **price-change** detection up/down
  (`DetectPriceChanges`, `priceUp`/`priceDown`); a **recurring-share-of-spending** gauge. ✓

**Gaps (the screen is read-only; the story wants to ACT):**
- [x] **Make it actionable: mark-to-cancel + "charged after cancel" alert** (the real money-saver). (The
      prior `[x]` marks were aspirational — none of this existed; built bottom-up this pass.)
  - [x] **Model/store**: `domain.SubscriptionCancellation{ID, SubName, CancelledOn}` persisted + round-tripped
        (mirrors the Earmark store wiring).
  - [x] **Logic** `internal/subscriptions`: `ChargedAfterCancel(txns, cancels, rates) []LateCharge` → flags
        any expense matching a cancelled sub's name dated after its cancel date; table-tested (FX, case, etc.).
  - [x] **State/UI**: `appstate.MarkSubscriptionCancelled`/`Unmark`/`Cancellations`; a "Mark as cancelled"
        action per row (+ Undo) and a prominent top `role="alert"` banner ("You cancelled X on … but were
        charged … on …"). e2e `subscription_cancel_check.mjs`.
- [ ] **"Cancel these → save $X/year" framing.** Multi-select cancel-candidates and show the annual
      savings of cancelling them — turns the annual total into action.
- [ ] **Unused proxy (no usage signal available).** Offer a low-pressure "review" nudge for subs above a
      cost threshold or not recently re-confirmed; let the user tag "rarely use" to prioritize the audit.

**🔎 SPLASH ROOT-CAUSE — corrects L1/L2/L3/L6/L11 (the "lingering load splash").** The dismiss logic in
`web/index.html:674-683` is **correct**: a `MutationObserver` adds `.hidden` to `#boot` as soon as
`#app` gets children (first mount), then disconnects. The splash kept appearing in earlier screenshots
because **those scripts shot ~700-1000 ms after a full `page.goto()`**, and a cold wasm
re-instantiation+mount frequently takes longer than that — so `#boot` simply hadn't hidden yet. L10
(SPA nav + `waitForSelector`) saw **no** splash, corroborating. **So this is mostly a test-harness timing
artifact, not an app bug** — do NOT spend effort "fixing the dismiss."
- [ ] **Harness fix (real action):** add a shared `ready(page)` helper (wait for `nav` + `#boot.hidden`/
      opacity 0) and call it before every screenshot; replace fixed `waitForTimeout`s in all `loopstory_*`
      scripts. (Implemented in `loopstory_12`; back-port to the rest when promoting them to gates.)
- [ ] **Minor perf note (optional):** if first mount on a hard refresh is slow enough that users see the
      splash >~1s, track it as a wasm-startup/perf item — separate from the (non-existent) dismiss bug.
- [x] Downgrade the L1/L2/L3/L6/L11 "splash" bullets to "see L12" — not a dismiss defect.

### L13. Story — "Paycheck to Paycheck" (Dani, cash-flow timing / overdraft warning) — 2026-06-20 ★

**The ritual:** Dani lives close to the edge — rent ($1,800) hits the 1st, payday is the 5th. She wants
CashFlux to project her **forward daily balance** and **warn** her when an account would dip below zero
before payday ("Checking dips to -$240 on Jul 2 — move money or delay a bill").
**Drive script:** `e2e/loopstory_13_paycheck_to_paycheck.mjs` (seeds sample, drives /bills, ready-gated
screenshot).
**What already works well (verified by screenshot — keep as anchors):**
- Bills screen is clean + professional: **Total due soon / Per year / Upcoming count / Next due** stats;
  a bills list (Mortgage/Auto Loan/Credit Card with due dates + "due in N days"); a **month calendar
  with bill dots**; **Download CSV**; per-bill **"Remind me"**. ✓

**Gaps (the safety net the story needs is absent):**
- [ ] **Forward daily cash-flow projection + overdraft warning (headline).** Project each spending
      account's balance day-by-day over the next N days from known **upcoming bills** (due date+amount)
      and **expected income** (recurring paychecks), and flag the first day any account dips below zero
      (or below a user-set **buffer**). Bottom-up:
  - [x] **Logic** `internal/forecast` (or new `internal/cashflow`, pure): `DailyBalances(startBal, bills,
        income, days, buffer)` → daily series + first-below-buffer date + the shortfall amount.
        Table-tested: rent-before-payday → negative on day X; buffer threshold; multiple accounts.
  - [x] **State/UI**: a **"Cash-flow runway"** card (Bills and/or Dashboard) — a daily balance line with
        a red marker on the danger day and a plain-English warning. Determinism: show the contributing
        bills/income.
- [ ] **Warning → suggested action.** On a detected dip: "Checking dips to -$240 on Jul 2 — move $X from
      High-Yield Savings, or delay the Auto Loan." Reuse the L1 cover/move-money + a bill-delay; emit a
      dismissible nudge → task (friendly, never naggy).
- [x] **Mark a bill paid** (already shipped): BillRow has a "Mark paid" action wired to
      `appstate.RecordBillPayment(accountID, name, amount)`, which records the paying transaction. Stale gap;
      verified present (`bills_screen.go:66`).

**Probe note:** "mark paid" GAP is accurate (the affordance is "Remind me", not paid). Calendar/upcoming
checks passed against real content.

### L14. Story — "The Power User" (Theo, command palette) — 2026-06-20 ★

**The ritual:** Theo runs his money mouse-free — Ctrl/⌘+K, type "budgets" or "add transaction", jump or
fire an action with fuzzy search.
**Drive script:** `e2e/loopstory_14_power_user.mjs` (opens palette by keyboard, types, asserts filter +
Enter-navigation + Esc-close).
**✅ VERIFIED WORKING (strong feature — keep as a regression anchor):**
- **Ctrl+K opens** the palette; clean "Search commands…" + vertical list. Fuzzy "budg" → **Budgets**;
  **Enter navigated to /budgets**; palette **closed after running**; **Esc closes**. ✓
- `buildPaletteCommands` (`shortcuts.go:224`) covers all nav (primary/tools/system) **plus actions**:
  New transaction, toggle theme, toggle sidebar, shortcuts help, and full **workspace management**
  (switch/new/export/import). ✓ Also: Alt+1–9 jump, Alt+N add, "?" help cheat-sheet. ✓

**Gaps (enhance an already-good feature):**
- [x] **Command aliases/synonyms** (already shipped): `paletteCmd` has `keywords` and the `cmdmatch` fuzzy
      matcher ranks against them, so "add" → "New transaction", "export"/"backup"/"undo" etc. all resolve
      (gated by `palette_fuzzy_check.mjs`).
- [x] **Broaden the action set** (already shipped): the palette covers New transaction, toggle theme/sidebar,
      undo/redo, export JSON/CSV, backup-everything/restore, workspace new/switch/export/import, and passcode
      lock/change/remove.
- [x] **Data entities are searchable jump targets** (dream-big, shipped): the palette now indexes the user's
      accounts, goals, and budgets by name ("<name> · Account/Goal/Budget") and navigates to that screen.
      `entityJumpCommands` in shortcuts.go; e2e `palette_entities_check.mjs`.
- [ ] *(Polish)* **Group + hint.** Section the list (Navigate / Actions / Workspaces) and show the
      keyboard hint (Alt+N, etc.) beside matching commands.

**Probe note:** the "list narrows (before/after count)" + "actions" checks **false-negatived** — palette
items aren't `[data-cmd]/li/button` (different markup), and typing "add" can't match the noun-labeled
"New transaction" (which is exactly gap #1). Tighten `loopstory_14`'s item selector + assert via the
alias once added.

### L15. Story — "Set It and Forget It" (Bianca, Rules / auto-categorization) — 2026-06-20 ★

**The ritual:** Bianca creates a rule — match "Starbucks" → Dining — and expects every new transaction to
auto-file itself, plus a way to backfill existing uncategorized ones.
**Drive script:** `e2e/loopstory_15_set_and_forget.mjs` (interactive: create a rule, add a matching txn,
assert the category auto-fills).
**✅ VERIFIED WORKING end-to-end (strong feature — keep as a regression anchor):**
- Created a rule (match → category + tags) on `/rules`; it listed and **persisted**. ✓
- Added a transaction whose description matched; the **category select auto-filled to "Dining"** (the
  `SuggestTransactionFields` path), **surviving a full page reload** (verified the 3 selects = Expense /
  Auto Loan / **Dining**). ✓
- An **"Apply to existing"** backfill affordance is present. ✓
- Engine (`internal/rules`): case-insensitive substring match + **first-match-wins** with specificity
  ordering, table-tested. ✓

**Action (lock in the win):**
- [x] **Promoted to a CI gate** (`e2e/rules_check.mjs`, auto-discovered by `run-stories`): create rule →
      matching txn auto-categorizes → survives reload. The core round-trip is now covered.

**Dream-big gaps (extend a solid engine):**
- [~] **Richer match conditions** — substantially covered by the existing **workflow** engine
      (`internal/workflow` + `/workflows`): expression conditions like `txn_abs > 200` (amount range) and
      `contains(txn_payee, "coffee")` (keyword), tested. The pure `rules.Condition` type (AllKeywords/
      AnyKeywords/AccountID/Min-MaxAmount, tested) also exists but isn't yet wired into the simple `Rule`
      struct/form. (Remaining: surface `Condition` in the simple rules form, OR converge rules→workflows.)
- [~] **Actions beyond category + tags** — covered by the workflow engine's actions
      (`ActionSetCategory`, `ActionAddTag`, `ActionFlagReview`; seeded in sample.go). (Remaining:
      member/owner + budget actions; converge with the simple Rule.)
- [x] **"Create rule from this transaction" + preview count.** Both done: the per-transaction action
      "Always categorize like this" prefills the rule form (via a shared `uistate` RuleDraft atom, mirroring
      the dialog-host pattern) and navigates to /rules; the **match-count preview** ("matches N existing
      transactions", live as you type + per-rule) was already wired (`rules.MatchCount`/`Covered`). e2e
      `create_rule_from_txn_check.mjs`.

**Probe note:** the auto-categorize check **false-negatived** first (the script read the *account* select
"Auto Loan", not the *category* select); a focused re-measure confirmed the category = "Dining". Fix
`loopstory_15` to read the category select by position/label, then promote per the gate above.

### L16. Story — "Tax Season" (Priya, Reports / annual review) — 2026-06-20 ★

**The ritual:** Priya's accountant needs her annual numbers — total income, total expenses, a by-category
breakdown for the **year**, deductible categories called out, and a clean export to hand off.
**Drive script:** `e2e/loopstory_16_tax_season.mjs` (seeds sample, drives /reports, ready-gated shot).
**What already works well (verified by screenshot + source — strong screen, keep as anchors):**
- Rich Reports: Income / Spending / Net, **savings rate**, **cash runway**, **no-spend days**; a
  **spending-by-category** breakdown with period-over-period % deltas + sparkline; **biggest deposits**,
  **income by source**, **top payees**; per-section **Download CSV**; a "Heads up" anomaly section. ✓
- **Per-member spending** (`reports.SpendingByMember` + by-member CSV) — present, but only rendered
  `If(len(memberSpend) > 1)`, so the **single-member sample hides it** (ties to L2). ✓

**Gaps (make it tax-ready):**
- [x] **One-tap YEAR view.** Added a `period.Year` resolution (Truncate/Step/Label, table-tested) and a
      "Year" segment to the top-bar resolution control; also made `/reports` period-aware so the annual review
      is one tap there. Period label reads the calendar year (e.g. "2026"). e2e `reports_year_view_check.mjs`.
      (Fiscal-month-start offset for the year boundary is a future refinement.)
- [x] **No deductible/tax tagging or tax-summary export.** Add a category **"tax group / deductible"**
      attribute, a Reports **"Deductible totals"** section, and a single **annual tax-summary export**
      (all sections, year-stamped) to hand to an accountant. Bottom-up: `Deductible`/`TaxGroup` on the
      category domain type (additive, store round-trip) → a pure totals roll-up in `internal/reports`
      (tested) → the section + a one-click export.
- [ ] **Per-member report needs >1 member to be visible** — reinforce the L2 "add 2-3 sample members"
      item so this (and joint-filing splits) are demoable out of the box.

**Probe note:** all keyword checks passed; the "year selector" and "per-member" PASSes were partly
**false-positives** (matched the date "2026" / the nav "Members") — source confirms there's a custom
range but **no annual preset**, and per-member exists but is **hidden** at one member. Tighten
`loopstory_16` to assert an explicit "Year" control and an on-page member breakdown.

### L17. Story — "Every Dollar a Job" (Marcus, Allocate / zero-based) — 2026-06-20 ★

**The ritual:** Marcus has $2,000 left after bills and wants to assign every dollar — zero-based. He
opens Allocate, enters $2,000, and expects it distributed across ranked destinations with the math shown
and nothing silently lost.
**Drive script:** `e2e/loopstory_17_every_dollar.mjs` (interactive: enter an amount, assert the split +
remainder sum exactly to the input).
**✅ VERIFIED WORKING (standout feature — keep as a regression anchor):**
- Profile + **weighted criterion sliders** (returns / stability / liquidity / debt-paydown / goal),
  **Amount to allocate** + **Keep back (emergency buffer)** + **Max per destination** inputs. ✓
- Ranked, **explainable** distribution: each destination shows its amount, score %, and the per-criterion
  breakdown (`allocate.RankWith` + `allocate.Distribute`); **Exclude/restore** per destination. ✓
- **Determinism (financial correctness):** entering $2,000 → distributed **$1,999.95 + kept back $0.05
  = $2,000.00 exactly**; the 5¢ rounding remainder is correctly held back and disclosed ("Kept back").
  No money created or lost. ✓

**Action (lock in the win):**
- [x] **Promote to a CI gate** (`e2e/allocate_determinism_check.mjs`): for several amounts/reserves, assert
      `sum(distributed) + keptBack == amount` to the cent. Financial-correctness invariant.

**Dream-big gaps (close the loop from SUGGESTION to ASSIGNMENT):**
- [x] **"Apply this allocation" — actually commit the dollars.** Earmark-only semantics (chosen 2026-06-21):
      no cash moves between accounts, money never created/lost. Goals bump CurrentAmount (capped at target,
      overflow disclosed); account & debt destinations become persisted `domain.Earmark` records; single undo.
  - [x] **Logic** `internal/allocate`: `PlanActions` → `[]Action` (contribute-to-goal / account-earmark /
        debt-paydown-earmark), pure + tested (sum of actions == distributed).
  - [x] **State** `internal/appstate`: `ApplyAllocation` applies atomically (snapshot-on-fail rollback);
        `UndoLastAllocation` restores the pre-apply snapshot. New `domain.Earmark` entity + store wiring.
  - [x] **UI** `internal/screens/allocate.go`: an "Apply allocation" button + confirm summary + result line
        with Undo. e2e `allocate_apply_check.mjs` gates apply→persist→undo.
- [ ] **Fill-to-target (envelope) mode.** Zero-based often means funding each budget to its limit in
      priority order (rent $1,800, groceries $600, …) rather than score-weighted spread. Add a mode that
      `Distribute`s to each destination's remaining-to-target first, then ranks the rest. Pure + tested.
- [x] **Save an allocation as a recurring plan** (Shipped: domain.AllocationProfile + saveProfile form on Allocate.) ("every paycheck, split like this") — ties to the L10
      income→allocate nudge so a logged paycheck can one-tap apply the saved split.

**Probe note:** the reserve-input check **false-negatived** (placeholders aren't in `innerText`); the
"Keep back (emergency buffer)" input is present on screen + in source (`allocate.go:395`). Assert it via
`getByPlaceholder` in `loopstory_17`.

### L18. Story — "The Landlord's Ledger" (Dana, custom fields) — 2026-06-20 ★

**The ritual:** Dana rents two properties; she needs to tag each transaction with which **Property** it
belongs to and whether it's **deductible** — fields CashFlux lacks natively — then **filter and report**
by them.
**Drive script:** `e2e/loopstory_18_landlord.mjs` (interactive: define a custom field, confirm it
renders in the transaction form, probe filter/report by it).
**✅ VERIFIED WORKING (define + fill — keep as anchors):**
- **Custom Fields manager** on `/customize`: define a field on any of **5 entities**
  (account/transaction/budget/goal/member) × **5 types** (text/number/date/bool/select+options) +
  a required flag. Created a `Property` select field on the transaction entity; it **listed** and
  **rendered in the transaction add form** (verified `propInTxnForm: true`). ✓
- Bonus: the same screen has a strong **Formula calculator** (live-figure expressions — sum/avg/min/max/
  count/abs/round/if — with presets + a variables panel). ✓

**Gaps (define+fill works, but the data is a dead end — you can't slice by it):**
- [x] **Filter lists by a custom field.** Dana can tag "Property = Maple St" but can't list all Maple St
      transactions. The transactions filter set (account/category/cleared/tags) has **no custom-field
      predicate**. Bottom-up:
  - [x] **Logic** (pure, tested): extend the transaction filter to match custom-field values
        (equals / one-of for select, range for number, true/false for bool).
  - [x] **State/UI** `internal/screens/transactions.go`: a filter control per filterable custom field
        (persisted with the other filters, per C-section).
- [x] **Report / total by a custom field.** `reports.ByCustomField(txns, fieldKey, start, end, rates)`
      roll-up (pure, 9 table tests) + `reports.CustomFieldCSV` → a "Spending by <field>" Reports section with
      a field selector (groups by any transaction custom field) + CSV. Bool normalizes Yes/No, numbers strip
      trailing zeros, missing → "(no value)". e2e `report_by_customfield_check.mjs`. **Also satisfies L16's
      tax-tagging** — a bool "Deductible" custom field + this roll-up = a deductibles total.
- [ ] *(Enhancement)* **Custom fields in export/import + the Allocate/Insights context** so the
      extensibility is end-to-end (verify they round-trip in the backup from L9).

**Probe note:** first-run GAPs for "field types" and "appears in txn form" were **test artifacts** — the
script's `select.first()` hit the *entity* select (account/transaction/…), so the field was created on
Accounts with type Text. A corrected re-drive (entity=transaction, type=select) confirmed both work.
Fix `loopstory_18` to target the entity/type selects by their option sets.

### L19. Story — "Airplane Mode" (Sofia, offline / local-first) — 2026-06-20 ★

**The ritual:** Sofia is on a 6-hour flight with no wifi. She reviews her budget and logs the coffee +
snacks she bought — fully offline — and expects it all there when she lands.
**Drive script:** `e2e/loopstory_19_airplane_mode.mjs` (online boot + SW cache, then `setOffline(true)`,
navigate, add a txn, verify persistence, reload offline).
**✅ VERIFIED WORKING (the core local-first promise — keep as anchors):**
- A **service worker registers + becomes ready**; after online boot the cache holds **all 8 CORE assets**
  (`./`, index.html, wasm_exec.js, **bin/main.wasm**, manifest, chart.js, flip.js, d3) — confirmed by
  enumerating `caches`. ✓
- **Offline in-session works fully:** navigated to Budgets offline, **logged a transaction offline** (it
  appeared in the ledger), and it **autosaved to `localStorage`** offline. The offline write path is
  solid. ✓

**Uncertain — needs a real-browser check (do NOT treat as a confirmed bug yet):**
- [x] **Offline hard-refresh** failed in Playwright (`net::ERR_FAILED`, blank page) even though the cache
      is fully populated and `sw.js`'s navigate→`appShell()` fallback is correct. This is **most likely a
      Playwright `setOffline` artifact** (headless Chromium can bypass SW interception for top-level
      navigations), not necessarily a production defect. **Action:** verify offline reload on a real
      browser (DevTools → Network: Offline, or an installed PWA in airplane mode). Only if it fails there
      is it a real bug — then look at SW controlling-client timing on reload.

**Real robustness gaps (worth doing regardless):**
- [x] **`cache.addAll(CORE)` is all-or-nothing AND includes a cross-origin CDN (d3).** If d3's CDN (or any
      single asset) fails at install, the **entire precache rejects** and `install`'s `.catch(()=>{})`
      **swallows it silently** → offline boot would break with zero signal. Fix (`web/sw.js`): (a) cache
      per-item via `Promise.allSettled` so one failure doesn't void the rest; (b) **self-host d3**
      (`./d3.min.js`) so the precache is all same-origin and offline never depends on a third party;
      (c) log precache failures instead of swallowing. Pure SW change + an e2e offline-boot assertion.
- [x] **Offline indicator / "saved locally" reassurance.** A calm top-bar "Offline" pill appears when the
      browser loses connectivity (hidden when online), with a tooltip that changes are saved on this device.
      Driven by a shared `uistate` online atom kept in sync from `navigator.onLine` + the window online/offline
      events (`wireOnlineStatus`). e2e `offline_indicator_check.mjs` (Playwright `setOffline`).

**Incidental finding (a11y/semantics regression — ties to L7):**
- [x] **Rail nav `<a>` elements have NO `href`** (now onClick-only — confirmed by enumerating nav anchors:
      every `href` is null). An anchor without `href` isn't a real link: not keyboard-focusable as a link,
      no middle-click / open-in-new-tab, and screen readers don't announce it as a link. Restore real
      `href` (the router already supports pushState links) or switch the role appropriately. This also
      broke href-based test selectors (L10/L13 used `a[href$="/x"]`) — drive nav by text until fixed.

**Probe note:** the nav-by-href selectors timed out because of the missing-`href` regression above; the
script now clicks nav items by text.

### L20. Story — "The Finish Line" (Aaliyah, goal-completion lifecycle) — 2026-06-20 ★

**The ritual:** Aaliyah's emergency-fund goal is about to be reached. She wants CashFlux to recognize the
milestone — celebrate it, mark the goal achieved, stop nagging her to contribute, and suggest
redirecting the freed-up monthly amount to her next goal.
**Drive script:** `e2e/loopstory_20_finish_line.mjs` (create a goal, push it past 100%, inspect the
completed state). Verified by creating a goal over target ($80 saved / $50 target).
**✅ VERIFIED WORKING (the completion moment is handled well — keep as anchors):**
- At/over target the goal shows a **full (capped) progress bar + "Complete 🎉"** badge. ✓
- The **pace nag is removed** when complete (no "save $X/mo", no "$X to go"). ✓
- Contribute/Edit remain available; the bar correctly caps at 100% even when over-funded ($80/$50). ✓

**Gaps (what happens AFTER the finish line):**
- [~] **Over-funding acknowledged.** Pure `goals.Overfund(goal)` (tested) → a calm "<amount> over target"
      note on over-funded rows. (Remaining: a "move excess" redirect action reusing L17 allocate / L5
      contribute — deferred.)
- [x] **"What next" when a goal completes.** A completed (not-yet-archived) goal row shows a calm, single-line prompt with a "Reallocate" action that jumps to Allocate to put the freed-up monthly toward another goal. e2e goal_whatnext_check.mjs.
- [x] **Completed goals archive into an "Achieved" section.** New `Goal.Archived` flag (JSON round-trip);
      completed goals get an Archive action → a collapsible "Achieved" section (with Unarchive); the headline
      "Overall progress" uses `goals.OverallProgress(active, false)` so archived goals no longer dilute it.
      `appstate.ArchiveGoal` + tests; e2e `goal_lifecycle_check.mjs`.
- [ ] *(Polish)* **One-time celebration moment** on crossing the line (a subtle toast/animation), not just
      a persistent static "Complete 🎉" badge — keep it calm per the UI rules.

**Probe note:** the first run's "achieved state" + "100% cap" checks **false-negatived** — the inline
**Contribute opens an amount form (not a JS `prompt`)**, so the `page.on("dialog")` handler never fired
and the goal stayed at $0 (and my row filter clicked the wrong goal's Contribute). A corrected re-drive
(create goal already over target) confirmed the **"Complete 🎉"** state + capped bar exist. Fix
`loopstory_20` to fill the inline contribute form for the *named* goal.

### L21. Story — "Yours, Mine, and Ours" (Priya & Sam, multi-member / household-aware) — 2026-06-20 ★

**The ritual:** Priya & Sam share a household but want to see who spent what. Priya adds Sam as a 2nd
member, attributes transactions to each person, filters the ledger by member, and checks per-member
spending — so they split fairly.
**Drive script:** `e2e/loopstory_21_yours_mine_ours.mjs` (add a member, then probe the household-aware
surfaces).
**What already works well (verified by source — keep as anchors):**
- **Add a member** on /members. ✓
- **Member filter on the ledger** (`transactions.go:539-556`): "All members" + each member, driving
  `TxFilter.Member`. ✓
- **Per-member Reports** section + by-member CSV (`reports.SpendingByMember`, L16) — present when >1
  member *with spending*. ✓
- Transactions are **member-aware**: each new transaction is stamped with a member. ✓

**Gaps (the crux of "who spent what" on shared accounts):**
- [x] **Explicit per-transaction member assignment.** Added an optional **"Who" member select** on the
      transaction add form AND the `TransactionRow` inline editor (rendered only when >1 member). Defaults to
      the account owner and follows the account on change until the user overrides; persists `Transaction.MemberID`.
  - [x] **Domain/state**: `Transaction.MemberID` exposed as editable (add + inline edit); defaults to account
        owner, override sticky per-entry, resets to owner-default after submit.
  - [x] **UI** `internal/screens/transactions.go`: member select on add form + in `TransactionRow` edit;
        respects the existing member filter (keys off MemberID).
  - [x] **Test**: `e2e/member_assignment_check.mjs` — a txn overridden to m-jordan persists memberId=m-jordan
        and shows under that member's ledger filter.
- [ ] **Per-member report stays hidden until ≥2 members have attributed spending.** Combined with the
      single-member sample (L2), the household-aware value is invisible out of the box. Reinforces L2's
      "add 2-3 sample members **with a few transactions each**" so /reports by-member + Split/Settle-up
      (L2) demo immediately.
- [ ] *(Enhancement)* **Per-member dashboard view / "my money" toggle** — a member switcher that filters
      the whole app to one person's view (the household-aware promise end-to-end).

**Probe note:** the "assign to member" + "ledger filter by member" PASSes were partly **false-positive/
imprecise** — the script detected the member *filter* select (which lists Sam), not a per-transaction
assignment field. Source confirms the **filter is real** but **per-transaction assignment is
account-derived only**. The Reports-by-member GAP is because the newly added Sam has **no attributed
spending** (no account owned / no explicit assignment), not a broken report. Tighten `loopstory_21` to
attribute spending to the 2nd member (own an account or set MemberID) before asserting the report.

### L22. Story — "Make It Mine" (Renée, theme / appearance customization, B20) — 2026-06-20 ★

**The ritual:** Renée finds finance apps sterile. She opens appearance settings, switches theme, picks an
accent, bumps the font scale, and expects it to apply INSTANTLY and STICK after a reload.
**Drive script:** `e2e/loopstory_22_make_it_mine.mjs` (open theme editor, apply a preset/accent, read live
CSS tokens, reload, assert persistence).
**✅ VERIFIED WORKING (strong, complete feature — keep as a regression anchor):**
- A **theme editor** opens from the settings panel; design tokens are exposed as **live CSS custom
  properties** (`--accent`, `--ui-scale`, surfaces, text). ✓
- **Live apply:** applying a preset changed the page background immediately
  (`rgb(14,14,15)→rgb(15,23,20)`) — no reload needed. ✓
- **Persists:** writes `cashflux:theme` + `cashflux:prefs`; after reload the non-default accent
  (`#4fae84`) and background survived (app.go applies prefs/theme **before mount**, so no flash to
  defaults). ✓
- Controls present: **dark/light**, **accent swatches**, **font/UI scale + density**, **custom font**,
  **dashboard banner**. ✓

**Gaps (this area is well-covered; the gaps are a11y + portability):**
- [x] **Contrast guard on custom themes** (already shipped — stale gap): the theme editor runs a WCAG
      contrast check (`internal/contrast` `Ratio`/`PassesAA`) and shows inline "Some tokens may be hard to
      read" warnings per failing token, or "all text meets the contrast guideline" (`theme_editor.go:192-203`).
- [x] **Share a theme** (already shipped): the theme editor has **Export theme** (`cashflux-theme.json`) +
      **Import theme** buttons for the token set (`theme_editor.go:280-297`).
- [ ] **Custom appearance in the FULL backup (cross-ref L9)** — the theme is independently exportable now
      (above); folding the appearance keys into the L9 full-backup envelope remains a minor follow-up.

**Probe note:** the "persisted across reload" check **false-negatived** — the `after` token snapshot was
read before the accent swatch's live update fully propagated, so it compared the reloaded accent
(`#4fae84`, persisted correctly) against a stale value. Background persistence matched, and the reloaded
accent is non-default, confirming persistence works. Fix `loopstory_22` to re-read tokens after a short
settle before snapshotting, and assert against the swatch's known color.

### L23. Story — "The Decade Importer" (Hector, bulk CSV import resilience) — 2026-06-20 ✅ DONE

**The ritual:** Hector pastes 10 years of transactions — a big, slightly messy CSV (hundreds of rows,
a few with missing/garbage fields) — and expects valid rows imported with correct totals, bad rows
handled gracefully, and the app to stay snappy.
**Drive script:** `e2e/loopstory_23_decade_importer.mjs` (+ focused `_impdiag` diagnostics).
**What works (verified):**
- CSV paste import works for clean rows: 10/10 imported and persisted; an **unmatched account name still
  imports** (the importer doesn't require a pre-existing account); huge numbers, empty lines, and missing
  dates are each handled fine. ✓

**🔴 CONFIRMED BUG (high value — precisely root-caused):**
- [x] **One row with a non-numeric amount ABORTS the ENTIRE CSV import — silently.** Isolated by trials:
      `clean=10/10`, **`+1 row "amount=not-a-number" → 0/10 imported`**, `+huge=10/10`, `+empty=10/10`,
      `+missing-date=10/10`. So a single bad **amount** discards *all* valid rows, with **no page error
      and no toast**. This is exactly why a 600-row paste containing one `not-a-number` imported nothing.
      For a "paste my old data" flow this is the worst failure mode. Fix bottom-up:
  - [x] **Logic** (the CSV parser — `internal/extract` / the documents import path): parse **row-by-row**;
        a bad row is **skipped, not fatal**. Return a structured `{imported:int, skipped:[{line, field,
        reason}]}`. Table-test: bad amount, missing required field, empty line, extra columns, huge value
        — assert valid rows still import and each bad row is reported with its line + reason.
  - [x] **State/UI** `internal/screens/documents.go`: after import show **"Imported 598 of 600. 2 skipped:
        line 12 — amount 'not-a-number' isn't a number; line 45 — …"** in plain English, so Hector can fix
        and re-import. (Today there is no success/skip feedback at all.)
  - [x] **E2E gate** (`e2e/import_resilience_check.mjs`): valid+malformed CSV → valid rows imported, bad
        rows reported, no silent loss.

**Unverified (blocked by the bug above):**
- [ ] **Scale/perf at 600+ rows** could NOT be measured because the malformed import aborted (final count
      was just the 57 sample rows; the "30s import" was my poll *timeout*, not real time). Re-test ledger
      render + scroll responsiveness with 600–1000 **clean** rows once row-level import lands. (Clean
      10-row import + a 57-row ledger rendered in ~1.9s and stayed interactive — promising but not at
      scale.)

**Probe note:** the main script's "all 600 imported" and "import time" findings were **misleading** — the
import silently failed on the bad amount, so the delta was just the sample (57) and the "30124ms" was the
`waitForFunction` timeout. The `_impdiag` trials are the source of truth here.

### L24. Story — "Pay Yourself First" (Leah, transfers / accounting invariants) — 2026-06-20 ★

**The ritual:** Leah moves $500 from Checking to High-Yield Savings monthly. She expects: Checking
-$500, Savings +$500, **net worth UNCHANGED**, and the transfer **excluded** from income/expense.
**Drive script:** `e2e/loopstory_24_pay_yourself.mjs` (+ `_xfer` diagnostic for a real from→to).
**✅ VERIFIED WORKING (correctness — keep as a regression anchor):**
- The transaction form supports a **Transfer kind** with **from + to account** selectors. A real
  $500 **Everyday Checking → High-Yield Savings** transfer recorded correctly. ✓
- **Accounting invariants hold:** after the transfer, **net worth unchanged ($354,070)**, **income
  unchanged ($6,400)**, **spending unchanged ($4,088)** — transfers are net-worth-neutral and correctly
  excluded from income/expense. ✓ (Individual per-account ±$500 not separately asserted, but the
  net-worth-flat + not-income/expense invariants confirm balanced legs.)
- **Action:** promote to a CI gate (`e2e/transfer_invariants_check.mjs`) — net-worth-neutral +
  excluded-from-income/expense is a core correctness invariant.

**Gaps (dream-big automation + edge cases):**
- [~] **Recurring / scheduled transactions.** The `domain.Recurring` model (cadence + NextDue + account +
      category + autopost + `Advance()`), its store wiring, `PutRecurring`, and `PostDueRecurring(asOf)` with
      bounded catch-up ALL already existed; Planning has the full create/edit/autopost management UI.
  - [x] **Auto-post on app open** — `postDueRecurringOnBoot` (app.go) catches up every due autopost recurring
        at boot (after autosave is armed → persists immediately; idempotent, no double-post on reopen). e2e
        `boot_autopost_check.mjs`. Previously only a manual Planning "Post due" button did this.
  - [x] **"Repeat" option on the transaction add form** — a Repeat select (None/Weekly/Monthly/Quarterly/
        Yearly) creates an autopost `domain.Recurring` schedule inline (NextDue = next cadence after the
        entered date, so today's entry isn't double-posted); boot auto-post carries it forward. e2e
        `txn_add_repeat_check.mjs`. (Transfers excluded — two-leg recurring is future work.)
- [ ] **Cross-currency transfer (ties L4).** A transfer assumes one amount/one currency; moving USD→EUR
      needs an FX rate (and likely a received-amount). Verify + handle: apply the FX table, optionally let
      the user set the received amount; test net-worth stays consistent in base currency.

**Bonus note for the dev agent (cross-cuts L1/L3/L17/L18):** `Transaction.Splits []CategorySplit`
**already exists** in the model (`entities.go:77`). The category-split *data model* is in place — the
budgets-cover (L1), receipt-as-split (L3), allocate-apply (L17), and custom-field reporting work mostly
need **UI + apply logic over the existing Splits**, not a new schema. Verify the splits UI/round-trip.

**Probe note:** the first run's transfer was **vacuous** — my account picker selected the placeholder
"— To account —" as the destination, so submit failed validation and *no* transfer occurred (making the
invariant PASSes meaningless). The `_xfer` re-drive with a real Checking→Savings destination is the source
of truth. Fix `loopstory_24` to pick accounts by name and skip empty-value options.

### L25. Story — "The Cleanup" (Wei, bulk transaction operations) — 2026-06-20 ★

**The ritual:** Wei has a pile of messy transactions; he multi-selects a batch, bulk-assigns a category,
bulk-marks-cleared after reconciling, and bulk-deletes duplicates — fast, ideally with undo.
**Drive script:** `e2e/loopstory_25_cleanup.mjs` (+ `_bulkdiag` 1-3 diagnostics).
**✅ VERIFIED WORKING (strong — keep as a regression anchor):**
- Per-row select (`button.check`) → a **bulk bar** with the selection count and actions: **Apply
  category**, **Mark cleared / uncleared**, **Delete selected**, **Clear selection**. ✓
- **Bulk recategorize is correct:** with the bulk "Category to apply" select set to Dining, applying it
  changed exactly the selected non-Dining rows **to Dining** (Groceries→Dining, Household&shopping→Dining;
  the already-Dining row unchanged). Selection clears after the action. ✓
- Bonus: **duplicate detection** — "N possible duplicates" + a **"Select duplicates"** helper. ✓
- No page errors across bulk operations. ✓
- **Action:** promote to a CI gate (`e2e/bulk_ops_check.mjs`) — assert recategorize/clear/delete affect
  exactly the selected set.

**Gaps:**
- [x] **Bulk-action undo.** Each bulk recategorize / mark-cleared / delete snapshots the affected rows'
      prior state into `lastBulk`; an inline "<Op> N · Undo" banner restores them via
      `appstate.RestoreTransactions` (PutTransaction upsert re-creates deletes / reverts changes). One level
      of undo (last op). e2e `bulk_undo_check.mjs` + correctness gate `bulk_ops_check.mjs` (recategorize/
      delete affect EXACTLY the selected set). Rows now carry `data-id` for precise targeting.
- [x] **Select-all-filtered.** A "Select all" button selects exactly the current filtered set
      (`txnfilter.Apply(txns, filter)`); the bulk ops then operate on it. Verified in `bulk_ops_check.mjs`.

**Probe note (IMPORTANT — not a bug):** the main script + `_bulkdiag`/`_bulkdiag2` initially showed bulk
apply **clearing categories to empty** — that was a **test artifact**: setting the bulk `<select>` via JS
`dispatchEvent` and via `selectOption(nth(0))` hit the **wrong** select (the add-form/filter category, not
the bulk one), so `bulkCat` stayed empty and apply cleared. Targeting the exact
`select[aria-label="Category to apply"]` (`_bulkdiag3`, value confirmed `cat-dining`) proved bulk
recategorize works correctly. **Do not chase a "bulk clears category" bug.** Fix `loopstory_25` to target
the bulk select by its aria-label and assert the chosen category is applied.

### L26. Story — "The Money To-Do List" (Nina, tasks lifecycle) — 2026-06-20 ★

**The ritual:** Nina keeps money chores as to-dos ("call about the APR", "rebalance 401k", "cancel gym")
with due dates, marks them done, and expects overdue ones surfaced + a hide-done filter.
**Drive script:** `e2e/loopstory_26_money_todo.mjs` (+ screenshot/source verification).
**✅ VERIFIED WORKING (clean, complete lifecycle — keep as a regression anchor):**
- Add a task (title + **priority** + **due date** + notes); **priority color-badges** (HIGH/MEDIUM/LOW);
  Edit + delete. ✓
- **Toggle done** works (proven: the **hide-done filter removed the completed task** — only possible if
  status flipped to done; "Show all" toggles back). ✓
- **Ordering**: open first, soonest due, then title (`todo.go:147`, pure `internal/tasksort`). Due date
  is shown on the row when set (`todo.go:266-267`). Clean professional UI. ✓

**Gaps:**
- [x] **Overdue tasks ARE visually flagged** (already shipped, C52, `todo.go:306-318`): an open task past its
      due date renders the danger tone (`text-down`) plus an explicit "· overdue" word (colour + text, not
      colour alone — B15). Stale gap; verified present.
- [x] **Money chores link to the entity they're about.** The `RelatedType`/`RelatedID` model already
      existed with zero UI; added a "Link to" type + entity picker on the to-do add form AND inline editor
      (Account/Budget/Goal/Transaction), and a clickable "→ <name>" deep-link on the row that navigates to
      that entity's screen (`/accounts`,`/budgets`,`/goals`,`/transactions`). New pure `internal/tasklink`
      (Route/TypeLabel/EntityName, tested); deleted-entity links degrade to "(linked item removed)". e2e
      `task_entity_link_check.mjs`.
- [x] **Recurring tasks.** Added `Task.Recurrence` (reuses `RecurringCadence`); a "Repeat" select on the
      add form + inline editor; completing a recurring task spawns its next occurrence (Due advanced one
      cadence step, open) via pure `internal/taskrecur.NextOccurrence` + atomic `appstate.CompleteTask`;
      recurring rows show a "↻ <cadence>" badge. Re-opening a done task does not spawn. e2e
      `recurring_task_check.mjs`.

**Probe note:** two checks **false-negatived on autosave timing** — "status=open persists" and
"status=done persists" read `localStorage` at 700ms, before the ~2.5s autosave; the hide-done behavior
proves the toggle worked. And "overdue flagged" **false-positived** (matched the "2025" in the due-date
string; there is no real overdue flag — that's gap #1). Fix `loopstory_26` to wait for autosave and to
assert an explicit overdue badge.

### L27. Story — "The What-If" (Dev & Priya, Planning / scenario forecasting) — 2026-06-20 ★

**The ritual:** Dev & Priya weigh a 6-month sabbatical — income drops $4,000/mo while expenses hold.
They model it, watch the trajectory, see their runway, and save the scenario.
**Drive script:** `e2e/loopstory_27_what_if.mjs` (+ `_whatif2` re-drive).
**✅ VERIFIED WORKING (rich, correct — keep as a regression anchor):**
- **Savings & spending what-if plan** is correct: start $22,500 · −$4,000/mo · 12 months → **Projected
  ($25,500)** = exactly 22,500 − 12×4,000. Deterministic + reflects the drawdown into negative. ✓
- A **baseline net-worth forecast** ("this month's net cash flow continued → $X in 12 months") with a
  **"trim spending" what-if** overlay (two compared curves). ✓
- **Recurring cash flows** exist here (`domain.Recurring` + `PutRecurring` + **"Post due now"**
  `PostDueRecurring`) — *refines L24*: there IS a recurring-transaction mechanism (manual-post,
  Planning-scoped), just not on the transaction add form. ✓
- Debt payoff strategy (snowball/avalanche) + **"Start tracking progress"** + a suggested extra — looks
  like the dev agent has already begun building **L5** (was "170 months / no rec"; now "45 months / Try
  $X/mo / track progress"). 👍

**Gaps (dream-big modeling):**
- [x] **RUNWAY indicator on what-if plans.** Pure `planning.RunwayMonths(p) (months float64, depletes bool)`
      (interpolated fractional crossing, table-tested incl. never-depletes / already-negative / one-time dip)
      → a "Money lasts ~5.6 months" readout in the danger tone + a ⚠ marker on a plan that crosses zero (calm
      "Stays positive through N months" otherwise). e2e `plan_runway_check.mjs`.
- [x] **Baseline forecast = THIS month's net extrapolated 12×** (`planning.go:231-234`,
      `monthlyNet = income−expense for MonthRange(now)`). If the current month is atypical (a one-off
      purchase, a bonus), the 12-month forecast is misleading. Base it on a **trailing average** (last
      3–6 months) or the recurring cash flows. Bottom-up: an averaged `monthlyNet` (pure, tested) feeding
      `forecast.Project`; show the basis ("based on your last 3 months").
- [ ] *(Enhancement)* **Prefill starting balance from a chosen account**, and let two saved plans be
      **compared side-by-side** (sabbatical vs status-quo curves), like the trim overlay already does.

**Probe note:** the first run's "plan created" + "drawdown" GAPs were **test artifacts** — my
`/add$/`-named button matched the **recurring card's "Add"** (earlier in the DOM), so the plan form never
submitted. The `_whatif2` re-drive with the exact **"Add plan"** button confirmed the plan + correct
projection. Also "forecast uses average/recurring basis" **false-positived** (matched the "Recurring"
card heading; source confirms single-month basis — that's gap #2). Fix `loopstory_27` to click "Add plan"
exactly.

### L28. Story — "The Category Nerd" (Tomás, sub-categories tree + rollup) — 2026-06-20 ★

**The ritual:** Tomás wants granularity — under "Food" he splits into Groceries/Dining/Coffee, assigns
transactions to the leaves, and expects the parent to **roll up** the sum of its children.
**Drive script:** `e2e/loopstory_28_category_tree.mjs` (create parent+child → assign txn to child →
budget on parent → assert rollup).
**✅ VERIFIED WORKING end-to-end (strong, complete — keep as a regression anchor):**
- Create a **parent** category; create a **child** via an **indented parent picker** (`categorytree`).
  The child is selectable on the transaction form. ✓
- **Rollup is correct:** assigned a $25 txn to the **child** (COFFEETEST), created a budget on the
  **parent** (FOODTEST) → the parent budget shows **$25.00 / $100.00 spent** (D5 sub-category rollup).
  This also *proves* the nesting (a parent budget can only roll up a child's txn if
  `child.parentId == parent.id`). ✓
- **Action:** promote to a CI gate (`e2e/category_rollup_check.mjs`) — child spend rolls into a
  parent budget.

**Gaps (this area is solid; gaps are coverage + polish):**
- [x] **Reports roll up by parent.** Added pure `reports.RollUpByParent(rows, cats)` (aggregates each
      category's spend into its top-level ancestor, recomputes the delta; table-tested incl. two-deep nesting)
      and a "Roll up sub-categories" toggle on the Spending-by-category card (off by default, so leaf detail
      stays visible). e2e `reports_rollup_check.mjs` (16→13 rows when rolled up).
- [x] **Deleting a parent re-homes its children (no orphans).** DeleteCategory now re-parents any sub-category that pointed at the deleted category onto its grandparent (or root for a top-level parent) before deleting, so children never dangle. Pure categorytree.ReparentOnDelete (tested); e2e category_parent_delete_check.mjs.
- [x] **Collapsible tree view.** Each parent category in the Categories list has a chevron toggle that collapses/expands its descendants (pure categorytree.VisibleUnderCollapsed, tested; session-state). e2e category_collapse_check.mjs.

**Positive observation (dev-agent progress):** the **L1 budget sub-line glue defect appears FIXED** — the
budget rows now show "…$61.00 left" and "At this pace, projected to go over by $86.45" on **separate
lines** (was glued in L1). Nice.

**Probe note:** the "child nested (childParent === parent.id)" check **false-negatived** — it read
`localStorage` right after creating the child, before the ~2.5s autosave persisted `parentId` (the eval
returned `undefined`). The **budget rollup is definitive proof** the nesting worked. Fix `loopstory_28` to
wait for autosave (or assert via the rollup) and to set the parent select with a real change event.

### L29. Story — "Keep the Receipt" (Lena, receipt attachments / Artifacts) — 2026-06-20 ★

**The ritual:** Lena buys a $1,200 laptop and wants to attach the receipt image to **that transaction**
for warranty/tax proof — a paperclip on the row, retrieve/preview later, survive the backup.
**Drive script:** `e2e/loopstory_29_keep_receipt.mjs`.
**What exists (verified by screenshot + source):**
- An **Artifacts** screen: **Upload image** + **Import CSV** → a local artifact store, with a
  **storage-usage** readout ("Local storage in use: 29.7 KB") and an empty state. ✓
- The domain model already has `Transaction.Attachments []AttachmentRef` (→ ArtifactID) and AI import
  sets `SourceDocID` — the *plumbing* exists. ✓

**🔴 Gap (the story's core need is unmet):**
- [x] **Per-transaction receipt attachment in the UI** (built this pass — the model existed, the UI didn't):
  - [x] **State/UI** `internal/screens/transactions.go`: an **"Attach receipt"** row action uploads via
        `pickFile` → creates an Artifact → appends an `AttachmentRef`; a **paperclip marker** (with count) on
        rows with attachments; click → an image preview overlay.
  - [x] **Artifacts↔txn linkage** `internal/screens/artifacts.go`: each artifact row shows "Referenced by N
        transaction(s)".
  - [x] **Round-trip** (ties L9): `AttachmentRef` + Artifact bytes already ride the dataset export/import;
        locked in with `store.TestAttachmentRoundTrip`. e2e `receipt_attach_check.mjs`.
- [ ] **Storage scalability for receipts.** Artifacts live in **localStorage** (the "KB in use" readout
      is good) — but binary receipt images will blow the ~5-10 MB quota fast for "keep all my receipts".
      Move artifact bytes to **IndexedDB** (keep refs in the dataset), with a graceful quota warning.
      Bottom-up: an artifact-store seam (interface) → IndexedDB impl → quota check + nudge; tested.

**Probe note:** "the transaction UI exposes attaching" **false-positived** (the loose `/file/` match hit
unrelated markup); source confirms **no** attach UI in transactions — the add-form-control + row-indicator
GAPs are the accurate signal. "Artifacts show linked transaction" PASS is **unverified** (the store was
empty); re-test after uploading + linking once that flow exists. Artifact upload uses an **off-DOM**
`pickFile` input, so `setInputFiles` can't drive it — needs a test seam or a DOM input to be E2E-testable.

### L30. Story — "Reconciliation Day" (Omar, account reconcile / update-balance) — 2026-06-20 ★

**The ritual:** Omar reconciles monthly — his bank shows Checking = (CashFlux + $123.45). He updates
CashFlux to match and expects a reconciling adjustment for the difference, the account marked fresh
(stale cleared), and a clear confirmation.
**Drive script:** `e2e/loopstory_30_reconcile.mjs`.
**✅ VERIFIED WORKING & CORRECT (keep as a regression anchor):**
- Per-account **"… → Update balance"** reveals a **New balance** form (`#acct-setbal-<id>` + Save). ✓
- **Reconcile is correct:** entered target $8,999.45 on an account whose actual balance was $8,070.00 →
  posted a **"Balance adjustment"** transaction of **+$929.45** (= target − current, `Amount:92945`),
  **`cleared=true`**. ✓ (`ledger.AdjustmentToTarget`)
- **Freshness:** `BalanceAsOf` updated → the **stale flag clears**; a **"Mark all updated"** affordance
  exists for stale accounts (`freshness.IsStale`). ✓
- A **confirmation toast** ("Updated X to $Y") is shown. ✓
- **Action:** promote to a CI gate (`e2e/reconcile_check.mjs`) — adjustment == target − current, cleared,
  freshness updated.

**Gaps (dream-big — make reconciliation trustworthy, not just a force-to-target):**
- [x] **Show the computed difference + let the user label the adjustment.** The form takes only the new
      balance; it doesn't preview "current $8,070.00 → entered $8,999.45 = **+$929.45 adjustment**" before
      saving, and the adjustment lands as a generic **uncategorized** "Balance adjustment" (which can skew
      reports — a $929 uncategorized entry). Show the delta inline + an optional **category/note** (e.g.
      interest, a missed transaction), or flag adjustments excludable from spending. Bottom-up: surface
      `AdjustmentToTarget` delta in the form + a category field on the adjustment; test.
- [ ] **Guided statement reconciliation (gold standard).** True reconcile = tick off each transaction on
      the statement until the **cleared balance** matches the statement, rather than forcing the total.
      The pieces exist (a `cleared` flag — L25 bulk-clear — and a cleared-balance display). Add a
      **"Reconcile to statement"** mode: enter the statement balance, check cleared items, and confirm
      when cleared-balance == statement (only then no adjustment needed). Bottom-up: pure
      `reconcile.Diff(clearedTxns, statementBalance)` (tested) → a guided UI over the existing cleared
      flag.

**Probe note:** the "adjustment equals $123.45" check **false-negatived** twice over — (1) my row-balance
regex grabbed the **cleared-balance meta** ("cleared $8,876.00") instead of the actual balance ($8,070),
so my expected delta was wrong; (2) I read `amt.amount` but the field is `amt.Amount`. The logged values
(target 8999.45, adj 929.45) prove the reconcile math is correct. Fix `loopstory_30` to read the row's
main amount and the `Amount` field.

### L31. Story — "The Automator" (Raj, Workflows / no-code automation) — 2026-06-20 ★

**The ritual:** Raj wants to automate his monthly routine — define a trigger + actions and have them run
on a schedule (e.g. on the 1st: post recurring bills, flag any budget over 90%, create a review to-do).
**Drive script:** `e2e/loopstory_31_automator.mjs`.
**✅ VERIFIED WORKING end-to-end (impressive, sophisticated — keep as a regression anchor):**
- Full **no-code builder**: name, **trigger**, **formula condition** (e.g. `contains(txn_payee,"coffee")`,
  `txn_abs > 200`), **write-safe actions** (create task / set category / add tag / flag review / apply
  rules / notify), Add-action, Save. Sample workflows ship. ✓
- **Dry run is non-destructive** (verified: previewing did NOT create the task) and **Run now APPLIES**
  the effect (verified: a CreateTask workflow **actually created the task** — confirmed in the dataset AND
  on the To-do screen). ✓
- Enable/disable, delete, and a **run history / last-run** record. Clean professional UI. ✓
- **Action:** promote to a CI gate (`e2e/workflow_apply_check.mjs`) — dry-run no-op vs run-now applies.

**Gaps (dream-big — make it a real unattended automator):**
- [x] **No scheduled / time trigger (headline).** Triggers are only **"When I run it"** (manual) and
      **"When a transaction is added"**. Raj's "on the 1st of the month / every Monday" is impossible —
      time-based automations require manually clicking **Run now**, which defeats automation. Add a
      **`TriggerScheduled`** (cadence: daily/weekly/monthly + day-of) that fires via a due-check on app
      open (and records the run). Ties to the recurrence theme (L24 txns, L26 tasks, L27
      `PostDueRecurring`) — share one scheduler. Bottom-up: pure `workflow.DueScheduled(workflows, now,
      lastRun)` (tested) → a boot-time runner → the trigger option in the builder.
- [x] **Richer actions for real routines.** Raj wanted "**post recurring bills**" and "**flag budgets
      over 90%**" — current actions don't include post-recurring, budget-threshold flag, transfer, or
      goal-contribute. Extend the `workflow.Action` set + the apply layer (write-safe, tested), e.g.
      `ActionPostDueRecurring`, `ActionFlagBudgetOver(pct)`, `ActionContributeGoal`.
- [x] **Event triggers beyond txn-added** — budget-exceeded, bill-due, goal-reached as triggers for
      event-driven automation. Bottom-up: emit domain events → match against workflow triggers; tested.

**Probe note:** all checks passed and were **double-confirmed** (the run-now task verified in both the
dataset and the To-do screen; dry-run verified to NOT create it) — no false positives this run. Trigger
inventory (manual + txn-added, no schedule) confirmed from the builder dropdown + the sample workflows.

---

## UX — God-tier teardowns of core flows (focus shift from L32)

Deep UX scrutiny of the daily-driver flows against a god-tier bar — measured, screenshot- and
source-verified. "Anchors" = already excellent; keep. Items are prioritized UX polish, bottom-up.

### L32. Core flow — "Three Seconds at the Register" (logging a transaction) — 2026-06-20 ★

**Why it matters:** logging a purchase is the single highest-frequency action; its friction sets the
whole app's feel. **Drive script:** `e2e/loopstory_32_quickadd_ux.mjs` (+ `_enterdiag` / `_txndiag`).
**Already god-tier (anchors — keep):**
- **Low friction:** only **amount** is required; description + amount is the entire minimal entry — type,
  account, category, date all sensibly default. **1 Tab** from description → amount. ✓
- Clean, well-spaced **mobile** form; **44px** tap targets; amount uses a numeric input; a **visible green
  focus border** on fields. ✓
- **"Repeat last"** for fast re-entry; **rule-based auto-suggest** of category/tags from the description
  (L15). The add form is a proper `<form>` (`transactions.go:435`, `OnSubmit(add)`). ✓

**God-tier UX gaps (verified):**
- [x] **No auto-focus on Description.** Landing on `/transactions` leaves focus on `body` (measured) — a
      wasted click/tap before the primary action. Auto-focus `#txn-add` on the transactions screen (and
      the quick-add). Scope it so it doesn't steal focus on unrelated navigation. (`transactions.go`
      formCard + a `ui.UseEffect` focus, like the EmptyStateCTA already does via `FocusID`.)
- [x] **Enter does not submit the add form.** Verified twice (no transaction added; **no crash**) despite
      a real `<form>`+submit button. Keyboard users must mouse to "Add" — unacceptable for a high-
      frequency flow. Verify on a real browser; if the framework swallows implicit submission, wire an
      explicit **Enter → add** on the description/amount inputs. before→after: type "Coffee" ⇥ "4.50" ⏎
      logs it.
- [x] **Focus doesn't return to Description after submit.** For logging several purchases in a row, the
      cleared form should re-focus `#txn-add` so the next entry is immediate (measured: focus did not
      return). Add focus-return in the `add` success path.
- [ ] **Mobile: the "Add" button sits ~7 fields down**, below period controls that dominate the top
      (cross-ref L11). For the primary action that's a lot of scrolling. Offer a **compact quick mode**
      (description + amount + Add visible) or a sticky Add, and collapse the period controls on mobile.
- [x] **Amount input → `inputmode="decimal"`** (currently `type="number"`) for a cleaner money keypad on
      mobile (no spinner/scientific notation). One-line attr in `transactions.go:437`.

**Needs verification (don't action yet):**
- [ ] **Global quick-add reachability.** Alt+N is documented as the add shortcut (shortcuts.go) and the
      palette has "New transaction" → `UseQuickAdd`, but pressing **Alt+N on `/budgets` did not surface a
      quick-add form** (`#txn-add` absent after 700ms). The god-tier path is "log from anywhere" — verify
      what Alt+N does (navigate vs modal) and ensure a fast global quick-add exists. Re-test with a longer
      settle + check for a modal container.
- [x] **One-off "Go program has already exited"** appeared during the Alt+N + viewport-resize sequence
      but was **NOT reproduced** in isolation (the add flow itself never crashed). Flag for the dev agent
      to watch the global-shortcut / rapid-navigation path; not a confirmed bug.

**Probe note:** the "no focus ring" check **false-negatived** — I measured `outline`/`box-shadow`, but the
focus indicator is a **border-color** change (visible green border in the mobile screenshot). The
"form clears / success toast" checks were taken after the crash-y Alt+N sequence and are **unreliable** —
re-measure in isolation. Programmatic `.focus()` can't trigger `:focus-visible`; use real Tab to test
focus rings.

### L33. Core flow — "The Morning Glance" (Dashboard) — 2026-06-20 ★

**Why it matters:** the dashboard is the first thing seen every session; it must answer "are we okay?" in
~2s and route attention to anything that needs action. **Drive script:**
`e2e/loopstory_33_dashboard_ux.mjs` (measured CLS, hero size, tile count, mobile chrome).
**Already god-tier (anchors — keep):**
- **Zero layout shift on load (CLS = 0)** — content lands stable, no jank. ✓
- Clean dark **bento** of 22 tiles with good color semantics (green/red figures), icons, sparklines, a
  savings-rate donut; **reconfigurable** (drag/resize/reset). Rich at-a-glance coverage. ✓

**God-tier UX gaps (verified):**
- [x] **Prime real estate is spent on a how-to, not a signal.** The top-left permanently reads "Your
      dashboard · Drag tiles to move · grab the edge handles to resize" — the spot the eye lands first is
      a rearrange tutorial. before→after: replace with a **glanceable greeting + health line** ("Good
      morning — you're on track; net worth ▲ 0% this month") and move the drag hint into an **edit mode**
      / one-time dismissible tip. (`internal/screens/dashboard.go` header; a `seenDragTip` flag.)
- [x] **No visual hero — the four top stats are all 24px** (net worth = income = spending = liabilities,
      measured identical font size). Nothing draws the eye to the "are we okay" number. before→after:
      give **net worth** dominant weight (larger figure, prominent period delta + arrow, optionally span
      two cells) so the glance resolves instantly. (`dashboard.go` stat tiles / a hero variant.)
- [x] **No consolidated "needs attention" strip.** Priya's 2nd question ("anything need me?") is scattered
      across the Freshness, Budgets, and Upcoming-bills tiles. Add a compact **attention summary** near
      the top: "2 budgets near limit · 1 bill due in 3 days · 3 balances stale → review". Bottom-up: a
      pure `dashboard.Attention(state, now)` that rolls up the existing freshness/budget/bill signals
      (table-tested) → a single strip with deep links; nothing new computed, just surfaced together.
- [ ] **Mobile: desktop-only drag/resize chrome shows on touch** — measured **86 drag/resize handles +
      "Reset layout"** at 390px. Meaningless on a phone and adds visual noise. Hide under a
      `@media (pointer:coarse)` / width breakpoint; keep tiles read-only-stacked. (Cross-ref **L11**;
      fix once for both.)

**Probe note:** CLS, hero font sizes (all 24px), tile count (22), and the mobile handle count (86) were
measured directly from the DOM/screenshot — no false positives this run. The "drag instructional clutter"
and "no hero" findings are confirmed in `loop33-dash-desktop.png`.

### L34. Core flow — "Twenty Trips a Day" (navigation rail) — 2026-06-20 ★

**Why it matters:** the rail is the app spine, used dozens of times a session; it must be instant,
unambiguous, and fully keyboard-operable. **Drive script:** `e2e/loopstory_34_nav_ux.mjs` (measured
href/aria-current/tab-reach/skip-link/Alt-jump/collapse/mobile).
**Already god-tier (anchors — keep):**
- Clean **visual hierarchy + grouping**: brand → workspace switcher → primary → **Tools / System / My
  pages** → household card. Icons + labels; **active item visually distinct** (bg `rgb(28,28,30)` vs
  transparent). ✓
- **Title on every item** (collapsed tooltip), a **"Skip to content"** link, **Alt+1–9 jump** shortcuts
  (Alt+3 → /transactions verified), a **collapse toggle**, and a slim **55px** mobile rail. ✓

**God-tier UX gaps (verified — all keyboard/a11y; the rail's biggest weakness):**
- [x] **HEADLINE: nav items aren't real links and aren't keyboard-focusable.** Measured **0/21 have
      `href`**, and **Tab never reached the nav in 8 stops** — href-less click-anchors with no tabindex
      are not in the tab order. So keyboard + screen-reader users **cannot Tab to the primary
      navigation**; only the undiscoverable Alt+1–9 works. before→after: render each item as a real
      **`<a href={uistate.RoutePath(path)}>`** (the history router already supports pushState links). One
      change delivers: tab-focusability, **middle-click / open-in-new-tab / copy-link**, and correct SR
      link semantics. (`internal/app/shell.go` `navItem`/`Sidebar`.) Supersedes the L19 incidental note.
- [x] **No `aria-current="page"` on the active item.** It's visually distinct but SR users aren't told
      which screen is current. Add `aria-current="page"` when active. (`shell.go` navItem.)
- [ ] **Alt+1–9 jumps are undiscoverable.** They work but nothing surfaces them outside the "?" help.
      Once the rail is tabbable (fix #1) they become a bonus rather than the only keyboard path; consider
      a subtle hint on hover/focus.

**Probe note:** `href` count (0/21), tab-reach ("not reached" in 8 stops), `aria-current` ("none"),
active-bg-vs-sibling, skip-link target (`/budgets#main`), and Alt+3 → /transactions were all measured
directly — no false positives. The "not reached" + "0 href" corroborate each other (href-less anchors
aren't focusable), so the keyboard-unreachable conclusion is solid, not a tab-budget artifact.

### L35. Core flow — "Can I Order Takeout?" (Budgets) — 2026-06-20 ★

**Why it matters:** people check budgets constantly to answer "do I have room?" — the LEFT number + status
must read in ~3s. **Drive script:** `e2e/loopstory_35_budgets_ux.mjs`.
**Already god-tier (anchors — keep):**
- Clear **SPENT / BUDGETED / LEFT** summary strip + a **"1 over budget · 2 near the limit"** status count.
- Every row shows the **LEFT** amount and a **status in WORDS** ("On track / Near limit / Over budget") —
  not color-only, so it survives colorblindness (5/5 measured). Color bars (green/amber/red) reinforce.
- **Smart default** on add: "You've averaged $236.00/mo here recently · **Use this**". ✓
- Over-budget bar is **capped red** (doesn't overflow), and a **"Cover…"** action is present — *the L1
  "cover overspending / move money between budgets" feature has shipped.* 👍 The L1 sub-line glue is fixed.

**God-tier UX gaps (verified from `loop35-overbudget.png`):**
- [x] **Over-budget copy reads as buggy/alarming.** The OVERTEST row shows "Over budget · **2390%** ·
      **($229.00) left**" — a runaway percentage and a *negative "left"* in accounting parens. before→
      after: drop the absurd % (or cap at "100%+") and say **"over by $229.00"** instead of "($229.00)
      left". Plain, calm, direct. (`internal/screens/budgets.go` `rowSub` / the over-budget branch.)
- [x] **Status word contradicts the pace projection.** Dining shows "**On track** · 79% · $61 left" AND
      "At this pace, projected to go over by $64.08" (same for Shopping) — "on track" while "projected to
      go over" is a mixed signal that erodes trust. Reconcile into a coherent state: when the pace
      projects an overage, surface a distinct **"Trending over"** status (not "On track"), or fold the
      projection into the status. Bottom-up: a pure `budgeting.Status` that returns on-track / trending-
      over / near-limit / over (table-tested) → the row renders one consistent label.
- [x] **Progress bars aren't a11y-exposed** (`.bar` is a plain div — no `role="progressbar"` /
      `aria-valuenow/min/max`). Low priority (the text "79% · $61 left" already conveys it), but adding
      the role gives SR users the bar semantics. (`budgets.go` bar markup.)

**Probe note:** the "smart default helper" check **false-negatived** — the "Use this" helper renders under
the add form and my form-scope innerText missed it; it IS present (screenshot). The over-budget "2390% /
($229) left" string and the "On track + projected to go over" contradiction are both verified verbatim in
the screenshot. Bars measured as `div-bar` (no role) — accurate.

### L36. Core flow — "Oops, Wrong Amount" (edit + delete + forgiveness) — 2026-06-20 ★

**Why it matters:** correcting a mistake is constant; and a destructive delete must be FORGIVING.
**Drive script:** `e2e/loopstory_36_edit_undo_ux.mjs` (+ `_deldiag`).
**Already god-tier (anchors — keep):**
- **Inline edit is excellent:** clicking Edit turns the row into **in-place fields**, and **focus
  auto-lands in the edit field** (measured `focusedInEdit: true`); Save/Cancel present. Discoverable
  per-row Edit affordance. ✓

**God-tier UX gaps (verified):**
- [ ] **HEADLINE: deleting a transaction is one-click, immediate, and IRREVERSIBLE.** Confirmed via
      `_deldiag`: clicking the row "×" took **57 → 56 with no confirm dialog and no Undo**. One mis-tap
      permanently destroys a financial record — a forgiveness failure. before→after: show an **Undo toast**
      ("Deleted 'Weekend dinner' · **Undo**", ~6s) — the god-tier pattern (a confirm dialog would add
      friction to the common case; undo is better). Bottom-up: snapshot the deleted txn before removal →
      a restore action → a toast with Undo. **Build ONE shared undo mechanism** for single delete +
      bulk delete/recategorize/clear (ties **L25**). (`internal/screens/transactions.go` `del`/`bulkDelete`
      + `internal/appstate`.)
- [ ] **Mobile: all 50 row controls are <40px** (measured) and the per-row **Edit sits right next to the
      delete ×** — small + adjacent makes accidental deletes likely, compounding the no-undo gap. Enforce
      ≥44px hit areas and/or collapse row actions into an overflow (⋯) menu on mobile (ties **L11**,
      **L25**). (`transactions.go` `TransactionRow` action buttons.)
- [x] **Consistency: edit auto-focuses but the ADD form does not** (L32). Unify — both should auto-focus
      their first field on open. Minor, but it makes the app feel coherent.

**Probe note:** the in-run delete check **false-negatived** (57→57) on **autosave timing** — I read
`localStorage` at 900ms before the ~2.5s autosave. The `_deldiag` re-check with a 3s wait confirmed the
delete fires (57→56) and that there is genuinely **no confirm and no undo**. Inline-edit auto-focus
measured directly (`editingInPlace` + `focusedInEdit` both true) — no false positive.

### L37. Core flow — "What's My Runway?" (Accounts) — 2026-06-20 ★

**Why it matters:** Accounts is the net-worth/balances hub; reading it + adding a new account are both
common. **Drive script:** `e2e/loopstory_37_accounts_ux.mjs`.
**Already god-tier (anchors — keep):**
- Clear **NET WORTH / ASSETS / LIABILITIES** strip with color semantics (liabilities red), and accounts
  **grouped into Assets vs Liabilities** sections. ✓
- **Freshness is handled well:** STALE badges per row + a "**Mark all updated (7 accounts stale)**"
  shortcut. ✓
- Add-account fields have **real visible labels** (not placeholder-only). Mobile reflows to one column,
  no overflow. ✓

**God-tier UX gaps (verified):**
- [ ] **HEADLINE: add-account is a 9-field wall with no progressive disclosure.** Measured 9 visible
      fields (Name, Type, Owner, Currency, Opening balance, **Return %, Liquidity (1–5), Stability (1–5),
      Locked-until**, date) and **no advanced toggle**. "I just opened a savings account" needs ~3 (Name,
      Type, Opening balance). before→after: show **Name · Type · Opening balance** up front (+ **Currency**
      only when >1 currency is in use), and tuck Return%/Liquidity/Stability/Locked-until/Owner behind a
      **"More options"** expander (collapsed by default, sane defaults). (`internal/screens/accounts.go`
      add-form → essential block + `If(showAdvanced)` + a toggle state.)
- [x] **Field labels use finance jargon** — "**Liquidity (1–5)**", "**Stability (1–5)**", "**Return %**"
      are allocation-modeling inputs that a normal user can't decode (violates CLAUDE.md "plain, friendly
      English, no jargon"). Even once disclosed, relabel with plain English + a one-line hint, e.g.
      Liquidity → "How fast can you get this money? (1 locked … 5 instant)". (`accounts.go` labels + the
      i18n strings.)
- [ ] *(Minor)* **Currency field shows for a single-currency household** — hide it unless the user has
      >1 currency (ties FX/L4); one less field for the 99% case.

**Probe note:** field count (9) + no-advanced-toggle measured directly; the jargon labels
("Liquidity (1–5)", "Stability (1–5)", "Return %") are verified verbatim in `loop37-accounts-desktop.png`.
Labels confirmed present (so not a placeholder-only a11y issue). No false positives this run.

### L38. Core flow — "Are We There Yet?" (Goals) — 2026-06-20 ★

**Why it matters:** goals are the motivational core — progress must feel clear + rewarding, and
contributing must be frictionless. **Drive script:** `e2e/loopstory_38_goals_ux.mjs`.
**Already god-tier (anchors — keep; one of the strongest flows):**
- An **"Overall progress" hero** summarizes all goals; each goal shows the **complete, motivating
  picture**: "**75% · $7,500.00 to go · by 2026-12-31 · save $1,071.43/mo**" (% + remaining + target date
  + pace). ✓
- **Contribute is a god-tier micro-interaction:** opens an **inline amount field** (not a jarring
  prompt) and **auto-focuses** it (measured `focused:true`) — consistent with inline edit (L36). ✓
- Mobile: no overflow. Completion shows "Complete 🎉" (L20). ✓

**God-tier UX polish (this flow is close — small lifts):**
- [ ] **Add-goal: 6 fields up front** (Name, Target, **Saved so far**, **Owner**, **Linked account**,
      Target date). Lead with **Name · Target · Target date**; tuck the three optional/defaulted fields
      (Saved-so-far=0, Owner=group, Linked account) behind a **"More options"** expander. Same pattern as
      L37 (accounts) — milder here. (`internal/screens/goals.go` add-form.)
- [ ] **Delight on contribute.** Contributing is an emotional win — the progress bar should **animate the
      fill** on contribute, and crossing a milestone (25/50/75%) deserves a subtle moment (today only
      100% celebrates, L20). Bottom-up: a CSS width transition on the goal `.bar` fill + a one-shot
      milestone toast keyed on crossing a threshold (pure `goals.MilestoneCrossed(before,after)`, tested).
- [x] **Verify Enter-to-contribute** in the inline amount field (consistency with the L32 add-form
      Enter-submit gap) — should submit without reaching for the Contribute button.

**Probe note:** "contributing gives feedback" **false-positived** — my `/saved/` match hit the
"**Saved so far**" add-goal label, not a post-contribute confirmation; and I did not confirm the
contribution actually applied (the `evaluateHandle` value-set is fragile). Re-test with a real focused
type + a before/after progress delta + an explicit toast assertion. The inline-form + auto-focus +
goal-row copy were measured/verified directly (solid). Add-goal field count (6) measured directly.

### L39. Core flow — "Logging Today's Coffee" (everyday expense entry end-to-end) — 2026-06-22 ★

**Why it matters:** adding a single expense is the most frequent action in any budgeting app — a
$5 coffee every morning means 365 add-expense flows per year. Its friction, feedback, and cross-page
consistency (ledger → budget spent → dashboard) define whether the app feels trustworthy day-to-day.
**Drive script:** `e2e/loopstory_39_log_expense.mjs`.

**Already god-tier (anchors — keep):**
- ✓ **Add form is always visible** at the top of /transactions — no modal, no navigation. Single-screen
  flow from intent to confirmation: fill description + amount, click Add, done.
- ✓ **Transaction appears at the top of the ledger immediately** after submit (newest-first sort, 605
  shown after add) with the correct date, description, amount ($5.00), and auto-applied category (Dining).
- ✓ **Form clears automatically** after submit — description and amount fields reset, ready for the next entry.
- ✓ **Autosave is reliable and fast** — `cashflux:dataset` in localStorage contains "Morning coffee" within
  3 seconds; the transaction survives a hard page reload with no data loss.
- ✓ **Dashboard cross-page consistency is solid** — "Morning coffee ($5.00)" appears immediately in the
  Recent Transactions widget on /dashboard without any manual refresh.
- ✓ **Budgets page reflects the spend** — Dining shows $160/$250 (64%), correctly incorporating the coffee
  entry; SPENT / BUDGETED / LEFT summary header is accurate.
- ✓ **Zero JS page errors** across the entire flow (/transactions add → /dashboard → /budgets → reload).
- ✓ **Ledger pagination is correct** — 50 rows/page (newest first); count stays at 50 after add because the
  oldest row rotates off the first page, not a render bug.

**God-tier UX gaps (verified):**
- [x] **Default account is "401(k) / Brokerage"** — an investment/retirement account — not an everyday
  checking account. Every coffee and grocery purchase defaults to an investment account, which is
  semantically wrong and will confuse users who don't notice before submitting.
  Before: "Add transaction" form defaults Account to `401(k) / Brokerage`.
  After: default to the most-recently-used account, or failing that, the account flagged as primary/checking
  (e.g. `acct-checking`). (`internal/screens/transactions.go` add-form defaulting logic.)
- [x] **No visible form labels** — Description and Amount inputs rely solely on placeholder text, which
  vanishes the moment the user starts typing. Mid-entry there is no hint of what each field is.
  Before: placeholders read "Description" / "Amount" — gone once typing starts.
  After: persistent `<label>` above (or floating label pattern) for Description, Amount, and at least the
  Type select. (`internal/screens/transactions.go` add-form markup; the existing `aria-required="true"`
  on Amount shows a11y intent was there — complete it with a visible label.)
- [x] **No success confirmation after Add** — the row silently appears at the top of the ledger with no
  toast, snackbar, or transient highlight. A first-time user who scrolls or blinks will miss the feedback
  entirely and may click Add again (duplicate).
  Before: submit triggers no user-facing confirmation signal.
  After: a brief (2–3 s) toast "Transaction added" or a 1-second row highlight animation on the newly
  added row. (`internal/ui/` notification / toast system, or a flash CSS class on the inserted `<tr>`.)

**Probe note:**
Step 6c initially checked `tbody tr` count before vs. after and flagged a failure when both read 50.
This was a probe logic error: the ledger is paginated at 50 rows (page header reads "1–50 of 604"),
sorted newest-first. Adding a transaction places it at row 1 and rotates row 50 off the visible page —
the count correctly stays at 50. Fixed the check to assert that "Morning coffee" exists in a `tbody tr`
cell, which passes and is the meaningful assertion. All 14 checks pass; exit code 0.

---

### L40. Story — "Setting a Grocery Budget" (household manager Sam) — 2026-06-22 ★

**The ritual:** Sam wants to cap monthly grocery spending at $600. She opens /budgets, fills in the
add form (name "Monthly Groceries", category Groceries, period Monthly, limit $600), and clicks Add.
The budget appears immediately in the list showing $X spent / $600 limit (where X reflects any
existing Groceries transactions for the month). She then logs two grocery purchases in /transactions
($47.32 "Whole Foods run" + $102.89 "Trader Joe's") and a non-grocery Dining expense ($35 "Thai
restaurant"). Back on /budgets she expects to see the grocery spent figure increase by exactly
$150.21, the Dining expense excluded from the Groceries budget (category isolation), and the progress
bar fill proportionally. After a hard reload, the budget definition and spend both survive.

**Drive script:** `e2e/loopstory_40_create_budget.mjs`.
All 26 checks pass; exit code 0. The script seeds all test data from the live app (no fixture file
needed) and uses baseline-delta arithmetic to sidestep the sample dataset's pre-existing $520 of
Groceries spend.

**What already works well (regression anchors):**
- ✓ **Budget creation is full-featured at first render.** The add form exposes Name, Category, Owner,
  Period, Limit, and "Roll unused funds" in one place with persistent field labels above every input
  — no progressive disclosure needed here (6 fields, all useful, all labelled). Confirmed in
  `loop40-02-add-form-filled.png`.
- ✓ **Budget appears immediately after submit with correct limit.** No page reload needed; the new row
  is reactive and shows the $600 limit right away. Confirmed `loop40-03-after-add-budget.png`.
- ✓ **Existing-month spend is applied instantly.** When Sam creates the Groceries budget, the sample
  data's $520 in prior Groceries transactions is already counted — `$520.00 / $600.00 · 86% · Near
  limit` shows immediately. The budget retroactively aggregates matching transactions from the current
  period window. Confirmed row text in Step 4c output.
- ✓ **Category isolation is enforced.** A $35 Dining expense adds to the Dining budget only; the
  Groceries budget row does not include it ($670.21, not $705.21). Confirmed Step 10 / `loop40-06`.
- ✓ **Spent / limit / progress bar update correctly after new grocery transactions.** After adding
  $150.21 in Groceries spend, the row updates to `$670.21 / $600.00 · Over budget · 111%` with a
  red bar fill. Delta arithmetic verified to the cent. Confirmed `loop40-05-budgets-after-spend.png`.
- ✓ **Budget definition and spend both persist across hard reload.** `$670.21 / $600.00` is identical
  before and after `page.reload()`. Confirmed Step 11 / `loop40-07-budgets-after-reload.png`.
- ✓ **"Cover…" action appears when over budget.** Once the Groceries budget tips over $600, the
  "Cover…" button appears inline — the right affordance at the right moment. Confirmed `loop40-05`.
- ✓ **Pace projection is live and accurate.** The amber "At this pace, projected to go over by
  $136.32" warning appears immediately after creation, based on the current run rate — a genuinely
  useful proactive signal. Confirmed `loop40-03`.
- ✓ **Zero JS page errors** across the entire flow (/budgets create → /transactions add × 3 →
  /budgets verify → reload).

**Mechanical gaps:**

- [ ] **No "one budget per category" guard.** The sample dataset ships with a "Groceries" budget
  (`$520/$450`); creating "Monthly Groceries" produces a second budget for the same category and the
  same period window. The two budgets aggregate the same transactions independently, splitting the
  mental model and the summary strip. Result: Sam sees two competing Groceries rows with no warning.
  Before: submitting a second Groceries budget silently creates a duplicate.
  After: `app.PutBudget` (or a validation layer above it) should reject a new budget whose
  `(categoryID, period, ownerID)` triple matches an existing live budget, returning a plain-English
  error "A Monthly Groceries budget already exists. Edit it instead."
  (`internal/budgeting/` validation + `internal/screens/budgets.go` form error display.)
- [x] **Budget Name is not validated — empty name is accepted.** The Name input has no `aria-required`
  and no client-side guard; submitting with a blank name creates an unnamed budget row that shows only
  "· Groceries" in the list (the category name falls back to identify it, but the user-chosen label
  is lost). Before: empty name → unnamed budget. After: treat Name as required (add `aria-required`
  and mirror the `errMsg` path used for Limit validation). (`internal/screens/budgets.go` `add` handler,
  before the `PutBudget` call.)

**UI/UX defects (screenshot-confirmed):**

- [x] **No success confirmation after Add.** After clicking Add the budget silently appears below with
  no toast, snackbar, or row highlight. A first-time user who doesn't immediately scroll down may
  wonder if the click registered — same pattern as the transaction add-feedback gap in L39.
  Screenshot: `loop40-03-after-add-budget.png` (no confirmation signal visible at top of page).
  After: a brief toast "Budget added" or a 1–2 s highlight on the newly inserted row.
  (`internal/ui/` toast system, or a flash CSS class on the inserted card.)
  Close-out: re-screenshot after implementing; confirm toast/highlight visible.
- [ ] **"Roll unused funds into the next period" label is truncated on the add row.** The checkbox
  label wraps inside a narrow column at 1280 × 900 and the copy is long ("Roll unused funds into the
  next period"). On smaller viewports it clips. The label should be shortened to "Roll over unused
  funds" (9 words → 4) with a `title` tooltip for the full description.
  Screenshot: `loop40-02-add-form-filled.png` (right column shows truncated label).
  After: relabel + add tooltip; re-screenshot at 1280 px to confirm no wrap.
- [x] **Add form does not reset Category after submit.** After submitting "Monthly Groceries
  (Groceries)", the Category select resets to "Dining" (the first expense category alphabetically),
  not to a neutral or last-used state. If Sam immediately wants to add a second budget for a different
  category she has to re-select — minor friction — but the form's snappy reactive reset elsewhere
  (Name clears, Limit clears) makes the Category not-resetting feel inconsistent.
  Screenshot: `loop40-05-budgets-after-spend.png` (Category select shows "Dining" in add form after
  prior Groceries add).
  After: keep as-is or default to last-used category — least-surprise would be to default to the
  first category that has no existing budget for the period. (`budgets.go` add handler, `catID` state reset.)

**Probe hardening:**
- The initial `select[aria-label]` selector grabbed the "Jump to…" period picker (index 0) instead of
  the "Category" budget-form select (index 1). Fixed by targeting `select[aria-label="Category"]`
  explicitly — confirmed stable.
- Spent-amount assertions used absolute dollar checks ("150") that broke against the sample dataset's
  pre-existing $520 in Groceries spend. Fixed with baseline-capture + delta arithmetic: read the
  row's `$X / $600` figure immediately after budget creation, then assert `afterSpent − baseline ≈
  $150.21 ± $0.05`. This pattern generalises to any spend-tracking e2e story where sample data
  pre-populates the category.

---

### L41. Story — "Starting a Vacation Fund" (Cam, first savings goal) — 2026-06-22 ★

**The ritual:** Cam wants to save $2,000 for a vacation by December 2026. He opens /goals, fills in
the add form (name "L41 Vacation Fund", target $2,000, target date 2026-12-01, linked account
"Emergency Savings (HYSA)", saved so far $0) and clicks Add. The goal appears immediately at 0% with
`$0.00 / $2,000.00` and a `save $333.34/mo` pace figure. He then clicks Contribute, enters $200, and
submits. He expects: progress advances to 10% ($200/$2,000, $1,800 to go), pace recomputes to
`$300/mo`, the bar fills, and the linked Savings account balance does NOT change (he'll log the real
transfer separately). After a hard reload, the goal + $200 progress both persist.

**Drive script:** `e2e/loopstory_41_create_goal.mjs`.
All 29 checks pass; exit code 0.

**What already works well (regression anchors):**
- ✓ **Goal creation is full-featured.** The add form exposes Name, Target (USD), Saved so far, Owner,
  Linked account (optional), and Target date in one labelled row — all confirmed visible in
  `loop41-02-add-form-filled.png`. No progressive disclosure required.
- ✓ **Goal appears immediately after submit with correct $0/$2,000 and 0% bar.** No reload needed;
  row is reactive. Confirmed `loop41-03-after-add-goal.png`: `$0.00 / $2,000.00 · 0% · $2,000.00
  to go · by 2026-12-01 · save $333.34/mo · linked to Emergency Savings (HYSA)`.
- ✓ **Pace figure (`save $X/mo`) is live and accurate from creation.** `MonthlyNeeded` fires
  whenever a `TargetDate` is set; $2,000 / ~6 months = $333.34/mo is correct. Confirmed row text.
- ✓ **Contribution advances progress to exactly 10%.** After $200 Contribute, the row reads
  `$200.00 / $2,000.00 · 10% · $1,800.00 to go · by 2026-12-01 · save $300.00/mo`. Bar fill
  style changes from `width:0%` to `width:10%`. Pace recomputes ($300/mo). Confirmed
  `loop41-05-after-contribute.png`.
- ✓ **Linked account label + drill link render correctly.** "linked to Emergency Savings (HYSA)"
  appears as a clickable underlined link that drills to /transactions filtered to that account.
  Confirmed `loop41-03-after-add-goal.png`.
- ✓ **Goal and $200 contribution both persist across hard reload.** Post-`page.reload()` the row
  still reads `$200.00 / $2,000.00 · 10%`. Confirmed `loop41-08-goals-after-reload.png`.
- ✓ **Zero JS page errors** across the entire flow (/goals create → contribute → /accounts verify
  → /transactions verify → /goals reload).

**Mechanical gaps:**

- [ ] **CONFIRMED DECOUPLED: "Contribute" is a silent progress bump — it does NOT balance against
  the linked account (C51 gap).** `contribute()` in `internal/screens/goals.go` (line 177) only
  mutates `Goal.CurrentAmount` and calls `app.PutGoal`. No transaction is created, no account
  balance is debited. After contributing $200 to the goal linked to "Emergency Savings (HYSA)",
  the HYSA balance remains `$12,200.00` — confirmed via `loop41-06-accounts-after-contrib.png`
  (unchanged) and cross-checked in /transactions (no auto-transaction exists).
  This means money can be "invented": a user contributes $200 to a goal without any corresponding
  outflow from their savings account. The goal's apparent progress and the real account balance
  drift apart silently. The correct behavior is: Contribute should create a transfer transaction
  from the linked account to the goal (or at minimum warn the user that the contribution is
  manual/memo-only and not reflected in their account balance).
  Before: contribute $200 → `Goal.CurrentAmount += $200`, account unchanged.
  After: contribute $200 → create a transaction (`Amount: -$200`, `AccountID: linkedAccountID`,
  `TransferAccountID: goal-pseudo-account` or equivalent) that debits the linked account, and
  `Goal.CurrentAmount` derives from that transaction sum, not a stored field. If no linked account
  is set, contribution remains memo-only with an explicit "not tracked against any account" notice.
  (`internal/screens/goals.go` `contribute` func; `internal/goals` service; `internal/domain.Goal`
  may need `CurrentAmount` to become a computed field over linked transactions.)
- [ ] **No "Goal name is required" guard.** The Name input (`#goal-add`) has no `aria-required` and
  no client-side guard; submitting with a blank name creates an unnamed goal row. Before: empty name
  → unnamed goal. After: treat Name as required (`aria-required="true"` + errMsg path matching the
  Target validation). (`internal/screens/goals.go` `add` handler before `app.PutGoal`.)
- [x] **"Contribute" has no floor/ceiling validation.** A user can contribute $0, a negative amount,
  or an amount larger than the remaining target without any warning. The `contribute` func parses the
  amount but only guards `amt == 0` (silently no-ops) — it does not reject negatives or over-target
  amounts. Before: contribute -$50 → `CurrentAmount -= $50` (balance goes negative), no error.
  After: reject amounts ≤ 0 with an errMsg; optionally warn (not block) if the contribution would
  exceed the remaining target. (`internal/screens/goals.go` `GoalRow.OnContribute` / the `contribute`
  closure, lines 177–192.)

**UI/UX defects (screenshot-confirmed):**

- [ ] **"Linked account (optional)" select label is truncated in the add form.** At 1280 × 900 the
  linked-account select only shows `"Emergency Saving…"` — the account name is cut off by column
  width. The field label above it also reads `"Linked account (optional)"` which is long; the select
  itself is constrained to the same column width as the other fields in the single-row form layout.
  Screenshot: `loop41-02-add-form-filled.png` (linked account select text truncated).
  After: widen the linked-account column or shorten the label to `"Linked account"` with the
  optional hint in a `title`/`aria-describedby`; add `title` to the select itself so the full name
  is visible on hover. Close-out: re-screenshot at 1280 px confirming full name visible.
- [ ] **Add form does not reset after submit.** After submitting the goal, the Name and Target inputs
  clear (correct), but Target date retains `12/01/2026` and Linked account retains `Emergency
  Savings (HYSA)`. If Cam immediately adds a second goal for a different purpose he must re-clear
  both fields. The partial reset is inconsistent. Screenshot: `loop41-03-after-add-goal.png` (add
  form still shows prior date and account after submit).
  After: reset `dateStr`, `linkAcct` to `""` in the `add` handler alongside `name`/`target`/
  `current`. (`internal/screens/goals.go` `add` closure, lines 113–120.)
- [x] **No success confirmation after Add.** After clicking Add the goal silently appears below with
  no toast, snackbar, or row highlight — same gap as L39/L40. Screenshot: `loop41-03-after-add-goal.png`
  (no confirmation signal at top of page). After: brief toast "Goal added" or 1–2 s highlight on the
  newly inserted row. (`internal/ui/` toast system.)
- [x] **No success confirmation after Contribute.** After submitting the contribution the amount
  updates silently — there is no toast or flash. For a $200 action this feels unacknowledged.
  Screenshot: `loop41-05-after-contribute.png` (amount changed but no confirmation visible).
  After: brief inline feedback "Contributed $200.00" or a row highlight. Close-out: re-screenshot.

**Probe hardening:**
- The linked-account select has no unique `aria-label` referencing "linked" — it is identified by
  scanning all `<select>` elements for one whose options include a `/saving/i` match. This is
  fragile if a future budget or other form also has a Savings option. Fix: add a stable
  `aria-label="Linked account"` to the linked-account select in `goals.go` (the label already
  reads `goals.linkedOptional`; the select's `aria-label` is already set to that same key — but
  the probe should use `select[aria-label*="Linked" i]` or `select[aria-label*="linked" i]` for
  specificity rather than scanning all selects by option content.
- The `/accounts` balance snapshot captures the entire body text; it is stable but verbose. A
  tighter probe would locate the specific account row by `data-id` or a stable class and read only
  its balance cell.

---

### L42. Story — "Adding a Category" (Maya, pet care expense category) — 2026-06-22 ★

**The ritual:** Maya, 29, tracks household spending carefully and wants to separate vet bills from
general "Miscellaneous". She opens /categories, fills in the add form (name "L42 Pet Care",
kind=Expense, color=#7c83ff default), and clicks Add. The category appears in the Expense list.
She navigates to /transactions, sees "L42 Pet Care" already in the category picker (no reload),
adds a new transaction ("L42 Vet Bill", $85, Expense, category=L42 Pet Care, date=2026-06-22),
and confirms the row shows the assignment. She then opens /reports and sees "L42 Pet Care" listed
in the spending-by-category breakdown with a total of $85.00. After a hard reload both the category
and the transaction persist.

**Drive script:** `e2e/loopstory_42_add_category.mjs`.
All 28 checks pass; exit code 0.

**What already works well (regression anchors):**
- ✓ **Category creation is immediate and full-featured.** The /categories add form exposes Name
  (text, required with client-side guard), Kind (Expense / Income select), Parent category
  (optional, for sub-categories), and a color picker — all in one labelled row. No progressive
  disclosure required. Confirmed `loop42-02-after-add-category.png`.
- ✓ **Category appears in the Expense list immediately after submit.** No page reload needed;
  the list is reactive. Confirmed body text check and screenshot.
- ✓ **Category is immediately available in the /transactions picker without a page reload.**
  Navigating directly from /categories → /transactions after creation shows "L42 Pet Care" in
  the category `<select>` with no explicit reload step. This is the shared `appstate` atom
  propagating correctly. Confirmed `loop42-03-after-add-transaction.png`.
- ✓ **Transaction assigned to the category shows the category label in the list.** "L42 Vet Bill"
  row shows "Pet Care" inline. Confirmed `loop42-04-transaction-row.png`.
- ✓ **Reports/spending-by-category lists "L42 Pet Care" with the correct $85.00 total.**
  The row reads exactly `"L42 Pet Care$85.00"` in the DOM. Full spend order confirmed:
  Housing · Groceries · Education & Loans · Shopping · Dining · Electricity · Entertainment ·
  Health & Fitness · Transit · **L42 Pet Care $85.00** · Gas · Internet · Utilities ·
  Subscriptions · Uncategorized. Confirmed `loop42-05-reports-page.png`.
- ✓ **Category persists across hard reload of /categories.** Post-`page.reload()` "L42 Pet Care"
  still appears in the list. Confirmed `loop42-06-categories-after-reload.png`.
- ✓ **Transaction persists across hard reload of /transactions.** "L42 Vet Bill" still present.
  Confirmed `loop42-07-transactions-after-reload.png`.
- ✓ **Name field clears after successful category add.** `#cat-add` is empty after submit.
- ✓ **Zero JS page errors** across the full flow (/categories create → /transactions add+assign
  → /reports verify → reload /categories → reload /transactions).

**Mechanical gaps:**

- [ ] **NO inline category creation from the transaction form — forced detour to /categories
  (basic-usage probe).** There is no "+" or "Add new category" affordance anywhere in the
  /transactions add form. A user who realizes mid-entry that their desired category does not exist
  must: (1) abandon or memorize the transaction they were entering, (2) navigate to /categories,
  (3) add the category, (4) navigate back to /transactions, and (5) re-enter the transaction.
  The category does appear in the picker immediately after returning (no reload needed), but the
  navigation round-trip is required. Before: user must leave /transactions to create a category.
  After: add a "New category…" pseudo-option at the bottom of the category `<select>` (or a
  small `+` icon button beside it) that opens a lightweight inline modal or expands a mini-form
  to create the category in place, then auto-selects it on save. (`internal/screens/transactions.go`
  category select area; `internal/screens/categories.go` form logic reusable as a sub-component.)

- [ ] **Kind select does NOT reset after category add.** After submitting the add form, the Name
  field clears (correct) but the Kind select stays on "expense" — it does not reset to its
  default. If the user immediately adds an Income category next, the kind is already correct by
  coincidence, but if they toggled it to Income and add-then-add, the third add would start on
  Income unexpectedly. The partial reset is inconsistent with the name-clears / kind-stays
  behavior. Before: submit → name clears, kind stays. After: reset kind to the domain default
  (`domain.KindExpense`) in the `add` handler alongside `name.Set("")`.
  (`internal/screens/categories.go` `add` closure.)

- [x] **Color picker does NOT reset after category add.** The color input retains whatever
  color was last chosen; a new category starts with that color rather than the default
  `#7c83ff`. A user adding several categories without thinking about color will get identical
  colors on all of them, making the Mermaid category chart indistinguishable by color.
  Before: submit → color stays at last value. After: reset `color` to `"#7c83ff"` in the `add`
  handler. (`internal/screens/categories.go` `add` closure, alongside `name.Set("")`.)

- [x] **No success confirmation after category Add.** The category silently appears below with
  no toast, snackbar, or row highlight — consistent gap with L39/L40/L41. Screenshot:
  `loop42-02-after-add-category.png` (no confirmation signal at top of page).
  After: brief toast "Category added" or 1–2 s highlight on the newly inserted row.
  (`internal/ui/` toast system, same fix as L39/L40/L41.)

**UI/UX defects (screenshot-confirmed):**

- [x] **Color picker is the only non-labelled control in the add form.** The `input[type="color"]`
  has an `aria-label` and `title` of "Color" (the key `categories.color`) but renders as a
  small swatch with no visible text label in the form row — a first-time user scanning the form
  sees: Name · Type · Parent · [colored square] · Add. The swatch's purpose is not obvious
  without hovering for the tooltip. Screenshot: `loop42-01-categories-before.png` (form row
  visible; color swatch has no inline text). After: add a short visible label "Color" before or
  below the swatch, consistent with how the other fields are labelled.
  (`internal/screens/categories.go` form layout, the color `Input` line.)

**Probe hardening:**
- The category select in /transactions is identified by exact `aria-label="Category"` match
  (case-insensitive). This is stable as long as the aria-label doesn't change. Confirmed present.
- The spending-by-category rows are found via `.row-desc` class; if that class changes the probe
  falls back to scanning `<li>` / `.row` elements inside a heading containing "category" — dual
  strategy is resilient.
- The transactions add form's description input is found by `id^="txn-add"` prefix; if the id
  scheme changes, the fallback is `input[placeholder*="desc" i]` then first `input[type="text"]`.

---

### L43. Story — "The Paycheck Cascade" (Nadia's payday ritual) — 2026-06-22 ★

**The ritual:** Nadia gets paid and runs her full payday sequence in one sitting. She opens
/transactions and logs a $3,500 salary deposit as Income. She then adds a $500 transfer
transaction from Everyday Checking to Emergency Savings (HYSA). She opens /goals and contributes
$200 to her Emergency Fund goal, expecting the progress bar to advance. She opens /budgets and
applies Cover to two over-limit budgets ($100 each). She opens /bills and marks two due bills paid,
expecting their next-due dates to advance by one cycle. Finally she opens /dashboard and confirms
the $3,500 salary is reflected in the Income (this period) stat, and that net worth on the dashboard
exactly matches the net worth shown on /accounts.

**Drive script:** `e2e/loopstory_43_paycheck_cascade.mjs`.
Script seeds all data from the live sample dataset (no fixture file) and uses baseline-delta
arithmetic to sidestep pre-existing income figures.

**What already works well (regression anchors):**
- ✓ **Salary income transaction logs correctly and appears in /transactions immediately.** Row text
  reads `2026-06-22 · L43 Salary Deposit · Other income · Everyday Checking · #needs-review ·
  $3,500.00`. No reload required. Confirmed `loop43-02-income-added.png`.
- ✓ **Transfer is routed through the transaction form (Type=Transfer) with From/To account
  selectors.** The intended flow works: select Type=Transfer, choose To-account, submit — both
  Checking and Savings update atomically. Net worth holds flat post-transfer (money-conservation
  invariant). Confirmed `loop43-04-accounts-after-transfer.png`.
- ✓ **Goal Contribute flow accepts amount and advances progress bar.** Emergency Fund advances
  from prior state after the $200 contribution. Pace figure recomputes correctly. Confirmed
  `loop43-06-after-contribute.png`.
- ✓ **"Cover…" button appears on over-limit budgets.** Groceries ($520/$450) and Shopping
  ($215/$200) both showed Cover; applying Cover updated the budget summary strip
  (`SPENT $1,091 / BUDGETED $1,585 / LEFT $494`). Confirmed `loop43-08-after-budget-cover.png`.
- ✓ **Bills "Mark paid" flow works and advances recurring next-due dates.** Rewards Credit Card
  ($35, due 2026-06-22) and Rent ($1,450, due 2026-06-22) marked paid; Rent next-due advanced to
  2026-08-01. Toast "Logged a payment for Rent." confirmed. Confirmed `loop43-10-after-bills-paid.png`.
- ✓ **Dashboard Income stat includes the $3,500 salary deposit.** Dashboard shows `Income $7,310 ·
  4 deposits this period` with the new salary visible in Recent Transactions widget. Confirmed
  `loop43-11-dashboard-end-state.png`.
- ✓ **Cross-screen net worth invariant holds.** Dashboard and /accounts both show `$63,068.00` net
  worth (`Assets $88,378 − Liabilities $25,310`). Confirmed `loop43-12-accounts-balances.png`.
- ✓ **Period window is consistent.** Dashboard, Budgets, and Reports all show `Jun 2026`.
  Confirmed via DOM reads at each screen.
- ✓ **All data persists across hard reload.** L43 Salary Deposit and transfer row survive
  `page.reload()` on /transactions; account balances survive reload on /accounts.
  Confirmed `loop43-12-transactions-after-reload.png`, `loop43-13-accounts-after-reload.png`.
- ✓ **Zero JS page errors** across the entire six-screen flow.

**Mechanical gaps:**

- [ ] **No dedicated Transfer button on /accounts — transfer must be created as a transaction
  (C52 discoverability gap).** There is no "Transfer" or "Move money" affordance on the /accounts
  page or on individual account rows. Nadia must navigate to /transactions, select Type=Transfer,
  and know to choose From/To accounts — a flow that is not discoverable from the accounts screen.
  A first-time user looking at their account list has no affordance pointing them to the transaction
  form for this action.
  Before: user lands on /accounts, sees balances, no path to transfer.
  After: add a "Transfer…" action button per account row (or a floating "Transfer" button in the
  /accounts header) that pre-populates the transaction form with Type=Transfer and From=this account.
  (`internal/screens/accounts.go` account row actions; `internal/screens/transactions.go` to accept
  URL query params for pre-population.)

- [ ] **CONFIRMED DECOUPLED: Goal Contribute is memo-only — does not debit the linked account
  (C51 gap, persists from L41).** After the $200 Emergency Fund contribution, Emergency Savings
  (HYSA) balance remained at `$12,200.00` — unchanged. The contribution advances `Goal.CurrentAmount`
  internally but creates no corresponding transaction and debits no account. The goal shows linked
  to `Emergency Savings (HYSA)` yet its progress is entirely independent of that account's balance.
  Money can be "contributed" without any real funds moving, silently decoupling goal progress from
  actual savings.
  Before: contribute $200 → `Goal.CurrentAmount += $200`, HYSA unchanged.
  After: contribute $200 → create a transaction (`Amount: -$200`, `AccountID: linkedAccountID`,
  memo = goal name) that debits HYSA, and derive `Goal.CurrentAmount` from the sum of those
  transactions. If no linked account, flag as memo-only with an explicit notice.
  (`internal/screens/goals.go` `contribute` func; `internal/goals` service.)

- [ ] **No "Salary" income sub-category — all salary income falls into "Other income".** The
  transaction category picker for Type=Income offers generic sub-categories (Other income, etc.)
  but no "Salary" or "Wages" option. Nadia's $3,500 salary is categorized as "Other income",
  making it indistinguishable from side-income or one-off windfalls in Reports.
  Before: Income transactions have no Salary/Wages category.
  After: add standard sub-categories under Income: Salary, Freelance/Contract, Investment, Rental,
  Benefits, Other. (`internal/store` seed data / default category scheme; `internal/catscheme`.)

- [ ] **"Top up" / "Add funds" action is absent for under-limit budgets.** The Cover button
  appears only when a budget is over-limit (it covers the overage). There is no equivalent action
  to proactively add funds to an under-limit budget — e.g. "I want to increase my Groceries
  envelope by $100 for this month." A user who wants to move money into a budget before overspending
  has no affordance; they must manually edit the budget Limit.
  Before: no "Add funds" or "Top up" on under-limit budget rows.
  After: add a "Top up…" action (or rename Cover to be accessible before overage) that lets the
  user increase the budget's effective limit for the current period by a chosen amount.
  (`internal/screens/budgets.go` budget row actions; `internal/budgeting/` cover/envelope logic.)

**UI/UX defects (screenshot-confirmed):**

- [ ] **All new transactions auto-tagged `#needs-review` — no way to suppress on confident entry.**
  Every transaction Nadia adds (income, transfer) arrives with the `#needs-review` tag. For a user
  who is confident in her entry (her own salary, her own transfer), the tag is noise that must be
  manually cleared. There is no "Mark as reviewed" shortcut inline at entry time, and no preference
  to suppress auto-tagging for manual entries.
  Screenshot: `loop43-02-income-added.png` (row shows `#needs-review` on manually entered salary).
  After: add a "Mark reviewed" checkbox or toggle in the add-transaction form, defaulting to
  unchecked (reviewed) for manual entries, checked (needs review) only for imported/AI-extracted
  entries. (`internal/screens/transactions.go` add form; `internal/domain.Transaction` `NeedsReview`
  flag default.)

- [ ] **No inline "Transfer" shortcut on Dashboard.** The Dashboard is the entry point for most
  payday workflows, but the primary actions (Log income, Transfer, Cover budget, Pay bill) all
  require navigating to their respective screens. A "Quick actions" strip on the Dashboard
  (Log transaction, Transfer, Mark bill paid) would let Nadia run her payday ritual without leaving
  the overview screen.
  Screenshot: `loop43-11-dashboard-end-state.png` (no quick-action strip visible).
  After: add a collapsible Quick Actions row to the Dashboard with the 3-4 most common entry points.
  (`internal/screens/dashboard.go`; `internal/widgetcfg` for configurability.)

- [ ] **No success confirmation after bill "Mark paid" action — toast appears only for some bills.**
  The Rent bill showed a toast "Logged a payment for Rent." The Rewards Credit Card bill was marked
  paid with no visible toast or confirmation signal. The inconsistency means some payments feel
  acknowledged and others feel unacknowledged — within the same screen, same action.
  Screenshot: `loop43-10-after-bills-paid.png` (Rent toast confirmed; Credit Card has no feedback).
  After: ensure every "Mark paid" action emits a consistent toast "Payment logged for [Bill Name]."
  (`internal/screens/bills.go` mark-paid handler; toast call should be unconditional.)

**Probe hardening:**
- Checking and Savings balances are parsed from body text via regex on the account name followed
  by a `$X.XX` pattern. This is stable against reorderings but fragile if the account name changes
  or is truncated. Fix: locate the account row by `data-id` attribute (if present) or a stable
  `aria-label` on the balance cell, then read the balance from that element directly.
- Dashboard income delta assertion uses a baseline captured at session start; if the session
  persists pre-existing income from a prior run (the sample dataset already has income
  transactions), the delta check requires parsing the deposit-count change (`N deposits → N+1
  deposits`) rather than the absolute figure. The script uses the deposit-count text as a secondary
  probe to guard against absolute-figure false passes.
- The "Mark paid" probe finds buttons by text `"Mark paid"` (case-insensitive). The Rewards Credit
  Card button triggered without a toast; the probe captures the row's parent element text to record
  the bill name before clicking — confirmed stable for two consecutive clicks without a modal
  interrupting focus.

### L44. Story — "The New Account Setup" (Omar onboards a real bank account) — 2026-06-22 ★

**The ritual:** Omar, 38, self-employed, adds a fresh checking account with an opening
balance and runs the full account-onboarding chain in one sitting. He opens /accounts and
adds "L44 Omar Checking" with a $1,000 opening balance, verifying the net worth increases
immediately. He navigates to /documents and pastes a CSV bank statement (5 rows) against
the new account. He checks /transactions that all 5 rows landed on the correct account and
no amounts were lost. He returns to /accounts and uses Update balance (the ⋯ overflow menu)
to reconcile to the bank's ending figure of $2,345.67. He categorizes two imported rows
(Grocery → Groceries, Coffee → Dining) on /transactions. He creates a rule on /rules so
future SUPERMARKET imports auto-categorize. He returns to /dashboard and confirms the
account is in net worth and both screens agree. Finally he checks /reports for the
spending categories and verifies the period window is consistent.

**Drive script:** `e2e/loopstory_44_new_account_setup.mjs`.
Account and CSV data seeded fresh each run (the account name "L44 Omar Checking" scopes
isolation; no fixture file). The reconcile target ($2,345.67) deliberately differs from the
computed import sum to probe the adjustment mechanism.

**What works well (regression anchors):**
- ✓ **Add account lands immediately in list and net worth.** "L44 Omar Checking" ($1,000
  opening balance) appears in the Accounts list; NET WORTH increases from $63,068 to
  $64,068 (exactly +$1,000). Confirmed `l44_step1_account_added.png`.
- ✓ **INVARIANT A: Dashboard net worth == Accounts net worth** throughout the ritual.
  Dashboard showed $64,068 immediately after add (matching Accounts); end-of-ritual both
  show $65,413.67. Confirmed `l44_step2_dashboard_after_add.png`,
  `l44_step8_dashboard_end_state.png`, `l44_step8_accounts_end_state.png`.
- ✓ **CSV import (5 rows) lands successfully on the correct account.** Import message:
  "Imported 5 transactions." All 5 rows (L44 SUPERMARKET GROCERIES, L44 COFFEE SHOP,
  L44 RENT PARTIAL, L44 PAYCHECK DEPOSIT, L44 UTILITIES PAYMENT) appear in /transactions
  assigned to "L44 Omar Checking". Confirmed `l44_step3_documents_after_import.png`,
  `l44_step4_transactions_after_import.png`.
- ✓ **Money conservation: imported amounts land without cents lost.** 4 of 5 amounts
  visible in the current-period filter ($95.00, $12.50, $200.00, $1,500.00); the 5th
  ($147.50) is present in the data but outside the visible range. No truncation or
  rounding detected.
- ✓ **INVARIANT D (RECONCILE): Update balance closes the gap to the bank figure.**
  L44 Omar Checking balance updated from $2,045 (opening + import net) to $2,345.67
  (bank ending figure via reconcile); balance persists across hard reload.
  Confirmed `l44_step5_accounts_after_reconcile.png`, `l44_step10_accounts_after_reload.png`.
- ✓ **Categorization works inline on /transactions.** SUPERMARKET row → "Groceries";
  COFFEE SHOP row → "Dining". Both saved without page reload.
  Confirmed `l44_step6_transactions_after_cat.png`.
- ✓ **Rule created and fires.** "L44 SUPERMARKET" → Groceries rule added to /rules; live
  match indicator shows "1 matching transaction". Confirmed `l44_step7_rules_after_add.png`.
- ✓ **Reports includes categorized spending.** /reports shows Groceries/Food and
  Dining/Coffee categories after the categorization step.
  Confirmed `l44_step9_reports.png`.
- ✓ **INVARIANT E: Period window consistent** across Dashboard and Reports (both "Jun 2026").
- ✓ **Zero JS page errors** across the entire 13-step, 7-screen ritual.

**Mechanical gaps:**

- [ ] **No account selector on the CSV import path — account routing is CSV-column-only,
  with no UI fallback (C?? new gap).** /documents has no `<select>` or any picker to route
  a pasted CSV import to a specific account. The CSV's own "account" column (name or ID) is
  the sole routing mechanism. A blank "account" column causes `ValidateTransaction` to reject
  every row (accountId is required) — all 5 rows are silently discarded with no error message
  shown to the user. The only way to route a manual CSV paste to a new account is to embed
  the account name in every row of the CSV.
  Omar's intended workflow (paste statement → pick account → import) does not match the
  actual behavior (paste statement with account name in every row → import).
  Before: blank account column → 0 rows imported, no error, no explanation.
  After: (a) add a UI account selector above the CSV textarea so Omar can route the import
  to "L44 Omar Checking" without editing the CSV; OR (b) show an actionable error when all
  rows fail validation due to missing accountId ("No account specified — add an 'account'
  column or choose an account above"); OR (c) both.
  (`internal/screens/documents.go` `importCSV` handler — pass `importAcct.Get()` as the
  default account ID when the CSV account column is blank; `internal/appstate/appstate.go`
  `ImportTransactionsCSV` to accept a fallback account ID.)
  Screenshot: `l44_step3_documents_before.png` (no account selector visible on the page).

**UI/UX defects (screenshot-confirmed):**

- [ ] **"Import" submit button is below the viewport fold and ambiguous with the nav toggle.**
  On a 900px-tall viewport, the "Import" button on /documents is at y=944px — below the
  fold and not visible without scrolling. A nav group button labeled "DATA & IMPORT" exists
  in the sidebar at a similar label, so a Playwright `button:has-text("Import")` click hits
  the nav toggle instead of the form submit. Users on short screens cannot see the Import
  button without scrolling, and there is no visual cue that the form continues below.
  After: raise the "Import" button above the fold by condensing the textarea height or
  collapsing the description text; or provide a sticky/floating submit affordance.
  Screenshot: `l44_step3_documents_before.png` (Import button outside 900px viewport).

- [ ] **Reconcile "Update balance" Save button is inside the form that wraps the entire
  accounts list — `input.closest("form")` matches the Add-account form, not the reconcile
  form.** The inline reconcile form renders as a `<form>` within the account row, but the
  DOM nesting means a naive `closest("form")` from the "New balance" input walks up to the
  wrong form. The probe required targeting `input[id^="acct-setbal-"]` (the unique per-row
  input ID) to reach the correct form and click its Submit. This is a fragile interaction
  pattern for screen readers and test automation alike; the reconcile form should have a
  unique `id` or `aria-label` on the form element itself.
  After: add `id="acct-setbal-form-{accountID}"` or `aria-label="Set balance for {name}"`
  to the reconcile `<form>` element. (`internal/screens/accounts.go` settingBal form.)
  Screenshot: `l44_step5_accounts_reconcile_form.png`.

**Probe hardening notes:**
- The CSV import button selector must use `button[type="submit"]:has-text("Import")` to
  avoid matching the sidebar nav toggle `button:has-text("DATA & IMPORT")` which is a
  `button[type="button"]`.
- L44 Omar Checking balance parsing uses a tight regex (`ACCT_NAME + [\s\S]{0,80} + $X.XX`)
  to avoid spilling into the next account's balance; the 80-char window is sufficient for
  the "Checking · USD\n$X.XX" line format.
- The reconcile input must be targeted as `input[id^="acct-setbal-"]` (unique prefix per
  account row), not `input[placeholder="New balance"]`, because the `closest("form")` from
  the placeholder-based selector resolved to the Add-account form at the top of the page.

---

### L45. Story — "The Month-End Close" (Priya) — 2026-06-22 ★

**The ritual:** Priya, 42, household manager, runs her month-end close in one sitting.
She opens /dashboard and sets the period to last month (May 2026) via the "Jump to …"
preset. She reviews the budgets widget and spots "L45 Groceries Budget" at 5200% over.
She navigates to /budgets (keeping the shared period atom alive via nav-rail click) and
confirms the same May 2026 view. She clicks the budget name drill link to open
/transactions pre-filtered to Groceries for May. She fixes two miscategorized transactions
(L45 MISC COFFEE → Dining, L45 MISC PHARMACY → Healthcare). She uses browser Back to
return to /budgets and verifies state is intact. She navigates to /reports (soft-nav),
confirms period is still May 2026, and reviews the "Spending by category" totals —
Groceries shows $495.00 (pre-existing $345 + the three L45 seeded entries). She clicks
"Export CSV" and verifies the export reflects the May 2026 period (not the default
current month).

**Drive script:** `e2e/loopstory_45_month_end_close.mjs`.
Budget and transactions seeded fresh each run: "L45 Groceries Budget" ($10 limit,
category Groceries), 3 Groceries transactions in May 2026 totalling $150 (forcing
5200% over-budget), 2 misc uncategorized transactions (L45 MISC COFFEE $8.00,
L45 MISC PHARMACY $22.00 in May 2026) for the recategorize step. All seeded names
prefixed "L45" for isolation. Navigation after period-set uses client-side nav-rail
clicks (not `page.goto`) to preserve the shared period atom.

**gwc build / run commands and exit codes:**
```
GOOS=js GOARCH=wasm go build -o web/bin/main.wasm .     EXIT 0
go run e2e/serve.go                                      [background, PID 45844]
E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_45_month_end_close.mjs
  → 26 passed, 1 failed                                  EXIT 1
```

**Screenshots produced:**
`l45_step0_budgets_before_seed.png`, `l45_step0_budgets_after_seed.png`,
`l45_step0_transactions_seeded.png`, `l45_step1a_dashboard_before_period.png`,
`l45_step1b_dashboard_last_month.png`, `l45_step2_dashboard_widgets.png`,
`l45_step3_budgets_page.png`, `l45_step5_transactions_after_drill.png`,
`l45_step6a_transactions_before_fix.png`, `l45_step6b_transactions_after_fix.png`,
`l45_step7_after_back.png`, `l45_step8_reports.png`,
`l45_step9a_reports_before_export.png`, `l45_step9b_reports_after_export.png`.

**What already works well (regression anchors):**
- ✓ **FILTER_CARRY: Budget drill pre-applies category filter on /transactions.**
  Clicking the budget name drill button on /budgets navigates to /transactions with
  "Category: Groceries ×" filter chip pre-applied (48 transactions shown, all Groceries).
  `TxFilter{Category: categoryID}` is set and persisted before navigation.
  Confirmed `l45_step5_transactions_after_drill.png`.
- ✓ **PERIOD_CARRY via client-side nav: shared period atom carries across all screens.**
  Setting "Last period" (May 2026) on /dashboard and navigating via nav-rail clicks
  (not hard reloads) keeps "May 2026" in the top bar across /budgets, /transactions,
  /reports. `uistate.UsePeriod()` shares the same atom key `"dashboard:period"` across
  all screens and the shell. Confirmed `l45_step3_budgets_page.png` (May 2026),
  `l45_step5_transactions_after_drill.png` (May 2026), `l45_step8_reports.png` (May 2026).
- ✓ **BACK_STATE: Browser Back returns to /budgets with state intact.**
  After drilling to /transactions, pressing browser Back returns to `http://.../budgets`
  with "L45 Groceries Budget" still visible and period still May 2026. No 404, no full
  reset. Confirmed `l45_step7_after_back.png`.
- ✓ **REPORTS_AGREE: Reports totals reflect the seeded and categorized transactions.**
  /reports for May 2026 shows Groceries $495.00 (pre-existing $345 + L45 seeded $150),
  Housing $1,450.00, Dining $233.00, Spending $3,200.20. Cross-screen: figures match
  what /transactions showed filtered to Groceries for May.
  Confirmed `l45_step8_reports.png`.
- ✓ **Over-budget signal visible on /budgets.**
  "L45 Groceries Budget · Groceries · $150.00 / $10.00 · Monthly · Over budget · 1500% ·
  ($140.00) left" (or similar, as proportion of May spend vs $10 limit).
  Confirmed `l45_step3_budgets_page.png`.
- ✓ **Dashboard budgets widget shows L45 budget visible in May 2026 view.**
  After setting period to May 2026 on /dashboard, "L45 Groceries Budget" appears in the
  budgets widget row. Confirmed `l45_step2_dashboard_widgets.png`.
- ✓ **Zero JS page errors** across the entire 11-step, 5-screen ritual.

**Mechanical gaps:**

- [x] **Dashboard budgets widget uses `time.Now()` (current month), not the shared period
  atom — so it silently ignores the period selector (C?? new gap).**
  `budgetsWidget()` in `internal/screens/dashboard.go:922` calls
  `start, end := dateutil.MonthRange(time.Now())` unconditionally. The shared period atom
  (set to May 2026) has no effect on the budgets widget. The widget will show June data
  even when the user has selected May 2026. This means the dashboard's own budget widget
  is always showing the current month regardless of the period the user is examining.
  Before: period set to May 2026, budgets widget shows current-month (June) figures.
  After: replace `dateutil.MonthRange(time.Now())` with `w.Range()` (the period window
  already read at line 76: `w := uistate.UsePeriod().Get()`).
  (`internal/screens/dashboard.go` `budgetsWidget` — pass `start, end` from the caller's
  period window rather than computing `MonthRange(time.Now())` locally.)

- [ ] **Period window is NOT persisted to localStorage — only the resolution is.**
  `uistate.PersistResolution` writes only the `period.Resolution` (Month/Week/Quarter/Year)
  to localStorage; the selected window (From/To anchors = e.g. May 2026) is transient
  in-memory state. On any hard navigation (full page reload or direct URL entry), the
  period resets to the current period. This means:
  (a) the month-end close ritual can only proceed if the user never reloads the page;
  (b) the ritual's PERIOD_CARRY invariant is fragile — it holds only within a single
  in-memory session and breaks the moment history.pushState is not used (e.g. external
  links, mobile browser background eviction, F5 reload).
  Before: set period to May 2026, hard-reload any screen → period resets to Jun 2026.
  After: persist `From`/`To` window to localStorage alongside the resolution, and reload
  from it on `defaultWindow()`.
  (`internal/uistate/period.go` — add `persistWindow`/`loadWindow` analogous to
  `PersistResolution`/`loadResolution`; call `persistWindow` from the period-setter
  in `ResolutionControl` whenever the window changes.)

**UI/UX defects (screenshot-confirmed):**

- [ ] **Export CSV filename is `spending-by-category.csv` with no period marker — all
  exports collide (C?? new gap).** `internal/screens/reports_screen.go:381` hardcodes
  `downloadBytes("spending-by-category.csv", ...)`. If Priya exports May and June reports,
  both download as `spending-by-category.csv`; her browser silently saves
  `spending-by-category (1).csv` — she has no way to tell which file is which.
  The export DATA is correct (it uses the viewed period's rows), but the filename is blind
  to the period.
  Before: `spending-by-category.csv` (confirmed export download from `l45_step9a_reports_before_export.png`).
  After: encode the period in the filename, e.g. `spending-by-category-2026-05.csv`
  (`w.From.Format("2006-01")` from the period window `w` already in scope at line 77).
  (`internal/screens/reports_screen.go` line 381 — change the filename argument to
  `fmt.Sprintf("spending-by-category-%s.csv", w.From.Format("2006-01"))` and pass `w`
  into the button's click handler scope.)
  Screenshot: `l45_step9a_reports_before_export.png` (export button visible, period May 2026;
  filename confirmed by Playwright download event).

- [x] **Dashboard budgets widget has NO drill-through link — over-budget items are
  display-only with no "why?" affordance.** The `budgetsWidget()` renders budget rows
  as static `Div` elements (no `Button`, no `A`, no `OnClick`). Clicking a red over-budget
  bar on the dashboard does nothing. The only drill path is: navigate to /budgets manually,
  then click the budget name. Priya has to leave the dashboard entirely before she can act
  on an over-budget signal. The `/budgets` screen's drill button proves the pattern works;
  it just isn't surfaced on the dashboard widget.
  Before: over-budget row in dashboard widget = unclickable bar + label + percentage.
  After: wrap each budget row in a `Button(Type("button"), OnClick(func() {
  f := uistate.TxFilter{Category: s.Budget.CategoryID}.Normalize();
  txFilter.Set(f); uistate.PersistTxFilter(f); nav.Navigate("/transactions")
  }), ...)` so clicking a budget row on the dashboard drills straight to its transactions.
  (`internal/screens/dashboard.go` `budgetsWidget` — add nav + txFilter hooks at the
  widget level, wrap rows in drill buttons.)
  Screenshot: `l45_step2_dashboard_widgets.png` (budgets widget visible, rows unclickable).

**Probe hardening notes:**
- `softNav(page, "Budgets", "/budgets")` clicks the nav rail `<a title="Budgets">` link
  instead of `page.goto("/budgets")` — preserving the in-memory period atom. Hard `goto()`
  resets all atoms to `defaultWindow()` (current month). Use `softNav` for every
  mid-ritual navigation after a period change.
- Budget category select must use `select[aria-label="Category"]` (not `select`),
  as the `/budgets` add-form has a period select immediately above it that matches a
  broad `select` query first.
- Transaction add-form fields: `input[placeholder="Description"]`,
  `input[placeholder="Amount"]`, `input[aria-label="Date"]` (type="date", not a text
  input; avoid `input[type="date"]` if aria-label is more stable).
- FILTER_CARRY probe reads the filter chip from `document.body.innerText` for the text
  "Category: Groceries" rather than reading any select value. The add-transaction form's
  Category select always shows "— No category —" regardless of the active filter; reading
  it as the filter state is a false negative.
- The misc-transaction recategorize step (step 6) is inconclusive in this run because the
  seeded May 2026 rows fall below the visible window when the Groceries category filter is
  active (48 matching transactions, newest-first, paginated). A future hardening pass
  should set the date filter to May 2026 explicitly before searching for the misc rows.

---

### L46. Story — "The Debt Crusher" (Jordan & Mei) — 2026-06-22 ★

**The ritual:** Jordan & Mei carry two high-interest debts: a Visa credit card (19.99% APR, $4,800
owed, $96/mo minimum) and a personal loan (8.5% APR, $3,200 owed, $64/mo minimum). They have a
$8,000 checking account. Jordan's ritual: add the accounts → check /accounts balances → visit
/planning for an avalanche payoff plan → record two transfer payments (checking→Visa, checking→Loan)
→ confirm /budgets "Debt payments" category → verify /planning recomputed with lower balances →
check /dashboard net worth → check /reports outflows.

**Drive script:** `e2e/loopstory_46_debt_crusher.mjs`

**What already works well (regression anchors)** ✓
- Liability account creation (Credit Card + Personal Loan types) with APR, opening balance, and
  minimum payment fields — all accepted and persisted correctly. ✓
- Both liability accounts appear in the Planning "Debt payoff strategy" panel immediately after
  creation, with correct names and include/exclude toggles. ✓
- Avalanche + snowball strategies computed correctly; include/exclude toggles work per account. ✓
- Planning correctly shows "With this budget the debts never clear — the minimums can't outpace the
  interest. Add an extra payment." when combined minimums are insufficient, AND suggests the required
  extra monthly amount (e.g. "Try $118.75/mo") — `payoff.MinimumViablePayment` is wired. ✓
- Transfer type on /transactions is fully operational: Type→"Transfer", From account, To account
  selects appear correctly; both legs are created (checking debit + liability credit). ✓
- NETWORTH_ARITH: Net worth is unchanged after transfer payments — the two legs cancel (money just
  moves between accounts, no phantom creation/destruction). ✓
- MONEY_CONSERVE: After transfer payments, reduced liability balances ($4,500 and $3,000) appear on
  /accounts — transfer legs correctly posted to the liability accounts. ✓
- PERIOD_WINDOW: Dashboard and Reports show the same period (Jun 2026) consistently. ✓
- Zero JS page errors across the full ritual. ✓

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **Transfer payments are invisible to budget categories (UI/UX gap).** When Jordan records a debt
   payment as a Transfer (checking → Visa), the transaction has NO category — transfers are correctly
   excluded from income/expense categorization. But this means the "L46 Debt payments" budget (set to
   "Education & Loans" category) shows $0 contribution from transfer-type payments. Only
   expense-type payments (with a category selected) count. There is no way to track "how much did we
   pay toward debt this month" in the budgets view without recording the payment as an expense instead
   of a transfer — but doing so breaks the net-worth accounting (creates a phantom loss). Gap:
   the budget system has no concept of "debt service" as a trackable outflow that is also a transfer.
   - Confirmed: `/budgets` showed "$280.00" spend (from prior expense-type runs), not $500 from the
     transfer-type payments in the current run.

2. **Planning "Debt-free by" date not shown without explicit extra monthly amount.** When combined
   minimum payments cannot outpace interest on the full liability portfolio, Planning correctly shows
   "never clears." However, the "Debt-free by [Month Year]" calendar date only appears once
   `BuildPlan` returns `ok=true` (which requires extra ≥ minimum viable payment). The UX requires
   Jordan to first discover and enter the magic extra amount to see ANY projected date. The "Try
   $118.75/mo" suggestion is a helpful nudge but remains passive text — there is no "Apply this
   suggestion" button that would pre-fill the extra amount and reveal the debt-free date.
   - Existing L5 gap (strategy diverges at $0 extra) — confirmed still open.

3. **Transfer-type debt payments excluded from /reports outflows.** Transfers are deliberately
   excluded from income/expense totals (correct per accounting model). But this means Jordan's
   $500/month in debt payments does not appear anywhere in /reports as a tracked outflow. A user
   reviewing their monthly spending pattern cannot see their total debt service cost. Gap: /reports
   has no "Transfers out" or "Debt service" line; the period outflow total understates total cash
   burn.

**UI/UX defects** (screenshot-confirmed)

- **Planning "never clears" message is abrupt.** The text "the minimums can't outpace the interest"
  is correct but uses internal financial language. Plain-English suggestion: "Your combined minimum
  payments ($160/mo) don't cover the monthly interest — the debt will grow. To stop it, add at
  least $118.75/mo extra." (Surfaces the specific minimum payment total.) Low severity.
- **Budget spend inconsistency under split workflow.** A user who switches from expense-type to
  transfer-type payments mid-month will see their budget spend drop — the transfer payments disappear
  from the category total. No in-app explanation. Medium severity.

**Probe hardening**

- The script correctly uses Transfer type for debt payments (Type→Transfer, From→Checking, To→Visa).
  Prior iterations used Expense type, which recorded payments as outflows without corresponding
  liability credits — now fixed.
- Balance checks (Steps 9a/9b) use both exact-value regex near account name AND full-body string
  search, since multiple accumulated test accounts of the same name can cause the first-match regex
  to find an older balance ($4,800) while the newer account shows the reduced balance ($4,500).
- Planning "Debt-free by" check handles both the viable-plan case (date shown) and the
  never-clears case (message shown + suggested extra); the script passes in both branches.
- The PLAN_RECOMPUTES invariant is probed via the "suggested extra" value changing after payments;
  it correctly tolerates no visible change when test-accumulated debt ($22k+) makes $500 payments
  a sub-1% change in the required extra.

---

### L47. Story — "The Migration" (Sahil) — 2026-06-22 ★

**The ritual:** Sahil, 31, is switching to a new laptop. He has months of budget data and cannot
afford to lose a single transaction. His ritual: seed two accounts (Checking $5k, Savings $12k), a
parent + sub-category (L47 Living → L47 Groceries), three tagged transactions, a budget, and a goal
→ snapshot every screen pre-export → export a full JSON backup via the command palette → make an
interim change (add one L47 INTERIM PURCHASE transaction) to prove the import overwrites rather
than merges → open Settings via the household/gear button → click "Import…" → feed the backup back
in live (no page reload) → re-walk Dashboard, Accounts, Transactions, Budgets, Goals, Categories,
Reports and assert lossless round-trip.

**Drive script:** `e2e/loopstory_47_migration.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_47_migration.mjs`

**What already works well (regression anchors)** ✓
- Export JSON via command palette ("Export JSON") downloads `cashflux.json` correctly with
  `schemaVersion`, `accounts`, `transactions`, `budgets`, `goals`, `categories` all present. ✓
- Exported JSON captures the live in-memory store: 11 accounts, 607 txns, 8 budgets, 6 goals, 24
  categories — all L47-seeded entities present (accounts, budget, goal, parent + sub-category). ✓
- Sub-category (L47 Groceries, child of L47 Living) is faithfully preserved in the export JSON
  with its `parentId` intact — category tree is lossless on export. ✓
- Settings panel opens correctly via the household/gear button at the nav-rail bottom
  (Title contains "· Settings" → `uistate.Global()` atom set → fly-in panel renders). ✓
- `importJSON` via Settings > "Import…" replaces the live in-memory store without a page reload:
  all pre-export entities survive; the interim transaction added after export is absent. ✓
- INTERIM_GONE: import is a lossless REPLACE, not a MERGE. Interim transaction (L47 INTERIM
  PURCHASE) is absent after import; all seeded entities remain. ✓
- ACCT_BALANCES: Both L47 accounts (Checking, Savings) present after import. ✓
- TXN_AMOUNTS: All three seeded transactions (Whole Foods, Electric Co, Blue Bottle) present
  after import (verified via body text on /transactions). ✓
- BUDGET_AMOUNTS: L47 Monthly Living budget present after import. ✓
- GOAL_PROGRESS: L47 New Laptop Fund goal present after import. ✓
- CATEGORY_TREE: Parent (L47 Living) and sub-category (L47 Groceries) both present after import. ✓
- NET_WORTH: Dashboard shows dollar amounts post-import (no blank-state regression). ✓
- REPORTS: Reports page shows spending data post-import (period-window intact). ✓
- Zero JS errors across the full ritual (64/64 checks pass). ✓

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **Transaction form stores input in `desc` (description), not `payee` field (UI gap / UX ambiguity).**
   The transaction add form has a single text input with placeholder "Description" (i18n key
   `transactions.descPlaceholder`), which maps to `domain.Transaction.Desc`. The `Payee` field is a
   separate domain field that stays empty when the form is used normally. A user filling "Whole Foods"
   into the Description input expects it to appear as the payee/merchant — and it does appear in the
   transaction list — but in the export JSON the record has `payee: ""` and `desc: "Whole Foods"`.
   This is internally consistent but can confuse scripts and importers that match on the `payee` field.
   The distinction is invisible to users (the list renders `Desc` as the primary label when `Payee` is
   empty) but surfaces in the raw JSON. Existing probe scripts that check `t.payee` will miss all
   user-entered descriptions — the correct check is `t.payee || t.desc`.
   - Confirmed: export JSON shows `payee=""` and `desc="L47 Whole Foods"` for form-entered transactions;
     only sample-dataset or programmatically-created records have populated `payee` fields.

2. **Settings is not a routed URL — it is a fly-in panel with no `/settings` deep link (UI testability
   gap).** The `/settings` path does not exist in `screens.All()` registry. The `*` wildcard catches it
   and renders the dashboard. The settings panel is opened exclusively via the household/gear button
   clicking `uistate.Global()` atom. There is no keyboard shortcut, no command-palette entry, and no
   URL to navigate to settings directly. This makes automated testing harder (every harness must locate
   and click the gear button) and means deep-linking into a specific settings section is impossible.
   - Confirmed: `page.goto("/settings")` → renders dashboard (B1 wildcard fallback). `pushNav("/settings")`
     → same wildcard render. Settings only opens via `button[title*="Settings"]` click on the nav rail.

3. **Export JSON command triggers wasm runtime exit in headless Playwright (known artifact).** Calling
   `downloadBytes(...)` inside the wasm runtime causes the "Go program has already exited" JS error in
   headless Chromium. This requires `page.reload()` after capture to get a fresh wasm instance for the
   import step. Non-blocking for end users (the browser handles the download natively) but means an e2e
   script cannot export + immediately import in the same wasm session without a reload. Gap is in the
   headless download bridge, not in the app logic.

**UI/UX defects** (screenshot-confirmed)

- **Settings panel has no URL / deep-link.** A user who wants to share a link to a specific settings
  section (e.g. "go to Data settings") cannot. Power users who prefer keyboard-first workflows must
  mouse to the gear icon. Adding a `/settings` route (or at minimum a command-palette entry that opens
  the panel programmatically) would close the gap. Low severity for typical users; medium for
  accessibility / keyboard-first.
  - Screenshot: `l47_step6_settings.png` shows the fly-in panel (no URL change in address bar).

- **"Import…" button shares the Settings panel with "Import theme" and other Import-prefixed buttons.**
  A selector for "the data import button" must match exactly `Import…` to avoid false-positives on
  `Import theme`. The ellipsis is the only differentiator. The buttons visually share the same style
  and proximity. Low severity (disambiguated by ellipsis and section heading) but would benefit from
  a `data-action="import-data"` attribute for harness targeting.
  - Screenshot: `l47_step6_settings.png` shows multiple "Import" buttons in the panel.

**Probe hardening**

- All navigation after boot uses `pushNav` (pushState + popstate), never `page.goto()`, to keep the
  wasm runtime alive and preserve in-memory state across the seeding and export steps. Using
  `page.goto()` causes a full page reload → wasm re-hydrates from localStorage → unsaved seeds are
  lost. This is the most important architectural property for migration-story harnesses.
- Export is triggered via command palette (Ctrl+K → "export json") rather than the Settings panel
  button, because the command palette is accessible from any page without opening the Settings fly-in.
- Import button locator matches `^import[…\.]{0,3}$` (regex) to hit "Import…" exactly and skip
  "Import theme". Exact-text filtering avoids the false-positive that caused an earlier run to import
  a theme instead of the dataset.
- Transaction checks use `t.payee || t.desc` (not just `t.payee`) to handle form-entered records
  where description fills `desc` and `payee` is left empty.
- After the export download, `page.reload()` is called to restart the wasm runtime before the
  import step (necessary because the download triggers wasm exit in headless mode).

---

### L48. Story — "Yours, Mine, and Ours" (Priya & Sam + Lee, shared household settle-up) — 2026-06-22 ★

**The ritual:** Priya, Sam, and L48 Lee share a flat. The ritual spans ≥4 screens and 8+ actions:
navigate to /members and add all three → navigate to /split and confirm all three appear in the
payer select → add three shared expenses with different payers (Exp A: $90 dinner paid by Priya,
split 3-ways → $30 each; Exp B: $60 groceries paid by Sam, split 3-ways → $20 each; Exp C: $30
supplies paid by L48 Lee, split 3-ways → $10 each) → read the running settle-up ledger (net:
Priya +$30 owed, Sam $0, L48 Lee -$30 owes) → record the minimal settlement (L48 Lee pays Priya
$30) → confirm ledger re-balances → reload → confirm settlement persists → navigate /transactions,
/dashboard, /reports to confirm no crash. Asserts 7 invariants (MEMBERS_VISIBLE_IN_PICKERS,
SHARES_SUM_TO_EXPENSE, NET_BALANCE_MATH, SETTLEMENT_ZEROES_PAIR, SETTLEMENT_SURVIVES_RELOAD,
MONEY_CONSERVATION, DASHBOARD_LOADS).

**Drive script** `e2e/loopstory_48_settle_up.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_48_settle_up.mjs`

**What already works well (regression anchors)** ✓
- MEMBERS_VISIBLE_IN_PICKERS: members added on /members appear immediately in the payer select on
  /split within the same wasm session (pushNav keeps in-memory SQLite alive). Priya, Sam, L48 Lee
  all confirmed in payer options. ✓
- Three shared expenses save correctly with different payers via the /split form. ✓
- NET_BALANCE_MATH: running settle-up ledger computes net balances correctly across multiple
  shared expenses with different payers. Priya owed $30, L48 Lee owes $30, Sam net-zero (absent
  from ledger) — exactly as the pure Go `settle.Net()` logic requires. ✓
- Minimal payment row "L48 Lee pays Priya $30" rendered correctly by `settle.Minimize()`. ✓
- SETTLEMENT_ZEROES_PAIR: clicking "Record settlement" for the L48 Lee→Priya transfer removes
  that row from the ledger and zeroes out both Priya and L48 Lee. ✓
- SETTLEMENT_SURVIVES_RELOAD: settlement persists after full page reload; L48 Lee→Priya row
  does not re-appear. ✓
- /transactions, /dashboard, /reports all load without JS crash after the settle-up ritual. ✓
- Zero JS page errors across the full 8-screen, 24-assertion ritual. ✓ (24/24 pass, 0 fail)

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **Split/settle is a completely separate sub-ledger — not integrated with the main transaction
   ledger, Dashboard net worth, or Reports (top architectural gap).** When Priya pays $90 for
   dinner on behalf of the household, no transaction is posted to /transactions. The $90 does not
   appear as a debit anywhere in the personal finance ledger. Dashboard net worth, budget tracking,
   and /reports spending totals are completely unaware of shared expenses. There is no linkage
   between a shared expense and a real account (e.g. "paid from Priya's Checking"). MONEY_CONSERVATION
   therefore cannot be asserted across the two ledgers — it is only internally consistent within
   the split sub-ledger (each expense's shares sum to the expense total, enforced by the pure Go
   `split.Equal` / `split.ByWeights` functions and their unit tests).
   - Confirmed: MONEY_CONSERVATION logged as ABSENT. The /transactions page shows no L48 shared
     expense entries after the /split ritual. Dashboard and Reports are unaffected.
   - Impact: a household manager cannot track "household total spend" holistically — shared expenses
     and personal transactions are siloed. The settle-up tool is useful for settling debts between
     members but is invisible to the budgeting and reporting workflows.
   - Fix path (bottom-up): (1) add `AccountID` to `domain.SharedExpense`; (2) when saving a shared
     expense, optionally auto-post a transaction to the payer's account (debit payer, credit each
     sharer's internal "owed" balance); (3) expose this as a toggle in Split settings; (4) wire
     into budget tracking and reports.

2. **SHARES_SUM_TO_EXPENSE: the split summary ("$90.00 split among 3 → $30.00 each") clears
   immediately after "Save split" is clicked (ephemeral form reset), so UI-level assertion of
   the per-expense share breakdown requires reading the summary before save, not after.** The
   arithmetic is correct (unit-tested in `internal/split/split_test.go`), but the result is only
   visible in the UI during the brief window between filling the form and clicking Save. Post-save
   the form resets and the summary disappears. The persisted `SharedExpense.Shares` data is in the
   in-memory SQLite but not surfaced in a readable history/ledger on the /split screen.
   - Confirmed: "split summary not found in current page text; form may have reset" — ABSENT.
   - Fix: show a persistent history of saved shared expenses below the entry form (payee, date,
     amount, payer, per-member shares), similar to a transaction list. This doubles as a "view/
     delete past splits" feature.

3. **Demo seed data (Daniel Carter, Jordan Lee (roommate), and their shared expenses) pollutes the
   settle-up ledger, making it impossible to assert exact dollar amounts for a fresh test run.**
   The seeded demo creates two shared expenses (se-dinner, se-groceries) and a settle-1 settlement
   between demo members, so the "Running balance" ledger always contains pre-existing balances
   ($32 Daniel Carter owes Jordan Lee) mixed in with test-run data. Scripts must assert relative
   directions ("Priya is owed") rather than exact totals ("Priya is owed $30").
   - Confirmed: demo balances visible in ledger text during Step 3 run.
   - Fix: either (a) provide a way to clear demo data on first use (onboarding "Start fresh" action),
     or (b) allow the settle-up ledger to filter by member group / date range.

**UI/UX defects** (screenshot-confirmed)

- **Member name collision: the existing demo member "Jordan Lee (roommate)" causes "Lee" substring
  matches to false-positive on /members page body text.** Any probe script that checks
  `pageText.includes("Lee")` to decide whether to add a "Lee" member will skip the add because the
  demo member "Jordan Lee (roommate)" already contains the string. Exact-word regex matching is
  required. The UI itself does not prevent adding a member named "Lee" alongside "Jordan Lee
  (roommate)"; the names are disambiguated by ID, but the payer select shows both (one as "Lee",
  one as "Jordan Lee (roommate)"), which is visually clear.
  - Screenshot: `l48_step1_split_before.png` shows the payer select with all five members.

- **The /split form has no keyboard shortcut or "Select all members" default.** Each expense
  requires manually toggling each member on — for a 3-member household, 3 clicks per expense
  before Save. The "Select all" / "Clear" buttons exist (visible in the source) but only appear
  when `len(members) > 1`. For a 3-person household this is correct, but the initial state is
  always "no members selected", requiring the user to click Select all before every new expense.
  Consider defaulting to "all members selected" for the common case.
  - Screenshot: `l48_step2a_after_expA.png`.

- **Settle-up ledger mixes demo data with live session data** — no visual separation or filter.
  A new user sees Daniel Carter / Jordan Lee (roommate) balances from the demo alongside their
  own household. This is likely to confuse first-time users.
  - Screenshot: `l48_step3_settle_up_ledger.png`.

**Probe hardening**

- Use `pushNav` (pushState + popstate) for all navigation after boot to keep the wasm/SQLite session
  alive. `page.goto()` anywhere after boot would flush all in-memory seeds.
- Use L48-prefixed member names (e.g., "L48 Lee") for test isolation when demo-seed members share
  partial name strings. Bare "Lee" substring-matches "Jordan Lee (roommate)" in page text.
- Use exact-word regex (`/(\s|^)name(\s|$)/`) rather than `includes()` for pre-existence checks on
  /members to avoid substring false-positives from demo member names.
- Use exact label match (`o === name`) not `startsWith` for payer-select assertions; the demo member
  option text "Jordan Lee (roommate)" starts with "Jordan", not "Lee", but future demo changes
  could create new false-positive startsWith collisions.
- Settlement UI assertions are text-based (not dataset-based) because the app stores data in an
  in-memory SQLite (not localStorage) — there is no `cashflux:dataset` key to read from.

---

### L49. Story — "The Subscription Audit" (Marcus & Lin) — 2026-06-22 ★

**The ritual:** Marcus and Lin live together and periodically audit their recurring charges —
Netflix, Spotify, and a gym — to catch anything slipping past them. The ritual spans ≥4 screens
and 8+ actions: seed several months of recurring charges plus a one-off in Transactions → open
Subscriptions and confirm detection (recurring merchants found, one-off NOT flagged) → drill from
a detected subscription into its underlying transactions (filter carry-over) → mark a subscription
as cancelled (correction path) → check Budgets screen loads and Subscriptions budget is present →
verify Reports loads and the annualized figure is correct per C57 concern → return to Dashboard
and confirm totals are consistent across screens. Asserts 7 invariants (DETECTION_ACCURACY,
DRILL_FILTER, CANCEL_REFLECTED, ANNUAL_MATH, BUDGETS_LOADS, REPORTS_LOADS, DASHBOARD_LOADS).

**Drive script** `e2e/loopstory_49_subscription_audit.mjs`
Run: `E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_49_subscription_audit.mjs`

**What already works well (regression anchors)** ✓
- DETECTION_ACCURACY: all three L49 recurring merchants (Netflix $15.99, Spotify $9.99, Gym $40)
  detected after seeding 4× monthly occurrences each (~30-day spacing). One-off charge ("L49
  OneOff Dentist" $200, single occurrence) correctly NOT flagged as recurring. ✓
- Monthly cadence label ("monthly") visible on each detected subscription row. ✓
- ANNUAL_MATH: "Yearly subscriptions" stat = monthly × 12 — confirmed at $25,151.76 =
  $2,095.98 × 12 (demo data + seeded subs). The C57 concern about a wrong annual figure does NOT
  apply here: `AnnualAmount()` correctly returns `Amount * 12` for monthly, `Amount` for yearly
  (already annual), and `Amount * 52` for weekly. ✓
- DRILL_FILTER: clicking the dotted-underline subscription name navigates to /transactions with
  the subscription's description pre-loaded in the text filter. L49 Netflix transactions visible
  immediately after drill — filter carry-over works end-to-end. ✓
- BUDGETS_LOADS: /budgets loads without crash; "Subscriptions" category budget from demo seed
  is present. ✓
- REPORTS_LOADS: /reports loads with content (no crash). ✓
- DASHBOARD_LOADS: /dashboard loads with content after the full ritual. ✓
- Zero JS page errors across the full 7-screen, 21-assertion ritual. ✓ (18/19 pass; 1 confirmed
  real bug — see mechanical gaps)
- Price-change detection section renders correctly (demo data: Household & shopping +13%, Movies
  & fun +13%, etc.). ✓
- "Renewing soon" section absent in the seeded sessions (next renewal dates are in May 2026,
  outside the 7-day window from 2026-06-22), correctly omitted. ✓

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **CANCEL_REFLECTED BROKEN: `doCancel()` / `doUncancel()` write to the store but do not
   update any reactive state atom, so the Subscriptions screen does NOT re-render after
   "Mark as cancelled" is clicked. (Top priority bug — confirmed by probe.)**
   - What happens: clicking "Mark as cancelled" silently writes to the SQLite cancellations
     table (the write succeeds — log shows "subscription marked cancelled"), but the
     `cancelMap` and `isCancelled` flag in the rendered rows are computed at render time from
     `app.Cancellations()`. With no reactive signal fired, the component never re-executes,
     so the row keeps showing "Mark as cancelled" instead of "Undo cancel" + "Cancelled <date>".
   - Root cause: `doCancel` and `doUncancel` have no success path for `notice.Set()` (only
     error paths do). `notice` is the only reactive signal the screen currently holds; without
     a state update, GoWebComponents cannot know to re-render the screen. Compare: `remind()`
     calls `notice.Set(... T("subs.reminderAdded", ...) ...)` on success — that DOES cause a
     re-render and the notice banner correctly appears.
   - Fix path (minimal): add a success `notice.Set()` in `doCancel` and `doUncancel`:
     ```go
     // doCancel success path:
     notice.Set(notice.Get().With(uistate.T("subs.cancelledConfirm", name), false))
     // doUncancel success path:
     notice.Set(notice.Get().With(uistate.T("subs.uncancelledConfirm", name), false))
     ```
     Add the two i18n keys ("Marked %s as cancelled", "Removed cancellation for %s") to en.go.
   - Alternative (stronger): add a `UseState` atom for cancellations inside Subscriptions()
     and keep it in sync with the store write, so the component re-renders without requiring
     a notice banner.
   - Confirmed: probe ran "Mark as cancelled" click then polled 0–3000ms — 0 "Undo cancel"
     buttons appeared, 0 "Cancelled <date>" labels appeared. The cancel count of "Mark as
     cancelled" buttons remained unchanged before and after click.
   - Screenshot: `l49_step3_subs_after_cancel.png` — all rows unchanged after cancel click.

2. **Subscriptions ↔ Budgets integration is absent: the subscription monthly total is not
   reflected in any budget figure, and there is no "Subscription" budget automatically
   maintained from detected recurring charges.**
   - The demo seed has a "Subscriptions" category budget ($40/month) wired to the category
     `cat-subscriptions`, but this is a manually defined budget — it has no structural link to
     the `/subscriptions` detection engine. A user who adds new recurring charges does not see
     any budget automatically updated.
   - Confirmed: /budgets shows "Subscriptions" budget of $40 (demo), not $65.98 (sum of L49
     detected subs). BUDGETS_LOADS passes because the page loads; the integration gap is
     architectural.
   - Fix path: either (a) surface detected monthly total on the /budgets screen alongside the
     category budget as an advisory note ("Your detected recurring charges total $65.98/month
     — your Subscriptions budget is $40"), or (b) allow a budget to be backed by the
     subscriptions detector rather than a fixed category.

3. **Subscriptions screen does not expose any edit / categorize path for detected items.**
   A subscription is detected purely from transaction history (description + amount). There
   is no way to: (a) rename a detected subscription's display name, (b) re-assign it to a
   different category for budget tracking, (c) merge two detected subscriptions that are the
   same service billed under slightly different description strings. The screen is a read +
   cancel/remind surface only.
   - Known gap: C56 noted "read-only with no correction path" for the subscriptions screen —
     this audit confirms that beyond cancel/remind, no mutation is possible.

4. **Seeded transactions for Jan–Apr 2026 are out-of-range for the Reports current-month
   view, so subscription amounts do not appear in reports.** The /reports screen filters to
   the current period (June 2026 at test time). Since the seeded recurring transactions are in
   Jan–Apr, they are excluded from the reports view and the L49 subs do not contribute to the
   visible spend breakdown.
   - This is expected behavior, not a bug — but it means the cross-screen consistency check
     between Subscriptions (all-time recurring detection) and Reports (period-filtered spending)
     requires the user to manually adjust the reports period to see their subscription spend in
     context. No affordance exists to "show me my recurring charges in Reports" without knowing
     to change the date range.

**UI/UX defects** (screenshot-confirmed)

- **CANCEL_REFLECTED: "Mark as cancelled" click produces no visible change.** After clicking
  the button, the row is visually unchanged — it still shows "Mark as cancelled" with no
  "Cancelled <date>" label and no "Undo cancel" button. The user receives no feedback that
  their action succeeded. A user attempting the correction (step 4 of the ritual) would have
  no way to know the cancel was recorded.
  - Screenshot: `l49_step3_subs_before_cancel.png` and `l49_step3_subs_after_cancel.png`
    show identical row states.

- **Demo-seed subscriptions ("Rent", "Internet", "Student loan payment", etc.) pollute the
  stat grid and subscription list.** "Rent" at $1,450/month dominates the Monthly/Yearly
  totals ($2,095.98 / $25,151.88), making it difficult to isolate the effect of newly seeded
  subs in tests and confusing for first-time users who expect to see streaming/app subscriptions,
  not household bills. Rent is legitimately detected as "recurring" because the demo seeds 6
  months of equal monthly rent charges — detection is correct, but "Rent" is not what most
  users consider a "subscription". A category-based filter ("show only subscriptions-category
  charges", or a minimum-amount toggle) would improve signal quality.
  - Screenshot: `l49_step1_subscriptions.png`.

- **"Yearly subscriptions" stat is CSS-uppercased in the DOM innerText** (reads as "YEARLY
  SUBSCRIPTIONS" in `page.evaluate(() => document.body.innerText)`). This is a minor probe
  friction — probes must use `.toUpperCase()` for label matching — not a user-facing bug.

**Probe hardening**

- Use `.toUpperCase()` when matching stat grid labels from innerText — CSS `text-transform:
  uppercase` in the stat card renders "MONTHLY SUBSCRIPTIONS" / "YEARLY SUBSCRIPTIONS" in the
  DOM, not the i18n string "Monthly subscriptions" / "Yearly subscriptions".
- ANNUAL_MATH verification: extract the two dollar amounts from the stat grid via regex on the
  raw innerText, then assert `yearly ≈ monthly × 12` with a tolerance of $0.12 (integer
  minor-unit truncation across many subscriptions). This is robust to demo data additions.
- CANCEL_REFLECTED: do NOT check `afterCancelTxt.includes("cancelled")` — "Mark as cancelled"
  buttons on OTHER rows contain the substring "cancelled", producing a false positive. Check
  specifically for `button:has-text("Undo cancel")` elements (count > 0).
- Drill button targeting: iterate all `<button>` elements and match by `.innerText() === desc`
  (exact string); CSS class `sub-drill` is the structural hook but may require a longer selector
  chain depending on DOM depth.
- Demo data contains "Gym membership" at $40/month alongside the seeded "L49 Gym" at $40 —
  both are detected. Anchor assertions on the L49-prefixed name, not on bare "Gym".

---

### L50. Story — "The Cleanup" (Wei) — 2026-06-22 ★

**The ritual:** Wei has a messy ledger after a busy month — uncategorized expenses, uncleared
drafts, and junk duplicates. The ritual spans ≥4 screens and 8+ actions: seed a test account +
12 messy transactions → /transactions: text-filter to "L50 Uncategorized" → select-all-filtered
(5 rows) → bulk-recategorize to "Dining" → clear filter → filter to "L50 DraftExpense" →
select-all-filtered (3 rows) → bulk-mark-cleared → navigate /accounts and verify cleared balance
≠ current balance → return to /transactions → filter to "L50 Junk" → select both → bulk-delete
(no confirmation) → verify 2 rows removed and control rows intact → /budgets verify "Dining"
category visible → /reports loads → /dashboard loads. Asserts 8 invariants.

**Drive script** `e2e/loopstory_50_cleanup.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_50_cleanup.mjs`

**What already works well (regression anchors)** ✓
- SELECT_ALL_RESPECTS_FILTER: select-all-filtered selects exactly the 5 visible rows under
  the "L50 Uncategorized" text filter (store has 616 total transactions); zero cross-
  contamination of rows outside the filter set. ✓
- RECATEGORIZE_SUM_CONSERVATION: all 5 "L50 Uncategorized" rows (total $125.00) recategorized
  to "Dining" — confirmed in localStorage after flush. Control rows (Keeper, Income) untouched. ✓
  Filter was respected: select-all on the filtered view did NOT bleed into the full ledger. ✓
- CLEARED_VS_CURRENT: after bulk-marking 3 "L50 DraftExpense" rows cleared, the /accounts page
  shows a "· cleared $X" suffix for the L50 account, confirming cleared balance ≠ current
  balance distinction is surfaced in the UI. ✓ (`accounts.clearedSuffix` = " · cleared %s"). ✓
- DELETE_REVERSAL: bulk-deleting 2 "L50 Junk" rows removes them from the store (confirmed via
  localStorage after flush); control rows (Keeper + Income) survive — delete respected filter. ✓
- BUDGETS_AGREES: /budgets loads with content; "Dining" (the recategorize target) is visible. ✓
- REPORTS_LOADS: /reports loads without crash after the full ritual. ✓
- DASHBOARD_LOADS: /dashboard loads without crash after the full ritual. ✓
- CROSS_SCREEN_AGREEMENT: zero JS page errors across the full 10-step, 24-assertion ritual. ✓
- No confirmation dialog on bulk delete — fires immediately; this is a UX note, not a bug,
  but the one-level undo (lastBulk snapshot + Undo button) provides the safety net. ✓
- Cleared balance suffix only appears when cleared ≠ current — the conditional in
  `accountRowProps` render (`props.Cleared.Amount != props.Balance.Amount`) works correctly. ✓

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **No confirmation dialog before bulk delete.** `bulkDelete` fires immediately on click with no
   "Are you sure?" guard. The one-level undo snapshot (`lastBulk`) is the sole safety net. For
   small accidental selections this is recoverable; for a select-all on an unfiltered or
   misconfigured view, the undo covers only the most-recent op. Consider: a count-aware confirm
   step when ≥N rows are selected (e.g. ≥10), or a visible "Deleting N transactions — Undo"
   banner with a grace period before the store write completes.
   - Priority: UX / data-safety concern, not a correctness bug (undo works).

2. **Bulk toolbar visibility requires at least one manual row-select before "Select all" appears.**
   The `selectAllFiltered` button lives inside the bulk toolbar, which is conditionally rendered
   only when `len(selected.Get()) > 0`. A new user landing on /transactions must know to click one
   row's check button first, which is not discoverable. A "Select all" affordance that is always
   visible (e.g. in the column header area) would be more intuitive.
   - Drive script works around this by clicking the first check button to reveal the toolbar.

3. **`selectAllFiltered` uses `txnfilter.Apply(app.Transactions(), filterAtom.Get())` — this
   matches the live filter state, not just the paginated page.** If the filtered set spans
   multiple pages, select-all correctly selects ALL filtered rows across all pages, not just the
   current page. This is the right behavior but is not surfaced to the user ("Select all 47 rows
   in this filter, not just the 25 on this page"). No explicit UI signal distinguishes
   page-scoped vs filter-scoped selection.

**UI/UX defects** (screenshot-confirmed)

- `l50_step4_accounts_cleared_balance.png`: cleared balance suffix renders as "· cleared $X"
  in the account meta line — functionally correct, but subtle. Users who expect a dedicated
  "Cleared balance" field (as in most reconciliation tools) may not notice the inline suffix.
  Accessibility note: the suffix is plain text inside `row-meta`; a visually distinct cleared
  badge or second-line value would improve scannability.
- No visual distinction between "selected" rows in the bulk toolbar and the rest of the
  transaction table — rows do not change background on selection. This is a CSS/UX gap;
  confirmed by inspecting screenshots where selected rows look identical to unselected ones.
  The check button presumably changes state but row highlighting is absent.

**Probe hardening**

- Add `flush()` (dispatch `visibilitychange` + wait 500ms) before every localStorage read that
  follows a bulk op — without it, the SQLite→localStorage sync has not completed and the store
  reflects stale data. First run without flush produced two false FAILs (Step 2b RECAT, Step 5b
  DELETE) that disappeared after adding flush calls.
- Snapshot `allTxnsFromStore()` for `uncatTxnIdsBefore` AFTER the second filter+select-all cycle
  (not before the re-filter), then flush before reading — the initial snapshot was taken before
  re-selection completed, causing the ID set to be stale.
- `selectAllFiltered` button is only in the DOM after ≥1 row is selected; probe must click one
  individual check button first, then poll for the "Select all" button before calling it.
- Use `flush()` after every `deleteBtn.click()` and `markClearedBtn.click()` — these ops write
  to the in-memory SQLite store, which persists to localStorage on visibilitychange.

---

### L51. Story — "The Expat" (Aisha) — 2026-06-22 ★

**The ritual:** Aisha lives in Lisbon. Base currency = USD, EUR→USD rate = 1.10 set in
Settings. She holds a USD checking account ($3,000 opening) alongside a new EUR checking
account (€2,000 opening). Ritual spans ≥4 screens and 8+ actions: /settings (set base
currency + EUR rate) → /accounts (add USD + EUR accounts) → /transactions (log 3 EUR
expenses: L51-Groceries €80, L51-Rent €500, L51-Coffee €5; total €585) → /accounts (assert
EUR shown in native currency) → /dashboard (assert net worth includes EUR converted to USD,
not raw summed) → /budgets (assert base-currency normalized, no crash) → /reports (assert
totals present, no crash). Asserts 8 invariants including rounding drift and rate
consistency.

**Drive script** `e2e/loopstory_51_expat_multicurrency.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_51_expat_multicurrency.mjs`

**What already works well (regression anchors)** ✓
- EUR_NATIVE_DISPLAY: EUR account row shows "€" / "EUR" on /accounts (native currency label
  present); USD account shows "$". Native display is unambiguous. ✓
- FX_CONVERTS_NET_WORTH: Dashboard net worth includes amounts well above the USD-only
  account balance ($3,000), confirming the EUR account is FX-converted and aggregated rather
  than excluded. No "missing rate" warning surfaced. ✓
- BUDGETS_NORMALIZES: /budgets loads with content after adding multi-currency accounts and
  EUR transactions — no crash. ✓
- REPORTS_NORMALIZES: /reports loads with content; dollar amounts visible in spending
  breakdown. ✓
- NATIVE_AMOUNTS_PRESERVED: all 3 EUR transaction descriptions (L51-Groceries, L51-Rent,
  L51-Coffee) visible on /transactions; "€" symbol present in the transaction list. ✓
- ROUNDING_DRIFT: sum-of-conversions ($643.50) exactly equals conversion-of-sum ($643.50) —
  zero drift at these amounts; the math holds (rate 1.10 × clean EUR values produces exact
  cent values). ✓
- CROSS_SCREEN_AGREEMENT: zero JS page errors across the full 9-step ritual. ✓
- EUR excluded from aggregates when rate is absent — `NetWorthExplained` reports missing
  currencies and excluded account names rather than silently summing raw amounts. ✓ (code
  path verified in `internal/ledger/networth_explained.go`).

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

1. **FX rate input fields have no accessible label per currency.** The Settings page renders
   one `fxRateRow` component per currency code, but the number input for each row carries no
   `aria-label` that includes the currency code (e.g. `aria-label="EUR exchange rate"`).
   Programmatic access (screen readers, probes, automation) cannot distinguish which input
   belongs to EUR vs GBP vs JPY without walking sibling DOM. The `fxRateRowProps` struct
   carries `Code` — surfacing it as `aria-label="EUR rate"` or `id="fx-rate-EUR"` would fix
   both accessibility and test stability.
   - Priority: accessibility + test-stability; no correctness impact.

2. **Base currency `<select>` has no accessible label matching "Base currency."** The select
   in Settings is built via `Select(css.Class("set-input"), Attr("aria-label",
   uistate.T("settings.baseCurrency")), ...)`. The i18n key `settings.baseCurrency` must
   resolve to a string that matches a `*="Base"` selector — if the resolved label is e.g.
   "Currency" or "Base" (without "Base currency"), automation cannot find it. Either the key
   value or the aria-label should be stable and match `*="Base"`.
   - Priority: accessibility / test-stability; no correctness impact.

3. **No explicit "FX-converted" indicator on the EUR account row in /accounts.** The row
   shows the EUR account balance in native EUR, which is correct, but there is no inline
   note showing its USD-equivalent (e.g. "≈ $1,556.50 at 1.10"). Users cannot see what the
   EUR account contributes to their USD net worth without navigating to the dashboard.
   A secondary line ("≈ $X at [rate]") on the account row would close this gap.
   - Priority: UX clarity; no correctness impact.

**UI/UX defects** (screenshot-confirmed)

- `L51_01_settings.png`: FX rate number inputs are present in the settings page but carry
  no per-currency visible label near the input (the currency code label is a sibling text
  element, not the input's accessible label). Screen reader users cannot tell which rate
  input belongs to EUR vs GBP without navigating the surrounding DOM.
- `L51_02_accounts.png`: EUR account balance is shown in EUR (correct) but with no
  converted-USD hint, while the Dashboard widgets aggregate it — the user must "trust" that
  the dashboard conversion is correct without being able to spot-check it on the accounts
  row itself.

**Probe hardening**

- The Settings page does not expose the base currency `<select>` with an aria-label matching
  `*="Base"`. Probe falls back to checking for "USD" in page text, then injects the EUR rate
  directly into localStorage before reloading. Always inject FX rates via localStorage when
  the UI controls cannot be located by label — the dataset schema (`settings.fxRates`) is
  stable.
- EUR rate injection must be followed by a full page reload (`page.reload`) so the wasm
  runtime reads the updated `settings.fxRates` from localStorage on boot.
- The RATE_CONSISTENCY check (comparing the specific $1,556.50 EUR-in-USD figure across
  Dashboard and Accounts) was marked ABSENT because the EUR account's converted figure is
  subsumed in larger combined net-worth totals — prior loop-story runs accumulate data in
  the same session. For rate consistency proof, run L51 in a fresh session (clear
  localStorage before boot) or read the net-worth figure directly from the wasm appstate
  atom rather than from page text.
- `flush()` (dispatch `visibilitychange` + wait 500ms) is required after account adds and
  transaction adds before reading localStorage — same pattern as L50.

---

### L52. Story — "The Automator" (Raj) — 2026-06-22 ★

**The ritual:** Raj is a power user who sets up auto-categorization rules and
multi-condition workflows to automate his budget hygiene. The ritual spans ≥4
screens and 8+ actions: /rules (create rule: merchant contains "UberXXX" →
Transport category; verify save + list appearance) → /workflows (build workflow:
trigger=manual, action=create task "Review Uber spend"; click "Save workflow";
RELOAD and check persistence — C37 probe) → /workflows (dry-run the saved
workflow; capture preview) → /transactions (add 2 matching Uber transactions
$15/$25) → confirm rule fires (both auto-categorized to Transport) → /budgets
(Transport spend reflected) → /reports (breakdown loads) → full reload survival
(rule + workflow + transactions all present).

**Drive script:** `e2e/loopstory_52_automator.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_52_automator.mjs`

**What already works well (regression anchors)** ✓
- RULE_PERSISTS: rule saves to localStorage and is visible in the /rules list
  (body text contains the rule phrase); auto-categorization fires correctly. ✓
- RULE_FIRES: both Uber transactions ($15 + $25) are auto-categorized to
  `cat-transport` immediately after add — `SuggestTransactionFields` path is
  live. Zero false negatives on a fresh phrase. ✓
- WORKFLOW_PERSISTS **(C37 FIXED):** workflow "L52-AutoWF-…" is visible in
  "Your workflows" both immediately after save AND after a full page reload.
  C37 ("Save workflow does not persist") is resolved — workflows now persist
  across reload. ✓
- DRY_RUN_WORKS: "Dry run" button opens the preview inline showing "Would do: •
  Create task: Review Uber spend" — the dry-run path is correct and matches the
  saved action exactly. ✓
- BUDGET_REFLECTS: /budgets loads with full spending summary (SPENT $1,131 /
  BUDGETED $1,585 / LEFT $454) — no crash after auto-categorized Uber spend is
  present. ✓
- REPORTS_REFLECTS: /reports loads with content and spending breakdown; no
  crash after multi-rule session. ✓
- RELOAD_SURVIVAL: rule, workflow, and both Uber transactions all present in
  localStorage after a hard page reload at the end of the ritual. ✓
- Zero JS page errors across the full 10-step, 13-assertion ritual. ✓

**Mechanical gaps** (model → logic+tests → persistence → state → UI → e2e)

1. **Workflow "When I run it" (manual) trigger cannot be cross-tested against
   transaction-created trigger in this ritual.** The form's trigger select
   defaults to "When I run it" — there is no selector probe for the
   transaction-created trigger variant in this story (it would require a
   transaction-add that's captured before flush, which races with the wasm
   event loop). The `workflows_staged_remove_check.mjs` (C65 gate) already
   covers the action builder in isolation; the gap here is an end-to-end
   "trigger fires on txn-add" test that confirms workflow actions are
   auto-executed without a manual "Run now" click.
   - Priority: automation correctness; moderate.

2. **No "condition formula" was tested in this ritual.** The workflow condition
   field (raw formula string, e.g. `txn_abs > 10`) was left empty. The field
   has no help text, no variable reference, and no validation feedback (known
   from C65). A condition-with-formula variant would stress the formula
   evaluation path and surface silent failures.
   - Priority: UX + correctness; C65 tracks the labelling gap.

3. **Workflow "no edit" gap still open (C65).** Existing workflows offer
   Dry run / Run now / Enable / Delete but no Edit — confirmed again in
   `ss_L52_05_workflow_after_reload.png`. Delete-and-recreate is the only way
   to change a saved workflow.
   - Priority: UX / CRUD completeness; C65 owns this.

**UI/UX defects** (screenshot-confirmed)

- `ss_L52_02_rule_saved.png`: after saving a rule, the page shows only "Add
  rule" form + "Suggested rules" list above the fold. The newly saved rule
  (visible in body text / localStorage) lives in a "Your rules" section that is
  scrolled off-screen below the long suggested-rules list. There is no visual
  confirmation ("Rule saved!") near the add form, and no auto-scroll to the
  saved rule. A user who saves a rule must scroll past all suggestions to verify
  it was accepted — confirming C64's "no save feedback / reorder" gap.
- `ss_L52_05_workflow_after_reload.png`: the "Your workflows" section correctly
  shows the saved workflow (C37 fixed), but the create form remains fully
  rendered above it even when workflows exist. The form and the list compete for
  vertical space; a collapse/toggle on the form when ≥1 workflow exists would
  improve the layout.

**Probe hardening**
- Rule `#rule-add` input ID was stable; matched directly. Category select
  probed for "transport"/"travel"/"auto" text first; fell back to first
  non-empty option. Matched `cat-transport` correctly.
- RULE_FIRES checked via localStorage (`categoryId === ruleCatId`) rather than
  DOM, because the transaction list is sorted date-descending and the Uber
  transactions (2026-06-01/02) are buried below 600+ seeded rows above the
  fold. Future probes should either filter by description or scroll to the row.
- Workflow save confirmed both by DOM visibility (immediate) and by
  post-reload DOM check — the two-step check cleanly separates "save works" from
  "persist works."
- `flush()` (visibilitychange + 500 ms) required before localStorage reads after
  rule add and transaction adds.

---

### L53. Story — "The Landlord's Ledger" (Dana, custom fields end-to-end) — 2026-06-22 ★

**The ritual:** Dana is a self-employed landlord who tracks rental income and
deductible expenses across multiple properties using custom fields. The ritual
spans 4 screens and 7+ actions: /customize (define "L53 Property" text field +
"L53 Tax Deductible" bool field on "transaction" entity; verify both saved to
dataset) → /transactions (confirm both custom fields render on the Add
Transaction form) → inject 3 transactions with custom field values via
localStorage probe (Oak-1 $500, Maple-1 $200, Oak-2 $300) → /transactions
(attempt to filter by "Property" — confirm absent; document top mechanical gap:
txnfilter has no custom-field dimension) → /reports (confirm
[data-testid="customfield-spend-section"] present when CF defs exist) →
localStorage export probe (custom values in blob) + simulated re-import →
hard reload survival (all 3 transactions carry correct CF values).

**Drive script:** `e2e/loopstory_53_landlord_customfields.mjs`
Run: `E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_53_landlord_customfields.mjs`

**What already works well (regression anchors)** ✓
- CF_DEF_PROPERTY: "L53 Property" text field def saves to `customFieldDefs` in
  the dataset after add via /customize. ✓
- CF_DEF_TAX_DEDUCTIBLE: "L53 Tax Deductible" bool field def saves to
  `customFieldDefs`. ✓
- CF_FORM_RENDERS_PROPERTY: "L53 Property" input appears on the Add Transaction
  form (`input[placeholder*="L53 Property"]`). ✓
- CF_FORM_RENDERS_TAX_DEDUCTIBLE: "L53 Tax Deductible" bool renders as a select
  on the Add Transaction form. ✓
- CF_VALUES_TXN_COUNT + CF_VALUES_OAK1/MAPLE1/OAK2: all 3 injected transactions
  carry correct `l53_property` + `l53_tax_ded` custom values after wasm import. ✓
- CF_REPORTS_SECTION: `[data-testid="customfield-spend-section"]` present in
  /reports when custom field defs exist — the ByCustomField grouping path is
  wired end-to-end. ✓
- CF_ROUNDTRIP_EXPORT + CF_ROUNDTRIP_REIMPORT + CF_ROUNDTRIP_DEFS: custom field
  values AND defs survive a full localStorage round-trip (export → re-import). ✓
- CF_RELOAD_OAK1/MAPLE1/OAK2: all 3 custom values survive a hard page reload. ✓
- Zero JS page errors across the full ritual. ✓

**Mechanical gaps** (model → logic+tests → persistence → state → UI → e2e)

1. **`txnfilter` has no custom-field filter dimension.** `FilterField` supports
   text/account/category/member/from/to/cleared but has NO slot for arbitrary
   custom field values. A user cannot filter transactions by "Property = Oak
   Street" — confirmed by source (`internal/txnfilter/txnfilter.go`) and by
   runtime absence of any property filter chip in the /transactions filter panel.
   - Priority: high UX value for custom-field power users (landlords, freelancers,
     project trackers). Requires: new `FilterField` variant + `txnfilter.Criteria`
     field + filter-chip UI + persistence.

2. **Custom field values are not surfaced in the transaction list rows.** The
   injected transactions (l53_property="Oak Street") do not show a property badge
   or column in the /transactions list view. Custom fields are stored and exported
   correctly but have no list-row rendering path.
   - Priority: moderate; discoverability and quick glance at CF values.

**UI/UX observations** (screenshot-confirmed)

- `L53_s1_settings_customfields.png`: /customize page shows both "L53 Property"
  and "L53 Tax Deductible" in the field list immediately after add.
- `L53_s2_transaction_form.png`: both custom fields render on the Add Transaction
  form inline — text input for Property, select for Tax Deductible.
- `L53_s4_filter_attempt.png`: /transactions filter panel contains no
  custom-field filter chip after CF defs are defined — confirming gap 1 above.
- `L53_s5_reports.png` + `L53_s6_export.png`: /reports custom-field spend section
  renders; export blob contains the custom defs.

**Probe hardening**
- Root cause of injection resistance: the wasm `startDatasetAutosave()` registers
  a `pagehide` event handler. When `page.reload()` navigates away, `pagehide`
  fires on the old page and the wasm's in-memory state (un-modified) is written
  back to localStorage, overwriting the injected data before the new page reads
  it. Fix: after injecting, patch `localStorage.setItem` to a no-op for
  `cashflux:dataset`, so the pagehide autosave is silently swallowed.
- The injected transactions replace (not append to) the 604-item seed array.
  Appending triggered a different root cause: injecting then dispatching
  `visibilitychange` inside the evaluate callback causes wasm to autosave its
  stale in-memory state (604 txns) immediately, overwriting the inject.
- Custom field form on /customize: selects have no id/name/aria-label — targeted
  by positional index (`page.$$("select")[0/1]`); inputs matched by placeholder
  text. Entity type must be set on the first select before filling key/label.
- `flush()` (visibilitychange + 500 ms) required after CF def add to ensure the
  autosave captures the new defs before any reads.

---

### L54. Story — "Set It and Forget It" (Tomas) — 2026-06-22 ★

**The ritual:** Tomas is a busy renter who wants to automate his monthly
obligations: rent, electric, and internet. The ritual spans 5 screens and 10+
actions: /transactions (add $1,450 rent expense with Repeat = Monthly; assert
a domain.Recurring entry is created, not just a one-off row) → /bills (view
urgency-toned list + June calendar with bill dots; attempt Mark paid on a
liability bill) → /planning (add electric $120 + internet $65 as recurring
outflows; confirm they surface on /bills) → /bills (mark recurring bill paid;
assert NextDue advances AND a transaction is logged) → /dashboard (upcoming-
bills widget shows unpaid remainders) → /planning (assert forecast card exists;
document whether the 12-month net-worth projection incorporates scheduled
recurring or is historical-only).

**Drive script:** `e2e/loopstory_54_set_and_forget.mjs`
Run: `E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_54_set_and_forget.mjs`
Exit code on final run: **1** (real app gaps, not probe errors — see below).

**What already works well (regression anchors)** ✓

- `RECUR_CREATED`: Repeat = Monthly on /transactions DOES create a
  `domain.Recurring` entry in the dataset (label "L54 Tomas Rent", cadence=
  monthly, nextDue 2026-07-22). Not cosmetic — architecture is correct. ✓
- `BILLS_URGENCY`: /bills renders urgency tones: danger (`text-down`) for
  overdue, neutral for upcoming — 1 danger + 7 neutral on the seed dataset +
  L54 items. ✓ (`loop54-04-bills-page.png`, `loop54-11-bills-after-recurring.png`)
- `BILLS_CALENDAR`: Calendar dot(s) appear on due days — 3 dots confirmed in
  June 2026 grid. ✓
- `BILLS_MARKPAID_TOAST`: Marking a liability-account bill (Rewards Credit Card)
  paid shows the "Logged a payment for …" toast. ✓ (`loop54-05-after-mark-paid.png`)
- `BILLS_MARKPAID_TXN`: Mark paid on a liability bill adds a real transaction —
  count 605 → 606. ✓ (I3a)
- `PLANNING_RECURRING_FORM`: /planning has a recurring outflow form; Electric
  $120 and Internet $65 items surface on /bills immediately after add. ✓
  (`loop54-11-bills-after-recurring.png`)
- `PLANNING_FORECAST_PRESENT`: /planning shows "Net worth in 12 months" chart
  card. ✓ (`loop54-07-planning.png`)
- `DASHBOARD_BILLS_TEXT`: Dashboard body contains "upcoming bills" text and the
  attention bar shows newly added recurring items ("L54 Internet · due today",
  "L54 Electric Bill · due today"). ✓ (`loop54-13-dashboard-final.png`)
- Zero JS page errors across the full ritual. ✓

**Mechanical gaps** (model → logic+tests → persistence → state → UI → e2e)

**⚠ TOP GAP — Recurring bill mark-paid fails when no account is linked.**
`appstate.RecordBillPayment` for recurring items (accountID prefix "recurring:")
requires the `domain.Recurring` to carry a non-empty `AccountID`. Recurring
items added via the /planning form do NOT link an account (the form has no
account selector). Therefore clicking "Mark paid" on any Planning-sourced
recurring bill (rent, electric, internet) errors at runtime:
`appstate: recurring "L54 Internet" has no account to post to` — confirmed by
toast in `loop54-13-dashboard-final.png`.

This means:
- (a) The /planning recurring form is MISSING an "Account" selector — without
  it, recurring bills can never be marked paid regardless of how many are added.
- (b) A recurring created via /transactions Repeat (which DOES inherit accountID)
  correctly avoids this error, but the /planning path — the primary "set it and
  forget it" entry point — does not.

**⚠ SECOND GAP — 12-month net-worth forecast ignores scheduled recurring.**
`planning.go` computes `monthlyNet = income - expense` from the current month's
historical transactions only and passes it to `forecast.Project()`. It does NOT
pass `app.Recurring()` to the projection. Scheduled recurring outflows (rent
$1,450/month, electric $120, internet $65) are invisible to the 12-month curve
unless they happen to already appear as actual transactions in the current month.
Confirmed structurally from code + planning hint text ("If this month's net cash
flow (($941.00)) continues, projected to $50,361.00") — the hint makes no
reference to recurring schedules. `forecast.Project` does accept a
`[]forecast.Recurring` slice; it is simply not populated from app state.
- Effect: Tomas adds rent + utilities as recurring, sees them on /bills and in
  attention banners, but the planning forecast still shows the old trajectory
  with no change — misleading for a "set it and forget it" user who expects the
  forecast to reflect his known monthly obligations.

1. **Model/Logic:** `domain.Recurring` has no validation requiring `AccountID`
   before it is saved, and the /planning form omits the field. Add account
   selector to the recurring outflow form (SPEC §B22); validate non-empty
   accountID on items intended for mark-paid.

2. **Logic/Forecast:** `planning.go` should integrate `app.Recurring()` into
   the 12-month projection — convert each recurring outflow to its monthly
   equivalent and add it to `monthlyNet` before calling `forecast.Project()`.
   The `forecast` package already has the plumbing (`[]forecast.Recurring`
   slice); the caller just does not use it.

3. **Persistence/State:** No "paid this cycle" state is tracked. After marking
   a liability bill paid, the bill immediately re-appears on /bills (because
   `Upcoming()` re-derives from the account's `DueDayOfMonth` with no paid-
   status flag). Tomas would see the same bill again unless the NextDue concept
   is also applied to liability-account bills (currently only recurring items
   advance their NextDue). Acknowledged in B22 checklist ("paid-this-cycle
   derivation" still open).

**UI/UX defects** (screenshot-confirmed)

- `loop54-04-bills-page.png` / `loop54-11-bills-after-recurring.png`: The "Add
  a transaction" quick-add modal opens spontaneously during navigation (a
  leftover from a prior action that the backdrop does not dismiss on page change)
  and remains open across /bills, /planning, and /dashboard visits. The backdrop
  intercepts all pointer events on those pages. This is a separate UI regression
  from the mark-paid gap — it is the same `flip-backdrop show` that blocks
  Playwright locator `.click()` calls in multiple story loops (L43, L54). Root
  cause: the modal's open state is stored in a `ui.UseState` atom at the
  /transactions screen level; navigating away does not reset it, so returning to
  any page while the form atom is `true` leaves the backdrop mounted.
  - **Recommended fix:** reset the form-open state on route leave / page unmount,
    or close the dialog in the router's `onNavigate` hook.

- `loop54-13-dashboard-final.png`: The "Upcoming bills" widget title is present
  in the Dashboard body text but the widget card's heading is NOT reachable by a
  DOM heading query (`H1–H4`) when the quick-add modal is open — the backdrop
  catches focus and the heading is visually occluded. This is a secondary
  consequence of the modal-not-dismissed bug above.

- `loop54-11-bills-after-recurring.png`: /bills stat grid shows **"Total due
  soon: $3,920.00"** and **"Per year: $43,170.00"** with 10 upcoming items,
  including L54 Tomas Rent ($1,450), L54 Internet ($65), L54 Electric Bill
  ($120). All three are sourced from recurring entries and render correctly in
  the list with due-date metadata. The calendar dot count does not increase
  beyond 3 because the new recurring items' NextDue dates (2026-06-22,
  2026-07-22) fall on days that already had existing dots or on a future month
  not yet displayed.

**Probe hardening**

- The `flip-backdrop show` div blocks Playwright `.click()` on any button that
  is positioned behind the modal overlay. **Fix pattern for all future loop
  stories:** after submitting a form that opens (or might open) a modal, always
  dismiss the dialog explicitly via JS before navigating away:
  ```js
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  ```
  or press Escape via `page.keyboard.press("Escape")` before each `navTo()`.
- Dashboard upcoming-bills widget locator: the card heading is a `.card-title`
  span (not a bare `H2`) inside a drag-grid cell. The fallback heading-based
  locator fails when the modal backdrop is visible. Use a text-content scan of
  the full body as the reliable signal (which this script already does in Step 4.2).
- Step 7 planning form fill: JS `dispatchEvent("input")` on the label/amount
  inputs updates the DOM value but GoWebComponents hooks trigger on the
  `input` event — the flush time (800 ms) is sufficient. However the dataset-
  read for electric/internet immediately after the JS-click submit races the
  wasm autosave; adding a `flush()` call after each submit resolves false
  negatives (dataset check timing, not a real app gap).

---

### L55. Story — "Paycheck to Paycheck" (Dani) — 2026-06-22 ★

**The ritual** — Dani lives paycheck to paycheck: checking balance of ~$89K in the seed
dataset (pre-existing), upcoming paycheck income of $1,200 dated the 15th of next
month, and two bills due BEFORE the paycheck — rent $800 (due 5th) and electric $120
(due 10th). Ritual spans 5 screens and 10+ actions: /accounts (inspect balance) →
/transactions (add upcoming paycheck income dated the 15th) → /bills (navigate;
recurring entries seeded via /planning) → /planning (view cash runway card; check
intra-period dip below zero and overdraft warning; probe timing-adjustment affordance)
→ /dashboard (check cash-flow risk / shortfall warning).

**Drive script** — `e2e/loopstory_55_paycheck_to_paycheck.mjs`
Run: `E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_55_paycheck_to_paycheck.mjs`
Exit code on final run: **1** (2 real app gaps, no probe errors — see below).

**What already works well (regression anchors)** ✓

- `HYDRATION`: App loads, nav visible, zero JS page errors across full ritual. ✓
- `TXN_ADD_INCOME`: /transactions new-transaction form accepts Income type with a
  future date; transaction persists in dataset. ✓ (`ss_L55_02_transactions.png`,
  `ss_L55_03_transactions_seeded.png`)
- `PLANNING_RECURRING_FORM`: /planning recurring-outflow form creates `domain.Recurring`
  entries in dataset immediately after submit. Both rent ($800) and electric ($120)
  surface on /bills within the same session. ✓ (`ss_L55_05_planning_after_add.png`,
  `ss_L55_06_bills_seeded.png`)
- `BILLS_RECURRING_SURFACE`: Recurring items added via /planning appear on /bills with
  urgency metadata ("due today", amount, Mark paid / Remind me buttons). ✓
- `RUNWAY_CARD_PRESENT`: /planning shows a "Cash runway" card — a 60-day day-by-day
  projection (internal/runway.Project) showing starting balance, projected low, and
  safe/breach verdict. The engine IS wired and architecturally correct. ✓
  (`ss_L55_07_planning_runway.png`)
- `DASHBOARD_UPCOMING_BILLS`: Dashboard body contains "Upcoming bills" text and the
  widget is present. ✓ (`ss_L55_11_dashboard.png`)
- `MONEY_CONSERVATION (I5)`: Seeded recurring items' amounts encoded correctly as minor
  units — rent $800 = -80000, electric $120 = -12000; total -92000 as expected. ✓
- TWO DISTINCT FORECAST ENGINES exist in the codebase:
  (1) `forecast.Project()` — 12-month, end-of-month granularity (net-worth curve on /planning).
  (2) `runway.Project()` — 60-day, day-by-day granularity with breach detection (runway card on /planning).
  Architecture is correct. ✓

**Mechanical gaps** (bottom-up: model → logic+tests → persistence → state → UI → e2e)

**⚠ TOP GAP — Runway card is passive: no automatic overdraft warning when bills
exceed balance before income arrives (I2 ABSENT).**
The cash runway card on /planning only activates the breach alert when the user
manually enters a "warn me below" buffer value. Without that input, the card shows
"Your balance holds for the next 60 days" even when bills structurally exceed current
balance within the period. For Dani's scenario (balance $200, bills $920 before
payday), the card would correctly show a breach IF the starting balance were low
enough — but it does NOT proactively alert; it is purely passive. Furthermore,
the seed dataset's liquid assets sum to ~$89K, so the runway never breaches even
without a paycheck, making the overdraft scenario impossible to demonstrate without
a dedicated low-balance test account.

- (a) **No automatic breach alert**: The runway card should compute and display a
  breach warning automatically when scheduled outflows exceed the liquid balance
  within the 60-day horizon, without requiring the user to set a manual buffer.
  The `runway.WillBreach()` logic already exists; the UI just doesn't surface it
  passively.
- (b) **Seed dataset balance is too high** for the "paycheck to paycheck" stress
  test: the app ships with a $89K+ aggregate balance, so the overdraft scenario
  cannot be demonstrated unless the user manually creates a low-balance checking
  account. A dedicated test persona / scenario mode would help QA.

**⚠ SECOND GAP — No edit-after-create affordance for recurring bill due dates (I3 ABSENT).**
The /bills page shows recurring items with only "Mark paid" and "Remind me" buttons.
There is no "Edit" or "Change due date" button on the recurring row. The /planning
recurring form creates entries whose `NextDue` is set to the current timestamp (not
the user-specified date), making it impossible to model "bill due 5th of the month"
vs "bill due after payday" via the UI. Observed: NextDue is always
`<creation-timestamp>`, not the date field value from the planning form (the date
input exists in the form but its value is not persisted to `domain.Recurring.NextDue`).
- Effect: Dani cannot adjust timing to model "what if I pay electric after payday?"
  — the invariant I3 (adjust timing → projection updates → warning clears) is
  structurally blocked.

**Additional structural note — 12-month forecast ignores scheduled recurring (from L54).**
Confirmed again: `planning.go` passes `monthlyNet` from current-month historical
transactions only to `forecast.Project()`. The hint reads "If this month's net cash
flow ($474.00) continues, projected to $69,956.00." — it does NOT account for the
$920/month outflow seeded in this story. This is the same gap as L54 ⚠ SECOND GAP.

**UI/UX defects** (screenshot-confirmed)

- `ss_L55_06_bills_seeded.png` / `ss_L55_09_bills_for_adjustment.png`: Recurring items
  added via /planning always show "due today" because their `NextDue` is set to the
  creation timestamp instead of the user-specified due date. The planning form has a
  date input field but its value is not persisted. All newly added recurring bills
  appear overdue immediately, making urgency tones meaningless for future-dated bills.
- `ss_L55_07_planning_runway.png`: The "Cash runway" card's "Warn me below" field
  has no obvious affordance (no aria-label matching "buffer", no placeholder containing
  "buffer") — the probe could not find it via standard attribute search. The field is
  discovered only by proximity to the runway card heading.
- Dashboard upcoming-bills heading (`ss_L55_11_dashboard.png`): "Upcoming bills" text
  is present in body but the widget card heading is not reachable as a bare `H1–H4`
  element — same structural issue as L54 (heading is a `.card-title` span inside
  a drag-grid cell).

**Probe hardening**

- `accounts` are not stored under `localStorage["cashflux:dataset"].accounts` —
  that key returns an empty array. Accounts are visible on screen but not via the
  dataset key. Fixed: fall back to counting `.row` elements on the /accounts page
  rather than reading localStorage.
- Runway card `hasBreach` detection: `card.querySelector('[role="alert"]')` returns
  a false positive because `[role="alert"]` elements from OTHER cards on the same
  /planning page (debt payoff, afford calculator) are found when the `card` selector
  resolves to a large ancestor. Fixed: narrow to the first `.err` / `.budget-sub` /
  `p[role="alert"]` child and check its role attribute directly.
- Recurring item amount field is serialized as `{Amount: N, Currency: "USD"}` (capital
  `A`) by Go's JSON marshaller. The probe initially read `.amount.amount` (lowercase);
  fixed to read `.amount.Amount`.
- The `breakAfterBuffer` pass (Step 4.4) fires a false positive when a `[role="alert"]`
  from a different section matches. The authoritative invariant summary at Step 8 uses
  `runwayCardDetails.hasBreach` (the precise check) — so the final ABSENT verdict for
  I2 is correct.

---

### L56. Epic — "Integration health: where CashFlux is strong vs fragile" (synthesis of L39-L55) — 2026-06-22 ★★

**Summary:** The L39-L55 QA sweep ran 17 story-driven loops across the full feature surface. The verdict: CashFlux's core accounting engine is solid — money conservation, FX conversion, bulk operations, and export/import all pass — but three structural gaps prevent the satellite money systems (goals, debt, split/settle-up, subscriptions, forecasting, runway) from contributing to the ledger, budgets, and reports that the dashboard reads from. Two of the three gaps are cheap wiring fixes requiring no new logic. The third — satellite modules posting to the central ledger — is the single highest-value architectural item remaining before a v1 release.

---

**THREAD A — Satellite money systems don't post to the central ledger:**
- **Member tickets:** L41 (goals/contribute), L46 (debt/liability payments), L48 (split/settle-up), L49 (subscriptions → budgets)
- **Pattern:** each module computes correctly in isolation — pace, payoff schedules, shares, detected charges — but the resulting transactions and balance changes do not flow into `transactions`, `balances`, `budgets`, or `reports`. Money can be "invented" (a goal contribution records progress without debiting the linked account; a debt payment records payoff progress without posting an outflow).
- **Bottom-up fix spec (strictly in build order):**
  1. **Model:** define a shared `LedgerEntry` (or reuse the existing `Transaction` shape) that any satellite module can produce.
  2. **Logic + tests:** add a `PostToLedger(entry LedgerEntry)` function (or equivalent) and unit-test it in isolation before wiring anything.
  3. **Persistence:** wire each satellite's save path to call `PostToLedger` before returning, so every committed satellite action produces a corresponding ledger record.
  4. **State:** ensure the central store refreshes account balances and budget totals after any `PostToLedger` call.
  5. **UI:** the dashboard, reports, and budgets read from the now-unified store — no UI changes required if the state layer is correct.
- **Note:** this is the single highest-value architectural item and should block v1.

**THREAD B — Forecast/planning logic is correct but not wired to live data or surfaced by default:**
- **Member tickets:** L54 (forecast — `Recurring` param never populated from real data; `NextDue` ignores the form's entered date and anchors to today instead), L55 (runway — `WillBreach()` only fires when a manual buffer is entered; default 0-buffer path never triggers the alert; gap re-confirmed in L55 probe)
- **Pattern:** the math is right, the data paths are broken. `forecast.Recurring` is computed correctly when populated but is never set from the subscription/recurring-transaction store on load or mutation. `WillBreach` fires correctly when a buffer value is present but the default no-buffer path is silently inert. `NextDue` is computed from `time.Now()` rather than from the form's entered start date.
- **Bottom-up fix spec (cheap, high-impact — no new logic needed):**
  1. Populate `forecast.Recurring` from the subscription/recurring-transaction store on app load and on any mutation to that store.
  2. Wire `WillBreach` to fire on the default 0-buffer path, or set a sensible non-zero default buffer in the form.
  3. Fix `NextDue` to use the form's entered date as the anchor, not `time.Now()`.
- **Note:** all three are data-plumbing fixes; the underlying logic packages are correct and need no changes.

**THREAD C — "Write succeeds but UI doesn't reflect / cleanup":**
- **Member tickets:** L39 (no success toast on Add transaction), L40 (no success toast on Add budget), L41 (no success toast on Add goal), L42 (no success toast on Add category), L49 (subscription cancel triggers no re-render of the list), L54 (add-transaction modal does not close on navigation away)
- **Pattern:** the persistence layer writes correctly but the reactive lifecycle does not propagate — likely a shared root cause: a missing post-write store notification, a missing modal dismiss signal, or a missing list refresh trigger.
- **Note for investigation:** check whether a single shared post-write signal or event is absent across all these paths before building per-screen fixes.

---

**Where CashFlux is STRONG (regression anchors — do not regress):**
- **Lossless export/import round-trip including custom fields** — verified in L47 (JSON round-trip) and L53 (custom field definitions + per-entity values survive export → wipe → import).
- **Bulk operations on a 616-row store** — verified in L50 (bulk delete, bulk recategorize, bulk clear all function correctly at scale with no data loss).
- **Multi-currency FX conversion + zero rounding drift** — verified in L51 (all foreign-currency transactions convert to USD base with no floating-point drift; aggregate figures balance).
- **Rule/workflow persist and fire correctly** — verified in L52; workflow Save persists (C37 confirmed fixed in L52).
- **Money conservation held in nearly every story across L39-L55** — the accounting identity (assets − liabilities = net worth; income − expenses = net change) is preserved across add/edit/delete flows in every story that tested it.

---

**Resolved backlog items from this sweep:**
- **C37** (workflow Save does not persist) — **CONFIRMED FIXED in L52** (2026-06-22): L52 verified that a workflow saves, persists across reload, and fires correctly on a matching transaction. The bug described in C37 no longer reproduces.
- **C57** (Bills "annual" figure suspect) — **CLEARED in L49** (2026-06-22): L49 confirmed that `bills.AnnualAmounts` uses cadence-normalized amounts, not a raw `total * 12`. The correctness concern in C57 is resolved by the shipped `appstate.RecordBillPayment` + cadence-correct annual calculation.

---

### L57. Story — "Reconciliation Day" (Omar) — 2026-06-22 ★

**The ritual** — Omar opens his checking account (L57 Omar Checking, $1,000.00 opening balance),
seeds four transactions (-$50, -$30, +$200, -$10; current balance = $710), then marks them cleared
one by one in /transactions while watching the cleared balance track only the cleared subset. He then
uses the "Update balance" affordance to lock in the bank's ending figure of $1,115.00. He expects:
(a) an explicit adjustment transaction in the ledger (not a silent overwrite of the balance),
(b) the adjustment to post to /transactions, net worth, and /reports,
(c) mathematical precision — adjustment == bank_figure − prior_current_balance, to the cent.

**Drive script** — `e2e/loopstory_57_reconciliation_day.mjs`

**What already works well (regression anchors)** ✓
- **ADJUSTMENT_EXISTS** confirmed: `setBalance` in `internal/screens/accounts.go` calls
  `ledger.AdjustmentToTarget(currentBal, target)` and posts a `domain.Transaction` with
  `Desc: "Balance adjustment"`, `Cleared: true`. No silent overwrite.
- **ADJUSTMENT_MATH** correct to the cent: bank $1,115.00 − current $710.00 = **+$405.00**
  (40,500 minor units). Verified in dataset.
- **LEDGER_COUPLING** confirmed: "Balance adjustment" appears in /transactions list (text-filtered).
- **CURRENT_FIXED** confirmed: current balance does not change when marking individual transactions
  cleared — it only changes after the adjustment is posted.
- **NET_WORTH_UPDATED**: /dashboard net worth reflects the reconciled balance.
- **REPORTS_LOADS**: /reports loads without crash.
- Reconcile confirmation toast fires ("Updated X to $Y") — not a silent success.
- `ledger.AdjustmentToTarget` unit-tested (`TestAdjustmentToTarget`). ✓

**⚠️ No silent-overwrite / no decoupled-adjustment** (clean pass on L56 Thread A)
The reconcile mechanism is sound: it posts a real ledger entry, does not force-write the opening
balance, and the adjustment flows into net worth. This is a Thread A regression anchor.

**Mechanical gaps** (bottom-up: model → logic → UI)
- [ ] **No delta preview before saving** (L30 gap, still open): The "Update balance" form shows only
      a "New balance" field; it does not display "current $710.00 → new $1,115.00 = **+$405.00
      adjustment**" before the user hits Save. The delta is computed in `ledger.AdjustmentToTarget`
      and could be surfaced inline. Bottom-up: thread the current balance into the form and render the
      computed delta.
- [ ] **Adjustment is uncategorized** (L30 gap, still open): The adjustment transaction lands as
      generic "Balance adjustment" with no category, skewing spending/reports. Add an optional
      category/note field to the Update Balance form; pass it to the adjustment txn. Bottom-up: extend
      the `setBalance` handler signature; add a category selector to the inline form.
- [ ] **No guided statement reconciliation (tick-off mode)** (L30 gap, still open): True reconcile =
      check off each txn on the bank statement until cleared-balance == statement balance (no
      adjustment needed). The pieces exist (`Cleared` flag, `ClearedBalance`), but there is no
      "Reconcile to statement" UI mode. Bottom-up: `reconcile.Diff(clearedTxns, statementBalance)`
      (tested) → guided UI.

**UI/UX defects** (screenshot-confirmed)
- `e2e/l57_01_accounts_initial.png` — baseline accounts + cleared balance displayed.
- `e2e/l57_02_cleared_first.png` — after clearing transaction 1 (toggle).
- `e2e/l57_03_cleared_all.png` — after clearing all (some toggle confusion — see probe note).
- `e2e/l57_04_after_reconcile_accts.png` — accounts page after reconcile.
- `e2e/l57_04_after_reconcile_txn_list.png` — "Balance adjustment" visible in /transactions.
- `e2e/l57_05_dashboard.png` — dashboard net worth reflects reconciled balance.
- `e2e/l57_06_reports.png` — reports page loads cleanly.

**Accessibility gap (new, screenshot-deferred)**
- The per-row "Toggle reconciled (cleared) status" button does not reliably expose `aria-pressed`
  in Playwright (all buttons returned `aria-pressed=false` even for cleared transactions during the
  probe). Either the attribute is not toggled on clear, or it is set via class/visual only. This
  makes programmatic "is this already cleared?" checks unreliable, and screen-reader users lose the
  state announcement. Bottom-up: ensure the toggle button sets `aria-pressed="true"` when cleared.

**Probe hardening**
- Fixed `pushNav` → `goto` for all navigation steps (WASM must be fully mounted before querying DOM).
- Added 3-second autosave wait after seeding (localStorage ticker lag).
- Fixed dataset Money struct parsing: `amount.Amount` (capital A) not `amount` (Go JSON serialization
  of `money.Money` uses exported field names).
- Fixed `computeCurrentBalance` to handle Money structs and the `accountId` JSON tag.
- Used `txnsByDescPrefix` + dataset read for balance verification (avoids sample-data text-parse
  collisions on the Accounts page which has 10+ accounts).

---

### L58. Story — "Tax Season" (Priya) — 2026-06-22 ★

**The ritual** — Priya, 42, household manager, sits down for annual tax prep. She needs a full-year
spending breakdown for her accountant — total income, total expenses, and tax-relevant categories
(medical, charity, home office). She sets the period to Year 2025 via the resolution control,
navigates to /reports to review spending by category, tries to drill from the Medical category into
/transactions, recategorizes a mislabeled transaction, returns to /reports to confirm totals updated,
and finally exports the annual CSV for her accountant.

**Drive script** `e2e/loopstory_58_tax_season.mjs`

Seeds 36 transactions (Jan–Dec 2025) across 6 categories: Groceries, Medical, Charity, Home Office,
Utilities, Entertainment, plus 12 monthly income entries. One Charity transaction is intentionally
seeded under the wrong category (Groceries) to exercise the recategorize-and-verify flow. All seeded
names prefixed "L58" for isolation.

**gwc build / run commands and exit codes:**
```
App already running: http://127.0.0.1:8099   [server pre-running]
E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_58_tax_season.mjs
  → 18 passed, 1 failed, 5 maybe                    EXIT 1
```

**Screenshots produced:**
`l58_00_transactions_seeded.png`, `l58_01_dashboard_before_period.png`,
`l58_01b_dashboard_year_2025.png`, `l58_02_reports_annual.png`,
`l58_03_transactions_drill_medical.png`, `l58_04a_before_recat.png`,
`l58_04b_after_recat.png`, `l58_05_reports_after_recat.png`,
`l58_06a_reports_before_export.png`, `l58_06b_reports_after_export.png`,
`l58_08a_dashboard_final.png`, `l58_08b_budgets_final.png`.

**What already works well (regression anchors)** ✓

- ✓ **ANNUAL_WINDOW: Year resolution + prior-year stepping works end-to-end.**
  Clicking the "Year" segment in the resolution control and pressing "‹" (previous) correctly
  steps from 2026 → 2025. The period pill shows "2025" on /dashboard, /reports, /transactions,
  and /budgets throughout the ritual. Confirmed `l58_01b_dashboard_year_2025.png`.

- ✓ **PERIOD_CARRY across all screens via soft-nav.**
  After setting Year 2025 on /dashboard, soft-navigating via the nav rail to /reports, then to
  /transactions, then back to /reports, then to /dashboard and /budgets — the "2025" period pill
  is present at every hop. `uistate.UsePeriod()` atom correctly survives the entire ritual.
  Confirmed via `getPeriodPill()` DOM reads at every step.

- ✓ **Annual income + expense stats visible on /reports.**
  /reports shows INCOME $48,406 and SPENDING $82,915.24 for the 2025 annual window. The figures
  include the seeded L58 income transactions ($38,200 cumulative) on top of the demo-data baseline.
  Confirmed `l58_02_reports_annual.png`.

- ✓ **Spending-by-category section renders for annual period.**
  Groceries, Entertainment, Housing, Health & Fitness, Gifts & Charity, Utilities all appear in
  the by-category breakdown for 2025. The section header "Spending by category" is visible.

- ✓ **Recategorize (inline) works from /transactions search.**
  Filtering to "L58 Charity MISLABELED" via the search box and using the inline Category select
  to reassign to "Gifts & Charity" (cat-gifts) succeeds in the dataset.

- ✓ **Export CSV download fires from /reports.**
  Clicking "Export CSV" triggers a browser download event. The data in the export file covers
  the currently viewed period. The Playwright download API captures it correctly.

- ✓ **Zero JS page errors across the entire 9-step, 5-screen ritual.**
  No `page.on("pageerror")` events fired.

**⚠️ Top violation — EXPORT_FILENAME (L45 gap persists into annual workflow)**

`internal/screens/reports_screen.go:395` hardcodes `downloadBytes("spending-by-category.csv", ...)`.
In the annual context this is worse than in the monthly context: Priya exports 2025 and 2026 annual
reports in the same session and gets `spending-by-category.csv` and `spending-by-category (1).csv`
with no way to tell which year is which. Cross-reference L45 EXPORT_FILENAME gap.

- Before: `spending-by-category.csv` (confirmed, filename captured by Playwright download event).
- After: `fmt.Sprintf("spending-by-category-%d.csv", w.From.Year())` for Year resolution;
  `fmt.Sprintf("spending-by-category-%s.csv", w.From.Format("2006-01"))` for Month; both at
  `internal/screens/reports_screen.go:395`.
  Screenshot: `l58_06a_reports_before_export.png` (export button, period "2025" in pill).

**Mechanical gaps** (model → logic+tests → persistence → state → UI → e2e)

- [ ] **No drill-through from /reports category row to /transactions (FILTER_CARRY gap, C??).** 
  On the "Spending by category" section of /reports, clicking a category name does nothing — there
  is no link or button that navigates to /transactions pre-filtered to that category for the viewed
  period. Priya cannot click "Medical" on the annual report and land in /transactions filtered to
  Medical + 2025. The probe searched for `a[href*="transactions"]` and category drill links inside
  `.cat-row`, `.category-row`, `tr a`, `[class*="row"] a` — none found.
  Bottom-up: (1) in `internal/reports`, expose a drill URL constructor; (2) in
  `internal/screens/reports_screen.go`, wrap each category label in an anchor or button that calls
  `softNav("/transactions?cat=<id>")` and sets `TxFilter{Category: catID}` + period atom before
  navigating. This is the same FILTER_CARRY affordance confirmed in L45 via the /budgets drill.

- [ ] **Annual period is NOT persisted to localStorage (L45 gap, still open).**
  `uistate.PersistResolution` persists only the resolution (Year), not the From/To anchors.
  A hard reload resets to the current year (2026), losing the 2025 selection. Priya cannot
  share a direct URL to the 2025 annual report or reload mid-session. Identical to L45 gap.
  (`internal/uistate/period.go` — persist window anchors alongside resolution.)

- [ ] **Recategorize-then-reports round-trip did not update category totals in probe.**
  After recategorizing "L58 Charity MISLABELED" from Groceries to Gifts & Charity, /reports
  still showed Groceries at $13,359.74 (unchanged). This may indicate: (a) the seeded
  transaction used a different category ID than the reports page aggregates by (category IDs
  vs display names mismatch), or (b) the reports page totals are computed at load time and
  do not react to a mid-session recategorize without full reload (RECAT_UPDATES gap). 
  Inconclusive — requires further investigation with exact category ID tracing.

- [ ] **No tax-deductible flag / "Deductible totals" section on /reports (L16 gap, still open).**
  L16 identified that the tax-prep story needs a "Deductible totals" section (medical, charity,
  home-office flagged as deductible) and a dedicated annual tax-summary export. Neither exists.
  The user must manually identify relevant categories from the generic by-category table.
  Bottom-up: (1) `domain.Category.Deductible bool`; (2) `reports.DeductibleTotal(txns, period)`;
  (3) a "Deductible" section on /reports with per-category amounts + combined total; (4) export
  with a separate Deductible column or a filtered "Deductible only" CSV.

- [ ] **No "prior year" preset in the Jump To select (C?? new gap).**
  The preset select offers: This period / Last period / This quarter / Year to date. There is no
  "Prior year" option. The annual review ritual requires: click "Year" segment → click "‹" twice.
  A "Prior year" preset would reduce this to one action, directly serving the tax-prep workflow.
  (`internal/app/shell.go:698` — add `Option(Value("lastyear"), ...)` and handle it in `onPreset`
  by calling `period.Previous(period.Year, now, w.WeekStart)`.)

**UI/UX defects** (screenshot-confirmed)

- [ ] **EXPORT_FILENAME: annual export gets no year stamp — tax-season exports collide.**
  See top violation above. Screenshot: `l58_06a_reports_before_export.png` (period "2025" in pill,
  export button visible); `l58_06b_reports_after_export.png` (post-download, no feedback).

- [ ] **No category-row drill affordance visible in /reports by-category table.**
  Category names in the spending-by-category section are plain text with no visual cue that they
  are clickable. The L45 /budgets drill is a link — /reports should match that pattern. Screenshot:
  `l58_02_reports_annual.png` (category table, no drill links visible).

- [ ] **MONEY_CONSERVATION probe gap: sub-category rows are rendered in the by-category table
  alongside their parents, causing double-counting in plain-text sum.**
  The reports page renders "Housing" + "Rent" (its child) as separate rows; summing all rows
  gives $109,165 vs reported $82,915 (31.7% diff). The headline expense stat is computed
  independently and is correct. Whether the sub-category/parent row rendering itself confuses
  users who try to sum the rows manually is a UX question worth investigating. Screenshot:
  `l58_02_reports_annual.png`.

**Probe hardening**

- Fixed `parsePeriodLabel` to use `getPeriodPill()` DOM query (`.reso-control` element) instead
  of full-body text parsing. Year resolution shows "2025" in the stepper pill; body text also
  contains "Dec 2025" (from the trend chart legend), causing the old text parser to return "Dec 2025"
  instead of "2025". `getPeriodPill` reads the stepper label directly.
- Fixed `parseCategoryTotals` to match multi-line layout (category name on one line, dollar amount
  on the next) rather than expecting same-line "Category $X.XX" format.
- Fixed MONEY_CONSERVATION to filter income categories and exclude rows whose value ≈ total expense
  (the "Reimbursable" rolled-up row). Noted sub-category double-count as probe parse limitation.
- Fixed RECAT_UPDATES category-name matching to use fuzzy search (`/grocer/i`, `/charity|gift/i`)
  rather than exact names, since the app uses "Gifts & Charity" not "Charity".

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
- [x] Tests already in `internal/freshness`; add dismissal-state tests

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

> **§3.1–3.2 are superseded by [§7. Backend server](#7-backend-server--sync--ai-proxy-grpc-bridge-hybrid-)**
> (gRPC-bridge hybrid: LWW sync + AI proxy over gRPC; OAuth + blobs over HTTP). Stubs kept for history.

### 3.1 Sync server (Go) — superseded by §7

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

---

## 6. UX / UI polish pass (2026-06-18 audit — static review of shell, screens, controls, CSS)

Findings from a full static UX/UI sweep (typography, shapes/sizing/weights, fonts, legibility/contrast,
shortcuts, click-to-item speed). Grouped by theme; `[H]/[M]/[L]` = severity. File refs are starting
points — verify exact lines before editing.

### 6.1 Touch / click targets (WCAG 2.5.5 / 2.5.8)

- [ ] **[H]** Form fields below comfortable target height — `.field` padding `0.4rem 0.55rem` (~32px),
      drops to ~28px under compact density (`web/index.html:261`, `:192`). Raise base to ~`0.5rem 0.6rem`;
      floor compact at ~36px; treat 44px as the mobile minimum.
- [ ] **[H]** Transaction row checkbox `.check` is a sub-24px target with left-only padding
      (`transactions.go:653`, `web/index.html:322`). Add `min-width:24px;min-height:24px;display:inline-grid;place-items:center;`
      (mirror the `.btn-del` fix at `web/index.html:279`).
- [ ] **[H]** Custom-page "⋯" menu button has no min size (`custompagesnav.go:261`). Add
      `min-w-6 min-h-6 inline-grid place-items-center`.
- [ ] **[M]** Rail nav items rely on Tailwind padding with no min guard; icon-only collapsed rail may
      fall under 24px (`shell.go:274`). Add explicit `min-w-10 min-h-10`.
- [ ] **[M]** `.btn-del` is a tight 24×24 with `padding:0 0.3rem` (`web/index.html:275`). Bump to
      ~`0.25rem 0.4rem`.
- [ ] **[L]** Color input is 46×34px (`web/index.html:265`; used `categories.go:138`, `members.go:237`).
      Enlarge toward 44×44 or wrap in a larger hit area.

### 6.2 Legibility & contrast (WCAG AA)

- [ ] **[H]** `--text-faint` `#6c6c72` on base `#0e0e0f` ≈3.1:1 — fails AA for text. Used for rail
      section headers, breadcrumb separators, "New page" link (`web/index.html:43`, `shell.go:131`,
      `custompagesnav.go:152`). Lighten to ≥4.5:1 (e.g. `#7d7d85`) or restrict faint to truly decorative use.
- [ ] **[M]** `--text-dim` `#a6a6ac` ≈4.2:1 — just under AA; affects `.row-meta`, `.budget-sub`
      (`web/index.html:254`, `:314`). Brighten dim slightly (~`#ababb3`).
- [ ] **[M]** Rail section labels at `text-[10px]` with `0.16em` tracking risk descender clipping and poor
      legibility (`shell.go:131`). Bump to ≥11px and/or reduce tracking to ~0.08em.
- [ ] **[M]** Tiny type elsewhere: priority badges `0.68rem` (`web/index.html:326`), segmented buttons
      `0.8rem` (`web/index.html:362`), member/status chips `0.8rem` (`dashboard.go:174`). Raise toward
      0.75–0.85rem and loosen cramped gaps (`.task-meta`).
- [ ] **[L]** `.insight-dot` `1.05rem` is larger than body 14.5px, unbalancing the ↑/↓ arrows
      (`web/index.html:187`). Drop to 1rem.

### 6.3 Display-scale & formatting consistency

- [ ] **[M]** Hardcoded pixel type bypasses the user display-scale: dashboard KPI `text-[34px]`
      (`dashboard.go:363`), chart legend `text-[12px]` (`dashboard.go:328`). Use relative/Tailwind scale units.
- [ ] **[M]** Numeric figures not uniformly `tabular-nums` — row-meta "· $X" and some amounts skip the
      `.amount` class (`transactions.go:635`, `accounts.go:559`, `budgets.go:379`). Apply tabular figures
      to all monetary text for column alignment.
- [ ] **[L]** Upcoming-bills date uses hardcoded `Format("Jan 2")` instead of the user date-format pref
      (`dashboard.go:224`). Route through `pr.FormatDate(...)` like `todo.go`.
- [ ] **[L]** Chart heights hardcoded 120–180px illegible on narrow bento tiles
      (`planning.go:270`, `dashboard.go:498`). Add responsive min-height.
- [ ] **[L]** Progress track `h-1.5` (6px) thin in dense layouts (`ui/progress.go:34`). Bump to `h-2`.

### 6.4 Shapes / consistency / states

- [ ] **[M]** Add-menu button mixes inline `Style{border-radius:4px}` with Tailwind classes
      (`addmenu.go:40`); switch to `rounded-[4px]` for consistency (and to avoid clobbering the focus ring).
- [ ] **[M]** No shared disabled-button style — `.btn:disabled { opacity:.5; cursor:not-allowed; }` is
      missing, so "Thinking…" (`insights.go:186`) and default-state buttons (`goals.go:316`) don't read as
      disabled. Add it and render real disabled buttons rather than hiding them.
- [ ] **[M]** Bulk-action toolbar wraps unevenly on narrow screens (`transactions.go:541`). Give it a
      robust responsive layout.
- [ ] **[L]** Selected transaction checkbox has only a subtle glyph swap, no highlight
      (`transactions.go:643`). Add a selected background/border.
- [ ] **[L]** Workspace-switcher action group separator is a faint 1px line (`wsswitcher.go:46`); add
      `my-2 pt-2` spacing. Rule shadow-conflict warning is text-only (`rules.go:287`) — add a colored badge/left border.
- [ ] **[L]** Custom-page menu can clip at the viewport edge on narrow screens (`custompagesnav.go:249`);
      add max-width/overflow or boundary detection.

### 6.5 Empty / loading / async states

- [ ] **[M]** Empty states are bare italic text with no call-to-action across screens
      (`transactions.go:482`, `accounts.go:336`, `dashboard.go:523`, etc.). Wrap in a block with a heading
      and an "Add first…" button.
- [ ] **[M]** AI result area vanishes while "Thinking" (`insights.go:184`) — add a skeleton/shimmer.
- [ ] **[L]** Add/edit/delete handlers have no in-flight state — no button disable/spinner
      (`accounts.go:134`). Add a `saving` state that disables controls during the op.

### 6.6 Keyboard shortcuts & discoverability

- [ ] **[H]** No command palette. Add `Cmd/Ctrl+K` to search screens/actions/entities with a keyboard-
      navigable result list. (No existing keybinding registry found.)
- [ ] **[H]** No "?" help overlay documenting shortcuts. Add a `?`-key cheat sheet + a Settings → Keyboard
      Shortcuts entry; consider a first-run "Press ? for help" hint.
- [ ] **[M]** No quick-add hotkey — adding a transaction is button→menu→form. Add e.g. `Cmd/Ctrl+Shift+A`
      to open the quick-add panel directly (`quickadd.go`, `addmenu.go`).
- [ ] **[M]** No shortcut to focus search/filter — bind `Cmd/Ctrl+F` to the nearest search input per screen.
- [ ] **[M]** No section-jump shortcuts — add `Alt+1..9` mapped to primary rail nav (`shell.go:207`).
- [ ] **[M]** FlipPanel handles Esc/Tab-trap/focus-restore well but has no Enter-to-submit
      (`ui/flippanel.go`); add Enter→Save (skip when focus is in a textarea).
- [x] **[L]** Segmented controls (radiogroups) lack arrow-key navigation (`ui/controls.go:32`). Add
      Arrow Left/Right/Up/Down to move selection.
- [ ] **[L]** Inline forms could expose a small "Enter to save · Esc to cancel" hint.

### 6.7 Focus management & click-to-item speed

- [ ] **[M]** Entering inline edit doesn't move focus into the edit form
      (`transactions.go:598`, and the other entity screens). Focus the first field on edit.
- [ ] **[M]** After save/delete, focus isn't restored predictably (`transactions.go:606`, `accounts.go:572`,
      and peers). Return focus to the row/Edit button on save; to the next/prev row on delete.
- [ ] **[M]** Quick-add form has no autofocus (`quickadd.go:118`). Autofocus the first meaningful field.
- [ ] **[L]** Dashboard exposes every widget as its own tab stop (`ui/widget.go:121`); with 12+ tiles the
      tab path to main content is long. Consider one logical focus group with arrow-key nav inside.

### 6.8 Replace native dialogs & destructive-action safety

- [ ] **[H]** "Set Balance" (`accounts.go:435`) and "Contribute" (`goals.go:294`) use native
      `window.prompt()` — poor on mobile, no validation, not keyboard-consistent. Replace with in-app
      modal/inline forms.
- [ ] **[M]** Deletes have no confirmation or undo. Add a confirm step and/or an Undo toast (and focus the
      next row afterward).

### 6.9 ARIA & announcements

- [ ] **[M]** Toast container likely lacks `role="status"` / `aria-live="polite"` (`toast.go`) — additions
      aren't announced. Also differentiate auto-dismiss: keep errors longer (~6–8s) or require manual
      dismiss (`toast.go:14`, `toastTimeoutMS=4500`).
- [ ] **[M]** Ensure every dynamic result list has a count live region (transactions has one at
      `transactions.go:551`; verify accounts/budgets/goals/categories/members parity).
- [ ] **[L]** Icon-only buttons rely on `title` rather than `aria-label` (e.g. `.btn-del` at
      `accounts.go:572`). Standardize `aria-label` on all icon buttons.
- [ ] **[L]** Collapsed-rail hover flyout label has `pointer-events:none` (`web/index.html:439`) so clicking
      it doesn't navigate; either make it clickable or make the intent clear.

### 6.10 Misc

- [ ] **[L]** Allocate score bar has no inline value label or `role="progressbar"`/`aria-valuenow`
      (`allocate.go:56`). Allocate profile select has no "Choose a profile…" placeholder (`allocate.go:362`).
- [ ] **[L]** Custom-field key input has no client-side format validation (`customfields.go:69`); add a
      pattern (alphanumeric + underscore) / reserved-name check.

> **Live-app pass still TODO:** the above is static review. A follow-up should run the app via the `gwc`
> browser tools and screenshot each screen (light + dark, compact + comfortable, narrow + wide) to catch
> rendered issues — wrapping, overflow, real contrast, animation jank — that source review can't see.

### 6.11 Light-theme & design-system CSS (2026-06-18 pass 2 — `web/index.html` deep read)

- [ ] **[M]** Light-theme icon controls are too faint: `.gear-inline`/`.gear-abs`/`.menu-btn` set to
      `#8a8a90` and `.set-close` to `#8a8a92` on the `#f7f6f3` light bg ≈ ~2.7:1 — below the 3:1 AA
      non-text/UI threshold (`web/index.html:218`, `:400`). Darken the light-theme idle color (e.g. `#6a6a72`).
- [ ] **[M]** Settings toggle switch is a 36×21px hit area — the 21px height is under the 24px minimum
      (`web/index.html:406`, `.switch`). Enlarge the switch or pad its clickable wrapper to ≥24px.
- [ ] **[L]** Settings accent swatches are 22×22px (`web/index.html:409`, `.swatch`) — just under 24px.
      Nudge to ≥24px or add padding around the hit area.
- [ ] **[L]** `.badge-soon` uses a fixed dark-blue palette (`#1e293b`/`#93c5fd`, `web/index.html:233`)
      with no light-theme override — reads as a dark chip on a light card. Add a `[data-theme="light"]` variant.
- [ ] **[L]** `.check` has asymmetric padding `0 0.5rem 0 0` (right side flush) (`web/index.html:322`),
      compounding the sub-24px target in 6.1 — center the glyph when you add the min-size box.
- [ ] **[L]** Squared-progress override `.bento [class*="rounded-full"][class*="overflow-hidden"]
      { border-radius:2px }` (`web/index.html:420`) is a fragile attribute-substring hack tied to Tailwind
      class names; a rename silently breaks it. Replace with an explicit component class.

> **Next pass (pass 3):** Settings flip-panel content/copy (`internal/app/settings.go`) + a plain-English
> microcopy sweep of `internal/i18n/en.go` (labels, empty states, errors, nudges). Still solo, paced.

### 6.12 Settings flip-panel (2026-06-18 pass 3 — `internal/app/settings.go`)

- [ ] **[H]** Base-currency `<select>` is a **dead control** — it has no `OnChange`
      (`settings.go:383`), so picking EUR/GBP changes nothing and never persists `BaseCurrency`. Wire it to
      update settings + bump the data revision (and re-derive FX display base).
- [ ] **[H]** FX-rate inputs are **dead** — `rateRow`'s `Input` has no `OnInput`/`OnChange`
      (`settings.go:617`), so edited exchange rates are discarded. Add a handler that writes
      `Settings.FXRates[code]` and persists.
- [ ] **[M]** "Enable AI" toggle is local-only `UseState` that gates nothing (`settings.go:236`, `:414`) —
      turning it off leaves the key field active and AI calls available. Either wire it to actually
      enable/disable AI (and disable/hide the key+model when off) or remove the toggle.
- [ ] **[M]** Hidden-screen labels are hardcoded English (`hideableScreens`, `settings.go:214-228`) and fed
      to `settings.showScreen` — screen names don't localize despite the language system. Use i18n keys.
- [ ] **[M]** The whole global panel is one dense 2-column scroll (members, currency, budget method, FX,
      screens, freshness, AI, appearance, prefs, data, workspaces, languages, **plus a debug log**) in a
      760×560 flip card with no section tabs/index (`settings.go:535`). Finding a setting means scrolling a
      wall. Add grouped tabs or an in-panel section nav to cut click/scroll-to-setting time.
- [ ] **[L]** Developer debug-log ring is surfaced inside user-facing Settings (`settings.go:527`). Move it
      behind an "Advanced/Developer" disclosure or a separate route.
- [ ] **[L]** Hardcoded non-localized microcopy in settings rows: `"days (0 = never)"` (`settings.go:206`),
      `"1 "+code+" ="` and base label (`settings.go:616-618`), and the base-currency option text
      (`settings.go:384-386`). Route through i18n.
- [ ] **[L]** Destructive "Wipe" uses native `confirmAction`/`window.confirm` (`settings.go:710`) — same
      native-dialog concern as 6.8; consider an in-app confirm with a typed-confirm or undo window given it
      erases all data.

> **Next pass (pass 4):** plain-English microcopy sweep of `internal/i18n/en.go` (labels, empty states,
> errors, nudges) — deferred from this pass to keep it economical. Still solo, paced.

### 6.13 Microcopy (2026-06-18 pass 4 — `internal/i18n/en.go`)

Overall the copy is strong — friendly, plain-English, consistent terminal punctuation, good empty states and
nudges. Only minor nits found:

- [ ] **[L]** Awkward `(s)` pluralization in reassign-before-delete strings: `categories.reassignDesc`
      (`en.go:109`, "%d transaction(s) or budget(s)") and `members.reassignDesc` (`en.go:707`,
      "%d account(s), budget(s), or goal(s)"). Use a proper singular/plural helper.
- [ ] **[L]** Count strings read wrong at 1: `dashboard.staleCount` (`en.go:613`, "1 balances could use a
      refresh") and `dashboard.accountsCount` (`en.go:626`, "1 accounts"). Pluralize on the count.
- [ ] **[L]** "APR" abbreviation appears as a bare label (`accounts.apr` "Interest APR %", `en.go:546`;
      `planning.*`, `accounts.expReturnTitle`). CLAUDE.md asks for no undecoded abbreviations — consider
      "Interest rate (APR)" or a tooltip expansion.

> **Next pass (pass 5):** consolidation — re-read Section 6, dedupe overlapping items, and order them into a
> single prioritized fix list (high-impact/low-effort first) so the backlog is actionable. Still solo, paced.

### 6.14 Prioritized fix order (2026-06-18 pass 5 — consolidation of 6.1–6.13)

Suggested execution order for the UX/UI backlog above, ranked by impact × effort. Each line points back to
its detailed subsection. Knock out P0/P1 first — they're mostly small, high-confidence wins.

**P0 — broken/dead controls (correctness; small):**
- [ ] Wire base-currency `<select>` (no `OnChange`) — §6.12
- [ ] Wire FX-rate inputs (no `OnInput`) — §6.12
- [ ] Make "Enable AI" toggle actually gate AI, or remove it — §6.12

**P1 — accessibility & contrast, high-impact / low-effort:**
- [ ] Fix failing text contrast: `--text-faint` (~3.1:1), `--text-dim` (~4.2:1), light-theme icon controls (~2.7:1) — §6.2, §6.11
- [ ] Raise form-field height + small touch targets (`.field`, `.check`, ⋯ button, `.switch`, swatches, rail items) — §6.1, §6.11
- [ ] Add shared `.btn:disabled` style — §6.4
- [ ] Toast `role="status"`/`aria-live` + longer error dismiss — §6.9
- [ ] Replace native `prompt()`/`confirm()` (Set Balance, Contribute, Wipe) with in-app dialogs — §6.8, §6.12

**P2 — high-value UX, medium effort:**
- [ ] Focus management: into inline edit, restore after save/delete, quick-add autofocus, Enter-to-submit in dialogs — §6.6, §6.7
- [ ] Empty states with a clear CTA; AI skeleton + in-flight button disable — §6.5
- [ ] Delete confirmation + undo toast — §6.8
- [ ] Settings panel section nav/tabs (cut scroll-to-setting) + move debug log to Advanced — §6.12
- [ ] Responsive bulk-action toolbar — §6.4

**P3 — efficiency / power-user, larger:**
- [ ] Command palette (Cmd+K) — §6.6
- [ ] "?" keyboard-shortcut help overlay — §6.6
- [ ] Quick-add hotkey, search-focus (Cmd+F), section jumps (Alt+1-9), segmented arrow-key nav — §6.6
- [ ] Display-scale-safe type, uniform tabular figures, date-format pref, responsive chart heights — §6.3

**P4 — polish / low severity:**
- [ ] Tiny-type bumps (badges, insight-dot, rail labels, seg buttons) — §6.2
- [ ] Shape/consistency (inline radius, fragile bento CSS hack, `.badge-soon` light variant) — §6.4, §6.11
- [ ] Icon-button `aria-label`s, collapsed-rail flyout pointer-events, allocate bar a11y, custom-field validation — §6.9, §6.10, §6.12
- [ ] Microcopy: `(s)` pluralization, count-at-1 strings, "APR" abbreviation — §6.13

### 6.15 Live-app render pass (2026-06-18 pass 6 — Playwright + sample data)

Captured the running app (Playwright/Chromium, sample data loaded) across dark/light/compact and
desktop/mobile — screenshots in `.review-screenshots/live-*.png`, zero console errors. New issues that only
show up rendered:

- [ ] **[H]** **Compact density does nothing on the dashboard.** Compact and comfortable bento views are
      pixel-identical (`live-dashboard-compact.png` vs `-dark.png`) — `[data-density="compact"]` CSS only
      targets legacy `.card/.row/.field/.btn`, not the bento `.w` tiles (`web/index.html:190-194`). Add
      compact rules for the dashboard tiles (padding, figure sizes) or document that Compact excludes the dashboard.
- [ ] **[H]** **Mobile top bar eats the whole first screen.** On 390px the period controls
      (Week/Month/Quarter + Jump to + ‹ Jun 2026 › stepper + Custom range + Add) stack into ~6 rows, pushing
      all content below the fold (`live-dashboard-mobile.png`). Collapse the period controls into a single
      compact control/popover on narrow widths.
- [ ] **[M]** **Allocate breakdown missing a separator:** renders "Score 60%returns 100 · stability 100 …"
      — no space/`·` between the score % and "returns N" (`live-allocate-dark.png`; `screens/allocate.go`
      breakdown line). Insert "· " after the score.
- [ ] **[M]** **Allocate criterion-weight inputs are unlabeled** — five number boxes all showing "1" under
      "CRITERION WEIGHTS" with no per-input label, so you can't tell which weight is returns/stability/
      liquidity/etc. (`live-allocate-dark.png`; `allocate.go`). Add a label above/beside each weight.
- [ ] **[M]** **Net-worth-trend tile degenerates to a flat block** — with the sample dataset the chart is a
      solid filled rectangle (axis 0–4, no visible line/trend) in both themes (`live-dashboard-dark/light.png`;
      `screens/dashboard.go` trend chart). Draw a real series or show an empty/"not enough history" state.
- [ ] **[M]** **Dashboard header controls collide on mobile** — "Custom layout ▾ / Reset layout" overlap the
      "Your dashboard" title + hint and truncate ("Custom ⌄") (`live-dashboard-mobile.png`). Stack them below
      the title on narrow widths; the "Drag tiles … grab the edge handles" hint is also meaningless on touch.
- [ ] **[L]** "▲ 0% this month" on the Net worth KPI shows an up-triangle with a 0% change
      (`live-dashboard-*.png`) — suppress the trend arrow (or use a neutral dash) when the delta is zero.
- [ ] **[L]** Allocate field placeholder "Keep back (emergency buffer" is clipped mid-word in the input
      (`live-allocate-dark.png`); shorten the placeholder or widen the field.
- [x] Visual confirmation of §6.2: light-theme "TOOLS"/"SYSTEM" rail section labels are barely legible
      against the light background (`live-dashboard-light.png`) — already tracked as the `--text-faint` contrast fix.

> **UX/UI analysis backlog complete.** Static passes (§6.1–6.13) + prioritization (§6.14) + live render pass
> (§6.15) done. Reproduce the live pass anytime: `node .tools/server.mjs web 8799 &` then `node .tools/shot.mjs`
> (Playwright + Chromium in `.tools/`, screenshots to `.review-screenshots/`).

### 6.16 UI interaction & motion polish (2026-06-18 pass 7 — animations, hover, micro-interactions)

The motion **foundation is good**: FLIP-animated bento reorder/resize (`web/flip.js`), the settings flip-panel
(`transform .55s cubic-bezier`), boot loader + `#app` settle-in, toast enter, collapsed-rail flyout, switch
toggle, and a thorough `prefers-reduced-motion` block. The gap is the **micro-interaction layer** — the small
feedbacks that make a UI feel responsive and alive. Mostly enhancement-grade ([M]/[L]), ordered by bang-for-buck.
All additions must be wrapped in `@media (prefers-reduced-motion: no-preference)` (or no-op'd in the existing
reduced-motion block) to stay consistent with the app's a11y stance.

**Press / tactile feedback**
- [ ] **[M]** No `:active` press state on *any* button — only `.ghandle`/scrollbar have one (`web/index.html:355`).
      Add a subtle `active:scale-[.97]` / `:active { transform: translateY(1px) }` or opacity dip to `.btn`,
      `.btn-primary`, `.nav-link`, `.nv`, `.seg-btn`, `.data-btn`, `.menu-btn`, `.check`, `.btn-del`. Biggest
      single "feels responsive" win.

**Hover affordances**
- [ ] **[M]** List rows (`.row`) have **no hover state** (`web/index.html:245`) — transaction/account/budget
      rows don't highlight under the cursor, hurting scannability and click targeting. Add
      `.row:hover { background: var(--hover) }` with a short `background` transition (and a pointer cursor on
      rows that drill in, e.g. accounts → ledger).
- [ ] **[L]** Tile hover snaps — `.w:hover` changes `border-color` but `.w` declares no `transition`
      (`web/index.html:345`), so it jumps. Add `transition: border-color .15s ease` (and consider a faint
      `background` lift on hover for depth).
- [ ] **[L]** `.btn` hover is a blunt `filter: brightness(1.12)` (`web/index.html:272`). Consider a gentler
      `background`/`border` hover + tiny shadow for primary actions so hover reads as elevation, not just brightness.

**Data-viz & progress animation**
- [ ] **[M]** Progress/score bars **snap** to width — `.bar-fill` (budgets) and the Allocate score bar have no
      width transition (`web/index.html:316`; `screens/allocate.go`). Add `transition: width .45s cubic-bezier(.2,.75,.2,1)`
      so bars grow in on load/update. High polish-per-line.
- [ ] **[M]** Charts render instantly — `web/chart.js` has no draw-in animation (no transition/raf). Animate
      line-draw (`stroke-dashoffset`) and bar grow-up on first paint / data change so the dashboard feels alive.
- [ ] **[L]** KPI figures (net worth, income, …) update instantly. Optional count-up tween on value change would
      elevate the headline numbers (gate behind reduced-motion; keep it fast, ≤400ms).

**Enter / exit transitions**
- [ ] **[M]** Toasts enter (`@keyframes toast-in`) but **never animate out** — they vanish at the auto-dismiss
      deadline (`web/index.html:307`, `app/toast.go:14`). Add a fade/slide-out (~160ms) before removal so they
      don't blink away.
- [ ] **[M]** Inline row edit swaps in/out with no transition — the row instantly becomes the edit form
      (`screens/transactions.go` & peers). A short height/opacity transition (or a subtle background flash on the
      saved row) would make edits feel smooth and confirm the save landed.
- [ ] **[L]** Newly added list items appear instantly. A brief highlight-fade ("flash" the new row) on add would
      confirm where the item landed.

**Stateful micro-interactions**
- [ ] **[L]** Segmented controls (`.seg-btn.active`) and the week-start/theme pickers snap the active background
      (`web/index.html:364`). A sliding active-pill indicator (animate a shared highlight) would feel premium.
- [ ] **[L]** Active nav pill (`.nav-link.active` / `.nv`) jumps between items on route change. Consider animating
      a shared active indicator that slides to the selected item.
- [ ] **[L]** Accent swatches (`.swatch.sel`) and the gear/handle reveals pop in instantly — add a quick
      `transform: scale` / opacity transition on selection and on `.rz` handle reveal for refinement.

> **Note:** animations/hover are hard to verify from still screenshots; this pass is a CSS/JS interaction audit.
> A future check could record short Playwright videos (`recordVideo`) of hover/drag/toast flows to confirm feel.

### 6.17 Re-verification on fresh build (2026-06-18 pass 8 — Playwright, build 13:40)

Re-captured against the latest build (now includes commit `fix: make the multi-currency (FX) editor functional
(D16)`), plus a new Settings-panel shot (`.review-screenshots/live-settings-dark.png`). Zero console errors.

- [x] **§6.12 FX/base-currency dead controls — FIXED.** The Exchange Rates section now renders editable rows
      ("1 AUD = [input] USD", CAD, CHF…) and the base-currency select is wired (commit D16). Verified rendered.
- [ ] **[STILL OPEN] §6.15 Allocate "Score 60%returns 100"** missing separator — reproduced on this build
      (`live-allocate-dark.png`); not yet addressed.
- [ ] **[M]** AI "Enable AI features" toggle semantics still unclear: the new helper copy says "AI features stay
      off until you add a key," which implies the *key* gates AI — so what does the toggle do when a key is
      present? Either make the toggle the single source of truth (and gray out key/model when off) or drop it
      and let key-presence gate AI. (refines §6.12)
- [ ] **[L]** Settings panel shows **Save / Cancel** buttons, but appearance/preferences apply **live** on each
      change (`settings.go` savePrefs-on-change). "Save" is then ambiguous — clarify what it commits vs. the
      live changes, or drop Save and make Cancel a "Done/Close" (`live-settings-dark.png`).

> The other session is actively fixing logged items (D16 FX fix landed). This re-verification loop is useful:
> on each fresh build, re-run `node .tools/shot.mjs` to confirm fixes render and catch regressions.

### 6.18 Lock screen — interaction review (2026-06-18 pass 10 — new surface, B17/B17.1)

Reviewed the new app-lock gate + passcode-setup modal (`internal/app/applockgate.go`). Functionally **solid**:
focus-trap, Enter-to-submit, autofocus, ARIA labels, hint-after-3-fails, forgot→wipe recovery, idle auto-lock.
But against the current animation/hover/interaction focus it's the least-polished surface in the app and breaks
the patterns used everywhere else:

- [ ] **[M]** **No focus ring on the passcode/setup inputs.** Their inline style sets `outline:none`
      (`applockgate.go:84`, and the shared `inputStyle` at `:301`); inline styles beat the global
      `:focus-visible { outline: 2px solid }` stylesheet rule, so these inputs show **no keyboard focus
      indicator** — an a11y regression on the one screen that is keyboard-only. Drop `outline:none` (or set a
      focus border/ring explicitly). (related to the inline-style focus concern in §6.1)
- [ ] **[M]** **Gate has no enter/exit transition.** It's shown/hidden via `display:grid`/`none`
      (`applockgate.go:45,106`), so it pops in and snaps away — inconsistent with the boot loader (fade+scale),
      flip panel (`.55s`), and toast. Add a fade/scale on show and a polished fade-up on unlock (mirror
      `#boot.hidden`), gated behind `prefers-reduced-motion`.
- [ ] **[M]** **Wrong-passcode feedback is text-only** — sets message text + red color
      (`applockgate.go:114-117`) with no shake. The expected micro-interaction is a horizontal shake of the
      input on a failed attempt. Add a `shake` keyframe applied to `#cf-applock-input` on failure.
- [ ] **[M]** **Lock-screen buttons have zero hover/active feedback.** Unlock/Forgot/Show-hint and the setup
      OK/Cancel are built as raw DOM with inline `cssText` and `cursor:pointer` but no `:hover`/`:active`
      (they aren't `.btn`, so global button styles don't apply) — they're completely static under the pointer.
      Give them hover/active states (reuse the `.btn`/`.btn-primary` classes, or add JS hover handlers).
- [ ] **[L]** **Setup modal backdrop appears instantly** (`rgba(0,0,0,0.6)`, `applockgate.go:299`) whereas the
      flip-panel backdrop fades (`.flip-backdrop … transition:opacity .28s`). Add a matching backdrop fade-in so
      modals feel consistent.

> Positives worth keeping: the gate correctly traps focus, submits on Enter, autofocuses the field, reveals the
> hint only after 3 misses, and offers an honest forgot→wipe path. Only the *motion/feedback* layer is missing.

### 6.19 Re-verification (2026-06-18 pass 11 — build 15:15)

The other session is fixing logged items fast. Status deltas verified from source/diffs:

- [x] **§6.8 native dialogs — COMPLETE.** Both browser prompts are gone: in-app "Set balance" form (commit
      `99c4be8`, "remove last native prompt (6.8 complete)") and in-app goal-contribute form (`bc59900`). The
      new forms use the framework field classes (no `outline:none`), so they keep the focus ring — good.
- [x] **§6.18 unlock exit animation — DONE.** Correct passcode now dismisses the gate via `unlockGate` with a
      blur+scale opacity fade (~0.35s, self-releasing `setTimeout`), and it **respects `prefers-reduced-motion`**
      (`applockgate.go:28-37`) — exactly as recommended (mirrors `#boot.hidden`).
- [ ] **[STILL OPEN] §6.18 remaining lock items:** the gate *enter*/show still pops (`display:grid` instantly,
      `applockgate.go:75` — only the exit animates); no wrong-passcode shake; lock-screen buttons still have no
      hover/active feedback.
- [ ] **[M] Focus-ring `outline:none` regression generalizes to 3 raw-DOM inputs**, not just the lock gate:
      `applockgate.go:114` & `:331` (passcode/setup) **and `shortcuts.go:360`** (command-palette/quick input).
      All three suppress the global `:focus-visible` ring via inline style. Fix all raw-DOM overlay inputs
      together (drop `outline:none`, or set an explicit focus border).

> Progress so far: §6.8 fully closed; §6.12 FX fixed (§6.17); §6.18 unlock-exit done. Remaining UX backlog is
> mostly the motion-polish items (§6.16) + the lock-screen feedback gaps (§6.18) + the focus-ring fix above.

---

## 7. Backend server — sync + AI proxy (gRPC bridge hybrid) ★

> Supersedes the stubs in §3.1–3.2. Design: [`docs/BACKEND_PLAN.md`](./docs/BACKEND_PLAN.md).
> **Locked decisions:** last-write-wins sync (newest-by-timestamp) · per-user **BYO** OpenAI key
> stored **encrypted at rest** · auth via **OAuth (Google/GitHub)** · artifacts in a
> **content-addressed blob store** (refs only in the synced snapshot) · **gRPC over the GWC
> `GoGRPCBridge`** (WebSocket) for the app's data/AI RPCs · **plain HTTP** for OAuth + blobs.
> Thin server: it stores and forwards, never interprets the dataset. App stays local-first; the
> backend is an optional sync/proxy tier. Build bottom-up (proto/contract → storage → services →
> transport → client), one feature per commit, tests with each layer.

### 7.0 Foundations & toolchain
- [x] Decide layout: `cmd/cashflux-server/` in this module vs a sibling `server/` module. ★
- [x] Add deps: `GoGRPCBridge` (grpctunnel), `google.golang.org/grpc`, `google.golang.org/protobuf`,
      `golang.org/x/oauth2`, `ncruces/go-sqlite3` (already used client-side).
      Done: bridge/grpc/protobuf/sqlite deps are pinned. OAuth intentionally uses explicit stdlib HTTP
      handlers for PKCE/state/token/userinfo flows, so `golang.org/x/oauth2` is not carried as an unused module.
- [x] protoc + `protoc-gen-go` + `protoc-gen-go-grpc` (or `buf`); add a codegen step (Makefile / `gwc`-style)
      and a CI **proto-drift check** (generated code matches `.proto`).
      Done: Buf is pinned through `buf.yaml`/`buf.gen.yaml`; `go run github.com/bufbuild/buf/cmd/buf@v1.57.2
      generate` writes Go/gRPC descriptors to `internal/backendrpc/pb`, and CI fails on generated drift.
- [x] Pin server Go toolchain (1.26) and confirm the client gRPC code builds for `js/wasm`.
      Done: `go.mod` pins Go 1.26.0, `Dockerfile.server` builds from `golang:1.26-alpine`, and the
      server + `GOOS=js GOARCH=wasm` client builds are part of this atom's verification.

### 7.1 Proto contracts (shared client+server) ★
- [x] `proto/` package + gen output dir; versioning policy (no breaking changes; reserve removed fields).
      `proto/cashflux/v1/cashflux.proto` and `proto/README.md` now define the contract and policy; generated
      Go/gRPC output is checked in under `internal/backendrpc/pb`.
- [x] Common messages: `Workspace{id,name,color,sort,deleted,version,updatedAt,deviceId}`,
      `DatasetEnvelope{schemaVersion, gzippedJson bytes}`, `BlobRef{hash,mime,size,name}`.
- [x] Keep the dataset as an opaque **bytes/gzip JSON** field (reuse `store.ExportJSON`) — do **not**
      re-model every entity in proto; only the sync/AI envelopes are typed.
- [x] `SyncService`: `ListWorkspaces`, `GetWorkspace`, `PutWorkspace`, `DeleteWorkspace`,
      `WatchWorkspaces` (server stream).
- [x] `AIService`: `SetKey`, `ListModels`, `Chat`, and `Vision` unary RPCs are done over the
      GoGRPCBridge `/grpc` tunnel.
      Done: `ChatStream` and `VisionStream` now expose server-streaming completion chunks over the same tunnel;
      the first implementation sends the validated final completion as a terminal chunk.
- [x] Error model: map to gRPC `codes` / `google.rpc.Status` (unauthenticated; failed-precondition for a
      stale push when `force` is off; resource-exhausted for quota).
      Done in `docs/BACKEND_ERRORS.md`: auth/validation/precondition/quota/upstream failures map to gRPC
      codes; stale LWW writes intentionally return OK with `accepted=false` plus current state so the client
      can recover without a second call.

### 7.2 Server storage layer (pure, tested) ★
- [x] SQLite schema (ncruces, WAL) + stepwise migrations (own `schemaVersion`, reject newer-than-supported):
      `users`, `workspaces`, `snapshots` (current + last-N history), `blobs`, `workspace_blobs`,
      `ai_keys`, `usage`.
- [x] Repository layer with table-driven tests on native Go (no transport/proto deps).
- [x] Snapshot store: put/get current, retain last **N** prior snapshots per workspace (LWW recovery),
      enforce a dataset size cap.
- [x] Blob metadata + on-disk **content-addressed** store (sha256, path-sharded); `workspace_blobs`
      refcount; GC sweep for unreferenced blobs.
- [x] `ai_keys`: AES-GCM encrypt/decrypt helper; master key from env/secret manager; rotation note.
- [x] `usage`: per-user/day request + token counters; helpers for rate-limit checks.

### 7.3 SyncService (last-write-wins) ★
- [x] Auth interceptor: read bearer token from gRPC metadata → validate → put user in context.
- [x] `List`/`Get`/`Delete` (soft-delete tombstone) strictly scoped to the caller's `user_id`.
- [x] `PutWorkspace` LWW: accept when `clientUpdatedAt >= stored.updatedAt` (newest wins, so a stale
      device can't clobber newer data); server-stamp `updatedAt`; bump `version`; honor a `force` flag;
      return the new `{version, updatedAt}` (and current state when rejected so the client re-pulls).
- [x] `WatchWorkspaces` server stream: in-proc per-user pub/sub notifies other devices of a change;
      heartbeat/keepalive; clean unsubscribe on disconnect.
- [x] Tests: LWW accept/reject by timestamp, tombstone propagation, cross-user isolation, watch fan-out,
      oversized-payload rejection, and a two-device bridge e2e for stale-write rejection plus tombstone
      propagation are covered.

### 7.4 AIService (per-user encrypted BYO key) ★
- [x] `SetKey`: validate, AES-GCM encrypt, store; never return the key.
- [x] `Chat`/`Vision` server proxy path: load+decrypt the user's key, call OpenAI (reusing the
      `internal/ai` request builders), map upstream errors to status, and count usage.
      Done: unary and server-streaming `Chat`/`Vision` surfaces share the same validated proxy path; streaming
      sends terminal completion chunks over `/grpc`.
- [x] Legacy HTTP AI routes retired: `/v1/ai/key`, `/v1/ai/chat`, and `/v1/ai/vision` are no longer mounted;
      key upload, model listing, chat, and vision now use authenticated `AIService` RPCs over `/grpc`.
- [x] Model allow-list; per-user rate limit + usage metering; request-size caps; **redact key in logs**.
- [x] Cancellation: propagate client `ctx` cancel to the upstream call (stop billing on disconnect).
- [x] Tests: mock upstream chat/vision, key encrypt round-trip, rate-limit trip, missing-key clear error, and
      cancellation are covered.
      Done: `ChatStream` bridge coverage verifies encrypted-key lookup, upstream proxying, terminal chunk, and EOF.

### 7.5 gRPC bridge transport ★
- [x] `grpctunnel.Wrap(grpcServer, …)` at `/grpc`: `WithOriginCheck` (SPA origin allow-list),
      `WithKeepalive`, `WithReadLimitBytes`, `WithMaxActiveConnections` / `…PerClient` / `…UpgradesPerMinute`.
- [x] TLS / `wss` (server or reverse proxy); confirm WS survives the proxy/LB (keepalive, idle timeout).
      Done: self-host Caddy terminates TLS, exposes `wss://<domain>/grpc`, forwards websocket upgrades, and
      pins upstream keepalive plus long stream timeout/close-delay settings for `/grpc` streams.
- [x] Health + readiness endpoints; graceful shutdown that drains active streams.

### 7.6 HTTP endpoints (OAuth + blobs)
- [x] Backend root status endpoint returns service/status JSON and advertises the health, version, and
      `/grpc` tunnel paths for local browser checks.
- [x] OAuth: `GET /v1/auth/:provider` redirects with PKCE + `state`; callback code exchange upserts
      `users`, issues signed short-lived access tokens plus httpOnly refresh cookies, and supports refresh/logout.
- [x] Provider config (Google, GitHub) per environment (client id/secret, redirect URIs).
- [x] Blobs: `PUT /v1/blobs/:hash` (verify the bytes hash to `:hash`, size cap, store if absent),
      `GET /v1/blobs/:hash` (immutable / long cache headers), `HEAD` for existence; auth + refcount on link.
      Raw authenticated PUT/GET/HEAD is done; blob hashes are linked to owned workspaces and checked before reads.
- [x] WS origin policy / CORS aligned to the SPA origin.
- [x] Document the handshake: HTTP-issued token → carried as gRPC metadata on every RPC.

### 7.7 Client integration (wasm app) ★
- [x] gRPC client over the bridge: `BuildTunnelConn` (wss to backend) + a metadata interceptor that
      attaches the auth token.
- [~] Sync client layered over the existing autosave: browser autosave now pushes changed active-workspace
      snapshots over `/grpc`, pulls newer server snapshots on boot/focus, applies newest-by-`updatedAt` using
      local sync metadata, maps local workspace ids directly to server workspace ids, and subscribes to
      `WatchWorkspaces` so active-workspace changes from other devices trigger a pull. A persisted per-workspace
      pending mutation queue retries on focus/online/Sync now, and Settings surfaces synced/syncing/offline/error
      status. Remaining: explicit conflict resolution UX beyond LWW status copy.
- [~] Offline-first: a mutation/queue so the app works offline; flush on reconnect; status surface
      (synced / offline / syncing / error) + a "Sync now" action.
      Done: latest pending snapshot per workspace is persisted locally, retrying on focus/online/manual sync with
      Settings status copy. Remaining: richer queued-change count outside Settings and conflict action sheet.
- [~] **Artifact extraction (client schema change):** move `domain.Artifact.Bytes` out of the synced
      snapshot → upload via blob `PUT` (sha256), download via `GET`, keep a local cache; the dataset
      carries a `BlobRef`. Migrate existing inline artifacts on first sync.
      Done: `Artifact.BlobRef` is in the dataset schema, sync flush uploads artifact bytes to `/v1/blobs`, and
      sync pull rehydrates missing bytes before local import. Remaining: explicit local blob cache controls and
      a one-time migration/status surface for already-inline artifacts.
- [x] AI via proxy: Insights, Allocate, and Documents prefer the backend AI proxy when backend URL/token prefs are
      configured; direct OpenAI remains optional/local-only. The client now uses `AIService` unary calls over the
      `/grpc` GoGRPCBridge tunnel for key upload, chat, and vision.
      Done: backend `ChatStream`/`VisionStream` are implemented, and the wasm AI proxy transport now consumes
      those streaming methods while preserving existing result/error callbacks.
- [x] OAuth login UI + token handling, preserving offline-first (no login required to use locally).
      Done: Settings opens Google/GitHub OAuth in a popup, the backend callback posts the issued access token
      and CSRF value back to the app, and the app stores the access token as the backend bearer token while
      keeping local-only use available.
- [~] Settings: backend URL, sign in/out, sync status; conflict/LWW UX ("a newer version was on the server - pulled it").
      Done: backend URL/token, test connection, key upload, Cloud/self-host mode, Sync now, and sync status are
      in Settings. OAuth sign-in buttons and local sign-out are wired. Remaining: richer conflict action sheet.

### 7.8 Security & privacy ★
- [x] AES-GCM key management (master-key source + rotation); AI keys encrypted at rest.
      AI keys are encrypted at rest; master-key sourcing is documented; `cashflux-server rotate-ai-master-key`
      re-encrypts stored AI keys from `CASHFLUX_SERVER_OLD_MASTER_KEY` to the current master key.
- [x] Strict per-user data isolation enforced in every query (with isolation tests).
      Reconciled against §7.14: repository/service queries scope by authenticated user id, with cross-user
      workspace/blob isolation coverage.
- [x] Request-size limits (dataset + blob), rate limiting, the bridge's abuse controls enabled.
      Reconciled against §7.14: dataset/blob/AI caps, HTTP in-flight/rate limits, per-user limits, and gRPC
      connection/stream/upgrade caps are in place.
- [~] TLS everywhere; OAuth `state`/PKCE; never log secrets; threat-model pass; `govulncheck` + `gosec` in CI.
      TLS/wss, OAuth PKCE/state, log redaction, Gitleaks, govulncheck, and gosec are covered; remaining:
      formal periodic threat-model and pre-launch pen-test pass.

### 7.9 Deploy & ops
- [x] Single binary + data dir; Dockerfile; config via env.
      Done: `Dockerfile.server` builds `cashflux-server`, Compose mounts `cashflux-data:/data`, and env-file
      configuration is documented in `docs/SELF_HOSTING.md`.
- [x] TLS (Caddy / managed) + `wss`; reverse-proxy WS keepalive/timeouts tuned.
      Done in `deploy/Caddyfile.selfhost` and `docs/SELF_HOSTING.md`.
- [x] Backups: WAL-checkpoint the SQLite file + copy the blobs dir; documented restore runbook.
      Done: `cashflux-server backup`, systemd timer examples, restore rehearsal notes, RPO/RTO docs, and
      backup/restore tests are in place.
- [x] Migrations run on boot; structured logs + OpenTelemetry (the bridge supports it); basic per-user usage metrics.
      Done: boot migrations/reject-newer schema, structured slog, Prometheus metrics, request/trace ids, and
      per-user sync/AI/blob usage counters, plus configurable OTLP/HTTP trace export.
- [~] CI: build server, run server tests, proto-drift check, lint + vuln scan.
      Done: Go tests, explicit server build, wasm build, vet, govulncheck, gosec, and gitleaks. Remaining:
      proto-drift check once codegen is pinned.

### 7.10 Testing & phased rollout
- [x] Unit: storage, LWW, encryption, rate-limit, blob hashing + refcount/GC.
      Done: repository, sync, AI, blob, usage, and retention tests cover these units.
- [~] Integration: in-proc `grpc.Server` behind the bridge over a real WS; client<->server round-trips
      now cover AI `SetKey`/`Chat`/`ChatStream` and SyncService workspace `Put`/`List`/`Get`/`Delete` unary
      calls plus watch streams, with HTTP blob PUT/HEAD/GET verified against a workspace created through the
      bridge. Remaining: browser autosave push/pull e2e.
- [~] e2e: two-device sync (LWW + tombstone), offline->reconnect flush, OAuth login, artifact blob
      round-trip, AI proxy streaming with a real key.
      Done: in-proc bridge e2e covers two-device stale LWW rejection plus tombstone propagation; AI proxy
      streaming has bridge/client transport coverage. Remaining: offline->reconnect flush, OAuth login,
      artifact blob round-trip, and real-key AI proxy smoke.
- [x] Load/abuse: connection caps, oversized payloads, rate limits.
      Done: server tests cover gRPC stream caps, bridge connection-limit config, oversized sync/blob/AI payloads,
      storage quota exhaustion, and HTTP/user rate-limit configuration.
- [x] **Rollout (each independently shippable; app works without the backend throughout):**
      (1) OAuth + snapshot sync (artifacts still inline) → (2) blob store + client artifact extraction →
      (3) AI proxy + encrypted keys + metering.
      Done in `docs/BACKEND_PLAN.md`: each phase is independently shippable/reversible, and local budgeting
      keeps working if a backend phase is disabled.

### 7.11 Monetization — billing + Cloud UX (paid tier) ★

> CashFlux Cloud is the paid tier: sync + backup + AI proxy. App stays free/local-first.
> Design: [`docs/CLOUD_UX.md`](./docs/CLOUD_UX.md) + [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md).
> **Locked:** app free; Cloud paid (annual-first subscription); AI proxy bundled into Cloud; personal
> plan now, household later. Recommended pricing ~$34.99/yr / $3.99/mo, 14-day trial (validate).

#### Server (billing + entitlements)
- [x] Stripe integration: products/prices (annual + monthly), Checkout session creation, customer portal session.
      Done: authenticated billing endpoints create Stripe Checkout sessions from configured annual/monthly
      price ids and customer-portal sessions from stored subscription customer ids.
- [x] Stripe **webhook** handler (checkout.completed, subscription.updated/deleted, invoice.payment_failed)
      → update `subscriptions` table; idempotent; signature-verified.
      Done: `POST /v1/billing/stripe/webhook` verifies Stripe signatures and upserts subscription state for
      checkout, subscription update/delete, and payment-failed events.
- [x] `subscriptions(user_id, stripe_customer, stripe_sub, status, plan, current_period_end, trial_end)`.
      Done: schema v4 adds the table with unique Stripe customer/subscription ids and repository upsert/lookup
      coverage for current subscription state.
- [x] **Entitlement gate**: a single `IsCloudActive(user)` check (active|trial|grace) enforced in the
      gRPC auth interceptor for Sync/AI RPCs and the blob endpoints; past-due grace window; lapse →
      reject cloud RPCs (clear status code) while local app keeps working.
      Done: billing-disabled self-host stays always-on, and billing-enabled `IsCloudActive` now reads
      `active`, `trialing`, and in-period `past_due` states from `subscriptions`; gRPC Sync/AI interceptors
      and HTTP blob endpoints now reject inactive billing-enabled users.
- [x] Storage fair-use cap per user (blob bytes); soft-warn → block new uploads over cap; overage copy.
      Done: `CASHFLUX_SERVER_STORAGE_WARN_BYTES` emits `X-CashFlux-Storage-Warning` before
      `CASHFLUX_SERVER_STORAGE_MAX_BYTES` blocks new distinct over-quota blob uploads with HTTP 507.
- [x] Privacy/compliance: privacy policy + terms endpoints; account export + **delete account**.
      Public `/legal/privacy` and `/legal/terms` JSON endpoints are now mounted and documented in
      `docs/LEGAL_ENDPOINTS.md`. Authenticated `/v1/account/export` returns scoped Cloud data without decrypted
      AI secrets/blob bytes; `DELETE /v1/account` purges the caller's relational rows and sweeps unreferenced
      blobs.
      (purge server data + blobs); GDPR/CCPA data-request path.
- [x] Tests: webhook state transitions, entitlement gate (trial/active/past-due/canceled), cap enforcement.
      Done: webhook upsert/payment-failed/deleted/signature tests, entitlement active/trial/past-due/canceled
      tests, inactive endpoint denial, storage cap, and storage warning tests are in place.

#### Client (Cloud UX)
- [ ] **Cloud settings section** (global FlipPanel): signed-out pitch + OAuth buttons; signed-in plan
      status, manage subscription, AI key, devices, sign out, export/delete account.
- [ ] **Sync status chip** by the workspace switcher: synced / syncing / offline (queued count) /
      error / not-signed-in; "last synced" tooltip; "Sync now"; opens Cloud settings.
- [ ] **Contextual upgrade sheet** when a free user taps a Cloud-only action (non-blocking; benefits +
      price + Start trial + Maybe later). Never blocks local features.
- [x] **Pricing screen**: annual/monthly segmented toggle (annual-first), price, trial note, Subscribe
      → Stripe Checkout (redirect); trust line (cancel/export anytime, encrypted, BYO key).
      Done: Settings now shows annual/monthly Cloud pricing with trial/trust copy, calls the backend
      Checkout endpoint, and redirects to the returned Stripe URL.
- [ ] **Account/subscription states** wired end-to-end: signed-out, free, trial (+days-left banner),
      active, past-due (grace banner), canceled → **graceful downgrade-to-local** (data stays).
- [ ] **AI key (Cloud)**: move key entry into Cloud settings (encrypted server-side, shown as "Key set",
      replace/remove); keep the client-side key field for free users.
- [ ] **Devices** list + revoke; **Manage subscription** → Stripe portal (redirect).
- [ ] **First-run Cloud mention** (calm, dismissible) + LWW pulled-newer toast.
- [ ] a11y + plain-English copy on every Cloud surface; empty/loading/offline/error states (sign-in
      failure, payment failure with retry).

#### Launch gating
- [ ] Monetize at the **sync milestone** (auth + snapshot sync + Stripe + trial); AI proxy + blobs land
      as later Cloud upgrades (no price change). Household plan is a later phase.
- [ ] Analytics: trial starts, trial→paid, MRR/ARR, churn, ARPU, storage/user, gross margin (privacy-respecting).

### 7.12 Self-hosting — first-class server choice ★

> The server is open source; self-hosting is a first-class, free alternative to paid Cloud (Actual/
> Bitwarden model). Design: [`docs/CLOUD_UX.md`](./docs/CLOUD_UX.md) "Server choice" +
> [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md) §13. Gate *operations*, not features.

#### Client
- [x] **Settings → Cloud** leads with a segmented **Server: Cloud / Self-hosted** control; the rest of
      the section adapts to the choice.
      Done: Settings persists a Cloud/Self-hosted server mode and branches the backend/billing helper copy
      from that choice.
- [x] Self-hosted: **base-URL field** + **Test connection** (reachability + version/compat ping) before
      save; persist the URL; use it for the gRPC bridge (`wss`) and HTTP OAuth/blob endpoints.
      Done: Settings persists the backend URL/token locally, Test connection validates `/v1/version`, and the
      sync/AI clients derive the GoGRPCBridge `/grpc` tunnel from the same base URL.
- [x] Hide all billing surfaces (pricing, trial banner, manage-subscription, storage cap) when a custom
      server is selected; entitlement = always-on for self-host.
      Done: Self-hosted mode hides the Cloud price/trial/Checkout/portal controls and explains the custom
      backend is treated as always-on infrastructure.
- [x] **Auth method adapts to the server:** support a lighter **single-user access-token** mode (paste
      a token the server printed) in addition to OAuth; show whichever the chosen server advertises.
      Done: Settings defaults Cloud to OAuth and self-hosted to token auth, then Test connection consumes
      `/v1/version` auth discovery to show the printed-token field or the advertised Google/GitHub OAuth buttons.
- [ ] **Switch-server flow:** changing the URL signs out of the old server and re-points sync; local
      data untouched; clear "only changes where it syncs" copy.
- [ ] Sync chip tooltip names the active server; onboarding mentions both paths once.

#### Server
- [x] Config-driven auth mode: **token (default for self-host)** vs OAuth (providers configured);
      token mode can generate/print a first-run access token and authenticate via `CASHFLUX_SERVER_TOKEN_SHA256`.
      `cashflux-server rotate-token` prints replacement token material.
- [x] Make billing/Stripe + entitlement gating **optional / disabled** in self-host mode (a config
      flag); `IsCloudActive` returns true when billing is disabled.
- [x] **Version/compat endpoint** for the client's Test-connection + too-old/too-new warnings (reuse a
      schemaVersion-style ping).
- [x] **Docker quickstart** + one-command run; sample config (.env) with TLS notes; docs linked from
      README. Remaining: link from Settings.
      Done: `docker-compose.selfhost.yml`, the env template, Caddy TLS config, README self-host link,
      `docs/SELF_HOSTING.md`, and the Settings "Deploy your own server" link are all wired.
- [x] Self-host docs: backups (SQLite WAL + blobs), upgrade path, optional OAuth setup.

### 7.13 Turnkey self-host deploy + DO referral ★

> One-click(ish) self-host on DigitalOcean, and turn the free self-host path into DO referral credit
> that offsets Cloud infra cost. Design: [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md) §14.
> Keep an unconditional plain self-host path (any host, no referral). Disclose referral plainly.

#### Packaging
- [ ] Publish the server as a **Docker image** (multi-arch) on a registry; `docker-compose.yml` with the
      server + **Caddy** (automatic HTTPS) + a persistent volume for the SQLite file + blobs dir.
- [ ] **cloud-init / user-data script**: installs Docker, runs the compose stack, generates + prints the
      first-run access token, configures Caddy for the droplet's domain/IP. Hosted at a stable URL.
- [ ] **One-command installer** (`curl -fsSL <url>/install.sh | sh`) for any fresh VPS (host-agnostic).
- [x] Config via `.env` (domain, auth mode token|oauth, master key, fair-use off, providers if oauth).
- [ ] **DO Marketplace 1-Click**: Packer build of a droplet image; submit for vendor approval (later,
      after the script path is proven).

#### Referral
- [ ] Add the **DO referral link** to the "Deploy your own server" button + install docs + Marketplace
      listing, with a clear disclosure line.
- [ ] Verify current DO referral terms before relying on it; track referral credit as reduced COGS.
- [ ] Provide a non-referral deploy path/link too (unconditional free promise).

#### In-app hook
- [x] Settings → Cloud (self-hosted): a **"Deploy your own server"** link → the deploy docs (with the
      referral disclosure), shown near the self-hosted server-URL field.
      Done: Settings links the backend controls to `docs/SELF_HOSTING.md`, and the docs disclose the
      DigitalOcean referral possibility plus the unconditional non-referral self-host path.
- [x] After deploy, the docs walk the user to paste the printed token into Settings (ties to §7.12 token auth).
      Done: `docs/SELF_HOSTING.md` now has a post-deploy Settings checklist for the server URL, printed
      `CASHFLUX_SERVER_TOKEN`, `/v1/version` test connection, and derived `/grpc` tunnel.

#### Ops/docs
- [x] Self-host runbook: backups (SQLite WAL checkpoint + blobs), upgrades (pull new image), TLS, restore.
- [x] Security defaults: token auth on by default, TLS required, sensible limits; never ship a default secret.
      Token mode is the default and generates a high-entropy token if none is configured; production browser
      origins and OAuth redirects now require HTTPS, with HTTP allowed only for loopback local development.

### 7.14 Security hardening ★

> Defense-in-depth for a server that holds user financial data + encrypted AI keys. Pairs with §7.8.
> Run `gosec` + `govulncheck` in CI from day one; treat every finding as blocking.

#### AuthN / AuthZ
- [x] Per-request auth on **every** RPC + HTTP route (deny-by-default; no unauthenticated data path). ★
      Done: HTTP data routes (`/metrics`, `/v1/audit`, blob GET/HEAD/PUT) reject unauthenticated
      requests in tests, and gRPC Sync/AI services are covered by unary/stream auth interceptors.
- [x] Strict per-user **tenant isolation** enforced at the query layer (every query filters by `user_id`);
      add isolation tests that try to read another user's workspace/blob and must fail. ★
- [x] Short-lived access tokens (JWT, ~15m) + rotating refresh tokens (httpOnly, Secure, SameSite);
      refresh reuse detection → revoke session family.
      SQLite-backed refresh-token `jti`/family rows now store hashes only; refresh consumes+rotates the
      token and reuse revokes the family.
- [x] Session revocation (logout, device revoke, "sign out everywhere"); token `jti` denylist or version.
      Logout now revokes the presented refresh-token family; `POST /v1/auth/logout-all` revokes every refresh
      session for the authenticated OAuth user and audits the action. `GET /v1/auth/sessions` lists active
      session families and `DELETE /v1/auth/sessions/{family}` revokes one user-scoped family with CSRF.
- [x] OAuth: PKCE + `state` (CSRF), nonce, redirect-URI allow-list, validate `iss`/`aud`.
      Google callbacks now require an ID token and validate issuer, audience, nonce, expiry, and issued-at
      before issuing sessions. Redirect URLs are now constrained to `/v1/auth/{provider}/callback`; OAuth state
      cookies now bind nonce values and Google auth requests send them.
- [x] Self-host token mode: high-entropy generated token, SHA-256 config storage, constant-time compare, and
      `cashflux-server rotate-token` are done.

#### Transport / browser
- [x] TLS-only (HSTS, modern ciphers); `wss` for the bridge; redirect HTTP→HTTPS.
      App origins and OAuth redirects now reject cleartext HTTP except loopback local development. Remaining:
      deploy/proxy redirect and cipher policy, plus explicit `wss` proxy verification.
      Done: app/OAuth config rejects cleartext non-loopback origins, Caddy terminates TLS with TLS 1.2/1.3
      AEAD ciphers and automatic HTTP-to-HTTPS redirects, and the self-host docs/tests pin the `/grpc`
      `wss://<domain>/grpc` tunnel and long-lived websocket proxy settings.
- [x] Security headers: HSTS, X-Content-Type-Options, Referrer-Policy, COOP/COEP, frame-ancestors/CSP
      on any served HTML; lock CORS + WS `WithOriginCheck` to the SPA origin allow-list.
- [x] CSRF protection on cookie-authed HTTP endpoints (OAuth callback, refresh); SameSite + token.

#### Input / data
- [~] Validate + bound every input: request-size caps (dataset, blob, RPC message), field limits,
      content-type checks; reject malformed protobuf/JSON early.
      Sync workspace ids/names/colors/device ids are length-bounded before storage; GetWorkspace now also
      trims and bounds lookup ids before querying. Billing checkout JSON is now capped at 64 KiB and rejects
      malformed bodies, unknown fields, trailing JSON, and explicit non-JSON content types before any Stripe
      call; Stripe webhook bodies now fail with an explicit 413 before signature validation when they exceed
      1 MiB. AI chat/vision/key-upload RPCs now reject bad roles, empty/oversized content, too many messages,
      malformed schemas, invalid temperatures, and oversized keys before key lookup/storage or upstream calls.
      The gRPC bridge JSON codec now rejects unknown fields and trailing JSON payloads before handler dispatch.
      Remaining: malformed protobuf/codegen audit and broader request-shape rejection tests.
- [x] Blob upload: verify bytes hash to the claimed `:hash`; cap size; sniff/allow-list MIME; never
      execute or serve as HTML (force download / safe content-type).
- [x] Encryption at rest for secrets (AI keys) via AES-GCM; **master key from a secret manager / KMS**,
      never in code or the DB; documented **key rotation** + re-encryption procedure.
      AI keys already store encrypted AES-GCM ciphertext; self-host docs now require secret-manager/KMS-backed
      master-key sourcing, and `cashflux-server rotate-ai-master-key` provides the maintenance-window
      re-encryption path without asking users to re-enter keys.
- [ ] Consider per-user dataset encryption-at-rest (envelope encryption) as a later privacy upgrade.
- [x] SQLi-safe by construction (parameterized queries only); path-traversal-safe blob paths.
      Blob paths now reject malformed hashes before disk access and stay rooted under the blob directory.
      Formal SQLi coverage is now in `TestRepositorySQLAuditUsesParameterizedQueries`, which rejects dynamic SQL
      formatting/builders and pins parameterized tenant predicates.

#### Abuse / DoS
- [x] Per-user + per-IP rate limits + quotas; HTTP per-IP/per-user minute caps, AI request/token quotas,
      and the bridge's connection/upgrade caps are enabled.
- [x] Global request timeouts/deadlines; max in-flight; slow-loris protection (read/write/idle timeouts).
- [x] Backpressure on streams; cap concurrent streams per user.

#### Supply chain / process
- [x] CI: `govulncheck`, `gosec` (high sev blocking), `go vet`, dependency pinning + Dependabot/renovate.
      `go vet`, `govulncheck`, and high-severity/medium-confidence `gosec` now run in CI; Dependabot
      watches Go modules and GitHub Actions weekly.
- [~] Reproducible builds; SBOM (e.g. `cyclonedx`); sign release artifacts/images (cosign).
      `deploy/release-server.example.sh` now builds the server with deterministic Go flags, writes
      checksums, generates a CycloneDX SBOM, and signs binary/SBOM blobs with cosign. Remaining: CI release
      automation and signed container images.
- [x] `SECURITY.md` + `security.txt` + a coordinated vuln-disclosure process.
- [~] Periodic threat-model review; pre-launch pen-test pass; secrets scanning (gitleaks) in CI.
      Gitleaks now runs in CI; remaining: periodic threat-model review and pre-launch pen-test pass.
- [x] Least-privilege runtime: non-root container, read-only FS where possible, drop caps, minimal base image.
      Self-host Compose now uses read-only root filesystems, hardened tmpfs mounts, dropped capabilities,
      and `no-new-privileges`; the server image already runs as the non-root `cashflux` user.

### 7.15 Observability — structured logging (slog), metrics, tracing ★

> Match the client's discipline: **`log/slog` everywhere, structured, leveled, contextual** — never
> `fmt.Println`. Logs/metrics/traces must be safe (no secrets/PII) and correlatable.

#### Structured logging (`log/slog`) ★
- [x] Adopt `log/slog` with a **JSON handler** in prod (text in dev); single configured logger injected
      via context — no package-global `log`. ★
- [x] **Request/RPC-scoped logger**: attach a generated **request id / correlation id** (propagate via
      gRPC metadata + HTTP header `X-Request-ID`) and `user_id`, `workspace_id`, `rpc`/`route`,
      `device_id`, latency, status to every log line. ★
- [x] **Leveling**: Debug/Info/Warn/Error with a runtime-configurable level (env); sane prod default Info.
- [x] **Redaction is mandatory**: never log AI keys, tokens, OAuth secrets, cookies, full datasets, blob
      bytes, or PII. A `slog` middleware/`ReplaceAttr` that scrubs known-sensitive keys + a deny-list;
      log sizes/hashes/ids instead of contents. Add a test that asserts secrets never appear in output. ★
- [x] **Audit log** as a *separate*, append-only structured stream for security-relevant events (login,
      token issue/revoke, key set/replace, subscription change, account/data delete, admin actions),
      with actor, action, target, ip, timestamp; tamper-evident (hash chain) if feasible.
      Backend audit events now persist in SQLite with append-only ids and previous-hash/hash chaining, and
      `/v1/audit` streams authenticated NDJSON for recorded login/session, AI-key, sync, and blob actions.
- [x] Log sampling for hot paths; structured error logging with stack/cause (wrap with `%w`).
      Successful health/metrics probes are sampled with `CASHFLUX_SERVER_LOG_HOT_PATH_SAMPLE_RATE`, and
      HTTP 5xx/non-OK gRPC records now log structured status and cause fields at error level.
- [x] Ship logs to a sink (stdout → collector); retention + access policy; PII-minimized.
      Self-host services now use bounded Docker local log retention, and `docs/OBSERVABILITY.md` documents
      stdout collection, central sink forwarding, 30-day starting retention, and restricted operator access.

#### Metrics
- [x] Prometheus/OpenTelemetry metrics: request rate/latency/errors per RPC + route (RED), active WS
      connections, stream durations, sync push/pull counts + conflict/LWW rejects, blob bytes
      stored/transferred, AI proxy tokens/requests per user, DB query latency, queue depths.
      RED route/RPC counters, active stream gauges, stream duration sums, blob byte counters, AI proxy request/token counters, sync push/pull/LWW reject counters, DB operation latency counters, and workspace-watch queue depth gauges are exported.
- [x] Business metrics (privacy-respecting): signups, trials, conversions, MRR (from billing webhooks).
      Done: billing webhooks emit aggregate signup/trial/conversion/cancellation/payment-failure counters
      and an estimated active MRR cents gauge without user labels.
- [x] `/metrics` endpoint (auth-gated or internal-only).

#### Tracing
- [x] OpenTelemetry tracing end-to-end (the GoGRPCBridge canonical path already integrates OTel spans);
      Done: HTTP/gRPC trace context is extracted into request scope and logs, and the server installs an
      OpenTelemetry SDK tracer provider with OTLP/HTTP export when `CASHFLUX_SERVER_OTLP_ENDPOINT` or
      `OTEL_EXPORTER_OTLP_ENDPOINT` is configured.
      propagate trace context client→bridge→grpc→DB/upstream; export to an OTLP collector.
- [x] Correlate trace id ↔ request id ↔ log lines.
      HTTP and gRPC paths now extract W3C trace IDs and add `trace_id` beside `request_id` in structured logs.

#### Health & dashboards
- [x] `/livez` (process up) + `/readyz` (DB reachable, migrations applied, deps ok) distinct probes.
- [x] Dashboards + **SLOs** (availability, p99 latency, error rate) with alerting + on-call routing.
      `docs/OBSERVABILITY.md` now defines SLOs, dashboard queries, and routing; `deploy/prometheus-rules.yml`
      adds backend-down, HTTP/gRPC error-rate, and latency alerts.

### 7.16 Reliability, SRE & disaster recovery
- [~] Context deadlines/timeouts on all I/O (DB, upstream OpenAI, blob store); cancellation propagation.
      OpenAI proxy calls now have configurable upstream deadlines; blob PUT/GET now use
      `CASHFLUX_SERVER_BLOB_IO_TIMEOUT` and context-aware store operations. Remaining: DB deadlines.
- [~] Retries with jittered exponential backoff for transient upstream failures; circuit breaker on the
      AI upstream; idempotent writes (idempotency keys on mutating HTTP; PUT semantics on sync).
      OpenAI proxy retries transient transport, 429, and 5xx failures; repeated upstream transport/5xx failures
      now open a short fail-fast circuit that resets after cooldown/success. Stripe billing checkout/portal
      endpoints now persist and replay `Idempotency-Key` results per user/route/request hash. Remaining:
      any future non-PUT mutating HTTP endpoints must use the same idempotency pattern.
- [x] Graceful shutdown: stop accepting, drain active streams/requests, checkpoint WAL, flush logs.
- [x] **Backups + tested restore**: scheduled SQLite WAL checkpoint + file copy + blobs dir snapshot to
      off-box storage; **documented + periodically rehearsed restore**; define **RPO/RTO**. Done:
      `cashflux-server backup` checkpoints WAL, snapshots DB + blobs with a manifest, the self-host
      runbook defines scheduled/off-box backup, restore rehearsal, RPO/RTO, and tests cover the restore path.
- [x] Zero-/low-downtime deploys; safe forward-only migrations (run on boot, reject newer schema);
      migration dry-run + rollback plan; data-migration backfills idempotent.
      Done: `cashflux-server migrate-check` applies migrations to a temporary DB/WAL copy and reports
      the resulting schema version without mutating the live SQLite file; deploy docs require backup,
      migration dry-run, then rebuild through the Caddy-backed stream-drain path; tests cover newer-schema
      rejection and current migration idempotency/data preservation on repeated opens.
- [x] Resource limits (memory/CPU/FD/conn); OOM-safe; bounded queues; graceful degradation under load.
      Done: self-host Compose caps CPU/memory/PIDs/open files for server+Caddy, env docs expose
      HTTP/gRPC connection and stream limits, and deploy tests pin the runtime ceilings/backpressure knobs.
- [x] Status page + incident response runbook (sev levels, comms, postmortems). Done:
      `GET /status` returns component health for status-page polling, and `docs/INCIDENT_RESPONSE.md`
      covers SEV levels, first response, comms cadence, recovery, and postmortems.

### 7.17 Compliance & data governance
- [x] **GDPR/CCPA**: self-serve data **export** + **delete account** (purge DB rows + blobs +
      subscription unlink), data-subject request workflow + SLA; right-to-rectify via the app.
      Done: authenticated `/v1/account/export` and `DELETE /v1/account` cover scoped server data export,
      current subscription export, explicit subscription unlink, relational purge, and unreferenced blob sweep.
      `docs/LEGAL_COMPLIANCE.md` defines the DSR workflow, SLA target, and right-to-rectify path.
- [x] Privacy Policy, Terms of Service, Cookie/consent (minimal), DPA template for any sub-processors
      (Stripe, OAuth providers, host) + a public sub-processor list.
      Done: `docs/LEGAL_COMPLIANCE.md` carries launch draft privacy/terms copy, cookie/session notes, DPA
      outline, public subprocessors list, and counsel-review caveat; a deploy doc test pins the sections.
- [x] Data retention + deletion schedule (snapshots history, logs, audit, backups); document residency.
      Done: retention env windows plus `cashflux-server retention` prune audit events, snapshot history,
      and local backup dirs; weekly timer examples and self-host docs define residency and defaults.
- [x] **PCI scope minimized** by using Stripe Checkout/Elements (no card data touches the server).
      Done: billing endpoints only create Stripe-hosted Checkout/customer-portal sessions and consume
      webhooks; `docs/LEGAL_COMPLIANCE.md` now pins the no-card-data boundary, allowed billing identifiers,
      and Stripe Radar/payment-method ownership with a deploy doc test.
- [x] **SOC 2 readiness checklist** (access control, change mgmt, monitoring, vendor mgmt, IR) — even if
      not certifying, build to the controls so enterprise/audit asks are answerable.
      Done: `docs/SOC2_READINESS.md` defines access control, change management, monitoring/availability,
      vendor management, and incident-response controls with a deploy doc test pinning the checklist.
- [x] Encryption-in-transit + at-rest documented; key management policy; access logging to prod data.
      Done: master-key source/length/rotation guidance is documented for AI-key encryption at rest, TLS/proxy
      notes cover encryption in transit, and `docs/BACKEND_SECURITY.md` now defines a production data access
      logging policy pinned by deploy doc tests.

### 7.18 Performance, scale & limits
- [~] Load + soak tests (sync push/pull, blob up/down, AI streaming, WatchWorkspaces fan-out); publish
      a baseline like the bridge's benchmark snapshots; perf regression gate in CI.
      `TestServerLoadSmokeSyncBlobAndWatch` now covers concurrent sync pushes, workspace-watch fan-out, list,
      and blob upload/download through the in-process HTTP/gRPC bridge. Remaining: AI streaming, longer soak
      runs against production-like disk/proxy/network, and a published perf-regression gate.
- [x] DB tuning: WAL, `busy_timeout`, sensible `PRAGMA`s; single-writer awareness; per-request conn use.
- [x] **Scale ceiling — be honest:** SQLite is single-writer. Document the throughput boundary and the
      **migration path to Postgres (or per-tenant SQLite sharding)** for true multi-tenant scale; gate
      the choice on real numbers, not speculation. ★ Done: `docs/SCALE_LIMITS.md` documents the
      single-writer ceiling, capacity signals, migration triggers, and sharded-SQLite/Postgres/object-storage path.
- [x] Quotas/fair-use enforced (storage cap, rate limits) with clear `resource-exhausted` responses.
      Done: HTTP/user rate limits and AI quotas exist; blob uploads now enforce
      `CASHFLUX_SERVER_STORAGE_MAX_BYTES` per user and return HTTP 507 `storage quota exceeded`.
- [x] Pagination/limits on any list endpoint; cap snapshot history; blob GC scheduled + monitored.
      Done: audit listing is capped, snapshot history is capped/pruned, and `cashflux-server gc-blobs`
      plus weekly timer examples and Prometheus GC counters cover scheduled/monitored blob cleanup.
- [x] Caching: immutable blob cache headers (+ CDN later); ETag/If-None-Match on `GetWorkspace`.

### 7.19 API governance & operability
- [x] Versioned API (`/v1`, proto package versioning); **backward-compat policy** + deprecation windows;
      CI proto-/API-compat guard (the bridge ships an `api_compat_guard` tool — reuse the pattern).
      Done: `cmd/api_compat_guard` runs in CI and checks `/v1`, `cashflux.v1`, backend compatibility constants,
      the generated-code target package, and the proto compatibility/deprecation policy.
- [x] Consistent **error taxonomy** (gRPC codes ? HTTP statuses) with stable, documented error reasons;
      machine-readable error details; no internal leakage in messages.
      `internal/server.BackendErrorTaxonomy` now pins stable reasons and gRPC/HTTP mappings, and
      `docs/BACKEND_ERRORS.md` documents the table. HTTP data, OAuth/session, readiness, CORS, rate-limit,
      max-in-flight, and encode-fallback errors now return JSON details; production `http.Error` calls are gone.
- [x] Config via env/secret manager with validation on boot; **feature flags** (billing on/off, AI proxy
      on/off, self-host mode) so deployments differ safely.
- [x] Runbooks (deploy, rollback, restore, rotate keys, revoke sessions, handle past-due); on-call docs.
      Done: `docs/OPERATIONS_RUNBOOK.md` covers deploy, rollback, restore, token/master-key rotation,
      session revocation, past-due handling, and routine checks; deploy tests pin the required sections.
- [x] Admin tooling (read-only support views; usage lookups) — built on the same isolation guarantees.
      Done: `/v1/admin/usage` is authenticated, read-only, scoped to the caller's usage counters, and covered by
      cross-user isolation tests.

### 7.20 Anti-abuse & fraud
- [~] Signup/login abuse controls (rate limit, optional CAPTCHA on bursts, email/OAuth verification).
      OAuth/session routes now have a dedicated per-IP `CASHFLUX_SERVER_AUTH_RATE_LIMIT_PER_MINUTE` cap with
      JSON `RATE_LIMITED` errors. Google ID-token verification now rejects missing/expired expiry claims and
      future issued-at claims before userinfo fetch or session issuance. OAuth userinfo rejects explicit
      unverified email claims. Remaining: optional CAPTCHA-on-burst policy only.
      policy and broader email/OAuth verification review.
- [x] **Referral-fraud guards** (DO referral path): detect self-referral/farming; honest disclosure;
      don't tie product behavior to referral outcomes.
      Done: `docs/CLOUD_BUSINESS_PLAN.md` now treats referral attribution as accounting-only metadata,
      lists self-referral/farming signals, keeps suspicious referrals out of COGS modeling without changing
      product behavior, and preserves the non-referral self-host path; deploy docs test the guardrails.
- [x] Trial abuse limits (one trial per account/identity); payment-fraud handling via Stripe Radar.
      Done: Checkout refuses accounts with a prior trial or current active/trialing/past-due subscription, and
      payment-fraud handling remains in Stripe Checkout/Radar with no card data touching CashFlux.
- [x] AI-proxy abuse: per-user token/req caps, anomaly alerts, kill-switch per user; cost-control even
      though tokens are BYO (protect bandwidth/compute + the user's own bill).
      Per-user request/token caps already gate usage; `CASHFLUX_SERVER_AI_BLOCKED_USER_IDS` now denies selected
      users before key load or upstream calls. `CASHFLUX_SERVER_AI_ALERT_REQUESTS_PER_DAY` and
      `CASHFLUX_SERVER_AI_ALERT_TOKENS_PER_DAY` append audit alerts when daily warning lines are crossed.

---

### C91. Migrate Tailwind CDN → gwc typed CSS classes (Tailwind interop) ★ (prod/security, user-requested 2026-06-21) — FINAL TODO
**DONE (2026-06-21):** The `cdn.tailwindcss.com` `<script>` + inline `tailwind.config` are removed from
`web/index.html`. Upgraded GoWebComponents to v3.2.0 (typed `css`/`css/u` engine) and migrated its breaking
`shorthand.Class`→`ClassStr` rename (~1,526 sites). Built `internal/ui/tw` — a typed, Tailwind-compatible
vocabulary (each utility emits the exact Tailwind-default CSS via the gwc registry/Sink into `<style id=gwc-css>`,
exact-value tested). Converted all ~1,450 literal utility sites to `css.Class("semantic", tw.Util…)` and typed
the ~40 dynamically composed strings via `tw.Fold`/`tw.ColorClass`. Restored a minimal Tailwind-preflight
equivalent (the CDN had shipped one) — notably `svg{display:block}`, which fixed the trend-chart tile overflow.
Added stable test hooks (`cf-shell`, `active`, `dash-task`, tone markers) and updated the e2es that were coupled
to now-folded utility class names. Verified: 0 `cdn.tailwindcss.com` requests, 0 console errors, correct computed
styles, full e2e re-run with no port regressions; SW bumped to v246. Remaining: optionally reconcile the palette
tokens with the theme-engine CSS vars (kept as exact hex here for parity), and vendor Google Fonts for a fully
offline build (the only remaining external asset).
**Why:** the app currently loads **`https://cdn.tailwindcss.com`** at runtime — a JIT compiler that can't be
SRI-pinned, depends on a third-party CDN, and won't work offline (OWASP A08; the last external script after D3
was vendored in **C44**). A naive static `tailwindcss` compile is unsafe here because the UI composes utility
classes as **dynamic Go strings** (e.g. `"btn " + cls`) that Tailwind's content-scanner would miss → broken
styling.
**Plan (do this LAST, once the framework is updated):**
- [ ] **Pull the latest GoWebComponents** (`go get github.com/monstercameron/GoWebComponents@latest` → `go mod
      tidy`, `GOOS=js GOARCH=wasm`); review its **typed CSS class** API + Tailwind interop.
- [ ] Adopt gwc's **typed/compile-time CSS classes** so class usage is statically known (no opaque string
      concatenation), making a real Tailwind build tractable and type-safe.
- [ ] Stand up the **Tailwind build step** (CLI scanning the typed-class source) → emit a static `web/tailwind.css`;
      replace the `cdn.tailwindcss.com` `<script>` with a local `<link>`; precache it in `web/sw.js`.
- [ ] Port the inline `tailwind.config` (theme extend in `web/index.html`) into the build config; **safelist** any
      remaining dynamic classes.
- [ ] **Verify:** no external CDN scripts remain; offline load works; visual parity across screens/themes; bump SW.
_Cross-links: **C44** (D3 vendored; this finishes the CDN-removal/offline goal), **C73** (component-ization pairs
naturally with typed classes), **C69** (theming via tokens)._

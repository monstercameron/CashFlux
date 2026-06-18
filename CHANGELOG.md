# Changelog

All notable changes to CashFlux are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Policy: **one feature per commit**,
and every commit updates this file under `Unreleased`.

## [Unreleased]

### Added
- **Budgeting method: Simple or Zero-based (D6):** Settings now has a budgeting-method selector. Under
  **Zero-based**, the Budgets screen shows how much of the month's income is still unassigned —
  "$X left to assign", "Every dollar is assigned", or "Over-assigned by $X". The choice is household
  config and persists. Backed by `budgeting.Methodology`/`ToAssign` (pure, table-tested). Verified live.
- **Reorder the sidebar by dragging (B8):** drag a primary nav item onto another to reorder the menu;
  the order persists across reloads. New screens append and hidden ones are skipped automatically.
  (Clicking a nav item still navigates as before.) Backed by a new pure `navorder` package with table
  tests; verified live (dragging Accounts to the top reorders and persists).
- **Empty dashboard tiles now offer an "Add" button (C23):** an empty Accounts / Goals / Budgets / To-do
  widget shows an in-context "Add a …" button that jumps to the relevant screen, so you can create data
  from the dashboard. The Budgets tile only offers it when there are genuinely no budgets (not when the
  at-risk filter is simply empty).
- **Opt-in "Remember my key on this device" (C27):** Settings → AI now has a toggle (off by default) to
  keep your OpenAI key across reloads. When off, the key stays session-only (the dataset autosave always
  redacts it); when on, the key is saved to its own localStorage entry and restored on boot, so AI stays
  on after a refresh. A plain-English note explains it's stored unencrypted in this browser. Verified live
  (toggling on persists the key, off clears it). Closes the AI-key-lost-on-reload rough edge.
- **Your data now survives a page reload (local persistence):** previously every reload reset the app to
  the sample dataset (data was in an in-memory store with only manual Export/Import). The dataset is now
  autosaved to localStorage — snapshotted on a short ticker (catching every change) and on page-hide,
  writing only when it changes — and loaded on boot (falling back to the sample on first run). The OpenAI
  key is **redacted** before saving, so the secret stays session-only; a save that exceeds the storage
  quota is caught rather than crashing. Verified live: a redacted dataset (no `openAiKey`) is written
  within a few seconds and the app boots with its data.
- **"+ Add" is now a multi-entity add menu (C23):** instead of jumping straight to a transaction form,
  the top-bar "+ Add" opens a small menu — New transaction (the inline quick-add panel) · New account ·
  New budget · New goal · Scan a document — routing to the right place so data entry isn't trapped on
  each entity's own screen. Verified live (menu opens with 5 items, "New transaction" opens the quick-add
  panel, the menu closes on select). SW cache bumped (v10 → v11).
- **Auto-layout engine for the dashboard (C24, model):** a pure `dashlayout.Arrange(items, mode)` that
  reorders tiles by a chosen `Mode` — **Custom** (your manual order), **Auto: default** (the canonical
  built-in order), or **Auto: importance** (sort by a per-tile importance, ties broken by the default
  order) — and the existing `Pack` then derives positions. Auto-layout only reorders; tile sizes stay
  user-set. Tile gained an `Importance` field (additive; older saved layouts keep working). Table-tested
  (order determinism, stability, no-overlap-after-pack, no input mutation).
- **Dashboard layout-mode selector (C24):** the dashboard header now has a Custom / Auto: default /
  Auto: importance selector; the render path applies `Arrange` before `Pack`, the choice persists across
  reloads, and a manual drag bakes the current arrangement and switches back to Custom.
- **Per-tile importance ranking (C24):** in Auto-importance mode every tile's gear opens a settings panel
  with an Importance control (Highest/High/Normal/Low); ranking a tile reorders the dashboard (sizes
  stay as you set them). Because importance is a universal setting, a tile's gear panel is never empty —
  so the gear can appear on every tile in importance mode without reintroducing C21's empty panel. End-
  to-end verified live: ranking the bottom freshness tile "Highest" moved it from grid-row 8 to row 2,
  and the choice persisted. This completes the C24 auto-layout feature.

### Changed
- **Text/display size now scales to 200% for accessibility (C26):** the display-scale control (Settings →
  Appearance, relabelled "Text & display size") now goes up to 200% (was 130%), meeting WCAG 2.1 SC 1.4.4
  "Resize text." This works now because the C10/C19 responsive fixes make the app *reflow* at high zoom
  instead of overflowing — verified live: at 200% on a 1280px window the page reflows with no horizontal
  scroll. It composes with the density setting (an independent zoom multiplier on top of the base tokens).
- **Tighter default density (C25):** the out-of-the-box UI felt too heavy for a dense finance app. The
  base font is now 14.5px (from 16) with line-height 1.45, and the shared control/widget tokens are
  trimmed — `.field` ~34px (was ~40) with 6px corners, `.btn` padding reduced, `.wbody` padding tightened.
  The Fraunces display figures keep their sizes, so the data accents stay prominent; the Compact toggle
  and Display scale remain as further levers. Nothing drops below the 24px touch-target minimum. Verified
  live (no text clipping; KPI figures still fit). SW cache bumped (v11 → v12).
- **Negative money now reads the same on every screen (C2):** all figure displays use one accounting
  formatter, so negatives show in parentheses (`($60.20)`) with thousands grouping everywhere — the
  Transactions list, Accounts, Budgets, Goals, Allocate, Planning, etc. now match the Dashboard instead
  of mixing in a minus sign (`-$60.20`). The two display formatters (`fmtMoney`/`fmtAccounting`) were
  collapsed into one. Editable inputs are unaffected (they format with a plain minus and never parse a
  parenthesized value). Verified live in a headless browser (Dashboard figures unchanged: `$20,749.25`,
  `($1,500.00)`).
- **Dashboard tiles now reflow instead of overlapping (C14/B2):** the bento is now an ordered sequence
  packed into the grid, so dragging a tile reorders it and the others flow to fill the gap, and resizing
  reflows around the new size — fixing the old behavior where a widened tile overlapped its neighbor and
  the resize handle then "stopped working." Resize handles cycle the span (tooltips now say so). The
  default arrangement is unchanged (verified pixel-for-pixel in a headless browser).

### Fixed
- **Budgets has a single period control now (C7):** the Budgets card had its own `‹ January 2006 ›`
  month stepper competing with the global top-bar resolution control (and in a different format). The
  in-card stepper is removed; the screen now follows the shared top-bar period, so there's one control
  and one format.
- **Receipt import matches near-miss category names (C27):** the vision model often returns a near-name
  ("Food & Drink") for a household category ("Food"), which previously imported uncategorized. Imports now
  fall back to a substring match (either direction, min 3 chars, deterministic order) before the
  auto-rules, so close category names land in the right category.
- **"Save as task" gives the to-do a sensible title (C27):** saving an AI insight used the entire first
  sentence of the answer as the task title (long, truncated). The title is now the question you asked
  (or a short "Money insight" label for "Explain my month"), with the full answer kept in the notes.
- **Document-review amounts use accounting style (C27):** the AI receipt-import review rows showed raw
  "−4.50" values; they now format through the same accounting formatter as the rest of the app
  (parentheses for negatives, grouped, in the chosen account's currency), with a raw fallback while a
  value is still being edited.
- **CSV import accepts its own documented format (C27):** pasting the on-screen example
  `date,payee,amount,account` failed demanding an undocumented `currency` column (and leaked a raw
  `store:` error). Currency is now optional — it defaults to your base currency — and the account /
  category / member columns accept either an ID or a **name** (resolved case-insensitively to the right
  entity). The friendly `account`/`category`/`member` headers work alongside the export's `*_id` headers
  (the explicit ID wins). The import error no longer shows the internal `store:` prefix. Covered by new
  table tests.
- **List-row action buttons wrap instead of overlapping at narrow widths (C19):** on a phone/tablet the
  transaction row's buttons (Mark cleared / Edit / Duplicate / ✕) overlapped the description and date.
  Rows now wrap below 1024px so the actions flow under the text. Shared by every list screen; a no-op
  when the row still fits. Verified the wrap mechanism in a headless browser. SW cache (v9 → v10).
- **Dashboard KPI figures no longer clip on tablets (C19):** between the phone breakpoint and the
  desktop the 4-column bento squeezed tiles to ~150px and figures like "$20,749.25" clipped. A tablet
  layout (768–1024px) now flows the tiles into two columns (the header stays full-width), so figures fit.
  Verified live at 900px (no clipped figures, KPI tiles ~315px, no horizontal scroll). SW cache (v8 → v9).
- **The collapsed/expanded sidebar state now survives reloads (C20):** collapsing the rail was a
  transient choice lost on refresh. It's now persisted to localStorage (like the other UI prefs) and
  restored on load. Combined with C15 (collapse keeps the nav icons), collapsing the sidebar is now
  usable rather than reading as "the panel disappeared." Verified live: toggling writes the stored flag
  and the rail goes 58px↔240px. (An on-panel collapse chevron is still a separate UX item.)
- **The widget gear now appears only where there's something to configure (C21):** the four KPI tiles
  and the cashflow/bills/freshness tiles have no settings, but their gear still opened the empty "This
  widget doesn't have any settings yet" panel — reading as broken. The gear now renders only on tiles
  with a settings schema (or an explicit action); the rest get an inert, equal-width slot so the header
  stays balanced. The gear also brightens on tile hover/focus so per-tile settings are more discoverable.
  Verified live: 8 configurable tiles show a real gear, 8 non-configurable tiles don't. SW cache (v7 → v8).
- **Top-bar controls are reachable on tablets and phones (C19):** below 1024px the time-resolution
  control + "+ Add" ran off the right edge with no wrap, so some were unreachable and the breadcrumb was
  clipped to "D". The bar now grows to two rows — breadcrumb on top, the controls wrapping onto a
  full-width row below. Verified live at 768px (bar ~175px, breadcrumb readable, nothing past the
  viewport) and 390px (all controls reachable, no horizontal scroll). SW cache bumped (v6 → v7).
  (Transaction-row action-button wrapping and KPI figure clipping at squeezed widths remain open under
  C19.)
- **Inline-edit now lays out like the Add form on every screen (C18):** editing a **Transactions** or
  **Accounts** row stacked its fields vertically in a narrow left column (tall, with empty space to the
  right), while **Budgets** edited horizontally. The edit form (already a `form-grid`) was wrapped in the
  flex `.row`, which shrink-wrapped it to a single 150px column. It now uses a full-width `.row-edit`
  block, so the grid expands to multiple columns and editing matches adding. Verified: the grid yields
  3 columns at 600px in `.row-edit` vs 1 in the old `.row`. SW cache bumped (v5 → v6).
- **Collapsing the sidebar no longer hides all navigation (C15):** the collapsed rail showed only the
  brand mark and the active highlight — no nav icons — so you couldn't navigate while collapsed. The CSS
  rule that hides the "TOOLS"/"SYSTEM" section labels (`nav > div`) also matched every nav item, because
  the framework wraps each item in a `<div>`. The section labels now carry a `rail-section` class and the
  rule targets only those, so the icon buttons stay visible (and B5's hover-flyout label works). The same
  fix covers the <768px mobile rail, which had the identical bug. Verified live (collapsed rail shows all
  14 icons; both section headers hidden). SW cache bumped (v4 → v5).
- **Period totals no longer silently drop first-of-period transactions (C1):** the Dashboard Income KPI
  read `$0.00` for a month that clearly held a $4,200 salary dated the 1st. Period windows were built at
  the machine's *local* midnight while transaction dates are stored at UTC midnight, so on any machine in
  a timezone behind UTC the month-start landed *after* a `00:00Z` first-of-month transaction and excluded
  it. Period boundaries are now UTC-midnight calendar dates throughout (`dateutil`, `period`), matching
  the UTC-dated transactions. Added a table test that a `00:00Z` first-of-month transaction is counted
  regardless of the machine timezone. Income KPI now shows `$4,200.00` (verified live).
- The **net-worth trend chart** Y-axis is now readable and correct (C16): it plotted raw minor units
  (cents), so the axis showed clipped, non-monotonic labels like "000,000 / 500,000". The chart now
  plots major units (dollars) and formats ticks as compact currency — `$0 / $5k / $10k / $15k / $20k`
  (verified live in a headless browser). The D3 shim now honors the per-axis `format` hint
  (`chartspec.Axis.Format`). Service-worker cache bumped (v3 → v4) so returning users get the new shim.
- The **quick-add** transaction panel no longer floats in a tall, mostly-empty card: the panel height
  is now sized to its compact form (420px instead of the default 470px) with the body still scrolling
  if it ever overflows. Verified live in a headless browser (panel opens at 420px on "+ Add"). (C13)
- The **Accounts** add/edit form's asset inputs no longer clip their labels ("Expected returr",
  "Liquidity 0–10…"): the placeholders are now short ("Return %", "Liquidity", "Stability") with the
  full label + range on hover (`title`). (C9)
- **Mobile/responsive layout (C10):** below 768px the app no longer scrolls horizontally with the
  content pushed off-screen — the sidebar collapses to an icon rail, the main area takes the full
  width, and the dashboard bento stacks into a single column. Verified in a headless browser at 390px
  (no horizontal overflow). Desktop is unchanged.
- The **Insights** screen is no longer near-empty without an OpenAI key: the "Ask about your money" box
  now always shows (a disabled preview + a hint to add a key when none is set), advertising the feature
  — the offline Spending-highlights card already displayed. (C9)
- The last row of the **settings panel** (e.g. "Display scale") is no longer clipped against the
  sticky footer — the scrollable body now has extra bottom padding so it clears the fold. (C12)
- The rail's **household card** summary no longer repeats "Settings" (the gear icon and tooltip already
  convey it) — it reads "N members · USD base". (The earlier "GWC avatar overlap" symptom was from the
  old mockup and is gone in the current flex layout.) (C3)
- Money amounts everywhere now show **thousands grouping** (e.g. `$20,749.25` instead of `$20749.25`)
  — Accounts, Budgets, Goals, Allocate, etc. that used the ungrouped `fmtMoney` are fixed in one place. (C2)
- The top-bar **time-resolution control** (Week/Month/Quarter + period stepper) now appears only on
  period-aware screens (Dashboard, Transactions, Budgets, Planning, Insights) — it's hidden on Members,
  Categories, Rules, Customize, Allocate, Documents, To-do, and Goals where a period does nothing. (C4)
- **Categories** can now have a **color** and show it: a color swatch appears on each category row, and
  the Add/Edit category forms have a color picker (the `Color` field existed in the model but was never
  surfaced). (C9)
- The **member color picker** (Add/Edit member) now renders as a proper clickable color swatch with a
  label instead of a thin bare line (it was a native color input squeezed into a text-field style). (C8)
- The dashboard no longer shows two tiles both titled **"Net worth"**: the trend chart tile is now
  titled **"Net worth trend"**, distinct from the net-worth KPI. (C5)
- The Allocate screen no longer lists **zero-score candidates** (accounts with no expected-return/
  stability/liquidity set, which rendered as "0% · returns 0 · stability 0 …" noise); when that hides
  everything, it nudges you to set those account attributes instead. (C6)
- A widget whose gear opens a settings panel with **no settings** now shows a single **Close** button
  instead of a Cancel/Save pair that implied there was something to commit. (C11)
- Budget rows no longer show a redundant **"Food · Food"** when a budget is named after its category —
  they show one label; an unnamed budget shows just its category (no leading "· "). (C7)

### Changed
- The dashboard's **net-worth trend** widget now renders through the new D3 `ui.Chart` (with axes)
  instead of the pure-SVG sparkline — the proof of the D3 pipeline. (Other charts still use the
  pure-SVG renderer; this one needs an in-browser check.)

### Added
- Charting (D3): a Go `ui.Chart` component now drives the D3 shim — it owns a managed container and an
  effect (keyed on the serialized spec) that hands the element and a `chartspec.Spec` to D3, redrawing
  on data change and clearing on unmount. Ready for widgets to adopt. (Render needs an in-browser check.)
- Charting groundwork (D3): a pinned D3 v7 and a theme-aware `cashfluxRenderChart` shim that draws a
  `chartspec.Spec` (line/area/bar/donut, with axes) are now loaded and service-worker cached for
  offline use; the `chartspec` types are JSON-tagged for the wire format. The Go `ui.Chart` component
  that drives this lands next. (The D3 rendering itself needs an in-browser check.)
- Accessibility: required fields are now marked `aria-required` across the add forms — accounts,
  categories, budgets, goals, members, rules, to-do, transactions, plus quick-add and plans — so
  screen readers announce which fields must be filled.
- Accessibility: inline **form validation errors** are now `role="alert"`, so screen readers announce
  them the moment they appear (e.g. "Enter a positive target amount") instead of leaving the failure
  silent.
- Each saved plan now shows a small **projected-balance sparkline** of its trajectory over the horizon
  (green if it ends up, red if down) next to its end figure — so you can see the shape, not just the
  number.
- Settings → Appearance now shows the **contrast ratio of the chosen accent** against the theme
  surface, with a warning when it's low (e.g. the default green on the light theme) — so you can pick a
  more legible accent. Powered by the new `contrast` package.
- Plans can now include a **one-time amount** (a bonus or big expense) in a chosen month, alongside the
  steady monthly change — so "what if I also get a $2,000 bonus in month 6" shows up in the projection.
- Accessibility: the **toggle switches** and **accent color swatches** are now fully keyboard-operable —
  they're focusable (Tab) and respond to Space/Enter, with a visible focus ring — so settings and the
  accent picker no longer require a mouse. (Previously they were mouse-only `<div>`s.)
- New pure `contrast` package: WCAG relative-luminance and contrast-ratio math with AA/AAA pass
  predicates — the foundation for checking that colors (especially a user's chosen accent) are legible
  against their background. Fully table-tested (black/white = 21:1, symmetry, known boundary pairs).
- Accessibility: the flip-panel dialogs (quick-add, settings) now **trap keyboard focus** — focus moves
  into the dialog on open, Tab/Shift+Tab cycle within it instead of escaping to the page behind, and
  focus returns to whatever you opened it from when it closes. Completes the modal dialog semantics.
- Accessibility: small icon-only buttons (delete ✕, toast dismiss, dialog close, the time-resolution
  arrows) now have a **minimum 24×24px hit area** with the glyph centered (WCAG 2.5.8) — easier to tap.
- Accessibility: with the OS **"reduce motion"** preference on, the flip-panel no longer flips/lifts,
  the toast no longer slides in, and the sidebar resizes instantly — the app respects users who are
  sensitive to motion (the boot animation and rail flyout already did).
- New pure `chartspec` package: a **typed, declarative chart description** (kind + series + axes +
  options) with `Validate` and data-`Extent` helpers — the framework-agnostic foundation for richer
  charts that any renderer (pure-Go SVG today, possibly D3 later) can consume. Fully table-tested.
- New pure `icon` package: the app's curated line-icon set is now a **type-safe registry** —
  compile-checked `Name` constants with `Inner()`/`Valid()`/`All()` — so icons can't be referenced by
  a typo'd string. Fully table-tested; the view layer adopts it next.
- The Allocate screen's amount split now has a **"Max per destination"** input — cap how much any one
  account/goal/debt can receive, and the overflow is held back (reported in the kept-back note). This
  surfaces the split engine's already-tested per-destination cap.
- Groundwork for the simpler time-resolution control: the period `Window` now knows when it's a single
  period and renders one clean label ("Jun 2026") instead of a redundant "Jun 2026 – Jun 2026", with a
  helper to collapse a range back to a single period.
- The dashboard **Goals widget** is now configurable: feature the **goal nearest completion** instead
  of the first, and optionally hide the target date.
- The dashboard **Budgets widget** is now configurable: cap how many budgets to show (3–20, default 6)
  and optionally show **only those near or over budget**, so it can focus on what needs attention.
- The dashboard **Accounts widget** is now configurable: set how many accounts to show (3–12,
  default 6) and whether to show only **cleared** balances (reconciled money) instead of current.
- The dashboard **To-do widget** is now configurable: open its gear to set how many tasks it shows
  (1–10, default 3), instead of a fixed three.
- Accessibility: the flip-panel overlay (quick-add, household/global settings, per-widget settings) is
  now a proper modal dialog — `role="dialog"` + `aria-modal="true"` + an accessible name from its
  title — and **Esc closes it**, so screen-reader and keyboard users get expected modal behavior.
- The top bar's **"+ Add"** button now opens a **quick-add transaction** flip panel from anywhere —
  pick the account, expense/income, amount, description, category, and date, and save without leaving
  the screen you're on. The result is announced via the toast.
- The Planning screen now has a **Savings & spending plans** card: name a what-if, set a starting
  balance, a monthly change (+ in / − out), and a horizon in months, and each saved plan shows where
  you'd land (its projected end-of-horizon balance, toned green/red) — backed by the planning engine.
  Plans list and can be deleted.
- Plans now **persist**: saved what-if scenarios survive reloads and round-trip losslessly through
  JSON/CSV export/import, with validated save (needs an id, a name, and a positive horizon).
- New **Plan** model and `planning` engine: a saved what-if scenario (a starting balance projected
  over a horizon under a set of recurring/one-time assumptions) can now be projected into a balance
  curve, its steady monthly net, and its end-of-horizon balance — composing the pure domain types
  with the existing forecast engine. Fully table-tested; persistence and the Planning UI come next.
- The Documents screen now shows a **monthly-spend summary** of the rows awaiting import — out vs. in
  vs. net per month — so you can see what a receipt or statement says you spent before committing any
  rows. Amounts use the chosen account's currency; undated rows are listed under "No date".
- New `spendsummary` package: turns extracted document rows into a **per-month spend summary**
  (money out vs. money in, with net), tolerant of varied date formats and currency symbols, surfacing
  undated rows rather than dropping them. Fully table-tested; the Documents screen view comes next.
- The Allocate screen now exposes the **goal-progress criterion** end to end: a "Goal-progress weight"
  input and a new **"Finish goals"** profile, each goal candidate carries its real completion
  percentage (so weighting it ranks goals nearest the finish line first), and the per-suggestion
  breakdown shows the goal's progress (e.g. "· goal 85%"). Saved profiles keep the new weight.
- Saved allocation profiles now remember their **goal-progress weight** too (round-trips losslessly
  through save/load and JSON/CSV export/import; older profiles without it load as 0).
- Capital allocation now has a **goal-progress criterion**: destinations funding a savings goal score
  by how close that goal is to completion (clamped 0–100%), so a "finish what's almost done"
  weighting can prioritize goals near the finish line. Fully tested and explainable (it shows in the
  per-criterion breakdown); the Allocate screen's weight control wires up next.
- Accessibility: every screen now has exactly one top-level `<h1>` — the page title in the top bar
  is now a real heading (the dashboard's in-canvas title dropped to `<h2>` to match) — so
  screen-reader users can jump to the page heading and the heading order is valid.
- Accessibility: the dashboard To-do widget's priority markers no longer rely on color alone — high,
  medium, and low now use distinct shapes (▲ / ● / ○) and each carries an accessible name
  ("High priority", etc.), so colorblind users and screen readers can tell them apart.
- Accessibility: the app-wide notice (toast) is now a **persistent live region** — it stays in the
  DOM while idle so screen readers reliably announce each new notice, and error notices are now
  `assertive`/`role="alert"` (they interrupt) while ordinary notices stay polite. So async outcomes
  (saves, imports, AI results, failures) are spoken aloud, with failures given priority.
- The browser tab and history entry now show the current screen's name (e.g. "Budgets · CashFlux")
  instead of a static title — so tabs, the back-button menu, and screen readers all name the page
  you're actually on.
- Accessibility: navigating to a new screen now moves keyboard and screen-reader focus into the
  main content region (not on first page load, so the first Tab still reaches the skip link) — so
  SPA navigation no longer strands focus on the screen you just left.
- Accessibility: a **"Skip to content"** link (the first focusable element, visible only on keyboard
  focus) jumps past the sidebar to the now-focusable `<main>`, and a clear **focus-visible ring** is
  drawn on every interactive element in both themes — so keyboard users can navigate efficiently and
  always see where focus is.
- Accessibility: the time-resolution stepper's ‹/› arrows now have `aria-label`s ("Move start
  earlier", etc.) and the accent **color swatches** are a labelled `role="radiogroup"` of `role="radio"`
  chips (each labelled by its hex, `aria-checked` reflecting the selection) — so these icon/color-only
  controls are no longer silent to screen readers.
- Accessibility: the shared **Toggle** switch now exposes `role="switch"` + `aria-checked` + an
  accessible name (from its row label), and the **Segmented** control is a `role="radiogroup"` of
  `role="radio"` buttons with `aria-checked` — so every theme/week-start/density/resolution toggle and
  every settings switch announces its state to screen readers (one central change covers them all).
- Accessibility: the SVG trend/forecast charts are now `role="img"` with a descriptive `aria-label`
  (e.g. "Net worth trend, currently $X"), and the sidebar's navigation landmark is labelled "Main
  navigation" (distinct from the breadcrumb nav) — so screen readers announce them meaningfully.
- The top bar now shows a **breadcrumb**: off the dashboard it reads "Dashboard › <screen>", with the
  Dashboard crumb a keyboard-operable button that navigates home; the current screen is marked
  `aria-current="page"`. On the dashboard it's just the title.
- The collapsed sidebar now **reveals each item's label on hover/focus** as a flyout (no rail
  widening), and every nav item + the household card carry a `title` so the name is available on hover
  and to screen readers when only the icon shows. The flyout respects `prefers-reduced-motion`.
- A **Display scale** setting (Settings → Appearance): pick 70%–130% (100% default) to make the whole
  UI larger or smaller — applied live via a `--ui-scale` CSS zoom and persisted across reloads. The
  scale value is a pure, clamped `prefs.Scale` (table-tested). Addresses the "fonts/buttons feel ~30%
  too large for me but fine for others" feedback without forcing one size on everyone.
- The Customize screen can now **save formulas**: name the current expression and save it; saved
  formulas appear in a list with their **live result**, an Edit button (loads it back into the editor),
  and delete. So a custom KPI you build once can be kept and revisited.
- Saved **formulas** (`domain.Formula` + store/state): persist a named custom calculation (expression
  + enabled flag) with full CRUD, export/import round-trip, and validated `appstate.Formulas`/
  `PutFormula`/`DeleteFormula` (id + name + expr required). Table-tested. The store behind reusable
  custom KPIs on the Customize screen / dashboard (UI to follow).
- The Allocate screen now has **editable criterion weights** and **saved profiles**: tweak the
  returns/stability/liquidity/debt weights directly (the ranking updates live), pick a built-in preset
  or one of your saved profiles to load its weights, and **Save profile** persists the current mix
  under a name (delete removes it). Custom allocation strategies beyond the four presets.
- Saved **allocation profiles** (`domain.AllocationProfile` + store/state): persist a named mix of
  capital-allocation weights (returns/stability/liquidity/debt) with full CRUD, export/import
  round-trip, and validated `appstate.AllocProfiles`/`PutAllocProfile`/`DeleteAllocProfile`.
  Table-tested. The store behind custom Allocate profiles beyond the built-in presets (picker UI to
  follow).
- Recurring cash flows can now **auto-post**: pick an account/category and flip "Auto-post", and a
  **Post due now** button creates real transactions for every due item — catching up any missed
  periods and advancing each schedule past today. Backed by a table-tested `appstate.PostDueRecurring`.
- The Recurring cash flows card now shows a **net monthly equivalent** total — every cadence
  normalized to a per-month figure and summed (e.g. a yearly insurance bill counts as 1/12 a month),
  so you can see your true monthly commitment at a glance. Backed by a pure, tested
  `domain.Recurring.MonthlyEquivalent`.
- The Planning screen now has a **Recurring cash flows** card: add a bill/paycheck/subscription
  (label, signed amount, cadence) and see/remove the list — amounts colored by sign, each showing its
  cadence and next-due date. Backed by the recurring store.
- A **Recurring** cash-flow model (`domain.Recurring` + store/state): a scheduled bill/paycheck/
  subscription with a label, signed amount, cadence (weekly/monthly/quarterly/yearly), next-due date,
  account/category, and an autopost flag. Cadence math (`Cadence.Next`, `Recurring.Advance`) and full
  persistence (CRUD, export/import round-trip, validated `appstate.Recurring`/`PutRecurring`/
  `DeleteRecurring`) are table-tested. The data model behind recurring transactions + richer forecasts
  (management UI + autoposting to follow).
- Hardened the **forecast** and **debt-payoff** engine tests: forecast now pins one-times outside the
  horizon being ignored, same-month one-times summing, negative-horizon → empty, and balances allowed
  to go negative; payoff pins single-month clearing (final payment capped), payment-equal-to-interest
  being non-viable, negative balance treated as paid, and the TotalPaid = principal + interest
  invariant across inputs.
- Hardened the **capital-allocation engine** tests: explicit determinism (Rank + Distribute give
  identical results across repeated runs), tie-stability (equal scores keep input order), and
  breakdown clamping (out-of-range APR/stability/liquidity normalize into [0,1]) — pinning the
  "deterministic & explainable" guarantee against regressions.
- Hardened the sandboxed **formula engine** with security + edge-case tests: non-allow-listed/host-like
  functions (`exec`, `eval`, `system`, `import`, even `SUM`/`Sum` — the allow-list is case-sensitive)
  are rejected, undeclared variables never silently resolve, evaluation only ever yields a
  number/string/bool, deep nesting and determinism hold, and malformed input errors instead of
  panicking. (`internal/formula` — proves the "no escape" guarantee.)
- The Rules screen now flags rules that **never run**: if an earlier rule's phrase already matches
  everything a later rule would (first-match-wins), the shadowed rule shows "Never runs — an earlier
  rule (…) already matches it." Backed by a pure, table-tested `rules.Conflicts` detector.
- Insights answers can now be **pinned**: a Pin button saves the answer to a "Pinned insights" card
  (newest first, each removable) so you can keep an explanation to revisit without adding it to your
  to-do list. Backed by the saved-insight store.
- Pinned-insight storage (`domain.SavedInsight` + store/state): persist an AI insight's text with a
  timestamp, with full CRUD, export/import round-trip, and validated `appstate.SavedInsights`/
  `PutSavedInsight`/`DeleteSavedInsight` (id + non-empty text required). Table-tested. The store behind
  a "pin this insight" action and a pinned-insights list (UI next).
- AI requests can now be **cancelled**: Insights shows a **Cancel** button while a request is in
  flight, which aborts the call (via `AbortController`) and clears any pending retry so the callbacks
  go quiet. `ai.SendChat`/`SendVisionChat`/`SendStructuredVisionChat` now return a cancel function.
- README "Hosting (SPA history fallback)" → a **Local development** note documenting that `gwc dev`
  does not yet serve the app shell for history routes (deep link / hard refresh at `/accounts` 404s;
  only built assets serve), with the workaround (start from `/` and navigate in-app, or run a
  production build behind a rewrite). Empirically confirmed this session; the deployed PWA is
  unaffected. Pins the last open B1 item as a framework-side gap.
- README "Hosting (SPA history fallback)" section documenting the rewrite rule static hosts need
  (unknown non-asset paths → `index.html`) so deep links/refreshes work, with concrete snippets for
  GitHub Pages (the auto-generated `404.html`), Netlify, Vercel, nginx, and Caddy.

### Changed
- Accessibility: the **faint/secondary text color** was darkened (light theme) / lightened (dark theme)
  so captions, hints, and meta text now meet WCAG AA (4.5:1) against both the base and elevated
  surfaces — previously ~3:1, which failed for normal-size text. (Audited with the new `contrast`
  package; the shared brand **accent** is low-contrast on the light theme but changing its hue is a
  brand decision, so it's left for review.)
- Icons are now **type-safe end to end**: `ui.Icon` takes an `icon.Name` and every call site (sidebar
  nav, top bar, household card) uses the compile-checked constants — a mistyped icon name is now a
  build error instead of a silently-blank glyph. Rendering is unchanged.
- The top bar's **time-resolution control** is simplified: the common case is now a **single period**
  with one ‹ Jun 2026 › stepper that pages the whole window (reading as one clean label). A **"This
  period"** reset appears only when you've moved off the current period, and the dual From/To range
  steppers are now behind a **"Custom range"** toggle (which collapses back to a single period when
  you leave it) — so the 90% single-period case is one tap and ranges stay available for power users.
  A **"Jump to…"** quick-pick menu offers This period / Last period / This quarter / Year to date in
  one tap.
- The Settings → Screens **show/hide toggles now cover every main-line screen**, including the Tools
  group (Planning, Allocate, Insights, Documents, Customize) and Rules — so any nav item except the
  dashboard can be hidden from the sidebar.
- Removed the placeholder **"My pages"** sidebar segment (the example "Debt payoff plan / FIRE tracker /
  Side hustle P&L" entries and the "New page" affordance) — they were mockup stubs, not real pages, so
  the rail is now just the actual screens. (Menu visibility is already configurable via the
  module-visibility toggles in Settings → Screens.)
- **One settings entry point** now: the duplicate `/settings` screen is gone — its only unique piece,
  the debug-log viewer, moved into the household-card settings panel (where currency/AI/appearance/
  data already live). The "Settings" sidebar item is removed; the household card at the bottom of the
  rail is the single way in. (Module-visibility's locked set is now just the dashboard.)

### Fixed
- The sidebar was missing five routed main-line screens — Planning, Allocate, Insights, Documents, and
  Customize were only reachable by typing the URL. Added a **Tools** nav group for them (each with an
  icon, respecting module-visibility toggles), so every main-line screen is now reachable from the menu.
- Deep-link refresh in the installed/offline PWA: the service worker now serves the cached app shell
  for navigation requests, so hard-refreshing a client-side route like `/accounts` boots the app (which
  then routes to that screen) instead of failing on a 404 or while offline. Complements the static
  `404.html` shell that covers the first load on GitHub Pages. (Cache bumped to `cashflux-v2`.)

### Changed
- Reading transactions from a receipt/statement image now uses OpenAI **structured outputs**: the
  vision request carries a strict JSON schema, so the model returns a well-formed transactions array
  instead of free-form text coaxed by prompt wording. More reliable extraction; the tolerant parser
  still handles the result. (`ai.BuildStructuredVisionRequest` / `SendStructuredVisionChat`.)
- The data CashFlux sends to OpenAI for insights is now a single explicit, tested
  `ai.FinancialContext` — by construction only aggregate totals and an account count, never payees,
  account numbers, or per-transaction detail. Both "Explain my month" and "Ask about your money" build
  their prompt from it, so the privacy scope is reviewable in one place rather than inlined per call.

### Added
- The Documents screen now has an **Import history** card listing every recorded import (newest
  first) — kind, date, status, row count, and target account — each removable. Completes the document
  lifecycle: import → recorded → reviewable/auditable.
- Importing transactions (CSV paste or receipt/statement image) now records a **Document** in the
  history — kind, time, target account, status, and (for image imports) the rows read — so every
  import leaves an auditable trail. Recorded best-effort, only when at least one transaction lands.
- An imported-**Document** record (`domain.Document` + store/state): filename, kind (CSV/image),
  upload time, target account/member, a lifecycle status (pending → extracted → imported / failed),
  and the rows read from it — persisted with full CRUD, export/import round-trip, and validated
  `appstate.Documents`/`PutDocument`/`DeleteDocument`. Table-tested. The model behind a documents
  history/audit view (recording on import + the list UI are follow-ups).
- A pure codec for OpenAI **structured outputs** (`ai.BuildStructuredRequest`): builds a chat request
  with a `response_format` JSON-schema so the model returns JSON matching a given schema, decodable
  straight into a Go struct instead of parsed out of prose. Round-trip tested. The building block for
  reliable AI extraction (e.g. document parsing) going forward.
- The Rules screen now shows a **Suggested rules** card driven by the suggester: each proposal reads
  "Categorize "Starbucks" as Cafe · Seen in 6 transactions" with an **Add** button that creates the
  rule in one click. Suggestions a rule already covers don't appear, and the card hides itself when
  there's nothing to propose.
- A pure, deterministic rule suggester (`internal/rulesuggest`): it studies how you've already
  categorized transactions and proposes auto-categorization rules where a payee/description reliably
  maps to one category — appearing often enough, agreeing ≥80% of the time, and not already covered by
  a rule — ranked by supporting evidence. No AI needed; explainable (each suggestion carries its
  support/total counts). Table-tested. The data behind a future "suggested rules" review on the Rules
  screen.

### Changed
- AI requests now retry transient failures automatically: a rate limit (429), server error (5xx), or
  network blip is retried up to three times with exponential backoff (0.5s → 1s → 2s) before giving
  up with the plain-English message. Client errors (bad key, unknown model) aren't retried. The
  decision logic (`ai.IsRetryable`, `ai.RetryDelayMS`) is pure and table-tested.
- AI failures now show plain-English, actionable messages instead of a raw error: a rejected key,
  rate limiting vs. spent quota, an unknown model, and server trouble each get their own guidance
  (e.g. "OpenAI didn't accept your API key. Check it in Settings."), and a network/CORS failure says
  to check your connection. Backed by a pure, table-tested `ai.ErrorMessage(status, body)` and an
  HTTP-status check in the fetch transport.

### Added
- Settings → AI now offers a fuller **model picker** (GPT-4o mini, GPT-4.1 nano/mini, GPT-4o,
  GPT-4.1, o4-mini) — all models the cost estimator knows, so token-cost surfacing stays accurate —
  and shows an "AI features stay off until you add a key" hint while no key is set, reinforcing the
  local-first, bring-your-own-key model.
- Insights now shows token usage and approximate cost after an AI answer — "Used 1,234 tokens ·
  about $0.0019" — using the call's reported usage and the model's pricing (just the token count when
  pricing is unknown). The fetch transport now hands the token usage back alongside the content.
- A pure AI cost estimator (`ai.EstimateCostUSD` + `ai.FormatCostUSD`): a per-model price table turns
  a response's token usage into an approximate USD cost, with longest-prefix matching for dated model
  variants and sub-cent amounts shown to four decimals. The foundation for surfacing "this used ~N
  tokens (~$0.00x)" after a call; table-tested.
- The Rules screen has an **Apply to existing** button that retroactively categorizes every
  uncategorized, non-transfer transaction matching a saved rule (first match wins, adding the rule's
  tags when a transaction has none) and reports how many it updated. This is the clean way to apply
  rules to transactions added via the CSV-paste path or imported before a rule existed —
  `appstate.ApplyRules`, table-tested.
- Importing transactions from a receipt/statement image now applies your auto-categorization rules:
  when an imported row has no category (or its name doesn't match one of yours), the saved rules and
  implicit category-name matching fill the category and tags from the description. Rows that already
  carry a recognized category keep it.
- Saved auto-categorization rules now apply as you type a transaction's description: the matching
  rule's category (and any tags) auto-fill the add form, never overriding a choice you've already
  made. Your saved rules take priority over the implicit category-name matching, and a rule's tags
  fill the tags field too. (Applies on manual entry; CSV/image import wiring to follow.)
- A **Rules** screen (System nav, `/rules`) to manage auto-categorization rules: add a rule (match
  phrase → category, with optional comma-separated tags), see all rules, edit any rule inline, and
  delete. Client-side validation shows friendly messages (match phrase + category required), and the
  hint explains first-match-wins. Built on the persisted rule store.
- Auto-categorization rules are now persisted: a `rules` table in the store with full CRUD
  (`PutRule`/`GetRule`/`DeleteRule`/`ListRules`), inclusion in the export/import dataset (lossless
  round-trip), and validated `appstate` accessors (`Rules`/`PutRule`/`DeleteRule` — a rule needs an
  id, a non-empty match phrase, and a target category). The store/state foundation for the rules
  management UI and apply-on-entry; table-tested at both the store and appstate layers.
- The dashboard now has a **Spending highlight** widget: it surfaces the single most significant
  spending change this month (reusing the same anomaly detection as the Insights card) as a one-line
  plain-English highlight with a green/red marker, or a calm "no big changes" message. Draggable and
  resizable like the other bento tiles. The anomaly detection + sentence rendering are now shared
  helpers (`detectSpendingAnomalies`, `highlightText/Tone/Arrow`) between the dashboard and Insights.
- Insights now shows an offline **Spending highlights** card: it detects categories whose spend this
  month deviates materially from their recent average (via `ledger.CategorySpendSeries` →
  `insights.Detect` over the last four months) and explains each in plain English — "Dining spending
  is up 90% — $90.00 this month vs about $47.00 a month" — with a green/red up/down marker, most
  significant first. No AI key required; the card simply doesn't appear when nothing is notable.
- `ledger.CategorySpendSeries` buckets non-transfer expense into consecutive periods (defined by a
  list of boundaries) and returns each category's per-period spend in base-currency minor units,
  oldest first — the feeder that turns transactions into the per-category histories `internal/insights`
  consumes for anomaly detection. Income/transfers are excluded, FX is converted to base, and slices
  align to the period count (zeros where idle). Table-tested.
- A pure spending trend/anomaly engine (`internal/insights`): `Detect` compares each category's
  current-period spend against the trailing average of its prior periods and flags material
  deviations, returning explainable `Anomaly` records (baseline, signed delta, whole-percent change,
  up/down direction) sorted most-significant-first. Tunable via `Options` (min baseline periods, a
  noise floor so tiny baselines don't read as huge percentages, and a percent threshold), with
  sensible `DefaultOptions`. Table-tested; the data layer behind Phase 2's "trend/anomaly highlights"
  (Insights UI wiring to follow).
- The README now opens with status badges (MIT license, Go 1.26+, WebAssembly, live demo) and a
  prominent **Live demo** callout linking to the GitHub Pages build
  (https://monstercameron.github.io/CashFlux/), with a note that it starts empty and changes stay in
  local storage. Added a **License** section pointing at `LICENSE` — closing the README's live-demo
  link and the MIT item's README follow-up.
- The project is now licensed under the **MIT License**: added a top-level `LICENSE` file (standard
  MIT text, 2026, monstercameron) and established the lightweight per-file convention with a one-line
  `// SPDX-License-Identifier: MIT` marker in the `main.go` entrypoint (placed above the `//go:build`
  constraint so the wasm build is unaffected). The README "License" section/badge will land with the
  README; a full tree-wide SPDX sweep is intentionally deferred to avoid churn and build-tag fragility.
- A CI guard for the source-of-truth English message catalog (`internal/i18n` `TestDefaultCatalogQuality`):
  every key must be dot-namespaced with no whitespace, and every key must define a non-empty string —
  so a blank or malformed entry (which would silently surface the raw key in the UI) fails `go test`
  in CI instead of shipping. Suffix fragments and literal `%` are intentionally left unconstrained.
- A Phase 0 backlog item to set the project up under the **MIT license** (`TODOS.md` §0): add a
  top-level `LICENSE` file, light SPDX (`// SPDX-License-Identifier: MIT`) references per repo
  convention, and a "License" section + badge in the README.
- A new "Future / nice-to-have (post-core)" backlog section (`TODOS.md` §5) for enhancements to pick
  up only after the core product (Phases 0–3) is complete. First item: **standalone desktop app via
  Electron** (§5.1) — wrap the existing Go→wasm / PWA build as a native installable desktop app,
  reusing the same `web/` shell and wasm bundle, sequenced after the Phase 3 / sync work.
- An app-wide toast surface for transient notices, pinned to the bottom of the screen with a
  dismiss button and a ~4.5s auto-dismiss (`uistate.Notice` atom + `app.Toast`). Bulk actions that
  previously failed silently now report problems through it: bulk recategorize, bulk mark
  cleared/uncleared, and removing a transfer's paired side all surface a friendly error instead of
  swallowing it.
- Two more silent-failure sites now report through the toast: "Mark all updated" on Accounts (per-
  account balance refresh) and the dashboard freshness nudge's "Remind me" (which now skips the jump
  to the to-do list if the reminder couldn't be created).
- The Settings data actions now confirm their outcome via the toast: Export JSON/CSV, Import, Load
  sample, and Wipe data each show a success message (or a friendly error) instead of finishing
  silently — and failures that were previously swallowed now surface. "Mark all updated" also
  reports how many balances it refreshed.

### Fixed
- The dashboard's net-worth "this month" change percentage was computed inline and divided by the
  signed baseline, so it showed the wrong direction when net worth was negative (a move from −$1,000
  to −$500 read as a decline). Extracted into a pure, tested `ledger.PercentChange` that divides by
  the baseline's magnitude, so the sign always reflects the real direction.
- The dashboard's week resolution now honors the configured week-start (Sunday/Monday) instead of
  always starting weeks on Monday. The window is seeded from the saved preference on boot, and
  changing the week-start in Settings re-snaps the dashboard's week boundaries live (new pure,
  tested `period.Window.WithWeekStart`).

### Added
- A polished boot experience: the wasm-load screen now shows an on-brand animated loader (a spinning
  accent ring around the "C" mark with the wordmark fading in), and the app settles in with a calm
  fade + slight lift once mounted. Both respect `prefers-reduced-motion`.
- Dashboard widget resize handles now appear only while you hold **Shift**, keeping the bento grid
  visually calm the rest of the time (they fade in/out; window-blur clears the state so they never
  get stuck visible).
- The dashboard time resolution (Week / Month / Quarter) now persists across reloads. Only the
  resolution is remembered — the view re-anchors to the current period on load, so you keep your
  preferred granularity without landing on a stale week or month.

### Added
- The per-widget settings panel (gear → flip) is now **schema-driven and persisted**: it renders the
  widget's registered `widgetcfg.Schema` (toggle / number / select) bound to a localStorage-backed
  `WidgetConfigs` atom, so changes survive reloads. Savings rate is the first widget with real
  settings (target rate %, show progress bar); widgets without a schema show a friendly placeholder.
- The Savings rate widget now reflects its settings: it compares the actual rate against your target
  (green at/above target, amber when positive but short, red when negative) and shows the target in
  the subline; the progress bar can be hidden.
- The Recent transactions widget has a "Rows to show" setting (3–20, default 6).
- The Net worth trend widget has a "Months of history" setting (3–12, default 6).
- The Spending breakdown widget has a "Top categories" setting (2–6, default 3; the rest group as Other).
- GitHub Pages deployment via Actions (`.github/workflows/deploy-pages.yml`): every push to `main`
  builds the wasm app and publishes it to Pages, so the latest build is reviewable from anywhere. A
  `404.html` app-shell is generated for deep-link routing.
- A per-widget settings API (`internal/widgetcfg`): each dashboard widget registers a typed `Schema`
  (toggle/number/select fields with defaults and bounds), and reads its values from a persisted
  `Config` via clamping/validating accessors — the bridge between a widget's flip-panel settings and
  its content. Pure and table-tested; savings rate ships the first schema (target rate + show-bar).
- Settings → Languages: a **Display language** picker lists every language the bundle carries and
  switches the whole UI to it. The choice persists to `localStorage` and applies on a reload, so all
  rendered strings re-resolve in the chosen language (English remains the fallback for any
  untranslated key). Completes the central-language-store loop: pick, export, import.
- Settings → Languages: **Export languages** downloads the whole language bundle as JSON (for
  translators) and **Import languages** loads a translated bundle back, merged and persisted across
  reloads — the round-trip for every language the app supports.
- The sidebar verbiage now flows through the language store: the brand, primary + System nav labels,
  the "My pages"/"System"/"New page" headers, and the household card all resolve via `uistate.T(key)`
  against the English catalog (no visible change — first screen migrated onto i18n).
- The top bar's chrome (menu-toggle tooltip and the "+ Add" button + its tooltip) now resolves via
  `uistate.T` too, completing the app-shell verbiage migration.
- The To-do screen's verbiage is now fully on the language store (form labels/placeholders, priority
  options, empty/all-done states, hide-done toggle, row actions, validation message), with shared
  `priority.*` and `common.notReady` keys other screens can reuse.
- The Members screen's verbiage is now on the language store too (add form, reassign-before-delete
  panel, member rows incl. make-default/transactions/edit/delete, net-worth-by-member, validation),
  with a shared `owner.group` key.
- The Transactions screen is now fully on the language store — the main view plus each transaction
  row's inline edit form, the category/transfer/uncategorized labels, the cleared status, and all row
  actions. **This completes the app-wide verbiage migration: every screen now renders through i18n.**
  (A few intentional exceptions stay literal: account-type names via `humanizeType`, currency/AI-model
  display names, date-format examples, and OpenAI prompt instructions.)
- The Accounts screen is now fully on the language store: the main view plus each account row's inline
  edit form, the update-balance prompt, the stale badge, the cleared-balance meta, and all row actions
  (view / update balance / mark updated / edit / archive·restore / delete).
- The Settings screen's verbiage is now on the language store (household summary + the debug-log
  viewer).
- The global settings panel is now fully on the language store: the left column (members/base-currency/
  exchange-rates/screens/freshness) and the right column (AI, appearance + theme, accent, density,
  preferences/week-start, date-format, data actions, languages). Currency/model display names and the
  date-format examples stay literal.
- The custom-fields UI is now on the language store: the manager (entity/type pickers, form, list,
  per-row meta/delete) and the per-field input control (required label, Yes/No), with the entity/type
  tables converted to i18n keys.
- The Dashboard's chrome is now on the language store: every widget title, the header cell
  (title/hint/Reset), the freshness nudge (incl. the stale-count and reminder), the savings sub-line,
  and the KPI assets/accounts sublines. (Some dynamic period+plural KPI sublines remain literal for a
  follow-up.)
- The Allocate screen's verbiage is now on the language store (profile picker + amount/reserve inputs,
  ranked suggestion rows incl. breakdown + exclude/restore, candidate name prefixes, empty states, and
  the AI-explanation card); numeric score formatting and the AI prompt stay literal.
- The Documents screen's verbiage is now on the language store (image vision-import card, CSV-import
  card, the review/edit list, and all status/error messages); the vision model prompt and the CSV
  example placeholder stay literal.
- The Customize (formula) screen's verbiage is now on the language store (calculator title/desc,
  placeholder, example chips, result/variables sections).
- The Planning screen's verbiage is now on the language store (debt-payoff calculator inputs/results,
  12-month forecast card + trim what-if, and all the projection/result notes).
- The Insights (AI) screen's verbiage is now on the language store (explain/ask cards, key hint,
  prompts' UI labels, answer + save-as-task, status messages); the AI model instructions stay English
  as they're sent to the model, not shown to the user.
- The Budgets screen's verbiage is now on the language store (add form, period picker, month stepper,
  spent/budgeted/left stats, over/near summary, and budget rows incl. on-track/near/over labels), and
  the shared `ownerSelectOptions` owner picker now localizes "Group (shared)".
- The Goals screen's verbiage is now on the language store (add form, owner/linked-account pickers,
  combined-progress stats, the progress sub-line incl. complete/by-date/save-per-month/linked
  fragments, contribute prompt, and row actions).
- The Categories screen's verbiage is now on the language store (add form, kind/parent pickers,
  reassign-before-delete panel, income/expense lists + empty states, row edit/delete), with shared
  `category.expense`/`category.income`, `common.name`, `common.reassignTitle`, `common.moveAndDelete`
  keys.
- A central language store (`internal/i18n`): a pure, table-tested message catalog keyed by stable
  dot-namespaced keys (e.g. `nav.accounts`), with English as the source/fallback language, `%s`/`%d`
  argument formatting, translation-coverage reporting (`MissingKeys`), and whole-bundle JSON
  export/import so every supported language round-trips for translators. English-only for now; screen
  verbiage is migrated onto it incrementally.

### Added
- A pure, table-tested default category scheme (`internal/catscheme`): a sensible starter set of
  income/expense categories (with a few sub-categories) for onboarding and a future "reset categories"
  action. Returns ID-less items with parents named, so the store assigns IDs.
- Pure, table-tested currency display helpers `Rates.FormatAccounting(m, target)` and
  `Rates.FormatInBase(m)` — convert a Money through the rate table and render it accounting-style in
  the target/base currency (symbol, decimals, negatives in parentheses).
- Pure, table-tested time-period presets in `internal/period` (`Previous`, `YearToDate`) plus
  `Window.Shift` (page the whole window as a unit) and `Window.IsCurrent` (is this the current period)
  — the foundation for the planned resolution-control redesign (B10). Not yet wired to the UI.

### Changed
- Extracted the dashboard's inline savings-rate calculation into a pure, table-tested
  `ledger.SavingsRate(income, expense)` (0 when income is non-positive; negative when overspent) —
  one more KPI computation moved out of view code.
- Moved the upcoming-bills "next due date" math out of the js-only dashboard into pure, table-tested
  `dateutil.NextMonthlyDue(now, day)` (next monthly due on/after today, day clamped to 1–28 so it's
  valid every month).
- Extracted the account credit-utilization calc into pure, table-tested `ledger.Utilization(balance,
  limit)` (uses the balance magnitude; ok=false when there's no limit) — the Accounts liability rows
  delegate to it.
- Added a pure, table-tested ordered-sequence + bin-packing model to `internal/dashlayout` (`Item`,
  `Pack`, `Move`, `ResizeItem`) — the foundation for iOS-home-screen-style dashboard reflow (drag =
  reorder + re-pack, multi-cell tiles never overlap). Not yet wired to the UI; the legacy
  placement/swap API stays until the dashboard is migrated. (Backlog B2.)
- Extracted the to-do list ordering/filtering into a pure, table-tested `internal/tasksort` package
  (`Order` + `Visible`); the to-do screen now delegates to it. No behavior change — the rules (open
  first, soonest due, then title; optional hide-done) are now unit-tested instead of inline in the
  js-only screen.
- Extracted transaction filtering/sorting into a pure, table-tested `internal/txnfilter` package
  (`Criteria` + `Apply` + `AbsAmount`); `uistate.TxFilter` now aliases `txnfilter.Criteria` and the
  ledger screen delegates to it. No behavior change — a core behavior is now unit-tested instead of
  living only in the js-only screen.

### Docs
- Refreshed the CLAUDE.md status section to reflect the now-comprehensive feature set (full
  CRUD/inline-edit, reconciliation, sub-categories, budget periods, preferences/themes, document
  vision import, allocation split, AI insight/nudge → tasks); multi-device sync noted as the sole
  remaining major item.

### Changed
- The sidebar now hides screens switched off via module visibility: the primary nav (Accounts,
  Transactions, Budgets, Goals, To-do) and the System items (Members, Categories) are filtered by the
  hidden-modules set, while locked screens (Dashboard, Settings) always stay. The Sidebar subscribes
  to the atom, so toggles take effect immediately.
- User-facing dates in the Transactions, Goals, and To-do lists now render using the chosen date
  format (via `prefs.FormatDate`) instead of always ISO. Each row component reads the preferences
  atom, so changing the format updates every list live.

### Added
- Budgets summary header: the Budgets screen shows total spent, total budgeted, and amount left for
  the viewed period across all budgets.
- Goals summary header: the Goals screen shows combined saved, total target, and overall progress
  percent across all goals.
- Filtered-transactions summary: the ledger shows "N shown · net $X" for the current filter — the
  count and net total (converted to base currency), updating as you filter.
- Per-member "Transactions" drill-down: each member row links to the ledger filtered to that
  member, matching the per-account drill-down.
- "Update balance" on account rows: enter an account's real balance and the app posts a cleared
  "Balance adjustment" transaction for the difference and marks it checked today — reconciling the
  computed balance to a statement without hunting for the missing entry.
- "Remind me" on the dashboard freshness nudge: when balances are stale, one click adds a
  Nudge-sourced to-do ("Update stale account balances") and jumps to the list — the create-from-nudge
  hook, completing both AI/nudge → to-do paths.
- "Save as task" on Insights: turn an AI answer/explanation into a to-do (full text in notes, source
  tagged AI) — wiring the create-from-insight hook so suggestions become actionable.
- Per-account "Transactions" drill-down: each account row has a button that filters the ledger to
  that account and jumps to it (sets the persisted transaction filter, then navigates).
- "Mark all updated" on the Accounts screen: when any balances are stale, a one-click action stamps
  every stale account as checked today, clearing the stale badges (and the dashboard freshness
  nudge) at once.
- Account "locked until" date (assets): set on the add form and the inline editor (blank unlocks),
  and the Allocate screen excludes an account locked until a future date from its suggestions (you
  can't add money to it yet).

### Changed
- The dashboard spending breakdown now rolls sub-category spend up to its top-level parent category,
  so e.g. Restaurants and Groceries are counted under Food — a cleaner high-level view.

### Added
- Re-parent categories: the category inline editor now has a parent picker too (same-kind, self
  excluded), so an existing category can be nested, moved, or promoted to top level.
- Sub-categories in the Categories screen: the add form has a parent picker (categories of the same
  kind, indented), and the category lists now display the parent/child hierarchy indented (via
  `categorytree.Flatten`). Lets you nest e.g. Restaurants and Groceries under Food.
- Category hierarchy engine (`internal/categorytree`): `Build` organizes a flat category list into a
  parent/child forest (siblings sorted by name) and `Flatten` returns a depth-tagged list for
  indented display, using the existing `Category.ParentID`. Defensive — orphans become roots and
  cycles are dropped rather than looping. Table-tested. Foundation for sub-categories.
- Cleared balance on accounts: a pure `ledger.ClearedBalance` (opening balance + only cleared
  transactions) is shown on each account row when it differs from the live balance — the figure to
  reconcile against a statement. Tested.
- Bulk mark transactions cleared/uncleared: the selection bar now has "Mark cleared" and "Mark
  uncleared" actions, so you can reconcile many at once.
- Filter transactions by cleared status (cleared / not cleared / both), persisted with the rest of
  the transaction filter — pairs with the cleared toggle to make reconciling against a statement
  easy (show only what's not yet cleared).
- Mark transactions cleared/reconciled: each transaction row has a toggle that flips the (now
  surfaced) `Cleared` flag, with the status shown in the row meta — useful for reconciling against a
  statement.
- Edit tasks inline: each to-do row has an Edit button to change the title, priority, due date, and
  notes. Every entity — including to-do — now supports inline edit.
- Edit categories inline: each category row has an Edit button to rename it and switch its kind
  (expense/income). With this, every entity supports inline edit.
- Edit members inline: each member row has an Edit button to change the name and color (members
  previously supported only add, delete, and set-default).
- Export filtered transactions to CSV: an "Export CSV" button on the transaction list downloads
  exactly the currently filtered and sorted set (shared `applyTxFilter` ensures the export matches
  the view), complementing the export-all in Settings.
- Change an account's owner: the account inline-edit form now has an owner picker (group or a
  member), updating ownership and scope. Ownership is now editable inline on accounts, budgets, and
  goals.
- Change a goal's owner: the goal inline-edit form now has an owner picker (group or a member),
  updating ownership and scope.
- Change a budget's owner: the budget inline-edit form now has an owner picker (group or a member),
  updating ownership and scope — so reassigning no longer requires deleting and recreating.
- Budget period selector in the UI: the add and inline-edit forms let you choose weekly, monthly, or
  quarterly, and each budget is now evaluated over its own period window (honoring the week-start
  preference) rather than a shared month. Each row shows its period.
- Budget periods (weekly / monthly / quarterly): the `Period` enum gained weekly and quarterly (with
  a `Label`), and `budgeting.PeriodRange` computes the current [start, end) window for a period
  containing a reference date (weekly honors the week-start preference; quarterly snaps to calendar
  quarters). Pure, table-tested. Foundation for per-budget periods in the UI.
- Link a goal to an account: the goal add and edit forms now have an optional "linked account"
  picker (populating `Goal.AccountID`), and the goal row shows "linked to <account>". Records which
  account a goal is funded from.
- Goal pace guidance: goals with a target date now show how much to save per month to hit it (via a
  new pure `goals.MonthlyNeeded` — remaining ÷ whole months left, rounded up). Shown only for
  incomplete goals with a future date.
- Edit accounts inline: each account row has an Edit button that swaps in a form for the name,
  opening balance, and the type-specific attributes (liabilities: credit limit, APR, minimum
  payment, due day, lender; assets: expected return, liquidity, stability), saving through the
  validated path. Completes inline edit across every main entity.
- Bulk transaction actions: each transaction row has a select toggle, and when any are selected a
  bar shows the count with a category picker + "Apply category" (recategorizes the selected
  non-transfer rows), "Delete selected" (transfer-aware, removing paired legs), and "Clear
  selection". Lets you reclassify or clean up many entries at once.
- Duplicate detection on document import: rows already present in the chosen account (same date and
  amount) are skipped, and the result reports how many duplicates were left out — so re-reading the
  same receipt won't double-enter transactions. Backed by `extract.Row.Signature`/`FilterNew` (tested).
- Edit rows in the document review list before importing: each extracted transaction has an Edit
  button to fix its date, description, amount, or category (e.g. correct a misread) prior to import.
- Remove rows from the document review list before importing: each extracted transaction has a ✕ to
  drop a misread, so only the rows you keep are imported.
- Document image import on the Documents screen: choose a receipt or statement image, "Read with
  AI" sends it to the OpenAI vision model (bring-your-own-key, client-side), and the extracted
  transactions appear in a review list — pick an account and import them through the validated path
  (categories matched by name, dates falling back to today). Ties together `ai.BuildVisionRequest`,
  `ai.SendVisionChat`, and `extract.ParseRows`. The CSV paste-import remains.
- Extraction parser (`internal/extract`): `ParseRows` turns an AI vision reply into reviewable
  `Row{Date, Description, Amount, Category}` values, tolerant of a bare array or an object wrapper
  (transactions/rows/items/data), numeric or string amounts, varied field names (merchant/payee/…),
  and a Markdown code fence; empties are skipped. Pure, table-tested. Bridges vision output to the
  import flow.
- Vision chat transport (`internal/ai`): `SendVisionChat` posts a multimodal request (system prompt
  + user text + one image) to OpenAI client-side with the user's key, same async one-callback
  contract as `SendChat`. The fetch promise chain is now shared via an internal `postCompletions`.
- Vision request codec (`internal/ai`): `BuildVisionRequest` marshals a multimodal OpenAI chat
  request — a system prompt plus a user message carrying text and an image (data/URL) part — for
  reading receipts and statements. The reply is plain text, read with the existing `ParseResponse`.
  Pure, table-tested. Foundation for document image import.
- Edit transactions inline: income and expense rows get an Edit button that swaps in a form for the
  description, amount, category, and date, saving through the validated path (the original
  income/expense sign and account are preserved). Transfers remain non-editable inline.
- Edit goals inline: each goal row has an Edit button that swaps in a form for the name, target
  amount, and target date (clearable), saving through the validated path. Complements the existing
  Contribute action.
- Edit budgets inline: each budget row has an Edit button that swaps in a form for the name and
  monthly limit, saving through the validated path. (Previously budgets could only be added or
  deleted.)
- Member delete now offers reassignment: deleting a member who owns accounts, budgets, or goals
  opens a panel to move everything to another owner (or the shared group) and then deletes them,
  instead of just refusing. Scope follows the new owner and transactions attributed to the member
  are re-attributed. Backed by `appstate.ReassignOwner` (tested).
- Category delete now offers reassignment: deleting a category that's still used by transactions or
  budgets opens a panel to move those records to another category and then deletes it, instead of
  just refusing. Backed by `appstate.ReassignCategory` (tested). Unused categories still delete
  immediately.
- Freshness reminders editor in the global Settings panel: per-account-type day inputs (credit
  cards, checking, savings, investments, loans, cash) that write `Settings.FreshnessOverrides`; 0
  means never flag that type. Changes apply immediately to the stale badges and dashboard widget.
- Freshness window overrides now take effect: `appstate.FreshnessWindows` layers the household's
  per-account-type overrides (from Settings) over the built-in defaults, and the Accounts stale
  badges and the dashboard Freshness widget both use it. Previously the stored overrides field was
  unused; overrides set via imported data now change which balances are flagged stale.
- The transaction list now remembers your filter and sort across reloads: the search text, account,
  category, member, date range, and sort are held in a single `uistate.UseTxFilter` atom persisted
  to localStorage. Clearing resets it. (Previously the filter reset on every reload.)
- Amount-split in the Allocate screen: enter an amount to allocate and an optional emergency buffer
  to keep back, and each ranked destination shows its suggested dollar amount (via `Distribute`),
  with a "Kept back" note for the buffer/leftover. Updates live as the amount, buffer, profile, or
  exclusions change.
- Allocation amount split (`internal/allocate`): `Distribute(ranked, total, SplitOptions)` spreads a
  total across ranked destinations in proportion to their scores, holding back an emergency-buffer
  `Reserve` and capping each at `MaxPer`, and returns per-destination `Plan`s plus the unallocated
  remainder. Even split when scores are absent. Table-tested (proportional, reserve, cap, edge
  cases). Turns the ranking into concrete dollar amounts.
- Exclude destinations in the Allocate screen: each ranked suggestion has an "Exclude" button that
  drops it from the ranking (via the new `RankWith` constraint), and an "Excluded" section lists the
  left-out destinations with "Restore". Updates live as you toggle.
- Allocation exclusion constraint (`internal/allocate`): a `Constraints` struct (currently an
  `Exclude` set of candidate IDs) with `Eligible`, plus `RankWith(candidates, weights, constraints)`
  that filters ineligible candidates before ranking. Zero-value constraints make it identical to
  `Rank`. Table-tested. Lets the user leave specific destinations out of the recommendation.
- Per-row "Duplicate" action on transactions: copies a transaction to today with a fresh id (tags
  and custom fields included), saving it through the validated path. Offered for income/expense rows
  only — a duplicate drops any transfer link, so it becomes a standalone entry rather than a broken
  transfer leg.
- Light theme: a `[data-theme="light"]` stylesheet that overrides the legacy palette variables, the
  shell's Tailwind utility colors (base/tile/hover/fg/dim/faint/line, the active-nav surface), and
  the widgets' hardcoded surface hexes (bento tiles, segmented/stepper pills, flip panel, settings
  inputs, switches, scrollbars). Choosing Light (or System on a light OS) in Settings now actually
  lightens the whole app, while the user accent stays applied on top. Completes the theme preference.
- Appearance preferences now apply to the page: `uistate.ApplyPrefs` writes `data-theme`
  (resolving "system" to the OS color scheme), `data-density`, and the `--accent` CSS variable onto
  the document root — applied on boot (before first paint) and on every change. The accent color
  retints buttons, bars, focus rings, and active states immediately; a new `[data-density="compact"]`
  stylesheet rule tightens cards, rows, and fields. (A full light-theme skin is still to come; the
  `data-theme` attribute is in place for it to hook.)

### Changed
- The Settings appearance controls (theme, accent, density) are now backed by the persistent
  preferences atom instead of throwaway local state, so the selections are remembered and saved to
  localStorage on change (they no longer reset when the panel closes).

### Added
- Theme, accent, and density added to the display-preferences engine (`internal/prefs`): `Theme`
  (dark/light/system), `Accent` (validated hex color), and `Compact` fields, with `Normalize`
  defaulting unknown themes to dark and invalid accents to the candidate-C green, plus an
  `isHexColor` check. Table-tested. Prepares the appearance controls to become real and persistent.
- Screen show/hide toggles in the global Settings panel: a "Screens" section with a Show toggle per
  hideable screen (Accounts, Transactions, Budgets, Goals, To-do, Members, Categories). Flipping a
  toggle updates the hidden-modules atom and persists to localStorage, hiding or restoring the
  screen in the sidebar immediately. Dashboard and Settings are omitted (locked visible). This
  completes module visibility end-to-end.
- localStorage-backed hidden-modules atom (`uistate.UseHiddenModules`/`PersistHiddenModules`): seeds
  the hidden-screen set from localStorage on boot and writes it back on change, so show/hide choices
  survive reloads. Loads are normalized (false/locked/stale entries dropped).
- Module-visibility engine (`internal/modules`): a `Hidden` set of hidden screen paths with
  `IsHidden`/`Toggle`/`Normalize`, plus locked core screens (home and settings) that can never be
  hidden. Toggle is immutable (returns a new minimal set) and a no-op for locked paths. Table-tested.
  Foundation for show/hide-screen settings.
- Preferences section in the global Settings panel: choose the week start (Sunday/Monday segmented
  control) and the date format (ISO / US / European / long), each showing a live example. Changes
  write the `UsePrefs` atom and persist to localStorage immediately, so they survive reloads.
- localStorage-backed preferences atom (`uistate.UsePrefs`/`PersistPrefs`): seeds the display
  preferences from localStorage on boot and writes them back on change, so week-start and date-format
  choices survive reloads — the same durable channel the dashboard layout uses (the dataset is
  re-seeded each boot). Loads are always normalized.
- Display-preferences engine (`internal/prefs`): a pure `Prefs` type (week start + date style) with
  `FormatDate` (ISO/US/EU/long), `WeekStartWeekday`, `WeekStartOf` (start-of-week honoring the
  configured first day), and `Normalize` (fills blank/unknown fields with defaults for forward
  compatibility). Table-tested. Foundation for reload-persistent user preferences.
- Custom fields on the Goals, Budgets, and Members forms — completing the rollout across all five
  entity types. Each add-form renders its registered custom fields via `CustomFieldInput`, types the
  values into the entity's `custom{}` map on save, and validates them through the matching appstate
  write path (`PutGoal`/`PutBudget`/`PutMember` now call `validateCustom`). Closes §1.16 form
  rendering: custom fields are available everywhere they're defined.
- Custom fields on the Transactions form: custom-field definitions registered for transactions now
  render in the add-transaction form (reusing `CustomFieldInput`) for income and expense entries,
  with values typed into the transaction's `custom{}` map on save and validated by
  `appstate.PutTransaction`. Transfer legs skip custom fields (an empty def list renders nothing).
- Custom fields on the Accounts form: any custom-field definitions registered for accounts now
  render as the right input (text/number/date box, yes-no or choice dropdown) in the add-account
  form via a reusable `CustomFieldInput` component (own event hook, safe in keyed lists). Values are
  typed into the account's `custom{}` map on save, and `appstate.PutAccount` validates them against
  the definitions (rejecting missing-required and wrong-typed values). Tested.
- Custom-field management UI on the Customize screen (`CustomFieldsManager`): add a field by picking
  the entity type (accounts/transactions/budgets/goals/members), a key and label, a data type
  (text/number/date/yes-no/choice), comma-separated options for choice fields, and optional vs
  required; saved through the validated `appstate.PutCustomFieldDef` path. Existing definitions list
  grouped by entity with type/required/options shown and per-row delete (own-component delete hook,
  honouring the loop-hook rule). Fulfils the Customize screen's "custom fields and formulas" promise.
- Persist custom-field definitions: `customfields.Def` now carries JSON tags and a `Validate`
  method (sound definition needs id/entity-type/key/label/known-type; choice fields need options);
  the store gains a `customfielddefs` table with full CRUD, a `CustomFieldDefsByEntity` query, and
  dataset Load/Snapshot + export/import round-trip; `appstate` exposes `CustomFieldDefs`,
  `CustomFieldDefsFor`, validated `PutCustomFieldDef`, and `DeleteCustomFieldDef`. Wipe clears the
  new table. Tested (store CRUD, dataset + export/import round-trip, wipe, Def validation).
- Custom-field definitions and validation engine (`internal/customfields`): typed `Def`
  (text/number/date/bool/select, required, select options) plus `Validate`, which checks an
  entity's `custom{}` value map against its definitions — flagging missing required fields, type
  mismatches, invalid dates, and out-of-list select values in plain English, while ignoring unknown
  keys so old data stays forward-compatible. Pure, table-tested. (Foundation for SPEC §1.16.)
- Initialize Go module and `.gitignore`.
- Product specification (`SPEC.md`).
- Project rules and code-quality standards (`CLAUDE.md`), including version-control and journaling
  policy (one feature per commit, changelog + devlog).
- Developer journal (`DEVLOG.md`) and this changelog.
- Consume GoWebComponents as a versioned Go module via `go get` (no local `replace`); Phase 0
  WebAssembly entrypoint (`main.go`) that builds and renders.
- Host page (`index.html`) with wasm boot glue, served via the `gwc dev` live-reload server.
- Wire the `gwc` runner (`.tools/gwc.exe`) and its **MCP server** (`.mcp.json`) for development.
- Framework notes (`docs/GOWEBCOMPONENTS.md`) and a quick-reference section in `CLAUDE.md` for
  new/other sessions.
- Routed app shell (`internal/app`) with top navigation and stub screens for every feature
  (`internal/screens`).
- Master feature backlog (`TODOS.md`), ordered by implementation priority.
- `.gitattributes`: normalize text to LF in the repo and on checkout (ends the Windows CRLF/LF
  warnings) and mark `*.wasm` and common asset types as binary.
- GitHub Actions CI (`.github/workflows/ci.yml`): on push/PR, runs `go vet`, `go test ./...` (the
  pure logic packages; js/wasm view packages are build-tagged out of native), and a `js/wasm` build.
- Project `README.md`: overview, feature highlights, stack, build/run, architecture, and doc links.

- `internal/money`: precise integer-minor-unit `Money` value type with safe, currency-checked
  arithmetic (`Add`/`Sub`/`Neg`/`Abs`/`Cmp`/`Sum`) and table-driven tests (backlog §1.1).
- `internal/money`: `FormatMinor`/`Money.Format` (plain decimal rendering) and `ParseMinor` (strict
  decimal → minor units with validation), round-trip tested — the basis for clean CSV and inputs.
- `internal/currency`: currency registry (code/symbol/decimals/name) + manual `Rates` table with
  base-currency `Convert`/`ToBase` (cross-currency, mixed decimals, nearest-minor rounding) + tests.
- `internal/id`: collision-resistant 128-bit hex ID generator (optional prefix; seedable source for
  deterministic tests) + tests.
- `internal/dateutil`: canonical date parsing/formatting plus month, week, and fiscal-month range
  helpers, `InRange`, and DST-safe `DaysBetween` + tests.
- `internal/domain`: core entity types (`Member`, `Account` incl. liability + allocation fields,
  `Category`, `Transaction`, `Budget`, `Goal`, `Task`) with custom-field maps, plus validated
  enumerations (`AccountClass`/`AccountType`/`CategoryKind`/`Scope`/`Period`/`TaskStatus`/
  `TaskPriority`/`RelatedType`/`TaskSource`), `AccountType.Class()`, transaction classification, and tests.
- `internal/ledger`: account `Balance`/`RunningBalances`, period income/expense totals (transfers
  excluded), `NetWorth` (assets − liabilities), and per-owner net rollups — all multi-currency via
  base conversion + tests.
- `internal/budgeting`: scope-aware `Spent` (individual vs group), `Evaluate`/`EvaluateAll` with
  remaining, percent, and ok/near/over `State` thresholds — multi-currency + tests.
- `internal/goals`: goal `Remaining`, `Percent` (clamped), `IsComplete`, and `Project` (read-only
  completion estimate from an assumed monthly contribution) via `Evaluate` + tests.
- `internal/freshness`: per-type staleness `Windows` (defaults + `Merge` overrides), `IsStale`,
  `DaysSinceUpdate`, and `StaleAccounts`; archived/exempt/untracked accounts never go stale + tests.
- `internal/validate`: per-entity validators returning all `Issues` at once (required fields, valid
  enums, positive amounts, currency consistency, class/type match, score/day ranges, related refs) + tests.
  Completes the Phase 1 pure-logic services layer.
- `internal/store`: pure `Dataset` aggregate + `Settings`, with schema-versioned JSON `Export`/
  `Import` (migration; rejects newer schema) and a lossless round-trip test. Storage-backend-agnostic
  — also the sync/transfer payload.
- `internal/store`: in-memory **SQLite** store backed by the pure-Go (no-cgo) `ncruces/go-sqlite3`
  driver, with `Load`/`Snapshot` clean dataset ingress/egress + round-trip tests. Verified to build
  for `js/wasm` (browser) and run natively.
- `internal/store`: per-entity CRUD (Put/Get/Delete/List for members, accounts, categories,
  transactions, budgets, goals, tasks) and query helpers (transactions by account/category/member/
  date-range via SQLite `json_extract`; tasks by status) + tests.
- `internal/store`: `TransactionsToCSV`/`TransactionsFromCSV` — human-readable CSV with decimal
  amounts, header-name column matching (order/extra-column tolerant), generated ids for id-less rows,
  and per-line error reporting; lossless round-trip tested.
- `internal/store`: `Get/PutSettings` accessors, atomic `Wipe`, and a valid `SampleDataset` starter
  seed (validated in tests). Completes the Phase 1 persistence layer.
- `internal/logging`: `log/slog`-based `Handler` writing human-readable lines to any `io.Writer`
  plus a bounded, concurrency-safe `Ring` buffer for an in-app log viewer; supports level filtering
  and `With`/`WithGroup` contextual attrs + tests.
- `internal/appstate`: the UI↔persistence/logic seam — owns the in-memory store + slog logger, with
  typed read accessors, validated write-through (`Put*`/`Delete*`), JSON export/import, and
  `Init`/`Default`; wired into `app.Run` to seed sample data on boot. Pure Go + native tests.
- Accounts screen: first real, data-backed screen — assets/liabilities grouped with live per-account
  balances (`internal/ledger`) and a net-worth/assets/liabilities summary, reading from `appstate`.
  Shared money display helpers (`fmtMoney`, amount classes).
- Dashboard screen: real headline metrics — net worth, this-month income/expense (via
  `ledger.PeriodTotals` over the current month), active-account count, and a recent-activity list.
- Accounts add form: create an account (name, type, owner, currency, opening balance) with
  validated write-through and a reactive refresh (`state.UseAtom` revision bump). First mutating
  feature. Added row/form/amount styles to the host page.
- Accounts per-row delete via an `AccountRow` component (stable `On*` hook) with `MapKeyed` keyed
  rendering; deleting refreshes the screen and net-worth summary.
- Transactions screen: add income/expense (account-aware currency, category/date), newest-first
  list, and per-row delete (`TransactionRow`), all with validated write-through and reactive refresh.
- Budgets screen: current-month spend vs limit per budget via `internal/budgeting` with a colored
  ok/near/over progress bar, plus add and per-row delete.
- Goals screen: savings goals with a progress bar (% complete + remaining via `internal/goals`),
  optional target date, plus add and per-row delete.
- To-do screen: tasks with priority badges and due dates — add, complete-toggle, delete, sorted
  (open first, then by due date).
- Dashboard design exploration: five HTML/Tailwind candidates in `design/`; **candidate C selected**
  (flat neutral-dark, Fraunces serif headings + accounting figures, bento grid, per-widget
  grip/title/gear header, drag-reorder + edge resize, gear→flip settings, collapsible icon sidebar,
  global-settings flip).
- Granular, reusability-focused component backlog for the candidate-C dashboard UI (`TODOS.md` §1.7c),
  every item referencing `design/candidate-c.html`.
- Dashboard design-system foundation in the host page: Fraunces + Inter web fonts, the candidate-C
  Tailwind palette/type config, and the full candidate-C component CSS (bento grid, unified widget
  header, drag/resize handles, flip-settings panel, dark scroll pane, sidebar collapse, control
  primitives) — ported verbatim from `design/candidate-c.html`, ready for the Go component port.
- `internal/money`: `Group` (thousands separators) and `FormatAccounting` — accounting-style display
  (`$1,234.56`, negatives in parentheses like `($240.55)`, always `decimals` places, caller-supplied
  symbol) for the candidate-C figure style; table-driven tests. Pure, no currency-registry dependency.
- `internal/ui`: new shared design-system package (Go port of `design/candidate-c.html`) with a
  reusable, props-driven `Icon` primitive — the candidate-C stroked SVG icon set (dashboard, accounts,
  transactions, budgets, goals, to-do, settings, page, plus, menu) that inherits color/size from the caller.

- PWA web manifest (`manifest.webmanifest`) + theme-color/apple meta tags, making CashFlux installable
  as a standalone dark-themed app (Phase 3 start; icons and a service worker follow).
- PWA service worker (`sw.js`, registered on load): network-first caching of same-origin GETs (core
  shell pre-cached on install) so the app stays fresh online and loads offline; cross-origin calls
  (e.g. OpenAI) pass through uncached.
- PWA install prompt: an "Install CashFlux" button appears when the browser offers installation
  (`beforeinstallprompt`) and hides after install.

### Changed
- Retargeted the legacy screen palette (the shared CSS variables) to candidate-C values, so the
  non-dashboard screens (Accounts, Transactions, Budgets, Goals, To-do) — cards, stats, rows, forms,
  bars — match the new flat neutral-dark bento shell, with squared (4px) corners.
- App shell replaced the top-navigation chrome with the candidate-C layout: a fixed left rail
  (brand + icon-led primary navigation with active highlighting and router navigation) and an
  independently scrolling main pane with a sticky top bar (menu toggle, page title, Add action).
  `internal/app` now composes `internal/ui` primitives (`Icon`); screen bodies render inside the
  new `main` scroll pane.
- Rail completed with the candidate-C lower groups: a **My pages** section (example custom pages with
  colored page icons + a muted "New page" action), a **System** group (Settings), and a
  household card pinned to the bottom that shows live member count and base currency and opens
  Settings. `navItem` is now reusable across all groups (optional path for placeholders, custom icon
  class, muted styling).
- Collapsible rail: the top-bar menu button toggles the sidebar into 58px icon-only mode (labels,
  captions, brand text, and household summary hidden), coordinated by a shared `rail:collapsed`
  state atom so the button and rail stay in sync.
- `internal/ui`: reusable `Segmented` (mutually-exclusive option toggle) and `StepperPill`
  (label + prev/next chevrons) control primitives — generic and props-driven, each interactive
  child its own component so click hooks stay stable in lists.
- `internal/period`: pure time-resolution model for the dashboard control — `Resolution`
  (week/month/quarter) with anchor `Truncate`/`Step`/`Label` and `Range` (from/to anchors → a
  half-open reporting range, clamped). Table-driven tests cover quarter boundaries, week starts,
  cross-year stepping, and range spanning. Pure, native-tested.
- `internal/period`: immutable `Window` value (resolution + from/to anchors + week start) with the
  control's stepping rules — `SetResolution` (re-snaps anchors), `StepFrom`/`StepTo` (move one
  anchor, clamping the other so from ≤ to), `Range`, and from/to labels. Drops straight into UI
  state; clamp behavior table-driven tested.
- Time-resolution control in the top bar: a Week/Month/Quarter `Segmented` toggle plus From/To
  `StepperPill`s, backed by a shared `internal/uistate` window atom over `period.Window`. The
  dashboard now derives its income/spending period from this control (re-rendering on change) instead
  of a hardcoded current-month range; stat labels are now period-relative ("Income"/"Spending").
- `internal/ui`: reusable `Widget` shell — the candidate-C bento cell with the unified header (grip ·
  centered title · gear) and a padded body, props-driven (title, body, grid span, draggable,
  resizable, gear handler) so every dashboard widget is `Widget` + content. Optional edge resize
  handles; gear is its own component for stable hooks in lists.
- `internal/ui`: reusable `FlipPanel` settings overlay — the candidate-C dimmed/blurred backdrop with
  a card that lifts and 3D-flips to a settings back face (centered title, close button, scrollable
  body, dark Save/Cancel footer). Generic (title, body, size, Save/close handlers) and reused by both
  per-widget and global settings; the open animation runs once on mount via `UseState`/`UseEffect`.
- `internal/ui`: reusable `Toggle` (pill switch) + `ToggleRow` (labeled settings row), `Swatch`
  (color chip) + `SwatchPicker` (accent row) control primitives — the building blocks of the settings
  forms, generic and props-driven, each interactive element its own component.
- Dashboard rebuilt as the candidate-C **bento grid**: a full-width header cell plus four KPI widgets
  (Net worth, Income, Spending, Liabilities) built from the live ledger and shown as accounting
  figures (`$1,234.56` / `($240.55)`, green/red tone). Each KPI is the reusable `Widget` shell +
  content; Income/Spending follow the time-resolution window. New `fmtAccounting`/`figTone` helpers.
  The Net worth tile shows a real month-over-month change (▲/▼ %) via `ledger.NetWorthSeries`; the
  Income/Spending tiles show the period plus the deposit/transaction count for it.
- Recent transactions widget (2×2) on the dashboard: newest activity as a compact table with short
  dates and accounting amounts (green/red), in the reusable `Widget` shell.
- `internal/ui`: reusable `ProgressBar` primitive — the candidate-C thin rounded track + colored fill
  (clamped percent, tone class, extra spacing), reused by budgets, goals, and savings-rate widgets.
- Budgets widget (1×2) on the dashboard: current-month spend per budget with an ok/near/over
  `ProgressBar` and percent (green/amber/red), via `internal/budgeting`. Always month-scoped since
  budgets are monthly.
- Goals widget (1×1): the first goal's progress (saved / target + percent and target date) via
  `internal/goals`, in the reusable `Widget` shell with a `ProgressBar`.
- To-do widget (1×1): up to three open tasks, each with a priority-toned dot (high = amber).
- Accounts widget (2×1): a small grid of up to six active account balances (accounting figures,
  negatives toned red) via `ledger.Balance`.
- `internal/chart`: pure SVG path geometry for dashboard sparkline/area charts — `Points` (scale a
  series into a w×h box, y-inverted, padded, flat/single series centered), `LinePath`, and
  `AreaPath` (closed to a baseline). Table-driven tested; no rendering dependency.
- `internal/ledger`: `NetWorthSeries` — net worth as of each cutoff time (transactions strictly
  before the cutoff counted), in base currency, for the net-worth trend chart. Table-driven tested.
- `internal/payoff` (Phase 2 start): pure debt-payoff projection — `Project(balance, aprPercent,
  payment)` simulates monthly APR accrual and a fixed payment, returning months-to-zero, total
  interest, and total paid, with `ok=false` when the payment can't cover the interest. Table-driven
  tested.
- `internal/forecast`: pure balance/net-worth projection over a horizon — `Project(start, recurring,
  oneTimes, months)` applies the recurring monthly net plus any one-time events each month and
  returns the end-of-month balance series; `MonthlyNet` sums the recurring flows. Table-driven tested.
- `internal/ai`: OpenAI chat request/response shapes + a pure codec — `BuildRequest` marshals a
  chat-completions body; `ParseResponse` extracts the assistant content and surfaces API errors /
  empty responses; `ParseUsage` reads token counts. Round-trip tested (no network; the fetch
  transport is a separate js layer).
- `internal/ai`: browser `fetch` transport (`SendChat`) — posts a chat request with the user's key
  asynchronously and calls back with the content or a plain-English error; the only network spot.
- `internal/rules`: pure auto-categorization engine — `Rule{Match, SetCategoryID, SetTags}` with
  case-insensitive substring matching over payee+description, first-match-wins `FirstMatch`,
  `Category`, and `Tags`. Empty matches never fire. Table-driven tested.
- Insights screen (replacing the stub): an **"Explain my month"** AI narrative generated client-side
  from your live figures via OpenAI with your own key; prompts to add a key in Settings when absent,
  with loading and error states. Plus a **natural-language "Ask about your money"** box that answers
  questions using your figures as context.
- Planning screen (replacing the stub): a **debt-payoff calculator** — enter balance, APR, and
  monthly payment to see months-to-zero, total interest, and total paid, updating live via the
  `internal/payoff` engine, with a friendly message when the payment can't cover the interest, and an
  optional **extra-payment** input that shows how many months sooner it clears and how much interest
  it saves. Plus a **12-month net-worth projection** chart (current net worth + this month's net cash
  flow, via `internal/forecast` + the area chart) with a what-if "trim monthly spending by…" input
  that re-projects and reports the improved 12-month figure.
- `internal/allocate`: pure capital-allocation scorer — normalizes each candidate on returns,
  stability, liquidity, and debt-reduction, combines by a user `Weights` profile into an explainable
  `Score` + `Breakdown`, and `Rank`s candidates highest-first. Table-driven tested; deterministic.
- Allocate screen (replacing the stub): builds candidates from asset accounts, high-interest
  liabilities, and **unfinished goals**, ranks them by a chosen profile (Balanced / Maximize returns /
  Safety & access / Pay down debt), and shows each suggestion's score bar and per-criterion breakdown.
  An optional **"Explain with AI"** narrative summarizes why the ranking suits the profile (BYO key).
- `internal/formula`: tokenizer for the sandboxed formula language — numbers (incl. leading-dot),
  identifiers, double-quoted strings, arithmetic/comparison operators, parens, and commas; errors on
  unterminated strings, stray `=`/`!`, and unexpected characters. Table-driven tested.
- `internal/formula`: recursive-descent `Parse` → AST (NumberLit/StringLit/Ident/Unary/Binary/Call)
  with correct precedence (comparison < additive < multiplicative < unary), left-associativity,
  parens, and function calls. Errors on malformed input. Table-driven tested via a canonical s-expr.
- `internal/formula`: allow-list `Eval` (completes the sandboxed engine) — arithmetic, comparisons
  (numeric + string equality), variable resolution from an `Env`, and the functions `sum/avg/min/max/
  count/abs/round/if`. Errors on unknown var/function, arity, division/modulo by zero, and type
  mismatch; no host access. Table-driven tested.
- Customize screen (replacing the stub): a live **formula calculator** — write an expression over your
  figures (net worth, assets, liabilities, income, expense, account/transaction/member counts) and
  see the result instantly via the sandboxed engine, with the available variables and their current
  values listed, plus one-click example chips (savings rate, spending ratio, etc.). Variables now
  include budget/goal/task counts alongside the financial figures.
- `internal/ui`: `AreaChart` helper renders a filled gradient sparkline from a value series (feeding
  the pure `chart` geometry into an `<svg>`). Net worth trend widget (1×2) on the dashboard: the
  current figure over a six-month end-of-month area chart via `ledger.NetWorthSeries`.
- Cash flow widget (2×1): income (green, up) vs expense (red, down) bars for the last four months,
  scaled to the largest bar, with the current month's net figure — via `ledger.PeriodTotals`.
- Savings rate widget (2×1): the share of the period's income that wasn't spent, as a big figure and
  a `ProgressBar` (toned green/red).
- Spending breakdown widget (2×1): a segmented bar of the period's expenses by category (top three
  plus "Other") with a color-keyed legend; totals converted to base currency.
- Upcoming bills widget (2×1): the next due date and minimum payment for each liability account that
  has them, soonest first, with due dates within a week toned amber. Completes the candidate-C widget
  catalog (12 widgets).
- Per-widget settings: each widget's gear opens its settings in the `FlipPanel` (driven by a shared
  `settings:target` atom + a `SettingsHost` mounted at the shell root). The settings back face has an
  editable title and behavior toggles (show on dashboard, allow moving/resizing, compact), built from
  the `ToggleRow` primitive. The household card's global panel opens too (body coming next).
- The rail's household card now opens the global settings flip panel (via the shared settings atom)
  instead of navigating to the Settings route.
- Global settings: the OpenAI API key and model inputs now persist to the store (`Settings.OpenAIKey`/
  `OpenAIModel`), so the Insights screen can use them. The key stays on-device. The "+ Add member"
  button now closes the panel and opens the Members screen.
- Global settings panel body: a two-column flip-panel form with live household member chips, base
  currency, and editable FX rate rows (left) and AI (BYO key toggle + key + model), Appearance (theme
  `Segmented` + accent `SwatchPicker` + compact), and Data action buttons (right). Built from the
  shared control primitives; appearance controls hold local state and data actions are wired next.
- Export JSON data action: downloads the full dataset as `cashflux.json` (the portable export/import
  + sync payload) via `appstate.ExportJSON` and a small Blob/anchor browser-download helper.
- `internal/appstate`: `ExportCSV` (transactions → CSV), `ImportTransactionsCSV` (parse CSV rows →
  validated writes, best-effort), `LoadSample` (replace with the sample dataset), and `Wipe` (clear
  all data) — the data-action seams; tested natively.
- Documents screen (replacing the stub): paste a CSV of transactions and import them (no AI needed) —
  header-name column matching, decimal amounts, negatives for expenses; reports how many imported.
- Global settings Data actions wired: Export CSV (download), Import (file picker → replace dataset),
  Load sample, and Wipe (with a confirm dialog). A shared `data:revision` atom is bumped on bulk
  changes so the dashboard re-renders; added `pickFile`/`confirmAction` browser helpers.
- `internal/dashlayout`: pure bento layout model — `Placement` (column/row + spans with CSS grid
  string helpers), `Layout` with the candidate-C `Default` arrangement, immutable `Swap` (exchange
  two widgets' cells) and `Resize` (clamped spans). Table-driven tested; underpins drag-reorder/resize.
- The `Widget` shell now sources its grid placement from a shared `dashboard:layout` atom (falling
  back to caller defaults), so reorder/resize changes flow to every widget via state.
- Drag-to-reorder: dragging one bento widget onto another swaps their grid cells (`dashlayout.Swap`
  via a shared drag-source atom; `dragover` allows the drop with `Prevent`). The dragged widget dims
  (`.drag`) and the source clears on drag-end.
- Resize handles: a widget's right/bottom edge handles now cycle its column/row span
  (`dashlayout.Resize`, clamped to the 4×3 grid bounds) and re-place it live. Every dashboard widget
  is now both draggable and resizable.
- Bento layout persistence: the arrangement is saved to `localStorage` after every reorder/resize and
  reseeds the layout atom on load, so a customized dashboard survives reloads (falls back to the
  default arrangement when absent or invalid).
- Reset layout action in the dashboard header restores the default bento arrangement and clears the
  saved layout.
- Transactions: account-to-account transfers — a "Transfer" kind swaps the category picker for a
  "To account" picker and creates paired entries (debit + credit, each with `TransferAccountID`) that
  move both balances and are excluded from income/expense. Same-currency only for now; rows labelled
  "Transfer". Deleting either leg removes the reciprocal so balances stay consistent.
- Transactions: a filter bar (description search + account + category + member pickers + a From/To
  date range, with Clear) narrows the ledger list, with a distinct "No matching transactions" state.
- Transactions: a comma-separated tags field on income/expense entries; tags show on the row
  (`#tag`) and the search box matches tags as well as descriptions.
- Transactions: a sort selector (newest first / largest amount / payee A–Z).
- Transactions: auto-suggests a category as you type the description (matching against category names
  via `internal/rules`), without overriding a category you've already chosen.
- Transactions: a "Repeat last" button pre-fills the form from the most recent transaction (kind,
  amount, account, category, transfer destination).
- Goals: a "Contribute" action per goal adds an entered amount to its saved total (advancing the
  progress bar) via a quick prompt. The list now sorts incomplete goals first, then alphabetically.
- Top bar: the "+ Add" button now navigates to the Transactions screen (was inert).
- Budgets: a month stepper (‹ month ›) lets you view budget spend for any month, not just the current
  one. A health line summarizes how many budgets are over or near their limit.
- To-do: an optional notes field on tasks, shown in the task row.
- To-do: a "Hide done" / "Show all" toggle to filter completed tasks, with an "All done 🎉" state.
- Accounts: archive/restore an account from its row — archived accounts move to a separate "Archived"
  section and drop out of the assets/liabilities lists and net-worth totals (already excluded by
  `ledger`).
- Categories screen: add categories (name + income/expense), listed grouped by kind with per-row
  delete; reachable from a new "Categories" rail item (tag icon) under System. Deleting a category
  still used by transactions or budgets is blocked with a plain-English message.
- Members screen: add household members (name + color), list with a color swatch, set the default
  member, and per-row delete; reachable from a new "Members" rail item (users icon) under System.
  Deleting a member who still owns accounts/budgets/goals is blocked with a plain-English message.
  Also shows a "Net worth by member" rollup (each member + group-shared) via `ledger.NetByOwner`.
- Freshness nudge widget (full-width, dashboard): a friendly reminder of which account balances look
  stale (via `internal/freshness`) with days since each was last updated; the bento grew to 8 rows.
- Settings screen (replacing the stub): a household summary (base currency, member/account/category
  counts) and an in-app **debug log viewer** (newest first, with Refresh) reading the slog ring buffer.
- Accounts: a "Mark updated" action per (active) account sets its `BalanceAsOf` to today, clearing the
  staleness flag the freshness nudge surfaces.
- Accounts: a welcome card with a "Load sample data" button when there are no accounts yet
  (onboarding) — seeds the store via `appstate.LoadSample`.
- Accounts: a "Stale" badge on accounts whose balance is overdue for a refresh (via
  `freshness.IsStale`), complementing the dashboard nudge and the per-row "Mark updated" action.
- Accounts: liability rows with a credit limit show their credit utilization ("N% of limit used").
- Accounts: the add form reveals a **liability sub-form** (credit limit, interest APR, minimum
  payment, due day, lender) when a liability type is selected — feeding the Upcoming-bills widget and
  credit-utilization display, which previously had no data entry path.
- Accounts: for asset types, the add form reveals **allocation attributes** (expected return APR,
  liquidity, stability) — giving the Allocate engine real per-account scores instead of zeros.
- Persistence switched from IndexedDB to pure-Go in-memory SQLite (`ncruces/go-sqlite3`, no cgo, no
  dependency on browser web storage); the JSON `Dataset` remains the portable import/export and sync
  payload. (Confirmed pure-Go SQLite compiles for `js/wasm` and runs in the browser.)
- Expanded `TODOS.md` into a granular, per-entity/service/screen backlog covering the full spec.
- Serve web assets from `web/` (clean project root); restyled host page with a dark theme.
- Require bottom-up SDLC build order in `CLAUDE.md` (data model → services/logic with tests →
  persistence → state → UI last).

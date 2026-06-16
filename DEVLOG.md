# CashFlux — Developer Journal

Narrative companion to `CHANGELOG.md`. Newest entries first. Capture decisions, trade-offs,
problems and fixes, and what's next.

## 2026-06-16 — Appearance prefs: apply to the DOM

- `uistate.ApplyPrefs(p)` reflects prefs onto `document.documentElement`: `data-theme` (with
  `resolveTheme` consulting `matchMedia` for "system"), `data-density`, and `--accent` via
  `style.setProperty`. Added `LoadPrefs()` so boot can apply the saved prefs without a hook (the
  atom can't be read outside a component). `app.Run` calls it right after `appstate.Init`, before
  mounting, so the first paint is already correct — no flash of defaults. `savePrefs` calls it too,
  so changes are instant.
- **Why accent works immediately:** the legacy `:root --accent` var is wired through the
  design-system CSS (buttons, `.bar-fill`, `.field:focus`, active nav), so overriding it on the root
  cascades everywhere at once. Density got a new `[data-density="compact"]` block tightening cards,
  rows, and fields.
- **Honest scope note — theme is half-applied on purpose.** The candidate-C surfaces are authored in
  fixed dark hexes (Tailwind config + hardcoded values), so a real light skin is a sizable CSS pass.
  This commit lands the mechanism (the `data-theme` attribute is set, system-resolved) and the two
  pieces that work cleanly today (accent, density). Picking "Light" sets the attribute but the skin
  is deferred to its own feature, so I don't ship a broken half-light look.
- **Next.** Either the light-theme stylesheet (a `[data-theme="light"]` palette pass) or move on to
  another backlog item (persist-last-filter, transaction duplicate, allocation constraints).

## 2026-06-16 — Appearance prefs: wire the controls

- Replaced the three local `UseState`s (theme/accent/compact) in `globalSettingsForm` with reads off
  the normalized `pr` and writes through the existing `savePrefs` (normalize → atom set →
  `PersistPrefs`). Dropping three hooks is safe — they were removed wholesale, so hook order stays
  consistent across renders.
- The Segmented/SwatchPicker/ToggleRow now reflect the persisted values and remember them; closing
  and reopening the panel keeps the selection, and it survives reload.
- **Note.** This makes the *preference* real and durable, but it does not yet *apply* visually — the
  page still renders dark with the green accent regardless. That is the next step: on change and on
  boot, set a `data-theme`/`data-density` attribute and the accent CSS variable on the document root
  so the choice actually changes the look.

## 2026-06-16 — Appearance prefs: extend the engine

- The settings panel's theme / accent / density controls have been local-only React-style state all
  along (they reset on close). Making them real reuses the prefs pipeline, so step one is extending
  `internal/prefs`: added `Theme` (dark/light/system), `Accent` (hex string), and `Compact` (bool).
- **Decision — validate the accent in `Normalize` with a tiny `isHexColor`.** Accent comes from a
  color `<input>` but persisted data could be anything; rather than trust it, normalize rejects
  non-`#rgb`/`#rrggbb` strings back to the default green. Keeps the "always-usable persisted data"
  invariant the rest of prefs already holds.
- `Default()` now seeds dark + green; `Compact` defaults to false (zero value), so no special case.
  Existing week-start/date tests unchanged; added theme/accent/hex-color tests.
- **Next.** Wire the settings appearance controls to these fields (atom + PersistPrefs), then apply
  them to the DOM (a `data-theme`/`data-density` attribute + accent CSS var on the document root),
  and seed that application on boot so it survives reload.

## 2026-06-16 — Module visibility: Settings toggles (feature complete)

- Final step: a "Screens" section in the global settings left column. A package-level
  `hideableScreens` list (label + path, excluding the locked dashboard/settings) drives a
  `ui.ToggleRow` per screen. Because `ToggleRow` is a `CreateElement` component owning its own hook,
  the per-row toggles render safely in a plain loop — no parent hook-ordering worry.
- Each toggle's `OnChange` calls `toggleModule(path)` → `Toggle` (immutable) → atom `Set` →
  `PersistHiddenModules`. Both the form (subscribed via `UseHiddenModules`) and the sidebar
  re-render, so a hidden screen vanishes from the rail the instant you flip it, and the choice
  survives reload.
- Module visibility is now end-to-end: pure engine (locked + toggle) → localStorage atom → sidebar
  filter → Settings toggles. Closes §1.18's show/hide-screens item.
- **Next.** Other §1.18 items remain (theme/density, fiscal-month start, budgeting methodology
  selector), or move to a contained Phase-3 sync primitive. Will pick the next granular increment.

## 2026-06-16 — Module visibility: sidebar filtering

- `Sidebar` now reads `uistate.UseHiddenModules().Get()` and filters: the primary nav is built into a
  `visibleNav` slice (skipping hidden paths) before the `MapKeyed`, and the two hideable System items
  (Members, Categories) are wrapped in `If(!hidden.IsHidden(path), …)`. Settings and Dashboard are
  locked in `internal/modules`, so they are never filtered — no special-casing needed here.
- Reading the atom subscribes the Sidebar, so flipping a toggle re-renders the rail at once.
- **Scope note.** Hiding is a *navigation* concern: the routes stay registered, so a hidden screen
  reached directly by URL (or the unknown-path fallback) still renders. That is deliberate — we are
  decluttering the rail, not building access control.
- **Next.** The Settings panel show/hide toggles, which write `Toggle` + `PersistHiddenModules` for
  each hideable screen — the last step to close this feature.

## 2026-06-16 — Module visibility: the persistence atom

- `uistate/modules.go` — the third localStorage-backed atom (after layout and prefs), same shape:
  `UseHiddenModules` seeds from `loadHiddenModules()` (key `cashflux:hidden-modules`, normalized,
  empty set on miss/parse error), `PersistHiddenModules` marshals the normalized set back. Empty set
  = everything visible, which is the right default.
- Thin plumbing again — the `Normalize`/locked-path logic all lives in the tested `internal/modules`
  package; this file is JSON ↔ localStorage only.
- **Next.** Filter the sidebar nav by the hidden set (shell.go), then add per-screen show/hide
  toggles to the global Settings panel.

## 2026-06-16 — Module visibility: the pure engine

- New backlog item (§1.18 module-visibility toggles). Same reload-persistent shape as preferences,
  so same approach: pure logic first, then a localStorage atom, then sidebar filtering + settings
  toggles. Pure package `internal/modules`.
- **Decision — lock the home and settings screens.** Hiding the dashboard or the settings screen
  (which is where you'd un-hide things) would be a footgun, so `IsLocked` makes them permanently
  visible and `Toggle`/`Normalize`/`IsHidden` all respect that. Cheap guard, big safety win.
- **Decision — immutable Toggle returning a minimal set.** `Toggle` clones rather than mutating
  (the atom value should be replaced, not edited in place) and the set only ever stores `true`
  entries, so it serializes compactly and `Normalize` can clean stale/false/locked keys on load.
- **Next.** `uistate` localStorage atom for the hidden set, then filter the sidebar nav by it and
  add per-screen toggles to Settings. (Routes themselves stay registered; hiding is a nav concern,
  and a hidden screen reached by URL still works.)

## 2026-06-16 — Reload-persistent preferences: wiring dates through (feature complete)

- Final step: the three user-facing date displays (TransactionRow, GoalRow, TaskRow) now format via
  `prefs.FormatDate` instead of `dateutil.FormatDate`. Each row reads `uistate.UsePrefs().Get()` at
  the top of its component — unconditionally, because GoalRow's date sits inside an
  `if !TargetDate.IsZero()` and a hook there would be conditional. Reading the atom also subscribes
  the row, so flipping the format in Settings re-renders every list immediately.
- Left `dateutil.FormatDate` in place for machine/edit contexts (date `<input>` values, parsing)
  where ISO is required — preferences only change *display*, not the canonical storage/parse format.
- §1.18 week-start + date-format preference is now end-to-end: pure engine → localStorage atom →
  Settings UI → live rendering, all surviving reload. Theme/density and fiscal-month start remain as
  separate future prefs.
- **Next.** Move to the next backlog area — likely module-visibility toggles (show/hide screens,
  also a reload-persistent preference) or a contained Phase-3 sync primitive.

## 2026-06-16 — Reload-persistent preferences: the Settings UI

- Step 5 (UI): a "Preferences" block in the global settings back-face. Week start is a `Segmented`
  (its OnSelect is a plain prop, so no parent hook needed); date format is a `Select`, whose
  `OnChange` *does* register a parent hook — so `onDateStyle` is declared unconditionally at the top
  with the other event hooks, keeping hook order stable.
- Both controls funnel through one `savePrefs` closure that normalizes, sets the atom, and calls
  `PersistPrefs`. Reading uses `prefsAtom.Get().Normalize()` so the rendered selection always
  reflects a valid value. Date options show a live example (2026-06-05, 06/05/2026, …) so the choice
  is self-explanatory — plain-English-UI rule.
- **Next.** The preference is captured and persists, but the screens still render dates via
  `dateutil.FormatDate`. Final step: route user-facing date rendering through `prefs.FormatDate` so
  the choice actually shows up in Transactions, Goals, etc.

## 2026-06-16 — Reload-persistent preferences: the persistence atom

- Step 4 (state): `uistate/prefs.go`, a near-mirror of `layout.go`. `UsePrefs` is a `state.Atom`
  seeded from `loadPrefs()` (localStorage key `cashflux:prefs`, normalized, defaults on miss/parse
  error); `PersistPrefs` marshals the normalized prefs back. No store involvement — by design,
  preferences live outside the dataset because the store is wiped on every boot.
- This keeps the wasm/persistence layer thin: all the meaning (formatting, week math, normalization)
  is in the tested `internal/prefs` package; this file is just JSON ↔ localStorage plumbing.
- **Next.** A Settings form (global panel) to choose week start and date style, calling
  `atom.Set` + `PersistPrefs`; then route the screens' date rendering through `prefs.FormatDate`.

## 2026-06-16 — Reload-persistent preferences: the pure engine

- New backlog area: preferences that survive a reload (week start, date format). Established first
  that store-backed `Settings` do *not* survive reload — `app.Run` calls `appstate.Init(nil, true)`,
  which re-seeds the in-memory SQLite store on every boot. The only durable channel is localStorage
  (that is how the dashboard layout persists). So preferences will follow the layout pattern: a
  localStorage-backed atom, seeded on boot, written on change — separate from the dataset.
- Per the SDLC rule, started with the pure logic: `internal/prefs`. `Prefs{WeekStart, DateStyle}`
  plus `FormatDate`, `WeekStartWeekday`, `WeekStartOf`, and `Normalize`. Keeping the display logic in
  a platform-free package means it is unit-tested on native Go and the wasm layer stays a thin
  localStorage + form shell.
- **Decision — `Normalize` everywhere a value is read.** Persisted prefs may be partial or from an
  older build, so every accessor normalizes first; blanks/unknowns fall back to defaults rather than
  producing an empty layout string. Same forward-compatibility stance as the custom-field defs.
- **Next.** Wrap `prefs` in a `uistate` localStorage atom (`UsePrefs`/`PersistPrefs`), then a
  Settings form to edit it, then route date rendering in the screens through it.

## 2026-06-16 — Custom fields: Goals, Budgets, Members (rollout complete)

- Applied the now-proven pattern to the last three entity forms in one pass: each gets a
  `customVals` value-map state, a `<entity>Defs := app.CustomFieldDefsFor(...)`, the `onCustom`
  push-up closure, a `MapKeyed` of `CustomFieldInput`s in the form, `Custom:
  customValuesToMap(...)` on the built entity, and a reset on success. The matching appstate write
  paths (`PutGoal`/`PutBudget`/`PutMember`) now call `validateCustom`.
- **Grouped as one feature deliberately.** The three integrations are byte-for-byte the same shape
  the Accounts/Transactions commits already established; splitting them into three near-identical
  commits would be noise, not granularity. The unit of work here is "finish the rollout", and it
  maps to one checklist line in §1.16.
- §1.16 form rendering is now closed for all five entity types (accounts, transactions, budgets,
  goals, members). The whole custom-fields feature — model, validate, persist, manage UI, render on
  forms, export/import — is complete.
- **Next.** Pick up the next backlog area: module-visibility toggles / reload-persistent
  preferences, or a contained Phase-3 sync primitive.

## 2026-06-16 — Custom fields: Transactions form

- Second entity wired up, and the reusable pieces paid off: the Transactions add-form now renders
  transaction custom fields via the same `CustomFieldInput` + `customValuesToMap` + parent value-map
  pattern, and `appstate.PutTransaction` gained the `validateCustom("transaction", …)` guard.
- **Decision — custom fields apply to income/expense, not transfer legs.** A transfer is two paired
  rows; hanging user fields off one leg is ambiguous, so when the kind is Transfer the form passes an
  empty def slice (`formTxnDefs = nil`) and nothing renders. Keeps the model honest without inventing
  transfer-pair custom semantics.
- Confirmed the empty-slice-flattens trick again: `MapKeyed(nil, …)` renders nothing, so no `If`
  guard is needed around the custom inputs.
- **Next.** Budgets, Goals, Members forms — same mechanical integration — then §1.16 is fully closed.

## 2026-06-16 — Custom fields: rendering on entity forms (Accounts first)

- The defs now drive real inputs. `CustomFieldInput` is a reusable component that picks the control
  for a field's type and reports `(key, value)` up to the parent form, which owns a
  `map[string]string` value state. Both event hooks (`onText` for inputs, `onSel` for selects) are
  declared unconditionally at the top so hook order is stable whatever the field type — the
  component is then safe to render from a `MapKeyed` list.
- **Decision — push values up, don't pull them down.** Each input is controlled and emits changes to
  a single parent map rather than holding its own state, so the submit handler can read every value
  at once and build the typed `custom{}` map (`customValuesToMap`: numbers→float64, yes/no→bool,
  else string; empties omitted so optional fields stay unset).
- **Validation lives in `appstate.PutAccount`, not the view.** Added `validateCustom`, which loads
  the account defs and runs `customfields.Validate`, returning `validate.Issues` — so any save path
  (not just this form) enforces required/typed custom fields. A defs *read* error never blocks a
  save (logged and ignored); only real value problems reject.
- **Framework gotcha hit:** `If(cond, MapKeyed(...))` doesn't compile — `MapKeyed` returns
  `[]ui.Node` and `If` wants a single `ui.Node`. The fix is to drop the `If`: an empty def list
  yields an empty slice that flattens to nothing, same as `Div(..., MapKeyed(...))`.
- **Next.** Repeat the integration for Transactions (and other entities) so custom fields are
  available everywhere they're defined.

## 2026-06-16 — Custom fields: management UI

- Step 5 (UI last) for §1.16: `CustomFieldsManager`, a thin shell over the now-tested persistence.
  Add-field form (entity type, key, label, type, options, required) + grouped list with per-row
  delete. Per-row delete is its own component (`CustomFieldRow`) so its `OnClick` hook sits at a
  stable render position — the cardinal framework rule.
- **Decision — host it on the existing Customize screen, not a new route.** That screen is already
  subtitled "Custom fields and formulas" but only did formulas; dropping the manager above the
  calculator fulfils the promise and keeps the nav uncluttered. No routing changes.
- **UI choices.** The choice-field options input only appears when the type is "Choice"
  (`If(isChoice, …)`); required is a plain Optional/Required select rather than a checkbox to match
  the other dropdown-driven forms. Validation errors from `Def.Validate()` surface inline via the
  shared `validate.Issues` error string. Entity list is curated (the five entities users actually
  annotate) rather than reflected, so the labels read in plain English.
- **Next.** The defs exist and persist; the remaining step is rendering these fields as inputs on
  the actual entity forms (accounts/transactions/…) and validating `custom{}` on save — a per-form
  integration I'll do entity by entity.

## 2026-06-16 — Custom fields: persistence layer

- Step 3 of the SDLC for §1.16: persist `CustomFieldDef`s. Added a `customfielddefs` table to the
  SQLite store (same id+JSON-document shape as every other entity), full CRUD, a
  `CustomFieldDefsByEntity` query (via `json_extract` on `$.entityType`, mirroring the
  transactions-by-account pattern), and wired the new entity through `Load`/`Snapshot`, `Wipe`'s
  `allTables`, and the `Dataset` aggregate so export/import round-trips it.
- **Decision — keep `Def` in `internal/customfields`, not `internal/domain`.** The type and its
  validation are inseparable, so the package that validates owns the type. `store` and `appstate`
  importing `customfields` is a clean one-way dependency (it only pulls in `dateutil`). Added JSON
  tags to `Def` so the persisted shape is stable and lowercase like the rest of the dataset.
- **No schema-version bump.** The new `customFieldDefs` array is additive and `omitempty`; old
  exports decode fine (nil slice), so `SchemaVersion` stays at 1 — the migration guard is reserved
  for shape changes that actually break old data.
- `appstate.PutCustomFieldDef` runs `Def.Validate()` and adapts the plain-English messages into the
  existing `validate.Issues` error type, so the write path behaves like every other entity.
- **Next.** State seam is thin here (defs are read directly), so the remaining work is UI: a
  management screen to add/edit/remove defs per entity type, then rendering the inputs on entity
  forms and validating `custom{}` on save.

## 2026-06-16 — Custom fields: the validation core first

- Started SPEC §1.16 (user-defined custom fields) bottom-up, per the SDLC rule: model + validate
  before any store or UI. New pure package `internal/customfields`.
- **Design.** `Def` is a strongly-typed field definition (id, entity type, map key, label, one of
  five `FieldType`s, optional select `Options`, `Required`). This honours code-rule #7: the core
  schema stays strongly typed; extensibility comes from *validated* custom fields, not from
  loosening entities into untyped maps. `Validate(defs, values)` collects *all* issues (not
  first-fail) so a form can show every problem at once, and returns plain-English messages.
- **Trade-offs.** Custom values arrive from JSON, so numbers are `float64` — `isNumber` accepts the
  float and int kinds rather than insisting on one. Dates are validated through the existing
  `dateutil.ParseDate` (single source of truth for the YYYY-MM-DD format) instead of a second
  parser. Unknown keys in a value map are ignored rather than flagged, so data written before a def
  existed (or after one is removed) never hard-fails — forward/backward compatible by default.
- **Next.** Persist `CustomFieldDef`s (store + export/import round-trip), expose them via appstate,
  then a thin Settings UI to manage defs and render the inputs on entity forms — strictly in that
  order.

## 2026-06-15 — Porting candidate C: design-system foundation

- Resumed the `/loop`, now executing §1.7c (port the chosen design into Go components), one feature
  per commit. First feature is the foundation everything else references.
- **Decision — adopt Tailwind (CDN) + the candidate-C custom CSS** rather than re-authoring every
  utility class as semantic CSS. The mockup was authored in Tailwind; faithful reproduction and low
  drift matter more here than shedding a CDN dependency, and it keeps the port mechanical. The
  palette/type scale live in `tailwind.config`; the bespoke component CSS (bento, widget header,
  drag/resize, flip panel, scrollbar, sidebar collapse, settings controls) is a single `<style
  id="design-system">` block ported verbatim from `design/candidate-c.html`.
- **Additive, not a switch-over:** the old semantic theme + top-nav shell still render so the build
  stays green at every commit; the new tokens just become available. The scroll-pane and body
  scrollbar selectors were namespaced (`main.cf-scroll`, `body.cf`) so they only apply once the new
  shell opts in, avoiding restyling the current screens mid-migration.

- Added the **accounting money formatter** as pure logic before any UI uses it (SDLC bottom-up):
  `money.Group` for thousands separators and `money.FormatAccounting` for the candidate-C figure
  style — symbol-prefixed, always two decimals, negatives in parentheses (`($240.55)`). Kept in
  `internal/money` (pure, native-tested) and currency-registry-free by taking the symbol as an
  argument, so the js/wasm screen layer composes `currency.Symbol(...)` + this without leaking the
  registry into money. Table-driven tests cover grouping boundaries, zero, sub-unit, and millions.

- Confirmed the framework's `html/shorthand` exposes SVG element constructors (`Svg`/`Path`/`Rect`/
  `Circle`/`G`/`Line`/`Polyline`/`Polygon`), a generic `Attr`/`Attrs` for arbitrary attributes, and a
  full pointer/drag event set (`OnPointerDown/Move/Up`, `OnDragStart/Over/Drop/End`) — so the SVG
  icons, charts, and the drag-reorder/resize interactions can all be expressed natively in Go.
- Started the shared design-system package `internal/ui` (js/wasm-tagged) with the first reusable
  primitive: **`Icon`** — the candidate-C stroked SVG set as a single props-driven component
  (`Icon(name, extra...)`), color via `currentColor`, size via caller classes. Builds clean for
  `GOOS=js GOARCH=wasm`.

- Ported the **app shell** to the candidate-C layout: `internal/app/shell.go` now renders a fixed
  left rail (`Sidebar`) + an independently scrolling `main.cf-scroll` pane with a sticky `TopBar`,
  replacing the old top-nav `Shell`/`NavBar`. The rail's primary nav is data-driven (`primaryNav()`)
  and each entry is rendered by a `navItem` component so its click hook stays stable (On*-in-loops
  rule). Imported the framework `ui` as `uic` to avoid colliding with our `internal/ui` (`ui`).
- Kept design data in the design layer: the route→icon mapping lives in `primaryNav()` (not the
  screen registry), so `internal/screens` stays free of presentation concerns. Phase-2 routes and
  Settings are reachable by URL but not yet in the rail (the My-pages/System groups come next).
- Full `GOOS=js GOARCH=wasm` build is green (~22 MB). Top bar's menu toggle, time-resolution control,
  and the Add action are present but static for now — wired in upcoming features.

- Completed the rail: **My pages** (example custom pages with colored page icons + a muted "New page"
  action), **System** (Settings), and a bottom **household card** that reads live member count + base
  currency from `appstate` and navigates to Settings (the global-settings flip panel replaces that
  navigate later). Generalized `navItem` into the one reusable rail primitive — optional `Path`
  (empty = non-navigating placeholder, used by the example pages until custom pages are real),
  `IconClass` for per-item icon tinting, and `Muted` styling. Section headers are direct `<div>`
  children of `<nav>` so the collapsed-rail CSS (`nav > div { display:none }`) hides them cleanly.

- Added `.gitattributes` (LF normalization + binary marks); `git add --renormalize` was a no-op
  (repo blobs were already LF), so the Windows CRLF warnings are gone with a one-file commit.
- **Collapsible rail**: the framework's `state.UseAtom` is global-by-id and re-renders every
  subscribed component, so a shared `rail:collapsed` bool atom cleanly coordinates the top-bar menu
  button (toggles) and the `Sidebar` (adds the `collapsed` class → the CSS does the 58px icon-only
  switch). No `syscall/js` needed. Collapse persists across navigation (atom is global); persisting
  across reloads waits on the prefs/settings wiring.

- Built the first **shared control primitives** in `internal/ui`: `Segmented` and `StepperPill`,
  both generic and props-driven. Each follows the export-thin-wrapper-over-CreateElement pattern so
  every call site is its own component instance with isolated hooks, and the per-option `segButton`
  is itself a component (the On*-in-loops rule). These back the time-resolution control next but are
  written for reuse anywhere (theme toggle, paging, etc.).

- Modeled the time-resolution control **bottom-up first**: new pure `internal/period` package wraps
  `dateutil` with a `Resolution` (week/month/quarter) and anchor math — `Truncate` (snap to unit
  start), `Step` (move by whole units), `Label` ("Jun 2026" / "Q3 2026" / "Jun 15 – Jun 21"), and
  `Range` (from/to anchors → half-open reporting range, with a to<from clamp). Table-driven tests
  green on native Go. The UI will just hold the resolution + two anchors in state and call this.

- Added an immutable `period.Window` (resolution + from/to anchors + week start) holding all the
  control's stepping/clamping rules as pure, tested methods (`SetResolution`, `StepFrom`/`StepTo`
  with the from ≤ to clamp, `Range`, labels). This is the value the UI will store in a single atom,
  so the top-bar control and dashboard share one source of truth and the view stays logic-free.

- Wired the **time-resolution control**: new `internal/uistate` package holds the shared dashboard
  window in one atom over `period.Window` (a neutral home so neither the app shell nor screens own
  it and there's no import cycle). The top-bar `ResolutionControl` composes `Segmented` +
  two `StepperPill`s; each action just stores the next immutable `Window` (no date math in the view).
  `Dashboard` now reads the same atom for its period range and re-renders on change — first proof the
  shared-state plumbing works end to end. Full js/wasm build green.

- Built the keystone **`Widget` shell** in `internal/ui`: the candidate-C bento cell (square outlined
  `.w`, unified `.wh` header = grip · centered title · gear, padded `.wbody`) as one generic
  props-driven component (title, body, grid span, draggable, resizable, `OnGear`). Every widget will
  be `Widget` + content, so the chrome is defined once. Grid placement is emitted as inline style per
  axis; the gear is its own component for hook stability in widget lists.

- Browser/DOM check: the gwc dev server is up at :8080 serving the current wasm, but the gwc MCP
  browser-driving tools (`gwc_dom`/`gwc_eval`/`gwc_screenshot`) aren't connected in this headless
  loop context, and the playwright lane isn't set up — so automated DOM assertions aren't available
  here. Staying on compile-green + review as the gate; the owner can eyeball the live server.
- Built the **`FlipPanel`** primitive (`internal/ui`): the candidate-C settings overlay — dimmed/
  blurred backdrop, a card that lifts and 3D-flips to a settings back face (centered title, close,
  scrollable body, dark Save/Cancel footer). Generic over title/body/size/handlers and reused by
  **both** per-widget and global settings (the reusability directive). The open animation runs once
  on mount: a `shown` `UseState` flipped to true inside a `UseEffect` (stable dep + guard against
  re-run), so the CSS transition animates from front→back rather than appearing pre-flipped.

- Added the remaining **control primitives** to `internal/ui`: `Toggle` (`.switch`) + `ToggleRow`
  (labeled `.toggle-row`), and `Swatch` (`.swatch`) + `SwatchPicker` (accent row). Each interactive
  element is its own component (hook stability), and `SwatchPicker` keys each chip by color. These
  complete the shared-control set the settings forms compose.

- Re-sequenced: built a **real bento dashboard** before the settings wiring, so the gear has a live
  widget to open. `Dashboard` now renders the `.bento` grid — a full-width header cell + four KPI
  widgets (Net worth, Income, Spending, Liabilities) composed from the `Widget` shell with
  accounting figures via new `fmtAccounting`/`figTone` helpers (`money.FormatAccounting` +
  `currency`). Income/Spending honor the shared time window; Net worth/Liabilities read
  `ledger.NetWorth`. Aliased `internal/ui` as `uiw` in screens (framework `ui` keeps the bare name).
  `recentTransactions` stays for the next widget (unused package funcs are legal Go; build confirms).

- Added the **Recent transactions** widget (2×2): newest six as a compact table (short "Jan 2" dates,
  payee, accounting amount with green/red tone) in the `Widget` shell. Display-only, so rows build in
  a plain loop (no per-row hooks needed). Reuses the existing `recentTransactions` helper.

- Added the `ProgressBar` primitive (`internal/ui`): a display-only helper (no hooks → plain
  function, not a component) rendering the candidate-C track + fill with a clamped percent and tone
  class. Reused by budgets/goals/savings-rate widgets next.

- Added the **Budgets** widget (1×2): current-month spend per budget via `budgeting.EvaluateAll`,
  each row a label + percent (toned green/amber/red by ok/near/over) over a `ProgressBar`. Kept it
  month-scoped on purpose (budgets are monthly) rather than following the dashboard window, so the
  percentages stay meaningful. Confirmed `appstate.Default` is `*appstate.App` (build passes).

- Added three more bento widgets reusing tested services: **Goals** (first goal's progress via
  `goals.Percent`), **To-do** (up to three open tasks, priority-toned dots), and **Accounts** (up to
  six active balances via `ledger.Balance`, negatives toned). All compose the `Widget` shell +
  `ProgressBar`; confirmed `appstate` exposes `Goals()`/`Tasks()`/`Accounts()`.

- Built chart geometry **bottom-up first**: new pure `internal/chart` package maps a value series to
  SVG coordinates (`Points`, y-inverted with padding; flat/single series centered) and emits
  `LinePath`/`AreaPath` strings with fixed precision for stable, testable output. Table-driven tests
  assert exact path strings. The view (`internal/ui`) will just feed these to an `<svg>`.

- Added `ledger.NetWorthSeries` (pure + tested): net worth as of each cutoff time by counting
  transactions strictly before it and reusing `NetWorth`, so first-of-month cutoffs give an
  end-of-month trend. Test walks a single account across Jan/Feb with a deposit and a withdrawal.

- Added the `AreaChart` ui helper (feeds `chart` paths into an `<svg>` with a gradient fill, built
  from generic `Tag("defs"/"linearGradient"/"stop")` SVG nodes) and the **Net worth trend** widget
  (1×2): current figure + a six-month end-of-month area chart from `ledger.NetWorthSeries`. Cutoffs
  are first-of-month from M-5 to M (AddMonths(start, i-4)).

- Added the **Cash flow** widget (2×1): income/expense bars for the last four months (div bars,
  height % scaled to the largest bar across all months) plus the current month's net, from
  `ledger.PeriodTotals` (confirmed it returns expense as a positive magnitude). Used a tiny div-bar
  approach rather than SVG since the mockup does.

- Added **Savings rate** (period income saved %, big figure + bar) and **Spending breakdown**
  (segmented bar of period expenses by category — top three + "Other" — with a color-keyed legend,
  all converted to base currency, sorted desc). Both reuse the `Widget` shell; breakdown reuses the
  window range already computed in `Dashboard`.

- Added the **Upcoming bills** widget (2×1), completing the 12-widget catalog: next due date + min
  payment per liability account (clamped due-day, soonest first, within-a-week dates toned amber),
  via a small `nextDue` helper. The whole candidate-C bento is now live data on the reusable shells.

- Wired **per-widget settings**: new `settings:target` atom in `uistate` (`SettingsTarget{Kind,ID,
  Title}` — closed/widget/global). The `Widget` gear defaults to opening its own panel (computes the
  open-closure during render so the `UseSettings` hook stays at a stable position, not in the click
  handler), overridable via `OnGear`. A `SettingsHost` component mounted at the shell root renders the
  `FlipPanel` for the active target and nothing (`Fragment()`) when closed — so each open is a fresh
  mount and the flip animation replays. The widget settings back face (editable title + behavior
  toggles via `ToggleRow`) holds local state for now; persisting visibility/layout to the store
  arrives with the layout model. Confirmed `internal/ui` can depend on `uistate` without a cycle.

- Built the **global settings** panel body: a two-column form inside the `FlipPanel` (760×560) with
  live household member chips, base currency, and sorted editable FX rate rows on the left; AI
  (BYO-key toggle + key + model), Appearance (theme `Segmented` + accent `SwatchPicker` + compact),
  and Data action buttons on the right. Reuses every shared control primitive. Members/base/FX are
  real reads from `appstate`; appearance is local state for now; the Data buttons are present but
  wired in the next feature (export/import/wipe need js download + store mutation + refresh).

- Wired the **Export JSON** data action: a tiny `downloadBytes` helper (the one DOM-touching spot for
  file egress — Blob + transient anchor via `syscall/js`) downloads `appstate.ExportJSON()` as
  `cashflux.json`. Generalized `dataBtn` into its own `dataButton` component taking an `OnClick` so
  the remaining actions slot in cleanly.

- Added the data-action seams to `appstate`: `ExportCSV` (via `store.TransactionsToCSV`), `LoadSample`
  (replace with `store.SampleDataset` — `store.Load` replaces, as the import path proves), and `Wipe`.
  Native test loads sample → asserts populated → wipes → asserts empty, plus a CSV smoke test.

- Wired all global-settings **Data actions**: Export CSV (download), Import (`.json` file picker →
  `ImportJSON`), Load sample, and Wipe (guarded by a native confirm). Added `pickFile` (file input +
  `FileReader` → bytes, releasing the js callbacks after read) and `confirmAction`. Refresh uses a
  shared `data:revision` atom: bulk actions bump it and `Dashboard` reads it so it re-renders behind
  the still-open panel. Other screens refresh on their own navigation/rev atoms.

- Started bento drag/resize **bottom-up**: new pure `internal/dashlayout` holds the grid model —
  `Placement` (col/row + spans, with `GridColumn`/`GridRow` CSS string helpers), `Layout` with the
  candidate-C `Default()` (14 widgets), and immutable `Swap`/`Resize`. Table-driven tests cover the
  CSS strings, swap symmetry + immutability, unknown-id no-ops, and span clamping. The UI will hold a
  `Layout` in an atom, source each widget's placement from it, and write back swaps/resizes.

- Wired placement through state: new `uistate.UseLayout()` atom (default `dashlayout.Default()`); the
  `Widget` shell looks up its own `Placement` by ID and uses its CSS grid strings when present, else
  the caller's `GridColumn`/`GridRow`. No visual change (default == the hardcoded positions) but now a
  single `layout.Swap`/`Resize` written to the atom re-places every widget. Widgets already subscribe
  to the atom via the hook, so reorder/resize will re-render the whole grid.

- Wired **drag-to-swap**: the framework's `Prevent` wrapper calls `PreventDefault` before the handler,
  so `OnDragOver(Prevent(func(){}))` enables the drop. `OnDragStart` stashes the widget id in a shared
  `drag-source` atom; `OnDrop` swaps via `dashlayout.Swap` written to the layout atom (re-placing both
  widgets) and clears the source; `OnDragEnd` clears it if dropped outside. The dragged cell dims via
  `.drag`. No `DataTransfer` needed — the atom carries the source id.

- Made the **resize handles functional**: the right/bottom edge handles cycle the widget's col/row
  span via `dashlayout.Resize` (clamped to the 4×3 grid), re-placing it live through the layout atom.
  Chose click-to-cycle over pointer-drag for now — it's reliable without browser testing and the math
  stays in the tested `dashlayout`; smooth pointer-drag resize is a later polish. Only `kpi-networth`
  currently carries handles (as in the mockup); enabling them across all widgets is the next commit.

- Enabled drag+resize on **all** widgets (normalized the net-worth widget then `Resizable`-stamped
  every call), and added **layout persistence to `localStorage`**: `PersistLayout` marshals the layout
  after each drag/resize and `loadLayout` seeds `UseLayout`'s initial value (falling back to
  `Default()` when absent/invalid). Chose `localStorage` over the store because the SQLite store is
  in-memory and re-seeded on boot — only browser storage actually survives a reload. Missing widgets
  fall back to their default placement, so adding widgets later degrades gracefully.

- Restyled all non-dashboard screens in one move by **retargeting the legacy CSS variables** to the
  candidate-C palette (base `#0e0e0f`, tile `#121214`, border `#2a2a2c`, up/down, radius 4px). Since
  the old screen components (cards, stats, rows, forms, bars) are all driven by these vars, they now
  match the flat neutral-dark shell without rewriting any Go — and they already inherit Inter from the
  shell root. Per-screen bento-style polish can follow, but the jarring blue theme is gone.

- Added a **Reset layout** action in the dashboard header cell: restores `dashlayout.Default()` to the
  atom and persists it, undoing any drag/resize. (Persisting the default overwrites the saved layout,
  which is the intended "clear customization" behavior.)

- Synced `TODOS.md` (§1.7c mostly done) and implemented **account transfers** (§1.11 ★). The model is
  paired entries: Balance only counts a transaction against its own `AccountID`, so a transfer needs a
  debit on the source and a credit on the destination, both carrying `TransferAccountID` (so
  `IsTransfer` excludes them from income/expense). The Transactions form gains a "Transfer" kind that
  swaps the category picker for a "To account" picker; submit validates distinct accounts + matching
  currency (cross-currency deferred) and writes both legs. Known gap: deleting one leg orphans the
  other — a paired delete is the follow-up.

- **Paired transfer delete**: deleting a transfer leg now finds and removes its reciprocal (accounts
  swapped, amount negated, same date) so balances don't drift. Heuristic match (no schema change /
  migration); a shared transfer-group id would be more robust if duplicate transfers collide — noted.

- Added **transaction filters**: a filter bar (case-insensitive description search + account picker,
  Clear button) narrows the in-memory list before render, with a separate "No matching transactions"
  empty state distinct from "No transactions yet". Date-range/category/member filters + persistence
  can follow.

- Added **account archive/restore**: each account row gets an Archive/Restore toggle (`AccountRow`
  grows a second action hook); archived accounts move to a dedicated "Archived" card and leave the
  assets/liabilities lists and net-worth totals (`ledger` already excludes them). Toggling just flips
  `Archived` and re-puts through the validated `appstate` path.

- Extended the transaction filter bar with a **category** picker (combines with search + account).

- Added the **Categories screen** (add name + income/expense kind, grouped lists, per-row delete via
  `CategoryRow`), registered `/categories`, and surfaced it in the rail's System group with a new
  `tag` icon. Category edit + color + delete-reassignment are follow-ups.

- Added the **Members screen**: add (name + color picker), list with a color swatch + Default badge,
  Make-default (flips `IsDefault` across members through the validated put path), and per-row delete
  (`MemberRow`). Registered `/members` with a new `users` rail icon under System. Delete-guard for
  members with owned entities is a follow-up.

- Added a **Freshness nudge** widget (full-width row 8 — grew the bento to 8 rows + a `dashlayout`
  placement, test count 14→15): friendly "N balances could use a refresh" with per-account days-since
  via `freshness.StaleAccounts`/`DaysSinceUpdate`. One-tap update + dismissal are follow-ups.

- Added a one-tap **"Mark updated"** action per active account (`AccountRow` grows a third hook,
  rendered only when not archived): sets `BalanceAsOf` to now via the validated put, clearing the
  staleness the freshness nudge reports. A full "update balance" (enter a new figure → adjustment txn)
  is the richer follow-up.

- Added a **member delete-guard**: deletion is blocked (with a plain-English count) when the member
  still owns any account, budget, or goal, so those references can't be orphaned. A reassign flow is
  the richer follow-up.

- Added a **category delete-guard** mirroring the member one: blocks deletion (with a count) when any
  transaction or budget still references the category. A pick-a-replacement reassign flow is the
  richer follow-up.

- Replaced the **Settings** stub with a real page: a household summary (base currency + member/
  account/category counts) and an in-app **debug log viewer** (the slog `Ring`, newest-first, Refresh
  button). Heavy editing stays in the global panel + dedicated screens to avoid duplication.

- Added **transaction tags**: a comma-separated tags field on income/expense entries (`parseTags`
  trims/drops empties), stored on `Transaction.Tags` and shown on the row as `#tag`. Tag-based
  filtering is a follow-up.

- Added **contribute-to-goal**: a per-goal Contribute button prompts for an amount (`promptText`
  wraps `window.prompt`), parses it in the goal's currency, and adds it to `CurrentAmount` via the
  validated put — advancing the progress bar. Auto-progress from a linked account is a follow-up.

- Wired the top-bar **"+ Add"** button to navigate to Transactions (was a no-op).

- Added a **budget month stepper**: a `monthOffset` state + ‹/› pills drive `dateutil.AddMonths` so
  you can review any month's budget spend, not just the current one.

- Added an optional **notes** field to tasks (form input + row display), stored on `Task.Notes`.

- Extended the transaction search to match **tags** as well as descriptions (`matchesText`).

- Added an **onboarding** welcome card on the Accounts screen (shown when there are no accounts) with
  a "Load sample data" button wired to `appstate.LoadSample` + the screen's revision bump.

- Added **transaction sort** (newest first / largest amount via `absAmount` / payee A–Z), applied to
  the filtered list before render.

- Replaced the Net worth KPI's static "Assets X" subline with a real **month-over-month delta** (▲/▼
  integer %) computed from `ledger.NetWorthSeries` at this month's start; falls back to the assets
  line when there's no prior figure. Removes the last fabricated "2.4%" placeholder from the mockup.

- Added a **Hide done / Show all** toggle to the To-do list (filters completed tasks; distinct
  "All done 🎉" state when everything's hidden).

- Sorted the **goals list incomplete-first** (then alphabetical) via a stable sort using
  `goals.IsComplete`, so active goals stay on top.

- Income/Spending KPI sublines now show the period plus the real deposit/transaction **count** for it
  (a `plural` helper), replacing the bare period label and matching the mockup's "June · 1 deposit".

- Hygiene pass after ~67 features: `go vet ./...` (js/wasm) is clean; `gofmt -w` tidied alignment in
  seven files (mostly table-comment columns + one entities.go gap). Native tests still green.

- Added a **per-account "Stale" badge** (amber) on the Accounts screen via `freshness.IsStale`,
  closing the loop with the dashboard freshness nudge and the per-row "Mark updated" action.

- Added a **budget health summary** line ("N over budget · M near the limit") from the evaluated
  statuses, shown above the budget list when any are over/near.

- Added **credit utilization** to liability account rows (an `accountMeta` helper appends "N% of limit
  used" when a liability has a credit limit), using the row's already-computed balance.

## 2026-06-15 — Phase 2 begins (bottom-up): debt payoff

- Phase-1 core is broadly built out (all candidate-C UI + accounts/transactions/budgets/goals/todo/
  members/categories/settings with filters, transfers, archive, freshness, tags, etc.), so I started
  **Phase 2 bottom-up** with a pure logic package: `internal/payoff`. `Project(balance, aprPercent,
  payment)` simulates monthly APR compounding + a fixed payment, returning months-to-zero, total
  interest, and total paid; `ok=false` when the payment can't cover the interest (so it would never
  clear) and a 1200-month cap as a backstop. Table-driven tests: 0% APR exact (10 months), an
  interest-bearing case (~11 months, interest > 0), payment-too-small, already-paid, zero-payment.

- Surfaced payoff in the **Planning screen** (replaced the stub): a live debt-payoff calculator
  (balance / APR / monthly payment → months, total interest, total paid) wired to `payoff.Project`,
  recomputing on each keystroke (no submit) with a plain-English non-viable message.

- Built the **allocation engine core** `internal/allocate` (pure, tested): `Candidate` criteria
  normalized to 0..1 (returns capped at 15% APR, stability/liquidity /100, debt-reduction boolean),
  combined by a `Weights` profile into a weight-normalized `Score` with a per-criterion `Breakdown`
  (explainable, no black box), and `Rank` sorting highest-first (stable). Tests cover normalization +
  capping, equal-weight averaging, zero-weight safety, returns ordering, and debt-priority weighting.

- Built the **Allocate screen** (replaced the stub): assembles candidates from non-archived asset
  accounts (return/stability/liquidity) and interest-bearing liabilities ("Pay down …", guaranteed
  return), ranks them with one of four preset profiles, and renders a score bar + per-criterion
  breakdown per suggestion. Amount input + constraints (emergency buffer, max-per-destination) later.

- Started the **formula engine** `internal/formula` with the **tokenizer**: numbers (including
  leading-dot), identifiers, double-quoted strings, `+ - * / %` and `== != <= >= < >` operators,
  parens, commas; EOF sentinel; errors on unterminated strings, a lone `=`/`!` (no assignment), and
  unexpected characters. Table-driven tests cover arithmetic, calls, comparisons+strings, and errors.

- Added the formula **parser** (`Parse` → AST): recursive descent with a precedence ladder
  (comparison < additive < multiplicative < unary < primary), left-associative binaries, parens, and
  function calls (incl. empty/nested args). AST nodes are NumberLit/StringLit/Ident/Unary/Binary/Call.
  Tested via a canonical s-expr renderer covering precedence, calls, and malformed-input errors.

- Completed the formula engine with the **evaluator** (`Eval`): `Value` is only float64/string/bool
  (no host references); arithmetic + comparisons (numeric, plus string equality), variables resolved
  from `Env.Vars`, and the allow-list functions `sum/avg/min/max/count/abs/round/if` (variadic where
  apt, arity-checked otherwise; `if` truthiness over bool/number/string). Errors on unknown
  var/function, division/modulo by zero, and type mismatch. Tests: arithmetic, comparisons, every
  function, variable formulas (a savings-rate expression), and the error cases.

- Surfaced the engine in the **Customize screen** (replaced the stub): a live formula calculator over
  real figures — net worth/assets/liabilities + current-month income/expense (converted to major
  units by base-currency decimals) and account/transaction/member counts — with the result (or the
  engine's error message) updating per keystroke and an available-variables reference table.

- Added the **forecast engine** `internal/forecast` (pure, tested): `Project(start, recurring,
  oneTimes, months)` walks the horizon applying the recurring monthly net (`MonthlyNet`) plus any
  one-time events scheduled in each month, returning the end-of-month balance series; empty for a
  non-positive horizon. Tests cover recurring-only, a mid-horizon one-time, flat, and zero-horizon.

- Surfaced the forecast on **Planning**: a 12-month net-worth projection chart seeded from current
  `ledger.NetWorth` and this month's net cash flow (income − expense) as the recurring monthly figure,
  fed through `forecast.Project` into the `AreaChart` (toned red when the monthly net is negative),
  with a plain-English caption of the projected end value.

- Built the **Documents** screen (replaced the stub): paste-and-import transactions from CSV via a new
  `appstate.ImportTransactionsCSV` (wraps `store.TransactionsFromCSV`, best-effort: stores each valid
  row through the validated path, skips invalid, returns the count). Header-name column matching means
  any spreadsheet export works; AI PDF/receipt parsing is flagged as arriving with the OpenAI client.
  (Reminder logged: don't run native `go test` with `GOOS=js` still set — it silently "fails" fast.)

- Built the **AI codec** `internal/ai` (pure, tested): OpenAI chat request/response shapes plus
  `BuildRequest` (marshal a chat body), `ParseResponse` (assistant content, surfacing API errors and
  empty responses), and `ParseUsage` (token counts). Round-trip tests stand in for a mock transport;
  the browser `fetch` layer (sending with the user's key) is a thin js/wasm file added next.

- Wired AI end to end: a js `fetch` **transport** (`ai.SendChat`) that POSTs a chat request with the
  user's key and resolves the promise chain (`then(text) → ParseResponse`), releasing its `js.Func`s
  on both success and catch paths; and an **Insights** screen with "Explain my month" that builds a
  system+user prompt from live figures (`fmtMoney(net/income/expense)`), shows a Thinking…/error/
  result state, and prompts to add a key in Settings when absent. `Settings.OpenAIKey/OpenAIModel`
  already exist in the store; the codec stays pure/native-tested, the transport is js-only.

- Wired the **AI key/model to persist**: the global-settings key input (seeded from `Settings.OpenAIKey`)
  saves on each input and the model select (GPT-4o mini / GPT-4o) saves on change, both via
  `app.PutSettings`. Insights now has a real key to use. (In-memory store, so it survives the session;
  reload-persistence rides on the broader settings/storage work.)

- Added the pure **auto-categorization engine** `internal/rules`: a `Rule{Match, SetCategoryID,
  SetTags}` matched case-insensitively as a substring of payee+description, first-match-wins, with
  `Category`/`Tags`/`FirstMatch` helpers; empty matches never fire. Table-driven tested (case folding,
  ordering, tags, empty-match). The transaction entry/import flows can apply it to auto-fill category.

- Applied auto-categorization on **transaction entry** with zero new storage: built implicit
  `rules.Rule`s from the existing categories (Match = name → SetCategoryID = id) and, on each
  description keystroke, suggest a category via `rules.Category` only when none is chosen — so it
  helps without fighting the user. A real `Rule` store + management UI (custom patterns/tags) is later.

- Added **natural-language Q&A** to Insights ("Ask about your money"): a question box that sends the
  user's question plus a figures context (net worth / income / spending / active accounts) to the
  same `ai.SendChat`, sharing the loading/result/error states with "Explain my month". Shown only
  when a key is set. Richer data context (per-category, history) is a follow-up.

- Started **Phase 3 (PWA)** with the web manifest: `manifest.webmanifest` (name, standalone display,
  dark background/theme colors, scope/start_url, categories) linked from the host page along with
  `theme-color` and the apple-mobile meta tags, so CashFlux can be installed as a standalone app.
  Icons + a caching service worker (offline shell) are the follow-ups.

- Added a **service worker** (`sw.js`, registered best-effort on load): **network-first** so it never
  breaks the gwc live-reload (always fetches fresh, caches the response) yet serves the last good copy
  when offline; pre-caches the core shell (index/wasm_exec/main.wasm/manifest) on install, evicts old
  caches on activate, and only touches same-origin GETs so cross-origin OpenAI calls pass through.

- Added **GitHub Actions CI** (`.github/workflows/ci.yml`): on push/PR it sets up Go from `go.mod`,
  runs `go vet ./...`, `go test ./...`, and a `GOOS=js GOARCH=wasm` build. Verified locally that
  native `vet`/`test ./...` don't choke on the js-only view packages (Go skips build-constraint-
  excluded packages silently), so the workflow is green-by-construction. Activates once the repo is
  pushed to GitHub (the create-repo step is still pending — needs the owner's `gh` auth).

- Completed the transaction **filter set** with a member picker (combines with search/account/
  category/sort and the shared Clear). Date-range filter + persisting the last filter remain.

- Added a **From/To date-range** to the transaction filters (parsed via `dateutil.ParseDate`,
  inclusive bounds, ignored when blank/invalid), combining with the other filters and Clear.

- Added the **PWA install prompt**: capture `beforeinstallprompt`, reveal an "Install CashFlux"
  button (dark-themed, fixed bottom-right), call `prompt()` on click, and hide it after the choice or
  on `appinstalled`. With the manifest + service worker, the PWA install path is complete.

- Added a **"Repeat last"** transaction helper: finds the newest transaction and pre-fills the form
  (description, account, category, kind from the amount sign / transfer, abs amount formatted in the
  account currency), so logging a recurring purchase is one click + Add.

- Added a **"Net worth by member"** rollup to the Members screen via `ledger.NetByOwner` (each member
  plus a Group (shared) row, base currency, green/red toned), defaulting an absent owner to zero base.

- Added an **extra-payment scenario** to the debt-payoff calculator: an optional extra-monthly input
  runs `payoff.Project` a second time at `payment + extra` and reports the months saved and interest
  saved in plain English — the engine's first what-if surfaced.

- Added a **trim-spending what-if** to the net-worth forecast: an input re-runs `forecast.Project`
  with `monthlyNet + trim` and reports the improved 12-month figure and the difference. (Declared the
  `trimStr` state at the component top so the hook stays unconditional even though the forecast card
  is built inside `if app != nil`.)

- Added one-click **example chips** to the formula Customize screen (savings rate, spending ratio,
  gross assets, over-budget bool) that populate the input. Rendered as four explicit buttons (not a
  loop) so the inline `OnClick` hooks stay at stable positions.

- Added the **liability sub-form** to the Accounts add form: when the selected type is a liability
  (`AccountType.Class() == ClassLiability`), it reveals credit-limit / APR / min-payment / due-day /
  lender inputs (each a conditional `If(isLiab, …)` so hooks stay stable) and the add handler parses
  them onto the `Account`. This finally gives the Upcoming-bills widget and credit-utilization a real
  data source.

- Added **unfinished goals as allocation candidates** (stability 80 / liquidity 60, no return), so the
  Allocate ranking can suggest funding a goal alongside accounts and debts. ~100 features in this
  session: the candidate-C design, all Phase-1 core, deep Phase 2, and Phase-3 PWA all shipped.

- Added an **AI narrative to Allocate** ("Explain with AI"): builds a short prompt from the top-5
  ranked candidates + profile and runs `ai.SendChat`, reusing the loading/result/error pattern. Hooks
  (states + handler) are placed after the unconditional `ranked` so their order stays stable.

- Refreshed the **`CLAUDE.md` status** line (was stuck at "Phase 0 … Phase 1 not yet started") to
  reflect reality for future sessions: Phase 1 complete, Phase 2 engines+screens live, Phase 3 PWA +
  CI in, with the remaining work (sync, custom fields, vision AI, reload-persistent prefs) noted.

- Wired the global-settings **"+ Add member"** button (was a no-op) to close the flip panel (clear the
  settings target) and navigate to `/members`, via `router.UseNavigate` + the settings atom.

- Added the **allocation-attributes sub-form** for asset accounts (expected return APR, liquidity,
  stability), mirroring the liability sub-form (conditional `If(!isLiab, …)` inputs, parsed in the
  add handler's else branch). Now the Allocate engine scores asset candidates on real data, not zeros.

- Wrote a project **`README.md`** (the repo had none): tagline, feature highlights, stack, build/run
  commands, the pure-logic-vs-thin-wasm-UI architecture, and links to the other docs — the GitHub
  landing page for when the repo is pushed.

- Expanded the formula calculator's variable set with budget/goal/task counts, broadening what
  user formulas can reference.

**Next:** per-row duplicate, persist-last-filter, then more Phase 2 polish — as the loop continues.

## 2026-06-15 — Dashboard design direction chosen (candidate C)

- Paused screen porting to explore the dashboard visual design with the owner. Built 5 static
  HTML+Tailwind candidates in `design/` (served at `/design/candidate-*.html`); iterated heavily.
- **Selected candidate C**: flat neutral-dark palette, Fraunces serif headings + accounting figures
  (negatives in parentheses), a **bento grid** with one base cell unit and integer-scaling widgets,
  unified per-widget header (grip · title · gear), drag-to-reorder + edge resize handles, a
  gear→center+flip per-widget settings panel, a collapsible icon-only sidebar with a "My pages"
  (custom pages) section, a top-bar time-resolution control (Week/Month/Quarter + From/To), and a
  large global-settings flip panel off the household card.
- Decomposed the mockup into a granular component backlog in `TODOS.md` §1.7c, each item referencing
  `design/candidate-c.html`. Drag/resize/flip will need pointer/DnD via `syscall/js`/`interop`;
  computation stays in the tested logic packages and layout/settings persist to the store.

**Next:** resume porting — apply the candidate-C shell (sidebar + top bar + bento) and start with the
design tokens + app shell, then the widget shell and first widgets, per §1.7c.

## 2026-06-15 — Phase 1 begins: data model (money)

- Started executing the backlog at §1.1, SDLC bottom-up. First service: `internal/money` — a
  precise `Money{Amount int64, Currency string}` type (integer minor units, never float), with
  currency-checked `Add`/`Sub`/`Cmp`/`Neg`/`Abs`/`Sum`. Pure Go, no `syscall/js`; table-driven
  tests pass on native Go (`go test ./internal/money`).
- Renamed the master backlog to `TODOS.md` (project-wide tracking list).

- Added `internal/currency`: registry + manual `Rates` table + `Convert`/`ToBase` (cross-currency
  via base, mixed decimals, nearest-minor rounding). A rounding test surfaced a good lesson —
  `1.005` as float64 is `1.00499…`, so exact half-cents aren't representable; tests now use
  float-stable rounding cases and the conversion rounds to the nearest minor unit.
- Expanded `TODOS.md` to a granular per-entity/service/screen backlog (full spec coverage).

- Added `internal/id`: 128-bit hex IDs via crypto/rand, optional prefix, seedable source for
  deterministic tests. (Test helper lesson: a single-byte counter wraps at 256 and collides — the
  uniqueness test now uses real crypto/rand.)
- Running as a self-paced `/loop`: one feature per iteration, granular commit + CHANGELOG each, with
  a ~1-minute cooldown between features.

- Added `internal/dateutil`: canonical date parse/format, month/week/fiscal-month ranges,
  half-open `InRange`, and DST-safe `DaysBetween` (computed via UTC calendar dates).

- Added `internal/domain`: all core entity types with custom-field maps and JSON tags, plus
  validated enums (`Valid()`/`String()`/`All*`), `AccountType.Class()`/`IsLiability()`, and
  `Transaction.IsTransfer/IsIncome/IsExpense`. Scope uses individual|shared (shared == group-level,
  owner `GroupOwnerID`). Tests cover enum validity, class mapping, and transaction classification.

- Added `internal/ledger`: `Balance`, `RunningBalances`, `PeriodTotals` (income/expense, transfers
  excluded, base-converted), `NetWorth` (assets − liabilities, liabilities reported positive), and
  `NetByOwner` rollups. All cross-currency math routes through the `currency.Rates` base. Tests cover
  mixed currencies, transfers, archived accounts, and currency-mismatch errors.

- Added `internal/budgeting`: scope-aware `Spent` (individual budgets count only the owner member's
  expenses; shared/group budgets count everyone), `Evaluate`/`EvaluateAll` returning remaining,
  percent, and ok/near/over `State` (default near threshold 80%). Handles multi-currency and
  zero-limit edge cases. Tests cover scope, currency conversion, and all three states.

- Added `internal/goals`: `Remaining` (never negative), `Percent` (0..100 clamped), `IsComplete`,
  and `Project` (ceil-months estimate from an assumed monthly contribution; already-complete goals
  project to `from`; non-positive contribution yields no projection) via `Evaluate`. Tested.

- Added `internal/freshness`: default per-type staleness windows (debt-like balances 14d, checking
  30d, savings 45d, investment 60d), `Merge` for settings overrides, `IsStale` (archived/exempt/
  untracked never stale; never-confirmed = stale), `DaysSinceUpdate`, `StaleAccounts`. Recurring
  fixed bills are exempt by design (modeled as Recurring, not accounts; window 0 also exempts). Tested.

- Added `internal/validate`: `Validate{Member,Account,Category,Transaction,Budget,Goal,Task}`
  returning `Issues` (all problems at once, form-friendly). Covers required fields, enum validity,
  positive limits/targets, currency consistency, account class/type match, score/due-day ranges, and
  related-ref requirements. Tested. **§1.3 pure-logic services layer is complete** — 10 packages,
  all green on native `go test`.

## 2026-06-15 — Persistence: pure-Go SQLite (corrected course)

- Built the pure store core first: `store.Dataset` aggregate + `Settings` + schema-versioned JSON
  `Export`/`Import` with a lossless round-trip test.
- **I was wrong, and the owner was right.** I claimed pure-Go SQLite can't run in a browser tab. It
  can: `github.com/ncruces/go-sqlite3` (no cgo, SQLite via wazero) **compiles for `GOOS=js
  GOARCH=wasm`** and the full app wasm still builds. Lesson: test the claim, don't assume.
- Switched persistence from IndexedDB to an in-memory SQLite store (`store.SQLiteStore`): schema +
  `Load`/`Snapshot` for clean dataset ingress/egress. Native tests pass; the JSON Dataset stays the
  portable import/export + sync format. Single pinned connection so `:memory:` is shared.
- Clean architecture paid off: switching the storage engine touched zero logic packages.

- Added per-entity CRUD (`Put/Get/Delete/List`) and query helpers on the SQLite store. Equality
  filters use SQLite `json_extract` (confirmed working with ncruces); date-range filters in Go via
  `dateutil.InRange`. `Put` upserts via `ON CONFLICT`. Tests cover CRUD, missing-key, upsert, and
  all queries.

- Added `internal/money` `FormatMinor`/`ParseMinor` (plain decimal ↔ minor units, strict, validated,
  round-trip tested). Kept it currency-agnostic (takes `decimals`, not a currency) to avoid an import
  cycle with `internal/currency`. This unblocks human-readable CSV. Symbol/grouping is a UI concern.

- Added `internal/store` CSV: `TransactionsToCSV`/`TransactionsFromCSV`. Decimal amounts via
  `money.FormatMinor`/`ParseMinor` + `currency.Decimals`; import matches columns by header name
  (order-independent, extra columns ignored), generates ids when missing, reports errors per line.
  Round-trip is `reflect.DeepEqual`-stable.

- Added `Get/PutSettings`, atomic `Wipe`, and `SampleDataset` (a valid starter seed — checked by
  running `internal/validate` over every entity in tests). **§1.4 persistence is complete.**

**§1.3 + §1.4 done — 11 packages green** (money, currency, id, dateutil, domain, ledger, budgeting,
goals, freshness, validate, store).

- Added `internal/logging` (§1.5): a `log/slog` `Handler` (writes lines to an `io.Writer` + records
  into a bounded `Ring`), with level filtering and `With`/`WithGroup`. Kept pure — the wasm app will
  pass a console-backed writer. Ring eviction, attr capture, grouping, and filtering are tested.

- Added `internal/appstate` — the UI↔logic seam. Kept it **pure Go** (no syscall/js): it owns the
  in-memory SQLite store + slog logger, exposes typed read accessors and validated write-through
  (`Put*` run `internal/validate` first), and does JSON export/import. `Init` seeds sample data and
  sets a package `Default` the screens will read. Wired into `app.Run`; the wasm app still builds and
  appstate is native-tested. Logging goes to `os.Stderr`, which Go's wasm runtime routes to the
  browser console — so no platform code needed.

- Converted the **Accounts** screen from a stub to real data: reads `appstate.Default`, computes
  per-account balances and net worth via `internal/ledger`, groups assets vs liabilities, and shows
  a summary. Added shared display helpers (`fmtMoney`/`amountClass`/`humanizeType`). First visible
  end-to-end feature on the live view. (Read-only for now; add/edit + reactivity next.)
- **Note:** embedding SQLite (ncruces wasm) pushed the raw wasm to ~20 MB (was ~6.5 MB). It
  compresses well (gzip/brotli) but is a real first-load cost — track it; consider lazy-loading or
  the tinygo path later if needed.

- Wired the **Dashboard** to real data: net worth, this-month income/expense (`ledger.PeriodTotals`
  over `dateutil.MonthRange(time.Now())`), active-account count, and a sorted recent-activity list.
  `time.Now()` works at runtime in wasm. Two real screens now read the live store.

- Built the **Accounts add form** — the first mutating feature. Reactivity pattern that works with
  this framework: a screen-level `state.UseAtom("rev:accounts", 0)` subscribes the component; after a
  successful `appstate.PutAccount` the handler bumps the atom, re-rendering the screen against fresh
  store data. Form hooks (`UseState`/`UseEvent`) sit at stable top-level positions; option lists are
  built in plain loops (no `On*` there). Added the missing row/form/amount CSS to the host page.

- Added per-row account **delete**: converted `accountRow` (plain func) into an `AccountRow`
  component so its delete-handler `On*` hook is stable, and switched the lists to `MapKeyed` (keyed
  by account id). The parent passes a `func(string)` delete callback that calls
  `appstate.DeleteAccount` and bumps the revision. This is the canonical per-row pattern for the
  whole app. (Note: deleting an account currently leaves its transactions; cascade/cleanup later.)

- Built the **Transactions** screen: add form (description, amount, income/expense, account,
  category, date) where the amount's currency follows the chosen account and expenses are stored
  negative; newest-first list; per-row delete via `TransactionRow`. Member is inferred from the
  account's owner for individual accounts. Same reactive-revision pattern.

- Built the **Budgets** screen on `internal/budgeting`: current-month spend vs limit per budget with
  a colored ok/near/over progress bar (`Attr("style", "width:N%")` for the fill), plus add and
  delete. Limit is recovered for display as `Spent + Remaining` (both base currency).

- Built the **Goals** screen on `internal/goals`: progress bar (% + remaining), optional target date,
  add and delete. Projection is deferred until we capture an assumed monthly contribution. Reused the
  budget/bar CSS. (Imported the goals package as `goalsvc` to avoid shadowing the local `goals` var.)

**Next:** the **To-do** screen (task list + add + complete + delete), then transfers, then the
remaining Phase-1 screens (Members, Categories, Settings with import/export + load-sample/wipe).

## 2026-06-15 — Project kickoff & spec

- **Toolchain (fresh Windows machine):** installed GitHub CLI, portable Git, and Go 1.26.4 into
  `%LOCALAPPDATA%\Programs` and added them to the user PATH (no admin; MSI installs were blocked).
- **Repo:** created `CashFlux`, initialized git on `main`. Name chosen with the owner.
- **Framework study:** analyzed the local `GoWebComponents` checkout — confirmed the public API
  (shorthand element + control-flow funcs, `ui` hooks, `state` atoms, history `router`), the
  module wiring needed for a standalone app (local `replace` + mirrored `agenthub`/GoGRPCBridge
  replaces), and a key gotcha: `On*` prop options register hooks on wasm, so per-row handlers must
  live in their own row components.
- **Spec:** iterated with the owner and locked Phase 1. Highlights: local-first, household/group
  aware (members, individual pools, group budgets), full asset+liability accounts (incl. informal
  "loan shark" debts), multi-currency with a manual FX table, freshness nudges, custom fields +
  formula builder, planning + to-do, OpenAI client-side (BYO key) for document parsing/insights,
  and a capital-allocation suggestion engine.
- **Standards:** wrote `CLAUDE.md` — pure idiomatic Go, clean architecture (logic packages with no
  `syscall/js`, unit-tested on native Go), `log/slog` logging, readable plain-English UI,
  import/export, heavy configurability, and strict VCS/journaling (one feature per commit).

- **Dependency cleanup:** replaced the local `../GoWebComponents` `replace` with a real `go get`
  module pin (pseudo-version `v1.1.1-0.20260613162601-cad8af8`). `go mod tidy` + wasm build are
  clean — `agenthub` is pruned (core packages don't import it); only `cbor`/`float16`/`goldmark`
  come along indirect. Phase 0 wasm entrypoint builds (6.17 MB).
- **Tooling:** built `gwc` from the framework checkout and wired it as `.tools/gwc.exe` + the `gwc`
  MCP server (`.mcp.json`, 81 `gwc_*` tools). Wrote `docs/GOWEBCOMPONENTS.md` and a CLAUDE.md
  quick-reference for new sessions. Moved pre-spec draft files to `_scratch/` (Go-ignored).

- **Skeleton:** built the routed app shell (`internal/app`: router + `Shell` + `NavBar`) and stub
  screens for all 12 features (`internal/screens`), driven by a single screen registry. Verified on
  the live `gwc dev` server (HTTP 200 for `/`, wasm, and glue; hot reload active).
- **Layout cleanup:** moved web/build assets under `web/` so the project root holds only Go source,
  config, and docs — clean and standard.
- **Framework bug found (parked):** `gwc dev` resolves `-html` relative to the build/module root,
  not the serve `-root` (contradicts its flag help). Workaround: pass `-html web\index.html`. Proper
  fix is in GoWebComponents `tools/gwc/dev.go` — to be done, then rebuild + recopy `gwc`.
- **Planning:** wrote `TODO.md`, the priority-ordered master backlog, and made bottom-up SDLC
  (model → services → store → UI) an explicit rule in `CLAUDE.md`.

**Next (per SDLC + TODO §1.1):** start the data model — `internal/domain` types + `internal/money`
and `internal/currency` services with table-driven tests — before any feature UI.

**Note:** a few pre-spec exploratory Go files (model/persist/dashboard/transactions/components)
remain in the tree from early prototyping; they predate the locked spec and will be replaced to
match it during Phase 1.

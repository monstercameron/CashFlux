# CashFlux — Developer Journal

Narrative companion to `CHANGELOG.md`. Newest entries first. Capture decisions, trade-offs,
problems and fixes, and what's next.

## 2026-06-16 — i18n: AccountRow (Accounts screen complete)

- Migrated the `AccountRow` component: inline edit form (reusing the accounts.* field keys + common.name/
  owner + action.save/cancel), the update-balance prompt (`%s (%s)` via T args), the stale badge, the
  cleared-balance suffix, and all row action buttons + titles (view→nav.transactions, update balance,
  mark updated, edit, archive/restore, delete). Accounts is now fully localized.
- Left `humanizeType` (a generic enum title-caser) as-is — localizing account-type display names is a
  separate domain-enum task. `i18n` tests + wasm green.
- **Last remaining UI verbiage: the Transactions screen** (the other giant).

## 2026-06-16 — i18n: Accounts screen (main function)

- Migrated the `Accounts()` function (the big add form with all asset/liability sub-fields, the welcome
  card, net-worth/assets/liabilities stats, mark-all-updated notice + button, section headers + empty
  states, the balance-adjustment txn desc, and the invalid-balance error) onto `uistate.T`. Reused
  `common.name`, `owner.group`, `dashboard.netWorth/liabilities`.
- Account *type* labels still come from the `humanizeType` helper (shared, used elsewhere) — left for a
  helper-level pass. The `AccountRow` component (inline edit, per-row actions) is the next chunk.
- `i18n` tests + wasm green. Split Accounts across two cycles to bound the diff.

## 2026-06-16 — i18n: settings panel right column (panel complete)

- Migrated the right column of `app/settings.go`'s global panel: AI section (title/enable/key
  placeholder), Appearance (theme seg Dark/Light/System, Accent, Compact), Preferences (week-start
  seg Sunday/Monday), Date format title, Data action buttons, and the Languages buttons — all via
  `uistate.T`. Currency/model display names and the date-format example option text stay literal.
- The global settings panel is now fully localized. `i18n` tests + wasm green.
- Remaining UI verbiage: the two large screens **Accounts** and **Transactions**.

## 2026-06-16 — i18n: settings panel (left column + chrome)

- Migrated the first half of `app/settings.go`'s global panel: SettingsHost panel title, the
  widgetSettingsForm no-settings placeholder, and the left column (household members, base currency,
  exchange rates, screens + hint, freshness + hint). Converted the `freshnessTypes` table from
  hardcoded `Label` to an i18n `Key` (like cfEntities), resolved at render.
- Kept the currency option display names ("USD — US Dollar") literal (registry territory). Split the
  panel across two cycles to keep edits bounded — the right column (AI/appearance/prefs/data/languages)
  is next.
- `i18n` tests + wasm green.

## 2026-06-16 — i18n: custom-fields components migrated

- Migrated the shared custom-fields UI onto i18n: `customfields.go` (CustomFieldsManager) and
  `customfieldform.go` (CustomFieldInput). Converted the package-level `cfEntities`/`cfTypes` tables
  from hardcoded `Label` to an i18n `Key` resolved at render (entities reuse `nav.*`; types get
  `cf.type*`), so `cfTypeLabel` and the section headers localize too. Added the form/list strings,
  the required suffix/label, and Yes/No. Added uistate imports to both files.
- `i18n` tests + wasm green. Remaining UI verbiage: the big `app/settings.go` global panel, and the
  two large screens Accounts + Transactions.

## 2026-06-16 — i18n: Settings screen migrated

- Twelfth screen onto i18n: `screens/settings.go` — household summary (reusing `nav.members/accounts/
  categories` for the row labels) + the debug-log viewer (title, refresh, empty state). Added uistate
  import; `fmt` stays for the count values.
- `i18n` tests + wasm green. Remaining UI verbiage: the big `app/settings.go` global panel, the
  Accounts + Transactions screens, and the CustomFieldsManager/CustomFieldInput components.

## 2026-06-16 — i18n: Dashboard chrome migrated

- Eleventh screen onto i18n: `dashboard.go` — every widget title (reusing `nav.*` for Accounts/
  Budgets/Goals/To-do), the header cell (title/hint/Reset), the freshness widget (all-fresh, stale
  count via T args, Remind), the savings sub-line + "this period", and the KPI assets/accounts
  sublines. The nudge task title is localized too.
- Left a couple of dynamic KPI sublines (`periodLabel + plural(...)`) literal — they concat a
  date-label with the English `plural()` helper, so cleanly localizing them is its own task (plural
  rules). The cashflow bar heights and the freshness "· %dd" chip stay `fmt` (numeric). `fmt` remains.
- `i18n` tests + wasm green. 11 screens + chrome. Remaining: Accounts, Transactions (the giants),
  Settings, and the CustomFieldsManager/CustomFieldInput components.

## 2026-06-16 — i18n: Allocate screen migrated

- Tenth screen onto i18n: `allocate.go` — profile picker + amount/reserve inputs, ranked rows (the
  breakdown via `T` args, exclude/restore), candidate name prefixes ("Pay down %s", "Goal · %s"),
  empty states, kept-back note, and the AI-explanation card. Kept the numeric `%.0f%%` score
  formatting and the AI prompt builder (`fmt.Fprintf`) literal, so `fmt` stays.
- `i18n` tests + wasm green. 10 screens + chrome. Remaining: Accounts, Transactions, Dashboard,
  Settings (+ CustomFieldsManager/CustomFieldInput).

## 2026-06-16 — i18n: Documents screen migrated

- Ninth screen onto i18n: `documents.go` — vision-import + CSV-import cards, the review/edit draft
  list, and all status/error messages (several via `T` args with `plural(...)`). Kept the vision
  *system prompt* (model instruction) and the CSV-format example placeholder literal.
- All `fmt.Sprintf`s became `T(key, args…)`, so the `fmt` import was dropped. `i18n` tests + wasm green.
- 9 screens + chrome. Remaining: Accounts, Transactions, Dashboard, Allocate, Settings (+ the
  CustomFieldsManager/CustomFieldInput shared components).

## 2026-06-16 — i18n: Customize screen migrated

- Eighth screen onto i18n: `customize.go` — formula-calculator title/desc, expression placeholder, the
  example chips ("Savings rate %", etc. — `%` is literal since T only Sprintf's with args), and the
  Result/Available-variables sections. The formula example *expressions* set on click stay literal
  (they're code), as do the `true`/`false` result values. Added uistate import; `fmt` stays for the
  `%v` value fallback.
- `CustomFieldsManager` (rendered here, defined elsewhere) still has its own strings — a later screen.
- `i18n` tests + wasm green. 8 screens + chrome. Remaining: Accounts, Transactions, Dashboard,
  Documents, Allocate, Settings (+ CustomFieldsManager + CustomFieldInput components).

## 2026-06-16 — i18n: Planning screen migrated

- Seventh screen onto i18n: `planning.go` — debt-payoff calculator (inputs, stat labels, hint/invalid/
  too-low messages, the extra-payment note via T args), and the 12-month forecast card (title, the
  cash-flow projection hint, trim-spending placeholder + note) all via `uistate.T`. Added uistate import.
- The `%d` months stat value stays as `fmt.Sprintf` (a number, not text), so `fmt` remains. `i18n`
  tests + wasm green.
- 7 screens + chrome migrated. Remaining: Accounts, Transactions (the giants), Dashboard, Documents,
  Allocate, Customize, Settings.

## 2026-06-16 — i18n: Insights screen migrated

- Sixth screen onto i18n: `insights.go` UI strings (explain/ask cards, the key hint, placeholders,
  answer + save-as-task, status messages). Added the missing `uistate` import.
- **Decision:** left the OpenAI system/user prompt strings in English — they're instructions sent to
  the model, not user-facing text, so they shouldn't be translated (and the `fmt.Sprintf` prompt
  builders stay). Only the visible chrome is localized.
- Sequencing: did Insights (moderate) rather than the 330-line Accounts to keep the cycle's token cost
  reasonable; Accounts/Transactions (the two giants) remain, plus Dashboard/Documents/Allocate/
  Planning/Customize/Settings. `i18n` tests + wasm green.

## 2026-06-16 — i18n: Budgets screen migrated (+ shared owner picker)

- Fifth screen onto i18n: `budgets.go` — add form (incl. `Limit (%s)` via T args), period picker, month
  stepper titles, spent/budgeted/left stats, the over/near summary, and budget rows (on-track/near/over
  labels, the `%s · %s · %d%% · %s left` sub via T args, edit/delete).
- Also localized the **shared** `ownerSelectOptions` helper ("Group (shared)" → `owner.group`), which
  the budgets add-form now uses (replaced its inline duplicate) and which goals' edit row also calls —
  so two screens' owner pickers are covered at once. Added shared `common.owner`.
- Period option labels still come from `domain.Period.Label()` (enum-level), left as-is. `fmt` stays
  for the CSS bar width. `i18n` tests + wasm green.
- **Next:** Accounts (the largest) or Transactions screen.

## 2026-06-16 — i18n: Goals screen migrated

- Fourth screen onto i18n: `goals.go` — add form (incl. `Target (%s)` placeholder via T args), owner +
  linked-account pickers, combined-progress stats, and the progress sub-line assembled from T
  fragments (`progressFmt`, `complete`, `bySuffix`, `saveSuffix`, `linkedSuffix`) plus the contribute
  prompt and row actions. Reused `common.name`, `owner.group`, `nav.goals`, `action.*`.
- The CSS bar-width `fmt.Sprintf` and the `%d%%` stat value stay as fmt (not user-facing text), so the
  fmt import remains. `i18n` tests + wasm green.
- **Next:** Budgets or Accounts screen.

## 2026-06-16 — i18n: Categories screen migrated

- Third screen onto i18n: `categories.go` fully migrated — add form, kind + parent pickers (incl. the
  inline-edit ones), reassign-before-delete panel (templated description via `T` args), income/expense
  list cards + empty states, and row edit/delete + the kind meta (replaced the `humanizeType` call with
  `category.income`/`category.expense`).
- Introduced shared keys to curb sprawl: `common.name`, `common.reassignTitle`, `common.moveAndDelete`,
  and `category.expense`/`category.income`. (Members still has its own reassign/name keys — a tiny
  later convergence; not worth churn now.)
- Dropped the unused `fmt` import; added the missing `uistate` import. `i18n` tests + wasm green.
- **Next:** Goals or Budgets screen. 5 screens migrated after this (shell, todo, members, categories;
  + chrome).

## 2026-06-16 — i18n: Members screen migrated

- Second screen onto i18n: `members.go` fully migrated — add form, the reassign-before-delete panel
  (incl. the `%q owns %d…` description via `T(key, args…)`), member rows (make-default, view
  transactions, edit, delete, default badge, role meta), net-worth-by-member, and validation messages.
- Reused shared keys: `common.notReady`, `action.save/cancel/edit`, `nav.transactions` (row button),
  and added `owner.group` ("Group (shared)") which the owner pickers elsewhere can adopt next.
- Dropped the now-unused `fmt` import (the only `fmt.Sprintf` became a `T(...)` call).
- `i18n` tests + wasm green. **Next:** Categories or Goals screen.

## 2026-06-16 — i18n: To-do screen migrated (first full screen)

- First full screen onto the language store: every user-facing string in `todo.go` now resolves via
  `uistate.T` — add-form title/placeholders, priority options (both the add form and inline edit),
  empty/all-done states, the hide-done toggle, the validation message, and the row actions
  (toggle/edit/delete titles, due prefix). `priorityMeta` returns translated labels.
- Added shared keys (`priority.high/medium/low`, `common.notReady`) so the other screens reuse them
  rather than re-adding. Catalog grew accordingly; English values match the old literals → no visible
  change.
- `i18n` tests + wasm green. **Next:** migrate the next screen (Members or Categories), reusing the
  shared keys; track coverage as the catalog grows.

## 2026-06-16 — Extract credit-utilization into ledger

- Moved the inline `owed*100/limit` from `accountMeta` into pure `ledger.Utilization(balance, limit)`
  (balance magnitude, ok=false for non-positive limit). Accounts liability rows delegate to it.
- Table-tested: no/negative limit not-ok, negative & positive balance magnitudes, zero owed, over-limit.
  `internal/ledger` + wasm green.
- Continues the "no math in view code" + trust theme; the ledger package now owns SavingsRate,
  PercentChange, and Utilization alongside the balance/net-worth functions.

## 2026-06-16 — Top-bar verbiage migrated to i18n (chrome complete)

- Migrated the top bar's chrome onto `uistate.T`: menu-toggle tooltip (`topbar.menu`), "+ Add" label
  (`topbar.addLabel`) and its tooltip (`topbar.add`). With the sidebar already done, the app shell's
  verbiage is fully on the language store.
- The page title itself still comes from `screens.All()` data (per-route Title/Subtitle); i18n-ing
  those needs per-route title keys — left for the screen-verbiage migration pass.
- `i18n` tests + wasm green. **Next:** begin per-screen verbiage migration, or pivot to another area.

## 2026-06-16 — Language bundle export/import in Settings

- Made the language store's round-trip user-accessible (the user's "easy to export/import all langs"
  ask): `uistate.ExportLanguages()`/`ImportLanguages()` wrap the bundle's JSON codec; import merges and
  persists to localStorage (`cashflux:languages`), and `loadBundle()` seeds those on boot. Added
  Settings → Languages "Export languages" / "Import languages" buttons, wired through the toast.
- Note: with T still non-reactive (English-only display), an imported language is stored/ready but not
  shown until the language selector lands — the notice says "reload to apply" and the selector is the
  remaining §1.19 piece. Export gives translators the English source + any existing langs.
- wasm build green; pure `i18n` codec already tested. **Next:** the language selector (then imported
  languages actually display), or continue migrating screen verbiage.

## 2026-06-16 — i18n live wiring + sidebar verbiage migrated

- Wired the language store into the UI: `uistate` holds a shared `i18n.DefaultBundle()`, a `UseLang`
  atom (default English), and a hook-free `T(key, args…)` helper (resolves against the bundle default).
  T deliberately takes no hook so it's safe in loops / row components (the nav maps over items).
- Migrated the first screen's chrome onto it: shell brand, primary + System nav labels, the
  "My pages"/"System"/"New page" headers, and the household card now call `uistate.T(...)`. All keys
  already existed in the English catalog and match the old strings, so zero visible change.
- **Design note:** kept T non-reactive for now (English-only). When a language selector lands (TODOS
  §1.19), the active lang gets threaded at render edges (read `UseLang` at a component top) rather than
  inside T — preserving the no-hooks-in-loops rule.
- wasm build green. **Next:** migrate more screens' verbiage onto T incrementally, or the next feature.

## 2026-06-16 — Extract bill next-due date into dateutil

- Moved `nextDue` out of the js-only dashboard into pure `dateutil.NextMonthlyDue(now, day)` — the
  next occurrence of a monthly due-day on/after today, day clamped to 1–28 so it's valid every month
  (incl. February). The upcoming-bills widget calls it.
- Table-tested the fiddly cases: later-this-month, on-the-day, already-passed→next-month, >28 clamp,
  February clamp, non-positive→1. `internal/dateutil` + wasm green.
- (Fixed a wrong expected value while writing the test — day=0 clamps to the 1st, which is already
  past the 10th, so it rolls to next month.)

## 2026-06-16 — Extract savings-rate calc into ledger

- Moved the dashboard's inline `(income-expense)*100/income` into pure `ledger.SavingsRate` (0 when
  income ≤ 0, negative when overspent, truncates toward zero). Same "no math in view code" + trust
  pattern as the earlier `PercentChange` extraction; the savings widget now calls it.
- Table-tested (no-income, negative-income, saved/overspent, spent-nothing, truncation). `internal/ledger`
  + wasm green.
- (Minor: the in-place edit briefly scrambled the SavingsRate/PercentChange doc comments; fixed.)

## 2026-06-16 — Default category scheme (§1.10, pure)

- Added `internal/catscheme.Default()` — a starter set of income/expense categories plus a few
  sub-categories (Housing → Rent/Utilities, Transportation → Fuel/Public transit). Returns ID-less
  `Item`s with `Parent` named so the persistence layer assigns IDs and resolves parents.
- Table-tested: both kinds present, unique names, parents resolve to a same-kind top-level item, hex
  colors. `internal/catscheme` green.
- Bottom-up piece of §1.10 "Default scheme + reset": the pure scheme now exists; the "reset
  categories" action (apply it via appstate, replacing/merging) and methodology presets (envelope/
  zero-based) remain.

## 2026-06-16 — Backlog: Electron desktop app as a post-core item (§5.1)

- Added a new `TODOS.md` §5 "Future / nice-to-have (post-core)" tier and logged **standalone
  desktop app via Electron** as its first item. Placed at the very bottom of the priority-ordered
  file, after Phase 3 / sync and the continuous Cross-cutting section, to mark it explicitly as
  lower-priority and post-core — not part of the spec.
- Scoped it as a thin wrapper that reuses the *exact* production `web/` build (wasm bundle +
  `wasm_exec.js` + `sw.js` + manifest) as the renderer, so there's no second UI codebase and the
  wasm stays the single source of truth. Sub-tasks cover wrapper choice (Electron vs. Tauri/Wails —
  to be decided), scaffold, window chrome, per-OS packaging, a CI artifact job, and verification.
- Docs-only change; no code or tests affected.

## 2026-06-16 — Currency display helper (§1.2)

- Closed the §1.2 "format a Money in a target/base currency" checklist item: added
  `Rates.FormatAccounting(m, toCurrency)` and `Rates.FormatInBase(m)` to `internal/currency` —
  convert via the rate table, then `money.FormatAccounting` with the target's symbol/decimals.
  Lives in currency (which already imports money) so the money package stays registry-free.
- Table-tested: same-currency, negative-parenthesized, cross-currency conversion (€→$ and $→€),
  missing-rate error. `internal/currency` green.
- Small decision-free increment between the bigger parked items; screens can adopt it for multi-
  currency display.

## 2026-06-16 — B10 foundation: pure period presets

- Shifted off the widget-settings sweep to the B10 resolution-control redesign, starting with the
  decision-free pure layer: `period.Previous`, `period.YearToDate`, plus `Window.Shift` (move the
  whole window as a unit — distinct from the edge-only StepFrom/StepTo) and `Window.IsCurrent` (flag
  when the view has paged off "now"). All take an explicit `now` so they're pure; table-tested.
- Dropped `LastNDays` from the plan: the Window model is unit-based (week/month/quarter), so an
  arbitrary N-day range doesn't fit it cleanly — noted for the UI step (those would need a different
  representation). ThisPeriod stays `NewWindow`.
- `internal/period` green + wasm build green. The UI redesign (single stepper + presets dropdown)
  still carries the keep-range-vs-drop decision, so it stays parked until the user calls it; these
  constructors are ready for whichever way it goes.

## 2026-06-16 — B12: Spending breakdown top-N setting

- Fourth widget on the settings API: "breakdown" schema (`topN`, 2–6, default 3); the widget reads it
  and groups the rest as "Other" (generalized the hardcoded top-3). Tones cycle so >4 segments are fine.
- widgetcfg tests + wasm green. Dashboard widgets with persisted settings now: savings, recent, trend,
  breakdown. **Next:** likely wrap up the widget-settings sweep and pick another backlog item.

## 2026-06-16 — B12: Net worth trend months setting

- Third widget on the settings API: "trend" schema (`months`, 3–12, default 6); `netWorthTrendWidget`
  reads it and generalizes the cutoff window (offset `i-(months-2)`, preserving the prior 6-month
  shape that ends one month ahead). Persisted via `WidgetConfigs`.
- widgetcfg tests + wasm green. **Next:** spending-breakdown top-N, or move on to another backlog item.

## 2026-06-16 — B12: Recent transactions row-count setting

- Second widget on the settings API: registered a "recent" schema (`count`, number 3–20, default 6)
  and had `recentWidget` read it (`widgetCfgs.For("recent")`) to size the list instead of the
  hardcoded 6. Same pattern as savings — schema in `widgetcfg`, consumption in the screen, persisted
  via the `WidgetConfigs` atom.
- widgetcfg tests + wasm build green.
- **Next:** keep extending — net-worth-trend range (months), spending-breakdown top-N, etc.

## 2026-06-16 — B12: savings widget consumes its settings

- Closed the loop on the per-widget settings example: `savingsRateWidget` now reads its persisted
  config (target rate %, show-bar). Dashboard reads `uistate.UseWidgetConfigs()` at the top (stable
  hook position) and passes `widgetCfgs.For("savings")` down — cleaner than a hook inside the helper.
- Behavior: tone now reflects performance vs target — at/above target green, positive-but-short amber
  (`text-warn`), negative red; the subline shows the target; the bar hides when `showBar` is off.
- End-to-end demoable: gear → set target/hide bar → persists across reload → widget reflects it.
- wasm build green. **Next:** register feasible schemas for more widgets (recent count, trend range,
  budgets scope) so "any feasible settings exposed and persisted" extends across the dashboard.

## 2026-06-16 — B12 wiring: schema-driven, persisted widget settings panel

- Loop back in implement mode; resumed B12 (the widget-settings wiring paused earlier). Picked it as
  the highest-value self-contained item with no blocking decision and no external deps.
- Re-added `uistate.WidgetConfigs` (localStorage-backed atom; `For` + copy-on-write `WithField`), and
  rewrote `app.widgetSettingsForm` to be schema-driven: it looks up `widgetcfg.SchemaFor(id)` (ID now
  threaded from `SettingsHost`) and renders each field via a dedicated `widgetFieldRow` component
  (toggle→ToggleRow, number→numeric input, select→Select) bound to the persisted config; placeholder
  for widgets without a schema. The old fake title/behavior toggles are gone.
- `widgetFieldRow` is its own component so each input's hook stays at a stable position (On*-in-loops
  rule); the row's branch is fixed per field type, so hook order is stable.
- Savings rate now shows real, persisted settings (target rate %, show-bar). **Next (next cooldown):**
  the savings widget *consumes* those values (compare actual vs target, optional bar), then register
  schemas for more widgets.
- wasm build green; native suite clean; `widgetcfg` unit tests already cover the accessors.

## 2026-06-16 — Pages deploy workflow (built); E2E test stories logged (B16)

- User: build a CI workflow that redeploys the build on every push so they can review from anywhere,
  and (separately) log an extensive E2E testing program.
- **Built `deploy-pages.yml`:** on push to main, set up Go, build `web/bin/main.wasm` (GOOS=js), copy
  `wasm_exec.js` from GOROOT (`lib/wasm` with `misc/wasm` fallback), generate `404.html` from
  index.html for deep-link routing, upload `web/` as a Pages artifact, deploy via `deploy-pages@v4`.
  - **Chose Actions-deploy over committing a `/docs` folder.** Same live URL
    (monstercameron.github.io/CashFlux), but no build artifacts committed and no push/commit loop.
    Updated the §0 hosting item to reflect this; the only manual step is setting Pages Source =
    "GitHub Actions" once (will try via `gh api`).
- **B16 — E2E stories:** logged a trustworthy-app testing program — dozens of scripted user-journey
  stories (canonical: add a transaction) asserting both UX (smooth standard path) and correctness
  (data/state/derived figures), covering every feature + cross-cutting (reload/offline/routing/a11y).
  Needs the Playwright/Chromium browser lane (§0, not installed) to run; authored/queued until then.
- Planning for B16; the deploy workflow is the one build action this turn (explicitly requested).

## 2026-06-16 — App-wide accessibility spike + program (B15)

- User: think deeply about app-wide accessibility and log it as a spike (it's extensive). Added B15.
- Framed it as a **spike first** (axe/keyboard/SR audit → catalogue framework a11y primitives → decide
  reusable patterns → output a prioritized plan), then a deep area checklist that becomes the tasks:
  semantics/landmarks, keyboard (flagging the pointer-only bento drag/resize as a real gap needing a
  keyboard alternative), dialog focus-trap for FlipPanel, correct ARIA for the custom controls
  (Segmented/ToggleRow/icon-buttons that rely on `title` today), focus-visible, live regions, color-
  not-the-only-cue, AA contrast (text-faint/accent suspect), reduced-motion, 200% zoom/reflow (ties to
  B6's px-heavy concern), forms, SPA route focus/title (ties B3/B9), chart alt-text (ties B14), touch
  targets, i18n'd labels, and CI axe once Playwright is in (§0).
- Pointed the §1.20 one-line a11y item at B15 (subsumed). Planning-only.

## 2026-06-16 — Designed Lucide (B13) + D3 (B14) integrations (planning)

- User: integrate Lucide (glyphs) and D3 (charts) with strong Go interfaces. Per planning mode, logged
  the designs as B13/B14 rather than building.
- **B13 Lucide:** the existing `ui.Icon(name string)` already emits 24×24 stroked currentColor SVGs —
  Lucide's exact format — so it's a clean swap. Proposed a type-safe `internal/icon` (`Name` constants
  → embedded Lucide path data, compile-checked), with a build-time generator to pull the curated set.
  Flagged embed-at-build (recommended, offline/vdom-safe) vs. CDN createIcons (fragile).
- **B14 D3:** proposed a pure, typed `chartspec.Spec` (Kind/Series/Axis + Validate + extent helpers)
  with `ui.Chart(spec)` driving D3 through a ref/portal + UseEffect (since D3 mutates the DOM and the
  framework owns a vdom), D3 pinned via CDN and SW-cached for offline. Flagged the real decision: adopt
  the heavy D3 dep vs. keep growing the working pure-Go SVG charts — needs the user's call before build.
- Both interfaces sketched in TODOS so the "strong Go interfaces" ask is answered at design level.

## 2026-06-16 — Switch pushes to gh; pause widget-settings wiring → B12

- User: use the `gh` CLI for pushing (more reliable), and go back to TODO-planning mode.
- **Push fix:** background `git push` was failing with "could not read Username for github.com" — the
  Windows/bare-git credential prompt pops a GUI dialog that can't complete in a non-interactive shell.
  Ran `gh auth setup-git` (gh is logged in as monstercameron, repo scope) so git uses gh's token
  non-interactively; pushes now succeed. Updated the CLAUDE.md push rule accordingly.
- **Back to planning:** the per-widget settings API foundation (`internal/widgetcfg`) is committed and
  tested, but I'd started the UI wiring (a `uistate.WidgetConfigs` atom). Per the user's "just todo
  planning", removed that uncommitted file and logged the remaining wiring as **B12** (persisted atom +
  schema-driven `widgetSettingsForm` + savings consumption + more widget schemas). The committed pure
  package stays — it's a standalone tested foundation with no consumers yet, which is fine.

## 2026-06-16 — Per-widget settings API (widgetcfg) — step 1

- User wants the per-widget flip panel wired to each widget's own settings (savings rate → savings
  settings), persisted. Building it bottom-up.
- Step 1: pure `internal/widgetcfg` — `Field` (toggle/number/select + default/bounds/options),
  `Schema` per widget, `Config` (key→string values) with typed accessors (`Str`/`Bool`/`Int` with
  default fallback + clamp + select validation), and a registry (`SchemaFor`/`Has`/`IDs`). Savings
  rate registers the first schema (target rate %, show-bar toggle). Table-tested.
- Next: a persisted `uistate` widget-configs atom + a schema-driven `widgetSettingsForm`, then the
  savings widget consuming its target setting.

## 2026-06-16 — README + Pages-hosting TODOs; background pushes

- Logged two §0 items: a proper **README.md**, and **hosting the app on GitHub Pages from `/docs`**
  (production build committed to `docs/`, relative paths for the `/CashFlux/` subpath, a `404.html`
  shell for deep-link routing — the static-host side of B1 — and a build script so `/docs` is
  regenerated, not hand-copied). Ticked the now-stale "create repo + push" §0 item.
- Per user: run `git push` in the **background** (fire-and-forget) so a credential/elevation prompt or
  UI button can't hang the non-interactive shell. Updated the CLAUDE.md push rule accordingly.

## 2026-06-16 — Logged "+ Add" flip-panel of add actions (B11)

- User wants "+ Add" to open a centered flip panel (settings-style) with add options: transaction,
  bills to scan, docs to scan, custom workflows, etc. Logged B11.
- Reuse note: the flip animation already exists as `ui.FlipPanel` via the `UseSettings` atom +
  `SettingsHost`; add an "add" target kind and an `addMenu` back face rather than a parallel overlay.
- Flagged the one scope question: what "custom workflows" maps to (existing Customize/formula vs. a
  new concept). Analysis/TODO only.

## 2026-06-16 — Deep-analyzed the time-resolution control (B10)

- User asked for a deep analysis to drastically improve the resolution-control UX. Logged B10.
- Core finding: the dual From/To stepper makes a *range* the default when ~90% of users want a single
  period — and it reads "Jun 2026 – Jun 2026" (looks broken) in that common case. Also no presets, no
  "back to now" reset, no off-current indicator, and it's wide (will crowd +Add and the B9 breadcrumb).
- Proposed: a single period stepper as the primary, a presets dropdown (This/Last month, This quarter,
  YTD, Last 30 days, Custom range…), a "this period"/Today reset, and the existing From/To range tucked
  behind "Custom range". Bottom-up: add pure `period` preset constructors (`ThisPeriod`/`Previous`/
  `YearToDate`/`LastNDays`/`IsCurrent`) + a single-period helper, table-tested, before the UI rebuild.
- One decision teed up: keep range power behind Custom range (recommended) vs. drop ranges entirely.
- Analysis/TODO only.

## 2026-06-16 — Logged top-bar breadcrumb (B9)

- User wants a clickable breadcrumb on the right of the top-level panel for stepping backwards.
  Logged as B9. Flagged the real design decision: routing is flat (no nesting), so the trail needs a
  defined meaning — home-rooted `Dashboard / {page}` (recommended, no history needed), a visited-
  history trail, or a logical hierarchy once drill-downs carry context. Build once the trail behavior
  is confirmed.
- Analysis/TODO only (consistent with the user's recent review-and-queue mode).

## 2026-06-16 — Logged sidebar menu management (B8)

- User (analysis/TODO-only): menu items should Shift+drag-reorder, the "My pages" segment should be
  removed (custom pages integrate into the page), and there should be settings for which menu items
  are visible. Logged as B8.
- Noted overlaps: the Shift-gating mirrors the dashboard resize-handle pattern (B-section) and the
  reorder can reuse the `dashlayout` ordered-sequence/`Move` model; the visibility piece already has a
  base in `internal/modules` + `hideableScreens` + Settings toggles, so B8 is mostly extending that to
  cover all nav items (incl. B7's additions) and exposing it as menu management.
- No code this turn (per the user's instruction).

## 2026-06-16 — Polished boot loader + app settle-in animation

- Replaced the plain "Loading CashFlux…" boot text with an on-brand animated loader: a spinning
  accent stroke-arc ring around the Fraunces "C" mark (gentle breathing), with the wordmark + subtle
  subline fading up. Pure HTML/CSS in `web/index.html` (no Go change), so it paints instantly before
  the wasm finishes.
- Added an app settle-in: the MutationObserver that reveals `#app` now adds an `app-enter` class that
  runs a calm fade + slight lift/scale (cubic-bezier ease-out); `#boot` fades and scales away. Both
  gated behind `prefers-reduced-motion`.
- **Next.** (User has queued more menu requests — logging those separately.)

## 2026-06-16 — Logged UI-scale (B6) and missing-menu-items (B7); reverted a premature impl

- User asked for a font/UI-scale setting and to update the menu with all main-line features, then
  clarified they want these **logged as TODOs, not implemented** right now. I had started building the
  scale feature (prefs field + ApplyPrefs + CSS + settings control) — reverted those uncommitted
  changes back to HEAD and captured both as backlog items instead.
- **B6 (UI scale):** design is px-heavy, so a rem scale won't catch buttons — logged a `--ui-scale`
  zoom on `#app` driven by a `prefs.Scale` percent (70–130). Analysis kept for when it's picked up.
- **B7 (menu gaps):** confirmed via `screens.All()` that Planning, Allocate, Insights, Documents, and
  Customize are routed but absent from the rail (URL-only). Logged adding them as a nav group + module
  toggles, and deriving nav from the routed set so screens can't silently miss the menu again.
- **Process note:** when the user is rapidly reporting issues/requests, default to logging them as
  TODOs unless they explicitly say implement.

## 2026-06-16 — Localization foundation: central language store (i18n)

- User wants all page verbiage localizable via a central store with easy export/import of all langs
  (English-only for now). Confirmed the one consequential fork first (asked): **dot-namespaced keys**
  (`nav.accounts`) over English-source-as-key — stable keys, copy edits don't orphan translations,
  cleaner export files.
- Built the bottom-up foundation: pure `internal/i18n` — `Bundle`/`Catalog`, `T(lang, key, args…)`
  with an en→key fallback chain and `fmt.Sprintf` formatting, `Set`, `Languages` (default first),
  `MissingKeys` (coverage gap), and `ExportJSON`/`ImportJSON` of the whole multi-language bundle
  (the translator round-trip the user asked for). `DefaultBundle()` seeds the English source catalog,
  starting with the shell/nav. Table-tested (fallback chain, empty-as-missing, arg formatting,
  missing-keys, export/import round-trip, merge/overwrite, bad-JSON, languages ordering).
- This is the store only — no UI wiring yet. "Hook up all verbiage" is a large per-screen migration,
  so it's tracked as a multi-commit task (see TODOS §1.19): active-language atom + localStorage of
  imported langs + a language selector (English-only now) + converting each screen's strings to `T`.
- Verified `internal/i18n` green.
- **Next.** Wire the live bundle + active-language atom and a `t()` helper, then migrate verbiage
  screen by screen (start with the shell/nav already seeded).

## 2026-06-16 — Logged settings-duplication (B4) and collapsed-rail-hover (B5)

- User reported two more items (analysis-only, add to TODOS):
- **B4 settings duplication:** confirmed two settings surfaces. The `/settings` nav screen
  (`screens.Settings()`) is just a read-only household summary + the debug log; all real editing lives
  in the household-card global flip panel (`globalSettingsForm`). So the menu "Settings" item is the
  emptier duplicate. Logged fix: make the household-card panel the single primary surface — move the
  debug log viewer into it, then remove the `/settings` route/screen (or repoint the item to open the
  panel), updating the locked-screens/module-visibility references.
- **B5 collapsed-rail hover:** the rail already collapses to icon-only (`.collapsed`); the missing
  piece is a hover/focus flyout revealing each item's label. Logged: add `title` attrs + a CSS flyout
  pill in collapsed mode (overlay, don't widen the rail), reduced-motion + keyboard-focus aware.
- Both analysis-only per the user's instruction; no code changes for them this turn.

## 2026-06-16 — B2 step 1: pure dashboard packing engine

- Started the B2 dashboard-grid rewrite bottom-up: added `internal/dashlayout/pack.go` — an
  ordered-sequence model (`Item{ID,ColSpan,RowSpan}`) plus `Pack(items, cols)` (first-fit, row-major,
  span-aware, no overlap, deterministic, 1-based output), `Move(items, id, toIndex)` (the reorder a
  drag produces), and `ResizeItem`. `DefaultItems()` reproduces the current bento when packed at 4
  cols (verified by test, modulo the +1 header row offset the view will apply).
- Kept the legacy `Placement`/`Swap`/`Resize` API intact — this commit is the engine only, fully
  table-tested (default reproduction, mixed-span no-overlap + clamping, first-fit gap backfill, Move
  reorder/clamp/unknown-noop, ResizeItem clamp, no-mutation). UI migration + FLIP animation are
  separate follow-up commits so each stays green.
- **Design note:** chose first-fit *with* gap backfill (CSS auto-flow "dense" semantics) — deterministic
  and space-efficient; order still drives placement. Can revisit if strict order-preservation feels
  better once it's on screen.
- **Next.** Migrate dashboard state/UI onto Items+Pack (replace Swap-on-drop with Move+re-pack, persist
  the order, offset rows for the header), then layer the FLIP reorder/resize animations.

## 2026-06-16 — Dashboard: Shift-to-reveal resize handles (+ animation reqs → B2)

- User asked for three dashboard-editing refinements: resize handles only while holding Shift,
  animated size scaling, and smooth animated reorder.
- Shipped the standalone one now: `.rz` handles are hidden by default and revealed only while Shift
  is held. A global keydown/keyup listener (`internal/app/resizereveal.go`, wired once in `Run`)
  toggles `data-resize` on the document root; CSS fades `.rz` in/out off that attribute. A window
  `blur` handler clears it so the handles can't get stuck visible if focus is lost mid-hold. The
  callbacks live for the app's lifetime, so they're intentionally not released.
- The two animation requirements are entangled with the B2 reflow-engine rewrite (CSS-grid placement
  changes don't transition natively — they need a FLIP technique over the packed layout), so I folded
  them into B2 rather than bolt on a half-animation that the rewrite would throw away. B2 now lists
  "animate reorder" and "animate resize" explicitly.
- Verified the wasm build.
- **Next.** B2 proper (ordered-sequence packing model + FLIP animation) when picked up.

## 2026-06-16 — Diagnosed the page-duplication-on-route bug (B3)

- User reported the page sometimes duplicating on navigation and asked me to scan the live DOM with
  gwc. Couldn't: `gwc probe` reports `playwright unavailable: install the driver` and the gwc MCP
  server isn't connected this session. Diagnosed from the framework router source instead — which is
  definitive for this bug.
- **Cause:** GoWebComponents' router is a *nested-layout* router. `expandPathPrefixes("/accounts")` →
  `["/", "/accounts"]`, so `resolveRouteStack` produces a route stack `[exact "/", exact "/accounts"]`
  and renders `/` as the parent layout wrapping `/accounts` via `router.GetOutlet()`. But `app.go`
  registers every route — including `/` — as a full `Shell`, and no Shell calls `GetOutlet()`. Result:
  non-root navigation renders the `/` Dashboard Shell **and** the target Shell = duplicated page. The
  `*` route is innocent — `Register("*", …)` is the router's not-found factory, not a stacking pattern.
- **Fix (logged as B3):** adopt the framework's layout+outlet structure — `/` becomes a chrome-only
  layout with `GetOutlet()`, screens become child routes rendering just their content, Dashboard
  becomes an index child. Real refactor of `app.go` + `Shell` + screen registration; deferred.
- This also interacts with B1: once `/` is a proper layout, the deep-link refresh fix still needs the
  SW/server history fallback, but the in-app routing will at least be structurally correct.

## 2026-06-16 — Logged two bugs (deep-link 404, dashboard drag) to the backlog

- User reported two bugs; analyzed both and added a high-priority **§B Bug fixes** section to TODOS.
- **B1 — deep-link 404:** not a router bug. `NewHistoryRouter` gives clean pushState URLs and its `*`
  fallback only runs after the wasm boots; a hard refresh at `/accounts` hits the server first, which
  404s before `index.html` loads. `sw.js` only cache-falls-back on a thrown error (not a non-ok
  response) and doesn't cache `/accounts`. Fix is layered: SW navigation fallback to the cached shell
  + a server SPA history rewrite (ties into the existing `gwc dev -html` item). Keep clean paths — no
  hash router.
- **B2 — dashboard drag:** `ui.Widget`'s drop calls `dashlayout.Swap`, a pairwise exchange of absolute
  Col/Row + spans, so nothing reflows, there's no live displacement, and span-swaps overlap neighbors.
  Logged the real fix: re-model `dashlayout` as an ordered sequence + pure size-aware `Pack`
  (bin-packing), `Move(id, toIndex)` + re-pack for drag, persisted/migrated, with pointer-based live
  reflow in the UI — iOS-home-screen behavior that respects multi-cell tiles.
- Analysis only this turn (user asked to add to TODOS, not implement). Both are bottom-up: B2 starts
  with the pure packing model + table tests before any UI.
- **Next.** Implement when picked up — B1 is the smaller/safer (SW + serve config); B2 is a real
  layout-engine rewrite (pure model + tests first).

## 2026-06-16 — Extract to-do ordering into a tested package

- Knocked off part of TODOS §1.14 "Tests: ordering, status transitions": the list ordering was a
  `sort.Slice` inline in the js-only `todo.go`, untestable. Moved it to pure `internal/tasksort`
  (`Order` returns a sorted copy; `Visible` applies the hide-done filter), both non-mutating, with
  table tests covering open-before-done, dated-before-undated, due ascending, title tie-break, the
  no-mutation guarantee, and the hide-done filter.
- Mirrors the earlier `txnfilter` extraction — same pattern of pulling a core behavior out from
  behind the wasm build tag so it gets native table tests. The screen lost its `sort` import.
- Verified `internal/tasksort` green + wasm build.
- **Next.** More inline-logic extraction / small polish (parked items await user input).

## 2026-06-16 — Extract (and fix) the net-worth percent-change calc

- The dashboard KPI computed `(net - prev) * 100 / prev` inline — both a "no computation in view
  code" violation and a latent bug: dividing by the *signed* baseline flips the arrow when net worth
  is negative (−1000 → −500 is a +50% improvement but rendered as ▼50%). Liability-heavy households
  hit this.
- Added pure, table-tested `ledger.PercentChange(curr, prev) (pct, ok)`: `ok=false` for a zero
  baseline, and division by `|prev|` so the sign tracks the real direction (cases cover increase,
  decrease, negative-baseline improving/worsening, crossing zero, and toward-zero truncation). The
  dashboard now calls it and only renders the delta when `ok`.
- **Why ledger and not a UI helper?** It's a money-derivation, same family as `NetWorth`/`PeriodTotals`
  — keeping it there means it's covered by the package's table tests and reusable by other KPIs.
- Verified `internal/ledger` green and the wasm build.
- **Next.** Keep extracting inline view computations into tested helpers, or other small polish.

## 2026-06-16 — Toast confirmations for data actions

- Made the toast dual-purpose (it already supported a non-error/info style via `Notice.Err=false`):
  the Settings data actions — Export JSON/CSV, Import, Load sample, Wipe — now post a success message,
  and the errors they previously swallowed (`if err != nil { return }`) now surface as error toasts.
  "Mark all updated" on Accounts reports the count it refreshed via `plural(n, "balance")`.
- **Plumbing note:** these are package-level funcs (`exportJSON`, …), not components, so they can't
  call the `UseNotice` hook. The `state` package exposes no non-hook atom setter (only `UseAtom`), so I
  threaded a `notify func(string, bool)` closure captured in `globalSettingsForm` (where the hook is
  valid) down into each. Clean and keeps the hook rules intact.
- Scope check: this is the §1.4 error-surface item, not feature-inference — export/import/wipe
  failing silently was a real gap; the success confirmations are the natural complement.
- Verified the wasm build; touched files are js-only (no native target).
- **Next.** Small polish; comprehensive feature set otherwise (layered config + sync remain
  deliberately out of scope pending spec agreement / backend).

## 2026-06-16 — Route remaining swallowed writes through the toast

- Followed up the toast surface by sweeping the last `_ = app.Put…` sites in the screens
  (`grep _ = app\.(Put|Delete|…)`): Accounts' "Mark all updated" bulk balance refresh, and the
  dashboard freshness nudge's "Remind me" task creation. Both now surface a friendly toast on failure.
- The nudge additionally **gates navigation on success** — previously it jumped to /todo regardless,
  so a failed `PutTask` left the user staring at a list missing the task they just "created." Now it
  stays put and explains.
- Screens-wide grep is clean of swallowed entity writes after this. Verified the wasm build; the
  touched files are js-only so there's no native target (native suite remains green from before).
- **Next.** Small polish; the local-first feature set is comprehensive (sync stays out of scope).

## 2026-06-16 — App-wide toast surface for silent failures

- Picked up TODOS §1.4 "Error/toast surface for failed persistence." Several bulk paths in the
  ledger screen swallowed errors with `_ = app.Put…` (bulk recategorize, bulk mark cleared/uncleared,
  and the paired-transfer delete) — so a failed write left the UI looking successful.
- Added `uistate.Notice` (a tiny `{Seq, Text, Err}` atom; `Seq` bumps per post so identical text
  still re-fires, `With`/`Cleared` helpers) and `app.Toast`, a single bottom-center toast mounted in
  the Shell. It auto-dismisses via `UseEffect` keyed on `Seq` (a `setTimeout` whose cleanup clears the
  timer and releases the `js.Func` exactly once — `Cleared()` preserves `Seq` so the fire doesn't
  re-trigger the effect). Wired the three swallowed sites to post friendly errors.
- **Why an atom + global component rather than per-screen error state?** Bulk/delete actions often
  have no visible error slot (unlike the add form's `errMsg`), and the surface should be reusable by
  any screen. One shared atom keeps it DRY and lets future call sites opt in with a one-liner.
- **Trade-off:** kept the add form's inline `errMsg` (validation feedback belongs next to the form);
  the toast is for incidental/background failures. Auto-dismiss timeout is a fixed 4.5s for now.
- Verified the wasm build and native suite (toast/notice are js-only, so no native target — expected).
- **Next.** Route more swallowed `_ = app.Put…` sites (other screens' bulk/delete) through the toast,
  or further small polish.

## 2026-06-16 — Dashboard week boundaries honor the week-start preference

- Closed the nit flagged in the previous entry: `defaultWindow` hardcoded `time.Monday`, so the
  dashboard's Week resolution ignored a Sunday week-start preference. Now it seeds from
  `loadPrefs().WeekStartWeekday()`.
- Added a pure, tested `period.Window.WithWeekStart(weekday)` mutator (re-snaps both anchors under
  the new convention; no-op when unchanged; leaves Month/Quarter anchors alone). Settings' `savePrefs`
  reconciles the period atom through it whenever the week-start changes, so the dashboard updates live
  — consistent with how the date-format pref already updates lists live.
- **Why a mutator in the pure package rather than rebuilding the window in view code?** Date/anchor
  math belongs in `internal/period`, not the shell; this keeps the screen a thin caller and gives the
  behavior table tests (week re-snap Mon→Sun, no-op, month-untouched).
- Verified: `internal/period` green, wasm build green, full native suite clean.
- **Next.** Further small polish; the local-first feature set is comprehensive (sync remains the only
  out-of-scope major item — needs a hosted backend).

## 2026-06-16 — Dashboard resolution persists across reloads

- The top-bar Week/Month/Quarter toggle now survives reloads. `uistate.PersistResolution` stores the
  chosen `period.Resolution` in localStorage; `defaultWindow()` seeds `UsePeriod` from it via
  `loadResolution()` and re-anchors to `time.Now()`. The `ResolutionControl`'s `OnSelect` persists
  before setting the atom.
- **Why persist only the resolution, not the whole window?** The From/To anchors are transient
  navigation — restoring last session's anchored week/month would dump the user on a stale period.
  Remembering just the granularity keeps their preference (e.g. "I think in quarters") while always
  landing on the current period. Stepping the pills stays in-memory by design.
- Pre-existing nit noted for later: `defaultWindow` still hardcodes `time.Monday` for week-start
  rather than reading the prefs week-start atom, so the dashboard week resolution may not match the
  user's configured first-day-of-week. Left as a separate, orthogonal fix (one feature per commit).
- Verified the wasm build and the native suite (`internal/period` green); `uistate` is js-only so it
  has no native target, as expected.
- **Next.** Reconcile the dashboard window's week-start with the prefs atom, or further small polish.

## 2026-06-16 — Refactor: transaction filtering → pure tested package

- Moved the ledger's filter+sort out of the js-only `transactions.go` (untestable) into pure
  `internal/txnfilter`: `Criteria` (the persisted shape), `Apply` (filter + sort, non-mutating), and
  `AbsAmount`. `uistate.TxFilter` is now a type alias for `txnfilter.Criteria`, so the localStorage
  atom and JSON are unchanged; the screen calls `txnfilter.Apply`.
- **Why:** filtering is core behavior (account/category/member/text/date/cleared + three sorts) that
  had zero tests because it lived behind the wasm build tag. Now it's table-tested (8 cases incl.
  tag-text match, date range, each sort, and a no-mutation check) per the standards.
- Kept the alias so nothing downstream changed type-wise; verified the full native suite plus the
  wasm build. The explicit `go test ./internal/uistate` "setup failed" is just that js-only package
  having no native build target — `./...` skips it cleanly.
- **Next.** Genuine small polish or further testability extraction; the feature set is comprehensive.

## 2026-06-16 — Budgets: period summary header

- Added a stat-grid above the budgets list: total spent, total budgeted (sum of each status's
  spent+remaining), and amount left for the viewed period. Folded the totals into the existing
  over/near counting loop so it's a single pass. "Left" tone follows its sign via `accentFor`.
- Parallels the goals summary; both give a one-line "where do I stand" without scanning every row.
- **Next.** Genuine small polish; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Goals: combined progress header

- Added a stat-grid above the goals list (when there are goals): total saved, total target, and
  overall progress % — the at-a-glance "how am I doing across everything" the per-goal bars don't
  give. Amounts sum directly since goals are stored in the base currency; percent clamps at 100.
- Reused the shared `stat` cell used on the accounts net-worth header for visual consistency.
- **Next.** Genuine small polish; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Transactions: filtered summary line

- Added a "N shown · net $X" line above the ledger list: the count of the filtered set plus its net
  total, each transaction converted to the base currency via the FX rates (skipping any that fail to
  convert). Recomputes from `shown` each render, so it tracks the filter (account, category, member,
  date, cleared) live.
- **Net, not income/expense.** It's the raw sum of shown amounts — for an account or category filter
  that's the meaningful figure; transfers (rare in a filtered view) net out within an account anyway.
- Pairs naturally with the existing "Export CSV" of the same filtered set — see the total, then
  export it.
- **Next.** Genuine small polish only; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Docs: status refresh

- The CLAUDE.md "Status" bullet had drifted badly — it still listed custom-field defs, document
  vision AI, and reload-persistent prefs as *remaining*, all of which shipped many commits ago.
  Rewrote it to reflect reality: Phases 1–2 essentially complete, every entity add/edit/delete, the
  full pure-package roster (now ~21 packages), and the headline features per screen. Multi-device
  sync is called out as the only major remaining item (needs a hosted backend, out of scope here).
- Keeps the new-session quick-reference accurate so a future session doesn't re-implement done work
  or mis-scope what's left. TODOS.md remains the granular checklist.
- **Next.** Genuine small polish only; the local feature set is comprehensive.

## 2026-06-16 — Members → ledger drill-down

- Added a "Transactions" button to each member row that sets the persisted `TxFilter.Member` and
  navigates to `/transactions` — same parent-owned-closure + `OnView` pattern as the account
  drill-down (router + filter atom in `Members`, `MemberRow` stays import-light).
- Drill-down now exists from both accounts and members into the filtered ledger; consistent
  navigation across the app.
- **Next.** Genuine small polish; sync is the only large remaining item and needs a backend.

## 2026-06-16 — Accounts: update-balance reconcile

- Added "Update balance" to account rows: `promptText` asks for the real balance, then `setBalance`
  posts a cleared "Balance adjustment" transaction for `target − currentBalance` and stamps
  `BalanceAsOf`. So the computed balance matches a statement in one step, with the gap recorded as a
  real (cleared) transaction rather than silently overwritten — keeps the ledger honest.
- Marked the adjustment `Cleared` since it's a reconciliation entry. Zero-delta is a no-op (just
  marks checked). Reused the existing balance prop for the current figure and `ledger.Balance`
  semantics implicitly via the displayed balance.
- Satisfies the §1.9 "Update balance → adjustment txn + set BalanceAsOf" backlog item.
- **Next.** Genuine small polish; sync is the only large remaining item and needs a backend.

## 2026-06-16 — Freshness nudge → to-do (create-from-nudge)

- The dashboard freshness widget's stale state now has a "Remind me" button that creates a
  Nudge-sourced task ("Update stale account balances") and navigates to /todo for confirmation. The
  handler is a `ui.UseEvent` created at the top of `Dashboard` (stable hook) and threaded into the
  widget as a `ui.Handler` param — keeping the widget a plain function while keeping the hook order
  safe.
- **Gotcha:** `ui.UseEvent` returns `ui.Handler`, not `func()`; the widget param had to be typed
  `ui.Handler` for `OnClick` to accept it.
- Both create-from hooks are now done: AI insight → task (Insights) and freshness nudge → task
  (dashboard). The `SourceNudge`/`SourceAI` task sources both have real producers.
- **Next.** Genuine small polish; multi-device sync is the only large remaining item and needs a
  backend.

## 2026-06-16 — Insights → to-do (create-from-insight)

- The Answer card now has "Save as task": it creates a `domain.Task` from the AI result (rune-safe
  80-char title, full text in notes, `Source: SourceAI`, medium priority, open) via `PutTask`, with
  an inline "Saved to your to-do list." confirmation that clears when a new answer is requested.
- Wires the §1.14 "create-from-insight" backlog hook — AI advice becomes an actionable, tracked
  to-do instead of disappearing when you leave the screen. The AI `Source` tag (long defined) now has
  a real producer.
- **Next.** Genuine small polish; the only large remaining item (multi-device sync) needs a backend.

## 2026-06-16 — Accounts → ledger drill-down

- Each account row gets a "Transactions" button that sets the persisted `TxFilter` to that account
  (normalized, other filters cleared) and navigates to `/transactions` — a quick way to see one
  account's activity. The transactions screen already renders from the filter atom, so it just works
  on arrival.
- Did it with a parent-owned `viewTransactions` closure (router + filter atom live in `Accounts`),
  passed as `OnView`, keeping `AccountRow` from importing router/uistate directly.
- A near-equivalent of the "per-account ledger view" backlog item, reusing the existing filtered
  ledger instead of a separate screen (running-balance series could still be a future dedicated view).
- **Next.** Genuine small polish; sync is the only large item left and needs a backend.

## 2026-06-16 — Accounts: Mark all updated

- Added a bulk "Mark all updated (N stale)" button (shown only when something's stale) that stamps
  `BalanceAsOf = now` on every stale, non-archived account via `PutAccount` — clearing all stale
  badges and the dashboard freshness nudge in one click, instead of per-row "Mark updated".
- Reused `freshness.IsStale` + `FreshnessWindows` for both the visible count and the action, so the
  button's label and effect agree.
- **Next.** Genuine small polish as it arises; sync remains out of scope without a backend.

## 2026-06-16 — Accounts: edit lock-until inline (lock-until complete)

- Added the "Locked until" date to the account inline editor's asset branch (seeded from
  `LockUntil`; a blank value clears it, unlocking the account). `saveEdit` parses it into `cp`.
- Lock-until is now fully manageable: set on add, change/clear on edit, and it gates allocation
  suggestions. Existing accounts can be locked (e.g. when you open a CD) or unlocked when it matures.
- **Next.** Genuine small polish; major remaining backlog (sync) needs a backend.

## 2026-06-16 — Allocation: honor account lock-until

- Put the long-unused `Account.LockUntil` to work. The Allocate screen now skips an asset account
  whose `LockUntil` is in the future when building candidates — you can't put new money into a locked
  account (a CD, a vesting lot), so it shouldn't be suggested. Added a "Locked until" date to the
  account add form's asset section.
- Clear semantics (locked = no new money before the date), so no spec ambiguity — unlike a member
  view filter, which I'm deliberately not inferring.
- **Next.** Add the lock-until field to the account *inline editor* too (so an existing account can
  be locked/unlocked), then it's fully manageable.

## 2026-06-16 — Spending breakdown: roll up to parent categories

- The dashboard breakdown now attributes each expense to its top-level ancestor category, so
  sub-category spend aggregates under the parent (Food, not Food/Restaurants + Food/Groceries
  separately). A small cycle/orphan-safe `rootOf` walks `ParentID` up to the root; uncategorized
  ("") stays its own bucket.
- Reused the existing top-3-plus-Other rendering — only the bucketing key changed (root ancestor
  instead of the literal category), so the chart and legend are unaffected.
- Puts the category hierarchy to work in reporting, completing sub-categories beyond just display.
- **Next.** Genuine small polish as it arises; the major remaining backlog item (sync) needs a
  backend.

## 2026-06-16 — Sub-categories: re-parent on the inline editor

- The inline category editor gains a parent `Select`, so an existing category can be nested under
  another, moved, or promoted to top level. `saveCat` now takes a parent and sets `ParentID`;
  `CategoryRow` receives `AllCategories` and builds same-kind, self-excluded, indented options.
- **Self excluded; deeper cycles tolerated.** The picker drops the category itself (the obvious
  self-parent); picking a *descendant* could form a cycle, but `categorytree.Build`'s visited-set
  guard drops cyclic nodes from display rather than looping — so the worst case is a temporarily
  hidden branch the user can fix, not a hang. Changing kind clears the parent (kinds must match).
- Sub-categories are now fully usable: create nested, re-parent, indented display. Breakdown
  rollup-to-parent remains an optional future enhancement.
- **Next.** Optional: roll the dashboard spending breakdown up to parent categories; otherwise other
  small polish.

## 2026-06-16 — Sub-categories: add-form picker + indented lists

- Wired the tree engine into the UI. The add form gains a parent `Select` populated from
  `categorytree.Flatten(kindCats)` (indented with `indentLabel`), filtered to the chosen kind; the
  category lists now render flattened-by-depth so children sit under their parent. `CategoryRow`
  takes a `Depth` and prefixes its label.
- **Kind/parent consistency.** Changing the kind clears the parent choice (`onKind` resets
  `parentID`), since a parent must share the child's kind — avoids creating a cross-kind nesting.
- Deferred parent editing on the inline editor to keep this commit focused; the engine is cycle-safe
  so even an odd edit can't break the display.
- **Next.** Parent selector on the category inline editor (excluding self), then optionally rolling
  spending-breakdown up to parent categories.

## 2026-06-16 — Sub-categories: the tree engine

- `Category.ParentID` has existed in the schema but was unused. Started sub-categories bottom-up with
  a pure `internal/categorytree`: `Build` → forest of `Node`s (siblings name-sorted), `Flatten` →
  depth-tagged list for an indented picker/list.
- **Defensive by construction.** Bad parent data shouldn't break the UI: an orphan (parent missing)
  or self-reference becomes a root, and a mutual cycle (a↔b) yields no roots rather than recursing
  forever (a shared `visited` set stops re-emission). Tested all three: nesting+sort, orphan-as-root,
  and cycle/self-reference safety, plus flatten depth.
- **Next.** A parent selector on the category add/edit forms (using `Flatten` for the indented
  options, excluding self/descendants to keep it acyclic), then indented display on the Categories
  screen. Spending breakdown rollup-to-parent could follow.

## 2026-06-16 — Ledger: cleared balance

- Added pure `ledger.ClearedBalance` (opening balance + only `Cleared` transactions) — the
  reconciliation figure — mirroring `Balance` but skipping uncleared rows. Tested against a mix of
  cleared/uncleared/other-account transactions.
- Surfaced on the account row: when the cleared balance differs from the live balance, the meta line
  shows "· cleared $X", so the gap (uncleared activity) is visible at a glance and the cleared figure
  can be matched to a statement. Computed per row in the accounts screen.
- Reconciliation is now complete: per-row + bulk cleared toggles, a cleared filter, and the cleared
  balance to check against.
- **Next.** Genuine small polish as it arises; sync remains out of scope without a backend.

## 2026-06-16 — Transactions: bulk mark cleared

- Added "Mark cleared"/"Mark uncleared" to the selection bar. One `bulkSetCleared(val)` closure sets
  the flag on each selected transaction (skipping ones already in the target state) and clears the
  selection; two thin event hooks bind the two buttons.
- Reconciliation is now fully ergonomic: filter to "not cleared", select a run of statement-matched
  rows, "Mark cleared" — repeat. Per-row toggle remains for one-offs.
- **Next.** Continue with genuine small polish where it helps; the major remaining backlog (sync)
  needs a backend.

## 2026-06-16 — Transactions: cleared-status filter

- Completed the reconciliation loop: a tri-state cleared filter (both / not cleared / cleared) added
  to `TxFilter` (so it persists with the rest of the filter), honored in the shared `applyTxFilter`,
  and surfaced as a dropdown. "Not cleared" gives a clean reconcile worklist; the toggle then clears
  items off it.
- Reused the existing persisted-filter + `applyTxFilter` plumbing — the new field flows through
  display, export, and persistence with no extra wiring.
- **Next.** Reconciliation is now usable end-to-end (toggle + filter). Will continue with genuine
  small polish; the major remaining item (sync) is out of scope without a backend.

## 2026-06-16 — Transactions: cleared/reconciled toggle

- Surfaced the long-defined-but-unused `Transaction.Cleared` flag. Each row gets a toggle
  ("Mark cleared" ↔ "Cleared ✓") that flips it via `PutTransaction`, and the meta line shows
  "· cleared" — the start of statement reconciliation.
- Used existing schema (no migration) and the per-row component hook pattern (`clr` event +
  `OnToggleCleared` prop taking the whole txn so the parent flips and persists).
- **Next.** A "cleared only / uncleared only" filter would round this out, but I'll keep increments
  genuine; otherwise polish or the out-of-scope sync.

## 2026-06-16 — To-do: inline edit (CRUD-edit complete across all entities)

- Added inline edit to `TaskRow` (title, priority, due, notes), the last entity without it.
  `saveTask` guards the priority with `TaskPriority.Valid()`, clears the due date when blank, and
  persists via `PutTask` — mirroring the goal date-clearing behavior.
- Now genuinely every entity — accounts, transactions, budgets, goals, members, categories, tasks —
  has inline edit, alongside add, delete, reassign-on-delete (categories/members), and bulk ops
  (transactions). The local CRUD surface is fully complete.
- **Next.** No edit gaps remain. Further work is optional UX polish (e.g. a member view filter,
  sub-categories) or the out-of-scope sync; I'll only add what's genuinely useful.

## 2026-06-16 — Categories: inline edit (CRUD-edit fully complete)

- The last edit gap: `CategoryRow` now edits inline (name + kind). `saveCat` guards the kind with
  `CategoryKind.Valid()` and persists via `PutCategory`.
- **Every entity is now fully editable inline** — accounts, transactions, budgets, goals, members,
  categories — each with the same unconditional-hooks + editing-toggle shape. Combined with the
  reassign-on-delete flows and bulk transaction ops, the CRUD surface is complete.
- **Next.** No CRUD gaps remain; further work is small UX polish or the out-of-scope sync. Will keep
  additions genuine and avoid churn.

## 2026-06-16 — Members: inline edit (name + color)

- Closed a real CRUD gap — members had add / delete / set-default but no edit. `MemberRow` now has
  an inline editor for the name and color via the same unconditional-hooks + editing-toggle pattern;
  `saveMember` persists through `PutMember`.
- **Next.** Category edit (name/kind) is the last remaining CRUD-edit gap; after that every entity
  is fully editable.

## 2026-06-16 — Transactions: export the filtered view to CSV

- Added an "Export CSV" button to the ledger filter bar that downloads exactly what's shown. To
  guarantee export==view, I extracted the inline filter+sort into a pure `applyTxFilter(txns, f)`
  used by both the render and the export handler — no duplicated predicate logic to drift.
- `appstate.TransactionsCSV(txns)` wraps `store.TransactionsToCSV` for an arbitrary subset (the
  existing `ExportCSV` does all), and a screens-local `downloadBytes` mirrors the app-package one
  (Blob + transient anchor) so the screens layer can trigger egress without reaching into app.
- **Caught my own refactor fallout:** removing the inline `fa/fc/fm` locals left the filter-option
  builders referencing them; pointed those at `f.Account/.Category/.Member`. Also re-ran the native
  suite in a clean shell — a stray `GOOS=js` from the combined build command had made the first
  `go test` falsely FAIL (the known lingering-env gotcha), green once isolated.
- **Next.** The feature set is comprehensive; I'll keep making small, genuinely-useful additions
  (export ergonomics, empty states) and avoid inventing churn — the major remaining item (sync)
  needs a backend.

## 2026-06-16 — Accounts: editable owner (ownership editing uniform)

- Added the owner picker to the account inline editor. Since `AccountRow` already builds a `cp` copy
  on save, this just sets `cp.OwnerID`/`cp.Scope` from the new `ownerS` and adds the select (reusing
  `ownerSelectOptions`).
- Ownership is now editable inline everywhere it can be owned — accounts, budgets, goals — closing
  the "ownership assignment UI" gap beyond the create-time selectors and the member-delete reassign.
- **Next.** The local, single-device feature set is now effectively complete (every entity: create /
  edit / delete / reassign; full budgeting/goals/planning/allocation/AI/documents/customization/
  preferences/PWA). The one remaining large backlog item, Phase-3 multi-device sync, needs a hosted
  backend and per-entity version metadata — out of scope for this local-first build to implement
  meaningfully. Will keep doing contained polish where it adds real value.

## 2026-06-16 — Goals: editable owner

- Added the owner picker to the goal inline editor too, reusing `ownerSelectOptions`. `saveGoal`
  gained an owner param and sets `OwnerID`/`Scope` the same way as budgets. Budgets and goals now
  both allow post-creation ownership changes inline.
- **Next.** Account owner edit to finish uniform ownership editing, then the local feature set is
  effectively complete.

## 2026-06-16 — Budgets: editable owner

- You could set a budget's owner at creation and reassign it only by deleting a member; now the
  inline editor has an owner picker. `saveBudget` gained an owner param and sets `OwnerID` + `Scope`
  (shared for the group, individual otherwise) consistently with the add path and the reassign flow.
- Added a reusable `ownerSelectOptions(members, selected)` helper (group + members) — the first step
  toward sharing owner editing across goals and accounts too.
- **Next.** Same owner picker on goal and account inline edits, then ownership editing is uniform.

## 2026-06-16 — Budget periods: the UI (feature complete)

- Wired periods into the budgets screen. A period `Select` (shared `periodOptions` over
  `domain.AllPeriods`) on both the add form and `BudgetRow`'s inline editor; `saveBudget` grew a
  period param (guarded by `Period.Valid()`).
- **Per-budget evaluation.** Replaced the single `EvaluateAll(start,end)` over one shared month with
  a loop that calls `budgeting.PeriodRange(b.Period, viewMonth, weekStart)` per budget and
  `Evaluate`s each in its own window. `weekStart` comes from the prefs atom, so weekly budgets
  respect the user's Sunday/Monday choice. Each row shows its period label.
- **Note on the month stepper.** It still navigates a reference date by month; a weekly/quarterly
  budget shows the period *containing* that reference. Stepping by the budget's own unit would be
  nicer but means a per-period stepper — deferred; the current behavior is correct and clear with the
  period label visible.
- Budget periods are now end-to-end: enum → `PeriodRange` engine (tested) → selector + per-budget
  evaluation.
- **Next.** The local feature set is essentially complete; the remaining large item (Phase-3 sync)
  needs a backend. Will continue with small polish or note completion.

## 2026-06-16 — Budget periods: enum + range engine

- Lifting budgets beyond monthly, bottom-up. `domain.Period` gains `PeriodWeekly`/`PeriodQuarterly`
  (with a `Label()` for the UI) and `Valid()`/`AllPeriods` updated. `budgeting.PeriodRange(p, ref,
  weekStart)` returns the half-open window of the period containing `ref`: weekly via
  `dateutil.WeekStart` (honoring the week-start pref) +7d, quarterly snapped to the calendar quarter
  (Apr–Jun → Apr 1..Jul 1), monthly via the existing `MonthRange`; unknown falls back to monthly.
- **Caught a brittle test.** `validate`'s budget test used `Period: "weekly"` as its *invalid*
  example (from when only monthly existed). Widening the enum made "weekly" valid and flipped the
  test; updated it to `"yearly"` (a genuinely unknown period). Good reminder that "invalid" fixtures
  age when an enum grows.
- Tested PeriodRange for monthly/weekly(Sun & Mon start)/quarterly.
- **Next.** Wire it into the budgets screen: a period selector on the add/edit form and per-budget
  evaluation using `PeriodRange(b.Period, ref, weekStart)` instead of one shared month range.

## 2026-06-16 — Goals: linked account

- Put the long-defined-but-unused `Goal.AccountID` to work. Added an optional "linked account"
  select to both the add form and the inline editor (shared `goalAccountOptions` helper with a
  leading "no link" choice), threaded the account id through `add` and `saveGoal` (its signature
  grew a param), and the row now shows "· linked to <name>" via an `accountName` lookup.
- **Scope — record the link, don't auto-sync the balance (yet).** Linking captures *which* account
  funds the goal; it doesn't override `CurrentAmount` from the account balance. That keeps the
  contribute flow and progress semantics intact while making the relationship explicit and editable.
  Auto-funding from the linked account could be a later opt-in.
- **Next.** Local feature set is essentially complete; remaining substantial item is Phase-3 sync
  (needs a backend). Will continue with small polish (e.g. owner editing on existing entities) or
  note completion.

## 2026-06-16 — Goals: pace guidance (save $X/mo)

- Added `goals.MonthlyNeeded(goal, from)`: remaining ÷ whole months until the target date, partial
  final month rounded up, ceil division so a goal is never under-funded. Returns ok=false for
  no-target-date, already-complete, or past-due goals. Pure, table-tested (incl. the rounding case).
- `GoalRow` shows "· save $X/mo" alongside the "by <date>" when the goal has a future target and
  isn't complete — turning a static deadline into an actionable pace. Reused `fmtMoney` and the
  prefs date format.
- **Decision — round up, minimum one month.** Better to suggest slightly too much than to land short
  of the goal by the date; a partial month (target day-of-month past today's) counts as a whole
  contribution month, and same-month targets floor to one month rather than dividing by zero.
- **Next.** Remaining backlog is dominated by Phase-3 sync (needs a backend); the local feature set
  is essentially complete. Will continue with small polish or flag completion.

## 2026-06-16 — Accounts: inline edit (CRUD-edit fully complete)

- The last CRUD-edit gap: `AccountRow` now edits inline. It mirrors the add form — name, opening
  balance, and the `If(isLiab,…)`/`If(!isLiab,…)` split for liability vs asset attributes. `OnSave`
  takes a fully-built `domain.Account`, so the row does the parsing (it has the currency) and the
  parent just `PutAccount`s it through validation.
- **Many hooks, all unconditional.** A dozen field states + their event hooks + the three action
  hooks are declared at the top; only the *return* branches on `editing`. Added small
  `moneyMajorOrEmpty`/`floatOrEmpty`/`intOrEmpty` seeders and `parseMoneyOrZero`/`parseFloatOrZero`/
  `parseIntOrZero` so blank optional fields round-trip as zero cleanly.
- **Currency intentionally not editable.** Changing an account's currency reinterprets every stored
  amount, so it stays fixed; everything else is editable. Opening balance edits flow through the
  balance calc immediately.
- Inline edit now exists for accounts, transactions, budgets, goals (+ categories/members via their
  forms). Every primary entity supports add / edit / delete.
- **Next.** The remaining substantial backlog item is Phase-3 sync (server + client), which needs a
  backend; otherwise the feature set is essentially complete. Will continue with contained polish or
  note completion.

## 2026-06-16 — Transactions: bulk recategorize (bulk actions complete)

- Added a category picker + "Apply category" to the selection bar. `bulkRecategorize` walks the
  ledger, and for each selected non-transfer transaction sets `CategoryID` and saves via
  `PutTransaction`, then clears the selection.
- **Transfers skipped.** Transfers aren't categorized (they show "Transfer"), so bulk recategorize
  ignores selected transfer legs rather than stamping a meaningless category on them — mirrors how
  the per-row edit/duplicate already exclude transfers.
- The empty option is "No category", so Apply can also *clear* categories — a legitimate bulk
  action — not just set one.
- Bulk actions are now complete: select → recategorize and/or delete → clear. Closes the §
  transactions bulk-ops TODO.
- **Next.** Remaining backlog is mostly Phase-3 sync (large) and smaller polish; will pick a
  contained item or note that the major feature set is essentially complete.

## 2026-06-16 — Transactions: bulk select + delete

- Added multi-select to the ledger. A `selected` set state in `Transactions`; each `TransactionRow`
  gets a `☐/☑` toggle button (reusing the to-do `check` style and the per-row-component hook rule),
  and when the set is non-empty a bar shows "N transactions selected" with Delete selected / Clear.
- **Bulk delete reuses `deleteTxn`.** Rather than a separate path, bulk delete calls the existing
  per-row `deleteTxn` for each selected id — so transfer pairs are still removed together, and a
  leg whose partner was already deleted is a harmless no-op. Selection clears afterward.
- Used a glyph toggle button instead of a real `<input type=checkbox>` to dodge the checked-attribute
  binding question and match the existing to-do check control — consistent and simple.
- **Next.** Bulk *recategorize* is the natural follow-up (reuse the category picker + a set update),
  or move to remaining polish / Phase-3.

## 2026-06-16 — Document import: dedupe vs existing (review TODO closed)

- Added duplicate detection so re-importing the same receipt doesn't double-enter rows. Pure side:
  `Row.Signature()` (date + normalized amount; description deliberately excluded) and `FilterNew`,
  with a `normalizeAmount` that strips `$`/commas/leading-`+` and formats to two decimals so "-4.5",
  "-4.50", and "$4.50" compare correctly.
- **Same Signature for both sides.** The screen builds the seen-set by rendering each existing
  transaction in the chosen account as a `Row{Date: FormatDate, Amount: FormatMinor}` and taking its
  `Signature()` — so the row-side and txn-side normalize identically. That sidesteps the earlier
  worry about "-4.5" vs "-4.50" mismatches: both go through the one normalizer.
- Scoped to the chosen account and to date+amount (not description), which is the right
  conservativeness — it won't suppress two genuinely-different same-day same-amount entries across
  accounts, and the user still sees and can re-add anything via the review list. Skips are reported.
- Closes the §2.2 review TODO (list, edit, remove, import, dedupe all done). Tests: signature
  normalization (incl. sign + description-excluded) and FilterNew.
- **Next.** Remaining large item is Phase-3 sync; otherwise smaller polish (bulk transaction ops,
  empty-state/a11y).

## 2026-06-16 — Document review: per-row edit (review complete)

- `DraftRow` now also edits inline: an Edit button reveals date/description/amount/category fields;
  Save calls `OnUpdate(index, Row)` which rebuilds the draft slice with the corrected row. Same
  unconditional-hooks-then-branch-on-editing shape as the budget/goal/transaction rows.
- Vision misreads (a smudged amount, a wrong date) can now be fixed in place rather than removed and
  re-entered, so the import is trustworthy without leaving the review step.
- The review screen is now full-featured: list → edit any row → remove any row → pick account →
  import. Only dedupe-vs-existing remains from the original review TODO.
- **Next.** Dedupe on import (skip rows already in the account by date+amount), or shift to Phase-3
  sync groundwork / other polish.

## 2026-06-16 — Document review: per-row remove

- Small polish on the just-shipped import: the review list rows are now `DraftRow` components with a
  ✕ that removes that row from the draft slice (`removeDraft(i)` rebuilds the slice without index i).
  So a misread line can be dropped before importing instead of importing everything or starting over.
- The row owns only an event hook (no state), so reusing instances across a removal is harmless; a
  plain index loop building `CreateElement` nodes is enough — no MapKeyed needed.
- That covers the "reject" half of the review TODO; per-row *editing* and dedupe-vs-existing remain
  as future polish.
- **Next.** Likely dedupe extracted rows against existing transactions, or shift to Phase-3 sync
  groundwork.

## 2026-06-16 — Document vision AI: the Documents UI (feature complete)

- Wired the three pieces into a working flow: Choose image → `pickImageDataURL` (a small js helper
  that creates a hidden file input, reads the chosen file via `FileReader.readAsDataURL`, and calls
  back with the data URL) → "Read with AI" → `SendVisionChat` with a strict JSON system prompt →
  `extract.ParseRows` → a review list → pick account → `importDraft` maps rows to transactions and
  saves through `PutTransaction`.
- **Forces a vision model.** Settings often holds `gpt-4o-mini` (fine for text, no vision), so the
  screen upgrades the model to `gpt-4o` for image reads rather than failing cryptically.
- **Mapping decisions at import:** amounts parse to minor units with the chosen account's currency
  and keep the model's sign (negative = expense); categories match by name (blank if unknown); an
  unparseable date falls back to today. Invalid/zero-amount rows are skipped. Review list is
  read-only for v1 — per-row editing can come later, but the user already controls the account and
  can decline to import.
- **js gotcha handled:** the framework's `OnChange` event doesn't expose the picked `File`, so the
  data-URL read is done with a direct `js.FuncOf` FileReader chain (funcs released on completion),
  the same pattern as the ai transport.
- Document vision import is now end-to-end (codec → transport → parser → UI). The CSV paste path is
  untouched and still key-free.
- **Next.** Larger remaining work is Phase-3 sync; smaller polish includes per-row editing of draft
  rows or empty-state/a11y passes.

## 2026-06-16 — Document vision AI: the extraction parser

- `internal/extract.ParseRows` bridges the model's reply to the import flow. Models are unreliable
  about output shape, so the parser is forgiving by design: bare array *or* object wrapper (tries
  transactions/rows/items/data/results), amounts as JSON numbers *or* strings, a spread of field-name
  synonyms (description/desc/merchant/payee/name), and it strips a ```json code fence. Rows with
  neither description nor amount are dropped.
- **Decision — strings out, not domain.Transaction.** `Row` is all strings and the package has no
  domain dependency. The user reviews/edits before import, and the screen maps rows → real
  transactions against a chosen account/currency at that point. Keeps extraction decoupled and the
  values exactly as the model gave them (editable).
- Fixed a first-draft bug where `amountString` returned early on a missing key instead of trying the
  next synonym — caught by the string-amount test. Six table tests cover array, wrapper, fence,
  skip-empty, and two error cases.
- **Next.** The Documents-screen flow: pick an image → base64 data URL → `SendVisionChat` with a
  strict "return JSON" prompt → `ParseRows` → editable draft rows → import against a chosen account.

## 2026-06-16 — Document vision AI: the transport

- Added `SendVisionChat` and, while there, factored the fetch promise chain out of `SendChat` into a
  shared `postCompletions(apiKey, baseURL, body, onResult, onError)`. Both senders now just build
  their body (text or vision) and hand it to the same network code — no duplicated js.Func juggling
  or release logic.
- Same contract preserved: exactly one of `onResult`/`onError` fires, errors are plain English, and
  the js.Funcs are released on completion. The vision reply parses through `ParseResponse` like any
  chat.
- **Next.** The Documents-screen flow: pick an image, base64 it into a data URL, call
  `SendVisionChat` with a "return JSON transactions" prompt, parse the JSON into draft rows, and let
  the user review and import. The JSON→transactions mapping should be a pure, tested helper.

## 2026-06-16 — Document vision AI: the request codec

- Started the document image-import feature (SPEC document vision) bottom-up with the pure codec.
  OpenAI vision differs from text chat in one way: the user message's `content` is an array of parts
  (`{type:"text"}` + `{type:"image_url",image_url:{url}}`) rather than a string.
- **Decision — a separate `visionRequest` shape, not a looser `Message`.** Rather than change
  `Message.Content` from `string` to `any` (which would ripple through every existing text call and
  weaken the type), vision gets its own small request/message/part structs in `ai/vision.go`. The
  *response* is identical to a text chat, so `ParseResponse` is reused as-is — no new parse path.
- Images travel as data: URLs, so the bytes go only to OpenAI (same BYO-key, client-side stance as
  the rest of the ai package). Tested the built JSON's structure (string system content, two-part
  user content, image url preserved) and that `ParseResponse` reads a vision reply.
- **Next.** A js/wasm `SendVisionChat` transport (read the picked file → base64 data URL → fetch),
  then a Documents-screen flow: pick an image, parse the model's JSON into draft transactions to
  review and import.

## 2026-06-16 — Transactions: inline edit (non-transfers)

- Completed CRUD-edit parity: income/expense rows now edit inline (description, amount, category,
  date). `TransactionRow` gained the editing toggle + four field states; the category picker needs
  the category list, so the row now takes a `Categories` prop. `OnSave(orig, desc, amount, cat,
  date)` hands the original txn back so the parent keeps the account and re-applies the sign.
- **Sign + account preserved, not re-entered.** The amount field shows the absolute value
  (`absAmount` + `FormatMinor`); on save the parent negates it iff the original was negative, so an
  expense stays an expense without a kind selector in the row. The account isn't editable inline
  (changing it is rare and affects currency) — that stays a delete-and-re-add.
- **Transfers excluded.** Editing one leg of a paired transfer can't keep the pair consistent, so —
  like Duplicate — Edit is hidden on transfer rows.
- Add / edit / delete now exist for accounts(+archive), transactions, budgets, goals, categories,
  members, tasks.
- **Next.** The big remaining items are document vision-AI parsing and Phase-3 sync. Likely start the
  sync groundwork with a pure, tested merge primitive, or do a smaller empty-state/a11y polish pass.

## 2026-06-16 — Goals: inline edit

- Same edit pattern as budgets, one entity over: `GoalRow` gets an `editing` toggle and name/target/
  date field states, with `saveEdit` calling a parent `OnSave(id, name, target, date)` that parses
  the target to minor units and the date via `dateutil.ParseDate`, saving through `PutGoal`.
- **Empty date clears the deadline.** A blank date field sets `TargetDate` to the zero time (no
  deadline) rather than erroring — matching the add form's optional-date behavior. The date input is
  seeded in ISO (`dateutil.FormatDate`) since `<input type=date>` needs ISO regardless of the user's
  display format preference.
- All ~12 hooks declared unconditionally; only the return branches on `editing`. Budgets and goals
  now both support add / edit / delete (goals also Contribute).
- **Next.** CRUD-edit parity is close; remaining big items are document vision-AI and Phase-3 sync.
  Could also do transaction edit, or start the sync groundwork with a pure merge primitive.

## 2026-06-16 — Budgets: inline edit

- Budgets were add/delete only; added inline editing of the name and monthly limit. `BudgetRow`
  gained an `editing` toggle plus name/limit field states and a `saveEdit` that calls a parent
  `OnSave(id, name, limit)`; the parent finds the budget, parses the limit to minor units, and saves
  through `PutBudget`.
- **Hook discipline.** All the row's hooks (`del`, `editing`, two field states, five event hooks)
  are declared unconditionally at the top; only the *return* branches on `editing`. So toggling edit
  mode never reorders hooks — the trap that bites when you wrap hooks in an `if`.
- Seeds the edit fields from the budget on each `startEdit` (not just initial mount), so reopening
  the editor always reflects the current values; the limit is shown in major units via
  `money.FormatMinor`.
- **Next.** Goal edit (same pattern) would round out CRUD-edit parity, or move to a larger item
  (document vision AI / Phase-3 sync groundwork).

## 2026-06-16 — Members: reassign-before-delete

- Mirrored the category reassign flow for members. `appstate.ReassignOwner(old, new)` moves owned
  accounts, budgets, and goals — and re-attributes the member's transactions — to the new owner,
  setting scope to match (shared for the group owner, individual otherwise) and clearing the
  transaction member when moving to the group. Tested with an account + goal reassigned to the group.
- The Members screen's delete now opens a reassign panel (default target: the shared group) instead
  of blocking; "Move and delete" reassigns then deletes. Same stable-hook discipline: panel hooks
  declared at the top, panel conditionally rendered, reusing the `Fragment()`-default pattern.
- **Decision — reuse the existing per-screen reassign shape rather than abstracting it.** Categories
  and members now have near-identical panels, but the entity types and the "what counts as owned/used"
  differ enough that a shared component would need awkward generics; two ~30-line panels read more
  clearly than one parameterized one. Noted in case a third reassign target appears.
- Both delete-guards (members §1.13, categories) are now reassign flows, not dead ends.
- **Next.** A larger remaining area — document vision-AI parsing, or a Phase-3 sync primitive — or an
  empty-state/accessibility polish pass.

## 2026-06-16 — Categories: reassign-before-delete

- Replaced the hard block on deleting an in-use category with a reassignment flow. Logic first:
  `appstate.ReassignCategory(old, new)` repoints every referencing transaction and budget (via the
  store directly — the records are already valid, just re-categorized) and reports how many moved;
  tested with a transaction and a budget.
- UI: `deleteCat` now opens a reassign panel (sets `reassignID`) when the category is in use, else
  deletes immediately. The panel lists the other categories in a select; "Move and delete" runs
  `ReassignCategory` then `DeleteCategory`, "Cancel" closes it. All the panel's hooks
  (`onReassignTo`, `confirmReassign`, `cancelReassign`) are declared at the component top and the
  panel itself is conditionally rendered, so hook order stays stable.
- **Decision — reassign to any category, not just same-kind.** Simpler and occasionally useful
  (recategorizing an expense as income-adjacent); validation already guarantees the target exists.
  Guard still prevents picking the same category or none.
- **Next.** Pick the next backlog item — likely the document vision-AI parse path, or a Phase-3 sync
  primitive, or smaller polish (empty-state/accessibility pass).

## 2026-06-16 — Freshness overrides: the editor (feature complete)

- Added a "Freshness reminders" section to the global settings left column: one number input per
  account type (a curated six), seeded from `app.FreshnessWindows()` so each shows its *effective*
  window (override or default). `setFreshness(typeKey, days)` writes `Settings.FreshnessOverrides`
  and bumps the data revision; the Accounts badges and dashboard widget re-read immediately.
- **Per-row component again.** Each input is a `freshnessRow` (CreateElement) so its `OnInput` hook
  is at a stable position — rendering them in a plain loop would break hook order.
- **0 means never.** Kept freshness's existing semantics (`window <= 0` → never stale) rather than
  inventing a separate "off" control; the helper text says "0 = never". To restore a default a user
  re-types it — acceptable, and avoids a tri-state.
- Freshness overrides are now end-to-end: stored field → `Merge` in the engine → `FreshnessWindows`
  application → Settings editor.
- **Next.** The category-delete reassign flow (move referencing transactions/budgets to another
  category before deleting), which currently just blocks with an error.

## 2026-06-16 — Freshness overrides: apply them

- `Settings.FreshnessOverrides` (a `map[string]int` of account-type → days) has existed and round-
  trips through export/import, but nothing read it — the screens always used `DefaultWindows()`.
  Wired it in via `appstate.FreshnessWindows()`, which converts the string-keyed overrides to a
  `freshness.Windows` and layers them over the defaults with the package's existing `Merge`.
- Both stale surfaces now use it: the Accounts list's stale badges and the dashboard Freshness
  widget (gave `freshnessWidget` a `windows` parameter rather than reaching for `app` inside it).
- **Bottom-up first.** No editor UI yet, but the feature is already functional — overrides set via
  imported JSON now change staleness — and the logic (`Merge`) was already tested. The Settings
  editor is the next, purely additive commit.
- **Next.** A Settings "Freshness" section: per-type day inputs that write `Settings.FreshnessOverrides`,
  so users can tune windows without editing JSON.

## 2026-06-16 — Transactions: persist the last filter

- The seven filter/sort fields were independent `UseState`s that reset on reload. Consolidated them
  into one `uistate.TxFilter` struct behind a localStorage-backed atom (`UseTxFilter`), the fourth
  durable atom alongside layout, prefs, and hidden-modules.
- **Refactor shape — one atom, a `setFilter(mutator)` helper.** Rather than seven persisted states,
  the component reads `f := atom.Get()` for rendering and every change calls
  `setFilter(func(x *TxFilter){ x.Field = v })`, which gets-mutates-sets-persists in one place. That
  keeps each handler a one-liner and guarantees the whole filter is saved atomically on any change.
  Clear writes a normalized empty filter.
- All read sites (`ft/fa/fc/fm`, the date parses, the sort switch, and every input's `Value`/
  `SelectedIf`) now derive from `f`, so the screen has a single source of truth.
- **Why an atom and not just persisted `UseState`s:** reading-then-persisting the full struct avoids
  the trap of `Set` followed by a stale `Get` in the same handler — the mutator operates on a fresh
  `Get()` and persists exactly what it sets.
- **Next.** Another contained backlog item — the category-delete reassign flow, or the
  freshness-window overrides editor (the `Settings.FreshnessOverrides` field already exists).

## 2026-06-16 — Allocation: amount-split UI

- Added two number inputs to the Allocate profile card — amount to allocate and emergency buffer —
  parsed to minor units via the base currency's decimals. When an amount is present, `Distribute`
  runs over the current ranking and a `planByID` map feeds each `AllocRow` its suggested dollar
  figure (shown beside the score). A "Kept back" line surfaces the returned remainder.
- **Reactive for free.** Everything recomputes from the input states each render, so changing the
  amount, buffer, profile, or excluding a destination instantly re-splits — no explicit wiring,
  just the atom/state re-render.
- **Money discipline held.** All arithmetic is int64 minor units (`money.ParseMinor` in,
  `money.New`+`fmtMoney` out); the engine owns the only float. Amount column is blank until an
  amount is entered, so the screen stays clean for users who just want the ranking.
- Allocation constraints are now meaningfully complete: rank → exclude/restore → split an amount
  with buffer and (engine-level) per-destination caps.
- **Next.** A different backlog item — persist-last-transaction-filter, the category-delete reassign
  flow, or the freshness-window overrides editor.

## 2026-06-16 — Allocation: amount-split engine

- The ranking told you the *order*; `Distribute` now turns it into *amounts*. Pure function:
  proportional-to-score split of a total (minor units), after a `Reserve` (emergency buffer) is held
  back and with an optional `MaxPer` cap per destination. Returns `[]Plan` + the unallocated
  remainder.
- **Decision — don't redistribute the remainder, return it.** Capped overflow and integer-rounding
  leftovers, plus the reserve, all flow into the returned remainder rather than being re-spread.
  That keeps the function simple and deterministic, and the remainder is meaningful to show ("kept
  back: $X"). A redistribution pass can come later if users want every cent placed.
- Money stays int64 minor units throughout (code-rule #6); the only float is the transient score
  proportion. Even-split fallback when all scores are zero avoids a divide-by-zero and still does
  something sensible.
- Tested proportional split, reserve hold-back, per-destination cap, even split, and the empty /
  over-reserve edges.
- **Next.** Wire it into the Allocate screen: an amount input (+ optional buffer) that runs
  `Distribute` over the current ranking and shows each destination's suggested dollar amount.

## 2026-06-16 — Allocation: exclusion UI

- Wired the engine constraint into the Allocate screen. An `excluded` map state feeds
  `allocate.Constraints{Exclude: …}` into `RankWith`; excluded destinations drop out of the ranked
  list and surface in a new "Excluded" card with Restore.
- **Component split for hooks.** The ranked list was a plain `for`-loop of `Div`s; adding an Exclude
  button there would put an `OnClick` hook in a loop — the cardinal sin. So the row became its own
  `AllocRow` component (rendered via `MapKeyed`), and excluded entries are `ExcludedChip`
  components, each owning its action hook.
- **One toggle for both directions.** `toggleExclude(id)` adds or removes the id (cloning the map so
  the atom gets a fresh value), so the same handler powers Exclude and Restore. Added an empty-state
  for "everything excluded" so the list never looks mysteriously blank.
- **Next.** Either the remaining allocation constraints (emergency buffer / max-per-destination,
  starting again at the engine) or a different backlog item like persist-last-filter.

## 2026-06-16 — Allocation: exclusion constraint (engine)

- New backlog area (allocation constraints), started bottom-up with the pure engine. Added a
  `Constraints` struct to `internal/allocate` rather than a bare `exclude map` parameter, so the
  obvious follow-ups (max-per-destination, required/emergency buffer, min-balance) slot in as more
  fields without breaking call sites. First field: `Exclude` (candidate-ID set) with an `Eligible`
  predicate.
- `RankWith(candidates, weights, constraints)` filters ineligible candidates, then delegates to the
  existing `Rank`. Kept `Rank` untouched and proved `RankWith(_, _, Constraints{})` is identical to
  it, so existing callers and tests are unaffected.
- Tests cover exclusion (excluded id absent, survivors correctly ordered), the zero-constraint
  equivalence, and the `Eligible` predicate including the zero-value-accepts-all case.
- **Next.** Wire it into the Allocate screen: per-candidate exclude toggles that build the `Exclude`
  set and call `RankWith`, so the user can park destinations they don't want recommended.

## 2026-06-16 — Transactions: per-row Duplicate

- Small, self-contained quality-of-life feature: a Duplicate button on each transaction row. The
  handler copies the struct, swaps in a fresh `id.New()` and today's date, deep-copies the tags
  slice (so the copy doesn't alias the original's backing array), and saves through
  `app.PutTransaction` — so it re-validates and honors custom fields like any new entry.
- **Decision — clear the transfer link on duplicate and only offer it for non-transfers.** A
  transfer is a matched pair; cloning one leg can't recreate the pairing, so a duplicate
  deliberately becomes a plain standalone entry. Rather than silently produce a half-transfer, the
  button is hidden on transfer rows (`If(!IsTransfer, …)`).
- Reused the established per-row component pattern: `OnDuplicate` prop + a `dup` hook owned by
  `TransactionRow`, so the action button's hook stays at a stable position.
- **Next.** Another contained item — persist-last-transaction-filter, or an allocation constraint
  (e.g. exclude destinations / emergency buffer) which would start with a pure engine change.

## 2026-06-16 — Appearance prefs: the light theme (feature complete)

- Authored the `[data-theme="light"]` skin deferred last commit. Three layers needed overriding,
  because the candidate-C styling mixes three coloring strategies: (1) the legacy `:root` CSS vars
  (one block flip covers all the screen components — cards, stats, fields, buttons, bars, badges);
  (2) the shell's Tailwind utility classes (`.bg-base`, `.text-fg`, `.border-line`, the
  arbitrary-value `.bg-[#1c1c1e]` active-nav surface — escaped as `.bg-\[\#1c1c1e\]`); (3) the
  widgets' hardcoded hexes (`.w`, `.seg`, `.rpill`, flip panel, `.set-input`, `.switch`, scrollbars).
- **Verified the one risky override.** `text-base` is also a Tailwind font-size utility, so blindly
  coloring it could turn body text invisible — grep showed it's used in exactly one place (the brand
  badge, alongside `bg-fg`), so inverting both there is correct and contained.
- The accent var is deliberately left out of the theme block: it is user-chosen and reads fine on
  both backgrounds, so it stays applied on top of whichever theme is active.
- Appearance preferences are now complete end-to-end: engine (week-start/date/theme/accent/density)
  → localStorage atom → Settings UI → `ApplyPrefs` to the DOM → working light/dark skins, all
  reload-persistent. Only fiscal-month start remains from the original preferences line.
- **Next.** A fresh backlog item — likely persist-last-transaction-filter or a per-row transaction
  duplicate action (both small, contained), or an allocation constraint.

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

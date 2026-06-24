# Changelog

All notable changes to CashFlux are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Policy: **one feature per commit**,
and every commit updates this file under `Unreleased`.

## [Unreleased]

### Added
- **SMART series — T10 (AI import-mapping) + BL5/BL15 Free engines (2026-06-24):** **T10** (AI) maps pasted CSV columns to date/amount/merchant/category via the input control. **BL5** suggests timing flexible payments to just after payday when bills cluster pre-paycheck. **BL15** learns each liability's real payment timing from history and shows the typical days-after-due (the effective last-safe-pay date). 53 Free engines + 17 AI features. Tests.

- **SMART series — Free engines G14 + G19 (2026-06-24):** **G14** suggests linking an unlinked goal to an account so its progress tracks the real balance automatically; **G19** borrow-from-goal warning compares a goal-linked account's balance to the goal's recorded progress and warns when funds appear to have been pulled out (setback amount shown). 51 Free engines total. Table-driven tests.

- **SMART series — Free engine BL8 (paycheck-aligned grouping) (2026-06-24):** infers the user's payday from recent income and flags how many upcoming bills (and their total) fall before the next paycheck, surfacing a between-paycheck cash crunch. 49 Free engines total. Table-driven tests.

- **SMART series — Free engines D1, P5, BL1 (2026-06-24):** **D1** auto-generated to-dos (turns a backlog of recent uncategorized transactions into a one-tap to-do); **P5** goal-aware forecast overlay (how much the active goals consume monthly and the net left after); **BL1** predicted amount for variable bills (averages a varying biller's recent charges to estimate the next one). 48 Free engines total. Table-driven tests.

- **SMART series — 6 more Free engines: G3, P6, P9, SU6, SU8, BL4 (2026-06-24):** **G3** auto-allocate surplus to goals; **P6** forecast confidence band (±swing from trailing monthly-net range); **P9** break-even spending threshold; **SU6** per-subscription cost-creep history; **SU8** forgotten-since (stale subscription) surfacing; **BL4** autopay reconciliation (a payment near the due date marks a bill auto-paid). 45 Free engines total. Table-driven tests.

- **SMART series — Free engines G15 + SU11 (2026-06-24):** **G15 Debt-payoff optimizer** reuses payoff.BuildPlan to compare avalanche vs snowball total interest and surfaces the interest saved by paying the highest-APR debt first. **SU11 Zombie-charge detection** flags small ($226410/mo), long-running (6+ periods) recurring charges that are easy to forget. 39 Free engines total. Table-driven tests.

- **SMART series — 8 more AI features (16 total) (2026-06-24):** added **A3** (clean up an account name + infer type), **T1** (auto-categorize a transaction against the user's category list), **T3** (parse a plain-English transaction search into filter terms), **T5** (normalize a messy merchant string), **T12** (scan recent transactions for tax-relevant/deductible items), **G9** (recommend which goal to fund first), **SU10** (price-benchmark context for a subscription), and **SU13** (find bundle opportunities) — each as a `smartai` prompt builder + an `aiSpec` row, with new hook-free context builders (categories, recent transactions, goals). All 16 AI features share the gpt-5.4-mini→5.5 routing, show their per-use cost, and are gated behind a configured provider. Pure builders unit-tested.
- **SMART series — 6 more AI features via a generic, data-driven control (2026-06-24):** generalized the AI hub into one `smartAIControl` driven by an `aiSpec(code)` table, so adding an AI feature is a prompt builder + an `Implemented` flag + a spec row. Added **A10** (explain account health), **G4** (draft a goal from a plain-English wish), **P2** (plain-language what-if scenario), **AL4** (allocate in plain English), **SU2** (find overlapping subscriptions), and **D4** (add a to-do in plain English) — 8 AI features total alongside A5/P3. Each reuses the gpt-5.4-mini→5.5 routing, shows its per-use cost, and is gated behind a configured provider. Context builders are hook-free (safe at click time; fixed the outlook builder to use `dateutil.MonthRange` instead of the period hook). Pure `smartai` builders unit-tested; e2e still 16/16.
- **SMART series — Free engine P4 (suggested affordability inputs) (2026-06-24):** derives a sensible cash buffer from real essential monthly spend so the runway floor and the "Can I afford it?" reserve are grounded in actual spending, not a guess. 37 Free engines total. Table-driven tests.

- **SMART series — Free engine P1 (auto-discovered recurring) (2026-06-24):** scans transaction history (reusing subscriptions.Detect) for recurring charges not yet in the Planning recurring set and nudges the user to add them (with the estimated monthly total) for a sharper forecast and runway. 36 Free engines total. Table-driven tests.

- **SMART series — Free engines G8 + P8 (2026-06-24):** **G8 Goal-impact preview** expresses the month's biggest expense in terms of a goal's saving pace ("that $300 TV is ~7 weeks of your Vacation"). **P8 Auto-suggested extra debt payment** reuses `payoff.SuggestedExtra` over the household's liabilities, capped by the monthly surplus so it never pushes cash flow negative, and names the highest-APR debt to target. 35 Free engines total. Table-driven tests.
- **SMART series — more Free engines: G12, G18, T11, BL13 (2026-06-24):** four more deterministic engines (33 total). **G12 Auto-suggest emergency fund** nudges users with no emergency goal (and enough spend history) to start one at ~3 months of essentials. **G18 Feasibility traffic-light** flags each deadline goal whose required monthly contribution exceeds a fair share of the surplus (the "red light" that the deadline is unrealistic). **T11 Timeline annotation** marks the month's biggest single expense as a calm info note. **BL13 Statement-vs-minimum clarity** shows a revolving card's balance, minimum, and the monthly interest cost of paying only the minimum. Each auto-surfaces in the `/smart` catalog. Table-driven tests.
- **SMART series — AI feature P3: narrated outlook summary (2026-06-24):** the second shipped AI feature, reusing the same mini→5.5 routing and provider gating. A "Summarize my outlook" card on `/smart` builds a compact live snapshot (net worth + assets/debts, this-period income/spending) and asks the model for one calm plain-English paragraph — what's going well, what to watch, the single next step. Pure builder (`smartai.Outlook` + `OutlookSystem`) is unit-tested; the wasm control shows the per-use cost and is gated behind a configured provider. Demonstrates the catalog pattern scales: add a builder + mark `Implemented` + add a `smartAIControl` case.
- **SMART series — more Free engines: G13 windfall, BL6 late-fee, SU3 trial-conversion (2026-06-24):** three more deterministic engines (29 total). **G13 Windfall routing** detects an unusually large recent income deposit (≥1.5× average monthly income) and suggests routing it to goals/debt. **BL6 Late-fee risk** estimates the interest cost (balance × APR/52) of paying a liability bill a week late when it's due within 5 days. **SU3 Free-trial conversion** detects a merchant's first real charge following a $0/intro amount and warns at conversion. Each auto-surfaces in the `/smart` catalog with a Free badge. Table-driven tests.
- **SMART series — AI tier: mini→5.5 routing + A5 account Q&A (2026-06-24):** the AI (`[AI]`) infrastructure and the first shipped AI feature. New pure `internal/smartai` package (unit-tested): prompt templates (A5 system + `AccountQA` builder), the `Implemented` registry that gates which AI features get a UI, and `Acceptable` — the conservative answer check (blank/refusal/too-short) that drives escalation. New `internal/screens/smartai.go` (wasm): `runSmartAI` implements the product routing policy — call **gpt-5.4-mini** first, and escalate ONCE to **gpt-5.5** only when the mini answer isn't `Acceptable` — over the existing `ai.SendChat`/`SendProxyChat` transport (direct key or hosted backend). **SMART-A5 Natural-language account Q&A**: an "Ask about your accounts" bar on `/smart` that grounds the model in a compact live account/balance snapshot and shows the per-question cost up front. The whole AI section is gated on a configured provider — with none, it shows a "needs a provider" hint, never a dead control. e2e extended (`smart_hub_check.mjs`, 16/16): A5's row shows the AI tier badge + per-use cost + "needs a provider", and enabling it surfaces the gated AI section. Pure-package unit tests cover the prompt builder, implemented registry, and acceptability check.
- **SMART series — `/smart` hub e2e test (2026-06-24):** `e2e/smart_hub_check.mjs` drives the real wasm app end to end: loads sample data, opens `/smart`, asserts the Manage catalog renders with Free cost badges, asserts no insight cards before opting in (opt-out default), enables SMART-B8 and waits for a live insight card with the matching `data-feature` (proving the adapter→engine→card pipeline), verifies the opt-in + insight survive a reload, and verifies dismissal removes the card. Passes against the deep-link server (`go run e2e/serve.go web 8099`).
- **SMART series — `/smart` hub UI: glanceable insight cards + honest opt-in catalog (2026-06-24):** the world-class surface for the per-page intelligence layer, all in new additive files. New **Smart** screen (`/smart`, GroupTools/Plan) with two sections: (1) **Your insights** — glanceable cards (`smart_card.go`, each its own component per the On*-hook rule) showing a severity dot + title, a toned headline amount, a plain-English reason, and a footer with the one-tap action (navigate, or add-a-to-do via `PutTask`) + dismiss; rendered via `MapKeyed` keyed on the stable insight Key so a dismiss doesn't disturb siblings. (2) **Manage smart features** — the opt-in catalog grouped by page, each row a switch with a **cost-transparency badge**: Free (on-device, $0) or AI (`/use` per-call estimate + a "needs a provider" hint when none is configured). Only shipped features (those with a working engine) are listed, so no toggle is a dead end. Adapter (`smart_adapter.go`) builds the pure `smartengine.Input` from live `appstate` data and runs only enabled engines; dismiss/toggle persist via `uistate` and bump the data revision for live re-render. i18n in `en_smart.go`. Honors the strictly-opt-in promise: nothing computes or costs anything until the user turns it on.
- **SMART series — opt-in settings persistence (2026-06-24):** `internal/uistate/smartsettings.go` persists `smart.Settings` (enabled features + dismissed insights) as JSON in the PRESERVED settings KV (survives a dataset wipe, like theme/prefs). `LoadSmartSettings`/`SaveSmartSettings` + convenience mutators (`SetSmartFeatureEnabled`, `DismissSmartInsight`, `RestoreSmartInsight`); persisting bumps the store mutation revision so a toggled feature surfaces/disappears without a manual reload. Missing/unparseable value → everything OFF (the safe opt-in default).
- **SMART series — Planning + Allocate rule engines (P10/AL1/AL3) (2026-06-24):** **P10 Bill-shock early warning** projects large irregular (yearly/quarterly) recurring charges landing within a 75-day horizon and suggests a monthly set-aside (with a one-tap to-do). **AL1 Auto-suggested profile** recommends the allocation weight profile that fits the situation — debt (high-APR liability), safety (thin emergency fund), goals, or balanced — each with a plain reason. **AL3 Smart reserve** pre-fills the emergency buffer from real essential monthly spend × 6 months. Table-driven tests.

- **SMART series — Budgets rule engines (B8/B9/B10) (2026-06-24):** three Free Budgets engines. **B8 Safe-to-spend** computes one glanceable number — liquid cash minus the bills still due this month minus this month's goal contributions — and warns when the month is tight (negative). **B9 Pacing nudges** reuses `budgeting.Evaluate` + `ProjectPace` to flag budgets projected to overspend by period-end (past an early-period noise floor). **B10 Uncovered-spending** surfaces categories with real trailing monthly spend that no budget covers yet. New trailing-expense-by-category + bills-rest-of-month + goal-needs helpers. Table-driven tests.

- **SMART series — Subscriptions rule engines (SU1/SU4/SU14) (2026-06-24):** three Free Subscriptions engines reusing the `subscriptions` package. **SU1 Cancel-candidates** ranks subs by combining staleness (`NeedsReview`), recent price rises (`DetectPriceChanges`), and high share of the recurring total into a "consider cutting — save $X/yr" shortlist with natural-language reasons. **SU4 Annual-vs-monthly** flags the typical saving of switching a monthly sub to an annual plan (above a $60/yr floor). **SU14 Cancellation tally** is a positive-reinforcement scoreboard of how many subscriptions the user has cancelled. Table-driven tests.

- **SMART series — Goals rule engines (G1/G5/G6/G11) (2026-06-24):** four Free Goals engines reusing the `goals` package. **G1 Suggested contribution** computes the monthly amount to hit each deadline goal (`goals.MonthlyNeeded`) and checks it against the household's trailing-3-month surplus. **G5 Conflict detection** flags when active goals collectively need more per month than the surplus frees up, with the trim/extend amount. **G6 Milestone nudges** celebrate completed goals and encourage ones ≥75% done. **G11 Emergency-fund adequacy** measures a named emergency-fund goal against real monthly essentials and flags the months-covered gap toward a 6-month target. Shared trailing-baseline helpers (surplus/essentials) computed from transactions. Table-driven tests.

- **SMART series — Transactions rule engines (T2/T6/T7/T13) (2026-06-24):** four Free Transactions engines in `internal/smartengine`. **T2 Duplicate detection** reuses `dedupe.FindDuplicates` to surface same-date/amount/description double entries. **T6 Spending-spike** flags a recent expense ≥4× its own category average (with min-sample + min-mean floors, candidate excluded from its baseline). **T7 Missing-transaction** reuses `subscriptions.Detect` to notice an overdue recurring charge that hasn't posted (grace + staleness window). **T13 Refund matching** pairs a positive credit with a recent same-merchant, same-magnitude charge so a return doesn't distort category totals. Table-driven trigger/non-trigger tests.

- **SMART series — Bills rule engines (BL2/BL3/BL7/BL9) (2026-06-24):** four Free Bills engines in `internal/smartengine`. **BL2 Can-you-cover-it** projects total liquid cash over recurring flows (reusing `runway.Project`) and alerts when it dips below zero before the next inflow, naming the soonest at-risk bill. **BL3 Missed/overdue bill** flags a liability whose most-recent statement due date passed with no payment recorded on the account (grace + staleness windows). **BL7 Bill increase** reuses `subscriptions.DetectPriceChanges` to surface bills whose amount rose (≥5% and ≥$1). **BL9 Sinking-fund nudge** suggests a monthly set-aside for large irregular (yearly/quarterly) bills and offers a one-tap to-do. Table-driven trigger/non-trigger tests for each.

- **SMART series — Accounts rule engines (A1/A2/A4/A7/A8) (2026-06-24):** the first deterministic (Free, on-device, $0) engines, in a new pure package `internal/smartengine` that turns the existing math engines into glanceable `smart.Insight`s. `Run(Input, Settings)` executes only the *enabled* engines (an off feature does zero work) and drops dismissed insights; `RunPage` scopes to one page. Engines: **A2 Dormant-account nudge** (no activity ≥6mo + non-trivial balance → idle-cost estimate at a benchmark savings rate), **A4 Cash-positioning** (move idle cash from a low-APR liquid account to the best-yield one, quantifying the yearly gain), **A1 Balance-anomaly watch** (current-month spend ≥3× the account's own trailing-month baseline), **A7 Recurring-charge detection per account** (reuses `subscriptions.Detect` to summarize each account's recurring monthly burden), **A8 Overdraft forecast** (reuses `runway.Project` to warn before an account dips below zero, with the date and shortfall). All reuse `ledger`/`runway`/`subscriptions` rather than re-deriving math. Table-driven tests cover each engine's trigger and non-trigger paths plus opt-out gating, dismissal, and page scoping.
- **SMART series — foundation: catalog, cost-tiering, opt-in settings + model routing (2026-06-24):** the platform-independent spine for the optional per-page intelligence layer (SMART-A…BL). New pure package `internal/smart` (no `syscall/js`, fully unit-tested): `Page`/`Tier`/`Severity`/`Insight`/`Action` vocabulary; a `Feature` **catalog as data** holding all ~84 SMART-series items with their page, plain-English summary, and tier — **Free** (deterministic, on-device, $0) vs **AI** (needs an inference provider, billed per call); `CostEstimate`/`EstimateCost`/`FormatCents` for honest cost badges (Free→$0, AI→indicative per-call cents on the model that runs it, plus an escalated worst-case); and `Settings` — the strictly opt-out enablement model (every feature OFF by default; `Active()` filters a fresh insight batch to enabled-and-not-dismissed). Model routing (`internal/aiprovider/smart.go` + a new `gpt-5.4-mini` registry entry): smart AI calls default to the cheap, fast reasoning model **gpt-5.4-mini** at medium effort and **escalate to gpt-5.5 at LOW effort** only when the default proves insufficient. Tests: `smart_test.go` (catalog integrity, tier/cost math, settings/dismissal/active filtering, deterministic sort) + `aiprovider/smart_test.go` (model resolution, cheaper-than-escalation invariant, profile effort routing).
- **Operator console — user management + usage analytics (2026-06-24):** turned the read-only admin console into a usable business-maintenance tool. New admin management API (`internal/server/admin_manage.go`), all admin-gated + audited, no secrets/ciphertext/blob-bytes exposed: `GET /v1/admin/users/{id}` (single-user detail — profile, subscription, workspace count, storage, today's usage), `GET /v1/admin/users/{id}/usage?days=N` (per-user daily usage history), `POST …/plan` (override plan/status on an existing subscription), `POST …/revoke-sessions` (force re-login on all devices), `DELETE /v1/admin/users/{id}` (hard-delete account; self-delete blocked). New read method `Store.ListUserUsage`. Console SPA (`cmd/cashflux-admin/manage.go`): clickable users table → per-user management view with an account summary, a 14-day usage bar chart, and the actions (override plan, revoke sessions, two-step delete); styles injected from Go so the layer is self-contained. Tests in `admin_manage_test.go` cover success, 404/403/401/412, self-delete block, and no-secret-leak. (Builds on the parallel console/landing/auth work; this adds the management layer.)
- **EC8 — Landing: sell the product (benefit copy + real screenshots) (2026-06-24):** reworked the console landing from operator/infra jargon into product marketing that sells CashFlux to a person. Copy rewritten benefit-first — hero "Finally know where your money goes." with "accounts, budgets, goals and bills in one calm dashboard… No bank logins. No ads. No account required."; the six feature cards now lead with outcomes (see your whole money picture / budget the way you think / plan ahead / private by default / every number explained / yours to keep) instead of "tenant-safe admin API"/"AES-GCM blob store"; stats band reframed to "$0 to get started · 100% on your device · zero ads/trackers/resold data · 1-click export"; CTA "Take control of your money today." Added **real product screenshots**: a browser-framed hero shot of the dashboard (with a bottom fade mask) plus a "See it in action" gallery framing the reports and transactions screens with captions. New `shotFrame` helper + `.frame`/`.frame-bar`/`.frame-dot`/`.shot-hero`/`.shots-grid`/`.shot-cap` styles; emoji feature icons replaced with gradient `01–06` numerals (`.feat-num`). Screenshots copied into `web/admin/img/` (dashboard/reports/transactions, reused from `docs/screenshots/`). Pure presentation; no API/auth/state changes.
- **EC7 — World-class operator-console landing redesign (2026-06-24):** rebuilt the console home screen (`cmd/cashflux-admin/main.go` `homeView` + `web/admin/index.html` stylesheet) from a flat card list into a modern marketing landing: a sticky blurred **nav** with a gradient CashFlux wordmark + Sign-in, a **gradient hero** (eyebrow pill, clamped gradient-text headline, sub, primary/secondary CTAs, check-marked trust row), a bordered **stats band**, a **features section** (glass feature cards with hover-lift + radial glow, staggered fade-in), a **closing CTA band**, and a **footer** (API/Status/Privacy links). New design system in the inline `<style>`: layered near-black canvas with two animated radial gradient glows + a masked dot-grid, emerald→teal brand gradient, glassmorphic surfaces, Inter (Google Fonts, system fallback), focus-visible rings, `prefers-reduced-motion` honored. The login card and console header gained the gradient brand mark. Render helpers `brandMark`/`trustItem`/`statPill` added. Served from disk via `CASHFLUX_SERVER_CONSOLE_DIR`, so no server rebuild is needed to ship CSS/markup. Pure presentation — no API/auth/state changes.
- **EC6 — Console landing → login → console flow with dev-only credential prefill (2026-06-24):** restructured the operator console SPA (`cmd/cashflux-admin/main.go`) from a single token-login screen into a three-screen state machine: **Home** (product hero + 6 feature-highlight cards; "Sign in" button; "Open console" secondary button when a stored token is present), **Login** (token/password field + "Sign in" validates via `/v1/admin/overview`; "Back" link; dev-only "Prefill admin (dev)" button fetched from `/console/devcreds`), and **Console** (existing stat-card grid + users table; "Refresh" + "Sign out" in header). Sign-out returns to Home; invalid stored token on mount returns to Home rather than auth-error. Server: `Config.DevMode bool` (env `CASHFLUX_SERVER_DEV_MODE`, default false) added to `config.go`; `devCredsHandler` in `console.go` serves `GET /console/devcreds` returning `{"adminToken":"<token>"}` only when DevMode=true AND RemoteAddr is loopback AND Token is non-empty — any gate failure → 404 (production-safe, no endpoint enumeration); registered in `http.go` BEFORE the `/console/` catch-all. SPA prefill is purely client-side: on login-view mount the SPA fetches `/console/devcreds`; 200 → show button that fills the input field; 404 → render nothing. No token is hardcoded anywhere. CSS: `web/admin/index.html` extended with `.home-page`, `.home-hero`, `.home-title`, `.home-tagline`, `.home-actions`, `.feature-grid`, `.feature-card`, `.dev-banner`, `.btn-dev`, `.btn-link`. Tests: 3 new cases in `console_test.go` — 200+token in DevMode+loopback, 404 when DevMode=false, 404 from non-loopback.
- **Widget Builder — styling + layout tools, persistent card sizes (2026-06-24):** the builder gains a **Style** palette group (Color, **Accent color** `style.accent`, **Tone ▲▼** `style.tone`) and a **Layout** group (Stack with a **Direction**: stacked top→bottom or side-by-side). `style.accent` is a composable transform that recolors *any* visualization (chart series, KPI/stat figure, badge, progress fill — accent input ports added to KPI/badge/progress in the engine); `style.tone` forces ±coloring regardless of sign. New showcase presets: **styled-kpi** (accent-colored net-worth KPI) and **dual-kpi** (income + spending KPIs composed side-by-side via a row stack). A card's dashboard **size now persists with the graph** (`cardgraph.Graph.Cols/Rows`, UI-only like `Node.Pos`): the W/H steppers restore from a loaded card or preset, the working draft keeps its size across a reload, re-publishing updates the dashboard tile's span in place, and `vbPublishedWidget` passes the span as a fallback for cards rendered outside the packed layout. Engine tests: `TestStyleAccentRecolorsAnyViz`, `TestStyleToneForcesTone`, `TestGraphSizeRoundTrips`. e2e extended: Style/Layout groups + style nodes present, styled-kpi renders in its accent color, dual-kpi lays out as a row, and a published 4×1 card is visibly wider than a 1-wide tile, keeps `data-col-span=4` across reload, and restores W=4/H=1 in the builder on reload.
- **EC5 — Operator console SPA served at /console/ (2026-06-24):** standalone Go→WebAssembly operator console SPA (`cmd/cashflux-admin/main.go`, build tag `js && wasm`) served at `/console/` with SPA fallback. Token login persists in `localStorage["cashflux.admin.token"]`; auto-loads on mount. Four view states: loading skeleton, auth error (401/403 → "Not authorized"), network error, and ready. Ready view: stat-card grid (total users, estimated MRR, active/trialing/past-due/canceled subscriptions, storage, today's requests/tokens) + users table (email, provider, plan, status, created date). Sign-out and Refresh buttons in header. Static assets: `web/admin/index.html` (dark operator theme, CSS-only), `web/admin/wasm_exec.js`. Server changes: `consoleHandler` in `internal/server/console.go` (file-exists-or-SPA-fallback); `GET /console` redirect + `GET /console/` route registered in `NewMux`; `GET /` now redirects browsers (Accept: text/html) to `/console/`; `Config.ConsoleDir` + `CASHFLUX_SERVER_CONSOLE_DIR` env. Tests: `internal/server/console_test.go` (5 cases: index serve, SPA fallback, no-slash redirect, browser redirect, JSON non-browser).
- **Widget Builder — visual node-graph card designer (2026-06-24):** an n8n-style visual-scripting screen (`VisualBuilder`, routed in `screens.go`) for composing dashboard cards from a typed node graph, with the explicit goal of cloning the existing dashboard widgets 1:1. Pure engine in `internal/cardgraph` (no `syscall/js`, table-tested): a directed acyclic graph of strongly-typed nodes (sources, transforms, logic, viz, interactivity) with type-checked ports + safe coercions, Kahn cycle detection, named-variable bindings, and graceful degradation around broken nodes. UI (`internal/screens/widget_builder.go`): a real 2D canvas with cursor-anchored wheel-zoom, drag-to-pan, fit/reset, draggable nodes + bezier wires, drag output-port → input-port to connect and click-a-wire to disconnect (JS pointer shim); a palette grouped Data/Transform/Logic/Display/Interact; an inspector with a per-kind param schema; and a live preview rendered through the dashboard's OWN renderers (`kpiBody`/`kpiBodyHero`, `uiw.Chart`+`chartspec`, accounting `fmtMoney`/`figTone`/`ColorClass`) so clones match exactly. Save-to-library + publish-to-dashboard: custom `wb:` cards persist via `localStorage` and survive reload through `dashlayout.Reconcile`'s custom-id keep. Presets cloning real tiles: net-worth / assets (+ month-over-month subline) / income / spending / liabilities / account-count KPIs, a cash-flow stat, spending-by-category bar, spending-breakdown donut, spending-trend line, net-worth-trend area, a recent-transactions list (headerless, currency-formatted, toned), and an accounts list. Interactivity nodes (`ui.button`, `ui.toggle`) run app actions (apply rules / post recurring / add task) and bump the data revision; the toggle persists its checked state in `localStorage`. Datasets surfaced for source nodes: transactions (with date/desc/signed columns), accounts, budgets, goals, tasks, bills, and a 6-month end-of-month `net_worth_series` (via `ledger.NetWorthSeries`). Verified end-to-end (`e2e/widget_builder_check.mjs`): canvas pan/zoom, drag-to-wire + disconnect, every preset renders (KPI/bar/line/area/donut/list/stat), the recent list is headerless + currency-toned, save/reload, and publishing MULTIPLE custom cards to the dashboard with built-in chrome + typography that survive a reload.

### Fixed
- **Custom page → custom page navigation now swaps the body (2026-06-24):** clicking one custom page in the rail and then another *directly* updated the URL and top-bar title but left the BODY showing the previous custom page; routing through a built-in page in between hid the bug. Root cause: every custom page renders through the same `screens.CustomPage` component, and its `/p/:slug` View closure is built at one source line, so all custom pages share a function code-pointer — the reconciler saw the same element type with equal (empty) props and skipped re-rendering the page subtree (a built-in page has a different component type, which forces a remount). Fix (`internal/app/shell.go`): the Shell renders the active screen as `WithKey(uic.CreateElement(props.View), props.ActivePath)` — a per-route key gives each navigation a distinct element identity, so the reconciler unmounts the old page and mounts the new one on every hop, and each screen keeps its own fiber (its hooks no longer share the Shell's). Replaces an earlier working-tree attempt that rendered the view inline inside a keyed `pageView` wrapper (fixed the symptom but shared one fiber across all page types). Regression test `e2e/loopstory_90_custompage_nav.mjs` creates two custom pages with distinct widgets and asserts the body swaps on a direct custom→custom hop in both directions while built-in pages stay distinct. (Noted separately: navigation app-wide logs one benign "call to released function" console error per route change — pre-existing and unrelated to this fix.)
- **Operator console page flicker + request storm (2026-06-24):** the console SPA's mount effect (`cmd/cashflux-admin/main.go`) that auto-loads a stored token was registered with **no deps key**, so it re-ran on *every* render — each run re-fetched `/v1/admin/overview` + `/v1/admin/users` and bumped view state, which re-rendered and re-ran the effect, replaying the entrance animations (visible flicker) and hammering the backend (~1 fetch/second; the dev server logged 9,000+ overview requests). Added a constant deps key (`ui.UseEffect(fn, "admin-autoload")`, the same run-once pattern as `widget_builder.go`'s `"vb-drag-shim"`) so it fires exactly once on mount. Verified headless: overview calls over a 6-second console session dropped from ~6 to **1**.
- **`data.groupby` now honours its `sort` prop** (`internal/cardgraph/nodes.go`): `value` (descending — the default, for ranked breakdowns), `label` (ascending — chronological time series), or `none` (preserve input order). It previously always sorted value-descending, silently mis-ordering time-series charts (spending/net-worth trend); the pre-existing chronological test passed only because its values happened to descend in date order. Added `TestGroupBySortModes` exercising all four modes with genuinely divergent orderings.

### Added (other)
- **EC4 — Strong dashboard homescreen hero (2026-06-24):** glanceable "home band" above the bento grid on the Dashboard (`/`). Two states: (1) **Empty dataset** — welcoming first-run hero with the app value prop, a primary "Load sample data" CTA (wires to `app.LoadSample()`, same as Settings and Accounts), and a secondary "Add your first account" button; (2) **Non-empty dataset** — time-of-day greeting (Good morning/afternoon/evening by local hour), net-worth hero figure with `data-countup` animation, a compact this-month stats row (income / spending / net / savings rate via the memoized §1.6 selectors `useNetWorth`/`usePeriodTotals`/`ledger.SavingsRate`), and quick-action buttons (add transaction → quick-add panel, add account → add modal). All text via `uistate.T`; every button carries `Type("button")` + `aria-label` + `data-testid`; hook positions are stable (each variant is its own component). New files: `internal/screens/dashboard_hero.go` (build tag `js && wasm`; `dashboardHero`, `heroSummary`, `heroWelcome`, `heroStat`), `internal/i18n/en_home.go` (14 new `home.*` keys via `init()` loop into `english`, matching the `en_enterprise.go` pattern; `en.go` untouched). Modified: `internal/screens/dashboard.go` (one-line insertion: `ui.CreateElement(dashboardHero)` above the bento `Div`).
- **EC3 — Admin console screen (2026-06-24):** operator UI for the EC1 admin API. New `/admin` screen (GroupSystem, `AdminOnly:true`) renders a platform-overview stat grid (total users, est. MRR, active/trialing/past-due/canceled subscriptions, total storage, today's requests/tokens) and a users table (email, provider, plan, status, joined). Screen states: sign-in prompt (no backend configured), admin-only empty state (403), error+retry, loading skeleton, and ready. Nav entry is gated: the boot probe fires `GET /v1/admin/overview` non-blocking; HTTP 200 → `uistate.SetAdminConsoleAvailable(true)` → the System rail section shows "Admin"; any other outcome leaves it hidden. New files: `internal/i18n/en_enterprise.go` (admin.*/nav.admin/screen.adminSub keys via `init()`), `internal/uistate/adminconsole.go` (bool atom + capture/set seam), `internal/app/adminprobe.go` (`probeAdminAccess()` goroutine), `internal/screens/admin.go` (`AdminConsole` screen). Modified: `screens.go` (`AdminOnly bool` on Route; `/admin` registration), `shell.go` (`navGroup` reads+captures admin atom, skips AdminOnly routes when false; `/admin` railMeta entry), `app.go` (calls `probeAdminAccess()`). i18n keys added in `en_enterprise.go` so `en.go` is untouched.
- **EC2 — Zero-knowledge encrypted artifact blobs (2026-06-24):** when client-side dataset encryption is active (passcode set), artifact bytes are encrypted client-side before upload to the backend blob store; the server stores ciphertext only and never sees plaintext. Download path transparently detects and decrypts envelopes; legacy plaintext blobs pass through unchanged (backward-compatible). New `internal/app/artifactcrypto.go` (`//go:build js && wasm`): `artifactSalt()` — stable per-install 16-byte salt persisted in `localStorage["cf.artifactSalt"]`; `cachedArtifactKey(saltB64)` — synchronous PBKDF2 key derivation with in-memory cache (pays the 600 000-iteration cost at most once per unique salt per session); `encryptArtifactSync(plain)` — AES-GCM encryption with **deterministic IV = sha256(plain)[:12]** so identical plaintext under the same key yields identical ciphertext (enables backend content-address dedup and stable hash routing); `decryptArtifactSync(envBytes)` — parses `cryptobox.Envelope`, derives/caches key from the envelope's embedded salt (supporting multi-device sync where the salt differs), decrypts. Both sync wrappers block the calling goroutine on a buffered channel while the async `crypto.subtle` Promise resolves — safe because the WASM scheduler parks the goroutine and services the JS event loop (same pattern as sync HTTP). Modified `uploadBackendArtifactBlob`: if `datasetEncryptionActive()`, encrypt payload via `encryptArtifactSync` (error → abort, never silently upload plaintext); hash and PUT the encrypted payload; set `Content-Type: application/octet-stream` to avoid leaking real MIME to the server; preserve real MIME in `BlobRef` for client rendering. Modified `downloadBackendArtifactBlob`: after reading bytes, `cryptobox.IsEnvelope()` → `decryptArtifactSync`; else return as-is (plaintext backward compat).
- **F1 — Admin role + tenant-safe admin API (2026-06-24):** `Config.AdminUserIDs` (env `CASHFLUX_SERVER_ADMIN_USER_IDS`, comma-separated) and `Config.IsAdmin(userID)` (deny-by-default; empty list → nobody is admin). Two new bearer-authenticated, audited endpoints: `GET /v1/admin/overview` (cross-tenant aggregates: total users, subscription counts by status, estimated MRR cents, total blob bytes, today's requests/tokens) and `GET /v1/admin/users?limit=&offset=` (paginated user list with subscription status/plan; no secrets, no AI ciphertext, no blob bytes). Non-admin bearer → 403 + audit entry; unauthenticated → 401. New repository methods `ListUsers` and `AdminOverview` (parameterized queries only). `planMonthlyCents` table: monthly=$9.99/mo (999¢), annual=$99/yr→$8.25/mo (825¢), unknown=$0. Table-driven tests: aggregate correctness, pagination, limit cap (200), cross-tenant secret exclusion, admin/non-admin/unauthenticated authz.

### Changed
- **a11y — Appearance + Notification Center control-group names (2026-06-24):** the three Appearance screen control groups (theme mode, motion, accent) now carry `role="group"` + `aria-label` so screen readers announce the group name when focus enters the Segmented or SwatchPicker control (WCAG 1.3.1 / 4.1.2). Theme-mode group: new wrapper `Div(role=group, aria-label=T("settings.appearance"))` around the H4 + Segmented. Motion and accent groups: `role="group"` + `aria-label` added directly to the existing `.toggle-row` divs — no extra wrapping needed. Notification Center: rows now rendered with `role="list"` on the container and `role="listitem"` on each row (via `Body:` rather than `Rows:` so `EntityListSection` is untouched); "Clear all" button gains `aria-label=T("notifications.clearAllAria")`. One new i18n key (`notifications.clearAllAria`) in `internal/i18n/en_a11y.go` (new file, same `init()` pattern as `en_enterprise.go`/`en_home.go`; `en.go` untouched). No business-logic changes; build rc=0; i18n tests pass.
- **a11y/i18n — Goals page (2026-06-24):** routed all remaining hardcoded English strings through `uistate.T()` (`goals.noLink` in account-option list, `" to go"` sub-line suffix, `"Funded %d%% — %s"` over-fund note, `"Show/Hide advanced fields"` toggle); added `aria-label` to Contribute and Edit row buttons (Title was present but aria-label was missing); wired focus-restore on goal delete using `captureRowDeleteFocus` / `focusRowAfterDelete` + `.goal-list` sentinel class so keyboard focus never drops to `<body>` after a delete; added four new catalog keys (`goals.remaining`, `goals.overfundFmt`, `goals.showAdvanced`, `goals.hideAdvanced`).

### Added
- **i18n (2026-06-24):** routed remaining hardcoded aria-label/Title strings through uistate.T() across accounts/allocate/budgets/categories/custompage/split/task/todo/transactions/workflows; added catalog keys (`accounts.markClearedTitle`, `allocate.openSettingsAiKey`, `budgets.rolloverTitle`, `categories.viewTxnsTitle`, `common.dueDate`, `custompage.dragReorder`, `split.whatForLabel`, `transactions.clearedStatus` (pre-existing), `workflows.actionTypeLabel`, `workflows.triggerLabel`).
- **i18n:** routed hardcoded aria-label/Title strings through uistate.T() in datatable row-size control, budgets row cover/topup, planning, documents draft-review, and customize-formula; added catalog keys.
- **Per-member preferences (§1.19):** members now carry an optional personal date style + default account (`domain.MemberPrefs`), edited inline on the Members screen; resolution layers member over household via the new pure, tested `internal/memberprefs` package (built on `configlayer`). Quick-add preselects the active member's default account when set. Verified via `e2e/verify_memberprefs*.mjs` (fields render + save round-trips).
- **Segmented-control sliding pill (§6.16):** a `.seg-pill` slides under the active segment (measured offset → standard `transform`/`width`), animating selection instead of snapping.
- **playwrightgo `gwc`** built (`.tools/gwc-pw.exe`) for automated DOM verification driving Chromium.

### Fixed
- **Reports donut charts now have a legend.** The category-split and income-by-source donuts were bare
  coloured rings — no way to tell which slice was which. `renderDonut` (`web/chart.js`) now draws the ring on
  the left and a legend (swatch · category · share%) on the right, falling back to just the ring when the box
  is too narrow. Verified in both themes (`e2e/donut_legend_verify.mjs`, 2/2): 6 swatches + labels + matching
  percentages (53%/13%/…), no overlaps.
- **Reports bar-chart axes now show money, not bare numbers.** The D3 ranked-bar charts (category / payees /
  biggest expenses) drew their value axis as "0 / 500 / 1,000 / 1,500" while the rest of the page formats money.
  Added a `"money"` axis format (`chartspec.Axis`) resolved in `web/chart.js` to compact currency ("$1.5k") via
  the base-currency symbol — a new `ui.ChartProps.CurrencySymbol` passed live (`currency.Symbol(base)`), so it's
  correct for EUR/GBP/JPY bases, not a hardcoded `$`. Verified in both themes (`e2e/chart_money_axis_verify.mjs`,
  6/6): Y ticks now "$0/$500/$1k/$1.5k/$2k"; category labels and donut/area charts unchanged.
- **i18n a11y:** routed hardcoded `aria-label` and `Title()` strings through `uistate.T()` in datatable pagination ("Previous page" / "Next page"), workflows staged-action remove button and condition-variable insert buttons, and split-screen "Save split" / "Record settled" buttons; added catalog keys `ui.table.prevPage`, `ui.table.nextPage`, `workflows.removeAction`, `workflows.insertCondVar` (%s verb for token name), `split.saveSplitTitle`, `split.recordSettledTitle` to `internal/i18n/en.go`.

### Added
- **§3.4 Switch-server flow:** editing the server URL to a different host now signs out of the old server (clears token/CSRF + cloud-AI-key flag), resets sync to offline, and notifies — while keeping local data. Host-compared via a new `backendHost()` helper so same-server path/query edits don't drop the session.
- Sync chip tooltip now names the active server (`Server: <host>`); the Cloud upgrade sheet now mentions the self-host path alongside managed cloud (onboarding names both once).
- `docs/SECURITY_REVIEW_AI.md` — security review of all off-device AI egress: scope (the opt-in `aicontext` privacy tiers + top-N/recent-N caps), redaction controls present today, and residual risks/recommendations.
- SPDX `MIT` license headers swept across all 667 first-party Go files.
- `e2e/capture_product.mjs` — refreshes the deliberate product screenshots in `docs/screenshots/` against the current UI.
- Settings global panel switched to a single **Close** button (`CloseOnly`); its Save/Cancel footer was misleading because every setting applies live on change (§6.17).
- **B34 — Appearance page (`/appearance`):** the theming engine (theme mode, accent, density, Motion, full theme editor) moved out of the crowded Settings panel into its own routed, deep-linkable page reachable from the left rail + a Settings "Appearance & theme →" link. New `internal/browser` package (file pick/download) so the theme editor could move `internal/app` → `internal/screens` without an import cycle; Settings de-crowded to a single link + the dead `internal/app/theme_editor.go` removed.
- `internal/ui` primitives: `Skeleton` (WONDER-gated shimmer placeholder for loading content) and `MeterBar` (proportion meter).
- `e2e/wonder.spec.mjs` — WONDER flourish regression suite (45 checks) with a **perceptibility guard** that fails if the page-enter rise / hover-lift fall below a visible threshold (guards against "tasteful" tweaks silently making flourishes invisible).
- GLAMOR Quick-wins QW-1..10 (CSS-only): card/stat border-radius, card-title weight, semantic stat-value colors (!important), 8px share bars, tabular budget amounts, section-divider rhythm, card gap, mermaid font alignment, period-caption promotion, ghost-small export buttons
- GLAMOR GX1 shell fixes (F1â€“F9): light-mode topbar/rail/active-chip/icon-buttons/+Add-menu surfaces, household-card surface, 768px topbar+rail collapse, `.breadcrumb` class on the topbar nav (F8), backdrop pointer-events guard (F9)
- Widget Builder publish path completed: `vbCardPrefix` + `vbPublishedWidget` render published cardgraph tiles on the dashboard (was an unbuildable stub)
- C74 Tier 3: friendly message for scanned/encrypted PDFs directing user to "Extract with AI" or image import
- C74: "Extract with AI" button on statement card â€” sends pasted text to LLM, parses result into draft rows (same pipeline as image import)
- C74: "Suggest categories" button on draft review â€” applies deterministic rules first (free/local), then optional AI for uncategorized rows
- C74 e2e: `e2e/c74_ai_extract_check.mjs` asserts Extract-with-AI + Suggest-categories buttons render and are operable

### Changed
- **WONDER (W-10):** route cross-fade via the View Transitions API (progressive enhancement, fail-safe to the W-9 page-enter, reduced-motion safe).
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **WONDER (W-9, W-21):** page-enter transition on route change + scroll-reveal for long pages (IntersectionObserver, fail-safe visible, reduced-motion safe).
- **WONDER theme integration:** `prefs.Motion` (off/subtle/full) drives `data-wonder` on `<html>` via `ApplyPrefs`; a Motion segment control in appearance settings (Off / Subtle / Full) persists the choice and live-updates flourish intensity; OS prefers-reduced-motion remains a hard CSS override regardless.
- **WONDER (W-11..W-20):** list stagger, bento entrance, modal backdrop blur, toast spring, progress ease, skeleton shimmer, focus-ring ease â€” token-driven + reduced-motion safe.
- **WONDER (W-3..W-8):** tile/row/nav/icon hover flourishes, primary-button click ripple, switch spring â€” all token-driven + reduced-motion safe. W-3 adds bento `.w` tile hover lift (excluded during drag); W-4 adds list `.row` 2px translateX nudge on hover (table rows excluded for column alignment). W-5..W-8 previously landed.
- **C73 Phase 2 COMPLETE â€” every screen card ported to primitives (2026-06-23).** Drove the raw-scaffold count
  **165 â†’ 38**, eliminating **all** bespoke `Section(css.Class("card"))` markup from `internal/screens` (now zero):
  every card on every screen renders through `Card`/`EntityListSection`, byte-identically. The primitives gained
  `TestID`, `Header` (verbatim bespoke header â€” H3 titles, flex headers, `.card-head`/`.budget-head`), `Rows`
  (wraps the canonical `Div(.rows)`), `ClassParts` (extra classes merged into one `css.Class` â€” fixed a latent bug
  where a second `css.Class` prop silently dropped the base class), and `HeaderAction` now emits the real `.card-head`
  class. The remaining 38 `Div(.rows)` are list-row containers inside ported cards. The Phase-5 ratchet now
  hard-asserts `Section(.card) == 0` and one-way-caps the row containers. **C73 epic fully complete.**
- **Accessibility (GX4):** visible focus rings on inputs/selects (removed `outline:none` from `:focus` overrides â€” `:focus-visible` ring now displays correctly for keyboard users), larger sort tap-targets (`.th-sort` meets WCAG 2.5.8 24px minimum), heading-hierarchy fix (widget titles are now H2, not H3, so H1â†’H2 is correct on dashboard and all bento screens), and resize-handle keyboard ring (`.rz:focus-visible` no longer suppresses `outline`).
- **Component primitives (GX3):** selects now match inputs; consistent button/input/badge states. Unified `--btn-py`/`--btn-px` tokens bring `.btn` and `.set-btn` to a shared 44px touch target. Select elements styled to match `.field` (eliminates white-box-in-dark glitch). DataTable light-mode pinned. Modal Save button now reads as primary. Pace badges get consistent 1px border ring.
- **C73 Phase 3 â€” big-row extraction (2026-06-23).** `AccountRow`, `BudgetRow`, and `GoalRow` each moved out of
  their screen file into a self-contained `*_row.go` (accounts_row.go / budgets_row.go / goals_row.go), matching
  the earlier `transactions_row.go` split. Each row keeps its display + inline-edit/set-balance/transfer/reconcile/
  contribute sub-forms and owns its own hooks (per-row component rule). Pure relocation â€” behavior byte-identical;
  all per-screen e2e gates green. Also fixed an undefined-variable typo in the `budget_topup` gate's success log.

### Added
- **C73 Phase 5 â€” component inventory + scaffold ratchet (2026-06-23).** `docs/COMPONENTS.md` documents every
  `internal/ui` primitive with a one-line usage + a porting guide mapping each legacy idiom to its primitive.
  `internal/screenlint/scaffold_baseline_test.go` is a native (host-runnable) ratchet that counts raw
  `Section(.card)`/`Div(.rows)` scaffolds in `internal/screens` and fails if they exceed the 2026-06-23 baseline
  of 165 â€” a one-way ratchet that blocks new bespoke markup and only ever ratchets down as screens are ported.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **C73 super-screen decomposition + primitive completion (2026-06-23).** Behavior-preserving refactor wave:
  - **3 new primitives** â€” `DeleteButton` (the 18Ã— `.btn-del` pattern, loop-safe), `ExportButton` (wraps the
    14Ã— `downloadBytes` flow), and `EntityListSection` (card + title + empty-state/body scaffold).
  - **All five super-screens decomposed** into single-purpose, hook-free sub-components that receive state as
    props (hooks stay in the parent shell): **Planning** â†’ `planning_{forecast,afford,runway,recurring,plans,debt}.go`;
    **Documents** â†’ `documents_{image_import,draft_review,spend_summary,csv_import,import_history}.go`;
    **Allocate** â†’ `allocate_{profile_config,weight_editor,suggestion_list,ai_explain}.go`;
    **Customize** â†’ `FormulaCalculator()` + `CustomFieldsManager()` (customize_formula.go);
    **Settings** â†’ `settingsLeftColumn`/`settingsRightColumn` (settings_section.go).
  - **TransactionRow** extracted to `transactions_row.go` (Display+Edit); **Categories/Rules/To-do** add + inline-edit
    forms migrated to `SelectInput`/`FormField` (orphaned `UseEvent` hooks converted to anonymous calls to keep
    hook ordering stable). Planning forms migrated to `FormField`/`OptionsFrom`/`StatGrid`.
  - Stale e2e gates refreshed for the +Add modal / TreeRows markup (category_parent_delete, categories_labels,
    allocate_determinism). `go test ./...` green, wasm builds clean, touched-screen gates pass.

### Added
- **C73/C74/C78 epic build-out (2026-06-23).**
  - **C73:** migrated Accounts/Budgets/Goals selects to the `SelectInput` primitive + consolidated the
    category-tree indent helpers (`IndentPx`/`IndentLabel`) across Categories.
  - **C74:** **XLSX, DOCX, and text-PDF parsers** (pure Go, zip-bomb-guarded) routed through
    `statement.ParseAny`; a **Map-columns import wizard** + saved profiles + per-bank cadence reminder
    on Documents.
  - **C78:** **SQLite `audit_log` persistence** + Phase-2 commit seam (replay-guarded), an inline **Undo**
    button on the Toast, and a per-entity filter on the **Activity** timeline (loaded into the feed at boot).

### Added
- **C-series epic + tail closeout (2026-06-23).** Built the genuinely-open remainder:
  - **C56** subscription correction path â€” `SubscriptionIgnore` entity (SQLite-persisted) + "Not a
    subscription" / Undo UI.
  - **C60/C65** Documents CSV file-picker + Workflows condition variable-reference (click-to-insert).
  - **C73** the missing reusable UI primitives â€” `SelectInput`/`OptionsFrom`, `OverflowMenu`,
    `InlineEditForm`, `TreeRows` (50+ tests).
  - **C74** statement-import Tier 2 â€” OFX/QFX (1.x SGML + 2.x XML) parser, import-map profiles, keyword
    categorizer, and `statement.ParseAny` auto-detect, wired into the Documents paste/import path.
  - **C78** audit log â€” `internal/auditlog` + a new **Activity** timeline screen (`/activity`) fed by the
    undo capture, with inline undo.
  - Narrow-screen row-action buttons collapse to icon-only (C49â€“C65 responsive bullets).

### Added
- **C74 Tier 2/3 â€” OFX parser, import-mapping profiles, categorizer, ParseAny (2026-06-23).** `internal/ofx`: pure-Go OFX 1.x SGML + OFX 2.x XML parser â†’ signed minor-unit rows; handles `[tz]` annotations, date format variants, and both bank/credit-card message sets. `internal/importmap`: `Profile` struct for saveable column-mapping + `Apply` (CSV rows â†’ `statement.Row`) + `DefaultProfile` (auto-detect from header names). `internal/statement`: `Category string` field added to `Row`; `Categorizer` interface + `Categorize` helper + `DefaultCategorizer` keyword-table; `ParseAny` format-sniffing dispatcher (OFX vs CSV/TSV, BOM-safe). All three packages: `go vet` clean, 20 tests passing.

- **C-series 6-lane sweep (2026-06-23).** Audited the C backlog (mostly already-shipped via the L-series
  work) and closed the remaining gaps: on-panel rail collapse toggle (C20); dashboard band-span + figure
  type tokens (C48); Accounts inline-edit advanced disclosure (C49); Allocate amount-field labels + AI
  "needs key â†’ Open Settings" link (C54); To-do long-notes truncation (C52); Workflows inline edit +
  H2 headings + labels (C65); Documents "needs key â†’ Settings" link + loading state (C60); Customize
  click-to-insert variable (C61); Artifacts storage-meter bar + CSV preview (C66); Split no-members CTA +
  ToggleRow alignment (C58). The dashboard bugs (C1 income, C14/C22 resize), money formatting (C2),
  empty-gear panel (C11), and many screen-review items were confirmed already-fixed with file:line proof.

### Added
- **L-series 6-lane sweep, round 3 â€” closing the long tail (2026-06-23).** Documents CSV-import account
  selector + above-fold Import button (L44); Reports category-row drill-through to Transactions (L58);
  budget "Top up" for under-limit budgets (L43) + rollover label fix (L40); definitive load-splash
  dismissal + label-wrap CSS (L2/L11/L37/L41); hide the currency control for single-currency households
  (L37) + goal-add progressive disclosure (L38) + clearer over-funded "Funded 120% â€” $X over" (L59);
  a shared e2e `ready()` helper + forms-a11y gate (L12/L7). Confirmed many items already shipped.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **GLAMOR series, GX17 loading states (2026-06-23).** **Loading & skeleton states (GX17):** thinking-bubble now visible in light mode (`.insights-thinking` CSS class + `[data-theme="light"]` rule), boot-ring-arc stroke pinned to mid-grey in light mode (GX17-F6), and all three Allocate AI i18n keys (`allocate.explainAINarrative`, `allocate.explainAI`, `allocate.thinking`) verified present. GX17-F2/F3/F5 deferred (GO-STRUCTURAL, build-gated).
- **Systemic light-mode tokens (GX14):** CSS `!important` surface/text pins verified live across all surfaces; theme-engine root fix deferred (build-gated).
- **GLAMOR series, GX16 charts in light mode (2026-06-23).** **Charts in light mode (GX16):** D3 axis text, Reports hero secondary stats, Mermaid/Sankey node text, and `.reports-hero` card now readable in light mode.
- **GLAMOR series, GX13 budgets re-check (2026-06-23).** **Budgets re-check (GX13):** light-mode stat tiles, bar tracks, category-name .budget-drill contrast, row separators.
- **GLAMOR series, GX12 transactions re-check (2026-06-23).** **Transactions re-check (GX12):** uniform row height, stronger zebra, narrower actions column.
- **GLAMOR series, GX11 dashboard re-check (2026-06-23).** **Dashboard re-check (GX11):** alert-chip + widget-card surfaces, light-mode widget titles, padding.
- **GLAMOR series, GX8 motion (2026-06-23).** **Motion (GX8):** reduced-motion guards on all interactive transitions, unified hover timing, light hover bg, draggable :active feedback.
- **GLAMOR series, GX9 onboarding/splash (2026-06-23).** **Onboarding/splash (GX9):** light-mode boot splash + synchronous theme-on-first-paint, splash card surface, sample-banner action spacing (full onboarding flow F1 deferred as a feature build).
- **GLAMOR series, GX10 app lock (2026-06-23).** **App lock (GX10):** dialog role/aria-modal + aria-label, 44px unlock button, progressive forgot-passcode, aria-live error, light-mode card surface.
- **GLAMOR series, GX6 iconography (2026-06-23).** **Iconography (GX6):** replaced stray Unicode glyphs (grip/carets/alert-triangle) with the SVG icon registry.
- **Empty states (GX2):** Transactions/Accounts/Insights now show a proper EmptyStateCTA with icon + action instead of a bare line.
- **App shell (GX1):** light-mode topbar/rail-active-chip/+Add button now switch correctly.
- **GLAMOR series, GX7 responsive (2026-06-23).** **Responsive (GX7):** ultra-wide content max-width guard + compact narrow-width topbar.
- **GLAMOR series, GX5 toasts & notices (2026-06-23).** **Toasts & notices (GX5):** per-type toast icons, readable light-mode toast surface, elevated notifications card.
- **GLAMOR series, G9.1 Reports redesign (2026-06-23).** **Reports redesign (G9.1):** hero zone (Net at 2.5rem/800, Income+Spend flanking at 1.75rem/700, period caption promoted, secondary row for savings rate/runway/no-spend days), card-title weight (font-weight: 600 app-wide), heads-up alert strip (.card-alert with danger left-border + tint), tabular amounts (font-variant-numeric + strong color on .budget-amount), Sankey moved up (category â†’ Sankey â†’ top payees â†’ biggest expenses), advanced collapse (custom field spend + deductible totals behind "Advanced â–¾/â–²" disclosure, collapsed by default). **Reports charts (G9.1a):** ranked category/payee/expense bars + spending & income donuts from existing aggregates.
- **GLAMOR series, second wave (G7+).** **Planning (G7):** the forecast card now leads with a
  display-weight headline stat (projected 12-month net worth + avg monthly net), and the forecast
  chart's X-axis shows real calendar months ("Jul 2026") instead of opaque indices. Series-wide:
  card titles pinned to a full-contrast token so they stay readable in light mode. **Allocate (G8):**
  weight tuning + save-profile collapse behind an "Advanced" disclosure so the typical flow is
  profile â†’ amount â†’ list; Mode/Profile selects get persistent labels; each candidate row shows a
  `#1/#2/#3` rank badge; emergency-buffer/cap placeholders shortened so they don't clip at 768px.
  **Reports (G9):** added Spending / Income / Trends **section dividers** so the long card scroll is
  navigable. **Definitive light-mode fix** for the whole series â€” legacy text classes
  (`.card-title`/`.row-desc`/`.muted`/etc.) now pin their color directly under `[data-theme="light"]`,
  so they no longer keep the dark theme's near-white `--text` (emitted as a runtime var by the theme
  engine) on white cards. **Subscriptions (G10):** fixed the critical bug where subscription names
  were squeezed to invisible at 1280/1440 (the action buttons claimed the whole row) â€” the name now
  keeps a reserved width and the actions sit in a fixed trailing group with a compact ghost-danger
  Cancel button. G10 follow-up (2026-06-23): each subscription row now shows a proportional share-bar
  (width = MonthlyAmount / MonthlyTotal Ã— 100%) inside `.row-main`, using `var(--accent)` fill on a
  `var(--border)` track â€” the same pattern as Reports category rows â€” so the cost distribution is
  scannable at a glance without mentally computing ratios from dollar figures. **Bills (G11):** dollar amounts now render full-contrast in light mode, and the
  "Next due" stat date no longer hyphenates across two lines at 768px.
  **Bills (G11) follow-up:** horizon filter (90 days default) + Show-all toggle, action-button group fix, two-column layout at â‰¥1024 px. **Split (G12):** the
  destructive "Clear" member button is now a ghost-danger style, visually distinct from "Select all".
  **Insights (G13):** the no-key "Settings" CTA is now a primary button, the New-chat/Edit-prompt
  pills stay outlined in light mode, and the starter chips get a softer dedicated pill style.
  **Documents (G14):** parsed-statement review rows now render *below* the statement card (not above
  it), and the neutral draft-import amounts are readable in light mode (scoped so the green/red
  semantic amount colors are preserved). **Customize (G15):** the formula calculator now leads the
  page (above the custom-fields manager, behind a section divider) instead of being buried below the
  fold, and the variable-reference amounts are readable in light mode. **Members (G16):** the
  net-worth-by-member amounts are now readable in light mode (they were inheriting the dark `--text`).
  **Categories (G17):** added an in-card "+ Add category" button to both the Expense and Income kind
  cards (add was previously only reachable from the command palette). **Rules (G18):** "Your rules"
  now leads the page (precedence chain next, the 15-row suggestions card last) instead of being
  pushed below the fold, plus an in-card "+ Add rule" button. **Rules (GI1):** suggested rules
  collapsed to 5 by default with a "Show all (N)" / "Show fewer" toggle; suggestion Add buttons
  demoted from `.btn-primary` to `.btn` (secondary); inline-edit Save/Cancel size to content
  (`.fit` class); drag-reorder hint "Drag â ¿ to reorder â€” first match wins" shown under the heading
  when â‰¥2 rules exist. **Categories (GI2):** Category map card moved first (above the list cards)
  so it's visible on arrival; usage count styled as `.btn-link` (accent color + underline + pointer)
  so "26 transactions" reads as a drill-through; sub-category rows get `.cat-child-row` background
  fill (dual-theme: `rgba(255,255,255,0.02)` dark / `rgba(0,0,0,0.02)` light) so nesting is
  visible at 768px; zero-usage rows get `.cat-zero-usage` (opacity 0.55) so safe-to-delete
  categories are spottable; sort-by-usage toggle (`sortByUsage` state) in the Expense card header
  flattens and sorts by descending transaction count for cleanup audits. **Workflows (G19):** primary buttons
  ("Run now" etc.) now use white text in light mode â€” the previous dark-green-on-green failed AA
  (~2.1:1); this fix applies app-wide. **Artifacts (G20):** row meta reordered (ref status leads),
  "Referenced by N" shown in green vs neutral-muted "not referenced", upload date added per row,
  `.notice`/`.notice-warn`/`.storage-bar`/`.csv-preview`/`.ref-positive` CSS rules added (were
  used in Go but missing from the stylesheet â€” quota nudge and storage bar were invisible).
  **Settings (G21):** two CRITICALs fixed â€” toggle-row labels were white-on-white in light mode (`[data-theme="light"] .toggle-row span { color:#1c1c1e }`) and the flip-backdrop was a dark overlay in light mode (now warm-white `rgba(239,237,232,0.75)`); panel height raised from fixed 560px to `min(90vh,900px)`; right column reordered to Appearance â†’ Preferences â†’ AI â†’ Cloud â†’ Data â†’ Advanced (usage-frequency); "Importâ€¦" dataset button renamed to "Import datasetâ€¦" (L47); AI key password input gets explicit `aria-label`; Save now fires a "Settings saved" toast via `PostNotice`; danger button moved from inline `Style()` to `.data-btn-danger` CSS class (`var(--danger)`/`var(--danger-muted)`); two-column grid collapses to single column at â‰¤768px.
  **Custom pages (G22):** three CRITICALs resolved â€” (1) newly created page now appears in MY PAGES rail immediately after creation (`bump()` added before `nav.Navigate` in `custompagesnav.go`, closing C32 gap #67); (2) custom widget tile titles are readable in light mode (`[data-theme="light"] .wh h2, .wh h3 { color:#1c1c1e }`); (3) content-area background no longer bleeds dark in light mode (`.bento` and `main > div` get `background-color:var(--bg)`). Also: KPI body shows a friendly muted placeholder instead of raw "widgetspec: no formula set" error string; widget tile heading corrected H3â†’H2 (fixes H1â†’H3 skip); resize buttons gain `aria-label`; drag grip gains `aria-label + role="button"`; add-widget form type select and title input gain `aria-label`; `.empty` text contrast improved in light mode (~3.5:1â†’~5.4:1 WCAG AA). **Design-system (G23):** light-mode shell/nav background now switches (no more dark bands between cards); muted text bumped to AA-safe contrast; verified the foreground + primary-button light pins from earlier waves.
  **Workflows (GI3):** collapsible Mermaid diagrams (collapsed by default, per-row "Show diagram" / "Hide diagram" toggle); condition input moved to its own full-width row (`.field-wide`); "Dry run" promoted to `.btn-primary` and "Run now" demoted to plain `.btn` (simulation-first hierarchy); `aria-label="Action type"` added to action-kind select; card titles already H2; condition variable hint + click-to-insert pills confirmed present. Deferred: full inline-Edit for existing workflows (C65).
  **Settings modal (GM1):** Three targeted fixes from the GM1 deep-dive audit. (1) 768px single-column collapse now works â€” the old media-query targeted `.grid-cols-2` class but `tw.GridCols2` emits an inline `style` attribute; selector changed to `div[style*="grid-template-columns"]`. (2) All 22 `set-label` section headings changed from `<div>` to `<h4>` so screen readers can navigate the panel by heading hierarchy inside the dialog. (3) Password inputs for AI key and web-search key now carry dedicated `aria-label` text (the placeholder string) instead of the generic section heading.
  **Add/Edit modals (GM2):** Eight targeted fixes from the GM2 audit. (1) QuickAdd form (most-used add path): all 5 non-checkbox inputs now wrapped in `ui.FormField()` so visible labels appear above each control â€” previously all 5 were placeholder-only (WCAG failure). (2) Inline transaction edit form: Description and Amount inputs now wrapped in `labeledField()` (were the only labeled-field-free fields in any edit form). (3) Budget modal: primary CTA changed from generic "Add" to "Add budget" (entity-specific, matches Account modal). (4) Goal modal: primary CTA changed from "Add" to "Add goal"; success toast "Goal created." now fires on add (was silent). (5) FlipPanel CloseOnly footer: "Close" button class changed from `.set-btn.save` (green/primary) to `.set-btn.close` (neutral dismiss) â€” semantically correct and no longer misleads users into thinking clicking it submits the form. (6) CSS: `.set-btn.close` neutral-dismiss style added (dark + light themes). (7) CSS: `.set-body` scrollbar overrides for light mode (warm-neutral thumb, was jarring dark-grey on white). Previously landed (confirmed): modal title light contrast fix, footer button light-mode fix, Add-btn light-mode border.
  **Confirm dialogs (GM3):** Four structural fixes. (1) Bulk-delete now shows a count-aware confirm dialog before executing â€” "Delete N transactions? This can't be undone." â€” closing the L50 data-loss gap where 50+ selected transactions could be destroyed in a single click. (2) Default focus for destructive confirms flipped to Cancel (`id="cf-dialog-cancel"`) so Enter can't accidentally confirm a delete (WCAG SC 3.2.4). (3) Destructive dialogs now auto-derive a title ("Are you sure?") and the backdrop role is upgraded to `alertdialog`, which screen readers announce with urgency. (4) `aria-labelledby="cf-dialog-title"` wired between the backdrop and the `<h3 id="cf-dialog-title">`. CSS: dialog padding increased to `1.5rem/1.25rem` and `min-height:6rem` added to relieve the cramped 110px layout (D7). Scrim blur + theme-ring shadow were already landed in the prior GM3-5/6 patch.
  **Palette/gear (GM4):** Six targeted fixes from the GM4 UX audit. (1) Palette card now carries `role="dialog"` + `aria-modal="true"` + `aria-label` (GM4-1); backdrop gets `aria-label` (GM4-3). (2) `#cf-cmd-list` gets `role="listbox"`; each result row emits `aria-selected="true/false"` and `movePaletteSel` keeps it live on arrow-key navigation (GM4-2). (3) Keyboard hint footer `â†‘â†“ navigate Â· â†µ select Â· Esc close` added below the result list in `buildCommandPalette` (GM4-11). (4) Entity-jump commands capped at 8 in the unfiltered view (`entityJumpMaxUnfiltered = 8`), trimming the default list from 58 rows to a manageable scan; fuzzy filter still surfaces all entities (GM4-12). (5) `FlipPanel` close (Ã—) button given `tabindex="-1"` so initial focus lands on the first form control, not the dismiss button (GM4-17). (6) Backdrop click-to-close wired in `FlipPanel.UseEffect` via a `document` click listener that checks `event.target == .flip-backdrop` (GM4-19). Also fixed a latent `movePaletteSel` bug where `i == cmdPaletteSel` was compared against DOM child index rather than a row-only counter, causing the wrong row to highlight when group-header divs were present.

### Added
- **L-series 6-lane parallel sweep, round 2 (2026-06-23).**
  - **Receipts â†’ IndexedDB (L29):** artifact image bytes now live in IndexedDB (`internal/artifactstore`)
    with lightweight refs in the dataset, a render-safe cached usage meter, a quota nudge, and
    self-contained export/import; graceful localStorage fallback. The prior render-path deadlock is fixed.
  - **Responsive/mobile (L11/L32/L33/L36):** a mobile bottom tab-bar (`MobileTabBar`), 44px tap targets,
    condensed period controls, touch-chrome hiding on the bento, and reliable splash dismissal â€” gated by
    a 390Ã—844 Playwright viewport check.
  - **Tax-deductible reporting (L16/L58):** a `Category.Deductible` flag (with a category-form checkbox)
    and a Reports "Deductible totals" section + CSV, backed by pure `reports.DeductibleTotals`.
  - **Income â†’ Allocate (L10):** an "Allocate this month's income" nudge that pre-fills the amount from the
    period's net income; custom-field values now feed the Insights Q&A context (L18).
- **Fixed the long-failing `internal/icon` curated-set test** (`Paperclip` was missing from the curated
  list); added `docs/TESTING.md`. **`go test ./...` is now fully green.**

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **To-do screen glamor pass (G6).** Added a page-level **"+ Add task"** button in the card header,
  a compact **"N open Â· N overdue Â· N done"** summary strip above the list (matching the stat strip
  every other list screen opens with), and pinned the primary row label to a full-contrast token so
  task titles stay readable in light mode. Long entity chips no longer collide with the action
  buttons at narrow widths (shared row reflow).
- **Dashboard screen glamor pass (G1).** The sample-data banner is now properly styled (its classes
  were unstyled, so "Start fresh" and "Dismiss" ran together as "Start freshDismiss"); the Income
  KPI tile shows a signed **cash-flow** sub-line (income âˆ’ spending), so Elena's "what changed?" is
  answerable above the fold; and the net-worth tile's trend sub-line shows the **absolute delta**
  and reads "No change this month" at a true zero instead of a misleading "â–² 0%".
- **Transactions screen glamor pass (G2).** The ledger is far denser and faster to scan: the
  **Actions column collapses to an icon strip** (was ~44% of table width), **Amount moves to
  column 3** (Date â†’ Amount â†’ Description), rows get **zebra striping + a themed hover band**,
  cleared state shows a **green âœ“ / dim â—‹ toggle** (cleared rows dim) instead of a "Mark cleared"
  button in every cell, the **Tags column hides when empty** (tagged rows show an inline #chip),
  the **"Select all" button moves onto the summary line**, and tighter row padding + a single-line
  ellipsised Description lift the visible row count.
- **Accounts screen glamor pass (G3).** The summary now leads with a **dominant net-worth hero
  tile** (larger figure, full-height) beside smaller asset/liability tiles, with a **month-to-date
  trend subtitle** (â†‘/â†“ signed delta from two net-worth snapshots). Accounts within each group
  sort by **balance, largest first**; each row gains an **account-type glyph**; stale accounts get
  an inline amber **"Update balance"** action and the "Mark all updated" button is restyled to tie
  it to the STALE badge; list rows reflow cleanly below 760px so long names no longer collide with
  the action buttons.
- **Budgets screen glamor pass (G4).** Budget rows now sort **health-first** â€” Over â†’ Near/At-risk â†’
  On track, then percent-used descending â€” so the budgets that need action rise to the top. The
  over/near **summary pills** (previously unstyled bare text) get a proper filled-chip treatment;
  the empty progress-bar **track** gains a hairline border so a 0% bar still reads; added a
  discoverable **"+ Add budget"** header button; and the row sub-line is split into a primary
  statusÂ·remaining line over a dimmed periodÂ·%-used line.
- **Goals screen glamor pass (G5).** Active goals now sort by the most actionable first â€”
  nearest target date, then highest percent complete, then name (`goals.LessForList`, pure +
  table-tested) â€” so a near-complete or time-pressed goal surfaces at the top. Each row gains a
  compact **pace badge** (Final stretch / Past due / Due soon / On track) and the progress-bar fill
  takes a matching tone (`goals.ClassifyPace`, pure + tested) instead of one flat accent. Added a
  discoverable **"+ Add goal"** button in the card header (`.card-head`), a 768px row-wrap so long
  goal names no longer collide with the amount, and explicit full-contrast tokens on stat figures +
  goal names for light mode.

### Added
- **L-series 6-lane parallel sweep (2026-06-22).**
  - **Transactions:** single-row delete now asks for confirmation (L36); a "Mark as reviewed" checkbox
    on quick-add suppresses the auto `needs-review` tag on confident entry (L43, new `Transaction.Reviewed`).
  - **Accounts:** a dedicated **Transfer** action (`appstate.CreateTransferPair`); reconcile **Update balance**
    now previews the computed delta and lets you categorize the adjustment (L57/L30); Save-button form id (L44).
  - **Goals:** optional **ledger posting** on contribute (debit the linked account, `appstate.ContributeToGoal`);
    completion prompt + archive on reaching 100% (L59).
  - **Onboarding/data:** wipeâ†’reload no longer re-seeds the sample (verified, L6); seeded members now carry
    attributed spend so per-member Reports/Split demo out of the box (L16); 600-row + malformed CSV import
    resilience test (L23).
  - **Reports/period:** the period **window** (not just resolution) persists to localStorage (L45/L58);
    CSV exports get **period-stamped filenames** (`reports.ExportFilename`); a **Prior year** Jump-To preset.
  - **Navigation/AI:** the âŒ˜K palette groups results (Navigate/Actions/Workspaces) with hints (L14);
    Alt+1â€“9 rail hints (L34); a Dashboard **Transfer** shortcut; a deterministic **mock AI provider** for
    testing the askâ†’answer flow without a key (L8).

### Added
- **Dashboard/To-do/Documents/Artifacts polish (L-series lane D).** Custom pages gain a **Bills** list
  source (`widgetdata` SourceBills, table-tested); the To-do screen gains a **priority filter**; the
  Documents importer shows an **image preview** of the picked receipt; Artifacts get **inline rename**,
  **section headings**, and a **"used by N pages" delete guard** so an artifact referenced by a custom
  page can't be silently removed. 4 new e2e gates; existing `todo_nesting` updated for the +Add modal.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **"+ Add" modal-in-place completed for all entities (C73/C79, C72A).** To-do task, Category, Member,
  and Rule add forms now open in the +Add FlipPanel modal too, and the inline add card was removed from
  every screen (including Transactions, which uses quick-add) so each page leads with its content. New
  reusable `TaskAddForm`/`CategoryAddForm`/`MemberAddForm`/`RuleAddForm`; menu items + i18n added. Quick-add
  now applies auto-categorization rules on save (restoring the inline form's behaviour now that it is the
  sole manual add path). Gate: `e2e/add_modal_entities_check.mjs`; ~23 add-form e2es updated to open the modal.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **"+ Add" opens entity modals in place (C73/C79, foundation + Goal/Account/Budget).** The top-bar
  "+ Add" menu now opens Goal, Account, and Budget add forms in a centered **FlipPanel modal** instead
  of navigating to their screens, and those screens no longer carry an inline add card (they lead with
  their content). New `uistate.UseAddTarget`/`SetAddTarget` atom + `app.AddHost` (mounted at the shell
  root) drive the modal; each form is a reusable component (`GoalAddForm`/`AccountAddForm`/
  `BudgetAddForm`) that keeps its own submit + validation so an invalid submit keeps the modal open and
  a valid one closes it (working around FlipPanel's unconditional close-on-save). `EmptyStateCTA` gained
  an `AddTarget` so an empty list's CTA opens the modal. Gate: `e2e/add_modal_check.mjs`.

### Added
- **Quick-hit UX polish (L audits).** Budgets show the over/near summary as tone'd badges; the Reports
  screen now states the covered date range ("Covering â€¦ compared with â€¦"); Rules' match/category/tags
  controls and the Members name + reassign-target select gained accessible labels, and opening the
  member reassign panel moves focus to its select.

### Fixed
- **`+ Add` menu opened over the left rail, leaving its items half-unclickable.** The single +Add button sits
  ~24px right of the rail, and `.add-menu { right:0 }` made its 210px panel extend leftward back over the
  sidebar — the items' clickable centres fell inside the rail, which intercepted the clicks. Changed it to open
  rightward (`left:0`) into the content column. Verified in both themes (`e2e/addmenu_verify.mjs`, 8/8): items
  clear the rail (minLeft=269 ≥ railRight=240), menu fits the viewport, and "New transaction" is now clickable
  and opens the add modal. The open direction is now chosen **live at open-time** (`internal/app/addmenu.go`
  measures the button's gap to the viewport's right edge via `syscall/js`): the +Add button reflows between the
  left and right of the topbar across widths, so a fixed side or breakpoint isn't robust — it opens rightward
  by default and flips left (`.open-left`) when there's < ~224px on the right. Verified at 6 widths
  (1280/1100/1025/1024/768/390): no overflow and no rail overlap anywhere (`e2e/addmenu_widths_verify.mjs`).
- **`+ Add` menu now closes on Escape (keyboard-a11y).** It previously only dismissed via item-click or a
  backdrop click; pressing Escape left it open (`aria-expanded="true"`). Added a document `keydown` listener
  (registered only while open, mirroring `dialoghost.go`) that closes the menu on Escape and returns focus to
  the +Add button per the WAI-ARIA menu-button pattern. Verified `e2e/escape_addmenu_verify.mjs` (5/5): closes
  + refocuses, still reopens/positions/backdrop-closes.
- **`+ Add` menu now dismisses on outside-click over page content.** The `.add-backdrop` is `position:fixed`
  inside the topbar's sticky (`z-index:5`) stacking context, so it doesn't paint over the page content — clicks
  there fell through without closing the menu. Added a document `pointerdown` listener (alongside the Escape
  one, only while open) that closes the menu when the press lands outside `.add-wrap` — immune to stacking.
  Verified `e2e/addmenu_outside_verify.mjs` (4/4): opens & stays open (no self-close), outside-click over
  content closes, Escape + item flows intact.
- **Account-row `⋯` overflow menu: Escape/outside-click dismissal + `aria-expanded`.** The hand-rolled menu
  on each account row (and the shared `OverflowMenu` primitive) lacked Escape-to-close, `aria-expanded` on the
  trigger, and relied on the non-covering `.add-backdrop` for outside-clicks. Added a reusable
  `ui.DismissPopover` custom-hook (`internal/ui/dismiss.go`) wiring Escape (close + refocus trigger) and
  outside-`pointerdown` dismissal, plus `aria-expanded`; used it in `OverflowMenu` and `accounts_row.go`. Fixed
  a latent bug surfaced by verification: `UseId()` ids contain colons (`gwc:3:1`) — invalid in a `#id` selector,
  so `querySelector` threw and panicked the callback; switched to `getElementById`. Verified on `/accounts`
  (`e2e/accounts_menu_verify.mjs`, 8/8).
- **Custom-page `⋯` menu (rail rename/hide/delete): dismissal + `aria-expanded`.** This per-page menu had no
  dismissal at all (not even a backdrop — it stayed open until an item was picked) and no
  `aria-haspopup`/`aria-expanded`/`role=menu`. Wired `ui.DismissPopover` (Escape + outside-click) and added the
  ARIA roles/state in `internal/app/custompagesnav.go`. Verified on the live rail after creating a page
  (`e2e/custompage_menu_verify.mjs`, 7/7). (Completes the hand-rolled `⋯`-menu sweep — `rules.go`/`widgets.go`
  use `MoreH` only as a drag grip, not a menu.)
- **Overflow menus: WAI-ARIA arrow-key navigation.** `ui.DismissPopover` now roves focus among `[role=menuitem]`
  entries with ArrowDown/ArrowUp (wraparound) + Home/End, gated on focus being inside the popover so global
  arrow keys aren't hijacked. Completes the menu-button keyboard pattern (alongside Escape, outside-click,
  `aria-expanded`) for every consumer — accounts `⋯`, custom-page `⋯`, and the `OverflowMenu` primitive.
  Verified on the accounts menu (`e2e/menu_arrowkeys_verify.mjs`, 8/8); prior dismissal guards still green.
- **`+ Add` menu unified onto the shared dismissal helper.** It was the last dropdown running its own ~50-line
  inline Escape/outside-click effect (no arrow-key nav, duplicated logic). Migrated it to `ui.DismissPopover`
  (keeping its live open-direction logic), so now *every* app dropdown shares one helper with the full
  menu-button keyboard pattern. Verified `+ Add` arrow-keys (`e2e/addmenu_arrowkeys.mjs`, 6/6) and that its
  existing positioning/escape/outside-click guards are unchanged; net ~50 fewer lines in `addmenu.go`.
- **Widget Manager table clipped its "Order" column on phones.** The 4-column `.wm-table` (~404px min-width)
  overflowed and clipped the reorder controls at phone width (`tableRight=483 > docW=390`). Since it has no
  sticky header (unlike `.txn-table`), wrapped it at the call site (`internal/screens/widgets.go` →
  `.wm-table-wrap { overflow-x: auto }`, scoped — the shared DataTable and txn-table are untouched) so it
  scrolls sideways on phones and is unaffected on desktop. Verified at 390 (scrolls, Order reachable, no
  page clip) and 1280 (fits, no scrollbar) via `e2e/wmtable_scroll_verify.mjs`.
- **Transactions table clipped its right-hand columns at tablet widths.** The 8-column `.txn-table` has a
  ~949px intrinsic min-width, so on tablet viewports (768–900px) the wide table overflowed the content column
  and clipped Account/Tags/Cleared/Actions with no way to reach them. Raised the existing C10/C19 card-layout
  breakpoint `760 → 900px` (`web/index.html`) so tablets get the stacked-card view — which also avoids the
  sticky-header breakage a horizontal-scroll wrapper would cause (an `overflow-x:auto` wrapper turns into a
  scroll container and the sticky `th` scrolls away; measured and reverted). Verified at 768/880 (cards, no
  clip) and 1280 (table fits, sticky header intact) via `e2e/txn_responsive_verify.mjs`. (Residual 901–1185px
  laptop clip is tracked under B31 — needs a product call.)
- **Reports "Money flow" Sankey showed raw minor units (cents) as node labels.** It rendered "Income 406800",
  "Housing 217500", "Groceries 52000" because Mermaid `sankey-beta` displays the flow weight verbatim and the
  Go side passed minor units. Mermaid requires a numeric weight (can't take a formatted money string), so:
  `internal/screens/reports_screen.go` now rounds each flow minor→major (whole currency units via
  `currency.Decimals`), and `web/mermaid.js` adds `sankey: { prefix: "$", showValues: true }`. Labels now read
  "Income $4068 / Housing $2175 / Groceries $520 …", matching the hero + ranked-bar figures (verified via
  rendered SVG text in both themes, `e2e/sankey_verify.mjs`). The currency prefix is now **per-render and
  base-currency-aware** (a new `ui.MermaidProps.ValuePrefix` carrying `currency.Symbol(base)`), so a GBP/JPY
  household gets "£"/"¥" instead of a hardcoded "$"; JPY's 0-decimal currency correctly skips the minor→major
  division. (Supersedes the initial hardcoded `prefix:"$"` in the Mermaid init.)
- **Drag affordances clobbered by WONDER entrance animations (filled-animation sweep).** Two functional drag
  cues measured `opacity: 1` instead of their intended dim: the dashboard tile drag-**ghost** (`.w.drag`,
  should be `.35`, clobbered by `wonder-bento-enter`) and the rule-row drag-**grab** (`.row[draggable]:active`,
  should be `.85`, clobbered by `wonder-row-enter`). A filled animation (`fill-mode: both`) outranks every
  non-`!important` author rule, and its `opacity: 1` end-state had been silently overriding both. Fixed with
  `opacity: … !important` (unconditional — drag cues must show regardless of the WONDER setting). Re-measured
  0.35 / 0.85; guarded by `e2e/drag_affordance_verify.mjs` (2/2) + a `.w.drag` ghost check folded into
  `e2e/wonder.spec.mjs` (46→47). Audited the remaining filled animations (page-enter, toast-in,
  chart-draw/fade, success-pulse) — none land on elements with a competing hover/static transform/opacity.
- **W-3 bento tile hover-lift was silently broken (filled-animation clobber).** Dashboard `.bento .w` tiles
  measured `translateY: 0` on hover — the `wonder-bento-enter` entrance animation (`fill-mode: both`, final
  keyframe `transform: none`) outranked the non-`!important` `html .w:not(.drag):hover` transform (same cascade
  trap as the W-4 row-hover and GI2 zero-usage fixes). Split the tile hover into its own rule with `transform:
  … !important` (box-shadow stays non-important; `:not(.drag)` kept so a tile being dragged is never touched).
  Now: −5px lift at full, identity when off/reduced-motion, drag-excluded. Added a permanent **W-3 tile-hover
  guard to `e2e/wonder.spec.mjs`** (the suite had no tile-hover coverage — 45→46 checks, all pass).
- **GI2 zero-usage category dim was silently broken (WONDER-over-GI2 regression).** `.cat-zero-usage`
  rows never dimmed (computed `opacity: 1`, not 0.55) because they carry the `wonder-row-enter` entrance
  animation (`fill-mode: both`, final keyframe `opacity: 1`), and a filled animation's value outranks every
  non-`!important` author rule — same class of bug as the W-4 row-hover fix. Changed to `opacity: .55
  !important` in `web/index.html`. Caught by the GI1/GI2/GI3 both-theme verification pass
  (`e2e/gi123_theme_verify.mjs`, now 18/18); fix re-verified at 0.55 in both themes.
- **WONDER amplification fixes actually landed in `main` (W-1/W-2/W-4/W-11).** The earlier "lift 5px /
  hover + row + off-suppression" fixes were made in a worktree that was removed before committing, so
  `main` still shipped the imperceptible values and `e2e/wonder.spec.mjs` ran 40/45. Re-derived and
  committed them as a late **WONDER override-hardening block** in `web/index.html`: `--wonder-lift` 2px→5px
  (hover lift now ≥4px perceptibility), and the hover/press transforms re-asserted after the base
  component rules that were silently clobbering them. Root cause newly diagnosed: list rows carry the
  `wonder-row-enter` entrance animation (`fill-mode: both`, final keyframe `transform: none`), and a
  filled animation's value outranks every non-`!important` author rule regardless of specificity — so the
  W-4 row-hover nudge needs `!important` (still off-safe; it scales by `--wonder-on`→0). Suite now **45/45**.
- **W-10 route cross-fade deliberately NOT shipped.** The stranded `8654d27` branch's View-Transitions-API
  cross-fade was recovered, fixed (its `defer cb.Release()` freed the `js.Func` before `startViewTransition`
  invoked it asynchronously — a use-after-release crash on every Chrome navigation), and verified — then
  rejected: it *suppresses* the richer W-9 fade-rise (replacing translateY+opacity with a plain opacity
  cross-fade on snapshot pseudo-elements) and regressed the two W-9 sweep checks. Net downgrade; clean
  main's W-9 transitions already pass.
- **Customize no longer duplicates a formula on loadâ†’save (L #43).** Editing a saved formula now updates
  it in place (the editor tracks the loaded id) instead of minting a new id on every Save. Gate:
  `e2e/formula_save_inplace_check.mjs`.

### Added
- **Guided empty states on derived & planning screens (L-quickhits).** Bills, Subscriptions, and the
  Reports breakdown now render a friendly `EmptyStateCTA` that routes the user to where the data is
  created (Accounts / Transactions) instead of a bare "nothing here" line; Planning's Recurring and
  Plans empties now jump focus to their add form. `EmptyStateCTA` gained an optional `Href` for
  route-based guidance on screens that have no on-page add form.

### Added
- **Per-member "my money" view (L21).** A top-bar member switcher (Everyone + each household member, shown
  when â‰¥2 members) backed by `uistate.UseActiveMember()` â€” a persisted atom (`cashflux:active-member`) â€”
  scopes the Transactions ledger and Dashboard KPIs/widgets to one person or Everyone. Net worth stays
  household-wide (it's account-based, not per-transaction). The by-member Reports section now shows for any
  â‰¥2-member household with â‰¥1 attributed spend. New `MemberSwitcher` component; `member_view_toggle_check.mjs`.
- **Guided statement reconciliation (L30).** A new "Reconcile to statement" mode (per-account â‹¯ menu)
  lets you enter the statement ending balance, tick off cleared transactions, and watch the live
  difference close â€” when cleared-balance equals the statement, a "Reconciled âœ“" confirmation appears and
  no balance adjustment is posted (unlike the force-to-target "Update balance" flow). Backed by the new
  pure, table-tested `internal/reconcile.Diff`; reuses `ledger.ClearedBalance` and the existing cleared
  flag. New `e2e/reconcile_statement_check.mjs`.

### Accessibility
- **Roving tabindex on radiogroups + a committed a11y gate (L7).** `Segmented` options and color swatches
  now expose exactly one Tab stop per group (the selected option, `tabindex=0`; the rest `tabindex=-1`),
  with Arrow keys moving DOM focus between them â€” the standard ARIA radio pattern. Text inputs (`.field`)
  regained a visible keyboard focus ring (`:focus-visible` outline; the prior `:focus` rule stripped it).
  New `e2e/a11y_check.mjs` sweeps `/transactions` + `/accounts` for landmarks, accessible names, labeled
  fields, a visible focus indicator, and the one-tab-stop radiogroup invariant.

### Fixed
- **Deleting a parent category no longer orphans its sub-categories (L28).** Removing a parent now re-homes
  each child onto the parent's own parent (the grandparent, or the root for a top-level category) before
  deleting, instead of leaving children with a dangling `parentId` that pointed at a category that no longer
  exists. New pure `categorytree.ReparentOnDelete` (table-tested); e2e `category_parent_delete_check.mjs`.

### Added
- **Collapsible category tree (L28).** Each parent category in the Categories list now has a chevron toggle to
  collapse or expand its sub-categories, so a deep tree stays scannable. Pure `categorytree.VisibleUnderCollapsed`
  (table-tested, cycle-safe); collapse state is session-scoped. e2e `category_collapse_check.mjs`.
- **Fill-to-target allocation mode (L17).** Allocate gains a "Fill to target" mode alongside the score-weighted
  one: it funds each destination up to its remaining-to-target in ranked priority order (give every envelope
  its due first), then spreads any leftover by score â€” zero-based budgeting's "fund the essentials, then
  optimize." New pure `allocate.DistributeFillToTarget` (sum-to-the-cent invariant, table-tested); goals
  contribute their target shortfall. e2e `allocate_fill_to_target_check.mjs`.
- **"What next" prompt when a goal is funded (L20).** A completed goal's row now shows a calm one-line prompt
  with a "Reallocate" action that jumps to Allocate, so the money you were putting toward it can be redirected
  to another goal instead of quietly sitting idle. e2e `goal_whatnext_check.mjs`.
- **Reports roll-up by parent category (L28).** The Spending-by-category breakdown now has a "Roll up
  sub-categories" toggle that combines each category's children into its top-level parent total (e.g.
  Electricity + Internet â†’ Utilities), so a deep category tree reads at the parent level; off by default so
  leaf detail stays visible. Pure `reports.RollUpByParent` (table-tested incl. nested children); e2e
  `reports_rollup_check.mjs`.
- **FX rate staleness signal (L4).** Each exchange rate is now stamped with when it was last set, and the
  Settings FX table flags any rate not refreshed in over 30 days with a "Stale" badge â€” so manual rates that
  silently drift (and quietly skew every multi-currency total) become visible. Pure `currency.RateStale` +
  `DefaultRateMaxAge`, table-tested; new `Settings.FXUpdatedAt` round-trips with the dataset. e2e
  `fx_staleness_check.mjs`.
- **Debt-free date on the payoff calculator (L5).** The payoff result now shows a calendar "Debt-free by
  <Mon YYYY>" date beside the month count, so "24 months" is also "May 2028" â€” a concrete finish line. Uses the
  existing pure `payoff.DebtFreeMonth`. e2e `payoff_debtfree_date_check.mjs`.

### Fixed
- **Command-palette and keyboard actions no longer crash the app.** Running the "New transaction" command,
  toggling the theme or sidebar, or pressing Alt+N called framework hooks (`UseQuickAdd`/`UseRailCollapsed`/
  `UsePrefs`) from inside a JS event callback â€” outside any component render â€” which panicked the whole wasm
  app (`GoUseAtom called outside component context`). These now route through captured-atom setters
  (`SetQuickAdd`/`ToggleRailCollapsed`/`SetPrefs`), the same pattern as the toast notice. New e2e
  `palette_toggle_action_check.mjs` covers Ctrl/âŒ˜+K open/close/Escape toggling and that direct actions
  actually fire (quick-add opens, sidebar collapses).

### Added
- **Command palette jumps to your data (L14).** The Ctrl/âŒ˜-K command palette now indexes your own accounts,
  goals, and budgets by name â€” type "Everyday Checking" and run it to jump straight to that screen â€” instead of
  only listing screens and actions. (The palette's verb aliases and broad action set were already in place.)
  e2e `palette_entities_check.mjs`.
- **One-tap Year view for Reports (L16).** Added a "Year" option to the period resolution control (alongside
  Week / Month / Quarter) and made the Reports screen period-aware in the top bar, so an annual / tax-season
  review is a single tap â€” every report, total and breakdown recomputes for the whole calendar year. New pure
  `period.Year` resolution (Truncate/Step/Label, table-tested); e2e `reports_year_view_check.mjs`.
- **Offline indicator (L19).** A calm "Offline" pill now appears in the top bar when the browser loses
  connectivity (and disappears when it returns), with a tooltip reassuring you that changes are saved on this
  device and will sync when you're back â€” fitting for a local-first app used on a plane. Backed by a shared
  online-state atom kept in sync with `navigator.onLine` and the window online/offline events. e2e
  `offline_indicator_check.mjs`.
- **Per-transaction receipt attachments (L29).** "Keep the receipt": each transaction row now has an "Attach
  receipt" action that uploads an image and links it to that transaction, a paperclip marker (with a count) on
  rows that have receipts, and a click-to-preview overlay. The Artifacts screen shows "Referenced by N
  transaction(s)" on each artifact. Receipts ride the dataset backup (the `AttachmentRef` lives on the
  transaction and the image bytes on the Artifact), locked in by `store.TestAttachmentRoundTrip`; e2e
  `receipt_attach_check.mjs`. (Moving artifact bytes to IndexedDB for large receipt libraries is deferred.)
- **Bulk-action undo + select-all-filtered (L25).** Destructive ledger bulk actions are now reversible: bulk
  delete, recategorize, and mark-cleared each capture the affected rows' prior state and show an inline
  "Deleted 5 Â· Undo" banner that restores them (re-creating deleted rows with their original IDs). A new
  "Select all" button selects exactly the current filtered set in one click. New `appstate.RestoreTransactions`
  (unit-tested); ledger rows now carry `data-id`. e2e `bulk_undo_check.mjs` + correctness gate
  `bulk_ops_check.mjs` proving bulk ops affect exactly the selected rows.
- **Subscription cancellation tracking + charged-after-cancel alert (L12).**

### Added
- **"Always categorize like this" â€” create a rule from a transaction (L15).** Every transaction row gains an
  action that opens the Rules screen with the rule form prefilled from that transaction (match phrase = its
  payee/description, category = its current category), so turning a one-off categorization into a standing rule
  is one click. The prefill rides a shared `uistate` rule-draft atom (same pattern as the dialog host). Pairs
  with the existing live match-count preview. e2e `create_rule_from_txn_check.mjs`.

### Tests
- **Rule auto-categorization round-trip gate (L15).** `e2e/rules_check.mjs` covers the core "set it and forget
  it" flow end to end: create a rule (phrase â†’ category), add a transaction whose description matches, assert
  it is auto-filed into the rule's category, and confirm it survives a reload. Auto-discovered by run-stories. The Subscriptions screen is no
  longer read-only: each detected subscription has a "Mark as cancelled" action (with Undo), and if a cancelled
  subscription bills you again, a prominent alert banner calls it out â€” "You cancelled Gym membership on May 20
  but were charged $40.00 on Jun 3" â€” the real money-saver. New `domain.SubscriptionCancellation` entity
  (persisted + round-tripped), pure `subscriptions.ChargedAfterCancel` (table-tested, FX-aware), and
  `appstate.MarkSubscriptionCancelled`/`Unmark`/`Cancellations`; e2e `subscription_cancel_check.mjs`.
- **Runway indicator on what-if plans (L27).** A what-if plan that draws its balance down now shows the key
  number â€” "Money lasts ~5.6 months" â€” with a âš  danger marker, instead of just silently projecting to a
  negative end balance. Plans that stay solvent over the horizon show a calm "Stays positive through N months."
  New pure `planning.RunwayMonths(plan)` with interpolated fractional crossing (table-tested incl.
  never-depletes, already-negative, and one-time-dip cases); e2e `plan_runway_check.mjs`.
- **Goal-completion lifecycle (L20).** Finished savings goals now have somewhere to go. An over-funded goal
  shows a calm "<amount> over target" note; a completed goal gains an Archive action that moves it into a
  collapsible "Achieved" section (with Unarchive), and archived goals are excluded from the headline "Overall
  progress" so a pile of finished goals no longer dilutes the figure. New `Goal.Archived` flag (JSON
  round-trip), pure `goals.Overfund` + `goals.OverallProgress(goals, includeArchived)` (table-tested), and
  `appstate.ArchiveGoal`; e2e `goal_lifecycle_check.mjs`.
- **Spending report grouped by a custom field (L18 / L16).** A new "Spending by <field>" section on Reports
  totals expenses grouped by any transaction custom field's value, with a selector to switch fields and a CSV
  export. Booleans show as Yes/No, numbers strip trailing zeros, and untagged transactions fall into a
  "(no value)" bucket. This turns custom fields from a dead end into a reporting dimension â€” e.g. spending per
  Property, or a "Deductible" total for tax time (which also covers L16's tax-tagging with no extra schema).
  Pure `reports.ByCustomField` + `reports.CustomFieldCSV`, 9 table tests; e2e `report_by_customfield_check.mjs`.
- **"Repeat" a transaction from the add form (L24).** The transaction add form now has a Repeat picker
  (weekly/monthly/quarterly/yearly). Choosing a cadence posts the entered transaction now and creates an
  auto-posting recurring schedule (first future due one cadence step after the entered date), so recurring
  bills, income and "pay yourself first" can be set up inline instead of only on the Planning screen â€” and the
  boot auto-post carries them forward. Transfers are excluded for now. e2e `txn_add_repeat_check.mjs`.
- **Recurring to-do tasks (L26).** A money chore can now repeat: a "Repeat" picker (weekly/monthly/quarterly/
  yearly) on the to-do add form and inline editor marks a task recurring, and completing it automatically
  spawns the next occurrence with its due date advanced one cadence step. Recurring rows show a "â†» <cadence>"
  badge. Re-opening a completed task does not spawn a duplicate. New pure `internal/taskrecur` package
  (next-occurrence logic, unit-tested) + atomic `appstate.CompleteTask`; e2e `recurring_task_check.mjs`.
- **To-do items can link to the entity they're about (L26).** A money chore ("pay the credit card", "rebalance
  the 401k") can now be attached to a specific account, budget, goal or transaction via a "Link to" picker on
  the to-do add form and inline editor. The task row then shows a clickable "â†’ <name>" deep-link that navigates
  straight to that entity's screen, turning the to-do list into an actionable money command center. Resolution
  is graceful â€” a link to a since-deleted entity shows "(linked item removed)". New pure `internal/tasklink`
  package (route + name resolution, unit-tested); e2e `task_entity_link_check.mjs`.
- **Due recurring transactions auto-post on app open (L24).** Scheduled bills, paychecks and "pay yourself
  first" transfers (the `Recurring` schedules managed on Planning, with autopost enabled) now post the moment
  the app boots â€” catching up any periods missed while it was closed â€” instead of only when you visit Planning
  and click "Post due". It runs after autosave is armed so the advanced schedule and new transactions persist
  immediately, and is idempotent across reopens (each schedule's next-due advances past today, so reopening
  never double-posts). e2e `boot_autopost_check.mjs` gates the catch-up and the no-double-post invariant.
- **Per-transaction member assignment (L21).** The transaction add form and the inline row editor now carry an
  optional "Who" member picker (shown only when a household has more than one member). It defaults to the
  account's owner and follows the account when you switch it, until you explicitly override â€” so on a shared or
  joint account you can attribute a single purchase to a specific person instead of always inheriting the owner.
  The choice persists to `Transaction.MemberID` and is respected by the existing ledger member filter and the
  per-member reports. e2e `member_assignment_check.mjs` gates it.
- **Apply allocation (L17).** The Allocate screen no longer only *suggests* â€” an "Apply allocation" button
  commits the plan with earmark-only semantics (no cash moves between accounts; money is never created or
  lost). Goal destinations add to the goal's saved amount, capped at target with any overflow disclosed;
  account and liability "pay-down" destinations become persisted earmark records (new `domain.Earmark`
  entity). Apply is atomic (snapshot-on-failure rollback) and reversible via a single Undo. Pure
  `allocate.PlanActions` mapping + `appstate.ApplyAllocation`/`UndoLastAllocation`, fully unit-tested; e2e
  `allocate_apply_check.mjs` gates applyâ†’persistâ†’undo and `allocate_determinism_check.mjs` asserts
  `sum(distributed) + keptBack == amount` to the cent across several amounts/reserves.

### Fixed
- **CSV import is now row-resilient (L23).** A single malformed row (non-numeric amount, missing required
  field) no longer aborts the entire paste. The parser (`store.TransactionsFromCSVResilient`) processes
  rows independently â€” valid rows import, bad rows are collected as `{line, reason}` and skipped â€” and the
  Documents importer reports "Imported N. Skipped K row(s) (couldn't be read)." in plain English. Table
  tests cover all-valid / some-bad / empty / header-only / totally-malformed; e2e gate
  (`import_resilience_check.mjs`) pastes 3 valid + 2 malformed rows and asserts exactly the 3 land.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Unified top-bar controls into consistent filled icon buttons.** The notification bell, music play/pause
  toggle, and the "+ Add" control now share one borderless, filled icon-button style (the Add control is now a
  plus icon instead of "+ Add" text). All three carry an `aria-label` and a hover `title`, and the Add button
  exposes `aria-haspopup`/`aria-expanded` for its menu.

### Removed
- **The Tailwind CSS CDN is gone (C91).** Deleted `<script src="https://cdn.tailwindcss.com">` and the inline
  `tailwind.config` from `web/index.html`. The app no longer loads any third-party CSS/JS to style itself â€” one
  fewer external dependency, SRI-pinnable, and it works offline. (Google Fonts is the only remaining external
  asset.)

### Added
- **Typed, CDN-free CSS (C91).** All Tailwind utility classes are now emitted by a typed Go vocabulary
  (`internal/ui/tw`, built on the gwc `css`/`css/u` engine) that injects the exact same CSS at runtime into
  `<style id="gwc-css">`. ~1,450 static call sites use `css.Class("semantic", tw.Utilâ€¦)`; ~40 dynamically
  composed class strings (rail items, KPI tiles, menus, chips, progress bar) fold typed rules into hashed
  classes via `tw.Fold`/`tw.ColorClass`. Exact-value table tests; verified zero `cdn.tailwindcss.com` requests
  and correct computed styles after removal.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Upgraded GoWebComponents to v3.2.0** (typed CSS `css`+`css/u`, `db/sqlite`, `RawHTML`, hookcheck, and more).
  Migrated the one breaking change: the shorthand string-class setter `shorthand.Class(string)` was renamed to
  `shorthand.ClassStr` (the typed `css.Class(...any)` now subsumes the string form), updated across all 49
  shorthand-importing files (~1,526 call sites). No behavior change â€” verified build/vet/tests green and a clean
  boot with zero console errors. This is the groundwork for the C91 Tailwind-CDN removal (typed-CSS migration).

### Added
- **Undo / redo (Ctrl+Z / Ctrl+Shift+Z) (C78).** A diff-based change history over the whole dataset. Every
  autosave write now captures an undo point automatically (diff vs the last snapshot â€” no per-write-path
  instrumentation), built on the pure `internal/history` engine and a new pure `internal/undosnap` converter
  (dataset export-JSON â†” snapshot, table-tested). Undo/redo apply the inverse/forward change set, re-hydrate the
  store, and re-render. Wired to Ctrl+Z / Ctrl+Shift+Z (works from a focused field) and command-palette
  Undo/Redo, with help-overlay rows. Covered by an e2e (add task â†’ Ctrl+Z reverts end-to-end).
- **Encrypt dataset at rest (C45).** When a passcode lock is active, the autosaved dataset in localStorage is now
  encrypted (AES-GCM-256, key derived from the passcode via PBKDF2-SHA256 600k; key never persisted). New pure
  `internal/cryptobox` defines the on-disk envelope (marker + base64 salt/iv/cipher, 18 tests);
  `internal/app/datasetcrypto.go` drives Web Crypto. With no passcode the data stays plaintext (zero migration);
  setting/removing a passcode migrates the at-rest copy immediately. On boot an encrypted dataset defers
  hydration until the passcode gate is satisfied, then decrypts. No lockout: decrypt failure keeps the
  ciphertext; "Forgot â†’ wipe" stays the only destructive recovery. Covered by an e2e (plaintext without a
  passcode; envelope-at-rest with one; reload â†’ unlock â†’ decrypt round-trip).
- **Bank/card statement import (C74).** A new "Import a bank or card statement" card on the Documents screen
  parses statement exports in almost any delimited layout â€” delimiter auto-detect, BOM/CRLF, quoted fields, and
  automatic column mapping (date/description/amount, or separate debit/credit) by common bank header labels.
  Amounts normalise to signed minor units (parentheses/sign/symbol/DR-CR aware, many date layouts); unparseable
  rows are skipped with per-row errors rather than aborting. Parsed rows flow into the existing review â†’ dedupe â†’
  import pipeline. Pure `internal/statement` package (8 tests) + e2e (auto-mapping, bad-row skip, signed amounts,
  dedupe on re-import).
- **Reusable UI primitives (C73).** `internal/ui/primitives.go` adds `Card`, `FormField`, `IconButton`
  (loop-safe, owns its hook), `EntityRow` (hookless, loop-safe), and `StatGrid`, matching the existing DOM
  classes so no CSS changes are needed; plus a pure `JoinClass` helper (`internal/ui/classutil.go`, tested).
  `internal/screens/members.go` ported to `Card` as the reference; other screens adopt them incrementally.
- **Muzak resume travels with your data (checkpoint-only DB persistence).** The music state (on/off, volume, track,
  position) is now also mirrored into the dataset's `Settings.Music`, so it survives a localStorage wipe and rides
  along with export/import and backups â€” on a fresh device the player resumes the saved track/volume. To avoid
  re-serializing the whole dataset on every position tick, it's written only at **checkpoints** (track change,
  pause, page close, toggle, volume release) via a Goâ†”JS bridge (`window.cashfluxMusicSave` â†’ `appstate.PutMusicState`);
  the high-frequency live position stays in localStorage. On boot, the dataset's music state seeds this device's
  resume point when it has none. Covered by an e2e (checkpoints into the dataset; reseeds + resumes on a fresh
  device).
- **Background music ("muzak").** A low-volume looping ambient player, **on by default**, toggled from a
  speaker/mute icon in the top bar (next to + Add) and with a **volume slider + on/off in the Settings modal**.
  Ships an 8-track calming playlist (`web/audio/calm-01..08.mp3`). `web/muzak.js` has a proper `Playlist` data
  structure (list + cursor, advance/shuffle), **crossfaded track transitions** (two `<audio>` elements overlapped
  near track end), **volume fading** (fade-in on enable, fade-out on disable, fade-in on loop), and **resume**:
  it remembers the current track + position (localStorage) and continues from there on reload. Browsers block
  autoplay, so playback starts on the first click/keypress; missing files are skipped and an all-tracks-failed
  case backs off instead of busy-looping. The on/off choice and volume persist. New `Volume`/`VolumeMute` icons
  (and the curated icon set's missing `Copy` entry is fixed). Covered by an e2e (default-on, toggle, controller +
  playlist DS, cursor advance, persistence, resume-to-saved-track, Settings slider).
- **Widget Manager â€” Phase 2 (tile styling with live preview).** A new "Tile style" editor on the Widget Manager
  page lets you style tiles: pick **All widgets** for the global default or a single widget to override it, and set
  **background, text, border color, accent, border width, corner radius, font, weight, and shadow** â€” with a **live
  preview tile** that updates as you go and a **Reset to theme**. Per-widget overrides layer over the global tile
  style, which layers over the app theme; only the fields you set are applied (everything else inherits). New pure
  `widgetstyle` package resolves a config into inline tile CSS (tested); the dashboard tiles apply it live (reusing
  the existing per-widget config store â€” global default under id `_all`). Per-widget accent now renders as a tinted
  top strip composed with the chosen shadow. Covered by a new e2e (preview updates, the override reaches the
  dashboard tiles, reset clears).
- **Widget Manager â€” Phase 1 (layout, visibility, reorder).** The `/widget-manager` page is now a working hub for
  the dashboard's widgets, built on the reusable sortable `DataTable`: each widget is a row with a visibility
  switch, width/height steppers, and reorder up/down; the toolbar holds the arrangement mode (Custom/Auto) +
  Reset and bulk Show-all/Hide-all. New pure `widgetvis` set (instance-keyed) persists hidden widgets; the
  dashboard now renders from the layout-items list and **skips hidden tiles** (reflowing the rest), so every
  manager control is wired straight back into the dashboard. The dashboard layout controls moved here from
  Settings, and the previously-unmanaged "Spending highlight" tile is now part of the layout. Covered by a new
  e2e (hide removes the tile; resize + reorder persist) plus `widgetvis` unit tests.

### Added (earlier)
- **Widget builder & Widget manager pages (scaffolding).** Two new left-rail screens under Tools â€º Build â€”
  `/widget-builder` (widget creation) and `/widget-manager` (widget management) â€” registered in the screens
  registry with rail icons and i18n. Blank placeholder pages for now (routing + nav only); the composition engine
  lands later. Covered by an e2e (both appear in the rail and render).

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **The dashboard's dead header cell is now a configurable "Needs attention" widget.** The full-width top cell
  used to hold only a title and the layout manager; it's replaced by a real draggable/resizable widget (default
  **4Ã—1**, top of the grid) that surfaces the urgent, act-now signals â€” bills due soon, near/over budgets, stale
  balances, overdue & high-priority to-dos, and the biggest spending spike â€” ranked by the new `internal/attention`
  package. Each row deep-links to its screen and scrolls to the item. It's **responsive by span** (one item + a
  count at 1Ã—1; a wrapping chip row when wide-and-short; a stacked list when taller) and **configurable via its
  gear**: per-source toggles, a bills-due window, a max-items cap, and a minimum-severity floor. Existing saved
  layouts gain the widget automatically (a new `dashlayout.Reconcile` merges the current widget set into a saved
  layout â€” surfacing newcomers at the top, dropping retired ids, preserving the user's order and sizes).
- **The dashboard layout manager moved to Settings.** The Custom/Auto mode selector and Reset-layout action now
  live under a **Dashboard layout** section in the Settings modal, freeing the canvas to be all widgets.

### Added
- **`internal/attention` â€” urgency-ranking for the dashboard digest.** Pure package that takes the already-computed
  dashboard signals (bills due soon, near/over budgets, stale balances, overdue & high-priority to-dos, the top
  spending spike) and returns one severity-ordered, deduped, capped digest under a `Config` (per-source toggles, a
  bills-due window, a max-items cap, and a minimum-severity floor). Soonest deadline wins within a severity, so an
  overdue task outranks a bill due tomorrow. Fully table-tested; no `syscall/js`. Feeds the new "Needs attention"
  widget.

### Fixed
- **Toasts/data-revision can post from global callbacks without a framework panic.** `paletteNotify` and the
  data-revision bump previously called the `UseAtom`-based hooks (`UseNotice`/`UseDataRevision`) directly, which
  panics ("GoUseAtom called outside component context") when invoked from a non-render callback (keyboard
  shortcut, command-palette action). Added captured-atom helpers `uistate.PostNotice` and
  `uistate.BumpDataRevision` and routed those callers through them. The Shell and the To-do screen now subscribe
  to the data-revision atom so a whole-dataset replacement (undo/redo, decrypt-hydrate, import) re-renders.
- **Offline service worker pins the vendored D3 (not the CDN URL).** Updated the deploy runtime test to assert
  the SW caches the local `./d3.min.js` (matching the C44 no-CDN vendoring) instead of the old jsdelivr URL;
  bumped the service-worker cache to `cashflux-v244`.
- **Chat deep links no longer trigger a full page reload (C90.2).** The in-app link interceptor now reads the
  anchor's parsed `origin`/`pathname`/`hash` instead of string-matching the raw `href`, so it also catches links
  the model phrases as an absolute same-origin URL (`http://host/todo#id`) â€” previously those slipped past the
  `/`-prefix check and the browser did a full navigation (which, given the gwc deep-link 404 â†’ SW shell fallback,
  reloads the whole app and wipes in-memory state). It now also recognizes links to any known app route even when
  phrased with a different host (`isAppRoutePath`), runs in the capture phase, ignores modifier/middle clicks (so
  cmd-click still opens a new tab), and guards against undefined modifier fields. Bumped the service-worker cache
  to `v225` so already-open clients drop any stale shell and pick up the fix. The e2e detects a real reload via a
  window sentinel and exercises relative, absolute-same-origin, and cross-host hrefs.

### Added
- **Creation tools return a deep link, and chat links navigate in-app (C90.2).** Every creating tool
  (`add_task`, `add_transaction`, `add_account`, `add_transfer`, `add_goal_contribution`) now returns the new
  entity's id as a Markdown link to its screen anchored to that id â€” e.g. `[Open it](/todo#<id>)`. The system
  prompt instructs the assistant to always surface that exact link in its reply. Clicking an internal link in a
  chat answer is intercepted to route within the app (no full reload) and smooth-scroll/flash the target row;
  entity rows carry an `id` anchor for the jump. Covered by a new e2e (link returned, click navigates to /todo,
  row anchor present).
- **Creation tools dedupe before creating (C90.2).** Each creating tool first checks for an existing or
  near-identical entity (Jaccard-similar titles/names, or same account+amount+day+payee for transactions, or
  matching account names) and, if found, returns the existing item's link instead of spawning a duplicate â€” so
  the model relays "a similar one already exists" rather than cloning. Covered by the new e2e (a task matching a
  sample task is blocked).
- **Insights chat account + transfer tools, modeled correctly (C90.2).** New `add_account` (assets and
  **liabilities** â€” loans, credit cards, mortgages â€” with APR/credit-limit/min-payment; a liability balance is
  the amount owed), `add_transfer` (matched two-leg transfer between accounts, FX-aware), and
  `update_account_balance` (reconcile). The system prompt now guides multi-account events so net worth stays
  correct â€” e.g. a 401(k) loan is treated as **net-worth-neutral** (a new liability *plus* the cash received),
  not a one-sided loss. Also fixed `add_transaction`/transfers failing with "desc is required" (a description is
  now always set, with an optional `description` arg) â€” the cause of the assistant getting stuck asking for one.
  Covered by a new e2e (creates a liability that shows on Accounts; performs a transfer).
- **Insights chat auto-names itself (C82).** Once a chat has a few exchanges (â‰¥4 messages), it asks the model for
  a short 2-4 word title from the conversation and updates the switcher tab â€” once per chat, preserved across
  sessions (a `Named` flag stops autosave from re-deriving the title). Covered by an e2e.
- **Insights chat can now make changes, with approval (C90.0 + first write tools).** Mutating tools pause the
  agent loop and show an **approval card** in the thread (preview of the change + Approve/Decline) before running;
  reads never prompt. First write tools: **add_task**, **complete_task**, **add_transaction** (resolves account/
  category by name), and **add_goal_contribution**. Approving runs the change through `appstate`; declining feeds
  "declined" back to the model. Covered by an e2e (approve creates the task â†’ shows on To-do; decline makes no
  change). _Known issue: a second mutating approval within the same chat session can hang (goroutine-scheduling
  interaction); starting a new chat resets it â€” to be fixed before broad write-tool rollout._
- **Insights chat read tools across more screens (C90.1).** Added `list_budgets`, `list_goals`, `list_tasks`,
  `list_recurring` (upcoming bills), and `spending_breakdown` (top categories for a period) â€” so the chat can
  answer about budgets, goals, to-dos, recurring/bills, and where the money went, from live data. Covered by the
  tools e2e (now exercises 11 tools end-to-end).
- **Insights chat: a fetch_webpage tool to read search results (C82).** `web_search` now also returns source
  URLs, and a new `fetch_webpage` tool reads a page's readable text (via the CORS-friendly Jina Reader) so the
  model can dig into a result instead of relying on the snippet.
- **Insights chat: a web_search tool + a prompt that estimates (C82).** The chat can now look up current/external
  facts (tax brackets, rates, prices) via a `web_search` tool (keyless DuckDuckGo Instant-Answer by default) and
  combine them with the calculator + the user's figures to **estimate** things the data doesn't directly contain
  (e.g. taxes) instead of refusing. Settings gains an optional **web-search API key** field (kept on-device) for
  paid/higher-limit access, sent only with search requests. Covered by an e2e that runs web_search + calculator
  end-to-end.
- **Insights chat: an editable system prompt (C82).** An "Edit prompt" button opens a flip-panel where you can
  customize the assistant's persona/instructions (saved on-device). The live financial context and the data
  tools are always injected automatically, so a custom prompt never loses them; "Reset to default" reverts.
- **Insights chat now uses tools to answer from real data (C82).** The chat drives a bounded tool-calling loop:
  the model can call local, read-only finance tools and answer specific questions from the user's own figures
  instead of guessing. Tools: **spending_by_category** (resolves the category by name â†’ totals it for a period),
  **list_transactions**, **list_members**, **account_balances**, **financial_summary**, **check_affordability**
  (backed by the `afford` engine), and a **calculator** over a finance expression (`net_worth`, `assets`,
  `liabilities`, `income`, `spending`, `net_cashflow`) via the sandboxed `formula` engine. The system prompt now
  injects the live aggregates + the user's category names and directs the model to call a tool for any specific
  number. New pure `ai` tool-call wire types (`BuildToolRequest`/`ParseChat`/`ToolResultMessage`, table-tested)
  and an `ai.SendChatTools` transport. The backend-proxy path falls back to a plain (toolless) reply until the
  proxy supports tools. Covered by a new e2e that runs all six tools against the sample dataset and verifies each
  result, plus the existing send/resume/error e2e.

### Fixed
- **Crash on keydown after the chat-history change.** The composer's `OnKeyDown` prop dispatched a synthetic
  keydown event that lacked modifier properties; the app's global keyboard-shortcut listener then called
  `Value.Bool()` on an undefined `metaKey`, which **panicked and exited the whole Go program** â€” after which
  nothing in the app worked. Reverted to a raw document keydown listener (native events only) that dispatches a
  native `input` event to keep the framework state in sync (so clicks still work after cycling), and hardened the
  global shortcut listener to read modifier flags defensively. Covered by a new e2e that dispatches a malformed
  keydown and asserts the app doesn't crash.
- **Insights chat: Send and Enter work after cycling messages with the arrow keys.** The Up/Down history was a
  raw DOM keydown listener that set the input value directly, which desynced the framework's vdom and broke the
  next click/Enter. It now uses the framework's `OnKeyDown` (Enter sends, Up/Down cycle, typing exits history),
  so state updates re-render cleanly and Send/Enter keep working. Covered by an e2e (cycle â†’ Send and cycle â†’
  Enter both send).
- **Insights chat: Send / Enter no longer risks reloading the page.** The composer is no longer a `<form>` â€”
  Send is a plain button and Enter is handled by the keydown listener (Shift+Enter is ignored) â€” so there's no
  native submit that could trigger a full page reload. Service-worker cache bumped to evict any stale shell.
- **Insights chat: on load, the thread starts at the latest message.** Reopening a saved chat left the thread
  scrolled to the top â€” the auto-scroll fired before each bubble's Markdown filled in, so the container had no
  height yet. The scroll is now deferred until after layout, landing on the most recent message.
- **Insights chat: assistant Markdown replies are now styled.** The replies were converted to HTML (marked +
  DOMPurify) but Tailwind's preflight reset stripped heading sizes, list bullets, and spacing, so they looked
  like flat text. Added a theme-agnostic prose stylesheet for `.insights-answer` (headings, lists, bold/italic,
  links, inline/block code, blockquotes, tables, rules) that works in light and dark; service-worker cache
  bumped so a stale cached shell refreshes.
- **Insights chat: the first message after reopening a saved chat now works.** Reopening Insights resumes the
  most recent conversation; the first send into a resumed chat appeared to do nothing (the request was made but
  the reply never showed). Cause: under the state churn of the resume + autosave, the assistant turn was
  appended via a functional state Update that read a stale base and dropped it. The reply is now written by
  setting the thread to the exact sent history plus the reply (sending is disabled while in flight, so it's
  authoritative); the same hardening was applied to message deletion. Covered by an expanded chat e2e that
  reloads, resumes, and sends.
- **Insights chat works with reasoning models.** o-series / gpt-5.x models reject a custom temperature on
  /chat/completions; the chat now omits temperature for them (mild 0.4 for other models), so the configured
  OpenAI model no longer silently errors.

### Added
- **Insights: backend-vs-OpenAI mode toggle.** When a backend is configured, a one-line toggle in the chat
  lets you switch between the **backend AI proxy** and the **direct OpenAI provider** without leaving the
  screen (writes the `BackendDisabled` pref). With no backend configured the chat always uses OpenAI directly.
- **Insights conversation switcher â€” multiple saved chats (C82).** A switcher row with **New chat** and a pill
  per saved conversation: tap to switch, Ã— to delete. The live thread **auto-saves** to the store on every
  message (and on delete/retry), titled from its first question; opening Insights **resumes the most recently
  updated chat**. Deleting the open chat starts a fresh one.
- **Insights conversations persist to the local store (C82).** New `domain.Conversation` + `domain.ChatMessage`
  types, a `conversations` SQLite table (one JSON row per chat, messages embedded), `SQLiteStore`
  Put/Get/List/Delete, lossless export/import wiring, and `appstate` `Conversations()`/`PutConversation()`/
  `DeleteConversation()`. Table-tested (CRUD + exportâ†’import round-trip). The conversation-switcher UI consumes
  this next.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Insights composer: Up/Down cycles your previous messages (C82).** Shell-style history â€” Arrow Up walks back
  through your prior messages in the chat, Down walks forward, and going past the newest restores your in-progress
  draft. Covered by an e2e.
- **Pinned insights render as Markdown, collapsed to 3 lines (C82).** Pinned insights are now marked-rendered
  (matching the chat bubbles) and clamped to three lines with a Show more/less toggle, so a long saved insight
  stays a compact quick-reference and expands on demand.
- **Insights chat: the thread scrolls in a bounded region so the composer stays on screen (C82).** Previously a
  long conversation grew the page and pushed the text input below the fold. The thread is now a bounded
  (`max-h-[55vh]`), internally-scrolling region with the composer pinned beneath it; auto-scroll moves only that
  container (never the page) to keep the newest message in view.
- **Insights chat: actions hover-reveal, delete unravels the thread, auto-scroll to newest (C82).** The action
  icons under a message stay hidden until you hover (or focus) that message. Deleting a message now removes it
  **and every later turn** (a conversation is a chain, so a mid-thread delete would orphan the continuation).
  And the thread auto-scrolls to the bottom when a bubble (or the "thinking" indicator) spawns, so new messages
  aren't left below the fold.
- **Insights chat: action icons moved below the bubble; Retry on the last message either side (C82).** Copy /
  Pin / Retry / Delete now sit in a row *under* each message rather than inside it. Retry is offered on the
  latest message whether it's the assistant's reply or **your own message** â€” so a turn that errored with no
  reply can still be re-sent.
- **Insights chat: message actions reworked (C82).** Assistant replies are now rendered with **marked**
  (vendored locally) sanitized by **DOMPurify** before becoming innerHTML â€” richer Markdown than the prior
  inline renderer. Each reply gets **Copy** (to clipboard) and **Delete**, plus **Retry** on the latest reply
  (re-answers the last prompt). User messages can be **deleted** too (hover to reveal). **Pin** stays. The
  per-message **Save-as-task button is removed** â€” saving to To-do will become an agent tool the model invokes
  when asked. New `copy` icon; `marked`/`DOMPurify` added to the service-worker precache.
- **Insights is now a chat interface (C82 wiring, supersedes the C59 two-card layout).** The Explain/Q&A cards
  are replaced by a conversation thread: user bubbles, Markdown assistant bubbles each with Save-as-task / Pin
  and a token/cost note, a sticky composer, and starter-question chips that send on tap. The whole history is
  sent each turn so it's genuinely conversational. This is the wasm/UI half of C82; the gated tool-loop
  (richer answers, affordability) and token streaming land next on top of the same screen.
- **Insights gives Explain and Q&A their own answer cards (C59).** They previously shared one result slot, so
  asking a question wiped the "Explain my month" narrative and vice-versa. Each now has its own slot, token/cost
  note, and Save-as-task / Pin actions, so both answers coexist; the in-flight action (`explain`/`qa`) is tracked
  so only that card shows the busy/cancel state while the other stays guarded.
- **Pinned insights clamp long text with a Show more/less toggle (C59).** Pinned rows over ~140 characters now
  collapse to two lines (`line-clamp-2`) with an expand toggle instead of stretching the list; each row owns its
  own expand state.
- **Subscription price-change rows show tone + an arrow icon (C56/C46).** A price increase now renders red with an
  up arrow and a decrease green with a down arrow, instead of conveying direction by wording alone â€” color-plus-
  shape, matching the Reports trend markers. Covered by a new `subscriptions_price_tone_check` e2e.
- **Category tree nesting uses real indentation, not em-dashes (C63).** Sub-category rows now indent with depth-
  proportional left padding and a subtle guide line instead of literal "â€” " prefixes, for a cleaner hierarchy. The
  parent-picker dropdowns (where CSS padding can't reach) indent with non-breaking spaces rather than em-dashes.
  Covered by a new `categories_nesting_check` e2e.

### Added
- **Clear backend on/off switch in Settings (C81 follow-up).** The Settings modal now leads the backend section
  with a "Connect to a backend (sync + AI proxy)" toggle. Turning it off keeps the app fully local even with a
  server URL/token saved â€” no sync loop, no visibility/online listeners, no AI-proxy dialing â€” so an unreachable
  backend can't surface websocket connection errors the user can't dismiss. Backed by a new (inverted, default-on)
  `BackendDisabled` pref and a central `prefs.BackendActive()` predicate that every sync and AI-proxy path now
  gates on. Covered by a new `settings_backend_toggle_check` e2e.
- **Insights system-prompt assembler (C89 prompts, logic).** New pure, table-tested `internal/aiprompt`: `System`
  composes the agent's system prompt from the house rules (determinism â€” narrate computed figures, never invent
  numbers; call a tool for exact values; plain English), the bounded context block (`aicontext`), and a tool
  manifest (`agent.Registry` specs). Empty context/tools sections are omitted. The "prompts" piece that ties the
  context and tools together.
- **Anthropic Messages-dialect shaping (C81 phase 3, logic).** New pure, table-tested `internal/anthropic` builds
  the Anthropic `/messages` request body and parses its response â€” the one wire dialect that isn't OpenAI-compatible.
  It models the differences: a top-level `system` field, required `max_tokens`, tools with `input_schema`, and
  base64 image content blocks for vision. `ParseResponse` returns concatenated text + `tool_use` calls + usage +
  stop reason, and turns an error envelope into a Go error. No I/O â€” the transport wires it later.
- **Insights agent read-tools (C89 phase 2, logic).** New pure, table-tested `internal/aitools` registers the
  Insights agent's read tools on the C82 `agent.Registry`: `query_transactions` (reuses `txnfilter.MultiCriteria`
  to answer "how much on groceries?"), `account_balances`, and `affordability` (reuses `afford.CanAfford` for real
  math, not an LLM guess). Tools bind to a small `DataSource` interface â€” not `appstate` â€” so the package is pure
  and fakeable; appstate provides the production source when wired, and the write tools + audit/undo are phase 3.
- **gpt-5.5 default + Responses/websocket/streaming request profiles (C81/C89, logic).** `internal/aiprovider` now
  leads with **gpt-5.5** (a reasoning model) as the default and models the app's preferred request shape: a `Profile`
  of `APIStyle` (Responses / chat-completions), `Transport` (websocket / https), streaming, and reasoning `Effort`.
  `DefaultProfile()` = Responses over a websocket, streaming, **medium** effort; `LowEffortProfile()` for lightweight
  chain-of-thought; `Provider.For(model, base)` resolves it â€” downgrading to chat-completions/https for non-OpenAI
  dialects, dropping effort for non-reasoning models. Table-tested; the websocket transport itself is the C81-p2 step.
- **Bounded Insights-agent context builder (C89 phase 1, logic).** New pure, table-tested `internal/aicontext`
  assembles a richer, privacy-tiered financial snapshot for the model's system prompt â€” net worth, period income/
  expense, accounts, budgets, goals, top categories/payees, recent transactions, and **every enabled Formula
  evaluated to its current value** â€” replacing the 4-aggregate `ai.FinancialContext`. Tiers (aggregates â†’ +formulas
  â†’ +breakdowns â†’ +recent txns) gate what's shared and top-N/recent-N cap the lists, so it injects a summary, not
  the raw ledger. Independently fixes the C59 "Q&A context too thin" gap; the tools/UI phases come next.
- **Category rows show a usage count that drills into Transactions (C63).** Each category row now displays how
  many transactions are filed under it (e.g. "25 transactions"); clicking the badge navigates to Transactions
  pre-filtered to that category, matching the Accounts/Members drill pattern. Categories with no transactions show
  a muted "No transactions". New `categories_usage_drill_check` e2e covers the badge and the persisted-filter drill.
- **Custom-page Text widget renders Markdown (C66/C32).** The Text widget on custom pages now renders its content
  as Markdown (headings, lists, emphasis, links) instead of a flat paragraph, via the same GFM-aware, raw-HTML-
  escaping framework `Markdown` used for Insights â€” so a note can be a real rich-text block and imported page
  content still can't smuggle an executable href. The widget palette description notes Markdown support.
- **Insights AI answers render as Markdown (C59).** The assistant emits Markdown (headings, bold/italic, inline
  and fenced code, links, bullet/number lists) that previously showed as one flat paragraph; the answer card now
  renders it as rich text via the framework's GFM-aware `Markdown`, which escapes raw HTML and drops active URL
  schemes (`javascript:`/`data:`) so model-authored text can't smuggle an executable href. Links open in a new
  tab with `rel="noopener noreferrer"`.
- **Asset "Advanced" disclosure on the account add-form (C49).** The optional scoring fields (Return %,
  Liquidity, Stability, Locked-until) now sit behind a "Show advanced fields" toggle so the common add path
  stays short; most accounts never set them. The toggle carries `aria-expanded` for screen readers and only
  appears for asset (non-liability) types. New `accounts_advanced_disclosure_check` e2e asserts collapse â†’
  expand (all 4 fields) â†’ re-collapse; existing `accounts_field_constraints_check` updated to expand first.
- **Rule precedence-chain diagram (C70/C64).** `internal/mermaid.FromRules` renders auto-categorize rules as a
  top-down "match â†’ category" chain (first match wins), flagging rules that can never fire â€” "(shadowed)" or
  "(matches nothing)" via `rules.Conflicts`. Wired into the Rules screen as a "Rule order" card (5th wired diagram).
  Pure + table-tested; e2e `rules_diagram_check` asserts real `<svg>`.
- **Gravatar avatar URLs (C88, logic).** New pure, table-tested `internal/gravatar`: `Hash(email)` (hex MD5 of the
  trimmed, lowercased address) and `URL(email, size)` (the avatar URL with an identicon fallback, size clamped to
  1â€“2048, default 80). The pure Gravatar half of member avatars; the members-screen wiring and uploaded-photo/Giphy
  options are later steps.
- **Delimited statement parser (C74, logic).** New pure, table-tested `internal/statement` parses a bank/card
  statement (CSV/semicolon/tab/pipe) into normalized rows: `DetectDelimiter`, `MapColumns` (header-name heuristics â†’
  date/description/amount/debit/credit/balance), a lenient `ParseAmount` (currency symbols, thousands separators,
  parentheses- and DR-negatives, signs â†’ signed minor units), a multi-layout `ParseDate` (MM/DD-first, DD/MM
  fallback for day>12), and `Parse` tying them together â€” amount from an Amount column or Creditâˆ’Debit, bad rows
  recorded and skipped. The extraction/mapping core of the import engine; the Documents-screen wiring is later.
- **Reports money-flow Sankey (C70).** The Reports screen now renders a Mermaid Sankey of income â†’ spending
  categories â†’ savings, via `uiw.Mermaid` over `mermaid.Sankey` â€” the "highest wow" diagram. Fourth wired diagram;
  covered by a new `reports_sankey_check` e2e (asserts real `<svg>`).
- **Split settle-up who-owes-whom diagram (C70).** The Split screen's settle-up card now renders a Mermaid digraph
  (debtor â†’ payer, labelled with the amount) via `uiw.Mermaid` over `mermaid.FromSettleUp` â€” a third wired diagram.
  Covered by a new `split_diagram_check` e2e (asserts real `<svg>`).
- **Diff-based change-history core for undo/redo (C78 phase 1, logic).** New pure, table-tested `internal/history`:
  a `Snapshot` (collection â†’ id â†’ row JSON) and `Diff(before, after)` that yields a minimal, deterministic
  `ChangeSet` of add/update/delete changes, `Invert()` (so undo applies the inverse â€” cascades reverse for free),
  and `Apply()` (returns a new snapshot, never mutates input). Plus a bounded undo/redo `Stack` with a redo-tail
  discard, a byte cap that drops oldest, and same-row coalescing so a burst of rapid edits is one undo step. Generic
  over the dataset (no store/appstate import); the commit seam, SQLite audit log, and UI are later phases.
- **Sankey Mermaid generator (C70).** `internal/mermaid.Sankey` emits `sankey-beta` money-flow source from weighted
  flows (CSV-quoting labels and skipping non-positive weights) â€” the foundation for an incomeâ†’categoriesâ†’savings/debt
  flow chart. Pure + table-tested; fourth of the C70 generators.
- **In-house agent tool-calling loop (C82, logic).** New pure, table-tested `internal/agent`: a `Tool`/`ToolSpec`/
  `ToolCall`/`ToolResult` type set, a name-keyed `Registry`, and `Run` â€” a bounded modelâ†’tool-callsâ†’executeâ†’repeat
  loop with step and token-budget caps, context cancellation, and a recorded `Transcript` (steps, final answer, stop
  reason, tokens). The `Model` is an interface the AI layer implements over a real provider; tools are plain Go
  handlers, and every tool failure becomes a result the model can react to rather than aborting the loop. The core
  the agentic AI builds on; binding tools to appstate (actor=agent, audited/undoable) and the UI are later phases.
- **Category map diagram on the Categories screen (C70/C63).** The category hierarchy now renders as a Mermaid
  graph beneath the lists, via `uiw.Mermaid` over `mermaid.FromCategories` â€” a second wired diagram alongside the
  Workflows flowcharts. Covered by a new `categories_diagram_check` e2e (asserts real `<svg>`).
- **AI provider registry (C81 phase 1, logic).** New pure, table-tested `internal/aiprovider` models the inference
  providers CashFlux can use: a `Provider`/`Model`/`Capabilities` type set, a `Dialect` enum (one `openai` dialect
  covers OpenAI/OpenRouter/Cerebras/DeepSeek/GLM/Kimi; `anthropic` is the one needing its own wire), an auth-style
  and a structured-output enum (`json_schema`/`json_object`/`none` â€” the cross-provider gotcha), a curated registry
  of 7 providers with default endpoints + key links + indicative per-model pricing, lookups, and `EstimateCents`.
  No transport/UI/settings change (those phases touch the contended AI/settings/store) â€” this is the data model the
  rest builds on.
- **Mermaid diagrams now render in the app (C70).** A new `uiw.Mermaid` component (mirroring `uiw.Chart`) renders
  generated Mermaid source to inline SVG via a vendored-locally `web/mermaid.min.js` (no CDN, C44) + a `web/mermaid.js`
  shim initialised with `securityLevel:'strict'` (no click-JS / raw-HTML labels, C45/C70). Wired the first case: the
  **Workflows screen shows a flowchart of each workflow** (trigger â†’ condition â†’ actions). Covered by a new
  `mermaid_render_check` e2e that asserts real `<svg>` output.
- **Multi-select transaction filter model (C83, logic).** New pure, table-tested `txnfilter.MultiCriteria` matches
  transactions with the standard mental model â€” OR within a dimension, AND across â€” over Accounts/Categories/Members
  and a new Tags dimension (a transaction matches Tags when it shares any selected tag; an empty dimension is
  unconstrained). It carries the operations the toolbar needs: `Normalize` (dedup+sort), `Equal` (explicit, since
  slices aren't comparable), `Add`/`Without(field, value)`/`Toggle` for per-value chips, and `ActiveValues`. Added
  additively (the single-value `Criteria` is unchanged); the Transactions-screen wiring is a later step.
- **Derived shell tokens in the theme engine (C69, logic).** The theme now emits the CSS tokens the shell needs but
  the engine never produced â€” `--bg-elev` (elevated surface), `--text-faint`, `--accent-dim`, `--warn`, and a
  `--danger` alias of the down color â€” derived from the theme's own tokens via a new pure `mixHex` blend, so any
  built-in or custom theme gets sensible values with no migration. `CSSVars()` emits them and `Validate()` checks
  text legibility on the elevated surface. Pure + table-tested; the prep step before rewiring the shell's hardcoded
  colors to these vars (which touches `index.html`, deferred).
- **Settle-up Mermaid generator (C70).** `internal/mermaid.FromSettleUp` renders a split settle-up plan as a
  who-owes-whom digraph (debtorâ†’creditor edges labelled with the amount), taking name/amount formatter closures so
  the package stays currency-free. Pure + table-tested; third of the C70 generators.
- **Category-tree Mermaid generator (C70).** `internal/mermaid.FromCategories` renders a category hierarchy as a
  left-to-right graph (parentâ†’child edges), with generated node ids so unsafe category IDs can't break the syntax
  and orphan parent references don't produce dangling edges. Pure + table-tested; second of the C70 generators.
- **"Restore from a backup file" â€” the L9 import half.** The inverse of the full-install export: a command-palette
  action that picks a backup `.json`, validates it via the `backup` envelope, confirms the destructive replace, and
  writes the workspace registry, appearance side-state, and every workspace's dataset back into place before
  reloading. Find it as "Restore from a backup fileâ€¦" (aliases restore/import/recover). Covered by a new
  `restore_backup_check` e2e (export â†’ tamper â†’ restore â†’ assert it persisted across the reload).
- **"Back up everything" full-install export (L9).** A new command-palette action exports the whole install â€” every
  workspace's dataset, the workspace registry, and the device-local appearance side-state (theme/fonts/banner/prefs)
  â€” into one versioned `cashflux-backup.json` via the pure `backup` envelope, so moving to a new device is lossless
  rather than per-workspace. The active workspace's dataset is taken live so it's current even before the autosave
  flushes. Find it as "Back up everything" (aliases backup/everything/migrate/full). Covered by a new
  `backup_everything_check` e2e. (Restore/import is the file-picker half and lands separately.)
- **Mermaid diagram source generators (C70, foundation).** New pure, table-tested `internal/mermaid`: a label
  `Escape` (collapses whitespace, single-quotes embedded quotes, entity-escapes `<`/`>` so comparison operators
  survive while no raw HTML tag can form), a `Flowchart` builder (box/round/diamond nodes + labelled edges), and
  `FromWorkflow` (trigger â†’ optional condition diamond â†’ actions, with the condition's yes-path highlighted). The
  `ui.Mermaid` renderer + locally-bundled shim are the follow-up.
- **The product version is now shown in the UI (C80).** A new dependency-free `internal/version` package holds one
  source of truth (`var Version = "0.1.0"`, override-able at build time via `-ldflags -X`), surfaced as a small
  muted `v0.1.0` line at the foot of the navigation rail under the household card. Covered by a native `version`
  test and a `version_rail_check` e2e.
- **Cash-runway card on the Planning screen (L13).** A new card projects your accounts' liquid balance over the
  next 60 days against your scheduled recurring cash flows (via the pure `runway`/`cashflow` engines) and reports
  the first day it dips below an optional buffer â€” "Dips below your buffer on <date> â€” short $X" â€” alongside the
  starting balance and projected low. Short-term liquidity, distinct from the 12-month net-worth forecast above.
  Covered by a new `runway_check` e2e.
- **Tools rail sub-group data layer (C67, foundation).** The screen registry's `Route` gains a `SubGroup` field;
  the 11 Tools screens now declare one of four sub-sections â€” Plan & analyze, Bills & recurring, Data & import,
  Build â€” keeping rail membership registry-driven (B7). Table-tested that every Tools route maps to exactly one
  sub-group, non-Tools routes carry none, and the four partition all Tools routes. (The nested rail rendering is a
  follow-up over this data.)

### Fixed
- **Category form selects are now labelled (C63/B15).** The category type and parent pickers in both the add and
  inline-edit forms had no accessible name (only the parent carried a hover title), so screen readers announced
  unlabelled comboboxes. Added `aria-label`s to all four; covered by a new `categories_labels_check` e2e.
- **Documents importer account picker is now labelled (C49/B15).** The "import into account" `Select` on both the
  CSV-draft and receipt-import footers had no accessible name, so screen readers announced it as an unlabelled
  combobox. Added an `aria-label`; the CSV import flow is regression-covered by `story_documents_csv`.
- **Mermaid diagrams now match the app theme (C70/C69).** The diagram shim hardcoded a dark theme, which read poorly
  once light themes (Paper) lit the shell. It now picks Mermaid's "default" (light) theme when `data-theme="light"`
  and re-initialises on a theme change, so diagrams follow the active palette. Regression-checked by `mermaid_render_check`.
- **Upcoming bills now show urgency at a glance (C57).** A bill's "due today / in N days" line is now toned â€”
  danger when due today (or past), warn within three days â€” so an imminent payment stands out (colour + the
  existing wording, B15) instead of reading like any other row.
- **Light themes now light the shell, not just the cards (C69).** The Paper preset set light content tokens but
  the rail/header/dashboard stayed dark, because the `[data-theme="light"]` stylesheet override that re-skins the
  shell only fires off the `data-theme` attribute â€” which the theme engine never set. New pure, table-tested
  `Theme.IsLight()` (WCAG luminance of the base surface) lets `ApplyTheme` set `data-theme` from the theme's own
  tokens, so any light theme lights the shell. This is the immediate Paper unblock; rewiring the hardcoded shell
  literals to the engine's CSS vars and retiring the dual data-theme/`--accent` system are the later C69 steps.
  Covered by a new `theme_shell_skin_check` e2e.
- **Artifact upload/import failures are no longer silent (C66, reliability).** Both image upload and CSV import
  swallowed errors (`if err == nil`), so a failed save â€” very plausibly a localStorage-quota overflow, since the
  whole dataset is one blob â€” just made the file silently not appear. Both paths now surface the actual error in the
  app toast (and CSV parse errors too). Covered by a new `artifacts_error_check` e2e.

### Added
- **Recurring â†’ cash-flow runway bridge (L13, logic).** New pure, table-tested `internal/runway`: `Events(recs,
  from, days, rates)` expands the household's `domain.Recurring` cash flows into the dated `cashflow.Event`s that
  fall in a horizon (stepping each by its cadence, fast-forwarding a stale `NextDue`, converting amounts to the base
  currency with sign preserved), and `Project(...)` runs them through `cashflow.DailyBalances` to flag the first day
  the balance dips below a buffer. Bridges real recurring data to the cash-flow engine ahead of the runway card.
- **Staged workflow actions can be removed before saving (C65).** The action builder only ever added actions â€” a
  mistaken one meant starting the whole workflow over. Each staged action row now has a remove button. Covered by a
  new `workflows_staged_remove_check` e2e.
- **Live match-count preview when authoring a rule (C64).** As you type a rule's match phrase, the add form now
  shows "Matches N transactions" against your existing history, so you can trust a rule before saving it (rules
  already showed per-rule counts; this brings the same signal to authoring). Covered by a new
  `rules_live_count_check` e2e.

### Fixed
- **Category reassign-before-delete only offers same-kind targets (C63, correctness).** Deleting an in-use category
  let you reassign its transactions/budgets to a category of the *other* kind â€” e.g. moving an expense category's
  data onto an income category â€” a semantic/data-integrity hazard. The reassign picker now lists only categories of
  the same kind as the one being deleted (and labels the select). Covered by a new `categories_reassign_kind_check`
  e2e.

### Added
- **"Can I afford it?" check on the Planning screen (L8).** A new Planning card answers an affordability question
  from your own projected cash flow (deterministic, not an AI guess): enter a purchase amount, an optional "in N
  months" horizon, and an optional buffer to reserve, and it runs the tested `afford` engine against today's net
  worth and this month's net cash flow. It shows the projected balance and free-to-spend amount, then a verdict â€”
  it fits, or short by $X with "affordable in about N months at this pace" (or that the cash flow won't cover it).
  Covered by a new `afford_check` e2e.
- **Members show a colored initial avatar (C62).** Each member row now leads with a small disc carrying the
  member's first initial, tinted with their chosen color â€” more scannable and personable than the bare swatch.
  Decorative (the name still follows as text), so it's `aria-hidden`. Covered by a new `members_avatar_check` e2e.

### Fixed
- **Customize formats numbers instead of raw floats (C61).** The formula result and the available-variables
  reference printed raw floats (net worth as `354070`), jarring against the app's money formatting. They now
  thousands-separate with up to two trimmed decimals (`354,070`), matching the C2 style.
- **Imported draft rows pick a real category instead of free text (C60).** When reviewing extracted transactions
  before import, the category field was a free-text box, so the AI's guessed category (or a typo) could create an
  orphan category. It's now a select of existing categories, with the extracted value preserved as an option when
  it doesn't match one â€” keeping the import constrained to real categories.
- **The Insights "needs a key" hint now links to Settings (C59).** Both the Explain action and the Q&A box showed
  a dead-end "add your OpenAI key in Settings" sentence; it now includes a Settings button that navigates there in
  one hop (same dead-end fix flagged on Allocate, C54). Covered by a new `insights_keyhint_check` e2e.

### Added
- **Fuzzy keyword matching in the command palette (L14).** The Ctrl/âŒ˜+K palette now ranks commands with the tested
  `cmdmatch` engine instead of a plain substring filter: a query matches as a subsequence of a command's label or
  any of its keywords, best match first. Direct actions carry search aliases (New transaction â† add/new/expense,
  Export â† backup/download, the passcode commands â† lock/security, â€¦), so typing a verb like "add" surfaces the
  noun-labeled "New transaction". No new visible text (keywords are search-only). Covered by a new
  `palette_fuzzy_check` e2e.
- **Split gets select-all/clear and a result summary (C58).** For households with several members, the sharer
  picker now has Select-all and Clear buttons, and once an amount and sharers are set it shows a legible summary â€”
  "$X split among N â†’ $Y each" (with any rounding remainder the core hands the first sharer; weighted splits note
  "(weighted)"). Covered by a new `split_summary_check` e2e.

### Fixed
- **Bills rows use a collision-proof key (C57).** The bills list keyed each row by `AccountID` alone; a composite
  key (account + due date + name) removes the latent risk of two bills on one account colliding and a row being
  silently dropped by the keyed-list diff.
- **The Bills "Per year" figure is now cadence-correct (C57, correctness).** It was computed as the upcoming-total
  Ã— 12, which mixed cadences â€” a one-off sum of differently-recurring items (monthly liabilities, weekly/quarterly/
  yearly recurring) multiplied by 12 misstated the annual cost. A new pure, table-tested `bills.AnnualAmounts`
  annualizes each obligation by its own cadence (liability min payment Ã—12; recurring normalized weekly Ã—52 /
  monthly Ã—12 / quarterly Ã—4 / yearly Ã—1); the screen FX-converts and sums those.

### Added
- **Rule match-count + coverage preview on the Rules screen (L15).** The Rules screen now wires up the L15 preview
  logic: each rule row shows "Matches N transactions" (how many existing transactions its phrase hits) and the
  list card shows a coverage line "Your rules auto-file N of M transactions" â€” so you can see what a rule does to
  your data before hitting "Apply to existing". The counted text mirrors the engine (payee + description). Covered
  by a new `rules_preview_check` e2e.
- **Click a detected subscription to see its charges (C56).** A subscription's name is now a button that opens
  Transactions searched for that payee, so you can verify the auto-detection against the underlying charges â€”
  the same drill pattern as Budgets/Goals/Accounts (C30/C56). Covered by a new `subscriptions_drill_check` e2e.
- **Reports ranked lists now show proportion bars (C55).** The spending-by-category, top-payees and biggest-expenses
  lists were plain name + amount rows. Each row now carries a thin bar sized to its share of the list's largest
  value, so the distribution is scannable at a glance instead of having to read every figure. Covered by a new
  `reports_sharebars_check` e2e.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Allocate suggestions show their score once (C54).** Each ranked suggestion printed the score twice â€” in the
  row head and again as a "Score NN%" sub-line â€” with a hand-rolled `" Â· "` separator span before the breakdown.
  The duplicate sub-line and separator are gone; the score now lives in the head plus the labelled progress bar,
  and the returns/stability/liquidity breakdown is the lone sub-line. Covered by a new `allocate_score_check` e2e.

### Fixed
- **Planning number fields now have sensible constraints (C53).** The plans horizon (â‰¥ 1) and one-time-month
  (1..horizon) inputs, and the payoff calculator + debt-strategy money inputs (â‰¥ 0), were validated only after
  submit. They now carry `min`/`max` so bad values are caught at the field. Covered by a new
  `planning_constraints_check` e2e.
- **The to-do priority and due-date controls are now labelled (C52).** Both the add and inline-edit forms had a
  priority `Select` and a due-date input with no `aria-label` or visible label â€” invisible to screen readers. Each
  now has a visible label (via the shared `labeledField`) plus a matching `aria-label`. Covered by a new
  `todo_labels_check` e2e.
- **Overdue to-dos now stand out (C52).** An open task past its due date used to look identical to one due next
  month. Overdue tasks now render their due line in the danger tone with an explicit "overdue" word (colour + text,
  not colour alone), so a past-due item is obvious at a glance. Covered by a new `todo_overdue_check` e2e.

### Added
- **Per-category spend trend series for sparklines (L16, logic).** `internal/reports` gains pure, table-tested
  `CategoryTrends(txns, bounds, rates)` â€” one `CategoryTrend{CategoryID, Spend []int64, Total, DeltaPct, HasDelta}`
  per category, where `Spend` is the absolute expense for each consecutive bucket (oldest first, base currency,
  income/transfers excluded) using the same `bounds` convention as `IncomeExpenseSeries`. It carries each
  category's window `Total` and firstâ†’last percent change, sorted by `Total` descending â€” the data behind the
  "category trends (sparklines + biggest movers %)" report.
- **Year-end / tax summary report (L16, logic).** `internal/reports` gains pure, table-tested `YearTax(txns,
  year, start, end, rates)` returning a `YearTaxSummary` of per-category `{Income, Expense, Net}` rows plus
  headline `TotalIncome`/`TotalExpense`/`NetIncome` â€” the annual category totals you hand a tax preparer.
  Income and expense roll up in the base currency (FX-converted, transfers excluded); rows sort by largest net
  magnitude first; the half-open `[start, end)` bounds a calendar **or** fiscal year and `year` labels the header.
- **Click a goal's linked account to see its transactions (C51).** A goal linked to an account now shows that link
  as a clickable affordance that opens Transactions filtered to the account â€” the same drill pattern as
  Budgetsâ†’category and Accountsâ†’Transactions (C30/C50). It also splits the linked-account bit out of the run-on
  progress sub-line into its own element. Covered by a new `goals_drill_check` e2e.

### Fixed
- **Goal add / edit / contribute forms now have persistent visible labels (C51).** All three goal forms were
  placeholder-only (name, target, saved-so-far, owner/linked selects, date, contribute amount), the same systemic
  gap fixed for Accounts/Budgets. Each control is now wrapped in the shared `labeledField` helper. Covered by a new
  `goals_labels_check` e2e.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Completed goals now read as done at a glance (C51).** A goal's progress bar was always the same flat accent
  colour even at 100%. A reached goal's bar now uses a brighter success tone, so finished goals stand out from
  in-progress ones without reading the numbers. Covered by a new `goals_bar_tone_check` e2e.

### Added
- **Click a budget to see its transactions (C50).** A budget's title is now a button that opens Transactions
  filtered to that budget's category â€” the natural "why am I over?" drill-down, mirroring Accountsâ†’Transactions and
  the dashboard tile-click (C30). It reuses the persisted `txfilter` so the filter sticks, and is only clickable
  when the budget has a category. Covered by a new `budgets_drill_check` e2e.

### Fixed
- **Budget add and inline-edit forms now have persistent visible labels (C50).** Both budget forms were
  placeholder-only (name, limit, and the Category / Owner / Period selects), so labels vanished on input â€” the same
  systemic issue fixed for Accounts. Each control is now wrapped in the shared `labeledField` helper with visible
  text above it; the helper's hook class was generalised from `acct-field` to `labeled-field` since it's now shared
  across screens. Covered by a new `budgets_labels_check` e2e.
- **The inline account editor and the set-balance form now have visible labels too (C49).** Extends the add-form
  labelling to the per-row edit form and the "set balance" form, so every account field is self-describing in every
  entry path (not just when adding). Uses the same `labeledField` wrapper; the `accounts_labels_check` e2e now also
  opens a row's editor and asserts the labels render.
- **The Add-account form now has persistent visible labels (C49).** Every field in the add form was placeholder-only,
  so the label vanished once you typed (and several â€” APR, Liquidity, Stability, Due day â€” were cryptic empty number
  boxes). Each control is now wrapped in a labeled field with visible text above it (Name, Account type, Owner,
  Currency, Opening balance, and the type-specific fields), via a small `labeledField` helper; the wrapping `<label>`
  also associates the text with its control for screen readers. Covered by a new `accounts_labels_check` e2e.
- **Account number fields now have ranges and clearer hints (C49).** The Liquidity and Stability score inputs (both
  the add form and inline edit) are constrained to **1â€“5** with `min`/`max`/`step` and a visible `(1â€“5)` hint, and
  the Due day field is constrained to a valid **1â€“28** day-of-month; the money fields (credit limit, APR, minimum
  payment) get `min="0"` so negatives can't be typed. Removes the guesswork from those bare number boxes. Covered
  by a new `accounts_field_constraints_check` e2e.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Dashboard now uses a semantic type scale instead of ad-hoc pixel sizes (C48).** The dashboard scattered
  `text-[11px]/[12px]/[13px]/[22px]/[24px]/[34px]` with no shared scale, so sizing was inconsistent tile-to-tile.
  Replaced all 27 sites with four rem-based tokens â€” `.t-caption` (labels/captions), `.t-body` (tile body),
  `.t-figure` (the single primary data-figure size), `.t-figure-lg` (the hero figure). KPI, net-worth-trend and
  goal figures that were 22/24px ad hoc now share one primary size, so the figure hierarchy is consistent (hero
  34px â†’ primary 24px â†’ body 13px â†’ caption 12px). rem keeps the whole scale tracking the display-scale zoom.
  Covered by a new `dashboard_typescale_check` e2e (figure sizes + no leftover ad-hoc px in the bento).
- **The transactions filter toolbar is now a portable, reusable `uiw.FilterToolbar` widget (C47, refactor).** The
  compact search + Filters-popover + removable-chips UI was extracted out of `transactions.go` into
  `internal/ui/filtertoolbar.go`, mirroring how `uiw.DataTable` owns the ledger table â€” so any screen with filters
  (Budgets, Accounts, Reports, â€¦) can reuse it. It's screen-agnostic: callers pass the search wiring, the popover
  field controls, a `[]uiw.Chip` active-filter list, and the handlers; the component owns the popover open/close
  state and the count badge internally. The transactions-specific `FilterChip` and inline toolbar assembly are
  gone. Behavior is unchanged (e2e green).
- **Transactions filters are now a compact toolbar with a Filters popover and removable chips (C47, completes it).**
  The cramped 10-control `form-grid` strip is replaced by an always-visible search box, a **Filters** button
  badged with the active-filter count, and **Clear** / **Export CSV** beside it. The button opens a `FlipPanel`
  popover holding account / category / member / date-range / cleared (each a labelled field; filters still apply
  live). Active filters render below as **removable chips** (âœ• clears just that one, with a "Clear all filters"
  link), so it's obvious what's narrowing the list. The summary line and the screen-reader live region are kept.
  Built over the `txnfilter` active-filter logic; the sortable table + pagination were already in place, so C47 is
  now complete. Covered by a new `story_txn_filter_toolbar` Playwright e2e (badge, popover, chip removal, clear-all);
  full suite green.

### Added
- **Richer rule match conditions (L15, logic).** `internal/rules` gains a pure, table-tested `Condition`
  (`AllKeywords`/`AnyKeywords []string`, `AccountID`, `MinAmount`/`MaxAmount int64`) matched against a minimal
  `TxnView{Text, AccountID, Amount}`. `(Condition).Matches` AND-composes only the parts that are set: every
  `AllKeyword` must appear, at least one `AnyKeyword` must appear (case-insensitive substrings, blanks ignored),
  the account must match when scoped, and the absolute amount must fall in the inclusive `[Min, Max]` range
  (0 = unbounded) â€” a zero-value `Condition` matches everything. Additive: the shared `Rule` type is untouched.
- **Rule match-count preview + coverage stats (L15, logic).** `internal/rules` gains pure, table-tested
  `(Rule).MatchCount(texts)` â€” how many existing transactions a rule would hit, the "matches N existing
  transactions" preview before a blind Apply-to-existing â€” plus `Covered`/`Uncovered(rules, texts)` for a "N of
  M auto-file by your rules" coverage signal (texts are each transaction's payee + description).
- **Fuzzy command-palette match with keyword aliases (L14, logic).** New pure, table-tested `internal/cmdmatch`
  package: `Command{ID, Title, Keywords}` + `Match(query, cmds)` ranks commands by a case-insensitive
  subsequence score over the title **and** each keyword â€” so a verb query like "add" or "export" surfaces a
  noun-labeled command ("New transaction"). Title matches outrank keyword-only matches, a prefix beats a
  scattered match, an empty query returns all in order, and ties keep input order.
- **Forward daily cash-flow projection + overdraft warning (L13, logic).** New pure, table-tested
  `internal/cashflow` package: `DailyBalances(startBal, events, days, buffer)` projects an account's running
  balance day by day from upcoming bills + paychecks and returns the daily series, the lowest balance and when
  it hits, and the **first day the balance dips below the buffer** (overdraft when buffer is 0) with the
  shortfall â€” the safety net for living paycheck-to-paycheck ("Checking dips to -$240 on Jul 2").
- **Full-backup envelope for lossless migration (L9, logic).** New pure, table-tested `backup.Envelope` (with
  `MarshalEnvelope`/`UnmarshalEnvelope`/`IsEnvelope`): a versioned "back up everything" container holding every
  workspace's dataset, the workspace registry, and the device-local appearance keys (theme/fonts/banner/prefs) â€”
  not just the active workspace's dataset that "Export JSON" carries today, which silently drops the rest. The
  round-trip is deep-equal lossless; `IsEnvelope` lets an import tell a full backup from a single dataset.
- **Grounded affordability check (L8, logic).** New pure, table-tested `internal/afford` package: `CanAfford`
  projects the balance to a target date from the steady monthly net cash flow, subtracts what's reserved
  (commitments / safety buffer / goal contributions), and returns whether the amount fits plus the projected
  balance, what's available, any shortfall, and the months until it becomes affordable at the current rate â€” so
  an "Can we afford $X by [date]?" answer can show the math rather than guess.
- **Suggested starter questions for the Insights Q&A (L8).** The "Ask about your money" box now offers up to four
  **tappable starter questions** above it â€” tailored to the user's top spend category ("How much did we spend on
  Housing last month?") with generic fallbacks â€” so a blank box never stalls the user; tapping one fills the box.
  Backed by a pure, table-tested `insights.SuggestedQuestions` (deterministic, de-duplicated, never empty), with
  the chips also acting as a compose aid on the no-key preview path. Covered by a Playwright story.
- **Active-filter introspection for the transactions toolbar (C47, logic).** `txnfilter.Criteria` gains a pure
  `ActiveFilters()` (the engaged filters in toolbar order â€” search, account, category, member, from, to, cleared;
  whitespace-only values and sort/direction/pagination never count), `ActiveCount()` for the "Filters" trigger
  badge, and `Without(field)` to clear one filter when its chip âœ• is clicked (sort, direction and page size are
  preserved; removal is a scope change so the page resets on re-apply). Table-tested. This is the logic layer for
  C47's remaining piece â€” replacing the cramped 10-control filter strip with a compact toolbar + Filters popover +
  removable chips (UI to follow).

### Fixed
- **A wiped store stays empty instead of re-seeding the sample household (L6).** Boot used to re-seed the sample
  whenever the dataset key was empty/missing, so wiping your data (or any genuinely empty store) brought a
  stranger's finances back on the next reload â€” a clean slate was unreachable. Boot now records a `cashflux:seeded`
  flag and only seeds the sample on a **true first run** (never seeded); once seeded, an empty dataset is treated
  as an intentional clean slate and preserved. The decision is a pure, natively-tested `decideHydrate`
  (first-run â†’ seed, saved dataset â†’ import, empty-after-seed â†’ stay empty), and an e2e proves wipeâ†’reload stays
  empty while a genuine first run still seeds.
- **The boot splash fully dismisses instead of lingering over the app (L12 root-cause; clears L1/L2/L3/L6/L11).**
  The "Getting your money in orderâ€¦" splash (`#boot`, a full-viewport `position:fixed; z-index:10` overlay) was
  only faded out via a CSS opacity transition once the app rendered â€” so a slow or interrupted transition could
  leave it stuck translucent over the content (seen on /planning, /split, /goals, /documents). It now also drops
  out of the layer (`display:none`) once faded (via `transitionend` plus a fallback timeout), checks for content
  already mounted before the observer attaches, and has a safety timeout so a re-mount can't outrace it. Guarded
  by a new `splash_dismiss_check` e2e across all four routes.
- **Net worth no longer silently miscomputes when an FX rate is missing (L4, determinism rule).** Previously a
  single account in a currency with no exchange rate made the whole net-worth roll-up return an error that the
  screens discarded â€” collapsing the entire figure to zero. Now a new `ledger.NetWorthExplained` **excludes** any
  rate-less account from the totals and reports which currencies/accounts it dropped, and both the Accounts
  net-worth header and the Dashboard net-worth tile show a notice ("Net worth excludes 1 account â€” no GBP rate.
  Add it in Settings"). A rate-less balance is never treated as base or zero. Table-tested (asset, liability, and
  all-rates-present cases) and covered by a Playwright story.
- **"Snap a receipt" opens the camera on mobile (L3).** The Documents image picker set `accept="image/*"` but no
  `capture` attribute, so on a phone â€” the primary device for photographing a receipt â€” it opened the file
  browser instead of the camera. It now sets `capture="environment"` to ask for the rear camera directly;
  desktop browsers ignore it and still show a file picker.
- **Budget row sub-lines no longer glue together (L1).** A budget row stacks several status lines â€” the
  status (`Monthly Â· On track Â· 79% Â· $61.00 left`), the pace heads-up, the rollover carry, and the envelope
  balance â€” but `.budget-sub` was inline, so adjacent lines ran into each other (`â€¦$61.00 leftAt this paceâ€¦`).
  `.budget-sub` is now block-level with a little top margin, so each line sits on its own row. Screenshot-confirmed.
- **CSV import of the documented shape actually imports (C27 follow-up).** Pasting the importer's own documented
  `date,payee,amount,account` format reported "Imported 0 transactions" because the payee column filled
  `Transaction.Payee` while the ledger requires a description â€” every row failed validation silently. The CSV
  parser now falls back to the payee for the description when no `desc` column is present (an explicit `desc`
  still wins), so the documented shape imports as intended. Caught by the new B16 documents-CSV E2E story;
  guarded by table tests in `internal/store`.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Radiogroups use roving tabindex for proper keyboard navigation (L7, a11y).** The Segmented control
  (Week/Month/Quarter, etc.) and the accent SwatchPicker followed the ARIA `radiogroup` role but made *every*
  option a Tab stop. They now use a roving tabindex â€” exactly one Tab stop (the checked option, or the first when
  none is checked) with the rest `tabindex=-1` â€” and arrow keys move selection (which follows focus) within the
  group; the SwatchPicker gained the arrow-key navigation it was missing. Locked in by a new `roving_tabindex_check`
  e2e (one Tab stop per radiogroup, and the checked option is that stop) across the period control + Settings.
- **Committed a11y sweep gate (L7).** A new `a11y_check` e2e asserts, across /transactions, /accounts, /budgets,
  and /goals, that the `nav` + `main` landmarks are present and that **every** visible focusable control and form
  field has an accessible name (computed from aria-label/labelledby/title/associated-label/text/placeholder) â€” so
  unlabeled-control regressions fail CI. (The sweep also surfaced unlabeled FX-rate inputs in the Settings panel;
  that screen is owned by a separate work stream, so its fix + sweep is left to that owner.)
- **Transactions ledger is now a sortable table (C47).** The flat row list is replaced by a semantic `<table>`
  with aligned columns (select Â· Date Â· Description Â· Category Â· Account Â· Tags Â· Amount Â· Cleared Â· Actions),
  right-aligned tabular amounts, and **click-to-sort column headers** (real buttons that sort by Date/
  Description/Category/Account/Amount and flip direction on a second click, with a caret and `aria-sort` on the
  active column). Category and account sort by display name. The standalone Sort dropdown is gone. Every
  behavior is preserved â€” inline edit, duplicate, delete (+transfer pair), per-row select + bulk actions,
  dedupe notice, persisted filters, CSV export, the a11y live region â€” and the table collapses to stacked cards
  on narrow screens. (The compact filter toolbar lands next.)
- **One appearance system â€” density & display scale unified into the theme (B20).** Density and text size are
  now owned by the theme engine: `ApplyTheme` sets `data-density` (so the editor's density control is finally
  live) and `--ui-scale`, while `ApplyPrefs` no longer touches either (no more two systems fighting over the
  same vars). The duplicate legacy "Compact density" toggle and "Display scale" select were removed from
  Settings; the theme editor's density and text-size controls are the single source of truth, and they mirror
  back into `prefs.Compact`/`Scale` so migration and any prefs reader stay consistent. Verified via Playwright
  (editor density sets `data-density`, text size drives `--ui-scale` and syncs prefs; legacy select gone).
- **Fonts are now themeable (B20).** The app consumes the `--font-ui` / `--font-display` theme tokens: the
  Tailwind `font-sans` / `font-display` families and the base `body` / heading styles lead with `var(--font-ui)`
  / `var(--font-display)` (with the Inter/Fraunces stack as fallback), and `:root` carries static defaults. So
  choosing an interface or heading font in the theme editor now actually changes the app's type. Verified live
  via Playwright (switching the interface font changes the body's computed font-family).

### Added
- **Payoff progress tracking against a baseline (L5).** A **"Start tracking progress"** button snapshots today's
  total debt; the debt card then shows a progress strip â€” **"Paid off $X of $Y (NN%) since <date>"** with a bar â€”
  that updates as balances fall, and a **Reset** to start over. Backed by a pure `payoff.TrackProgress`
  (paid-off clamped at â‰¥ 0, table-tested) and an additive `store.Settings.PayoffBaseline` (persisted + round-trip
  tested) with `appstate.StartPayoffTracking`/`ClearPayoffTracking`/`PayoffProgress`; covered end-to-end by a
  Playwright story (start tracking â†’ strip shows â†’ survives reload). This completes the L5 "Debt Crusher" story.
- **Debt payoff burn-down chart (L5).** The debt-strategy card now draws an area chart of the remaining total
  balance falling to zero across the payoff timeline, so the plan reads at a glance. Backed by an additive
  `payoff.Plan.Schedule []int64` (the remaining balance at the end of each month, ending at 0) populated by
  `BuildPlan` and table-tested (length == Months, ends at 0, non-increasing).
- **Exclude a debt (the mortgage by default) from the payoff plan (L5).** Real debt-crusher plans target
  revolving/consumer debt, so a 30-year mortgage no longer dominates the timeline: each liability with a balance
  now has an **"include in payoff plan"** toggle in the debt-strategy card, and a **mortgage is excluded by
  default** (any liability can be toggled either way; the choice persists). Backed by an additive
  `domain.Account.IncludeInPayoff *bool` (nil = default-exclude mortgages, set = explicit) + an
  `IncludedInPayoff()` helper, table-tested, with a store round-trip test and a Playwright story (toggle a debt
  out â†’ it leaves the payoff order and the flag survives reload).
- **Debt strategy suggests a starting extra so the comparison is meaningful (L5).** At $0 extra, snowball and
  avalanche are identical, so the strategy card now prompts "At $0 extra the strategies tie" with a one-tap
  **"Try $X/mo"** button (a quarter of the total minimum payments, or 1% of balance when minimums are unknown â€”
  pure `payoff.SuggestedExtra`, table-tested) that fills a sensible amount; and when the two strategies are truly
  identical it explains why ("Snowball and avalanche match here â€” add an extra monthly amount above to see them
  diverge"). Covered by a Playwright story.
- **Debt payoff shows a calendar debt-free date, not just a month count (L5).** The debt-strategy card now reads
  "Debt-free by Nov 2035 (snowball) Â· â€¦ (avalanche)" beside the month totals, and the payoff order dates each
  debt as it clears ("Auto Loan (Aug 2027) â†’ Credit Card (Jan 2028)"). Backed by a pure `payoff.DebtFreeMonth`
  (month-count â†’ calendar month) and a new `Plan.ClearedMonths` exposing when each debt is paid off from
  `BuildPlan`; both table-tested, and the card's extra-payment input gained an accessible label. Covered by a
  Playwright story.
- **Account currency is a validated picker, not free text (L4).** Adding an account now chooses its currency from
  a labelled dropdown ("EUR â€” Euro") sourced from the known ISO registry plus any code already in play (the base
  currency and the FX-table currencies), defaulting to the household base â€” so an expat picks EUR/GBP without the
  typos or lowercase codes that used to silently break conversion. Backed by new pure `currency.Valid(code)` and
  `currency.List()` (table-tested), and covered by a Playwright story (add a EUR account â†’ persists as `EUR`,
  survives reload).
- **Receipt vs Statement import toggle on the Documents review (L3).** After reading an image with AI, the
  review now offers **"Import as one receipt (split across categories)"**. In receipt mode the extracted lines
  become the category splits of a single transaction: enter the store name + receipt total (pre-filled to the
  line sum), and a live remainder shows whether the lines add up â€” **Import is disabled until they reconcile to
  the total**, then imports one split transaction via `appstate.ImportReceipt` (mapping each line's category +
  applying rules). Statement mode keeps the existing many-transactions path. (The dedicated receipt-flow e2e +
  screenshot is deferred behind an image-picker DOM refactor + mocked vision response; the import logic itself is
  fully unit-tested.)
- **Import a receipt as one split transaction, with category mapping + rules (L3).** `appstate.ImportReceipt`
  turns a reconciled `extract.Receipt` into a single expense transaction whose category splits sum to the total
  (so it counts once against the account yet reports per-category spend). Each line's free-text category is
  resolved to a real category â€” the extracted per-line category by name first (exact, then fuzzy substring), then
  a fallback through the user's auto-categorization rules on the line + merchant (so a "Costco â†’ Groceries" rule
  still applies). Amounts import as expenses (negative), a single-category receipt also tags the transaction, and
  non-reconciling / account-less imports are rejected. Unit-tested (name mapping, merchant-rule fallback,
  validation).
- **Transaction category-split model (L3).** `domain.Transaction` gains an additive `Splits []CategorySplit`
  (`omitempty`) so a single bank charge can carry a per-category breakdown â€” a grocery receipt counts once
  against the account yet reports produce/dairy/household spend separately. Pure helpers `SplitsTotal`,
  `SplitsReconcile` (splits must sum to the amount to the minor unit; an unsplit transaction reconciles
  trivially), and `Transaction.HasSplits()`/`SplitsReconcile()`. The field rides the existing transactions JSON,
  so it survives a store export/import round-trip with no schema change. Table-tested (including discount lines)
  plus a store round-trip test.
- **Receipt-mode logic â€” one charge split across categories (L3, bottom-up start).** `internal/extract` gains a
  `Receipt` (a single total plus categorized `ReceiptLine` splits) distinct from a statement: a statement is many
  charges â†’ many transactions, a receipt is one charge â†’ one transaction split across categories (so importing a
  grocery receipt no longer double-counts against the single card charge or breaks dedupe). `ReceiptFromRows`
  turns extracted vision rows into a receipt (defaulting the total to the line sum), and `Residual`/`Reconciles`
  check that the splits sum to the total to the cent. Tolerates the `$`/comma formatting models emit. Table-
  tested: reconcile, short/over remainder, discount (negative) lines, currency-symbol parsing, and unparsable
  amounts.
- **Split screen "Settle up" panel â€” who owes whom across every saved split (L2).** The Split calculator can now
  **Save split** (with an optional "what was it for?" note), recording it as a shared expense. A new **Settle up**
  card then shows the running balance across every saved split â€” each member's net ("is owed $X" / "owes $X") â€”
  plus the **simplest way to square up** (the minimal set of "X pays Y $Z" payments) with a per-payment **Record
  settlement** button. Recording a payment re-balances the ledger immediately (and reads "All settled up" once
  everyone is even); it all persists across reloads. Covered by a new Playwright story (three expenses with
  different payers net to a single Leeâ†’Priya payment; recording it squares everyone up and survives reload).
- **App state for the settle-up ledger (L2).** `appstate` gains `SharedExpenses()`/`Settlements()` accessors,
  validated `PutSharedExpense`/`RecordSettlement` write actions (and their deletes), and a `SettleUp(currency)`
  helper that builds the pure `settle` inputs from the persisted records and returns each member's net balance
  plus the minimal set of transfers to zero everyone out. Unit-tested end to end (persist a 3-way split â†’
  net + minimal transfers; record a settlement â†’ the ledger re-balances), with validation rejections covered.
- **Shared expenses + settlements are first-class persisted records (L2).** New `domain.SharedExpense` (a cost
  fronted by one member with per-member shares) and `domain.Settlement` (a payment squaring members up) are now
  stored in their own SQLite tables and carried in the exported `Dataset`, so the roommate settle-up ledger
  survives reload and round-trips losslessly through export/import. Full CRUD on the store
  (`Put/Get/Delete/List SharedExpense` and `â€¦Settlement`) plus a `SharedExpense.Total()` helper, covered by
  round-trip and CRUD tests.
- **Settle-up logic for shared expenses (L2, bottom-up start).** A new pure `internal/settle` turns a set of
  shared expenses (who paid + each member's share) and any recorded settlements into each member's **net
  balance** (positive = the group owes them) and a **minimal set of "X pays Y $Z" transfers** that zero everyone
  out (greedy largest-debtor-pays-largest-creditor, at most nâˆ’1 transfers, deterministic by member ID). All
  arithmetic is on integer minor units, so no cents are lost or created; a `SplitEqually` helper divides a total
  into shares that sum exactly to it (remainder cents handed to the first members in order). Table-tested:
  three-way uneven shares, a partial settlement, a fully-balanced group (zero transfers), an already-settled net
  (empty), and the equal-split remainder distribution.
- **"Coverâ€¦" an overspent budget from the Budgets screen (L1).** An over-budget row now offers a **Coverâ€¦**
  action that opens a small inline form: pick a funding budget (each labelled with its remaining room), an
  amount prefilled to the exact overspend (with a one-tap "Full $X" button), and apply â€” moving budgeted money
  from the source's limit into the over budget without changing the household's total. The move persists and
  survives reload. Backed by a new `appstate.CoverBudget(fromID, toID, amount)` action (applies the pure
  `budgeting.Transfer`, persists both budgets, and refuses to drain a source below a valid limit), unit-tested,
  and covered end-to-end by a new Playwright story (overspend Groceries â†’ cover $50 from Shopping â†’ both rows
  re-balance and survive a reload).
- **Inter-budget transfer logic â€” "cover overspending" (L1, bottom-up start).** `internal/budgeting` gains a
  pure `Transfer(from, to, amount, allowNegativeSource)` that moves budgeted money from one budget's limit to
  another's. It is **balanced** (the household's total budgeted amount never changes) and **explainable** â€” the
  returned `TransferResult` records both legs (each budget's limit before/after) so the UI can show exactly what
  changed. The source limit cannot go negative unless explicitly allowed; same-budget, non-positive, and
  cross-currency moves are rejected with sentinel errors. A companion `CoverAmount(Status)` returns the exact
  shortfall to clear an overspend (the default for a "cover the full $X over" one-tap). Table-tested, including
  overspend-cover, exact-to-zero, insufficient-source (allowed and rejected), no-input-mutation, and the
  balanced-total invariant.
- **Reusable `DataTable` component + ledger pagination bar (C47).** A new generic `internal/ui` `DataTable`
  owns the table chrome â€” semantic `<table>`, click-to-sort column headers (with `aria-sort` + caret), and an
  optional pagination footer â€” while each screen still renders its own body rows, so it can be reused across the
  app (accounts, categories, etc.) instead of being hardcoded in the transactions screen. The transactions
  ledger now consumes it and gains a real pagination bar: **Prev / Next** (disabled at the ends, aria-labelled),
  a "1â€“50 of N" position label, and a **Rows per page** select (25 / 50 / 100 / All) â€” replacing the old
  "Show more" button. The page and page size persist in the saved filter, the page clamps to range, and changing
  any filter or sort resets to page 1. Backed by `internal/pagination` (window math) and verified via Playwright
  (the pager renders "Prev Â· 1â€“50 of 57 Â· Next Â· Rows per page" with no console errors).
- **Sortable-column logic for the ledger (C47, bottom-up start).** `internal/txnfilter` gains an explicit sort
  **direction** (`asc`/`desc`) and three new sort keys â€” **category** and **account** (name-aware via a new
  `ApplyWithLabels` that takes idâ†’name maps) on top of date/amount/payee â€” with per-key default directions
  (date/amount lead descending, text columns ascending) and deterministic ID tie-breaking. Table-tested for
  every key Ã— direction; this is the pure foundation for the upcoming click-to-sort table headers. New pure
  `internal/pagination` provides the page window math â€” total pages, page clamping, slice bounds, a generic
  `Slice`, and the "from-to of total" `Window` (with a "show all" mode) â€” also table-tested. The ledger filter
  state (`txnfilter.Criteria`) now also carries the persisted **page** and **page size** (defaults to 50, with
  a "show all" sentinel), plus a `ScopeChanged`/`ResetPageIfScopeChanged` rule that snaps back to page 1 when
  the filters or sort change â€” all table-tested.
- **E2E stories (B16).** Scripted user-journey tests, now that Playwright + Chromium are installed â€” each
  asserts the standard path end-to-end (UX + data correctness + persistence across reload): **add a
  transaction** (logs an expense, sees it in the ledger with its amount, autosaved), **add an account**
  (adds an asset with an opening balance, sees it listed and the net-worth summary rise by exactly that
  balance), **create a budget** (adds a Weekly budget, sees it listed with its limit, and confirms the
  saved budget carries the chosen period), **create a goal + contribute** (adds a goal, contributes to it,
  and confirms the saved amount advances and persists), **settings exportâ†’import round-trip** (exports the
  dataset, imports it back, re-exports, and proves the round-trip is lossless â€” same entities preserved), and
  **transactions filter persistence** (filters the ledger to a unique transaction, confirms the list narrows
  to the one match, and that the filter and narrowed view survive a reload), and **reconcile / cleared**
  (toggles a transaction's cleared status, confirms the cleared-status filter includes/excludes it and that the
  flag persists), and **to-do complete-toggle** (adds a task, marks it complete, and confirms the status flips
  to done and persists), and **category reassign-on-delete** (assigns a transaction to a category, deletes the
  category choosing a reassignment target, and confirms the transaction moves to the target with no orphan),
  **member reassign-on-delete** (gives a member an account, deletes the member choosing a new owner, and
  confirms the account is reassigned with no orphan), and **transfer excluded from totals** (transfers between
  two accounts and confirms the paired legs are created while the Income/Spending KPIs stay unchanged), and
  **account archive + restore** (archives an account via its row menu and restores it, confirming the archived
  flag round-trips), and **sub-category nesting** (adds a parent and a child category and confirms the child
  links to the parent while the parent stays top-level â€” the linkage the tree rollup is built on), and
  **duplicate a transaction** (duplicates a ledger row and confirms a standalone copy is created â€” two rows,
  neither a transfer leg), and **set the default member** (marks a member the default and confirms exactly one
  member is flagged default), and **bulk clear** (selects two transactions and marks them cleared in one bulk
  action, confirming both flip to cleared), and **allocate exclude/restore** (excludes a ranked allocation
  suggestion and confirms it leaves the active list and can be restored), and **planning recurring item**
  (adds a recurring cash-flow and confirms it lists, persists, and survives a reload), and **customize formula**
  (types an arithmetic expression, confirms the live result, and saves a formula that persists), and
  **documents CSV import** (pastes a CSV row and confirms the transaction it describes is imported into the
  chosen account and persisted).
  The
  start of B16's "every feature, provably flawless" story suite (`e2e/story_*.test.mjs`). The whole suite â€”
  every story plus the feature checks (theme/fonts/banner/icon-weight/density-unify/per-widget-color) â€” now
  runs as one command â€” `e2e/run-stories.ps1` (Windows) or the cross-platform `e2e/run-stories.mjs` (Node,
  CI-friendly: builds the wasm + serve binary, runs each `.mjs` in a fresh browser, exits non-zero on any
  failure): currently **29 green**.
- **Per-widget colors (B20).** Each dashboard tile can now be tinted with its own accent: open the tile's
  settings (every tile shows a gear now) and pick a "Tile color" â€” it paints a colored strip across the top of
  that tile, stored per-widget and reversible with Clear. The color is validated (a bad hex is ignored) and
  kept in the widget config under a reserved key, so it survives reloads and travels with the widget settings.
  Verified end-to-end via Playwright (set tints the tile and persists; Clear reverts).
- **Remove uploaded fonts (B20).** Each uploaded custom font now lists in the theme editor with a Remove
  button. Removing it drops the font from storage, clears its `@font-face`, and â€” if the active theme was using
  it â€” falls back to a curated font (Inter/Fraunces) so nothing points at a missing face. Verified end-to-end
  via Playwright (upload â†’ row + Remove appear â†’ remove clears store, face, and falls back).
- **Selectable icon weight (B13).** The theme editor gains an "Icon weight" control (Thin / Regular / Bold) â€”
  `ui.Icon` now draws every glyph at the theme's `--icon-stroke` width, so the whole curated icon set thins or
  thickens together, live and persistent. Verified via Playwright (Bold takes a rail icon from 1.6px â†’ 2.2px,
  persists; screenshot confirms).
- **Icon stroke weight token (B13/B20).** New pure `theme.IconStroke` (SVG line thickness, default 1.6)
  carried through `Default`/presets/migration, validated (1.0â€“3.0), merged, and emitted as the `--icon-stroke`
  CSS var. Table-tested foundation for a selectable icon weight; the renderer wiring and editor control follow.
- **Dashboard banner (B20).** The theme editor gains a "Dashboard banner" section: pick a built-in gradient
  (Aurora / Sunrise / Forest / Slate) or upload your own image (PNG/JPEG/WebP/GIF, â‰¤2 MB), with a one-click
  remove. The chosen banner shows as a decorative full-width band above the dashboard bento grid, stored in its
  own `cashflux:banner` slot and applied at boot. It's purely decorative (no essential text on it), so it can't
  hurt legibility. Verified end-to-end via Playwright (preset activates the band, persists, removes cleanly;
  screenshot confirms the band renders).
- **Banner image logic (B20).** New pure `theme.Banner` (none / built-in gradient / uploaded image) with
  `CSS()` (the `background-image` value), built-in gradient presets (`BannerPresets`), and image-upload
  validation â€” `ValidateImageUpload` (PNG/JPEG/WebP/GIF up to 2 MB), `ValidImageMIME`, and `ImageMIMEForName`
  (extension fallback). Table-tested foundation for the dashboard header band; the UI follows.
- **Upload your own font (B20).** The theme editor now has an "Upload font" button that accepts a WOFF2/WOFF/
  TTF/OTF file (â‰¤1 MB): it's validated, stored as a data URL in its own `cashflux:fonts` slot, registered via
  an injected `@font-face` rule, added to the interface/heading font pickers, and applied immediately. Uploaded
  fonts are registered at boot too, so a theme that selects one renders correctly on reload. Verified
  end-to-end via Playwright (upload â†’ @font-face injected, persisted, selected, applied; no console errors).
- **Custom-font upload logic (B20).** New pure `theme.FontAsset` (family + MIME + data URL) with
  `FontFaceCSS` (renders an `@font-face` rule with a `format()` hint and `font-display: swap`),
  `ValidateFontUpload` (accepts WOFF2/WOFF/TTF/OTF up to a 1 MiB cap, rejects other formats / empty /
  oversize), and `FontMIMEForName` (recovers a MIME type from the file extension when the browser reports
  none). Table-tested foundation for letting users bring their own font; the upload UI and live `@font-face`
  injection follow.
- **Shareable theme import/export (B20).** The theme editor can now export the active theme to a
  `cashflux-theme.json` file and import one back, so themes are portable between devices and people. Import
  validates the file and shows a friendly inline message if it isn't a valid theme.
- **Theme editor in Settings â†’ Appearance (B20).** A new live theme editor lets you start from a built-in
  preset (Forest / Midnight / Paper), then fine-tune every design token â€” the eight surface/text/accent/
  semantic colors via native color pickers, corner radius, text-size scale, the interface and heading fonts
  (curated list), and density. Every change applies and persists instantly, with a live contrast check that
  warns if any text would be hard to read, plus a one-click "Reset to default" that restores the theme
  migrated from your display preferences. Verified in-browser via Playwright (renders, live-applies, no
  console errors).
- **Theme tokens drive the live UI (B20).** New wasm `uistate.ApplyTheme/LoadTheme/PersistTheme` bridge the
  pure `theme` engine to the document: `ApplyTheme` writes a theme's design tokens onto `:root` as CSS custom
  properties (surfaces, border, text, accent, radius, fonts, scale, plus a `--bg` alias), `LoadTheme` returns
  the saved custom theme or â€” on a fresh install â€” one migrated from the display preferences (with `system`
  resolved to a concrete light/dark palette), and `PersistTheme` saves it. Applied at boot after `ApplyPrefs`;
  with no custom theme yet every token equals the stylesheet default, so the first application is invisible.
- **Theme migration from display preferences (B20).** New pure `theme.FromPrefs` upgrades the legacy
  theme/accent/density/display-scale preferences into a full `theme.Theme` of design tokens â€” the migration
  path for the unified appearance engine. It picks the dark or light surface palette to mirror today's live
  `web/index.html` colors exactly and overlays the user's accent, scale, and density, so moving the app onto
  the theme engine is a visual no-op until a token is edited. Table-tested (valid in both palettes, system â†’
  dark fallback, accent/scale/density overlay, minimum-zoom stays valid).

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Theme scale floor aligned to 70%.** `theme.Validate` now accepts a font-size scale down to 0.70 (was
  0.75) to match the display-scale preference's 70% minimum, so migrating a user at minimum zoom yields a
  valid theme.
- **Guard against blank icons.** A new `internal/icon` test asserts every curated glyph's markup contains a
  renderable `<path>/<circle>/<rect>` shape (and no element the renderer can't draw), so a new icon can't
  silently render blank again.
- **Workspace reorder buttons use arrow icons (C46).** The switcher's move-up/move-down controls use
  arrow-up/arrow-down glyphs (with aria-labels) instead of bare â†‘/â†“.
- **Reports change-% arrows use colored icons (C46).** The â–²/â–¼ delta markers on report rows are now
  arrow-up/arrow-down glyphs that inherit the up/down tone color.
- **Insights anomaly arrows use colored icons (C46).** The bare â†‘/â†“ direction markers on each spending
  highlight are now arrow-up/arrow-down glyphs that inherit the row's up/down tone color via currentColor.
- **Dashboard KPI tiles have leading header icons (C46).** Each tile header now leads with a glyph keyed off
  its stable id â€” Net worth (wallet), Income (down-circle), Spending (up-circle), Liabilities (credit-card),
  Recent (receipt), Budgets/Breakdown (pie), Goals (target), To-do (check), Accounts (landmark), Net-worth
  trend / cash flow (trending-up), Bills (calendar), Freshness (clock), Highlights (insights). Derived via a
  `widgetIcon(id)` map, so no per-tile wiring; user custom-page widgets stay icon-less.
- **First-run empty states show a friendly glyph (C46).** The CTA empty-state block (e.g. "Add your first
  goal") now renders a muted icon above the message â€” defaulting to a neutral box, with an optional per-screen
  `Icon` â€” so otherwise-blank panels feel intentional. Transient "no match" / "all done" lines stay text-only.
- **AI actions share a sparkle glyph (C46).** "Explain my month", "Ask about your money", and "Read with AI"
  now lead with the same `sparkles` icon, so AI affordances read as one consistent family.
- **Planning and Documents row actions have icons (C46) â€” completes the row-action icon pass.** The recurring/
  plan delete buttons, the import-row remove + edit, and delete-import-history now use pencil/`x` glyphs (with
  aria-labels). With this, every per-row Edit/Delete button across the screens reads from the typed icon set.
- **Custom fields, Customize, and custom-page widgets have action icons (C46).** Their delete buttons use the
  `x` glyph (with aria-labels), and the custom-page widget edit button uses a pencil â€” replacing the bare
  "âœ•"/"âœŽ" glyphs.
- **Artifacts and Workflows delete buttons use the x icon (C46).** Both screens' per-row delete now uses the
  `x` glyph (with an aria-label) instead of a bare "âœ•".
- **Budgets row actions have icons (C46).** Edit leads with a pencil and the delete button uses the `x` glyph
  (with an aria-label) instead of a bare "âœ•".
- **Transactions row actions have icons (C46).** Edit leads with a pencil and the delete button uses the `x`
  glyph (with an aria-label) instead of a bare "âœ•".
- **Insights unpin button uses the x icon (C46).** The pinned-insight remove button now uses the `x` glyph
  (with an aria-label) instead of a bare "âœ•".
- **Accounts row actions have icons (C46).** View-transactions leads with a list glyph, Edit with a pencil, the
  more-actions toggle uses more-horizontal, and the delete button uses the `x` glyph (with an aria-label).
- **Rules row actions have icons (C46).** Edit leads with a pencil and the delete button uses the `x` glyph
  (with an aria-label) instead of a bare "âœ•".
- **Categories row actions have icons (C46).** Edit leads with a pencil and the delete button uses the `x`
  glyph (with an aria-label) instead of a bare "âœ•".
- **To-do row actions have icons (C46).** Edit leads with a pencil and the delete button uses the `x` glyph
  (with an aria-label) instead of a bare "âœ•".
- **Members row actions have icons (C46).** View-transactions leads with a list glyph, Edit with a pencil, and
  the delete button uses the `x` glyph (with an aria-label) instead of a bare "âœ•".
- **Goals row actions have icons (C46).** Contribute leads with a plus-circle, Edit with a pencil, and the
  delete button uses the `x` glyph (with an explicit aria-label) instead of a bare "âœ•".
- **Re-render stress E2E guards against chrome duplication.** `e2e/rerender.test.mjs` fires many re-render
  triggers (rail collapse toggling, add-menu open/close, rapid same-route re-clicks, cross navigation, browser
  back/forward) and asserts exactly one rail / top bar / `<h1>` / `#app` subtree throughout â€” a standing guard
  for the "page duplicates on rerender" symptom (not reproducible via these paths, so the trigger is elsewhere).
- **Hardened sub-path routing with a deep contract test (B30).** `internal/routebase` gains a contract test
  that mirrors the full register â†’ match â†’ strip â†’ highlight cycle across several base prefixes (`""`,
  `/CashFlux`, `/app`, `/a/b`, `/Repo-Name`): route registrations stay unique under the prefix, the wildcard is
  never prefixed, `/p/:slug` round-trips, the default route resolves to the base root, and every live pathname
  strips back to its logical route for the active comparison â€” so GitHub Pages and other sub-path deploys stay
  navigable. Plus more `Normalize`/`Strip` edge cases (resolved absolute `<base href>`, multi-segment bases,
  look-alike-prefix safety).

### Fixed
- **Most icons rendered blank (the whole C46 pass was invisible).** `ui.Icon` drew shapes from a hardcoded
  switch that only covered ~16 of the curated glyphs, so every newly added icon â€” all the row-action, AI,
  KPI-tile, status, and even the Reports/Subscriptions/Bills/Split rail icons â€” rendered as an empty SVG.
  `iconBody` now renders each icon from `internal/icon`'s canonical `Inner()` markup (the single source for the
  whole set) by parsing its path/circle/rect elements, so every `icon.Name` paints â€” verified with screenshots.
- **Active rail highlight (and breadcrumb) didn't follow navigation.** The Sidebar/TopBar derived the current
  screen from a non-reactive `router.InspectCurrentRoute()` snapshot and, taking no props, were memoized â€” so
  the highlight froze on the first screen ("the menu item doesn't move"). Each route's logical path is now
  threaded from its factory through `ShellProps.ActivePath` to the rail and top bar, so the highlight and the
  breadcrumb "are-we-home" check react to every navigation. Verified by a Playwright E2E that clicks all 20
  rail items and asserts the URL, heading, exactly one active item, and exactly one rail/top bar.
- **Left rail items were not navigable (routing regression).** A Layout/outlet router restructure left child
  routes rendering outside the Shell (into a missing outlet), so clicking most rail items showed nothing.
  Reverted to flat per-route registration â€” each route renders its own Shell + screen, which the history router
  resolves to exactly one Shell (it only stacks routes registered as layouts, and none are). Added a
  `screens.TestRailRoutesResolve` registry-invariant guard plus a Playwright E2E that clicks every rail item.

### Added
- **Ad-hoc Unicode chrome glyphs replaced with real icons (C46).** The shared controls and chrome now use
  typed `ui.Icon`s instead of `â–¾ â€¹ â€º âœ• â‹¯ âš™`: the period stepper (chevron-left/right), the FlipPanel close (x),
  the dashboard widget gear (settings, plus its width-balancing spacer), the workspace switcher (chevron-down),
  and the "My pages" row menu (more-horizontal) â€” so the whole app reads from one consistent glyph family.
- **Server-advertised auth controls (7.12).** Settings now adapts backend auth controls from `/v1/version`,
  showing the printed-token field for token-mode self-hosting and only the advertised OAuth provider buttons
  for OAuth servers.
- **Quick-add menu now has leading icons (C46).** Each "+ Add" menu item shows its glyph â€” New transaction
  (arrow-left-right), New account (wallet), New budget (pie), New goal (target), Scan a document (scan-line) â€”
  so the menu is scannable by shape, not just text.
- **OAuth login UI and popup token handoff (7.7).** Settings now offers Google/GitHub backend sign-in, with
  OAuth callbacks posting the access token and CSRF value back to the app while preserving token-mode self-hosting.
- **Expanded the typed icon registry for the iconography pass (C46/B13).** `internal/icon` gains 35 curated
  Lucide-style glyphs the screens need â€” chevrons, close, more, check/alert/clock status marks, trending and
  arrow variants, edit/refresh/list/contribute actions, the AI sparkle + message glyphs, and domain accents
  (credit-card, receipt, landmark, filter, box, workflow, scale, repeat, calculator, scan-line, upload, history,
  ban, help). All compile-checked `Name` constants with table tests; the wasm `ui.Icon` already renders them.
- **Artifact blob refs for backend sync (7.7).** Synced datasets can now carry `Artifact.BlobRef` metadata while
  the wasm sync client uploads artifact bytes through `/v1/blobs` and rehydrates them on pull.
- **Client sync queue and status (7.7).** Browser autosave now persists the latest pending backend mutation per
  workspace, retries on focus/online/manual sync, and exposes sync status plus a Sync now action in Settings.
- **Blob bridge round-trip coverage (7.10).** Added an integration test that creates a workspace through the
  gRPC tunnel and verifies authenticated HTTP blob PUT, HEAD, and GET on the same backend server.
- **Two-device sync bridge e2e (7.3/7.10).** Added integration coverage for two devices connected through
  the real gRPC tunnel, proving stale LWW writes are rejected and delete tombstones propagate to watchers.
- **AI streaming RPC surface (7.1/7.4).** Added `ChatStream` and `VisionStream` server-streaming RPCs over the
  gRPC tunnel, returning terminal completion chunks while preserving the existing unary AI calls.
- **Proto codegen and drift check (7.0/7.1).** Added Buf-based generation for
  `proto/cashflux/v1/cashflux.proto`, checked-in Go/gRPC descriptors under `internal/backendrpc/pb`, and a CI
  drift check.
- **OTLP trace export (7.15).** The server now installs an OpenTelemetry SDK tracer provider when
  `CASHFLUX_SERVER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` is configured, exporting spans over OTLP/HTTP.
- **Device/session revocation endpoints (7.14).** Added `GET /v1/auth/sessions` and
  `DELETE /v1/auth/sessions/{family}` for user-scoped session-family listing and revoke, with CSRF on revoke.
- **Account export includes billing state (7.17).** Self-serve account export now includes the caller's current
  Stripe subscription identifiers/status without exposing any other user's billing rows.
- **Billing idempotency keys (7.16).** Stripe Checkout and customer-portal session endpoints now persist
  `Idempotency-Key` results per user/route/request hash and replay duplicate requests without a second Stripe call.
- **AI master-key rotation command (7.8).** Added `cashflux-server rotate-ai-master-key`, which re-encrypts
  stored AI keys from `CASHFLUX_SERVER_OLD_MASTER_KEY` to the current `CASHFLUX_SERVER_MASTER_KEY`.
- **AI upstream circuit breaker (7.16).** The backend AI proxy now opens a short fail-fast window after
  repeated upstream transport or 5xx failures, then resets after cooldown and a successful upstream response.
- **Backend load smoke coverage (7.18).** Added an in-process load smoke test covering concurrent sync pushes,
  workspace watch fan-out, and blob upload/download through the real HTTP/gRPC bridge.
- **Sign out everywhere endpoint (7.14).** Added `POST /v1/auth/logout-all` to revoke every refresh session
  for the authenticated OAuth user while clearing the current browser cookies and auditing the action.
- **SOC 2 readiness checklist (7.16).** Added a backend readiness checklist covering access control, change
  management, monitoring/availability, vendor management, and incident response.
- **Server migration dry-run (7.16).** Added `cashflux-server migrate-check`, which migrates a temporary
  SQLite/WAL copy and reports the resulting schema version without mutating live data.
- **Serve the SPA under a URL sub-path (B30).** The app now routes correctly when hosted under a sub-path
  (e.g. a GitHub Pages project site at `/CashFlux/`). A new pure, table-tested `internal/routebase` package
  derives the prefix from the document `<base href>`; a thin wasm layer (`uistate.RoutePath`/`LogicalPath`)
  prefixes every route registration, `DefaultRoute`, the `/p/:slug` pattern, and all navigation, while
  active-link/breadcrumb/period comparisons read the stripped logical path. At the server root the prefix is
  empty, so local dev, custom domains, and native tests are unaffected (the wildcard `*` is never prefixed).

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Client AI proxy uses streaming RPCs (7.7).** Browser AI proxy calls now use `ChatStream` and `VisionStream`
  over the gRPC tunnel, aggregating completion chunks before updating the existing UI callbacks.
- **OAuth email verification (7.20).** OAuth callbacks now reject provider profiles that explicitly report an
  unverified Google/GitHub email claim before account upsert or session issuance.
- **Account deletion unlinks billing state (7.17).** `DELETE /v1/account` now explicitly removes the caller's
  stored subscription row inside the account-delete transaction before purging the user and sweeping blobs.
- **OAuth ID-token verification (7.20).** Google OAuth callbacks now reject missing or expired ID-token expiry
  claims and future issued-at claims before userinfo fetch or session issuance.
- **AI proxy request validation (7.14).** The gRPC AI service now rejects malformed chat, vision, and key-upload
  requests with bounded field sizes before key lookup, storage, or upstream OpenAI calls.
- **Referral-fraud guardrails (7.20).** The Cloud business plan now defines referral attribution as
  accounting-only metadata and forbids product behavior changes based on referral outcomes.
- **Production data access logging policy (7.16).** Backend security notes now define when production data
  access is allowed and which actor/reason/request fields must be recorded.
- **PCI scope documentation (7.16).** The legal compliance pack now explicitly states that payment-card entry,
  updates, fraud screening, and cardholder data stay in Stripe-hosted surfaces.
- **Server deploy/migration procedure (7.16).** The operations runbook now places backup, migration dry-run,
  forward rebuild, and Caddy stream-drain verification into the deploy sequence.
- **Self-host TLS policy (7.14).** The bundled Caddy config now pins TLS 1.2/1.3 with modern AEAD cipher suites
  while preserving long-lived `/grpc` websocket streams.
- **Self-host Docker quickstart status (7.12).** Confirmed the Compose quickstart, env template, Caddy TLS notes,
  README docs link, and in-app Settings link are all wired.
- **Export/import round-trip test now covers custom-field definitions.** The lossless-export test exercised
  every dataset entity except `customFieldDefs`; it now includes a select-type definition (with options and
  the required flag) and asserts it survives the round trip, closing the last untested `Dataset` field.
- **Transactions list is paginated (C39).** Long ledgers now render the first 50 filtered rows with a
  "Show more (N hidden)" button that reveals the next batch, keeping the screen responsive instead of
  building thousands of rows at once.

### Fixed
- **gRPC JSON tunnel strictness (7.14).** The browser/server JSON codec now rejects unknown fields and trailing
  JSON payloads before dispatching RPC handlers.
- **Server migrations are tested idempotent (7.16).** Reopening an already migrated SQLite store now has
  coverage for schema stability, single metadata row, and preserved user data.
- **Billing checkout content type validation (7.14).** Checkout requests with an explicit non-JSON
  `Content-Type` now fail with `REQUEST_UNSUPPORTED_MEDIA` before any Stripe call.
- **Stripe webhook body cap (7.14).** Oversized webhook payloads now return `REQUEST_TOO_LARGE` before Stripe
  signature validation instead of being truncated into a misleading signature failure.
- **Billing checkout JSON validation (7.14).** Checkout requests now reject malformed, oversized, unknown-field,
  or trailing JSON before any Stripe call.
- **Deep-link refresh verification (B1).** Hard-refreshing clean SPA routes now has browser coverage online
  and offline after service-worker activation.
- **Nested routing no longer duplicates the app shell (B3).** The root route is now the single layout route
  that renders `Shell` once and places child screens through `router.GetOutlet()`.
- **Stripe deleted webhooks could preserve an active status.** `customer.subscription.deleted` now forces
  stored subscription state to `canceled` even if the event object carries a stale status value.
- **Deleting a member left their transactions dangling.** The Members screen decided whether to reassign
  before deleting by counting only owned accounts, budgets, and goals â€” not transactions, which carry a
  direct member tag. A member used only as a transaction tag was deleted outright, leaving those
  transactions pointing at a member that no longer existed. The check now counts transactions too, routing
  the delete through the existing reassign step (which clears/moves their member tag).
- **Goal totals ignored currency.** The Goals screen summed each goal's raw minor units into the combined
  Saved / Total target / Overall progress stats, so a goal in a non-base currency skewed the totals. Each
  amount is now converted through the FX table first (falling back to its raw amount when no rate exists),
  matching every other screen.
- **Deleting an account with transactions would orphan them.** The Accounts delete button removed the
  account row outright, leaving its transactions (and the far leg of any transfer) pointing at an account
  that no longer existed. Delete is now refused when the account still has transactions, with a message
  steering to Archive (which retires the account but keeps its history).
- **App-lock display prefs reset on passcode change.** Changing the passcode rebuilt the lock config from
  scratch, silently turning the lock-screen quotes/meta back on. `applock.WithPasscode` now carries those
  display choices over (they're unrelated to the credential) and the UI path seeds from the current config,
  so a passcode change keeps the user's lock-screen preferences.
- **Allocate weight inputs were unlabeled (C6).** The five criterion-weight fields showed as bare "1" boxes
  (label only on hover/placeholder). Each now has a visible caption (Returns / Stability / Liquidity / Debt
  reduction / Goal progress) via a wrapping `<label>`, which also gives screen readers an accessible name.

### Fixed
- **Budgets "Quarter" spend appeared less than "Month" (C40).** The Budgets screen anchored each budget's
  period to the *start* of the viewed window, so under a Quarter view a Monthly budget showed the quarter's
  first month (e.g. April) â€” making quarterly spend look smaller than monthly. Budgets now anchor to today
  when the viewed window contains today (else to the window's start), so current-period spend is correct
  under any view, and past-window navigation still works.

### Added
- **Backend trial abuse guard (7.20).** Checkout creation now refuses accounts that already used a Cloud trial
  or still have an active/trialing/past-due subscription.
- **Backend business metrics (7.15).** Billing webhooks now publish privacy-safe aggregate signup, trial,
  conversion, cancellation, payment-failure, and estimated MRR metrics.
- **Backend billing coverage (7.11).** Added explicit subscription-deleted webhook coverage alongside the
  existing entitlement-state and storage-cap tests.
- **Server mode setting (7.12).** Added a persisted Cloud/Self-hosted Settings control and hides Cloud billing
  controls when self-hosted mode is selected.
- **Cloud pricing controls (7.11 Group B).** Added Settings controls for annual/monthly Cloud pricing,
  Stripe Checkout redirects, and Stripe customer-portal management.
- **Backend Stripe billing sessions (7.11).** Added authenticated billing endpoints for Stripe Checkout and
  customer-portal session creation using configured annual/monthly price ids.
- **Backend Stripe webhook state updates (7.11).** Added a signed Stripe webhook endpoint that updates
  stored subscription state for checkout, subscription update/delete, and payment-failed events.
- **Backend storage fair-use warnings (7.11).** Added `CASHFLUX_SERVER_STORAGE_WARN_BYTES` so blob uploads can
  warn before the existing per-user storage cap blocks new over-quota uploads.
- **Backend entitlement enforcement (7.10).** Billing-enabled deployments now deny inactive Cloud users at the
  gRPC Sync/AI interceptor layer and HTTP blob endpoints while self-host mode remains always-on.
- **Backend subscription entitlement reads (7.10).** `IsCloudActive` now reads billing-enabled Cloud
  entitlement state from stored subscription rows, including active, trialing, and past-due grace states.
- **Backend subscription persistence (7.10).** Added the server `subscriptions` table and repository APIs for
  current Stripe subscription state lookup by user or Stripe subscription id.
- **Backend AI usage alerts (7.20).** Added configurable AI proxy daily request/token alert thresholds that
  append audit events when a user crosses warning lines before hard caps trip.
- **Backend AI abuse kill switch (7.20).** Added `CASHFLUX_SERVER_AI_BLOCKED_USER_IDS` to deny selected users
  before AI-key load or upstream OpenAI calls.
- **Backend auth abuse limiter (7.20).** Added a dedicated per-IP OAuth/session route rate limit via
  `CASHFLUX_SERVER_AUTH_RATE_LIMIT_PER_MINUTE`.
- **Backend general JSON errors (7.19).** Readiness, version CORS, blob preflight, in-flight, rate-limit,
  and encode-fallback failures now use the shared JSON error taxonomy.
- **Backend OAuth JSON errors (7.19).** OAuth start/callback/refresh/logout failures now return stable
  machine-readable JSON error reasons.
- **Backend audit/metrics JSON errors (7.19).** Audit, metrics, and CORS preflight failures now return stable
  machine-readable JSON error reasons.
- **Backend blob JSON errors (7.19).** Blob upload/download failures now return stable machine-readable
  JSON error reasons for auth, validation, size, media-type, quota, and lookup failures.
- **Backend JSON error details (7.19).** Account and admin support HTTP errors now return stable
  machine-readable JSON error reasons.
- **Backend error taxonomy (7.19).** Added stable machine-readable backend error reasons with gRPC/HTTP
  mappings and pinned the documented taxonomy in tests.
- **Backend compliance docs (7.17).** Added the legal compliance pack with launch draft privacy/terms,
  cookie/consent note, DPA outline, public subprocessors list, and data-subject request workflow.
- **Backend account export/delete (7.11).** Added authenticated `/v1/account/export` and `DELETE /v1/account`
  compliance endpoints with scoped export data, secret omission, and blob GC after account deletion.
- **Backend legal endpoints (7.11).** Added public `/legal/privacy` and `/legal/terms` JSON discovery
  endpoints for Cloud onboarding and billing surfaces.
- **Backend SQLi audit coverage (7.14).** Added a repository source guard that rejects dynamic SQL construction
  patterns and pins parameterized user/workspace predicates.
- **Backend usage support view (7.19).** Added authenticated `/v1/admin/usage`, a read-only usage lookup scoped
  to the caller, with cross-user isolation tests.
- **Self-host deploy link in Settings (7.13).** The backend Settings controls now link to the self-host
  deployment docs, which include the referral disclosure and non-referral path.
- **Backend Settings connection test (7.12).** Settings now has a Test connection action for the configured
  backend URL/token, validating `/v1/version` before the same base URL is used for `/grpc`.
- **Self-host token setup docs (7.13).** Added a post-deploy Settings checklist so operators know to paste the
  printed access token, test `/v1/version`, and let the app derive the `/grpc` tunnel.
- **Backend API compatibility guard (7.19).** Added `cmd/api_compat_guard`, CI coverage, and proto
  deprecation-window docs to keep `/v1`, `cashflux.v1`, and server compatibility constants aligned.
- **Backend error model.** Added `docs/BACKEND_ERRORS.md` documenting gRPC code mappings, HTTP status
  equivalents, and the in-band `accepted=false` LWW stale-write response.
- **Smart-quotes provider (B17.5).** New pure, table-tested `internal/quotes`: a curated set of
  finance/motivation quotes with a deterministic once-per-day rotation (`OfDay`), ready for the lock screen's
  optional smart-quotes display.
- **Backend proto contract (7.1).** Added `proto/cashflux/v1/cashflux.proto` plus contract-policy docs and
  tests covering SyncService/AIService methods, opaque dataset bytes, and blob references.
- **Backend toolchain pin reconciliation (7.0).** Locked the server/client backend toolchain expectation
  with deploy coverage for `go.mod` Go 1.26.0 and the `golang:1.26-alpine` server build image.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Backend dependency reconciliation.** Documented the backend dependency set and the intentional stdlib OAuth
  implementation, avoiding an unused `golang.org/x/oauth2` module, with deploy coverage for the decision.
- **Backend rollout contract reconciliation.** Tightened `docs/BACKEND_PLAN.md` so backend phases are explicitly
  independently shippable/reversible, preserve the local-first app fallback, and stay covered by deploy tests.
- **Backend unit coverage reconciliation.** Documented that storage, LWW sync, AI-key encryption, usage
  rate limits, blob hashing/linking, and blob GC are covered by focused server unit tests.
- **Backend load/abuse test coverage reconciliation.** Documented the existing coverage for gRPC stream caps,
  connection-limit config, oversized payloads, storage quotas, and HTTP/user rate-limit configuration.
- **Backend CI server build.** CI now explicitly builds `./cmd/cashflux-server` alongside native tests,
  WebAssembly build, vet, govulncheck, gosec, and gitleaks.
- **Backend deploy/ops checklist reconciliation.** Documented the single-binary/data-dir deployment surface and
  reconciled the 7.9 deploy checklist against existing backup, migration, logging, metrics, and CI coverage,
  with deploy tests covering the self-host surface.
- **Backend security checklist reconciliation.** Added a `docs/BACKEND_SECURITY.md` coverage map tying the
  top-level 7.8 privacy/security TODOs to the detailed 7.14 controls, and marked isolation plus abuse limits done.
- **Self-host gRPC proxy tuning.** The Caddy self-host config now pins upstream keepalive and long stream
  timeout/close-delay settings so `/grpc` websocket sync/watch streams survive normal TLS proxy behavior.
- **Backend AI transport docs.** Updated `docs/BACKEND_PLAN.md` to describe AI key upload, chat, vision, and
  model listing over `AIService` on `/grpc`; the old `/v1/ai/*` HTTP/SSE proxy routes are documented as retired.
- **Liquid-balance helper.** New pure, table-tested `ledger.LiquidBalance` sums spendable cash (checking,
  debit, savings, cash; non-archived, FX-converted) â€” the canonical figure behind the cash-runway metric. The
  Reports runway now uses it instead of an inline loop.
- **Investments scope decision (B27).** Added `docs/INVESTMENTS_SCOPE.md` documenting the balance-only
  core decision: no holdings, cost basis, tax lots, live prices, or market-data dependency in CashFlux core.
- **Transaction attachment references (B23).** Transactions now carry persisted `AttachmentRef` links to
  Artifact-backed receipts/documents, with SQLite CRUD and dataset export/import round-trip coverage.
- **Report export design note (B21).** Added `docs/REPORT_EXPORTS.md` to pin the shareable-report policy:
  visual exports embed already-rendered static SVG snapshots instead of live D3, CSV/JSON export typed data,
  and D3 7.9.0 stays service-worker cached for the app runtime.
- **Spending stats on Reports (B21).** New pure, table-tested `reports.SpendingStats` (count, total, mean, and
  median â€” median resists big-purchase skew) surfaced as a "%d purchases Â· average Â· median" line on Reports.
- **Renewing-soon subscriptions (B25).** New pure, table-tested `subscriptions.UpcomingRenewals` (subs renewing
  within N days, soonest first) surfaced as a "Renewing soon" card on the Subscriptions screen.
- **Backup reminder checklist reconciled (B28).** Marked the automated backup reminder TODO complete against
  the shipped `lastBackupAt`, cadence, B19 catch-up, Settings selector, export-stamp wiring, and completed
  checklist state.
- **Recurring-aware bills (B22).** Bills, the dashboard bill widget, and bill-due notifications now include
  negative Planning recurring items alongside liability-account minimum payments, advancing stale recurring
  due dates to the next upcoming cadence occurrence.
- **Split tracker status reconciled (B24).** Marked the shipped pure split/settle-up engine complete and
  recorded the standalone Split screen as partial UI coverage, with transaction-level persistence still open.
- **Subscriptions tracker marked complete (B25).** Reconciled the backlog with the shipped
  `internal/subscriptions` detector and Subscriptions screen, including renewal reminders, CSV export,
  price-change rows, and spending-share stats.
- **Per-budget rollover controls (B26).** Budgets now persist a `Rollover` flag, expose it in add/edit
  forms, and show the previous period's carried amount on each rollover-enabled budget row.
- **Backup-reminder cadence selector (B28).** Settings â†’ Data now has a "Backup reminders" control
  (Monthly / Weekly / Off), persisted locally; the gentle "back up your data" nudge honors it. Fully completes
  the backup-reminder feature (shipped in `f9ac390`).
- **Backend master-key handling docs.** The self-host env template no longer ships a default-looking master
  key, and the runbook now directs operators to source `CASHFLUX_SERVER_MASTER_KEY` from a secret manager or
  KMS-backed secret with the current maintenance-window rotation path.
- **Backend release supply-chain helper.** Added an example server release script that builds with
  deterministic Go flags, writes checksums, generates a CycloneDX SBOM, and signs the binary/SBOM with
  `cosign sign-blob`.
- **Backend sync lookup ID bounds.** Workspace sync reads now trim and reject oversized workspace ids before
  querying SQLite, matching the existing Put/Delete field limits and closing a remaining input-validation gap.
- **Backup reminders are live (B28).** Exporting your data now stamps the time, and opening CashFlux surfaces a
  gentle monthly "back up your data" reminder when it's been too long (or you've never exported), through the
  same B19 catch-up engine â€” suppressed on a fresh, empty install. Completes the backup-reminder feature end
  to end (the per-cadence Settings selector remains a future refinement; the default is monthly).

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Unified anomaly detection.** The Reports "Heads up" card now uses the shared `internal/insights` detector
  (already behind the Insights highlights and dashboard widget) instead of a second, redundant detector,
  removing duplicated logic while keeping the same overspend heads-up.

### Added
- **Net-worth change on Reports (B21).** The net-worth card now shows a "Change this period" figure (the most
  recent period's net-worth delta, color-cued up/down) alongside assets/liabilities/net, so wealth direction
  is visible at a glance.

### Added
- **One-click duplicate cleanup (imports).** The Transactions duplicate notice now has a "Select duplicates"
  button that selects the extra copy in each duplicate group (keeping one), so the existing bulk-delete can
  remove them in a single step.

### Added
- **Duplicate-transaction heads-up (C39/imports).** New pure, table-tested `internal/dedupe` package finds
  likely double entries (same date, signed amount, and description; transfers excluded) and the Transactions
  screen now shows a "Heads up: N possible duplicates" notice so accidental re-imports are easy to spot and
  clean up.

### Added
- **Spending anomaly heads-up (B21).** New pure, table-tested `reports.SpendingAnomalies` flags categories
  whose current-month spend runs well above their trailing monthly average (more robust than a single
  prior-period delta, with an absolute floor to skip noise). The Reports screen shows a "Heads up" card â€”
  e.g. "Dining is 200% above its usual." â€” for the top few.

### Added
- **Minimum-payment guidance on Planning (D9).** New pure, table-tested `payoff.MinimumViablePayment` (the
  smallest monthly payment that ever clears a debt). The payoff calculator's "payment too low" message now
  names the exact figure: "Pay at least $X a month to start clearing it."

### Added
- **No-spend days on Reports (B21).** New pure, table-tested `reports.NoSpendDays` counts the elapsed days in
  the period with zero spending (future days in the current period don't count), surfaced as a motivating
  "No-spend days" stat on the Reports grid.

### Added
- **Large-transaction alerts are live (B19).** Opening CashFlux now surfaces a "while you were away" alert for
  any unusually large charge in the last 30 days (at or above the rule's threshold), wired through the
  catch-up engine and de-duped per transaction so it shows once. Completes the large-transaction notification
  end to end.

### Added
- **Large-transaction notifications (B19).** A `notifyfeed.LargeTransactionCandidates` generator plus a default
  in-app rule (`default-large`, threshold $500) complete the notify event coverage: an expense at or above the
  threshold (in base currency, since the last open) becomes a once-per-transaction "a big charge hit your
  account" alert through the same B19 catch-up engine. Pure and table-tested.

### Added
- **Settle-up CSV export (B24).** New pure, table-tested `split.CSV` and a Download CSV button on the Split
  screen's settle-up card, so you can save or share the who-owes-whom plan (From, To, Amount) â€” matching the
  CSV export the Bills, Subscriptions, and Reports screens already offer.

### Added
- **Budget limit suggestions from history (D6).** New pure, table-tested `budgeting.SuggestLimit` computes a
  category's average monthly spend over recent full months (averaged across its span of activity, so new
  categories aren't diluted), and the Budgets add form now shows "You've averaged $X/mo here recently." for the
  selected category with a one-tap "Use this" that fills the limit field.

### Added
- **Headline spending trend on Reports (B21).** Below the stat grid the Reports screen now shows a one-line
  "Spending is up/down X% versus the previous period" summary (backed by the shared `ledger.PercentChange`),
  so the overall direction is clear at a glance â€” not just the per-category deltas.

### Added
- **Subscriptions' share of spending (B25).** The Subscriptions stat grid now shows what percent of this
  month's spending goes to recurring subscriptions, so the recurring slice of your outflow is visible at a
  glance. Shown only when there's spending this month to compare against.

### Added
- **Biggest deposits (B21).** New pure, table-tested `reports.LargestIncome` (the income mirror of
  LargestExpenses) and a "Biggest deposits" card on the Reports screen listing the period's largest individual
  income transactions â€” completing the income picture alongside income-by-source.

### Added
- **Annual bill cost on the Bills screen (B22).** The Bills stat grid now shows a "Per year" figure (the
  upcoming monthly obligations Ã— 12) next to "Total due soon", so the yearly weight of recurring debt payments
  is visible at a glance.

### Added
- **Net-worth breakdown on Reports (B21).** The Reports screen now shows a "Net worth" card with assets,
  liabilities, and net totals (as of now) above the existing net-worth trend chart, so you see the composition
  and not just the line. Backed by the existing `ledger.NetWorth`.

### Added
- **Goal on-track / pace check (D12).** New pure, table-tested `goals.OnTrack` (and `OnTrack`/`PaceKnown`
  fields on `goals.Status`): at an assumed monthly contribution, reports whether a dated goal is projected to
  be met on or before its target date â€” the "am I on schedule?" complement to `MonthlyNeeded`. An
  already-complete goal is on track; undated goals and zero-contribution unmet goals report "not judgeable".

### Added
- **Proportional mode on the Split screen (B24).** The Split calculator now has a "Split by weight" toggle:
  switch it on and each included member gets a weight field (a share count or income), splitting the cost
  proportionally instead of evenly (a blank weight defaults to 1, an explicit 0 excludes). Settle-up follows
  the weighted shares. Backed by `split.ByWeights`.

### Added
- **Weighted expense split (B24).** New pure, table-tested `split.ByWeights`: splits a shared cost in
  proportion to per-member weights (share counts like 2:1, or incomes to split by earnings), distributing the
  rounding remainder by the largest-remainder method so shares sum exactly. Zero-weight members get nothing.
  The Split-screen proportional mode builds on this.

### Added
- **Downloadable income & member breakdowns (B21).** The Reports screen's "Income by source" and "Spending by
  member" cards now each have a Download CSV button, matching spending-by-category. Income reuses
  `reports.CategoryCSV`; a new pure, table-tested `reports.MemberCSV` backs the member export.

### Added
- **Income by source (B21).** New pure, table-tested `reports.IncomeByCategory` (income totals by category,
  largest first, transfers/expenses excluded) and an "Income by source" card on the Reports screen â€” the
  symmetric "where the money comes from" view alongside spending.

### Added
- **Debt payoff strategy comparison on Planning (D9).** The Planning screen now compares the snowball and
  avalanche methods across your liability accounts (pulling each one's balance, APR, and minimum payment),
  with an optional "extra per month" input. It shows months-to-clear and total interest for each method, the
  avalanche payoff order, and how much interest avalanche saves â€” built on the pure `payoff.BuildPlan`.

### Added
- **Debt snowball / avalanche planner (D9).** New pure, table-tested `payoff.BuildPlan`: simulates clearing
  several debts together with the classic debt-snowball method â€” pay every minimum, then throw all remaining
  firepower (extra plus minimums freed by cleared debts) at one focus debt chosen by strategy (`Snowball` =
  smallest balance first, `Avalanche` = highest APR first), cascading when a debt clears mid-month. Reports
  total months, total interest, total paid, and the payoff order; flags plans that can never clear. The
  Planning-screen comparison builds on this.

### Added
- **Spending-by-weekday insight (B21).** New pure, table-tested `reports.SpendingByWeekday` (totals indexed
  Sundayâ€“Saturday) and `reports.PeakWeekday`, and the Reports screen now shows a one-line insight â€” "Most
  spending happens on Fridays ($X)." â€” surfacing the day money tends to leave.
- **Spending by member (B21).** A new pure, table-tested `reports.SpendingByMember` totals each household
  member's expenses for the period (largest first; transfers and income excluded), and the Reports screen
  shows a "Spending by member" card whenever more than one member (or an unassigned bucket) has spend â€” the
  household "who spent what?" view.
- **Cash runway on the Reports screen (B21).** The Reports stat grid now shows a "Cash runway" figure â€”
  how many months your spendable cash (checking/debit/savings/cash accounts) would last at the average burn
  over the last six full months. Color-cued (under three months reads as a warning, six-plus as healthy) and
  shown only when there's real spending history. Built on `reports.EstimateRunway`/`AverageMonthlyExpense`.
- **Financial runway estimator (B21).** New pure, table-tested `reports.EstimateRunway` and
  `reports.AverageMonthlyExpense`: from a cash balance and recent monthly spend they compute how many
  months (and days) of buffer you have â€” the classic "how long would my savings last?" metric. A
  non-positive burn reads as sustainable (never depletes); the average skips fully-inactive months so gaps
  in history don't understate the burn. Logic-first per the SDLC; the Reports-screen surfacing builds on it.
- **Budget pace warning on screen (D2).** Each budget row now shows a gentle "at this pace, projected to go
  over by $X" line while its period is still in progress and current spending is trending over â€” backed by
  `budgeting.ProjectPace`. Hidden once the period ends or the budget is already over (so it never doubles up
  with the "over budget" state).
- **Subscription price changes on screen (B25).** The Subscriptions screen now shows a "Recent price changes"
  card when any recurring charge's price has moved â€” each one's up/down delta, percent, new amount, and the
  date it changed, most-recent first. Read-only over `subscriptions.DetectPriceChanges`.
- **Backend TLS-safe browser config defaults.** Server config now rejects wildcard CORS origins and rejects
  cleartext browser origins or OAuth redirect URLs unless they target loopback local development, keeping
  production app origins and OAuth callbacks HTTPS-only by default.
- **Subscription price-change detection (B25).** New pure, table-tested `subscriptions.DetectPriceChanges`:
  the "your subscription went up" signal. Where `Detect` groups by name and amount (so a price change splits
  into two), this groups recurring charges by name only, confirms a regular cadence, and reports the most
  recent amount transition â€” old vs new price, the delta, the rounded percent change, and the date it changed
  (`Increased()` flags rises). Floors at three charges so a one-off isn't mistaken for a change.
- **Budget pace projection (D2).** New pure, table-tested `budgeting.ProjectPace`: from a budget's Status and
  its period bounds it forecasts end-of-period spend at the current rate (spent Ã· fraction-elapsed), reporting
  the projected total, any projected overspend, and whether you're on track â€” the forward-looking complement
  to Status, which only reports spend so far. Recovers the limit from the Status (no rate table needed), guards
  against extrapolating before any time has elapsed, and clamps to avoid int64 overflow on tiny fractions.
- **Backup reminders wired into notifications (B28).** A new `notify` event (`backup-due`, with a default
  in-app rule) and a `notifyfeed.BackupCandidates` generator that turns the backup cadence into a gentle,
  informational "back up your data" reminder â€” surfaced at most once per cadence period (ISO-week for weekly,
  month for monthly) via the same B19 catch-up-on-wake engine, and never when the cadence is off. Pure and
  table-tested; the dismissible export nudge + Settings cadence control build on this.
- **Budget rollover & sinking-fund math (B26).** New pure, table-tested `internal/budgeting` helpers:
  `Carryover` advances envelope budgeting one period (last period's remaining â€” negative when overspent â€”
  plus this period's limit), the single-step recurrence behind a "carried over $X" badge; and a sinking-fund
  trio (`SinkingFundContribution` with ceiling rounding so the goal is always met by the deadline,
  `SinkingFundAccrued` capped at target so it never overshoots, and `SinkingFundProgress`) for saving
  steadily toward a known future expense. Logic-first per the SDLC; the per-budget toggle + UI build on this.
- **Backend blob garbage collection.** Added `cashflux-server gc-blobs`, weekly self-host systemd examples, and Prometheus counters for blob GC sweeps/deletions.
- **Structured backend logging foundation.** The server now configures `log/slog` with text/json
  formats, runtime log levels, and redaction for token/key/secret/cookie/password attributes.
- **Distinct backend liveness probe.** Added `/livez` as a process-up probe separate from
  `/readyz`, which remains the SQLite readiness check.
- **Split a shared expense (B24).** A new **Split** screen in the Tools nav: enter an amount, tick who's
  sharing it, and it shows each member's even share (the rounding remainder distributed so they add up
  exactly); pick who paid and it lists who owes them what. Built on the pure `internal/split` core â€” no
  setup, a handy household calculator. It has its own split icon in the rail.
- **Reports: Savings-rate trend (B21).** The Reports screen now charts your savings rate (percent of income
  kept) over the last six periods, so you can see whether it's trending up. Backed by a pure, table-tested
  `reports.SavingsRateSeries`.
- **AI upstream timeout and retries.** The backend AI proxy now applies a configurable upstream OpenAI
  deadline and bounded jittered retries for transient transport, 429, and 5xx failures.
- **Reports: Biggest expenses (B21).** The Reports screen now lists the period's largest individual
  purchases (description, date, amount), backed by a new pure, table-tested `reports.LargestExpenses`.
- **Reminders on open â€” notifications are live (B19).** When you open CashFlux it now surfaces a gentle
  "while you were away" toast for anything that needs attention â€” accounts whose balance has gone stale,
  bills due within a week, and budgets that are near or over their limit â€” plus a once-a-week recap of
  last week's money in and out. Each reminder fires at most once per its natural period (a stale account
  weekly, a bill once per due date, a budget once per state per month, the digest once per week), tracked
  in a persisted delivered log so reopening doesn't re-nag.
  Boot-safe (a notification hiccup can never block startup). The full in-app center + per-rule settings
  build on this.
- **Notifications: first event evaluator (B19, internal).** New pure `internal/notifyfeed` package bridges
  domain data to notification candidates. Its first generator, `StaleBalanceCandidates`, turns
  freshness's stale-account detection into weekly de-duped notify candidates â€” the first concrete event
  the catch-up engine can surface. Table-tested; keeps `notify` itself free of domain dependencies. A
  second generator, `BudgetCandidates`, turns budgets that are near or over their limit into candidates
  (over = critical), deduped per budget + state per month so a budget crossing from near to over still
  fires a fresh alert. A third, `BillDueCandidates`, turns bills due within a window (default 7 days) into
  candidates keyed by due date (due today/tomorrow = critical). A fourth, `DigestCandidates`, emits a
  periodic summary keyed by period (week/month). **All four recommended Phase-A notification events now
  have pure, tested generators feeding the catch-up engine** â€” the notification logic is complete; the
  in-app surface follows. A `notify.DefaultRules()` factory provides the recommended out-of-the-box rule
  set (all four events, in-app, no quiet hours) the UI will seed and let you tweak.
- **Bills: Download CSV (B22).** A "Download CSV" button on the Bills screen exports your upcoming bills
  (name, due date, days until, amount) as a CSV. Backed by a pure, table-tested `bills.CSV`.
- **Subscriptions: Download CSV (B25).** A "Download CSV" button on the Subscriptions screen exports your
  detected subscriptions (name, cadence, charge, monthly, annual, next renewal) as a CSV. Backed by a
  pure, table-tested `subscriptions.CSV`.
- **Reports: Top payees (B21).** The Reports screen now also shows where your money went by merchant â€” the
  period's expenses grouped by description (case-insensitively) and ranked by total, top 8. Backed by a
  new pure, table-tested `reports.TopPayees`.
- **Reports: Download CSV (B21).** A "Download CSV" button on the Reports screen exports the
  spending-by-category breakdown (category, amount, prior, change %) as a spreadsheet-friendly CSV. Backed
  by a pure, table-tested `reports.CategoryCSV`.
- **Self-host Docker quickstart.** Added `Dockerfile.server`, `docker-compose.selfhost.yml`, a Caddy
  reverse-proxy config, a server env template, and `docs/SELF_HOSTING.md` with token setup, TLS,
  backup/restore, upgrade, and optional OAuth notes. The README now links this runbook from the build
  section.
- **Split / settle-up â€” the pure core (B24, internal).** New `internal/split` package for sharing costs
  between household members: `Equal` divides a cost evenly (distributing the rounding remainder so the
  shares sum exactly), `NetBalances` nets who paid against who owes across many shared expenses, and
  `SettleUp` proposes a small, deterministic set of "X pays Y $Z" transfers to clear the balances. All
  integer minor units, table-tested. The Split-on-a-transaction and Settle-up views build on this next.
- **Self-host token rotation helper.** `cashflux-server rotate-token` now prints a fresh high-entropy
  bearer token and matching `CASHFLUX_SERVER_TOKEN_SHA256` value for self-host deployments.
- **Billing-disabled entitlement seam.** `IsCloudActive` now centralizes Cloud entitlement checks
  and treats billing-disabled self-host deployments as active by default.
- **Self-host token hardening.** Token mode now accepts `CASHFLUX_SERVER_TOKEN_SHA256`,
  compares bearer tokens by digest, and generates/prints a high-entropy token when no token is configured.
- **OAuth start endpoint.** `GET /v1/auth/{provider}` now redirects configured Google/GitHub providers
  with PKCE S256 and a short-lived HttpOnly state/verifier cookie.
- **OAuth provider discovery config.** The backend now loads Google/GitHub OAuth client settings from
  environment variables and exposes configured provider names from `/v1/version` with CORS for client reachability checks.
- **AI model listing over gRPC.** `AIService.ListModels` now returns the configured model allow-list
  (or the app's default model picker list) through the GoGRPCBridge tunnel.
- **Sync oversized snapshot rejection.** `PutWorkspace` now maps over-limit datasets to
  `ResourceExhausted` over the gRPC bridge, with bridge coverage for the rejection path.
- **Browser sync watch subscription.** The wasm app now opens `SyncService.WatchWorkspaces`
  over the gRPC bridge, ignores same-device echoes, and pulls the active workspace when another device changes it.
- **SyncService workspace watch stream.** The backend now exposes `WatchWorkspaces` over the
  GoGRPCBridge tunnel with per-user in-process fan-out for workspace put/delete events.
- **Browser autosave sync over gRPC.** The wasm app now pushes changed active-workspace snapshots through
  SyncService over GoGRPCBridge and pulls newer server snapshots on boot/focus using local sync metadata.
- **Sync snapshots over gRPC.** Workspace `Put` and `Get` RPCs now carry opaque dataset snapshot bytes, storing
  them in the existing current/history snapshot tables and returning the server copy on stale LWW rejects.
- **SyncService over the gRPC bridge.** The backend now registers workspace `List`, `Get`, `Put`, and `Delete`
  RPCs behind GoGRPCBridge `/grpc`, with a bridge integration test covering LWW accept/reject and tombstones.
- **AI proxy over the gRPC bridge.** Settings key upload and AI proxy calls now use the GoGRPCBridge `/grpc`
  websocket tunnel with authenticated gRPC unary calls, backed by a bridge integration test for SetKey + Chat.
- **Backend AI proxy CORS preflight.** The backend now answers local SPA preflight requests for
  `/v1/ai/chat` and `/v1/ai/vision`, exposes response metadata headers, and caches successful preflights
  so browser-based AI proxy calls can reach the local dev server.
- **Backend readiness and graceful shutdown.** `/readyz` now verifies the SQLite store is configured,
  pingable, and migrated before reporting ready, while `cashflux-server` now drains through
  `http.Server.Shutdown` on interrupt or SIGTERM instead of exiting abruptly.
- **Backend auth handshake documented.** `docs/BACKEND_PLAN.md` now spells out how the Settings backend URL is
  used for HTTP routes and converted to the GoGRPCBridge `/grpc` websocket target, and how the same bearer token
  flows through HTTP `Authorization` and gRPC metadata.
- **Authenticated blob HTTP endpoints started.** The backend now exposes bearer-protected `PUT`, `GET`, and
  `HEAD /v1/blobs/{hash}` endpoints that enforce claimed SHA-256 hashes, request-size caps, content-addressed
  SQLite/file storage, immutable cache headers, and CORS preflight support.
- **Client gRPC bridge transport foundation.** Added a `syncbridge` client helper that converts the saved
  backend URL to the `/grpc` websocket tunnel target, builds a GoGRPCBridge `BuildTunnelConn`, and attaches
  the backend bearer token as gRPC metadata for unary and streaming calls.
- **Backend gRPC bridge transport mounted.** The server now exposes `/grpc` through GoGRPCBridge with SPA
  origin checks, keepalive/idle settings, read limits, and active/per-client/upgrade caps, reusing the
  existing bearer-token auth path for RPC metadata.
- **Bills â€” month-calendar layout (B22, internal).** `bills.MonthCalendar` lays out a month as a grid of
  whole weeks (starting on the user's week-start), placing each bill on its due day and padding the first
  and last weeks with adjacent-month days. Pure, table-tested; the calendar view renders on this next.
- **Bills tracker â€” the pure core (B22, internal).** New `internal/bills` package that derives upcoming
  bills from your liability accounts: each account with a statement due-day and a minimum payment becomes a
  monthly bill with its next due date (correctly clamped for short months â€” a "due on the 31st" bill lands
  on Feb 28/29) and days-until, soonest first. Pure, table-tested.
- **Bills screen (B22).** A new **Bills** item in the Tools nav lists those upcoming payments â€” name, next
  due date, how soon it's due ("due today / tomorrow / in N days"), and the amount â€” with the total due
  soon and the next due date up top. It has its own calendar icon in the rail, and each bill has a
  **Remind me** button that adds a to-do dated to the bill's due date. The month calendar and mark-paid
  come next.
- **Subscriptions detection â€” the pure core (B25, internal).** New `internal/subscriptions` package that
  finds recurring charges in your transaction history: it groups identical repeated expenses, infers a
  cadence (weekly / monthly / yearly) from the spacing between them, and reports each one's normalized
  monthly and annual cost plus the next expected renewal date â€” with a total monthly burden. Base-currency,
  ignores one-offs and irregular spacing, deterministic and table-tested.
- **Subscriptions screen (B25).** A new **Subscriptions** item in the Tools nav lists those detected
  recurring charges â€” name, cadence, charge, normalized monthly cost, and next renewal date â€” with your
  total monthly and yearly subscription burden up top. It has its own repeat-cycle icon in the rail, and
  each row has a **Remind me** button that adds a to-do dated to that subscription's next renewal so you
  can decide whether to keep or cancel it before the next charge.
- **Reports screen (B21).** A new **Reports** item in the Tools nav: for the period chosen in the top bar
  it shows income / spending / net / savings-rate, a plain-English summary of where the money went, and
  spending by category compared to the prior period (each category's amount with a green â–¼ / red â–² change
  badge). Works with no AI key â€” it's all from the deterministic reports core, so the figures match the
  dashboard. It also charts a **cash-flow trend** and a **net-worth trend** over the last six periods of
  the chosen resolution â€” and has its own bar-chart icon in the rail.
- **Reports engine â€” the pure reporting core (B21, internal).** New `internal/reports` package with the
  first report: spending by category over a period, sorted largest-first, with an optional comparison to
  the prior period (each category's prior amount + percent change, and a union so a category that dropped
  to zero still shows as a mover). Base-currency, transfers excluded, deterministic and table-tested. The
  Reports screen + charts build on this next.
- **Reports engine â€” income-vs-expense / cash-flow report (B21, internal).** `reports.IncomeVsExpense`
  for a single period and `reports.IncomeExpenseSeries` across consecutive buckets (for the cash-flow
  trend chart), each carrying net and savings-rate, reusing the shared ledger totals so figures match the
  dashboard. Pure and table-tested.
- **Reports engine â€” deterministic narrative summaries (B21, internal).** `reports.SpendingNarrative`
  turns a spending report into a short plain-English summary ("You spent $X across N categories. Your
  biggest expense was Rent at $Y. Fun fell 100% to $0 versus the prior period.") â€” template-based, not AI,
  so it's stable and testable. Formatter/name callbacks keep it decoupled from the UI. Pure, table-tested.
- **Reports engine â€” top movers (B21, internal).** `reports.TopMovers` ranks the categories that changed
  most versus the prior period (largest absolute change first, deterministic ties); the narrative summary
  now reuses it. Pure, table-tested.
- **Notifications foundation â€” the pure rules core (B19 Phase A, internal).** New `internal/notify`
  package with notification/rule types, channel selection, daily quiet-hours (with past-midnight wrap),
  per-period idempotency keys (day/ISO-week/month) and a delivered-log so catch-up-on-wake won't replay
  the same alerts. Pure and table-tested; the in-app center, browser pop-ups, and catch-up engine build
  on this next. No user-visible change yet.
- **Notifications â€” the catch-up engine (B19 Phase A, internal).** `notify.CatchUp` turns the candidate
  occurrences found for the time you were away into the "while you were away" list: it gates by rule
  (enabled + has a channel), skips anything already delivered, and applies each rule's frequency cap
  (keeping the most recent and collapsing the rest so a long absence never floods), marking everything it
  considered as delivered so reopening doesn't replay. Deterministic and table-tested. Still no UI.
- **Dashboard tiles drill into their data screen (C30).** Each tile's title is now a link â€” click it (or
  press Enter) to jump to the screen that owns that data: Net worth / Liabilities / Accounts / Upcoming
  bills / Net-worth trend â†’ Accounts; Income / Spending / Recent / Cash flow / Savings rate / Breakdown â†’
  Transactions; Budgets â†’ Budgets; Goal â†’ Goals; To-do â†’ To-do; Highlight â†’ Insights. The grip (drag) and
  gear (settings) keep their roles, and the title shows a pointer + hover underline so it reads as clickable.
- **Empty lists now invite you to add the first item (Â§6.5).** Goals, budgets, to-do, members, rules,
  transactions, and both category lists show a centered "Add your firstâ€¦" button on their empty state
  that jumps the cursor straight to the add form, instead of just a bare line of grey text. (A filtered
  list that matches nothing still shows a plain "no matches" line â€” that's a filter result, not an empty
  account.)
- **Custom-page widgets are now fully arrangeable and editable.** Each widget tile gained a drag handle
  (drop onto another tile to reorder), width/height resize buttons (â†” / â†• cycle the span), an **edit**
  button (âœŽ â€” change the title and binding/config in place), and the existing delete. Reorder + resize
  persist in the page's layout via the pure `dashlayout` engine. This completes custom-page widget
  management (add / edit / delete / reorder / resize).
- **Pause the lock without losing your passcode (B17).** Settings â†’ App lock has a **Lock screen** switch
  that turns the gate off while keeping the passcode â€” flip it back on and no re-entry is needed (distinct
  from "Remove passcode lock", which clears it). A paused lock won't gate at startup or auto-lock. Backed by
  `Config.Suspended` + `Active()` (table-tested).
- **Unlock animation (B17.1).** Entering the right passcode now dismisses the lock screen with a brief
  blur-and-fade so the app appears to sharpen into focus, instead of snapping away. Respects
  `prefers-reduced-motion` (instant hide when reduced motion is requested).
- **Passcode hint, shown only after repeated misses (B17).** When setting a passcode you can add an
  optional hint. It stays hidden on the lock screen until **3 failed attempts**, then a "Show hint" link
  appears. A guard rejects any hint that contains the passcode (case-insensitive) so it can't leak the
  secret â€” validated in the pure `applock` package (table-tested) and at the form.
- **Lock screen shows a greeting, the date, and a daily quote (B17.1).** The unlock screen is no longer
  bare: it now greets you by time of day, shows the date, and a rotating finance/motivation line â€” all
  privacy-safe (nothing financial). Quotes come from a new curated, table-tested `internal/lockquotes`
  package and rotate deterministically by day ordinal (no randomness). Metadata refreshes each time the
  screen appears. (Settings toggles for these, and opt-in glanceable data, are follow-ups.)
- **Currency conversion edge coverage.** The `currency` unit tests now cover missing target rates,
  negative-amount rounding, and repeated cross-rate conversions so D16's pure conversion path has a tighter
  regression net. The missing-rate coverage now checks both the source and target sides of a conversion.
- **Budget scope aggregation coverage.** The `budgeting` unit tests now pin the mixed-member D4 case: an
  individual budget counts only its owner while a group budget counts the whole household for the same category.
- **Planning end-balance coverage.** The `planning` unit tests now assert that one-time plan items affect
  `EndBalance`, closing the D11 pure projection checklist item. The test pairs the one-time outflow with a
  recurring monthly inflow so the final balance proves both inputs compose correctly.
- **Goal pace coverage.** The `goals` unit tests now prove `MonthlyNeeded` feeds `Project` back to the goal's
  target date, complementing the existing allocate goal-progress scorer tests for D12.
- **Payoff final-month coverage.** The `payoff` unit tests now pin the exact final payoff month where the
  last payment is capped at the remaining balance plus interest, completing the D9 payoff/allocate unit item.
- **Net-worth rollup coverage.** The `ledger` unit tests now cover multi-member, group, multi-currency, and
  archived-account net-worth rollups in one D18 regression case.
- **Reconcile adjustment math is tested.** Balance-update adjustment calculation now lives in `ledger` and is
  covered alongside `ClearedBalance`, closing the D15 pure unit item.
- **Forecast net-worth feed coverage** now checks the projected month-by-month values after seeding from
  `ledger.NetWorthSeries`, so the D13 test proves the bridge, not just the standalone forecast math.
- **Forecast net-worth feed coverage.** The `forecast` unit tests now bridge from `ledger.NetWorthSeries`
  into `Project`, extending D13 horizon coverage to the dashboard/planning feed shape.
- **Forecast feed regression note.** D13 now also has an isolated commit-level TODO closeout so the
  net-worth feed test and checklist completion travel together.
- **Owner reassignment coverage.** The `appstate` unit tests now assert the post-move owner and scope for
  accounts, budgets, goals, and transactions, closing the D-style ReassignOwner coverage gap.
- **Recurring cadence catch-up coverage.** `Recurring.Advance` is now covered across every cadence, and
  autopost catch-up has an exact no-double-count regression test.
- **Rules retroactive coverage.** `ApplyRules` now has regression coverage for transfer exclusion and
  preserving existing tags while applying the first matching category retroactively.
- **Rules TODO closeout.** The retroactive rules test commit now carries its own post-B17 docs delta with
  the completed checklist item, after the lock-screen quote commit landed.
- **Formula/custom-field bridge coverage.** Appstate tests now export/import a custom field definition,
  account custom value, and saved formula, then validate and evaluate the imported data together.
- **Formula/custom-field TODO closeout.** The bridge test now has a post-B17 hint commit docs delta so the
  test and completed checklist entry remain atomic.
- **Formula/custom-field commit note.** Reattached the bridge-test changelog entry after the B17.1 settings
  toggle commit landed, keeping this TODO closeout self-contained.
- **Formula/custom-field verification note.** Reattached the bridge-test docs after the B17.1 unlock
  animation commit so this test/TODO closeout still updates the changelog.
- **Extract/CSV import coverage.** The CSV import unit path now proves reordered friendly columns resolve
  account, category, and member names while preserving amount/date/tags metadata.
- **Config layering coverage.** Appstate tests now document the current defaults-to-household settings
  behavior for budget methodology, including the absence of member-level methodology overrides.
- **Transfer delete pairing coverage.** Appstate now owns deleting the reciprocal leg of a transfer, with
  regression coverage that leaves unrelated same-account transfer decoys intact.
- **Freshness nudge dismissal coverage.** The dashboard freshness nudge is now dismissible, persisted per
  account, and backed by pure tests that reset the dismissal after a balance update.
- **Transfer behavior coverage.** Ledger and budgeting tests now prove paired transfers move both account
  balances while staying neutral to net worth, income/spending totals, and budget spend.
- **FX aggregate coverage.** A shared ledger/budgeting test now proves a foreign-currency account and
  transactions re-convert net worth, period totals, and budget spend when the FX rate changes.
- **Freshness reminder task coverage.** The dashboard's "Remind me" stale-balance nudge now goes through
  appstate, with a native test proving it creates an open medium-priority task from a nudge source.
- **Net-worth assembly behavior coverage.** Ledger tests now explicitly prove household net worth equals
  assets minus liabilities, owner rollups sum back to the household total, and restored archived accounts
  re-enter the net-worth figures.
- **Member ripple behavior coverage.** Appstate now owns default-member selection, new-transaction member
  attribution, and reassign-then-delete member cleanup, with tests proving no owner/member orphans and
  recomputed net-worth rollups.
- **The sample data now ships example workflows.** A first run (or a reset) comes with three ready-made
  automations so the feature is discoverable: "Flag large purchases" (`txn_abs > 200` â†’ flag for review),
  "Categorize coffee runs" (`contains(txn_payee, "coffee")` â†’ Dining), and a disabled manual "Tidy up
  categories" (apply rules). They demonstrate per-transaction conditions and transaction-mutating actions.
- **Passcode lock (B17).** You can now set a passcode that gates the app: a full-screen unlock screen
  covers everything at startup (and on demand via **Lock now**) until the right passcode is entered. Manage
  it from **Settings â†’ App lock** or the Cmd/Ctrl+K palette â€” **Set passcode lock**, **Change passcode**,
  **Lock now**, **Remove passcode lock**. The unlock screen has a **Forgot passcode?** reset (erases local
  data â€” the honest recovery for a soft, unencrypted gate). The passcode is stored only as a salted SHA-256 hash (user-global, shared
  across workspaces) and verified in constant time; it's a soft deterrent for a local-first app, not
  encryption. **Optional auto-lock** re-shows the gate after a chosen number of minutes of inactivity
  (pointer/key/scroll resets the idle clock); set the window when you create the passcode. Setting a
  passcode now uses a proper **in-app form** (passcode + confirm + auto-lock minutes, with inline
  validation) rather than native browser prompts, and every app-lock string is translatable.
- **Workflows are now real transaction automation (was: a demo).** Acting on a product critique that the
  engine couldn't see the transaction that triggered it, "when a transaction is added" workflows now get
  **per-transaction condition variables** â€” `txn_amount`/`txn_abs` (major units) and string fields
  `txn_payee`/`txn_desc`/`txn_category`/`txn_account`/`txn_tags` â€” plus a `contains()` matcher in the
  formula engine. New **transaction-mutating actions** act on the triggering transaction: **set category**,
  **add tag**, and **flag for review**. So you can finally express things like *"when a transaction's payee
  contains 'bistro', set its category to Dining"* or *"when txn_abs > 200, flag it for review."* The
  **notify** action now shows a real in-app toast (it previously only logged). Browser-verified end to end.
- **App-lock â€” pure passcode core (B17 groundwork).** New platform-independent `internal/applock` package:
  a salted **SHA-256** passcode hash (never stores the passcode in the clear), constant-time `Verify`,
  enable/clear, and inactivity `ShouldAutoLock` logic, all table-tested. This is the deterministic
  foundation for the optional passcode gate; the salt (crypto/rand), idle timing, and the lock-screen UI
  come in follow-ups. (It's a soft deterrent for a local-first app, not encryption.)
- **Command palette: Cmd/Ctrl+K (Â§6.6).** Press Cmd/Ctrl+K to open a searchable palette â€” type to filter,
  â†‘/â†“ to move, Enter to run, Esc or a backdrop click to close. It lists every screen (jump to Dashboard,
  Accounts, Planning, Workflows, â€¦), quick actions (Add a transaction, toggle light/dark theme, collapse
  the sidebar, export data as JSON/CSV, Keyboard shortcuts), and a
  full **workspace management** â€” switch to any other workspace, or create / export / import one â€” straight
  from the keyboard. Built as a self-contained DOM overlay owned by the shortcut layer, with delegated row
  clicks (no per-row listeners); the command list rebuilds on each open so the workspace entries stay current.
- **Quick-add hotkey: Alt+N (Â§6.6).** Press Alt+N anywhere (outside a text field) to open the quick-add
  transaction panel directly, skipping the +Add menu. Chose Alt+N over the audit's Ctrl/Cmd+Shift+A â€”
  that chord is reserved by Chrome (tab search) and Firefox (add-ons) â€” keeping it in the Alt family with
  the section-jump keys. Listed in the `?` shortcuts overlay.
- **"?" keyboard help overlay (Â§6.6).** Press `?` anywhere (outside a text field) to pop a cheat sheet of
  the keyboard shortcuts â€” Alt+1â€“9 section jump, Enter to save a panel, Esc to close, hold Shift for resize
  handles. Dismiss with `?` again, Esc, the âœ•, or a click on the backdrop. Self-contained (built and
  toggled entirely by the shortcut layer), so it adds no per-screen wiring.
- **Enter submits settings panels (Â§6.6).** Pressing Enter in a FlipPanel (per-widget and global settings,
  and every flip-panel form) now triggers Save and closes, like a native form. It's skipped while focus is
  in a multi-line textarea, on a button (so the button clicks normally), or in a select, and on close-only
  panels that have nothing to save. Joins the panel's existing Esc-to-close / Tab-trap behavior.
- **Keyboard shortcut: Alt+1â€¦9 jumps to a primary section (Â§6.6).** Press Alt+1 for Dashboard, Alt+2 for
  Accounts, and so on down the primary rail nav â€” move between sections without the mouse. Keys off
  `KeyboardEvent.code` so it's keyboard-layout independent and never collides with numpad alt-codes, and
  it stays inert while you're typing in a field. Installed once at boot (`wireKeyboardShortcuts`).
- **Workflows screen â€” build, run, and audit automations (Phase D).** A new **Workflows** screen (Tools)
  lets you create an automation (name, trigger â€” *when I run it* or *when a transaction is added* â€” an
  optional condition formula, and write-safe actions: create a task, apply rules, notify), enable/disable
  it, **Dry run** it to preview exactly what it would do, **Run now** to apply it, and review a **run
  history**. Adding a transaction now fires enabled "transaction added" workflows automatically. Apply +
  dry-run + condition-gating are unit-tested (a real run creates the task and records an audit run; a dry
  run changes nothing).
- **Workflow engine â€” pure core + persistence (Phase D groundwork):** new `internal/workflow` package
  models user automations (a trigger, an optional sandboxed-formula condition, and write-safe actions â€”
  create task, apply rules, notify) and plans them deterministically into explainable Effects without side
  effects (`Match`/`Eval`/`Plan`, table-tested). Workflows and their run history persist in the dataset
  (new `workflows` + `workflowruns` tables, CRUD, appstate accessors; round-trip tested). `appstate.
  RunWorkflow` plans against live figures and, unless it's a dry run, applies the effects and records an
  audit Run; `RunTriggered` fires enabled workflows for an event (e.g. txn-added). The Workflows screen
  follows.
- **Reorder workspaces.** Each row in Settings â†’ Workspaces has up/down arrows to arrange the list; the
  order flows through to the rail switcher's dropdown so your most-used workspaces sit where you want them.
  Backed by `Registry.Move` (clamped, order-preserving, leaves the active/startup selections untouched â€”
  they're tracked by id, not position) with table tests.
- **Artifacts manager + Image/Table widgets (Phase C).** A new **Artifacts** screen (Tools) lets you upload
  an image or import a CSV dataset, see them listed with size, and delete them â€” with a local-storage meter
  so you can watch usage. Two new custom-widget types bind to artifacts by id: **Image** (renders an
  uploaded image) and **Table** (renders an imported dataset's columns + rows). Verified end-to-end: an
  image-backed tile and a CSV-backed table render on a custom page.
- **Export & import a whole workspace.** Settings â†’ Workspaces now has a per-workspace **Export** (downloads
  a self-contained `workspace-<name>.json` â€” the dataset plus layout/settings) and a section-level **Import
  workspace** (adds the file as a new workspace and switches to it, bundling the current one out first so
  nothing is lost). Lets you move a workspace between devices or share a setup. The envelope is versioned
  (`{version, name, color, bundle}`) and carries no secrets â€” the OpenAI key is user-global, outside the
  per-workspace bundle. A malformed file is rejected with a clear message; an imported workspace with no
  color gets one from the palette.
- **User artifacts â€” persisted images & datasets (Phase C groundwork):** new `domain.Artifact` plus a pure,
  tested `internal/artifacts` package (kinds, CSV parsing to columns+rows, image data-URL building, byte-
  size accounting, validation). Artifacts persist in the dataset (new `artifacts` table + CRUD + appstate
  accessors), so uploaded images and imported datasets survive reload and travel with export/import
  (round-trip tested, including raw image bytes). Added `App.DatasetBytes()` so the UI can warn as storage
  approaches the browser quota. Artifacts manager + Image/Table widgets follow.
- **Per-workspace color.** Each workspace can carry an accent color so you can tell contexts apart at a
  glance: a colored dot next to the name in the rail switcher and its dropdown, and a color-tinted border
  on the collapsed-rail glyph. New workspaces (and the initial "Default") are auto-assigned a distinct
  color from a six-swatch palette, cycling by creation order; you can change it any time via the swatch
  picker in Settings â†’ Workspaces. Stored as `Workspace.Color` in the registry (`Registry.SetColor` +
  table test); empty falls back to a neutral dot.
- **Custom pages now render custom widgets (Phase B).** A custom page shows a bento grid of user-authored
  widgets bound to the app engine: **KPI** (a formula over your figures â€” net_worth, income, â€¦, formatted
  as number/percent/currency), **List** (rows from transactions/accounts/budgets/goals/tasks), **Chart**
  (your net-worth trend), and **Text** (an authored note). An **"Add widget"** toolbar picks a type, names
  it, and sets its one binding; each tile has a remove button. Widgets persist in the page (and so export/
  import and survive reload). Verified end-to-end in a browser (KPI = live net worth, list of recent
  transactions, rendered trend chart, and a text note on one page).
- **Startup workspace preference.** Settings â†’ Workspaces now has an **"On launch, open"** selector:
  *Last used workspace* (the default â€” resumes whatever you had active) or a specific pinned workspace
  that the app always opens with, regardless of which one you left it on. The choice lives in the
  workspace registry (`Registry.StartupID` â€” empty means last-used) and is applied at boot, before the
  first paint, by swapping the pinned workspace's context into place (no reload, no data loss â€” the
  last-active workspace is bundled out first). A pinned workspace that gets deleted automatically falls
  back to last-used. New `Registry.SetStartup`/`StartupTarget` with table tests.
- **Custom widgets â€” pure engine (groundwork):** two new platform-independent, table-tested packages back
  the custom-widget feature. `internal/engineenv` builds the "app engine variable surface" (net_worth,
  income, expense, counts, â€¦) a KPI formula or workflow condition can reference. `internal/widgetspec` is
  the widget catalog (KPI/List/Chart/Text + list data sources) plus deterministic KPI evaluation
  (`EvalKPI` over the sandboxed formula engine) and value formatting. Rendering + the grid follow.
- **Custom pages â€” page management:** each "My pages" entry now has a "â‹¯" menu to **rename** (re-slugs and
  follows the page), **hide/show** (a "Hidden pages" sub-section brings hidden ones back), and **delete**
  (with confirm). Rounds out Phase A page management alongside create + drag-reorder.
- **Custom pages â€” "My pages" rail group:** the sidebar now has a "My pages" section listing your custom
  pages in order, each navigating to `/p/<slug>`, with a "New page" action that names + creates a page
  (unique slug) and jumps to it. Pages are drag-reorderable (persists their order). Built on the pure
  `internal/pages` logic and the existing `navItem` (so click, drag, and the collapsed-rail flyout all
  work). Rename/delete/hide management and the page's widget grid follow.
- **Custom pages â€” screen + routing:** a generic `screens.CustomPage(slug)` renders a user-authored page,
  resolved by slug from app state, with friendly empty/not-found states (the bento grid of widgets lands in
  Phase B). All custom pages ride a single `/p/:slug` pattern route registered at startup, so new pages are
  reachable without mutating the router after mount. Adds `pages.*` i18n strings.
- **Workspaces â€” multiple independent contexts with quick switching:** one user can now keep several
  separate workspaces (e.g. real money vs. an experimental sandbox), each with its **own dataset and UI/
  layout**. A picker at the top of the sidebar shows the active workspace and lets you **switch**, create a
  **+ New workspace** (seeded with the sample), or **duplicate** the current one; **Settings â†’ Workspaces**
  manages rename/delete. Switching swaps *everything* except your **OpenAI key**, which stays available
  across workspaces. Existing data migrates automatically into a "Default" workspace on first load. Under
  the hood the active workspace lives in the canonical `localStorage` keys and inactive ones are bundled
  under `cashflux:ws-data:<id>`; switching restores the bundle and reloads so boot rehydrates cleanly.
- **Custom pages â€” persistence:** custom pages now round-trip through the store. Added a `custompages`
  table, the `Dataset.CustomPages` field, `Load`/`Snapshot` wiring, `Put/Get/Delete/ListCustomPage(s)`
  CRUD, and `appstate` accessors (`CustomPages`, validated `PutCustomPage`, `DeleteCustomPage`). The
  exportâ†’import and SQLite round-trip tests now cover a page with a layout + a bound KPI widget, so pages
  travel losslessly with the rest of the dataset.
- **Custom pages â€” data model + ordering logic (groundwork):** new `domain.CustomPage`/`PageWidget`/
  `WidgetBinding` types model user-authored pages (their own rail entry, order, visibility, and a bento
  grid of custom widgets), stored in the dataset so they export/import with everything else. A new pure
  `internal/pages` package handles slugging (`Slug`/`UniqueSlug`), display ordering (`Ordered`/`Visible`/
  `NextOrder`), drag-reorder (`Reorder`, renumbering positions), lookup (`BySlug`/`ByID`), and validation â€”
  all table-tested on native Go, no `syscall/js`. First slice of the custom-pages / widget / workflow
  feature; persistence, routing, nav, and UI follow.
- **Dashboard tiles are fully keyboard-operable (B15):** focus a tile (Tab), use the arrow keys to move
  it one slot earlier/later, and **Shift+Arrow to resize** it â€” a keyboard alternative to drag-and-resize
  (WCAG 2.1.1), animated by the same FLIP and persisted. Tiles expose `aria-keyshortcuts`.
- **Live drag-over preview on the dashboard (B2):** while dragging a tile, the grid now reflows *during*
  the drag to show where it will land (FLIP-animated), instead of only rearranging on drop. It's a
  render-only preview â€” the saved layout isn't touched, so dropping keeps the arrangement and releasing
  outside reverts it.
- **Dashboard tiles animate when they rearrange (B2):** dragging, resizing, or switching the auto-layout
  mode now glides the tiles to their new spots instead of snapping, via a FLIP shim (`web/flip.js`).
  Honors "reduce motion." Backed by a layout-signature-keyed effect so it fires only when the arrangement
  actually changes.
- **Envelope budgeting (D6):** the budgeting-method selector now offers **Envelope** â€” each budget's
  unspent funds carry forward to the next period. The Budgets screen shows a per-budget "Envelope
  balance: $X" (red when overdrawn) under a note. The balance accumulates `limit âˆ’ spent` over every
  period from the budget category's first transaction through the current one. Backed by a pure,
  table-tested `budgeting.EnvelopeAvailable`. Verified live.
- **Budgeting method: Simple or Zero-based (D6):** Settings now has a budgeting-method selector. Under
  **Zero-based**, the Budgets screen shows how much of the month's income is still unassigned â€”
  "$X left to assign", "Every dollar is assigned", or "Over-assigned by $X". The choice is household
  config and persists. Backed by `budgeting.Methodology`/`ToAssign` (pure, table-tested). Verified live.
- **Reorder the sidebar by dragging (B8):** drag a primary nav item onto another to reorder the menu;
  the order persists across reloads. New screens append and hidden ones are skipped automatically.
  (Clicking a nav item still navigates as before.) Backed by a new pure `navorder` package with table
  tests; verified live (dragging Accounts to the top reorders and persists).
- **Empty dashboard tiles now offer an "Add" button (C23):** an empty Accounts / Goals / Budgets / To-do
  widget shows an in-context "Add a â€¦" button that jumps to the relevant screen, so you can create data
  from the dashboard. The Budgets tile only offers it when there are genuinely no budgets (not when the
  at-risk filter is simply empty).
- **Opt-in "Remember my key on this device" (C27):** Settings â†’ AI now has a toggle (off by default) to
  keep your OpenAI key across reloads. When off, the key stays session-only (the dataset autosave always
  redacts it); when on, the key is saved to its own localStorage entry and restored on boot, so AI stays
  on after a refresh. A plain-English note explains it's stored unencrypted in this browser. Verified live
  (toggling on persists the key, off clears it). Closes the AI-key-lost-on-reload rough edge.
- **Your data now survives a page reload (local persistence):** previously every reload reset the app to
  the sample dataset (data was in an in-memory store with only manual Export/Import). The dataset is now
  autosaved to localStorage â€” snapshotted on a short ticker (catching every change) and on page-hide,
  writing only when it changes â€” and loaded on boot (falling back to the sample on first run). The OpenAI
  key is **redacted** before saving, so the secret stays session-only; a save that exceeds the storage
  quota is caught rather than crashing. Verified live: a redacted dataset (no `openAiKey`) is written
  within a few seconds and the app boots with its data.
- **"+ Add" is now a multi-entity add menu (C23):** instead of jumping straight to a transaction form,
  the top-bar "+ Add" opens a small menu â€” New transaction (the inline quick-add panel) Â· New account Â·
  New budget Â· New goal Â· Scan a document â€” routing to the right place so data entry isn't trapped on
  each entity's own screen. Verified live (menu opens with 5 items, "New transaction" opens the quick-add
  panel, the menu closes on select). SW cache bumped (v10 â†’ v11).
- **Auto-layout engine for the dashboard (C24, model):** a pure `dashlayout.Arrange(items, mode)` that
  reorders tiles by a chosen `Mode` â€” **Custom** (your manual order), **Auto: default** (the canonical
  built-in order), or **Auto: importance** (sort by a per-tile importance, ties broken by the default
  order) â€” and the existing `Pack` then derives positions. Auto-layout only reorders; tile sizes stay
  user-set. Tile gained an `Importance` field (additive; older saved layouts keep working). Table-tested
  (order determinism, stability, no-overlap-after-pack, no input mutation).
- **Dashboard layout-mode selector (C24):** the dashboard header now has a Custom / Auto: default /
  Auto: importance selector; the render path applies `Arrange` before `Pack`, the choice persists across
  reloads, and a manual drag bakes the current arrangement and switches back to Custom.
- **Per-tile importance ranking (C24):** in Auto-importance mode every tile's gear opens a settings panel
  with an Importance control (Highest/High/Normal/Low); ranking a tile reorders the dashboard (sizes
  stay as you set them). Because importance is a universal setting, a tile's gear panel is never empty â€”
  so the gear can appear on every tile in importance mode without reintroducing C21's empty panel. End-
  to-end verified live: ranking the bottom freshness tile "Highest" moved it from grid-row 8 to row 2,
  and the choice persisted. This completes the C24 auto-layout feature.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- **Dashboard tiles ease their hover border (Â§6.11).** Bento tiles (`.w`) gained a `border-color` transition so
  the hover highlight fades in smoothly instead of snapping.
- **Charts draw in on first paint (Â§6.16).** Bar charts grow up from the baseline and line/trend charts draw
  left-to-right the first time they render, instead of snapping into place. Animates once per chart (guarded by
  a `data-cf-drawn` flag so data ticks don't re-trigger it) and is skipped under `prefers-reduced-motion`.
- **Lock screen fades in instead of popping (Â§6.18).** Showing the passcode gate (on boot, manual lock, or
  auto-lock) now plays a brief opacity + scale settle â€” the mirror of the unlock fade-out â€” so the gate appears
  smoothly. Web Animations API, skipped under `prefers-reduced-motion`.
- **List rows highlight on hover; progress bars grow into place (Â§6.16).** List rows now show a subtle
  background highlight under the cursor (with a short fade) so the active row is obvious and lists are easier
  to scan. Budget/allocate progress bars animate their width on load and update instead of snapping (gated
  behind `prefers-reduced-motion`).
- **Wrong-passcode shake on the lock screen (Â§6.18).** Entering an incorrect passcode now shakes the input
  field â€” the familiar "no" cue â€” in addition to the red message. Implemented with the Web Animations API (no
  stylesheet needed) and skipped under `prefers-reduced-motion`.
- **Tactile press feedback on interactive controls (Â§6.16).** Buttons, nav items, segmented controls, the
  add/menu buttons, checkboxes, and chips now dip 1px on `:active`, so a click reads as a physical press
  instead of a dead state. Gated behind `prefers-reduced-motion` to honor the app's motion preferences.
- **The dashboard "Upcoming bills" widget now shares the Bills screen's logic (B22).** It derives its
  bills from the same `bills.Upcoming`, so the widget and the screen always show the same due dates
  (including correct month-end clamping) instead of two slightly different calculations.
- **AI proxy is gRPC-only.** Retired the legacy `/v1/ai/key`, `/v1/ai/chat`, and `/v1/ai/vision`
  HTTP routes so key upload, model listing, chat, and vision all use authenticated AIService RPCs
  over the GoGRPCBridge `/grpc` tunnel. The HTTP mux now has regression coverage that keeps those
  routes unmounted.
- **Backend AI proxy cancellation is pinned by tests.** Canceling an AI request context now has regression
  coverage proving the upstream OpenAI request sees the canceled context and the service returns `Canceled`.
- **AI screens can use the backend proxy instead of browser OpenAI calls.** Insights, Allocate, and Documents now
  route chat/vision requests through the configured backend URL/token when present, keeping direct browser OpenAI
  as the local-only fallback.
- **Backend AI proxy now enforces abuse guards.** Server-side OpenAI calls can be constrained with an
  allow-list of model IDs, max request-body bytes, and per-user daily request/token caps before the encrypted
  BYO key is loaded or an upstream call is made.
- **Screen registry routes through i18n (copy pass).** The `screens.go` route registry hardcoded every
  screen's nav label, page title, and subtitle in English. Labels/titles now hold the existing `nav.*` keys and
  subtitles hold new `screen.*Sub` keys; the shell resolves them via `uistate.T` at render. The custom-page
  fallback title ("Page") is keyed too. No display English remains in the registry â€” page headings and
  subtitles now localize with the rest of the app.
- **Dashboard empty states route through i18n (copy pass).** Three hardcoded strings on the dashboard â€” the
  "App state is not ready yet." fallback (now reuses the shared `common.notReady` key), the upcoming-bills
  empty state, and the budget-alerts empty state â€” now go through the language store. Copy nudged friendlier:
  "Nothing's near or over budget."
- **Lock-screen greeting routes through i18n (copy pass).** The time-of-day greeting ("Good morning/
  afternoon/evening") on the passcode lock screen was hardcoded English; it now uses `applock.greeting*`
  keys so it localizes with the rest of the lock screen.
- **Settings data-actions now route through i18n (copy pass).** The export/import/load-sample/wipe toasts and
  the wipe confirmation, the FX-rate row label, and the freshness "0 = never" hint were hardcoded English; they
  now go through the language store (`settings.*` keys) so they localize and read consistently. Success toasts
  take the filename as a parameter (rebrand-friendly), and the freshness hint reads "days Â· 0 means never".
- **Backend AI proxy can call OpenAI with the encrypted server key.** Added a server-side AI service plus
  `/v1/ai/chat` and `/v1/ai/vision` endpoints that authenticate the caller, decrypt the user's stored BYO key,
  reuse the existing request builders, forward to OpenAI, map upstream failures, and record usage totals.
- **Client OpenAI keys can be handed to the backend securely.** Settings now has backend URL/token controls and
  an upload action that sends the current OpenAI key to `/v1/ai/key`; the server requires bearer auth plus an AES
  master key, stores the key encrypted in SQLite, and never returns it.
- **Backend SyncService applies LWW workspace puts.** Workspace updates now accept fresh client timestamps,
  reject stale writes with the current server state, support a force override, bump server versions, and block
  cross-user workspace ID takeover.
- **Backend SyncService scopes workspace reads and tombstones.** Added authenticated-user service helpers for
  workspace list/get/delete that route through the store with caller `user_id` isolation and reject unauthenticated
  or malformed requests.
- **Backend RPC auth now has bearer middleware.** Added gRPC unary and stream interceptors that read bearer
  metadata, validate tokens through a server hook, and attach the authenticated user to the RPC context.
- **Backend usage counters are ready for rate limits.** Server storage now tracks per-user UTC-day
  request and token counters with helpers for daily limit checks and tests for increments, isolation,
  empty users, and invalid caps.
- **Backend AI keys are encrypted at rest.** Server storage now accepts an env-provided AES master key,
  stores per-user provider keys with AES-GCM, and tests rotation, wrong-key failure, and plaintext
  avoidance.
- **Backend blobs are content-addressed.** Server storage now writes artifact bytes under sha256
  path-sharded filenames, records blob metadata, links blobs to workspaces, verifies reads, and sweeps
  unreferenced blobs for future artifact sync.
- **Backend snapshot storage retains recovery history.** Server storage now writes current workspace
  snapshots, preserves prior versions in last-N history, and rejects oversized dataset payloads before
  they reach SyncService.
- **Backend repository layer has native coverage.** Added typed server-store methods for users and
  workspace registry rows, including per-user listing/getting and soft-delete tombstones for the
  future SyncService.
- **Backend storage schema is pinned.** Added the server SQLite migration foundation with WAL/foreign-key
  setup, schema-version rejection for newer databases, and the planned Cloud tables for users,
  workspaces, snapshots/history, blobs, encrypted AI keys, and usage.
- **Backend server foundation started.** Added the `cmd/cashflux-server` entrypoint plus a native
  `internal/server` package with env config, health/readiness checks, and a `/v1/version` compatibility
  response for the self-host Test connection path.
- **Reviewed document imports are testable through appstate.** The image-review import path now shares an
  appstate helper that skips duplicates, records import history, and commits reviewed rows so spending
  totals, budgets, and statement summaries can be covered without a browser-only code path.
- **Rules auto-fill now shares one tested path.** Transaction entry and CSV import now both run through
  the appstate auto-categorization helper, preserving manual category/tags while first-match rules fill
  empty fields; coverage also asserts imported budget impact, apply-to-existing, and conflict warnings.
- **Inline editors now put the cursor in the first field (Â§6.7).** Opening any inline edit form â€” goals
  (incl. *Contribute*), accounts (incl. *Update balance*), transactions, budgets, categories, members,
  to-do tasks, rules, document drafts, and custom-page widgets â€” focuses the first input automatically,
  so you can start typing without reaching for the mouse.
- **Lock-screen content is now toggleable (B17.1).** Settings â†’ App lock has two switches â€” *Show greeting
  & date* and *Show a daily quote* â€” both ON by default; turning one off hides it on the unlock screen.
- **App lock is now in Settings.** Added a **Settings â†’ App lock** section so the passcode lock is
  discoverable (it was previously only reachable via the Cmd/Ctrl+K palette). The section shows the current
  status and adapts: **Set passcode lock** when off; **Lock now / Change passcode / Remove** when on. The
  in-app setup form now refreshes the section on success.
- **Keyboard UI is now translatable (Â§6.6 i18n).** The `?` cheat sheet (title + row labels) and the
  Cmd/Ctrl+K command palette (search placeholder, "No matching commands", and the action labels â€” toggle
  theme, collapse sidebar, switch/new/export workspace) now go through the language catalog (`uistate.T`,
  new `shortcuts.*` / `cmd.*` keys) instead of hardcoded English, so they translate with the rest of the
  UI; the key chords stay literal.
- **Workspace switcher adapts to the collapsed rail.** In the 58px collapsed sidebar the full-width
  labelled switcher button doesn't fit, so it now renders as a compact icon-only square showing the
  active workspace's initial; its menu flies out to the right at a readable fixed width (with a hover
  title carrying the full name). The expanded rail is unchanged. Keeps workspace switching reachable in
  both rail states instead of leaving a cramped, clipped control.
- **Service-worker cache bumped to v16** so clients re-fetch the updated wasm after the sample-data change
  (it's network-first, but the bump evicts any stale cached `main.wasm`). To populate the new persona at
  runtime, use **Settings â†’ Data â†’ "Load sample"**, which replaces the current data with the fresh seed.
- **Sample data is now a realistic persona:** first-run / "Load sample data" loads the finances of Michael
  Brooks â€” a 46-year-old single homeowner â€” instead of the bare placeholder. It includes a full balance
  sheet (checking, high-yield savings, brokerage/401(k), home, mortgage, auto loan, credit card), ten
  spending categories, and **three months of recurring activity** (Aprilâ€“June 2026: salary, mortgage,
  utilities, groceries, dining, car, insurance, health, subscriptions, shopping, plus monthly transfers to
  savings and the brokerage) so the trend charts, breakdowns, and net-worth history have real data. Five
  monthly budgets, three goals (emergency fund, retirement, new-car), and a few tasks round it out.
- **Spend-breakdown ranking moved into the tested ledger package (internal):** the dashboard's
  sort-categories-by-spend / top-N / collapse-the-rest-into-"Other" logic lived inline in the view. It's now
  `ledger.RankSpending(totals, n) (top, other)` â€” pure and table-tested â€” with name resolution and labels
  kept in the view. No behavior change.
- **Account-by-id lookup consolidated into a tested `domain.AccountByID` (internal):** the documents view's
  `accByIDFrom` and the goals view's `accountName` each re-implemented the same linear scan. Both now use one
  pure, table-tested `domain.AccountByID(accounts, id) (Account, bool)`. No behavior change.
- **`firstNonEmpty` display fallback moved to a tested helper (internal):** the documents view's untested
  `firstNonEmpty(a, b)` is now `textutil.FirstNonEmpty` (pure, table-tested, treats whitespace as empty).
- **Numeric form parsing consolidated into tested helpers (internal):** the view layer had untested
  `parseFloatOrZero`/`parseIntOrZero` (accounts) and a near-identical `parseWeight` (allocate). Added
  `textutil.ParseFloat`/`ParseInt` (pure, table-tested, tolerant: 0 on blank/garbage); accounts uses them
  directly and allocate's `parseWeight` now delegates (keeping its non-negative clamp). No behavior change.
- **Comma-list parsing unified into a tested helper (internal):** `parseTags` (transactions/rules) and
  `parseOptions` (custom fields) were duplicate, untested copies of the same "split on commas, trim, drop
  empties" logic in the view layer. Both are now `textutil.CommaFields` â€” one pure, table-tested helper â€”
  removing the duplication. No behavior change.
- **"Recent transactions" logic moved into the tested ledger package (internal):** the dashboard's
  newest-first/top-N transaction selection lived in the wasm-only view (`dashboard.go`) with no tests. It's
  now `ledger.Recent(txns, n)` â€” pure and table-tested (ordering, limit, nâ‰¤0, no input mutation) â€” with a
  negative-n guard added (the old inline version would have panicked). No behavior change for valid input.
- **Dashboard span math moved into the tested layout package (internal):** the tile resize grow/shrink/clamp
  arithmetic lived in the wasm-only view (`internal/ui/widget.go`) with no tests, against the project rule
  that computation belongs in pure, unit-tested packages. It's now `dashlayout.CycleSpan`/`dashlayout.ClampSpan`
  with table tests; the widget just calls them. No behavior change.
- **The sidebar is now derived from the screen registry (B7):** each rail section (Primary, Tools, System)
  is built by filtering `screens.All()` on a new `Route.Group` field instead of three hand-maintained
  lists. Membership lives in one place, so a newly registered screen can't silently miss the menu â€” an
  unmapped screen even falls back to its registry label and a default icon rather than being dropped. No
  visible change: the derived order matches the previous hardcoded order.
- **Planning forecast chart upgraded to a labelled comparison (D10):** the 12-month net-worth forecast
  now renders with the D3 chart (a proper **dollar** Y axis like C16, not the axis-less sparkline), and
  when you enter a "trim spending" amount it **overlays the trimmed scenario beside the baseline** (two
  labelled, color-coded lines + a legend) so you can compare the curves directly.
- **Account rows are less cluttered (C9):** the six per-row actions are now Transactions / Edit / âœ• inline
  plus a **â‹¯ overflow menu** holding the secondary actions (Update balance / Mark updated / Archive).
- **Parent-category budgets now include sub-category spend (D5):** a budget on a parent category (e.g.
  "Food") counts spend in its sub-categories (e.g. "Groceries", "Restaurants") too, rolling the subtree
  up. Period and per-owner scope are still respected, and reparenting a sub-category moves its spend to
  the new parent. Backed by a new pure `categorytree.Descendants` + `budgeting.EvaluateRollup` (table-
  tested: multi-level, reparent, scope). Budgets with no sub-categories are unaffected. (The spending-
  breakdown widget already rolled sub-categories up; this brings budgets in line.)
- **Text/display size now scales to 200% for accessibility (C26):** the display-scale control (Settings â†’
  Appearance, relabelled "Text & display size") now goes up to 200% (was 130%), meeting WCAG 2.1 SC 1.4.4
  "Resize text." This works now because the C10/C19 responsive fixes make the app *reflow* at high zoom
  instead of overflowing â€” verified live: at 200% on a 1280px window the page reflows with no horizontal
  scroll. It composes with the density setting (an independent zoom multiplier on top of the base tokens).
- **Tighter default density (C25):** the out-of-the-box UI felt too heavy for a dense finance app. The
  base font is now 14.5px (from 16) with line-height 1.45, and the shared control/widget tokens are
  trimmed â€” `.field` ~34px (was ~40) with 6px corners, `.btn` padding reduced, `.wbody` padding tightened.
  The Fraunces display figures keep their sizes, so the data accents stay prominent; the Compact toggle
  and Display scale remain as further levers. Nothing drops below the 24px touch-target minimum. Verified
  live (no text clipping; KPI figures still fit). SW cache bumped (v11 â†’ v12).
- **Negative money now reads the same on every screen (C2):** all figure displays use one accounting
  formatter, so negatives show in parentheses (`($60.20)`) with thousands grouping everywhere â€” the
  Transactions list, Accounts, Budgets, Goals, Allocate, Planning, etc. now match the Dashboard instead
  of mixing in a minus sign (`-$60.20`). The two display formatters (`fmtMoney`/`fmtAccounting`) were
  collapsed into one. Editable inputs are unaffected (they format with a plain minus and never parse a
  parenthesized value). Verified live in a headless browser (Dashboard figures unchanged: `$20,749.25`,
  `($1,500.00)`).
- **Dashboard tiles now reflow instead of overlapping (C14/B2):** the bento is now an ordered sequence
  packed into the grid, so dragging a tile reorders it and the others flow to fill the gap, and resizing
  reflows around the new size â€” fixing the old behavior where a widened tile overlapped its neighbor and
  the resize handle then "stopped working." Resize handles cycle the span (tooltips now say so). The
  default arrangement is unchanged (verified pixel-for-pixel in a headless browser).

### Added
- **Updating an account balance now confirms it out loud (B15):** the reconcile / "Update balance" flow
  used to apply the change silently. It now posts a polite toast â€” "Updated <account> to $X" â€” so the
  result is visibly acknowledged and announced to screen readers via the live region, matching what
  "Mark updated" already did.
- **Dashboard tiles can be shrunk with the mouse, not just grown (C14/#1032):** the edge resize handles
  used to only grow a tile's span (clicking cycled up and wrapped to 1 at the max). Now **Shift+click
  shrinks** the span one step directly (clamped at 1), while a plain click still grows. It mirrors the
  keyboard Shift+Arrow resize, and the handle tooltips say so.
- **Screen readers hear the filtered transaction count (B15):** the Transactions list gained a polite
  `role="status"` live region that announces how many transactions match the current filters â€” e.g.
  "Showing 12 transactions, net âˆ’$340.00" or "No transactions match your filters" â€” and updates as you
  change the search, account, category, member, date range, or cleared filter. It stays mounted (so the
  zero-results case is announced too), and the existing visible summary is now `aria-hidden` to avoid a
  double read.

### Removed
- **Committed wasm build artifacts untracked (repo hygiene):** `static/bin/main.wasm` (â‰ˆ27 MB, rebuilt and
  re-committed on every change), the stale `bin/main.wasm` + its hot-reload manifest, and a stray
  `internal/screens/static/bin/main.wasm` were all git-tracked because `.gitignore` only ignored the old
  `/web/bin/` path. They're now untracked and ignored (`static/bin/`, `/bin/`). Deploy is unaffected â€”
  GitHub Pages CI rebuilds `web/bin/main.wasm` fresh and serves `web/`, never these files. Also untracked
  four stray review screenshots under `bin/` (`dash*.png`, `mobile*.png`) â€” unreferenced and misplaced
  (review captures belong in the already-ignored `.review-screenshots/`); `bin/` is now ignored wholesale.
- **Dead `stub` placeholder helper (internal):** the `screens.stub(...)` "Planned Â· Phase N" placeholder
  is no longer referenced now that every screen is built, so it was deleted (the project bars dead code).
- **Dead `budgeting.matches` helper (internal):** the exact-category `matches(...)` helper was superseded by
  inline cover predicates in `Spent`/`Evaluate` and had no callers; surfaced by a coverage audit (0%) and
  removed.

### Fixed
- **Allocate breakdown no longer runs the score into "returns" (Â§6.15).** The ranked-suggestion subline rendered
  "Score 60%returns 100 Â· â€¦" because the score and breakdown were adjacent inline spans with no separator.
  Added an explicit "Â·" separator so it reads "Score 60% Â· returns 100 Â· stability 100 Â· â€¦".
- **Keyboard focus ring restored on the passcode and command-palette inputs (Â§6.18).** These raw-DOM inputs set
  `outline:none` inline, which beats the global `:focus-visible` rule and left keyboard users with no visible
  focus indicator on the lock screen and command palette. Dropped the inline `outline:none` from all three
  (passcode, passcode-setup, palette search) so the accent focus ring shows again.
- **Switching the time period no longer drifts the view backward in time (C41).** Changing Week / Month /
  Quarter in the top bar now re-anchors to the period that contains today (this week/month/quarter),
  instead of re-snapping the old window's start â€” which used to land you on, e.g., June's *first* week or
  even the previous quarter, and compounded with each switch. Every switch now shows the current period.
- **Saving a workflow no longer silently drops the action you just typed (C37).** If you fill in an
  action and click *Save workflow* without first clicking *Add action*, that action is now folded into the
  saved workflow instead of being lost (which previously made Save look like a no-op). *Add action* also
  tells you when a field is empty rather than staging a blank action.
- **Transactions form controls are now labelled for screen readers (C47).** The filter/sort/bulk bar and
  the add/edit forms had bare `<select>`s and date inputs that a screen reader announced as just "combo
  box" / "edit text". Each now carries an `aria-label` (Type, Account, Category, Member, From/To date,
  Cleared status, Sort by, Filter by account/category, â€¦). The same fix now also covers the **budgets,
  goals, and accounts** add/edit forms (category, owner, period, type, linked-account, and target/lock
  date controls), the **planning** recurring-item form (cadence/account/category), and the **settings**
  panel (base currency, budget method, AI model, display scale, date format, language), and the top-bar
  time-period **"Jump toâ€¦" select** â€” completing the C47 form-labelling pass.
- **The top bar no longer shows a scrollbar â€” it wraps instead (C34).** When the breadcrumb, time
  controls, and "+ Add" don't fit (notably in Custom-range mode around 1100px wide), the bar now wraps
  onto a second row at any width instead of becoming a horizontal scroll container that stole height.
- **The left rail no longer shows a scrollbar (C31).** When the nav overflows (e.g. as "My pages" grows)
  it stays scrollable by wheel/trackpad/keyboard, but the native scrollbar is hidden, matching the clean
  sidebar look.
- **No more browser prompts â€” Goal "Contribute" and Account "Set balance" use in-app forms (Â§6.8).** Both
  now reveal an inline amount field (Add/Cancel), matching the inline-edit pattern, instead of a native
  `window.prompt` â€” better on mobile, keyboard-consistent, and styled. This removes the last `window.prompt`
  from the screens.
- **Passcode lock now actually blocks the keyboard (B17).** While the unlock gate is up, the global
  shortcuts (Alt+1â€“9, Alt+N, Cmd/Ctrl+K) were still firing as document-level listeners â€” so a "locked" app
  could be navigated or have the command palette opened behind the gate. The shortcut handler now bails
  whenever the gate is showing, and the gate **traps Tab focus** within its own controls so the covered
  background can't be reached by keyboard; the gate's own passcode input keeps working.
- **The multi-currency editor actually works now.** Settings â†’ Base currency and the exchange-rate inputs
  were inert stubs â€” the base-currency `<select>` had no change handler and the rate inputs no handler, so
  neither could be changed (and there was no way to add a rate for a currency not already in the table).
  Now: changing the base currency saves and re-windows every currency-aware figure (net worth, period
  totals, budgets, forecasts â€” all already convert via the FX table); each registered currency shows an
  editable rate row (`1 EUR = â€¦ USD`) that commits on blur (so decimals like `1.08` aren't mangled) and
  clears when blank. The model + ledger conversion already existed (`Settings.FXRates`, `currency.Rates`);
  this wires up the editor. Adds `currency.Codes()` (table-tested).
- **Segmented controls support arrow-key navigation (UX audit Â§6.6).** Shared radiogroups now move with
  Left/Up and Right/Down keys, wrapping across options. Browser verification covered the period selector.
- **Workspace switcher actions have clearer separation (UX audit Â§6.4).** The menu divider now carries
  top padding as well as vertical margin, giving management actions more breathing room. Browser
  verification covered the rendered divider class.
- **Collapsed rail flyout labels are clickable (UX audit Â§6.9).** Hover/focus labels in the icon-only rail
  now accept pointer events instead of letting clicks fall through; hover-state browser verification and
  `gwc verify` both passed.
- **Delete buttons have a larger touch target (UX audit Â§6.1).** `.btn-del` controls now carry an explicit
  32Ã—32px floor instead of relying on the shared 24px icon-button minimum; browser verification confirmed
  the computed size, and `gwc verify` stayed green after the app-lock setup form landed.
- **Selected transaction rows have a real visual state (UX audit Â§6.4).** Bulk-selection checkboxes now get
  an accent background/border when selected instead of relying on the glyph alone. Browser verification
  covered the selected checkbox's computed colors, and `gwc verify` stayed green after the app-lock updates.
- **Soon badges now adapt to light theme (UX audit Â§6.11).** `.badge-soon` keeps its dark badge treatment
  in dark mode and gains a light-theme color override.
- **Form fields have comfortable touch targets (UX audit Â§6.1).** Shared `.field` controls now default to
  44px tall, with compact density still holding a 40px floor.
- **Segmented controls are easier to read (UX audit Â§6.2).** Shared `.seg-btn` labels now use 0.85rem type
  instead of 0.8rem while preserving the compact control shape.
- **Settings accent swatches meet the 24px hit-area floor (UX audit Â§6.11).** Theme accent chips now render
  at 24Ã—24px instead of 22Ã—22px.
- **Priority badges are less cramped (UX audit Â§6.2).** To-do priority chips now use 0.75rem text and a
  little more metadata spacing, keeping compact rows readable. Browser verification confirmed the computed
  badge size and gap.
- **Disabled buttons now read as disabled (UX audit Â§6.4).** Shared `.btn` disabled styling dims inactive
  actions, suppresses hover brightening, and switches the cursor to `not-allowed`.
- **Upcoming bill dates honor the display preference (UX audit Â§6.3).** The dashboard bills widget now uses
  the shared date formatter instead of hardcoding `Jan 2`.
- **"When a transaction is added" workflows now fire from every add path (was: quick-add only).** The
  trigger was wired into a single screen, so adding a transaction via the inline editor, a transfer, a
  duplicate, or CSV/image import never ran the workflow. Firing is now centralized in `PutTransaction`
  (on genuinely new transactions, not edits), so all add paths honor the trigger. Bulk imports fire it
  once (not once per row) via a suspend guard, and applying a workflow's effects can't recursively
  re-fire it.
- **Workflow "create task" no longer piles up duplicates.** A repeatedly-firing workflow (e.g. on every
  transaction in a month) created a new identical task each time; it now skips when an open task with the
  same title already exists.
- **Currency KPI widgets never drop a cent** (now rounds `value Ã— 100` instead of truncating) â€” confirmed
  by the new `internal/widgetdata` tests rather than only a screenshot.
- **Rail section labels are easier to read (UX audit Â§6.2).** Sidebar group labels now use 11px type with
  calmer tracking, reducing clipping risk while keeping the compact rail rhythm. Browser verification covered
  the rendered "Tools" label class.
- **Rail navigation items have a real minimum hit area (UX audit Â§6.1).** Sidebar nav rows now carry
  explicit `min-w-10 min-h-10` guards, so icon-only collapsed items stay comfortably tappable instead of
  relying only on padding. Browser verification covered the Dashboard row carrying both guards.
- **Error toasts linger longer + a labelled dismiss (Â§6.9).** Error notices now stay up 7.5s (vs 4.5s for
  ordinary notices) so there's time to read what failed, and the toast's dismiss button gained an
  `aria-label` to go with its title. (Errors already announced assertively via `role="alert"`/`aria-live`.)
- **Currency KPI widgets no longer drop a cent.** A custom-page KPI formatted as currency truncated
  `value Ã— 100` to an int, which floating-point error could round down (e.g. $15,343.50 â†’ $15,343.49). It
  now rounds to the nearest minor unit. Found during the custom-pages/workflow end-to-end pass (10 user
  stories, `internal/appstate/scenarios_test.go` + browser verification; see `docs/CUSTOM_PAGES_STORIES.md`).
- **Custom-field keys are validated before they can pollute data (UX audit Â§6.10).** Custom field
  definitions now reject keys with spaces, punctuation, or reserved metadata names; the add-field form
  also exposes the allowed letters/numbers/underscore pattern to the browser before save.
- **Allocate score bars are labelled for sighted and assistive users (UX audit Â§6.10).** Each allocation
  suggestion now shows an inline `Score N%` label and exposes its bar as a real `progressbar` with
  `aria-valuenow`, so the rank score is no longer a purely visual fill.
- **Add-menu button uses the shared radius utility (UX audit Â§6.4).** The top-bar **+ Add** button no
  longer carries an inline `border-radius` style; it now uses `rounded-[4px]` with the rest of the app's
  utility-class styling and keeps its visual shape in the same class-based path as neighboring controls.
- **Small UX polish (Â§6.3/Â§6.4).** Progress bars are a touch thicker (`h-1.5` â†’ `h-2`) so they read in
  dense layouts; the workspace-switcher dropdown's action-group separator gets more breathing room
  (`my-1` â†’ `my-2`).
- **Light-theme contrast & toggle target size (WCAG, Â§6.11 CSS).** The light theme's idle icon controls
  (`.gear-inline`/`.gear-abs`/`.menu-btn`/`.set-close`) were `#8a8a90`/`#8a8a92` on the `#f7f6f3` light
  background (~2.7:1, below the 3:1 UI threshold) â€” darkened to `#6a6a72` (~5:1). The Settings toggle
  switch was a 36Ã—21px hit area (under the 24px minimum); enlarged to 40Ã—24 with a proportionally larger
  knob.
- **Accessibility pass â€” text contrast & touch-target sizes (WCAG AA, Â§6.1â€“6.2 CSS).** Muted text now
  meets AA: the `faint` token went `#6c6c72` â†’ `#7d7d85` (was ~3.1:1 on the base, used for rail section
  headers, breadcrumb separators, the "New page" link) and `dim` `#a6a6ac` â†’ `#ababb3` (row meta, budget
  sub-text). Interactive targets grew toward the 24â€“44px minimums: form `.field` padding raised with a
  38px floor (36px under compact), the to-do `.check` checkbox is now a centered 24Ã—24 grid, `.btn-del`
  padding bumped, and the native color picker enlarged 46Ã—34 â†’ 44Ã—44. Also nudged the oversized
  `.insight-dot` (1.05rem â†’ 1rem) back into balance with the body type.
- **Deep-link refresh works on nested routes (e.g. `/p/<page>`).** Refreshing a custom-page URL showed
  "wasm_exec.js failed to load": the relative asset paths (`./wasm_exec.js`, `./bin/main.wasm`) resolved
  against the route's directory (`/p/`) and 404'd. Added a `<base href>` set at the very top of `<head>`
  (server root for local/custom domains, `/<repo>/` on `*.github.io`), so assets resolve to the app root at
  any depth â€” fixing both the dev server and GitHub Pages 404-shell deep links. The skip-to-content link is
  now anchored to the live path so the base tag doesn't turn it into a root navigation.
- **A new workspace now starts empty instead of cloning the current one's data.** "+ New workspace"
  was clearing only the canonical `cashflux:dataset` key; boot then saw an empty dataset key and
  re-seeded the Michael Brooks demo sample â€” so a freshly created workspace looked like a copy of the
  current (sample-based) one. `createWorkspace` now persists `store.Export(store.EmptyDataset())`
  explicitly: a clean slate with one default "You" member, USD base currency, and no accounts /
  transactions / budgets / goals. (`duplicateWorkspace` still copies the current data on purpose;
  that's the deliberate "clone this workspace" path.) New `store.EmptyDataset()` + `TestEmptyDataset`
  cover the blank starting point and its exportâ†’import round-trip.
- **All icons now render (and the sidebar collapse button is visible again):** inline SVG icons across
  the app â€” the left-rail nav glyphs, the top-bar menu/collapse toggle (which is icon-only, so it had
  no visible affordance), the household gear, and the per-tile grip/gear â€” were invisible. Root cause was
  in the framework: the wasm DOM renderer built every node with `document.createElement`, placing SVG
  elements in the HTML namespace where they never paint. Fixed upstream in GoWebComponents
  (`createElementNS` for SVG tags) and re-pinned the module here
  (`v1.1.1-0.20260618120835-bfe3011d7f39`). Screenshot-verified on the live dashboard.
- **A few user-facing strings now go through the language catalog (i18n):** the "Enter a valid opening
  balance" validation message, the dashboard "Couldn't create the reminder" toast, the dashboard
  tile resize-handle tooltips, and the spending-breakdown "Other"/"Uncategorized" labels were hardcoded
  English. They're now resolved via `uistate.T` like the rest of the UI, so they translate with everything else.
- **Form errors are tied to their input for screen readers (B15):** each add-form's validation error now
  carries a stable `id` and the form's primary input references it via `aria-describedby` (plus
  `aria-invalid`) while the error is showing. Previously the error only announced once via `role="alert"`;
  now a screen reader re-announces it whenever focus returns to the field. Applied to all 11 add-forms
  (accounts, budgets, categories, custom fields, goals, members, rules, to-do, transactions, and the
  planning recurring & plan forms) via a shared `errAttrs`/`errText` helper.
- **Default accent now passes contrast on both themes (B15):** the out-of-the-box accent changed from the
  mint green `#54b884` (which failed WCAG AA-UI on the light theme at ~2.1:1) to seagreen `#2e8b57`, chosen
  with `internal/contrast` to clear the 3:1 UI/large threshold against **both** surfaces (dark 4.09:1,
  light 3.63:1). The accent drives the focus ring and large strokes, so it has to read on whichever theme
  the user picks. Also updated the swatch palette's default entry and the chart stroke fallbacks to match.
- **Accessibility polish (B15):** the icon-only widget gear and the accounts "â‹¯" overflow button now carry
  explicit `aria-label`s, and the decorative drag grip is `aria-hidden`, so screen readers announce the
  controls correctly. (Reduced-motion already covers the new tile animations, and the layout reflows at
  200% zoom â€” both verified.)
- **Budgets has a single period control now (C7):** the Budgets card had its own `â€¹ January 2006 â€º`
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
  "âˆ’4.50" values; they now format through the same accounting formatter as the rest of the app
  (parentheses for negatives, grouped, in the chosen account's currency), with a raw fallback while a
  value is still being edited.
- **CSV import accepts its own documented format (C27):** pasting the on-screen example
  `date,payee,amount,account` failed demanding an undocumented `currency` column (and leaked a raw
  `store:` error). Currency is now optional â€” it defaults to your base currency â€” and the account /
  category / member columns accept either an ID or a **name** (resolved case-insensitively to the right
  entity). The friendly `account`/`category`/`member` headers work alongside the export's `*_id` headers
  (the explicit ID wins). The import error no longer shows the internal `store:` prefix. Covered by new
  table tests.
- **List-row action buttons wrap instead of overlapping at narrow widths (C19):** on a phone/tablet the
  transaction row's buttons (Mark cleared / Edit / Duplicate / âœ•) overlapped the description and date.
  Rows now wrap below 1024px so the actions flow under the text. Shared by every list screen; a no-op
  when the row still fits. Verified the wrap mechanism in a headless browser. SW cache (v9 â†’ v10).
- **Dashboard KPI figures no longer clip on tablets (C19):** between the phone breakpoint and the
  desktop the 4-column bento squeezed tiles to ~150px and figures like "$20,749.25" clipped. A tablet
  layout (768â€“1024px) now flows the tiles into two columns (the header stays full-width), so figures fit.
  Verified live at 900px (no clipped figures, KPI tiles ~315px, no horizontal scroll). SW cache (v8 â†’ v9).
- **The collapsed/expanded sidebar state now survives reloads (C20):** collapsing the rail was a
  transient choice lost on refresh. It's now persisted to localStorage (like the other UI prefs) and
  restored on load. Combined with C15 (collapse keeps the nav icons), collapsing the sidebar is now
  usable rather than reading as "the panel disappeared." Verified live: toggling writes the stored flag
  and the rail goes 58pxâ†”240px. (An on-panel collapse chevron is still a separate UX item.)
- **The widget gear now appears only where there's something to configure (C21):** the four KPI tiles
  and the cashflow/bills/freshness tiles have no settings, but their gear still opened the empty "This
  widget doesn't have any settings yet" panel â€” reading as broken. The gear now renders only on tiles
  with a settings schema (or an explicit action); the rest get an inert, equal-width slot so the header
  stays balanced. The gear also brightens on tile hover/focus so per-tile settings are more discoverable.
  Verified live: 8 configurable tiles show a real gear, 8 non-configurable tiles don't. SW cache (v7 â†’ v8).
- **Top-bar controls are reachable on tablets and phones (C19):** below 1024px the time-resolution
  control + "+ Add" ran off the right edge with no wrap, so some were unreachable and the breadcrumb was
  clipped to "D". The bar now grows to two rows â€” breadcrumb on top, the controls wrapping onto a
  full-width row below. Verified live at 768px (bar ~175px, breadcrumb readable, nothing past the
  viewport) and 390px (all controls reachable, no horizontal scroll). SW cache bumped (v6 â†’ v7).
  (Transaction-row action-button wrapping and KPI figure clipping at squeezed widths remain open under
  C19.)
- **Inline-edit now lays out like the Add form on every screen (C18):** editing a **Transactions** or
  **Accounts** row stacked its fields vertically in a narrow left column (tall, with empty space to the
  right), while **Budgets** edited horizontally. The edit form (already a `form-grid`) was wrapped in the
  flex `.row`, which shrink-wrapped it to a single 150px column. It now uses a full-width `.row-edit`
  block, so the grid expands to multiple columns and editing matches adding. Verified: the grid yields
  3 columns at 600px in `.row-edit` vs 1 in the old `.row`. SW cache bumped (v5 â†’ v6).
- **Collapsing the sidebar no longer hides all navigation (C15):** the collapsed rail showed only the
  brand mark and the active highlight â€” no nav icons â€” so you couldn't navigate while collapsed. The CSS
  rule that hides the "TOOLS"/"SYSTEM" section labels (`nav > div`) also matched every nav item, because
  the framework wraps each item in a `<div>`. The section labels now carry a `rail-section` class and the
  rule targets only those, so the icon buttons stay visible (and B5's hover-flyout label works). The same
  fix covers the <768px mobile rail, which had the identical bug. Verified live (collapsed rail shows all
  14 icons; both section headers hidden). SW cache bumped (v4 â†’ v5).
- **Period totals no longer silently drop first-of-period transactions (C1):** the Dashboard Income KPI
  read `$0.00` for a month that clearly held a $4,200 salary dated the 1st. Period windows were built at
  the machine's *local* midnight while transaction dates are stored at UTC midnight, so on any machine in
  a timezone behind UTC the month-start landed *after* a `00:00Z` first-of-month transaction and excluded
  it. Period boundaries are now UTC-midnight calendar dates throughout (`dateutil`, `period`), matching
  the UTC-dated transactions. Added a table test that a `00:00Z` first-of-month transaction is counted
  regardless of the machine timezone. Income KPI now shows `$4,200.00` (verified live).
- The **net-worth trend chart** Y-axis is now readable and correct (C16): it plotted raw minor units
  (cents), so the axis showed clipped, non-monotonic labels like "000,000 / 500,000". The chart now
  plots major units (dollars) and formats ticks as compact currency â€” `$0 / $5k / $10k / $15k / $20k`
  (verified live in a headless browser). The D3 shim now honors the per-axis `format` hint
  (`chartspec.Axis.Format`). Service-worker cache bumped (v3 â†’ v4) so returning users get the new shim.
- The **quick-add** transaction panel no longer floats in a tall, mostly-empty card: the panel height
  is now sized to its compact form (420px instead of the default 470px) with the body still scrolling
  if it ever overflows. Verified live in a headless browser (panel opens at 420px on "+ Add"). (C13)
- The **Accounts** add/edit form's asset inputs no longer clip their labels ("Expected returr",
  "Liquidity 0â€“10â€¦"): the placeholders are now short ("Return %", "Liquidity", "Stability") with the
  full label + range on hover (`title`). (C9)
- **Mobile/responsive layout (C10):** below 768px the app no longer scrolls horizontally with the
  content pushed off-screen â€” the sidebar collapses to an icon rail, the main area takes the full
  width, and the dashboard bento stacks into a single column. Verified in a headless browser at 390px
  (no horizontal overflow). Desktop is unchanged.
- The **Insights** screen is no longer near-empty without an OpenAI key: the "Ask about your money" box
  now always shows (a disabled preview + a hint to add a key when none is set), advertising the feature
  â€” the offline Spending-highlights card already displayed. (C9)
- The last row of the **settings panel** (e.g. "Display scale") is no longer clipped against the
  sticky footer â€” the scrollable body now has extra bottom padding so it clears the fold. (C12)
- The rail's **household card** summary no longer repeats "Settings" (the gear icon and tooltip already
  convey it) â€” it reads "N members Â· USD base". (The earlier "GWC avatar overlap" symptom was from the
  old mockup and is gone in the current flex layout.) (C3)
- Money amounts everywhere now show **thousands grouping** (e.g. `$20,749.25` instead of `$20749.25`)
  â€” Accounts, Budgets, Goals, Allocate, etc. that used the ungrouped `fmtMoney` are fixed in one place. (C2)
- The top-bar **time-resolution control** (Week/Month/Quarter + period stepper) now appears only on
  period-aware screens (Dashboard, Transactions, Budgets, Planning, Insights) â€” it's hidden on Members,
  Categories, Rules, Customize, Allocate, Documents, To-do, and Goals where a period does nothing. (C4)
- **Categories** can now have a **color** and show it: a color swatch appears on each category row, and
  the Add/Edit category forms have a color picker (the `Color` field existed in the model but was never
  surfaced). (C9)
- The **member color picker** (Add/Edit member) now renders as a proper clickable color swatch with a
  label instead of a thin bare line (it was a native color input squeezed into a text-field style). (C8)
- The dashboard no longer shows two tiles both titled **"Net worth"**: the trend chart tile is now
  titled **"Net worth trend"**, distinct from the net-worth KPI. (C5)
- The Allocate screen no longer lists **zero-score candidates** (accounts with no expected-return/
  stability/liquidity set, which rendered as "0% Â· returns 0 Â· stability 0 â€¦" noise); when that hides
  everything, it nudges you to set those account attributes instead. (C6)
- A widget whose gear opens a settings panel with **no settings** now shows a single **Close** button
  instead of a Cancel/Save pair that implied there was something to commit. (C11)
- Budget rows no longer show a redundant **"Food Â· Food"** when a budget is named after its category â€”
  they show one label; an unnamed budget shows just its category (no leading "Â· "). (C7)

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- The dashboard's **net-worth trend** widget now renders through the new D3 `ui.Chart` (with axes)
  instead of the pure-SVG sparkline â€” the proof of the D3 pipeline. (Other charts still use the
  pure-SVG renderer; this one needs an in-browser check.)

### Added
- Charting (D3): a Go `ui.Chart` component now drives the D3 shim â€” it owns a managed container and an
  effect (keyed on the serialized spec) that hands the element and a `chartspec.Spec` to D3, redrawing
  on data change and clearing on unmount. Ready for widgets to adopt. (Render needs an in-browser check.)
- Charting groundwork (D3): a pinned D3 v7 and a theme-aware `cashfluxRenderChart` shim that draws a
  `chartspec.Spec` (line/area/bar/donut, with axes) are now loaded and service-worker cached for
  offline use; the `chartspec` types are JSON-tagged for the wire format. The Go `ui.Chart` component
  that drives this lands next. (The D3 rendering itself needs an in-browser check.)
- Accessibility: required fields are now marked `aria-required` across the add forms â€” accounts,
  categories, budgets, goals, members, rules, to-do, transactions, plus quick-add and plans â€” so
  screen readers announce which fields must be filled.
- Accessibility: inline **form validation errors** are now `role="alert"`, so screen readers announce
  them the moment they appear (e.g. "Enter a positive target amount") instead of leaving the failure
  silent.
- Each saved plan now shows a small **projected-balance sparkline** of its trajectory over the horizon
  (green if it ends up, red if down) next to its end figure â€” so you can see the shape, not just the
  number.
- Settings â†’ Appearance now shows the **contrast ratio of the chosen accent** against the theme
  surface, with a warning when it's low (e.g. the default green on the light theme) â€” so you can pick a
  more legible accent. Powered by the new `contrast` package.
- Plans can now include a **one-time amount** (a bonus or big expense) in a chosen month, alongside the
  steady monthly change â€” so "what if I also get a $2,000 bonus in month 6" shows up in the projection.
- Accessibility: the **toggle switches** and **accent color swatches** are now fully keyboard-operable â€”
  they're focusable (Tab) and respond to Space/Enter, with a visible focus ring â€” so settings and the
  accent picker no longer require a mouse. (Previously they were mouse-only `<div>`s.)
- New pure `contrast` package: WCAG relative-luminance and contrast-ratio math with AA/AAA pass
  predicates â€” the foundation for checking that colors (especially a user's chosen accent) are legible
  against their background. Fully table-tested (black/white = 21:1, symmetry, known boundary pairs).
- Accessibility: the flip-panel dialogs (quick-add, settings) now **trap keyboard focus** â€” focus moves
  into the dialog on open, Tab/Shift+Tab cycle within it instead of escaping to the page behind, and
  focus returns to whatever you opened it from when it closes. Completes the modal dialog semantics.
- Accessibility: small icon-only buttons (delete âœ•, toast dismiss, dialog close, the time-resolution
  arrows) now have a **minimum 24Ã—24px hit area** with the glyph centered (WCAG 2.5.8) â€” easier to tap.
- Accessibility: with the OS **"reduce motion"** preference on, the flip-panel no longer flips/lifts,
  the toast no longer slides in, and the sidebar resizes instantly â€” the app respects users who are
  sensitive to motion (the boot animation and rail flyout already did).
- New pure `chartspec` package: a **typed, declarative chart description** (kind + series + axes +
  options) with `Validate` and data-`Extent` helpers â€” the framework-agnostic foundation for richer
  charts that any renderer (pure-Go SVG today, possibly D3 later) can consume. Fully table-tested.
- New pure `icon` package: the app's curated line-icon set is now a **type-safe registry** â€”
  compile-checked `Name` constants with `Inner()`/`Valid()`/`All()` â€” so icons can't be referenced by
  a typo'd string. Fully table-tested; the view layer adopts it next.
- The Allocate screen's amount split now has a **"Max per destination"** input â€” cap how much any one
  account/goal/debt can receive, and the overflow is held back (reported in the kept-back note). This
  surfaces the split engine's already-tested per-destination cap.
- Groundwork for the simpler time-resolution control: the period `Window` now knows when it's a single
  period and renders one clean label ("Jun 2026") instead of a redundant "Jun 2026 â€“ Jun 2026", with a
  helper to collapse a range back to a single period.
- The dashboard **Goals widget** is now configurable: feature the **goal nearest completion** instead
  of the first, and optionally hide the target date.
- The dashboard **Budgets widget** is now configurable: cap how many budgets to show (3â€“20, default 6)
  and optionally show **only those near or over budget**, so it can focus on what needs attention.
- The dashboard **Accounts widget** is now configurable: set how many accounts to show (3â€“12,
  default 6) and whether to show only **cleared** balances (reconciled money) instead of current.
- The dashboard **To-do widget** is now configurable: open its gear to set how many tasks it shows
  (1â€“10, default 3), instead of a fixed three.
- Accessibility: the flip-panel overlay (quick-add, household/global settings, per-widget settings) is
  now a proper modal dialog â€” `role="dialog"` + `aria-modal="true"` + an accessible name from its
  title â€” and **Esc closes it**, so screen-reader and keyboard users get expected modal behavior.
- The top bar's **"+ Add"** button now opens a **quick-add transaction** flip panel from anywhere â€”
  pick the account, expense/income, amount, description, category, and date, and save without leaving
  the screen you're on. The result is announced via the toast.
- The Planning screen now has a **Savings & spending plans** card: name a what-if, set a starting
  balance, a monthly change (+ in / âˆ’ out), and a horizon in months, and each saved plan shows where
  you'd land (its projected end-of-horizon balance, toned green/red) â€” backed by the planning engine.
  Plans list and can be deleted.
- Plans now **persist**: saved what-if scenarios survive reloads and round-trip losslessly through
  JSON/CSV export/import, with validated save (needs an id, a name, and a positive horizon).
- New **Plan** model and `planning` engine: a saved what-if scenario (a starting balance projected
  over a horizon under a set of recurring/one-time assumptions) can now be projected into a balance
  curve, its steady monthly net, and its end-of-horizon balance â€” composing the pure domain types
  with the existing forecast engine. Fully table-tested; persistence and the Planning UI come next.
- The Documents screen now shows a **monthly-spend summary** of the rows awaiting import â€” out vs. in
  vs. net per month â€” so you can see what a receipt or statement says you spent before committing any
  rows. Amounts use the chosen account's currency; undated rows are listed under "No date".
- New `spendsummary` package: turns extracted document rows into a **per-month spend summary**
  (money out vs. money in, with net), tolerant of varied date formats and currency symbols, surfacing
  undated rows rather than dropping them. Fully table-tested; the Documents screen view comes next.
- The Allocate screen now exposes the **goal-progress criterion** end to end: a "Goal-progress weight"
  input and a new **"Finish goals"** profile, each goal candidate carries its real completion
  percentage (so weighting it ranks goals nearest the finish line first), and the per-suggestion
  breakdown shows the goal's progress (e.g. "Â· goal 85%"). Saved profiles keep the new weight.
- Saved allocation profiles now remember their **goal-progress weight** too (round-trips losslessly
  through save/load and JSON/CSV export/import; older profiles without it load as 0).
- Capital allocation now has a **goal-progress criterion**: destinations funding a savings goal score
  by how close that goal is to completion (clamped 0â€“100%), so a "finish what's almost done"
  weighting can prioritize goals near the finish line. Fully tested and explainable (it shows in the
  per-criterion breakdown); the Allocate screen's weight control wires up next.
- Accessibility: every screen now has exactly one top-level `<h1>` â€” the page title in the top bar
  is now a real heading (the dashboard's in-canvas title dropped to `<h2>` to match) â€” so
  screen-reader users can jump to the page heading and the heading order is valid.
- Accessibility: the dashboard To-do widget's priority markers no longer rely on color alone â€” high,
  medium, and low now use distinct shapes (â–² / â— / â—‹) and each carries an accessible name
  ("High priority", etc.), so colorblind users and screen readers can tell them apart.
- Accessibility: the app-wide notice (toast) is now a **persistent live region** â€” it stays in the
  DOM while idle so screen readers reliably announce each new notice, and error notices are now
  `assertive`/`role="alert"` (they interrupt) while ordinary notices stay polite. So async outcomes
  (saves, imports, AI results, failures) are spoken aloud, with failures given priority.
- The browser tab and history entry now show the current screen's name (e.g. "Budgets Â· CashFlux")
  instead of a static title â€” so tabs, the back-button menu, and screen readers all name the page
  you're actually on.
- Accessibility: navigating to a new screen now moves keyboard and screen-reader focus into the
  main content region (not on first page load, so the first Tab still reaches the skip link) â€” so
  SPA navigation no longer strands focus on the screen you just left.
- Accessibility: a **"Skip to content"** link (the first focusable element, visible only on keyboard
  focus) jumps past the sidebar to the now-focusable `<main>`, and a clear **focus-visible ring** is
  drawn on every interactive element in both themes â€” so keyboard users can navigate efficiently and
  always see where focus is.
- Accessibility: the time-resolution stepper's â€¹/â€º arrows now have `aria-label`s ("Move start
  earlier", etc.) and the accent **color swatches** are a labelled `role="radiogroup"` of `role="radio"`
  chips (each labelled by its hex, `aria-checked` reflecting the selection) â€” so these icon/color-only
  controls are no longer silent to screen readers.
- Accessibility: the shared **Toggle** switch now exposes `role="switch"` + `aria-checked` + an
  accessible name (from its row label), and the **Segmented** control is a `role="radiogroup"` of
  `role="radio"` buttons with `aria-checked` â€” so every theme/week-start/density/resolution toggle and
  every settings switch announces its state to screen readers (one central change covers them all).
- Accessibility: the SVG trend/forecast charts are now `role="img"` with a descriptive `aria-label`
  (e.g. "Net worth trend, currently $X"), and the sidebar's navigation landmark is labelled "Main
  navigation" (distinct from the breadcrumb nav) â€” so screen readers announce them meaningfully.
- The top bar now shows a **breadcrumb**: off the dashboard it reads "Dashboard â€º <screen>", with the
  Dashboard crumb a keyboard-operable button that navigates home; the current screen is marked
  `aria-current="page"`. On the dashboard it's just the title.
- The collapsed sidebar now **reveals each item's label on hover/focus** as a flyout (no rail
  widening), and every nav item + the household card carry a `title` so the name is available on hover
  and to screen readers when only the icon shows. The flyout respects `prefers-reduced-motion`.
- A **Display scale** setting (Settings â†’ Appearance): pick 70%â€“130% (100% default) to make the whole
  UI larger or smaller â€” applied live via a `--ui-scale` CSS zoom and persisted across reloads. The
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
  **Post due now** button creates real transactions for every due item â€” catching up any missed
  periods and advancing each schedule past today. Backed by a table-tested `appstate.PostDueRecurring`.
- The Recurring cash flows card now shows a **net monthly equivalent** total â€” every cadence
  normalized to a per-month figure and summed (e.g. a yearly insurance bill counts as 1/12 a month),
  so you can see your true monthly commitment at a glance. Backed by a pure, tested
  `domain.Recurring.MonthlyEquivalent`.
- The Planning screen now has a **Recurring cash flows** card: add a bill/paycheck/subscription
  (label, signed amount, cadence) and see/remove the list â€” amounts colored by sign, each showing its
  cadence and next-due date. Backed by the recurring store.
- A **Recurring** cash-flow model (`domain.Recurring` + store/state): a scheduled bill/paycheck/
  subscription with a label, signed amount, cadence (weekly/monthly/quarterly/yearly), next-due date,
  account/category, and an autopost flag. Cadence math (`Cadence.Next`, `Recurring.Advance`) and full
  persistence (CRUD, export/import round-trip, validated `appstate.Recurring`/`PutRecurring`/
  `DeleteRecurring`) are table-tested. The data model behind recurring transactions + richer forecasts
  (management UI + autoposting to follow).
- Hardened the **forecast** and **debt-payoff** engine tests: forecast now pins one-times outside the
  horizon being ignored, same-month one-times summing, negative-horizon â†’ empty, and balances allowed
  to go negative; payoff pins single-month clearing (final payment capped), payment-equal-to-interest
  being non-viable, negative balance treated as paid, and the TotalPaid = principal + interest
  invariant across inputs.
- Hardened the **capital-allocation engine** tests: explicit determinism (Rank + Distribute give
  identical results across repeated runs), tie-stability (equal scores keep input order), and
  breakdown clamping (out-of-range APR/stability/liquidity normalize into [0,1]) â€” pinning the
  "deterministic & explainable" guarantee against regressions.
- Hardened the sandboxed **formula engine** with security + edge-case tests: non-allow-listed/host-like
  functions (`exec`, `eval`, `system`, `import`, even `SUM`/`Sum` â€” the allow-list is case-sensitive)
  are rejected, undeclared variables never silently resolve, evaluation only ever yields a
  number/string/bool, deep nesting and determinism hold, and malformed input errors instead of
  panicking. (`internal/formula` â€” proves the "no escape" guarantee.)
- The Rules screen now flags rules that **never run**: if an earlier rule's phrase already matches
  everything a later rule would (first-match-wins), the shadowed rule shows "Never runs â€” an earlier
  rule (â€¦) already matches it." Backed by a pure, table-tested `rules.Conflicts` detector.
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
- README "Hosting (SPA history fallback)" â†’ a **Local development** note documenting that `gwc dev`
  does not yet serve the app shell for history routes (deep link / hard refresh at `/accounts` 404s;
  only built assets serve), with the workaround (start from `/` and navigate in-app, or run a
  production build behind a rewrite). Empirically confirmed this session; the deployed PWA is
  unaffected. Pins the last open B1 item as a framework-side gap.
- README "Hosting (SPA history fallback)" section documenting the rewrite rule static hosts need
  (unknown non-asset paths â†’ `index.html`) so deep links/refreshes work, with concrete snippets for
  GitHub Pages (the auto-generated `404.html`), Netlify, Vercel, nginx, and Caddy.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- Accessibility: the **faint/secondary text color** was darkened (light theme) / lightened (dark theme)
  so captions, hints, and meta text now meet WCAG AA (4.5:1) against both the base and elevated
  surfaces â€” previously ~3:1, which failed for normal-size text. (Audited with the new `contrast`
  package; the shared brand **accent** is low-contrast on the light theme but changing its hue is a
  brand decision, so it's left for review.)
- Icons are now **type-safe end to end**: `ui.Icon` takes an `icon.Name` and every call site (sidebar
  nav, top bar, household card) uses the compile-checked constants â€” a mistyped icon name is now a
  build error instead of a silently-blank glyph. Rendering is unchanged.
- The top bar's **time-resolution control** is simplified: the common case is now a **single period**
  with one â€¹ Jun 2026 â€º stepper that pages the whole window (reading as one clean label). A **"This
  period"** reset appears only when you've moved off the current period, and the dual From/To range
  steppers are now behind a **"Custom range"** toggle (which collapses back to a single period when
  you leave it) â€” so the 90% single-period case is one tap and ranges stay available for power users.
  A **"Jump toâ€¦"** quick-pick menu offers This period / Last period / This quarter / Year to date in
  one tap.
- The Settings â†’ Screens **show/hide toggles now cover every main-line screen**, including the Tools
  group (Planning, Allocate, Insights, Documents, Customize) and Rules â€” so any nav item except the
  dashboard can be hidden from the sidebar.
- Removed the placeholder **"My pages"** sidebar segment (the example "Debt payoff plan / FIRE tracker /
  Side hustle P&L" entries and the "New page" affordance) â€” they were mockup stubs, not real pages, so
  the rail is now just the actual screens. (Menu visibility is already configurable via the
  module-visibility toggles in Settings â†’ Screens.)
- **One settings entry point** now: the duplicate `/settings` screen is gone â€” its only unique piece,
  the debug-log viewer, moved into the household-card settings panel (where currency/AI/appearance/
  data already live). The "Settings" sidebar item is removed; the household card at the bottom of the
  rail is the single way in. (Module-visibility's locked set is now just the dashboard.)

### Fixed
- The sidebar was missing five routed main-line screens â€” Planning, Allocate, Insights, Documents, and
  Customize were only reachable by typing the URL. Added a **Tools** nav group for them (each with an
  icon, respecting module-visibility toggles), so every main-line screen is now reachable from the menu.
- Deep-link refresh in the installed/offline PWA: the service worker now serves the cached app shell
  for navigation requests, so hard-refreshing a client-side route like `/accounts` boots the app (which
  then routes to that screen) instead of failing on a 404 or while offline. Complements the static
  `404.html` shell that covers the first load on GitHub Pages. (Cache bumped to `cashflux-v2`.)

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- Reading transactions from a receipt/statement image now uses OpenAI **structured outputs**: the
  vision request carries a strict JSON schema, so the model returns a well-formed transactions array
  instead of free-form text coaxed by prompt wording. More reliable extraction; the tolerant parser
  still handles the result. (`ai.BuildStructuredVisionRequest` / `SendStructuredVisionChat`.)
- The data CashFlux sends to OpenAI for insights is now a single explicit, tested
  `ai.FinancialContext` â€” by construction only aggregate totals and an account count, never payees,
  account numbers, or per-transaction detail. Both "Explain my month" and "Ask about your money" build
  their prompt from it, so the privacy scope is reviewable in one place rather than inlined per call.

### Added
- The Documents screen now has an **Import history** card listing every recorded import (newest
  first) â€” kind, date, status, row count, and target account â€” each removable. Completes the document
  lifecycle: import â†’ recorded â†’ reviewable/auditable.
- Importing transactions (CSV paste or receipt/statement image) now records a **Document** in the
  history â€” kind, time, target account, status, and (for image imports) the rows read â€” so every
  import leaves an auditable trail. Recorded best-effort, only when at least one transaction lands.
- An imported-**Document** record (`domain.Document` + store/state): filename, kind (CSV/image),
  upload time, target account/member, a lifecycle status (pending â†’ extracted â†’ imported / failed),
  and the rows read from it â€” persisted with full CRUD, export/import round-trip, and validated
  `appstate.Documents`/`PutDocument`/`DeleteDocument`. Table-tested. The model behind a documents
  history/audit view (recording on import + the list UI are follow-ups).
- A pure codec for OpenAI **structured outputs** (`ai.BuildStructuredRequest`): builds a chat request
  with a `response_format` JSON-schema so the model returns JSON matching a given schema, decodable
  straight into a Go struct instead of parsed out of prose. Round-trip tested. The building block for
  reliable AI extraction (e.g. document parsing) going forward.
- The Rules screen now shows a **Suggested rules** card driven by the suggester: each proposal reads
  "Categorize "Starbucks" as Cafe Â· Seen in 6 transactions" with an **Add** button that creates the
  rule in one click. Suggestions a rule already covers don't appear, and the card hides itself when
  there's nothing to propose.
- A pure, deterministic rule suggester (`internal/rulesuggest`): it studies how you've already
  categorized transactions and proposes auto-categorization rules where a payee/description reliably
  maps to one category â€” appearing often enough, agreeing â‰¥80% of the time, and not already covered by
  a rule â€” ranked by supporting evidence. No AI needed; explainable (each suggestion carries its
  support/total counts). Table-tested. The data behind a future "suggested rules" review on the Rules
  screen.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- AI requests now retry transient failures automatically: a rate limit (429), server error (5xx), or
  network blip is retried up to three times with exponential backoff (0.5s â†’ 1s â†’ 2s) before giving
  up with the plain-English message. Client errors (bad key, unknown model) aren't retried. The
  decision logic (`ai.IsRetryable`, `ai.RetryDelayMS`) is pure and table-tested.
- AI failures now show plain-English, actionable messages instead of a raw error: a rejected key,
  rate limiting vs. spent quota, an unknown model, and server trouble each get their own guidance
  (e.g. "OpenAI didn't accept your API key. Check it in Settings."), and a network/CORS failure says
  to check your connection. Backed by a pure, table-tested `ai.ErrorMessage(status, body)` and an
  HTTP-status check in the fetch transport.

### Added
- Settings â†’ AI now offers a fuller **model picker** (GPT-4o mini, GPT-4.1 nano/mini, GPT-4o,
  GPT-4.1, o4-mini) â€” all models the cost estimator knows, so token-cost surfacing stays accurate â€”
  and shows an "AI features stay off until you add a key" hint while no key is set, reinforcing the
  local-first, bring-your-own-key model.
- Insights now shows token usage and approximate cost after an AI answer â€” "Used 1,234 tokens Â·
  about $0.0019" â€” using the call's reported usage and the model's pricing (just the token count when
  pricing is unknown). The fetch transport now hands the token usage back alongside the content.
- A pure AI cost estimator (`ai.EstimateCostUSD` + `ai.FormatCostUSD`): a per-model price table turns
  a response's token usage into an approximate USD cost, with longest-prefix matching for dated model
  variants and sub-cent amounts shown to four decimals. The foundation for surfacing "this used ~N
  tokens (~$0.00x)" after a call; table-tested.
- The Rules screen has an **Apply to existing** button that retroactively categorizes every
  uncategorized, non-transfer transaction matching a saved rule (first match wins, adding the rule's
  tags when a transaction has none) and reports how many it updated. This is the clean way to apply
  rules to transactions added via the CSV-paste path or imported before a rule existed â€”
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
  phrase â†’ category, with optional comma-separated tags), see all rules, edit any rule inline, and
  delete. Client-side validation shows friendly messages (match phrase + category required), and the
  hint explains first-match-wins. Built on the persisted rule store.
- Auto-categorization rules are now persisted: a `rules` table in the store with full CRUD
  (`PutRule`/`GetRule`/`DeleteRule`/`ListRules`), inclusion in the export/import dataset (lossless
  round-trip), and validated `appstate` accessors (`Rules`/`PutRule`/`DeleteRule` â€” a rule needs an
  id, a non-empty match phrase, and a target category). The store/state foundation for the rules
  management UI and apply-on-entry; table-tested at both the store and appstate layers.
- The dashboard now has a **Spending highlight** widget: it surfaces the single most significant
  spending change this month (reusing the same anomaly detection as the Insights card) as a one-line
  plain-English highlight with a green/red marker, or a calm "no big changes" message. Draggable and
  resizable like the other bento tiles. The anomaly detection + sentence rendering are now shared
  helpers (`detectSpendingAnomalies`, `highlightText/Tone/Arrow`) between the dashboard and Insights.
- Insights now shows an offline **Spending highlights** card: it detects categories whose spend this
  month deviates materially from their recent average (via `ledger.CategorySpendSeries` â†’
  `insights.Detect` over the last four months) and explains each in plain English â€” "Dining spending
  is up 90% â€” $90.00 this month vs about $47.00 a month" â€” with a green/red up/down marker, most
  significant first. No AI key required; the card simply doesn't appear when nothing is notable.
- `ledger.CategorySpendSeries` buckets non-transfer expense into consecutive periods (defined by a
  list of boundaries) and returns each category's per-period spend in base-currency minor units,
  oldest first â€” the feeder that turns transactions into the per-category histories `internal/insights`
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
  local storage. Added a **License** section pointing at `LICENSE` â€” closing the README's live-demo
  link and the MIT item's README follow-up.
- The project is now licensed under the **MIT License**: added a top-level `LICENSE` file (standard
  MIT text, 2026, monstercameron) and established the lightweight per-file convention with a one-line
  `// SPDX-License-Identifier: MIT` marker in the `main.go` entrypoint (placed above the `//go:build`
  constraint so the wasm build is unaffected). The README "License" section/badge will land with the
  README; a full tree-wide SPDX sweep is intentionally deferred to avoid churn and build-tag fragility.
- A CI guard for the source-of-truth English message catalog (`internal/i18n` `TestDefaultCatalogQuality`):
  every key must be dot-namespaced with no whitespace, and every key must define a non-empty string â€”
  so a blank or malformed entry (which would silently surface the raw key in the UI) fails `go test`
  in CI instead of shipping. Suffix fragments and literal `%` are intentionally left unconstrained.
- A Phase 0 backlog item to set the project up under the **MIT license** (`TODOS.md` Â§0): add a
  top-level `LICENSE` file, light SPDX (`// SPDX-License-Identifier: MIT`) references per repo
  convention, and a "License" section + badge in the README.
- A new "Future / nice-to-have (post-core)" backlog section (`TODOS.md` Â§5) for enhancements to pick
  up only after the core product (Phases 0â€“3) is complete. First item: **standalone desktop app via
  Electron** (Â§5.1) â€” wrap the existing Goâ†’wasm / PWA build as a native installable desktop app,
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
  silently â€” and failures that were previously swallowed now surface. "Mark all updated" also
  reports how many balances it refreshed.

### Fixed
- The dashboard's net-worth "this month" change percentage was computed inline and divided by the
  signed baseline, so it showed the wrong direction when net worth was negative (a move from âˆ’$1,000
  to âˆ’$500 read as a decline). Extracted into a pure, tested `ledger.PercentChange` that divides by
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
  resolution is remembered â€” the view re-anchors to the current period on load, so you keep your
  preferred granularity without landing on a stale week or month.

### Added
- The per-widget settings panel (gear â†’ flip) is now **schema-driven and persisted**: it renders the
  widget's registered `widgetcfg.Schema` (toggle / number / select) bound to a localStorage-backed
  `WidgetConfigs` atom, so changes survive reloads. Savings rate is the first widget with real
  settings (target rate %, show progress bar); widgets without a schema show a friendly placeholder.
- The Savings rate widget now reflects its settings: it compares the actual rate against your target
  (green at/above target, amber when positive but short, red when negative) and shows the target in
  the subline; the progress bar can be hidden.
- The Recent transactions widget has a "Rows to show" setting (3â€“20, default 6).
- The Net worth trend widget has a "Months of history" setting (3â€“12, default 6).
- The Spending breakdown widget has a "Top categories" setting (2â€“6, default 3; the rest group as Other).
- GitHub Pages deployment via Actions (`.github/workflows/deploy-pages.yml`): every push to `main`
  builds the wasm app and publishes it to Pages, so the latest build is reviewable from anywhere. A
  `404.html` app-shell is generated for deep-link routing.
- A per-widget settings API (`internal/widgetcfg`): each dashboard widget registers a typed `Schema`
  (toggle/number/select fields with defaults and bounds), and reads its values from a persisted
  `Config` via clamping/validating accessors â€” the bridge between a widget's flip-panel settings and
  its content. Pure and table-tested; savings rate ships the first schema (target rate + show-bar).
- Settings â†’ Languages: a **Display language** picker lists every language the bundle carries and
  switches the whole UI to it. The choice persists to `localStorage` and applies on a reload, so all
  rendered strings re-resolve in the chosen language (English remains the fallback for any
  untranslated key). Completes the central-language-store loop: pick, export, import.
- Settings â†’ Languages: **Export languages** downloads the whole language bundle as JSON (for
  translators) and **Import languages** loads a translated bundle back, merged and persisted across
  reloads â€” the round-trip for every language the app supports.
- The sidebar verbiage now flows through the language store: the brand, primary + System nav labels,
  the "My pages"/"System"/"New page" headers, and the household card all resolve via `uistate.T(key)`
  against the English catalog (no visible change â€” first screen migrated onto i18n).
- The top bar's chrome (menu-toggle tooltip and the "+ Add" button + its tooltip) now resolves via
  `uistate.T` too, completing the app-shell verbiage migration.
- The To-do screen's verbiage is now fully on the language store (form labels/placeholders, priority
  options, empty/all-done states, hide-done toggle, row actions, validation message), with shared
  `priority.*` and `common.notReady` keys other screens can reuse.
- The Members screen's verbiage is now on the language store too (add form, reassign-before-delete
  panel, member rows incl. make-default/transactions/edit/delete, net-worth-by-member, validation),
  with a shared `owner.group` key.
- The Transactions screen is now fully on the language store â€” the main view plus each transaction
  row's inline edit form, the category/transfer/uncategorized labels, the cleared status, and all row
  actions. **This completes the app-wide verbiage migration: every screen now renders through i18n.**
  (A few intentional exceptions stay literal: account-type names via `humanizeType`, currency/AI-model
  display names, date-format examples, and OpenAI prompt instructions.)
- The Accounts screen is now fully on the language store: the main view plus each account row's inline
  edit form, the update-balance prompt, the stale badge, the cleared-balance meta, and all row actions
  (view / update balance / mark updated / edit / archiveÂ·restore / delete).
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
  `Rates.FormatInBase(m)` â€” convert a Money through the rate table and render it accounting-style in
  the target/base currency (symbol, decimals, negatives in parentheses).
- Pure, table-tested time-period presets in `internal/period` (`Previous`, `YearToDate`) plus
  `Window.Shift` (page the whole window as a unit) and `Window.IsCurrent` (is this the current period)
  â€” the foundation for the planned resolution-control redesign (B10). Not yet wired to the UI.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- Extracted the dashboard's inline savings-rate calculation into a pure, table-tested
  `ledger.SavingsRate(income, expense)` (0 when income is non-positive; negative when overspent) â€”
  one more KPI computation moved out of view code.
- Moved the upcoming-bills "next due date" math out of the js-only dashboard into pure, table-tested
  `dateutil.NextMonthlyDue(now, day)` (next monthly due on/after today, day clamped to 1â€“28 so it's
  valid every month).
- Extracted the account credit-utilization calc into pure, table-tested `ledger.Utilization(balance,
  limit)` (uses the balance magnitude; ok=false when there's no limit) â€” the Accounts liability rows
  delegate to it.
- Added a pure, table-tested ordered-sequence + bin-packing model to `internal/dashlayout` (`Item`,
  `Pack`, `Move`, `ResizeItem`) â€” the foundation for iOS-home-screen-style dashboard reflow (drag =
  reorder + re-pack, multi-cell tiles never overlap). Not yet wired to the UI; the legacy
  placement/swap API stays until the dashboard is migrated. (Backlog B2.)
- Extracted the to-do list ordering/filtering into a pure, table-tested `internal/tasksort` package
  (`Order` + `Visible`); the to-do screen now delegates to it. No behavior change â€” the rules (open
  first, soonest due, then title; optional hide-done) are now unit-tested instead of inline in the
  js-only screen.
- Extracted transaction filtering/sorting into a pure, table-tested `internal/txnfilter` package
  (`Criteria` + `Apply` + `AbsAmount`); `uistate.TxFilter` now aliases `txnfilter.Criteria` and the
  ledger screen delegates to it. No behavior change â€” a core behavior is now unit-tested instead of
  living only in the js-only screen.

### Docs
- Refreshed the CLAUDE.md status section to reflect the now-comprehensive feature set (full
  CRUD/inline-edit, reconciliation, sub-categories, budget periods, preferences/themes, document
  vision import, allocation split, AI insight/nudge â†’ tasks); multi-device sync noted as the sole
  remaining major item.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
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
- Filtered-transactions summary: the ledger shows "N shown Â· net $X" for the current filter â€” the
  count and net total (converted to base currency), updating as you filter.
- Per-member "Transactions" drill-down: each member row links to the ledger filtered to that
  member, matching the per-account drill-down.
- "Update balance" on account rows: enter an account's real balance and the app posts a cleared
  "Balance adjustment" transaction for the difference and marks it checked today â€” reconciling the
  computed balance to a statement without hunting for the missing entry.
- "Remind me" on the dashboard freshness nudge: when balances are stale, one click adds a
  Nudge-sourced to-do ("Update stale account balances") and jumps to the list â€” the create-from-nudge
  hook, completing both AI/nudge â†’ to-do paths.
- "Save as task" on Insights: turn an AI answer/explanation into a to-do (full text in notes, source
  tagged AI) â€” wiring the create-from-insight hook so suggestions become actionable.
- Per-account "Transactions" drill-down: each account row has a button that filters the ledger to
  that account and jumps to it (sets the persisted transaction filter, then navigates).
- "Mark all updated" on the Accounts screen: when any balances are stale, a one-click action stamps
  every stale account as checked today, clearing the stale badges (and the dashboard freshness
  nudge) at once.
- Account "locked until" date (assets): set on the add form and the inline editor (blank unlocks),
  and the Allocate screen excludes an account locked until a future date from its suggestions (you
  can't add money to it yet).

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- The dashboard spending breakdown now rolls sub-category spend up to its top-level parent category,
  so e.g. Restaurants and Groceries are counted under Food â€” a cleaner high-level view.

### Added
- Re-parent categories: the category inline editor now has a parent picker too (same-kind, self
  excluded), so an existing category can be nested, moved, or promoted to top level.
- Sub-categories in the Categories screen: the add form has a parent picker (categories of the same
  kind, indented), and the category lists now display the parent/child hierarchy indented (via
  `categorytree.Flatten`). Lets you nest e.g. Restaurants and Groceries under Food.
- Category hierarchy engine (`internal/categorytree`): `Build` organizes a flat category list into a
  parent/child forest (siblings sorted by name) and `Flatten` returns a depth-tagged list for
  indented display, using the existing `Category.ParentID`. Defensive â€” orphans become roots and
  cycles are dropped rather than looping. Table-tested. Foundation for sub-categories.
- Cleared balance on accounts: a pure `ledger.ClearedBalance` (opening balance + only cleared
  transactions) is shown on each account row when it differs from the live balance â€” the figure to
  reconcile against a statement. Tested.
- Bulk mark transactions cleared/uncleared: the selection bar now has "Mark cleared" and "Mark
  uncleared" actions, so you can reconcile many at once.
- Filter transactions by cleared status (cleared / not cleared / both), persisted with the rest of
  the transaction filter â€” pairs with the cleared toggle to make reconciling against a statement
  easy (show only what's not yet cleared).
- Mark transactions cleared/reconciled: each transaction row has a toggle that flips the (now
  surfaced) `Cleared` flag, with the status shown in the row meta â€” useful for reconciling against a
  statement.
- Edit tasks inline: each to-do row has an Edit button to change the title, priority, due date, and
  notes. Every entity â€” including to-do â€” now supports inline edit.
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
  updating ownership and scope â€” so reassigning no longer requires deleting and recreating.
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
  new pure `goals.MonthlyNeeded` â€” remaining Ã· whole months left, rounded up). Shown only for
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
  amount) are skipped, and the result reports how many duplicates were left out â€” so re-reading the
  same receipt won't double-enter transactions. Backed by `extract.Row.Signature`/`FilterNew` (tested).
- Edit rows in the document review list before importing: each extracted transaction has an Edit
  button to fix its date, description, amount, or category (e.g. correct a misread) prior to import.
- Remove rows from the document review list before importing: each extracted transaction has a âœ• to
  drop a misread, so only the rows you keep are imported.
- Document image import on the Documents screen: choose a receipt or statement image, "Read with
  AI" sends it to the OpenAI vision model (bring-your-own-key, client-side), and the extracted
  transactions appear in a review list â€” pick an account and import them through the validated path
  (categories matched by name, dates falling back to today). Ties together `ai.BuildVisionRequest`,
  `ai.SendVisionChat`, and `extract.ParseRows`. The CSV paste-import remains.
- Extraction parser (`internal/extract`): `ParseRows` turns an AI vision reply into reviewable
  `Row{Date, Description, Amount, Category}` values, tolerant of a bare array or an object wrapper
  (transactions/rows/items/data), numeric or string amounts, varied field names (merchant/payee/â€¦),
  and a Markdown code fence; empties are skipped. Pure, table-tested. Bridges vision output to the
  import flow.
- Vision chat transport (`internal/ai`): `SendVisionChat` posts a multimodal request (system prompt
  + user text + one image) to OpenAI client-side with the user's key, same async one-callback
  contract as `SendChat`. The fetch promise chain is now shared via an internal `postCompletions`.
- Vision request codec (`internal/ai`): `BuildVisionRequest` marshals a multimodal OpenAI chat
  request â€” a system prompt plus a user message carrying text and an image (data/URL) part â€” for
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
  only â€” a duplicate drops any transfer link, so it becomes a standalone entry rather than a broken
  transfer leg.
- Light theme: a `[data-theme="light"]` stylesheet that overrides the legacy palette variables, the
  shell's Tailwind utility colors (base/tile/hover/fg/dim/faint/line, the active-nav surface), and
  the widgets' hardcoded surface hexes (bento tiles, segmented/stepper pills, flip panel, settings
  inputs, switches, scrollbars). Choosing Light (or System on a light OS) in Settings now actually
  lightens the whole app, while the user accent stays applied on top. Completes the theme preference.
- Appearance preferences now apply to the page: `uistate.ApplyPrefs` writes `data-theme`
  (resolving "system" to the OS color scheme), `data-density`, and the `--accent` CSS variable onto
  the document root â€” applied on boot (before first paint) and on every change. The accent color
  retints buttons, bars, focus rings, and active states immediately; a new `[data-density="compact"]`
  stylesheet rule tightens cards, rows, and fields. (A full light-theme skin is still to come; the
  `data-theme` attribute is in place for it to hook.)

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
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
  choices survive reloads â€” the same durable channel the dashboard layout uses (the dataset is
  re-seeded each boot). Loads are always normalized.
- Display-preferences engine (`internal/prefs`): a pure `Prefs` type (week start + date style) with
  `FormatDate` (ISO/US/EU/long), `WeekStartWeekday`, `WeekStartOf` (start-of-week honoring the
  configured first day), and `Normalize` (fills blank/unknown fields with defaults for forward
  compatibility). Table-tested. Foundation for reload-persistent user preferences.
- Custom fields on the Goals, Budgets, and Members forms â€” completing the rollout across all five
  entity types. Each add-form renders its registered custom fields via `CustomFieldInput`, types the
  values into the entity's `custom{}` map on save, and validates them through the matching appstate
  write path (`PutGoal`/`PutBudget`/`PutMember` now call `validateCustom`). Closes Â§1.16 form
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
  entity's `custom{}` value map against its definitions â€” flagging missing required fields, type
  mismatches, invalid dates, and out-of-list select values in plain English, while ignoring unknown
  keys so old data stays forward-compatible. Pure, table-tested. (Foundation for SPEC Â§1.16.)
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
  arithmetic (`Add`/`Sub`/`Neg`/`Abs`/`Cmp`/`Sum`) and table-driven tests (backlog Â§1.1).
- `internal/money`: `FormatMinor`/`Money.Format` (plain decimal rendering) and `ParseMinor` (strict
  decimal â†’ minor units with validation), round-trip tested â€” the basis for clean CSV and inputs.
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
  excluded), `NetWorth` (assets âˆ’ liabilities), and per-owner net rollups â€” all multi-currency via
  base conversion + tests.
- `internal/budgeting`: scope-aware `Spent` (individual vs group), `Evaluate`/`EvaluateAll` with
  remaining, percent, and ok/near/over `State` thresholds â€” multi-currency + tests.
- `internal/goals`: goal `Remaining`, `Percent` (clamped), `IsComplete`, and `Project` (read-only
  completion estimate from an assumed monthly contribution) via `Evaluate` + tests.
- `internal/freshness`: per-type staleness `Windows` (defaults + `Merge` overrides), `IsStale`,
  `DaysSinceUpdate`, and `StaleAccounts`; archived/exempt/untracked accounts never go stale + tests.
- `internal/validate`: per-entity validators returning all `Issues` at once (required fields, valid
  enums, positive amounts, currency consistency, class/type match, score/day ranges, related refs) + tests.
  Completes the Phase 1 pure-logic services layer.
- `internal/store`: pure `Dataset` aggregate + `Settings`, with schema-versioned JSON `Export`/
  `Import` (migration; rejects newer schema) and a lossless round-trip test. Storage-backend-agnostic
  â€” also the sync/transfer payload.
- `internal/store`: in-memory **SQLite** store backed by the pure-Go (no-cgo) `ncruces/go-sqlite3`
  driver, with `Load`/`Snapshot` clean dataset ingress/egress + round-trip tests. Verified to build
  for `js/wasm` (browser) and run natively.
- `internal/store`: per-entity CRUD (Put/Get/Delete/List for members, accounts, categories,
  transactions, budgets, goals, tasks) and query helpers (transactions by account/category/member/
  date-range via SQLite `json_extract`; tasks by status) + tests.
- `internal/store`: `TransactionsToCSV`/`TransactionsFromCSV` â€” human-readable CSV with decimal
  amounts, header-name column matching (order/extra-column tolerant), generated ids for id-less rows,
  and per-line error reporting; lossless round-trip tested.
- `internal/store`: `Get/PutSettings` accessors, atomic `Wipe`, and a valid `SampleDataset` starter
  seed (validated in tests). Completes the Phase 1 persistence layer.
- `internal/logging`: `log/slog`-based `Handler` writing human-readable lines to any `io.Writer`
  plus a bounded, concurrency-safe `Ring` buffer for an in-app log viewer; supports level filtering
  and `With`/`WithGroup` contextual attrs + tests.
- `internal/appstate`: the UIâ†”persistence/logic seam â€” owns the in-memory store + slog logger, with
  typed read accessors, validated write-through (`Put*`/`Delete*`), JSON export/import, and
  `Init`/`Default`; wired into `app.Run` to seed sample data on boot. Pure Go + native tests.
- Accounts screen: first real, data-backed screen â€” assets/liabilities grouped with live per-account
  balances (`internal/ledger`) and a net-worth/assets/liabilities summary, reading from `appstate`.
  Shared money display helpers (`fmtMoney`, amount classes).
- Dashboard screen: real headline metrics â€” net worth, this-month income/expense (via
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
- To-do screen: tasks with priority badges and due dates â€” add, complete-toggle, delete, sorted
  (open first, then by due date).
- Dashboard design exploration: five HTML/Tailwind candidates in `design/`; **candidate C selected**
  (flat neutral-dark, Fraunces serif headings + accounting figures, bento grid, per-widget
  grip/title/gear header, drag-reorder + edge resize, gearâ†’flip settings, collapsible icon sidebar,
  global-settings flip).
- Granular, reusability-focused component backlog for the candidate-C dashboard UI (`TODOS.md` Â§1.7c),
  every item referencing `design/candidate-c.html`.
- Dashboard design-system foundation in the host page: Fraunces + Inter web fonts, the candidate-C
  Tailwind palette/type config, and the full candidate-C component CSS (bento grid, unified widget
  header, drag/resize handles, flip-settings panel, dark scroll pane, sidebar collapse, control
  primitives) â€” ported verbatim from `design/candidate-c.html`, ready for the Go component port.
- `internal/money`: `Group` (thousands separators) and `FormatAccounting` â€” accounting-style display
  (`$1,234.56`, negatives in parentheses like `($240.55)`, always `decimals` places, caller-supplied
  symbol) for the candidate-C figure style; table-driven tests. Pure, no currency-registry dependency.
- `internal/ui`: new shared design-system package (Go port of `design/candidate-c.html`) with a
  reusable, props-driven `Icon` primitive â€” the candidate-C stroked SVG icon set (dashboard, accounts,
  transactions, budgets, goals, to-do, settings, page, plus, menu) that inherits color/size from the caller.

- PWA web manifest (`manifest.webmanifest`) + theme-color/apple meta tags, making CashFlux installable
  as a standalone dark-themed app (Phase 3 start; icons and a service worker follow).
- PWA service worker (`sw.js`, registered on load): network-first caching of same-origin GETs (core
  shell pre-cached on install) so the app stays fresh online and loads offline; cross-origin calls
  (e.g. OpenAI) pass through uncached.
- PWA install prompt: an "Install CashFlux" button appears when the browser offers installation
  (`beforeinstallprompt`) and hides after install.

### Changed
- **WONDER (W-15, W-18):** KPI number count-up on change + chart draw-in (line/area path stroke-dashoffset draw-in, area fade-in) — fail-safe to final value, reduced-motion safe. WONDER catalog complete.
- Retargeted the legacy screen palette (the shared CSS variables) to candidate-C values, so the
  non-dashboard screens (Accounts, Transactions, Budgets, Goals, To-do) â€” cards, stats, rows, forms,
  bars â€” match the new flat neutral-dark bento shell, with squared (4px) corners.
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
  (label + prev/next chevrons) control primitives â€” generic and props-driven, each interactive
  child its own component so click hooks stay stable in lists.
- `internal/period`: pure time-resolution model for the dashboard control â€” `Resolution`
  (week/month/quarter) with anchor `Truncate`/`Step`/`Label` and `Range` (from/to anchors â†’ a
  half-open reporting range, clamped). Table-driven tests cover quarter boundaries, week starts,
  cross-year stepping, and range spanning. Pure, native-tested.
- `internal/period`: immutable `Window` value (resolution + from/to anchors + week start) with the
  control's stepping rules â€” `SetResolution` (re-snaps anchors), `StepFrom`/`StepTo` (move one
  anchor, clamping the other so from â‰¤ to), `Range`, and from/to labels. Drops straight into UI
  state; clamp behavior table-driven tested.
- Time-resolution control in the top bar: a Week/Month/Quarter `Segmented` toggle plus From/To
  `StepperPill`s, backed by a shared `internal/uistate` window atom over `period.Window`. The
  dashboard now derives its income/spending period from this control (re-rendering on change) instead
  of a hardcoded current-month range; stat labels are now period-relative ("Income"/"Spending").
- `internal/ui`: reusable `Widget` shell â€” the candidate-C bento cell with the unified header (grip Â·
  centered title Â· gear) and a padded body, props-driven (title, body, grid span, draggable,
  resizable, gear handler) so every dashboard widget is `Widget` + content. Optional edge resize
  handles; gear is its own component for stable hooks in lists.
- `internal/ui`: reusable `FlipPanel` settings overlay â€” the candidate-C dimmed/blurred backdrop with
  a card that lifts and 3D-flips to a settings back face (centered title, close button, scrollable
  body, dark Save/Cancel footer). Generic (title, body, size, Save/close handlers) and reused by both
  per-widget and global settings; the open animation runs once on mount via `UseState`/`UseEffect`.
- `internal/ui`: reusable `Toggle` (pill switch) + `ToggleRow` (labeled settings row), `Swatch`
  (color chip) + `SwatchPicker` (accent row) control primitives â€” the building blocks of the settings
  forms, generic and props-driven, each interactive element its own component.
- Dashboard rebuilt as the candidate-C **bento grid**: a full-width header cell plus four KPI widgets
  (Net worth, Income, Spending, Liabilities) built from the live ledger and shown as accounting
  figures (`$1,234.56` / `($240.55)`, green/red tone). Each KPI is the reusable `Widget` shell +
  content; Income/Spending follow the time-resolution window. New `fmtAccounting`/`figTone` helpers.
  The Net worth tile shows a real month-over-month change (â–²/â–¼ %) via `ledger.NetWorthSeries`; the
  Income/Spending tiles show the period plus the deposit/transaction count for it.
- Recent transactions widget (2Ã—2) on the dashboard: newest activity as a compact table with short
  dates and accounting amounts (green/red), in the reusable `Widget` shell.
- `internal/ui`: reusable `ProgressBar` primitive â€” the candidate-C thin rounded track + colored fill
  (clamped percent, tone class, extra spacing), reused by budgets, goals, and savings-rate widgets.
- Budgets widget (1Ã—2) on the dashboard: current-month spend per budget with an ok/near/over
  `ProgressBar` and percent (green/amber/red), via `internal/budgeting`. Always month-scoped since
  budgets are monthly.
- Goals widget (1Ã—1): the first goal's progress (saved / target + percent and target date) via
  `internal/goals`, in the reusable `Widget` shell with a `ProgressBar`.
- To-do widget (1Ã—1): up to three open tasks, each with a priority-toned dot (high = amber).
- Accounts widget (2Ã—1): a small grid of up to six active account balances (accounting figures,
  negatives toned red) via `ledger.Balance`.
- `internal/chart`: pure SVG path geometry for dashboard sparkline/area charts â€” `Points` (scale a
  series into a wÃ—h box, y-inverted, padded, flat/single series centered), `LinePath`, and
  `AreaPath` (closed to a baseline). Table-driven tested; no rendering dependency.
- `internal/ledger`: `NetWorthSeries` â€” net worth as of each cutoff time (transactions strictly
  before the cutoff counted), in base currency, for the net-worth trend chart. Table-driven tested.
- `internal/payoff` (Phase 2 start): pure debt-payoff projection â€” `Project(balance, aprPercent,
  payment)` simulates monthly APR accrual and a fixed payment, returning months-to-zero, total
  interest, and total paid, with `ok=false` when the payment can't cover the interest. Table-driven
  tested.
- `internal/forecast`: pure balance/net-worth projection over a horizon â€” `Project(start, recurring,
  oneTimes, months)` applies the recurring monthly net plus any one-time events each month and
  returns the end-of-month balance series; `MonthlyNet` sums the recurring flows. Table-driven tested.
- `internal/ai`: OpenAI chat request/response shapes + a pure codec â€” `BuildRequest` marshals a
  chat-completions body; `ParseResponse` extracts the assistant content and surfaces API errors /
  empty responses; `ParseUsage` reads token counts. Round-trip tested (no network; the fetch
  transport is a separate js layer).
- `internal/ai`: browser `fetch` transport (`SendChat`) â€” posts a chat request with the user's key
  asynchronously and calls back with the content or a plain-English error; the only network spot.
- `internal/rules`: pure auto-categorization engine â€” `Rule{Match, SetCategoryID, SetTags}` with
  case-insensitive substring matching over payee+description, first-match-wins `FirstMatch`,
  `Category`, and `Tags`. Empty matches never fire. Table-driven tested.
- Insights screen (replacing the stub): an **"Explain my month"** AI narrative generated client-side
  from your live figures via OpenAI with your own key; prompts to add a key in Settings when absent,
  with loading and error states. Plus a **natural-language "Ask about your money"** box that answers
  questions using your figures as context.
- Planning screen (replacing the stub): a **debt-payoff calculator** â€” enter balance, APR, and
  monthly payment to see months-to-zero, total interest, and total paid, updating live via the
  `internal/payoff` engine, with a friendly message when the payment can't cover the interest, and an
  optional **extra-payment** input that shows how many months sooner it clears and how much interest
  it saves. Plus a **12-month net-worth projection** chart (current net worth + this month's net cash
  flow, via `internal/forecast` + the area chart) with a what-if "trim monthly spending byâ€¦" input
  that re-projects and reports the improved 12-month figure.
- `internal/allocate`: pure capital-allocation scorer â€” normalizes each candidate on returns,
  stability, liquidity, and debt-reduction, combines by a user `Weights` profile into an explainable
  `Score` + `Breakdown`, and `Rank`s candidates highest-first. Table-driven tested; deterministic.
- Allocate screen (replacing the stub): builds candidates from asset accounts, high-interest
  liabilities, and **unfinished goals**, ranks them by a chosen profile (Balanced / Maximize returns /
  Safety & access / Pay down debt), and shows each suggestion's score bar and per-criterion breakdown.
  An optional **"Explain with AI"** narrative summarizes why the ranking suits the profile (BYO key).
- `internal/formula`: tokenizer for the sandboxed formula language â€” numbers (incl. leading-dot),
  identifiers, double-quoted strings, arithmetic/comparison operators, parens, and commas; errors on
  unterminated strings, stray `=`/`!`, and unexpected characters. Table-driven tested.
- `internal/formula`: recursive-descent `Parse` â†’ AST (NumberLit/StringLit/Ident/Unary/Binary/Call)
  with correct precedence (comparison < additive < multiplicative < unary), left-associativity,
  parens, and function calls. Errors on malformed input. Table-driven tested via a canonical s-expr.
- `internal/formula`: allow-list `Eval` (completes the sandboxed engine) â€” arithmetic, comparisons
  (numeric + string equality), variable resolution from an `Env`, and the functions `sum/avg/min/max/
  count/abs/round/if`. Errors on unknown var/function, arity, division/modulo by zero, and type
  mismatch; no host access. Table-driven tested.
- Customize screen (replacing the stub): a live **formula calculator** â€” write an expression over your
  figures (net worth, assets, liabilities, income, expense, account/transaction/member counts) and
  see the result instantly via the sandboxed engine, with the available variables and their current
  values listed, plus one-click example chips (savings rate, spending ratio, etc.). Variables now
  include budget/goal/task counts alongside the financial figures.
- `internal/ui`: `AreaChart` helper renders a filled gradient sparkline from a value series (feeding
  the pure `chart` geometry into an `<svg>`). Net worth trend widget (1Ã—2) on the dashboard: the
  current figure over a six-month end-of-month area chart via `ledger.NetWorthSeries`.
- Cash flow widget (2Ã—1): income (green, up) vs expense (red, down) bars for the last four months,
  scaled to the largest bar, with the current month's net figure â€” via `ledger.PeriodTotals`.
- Savings rate widget (2Ã—1): the share of the period's income that wasn't spent, as a big figure and
  a `ProgressBar` (toned green/red).
- Spending breakdown widget (2Ã—1): a segmented bar of the period's expenses by category (top three
  plus "Other") with a color-keyed legend; totals converted to base currency.
- Upcoming bills widget (2Ã—1): the next due date and minimum payment for each liability account that
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
- `internal/appstate`: `ExportCSV` (transactions â†’ CSV), `ImportTransactionsCSV` (parse CSV rows â†’
  validated writes, best-effort), `LoadSample` (replace with the sample dataset), and `Wipe` (clear
  all data) â€” the data-action seams; tested natively.
- Documents screen (replacing the stub): paste a CSV of transactions and import them (no AI needed) â€”
  header-name column matching, decimal amounts, negatives for expenses; reports how many imported.
- Global settings Data actions wired: Export CSV (download), Import (file picker â†’ replace dataset),
  Load sample, and Wipe (with a confirm dialog). A shared `data:revision` atom is bumped on bulk
  changes so the dashboard re-renders; added `pickFile`/`confirmAction` browser helpers.
- `internal/dashlayout`: pure bento layout model â€” `Placement` (column/row + spans with CSS grid
  string helpers), `Layout` with the candidate-C `Default` arrangement, immutable `Swap` (exchange
  two widgets' cells) and `Resize` (clamped spans). Table-driven tested; underpins drag-reorder/resize.
- The `Widget` shell now sources its grid placement from a shared `dashboard:layout` atom (falling
  back to caller defaults), so reorder/resize changes flow to every widget via state.
- Drag-to-reorder: dragging one bento widget onto another swaps their grid cells (`dashlayout.Swap`
  via a shared drag-source atom; `dragover` allows the drop with `Prevent`). The dragged widget dims
  (`.drag`) and the source clears on drag-end.
- Resize handles: a widget's right/bottom edge handles now cycle its column/row span
  (`dashlayout.Resize`, clamped to the 4Ã—3 grid bounds) and re-place it live. Every dashboard widget
  is now both draggable and resizable.
- Bento layout persistence: the arrangement is saved to `localStorage` after every reorder/resize and
  reseeds the layout atom on load, so a customized dashboard survives reloads (falls back to the
  default arrangement when absent or invalid).
- Reset layout action in the dashboard header restores the default bento arrangement and clears the
  saved layout.
- Transactions: account-to-account transfers â€” a "Transfer" kind swaps the category picker for a
  "To account" picker and creates paired entries (debit + credit, each with `TransferAccountID`) that
  move both balances and are excluded from income/expense. Same-currency only for now; rows labelled
  "Transfer". Deleting either leg removes the reciprocal so balances stay consistent.
- Transactions: a filter bar (description search + account + category + member pickers + a From/To
  date range, with Clear) narrows the ledger list, with a distinct "No matching transactions" state.
- Transactions: a comma-separated tags field on income/expense entries; tags show on the row
  (`#tag`) and the search box matches tags as well as descriptions.
- Transactions: a sort selector (newest first / largest amount / payee Aâ€“Z).
- Transactions: auto-suggests a category as you type the description (matching against category names
  via `internal/rules`), without overriding a category you've already chosen.
- Transactions: a "Repeat last" button pre-fills the form from the most recent transaction (kind,
  amount, account, category, transfer destination).
- Goals: a "Contribute" action per goal adds an entered amount to its saved total (advancing the
  progress bar) via a quick prompt. The list now sorts incomplete goals first, then alphabetically.
- Top bar: the "+ Add" button now navigates to the Transactions screen (was inert).
- Budgets: a month stepper (â€¹ month â€º) lets you view budget spend for any month, not just the current
  one. A health line summarizes how many budgets are over or near their limit.
- To-do: an optional notes field on tasks, shown in the task row.
- To-do: a "Hide done" / "Show all" toggle to filter completed tasks, with an "All done ðŸŽ‰" state.
- Accounts: archive/restore an account from its row â€” archived accounts move to a separate "Archived"
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
  (onboarding) â€” seeds the store via `appstate.LoadSample`.
- Accounts: a "Stale" badge on accounts whose balance is overdue for a refresh (via
  `freshness.IsStale`), complementing the dashboard nudge and the per-row "Mark updated" action.
- Accounts: liability rows with a credit limit show their credit utilization ("N% of limit used").
- Accounts: the add form reveals a **liability sub-form** (credit limit, interest APR, minimum
  payment, due day, lender) when a liability type is selected â€” feeding the Upcoming-bills widget and
  credit-utilization display, which previously had no data entry path.
- Accounts: for asset types, the add form reveals **allocation attributes** (expected return APR,
  liquidity, stability) â€” giving the Allocate engine real per-account scores instead of zeros.
- Persistence switched from IndexedDB to pure-Go in-memory SQLite (`ncruces/go-sqlite3`, no cgo, no
  dependency on browser web storage); the JSON `Dataset` remains the portable import/export and sync
  payload. (Confirmed pure-Go SQLite compiles for `js/wasm` and runs in the browser.)
- Expanded `TODOS.md` into a granular, per-entity/service/screen backlog covering the full spec.
- Serve web assets from `web/` (clean project root); restyled host page with a dark theme.
- Require bottom-up SDLC build order in `CLAUDE.md` (data model â†’ services/logic with tests â†’
  persistence â†’ state â†’ UI last).

# CashFlux — Product Spec (v1)

> A nextgen personal & household budgeting tool: **local-first, multi-account**, **household/
> group-aware**, with **client-side OpenAI** for document parsing, insights, and forecasting.
> Built in Go compiled to WebAssembly on the GoWebComponents framework. Optional Go sync
> server in a later phase.

Status: **LOCKED for Phase 1** (decisions agreed). Phase 2+ scoped but built later.

---

## Locked decisions (2026-06-15)

1. **AI provider & hosting:** **OpenAI, called client-side** (this is a SPA). User supplies
   their own OpenAI API key in Settings; the wasm client calls OpenAI directly — no backend.
   Caveat shown in UI: key lives in the browser (stored only in local IndexedDB on-device) and
   is subject to CORS/cost on the user's own account.
2. **Local store:** IndexedDB now (via the `interop` package).
3. **Currency:** Multi-currency. Each account has its own currency; aggregate/net-worth views
   convert to a chosen **base display currency** via a **manually editable rate table** in
   Settings (no live FX feed in v1).
4. **Users/sharing:** **Single root user** (no auth/login). One local dataset per device, but it
   is **household/group-aware**: multiple **members**, each with **individual pools**, plus
   **shared group accounts and group budgets**.
5. **Build order:** Full local core (all Phase 1 screens) first, then Phase 2 (AI/forecast/docs).

---

## 1. Vision & differentiators

Standard budgeting apps just record one person's past. CashFlux is built around four pillars:

1. **Household/group budgeting** — individual pools per member + shared group accounts & budgets.
2. **Everything you owe and own** — full asset *and* liability accounts: debit, credit, loans,
   informal/personal debts ("loan sharks"), mortgages, investments.
3. **AI (OpenAI, client-side)** — parse statements/receipts into transactions, auto-categorize,
   analyze spend, answer natural-language questions, and forecast.
4. **Stays useful** — friendly **freshness nudges** that surface data likely to be stale
   (e.g. debt balances drift with interest/payments) and prompt a quick update.
5. **Capital-allocation suggestions** — given money to deploy, recommend the best destinations
   (savings, investments, high-interest debt payoff, goals) ranked by **user-defined weighted
   criteria** (stability, returns, ease of withdrawal, debt reduction, …), with clear rationale.

CashFlux is a small **suite over one shared dataset**: a **Budgeting** tool, a **Planning** tool,
and a budgeting **To-do list** — built on a **well-defined core schema** that users can **extend
with custom fields** and **user-defined calculations via a formula builder**, without forking it.

## 2. Data scope

- **Members**: people in the household/group (labels/owners within the one local dataset).
- **Multi-account, assets + liabilities**, each owned by a member (**individual pool**) or by
  the group (**shared**), each with its own currency.
- **Transfers** between accounts (incl. paying down a liability), not counted as income/expense.
- **Budgets**: individual (per member) and group (shared) limits.
- **Local-first**: all data on-device; no login. Optional server sync later (Phase 3).

## 3. Data model

```
Member   { id, name, color, isDefault }

Account  { id, name, ownerId(memberId | "group"), scope(individual|shared),
           class(asset|liability),
           type(checking|debit|savings|cash|credit_card|line_of_credit|
                loan|personal_loan|mortgage|investment|other),
           currency, openingBalance,
           balanceAsOf,                 // freshness: when balance was last confirmed
           // liability-only:
           creditLimit?, interestRateAPR?, minPayment?, dueDayOfMonth?, lender?,
           // allocation-engine attributes (asset side):
           expectedReturnAPR?, liquidityScore(0-100)?, stabilityScore(0-100)?, lockUntil?,
           archived }

Category    { id, name, kind(income|expense), color, parentId? }

Transaction { id, accountId, date, payee, desc, categoryId, amount(signed),
              transferAccountId?, cleared, tags[], memberId?, sourceDocId? }

Budget   { id, name, scope(individual|group), ownerId(memberId | "group"),
           categoryId, period(monthly), limit }

Goal     { id, name, scope(individual|group), ownerId, targetAmount, currentAmount,
           targetDate, accountId? }

Rule     { id, match(payee/desc contains…), setCategoryId, setTags[] }   // automation

Document { id, filename, kind(statement|receipt|other), uploadedAt, accountId?,
           memberId?, status(parsing|review|imported|discarded), extracted[Transaction] }

Settings { baseCurrency, fxRates{code->rate}, openAiKey?, openAiModel, freshnessOverrides{} }

// --- Extensibility (well-defined core + user-configurable additions) ---
CustomFieldDef { id, entityType(account|transaction|budget|goal|member|task|...),
                 key, label, type(text|number|date|bool|select|money), options[],
                 required, defaultValue? }
// Every core entity carries an open `custom { key -> value }` map validated against the
// CustomFieldDefs for its entityType. Core fields stay strongly typed; custom fields are additive.

Formula { id, name, target(dashboard|account|transaction|budget|member|group),
          expr, resultType(number|money|percent|bool|text), format, enabled }
// User-defined calculation. `expr` references core fields, custom fields, and aggregates
// (e.g. sum/avg/count over filtered transactions). Evaluated by a sandboxed expression engine
// — no arbitrary code, a fixed allow-list of functions/operators.

AllocationProfile { id, name, weights{stability, returns, liquidity, debtReduction, goalProgress, ...},
                    constraints{minEmergencyBuffer?, maxPerDestination?, excludeAccountIds[]},
                    customCriteria[formulaId] }
// Drives the capital-allocation suggestion engine: user-defined, weighted criteria
// (stability, max returns, ease of withdrawal, debt reduction, …) plus optional custom
// criteria backed by formula-builder expressions.

Plan { id, name, horizonMonths, baseScenario, assumptions[PlanItem] }
PlanItem { id, kind(recurring_income|recurring_expense|one_off|extra_debt_payment|rate_change),
           label, amount, currency, startDate, endDate?, cadence(monthly|weekly|yearly|once),
           accountId?, categoryId? }
// Planning tool: scenario-based projections layered on actuals; feeds the Forecast screen.

Recurring { id, kind(income|expense|transfer), label, amount, currency, cadence, nextDate,
            accountId, categoryId?, autopost(bool) }
// Known repeating items — feed freshness, budgets, and forecast.

Task { id, title, notes?, due?, status(open|done), priority(low|med|high),
       relatedType(account|budget|goal|transaction|document|none), relatedId?,
       memberId?, source(manual|ai|nudge) }
// Budgeting-related to-do list. Items can be created manually, from a freshness nudge,
// or suggested by AI (e.g. "review streaming subscriptions", "pay car loan by the 15th").
```

Derived (computed, not stored): per-account & per-member balances, group rollups, net worth
(assets − liabilities), income/expense totals, budget spent/remaining, goal progress, debt
payoff schedules, forecast series, **staleness** (now − balanceAsOf vs the type's window).

## 4. Freshness & friendly nudges

Each account tracks `balanceAsOf`. A per-type **freshness window** decides when data is "likely
stale" and the app gently nudges the user to confirm/update — never nagging, always dismissible.

| Account type                         | Goes stale? | Default window |
|--------------------------------------|-------------|----------------|
| credit_card, line_of_credit, loan, personal_loan, mortgage | yes (balances drift w/ interest & payments) | ~7–14 days |
| checking, debit                      | yes (active spending)                          | ~30 days |
| savings, investment                  | slowly                                         | ~45–60 days |
| cash                                 | manual only                                    | ~30 days |
| Recurring fixed bills (rent, subs)   | **no** — amount is known/stable                | n/a |

- Windows are defaults, **editable in Settings** (`freshnessOverrides`).
- **Dashboard nudge**: a friendly card — e.g. *"3 balances could use a refresh — your car loan
  was last updated 41 days ago."* with one-tap "Update balance".
- Updating a balance writes a balance-adjustment transaction (or sets balanceAsOf) and clears
  the nudge.

## 5. Screens / information architecture

- **Dashboard** — net worth (assets − liabilities), per-member + group rollups, this-month
  income/expense, budget health, **freshness nudges**, recent activity, top AI insight, next goal.
- **Household** — manage members; assign account/budget ownership (individual vs group).
- **Accounts** — assets & liabilities; balances + `balanceAsOf`; add/edit/archive; quick "update
  balance"; per-account ledger. Liability cards show APR, min payment, due date, payoff estimate.
- **Transactions** — global ledger: add/edit/delete, transfers, filters (member/account/category/
  date/text), AI category suggestion on entry.
- **Budgets** — individual (per member) and group budgets vs spent, progress bars, alerts.
- **Goals** — individual & group savings goals with progress and projected completion.
- **Documents (AI)** — upload statements/receipts; AI extracts transactions → review → import.
- **Forecast (AI-assisted)** — projected balance/net-worth curve N months out; what-if (add
  recurring item, change spend, extra debt payment) incl. debt payoff timelines.
- **Insights (AI)** — generated analysis & advice; on-demand "explain my month"; NL query over data.
- **Planning** — the planning tool: build scenarios from `Plan`/`PlanItem`/`Recurring`, compare
  against actuals, and push a chosen scenario into the Forecast.
- **Allocate** — capital-allocation engine: enter an amount, pick an `AllocationProfile`, and get
  a ranked list of destinations with scores and rationale; optional AI narrative explanation.
- **To-do** — budgeting-related task list: open/done, due dates, priority, linked to an account/
  budget/goal; items can originate from freshness nudges or AI suggestions.
- **Customize** — manage **custom fields** per entity and the **formula builder** for user-defined
  calculations (live preview + validation). Surfaces results on the dashboard/relevant entities.
- **Settings** — members defaults, categories, automation rules, base currency + FX rate table,
  freshness windows, OpenAI key + model, data export/import.

## 6. AI integration (OpenAI, client-side)

All AI runs in the browser against the user's own OpenAI key (from Settings). No backend.

- **Document parsing** — statements/receipts (CSV parsed locally in Go; PDF/images sent to an
  OpenAI vision-capable model). Returns **structured transactions** (date, payee, amount,
  suggested category) for the **Documents → review → import** flow; powers "monthly spend".
- **Auto-categorization** — suggest categories on entry and for imported rows; can propose Rules.
- **Spend analysis & insights** — plain-language "where did my money go / what to change".
- **Natural-language query** — "how much did I spend on food in May across the household?"
- **Forecasting/what-if** — narrative + numeric projections, including debt payoff scenarios.

Implementation notes: use OpenAI **structured outputs (JSON schema)** for parsing/categorization
so results map straight onto Go structs. Model is **configurable in Settings**; default to a
current OpenAI model (confirmed at implementation time). Show token/cost and a hard
"AI off by default until a key is provided" state.

### 6.1 Capital-allocation suggestion engine

Deterministic, explainable, **rules-first** (AI optional):

- **Inputs**: amount to deploy + chosen `AllocationProfile` (weights + constraints). Candidate
  destinations = eligible asset accounts, goals, and high-interest liabilities (payoff treated as
  a guaranteed "return" = its APR).
- **Scoring**: each destination scored per criterion — *returns* (expectedReturnAPR / debt APR),
  *stability* (stabilityScore), *liquidity / ease of withdrawal* (liquidityScore, lockUntil),
  *debt reduction*, *goal progress* — then combined by the profile's weights. Constraints
  (emergency buffer, max-per-destination, exclusions) filter/clamp results.
- **Output**: ranked suggestions with per-criterion breakdown and a one-line rationale; optional
  AI narrative ("why this split") when a key is configured. Custom criteria come from
  formula-builder expressions referenced in the profile. **No black-box math** — the score
  breakdown is always visible.

## 7. Extensibility — custom fields & formula builder

The core schema (§3) stays strongly typed and stable; flexibility is **additive**, never by
mutating core types:

- **Custom fields** — users define `CustomFieldDef`s per entity type (text/number/date/bool/
  select/money). Each entity stores a validated `custom{key->value}` map. Forms render core fields
  + any custom fields automatically; export/import round-trips them.
- **Formula builder** — users compose `Formula`s ("Debt-to-income", "Savings rate",
  "Group rent share") from a guided builder. Expressions reference core fields, custom fields, and
  aggregates over filtered transactions.
  - Evaluated by a **sandboxed expression engine**: a fixed allow-list of operators and functions
    (`sum`, `avg`, `min`, `max`, `count`, `if`, arithmetic, comparisons) over named variables —
    **no arbitrary code execution**. Results are typed (number/money/percent/bool/text) and
    formatted, then shown on the dashboard or the relevant entity.

## 8. Architecture on GoWebComponents

- **Client**: Go → wasm SPA. `ui` hooks for local UI, `state` atoms for app-wide data,
  `router` (history) for pages, `html/shorthand` (+ control-flow funcs) for views.
- **Persistence**: IndexedDB via the `interop` package; a small Go `store` layer wrapping object
  stores for each entity. PWA/offline later.
- **Logging**: a `log/slog`-based logging package with a custom `slog.Handler` that bridges to the
  browser console and an in-app ring buffer (viewable in a debug panel). Levelled, structured,
  context-aware; no `fmt.Println` debugging.
- **Import/export**: first-class. Full dataset export/import as JSON (schema-versioned, includes
  custom fields/formulas/settings); CSV import/export for transactions; (Phase 2) AI document
  import. All round-trips are tested.
- **AI client**: a Go `ai` package issuing `fetch` calls to OpenAI with the user key + JSON-schema
  structured outputs.
- **Sync server (Phase 3)**: Go HTTP service; pull/push deltas of the one household dataset;
  shares the same Go structs as the client.
- **Build/run**: `gwc dev` (live reload) for the inner loop; `gwc release` for compressed wasm.

## 9. Phasing

- **Phase 0 — Skeleton:** repo + toolchain (Go/git/gh installed ✓), routed shell, IndexedDB store
  scaffold with the core schema + custom-field plumbing. *(Early draft code exists; being reshaped.)*
- **Phase 1 — Local household core:** Members, Accounts (assets+liabilities), Categories,
  Transactions (+transfers), individual + group Budgets, Goals, **freshness nudges**, **To-do
  list**, **custom fields**, multi-currency + FX table, export/import. **No AI.**
- **Phase 2 — Intelligence & power tools (OpenAI, client-side):** Documents/parsing, Insights,
  auto-categorization + Rules, **Planning** + Forecast, **formula builder**, **capital-allocation
  engine** (+ allocation profiles), NL query. *(Allocation account attributes — return/liquidity/
  stability — are captured in Phase 1 so the engine has data to work with.)*
- **Phase 3 — Sync & PWA:** Go sync server, multi-device sync, offline.

## 10. Still-open (small) decisions

1. **Debt payoff math** in v1: simple read-only estimate (balance × APR, min/extra payment) in
   Phase 1, or defer all payoff math to the Phase 2 Planning/Forecast tool?
2. **FX rates**: manual table confirmed — keep purely manual, or allow one-tap paste of rates the
   user supplies?
3. **Custom fields & To-do in Phase 1, formula builder + Planning in Phase 2** — confirm this split
   (formula builder leans on aggregates the AI/forecast work also needs), or pull either forward?

## 11. Engineering principles & non-functional requirements

This is an owned platform; the quality bar is high and applies from Phase 1.

- **Pure, idiomatic, beautiful Go.** Proper Go patterns, small composable packages, clear names,
  documented exported symbols, errors wrapped with context. No hacks or non-Go shortcuts.
- **Clean architecture.** Domain types and pure business logic (balances, FX, scoring, formula
  evaluation, freshness) live in plain, platform-independent Go packages with **no `syscall/js`**,
  so they are unit-testable on native Go. The wasm/UI layer is a thin shell over them.
- **Thorough testing.** Table-driven unit tests for all logic packages (money/FX, freshness,
  allocation scoring, formula engine, import/export round-trips); wasm/browser tests for UI flows.
  Tests run via `gwc test` lanes and `go test ./...`.
- **Structured logging (`log/slog`).** A logging package with a custom handler → browser console +
  in-app ring buffer; levelled and contextual. No ad-hoc print debugging.
- **Readable, modern UI in plain English.** Exceptionally legible typography and spacing, generous
  contrast, clear hierarchy; all copy is plain, friendly, jargon-free English. Accessibility via
  the framework's a11y primitives.
- **Determinism & explainability.** User-facing computations (allocations, forecasts, formulas)
  always expose their breakdown — no black boxes.

## 12. Configuration & modalities

CashFlux is **heavily configurable** so one app fits many ways of budgeting and many households:

- **Layered config**: sensible defaults → per-household settings → per-member preferences →
  per-screen options, all in the local store and export/importable.
- **Modalities to support via config** (not separate apps): budgeting methodology (envelope,
  zero-based, simple tracking), personal vs household vs group, which modules/screens are shown,
  number/date/currency formatting, freshness windows, category schemes, default AllocationProfile,
  theme/density, AI on/off and model, units, week-start, fiscal-month start, etc.
- **Custom fields + formulas (§7)** are the escape hatch when built-in config isn't enough.
- Every option has a clear plain-English label and a sane default; nothing requires editing code.

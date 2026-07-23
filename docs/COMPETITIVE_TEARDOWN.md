# CashFlux — Full Competitive Teardown (2026-07-23)

> **What this is.** A feature-by-feature comparison of CashFlux against every meaningful commercial
> and free/open-source competitor, per Cam's directive: for each CashFlux feature area, what the
> comps do better, every missing feature, every missing *intra-feature connection*, UI mechanics
> down to interaction cost (click/tap counts), and each comp's announced/planned features. This is
> a research document — it feeds the backlog (WF/PS/FB/RH series in `TODOS.md`), it does not
> replace it. Cross-references to tickets appear as `→ WF1` etc.

## 0. Method & honesty notes

- **Hands-on:** CashFlux itself (source + FEATURE_MAP + live app on :8080 with sample data) and
  **PiggySize** (full Playwright walkthrough of all 15 live-demo pages, 2026-07-23).
- **Doc-derived:** YNAB, Monarch, Copilot, Simplifi, Rocket Money, Lunch Money, PocketSmith,
  Empower, Boldin, ProjectionLab, Goodbudget, EveryDollar, PocketGuard — from official help-center
  articles, vendor feature pages, release notes/changelogs, and detailed 2026 reviews (Forbes,
  NerdWallet, Engadget, CNBC, The College Investor, Money with Katie, Rob Berger, et al.).
  Help-center articles describe exact UI flows, so click paths quoted for these are precise to the
  documented flow but were not physically executed here.
- **Docs + source:** Actual Budget and Firefly III (public docs, release notes, repos).
- **Interaction-cost convention.** "Clicks" counts *deliberate activations* (click/tap/keypress
  that commits an action) from the app's default landing screen, excluding typing. Ranges mean the
  flow varies by state. CashFlux counts were derived from the live route map and screen structure
  (FEATURE_MAP §1–3) and the running app.
- **Bias control.** Each area lists what comps do *better* only. CashFlux advantages are stated
  once in §2 and then deliberately not repeated — this document's job is gap-finding, not
  reassurance. Where a comp's "better" is really "different trade-off," that's said explicitly.

## 1. Comp roster & one-line identity

| # | Comp | Tier | Identity in one line | Price (mid-2026) |
|---|---|---|---|---|
| 1 | **YNAB** | Commercial, budgeting-first | Zero-based envelope method as a lifestyle; targets; loan payoff simulator | $109/yr · $14.99/mo |
| 2 | **Monarch** (ex-Monarch Money) | Commercial, all-in-one | The Mint successor: aggregation + flex budgets + goals v2 + forecasting + AI; Core/Plus tiers | $99.99/yr · $14.99/mo; Plus higher |
| 3 | **Copilot Money** | Commercial, design-first (Apple + web) | ML categorization ("Copilot Intelligence"), Tinder-style review, goal-funded spending | ~$95/yr · $13/mo |
| 4 | **Quicken Simplifi** | Commercial, value/automation | Auto-generated Spending Plan, watchlists, 12-month projected cash flow, refund tracker | $3.99–6.99/mo |
| 5 | **Rocket Money** | Commercial, subscription-killer | Recurring detection + human concierge cancellation + bill negotiation + smart savings | Free tier; $7–14/mo premium |
| 6 | **Lunch Money** | Commercial, indie/power-user | Web-first multicurrency ledger; the best rules engine; crypto; developer API | ~$100/yr |
| 7 | **PocketSmith** | Commercial, forecasting-first | Calendar-based budgeting with daily projected balances 10–60 years out; scenarios | tiered, to ~$26.66/mo |
| 8 | **Empower Personal Dashboard** | Commercial, free-as-funnel | Net worth + retirement Monte Carlo + fee analyzer + recession simulator, free | Free (advisory upsell) |
| 9 | **PiggySize** | Commercial, planner-no-ledger | Balances-and-obligations planner; retirement Monte Carlo; family logins; $9 undercut | Free tier; $9/mo Pro |
| 10 | **Boldin** (ex-NewRetirement) | Commercial, planning-depth | Consumer retirement planning: Roth explorer, IRMAA, guardrails, 10-scenario compare | $129/yr |
| 11 | **ProjectionLab** | Commercial, planning-depth | FIRE/scenario modeling; privacy-respecting; tax analytics; 10k Monte Carlo | $129/yr (lifetime $1,199) |
| 12 | **Actual Budget** | OSS, local-first | The architectural twin: local-first envelope budgeting, rules with regex, e2e-encrypted sync | Free (MIT) |
| 13 | **Firefly III** | OSS, self-hosted | Double-entry power tool: rules, recurrences, piggy banks, webhooks, importer | Free (AGPL) |
| 14 | **Goodbudget / EveryDollar / PocketGuard** | Commercial, simple tier | Envelope-manual / Ramsey ZBB / "In My Pocket" + Pace | free tiers + $10/mo · $79.99/yr · tiered |
| 15 | **Tiller** | Commercial, spreadsheet | Bank feeds into Sheets/Excel; the "full control" audience | $79/yr |

Dead-but-relevant: **Mint** (closed Jan 2024) — the switcher pool everyone above markets to.

## 2. CashFlux's standing advantages (stated once, then assumed)

Local-first with genuinely-local data; no account; strongest privacy claim in the roster
(Actual is the only peer). Full ledger + budgets (3 methodologies) + debt + recurring + reports +
investments + goals in ONE product. Extensibility no competitor has: workflow automations,
formula engine, custom fields, widget builder/custom pages, theme editor. Deterministic,
explainable math as law. Household ownership + split/settle. i18n-zero ratchet, a11y baselines,
6-layer e2e. These are real. Everything below is about what's *missing or worse*.

## 3. CashFlux interaction-cost baseline (for the click-count comparisons)

From `/` (dashboard) with sample data, default layout, desktop:

| Task | Path | Cost |
|---|---|---|
| Add a transaction | Dashboard → **Add transaction** button (hero) → form → save | **1 click + form + 1** |
| Add txn from anywhere | rail Transactions → Add transaction → form → save | 2 + form + 1 (or Ctrl-K palette) |
| Recategorize one txn | Transactions → row inline edit → category select → save | 3–4 |
| Bulk recategorize N txns | Transactions → select N (N clicks) → picker → Apply | N + 2 |
| Create a rule from a txn | Transactions → row ⋯ → create-rule | 2–3 |
| Reconcile an account | Accounts → row ⋯ → Reconcile → enter statement → mark cleared ×k | 3 + k |
| See why health score moved | /health → "How this number is made" disclosure | 2 |
| Set up a recurring bill | /recurring → add-form (label/amount/cadence/account/category/first-due/autopost/autopay) | 2 + form |
| Cancel-intent a subscription | /recurring → Subscriptions tab → row → Cancel | 3 |
| Run a what-if plan | /planning → What-if plans → form → save | 2 + form |
| Extra-payment debt sim | /debt → Strategy → extra-payment input | 2–3 |
| Import a CSV | /transactions (Import drawer) → file → account → map → import | 4–6 |
| Ask the assistant | /assistant → chat input | 2 |
| Change theme accent | /appearance → swatch | 2 |

Weak spots visible already at this altitude: **review of imported transactions is row-by-row**
(no single-keypress confirm queue — → WF1/FB2), and several flows live 2 levels deep behind
tabs-in-hubs where comps put them one level up.

---

# PART I — AREA-BY-AREA TEARDOWN

Each area: **(a)** CashFlux today · **(b)** what each relevant comp does better, with mechanics ·
**(c)** missing features · **(d)** missing intra-feature connections · **(e)** verdict + tickets.

---

## Area 1 — Dashboard & attention

**(a) CashFlux today.** 4-col bento, 21 tiles, resize/hide/reorder, three layout modes
(custom/auto-default/auto-importance), focus selector swapping tile sets, "Good morning" hero
with net worth + sparkline + KPI strip, Needs-attention banner (full-width, first), Monthly recap
strip, per-tile style editor, user-built widgets publishable to the grid.

**(b) What comps do better.**

- **Copilot 📄 — the review queue IS the dashboard.** Copilot's Dashboard tab leads with a
  **"To Review"** section: every new transaction lands there with a suggested type + category
  (after ~30 txns of learning) and is dispatched with a tap ("Mark as Reviewed" — reviewers call
  it Tinder-like). The dashboard's primary interaction is *dispatching work*, not reading tiles.
  Interaction cost per ordinary transaction: **1 tap**. CashFlux's Needs-attention banner names
  problems but its chips deep-link to pages where the user re-locates the item (2–4 more clicks)
  — the queue itself doesn't process anything in place. → WF1, FB2.
- **Monarch 📄 — weekly recap as narrative.** "Weekly Recap" is an AI-written automatic summary
  of the week's finances delivered as a digest. CashFlux's smart-digest tile is a strip, not a
  written recap with narrative continuity (our annual-review prose exists but only in Reports).
  The cadence gap: they have week-granular narrative; we have month/year.
- **Actual 📄 — multiple named dashboards.** Actual (26.x) supports **multiple dashboard pages**,
  each its own set of report widgets (spending, net-worth, cash-flow, custom-report widgets,
  **balance-forecast** and **age-of-money** cards), importable/exportable as dashboard files.
  CashFlux has one dashboard + custom pages — close, but custom pages don't take the *built-in*
  analytics tiles, only KPI/List/Chart/Text/Image/Table widget types; you cannot compose a second
  dashboard out of the real dashboard tiles. Missing connection: `dashlayout` tiles ↔ `/p/:slug`
  pages are two separate widget systems.
- **PiggySize ✋ — teaching microcopy on every tile.** Every KPI card carries a visible
  explainer sublabel ("Spending money — after all bills & taxes"). CashFlux hides explainers in
  tooltips. → PS13.
- **PocketGuard 📄 — one number, defended.** The entire product leads with "In My Pocket" —
  a single defensible discretionary number, plus **"Pace"** (are you spending it too fast for the
  days remaining). CashFlux computes safe-to-spend but presents it as one KPI among six; there is
  no pace-against-days-remaining framing on the tile itself (budgets have pace; the dashboard
  number doesn't).

**(c) Missing features (dashboard).** Severity-ranked actionable queue with in-place dispatch
(WF1); weekly narrative recap; second/multiple dashboards composed from built-in tiles;
balance-forecast tile (a 30/60-day projected-balance mini-chart as a *dashboard* widget — the
data exists in /planning); age-of-money metric (YNAB/Actual concept — how old is the money you
spend; a genuinely good freshness-of-buffer signal CashFlux lacks entirely); pace framing on
safe-to-spend.

**(d) Missing intra-feature connections.** Needs-attention chips → open the referenced item in an
actionable overlay instead of navigating away; smart-digest ↔ notifications (currently parallel
feeds — a digest item dismissed is not a notification read); dashboard tile system ↔ custom-page
widget system (unify or bridge); recap strip stats → drill into the exact filtered txn set that
produced each number (some do, not all).

**(e) Verdict.** Structurally ahead (bento + builder is unique), behaviorally behind: comps'
dashboards *process*; ours *presents*. → WF1, FB2, PS13; new items filed in §Master-gaps.

---

## Area 2 — Transactions, review, rules, import

**(a) CashFlux today.** Full ledger table (paginated, sortable, filter toolbar with persisted
filters incl. cleared/custom-field/amount-range), bulk bar (recategorize/clear/export/delete +
bulk undo snapshot), inline edit, split editor, per-row create-rule, receipt attach + vision
extract, transfers, tags, repeat, duplicate detection page + one-click "select duplicates,"
review inbox (250-item count seen in top bar), Rules page: quick-add with live match-count,
drag priority, shadow warnings, coverage summary ("N of M covered"), AI-suggested rules,
Mermaid precedence flowchart. Import: CSV + column-mapping profiles + OFX/statement paste +
AI receipt/statement extraction + draft review with dedupe + import history.

**(b) What comps do better.**

- **Copilot 📄 — learning loop with confidence + second-best suggestion.** After a wrong
  categorization, Copilot **immediately surfaces the second-best category** so correction is
  1 tap, and each correction trains the model; reviewers report manual work "nearly disappears
  after two to three weeks." It auto-recognizes internal transfers so card payments never
  double-count. CashFlux's rules are deterministic keyword rules + AI suggestions — there is no
  per-transaction confidence score, no ranked alternates on the edit form, and no automatic
  transfer pairing (transfers must be entered as transfers or manually matched). Cost comparison
  for "fix one miscategorized txn": Copilot **1–2 taps** from the review card; CashFlux 3–4
  clicks through inline edit.
- **Monarch 📄 — rules that split, and retro-application as a first-class choice.** Monarch
  rules match on **Original statement** (raw bank text) with exactly/contains, and actions
  include **auto-splitting a transaction by percentage or fixed amounts** — with an explicit
  "Apply # changes to existing transactions" checkbox that shows the count before you commit.
  CashFlux rules cannot split, and Apply-to-existing is a page-level button, not a per-rule
  preview-with-count at save time. Monarch also ships **Smart Split** and a mobile fast-review
  flow ("Review faster than ever"). → WF7.
- **Actual 📄 — the reference rules engine for a local-first app.** Conditions on imported-payee
  vs payee (pre-rename vs post-rename — prevents rule conflicts CashFlux can hit when a rename
  changes match text), operators `is / contains / one of / matches(regex)`, **three stages
  (pre/default/post)** with auto-ranking by specificity inside a stage ("is" outranks
  "contains"), actions set category/payee/notes/cleared/account/date/amount with prepend/append
  on notes and an experimental **formula mode** for computed values, **auto-generated rules from
  user behavior** (payee renames prompt "apply this rename in the future?"; most-common-category
  learning maintains rules silently, with per-payee/global opt-out), and the killer UI move: the
  **rule editor doubles as a batch editor** — it live-lists every matching transaction and lets
  you "Apply actions" to selected rows without even saving the rule. CashFlux has live
  match-count but not live match-*list* in the editor, no stages, no regex, no learned
  auto-rules, no rename-tracking. → WF7 (this is the spec to beat).
- **Lunch Money 📄 — multi-condition/multi-action + notify.** Rules take multiple trigger
  conditions AND multiple actions in one rule — including **split the transaction, add tags, and
  send an email notification** — with explicit priority levels. A rule that emails you is an
  automation primitive CashFlux's rules don't have (our Workflows can, but rules and workflows
  are unconnected systems — see (d)).
- **YNAB 📄 — reconciliation as ritual + keyboard entry.** YNAB's reconcile flow is a two-input
  guided ritual ("Is this your current balance?") that locks reconciled txns behind an explicit
  state; its transaction entry is keyboard-complete with payee/category autosuggest memory.
  CashFlux reconcile exists per-account inline but doesn't lock rows (cleared ≠ locked) and no
  keyboard-first entry mode has been verified end-to-end. 
- **Simplifi 📄 — refund tracker.** Mark an expected refund; the app watches for the matching
  credit and nags until it arrives. CashFlux has nothing watching for *expected inbound* money
  tied to a specific past transaction. Reviewers single this out as "uncommon and genuinely
  useful."

**(c) Missing features.** Per-txn categorization confidence + ranked alternates; learned
auto-rules with opt-out (Actual-style); rename-aware payee model (imported-payee vs display-payee
as separate fields — CashFlux stores one description); rule stages/specificity ranking; regex
matching; rule actions: split, tags-add, notify; retro-apply with count preview at rule save;
automatic transfer pairing (detect the two legs cross-account and link — CashFlux ledger has
transfers but imports don't auto-pair); refund/expected-credit tracker; reconciliation lock
state; amount-range + "is one of" operators in rules (filters have them; rules don't).

**(d) Missing intra-feature connections.** **Rules ↔ Workflows**: two automation engines, no
bridge (a rule can't trigger a workflow action; a workflow can't create/modify a rule) — comps
have one engine doing both (Lunch Money) or none; having two unlinked ones is worse than either.
**Duplicates ↔ import**: dedupe preview exists at import, but post-import the duplicates page
doesn't know which import batch introduced a dupe (provenance exists in history; not linked into
the dupe card). **Vision receipts ↔ recurring**: an extracted receipt for a known subscription
merchant doesn't attach to the recurring item's history. **Bulk edit ↔ rule creation**: after
bulk-recategorizing N rows of the same payee, no "make this a rule?" prompt (Actual prompts on
rename; Copilot learns silently; we do neither from bulk actions).

**(e) Verdict.** CashFlux's ledger *feature list* matches or beats everyone; the *learning loop
and review economics* lose to Copilot/Actual/Monarch. The entire area is governed by FB2's
metric: median seconds per reviewed transaction. → WF1, WF7, WF11, FB2.

---

## Area 3 — Accounts, net worth, freshness

**(a) CashFlux today.** Asset/liability accounts with type-contextual fields (APR/limit/min/
due-day/lender; return/liquidity/stability/lock-until), archive, stale badges + Mark-all-updated,
set-balance with delta preview, reconcile-to-statement, per-row transfer, valuation history for
investment types, net-worth strip + trend chart, FX table with missing-rate alerts, per-owner
net worth.

**(b) What comps do better.**

- **Every aggregator (Monarch/Copilot/Simplifi/YNAB/Rocket/Empower) 📄 — zero-maintenance
  balances.** Not a feature CashFlux can copy (by design), but the teardown must be honest about
  the cost: their balance freshness is automatic and continuous; ours is a chore. This is why
  WF4/FB5/FB9 exist — the *management of manualness* is CashFlux's competitive surface, and
  today it's a badge + a button, not a system. PiggySize ✋ handles the same constraint better
  *socially*: "about 15 minutes a month," expected-cadence framing, and the whole product shaped
  around balances-not-transactions so the maintenance ask is small and predictable.
- **Actual 📄 — opt-in sync without credentials-to-us.** SimpleFIN (US/CA, ~$1.5/mo to a
  third party, read-only tokens, ~daily refresh, 90-day backfill) and GoCardless (EU/UK) give a
  local-first app *optional* bank feeds without the vendor ever holding credentials. This is the
  proof-of-concept that "local-first" and "bank sync" are not mutually exclusive — the user
  chooses the bridge. CashFlux Cloud could adopt the same posture (sync adapter as a paid
  convenience, keys held client-side). Nothing in the CashFlux backlog currently contemplates a
  SimpleFIN-style adapter; it is the single most-requested thing local-first users grudgingly
  leave for. (Non-goal per WF-series today — worth re-examining for Cloud.)
- **Monarch 📄 — credit score + equity + manual-asset breadth.** Native credit-score tracking
  (July 2025) with configurable alerts; **equity tracking** (private-company equity as an asset
  class, Dec 2025); browser-extension-assisted amounts (Target/Amazon purchase itemization via
  the Monarch Extension). CashFlux's credit surface is a *proxy* score from utilization — honest,
  but no bureau data and no way to even manually log a real score over time (missing: a
  score-history manual series).
- **Empower 📄 — investment-fee X-ray on accounts.** Fee analyzer multiplies expense ratios by
  holdings to show the fee drag per account. CashFlux holdings have no expense-ratio field.

**(c) Missing features.** Expected-update-cadence per account (→ WF4 refinement, filed);
manual credit-score history series; expense-ratio field + fee rollup on investment accounts;
account-level notes/documents (statements attach to txns, not accounts); optional read-only sync
bridge posture (SimpleFIN-style) as a Cloud decision; per-account "exclude from net worth"
toggle (comps have hide-from-reports at account level; CashFlux archive is all-or-nothing).

**(d) Missing intra-feature connections.** Stale badge ↔ recurring: a bill that autoposted
against a stale account should escalate ("posted against a 40-day-old balance"); valuation
history ↔ net-worth chart (manual asset valuations exist but the NW trend doesn't annotate
valuation-driven jumps vs flow-driven ones — WF5 territory); FX missing-rate alert ↔ the
excluded accounts (alert says N excluded; no one-click "show me the excluded rows").

**(e) Verdict.** Feature-par on manual accounts, behind on making manualness cheap. → WF4, FB5,
FB6, FB9; new: cadence field (filed), sync-bridge posture question (flag for Cloud research).

---

## Area 4 — Budgets

**(a) CashFlux today.** Three methodologies (Simple / Zero-Based / Envelope) with per-budget
method override, weekly/quarterly periods via PeriodRange, rollover, 50/30/20 starter,
pace warnings, cover-from-another + top-up (period boosts), sinking-fund set-aside line,
over-budget banner + status pills, health-ordered rows, per-budget formulas modal, sort picker,
add-modal with owner.

**(b) What comps do better.**

- **YNAB 📄 — targets as a *typed* system.** Target cadences Weekly/Monthly/Yearly/Custom
  (incl. non-repeating "by date"), and two explicit behaviors: **"Set aside another X"**
  (fund the same amount regardless of rollover — for bills and accumulating savings) vs
  **"Refill up to X"** (top-up — for spend-to-a-level categories like gas/dining). The
  distinction is the whole game for envelope users, and YNAB makes the user choose it per
  category with plain-language consequences. CashFlux's rollover toggle approximates refill
  behavior but there is no "set aside another" vs "refill up to" *vocabulary or divergent math*,
  no by-date targets on budgets (goals have dates; budgets don't), no weekly-cadence target on a
  monthly-period budget. → WF17 (this is the exact spec; YNAB named as benchmark there).
- **Monarch 📄 — Flex budgeting: one-number mode.** Three buckets — Fixed / Non-monthly /
  **Flex** — where the user tracks ONE number (total flexible spending) instead of ten volatile
  categories; category-level rollovers behave differently per bucket (fixed/non-monthly rollovers
  hit both category and bucket "remaining"; flex rollovers hit only the category). It's a
  deliberate cognitive-load reducer for people who fail category budgeting. CashFlux has no
  one-number budget mode; Simple mode still renders per-category rows. This is the single most
  copied budgeting innovation of 2025–26.
- **Simplifi 📄 — the auto-generated Spending Plan.** Income − bills/subscriptions = a
  personalized plan generated *for* you that adjusts automatically as you spend — onboarding to
  a working budget in minutes with zero category-by-category setup. CashFlux's 50/30/20 starter
  is one click but still produces category budgets to maintain. The gap is a **maintained-for-you
  plan** vs **starter you then own**.
- **Lunch Money 📄 — budget periods beyond months.** Their 2026 budgeting beta ships custom
  budget periods including **"twice a month,"** zero-based support, **carryover with adjustable/
  resettable starting balance at any time**, and allocation pools. CashFlux has weekly/quarterly
  but not semimonthly (paycheck-aligned), and rollover balances can't be manually adjusted/reset
  without editing history. → WF17.
- **Copilot 📄 — cumulative rollovers with a chosen epoch.** Rollover is enabled per category
  *with a "first month with a rollover"* setting — i.e., an explicit rollover epoch — and
  accumulates signed (over AND under) months. CashFlux rollover exists; signed accumulation and
  a user-chosen epoch are not exposed.

**(c) Missing features.** Typed targets (set-aside vs refill, by-date, weekly-on-monthly);
one-number Flex mode with bucket math; auto-generated maintained spending plan; semimonthly
periods; adjustable/resettable rollover balance; rollover epoch; budget "snooze" (pause a
category without deleting — YNAB/Monarch both effectively support via hiding); per-category
notes on the budget row.

**(d) Missing intra-feature connections.** **Budgets ↔ recurring**: the recurring-in-your-budgets
strip (shipped) shows commitments but budgets don't *pre-reserve* upcoming recurring amounts
inside the period ("$380 of this category's remaining is already spoken for by HOA on the 1st")
— Simplifi's Spending Plan does exactly this subtraction. **Budgets ↔ goals**: sinking funds
bridge them, but goal-funded spending (Copilot's blue-bar mechanic: a goal-linked purchase bumps
the category budget so it doesn't read as overspend) is unbuilt → WF18 (Copilot's exact
mechanics now documented: "Update Budgets on Spend" toggle; blue bar in category view; $500
repair linked to Home Improvement goal = $500 budget bump). **Budgets ↔ allocate**: an
underfunded budget is not an allocate destination today (allocate ranks accounts/debts/goals —
not budget shortfalls) → WF10 lists it; connection restated here because three comps (YNAB via
targets, Monarch via flex, Simplifi via plan) treat "fund the plan" as the same verb.

**(e) Verdict.** Methodological breadth is real but *untyped*; YNAB's target grammar +
Monarch's flex mode are the two things users actually switch for. → WF17 (extend with flex-mode
+ rollover-epoch + semimonthly), WF18, WF10.

---

## Area 5 — Goals & sinking funds

**(a) CashFlux today.** Goals with kinds (checklist/milestone/habit), pace badges (Final
Stretch/Overdue/Due Soon/On Track), monthly-needed, linked account, contribute form with
post-to-ledger toggle, sinking funds (category + monthly set-aside, budgets integration),
achieved archive, smart empty-state, todo↔goal linkage (Task.RelatedGoal).

**(b) What comps do better.**

- **Monarch 📄 — Goals v2 (June 2026, out of beta).** Rebuilt "from the ground up": goals bound
  to *real account balances* (a goal is funded by designated accounts, so progress moves when the
  money actually moves — no manual contribute step), used for both saving targets AND debt-payoff
  goals in one system, with plan-vs-actual pacing. CashFlux goals link *one* account optionally
  and progress is contribution-driven (manual or posted), which desynchronizes from reality when
  money moves outside the contribute flow. Missing: multi-account funding, balance-derived
  progress as the default, debt goals unified into the goal system (CashFlux debt payoff is a
  separate engine with no "goal" wrapper — you cannot put "pay off Rewards Card" next to "Emergency
  fund" in one prioritized list).
- **Copilot 📄 — goal-funded spending** (full mechanics in Area 4d; the other half is goal-side):
  goals have **Active vs Ready-to-Spend states** and optional auto-**reactivation** when a
  completed goal's balance dips below target — i.e., a goal can *be* a sinking fund with a
  lifecycle. CashFlux sinking funds don't have a "ready/spent/refilling" state machine.
- **Simplifi 📄 — goals inside the Spending Plan.** A goal's monthly contribution is a line the
  plan *subtracts* like a bill, making the trade-off visible ("planned spending includes $300 to
  goals"). CashFlux shows monthly-needed on the goal but doesn't subtract it from safe-to-spend
  (the sinking-fund set-aside line on /budgets is display-only relative to STS).
- **PiggySize ✋ — projection framing.** Each goal card shows *projected completion at current
  pace* ("Est. $29,000 by Jan 1, 2027 — FALLS SHORT") — pace converted to a dated verdict.
  CashFlux badges pace but doesn't print the projected end-state number/date on the card.

**(c) Missing features.** Balance-derived goal progress; multi-account goal funding; debt-payoff
goals inside the goals list; goal lifecycle states (active/ready/spent-down/reactivated);
projected-completion verdict line; goal images/emoji (minor but universal in comps).

**(d) Missing intra-feature connections.** Goals ↔ safe-to-spend (subtract committed
contributions); goals ↔ allocate (allocate suggests goal contributions — but confirming one does
not mark the goal's month as funded vs the pace engine); goals ↔ recurring (a scheduled transfer
to the linked account should auto-log as contribution — today the recurring engine and
contribute flow don't touch); goals ↔ workflows (goal-reached trigger exists; goal-behind-pace
trigger does not).

**(e) Verdict.** Kinds/habits are ahead of everyone; the *funding model* is behind Monarch v2
(balance-derived) and Copilot (spend-from-goal lifecycle). → WF18, GL-series follow-on.

---

## Area 6 — Debt, loans, credit

**(a) CashFlux today.** /debt consolidation: total-owed hero + plan hero (method/extra/in-plan/
debt-free date), jump-to pill nav, watch-outs (no-minimum-recorded, utilization creep), payoff
ladder (PAY FIRST badge, APR chips, per-debt bars), avalanche/snowball strategy + burn-down
chart, per-liability include/exclude + inline APR/min editors, payoff calculator, per-loan
amortization cards + extra-payment simulator, credit proxy score ring + per-card utilization +
"pay $X to reach 30%," debt-coaching engine + tuner persisting DebtConfig, Learn tile.

**(b) What comps do better.**

- **YNAB 📄 — the loan account IS the planner.** Opening a loan account pops the payoff
  simulator directly: interest/time remaining, then a live slider/what-if where **every extra
  dollar shows time-and-interest saved in real time** on a burndown chart. Interaction cost:
  loan row → simulator = **1 click**; CashFlux: /debt → jump-to Loans → card → extra-payment
  input = 3–4. The deeper point: YNAB fused account + plan; CashFlux still has account rows
  (/accounts liabilities... now moved to /debt) and planning blocks as separate sections on one
  long page.
- **PiggySize ✋ — outcome-embedded strategy chooser + "Try $100/mo."** Documented in PS15;
  restated because it's the debt page where it lives: consequences printed *inside* the
  avalanche/snowball radio cards, preset chips, and a one-click apply on the insight banner.
  CashFlux's tuner is more powerful; their *first-90-seconds* experience is better.
- **Rocket Money 📄 — credit score as retention loop.** Full bureau-sourced score + report
  factors (premium). CashFlux's proxy is honest but the teardown point is: comps use credit as a
  weekly-return hook; CashFlux's proxy updates only when the user updates balances (see Area 3
  missing: manual score series as the honest middle ground).
- **Monarch 📄 — debt goals unified** (see Area 5b) and recurring-linked minimums: minimum
  payments appear in the recurring calendar with paid/missed states; CashFlux liabilities carry
  min-payment fields but the recurring surface doesn't auto-materialize minimum-payment
  expectations from them (see (d)).

**(c) Missing features.** One-click strategy presets with printed consequences (PS15);
promo-APR/0%-intro expiry modeling (no comp does this well either — Lunch Money forum-requested;
an actual differentiator opportunity: CashFlux has APR + dates and could model "promo ends
Sept → payment reallocates"); consolidated "debt-free journey" timeline view (burn-down exists;
a milestone-annotated timeline à la PiggySize stars does not); credit-utilization *forecast*
("after these scheduled payments post, utilization will be 27%").

**(d) Missing intra-feature connections.** **Liability min/due-day ↔ recurring**: liabilities
know min and due-day, but /recurring doesn't auto-create the expected payment (user re-enters it;
double-entry risk and the missed-payment detector can't fire on debts it doesn't know about).
**Debt plan ↔ allocate**: allocate ranks debt paydown, but confirming an allocation doesn't
update the debt plan's "extra/mo" — the two engines each think they own spare cash. **Payoff
ladder ↔ transactions**: paying a debt doesn't check off a ladder step or advance the plan
timeline (WF6's preview covers the before; nothing owns the after). **Credit proxy ↔ health**:
both compute utilization independently (two codepaths, one concept — the accent-two-systems
class of bug waiting to happen).

**(e) Verdict.** Deepest debt surface in the roster *on paper*; loses to YNAB on
loan-to-simulator immediacy and to PiggySize on first-touch legibility; the liability↔recurring
non-connection is the worst single gap (it breaks WF1's missed-payment detection). → PS15, WF6,
new connection items in §Master-gaps.

---

## Area 7 — Recurring, bills, subscriptions

**(a) CashFlux today.** Unified /recurring: scheduled flows (label/amount/cadence/account/
category/first-due/autopost/autopay), auto-detected charges with one-click add + detection
sensitivity prefs, bills list + 7×N calendar with urgency dots, mark-paid/remind, subscriptions:
select-to-cancel with savings math, price-change tracking, renewing-soon, late-charges-after-
cancel alert, how-to-cancel, ignored list, CSV, monthly/annual burden stats, overdue strip +
agenda + roster (RH-series hardening in progress).

**(b) What comps do better.**

- **Monarch 📄 — the recurring calendar's state colors.** Calendar entries are stateful:
  **green check = paid as expected; yellow check = paid at a different amount; red X = missed.**
  That third state is the one CashFlux lacks — we detect late charges after cancellation, but a
  *missed expected bill* (no charge arrived) isn't a first-class calendar state with an X the
  user can act on (→ WF12 lists it; Monarch's tri-state is the exact UI). Amount-drift (yellow)
  is also per-occurrence on the calendar, not only in a price-changes list.
- **Rocket Money 📄 — humans finish the job.** Concierge cancellation (premium) and bill
  negotiation (35–60% of first-year savings, success-fee). CashFlux correctly won't promise
  merchant-side action (WF12 non-goal); the honest gap is *outcome confirmation*, which WF12
  already specs (post-cancellation monitoring on later imports). What Rocket also does better:
  the cancellation flow captures **provider, method, date, confirmation number** as structured
  data; CashFlux's cancel-intent is a lighter record.
- **Simplifi 📄 — bills feed the plan** (Area 4); plus its calendar shows **projected balance
  effects per date**, fusing bills-calendar with cash-flow calendar (CashFlux has both — as two
  different pages: /recurring calendar and /planning runway chart) → WF9 is exactly this fusion;
  restated as a page-merge opportunity, not just a new feature.
- **PiggySize ✋ — needs/wants tagging + group subtotals** on bills (PS19), and email reminders
  (PS3).

**(c) Missing features.** Tri-state occurrence status (paid/amount-changed/missed) on the
calendar; variable-bill estimation (average-based expected amount with tolerance band — Monarch
and Rocket both estimate variable bills; CashFlux recurring amounts are fixed numbers with
price-change *detection* but no expected-range concept); annual-review of subscriptions
("you spent $842 on subscriptions this year, up $120" — we have the stats, not the yearly recap
moment); trial-ending detection (WF12 has renewal/trial notice — reaffirmed).

**(d) Missing intra-feature connections.** Liability payments ↔ recurring (Area 6d — the big
one); recurring ↔ budgets pre-reservation (Area 4d); recurring detection ↔ rules (a detected
recurring merchant should offer to create the categorization rule in the same confirm step —
today detection adds the flow but categorization is a separate pass); subscription cancel-intent
↔ workflows (no "when a charge from X arrives after cancel date, escalate" user-visible
automation — the late-charge alert is hardcoded, not a workflow the user can edit/extend).

**(e) Verdict.** The unified surface is the right call (comps agree — Monarch/Simplifi/Rocket
all converged here); occurrence-level state and variable-amount tolerance are the two mechanics
that make theirs feel "alive" and ours feel like a roster. → WF12, WF9, RH-series, PS3/PS19.

---

## Area 8 — Reports & cash flow analysis

**(a) CashFlux today.** Hero zone (net/income/spending + NW + savings rate + runway + no-spend
days), annual-review narrative header with 74/100 score + prose, Summary/Full report tabs, top
strengths/risks, recommended actions, Sankey money flow, top payees/expenses/deposits, income by
source, spending by member, category tab (peak-weekday, paired bars, donut, drill-through with
deltas, YoY, rollup), net-worth tab, advanced (custom-field spending, deductible totals),
per-dataset CSV + PDF, anomaly heads-up card.

**(b) What comps do better.**

- **Monarch 📄 — a report *builder*.** Custom charts: pick measure, group-by, chart type, date
  range, filters; save the configuration; **June 2026: full reports on mobile**. CashFlux ships
  ~fixed compositions + custom-field slice; a user cannot compose "median dining spend by member
  by quarter, as bars" without the formula engine. → W4 (reports builder) already tracks this;
  the teardown adds: Monarch's builder is the market bar now, not a power feature.
- **Actual 📄 — custom-report widgets on dashboards.** Custom reports (filters, live vs static
  date ranges, monthly mode) can be **pinned to any of several dashboards** — report-as-widget.
  CashFlux custom pages can't host report compositions (Area 1c). Actual also ships a **Budget
  Analysis report** (budgeted vs actual vs cumulative balance over time) — CashFlux has no
  budget-performance-over-time report at all (budgets show current period; Reports doesn't have
  a budgets tab).
- **Lunch Money 📄 — query-grade filtering everywhere.** Every list view is a saved-filter
  surface (tags/categories/accounts/amounts), effectively watchlists (→ WF8), and CSV-first
  export philosophy.
- **YNAB 📄 — Spending Breakdown + Income vs Spending.** 2026's Reflect additions: stack-ranked
  top-categories view and a six-month income-vs-spending bar with "more/less/in-line" verdicts —
  both *opinionated simplifications*, which is the same design direction as CashFlux's takeaways
  (parity, but their two-view minimalism onboards non-analysts faster than our four-tab surface).

**(c) Missing features.** Report builder (measure × group-by × chart × save) → W4; budget
performance-over-time report; report-as-widget on custom pages; scheduled report export
(no comp does email PDF well except enterprise tools — minor); benchmark comparisons
("households like yours" — Monarch/Copilot dabble; skip, anti-privacy).

**(d) Missing intra-feature connections.** Reports ↔ watchlists (WF8: a saved report slice
should be pinnable as a monitored number with thresholds); reports ↔ what-changed (WF5: every
report delta should open the attribution drawer); anomaly card ↔ notifications (the heads-up
card doesn't create dismissible notification items — parallel signals again).

**(e) Verdict.** Narrative layer is best-in-class; composition freedom is worst-in-class among
power tools (Monarch, Actual, Lunch Money all beat us). → W4, WF8, FB7.

---

## Area 9 — Investments

**(a) CashFlux today.** Per-account cards: holdings (ticker/shares/cost/price/class), 2×2
performance stats, asset-class allocation bars, portfolio summary card, valuation snapshots on
accounts. Manual prices.

**(b) What comps do better.**

- **Monarch 📄 — live security pricing.** Pick any market-traded security or crypto, enter
  quantity + purchase date → real-time portfolio value, benchmark comparisons, allocation.
  The price feed is the whole feature; CashFlux manual prices decay within days (the freshness
  problem squared). A Cloud-tier delayed-quotes feed (even EOD) is the minimum viable answer;
  fully local can still do user-pasted price CSV import (missing today — prices are per-holding
  hand-edits).
- **Empower 📄 — analysis, not just display.** Investment Checkup (portfolio vs recommended
  allocation, risk framing), **Fee Analyzer** (expense-ratio drag, projected to retirement),
  allocation drift. All free. CashFlux computes gain/loss and class mix only — no target
  allocation, no drift, no fee model (no expense-ratio field, Area 3c).
- **Ghostfolio (OSS) 📄 — proof it's doable open.** Activities-based portfolio tracking with
  performance (TWR), dividends, benchmarks, self-hosted. If CashFlux ever deepens /investments,
  Ghostfolio defines the OSS feature floor: buy/sell/dividend activity types (CashFlux holdings
  have no transaction/lot model — cost basis is a single number, so realized-vs-unrealized,
  lots, and dividends are unrepresentable).

**(c) Missing features.** Price feed (Cloud) or price-CSV import (local); activity/lot model
(buys/sells/dividends/splits); target allocation + drift; expense ratios + fee projection;
benchmark line; dividend income surfaced into Reports income.

**(d) Missing intra-feature connections.** Investment accounts ↔ transactions (a brokerage
contribution posted in the ledger doesn't create/update cost basis); investments ↔ allocate
(allocate's "Returns" profile scores accounts by user-entered expected return, not by actual
class mix); investments ↔ net-worth attribution (WF5: "NW +$3.1k: market +$2.8k, contributions
+$300" is impossible without the activity model).

**(e) Verdict.** Thinnest area relative to comps; W3 (investment depth) already filed — add the
activity/lot model as its prerequisite. → W3, WF5.

---

## Area 10 — Planning, forecasting, retirement

**(a) CashFlux today.** 12-month NW forecast (trailing-avg + trim-spending overlay + saved-plan
compare), can-I-afford-it, 60-day cash runway with payday balance + breach warning, what-if
plans (horizon/monthly/one-time, sparklines), health stress-tests (pay cut/surprise bill/rate
hike with sentence outcomes), /plan FOO-vs-Ramsey roadmap (new), five-year sample narrative.

**(b) What comps do better.**

- **PocketSmith 📄 — the forecasting ceiling.** Daily projected balance **on every calendar
  day**, forecast graph to 10/30/**60 years** by tier, multiple named scenarios each composed of
  different budget sets, what-if guides (income loss, habit changes). Their entire product is
  CashFlux's /planning page promoted to the organizing principle. Two specific mechanics to
  steal: **(1)** budgets and forecast are the *same object* (a budget line IS a forecast event —
  no separate what-if entry), **(2)** scenario = a complete alternative set of budget lines,
  switchable, not a single-variable overlay. CashFlux what-if plans are single-stream
  projections; scenarios-as-worlds → WF2 (restated: PocketSmith is the mechanics reference,
  Monarch Forecasting the UX reference).
- **Monarch 📄 — Forecasting (Apr 2026, Plus tier).** What-if modeling for "retirement, home
  buying, career transitions **using actual account data**" — the mainstream version of WF2,
  now shipped by the market leader. Also proves the tiering thesis: forecasting/business =
  premium tier (their Plus ≈ our Cloud differentiation opportunity).
- **PiggySize ✋ / Boldin 📄 / ProjectionLab 📄 — the retirement tier CashFlux lacks entirely**
  (PS1 filed with PiggySize's mechanics). Boldin adds beyond PiggySize: **up to 10 scenarios
  side-by-side**, Roth Conversion Explorer with four optimization strategies (max estate / min
  lifetime tax / to income-tax threshold / to IRMAA threshold), lifetime IRMAA in scenario
  manager, **Spending Guardrails insight (Mar 2026)**, 15+-metric wellness score with coach.
  ProjectionLab adds: 10k Monte Carlo, tax analytics with **visualized brackets per income
  type**, 72t/SEPP modeling, drawdown-order optimization ("Optimize" coordinates conversions +
  gain harvesting + drawdown order), milestone timelines, PDF plan reports — while being
  privacy-respecting and manual-entry-friendly (the philosophical proof that PS1 fits
  local-first).
- **Empower 📄 — recession simulator.** Replay your plan through historical events (Dotcom,
  2008). A cheap, emotionally powerful variant of stress-testing CashFlux's what-if shocks
  don't have (ours are parameter shocks, not historical replays).

**(c) Missing features.** Retirement engine (PS1 — the decisive gap, third source confirming);
scenarios-as-worlds (WF2); historical-replay stress test; forecast horizon beyond 12 months
(even 5y, given the five-year sample seed exists); tax awareness anywhere in planning (PS9
tax-ref pack is the enabler); guardrails-style safe-spend band; plan PDF export.

**(d) Missing intra-feature connections.** Forecast ↔ recurring (the 12-month forecast uses
trailing averages, not the recurring roster — CashFlux *knows* next year's bills and ignores
them in its own forecast; PocketSmith/Simplifi build the forecast *from* scheduled items first);
what-if plans ↔ debt/goals (a what-if can't include "pay off card in March" as an event —
single-stream); /plan roadmap ↔ allocate (the FOO step "build emergency fund" should hand its
dollar amount to allocate as a constraint); stress tests ↔ notifications (a failing stress test
is not a surfaced signal anywhere outside /health).

**(e) Verdict.** Best-in-class *short-horizon* explainable planning; absent long-horizon.
Forecast-from-recurring is the highest-value cheap fix in this entire document (data exists,
engines exist, they're just not wired). → PS1, WF2, WF9; new items §Master-gaps.

---

## Area 11 — Household, sharing, multi-user

**(a) CashFlux today.** Members with roles/colors/defaults, reassign-on-delete, per-owner net
worth + spending, split-a-bill (weights, Mermaid settle diagram, running balance, record
settle-up), owner on every entity, single-device.

**(b) What comps do better.**

- **Monarch 📄 — Shared Views ("yours, mine, ours," Oct 2025).** They shipped FB3's exact
  vocabulary as *view filters with partner-scoped dashboards* — each partner sees their slice
  and the shared slice under one subscription, real multi-login. Plus **advisor/collaborator
  seats** (read-only professional access) — a whole audience CashFlux can't serve single-device.
- **YNAB 📄 — YNAB Together.** Up to **six people** on one subscription with real-time shared
  budget editing (2026: "Shared Family Budgets… update the same budget in real time"), and the
  culture around "money dates." The collaboration primitive is co-editing, not just co-viewing.
- **PiggySize ✋ — separate logins + row-level security** (PS4, filed).
- **Goodbudget 📄 — sync-as-the-product.** Envelope sharing across two devices free, more paid
  — proof that even the simplest tier treats multi-device as table stakes.

**(c) Missing features.** Multi-login/multi-device (§7 sync — everything else here is blocked on
it); partner-scoped dashboard views (FB3 extends from labels to views); advisor read-only seat
(Cloud); real-time co-presence (post-sync; YNAB is the only one doing true realtime — mark as
later).

**(d) Missing intra-feature connections.** Split ↔ recurring (a recurring shared bill can't
auto-generate per-member splits each period); member roles ↔ AI/assistant (no per-member
assistant permissions — matters the day sync lands); per-owner reports ↔ budgets (budgets have
owners but there's no "Priya's budget view" that filters the whole surface — the Everyone
selector exists in the top bar; verify it filters budgets deeply, FEATURE_MAP suggests partial).

**(e) Verdict.** Analytically ahead (per-owner math, split/settle), operationally behind
everyone with sync. Nothing new to file — §7 + PS4 + FB3 cover it; the teardown's contribution
is Monarch's shipped naming (validation) and the advisor-seat idea for Cloud.

---

## Area 12 — Automation, workflows, API, extensibility

**(a) CashFlux today.** Workflows: 6 triggers (manual/txn-added/scheduled/budget-exceeded/
goal-reached/bill-due), condition formulas with and()/or()/not(), staged action lists, dry-run,
run history, Mermaid flowcharts, PYF/surplus-sweep/round-up savings automations; formula engine
with molecules; custom fields (5 entity types, 5 data types); widget builder (node graph, 16
presets); custom pages; theme editor. No public API.

**(b) What comps do better.**

- **Lunch Money 📄 — a real developer API.** Documented REST API (lunchmoney.dev) over
  transactions, categories, tags, accounts, budgets, recurring; community projects directory;
  automation via triggers/actions. CashFlux's equivalent audience (GitHub-sourced power users)
  gets… nothing scriptable: no CLI, no local HTTP API, no import automation beyond the UI. For a
  local-first Go app this is unusually cheap to add (the store + logic packages are pure; a
  `cashflux serve --api` local endpoint or file-watch import folder would leapfrog Lunch Money
  on privacy since it never leaves localhost).
- **Firefly III 📄 — webhooks + headless import.** Webhooks on transaction create/update/delete
  feed n8n/Huginn pipelines ("budget exceeded → automation"); the **Data Importer** runs as a
  separate container, can *fetch bank statements from SFTP or email attachments on cron* —
  hands-free ingestion into a self-hosted app. That inbox-driven import pattern (forward your
  bank's statement email → it lands in drafts) is the single best manual-import friction killer
  in the OSS world and directly attacks CashFlux's freshness problem (Area 3) — a Cloud-tier
  email-ingest address, or local watch-folder, fits CashFlux's privacy posture.
- **Actual 📄 — API + headless library.** Actual exposes a JS API against the local file; the
  community built sync bridges on it (e.g., SimpleFIN sync daemons). Same lesson as Lunch Money.

**(c) Missing features.** Local API/CLI; watch-folder or email-ingest import; webhooks (local:
workflow action "call URL" — the workflow engine has no outbound action); workflow triggers
missing: goal-behind-pace, stress-test-failing, price-change-detected, missed-bill; scheduled
*reports* as workflow output.

**(d) Missing intra-feature connections.** Rules ↔ workflows (Area 2d — the flagship
disconnect); formulas ↔ workflows conditions are connected, but formulas ↔ *widget builder*
bindings and formulas ↔ custom fields (a formula can't read a custom field per-entity —
verify; if true, the three extensibility systems are pairwise-disconnected); workflows ↔
notifications (workflow run results don't emit notification items).

**(e) Verdict.** The *in-app* automation ceiling is the highest in the roster; the *ecosystem*
ceiling is the lowest among power tools (no API at all). For the audience CashFlux actually
reaches today, that ordering is exactly backwards. New: local API/CLI + watch-folder + outbound
action → §Master-gaps (candidate series).

---

## Area 13 — AI & assistant

**(a) CashFlux today.** BYO-OpenAI chat with tool-calling agent harness, conversations,
system-prompt editor, pin/save-as-task, insights (anomalies, highlights, merchants), ~84-item
Smart catalog with per-feature cadence/cost controls + digest, vision receipt/statement
extraction, allocate explainer, no-key fallbacks.

**(b) What comps do better.**

- **Monarch 📄 — the three-product framing.** Assistant (Q&A + navigation help), AI Insights
  (interpretation), Weekly Recap (automatic narrative) — three *named, scoped* AI features with
  an explicit "About Monarch's AI" trust page. CashFlux's Smart catalog is far bigger but reads
  as a catalog; Monarch's is legible as three promises. (Packaging, not capability.)
- **PiggySize ✋ — turnkey + spotlight + tax refs + capability honesty** (PS7–PS10, filed).
- **Copilot 📄 — invisible AI.** "Copilot Intelligence" isn't a chat; it's categorization,
  transfer detection, and suggestions woven into flows. The lesson for the Smart catalog:
  the best-reviewed AI in the market is the one users never configure. Our density
  dial/cadence/mute manage exposure but still ask the user to *manage a catalog*.
- **Rocket/Simplifi 📄 — nothing notable.** (Stated for completeness: AI is not why anyone
  picks them.)

**(c) Missing features.** Turnkey path (PS7); guided-nav spotlight (PS8); weekly narrative
recap (Area 1); versioned tax refs (PS9). Nothing else new — CashFlux's agent tooling exceeds
every comp's shipped chat.

**(d) Missing intra-feature connections.** Assistant ↔ workflows (the agent can't create a
workflow from a request like "email me when dining exceeds $400" — also blocked on outbound
action); assistant ↔ what-changed (WF5 outputs should be the assistant's evidence base);
insights ↔ review inbox (WF1 will fix the parallel-feeds problem — reaffirmed from the AI side).

**(e) Verdict.** Capability ahead, friction and packaging behind. → PS7–PS10, AG-series.

---

## Area 14 — Health, insights, financial guidance

**(a) CashFlux today.** Six-factor health score with contribution bar + live-formula
disclosure, stress tests, money-leaks, resilience, trend; /plan FOO/Ramsey roadmap with
NotAssessable honesty; debt coaching; recommendation surfaces.

**(b) What comps do better.**

- **Boldin 📄 — wellness as program.** 15+-metric wellness score + digital coach + "chance of
  success" + guardrails. The structural idea CashFlux lacks: **score → program → tracked
  progression** (coach items persist and check off; our "Where to focus next" regenerates
  statelessly each visit — no memory of what you already did). Connect health actions to the
  todo system and it becomes a program (the XC machinery exists!).
- **PiggySize ✋ — nothing** (smiley face). Stated because it's the rare area where the market
  is *behind* CashFlux almost uniformly — YNAB/Monarch/Copilot have no health score at all.

**(c/d) Missing.** Score-to-program persistence (health recommendation ↔ todo with completion
feeding the score's trend annotation); factor-level history ("utilization factor improved 3
months straight"); that's it.

**(e) Verdict.** CashFlux's differentiator area; one connection (recommendations→tracked
program) turns it from scorecard to product spine. → new item §Master-gaps; composes with FB4's
outcome-first onboarding.

---

## Area 15 — Privacy, sync, platform, data safety

**(a) CashFlux today.** Local-first wasm, IndexedDB/SQLite, JSON/CSV export, PWA + offline SW,
cloud opt-in (portal/zero-knowledge envelope sync in productionization), passcode/applock,
about-page disclosures.

**(b) What comps do better.**

- **Actual 📄 — e2e-encrypted *working* multi-device sync, today.** Optional server, e2e
  encryption, local files with export at will, migration importers (YNAB4/nYNAB). The exact
  posture CashFlux Cloud is building — shipped, free, OSS. Also: **budget file management**
  (multiple budget files per instance — CashFlux has one dataset per browser profile; no
  "second household file" concept).
- **Monarch 📄 — SOC 2 (Jan 2026).** Compliance as marketing. Cloud-tier note for later.
- **PiggySize ✋ — plain-language privacy page + AI capability framing** (PS7/PS22).
- **Everyone 📄 — account recovery.** The dark side of no-account: browser-profile loss = data
  loss (FB6 filed). Comps' answer is trivially "log back in."

**(c/d) Missing.** Multiple local datasets/files; migration importers (Mint CSV exists-ish via
mapping profiles, but *named* importers for YNAB/Monarch/Mint formats with field semantics —
cheap and high-converting for switchers); encrypted export as the default backup format
(export exists; encryption optionality unclear → fold into FB6); device-transfer flow
("move to a new computer" wizard).

**(e) Verdict.** Posture is the strongest in the roster *once sync ships*; until then Actual
holds the crown. → FB6, §7, new: named importers + multi-file + transfer wizard.

---

## Area 16 — Onboarding & first-run

**(a) CashFlux today.** 4-step wizard (currency → income → account → members), sample-data
loader, six-step help checklist, empty-state CTAs.

**(b) What comps do better.**

- **Simplifi 📄 — outcome in minutes.** Link accounts → the Spending Plan self-assembles.
  Time-to-first-insight is minutes with zero configuration. CashFlux (by nature) needs data
  entry first — which is exactly why the sample-data path and FB4's outcome-first shape carry
  the load.
- **Copilot 📄 — progressive trust.** First 30 transactions un-categorized on purpose (the
  model watches you), then automation ramps. Onboarding as calibration, not configuration.
- **PiggySize ✋ — persona demo + tour + per-page guides** (PS21/PS20, filed).
- **YNAB 📄 — pedagogy.** "The Plan"/method guides, template categories, an education machine
  around one idea. CashFlux's /help topics are reference, not curriculum; /plan's FOO/Ramsey is
  the right raw material for a curriculum framing.

**(c/d) Missing.** FB4 (filed) covers the structure; add: template category sets by persona
(YNAB templates; our 50/30/20 starter is one template — where are "family with kids,"
"freelancer, irregular income"?); import-first onboarding branch (start from a CSV, infer
categories/recurring, THEN ask questions — the reviewer's Journey 1).

**(e) Verdict.** Weakest area vs effort required to fix; every comp with good retention leads
here. → FB4, PS21, new: persona templates + import-first branch.

---

# PART II — ANNOUNCED & RECENTLY SHIPPED ROADMAPS PER COMP

What each comp has publicly shipped-recently or announced (dates from vendor release pages).
Read this as "where the market is moving."

**Monarch** (whats-new page, verbatim timeline): Jul 2025 credit-score tracking → Aug 2025
browser-extension Target support + credit health → Oct 2025 **Shared Views (yours/mine/ours)**
→ Oct 2025 domain move to monarch.com → Dec 2025 Winter Release (**AI Assistant, Goals rebuild,
equity tracking, receipt scanning**) → Jan 2026 SOC 2 → **Apr 2026 Forecasting** (what-if on
real data) → **Apr 2026 Monarch Plus tier** (retirement planning, business tracking, financial
modeling) → Jun 2026 mobile reports + smarter forecasting + goals enhancements → **Jun 2026
Goals v2 out of beta**. Direction: upmarket into planning/business via a premium tier — straight
at PS1/PS2 territory, from the aggregation side.

**YNAB**: 2026 messaging = smarter automation, faster performance, family tools. Shipped:
Spending Breakdown (stack-ranked categories), Income-vs-Spending six-month Reflect insight,
improved reports dashboard, **real-time shared family budgets**, faster bank syncing + mobile.
Direction: families + reducing its own famous learning curve.

**Copilot**: Dec 2025 **web app** (ends Apple exclusivity; Android via web); continuous
Intelligence improvements (95% auto-categorization claim, transfer detection). Direction:
platform expansion + invisible AI.

**Simplifi**: continuous; the Quicken blog pushes retirement-calculator content around it —
expect planning features to seep down from Quicken Classic. Watchlists/Pace-style refinements
iterate quietly.

**Rocket Money**: stable feature set; pricing pressure (free tier + $7–14 band); credit &
"savings autopilot" emphasis.

**Lunch Money**: **budgeting v2 in public beta** — custom periods incl. twice-monthly,
zero-based, carryover pools with adjustable/resettable starting balances; redesigned budgeting
UI; calendar view detail; mobile split/unsplit; **financial-coach directory** (marketplace
motion). Direction: shoring up its one weak area (budgets) + services.

**PocketSmith**: tier ladder stable (10/30/60-year horizons); iterating on forecast graph +
calendar integration.

**Actual Budget**: monthly releases (26.x): **multiple dashboard pages**, budget-analysis
report, balance-forecast + age-of-money widgets, dashboard import/export, SimpleFIN hardening
(SSRF), custom themes, experimental Excel-like rule formulas. Direction: dashboards/reports
maturity + sync robustness — closing exactly the gaps CashFlux still has open (Area 1c/8c).

**Firefly III**: v6.x cadence (v6.4→6.6 through 2026), subscriptions concept maturing, importer
automation (SFTP/email cron), webhooks stable. Direction: automation depth.

**PiggySize**: covered in PS-series; positioning vs YNAB/Monarch on price + no-bank-credentials;
free-tools SEO farm.

**Boldin**: Mar 2026 Spending Guardrails; lifetime IRMAA; scenario manager depth. **ProjectionLab**:
Optimize (multi-strategy coordination), tax analytics; lifetime price raised $799→$1,199 (demand
signal for exactly the PS1 feature class).

Synthesis: the market is converging from both ends on the same seam CashFlux sits on — trackers
are adding planning tiers (Monarch Plus), planners are adding guardrails/coaching (Boldin), and
the OSS local-first twin (Actual) is adding dashboards + sync hardening. The unoccupied position
remains: **ledger + planning + explainability, fully local** — but Monarch Plus shows the
clock is running.

---

# PART III — MASTER LISTS

## III.1 Missing features not previously filed (NEW in this teardown)

Grouped; existing-ticket overlaps excluded (those are cross-referenced in Part I).

**Dashboard/metrics:** age-of-money metric; balance-forecast dashboard tile; pace framing on
safe-to-spend; weekly narrative recap; multiple dashboards composed from built-in tiles.
**Transactions/rules:** per-txn confidence + ranked alternates; learned auto-rules w/ opt-out;
imported-payee vs display-payee model; rule stages + specificity ranking; regex conditions;
rule actions split/tag/notify; retro-apply count preview at save; automatic transfer pairing;
refund/expected-credit tracker; reconciliation lock; bulk-edit→rule prompt.
**Accounts:** expected-update cadence (filed WF4 refinement); manual credit-score series;
expense-ratio field + fee rollup; exclude-from-net-worth toggle; account-level docs/notes.
**Budgets:** typed targets (set-aside vs refill, by-date); one-number Flex mode; auto-maintained
spending plan; semimonthly period; adjustable rollover balance + epoch; category snooze.
**Goals:** balance-derived progress; multi-account funding; debt goals in goal list; lifecycle
states (active/ready/reactivate); projected-completion verdict on card.
**Debt:** promo-APR expiry modeling (differentiator opportunity); utilization forecast;
milestone timeline view.
**Recurring:** tri-state occurrence status (paid/amount-drift/missed); variable-amount
tolerance bands; structured cancellation record (method/date/confirmation #); annual
subscription recap.
**Reports:** budget-performance-over-time report; report-as-widget on custom pages.
**Investments:** activity/lot model; price CSV import (local) / price feed (Cloud); target
allocation + drift; benchmark; dividends→income.
**Planning:** forecast-built-from-recurring (highest-value cheap fix in the document);
historical-replay stress test; ≥5y horizon; plan PDF export.
**Ecosystem:** local HTTP API/CLI; watch-folder import; email-ingest import (Cloud);
workflow outbound-URL action; new workflow triggers (goal-behind-pace, stress-fail,
price-change, missed-bill).
**Data safety/switching:** named importers (YNAB/Monarch/Mint semantics); multiple local
datasets; device-transfer wizard.
**Onboarding:** persona category templates; import-first onboarding branch.
**Health:** score-to-program persistence (recommendations → tracked todos feeding trend).

## III.2 Missing intra-feature connections (the full connection map)

The recurring theme of this teardown: CashFlux builds excellent engines that don't talk.
Consolidated from Part I (d) sections, ranked by damage:

1. **Liability min/due-day ↛ recurring** — breaks missed-payment detection & double-entry risk. (A6)
2. **Forecast ↛ recurring roster** — the app ignores its own knowledge of next year's bills. (A10)
3. **Rules ↛ workflows** — two automation engines, zero bridge. (A2, A12)
4. **Allocate ↛ debt plan / goals / budgets** — three owners of spare cash that don't reconcile
   after an allocation is applied. (A4, A5, A6)
5. **Recurring ↛ budget pre-reservation** — remaining-this-period lies about committed spend. (A4)
6. **Goals ↛ safe-to-spend** — committed contributions not subtracted. (A5)
7. **Health recommendations ↛ todo program** — stateless advice, no tracked progression. (A14)
8. **Credit proxy ∥ health utilization** — one concept, two codepaths. (A6)
9. **Dashboard tiles ∥ custom-page widgets** — two widget systems. (A1)
10. **Anomaly card / digest / notifications** — three parallel signal feeds, no shared state. (A1, A8)
11. **Recurring detection ↛ rule creation** — same confirm, separate passes. (A7)
12. **Vision receipts ↛ recurring history**; **dupes ↛ import batch provenance**. (A2, A7)
13. **Investment accounts ↛ ledger cost basis**; **valuations ↛ NW-trend annotation**. (A9, A3)
14. **Split ↛ recurring** (shared bills don't auto-split per period). (A11)
15. **Stress tests / workflow runs ↛ notifications**. (A10, A12)
16. **Formulas ↛ custom fields** (verify) and extensibility systems pairwise-disconnected. (A12)

WF1 (queue) + WF6 (action preview) + WF5 (attribution) are the systemic fixes for #4/#7/#10/#15;
#1/#2/#5/#6 are cheap dedicated wiring wins; #3/#9 are architecture consolidations.

## III.3 Cross-app interaction-cost comparison (selected tasks)

Legend: ✋ hands-on measured · 📄 doc-derived flow. CashFlux from §3 baseline.

| Task | CashFlux | Best comp | Their cost | Notes |
|---|---|---|---|---|
| Dispatch one ordinary imported txn | 3–4 clicks (locate + inline edit) | Copilot 📄 | **1 tap** from To-Review card | The FB2 metric in one row |
| Fix miscategorization | 3–4 | Copilot 📄 | 1–2 (second-best suggested) | |
| Create rule from a txn | 2–3 | Actual 📄 | 0–1 (rename prompts rule) | Actual asks; we require intent |
| Batch-edit via rule preview | n/a (count only) | Actual 📄 | rule editor lists matches + Apply actions | WF7 spec |
| See loan payoff simulator | 3–4 (/debt → Loans → card) | YNAB 📄 | 1 (open loan account) | |
| Try +$100/mo on debt | 2–3 + type | PiggySize ✋ | **1 click** (Try $100/mo banner) | PS15 |
| Answer "can I afford $X" | 2 + form (/planning) | PocketGuard 📄 | 0 (In-My-Pocket always on) | Different question-shape, same job |
| See projected balance on a date | ~3 (/planning runway, 60d max) | PocketSmith 📄 | 1–2 (calendar day, decades) | |
| Switch budget month context | 1–2 (top-bar period) | YNAB 📄 | 1 | parity |
| Confirm a bill occurrence paid | 3 (/recurring row) | Monarch 📄 | 1–2 (calendar check) | tri-state gap |
| Reach "what changed this week" | n/a (month in Reports) | Monarch 📄 | 0 (recap delivered) | |
| Export everything | 2–3 | Actual 📄 | 2 | parity |
| Start a retirement projection | **∞ (absent)** | PiggySize ✋ | 2 from nav | PS1 |

## III.4 Sources

Vendor: ynab.com features/whats-new + support.ynab.com (targets, reflect) · monarch.com
whats-new + help.monarch.com (rules, flex, rollovers, goals, recurring, AI) · copilot.money +
help.copilot.money (rollovers, savings-goal spending, quick start) · quicken.com/simplifi ·
lunchmoney.app (+ feedback.lunchmoney.app changelog, lunchmoney.dev) · pocketsmith.com +
learn.pocketsmith.com · rocketmoney.com · empower.com · piggysize.com (+ live demo, hands-on
2026-07-23) · boldin.com (features, release notes, Roth explorer) · projectionlab.com ·
actualbudget.org/docs (rules, reports, bank-sync, releases 26.x) · docs.firefly-iii.org +
github firefly-iii releases · goodbudget/everydollar/pocketguard sites.
Reviews (2026): Forbes Advisor, NerdWallet, Engadget, CNBC Select, The Penny Hoarder, The
College Investor, Money with Katie, Rob Berger, Wall Street Survivor, FinanceBuzz, Ramsey
comparisons, MoneyCrashers, moneywise.
CashFlux: FEATURE_MAP.md §1–3, live :8080 session (2026-07-23), TODOS.md series, SPEC.md.

*Caveats: paywalled apps not operated hands-on (flows quoted from official help docs); prices
as reported mid-2026; Monarch Plus pricing not publicly itemized in sources reviewed.*

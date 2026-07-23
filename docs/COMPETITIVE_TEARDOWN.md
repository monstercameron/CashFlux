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

---

# PART IV — FINE-GRAINED FEATURE CROSS-EXAMINATION

Added 2026-07-23 (second pass, per Cam: "fine detailed feature cross examination"). Part I
compared *areas*; this part interrogates *individual mechanics* — one section per feature, a
matrix across comps, then a cross-examination and the exact CashFlux deltas. Comp abbreviations:
**YN**=YNAB · **MO**=Monarch · **CP**=Copilot · **SI**=Simplifi · **RM**=Rocket Money ·
**LM**=Lunch Money · **PSm**=PocketSmith · **AC**=Actual · **FF**=Firefly III · **PG**=PiggySize
· **CF**=CashFlux. "—" = feature absent. Cells marked (?) were not verifiable from sources.

## IV.1 Transaction entry & quick capture

| | Manual entry | Quick-add from anywhere | Keyboard-complete entry | Scheduled/future-dated entry | Mobile widget/watch |
|---|---|---|---|---|---|
| YN | register row, payee/category memory autosuggest | mobile quick-add + **home-screen widgets** | **Yes — famous shortcut system**, discoverable (hints shown per action; toggleable in Settings) | Yes — scheduled txns, 9 frequencies (see IV.9) | Yes (widgets) |
| MO | web + mobile forms | mobile quick actions | partial | recurring items, not arbitrary future txns (?) | push-first mobile |
| CP | mobile-first form | iOS shortcuts/widgets | n/a (touch-first) | via recurring | Yes (Apple ecosystem) |
| LM | web row entry | — | good web-row ergonomics | future-dated supported | mobile app v2 |
| AC | register row | — | strong register keyboard flow | schedules cover it | PWA |
| FF | multi-split form (heavyweight by design) | — | form-based | recurrences | community apps |
| PG ✋ | form per entity (no txns at all) | — | — | n/a | PWA |
| **CF** | modal form; dashboard hero **1-click** to form | **+ Add flip-panel planned (B11)**; Ctrl-K palette exists | **unverified end-to-end** — palette + shortcuts (?/Ctrl-K/Alt+1–9) exist, but no documented register-level key grammar | recurring autopost covers repeating; **arbitrary future-dated txn allowed but doesn't drive forecasts** | PWA only |

**Cross-examination.** YNAB wins entry on two mechanics CashFlux lacks: *payee memory* (typing a
payee pre-fills last category+amount — entry converges to 3 keys: payee-prefix ⇥ Enter) and a
*documented, discoverable* shortcut grammar (in-context hints teach it). CashFlux's 1-click hero
button is the best *button* placement in the roster, but the form after it is untyped repetition
— no payee-memory prefill. **Deltas:** payee-memory prefill (last category/amount/account per
payee — the rules engine's data can seed it); register-level shortcut grammar documented in /help
and hinted in-context; finish B11 quick-add panel; future-dated txns should register in runway.

## IV.2 Splitting

| | Split by | Rule-driven auto-split | Un-split reversible | Temporal spread | Split visibility |
|---|---|---|---|---|---|
| YN | amount subtransactions (API: `subtransactions[]`) | — | delete splits | — | inline expand |
| MO | amount or **percentage**; **Smart Split rules** auto-split matching txns, retro-applicable with count | **Yes** | yes | — | split filter exists |
| CP | equal / dollar / **percentage**; shortcuts | — | original amount always on record | **Yes — spread across 3/6/12 months as dated child txns, full history** | dedicated split icon |
| LM | split + **group** (inverse: merge several txns into one logical txn) | via rules | yes (mobile can split/unsplit) | — | grouped-view |
| AC | splits in register; **unsplit keeps parent info** | — (rule formula mode experimental) | **yes, lossless** | — | ⚠ splits distort filtered-view totals (documented bug — a cautionary spec note) |
| **CF** | SplitEditor inline (amounts) | — | (?) | — | split rows in table |

**Cross-examination.** Three mechanics CashFlux lacks: **percentage splits** (MO/CP), **rule-
driven splitting** (MO — the household use-case: auto-split every paycheck/joint-card txn by
60/40), **temporal spread** (CP — semi-annual insurance amortized into 6 monthly child txns with
the original preserved; this is the correct answer to lumpy expenses that CashFlux currently
answers with sinking funds only), and **grouping** (LM — merging N real txns into one logical
event, the inverse of split; useful for trips/events; CashFlux tags approximate but don't total
as one unit). Actual's documented filtered-totals distortion is a spec warning: define split
semantics in filtered aggregates before building. **Deltas:** percentage mode on SplitEditor;
split action in rules (→ WF7); temporal-spread as first-class ("amortize this txn over N
months" creating linked children, budgets seeing the children); txn grouping.

## IV.3 Transfers & internal-movement intelligence

| | Transfer entity | Auto-detect/pair imported legs | Cross-currency | Excluded from spend correctly |
|---|---|---|---|---|
| YN | payee-based transfer, category-free | on import, matches legs (linked accounts) | — | yes |
| MO | transfer category + rules can set | detection + hide from cash flow | (?) | yes |
| CP | **Intelligence auto-recognizes internal transfers** | **yes, no user action** | — | yes ("not double-counted") |
| LM | transfer grouping | pairs plaid legs | multi-currency aware | yes |
| AC | transfer payees between on-budget accounts | on import via matching | — | yes |
| FF | first-class transfer type (double-entry) | importer rules | yes | yes |
| **CF** | first-class transfer (from/to/amount/date) | **no pairing on import** | FX-aware ledger | yes |

**Cross-examination.** CashFlux's *entered* transfers are fine; **imported** ones arrive as two
unlinked txns (a Venmo-out and a deposit-in) that inflate spend+income until hand-fixed. Copilot
treats this as a zero-click Intelligence feature; even Actual pairs on import. This is WF11's
core case and the #1 source of "my numbers look wrong" for import-heavy users. **Delta:**
import-time transfer pairing (amount ± tolerance, date window, opposite signs, cross-account) →
propose-link UI in the draft-review step, and retro-detector over the ledger.

## IV.4 Tags, flags, labels, marks

| | Mechanism | Multiple per txn | Budget/report integration | Special semantics |
|---|---|---|---|---|
| YN | **7 color flags** + memo (no tags) | one flag | flag filter in register | flags-as-workflow (e.g. "check later") |
| MO | tags | yes | tag filters in reports/rules; CSV import maps tags | needs-review is a separate state |
| LM | tags | yes | **budgetable-by-tag** (watch-list style), rules add tags | "exclude from totals" is a **category property** |
| PSm | **labels** | yes | search/filter + saved searches | explicit tax-deductible labeling pattern |
| AC | **#tags inside notes text** (case-sensitive, no spaces, ## escapes) | yes | 'has tag(s)' filter; find-existing-tags button | zero-schema — tags are conventions |
| FF | tags | yes | tags in search/rules/reports | tag "clouds," tag pages |
| **CF** | tags | yes | filter toolbar + rules can set tags (?) — **reports cannot group by tag** | deductible flag lives on *categories*, not txns |

**Cross-examination.** CashFlux tags exist but are a filter, not an analysis dimension: Reports
has by-category/payee/member/custom-field — **no by-tag report**, and no tag-scoped budget/
watchlist. YNAB's flags suggest a second, orthogonal need CashFlux also lacks: a lightweight
*workflow mark* ("look at this later") distinct from taxonomy — our review inbox partially
covers it, flags are the manual version. **Deltas:** tag dimension in Reports + WF8 watchlists;
per-txn deductible override (category-level only today); consider flag/star as a workflow mark.

## IV.5 Search & filtering

| | Filter surface | Query language | Saved searches | Bulk actions from search | Special |
|---|---|---|---|---|---|
| MO | rich filter set: debit/credit, hidden, synced-vs-manual, **has attachments / splits / notes** | — | shared views | yes | filters double as rule criteria |
| LM | every column filterable | — | saved filters | yes | API queries |
| PSm | search engine over txns | criteria search | **saved searches, one-click in side panel** | **yes — bulk categorize from search results** | search-as-workbench |
| AC | stacked multi-filters | — | (dashboard widgets take filters) | via rule-editor trick (IV.7) | 'has tag(s)' |
| FF | global search | **full operator language: from:, to:, amount ranges, dates, has_attachments:true, AND/OR (no NOT)** | via rules ("search rule engine") | **searches can BE rules** | the ceiling |
| **CF** | filter toolbar (account/category/member/tag/date/amount/cleared/custom-field) + chips, persisted | — | **no saved filter sets** | bulk bar on selection | Ctrl-K palette (nav, not txn search?) |

**Cross-examination.** CashFlux's filter *breadth* is top-3; what's missing is the *workbench
layer*: no saved searches (PSm's one-click side panel; MO's shared views), no query syntax for
power users (FF), and — the sharpest gap — no path from "this filtered set" to "monitor this"
(WF8) or "make this a rule" (WF7). Firefly's deepest idea: **a search IS a rule** (the
search-rule engine runs a stored query as the rule trigger) — one grammar for finding,
monitoring, and automating. That's the architecture WF7/WF8 should share instead of three
grammars. **Deltas:** saved filter sets (→ WF16/WF13 saved views — reaffirmed); filtered-set →
watchlist/rule handoff; evaluate one shared predicate grammar (filters = rules conditions =
watchlist criteria = formula predicates).

## IV.6 Review states & assignment

| | New-txn state | Dispatch gesture | Assignment | Bulk review | Session undo |
|---|---|---|---|---|---|
| CP | To-Review queue on Dashboard | **1 tap** Mark-Reviewed; suggested type+category shown | — | yes | (?) |
| MO | "Needs review" flag; review-status buttons atop txn page | 1–2 clicks | **assign a specific household member to review** | yes | — |
| LM | "unreviewed" default state | row-level | — | bulk | — |
| **CF** | review inbox (count badge) | navigate → row edit (3–4) | — | bulk bar exists | **bulk-undo snapshot exists** (better than comps) |

**Cross-examination.** Monarch's **assign-to-member review** is the one mechanic here nobody
else has and CashFlux's household model is *perfectly shaped for* (owner exists on every entity;
a "review by Priya" assignment is a natural extension) — a differentiator we could take from
them cheaply. Otherwise this table is WF1/FB2 restated with numbers. **Deltas:** WF1 queue with
1-keypress dispatch; per-member review assignment (new — extends WF1); keep and advertise
bulk-undo (unique strength).

## IV.7 Categorization intelligence (rules vs learning)

Consolidated matrix for the Area-2 prose:

| | Match fields | Operators | Stages/priority | Actions | Learning | Retro-apply | Editor superpower |
|---|---|---|---|---|---|---|---|
| MO | original statement, merchant, amount, more | exact/contains | list order | rename merchant, category, **tags, owner, hide, SPLIT** | — | **checkbox with change-count** | criteria from filters |
| LM | payee, amount, etc., multi-condition | contains/starts/exact | **explicit priority levels** | multi-action: category, tags, **split, email-notify** | — | yes | one rule, many actions |
| AC | imported-payee vs payee, account, category, date, notes, amount(in/out) | is/contains/one-of/**regex** | **pre/default/post + specificity auto-rank** | category/payee/notes(prepend/append)/cleared/account/date/amount, formula mode | **auto-rules from renames + most-common-category, opt-out per payee** | via editor | **live match list + Apply-actions = batch editor** |
| FF | any search operator | full query language | rule groups, strict/non-strict | set category/budget/tags, **link to bill/piggy bank**, description | — | run on stored txns | searches-as-rules |
| CP | n/a (model, not rules) | — | — | — | **ML, confidence, 2nd-best suggestion, transfer detection** | — | invisible |
| PSm | merchant memory + filters | contains | filter order | category, rename, labels | **bank-feed auto-categorize default-on + merchant memory** | filters re-run | — |
| **CF** | keyword phrase (+conditions override) | contains | drag order, shadow warnings | category, tags(?), rename-desc | AI-suggested rules (batch) | page-level Apply-to-existing | live match **count**, coverage %, Mermaid precedence |

**Cross-examination.** CashFlux's *transparency* tooling (coverage %, shadow warnings,
precedence flowchart) is unique — nobody else shows rule-system health. But on raw capability
the ranking is AC > FF ≥ LM ≥ MO > CF: we lack regex, multi-field conditions in one rule,
stages, split/notify actions, learned rules, and the live match-*list* editor. WF7 should be
scoped as: adopt Actual's engine semantics + keep CashFlux's transparency layer + Firefly's
search-rule unification (IV.5). That combination would be the best rules system in the market.

## IV.8 Reconciliation & balance assertion

| | Flow | Lock state | Adjustment txn | Frequency nudge |
|---|---|---|---|---|
| YN | "Is this your current balance?" → auto-locates discrepancy → creates adjustment | **locked** reconciled txns | auto-created | prompted cadence |
| AC | reconcile mode with target balance, running delta | cleared vs reconciled distinct | auto adjustment | — |
| **CF** | per-account reconcile-to-statement: uncleared list + mark-cleared | **no lock** — cleared only | set-balance with delta preview (separate flow) | stale badges (days-based, not reconcile-based) |

**Cross-examination.** CashFlux has the pieces (reconcile inline, set-balance, stale badges) but
not the *state machine*: cleared ≠ reconciled ≠ locked, and freshness (WF4) counts days since
update, not days since *reconciliation*. YNAB's flow is 2 inputs + auto-adjustment; ours is a
list-marking session. **Deltas:** reconciled-as-state (locks rows, feeds WF4 confidence);
one-question reconcile flow ("statement balance?" → propose adjustment); reconcile recency as a
freshness input.

## IV.9 Scheduled & recurring mechanics

| | Frequency vocabulary | Variable amounts | Occurrence states | Auto-enter vs remind | Detection |
|---|---|---|---|---|---|
| YN | never/daily/weekly/everyOtherWeek/**twiceAMonth**/every4Weeks/monthly/everyOtherMonth/every3Months (API enum) | flexible targets instead | — | auto-enter N days ahead | — |
| FF | cron-grade: "**last Friday of month**", "every 3 weeks" | — | fired/pending | auto-create | importer patterns |
| MO | learned from history | **estimated for variable bills** | **paid / paid-different-amount / missed** | remind + calendar confirm | strong |
| SI | learned + manual | estimates | in Spending Plan | reminders + projected-balance effects | strong |
| AC | schedules with templates; **schedule preview shows splits** | tolerance-based matching to real txns | upcoming/missed/paid via matching | post automatically option | matching engine links real txn to schedule |
| **CF** | daily?/weekly/monthly/quarterly/yearly + first-due (store-order slugs) | fixed amounts (price-*change* detection only) | due/overdue/paid-mark | autopost + autopay badge + "Post due" | detector w/ sensitivity prefs |

**Cross-examination.** Two vocabulary gaps: **semimonthly/twice-a-month** (YN enum, LM beta —
paycheck reality for huge US cohort) and **positional patterns** ("last Friday") (FF). One
mechanic gap that's bigger than both: **Actual's schedule↔transaction matching** — a schedule
isn't just a generator, it's a *matcher* that recognizes the real imported txn (amount within
tolerance, date within window) and marks the occurrence satisfied. That's the machinery behind
Monarch's tri-state calendar and the prerequisite for honest missed-bill detection (WF12).
CashFlux's autopost generates txns but nothing reconciles generated-vs-actual when both exist
(double-count risk documented in RH-series). **Deltas:** semimonthly + positional cadences;
schedule-matching engine (expected occurrence ↔ real txn linking, tolerance bands); tri-state
occurrence status (Part I A7 reaffirmed, now with the mechanism named).

## IV.10 Budget math edge cases

| | Overspend handling | Negative/credit months | Rollover sign | CC float handling | Month epoch |
|---|---|---|---|---|---|
| YN | overspending turns category red, must be covered — cash vs credit overspend *distinguished*; **credit-card payment categories auto-move budgeted cash** | yes | envelope carries positive only (overspend resets unless covered) | **the** reference implementation | month rolls at calendar month |
| MO | flex bucket absorbs (IV. Area 4) | rollover ± at category level | signed | — | month |
| CP | cumulative signed rollover | signed | **signed, cumulative** | — | **user-chosen epoch ("first month with rollover")** |
| AC | envelope: cover-from-category flows; hold-for-next-month | yes | positive carry; overspend must be covered | manual convention | month |
| **CF** | over-budget banner + pills; cover-from-another; PeriodBoosts top-ups | (?) | rollover toggle (sign semantics undocumented) | **absent** | fixed |

**Cross-examination.** The YNAB credit-card mechanic deserves its own paragraph because no
other comp does it and CashFlux *has the data to*: when you budget cash for Groceries and swipe
a credit card, YNAB silently moves that budgeted cash into the card's *payment category* — so
"money available to pay the card" is always true cash, and float never lies. CashFlux tracks
card liabilities and budgets separately; nothing connects "spent on card" to "reserved to pay
card." With our ledger + liability model this is buildable and would be a genuine
YNAB-parity claim no aggregator matches. **Deltas:** define signed-rollover semantics + epoch
(CP); cash-vs-credit overspend distinction; credit-card payment reservation (new, significant —
candidate ticket).

## IV.11 Goals fine mechanics

| | Funding source | Progress driver | Spend-from-goal | States | Plan integration |
|---|---|---|---|---|---|
| MO v2 | **designated real accounts (multi)** | **balance movement** | — | on/off track | in budget as contributions |
| CP | balance allocation slider or txn association | allocations | **yes — blue-bar budget bump, "Update Budgets on Spend" toggle, reactivation** | Active / Ready-to-Spend | budget-integrated |
| SI | contribution amount | contributions | release funds back | — | **subtracted from Spending Plan like a bill; Available Balance = bank − goal set-asides** |
| RM | Smart Savings: **auto-transfers sized by checking balance every 1–3 days**, pause, auto-pause at target | real money movement | withdraw | active/paused/complete | — |
| **CF** | linked account (one) + contribute form (optional post-to-ledger) | contributions | — | active/achieved | monthly-needed shown; **not subtracted from STS** |

**Cross-examination.** Simplifi's *Available Balance* mechanic is the cleanest expression of
what CashFlux's safe-to-spend should do with goals: bank balance minus goal set-asides, shown
side-by-side. Rocket's Smart Savings is the automated version (balance-aware micro-transfers) —
CashFlux's surplus-sweep/round-up workflows are the local-first equivalent but aren't connected
to goals (they move money between accounts, not into goal progress). **Deltas:** STS subtraction
(reaffirmed A5); goal-aware available-balance display on accounts; wire savings workflows →
goal contributions; balance-derived progress (A5).

## IV.12 Alerts & notification channels

| | In-app | Push | Email | Per-signal config | Assignable |
|---|---|---|---|---|---|
| MO | yes | yes | yes | **Settings→Notifications: per-type toggles per channel** | review-assignment |
| SI | yes | yes | yes | watchlist thresholds at 50/75/80/90% | — |
| RM | yes | yes | yes | bill-due, low-balance | — |
| PG ✋ | yes | — | **bill-due emails (opt-in)** | per-bill | — |
| **CF** | notification center (severity, snooze 1d, catch-up banner) + R28 configurable alerts | **no push** (PWA notification API unused?) | **no email (local)** | per-alert config exists (R28) | — |

**Cross-examination.** CashFlux's in-app center is competitive; the *channel* story is the gap
(FB1/PS3 territory): no push even though it's a PWA (service-worker Notification API works
offline-scheduled on desktop — verify platform limits), no email without Cloud. Simplifi's
threshold ladder (alert at 50/75/90%) is a config pattern our watchlists (WF8) should adopt.
**Deltas:** PWA push via SW (research spike — the one channel local-first CAN do); threshold
ladders on watchlists; snooze durations beyond 1 day.

## IV.13 Data I/O: import, export, API

| | Import formats | Bank feeds | Export | Public API | Automation hooks |
|---|---|---|---|---|---|
| YN | file-based import, migration importers | direct + Apple Card | CSV | **Yes — famous public REST API, SDKs, community ecosystem ("Works with YNAB")** | API |
| MO | CSV (+tags), Mint migration | Plaid/MX/Finicity | CSV | unofficial only (community MCP/scrapers) | extension |
| LM | CSV | Plaid | **CSV-first philosophy** | **Yes — lunchmoney.dev, dev community directory** | API triggers/actions |
| AC | QIF/OFX/QFX/CSV, **YNAB4/nYNAB importers** | SimpleFIN/GoCardless | full export | **JS API on local file** | community daemons |
| FF | CSV importer (separate container), **SFTP/email fetch on cron** | via importer (GoCardless etc.) | CSV/JSON, full | **full REST API + webhooks** | the ceiling |
| **CF** | CSV+mapping profiles, OFX/statement paste, receipt/statement vision | — (by design) | per-dataset CSV, JSON full | **none** | workflows (internal only) |

**Cross-examination.** The starkest single table in this document. Every power-user comp has an
API; the two OSS local-first peers have *both* API and headless import; CashFlux — the app whose
audience is literally "found it on GitHub" — has neither. YNAB's API is the model for ecosystem
gravity ("Works with YNAB" splitwise-sync, Raycast extensions, MCP servers *built by users*);
Firefly's email-fetch importer is the model for freshness automation. **Deltas (reaffirming
A12 + new):** local HTTP API (localhost-only, token) over the pure store; CLI (`cashflux import
file.csv --account=X`); watch-folder; MCP server (the gwc-MCP pattern already exists in-repo —
a *product* MCP server would make CashFlux the first budget app with native agent tooling as a
feature — genuine first-mover slot, nobody in the roster ships one officially).

## IV.14 Multi-currency fine points

| | Per-account currency | Home-currency rollup | Rate source | Per-txn override |
|---|---|---|---|---|
| LM | yes | **"every dollar, euro and yen" rolled to home currency** | automatic | yes |
| FF | yes | yes | manual/automatic | yes |
| PSm | multi-currency accounts | yes | feeds | — |
| **CF** | yes | yes, via FX table | **manual only** + missing-rate alerts | (?) |

**Delta:** scheduled rate refresh is a Cloud candidate; local option: rate CSV paste (pairs
with price CSV, A9). Missing-rate alert already best-in-class honesty.

## IV.15 Attachments & documents

| | Receipt attach | OCR/extract | Statement store | Search by has-attachment |
|---|---|---|---|---|
| MO | yes (attachments on txns) | receipt scanning (Dec 2025) | — | **filter: has attachments** |
| PG ✋ | paystub scan | AI extract | — | — |
| **CF** | receipts on txns + artifacts store + vision extract | **yes (vision)** | artifacts hold images/CSVs | **no has-attachment filter** |

**Delta:** has-attachment/has-note/has-split filters (MO parity, trivial); account-level
statement attachments (A3).

## IV.16 Feature-by-feature scoreboard

Verdict roll-up at mechanic level (win = best-in-roster, par = competitive, lose = behind):

| Mechanic | CF verdict | Beat by | Ticket |
|---|---|---|---|
| Entry button placement | **win** | — | — |
| Payee-memory prefill | lose | YN | new |
| Keyboard grammar | lose | YN | new |
| % / rule / temporal splits, grouping | lose | MO/CP/LM | WF7+new |
| Transfer pairing on import | lose | CP/AC | WF11 |
| Tag analysis dimension | lose | LM/PSm | WF8 |
| Filter breadth | par-win | FF (syntax) | WF16 |
| Saved searches → monitor/rule handoff | lose | PSm/FF | WF7/WF8 |
| Review dispatch cost | lose | CP | WF1/FB2 |
| Review assignment | absent (uniquely buildable) | MO | new |
| Bulk-undo snapshot | **win** | — | advertise |
| Rules transparency (coverage/shadow/precedence) | **win** | — | keep in WF7 |
| Rules capability | lose | AC/FF/LM | WF7 |
| Reconcile state machine | lose | YN/AC | new |
| Cadence vocabulary (semimonthly/positional) | lose | YN/FF | WF17 |
| Schedule↔txn matching | lose | AC/MO | WF12 |
| Rollover semantics (signed/epoch) | lose | CP | WF17 |
| CC float / payment reservation | absent | YN | new (candidate flagship) |
| Goal lifecycle + spend-from-goal | lose | CP | WF18 |
| Goal STS subtraction / available balance | lose | SI | new |
| Automated savings → goals wiring | par (engines exist, unwired) | RM | XC |
| Notification channels | lose | MO/SI | PS3+new (PWA push) |
| Threshold ladders | lose | SI | WF8 |
| API/CLI/ecosystem | **absent** | YN/LM/AC/FF | new (A12) |
| Vision extraction | **win** | — | — |
| FX honesty (missing-rate alerts) | **win** | — | — |
| Health/stress/coaching | **win** | Boldin only | A14 |
| Explainability (formula disclosure, breakdowns) | **win** | — | — |

Net: CashFlux wins on transparency, explainability, vision import, breadth-in-one-app; loses
concentrated in six mechanic clusters — **review economics, rules capability, schedule
matching, budget-math typing, goal funding mechanics, and ecosystem I/O** — all of which are
already series-anchored (WF1/WF7/WF12/WF17/WF18) plus two genuinely new flagship candidates
from this pass: **credit-card payment reservation (YNAB-parity claim)** and **product MCP
server / local API (first-mover claim)**.

---

# PART V — LONG-TAIL SWEEP: MISSING & PARTIAL FEATURE CENSUS

Added 2026-07-23 (third pass, per Cam: "scour the apps to search for missing or partial
features"). Method: swept each comp's *full help-center/docs article inventory* — the long tail
of features that never make marketing pages — and classified every find against CashFlux:
**MISSING** (absent), **PARTIAL** (exists but thinner, with the exact thinness named),
**PRESENT** (parity+, listed only when the comp's version has a twist), **CF-WIN** (we're
ahead; listed for the record). Items already covered in Parts I–IV are cross-referenced, not
repeated. This is the census of everything found; nothing withheld.

## V.1 Per-comp finds

**Monarch (help-center sweep):**
- **Zillow Zestimate auto-valuation** — home value syncs from Zillow's estimate, auto-refreshed
  **every Monday**. CashFlux: MISSING (manual valuation snapshots only). A Cloud-tier adapter
  slot; local option: none honest.
- **VinAudit vehicle valuation** — vehicle market value auto-refreshed **monthly (~27th)**.
  CashFlux: MISSING. Same adapter class as Zillow.
- **Advice section** — a questionnaire-driven advice wizard "built by actual financial
  advisors": rent-vs-buy, how to save more, which goals to prioritize. CashFlux: PARTIAL —
  /plan (FOO/Ramsey) + recommendation hub cover "next step" advice, but there is no
  *decision-wizard* class (rent-vs-buy, lease-vs-buy, new-vs-used) despite the affordability
  engine existing. Cheap to build on what-if + affordability.
- **Holdings view** — securities grouped by type/institution/account + performance graph +
  manual holdings alongside synced. CashFlux: PARTIAL (per-account cards; no cross-account
  grouped holdings view — the same security in two accounts never aggregates).
- CSV import **maps tags** on the way in. CashFlux: PARTIAL (mapping wizard has
  Date/Description/Amount/Debit/Credit; no tag/member/custom-field columns).

**Copilot (help-center sweep):**
- **Investment Benchmark** — compare portfolio performance against **any chosen holding**
  (e.g. vs SPY). CashFlux: MISSING (no benchmark concept; needs price series — Cloud or CSV).
- **Category emoji/color customization** — names, colors, AND emoji per category. CashFlux:
  PARTIAL (color swatch only; no icon/emoji — small but users notice; theme-editor-adjacent).
- **Shared budgets for couples** (within one subscription's shared space). CashFlux: blocked on
  sync (Area 11) — recorded for the census.
- **International currency: USD-only, no conversion.** CashFlux: **CF-WIN** (full multi-currency
  ledger + FX table). Worth stating in marketing comparisons.
- **Recurring created from any past transaction** ("as long as you have at least one previous
  transaction"). CashFlux: PRESENT (detected charges + one-click add) — parity.

**YNAB (support-center sweep):**
- **Payee as a first-class entity + Manage Payees** — combine/merge payees, delete, and
  **renaming rules** created from an edit ("update future transactions?"). CashFlux: MISSING
  at the entity level — payees are description strings; there is no payee list, no merge, no
  per-payee stats page (top-merchants insight computes over strings). A payee entity would
  strengthen rules (IV.7), subscriptions (payee drill), and reports simultaneously. **One of
  the two biggest finds of this sweep.**
- **Auto-Assign** — one click distributes Ready-to-Assign across categories per their targets
  (underfunded first). CashFlux: MISSING on budgets — /allocate ranks *accounts/debts/goals*
  but there's no "fund all budget targets" single action (WF17's auto-assign line reaffirmed;
  the census point is it's a *one-click* affordance, not a planner).
- **Money Moves log** — a 34-day history of every cover/move between categories, viewable from
  the plan screen (and undoable on mobile). CashFlux: PARTIAL — Activity logs entity changes,
  but budget cover-from/top-up moves are not first-class logged events with their own history
  view (→ WF-AUDIT scope note).
- **Global undo/redo** — web buttons + mobile undo of recent money movements. CashFlux:
  PARTIAL (bulk-op undo snapshot + most-recent Activity undo; no global multi-step undo/redo
  stack).
- **Focused Views** — saved lens presets over the budget screen (e.g. show only underfunded).
  CashFlux: MISSING (status pills filter-ish; not savable lenses) → merges into WF13/WF16.
- **Category colors AND icons.** CashFlux: PARTIAL (colors only — same as Copilot find).
- **"Get a month ahead" framing** — aging money into next month as an explicit practice +
  **Age of Money** metric. CashFlux: MISSING both metric and framing (Part I A1 filed
  age-of-money; the census adds the month-ahead practice as onboarding/coaching content).

**Actual (docs/release sweep):**
- **Payee merge UI** (select → merge/delete, undoable) — same finding as YNAB; reinforces the
  payee-entity gap.
- **Register hotkeys** — single-key bulk setters on selected rows: **P**=payee, **N**=notes,
  **C**=category, **M**=amount, **G**=merge selected transactions. CashFlux: MISSING (bulk bar
  is mouse-only; IV.1 keyboard-grammar gap made concrete).
- **Merge selected transactions (G)** — a lighter cousin of Lunch Money grouping; MISSING.
- **Category notes & month notes** — free-text notes on a category and on a budget month,
  API-accessible (used to carry template/goal directives). CashFlux: MISSING (no notes on
  categories, budgets, or periods — users have nowhere to record "why I set it to $400 this
  month"). Cheap, real.
- **Pending-transaction import preference** (sync-import-pending; incoming cleared-status
  preferred). CashFlux: n/a for sync but PARTIAL for imports — draft review doesn't carry a
  pending/cleared distinction from OFX.

**Lunch Money (KB/features sweep):**
- **Query tool** — ad-hoc "insightful overviews" query surface over spending. CashFlux:
  PARTIAL (formula engine computes metrics; Reports has fixed slices; no ad-hoc query UI over
  transactions — the formula engine reads aggregates, not arbitrary txn queries). Related to
  IV.5's one-grammar argument.
- **Category property: "exclude from totals"** (IV.4) — CashFlux: MISSING as a category-level
  property (per-account/per-txn exclusions don't exist either; reports include everything).
  Needed for reimbursables/pass-throughs.
- Net-worth what-if tied to expense reduction — PRESENT (our /planning trim-spending overlay).

**PocketSmith (learn-center sweep):**
- **Saved searches in the side panel + bulk actions from search results** (IV.5) — reaffirmed;
  census adds: search **saves are one click to re-run**, which is the retention mechanic.
- **Income & Expense statements** — accountant-style P&L per period. CashFlux: PARTIAL
  (Reports Overview approximates; no formal statement layout/export). Low priority.
- **Attachments on transactions** widely supported — CashFlux PRESENT (receipts) with the
  IV.15 filter caveat.

**Rocket Money (site/review sweep):**
- **FICO Score 2 via Experian with factor breakdown + improvement suggestions.** CashFlux:
  MISSING (proxy only; Area 3 manual-series delta stands). Bureau data is inherently
  cloud/partner — census records it as a known non-goal with a named workaround.
- **Unlimited custom categories as a premium gate** — pricing-model note only (CashFlux
  unlimited free; CF-WIN for comparison pages).
- **RocketBNK earned-wage access** — adjacent fintech product, out of scope; recorded so the
  census is complete.

**Firefly III (docs sweep, residual):**
- **Object groups** (group accounts/piggy banks arbitrarily) — CashFlux PARTIAL (account types
  only; no user-defined account groups for reporting).
- **Attachments on any object** (accounts, bills, piggy banks — not just txns) — CashFlux
  PARTIAL (txns + artifacts only; Area 3 account-docs delta reaffirmed).
- **Audit log per object** ("show all changes to this account") — CashFlux PARTIAL (global
  Activity timeline; no per-entity history view → WF-AUDIT scope note).

**Empower / Boldin / ProjectionLab / PiggySize:** long tails already fully captured in Parts
I–II (fee analyzer, recession simulator, guardrails, IRMAA, tax-bracket viz, paystub scan,
persona demo). No additional finds.

## V.2 Census roll-up — net-new items from this sweep

**MISSING (not previously filed anywhere):**
1. Payee entity: list, merge, per-payee stats, rename-rules integration (**flagship find #1**).
2. Auto-Assign one-click "fund all targets" on budgets.
3. Category & month notes.
4. Single-key bulk setters + merge-selected in the ledger (**concrete keyboard spec**).
5. Category/entity "exclude from totals" property.
6. Decision-advice wizards (rent-vs-buy class) on the affordability engine.
7. Zillow/VinAudit-class auto-valuation adapters (Cloud).
8. Investment benchmark-vs-holding.
9. Cross-account grouped holdings view.
10. Category icons/emoji.
11. Focused views (saved budget lenses).
12. Money-moves first-class log (budget cover/top-up history).
13. Global undo/redo stack (**flagship find #2** — local-first + SQLite makes this uniquely
    buildable as a marquee trust feature; comps only do scoped undo).
14. Import mapping for tags/member/custom-field columns.
15. Pending/cleared carried from OFX drafts.
16. User-defined account groups.
17. Per-entity audit view (filtered Activity).
18. "Month ahead" coaching content + age-of-money (metric already filed A1).

**CF-WIN for the comparison page:** full multi-currency (vs Copilot USD-only); unlimited
categories free (vs Rocket's premium gate); no-credential posture (vs all aggregators).

**Fold-into-existing:** #2→WF17 · #4→IV.1/keyboard ticket · #5→reports/watchlists ·
#11→WF13/WF16 · #12+#17→WF-AUDIT · #14/#15→import lane · #1/#3/#6/#13→new tickets via CT1.

## Sources addendum (Parts IV & V)

YNAB API docs + SDKs (frequency enum, subtransactions), support.ynab.com: keyboard-shortcuts,
reconcile, manage-payees, age-of-money, undo/redo, moving-money/money-moves, auto-assign,
focused-views, colors-and-icons · Monarch help: editing-transactions, tags, rules, review blog,
notifications, tracking-property-vehicles-valuables (Zillow weekly / VinAudit monthly),
investments-in-Monarch, Advice launch blog; Monarch-Tweaks community repo · Copilot help:
splitting (3/6/12 spread), rollovers, savings-goal spending, categories FAQ (emoji/colors),
international-currency (USD-only), investments-tab (Benchmark); roadmap.copilot.money · Actual
docs + releases: rules, filters, tags, schedules/goal-templates, payees (merge), hotkeys
(P/N/C/M/G), category-notes API, sync-import-pending, split caveats · Firefly docs + DeepWiki:
search operators, search-rule engine, recurrences, object groups, attachments, audit ·
PocketSmith Learn Center: labels, saved searches, auto-categorize, filters, statements ·
Simplifi help: savings-goals (available-balance mechanics), watchlists (threshold ladder) ·
Rocket: smart-savings setup/pause, FICO Score 2/Experian factor insights · Lunch Money KB +
features: rules, category properties (exclude-from-totals), pending, query tool; changelog.

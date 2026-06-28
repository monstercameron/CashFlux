# CashFlux — Feature Map

> Page-by-page map of every screen: navigation metadata, then a breakdown of every distinct
> **section / widget** ("blob") the page renders — what each one does and how prominent it is.
> Most pages are a list/form + a few summary or helper sections; this doc names each so you know
> exactly what lives where. Grounded in `internal/screens/*.go`, the dashboard layout
> (`internal/dashlayout`), and i18n labels (`internal/i18n`). Nav metadata is kept verbatim so
> later steps (e2e crawling, screenshotting, deep-linking) can drive straight off it.
>
> **Status:** CashFlux is alpha; maturity varies by area. This maps what's wired in, not a quality
> grade. The sections below were read from source; visually confirm rough screens with an e2e pass
> (see end). "Section/widget" = a Card, EntityListSection, summary/stat grid, banner/alert,
> add-form/modal, toolbar, chart, table, calendar, canvas, or wizard step.

## How to read this

- **Path** — the clean (non-hash) route. Deep-linkable; the SPA host rewrites unknown paths to the
  shell. For e2e, navigate from `/` then click, or use the deep-link-aware `e2e/serve.go`.
- **Group / Sub-group** — left-rail placement. Groups: `Primary`, `Tools` (sub-grouped:
  `Plan`, `Bills`, `Data`, `Build`), `System`. Off-rail routes have no group.
- **Phase** — build phase (1 = core, 2 = tools/AI). All routes are registered even if hidden.
- **Prominence** — relative size/weight of each section: full-width banner, main list/table,
  compact summary strip, modal, small stat tile, chart, calendar, canvas, etc.

---

## 1. Navigation map (rail order)

_Updated 2026-06-28 to reflect the §5 IA remap. Rail grouping matches `internal/screens/screens.go`._

| Path | Label | Group / Sub-group | Phase | One-liner |
|---|---|---|---|---|
| `/` | Dashboard | Primary | 1 | Reconfigurable bento grid of headline tiles |
| `/transactions` | Transactions | Primary | 1 | The global ledger (income/expense/transfer) |
| `/accounts` | Accounts | Primary | 1 | Asset accounts, balances, net worth |
| `/budgets` | Budgets | Primary | 1 | Spend-against-budget for the period |
| `/goals` | Goals | Primary | 1 | Savings goals + sinking funds |
| `/todo` | To-do | Primary | 1 | Budgeting task list (hierarchical) |
| `/notifications` | Notifications | Primary | 1 | Catch-up reminder feed |
| `/debt` | Debt | Tools / Plan | 2 | Owed hero + liabilities + credit + loans + payoff |
| `/investments` | Investments | Tools / Plan | 2 | Holdings, performance, allocation |
| `/allocate` | Allocate | Tools / Plan | 2 | Ranks where to put new money next |
| `/planning` | Planning | Tools / Plan | 2 | Forecast, affordability, runway, what-if |
| `/recurring` | Recurring | Tools / Plan | 2 | Scheduled flows + bills + subscriptions hub |
| `/reports` | Reports | Tools / Understand | 2 | Spending/income/net + trends for the period |
| `/networth` | Net worth | Tools / Understand | 2 | Assets, liabilities, net-worth trend |
| `/health` | Financial health | Tools / Understand | 2 | Score ring + per-factor breakdown |
| `/assistant` | Assistant | Tools / Understand | 2 | Chat, insights, and smart features hub |
| `/customize` | Formulas | Tools / Build | 2 | Formula calculator (metrics) |
| `/fields` | Custom fields | Tools / Build | 2 | Custom-field definitions for any entity |
| `/studio` | Studio | Tools / Build | 2 | Widget builder, manager, and custom pages |
| `/workflows` | Workflows | Tools / Build | 2 | Automations (trigger → condition → actions) |
| `/household` | Household | Tools / Data | 2 | Members, shared expenses, per-person views |
| `/categories` | Categories | Tools / Data | 1 | Income/expense category tree |
| `/rules` | Rules | Tools / Data | 2 | Keyword auto-categorization rules |
| `/artifacts` | Artifacts | Tools / Data | 2 | Stored images & CSV datasets |
| `/activity` | Activity | Tools / Data | 2 | Change/history timeline |
| `/appearance` | Appearance | System | 1 | Theme, accent, fonts, density, banner |
| `/help` | Help | System | 1 | Setup checklist, topics, shortcuts |
| `/about` | About | System | 1 | Identity, privacy, AI, version |
| `/admin` | Admin | System (AdminOnly) | 2 | Operator console (needs backend) |
| `/setup` | Setup | System | 1 | 4-step onboarding wizard |
| `/credit` | Credit health | _off-rail / deep-link_ | 2 | Local credit-health proxy — embedded in /debt |
| `/loans` | Loans | _off-rail / deep-link_ | 2 | Per-loan amortization — embedded in /debt |
| `/bills` | Bills | _off-rail / deep-link_ | 2 | Upcoming payments + calendar — tab in /recurring |
| `/subscriptions` | Subscriptions | _off-rail / deep-link_ | 2 | Auto-detected recurring charges — tab in /recurring |
| `/insights` | Insights | _off-rail / deep-link_ | 2 | AI highlights — tab in /assistant |
| `/smart` | Smart | _off-rail / deep-link_ | 2 | Smart/AI feature suite — tab in /assistant |
| `/members` | Members | _off-rail / deep-link_ | 1 | Member list + roles — tab in /household |
| `/split` | Split a bill | _off-rail / deep-link_ | 2 | Shared-expense calculator — tab in /household |
| `/widget-builder` | Widget builder | _off-rail / deep-link_ | 2 | Node-graph editor — tab in /studio |
| `/widget-manager` | Widget manager | _off-rail / deep-link_ | 2 | Dashboard layout manager — tab in /studio |
| `/documents` | Documents | _off-rail / deep-link_ | 2 | Import CSV / receipts (vision AI) |
| `/duplicates` | Duplicates | _off-rail / deep-link_ | 2 | Review & remove double entries |
| `/plans` | Plans | _off-rail_ | 1 | Free-vs-Cloud pricing comparison |
| `/p/:slug` | Custom pages | _pattern route_ | 2 | User-built pages of widgets |

---

## 2. Dashboard widgets (`/`)

The dashboard is the only screen with explicit per-widget sizing. The grid is **4 columns wide**;
tiles span **1–4 cols × 1–3 rows** (`dashMaxColSpan=4`, `dashMaxRowSpan=3`; cell 152px, gap 10px).
Sizes are the built-in defaults (`dashlayout.DefaultItems`); every tile can be resized, hidden, or
reordered via **Widget manager**, and the arrangement persists in `localStorage`. Layout modes:
`custom` (drag), `auto-default` (canonical order), `auto-importance` (by tile importance).

| Tile ID | Name | Default size (C×R) | What it shows |
|---|---|---|---|
| `attention` | Needs attention | **4 × 1** (full-width banner) | Smart nudges: stale balances, spending alerts |
| `kpi-networth` | Net worth | 1 × 1 | KPI: total net worth |
| `kpi-income` | Income | 1 × 1 | KPI: period income |
| `kpi-spending` | Spending | 1 × 1 | KPI: period spending |
| `kpi-liabilities` | Liabilities | 1 × 1 | KPI: total owed |
| `kpi-assets` | Assets | 1 × 1 | KPI: total assets |
| `kpi-safetospend` | Safe to spend | 1 × 1 | KPI: discretionary headroom |
| `recent` | Recent | **2 × 2** (largest) | Recent transactions list |
| `budgets` | Budgets | **1 × 2** (tall) | Budget health/pace |
| `trend` | Net worth trend | **1 × 2** (tall) | Net-worth trend chart |
| `goals` | Goals | 1 × 1 | Goal progress |
| `todo` | To-do | 1 × 1 | Open tasks |
| `accounts` | Accounts | 2 × 1 (wide) | Account balances |
| `cashflow` | Cash flow | 2 × 1 (wide) | Cash-flow chart |
| `bills` | Upcoming bills | 2 × 1 (wide) | Bills due soon |
| `savings` | Savings rate | 2 × 1 (wide) | Savings-rate gauge |
| `health` | Health score | 2 × 1 (wide) | Financial-health summary |
| `breakdown` | Breakdown | 2 × 1 (wide) | Spending-by-category |
| `freshness` | Freshness | 2–4 × 1 (wide) | Stale-balance nudge → make a task |
| `highlight` | Spending highlight | 2 × 1 (wide) | Notable spending callout |
| `smart-digest` | Smart digest | 2 × 1 (wide) | AI/smart digest strip |

---

## 3. Per-page section breakdown

Every section below names a distinct on-page blob, what it does, and its prominence. Sections are
listed in render order. Conditional sections note when they appear.

### Primary

#### `/accounts` — Accounts
- **Net-worth summary strip** — Three stat tiles: hero net worth (with month-to-date ↑/↓ delta), total assets, total liabilities. _Compact full-width summary strip._
- **Missing-FX-rate alert** _(conditional)_ — `role="alert"` naming currencies with no rate and how many accounts are excluded from the total. _Compact inline alert._
- **Manage exchange rates** _(conditional)_ — Ghost button opening Settings FX panel. _Small action button._
- **Mark-all-updated** _(conditional, stale > 0)_ — "Update N balances" — stamps `BalanceAsOf=now` on all stale accounts at once. _Compact action button._
- **Transfer money / page-level transfer form** _(≥2 accounts)_ — Button reveals an inline From/To/amount/date/description transfer form. _Full-width inline form._
- **Welcome / first-run empty state** _(no accounts)_ — Heading + "Load sample data" CTA, replaces the lists. _Full-width card._
- **Assets section** — List of asset accounts (balance-desc). Each row: type icon, name, stale/smart badges, current/cleared balance, Transactions, Edit, ⋯ overflow (Set balance, Reconcile, Transfer, Mark updated, Archive), Delete. _Main full-width list._
- **Liabilities section** — Same row structure for liabilities; calm empty state with no add-CTA (intentional — no nudge to add debt). _Main full-width list._
- **Archived accounts section** _(conditional)_ — Collapsible; rows support restore/delete. _Secondary list._
- **Add account form** (modal) — Name, type, owner, currency (hidden for single-currency), opening balance, contextual liability fields (limit/APR/min/due-day/lender) or asset advanced fields (return/liquidity/stability/lock-until/custom). _Modal overlay._
- **Per-row inline editors** — Set-balance (delta preview), Reconcile-to-statement (uncleared list + mark-cleared), per-row Transfer, Edit. _Full-width inline panels replacing the row._
- **Valuation history panel** _(investment-type rows, ≥2 snapshots)_ — Last 6 date+value snapshots. _Compact read-only sub-panel._

#### `/transactions` — Transactions
- **Transactions list section** — Full-width wrapper for everything below.
- **Receipt preview overlay** _(on attachment click)_ — Modal showing the attached image (or "missing"). _Full-screen overlay dialog._
- **Filter toolbar** — Search box + "Filters" FlipPanel popover (account, category, member, tag, date range, amount range, cleared, custom field), removable active-filter chips, Clear, Export CSV. _Compact full-width toolbar._
- **Bulk action bar** _(rows selected)_ — Count + recategorize picker, Apply category, Mark cleared/uncleared, Export selected, Delete selected, Clear selection. _Compact contextual bar._
- **Bulk undo banner** _(after bulk op)_ — Describes the last op + Undo (restores from snapshot). _Compact bar._
- **Summary + select-all bar** _(non-empty set)_ — Match count + net total + uncleared note + "Select all". _Compact info strip._
- **Duplicate notice bar** _(dupes found)_ — Count + "Select duplicates" one-click selection. _Compact bar._
- **Transactions data table** — Paginated, sortable (Date, Amount, Description, Category, Account, Tags [hidden if none], Cleared ✓, Actions). Each row: select, inline-edit, duplicate, delete, receipt attach/view, split editor, create-rule, toggle-cleared. _Main full-width table with pagination._
- **Empty / no-match states** — EmptyStateCTA ("Add your first transaction" + import) when empty; "no match" paragraph when filters exclude all.

#### `/budgets` — Budgets
- **Summary stat grid** _(≥1 budget)_ — Spent / Budgeted / Left (safe-to-spend, with explainer tooltip). _Compact summary strip._
- **Budgets list section** — Header action: methodology picker (Simple / Zero-Based / Envelope), 50/30/20 starter button, Add budget.
- **Custom-range hint** _(conditional)_ — Notes a top-bar custom date range only changes the view window, not each budget's own period. _Compact note._
- **Income / assign banner** — Methodology-dependent one-liner: Simple (income·budgeted·unbudgeted), Zero-Based ("To assign: $X"), Envelope (rollover note). _Compact banner._
- **Sinking-fund set-aside line** _(conditional)_ — Total monthly commitment across active sinking funds. _Compact note._
- **Over-budget alert banner** _(over > 0)_ — ⚠ count + total overspend. _Full-width danger banner._
- **Status pill row** _(conditional)_ — "N over" / "N near limit" colored count pills. _Compact pill strip._
- **Budget rows** — Expandable per-budget: progress bar, spent/limit/remaining, pace warning, rollover/effective-cap/prorated notes, envelope balance, method-override badge; overflow (Edit, Cover-from-another, Top-up, View transactions, Delete). Health-ordered. _Main full-width list._
- **Add budget form** (modal) — Category, name, limit, period, owner, method override, rollover toggle. _Modal overlay._

#### `/goals` — Goals
- **Summary stat grid** _(≥1 goal)_ — Saved so far / Total target / Overall progress % (hero, explainer tooltip). _Compact summary strip._
- **Sinking funds section** _(conditional)_ — Card of active sinking-fund goals (alpha), each row showing monthly set-aside + linked category. _Full-width secondary card._
- **Goals list section** — Active goals sorted by urgency. Each row: name, pace badge (Final Stretch/Overdue/Due Soon/On Track), progress bar, saved/target, monthly-needed, linked account, contribute form (+post-to-ledger toggle), inline edit, archive/delete. Header: smart-insights + Add goal. _Main full-width list._
- **Empty state** _(no goals)_ — "Add your first goal" CTA + smart empty-state nudge. _Full-width empty state._
- **Achieved goals section** _(conditional)_ — Collapsible card of archived goals (alpha). _Full-width collapsible card._
- **Add goal form** (modal) — Name, target, date, linked account, is-sinking-fund (reveals category + set-aside), owner. _Modal overlay._

#### `/todo` — To-do
- **Task list section** — Header: priority filter (All/High/Medium/Low), Hide-done toggle, Add task.
- **Summary stats bar** _(any tasks)_ — Open / overdue / done counts. _Compact info strip._
- **Error message** _(conditional)_ — Inline `role="alert"`.
- **Task list (hierarchical)** — Parent→child tree. Each row: priority badge, title, due date (overdue/due-today states), notes tooltip, recurrence badge, entity deep-link; actions checkbox (recurring auto-spawns next), Add subtask, Edit, Delete. _Main full-width hierarchical list._
- **Inline edit form** _(per row)_ — Title, priority, due, notes, repeat cadence, link-to type + entity. _Full-width inline form._
- **Hidden-done note** _(conditional)_ — "N completed tasks are hidden". _Compact note._

#### `/notifications` — Notifications
- **Notification center section** — Header: title + "Clear all".
- **Empty state** _(no items)_ — "Nothing here". _Full-width empty state._
- **Catch-up banner** _(new since last visit)_ — "Since your last visit" + count, `aria-live`. _Compact banner._
- **Notification feed list** — Priority-sorted (Critical→Warning→Info). Each row: title, body, severity pill, date, controls (read/unread, snooze 1 day, dismiss). Opening auto-marks read + stamps last-seen. _Main full-width feed._

### Tools → Plan

#### `/planning` — Planning  _(also `/debt` and `/recurring`, which render the same screen)_
- **12-month net-worth forecast** — Projects net worth from 3-month trailing average; hero projected-NW + avg-monthly stat, dip-below-zero warning, line chart (calendar X-axis), "trim spending by $X" what-if overlay, compare-with-saved-plan dropdown. _Full-width section + line chart._
- **Can I afford it?** — Affordability within N months keeping a cash reserve (liquid balance + safe-to-spend); projected balance, available amount, yes/no verdict, months-needed. _Full-width form + 2-stat grid._
- **Cash runway** — 60-day liquid-balance projection vs scheduled recurring; liquid start, 60-day low, optional payday balance, breach warning + transfer suggestion, daily-balance line chart. _Full-width section + chart._
- **Bills & recurring** — Add-form (label/amount/cadence/account/category/first-due/autopost/autopay), auto-detected charges sub-section ("N found" + one-click add), monthly-total note, recurring rows (inline-edit), "Post due". _Full-width main list._ (This is the `/recurring` surface.)
- **What-if plans** — Horizon-based balance projections; each plan: name, horizon, optional start (prefill from account), monthly amount, one-time event; saved rows show sparkline + end balance + runway. _Full-width add-form + list._
- **Debt strategy** — Snowball vs avalanche across liabilities; debt table (name/balance/APR/min), 2-stat months grid, interest comparison, payoff order, interest-saved rec, extra-payment input, per-liability include/exclude + inline APR/min editors, dual-series burn-down chart. _Full-width section._
- **Payoff calculator** — Manual single-debt: balance, APR, monthly payment, extra payment. _Inline form._
- **Payoff projection** — Result of the calculator: 4-stat grid (months-to-zero, debt-free date, total interest, total paid) + extra-payment impact note. _Compact result panel._

#### `/allocate` — Allocate
- **Income nudge banner** _(positive income, not dismissed)_ — One-click "Use this month's income ($X)" pre-fill + dismiss. _Full-width banner._
- **Profile config** — Mode toggle (score-weighted vs fill-to-target), profile select (Balanced/Returns/Safety/Debt/Goals + saved), amount/reserve/max-per-destination inputs, Advanced panel with 5 weight sliders + save-profile form. _Full-width form section._
- **Suggestion list** — Ranked destinations; each row: rank badge, name, score %, score bar, breakdown sub-line (returns/stability/liquidity + debt/goal notes), suggested split amount, Exclude. Excluded items restorable below. _Main full-width ranked list._
- **AI explain card** — Keyless plain-English "why this order" one-liner + "Ask AI to explain" (OpenAI narrative, loading/error states). _Compact card._
- **Apply allocation** _(amount entered)_ — Apply opens a confirm panel listing each action (earmark / goal contribution / debt paydown), Confirm/Cancel; result message + Undo after applying. _Full-width section + confirm sub-panel._

#### `/reports` — Reports
- **Hero zone** — Period header: Net (hero + delta chip), Income, Spending; secondary Net Worth (+monthly change chip), Savings Rate, Cash Runway, No-spend days. _Above-fold full-width banner._
- **Spending trend chip** — "Spending up/down N%" vs prior period. _Compact text line._
- **Export / Save-as-PDF controls** — Disclosure with per-dataset CSV downloads (by category, income by source, top payees, biggest expenses, by member, tax) + Save as PDF. _Compact toolbar._
- **Spending stats line** — N transactions, average, median. _Compact text line._
- **Heads-up anomaly card** _(conditional)_ — Up to 3 categories above their recent norm. _Compact urgency-bordered card._
- **Report type selector** — 4-tab segmented (Overview / Categories / Net Worth / Advanced); gates everything below. _Full-width tab bar._
- **Overview tab** — Money Flow (Mermaid Sankey, full-width); Top Payees (bar + share-bar list); Biggest Expenses (bar + list); Biggest Deposits (list); Income by Source (sentence + bar + donut + list); Spending by Member (list, ≥2 members). _Grid of chart/list cards._
- **Categories tab** — Spending by Category: narrative + peak-weekday line + paired bar (top 8) + donut (top 5 + Other) + drill-through list with deltas, YoY toggle, rollup toggle.
- **Net Worth tab** — Net Worth (stat grid + 6-month area chart + View Accounts); Cash Flow Trend (sparkline + takeaway); Savings Rate Trend (sparkline + takeaway).
- **Advanced tab** _(collapsible)_ — Spending by Custom Field (selector + ranked list + CSV); Deductible Totals.

#### `/networth` — Net worth
- _(Rendered by `NetWorth`.)_ Assets / liabilities / net-worth stat grid + net-worth trend chart with calendar labels and an accounts link. _Full-width summary + trend chart._ (See also the Reports → Net Worth tab, which reuses the same building blocks.)

#### `/health` — Financial health
- **Score ring hero** — 150px SVG gauge (red→green hue), score + band label, since-last-month delta chip, negative-cash-flow warning. _Full-width dominant card._
- **What goes into your score** — Per-factor list: label + value, scored progress bar, contribution %, target; "Not applicable" where relevant. _Full-width main card._
- **Where to focus next** _(conditional)_ — Prioritized action steps: factor, plain-English action, target. _Compact step card._
- **Privacy note** — "Calculated on your device…". _Compact footer._

#### `/credit` — Credit health
- **Proxy score ring hero** — 150px gauge (0–100), band label, aggregate utilization %. _Full-width dominant card._
- **Per-card utilization breakdown** — Per credit card: name, utilization % + bar, balance/limit, band, "Pay $X to reach 30%" nudge, utilization trend bars (≤8 snapshots), inline credit-limit editor. _Full-width main list._
- **Missing-limit note** _(conditional)_ — When a card has no limit entered. _Compact advisory card._
- **Disclaimer** — Privacy/disclaimer caption. _Compact footer._

#### `/loans` — Loans
- _(One card per installment loan/mortgage.)_ Each card: **header** (name, type badge, APR, principal); **term input** (repayment months; 360 mortgage / 60 else); **amortization summary** (2×2 stat grid: monthly payment, total interest, total paid, payoff date); **extra-payment simulator** (input → months/interest saved, new date, payments remaining). _Full-width card list._

#### `/investments` — Investments
- **Overall portfolio summary** _(≥1 holding)_ — 2×2 stat grid: total value, cost, gain/loss, return %. _Full-width top card._
- _(Then one card per investment account.)_ Each card: **header** (name, type badge, total value); **performance summary** (2×2 stat grid, C220); **asset-class allocation bars** (per-class label + % + bar, C221); **holdings list** (name/ticker, class tag, shares, value, cost, gain/loss, return %, Delete; C219); **add holding form** (ticker, name, shares, cost basis, current price, asset class). _Full-width card list._

#### `/insights` — Insights
- **No-data empty state** _(no accounts/txns)_ — Guided CTA. _Full-width card._
- **Ask-a-question shortcut** — Top button that focuses the chat input. _Compact affordance._
- **Spending highlights** — Category spend-anomaly cards, drill-through to filtered `/transactions`. _Compact card list._
- **Top merchants** — Top payees (90 days), drill-through with text filter. _Compact ranked card._
- **Monthly spending chart** — 6-month expense area sparkline. _Compact chart._
- **Anomaly highlights** — Four detectors (duplicate txns, spikes, missing charges, balance anomalies), no Smart gate. _Compact card list._
- **Pinned insights** _(conditional)_ — Saved snippets, newest first, with delete. _Row list._
- **Chat / Ask section** — The main interactive surface (`#ask`): conversation switcher (New chat, Advanced, Edit prompt, per-conversation pills), backend/OpenAI toggle, intro hint or example Q→A, scrollable thread (user/assistant bubbles with Copy/Pin/Retry/Delete + token/cost), thinking indicator, tool-approval card, suggested-question chips, composer (input + Send/Cancel), error line. _Full-width primary section._
- **System prompt editor** _(when open)_ — FlipPanel (640×520) to edit AI persona + Reset. _Modal overlay._

#### `/smart` — Smart
- **Tab bar** — Two tabs: Insights / Manage. _Full-width tab bar._
- **Insights tab** — **Free insights** (paginated severity-sorted cards: dot, title, amount, reason, one-tap action [navigate/create task/goal/recurring/cancel sub/automate], Dismiss); **AI features** (per-feature run-control with trigger + result, or "configure a provider" note); **Digest** (description + on/off + cadence picker). _Branded cards._
- **Manage tab** — **Manage catalog**: opt-in features grouped by page; global controls (density dial Minimal/Standard/Full, Enable Free Only, Enable All, Disable All); each row: name, Free/AI cost badge (+est. cost), summary, on/off toggle; AI rows add cadence + Mute. _Branded page-grouped card._

### Tools → Bills

#### `/subscriptions` — Subscriptions
- **Late-charges alert** _(conditional)_ — Danger card listing charges after a recorded cancellation date. _Full-width high-urgency banner._
- **Detection preferences panel** _(collapsible)_ — Min-occurrences sensitivity, account-type + expense-category filters; active-filter count badge. _Compact collapsible panel._
- **Summary stat grid** _(≥1 sub)_ — Monthly burden (hero), Annual burden, Count, Share of month's spend. _Compact strip._
- **Subscriptions list** — Select-all bar + per-sub rows (cancel checkbox, payee drill-link, cadence, next renewal, share-bar, amount, cadence badge + per-month avg, Remind/Cancel/How-to-cancel/Ignore); selected-savings bar + bulk Cancel. Header: smart button + CSV. _Main list._
- **Price changes** _(conditional)_ — Net summary + per-sub direction/delta/%/new amount. _Compact secondary list._
- **Renewing soon** _(conditional)_ — Subs renewing within 7 days (full rows, review badge suppressed). _Compact secondary list._
- **Ignored** _(conditional)_ — Marked "not a subscription" with Undo. _Compact secondary list._
- **Cross-link to Insights** — Inline button to `/insights`. _Compact footer note._

#### `/bills` — Bills
- **Summary stat grid** _(upcoming bills exist)_ — Total due (hero, smart tooltip), Annual cost, Count, Next due. _Compact strip._
- **Bills list** — Side-by-side with calendar (≥1024px). Each row: name, due date + days-until (urgency-colored), autopay badge, amount, Mark paid / Remind; "Show next 90 days / Show all" toggle + CSV. _Main list._
- **Bills calendar** — 7×N month grid; days with bills get a colored dot (danger/warn/soon) + count badge; prev/next-month + "This month" controls. _Full-width calendar widget._

#### `/split` — Split a bill
- **Split calculator** — Amount, What for, Payer (member select), "By weight" toggle (equal vs proportional), error line. _Main form card._
- **Members picker** — Select-all/Clear; per-member row (include toggle, optional weight, computed share); split-summary line. _Main list._
- **Payer prompt** _(conditional)_ — "Pick who paid…". _Inline hint._
- **This split** _(payer+members+amount set)_ — Who-owes-whom rows + Mermaid debtor→payer diagram + Save split + CSV. _Conditional full-width card._
- **Running balance** — Per-member net rows (owed/owes) + "simplest way to square up" minimal-payment rows each with a "Record" button; empty state otherwise. _Persistent main card._
- _(Note: `SplitEditor` in `split_editor.go` is the inline per-transaction split widget embedded in Transactions — not rendered on this page.)_

### Tools → Data

#### `/duplicates` — Duplicates
- **Summary banner** _(dupes exist)_ — "N likely duplicates across M groups" + hint; empty-state card otherwise. _Full-width banner._
- **Duplicate group cards** — One per group: title + count badge + Keep note + Merge group; group header (payee/date/amount); per-txn rows (first = "Keep"; others show account/date + "Delete duplicate"). Delete + merge guarded by ConfirmModal. _Main list of cards._

#### `/documents` — Documents
- **CSV import** — Privacy note, "How to get your bank's CSV" disclosure, file picker, account selector, paste textarea, Import; pre-import duplicate-preview warning + "Import anyway". _Main import form._
- **Statement paste & AI extraction** — Parse / Extract-with-AI buttons, textarea (delimited or OFX), cadence-reminder button (creates monthly to-do). _Main import form._
- **AI section separator** — Labelled `role="separator"`. _Visual divider._
- **Image import** — Choose image / Read with AI, cost note, thumbnail preview, missing-key warning + Settings link, error line. _Main import form._
- **Column-mapping wizard** _(conditional)_ — Five selects (Date/Description/Amount/Debit/Credit), Apply, save-profile sub-form. _Conditional card._
- **Saved mappings** _(conditional)_ — Show/Hide; saved import profiles with Apply/Delete. _Compact card._
- **Suggest-categories toolbar** _(drafts exist)_ — "Suggest categories" (rules pass, then AI) + loading. _Compact strip._
- **Draft review** _(drafts exist)_ — Step-active highlight; duplicate-count banner, Start over, receipt-mode toggle, sticky account-selector + Import bar, editable draft rows (date/desc/amount/category; "Already imported" badge; edit/remove), footer plain/receipt import form (store + total + reconciliation remainder). _Prominent step card._
- **Spend summary** _(drafts exist)_ — Per-month row (label, count, out/in/net). _Compact summary list._
- **Import history** — Newest-first imports (filename/kind, date, status, count, account, delete). _Compact history list._

#### `/artifacts` — Artifacts
- **Upload / storage** — Upload image / Import CSV, storage-usage label (localStorage + IndexedDB), storage progress bar (warns >80% of ~10MB), quota-warning banner near limit. _Main upload card._
- **Artifacts list** — Per-artifact row: thumbnail/icon, name (inline rename), "Referenced by N transaction(s)", custom-page usage, kind+size, upload date, 3-row CSV preview (datasets); rename + delete (delete blocked when used by a custom-page widget); empty state. _Main list._

#### `/activity` — Activity
- **Activity timeline** — Subtitle, entity-type filter (All / Transactions / Accounts / Budgets / Goals / Tasks / Categories / Members), up to 50 newest-first rows (action label, type, summary, timestamp, actor You/System; Undo on the most-recent row when the undo stack is non-empty). Falls back to synthesising from live txn/task data when the audit feed is empty. _Full-width timeline list._

### Tools → Build

#### `/customize` — Customize
- **Formula calculator** — Expression input, 4 preset example buttons, live result, name + Save. _Full-width section, above fold._
- **Formula result** — Live-evaluated output (number/boolean/error). _Compact value section._
- **Available variables** — Engine variables (net_worth, assets, liabilities, income, expense, accounts, transactions, members, budgets, goals, tasks) with live values + click-to-insert. _Rows list._
- **Saved formulas** _(conditional)_ — Persisted formulas (name, expression, live result, Load, Delete). _Rows list._
- **Section divider** — `customFieldsSection` `<h3>` separator. _Divider._
- **Custom fields — Add field** — Entity-type selector (Accounts/Transactions/Budgets/Goals/Members), key/label, data-type (text/number/date/bool/select), choices (select only), required toggle, Add. _Form section._
- **Custom fields — per-entity lists** — Five always-rendered sections (one per entity type) listing defined fields (key, type, required badge, options). _Main full-width lists._

#### `/workflows` — Workflows
- **Savings automations / pay-yourself-first** — Framing text + PYF template (From/To accounts, Amount, Cadence, Save). _Main card._
- **Surplus-sweep config** (inline sub-section) — Enable toggle, From/To, buffer floor, Save.
- **Round-up savings config** (inline sub-section) — Enable toggle, From/To, granularity ($1/$5/$10), Save.
- **Create workflow** — Name, Trigger (Manual/Transaction added/Scheduled/Budget exceeded/Goal reached/Bill due), optional Cadence, Condition formula + variable pills (txn_abs/amount/payee/category), action builder + staged list, Save. _Full-width form._
- **Your workflows** _(with count)_ — Per-workflow row: name, trigger, action count, last-run result, Dry run / Run now / Enable-Disable / Edit / Delete, collapsible Mermaid flowchart. _Rows list._
- **Run history** _(conditional)_ — Last 12 applied runs (name, timestamp, effect count). _Compact section._

#### `/widget-builder` — Widget builder (VisualBuilder node-graph editor)
- **Toolbar** — "Widget builder" label, Preset dropdown (16), "My cards" dropdown, Card-name input, Save / Publish to dashboard / Delete / Undo / Redo / New, status, W/H steppers (cols 1–4, rows 1–3). _Full-width strip._
- **Node palette** — Left pane (170px) grouped Data/Transform/Logic/Display/Style/Layout/Interact; "+ Label" adds a node. _Narrow vertical pane._
- **Canvas** — Center pan/zoom dot-grid; draggable node boxes (kind + var name, input/output ports) wired by SVG beziers; zoom toolbar (fit/−/reset/+). _Main canvas._
- **Inspector** — Right pane (250px): "Select a node" prompt, or Node (kind + var), Props (type-appropriate fields), Inputs (per-port wire selectors), Actions (Set as output, Delete). _Narrow vertical panel._
- **Live preview** — Centered `wb-stage` rendering the evaluated tile at true bento proportions. _Compact preview strip._

#### `/widget-manager` — Widget manager
- **Layout controls card** — Hint, preset layout selector (DashboardLayoutControls), Show all / Hide all. _Full-width card._
- **Widgets table** — Sortable DataTable (Name / Visible / Size / Order): display name, visibility toggle, W×H steppers (on hover), ↑/↓ reorder. _Main table card._
- **Tile style editor** — Target selector ("All widgets" or one widget), color pickers (Background/Text/Border/Accent), selects (Border width/Radius/Font/Weight/Shadow), Reset, and a live preview tile. _Full-width two-column card._

### System

#### `/members` — Members
- **Reassign-before-delete panel** _(conditional)_ — When deleting a member with owned entities: count + target-member dropdown + Move-and-delete / Cancel. _Inline modal-style card._
- **Member list** — Orientation + single-device note. Each row: colored initial avatar, name, Role badge, Default chip, Make default / View transactions / Edit / Delete; inline edit (name, color, date style, default account, role); empty state + Add CTA. _Main list card._
- **Net worth by owner** _(members exist)_ — Per-member + "Shared" net-worth rows. _Compact section._
- **Spending this period** _(conditional)_ — Per-member period spend; unattributed under "Shared". _Compact section._

#### `/categories` — Categories
- **Reassign-before-delete panel** _(conditional)_ — Deleting a used category: usage count + same-kind target dropdown + Move-and-delete / Cancel. _Inline section._
- **Category map** _(categories exist)_ — Visual grid of parent chips with child sub-pills. _Above-fold grid section._
- **Expense categories** — Collapsible tree: color swatch, expand toggle, indented name, kind, txn count / drill-link, Edit/Delete; inline edit (name, type, parent, color, deductible). Header: Sort-by-usage + Add. _Main list._
- **Income categories** — Same structure for income-kind categories. _List section._

#### `/rules` — Rules
- **Your rules** — Header (Apply-to-existing, Add rule); quick-add inline form (match phrase + category + tags + live match-count); drag-hint; coverage summary (N of M covered); draggable rule list (grip, phrase, category, tags, rename-desc, match count, shadow warning, Edit/Delete); inline edit form. _Main card._
- **Suggested rules** _(conditional)_ — AI-inferred mappings (description + supporting count + Add); Show all/fewer when >5. _Section._
- **Rule order** _(>1 rule)_ — "First match wins" + Mermaid precedence flowchart (shadowing arrows). _Full-width diagram section._

#### `/appearance` — Appearance
- **Theme mode** — Segmented Dark / Light / System. _Inline row._
- **Motion** — Segmented Full / Subtle / Off + hint. _Inline row._
- **Accent color** — Four-preset SwatchPicker. _Inline row._
- **Theme editor** (embedded `ThemeEditor`) — Preset buttons; colors grid (8 tokens: app bg, card, borders, text, muted, accent, positive, negative); shape & type (radius, text size %, interface/heading fonts, upload-font); density + icon-weight segmented; dashboard banner (gradient presets, upload/remove image); live validation; Export / Import / Reset. _Full-width embedded component._

#### `/help` — Help
- **Getting set up** — Six-step checklist with live ✓/○ from real data, links to routes, completion message. _Full-width card._
- **What's new** — Tagline, privacy statement, version, four highlights, changelog link. _Full-width card._
- **Support & feedback** — Invite + Bug report / Feature request GitHub links. _Full-width card._
- **Topic cards** — Getting started, Bringing in your data, Budgets/goals/reports, The Smart layer, Keyboard shortcuts (? / Ctrl-K palette / Alt+1–9), Your privacy. _Card grid._
- **Offline footer** — "Everything here works offline…". _Compact caption._

#### `/about` — About
- **App identity** — Name, tagline, description. _Card._
- **Privacy & local-first** — Local storage, export, no-tracking. _Card._
- **Cloud sync disclosure** — Off by default, what happens when on, user control. _Card._
- **AI features** — BYO key, local key storage, when data goes to OpenAI, Settings link. _Card._
- **Version & links** — Version, changelog, source repo, license. _Card._

#### `/admin` — Admin _(AdminOnly; needs backend)_
- **Sign-in gate** _(no endpoint/token)_ — Explanation + "Open Cloud settings". _Section._
- **Access denied** _(401/403)_ — EmptyStateCTA. _Section._
- **Error / Loading states** — Error message + Retry, or 4-line skeleton. _Card._
- **Platform overview** _(ready)_ — StatGrid of 9 (Total users, Est. MRR, Active/Trialing/Past-due/Canceled subs, Total storage, Requests today, Tokens today) + day label. _Full-width card._
- **Users table** _(ready)_ — DataTable (Email / Provider / Plan / Status / Created), up to 50. _Section._

#### `/setup` — Setup wizard
- **Welcome card** — Orientation text. _Full-width card, always visible._
- **Step progress bar** — Four-step ✓/○ breadcrumb (Currency, Income, Account, Members). _Compact strip._
- **Step 0 — Currency & week-start** — Currency select, week-start, Continue. _Wizard card._
- **Step 1 — Monthly income** — Income input, Continue / Skip. _Wizard card._
- **Step 2 — First account** — Name, type, opening balance, Add / Skip. _Wizard card._
- **Step 3 — Household members** — Member list + name input + Add, Skip. _Wizard card._
- **Completion** — Success/partial message + "Go to dashboard". _Wizard card._

### Off-rail

#### `/plans` — Plans
- **Current-plan chip** — Active plan pill ("Free / local"). _Compact chip._
- **Free tier card** — Tagline, "$0", four feature bullets; no CTA. _Comparison card._
- **Cloud tier card** — Tagline, annual (primary) + monthly price, four bullets, "Start trial" → /settings + trial note. _Highlighted comparison card._
- **Trust / self-host notes** — Cancel-anytime line + self-host line. _Compact footnotes._

#### `/p/:slug` — Custom pages
- **Add-widget toolbar** — "Add widget" expands a form: type (KPI / List / Chart / Text / Image / Table), title, binding (formula for KPI, source dropdown for List, text for Text, artifact picker for Image/Table, placeholder for Chart), Add / Cancel. _Full-width toolbar._
- **Empty-state card** _(no widgets)_ — Replaces the grid. _Card._
- **Custom widget tiles** — Bento grid (4-col), one tile per widget. Header = drag handle (title, ↔/↕ resize, edit toggle, delete). View body by type: KPI (figure from formula), List (label+value rows), Chart (net-worth 6-month area), Text (Markdown), Image (artifact), Table (first 8 dataset rows). Edit body = inline form (title + binding, Save/Cancel). _Full-width bento grid; each tile spans its saved C×R._

---

## 4. Notes for the e2e / screenshot pass

Section breakdowns above were read from source; **rough/evolving** screens warrant visual
confirmation. To capture fresh screenshots:

1. Build + serve: `.\.tools\gwc.exe dev -app .\main.go -root .\web -html .\web\index.html -wasm .\web\bin\main.wasm`
   (or the deep-link-aware `e2e/serve.go` so sub-routes survive a hard refresh).
2. Load `/`, hit **Settings → "Load sample"** for realistic data before shooting.
3. Drive headless via the `gwc` MCP browser tools (`gwc_screenshot`, `gwc_click`, `gwc_dom`) or
   `node e2e/readme_shots.mjs` (writes into `docs/screenshots/`).
4. The Path column in §1 is the crawl list — iterate it to visit every screen.

**Verify visually** (size/maturity uncertain from source alone): `/smart` (tabbed, evolving),
`/planning` vs `/debt` vs `/recurring` (one shared screen — confirm what each entry emphasizes),
`/widget-builder` (some node config is phased/placeholder), `/admin` (needs backend, likely empty
locally), `/networth` (confirm it isn't just the Reports tab), and the rendered spans of the
`freshness` / `smart-digest` dashboard tiles.

---
---

# 5. Themed remapping (proposed information architecture)

> **IMPLEMENTED as of 2026-06-28.** The remap described in §5 is now live in the route registry
> (`internal/screens/screens.go`) and the rail nav (`internal/app/shell.go`). The §1 navigation
> table above reflects the updated grouping. Sections 5.1–5.7 below document the rationale and
> feature-by-feature remapping decisions that drove the implementation.

> _Original note:_ Sections 1–4 describe how the app is structured *today* (post-remap); this
> section documents the IA design rationale. Nothing in §5.1–5.7 is aspirational anymore — the
> work is done.

## 5.1 Why remap

§3 shows several pages are **multi-theme grab-bags**, and several **themes are scattered** across
pages:

- **`/planning` is a kitchen sink** — forecasting *and* recurring bills *and* debt payoff *and*
  what-if, all on one screen. Three different jobs.
- **`/debt`, `/recurring` are aliases of `/planning`** — three rail entries, one screen; the theme
  a user clicked for isn't what dominates the page.
- **"What you owe" is scattered** across `/credit`, `/loans`, the debt block in `/planning`, and the
  liabilities half of `/accounts`. There's no single home for debt (the user's "handling credit
  cards" example).
- **"Money that repeats" is scattered** across `/bills`, `/subscriptions`, and the recurring block
  in `/planning`.
- **AI is scattered** across `/insights`, `/smart`, the vision import in `/documents`, and the AI
  explainer in `/allocate`.
- **"Get transactions in / keep them clean"** lives on separate Data pages (`/documents`,
  `/duplicates`) away from `/transactions` itself.
- **Net worth** is computed and shown on `/accounts`, `/networth`, a Reports tab, and the dashboard.

**Design rule for the remap:** one page = one noun a user holds in their head. If you'd describe a
page with "and", it's two pages.

## 5.2 Proposed themed page set

Status legend: **Keep** (already single-theme) · **Narrow** (strip off-theme blocks) · **Absorb**
(pull scattered features in) · **New** (no good home today) · **Merge** (fold two near-duplicates).

| # | Theme (the one noun) | Route | Status | One-line scope |
|---|---|---|---|---|
| 1 | At-a-glance | `/` Dashboard | Keep | Tiles only; no editing surface. |
| 2 | The ledger | `/transactions` | Absorb | Record/edit money movement **+ import + dedupe**. |
| 3 | What you own (assets) | `/accounts` | Narrow | Asset accounts + balances only; liabilities leave. |
| 4 | **What you owe** | `/debt` | **New/Absorb** | Credit cards + loans + payoff + liability accounts. |
| 5 | Investments | `/investments` | Keep | Holdings, performance, allocation. |
| 6 | Net-worth trend | `/networth` | Keep | The wealth-over-time line; sources from 3+4+5. |
| 7 | Spending limits | `/budgets` | Keep | Budgets only. |
| 8 | Savings targets | `/goals` | Keep | Goals + sinking funds. |
| 9 | Where to put new money | `/allocate` | Narrow | The ranking tool; AI explainer becomes a call-out. |
| 10 | **Money that repeats** | `/recurring` | **Merge/Absorb** | Bills + calendar + subscriptions + recurring cash flows. |
| 11 | The future | `/planning` | Narrow | Forecast + runway + can-I-afford + what-if **only**. |
| 12 | The past (analysis) | `/reports` | Keep | Historical reporting + trends. |
| 13 | Financial health | `/health` | Keep | The single scorecard. |
| 14 | **The assistant (AI)** | `/assistant` | **Merge** | Insights chat + Smart hub + AI features in one home. |
| 15 | Tasks | `/todo` | Keep | To-do list. |
| 16 | Reminders | `/notifications` | Keep | The catch-up feed. |
| 17 | **Your household (people)** | `/household` | **New/Absorb** | Members + split-a-bill + per-owner views. |
| 18 | Automations | `/workflows` | Keep | Trigger → condition → action. |
| 19 | Build a metric | `/customize` | Narrow | Formula calculator **only**. |
| 20 | Your data shape | `/fields` | **New (split)** | Custom-field definitions (was half of `/customize`). |
| 21 | Build a tile/page | `/studio` | **Merge** | Widget builder + manager + custom pages **+ artifacts/assets**. |
| 22 | Category taxonomy | `/categories` | Keep | Income/expense category tree. |
| 23 | Auto-categorize | `/rules` | Keep | Keyword rules. |
| 24 | Look & feel | `/appearance` | Keep | Theming. |
| 25 | History & audit | `/activity` | Keep | Change/history timeline (already single-theme). |
| 26 | System | `/settings` `/help` `/about` `/admin` `/setup` `/plans` | Keep | Unchanged. |

That's **26 single-theme destinations** vs. ~40 routes today, mostly by *consolidating scatter*
(debt, recurring, AI, household) and *splitting one grab-bag* (`/planning`, `/customize`).
(`/documents` + `/duplicates` fold into `/transactions`; `/artifacts` folds into `/studio` — see §5.4.)

## 5.3 New / changed pages (rationale)

- **`/debt` — "What you owe" (NEW consolidation).** The user's headline example. Today debt is in
  four places. New page owns: a total-owed hero + payoff-date, the **credit-card utilization block**
  (from `/credit`), the **per-loan amortization cards** (from `/loans`), the **snowball-vs-avalanche
  strategy + payoff calculator** (from `/planning`), and the **liability accounts list** (from
  `/accounts`). One place to see and attack everything you owe. `/credit` and `/loans` become
  in-page sections (or sub-tabs) rather than separate rail items.
- **`/recurring` — "Money that repeats" (MERGE).** Fold `/bills` (list + calendar), `/subscriptions`
  (detection + renewals + price changes), and the recurring-cash-flow manager (from `/planning`)
  into one page. Sub-views: *Bills* (calendar), *Subscriptions*, *Scheduled flows*. Single theme:
  anything that recurs.
- **`/planning` — "The future" (NARROW).** After debt and recurring leave, Planning is purely
  forward-looking scenarios: 12-month forecast, cash runway, can-I-afford-it, what-if plans. Retire
  the `/debt` and `/recurring` aliases (they now point at the real consolidated pages).
- **`/assistant` — "The AI" (MERGE `/insights` + `/smart`).** One AI home: the chat/Q&A surface, the
  Free-engine insight cards, the AI-feature run controls, the manage-catalog, and the digest. The
  vision receipt-import stays physically in `/transactions` import but is labelled "powered by the
  assistant". The `/allocate` AI explainer stays inline (it's about *that* decision).
- **`/household` — "Your people" (NEW).** Members (`/members`), split-a-bill (`/split`), and the
  per-owner **net-worth-by-member** / **spending-by-member** views (today buried on `/members` and
  `/reports`). Theme: shared money and who it belongs to.
- **`/customize` → split into `/customize` (formulas) + `/fields` (custom-field definitions).**
  Today `/customize` is two unrelated power-tools welded by an `<h3>`. Formula calculator is "define
  a metric"; custom fields is "reshape my data" — different jobs, different mental models.
- **`/studio` — "Build a tile/page" (MERGE).** Widget builder (`/widget-builder`), widget manager
  (`/widget-manager`), and custom pages (`/p/:slug`) are all "construct your own dashboard surface".
  One Studio with tabs: *Build widget*, *Manage widgets*, *My pages*.
- **`/accounts` — "What you own" (NARROW).** Asset accounts only. The net-worth summary strip stays
  (it's the headline of "what you own", with liabilities shown as a single subtracted figure linking
  to `/debt`). Liability accounts, credit, loans all relocate to `/debt`.
- **`/transactions` — "The ledger" (ABSORB).** Pull in **import** (`/documents`: CSV + statement +
  receipt-vision, as an "Import" mode/drawer) and **duplicates** (`/duplicates`, as a "Review
  duplicates" tool). Getting transactions in and keeping them clean is the same theme as the ledger.

## 5.4 Feature-by-feature remapping

Every current section from §3, with its destination and the action. **Move** = relocate as-is;
**Stay** = no change; **Absorb-into** = becomes a section of a consolidated page; **Split-out** =
leaves its current page for a new focused one.

### From `/accounts`
| Current section | → Destination | Action |
|---|---|---|
| Net-worth summary strip | `/accounts` (assets) + link to `/debt` | Stay (liabilities shown as 1 linked figure) |
| Assets section + add/edit/reconcile/valuation | `/accounts` | Stay |
| Liabilities section | `/debt` | Move |
| Missing-FX-rate alert, Manage exchange rates | `/accounts` | Stay |
| Transfer money form | `/accounts` | Stay |

### From `/transactions`
| Current section | → Destination | Action |
|---|---|---|
| All ledger sections (table, filters, bulk, create-rule, split editor) | `/transactions` | Stay |
| (NEW) Import drawer | `/transactions` | Absorb-into (from `/documents`) |
| (NEW) Review-duplicates tool | `/transactions` | Absorb-into (from `/duplicates`) |

### From `/credit`, `/loans`, `/planning` (debt blocks)
| Current section | → Destination | Action |
|---|---|---|
| Credit proxy score ring + per-card utilization + limit editors | `/debt` (Credit section) | Move |
| Per-loan amortization cards + extra-payment sim | `/debt` (Loans section) | Move |
| Debt strategy (snowball/avalanche, burn-down) | `/debt` (Strategy section) | Move |
| Payoff calculator + projection | `/debt` (Calculator section) | Move |

### From `/bills`, `/subscriptions`, `/planning` (recurring block)
| Current section | → Destination | Action |
|---|---|---|
| Bills list + calendar + summary | `/recurring` (Bills view) | Move |
| Subscriptions list + detection + price changes + renewing + ignored | `/recurring` (Subscriptions view) | Move |
| Recurring cash-flow manager (label/cadence/autopay/post-due) | `/recurring` (Scheduled view) | Move |

### From `/planning` (what remains)
| Current section | → Destination | Action |
|---|---|---|
| 12-month net-worth forecast | `/planning` | Stay |
| Cash runway | `/planning` | Stay |
| Can I afford it? | `/planning` | Stay |
| What-if plans | `/planning` | Stay |
| Debt strategy / payoff | `/debt` | Split-out |
| Bills & recurring | `/recurring` | Split-out |

### From `/insights` + `/smart`
| Current section | → Destination | Action |
|---|---|---|
| Chat / Ask, pinned insights, system-prompt editor | `/assistant` (Ask) | Merge |
| Spending highlights, top merchants, monthly chart, anomaly highlights | `/assistant` (Insights) **or** `/reports` | Merge — see §5.5 |
| Smart Free-insight cards, AI features, digest, manage catalog | `/assistant` (Smart) | Merge |
| Allocate AI explainer | `/allocate` | Stay (decision-local) |
| Documents vision receipt-extract | `/transactions` import | Move ("assistant-powered") |

### From `/members` + `/split`
| Current section | → Destination | Action |
|---|---|---|
| Member list + add/edit + roles + reassign | `/household` (Members) | Move |
| Net worth by owner, Spending by member | `/household` (By person) | Move (also surfaced on `/reports`) |
| Split calculator + members picker + running balance + settle | `/household` (Split) | Move |

### From `/customize`
| Current section | → Destination | Action |
|---|---|---|
| Formula calculator + result + variables + saved formulas | `/customize` | Stay (narrowed) |
| Custom fields add-form + per-entity lists | `/fields` | Split-out |

### From `/widget-builder` + `/widget-manager` + `/p/:slug`
| Current section | → Destination | Action |
|---|---|---|
| Node-graph builder (palette/canvas/inspector/preview) | `/studio` (Build) | Merge |
| Widget manager (layout, table, tile-style) | `/studio` (Manage) | Merge |
| Custom-page add-widget + bento tiles | `/studio` (My pages) → renders `/p/:slug` | Merge |

### From the Data tools (`/documents`, `/duplicates`, `/artifacts`, `/activity`)
| Current section | → Destination | Action |
|---|---|---|
| Documents: CSV / statement / receipt-vision import + history + column-mapping | `/transactions` (Import drawer) | Absorb-into (vision = "assistant-powered") |
| Duplicates: group review + merge/delete | `/transactions` (Review-duplicates tool) | Absorb-into |
| Artifacts: image / CSV-dataset upload + list + storage meter | `/studio` (Assets) | Move — they back custom-page Image/Table widgets, so they live with the page builder |
| Activity: change / history audit timeline | `/activity` | Stay (already single-theme: history/audit) |

### Routes kept but **re-scoped** (de-aliased per §5.7a — not "unchanged")
- `/networth` — stop aliasing `Reports()`; render only the net-worth widget.
- `/debt`, `/recurring`, `/planning` — stop sharing one `Planning()` body; each renders only its theme.

### Unchanged single-theme pages
`/` · `/budgets` · `/goals` · `/allocate`¹ · `/reports`² · `/health` · `/investments` · `/activity` ·
`/todo` · `/notifications` · `/workflows` · `/categories` · `/rules` · `/appearance` · `/setup` ·
`/help` · `/about` · `/admin` · `/plans` — already each own one theme; keep as-is.

¹ `/allocate` keeps its theme but is *narrowed* (AI explainer becomes an inline call-out, §5.2).
² `/reports` keeps its theme but *absorbs* the duplicated insights charts and *sheds* AI-narrated
analysis to `/assistant` (§5.5/§5.7) — same page, cleaner boundary.

## 5.5 Cross-cutting features & judgment calls

A few features genuinely serve two themes; pick a primary home and *link* from the other rather than
duplicating logic:

- **Net worth.** Computed in many places. Primary home = `/networth` (the trend). `/accounts` shows
  the assets contribution, `/debt` the liabilities contribution, dashboard the headline tile — all
  read the same `ledger`/`forecast` core. Don't fork the computation.
- **Insights highlights vs Reports.** Spending highlights / top merchants / anomalies overlap with
  `/reports`. Recommendation: **analysis that's a static read → `/reports`; analysis phrased as
  advice or AI-narrated → `/assistant`.** (Mirrors the "deterministic & explainable" principle — the
  report is the truth, the assistant interprets it.)
- **Allocate.** Sits between budgets, debt, and goals (it suggests funding all three). Keep it
  standalone — it's a *decision* theme ("what next"), not an entity theme. Its AI explainer stays
  inline, not on `/assistant`, because it explains *that* ranking.
- **Bills derived from liabilities.** A credit-card due-date is both "debt" and "recurring". The
  *obligation* lives on `/debt` (it's a balance), the *calendar/reminder* on `/recurring` (it's a
  date). One due-date model, surfaced by both.
- **Categories vs Rules vs Custom fields.** All "data shaping". They stay separate (taxonomy /
  automation / schema are distinct jobs) but belong in one rail **group** ("Data & rules") so the
  relationship is legible.

## 5.6 Resulting rail (proposed grouping)

- **Money:** Dashboard · Transactions · Accounts · Debt · Investments
- **Plan:** Budgets · Goals · Allocate · Planning · Recurring
- **Understand:** Reports · Net worth · Financial health · Assistant
- **Act:** To-do · Notifications · Workflows
- **People:** Household
- **Build:** Customize · Fields · Studio
- **Data & rules:** Categories · Rules
- **System:** Appearance · Settings · Help · About · Admin

> **Open questions to settle before any refactor:** (1) Should `/debt` swallow `/credit` + `/loans`
> as sub-tabs, or keep them as deep-links into one page? (2) Is `/assistant` one page with tabs, or
> does the Smart *manage-catalog* deserve to stay under Settings? (3) Does splitting `/customize`
> into `/customize` + `/fields` add a rail item users won't find — or is discoverability better
> served by one "Build" group? These are UX calls, not code calls — worth a quick check against the
> SPEC and real usage before moving anything.

## 5.7 Deduplication — duplicated widgets across pages

### 5.7a Whole-screen route aliases (the literal duplication — fix these first)

The worst case isn't a widget shared between two designed screens — it's **multiple rail entries
that render the byte-identical same screen**, so *every* widget on it is duplicated across pages.
Verified in the source (`internal/screens`): two alias clusters exist, where the "page" is a
one-line shell returning another page's function:

| Rail entry | Route | Render function | Actually renders |
|---|---|---|---|
| Planning | `/planning` | `Planning()` | the full Planning screen |
| **Debt payoff** | `/debt` | `DebtPlanner()` → `return Planning()` | **the entire Planning screen** |
| **Bills & recurring** | `/recurring` | `Recurring()` → `return Planning()` | **the entire Planning screen** |
| Reports | `/reports` | `Reports()` | the full Reports screen |
| **Net worth** | `/networth` | `NetWorth()` → `return Reports()` | **the entire Reports screen** |

Consequences (this is the duplication you flagged with "planning and debt payoff"):

- Click **"Debt payoff"** and you get forecast + affordability + cash runway + recurring manager +
  what-if **and** the debt-strategy/payoff widgets — none of it scoped to debt. The debt-payoff
  widget is duplicated onto Planning, Debt, *and* Recurring simultaneously.
- Click **"Bills & recurring"** → same full Planning screen, including the debt-strategy and
  forecast widgets that have nothing to do with recurring bills.
- Click **"Net worth"** → you get the *whole Reports screen* (all four tabs), not a net-worth view.
  (This is why §5.5 flagged "confirm `/networth` isn't just the Reports tab" — it's worse: it's the
  whole Reports page.)

**Fix (matches §5.2/§5.3):** these aliases must become *real, scoped* screens, each rendering only
its theme's widgets — not `return SomeOtherScreen()`:

| Route | Today (alias) | Should render |
|---|---|---|
| `/debt` | `return Planning()` | **only** debt-strategy + payoff calculator + credit + loans + liability accounts (the §5.3 `/debt` page) |
| `/recurring` | `return Planning()` | **only** bills + calendar + subscriptions + scheduled cash flows (the §5.3 `/recurring` page) |
| `/planning` | full kitchen sink | **only** forecast + runway + affordability + what-if (the narrowed §5.3 `/planning`) |
| `/networth` | `return Reports()` | **only** the net-worth stat grid + trend chart (the §5.7b canonical net-worth widget) — or retire the route and let it deep-link to the Reports Net Worth tab |

Until then, each alias is literally the same `ui.Node` tree under three/two different labels — the
single biggest source of "same widget on multiple pages" in the app.

### 5.7b Cross-page widget duplication (same widget, genuinely different screens)

Beyond the aliases, several widgets are rendered **in full on multiple distinct pages** (not just
mirrored as a dashboard tile). The remap must collapse each to **one canonical implementation**;
every other surface either embeds that same component or links to its home — it never re-renders its
own copy.

**First, the legitimate exception — dashboard tiles.** The Dashboard tiles (`recent`, `budgets`,
`goals`, `trend`, `cashflow`, `savings`, `health`, `breakdown`, `bills`, `kpi-networth`, …) are
**glanceable mirrors/launchers** of full pages, by design. Keep them — but they must read the same
pure-logic core (`ledger`/`forecast`/`reports`/`budgeting`) as the full page, not a parallel
computation. A tile is a *view*, never a second source of truth.

**The real cross-page duplicates** (same full widget on two+ non-dashboard pages). This table is the
*IA view* (where each widget's theme should live after the remap); §5.7c below is the **code-verified
audit** (file:line) that separates genuine code dupes from already-shared helpers — read them together:

| Duplicated widget | Renders today on | Canonical home | Other surfaces become |
|---|---|---|---|
| **Net-worth stat grid + trend chart** | `/reports` (Net Worth tab); `/networth` (= whole Reports, see §5.7a); `/accounts` (summary strip); `/planning` (forecast base) | **`/networth`** (made into a real scoped page) | `/networth` stops aliasing Reports and renders only the net-worth widget; `/reports` Net Worth tab embeds that same component; `/accounts` keeps a one-line headline figure linking out; `/planning` reads the same `forecast` core |
| **Cash-flow trend** | `/reports` (Net Worth tab), Dashboard `cashflow` tile | **`/reports`** | Dashboard tile mirrors it |
| **Savings-rate trend** | `/reports` (hero + Savings Rate Trend), Dashboard `savings` tile | **`/reports`** | Dashboard tile mirrors it |
| **Spending-by-category breakdown** | `/reports` (Categories tab), Dashboard `breakdown` tile | **`/reports`** | Dashboard tile mirrors it. (Note: `/insights` "spending highlights" is NOT a second breakdown — it calls the shared `detectSpendingAnomalies` helper; see §5.7c.) |
| **Top merchants / top payees** | `/reports` (Overview: Top Payees), `/insights` (Top merchants) | **`/reports`** | `/assistant` links/embeds; drop the duplicate `/insights` card |
| **Monthly-spending chart** | `/insights` (monthly chart), `/reports` (spending trends) | **`/reports`** | `/assistant` embeds the report chart instead of its own |
| **Net-worth-by-member / spending-by-member** | `/members`, `/reports` (Spending by Member) | **`/household`** | `/reports` deep-links to `/household` "By person" (or embeds it) |
| **SVG score ring** | `/health` (`healthRing`) and `/credit` (`creditScoreRing`); plus the Dashboard `health` tile | **already shared** (`scoreRingNode`) | `healthRing`/`creditScoreRing` are already thin wrappers over the one `scoreRingNode` helper, each supplying only domain color/labels. NOT a dup — see §5.7c correction. |
| **Upcoming bills / dated obligations** | `/bills`, `/subscriptions` (Renewing soon), `/planning` (recurring detection), Dashboard `bills` tile | **`/recurring`** | All three views read one due-date model; tile mirrors it |
| **Recent-transactions list** | `/transactions`, Dashboard `recent` tile | **`/transactions`** | Dashboard tile mirrors it |
| **Goal / budget progress** | `/goals`, `/budgets`, Dashboard `goals`/`budgets` tiles | **`/goals`** / **`/budgets`** | Dashboard tiles mirror them |

**Duplicate-detection — three copies of one engine.** The `dedupe` finder is surfaced on
`/duplicates` (full review), `/transactions` (the "duplicate notice" bar), and `/documents` (the
draft-review "already imported" banner). After the §5.3 absorb, all three live under **one
`/transactions` "Review duplicates" tool** backed by the single `dedupe` package — the import flow
and the ledger call the same finder, not three.

**Anomaly findings — two surfaces, one engine.** `/insights` "anomaly highlights" (duplicate txns,
spending spikes, missing charges, balance anomalies) overlaps the `/smart` Free-engine insight
cards. Collapse both into **`/assistant`** reading the one rules/anomaly engine; the deterministic
findings can still be *cited* on `/reports`, but they're computed once.

**Payoff — two widgets, one calculation.** Within `/planning` today the **Payoff calculator +
projection** and the **Debt strategy** block both compute payoff. On the new **`/debt`** page these
become one calculation surface: the strategy view *is* the multi-debt payoff; the single-debt
calculator is a mode of it, not a separate widget.

**Smart entry-point buttons.** `smartSectionAction` adds a "smart" affordance to `/accounts`,
`/transactions`, `/budgets`, and `/goals` headers. These are fine to keep (per-page launchers), but
they should all route into the **one `/assistant`** home rather than four divergent panels.

**Dedup principle (one line):** *compute once in the pure-logic core; render the canonical widget on
its theme page; everywhere else embeds that component or links to it — no page re-implements another
page's widget.* This is just the CLAUDE.md "single write seam / platform-independent logic" rule
applied to read-side widgets.

### 5.7c Code-verified audit (all 37 screens, file:line)

A full scan of `internal/screens`, **re-verified against the live code**, grounds §5.7a/b and
corrects an important nuance: **most "same widget on two pages" cases are NOT duplication bugs** —
they're either *one shared helper with several callers* or *intentional dashboard mirror tiles*. And
two suspected dupes turned out to be **already deduped** in the code. Net deduping actually needed =
**2 route aliases (§5.7a) + 1 real code extraction**; everything else is correct factoring or IA
consolidation.

> **Correction (verified 2026-06-28):** an earlier pass flagged the SMART-anomaly run and the SVG
> score ring as "verbatim copy-paste". That was wrong — both are **already extracted** into shared
> helpers and both call sites delegate to them. Confirmed in code:
> `runAnomalyDetectors` (`smart_adapter.go:54`) ← called by `insights.go:1538` **and**
> `dashboard.go:1716`; `scoreRingNode` (`scorering.go:30`) ← called by `health.go:235` **and**
> `credit.go:162`. No action needed on either.

**(i) Genuine code duplication — real dedupe targets:**

| # | Widget | Status | Detail / fix |
|---|---|---|---|
| 1 | SMART anomaly run | ✅ **already deduped** | both callers delegate to `runAnomalyDetectors` (`smart_adapter.go:54`); only the row renderer differs by design. No action. |
| 2 | SVG score ring | ✅ **already deduped** | `healthRing`/`creditScoreRing` are thin wrappers over `scoreRingNode` (`scorering.go:30`); each only supplies domain color/labels. No action. |
| 3 | Top merchants / payees | ⚠️ **real, open** | `insights.go:1817` `topMerchantsSpendCard` hand-rolls a 90-day payee aggregation that the pure `reports.TopPayees` (`reports/payees.go:30`, used at `reports_screen.go:538`) already does. Fix: insights calls `reports.TopPayees` with a computed 90-day range. The one true remaining extraction. |
| 4 | Auto-detected recurring | ℹ️ **intentional, not a bug** | `planning.go:765` calls `subscriptions.Detect(..., 3)` (simple, ungated) and `subscriptions_screen.go:121` calls it with user prefs — same pure func, two contextually-justified call sites. Optional: unify sensitivity. Not duplication of logic. |

**(ii) Correctly shared / acceptable — leave as-is (NOT bugs):**

- **`detectSpendingAnomalies`** (`insights.go:1647`) — one helper, three consumers (`insights.go:1618`,
  `dashboard.go:656`, `reports_screen.go:420`). This is the *good* pattern, not a dup.
- **`reports.SpendingByMember`** — one package func called by `reports_screen.go:582` and
  `members.go:215`; correct on both, render differs by context (IA: surface the per-member view on
  `/household`).
- **Monthly expense chart vs Reports net chart** (`insights.go:1730` vs `reports_screen.go:258`) —
  same `reports.IncomeExpenseSeries` source; intentional semantic difference (expense-only vs net).

**(iii) Dashboard mirror tiles — intentional (17), keep.** `dashboard.go` renders glanceable
mirrors of full pages, each reading the same pure-logic core: `kpi-networth/income/spending/assets/
liabilities/safetospend` (share `ledger`/`safespend.Compute`), `trend` (`netWorthTrendWidget` ↔
Reports NW tab), `cashflow`, `savings`, `breakdown`, `bills` (shares the Bills derivation),
`highlight` (`detectSpendingAnomalies`), `anomaly-hub` (the #1 dup above), `health` (shares
`healthRing`), `budgets`, `goals`, `todo`, `accounts`, `recent`. By design — not flagged.

**Reconciliation note:** the §5.7b "duplicate-detection three copies" and "payoff two widgets" lines
came from the §3 feature inventory, **not** this code audit — the audit did not confirm a single
`dedupe`-package widget reused three times, nor two separate payoff implementations (Planning's
payoff calculator and debt-strategy are sections of one screen). Treat those two as IA observations
to verify in code before acting; the code-confirmed remaining dedup is just item (i) #3.

---

## 5.8 Feasibility audit (can the remap actually be built?)

A code-grounded structural pass over every screen in the remap. The question here is **buildability**
— is each proposed move/split/merge possible given how the widgets are composed — not whether it's a
good idea. Verdicts: **FEASIBLE** (move as-is) · **NEEDS-EXTRACTION** (refactor section/state out
first, then movable) · **HARD** (deeply entangled).

### Headline finding

The remap is **structurally feasible, but front-loaded on two "god functions":**

- **`Planning()`** (`planning.go:94`) — ~1,180 lines, **~30 interleaved `UseState`/`UseEvent` hooks**
  in one chain, with all 7 sections rendered into local vars (no sub-functions). It is the body
  behind `/planning`, `/debt`, **and** `/recurring`.
- **`Reports()`** (`reports_screen.go:136`) — ~1,200 lines, ~9 hooks, 4 tab sections as inline
  blocks. It is the body behind `/reports` **and** `/networth`.

Six of the ten screen-level items depend on extracting sections out of these two. The good news:
**every section in both draws only on portable data** (`appstate` + pure packages like `ledger`,
`forecast`, `payoff`, `reports`, `subscriptions`), and row-level hook isolation already uses the
`ui.CreateElement` component pattern correctly throughout — so the work is **mechanical extraction,
not architectural redesign.** Nothing is HARD; nothing is blocked.

### Per-item verdicts

| # | Remap item | Verdict | What it takes |
|---|---|---|---|
| 1 | De-alias `Planning()` → scoped `/planning` | **NEEDS-EXTRACTION** | Split ~30 hooks across 7 inline sections into discrete screen funcs. Recurring-manager section has the largest surface (11 `UseState` + 5 `UseEvent`). |
| 2 | De-alias `NetWorth()` → real `/networth` | **NEEDS-EXTRACTION** | `netWorthSection` (`reports_screen.go:1105–1153`, anchor `id="networth"`) is inline; extract its data pipeline + the `reportView` selector into `NetWorthScreen()`. |
| 3 | Consolidate `/debt` (credit+loans+strategy+liabilities) | **NEEDS-EXTRACTION** | `CreditScreen`/`LoansScreen` are drop-in portable (only `UseDataRevision`). Debt-strategy needs ~3 hooks pulled from Planning(); liability rows need the Accounts() mutation closures recreated (`AccountRow` is already props-driven, so the pattern is clear). |
| 4 | Consolidate `/recurring` (bills+subs+scheduled) | **NEEDS-EXTRACTION** | `Bills`/`Subscriptions` portable & independent (global atoms, no collisions). Recurring-manager's 11+5 hooks must be extracted from Planning(). |
| 5 | Merge `/assistant` (insights+smart) | **NEEDS-EXTRACTION** | `SmartHub` merges trivially (3 hooks). **`Insights` (18 page-local hooks) is `func() ui.Node`** — must become `func(props) ui.Node` so it mounts via `ui.CreateElement`; otherwise tab-gating it conditionally renders its hook chain and breaks hook-stability. |
| 6 | New `/household` (members+split) | ✅ **FEASIBLE** | Both already discrete, independent, self-contained funcs with separate hook chains. Render as sibling sections — no structural change. |
| 7 | Split `/customize` → `/customize`+`/fields` | ✅ **FEASIBLE** | `Customize()` is a 3-line compositor of `FormulaCalculator()` + `CustomFieldsManager()` (already separate files). Just point two routes at the two funcs. |
| 8 | Merge `/studio` (builder+manager+pages+artifacts) | ✅ **FEASIBLE** | All four discrete & independent; vb-prefix convention already avoids name collisions. Only nuance: `CustomPage(slug string)` takes a param — minor routing adaptation. |
| 9 | Absorb into `/transactions` (import+dupes) | **MIXED** | `DuplicatesScreen` is near-hookless → drop in as a tab (**FEASIBLE**). **`Documents` has 20+ `UseState`** → must be a `ui.CreateElement` component if tab-mounted, else keep as a linked sub-route (**NEEDS-EXTRACTION**). |
| 10 | Narrow `/accounts` to assets | **NEEDS-EXTRACTION** | Liabilities render block is only 3 inline lines, but `renderRow` captures Accounts()-local closures — recreate them in `/debt`. Verified: removing the liabilities **render** does NOT affect the net-worth summary (it's computed by `ledger.NetWorthExplained` over all accounts, independent of what's rendered). |
| 11 | The 4 "code dupes" | ✅ **MOSTLY DONE** | #1 & #2 already extracted (§5.7c); #3 top-merchants is a clean one-call swap to `reports.TopPayees`; #4 is intentional. |

### Critical path & sequencing

1. **Refactor `Planning()` first** — extract *recurring manager*, *debt strategy*, and *payoff
   calculator* into standalone screen functions (each owning its slice of the hook chain). This single
   refactor unblocks items 1, 3, and 4 and lets `/debt` + `/recurring` become real routes.
2. **Refactor `Reports()`** the same way at smaller scale → unblocks item 2 (`/networth`).
3. **Make `Insights` component-shaped** (`func(props) ui.Node`) → unblocks the `/assistant` merge (5)
   and the `Documents` tab-mount path (9).
4. **Recreate the props-driven mutation closures** for liability/account rows in `/debt` → items 3 & 10.
5. Items 6, 7, 8 can be done any time — they're independent and trivial.
6. Item 11 #3 (top-merchants) is a standalone one-liner cleanup.

**Bottom line:** the entire remap is feasible with **zero architectural blockers** — the data layer
(`appstate` + pure packages) already supports relocation everywhere. The cost is concentrated in
de-monolithing `Planning()` and `Reports()` and making `Insights` component-shaped; everything
downstream is mechanical. None of this is in scope to implement here (document-only) — it's the
buildability map for when the refactor is greenlit.

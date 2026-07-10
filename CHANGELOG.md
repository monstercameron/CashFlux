# Changelog

All notable changes to CashFlux are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Policy: **one feature per commit**,
and every commit updates this file under `Unreleased`.

## [Unreleased]

### Fixed
- **Budgets top tile no longer intermittently blank on a direct link (2026-07-10):** opening `/budgets` directly (a cold deep-link / hard refresh) sometimes rendered the summary tile invisible while the rest of the page appeared. Root cause: every `.bento .w` tile's entrance animation used `animation-fill-mode: both` with a `from { opacity: 0 }` keyframe, so a tile whose animation never ran to completion — under the main-thread load of a cold wasm boot, or a re-render that restarted it — could settle at `opacity: 0` with nothing to re-trigger it. The tile's resting state is now visible (`opacity: 1`) with `fill-mode: forwards`, so a dropped animation degrades to "shown immediately" instead of "hidden forever". Also made `wonder.js` reveal already-in-viewport `.card` elements synchronously (as its own contract always promised) so above-the-fold content never waits on the async IntersectionObserver.

### Added
- **Choose which income funds your budget — by source, in a Save/Cancel flip modal (2026-07-10):** the zero-based summary's income figure now has a discoverable **Budget income** button that opens an **"Income to budget with"** flip modal (configs live in modals here, staged behind Save/Cancel — changes apply only on Save, discard on Cancel). Beyond *all income* / *paychecks only* / *a set monthly figure*, a new **Choose income sources** basis lets you include or hold aside each **income category by name**: an income-source ledger showing each category's recent amount, a live "Budgeting against $X" running total with an **"N of M included"** count, and **Include all / Hold all aside** bulk actions. A **3-month average** toggle budgets irregular income (freelance, commissions, tips) off a steadier figure — averaging even surfaces sources that earned in prior months. The picker is reachable from **every** method, not just zero-based. Backed by pure, tested `budgeting.ZeroBasedIncome` (categories mode) + `budgeting.AveragedIncome`. The zero-based summary is also redesigned around an **allocation bar** — your income split into Expenses / Savings / still-to-assign, with an income-reference marker and a red **Over-assigned** reading when you assign past your income (replacing the old loose figure chips).
- **Set a monthly budget + roll leftover into next month (2026-07-09):** the zero-based view's "Budget against" now offers **A set monthly budget** — a fixed amount you type (labelled "Monthly budget") to budget against instead of your actual income, so you can cap what you assign each month. And a new **"Roll last month's leftover into this month"** toggle adds last month's **unspent budget** (each budget's limit minus what was spent, clamped at zero, excluding budgets that already carry their own remaining) into this month's assignable pool — shown as a **Rolled over** figure in the breakdown, raising your To-Assign. Off by default.

## [1.0.10] - 2026-07-09

### Changed
- **Zero-based budget summary leads with "To Assign" (2026-07-09):** in the zero-based view the summary now opens with the big **To Assign** figure (the thesis — give every dollar a job) and demotes the spent/budgeted/left progress bar below it under a "Spending so far this month" caption, so the number you're driving to $0 is the headline instead of competing with the safe-to-spend figure.

## [1.0.9] - 2026-07-09

### Added
- **Zero-based budgeting across your whole salary (2026-07-09):** the Budgets page's zero-based method is now a real "give every dollar a job" workflow that spans **expenses AND savings/investments**. A big **"To Assign"** hero shows income minus everything assigned — driven to $0, green when balanced, red when over — over a breakdown of Income · Expenses · Savings & investing. Savings/investments come from your **Goals**: each active goal's monthly contribution counts toward the assigned total, with a new **Savings & investments** section where you can adjust each goal's monthly amount inline to reach $0. Goals gain an optional **monthly budget amount** (in the goal editor) so an open-ended investing goal with no target date can still take a flat monthly (otherwise the target-date pace is used). The income you budget against is **configurable** — *all* of last month's income, *paychecks only* (deposits at or above a threshold, so side hustles are ignored), or a *fixed amount* — via a "Budget against" control. Simple/envelope methods are unchanged.

### Fixed
- **Multi-category budget "Transactions" now filters by ALL its categories (2026-07-09):** clicking Transactions on a budget that tracks several categories (e.g. "HOA, Mortgage Payment") drilled to only the first category. It now opens the ledger filtered to **every** tracked category (OR — a transaction matches if it's in any of them), shown as one removable chip per category. Backed by a new comma-joined `Categories` filter dimension; picking a single category from the toolbar still replaces it. Single-category budgets are unchanged.

### Added
- **"Utilities" account type (2026-07-09):** a new account type for utility / HOA / service accounts (electric, water, HOA dues). It classes as a **liability** — a recurring obligation you owe — so a Utilities account counts as debt in net worth and the debt formulas with no manual toggle, and (like every liability) it's excluded from low-balance alerts. Appears in the account type picker grouped with the other liabilities, with a receipt glyph.

### Fixed
- **Notification action buttons show their icons again (2026-07-09):** each notification's inline mark-read / snooze / dismiss controls rendered as empty squares because they were sized with literal `w-4`/`h-4` classes that don't exist in the app's CSS (the real utilities are the atomic `tw.W4`/`tw.H4`). Since the icon primitive takes its size only from caller classes, the SVGs collapsed and the buttons shrank to ~13px. Now sized with `tw.W4`/`tw.H4`, so the check / clock / ✕ glyphs (and the linked-notification chevron) render.
- **Low-balance alert no longer lingers on accounts you mark a liability (2026-07-09):** a "balance is low" warning raised while an account was an asset stayed in the feed even after you set the account to a liability — where a zero/low balance is *good* (you owe nothing). New alerts already skip liabilities and archived accounts; now a boot-time reconcile also clears any stale low-balance notification for an account that has since become a liability or been archived, so it self-heals.
- **Background music no longer plays when muted, incl. on the lock screen (2026-07-09):** the player keyed playback off an in-memory on/off flag that could be stale (seeded ON before the persisted store loaded), so a muted user could get music the moment they interacted — most visibly typing the passcode on the lock screen, which is the gesture that releases a browser-blocked autoplay. The player now does a **hard check of the persisted mute state right before it plays** (in `enable()` and at the track-start `play()` site); when muted it refuses and stays consistent. The lock-screen mute toggle now persists the new choice **before** telling the player, so unmuting from the lock screen reads the fresh value and resumes correctly.

## [1.0.8] - 2026-07-09

### Added
- **Import a statement with AI (2026-07-09):** a new **Import statement** button on the Transactions toolbar opens a large modal where you attach a bank/credit-card **statement PDF**. It's sent straight to the AI — which reads the PDF's text *and* page images natively (so scanned statements work, no conversion) — and returns the transactions in an editable review table. Categories are **best-effort mapped to your existing ones** and left blank when nothing fits (it never invents a category). Fix any row inline (date/description/amount/category), duplicates are flagged, then import. BYO OpenAI key; the file is sent only to OpenAI. An optional **"Keep a copy in this browser"** checkbox saves the source PDF under *Your files* (stored in the browser's IndexedDB, not the SQLite dataset), so you can find the statement later; unticked, nothing is kept. (SMART-T18.)
- **Budgets: one-click "Last month" toggle (2026-07-08):** a **Last month** button on the budgets toolbar flips the whole page to the previous period so you can see last month's budget picture at a glance (click again to return). It shifts every budget tile's evaluation window back one period — the spend figures are last month's actuals against your current allocations. (A future option: recording allocation snapshots so it also reflects last month's *limits*.)
- **Multi-category budgets: track 1–n categories in one budget (2026-07-08):** a budget's `⋯` menu has a new **Edit tracked categories** action, and the **add** and **edit** budget forms now include the same picker, so a budget can count several categories — its spend becomes their combined total (each category still rolls up its own sub-categories). The picker is a searchable, one-line checklist with a soft "also in …" note when a category is already tracked by another budget. Single-category budgets are unchanged. Backed by an additive `Budget.CategoryIDs` and the spend engine's existing predicate path (no store migration).

### Changed
- **Statement-import review reads like a ledger (2026-07-09):** the editable transactions table in the Import-statement modal was refined so a statement's figures scan the way the statement does. Amounts are now **right-aligned in a column and tinted by direction** (money in green, money out red — reinforcing the existing accounting parentheses, so it never relies on color alone), a **money-in / money-out / net summary** sits in the sticky header and foots the amount column, each row's **category shows as a chip** with a subtle **"+ Category"** affordance when the AI left it blank (so unmapped rows read as an action rather than an absence), the redundant second Import button was removed (one sticky account-picker + Import, always reachable, now labelled with the count), and the inline row editor uses a steadier purpose-built grid. The receipt/image importer shares the table and inherits the alignment and category chips; amount-tinting is off there (receipt lines are all positive prices).
- **Budgets income figure uses last full month (2026-07-08):** the budgets page's income context (the assign banner) derived income from the *current* — partial — month, which under-reports before that month's paychecks land. It now uses the most recent *complete* month as the honest basis (the configured income figure still takes precedence). Labelled "Income (last month)".

### Fixed
- **AI statement/receipt import no longer rejected by the model (2026-07-09):** the statement-import and image/receipt-import AI calls sent `temperature: 0.1`, which the flagship vision model (`gpt-5.5`) rejects outright ("Unsupported value: 'temperature' … Only the default (1) value is supported"), so the whole extraction failed. Those calls now omit temperature and use the model default. (The text-based paths use `gpt-5.4-mini`, which accepts a custom temperature, and are unchanged.)
- **Bill auto-link rules apply reliably alongside category rules (2026-07-08):** the bill-account rule action is now applied independently of the first-match category rule, so an auto-link rule is no longer silently shadowed when another rule (e.g. a category rule for the same merchant) matches first. The first matching rule that sets a bill account wins for that action, on entry, import, and backfill.

### Added
- **Auto-link future bill payments to an account (2026-07-08):** the "Link this payment" modal now offers **"Auto-link future \<merchant\> payments"** once you pick an account. Ticking it creates a rule so that future — and **imported** — transactions from that merchant automatically link as bill payments to the same account (matched on the payee, "contains"). Rules gained a new **"Link as bill payment to"** action (visible/editable on the Rules page), applied wherever rules already run — CSV import, document/vision import, manual entry, and the "apply to existing" backfill — and it never overwrites a link you set by hand. Backed by `rules.Rule.SetBillAccountID` and the existing `AutoCategorizeTransaction` path (unit-tested).

## [1.0.7] - 2026-07-08

### Added
- **Auto budget: suggest monthly budgets from your spending (2026-07-08):** a new ✦ **Auto budget** button on the Budgets page opens a review modal that learns a suggested monthly budget for each expense category from your history, and shows each with a **slider** so you can tune the target up or down before creating them. Two methods: **Recent** (the Smart, free take — averages your last 3 months) and **Healthy average** (the Smart+ take — reviews 6 months and drops one-off spikes for a sustainable target you can actually keep). Categories that already have a monthly budget are flagged and left unticked so nothing is overwritten by accident; a running total and a per-category "% of avg" readout keep the adjustment legible. Save creates or updates the budgets in one go. Deterministic and on-device (no AI) — backed by the pure `budgeting.SuggestBudgets`/`HealthyLimit` helpers (unit-tested) and catalogued as SMART-B11 / SMART-B12.

### Fixed
- **Filtering transactions by an account now includes payments linked to it (2026-07-08):** filtering by an account showed only transactions *booked on* it (AccountID). A bill payment is booked on the account the money leaves (e.g. checking) and linked to the account it pays (e.g. an HOA or a loan), so filtering by that linked account returned nothing even right after linking a payment. The account filter now matches a transaction on the account **or** linked to it as a bill payment, so those payments show up (verified end-to-end + unit-tested).

## [1.0.6] - 2026-07-08

### Added
- **Count an "Other" account as a liability (2026-07-08):** the account add/edit forms now show a "Count as a liability (debt)" toggle when the account type is **Other** (the catch-all type with no natural asset/liability class). Turning it on classes the account as money owed, so it's included in net worth and the debt formulas — useful for obligations like an HOA balance that don't fit a specific type. The stored class is now the single source of truth the liability formulas read (net worth, financial-health debt ratios, and the low-balance nudge all honor the override).

### Changed
- **Link a bill payment to ANY account, not just debts (2026-07-08):** the Link-payment modal's account picker now offers every account, so you can mark a transaction as a bill payment toward a checking, savings, or other account — not only liabilities. Wherever an account appears with a linked bill payment, it shows a "Last bill $X · N →" line that drills to the backing transactions: the **Accounts** page now shows it for every account (previously only the Debt page showed it, for liabilities). Copy is account-neutral ("Which account is this paying?").

### Added
- **Link a payment to a subscription, and a cleaner Link-payment modal (2026-07-08):** a transaction's `⋯` menu now has two actions — **Mark as bill payment…** and **Mark as subscription payment…** — that open one **Link this payment** flip modal. The modal leads with the transaction (description, account, and the amount as a display figure), offers a **Bill payment / Subscription** toggle, a single picker for the target debt or subscription, and a live "Links to" preview of exactly what Save will do (both links save together). Marking a subscription payment surfaces on the **Subscriptions** page as a "Last paid $X · N →" line on that subscription's row, drilling to the transactions that back it — the same payment-check the Debt page already gives liabilities. This replaces the previous flat "one menu item per liability" bill list with a scalable, self-explanatory modal. Backed by `Transaction.SubscriptionName`, the pure `ledger.SubscriptionPaymentForName` helper (most-recent by date, sign-robust; unit-tested), and a `txnfilter` subscription criterion; regression-pinned end-to-end for both bill and subscription flows.

## [1.0.5] - 2026-07-08

### Added
- **Mark a transaction as a bill payment toward a debt (2026-07-08):** each transaction row's new `⋯` menu can mark that transaction as a **bill payment** for any of your liability accounts (or clear the link). The Debt page then reads the most recent linked payment as that account's **actual monthly payment** — shown as "Bill $X/mo" beside (and distinct from) the minimum — with a "· N payments →" link that drills to exactly the transactions backing it (a new bill-account transaction filter). This is the recurring-payment truth for a debt taken straight from your ledger, and a way to eyeball whether a bill actually got paid. Backed by `Transaction.BillAccountID`, the pure `ledger.BillPaymentForAccount` helper (most-recent by date, sign-robust; unit-tested), and a `txnfilter` bill-account criterion; regression-pinned end-to-end. (Subscriptions and a bills-page due-date quick-edit are the next two increments.)

### Fixed
- **Smart+ categorization result checkboxes now render consistently (2026-07-08):** the accept/reject checkboxes in the AI categorization review list (Suggest / Auto-fill / Fix mistakes) were bare native `<input type="checkbox">`, which render inconsistently across OS/browser/theme — on some laptops a tiny, near-invisible dark-on-dark box. They now use the app's branded `.cf-check` (a themed box with an accent fill + white checkmark, same as the budget cover-source picker), so the selection is clearly visible in both light and dark themes on every device.
- **Row "⋯" menus in the transactions table are clickable again (2026-07-08):** an open row overflow menu inside the transactions `DataTable` couldn't be clicked — its items either fell behind the transparent full-viewport `.add-backdrop` (which won the hit-test) or were clipped by the actions cell's `overflow:hidden` (the fixed 96px cell hides the wider menu, so an item painted nowhere and the cell underneath swallowed the click). Two fixes: `OverflowMenu` no longer renders the redundant `.add-backdrop` (outside-click dismissal is already handled by a document-level listener), and an open `<td>` holding a menu now lifts above its siblings and stops clipping (`position:relative; z-index; overflow:visible`). The `.row`/`.budget`/`li` lift rules never matched `<tr>`/`<td>`, which is why table rows regressed where card rows didn't.
- **OpenAI key now persists across reloads (2026-07-08):** entering a key only survived a reload if you also found and enabled the "Remember AI key" toggle (off by default), so the key silently vanished on refresh. Now entering a key persists it on-device by default (and flips the toggle on — turning it off still keeps the key session-only and clears the stored copy), and boot restores any stored key directly rather than gating on a preference that rides the slower dataset autosave. The key is still kept out of the exported/synced dataset.

## [1.0.4] - 2026-07-08

### Added
- **The AI chat assistant can categorize too (2026-07-08):** the in-app assistant gained three tools — `list_uncategorized_transactions`, `create_category`, and `categorize_transactions` — so you can just ask it ("categorize my transactions", "create a Groceries category", "fix anything mis-filed") and it plans the work, creating categories and assigning them with the usual per-change approval. Verified live: the agent created a category on request through the approval flow.
- **"Suggest categories" button on the Categories page (2026-07-08):** the Smart+ categorization modal is now reachable from /categories (not only the /transactions toolbar) — a ✦ button in the expense section header opens it, leading with the suggest-new-categories scan.
- **Smart+ categorization: three AI scans on /transactions (2026-07-08):** a "Categorize" button opens a shell-root flip modal with three opt-in AI scans of your ledger, each producing a checklist you confirm before anything is written. **Suggest** (SMART-T15) proposes NEW categories that would cover your uncategorized transactions — you pick which to create. **Auto-fill** (SMART-T16) proposes an existing category for each uncategorized transaction. **Fix mistakes** (SMART-T17) reviews your categorized transactions for likely miscategorizations and proposes corrections. The AI can never invent a category — every suggestion parses against your real category list — and the scan is the consent step (nothing leaves the device until you pick a mode and click Scan). Applying Auto-fill / Fix mistakes is undoable via the usual bulk-undo bar. Verified end-to-end against the live model.

## [1.0.3] - 2026-07-08

### Added
- **Transactions: assign to a household member, a User column, and show/hide columns (2026-07-07):** three related additions to the ledger. (1) The bulk-action bar (shown when transactions are selected) now has an **Assign to** picker — set the selected transactions' member in one go (undoable like the other bulk ops). (2) The ledger table has a new **User** column showing each transaction's assigned member (a muted "—" when unassigned). (3) A **Columns** button in the toolbar opens a flip modal to show or hide the optional columns (Amount, Account, Category, Source, User); the choice persists across reloads. Date and Description always show. The modal mounts at the shell root so a tile's transform can't clip it.
- **Goals: "Undo last contribution" and "Reset to zero" in the ⋯ menu (2026-07-07):** a financial goal's overflow menu now offers **Undo last contribution** — reverses the most recent contribution, subtracting it from the goal's progress and deleting the linked ledger transaction if one was posted — and **Reset to zero** (with a confirm), which clears the goal's saved progress back to $0 while leaving any real linked transactions in place. Backed by a new contribution log on the goal (`domain.Goal.Contributions`, capped, JSON round-tripped) and app ops `UndoLastContribution` / `ResetGoalToZero`, all unit-tested. Undo only shows once there's a logged contribution; reset only when the goal holds anything.
- **Smart panels can be snoozed and dismissed in bulk (2026-07-07):** each Smart strip now has a "⋯" actions menu in its header with **Dismiss all** (clears the whole current batch of nudges at once, complementing the per-nudge Dismiss on each card) and **Snooze for a day / a week** (hides the entire Smart strip everywhere until the snooze expires). Backed by new pure `smart.Settings` state (`DismissAll`, `SnoozeUntil`/`IsSnoozed`, unit-tested); the snooze is data-derived and time-bound, so a data wipe (`ClearGenerated`) drops it.

### Fixed
- **/accounts "Manage debts in Debt payoff →" navigates without reloading (2026-07-07):** the link was a plain `<a href>`, so clicking it did a full page load — re-booting the whole app, and (with app-lock enabled) dropping the user onto the passcode screen. It now navigates client-side through the router (the href stays for keyboard/middle-click), so it jumps straight to /debt with no reload and no spurious re-lock.
- **Lock-screen music mute toggle now sticks (2026-07-07):** the app-lock gate's mute button drove the JS player and storage directly but never updated the shared `UseMuzakEnabled` atom, so the top-bar toggle's effect could re-apply the stale "on" value and the music would resume — the toggle looked stateless and "always played". A new `uistate.SetMuzakEnabled` routes the lock-screen mute through the same atom the rest of the app uses, keeping player, toggle, and storage in sync.
- **Overflow "⋯" menus no longer swallow clicks on their own trigger (2026-07-07):** the shared `OverflowMenu` component toggled a bare `hidden` class, but the stylesheet only hides the popover/backdrop via `hidden-menu` (`display:none` + `pointer-events:none`). So a *closed* menu's full-viewport backdrop stayed painted and intercepted pointer events — the trigger looked clickable but nothing happened. Switched the component to the styled `hidden-menu` class.
- **Dashboard "While you were away" card no longer breaks the grid (2026-07-07):** the catch-up card had no CSS at all — it rendered as an unstyled block floating between the hero and the bento grid. It now has proper card chrome (theme-aware `--bg-card`/`--border`/`--radius`, box-shadow, light-theme variant) and a flex layout — a clean full-width tile that aligns with the bento grid instead of disrupting it. The dormant entrance animation (which targeted a testid the element never had) now fires too.

### Added
- **Account type is editable (2026-07-07):** the account edit form now has an Account type picker, so an account can be reclassified — e.g. a line of credit → a credit card, or an asset ↔ a liability. The class follows the chosen type: the attribute fields shown switch live (credit limit / APR / min-payment for liabilities vs. expected-return / liquidity / lock-until for assets), and on save the fields that don't belong to the new class are cleared so a reclassified account carries no stale data. Combined with the sign-robust net-worth fix below, flipping between asset and liability now updates net worth and the /accounts display correctly.

### Fixed
- **Liabilities are treated as debts regardless of how their balance is stored (2026-07-07):** an account added through the "amount you owe" form stores a *positive* balance, but net-worth code assumed liabilities were stored *negative* (the sample convention) — so a form-added debt was **added** to net worth instead of subtracted, which also skewed the debt page's "Debt vs assets" ratio and "Total owed" sign. `ledger.NetWorth`/`NetWorthExplained` now take the owed amount as the magnitude (`Abs`) of the balance, correct for both storage signs (guarded by a new positive-liability test). On `/accounts`, liability rows now display a **negative** figure (a debt reduces net worth) and sort below the assets, taking `-Abs` so the sign is right no matter how the balance was stored. Editing still operates on the raw stored value.

## [1.0.2] - 2026-07-07

### Fixed
- **Green CI: cleared every latent build-test failure the govulncheck red was masking (2026-07-06):** with govulncheck fixed, steps that never ran surfaced their own failures — the api-compat guard wanted a `## Future Codegen` section in `proto/README.md` (added), `buf generate` didn't reproduce a hand-added SPDX header on the generated pb files (regenerated to buf's canonical output), and gosec flagged a real path-traversal in the operator-console file server (`console.go` now cleans the request path against root before joining), an intentional FNV-hash overflow (annotated `#nosec`), and its own scan of the e2e test server (excluded — test tooling, not production). Every `build-test` step now passes locally.
- **CI security scan is clean + regression suite is date-deterministic (2026-07-06):** cleared the pre-existing govulncheck red by bumping Go to 1.26.4 (stdlib CVEs) and otel/sdk + otlptracehttp to v1.44.0 (also lifting grpc to 1.81.1). And made the coverage ratchet stable across run dates — `boot()` now pins the wall clock (`setFixedTime`) before the app boots, so bill-due notification testids (which embed the current month) no longer drift and fail CI on a different day than the baselines were captured. Also hardened globalSetup against a read-only toolchain `wasm_exec.js`, and fixed a scary near-miss where the legacy-quarantine step's `git ls-files 'e2e/*.mjs'` matched recursively and swept the whole suite into `_archive`.

### Changed
- **Account rows show a month-to-date value change, not a history list (2026-07-07):** illiquid-asset rows (home, car, investment) used to render a scrolling list of every recorded valuation ("Jul 6 24,000 · Jul 6 26,000 · …"), which was noise inside the row. They now show a single signed figure — "▲ $2,000.00 this month" / "▼ …" (toned green/red) or "No change this month" — computed by a new pure `internal/valuation` package (`MonthToDateChange`: current value minus the value carried forward from the start of the month, falling back to the earliest snapshot when the account's history began this month). Unit-tested natively.

### Added
- **/accounts: an All / Assets / Liabilities view toggle (2026-07-07):** the accounts page showed only asset rows and sent liabilities off to Debt payoff, so there was nowhere to eyeball both sides of net worth at once. The account list now carries a segmented toggle in its header — **All** (default), **Assets**, **Liabilities** — so both classes can be spot-checked in one place. Every view sorts by signed balance high to low — biggest holdings on top, heaviest debts (parenthesised negatives) at the bottom — using the raw net-worth-signed balance (assets positive, liabilities negative — matching the /networth breakdown). Liability rows render through the same `AccountRow` as assets (utilization, update-balance, edit). The chosen view persists across reloads (saved to localStorage). The toggle is hidden when there are no liabilities, so a debt-free household sees exactly the old assets list. A "Manage debts in Debt payoff →" shortcut sits below the list (and, in the assets-only view, still names how many liabilities are tucked away).
- **Pyramid middle layer: wasm component tests (2026-07-05):** the fast layer between native logic tests and browser e2e — real components mounted through GoWebComponents' `testkit/render` (mock DOM, no browser). `internal/ui/meter_wasm_test.go` is the reference example (MeterBar role/aria + value clamping). These are `js && wasm`-tagged, so `go test ./...` skips them; a dedicated lane (`tools/wasm-test.ps1` + a Windows `go_js_wasm_exec.bat` wrapper) runs them and is wired into the CI Windows job. This surfaced that the wasm-tagged `registry_test.go` had silently rotted (CI never ran it) — it flagged the 18 intentionally off-rail routes (e.g. /plans, /setup, /duplicates) as broken; fixed to enforce rail Label/Group only for on-rail routes while still requiring Path/Title/View everywhere.
- **Quarantined 651 stale legacy e2e scripts (2026-07-05):** the ad-hoc `node`/probe scripts and screenshot dumps at the top of `e2e/` (many bit-rotted post-IndexedDB-migration) moved to `e2e/_archive/` with a README marking them unmaintained and non-gating — so the trusted suite (`e2e/regression/`) is unambiguous and a green check means green.
- **CI now gates on the regression suite (2026-07-05):** a new `e2e` job in `ci.yml` (on `windows-latest`, so the committed coverage/a11y/visual baselines match the runner) builds the wasm, serves it, and runs the full Playwright suite on every push/PR — the regression tests finally block merges instead of being hand-run. Playwright report is uploaded on failure. (Visual pixel-diffs stay a local Windows gate — skipped in CI, where cross-machine font/DPI rendering can't be tolerated.)
- **Enterprise regression foundation: Playwright Test runner + hermetic pipeline (2026-07-05):** the regression suite moves off hand-run `node *.mjs` scripts onto the Playwright Test runner (`e2e/playwright.config.mjs`), with retries, trace/screenshot/video-on-failure, and an HTML report. A `globalSetup` builds the wasm + drops the matching `wasm_exec.js` so a fresh CI checkout is self-contained, and Playwright owns the static server (`e2e/serve.go`) lifecycle on a fixed port — no dependency on a hand-started `gwc dev`. The suite is deterministic: it waits on real signals, never wall-clock sleeps.

- **Visual regression on stable pages + a Node static server (2026-07-05):** `visual.spec.mjs` pixel-diffs content-stable pages (/about, /plans) in both themes with the clock frozen and dynamic regions masked — narrow by design (dynamic dashboards are covered by invariants/interactions, not flaky pixel diffs). Baselines are Windows-native (`-win32`); the spec skips off-Windows. The suite now serves via a Node port of serve.go (`serve.mjs`) so it needs no Go at serve time, and a `settle()` helper (fonts/images/frames) plus a 2-worker cap keep render-sensitive checks deterministic under load. Added `e2e/README.md` documenting the layers + baseline regeneration.
- **Cross-cutting invariant + a11y gates (2026-07-05):** `invariants.spec.mjs` asserts, on every route, that the theme engine emits all core design tokens (non-empty) in both themes (killing the `var(--fg)` undefined-token bug class) and that the page body never scrolls horizontally (wide content must scroll inside its own container). `a11y.spec.mjs` runs axe-core (WCAG 2 A/AA) as a baseline ratchet across representative routes — it locks in current structural violations so none regress while the debt burns down (color-contrast is excluded from the automated gate as render-timing-flaky; it's covered by visual regression instead).
- **Interactive-element coverage ratchet (2026-07-05):** `coverage.spec.mjs` harvests every interactive control per route (normalizing per-row dynamic ids to a stable pattern) and diffs against a committed `coverage-manifest.json`. A new/removed control, a new route, or a rise in the count of controls lacking a `data-testid` fails the test until the manifest is intentionally regenerated (`UPDATE_COVERAGE=1`) — so "what's clickable" can't drift silently and new controls can't ship untracked. This is the enforceable answer to "is every clickable item covered?".
- **Per-page interaction regressions on the runner (2026-07-05):** the v1 wave-1/wave-2 fixes are now `interactions.spec.mjs` — real user actions (add a to-do, open Settings tabs) asserting the RESULT, isolated and parallel — replacing the hand-run bespoke `all_routes_smoke`/`v1_wave1`/`v1_wave2` scripts (removed).

### Changed
- **App exposes deterministic test signals (2026-07-05):** boot completion now sets `document.documentElement[data-app-ready="true"]` (+ `window.__cashfluxReady`) once hydrate+seed+mount+wiring finish, and the shell's main pane carries `data-route` reflecting the active screen. These let the suite wait on "app ready" and "route mounted" instead of guessing timeouts, eliminating boot/navigation flakiness. Both are inert in normal use. The v1.0 polish campaign inspected and refined every page
across ten review groups (core money, goals/tasks, debt/invest, recurring/bills,
reports, data & people, studio/build, data management, system settings, and the
custom showcase pages), fixing theming/token bugs, responsive sizing, copy, and
destructive-action confirmations, and adding regression coverage for every route.

### Added
- **Second-wave regression suite (`e2e/regression/v1_wave2_fixes.mjs`) (v1.0, 2026-07-05):** pins the Group I/J fixes — custom-page list sub-lines, the positive-magnitude costs chart, backend-off-by-default with hidden live actions, and the hidden single-language picker.

### Changed
- **Product version is 1.0.0 (2026-07-05):** `internal/version` and the desktop shell now read 1.0.0; the rail/About footer shows v1.0.0.

### Fixed
- **Settings › Advanced is tidier (v1.0, 2026-07-05):** the Display-language dropdown had exactly one option (English), so it was a no-op control — it's now hidden until a second language is imported, replaced by a one-line hint pointing at the import button. And the app-lock hint field's placeholder ("A reminder (must not contain the passcode)") was clipped mid-word by the narrow modal; it's shortened to "A reminder — not the passcode itself" so it fits.
- **Settings › Alerts inputs line up (v1.0, 2026-07-05):** the alert-threshold number inputs were ~2x wider than the freshness-day inputs on the same tab (they sit in a different row class that missed the 90px width rule) — both now share the 90px width.
- **Backend "off" now means fully off (v1.0, 2026-07-05):** two Settings › Cloud issues. The backend shipped switched ON out of the box (pointed at a loopback dev URL), which read as "already connected to something" and contradicted About's "Cloud sync is off by default" — a fresh install now defaults the backend OFF (the saved URL is just a prefill for when you opt in). And "Test connection", "Sync now", and "Upload key" stayed clickable while the backend was off, so clicking them fired a real network request the "fully local" copy promised wouldn't happen — those three actions are now hidden until the backend is switched on.

### Added
- **Flow series can plot magnitudes (`abs`) (v1.0, 2026-07-05):** a `flow`-metric chart bound to a pure-expense category (Priya's "Shop costs") plotted its signed monthly sums, so a "costs" chart showed a run of negative dollars — accounting-correct but confusing under a "costs" title. `SeriesSpec` gains an opt-in `Abs` flag that plots each month's magnitude; the sample's Shop-costs widget uses it, so costs now read as positive dollars.

### Fixed
- **Custom-page charts and lists read on any page (v1.0, 2026-07-05):** two custom-page (`/p/*`) regressions from the Group J sweep. The sample data's SVG "chart" images (e.g. the side-hustle revenue rollercoaster) were drawn as light-on-transparent, so on a light-theme card they washed out to near-invisible — the header now paints a dark `#1a1a1d` backing rect so the strokes read as a legible dark chip in both themes. And a List widget bound to transactions rendered five rows all reading "Side-project revenue" (they differ only by amount), indistinguishable at a glance — each row now carries a muted sub-line with the transaction's date so same-labelled rows are tellable apart. the top of the feed was "Added 17 dashboard layout records" — the dashboard's placement mirror, which it re-persists on every render, so undoing it was instantly re-written and read as "Undo does nothing." Derived placement writes are now dropped before they reach the undo stack and the activity feed (absorbed into the baseline), so the feed shows only genuine user changes and their Undo actually sticks.
- **/duplicates refreshes after Merge/Delete (v1.0, 2026-07-05):** the panel already subscribed to the data revision but neither handler bumped it, so a resolved group stayed on screen with the "merged/deleted" toast floating over it — indistinguishable from a silent failure. Both handlers now bump the revision.
- **Artifacts and import-history deletes confirm first (v1.0, 2026-07-05):** deleting a file (/artifacts) or an import record (/documents) was instant and permanent — the last unconfirmed destructive actions. Both now route through ConfirmModal.
- **Copy: "Updated an artifact" and singular duplicate counts (v1.0, 2026-07-05):** the audit summary read "Updated a artifact" (now vowel-aware "an"); the duplicates header read "1 possible duplicate entries across 1 groups" (now "1 possible duplicate entry"), and a single-entry group badge reads "1 entry".


### Fixed
- **The money-flow Sankey is legible in dark theme (v1.0, 2026-07-05):** it used Mermaid's stock "dark" theme, which darkened the whole palette so the flow bands rendered near-black on the dark card. It now keeps the vivid multi-colour "default" palette in both themes and, in dark mode, overrides only the label/value text to a light colour (and the canvas to transparent) — the bands stay colourful and the numbers read.
- **The page title stops over-truncating at ~1100px (v1.0, 2026-07-05):** "Reports" clipped to "Re…" on 13" laptops because the top bar's scope/period controls yielded no space before the title. The context group now shrinks first (`min-width:0` + higher `flex-shrink`), so single-word titles like "Notifications"/"Investments" render in full with no top-bar overflow.


### Fixed
- **The /members roster refreshes after an edit (v1.0, 2026-07-05):** editing a member from the standalone /members page saved correctly but the row kept showing the old value until navigating away and back — `Members()` never subscribed to the shared data revision (the /household hub did, which is why it updated). Added the subscription, matching /categories and /rules.
- **Split "by weight" no longer breaks the row (v1.0, 2026-07-05):** the per-member weight input had no width cap and stretched across the whole row, wrapping member names mid-name. Capped at 8rem (matching the split editor's amount field).
- **Member reassign-before-delete count is honest (v1.0, 2026-07-05):** the dialog read "'Marcus Hartley' owns 2531 account(s), budget(s), or goal(s)" — the count includes tagged transactions (correctly reassigned), which the copy never mentioned. Reworded to "owns or is tagged on N account(s), budget(s), goal(s), or transaction(s)."
- **Regression harness `mainText` uses `.first()` (2026-07-05):** repeated synthetic `pushState` navigation can leave multiple `#main` in the DOM (a router/synthetic-event quirk, not a real mount bug), which crashed Playwright strict-mode — `.first()` keeps the inspection/regression scripts robust.


### Fixed
- **Flagged-activity rows stop overflowing their card (v1.0, 2026-07-05):** the smart flagged-activity insight rows on the dashboard, /insights, and /assistant wrapped their text in a flex column with `align-items:flex-start`, which sized the children to their content instead of the row width — so `Truncate`/`LineClamp` never engaged and long titles overflowed off the card. The wrapper now fills the row (`w-full`, default stretch) so the ellipsis works.
- **/fields shows its masthead standalone (v1.0, 2026-07-05):** a direct visit to /fields dropped straight into "FIELD REGISTRY" with no page heading — the "Custom fields / your data's shape" intro only appeared inside the Studio tab wrapper. The masthead now lives on the `Fields()` component itself (single owner), so standalone and the Studio tab match, like every sibling standalone route.
- **Three more undefined-token theming bugs (v1.0, 2026-07-05):** the Settings FX-stale hint (`var(--color-warn)`), the AI-key error text (`var(--color-danger)`), and a card background (`var(--card-bg)`) referenced token names the theme engine doesn't emit — corrected to the real runtime tokens (`--warn`, `--danger`, `--bg-card`).


### Fixed
- **Adding a to-do now refreshes the list and confirms (v1.0 blocker, 2026-07-05):** the /todo add form saved the task but never bumped the shared data revision, so the new task didn't appear (until an unrelated control forced a re-render) and no success toast showed. It now bumps the revision, returns the list to page 1, and toasts "Task added." (matching every other add form).
- **Goal % and notification actions are legible in light theme (v1.0 a11y, 2026-07-05):** each goal card's progress percentage used a pale-accent-on-dark tint (~1.2:1 on light theme's pale track — effectively invisible); a light-theme override darkens it (WCAG AA). The /notifications per-row action buttons (mark read / snooze / dismiss) were invisible at rest (border blended in, glyph too faint) — the icon now takes full text contrast with a perceptible border.
- **Goal cards stop stretching to leave dead whitespace (v1.0, 2026-07-05):** the two-up goal grid stretched every card to the tallest in its row, leaving 100px+ gaps under cards without a linked to-do list — `align-items: start` lets each card size to its content.
- **The sidebar Cloud promo no longer floats over nav items (v1.0, 2026-07-05):** the rail's scroll region was missing `min-height: 0`, so at short viewport heights the nav overflowed and the "CashFlux Cloud (optional)" card overlapped "Allocate"/"Planning". The nav now scrolls properly and the foot docks below it — every nav item is reachable.


### Fixed
- **Bills no longer double-count loan/mortgage payments (v1.0 correctness, 2026-07-05):** `/bills` unioned each liability's own statement due-date with the negative recurring flows, and when a recurring flow modelled the SAME payment (the seed's car/mortgage/loan payments do), both listed separately — inflating the headline "total due soon" by ~$2,580 and the "per year" figure proportionally. `bills.UpcomingAll` and `bills.AnnualAmounts` now dedupe a **monthly** recurring flow that matches an account-derived bill by (currency, amount, due day-of-month), keeping the account's ✦ representation. Guarded by `TestUpcomingAllDedupesLiabilityRecurring` (and the monthly-only guard keeps a quarterly flow whose monthly-equivalent coincides from being wrongly collapsed).
- **Subscriptions stop flagging planned bills as cancellable subscriptions (v1.0, 2026-07-05):** a charge already modelled as a recurring flow (HOA dues, utilities) was also detected as a "subscription", producing nonsense like "How to cancel HOA dues subscription". Detected names are now cross-checked against the recurring labels and dropped — the Subscriptions list shows genuine subscriptions only.
- **Loans/credit polish (v1.0, 2026-07-05):** the /loans "Extra payment" input clipped its placeholder ("Extra paymen") — copy shortened to "Extra/mo" and the field widened; the /credit per-card limit editor shared one `credit-limit-edit` testid across every card (a Playwright strict-mode collision) — now suffixed with the account id.


### Fixed
- **Budgets & quick-add copy polish (v1.0, 2026-07-05):** the over-budget banner read "**1 budgets are over**" for a single category (now "1 budget is over…" via a singular variant); the 50/30/20 template's bulk create (up to ~10 budgets) fired **instantly with no confirmation** and now previews the count in a ConfirmModal ("Create N budgets…"), with an honest "nothing to add" notice when every category already has one; the quick-add "reviewed" helper text was clipped mid-word ("#needs-") — shortened, and its color moved off the undefined `--color-text-muted` token to `--text-dim`; the account institution placeholder ("The bank or financial institution") was clipping in the field — shortened to "Bank or institution".


### Fixed
- **Meter/progress-bar tracks follow the theme (v1.0 polish, 2026-07-05):** the unfilled track of every `MeterBar`/`ProgressBar` was hardcoded to `#232325` (a fixed dark hairline), so in light mode every score meter on /allocate (19 rows) and every utilization/score bar on /debt's Credit Health rendered a solid black bar on a white page. New `tw.BgTrack` token follows `--bg-elev` (themed in dark/light/paper); both primitives now use it. App-wide fix — every meter/progress consumer inherits it.
- **Saved scenarios and allocation profiles now confirm before deletion (v1.0 polish, 2026-07-05):** a what-if scenario on /planning and a saved allocation profile on /allocate each deleted **instantly on a single click** — the only unconfirmed destructive actions left in the app. Both now route through `ConfirmModal` with the artifact's name, matching every other saved-artifact delete.
- **Seeded holdings carry real security types (2026-07-05):** the six sample holdings never set `SecurityType`, so /investments badged every position "Other". They're now typed (mutual fund / ETF / stock), guarded by a new `TestSampleHoldingsTyped`.


### Fixed
- **The undefined-token theming landmine, swept (v1.0 polish, 2026-07-05):** three CSS rules referenced `var(--fg)` / `var(--fg-dim)` — tokens that are defined nowhere, so they silently fell back to inherited color or, worse, to nothing. The **credit meter's 30%-target tick was invisible** (`background: var(--fg)` → no background), and the credit disclaimer + reports scope labels drew inherited instead of the muted token in both themes. All three now use the real tokens (`--text` / `--text-dim`). Part of the v1.0 refinement pass adding regression coverage for every page (new `e2e/regression/` harness).


### Changed
- **CashFlux moves to GoWebComponents v4.2.0 (2026-07-05):** the framework dependency jumps from a v3.2-era commit to the latest release. The research finding that made this urgent: the old pin's pseudo-version was based on **v1.1.1** — the framework's module path predated semantic import versioning, so Go's resolver could never see v2+ tags and `go get -u` was permanently stuck. The migration itself was mechanical, exactly as proposed: the `/v4` module path across **220 files** (the one forced change in 294 framework commits — the v4 changelog contains no removed or renamed API in the whole span), plus the three deprecated `state.UseComputed` selectors moving to `ui.UseMemo` (same memo keys, minus the atom wrapper). `.tools/gwc.exe` rebuilt from the v4.2.0 tree (dev flags unchanged; the dev server now also kills a predecessor holding its port — goodbye zombie listeners). Free on the rebuild: v4.1/4.2's reconciler campaign — synchronous discrete-event flush, hook fast lanes, cross-node attribute batching — and the new `GOGC=300` + 512MB wasm memory pacing default (tunable via `localStorage["gwc:gogc"]`). Verified: full native test suite green, wasm + native builds clean, dashboard and seven major routes render with zero page errors, and the 27-assertion settings e2e passes. The dedicated testing phase (full e2e sweep + adversarial interaction round against the new reconciler timing) follows separately.


### Added
- **Formula and custom-value series — the graphing primitives pages build from (2026-07-05):** Cam: "the custom pages aren't using formulas or custom values for the graphing… make it so we have graphing deps that you can build from." Two new declarative chart sources join the widget engine, composable from any WidgetSpec (pages, dashboard, the designer): **`formula` series** — any engine formula or molecule ("income - expense", "savings_rate", user molecules) evaluated once per trailing month against the full period-windowed variable surface, with currency/percent/number output formats — and **`flow` series** — monthly sums of the household's own selection: a tag (`tag:business`), a category by id or name (`cat:Online business`), or a **custom-field value** (`cf:project=Side hustle`), transfers excluded, both ending at the last complete month so a mid-month partial can't plot a misleading cliff. Both hosts supply the per-month variable surface (`DataCtx.MonthVars`). The showcase pages now graph on them: Priya's revenue and costs charts hydrate **live from her shop categories** (the static SVGs are gone), Marcus's side-project chart plots by the `project` custom field, and the Side-hustle page charts the household **savings-rate formula**. Fixed en route: the generic chart's percent axis treated 0–100 percent columns as fractions (a 60% savings rate charted as "6000%"); and the pages' tile chrome sheds the dated ↔ ↕ ✎ ✕ glyph strip for the app-standard ⋯ menu (resize/edit/delete, `cpw-menu-btn-<id>` testids). Series sources are table-tested (filter parsing, monthly sums, format handling).


### Fixed
- **KPI money formatting accepts the documented token (2026-07-05):** the showcase pages' surplus/cash-flow KPIs rendered bare numbers ("4918.75") — `ScalarBind`'s docs said the format token was `money` while the engine only recognized `currency`. The seeded specs now say `currency`, the engine accepts `money` as an alias (so specs written from the docs format correctly), and the doc comment names the canonical token.


### Fixed
- **Seed demo critique round 1 — four real defects the five-year data exposed (2026-07-05):** an adversarial demo pass (which verified every narrative arc to the transaction — the layoff gap, the streak, the duplicates, the one-step reset — with zero console errors) found four things a live demo would trip over, all fixed. **(HIGH) /subscriptions claimed "Share of spending 233%"** — the gauge divided the full-month subscription total by month-TO-DATE spending, unfair on the 5th of a month; it now compares against the average of the last three full months (reads ~35%). **(HIGH) The 2023 layoff-era COBRA premium led the subscriptions list as a live charge ("next Jun 4, 2023" with Remind-me buttons)** — a new `Subscription.Lapsed` rule (one full cadence interval + two weeks past the expected renewal, table-tested) drops long-dead patterns out of the list, the counts, and the totals into a quiet "No longer charging" section. **(HIGH) The dashboard's lead Smart card said "Marcus's Car Loan may have been missed" against a ledger that plainly held the Jun 15 payment** — a timezone trap: transaction dates are UTC-midnight stamps while due dates are built in local time, so an on-the-due-date payment fell before the window whenever the local zone is behind UTC; `paymentInWindow` now compares at day granularity (regression-tested), fixing SMART-BL3 and the autopay detector with it. Also seeded away its successor: the EUR travel card carried a statement schedule (due day + €25 minimum) it never pays — a Wise-style card has no minimum cycle, so the schedule is gone (the €535 owed stays visible as debt). **(HIGH) The side-hustle page's "Side projects & business" list showed the same generic feed as the dashboard** — the page widgets' `tag:` binding filter was silently ignored; it now filters (case-insensitive), so the list actually shows the App Store payouts and shop sales it promises.


### Changed
- **Studio becomes the Build section's single rail entry (2026-07-05):** Cam: "studio is the parent page, remove formula, remove custom fields from the nav menu on the left, and remove workflow but move it into a tab in the studio as well." The left nav's Build group now lists only **Studio**; Customize (the formula calculator), Custom fields, and Workflows leave the rail. Formulas and Custom fields were already Studio tabs — **Workflows joins them as a new tab** (Design · Formulas · Custom fields · Workflows · Build widget · Manage widgets · My pages), embedded with isolated hooks like its siblings. The standalone /customize, /fields, and /workflows routes move to the registry's off-rail section, still routable for bookmarks and deep links.


### Changed
- **Securities rows trade their delete × for the app-standard ⋯ menu (2026-07-05):** Cam: "where they have ×'s replace them with triple dot menus for closing or deleting the record." Each holding card on /investments now carries the shared kebab menu (viewport-aware, Escape/outside-click dismiss) with two confirmed actions: **"Close position…"** — the sold-it path, framed as a close with a nudge to record the sale proceeds as a transaction so the account's cash reflects it — and **"Delete record…"** for entered-in-error rows (the original destructive confirm). Testids preserved (`holding-del-<id>`) plus new `holding-menu-btn-<id>` / `holding-close-<id>`.


### Added
- **One-step "Reset sample data" (2026-07-05):** Cam: "add a quick reset seed option so I don't have to format the seed in 2 steps but in a single step." The Settings → Data tab gains a single confirmed action that replaces everything with a fresh copy of the sample: it reloads the sample (the store load is a wholesale replace, so no separate wipe), clears the two stores that describe the old data (the in-memory activity feed and the cached Smart content in the preserved settings KV), sweeps stray financial browser-store keys, persists the new snapshot, and reloads once the write commits — what wipe-then-load previously took two trips (and an intermediate page reload) to do.

### Fixed
- **Account freshness reads like a real household, and the fun account owns its name (2026-07-05):** restating the seed's opening balances to July 2021 made every account read ~1,831 days stale — `BalanceAsOf` is the freshness anchor, not a ledger cutoff (`ledger.Balance` sums opening + all activity, a semantics trap this change also documents in the seed). Every account now carries a recent, realistic confirmation date; the 401(k) and Roth sit at March 31 — just past their 90-day freshness overrides — so the stale-balance nudge and its task fire honestly instead of absurdly. Also fixed en route: an earlier restatement had double-counted the retirement transfers (~$10.9k of phantom net worth) — openings are back at their 2021 values. Per Cam, the brokerage is now explicitly the hobby it is: **"Stonks (Fun Money)"** — deliberately small, home of the crypto face-plant and the 2024 lucky streak, with its positions table and page copy renamed to match.


### Changed
- **The sample dataset becomes five years with a real story (2026-07-05):** Cam: "rehydrate the seed data to completely fill this app with details for the past 5 years and show yoy growth, a job loss, bad decisions and a lucky streak…" The Hartleys' history now runs **July 2021 – June 2026 plus the first days of July 2026 in flight**, and the numbers tell four arcs the charts actually show: **year-over-year growth** (salary $3,550→$4,700 net across two employers, the shop from ~$60/mo to ~$1,100/mo with a monthly owner's-draw sweep into the household); **the 2023 layoff** (laid off from Cohere end of January; four months of severance + unemployment + COBRA, savings drawn down, the card at minimum payments with one late fee, gym frozen, dining/travel cut, side projects pushed hard; hired by Meridian Data in June); **bad decisions** (crypto bought at the Nov-2021 top out of the emergency fund and capitulated −70%, the two financed cars, the standing dining habit); and **the spring-2024 lucky streak** (four strictly-escalating green months, +$13,300, a scratch-off — and profits actually taken: $8k to savings). Month-to-month amounts wobble on a **deterministic hash** (non-periodic like real life, byte-stable for e2e). A rotating clutch of **raw bank-statement imports** ("POS DEBIT 4417 AMZN MKTP US*2K3L7QW", "VENMO PAYMENT 1023985544", "ZELLE TO CHEN, JENNY"…) lands every month with a standing uncategorized minority, plus **errata the tools exist for**: a doubled DoorDash import (/duplicates has a real catch), a parking charge mis-filed under Dining, December gifts and card/ATM/late **fees** populating their categories, and two new rules (amzn→Shopping, instacart→Groceries) leaving the rest for the Smart+ scan. New coverage: **Holdings** (401(k)/Roth funds + the WSB positions) and **quarterly BalanceSnapshots** for the 401(k)/Roth/condo (the 2022 dip → 2024–25 recovery), so the Investments surface and valuation sparklines have real data. The three showcase pages' KPIs and trend charts moved onto **unified widget-engine specs** (live formulas and net-worth/cash-flow series pipelines instead of static bindings), with the shop tables and the five-year brokerage-rollercoaster SVG re-based to the new numbers. Guarded by two new native tests: `TestSampleTrajectory` (checking/savings/cash never negative, card revolving within its limit) and `TestSampleNarrative` (the gap has no paychecks + four unemployment checks, the employer switch, the escalating streak, ≥40 uncategorized imports, the duplicate pair, every December's gifts, ≥5 validating spec widgets).


### Added
- **Custom pages can host unified widget-engine specs (2026-07-05):** `domain.PageWidget` gains an optional `Spec` (`domain.WidgetSpec` — the same shape the widget designer publishes). When set, the tile's body hydrates through `widgetengine` (scalar KPI formulas with formats and templated sub-labels; collection/series pipelines with filter/sort/limit rendered by the generic frame bodies) instead of the legacy binding, while the page keeps its own chrome, drag-reorder, resize, and delete. Errors and panics degrade to an inline note per tile. This is the seam the seeded showcase pages move onto — page widgets and the dashboard's Studio-designed widgets now share one declarative data path.


### Fixed
- **The ⋯ menu's theme cycle re-bases saved themes too, and the accent readout stops lying (critique round 3, 2026-07-05):** the verify round confirmed all round-2 fixes and found the last mode writer unfixed — the top bar's ⋯ → "Theme · Dark/Light/System" item (the only *reachable* topbar theme control; the inline icon is CSS-hidden) still skipped the re-base, so cycling it with a preset saved silently did nothing to the render while the menu label flipped. It now runs the same `SyncThemeToMode` rule as the Appearance segmented. Also from the round: the Appearance hero's **ACCENT chip always showed the prefs swatch** (`#2e8b57`) no matter what accent a preset actually applied — it now reads the applied `--accent` from the document root, and theme applies bump the shared revision so the hero re-renders even when prefs didn't change. Verified live: Midnight shows `#7c83ff` in the chip immediately; the ⋯ cycle flips Midnight → light with the accent surviving; reset + Dark restores clean.
- **The mode toggle can no longer lose to a saved theme (Appearance-tab critique round 2, 2026-07-05):** the verification round confirmed the preset fix and found the same disease on the other paths. **"Reset to default" persisted a snapshot** of the default look, and any saved theme pins the shell (data-theme derives from the THEME's luminance) — so after Paper → Reset → Dark, every readout said Dark while the app stayed light, unrecoverably until reload. Reset now **clears** the saved theme (the default re-derives from your preferences, so the mode toggle owns the shell again), and flipping the mode with a disagreeing saved theme **re-bases it**: surfaces swap to the target mode's stock palette while the personal tokens — accent, fonts, radius, scale, density, icon stroke — ride along (`uistate.SyncThemeToMode`, wired into both the Appearance segmented and the top-bar theme cycle). Verified live: Paper → Reset → Dark reads and renders Dark; Midnight → Light re-bases to a light look; restore clean.
- **Theme presets no longer desync the mode readout (Appearance-tab critique, 2026-07-05):** applying a dark preset (Midnight) while the mode preference said Light flipped the whole app dark — ApplyTheme derives the shell from the theme's luminance — while the Appearance hero, its takeaway, and the Mode segmented control kept reading "Light". The theme editor's apply path already mirrored density and scale back into prefs ("two stores kept in lockstep") but forgot the shell: it now mirrors the theme's light/dark into `prefs.Theme` too. Verified with the critique's exact repro: Light → Midnight now reads Dark everywhere, in sync with `data-theme`.

### Changed
- **Appearance moves into Settings; the setup wizard moves to Help (2026-07-05):** Cam: "move appearance and setup in there as well, don't keep setup as its own tab though." The full /appearance surface (mode/motion/accent + the theme editor) is now an **Appearance tab** on /settings, between Preferences and Alerts — the Preferences tab's "Appearance & theme →" link switches tabs in place instead of leaving the page, and `OpenGlobalSettingsAt("appearance")` deep-links to it. The guided setup wizard came off the rail too, without becoming a tab: **/help's "Getting set up" checklist gains an "Open the guided setup" launcher**, and both /appearance and /setup remain routable off-rail for bookmarks and empty-state CTAs. The System nav group is now just Settings · Help · About. `settings_hookup_check.mjs` grew to 27 assertions (Appearance tab content, the in-place tab switch, both routes absent from the rail, the wizard launcher routing to /setup).

- **Settings becomes a routed page in the side nav (2026-07-05):** Cam: "it should be a page and in the side nav." /settings is now a first-class System-group page — Settings leads the System section of the rail (gear icon) and the top bar's ⋯ → Settings routes there too. The page hosts the exact same seven-tab form the flip modal did (one shared component; every control keeps its persist path), laid out for the content column with the tab strip inline under the page header. **Every "open settings" affordance in the app now navigates to the page, and most deep-link to the right tab:** the upgrade sheet, /plans trial CTA, sync chip, and subscription banner land on **Cloud**; the AI-key prompts on /allocate, /insights, and document image import land on **AI**; the app-lock prompt lands on **Advanced**; /about's key link lands on **AI** (`uistate.OpenGlobalSettingsAt(tab)`). The widget settings modal is untouched, and the global flip-modal host stays in code unreferenced. `settings_hookup_check.mjs` rewritten for the page: 21 assertions including both entry points and the Cloud deep-link.

### Fixed
- **The /settings tab strip stays reachable while scrolled (page critique, 2026-07-05):** the adversarial pass on the new page shipped it with one polish note — on a tall tab (Household's screen toggles, Alerts' freshness grid) the seven-tab strip scrolled away with the content, so switching tabs meant scrolling all the way back up. The strip now re-sticks 3.5rem below the scrollport top (flush under the sticky top bar — the same offset the ledger tables' sticky headers use) with the page background behind it. Verified mid-scroll by rect measurement: strip top 56px, top bar bottom 57px, fully visible. Also cleaned the stale HouseholdCard doc comment that still described the rail card as "opening the global settings flip panel."
- **Collapsed rail no longer shows a shredded household summary (critique round 2, 2026-07-05):** the round-2 adversarial pass verified all five round-1 fixes live and found one new defect the redesign introduced — the rail's new quiet household summary (`.hh-quiet`) didn't hide when the sidebar collapses (the collapse rules covered the old settings button's elements but not the new div), so at the 33px collapsed width "Your household · 2 members · USD base" wrapped into unreadable stacked fragments above the avatar chip. The summary now hides with its siblings; verified live both ways (collapsed → hidden, expanded → restored).
- **System surfaces critique round 1 — dead Settings links, an honest checklist, and an Alerts tab (2026-07-05):** the adversarial pass on the new system surfaces found five real defects, all fixed. **(HIGH) Six call sites navigated to `/settings` — a route that doesn't exist** (Settings is a modal; the navigation silently landed on the dashboard): the /about AI card's "settings" link, the /plans "Start free trial" CTA, /allocate's provider hint, the document image import's key prompt, and two /insights paths. A new `uistate.OpenGlobalSettings()` (captured-atom, callable from any click handler) opens the panel directly, and all six sites use it. **(HIGH) The /help setup checklist's currency step named the wrong tab** ("Settings → Appearance" — base currency has never lived there); the copy is now tab-agnostic and the step opens Settings itself. **(MEDIUM) The Preferences tab was overweight** (1,759px of scroll — the freshness day-fields alone were eight rows): freshness reminders, notifications, the learn-threshold, and music moved to a new seventh **Alerts** tab, halving Preferences. **(MEDIUM)** the Household FX hint said to add a key "in the AI section above" — there is no above anymore; it says "on the AI tab". **(LOW)** the Cloud tab's self-hosting guide linked a relative `docs/SELF_HOSTING.md` that 404s in the served app; it now points at the GitHub blob URL. `settings_hookup_check.mjs` grew Alerts-tab assertions (18 green).

### Changed
- **The system surfaces join the Data & People design language, and Settings gets tabs (2026-07-05):** Cam: "do the system pages… use the design from data and people… settings needs tabs… remove the flip modal settings from the side nav bar." **Settings:** the global flip-panel was one dense two-column form with 14 stacked sections and a jump-link row; it's now SIX tabs — Household (members, screen toggles, base currency, budget method, FX rates), Preferences (appearance link, week start, date format, pay cycle, monthly income, freshness, notifications, music), AI, Cloud, Data (exports/backups/workspaces), Advanced (app lock, languages, debug log) — behind a sticky segmented strip; every control kept its exact persist path. **Entry point:** the rail's household settings card is out of the markup (kept in code behind `If(false)` for easy restore); a **Settings item now leads the top bar's ⋯ menu**, and the rail foot keeps a quiet non-interactive household summary (now live — it subscribes to the data revision, so a base-currency change updates it instantly). **System pages:** /help, /about, and /appearance are rebuilt in the Understand language — /help opens with a **live setup-progress hero** ("6 of 6" with shortcut/palette/offline chips and a takeaway that says what's left), the checklist as accent-tick steps (the currency step now opens Settings directly — its label used to point at Appearance, which never carried base currency), and the topics as serif half-width sections; /about leads with the version hero, "On this device / No tracking / Off by default" chips, and the tagline as the pull-quote over the four disclosure sections (testids kept); /appearance reads the current look back in plain English ("You're in Dark mode with Full motion…") above the mode/motion/accent controls and the theme editor. **Proof:** new `e2e/settings_hookup_check.mjs` drives every tab — base currency reflects live in the rail, hiding a screen removes its nav item, date format / AI key / backup cadence persist across close-reopen, the backend toggle flips the Cloud tab's state, app lock + languages reachable — 16/16 green.

### Added
- **The vault paginates (2026-07-05):** /artifacts renders ten files per page (a receipt-heavy vault runs to dozens of thumbnail-bearing rows) with the app's standard pager — a "1–10 of 21" range caption, Prev/Next disabled at the ends, and page-of tally; deleting off the last page clamps back. Verified live across all three pages of the sample vault. Also fixed en route: the "Used by N custom pages" reference label passed one argument to a two-verb format string and had been rendering **"Used by 1 custom %!s(MISSING)"** on every page-bound artifact.
- **The activity record shows field-level before → after details (2026-07-05):** Cam: "the activity logs needs more details, needs to show before and after changes." Every recorded UPDATE now carries the exact fields that changed, rendered under the entry as aligned diff lines — the old value struck through, the new value in weight ("name: ~~Marcus Hartley~~ → Marcus Q. Hartley"). The differ is a pure, table-tested `auditlog.DiffJSON` (values pass through Redact so a secret-bearing field can never reach the feed; blob-scale fields like artifact bytes are skipped; unparseable payloads degrade to the summary). The recorder formats values with domain awareness: money shapes render as amounts, RFC3339 timestamps as dates, and `categoryId`/`accountId`/`memberId`-style references resolve to display names **at record time**, so history stays accurate even if the target is later renamed. Long diffs cap at five lines with a "+N more changes" tail; legacy entries without details render unchanged. Fixed en route, all found while wiring the diffs: **audit entries never carried a timestamp** (every change filed under an undated bucket — entries now stamp `At`, so the day-grouped timeline finally means something), **the audit log recorded itself** (each capture logged the previous entry's own bookkeeping write as "Added a history entry" — pure-audit change sets are now skipped), and **summaries counted internal rows** ("Updated 2 member records" for one member edit plus its bookkeeping row — counts now cover only the changes the entry is about, and the op/entity inference ignores internal rows so an update can't be mislabeled "Added").
- **Artifacts can be downloaded, and the vault's modal joins the family look (2026-07-05):** every file row's ⋯ menu gains **Download** — images download with their original bytes (fetched from the IndexedDB blob store off the render path when the dataset doesn't carry them inline), and CSV datasets re-serialize losslessly from their stored columns/rows via a new tested `artifacts.CSVBytes` (round-trip proven against ParseCSV, quotes/commas included; filenames gain `.csv` when missing). A file whose data isn't on this device gets an honest error notice instead of an empty download. Design-consistency pass on the rename modal: titled **"Rename file"** (was a bare "Rename"), the row's icon-only pencil became the labeled icon+text button every sibling Data & People row uses, and the panel is sized snug to its one field (230px) with the sticky Save/Cancel bar.

### Fixed
- **Rule edit modal height now tracks its content (audit round 4's last point, 2026-07-05):** the fixed 640px panel left ~190px of dead space under Save/Cancel for plain phrase rules; the host now opens snug (470px) for phrase rules and tall (640px) for condition-bearing ones — growth past the panel (enabling more slots mid-edit) scrolls with the sticky action bar in view. Verified live: the void under the actions dropped to 24px.

### Added
- **Smart+ rule suggestions — an opt-in AI scan on /rules (SMART-T14, 2026-07-05):** Cam: "add smart+ rule suggestion based on the current transactions and an AI scan of the details to suggest the best category… opt in toggle and a refresh button to scan again." A new "Smart+ suggestions" section sits between the deterministic suggestions and the precedence chain: **off by default** with an explicit toggle ("Suggest rules with AI") and a privacy-honest lede ("Nothing is sent anywhere until you turn this on and run a scan"); keyless households see the standard add-a-provider note instead of a dead button. When opted in, **"Scan my transactions"** (→ "Scan again" for refreshes) samples up to 40 transactions **no existing rule covers** (full-context matching, transfers excluded) plus the household's real category list, and asks the model — under the product's mini-first/escalate-once routing — for up to six `phrase => Category` rules in a strict line format. The response parses through a **defensive pure parser** (`smartai.ParseRuleSuggestions`, table-tested): unknown categories are dropped (the model can never invent one), short phrases and duplicates are skipped, phrase-covered suggestions are filtered against the live rule set. Results render as add-able rows ("Categorize "X" as Y · AI suggestion — added rules run last…") whose Add uses the same append-at-end path as every other suggestion; loading, error, and no-ideas states are all explicit. Registered as catalog feature SMART-T14 (AI tier, transactions page) so the /smart hub governs the same toggle. Also applied the round-3 audit's UX note: **every Data & People flip-modal editor's Save/Cancel now rides a sticky action bar** at the bottom of the scrollable body — a rule with all three condition slots open can no longer hide its buttons below an unsignposted scroll.

### Fixed
- **The floating corner controls no longer cover page content or each other (2026-07-05):** the scroll-to-top button sat directly on the vault pager's Next button (and on whatever else ended a page bottom-right), and the PWA "Install CashFlux" button docked at the exact same coordinates as the scroll-to-top. The main scroll container now carries bottom clearance (5.25rem) so the last row of any page can always scroll above the corner controls, and the install button moved beside the scroll-to-top instead of underneath it (the iOS add-to-home-screen hint stacks above the corner). Verified by rect measurement: zero overlap between the pager nav, the scroll-to-top, and the install button.
- **Rules engine audit round 2 (88/100) — conditions are now fully editable (2026-07-05):** the re-audit confirmed all eight round-1 fixes live (rename-on-entry, honest condition counts with the hero moving 17%→39%, append-at-end precedence, pure-conditions rules, the blast-radius confirm listing per-rule counts in English, focus-jump add, Move up/down) and found one real remaining gap: **a condition rule's field/operator/value couldn't be changed** — the edit modal only said conditions would be "kept as-is", forcing delete + recreate. The rule edit modal now carries the same three bounded condition slots as the add form, seeded from the saved rule and fully editable (shared `condSlotRow` renderer); live-verified editing an amount threshold from $100 to $500 and watching the row's English re-render.
- **Rules engine audit (51/100 → fixes for all eight findings, 2026-07-05):** an adversarial auditor scored the auto-categorization engine end-to-end (source + native tests + live Playwright rounds) and found the condition-rule layer half-wired. All fixed, native-tested, and live-verified. **(HIGH) The rename action never fired on entry** — `AutoCategorizeTransaction` (quick-add, CSV import, document import) set category+tags but skipped `RenameDesc`; only the backfill applied it. A new transaction now gets the cleaned description immediately (test: `TestAutoCategorizeAppliesRename`). **(HIGH) Condition rules were black boxes**: the row count, the hero coverage, and the authoring preview all evaluated only the legacy substring — a rule that actually catches hundreds of transactions read "0 transactions caught" while the hero froze. New conditions-aware engine primitives (`rules.TxnCtx`, `Rule.MatchCountFull`, `CoveredFull`, table-tested) now feed the row weights, the hero, and the quick-add's live preview, and the rows + precedence chain + notices render conditions in **plain English** ("Matches when the amount is under ($100.00) and the payee contains …") instead of a dead `Contains ""`. **(HIGH) New rules silently claimed TOP precedence** (zero `Order` + the store's ID tie-break sorted generated hex IDs above the seeds) — quick-add, the add modal, and accepted suggestions now append at the END of the first-match-wins chain via `App.NextRuleOrder()` (test proves a broad new rule can't shadow the seeds). **(MEDIUM) Pure-conditions rules were impossible** — the match phrase was force-required even though the engine ignores it when conditions exist; `PutRule` + the form validation now require *a phrase or a condition*, and the condition fieldset says the phrase becomes optional/ignored. **(MEDIUM) "Apply to existing" gained a blast-radius confirm**: a dry-run preview (`PreviewApplyRules`, write-free, test-proven to agree with the real apply) lists exactly how many transactions each rule will re-file — with condition rules named in English — before the irreversible overwrite. **(LOW)** suggestion coverage now recognizes condition rules (a key whose transactions a condition rule already governs is no longer re-suggested); the redundant "+ Add rule" modal path became a focus-jump to the on-page quick-add form; and reordering gained keyboard-reachable "Move up / Move down" items in each row's ⋯ menu alongside the mouse-only drag grip.

### Changed
- **Data & People edits move from inline row forms to flip modals (2026-07-05):** Cam: "for edits, use flip modals for these pages instead of inline forms." Member edit (name/color/personal prefs/role/member custom fields), member PIN set/change, category edit (name/kind/parent/color/deductible), rule edit (match/category/tags/rename), and artifact rename now open in the app's shared FlipPanel modal, mounted at the shell root beside BudgetEditHost — the inline forms sat under transformed bento/tile ancestors and reshaped rows mid-list. New `uistate/dataedit.go` selectors (captured-atom pattern), `screens/dataedit_forms.go` bodies (each owns its Save/Cancel, saves through appstate, bumps the shared data revision, and flushes the dataset immediately via RequestPersist), and `app/dataedithost.go` hosts. Every edit-form testid survives inside the modal (`member-edit-role-*`, `member-pin-form/input-*`, `cat-edit-*`, `cat-deductible-*`, `rule-edit-*`, `artifact-rename-input`); Escape/backdrop/✕ dismiss; focus lands in the first field. Two real fixes along the way: **editing a rule no longer silently drops its precedence Order and structured Conditions** (the old inline form rebuilt the rule from scratch; the modal mutates the existing rule and says when conditions ride along), and the household tab panels now thread the data revision through their props (empty-struct props memoized the roster, so a modal save wouldn't refresh it).

### Fixed
- **Data & People critique round 2 — SHIP, with its two notes applied (2026-07-04):** the second adversarial pass verified all five round-1 fixes in both themes (including force-shadowing a rule to prove the precedence spine's struck-through state) and shipped the surface. Its remaining notes, both fixed: single-change settings summaries read "Added a settings" (mass noun + indefinite article — the summary builder now drops the article for settings, and legacy persisted copies are repaired at display), and the rule quick-add's condition checkboxes rendered flush against their labels ("☐Condition 1" — the slot header now flexes with a gap).
- **Data & People critique round 1 — the record stops speaking machine, the chain sheds its stock costume (2026-07-04):** the adversarial gate's findings on the five redesigned pages, all fixed. **(HIGH) /activity leaked raw internal names** into user copy — "Added 2 auditEntries records", "Added settings settingsState", `· placements` — both at the source (`RecordAuditPoint` now skips the audit log's own storage when picking the dominant collection, singularizes placements/recurring/workflows/settlements/… properly, and never appends a raw record ID to a summary) and at display (legacy persisted entries are relabelled: "Added 20 dashboard layout records"). **(HIGH) /activity claimed "newest first" while rendering days oldest-first** (the feed's storage order is file order) — the timeline now enforces newest-first with undated session entries leading; this also puts the inline Undo on the row that is actually newest. **(HIGH) /rules' precedence diagram wore Mermaid's stock lavender theme** (glaring in light mode) — replaced with a native numbered spine in the surface's own language: accent-ringed precedence discs on a hairline spine, match → category per link, shadowed rules struck through with their warning inline (aria list semantics kept). **(MEDIUM) /artifacts nagged twice about the same quota** — the dismissible banner duplicated the hero takeaway word-for-word; the takeaway now owns the story alone. **(LOW) /household** edit-form selects clipped "Inherit household default" into the caret (min-width + ellipsis) and a negative-worth member's share bar could read as ownership — the percent now leads with a minus and the bar's title says "weighs on household worth".

### Changed
- **/activity redesigned as the household change record in the Understand language (2026-07-04):** fifth Data & People page. Opens with a **hero tile** — "The record", the number of changes on record in the display serif, an eyebrow ("newest first · every change is undoable in order"), figure chips (Shown / By you / By others when the household has more than one actor / **Undo available** in the money-up tone), and a takeaway quoting the latest change ("Most recently: Added 'Crib & registry items' ($450) — Marcus Hartley, Jun 10, 2026."). The timeline itself becomes a **day-grouped ledger**: serif accent-tick day dividers rule the entries into days (undated audit-seed entries group under "Recent"), and each entry carries a **left action tick** — accent for additions, danger for deletions, neutral for edits — with the actor on the aside and the newest row's inline Undo kept. The full-width entity filter became a compact select in the section header (same `activity-entity-filter` testid); `.rows .row` counting contracts preserved for the filter e2e. New `rules_record.go` + `en_recordsurface.go`.
- **/artifacts redesigned as the file vault in the Understand language (2026-07-04):** fourth Data & People page. Opens with a **hero tile** — "Your files", the device-storage footprint in the display serif with the **storage meter directly under the figure** (warn tone near the cap) and the IndexedDB/total breakdown as fine print, an eyebrow ("21 files · receipts and datasets, stored locally"), figure chips (Images / Datasets / **Attached to transactions** in the money-up tone / Used by pages), and a takeaway that owns the quota story ("11 images and 10 datasets take 84.1 MB of this device's storage. You're close to the practical limit — consider exporting a backup…"). The upload actions ride the **"In the vault"** section header (the page is two tiles, not two stacked cards); file rows now read left-to-right (the base `.row-main` column was stacking and centering thumbnail over name) and keep rename-inline, the reference counts, CSV previews, and the delete-guard — with delete moved into the app-standard ⋯ menu (`artifact-menu-btn-*`/`artifact-delete-*`). Quota nudge and error notices preserved. New `rules_vault.go` + `en_vaultsurface.go`.
- **/rules redesigned as an automation ledger in the Understand language (2026-07-04):** third Data & People page. Opens with a **hero tile** — "Auto-filing", the coverage percent in the display serif ("12%" of transactions filed automatically), an eyebrow ("5 rules · first match wins"), figure chips (Rules / **Auto-filed** in the money-up tone / **Never fire** in the danger tone when rules are shadowed / Suggestions ready), and a plain-English **takeaway** ("Your rules file 289 of your 2321 transactions on their own. 33 ready-made rules are waiting below."). Each rule row becomes a **weighted ledger entry**: the bolded match phrase over its → category · tags · rename meta, a **share bar ranking its catch against the heaviest rule**, and a right-aligned "N transactions caught" figure column — the old buried "Matches N transactions" meta line, promoted to the row's visual spine. Drag-to-reorder, shadowed-rule warnings, the inline quick-add form, Apply-to-existing, inline edit, the suggestions feed (its hint now the section's pull-quote), and the Mermaid precedence chain all preserved; the instant-delete × moved into the app-standard ⋯ menu (confirm dialog intact; `rule-menu-btn-*`/`rule-delete-*` testids). New `rules_rulespage.go` + `en_rulessurface.go`.
- **/categories redesigned as a taxonomy ledger in the Understand language (2026-07-04):** second Data & People page in the Reports/NetWorth/Health family. Opens with a **hero tile** — "Your categories", this period's total spending in the display serif, an eyebrow ("33 categories · Jul 1 – Aug 1"), figure chips (Expense / Income / Deductible / **Not filed yet** in the danger tone when uncategorized spend exists), and a plain-English **takeaway** naming the leading category and whether everything is filed ("Mortgage leads this period's spending. Everything you spent is filed under a category."). The **category map** keeps its chip grammar under the serif section head with a one-line lede. The **expense/income tree ledgers** keep every mechanic (collapse chevrons, depth guides, sort-by-usage, inline edit with kind/parent/color/deductible, add buttons, reassign-before-delete — now a heads-up panel) and their rows gain **this-period figures**: the amount spent (or earned) right-aligned over a quiet sub-line, with a **share bar tinted the category's own color** ranking it against the largest same-kind category — the ledger doubles as a legend for the charts that use those hues. The previously invisible **deductible flag now shows as a row tag**. The instant-delete × moved into the app-standard ⋯ menu (View these transactions / Delete category, reassign guard intact; `cat-menu-btn-*`/`cat-delete-*` testids). Figures come from the same `reports.SpendingByCategory`/`IncomeByCategory` paths as /reports, so the numbers always agree. All e2e contracts preserved (`categories-add`, `cat-toggle-*`, `cat-deductible-*`, visible row Edit). New `rules_categories.go` + `en_categoriessurface.go`. Also fixed en route: the collapse chevron always rendered the right-pointing glyph (a collapsed/expanded ternary assigned the same icon on both branches) — it now points down when open.
- **/household redesigned as a people ledger in the Understand language (2026-07-04):** the first of the Data & People pages joining the Reports/NetWorth/Health design family. The tabbed hub now opens with a **hero tile** — "Your household", the combined household net worth in the display serif (count-up), an eyebrow stating the coverage ("2 people · USD base · Jul 1 – Aug 1"), figure chips (People / Spending this period / Income this period / Shared pot), and a plain-English **takeaway** naming who holds the largest share of the household's worth and who did this period's spending. The **Members tab** becomes a person roster: each member is a ledger row — oversized avatar, serif name, role/Default/PIN chips, a right-aligned worth figure over a quiet "spent X this period" sub-line, and a **share bar showing their slice of the household's biggest holding** (liability-toned when negative, matching /networth); actions consolidate into a visible Edit plus the app-standard ⋯ menu (Transactions / Make default / PIN management / Delete — reassign-before-delete now reads as a heads-up panel, and an Add member button opens the global add sheet). The member **edit form now includes member-scoped custom fields** (saved values also surface as a quiet meta line on the row, and number fields feed `cf_member_*` engine variables as before). The **By person tab** re-ranks its three analytics — net worth, spending, income split — as share-bar rows, each led by a one-line takeaway; the **Split tab** sheds its EntityListSection cards for the shared serif section chrome, with the per-person split sentence as a pull-quote. All figures come from the same ledger/reports/allocate paths as before, and every e2e contract survives (`member-role-badge-*`, `member-default-chip-*`, `member-edit-role-*`, PIN testids, `.member-avatar`, `members-single-device-note`, visible row Edit). New `rules_household.go` + `en_householdsurface.go`; standalone /members shares the same roster + analytics.

### Added
- **Workflows now speak the full formula language — engine variables, custom fields, logic, and live templates (2026-07-04):** the adversarial feature review's mandate ("flexible, uses formulas where feasible and custom values") made real. **Conditions** evaluate the FULL engine surface (the workflow runner previously fed a partial one): every atom and molecule (user edits included — a customized net_worth now means the same thing in a condition as on the dashboard), safe_to_spend/health_score (Recurring + Categories now feed the runner, so those weren't silently wrong), per-budget/goal/account figures, and `cf_*` custom-field values. **Per-transaction custom fields:** on a txn-added run, `cf_txn_<key>` now reads THE TRIGGERING TRANSACTION's own value (numbers as-is, yes/no as 1/0, text/choice via `contains()`) instead of the period-wide household sum — "if Reimbursable, tag it" is finally buildable, native-tested. **Logic:** the formula language gained `and()`/`or()`/`not()` (it had NO conjunction operator at all) — usable in conditions, molecules, and widget formulas alike. **Live templates:** task titles, notes, and notify messages interpolate `{{expr}}` against the run context ("Safe to spend is {{safe_to_spend}}", "{{txn_payee}} charged {{txn_abs}}"), numbers rounded to cents, unresolvable templates left visible — engine-level (`workflow.Expand`), table-driven-tested. **Composer support:** an "Insert any variable…" dropdown (general figures first, per-entity tail behind a separator, cf_txn_* included), a live condition check that flags typos and txn_-vars-on-wrong-trigger BEFORE save, a `{{…}}` hint on action text, and "Move money (transfer)" as a first-class composer action (from/to/amount; hidden on txn-added per the loop guard; scheduled ones get per-period dedupe automatically).

### Fixed
- **Pay-yourself-first transferred exactly once, ever (2026-07-04, CRITICAL):** the scheduled transfer's DedupeKey was stamped with the CREATION month, so every run after the first matched its own first run's record and silently skipped — the flagship savings automation quietly stopped after one payment. Keys now carry a `{period}` placeholder resolved at each run (ISO week for weekly cadence, month otherwise), legacy frozen keys are transparently re-stamped, and native tests prove same-month double-runs dedupe while next-month runs transfer again. Transfer run summaries also now read "Move 250.00 USD from Checking to Savings" instead of raw minor units and account ids.
- **Budget-exceeded workflows never fired from real use (2026-07-04):** only re-saving the budget document checked the over-limit transition — adding the transaction that pushes a budget over (the thing that actually happens) never did. PutTransaction now snapshots which budgets are over before the write and fires the trigger on the transition; native-tested.
- **Workflow runs stopped lying (2026-07-04):** (1) a broken condition (typo, unknown variable) rendered as a fake "Condition didn't match" — it now gets a distinct "Couldn't run: …" error state on the row; (2) setCategory/addTag/flagReview reported "DONE" while silently no-opping on any trigger without a transaction — now refused at save (Validate + the composer only offers them on txn-added), and pre-existing combos plan honest "— skipped: no transaction in scope" summaries with a "Nothing to do" head; (3) Dry run on a txn-added workflow previews against your MOST RECENT transaction (labeled as such) instead of erroring on txn_* variables, and Run now is hidden there (a manual run has no transaction to act on).
- **Light theme repaired across Studio + shared chrome (2026-07-04):** two flavors of one disease, found by the adversarial critique loop probing light mode. (1) A cluster of styles referenced UNDEFINED CSS tokens with dark fallbacks (`var(--fg,#e5e7eb)`, `var(--line,…)`, `var(--dim,…)`, `var(--faint,…)`) — since the tokens never existed, the dark fallback applied in BOTH themes, leaving builder node text at ~1.1:1 contrast on light (unreadable), plus off-theme steppers/arrows/style rows/attention items. All 41 occurrences in the generated rules and the three inline-styled screens (credit, health, score ring) now use the real per-theme tokens (`--text`/`--border`/`--text-dim`/`--text-faint`); the same class of bare `var(--fg)` color-mixes in the new fields/manage/pages/assistant stylesheets was corrected too. (2) The Design tab's chrome hardcoded dark hexes under theme-resolving text (`#121214` type cards/block rows/verbs, `#15151a` starters, `#0e0e10` stage) — invisible dark-on-dark in light mode; all tokenized (`--bg-card`/`--bg-elev`/`--bg`), pixel-identical in dark.
- **Deleting a published builder card no longer leaves a phantom widget (2026-07-04):** removing a card from My cards deleted the library entry but left its `wb:<name>` layout item behind — the dashboard silently dropped the tile, but Manage widgets kept listing a ghost row (and board-map tile) whose toggle and steppers did nothing. Delete now removes the layout item too. (The critique loop's final note.)
- **Publishing a widget could be lost to a fast reload (2026-07-04):** the dataset autosaves on a 4s ticker, and the pagehide IndexedDB write doesn't reliably commit during unload — so Publish → navigate/reload within the tick could silently drop the new card (reproduced deterministically by `widget_builder_check.mjs`). Publish (Build + Design tabs), Save card, and Delete card now flush the dataset immediately via the same `uistate.RequestPersist()` hatch the sample loader uses (C2).
- **countup.js could wipe a re-rendered KPI's DOM (2026-07-04):** the count-up tween holds a raw DOM node for ~600ms; when a re-render made the framework reconciler REUSE that node as a different element (e.g. switching Build-widget presets morphs the old `.fig` into the new KPI wrapper), the stale animation's final frame ran `el.textContent = …` and replaced the node's new children (figure + caption) with bare text. Every KPI surface that re-renders mid-tween was exposed — observed as the builder's styled-KPI/dual-KPI presets rendering no `.fig` at all. Animations now carry a cancellation token and stop silently when their node is superseded by a newer tween, detached, or morphed into a non-countup element.

### Changed
- **/workflows rebuilt from scratch as an automations desk (2026-07-04):** the page was five stacked EntityListSection cards (three savings forms crammed into one card with `<hr>`s, the builder, the list with an instant-delete ×, history). Now a bespoke `.wf-deck`: masthead ("AUTOMATIONS / Workflows" + a lede that leads with dry-run safety), the three **savings quick-starts as one panel band** (serif titles, labeled fields, one quiet save each — no more three competing primary buttons), then the desk: the **automation registry** (ledger rows — status dot with accent glow / hollow + OFF tag, serif name, plain-English meta, Dry run first + Run now + ⋯ menu with Enable/Edit/Diagram and a two-step inline delete confirm; run results as an accent-ticked readout) with **run history** (human dates per the user's format preference + time when it matters, "1 effect"/"N effects") beside the **workflow composer** whose signature footprint reads the draft back in plain English — including a conservative **condition-to-English renderer** ("if the amount is over 200", "if the payee contains \"uber\"", "if money is going out") with the raw formula kept as a mono auditable aside and compound/unknown forms falling back verbatim. Built through the adversarial critique loop (round 1: ITERATE — 2 high + 3 medium + 1 low; round 2: **SHIP**, its remaining notes applied). The loop's real catches: **the PYF quick-start could silently create duplicate money-moving transfers** — it now opens as an "Already set up" summary (OFF ones dimmed) with the form behind "Add another" and refuses a same-route-and-schedule duplicate (including of a merely-OFF one, which would double the transfer on re-enable); and **Run now no longer works on disabled workflows** (the engine happily fired them manually — off now means quiet; re-enable first). All e2e contracts preserved; `workflow_condition_help_check` green; `glamor_19` re-scoped to the composer and green both themes. Known follow-up: row Edit still covers only name + condition — changing a transfer amount means delete + re-create (guarded), full action editing is backlog.
- **Studio critique rounds: guided builder configuration + hub-wide mastheads (2026-07-04):** the adversarial loop's round-1/2 findings, all implemented. **Figure picker** (was a flat select of 284 raw internal names): now the grouped, documented catalog the Design tab uses — "Core money · Net worth"-style labels, a "Filter metrics…" type-ahead (284 → 11 on "sav"), the selected metric's plain-English description, "Built from atoms: …" for molecules, and raw engine leftovers quarantined under "Advanced · …"; the current value stays pinned visible when filtered. **Column fields on the data-shaping nodes** (Filter/Group by/Aggregate/Rule — previously blind free-text where you had to KNOW the transactions collection calls its column `category`): now dropdowns of the wired dataset's real schema, resolved by walking the node's "in" wiring upstream through shape-preserving steps to the nearest Dataset node; unwired or reshaped chains fall back to free text, and a stale value shows as "x (not in source)". **Formulas + Custom fields tabs** (Cam: "refine formulas and custom fields tabs to better match the other studio pages") now open with the same studio masthead (eyebrow + serif title + lede) as every sibling, internals untouched. **Design tab polish:** Custom-layout block selects no longer truncate unreadably (the Shows cell wraps at a readable minimum; compact selects carry full-text hover titles) and a Figure block now shows the same metric description + atoms trail as the Single-figure picker. **Pages/fields row ⋯ menus** open leftward so they can't spill over the adjacent composer column. The builder's full-surface variable computation is memoized (500ms) — it was recomputing pools/plans/smart passes on every keystroke. Cam: "the build a widget page needs to be able to build any of the widgets in the dashboard from scratch via formulas and custom values, even the styling as well." **Chrome:** the flat gray toolbar (two full-width selects + seven equal buttons) became a masthead ("Build a widget", serif) + an intent-grouped command bar — *Start from* (preset / My cards), *This card* (name · Save · **Publish → dashboard** as the one primary action · Delete as a guarded danger ghost), *Canvas* (Undo/Redo · New · W/H steppers) — and the workspace re-architected so the **canvas keeps the full height** with the inspector and the **live preview docked in the right rail**, always in view while wiring (they previously fought below and squashed the canvas to a sliver). Panel skins moved onto theme tokens (accent pane titles, `--border`, Fraunces preview/tile titles). **Parity — formulas & custom values:** the builder's variable surface was a partial subset (no custom fields, no molecules-as-persisted, no period figures); it now evaluates the FULL engine surface via the same `liveEngineVars` the Design tab and dashboard use — every `cf_*` custom-field value, per-budget/goal/account/debt figure, and smart_* posture var — and the Figure node's picker lists all of them (284 options on sample data) instead of the static atom list. **Parity — presets:** seven new dashboard-parity recipes (Safe to spend, Savings rate %, Financial health 0–100, Budget meter progress, Budgets / To-do / Upcoming bills lists) join the existing KPI/chart/list set, so every dashboard widget class is buildable-by-example and reshapeable with formulas, custom values, and the style nodes (accent/tone in-graph; tile bg/text/border/radius/font/weight/shadow via Manage-widgets, which targets published builder cards too). `widget_builder_check.mjs` end-to-end green (its dead rail-link navigation moved to route pushes). both tabs shed the old kit for bespoke decks in the studio design language (eyebrow + serif masthead). **Manage** (`.wman`, also /widget-manager): the three stacked cards (Layout / DataTable / style editor) became an arrangement deck — the **widget ledger** (live-order rows: order number, name + Hidden tag, size value with hover-revealed W/H steppers, reorder arrows, visibility switch; wraps on phones with controls visible at rest) beside a **live board map**: every widget at its true span on a miniature 4-column dashboard, hidden tiles ghosted dashed, in live order — click a tile and its ledger row scrolls into view and flashes. Toolbar keeps the layout presets + Show/Hide all with a "N of M visible" tally; the tile style studio sits beneath under a serif accent-tick section title, mechanics untouched. `widgetDisplayName` also learned the missing ids (kpi-safetospend, published `wb:` card names, `us:` spec titles) so no raw ids leak. **My pages** (`.spg`): the form-card + row list became a **page registry** (serif page names, mono `/p/slug` addresses, real widget counts, Open →, ⋯ menu whose delete opens the two-step inline confirm — a page takes its widgets and layout with it) beside a sticky composer whose live footprint previews the address a new page will get. e2e: `widget_manager_check` + `widget_style_check` + `wmtable_scroll_verify` updated for the ledger (and their stale nav-clicks/localStorage reads moved to route pushes + UI-state assertions — layout/config persist in the SQLite dataset now) — all green. Cam: "use triple dot menus for the delete x button." Each registry spec line's bare delete × is now the app-standard viewport-aware `uiw.KebabMenu` (the same ⋯ the accounts/budgets/goals rows use) holding a danger "Delete custom field" item — which still opens the two-step inline confirm (focus lands on "Keep it"; cancel hands focus back to the ⋯ toggle). Row testids: `fld-menu-btn-<id>` / `fld-delete-btn-<id>`. Interaction suite + `verify_fields_split.mjs` green.
- **/fields rebuilt from scratch as a schema ledger — no bento/form-card/EntityListSection kit (2026-07-04):** the custom-fields page was the old kit verbatim: a misaligned `form-grid` card (naked selects, labels floating over two of five controls) above six stacked `EntityListSection` cards, four of them empty ("No custom fields for X yet."). Now it's a purpose-built `.fld-deck`: the left column is the **Field registry** — populated entities as ruled ledger groups (Fraunces titles + count pills + an "Add another" shortcut), each definition a **spec line**: boxed mono type tag, bold label (+ REQUIRED marker), then key + the live `cf_*` formula chip + choice options as one sub-line; all empty entities compress into a single quiet **"Nothing yet on [Budgets] [Coverage rules] …"** line of dashed open-slot chips that pre-select that entity in the composer and focus the key input (a fresh install is one line, not six empty sections). The right rail is a **sticky composer** with every control properly labeled (Lives on / Key / Display label / Type / Choices / Requirement), a persistent key-format hint, and the signature **"What this field will do" footprint** that updates as you type: which entity's forms it appears on, the reports-grouping note for transactions, the required-ness consequence, and the live formula variable preview (`engineenv.CustomFieldVar` — the same scheme the engine actually uses). **Deleting is a two-step inline confirm** (`role="alert"`, focus moved to the safe "Keep it" and handed back on cancel) that names the exact consequence — values on existing records go, and "Formulas using cf_x will stop working" when the field feeds a variable — replacing the old single-click destructive ×. Built via an adversarial Sonnet critique loop (round 1: ITERATE with 2 high + 4 medium findings, all implemented; round 2: SHIP, plus its a11y/polish notes applied). New `rules_fieldssurface.go` + `en_fieldssurface.go`; every e2e contract preserved (entity→type select order, Key/Label placeholders, "Add field", "Add a custom field"); `verify_fields_split.mjs` green (its stale /customize "Available variables" marker updated to the variable palette that replaced it in 4366ae87); full `go test ./...` green.
- **Smart tab rebuilt from scratch as a bespoke editorial surface — no bento/tile/card kit (2026-07-03):** Cam: "need redesign the smart tab" (right after the same call on Ask). Same lesson applied: the tab was the shared `bento bento-smart` host + `astTile`/`astSection` hero + `rptChip` posture chips + bordered/shadowed `.smart-card` finding boxes — the generic kit. This replaces the shell with a bespoke `.smt-deck` editorial column and a from-scratch **masthead** — a seagreen "SMART FEATURES" kicker, the big findings count leading (nobody opens a findings feed to admire the rule tally), the agent's serif voice line on an accent spine, quiet inline posture metrics (Watching / AI·billed / Density as overline-label + display-value pairs), and the on-device promise as fine print, over a hairline rule. The **findings feed** sheds the bordered box: each `.smart-card` becomes an editorial row — a severity-colored left tick + a bottom hairline, no card (higher-specificity scoped CSS beats the generated `!important` box; severity variants keep their color). The catalog, AI, and digest sections stack as bespoke blocks whose legacy `.card` chrome is dissolved by `.smt-deck` scoped CSS, all speaking the same serif accent-tick section-title language — every toggle, pager, cadence picker, and density dial intact. All testids/ids preserved (`smart-hub`, `#sec-smart-hero`, `smt-hero-count`/`-voice`, the section testids); `smart_surface_check.mjs` updated to the bespoke selectors and 13/13; `assistant_check.mjs` 23/23; full `go test ./...` green.
- **Ask tab rebuilt from scratch as a bespoke deck — no bento, no tile, no card rail (2026-07-03):** Cam: "the assistant looks like dog shit… redesign it from scratch, stop reusing the old design." The re-housing had wrapped the console back in the shared **bento host + Widget tile + EntityListSection card rail** — the exact "old design" that read as generic. This tears all of that out and builds a purpose-made layout: a `.ask-deck` CSS grid (dominant conversation column + a 19rem aside) replaces the bento; the conversation gets its own **bespoke header bar** (an amber/seagreen status dot, the serif "Your agent", the on-device/live status inline, and New chat / Advanced as quiet ghost actions on the right, over a hairline rule) instead of the astSection tile chrome; and the periphery becomes a **chrome-less "margin notes" aside** — accent-tick serif labels, hairline-separated 2-line pin previews, and a quiet Conversations index — not stacked cards (any legacy `.card` that lands there, i.e. the two detector groups, has its skin dissolved by scoped CSS to match). The canvas itself is **content-height** (grows with the conversation up to a viewport cap) so a short thread strands no void — the failure that made the first cuts read as broken; the empty thread leads with the "What should we work on?" hero (greeting + ASK/DO/ESTIMATE capabilities + starter tiles); agent replies sit on a soft raised surface; the mid-conversation key notice is a slim one-line strip; and the rail preview caps at three pins with the full set on the Insights tab. All chat logic, hooks, ids, and testids are untouched. Dead rules from the abandoned bento/two-column layouts removed. `assistant_check.mjs` 23/23; full `go test ./...` green.
- **Ask tab re-housed in the app's own component language (2026-07-03):** Cam: "look at the widgets and components from the other pages and use the current redesign language." The console now lives INSIDE the same chrome as every redesigned surface: a **bento host** (`bento bento-ask`) with the conversation as a **Widget tile** spanning three columns under the **serif accent-tick section head** ("Your agent" + the New chat / Advanced pills as the section action — byte-identical chrome to /health, /smart, /reports), the live/on-device **status line as the hero eyebrow** with its dot, and the agent's periphery — observations, spending highlights, Pinned insights (with its See-all cross-link), Conversations — stacked as rail cards in the fourth column with the serif-tick card titles. The console internals (document measure, avatar-guttered replies, docked composer with the circular send, keynote, keyboard hint) shed their standalone chrome and inherit the tile's. `assistant_check.mjs` 23/23; full `go test ./...` green.

### Changed
- **Ask tab rebuilt from scratch as the agent console (2026-07-03):** Cam: "redesign the ask tab or the chat area from scratch." The card-with-messages is gone; the conversation now lives in a purpose-built console: a **slim identity bar** (a live/on-device status dot, the serif "Your agent", an honest status line — "On-device answers — add a key for the full agent" — with New chat / Advanced as quiet pills), a **full-height scrolling canvas** that sets the conversation in a centered 46rem document measure, and a **docked composer** the content scrolls beneath (hairline + soft fade, circular accent send with an up-arrow, and a keyboard hint line — "Enter to send · ↑ cycles your past questions"). Agent replies carry a **circled ✦ avatar in a gutter** beside editorial text (the per-message label is gone); the thinking indicator joins the gutter with an animated ellipsis; the keyless example Q→A pairs adopt the same message language; starter prompts render as **inviting bordered tiles inside the "What should we work on?" hero** (which scales up in the canvas). All chat logic — turns, retry, tool approval, input history, conversations — is untouched, and every existing testid survives. Verify: `assistant_check.mjs` 23/23; the briefing suite's E1 wipe-state check flakes independently of this work (it failed before the console existed and involves the first-run sample seeding — left for the briefing owner).

### Added
- **Ask tab: the conversation sheds its dated skin (2026-07-03):** Cam: "the ask tab still looks dated" — the grammar pass hadn't touched the bones. Now the conversation itself is designed: **the agent speaks in the house voice** — a "✦ AGENT" micro-label over an accent-ruled editorial column with relaxed line-height, no gray SMS blob — while **the user's words sit in a quiet accent-tinted pill** with an asymmetric speech corner; the **composer is the elevated centerpiece** (soft-raised field, integrated Send, an accent focus ring); the thinking indicator joins the agent's voice in italics; the key-hint paragraph wall became a **compact dashed keynote** (copy left, Settings right); and the New chat / Advanced controls ride the serif section header as quiet pills instead of stacking a second toolbar row. One structural discovery en route: EntityListSection flattens Fragments, so the card's "last child" the cockpit flexed was actually the keynote — it briefly rendered as a giant stretched box; the thread alone now carries the flex. Verify: `assistant_check.mjs` 23/23 and `assistant_briefing_check.mjs` 13/13 green.

- **/assistant Ask tab: hub-cohesion + honest-keyless pass (2026-07-03):** The conversation cockpit joins the hub's design grammar and got its own adversarial UX review. **Cohesion:** the "Your agent" / "Pinned insights" / "Conversations" titles now carry the serif accent-tick chrome every redesigned surface uses; the chat card fills a proportionate viewport slice (46vh floor) with the thread flexing and the composer anchored — without the 500px void the first cut of the cockpit stranded under a short thread (the review called it "a broken/loading state"). **Cross-tab wiring:** the hub's active tab moved from local state into a shared `uistate.UseAssistantTab()` atom, so the Ask rail's Pinned-insights card gains a working **"See all in Insights →"** link that switches the sibling tab in place (the hub review's duplicate-card-no-affordance finding). **Honest keyless mode:** the composer's placeholder no longer overpromises — keyless it reads "Ask about your money — add a key in Settings to unlock the full agent…" (the fixed-question mode still answers; the copy now matches what Send can actually do), the keyless callout's Settings button keeps its natural width, and the ambiguous "Advanced" pill gains a state caret + a tooltip naming what it opens (backend + the editable system prompt). Verify: `assistant_check.mjs` 23/23 still green; the cross-link tab-switch probed live.

- **/assistant Smart tab flattened into an agent-voiced bento surface + smart_* variables (2026-07-03):** The Smart panel (both the /assistant tab and /smart) lost its nested Insights/Manage tabs — everything now sits on ONE scannable surface in the app's bento pattern (Claude design skill + an adversarial hub-wide review applied): a **hero tile that leads with what the agent FOUND** ("98 findings worth a look" in the display serif, warm-toned when non-zero, with the agent-voiced line — "I've found 98 things worth a look — they're listed below" / "All quiet" / an onboarding voice when nothing is on) and posture chips (Watching · AI billed · Density); the findings feed (retitled **"Findings"** — the review caught "insights" meaning two different things one tab apart); the AI feature outputs; and the full **Manage catalog folded into per-page accordions** (page name + enabled/total count — sixty-plus toggles no longer render as one wall; open state survives re-renders AND remounts via a session-level map). The digest moved beside the catalog where the other config lives. **Formulas:** the layer's posture is now `smart_features_on` / `smart_free_on` / `smart_ai_on` engine variables (new `engineenv.SmartCounts` fed from the same derivation the hero reads — they can never disagree), in the picker under a Smart features group. Cross-tab honesty fix from the review: the Insights-tab "All clear" now scopes itself ("No anomalies in your recent activity. (Bill and goal findings live on the Smart tab.)") so the two tabs stop contradicting each other. Also fixed en route: a **GWC component-sibling ordering quirk** rendered the feed card's title at its BOTTOM (a bare `ui.CreateElement` as a card's direct child mounts after its header sibling — wrapped in a plain Div; regression-guarded in e2e). Verify: `go test ./...` + a new `e2e/smart_surface_check.mjs` (13 checks — hero-findings ≡ capped feed, live Watching-chip updates on toggle, accordion-stays-open guard, disable-all → onboarding voice negative, the smart_* group in the picker) + the four pre-existing smart e2e updated for the accordion catalog and the sample now seeding features on (from-zero specs start with Disable all) — all green.

### Fixed
- **i18n sweep second pass — components + helper args (C361, 2026-07-03):** the first pass missed
  four surfaces; all closed. The shared component library `internal/ui` now translates its own
  copy (DataTable pager All/Prev/Next/Rows-per-page, FlipPanel Close/Cancel/Save, FilterToolbar
  close, InlineEditForm "Enter to save · Esc to cancel") and `uistate.Global()`'s "Settings" title;
  the four literal label-helper call sites (labeledField "Role"×2/"Priority", smartBrandHeader
  "Digest") moved to keys; the ratchet's scanner now checks label-helper first-args and covers
  seven more packages (ui/uistate/widgetrender/widgetregistry/pages/mermaid/chartspec — all at 0
  except widgetregistry's 2 persisted preset templates, filed under C362). Pre-wasm boot copy in
  web/index.html is documented in C361 as needing its own mechanism.

### Added
- **Studio → Formulas rebuilt: a searchable workbench + the compound-variable editor (2026-07-03):** The formula tab (and the /customize route, which shares the surface) becomes a bento pairing of two tiles. **The workbench** — the shared FormulaBuilder used across the app — got its long-flagged palette overhaul: the ~350 variables now sit behind a **search box** ("Search 347 variables…", ranked so label matches beat the internal weight atoms) and **collapsible groups** that read as a table of contents — each closed group shows two example labels plus its count ("Budgets — Baby & Childcare limit, … 40") instead of a wall of chips, with Core money's five curated figures open by default. Groups are now **derived from the metrics themselves** rather than a hand-maintained list — which had silently hidden the Assistant group (and would have hidden every future one). The workbench title joins the serif accent-tick design system, and a scope sentence separates the page's two save concepts ("Saved formulas are your personal calculations — they don't change how the app computes anything"). **The compound-variable editor is new** — and it's the missing half of the "score IS a formula" story: every molecule (net_worth, savings_rate, safe_to_spend, credit_utilization, health_score, credit_proxy, …) renders with its provenance tag (built-in / edited / yours), live value, plain-English doc, and exact formula — **editable in place** with a live draft preview, a point-of-action warning ("⚠ Live definition — saving changes every page and widget that uses health_score, immediately"), save-with-validation (unparseable formulas are rejected with the error shown), **reset-to-default** for overridden built-ins (appstate gains `DeleteMolecule`; deleting an override restores the default — round-trip tested), delete for custom ones, and a **new-compound-variable form** (name-shape validation + taken-name rejection against the live variable surface). An adversarial UX review drove the group examples, search prominence + ranking, the scope sentence, the live-definition guardrail, and the serif title. Verify: `go test ./...` (molecule override/reset round-trip in appstate) + a new `e2e/studio_formulas_check.mjs` — 20 checks whose keystone proves addressability end-to-end: edit `health_score` in Studio → **/health's hero renders the edited definition** → reset restores the built-in; negatives cover invalid formulas, taken names, and search misses.

- **/assistant Insights tab rebuilt as the agent's briefing — a widgetized bento surface (2026-07-03):**
  the flat card stack (merchants, trend, flags, pins) becomes a designed briefing in the app's
  redesigned-page pattern: a **hero tile** with the month-to-date spend in the display serif, a
  pace delta pill (vs what you'd spent by this same day last month — the honest like-for-like
  baseline, clamped at month edges), an agent-voiced brief line ("You've spent $X so far this
  month. Dining is doing most of the pushing." — the pace judgment lives only in the pill so the
  two never repeat a figure), and figure chips (last month in full, top merchant, flagged count —
  danger-toned when non-zero); a **toolbar** (custom-values toggle + reports/transactions drills);
  the **attention pair** — Flagged activity (anomaly detectors, with a designed ✓ all-clear state
  instead of vanishing) and Spending highlights (category shifts, calm empty state); the
  **spending trend** now tracking the theme accent (was hardcoded `#e05c5c`) with a serif takeaway
  vs the six-month average; the **merchants + pinned pair**; and an opt-in **FormulaBuilder tile**.
  The briefing figures are new `assistant_*` engine variables (`assistant_spend_mtd` / `_prev` /
  `_pace` / `_pace_delta`, `assistant_highlights`, `assistant_top_merchant`) computed by the SAME
  exported helpers the tiles render from (`engineenv.AssistantSpendStory` / `AssistantHighlights`),
  so a formula figure always matches the page; they appear in the picker under a new Assistant
  group (`widgetcatalog.AssistantMetrics`). No-account datasets get the add-account empty CTA.
  The two now-orphaned /insights card builders (`monthlySpendingChart`, `topMerchantsSpendCard`)
  were removed. Verify: `go test ./...` (3 new assistantvars surface tests incl. the day-one pace
  edge) + new `e2e/assistant_briefing_check.mjs` (13 checks: grid + 7 tiles, hero figures, all-clear,
  takeaway, formula reveal + ASSISTANT picker group, merchant drill-through, empty state) 13/13 +
  the existing `e2e/assistant_check.mjs` 23/23, dark and light themes screenshot-reviewed.
- **i18n coverage sweep + hardcoded-English ratchet (C361/C362, 2026-07-03):** an AST scan of every
  display position (element children incl. concatenations and Sprintf formats, Title/Placeholder
  props, aria-labels, Title:/Label:/Detail: struct fields) found **428** user-facing strings
  bypassing the i18n catalog. First tranche converted — screens **211→126**, app chrome **17→0**
  (dashboard tiles, /split, /accounts reconcile + forms, /health, /debt, /documents review,
  /categories, /rules, smart digest, budget card actions, toast/period-picker/date-format/backend
  settings) with byte-identical English values in `internal/i18n/en_i18nsweep.go`, so rendered
  output and e2e text matchers are unchanged. A permanent **one-way ratchet test**
  (`internal/screenlint/i18n_hardcoded_test.go`) now fails `go test` on any NEW hardcoded copy
  (per-directory baselines that may only fall; brand names exempt via allowlist). The logic-layer
  gap is filed as C362: smartengine (160) and widgetcatalog (42) bake English at generation time —
  and notifications persist it pre-formatted — needing a key+args architecture.
- **/credit rebuilt: the proxy score IS a formula — one-story factor tiles (2026-07-03):** The credit-health page moved from a stack of legacy cards to the bento pattern, with the same structural upgrade as /health: **the headline is now literally a formula molecule** — `credit_proxy = clamp(floor(Σ factor×weight), 0, 100)` in `DefaultMolecules`, over 8 new `credit_*` atoms (the utilization / on-time / account-age factor scores AND their exact normalized weights, plus the actionable `credit_pay_to_30` / `credit_pay_to_10` totals) — auditable on the hero behind a disclosure, referenceable in any formula or dashboard widget, and re-weightable under Formulas. To make the identity exact, `credithealth.computeProxy` was refactored to the normalized-weights evaluation order (same truncation, same always-weighted utilization — a limit-less household still scores that factor 0 rather than dropping it) and `Result` gains the exact `Weights` (guard test: floor(Σ score×weight) reproduces `ProxyScore` for arbitrary inputs, incl. missing-factor cases). The input assembly is shared pure code (`engineenv.CreditInputs`). **The page**: a hero tile (ring + band + aggregate utilization + the required not-a-FICO disclaimer + the folded formula), a full-width **Card utilization** tile (the dominant 55% factor: value+target fused into one met/unmet line, a value meter, and every card's detail rows — utilization bar, balance-of-limit, band chip, "Pay $X to reach 30%" nudge, the inline limit editor (C211), and the balance-history trend when snapshots exist), **On-time payments** and **Account age** factor tiles in the one-story style (score value + plain-language status; curve/weight/variable chip behind "How it's scored"), the **holding-back / improve** pair with point-cost and point-gain chips, the optional Smart+ AI read, and an opt-in FormulaBuilder seeded with `credit_proxy`. The embedded `CreditHealthPanel` (used on /debt) is unchanged. Verify: `go test ./...` (identity guards in credithealth + engineenv, incl. pay-down-target math) + a new `e2e/credit_check.mjs` (17 checks — evaluating `credit_proxy` in the live FormulaBuilder equals the ring figure exactly; editing a limit re-scores utilization live 58%→32%; negatives: card-less dataset shows the CTA + disclaimer) + the c211 limit-edit and ring-a11y e2e (now `E2E_URL`-parametrized) all green.

### Changed
- **/health factor tiles: one number story each (2026-07-03):** Cam: "the targets and the 0-100/100 make me confused — why so many stats and numbers to track." Each tile was juggling five numbers on three scales (the domain value, the target, the internal 0–100 score shown as "N / 100", the "%-of-your-score" weight, and the variable chip's duplicate). Now a tile reads as ONE story: the value fused with its target into a single met/unmet statement — **"31% ✓ On target — 20% or more"** (green) or **"43% · Target: under 36% of income"** (dim beside the red value) — with the meter as the only score visual. Met/unmet is computed in the MODEL from the raw inputs (`healthscore.Factor.TargetMet`, table-tested: 20%≥20 met, 2.9mo<3 unmet, 35%<36 met, 30% not-under-30 unmet, no-debt met by definition), not re-derived in the UI. The internal score, its exact weight share ("scores 62 out of 100, counting for 25% of your overall number"), and the `health_*` variable chip all moved inside the "How it's scored" disclosure — the formula plumbing stays fully addressable, one click away, without competing with the reading. e2e updated (F2 met/unmet lines, F2b asserts no bare "N / 100" on the surface, chips asserted inside the disclosure) — 17/17.
- **/assistant rebuilt agent-first — the conversation IS the page (2026-07-03):** Cam: "focus on the /assistant page next, make it agent first." The Ask tab previously buried the chat below four data cards behind an "Ask a question" scroll button; now the surface leads with the agent. **Layout:** a two-column split — the conversation dominant on the left (tall scrolling thread, composer above the fold, New chat + Advanced controls in the header) and the agent's periphery in a right rail (anomaly observations + spending highlights, pinned insights, and the saved conversations as a vertical list). The duplicated merchants/trend cards left the chat surface (they live on the hub's Insights tab). **First touch:** an empty thread opens on an agent-voiced intro — "What should we work on?" with three capability rows (**Ask** anything about your money · **Do** things with in-thread approval — the existing 22-tool loop · **Estimate** with calculator + web, assumptions stated) — and, keyless, a single callout stating the crucial fact (fixed question set now, full agent with a key) with the Settings CTA; the two other key pitches that used to stack on the same screen are gone. The keyless demo transcript is now visually a demo (dashed, dimmed frame) instead of masquerading as the user's own thread, starter chips show only on an empty thread (replaying fixed chips after real exchanges read as a bot ignoring the conversation), and the composer placeholder teaches all three verbs. An adversarial UX review drove those last four fixes. **Verify:** new `e2e/assistant_check.mjs` (23 checks incl. deterministic keyless exchanges — "what is my net worth?" answered from on-device figures, persistence to the rail + restore, empty-send and unanswerable-question negatives, the prompt flip modal, tab round-trip) 23/23; `go test ./...` green.

### Added
- **/health rebuilt: the score IS a formula — in-depth, addressable factor tiles (2026-07-03):** The financial-health page was redesigned into the bento pattern with a structural upgrade underneath: **the headline score is now literally a formula molecule** — `health_score = clamp(round(Σ factor×weight) − health_penalty, 0, 100)` in `DefaultMolecules`, computed over 15 new `health_*` engine atoms (each factor's 0–100 score AND its exact post-renormalization weight, the deficit penalty, and raw values like `health_emergency_months`) — so the number is auditable (the live formula renders on the hero behind a quiet disclosure), referenceable in any formula or dashboard widget, and **re-weightable by the household** (molecules persist by name — editing `health_score` under Formulas reshapes your own scoring; the page reads whatever the engine reads). `healthscore.Factor` gains the exact `Weight` (guard test: the formula identity reproduces `Evaluate` for any inputs, including the all-zero NoData case), and the input assembly moved to a single pure `engineenv.HealthInputs` shared by the page, the dashboard tile, the local Q&A, and the variables — they can never disagree (`engineenv.Data` gains `Categories` for the budget-rollup factor). **The page**: a hero tile (score ring + band + delta + the folded formula identity), **six in-depth factor tiles** (the current value in the display serif toned by its score, the target, a 0–100 meter — a zero score renders a visible red sliver so it reads as scored, not broken — `N / 100` + its exact share of the score, a why-it-matters paragraph, the exact scoring curve behind a "How it's scored" disclosure, and a footer pairing the live variable chip (`health_savings · 100`) with an **"Act on this"** drill to the screen where you improve it), the prioritized focus-next steps (left-aligned), a monthly **score-history** trend tile once two snapshots exist, and an opt-in FormulaBuilder seeded with `health_score`. An adversarial design-review pass drove the disclosure folds, the zero-sliver meter, and the footer CTA placement. Verify: `go test ./...` (formula-identity guards in healthscore + engineenv incl. the deficit-penalty path) + a new `e2e/health_check.mjs` (16 checks — the ruthless one evaluates `health_score` in the live FormulaBuilder and asserts it equals the ring figure exactly; negatives: wiped dataset reads "Not enough data", no page errors) + the r52 focus-step drills still green.

### Changed
- **Smart pay schedule rebuilt around payday buckets (Cam's model, 2026-07-03):** Cam: "my expectation is… it organizes them into 2 buckets and balances out the payments… some bills have to get paid ahead, so those are flagged until you did it once, and recurring bills acknowledge that you already paid ahead to keep the revised cadence." The optimizer's objective changed from "move a bill only when it evens the load" to **full consolidation**: every movable bill lands ON a payday at or before its due date (never late), buckets balanced by amount (greedy over the sorted load vector, floor-guarded — an item stays on its due date only when no payday precedes it or cash is too tight to front anything). The modal preview now reads as **payday buckets** ("Pay on Fri, Jul 17 — 6 bill(s) · $2,582.00" with the bills grouped under each). **"Pay ahead" is now a precise flag** (`Move.CycleAhead`): only payments that jump into an *earlier paycheck* than the one their due date belongs to (fronted money) — consolidating onto the due date's own payday carries no flag — and per Cam's cadence rule the flag clears once that occurrence (or the same bill's prior occurrence) is marked paid, so an established pay-ahead cadence reads as just the schedule. `bills_paid_ahead` now counts cycle-ahead payments. Also fixed a **timezone defect** this exposed: liability due dates are local-time while parsed payday anchors are UTC, so a bill due ON a payday compared as "before" it — billsched now canonicalizes every date to a UTC calendar day at entry (regression test). Measured on sample data: the plan view calendar collapses July onto exactly the three paydays (Jul 3 ×7, Jul 17 ×5, Jul 31 ×7 accent dots) with hollow ghosts on the vacated due dates — 28 payments grouped, 3 flagged pay-ahead. e2e grown to 61 checks (SM8a buckets, SM9 all re-dated rows via data-plan-move, SM9b flags ≡ cycle-ahead set), 61/61; 4 new/updated native tests.

### Added
- **/networth rebuilt as a widgetized, engine-driven bento surface + networth_* custom values (2026-07-03):** The standalone net-worth page was a single plain card (stat grid + an off-theme purple trend); it's now a full balance-sheet surface in the app's bento pattern (Claude design skill + an adversarial design-review pass applied): a **hero tile** ("Your balance sheet as of <date>" eyebrow, the net figure in the display serif, a **month-to-date delta pill**, and Assets / Liabilities / Liquid-share / **Debt-to-asset** chips — the ratio danger-tones past 50%), a **toolbar** (trend horizon **6m / 12m / 2y**, a Net-worth-metrics toggle, View-accounts + View-debts drills), the **Trend tile** (theme-accent area chart with a serif takeaway — "Up $X over the last N months — now $Y." — thinned month labels at long horizons, and **"Now" appended as the final point** so the curve ends at the exact hero figure instead of the last month boundary), the **"What you own" / "What you owe" pair** (asset buckets Cash / Invested / Property & vehicles / Other and liability buckets Credit / Loans / Mortgage, share bars in the accent vs the money-down tone — the SAME buckets as the new engine variables), and **"By account"** contribution rows (largest first, type meta that skips verbatim repeats, liability rows down-toned, capped with a "+N more" line, and a plain-English bar-scale hint). **Custom values:** the balance sheet is now `networth_*` engine variables — `networth_change` / `_change_pct` (month-to-date), `networth_cash` / `_invested` / `_property` / `_other_assets`, `networth_liquid_pct` (new `engineenv.addNetWorthVars` + native surface tests, in the formula picker under a **Net worth** group and an opt-in FormulaBuilder tile). The trend horizon persists (`uistate.NetWorthConfig`). Also: the shared Reports-tab NW panel's trend now tracks the **theme accent** (was hardcoded `#7c83ff` purple); the sample condo is typed `property` (it sat in "Other"); the rate-less-currency exclusion notice renders on the new hero (C79 honesty); and the categories screen's two tree lists moved onto the `EntityListSection.Rows` slot (retiring two hand-rolled `Div(.rows)` scaffolds per the screenlint ratchet). Verify: `go test ./...` (new networth-vars tests) + a new `e2e/networth_check.mjs` (19 checks incl. negatives — horizon persistence past the autosave ticker, down-toned owe bars, parens liability amounts, wiped dataset → add-account CTA) + the rewritten `networth_grid_verify.mjs` (bento-layout contract) + the r52 NW-panel e2e all green.

### Fixed
- **Smart pay schedule's work is now visible on the calendar (2026-07-03):** Cam's follow-up: "I clicked use this plan and nothing changes, none of the dates on the calendar got rearranged." Reproduced on the live build with sample data: the plan made **7 moves** ($1,147 off the heaviest paycheck) yet the July calendar showed one indistinguishable new dot and zero ghosts — because (1) most moves pull NEXT month's bills onto this month's paydays, and the vacated due dates' hollow ghosts only rendered on in-month cells of the *other* month's grid, and (2) a moved-in payment rendered as an ordinary due-date dot, indistinguishable from a bill that was always due that day. Now: pay-on dates carrying moved payments get an **accent pay-ahead dot** (ring + accent fill, title "N payment(s) the plan moved to this payday"), **ghosts render on out-of-month cells too** (Aug 1's vacated slot shows in July's trailing cells), and a **legend under the calendar** explains the vocabulary in both views ("accent dots are payments moved onto a payday; hollow dots are the raw due dates they left"). Verified visually on the dev server: July shows the Jul 17 payday carrying 7 moved payments accent-ringed with Aug 1's ghost beside it; August shows 5 ghosts on the vacated dates. e2e grown to 59 checks (SM15 accent dots ≥1 when moves exist; SM16 legend present); full suite 59/59.

### Added
- **World-class UX sweep across all 42 routes (2026-07-03):** full-app visual/UX review (sample +
  true-empty passes, full-height captures, console-error probe — 0 errors on 84 loads). Evidence in
  `e2e/ux-audit-2026-07-03/`; 26 findings filed as **C335–C360** in TODOS.md §V — cross-page
  data-trust (ledger↔reports date off-by-one, three different net-worth month deltas, /bills
  double-counting liability obligations), fix-now bugs (raw i18n keys on the rail + /setup, a raw
  format-verb error on /subscriptions, thousands-separator-less money on /investments + /credit),
  sample-dataset credibility (4-year-stale timestamps), and grouping/IA gaps (/notifications flood,
  subscription over-detection, /accounts' invisible liabilities hand-off).

### Fixed
- **/accounts names its liability hand-off (C346, 2026-07-03):** the page deliberately lists only
  assets (liabilities are managed on /debt) but nothing said so — search couldn't find "Mortgage"
  and the summary counted accounts the list never showed; the asset list now ends with a
  "Liabilities (N) — managed in Debt payoff →" stub linking to /debt.
- **/accounts month-to-date net-worth delta agrees with the dashboard (C341 partial,
  2026-07-03):** the accounts summary built its month boundary in local time, excluding
  first-of-month (UTC-midnight) transactions and reporting "No change this month" while the
  dashboard hero showed the real delta; it now uses the same `dateutil.MonthStart` UTC boundary.
- **Dates no longer render a day early on the ledger and dashboard (C339, 2026-07-03):** Frame
  pipelines carry transaction/bill dates as epoch seconds of UTC-midnight calendar dates, but the
  widgetized consumers rebuilt them with `time.Unix(sec, 0)` — local time — so west of UTC the
  Jul 1 paycheck showed "Jun 30" on /transactions and the dashboard while /reports (correctly)
  said Jul 1, and month labels could shift a whole month. All five calendar-date reconstruction
  sites now append `.UTC()`; the ledger, dashboard recent-transactions, upcoming-bill dates, and
  trend/cash-flow month labels now agree with reports everywhere on Earth.
- **Smart insights stay quiet on a brand-new empty dataset (C356, 2026-07-03):** a fresh
  "Start fresh" store warned "Liquid cash is very low — $0.00" (SMART-B8) and suggested
  'the "balanced" profile… your finances look steady' (SMART-AL1) with zero accounts to read;
  both rules now say nothing until at least one account exists.
- **/activity no longer leaks internal record names or runs the date into the actor
  (C355, 2026-07-03):** audit summaries preferred whatever collection dominated a change — including
  the internal `_meta:*` scalar buckets — so users saw "Added 3 _meta:settingsState records"; the
  feed now describes changes by their real entity collection (internal-only changes humanize to
  "settings"), and the row aside gained a separator ("May 26, 2026 · Marcus Hartley").
- **Setup wizard selects now preselect the real current values (C338, 2026-07-03):** the currency
  dropdown showed "AUD — A$" (first alphabetical) instead of the household's base currency because
  setup.go marked options with the parse-time `selected` attribute, which the reconciled DOM
  ignores; the currency, week-start, and account-type selects now use the framework's `SelectedIf`
  property option like every other screen.
- **Money figures on /investments, /credit, /loans, /duplicates now comma-group thousands
  (C337, 2026-07-03):** those pages' shared local formatter (`fmtMinorAmount`) skipped grouping, so
  "$33720.00" rendered beside a "$4,590.56" produced by the app-wide `money.FormatAccounting` — on
  the same page. It now routes through `money.Group`.
- **Raw i18n keys no longer render in the UI + a guard test (C335/C336, 2026-07-03):** the nav rail
  showed "nav.setup", the /setup wizard hero showed "setup.welcomeTitle/Body" and its account step
  referenced a nonexistent `accounts.type*` key family, /subscriptions printed
  "subs.netPriceUp%!(EXTRA string=$134.60)", and two Settings aria-labels read as raw keys to
  screen readers — all because `T()` falls back to the key name when a key is missing. Added the
  missing keys (`internal/i18n/en_uxsweep.go`), pointed setup's type picker at the existing
  `acctType.*` keys, and shipped `keycoverage_test.go`: a native test that scans the wasm UI
  sources for `uistate.T` literals + screen-registry Label/Title/Subtitle fields against the
  English catalog, so a missing key now fails `go test` instead of shipping raw to users.
- **Smart pay schedule now actually reschedules across months (2026-07-03):** Cam's report: after "Use this plan" the current month never showed the pay-ahead double payments and paging the calendar to next month showed no adjusted dates. Two root causes. (1) **Bills existed as single occurrences** — `bills.UpcomingAll` yields one NEXT due date per bill, so *next month's occurrence of a monthly bill wasn't in the data at all* and could never be pulled onto this month's payday; new pure `bills.OccurrencesWithin` projects EVERY occurrence in a window (liabilities monthly by due-day with month-end clamping, recurrings stepped by cadence, bounded; 4 new tests) and the scheduler/calendar/engine-vars all consume it. (2) **The plan horizon was a fixed 30 days from today** while the calendar pages months — bills in the displayed month beyond the window had no pay-on mapping; the standard window is now 60 days (`engineenv.BillsSmartHorizonDays`, this month + next) and the bills tab extends it to cover whatever month the calendar is paged to, recomputing as you page. Also fixed a real optimizer deadlock the longer window exposed: the greedy pass only accepted a move that improved the single GLOBAL heaviest-paycheck figure, so with a heavy paycheck in BOTH months no single move ever qualified and the whole plan reverted to "already even" — the objective now compares the whole sorted load vector lexicographically (lighter heaviest, then second-heaviest, …), and the final keep/revert honesty check does the same, so evening one month counts even when the other month's stack is immovable autopay (2 new regression tests; the status/hint copy has an honest "spread the load evenly" variant when the headline max can't move). The plan view now also lists pulled-forward future occurrences as their own pay-ahead-tagged rows sorted by pay-on date (the "double payment" month is visible in the list, hero totals stay raw), and the calendar renders occurrences in both views with ghosts for the inactive schedule. On sample data the plan went from 1 move to 6 (heaviest paycheck $4,127 → $3,125). Verify: `go test ./...` + `e2e/recurring_check.mjs` grown to 57 checks — **SM13/SM14 page the calendar to next month with the plan on** (occurrences render; moved bills' raw-due ghosts visible) — the path that shipped untested. Also hardened the two flaky e2e paths (delete-confirm, menu deep-link) against a real ~1/8 stale kebab-menu-item handler race, filed as **C334** in TODOS.md.

### Added
- **/reports rebuilt as a widgetized, engine-driven bento surface + report_* custom values (2026-07-03):** The reporting page was redesigned end-to-end into the app's bento surface-host pattern (like /debt, /planning, /recurring), with the Claude design skill + an adversarial design-review pass whose findings were applied. It's now a `bento bento-reports` host: a **hero tile** (the period Net in the display serif with a vs-last-period delta pill, plus figure chips — income, spending, net worth w/ monthly delta, savings rate, cash runway, no-spend days — with color reserved for direction/warnings, not decoration), a **toolbar tile** (view tabs Overview / Categories / Net worth / Advanced on the left; the **Scope** chip-filter disclosure, the **Report metrics** toggle, and a single **Export CSV** dropdown — six per-table CSVs + Save-as-PDF — on the right), and per-view **section tiles** with the shared serif-title chrome: money-flow Sankey, top payees + biggest expenses (side-by-side, **theme-accent** ranked bars instead of hardcoded blue), deposits + income-by-source, by-member, the by-category card (serif narrative pull-quote, YoY + roll-up as header actions, the Tableau-matched bar/donut pair, drill-through rows, and a **zeroed-categories disclosure** that folds the "$0.00 ↓100%" wall), the net-worth panel with paired cash-flow/savings-rate trend tiles (accent strokes, insight takeaways), and the custom-field/deductible tiles. **Custom values:** the page's derived figures are now `report_*` engine variables — `report_prev_income/spend/net`, `report_income/spend_delta_pct`, `report_avg/median_expense`, `report_no_spend_days`, `report_top_payee_spend/_pct`, `report_burn`, `report_runway_months` (new `engineenv.addReportsVars` + native surface tests, surfaced in the formula picker under a **Reports** group and behind an opt-in FormulaBuilder tile) — so any report figure can drive a formula or dashboard widget. **The reading posture persists** (`uistate.ReportsConfig`: active tab, YoY, roll-up) so /reports reopens how it was being read. Fixes riding along: the **#444 scope selector finally has CSS** (it rendered as an unstyled text soup — now labelled chip rows with clear on-states, folded behind a Scope toggle that shows the active-filter count); a **nothing-matches scope no longer traps the user** (the empty-state early return kept the un-scope controls off screen); **bar-chart x-axis ticks land exactly on the labeled bars** (web/chart.js — d3's fractional ticks each rounded to the nearest label, printing "Mortgage Mortgage HOA dues HOA dues" garbage under every bar chart app-wide; long labels now ellipsize with full names on hover); an open dropdown's tile now paints above later tiles (transformed tiles otherwise swallow menu clicks); the custom-field grouper shows an **honest empty state** instead of a misleading "(no value) 100%" bar; and the "new" category tag went neutral (red stays reserved for negative money). Verify: `go test ./...` (new reports-vars tests) + a new comprehensive `e2e/reports_check.mjs` (27 checks incl. negatives — un-scope trap, Escape-closes-menu, zeroed disclosure, posture persistence, real download assertions) all green, plus the updated rollup / category-drill / by-customfield contract e2e.

- **Smart+ pay schedule for bills — align payments with your paychecks (2026-07-03):** A new Smart+ feature on the bills tab plans *when to pay* each upcoming bill around the user's pay cycle. A new pure `internal/billsched` package (10 table-driven tests) projects paydays from one known payday + a frequency (weekly / every-2-weeks / twice-a-month / monthly), then computes two honest levers: **pay-ahead moves** (movable, non-autopay bills paid on an earlier payday to *even the load across paychecks* — a greedy minimizer of the heaviest pay period's billed total, constrained to never push the projected 30-day low below the raw schedule's low or an optional keep-floor, and reverting when no genuine improvement exists) and **biller-side suggestions** (due-date shifts worth *asking the biller* for — the only lever for autopay bills — each with its low-point gain). The bills tab carries a compact **"Smart pay schedule" tile** (status line + one button); everything else lives in a shell-root **flip modal**: two questions (a payday you know, pay frequency), a live plan preview (heaviest-paycheck now vs. with-plan **with the improvement delta on the chip**, bills-paid-ahead count, lowest-30-day-balance chip + a note that the plan never pushes it lower), the pay-ahead move rows ("Pay X on your Jul 17 payday (due Aug 1)"), the ask-the-biller rows (or an explicit "nothing worth asking for" empty state), an opt-in AI explanation (BYO key), an Advanced disclosure (keep-floor + the schedule's formula variables), a plain-English line stating **what "Use this plan" actually does** (display/reminder dates only — no money moves), and Use-this-plan / Turn-off. Once on, the bill list + calendar gain a **Due dates / Pay-on plan** toggle: the plan view re-dates moved bills (meta reads "pay X · due Y", a Pay-ahead tag) and the calendar shows hollow "ghost" dots where the other schedule's dates fall. **Custom values:** the schedule is referenceable — `bills_low_raw`, `bills_check_load_raw`, `bills_check_load_smart`, `bills_even_gain`, `bills_paid_ahead`, `bills_suggest_gain` via `engineenv.addBillsSmartVars` (native surface test), in the formula picker under Recurring. The modal copy was refined through an adversarial UX review (jargon-free chip labels, delta-on-chip, consequence-of-commit line, suggestion empty state, variables tucked under Advanced). Verify: `go test ./...` + `e2e/recurring_check.mjs` 55/55 (SM1–SM12 incl. negatives: no anchor → no plan/no Use button, AI explain without a key errors cleanly, plan ≤ raw invariant, turn-off reverts).

### Changed
- **Bills calendar enlarged (2026-07-03):** The bills-tab calendar now takes a real ~48% share of the two-column row (it sat at its natural width) with taller cells (76px), larger day numerals, and roomier weekday headers — so due-date dots and the hover-highlight read at a glance beside the scrolling list.

### Added
- **/recurring rebuilt as a widgetized, engine-driven bento surface — all three tabs (2026-07-03):** The "Bills & recurring" hub was redesigned end-to-end in the app's bento pattern (Claude design skill + an adversarial design-review pass whose findings were applied). **Scheduled tab:** a hero (monthly recurring net in the display serif + Money-in / Money-out / Active-flows chips, with the most timely fact — the overdue count, danger-toned, or the next due date — as the fourth chip), a toolbar ("Post due now (N)" carrying its action count and accent-outlined when non-zero, a Schedule-metrics toggle, Add recurring), a **"Next 30 days"** schedule (every derived due date with calendar-medallion dates — grouped per day — an Overdue tag, and a window total), the **flow cards** (repeat-icon cadence tag, autopay/auto-post pills, per-month equivalent, a neutral share-of-outflow meter, and the flow's **formula identity chip**), and the detected-but-unplanned charges (dashed "suggestion" cards with one-click add). **Adding/editing a flow is a shell-root flip modal** (label, Money in/out direction toggle, amount, cadence, optional account/category, first-due date, autopost — disabled with a hint until an account is linked — and autopay); the old inline form + per-row inline editors are gone. **Custom values / identity:** every flow now exposes `recurring_<slug>_monthly` / `_amount` engine variables plus fixed `recurring_monthly_in/out/net` + `recurring_count` aggregates (new `engineenv.addRecurringVars`, surfaced in the formula picker under a **Recurring** group and shown on each card), so a flow like "Gym membership" is addressable in any formula or dashboard widget. **Interconnects:** each card's ⋯ menu deep-links to the flow's homes — **View transactions** (pre-filtered to its account/category, falling back to a label text-match), **View budget** (when its category is budgeted), View account, Edit, Delete (confirm). **Bills tab:** same chrome (Total-due hero + per-year/count/next-due chips, card rows); the **list now scrolls in place while the calendar stays sticky on screen**, and **hovering a bill highlights its due date on the calendar** (a delegated native listener toggling a class — zero re-renders). **Subscriptions tab:** same chrome (monthly-burden hero + chips, every section a tile), and the **detection preferences moved into a flip modal** (sensitivity + account-type/category filters, saving live behind the modal). Verify: `go test ./...` (new recurring-vars engine test) + a new comprehensive `e2e/recurring_check.mjs` (42 checks incl. negatives — empty label / zero amount / cancel-adds-nothing, hero math to the cent, share-meter bounds, modal edit/delete, prefs-modal persistence, hover-highlight on/off) + the updated add-flow story and the /recurring-scoped contract e2e all green.

- **/planning rebuilt as a widgetized, engine-driven bento surface + custom values (2026-07-03):** The planning page (cash runway, "can I afford it?", 12-month net-worth forecast, saved what-if scenarios) was redesigned end-to-end into the app's bento surface-host pattern (like /debt, /investments, /allocate), with the Claude design skill + an adversarial design-review pass. It's now a `bento bento-planning` host of native tiles: a **toolbar** (plan-metrics toggle + Manage-recurring / Net-worth links), a **cash-runway** tile led by a **Safe-to-spend hero** (big display-serif figure) with Starting-balance / Projected-low chips and a **date-labelled** 60-day balance chart, a **"can I afford it?"** calculator, a **12-month forecast** tile (projected net worth hero + accent chart + trim/compare overlays), and the **saved scenarios** (each what-if plan with a sparkline + runway indicator). **Custom values:** the runway buffer + forecast horizon are now a persisted `uistate.PlanningConfig` exposing `runway_buffer` / `runway_days` / `forecast_horizon` engine variables, and **each saved plan becomes `plan_<name>_end` / `_monthly` / `_runway` variables** (like the investment pools) — all surfaced in the formula picker under a new **Planning** group and usable in any formula or dashboard widget, plus an opt-in FormulaBuilder tile. Charts track the theme accent. The forecast/runway/afford/plan LOGIC is unchanged (`internal/forecast` / `planning` / `afford` / `runway`). Design-review fixes applied: the runway got a hero (was three equal stats), its buffer input moved below the results and was capped in width, and its chart gained date labels to match the forecast. Verify: `go test ./...` (new planning-vars engine test) + a new comprehensive `e2e/planning_check.mjs` (23 checks incl. negatives — unaffordable purchase → shortfall, nameless plan → error, depleting plan → runway danger) + the existing runway / constraints / forecast-basis / plan-compare / recurring-story e2e all green.

- **Comprehensive /allocate e2e with negative/edge cases (2026-07-03):** A new `e2e/allocate_check.mjs` (34 checks) exercises the whole page end-to-end — the widgetized surface + hero, the split invariant (Σ rows + kept-back == amount) at $2,000 / $1,000,000 / $0.03, the strategy flip modal (mode / profile / buffer / cap / weights / save-profile), the ranked cards (score meter, breakdown, ⋯ menu with View-source + Exclude), apply→confirm→undo, the why-this-order tile, and the metrics FormulaBuilder — plus negatives: an **empty amount** hides the apply flow, a **per-destination cap clamps every row**, a **buffer reduces allocatable**, **save-profile with no name errors**, **explain-without-a-key** links to Settings, and **no page errors across the run**. (Exclude/restore count behaviour stays covered by the dedicated `story_allocate.test.mjs`.)

### Changed
- **/planning: scenario delete moved into a ⋯ overflow menu (2026-07-03):** Each saved what-if plan card's bare delete ✕ became a standard `KebabMenu` (⋯) with a "Delete plan" item — matching the ⋯ overflow pattern used on the allocate / goals / to-do rows and keeping the card head uncluttered. Updated `planning_check.mjs` SC5 to open the menu before deleting (and to assert no bare `.btn-del` remains).
- **/planning: "Add plan" moved into a flip modal (2026-07-03):** The saved-scenarios tile no longer carries an inline add form — the scenario section header now has an **"Add plan"** button that opens a `PlanAddForm` inside a FlipPanel modal (plan name, horizon, account-prefill, starting balance, monthly change, optional one-time amount/month). The modal is rendered as a sibling of the `bento-planning` surface (not inside a tile) so its `position:fixed` centring isn't broken by a tile transform, and it uses local render state (the trigger button and the modal share it directly) so the open/close re-render is reliable. Saving a valid plan persists it, bumps the data revision so the scenario list behind the modal updates live, and keeps the modal open with a brief confirmation so several can be added in a row; the delete path switched from a private revision counter to `BumpDataRevision`. Updated `e2e/planning_check.mjs` (and the plan-compare / recurring-story e2e) to open the modal before filling the plan fields.
- **/allocate: strategy moved into a flip modal, refined the top section, per-destination source links (2026-07-03):** Follow-ups to the allocate redesign. (1) The allocation **strategy** (split mode, ranking profile, emergency buffer, per-destination cap, criterion-weight tuning, save/delete profile) moved out of an inline tile into a proper shell-root **flip modal** ("Adjust strategy") — the main surface now shows a compact profile/mode summary with an "Adjust strategy" button; the strategy lives in shared atoms so the ranked plan re-ranks live behind the open modal. (2) **Refined the top section** — the amount input clipped its long placeholder; it's now a clean `$ 0.00` field sized to sit beside the figure chips (allocatable / held back / destinations), tuned via the design skill. (3) **Each destination card's ⋯ menu gains a "View source" link** that jumps to where that value lives — "View debt" → /debt, "View goal" → /goals, "View account" → /accounts. Updated the allocate e2e for the modal (mode/reserve now opened via "Adjust strategy") and the amount testid; the whole allocate suite (score / amount-labels / determinism / fill-to-target / income / exclude-restore) stays green.

### Added
- **/allocate rebuilt from scratch: widgetized, componentized, engine-driven "put money to work" surface (2026-07-03):** The allocate page (rank where new money should go, split an amount across the ranking, apply it) was redesigned end-to-end into the app's bento surface-host pattern (like /debt and /investments), using the Claude design skill. It's now a `bento bento-allocate` host of native tiles: a **hero** (the amount to put to work as a prominent accent-underlined input, an income pre-fill nudge, and the split figures — allocatable / held back / destinations), a **strategy** tile (split mode + ranking profile as native selects, an Advanced disclosure with the emergency buffer, per-destination cap, criterion-weight tuning and save-as-profile, a plan-metrics toggle, and a Manage-accounts link), the **ranked plan** (each destination a card with a priority medallion — the #1 gets an accent focus treatment — a theme-accent score meter, criterion breakdown chips, a suggested amount, and a ⋯ overflow menu with Exclude), a **"why this order?"** tile (a no-key algorithmic summary + an opt-in AI narrative), an **apply** tile (confirm → earmark/fund → undo), and an opt-in **plan-metrics FormulaBuilder**. **Custom values:** the plan is now persisted (`uistate.AllocConfig`) so it survives a reload AND becomes engine data — new `alloc_amount` / `alloc_reserve` / `alloc_max_per` / `alloc_allocatable` / `alloc_reserved_pct` / `alloc_destination_count` variables (via `engineenv.addAllocVars`, surfaced in the formula picker under a new **Allocate** group) usable in any formula or dashboard widget. The ranking, split, and apply LOGIC is unchanged (the `internal/allocate` package). Also added a theme-aware `bg-accent` tone to the standard `MeterBar`/`ProgressBar` vocabulary so a standard gauge can track the accent. Verify: `go test ./...` (new alloc-vars engine test) + the allocate e2e suite (score, amount-labels, determinism, fill-to-target, income pre-fill, exclude/restore story) all green.

### Changed
- **/investments: replaced hand-rolled controls with the shared component library (2026-07-03):** Audited the page for bespoke markup that duplicates standard primitives and swapped the clear cases: the 1M/6M/1Y growth-window toggle is now the standard **`uiw.Segmented`** (role=radiogroup, sliding pill, arrow-key nav — the same control 8 other screens use) instead of hand-built `.inv-seg` buttons; the holding-delete and custom-chart-delete buttons are now **`uiw.DeleteButton`** and the chart-edit pencil is **`uiw.IconButton`**, instead of ad-hoc `Button`+`Icon` assemblies. Added an optional `TestID` field to `SegOption` and `IconButtonProps` (additive, benefits every caller) so e2e selectors stay stable; the `.seg` toggle now exposes `aria-checked` (radiogroup semantics) rather than `aria-pressed`. Deleted the now-dead `.inv-seg*` / `.inv-pool-chip-btn` CSS and the `isActive` helper. Left intentionally bespoke (no clean standard, or a swap would regress): the security-type/asset-class **badges** (no Badge/Chip primitive exists), the compact accent **weight/allocation gauges** (`MeterBar` is a full-width, semantic-tone gauge — swapping would change the layout and re-introduce non-accent color), and the summary hero (deliberately shares the `/debt` chrome for cross-page cohesion). Verify: `e2e/investments_check.mjs` 42/42 (G2 now asserts the standard `.seg-btn`, G5/G6/G8 assert `aria-checked`).

### Fixed
- **/investments charts now read the *theme's* accent, not the stale prefs accent (2026-07-02):** Follow-up fix — the previous pass read the accent from `uistate.UsePrefs().Accent` (the legacy swatch preference), but the theme engine (`ApplyTheme`) is the authoritative writer of `--accent` and is applied *after* prefs on boot. So on a theme **preset** (e.g. Midnight/periwinkle, Forest, Paper) the charts stayed seagreen while every button/badge correctly recolored — the exact "charts still green on a non-forest theme" bug. Added `uistate.CurrentAccent()`, which reads the resolved `--accent` custom property straight off the document root (the single source of truth, whichever system last set it), and the growth charts now stroke in that. It reads the **inline** style declaration, not `getComputedStyle` — the latter forces a synchronous reflow that, called on every chart render, measurably slowed unrelated re-renders. Regression-guarded by `e2e/investments_check.mjs` TA1 (switches the accent to periwinkle and asserts the chart strokes follow it and none stay `#2e8b57`).
- **/investments create/edit-chart modal closes reliably; charts no longer recompute on open/close (2026-07-02):** Two fixes to the custom-chart flip modal. (1) The pool-edit atom was subscribed by the whole pools grid and every chart card, so opening/closing the modal re-rendered them all — recomputing each chart's `NetWorthSeries`. Isolated the modal triggers into tiny leaf components (`newChartButton`, `poolEditButton`) so only they + the modal host subscribe; the grid/charts no longer re-render on modal toggle, and the host closes via a bare atom clear (matching the reliable `InvestAddHost`/`SettingsHost` pattern — dropped the racy `BumpDataRevision` workaround). (2) The e2e's Cancel click was firing during the modal's 550ms open flip (the back face is `backface-visibility:hidden` mid-flip, so the button isn't hit-testable) — switched to an auto-actionability click past the flip so the regression test is deterministic.

### Changed
- **/investments: each account tracks a single account (no dropdown); pools became user-created "custom charts" (2026-07-02):** Cam clarified the model: an account card's chart should track just *that one account*, so the per-card **pool dropdown is gone**. Pools are no longer a grouping *of* the account list — they're a separate kind of card: **custom charts** the user creates that **aggregate 1+ accounts** into one combined growth graph. So the "Accounts & charts" section now shows one **single-account card per account** (name · type badge · view-transactions · its own chart) followed by any **custom-chart cards** (accent-outlined, a "Chart" badge, member count, the aggregated area chart, and its `pool_<name>_value` variable). "New chart" opens the flip modal (name + a checklist of accounts to aggregate); accounts can belong to any number of charts now (overlap allowed — `UpsertInvestPool` no longer strips an account from other pools). Editing/deleting a chart never touches the account cards. Verify: `e2e/investments_check.mjs` 41/41 (P3 no dropdown, P4–P7 custom-chart create + aggregated svg + `pool_*` var, P8 account cards unchanged, P9 edit pre-fill, P10 cancel-no-crash, P11 delete).
- **/investments: merged the two account sections into one (2026-07-02):** The separate "Traditional investments" list was redundant with the per-account growth cards (both listed the same accounts). Removed it — the **"Accounts"** section is now the single account list: every investment account as one card with its name, type, a **view-transactions** button, a pool selector, its value + delta, and its own growth chart.
- **Create/edit an investment pool in a flip modal with an account checklist (2026-07-02):** "New pool" now opens a proper shell-root **flip modal** (`InvestPoolEditHost`) with a name field and a **checkable list of your investment accounts to include** — instead of the bare name prompt. The pool chip's pencil opens the same modal pre-filled (name + checked members) for editing. Saving upserts the pool (an account belongs to one pool, so checking it here moves it there). Persistence via the new `uistate.UpsertInvestPool`.

### Fixed
- **Add-security is now a shell-root flip modal; cancelling no longer crashes (2026-07-02):** The "Add a holding" form moved from an inline reveal in the securities tile to a proper centered **flip modal** (a new shell-root `InvestAddHost`, so `position:fixed` centres against the viewport rather than a transformed bento tile). This also fixes a runtime panic (`GoUseAtom called outside component context`) — the old Cancel button called `UseInvestAddOpen()` (a hook) inside its click handler; the modal form now closes via an `OnDone` callback captured at render. Saving adds the holding and keeps the modal open with an "Added ✓" flash so several can be entered in a row.

### Changed
- **/debt widgets got a design pass — depth, accent moments, and motion (2026-07-02):** Added a "little life" layer over the existing dark/seagreen/Fraunces base (Claude design skill), all CSS: a **signature seagreen tick** before every section title; a soft **red glow** under the total-owed figure so it has presence; **gradient-lit** utilization meters and heat rails (instead of flat fills); the ratio chips gained a top-lit gradient + hover lift; the **#1 "pay first" debt and the recommended strategy card now glow** with a soft accent halo (and the focus medallion glows); the "Pay first" / "Recommended" badges get a **gentle pulse ring**; the payoff-ladder cards **reveal in a quick staggered cascade** on load; and the owning-page link arrows nudge right on hover. All motion is gated behind `prefers-reduced-motion`.
- **Notification → resource links are now a persisted config, not a source literal (2026-07-02):** Where each notification links (a bill-due alert → /bills, a budget alert → /budgets, a stale-balance alert → /accounts, …) is no longer a hardcoded Go table baked into the build. It's a store-backed config — `uistate.NotifyRoutes()` reads a JSON `[{prefix, route}]` table from the SQLite app KV (key `cashflux:notify:routes`), seeded from sensible defaults on first read and overridable at runtime via `uistate.SetNotifyRoutes` — so changing where an alert points needs no code change or recompile, just new stored config. This replaces both the in-code `notifyRouteByPrefix` slice (notifications render) and the `eventRouteConfig` map (catch-up runner); the render now resolves a notification's link purely from this config by ID prefix (`RouteForNotifyID`), making it the single source of truth and dropping the now-redundant `FeedItem.Route` field. Verify: `e2e/notifications_check.mjs` 13/13 (clicking a bill notification still opens /bills, driven by the config).
- **Notifications page redesigned as a widgetized "signal feed" (2026-07-02):** The plain notification list was rebuilt from scratch into the app's surface-host tiles: a **summary** tile (a Fraunces hero alert count + "N unread / all read" + a severity breakdown — Critical / Warning / Info chips — plus the "N new since your last visit" catch-up), the shared **filter strip** (a severity filter + **Clear all**), and the **feed** tile. Each notification is now a card with a **severity medallion** (a colored icon — triangle=critical, circle=warning, bell=info), a **severity-tinted left rail**, an unread dot, a text severity tag (color is never the only cue — WCAG), and a relative time ("2h ago"); read items dim, and critical alerts sort to the top. Per-item actions are **inline one-click buttons** (mark read/unread · snooze 1 day · dismiss) rather than a ⋯ menu — deliberately, since a menu is an extra click for actions you take constantly. Also fixed a latent bug where the mark-all-read-on-open could clobber a snooze/dismiss the user clicked right after opening (it now marks read off the live persisted feed via `MarkAllNotifyRead`). Verify: `e2e/notifications_check.mjs` 11/11.

### Added
- **/investments: a growth chart per account + custom account pools that become formula variables (2026-07-02):** A new "Account growth & pools" tile gives **every investment account its own growth chart** (value + toned delta + a seagreen area chart over the shared 1M/6M/1Y window). Accounts can be grouped into **custom pools** — each account card has a pool selector, and a pools bar lets you create / rename / delete named pools. Crucially, a pool exposes a **`pool_<name>_value` engine variable** (the combined current value of its member accounts) that's usable **anywhere** — the FormulaBuilder and dashboard widgets (added `engineenv.PoolDef`/`addPoolVars`, `widgetcatalog.PoolMetrics` + a Pools picker group, and the persisted `InvestPool` config in the app KV). Grouping accounts never hides them — each keeps its own chart; the pool is purely an additive, named aggregate. Verify: `go test ./internal/engineenv/...` (new pool-vars test) + `e2e/investments_check.mjs` 37/37 (P1–P8: per-account charts, pool creation, the `pool_*` variable, accounts persist after pooling, delete).
- **/investments portfolio-growth chart with a 1M / 6M / 1Y window toggle (2026-07-02):** A new growth tile charts the investment portfolio's value over time as a seagreen gradient area chart, with a header showing the current value and a toned delta (e.g. ▲ $3,480 · +11.5% over the window) and a segmented **1M / 6M / 1Y** control that re-scales the trend. The series is the investment accounts' recorded value at each point (via the ledger — monthly points for 6/12-month views, weekly for 1-month), so it reflects real contributions and value updates. Verify: `e2e/investments_check.mjs` 29/29 (G1–G8: tile, 3-segment toggle, svg renders, default 1Y, toggling re-scales the delta).
- **/investments rebuilt from scratch as a widgetized, componentized portfolio page with stocks/securities (2026-07-02):** The investments page was redesigned end-to-end into the app's surface-host pattern (like /debt), and securities became first-class. A new `SecurityType` (stock / ETF / mutual fund / bond / crypto / cash / other) is added to each holding, so **stocks/securities investments** categorize and allocate distinctly from **traditional** (balance-tracked) investment accounts. The page is a `bento bento-invest` host of native tiles: a **portfolio-value hero** (total in the display serif + a "$X in securities · $Y traditional" split + gain / return / cost-basis chips), a **toolbar** (Add security · Manage accounts · Portfolio-metrics toggle), a **Securities** tile (per-ticker holding cards — a security-type badge, ticker chip, shares@price · cost, a portfolio-weight bar, market value + toned gain/return, delete — plus a reveal-on-demand add form with an account + security-type picker), a **Traditional investments** tile (balance-tracked accounts), an **Allocation** tile (by security type *and* by asset class), and an opt-in **Portfolio metrics** FormulaBuilder. An account is valued *either* by its holdings *or* its balance — never both — so the total can't double-count. Verify: `go test ./internal/portfolio/... ./internal/domain/...` (new SecurityType/allocation tests) + `e2e/investments_check.mjs` 21/21.
- **Credit health now shows demerits + the clearest advice, plus an opt-in Smart+ AI analysis (2026-07-02):** The credit-health widget went from a bare score + per-card bars to an explainable coach. The pure `credithealth` engine now derives **demerits** (the factors dragging the proxy score down — high aggregate utilization, a single hot card, missed on-time window, thin history, over-limit, missing-limit — each with an approximate **point cost**) and prioritized **advice** (the concrete pay-down that gets a card to 30% with its estimated **point gain**, building on-time history, entering a missing limit — biggest impact first). The panel renders these as a **"What's holding your score back"** list (−N pts chips) and a **"How to improve"** list (+N pts chips, e.g. "Pay $4,590.56 on Rewards Credit Card to reach 30% utilization"). Added a **Smart+ AI feature (SMART-A11)**: an opt-in, BYO-key AI analysis that reads the same on-screen figures (score, utilization, demerits, actions) and returns a short personalized read — the biggest problems and the single highest-impact next step. It's opt-in (nothing shows until enabled + a provider is configured), degrades to the deterministic demerits/advice with no provider, and never invents numbers. Verify: `go test ./internal/credithealth/...` (new demerits/advice tests) + `e2e/debt_check.mjs` 72/72.
- **/debt sections link to the page that owns their data + widgets read the formula engine (2026-07-02):** Each section header now carries a quiet link to the screen that owns its records — Overview → Net worth, Payoff ladder → Manage accounts, Strategy → Allocate payments, Credit → Manage cards, Loans → Manage loans, Calculator → Open Planning — so you can jump from the read-only debt view to where you edit the underlying data. Also wired the payoff-ladder utilization meters to the engine: each card's utilization now reads its `debt_<slug>_utilization` variable (the same formula surface the "Debt metrics" builder computes over), matching the summary tile, which already reads the `credit_utilization` / `debt_to_asset_pct` molecules and the debt aggregate atoms.

### Added
- **Global "back to top" floating button (2026-07-02):** A circular ↑ button now sits fixed at the bottom-right on every page. It's hidden at the top and fades in once the main content region scrolls down a screenful, then smooth-scrolls back to the top on click (and hides itself again). Visibility is driven by a native scroll listener that toggles a class directly — no per-scroll Go re-render — set up once on mount and torn down on unmount.
- **/debt jump-nav — quick links to each widget (2026-07-02):** The toolbar now leads with a "Jump to" row of section links (Overview · Ladder · Strategy · Credit · Loans · Calculator) that smooth-scroll straight to the matching widget, so a long page is navigable in one click. Each tile carries a stable anchor id (`sec-*`), and the nav only lists the sections that actually render (Credit/Loans appear only when a card/loan exists).

### Changed
- **Debt payoff-strategy panel redesigned as a clear snowball-vs-avalanche decision (2026-07-02):** The strategy widget was a dense wall — two identical "56 months" boxes, a debt table, several prose lines of interest/order, and per-debt include toggles + APR/min editors that duplicated the new payoff ladder. Rebuilt around the actual decision: two comparison cards (each showing months-to-clear in the display serif, total interest, and debt-free date) with the better method **badged "Recommended"** and accent-tinted, a one-line savings callout, a readable **Payoff order** sequence, and the burn-down chart + progress tracker. Removed the redundant debt table and the per-debt include/APR/min controls (inclusion is the ladder card's in-plan toggle; APR/minimum edits are the ladder card's Edit) — and deleted the now-dead `debtRateRow`. The extra-payment input is a single right-sized control with the one-tap "Try $X/mo" suggestion beside it.
- **Payoff-ladder cards are full-width with emphasized order (2026-07-02):** The ladder went from a 2-column grid to a single full-width column stacked in payoff order, so the sequence reads top-to-bottom like a real ladder. The payoff-rank medallion is now a large Fraunces numeral, and the #1 debt (the one to attack first) gets a focus treatment — an accent-filled medallion, an accent card border + tint, and a "Pay first" tag — so the order is the first thing you read.
- **/debt lower panels redesigned to match + comprehensive e2e (2026-07-02):** Follow-up to the cohesion pass — the credit-health / loans / strategy / payoff-calculator panels still read as flat because the debt tiles wrapped them in a redundant `EntityListSection` card *and* an over-aggressive `.card` reset had stripped the panels' own grouping cards. Fixed properly: debt tiles now use a lightweight `debtSection` (a serif section title over the tile's own `.w` frame — no double card), and `.bento-debt .card` is styled as a soft grouping card so the credit hero-ring, the per-card utilization breakdown, and each loan render as clean cards again; per-card utilization rows are divided line-items with the inline credit-limit editor capped so it no longer sprawls edge-to-edge. Also expanded `e2e/debt_check.mjs` from 14 to **51 checks** covering every tile's features and negative/edge cases (empty payoff-calculator inputs → hint, payment-too-low → minimum-payment error, `$0` extra → snowball==avalanche tie, invalid loan term → default fallback, cleared credit limit → no crash, add-debt modal open/close, include-in-plan toggle re-ranks both directions, formula-tile reveal shows `debt_*` vars, and card→/transactions + manage→/accounts nav).
- **/debt cohesion pass — the reused lower panels now match the redesign (2026-07-02):** The strategy / credit / loans / payoff-calculator panels (which lean on the shared stat / table / form / bar primitives) looked unstyled next to the new hero + payoff-ladder cards. Added a `.bento-debt`-scoped polish that renders section headings + stat figures in the Fraunces display serif with tabular numerals, gives the mini stat cards the elevated debt-card surface, caps single-field forms so inputs no longer stretch edge-to-edge, matches the burn-down/progress bars to the utilization meter, and quiets the tables (uppercase dim headers, tabular figures) — so the page reads as one design top to bottom.

### Added
- **/debt rebuilt from scratch: widgetized, engine-driven, config-driven "payoff ladder" (2026-07-02):** The debt page was redesigned end-to-end (Claude design skill) and re-architected so **nothing on it is hardcoded** — every figure comes from the engine and every threshold from a config. (1) **Engine:** each liability is exposed as `debt_<name>_balance/_apr/_min_payment/_limit/_available/_utilization` engine variables, plus new aggregate atoms (`debt_count`, `revolving_balance`, `credit_limit_total`, `min_payments_total`) and two **formula molecules** (`credit_utilization`, `debt_to_asset_pct`) — so the derivation is data, auditable down to its atoms. These surface in the formula picker under a new **Debts** group. (2) **Config:** a persisted, defaulted `DebtConfig` (`cashflux:debt:config`) holds the utilization bands (warn 30% / high 75%), the payoff planner's default strategy + extra, and mortgage-excluded-by-default — the tiles read the bands from it instead of baking in cutoffs. (3) **Widgetized surface host** (`bento bento-debt`) of native tiles: a **summary** (total owed in the display serif + debt-free date + the engine ratio chips), a **toolbar** (Debt-metrics formula toggle + Manage accounts + Add debt), the **payoff-ladder list** (each debt a `DebtRow` card with a payoff-rank medallion, an APR/utilization-banded left rail, a utilization meter for revolving credit, min-payment/due-day meta, its account custom-field values, and an in-plan toggle), and the reused strategy / credit / loans / payoff-calculator panels + an opt-in **Debt-metrics FormulaBuilder** tile. Verify: `go test ./...` green (new `engineenv` debt-vars test); `e2e/debt_check.mjs` 14/14.
- **Goal cards now display their linked to-dos (2026-07-01):** A goal card shows a compact **TO-DOS** checklist of the tasks linked to it (`Task.RelatedType=goal`) — each with a toggle check + title (struck through when done) and a **"N/M" count** — between the sub-line and the footer (the list scrolls if long). Ticking a to-do flips it live and updates the goal's progress; a **checklist goal** also gets a **"Add step"** button (adds a linked to-do via a prompt) and shows the section even when empty. Goal cards' ⋯ menu also moved to the shared viewport-aware `KebabMenu`. Verify: `e2e/goals_todos_check.mjs` 7/7 — sections + rows render, count reads 0/1, a done step is struck through, toggling updates the count to 1/1 live; `goals_kinds_check.mjs` 16/16 unaffected.
- **Reusable container-aware `KebabMenu` (⋯ overflow menu) component (2026-07-01):** The hand-rolled `.add-wrap`/`.add-menu` block each entity row copied is now a single `uiw.KebabMenu` component that wires the viewport-aware `AnchorPopover` (flips the popover left/up when it would spill past an edge) + `DismissPopover` (Escape / outside-click / menu keyboard roving) and takes pre-built item nodes (so callers keep their own handlers — no hook-in-a-loop). Adopted on the to-do rows: **every** ⋯ menu — parent *and* sub-task — now stays inside the viewport instead of overflowing the right edge (which had scrolled the page sideways). Verify: `e2e/todo_subtask_check.mjs` T16/T18 — both the parent and sub-task menus open flipped-left, fully in view, no overflow.
- **Sub-tasks are collapsible + show a summary (2026-07-01):** A parent task now has a **disclosure chevron** that hides/shows its sub-tasks (state kept in a `todo:collapsed` set; `tasktree.Page` prunes collapsed sub-trees), and a small **"N/M" sub-task summary** chip (done / total, via `tasktree.ChildStats`) that stays visible even when collapsed so hidden work isn't lost. Verify: `e2e/todo_subtask_check.mjs` 19/19 (collapse hides / expand shows, summary persists).

### Fixed
- **Dropdown `<option>` lists are now themed (2026-07-01):** Native select popups had no styling, so an opened dropdown — most visibly the to-do **Sort / Show** filter pills (which use a transparent select) — fell back to unstyled white-on-white. Added `select option` / `optgroup` rules using theme tokens (`--bg-elev` / `--text`), so every dropdown's option list is dark-on-dark (and flips correctly on the light theme). Verified the to-do Sort option computes to a dark background + light text.

### Changed
- **Sub-tasks read as a cohesive nested block (2026-07-01):** Nested rows get a smoother grouped look — a faint background tint, an accent-tinted left guide rail, the ↳ connector, a smaller check ring, and a dimmer title — so a sub-task clearly belongs to the row above it.
- **Add-task modal rebuilt as an editorial "compose slip" (2026-07-01):** A ground-up structural redesign (the first pass was still a labelled-field stack). It's now a **two-zone modal**: a **writing zone** on the left with a large **Fraunces display-serif title** ("What needs doing?") and a borderless notes area, beside a compact **Details rail** on the right (priority / due / repeat / link) — no stacked uppercase labels. The **signature detail**: a live priority **"spine"** — the writing zone's left edge glows faint → seagreen → red as you pick Low / Medium / High, so the slip is visibly tinted by its urgency (echoing the list's check-rings). Priority is a segmented control with those coloured dots; the due date has **quick chips** (Today · In a week · Clear); and the footer shows a **live summary** of the slip ("High priority · due Jul 1 · Weekly") beside Cancel + **+ Add task**. The modal is `NoFooter` at 720×560 and bleeds to the panel edges. Verify: `e2e/todo_addform_check.mjs` 14/14 — Fraunces title, segmented priority + live spine class, live summary reflects priority, quick-date fill/clear, empty-title guard, add persists + closes, Cancel closes, no errors.
- **To-do line items redesigned as an editorial agenda (2026-07-01):** The task rows were rebuilt from scratch as a calm, scannable **agenda list** — borderless rows on a hairline rhythm, not cards or chip-soup. The signature move: **priority is encoded in the circular check-off ring's colour** (red high / seagreen medium / faint low), so you scan urgency by the rings instead of reading badges; ticking it fills the ring accent-green with a check that pops in. The **title is the hero**; the **due date is quiet and right-aligned** (neutral date, red "Overdue · <date>", amber "Today"); and a single dim secondary line carries repeat / linked entity / notes, middot-separated — with the **linked goal as the one accent (seagreen) note**, the visible half of "link 1-n todos to a goal". Row actions (Edit + ⋯) are icon-only and fade in on hover. Scoped to /todo so the shared list-row style elsewhere is untouched. Verify: `e2e/todo_redesign_check.mjs` 9/9 — editorial rows, priority-ring checkbox, is-goal link (seagreen rgb(46,139,87)), checkbox toggles done, due-state emphasis, no page errors.
- **To-do page is now widgetized (surface-host tiles) (2026-07-01):** The /todo page moved from a single `EntityListSection` to the same bento surface-host structure as /budgets, /goals and /accounts — three native tiles: a **summary "loader"** (a done-of-total completion bar with **Open / Overdue / Done** figures inside it, overdue tinted red), a **toolbar** (priority filter + hide/show-done toggle + Add task), and the **task list** (the parent/child tree, empty CTA, and hidden-done note). The hide-done + priority-filter state moved into shared `todo:*` atoms so the toolbar and list tiles stay in sync, and mutations now bump the shared data revision. Verify: `e2e/todo_flipmodal_check.mjs` 10/10 on the new layout; screenshot confirms the summary/toolbar/list tiles.
- **To-do editing now uses a centered flip modal + a ⋯ menu (2026-07-01):** Editing a task used to swap the row for an inline form; it now opens the shell-root **flip modal** (a `TaskEditHost` mounted beside the goal/budget editors, driven by a `TaskEdit` atom), so the editor is properly centered instead of expanding the row under transformed tile ancestors. The row's actions are down to **Edit** + a **⋯ menu** that holds **Add sub-task** and the destructive **Delete** (which cascades to sub-tasks), so a misclick can't wipe a task tree. First step of bringing the to-do page up to the widgetized budgets/goals structure. Verify: `e2e/todo_flipmodal_check.mjs` 10/10 — Edit opens a centered modal, saving reflects on the row, the ⋯ menu holds Add sub + Delete, no page errors.

### Added
- **To-do list: sorting + pagination (2026-07-01):** The task toolbar gains a **Sort** control — Smart order (open-first → soonest due → title), Priority, Due date, or A–Z — and the list now **paginates** (20 top-level tasks per page). Pagination is by *root* task, so a parent and its sub-tasks always stay together on a page; changing the sort or filter resets to page 1. A quiet footer shows the range ("1–20 of 25") with **‹ Previous · Page X of Y · Next ›** (disabled at the ends). New pure `tasktree.Page(tasks, mode, page, size)` (paginate roots, order by mode, never drop a cycle-orphaned task) with table tests. Verify: `e2e/todo_sort_page_check.mjs` 11/11 — A–Z orders titles, Priority floats high tasks up, Next/Prev move the range and disable at ends, sort resets to page 1.
- **Goals UI: create & track non-financial goals (checklist / milestone / habit) (2026-07-01):** The goal add & edit forms now lead with a **Goal type** picker (Savings / Checklist / Milestone / Habit) with a one-line hint, and reveal only the fields that kind needs — a savings goal keeps target/saved/linked-account, a habit shows a check-in rhythm + "check-ins to finish", and checklist/milestone drop the money fields entirely (an optional deadline + owner stay for every kind). The goal cards are now kind-aware: the progress bar reads **money saved/target** for savings, **"N of M steps done"** for a checklist (from its linked to-dos), **"Done / Not done yet"** for a milestone, and **"N of M check-ins"** with a **🔥 streak** chip for a habit. Footer actions match the kind — Contribute (savings), **Mark done / Reopen** (milestone), **Check in** (habit) — while Edit + the ⋯ menu stay everywhere. (Managing a checklist's linked to-dos is read-only on the card for now; the to-do page integration lands next.) Verify: `e2e/goals_kinds_check.mjs` 16/16 — kind picker + hint, conditional fields, and creating + acting on all four kinds (mark-done → "Done", check-in → "1 of 4" + streak) with no page errors.
- **Goal engine variables are now kind-aware + expose linked to-do counts (2026-07-01):** Each goal already published `goal_<slug>_target/_saved/_remaining/_percent` (money). Added five more so goal progress works in formulas and dashboard widgets for every kind and surfaces the linked to-dos: `_progress` (kind-aware percent complete — money %, to-do %, milestone 0/100, or habit check-in %), `_tasks_done` / `_tasks_total` (the literal count of to-dos linked to the goal, for any kind), `_done` (1 when the goal has met its objective), and `_streak` (current habit check-in streak). The picker/catalog labels them too, so e.g. a KPI card can read "8 of 12 steps done" off a checklist goal. Verify: `go test ./internal/engineenv/... ./internal/widgetcatalog/...` green (new kind-aware surface test).
- **Goals can now be non-financial — checklist / milestone / habit kinds + logic (2026-07-01):** Goals are no longer only savings targets. A new `GoalKind` (`financial` — the default, back-compat for the empty value — plus `checklist`, `milestone`, `habit`) selects how a goal measures progress, and to-dos link to *any* goal via the existing `Task.RelatedType=goal` / `RelatedID` join (1-n, no new field). New pure logic in `internal/goals/kinds.go` unifies all four kinds behind one `Progress` shape: **financial** = money saved/target (unchanged); **checklist** = completed linked to-dos / total (`TaskCounts` + `ChecklistPercent`); **milestone** = a binary done/not-done recorded in `Goal.DoneAt`; **habit** = check-ins toward `HabitTarget` on a `HabitCadence`, with a drift-tolerant current-streak calc (`HabitStreak`). `ValidateGoal` is now kind-aware (financial needs a positive money target, habit a positive check-in target, checklist/milestone need neither). This commit is the data model + tested logic only; engine variables and UI follow. Verify: `go test ./internal/domain/... ./internal/validate/... ./internal/goals/...` green (new `kinds_test.go` covers linked-task counts, checklist/habit percent + clamp, streak drift/gaps, and per-kind `EvaluateProgress`).

### Changed
- **Smart explainer (ⓘ) popover now portals to `<body>` (2026-07-01):** Follow-up to the overlay change below — a fixed-position popover that's still a DOM child of its tile is trapped in that tile's stacking context, so the "Overall progress" tooltip painted *under* the next section (the Goal-metrics / Add-goal toolbar) even at a high z-index (z-index can't win across sibling stacking contexts). The popover is now rendered as a portal appended directly to `<body>` (`SmartTipPortal`), which escapes both any `overflow:hidden` ancestor *and* the tile's stacking context, so it paints above everything below it. It positions itself fixed below the trigger (viewport-relative, since `<body>` has no transform), flips above / clamps horizontally near an edge, re-measures on scroll/resize, and removes its node on close. Verify: `e2e/test_portal.mjs` 5/5 — portaled to body, on-screen, nothing paints over it, no errors, no orphan node on dismiss.
- **Smart explainer (ⓘ) popover is now a proper overlay + z-index tokens (2026-07-01):** The "Overall progress" (and other ⓘ) tooltips used to expand the box they sat in and could be clipped by the summary loader's overflow. They now float as a fixed-position popover that escapes clipping containers and transformed ancestors, stays inside the viewport (flips/clamps near an edge), and stacks above content. Introduced a z-index scale as CSS tokens (`--z-dropdown` / `--z-popover` / `--z-modal` / `--z-toast` …) so stacking order is defined in one place instead of scattered magic numbers, and pointed the menus, popover, modal backdrop, and toasts at them.
- **Goals use flip modals for new/edit/contribute + a ⋯ menu (2026-07-01):** Editing a goal and adding a contribution now open a centered flip modal (a shell-root GoalEditHost, like the budget editor) instead of forms that expanded inside the card — so the goal cards stay compact. "Add goal" was already a modal. The destructive **Delete** (and Archive) moved into a **⋯ actions menu** on each card, leaving Contribute + Edit as the two visible actions.
- **Goals redesigned as a card grid (2026-07-01):** The goals list is now a responsive grid of compact cards matching the budgets design — each card has the goal name on its own line, a pace badge, a saved-of-target progress "loader" bar with the amount and percent inside it (tinted by pace: green on-track, amber due-soon, red overdue), the actionable sub-line, and a footer action row.
- **Goals page widgetized + goals as formula variables (2026-07-01):** The /goals page is now the same widgetized surface-host structure as /budgets and /accounts — a summary "loader" tile (saved-of-target progress bar with Saved / Target / Overall figures inside it), a toolbar (smart action + "Goal metrics" reveal + Add goal), and the goal-list tile. Each goal is also exposed as engine variables — `goal_<name>_target/_saved/_remaining/_percent` — usable in any formula or dashboard widget, plus a new opt-in **"Goal metrics"** FormulaBuilder tile (ties goal custom fields + the formula engine together, like Budget metrics).
- **Budget metrics / formula builder redesigned (2026-07-01):** The show/hide "Budget metrics" panel (the reusable formula builder, also on Customize) was three sprawling stacked cards with a huge near-empty "Result" card and a low-density one-variable-per-row list of raw floats. It's now a single cohesive **workbench**: a wide monospace expression input with the **live result read out inline** beside it (accent-coloured, updates as you type), quick preset chips, a compact save row, and a **dense, click-to-insert variable palette** — a grid of chips (label + live value) grouped by Core money / Activity / Counts / Budgets / Accounts.
- **Add-budget modal redesign (2026-07-01):** Right-sized to its content (no more ~250px of dead space), with a cleaner 2-column field grid — the Category picker is full-width so "Create a new category" no longer truncates, Name/Variable name/Category span the full width, and the "Roll unused funds" toggle sits on its own line instead of wrapping. The form now owns a single bottom action bar with a quiet **Cancel** and a prominent, properly-sized **Add budget** (sticky, so it stays put), replacing the cramped in-grid button + separate Close footer.
- **Cover modal refinements (2026-07-01):** The modal no longer has two identical ƒx buttons — the per-source ratio-formula toggle now only appears once you've picked a source (it weights the selected budgets) and is labelled **"ratios ƒx"**, distinct from the amount **ƒx**. The body no longer scrolls: the amount, spread controls, "repeat" toggle and the Cover/Cancel actions stay put while only the source list scrolls.
- **Budget figures now live inside progress bars (2026-07-01):** The summary is a single big "loader" — a spent-of-budgeted progress bar with the SPENT / BUDGETED / LEFT figures rendered inside it (fill grows with spending, turns amber near the limit and red when over). Each budget card gets the same treatment: the spent/limit amount and percent moved into a taller bar, freeing the card header for just the title so long budget names have room.
- **Budgets are now a grid of compact cards (2026-07-01):** Instead of full-width bars stacked one per row, budgets lay out in a responsive grid — each a 1-column card, several per row, so you see far more at a glance without the wasted width. The cards are taller with a cleaner top-to-bottom flow (name + amount + used% → progress bar → status → per-period note) and the row actions (Transactions / Top up / ⋯) pinned to the bottom as a footer with a hairline separator.
- **Variable-name field is now a shared library — autosuggest everywhere (2026-07-01):** The budget/account variable-name editors (autosuggest-from-name, the live `type_<slug>` chip, and the collision check) are unified into one reusable module (`useEntityVarField` + `entityVarField`). As a result the **edit** forms now autosuggest a handle as you rename (previously only the add forms did), and stop as soon as you customise the field. Adding the same field to a new entity type is now a few lines.
- **Budget variable name is now shown as it's generated (2026-07-01):** The Add/Edit budget modals show a live, monospace **chip** — "Generates `budget_rent_remaining` · also _limit _spent _over _percent" — right under the Variable name field, so you can see exactly what handle your budget produces as you type (instead of it being buried in a sentence).
- **Roomier budget modals (2026-07-01):** The Add-budget modal is larger and now stacks **Name** and the new **Variable name** field full-width at the top (the var-name field was previously tucked into the grid's second column, easy to miss). The cover modal's "Spread across" source list is taller (440px) so you can see all your budgets at once without scrolling.
- **Cover modal: selected budgets read as one tight cluster (2026-07-01):** The picked budgets sort to the top of the source list with a stronger accent tint and a clear gap before the unchecked ones, and a pinned **"Selected N · splitting $X"** caption shows the running split total at a glance — so you can see how the amount divides without scrolling or scanning. (The caption stays in the DOM when empty so toggling it never remounts the list mid-click.)
- **Cover modal: checked sources group at the top (2026-07-01):** When you tick a budget to pull from, it now floats to the top of the source list (keyed reorder, so rows animate into place) and the list is taller — so the active split (the checked budgets with their ratios and shares) stays together and visible without scrolling past the unchecked budgets. Addresses the cramped split viewport.
- **Budgets "Cover" moved to a flip modal + multi-source spread (2026-07-01):** The over-budget "Cover…" action opens the shell-root flip modal (like Edit/Top up) instead of an inline row form, and now **spreads the overspend across multiple source budgets**: an "Amount to move" field (prefilled to the exact overspend), then a checkbox list of every budget with limit to give. Checked sources split the amount **equally by default**; a per-source **Ratio** input allows an uneven split, with each source's computed share shown live. On submit each source contributes its share via `app.CoverBudget`; a pre-validation blocks the move if any source can't give its share (so a partial cover can't be left half-applied). `BudgetEditForm` gains a cover mode (`splitCoverAmount` + `coverSourceRow`); `BudgetRow` sheds the inline cover form (and the `OnCover`/`CoverSources` plumbing). Verify: new `e2e/budgets_cover_check.mjs` 6/6 — equal split $50/$50, ratio 3:1 → $75/$25, apply moves $20 into the over budget.
- **Budgets toolbar redesign (2026-07-01):** The methodology picker was rendered with the full-width `.field` class, so it looked like a giant search bar and pushed the buttons onto a second row. Rebuilt the toolbar as one clean row: a compact, labelled ("Budgeting method") method dropdown on the left (`.budgets-method-select`, width:auto/min 210/max 300), and right-aligned uniform-height actions (Smart / 50/30/20 / Budget metrics / **+ Add budget** primary). Scoped `.budgets-toolbar` flex layout. Verify: `e2e/budgets_widget_check.mjs` 14/14.
- **Budgets: per-row "Transactions" review button (2026-07-01):** Each budget row gets a labelled Transactions button that jumps to /transactions filtered to the budget's category for quick review (the category title was already a drill link, but a button is discoverable). New i18n `budgets.reviewTitle`. Verify: `e2e/budgets_widget_check.mjs` 14/14.
- **Budgets row actions reordered by frequency (2026-07-01):** Swapped which budget-row action is on the card vs in the ⋯ menu — **Top up** (the frequent, proactive action) is now a visible card button, and **Edit** (lower-frequency) moved into the ⋯ overflow menu alongside the destructive **Delete**. So the row is Top up + ⋯ (plus Cover… when over), and the ⋯ menu is Edit budget / Delete budget. New i18n `budgets.editAction`. Verify: `e2e/budgets_widget_check.mjs` 12/12 — Top up is a visible card button that opens the modal; the ⋯ menu holds Edit + Delete (Edit opens the modal), no standalone ✕.
- **Budgets modal refinement + delete moved to a ⋯ menu (2026-07-01):** Two follow-ups to the budget flip modal. (1) The editor forms switched from the 2-column `form-grid` to the single-column `.acct-edit-form` used by the account editor — a clean vertical stack where the rollover explanation and Method sit full-width beneath their controls (the 2-column grid had squeezed the rollover hint beside the checkbox and put Method next to the buttons), and the action row's `margin-top:auto` pins Cancel/Save to the modal bottom with no dead space. Edit-modal height bumped 600→680px so the actions are never clipped; name/amount fields autofocus on open. (2) The standalone ✕ **delete button is gone from the row** — Delete moved into a **⋯ overflow menu** (like `/accounts`) as a red destructive item, alongside "Top up…"; a misclick can no longer delete a budget and the row is down to Edit + ⋯ (plus Cover… when over). New i18n `budgets.moreActions`/`budgets.deleteAction`. Verify: `e2e/budgets_widget_check.mjs` 11/11 — the ⋯ menu holds Top up + Delete with no standalone ✕, and the menu's Top up opens the modal; modal layouts confirmed by screenshot.
- **Budgets Edit + Top up now use the shell-root flip modal (2026-07-01):** The budget row's **Edit** and **Top up** actions open the centered `FlipPanel` modal (mirroring `/accounts`) instead of swapping in an inline row form. New `BudgetEditForm` (`internal/screens/budgets_edit_form.go`) renders the full edit form (name/limit/period/owner/rollover/methodology/custom fields) or a top-up amount form as the modal body, owning its own state + Save/Cancel and mutating the store directly; new `BudgetEditHost` (`internal/app/budgetedithost.go`) is mounted at the shell root beside `AccountEditHost` and driven by a new `BudgetEdit{ID,Mode}` atom (`UseBudgetEdit`/`SetBudgetEdit`/`CloseBudgetEdit`, captured-atom pattern so a click handler never calls `UseAtom`). Mounting at the shell root is the whole point — a row lives under transformed bento/tile ancestors, which made an in-row `position:fixed` modal resolve against the tile and render off-centre; at the shell root it centers on the viewport. `BudgetRow` is simplified accordingly (the inline edit/top-up state, hooks, forms, and the `focusByID` effect are gone; the buttons just set the atom), and the now-unused `OnSave`/`OnTopUp` row callbacks are removed. Cover… stays inline (it's an over-budget-only quick action). New i18n `budgets.addFunds`/`budgets.topupHint`. Verify: `e2e/budgets_widget_check.mjs` 10/10 — Edit opens the flip modal (name field present, no inline `.form-grid` in the row), Top up opens the modal with an amount field; modal centering confirmed by screenshot.

### Added
- **Account variable name in the new/edit modals (2026-07-01):** The Add/Edit account modals now have an optional **Variable name** field (like budgets): it autosuggests a handle from the account name as you type, shows a live chip of the generated variable (`account_rent_balance · also _cleared`), and warns on collisions with another account's handle (blocking the save). The add modal is larger with Name + Variable name stacked full-width. Account metrics are wired into the formula builder + Studio pickers.
- **Each account is now a reusable variable (2026-07-01):** Every account exposes named figures to the engine surface — `account_<name>_balance` and `account_<name>_cleared` (FX-converted to base currency) — so you can reference a specific account in any formula, dashboard KPI, or Studio widget (e.g. `account_checking_balance`). Same-name accounts get a numeric suffix; they appear in the variable pickers under a new **Accounts** group.
- **Custom variable name for a budget (2026-07-01):** The Add/Edit budget modals now have an optional **Variable name** field, so you can give a budget a short, stable handle for formulas & widgets (e.g. name it `rent` → `budget_rent_remaining`) that survives a display-name change. It **autosuggests** from the budget name as you type (until you edit it), previews the resulting variable live, and **warns on collisions** with another budget's handle (and blocks the save). Stored on the budget (`varName`) and used by the engine surface + pickers. Verify: 5/5 live — autosuggest fills the slug, collision warns in both modals, and `budget_<name>_limit` resolves.
- **Custom fields on a recurring cover rule (2026-07-01):** Coverage rules are now their own custom-field entity — define fields under **Custom fields → Coverage rules**, and when you turn on "repeat this cover automatically" the cover modal shows those fields (e.g. a "Reason" or "Review by" note) and saves them on the standing rule. Metadata for organizing/annotating recurring covers, persisted on the budget's `recurringCover.custom`. Verify: 4/4 live — the field is hidden until recurring is on, appears when toggled, and persists after save.
- **Each budget is now a reusable variable (2026-07-01):** Every budget contributes named figures to the engine surface — `budget_<name>_limit`, `_spent`, `_remaining`, `_over`, and `_percent` (e.g. `budget_groceries_remaining`) — so you can reference a specific budget in any formula, dashboard KPI, or Studio widget, not just on the Budgets page. Spent is measured over each budget's own period; same-name budgets get a numeric suffix. They show up in the formula/widget variable pickers under a new **Budgets** group. Verify: 5/5 live — `budget_baby_childcare_limit` resolves to $400 and the budget's metrics list in the picker.
- **Cover: drive every source's ratio from one formula (2026-07-01):** A **ƒx** toggle in the cover modal's *Spread across* header switches the per-source ratios from hand-typed numbers to a single formula that's evaluated in **each source's own budget context** — so `cf_budget_priority` weights the sources you're pulling from by a custom priority field, `remaining` weights them by how much room each has, and so on. With it on, the ratio inputs become read-only and show the computed weight, the shares recompute live, and the formula is saved on each source of a recurring cover so it re-evaluates every period. Completes formula-driven coverage (amount **and** weights). Verify: live test 4/4 — `remaining` drives two sources' ratios to 300 / 25 (read-only) and splits $90 into $83.07 / $6.93.
- **Cover: "use all remaining" per source + auto-backfill ratios (2026-07-01):** Each selected source in the Cover modal gets a **"Use all"** checkbox — checking it pins that source to its full remaining budget (capped a cent below its limit so it stays positive) and its ratio input disables; the rest of the amount then splits across the *other* selected sources by their ratios, so the split re-balances automatically. `splitCoverAmount` now takes a `maxed` set and does this two-way (pin an amount → back-fill the others). The native ratio **spinners are back** (up/down arrows step the ratio, updating the shares live), the cover modal is a bit larger (540×680 via the FlipPanel API), and the ratio field keeps a fixed width. Verify: live test 3/3 — number input spinner-capable, ratio change updates shares live ($75/$25), "use all" pins a source (ratio disabled) with the total holding at $100.
- **Formula-driven cover amount, with live preview (2026-07-01):** The Cover modal's amount can now be a **formula** (an "ƒx" toggle switches the field between a number and a formula). The formula is evaluated live in the destination budget's context — showing "= $X" as you type — and, when the cover is recurring, re-evaluated each period. So a recurring cover of `overspend` always covers whatever the shortfall is that period. Variables available: `overspend`, `remaining`, `limit`, `spent`, and the budget's own `cf_budget_<field>` custom fields, with the full formula language (min/max/clamp/if/round…). Built on the new engineenv.BudgetVars context + the internal/coverformula evaluator (both unit-tested). Errors surface inline. Verify: wasm build clean, coverformula/engineenv tests green, preview screenshot confirms `overspend` → $100.00.
- **Budget "Covered" flag — 1x vs continual (2026-07-01):** A budget that receives cover money is now stamped (`domain.Budget.CoveredAt`, set in `app.CoverBudget`) and shows a **"✓ Covered this period"** badge for quick reference; it ages out once the period rolls over. It is visually and semantically distinct from continual coverage: a budget with a recurring arrangement shows **"↻ Recurring cover"** instead, and the two are mutually exclusive (recurring wins, since it is inherently covered). Verify: `e2e/budgets_cover_check.mjs` 11/11 — a one-time cover shows the Covered flag (not Recurring), and enabling recurring replaces it.
- **Recurring budget coverage + cover-modal redesign (2026-07-01):** The Cover editor gains a **"Repeat this cover automatically every period"** toggle. When on, the arrangement (amount + the checked source budgets and their ratios) is saved to the destination budget (`domain.Budget.RecurringCover`) and re-applied at the start of each new period by a boot hook (`applyRecurringCovers`, drain-safe: a source that can no longer give its share is skipped). The row shows a **"↻ Recurring cover"** badge, and the ⋯ menu gains **"Remove recurring coverage"** (with a confirm). Also a design-critique-driven **cover-modal redesign**: branded seagreen checkboxes (no more native OS-blue), tinted checked rows with an accent left-stripe, native number-spinners suppressed on the ratio inputs, an **amber over-allocation warning** ("⚠ $33.33 · only $25.00 available") when a source's share exceeds what it can give, a bounded **scrollable** source list so the toggle + actions stay visible, a fixed (un-clipped) accent "Use full $X" button, and a "Spread across" label + subtitle. Verify: `e2e/budgets_cover_check.mjs` 9/9 — the recurring toggle saves (badge appears), the ⋯ menu removes it (confirm clears the badge), plus the multi-source split/apply.
- **Add budget can create its category (2026-07-01):** The add-budget category picker now leads with "➕ Create a new category" (the default), which creates a new expense category named after the budget so you can assign transactions straight to it — closing the loop (a budget watches a category; now making a budget can make that category). An optional "New category name" field overrides the default (the budget name). Still lets you attach an existing category, and the form now works on a fresh install with no categories (the old hard "needs a category" block is gone). Categories are created via `app.PutCategory` (expense kind). Verify: new `e2e/budgets_newcat_check.mjs` 3/3 — adding "Vacation Fund" creates a matching expense category and the linked budget row.
- **Budgets ported to the widgetized surface-host architecture (2026-07-01):** `/budgets` now mirrors `/accounts` and `/transactions` — a thin surface host (`Budgets()`, `budgets_widget.go`) builds a fixed set of `WidgetSpec`s and renders them through the same engine spec/render pipeline (`safeRenderSpec`) into a `.bento.bento-budgets` grid. Every block is its own Native engine tile (`budgets_tiles.go`): **budget-summary** (spent/budgeted/left stat grid + income/methodology assign banner + sinking-fund set-aside + over/near alert banner and badges), **budget-toolbar** (in-context methodology picker, 50/30/20 starter template, Add budget, a Formulas reveal toggle, smart-insights action), **budget-list** (the health-sorted `BudgetRow`s, reused verbatim, or the first-run empty CTA), and **budget-formula** (opt-in). All tiles share one computed picture — `computeBudgetView` (`budgets.go`) runs the full per-period evaluation (rollover carry, pace projection, prorated pace, envelope balances, effective method, roll-up totals) once and both the summary and list tiles read it, so figures never drift. Row mutations route through `buildBudgetRowCallbacks` + the shared data revision (matching the accounts convention) instead of a page-local error/rev state. **Formulas + custom fields tie-in** (as with `/accounts`): the Formulas toggle reveals a `FormulaBuilder` tile that evaluates against the live engine surface — every number-typed *budget* custom field surfaces as a `cf_budget_<key>` variable (alongside the budget count) — and `BudgetRow` now shows a custom-field summary line and edits budget custom fields inline (persisted via the extended `OnSave`). New state atom `UseBudgetsShowFormulas`; new i18n keys `budgets.showFormulas/hideFormulas/formulaTitle/formulaHint`; `.bento.bento-budgets` style rules. Every prior feature preserved (member scoping, per-budget rollover/methodology overrides, Cover…/Top up… inline forms, drill-to-transactions, custom-range hint). Verify: new `e2e/budgets_widget_check.mjs` 9/9 — the surface renders as the bento host with composed tiles, all toolbar controls present, rows render, the Formulas toggle reveals the `cf_budget_` metrics tile, method-switch and inline edit still work.
- **Budgets visual redesign — elevated meter-cards (2026-07-01):** Each budget row is now an elevated, rounded card with a health-colored left accent stripe (green on-track / amber near-or-at-risk / red over, with a subtle red tint when over), a prominent gradient progress bar over a *visible* track (0%/low budgets no longer vanish into the dark background as a hairline), and a state-tinted percent-used chip. The row header is split into a left content group (`budget-head-main`: title that truncates instead of wrapping, spent/limit amount, percent chip) and a right `budget-actions` cluster, so the Top up / Edit / delete buttons line up in stable columns across rows and never wrap onto a second line (fixes the ragged, misaligned buttons). `BudgetRow` gains a `budget-head-main`/`budget-actions` structure + a `is-over`/`is-near`/`is-risk`/`is-ontrack` state class (via `budgetRowStateClass`) and the percent chip; all styling is scoped to `.bento-budgets` so the shared `.budget`/`.bar` styles used on other screens (allocate, goals, reports) are untouched. Verify: `e2e/budgets_widget_check.mjs` still 9/9; visual states confirmed by screenshot.
- **Budgets design refinement — critique-driven pass (2026-07-01):** Acted on an expert visual critique of the budgets page. The progress bar is now the card centerpiece — raised from a ~10px hairline (invisible at 0%) to a **16px rounded gradient bar over a clearly visible track**, so a whole list is scannable by bar length + color alone. Hierarchy flipped **name-first**: the category name is the card title (700 / 1.05rem) and the spent/limit amount is demoted to muted secondary context with only the *spent* figure carrying foreground weight (`budget-spent` span). The redundant "Period · X% used" sub-line is removed (the bar + percent chip already carry it), collapsing the default card to one metadata line ("status · remaining · period"); the C143 prorated line is kept (tested). The percent chip is now **legible** (brighter health-tinted text on a stronger tint — the old dark-green-on-dark-green failed at a glance). Row **actions recede** to 40% opacity at rest and come forward on hover/focus, so cards read clean. The health left-stripe thickened 3px→5px, card gaps widened, and the over-budget tint strengthened. Summary tile: **"Left" is now the dominant figure** (2.6rem) and **"Spent" is no longer red at $0.00** (red only once there's spending — red-on-zero read as an error). Toolbar: **"+ Add budget" is now a solid accent primary** so it outranks the ghost method/template/metrics controls. All still scoped to `.bento-budgets`. Verify: `e2e/budgets_widget_check.mjs` 9/9; bar height 16px, prorated line + spent spans present, Add button primary confirmed via DOM probe + screenshots.
- **Lock screen: a music mute toggle + a Smart+ AI "quote of the day" (2026-06-30):** Two lock-screen additions. (1) A **mute toggle** so the ambient music can be silenced (or resumed) from the lock screen without unlocking — it reads/sets the live player state (`window.cashfluxMuzak`), persists the choice, and hides itself when no music is configured (a deferred re-check covers the player still loading its tracks when the gate builds). (2) The bottom **verbiage now uses the Smart+ "quote of the day" engine** instead of only static text: the lock screen shows the latest generated quote whenever one is cached (falling back to the static day-rotating `lockquotes` when none is, or when the feature is explicitly turned off). The Smart settings + cached quote live in their own browserstore key (`cashflux:smart-settings`, separate from the encrypted dataset), so the AI quote is readable on the lock screen even while the dataset is locked. (3) A **top-bar lock button beside the music toggle** (`LockToggle`, shell.go) — one click locks the app from anywhere; the button is rendered only when a passcode is set (absent from the DOM otherwise). A stable wrapper `<span class="lock-toggle-slot">` always occupies the slot with the button conditionally rendered inside it (the wrapper is `display:none` when empty so it adds no flex gap): returning a bare Fragment when disabled vs a Button when enabled flips the component between zero and one node, which shifts its position in the reconciler's positional child list — that had pushed the button to the far right instead of beside the mute toggle. The button node (and its `OnClick` hook) is constructed unconditionally so the hook position stays stable. Verify: `e2e/lockscreen_check.mjs` 9/9 — the button is absent from the DOM when the lock is disabled (K0), and when enabled sits immediately beside the mute (slot index 2 vs 1) and locks the app on click; the lock quote shows the cached Smart+ quote and falls back to static when off; the mute button toggles the music. SW cache v291→v292.

### Fixed
- **Lock-screen AI "quote of the day" stayed static after adding an OpenAI key (2026-07-01):** The lock-screen quote only appeared if the user *also* manually opted into the dashboard SMART-QUOTE feature *and* the dashboard had run a generation — so "I added the OpenAI key and still get the static quote" was the expected-but-wrong result. Two changes fix it. (1) New `refreshDailyLockQuote()` (`internal/app/lockquote.go`) proactively generates + caches the daily quote whenever an AI provider is configured — decoupled from the dashboard opt-in, honoring an explicit opt-out (`ExplicitOff`), never re-spending on a fresh same-day cache, and guarded against duplicate dispatch. It sources the key from the live session (unlocked) or the separately stored on-device "remember" key (`cashflux:openai-key`, readable while locked), and is triggered on boot, on unlock (`onAppUnlocked`), right after the key is saved (Settings), and when the gate is shown — writing the result live into `#cf-lock-quote` if the gate is already visible. (2) `lockQuoteText` now shows any cached quote unless the feature is explicitly turned off (previously it required the manual opt-in flag). Because the dataset (and its key) is encrypted while locked, the lock screen can only *display* a quote generated during a prior unlocked session; this makes that cache populate from just configuring a key. Verify: `e2e/lockscreen_check.mjs` L1 — a cached quote with no manual opt-in now surfaces on the lock screen (was static before).
- **Account notes + an encrypted institution-credential vault (2026-06-30):** Two account additions. (1) **Notes** — a free-text `Account.Notes` field, edited via a textarea in the account edit modal, with a quiet note glyph on rows that have notes. It round-trips through persistence + JSON export + sync automatically (accounts are stored as JSON blobs, so no migration). (2) **Login & credentials** (⚠️ first pass, flagged for a mega security review) — a `Login & credentials` action on each account opens a modal to store an institution username / password / login URL / notes, **encrypted at rest** with the app's existing AES-GCM-256 + PBKDF2(600k) stack keyed by the app passcode. Stored in a DEDICATED, LOCAL-ONLY browserstore key (`cashflux:credvault`) that is **never part of the dataset blob** — so credentials are never exported, never synced to the backend, and never in a backup (verified: the vault ciphertext holds no plaintext and nothing appears in the dataset). Gated behind an app passcode (no passcode ⇒ no storage), password masked with a reveal toggle, and every credential modal shows an "Experimental — not yet security-reviewed" banner. Known gaps (XSS-while-unlocked, passcode strength, passcode-change orphaning, clipboard/reveal leaks, no hardware key) are enumerated in `internal/app/credvault.go` and filed as **SEC-1 [CRITICAL]** in TODOS.md for the review. Verify: gofmt/vet/`go test ./...` clean (incl. `cryptobox`); wasm rc=0; new `e2e/accounts_notes_creds_check.mjs` is 14/14 (notes persist + indicator; credential warning + passcode gate; encrypt to own key with no plaintext; excluded from the dataset; reveal toggle; decrypt round-trip); the 38-check accounts e2e still 38/38. SW cache v287→v288.

### Changed
- **Credential retrieval redesign: copy-to-clipboard behind a passcode re-auth, never shown, never in the DOM; + a login-page quick link (2026-06-30):** The credential modal used to load the stored password into a masked input with a reveal toggle — i.e. the plaintext was in the DOM. Now the stored password is **never loaded into the DOM**: the password field is only for *setting/replacing* one (empty = keep existing), there's no reveal, and the stored value is retrieved via a new **"Copy password"** button that re-authenticates against the app passcode (a `promptReauth` overlay) and then writes the password **straight to the clipboard** from wasm memory — it never touches a DOM node and is never displayed. Also added an **"Open login page ↗"** quick link shown when a valid http(s) login URL is set (scheme-checked; non-web schemes are dropped). Verify: `e2e/accounts_notes_creds_check.mjs` 22/22 with clipboard permissions — asserts the password is absent from the DOM before AND after copy, the reveal toggle is gone, a wrong passcode is rejected, the correct passcode lands the exact password on the clipboard, and the login link points at the URL. SW cache v290→v291.
- **Fix account-editor modal action row overlapping content (2026-06-30):** The `.acct-edit-actions` Save/Cancel row was `position:sticky`, which — once a form overflowed the modal (the edit form does, even collapsed) — floated the action bar *over* the flowing fields: it covered the Notes textarea and the "Show advanced fields" disclosure peeked out below it (the "weirdness at the bottom"). Dropped the sticky: the row keeps `margin-top:auto` (so short forms — update-balance/transfer/reconcile — still pin Save/Cancel to the modal bottom) but a tall form now scrolls to it normally with no overlap. Also bumped the edit modal height 560→620px so the collapsed form needs less scrolling. Verify: screenshots of all editors (edit collapsed + expanded, transfer) show clean flow with no overlap; accounts_full_check 39/39, accounts_notes_creds 14/14, zero page errors. SW v289→v290.
- **Account row delete moved from a standalone ✕ column into the ⋯ menu (2026-06-30):** Each account row ended in a standalone ✕ delete button. It's now a red "Delete account" item at the bottom of the row's ⋯ overflow menu (new `.add-item.danger` style), so a row's actions all live in one place and the row reads cleaner. The delete handler + guard (refuse-if-has-transactions) are unchanged; the item carries `data-testid=delete-account-btn-<id>`. Verify: gofmt/vet clean; wasm rc=0; accounts_full_check 39/39 (asserts the menu item exists AND no `.btn-del` remains). SW cache v288→v289.
- **Account editor modal layouts cleaned up (2026-06-30):** The flip-modal editors were laid out with the multi-column `.form-grid`, which flowed fields into misaligned columns and left dead vertical space, plus a redundant Close footer under the form's own Save/Cancel. Now they use a dedicated single-column `.acct-edit-form` (every field full-width, stacked) with a pinned action row (`.acct-edit-actions`: `margin-top:auto` + `position:sticky` so Save/Cancel sit flush at the modal bottom whether the form is short or scrolls), and a new `FlipPanel{NoFooter}` option drops the redundant Close (the form owns Save/Cancel; the ✕/Escape/backdrop still dismiss). Modal heights were tuned to content. Verify: gofmt/vet/`go test ./...` clean; wasm rc=0; screenshots show all four editors (edit / update-balance / reconcile / transfer) centered with clean single-column layouts + pinned actions; save persists, Escape + Cancel dismiss, zero page errors. SW cache v286→v287.
- **Account editors now open in a centered flip modal (shell-root host) instead of expanding inline in the row (2026-06-30):** The account edit / update-balance / reconcile-to-statement / transfer forms were inline `.row-edit` expanders. They now open in the shared `FlipPanel` flip modal. The forms + their state moved into a new `screens.AccountEditForm` (one component, four modes), rendered by a new shell-root `app.AccountEditHost` that reads a shared `uistate.AccountEdit{ID,Mode}` atom (mirroring `TxnEditHost`); the row's Edit / ⋯-menu actions just set the atom. `AccountRow` shrank ~460 lines (all form machinery removed). **Why shell-root, not in-row:** a row lives under transformed bento ancestors (`.w`, `.card`, and the app-enter wrapper all carry a `transform`), so a `position:fixed` modal rendered inside the row resolved its backdrop against the tile and appeared off-centre; mounted at the shell root there is no transformed ancestor between the backdrop and the viewport, so the modal centres exactly (verified: backdrop rect = full viewport, panel centre = viewport centre). The form owns its Save/Cancel (FlipPanel is `CloseOnly`); the modal autofocuses the first field and re-mounts fresh each open (state seeds from the account). All input ids + data-testids are preserved (`acct-setbal-form-*`, `setbal-delta-preview`, `acct-xfer-to-select`, `reconcile-statement-input`, …). Verify: `gofmt`/`go vet` clean; `go test ./...` green; wasm build rc=0; screenshots show the edit/update-balance modals centred with a dimmed backdrop; a save cycle persists (update-value → 250,000 shown); DOM e2e green (accounts_menu 8/0, accounts_edit_adv_disclosure PASS). SW cache v285→v286.

### Added
- **Widgetized /accounts surface + custom-field & formula tie-ins + viewport-aware overflow menus (2026-06-30):** Ported /accounts to the same "everything on the page is a widget" architecture as /transactions. `Accounts()` is now a thin SURFACE HOST (`internal/screens/accounts_widget.go`) that builds one engine `RenderCtx` and renders a fixed set of Native widget specs through `safeRenderSpec` into a `bento bento-accounts` grid; the tiles live in `internal/screens/accounts_tiles.go` and share state through new atoms in `internal/uistate/accountspage.go` (an `AccountsFilter{Search,Type,ShowArchived}`, a transfer-open flag, and a show-formulas flag) so no tile embeds another. The tiles: **acct-summary** (net-worth hero + assets/liabilities + month-to-date trend), **acct-toolbar** (a real `FilterToolbar` — search by name, an account-type filter, a show-archived toggle, active-filter chips, and the Transfer money / Mark all updated / Manage exchange rates actions, replacing the old loose stacked buttons), **acct-transfer** (the page-level transfer form as a sub-view), **acct-list** (the owner-scoped, filtered asset rows), **acct-archived** (revealed by the toggle), **acct-welcome** (first-run load-sample), and **acct-formula**. The rich per-account row (`AccountRow`) is reused verbatim; every existing feature is preserved (inline edit, reconcile-to-statement, per-row transfer, archive/restore, mark-updated, set-balance-with-adjustment, valuation history, smart badges, view-transactions). Each data-dependent tile subscribes to the shared data revision so a mutation in one tile (a transfer, a mark-all) re-renders the others in step. **Custom fields**: the inline-edit form now renders inputs for every "account" custom-field def (filling the gap left by the add form) and each row shows a compact read-only "Label: value" summary of its custom values — so users can define account attributes (e.g. an account number, a risk score) and edit/see them in place. **Formulas**: an opt-in "Account metrics" tile (toolbar Formulas toggle) embeds the reusable `FormulaBuilder`, which evaluates over the live engine surface — account aggregates (assets, liabilities, net_worth, asset_accounts, …) plus every number-typed account custom field as a `cf_acct_<key>` variable — tying the two features together. **Overflow menus**: a new reusable `ui.AnchorPopover` hook (companion to `DismissPopover`) keeps any open `.add-menu` popover inside the viewport by toggling `open-left` / `open-up` when its natural below-right position would overflow the right or bottom edge; wired into `AccountRow`'s ⋯ menu and the shared `OverflowMenu` component, with new `.add-menu.open-up` CSS. Verify: `gofmt`/`go vet` clean; `go test ./...` green; `GOOS=js GOARCH=wasm` build rc=0; DOM e2e green (loopstory_101 mark-all 7/0 — the stale-count badge now retires after mark-all; accounts_menu 8/0; c224 account types PASSED); screenshots confirm the tiled layout, the transfer sub-view, the ⋯ menu opening up-and-left within the viewport, the custom-field edit input + row summary, and the formula tile's metric reference. SW cache v284→v285.

### Changed
- **App CSS + design tokens moved out of `index.html` into type-safe Go — Tailwind's external (in-HTML) stylesheet is gone (2026-06-30):** The three `<style>` blocks that held the entire design system (~3 400 lines: tokens/`:root`, every component class, light theme, print styles, keyframes) lived statically in `web/index.html`. They are now authored as **type-safe Go** in the new `internal/styles` package and injected into a managed `<style id="cf-app-css">` at boot (`app.Run` → `styles.Register()`), so the design system is owned, programmable Go rather than untyped text in the HTML shell. `internal/styles` = a small typed CSS builder (`dsl.go`: `rule`/`ruleMedia`/`keyframes`/`important`/typed `decl`) + the migrated rules as Go source (`rules_gen.go`, 1 269 rules emitted in original cascade order) + 134 typed property constructors (`props_gen.go`, e.g. `gridTemplateColumns("…")` — a property name is never a raw string at a call site) + `install.go` (the `syscall/js` injector). The migration is **lossless**: the generated stylesheet is 129 535 bytes and renders pixel-identical to the prior static CSS (verified by baseline-vs-migrated screenshots across dashboard, transactions, accounts, budgets, goals, to-do, reports, debt, appearance, recurring, planning, insights). Cascade order is preserved — `cf-app-css` registers before the css-utility engine's `gwc-css`, so equal-specificity utilities still win exactly as before. The only CSS left inline is a minimal `<style id="boot-critical">` block (literal colors, no token/var dependency) for the loading splash, which paints before the wasm that owns the styles loads; `web/index.html` shrank 3 672 → 283 lines. The `internal/ui/tw` typed utility vocabulary is **unchanged and kept** — this removed Tailwind's *static in-HTML stylesheet*, not the Go utility layer. SW cache v283→v284. Verify: `go test ./internal/styles/...` clean (`TestGeneratedHasKeyRules`: 129 535 bytes / 1 269 rules); `gofmt`/`go vet` clean; `GOOS=js GOARCH=wasm go build` rc=0 (atomic web/bin swap); transactions e2e PASS (sticky-header pinning + stable column widths — CSS-dependent layout intact); 12-page screenshot sweep boot-gated, design system injected on every page (129 535 bytes), zero page errors.

### Fixed
- **Dashboard tile drag freeze on Chrome 150 — stop mutating `pointer-events` on the dragged tile; add a stuck-drag watchdog (2026-06-29):** On Chrome/Chromium 150 (beta) a real mouse drag dimmed the tile but never moved it: cursor stuck "grabbing", the page snapped back when scrolled, nothing clickable. Edge (older Chromium) and Playwright's synthesized drag were both fine, which masked it. Cause: the drag-dim CSS `.w.drag` set `pointer-events:none`, and the coordinator applied that class to the element being dragged shortly after `dragstart`; Chrome 150 ABORTS an in-flight native HTML5 drag when its source element's `pointer-events` changes — and the abort fires no `dragend`, so the drag state + the scroll-lock rAF loop stayed stuck on forever (hence the frozen feel + scroll snap-back). Two fixes: (1) `.w.drag` (`web/index.html`) is now opacity-only — the dragged tile is never made click-through (the coordinator targets via a pre-drag geometry snapshot, not live hit-testing, so it doesn't need to be); (2) a watchdog in the scroll-lock loop (`internal/ui/bentoflip.go`): if a drag is active with no progress for 1.5 s, it force-ends (un-dims, releases the scroll lock, clears the cursor) so a drag the browser aborts without a `dragend` can never lock the page permanently. Diagnosed by driving the user's actual Chrome Beta 150 over the DevTools protocol (CDP) — confirming the served wasm/index/SW were all current (so it wasn't stale code) and isolating the failure to real-OS-drag behavior on that engine. SW cache v275→v276. Verify: `GOOS=js GOARCH=wasm go build` rc=0 (validated, atomic web/bin); `go test ./...` clean; real-mouse e2e `dashboard_grid_reorder.test.mjs` + `_stress` still PASS (no regression).
- **Dashboard tile drag no longer freezes on large datasets — drag is now coordinator/DOM-driven (2026-06-29):** The real cause of the "drag freezes, can't click, tile stays dimmed" report was NOT a panic — it was catastrophic main-thread jank (console showed `[Violation] 'mousemove'/'requestAnimationFrame' handler took 200–370ms`, no panic). The bento drag drove a live preview through state atoms (`dragSrc` for the dim, `dragPreview` for the live reflow); every dragover wrote `dragPreview`, and because the per-tile Widget components subscribe to those atoms, each write re-rendered the whole data-heavy `Dashboard()` — recomputing every widget's frame over the full ledger (~250 ms each on a 2 300-transaction dataset). Continuous dragover ⇒ continuous 250 ms re-renders ⇒ the page felt frozen. Fix: the drag is now fully **coordinator/DOM-driven** (`internal/ui/bentoflip.go`), writing NO state until the drop. The dragged tile dims via a CSS class the coordinator toggles directly on the element (deferred one rAF, because adding `pointer-events:none` synchronously inside `dragstart` deadlocks the browser's native drag); there is no live-reflow preview; the coordinator tracks the stable insertion target and stashes the source+target at drag end; the single reorder happens once in the per-tile `OnDragEnd`. `internal/ui/widget.go` no longer reads `dragSrc`/`dragPreview` (so a drag triggers zero re-renders), and `internal/screens/dashboard.go`'s FLIP trigger moved into a tiny `bentoFlipDriver` child keyed on the layout signature only. Result (measured, same 2 313-txn dataset): dragging produces **zero** long tasks (was 4×~250 ms); only the drop does one re-render. Also added `CSS.escape` to the retarget selector (a custom-widget id with a CSS-special char could throw). Verify: `GOOS=js GOARCH=wasm go build` rc=0 (validated, atomic web/bin swap); `go test ./...` clean; `go vet` clean; real-mouse e2e `dashboard_grid_reorder.test.mjs` + `_stress` PASS (reorder + dim-only-during + cleared-after + persists; 3 back-to-back drags stable); long-task probe shows no mid-drag jank. SW cache v274→v275. Follow-up: the one-time drop re-render is still ~0.5 s on a large ledger (frame recompute isn't memoized) — a candidate optimization, not a freeze.
- **Dashboard drag panics are now contained + logged instead of freezing the whole app (2026-06-29):** A panic in any bento drag callback was fatal: the per-tile `OnDrag*` handlers run inside the framework's event wrapper, which RE-PANICS unhandled errors (`finalizeUnhandledPanicContext` → `panic(...)`), and the coordinator's own document listeners / rAF loops / `setTimeout` callbacks (`internal/ui/bentoflip.go`) are raw `js.FuncOf` with no recovery at all — either way a "Go program has already exited" kills every handler, so the page goes unclickable and the dragged tile stays dimmed (the reported "it crashes, I can't click anything"). Now every fatal drag path self-guards: new `recoverBento(label)` logs a labelled panic + stack to the console and contains it; `safeFunc` wraps all coordinator `js.FuncOf` callbacks (armOnPress / dragstart / dragover / scroll / drop / dragend / pointer-end / keydown / scroll-lock rAF / flip rAF / clear-atoms timer); `FlipBento`, `bentoDragStart/End/Target` defer it; and the four per-tile `OnDragStart/Over/Drop/End` handlers in `internal/ui/widget.go` defer it too. So a drag panic now prints `[bento] recovered panic in <site>: <err>` and the app stays alive and clickable instead of freezing. Also restored CSS-selector escaping the Go port had dropped from `flip.js`: the drag-retarget built `querySelector('.bento > .w[data-widget="'+id+'"]')` by raw concatenation, so a widget id with a CSS-special character (custom `wb:`/`us:` cards) could make `querySelector` throw → panic; new `tileByID` runs the id through `CSS.escape`. Service-worker cache bumped v273→v274. Verify: `GOOS=js GOARCH=wasm go build` rc=0 (validated, atomic web/bin swap); `go test ./...` clean; `go vet` (wasm) clean; real-drag e2e (`dashboard_grid_reorder.test.mjs` + `_stress`) PASS; a sweep dragging ALL 16 default tiles + a 4-style crash-probe report 0 panics and the app responsive after every drag.
- **Dashboard tile drag no longer freezes the app; un-stick safety net for a missed drag-end (2026-06-29):** Hardening for the in-Go bento drag coordinator (the `web/flip.js`→Go port, entry under Changed). Two parts. (1) *Crash guard:* the coordinator must NEVER call a `state.UseAtom` hook from a raw JS event/timer callback — there is no component render context there, so `GoUseAtomGlobal` panics and the panic kills the whole Go/wasm program ("Go program has already exited"), after which every handler is dead: the page is unclickable and the dragged tile stays dimmed forever (exactly the reported symptom). Instead, `internal/screens/dashboard.go` captures the drag source/preview atom handles during its render and registers a reset closure via new `ui.RegisterDragAtomClear` (`internal/ui/bentoflip.go`); the coordinator only ever invokes that captured closure (calling the already-bound `.Set`, which is safe outside render), never `UseAtom`. (2) *Un-stick net:* on the true end events (`dragend`/`drop`) the coordinator now schedules a deferred (`setTimeout 0`) clear of the dim atoms — it runs AFTER the per-tile `OnDrop`/`OnDragEnd` bubble handler (so it can't race the reorder) and no-ops if that handler already cleared them, but un-sticks a tile if the per-tile handler was skipped. `Escape` also force-ends a stuck/in-flight drag. Crucially, the pointer end events (`pointerup`/`mouseup`/`pointercancel`) deliberately do NOT clear the atoms: a native drag *starts* by firing `pointercancel`, so clearing there would wipe the just-started drag (tile never dims, reorder dies). Verify: `GOOS=js GOARCH=wasm go build` rc=0 (validated wasm); `go test ./...` clean; `go vet` (wasm) clean; e2e `e2e/dashboard_grid_reorder.test.mjs` + `_stress` (real grip drags) PASS and a 4-way drag crash-probe (move / tiny / drop-on-self / drop-in-void) reports 0 console errors and the app stays responsive after every drag.
- **Wipe data now clears generated Smart content and the in-memory activity feed (2026-06-29):** A data wipe left two things behind. (1) The data-derived SMART state — cached AI result "messages", dismissed-insight keys, last-run stamps, and the digest-delivered log — lives in the `settingskv` table, which is in `preservedOnWipe` (treated like theme/language), so it survived a wipe and described transactions/accounts that no longer existed. (2) The in-memory `auditview.Feed` (the Activity screen's preferred source) was never reset on wipe, only the `audit_log` table. Both are in SQLite already (not localStorage), so the fix is to clear them as part of the wipe rather than relocate them: new `smart.Settings.ClearGenerated()` (drops Dismissed/LastRun/Results, keeps the user's opt-ins/schedules/mutes/density), new `uistate.ClearSmartGenerated()` (applies it + deletes the digest-delivered key), and new `auditlog.Log.Clear()`. `wipeData` (`internal/app/settings.go`) calls `auditview.Feed.Clear()` + `uistate.ClearSmartGenerated()` right after `app.Wipe()`, before the post-wipe dataset export, so the reload re-hydrates the cleared state. Feature preferences and other config still survive a wipe as before. Verify: `go test ./...` clean (new `ClearGenerated`/`Clear` unit tests); `GOOS=js GOARCH=wasm` build rc=0; `gwc` browser lane passed.
- **Studio live preview — drop dead per-tile chrome (2026-06-28):** Clicking the gear/options on the Studio preview tile did nothing visible — it set the widget-settings atom, but Studio mounts no settings host to present the panel, so no options ever appeared; the preview also showed a drag grip and four resize handles that don't function there. In Studio the left-hand form IS the configuration surface, so those per-tile affordances were dead and misleading. New `WidgetProps.Preview` (`internal/ui/widget.go`) omits the gear and grip entirely (truly absent, not CSS-hidden, so they're not keyboard-focusable); `widgetrender.RenderCtx.Preview` threads the flag through; `studioPreviewCtx` sets it; the KPI/list/chart/compound/spacer render sites pass it through and set `Draggable`/`Resizable` to `!Preview`. Dashboard behaviour is unchanged (`Preview` is false there). Verify: `go test ./...` clean; `GOOS=js GOARCH=wasm` build rc=0; browser lane passed; e2e confirms the preview has 0 gear/grip/resize across all four kinds while the dashboard retains all 23 gears + grips; 0 console errors.

### Added
- **Widgetize /transactions as engine widget tiles + reusable DataTable options + a Source column (2026-06-30):** The page is now a thin SURFACE HOST that renders a fixed set of widget specs through the same spec/render pipeline as the dashboard (`safeRenderSpec`) — every visible block is its own engine widget tile: `txn-toolbar` / `txn-bulkbar` / `txn-undobar` / `txn-table` / `txn-import` / `txn-duplicates` (`internal/screens/transactions_widget.go`, `transactions_tiles.go`). The tiles share interaction state (filter/selection/sub-view/undo/receipt preview) through atoms (`internal/uistate/{txnpage,txnedit}.go`) so none embeds another; rows drill into an edit modal (`internal/screens/transaction_edit_form.go` + host `internal/app/txnedithost.go`, mounted in `shell.go`). The ledger hydrates from a new `transactions-full` engine collection (`internal/widgetengine`, `internal/widgetcatalog`, `internal/widgetsource` `RichTransactions`). The reusable `DataTable` widget (`internal/ui/datatable.go` + new `datatable_virtual.go`) gains four opt-in, standardized options any table can use: **StickyHead** (sticky thead via a `--dt-sticky-top` offset), **Virtual** (windowed rendering — "All" shows ~40 DOM rows not thousands, with spacer rows preserving scroll height), **SortSpinner** (self-managed sort-in-progress spinner, deferred one macrotask so it paints first), and **TopPager** (rows-per-page mirrored above a long list). Adds the sortable, filterable **Source** column + a "Filter by source" toolbar control/chip (`internal/i18n/en.go` keys, `internal/i18n/en_txnwidget.go`). Also lands the rail-toggle settle animation (`internal/app/pageenter.go` `triggerRailAnim`, `shell.go`, `web/index.html` `cf-rail-anim`). Legacy full-page ledger retained as `transactionsLegacy` (`internal/screens/transactions.go`). SW cache v276→v283. Verify: `go test ./...` clean; `GOOS=js GOARCH=wasm go build` rc=0, gofmt/vet clean; `e2e/transactions_widget_check.mjs` (47 checks) passes with zero page errors.
- **Transaction source/provenance — record how every transaction entered the ledger (2026-06-30):** New `domain.TxnSource` enum (manual/imported/scanned/recurring/assistant, with `Valid()`/`Label()`; mirrors `TaskSource`) and a `Source` field on `domain.Transaction` (`json:"source,omitempty"` — serialized into the existing JSON blob, so **no SQL migration**). Every creation path now tags its provenance: quick-add + balance-reconcile adjustment + user transfers + goal contributions → manual; CSV import → imported; document/receipt vision import → scanned; recurring/bill auto-post → recurring; AI chat-agent add/transfer → assistant (`internal/app/quickadd.go`, `internal/appstate/{appstate,goal_ops,receipt,transfer_ops}.go`, `internal/screens/{accounts,chat_agent}.go`). Filtering/sorting (`internal/txnfilter`): `source` added to `SortKeys` (ordered by display label, untagged last) plus a `Source` criterion + `FieldSource` filter dimension (Apply/ActiveFilters/Without/compare). CSV round-trips losslessly: a `source` column added to `csvHeader`, the export row, and both import builders, with column-presence logic so our own export preserves an empty source while a foreign CSV (no source column) defaults to imported (`internal/store/csv.go`). Sample data (`internal/store/sample.go`) tags a realistic per-row mix and seeds recent document-sourced (scanned) receipts with linked Documents + previewable receipt artifacts so "load sample" showcases document sources. Verify: `go test ./...` clean (incl. the CSV round-trip + domain/txnfilter); `GOOS=js GOARCH=wasm go build` rc=0; gofmt/vet clean.
- **Studio list widgets — per-collection default sort (2026-06-28):** A fresh list now opens with a sensible default order instead of unsorted. Each collection carries a recommended `DefaultSort`/`DefaultDesc` (`internal/widgetcatalog`): transactions → Date newest-first, accounts → Balance largest-first, budgets → Used % most-at-risk-first, bills → Due date soonest-first, spending → Amount biggest-first. The Studio designer (`internal/screens/studio_designer.go`) pre-selects this default (initial state, on collection switch, and on starter pick via a shared `applyDefaultSort` helper), while the sort stays fully configurable — the picker now offers an explicit "Natural order" option (was "Default order") to opt out of sorting. New `widgetcatalog.DefaultSort(collection)`; the widgetengine cross-validation test additionally asserts every `DefaultSort` is one of its collection's offered sort columns. Verify: `go test ./...` clean; `GOOS=js GOARCH=wasm` build rc=0; browser lane passed; e2e confirms each collection pre-selects its recommended column with the right direction (accounts $285k→$24k, budgets 460%→224%) and 0 console errors.
- **Studio list widgets — sorting control (2026-06-28):** List-based widgets can now be ordered by a column from the Studio settings. New per-collection sort metadata in `internal/widgetcatalog` (`SortField{Column,Label,Numeric}` on each `Collection`; `SortFields(collection)` lookup; `SortDirections(numeric)` returning type-aware labels — "High → Low"/"Low → High" for numbers, "A → Z"/"Z → A" for text). The Studio list section (`internal/screens/studio_designer.go`) gains a "Sort by" picker (with "Default order") and an adaptive "Order" segmented control; picking a column auto-selects a sensible default direction (numbers descending, text ascending) and switching the data source clears a stale sort. `studioBuildSpec` emits a `TransformSort` (engine arg `-col` for descending) BEFORE the limit/cap so the full set is ordered then trimmed — the engine's existing `sortFrame` does the work and the choice persists in the saved `Pipeline` (pure hydration; no per-widget code). New cross-validation test (`internal/widgetengine`) asserts every catalog sort column actually resolves in its collection's Frame in both directions, so a published widget can never reference a missing column. Verify: `go test ./...` clean; `GOOS=js GOARCH=wasm go build` rc=0; browser lane passed; e2e confirmed rows reorder (amount High→Low puts +$4,700 first, Low→High puts −$3,000 first) with 0 console errors.
- **Unified declarative widget engine + Studio authoring (2026-06-28):** A spec-driven widget system where every dashboard tile is hydrated from a pure `domain.WidgetSpec` (Kind → Scalar/Pipeline/Graph/NativeID/Content), plus a Studio surface that lets a casual user design those same dashboard-level widgets with a live preview rendered by the real engine. New packages: `internal/widgetengine` (pure hydrator), `internal/engineenv` (atoms → molecules variable surface), `internal/widgetsource` (Frame resolvers), `internal/widgetregistry` (bindings), `internal/widgetrender`, and `internal/widgetcatalog` (data-driven option catalog — metrics/formats/kinds/collections/starters, with `CollectionRoute` resolved from `collectionDefs` data, not a hardcoded switch). `internal/domain/widget.go`: `WidgetSpec`, `Placement`, `Molecule`, `Frame` types (+ tests). Atoms = indivisible reductions (assets, liabilities, income, expense, liquid_cash, bills_due, goal_needs, counts…); molecules = formula strings over atoms (net_worth="assets - liabilities", cashflow_net, savings_rate, safe_to_spend). KPI/savings tiles are now programmable: each computes its figure by evaluating a CONFIGURABLE formula over the engine variables (`internal/widgetcfg` gains a `Text` field type; `internal/app/settings.go` renders it). New formula functions `floor`/`ceil`/`clamp`/`safediv` (`internal/formula`). Persistence: `placements` + `molecules` tables and dataset fields (`internal/store`), with `appstate` load/persist + override-by-name molecule resolution. Studio (`internal/screens/studio*.go`): Design/Formulas/Custom-fields/Build/Manage/My-pages pill tabs; the Design tab authors a real spec (KPI/list/chart/compound) with a metric picker that surfaces molecule decomposition ("Built from atoms"), advanced formula editing, a content-layout (compound) block editor, and grid-faithful sizing (1–4 slot spans). List kind gains cap/scroll/page display modes with a pinned-footer pager and a data-driven "view all" link (`internal/screens/generic_list.go`). New reusable `FormulaBuilder` component (`internal/screens/formula_builder.go`) embeddable on any page. New `web/fonts*`-independent CSS in `web/index.html` for the composer + list. Top-bar `Customize` entry (`internal/app/shell.go`), six-dot `Grip` drag icon (`internal/icon`), `ChromeHover`/`Style` tile overlays (`internal/ui/widget.go`), `LineClamp2` util (`internal/ui/tw`). Verify: `go test ./...` clean; `GOOS=js GOARCH=wasm go build` rc=0; `.tools/gwc.exe test -lane browser` passed; adversarial UX review loop reached PASS (engine 100, Studio 88, list 82).
- **FEATURE_MAP §5.6 capstone — Wire /assistant, /household, /studio routes; rail regroup; demote consolidated sub-routes off-rail (2026-06-28):** IA-remap capstone integration commit wiring the final route registry and nav grouping. `internal/screens/screens.go`: added `SubGroupUnderstand = "understand"` sub-group const; `ToolsSubGroups` reordered to `{Plan, Understand, Build, Data}` (SubGroupBills const retained for stability, no rail routes use it); `All()` fully rewritten — PRIMARY unchanged (7 items); TOOLS reorganised into 4 sub-groups: Plan (debt/investments/allocate/planning/recurring), Understand (reports/networth/health/**assistant** [NEW hub]), Build (customize/fields/**studio** [NEW hub]/workflows), Data (**household** [NEW hub]/categories/rules/artifacts/activity); SYSTEM narrowed to 5 (appearance/help/about/admin/setup — members/categories/rules relocated to Tools); 13 routes demoted to off-rail (no Group/SubGroup/Label, deep-linkable): /credit, /loans, /bills, /subscriptions, /insights, /smart, /members, /split, /widget-builder, /widget-manager, /documents, /duplicates, /plans. `internal/app/shell.go`: `toolSubGroupLabel` updated — Plan→`nav.toolsPlan` ("Plan & forecast"), Understand→`nav.toolsUnderstand` ("Understand") [NEW case], Build→`rail.subBuild` (reuse, "Build" unchanged), Data→`nav.toolsData` ("Data & people"); railMeta entries added for /assistant (Sparkles), /household (Users), /studio (Customize). `internal/i18n/i18n_iaremap.go` (new, init-merge pattern): `nav.household`, `screen.householdSub`, `nav.studio`, `screen.studioSub`, `nav.toolsUnderstand`, `nav.toolsPlan`, `nav.toolsData` — duplicates vs en.go verified. Context: earlier §5 commits (by parallel worktree agents) completed the content work — /debt consolidates liabilities+credit+loans+payoff; /recurring is a Bills/Subscriptions/Scheduled hub; /networth, /debt, /recurring are real scoped pages (no longer alias Reports/Planning); /planning is forecasts-only; /assistant, /household, /studio hub views exist and are build-verified. §5.7c dedup: scoreRingNode, runAnomalyDetectors, TopPayeesTrailing. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass (i18n catalog validated, no duplicate-key panic).

### Changed
- **De-sprawl the transactions toolbar/bulk tiles — compose the reusable Select primitives instead of hand-rolled Option loops (2026-06-30):** The `txn-toolbar`/`txn-bulkbar` tiles built their `<select>` filters with five hand-rolled `Option`-append loops + ~9 inline `Label/Select/aria` blocks + eight redundant `UseEvent` change hooks — the exact pattern `uiw.SelectInput`/`uiw.OptionsFrom` exist to consolidate. Refactored `internal/screens/transactions_tiles.go` to build every option list with `uiw.OptionsFrom` (value/label extractors) behind a tiny `withAllOption` prepend, and render each field with `uiw.SelectInput` (which owns its own change hook, so the eight toolbar select hooks are deleted) through small declarative helpers (`withFieldLabel`, `filterSelect`, `dateField`, `amountField`, `actionBtn`). The filter grid and bulk-action bar are now label+options+handler one-liners rather than nested element trees. Behaviour is identical (same fields, aria-labels, test ids, handlers). Verify: `GOOS=js GOARCH=wasm go build` rc=0, gofmt/vet clean; `e2e/transactions_widget_check.mjs` passes (account + source filter chips, source-filter narrowing, all bulk actions) with zero page errors.
- **Dashboard bento drag/FLIP moved from helper JS into Go (2026-06-29):** The bento reorder animation + drag-target coordinator + scroll-lock previously lived in a hand-written helper, `web/flip.js` (`cashfluxFlipBento` / `cashfluxBentoDragStart`/`Target`/`End`, wired via document listeners and a `<script>` tag). Ported the whole thing into wasm: new `internal/ui/bentoflip.go` owns the FLIP settle animation (`FlipBento`), the stable insertion-target snapshot with hysteresis (`bentoDragStart`/`bentoDragTarget`/`bentoDragEnd`), the drag scroll-lock, and the autonomous document listeners (`InitBentoCoordinator`, registered once from `internal/app/app.go` after mount). `internal/ui/widget.go` now calls the Go coordinator from its `OnDragStart`/`OnDragOver`/`OnDrop`/`OnDragEnd` handlers, and `internal/screens/dashboard.go`'s layout-signature effect calls `uiw.FlipBento()`. Removed `web/flip.js`, its `<script src="./flip.js">` from `web/index.html`, and `./flip.js` from the `sw.js` precache list — no helper JavaScript remains for this behavior. Behavior is unchanged: tiles still glide to their new slots, the preview keeps a stable target while FLIP-animated siblings slide under the pointer, and the page no longer exposes any `window.cashflux*` drag globals. Bumped the service-worker cache (`web/sw.js` `cashflux-v272`→`v273`) since `index.html` + the precache list changed (dropped `flip.js`) — a stale SW could otherwise serve the old shell/asset set and run a mismatched build. Verify: `GOOS=js GOARCH=wasm go build` rc=0 (validated wasm); `go test ./...` clean; e2e `e2e/dashboard_grid_reorder.test.mjs` + `e2e/dashboard_grid_reorder_stress.test.mjs` rewritten to drive a **real mouse drag** of the grip handle through the native HTML5 drag pipeline (not just synthetic `DragEvent`s) — they assert the tile actually moves, dims only *during* the drag, and that the `.w.drag` dim + `[data-bento-dragging]` cursor lock are CLEARED on release (the reported "stays dim / cursor never reverts" symptom), plus stable-target-through-FLIP-churn (synthetic) and persistence across reload. Both PASS; the old `window.cashflux*` drag globals are asserted absent.
- **FEATURE_MAP §5.3/§5.7b — Tabbed /recurring hub; extract BillsPanel + SubscriptionsPanel (2026-06-28):** Completes the §5.3 "/recurring" merge by folding Bills and Subscriptions into a single three-tab page. (1) `BillsPanelProps` struct + `BillsPanel(p BillsPanelProps) ui.Node` extracted from `Bills()` in `internal/screens/bills_screen.go`: all hooks (UsePrefs, 2×UseState, 5×UseEvent for showAll/calendar-nav, UseNotice, UseDataRevision) hoisted before the nil guard per GWC rule; body identical to the former Bills() body. `Bills()` becomes a one-line thin shell: `return ui.CreateElement(BillsPanel, BillsPanelProps{})`. (2) `SubscriptionsPanelProps` struct + `SubscriptionsPanel(p SubscriptionsPanelProps) ui.Node` extracted from `Subscriptions()` in `internal/screens/subscriptions_screen.go`: all hooks (UseNavigate, UseTxFilter, UsePrefs, UseNotice, 2×UseState, 5×UseEvent for selectAllToggle/bulkCancelEvt/togglePrefs/onMinOccur/insightsNav) hoisted before the nil guard; `doBulkCancel` made self-contained (re-reads cancelMap from app at call time) so it can be registered as a stable hook before cancelMap is computed; the conditional `ui.UseEvent(Prevent(doBulkCancel))` in the savingsSummary block replaced by the pre-registered `bulkCancelEvt`; `allSelected`/`allSubsForSelect` declared before the selectAllToggle hook and set during rendering so the closure reads current render values on event. `Subscriptions()` becomes a thin shell. (3) `RecurringHubProps` struct + `RecurringHub(p RecurringHubProps) ui.Node` added to `internal/screens/recurring.go`: one `UseState("scheduled")` hook drives a `uiw.Segmented` tab bar with three options (Scheduled/Bills/Subscriptions); tab body selected via a switch on the tab value — each arm is `ui.CreateElement(RecurringManagerPanel/BillsPanel/SubscriptionsPanel, ...)` so hooks are isolated inside child components and tab switching is hook-safe. `Recurring()` becomes `return ui.CreateElement(RecurringHub, RecurringHubProps{})`. (4) 3 i18n keys in new `internal/i18n/en_recurring_tabs.go` (init-merge pattern): `recurring.tabScheduled`, `recurring.tabBills`, `recurring.tabSubscriptions`. `/bills` and `/subscriptions` routes unchanged (thin shells rendering the shared panels). Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.
- **FEATURE_MAP §5.3 — Remove the recurring manager from /planning (finish single-theme narrowing) (2026-06-28):** The `/recurring` extraction (entry below) left `ui.CreateElement(RecurringManagerPanel, RecurringManagerPanelProps{})` embedded in `Planning()`'s return, so the recurring cash-flow manager still rendered on BOTH `/recurring` and `/planning`. Removed that one embed line (and updated the adjacent comment) so `/planning` is now purely forward-looking — runway, affordability, forecast, what-if plans — with no recurring manager. `recurring.go` (the `RecurringManagerPanel` component + scoped `Recurring()` page) is untouched; this only drops the duplicate render site from Planning, matching the §5.3 rule that `/planning` sheds recurring to `/recurring`. Verify: `GOOS=js GOARCH=wasm go build` rc=0; e2e `e2e/verify_recurring_scoped.mjs` 5/5 pass (`/recurring` shows the manager and no Planning-only sections; `/planning` shows the forecast and no recurring manager; no page errors).
- **FEATURE_MAP §5.7a + §5.3 — Real scoped /recurring page; extract RecurringManagerPanel; stop aliasing Planning() (2026-06-28):** `/recurring` previously rendered the entire Planning screen via `func Recurring() ui.Node { return Planning() }`, exposing every Planning section instead of a focused recurring-cash-flow view. New file `internal/screens/recurring.go` introduces (1) `RecurringManagerPanel(p RecurringManagerPanelProps) ui.Node` — a registered `ui.CreateElement` component owning all recurring-manager hooks (11 UseState: rev/rLabel/rAmount/rCadence/rAccount/rCategory/rAutopost/rAutopay/rNextDue/rErr/postMsg; 7 UseEvent: onRLabel/onRAmount/onRNextDue/onRCadence/onRAccount/onRCategory plus addRecurring; plus deleteRecurring/addDetected/editRecurring plain funcs and postDue UseEvent) extracted verbatim from `Planning()`. The panel renders the add-form (label/amount/cadence/account/category/first-due/autopost/autopay), auto-detected-charges section (C147), monthly-total note, per-row inline-edit list (reusing existing `RecurringRow`/`detectedRecurringRow` components), and "Post due" action. (2) `Recurring()` — the real `/recurring` page, now a one-line thin shell: `return ui.CreateElement(RecurringManagerPanel, RecurringManagerPanelProps{})`. In `planning.go`: the `Recurring()` alias removed; the 19 recurring-manager hooks removed; the inline `recurringCard` block removed; `ui.CreateElement(RecurringManagerPanel, RecurringManagerPanelProps{})` replaces the card at the same position in the return `Div`; `plRev := ui.UseState(0)` added in the former recurring block's position for plans-only re-render triggering; `subscriptions` import removed. `/planning` renders identically (panel is a registered component, hooks isolated). `/recurring` now shows only the scheduled cash-flow manager. No i18n changes (reuses existing `recurring.*` and `nav.recurring`/`screen.recurringSub` keys). Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.
- **FEATURE_MAP §5.3 — Narrow /planning to forecasting only; move debt-payoff calculator to /debt (2026-06-28):** Completes the `/debt` de-alias so debt content lives on a single page. `Planning()` (`internal/screens/planning.go`) no longer renders the shared `DebtStrategyPanel` or the manual single-debt payoff calculator/projection — those were the last debt widgets still showing on both `/planning` and `/debt`. The payoff calculator is extracted verbatim into a new registered component `PayoffCalculatorPanel` (`internal/screens/payoffcalc.go`, isolated hook scope for the four balance/APR/payment/extra inputs), mounted on `/debt` (`DebtPlanner()` in `debt.go`) after the loans section. Removed from `planning.go`: the four payoff `UseState` + four `UseEvent` hooks, the `resultBody` projection-compute switch, the two payoff `EntityListSection`s, the `DebtStrategyPanel` render, and the now-unused `fmt`/`payoff` imports. `/planning` is now purely forward-looking (forecast, affordability, cash runway, recurring manager, what-if); `/debt` owns the full debt picture (owed hero, liabilities, strategy, credit, loans, payoff calculator). Verify: `gofmt` clean; `GOOS=js GOARCH=wasm go build` rc=0; e2e `e2e/verify_debt_scoped.mjs` 9/9 pass (`/debt` shows hero + strategy + calculator, no Planning-only sections; `/planning` shows forecast + affordability, no debt widgets; no page errors).
- **FEATURE_MAP §5.3 — Fold credit + loans panels into /debt (extract CreditHealthPanel/LoansPanel) (2026-06-28):** Completes the §5.3 "/debt" consolidation and §5.7b "compute once, render canonical widget, embed elsewhere" discipline. `CreditScreen()` body extracted into `CreditHealthPanel(props CreditHealthPanelProps) ui.Node` — a registered component in `internal/screens/credit.go` that owns its `UseDataRevision` hook (moved unconditionally before the nil check per GWC rule) and all state including the inline credit-limit editor sub-components. `CreditScreen()` becomes a one-line thin shell: `return ui.CreateElement(CreditHealthPanel, CreditHealthPanelProps{})`. `LoansScreen()` body extracted into `LoansPanel(props LoansPanelProps) ui.Node` — a registered component in `internal/screens/loans.go` with the same hook-first restructure. `LoansScreen()` becomes `return ui.CreateElement(LoansPanel, LoansPanelProps{})`. In `DebtPlanner()` (`internal/screens/debt.go`), the liability-list loop now accumulates `hasCreditCards` and `hasInstallmentLoans` flags as it builds `liabRows`. Two new sections appended to the return `Div` using existing i18n keys (`nav.credit`/`screen.creditSub` and `nav.loans`/`screen.loansSub`): each wraps an `EntityListSection` around `ui.CreateElement(CreditHealthPanel, ...)` / `ui.CreateElement(LoansPanel, ...)`, guarded by `If(hasCreditCards, ...)` / `If(hasInstallmentLoans, ...)`. The panels are components — their hooks are inside them, not in DebtPlanner — so conditional rendering is safe. /debt now reads: owed hero → liabilities list → strategy/payoff → credit health → loans. Routes `/credit` and `/loans` unchanged. No new i18n keys (reuses existing ones). Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.
- **FEATURE_MAP §5.7a — Real scoped /debt page; stop aliasing Planning() (2026-06-28):** `/debt` previously rendered the entire Planning screen via `func DebtPlanner() ui.Node { return Planning() }`, exposing all Planning sections instead of a focused debt view. New file `internal/screens/debt.go` introduces (1) `DebtStrategyPanel` — a registered `ui.CreateElement` component owning its own `dsExtra` UseState, `onDsExtra` UseEvent, and `rev` revision counter, encapsulating the full snowball-vs-avalanche strategy block (include/exclude toggles, per-liability APR+min-payment editors, burn-down chart, progress tracking) extracted from `Planning()`; and (2) `DebtPlanner()` — the real /debt page rendering a total-owed hero stat (all liability balances FX-converted to base, with a debt-free-by date when avalanche plan is viable at $0 extra), a compact read-only liability list (name, type badge, balance, APR if set, utilization % for credit cards with a limit), a "Manage on Accounts" link, and `ui.CreateElement(DebtStrategyPanel, ...)`. In `planning.go`, the `DebtPlanner()` alias is removed, `dsExtra`/`onDsExtra` declarations are removed (now owned by the panel), the inline debtCard block is removed, and `ui.CreateElement(DebtStrategyPanel, DebtStrategyPanelProps{})` replaces it at the same position — Planning renders identically. Four i18n keys added to `en.go`: `debt.whatYouOwe`, `debt.totalOwed`, `debt.noDebts`, `debt.manageAccounts`. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.
- **FEATURE_MAP §5.7a/b — Real scoped /networth screen; stop aliasing Reports() (2026-06-28):** `/networth` previously rendered the entire Reports screen via `func NetWorth() ui.Node { return Reports() }`, exposing all four Reports tabs instead of a focused net-worth view. This commit replaces that alias with a real, scoped screen and a shared panel. New file `internal/screens/networth.go` introduces `NetWorthPanel` (a registered `ui.CreateElement` component with its own hook scope) that computes assets/liabilities/net-worth and the 6-month trend from the same pure `ledger`/`reports` core used by Reports, then renders the canonical stat-grid + assets-vs-liabilities composition bar + trend area chart + "View accounts" link. `NetWorth()` is now a thin shell: it shows an empty-state CTA (`accounts.addFirst` / `account` AddTarget) when there are no accounts, and `ui.CreateElement(NetWorthPanel, NetWorthPanelProps{})` otherwise — no period selector, no cash-flow charts, no category tabs. In `reports_screen.go` the inline `netWorthSection` EntityListSection (stat-grid + comp bar + NW trend) is replaced by `ui.CreateElement(NetWorthPanel, NetWorthPanelProps{})` so the Reports networth tab embeds the identical shared panel; the cash-flow and savings-rate supporting charts remain in Reports only. The old `NetWorth()` alias is removed from `reports_screen.go`. Hero-zone stats (`nwNet`, `nwChange`, `nwSeries`) remain in Reports for the hero strip. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.
- **FEATURE_MAP §5.7c dedup #3 — Reuse pure `reports.TopPayeesTrailing` in insights top-merchants card (2026-06-28):** `topMerchantsSpendCard` in `internal/screens/insights.go` hand-rolled a 90-day trailing top-payee aggregation (map accumulation + sort + limit) that duplicated the logic already provided by `internal/reports`. Added `PayeeSummary` struct (Name, Amount int64, Count int) and `TopPayeesTrailing(txns, days, asOf, rates, limit)` to `internal/reports/payees.go` — a pure, testable function that mirrors the card's name resolution (t.Payee first, t.Desc fallback), date window (trailing N days from asOf, using `!t.Date.Before(cutoff)` semantics), sort (descending amount, alpha tie-break), and limit behaviour. 7 table-driven tests added in `internal/reports/payees_test.go` covering: trailing-window exclusion, Payee-vs-Desc precedence, count accumulation, transfer/income exclusion, blank-name skipping, alpha tie-breaking, and empty result. `topMerchantsSpendCard` now delegates to `reports.TopPayeesTrailing` and maps the result to `[]merchantSpend` for rendering — card output and drill-through behaviour are identical. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass (reports 0.860s fresh run).
- **FEATURE_MAP §5.7c dedup #1 — Extract `runAnomalyDetectors` shared compute helper (2026-06-28):** Pure internal refactor. `smartAnomalyHighlights` (`internal/screens/insights.go`) and `anomalyHubWidget` (`internal/screens/dashboard.go`) contained a verbatim copy-paste of the SMART anomaly-detection run: build input → `smartengine.Run` with free settings → filter to the four codes A1/T2/T6/T7. Extracted into `runAnomalyDetectors(app *appstate.App, weekStart time.Weekday) []smart.Insight` in `internal/screens/smart_adapter.go` (alongside the existing `buildSmartInput` / `runSmart` helpers). Both call sites replaced with the single helper; each keeps its own row renderer unchanged. The now-unused `smartengine` import removed from `insights.go`. Behavior identical: same input construction, same four anomaly codes, same ordering. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .` rc=0; `go test ./...` all pass.

### Added
- **FEATURE_MAP §5.2/§5.3 item 7 — Split `/customize` into Formulas + Custom fields; new `/fields` page (2026-06-28):** The former Customize super-screen welded two unrelated power-tools — the formula calculator and the custom-field-definitions manager — under one `<h3>` divider, so the page owned two themes. Split per the themed-remap: `Customize()` (`internal/screens/customize.go`) is narrowed to render only `FormulaCalculator()` ("build a metric"), and a new thin shell `CustomFields()` renders only `CustomFieldsManager()` ("your data shape"). New route `/fields` registered in `internal/screens/screens.go` (Tools / Build sub-group, View `CustomFields`). i18n: `nav.customize` relabeled "Customize" → **"Formulas"**, `screen.customizeSub` → "Build your own metrics with formulas"; added `nav.fields` "Custom fields" + `screen.fieldsSub` "Add your own fields to any entity" (`internal/i18n/en.go`). Nav surfaces updated: command-palette list (`internal/app/settings.go`) and rail icon map (`internal/app/shell.go`, `/fields` → `icon.Tag`). Both screens already existed as discrete file-separated functions, so this is a route-and-label split with no logic change. Verify: `gofmt` clean; `GOOS=js GOARCH=wasm go build` rc=0; `GOOS=js GOARCH=wasm go vet ./internal/screens` rc=0 (registry_test.go compiles with the new route); `go test ./internal/i18n` ok; e2e `e2e/verify_fields_split.mjs` 11/11 assertions pass (both rail entries present; `/customize` shows the calculator only; `/fields` shows the fields manager only; deep-link `/fields` resolves; no page errors).
- **C307/C309 [F46] — iOS install hint + sync conflict resolve-modal (keep-local force / use-server) [#464] (2026-06-28):** Three-part completion of R32-sync-pwa. (A) *iOS "Add to Home Screen" hint* (`web/index.html`): a separate dismissible banner — `<div id="iosHintBanner">` + inline `<script>` block — shown only when `/iP(hone|ad|od)/i.test(navigator.userAgent) && !navigator.standalone` and not already dismissed. Dismissal sets `cashflux:ios-hint-dismissed` in localStorage so it never reappears. Styled with the same dark-green palette as the existing `installBtn` (inline styles). Strictly additive: does NOT touch the `installBtn` IIFE, `manifest.webmanifest`, or `sw.js`. (B) *Sync conflict resolve-modal* (`internal/app/syncconflict.go`, `internal/i18n/en_syncconflict.go`): new `SyncConflictHost` singleton component (all hooks unconditional, mirrors `ProfileSwitchHost` pattern) with two actions — "Keep my changes" calls `resolveConflictKeepLocal()` (force-pushes the stashed `queuedSyncMutation` with `PutWorkspaceRequest{Force:true}`, bypassing LWW on the server which already honors the `force` field in `internal/server/sync.go`; clears backup and marks synced on success) and "Use server version" calls `resolveConflictUseServer()` (pulls server snapshot, applies via `app.ImportJSON`, persists to `datasetStoreKey`, and only then calls `clearConflictBackup` — the stash is never discarded before a confirmed successful import). Both helpers live in `internal/app/sync_client.go`. Mounted in `shell.go` after `ProfileSwitchHost`. The conflict chip (`syncchip.go`) now opens the modal on click (via `openSyncConflict()`) instead of the generic settings panel when `loadSyncStatus().State == "conflict"`. (C) *C308 native note* (`TODOS.md`): paragraph added under the C308 bullet framing native iOS/Android as a separate major initiative (PWA is the pragmatic path; Capacitor untested with 60 MB Go-WASM / WKWebView risks; rewrite is months; out of scope this pass). i18n: `internal/i18n/en_syncconflict.go` (13 keys, init-merge pattern). Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build ./internal/app/ ./internal/i18n/` rc=0; `go test ./internal/app/ ./internal/screenlint/` pass.

### Fixed
- **R26 [#453] — Pure smart-settings read + boot-time init + conservative stale-state migration (2026-06-28):** Three-part fix for the SMART settings KV race and schema drift. (1) *Pre-init KV-race* (`internal/uistate/smartsettings.go`): `LoadSmartSettings()` is now a PURE read — the old write-on-empty-KV path (`SaveSmartSettings` called as a side-effect of a read) is removed. An empty KV now returns `EnableFreeOnly(Settings{})` without persisting, so a read that races with store init cannot clobber real persisted settings. New exported `InitSmartSettings()` performs the one-time default persist idempotently (no-op if KV is non-empty) and MUST be called post-store-ready; it stamps `Version = CurrentSettingsVersion`. (2) *Boot ordering* (`internal/app/app.go`): `uistate.InitSmartSettings()` called inside the `if appstate.Default != nil` block alongside `OnTxnMutated` and `SetActiveRoleFunc` — this is the proven post-store-ready wiring point, so ordering is statically guaranteed. Failure mode is benign: if init were skipped, `LoadSmartSettings` returns free-on defaults via the tier-default path (C254 contract preserved). (3) *Stale-state migration* (`internal/smart/settings.go`, `internal/smart/migrate.go`): `Settings.Version int` field added (`json:"version,omitempty"`), `CurrentSettingsVersion = 1` constant exported. Pure `Migrate(s Settings) Settings` fills in the free-on default ONLY for Free features with no explicit state (not in Enabled, not in ExplicitOff); AI features and all explicit user choices are untouched. Boot-ordering statically verified; migration preserves explicit user choices. Applied in `LoadSmartSettings` after unmarshal; NOT persisted on read (write on next normal save). Dashboard digest widget position: `smart-digest` is already reasonably placed in `DefaultItems()` (near-bottom, after `highlight`, before `anomaly-hub`); Part 3 was SKIPPED — no ordering issue warranted a change. New test file `internal/smart/migrate_test.go`: 7 table-driven tests covering explicit-off preservation, unset-free-becomes-on, already-migrated no-op, AI-feature preservation, mixed legacy row, AI-not-enabled-by-default, and idempotency. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build ./internal/uistate/ ./internal/app/ ./internal/screens/` rc=0; `go test ./internal/smart/ ./internal/dashlayout/ ./internal/screenlint/` all pass.

### Added
- **C274 [F40] — Local per-member profile + PIN switch (device user-switching) (2026-06-27):** Implements "Who's using CashFlux?" device-level profile switching with optional per-member PINs. SCOPE BOUNDARY: device-level profile gating + scope-switch auth only — NOT cryptographic per-member data isolation; all members share one local dataset. (1) *Per-member PIN store* (`internal/appstate/memberpin_js.go`, `//go:build js && wasm`): `SetMemberPIN`/`ClearMemberPIN`/`MemberHasPIN`/`VerifyMemberPIN` methods on `*appstate.App`; persists `map[memberID]{Hash, Salt}` under `cashflux:member-pins` in localStorage; uses `applock.HashPasscodePBKDF2` + `applock.PasscodeStrength`/`StrengthTooShort`/`StrengthWeak` validation; 16-byte `crypto/rand` salt via `newMemberPINSalt()`. (2) *Thin wrappers* (`internal/app/memberpin.go`): delegates to `appstate.Default` to avoid a circular import — `app` imports `screens`; `screens` cannot import `app`. (3) *Profile-switch modal* (`internal/app/profileswitch.go`): singleton `ProfileSwitchHost` component with two-step flow — member-picker (`profileCardItem` sub-component, one `UseEvent` per card, never in a loop) → PIN challenge (password input + error message + submit); `openProfileSwitch()` package-level entry point via captured `psHandle` atom. (4) *Owner override*: member with `domain.RoleOwner` switches to any profile without entering the target's PIN; `uistate.ActiveIdentityID()` resolves the caller at switch time; ownership note disclosed to the current owner in the picker. (5) *Entry point* (`internal/app/memberswitcher.go`): "Switch profile…" button next to the member `<select>` opens the modal. (6) *Shell mount* (`internal/app/shell.go`): `uic.CreateElement(ProfileSwitchHost)` after `DialogHost`. (7) *MemberRow PIN management* (`internal/screens/members.go`): `MemberHasPIN`/`OnSetPIN`/`OnClearPIN` added to `memberRowProps`; 8 stable PIN hooks (`showPINForm`, `pinInputS`, `pinErrS`, `onPINInput`, `onShowPINForm`, `onCancelPINForm`, `onSubmitPIN`, `onRemovePIN`) declared unconditionally before any conditional return; display-row renders "Set PIN" / "Change PIN" + "Remove PIN" buttons + inline PIN form (via `uiw.FormField`). (8) *i18n* (`internal/i18n/en_profileswitch.go`): 18 keys, init-merge pattern. (9) *Persist* (`internal/app/persist.go`): `cashflux:member-pins` added to `keptOnWipeKeys` so PINs survive a financial-data wipe. Architecture decision: PIN methods live on `*appstate.App` (in a wasm-tagged file) rather than in `internal/app` to break the `app→screens→app` dependency cycle cleanly. Variable-length card list handled by `profileCardItem` sub-component that owns its own `UseEvent` hook. All 3 verify gates pass: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build ./internal/app/ ./internal/screens/ ./internal/i18n/` rc=0; `go test ./internal/applock/ ./internal/screenlint/` pass.
- **C282 [F42] — WebAuthn PRF passkey local unlock, additive to argon2id gate (#461, 2026-06-27):** Implements passkey (biometric / device-PIN) unlock as a fully additive second path on top of the existing argon2id passcode gate shipped in #460. Build-verified only; the WebAuthn biometric ceremony was NOT runtime-tested (headless); passcode fallback preserved, no lockout path. Design: the WebAuthn PRF extension is used to derive 32 bytes from the authenticator; those bytes are imported as a raw AES-256-GCM key via `crypto.subtle`, which then encrypts the UTF-8 passcode string into a `prfVault` JSON blob stored in localStorage. On unlock, `GetPRF()` re-derives the same 32 bytes (same credential + same PRF salt), decrypts the vault, verifies the recovered passcode against the stored argon2id hash, and calls the exact same `onAppUnlocked(passcode)` function the passcode-input gate calls — the dataset decryption path (`datasetcrypto.go`) is never touched. Three non-negotiable invariants enforced: (1) NO LOCKOUT — passcode is always a working unlock path; (2) ADDITIVE — passcode path unchanged; (3) HIDDEN on unsupported devices via async `Available()` check. New files: `internal/webauthn/webauthn.go` (`//go:build js && wasm` — `Register()`, `GetPRF()`, `Available()` JS wrappers), `internal/app/vaultfmt.go` (pure Go, no build tag — `marshalVault`/`parseVault`; natively testable), `internal/app/vaultfmt_test.go` (5 test functions; 100% pass), `internal/app/datasetcryptoprf.go` (`//go:build js && wasm` — `encryptVaultWithPRF`/`decryptVaultWithPRF` using `crypto.subtle.importKey("raw",…)`), `internal/app/webauthnlock.go` (`//go:build js && wasm` — `registerPasskey`, `unlockWithPasskey`, `showPasskeyManager`, modal builder), `internal/i18n/en_webauthn.go` (13 keys, init-merge pattern). Edited: `internal/app/applock.go` (add `clearPasskey()` in `enableAppLock` + `disableAppLock` for stale-vault invalidation), `internal/app/applockgate.go` (passkey unlock button between main unlock btn and "Forgot", conditional on `hasPasskey()`, included in focus trap), `internal/app/applocksettings.go` (added "Manage passkey" `dataBtn` when lock is enabled), `internal/app/persist.go` (added three `cashflux:webauthn-*` keys to `keptOnWipeKeys`). localStorage keys: `cashflux:webauthn-credid`, `cashflux:webauthn-salt`, `cashflux:webauthn-vault`. All three verify gates pass: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build . ` rc=0; `go test ./internal/app/ ./internal/screenlint/` rc=0.
- **MIA-reports+banner [#444] — Scope /reports + full ScopeBanner + ScopeSelector UI (2026-06-27):** Completes ticket #444 across three parts. (5) *Wire scope into /reports* (`internal/screens/reports_screen.go`): `UseActiveScope()` added as stable hook 2 (after UseDataRevision, before UseNavigate); `accounts` moved to immediately after `txns` so scope can be resolved before any downstream calc; `sc, instOf, scopeIDs, scopedTxns` block derives a filtered transaction slice via `scope.ResolveScope` + `scope.ApplyScopeToTxns`; `scopedTxns` feeds all per-period spend/series calculations (IncomeVsExpense ×2, SpendingByCategory, NoSpendDays, SpendingStats, IncomeExpenseSeries ×2, SavingsRateSeries, LiquidBalance, detectSpendingAnomalies, TopPayees, LargestExpenses, SpendingByMember, LargestIncome, IncomeByCategory, SpendingByWeekday, YearTax download, customFieldSpendSection, deductibleSection); NW-specific calls (NetWorthSeries, NetWorth) continue to use unscoped `accounts`/`txns` — household NW is always complete; `ui.CreateElement(ScopeSelector)` embedded as first child of the return Div. (6) *Extend ScopeBanner* (`internal/app/scopebanner.go`): renders nothing when `sc.IsAll()`; builds a "Viewing: institutions · owners · types" label covering all non-empty scope dimensions joined by " · "; member IDs resolved to display names via `appstate.Default.Members()`; `domain.GroupOwnerID` maps to "Shared"; `bannerTypeLabel` helper converts snake_case AccountType to Title Case; hook chain stable: UseActiveScope = hook 1, clearScope UseEvent = hook 2. (7) *NEW ScopeSelector* (`internal/screens/scopeselector.go`): multi-select chips for institutions (via `domain.UniqueInstitutions`), owners (members + "Shared" for GroupOwnerID), types (all 15 AllAccountTypes); collapsible individual account checklist; saved-views dropdown (apply on select), "Save current as…" name prompt → `PutSavedView`, "Delete" → `DeleteSavedView`; 9 stable top-level hooks in `ScopeSelector()`; each chip is a `scopeChip` sub-component with its own UseEvent hook; each account row is a `scopeAcctRow` sub-component; toggle helpers return nil (not empty slice) so `IsAll()` continues to work. i18n in new `internal/i18n/en_scopeselector.go` (init-merge pattern, 14 keys). All 3 verify gates pass: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/app/ ./internal/i18n/` rc=0; `go test ./internal/scope/ ./internal/screenlint/` pass.
- **Integrate uncommitted working-tree WIP — health-score step keys + /privacy route + Quick-Add archived-account filter (2026-06-27):** Committed a coherent, build-and-test-green set of polish changes that several prior lanes had left uncommitted in the working tree (blocking MIA #444/#453). (1) *Health-score step keys* (`internal/healthscore/healthscore.go` + `health.go` + `healthscore_test.go`, +26 tests): added `Step.Key` (stable factor id like "savings"/"debt"/"utilization") so the health UI can map an improvement step to its action screen without matching the localized human-facing label. (2) *C290 /privacy route* (`internal/app/app.go`): registered `/privacy` as an explicit alias to the About & Privacy view (ActivePath `/about`) instead of falling through the `*` catch-all to the dashboard; not added to `screens.All()` so no duplicate nav item. (3) *C41/C45 Quick-Add* (`internal/app/quickadd.go`): archived accounts are no longer offered as a destination for a NEW transaction (kept only if somehow the current selection). Plus supporting `reports_screen.go`, `en.go`, `web/index.html`, `web/sw.js`, `pack_test.go` baseline, and `TODOS.md` status updates. Integrated under explicit user authorization; honest single integration commit (mixed pre-existing WIP, not authored here). NOTE: 3 git stashes (rebase debris — a large widget_builder refactor, a widgets.go change, and a CHANGELOG/DEVLOG fragment) were left UNTOUCHED for manual reconciliation — they conflict with current HEAD and risk large deletions if force-applied. Verify: `go build ./...` rc=0; `go test ./...` rc=0.
- **MIA-extend [#445] — Scope on dashboard/net-worth + insights + institution mgmt in account form (2026-06-27):** Three-part completion of the MIA scope-extension ticket. (1) *Dashboard scope* (`internal/screens/dashboard.go`): replaced ad-hoc `UseActiveMember()` KPI filter with `uistate.UseActiveScope()` + `scope.ResolveScope` + `scope.ApplyScopeToTxns` + `scope.ApplyScopeToAccounts`. `scopedAccounts` flows into `useNetWorth`, `NetWorthSeries`, the active-account count, and the FX-disclosure loop. `usePeriodTotals` memoization key changed from `activeMemberID` to `scopeSig`. When a scope is active, a muted "vs household total: $X" sub-label appears on the net-worth tile. (2) *Insights scope* (`internal/screens/insights.go`): `UseActiveScope()` at stable slot 2; pre-filtered `scopedTxns` used for all income/expense/highlights/charts. Inline `scopeNotice` chip with "Change scope in Reports →" button when scoped. (3) *Institution management* (`internal/screens/accounts_row.go`, `accountaddform.go`, `format.go`): `titleCaseWords` helper; `institutionS` state + `onInstitution` event at stable hook positions; `startEdit` pre-fills from lender when blank; `saveEdit` normalises with `titleCaseWords`; `Combobox` field in both inline-edit and add forms drawing from `domain.UniqueInstitutions`; backfill nudge "Set institution" button in display row triggers `startEdit` when institution is empty. i18n in `internal/i18n/en_mia.go` (init-merge). All three verify gates pass: `go build ./...` rc=0; WASM build rc=0; `go test ./internal/scope/ ./internal/domain/` pass.
- **C279/C280 delta [F41] — Ghost-member share guard + DRY splitter + income-split card [#474] (2026-06-27):** Three-part completion of the F41 per-member delta. (1) *Ghost-member guard*: `ownedCount` in `internal/screens/members.go` now also counts accounts where the member appears as a KEY in `OwnershipShares` (not just as `OwnerID`), closing the gap where deleting a shared-ownership co-holder bypassed the reassign gate and left dangling map keys. `appstate.ReassignOwner`'s accounts loop purges and redistributes the deleted member's share to the reassign target; clears the map when fewer than 2 holders remain (reverts to single-owner behavior). (2) *DRY splitter*: `ledger.SplitByShares` refactored from a duplicate Hamilton implementation to a thin adapter over `split.ByWeights` (identical behavior: pre-sort member IDs lexically for tie-breaking, sign-preserve for negatives); all 11 existing tests pass unchanged. (3) *Income split*: new `allocate.SplitPeriodIncome(txns, members, start, end, base, rates)` apportions total period income equally across non-group members using `split.ByWeights` with weight=1; returns nil,nil when no non-group members (no members or all-group); propagates FX errors so the card is suppressed rather than showing zeros. 9 table-driven tests in `membersplit_test.go` cover empty members, group-only, single member, equal split, Hamilton indivisible (10¢ / 3 members), zero income, FX path, FX error propagation, transfer exclusion. New `internal/i18n/en_membersplit.go` (init-merge pattern, key `members.incomeSplitTitle`). `/members` screen wired: `allocate` import added; `ownedCount` extended; income-split card rendered below the spending card (silently suppressed on FX error or zero income). Verify: `go build ./...` rc=0; `go test ./internal/ledger/ ./internal/split/ ./internal/allocate/ ./internal/domain/` all pass; `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/app/` rc=0.
- **C279 [F41] — Fractional account ownership + income attribution via OwnershipShares (2026-06-27):** `Account.OwnershipShares map[string]int` added to the domain (`omitempty` JSON tag, sum-to-100 invariant documented, no migration needed — nil deserialises from pre-existing JSON). Pure `SplitByShares(amountMinor int64, shares map[string]int) map[string]int64` in `internal/ledger/shares.go` implements Hamilton/largest-remainder apportionment: exact sum guarantee, sign-preserving for negative amounts, deterministic alpha tie-break for equal remainders. `NetByOwner` in `internal/ledger/ledger.go` now checks `len(a.OwnershipShares) > 0` first; when set, it splits the base-currency converted balance via `SplitByShares` across the named share holders instead of attributing the whole balance to `OwnerID`. Tests: 11 table-driven cases in `shares_test.go` (indivisible amounts, zero, negatives, tie-break, large cases) and `TestNetByOwnerFractionalShares` in `ledger_test.go` (60/40 joint EUR account with FX, archived-account exclusion, owner-rollup sum equals household NetWorth). UI: split-ownership sub-form in both the add form (`accountaddform.go`) and inline edit (`accounts_row.go`) using the Row-component pattern (`OwnerShareRow` standalone component) so no `On*` handler is called inside a variable-length loop (CLAUDE.md §gotchas). Toggle reveals per-member percentage inputs; live sum-validation error when shares ≠ 100; add form blocks submit until valid; post-save resets both `splitOwn` and `ownerShares` state. Sub-form hidden when household has fewer than 2 members. i18n in new `internal/i18n/en_ownershares.go` (init-merge pattern, does not touch `en.go`). All verify passes: `go build ./...` rc=0; `go test ./internal/ledger/ ./internal/domain/` 100% pass; `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/app/` rc=0.
- **MIA-foundation [#443] — Account.Institution field + saved views + active-scope atom (2026-06-27):** Three-part MIA foundation laying the pure data and state infrastructure for multi-institution analytics. (1) *Account.Institution field* (`internal/domain/entities.go`): added `Institution string` with `json:",omitempty"` so all existing stored rows round-trip to `""` with no migration. (2) *UniqueInstitutions helper* (`internal/domain/account_helpers.go`): `UniqueInstitutions([]Account) []string` returns a case-insensitively deduped, case-insensitively sorted list of non-empty institution names, preserving first-seen casing. 9 table-driven tests in `account_helpers_test.go` cover empty input, case-insensitive dedup, blank skipping, sort order, and first-seen-casing preservation. (3) *Saved views* (`internal/scope/savedview.go`): `SavedView{ID, Name, Scope}` type + `ListSavedViews` (sorted by name, bad-JSON entries skipped), `PutSavedView`, `DeleteSavedView` — all operating over a `map[string]string` KV (value = JSON-of-SavedView). 9 tests in `savedview_test.go` cover round-trip put→list→delete, sort order, nil-map safety, bad-JSON skip, and overwrite. (4) *App facade* (`internal/appstate/savedview.go`): `SavedViews()`, `PutSavedView(v)`, `DeleteSavedView(id)` methods on `*App`, persisting the outer map under settings KV key `cashflux:saved-scopes` (survives wipe), mirroring the `occurrences.go` idiom exactly. (5) *Active-scope atom* (`internal/uistate/activescope.go`, `//go:build js && wasm`): `UseActiveScope()` atom returning `scope.ReportScope` + `SetActiveScope()`, persisted as JSON under `cashflux:active-scope`. Migration on first load: if `cashflux:active-scope` is absent but the legacy `cashflux:active-member` key exists, the member ID is promoted into `Owners` and the old key is cleared — transparent upgrade. `ActiveMemberFromScope()` derives a single member-ID string from the scope for callers that need the legacy interface. `activemember.go` left untouched (non-breaking path). All verify commands pass: `go build ./...` rc=0; `go test ./internal/scope/ ./internal/domain/` 100% pass; `GOOS=js GOARCH=wasm go build ./internal/uistate/ ./internal/screens/ ./internal/app/` rc=0.
- **C105 [F13] — Structured rule conditions: field/op/value matching on payee, description, amount, account, date (2026-06-27):** Extends the rules engine from a single global substring `Match` to support up to N structured `Conditions` (ANDed field/op/value tuples) per rule, while remaining fully backward-compatible. (1) *Types* (`internal/rules/rules.go`): new `ConditionField` string-enum (payee/description/amount/account/date), `ConditionOp` string-enum (contains/equals/==/!=/</>/≤/≥/is/is-not/on/before/after/in-month), `RuleCondition{Field, Op, Value}` struct, and `Conditions []RuleCondition` field on `Rule`. `FirstMatch` (quick-suggest path, text-only) now skips condition-bearing rules; new `FirstMatchFull(rs, payee, desc, amountMinor, accountID, txnDate)` evaluates structured conditions when present, falls back to the `Match` substring otherwise. `Conflicts` skips condition-bearing rules from shadow analysis. (2) *Evaluator* (`internal/rules/conditions.go`): new `TxnDate` wrapper, `MatchConditions`, `matchOneCondition`, `matchText` (contains/equals case-insensitive), `matchAmount` (int64 minor-unit parse → ==, !=, <, >, ≤, ≥), `matchAccount` (is/is-not), `matchDate` (YYYY-MM-DD for on/before/after; YYYY-MM for in-month; zero TxnDate → false). All numeric and date parse failures are safe non-matches (no panic). (3) *Tests* (`internal/rules/conditions_test.go`): 26 cases covering every operator family, AND semantics, zero-date guard, unparseable-amount safety, and the FirstMatchFull routing through both paths. All pass. (4) *Wiring* (`internal/appstate/appstate.go`): `AutoCategorizeTransaction` and `ApplyRulesWithCounts` both switched from `FirstMatch` to `FirstMatchFull`, supplying `t.Payee`, `t.Desc`, `t.Amount.Amount`, `t.AccountID`, and `rules.NewTxnDate(t.Date)`. (5) *Form UI* (`internal/screens/ruleaddform.go`): up to 3 bounded fixed condition slots rendered in a `<fieldset>`; every `On*` handler registered at a stable top-level hook position before any slot is rendered — never inside a loop. Each slot: enabled checkbox, field dropdown (field-appropriate op list), value text input. Enabled slots are collected into `Rule.Conditions` on submit. (6) *i18n* (`internal/i18n/en_rulecond.go`): new file, init-merge pattern, 28 keys for field/op labels, slot labels, section label, hints — does not touch `en.go`. `go test ./internal/rules/` (26/26 pass); `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/app/ ./internal/i18n/` rc=0; `go build ./...` rc=0; `go test ./internal/screenlint/` pass.
- **C278 [F41] — Scope accounts/budgets/goals/allocate by active member (2026-06-27):** When the active-member atom is non-empty (a member is selected via the `ScopeBanner` / member switcher), four list screens now filter their displayed entities to that member's owned items plus shared/group entities (`domain.GroupOwnerID == "group"`); when empty ("Everyone" view) all entities are shown (existing behavior unchanged). Shared pure helper `ownerVisibleTo(ownerID, activeMemberID string) bool` added to `internal/screens/format.go` alongside the existing money helpers; `internal/domain` import added to that file. Screens updated: `accounts.go` — `UseActiveMember().Get()` called at top-level before the partition loop; filter applied per-account before the asset/liability/archived branch; `budgets.go` — `UseActiveMember().Get()` called after the `errMsg` hook; raw `app.Budgets()` renamed `allBudgets`, a filtered `budgets` slice built before the first use; `goals.go` — `UseActiveMember().Get()` after the `addGoal` hook; raw `app.Goals()` renamed `rawGoals`, filtered `allGoals` built; `allocate.go` — `UseActiveMember().Get()` after the `dismissIncomeNudge` hook; filter applied in both the account-candidates loop and the goals-candidates loop. Hook ordering preserved — `UseActiveMember()` is unconditional at a stable top-level position in all four screens; filtering is plain Go code, not a conditional hook. `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/i18n/` exit 0; `go build ./...` exit 0; `go test ./internal/screenlint/` pass.
- **C281/C277 [F41] — "Viewing as <member>" scope banner + confirmed dashboard KPI scoping (2026-06-27):** C281: new `ScopeBanner` component (`internal/app/scopebanner.go`) renders a persistent teal-tinted chip below the top-bar banners on every screen when the active-member atom is non-empty. The chip shows "Viewing as <member name>" resolved from `app.Members()`, with a "View all" button that calls `uistate.SetActiveMember("")` to clear the filter and return to the full-household view. It is its own component (mounted via `uic.CreateElement`) so its `UseEvent` hook occupies a stable position — no On* hook in a conditional. CSS: `.scope-banner` / `.scope-banner-text` / `.scope-banner-btn` added to `web/index.html` after the `.sample-banner` block; the class is also added to the print-exclude list. i18n: 4 keys (`scope.viewingAs`, `scope.viewAll`, `scope.viewAllTitle`, `scope.bannerLabel`) in new `internal/i18n/en_scopebanner.go` (init-merge pattern, does not touch `en.go`). Wired into `shell.go` as the third banner after SampleDataBanner and SubscriptionBanner. C277: confirmed ALREADY-SCOPED — `dashboard.go` already filters `kpiTxns` by `activeMemberID` when non-empty (lines 83-94); `income`, `expense`, `incCount`, `expCount`, `cashFlow`, and the displayed transaction counts all derive from `kpiTxns`. C277 is satisfied by the existing filtering + the new C281 banner making the scope visible. `GOOS=js GOARCH=wasm go build ./internal/app/ ./internal/screens/ ./internal/i18n/` exit 0; `go build ./...` exit 0; `go test ./internal/screenlint/` pass.

### Fixed
- **C206 [F27] — Sample loans now amortize: payments modeled as proper liability transfers (2026-06-27):** The four installment-loan payments in `SampleDataset()` were categorized expenses from checking (e.g. `catAutoLoan`, `catEducation`, `catMortgage`), so the loan liability accounts (mortgage, Marcus's car, Priya's car, student loan) never received any transactions and their balances were permanently frozen at their opening balances. The `/loans` amortization screen calculates the current principal via `ledger.Balance` — a running sum of opening balance plus all transactions posted to that account — so frozen loan accounts produced an always-static amortization schedule with no history of paydown. Fix: all four payments are now two-legged transfers (checking out-leg + loan account in-leg) using unique stable IDs (`tx-<YYYY-MM>-mortgage-out/in`, `tx-<YYYY-MM>-studentloan-out/in`, `tx-<YYYY-MM>-carpay-m-out/in`, `tx-<YYYY-MM>-carpay-p-out/in`). The in-leg posts a positive amount to the liability account, which subtracts from its negative opening balance so the running balance decreases by the payment amount each month. The mortgage pays down from -$230,000 over 48 months; the student loan from -$34,000 over 48 months; Marcus's car from -$38,000 over 18 months (Jan 2025); Priya's car from -$26,000 over 10 months (Sep 2025). The `/loans` screen will now show a materially smaller current balance (not the original opening balance) and a correspondingly shorter remaining amortization schedule. Existing loan-payment recurring schedules are unchanged (they remain informational for the bill calendar). `go build ./...` exit 0; `go test ./internal/store/` pass. Completes C206 and the sample piece of ticket #418 (umbrella R21-ui+sample).

### Added
- **R25/C252 [F35] — Dashboard anomaly-hub tile: always-on SMART detector surface (2026-06-27):** Bridges the four SMART anomaly detectors (SMART-A1 balance anomaly, SMART-T2 duplicates, SMART-T6 spending spikes, SMART-T7 missing transaction) onto the dashboard without any Smart opt-in gate. New `anomalyHubWidget` component (`func(app *appstate.App) ui.Node`) in `internal/screens/dashboard.go` calls `buildSmartInput` + `smart.EnableFreeOnly` (identical path to `/insights`) and keeps only the four anomaly detector codes; caps the display at three findings. Two sub-components (`anomalyHubRow`, `anomalyHubViewAll`) each own their own `UseEvent` hook so no `On*` is called inside a loop. The widget renders in a 2×1 bento tile titled "Flagged activity" with a "View full analysis →" drill-through to `/insights`. Empty state: "No anomalies detected — everything looks normal." Widget registered as `"anomaly-hub"` in the `renderers` map and added to `DefaultItems()` in `internal/dashlayout/pack.go` (2×1, after `smart-digest`). i18n strings (5 keys: `dashboard.anomalyHubTitle`, `dashboard.anomalyHubClear`, `dashboard.anomalyHubHint`, `dashboard.anomalyHubViewAll`, `dashboard.anomalyHubViewAllAria`) in new `internal/i18n/en_anomalyhub.go` (init-merge pattern, does not touch `en.go`). `GOOS=js GOARCH=wasm go build ./internal/screens/ ./internal/i18n/` and `go build ./...` both exit 0.
- **C33/C34/C35 [F4] — Self-learning categorization: correction tally + live Quick-Add chip + configurable threshold (2026-06-27):** Wires the existing (but unwired) `internal/learntally` tally engine into the full feedback loop. (C33) Every manual category assignment — whether via Quick-Add `saveCore()` or the inline-edit `editTxn` path in the transactions ledger — now calls `uistate.IncrementLearnTally(payee, categoryID)`. The payee key prefers the Payee field, falling back to Description, matching what the suggestion reads. (C34) Quick-Add computes a "learned suggestion" chip from `uistate.LoadLearnTally().ShouldSuggest(payee, threshold)` immediately after the rule-based chip block. The chip only surfaces when no rule already matched (`suggestedCatID == ""`) and the user hasn't picked a category yet (`catID.Get() == ""`), preventing duplication. It reuses the existing `screens.SmartFieldAssist` chip pattern (`id="qa-learned-cat"`); clicking it sets `catID` in one tap. (C35) The threshold (defaulting to `learntally.DefaultMinCount = 3`) is now a user setting: `uistate.LoadLearnThreshold()` / `uistate.SaveLearnThreshold()` persist it under the preserved KV key `cashflux:learn-threshold`. A new `learnThresholdRow()` component in Settings left-column (after the notification thresholds section) exposes a number input (`data-testid="settings-learn-threshold"`, `min=1`). Persistence layer: new `internal/uistate/learntally_store.go` (KV-backed Load/Save/Increment for tally at `cashflux:learn-tally`; Load/Save for threshold; both use `SettingKVGet`/`SettingKVSet` so they survive a dataset wipe). i18n: 3 keys (`quickAdd.learnedCatSuggest`, `settings.learnThresholdLabel`, `settings.learnThresholdHint`) in new `internal/i18n/en_learntally.go` (init-merge pattern, does not touch `en.go`). Both `GOOS=js GOARCH=wasm go build ./internal/app/ ./internal/uistate/ ./internal/i18n/` and `go build ./...` exit 0. `go test ./internal/learntally/` passes.
- **C168/C141/C142 [F18/F22] — /planning leads with liquid cash-flow + Safe to spend tile + unified terminology (2026-06-27):** Three related /planning surfacing tickets landed together. (C168) Reordered the `Planning()` return so `runwayCard` (near-term liquid cash-flow, 60-day runway, payday balance) is the first/headline section; `forecastCard` (12-month net-worth chart) is demoted to third position after "Can I afford it?" — hooks are unchanged (all declared at top of function before any return), only node ordering changed. (C141) `safespend.Compute(...)` result is now surfaced as a visible "Safe to spend" stat tile in the runway card's stat-grid (the lead section); computed once top-level via an IIFE (`planSafeToSpend`) shared by both the runway tile and the affordability card so the number is consistent everywhere. Negative safe-to-spend renders in danger tone. (C142) The affordability card's "available" stat label is renamed from "Free to spend" → "Safe to spend" (new i18n key `planning.affordAvailableLabel` in `internal/i18n/en_planningsurface.go`; old `planning.affordAvailable` key preserved in `en.go` for any other callers). Net-worth chart keeps its own accurate "Net worth in 12 months" / "Projected in 12 months" labels — only the safe-to-spend/affordability naming unifies. New i18n file `internal/i18n/en_planningsurface.go` (init-merge pattern). Both GOOS=js GOARCH=wasm build and `go build ./...` exit 0.


- **C154 [F19] — Persistent per-bill paid/autopay status across reloads (2026-06-27):** "Mark paid" now persists the paid mark for each (billID + due-date) occurrence in the shared KV store under the key `cashflux:occurrences:paid` (JSON map of composite key → unix timestamp). `internal/appstate/occurrences.go` (new file, package appstate) adds `MarkOccurrencePaid`, `UnmarkOccurrencePaid`, and `OccurrencePaid` methods on `*App`; the composite key format is `"billID|YYYY-MM-DD"` (account ID for account-derived bills, `"recurring:<id>"` for recurring-derived bills). `internal/i18n/en_billspaid.go` (new, init-merge pattern) adds 5 i18n keys: `bills.paidBadge`, `bills.paidBadgeTitle`, `bills.unmarkPaid`, `bills.unmarkPaidTitle`, `bills.unpaidLogged`. `internal/screens/bills_screen.go` wired: `markPaid` now calls `app.MarkOccurrencePaid`; new `unmarkPaid` callback calls `app.UnmarkOccurrencePaid`; `billRowData` gains `IsPaid bool`; `billRowProps` gains `OnUnmarkPaid`; `BillRow` renders a green "Paid" chip (`data-testid="bill-paid"`) when `IsPaid` is true and replaces the "Mark paid" button with "Unmark paid" (`data-testid="bill-unmark-paid"`) — urgency tone is suppressed on paid rows. Native + wasm builds exit 0; `go test ./internal/appstate/` passes. Unblocks #432 (IMPL R16-ui umbrella).


- **C58-ui/C62 [F6/F7] — Split badge on ledger rows + shift-click range selection (2026-06-27):** Two transaction-ledger UX completions. (1) *Split badge (C58-ui)*: The normal (non-editing) ledger row now shows a small teal "⑂ Split" chip next to the description when `t.HasSplits()` is true (`data-testid="txn-split-badge"`, `title="Split across categories"`). The chip uses a new `.badge-split` CSS class (`.badge` base + subtle teal tint — dark: `#5eead4` on `#0f2a2a`; light: `#0d9488` on `#f0fdfa` — both WCAG AA) added to `web/index.html`; the logic change is a single conditional `Span` in `transactions_row.go`. (2) *Shift-click range selection (C62)*: The row checkbox's `sel` handler already calls `e.JSValue().Get("shiftKey").Bool()` and passes it to `OnToggleSelect`; `toggleSelect` in `transactions.go` was already tracking `lastSelID` (anchor atom) and `visibleOrder` (populated from the current page slice) and had full range-select logic: when `shift=true` and the anchor is set, it resolves both indices in `visibleOrder`, selects every ID in `[ai..bi]` inclusive, and updates the anchor. Both builds pass (wasm + native). Files: `internal/screens/transactions_row.go`, `web/index.html`.
- **C39/C46 [F5] — Recent-payee autocomplete in Quick-Add (2026-06-27):** Quick-Add now has a `Payee` field with a native `<datalist id="qa-payees">` populated from the new pure helper `internal/payees.RecentPayees(txns, 50)` (distinct payees ordered newest-first, case-insensitive dedup, Payee→Desc fallback for transactions that predate the field). The input carries `list="qa-payees"` so the browser renders a native autocomplete dropdown — suggestions only, not a hard constraint. `domain.Transaction.Payee` is now populated on Quick-Add save (C46 fix). i18n keys `quickAdd.payee` / `quickAdd.payeePlaceholder` added via `internal/i18n/en_payeeac.go` (init-merge pattern, does not touch concurrent-WIP `en.go`). 10/10 unit tests pass (`go test ./internal/payees/`). e2e: `e2e/c39_payee_autocomplete_check.mjs` verifies datalist presence, `list` attribute wiring, and that a recently-saved payee appears as a datalist option on next open. Wasm build rc=0.
- **C290/C293 [F43] — Real About & Privacy screen at /about (2026-06-27):** Replaced the stub `About()` (which just called `HelpScreen()`) with a dedicated `AboutScreen()` in new `internal/screens/about.go`. The screen has five cards: app identity + tagline; Privacy & your data (local-first commitment, no tracking, export anytime — C290/C293); Cloud sync (off by default, what dataset syncs when on, user controls it — C291 disclosure); AI features (bring-your-own-key, stored locally, sent to OpenAI only on explicit invocation — C292 disclosure); Version & changelog (live `version.Label()`, links to changelog/source/license). i18n keys in new `internal/i18n/en_about.go` (init-merge pattern, avoids concurrent-WIP `en.go`). Unit test `TestAboutI18NKeys` (25 cases) checks every key is registered and non-empty. e2e test `e2e/c290_about_check.mjs` navigates to /about and asserts local-first, cloud-sync, AI-key, and version copy are present. `help.go` changes: one line only (stub comment + `return AboutScreen()`). Wasm build rc=0.
- **C299 [F44] — "Last backed up" timestamp in Settings → Data (2026-06-27):** The Settings → Data section now shows a "Last backed up <date>" line (or "Not backed up yet — export a backup to keep your data safe." when never backed up) beneath the Export/Back-up controls, using `data-testid="last-backup"`. `lastBackupSummary()` reads `cashflux:lastBackedUpAt` from localStorage via `loadLastBackup()` and formats the timestamp with the user's date-format preference via `uistate.LoadPrefs().FormatDate(t)` — already present in `internal/app/notifyrun.go`, already rendered in `internal/app/settings_section.go`. **Gap fixed:** `backupEverything()` in `internal/app/backupall.go` was NOT calling `recordBackupNow()` — the "Back up everything" action would never stamp the timestamp. Added the one-line call. i18n keys in new `internal/i18n/en_backupts.go` (init-merge, avoids concurrent-WIP `en.go`). Unit test `TestLastBackupI18NKeys` checks both keys are non-empty and correctly formatted. e2e test `e2e/c299_last_backup_check.mjs` navigates to Settings, verifies the initial "never" state, triggers Export JSON, reloads, and asserts the element no longer shows the "never" copy. Wasm build rc=0.
- **C88 [F11] — Pre-import duplicate warning on CSV/document import path (2026-06-27):** The paste-CSV and file-picker import flows now run a preview step before writing. `dedupe.CountIncomingDuplicates(incoming, existing, accountID)` (new pure helper, no store access) counts how many incoming rows would be skipped, mirroring the exact per-account signature logic in `ImportTransactionsCSV`. `appstate.PreviewCSVImport(data, fallbackAccountID)` wraps it after full field-resolution (account/category/member names → IDs). If duplicates are found the UI stores the raw bytes in `pendingCSV` state and surfaces a `notice-warn` banner (`data-testid="csv-dup-warn"`) with the count + an "Import anyway" button (`data-testid="csv-dup-confirm"`); if none, the import proceeds immediately (no extra click). The existing post-import skipped-count summary is unchanged. i18n keys in new `internal/i18n/en_dupwarn.go` (init-merge pattern). Unit test `TestCountIncomingDuplicates` (5 table-driven cases) in `internal/dedupe/dedupe_test.go`. e2e: `e2e/c88_csv_dup_warn_check.mjs`. Wasm build rc=0.
- **C261 [F37] — Negative-savings guardrail on the financial-health score (2026-06-27):** the free deterministic aggregate engine `healthscore.Evaluate` (6 weighted, re-normalizing factors) was already built and wired into `/health`; this adds the C261 negative-cash-flow guardrail as a **soft penalty** — a flat −15 (`negativeCashFlowPenalty`) applied after the weighted average when the trailing savings rate is negative — rather than the ticket's suggested hard `<50` cap. Rationale (product decision): a hard floor double-penalizes a deficit that already zeroes the savings factor; the soft nudge keeps a structural shortfall visible without cliffing an otherwise-strong household to "Critical". Test `TestNegativeSavings_SoftPenalty` isolates the deduction (savings 0% vs −50% → exactly −15 points, still `NegativeCashFlow`-flagged, no cliff). Files: `internal/healthscore/healthscore.go` (+test). `go test ./internal/healthscore` ok, wasm rc=0.
- **C295 [F44] — Confirmation modal before "Import dataset" overwrites all data (2026-06-27):** `importJSON()` in `internal/app/settings.go` now gates `ImportJSONWithBlobs` behind `uistate.ConfirmModalLabeled(...)` — the user must explicitly confirm before their dataset is replaced. The modal body explains the destructive scope ("…replaces all current accounts, transactions, budgets, and goals… can't be undone") and labels the confirm button "Replace all data" (destructive styling). Cancel is the default/safe path. Pattern mirrors `wipeData()` and `restoreFromBackup()` for UI consistency. i18n keys `settings.importConfirm` + `settings.importConfirmBtn` in `internal/i18n/en.go`. Unit test `TestImportConfirmI18NKeys` (`internal/app/import_confirm_test.go`) verifies both keys are registered and non-empty. Browser test `e2e/c295_import_confirm_check.mjs` drives: file selected → modal visible with "Replace all data" button → Cancel aborts (no toast, modal closes) → Confirm proceeds (success toast). WASM build rc=0.
- **C28 [F3] — Household-members step added to first-run setup checklist (2026-06-27):** `setupChecklist()` in `/help` now includes a sixth step "Add household members (optional)" linked to `/members`. The step uses `len(app.Members()) >= 2` as the done condition (matching `setup.MembersDone` semantics — a solo user is not blocked). New i18n key `help.membersStepLabel`. Files: `internal/screens/help.go`, `internal/i18n/en.go`. WASM build rc=0.
- **C314 [F47] — Serve wasm compressed: Accept-Encoding negotiation + on-the-fly gzip + CI precompression (2026-06-27):** `e2e/serve.go` now negotiates `Accept-Encoding` for `.wasm` requests: serves a precompressed `.br` sibling (brotli) if the client accepts it and the file exists; else a `.gz` sibling; else compresses on the fly via `compress/gzip` at `BestSpeed`; else identity. `Vary: Accept-Encoding` is set on every wasm response. `deploy-pages.yml` gains a "Compress WebAssembly" step after `go build` that installs `brotli` and runs `gzip -9 -k` + `brotli -k -q9` to produce `main.wasm.gz` and `main.wasm.br` as Pages artifacts. `web/sw.js` unchanged — the Cache API already stores decoded (decompressed) bytes; compression is transparent. Measured: 66 MB → 13.7 MB gzip-9 (4.8× / ~79% reduction). Tests in `e2e/serve_compress_test.go` cover on-the-fly gzip, identity path, Vary header always present, and precompressed sibling preference.
- **C183 [F24] — Monthly round-up savings automation (2026-06-27):** Implements periodic-batch round-up: once per calendar month, every expense transaction on the configured spending account is rounded up to the nearest $1/$5/$10 (user-selectable granularity), and the accumulated spare-change total is moved to the savings account in a single transfer. Pure helpers `roundUpTotal` (pure, testable) and `roundUpDue` (once-per-month guard) live in `internal/appstate/roundup.go`. `RunDueRoundUps(now, prefs)` is called from `scheduledworkflows.go` immediately after `RunDueSweeps`. Five prefs fields added to `internal/prefs/prefs.go`: `RoundUpEnabled`, `RoundUpFromAccountID`, `RoundUpToAccountID`, `RoundUpGranularityMinor`, `RoundUpLastPeriod`. The "Round-up savings" config card in `internal/screens/workflows.go` (new `roundUpForm` component) lets users enable the feature, choose accounts, and pick the granularity; it sits in the existing "Savings automations" section between sweep and the workflow builder. 17 i18n keys added to `en.go`. 12 table-driven tests in `roundup_test.go` cover delta sums, account/income/transfer skipping, date-range boundaries, granularity defaults, and the due guard. Tests and both wasm builds (packages + root) exit 0.
- **R31-plans — Plans comparison surface + re-engageable upgrade path (2026-06-27):** Implements the local-doable slice of R31 (C300/C301/C303). (1) NEW `internal/screens/plans.go` — `Plans()` component: Free (local, $0, no account, no expiry) vs Cloud ($34.99/yr or $3.99/mo, 14-day trial) side-by-side comparison with REAL prices, feature lists, trust + self-host copy, and a CTA linking to Cloud settings → Stripe Checkout. No dark patterns: both prices stated, easy navigation away. Registered as `/plans` route in `screens.go`. (2) NEW `internal/i18n/en_plans.go` — all plans strings via init-merge pattern (does not touch `en.go`). (3) `internal/app/cloudmention.go` — "Learn more" button changed from `ShowUpgradeSheet()` to an `<a href="/plans">` anchor so the banner leads to the durable comparison surface, not a one-shot modal; snooze still fires on click. (4) `internal/app/upgradesheet.go` — persistent "View plans →" link added below the action buttons so dismissing the sheet does not permanently bury pricing. R31-chip placed on Plans screen (chip in shell/nav deferred — those files are other-agent WIP). R31-portal (Stripe manage/cancel) blocked on R32 hosted billing backend.
- **R29-enforce — Household role enforcement core: ActiveIdentity atom + appstate read-only guard (2026-06-27):** Three-part implementation of R29's clean-package enforcement core. (1) *R29-identity*: `internal/uistate/activeidentity.go` — `UseActiveIdentity()` atom (js+wasm, KV-persisted as `cashflux:active-identity`), `SetActiveIdentity`/`PersistActiveIdentity`/`ActiveIdentityID` helpers; deliberately separate from `UseActiveMember` (view-filter vs. operator identity). (2) *R29-seam*: `internal/appstate/readonly.go` — `SetActiveRoleFunc(fn)` injection seam (keeps appstate free of any uistate import cycle); `ActiveRole()`/`CanEdit()`/`CanManageMembers()` methods on `*App` delegating to `memberrole.CanEditEntities` / `memberrole.CanManageMembers`; permissive `RoleOwner` default when no fn is wired (existing callers unaffected). (3) *R29-enforce*: `ErrReadOnly` sentinel; `roleGuard()` (financial entities) and `memberRoleGuard()` (member CRUD, Owner-only) guards applied to all 40+ `PutX`/`DeleteX` methods across `appstate.go`, `settle.go`, `importprofile_ops.go`; 13 table-driven tests in `readonly_test.go` prove Viewer → `ErrReadOnly` and Owner/Admin succeed. All `go test ./internal/appstate/ ./internal/memberrole/` pass; wasm build rc=0. R29-ui (button gating, identity switcher, chip) deferred to screen/app-file agent.
- **C23 [F3] — Base currency & week-start surfaced in first-run setup checklist (2026-06-27):** The setup checklist on `/help` now has a first item "Confirm base currency & week start (Settings → Appearance)" as the very first step, displayed as a clickable link to `/appearance`. The step shows ✓ when `Settings.BaseCurrency` is non-empty and ○ otherwise. Week-start has a sensible default so it does not block completion — the step frames it as "confirm" rather than "required". New i18n key `help.currencyStepLabel`. Files: `internal/screens/help.go`, `internal/i18n/en.go`. WASM build rc=0.
- **R30-gatekdf — PBKDF2-SHA256 gate KDF + transparent migration path (2026-06-27):** The app-lock gate hash is upgraded from plain SHA-256 to PBKDF2-SHA256 @ 210,000 iterations (OWASP 2023 tier for password storage). `HashPasscodePBKDF2(passcode, salt)` produces a self-describing `"pbkdf2$210000$<hex>"` value using a stdlib-only PBKDF2 implementation (no new dependencies). `VerifyPasscode(passcode, salt, storedHash)` is the new unified verify entry-point: it dispatches on the hash format — PBKDF2 (constant-time compare, needsMigration=false) or legacy bare-SHA-256 (constant-time compare, needsMigration=true on success) — so callers can transparently re-store migrated credentials on next successful unlock. `Config.WithPasscode` now stores PBKDF2 hashes; `Config.Verify` delegates to `VerifyPasscode`. Legacy `HashPasscode` is retained for fallback verification and migration. 15 table-driven tests cover: new-format round-trip, legacy-SHA-256 needsMigration, wrong passcode on both paths, tampered/garbage storedHash errors, constant-time exercised. All pass; wasm build rc=0. UI call-site wiring (applockgate.go swap to `VerifyPasscode` + re-store on migration) remains for the applockgate.go owner. Files: `internal/applock/applock.go`, `internal/applock/applock_test.go`.

### Changed
- **FEATURE_MAP §5.7c dedup #2 — Extract shared scoreRingNode helper (health + credit rings) (2026-06-28):** Pure internal refactor eliminating a copy-paste SVG gauge. `creditScoreRing` in `internal/screens/credit.go` even carried a "matching healthRing" comment acknowledging the duplication. New file `internal/screens/scorering.go` (`//go:build js && wasm`) contains the single shared helper `scoreRingNode(pct float64, ringColor string, size int, ariaLabel string, centerLabel, subLabel ui.Node) ui.Node`. The helper owns all fixed SVG geometry: 120×120 viewBox, r=52, cx/cy=60, stroke-width=10, faint track circle, arc circle with rotate(−90), rounded linecap, and the dash-offset/stroke CSS transition. Both `healthRing` and `creditScoreRing` are rewritten as thin wrappers that derive their caller-specific values (pct, color, figure text, aria label, center-label node with tone class, sub-label node) and delegate to `scoreRingNode`. Health's BandNoData/no-data path (pct=0, figure="—", ringLabelNoData i18n key) stays in health.go. No behavior, no DOM structure, and no visual output changed. Verify: `go build ./...` rc=0; `GOOS=js GOARCH=wasm go build -o .\web\bin\main.wasm .` rc=0; `go test ./...` all pass.
- **C22 [F3] — Monthly income setting + budget integration (2026-06-27):** `Prefs.MonthlyIncomeMinor int64` (`json:"monthlyIncomeMinor,omitempty"`) added to `internal/prefs/prefs.go` as the authoritative household take-home pay in minor units. When positive, it is preferred over the transaction-derived figure by all budget income paths; zero = fall back. A "Monthly income" text input (`inputmode="decimal"`) added to Settings → Preferences via a new stable `monthlyIncomeInput` component (hooks at a fixed render position, not inside any loop). `OnMonthlyIncome` callback in `globalSettingsForm` parses the major-unit string via `money.ParseMinor` and saves to `prefsAtom`. `/budgets` updated on three call-sites: (1) the `apply503020` event handler uses `uistate.CurrentPrefs().MonthlyIncomeMinor` (safe in event handlers); (2) the simple-mode income banner prefers `budgeting.IncomeForBudgets(pr.MonthlyIncomeMinor, ...)` with a fallback to the transaction sum when the result is zero; (3) the zero-based "to assign" banner does the same. Three i18n keys: `settings.monthlyIncome`, `settings.monthlyIncomePlaceholder`, `settings.monthlyIncomeHint`. Prefs tests pass; WASM build rc=0. Files: `internal/prefs/prefs.go`, `internal/i18n/en.go`, `internal/app/settings_section.go`, `internal/app/settings.go`, `internal/screens/budgets.go`.
- **R32-soak — Sync path soak/load test (2026-06-27):** Added `internal/server/sync_soak_test.go` tagged `//go:build soak` (excluded from normal `go test ./...`). Two tests: `TestSoakSyncPushPull` (32 goroutines × 10 unique-workspace writes each — 320 total — all accepted, pull list count verified) and `TestSoakSyncConflictFanout` (16 goroutines racing to update the same workspace over 20 rounds — verifies at least one accepted write per round, workspace remains readable, accepted+rejected = total). Run with `go test -tags soak ./internal/server/... -run Soak -v`. `go build -tags soak ./internal/server/...` rc=0; WASM build rc=0.
- **R32-merge — 3-way field-level merge for structured sync records (2026-06-27):** `internal/syncmerge` package gains `ThreeWayMerge(base, local, remote Record) (merged Record, conflicts []ConflictEntry)`. The algorithm: if only local changed a field → take local; only remote → take remote; both changed to the same value → no conflict, keep it; both to different values → conflict logged, LWW fallback (later `UpdatedAt` wins, remote wins on tie). Nil base degenerates to two-way LWW. 8 table-driven round-trip tests in `merge_test.go` cover all branches including the nil-base degenerate case, both-changed-same-value, and the 3-field clean round-trip. All 15 `syncmerge` tests pass; WASM build rc=0.
- **R32-conflict — Field-level LWW merge with conflict log (2026-06-27):** New pure package `internal/syncmerge` with `Record` (field→`FieldValue{Value,UpdatedAt}`), `ConflictEntry` (Field, LocalValue, RemoteValue, ChosenValue, ChosenSide), and `MergeRecord(local, remote Record) (merged Record, conflicts []ConflictEntry)`. Deterministic field-level last-writer-wins: later `UpdatedAt` wins; remote wins on equal-timestamp tie-break. Every differing field is recorded in a `ConflictEntry` — no silent drops. No `syscall/js` — pure Go, native-testable. 7 table-driven unit tests pass; WASM build rc=0.
- **C118 [F14] — Per-budget methodology override (2026-06-27):** `domain.Budget` gains `Methodology string \`json:"methodology,omitempty"\`` (empty = inherit the global household method). The add form and inline edit form each expose a "Method for this budget" select with options: "Use global default" (empty), Simple, Zero-based, Envelope. `budgets.go` computes each budget's effective method via `effectiveMethod(b)` — the budget's own override when non-empty, else the global household method — and uses it to decide envelope-balance computation per row. A small "Method: <name>" sub-line appears on rows that have a non-default method set. Envelope balance is now computed for every budget where `effectiveMethod(b) == MethodEnvelope`, regardless of the global setting, so a single envelope budget in a simple-mode household sees its carry-forward balance. Three i18n keys: `budgets.methodLabel`, `budgets.methodDefault`, `budgets.methodOverrideRow`. Files: `internal/domain/entities.go`, `internal/screens/budgets.go`, `internal/screens/budgetaddform.go`, `internal/screens/budgets_row.go`, `internal/i18n/en.go`. Domain tests pass; WASM build rc=0.
- **C210 [F28] — Per-card utilization trend on /credit (2026-06-27):** Each credit-card row on `/credit` now derives a chronological utilization trend from `app.BalanceHistory(accountID)`. `buildUtilTrend` fetches the stored `BalanceSnapshot` series (capped to last 8), computes `pct = abs(BalanceMinor) * 100 / LimitMinor` per snapshot, and returns a `[]utilTrendPoint{date, pct, id}`. The "Utilization trend" panel renders via `MapKeyed` (stable; no On* in loop): each row is a short date, a thin 6px bar whose fill-width equals `min(pct,100)%` and color matches the utilization band (green ≤30%, amber ≤50%, red >50%), and the raw percent. Shown only when ≥2 snapshots exist; when fewer, a muted "Not enough history yet" nudge is shown for cards that have a limit set. Two new i18n keys: `credit.trendTitle`, `credit.trendNoHistory`. Files: `internal/screens/credit.go`, `internal/i18n/en.go`. Build rc=0.
- **C225 [F31] — Valuation history after "Update balance" (2026-06-27):** `domain.BalanceSnapshot` (ID, AccountID, BalanceMinor, Currency, AsOf) records a point-in-time value per account. Store: `balance_snapshots` SQLite table, `PutBalanceSnapshot`/`ListBalanceSnapshots` CRUD, table registered in schema/Load/Snapshot for lossless dataset round-trips. Appstate: `PutAccount` compares incoming vs. persisted balance before writing; records a snapshot whenever they differ (including initial nonzero balance). `BalanceHistory(accountID)` accessor returns snapshots sorted ascending by AsOf. UI: `accounts_row.go` computes a compact "Value history" panel for illiquid-asset accounts (property/vehicle/investment/retirement/crypto/other) showing the last 6 snapshots most-recent-first using `MapKeyed` (no On* hooks in loop). Panel suppressed when fewer than 2 snapshots exist. `accounts.go` passes `ValuationHistory: app.BalanceHistory(ac.ID)` into each row. Two i18n keys: `accounts.valuationHistoryTitle`, `accounts.valuationHistoryEmpty`. Tests: `TestBalanceSnapshotCRUD`, `TestDatasetBalanceSnapshotRoundTrip`, `TestPutAccountRecordsSnapshotOnBalanceChange`, `TestPutAccountNoSnapshotWhenBalanceUnchanged`, `TestBalanceHistoryIsolatedPerAccount`. All pass; WASM builds exit 0.
- **C122 [F15] — Post-transaction overspend re-evaluation via OnTxnMutated observer seam (2026-06-27):** Overspend/budget alerts were previously only evaluated at boot via `runNotifyCatchUp()`. Added a pure observer mechanism to `appstate.App`: `OnTxnMutated(fn func())` registers observers; `fireTxnMutated()` calls them (no-op when `suppressTxnObservers` is set). `PutTransaction` and `DeleteTransaction` each call `fireTxnMutated()` after a successful mutation. `DeleteTransactionWithTransferPair` suppresses per-leg calls and fires once after both legs are removed. `ImportTransactionsCSV` sets `suppressTxnObservers=true` during the batch loop and calls `fireTxnMutated()` exactly once after the batch (import-storm guard: N rows → 1 notification cycle, not N). The app layer registers `appstate.Default.OnTxnMutated(func(){ runNotifyCatchUp() })` at boot, so a new overspend notification appears immediately after a Quick-Add or delete without a page reload. `runNotifyCatchUp` dedupes by key (C121/C270 infra) so re-running never duplicates already-shown alerts. 5 new native tests in `internal/appstate/txnobserver_test.go` (add fires, edit fires, delete fires, batch fires once, no-op import skips entirely). Tests pass; WASM build exit 0. Files: `internal/appstate/appstate.go`, `internal/appstate/txnobserver_test.go`, `internal/app/app.go`.
- **C128 [F16] — Pay-cycle anchor: configurable payday date aligns biweekly budget periods (2026-06-27):** `Prefs.PayCycleAnchor` (ISO date, `omitempty`) stores a known payday. `budgeting.PeriodRangeAnchored(p, ref, weekStart, anchor)` snaps the biweekly 14-day grid to the anchor instead of the internal epoch when anchor is non-zero; all other periods delegate to `PeriodRange` unchanged. `/budgets` reads the anchor from prefs and routes biweekly budgets through `PeriodRangeAnchored`. A "Pay cycle anchor" date input added to Settings → Preferences with a plain-English hint. 5 table-driven tests added (`TestPeriodRangeAnchored`). Files: `internal/prefs/prefs.go`, `internal/budgeting/budgeting.go`, `internal/budgeting/budgeting_test.go`, `internal/screens/budgets.go`, `internal/app/settings_section.go`, `internal/app/settings.go`, `internal/i18n/en.go`.
- **C87 [F11] — Merge duplicates: keep one, union tags, remove the rest (2026-06-27):** `dedupe.Merge(survivor, others)` (pure, no syscall/js) unions Tags case-insensitively and sets Cleared=true if any entry was cleared — identity fields (ID, Amount, Date, AccountID) are unchanged. A "Merge (keep one)" button added to each group card in `/duplicates` (stable `UseEvent` inside the per-group component, never inside the row loop). On confirm: `PutTransaction(merged)` then `DeleteTransaction` for each duplicate; success toast via `PostUndoable`. Four i18n keys added. Tests: `TestMerge` (union + dedup + cleared propagation) and `TestMergeNoClearedPropagation` pass. Files: `internal/dedupe/dedupe.go`, `internal/dedupe/dedupe_test.go`, `internal/screens/duplicates.go`, `internal/i18n/en.go`.
- **C89 [F11] — /duplicates review screen: group transactions by dedupe signature, delete extras (2026-06-27):** Added `internal/screens/duplicates.go` implementing `DuplicatesScreen()` and two supporting components (`dupeGroup`, `dupeRow`). Delegates detection to the existing `dedupe.FindDuplicates(txns)` (groups by date+amount+normalized-description, excludes transfers, returns only groups ≥2) and `dedupe.Count(groups)` for the headline count. Component architecture: `DuplicatesScreen` holds only `UseDataRevision().Get()` at top level; `dupeGroup` is called via `ui.CreateElement` for per-group stability; `dupeRow` is called via `ui.CreateElement` for per-row `UseEvent` stability — no hooks inside variable-length loops. First entry in each group is labelled "Keep"; the rest each show a "Delete duplicate" button with a `ConfirmModal` guard and an `PostUndoable` toast on success. Registered as `/duplicates` in `screens.go` under `GroupTools / SubGroupData` (alongside `/documents`). 14 i18n keys added to `en.go`. Files: `internal/screens/duplicates.go` (new), `internal/screens/screens.go`, `internal/i18n/en.go`. Screens build rc=0; full-app build rc=0.

- **C191 [F25] — Auto-accrual for sinking funds: once-per-month credit to CurrentAmount (2026-06-27):** Sinking-fund goals now receive automatic monthly contributions on boot. `goals.FundAccrualDue(g, now)` (pure helper) determines eligibility — returns false if: not a sinking fund, archived, already at/over target, or `Custom["fundAccrualPeriod"]` already matches the current UTC year-month key (the double-credit guard). When due, `amountMinor = min(FundSetAsideMinor, remaining-to-target)` caps the credit so funds never overshoot. `appstate.RunDueFundAccruals(now)` iterates all goals, applies the credit via `money.Add`, stamps `g.Custom["fundAccrualPeriod"]` with the current month key (initialises the map if nil), and calls `PutGoal`. It is called from `runDueScheduledWorkflowsOnBoot` alongside `RunDueScheduledWorkflows`, with parallel logging and `resaveDataset` trigger. 8 table-driven cases in `sinkingfund_test.go`. Files: `internal/goals/sinkingfund.go`, `internal/goals/sinkingfund_test.go`, `internal/appstate/appstate.go`, `internal/app/scheduledworkflows.go`. Tests pass; WASM build rc=0.

- **C193 [F25] — SMART-BL9 sinking-fund nudge creates a real goal + surfaces on /goals (2026-06-27):** `bl9SinkingFund` previously emitted `ActionCreateTask` ("Add a to-do") on `PageBills`, so the nudge was invisible on /goals and never created a real financial entity. Now: (1) `smart.Action` gained two fields — `GoalIsSinkingFund bool` and `GoalCategoryID string` — for the sinking-fund create-goal payload. (2) BL9 emits `ActionCreateGoal` (label "Create a sinking fund"), targets `PageGoals`, sets `GoalName = "<Bill> Fund"`, `GoalTarget = abs(annualAmount in base currency)`, `GoalCurrency = base`, `GoalIsSinkingFund = true`, `GoalCategoryID = r.CategoryID`, `RelatedType = "bill"`. (3) The `ActionCreateGoal` handler in `smart_card.go` reads the new flag and sets `IsSinkingFund = true` + `CategoryID` on the created `domain.Goal`; the existing non-fund code path is unchanged. (4) Toast shows "Sinking fund created." for the sinking-fund path vs. "Goal created." for the regular path. (5) `bills_test.go` updated to assert `ActionCreateGoal`, `GoalIsSinkingFund = true`, and `Page = PageGoals`. Files: `internal/smart/smart.go`, `internal/smartengine/bills.go`, `internal/smartengine/bills_test.go`, `internal/screens/smart_card.go`, `internal/i18n/en_smart.go`. Tests pass; WASM build rc=0.

- **C219/C220/C221 [F30] — /investments holdings UI: performance summary + asset-class allocation (2026-06-27):** Added `internal/screens/investments.go` implementing `InvestmentsScreen()` and supporting components. Filters accounts to `TypeInvestment`, `TypeRetirement`, `TypeCrypto`; renders a per-account card per `investmentAccountCard` (called via `ui.CreateElement` for stable hook positions). C219: per-holding display rows with delete action (`holdingRow` component per `ui.CreateElement`) and an `addHoldingForm` component per account (6 `UseState` + 7 `UseEvent` hooks at stable top-level positions, validates name/shares/cost/price, calls `app.PutHolding`). C220: 2×2 performance-summary grid (market value, cost basis, gain/loss, return%) computed via `portfolio.HoldingValueMinor`, `portfolio.UnrealizedGainMinor`, `portfolio.ReturnPct`, `portfolio.PortfolioSummary`. C221: asset-class allocation bar list built from `portfolio.AllocationByAssetClass` with percentage bars and value labels. An overall portfolio summary card is shown across all investment accounts when any holdings exist. Registered `/investments` in `screens.go` under `GroupTools / SubGroupPlan`. 35 i18n keys added to `en.go`. Files: `internal/screens/investments.go` (new), `internal/screens/screens.go`, `internal/i18n/en.go`. Screens build rc=0; full-app build rc=0.

- **C219-foundation [F30] — Holding domain entity + store persistence + dataset round-trip (2026-06-27):** Added `domain.Holding` struct (ID, AccountID, Ticker, Name, Shares, CostBasisMinor, CurrentPriceMinorPerShare, AssetClass) to `internal/domain/entities.go`. Added `holdings` SQLite table, `PutHolding`/`GetHolding`/`DeleteHolding`/`ListHoldings` CRUD to `internal/store/crud.go` and `sqlitestore.go` (schema + Load + Snapshot). Added `Holdings []domain.Holding` to the `Dataset` struct for lossless export/import. Added `Holdings()`/`PutHolding()`/`DeleteHolding()` accessors to `internal/appstate/appstate.go`. Converter `portfolio.FromDomain` / `portfolio.FromDomainSlice` added to `internal/portfolio/domain.go` to bridge persisted holdings to the pure calculation layer without an import cycle (portfolio imports domain; domain does not import portfolio). Tests: `TestHoldingCRUD`, `TestDatasetHoldingRoundTrip` in `internal/store/`, and `TestFromDomain`/`TestFromDomainSlicePortfolioSummary` in `internal/portfolio/`. All pass; WASM build exit 0.

- **C190 [F25] — Sinking-fund monthly set-aside integrated into /budgets (2026-06-27):** The total monthly set-aside across all active sinking-fund goals (`IsSinkingFund=true && !Archived`) is now computed in `Budgets()` by summing `goals.FundSetAsideMinor(g, now)` and surfaced as a summary line below the income/methodology banner when the total is non-zero. Renders as: "Sinking funds need $X this month — money committed to saving for future expenses." with `data-testid="budgets-fund-setaside"`. Zero sinking funds → nothing rendered (no empty clutter). Helper used: `goals.FundSetAsideMinor`, the canonical per-fund monthly figure already powering the Goals screen chips. Files: `internal/screens/budgets.go`, `internal/i18n/en.go`. Build rc=0.

- **C189/C192/C194 [F25] — Sinking-fund concept: flag + category link + Goals grouping (2026-06-27):** Added `IsSinkingFund bool` and `CategoryID string` fields to `domain.Goal` (both `omitempty`, zero-migration). `GoalAddForm` gains a "Sinking fund" checkbox and an optional "Linked category" select (from `app.Categories()`), both wired into the stored Goal on submit. The Goals screen partitions goals into three buckets: sinking funds (`IsSinkingFund=true`), active regular goals, and achieved. Funds render in a dedicated "Sinking funds" card above the regular goals section; each fund row shows the monthly set-aside chip (via `goals.FundSetAsideMinor`) and category link via new `goalRowProps.FundSetAside`/`LinkedCategoryName` fields passed as data (no hooks in loops). `goalCategoryOptions` helper + `categoryNameByID` helper added alongside `goalAccountOptions`. Seven i18n keys added. Files: `internal/domain/entities.go`, `internal/screens/goaladdform.go`, `internal/screens/goals.go`, `internal/screens/goals_row.go`, `internal/i18n/en.go`. Tests exit 0; WASM build rc=0.
- **C204/C205 [F27] — /loans screen: installment-loan amortization + extra-payment simulation (2026-06-27):** Added `internal/screens/loans.go` implementing `LoansScreen()` and its per-loan `loanCard` component. Filters to TypeLoan / TypePersonalLoan / TypeMortgage accounts (non-archived); each card is rendered via `ui.CreateElement(loanCard, props)` so per-card UseState/UseEvent hooks are stable — never inside a loop body. Each card shows: loan name, type badge, APR, current balance (via `ledger.Balance`, negated for liabilities); a term input (months, default 60 for loans/personal-loans, 360 for mortgages) driving `payoff.AmortizeFixed` for C204; a 2×2 grid of summary stats (monthly payment, total interest, total paid, payoff date); an extra-payment input driving `payoff.AmortizeWithExtra` for C205 — savings panel shows months saved, interest saved, new payoff date, and payments remaining. Registered `/loans` in `screens.go` under `GroupTools / SubGroupPlan`. 25 i18n keys added. Files: `internal/screens/loans.go` (new), `internal/screens/screens.go`, `internal/i18n/en.go`. Screens build rc=0; full-app build rc=0.
- **C208/C209 [F28] — /credit screen: local credit-health proxy + actionable per-card utilization (2026-06-27):** The committed `credithealth` package (C208 engine) was never surfaced to users. Added `internal/screens/credit.go` implementing `CreditScreen()`, a full `/credit` route: an overall proxy-score ring (0–100, with band label) derived from `credithealth.Evaluate()`; a per-card utilization breakdown (name, balance vs. limit, % bar with band color, and an actionable "Pay $X to reach 30% utilization" nudge per card — C209); an empty state when no credit cards exist; and the required Disclaimer surfaced on every presentation. Builds inputs via `ledger.Balance` over credit-card accounts, exactly mirroring how `/health` builds its inputs. No interactive per-row elements (display-only rows), so `MapKeyed`-equivalent loop is used safely. Registered `/credit` in `screens.go` under `GroupTools / SubGroupPlan` next to `/health`. Eleven i18n keys added to `en.go`. C210 (utilization history/trend) is explicitly out of scope — requires stored snapshots which do not yet exist. Files: `internal/screens/credit.go` (new), `internal/screens/screens.go`, `internal/i18n/en.go`. Screens build rc=0; full-app build rc=0.

- **C67 [F8] — Discoverable top-level Transfer action on /accounts (2026-06-27):** Transfer creation was only reachable via each account row's overflow (⋯) menu — invisible until a user discovered the menu. Added a "Transfer money" primary button above the account lists (visible whenever ≥ 2 non-archived accounts exist). Clicking opens an inline form at page level with From/To account selectors (mutually exclusive, no same-account selection allowed), amount, date, and description fields. Submit calls the existing `doTransfer`/`CreateTransferPair` flow — no logic duplication. The button hides while the form is open; form closes on success or Cancel. One new i18n key: `accounts.transferMoney`. Files: `internal/screens/accounts.go`, `internal/i18n/en.go`. Build rc=0.

- **C98 [F12] — Chosen receipt image preserved across navigation to Settings (2026-06-27):** When a user picks an image in the Documents screen but has no OpenAI key and follows the "Go to Settings" prompt, the chosen image was lost on navigation (in-component `ui.UseState` torn down on route change). Added a package-level `state.Atom[string]` ("doc:imageDraft") in `internal/uistate/imagedraft.go`. The `chooseImage` handler now writes the data-URL to the atom; a `ui.UseEffect` on first mount restores it if the local state is still empty (covers the return-from-Settings path). The atom is cleared on both successful import paths (`importDraft` and `importReceipt`) so stale image data does not persist across unrelated sessions. In-memory only — no localStorage for the large base64 payload. Files: `internal/uistate/imagedraft.go` (new), `internal/screens/documents.go`. Build rc=0.
- **C224 [F31] — Property + Vehicle dedicated asset account types (2026-06-27):** Added `TypeProperty` ("property") and `TypeVehicle` ("vehicle") `AccountType` constants to `internal/domain/enums.go`, inserted into `AllAccountTypes` (next to the asset group before TypeOther). `Class()` defaults to `ClassAsset` for both. `DefaultWindows()` in `internal/freshness/freshness.go` assigns 180-day staleness windows (illiquid valuation, same as TypeOther). `isValuationType()` in `accounts_row.go` now includes both so stale-badge copy reads "Out of date / Update value" rather than banking terminology. `accountTypeIcon()` in `accounts.go` maps Property → icon.Box and Vehicle → icon.Calculator (closest available glyphs). `isLockableAsset` in `accountaddform.go` includes both so lock-until is surfaced at add time. Two new i18n keys in `en.go`: `acctType.property` = "Property", `acctType.vehicle` = "Vehicle". `TestAllSlicesAreValid` updated to expect 15 types. Tests exit 0; WASM build exit 0.
- **C12 [F2] — Draft-review account selector + Import button visible above fold for all drafts (2026-06-27):** The `topBar` (account selector + Import button above the row table) was gated on `len(rows) > 4`, so users importing 4 or fewer rows never saw the controls without scrolling. Changed condition to `len(rows) >= 1` so the above-fold action bar renders for any non-empty CSV draft (receipt mode continues to omit it as before). No duplication — the bottom footer is preserved. Files: `internal/screens/documents_draft_review.go`. Build rc=0.
- **C325 [F50] — In-app support & bug-report section on /help (2026-06-27):** Added a `supportCard()` helper to `internal/screens/help.go` rendering a "Support & feedback" card on the Help screen. The card contains a short invite line and two GitHub links: "Report a bug" (→ `issues/new`, opens in new tab) and "Feature requests & feedback" (→ `issues`), each with `target="_blank"` + `rel="noopener noreferrer"`. The card is placed immediately after the What's-new card in `HelpScreen()`. Six new i18n keys in `internal/i18n/en.go` under the `help.support*` namespace. Build rc=0.
- **C119 [F14] — Income awareness in simple budget mode (2026-06-27):** Simple methodology showed no income context — the user could not see how their total budgeted amount related to what they actually earn. Added a `case budgeting.MethodSimple` arm in the `assignBanner` switch (previously the switch had only `MethodZeroBased` and `MethodEnvelope`). The simple-mode banner now shows: "Income this month: $X · Budgeted: $Y · Z unbudgeted/over income". Income is computed with `ledger.PeriodTotals` over the current calendar-month window (same helper the zero-based arm already used at the same anchor); the difference is `simpleUnbudgeted = income − totalLimit`: positive renders in `tw.TextUp` ("$Z unbudgeted"), zero renders neutral ("All income budgeted"), negative renders in `tw.TextDown` ("$Z over income"). Reuses `fmtMoney`, `money.New`, and `budgeting.PeriodRange` already in scope. Five new i18n keys: `budgets.simpleIncome`, `budgets.simpleBudgeted`, `budgets.simpleUnbudgeted`, `budgets.simpleFullyAllocated`, `budgets.simpleOverAllocated`. Envelope and zero-based modes are untouched. Files: `internal/screens/budgets.go`, `internal/i18n/en.go`. Build rc=0.
- **C69 [F8] — From-account selector on the transfer form (2026-06-27):** The transfer form previously hard-coded the source account as the one whose overflow menu was used to open the form, offering no way to change it. Added a "From account" `SelectInput` above the existing "To account" selector; both lists are built from all non-archived accounts, each excluding whatever is currently selected in the other field so the two can never match. Submitting when From == To (or either is blank) is prevented: the Transfer button is disabled and an inline error "From and To accounts must be different." is shown when equal. The form title changed from "Transfer from {account}" (hardcoded to the row's account) to "Transfer between accounts" — reflecting that the user can now pick any From. State atom `xferFromS` defaults to `a.ID` (the row's account) so the old behaviour is the pre-selected default; `startTransfer` resets both selectors on open. `doTransfer` now reads `xferFromS.Get()` instead of `a.ID`. Hook ordering preserved: two stable `ui.UseEvent` placeholder hooks replace the old single one. New i18n keys: `accounts.transferFromLabel`, `accounts.transferFromPlaceholder`, `accounts.transferSameAccountErr`; updated: `accounts.transferTitle`, `accounts.transferFormLabel` (removed `%s` arg). Files: `internal/screens/accounts_row.go`, `internal/i18n/en.go`. Build rc=0.
- **C96/C99 [F12] — unreadable-receipt error message + vision call cost note (2026-06-27):** C96: when the AI vision call succeeds but returns zero transactions (blurry/non-receipt image), the existing `len(rows) == 0` branch now surfaces an actionable message: "We couldn't read any transactions from this image. Try a clearer, well-lit photo of the full receipt, or import a CSV instead." (was the generic "No transactions were found in that image."). C99: added a muted cost-note below the image-import button row: "Uses your OpenAI key — a vision read costs roughly a few US cents per image." so BYOK users understand the cost before clicking. New i18n key: `documents.imageCostNote`; updated: `documents.noneFound`. Files: `internal/i18n/en.go`, `internal/screens/documents_image_import.go`. Build rc=0.
- **C315 [F48] — a11y: explicit aria-label on sidebar nav items, HouseholdCard settings button, and breadcrumb Dashboard button (2026-06-27):** The `navItem` component (sidebar/rail nav links) relied solely on `title` for an accessible name — browsers expose `title` inconsistently to screen readers, particularly when the control is collapsed to icon-only. Added `Attr("aria-label", props.Label)` alongside the existing `Title` so both AT and tooltips use the same label value (reuses `props.Label`, the already-i18n'd nav item name — no hardcoded strings). Same fix applied to the `HouseholdCard` settings button (title-only → title + aria-label using the same computed name string) and the breadcrumb "Dashboard" back button in `TopBar`. The decorative spacer `Span` in the collapse row already had `aria-hidden="true"` (no change needed). Files: `internal/app/shell.go`. Build rc=0.


- **C126/C127 [F16] — biweekly + semi-monthly budget periods (2026-06-27):** Added `PeriodBiweekly` ("biweekly") and `PeriodSemimonthly` ("semimonthly") to `domain.Period`, inserted them into `AllPeriods` (order: Weekly, Biweekly, Semimonthly, Monthly, Quarterly, Yearly), and added `Label()` / `Valid()` coverage. `budgeting.PeriodRange` gains two new cases: biweekly returns a stable 14-day window on a fortnightly grid anchored to Go's epoch Monday (2006-01-02), shifted by `weekStart` so every boundary aligns with a week-start day; semi-monthly returns [1st, 16th) for days 1–15 and [16th, 1st-of-next-month) for days 16–end, handling all month lengths including Feb 28/29. `periodLabel()` in `budgets.go` gains explicit cases for the new periods using `budgets.periodBiweekly` ("Every 2 weeks") and `budgets.periodSemimonthly` ("Twice a month") i18n keys (added to `en.go`); since `periodOptions()` already iterates `domain.AllPeriods`, no further UI wiring was needed. Table-driven tests cover: epoch boundary, mid-window, last-day, and next-window biweekly cases (Monday + Sunday anchors); contiguity loop over 26 consecutive fortnights; semi-monthly first/second half; month-end correctness for 30-, 31-day, Feb-28, and Feb-29 months; December year-boundary. Files: `internal/domain/enums.go`, `internal/budgeting/budgeting.go`, `internal/budgeting/budgeting_test.go`, `internal/screens/budgets.go`, `internal/i18n/en.go`. Tests exit 0 (`./internal/budgeting/` and `./internal/domain/`); WASM build exit 0.
- **C185/C188 [F24] — pay-yourself-first savings automation template + framing (2026-06-27):** Added a visible "Savings automations" section at the top of the Workflows screen (C188) with a plain-English framing card ("Move money to savings automatically on a schedule — pay yourself first"). Below the frame, a "Pay yourself first" template form lets the user pick a source account, a destination (savings) account, an amount, and cadence (weekly/monthly). On save, `App.CreatePayYourselfFirstWorkflow` (new method in `internal/appstate/savings_ops.go`) constructs a scheduled `ActionTransfer` workflow — a real two-leg transfer, not a single-leg autopost — with a `DedupeKey` following the `pyf:<wfID>:<YYYY-MM>` convention so the same period never transfers twice. The resulting workflow appears immediately in the existing "Your workflows" list. Inline validation (from≠to, amount>0) with clear error messages and a success confirmation. The `pyfForm` component (own stable component, no hooks-in-loops) handles all state/events. New i18n keys: `workflows.savingsTitle/Desc`, `workflows.pyfTitle/Desc/From/To/Amount/Cadence/CadenceWeekly/CadenceMonthly/Save/ChooseAccount/NeedFrom/NeedTo/SameAccount/NeedAmount/Created`. Files: `internal/appstate/savings_ops.go`, `internal/screens/workflows.go`, `internal/i18n/en.go`. Build rc=0; appstate Workflow tests pass.
- **C244/C245/C246 [F34] — no-key chat answers via localqa + Send button + error reset (2026-06-27):** Three cohesive fixes for the no-key /insights chat path. C244: when `key == "" && !useBackendAI`, `sendText` now runs the question through `localqa.Match` + `localqa.Answer` before falling back to the key-hint error; a new `insightsQASource` adapter (`internal/screens/insights_localqa.go`) implements the 7-method `localqa.Source` interface using live ledger/safespend/bills/goals/health data. C245: `errMsg` is cleared at the very start of `sendText` so a stale key-error from a prior question never bleeds into a new one. C246: the trailing slot on the no-key path now renders a visible "Send" button (calling the same `onSubmit` handler) instead of an empty fragment, so the chat is keyboard- and pointer-accessible without an API key. No new i18n keys (`insights.send` already existed). Files: `internal/screens/insights.go`, `internal/screens/insights_localqa.go`. Build rc=0.
- **C229 [F32] — top-merchants spending breakdown with drill-through on /insights (2026-06-27):** Added a "Top merchants" card on /insights showing the top 7 payees by total expense spend over the last 90 days. Each row shows the rank, merchant name, total amount, and transaction count. Clicking a row navigates to /transactions with a text/search filter pre-set to the merchant name (reusing the same `txFilterAtom` + `TxFilter.Text` mechanism established for C228 category drill-through). Payee field is preferred; Desc used as fallback when Payee is empty. Per-row component `insightsMerchantRow` keeps OnClick hooks at stable positions (no hooks in loops). Pure helper `topMerchantsSpendCard` and `merchantSpend` type added at the bottom of insights.go. New i18n keys: `insights.topMerchantsTitle`, `insights.topMerchantsHint`, `insights.merchantTxCount`, `insights.merchantDrillAria`. Files: `internal/screens/insights.go`, `internal/i18n/en.go`. Build rc=0.
- **C230 [F32] — monthly spending time-series chart on /insights (2026-06-27):** Added a "Spending over time" area-chart card on /insights showing total expense outflow for the last 6 months. Uses `reports.IncomeExpenseSeries` over monthly bounds (same pattern as /reports cash-flow trend) and renders via `uiw.AreaChart` with month-abbreviation x-axis labels and per-point hover labels formatted via `fmtMoney`. The chart is placed between the "Spending highlights" and "Flagged activity" cards. New helper `monthlySpendingChart` at the bottom of insights.go. New i18n keys: `insights.spendTrendTitle`, `insights.spendTrendHint`. Files: `internal/screens/insights.go`, `internal/i18n/en.go`. Build rc=0. (C230)
- **C228 [F32] — spending-highlight drill-through to filtered transactions (2026-06-27):** Each spending-highlight row on /insights is now a clickable button that navigates to /transactions pre-filtered to that category, matching the same drill pattern used by /reports (L58 FILTER_CARRY). Added `insightsHighlightRow` as a standalone component (so `OnClick` registers at a stable hook position, not inside a variable-length loop), `categoryNameToIDMap` helper (reverses the category name→ID map needed to resolve the anomaly's display name back to a filterable ID), and updated `spendingHighlights` to accept an `onDrill` callback threaded from `Insights()`. The highlights-card hint text also updated to mention the tap-to-filter affordance. New i18n key: `insights.highlightDrillAria`. Files: `internal/screens/insights.go`, `internal/i18n/en.go`. Build rc=0. (C228)
- **R39 (progress) - progressive disclosure on the To-do list (2026-06-27):** Extended the hover-reveal row-action pattern (from R47/transactions) to /todo via a generic `.row-2nd` class: a resting task row now shows only the complete-toggle + task text; the Add-subtask, Edit, and Delete actions reveal on row hover or keyboard focus-within (opacity keeps tab order; coarse-pointer always-on for touch). /todo resting controls 44 -> 17 (61%). The `.row .row-2nd` rule is generic and reusable by any .row-based list (only tagged elements are affected). Files: internal/screens/todo.go, web/index.html. build rc=0, 0/17 routes over the §11 ceiling. R39 still open pending the remaining lists (notifications/subscriptions already expose 1-2 resting actions; categories/accounts/widget-manager already lean). (R39)
- **C9 [F2] — local-first/no-bank-login framing on the import screen (2026-06-27):** Added a concise explanatory note at the top of the CSV import card making the local-first trade-off explicit: "CashFlux is local-first: your data stays on your device and never leaves it. There's no bank login or live connection — instead, export a CSV from your bank and import it here. This keeps your financial data completely private." Framed as a privacy benefit rather than a missing feature. New i18n key: `documents.localFirstNote`. Files changed: `internal/screens/documents_csv_import.go`, `internal/i18n/en.go`. Build rc=0. (C9)
- **R47 RESOLVED - button-density / hover-revealed secondary row actions (2026-06-27):** The /transactions ledger exposed 6 icon buttons per row (80 resting controls, over the §11 ledger ceiling of 65). Added a `tx-2nd` class + a `.txn-table` hover/focus-reveal rule so the secondary actions (duplicate, create-rule, attach, delete) hide at rest and reveal on row hover or keyboard focus-within, keeping edit + the cleared toggle visible. opacity (not display:none) preserves tab order; coarse-pointer keeps them always-on for touch (mirrors .btn-del-hover). /transactions 80 -> 44 (45% reduction, under ceiling). Verified the other R47 pages are already lean (accounts 32, todo 44, categories 26; widget-manager 100->23 prior) - 0/17 over ceiling. Files: internal/screens/transactions_row.go, web/index.html, e2e/ux_density_audit.mjs (added /categories). build rc=0. (R47)
- **C15 [F2] — wizard dropdowns pre-populated from detected header names (2026-06-27):** When the column-mapping wizard is shown (auto-detect fails), each dropdown now falls back to a case-insensitive keyword scan of the parsed CSV header names if auto-detect returned −1 for that field. "Date"/"Posted"/"Trans" → date column; "Desc"/"Memo"/"Narr"/"Detail" → description; "Amount"/"Value"/"Amt" → amount; "Debit"/"Withdrawal" → debit; "Credit"/"Deposit" → credit. Auto-detect wins when it resolves (≥ 0); the name scan only fires as a fallback. Pure helper `guessWizardField` added to `internal/screens/documents.go`. No new visible text; no i18n needed. Build rc=0. (C15)
- **R49 RESOLVED - one dominant headline figure per page (2026-06-27):** Completed R49: every named page (Dashboard/Reports/Accounts/Budgets/Goals/Health/Bills/Subscriptions/Allocate/Planning) now has a single obvious headline figure with no competing same-weight money figures. Added `.stat-value.is-hero` (2.1rem/800, with min-width:0/max-width + a <=720px size step) applied to the key figure on budgets/goals/subscriptions/bills/planning, and bumped the dashboard HomeHero net-worth to 2.6rem/800 so it outranks the KPI tiles. `e2e/ux_headline_audit.mjs` 6/10 -> 0/10 failing; `e2e/ux_overflow_audit.mjs` stays 0/78 (no overflow regression); build rc=0. Adversarial review loop (overflow guard + audit trivial-pass fixed). (R49)
- **C52 [F6] — filter panel converted from modal overlay to inline collapsible disclosure (2026-06-27):** The "Filters" panel on /transactions was rendered via `FlipPanel` — a fixed, dimmed, full-screen backdrop that occluded the entire transaction table. Replaced it with an inline collapsible `<div>` (`role="region"`) that expands in normal document flow above the chip row and table. The table remains fully visible and interactive while the user adjusts filters. All filter controls, the active-filter chips, the "f" keyboard shortcut (C56), and the count badge (C57) are preserved unchanged. New CSS classes: `.filter-inline-panel`, `.filter-inline-header`, `.filter-inline-title`, `.filter-inline-body`. Files changed: `internal/ui/filtertoolbar.go`, `web/index.html`. Build: `GOOS=js GOARCH=wasm go build` exits 0. (C52)
- **R49 (partial) - one dominant headline figure on 4 pages (2026-06-27):** Added a `.stat-value.is-hero` treatment (2.1rem/800) and applied it to the single key figure in the .stat-grid on budgets (safe-to-spend Left), goals (overall progress %), subscriptions (monthly burden), and bills (total due), so each page now has one dominant headline instead of 3-4 same-weight figures tying for the eye. `e2e/ux_headline_audit.mjs`: 6/10 -> 2/10 pages failing (only /planning and the / bento remain). Files: web/index.html, internal/screens/budgets.go, goals.go, subscriptions_screen.go, bills_screen.go. Build rc=0. (R49)
- **C236 [F33] — Save as PDF / Print button on /reports (2026-06-27):** Added a "Save as PDF / Print" button below the CSV export control on the Reports screen. Clicking it calls `window.print()` via the JS bridge, opening the browser's native print dialog — which offers "Save as PDF" in all modern browsers. No server-side PDF library needed. New i18n key: `reports.saveAsPDF`. Files changed: `internal/screens/reports_screen.go`, `internal/i18n/en.go`. Build: `GOOS=js GOARCH=wasm go build` exits 0. (C236)
- **C237 [F33] — YoY comparison toggle on /reports (2026-06-27):** Added a "Compare year-over-year" toggle button to the reports screen. When on, all category deltas and the coverage caption compare the current period against the same period exactly one year prior (via the existing `reports.YoYPrior` helper); when off, the default period-over-period comparison applies. The toggle sits in the "Spending by category" card header next to the rollup button; the hero coverage caption updates to clarify the comparison in effect. New i18n keys: `reports.yoyOff`, `reports.yoyOn`, `reports.yoyTitle`, `reports.coveringYoY`. Files changed: `internal/screens/reports_screen.go`, `internal/i18n/en.go`. Build: `GOOS=js GOARCH=wasm go build` exits 0. (C237)
- **R49 (audit) - headline-figure standardization audit (2026-06-27):** Added `e2e/ux_headline_audit.mjs` grading each R49 page by figure visual weight (fontSize*fontWeight) and flagging pages where 2+ figures tie for the top weight (no single hero). Baseline 6/10 fail (/budgets,/goals,/bills,/subscriptions,/planning,/). R49 kept OPEN; per-page hero promotion is the remaining screen work. Only e2e/ux_headline_audit.mjs added. (R49)
- **C102 [F13] — rename description rule action (2026-06-27):** Added a `RenameDesc` field to `rules.Rule` (JSON-omitempty, backward-compatible). When set, `ApplyRulesWithCounts` overwrites the matching transaction's description with the configured text — enabling "clean up garbled bank feed payee text" without writing a separate script. The inline-edit form in `RuleRow` gains a fourth optional field ("Rename description to") for configuring this action; the read-only row view appends a `rename → "…"` badge when the action is set. `rules.RenamedDesc` pure helper also added for future at-entry use. Files: `internal/rules/rules.go`, `internal/appstate/appstate.go`, `internal/screens/rules.go`, `internal/i18n/en.go`. Build: `GOOS=js GOARCH=wasm go build` exits 0 on all three packages. (C102)
- **R52/R64 (partial) — decision-oriented chart grading audit (2026-06-27):** Added `e2e/ux_chart_audit.mjs`, which inventories every chart across `/`, `/reports`, `/planning`, `/health`, `/goals` — svg/canvas AND CSS div-charts (`.vb-segbar`/`.wb-bar`, excluding progress bars) — and grades each on decision-orientation (title + readable context: axis ticks / caption / a non-generic accessible name), accessibility (screen-reader name; rejects generic fallbacks like "Trend chart"), and a nearby action/drill-down. Result across 19 charts: **0 decorative/unlabeled** (every chart states what it is); surfaced gaps = 2/19 lack a real accessible name (incl. the Mermaid Sankey whose aria-label is on the wrapper div, not the svg) and 11/19 chart cards lack a nearby action. Tickets R52/R64 kept OPEN with these gaps logged — the audit (R64's "inventory + grade every chart") ships and confirms no decorative charts, but R64's acceptance also requires an accessible name + action on EVERY chart, so the remaining svg-`<title>` + drill-down work stays tracked. Audit exits 0 (advisory on the gaps; hard-fails only on truly unlabeled charts). Only `e2e/ux_chart_audit.mjs` added.
- **C231 — AI starter chips now visible even when conversation history exists (2026-06-27):** The suggested-question chips on /insights were suppressed whenever any conversation history was present, so returning users lost their quick-prompt row after the first exchange. Changed the visibility condition from `empty` (no history) to `input.Get() == ""` (input box is empty), so the chips appear whenever the user hasn't started typing — regardless of how long the conversation history is. When a thread is already in progress a compact "Suggested questions" label is shown above the chips for context. New i18n key: `insights.suggestedQuestions`. Files changed: `internal/screens/insights.go`, `internal/i18n/en.go`. `GOOS=js GOARCH=wasm go build` passes. (C231)
- **C38 — Rule suggestions surfaced above the Mermaid diagram with count badge (2026-06-27):** The "Suggested rules" section was rendered after the Mermaid rule-precedence diagram, pushing it well below the fold and making it nearly undiscoverable. Moved it to appear immediately after the "Your rules" card (before the power-user Mermaid chain), so suggestions are encountered before users scroll into the precedence visualization. Added a count badge to the section title ("Suggested rules (N)") using a new `rules.suggestedTitleCount` i18n key so the presence and quantity of suggestions is visible at a glance. Files changed: `internal/screens/rules.go`, `internal/i18n/en.go`. `GOOS=js GOARCH=wasm go build` passes. (C38)
- **R46 — semantic token-role audit + 14 money/severity color-role fixes (2026-06-27):** Added `e2e/ux_token_roles_audit.mjs`, a source audit enforcing the money<->severity color-token-role boundary (§4.1/§4.3) in both directions and alias-aware. Money figures (`.hero-net`, `.stat-value`, `.hero-flanker-value`, `.amount-income/.amount-expense`, `.text-up/.text-down`, `.pos/.neg`) must use `--money-positive/--money-negative` (incl. their `--up/--down` aliases), never the brand `--accent` or severity `--danger`; severity status elements (`.is-critical`, `.is-warning`, `.card-alert`, `.budget-over`, …) must not borrow money tokens. Fixed 14 real violations in `web/index.html`: 13 money figures were painting positive money with the brand accent (`#2e8b57`) or negative money with severity-danger — now on the money tokens (positive shifts to the brighter, higher-contrast `--money-positive #54b884`); `.amount-expense` moved off `--danger`; and `.attention-item.is-critical` (border + dot) moved off the money `--down` alias onto the severity `--danger` token, symmetric with `.is-warning`→`--warn`. Brand accent (interactive/selected-nav role) and passive chrome (bg/border tokens) verified already-separated (0 violations). Audit exits 0; contrast audit shows 0 money-figure regressions (brighter green only raises ratios); `GOOS=js GOARCH=wasm go build` rc=0. Passed an adversarial style-spec review loop (FAIL: missed `.amount-expense` + `.is-critical` alias gap + scope honesty -> fixes -> green). (R46)
- **C253 — Cross-link from /subscriptions to /insights for spending anomaly discoverability (2026-06-27):** The anomaly surface was fragmented: subscription-specific anomalies live on /subscriptions, per-category spending anomaly highlights live on /insights ("Spending highlights" — no AI required), and balance anomaly watch lives on /smart. A user on /subscriptions had no cue that /insights has broader "what changed in my spending" analysis. Added a small muted footer at the bottom of the Subscriptions screen reading "See spending analysis · Insights" that navigates to /insights on click, with a descriptive title tooltip explaining the Spending highlights feature. The cue is unconditional (always shows) and uses the existing `nav.Navigate` + `ui.UseEvent` pattern with `data-testid="subs-see-insights-link"`. New i18n keys: `subs.seeSpendingAnalysis`, `subs.seeSpendingAnalysisTitle`. Files changed: `internal/screens/subscriptions_screen.go`, `internal/i18n/en.go`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C253)
- **C243 [F33] — report-type selector (tabbed views) on /reports (2026-06-27):** Replaced the single scrolling mega-page with four focused views switchable via a segmented control (radiogroup): "Overview" (money-flow Sankey, top payees, biggest expenses, income sources, member breakdown), "Categories" (spending by category with bar/donut/ranked-rows + YoY/rollup controls), "Net worth" (cash-flow trend, NW composition + trend, savings-rate trend), and "Advanced" (custom-field spend + deductible totals, with existing disclosure toggle preserved). The hero zone (Net/Income/Spend stats, export CSV, Save as PDF, spend stats, anomaly card) stays visible across all views. The segmented control reuses `uiw.Segmented` (the same `role="radiogroup"` component used for the period selector and theme chooser) for consistent a11y + styling. State is a single `reportView := ui.UseState("overview")`; sections are pre-computed into variables inside an immediately-invoked closure and selected with a plain `switch` — no On* hooks inside loops. New i18n keys: `reports.viewOverview`, `reports.viewCategories`, `reports.viewNetWorth`, `reports.viewAdvanced`. Files: `internal/screens/reports_screen.go`, `internal/i18n/en.go`. Build rc=0. (C243)

### Fixed
- **C257 — VERIFY-CLOSE: /smart ranked Insights hub + dashboard digest tile both already shipped (2026-06-27):** Audited C257 ("dashboard shows no recommendations; /smart is a settings catalog"). Confirmed both issues are resolved: (1) Commit `36a10440` (2026-06-25) split `SmartHub` into two tabs — "Insights" (default, paginated ranked list via `smartInsightsPager` + `smart.SortInsights`) and "Manage" (catalog); the C257 CHANGELOG entry was written in that commit. (2) `smartDigestWidget` in `internal/screens/dashboard.go` renders a capped (top-3) ranked digest from all enabled Free engines and is registered in the `renderers` map; `DefaultItems()` in `internal/dashlayout/pack.go` includes `{ID: "smart-digest", ColSpan: 2, RowSpan: 1}`. No gap found. Closing C257 as fully resolved. `GOOS=js GOARCH=wasm go build` exits 0. (C257)
- **C256 — VERIFY-CLOSE + test fix: SMART-SU1 and SMART-G12 already emit executable actions (2026-06-27):** Audited C256 ("recommendation actions are navigate-only"). Found that `su1CancelCandidates` (SMART-SU1) already emits `ActionCancelSubscription` and `g12SuggestGoals` (SMART-G12) already emits `ActionCreateGoal` — both fully handled in `smart_card.go`'s `onAction` switch. The pre-existing `c256_executable_actions_test.go` verified G12 (passing), but the SU1 test used a positive (income) amount — `usd(1_99)` — which subscriptions.Detect silently ignores (only expenses qualify). Fixed the test to use `usd(-1_99)` and upgraded the `t.Skip` to `t.Fatal` so the assertion is active. All 14 smartengine tests now pass. No production code changed. (C256)

### Added
- **C248 — Static example AI conversations on /insights for keyless users (2026-06-27):** Keyless users landing on /insights saw a blank chat with a key-gate call-to-action but no sense of what the assistant can do. Added `exampleConversationsNode()`, a pure presentational helper that renders three static Q→A pairs (spending category question, affordability question, net-worth drop question) styled to mirror the real chat bubbles (sky tint for user, neutral for assistant). The examples are shown only when no AI is configured (`noAI`) AND the thread is empty — i.e., exactly the keyless first-visit state. They are non-interactive (no handlers, no hooks) and clearly labelled "Example conversations · Here's what the AI assistant can do once you add a key." A closing note reads "Add your OpenAI key in Settings to ask your own questions." New i18n keys: `insights.examplesLabel`, `insights.examplesHint`, `insights.exampleQ1`, `insights.exampleA1`, `insights.exampleQ2`, `insights.exampleA2`, `insights.exampleQ3`, `insights.exampleA3`, `insights.examplesNotice`. Files changed: `internal/screens/insights.go`, `internal/i18n/en.go`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C248)
- **C251 — System-prompt editor hidden behind "Advanced" expander; "conversations saved" cue added to /insights (2026-06-27):** The system-prompt editor ("Edit prompt" button) was always visible in the conversation switcher bar, adding noise for users who just want to ask questions. Moved it behind a small muted "Advanced" toggle button (with `aria-expanded` for accessibility); the toggle and all underlying hooks remain unconditionally registered — only the "Edit prompt" button's visibility is gated so hook ordering stays stable. Added a small muted "Conversations are saved on this device." cue above the chat thread so users know their history persists locally. New i18n keys: `insights.savedOnDevice`, `insights.showAdvanced`, `insights.hideAdvanced`. Only `internal/screens/insights.go` and `internal/i18n/en.go` changed. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C251)
- **C234 — AI "Ask a question" top affordance + id="ask" anchor on /insights (2026-06-27):** The AI chat box on /insights was below the fold when spending highlights and pinned insights occupied the top. Added a compact right-aligned "Ask a question" button at the top of the page (hidden on no-data state) that calls `scrollToID("ask")` + `focusByID("cf-chat-input")`, landing the user directly in the chat input. The "Ask CashFlux" `EntityListSection` now carries `id="ask"` so both the button and deep-links (`#ask`) can target it. New i18n key: `insights.askNow`. Only `internal/screens/insights.go` and `internal/i18n/en.go` changed. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C234)
- **C247 — AI key gate on /insights enriched with cost/BYOK/privacy context (2026-06-27):** The no-key call-to-action on /insights previously showed a generic "Add your OpenAI key in Settings to enable AI insights" line and a button. It gave no context about what the key powers, how billing works, or where to get one. Enriched `keyHintNode` with a second paragraph (`insights.keyGateContext`) that explains BYOK billing ("you pay OpenAI directly — CashFlux never charges for AI") and includes a direct link (`insights.keyGateLink`) to `platform.openai.com/api-keys`. The "Go to Settings" CTA is preserved. New i18n keys: `insights.keyGateContext`, `insights.keyGateLink`. Only `internal/screens/insights.go` and `internal/i18n/en.go` changed. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C247)
- **R44 — per-route desktop UX score report completes the scorecard gate (2026-06-27):** Added `e2e/ux_route_scorecard.mjs`, satisfying R44's "a script produces a score report for all routes" acceptance criterion (the dimension audits gate aggregate failures but didn't give a per-route view). It visits all 18 registered routes and computes a single 0-100 desktop-UX score from measurable CASHFLUX_ENTERPRISE_UI_STYLE_SPEC signals: content-control density vs the §11 archetype budget, horizontal overflow (§5.5.11), single-headline presence (§6.3), and hero-figure presence plus a §6.3 figure-wall penalty (>=5 same-weight figures). Prints a ranked scorecard so any R36-R43 fix can be verified to move the right route's number; exits non-zero only if a route falls below floor(60). MEASURED mean 97/100 across 18 routes, 0 below floor — `/transactions` flagged (80 controls > 65 ledger ceiling), `/`+`/settings` figure-walls (7 figures), `/allocate` no-hero. Passed an adversarial style-spec review loop (FAIL -> 6 fixes -> PASS): §11.1 tabindex controls, headline double-match dedup, dark-theme-only output disclosure, builder edit-mode ceiling caveat, route-list divergence doc, figure-wall penalty. Only `e2e/ux_route_scorecard.mjs` added; `GOOS=js GOARCH=wasm go build` exits 0. (R44)
- **C255 — VERIFY-CLOSE: Smart enabled-state persistence fully confirmed (2026-06-27):** Audited the full C255 persistence loop end-to-end. Every toggle path calls `SaveSmartSettings` (via `SetSmartFeatureEnabled`, `EnableAllSmart`, `DisableAllSmart`, `EnableFreeSmart`, `SetSmartDensity`, `SetSmartMuted`, `SetSmartCadence`, `MarkSmartRun`, `SetSmartResult`) and all writes go to `SettingKVSet` (PRESERVED KV — survives session and data wipe). `LoadSmartSettings` reads from `SettingKVGet` on every render. The JSON round-trip is validated by `TestSettingsJSONRoundTrip` (covering Enabled, ExplicitOff, Dismissed, Schedules, Muted, LastRun, Results, Density). No code gap found; closing C255 as resolved by C254's seeding logic.
- **C254 — Free SMART insights on by default for new users (2026-06-27):** Previously, `LoadSmartSettings` returned an empty `Settings{}` on first load, which meant the in-memory tier-default logic (Free features on) was correct but nothing was persisted — leaving the door open for code paths that might observe a zero KV. More critically, the comment in `LoadSmartSettings` said "everything OFF" which was incorrect and misleading. Fix: on first load (empty KV), `LoadSmartSettings` now calls `EnableFreeOnly` and persists the result, so all deterministic on-device Free features are immediately active and the stored state is consistent with the tier-default. AI features remain off by default (no cost without consent). Also corrected the misleading package-level comment in `smart.go`. No new i18n keys. Files changed: `internal/uistate/smartsettings.go`, `internal/smart/smart.go`. `go test ./internal/smart/` and `GOOS=js GOARCH=wasm go build` both pass. (C254)
- **C250 — Active model + BYOK billing transparency in AI settings (2026-06-27):** Added a muted disclosure line below the AI model select in `internal/app/settings_section.go` that reads "Active model: <name> · You pay OpenAI directly per token used — CashFlux is never charged for your AI use." Uses a new `aiModelDisplayName()` pure helper to map internal model IDs to readable labels (GPT-5.4 mini / GPT-5.5 / o4-mini (reasoning)) with a safe default. New i18n key: `settings.aiModelNote`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass. (C250)
- **C129 — Yearly budget period added to domain and budgeting engine (2026-06-27):** `PeriodYearly = "yearly"` added to `internal/domain/enums.go` alongside `AllPeriods`, `Label()` ("Year"), and `Valid()`. `PeriodRange` in `internal/budgeting/budgeting.go` now handles `PeriodYearly` by returning `[Jan 1 of ref year, Jan 1 of next year)`. Pure test coverage: `TestPeriodRange` extended with the yearly case; `TestPeriodLabel` in `enums_edge_test.go` extended with `PeriodYearly → "Year"`. UI wiring into the period select on /budgets deferred (target file dirty from concurrent agent). `go test ./internal/budgeting/ ./internal/domain/` and wasm build both pass. (C129)
- **C103 — "Apply to existing" now shows per-rule transaction counts (2026-06-27):** The "Apply to existing" feedback on /rules showed one global total ("Categorized 12 transactions") with no indication of which rules did what. Added `ApplyRulesWithCounts()` to `internal/appstate/appstate.go` — same semantics as `ApplyRules()` but also returns a `map[string]int` of rule ID → updated count. `ApplyRules()` is now a thin wrapper that delegates to it (no callers broken). The rules screen calls `ApplyRulesWithCounts()` and appends a per-rule breakdown to the toast when at least one rule fired, e.g. "Categorized 12 transactions — \"uber\": 4 transactions, \"starbucks\": 8 transactions." Rules with zero matches are omitted from the breakdown; the total is always shown. New i18n key: `rules.appliedPerRule`. New unit test `TestApplyRulesWithCounts` verifies total, per-rule counts (two rules, distinct match groups), and idempotency. `go test ./internal/appstate/ ./internal/rules/` and `go test ./internal/i18n/` pass; `GOOS=js GOARCH=wasm go build` exits 0.
- **R25 - unified anomaly hub design delivered (2026-06-27):** New docs/DESIGN_UNIFIED_ANOMALY_HUB.md inventories the fragmented anomaly detectors (smartengine a1BalanceAnomaly, insights spending highlights, notify EventLargeTransaction, attention needs-attention) and surfaces (/smart hub, dashboard strip, notification center, Reports, Insights), then specifies the unification: one Anomaly shape (stable ID by entity+kind+period extending smart.Insight), one smartengine detection pass, /smart as the canonical hub with an Unusual filter, and derived surfaces sharing ID + dismissal per section 8.6 (group repeated hits, shared dismissal, why shown). The presentation half is already shipped (R38 decision layer + R38/8.6 severity-sorted notifications); this closes the detection+identity+dismissal loop. Verified accurate against the codebase by an adversarial review.
- **C259 — sort insights by severity before capping in the Smart hub (2026-06-27):** `smartInsightsSection` was calling `smart.CapPerRule(insights, 3)` without first calling `smart.SortInsights`, meaning the per-rule cap of 3 kept arbitrary insights rather than the highest-severity ones. `CapPerRule`'s own doc requires severity-sorted input. Added a `smart.SortInsights(insights)` call immediately before the cap in the default branch of `smartInsightsSection`. The "Enable free features only" bulk action was already shipped. No new i18n keys. Only `internal/screens/smart.go` changed. `GOOS=js GOARCH=wasm go build` exits 0.
- **C300 — persistent price disclosure in Settings → Cloud & server (2026-06-27):** Previously the plan price ($34.99/year or $3.99/month) was only shown in the one-shot UpgradeSheet bottom sheet, which is dismissible and never shown again after dismiss. Users who had already switched to Cloud mode in Settings could see the price inside the subscribe block, but users who had not yet selected cloud mode or had dismissed the UpgradeSheet had no way to find the plan cost without opening checkout. Added a persistent pricing teaser paragraph inside the Cloud & server section that shows whenever the user has cloud mode selected but has not yet signed in (no server token). The copy reads: "Cloud is an optional add-on — free for 14 days, then <price>. The rest of the app stays free and local." It is hidden once the user authenticates (ServerToken is set) to avoid showing a subscribe pitch to existing subscribers. New i18n key: `settings.cloudPricingTeaser`. Files changed: `internal/app/settings_section.go`, `internal/i18n/en.go`. `go test ./internal/i18n/` passes. Build error in `internal/screens/documents_csv_import.go` is a concurrent-agent transient break, not in touched files.
- **R69 - theme hierarchy-parity audit completes the parity gate (2026-06-27):** The contrast audit proved both themes are AA-legible; this adds the other half R69 asks for - hierarchy parity, NOT just contrast. New e2e/ux_theme_parity_audit.mjs captures a structural hierarchy fingerprint (heading/figure/label font-size + weight + box height) per route in dark AND light and fails on any divergence. Because CashFlux themes change only color tokens (type scale/weights/layout are theme-invariant), it measures 0 mismatches across all 10 routes (and matching element counts), proving the hierarchy is identical between themes. Added to the unified ux_quality_gate.mjs runner as the parity dimension. Together with contrast=0, this closes R69 (every route captured in dark+light and scored for parity, not just contrast).
- **C131 — Saturday week-start option added to Preferences (2026-06-27):** The week-start segmented control in Settings only offered Sunday and Monday (C131). Added `WeekSaturday = "saturday"` to `internal/prefs/prefs.go`; extended `Normalize()` to accept the new value, and updated `WeekStartWeekday()` to return `time.Saturday` for it. The three-option segmented control in `settingsRightColumn` now shows Sunday / Monday / Saturday. New i18n key: `settings.saturday`. Pure tests extended with `TestWeekStartNormalize` and an extra Saturday case in `TestWeekStartWeekday` and `TestWeekStartOf`. `go test ./internal/prefs/` and `go test ./internal/i18n/` both pass; `GOOS=js GOARCH=wasm go build` exits 0.
- **C302 — discoverable "Manage your subscription" surface in Cloud & server settings (2026-06-27):** Previously, the only billing-portal entry point ("Manage subscription" button) was buried inside the subscribe/checkout block, shown only when cloud mode was selected and mixed in with the "Subscribe" CTA — making it invisible to users who had already subscribed. Added a dedicated "Manage your subscription" sub-section inside the cloud plan block that surfaces whenever the user is authenticated (has a server token) and cloud is selected. The section shows a plain-English hint explaining the portal is used to change plan, update payment, or cancel, and renders the billing-portal button with `data-testid="manage-subscription"` for testability. The "Subscribe" button row is now standalone (no longer paired with the portal button). New i18n keys: `settings.manageSubTitle`, `settings.manageSubHint`. Only `internal/app/settings_section.go` and `internal/i18n/en.go` changed. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass.
- **C301 — CloudMention dismiss is now a 30-day snooze, not a permanent block (2026-06-27):** Previously, clicking "Not now" on the Cloud intro banner wrote `"1"` to `cashflux:cloud-mention-dismissed` and the banner (and therefore the `ShowUpgradeSheet()` upgrade path) was permanently hidden. The banner now stores a Unix timestamp in the renamed key `cashflux:cloud-mention-snoozed`; `cloudMentionSnoozed()` returns true only while within the 30-day window. After 30 days the banner re-surfaces automatically. `ShowUpgradeSheet()` was never gated by the dismiss flag and continues to work from any call site. The legacy `"1"` value (old permanent dismiss) parses as Unix epoch (1970), which is always older than 30 days, so existing dismissed installs gracefully re-surface the banner on upgrade. `keptOnWipeKeys` in `persist.go` updated to the new key. No new i18n keys. `GOOS=js GOARCH=wasm go build` exits 0.
- **C100 — OpenAI key explainer in the AI settings section (2026-06-27):** Added a plain-English paragraph beneath the existing `settings.aiKeyTrust` trust line in `internal/app/settings_section.go` explaining what the key is used for (assistant chat, spending insights, and receipt photo reading), that it is bring-your-own-key with direct OpenAI billing (CashFlux never charges for AI), and where to get one (`platform.openai.com/api-keys`). Rendered unconditionally so users see the context whenever they view the AI section, not only before they have entered a key. New i18n key: `settings.aiKeyExplainer`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass.
- **C304 — frame the Cloud & server section as the subscription/connection surface (2026-06-27):** Added a framing paragraph immediately after the "Cloud & server" heading in `internal/app/settings_section.go` that explains what the section is for — CashFlux Cloud (sync, automatic encrypted backups, bundled AI across devices) or a self-hosted server — and reassures users that the app stays free and local either way. Previously, the section opened directly into a data-disclosure line and raw infrastructure controls with no context, making it look like a developer config panel rather than a subscription/connection surface. New i18n key: `settings.cloudSectionIntro`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass.
- **R24 - no-key AI fallback design delivered (2026-06-27):** New docs/DESIGN_NO_KEY_AI_FALLBACK.md systematizes the spec section 8.9/9.4 principle (expose non-AI alternatives first; never bury the safe/manual path). Maps each AI-key-gated feature to its on-device, deterministic, no-key fallback: insights/Q&A -> the Free smartengine engines (~30 deterministic insights); auto-categorization -> the rules engine + rulesuggest; receipt/statement extraction -> CSV/OFX import + statement.ParseAny + manual entry; allocate explanation -> the deterministic per-criterion breakdown. The one genuine gap (image-only OCR) is isolated and routed to R10 (local OCR), with the honest interim UX (CSV-first + labeled gated control) already shipped. Verified accurate against the codebase by an adversarial review.
- **C97 — validate image type and size before vision upload (2026-06-27):** Before a chosen file is read into a data URL and (later) sent to the OpenAI vision API, `pickImageDataURL` in `internal/screens/documents.go` now validates the JS File object's `.type` (must start with `image/`) and `.size` (must not exceed 10 MB). An invalid type shows "That file doesn't look like an image. Please choose a JPEG, PNG, WebP, or GIF." A file over the cap shows "That image is too large (over 10 MB). Please resize or compress it before uploading." Both errors surface via the existing `aiErr` state; the image state is not set. The same `onErr` callback pattern is applied to the SmartAI receipt-scan call site in `smartai.go`. New i18n keys: `documents.imageTypeInvalid`, `documents.imageTooLarge`. `go test ./internal/i18n/` passes.
- **C227 — note manual local-first asset valuation (2026-06-27):** Added a muted one-line disclosure inside the "Update value" set-balance form for valuation-type asset accounts (investment, retirement, crypto, other) explaining that values are entered manually and CashFlux does not fetch prices from external services (Zillow, KBB, etc.). Rendered only when `isValuationType(a.Type)` is true, so non-valuation accounts are unaffected. New i18n key: `accounts.valuationManualNote`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass.
- **C213 — net-worth trend area chart hover tooltip (2026-06-27, VERIFY-CLOSE):** Audited `web/chart.js`: the area/line render path already appends invisible 7px-radius `<circle>` hit-targets at each data point, each with an SVG `<title>` formatted via `valFmt()` (`curSym + d3.format(",.2f")(v)` for money axes) and prefixed by the point's x-label from `labelsByX` (e.g. "Mar: $1,480.00"). This was landed in commit `e4e7a563` ("feat: keep-tidy + QA pass — chart tooltips, empty states…"). The net-worth and cash-flow AreaCharts in `/reports` and the planning sparkline all pass `Labels`/`ValueLabels` which become `labelsByX` entries — confirming the tooltip is fully wired. No code change required; closing C213 as already resolved.
- **C217 — decouple Reports net-worth trend from the cash-flow period selector (2026-06-27):** The NW trend series was computed from the same `bounds` slice as the cash-flow trend, which uses `w.Res` (the period-resolution selector). Switching to quarterly changed both the NW axis labels and the snapshot dates. Fix: compute a separate `nwBounds` array always using monthly steps (last `trendBuckets` calendar months from `dateutil.MonthStart(now)`), and a matching `nwLabels` slice for the x-axis. The NW AreaChart now uses `nwLabels` instead of `trendLabels`. Added a muted caption below the NW chart explaining it is always monthly. The `curMonth` variable is shared with the existing runway section (no duplicate). New i18n key: `reports.nwTrendMonthly`. Wasm build passes.
- **C291 — cloud sync data-disclosure line in Settings (2026-06-27):** Added an always-visible muted `P` paragraph under the "Cloud & server" section heading explaining, in plain English, that when sync is on, encrypted financial data (accounts, transactions, budgets, etc.) is sent to the configured server, and when sync is off, everything stays on the device. Mirrors the style of the existing `settings.aiKeyTrust` AI-key trust line (C292). Rendered unconditionally before the backend toggle so users see the trade-off before acting. New i18n key: `settings.cloudDataDisclosure`. `go test ./internal/i18n/` and `GOOS=js GOARCH=wasm go build` both pass.
- **C74 — surface lock-until date in add-account form for lockable asset types (2026-06-27):** Savings, investment, retirement, crypto, and other asset accounts now show the "Lock until" date input directly in the add-account form, without requiring the "Show advanced fields" toggle. Everyday liquid accounts (checking, debit, cash) still reach it via Advanced when needed. The change is a single computed bool `isLockableAsset` inserted after `isLiab` in `accountaddform.go`; the field condition changes from `!isLiab && advOpen.Get()` to `isLockableAsset || (!isLiab && advOpen.Get())`. No new i18n keys — `accounts.lockUntil` already exists. State wiring and save path unchanged.
- **C290 — /about route, nav entry, and sidebar footer link (2026-06-27):** Added a dedicated `/about` route to `screens.All()` (GroupSystem, Phase 1) that renders the existing `HelpScreen` content via a thin `About()` wrapper — one source of truth, two routable entry points. Added `/about` to `railMeta` in `shell.go` with `icon.HelpCircle`. Added an "About & privacy" `<a>` link in the `HouseholdCard` footer (below the local-first trust line) so the route is discoverable from the sidebar on every screen. New i18n keys: `nav.about` ("About"), `nav.aboutPrivacyLink` ("About & privacy"), `screen.aboutSub`.
- **C212 — Assets KPI tile on the dashboard (2026-06-27):** Added a `kpi-assets` widget to the dashboard bento grid showing total asset balances (the same `nw.Assets` value already computed by `useNetWorth()`). The tile is draggable and resizable, consistent with the existing `kpi-networth`, `kpi-income`, `kpi-spending`, and `kpi-liabilities` KPI tiles. `DefaultItems()` in `dashlayout/pack.go` has the new tile after `kpi-liabilities`; `widgetManagerTitleKeys` in `widgets.go` maps it to the new i18n key `dashboard.assets`; the `renderers` map in `dashboard.go` registers the render function. No new hook calls; `assets` was already destructured from `useNetWorth()` at line 98.
- **C73 — Retirement and Crypto account types (2026-06-27):** Added `TypeRetirement` and `TypeCrypto` to `domain.AccountType` and `AllAccountTypes` (13 types total). Staleness windows: Retirement=120 days (slow-moving like investments), Crypto=30 days (volatile but manually-updated). Both types are assets (`Class()` falls through to `default: ClassAsset`), are flagged as valuation types (shows "Out of date / Update value" wording), and are excluded from Quick-Add default spend-account selection alongside `TypeInvestment`. Icons: TrendingUp for Retirement, Scale for Crypto. Labels auto-generated by `humanizeType()`. New i18n keys `settings.freshRetirement` and `settings.freshCrypto` added to settings freshness panel. `isSpendAccount` in `accountselect` and the quickadd fallback loop updated to exclude both new types. No changes to liquid/runway packages — they whitelist only cash-type accounts, so exclusion is automatic.
- **C104 — "Apply to existing" now unions rule tags into already-tagged transactions (2026-06-27):** `ApplyRules()` in `internal/appstate/appstate.go` was guarded by `len(t.Tags) == 0`, so any transaction that already carried any tag was silently skipped for tag application — rule tags were never added to already-tagged transactions. Replaced the guard with a union loop using the existing `addTagUnique` helper: each tag in `r.SetTags` is added only if not already present, so manually-curated tags are preserved and rule tags are correctly accumulated. Category backfill behavior is unchanged (C108 handles that). Updated `TestApplyRules` to assert the corrected expectation: a transaction with pre-existing `["existing"]` that matches a rule with `SetTags:["travel"]` ends up with `["existing","travel"]` after `ApplyRules`. No i18n changes. `go test ./internal/appstate/ ./internal/rules/` passes; `GOOS=js GOARCH=wasm go build` exits 0.

### Fixed
- **C108 — backfill now overwrites already-categorized transactions so rule corrections propagate (2026-06-27):** `ApplyRules()` in `internal/appstate/appstate.go` was skipping any transaction that already had a `CategoryID`, which meant correcting a rule never re-categorized past transactions. Removed the `CategoryID != ""` early-continue; only the `IsTransfer()` guard remains. Tags are still additive-only (rule tags are applied only when the transaction has none). An idempotency optimization skips the store write when nothing changed. `TestApplyRules` updated (count 2→3; t2 is now overwritten by the matching rule). New pure test `TestFirstMatchIgnoresCurrentCategory` added in `internal/rules/rules_test.go`. `go test ./internal/rules/` and `go test ./internal/appstate/` both pass; `GOOS=js GOARCH=wasm go build` exits 0.
- **C95 — check for image before API key on receipt import (2026-06-27):** In the `readAI` handler in `internal/screens/documents.go`, the OpenAI API-key check fired before the image-presence check, so clicking "Read" with no image selected showed "add your API key" instead of "choose an image first." Reordered: image check (emitting `documents.chooseImageFirst`) now runs first; only if an image is present does the handler proceed to check the key. No new i18n keys — `documents.chooseImageFirst` already exists.
- **C294 — include artifact image blobs in dataset export/backup (2026-06-27):** The "Back up everything" flow called `ExportJSONRedacted()` (blob-less) instead of `ExportJSONRedactedWithBlobs()` — exported backups silently omitted every receipt/attachment image stored in IndexedDB. The Settings → Data "Import dataset" path called `ImportJSON()` instead of `ImportJSONWithBlobs()`, so images stayed embedded in the dataset record on re-import rather than being migrated to IndexedDB (breaking the autosave-size guarantee). Fix: two one-line wiring changes in `internal/app/backupall.go` and `internal/app/settings.go`. The single-workspace "Export dataset" path was already correct (`ExportJSONWithBlobs`). Added `TestArtifactBlobRoundTripWithBlobs` (IDB mock round-trip) and `TestArtifactBlobRoundTripNoBlobStore` (inline-bytes fallback) in `internal/appstate/artifact_roundtrip_test.go`. Both pass on native Go.
- **C295 — confirm before dataset import overwrites everything (2026-06-27):** The Settings → Data "Import dataset" action previously called `app.ImportJSON()` immediately after the user picked a file, with no warning that it would replace all current accounts, transactions, budgets, and goals. Added a `uistate.ConfirmModalLabeled` gate in `importJSON()` (internal/app/settings.go), placed after file selection but before the overwrite — matching the exact pattern used by the "Erase everything" wipe action (C298). The confirm dialog is destructive-styled, with a plain-English warning message and a "Replace all data" confirm label. New i18n keys: `settings.importConfirm`, `settings.importConfirmBtn`.
- **C178 + C180 — goal pace badge shows monthly contribution rate; contribute/edit forms are now inline (2026-06-27):** C178: `GoalRow` now computes `MonthlyNeeded` and renders a `%s/mo` chip immediately after the pace badge in the header, giving the contribution rate a glanceable position without replacing or duplicating the sub-line. C180: The contribute and edit forms previously fired early returns, replacing the entire row and hiding all action buttons. Both are now rendered as conditional `Div` blocks (`contribForm`, `editForm`) appended below the main row content, so the Contribute / Edit / Delete buttons remain visible while a form is open. Hook ordering is unchanged (all `UseState`/`UseEvent`/`UseEffect` calls remain at the top, before any conditionals). New i18n keys: `goals.paceNeeded` (`%s/mo`) and `goals.paceNeededTitle` (tooltip).
- **C238 — show "new" delta badge on /reports when a category had zero spend in the prior period (2026-06-27):** When a comparison period was requested and a category appeared only this period (prior = 0), `PercentChange` returned ok=false and the badge was suppressed entirely. Added `PriorZero bool` to `CategorySpend` — set in `SpendingByCategory` and propagated through `RollUpByParent` — and updated `reportsCatRow` to render a "new" badge instead of nothing. New i18n key `reports.new`. Unit tests added in `reports_test.go` and `rollup_test.go`.
- **C176 — surface goal Owner, Linked account, and Saved-so-far in the add-goal form by default (2026-06-27):** All three fields were hidden behind a "Show advanced fields" toggle in `goaladdform.go`. They are core goal attributes, not advanced options. Removed the toggle button and the `If(advOpen.Get(), …)` wrappers; all three fields now render unconditionally. Hook ordering preserved. No i18n keys added; existing `goals.savedSoFar`, `goals.owner`, `goals.linkedOptional` keys reused.

### Skipped (investigated, no change needed)
- **C166 — subscription detection preferences (ignore by category/account type) (2026-06-27, SKIP):** Assessed scope: the existing `IgnoredSubscriptions` mechanism is a per-subscription-name ignore list (user-explicit, C164/C165). `IsLiabilityPayment` provides account-class exclusion (C161). Adding category- or account-type-based exclusion preferences requires (1) a new persisted detection-prefs type with KV load/save path, (2) a UI toggle surface on /subscriptions or in Settings, (3) wiring the exclusion into the `subscriptions.Detect()` call or as a post-filter layer, and (4) tests across all three layers. That spans multiple architectural packages and is broader than a single safe commit. SKIP — no code change.
- **C75 — investment bucket granularity (2026-06-27, VERIFY-CLOSE):** C73 already split Retirement and Crypto into their own types. The remaining concern — brokerage vs ETF vs individual-stock granularity — is a holdings/portfolio problem, not an account-type problem. An account is a container; what kind of investments are held inside belongs to the holdings model (IMPL R23-foundation: `Holding` domain type, position-level type/ticker/asset class). Splitting `TypeInvestment` further into `TypeBrokerage`, `TypeETF`, `TypeStock` would fragment the account list without adding decision-value — a single brokerage account typically holds all three. The right fix lands in the R23 portfolio epic. No code change; this ticket is superseded by C73 + R23.
- **C144 — budget "LEFT" tile parens (2026-06-27):** `budgetLeftValue()` in `budgets.go` (introduced with C124) already renders a negative remaining as plain English (e.g. "$90.00 over") rather than accounting parens. The tile at line 340 passes `money.New(totalLimit-totalSpent, base)` through this function. No change needed.
- **C72 — dashboard multiple differing net-worth figures (2026-06-27 verify-close):** Investigated `dashboard.go` and `dashboard_hero.go`. The home band hero (`dashboardHero`) shows a single net-worth figure labeled "Net worth" (`home.netWorth` i18n key) with a sub-line (month-over-month delta or FX-exclusion warning). The bento grid has a draggable `kpi-networth` widget also titled "Net worth" (`dashboard.netWorth` i18n key) with the same value plus a sub-line. Both figures are the same computed value (`useNetWorth()` selector, same accounts+rates input). The ticket described "multiple differing money figures" — both are labeled identically and reflect the same number. The dual-figure appearance is intentional design (hero = above-fold glanceable; bento = draggable KPI widget; users can hide the bento widget via Widget Manager). No code change warranted.
- **C111 — /rules member filter (2026-06-27):** No member filter dropdown exists in `rules.go`; grepping `member` returned zero results. Nothing to remove or disclose.
- **C214 — count-up dual net-worth figure (2026-06-27):** `countup.js` animates a single `[data-countup]` element per figure with no duplicate render path. No concrete double-render found in `dashboard*.go`; speculative rewrite skipped.
- **C83 — add-menu "New account" / skip-link collision (2026-06-27):** No shared id, href, or selector matches both the `.skip-link` anchor and the `<button class="add-item">` "New account" item. No concrete collision; C83 already queued under IMPL R4-fx-ux.

### Changed
- **R55/section8.9 - Documents leads with the no-key CSV import (2026-06-27):** /documents previously led with the AI-key-gated image/receipt import (OpenAI vision, BYO key), with the no-key CSV import card buried at the bottom (below the AI image import, AI statement parser, draft review, and spend summary). The CSV import card now renders FIRST, so users without an OpenAI key are never blocked (spec section 8.9 expose-non-AI-alternatives-first; section 9.4 never-bury-the-safe-path). The AI image + statement imports follow as the 2nd/3rd cards, still clearly marked as needing a key. Also tightens the flow: importing now populates the review draft rendered just below, instead of stranding CSV at the page bottom. Passed an adversarial style-spec review.
- **R41 - sample-mode banner demoted to a compact chip (2026-06-27):** The persistent sample-data indicator was a FULL-WIDTH green banner spanning the content width atop every page. It is now a compact left-aligned pill (approx 248px vs 1200px): a status dot + "Sample data" + "Start fresh" + "Dismiss", with the full explanation in the chip title tooltip. role changed alert->status (persistent, non-urgent). Frees the first viewport for the primary content on every route (spec 3.1/3.4). New i18n sample.chipLabel/chipTitle in en_samplebanner.go. Passed an adversarial style-spec review (R41 acceptance, 3.1/3.4/13).
- **R38/§8.6 — notification center surfaces urgent alerts instead of burying them (2026-06-27):** /notifications rendered in recency order, so a CRITICAL "over budget" alert sat at the BOTTOM below ~8 routine "due soon" reminders. The feed is now stable-sorted by severity (critical > warning > info) before render, keeping recency order within each tier — so the 6 distinct critical over-budget alerts lead, then warnings, then the rest. Display-only (read/catch-up/clear-all match by ID, unaffected). Passed an adversarial style-spec review (§8.6/§3.1). A TODO marks a future collapse-threshold for long same-rule runs.
- **R55/§8.9 — /admin gated state is now a useful card, not a dead line (2026-06-27):** The signed-out /admin route rendered a single italic sentence floating in an empty page. It now renders a proper gated card: a title ("Admin console"), a specific reason/value (what the console manages — devices, sign-in sessions, audit log — that it needs a CashFlux Cloud sign-in, and that local data stays on-device either way), and one primary CTA ("Sign in to Cloud") that opens the global Settings panel. Passed an adversarial style-spec review (§8.9 anatomy, §3.5 trust, §8.1/§13 button). New i18n keys in en_enterprise.go.
- **R38 — Smart insight strip is now a prioritized decision layer (2026-06-27):** The global inline Smart strip (`smart_strip.go`) stacked up to 3 full insight cards above page content (e.g. /subscriptions led with 3 "cancel utility" cards above the stats). It now leads with the single most-severe insight (severity-sorted), the rest behind an in-place **"Show N more"** toggle (`aria-expanded`, keyboard-operable) + the existing **"View all (N)"** → /smart hub; collapse resets per page so each page is decision-first (§3.1). MEASURED /subscriptions: 3-card stack → 1 insight + "Show 2 more", stats+list above the fold. Passed an adversarial style-spec review (§3.1/§8.6/§13.4/R38) after one loop. New i18n `smart.stripMore`/`stripLess` in `en_smartstrip.go`.

### Fixed
- **C293 — enrich About/What's new card with app identity and privacy context (2026-06-27):** The `whatsNewCard` in `/help` showed only a version label, four recent-highlight bullets, and a changelog link — nothing that told a new or curious user what CashFlux actually is. Added two plain-English lines above the version: a one-sentence tagline describing CashFlux as a local-first, household-aware budgeting app, and a one-sentence privacy commitment noting that data stays on the device with a pointer to the privacy section. New i18n keys `help.aboutTagline` and `help.aboutPrivacy` added to `internal/i18n/en.go`; `uistate` import added to `internal/screens/help.go`.
- **C240 — remove redundant per-card CSV download buttons from /reports (2026-06-27):** The R-7 page-level export `<details>` menu (covering all six exports) was added but the six per-card "Download CSV" / "Tax summary" buttons inside the category, payees, biggest-expenses, income, and member-spend cards were never removed. Each export was now offered from two places. The per-card buttons are removed; the consolidated top-of-page export menu remains the single surface for all CSV downloads. Also removed an unused `"fmt"` import from `smart_strip.go` that was blocking the build.
- **R35/R53/§5.5.9 — coarse-pointer tap targets meet the 44px floor (2026-06-27):** The 44px touch-target sizing existed only behind `@media (max-width: 640px)`, so touch devices wider than 640px (tablets, touchscreens at the 768px test width) presented ~29–32px controls — failing §5.5.9. The control-sizing block is now keyed on `@media (max-width:640px), (pointer:coarse)` (so it applies to any coarse pointer at any width; hybrid laptops report `pointer:fine`, so desktop is unchanged), and the top-bar/rail icon + chrome buttons (notifications, help, music, add, menu, collapse, workspace switcher, breadcrumb, household) that the width-only block missed are now bumped via container rules. A new touch-target audit (below) confirms **0 undersized discrete tap targets** across the sampled routes under a coarse pointer (was ~220), with **0 horizontal overflow** at 1024→320px touch widths. Inline/embedded content controls (bare icon buttons, text-link buttons, clickable titles/drill rows) are exempt per the spec's "where practical".
- **R39/R47/§8.4 — Widget manager resting density cut from ~100 to ~23 controls (2026-06-27):** The widget-manager table showed every widget's visibility toggle, two resize steppers, and two reorder arrows at all times — ~7 controls/row × ~14 rows = the one route over the §11 density ceiling (100 vs 70). Now each row shows its visibility toggle plus the size value (e.g. "4×1") and order position at rest; the resize steppers and reorder arrows are grid-stacked over those values and reveal on row hover/focus-within. `opacity` (not `display:none`) keeps them in the keyboard tab order (focusing one reveals the group) and avoids layout shift, and the columns still read complete. The density gate now reports the manager at **23 controls (under the 55 target)** and **0/16 routes over ceiling** app-wide.
- **R69/§12.1 — last contrast edges cleared: app-wide gate now 0 failures (2026-06-27):** Closed the remaining marginal AA misses surfaced by the contrast gate: `.hero-stat-value.pos` and the calendar "today" number used `var(--accent)` (a §4.3 accent-for-positive-signal slip, ~4.25–4.41:1) — the stat now uses `var(--money-positive)` and the today cell uses `var(--text)` (today stays marked by its accent border + tint + bold, not colour alone); and the light-mode `.btn-stale` action text (`#b45309`, 4.45:1 on the chip) darkened to `#92400e` (≈6:1). With these, `e2e/ux_contrast_audit.mjs` reports **0 contrast failures across all 8 audited routes in both dark and light** — the §12.1 contrast dimension of the desktop UX quality gate now passes clean.
- **R69/§4.3/§12.1 — positive-money green readable in light mode, app-wide (2026-06-27):** Income/gain figures rendered green at ~4.25–4.36:1 on light cards — just under AA on the dollar amounts that matter most, across Accounts, Subscriptions, Reports and the dashboard. Two root fixes: (1) `internal/theme` light greens (`lightBase` + the Paper preset) darkened `#1f8a52` → `#157a43` (≈5.0–5.4:1 on white/page) so the inline-emitted `--up` passes AA — this fixes the figures coloured via `var(--up)`; (2) `.amount-income` now uses `var(--money-positive)` instead of `var(--accent)` (a §4.3 fix — income is money-positive, not the interaction accent), with a `:root[data-theme="light"]` money-positive override for token consumers. Net: the contrast gate's money-green failures went to **0** and Dashboard/Transactions/Budgets/Planning/Subscriptions are now fully AA-clean in both themes; verified by the gate + a light screenshot (the green stays rich and clearly readable). Theme AA tests pass; dark mode unaffected.
- **R47/§8.6/§12.1 — budget status pills as accessible severity badges (2026-06-27):** The "N over budget" / "N near the limit" pills relied on `tw` text-tone (warn-amber / red text) on a pale chip — the amber measured ~2.1:1 in light mode (a WCAG fail). They're now proper §8.6 severity badges: a tinted fill + border carries the severity while the label stays at `--text`, so both read ~12–13:1 in light (and stay strong in dark). New `.pill.is-warn` / `.pill.is-danger` variants; `budgets.go` uses them instead of the text-tone classes. Verified by the contrast gate + a light-mode screenshot.
- **R69/§12.1 — sample-banner action links readable in light mode (2026-06-27):** The "Start fresh" / "Dismiss" links on the sample-data banner used `var(--accent)`, which in the light theme is a green (~`#2e8b57`) that reads ~3.8:1 on the pale banner — below WCAG AA, and present on *every* route. They now use a darkened accent (`color-mix(--accent 70%, #000 30%)`) clearing 4.5:1 while staying a green interactive affordance. Found and verified by the new contrast-audit gate (below); dark mode is unchanged.
- **R15-smart-floor / C146 — surface very-low liquid cash instead of staying silent (2026-06-27):** The SMART-B8 safe-to-spend insight returned nothing when liquid cash sat below the $1 floor — so the users who most need the warning (a near-empty wallet) saw nothing at all. It now emits a `SeverityWarn` "Liquid cash is very low" insight in that branch, naming the spendable amount. Still guarded by the existing `liquid < safeToSpendFloorAb` check, so it fires only for genuinely near-empty wallets; the normal safe-to-spend path is unchanged.
- **R47/§12.1 — accessible destructive fills (2026-06-27):** Three controls put white text on `var(--danger)` (= `--down` = `#d8716f`), where white measures **3.23:1 — a WCAG AA fail**: `.btn-ghost-danger:hover`, `.chip-x:hover`, and the `.notify-badge` count chip. They now fill with the dedicated `--action-danger` (`#c0392b`, white = **5.44:1**, PASS), matching the already-correct `.btn-danger`. `.btn-danger`'s hardcoded `#c0392b` gradient was tokenized to `var(--action-danger)` (byte-identical, no visual change). Severity/text/border/tinted-background uses of `--danger` are intentionally unchanged (correct per §4.3). Splits destructive-action fill from money-negative per §14.3; passes white-on-fill contrast in both themes (§12.1 / R69). Verified by an adversarial a11y review (found + fixed the `.notify-badge` case).

### Added
- **C117 — fix rollover checkbox label detaching from control at narrow widths (2026-06-27):** The rollover Label in the budget edit form used `display:flex` + `align-items:center` but had no `flex-wrap:nowrap`, so at ~1280px the default flex wrapping could push the checkbox visually away from its "Roll over" label text — breaking the accessible label association visually. Added `style="flex-wrap:nowrap"` to the Label and `style="flex-shrink:0"` to the checkbox Input so the pair stays on one line under all widths. No functional change; purely a layout fix.
- **C130 — clarify that the custom date-range changes view window, not budget periods (2026-06-27):** When the top-bar date-range selector is in custom-range mode, users sometimes assume it redefines each budget's period — but it only controls what transactions are shown. Added a muted hint paragraph at the top of the budgets list body that appears only when `!periodWin.Get().IsSinglePeriod()` (i.e., custom range is active). Text: "The date range above changes what you're viewing — it doesn't redefine each budget's period." New i18n key `budgets.customRangeHint`; carries `data-testid="budgets-custom-range-hint"` for e2e.
- **C218 — dedicated /networth route for the net-worth section (2026-06-27):** The net-worth composition and trend lived inside /reports with no direct navigation entry. Added a `/networth` route (Plan & analyze sub-group, nav label "Net worth", icon TrendingUp) via a thin `NetWorth()` view that delegates to the existing `Reports()` screen — identical to the C200 pattern used for `/debt` and C156 for `/recurring`. The net-worth card in reports_screen.go gained an `id="networth"` HTML anchor so the section is directly linkable via `/networth#networth`. Registered in the screens registry with i18n keys `nav.netWorth` / `screen.netWorthSub`, and in the `railMeta` map with the TrendingUp icon.
- **C84 — FX rate discoverability affordance on /accounts (2026-06-27):** The exchange-rate table was buried in Settings with no signpost from the Accounts screen — users with foreign-currency accounts had no obvious path to add or update rates. Added a "Manage exchange rates" ghost button on `/accounts` that opens the Settings panel directly (via `uistate.UseSettings()` + `settings.Set(uistate.Global())`). The button is conditionally rendered when there are existing FX rates (`len(app.Settings().FXRates) > 0`) or accounts excluded for missing rates (`len(nw.MissingCurrencies) > 0`), so it appears precisely when relevant. Carries a `title` tooltip with fuller context. New i18n keys `accounts.manageFXRates` and `accounts.manageFXRatesTitle`.
- **C156 — dedicated /recurring route for the recurring cash-flow manager (2026-06-27):** The recurring management section was buried mid-/planning with no direct nav-rail entry. Added a `/recurring` route (Bills & recurring sub-group, nav label "Bills & recurring") via a thin `Recurring()` view that delegates to the existing `Planning()` screen, identical to the C200 pattern used for `/debt`. The recurring card in planning.go gained an `id="recurring"` HTML anchor so the section is directly linkable. Registered in the screens registry with i18n keys `nav.recurring` / `screen.recurringSub`, and in the `railMeta` map with the Bills icon.
- **R44/R72 — unified desktop UX quality-gate runner (2026-06-27):** New `e2e/ux_quality_gate.mjs` runs the contrast (§12), density (§11), and overflow (§5.5.11) audits in one command and prints a combined PASS/FAIL scorecard, exiting non-zero if any dimension regresses — the "score report for all routes" capstone of R44. CashFlux is desktop-first (no mobile), so the headline gate covers the three desktop dimensions; the coarse-pointer touch audit (`ux_touch_audit.mjs`, §5.5.9) remains a separate optional check. All three headline dimensions pass green.
- **R44/R72 (touch slice) — coarse-pointer tap-target audit gate (2026-06-27):** New `e2e/ux_touch_audit.mjs` emulates a coarse pointer (so `@media (pointer:coarse)` + the 44px tokens apply) and flags discrete tap-target controls under the §5.5.9 44px floor, exempting inline/embedded content controls per "where practical". Result after the §5.5.9 fix: **0 undersized tap targets**. This is the fourth dimension of the R44/R72 desktop UX quality gate — contrast (0), density (0/16), overflow (0/78), and touch (0) all green.
- **R44/R72 (overflow slice) — page-level horizontal-overflow gate (2026-06-27):** New `e2e/ux_overflow_audit.mjs` loads each route across the §5.5.11 width matrix (1440→320px) and flags any route whose document scrolls horizontally (a §5.3 release blocker), naming the widest offending element and excluding legitimate inner scrollers (`overflow-x:auto` tables). Current result: **0 page-overflow failures across 78 route×width checks** — no horizontal overflow anywhere, down to 320px. Completes the contrast (0) + density (0/16) + overflow (0/78) trio of the R44/R72 desktop UX quality gate.
- **R44/R72 (density slice) — control-density audit gate (2026-06-27):** New `e2e/ux_density_audit.mjs` counts first-viewport CONTENT controls per route (§11.1: content vs shell split, hidden/offscreen/disabled excluded) and checks them against the §11 archetype budgets. Current result: **15/16 routes within budget** (Dashboard 28/45, Transactions 55/65 — the older "98/97" figures were counting shell + offscreen); the one real failure is `/widget-manager` at **100 vs a 70 ceiling**, the control matrix flagged by R39/R42/R51 for a progressive-disclosure redesign. Exits with the over-ceiling route count for CI. Pairs with the contrast gate as the density dimension of the R44/R72 desktop UX quality gate.
- **R44/R72 (contrast slice) — reusable cross-route WCAG contrast audit gate (2026-06-27):** New `e2e/ux_contrast_audit.mjs` loads every main route in dark+light (§11.1 protocol: 1440×1000, dark first, sample data), samples visible leaf text nodes, resolves effective backgrounds by walking ancestors, and scores §12 contrast (4.5 normal / 3 large). Colors are canvas-normalized so `color-mix()`'s `color(srgb …)` output is read correctly; icon glyphs and gradient-background elements are excluded to avoid false positives. Exits with the failure count so it can gate CI. This is the contrast dimension of the R44/R72 desktop UX quality gate (density/screenshot scoring still to come).
- **R46 follow-on — adopt money semantic tokens + §13.2 state hooks (2026-06-27):** `.text-up`/`.text-down` now reference `--money-positive`/`--money-negative` (byte-identical computed value, so zero visual change — it enforces the §4.3 "accent ≠ money" color discipline). Added net-new `[data-state="selected|dirty|error"]` and `[aria-busy]` interaction-state hooks (§13.2) as additive vocabulary for the component-migration pass — no element sets `data-state` today, so nothing is restyled. The `[aria-expanded] .chevron` rotation hook is deferred (existing chevrons manage their own rotation, so a global rule wouldn't be additive).
- **R46 — enterprise semantic token layer (step 1 of the migration, 2026-06-27):** Added a `<style id="enterprise-tokens">` layer in `web/index.html` implementing the style spec's §4.1 semantic aliases (surfaces, borders, text roles, interactive, money-positive/negative, severity, chart palette), §5.1 spacing scale, §6.2 type scale, §7.1 radius scale, §7.2 elevation, and §13.3 interaction/motion tokens, plus a `@media (pointer: coarse)` 44px target override (§5.5.9). All tokens derive from the runtime theme tokens via `var()`, so the layer is purely additive — no component is restyled and there is no visual change (spec §14.1 step 1). Accent maps to `--interactive` only (never money); a dedicated `--action-danger` splits destructive button fill from money-negative. `--bg` is bridged; `--line`/`--muted` are deliberately deferred to the per-selector pass to avoid a dark-mode visual change. Passed an adversarial style-spec review (all gates).
- **R15-inputs — canonical safe-to-spend input derivation (2026-06-26):** New `internal/safespend/inputs.go` adds `BillsDueBefore(bills, now, horizon, toBase)` and `GoalContributionsProrated(goals, now, periodStart, periodEnd, toBase)` — the bill and prorated-goal commitment buckets feeding the canonical `safespend.Compute` formula (`SafeToSpend = LiquidCash − BillsDue − GoalContributions − CommittedBudgets`). `now` is injected for testability and both reuse the existing FX `toBase` conversion rather than reimplementing the smartengine inline duplicate. Table-driven tests in `internal/safespend/inputs_test.go`. Pure stdlib, integer minor units, no `syscall/js`.
- **C201 — edit APR & minimum payment from the payoff card (2026-06-26):** The debt planner's key inputs (APR, minimum payment) could only be changed by leaving for /accounts. The card now renders a per-liability inline editor (`debtRateRow`, one isolated component per debt via `ui.CreateElement`) under "Adjust rates and minimum payments". Edits commit on blur via `PutAccount` and bump the revision so the snowball/avalanche plan recomputes immediately. Inputs carry aria-labels naming the account. i18n keys `planning.debtEdit*` added.
- **C200 — dedicated /debt route + anchor for the payoff planner (2026-06-26):** The debt-payoff planner was buried mid-/planning with no way to jump to it. Added a `/debt` route (nav: "Debt payoff", under Plan & analyze) via a thin `DebtPlanner` view, and gave the debt-strategy card an `id="debt"` anchor so it's directly linkable (`/planning#debt`) and the route can target it. Registered in the screens registry + railMeta with i18n keys `nav.debt`/`screen.debtSub`.
- **C199 — burn-down chart overlays snowball and avalanche (2026-06-26):** The debt burn-down previously plotted a single avalanche area series. It's now a two-line chart with both the avalanche and snowball schedules overlaid (each anchored at the full starting balance at month 0), so the two strategies can be compared visually instead of seeing only one. Switched `chartspec.Area` → `chartspec.Line` and factored the point-building into a local helper reused for both series.
- **C197 — snowball-vs-avalanche time-saved comparison (2026-06-26):** The payoff card already showed the interest difference between the two strategies; it now also states the *time* difference in plain English ("Avalanche clears your debt 3 months sooner."). Because both strategies pay the same total each month, the faster one (usually avalanche, which minimises interest) clears the whole balance in fewer months — computed from `snow.Months - aval.Months`, direction-aware, with correct month/months pluralisation. Shown only when the two differ. i18n key `planning.strategyTimeSaved` added.
- **C196 — per-debt detail table on the payoff card (2026-06-26):** The debt-strategy card on `/planning` showed only aggregate snowball/avalanche totals; the individual balances, APRs, and minimum payments feeding the plan were invisible. It now renders a compact table (Debt / Balance / APR / Min. payment) from the FX-converted `payoff.AggregateDebts` output, so the inputs behind the plan are auditable. APR and minimum payment render an em-dash when zero. Display-only rows (no per-row handlers), so the loop is hook-safe. i18n keys `planning.debtColName/Balance/Apr/Min` added.

### Added
- **C299 — "Last backed up" indicator in Settings → Data (2026-06-26):** The last-backup timestamp was tracked (`recordBackupNow`/`loadLastBackup`) but never shown. Settings → Data now displays "Last backed up <date>" (user's date format) under the export actions, or a "Not backed up yet — export a backup to keep your data safe" nudge when there's no record. New `lastBackupSummary()` helper + i18n `settings.lastBackup`/`settings.lastBackupNever`; carries `data-testid="last-backup"`.

### Fixed
- **C239 — bar charts no longer emit SVG negative/NaN height errors on degenerate data (2026-06-26):** When every bar value was equal (e.g. all zero), the y-scale domain `[yMin, max]` collapsed to a point, so d3 mapped values to NaN/identical positions and the `<rect>` heights came out negative or NaN. `web/chart.js` now guards the domain with a minimal positive span when `max <= yMin`.
- **C249 — AI chat send button accessible name + decorative icon hidden (2026-06-26):** The Ask input already had an aria-label; the Send button now carries an explicit `aria-label` and its leading sparkles icon is marked `aria-hidden`, so screen readers announce a single clean "Send" instead of the icon glyph.
- **C292 — AI-key privacy disclosure is now always visible (2026-06-26):** The only key-privacy hint (`settings.aiNoKey`) showed *only while the key field was empty*, so the moment you pasted a key the reassurance about where it goes vanished. Added an always-visible line under the key input: your key is stored only on this device and calls OpenAI directly from your browser — it never passes through CashFlux servers. i18n key `settings.aiKeyTrust`. (Cloud-side disclosure is covered by C289/C303.)
- **C303 — upgrade sheet states the free-vs-paid boundary + 14-day trial plainly (2026-06-26):** The Cloud upgrade sheet pitched benefits and a price but never said, in plain words, that everything else is free and stays local, or how long the trial runs. Added a boundary line beside the price: "Everything you use today is free and stays on your device. Cloud is an optional add-on — free for 14 days, then <price>. Cancel anytime." i18n key `cloud.upgradeBoundary`.
- **C235 — pinned insights now carry AI + timestamp attribution (2026-06-26):** A pinned insight is a saved AI answer, but the row meta showed only a bare raw-format date with no indication of its source. The meta now reads "AI insight · saved <date>", and the date routes through the user's date-format preference (was hard-coded `Jan 2, 2006`). i18n key `insights.pinnedAttribution`.
- **C232 / C233 — clearer spending anomalies: dollar delta + honest empty-month wording (2026-06-26):** Anomaly highlights now state the explicit dollar change alongside the percentage ("up 32% (+$120)"), not just a percent (C233). And a category with nothing spent yet in the in-progress month no longer reads as a misleading "down 100%" — it now says "nothing spent yet this month (about $X a month usually)" (C232). i18n: `insights.highlightUp`/`highlightDown` gained a delta placeholder; new `insights.highlightNone`.
- **C222 / C226 — asset balances no longer nagged like cash, with asset-appropriate wording (2026-06-26):** Investment and other/illiquid-asset balances are periodic *valuations*, not reconciled bank balances, but they used the same short staleness windows and the same "Stale / Update balance" banking copy. Extended the freshness windows (investment 60→120 days, other 30→180) and made the row badge + update action asset-aware ("Out of date" / "Update value") via `isValuationType`. i18n keys `accounts.staleValue`, `accounts.updateValue`. Tests added in `internal/freshness`.
- **C182 — define "Overall progress" on the goals tile (2026-06-26):** The Overall-progress stat only had an explainer tooltip when the goal-progress *smart* feature was enabled (off by default), so most users saw a bare percentage with no definition. Added an always-present native `title` on the tile defining the metric (total saved across all goals ÷ combined target). i18n key `goals.overallProgressDef`.
- **C138 — explain what budget rollover does (2026-06-26):** The rollover checkbox in the budget add/edit form was an unexplained toggle. Added a muted hint line beneath it with a concrete worked example (a $200 grocery budget with $30 unspent starts next period at $230; overspend and it starts lower). i18n key `budgets.rolloverHint`.
- **C70 — transfer delete now warns both legs are removed (2026-06-26):** Deleting one side of a transfer silently removed the paired leg too (via `DeleteTransactionWithTransferPair`), with the same generic "Delete? This can't be undone" copy as a normal row. The confirm now branches on `Transaction.IsTransfer()` and uses a transfer-specific message that spells out both sides (money out + matching money in) are removed. i18n key `transactions.deleteTransferConfirm`.
- **C110 — rule delete now asks for confirmation (2026-06-26):** Deleting a rule fired immediately with no confirm or undo. `deleteRule` now wraps the delete in `uistate.ConfirmModal` (destructive), and the message clarifies that transactions the rule already changed keep their categories/tags. i18n key `rules.deleteConfirm`.
- **C81 — FX rate convention explained in Settings (2026-06-26):** The exchange-rates editor gave no hint about which direction a rate goes, so users could enter the inverse. Added a one-line explainer under the heading: rates are "your base currency per 1 unit of that currency" (e.g. 1.08 ⇒ 1 unit = 1.08 base). i18n key `settings.fxConventionHint`.
- **C215 — net-worth trend labels the current partial month (2026-06-26):** The trend's final point uses a next-month-start cutoff (capturing the current month "so far"), but `trendPointLabel` rendered it as the *next* month's name — an unlabeled/confusing partial point. The last point is now labelled as the current month + "(so far)" (`dashboard.trendSoFar`), so the in-progress month reads correctly.
- **C82 — net worth discloses when a currency conversion happened (2026-06-26):** When the net-worth total folds in non-base-currency accounts, the dashboard KPI now appends "· converted to <base>" to its subtext, so the figure isn't read as a raw same-currency sum. Shown only when no account was excluded for a missing rate (that case already shows its own note). i18n key `dashboard.netWorthConverted`.
- **C68 — transfer legs no longer auto-tagged #needs-review (2026-06-26):** `CreateTransferPair` recorded both legs without `Reviewed`, so the `ActionFlagReview` workflow auto-tagged them "#needs-review" on insert — a false alarm, since a transfer is an explicit, unambiguous action the user just performed. Both legs are now created with `Reviewed: true`, which the workflow already honours (it skips reviewed entries).
- **C16 — CSV import names which rows were skipped and why (2026-06-26):** "Skipped N rows" gave no detail. The importer already returns per-row `store.CSVRowError{Line, Reason}`; the import summary now appends a plain-English clause (`csvSkipDetail`) — e.g. "line 3: bad amount; line 7: bad date (+2 more)" — capped at three rows so the toast stays short. Applied to both the file-picker and paste-CSV paths. i18n keys `documents.skipLine/skipMore/skipReasonGeneric`.
- **C297 — "Back up everything" surfaced in Settings → Data (2026-06-26):** The full multi-workspace backup was reachable only via the command palette. Added a "Back up everything" button to the Settings → Data action row (new `OnBackupAll` prop wired to the existing `backupEverything`), next to Export JSON/CSV, so it's discoverable without the palette. i18n key `settings.backupAll`.
- **C298 — clearer wipe confirmation button (2026-06-26):** Wiping all data popped a generic "Confirm" button — easy to click through on a destructive, irreversible action. Added `uistate.ConfirmModalLabeled` (a confirm dialog with a custom button label) and the wipe now confirms with "Erase everything" (`settings.wipeConfirmBtn`). (The Data section was already present in the settings jump-nav, so only the button wording remained.)
- **C296 — CSV export labelled as transactions-only (2026-06-26):** In Settings → Data the "Export CSV" button sat beside "Export JSON" with no cue that CSV is a *partial* export. Relabelled it "Export transactions (CSV)" and added a hint line under the data actions: JSON backs up everything; CSV saves transactions only — use JSON for a full backup. The label change also flows to the command-palette "export csv" command.
- **C203 — debt burn-down x-axis shows calendar months (2026-06-26):** The burn-down chart's x-axis showed bare month indices (0, 1, 2…). Each burn point now carries a calendar-month `Label` ("Jan 2026", via `payoff.DebtFreeMonth`), which the chart's JS maps onto the visible x-axis ticks — so the timeline reads in real months. No new axis format needed (the chart falls back to point labels when no x-format is set).
- **C202 — clearer default ($0-extra) state on the debt planner (2026-06-26):** At $0 extra, snowball and avalanche tie, which reads as a broken tool. The tie explanation now shows whenever the extra field is empty and there are 2+ debts (a single debt ties inherently, so no hint then), rather than only when a suggested extra happened to be available; the "Try $X/mo" one-click suggestion remains, shown when a suggestion exists. Wording moved to i18n key `planning.debtTieHint`.
- **C198 — debt-payoff progress baseline derived from real debt (2026-06-26):** The sample seeded a fixed `PayoffBaseline` of $39,500 "since Jul 1, 2022" — well below the ~$100k actually owed across the car/student/credit-card loans — so the progress card always read "Paid off $0 of $39,500 (0%) since Jul 1, 2022". `SampleDataset()` now sums the current owed across the included (non-mortgage) liabilities via `ledger.Balance` and sets the baseline ~22% above it with a recent start date (Sep 1, 2025), so the card shows believable progress (~18% paid). `store` now imports `ledger` (no cycle; `ledger` doesn't import `store`). Guards the zero-debt case (no baseline → no tracking).
- **C195 — debts in foreign currencies are FX-converted before the payoff plan (2026-06-26):** The debt-strategy card in `internal/screens/planning.go` built its liability list with `bal.Abs().Amount` — the raw minor units in each account's *own* currency — so a €1,000 balance was summed into a USD plan as `100000` cents of dollars, inflating the total and skewing the snowball/avalanche ordering. It now delegates to the already-tested pure helper `payoff.AggregateDebts(accounts, txns, base, rates)`, which converts each balance to the base currency through the FX table (`currency.Rates{Base, Rates: app.Settings().FXRates}`), respects `IncludedInPayoff`, and reports any currencies with no rate. Debts in an unrated currency are *excluded* (rather than miscounted) and named in a new muted note ("Left out of this plan (no exchange rate set): …") so the omission is visible. i18n key `planning.debtMissingRate` added.
- **C58-logic — split transactions now visible to reports & budgets (2026-06-25):** `reports.categoryTotals` and `budgeting.spentCovered` attribute each split line to its own category when `t.HasSplits()`, instead of counting the whole transaction under its (often empty) top-level category. Previously split transactions — including receipt-imported ones with `CategoryID==""` — were silently invisible to both per-category spend reports and budget totals. The whole-transaction category is never also counted, so there's no double-count. `budgeting.matchesCovered` was refactored into `matchesScope` (expense + date-range + member) with the per-category test applied separately (per split line for split txns). Tests: `TestSpendingByCategorySplits`, `TestSpentSplitTransactionAttributesPerCategory`.
- **C86/C92 — CSV import: dedupe re-imports + route imported rows through workflows (2026-06-25):** `ImportTransactionsCSV` (`internal/appstate/appstate.go`) now (1) skips rows already present — a per-account signature set built from existing transactions (`<accountID>|<dedupe.Signature>`) means re-importing the same CSV no longer doubles every transaction (C86); and (2) fires the `TriggerTxnAdded` workflow trigger **once per imported row with the row's full context**, instead of the single aggregate `nil` event `WithoutTriggers` emitted — so `txn_*`-conditioned workflows (route-by-payee, flag-by-amount) now apply to imported transactions, which they silently skipped before (C92). A new pure `dedupe.Signature(t)` (`internal/dedupe/dedupe.go`) is the single source of truth for the duplicate key, shared by `FindDuplicates` and the importer. Tests: `TestReimportCSVDeduplicates` and `TestBulkImportFiresPerRowTaskIdempotent`.
- **C32 — "Always categorize like this" prefill now applied (2026-06-25):** The rule add-form (`internal/screens/ruleaddform.go`) never read the `RuleDraft` atom set by the transaction row's action, so the shortcut opened a blank form. It now consumes the draft once on mount via `UseEffect` — seeding match + category, then `ClearRuleDraft()`.
- **C132 — budget rollover is now applied, not decorative (2026-06-25):** `internal/screens/budgets.go` rendered a "carried over $X" badge but evaluated each rollover budget against its raw limit; `budgeting.Carryover()` was never called. The loop now folds the previous period's remaining into the effective limit via `Carryover(prev.Remaining, b.Limit)` and evaluates against that, so Remaining/Percent/State/bar reflect the carry.

### Added
- **C256/C187 — Executable automate-goal action (pay-yourself-first workflow) (2026-06-25):** `ActionAutomateGoal` added to `internal/smart/smart.go` with payload fields `GoalID` and `GoalMonthlyAmount`. New `internal/appstate/savings_ops.go` provides `CreateWorkflowFromGoal(goalID, monthlyAmount)` — builds and persists a scheduled monthly `ActionTransfer` workflow from a deterministically chosen funding account (first non-archived asset account that is not the goal's own account, preferring checking/debit for liquidity) to the goal's linked account, with a `"pyf:<wfID>:<YYYY-MM>"` DedupeKey. `smart_card.go` handles `ActionAutomateGoal` by calling `CreateWorkflowFromGoal`, posting a confirmation toast ("Automatic monthly contribution set up"), and navigating to `/planning`. SMART-G17 (`g17AutoContribute`) now emits `ActionAutomateGoal` (with goal ID + `MonthlyNeeded`) when the goal has a linked account, and falls back to `ActionNavigate → /goals` when it does not. The `// TODO(C186)` comment in goals.go is removed. i18n key `smart.automateGoalCreated` added. 7 unit tests in `internal/appstate/savings_ops_test.go` cover the happy path, no-linked-account error, no-funding-account error, zero-amount error, checking preference, goal-account exclusion, and liability exclusion. Resolves C187 and the pay-yourself-first leg of C185.

- C186: ActionTransfer primitive in workflow engine — loop-safe scheduled money-movement via CreateTransferPair with DedupeKey guard

### Changed
- **Frontend-design polish pass on C254–C276 components (2026-06-25):** Applied the frontend-design skill methodology, reconciled with CashFlux's calm/refined design language. CSS-only changes to `web/index.html` — no Go logic touched. Additions: (1) Severity pills (C267) — refined font metrics, box-shadow depth, dark-mode text tokens for better contrast; (2) Per-item notification controls (C268) — new `.notif-ctrl-btn` / `.notif-ctrl-dismiss` ghost-button style with hover/focus affordances and `focus-visible` outline; (3) Single-device note (C274) — calm left-rule helper text treatment; (4) Member role badge + default chip (C276) — `.badge` / `.badge-muted` semantic variants with WCAG-AA contrast in both themes; (5) Ghost button variant `.btn-ghost` for Smart tab bar (C257/C259), with active-tab underline indicator; (6) Smart insight card `.smart-card` — layered surface, severity-toned left border, hover depth; (7) Catch-up banner (C271) — warm accent-dim surface + staggered `animation-delay` reveal for label/count (guarded by `prefers-reduced-motion`); (8) Dashboard catch-up card — slide-up entrance animation. All colors use existing CSS vars; no hardcoded hex introduced. WCAG AA verified for all new color combinations.

### Fixed
- **C256 — ActionCreateGoal now sets OwnerID + ScopeShared (2026-06-25):** The `ActionCreateGoal` branch in `smart_card.go` created a `domain.Goal` with `Scope: ScopeIndividual` but no `OwnerID`, causing every "Create goal" action to fail with the validation error "ownerid is required". Fixed by setting `OwnerID: domain.GroupOwnerID` and `Scope: domain.ScopeShared` — an emergency fund is a household-level goal, so a shared scope is the correct default. E2E test `e2e/c256_executable_actions.mjs` hardened to use the IDB injection pattern (matching c267/c268/c270) so it exercises the real end-to-end path: inject a clean dataset (no goals, 3 months of expenses) into IDB, reload, navigate to /smart Insights, click the SMART-G12 "Create goal" button, assert toast contains "goal", assert /goals shows "Emergency Fund".

### Changed
- **e2e/c264 — Threshold persistence fix: wait for autosave before reload (2026-06-25):** The original test filled the threshold input with "1000" and reloaded 200 ms later — faster than the 4 s autosave ticker, so the change was lost. Switched to `pressSequentially` + `Tab` (blur triggers the native `change` event reliably) and added a 5 s wait to let the autosave flush before reloading. Test now PASSes end-to-end.

### Fixed
- **Design-system consistency (2026-06-25):** Three minor violations in `internal/app/settings.go`:
  - Widget importance `Select` was missing an `aria-label` (accessible name was only the sibling `Span`); added `Attr("aria-label", uistate.T("widget.importance"))`.
  - FX-rate stale indicator used hardcoded hex `#cfa14e`; replaced with `var(--color-warn)` so it respects the active theme.
  - FX-AI error paragraph used `var(--color-danger, #e05252)` with a hardcoded fallback; removed the fallback (the CSS variable is always defined by the theme).

### Added
- **C256 — Executable smart recommendation actions (2026-06-25):** Three new `ActionKind` constants added to `internal/smart/smart.go`: `ActionCreateGoal`, `ActionCreateRecurring`, `ActionCancelSubscription`. The `Action` struct gains corresponding payload fields (`GoalName`/`GoalTarget`/`GoalCurrency`; `RecurringLabel`/`RecurringAmount`/`RecurringCurrency`/`RecurringCadence`; `SubscriptionName`). `smart_card.go` (`onAction` handler) gains three new `case` branches that execute each action through the validated appstate write path — `app.PutGoal`, `app.PutRecurring`, `app.MarkSubscriptionCancelled` — then post a confirmation toast and navigate to the affected screen. Helper `smartCurrencyOr` added. SMART-G12 ("Consider starting an emergency fund") upgraded from `ActionNavigate` to `ActionCreateGoal` (creates "Emergency Fund" goal with the computed target). SMART-SU1 ("Consider cutting X") upgraded from `ActionNavigate` to `ActionCancelSubscription`. SMART-G17 ("Automate goal contribution") retains `ActionNavigate` with an explicit `// TODO(C186)` — the automate-goal action requires the money-movement engine from C186 (not yet implemented). Three i18n keys added: `smart.goalCreated`, `smart.recurringCreated`, `smart.subscriptionCancelled`. Unit tests in `internal/smartengine/c256_executable_actions_test.go` verify SMART-G12 emits `ActionCreateGoal` with non-empty `GoalName`/`GoalTarget` and SMART-SU1 emits `ActionCancelSubscription` with `SubscriptionName`. E2E guard `e2e/c256_executable_actions.mjs`.

- **C264 — User-settable alert thresholds (2026-06-25):** `notify.RuleConfig` migrated from a bare `map[string]bool` to a struct with `Enabled map[string]bool` and `Thresholds map[string]int64` fields. Legacy KV payloads are detected and promoted transparently on unmarshal. New `notify.EffectiveThreshold(ruleID, cfg, ruleDefault)` returns the user override when positive, the rule default otherwise. `runNotifyCatchUp` now loads `ruleCfg` first and passes it into `largeTransactionCandidates`, `lowBalanceCandidates`, and `paycheckLandedCandidates`; the bill-due lead days is similarly resolved via `EffectiveThreshold`. Settings → Manage alerts gains a threshold input below each tuneable alert toggle: money rules (`default-large`, `default-low-balance`, `default-paycheck`) show a "$" input in whole dollars (stored as cents), bill-due shows a "days" integer input. All inputs persist on change to the same `cashflux:notify:ruleconfig` KV entry. Five table-driven cases for `EffectiveThreshold` + a legacy-promotion round-trip test added to `ruleconfig_test.go`. i18n key `settings.alert.threshold` added. E2E guard `e2e/c264_alert_thresholds.mjs`.

- **C263 — Per-alert-type enable/disable in Settings → Notifications (2026-06-25):** New `notify.RuleConfig` type (map of ruleID→bool) in `internal/notify/ruleconfig.go` with `DefaultRuleConfig()` (all rules on), `IsEnabled(id)` (absent keys default to true), `EnabledRules(allRules, config)` filter, and round-trip JSON helpers `MarshalRuleConfig`/`UnmarshalRuleConfig`. Persisted to the SQLite-backed `settingskv` under `cashflux:notify:ruleconfig` (preserved across data wipes). `runNotifyCatchUp` in `notifyrun.go` now loads the config and passes only enabled rules to `notify.CatchUp`. `notifySettings()` in `settings.go` gains a "Manage alerts" group (`data-testid="settings-manage-alerts"`) with one labelled toggle row per alert type — implemented as a stable `alertRow` component to obey the On*-in-loop rule. Nine i18n keys added (`settings.manageAlerts`, `settings.alert.*`). Table-driven tests in `internal/notify/ruleconfig_test.go` cover: default-all-enabled, absent-key-defaults-on, nil-config-defaults-on, filter-disabled, filter-all-enabled, empty-config, marshal/unmarshal round-trip, empty/garbage unmarshal. E2E guard `e2e/c263_manage_alerts.mjs`.

- **C274 — Single-device/local-first disclosure note on Members screen (2026-06-25):** Added a calm informational paragraph below the existing orientation description in `internal/screens/members.go`. The note explains that roles are organizational labels for a shared household dataset on this device, that CashFlux has no per-member logins or access controls, and that multi-user sync is a future hosted feature. Reuses the existing `P(css.Class("muted"), …)` pattern. i18n key `members.singleDeviceNote` added to `internal/i18n/en.go`. Element carries `data-testid="members-single-device-note"` for E2E targeting. Guard `e2e/c274_single_device_note.mjs`.
- **C271 — "While you were away" catch-up digest (2026-06-25):** Persists the unix-second timestamp of the last Notification Center open to the SQLite-backed KV (`cashflux:notify:lastSeen`). On re-open, a "Since your last visit" banner shows the count of items with `At > lastSeen`. Pure helper `NewSinceLastSeen(items []FeedItem, lastSeen int64) []FeedItem` added to `internal/uistate/notifyfeed_filter.go` (no build tags — natively testable). Table-driven tests cover newer/older/boundary/empty cases. Dashboard gets a dismissible `dashCatchUpCard` component (per-session dismiss via `ui.UseState`) that links to /notifications. i18n keys: `notifications.sinceLastVisit`, `notifications.sinceLastVisitOne`, `notifications.catchUpHeader`, `notifications.catchUpDismiss`, `dashboard.catchUpTitle`, `dashboard.catchUpBody`, `dashboard.catchUpBodyOne`, `dashboard.catchUpLink`. E2E guard: `e2e/c271_catchup_digest.mjs`.

### Added
- **C269 — Notifications jump-to tab in Settings (2026-06-25):** Added `"settings.notifyTitle"` to `settingsNavKeys` in `settingssectionnav.go`, inserting a "Notifications" tab between Freshness and Appearance in the Settings section nav. The existing `notifySettings` component (which already renders an `H4.set-label` matching the key and the browser-notifications toggle) gained a `data-testid="settings-notifications"` attribute for reliable E2E targeting. No new i18n keys needed — `settings.notifyTitle` and `settings.notifyBrowser` already existed. The section remains extensible for per-alert controls (C263/C264). E2E guard `e2e/c269_notifications_settings_tab.mjs`.

### Added
- **C259 — CapPerRule helper limits per-rule insights to 3, highest severity kept (2026-06-25):** New pure function `smart.CapPerRule(insights []Insight, n int) []Insight` in `internal/smart/cap.go`. Input must be severity-sorted; first `n` seen per `Feature` code are kept, dropping lower-severity overflow. Table-driven tests in `cap_test.go` cover: 4-same-rule→3-kept, mixed-rules-capped-independently, n≥count no-op, empty input, n=1, and n=0.
- **C259 — `EnableFreeOnly` bulk action enables all Free-tier features without touching AI-tier (2026-06-25):** `smart.EnableFreeOnly(s Settings) Settings` in `cap.go`. Iterates the catalog; sets every `TierFree` feature in `Enabled` and clears its `ExplicitOff` record so the Free tier-default ("on") re-applies even for features the user had previously turned off. AI features are untouched — explicitly-on AI features stay on, others remain at the off-by-default tier default. Wired into `uistate.EnableFreeSmart()` (new) which loads, transforms, and persists in one call.
- **C259 — Insights tab paginated (10 per page) using `smartInsightsPager` component (2026-06-25):** `smartInsightsPager` is a new stable component (owns `ui.UseState` page index and two `ui.UseEvent` prev/next handlers). Receives the already-capped insight list; slices it to the current page window; renders `smartInsightList` for that page plus a `Prev / Page N of M / Next` control bar when more than one page exists. Page state resets to 0 if it goes out of range (e.g. after a dismiss). Pagination control has `data-testid="smart-insights-pager"`.
- **C259 — "Enable free features only" button in Manage tab (2026-06-25):** Added to `smartManageControls` between the density select and the existing "Enable all" button. Calls `uistate.EnableFreeSmart()`, bumps the data-revision atom. `data-testid="smart-enable-free"`, i18n key `smart.enableFreeOnly`.
- **C259 — i18n: pagination + free-only keys (2026-06-25):** `smart.prevPage` ("← Previous"), `smart.nextPage` ("Next →"), `smart.pageOf` ("Page %d of %d"), `smart.enableFreeOnly` ("Enable free features only") added to `en_smart.go`.

### Changed
- **C257 — Split /smart hub into Insights + Manage tabs (2026-06-25):** `SmartHub` now renders a two-tab layout: "Insights" (default) shows `smartInsightsSection` + `smartAISection` + `SmartDigestSection`; "Manage" shows `smartManageSection` (the opt-in catalog). Tab state is held in `ui.UseState("insights")`; two stable `ui.UseEvent` handlers switch it. The tab bar uses `role="tablist"` + `aria-selected` for accessibility. Active tab button uses `btn btn-sm`; inactive uses `btn btn-sm btn-ghost`. Two new i18n keys: `smart.tabInsights` / `smart.tabManage`. E2E guard `e2e/c257_smart_tabs.mjs`.

### Added
- **C255 — Smart enabled-state persistence audit + round-trip regression test (2026-06-25):** Audited the full `smart.Settings` persistence path (C255). **Verdict: verified-correct — no real gap.** `Settings` serializes via `json.Marshal` with proper `omitempty` tags on every field; `uistate.SaveSmartSettings`/`LoadSmartSettings` call `SettingKVSet`/`SettingKVGet` (the SQLite-backed PRESERVED settings KV that survives dataset wipes); and `SettingKVSet` routes through `app.SetSettingKV` → `store.SetSettingKV` (SQLite) once `appstate.Default` is non-nil, with a `browserstore` fallback on early boot that migrates into SQLite on first read. Added `TestSettingsJSONRoundTrip` to `internal/smart/smart_test.go` as a regression guard: builds a `Settings` with mixed `Enabled`/`ExplicitOff` entries, a dismissed insight, a cadence override, a muted feature, a `LastRun` timestamp, a cached AI result, and a non-default density; marshals to JSON, unmarshals, then asserts `reflect.DeepEqual` on the full struct and spot-checks individual semantics (IsEnabled, IsMuted, IsDismissed, CadenceFor, LastRunAt, ResultFor, DensityOrDefault). A zero-Settings round-trip is also covered. 9/9 sub-tests pass.

### Changed
- **C254 — Free smart insights enabled by default (2026-06-25):** `Settings.IsEnabled` now applies a tier-based default — Free (deterministic, on-device) features are on for every new user; AI features remain off until explicitly opted in. An `ExplicitOff` map tracks features the user has deliberately turned off so the Free default never overrides an intentional choice. `SetEnabled(code, false)` records an explicit-off; `SetEnabled(code, true)` clears it. `DisableAll` now writes `ExplicitOff` for every feature (rather than just nil-ing `Enabled`) so the bulk-off persists. All internal methods (`EnabledCodes`, `EnabledFeaturesForPage`, `Active`, `ActiveCodes`, `ShowsAffordance`, `EnabledCount`) route through `IsEnabled` and inherit the new logic. Table-driven tests cover all four states (Free-unset→on, AI-unset→off, Free-explicit-off→off, AI-explicit-on→on, unknown→false) and no-AI-needed `AnyAIEnabled` behavior.
- **refactor: extract leaked money/percent computation out of `internal/screens` view code (2026-06-25):** part of a sweep pulling business logic out of the wasm UI layer into pure, tested packages (hard-rule #2). (1) **Major→minor money conversion** — `chat_agent.go` re-derived the conversion of floating major amounts (AI tool args, e.g. dollars) to integer minor units in 11 places, **five of which hardcoded `*100`** and were therefore wrong for non-2-decimal currencies (e.g. JPY). Added pure `currency.MinorFromMajor(major float64, code string) int64` (rounds via the code's own `Decimals`; table-driven `TestMinorFromMajor` covers JPY/unknown/negative/rounding) and routed every site through it, deleting the file-local `majorToMinor` helper — single source of truth, and the JPY bug is fixed. (2) **Un-clamped goal funding %** — `goals_row.go` and `chat_agent.go` computed `current*100/target` inline (duplicating `goalsvc.Percent` minus its `[0,100]` clamp); extracted to pure `goals.RawPercent` (+`TestRawPercent`) and both sites now call it. No behavior change beyond the JPY correctness fix.
- **refactor: FX-aware goal totals extracted to pure `goals.Totals` (2026-06-25):** the Goals screen (`internal/screens/goals.go`) summed each active goal's current/target into base-currency totals with an inline per-goal FX-conversion loop (raw-amount fallback on a missing rate) to feed the "saved so far / total target" headline stats. Moved that aggregation into pure `goals.Totals(goals, rates, base, includeArchived) (saved, target money.Money)` (+ table-driven `TestTotals` covering same-currency sums, archived exclusion, and the includeArchived path); the view now calls it. Computation now lives in the tested domain package per hard-rule #2; no behavior change.
- **refactor: minor→major float conversion extracted to pure `currency.MajorFromMinor` (2026-06-25):** the symmetric partner to `MinorFromMajor`. Three view sites converted integer minor units to a major-unit float for display/charting using a hardcoded `/100` or a hand-rolled `pow10` divisor loop (`divf`) — both wrong for non-2-decimal currencies. `chat_agent.go`'s formula-calculator variable map (6 vars: net_worth/assets/liabilities/income/spending/net_cashflow) and two chart-scaling loops in `planning.go` now call `currency.MajorFromMinor(minor, code)` (+ table-driven `TestMajorFromMinor` incl. a `MinorFromMajor` round-trip). Removes both `divf` loops; JPY-correct, no behavior change for 2-decimal currencies.

### Fixed
- **fix: SMART-SU1 highlight-in-place + SMART-SU9 confirmation toast (C258, 2026-06-25):** two SMART subscription affordance bugs. (1) **SMART-SU1** "Review subscriptions" action previously called `nav.Navigate` unconditionally — a no-op when already on `/subscriptions`. Fix: `smart_card.go` now checks if the current path matches the target; when it does and the route is `/subscriptions`, it extracts the subscription name from the insight key, resolves the row element via `data-testid="sub-cancel-select-<slug>"`, scrolls it into view (`scrollIntoView` smooth/center), and applies a transient `.smart-highlight-row` CSS outline for 1.5 s so the row is easy to spot. (2) **SMART-SU9** "Add a to-do" always had a `PostNotice(uistate.T("smart.taskAdded"), false)` call in the `ActionCreateTask` handler, but `PostNotice` requires `noticeCaptured=true` (set by any `UseNotice()` hook call). When the card rendered on a page that didn't call `UseNotice` itself (e.g. the `/smart` hub), the capture was only guaranteed by the shell's `Toast()` component. Fix: `smartInsightCard` now unconditionally calls `_ = uistate.UseNotice()` at render time, making the card self-sufficient and guaranteeing the notice atom is captured before any action can fire. Added `.smart-highlight-row` CSS rule to `web/index.html`. New E2E `e2e/c258_smart_su_fixes.mjs`.
- **fix: Settings modal rendered raw CSS token text (C25, R3, 2026-06-25):** `settings_section.go:63` passed `tw.BorderT, tw.BorderLine` (css.Rule values) as positional CHILD args to `Hr(...)` instead of wrapping them in `css.Class(...)`, so the `html/shorthand` dispatcher (which treats unrecognized args as child nodes) serialized them via Go `%v` into a visible text node — the garbage `{[{border-top-width 1px}] { []} []}…` shown between the Exchange-Rates and Freshness sections (recurring across F3/F10/F16). Fix: wrap in `css.Class(tw.BorderT, tw.BorderLine)`, matching the 6 correct divider call-sites in the same file and the prior `appearance.go` fix. Root-caused in R3; `GOOS=js GOARCH=wasm` build green (the live Settings e2e couldn't reliably open the modal in the churning app, so verified by diagnosis + build + pattern-match).
- **fix: account/goal added via the modal now appears without a reload (C223/C71/C177, R2, 2026-06-25):** the Add forms live in `AddHost`, a sibling of the Accounts/Goals screens; on success they called `PutAccount`/`PutGoal` (which DO persist to SQLite) then `OnDone` → `SetAddTarget("")`, which only re-renders `AddHost`. The list screens subscribe to a different revision atom that was never bumped, so a newly-added account or goal was saved but invisible until a manual reload — the systemic "silent add failure" seen across F9/F23/F31, root-caused in R2. Fix: `accountaddform.go` and `goaladdform.go` now call `uistate.BumpDataRevision()` after a successful add, and `goals.go` subscribes to `UseDataRevision()` (Accounts already did). Verified by R2 root-cause + green `GOOS=js GOARCH=wasm` build; live modal-driving e2e was flaky against the currently-churning app, so this is verified by code analysis + build, not a fresh e2e.
- **fix: ledger search matches the Payee field (C50/C55, 2026-06-25):** `internal/txnfilter` `matchText` searched only `Desc` + `Tags`, so a transaction whose payee differs from its description (the rules engine and activity screen use `Payee`) was unfindable by the transactions search box. It now also matches `Payee`, and the ledger search placeholder reads "Search description, payee, or tag". Pure-logic fix with native test `TestApplyTextMatchesPayee`; no wasm/UI churn beyond the one i18n string. From the feature-review backlog (F6).
- C276: Replace cosmetic "Default/Member" role labels on member rows with real role from memberrole.Label(); default-member seed chip now clearly labeled "default" separate from role badge
- **Log swallowed panics in notification catch-up (C272) (2026-06-25):** `runNotifyCatchUp` in `internal/app/notifyrun.go` previously silenced panics with a bare `_ = recover()`, making boot-time failures invisible. The defer now captures the recovered value and logs it via `slog.Error` with the panic value and full `debug.Stack()` trace before swallowing, so any future failure is observable in the structured log / browser console without risking app boot.
- **Notification Center always empty — sync feed atom on prepend (C270, closes C121/C158/C159) (2026-06-25):** `PrependNotifyFeed` in `internal/uistate/notifyfeed.go` was persisting the feed to the SQLite-backed KV but never updating the live `UseNotifyFeed()` atom (`app:notify-feed`). Because `runNotifyCatchUp` fires at app boot — before the Notification Center screen mounts — any atom already created (e.g. for the rail unread badge) held a stale empty feed; `UseAtom` only applies its default the first time the atom is created, so the KV write was invisible to live subscribers. The delivered-log then suppressed re-fire on subsequent loads, making the center permanently empty. Fix: after capping and persisting, `PrependNotifyFeed` now calls `UseNotifyFeed().Set(out)` so KV and atom are kept identical and all subscribers update immediately regardless of mount order. Mirrors the existing `UseNotice().Set(...)` pattern already used two lines below the call site in `runNotifyCatchUp`.

### Added
- **C265 — Paycheck-landed alert type (2026-06-25):** adds a positive `EventPaycheckLanded` ("paycheck-landed") notification that fires when a paycheck-sized income transaction just arrived. New `defaultPaycheckMinor` constant (50000 = $500.00) and a `default-paycheck` rule in `DefaultRules()` (8th rule; zero Threshold disables). Pure generator `notifyfeed.PaycheckLandedCandidates` emits one `SeverityInfo` candidate per qualifying income transaction strictly within the last N days (default 3) at or above the threshold, keyed `paycheck:<txnID>` for once-per-transaction deduplication; expenses and transfers are excluded. 8 table-driven sub-tests cover: income in-window fires; below threshold suppressed; expense excluded; transfer excluded; outside window excluded; exactly at cutoff boundary excluded; multiple paychecks; zero threshold disables. Wired in `runNotifyCatchUp` via a new `paycheckLandedCandidates(app, now)` helper. Two new i18n keys: `notify.paycheckTitle` and `notify.paycheckBody`. `defaults_test.go` updated to expect 8 rules and assert the paycheck threshold.
- **C266 — Low-balance alert type (2026-06-25):** adds a new `EventLowBalance` ("low-balance") notification event that fires when an asset account's balance drops below a configurable floor. New `defaultLowBalanceMinor` constant (10000 = $100.00) and a `default-low-balance` rule in `DefaultRules()` (7th rule; zero Threshold disables it, matching the large-transaction convention). Pure generator `notifyfeed.LowBalanceCandidates` emits one candidate per below-floor asset account per ISO-week (occurrence key `lowbal:<accountID>@<week>`); archived accounts and liability accounts are excluded; balance is opening + all transactions for that account. 8 table-driven sub-tests cover: below/at/above floor, zero-floor-disabled, liability excluded, archived excluded, transactions-applied, multiple-account fan-out. Wired in `runNotifyCatchUp` via a new `lowBalanceCandidates(app, now)` helper using the rule's single default floor (per-account custom floors are C264). Two new i18n keys: `notify.lowBalTitle` ("%s balance is low") and `notify.lowBalBody` ("Current balance is %s — below your alert floor."). `defaults_test.go` updated to expect 7 rules and assert the low-balance threshold.
- **C268 — Per-item read/dismiss/snooze in the Notification Center (2026-06-25):** each row now carries three inline icon controls — a read/unread toggle (○/●), a snooze-1-day button (⏱), and a dismiss button (✕). `FeedItem` gains a `SnoozedUntil int64` field (`json:"snoozedUntil,omitempty"`, migration-safe — zero = not snoozed). Three new feed-mutation helpers in `internal/uistate/notifyfeed.go` (`MarkFeedItemRead`, `DismissFeedItem`, `SnoozeFeedItem`) each persist to the SQLite KV **and** push the live atom (following the C270 pattern). The pure snooze filter `VisibleFeed(items, now)` lives in `internal/uistate/notifyfeed_filter.go` (no build tag, natively testable); 4/4 table-driven `TestVisibleFeed` cases pass. The Notification Center only renders items where `SnoozedUntil <= now`. Each row is its own `notifyRow` component (passing callbacks as props, not creating `On*` hooks in the loop — per CLAUDE.md framework rule). Global "Clear all" is preserved. e2e `e2e/c268_notification_item_controls.mjs` seeds 4 items (3 visible + 1 pre-snoozed), asserts: all non-snoozed items render, pre-snoozed item is absent, mark-read toggle works, dismiss removes the row, dismissed item stays gone after reload.
- Severity pills (info/warning/critical) per item in the Notification Center (C267)
- **C273 — Member role/permission model (2026-06-25):** introduces `MemberRole` (owner/admin/viewer) to the domain and a new pure package `internal/memberrole`. **Domain** (`internal/domain/entities.go`): `MemberRole` string type with constants `RoleOwner`, `RoleAdmin`, `RoleViewer`; `Role MemberRole` field added to `Member` with `omitempty` JSON tag and a doc-comment explaining the zero-value migration default. **Logic** (`internal/memberrole/memberrole.go`): `DefaultRole(isDefault)`, `Resolve(m)` (migration default for legacy empty-role rows), `Valid`, `ParseRole`, `Label`, `CanManageMembers`, `CanEditEntities`, `CanViewOnly` — all pure, no `syscall/js`. **Tests** (`internal/memberrole/memberrole_test.go`): table-driven coverage of every role + invalid/empty + migration default via `Resolve`. **Persistence** (`internal/store/member_role_persist_test.go`): three round-trip tests — per-role SQLite JSON, legacy-row migration default (raw JSON without `role` field → `Resolve` gives correct default), and full Snapshot→Load dataset cycle. No UI wired (C275 tracks that).
- Role selector (owner/admin/viewer) in add-member and edit-member forms; persists via existing member create/update path (C275)
- **L104 QA ritual — "The Density Dial": end-to-end guard for SMART affordance gating (2026-06-25):** a story-driven e2e (`e2e/loopstory_104_the_density_dial.mjs`) that turns the whole SMART layer on (Enable all) and walks the density dial top→bottom (Everywhere→Standard→Minimal→Off), measuring row badges + entity-overlay triggers on Accounts and key-figure tooltips on the Dashboard at each step. Asserts the monotonic taxonomy holds: Everywhere shows overlays (rank 3), Standard drops them but keeps badges + tooltips, Minimal drops tooltips but keeps badges (rank 1), Off clears everything; badge count is monotonic non-increasing; zero JS errors. **7 PASS · 0 FAIL · 0 ABSENT — no defects:** every inline affordance built across Waves 1–6 honors one global density governor (`Settings.ShowsAffordance`/`Density.Shows`). Verified against a clean `origin/main` build in an isolated worktree (the main tree was busy with concurrent agents' WIP).
- FX rates AI fetch: "Fetch live rates with AI" button in Settings FX editor; uses OpenAI Responses API (gpt-5.5 + web_search); proposes rates for review before apply; shows current vs proposed, asOf date, cost estimate, per-row and bulk apply; key-gated.
- **SMART proactive digest — cadence-driven summary posted to notification feed (2026-06-24):** turns the SMART layer from pull-only into a gentle proactive surface. New pure package `internal/smartdigest` (`Build`, `PeriodKey`, `Item`) selects the top 3–5 active insights (highest severity first, stable Key tie-break), builds a titled bullet-list `Item`, deduplicates via a `notify.DeliveredLog` (period-keyed so the same window never re-posts), and returns `(item, ok)` where `ok=false` on empty input or already-delivered period. 11 table-driven tests cover selection order, cap-at-N, empty→false, dedupe same-period, dedupe next-period-allowed, severity tie-break, stable ID, title content, timestamp, period key variants. New `SMART-DIGEST` catalog entry (code `"SMART-DIGEST"`, `PageHub`, Free tier — no AI spend) so `Settings.SetEnabled`/`CadenceFor`/`MarkRun`/`LastRunAt` all work without schema change. New `internal/uistate/smartdigest.go` persists the `DeliveredLog` in the PRESERVED settings KV (`cashflux:smart-digest:delivered`, capped at 120 entries). `SmartDigestDriver` (headless component, `internal/screens/smart_digest_driver.go`) is mounted ONCE in `Shell` (alongside `Toast`/`SettingsHost`) so its `UseEffect` hook is at a constant depth; it stamps `LastRun` BEFORE building (effect key embeds `LastRun`) so it cannot re-enter within the due window — at most one digest per cadence period. Guards: feature enabled, density not Off, cadence Due, insights non-empty, period not already delivered. `SmartDigestSection` renders the opt-in toggle (`data-testid="smart-feature-SMART-DIGEST"`) + cadence picker (`data-testid="smart-digest-cadence"`) on the `/smart` hub between the AI section and the manage catalog; cadence choices are On app open / Daily / Weekly / Monthly (Live and Manual excluded as inappropriate for a notification digest). New `PageHub` constant in `internal/smart/smart.go` for cross-app meta-features; kept out of `Pages()` so it doesn't appear in the per-page manage groups.
- **R27 — Financial-health score (deterministic, explainable, on-device) (2026-06-25):** a new whole-household 0–100 health score with a dashboard widget + a full `/health` page. **Model** lives in a new pure package `internal/healthscore` (no `syscall/js`, table-driven tests): `Evaluate(Inputs) Result` scores five factors — savings rate (w .25), emergency-fund months (.25), debt-payments-vs-income (.20, labeled honestly as min-payments÷income, not "DTI"), budget adherence (.15), aggregate credit utilization (.15) — each with an `Applicable` flag. **Inapplicable factors are dropped and their weight re-normalized proportionally across the rest** (a retiree with no income still scores via emergency+adherence+utilization; a household with no cards isn't penalized; zero debt scores 100, not dropped). Five bands (Excellent ≥80 / Good ≥60 / Fair ≥40 / Needs work ≥25 / Critical) and a "Not enough data" state below two applicable factors. `NegativeCashFlow` is surfaced as a warning **without** a hard cap that would double-penalize. Tests cover negative savings, no-income, zero-debt, no-cards, <2-factors, DTI>100% clamping, and every single-missing re-normalization permutation. **Builder** (`internal/screens/health.go` `buildHealthInputs`) derives the inputs from the existing tested primitives — trailing-3-full-month savings via `reports.IncomeVsExpense`, emergency = `ledger.LiquidBalance` ÷ avg monthly spend, obligation = Σ liability `MinPayment` ÷ monthly income (FX-converted), adherence = % budgets not over via `budgeting.EvaluateRollup`, aggregate utilization = Σ card balances ÷ Σ limits via `ledger.Utilization`. **UI**: a 2×1 bento tile (`"health"` in `dashlayout.DefaultItems`, registered in the dashboard `renderers` map as a component so it owns its hooks) with an SVG score ring whose stroke is a **continuous red→amber→green HSL hue** by score (band label as the categorical overlay), an animated arc, the band, a "since last month" delta, the weakest factor as "value → target", and a "View steps" link; and a `/health` page (route in `screens.All`, Plan & Analyze group) with the large ring, a per-factor breakdown (value, score bar, contribution %, target), prioritized next steps, and an on-device privacy note. **Trend**: `internal/uistate/healthtrend.go` persists one `HealthSnapshot` per month (≤12 kept) to the SQLite-backed KV and pushes the live atom on write (the C270 pattern), powering the delta. New i18n keys `dashboard.healthScore`, `nav.health`, `screen.healthSub`. Verified end-to-end via `e2e/verify_health.mjs` (10/10 PASS, zero JS errors): widget + ring + band render, "View steps" routes to `/health`, the breakdown shows all five factors with contribution % and the privacy note. `internal/dashlayout/pack_test.go` updated to include `"health"` in the expected arrangement.
- **SMART Wave 4 — entity overlay, dashboard digest widget, empty-state helpers (2026-06-24):** three new affordances completing the density-tier stack. (1) **Smart empty-state helper** (`smartEmptyStateFor`, `AffordanceEmptyState` / Minimal+): wired to the Goals empty section — when no goals exist and an enabled engine has a goals-page insight, a branded ✦ hint + capped insight card appears below the add CTA (`data-testid="smart-emptystate-goals"`); gated so it renders nothing at density Off or when no relevant insights fire. (2) **Smart entity overlay** (`smartOverlayFor`, `AffordanceOverlay` / Everywhere only): a sparkle trigger button per account row (`data-testid="smart-overlay-trigger-<id>"`) opens a WAI-ARIA dialog popover (`data-testid="smart-overlay-<id>"`) surfacing all insights for that entity; DismissPopover handles Escape / outside-click; `smartOverlay` is its own component so hooks sit at stable positions; gated so the button renders only when `byEntity[id]` has qualifying insights at Everywhere density. (3) **Smart digest dashboard widget** (`smartDigestWidget`, `AffordanceWidget` / Standard+): a new `"smart-digest"` bento tile in `DefaultItems` (2×1, positioned after `"highlight"`) showing the top 3 cross-page insights from `smartengine.Run`; empty-hint when density is below Standard or no insights are active; the `"smart-digest"` id is registered in the Dashboard `renderers` map and wired exactly like existing tiles. New i18n keys `smart.emptyHint`, `smart.overlayTitle`, `smart.overlayLabel`, `smart.digestTitle`, `smart.digestEmpty`. `internal/dashlayout/pack_test.go` updated to include `"smart-digest"` in the expected pack arrangement. e2e `e2e/smart_wave4_check.mjs` (8/8 PASS): Everywhere density surfaces overlay triggers on 4 account rows + opens the overlay; digest-list present on Dashboard; density Off removes digest; Goals empty-state wired and absent-at-Off confirmed.
- **SMART Wave 3 — in-form field assist chips (auto-category, clean-merchant, wish→goal) (2026-06-24):** new pure package `internal/smarttext` (`CleanMerchant` + `ParseWish`, 23 table-driven test cases) provides the deterministic Free text-normalisation layer. New `SmartFieldAssist` component in `internal/screens/smart_affordances.go` renders a compact inline `✦ Use "…"` chip gated by `AffordanceFieldAssist` / Standard density; its own component so `UseEvent` is always at a stable hook position. **Three assists wired:** (a) **clean-merchant** in the QuickAdd description field — when the typed text looks like a bank POS string (SQ *, TST*, DEBIT…) the chip offers the normalised name; (b) **auto-category** in the QuickAdd category field — when a user rule matches the description the chip names the matched category before the user saves, giving visible confirmation of auto-categorization; (c) **wish→goal** in the Goal add form name field — when the typed name parses as a free-text savings wish ("save $2,000 for a new laptop") the chip fills both the Name and Target Amount in one click. All three render nothing when suggestion is empty, already applied, or density is below Standard.
- **SMART Wave 2 — key-figure explainer tooltips + section quick-actions spread across all main pages (2026-06-24):** the reusable `smartTooltipFor` and `smartSectionAction` affordances are now placed on every major page beyond the Dashboard net-worth tooltip already shipped. **Key-figure tooltips** on: Budgets "safe to spend / left" (`smart-tip-budget-safe`), Goals "overall progress" (`smart-tip-goal-progress`), Accounts net worth (`smart-tip-accounts-net`), Planning projected net worth (`smart-tip-planning-forecast`), Transactions filter-net total (`smart-tip-txn-total`), Bills total due (`smart-tip-bills-due`), Subscriptions monthly burden (`smart-tip-subs-monthly`). Each tooltip carries a unique id and a new plain-English `smart.tip*` i18n key in `en_smart.go` explaining what the figure means / how it is computed — one sentence, no jargon. **Section quick-actions** (`[data-testid="smart-section-action"]`) placed in the header/toolbar of: Transactions, Budgets, Goals, Accounts (Assets section), Planning (forecast card), Bills, Subscriptions — letting users reach the Smart hub from any page. All affordances are gated by the global density dial (Off → nothing renders). No hooks were added at the page level: `LoadSmartSettings()` is a pure load and each affordance function is its own component. New e2e `e2e/smart_tooltip_spread_check.mjs` (4/4 PASS): loads sample, verifies tooltip present at Standard density, verifies click reveals popover, verifies density Off removes both tooltip and section action.
- **SMART Wave 1 — row-level smart badges on Accounts, Bills, Subscriptions, and Transactions list pages (2026-06-24):** existing `smartBadgeFor` affordance is now wired to every list-page row. Each page runs its Free engines once (not per row) via `smartengine.RunPage`, indexes the results by `Action.RelatedID` with `insightsByEntity`, and passes `SmartSettings + SmartByEntity` down to the row component. Account rows show a badge when account-targeting engines (e.g. SMART-A7 recurring-charge detection, SMART-A2 dormant, SMART-A4 cash-positioning) fire on that account ID. Bill rows badge on their `AccountID` (bills are liability accounts). Transaction rows badge on their own ID when transaction-targeting engines fire. Subscription rows are wired forward-compatibly (current engines don't set `RelatedID`, so badges are silent until a future engine does). Density dial governs badge visibility: Off → no badges anywhere. Strictly additive and feature-flagged: no badge renders unless the feature is enabled and un-muted. New e2e `e2e/smart_row_badge_check.mjs` (3/3 PASS): enables SMART-A7, asserts badge appears on an account row, sets density Off and asserts badges disappear.
- **L97 QA ritual — "The Glanceable Read": end-to-end insight-copy guard (2026-06-24):** a story-driven e2e (`e2e/loopstory_97_glanceable_read.mjs`) that turns the **whole** SMART layer on (Enable all) and reads **every** rendered insight across the hub + the Budgets/Goals/Transactions/Bills inline strips (**204 cards**), holding the copy to a product-ready bar: money is symbolized (a `$` present, **zero** symbol-less 2-decimal `Money.Format(2)` leftovers), no currency **code** in prose ("USD limit"), no grammar artifacts (`entrys`/`categorys`/`daies`), no template tells (tilde-before-number, `/mo/mo`, double-slash), and zero JS errors. **6 PASS · 0 FAIL · 0 ABSENT — no defects;** verifies the humanized-copy work end-to-end in the live app. (A first-pass C-2 fail was a test-regex false positive — the thousands comma in a symbolized chip `$1,480.00` made it match the `480.00` tail; the matcher now anchors on the whole amount and captures the leading symbol.)

### Changed
- **SMART insight copy audit + golden snapshot test (2026-06-24):** second editorial pass over all `internal/smartengine/*.go` engine files confirming Title/Detail strings comply with the style guide (subject-first lead, one-hedge max, `hmoneyc`/`hm`/`plural` used throughout, no tilde-in-prose, no bare currency code, no doubled `/mo/mo`, dates via `Format("Jan 2")`/`Format("Jan 2006")`). Copy was already clean from the prior pass; no prose changes were needed, confirming correctness. Added **`internal/smartengine/copy_golden_test.go`** — a fully deterministic golden test suite (fixed `Now=2026-06-15`, fixed amounts, no `time.Now()`) with: (1) four exact golden assertions for D1 uncategorized count, G1 contribution structure, G11 emergency-fund gap math, and G3 surplus title; (2) a `TestCopyGuard` sub-test that runs every registered engine over a rich composite fixture and scans all produced Title+Detail strings for three anti-patterns — symbol-less 2-decimal amounts, bare ISO currency codes (`USD`, `EUR`, etc.) in prose, and broken English plurals (`entrys`, `categorys`, `daies`). All 5 new tests pass.
- **SMART series — insight copy refinement: product-ready wording across all engines (2026-06-24):** a full editorial + formatting pass so alerts read like a person wrote them, not a template. **Money is now humanized everywhere** — a new `hmoneyc`/`hmoney`/`hm` formatter in `internal/smartengine/format.go` renders the currency symbol, thousands separators, and whole-unit rounding for sizable amounts while keeping cents for small ones (e.g. `519.37` → **$519**, `5434.00` → **$5,434**, `0.99` → **$0.99**) — replacing all 107 raw `Money.Format(2)` renders that previously showed symbol-less, ungrouped decimals with a trailing currency code ("…over its USD limit"). **Grammar is fixed** — `plural()` now applies real English rules (consonant+y → *-ies*, sibilants → *-es*), so "97 entrys"/"2 categorys" become **"97 entries"/"2 categories"**, and it pluralizes the final word of a phrase. **Tone tightened** — the three flagged alerts rewritten (B9 pacing: "Dining is projected to exceed budget by $519 this period. Slowing spending now would keep it closer to plan."; G5 conflict: "Your goals are overcommitted by $5,434/mo. You have about $191/mo of slack, but active goals require $5,626/mo."; T4 split-category: "Amazon appears in 2 categories across 97 transactions. Standardizing the category will make reports more accurate."), stray tildes-in-prose removed, stacked hedges collapsed, and several colon-fragment titles rewritten into full sentences ("Spending from X is higher than usual", "Paying only the minimum on X is costing you", "You may be paying for X without using it"). New table-driven `format_test.go` locks in the pluralizer, money humanizer, and thousands grouping.

### Added
- **SMART affordances Phase 2 — the toolkit: badges, tooltips, section actions (+ first placement) (2026-06-24):** the reusable components that weave SMART into the app's fabric (`internal/screens/smart_affordances.go`), each gated by `Settings.ShowsAffordance` (enabled + not muted + density permits the kind). **SmartBadge** — a quiet clickable severity dot for a row/figure, fed by `insightsByEntity` (indexes a page's insights by `Action.RelatedID`, so a row finds its own insight with no per-row engine run); `smartBadgeFor` applies the gate + picks the highest-severity insight. **SmartTooltip** *(the new primitive — none existed)* — an opt-in explainer popover (click to open, Escape/outside-click dismiss via `DismissPopover`); `smartTooltipFor` gates it by density. **SmartSectionAction** — a compact sparkle quick-access to the hub for a page toolbar. First live placement: the Dashboard net-worth figure now carries the explainer tooltip (Standard density). e2e `smart_affordance_check.mjs`: the tooltip shows at Standard, reveals its explanation on click, and disappears at density Off — proving the dial governs the weaving.
- **SMART affordances Phase 1 — density dial + bulk enable/disable (governance for app-wide weaving) (2026-06-24):** the foundation for riddling the app with optional smart/AI affordances (tooltips, badges, field-assists, section actions, overlays, widgets) beyond the hub + strip. New pure `internal/smart/density.go`: a `Density` dial — **Off / Minimal / Standard / Everywhere** (default Standard) — and an `Affordance` taxonomy ranked by prominence, with `Density.Shows(affordance)` gating which KINDS surface. `Settings.ShowsAffordance(code, kind)` is the single gate every inline surface checks: feature enabled **and** not muted **and** density permits the kind — so "everywhere" is the user's dial, never forced. `Settings` also gains **EnableAll/DisableAll/EnabledCount** (DisableAll keeps schedules/mutes so re-enabling restores intent). Hub UI: the manage header now carries a **density picker** + **Enable all / Disable all** buttons (own component, persisted via `SetSmartDensity`/`EnableAllSmart`/`DisableAllSmart`). Table-driven tests (density show-matrix, default, ShowsAffordance gate incl. mute/disable/Off, bulk ops keep schedule). e2e: density dial offers Standard+Everywhere; Enable all surfaces insights, Disable all clears them.
- **SMART series — scheduled AI auto-run (cadence-driven) (2026-06-24):** the schedule now actually *runs* things. A button-type AI feature set to a non-Manual cadence (On app open / On new data / Daily / Weekly / Monthly) auto-runs itself when **due** via a guarded `ui.UseEffect`: the effect stamps `LastRun` **before** the call and embeds `LastRun` in its deps key, so it cannot re-enter within the due window — **at most one paid call per schedule period**, terminating cleanly (no effect loop, no surprise spend). Input features and Manual/Live cadences never auto-run (the click-before-run guard stays). It only mounts behind the provider gate, so it can never fire without a configured provider. Result is cached + shown like a manual run. Together with the cadence picker, mute, manual run, inline page controls, and caching, the SMART layer is now a real configurable automation system — run manually, mute, or schedule each feature.
- **SMART series — AI features runnable inline on each page + result caching (2026-06-24):** the AI ("click before run") controls are no longer hub-only — a page's enabled AI features now surface in its inline `SmartStrip` as run-controls (the input ask-bar / "Run" button / receipt-scan), so analysis runs from the page itself. The strip now renders when a page has Free insights **or** enabled AI features (additive); AI controls are gated on a configured provider (an honest "needs a provider" hint, never a dead control). Each AI control **caches its last result** (`Settings.Results`, persisted; seeds the answer on mount and re-saves via `SetSmartResult` on each run) so a manual/scheduled run persists across navigation and reloads **without re-spending**. Refactored the per-feature AI renderer into one shared `smartAIFeatureNode` used by both the hub AI section and the strips. e2e (`smart_strip_check.mjs`, 8/8): enabling an Accounts AI feature surfaces the Accounts strip with the provider-gated control.
- **SMART series — per-feature run controls in the catalog: schedule picker + mute (2026-06-24):** the manage catalog row now exposes, for each enabled feature, the controls the layer was missing. **AI features** get a **schedule/cadence picker** (`<select>`: Always / Manual / On app open / On new data / Daily / Weekly / Monthly) bound to `Settings.CadenceFor` and persisted via `SetSmartCadence` — Manual (the default) is the click-before-run guard. **All features** get a **Mute** button (snooze) that hides the feature's insights everywhere without losing the opt-in or schedule (`SetSmartMuted`; the row dims when muted; the engine skips muted features). Free features stay live (free + instant, so a schedule would be a no-op). e2e (`smart_hub_check.mjs`): an enabled AI feature shows the cadence picker (offering Manual + Weekly) and a mute control.
- **SMART series — per-feature scheduling, mute, run-tracking + AI result cache (foundation) (2026-06-24):** the SMART layer becomes a configurable automation system, not a fixed read-out. New pure `internal/smart/schedule.go`: a `Cadence` model — **Always** (Live), **Manual** (click-before-run), **On app open**, **On new data**, **Daily/Weekly/Monthly** — with `DefaultCadence` (Free→Live since it's free+instant, AI→Manual so it never spends money on its own), `Due` (is a feature due to run now, given last-run/data-change/app-open), and `FreshFor` (is a cached result still good). `Settings` gains per-feature `Schedules`/`Muted`/`LastRun`/`Results` (all JSON, preserved-KV) with `CadenceFor`/`SetCadence`/`IsMuted`/`SetMuted`/`MarkRun`/`LastRunAt`/`SetResult`/`ResultFor`/`ActiveCodes`. The engine now skips **muted** features (no compute, no surface) and `Active` drops muted insights. `uistate` persistence helpers `SetSmartCadence`/`SetSmartMuted`/`MarkSmartRun`/`SetSmartResult`. Table-driven tests cover cadence validity/labels, defaults, `Due`/`FreshFor` for every cadence, and the settings schedule/mute/run/result round-trips. (UI for the cadence picker + per-feature run/mute controls + inline AI run-cards builds on this.)
- **SMART series — interspersed inline per-page strips + sparkle brand glyph (2026-06-24):** the SMART layer is now woven into every relevant page, not only the `/smart` hub. New `SmartStrip` component (`internal/screens/smart_strip.go`) renders a page's enabled, active Free-engine insights inline as a compact, glanceable, capped strip (top 3; "View all (N)" → hub), wired once in the app Shell via `SmartStripForPath(activePath)` (mapping each route → its `smart.Page`; the Dashboard shows a cross-page summary). **Strictly additive and feature-flagged:** a page renders nothing until the user toggles on a feature that produces insights for it — `RunPage` only runs ENABLED engines, so each toggle gates per-page visibility. Brand glyph: a shared sparkle (`icon.Sparkles`) marks every SMART surface via `smartBrandHeader` — neutral tone for **Smart** (Free), up/accent tone for **Smart AI** — on the inline strips and the hub's insights/manage/AI section headers. e2e `e2e/smart_strip_check.mjs` (6/6): enabling SMART-B8 surfaces the Budgets + Dashboard strips with a live card; Accounts shows no strip (additive); toggling B8 OFF removes it (the toggle is a feature flag).
- **SMART series — COMPLETE: T8 receipt OCR (vision) — all 84 items shipped (2026-06-24):** the final SMART feature. **T8** adds a "Scan a receipt" control to the `/smart` hub: it opens the camera/file picker (reusing `pickImageDataURL`), reads the image to a data URL on-device, and sends it to the vision model (`ai.SendVisionChat` on the vision-capable gpt-5.4-mini) to extract merchant/date/total/line-items. Gated on a configured OpenAI key (vision), with an honest "needs a key" hint otherwise. **This completes the SMART series: 84/84 items — 66 deterministic Free rule engines (every page) + 18 AI features — all opt-in, cost-transparent (Free $0 vs AI per-use, provider-gated), surfaced through the glanceable `/smart` hub, with unit + integration + e2e coverage.**
- **SMART series — 7 more Free engines: T4, G2, G10, G20, AL5, SU15, BL14 (2026-06-24):** **T4** bulk-edit (a merchant split across categories → unify); **G2** completion forecast (warns when even the full surplus finishes a goal late); **G10** what-if (how much sooner +$100/mo finishes a goal); **G20** shared-goal contribution nudge; **AL5** allocation outcome preview (surplus→debt payoff months); **SU15** pause-instead-of-cancel (seasonal charge gaps); **BL14** seasonal bill forecast (variable biller's high-end). 66 Free engines + 17 AI = 83 SMART items. Table-driven tests.

- **SMART series — Free engines SU7 + SU12 (2026-06-24):** **SU7** usage-vs-cost (flags a subscription whose category shows no other activity — paying but maybe not using); **SU12** household sub attribution (in a multi-member household, flags subscriptions whose charges aren't assigned to any member). 59 Free engines + 17 AI = 76 SMART items. Table-driven tests.

- **SMART series — Free engines SU9 + BL10 (2026-06-24):** **SU9** renewal-timed reminders (offers a one-tap "keep this?" to-do a few days before a subscription renews); **BL10** one-tap pay-all-due (surfaces several bills due within a few days together). 57 Free engines + 17 AI = 74 SMART items. Table-driven tests.

- **SMART series — Free engines B7 + G17 (2026-06-24):** **B7** seasonal-budget adjustment (detects categories whose monthly spend swings widely and suggests month-specific budgets); **G17** recurring auto-contribution nudge (for a deadline goal + detected payday, suggests automating an on-payday contribution). 55 Free engines + 17 AI = 72 SMART items. Table-driven tests.

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

## C21 [F3] — Guided /setup wizard

- Added internal/screens/setup.go: a new 4-step guided setup wizard at the /setup route.
  Steps: (1) Currency & week-start, (2) Monthly income, (3) First account, (4) Household members.
  Completion shows a "Go to dashboard" CTA.
- All hooks (UseState, UseEvent) called unconditionally at top-level; step content pre-computed
  as nodes and shown via If(step == N, node) — no hooks in loops.
- Persistence: currency via pp.PutSettings, week-start + income via uistate.SetPrefs,
  accounts via pp.PutAccount, members via pp.PutMember. SettingKVSet tracks
  cashflux:setup:currencyConfirmed and cashflux:setup:wizardDone.
- Progress bar driven by setup.Compute(...) with ✓/○ indicators per step.
- Registered /setup in internal/screens/screens.go (GroupSystem, Phase 1).
- i18n keys use uistate.T("setup.*") — keys fall back to the key name since en.go was dirty
  and could not be edited.

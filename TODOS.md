# CashFlux — Master Feature Backlog

Single source of truth, **ordered top-to-bottom by implementation priority**. Work in order;
within a section earlier items unblock later ones. Build **bottom-up** per the SDLC rule
(data model → services/logic with tests → persistence → state → UI last). See [`SPEC.md`](./SPEC.md)
for product detail and [`CLAUDE.md`](./CLAUDE.md) for the rules.

**Legend:** `[ ]` todo · `[x]` done · `[~]` in progress · `(P#)` phase · `★` critical path.
**Discipline:** one feature per commit; update `CHANGELOG.md` + `DEVLOG.md` each commit; pure logic
packages have no `syscall/js` and ship with table-driven tests.

---

> Note: C-IDs are unique and continuous (C1–C329); R1–R34 = research/spec. Full evidence + fix
> detail for each is in the Claude Code task list; these are the durable one-line backlog entries.

### Review F1 — Frictionless signup / first-run (6/10)
- [ ] **C1 [MAJOR]** Sample banner permanently cleared after first reload (persist.go hydrateImport SetSampleActive(false)).
- [ ] **C2 [MAJOR]** Data-loss race: hero "Load sample data" + reload &lt;1s loses the sample (homehero.go; needs forced persist).
- [ ] **C3 [MINOR]** First-run auto-seed hides the "Load sample data" CTA (communicate demo mode via banner).
- [ ] **C4 [MINOR]** Sample-data banner low prominence. **C5 [MINOR]** ~~"Synced" pill on an empty workspace~~ **— FIXED (verified 2026-06-25):** `loadSyncStatus()`/`setSyncStatus()` defaulted an unset state to `"synced"`, so a local-only session (no cloud sync) showed a misleading "Synced" chip (and defeated SyncChip's "invisible until cloud sync" intent). Default is now `"local"` → SyncChip stays hidden + Settings shows "Saved on this device"; real cloud syncs still set `"synced"` explicitly. MEASURED live: sync-chip invisible, no standalone "Synced" text on a local session; `go test ./internal/app` ok, build rc=0, 0 errors. **C6 [MINOR]** ~~Missing &lt;meta name=description&gt;~~ **— DONE 2026-06-25:** added `<meta name="description">` + Open Graph (`og:title`/`og:description`/`og:type`/`og:site_name`) + Twitter card (`summary`/`title`/`description`) to `web/index.html`, copy mirrored from `manifest.webmanifest`; descriptive `<title>` ("CashFlux — Local-first household budgeting"; the SPA still sets per-route titles at runtime). (`og:image`/`twitter:image` later added once raster icons were generated — see C306.) MEASURED: `curl http://127.0.0.1:8099/` (raw HTML = exactly what share-crawlers read, no JS) returns all the new meta tags; app boots clean, 0 JS errors.
- [ ] **C7 [MINOR]** Add-account modal lacks first-run framing. **C8 [MINOR]** Empty dashboard renders full bento at $0, no hierarchy.

### Review F2 — Aggregation / import (4/10)
- [ ] **C9 [DESIGN]** No bank aggregation (local-first) — make the trade-off explicit.
- [ ] **C10 [MAJOR]** CSV confirm omits account. **C11 [MAJOR]** CSV history row lacks count+account. **C12 [MAJOR]** Draft account selector/Import below fold.
- [ ] **C13 [MAJOR]** Documents leads with AI-key-gated image import; key-less paths buried. **C14 [MAJOR]** No import entry from empty account/txn states.
- [ ] **C15 [MINOR]** Mapping wizard not pre-populated. **C16 [MINOR]** "Skipped N rows" no detail. **C17 [MINOR]** Dupes not per-row. **C18 [MINOR]** Remind-monthly no confirm.
- [ ] **C19 [DESIGN]** No "how to get your bank CSV" guidance. **C20 [DESIGN]** Vision import hard-gated behind user OpenAI key.

### Review F3 — Guided setup wizard (3/10)
- [ ] **C21 [MAJOR]** No guided setup wizard at all. **C22 [MAJOR]** Income setup absent from first-run. **C23 [MAJOR]** Base currency/week-start buried, no first-run visibility.
- [x] **C24 [MAJOR]** ~~No date-format preference~~ **— DONE (verified 2026-06-25):** Settings has a date-format `<select>` (`settings_section.go:165`) bound to `prefs.DateStyle` via `OnDateStyle` (`settings.go:528`); MEASURED live (Settings via household card): present with all 4 options "2026-06-05 (ISO) / 06/05/2026 (US) / 05/06/2026 (European) / Jun 5, 2026 (Long)", and the pref drives dates app-wide. **C25 [MAJOR]** ~~Settings raw CSS-token text (divider bug)~~ **— DONE (verified 2026-06-25):** walked all rendered text nodes in the open Settings panel (excl `<style>`/`<script>`) — **0** `--token:`/`var(--`/`color-mix(`/`:root{` leaks; 0 JS errors. **C26 [MAJOR]** Empty /accounts leads with "Load sample data"; add form only via unlabeled "+".
- [ ] **C27 [MAJOR]** Add-account modal no context/opening-balance help. **C28 [MAJOR]** Household-member setup not discoverable.
- [ ] **C29 [MINOR]** ~~/budget renders dashboard on empty store~~ **— DONE (verified 2026-06-25):** /budgets on an empty store renders the Budgets screen with a dedicated empty-state CTA (`EmptyStateCTA`, budgets.go:298 — "add your first budget"), not the dashboard. MEASURED live (empty store via Start fresh): /budgets shows h1 "Budgets" + the budget empty CTA, not net-worth/this-month dashboard content; 0 errors. **C30 [MINOR]** ~~Owner defaults "Group" with 0 members~~ **— DONE (verified 2026-06-25):** the owner picker only offered "Everyone/Group" until members exist, so a 0-member household saw a meaningless field defaulting to a group of no one. The owner field is now hidden in the account + budget add-forms until at least one member exists (owner stays `GroupOwnerID` = shared); it appears once members are added. MEASURED live: owner field shown with members (sample) and hidden on a 0-member household (after Start fresh); build rc=0, 0 errors. **C31 [DESIGN]** No setup checklist.

### Review F4 — Self-learning auto-categorization (5/10)
- [x] **C32 [BLOCKER]** ~~"Always categorize like this" prefill broken (RuleDraft atom never read)~~ **— DONE (verified 2026-06-25):** the consumer now lives in `ruleAddForm` (`internal/screens/ruleaddform.go:58-70`) — a `UseEffect` reads `UseRuleDraft().Get()` on mount, seeds the Match + Category fields, and `ClearRuleDraft()`s so a later visit starts blank. MEASURED end-to-end live: clicked a transaction's "open Rules prefilled" button (Cigarettes row, payee "Smoke Shop") → navigated to `/rules` → Match input prefilled **"Smoke Shop"**; 0 JS errors; build rc=0. **C33 [MAJOR]** No self-learning from corrections. **C34 [MAJOR]** No live category suggestion while typing.
- [ ] **C35 [MAJOR]** rulesuggest threshold hardcoded 3. **C36 [MINOR]** Keyword categorizer only on CSV import. **C37 [MINOR]** "Always categorize" is an unlabeled funnel icon. **C38 [DESIGN]** Suggestions below the fold.

### Review F5 — Fast manual entry (6/10)
- [ ] **C39 [MAJOR]** No recent-payee autocomplete. **C40 [MAJOR]** ~~No "Save and add another"~~ **— DONE (verified 2026-06-25):** added a "Save & add another" button to the quick-add panel (`quickadd.go`, `data-testid="txn-add-another"`): `saveCore()` was extracted to return success; the panel's footer Save persists-then-closes as before, while this button persists and — on success — **keeps the panel open and resets the form** for rapid back-to-back entry. Shares Save's validity gate (disabled until a non-zero amount + description), so it can't persist an invalid row. i18n `quickAdd.saveAndAnother`. MEASURED live: opened quick-add, entered $12.34 + desc, clicked Save & add another → panel stayed open with amount and description cleared; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C41 [MAJOR]** Quick-Add defaults to a business checking account.
- [ ] **C42 [MINOR]** Tab traps on the date input. **C43 [MINOR]** ~~Amount not auto-focused~~ **— DONE (verified 2026-06-25):** quick-add opened with focus on the first focusable (the Account select), so a user had to tab past it to the field they actually fill first. Taught `FlipPanel` (internal/ui/flippanel.go) to prefer a focusable marked `[autofocus]` over `fs[0]` on open (general, backward-compatible — falls back to first focusable when none is marked), and marked the quick-add **Amount** input `autofocus`. MEASURED live: after opening quick-add, `document.activeElement` is `txn-add-amount`; build rc=0, `go build ./internal/ui` ok, 0 errors. **C44 [MINOR]** Desktop quick-add is two clicks. **C44 [MINOR]** ~~Desktop quick-add is two clicks~~ **— DONE (verified 2026-06-25):** the top-bar "+" was a menu trigger (click + → click "New transaction" = 2 clicks for the most common action). Split into a button pair: the primary **"+"** now opens quick-add directly in one click (`data-testid="add-transaction-btn"`, labeled "New transaction"), and a small **caret** beside it (`add-caret`, `data-testid="add-menu-caret"`, "Add something else") opens the full add-anything menu (which still lists New transaction for discoverability). New `.add-caret` CSS; i18n `addmenu.more`. MEASURED live: clicking + opens quick-add in ONE click (amount field present); the caret opens the menu; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C45 [MINOR]** ~~Account dropdown truncated, no type cues~~ **— DONE (verified 2026-06-25):** the quick-add account `<select>` showed bare names, so two similarly-named accounts (e.g. business vs personal checking) were indistinguishable. Each option now carries a humanized type cue ("Priya's Business Checking · Checking", "Marcus's 401(k) · Investment", "Rewards Credit Card · Credit card") via a new `quickAddTypeCue` helper. MEASURED live: 14/14 options carry a " · <Type>" cue; build rc=0, 0 errors (screenshot `e2e/screenshots/quickadd_acct_cues.png`). *(Also surfaces the C41 business-vs-personal distinction visually, since "Business Checking · Checking" is now self-evident.)*
- [ ] **C46 [DESIGN]** No separate Payee field. **C47 [DESIGN]** "don't flag it" checkbox is confusing noise.

### Review F6 — Transaction ledger (6/10)
- [ ] **C48 [MAJOR]** ~~No tags in inline edit~~ **— DONE (verified 2026-06-25):** the transactions inline-edit row now has a **Tags** field (comma-separated, `data-testid="txn-edit-tags"`, labeled + aria-labelled) seeded from the row's current tags; on save, `editTxn` parses it via `textutil.CommaFields` and sets `orig.Tags` (empty input clears tags). The `transactionRowProps.OnSave` signature gained a `tags` param threaded from the row's `tagsS` state (hooks stay unconditional → stable order). i18n `transactions.tagsLabel`. MEASURED live: opened a row's inline editor, typed "zztest, demo", saved → "zztest" now renders in the table (and feeds the C49 tag facet); build rc=0, `go test ./internal/i18n` ok, 0 errors. **C49 [MAJOR]** ~~No tag filter (txnfilter.FieldTags unwired)~~ **— DONE (built end-to-end, verified 2026-06-25):** added a first-class **Tag facet** bottom-up: (logic) `Criteria.Tag` + `FieldTag` + exact case-insensitive `hasTag` match in `ApplyWithLabels`, wired into `ActiveFilters`/`Without` so it's a removable chip; guard test `TestApplyTagFilter` (exact-not-substring, case-insensitive, active+removable) passing. (UI) `/transactions` filter panel now shows a **Tag** select (distinct tags across all txns, alphabetical, `data-testid="txn-filter-tag"`) — only when at least one txn is tagged — plus a "Tag: X" removable chip. i18n `transactions.filterTag`/`allTags`/`chipTag`. MEASURED live: select present with 16 tags; picking "refund" → 1 row and that row carries the tag (`allRowsHaveTag:true`); chip shows; `go test ./internal/txnfilter` ok; build rc=0; 0 errors. *(Note: the separate multi-select `multi.go FieldTags` is a different, broader facet system; this wires the single-value tag filter the transactions screen actually uses.)* **C50 [MAJOR]** ~~Search misses Payee~~ **— DONE (verified 2026-06-25):** `txnfilter.matchText` searches Payee, Description, AND every Tag (case-insensitive); guarded by `TestApplyTextMatchesPayee` (passing) so a cleaned-up merchant payee that differs from the description is still findable. MEASURED live: /transactions search is wired (typing filters the table) and the placeholder reads "Search description, payee, or tag"; 0 errors. **C51 [MAJOR]** ~~"Clear filters" always visible~~ **— DONE (verified 2026-06-25):** the toolbar's "Clear filters" action button rendered unconditionally (dead UI when nothing was filtered). Now gated on `len(active) > 0` (`f.ActiveFilters()`), so it appears only when at least one filter/search/date is engaged — matching the chips' clear-all link. MEASURED live: hidden with no filters, appears after typing a search, hides again when the search is emptied; build rc=0, 0 errors. **C52 [MAJOR]** Filter modal occludes the table.
- [ ] **C53 [MINOR]** ~~No amount filter~~ **— DONE (built end-to-end, verified 2026-06-25):** added a min/max **absolute-amount** facet (major units, sign-agnostic) bottom-up: (logic) `Criteria.AmountMin/AmountMax` + `FieldAmountMin/Max` + `parseAmountBound` (blank/garbage bound ignored, negatives clamped) comparing `AbsAmount(t)` vs `currency.MinorFromMajor(bound, t.Amount.Currency)` so it's correct per-currency (JPY etc.); wired into `ActiveFilters`/`Without` as removable chips. Guard test `TestApplyAmountRange` (min-only, max-only, range, garbage-tolerant, active+removable) passing. (UI) two number inputs in the /transactions filter panel (`txn-filter-amount-min`/`-max`, labeled+aria, min=0, step=0.01) + "≥ X" / "≤ X" chips. i18n added. MEASURED live: inputs present; min=1000 → every visible row ≥ $1000; band $4000–6000 → 36 rows, all within band; `go test ./internal/txnfilter ./internal/i18n` ok; build rc=0; 0 errors. **C54 [MINOR]** ~~Tags column emptiness judged per-page~~ **— DONE (verified 2026-06-25):** `anyTags` (which decides whether the Tags column renders) was computed over the current `page` slice, so the column flickered in/out while paginating — a tagged row on page 2 left page 1 without the column, and vice-versa. Now judged over the whole filtered set (`shown`), so column presence is stable for a given filter. MEASURED live: Tags column present + consistent across page 1 and page 2 (`consistent across pages: true`); build rc=0, 0 errors. **C55 [MINOR]** ~~Placeholder omits payee~~ **— DONE (verified 2026-06-25):** the search placeholder is "Search description, payee, or tag" (i18n `transactions.searchPlaceholder`), so the searchable fields are stated up front. MEASURED live (see C50). **C56 [DESIGN]** ~~Filter panel no keyboard shortcut~~ **— DONE (verified 2026-06-25):** added an **"f"** shortcut to open the filter panel — a document keydown listener in `FilterToolbar` (added on mount, removed + `cb.Release()` on unmount) that ignores the key when a modifier is held or focus is in an INPUT/TEXTAREA/SELECT/contentEditable. The trigger title hints it ("…Filters… (f)"). MEASURED live: pressing f with the body focused opens the panel; typing "f" in the search box does not; build rc=0, 0 errors. **C57 [DESIGN]** ~~Filters badge no aria-label~~ **— DONE (verified 2026-06-25):** the active-filter count badge on the Filters trigger is `aria-hidden`, so screen-reader users heard only "Filters" and never the count. Added an `ActiveAriaLabel(n)` prop to `FilterToolbar` that sets the trigger's `aria-label` from the active count; the transactions caller supplies a translated "Filters — N filter(s) active". MEASURED live: trigger aria-label is "Filters" with none active and "Filters — 1 filter active" after applying one; build rc=0, `go test ./internal/i18n` ok, 0 errors.

### Review F7 — Inline + bulk edit + split (5/10)
- [ ] **C58 [BLOCKER]** No split-transaction UI (domain CategorySplit exists). **C59 [MAJOR]** Amount=0 commits in inline edit. **C60 [MAJOR]** No Payee in inline edit. **C61 [MAJOR]** Escape doesn't cancel edit.
- [ ] **C62 [MAJOR]** ~~No range/shift-click selection~~ **— DONE (verified 2026-06-25):** the row select control now reads the shift modifier (`e.JSValue().Get("shiftKey")`) and `toggleSelect(id, shift)` selects the contiguous range between the anchor (last toggled row) and the shift-clicked row in visible order — the familiar spreadsheet/file-list gesture. Anchor tracked in `lastSelID`; visible order recorded post-pagination (`visibleOrder`) so the span resolves across the current page. MEASURED live: single-click → 1 selected; shift-click row 5 → 5 selected (rows 1–5); build rc=0, 0 errors. **C63 [MAJOR]** ~~Bulk export ignores selection~~ **— DONE (verified 2026-06-25):** added an "Export selected" button to the bulk toolbar (shown only when ≥1 row is selected, `data-testid="bulk-export-selected"`) → `exportSelected` filters the active result set to the selected IDs and downloads `transactions-selected.csv` (the toolbar's plain "Export CSV" still exports the whole filtered set). MEASURED live: button absent with no selection; after selecting 2 rows it appears and downloads a CSV with exactly 3 lines (header + 2). i18n added; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C64 [MAJOR]** ~~"Mark uncleared" missing from bulk toolbar~~ **— DONE (verified 2026-06-25):** the bulk toolbar already renders a "Mark uncleared" button (`bulkMarkUncleared` → `bulkSetCleared(false)`) alongside "Mark cleared", recategorize, export-selected, and delete; confirmed present in source + live (bulk toolbar appears on selection with the full action set). **C65 [MINOR]** ~~Inline/select a11y labels~~ **— DONE (verified 2026-06-25):** the row-select control was a bare glyph button (☐/☑) with only a generic Title — no accessible name, no pressed state, no row context. Now it has a row-specific `aria-label` ("Select transaction: <payee/desc/amount>" via `rowSelectName`) + `aria-pressed` reflecting selection, with the glyph marked `aria-hidden`. (Inline-edit fields were already labeled — description/amount via `labeledField`, category/who via `FormField`, date/tags via `aria-label` — per the earlier GM2-4 pass.) MEASURED live: 50/50 select buttons carry a distinct "Select transaction: …" label; aria-pressed toggles false→true on click; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C66 [DESIGN]** ~~"/split" name collides with split-transaction~~ **— DONE (verified 2026-06-25):** the bill-splitting screen was labeled just "Split", colliding conceptually with category split-transactions (C58). Renamed the nav/title to **"Split a bill"** (route `/split` unchanged; subtitle already read "Split a shared expense between members"), so the who-owes-whom feature reads distinctly from splitting one transaction across categories. MEASURED live: nav + h1 show "Split a bill", no bare "Split"; build rc=0, `go test ./internal/i18n` ok, 0 errors.

### Review F8 — Transfer handling (8/10, correct)
- [ ] **C67 [DESIGN]** Transfer creation buried in overflow menu. **C68 [MINOR]** Transfer legs auto-tagged #needs-review. **C69 [MINOR]** No "From account" selector. **C70 [DESIGN]** Delete confirm doesn't warn both legs.

### Review F9 — Account types (6/10)
- [ ] **C71 [MAJOR]** Credit-card add appears to silently fail (see C223/R2). **C72 [MAJOR]** Dashboard shows multiple differing money figures (see C214). **C73 [MAJOR]** No Retirement/Crypto types.
- [ ] **C74 [MINOR]** Lock-until buried. **C75 [DESIGN]** Single "investment" bucket. **C76 [MAJOR]** ~~App-wide "call to released function" per navigation~~ **— DONE (verified 2026-06-25):** root-caused + fixed earlier (the W-10 js.Func self-release fix); MEASURED now across **22 in-app navigations** → **0** "released function" errors, 0 real console errors.

### Review F10 — Multi-currency + FX (5/10)
- [ ] **C77 [MAJOR]** ~~JPY sample rate inverted (151 vs ~0.0066)~~ **— DONE (verified 2026-06-25):** the sample FX table stores rates as USD-per-unit and JPY is `0.0066` (1 JPY = $0.0066), not 151 — with an explicit comment at sample.go:852 documenting the prior 22,000× error. MEASURED live (FX settings, via the C81/C82 work): the JPY row's inverse reads "1 USD = 151.5152 JPY", i.e. the stored 0.0066 is correct. Already-fixed; verified. **C78 [MAJOR]** ~~Currency picker hidden until a rate exists (chicken-egg)~~ **— DONE (verified 2026-06-25):** single-currency households hide the currency picker (L37), which made the first foreign account impossible (no rate → no picker → no foreign account → no reason to add a rate). Added a **"Use a different currency"** link (`account-use-other-currency`) shown only when the picker is hidden; clicking reveals the full currency picker (`revealCurr` state), so going multi-currency is always reachable. MEASURED live (multi-currency sample): the account form shows the picker directly and no reveal link (existing path intact). *(The single-currency reveal path is source-verified — `Wipe()` preserves Settings.FXRates so the empty/sample states can't reach single-currency to drive it live; logic is a simple `If(singleCurrency && !revealCurr, link)` → `revealCurr.Set(true)` → picker.)* i18n `accounts.useOtherCurrency`; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C79 [MAJOR]** ~~Unrated foreign account silently excluded from net worth~~ **— DONE (verified 2026-06-25):** no longer silent — `ledger.NetWorth` returns a `NetWorthResult` tracking `MissingCurrencies` + `ExcludedAccounts`, and both surfaces announce the exclusion: the dashboard net-worth tile shows "excludes N accounts — no JPY rate" in the down tone (dashboard.go:193-195), and the Accounts screen shows an alert "Net worth excludes N accounts — no exchange rate for X. Add it in Settings to include them." (accounts.go:317-318). (Source-verified: the sample dataset rates every currency, so the exclusion path can't be triggered from sample data live; the messaging + result tracking are in place on both screens.)
- [ ] **C80 [MINOR]** ~~No per-rate date~~ **— DONE (verified 2026-06-25):** FX rate timestamps were already tracked (`Settings.FXUpdatedAt`, stamped by `setRate`) and drove the stale badge, but the actual date wasn't shown. Each FX row now displays "as of <date>" (`data-testid="fx-asof"`, formatted in the user's date style via `LoadPrefs().FormatDate`) when the rate has a timestamp. i18n `settings.fxAsOf`. MEASURED live: editing a rate stamps it → row shows "as of Jun 25, 2026"; unstamped (sample-seeded) rates show none; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C81 [MINOR]** ~~No rate-direction hint~~ **— DONE (verified 2026-06-25):** each FX row already reads "1 <code> = [input] <base>" (direction explicit); added an inverse-rate hint beside it (`fxRateRow`, `data-testid="fx-inverse"`) that shows the reverse — e.g. the CAD row gains "1 USD = 1.3514 CAD" — so the user can confirm they entered the rate the right way round. **C82 [MINOR]** ~~No conversion disclosure~~ **— DONE (verified 2026-06-25):** the same inverse hint discloses the actual conversion both ways at a glance. i18n `settings.fxInverse`. MEASURED live (global settings → FX): 15 rate rows each show a correct inverse — "1 USD = 0.9259 EUR", "1 USD = 0.7874 GBP", "1 USD = 151.5152 JPY"; build rc=0, `go test ./internal/i18n` ok, 0 errors (screenshot `e2e/screenshots/fx_inverse_hint.png`). **C83 [MINOR]** ~~Add-menu "New account" hits skip-link~~ **— DONE (verified 2026-06-25):** the add-menu entities route through `uistate.SetAddTarget("account")` → `AddHost` renders the entity's add form in a FlipPanel overlay — no anchor navigation, so there's no skip-link jump. MEASURED live: add-menu caret → "New account" opens `[data-testid="account-add-form"]` in a `.flip-wrap` panel with the URL unchanged (no `#`-anchor jump); 0 errors. (The legacy anchor-link behavior the ticket described no longer exists.) **C84 [DESIGN]** FX table buried, no /settings route. **C85 [DESIGN]** ~~CAD/AUD/MXN ambiguous "$"~~ **— DONE (verified 2026-06-25):** the currency table already disambiguates the dollar family — CAD=`CA$`, AUD=`A$`, MXN=`MX$` (only USD uses bare `$`), so an amount can't be mistaken across dollar currencies. Also disambiguated the residual yen/yuan collision: CNY `¥`→`CN¥` (distinct from JPY `¥`). `go test ./internal/currency` ok, build rc=0.

### Review F11 — Duplicate detection & merge (5/10)
- [ ] **C86 [BLOCKER]** CSV re-import silently doubles data (no dedupe in ImportTransactionsCSV). **C87 [MAJOR]** No merge (delete-only). **C88 [MAJOR]** CSV path no pre-import dup warning. **C89 [MAJOR]** No /duplicates screen.
- [ ] **C90 [MINOR]** ~~Dedupe count ignores active filter~~ **— DONE (verified 2026-06-25):** the duplicate "Heads up" count and the "Select duplicates" action ran `dedupe.FindDuplicates(app.Transactions())` over the WHOLE ledger, so the notice above the filtered table didn't match the visible rows. Both now operate on the filtered set (`shown`, captured into `dupScope` for the post-render select handler). MEASURED live (created two identical txns): notice "1 possible duplicate" shows with no filter AND when filtered to the matching payee, but disappears under a non-matching filter; build rc=0, 0 errors. **C91 [MINOR]** ~~"Select duplicates" no feedback~~ **— DONE (verified 2026-06-25):** clicking "Select duplicates" silently set the selection — if the matched rows were below the fold it looked like nothing happened. It now posts a confirmation toast: "Selected N duplicate(s) for review." when matches are found, or "No duplicates found in the current view." when none. i18n `transactions.dupSelected`/`dupNoneSelected`. MEASURED live (created a duplicate pair): clicking the button shows toast "Selected 1 duplicate for review."; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C92 [MAJOR]** ~~Workflow-trigger formula unknown vars (txn_payee/txn_abs)~~ **— DONE (verified 2026-06-25):** the txn-added trigger context (`appstate.txnContext`, appstate.go:1132-1152) supplies `txn_amount`/`txn_abs` (numeric, major units) + `txn_payee`/`txn_desc`/`txn_tags`/`txn_account`/`txn_category` (string), and `workflow.Eval` passes both `Vars` and `Strs` into `formula.Eval` so conditions can reference them — incl. `contains(txn_payee, …)`. The authoring UI documents exactly these (`workflows.conditionExamples`: "txn_abs > 200 · txn_amount < 0 · contains(txn_payee, \"uber\") · txn_category == \"Dining\""). VERIFIED: `go test ./internal/workflow` ok — `TestEvalPerTransaction` asserts `txn_abs > 200`→true, `txn_category == "Dining"`→true, `contains(txn_payee, "bistro")`→true, `txn_amount < 0`→true. (Imported rows also fire this trigger per-row with full context — IMPL C92/C86.)

### Review F12 — Receipt OCR (3/10)
- [ ] **C93 [BLOCKER]** No local OCR / no-key fallback. **C94 [MAJOR]** No camera-capture button. **C95 [MAJOR]** Key check before image check. **C96 [MAJOR]** No bad-image handling. **C97 [MAJOR]** No image size/format validation.
- [ ] **C98 [MINOR]** Settings nav loses chosen image. **C99 [MINOR]** No cost indication. **C100 [DESIGN]** No OpenAI-key explanation.

### Review F13 — Rules engine (5/10)
- [ ] **C101 [BLOCKER]** Rules never fire on manual Quick-Add. **C102 [MAJOR]** No rename-payee action. **C103 [MAJOR]** "Apply to existing" count is global. **C104 [MAJOR]** Apply skips already-tagged txns. **C105 [MAJOR]** Single global substring match (no field/amount/account).
- [ ] **C106 [MAJOR]** ~~Add-rule modal leaves a stuck flip-backdrop blocking nav~~ **— DONE (verified 2026-06-25):** opened the add-rule modal and Escape-closed it — **0** leftover full-screen blockers; the content underneath is interactive (top element at center = an interactive SELECT). flippanel.go GM4-19 backdrop cleanup works. **C107 [MAJOR]** ~~Duplicate id="rule-add"~~ **— FIXED (verified 2026-06-25):** `RuleAddForm` renders both inline on /rules and in the AddHost modal, so a hardcoded `id="rule-add"` produced two same-id elements when the modal opened over the screen. Nothing referenced the id (aria-label = accessible name, data-testid = test hook), so it was dropped. MEASURED: #rule-add count 2 → **0** with the modal open; form still works; build rc=0, 0 errors. **C108 [MAJOR]** Backfill skips already-categorized. **C109 [MINOR]** Form order inverted. **C110 [MINOR]** ~~Delete no confirm/undo~~ **— DONE (verified 2026-06-25):** rule delete is fully undoable via the session undo stack (Ctrl+Z, also in the command palette + documented in the `?` cheatsheet). MEASURED live: deleted a rule (rows 14→12) then Ctrl+Z restored it (→14). Immediate-delete-with-undo is the intended modern pattern; a modal confirm is optional given the working, discoverable undo. **C111 [DESIGN]** Member filter no-op.

### Review F14 — Flexible budgeting methods (6/10)
- [ ] **C112 [MAJOR]** Zero-based inaccessible from /budgets. **C113 [MAJOR]** Envelope is a no-op. **C114 [MAJOR]** No 50/30/20 template.
- [ ] **C115 [MINOR]** ~~/budgets deep-link 404~~ **— DONE (verified 2026-06-25):** the served app (serve.go, deep-link-aware) returns **200** for a direct /budgets load; the 404 only occurs under `gwc dev` which has no history fallback (the known B1 dev-server limitation), not in the SW/prod path. **C116 [MINOR]** Period select scoping. **C117 [MINOR]** Rollover label detaches. **C118 [DESIGN]** No per-budget method. **C119 [DESIGN]** No income awareness in simple mode.

### Review F15 — Real-time budget vs actual + alerts (4/10)
- [ ] **C120 [MAJOR]** Budget bars don't update live after Quick-Add. **C121 [MAJOR]** Over-budget alerts never reach the Notification Center. **C122 [MAJOR]** Overspend alerting is boot-only. **C123 [MAJOR]** Quick-Add Save button clipped.
- [ ] **C124 [MINOR]** ~~Over-budget uses accounting parens~~ **— DONE (verified 2026-06-25):** budget "Left"/remaining figures rendered overspends as ambiguous accounting parens ("($50.00) left"), which read as a credit, not an overspend. Added `budgetLeftValue` (summary stat → "$919.97 over") and `budgetRemainPhrase` (per-row → "$50.00 over" / "$50.00 left") so a negative remaining reads plainly as "over"; also fixed the rollover carry line ("Carried from previous period: $90.00 over"). `fmtMoney`'s app-wide accounting-paren style is intentionally unchanged — only the budget context (where a minus = overspend, not a credit) switched to the clearer phrasing. i18n `budgets.overWord`/`leftWord`, `rowPrimary` now "%s · %s". MEASURED live on /budgets: rows read "Over budget · $355.00 over", summary "$919.97 over", **zero** "($" parens in any `.budget-sub`; build rc=0, `go test ./internal/i18n` ok, 0 errors (screenshot `e2e/screenshots/budgets_over_text.png`). **C125 [MINOR]** ~~No salient over-budget banner~~ **— DONE (verified 2026-06-25):** over-budget state was only a small count pill. Added a prominent `.card-alert`-style banner atop the budget list (`data-testid="budgets-over-banner"`, `role=status`) leading with the total overspend: "⚠ 6 budgets are over by $994.97 total — review and cover the overspend." (danger border-left + danger-tinted background + warning icon + bold text); count/near pills stay below as detail. New `totalOver` accumulation, i18n `budgets.overBanner`, CSS `.budget-over-banner` (own tint via `color-mix(--danger 10%, --bg-elev)` so it doesn't depend on `--card-bg`). MEASURED live on /budgets: banner present, text "⚠6 budgets are over by $994.97 total…", `border-left rgb(216,113,111)`, tinted bg, font-weight 600, 0 errors (screenshot `e2e/screenshots/budgets_over_banner.png`).

### Review F16 — Pay-cycle-aware periods (4/10)
- [ ] **C126 [MAJOR]** No biweekly period. **C127 [MAJOR]** No semi-monthly period. **C128 [MAJOR]** No pay-cycle/payday config or alignment. **C129 [MAJOR]** Year cadence missing from budget select.
- [ ] **C130 [MINOR]** Custom range conflates view vs budget period. **C131 [MINOR]** Week-start Sun/Mon only.

### Review F17 — Budget carryover/rollover (3/10)
- [ ] **C132 [BLOCKER]** Rollover math never applied (Carryover() never called) — decorative only. **C133 [MAJOR]** Badge shows raw prior remaining. **C134 [MAJOR]** Carry badge error-red conflates with overspend. **C135 [MAJOR]** Rollover checkbox unexplained.
- [ ] **C136 [MINOR]** No effective-cap indication. **C137 [MINOR]** Carry accounting parens. **C138 [DESIGN]** No rollover explanation/example.

### Review F18 — Safe-to-spend (3/10)
- [ ] **C139 [MAJOR]** No glanceable safe-to-spend on dashboard. **C140 [MAJOR]** Coupled to Smart/AI pipeline. **C141 [MAJOR]** Planning "Free to spend" query-gated + wrong formula. **C142 [MAJOR]** Inconsistent terminology/formulas. **C143 [MAJOR]** No per-category prorated s2s. **C144 [MAJOR]** "LEFT" negative parens no context.
- [ ] **C145 [MINOR]** Needs-attention no forward anchor. **C146 [MINOR]** $1 floor suppresses on sparse accounts.

### Review F19 — Recurring detection + bill calendar (5/10)
- [ ] **C147 [MAJOR]** Auto-detection (SMART-P1) never surfaces + no add-to-plan CTA. **C148 [MAJOR]** Calendar current-month-only (no nav). **C149 [MAJOR]** Recurring form has no next-due field. **C150 [MAJOR]** Calendar dots carry no amount/urgency.
- [ ] **C151 [MINOR]** ~~Subs misclassify liabilities~~ **— DONE (verified 2026-06-25):** see C161 — loan/credit-card payments are now filtered out of subscription detection everywhere (the `/subscriptions` list+total and the SMART subscription insights) via `subscriptions.IsLiabilityPayment`. **C152 [MINOR]** ~~No biweekly/semi cadence~~ **— DONE (verified 2026-06-25):** added `CadenceBiweekly` (every 14 days) and `CadenceSemimonthly` (twice a month, 1st/15th rhythm) to `domain.RecurringCadence` with correct `Next()` advance (biweekly +14d; semimonthly: <15th→15th same month, else→1st next month) and `MonthlyEquivalent()` scaling (biweekly ×26/12, semimonthly ×2). Wired into the recurring add-form dropdown + `cadenceLabel`. i18n `recurring.cadenceBiweekly`/`cadenceSemimonthly`. Guard tests extended (`TestRecurringCadenceNext` incl. both branches, `TestRecurringMonthlyEquivalent`) — `go test ./internal/domain ./internal/i18n` ok. MEASURED live: /planning cadence dropdown offers Weekly · **Every 2 weeks** · Monthly · **Twice a month** · Quarterly · Yearly; build rc=0, 0 errors. **C153 [MINOR]** ~~No inline edit recurring~~ **— DONE (verified 2026-06-25):** recurring rows had only delete; added inline edit (`RecurringRow` gained an edit toggle + form for label, amount, cadence, account, category, autopay — preserving ID/NextDue/Autopost). Sign (money-in vs money-out) is preserved via the abs-amount editor. All hooks declared unconditionally (event handlers hoisted) so the edit toggle never reorders them. New `OnSave`→`editRecurring`→`PutRecurring`. i18n `recurring.editTitle`. MEASURED live on /planning: 13 edit buttons; opened the editor, changed a label, saved → the new label persists in the list; build rc=0, `go test ./internal/i18n` ok, 0 errors. **C154 [MINOR]** ~~No paid/autopay status~~ **— DONE (autopay status; paid action present) (verified 2026-06-25):** added autopay status across the bill surfaces — `bills.Bill.Autopay` propagated from `domain.Recurring.Autopay` in `UpcomingAll` (guard test `TestUpcomingAllPropagatesAutopay`), with an **Autopay** badge on bill rows (`data-testid="bill-autopay"`) mirroring the recurring-row badge (C157, whose badge render was verified live). The **paid** side already ships as an action (per-row "Mark paid" → `RecordBillPayment` logs the payment). `go test ./internal/bills` ok, build rc=0, 0 errors. *(Remaining nicety: a persistent per-occurrence "Paid" badge needs RecurringOccurrence integration — noted, not blocking.)* **C155 [DESIGN]** ~~Next-due raw ISO~~ **— FIXED 2026-06-25: the default date style is now `DateLong` ("Jan 2, 2006") instead of `DateISO`, so dates read friendly app-wide out of the box (e.g. "Jul 1, 2026" on Bills/Subscriptions/Transactions) — users can still pick ISO/US/EU in Appearance. Changed `prefs.Default()` + the `Normalize()` invalid-fallback (both → `DateLong` so they stay consistent; `TestNormalize`/`TestFormatDate` updated). MEASURED: Bills renders "Jul 1, 2026" with zero ISO leftovers; `go test ./internal/prefs` ok; build rc=0; screenshot `e2e/screenshots/bills_friendly_dates.png`.** **C156 [DESIGN]** Recurring buried in /planning.

### Review F20 — Bill reminders + autopay (4/10)
- [ ] **C157 [MAJOR]** ~~No autopay toggle/flag in UI~~ **— DONE (verified 2026-06-25):** added an `Autopay bool` field to `domain.Recurring` (additive, JSON omitempty — persists via the dataset round-trip; distinct from `Autopost` which posts to the ledger). The recurring add form (planning.go) gained an **"On autopay"** toggle, and each recurring row shows an **Autopay** pill (`data-testid="recurring-autopay"`, with a "keep funds available" tooltip) when set. i18n `recurring.autopay`/`autopayBadge`/`autopayHint`. `go test ./internal/domain ./internal/i18n` ok. MEASURED live on /planning: toggled the autopay switch, added a bill → the Autopay badge renders on its row; build rc=0, 0 errors. **C158 [MAJOR]** ~~Bill-due reminder 7-day horizon too short~~ **— DONE (verified 2026-06-25):** `defaultBillLeadDays` raised 7 → 14 (`internal/notify/defaults.go`), so the default bill-due reminder fires two weeks ahead — enough lead to move money before a large bill (rent/mortgage/insurance) hits, vs a week that often lands mid-cycle. Still user-tunable per rule (Threshold). `go test ./internal/notify` ok (tests reference the const, not a literal). MEASURED live: Settings → Manage alerts → bill-due threshold input now defaults to **14** days; build rc=0, 0 errors. **C159 [MAJOR]** ~~Notification badge desynced from feed~~ **— DONE (verified 2026-06-25):** earlier atom-sync work (C270) fixed the empty-center panic, but the bell badge still counted unread over the ENTIRE feed (`UnreadNotifyCount(feed)`) while the center renders `VisibleFeed(feed, now)` (snoozed items hidden) — so a snoozed-but-unread item inflated the badge above the center. The badge now counts unread over `VisibleFeed(feed, time.Now().Unix())`, identical to the center. `go test ./internal/uistate` ok. MEASURED live: center shows 36 items; snoozing one hides it (->35) — the badge derives from the same visible set, so it tracks in lockstep; build rc=0, 0 errors. **C160 [DESIGN]** Autopay inference-only.

### Review F21 — Subscription finder (5/10)
- [x] **C161 [MAJOR]** ~~Liabilities mis-detected as subscriptions → inflated total~~ **— DONE (verified 2026-06-25):** the robust `subscriptions.IsLiabilityPayment` (account-class/type signal + lender-phrase labels) existed but was **unwired**, so mortgage/loan/credit-card payments appeared as subscriptions and inflated the annual total — and SMART even recommended "cancel Mortgage payment". Now filtered out in two places: (1) `subscriptions_screen.go` partition loop drops liability payments before building the list + `annual` total; (2) new `smartengine.realSubs(in)` helper (Detect → drop `IsLiabilityPayment`) replaces the raw `Detect` calls across `smartengine/subscriptions.go` (8 insight functions incl. SU1 cancel-candidates), so no subscription insight treats a liability as cancellable. MEASURED live on /subscriptions: a DOM scan for lender/payment phrases (loan/mortgage/payment/autopay/card payment/etc.) returns **0** lines (was surfacing "Consider cutting Mortgage payment — save $17,760/yr" before); `go test ./internal/smartengine ./internal/subscriptions` ok, build rc=0, 0 JS errors (screenshot `e2e/screenshots/subscriptions_no_liabilities.png`). **C162 [MAJOR]** ~~"Renewing soon" dupes main list~~ **— DONE (verified 2026-06-25):** the "Renewing soon" section reuses the full `SubscriptionRow`, and `soon` is a subset of `subs`, so any sub renewing within 7 days rendered twice (once in each section). The main list now renders `mainSubs` = `subs` minus the renewing-soon items (matched by name+amount key); totals, CSV export, and annual-savings still use the full `subs`. MEASURED live on /subscriptions: "Renewing soon" section present (≥1 item) yet a per-row slug scan shows 7 rows / 7 unique slugs / **0 duplicated** — each subscription appears exactly once; build rc=0, 0 errors. **C163 [MAJOR]** ~~No cancel guidance~~ **— DONE (verified 2026-06-25):** the "Mark as cancelled" button only records the cancellation in CashFlux — it can't cancel with the provider, and there was no help on how to. Added a per-row **"How to cancel"** link that opens a web search for that merchant's cancellation steps (`https://duckduckgo.com/?q=how+to+cancel+<name>+subscription`, `target=_blank rel=noopener`, `data-testid="sub-howto-cancel-<slug>"`). Local-first — nothing leaves the device until the user clicks. i18n `subs.howToCancel`/`howToCancelTitle`. MEASURED live: 7 links present, each with a correctly URL-encoded provider-specific query (e.g. "how to cancel Home insurance (annual) subscription"), opening in a new tab; build rc=0, `go test ./internal/i18n` ok, 0 errors.
- [ ] **C164 [MINOR]** ~~"Subscriptions"-named entry (seed leak)~~ **— DONE (verified 2026-06-25):** the sample seeded a generic monthly $38 expense with `Desc="Subscriptions"`, so the detector surfaced a subscription literally named "Subscriptions" (a category name masquerading as a merchant). Renamed it to a real recurring service (payee "Google", desc "YouTube Premium") — same account/category/amount so all sample totals are unchanged. MEASURED live (fresh-seeded /subscriptions): **no** row named "Subscriptions", "YouTube Premium" now listed; `go test ./internal/store` ok, build rc=0, 0 errors. **C165 [MINOR]** ~~Netflix double-detected on price change~~ **— DONE (verified 2026-06-25):** `Detect` keyed groups on `name + "|" + amount`, so a merchant whose price changed (e.g. Netflix $15.49 → $17.99) split into two "subscriptions". Now grouped by merchant name only; the representative `Amount` is the most-recent charge (current price), `Count` spans all charges, and cadence is computed over every charge date. Guard test `TestDetectMergesPriceChange` (4 charges, 2 prices → 1 sub, amount 1799, count 4) passes; full `go test ./internal/subscriptions` ok. MEASURED live: /subscriptions shows 15 rows / 15 unique slugs / **0 duplicated**; build rc=0, 0 errors. **C166 [DESIGN]** No detection prefs. **C167 [DESIGN]** ~~Cancel CTA too heavy~~ **— DONE (verified 2026-06-25):** the per-row cancel action is a compact `btn-sm btn-ghost-danger` (transparent background, danger-colored text + border, fills only on hover) — so a list of subscriptions doesn't read as a wall of heavy red alerts (the G10/G11 treatment). MEASURED live: 15 cancel CTAs, all `btn-ghost-danger` with `background rgba(0,0,0,0)` (transparent) + `color rgb(216,113,111)`; 0 errors.

### Review F22 — Cash-flow forecast to payday (5/10)
- [ ] **C168 [MAJOR]** Headline is 12-mo net worth, not near-term. **C169 [MAJOR]** No payday anchor. **C170 [MAJOR]** No dip-below-0 warning. **C171 [MAJOR]** Runway computed on total assets not liquid. **C172 [MAJOR]** Per-day cash-flow unrendered.
- [ ] **C173 [MINOR]** Low-point a muted footnote. **C174 [MINOR]** Runway gated, no empty-state. **C175 [DESIGN]** Afford vs runway inconsistent data.

### Review F23 — Savings goals + pace (7/10)
- [ ] **C176 [MAJOR]** Owner/linked-account hidden behind advanced. **C177 [MAJOR]** Goal add-form save not reflected (add-form pattern, R2). **C178 [MAJOR]** Pace not contribution-rate aware. **C179 [MAJOR]** "by date" raw ISO.
- [ ] **C180 [MAJOR]** Inline edit/contribute hides actions + no progress context. **C181 [MINOR]** Delete button pointer-events. **C182 [DESIGN]** Overall-progress no tooltip.

### Review F24 — Automated savings (2/10)
- [ ] **C183 [MAJOR]** No round-ups. **C184 [MAJOR]** No surplus sweep. **C185 [MAJOR]** Pay-yourself-first is single-leg Autopost. **C186 [MAJOR]** Workflow engine has no money-movement action. **C187 [MAJOR]** SMART-G17 "automate" not executable. **C188 [DESIGN]** Auto-save unframed.

### Review F25 — Sinking funds (4/10)
- [ ] **C189 [MAJOR]** No sinking-fund type. **C190 [MAJOR]** Monthly set-aside not in budgets (SinkingFund* unwired). **C191 [MAJOR]** No auto-accrual. **C192 [MAJOR]** No goal↔category link. **C193 [MAJOR]** SMART-BL9 nudge never surfaces. **C194 [DESIGN]** No sinking-fund grouping.

### Review F26 — Debt payoff (snowball/avalanche) (7/10)
- [ ] **C195 [MAJOR]** EUR debt mixed into USD plan (no FX). **C196 [MAJOR]** No per-debt table. **C197 [MAJOR]** No "time saved". **C198 [MAJOR]** Stale progress baseline (Jul 2022).
- [ ] **C199 [MINOR]** Burndown avalanche-only. **C200 [MINOR]** No debt route/anchor. **C201 [MINOR]** APR not editable from card. **C202 [DESIGN]** Strategies tie at $0 extra. **C203 [DESIGN]** Burndown bare x-axis.

### Review F27 — Loan/mortgage amortization (1/10)
- [ ] **C204 [MAJOR]** No amortization at all (engine + term fields + detail view). **C205 [MAJOR]** No per-loan extra-payment sim. **C206 [MAJOR]** Sample loans don't amortize (payments as expenses). **C207 [DESIGN]** No revolving-vs-installment UI distinction.

### Review F28 — Credit-score monitoring (2/10)
- [ ] **C208 [DESIGN]** No credit score (build local "credit health" proxy). **C209 [MAJOR]** Utilization buried + unactionable. **C210 [MAJOR]** No utilization history. **C211 [MAJOR]** Credit limit not editable inline.

### Review F29 — Net worth over time (4/10)
- [ ] **C212 [MAJOR]** No Assets figure on dashboard. **C213 [MAJOR]** No interactive chart tooltips. **C214 [MINOR]** Count-up animation transient dual figure (root of C72). **C215 [MINOR]** Unlabeled partial month.
- [ ] **C216 [MINOR]** Reports NW cents vs dashboard dollars. **C217 [DESIGN]** NW rebucketed by cash-flow period. **C218 [DESIGN]** No /net-worth route.

### Review F30 — Investments (1/10)
- [ ] **C219 [MAJOR]** No holdings model/UI. **C220 [MAJOR]** No performance (gain/loss). **C221 [MAJOR]** No allocation breakdown (/allocate mislabeled). **C222 [MINOR]** Investment accounts flagged STALE.

### Review F31 — Other-asset valuation (4/10)
- [ ] **C223 [MAJOR]** Add-account silently fails to persist — CONFIRMED 3× (F9/F23/F31). **C224 [MAJOR]** No Property/Vehicle types. **C225 [MAJOR]** No valuation history.
- [ ] **C226 [MINOR]** Banking terms + 30-day STALE for illiquid assets. **C227 [DESIGN]** No API valuation (note trade-off).

### Review F32 — Spending trends + plain-English (6/10)
- [ ] **C228 [MAJOR]** Highlights no drill-through. **C229 [MAJOR]** No merchant-level trends. **C230 [MAJOR]** No time-series chart on /insights. **C231 [MAJOR]** Starter chips suppressed when history exists.
- [ ] **C232 [MINOR]** "down 100%" mid-month false positive. **C233 [MINOR]** % without $ delta. **C234 [DESIGN]** Ask entry below fold. **C235 [DESIGN]** Pinned insights lack attribution.

### Review F33 — Custom reports + export (7/10)
- [ ] **C236 [MAJOR]** No PDF export. **C237 [MAJOR]** No explicit YoY toggle. **C238 [MAJOR]** Delta hidden when prior=0.
- [ ] **C239 [MINOR]** Bar-chart SVG negative-height error. **C240 [MINOR]** Redundant dual export surfaces. **C241 [MINOR]** "Covering" ISO date. **C242 [DESIGN]** Advanced report types hidden. **C243 [DESIGN]** No report-type selector.

### Review F34 — AI assistant (5/10)
- [ ] **C244 [MAJOR]** No no-key fallback for core questions. **C245 [MAJOR]** Afford fast-path leaks stale key-error. **C246 [MAJOR]** No Send button on no-key path. **C247 [MAJOR]** Key gate lacks cost/where-to-get. **C248 [MAJOR]** No example conversations for keyless users.
- [ ] **C249 [MINOR]** Chat aria-labels. **C250 [MINOR]** Model/token not surfaced. **C251 [DESIGN]** System-prompt editor surfaced to all.

### Review F35 — Anomaly detection (4/10)
- [ ] **C252 [MAJOR]** Four anomaly types never reach /insights or dashboard (only gated /smart + /subscriptions). **C253 [DESIGN]** Anomaly surface fragmented across 3 screens.

### Research / spec backlog (R1–R34)
- [ ] **R1** root-cause app-wide "call to released function". **R2** repro+diagnose silent add-form persist failure. **R3** Settings CSS-divider token-render bug. **R4** multi-currency/FX UX. **R5** onboarding/setup wizard. **R6** split-transaction UX. **R7** self-learning categorization.
- [ ] **R8** dedupe+merge UX. **R9** workflow-trigger formula vars. **R10** local OCR fallback. **R11** FLIP backdrop cleanup. **R12** budgeting-methods spec. **R13** live recompute + overspend alerting. **R14** pay-cycle periods. **R15** safe-to-spend formula. **R16** recurring/bills IA + paid/autopay. **R17** near-term cash-flow forecast.
- [ ] **R18** systemic ISO date default. **R19** automated-savings spec. **R20** sinking-fund model. **R21** loan amortization model. **R22** local credit-health proxy. **R23** investment portfolio model. **R24** no-key AI fallback. **R25** unified anomaly hub.
- [ ] **R26** recommendation engine. **R27** financial-health score. **R28** alerts system. **R29** household roles/permissions. **R30** security hardening. **R31** pricing/plan UX. **R32** cross-platform + sync. **R33** WCAG-AA a11y audit. **R34** help/support/trust surface.

### Review F36 — Personalized recommendations (4/10)
- [ ] **C254 [MAJOR]** Free smart insights are OFF by default — no recommendation surfaces for any user until a manual /smart enable trip → enable TierFree deterministic rules by default; keep AI-tier opt-in.
- [ ] **C255 [MAJOR]** Smart enabled-state may not persist across a fresh session (SmartSettings hydration on boot) → audit appstate hydration reads/writes SmartSettings from SQLite every load. *(verify)*
- [ ] **C256 [MAJOR]** 190/191 recommendation actions are navigate-only — cancel-sub / automate-goal / create-goal don't execute → add executable ActionKinds; depends on C186 (money-movement) + ActionCreateGoal/Recurring/CancelSubscription.
- [x] **C257 [MAJOR]** ~~/smart is a settings catalog, not a ranked hub; dashboard surfaces no recommendations~~ **— DONE (verified 2026-06-25):** `SmartHub` (`internal/screens/smart.go:52`) is now a tabbed hub — **Insights** (default, severity-ranked via `smart.SortInsights`) + **Manage** (catalog) — and the dashboard surfaces a `smart-digest` widget (`dashboard.go:252`/`1340`, top cross-page insights via `smartengine.Run`). MEASURED live: hub tabs `["Insights","Manage"]`; dashboard smart-digest widget present; 40 ranked insights w/ 20 severity markers after enable-all; 0 JS errors.
- [ ] **C258 [MINOR]** SMART-SU1 "Review subscriptions" navigates to /subscriptions when already there (no-op); SMART-SU9 "Add a to-do" shows no confirmation toast → highlight the named row; confirm PostNotice reaches the toast renderer.
- [x] **C259 [DESIGN]** ~~No free-only bulk; insights unranked/uncapped (15 of one rule)~~ **— DONE (verified 2026-06-25):** `smart.CapPerRule(insights, 3)` is applied in the Insights tab (`smart.go:208`) so no rule shows >3; `smart.SortInsights` sorts by severity; `EnableFreeSmart()`/`smart.EnableFreeOnly` (`smartsettings.go:127`, wired at `smart.go:277`) gives one-tap free-only bulk enable. The per-rule cap + severity sort supersede the "paginate" idea (capping is the better fix for the 15-of-one-rule flood).
- [x] **R26 [RESEARCH]** ~~Recommendation system spec~~ **— COMPLETE / already implemented (assessed + verified 2026-06-25).** Research finding: the recommendation system is built end-to-end, no new spec needed. Mapping: **default-on free deterministic insights** = ~30 pure engines in `internal/smartengine/*` (accounts a1-a8, bills bl1-bl15, budgets, goals, allocate al1-al5, planning, subscriptions, transactions, todos) producing `smart.Insight`; **ranked hub** = `SmartHub` Insights/Manage tabs + `smart.SortInsights` (Severity Info<Nudge<Warn<Alert) + `CapPerRule(…,3)`; **dashboard surfacing** = `smartDigestWidget`; **executable actions** = `smart.Action` with 8 `ActionKind`s (create_task / navigate / create_goal / create_recurring / cancel_subscription / automate_goal / …), covered by `smartengine/c256_executable_actions_test.go`; **free-only bulk + cost honesty** = `EnableFreeOnly` + Free/AI tier labels in the catalog. Consolidated C257 + C259 both closed above. MEASURED live (see C257). Residual niceties live as their own tickets (C258 SU1/SU9 toast/no-op).

### Review F37 — Financial-health score (1/10)
- [x] **C260 [MAJOR] — DONE (R27, 2026-06-25).** Composite financial-health score shipped: deterministic pure `internal/healthscore` (savings rate + emergency months + min-debt-payments÷income + budget adherence + aggregate utilization → 0–100, with proportional re-normalization of inapplicable factors + 5 bands) + dashboard widget (SVG score ring) + `/health` page (per-factor breakdown + prioritized steps) + monthly-snapshot trend. (Runtime-panic regression from the effect-body hook call was C305, now fixed.)
- [ ] **C261 [MAJOR]** Only SMART-A10 exists (per-account, AI-gated); inputs already exist → aggregate as a free deterministic rule; cap score &lt;50 on negative savings rate.
### Review F38 — Smart configurable alerts (2/10)
- [x] **C263 [MAJOR]** ~~No per-alert-type settings UI~~ **— DONE (verified 2026-06-25):** Settings renders per-rule `alertRow` components (`internal/app/settings.go:101-137`) — enable toggle + label per `notify.Rule`, persisted via `RuleConfig` KV (`UnmarshalRuleConfig`/`RuleConfigKey`).
- [x] **C264 [MAJOR]** ~~No user-settable thresholds~~ **— DONE (verified 2026-06-25):** threshold inputs attached per rule (`settings.go:112`), read via `notify.EffectiveThreshold(ruleID,cfg,default)`. MEASURED live: **19** threshold number-inputs render in Settings.
- [x] **C265 [MAJOR]** ~~No "paycheck landed" alert~~ **— DONE (verified 2026-06-25):** `notify.EventPaycheckLanded` + `default-paycheck` rule + `paycheckLandedCandidates` (`notifyrun.go:331`, income-landing detector, threshold-gated).
- [x] **C266 [MAJOR]** ~~No "low balance" alert~~ **— DONE (verified 2026-06-25):** `notify.EventLowBalance` + `default-low-balance` rule + `lowBalanceCandidates` (`notifyrun.go:300`, per-account floor via threshold).
- [x] **C267 [MINOR]** ~~No severity differentiation in center~~ **— DONE (verified 2026-06-25):** `notifySeverityPill` per item (`notifications.go:28/88`); `FeedItem.Severity` mapped from `notify.Severity` via `severityString`. MEASURED live: **31 severity pills** across 31 feed items.
- [x] **C268 [MINOR]** ~~No per-item read/dismiss/snooze~~ **— DONE (verified 2026-06-25):** per-item mark-read/unread + dismiss + snooze-1-day controls (`notifications.go:145`); MEASURED live: per-item action controls present on feed rows.
- [ ] **C269 [DESIGN]** "Notifications" missing from Settings jump-to tabs → add tab.
- [x] **R28 [RESEARCH]** ~~Alerts system spec~~ **— COMPLETE / implemented (assessed + verified 2026-06-25).** Research finding: the alerts system is built end-to-end. Mapping: **rules UI** = per-rule `alertRow`s in Settings (`settings.go:101`); **thresholds** = `RuleConfig` + `EffectiveThreshold` + 19 live inputs (C264); **new events** = `EventLowBalance` + `EventPaycheckLanded` with detectors (C265/C266); **live firing** = `runNotifyCatchUp` → `notify.CatchUp(EnabledRules(...))` at boot (boot-path hook panics fixed earlier, C270/C272); **unified badge** = `UnreadNotifyCount` + the C270 atom-sync fix; **severity** = `notifySeverityPill` (C267, 31 live). Consolidated C263–C268 closed above; C121/C122/C158/C159 addressed by the feed/severity/threshold work. 0 JS errors; build rc=0. (Residual: C269 "Notifications in settings jump-to tabs" — minor, left open.)

### Review F39 — "While you were away" digest (3/10, broken)
- [x] **C270 [MAJOR] ★ ROOT CAUSE of empty Notification Center — FIXED 2026-06-25** (fixes C121/C158/C159). The earlier "fix" (calling `UseNotifyFeed().Set` inside `PrependNotifyFeed`) actually made it worse: `runNotifyCatchUp` runs at boot (outside any component render), so every hook it touched — `UsePrefs()` (notifyrun.go:155,295), `UseNotice()` (notifyrun.go:108), and `UseNotifyFeed()` (via `PrependNotifyFeed` + the feed mutators) — panicked "GoUseAtom called outside component context", aborting catch-up before it wrote anything (the recover() at notifyrun.go:43 hid it; C272). Fix: route every boot-/handler-context atom write through the captured-atom pattern — `uistate.CurrentPrefs()` for week-start, `uistate.PostNotice()` for the summary toast, and a new `setNotifyFeed()` helper (captured `app:notify-feed` atom) replacing all four `UseNotifyFeed().Set(...)` calls in `notifyfeed.go` (PrependNotifyFeed + MarkFeedItemRead/DismissFeedItem/SnoozeFeedItem, which also run from non-render event handlers). MEASURED: cold boot WITH sample data (14 accounts / 2189 txns) → **0 "GoUseAtom outside component" panics**, 0 "runNotifyCatchUp panicked" logs, 0 console errors; health widget renders; build rc=0.
- [ ] **C271 [MAJOR]** No consolidated "while you were away" digest card/modal + no "since last visit" framing → dismissible dashboard catch-up card + labeled center section.
- [ ] **C272 [MINOR]** `runNotifyCatchUp` `recover()` swallows panics silently → add slog.

### Review F40 — Shared household access + roles (2/10)
- [ ] **C273 [MAJOR]** No role/permission model at any layer — domain.Member is {ID,Name,Color,IsDefault,Prefs}; IsDefault is a quick-add seed, not a role → add MemberRole (owner/admin/viewer) + enforce in entity access paths.
- [ ] **C274 [DESIGN]** No per-member login / access control / device user-switching (local-first single dataset) → add a local profile/PIN switch or explicitly surface the single-device limitation so users aren't misled.
- [ ] **C275 [MAJOR]** Add/Edit member forms have no role field → add a role selector to both.
- [ ] **C276 [MINOR]** Cosmetic "Default/Member" labels imply non-existent roles; member filter is display-only (no read-visibility enforcement) → remove misleading labels until roles exist; gate reads by role when implemented.
- [~] **R29 [RESEARCH]** Household roles/permissions + local multi-user — **SPEC delivered 2026-06-25** (research output below; implementation is follow-on).
  - **What already exists (assessed):** roles MODEL — `internal/memberrole` (`Owner`/`Admin`/`Viewer` + `Resolve` legacy-default + `Valid`/`ParseRole`/`Label` + predicates `CanManageMembers`/`CanEditEntities`/`CanViewOnly`), `domain.Member.Role`, store round-trip (C273); role SELECTOR UI in add/edit member (`members.go`, `memberaddform.go`, C275); a view-scope `ActiveMember` atom (a per-member *filter*, not an identity).
  - **The core constraint (the actual research finding):** CashFlux is local-first — the entire dataset lives UNENCRYPTED in one on-device SQLite/IndexedDB. So UI-level role gating is a **soft guardrail** (prevents accidental edits, tailors the view), NEVER a security boundary: anyone with device access can read the raw store regardless of role. A *real* per-member boundary requires per-profile encryption (separate encrypted stores keyed by a PIN/passphrase-derived key) — a large architectural change that also breaks household-wide aggregation (net worth across members). Conclusion: **do NOT market roles as security; ship them as collaboration guardrails**, and keep any PIN as an *app-open lock* (single shared device gate), not per-member data isolation. (Cross-ref R30 security hardening for the app-lock/KDF piece; per-member encryption is explicitly out of scope for the local build.)
  - **Recommended design — Phase 1 (soft guardrails, ~all the value, low risk):** (1) Add an **active identity** distinct from the view-filter: `uistate.ActiveIdentity` (the member operating the app), defaulting to the Owner; a header switcher to change it (optionally gated by an app-open PIN from R30, not per-member). (2) Wire the existing `Can*` predicates into a single seam: a `func canEdit(app) bool` / `canManageMembers(app) bool` helper read from `memberrole.Resolve(activeIdentity)`, and gate the entity mutation affordances (add/edit/delete buttons on accounts/txns/budgets/goals/rules + the Members screen) — render them disabled-with-tooltip ("Viewer — read-only") rather than hiding, so the role is legible. (3) Enforce defense-in-depth at the `appstate` mutation layer: `PutX/DeleteX` no-op + return a `ErrReadOnly` when the active identity is a Viewer (so a missed UI gate can't write). (4) Copy: a small "viewing as <member> · <role>" chip; an honest "roles guide collaboration on this shared device; they aren't a security boundary — your data stays local" note.
  - **Phase 2 (optional, only if real isolation is ever required):** per-profile PIN → Argon2id-derived key → separate encrypted dataset per member; household aggregation becomes opt-in/manual. Big change; defer unless demanded.
  - **Suggested implementation tickets (when picked up):** R29-identity (ActiveIdentity atom + switcher), R29-seam (`canEdit`/`canManage` helpers from active role), R29-ui (gate mutation affordances, disabled+tooltip), R29-enforce (appstate read-only guard + `ErrReadOnly` + tests), R29-copy (chip + honesty note). Pure-logic (`memberrole`) is already done & tested.

### Review F41 — Per-member views/allocations/privacy (5/10)
- [ ] **C277 [MAJOR]** Member views not visibly scoped — txns summary shows household total ("1725 shown") regardless of member; dashboard KPIs identical Everyone vs Marcus with no indicator → recompute summary from filtered subset (transactions.go:82-84); add "Showing X's activity" label.
- [ ] **C278 [MAJOR]** Accounts/budgets/goals/allocate don't scope by active member (UseActiveMember only in txns/dashboard/split/quickadd) → filter or badge by OwnerID across these screens (accounts.go, allocate.go).
- [ ] **C279 [MAJOR]** No income-allocation / fractional account ownership (binary Owner only) → optional AllocationShares sub-form (e.g. Marcus 60% / Priya 40%) feeding ledger.NetByOwner.
- [ ] **C280 [MINOR]** /members shows balance-sheet attribution only; reports.SpendingByMember exists but unwired → add a per-member "this month" income/spend row.
- [ ] **C281 [DESIGN]** No "Viewing as &lt;member&gt;" banner/framing → persistent scope badge when a non-Everyone member is active. (Privacy is display-only, no enforcement — see C274/R29.)

### Review F42 — Bank-grade security (5/10)
> Verified working: PBKDF2-600k→AES-GCM-256 full-dataset at-rest encryption, passcode lock gate (wrong rejected / right unlocks), manual + inactivity auto-lock, honest "forgot passcode" wipe.
- [ ] **C282 [MAJOR]** No biometric/WebAuthn unlock (B17.5 designed, unbuilt) → navigator.credentials.create() + PRF as a second unlock.
- [ ] **C283 [MAJOR]** No MFA for cloud/backend auth → surface MFA enrollment at the cloud layer; passkey = local 2nd factor.
- [ ] **C284 [MAJOR] ★security** Passcode gate hash is SHA-256 (applock/applock.go:58), not a memory-hard KDF → brute-forceable offline if localStorage is extracted; use the same PBKDF2-SHA256/Argon2id as the dataset key.
- [ ] **C285 [MAJOR]** App-lock section absent from settings jump-nav → add `applock.section` to settingsNavKeys (settingssectionnav.go).
- [ ] **C286 [MINOR]** Lock gate text low-contrast/invisible in dark mode (card text color falls through to white on a white surface) → scope card text color for dark.
- [ ] **C287 [MINOR]** No passcode-strength check — setup accepts "000000"; pwcheck exists but unwired → wire pwcheck.Validate(PIN); also show auto-lock timeout in the status line.
- [ ] **C288 [DESIGN]** No "Security" section heading/route → rename "App lock" to Security; consider /security.
- [~] **R30 [RESEARCH]** Security hardening — **SPEC delivered 2026-06-25** (assessment + phased plan; crypto changes left for a dev given migration sensitivity).
  - **What exists (assessed):** (a) **App-lock gate** — `internal/applock` (`Config`, `HashPasscode`, `Verify`, auto-lock idle window, hint that can't leak the passcode) + the unlock-gate UI (`app/applockgate.go`, `applocksettings.go`). (b) **Data-at-rest encryption** — `app/datasetcrypto.go` derives an **AES-GCM-256** key from the passcode via **PBKDF2-SHA-256 @ 600,000 iterations** (`cryptobox.PBKDF2Iterations`, OWASP-tuned), encrypting the dataset + artifacts (`artifactcrypto.go`); the derived key never leaves the JS runtime.
  - **Key finding / framing:** the **real** confidentiality boundary is the dataset crypto, and it's *already strong* (PBKDF2-600k → AES-GCM-256). The **gate** `HashPasscode` is plain **SHA-256(salt+passcode)** — a *fast* hash — but it only guards the UI gate, not the ciphertext, so it's a UX lock, not the security boundary. MFA is largely **N/A for a local single-device app** (it's a server-auth concept) — relevant only to the hosted sync tier (cross-ref R32).
  - **Recommended remediation (phased):** **P1 (low-risk, high-value):** passcode **strength meter + min-length** on set (`applocksettings.go` + an `applock.PasscodeStrength(s)` pure helper: length/charset/entropy bands; reject trivial 0000-style) — additive, no migration. **P2:** strengthen the **gate KDF** — verify the passcode through the same PBKDF2-600k path (or Argon2id via a wasm lib) instead of SHA-256, with a one-time migration of stored hashes on next successful unlock (keep SHA-256 verify as a fallback during migration). **P3 (optional):** **passkey/WebAuthn** unlock via `navigator.credentials` + the **PRF extension** to wrap/unwrap the AES data key (so a passkey can unlock the *data*, not just the gate) — platform-gated, falls back to passcode. **P4:** MFA only as part of the hosted sync tier (R32), not the local build.
  - **Suggested tickets:** ~~R30-strength (meter+min-length+tests)~~ **✅ SHIPPED 2026-06-25** — pure `applock.PasscodeStrength` (TooShort/Weak/Fair/Strong by length + char-variety; demotes trivial all-same/sequential like "1234"/"4321") + `MinPasscodeLength=4`, 13 table-driven tests (all pass); wired into the set-passcode submit (`applockgate.go`) to reject `StrengthTooShort` with new i18n `applock.tooShort`; build rc=0, app health 0 real errors. (Live raw-JS modal drive was harness-limited; behavior is unit-tested + build-verified.) Remaining: R30-gatekdf (PBKDF2/Argon2id gate + hash migration), R30-passkey (WebAuthn-PRF data-key wrap), R30-sync-mfa (defer to R32). The strong part (dataset AES-GCM/PBKDF2-600k) needs no change.

### Review F43 — Privacy stance / local-first (3/10)
- [x] **C289 [MAJOR]** ~~No user-facing privacy/local-first trust statement~~ **— DONE (verified 2026-06-25, R34-trust):** added an always-visible **rail-footer trust line** — "Private — your data stays on this device." (`shell.go` rail footer, new i18n `trust.localFooter`) — surfacing the core differentiator outside the admin console. MEASURED live both themes: renders + AA-clean (dark `#ababb3`/#0e0e0f = **8.46:1**, light `#56565c`/#f1f1f2 = **6.46:1**), build rc=0, 0 JS errors. (Hero + sample-banner placements remain optional follow-ons; the always-visible footer covers the differentiator app-wide.)
- [ ] **C290 [MAJOR]** No About/Privacy page/route or footer link (/privacy loads the dashboard; no &lt;footer&gt;) → add /about or /privacy (server-rendered or router) from docs/LEGAL_COMPLIANCE.md; link from the settings footer.
- [ ] **C291 [MAJOR]** Cloud sync section discloses nothing about what data leaves on sync → one-line disclosure under the backend toggle ("syncs encrypted snapshots; nothing leaves without this toggle").
- [ ] **C292 [MINOR]** AI-key disclosure + cloud trust line buried/conditionally hidden (cloud trust line only renders when CloudSelected) → surface at the Insights gate + Documents header; make the cloud trust line always visible.
- [ ] **C293 [DESIGN]** About surface is just version + changelog → expand: "Local-first · data stays on device · no account · export anytime".

### Review F44 — Data ownership: export/delete (8/10)
> Verified: Export JSON + CSV downloads fire; import round-trips losslessly; palette "Back up everything" + restore; wipe modal with Cancel. Read-only bank connections = N/A by design.
- [ ] **C294 [MAJOR]** Manual Export JSON calls `ExportJSON()` not `ExportJSONWithBlobs()` (settings.go:914) — receipt/document images excluded, so a "backup" can't self-restore images on a fresh device → switch to ExportJSONWithBlobs() (or warn).
- [ ] **C295 [MAJOR]** Import dataset overwrites all data with NO confirmation (importJSON settings.go:965-980 lacks confirmModal, unlike restore) → add a "this replaces your current data — continue?" modal.
- [ ] **C296 [MINOR]** CSV export is transactions-only but unlabeled, implying a backup → label "Export transactions (CSV)" + note JSON is the complete backup.
- [ ] **C297 [MINOR]** "Back up everything" (lossless multi-workspace) is palette-only, absent from Settings → Data → add a button/hint.
- [ ] **C298 [MINOR]** Settings "Data" not in jump-nav (buried below AI/Cloud); wipe confirm button labeled generic "Confirm" → add Data jump-link; relabel to "Wipe data".
- [ ] **C299 [DESIGN]** No "last backed up" timestamp shown (recordBackupNow stamps it but the UI never surfaces) → show "Last backed up: &lt;date&gt;" beside Export.

### Review F45 — Honest pricing / free tier / no dark patterns (6/10)
> Positives: free tier is genuinely generous (all core budgeting is local + ungated); UpgradeSheet is calm (no fake urgency, "Maybe later").
- [ ] **C300 [MAJOR]** No pricing page / price disclosure outside the one-shot UpgradeSheet (price strings en.go:951-953 render only in the sheet; no Plans tab) → add a "Plans" surface / "Cloud · $34.99/yr" in Settings; show price on every prompt.
- [ ] **C301 [MAJOR]** Upgrade path is one-shot — cloudmention.go writes `cloud-mention-dismissed` on BOTH buttons; the UpgradeSheet (sole caller cloudmention.go:38) is then permanently unreachable → add a persistent "View plans / Add Cloud" CTA in Settings → Cloud.
- [ ] **C302 [MAJOR]** No discoverable manage/cancel/downgrade surface — cancel routes via Stripe portal (billing_http.go:131 needs StripeCustomer); subscription banner only renders trialing/past_due/canceled → add a "Manage subscription" link in Settings → Cloud visible even to non-subscribers.
- [ ] **C303 [MINOR]** Free-vs-paid boundary + 14-day trial never stated in plain language in-app (cloud.benefit*/cloudTrialNote locked behind the unreachable sheet) → add "Free forever: budgeting/goals/reports · Cloud $34.99/yr: sync/backup/AI · 14-day trial" to the Cloud tab.
- [ ] **C304 [DESIGN]** Cloud & server tab is raw infra config (URL/token/test/deploy), not a billing surface → lead with plan status (tier/price/trial); collapse URL/token under Advanced.
- [~] **R31 [RESEARCH]** Pricing/plan UX — **SPEC delivered 2026-06-25.**
  - **What exists (assessed):** `UpgradeSheet`/`ShowUpgradeSheet` (`app/upgradesheet.go`) — benefits + annual-first price + "Start free trial" that opens Cloud settings → Stripe Checkout; `subscriptionbanner.go` (trial/upgrade prompt); `cloudmention.go`; a Cloud section in Settings hosting the trial→Stripe flow; admin console (`screens/admin.go`).
  - **Scope reality (the key finding):** the paid offering = the **hosted Cloud tier** (sync + backup + AI proxy), which CLAUDE.md marks **out of scope for the local build** (needs a hosted backend). So R31 is mostly **blocked on R32's hosted tier** — the *local* app can show pricing/benefits and start a trial, but a true Plans/manage/cancel surface requires the billing backend (Stripe customer portal). Don't build a fake manage/cancel locally.
  - **Gaps + recommended plan:** (1) **Visible Plans** — a `/plans` (or Settings→Plans) comparison: **Free (local-first, on-device, $0)** vs **Cloud (sync/backup/AI, $X/yr)** feature matrix; re-uses the upgrade-sheet copy as the source of truth. (2) **Free-vs-paid clarity** — a small "Free plan" chip near the household/sync status; AI/sync features already cost-label via the SMART catalog (Free/AI tiers) — extend the same honesty to Cloud-gated features. (3) **Re-engageable upgrade** — a persistent entry (Settings→Plans + an optional dismissible nudge) so upgrade isn't only reachable by hitting a gated action. (4) **Manage/cancel** — link to the **Stripe customer portal** for subscribed users (backend-dependent; stub "Manage subscription" → opens portal URL from the backend). (5) **C5** — the "Synced" pill must read "Local" / "Free" on a local-only session (it currently shows "Synced" misleadingly); fix when the plan-state is surfaced.
  - **Suggested tickets:** R31-plans (Plans comparison surface), R31-chip (free/local plan chip + C5 fix), R31-reengage (persistent upgrade entry), R31-portal (Stripe manage/cancel link — needs R32 backend). Gated on R32 for anything beyond the local marketing/trial-start UI.

### Review F46 — Cross-platform native + web sync (3/10)
- [x] **C305 [BLOCKER] ★ LIVE REGRESSION — FIXED 2026-06-25.** Dashboard panicked on load — GWC-RUNTIME-PANIC "GoUseAtom called outside component context" at uistate/healthtrend.go:31 → screens/health.go:306: the health widget's snapshot-recording `UseEffect` called `RecordHealthSnapshot`, which called the `UseHealthTrend()` hook inside the effect body. Fix: applied the captured-atom pattern (mirrors `notice.go`/`notifyfeed.go`) — `UseHealthTrend()` now captures the atom into a package var during render, and `RecordHealthSnapshot` pushes via that captured reference (`capturedHealthTrend.Set`) instead of re-calling the hook. MEASURED: dashboard loads with the health widget (score "30"), `/health` round-trip works, **0 page errors · 0 console errors · 0 GWC hook/panic errors** (`e2e/screenshots/health_panic_fixed.png`); build rc=0.
- [x] **C306 [MAJOR]** PWA not installable — manifest.webmanifest had `icons:[]` + no favicon/apple-touch-icon. **DONE 2026-06-25** (favicon + full raster icon set + og:image; details below). **PARTIAL (step 1):** created on-brand **`web/favicon.svg`** (green `#2e8b57` rounded square + dark-green "C", mirroring `.brand-mark`) — fixes the missing browser-tab favicon (was `favicon.ico` 404 / generic icon). Wired `<link rel="icon" type="image/svg+xml">` + `<link rel="mask-icon">` in `index.html`, added one SVG entry to the manifest `icons` array (`type:image/svg+xml, sizes:any, purpose:any` — Chrome/Edge accept this for install), precached `favicon.svg` in the SW (cache v269→v270). MEASURED: `favicon.svg` serves 200 `image/svg+xml`; both links present in served HTML + resolved in DOM; manifest valid JSON with the icon (served); renders as the brand "C" (`e2e/screenshots/favicon_render.png`); app boots clean, 0 JS errors; build rc=0.
  - **✅ COMPLETED 2026-06-25 (raster icons + og:image).** Rasterized the brand mark from the SVG via
    headless Chromium (no extra tooling) into full-bleed maskable-safe PNGs: **`icon-192.png`**,
    **`icon-512.png`**, **`apple-touch-icon.png`** (180). Manifest `icons` now has SVG (any) + 192 + 512
    both `purpose:"any maskable"`; `index.html` gained `<link rel="apple-touch-icon">`, a 192 PNG
    `<link rel="icon">`, and a real raster `og:image`/`twitter:image` (`icon-512.png` + width/height).
    Precached all PNGs in the SW (cache v270→v271). MEASURED: all three PNGs serve 200 `image/png` at
    correct dims (192/512/180); 7 icon/og head tags in served HTML; manifest valid JSON with 3 icons
    (any/any, 192 any-maskable, 512 any-maskable); apple-touch-icon + png192 + og:image all resolve in
    DOM; rendered icon = on-brand full-bleed green "C" (`e2e/screenshots/` icon render); app boots clean,
    0 JS errors; build rc=0. PWA is now install-ready with proper icons on Chrome/Edge/Android (maskable)
    and iOS (apple-touch-icon); link previews carry a real image. (`apple-mobile-web-app-capable` already
    present; modern iOS auto-derives the splash from the icon + theme/background colors.)
- [ ] **C307 [MAJOR]** Install prompt captured (beforeinstallprompt) but never exposed — no Install button; window._installPromptCaptured undefined → wire the deferred prompt to a visible "Install app" affordance + iOS fallback.
- [ ] **C308 [MAJOR]** No native iOS/Android app (web/WASM only) → acknowledge the trade-off; consider a Capacitor shell.
- [x] **C309 [MAJOR]** ~~Sync conflict resolution silently drops rejected local pushes~~ **— FIXED (silent-loss eliminated, 2026-06-25):** root cause was `flushBackendSyncQueue` calling `removeQueuedSyncMutation` *before* the `!resp.Accepted` check — a server-rejected (LWW-lost) push was dequeued and the local edit vanished with only a toast. Fix: (1) dequeue **only on `resp.Accepted`**; (2) on conflict, first `saveConflictBackup(item)` to a recoverable per-workspace slot (`cashflux:sync-conflict:<ws>`), then remove from the active queue (removal is still required, else it re-pushes/re-loses forever — an infinite conflict loop); (3) a clear toast (`sync.conflictBackedUp`) pointing to **Settings → Cloud sync**, where a new **Restore / Discard** affordance (`settings_section.go`, gated on `hasConflictBackup`) lets the user re-apply the saved copy (`restoreConflictBackup` re-stamps the client time so it wins the next round, re-enqueues, flushes) or discard it. So a local write is **never silently discarded** — it's backed up + surfaced + recoverable. Build rc=0, `go test ./internal/i18n` ok. MEASURED live: app boots on `/settings`, the conflict-restore row is correctly absent with no backup, 0 errors. *(The full server-rejection round-trip needs a live backend the local harness can't run; verified by the reorder logic + the no-backup UI state + clean boot. Field-level/3-way merge (R32-merge) and the chip "tap to resolve" state (R32-conflict-ui) remain as enhancements — this closes the data-LOSS bug.)*
- [ ] **C310 [DESIGN]** Real-time sync requires a self-hosted backend (no hosted tier); no multi-device onboarding / "add a device" flow → hosted option or explicit no-backend state + add-device wizard. ("Synced" with no backend = C5.)
- [~] **R32 [RESEARCH]** Cross-platform + sync — **SPEC delivered 2026-06-25.**
  - **What exists (assessed):** a real sync stack is already built — `internal/syncbridge/client.go` (client), `internal/syncstate/syncstate.go` (state), `internal/app/sync_client.go` + `syncchip.go` (UI), `internal/server/{sync,sync_grpc,grpcbridge}.go` (server), `internal/backendrpc/pb/...` (protobuf). **PWA installability is DONE** (C306 — favicon + 192/512 maskable icons + apple-touch-icon + manifest, this series).
  - **Gaps:** (1) **Field-level conflict resolution is the real open risk** — currently a known **silent-loss** bug (**C309**: `sync_client.go:168-178` dequeues before the conflict branch), i.e. effectively last-write-wins with possible drops. (2) **Native shell** (desktop/mobile wrapper) — out of scope for the local/web build (cross-ref the native-shell ticket); the PWA is the cross-platform vehicle for now.
  - **Recommended plan:** **P1 (correctness, do first):** fix C309 — don't dequeue an op until its push is acknowledged or its conflict is resolved; add a server-versioned **per-record revision** (or updated-at) and a deterministic merge: field-level last-writer-wins *with* a conflict log the user can review (never silently drop a differing field). **P2:** per-entity merge for the few structured types (split/tags/custom fields) where field-LWW is lossy → 3-way merge against the common base. **P3:** native shell only if a true app-store presence is wanted (the installable PWA already covers desktop+mobile home-screen). **P4:** load/soak the sync path (push/pull, blob up/down, AI streaming, WatchWorkspaces fan-out) — see the existing soak-test backlog item.
  - **Suggested tickets:** R32-conflict (C309 fix + per-record revision + conflict log + tests — highest priority), R32-merge (3-way merge for split/tags/custom), R32-soak (load tests), R32-native (defer). PWA part = closed via C306.

### Review F47 — Offline / PWA + performance (3/10, offline broken)
- [ ] **C311 [MAJOR]** Offline reload returns a BLANK page — offline boot broken: SW handleNavigate uses `{cache:"no-store"}` (throws offline), then appShell()'s `./index.html` cache lookup also misses → empty 504 → audit SW install/precache; test offline end-to-end before shipping.
- [ ] **C312 [MAJOR]** Wasm never cached by the SW — install `c.add(u).catch(()=>{})` silently swallows the 60 MB wasm fetch failure → offline /bin/main.wasm returns false → make the failure visible; retry / dedicated large-asset cache.
- [ ] **C313 [MAJOR]** SW active but not controlling the page on first load (clients.claim loses the race vs window.load) → controls only on 2nd load → register earlier or handle the first-load case.
- [ ] **C314 [MINOR]** 60 MB uncompressed wasm, no gzip/brotli at serve (serve.go sets no Content-Encoding; ~48s cold load @10Mbps vs ~12s compressed) → compress wasm + add a load progress bar. *(Panic C305, PWA icons/install C306/C307 reconfirmed here.)*

### Review F48 — Exceptional accessible UX (5/10)
- [ ] **C315 [MAJOR]** a11y missing/incorrect accessible names: icon-only sidebar buttons (.rail-section, .menu-btn) use `title` only (no aria-label); a bare `<span aria-label>` without a role (aria-prohibited-attr); dashboard SVG chart has role=img but no `<title>` → add aria-labels mirroring titles, role="img" on the span (or convert to SVG `<title>`), and a `<title>` on the chart SVG.
- [ ] **C316 [MAJOR]** Contrast: banner text fails WCAG AA (3.91:1 — #ababb3 on #205337 @13.76px) → darken fg / lighten bg to ≥4.5:1.
- [ ] **C317 [MAJOR]** No discoverable theme toggle — app boots `data-theme="dark"` with no light/dark control found on dashboard/settings → verify the Appearance control surfaces it; add a labeled theme toggle in topbar/Appearance.
- [ ] **C318 [MINOR]** Segment control (Week/Month/Quarter/Year) role=radio buttons have no enclosing role="radiogroup" + label → wrap in `<div role="radiogroup" aria-label="Time period">`.
- [ ] **C319 [DESIGN]** Dashboard bento customize/reorder affordance not reachable via keyboard/visible controls → surface a visible "Customize" entry if reconfigurability is in scope.
- [~] **R33 [RESEARCH]** Full WCAG-AA a11y audit + remediation — **SPEC + partial remediation (2026-06-25).**
  - **Audit method (reusable):** drive each screen via Playwright in BOTH themes + reduced-motion; measure `getComputedStyle` contrast vs resolved bg (≥4.5 normal / ≥3.0 large), Tab-walk focus-visibility, DOM scans for unnamed controls / unlabelled SVGs.
  - **VERIFIED AA-CLEAN this session:** text contrast both themes — `--text-dim` (light 7.29), `--text-faint` (light 5.07 / dark 4.85), up/down/warn semantic text (4.04–7.90), primary CTA white-on-accent (5.07); **focus visibility** — every Tab stop shows a 2px outline (14/14 sampled); **accessible names** — 283 `aria-label` usages, **0 unnamed icon-only buttons** (29/29 named).
  - **DONE this pass (SVG titles, WCAG 1.1.1):** base `ui.Icon` now emits `aria-hidden="true"` by default (`internal/ui/icon.go`) — decorative icons no longer announce as unlabelled graphics; the name stays on the wrapping labelled control. MEASURED: 128/129 SVGs hidden, 29/29 icon-only buttons still named, `go test ./internal/ui/...` ok (updated `TextFaint` golden), build rc=0, 0 JS errors.
  - **REMAINING checklist (own tickets):** C315 (sidebar icon-buttons `title`-only; chart SVG `role=img` needs `<title>`), C56 (filter keyboard shortcut), C57 (filters badge aria-label), C65 (inline-edit/select labels), C66 ("/split" name collision), C249 (chat aria-labels). The 8 GWC-RUNTIME-PANIC load errors (C305/C76) were fixed earlier.

### Review F49 — Connection/sync reliability (5/10)
> Works: offline-indicator pill (hidden online, shows offline, clears on reconnect); 5-state machine; LWW queue persisted; backend-active guard; watch/reconnect restart.
- [x] **C320 [MAJOR]** ~~False "Synced" chip when no backend was ever configured~~ **— FIXED (verified 2026-06-25):** `loadSyncStatus()` now short-circuits to an empty state when `!BackendActive()`, so a local-first session (or a backend that was configured then disabled — discarding any stale `"synced"` in localStorage) reports nothing and `SyncChip` stays invisible; `syncStatusLabel()` empty/`local` → "Saved on this device" (no false cloud claim). MEASURED live: sync-chip count 0, no standalone "Synced" text, build rc=0, `go test ./internal/app` ok, 0 errors. (Supersedes C5.)
- [x] **C321 [MAJOR]** ~~SyncChip has no data-testid~~ **— DONE 2026-06-25:** added `Attr("data-testid","sync-chip")` + `Attr("data-sync-state", st.State)` to the chip button (`syncchip.go`) so the e2e suite can target it and assert its state. Build rc=0.
- [x] **C322 [MINOR]** ~~No exponential backoff~~ **— DONE 2026-06-25:** replaced the fixed 10s/3s sleeps in `startBackendWatch` with exponential backoff + jitter (2s→120s cap, ±30%) via a new pure, unit-tested `internal/backoff` package (`Delay` + `Jitter`, 3 tests incl. overflow + bounds); `attempt` resets to 0 on a healthy stream so a brief blip recovers fast, while a persistent outage backs off to the cap instead of dialing every 3–10s. Jitter decorrelates many clients reconnecting at once. Build rc=0; `go test ./internal/backoff ./internal/app` ok; app boots clean, 0 errors.
- [x] **C323 [MINOR]** ~~No `offline` event handler~~ **— DONE 2026-06-25:** added a window `offline` listener alongside the existing `online` one (only wired when `BackendActive()`); on disconnect it sets `State="offline"` with the live queue depth via `setSyncStatus`, so the chip reflects the drop immediately instead of lingering on its last state until the next failed dial. Build rc=0.
- [x] **C324 [MINOR]** ~~Sync status not reactive~~ **— DONE 2026-06-25:** added a captured revision atom (`sync:rev`) that `setSyncStatus` bumps and `SyncChip` subscribes to via `state.UseAtom`, so background-goroutine status changes (watch/flush/pull) re-render the chip immediately instead of waiting for an unrelated render. Uses the project's captured-atom pattern (capture during render, `.Set` from out-of-render callers; no-op until mounted) to avoid the "GoUseAtom outside component context" panic. Build rc=0, `go test ./internal/app` ok, app boots clean, 0 errors. *(Conflict silent-loss = C309, sync_client.go dequeues before the conflict branch; onboarding = C310; panics = C305/C76.)*

### Review F50 — Support + in-app help + roadmap (2/10)
> Works: keyboard-shortcut overlay ("?") + command palette (Ctrl+K) exist and function.
- [x] **C325 [MAJOR]** ~~No in-app support contact / bug-report / feedback path~~ **— DONE 2026-06-25:** the command palette now answers "support"/"contact"/"feedback"/"bug"/"report"/"docs"/"faq" with a Help & support command that routes to `/help` (MEASURED live: typing "bug" surfaces Help); the actual bug-report payload (app version + in-app log ring → clipboard) shipped under R34 (`settings.go` `copyBugReport`). Build rc=0, 0 errors. *(A direct GitHub-Issues/mailto deep link can layer on later, but the path is no longer dead.)*
- [x] **C326 [MAJOR]** ~~No in-app roadmap / what's-new / changelog~~ **— DONE 2026-06-25:** two surfaces now — (1) R34's "What's new" card on `/help` (version + highlights + full-changelog link), and (2) a once-per-version **boot toast** (`whatsnew.go` `whatsNewToastOnBoot`): on a version bump it posts "CashFlux updated to vX — see what's new in Help" then advances the stored seen-version (idempotent); brand-new users get no toast (first run only records the version). Post is deferred ~1.2s so the toast surface has captured its atom (the boot point is too early — `PostNotice` would no-op). MEASURED live: fresh context → no toast; seeded older version → toast "updated to v0.1.0" shows once; 0 errors, build rc=0. *(A dedicated `/changelog` route with full rendered history can layer on later; the discovery + version-bump need is covered.)*
- [x] **C327 [MAJOR]** ~~Shortcut help undiscoverable + palette "help" returns nothing~~ **— DONE 2026-06-25:** added a visible "?" (HelpCircle) top-bar button (`HelpButton`, `data-testid="help-button"`, aria-labelled) that routes to `/help`; the palette now returns the Help center for "help"/"support"/etc. (the keyboard `?` overlay still toggles via the Keyboard-shortcuts command). MEASURED live: help button present, click → `/help` (h1 "Help"), palette "bug" → Help visible, 0 errors, build rc=0.
- [x] **C328 [MAJOR]** ~~No help center / docs / FAQ / contextual help~~ **— DONE 2026-06-25:** the `/help` center (R34: topics, what's-new, setup checklist, privacy) is now reachable from a persistent palette "Help & support" entry (keywords incl. docs/faq/guide) AND the new top-bar "?" button (C327). MEASURED live (see C327). *(Decoupling key-stat tooltips from the Smart gate is a separate, smaller follow-up — leaving that sub-item noted but not blocking the help-center close.)*
- [x] **C329 [DESIGN]** ~~No onboarding tips / feature-discovery~~ **— DONE 2026-06-25:** added a dismissible **first-run onboarding callout** at the top of the dashboard (`screens/dashboard_onboard.go` `dashOnboardCard`): a live setup checklist (Add an account / Record a transaction / Set a budget / Set a savings goal) with ✓/○ from actual data, each pending step a one-click jump to where it's done, plus a "Take the tour" button → `/help` and a "Dismiss". Self-hides once every step is complete (no nagging) OR on dismissal (persisted in `browserstore`, survives reload). Per-row click handlers are isolated in an `onboardRow` component (GWC no-On*-in-loop rule). i18n `onboard.*`. MEASURED live: on an empty dataset the card shows with all steps ○ + tour button; dismiss removes it and it stays gone after reload; with sample/complete data it is correctly absent; 0 errors, build rc=0. *(A fuller per-screen "did you know" tip-card tour can layer on later; the core first-run guidance is in place; setup-wizard tie = C21, settings-route = C84.)*
- [x] **R34 [RESEARCH]** Help/support/trust surface — **SPEC delivered 2026-06-25.**
  - **What exists (assessed + verified 2026-06-25 — more than expected):** ✅ **discoverable shortcuts** — `?` opens a `#cf-help-overlay` cheatsheet (MEASURED: visible, 9 rows "Jump to a section Alt+1–9 / Add a transaction Alt+N / Command palette Ctrl/⌘K…"); ✅ **command palette** — Cmd/Ctrl+K opens it (MEASURED: opens); ✅ **trust line** — shipped this session (C289, rail footer); the logging package keeps an in-app ring buffer (usable for a bug report). **Still missing:** `/help` topic center, feedback/bug-report form, what's-new/roadmap, onboarding tour.
  - **Recommended plan (all local-first, no backend needed):** (1) **Help center** — a `/help` route (Group System): short plain-English topics (getting started, importing CSV, budgets, the SMART layer, your-data-stays-local) sourced from static content; reuse the card/section chrome. (2) **Discoverable shortcuts** — a `?`-triggered shortcuts cheatsheet modal listing what `shortcuts.go` already binds (single source of truth: render from the shortcut registry). (3) **Bug report / feedback** — a form that bundles the in-app log ring buffer + app version + (opt-in) a redacted state summary into a copy-to-clipboard / mailto payload (no server: respects local-first; the user sends it). (4) **What's-new** — surface `CHANGELOG.md`'s Unreleased/last-release section in a dismissible "What's new" sheet keyed on version. (5) **Trust line** (C289) — a visible "Your data stays on this device — nothing is uploaded" line on the dashboard/sidebar footer + sample banner (the core differentiator, currently invisible outside admin). (6) **Onboarding** — a light first-run checklist (add account → add income → set base currency), cross-ref C21/C31.
  - **Suggested tickets:** ~~R34-help (/help route + topics)~~ **✅ SHIPPED 2026-06-25** — `/help` route in the System nav (`screens/help.go`, i18n `nav.help`/`screen.helpSub`), 6 plain-English topic cards (Getting started · Bringing in your data · Budgets/goals/reports · The Smart layer · Keyboard shortcuts · Your privacy); MEASURED live: in nav, navigates to /help, all 6 cards render, 0 JS errors, build rc=0. ~~R34-shortcuts (`?` cheatsheet)~~ **✅ DONE (verified)**, ~~command palette~~ **✅ DONE (Ctrl/⌘K)**, ~~R34-feedback~~ **✅ SHIPPED 2026-06-25** — "Copy bug report" button (Settings → Debug log) bundles app version + the in-app log ring to the clipboard, local-first (`settings.go` `copyBugReport`); MEASURED live: copied "CashFlux v0.1.0 / 3 log entries…" + toast, 0 errors. ~~R34-whatsnew~~ **✅ SHIPPED 2026-06-25** — "What's new" card atop /help (version + recent highlights + full-changelog link); MEASURED live: present, version shown, link resolves, 0 errors, ~~R34-trust (C289 trust line)~~ **✅ SHIPPED** (rail-footer privacy line, AA both themes), R34-onboard (first-run checklist, C21/C31). **Remaining: what's-new sheet + onboarding** (help/shortcuts/palette/trust/feedback all done). All local-first; none require the hosted backend.

### ★ Implementation-ready fixes (from research diagnoses, 2026-06-25)
> Precise, ready-to-build edits surfaced by the R-series root-cause diagnoses. Each is a concrete change.
> (R2→C223 and R3→C25 are already implemented+committed; C101/C305/C311/C121/C159/C270/C59/C76/C106 verified resolved in the current build.)
- [x] **IMPL C92 (from R9)** — DONE 2026-06-25: CSV importer suspends triggers manually then fires per imported row with full context (replacing WithoutTriggers nil aggregate); txn_*-conditioned workflows now route/flag imported rows. Tests added. (Document-import path unchanged this pass.) — `internal/appstate/appstate.go`: stop calling `RunTriggered(workflow.TriggerTxnAdded, nil)` inside `WithoutTriggers` (it strips the txn context). In `ImportTransactionsCSV` (~:204) and `ImportReviewedDocumentRows` (~:781), after the bulk write, loop the imported txns and call `a.RunTriggered(workflow.TriggerTxnAdded, &t)` so conditioned workflows fire with full `txnContext`. Fixes: imported transactions currently SILENTLY skip all `txn_*`-conditioned workflow triggers.
- [x] **IMPL C32** — DONE 2026-06-25: ruleaddform consumes RuleDraft once on mount (UseEffect) → seeds match/category → ClearRuleDraft. — `internal/screens/ruleaddform.go`: read `uistate.UseRuleDraft()` on first render, seed `match`/`categoryID` state from it, then `uistate.ClearRuleDraft()`. The atom is already captured (dialoghost.go:48-49) and set from transactions.go:202; the form just never reads it. Unblocks the "Always categorize like this" correction→rule shortcut. (~1-line read + seed.)
- [x] **IMPL C86** — DONE 2026-06-25: per-account dedupe.Signature seen-set skips already-present rows; re-import adds 0. New pure dedupe.Signature shared with FindDuplicates. — `internal/appstate/appstate.go` `ImportTransactionsCSV` (~:170-216): build a seen-signature set for the target account (mirror the dedup in `ImportReviewedDocumentRows`:769-776) and skip already-present rows, so re-importing the same CSV no longer doubles every transaction.
- [x] **IMPL C132** — DONE 2026-06-25: budgets loop evaluates rollover budgets against effectiveLimit=Carryover(prev.Remaining,b.Limit); badge=prev.Remaining unchanged. — `internal/screens/budgets.go` (~:209-214): when `b.Rollover`, compute `effectiveLimit = budgeting.Carryover(prev.Remaining, b.Limit)` and pass `effectiveLimit` into `EvaluateRollup` (currently the raw `b.Limit` is used) so Remaining/Percent/State/bar reflect the carry; derive the badge from `effectiveLimit - b.Limit`. (`Carryover()` exists in rollover.go:22 but is never called — rollover is decorative.)
- [ ] **IMPL C106-hardening (from R11)** — `web/index.html`:1750: add `pointer-events:none` to base `.flip-backdrop` and `pointer-events:auto` to `.flip-backdrop.show`. Defense-in-depth so an orphaned backdrop can never block clicks even if the unmount path regresses.

### ★ IMPL plan — C58 split-transaction (from R6 design)
> R6 also found a CURRENT correctness bug: receipt-imported splits have `CategoryID==""`, and neither budgets (`budgeting.go:55-86`) nor reports (`reports.go:37-49`) walk `t.Splits` — so split transactions are **silently invisible** to both today. Steps 1-2 fix that independent of the manual UI. Domain `CategorySplit` + `Transaction.Splits` + helpers (HasSplits/SplitsReconcile/SplitsTotal, category_split.go) and store JSON round-trip already exist — skip.
1. [x] **C58-logic** — **DONE 2026-06-25:** `reports.categoryTotals` + `budgeting.spentCovered` attribute split lines per-category when `t.HasSplits()` (never via the whole-txn category → no double count); `matchesCovered` refactored into `matchesScope` (expense/date/member) + per-line category test. Fixes receipt-split invisibility. Tests: `TestSpendingByCategorySplits`, `TestSpentSplitTransactionAttributesPerCategory`. Pure, native-green.
2. [ ] **C58-appstate** — new `appstate/splits.go`: validate `domain.SplitsReconcile(amount, splits)`, set/clear `Splits` and recompute `singleCategory()` before `PutTransaction` (mutually exclusive single-cat vs split paths).
3. [ ] **C58-editor** — new `internal/screens/split_editor.go`: per-line `[category select][amount][× remove]` + running remainder (via SplitsReconcile, green/red) + "+ Add line". AVOID `On*` handlers inside the line loop (pre-index/delegate — GWC rule).
4. [ ] **C58-wire** — `transactions_row.go`: "Split across categories" toggle in inline edit (hide the single-category select in split mode); auto-open split mode when `HasSplits()`; disable for transfers; block Save until remainder==0 and every line has a category; add a ledger "Split (N)" badge in the category cell.
5. [ ] **C58-test** — integration: manual split round-trip + reports/budgets attribution across categories.

### ★ IMPL plan — R21 loan amortization (C204-C207, from R21 design)
> All additive; engine is pure + native-testable; Account JSON fields round-trip with no migration; existing payoff BuildPlan untouched. Engine + tests FIRST.
1. [ ] **R21-enum** — `domain/enums.go`: add `IsInstallment()`/`IsRevolving()` on AccountType (loan/personal_loan/mortgage = installment) + test.
2. [ ] **R21-fields** — `domain/entities.go`: add `LoanTermMonths int`, `OriginalBalance money.Money`, `LoanStartDate time.Time` (json omitempty, liability-only); store round-trips automatically.
3. [ ] **R21-engine** — NEW `internal/payoff/amortization.go` (pure, native-test FIRST): `Row{PaymentNo,Date,Payment,Principal,Interest,Balance}`; `AmortizeFixed(balance,aprPct,termMonths)`; `AmortizeWithExtra(...,extraPerMonth)` (clamp final payment to balance, cf payoff.go:79); `Summary(rows)→(totalInterest,totalPaid,payoffDate)`; fallback = simulate from MinPayment when term==0. + amortization_test.go.
4. [ ] **R21-forms** — `accountaddform.go` + `accounts_row.go`: term fields in the liability sub-form/inline edit, shown only when `isLiab && type.IsInstallment()` (append hooks at the END of the fixed hook sequence; never conditional).
5. [ ] **R21-panel** — `accounts_row.go`: expandable loan-detail panel (overflow toggle, gated installment && LoanTermMonths>0): summary strip (payoff date / total interest / principal remaining), extra-payment input → AmortizeWithExtra recompute, balance-curve sparkline, first-12 schedule rows (+ Show all). Label schedule "from original terms"; show live ledger balance separately.
6. [ ] **R21-sample** — `store/sample.go`: add term fields to the 4 loans (mortgage 360/$230k/Jul'22; Marcus car 72/$38k/Jan'25; Priya car 60/$26k/Sep'25; student 120/$34k/Jul'22).
7. [ ] **R21-sample-transfers (C206, separate commit)** — convert the 4 loans' monthly payments from categorized expenses to checking→liability transfers (mirror the credit-card addTransfer) so balances actually amortize; re-check budget/chart history.

### ★ IMPL plan — R23 investment holdings/performance/allocation (C219-C222, from R23 design)
> Local-first, MANUAL price (no live feed; CurrentPrice+PriceAsOf are the seam a feed would later fill). Investment accts are balance-only today (no Holding type anywhere). Holdings live as a SEPARATE keyed entity (Dataset.Holdings []Holding), NOT Account.Holdings (mirrors Transactions). Pure calc package + tests FIRST.
1. [ ] **R23-calc** — NEW `internal/portfolio/portfolio.go` (pure, native-test FIRST): `HoldingValue(h)`, `UnrealizedGain(h)`, `ReturnPct(h)` (guard cost==0), `PortfolioSummary(holdings)→{value,cost,gain,returnPct}`, `AllocationWeights(holdings)→[]{label,weight}` (by holding + by asset class). Zero store/UI deps. + portfolio_test.go (zero shares, zero-cost guard, mixed classes).
2. [ ] **R23-domain** — `domain/entities.go`: add `Holding{ID,AccountID,Ticker,Name,Shares,CostBasis,CurrentPrice,PriceCurrency,PriceAsOf,AssetClass,Custom}` (PriceCurrency per Risk#multi-ccy); `domain/enums.go`: AssetClass consts (equity/bond/cash/real_estate/crypto/other). NAME the allocation type `PortfolioAllocation`/`HoldingWeight` — NOT AllocationProfile (collision w/ capital-allocator at entities.go:161).
3. [ ] **R23-store** — `store/dataset.go`: add `Holdings []domain.Holding`; `sqlitestore.go`: holdings table (JSON-blob pattern); `store/crud.go`: PutHolding/GetHolding/DeleteHolding/ListHoldings/ListHoldingsByAccount (mirror PutAccount ~line 121) + tests; confirm Dataset export/import round-trips Holdings (+integration test).
4. [ ] **R23-staleness** — NEW `internal/portfolio/staleness.go`: `PriceStale(h,now,maxAge)`, `DefaultPriceMaxAge=7d` (mirror currency/staleness.go:22).
5. [ ] **R23-ledger** — NEW `internal/ledger/holdings.go`: `InvestmentBalance(acct,holdings)` = sum(shares×price in acct ccy via currency.ConvertBetween) when holdings exist, else fall back to tx-derived Balance. Thread `holdings []Holding` into `NetWorth()` (ledger.go:182), NetWorthSeries, NetWorthExplained — per-acct dispatch, NO double-count (holdings bypass tx-sum + setBalance adjustments). + tests both paths.
6. [ ] **R23-appstate** — `appstate.go`: load/save holdings via store; expose `Holdings(acctID)`, `PutHolding`, `DeleteHolding`; thread holdings into all `ledger.NetWorth` call sites.
7. [ ] **R23-ui-holdings** — `screens/accounts_row.go` (investment branch ~line 81): collapsible Holdings section — rows (ticker/name/shares/cost/price inline-edit/asOf/stale-nag), add-holding form (+asset-class dropdown), delete. Largest screen change. One-time migration prompt when adding first holding to an acct with a manual balance (Risk: silent overwrite).
8. [ ] **R23-ui-perf** — same panel: summary tile (value/cost/unrealized gain signed+colored/return%) from PortfolioSummary.
9. [ ] **R23-ui-alloc** — same panel: asset-class donut/bar from AllocationWeights (reuse existing chart component if any; else CSS bar stack for MVP).
10. [ ] **R23-networth-audit** — confirm accounts.go hero (~162-168) + dashboard net-worth widget pull the threaded NetWorth path; patch any balance-only investment path.
> Deferred (note, don't build now): live price-feed fetch (fxai.go-style), cost-basis lots/realized-gain (model is single aggregate CostBasis — unrealized only), retirement-vs-brokerage AccountSubType (C73), portfolio-allocation dashboard widget.

### ★ IMPL plan — R15 canonical safe-to-spend (C139-C146, from R15 design)
> ONE pure formula, NO Smart/AI gate. Canonical: `SafeToSpend = LiquidCash − BillsDueBeforeNextPayday − GoalContributions(prorated) − CommittedBudgets(default 0)`. Reuse ledger.LiquidBalance + budgeting.PeriodRange + bills/goals (do NOT reimplement — smartengine/bills.go:617 inline dup is the anti-pattern). Today 6 divergent formulas live across smart/planning/budgets/insights/aitools, none reliably surfaced.
1. [x] **R15-pkg** — **DONE 2026-06-25:** NEW pure `internal/safespend/safespend.go` (stdlib only, integer minor units): `Breakdown{LiquidCash,BillsDue,GoalContributions,CommittedBudgets,SafeToSpend,IsNegative,Currency}`; `Compute(liquid,bills,goals,budgets,currency)→Breakdown` (nets liquid − the 3 commitment buckets; clamps negative buckets to 0; liquid may be negative; SafeToSpend may go negative→IsNegative); `ComputeCategory(remaining,daysLeft,daysInPeriod)→int64` (even-pace = remaining×daysLeft÷daysInPeriod, floored, guards daysInPeriod≤0 / daysLeft≤0 / remaining≤0, clamps daysLeft≤period). + `safespend_test.go` (positive/zero/negative/overdrawn/bucket-clamp/prorate/div0/floor — all pass). `go test ./internal/safespend` ok; build rc=0. **Next:** R15-inputs (derive the buckets) then the dashboard/planning/budgets wiring (R15-dashboard … R15-i18n).
2. [ ] **R15-inputs** — NEW `internal/safespend/inputs.go`: `BillsDueBefore(bills,now,horizon,toBase)` (reuse smartengine/budgets.go:232-242 pattern; horizon = period-END from PeriodRange, NOT hardcoded month-end); `GoalContributionsProrated(goals,now,periodStart,periodEnd,toBase)` (budgets.go:244-258, prorated). time.Time injectable → testable.
3. [ ] **R15-dashboard** — `screens/dashboard.go`: glanceable "Safe to spend" stat tile near cash-flow KPIs via Compute; red "−$X over" when IsNegative; NO smart import (also fixes C145 anchor).
4. [ ] **R15-planning** — `screens/planning.go:404-459`: pass `safespend.Compute(...).SafeToSpend` (liquid-based) into afford.CanAfford instead of net.Amount (net-worth basis = wrong, C141); rename planning.affordAvailable "Free to spend"→"Safe to spend" (C142).
5. [ ] **R15-budgets-left** — `screens/budgets.go:319-325`: drive LEFT tile off Breakdown.IsNegative → "−$X over" + red + hint tooltip (C144); keep Σ(limit−spent) as a separate "Budget headroom" label.
6. [ ] **R15-budgets-cat** — `screens/budgets.go ~278-295`: per-category prorated line via ComputeCategory(remaining, daysLeft/daysInPeriod from PeriodRange) → "~$X for rest of period" (C143). ORDER AFTER C132 rollover fix (Carryover feeds Remaining).
7. [ ] **R15-smart-floor** — `smartengine/budgets.go:92-97`: remove the $1 `safeToSpendFloorAb` early-return; fire with a low-balance note when liquid<$1 instead (C146).
8. [ ] **R15-redirect** — `aitools/aitools.go:105` + `screens/insights.go:339-351`: route both to safespend.Compute().SafeToSpend so every surface shares one formula (C142); audit fmtMinorUnits base-currency assumption.
9. [ ] **R15-i18n** — `i18n/en.go`: dashboard.safeToSpend, budgets.leftOverHint, budgets.categoryProrated, planning label rename.
> Risk: CommittedBudgets default 0 to avoid double-subtracting bills that are also budget categories — leave as caller opt-in (dedup is a follow-up). Cash basis = liquid only (tooltip it). Horizon depends on R14 payday anchor (falls back to calendar month).

### ★ IMPL plan — R14 pay-cycle-aware periods (C126-C131, from R14 design)
> ADDITIVE, pure-first, nil PayCycle = ZERO behavior change. Period enum (enums.go:126-160) has only weekly/monthly/quarterly; PeriodRange(p,ref,weekStart) (budgeting.go:132-148) silently defaults unknown→monthly; week-start locked Sun/Mon at 3 layers (prefs.go + settings_section.go:158-163); period <select> data-driven off AllPeriods (budgets.go:387-393 + budgetaddform.go:160-165). NO payday config (dataset.go Settings:34-44). R15's horizon = the returned period `end`.
1. [ ] **R14-enum** — `enums.go:126-160`: add PeriodBiweekly/PeriodSemiMonthly/PeriodYearly to consts+AllPeriods+Valid()+Label() (+test). Auto-populates the budget <select> — verify labels + order (Weekly/Biweekly/SemiMonthly/Monthly/Quarterly/Yearly).
2. [ ] **R14-periodmath** — NEW `budgeting/period_anchor.go` (pure time math): `PeriodAnchor{AnchorDate,SemiMonthDay1=1,SemiMonthDay2=15}`; `PeriodRangeAnchored(p,ref,weekStart,anchor)→[start,end)` — biweekly 14-day floor-div from anchor; semimonthly day1/day2 with EXPLICIT short-month clamp; yearly Jan1; else fall through to PeriodRange. Parse anchor at time.Local midnight (DST-safe). + tests (before/on/+13/+14 anchor, DST week, day2=31 Feb, leap, yearly Dec31/Jan1).
3. [ ] **R14-rollover** — `budgeting/rollover.go:29-31`: add PreviousPeriodRangeAnchored delegating to #2; keep PreviousPeriodRange as a zero-anchor shim (contiguity holds for semimonthly).
4. [ ] **R14-weekstart** — `prefs/prefs.go`: add Tue–Sat WeekStart consts; Normalize() accepts all 7; WeekStartWeekday() maps all 7 (+test).
5. [ ] **R14-paycycle-model** — `store/dataset.go:34-44`: add `PayCycle{Kind,AnchorDate(RFC3339-date),SemiMonthDay1,SemiMonthDay2}` + `Settings.PayCycle *PayCycle omitempty` (no migration); `PayCycleAnchor()→budgeting.PeriodAnchor`. Verify Settings JSON round-trip via store test.
6. [ ] **R14-settings-ui** — `app/settings_section.go`: new "Pay cycle" card — cadence select; anchor-date input (biweekly only); day1/day2 inputs (semimonthly only, 1/15); OnPayCycle→Settings.PayCycle; opt-in.
7. [ ] **R14-weekstart-ui** — `app/settings_section.go:158-163`: swap 2-option Segmented for a 7-option SelectInput (Sun–Sat).
8. [ ] **R14-thread** — `smartengine/budgets.go:133` + rollover call-sites: use PeriodRangeAnchored/PreviousPeriodRangeAnchored w/ Settings.PayCycle.PayCycleAnchor(); weekly/monthly/quarterly callers pass zero anchor (fall-through).
9. [ ] **R14-r15-horizon** — smartengine recentPayday heuristic (`bills.go:340`): when Settings.PayCycle!=nil use PeriodRangeAnchored `end` as the precise next-payday horizon; else keep heuristic (feeds R15-inputs).
> Defer: partial-first-period proration; trailing-pay-period spend analysis (SMART-B7/B10); MonthlyEquivalent for biweekly recurring. Risk: nil PayCycle + biweekly budget → degrade to monthly + UI warning.

### ★ IMPL plan — R12 budgeting methods (C112-C119, from R12 design)
> Methodology enum (budgeting/methodology.go:8-19) = simple/zero-based/envelope ONLY; set ONLY in Settings modal (app/settings.go:581-591), read ONLY at budgets.go:258 (C112 = no /budgets access). Envelope no-op (C113): EnvelopeAvailable IS computed + shown as a sub-line but NEVER folded into the bar's effective limit; Carryover() (rollover.go:22) has ZERO call-sites. No 50/30/20 (C114). NO configured income (C119; only inferred via ledger.PeriodTotals, hardcoded monthly at budgets.go:264). C116 period-select Go source is CLEAN (framework/WASM layer if it repros). C115 = dev-server SPA-fallback, not code.
1. [ ] **R12-budget-field** — `domain/entities.go:374`: add `Budget.Methodology string omitempty` (empty = inherit household; no migration).
2. [ ] **R12-income-field** — `store/dataset.go:41`: add `Settings.MonthlyIncomeMinorUnits int64` + `IncomeCurrency string` (omitempty; 0 = use tx-derived).
3. [ ] **R12-enum5030** — `budgeting/methodology.go:10-18`: add `Method5030 "50-30-20"` to consts + Valid() + ParseMethodology.
4. [ ] **R12-5030-engine** — NEW `budgeting/split503020.go` (pure): `Classify(cat,overrides)→Bucket` (default needs/wants/savings map); `Generate5030(income,cats,txns,now,rates)→Split5030Result{NeedsTarget=50%,Wants=30%,Savings=20% (int, remainder→savings),Proposals[]{Category,Bucket,Limit,Suggested},Unclassified}`. + split503020_test.go (exact split, remainder, no-history proportional).
5. [ ] **R12-helpers** — `budgeting/methodology.go`: `EffectiveMethodology(b,household)→Methodology` (per-budget override precedence); `IncomeForBudgets(configured,txns,start,end,rates)→int64` (configured if>0 else PeriodTotals). +tests. ALIGN/delegate with R15-foundation income source (avoid a 3rd income path).
6. [ ] **R12-envelope-fix (C113/C132 core)** — `budgets.go:275-282` + `budgets_row.go`: in MethodEnvelope, effectiveLimit = EnvelopeAvailable + spent; pass EnvelopeEffectiveLimit prop → bar denominator; "Left" tile = Σ envelope balances. EnvelopeAvailable is the authoritative multi-period walk; MEMOIZE (O(periods×txns)/budget/render, cap 240).
7. [ ] **R12-override-ui (C118)** — `budgetaddform.go` + `budgets_row.go`: "Budgeting method (override)" SelectInput (Inherit/Simple/Zero-based/Envelope/50-30-20)→b.Methodology in the budgets.go:104-136 save path; refactor the budgets.go:262 render loop to EffectiveMethodology(b,household) PER budget (currently household-for-all).
8. [ ] **R12-method-on-budgets (C112)** — `budgets.go` header: method dropdown/segmented (calls PutSettings like settings.go:581-591) so zero-based/etc. are reachable without the Settings modal.
9. [ ] **R12-5030-template-ui (C114)** — `budgets.go`: "Set up 50/30/20" header/empty-state CTA → modal showing Split5030Result (income source + 3 targets + per-category proposals, editable; per-cat bucket override in budget.Custom["bucket"]) → Apply = PutBudget per accepted proposal; prompt if no income.
10. [ ] **R12-income-tile (C119)** — `budgets.go:313-325` stat grid: "Income" tile (configured vs actual) via IncomeForBudgets when MonthlyIncome>0; + income input in Settings next to methodology.
11. [ ] **R12-rollover-css (C117)** — `budgetaddform.go:168`: add tw.Flex,tw.ItemsCenter,tw.Gap2 to the Label to match budgets_row.go:154 (one-liner).
> Risks: envelope effective-limit perf (memoize); income def (configured vs inferred — label which); 50/30/20 classification fragile (show+override before apply, store in Custom["bucket"], don't hard-persist); zero-based income period mismatch (IncomeForBudgets takes explicit period); C116 is framework-layer not Go.

### ★ IMPL plan — R13 live recompute + real-time overspend alerts (C120-C125, from R13 design)
> Mechanism = uistate UseDataRevision/BumpDataRevision (settings.go:49-70). C120 ROOT CAUSE confirmed: Quick-Add DOES bump (quickadd.go:154) but budgets.go:39-40 only uses a PRIVATE "rev:budgets" atom + never calls uistate.UseDataRevision().Get() — goals.go:43 does (the fix pattern). C122: over-budget alerts generated ONLY in runNotifyCatchUp (notifyrun.go:74-80), called once at boot (app.go:187); TriggerTxnAdded (appstate.go:1502) runs user workflows only. C123: quickadd.go:254 Height:420px < .flip-wrap 470px + .set-foot no flex-shrink. C124: budgets_row.go:310 fmtMoney→FormatAccounting (parens). C125: PostNotice (notice.go:44) + notify.budgetOverTitle (en.go:106) exist; no banner wired.
1. [ ] **R13-subscribe (C120, ONE-LINER)** — `budgets.go:39`: add `_ = uistate.UseDataRevision().Get()` (keep the private rev atom). Fixes live update after Quick-Add.
2. [ ] **R13-diff** — NEW `internal/app/budgetdiff.go` (PURE, native-test): `NewlyOverBudget(before,after []BudgetStatus)→[]string` (flipped Remaining>=0→<0). + test. Confirm BudgetStatus.Remaining is in the budget's own currency.
3. [ ] **R13-seam** — `appstate.go`: add `App.OnTxnMutated func()`, call nil-guarded after RunTriggered (~1503) in PutTransaction AND DeleteTransaction (edit/delete also cross/un-cross).
4. [ ] **R13-live-alert** — NEW `internal/app/livenotify.go` (js&&wasm): wrap OnTxnMutated — snapshot statuses before/after → NewlyOverBudget → per newly-over: PostNotice(notify.budgetOverTitle) (C125 toast) + notifyfeed.Entry; dedupe map[budgetName/yearMonth]bool PRE-POPULATED from runNotifyCatchUp (don't double-alert boot crossings); un-cross clears the key; wire as app.OnTxnMutated at app.go:~187.
5. [ ] **R13-banner (C125)** — `budgets.go:~243`: UseEffect(overCount dep, one-shot guard) → PostNotice when overCount>0 on navigate-in (pre-existing over-budget). Avoid render-loop re-fire (event-driven hook is the live path).
6. [ ] **R13-wording (C124)** — `format.go` add fmtMoneyPlain (money.Format, positive sign); `en.go` add budgets.rowPrimaryOver "%s · %s over budget"; `budgets_row.go:310` branch Remaining<0 → new key + fmtMoneyPlain(Abs()).
7. [ ] **R13-layout (C123)** — `quickadd.go:254` Height 420px→520px; `web/index.html` .set-foot add flex-shrink:0 (robust regardless of height; max-height:86vh caps small screens).
> Risks: dedupe scope (session map, pre-populate from boot catch-up, clear on un-cross); render-fire vs event-fire; edit+delete paths; multi-ccy Remaining basis; BumpDataRevision capture race (no-op before first render).

### ★ IMPL plan — R17 near-term "make it to payday" cash-flow forecast (C168-C175, from R17 design)
> KEY: the per-day projection ALREADY EXISTS — cashflow.Projection (cashflow.go:29-35: Daily/MinBalance/MinDay/BreachDay/BreachShortfall) via runway.Project (runway.go:69, expands domain.Recurring→dated Events). planning.go computes `proj` but reads only Min/Breach, NEVER proj.Daily (C172). Bugs are BASIS + SURFACING: runway start = ledger.NetWorth assets (planning.go:469) should be ledger.LiquidBalance (liquid.go:16) (C171); headline = 12-mo net-worth forecast.Project (planning.go:278) not near-term cash-flow (C168); low-balance = muted P (planning.go:528) not a tile (C173); runway gated len(recs)>0 no empty-state (planning.go:477, C174); afford uses net.Amount (planning.go:412/424) not liquid (C175 — SAME line R15 touches). No payday anchor on planning (C169; recentPayday lives only in smartengine/bills.go:340).
1. [ ] **R17-empty-guard** — cashflow.DailyBalances/runway.Events: confirm empty recs → flat series, no panic; add guard if needed (prereq for empty-state + fallback).
2. [ ] **R17-payday** — NEW `runway/payday.go` (pure): NextPaydayHorizon(from,payCycleDay,fallbackDays)→int (next day-of-month ≥from, clamp end-of-month, payCycleDay==0→fallback 30, MIN 1 day) + table tests. Consumes R14 PayCycle — coordinate field as `PayCycleDay int` (0=unset).
3. [ ] **R17-projectliquid** — `runway.go`: ProjectLiquid(liquidStart,recs,from,horizon,buffer,rates)→cashflow.Projection thin wrapper over Project w/ liquid-contract doc + test.
4. [ ] **R17-helpers** — `cashflow/helpers.go` (pure): DipDate(proj,from)→(time,bool) (BreachDay<0→false); PaydayBalance(proj,horizon)→Daily[min(horizon,len)-1].Balance (bounds-guard) + tests. Strict `<0` for dip.
5. [ ] **R17-basis (C171)** — `planning.go:469`: swap NetWorth assets → ledger.LiquidBalance.Amount as ProjectLiquid start; headline near-term start also liquid (C168). Snapshot rates ONCE, pass to both LiquidBalance + ProjectLiquid (FX consistency).
6. [ ] **R17-horizon** — `planning.go`: payCycleDay from Settings.PayCycle (0 if R14 unshipped); horizon = NextPaydayHorizon(now,payCycleDay,30) replacing hardcoded runwayDays=60.
7. [ ] **R17-chart (C168/C172)** — `planning.go` headline: render proj.Daily as the lead line chart (x=date,y=balance) + zero line + below-zero fill; demote the 12-mo net-worth forecast to a secondary "Long-term" section.
8. [ ] **R17-lowpoint-tile (C173)** — `planning.go:524-528`: promote MinBalance/MinDay from muted P → stat() tile; tone warn if <buffer, error if <0.
9. [ ] **R17-dip-warning (C170)** — `planning.go`: above chart, when BreachDay>=0, alert banner "cash may dip below $0 on <DipDate> (−<shortfall>)" (not a footnote).
10. [ ] **R17-payday-tile (C169)** — `planning.go`: when payCycleDay>0, "Balance on payday (<date>)" stat = PaydayBalance(proj,horizon); omit when 0.
11. [ ] **R17-emptystate (C174)** — `planning.go:477`: when len(recs)==0 render a nudge (→/recurring) instead of suppressing; fire regardless of liquidStart.
12. [ ] **R17-afford (C175)** — `planning.go:412/424`: afford.CanAfford start net.Amount→LiquidBalance.Amount. DEDUP with R15-planning (same line) — R15 owns this; defer/coordinate to avoid conflict.
13. [ ] **R17-integration-test** — planning data pipeline: liquid excludes investments; Min on liquid basis; DipDate false when never negative; empty recs→nudge; afford gets liquid.
> Risks: recurring source-acct (investment→transfer still reduces liquid — document); keep event model (don't spread MonthlyEquivalent); R14 field-name coordinate (PayCycleDay int); R15 afford overlap (R15 owns line 424); today boundary double-count (NextDue==today in ledger AND events?); FX rate snapshot; empty-recs vs genuine $0 (strict <0 + nudge regardless).

### ★ IMPL plan — R16 recurring & bills (C147-C156, from R16 design)
> Model: RecurringCadence (entities.go:209-217) = Weekly/Monthly/Quarterly/Yearly only (no biweekly/semimonthly, C152); Recurring{...Autopost bool} (entities.go:239-248) has NO paid/autopay state (C154; Autopost=auto-create-txn ≠ paid). SMART-P1 detect (smartengine/planning.go:171-206) IS computed but never surfaces (Smart off-by-default + CTA = bare ActionNavigate→/planning, C147). Bill calendar (bills_screen.go:213-216) hardcoded current month (C148); dots (243-248) title-only, no amount/urgency/click (C150; CalendarDay.Bills HAS amounts). Add form (planning.go:545-600) no NextDue (saves time.Now(), C149) + no biweekly/semimonthly (C152). RecurringRow (planning.go:856-867) delete-only (C153) + line 859 hardcodes Format("Jan 2, 2006") not pr.FormatDate (C155). Subscriptions Detect (subscriptions_screen.go:69) no account-type filter → liability payments misclassified (C151). NO /recurring route (screens.go:57-89, C156).
1. [ ] **R16-cadence** — `entities.go:209-217`: add CadenceBiweekly/CadenceSemiMonthly (COORDINATE w/ R14 — skip if R14 added them); Next() biweekly +14d, semimonthly 1↔15 via time.Date (DST-safe); MonthlyEquivalent() biweekly ×26/12, semimonthly ×2. +entities_test all 6.
2. [ ] **R16-paid-model** — `entities.go` + NEW `domain/occurrence.go` (pure): add Recurring.Autopay bool; RecurringOccurrence{RecurringID,DueDate,PaidAt *time.Time}; OccurrenceKey/MarkPaid/IsPaid/IsAutopayDue + test.
3. [ ] **R16-store** — state blob: add RecurringOccurrences []RecurringOccurrence + AddRecurringOccurrence/MarkOccurrencePaid; prune >12mo on load (unbounded growth).
4. [ ] **R16-classify (C151)** — `subscriptions/subscriptions.go` (pure): IsLiabilityPayment(sub,txns,accounts)→bool — source acct loan/CC OR payee/label lender heuristic (acct-type alone insufficient: CC min-payments debit from checking too). +test.
5. [ ] **R16-subs-filter (C151)** — `subscriptions_screen.go:69`: partition Detect → subs vs liabilityPayments; render liabilities in a separate "Loan & credit payments" section. dep #4.
6. [ ] **R16-p1-payload (C147)** — `smartengine/planning.go:171-206`: serialize per-candidate {label,amount,cadence,isLiability} into insight Data; retarget action /recurring?import=<key>; normalize label dedupe (lowercase+strip punct). +test. dep #1,#4.
7. [ ] **R16-route (C156)** — `screens.go:57-89`: register RecurringScreen at /recurring (SubGroupPlan) + sidebar nav; stub body first.
8. [ ] **R16-addform (C149/C152)** — `planning.go:545-600` + /recurring: date input for NextDue (parse→local midnight, blank→now); biweekly/semimonthly in cadence select; Autopay checkbox. dep #1,#2.
9. [ ] **R16-inline-edit (C153)** — RecurringRow (planning.go:856-867) + /recurring: edit button → pre-filled inline form → UpdateRecurring; cancel reverts. dep #8.
10. [ ] **R16-paid-toggles (C154)** — RecurringRow + /recurring: "Mark paid" (when !IsPaid && !Autopay) + Autopay toggle → MarkOccurrencePaid/SetAutopay. dep #2,#3,#9.
11. [ ] **R16-calendar-nav (C148)** — `bills_screen.go:213-216`: calendarOffset state + prev/next arrows → now.AddDate(0,offset,0) → MonthCalendar.
12. [ ] **R16-calendar-dots (C150)** — `bills/calendar.go` add pure DotUrgency(day,today)→overdue|due-soon|upcoming (+test); `bills_screen.go:243-248` apply urgency class + amount in title (pr.FormatMoney) + onclick→selectedDate scroll/highlight. dep #11.
13. [ ] **R16-p1-card (C147)** — NEW `screens/recurring_screen.go`: dismissible SMART-P1 card — candidate checkboxes + "Add selected" (pre-fills add form) + Dismiss (prefs key). dep #6,#7.
14. [ ] **R16-iso-date (C155)** — `planning.go:859` Format("Jan 2, 2006")→pr.FormatDate; audit screen files for bare display .Format. COORDINATE w/ R18 (owns systemic ISO fix — skip overlaps).
> Risks: per-occurrence vs per-recurring paid (occurrence ledger keyed RecurringID+DueDate, prune); autopay authority (user-authoritative; SMART-BL4 only seeds if unset, C160); cadence enum shared w/ R14 (don't add twice); subscriptions.Cadence vs domain.RecurringCadence mapping (document); classification false-positives (hybrid); Advance() is single-step (bounded loop cap 12); ISO coupling w/ R18.

### ★ IMPL plan — R19 automated savings (C183-C188, from R19 design)
> Workflow engine (workflow.go:52-68) = 8 actions, NONE move money — documented write-safe invariant (workflow.go:46-48): "no action creates transactions" (C186). Autopost/PostDueRecurring (appstate.go:1294-1327) is SINGLE-LEG (one txn, TransferAccountID never set) → "pay yourself first" inflates one acct w/o debiting source (C185). REUSE: App.CreateTransferPair (transfer_ops.go:51, two-leg + FX) — what UI doTransfer (accounts.go:222-255) calls. SMART-G17 (goals.go:136-140) has explicit `// TODO(C186)` → degrades to ActionNavigate (C187). Goal.AccountID = destination. Safe-to-spend math in b8SafeToSpend (budgets.go:97-126). ActionTransfer = SHARED infra for R19+R20.
1. [ ] **R19-txn-var** — `appstate.go:1101-1131` txnContext: add txn_is_transfer (1/0) + txn_amount_minor_local (acct-ccy minor units) — for round-up condition + delta.
2. [ ] **R19-action-model** — `workflow.go:51-121`: add ActionTransfer kind + Action{FromAccountID,ToAccountID,AmountMinorExpr,DedupeKey} + Effect{FromAccountID,ToAccountID,AmountMinor,DedupeKey}.
3. [ ] **R19-guard** — appstate PutWorkflow: reject ActionTransfer on TriggerTxnAdded at save (protect write-safe invariant; round-up uses accrual path #9).
4. [ ] **R19-rules-pkg** — NEW `internal/savings/savings.go` (pure): RoundUpDelta(amt,granularity) (ceil to boundary, 0 if on it); SurplusMinor(liquid,billsDue,goalContribs,cap)=min(max(0,…),cap); IsScheduleDue(lastRun,cadence,now); PeriodKey(t,period). + savings_test.go.
5. [ ] **R19-effect-wire** — `appstate.go:1175-1218` applyEffect ActionTransfer case: resolve AmountMinorExpr vs Context.Vars; dedupe (prior Run w/ DedupeKey → skip; dedicated TransferDedupeLog, not O(n) scan); OVERDRAFT clamp to source balance (skip if 0); CreateTransferPair under triggersSuspended (legs must NOT re-fire RunTriggered); tag Desc + Custom["automation"]=wfID.
6. [ ] **R19-plan-wire** — workflow planAction: produce ActionTransfer Effect (resolved amount + DedupeKey from wfID+PeriodKey) for dry-run/preview.
7. [ ] **R19-engine-vars** — `appstate.go:1088-1099` engineVars + txnContext: surface surplus_minor (savings.SurplusMinor; `// TODO(R15) replace w/ safespend.Compute`) + round_up_delta (savings.RoundUpDelta(-txn_amount_minor_local,100)).
8. [ ] **R19-pyf-template (C185)** — NEW `appstate/savings_ops.go`: CreateWorkflowFromGoal(goalID,amount)→Workflow{TriggerScheduled, ActionTransfer From=checking To=goal.AccountID, DedupeKey "pyf:wfID:periodKey"}→PutWorkflow. +test.
9. [ ] **R19-roundup (C183)** — `appstate/savings_ops.go`: on TriggerTxnAdded, for spends (amount<0 && !transfer) accumulate RoundUpDelta into a persisted RoundUpAccrual (rule+period); a separate TriggerScheduled workflow transfers the accrued total + resets (accrual-batch avoids recursion/guard). Disable round-up cross-currency.
10. [ ] **R19-sweep (C184)** — `appstate/savings_ops.go`: sweep = TriggerScheduled + ActionTransfer "surplus_minor", DedupeKey "sweep:wfID:periodKey"; use budgeting.PeriodRange to confirm PRIOR period ended + sweep its surplus (R14 timing); cap MaxSweepMinor.
11. [ ] **R19-g17-exec (C187)** — `goals.go:140`: add ActionCreateWorkflow kind (applyEffect deserializes→PutWorkflow); g17AutoContribute emits it w/ the pay-yourself-first template prefilled (MonthlyNeeded) for goal.AccountID; remove `// TODO(C186)`. +test.
12. [ ] **R19-automations-ui (C188)** — NEW `screens/automations.go`: list workflows w/ an ActionTransfer step — name, type (DedupeKey prefix pyf/sweep/roundup), enabled toggle (PutWorkflow), src→dest, amount/formula, "moved this period" (sum matched Run transfer effects); link to workflow editor; register route + nav.
> Risks: idempotency (dedupe before EVERY transfer, key=wfID+period, dedicated log); round-up only on spends (amount<0 && txn_is_transfer==0); recursion (triggersSuspended around legs — VERIFY scope); sweep timing (prior period via PeriodRange, R14); overdraft clamp; audit/undo (Desc+Custom; pair delete = both legs atomic); R15 overlap (delegate surplus when it lands); R20 reuse (keep ActionTransfer generic); FX (same-ccy or stale-rate insight; no cross-ccy round-up); Autopost coexistence (workflow transfer canonical; deprecate savings-Autopost — double-post risk).

### ★ IMPL plan — R20 sinking funds (C189-C194, from R20 design)
> SinkingFund* funcs EXIST + tested but UNWIRED — zero non-test callers (budgeting/rollover.go:40 SinkingFundContribution=ceil(target/periods), :59 SinkingFundAccrued capped, :79 SinkingFundProgress; rollover_funds_test.go) (C190, like R12 envelope). domain.Goal (entities.go:378) has NO IsFund/Kind (C189) + NO CategoryID (C192; Budget HAS one at :370). goals.MonthlyNeeded does the same ceil-division but feeds NO budget limit. SMART-BL9 (bills.go:31,578-611) fires PageBills only, emits ActionCreateTask (no fund-creation flow); Goals screen only RunPage(PageGoals) in the empty branch (goals.go:254) → never surfaces (C193). Goals partition = active/achieved only (goals.go:187-194), no fund group (C194). Fund = Goal flavor, NOT a new type. Auto-accrual REUSES R19 ActionTransfer.
1. [ ] **R20-model (C189/C192)** — `entities.go:378`: add Goal.IsSinkingFund bool + CategoryID string (omitempty; JSON blob = no migration).
2. [ ] **R20-math-wire (C190)** — `goals/goals.go`: FundSetAside helper wrapping budgeting.SinkingFundContribution (use the SAME months formula as MonthlyNeeded — off-by-one) + goals_test.go. (wires the existing funcs)
3. [ ] **R20-drawdown-logic (C192)** — `goals/goals.go` (pure): DrawDownFund(goal,spendAmount)→(Goal,err) — decrement CurrentAmount, floor at 0, currency-mismatch err + tests.
4. [ ] **R20-drawdown-wire (C192)** — appstate (near ContributeToGoal/PutTransaction): after a txn w/ CategoryID, scan IsSinkingFund goals where CategoryID matches; Amount negative && !transfer → DrawDownFund + PutGoal; convert via currency.Rates if fund ccy differs (skip+warn if no rate).
5. [ ] **R20-setaside-budget (C190)** — `smartengine/budgets.go:244` goalMonthlyNeedsBase ALREADY sums MonthlyNeeded incl funds (works when TargetDate set) — add comment + open-ended fallback (Custom period count). COORDINATE R15 (no double-subtract).
6. [ ] **R20-accrual (C191)** — `appstate/savings_ops.go`: CreateWorkflowFromSinkingFund(goalID,fromAcct)→TriggerScheduled + ActionTransfer To=goal.AccountID, DedupeKey "sf:goalID:periodKey" (mirror R19). GUARD skip when CurrentAmount>=TargetAmount. DEP: R19-infra (ActionTransfer) MUST land first.
7. [ ] **R20-bl9-action (C193)** — `bills.go:596-608`: BL9 ActionCreateTask→ActionCreateGoal (prefill name=bill, target=annual, IsSinkingFund=true); update smart.Action payload + bills_test golden.
8. [ ] **R20-bl9-surface (C193)** — `goals.go` non-empty branch: RunPage(PageBills), filter SMART-BL9, render "Suggested sinking funds" strip (smartInsightList, cap 3).
9. [ ] **R20-goals-group (C194)** — `goals.go:187-194`: 3-way partition funds/active/achieved; "Sinking Funds" collapsible section; goalRowProps += IsFund (sub-line "Set aside $X/mo · N months to go").
10. [ ] **R20-addform (C189)** — `goaladdform.go`: "This is a sinking fund" toggle → reveals category SelectInput; save IsSinkingFund + CategoryID.
11. [ ] **R20-editform (C189)** — `goals_row.go:138-165` edit branch: same toggle + category selector (categoryS hook at stable position); goals.go:103 saveGoal persists both.
> Risks: JSON-blob = no migration; draw-down vs budget = distinct quantities (no double-count, needs clear UI copy); set-aside vs R15 double-subtract (goalMonthlyNeedsBase already includes funds — audit when R15 lands); accrual DEP R19-infra; over-accrual guard (skip at target); BL9 gate (yearly/quarterly ≥$200 — strip empty otherwise, correct); multi-ccy draw-down convert-or-skip; MonthlyNeeded vs SinkingFundContribution off-by-one.

### ★ IMPL plan — R7 self-learning categorization (C32-C38, from R7 design)
> C32 BLOCKER ROOT CAUSE (unambiguous): the "Always categorize like this" path works UNTIL ruleaddform.go:53-55 — match/categoryID init to "" and UseRuleDraft() is NEVER called there. Atom IS written (transactions.go:197-204 SetRuleDraft) + captured (dialoghost.go:48-49) but ruleaddform never READS it → prefill silently dropped. C36: statement.DefaultCategorizer (statement/categorize.go) is DEAD — never called in production; CSV (documents.go:479-519) + Quick-Add (quickadd.go:192-206) use only rules.Category. C35: rules.go:167 rulesuggest.Suggest(...,3) literal. C33: editTxn (transactions.go:216-248) recategorize → PutTransaction (238) with NO learn hook. C37: transactions_row.go:234 funnel icon-only (i18n string is tooltip/aria only). C38: rules.go:202-241 suggestCard renders LAST (after rule list + Mermaid). C34: quickadd.go:192-206 category assist is Smart-gated, no keyword fallback.
1. [ ] **R7-c32-fix (BLOCKER)** — `ruleaddform.go:53-55`: call `rd := uistate.UseRuleDraft()`; if rd.Phrase!="" && match.Get()=="" → match.Set(rd.Phrase)+categoryID.Set(rd.CategoryID)+ClearRuleDraft() (one-shot flag; clear AFTER seeding). ~5 lines.
2. [ ] **R7-threshold-const (C35)** — `rulesuggest/rulesuggest.go`: add `const DefaultMinCount = 3`.
3. [ ] **R7-threshold-setting (C35)** — app settings/prefs: add RuleSuggestMinCount int (default 3); `rules.go:167` pass it not literal 3; numeric input in Settings.
4. [ ] **R7-tally (C33, pure)** — NEW `internal/learntally/tally.go`: Tally map[normPayee]map[catID]int; Increment/TopCategory/ShouldSuggest(payee,threshold)/NormalizePayee (lowercase+trim) + tally_test.go.
5. [ ] **R7-tally-persist (C33)** — appstate: LearnTally field, load on init, save on increment (existing storage).
6. [ ] **R7-learn-hook (C33)** — `transactions.go:238` editTxn: after PutTransaction, if CategoryID changed && Payee!="" → IncrementTally(Payee,newCat); if ShouldSuggest → NON-INTRUSIVE chip "Create a rule for [Payee]→[Cat]? (recategorized N×)" → SetRuleDraft + nav /rules. NEVER silent auto-rule. Note existing-rule conflict.
7. [ ] **R7-quickadd-categorizer (C36/C34)** — `quickadd.go:192-206`: after rules.Category=="" fallback to statement.Categorize(DefaultCategorizer,rawDesc), UN-gated (deterministic local lookup); feed catSuggestion (fixes live suggestion C34 too). Fallback, NOT override (rule match wins).
8. [ ] **R7-funnel-label (C37)** — `transactions_row.go:234`: add Span w/ i18n (short "Auto-rule") beside icon.Filter; btn-icon→btn-icon-label.
9. [ ] **R7-suggest-discover (C38)** — `rules.go:230-240`: move suggestCard BEFORE the Mermaid diagram (after "Your rules"); badge w/ suggestion count.
10. [ ] **R7-tests** — learntally/tally_test.go + rulesuggest_test.go: Increment/ShouldSuggest at/below/above threshold; NormalizePayee edges; Suggest configurable threshold; keyword fallback.
> Risks: NEVER silent auto-rule (always via SetRuleDraft confirm — surprises + F13/C105 order conflict); correction loops (suggest notes existing-rule conflict to reorder/delete, not just add); threshold persisted per-profile (privacy-correct); payee normalization (CSV vs free text; lowercase+trim start, strip #codes later); keyword fallback must NOT override a rule match; on-device only; explainability (show "recategorized N×"); ClearRuleDraft AFTER seeding (one-shot).

### ★ IMPL plan — R8 duplicate detection/review/merge (C86-C91, from R8 design)
> C86 BLOCKER ROOT CAUSE: ImportTransactionsCSV (appstate.go:204-213) assigns fresh id.New() UUID per row + PutTransaction upserts ON CONFLICT(id) (store/crud.go:47-49) — UUID-only key, so re-import NEVER conflicts → every row re-inserts. ZERO content dedup in this path. BUT ImportReviewedDocumentRows (appstate.go:769-777) ALREADY has a seen-map (date|normAmount via extract.FilterNew) wired ONLY to the vision/wizard path, not CSV paste. TWO inconsistent keys: extract.Signature (extract.go:~78 date|amount, NO desc) vs dedupe key (dedupe.go:44-46 date|signedAmount|ccy|normDesc). Dedupe notice (transactions.go:420) uses full txns not `shown` (C90). selectDuplicates (transactions.go:283-291) selects across unfiltered ledger, no count feedback (C91). NO merge — delete-only (C87). CsvImportCard (documents.go:171-193) parse→insert immediately (C88); DraftReviewList (documents.go:811-831) is vision-path only. NO /duplicates route (screens.go:57-90, C89).
1. [ ] **R8-fingerprint (pure)** — NEW `internal/fingerprint/fingerprint.go`: Fingerprint(date,amountMinor,payee,accountID)→sha256[:16] (date|signedMinor|normUpper(payee)|acct); NormalizePayee (lowercase/collapse-ws/strip POS #*); GroupDuplicates(txns)→[][]txn; MergeResolve(a,b) (prefer non-empty/longer memo/categorized/cleared/recent). + test. Zero WASM deps.
2. [ ] **R8-key-audit** — annotate extract.go:~78 + dedupe.go:44-46 dual-key divergence (don't delete; migrate callers in #3/#7).
3. [ ] **R8-import-dedup (C86 fix)** — `appstate.go:170-216` ImportTransactionsCSV: before the PutTransaction loop, fingerprint existing (target acct) + incoming → partition fresh/candidates; if candidates, return ImportResult{Fresh,Candidates} instead of inserting. Copy/adapt the seen-map pattern from ImportReviewedDocumentRows:769-777. dep #1.
4. [ ] **R8-import-warning (C88)** — `documents.go:171-193` importCSV: run #3 partition; if candidates, surface a review modal (reuse DraftReviewList or simpler inline) w/ per-row Skip/Import-anyway/Merge; only confirmed rows → PutTransaction + MergeTransactions. dep #1,#3.
5. [ ] **R8-merge-op (C87)** — `appstate.go`: MergeTransactions(keep,discardIDs) — write merged (MergeResolve) + delete discarded under WithoutTriggers (atomicity caveat: no store txn API → document). dep #1.
6. [ ] **R8-duplicates-screen (C89)** — `screens.go:57-90` register /duplicates + NEW `screens/duplicates.go`: GroupDuplicates → per-group card, side-by-side rows, Keep-first/newest/highest-detail presets + per-field select → MergeTransactions. dep #1,#5.
7. [ ] **R8-filtered-count (C90)** — `transactions.go:420`: FindDuplicates(shown) not txns; optional tooltip "N in view / M global".
8. [ ] **R8-select-feedback (C91)** — `transactions.go:283-291` selectDuplicates: after select, transient banner msg "Selected N duplicates" (ui.State, 2s reset); banner render 728-731 consumes it.
> Risks: false positives (2 legit identical coffees same day) → NEVER auto-delete, always user-confirm; payee normalization brittleness (conservative start, tune via Rules); transfer-pair merge (2 legs, atomic or relationship breaks); dual-key migration (#3/#7→canonical fingerprint); strict-hash misses renamed payee/amount-correction (surface via /duplicates not over-smart fingerprint); merge field-conflict (never silently discard memo/tags/category — surface in UI); filtered-count semantics (tooltip); merge atomicity (WithoutTriggers, crash-mid-merge caveat).

### ★ IMPL plan — R10 no-key receipt import (C93-C100, from R10 design)
> C93 BLOCKER: receipt import hard-gated behind OpenAI key; NO OCR libs vendored. HONEST OCR VERDICT: tesseract.js (only realistic client option) = 10-25MB bundle + 3-8s init + 2-5s/receipt on fanless X2 (thermal/battery) + 60-80% accuracy on crumpled receipts → STILL needs manual review → DEFER. Manual fallback = 90% value at 1% complexity. C95: documents.go:394 key-check BEFORE image-check (398). C99: documents.go:405 onResult discards `_ ai.Usage`; EstimateCostUSD/FormatCostUSD + pricing EXIST (ai.go:165,192-212) but unwired. C98: documents_image_import.go:77 hard nav to /settings drops component-local imageURL (documents.go:95). C97: pickImageDataURL (documents.go:1083-1113) no size/format guard. C96: only "noneFound" (documents.go:413), no blurry distinction. C94: capture="environment" on hidden input (documents.go:1091), no visible camera CTA. C100: en.go:725/742 bare "add key". REUSE: image preview (documents_image_import.go:57-72) + DraftReviewList (documents_draft_review.go:23-50 editable extract.Row) + importDraft/importReceipt (no pipeline change).
1. [ ] **R10-reorder (C95)** — `documents.go:394-398`: image-empty check FIRST, then key; no-key+image → offer manual path not error. ~2 lines.
2. [ ] **R10-validation (C97)** — `documents.go:1083-1113`: read file.size (>20MB err) + file.type (jpeg/png/gif/webp) before setting imageURL; pure JS reads, mockable.
3. [ ] **R10-nokey-fallback (C93 PRIMARY)** — `documents_image_import.go` + `documents.go`: when needsKey && imageURL!="" render split-view — image preview left + DraftReviewList prefilled w/ one blank extract.Row + "Add row" right; wire to existing draft state → importDraft/importReceipt (ZERO pipeline change). "No AI key — entering manually" callout. +i18n. Default receipt-mode.
4. [ ] **R10-badimage (C96)** — `documents.go:405-413` onResult: zero-rows + content has "unclear/blurry/cannot read" → clearer-photo err; HTTP 400/422 in ai.ErrorMessage (ai.go:240) → format-specific.
5. [ ] **R10-inline-key (C98 primary)** — `documents_image_import.go`: collapsible "Enter OpenAI key" input + Save (PutSettings like settings.go:555-564) so no nav needed; Settings link secondary.
6. [ ] **R10-image-persist (C98 safety-net)** — `documents_image_import.go:77`: before nav write imageURL to sessionStorage (syscall/js); Documents mount UseEffect restore+clear sessionStorage["pendingReceiptImage"].
7. [ ] **R10-key-explainer (C100)** — `en.go:725/742`: expand to what/how(platform.openai.com)/cost(<$0.01)/privacy(image→OpenAI, key not stored beyond session); render needsKey as styled collapsible info card. Privacy framing matters (receipts = financial data).
8. [ ] **R10-cost (C99)** — `documents.go:405`: keep `usage ai.Usage`; ai.EstimateCostUSD(model,usage)+FormatCostUSD → summary ("Extracted 5 (cost $0.003)"); static pre-call estimate by button.
9. [ ] **R10-camera (C94)** — `documents_image_import.go`: visible "Take photo" button (sets capture=environment) beside "Choose image" (clears capture); always show. HTTPS-only caveat.
10. [ ] **R10-ocr-spike (C93 OPTIONAL — DEFER, NO task)** — tesseract.js via syscall/js + naive regex→extract.Row, "Try local OCR (beta)" — ONLY if no-key adoption shows demand; 15-25MB + thermal cost. Do NOT build now.
> Risks: local-OCR bundle/perf/battery on fanless X2 (defer); manual-fallback friction (clear "manual means manual" copy); inline-key (#5) more robust than sessionStorage; cost-estimate accuracy (high-res base64 inflates tokens — actual Usage reliable); BYOK privacy (images→OpenAI, don't underplay); camera HTTPS-only; DraftReviewList receipt-mode default.

### ★ IMPL plan — R18 systemic ISO-date display fix (C155/C179/C241, from R18 audit)
> Canonical formatter: prefs.Prefs.FormatDate(t) (prefs.go:217-219; styles DateISO/US/EU/Long via dateLayout:204-215), read per-component via uistate.UsePrefs().Get(). ISO-only internal: dateutil.FormatDate (dateutil.go:28) — keep for <input>/keys/CSV/signatures, NEVER display. C241 (reports "Covering") ALREADY FIXED (reports_screen.go:666). C179 goals_row.go:184 ALREADY FIXED — live C179 bug is dashboard.go:1040. Need NEW prefs.FormatMonthYear (no style maps month+year) for the "Jan 2006" sites.
> Audit — 16 display-bug sites: planning.go:859(C155/R16),483,486,688 (→FormatDate) + 260,308,747,748,752 (→FormatMonthYear); dashboard.go:1040(C179),1155; accounts_row.go:568; artifacts.go:302; insights.go:1245; documents.go:373,990; widget_builder.go:676 (low-pri). Legit-ISO (don't touch): <input> defaults, map keys, extract.Row.Date, CSV/export, notify/dedupe/store. AI-prompt dates out of scope.
1. [ ] **R18-helper** — `prefs.go`: add FormatMonthYear(t) (US/EU/Long→"Jan 2006", ISO→"2006-01") + prefs_test.go. Ships alone.
2. [ ] **R18-planning (C155)** — `planning.go`: add pr:=uistate.UsePrefs().Get() to the render fn (currently ABSENT) + thread Prefs into recurringRowProps; 859 FormatDate (R16 DEFERS here), 483/486/688 FormatDate, 260/308/747/748/752 FormatMonthYear. Hooks at stable positions.
3. [ ] **R18-dashboard (C179)** — `dashboard.go:1040` g.TargetDate→pr.FormatDate, 1155 t.Date→pr.FormatDate (ensure pr in scope).
4. [ ] **R18-accounts** — `accounts_row.go:568` raw-ISO→pr.FormatDate (thread pr into reconcile-row props; leave 124/130 <input> ISO).
5. [ ] **R18-docs+artifacts+insights** — `documents.go:373/990`, `artifacts.go:302`, `insights.go:1245` → pr.FormatDate (leave documents.go:254/304/686/783 data fields).
6. [ ] **R18-widget (low-pri)** — `widget_builder.go:676` → pr.FormatDate.
7. [ ] **R18-close** — mark C241 + goals_row C179 already-fixed; R16-ui DEFERS planning.go:859 to R18-planning (no double-fix).
> Risks: planning.go has NO UsePrefs today (add + thread carefully); "Jan 2" sites gain a year via FormatDate (accept, no FormatDateShort); FormatMonthYear ISO "2026-01" widens chart x-axis; recurringRowProps signature change = update callers; R16 collision on :859 (R18 owns); don't touch CSV/keys/inputs.

### ★★ FEATURE — Multi-Institution Analytics: cross-institution/cross-account scoped reporting [USER REQUEST 2026-06-25]
> Goal: a flexible PERSISTENT scope selector filtering analytics/reports (+ dashboard/insights/net-worth) by any combo of institution / owner (personal/shared/member) / account-type / hand-picked accounts — NO separate profiles. CURRENT: Account (entities.go:74-101) has NO Institution field (Lender is liability-only free-text, unsuitable); owner/scope = OwnerID + Scope(individual/shared) + GroupOwnerID="group". Reports (reports_screen.go:194 → reports.SpendingByCategory) + dashboard (dashboard.go:44-150) + NetWorth (ledger.go:183, takes []Account) + insights (CategorySpendSeries) all take PLAIN SLICES → scope injects at the call-site, NO report-fn signature changes. F41 member scoping EXISTS (uistate/activemember.go UseActiveMember localStorage atom; dashboard KPIs scoped 79-93; NOT wired to reports/networth/insights — C277-C281); MIA's owner dim GENERALIZES it to multi-select → COMPOSE, don't duplicate. txnfilter/multi.go MultiCriteria{Accounts,Categories,Members,Tags} exists (reuse pattern). NO SavedView concept anywhere. Persist via Dataset.SettingsKV (dataset.go:94) + localStorage.
1. [ ] **MIA-institution-field** — `domain/entities.go:74-101`: add Account.Institution string (omitempty; JSON-blob = no migration, old rows ""). NEW domain/account_helpers.go UniqueInstitutions(accounts)→sorted case-insensitive dedup + test.
2. [ ] **MIA-scope-engine (pure)** — NEW `internal/scope/scope.go`: ReportScope{Institutions,Owners,Types,AccountIDs} (empty dim=all; AND across dims, OR within; AccountIDs union); IsAll(); ResolveScope(accounts,scope)→[]ids (skip archived, case-insensitive inst); ApplyScopeToTxns/ApplyScopeToAccounts + scope_test.go (empty→all, per-dim, multi-dim AND, AccountIDs union, archived excluded).
3. [ ] **MIA-savedviews (pure)** — NEW `internal/scope/savedview.go`: SavedView{ID,Name,Scope}; List/Put/Delete over map[string]string KV (key "cashflux:saved-scopes" in SettingsKV) + test; app facade ListSavedViews/PutSavedView/DeleteSavedView → UpdateSettings.
4. [ ] **MIA-activescope-atom** — NEW `internal/uistate/activescope.go`: UseActiveScope/SetActiveScope/persist (localStorage "cashflux:active-scope", JSON ReportScope). MIGRATION: if old "cashflux:active-member" set && activeScope empty → scope.Owners=[memberID], clear old key. activeMemberID becomes a derived read (SINGLE source of truth).
5. [ ] **MIA-reports-wire** — `reports_screen.go:194`: after Accounts()/Transactions(), resolve+apply UseActiveScope() before reports.* calls; SpendingByMember (491) then operates on already-scoped txns. (no reports pkg change)
6. [ ] **MIA-scopebanner (builds C281)** — NEW `internal/app/scopebanner.go`: reads UseActiveScope(); nothing when IsAll(); else "Viewing: <inst> · <owner> · <types> [Clear]" (owner→member-name / "Shared"). Covers F41 "Viewing as" + MIA. Embed in reports header.
7. [ ] **MIA-scopeselector** — NEW `internal/app/scopeselector.go`: chip multi-select institutions (UniqueInstitutions) / owners (Members()+Shared) / types (AccountType enum) + collapsible account picker + saved-views dropdown (Save as…/Delete) → SetActiveScope; sync single-owner with MemberSwitcher. Embed in /reports filter panel.
8. [ ] **MIA-dashboard-networth** — `dashboard.go:44-150`: replace ad-hoc activeMemberID filter (79-93) with ApplyScope* via UseActiveScope(); NetWorth gets scoped accounts (+ "vs household total: $X" sub-label); MemberSwitcher sets scope.Owners (keep in sync, don't clear inst/type).
9. [ ] **MIA-insights** — `insights.go`: pre-filter txns via ApplyScopeToTxns before CategorySpendSeries; embed ScopeBanner + compact selector.
10. [ ] **MIA-institution-mgmt** — account add/edit form: Institution text input w/ autocomplete from UniqueInstitutions; normalize on submit (trim+title); pre-fill from Lender if set & Institution empty; "Set institution" backfill prompt in accounts list for "".
11. [ ] **MIA-institution-entity (OPTIONAL, defer)** — first-class Institution{id,name,color,icon} in SettingsKV if free-string dedup/color proves insufficient; Account.Institution becomes a name-FK. Defer until feedback.
> Risks: F41/MIA owner double-filter — make activeScope the SINGLE source, activeMemberID derived, migrate (TOP risk); transfer half-in/out of scope = appears as expense (banner tooltip); net-worth-of-subset semantics (banner + "vs household total" sublabel; empty=household total); institution free-string dedup (normalize+case-insensitive; entity later); empty scope=ALL not none; saved-view stale AccountIDs (ResolveScope ignores missing; cleanup on delete deferred); multi-ccy across institutions (existing FX layer; banner warns); perf O(N) re-filter (negligible; memoize in atom if needed); Lender vs Institution independent. DEP: composes with F41 (C277-C281) + account types (C73).

### ★ IMPL plan — R4 multi-currency UX (C77-C85, from R4 design)
> FX CONVENTION (currency.go:122-125): Rates[code] = major-units-of-BASE per 1 major-unit-of-that-currency → Rates["EUR"]=1.08 means 1 EUR=1.08 USD (foreign→base); base not stored (=1.0); Convert (currency.go:155-176) routes thru base. C77 JPY inversion ALREADY FIXED (sample.go:849 JPY:0.0066; comment 846-848 documents the old 151× bug) but NO regression test. C78: accountaddform.go:54-64/214 hides picker when singleCurrency = chicken-and-egg. C79: NetWorthExplained (networth_explained.go:42-48) excludes unrated accts; dashboard.go:192-195 DOES notice "excludes N" — but NO add-time rate path. C80: FXUpdatedAt exists (stamped settings.go:616) but fxRateRow (settings.go:1066-1072) renders only the Stale bool, never the date. C81: fxRateRow shows "1 EUR =" (en.go:1046) — partial. C82: no conversion-success disclosure (dashboard.go:191-195). C84: FX table only in settings modal, no route/link from accounts. C85: USD/CAD/AUD/MXN all Symbol:"$" (currency.go:35-44). C83: NO real .skip-link/.add-item collision in source → likely PHANTOM. fxai.go AUTO-FETCH already built (key-gated button).
1. [ ] **R4-convention-test (C77 guard)** — currency_test.go: Convert(100 JPY,USD)≈$0.66 + a 151.0 rate yields wildly-wrong; store/sample_test.go: FXRates["JPY"]<1.0. Pure; prevents regression (sample already fixed).
2. [ ] **R4-symbol (C85)** — currency.go:39-44: CAD→"CA$", AUD→"A$", MXN→"MX$" (keep USD "$"); update format_test.go. Pure.
3. [ ] **R4-addtime-rate (C78/C79)** — accountaddform.go: remove singleCurrency gate (54-64,214), always show picker (default base); fxRate UseState UNCONDITIONAL (render conditional); when curr≠base && FXRates[curr]==0 show "1 [CODE] = ___ [BASE]"; on submit write rate+FXUpdatedAt BEFORE PutAccount. EXTRACT shared SetFXRate(code,rate) used by add-form + settings table (drift). Optional "Fetch rate" if key.
4. [ ] **R4-fx-date (C80)** — settings.go: add UpdatedAt to fxRateRowProps (from s.FXUpdatedAt[code] ~647); fxRateRow render "Updated <shortdate>"/"Never" after stale badge; formatShortDate helper.
5. [ ] **R4-convention-explain (C81)** — settings.go FX subhead "Each rate = how many <BASE> equal one unit of that currency"; same orientation label in add-form (#3); info-icon tooltip.
6. [ ] **R4-discoverability (C84)** — accounts.go: when MissingCurrencies>0 add "Set exchange rates" link → settings FX section; dashboard.go:193-195 make "excludes N" notice clickable.
7. [ ] **R4-networth-disclosure (C82)** — dashboard.go: when non-base accts && MissingCurrencies==0 && len(rates)>0, nwSub "includes converted balances" (+tooltip). COMPOSE w/ MIA ScopeBanner (separate FX line, don't collide).
8. [ ] **R4-c83-investigate** — reproduce skip-link/add-menu in browser; source shows NO collision (shell.go:175 anchor=live-path#main; addmenu <button role=menuitem>) → likely CLOSE invalid or re-describe.
> Risks: convention migration (users who typed old 151 JPY still broken — prefer stale-indicator + manual over a risky hydrate heuristic); inline-rate vs FX-table source of truth (shared SetFXRate); symbol change breaks format_test/e2e (update); JPY 0-decimals lossy tiny amounts (correct, doc); GWC hook stability (fxRate UseState unconditional); auto-fetch key+net+verify wording; C82/MIA tile footnote coordination.

### ★ IMPL plan — R5 onboarding / setup wizard (C21-C31, from R5 design) [re-applied after race]
> Detection: hydrate.go:10-33 decideHydrate → hydrateSeed (fresh, auto-loads sample)/hydrateImport/hydrateEmpty (wiped); seededBefore in localStorage; NO wizard flag. C26 accounts.go:292-298 empty leads with btn-primary "Load sample data"; add-account only via icon-only top-bar + (addmenu.go:104-125). C27/C30 accountaddform.go:70 owner=GroupOwnerID always even w/ 0 members; opening-balance bare input. C29 budgets.go:285-314 ALREADY renders real empty Budgets state — "renders dashboard" likely a dev-server 404 fallback NOT a code bug. C23 currency (settings_section.go:50)+week-start (157-163) buried. C22 NO Settings.MonthlyIncome (dataset.go:34-44, DEP R12). C28 members at /members, no first-run path. Host = AddHost/DialogHost/SettingsHost (shell.go:200-203)+FlipPanel(2-side); need new WizardHost. (Full per-step detail in tasks #448/#449.)
1. [ ] **R5-progress (pure)** — NEW `internal/setup/progress.go`: Compute(Settings,[]Account,[]Member)→Progress + AllRequired + NextIncompleteStep + IsFirstRun gated on WizardShownOnce NOT account count + test.
2. [ ] **R5-settings-flags** — `dataset.go:34-44`: WizardDismissed + WizardShownOnce + SetupCurrencyConfirmed bool.
3. [ ] **R5-owner-default (C30)** — `accountaddform.go:70`: solo/personal default when 0 members; audit ownerSelectOptions sentinels.
4. [ ] **R5-empty-cta (C26)** — `accounts.go:292-298`: btn-primary "Add your first account"→SetAddTarget; demote Load-sample to btn-outline.
5. [ ] **R5-c29-investigate** — confirm /budget empty = dev-server 404 fallback (serve index.html for all routes) not budgets.go.
6. [ ] **R5-balance-hint (C27)** — `accountaddform.go`+en.go: opening-balance sign-convention field-hint.
7. [ ] **R5-wizardhost** — NEW `app/wizardhost.go`: UseWizardOpen/UseWizardStep atoms + native <dialog> (trap, ESC→Skip) + Skip/Back/Next/Done; mount shell.go after SettingsHost.
8. [ ] **R5-step-currency (C23)** — NEW `app/wizard_step_currency.go`: EXTRACT shared `app/settings_controls.go` currency+week controls (avoids R4/R14 conflict); Next→SetupCurrencyConfirmed.
9. [ ] **R5-step-income (C22, DEP R12)** — NEW `app/wizard_step_income.go`: reuse R12 MonthlyIncome; SKIP step if R12 unshipped (IncomeDone=true).
10. [ ] **R5-step-account** — NEW `app/wizard_step_account.go`: embed AccountAddForm + "+ Add another"/Done (needs #3).
11. [ ] **R5-step-members (C28, optional)** — NEW `app/wizard_step_members.go`: embed MemberAddForm + "Skip — I'm the only one".
12. [ ] **R5-trigger** — shell.go/main.go post-hydrate UseEffect-once: IsFirstRun && !WizardShownOnce && !WizardDismissed → open + set WizardShownOnce.
13. [ ] **R5-checklist (C31)** — NEW `app/gettingstarted.go` on / : steps w/ check/incomplete + Continue→NextIncompleteStep; auto-hide when AllRequired (home only, never nag).
14. [ ] **R5-r12-wire** — progress.go IncomeDone=s.MonthlyIncome>0 once R12 lands.
> Risks: first-run vs intentional-empty (WizardShownOnce flag); sample vs wizard mutually exclusive; R12/R4/R14 unmerged (skip income; shared controls); owner sentinel; <dialog> a11y; never-nag; C29 = server config.

### ★ IMPL plan — R28 configurable alerts (C263-C269, from R28 design)
> ALREADY SHIPPED (verified in source — DO NOT redo): C265 paycheck-landed (notifyfeed.go:259), C266 low-balance (notifyfeed.go:198, default $100 notify/defaults.go:16), C267 severity (FeedItem.Severity notifyfeed_filter.go:15-23 + notifySeverityPill notifications.go:28 + severityString notifyrun.go:280-289), C268 per-item read/dismiss/snooze (notifyRow notifications.go:56 + uistate helpers + SnoozedUntil + VisibleFeed), C269 Settings Notifications jump-tab (settingssectionnav.go:31 + e2e). Config MODEL done: notify/ruleconfig.go RuleConfig{Enabled,Thresholds} + IsEnabled/EffectiveThreshold/marshal (absent key=on, new rules auto-on). GENUINE GAPS: C263/C264 = NO Settings UI exposing RuleConfig; R13 live OnTxnMutated seam not built (only boot runNotifyCatchUp notifyrun.go:38, dedupe via DeliveredLog "cashflux:notify:delivered").
1. [ ] **R28-i18n** — en.go: 8 per-rule labels (settings.notify.rule.<id>.label) + threshold-unit keys (days/amount) + descriptions.
2. [ ] **R28-settings-panel (C263/C264)** — app notifySettings (the C269 section, data-testid settings-notifications): ForEach notify.DefaultRules() → enable toggle (RuleConfig.Enabled[id]) + numeric threshold input for threshold rules (bill-due/large/low-balance/paycheck) in display units (days/$ ↔ minor at boundary); persist uistate.SettingKVSet(notify.RuleConfigKey(),…) on change; extract pure minor↔display helpers.
3. [ ] **R28-e2e** — e2e/c263_notify_rule_toggles.mjs + c264_notify_thresholds.mjs (pattern from c269): toggle persists; threshold survives reload.
4. [ ] **R28-r13-seam (SHARED with R13-reactivity #3)** — appstate.go: App.OnTxnMutated func() called nil-guarded after RunTriggered in PutTransaction AND DeleteTransaction. Build ONCE, shared by R13 + R28.
5. [ ] **R28-livenotify** — NEW `internal/app/livenotify.go` (js&&wasm): assign OnTxnMutated at app.go:~187; re-run ONLY txn-sensitive generators (PaycheckLanded/LowBalance/Budget/LargeTransaction) w/ current accounts/txns + effective thresholds; dedupe via a module map PRE-POPULATED from DeliveredLog at boot (no double-fire vs catch-up); new keys → PrependNotifyFeed + mark delivered + optional browser notif. Reuse notify.DedupeKey/EnabledRules, notifyfeed.*Candidates, severityString.
6. [ ] **R28-config-gate** — livenotify reads current RuleConfig from settingsKV EACH fire (cheap unmarshal, no cache) so a toggle (C263) suppresses live firing without reload; notify.EnabledRules filters.
7. [ ] **R28-livenotify-test** — internal/app/livenotify_test.go (no build tag): extract + test the pure dedupe-map + EnabledRules gating.
8. [ ] **R28-snooze-prune** — notifyrun.go runNotifyCatchUp: at boot loadNotifyFeed→VisibleFeed(now)→PersistNotifyFeed(pruned) before PrependNotifyFeed so expired snoozes don't hit the 50-cap.
> Risks: dedupe across catch-up+live (live map PRE-SEEDED from DeliveredLog, sequenced after catch-up — TOP risk); snooze storage growth (prune #8); paycheck false positives (IsIncome=!transfer&&amount>0 catches refunds — threshold+3d window; KindIncome filter later); low-balance per-account not aggregate (panel copy clarifies); config-without-reload (re-read settingsKV each fire); spam (FrequencyCap=0 in defaults; occurrence-keys ISO-week for low/stale, txnID for paycheck=idempotent). #4 SHARED with R13 — don't double-build.

### ★ IMPL plan — R27 financial-health score (C260-C262, from R27 design)
> SUBSTANTIVELY ALREADY SHIPPED (verified — C260+C262 effectively CLOSED): pure engine internal/healthscore/healthscore.go Evaluate(Inputs)→Result (deterministic, no-AI, table-tested); 5 weighted factors (savings .25/emergency .25/debt-obligation .20/budget-adherence .15/credit-util .15), composite 0-100 + bands (Excellent≥80…Critical<25, healthscore.go:194-206), top-3 improvement steps (210-229), re-normalizes inapplicable factors, min-2 else BandNoData. Wired in screens/health.go buildHealthInputs (37-151): savings (reports.IncomeVsExpense→ledger.SavingsRate), emergency (LiquidBalance÷spend), debt (liability MinPayment÷income), adherence (budgeting.EvaluateRollup), util (ledger.Utilization); 3mo lookback. /health route REGISTERED (screens.go:71); dashboard "health" widget WIRED (dashboard.go:243); 12mo trend persisted (uistate/healthtrend.go). C261 SMART-A10 = dead AI-gated per-account STUB (catalog.go:70, zero engine), separate + harmless. GENUINE GAPS: net-worth-trend 6th factor (ledger.NetWorthSeries exists, NOT wired); A10 annotation; R12 income fallback.
1. [ ] **R27-nwtrend-factor (pure)** — healthscore.go: Inputs.NWTrendPct + HasNWTrend; curve (shrink>10%→0, flat±2%→40, +5%→80, +10%→100); weight 0.10, rebalance (savings/emergency .25, debt .20, budget/credit→.10); improvement step; include in Evaluate when HasNWTrend + healthscore_test.go (declining/flat/growing/inapplicable; weights sum 1.0 across combos).
2. [ ] **R27-nwtrend-wire** — screens/health.go buildHealthInputs: ledger.NetWorthSeries(accounts,txns,rates) 3mo; ≥2 datapoints → NWTrendPct=(end-start)*100/abs(start), HasNWTrend=true; guard start==0→false.
3. [ ] **R27-nwtrend-ui** — health.go HealthScreen: NW-trend factor row (same pattern as 5 existing; delta+step generic).
4. [ ] **R27-a10-annotate (C261)** — catalog.go:70: comment SMART-A10 = per-account drill-down pending; point to internal/healthscore; note R22 credit-health is separate.
5. [ ] **R27-r12-income-fallback (DEP R12, defer)** — health.go ~line 59: if !HasIncome && Settings.MonthlyIncome>0 → use configured + HasIncome=true. Engine untouched. Build when R12 lands.
> Risks: NW-trend volatility (3mo + min-2-snapshots inapplicable); weight rebalance shifts existing scores (document, named const block); income w/o R12 (BandNoData graceful; R12 priority); emergency baseline lumpy (expense/3, 6mo avg later); DTI needs MinPayment populated (infer 2%-of-balance later); sparse new-user already BandNoData; multi-ccy (NetWorthSeries uses rates); R22 distinct from util factor; A10 zombie in catalog UI (annotate now, gate later).

### ★ IMPL plan — R26 recommendation hub (C254-C259, from R26 design)
> MOSTLY ALREADY SHIPPED (verified source + git log): C256 executable actions DONE — smart_card.go:66-204 dispatches all 6 kinds incl ActionAutomateGoal→CreateWorkflowFromGoal→workflow.ActionTransfer→applyEffect (appstate.go:1220-1244); ActionTransfer (workflow.go:77) + C186/C187 SHIPPED (commits 21d43755/3228820f — so R19 foundation partly landed). C258 SU1 (ActionCancelSubscription subscriptions.go:390-431) + SU9 toast (smart_card.go:134) DONE+tested. C259 DONE (smart/cap.go CapPerRule+EnableFreeOnly + "Enable Free Only" btn smart.go:312-317). C254 free-on-by-default WORKS (IsEnabled tier-default smart/settings.go:57-70: absent+!ExplicitOff && TierFree→on). C255 PERSISTS (PRESERVED SQLite KV smartsettings.go:17/37-43→kvbridge→dataset.go:92). C257 /smart HAS ranked Insights tab (smart.go:105-191 paginated 10 + CapPerRule 3) + Manage tab + dashboard "smart-digest" widget (dashboard.go:252/1346 cap 3). GENUINE GAPS ONLY: (a) C254 residual stale stored Enabled[free]=false not re-seeded; (b) C255 residual pre-init KV race (kvbridge.go:74 reads browserstore before appstate.Default ready); (c) C257 residual digest widget below fold (GridRow "10") + density-gated.
1. [ ] **R26-migrate (C254, pure)** — NEW smart/migrate.go: MigrateSmartSettings(s,catalog)→(Settings,changed) — per TierFree feature where !ExplicitOff[code], delete stale Enabled[code]=false; idempotent + migrate_test.go (zero-value no-op; stale-false cleared; ExplicitOff kept; TierAI untouched).
2. [ ] **R26-migrate-wire (C254)** — smartsettings.go:22-32 LoadSmartSettings: after deserialize call MigrateSmartSettings(s, smart.AllFeatures()); if changed → SaveSmartSettings.
3. [ ] **R26-kv-race (C255)** — kvbridge.go:74: if appstate.Default==nil return "" (don't read stale browserstore); caller returns correct zero-value (free-on); + test.
4. [ ] **R26-widget-position (C257)** — dashboard.go ~252/1352: "smart-digest" GridRow "10"→"4"/"5" (above fold); relax/remove AffordanceWidget density gate (capped 3, additive); verify collapses when len(insights)==0.
5. [ ] **R26-close** — C256/C258/C259 DONE (cite file:line), close no-code; note R19 ActionTransfer already shipped.
6. [ ] **R26-ranking (optional)** — verify smartengine.Run sort (run.go); if registration-order not severity×savings, add a ranking signal so "ranked" hub is genuine. Non-blocking.
7. [ ] **R26-action-transfer-smart (contingent)** — only if a future rule needs a direct (non-workflow) transfer: ActionTransfer smart kind + smart_card.go confirm-dialog handler → CreateTransferPair. Don't add preemptively.
> Risks: default-on noise (free=local-only, AI opt-in; migration only clears non-ExplicitOff stale-false); idempotency (changed flag, O(catalog)); pre-init race degrades to zero-value (safe); widget position cosmetic (collapse-when-empty); R25 anomaly dedupe (digest + anomaly hub may dup — coordinate ownership); ActionAutomateGoal runs on schedule not immediately (navigates /planning to review — safe).

### ★ IMPL plan — R25 unified anomaly hub (C252-C253, from R25 design)
> Detectors ALL EXIST + deterministic/free: insights.Detect (insights.go:92, category spend deviation ≥50%, ≥$10 floor, MinPeriods guard) + 4 SMART anomaly engines A1 a1BalanceAnomaly (accounts.go:156-197 ≥3× mean), T2 t2Duplicates (transactions.go:119), T6 t6SpendingSpike (transactions.go:140), T7 t7MissingTxn (transactions.go:191). ROOT GAP (C252): /insights (insights.go:1296/1323 spendingHighlights→detectSpendingAnomalies) uses ONLY insights.Detect, ZERO smartengine import → A1/T2/T6/T7 NEVER reach /insights or the dashboard highlight (dashboard.go:581 topHighlightWidget, insights.Detect-only). They appear only in /smart Insights tab (smart.go:198) + occasionally the density-gated smartDigestWidget. C253 fragmentation: 3 partial surfaces via 2 parallel pipelines (insights.Detect vs smartengine.Run) never bridged. Types: insights.Anomaly (insights.go:44-51) vs smart.Insight (different struct).
1. [ ] **R25-engine-filter-audit** — smartengine/engine.go: does Run accept a feature-code allowlist? If not, call a1/t2/t6/t7 DIRECTLY (avoid running ALL engines from /insights). Read first; gates #2.
2. [ ] **R25-detect-all (C252)** — NEW screens/insights_anomalies.go: detectAllAnomalies = insights.Detect + the 4 SMART anomaly engines (direct/filtered) merged to a common display struct; cap ~5 sorted by magnitude; category dedupe (insights.Detect vs T6); insufficient-data guard (mid-month "down 100%", C232). Deterministic, no Smart gate + test.
3. [ ] **R25-insights-wire (C252)** — insights.go:1296: spendingHighlights→detectAllAnomalies; render merged via existing insight-row/highlightText (need smart.Insight→common converter or parallel renderer).
4. [ ] **R25-dashboard-wire (C252)** — dashboard.go:581-582 topHighlightWidget: detectSpendingAnomalies→detectAllAnomalies, take [0]. One-line.
5. [ ] **R25-rename (C253)** — insights.go header "Spending Highlights"→"Anomalies".
6. [ ] **R25-import-guard** — add smartengine import to insights.go as a distinct commit to catch circular-dep early.
7. [ ] **R25-shared-helper** — promote detectAllAnomalies to screens/anomaly_helpers.go (insights.go + dashboard.go share; R26 widget reuse).
8. [ ] **R25-digest-dedupe-note** — review-gate: A1/T6 on /insights AND digest (different screens) = intentional; /insights=anomaly home, /smart=full engine view; document. No code.
> Risks: mid-month false positives (insights.Detect has MinBaseline/MinPeriods; SMART T6/A1 hardcoded 3×/$100 — shared insufficient-data guard); category overlap insights.Detect vs T6 (dedupe decision); unfiltered Run executes ALL engines (direct calls/filter, #1); circular dep (#6); highlight helpers typed to insights.Anomaly (converter, lossy); noise (cap+sort); ownership (/insights=home, /smart=full).

### ★ IMPL plan — R24 no-key AI fallback + chat UX (C244-C251, from R24 design) [re-applied after race]
> aitools SEED: query_transactions+account_balances (aitools.go:46-91) + affordability (93-122, uses LiquidBalance) + DataSource iface (28-36). afford fast-path (insights.go:339-351) = ONLY keyless answer; returns early WITHOUT clearing errMsg (C245; clear only run():242). noAI→trailing=Fragment() (780-798) = NO Send btn (C246). Key gate (353-356) blocks all other keyless (C244). Per-bubble cost SHIPPED (1074-1081, ai.go:165-198) but no model pill (C250). keyHintNode (133-138) thin (C247); same documents_image_import.go:75 (R10) — NO shared explainer. Sysprompt pill always shown (689-709/897-908); history persists (PutConversation), no saved cue (C251). Canonical safe-to-spend = b8SafeToSpend (budgets.go:97 liquid-billsLeft-goalNeeds). (Full per-step detail in tasks #455/#456.)
1. [ ] **R24-matcher (pure)** — NEW internal/insights/localqa/matcher.go: Match→7 intents (Balance/SpendByCat/SafeToSpend/NetWorth/Bills/Goals/Health); afford separate + test.
2. [ ] **R24-answerer (pure)** — NEW localqa/answerer.go: Source iface + Answer; SafeToSpend INLINES b8 (not net-worth); Health→healthscore.Evaluate (R27); + test.
3. [ ] **R24-source-adapter** — wasm: localqa.Source on appstate; EXTRACT buildHealthInputs→internal/insights/healthinputs.go (cycle-avoid); expose Bills/Goals.
4. [ ] **R24-c245-fix** — insights.go sendText ~238: errMsg.Set("") FIRST line. One line.
5. [ ] **R24-chat-firstpass (C244)** — insights.go 339-360: after afford, before key gate → localqa.Match→Answer→Role:"local" turn+persist+return. Precedence afford>localqa>OpenAI.
6. [ ] **R24-send-btn (C246)** — insights.go ~796: remove noAI→Fragment(), ALWAYS render Send.
7. [ ] **R24-key-explainer (C247/C248, SHARED w/ R10)** — NEW internal/ui/keyexplainer: KeyExplainer{Purpose,OnSettings} headline+cost+platform.openai.com+privacy+example cards; replace keyHintNode + documents_image_import.go:75.
8. [ ] **R24-aria (C249)** — Send aria-label + input askLabel + 2 i18n.
9. [ ] **R24-model-pill (C250)** — composer: when key!="" model + session cost; nothing when no key.
10. [ ] **R24-sysprompt+saved (C251)** — move Edit-prompt into compact ⚙; "saved on this device" footnote (no backup implication).
> Risks: intent collisions (afford>localqa>OpenAI); SafeToSpend MUST use b8 liquid (coordinate R15); explainer dup R10 (shared first); model pill gated key!=""; buildHealthInputs extraction cycle.

### ★ IMPL plan — R22 local credit-health proxy (C208-C211, from R22 design)
> ledger.Utilization(balance,limit)→(pct,ok) (ledger.go:277-290, ok=false if limit<=0), pure, used by R27. Account cc fields (entities.go:74-101): CreditLimit(86)/InterestRateAPR(87)/MinPayment(88)/DueDayOfMonth(89, 1-28)/BalanceAsOf(83); TypeCreditCard (enums.go:39)→ClassLiability. Util surfaces ONLY accounts.go:444-454 (per-card subtitle, no actions) + health.go:118-148 (aggregate = 1 of R27's 5 factors). NO /credit route, NO util history (C210; reuse uistate/healthtrend.go pattern). C211 CreditLimit APPEARS wired in inline edit (accounts_row.go:163/197/226/463 climS) — likely QA-verify not code. On-time = ephemeral inference (smartengine/bills.go:357-389), no history. No OpenedAt (age proxy via BalanceAsOf, underestimates). KEEP DISTINCT from R27.
1. [ ] **R22-c211-verify (prereq)** — accounts_row.go:163/197/226/463 QA-verify CreditLimit renders+saves in cc inline edit; fix if a render path hides it. Accurate limits gate ALL util math.
2. [ ] **R22-engine (pure)** — NEW internal/credithealth/credithealth.go: Inputs{Accounts,Balances,Transactions,Now}; CardUtil{UtilPct(-1 no limit),Target30=bal-(limit*30/100),Target10,Band}; AggUtil{CardsMissingLimit}; on-time proxy (DueDayOfMonth 3mo, reuse bills.NextDue+bl5 window, -1 unset); age via BalanceAsOf (-1 if zero); ProxyScore=0.55*util+0.30*onTime+0.15*age (redistribute when unavailable); Band; Disclaimer = typed CONST (not FICO/bureau). Reuse ledger.Utilization + share healthscore util thresholds. + test (per-card/targets/on-time 3of3&1of3&unset/age/disclaimer-always/weights=1.0).
3. [ ] **R22-history** — NEW uistate/credittrend.go (js,wasm; mirror healthtrend.go): CreditSnapshot{Month,UtilPct,ProxyScore,Band}; key "cashflux:credit:trend"; cap 24; Record/Use via kvGet/kvSet; forward-only, building-state <3.
4. [ ] **R22-snapshot-wire** — credit.go: on /credit render (match healthtrend cadence) RecordCreditSnapshot.
5. [ ] **R22-screen (C209)** — NEW screens/credit.go + register /credit (screens.go Phase 2, nav.credit): disclaimer callout (top, "Estimate" not "Score", 0-100 not 300-850); aggregate util; CardsMissingLimit callout; proxy score + factor breakdown; per-card pay-down targets (hide met); util trend chart (building-state); link →/health. DISTINCT from R27 (no household factors).
6. [ ] **R22-nav** — "Credit Health" → /credit near Health.
> Risks: proxy-vs-FICO honesty (typed Disclaimer, "Estimate", 0-100); C211 likely already-wired (verify-first); on-time DueDayOfMonth often unset (-1, redistribute, mirror R16 window const); age via BalanceAsOf underestimates (disclose, no OpenedAt this ticket); util needs CreditLimit (CardsMissingLimit inflates if hidden — surface); history bootstrap empty (building, no backfill); R27 drift (share util thresholds const); multi-ccy (FX like health.go, exclude unrated + notice); statement-vs-current balance (disclose).

### ★ IMPL plan — R33 WCAG-AA a11y remediation (C315-C319, from R33 audit)
> A11y primitives MOSTLY SHIPPED: ui/controls.go Segmented (role=radiogroup/radio + aria-checked + roving tabindex + arrow-keys, 131/144), Toggle (role=switch), Swatch (role=radio); ui/chart.go AreaChart role=img+aria-label (24/65-68); ui/chartd3.go Chart role=img+aria-label (77-83); aria.go errAttrs/errText. Theme system SHIPPED (prefs.go:35-41 Dark/Light/System; /appearance appearance.go:48-60 + settings link) but NOT discoverable from chrome (C317). GENUINE GAPS: C316 LIGHT-MODE banner contrast (--accent-dim has NO light override → .sample-banner-text #333 on #1f2c24 ≈ 1.16:1 CATASTROPHIC in light; dark already 5.86:1 ok); C315 TopBar menu btn (shell.go:729) title but no aria-label, decorative spacer Span (shell.go:674) no aria-hidden, HouseholdCard icon (shell.go:694) not aria-hidden, chartd3 Label empty-default (80-81); C318 Segmented component correct but CALLSITES omit Label (ResolutionControl shell.go:908; appearance.go) → anonymous radiogroup; C319 DashboardLayoutControls EXISTS (dashboard.go:1175-1205) but moved to Settings, NO dashboard-canvas Customize affordance.
1. [ ] **R33-contrast (C316, CSS)** — web/index.html ~after 2200: `[data-theme="light"] .sample-banner { background:#e8f2ec; }` → #333 on #e8f2ec ≈ 11:1 (dark already 5.86:1). 1 line.
2. [ ] **R33-aria-names (C315, markup)** — shell.go:729 menu btn aria-label=topbar.menu; shell.go:674 spacer Span aria-hidden; shell.go:694 HouseholdCard Icon aria-hidden; chartd3.go:80 aria-label fallback "Chart" + audit dashboard.go Chart() callers pass Label.
3. [ ] **R33-segment-labels (C318, markup+i18n)** — pass Label to Segmented callsites: ResolutionControl (shell.go:908) Label=resolution.granularity (+en.go key); appearance.go pass Label + drop redundant role=group wrapper. Audit ~13 Segment callsites.
4. [ ] **R33-theme-toggle (C317, new UI)** — shell.go:744-751 topbar-controls: compact Segmented/cycling btn Dark/Light/System ↔ UsePrefs().Theme → savePrefs + ApplyTheme(LoadTheme()) (copy appearance.go:27-38, don't reimpl); aria-label="Color theme".
5. [ ] **R33-bento-customize (C319, new UI)** — dashboard.go: "Customize" btn (aria-label, keyboard-reachable) → opens settings to layout section (uistate.UseSettings().Set). Controls already exist in Settings.
> Risks: --accent-dim no global light override (scoped fix safe; broader theme-engine change hits pills/badges); custom accent luminance (#1f2c24 is default only — verify engine computes --accent-dim consistently); chart aria-label ideally encodes period; Segmented 13 callsites (audit each); theme toggle same UsePrefs atom + savePrefs (no dup); radiogroup arrow-keys already done.

### ★ IMPL plan — R34 help/support/trust surface (C325-C329, from R34 design)
> Cheat sheet ALREADY EXISTS: buildHelpOverlay (shortcuts.go:144-228) triggered by `?` + palette "Keyboard shortcuts" cmd (paletteCmd shortcuts.go:286 keywords incl "help") — C327 "help returns nothing" INACCURATE; real gap = NO visible entry point. Shortcuts: Ctrl/Cmd+K palette, ? cheat, Alt+1-9 nav, Alt+N quick-add, undo/redo, Enter/Esc, Shift+Arrows (i18n en.go:283-293). About: settings.go:987-991 footer = version.Label() + external CHANGELOG <a>; CHANGELOG.md keep-a-changelog at root. GENUINE GAPS: C325 NO feedback/bug link (only changelog link); C326 no INLINE what's-new (external only, CHANGELOG not embedded); C328 NO /help|/faq|/about route (screens.go:57-89); C329 no feature-discovery tips (Smart tooltips smart_affordances.go:84 = AI opt-in; R5 wizard #448/#449 not built). Public repo github.com/monstercameron/CashFlux → feedback = issues link, no server.
1. [ ] **R34-help-entry (C327)** — shell.go rail/footer: visible "?" btn → toggleHelpOverlay() + "Ctrl+K · ? Help" hint; + palette cmd "Open help/shortcuts". Cheat sheet already registry-generated (no drift).
2. [ ] **R34-feedback (C325)** — settings.go:987-991: 2nd <a> "Report a bug / request a feature" → .../issues/new/choose; + palette cmd; + i18n help.reportBug. Trivial.
3. [ ] **R34-whatsnew (C326)** — NEW screens/changelog.go: //go:embed CHANGELOG.md (embed at repo-root main.go, pass bytes down — avoid ../../) → parse keep-a-changelog ([Unreleased] first, ### Added/Fixed/Changed) → accordion; register /changelog; settings.go:989 link→in-app nav; palette cmd; i18n help.whatsNew.
4. [ ] **R34-help-faq (C328)** — NEW screens/help.go + /help route: Getting Started (R5 wizard link when shipped), Keyboard Shortcuts (toggleHelpOverlay), FAQ (static internal/help/faq.go slice, filter via cmdmatch, ~10-15 Q: storage/privacy/import-export/categories/sync/subs), About/Version (absorbs C293, no /about route). Palette cmd.
5. [ ] **R34-discovery-tips (C329, compose R5)** — empty-state tip cards in accounts.go/budgets.go/goals.go(+lists): icon + one-line + CTA + OPTIONAL wizardStep (no-op "" until R5 #448/#449); render ONLY when list empty (never naggy, no dismiss/storage); i18n per section.
> Risks: go:embed path (embed at root main.go, pass down); keep-a-changelog parse ([Unreleased] first, simple line-split); FAQ drift (separate faq.go + review-on-feature); feedback URL hardcoded correct repo; tips empty-state-only = not naggy; wizardStep optional+guarded (don't break build pre-R5); C293 About absorbed into /help; group new palette cmds under "Help".

### ★ IMPL plan — R30 security hardening (C282-C288, from R30 design)
> KEY: the gate is NOT cosmetic — dataset IS genuinely encrypted at rest (migrateDatasetAtRest applock.go:63/72 → cryptobox.Envelope PBKDF2-SHA-256 600k → AES-GCM-256 via crypto.subtle, datasetcrypto.go:54-145, non-extractable key). BUT the gate hash is the WEAK LINK: HashPasscode (applock.go:58-61) = SHA-256(salt+passcode) → 6-digit PIN brute-forces offline in ms on GPU → recovers passcode → decrypts the PBKDF2 envelope (C284). pwcheck pkg EXISTS+correct (pwcheck.go MinPINLen=6 + trivial-seq + blocklist) but UNWIRED — "000000"/"123456" pass setup (applockgate.go:411-432) → C287 = one import. C286: applockgate.go:125 gate card background:var(--surface,#ffffff) → near-white #f4f4f5 text ~1.06:1 on the white FALLBACK. C285/C288: applock heading renders (settings_section.go:276) but absent from settingsNavKeys (settingssectionnav.go:22-36); no "Security" heading/route. C282: NO WebAuthn. C283 cloud MFA = NO backend → OUT OF SCOPE (theater without a server). golang.org/x/crypto/argon2 = pure Go, compiles to wasm.
1. [ ] **R30-strength (C287, wire existing)** — applockgate.go:411-432: import internal/pwcheck; pwcheck.Check(pass,pwcheck.PIN,nil); gate on !OK + show Issues[0]; 0-4 score bar in buildAppLockSetup. pwcheck already built+tested.
2. [ ] **R30-kdf (C284, pure)** — applock.go: HashPasscodeV2 = argon2.IDKey(pass,salt,t=3,mem=64MB,threads=1,len=32) base64 prefixed "v2:"; Verify() v2:→argon2id else bare-hex→SHA-256 (back-compat); WithPasscode always V2. Confirm x/crypto in go.mod. +test. Benchmark wasm (tune to 32MB/t=2 if >2-3s; "securing…" spinner).
3. [ ] **R30-migrate (C284)** — applock.go Verify(): after successful SHA-256 verify, re-hash argon2id + persist (return upgraded bool → applockgate.go:158-161 saves). Transparent, next-unlock +test. Only fires when passcode typed.
4. [ ] **R30-contrast (C286, CSS)** — applockgate.go:125: var(--surface,#ffffff) → var(--surface,#1a1a1d) (match setup modal :386). One line.
5. [ ] **R30-security-section (C285/C288)** — settingssectionnav.go:22-36 add "applock.section"/"settings.security"; settings_section.go:~271 group app-lock under a "Security" heading + i18n.
6. [ ] **R30-webauthn (C282, larger/optional)** — NEW internal/webauthn/webauthn.go (syscall/js navigator.credentials.create/get + PRF extension → 32-byte PRF output); datasetcrypto.go decryptDatasetWithPRF (PRF as PBKDF2 base/AES key); applockgate.go "Unlock with passkey" + setup registration; credential ID in localStorage. FEATURE-DETECT isUserVerifyingPlatformAuthenticatorAvailable + PRF (Chrome116+/Safari17+/FF119+) — hide if unavailable; passcode fallback ALWAYS; file:// = no WebAuthn. Assess before committing.
7. [ ] **R30-mfa-deferred (C283)** — comment in settings_section.go near backend/sync: "MFA deferred until sync backend ships." DO NOT implement (no server = theater).
> Risks: argon2 params vs wasm single-thread perf (benchmark; tune; spinner); migration only on typed-passcode unlock (biometric-only needs fallback trigger); WebAuthn PRF support not universal (feature-detect FIRST); RP ID (localhost ok, file:// = none); session plaintext key in wasm mem (accepted browser-model tradeoff); strength upgrade only on voluntary change; no lockout (argon2 ~1s natural throttle); salt in localStorage fine; cloud MFA genuinely OOS.

### ★ IMPL plan — R29 household roles (C273-C276, from R29 design)
> MOSTLY SHIPPED (honest SOFT-role model, NOT access control): MemberRole enum (entities.go:17, RoleOwner/RoleAdmin/RoleViewer 29/34/37) + Member.Role (entities.go:50, zero=RoleAdmin migration); memberrole pkg COMPLETE (CanManageMembers/CanEditEntities/CanViewOnly/Resolve/Label/ParseRole/DefaultRole) but NOTHING in UI calls the predicates. C275 role field SHIPPED in BOTH forms (memberaddform.go:101-108 + members.go:415-422) + badge (members.go:432) → CLOSE. C274 single-device disclosure SHIPPED (members.go:228-231 data-testid members-single-device-note + i18n en.go:1488 + e2e c274) → CLOSE. activemember switcher (activemember.go:21-26) scopes ONLY transactions (transactions.go:82-85) + dashboard KPIs (dashboard.go:82-91) — NOT accounts/budgets/goals/reports (= C277/C278, separate). Single app-wide passcode (R30), no per-user auth. GENUINE GAPS: predicates wired to NO UI action; no role-driven defaults; no in-form disclosure. HARD per-user auth/login = backend/OS-accounts = OUT OF SCOPE local.
1. [ ] **R29-audit-close (C273/C275)** — verify role SelectInput options come from memberrole (add AllRoles() if hardcoded — drift guard); DefaultRole(true)=RoleOwner (R5 first member); +ParseRole/DefaultRole/AllRoles test. Close C275 (shipped) + C273-partial.
2. [ ] **R29-uiperms (pure)** — NEW internal/memberrole/uipermissions.go: UIPermissions{CanManageMembers,CanEditEntities,ShowAdminActions,ShowViewerHint}; ForRole(role,activeIsSpecific)→UIPermissions +test (3×2=6). Foundation for #3/#6.
3. [ ] **R29-soft-gate (C276)** — members.go: ForRole(resolvedRole, activeMemberID!="") → SOFT-HIDE Add/Delete/Change-Role for non-Owner active member (hide trigger NOT disable save; tooltip "switch member to restore"). NOT enforcement.
4. [ ] **R29-default-active (heuristic)** — NEW uistate/activememberdefault.go: DefaultActiveMember(members)→id — first load, persisted empty && exactly one RoleOwner → that member else "Everyone". Pure +test. One-time init.
5. [ ] **R29-creation-defaults (compose MIA)** — account/budget/goal add forms: pre-select OwnerID=active member (when specific); Scope Viewer→Shared, Owner/Admin→Individual(specific)/Shared("Everyone"). DEFAULTS only (overridable); does NOT filter existing (C277/C278).
6. [ ] **R29-viewer-hint (C276)** — shared layout: activeMemberID!="" && Viewer → soft banner "Viewing as [Name] (Viewer) — switch member to make changes"; dismissible per session; NOTHING blocked. Pure isViewerHint().
7. [ ] **R29-inform-disclosure (C276)** — memberaddform.go + members.go edit: helper text under role SelectInput "Roles organize your view + set smart defaults; NOT access controls — all data shared on this device" (i18n memberrole.softDisclosure); point-of-use (don't dup main C274 note).
8. [ ] **R29-e2e** — extend e2e/c274_single_device_note.mjs to assert the in-form disclosure if new data-testid.
> Risks: role-as-enforcement temptation (soft-hide MUST tooltip "anyone can switch member"; real enforcement = per-user auth = OOS); migration (Role==""→RoleAdmin; multi-member w/o Owner → heuristic "Everyone"; prompt to designate Owner); compose-not-conflict MIA (defaults on CREATION only, NOT filter existing); Viewer soft-hide needs restore tooltip or reads as bug; scope gap = C277/C278 (don't conflate); C274 shipped (don't dup); AllRoles() single-source.

### ★ IMPL plan — R31 pricing/plan UX (C300-C304, from R31 design)
> HONEST-SCOPE CORRECTION: billing is REAL, not hypothetical — Stripe $34.99/yr + $3.99/mo + 14-day trial (i18n en.go:933-988), UpgradeSheet (upgradesheet.go), SubscriptionBanner (trial/past-due/canceled), "Manage subscription"→Stripe portal (settings_section.go:248). Problem = INVISIBLE + ONE-SHOT, not dishonest. C300/C301: UpgradeSheet only reachable via CloudMention; CloudMention (cloudmention.go:32/36) BOTH buttons write permanent dismiss "cashflux:cloud-mention-dismissed"=1 (read render :24) → after first tap UpgradeSheet permanently unreachable, NO reset. C304: Cloud&server tab (settings_section.go:191-251) = raw infra; plan/billing subsection (232-251) DOUBLE-GATED behind If(p.CloudSelected) → free/local users NEVER see pricing. C303: prices+trial in i18n but NO "Free" tier label, NO comparison, NO plain-language boundary; NO /plans route; billingStatus.Plan received but never rendered. Free = everything local forever, no account.
> NOTE for Cam: confirm if the hosted backend is publicly DEPLOYED → affects "Start free trial" (live) vs "Join waitlist" CTA framing (runtime backend-availability check, not a code decision).
1. [ ] **R31-i18n** — en.go: plans.pageTitle/freeTitle/freeBody/cloudTitle/cloudBody/startTrial/manageSub/doNotRemind/setupLink; REUSE settings.cloudPriceAnnual/Monthly/cloudTrialNote/cloud.upgradeTrust (REAL values).
2. [ ] **R31-plans-screen (C300/C303)** — NEW screens/plans/plans.go + register /plans (screens.All() auto-wires nav): plain-language Free (everything on device, no account/expiry) vs Cloud (sync+backup+bundled AI, $3.99/mo or $34.99/yr, 14-day trial, cancel anytime) w/ REAL prices; [Start free trial]→Settings→Cloud; [Manage subscription]→OnOpenPortal. NO dark patterns (show BOTH prices, no urgency). Canonical disclosure surface.
3. [ ] **R31-reengageable (C301)** — cloudmention.go:15/24/32/36: permanent dismiss → snooze (timestamp "cashflux:cloud-mention-snoozed", re-surface ~30d); treat legacy "1" as snoozed-long-ago (graceful) OR honor as explicit opt-out; "Learn more"→nav /plans (not ShowUpgradeSheet direct); explicit "Don't remind me" on Plans (user-chosen permanent).
4. [ ] **R31-ungate-billing (C304)** — settings_section.go:232: remove/relax If(p.CloudSelected) so plan heading+price+trial show when backend on (or always read-only); KEEP on/off+mode at TOP, plan info BELOW connection config (don't disrupt self-host users).
5. [ ] **R31-reframe-link (C304)** — retitle "Cloud & server" → "Cloud sync setup (advanced)"; Plans [Start free trial]→Settings→Cloud scroll-to-subscribe. Plans = DISCOVER pricing, Cloud tab = SET UP after deciding.
6. [ ] **R31-plan-name (low-pri)** — subscriptionbanner.go:21-25: render billingStatus.Plan (Annual/Monthly) on Plans for subscribed users (1-2 lines).
> Risks: HONESTY — billing is REAL (no fake coming-soon/waitlist unless backend genuinely undeployed — confirm w/ Cam); legacy permanent-dismiss re-surface on snooze change (treat "1" gracefully or keep old key); trial claim consistent Plans↔Settings; Cloud&server reframe (toggle/mode top, plan below — don't disrupt self-host); R32 sync same "multi-device sync" language; R34 nav placement; no dark patterns (both prices, easy opt-out, C45); ShowUpgradeSheet may orphan (keep utility, R32 may reuse).

### ★ IMPL plan — R32 cross-platform + sync (C306-C310, from R32 design)
> C306 PWA install + C307 install button = ALREADY DONE by parallel agent (VERIFIED): manifest.webmanifest has favicon.svg+icon-192+icon-512 (any+maskable), all icons present, index.html:67-75 apple-touch-icon + apple-mobile-web-app-capable + theme-color + manifest; sw.js (cashflux-v271) caches icons + navigate-fallback; #installBtn (index.html:2827-2855) captures beforeinstallprompt + prompts + hides on appinstalled. C306/C307 tickets STALE → close. C309 REAL DATA-LOSS BUG: syncstate.go ShouldApplyRemote = pure LWW (remote.After(local)); sync_client.go:~168-178 removeQueuedSyncMutation called BEFORE the !resp.Accepted check → server-rejected push silently DEQUEUES + drops local data, no signal. C310: proto device_id (cashflux.proto:15,69) + session-revocation endpoints (CHANGELOG:1875) but NO client pairing/add-device UI. C308 native: NONE.
1. [ ] **R32-c306-c307-close** — close C306+C307 (DONE; do NOT touch manifest/icons/installBtn — clobbering breaks PWA). Visual-check maskable safe-zone.
2. [ ] **R32-ios-hint (C307, client-only)** — index.html after installBtn IIFE (~2855): iOS-Safari branch (/iP(hone|ad|od)/i && !navigator.standalone) → dismissible "tap Share → Add to Home Screen" banner. iOS never fires beforeinstallprompt — static hint only. Additive.
3. [ ] **R32-c309-dequeue-fix (DATA LOSS, client-only)** — sync_client.go:~168-178: move removeQueuedSyncMutation INSIDE resp.Accepted==true; on !Accepted keep queued + retry counter; after 3 fails → syncStatus "conflict". HIGH severity, low-complexity, no backend dep + test.
4. [~] **R32-conflict-ui (C309)** — **PARTIAL 2026-06-25:** the data-loss core is fixed (C309 above) and a Settings **Restore / Discard** affordance exists for the backed-up loser. Remaining enhancement: render the "conflict" state on the **chip** itself (amber + ! "tap to resolve") opening the same Keep-local / Discard-and-pull choice inline (currently the chip shows `conflict`; the resolution UI lives in Settings). Force-push still needs a proto force flag (backend).
5. [ ] **R32-server-conflict-meta (C309, backend)** — internal/server/sync.go: on Accepted:false include server UpdatedAt+Version (proto field add) so client shows "server X min newer". Coordinate cmd/cashflux-server. After #3.
6. [ ] **R32-connected-devices (C310, backend-coupled)** — NEW app/devices.go: GET /v1/auth/sessions list + Revoke (endpoint exists CHANGELOG:1875, verify shape). Visibility+revocation FIRST.
7. [ ] **R32-add-device-pairing (C310, backend-coupled)** — follow-on #6: backend short-lived pairing token; client "Pair new device" displays token; new device enters on first launch. New backend endpoint. Coordinate R31. SEQUENCE LAST (don't add devices before C309 fixed).
8. [ ] **R32-c308-native-note** — TODOS C308: "separate major initiative — PWA install (done) is the pragmatic path; native = Capacitor (untested w/ Go-WASM, WKWebView memory/large-binary risks) or rewrite (months). Out of scope this pass."
> Risks: parallel-agent web/ overlap (C306/C307 DONE — do NOT touch manifest/icons/installBtn); no iOS beforeinstallprompt (manual A2HS); maskable safe-zone clip; C309 silent data-loss HIGH (dequeue fix client-only — prioritize); CRDT deferred (detect-don't-drop); backend coupling C309-force/C310; SW stale-cache may mask conflicts (network-first mitigates; F47); native cost honest; ORDER fix-C309-before-C310.

### ★ FEATURE-REVIEW IMPL — F5 fast manual entry / Quick-Add (C39-C47, parallel research)
> quickadd.go. C41 default acct = "first non-investment asset" (78-85), indeterminate; Prefs.DefaultAccountID (70-73) only if configured. C43 NO autofocus; FlipPanel UseEffect (flippanel.go:94-96) focuses Account <select> (213). C42 date <input type=date> (232) widget swallows Tab; FlipPanel trap (flippanel.go:128-144) lacks {capture:true}. C40 NO save-and-add-another. C39 NO recent-payee autocomplete (ui.Combobox inputs.go:112-136 EXISTS unused; reports.TopPayees uses Desc not Payee). C44 two-click mouse (Alt+N shortcuts.go:95-97). C45 dropdown = a.Name only (158-161), incl archived, no type cues. C46 NO Payee field — form sets only Desc, never domain Transaction.Payee (entities.go:121); rules match payee+desc (rules.go:72). C47 reviewed checkbox works (label clarity only). R7 OVERLAP: R7-selflearn touches rawDesc→catAssist (182-206).
1. [ ] **F5-autofocus (C43)** — quickadd.go: UseEffect when open → querySelector [data-testid=txn-add-amount].focus() (nil-check; setTimeout for wasm paint race).
2. [ ] **F5-tab (C42)** — quickadd.go:232 date → type=text pattern \d{4}-\d{2}-\d{2}, OR flippanel.go keydown {capture:true}.
3. [ ] **F5-default-acct (C41, pure)** — NEW internal/accountselect/accountselect.go DefaultID(accounts,txns,memberDefault) (memberDefault→most-used-90d non-archived non-investment→first checking/debit/savings→first non-investment) +test; wire quickadd.go 64-91.
4. [ ] **F5-dropdown-cues (C45)** — quickadd.go:158-161 filter !Archived + " · "+humanizeType (move to internal/ui/format.go or inline — screens cycle).
5. [ ] **F5-reviewed (C47)** — quickadd.go:233-235 move checkbox below save, muted; i18n "Skip auto-review flag".
6. [ ] **F5-fewer-clicks (C44)** — addmenu.go:104-108 + button title "(Alt+N)"; palette alias "t"→quick-add.
7. [ ] **F5-payee (C39/C46)** — NEW internal/quickpayee/quickpayee.go RecentPayees(txns,n) (distinct Payee, fallback Desc, dedup, ≤20) +test; quickadd.go payee UseState+onPayee (stable pos), reset, set Transaction.Payee (138), ui.Combobox datalist, validation desc OR payee, update rules.Category call (194). COORDINATE R7.
8. [ ] **F5-save-add-another (C40)** — quickadd.go: keepOpen UseState (stable pos); extract doSave(); "Save + Add Another" btn in body (not FlipPanel footer) → doSave+reset+focus amount.
> Risks: hook stability (payee/keepOpen unconditional before open guard); autofocus wasm paint race; date Tab Chrome-specific (capture:true or type=text loses mobile picker); humanizeType circular import; payee validation (desc OR payee); R7 overlap (sequence/same commit); default-acct cap 90d/200txns.

### ★ FEATURE-REVIEW IMPL — F9 account types + net-worth clarity (C72-C75, parallel research)
> AccountType enum (enums.go:34-46) = 11 types, NO retirement/crypto, NO AccountSubType (deferred TODOS:356); Class() default→ClassAsset. C75 single TypeInvestment undifferentiated. C72 dashboard (dashboard.go:207-234): net-worth tile=nw.Net (208), liabilities tile (228); ASSETS (nw.Assets, selectors.go:29-35) COMPUTED but NEVER rendered (=C212) → can't reconstruct Net=Assets−Liab (C72 root; count-up C214 cosmetic). ledger.NetWorthExplained (networth_explained.go:31-71) clean. C74 LockUntil (entities.go:97) asset-only, gated behind advOpen (accountaddform.go:242-243). R23 (Holding/AccountSubType/portfolio) pending.
1. [ ] **F9-enum (pure)** — enums.go:34-46 add TypeRetirement+TypeCrypto consts+AllAccountTypes (Class() default already→Asset) +enums_test. Migration-free. LEAVE AccountSubType for R23.
2. [ ] **F9-labels** — type→label map: "Retirement"/"Crypto".
3. [ ] **F9-networth-clarity (C72)** — dashboard.go:207-233 render nw.Assets+Liabilities as labeled sub-components under Net ("Total Assets"/"Total Liabilities") — PREFER over a 5th tile; add nw.Assets to count-up key. Render-only (already computed).
4. [ ] **F9-exclusion-notice** — dashboard.go: when MissingCurrencies show nw.ExcludedAccounts count (partial net worth clarity).
5. [ ] **F9-lockuntil-surface (C74)** — accountaddform.go:234-243 lift LockUntil out of advOpen for TypeRetirement (+low-liquidity); label "Penalty-free date (optional)".
6. [ ] **F9-type-cues (C75)** — accounts_row.go badge "Tax-advantaged"(retirement)/"Volatile"(crypto); crypto StabilityScore hint (0-15). Cosmetic, R23-additive.
> Risks: enum migration-free; tax-advantaged = LABELS ONLY (Roth-vs-trad = R23 AccountSubType, don't bloat); sub-component < 5th-tile (layout); C72 root = display omission NOT formula (don't conflate count-up C214); R23 boundary (TypeRetirement now, subtype/AssetClass in R23); crypto manual valuation (badge + tooltip, live feed deferred); NetWorthExplained holdings = R23.

### ★ FEATURE-REVIEW IMPL — F33 reports + export (C236-C243, parallel research)
> reports pkg (17 logic+17 test). C237 NO YoY toggle (always w.Shift(-1), reports_screen.go:191). C238 ledger.PercentChange (ledger.go:298-306) ok=false when prev==0 → badge SUPPRESSED (1047); rollup.go:38-41 reimplements inline a.Prior>0 (diverges neg baseline). C239 web/chart.js bar height Math.abs (148-178) can't go neg BUT flat/single-bar → .nice() [0,0] → degenerate scale → NaN → SVG error (root=degenerate domain). C240 8 export surfaces (1 dropdown 700-742 + 7 per-card 768-1194). C241 "Covering" ISO ALREADY FIXED (666 pr.FormatDate) — SKIP. C242 custom-field/deductible hidden behind showAdvanced (904-919). C243 NO report-type selector. C236 NO PDF/print.
1. [ ] **F33-delta (C238, pure)** — ledger.go:298-306: DeltaKind + Delta(curr,prev)→{Pct,Kind} (new/gone/zero/pct; handle prev<0 magnitude) +test.
2. [ ] **F33-rollup-fix (C238)** — rollup.go:38-41 inline a.Prior>0 → ledger.Delta (neg-baseline fix).
3. [ ] **F33-categoryspend (C238)** — reports.go:27-33/106 HasDelta/DeltaPct → Delta DeltaResult; update CSV/rollup consumers.
4. [ ] **F33-yoy (C237, pure)** — NEW reports/yoy.go YoYPrior(w) 12-mo shift +test; reports_screen.go ~188 yoyMode + "MoM/YoY" toggle (disable <13mo).
5. [ ] **F33-svg-fix (C239)** — web/chart.js ~89: yMin==yMax → expand domain ([0,1]/±spread or hide Y single-bar). No Go.
6. [ ] **F33-delta-badge (C238)** — reports_screen.go:1043-1076 render Kind text ("New"/"Gone"/"–") not suppress; CSV emits strings.
7. [ ] **F33-report-selector (C242/C243)** — reports_screen.go top: reportType tabs (Overview/Custom Fields/Deductibles/Tax Summary); retire showAdvanced; preserve filter/period/sort on tab change.
8. [ ] **F33-csv-consolidate (C240)** — remove 7 per-card buttons (768-1194); expand the one dropdown (700-742)+custom-field/deductible (conditional); ExportFilename tab-scoped.
9. [ ] **F33-pdf (C236)** — NEW web/print.css (@media print hide nav/buttons, page-break, svg width:100% height:auto) + "Print / Save as PDF" dropdown → window.print() via JS shim. (jsPDF ruled out.)
> Risks: prior-zero "New" not "∞%"/"−100%"; rollup neg-baseline magnitude+sign; SVG [0,0] expand looks odd for currency (use [0,1]/hide axis); YoY needs 13+mo (disable+tooltip); selector preserves period/search/sort; window.print() browser-dependent (Firefox SVG clip — svg width:100%); C241 skip; ExportFilename tab-scoped.

### ★ FEATURE-REVIEW IMPL — F44 data ownership / backup (C294-C299, parallel research)
> C294 Export-JSON (settings.go:1287)→ExportJSON plain MarshalIndent (dataset.go:105-112); Artifact.Bytes (entities.go:475) stripped nil on upload (artifact_ops.go:49-58, blobs→IndexedDB); ExportJSONWithBlobs/RedactedWithBlobs + rehydrateArtifactBytes EXIST (artifact_ops.go:83-129) but button + backupEverything (backupall.go:55) DON'T call them → blobs OMITTED. C295 importJSON (settings.go:1336-1353)→ImportJSON immediately, NO confirm (restoreFromBackup backupall.go:91-106 DOES — copy pattern). C296 TransactionsToCSV (csv.go:21-58) = 12 txn cols, "Export CSV" unlabeled-partial (en.go:1040). C297 backupEverything palette-only (shortcuts.go:331), absent Settings→Data (settings_section.go:255-261). C298 Data jump-nav SHIPPED (settingssectionnav.go:34); wipe confirm generic "Confirm" (settings.go:1371). C299 NO last-backed-up surfaced (recordBackupNow settings.go:1293 stores nothing user-facing).
1. [ ] **F44-roundtrip-test (pure, FIRST)** — NEW internal/store/export_test.go: Dataset w/ Artifact{Bytes:"sentinel"} → MarshalIndent → unmarshal → assert round-trips (lossless rule). Pure.
2. [ ] **F44-import-rehydrate (C294)** — artifact_ops.go: rehydrateArtifactBytesOnImport(dataset) → StoreBlobForArtifact per non-nil Bytes then clear; verify rehydrate covers all artifact types.
3. [ ] **F44-export-blobs (C294)** — settings.go:1287 ExportJSON→ExportJSONWithBlobs; backupall.go:55 RedactedWithBlobs. 2-line.
4. [ ] **F44-import-confirm (C295)** — settings.go:1336-1353 wrap ImportJSON in confirmModal (copy backupall.go:91-106); i18n "Replace all your data… can't be undone" + destructive "Replace data" (optional pre-parse count).
5. [ ] **F44-csv-label (C296)** — en.go:1040 "Export CSV"→"Export transactions (CSV)" + hint "Transactions only — use Export JSON for a full backup".
6. [ ] **F44-backup-btn (C297)** — settings_section.go:255-261 primary "Back up everything" (top of Data) → backupEverything() (reuse cmd.backupEverything label).
7. [ ] **F44-wipe-label (C298)** — en.go + settings.go:1371 "settings.wipeConfirmBtn"="Erase all data" destructive (jump-nav already shipped — don't re-add).
8. [ ] **F44-last-backed-up (C299)** — SettingsKV "lastBackedUpAt"; recordBackupNow() writes RFC3339 (verify called from ALL backup paths); settings_section.go Data "Last backed up: <date>"/"Never backed up".
> Risks: base64 blob bloat ~33% (size estimate/toggle/tooltip); lossless relies on IDB populated (test serialization separately); import-overwrite destructive (red, confirmModal destructive prop?); recordBackupNow coverage (all paths or misleads); R8 dedup-import same code (coordinate); MIA SettingsKV namespace; C293/R34 settings_section.go layout; Data jump-nav shipped.

### ★ FEATURE-REVIEW IMPL — F26 debt payoff planner (C195-C203, parallel research)
> payoff pkg SOLID: Project/BuildPlan pure snowball+avalanche → Plan{Schedule,Order,ClearedMonths,TotalInterest,TotalPaid,Months}; SuggestedExtra (extra.go:11) + DebtFreeMonth (date.go:11) SHIPPED. C195 FX BUG: planning.go:664-672 owed=bal.Abs().Amount + payoff.Debt{} NO conversion — EUR travelcard (sample.go:432) fed raw alongside USD into BuildPlan (currency.ConvertBetween exists currency.go:186, used health.go:83/132, not here). C196 no per-debt table. C197 both shown (785-791) but no months-saved delta (interest-saved only when avalanche wins, rec 735). C198 baseline stale (sample PayoffBaseline Jul-2022 sample.go:851 + currentOwed 677-679 mixed-currency). C199 burn-down avalanche-only (760-781). C200 no /debt route. C201 APR/min not editable. C202 dsExtra "" → 0 → tie (SuggestedExtra btn exists 805, not seeded). C203 x-axis bare float64(i+1).
1. [ ] **F26-aggregate (C195, pure)** — NEW payoff/aggregate.go AggregateDebts(accounts,base,rates,txns)→[]Debt (+native display) via ledger.Balance+currency.ConvertBetween; return missing-rate currencies for a warn; type-filter (exclude R21 installment); +test.
2. [ ] **F26-compare (C197, pure)** — NEW payoff/compare.go Compare(snow,aval)→{MonthsSaved,InterestSaved,FasterStrategy} +tests (tie/snow/aval).
3. [ ] **F26-sample-baseline (C198)** — sample.go:851 remove hardcoded Jul-2022 baseline (or seed relative-to-now); currentOwed uses base-converted sum.
4. [ ] **F26-fx-wire (C195)** — planning.go:656-679 owed loop → AggregateDebts; fix currentOwed (677-679).
5. [ ] **F26-seed-extra (C202)** — planning.go: dsExtra=="" && SuggestedExtra>0 → seed once (guard so user-cleared isn't re-seeded).
6. [ ] **F26-burndown (C199/C203)** — planning.go:760-781: 2 chartspec.Series (aval+snow, pad shorter) + Legend; x Point.Label=DebtFreeMonth(now,i+1).Format("Jan 2006") (verify shim honors Label first).
7. [ ] **F26-detail-table (C196)** — DebtDetailRow component (own comp, no hooks-in-loops): name/native+base balance/APR/min/projected clear.
8. [ ] **F26-inline-edit (C201)** — DebtDetailRow edit → APR%/MinPayment inputs → PutAccount.
9. [ ] **F26-compare-display (C197)** — replace rec (735-737) w/ Compare(): "Avalanche saves N months + $X interest" / "tie at $0".
10. [ ] **F26-route (C200)** — NEW screens/debt.go DebtScreen (extract card) + /debt + sidebar; link from /planning (don't duplicate atoms).
> Risks: FX staleness (ConvertBetween ErrUnknownRate — warn row, don't use raw like health.go:83); R21 coord (installment→amortization not BuildPlan; type-filter); burn-down length mismatch (pad); existing datasets keep stale baseline (ClearPayoffTracking + DEVLOG note); verify D3 shim Label before timestamps; auto-seed once.

### ★ FEATURE-REVIEW IMPL — F32 spending trends + explanations (C228-C235, parallel research)
> C228 highlights (insights.go:1296-1317) plain P, no drill-through (pattern categories.go:103-109 viewTxns→TxFilter→/transactions). C229 TopPayees (payees.go:30-68) single-period; CategoryTrends (trends.go:36-73) category-only — NO payee time-series. C230 NO /insights trend chart. C231 chips suppressed when convo exists (insights.go:804 empty&&len>0). C232 Detect (insights.go:92-139) only MinBaseline guard → "down 100%" mid-month. C233 Anomaly.Delta EXISTS (insights.go:48) but highlightText (1353-1365) never passes it. C234 Ask composer below-fold (859-909). C235 SavedInsight (entities.go:274-280) no Model field; pinText (178-189) no capture; row date-only (1245). OVERLAP R25 (C232/C233) + R24 (C231/C234/C235).
1. [ ] **F32-midmonth (C232, pure, COORD R25)** — insights.go Detect: prorate baseline (×elapsed/totalDays) or suppress current==0 when periodEnd>now + MidMonthZero flag. If R25 detectAllAnomalies first, add THERE (shared via Detect — don't double).
2. [ ] **F32-dollar-delta (C233, COORD R25)** — insights.go:1353-1365 + en.go:1442-1443: pass Anomaly.Delta ("+$X"). Field exists.
3. [ ] **F32-payee-trend (C229, pure)** — payees.go PayeeTrends(txns,buckets,topN)→[]{Payee,Spend[]int64} (reuse TopPayees normalization) +test.
4. [ ] **F32-drill-through (C228)** — insights.go:1296-1317 clickable rows → TxFilter{Category/Payee/period}+nav /transactions (reuse categories.go:103-109); verify Anomaly carries period bounds (add if missing).
5. [ ] **F32-trend-chart (C230)** — NEW insights_chart.go TrendSparkline(CategoryTrends top-5, 6mo) between highlights+pinned (871); ≥2-bucket guard.
6. [ ] **F32-chips (C231, COORD R24)** — insights.go:804 `empty &&`→always; overflow-x:auto.
7. [ ] **F32-ask-sticky (C234, COORD R24)** — composer position:sticky bottom:0.
8. [ ] **F32-pin-attribution (C235, COORD R24)** — entities.go:274-280 add SavedInsight.Model; pinText capture backend; row "date · model"; legacy empty→date.
> Risks: C232/C233 SHARED w/ R25 (do once in Detect, R25 inherits); payee normalization; drill-through needs Anomaly period bounds; trend ≥2mo; chips clutter (h-scroll); C234/C235 R24-owned (coordinate); legacy pins Model=="".

### ★ FEATURE-REVIEW IMPL — F8 transfers (C67-C70, parallel research)
> CreateTransferPair (transfer_ops.go:51) HAS From+To params + same-account guard (58-60); neither leg Reviewed:true nor trigger-suppressed → both hit PutTransaction→RunTriggered→ActionFlagReview (appstate.go:1226-1234) → #needs-review (C68 = trigger-suppression gap NOT authoring). C67 Transfer btn only in account-row overflow (accounts_row.go:537-539); global addmenu.go none. C69 form (accounts_row.go:420-444)=Amount/To/Date/Desc, NO From-selector (doTransfer accounts.go:222 fromID always=row acct). C70 individual delete (transactions.go:250-263)→DeleteTransactionWithTransferPair NO confirm; bulk (295-321) generic; pair-delete (appstate.go:1590-1613) silently removes reciprocal. R19 ActionTransfer SHIPPED (workflow.go:77/282) — calls CreateTransferPair, inherits C68 fix.
1. [ ] **F8-noreview (C68)** — transfer_ops.go ~105-120: Reviewed:true on BOTH legs before PutTransaction (ActionFlagReview skips Reviewed appstate.go:1228) OR SuspendTriggers around both (check re-entrancy) +test. Propagates to R19 free.
2. [ ] **F8-istransfer (C70 prereq)** — entities.go: Transaction.IsTransfer() (TransferAccountID!="") if not present (grep first).
3. [ ] **F8-transfer-form (C69)** — NEW screens/transfer_form.go: From+To SelectInput (filter archived)+Amount/Date/Desc→CreateTransferPair; client same-acct guard (mirror :58-60). Row path delegates w/ From pre-filled.
4. [ ] **F8-delete-confirm (C70)** — transactions.go:250-263: txn.IsTransfer() → ConfirmModal naming paired acct "both legs deleted" before pair-delete (store atomic, R8).
5. [ ] **F8-bulk-confirm (C70)** — transactions.go:295-321: scan selection IsTransfer → append "X transfer entries — both legs removed".
6. [ ] **F8-addmenu (C67)** — addmenu.go add "Transfer" → TransferForm; i18n addmenu.transfer. KEEP overflow shortcut.
7. [ ] **F8-e2e** — add-menu→Transfer→From/To/Amount→both legs appear→delete one→warning→both gone.
> Risks: Reviewed:true scope (system legs only; document); don't suppress triggers globally (breaks R19 goals); same-acct guard (backend authoritative, mirror client); keep overflow (loopstory_93); delete atomicity (R8 store-layer); verify no R19 test asserts Reviewed==false; FX legs future.

### ★ FEATURE-REVIEW IMPL — F6 ledger filters (C48-C57, parallel research; C50/C55 DONE)
> txnfilter FieldTags (multi.go:14) + MultiCriteria.Matches tag-OR (32-46) + tests COMPLETE but transactions.go NEVER references MultiCriteria (C49 100% UI-unwired). C53 amount min/max ABSENT (Criteria txnfilter.go:63-82 12 fields; MultiCriteria) — build. C51 Clear-filters (699) no If-guard. C52 panel = FlipPanel modal (filtertoolbar.go:93-99, role=dialog backdrop occludes). C57 filter-badge (filtertoolbar.go:64) aria-hidden=true (SR-invisible). C48 inline edit (transactions_row.go:154-181) no Tags. C54 anyTags (transactions.go:479-488) iterates `page` not `shown`. C56 no filter keydown. C50/C55 DONE (txnfilter.go:324-336 + en.go:1138) SKIP.
1. [ ] **F6-amount-filter (C53, pure)** — txnfilter.go:63-82 add AmountMin/Max (match txn amount type — int64 minor NOT float); ApplyWithLabels (233-278) abs-range predicate; FieldAmountMin/Max consts + Without/ActiveFilters; +tests.
2. [ ] **F6-tag-filter-wire (C49)** — transactions.go: MultiCriteria UseState; filtersBody (631-649) tag multi-select (unique tags from unfiltered); chipLabel (562-583) FieldTags; removeFilter (130-132) MultiCriteria.Without; pipeline MultiCriteria.Filter(shown). (logic exists, wire)
3. [ ] **F6-clear-conditional (C51)** — transactions.go:699 If(ActiveFilters>0 || !multiCriteria.IsEmpty()).
4. [ ] **F6-panel-drawer (C52)** — filtertoolbar.go:93-99 FlipPanel → inline collapsible drawer (max-height, no backdrop, table visible); re-impl Esc-close + focus mgmt; +CSS.
5. [ ] **F6-amount-ui (C53)** — filtersBody 2 number inputs → AmountMin/Max; chipLabel "≥ $X"/"≤ $X".
6. [ ] **F6-inline-tags (C48)** — transactionRowProps (26-50) Tags+OnTagsChange; inline form (154-181) tags chip/CSV (UseState from props.Tags, verify GWC []string stable — Member precedent 169-176); row passes Tags+store save. (R7 auto-tag merge, don't clobber)
7. [ ] **F6-tags-column (C54)** — transactions.go:479-488 `page`→`shown` (one-line).
8. [ ] **F6-badge-a11y (C57)** — filtertoolbar.go:64 drop aria-hidden + sr-only "N filters active" OR trigger aria-label w/ count.
9. [ ] **F6-keyboard (C56)** — transactions.go OnMount keydown Alt+Shift+F (Alt+F=Firefox) → toggle; btn Title + <kbd>.
> Risks: amount type (int64 minor not float — check model); MultiCriteria 2nd state (removeFilter/chipLabel dispatch); FlipPanel→drawer re-impl Esc/focus (verify other consumers); inline []string GWC stability; `shown` O(n) negligible; Alt+F conflict (Alt+Shift+F); R7 auto-tag merge; C50/C55 done.

### ★ FEATURE-REVIEW IMPL — F41 per-member views/allocations (C277-C281, parallel research)
> DELTA beyond MIA+R29: C277 active-member filtering WORKS (transactions.go:82-85, dashboard 82-93) but no visible indicator → MIA-dashboard-networth(8)+MIA-scopebanner(C281) → DEFER. C278 accounts/budgets/goals/allocate DON'T scope (only dashboard/transactions/split call UseActiveMember) → MIA threads scope into reports/dashboard/insights/networth NOT these 4 lists → F41 builds. C279 NO fractional ownership (Account.OwnerID single; NetByOwner ledger.go:237-240 single-owner; split.ByWeights split/weighted.go EXISTS+tested, wired only to settle-up → REUSE). C280 reports.SpendingByMember (reports/members.go) EXISTS+tested but NOT in members.go (only reports_screen.go:491). C281 ScopeBanner = MIA → DEFER.
1. [ ] **F41-fractional-model (C279, pure)** — entities.go: Account.AllocationShares []MemberShare{MemberID,Weight int64} (omitempty, no migration; empty=100% OwnerID).
2. [ ] **F41-networth-split (C279, pure)** — ledger.go:240 NetByOwner: AllocationShares non-empty → distribute via split.ByWeights (REUSE) else OwnerID +tests (empty/60-40/group/archived).
3. [ ] **F41-income-split (C279, pure)** — NEW allocate/membersplit.go SplitPeriodIncome (compose PeriodIncome + split.ByWeights) +tests.
4. [ ] **F41-spendingbymember (C280)** — members.go ~238: "Spending this period" card via reports.SpendingByMember(usePeriod range) + name resolve (reports_screen.go:492); empty/rates-unavailable states.
5. [ ] **F41-list-scoping (C278)** — accounts/budgets/goals/allocate: UseActiveMember(); filter OwnerID==active || GroupOwnerID (shared visible — SOFT) || in AllocationShares; muted "Showing [Name]'s X" chip. SWAP→UseActiveScope().Owners when MIA lands.
6. [ ] **F41-shares-ui (C279)** — accountaddform.go + accounts_row.go: collapsible "Split ownership" (member+weight rows from Members()); validate weights>0 && ≥2; save AllocationShares.
7. [ ] **F41-member-delete-shares** — members.go:76-90 reassign-on-delete: extend ownedCount/reassign to scan AllocationShares (ghost-member guard).
> Risks: DON'T dup MIA ReportScope/ScopeBanner (C277/C281 deferred); list-scoping SOFT display-only NOT privacy (shared/GroupOwnerID always visible); fractional = account-balance split only NOT per-txn (scope guard); split.ByWeights REUSE; SpendingByMember rates error (graceful); UseActiveMember→UseActiveScope swap when MIA lands.

### ★ FEATURE-REVIEW IMPL — F43 privacy/trust (C289-C293, parallel research)
> C289 SHIPPED (rail-footer "Private — your data stays on this device" shell.go:703 + /help "Your privacy" card help.go:110-112) — task #369 STALE. C290 no /about|/privacy route + no settings-footer link to /help (settings.go:1008-1012 = version+changelog; /help IS the About surface per R34, unlinked). C291 cloud "what leaves device" undisclosed (backendNote=AI key only; cloudPlanNote/cloudTrustLine partial + gated If(CloudSelected)). C292 keyHintNode (insights.go:133-138) thin; R24/R10 keyexplainer NOT built. C293 About absorbed into /help by R34 (version help.go:68 + What's New) DONE; footer bare. R34 /help SHIPPED; R31 Plans + R24 keyexplainer spec-only.
> ⚠ ACCURACY RISK: cloud.upgradeTrust "end-to-end encrypted" vs cloudTrustLine "encrypted server-side when uploaded" — INCONSISTENT. Verify actual sync crypto (backup.go/server) before privacy copy; one framing, all consistent. (Local store IS AES-GCM per R30; SYNC payload claim must be verified.)
1. [ ] **F43-verify-crypto (PREREQ)** — confirm sync payload (E2E client-encrypted vs server-side); reconcile upgradeTrust vs cloudTrustLine; pick accurate framing.
2. [ ] **F43-footer-link (C290)** — settings.go:1008-1012: "Your privacy →" → nav /help (i18n help.privacyLink) + palette cmd (R34 Help group). NO new /about|/privacy route (R34 owns /help).
3. [ ] **F43-cloud-disclosure (C291)** — settings_section.go after backend toggle BEFORE If(CloudSelected): always-visible "When sync is on, an encrypted snapshot is sent to your server — nothing leaves while off" (settings.cloudSyncDisclosure; wording per #1).
4. [ ] **F43-keyexplainer (C292, SHARED R24/R10)** — NEW internal/ui/keyexplainer/keyexplainer.go KeyExplainer{Purpose,OnSettings}: what-key/cost/platform.openai.com/privacy(question+summary→OpenAI, no raw txns)/storage(session-only); replace keyHintNode (133-138) + documents_image_import.go:75. Build ONCE (= R24-step7 + R10-step7).
5. [ ] **F43-cloudtrust-ungate (C292)** — settings_section.go:232-251 move cloudTrustLine outside If(CloudSelected).
6. [ ] **F43-help-privacy-accuracy (C291)** — help.go:110-112 "never uploaded or shared" TOO ABSOLUTE; add exceptions (cloud-sync snapshot; AI question+summary→OpenAI). Fix BEFORE #2 amplifies it.
7. [ ] **F43-close-c289** — mark #369/C289 done.
> Risks: ACCURACY (#1 — wrong claim = trust liability); ONE surface (no 2nd route); keyexplainer OWNED here (R24/R10 consume — don't triple-build); tone = members.singleDeviceNote register; "never uploaded" too absolute (fix #6 first).

### ★ FEATURE-REVIEW IMPL — F47 offline/PWA perf (C312-C314, parallel research)
> C312 + C313 ALREADY FIXED by parallel agent (VERIFIED): sw.js (cashflux-v271) CORE precache includes ./bin/main.wasm (line 7, install c.add 19) → C312; skipWaiting() (21) + clients.claim() (29) → C313. C311 offline-blank FIXED. tasks #394/#395 STALE. GENUINE GAP = C314 ONLY: wasm UNCOMPRESSED ~60MB; e2e/serve.go (26-28) Content-Type only no Content-Encoding; deploy-pages.yml (42-43) no compress step; no .wasm.gz/.br. gzip ~4-5× (→13-15MB), brotli ~11-12MB.
1. [ ] **F47-verify-pages (read-only)** — curl -H 'Accept-Encoding: gzip,br' -I Pages wasm URL; check Fastly auto-gzip of application/wasm.
2. [ ] **F47-build-compress (C314)** — deploy-pages.yml after go build: gzip -k -9 web/bin/main.wasm (+ apt brotli && brotli -k -q9) → artifact.
3. [ ] **F47-serve-negotiate (C314)** — e2e/serve.go .wasm branch: Accept-Encoding negotiate (br→.br, gzip→.gz, else raw) + Content-Encoding + Vary. ~20 lines (only source change).
4. [ ] **F47-local-build-doc** — CLAUDE.md/build script: gzip step after local go build for e2e.
5. [ ] **F47-cache-version-guard** — sw.js CACHE manually bumped; consider build-hash-derived per deploy (low-urgency; activate already evicts).
> Risks: DON'T touch sw.js (C312/C313 correct — compressed response caches transparently); cache invalidation (URL no hash — MUST bump CACHE per deploy or stale wasm forever); 60MB precache may timeout (gzip→13MB safer); Pages can't set custom Content-Encoding (verify Fastly #1); brotli -q9 ~30-90s CI; Cache API stores DECOMPRESSED bytes.

### ★ FEATURE-REVIEW IMPL — F49 sync reliability (C320-C324, parallel research)
> C320 false "Synced": loadSyncStatus (sync_client.go:401-415) defaults state="synced" when key absent+queue empty, NO BackendActive() check; SyncChip (syncchip.go:44-45) reads directly → green "Synced" w/o backend. C321 chip button (syncchip.go:72-84) NO data-testid. C324 NOT reactive: chip reads plain loadSyncStatus() (44), no atom; setSyncStatus (392-399)→localStorage only (cf online.go/notifyfeed.go captured-atom). C322 no backoff: startBackendWatch (185-215) fixed Sleep 10s(196)/3s(212). C323 no offline→status: wireOnlineStatus (onlinestatus.go:17-29) sets app:online only; startBackendSync (78-81) online-flush but no offline handler. R32 OVERLAP: "conflict" state ALREADY in syncChipFace (syncchip.go:28-30) set by flushBackendSyncQueue (170-177); R32-sync-pwa C309 touches flushBackendSyncQueue/upsert = DIFFERENT funcs than F49's startBackendWatch+setSyncStatus.
1. [ ] **F49-status-atom (C324, FOUNDATION)** — NEW uistate/syncstatus.go (mirror online.go): UseSyncStatus/CaptureSyncStatus/SetSyncStatus(exported, replaces app setSyncStatus); push atom + persist. Promote syncStatus struct to uistate (or string-atom). R32 C309 calls SetSyncStatus after this — sequence first.
2. [ ] **F49-chip-wire (C320/C321/C324)** — syncchip.go: pr.BackendActive() early-return Fragment() when no backend (C320 — local-first = no chip); st via UseSyncStatus+CaptureSyncStatus (C324); data-testid="sync-chip" (C321).
3. [ ] **F49-loadstatus-gate (C320)** — sync_client.go:401: !BackendActive()→syncStatus{}; setSyncStatus (392) no-op when !BackendActive (no phantom).
4. [ ] **F49-offline-handler (C323)** — sync_client.go startBackendSync ~78-81: "offline" listener → SetSyncStatus({State:"offline",Pending:len(queue)}); online flush updates on completion.
5. [ ] **F49-backoff (C322)** — sync_client.go startBackendWatch 185-215: watchBackoff(attempt) (base 2s ×2 cap 60s ±20% jitter) replacing Sleep 10s(196), reset on successful read; keep 3s normal-reconnect (212) + pure test.
> Risks: COMPOSE w/ R32 conflict (same chip+status — enum idle/local-only/syncing/synced/error/conflict; F49 atom seam #1 first so R32 C309 calls SetSyncStatus); struct visibility (export to uistate, mechanical); "no chip" not "Local only" (default ≠ error); captured-atom pre-render race (localStorage write durable, cf online.go); backoff cap+jitter; both touch sync_client.go (different funcs, mechanical merge).

<!-- END-REVIEW-FINDINGS -->

---

## B. Bug fixes (active, high priority) ★

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
- [~] Verify: multi-cell tiles never overlap + resize re-packs — **done** (Pack model + render verified
      in-browser); smooth FLIP animations — **done** (above). Still open: a live drag-over preview (reflow
      lands on drop) and pointer-events over HTML5 DnD for touch (the deferred top item).

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
- [~] Verify: one settings entry point, debug log in the panel, nothing regresses (full `go test ./...`
      + wasm green; browser spot-check pending).

### B5. Collapsed rail should reveal labels on hover ★

**Symptom / want:** the left menu should collapse to icons-only, and hovering an icon should show a
text label ("text highlight") for quick reference.
**Current state:** the rail already collapses to a 58px icon-only mode (`.collapsed`, shared
`rail:collapsed` atom; `internal/app/shell.go`), which hides each item's label `Span`. What's missing
is the hover affordance — collapsed, there's no quick way to see what an icon is.
**Fix:**
- [~] Verify: hover/focus reveals the label without expanding the rail (wasm green; browser spot-check pending).

### B6. Add a UI / font-size scale setting ★

**Want:** fonts and buttons feel ~30% too large for some users (e.g. on `/accounts`), though others
find them fine — add a setting to scale the whole interface up or down.
**Approach (analysis):** the design is px-heavy (Tailwind arbitrary px like `text-[13px]`), so a
rem-based root-font scale would NOT resize buttons/spacing. Use a **whole-UI zoom**: a `--ui-scale`
CSS variable applied via `zoom` on `#app` (Chromium target; `zoom` reflows and scales fonts + buttons
+ spacing together).
- [~] Verify: changing scale resizes the whole UI (wasm build green; browser spot-check pending); 100% == current.

### B7. Menu is missing main-line features ★

**Symptom:** the sidebar lists fewer items than the app implements. Primary nav has Dashboard /
Accounts / Transactions / Budgets / Goals / To-do; System has Members / Categories / Settings. But
`screens.All()` also routes five Phase-2 screens that are **not in the rail** — reachable only by
typing the URL: **Planning** (`/planning`), **Allocate** (`/allocate`), **Insights** (`/insights`),
**Documents** (`/documents`), **Customize** (`/customize`).
**Fix:**
- [~] Verify: every routed main-line screen now has a menu entry (wasm build green; browser spot-check
      pending). Module toggles cover them via the hidden-path filter.

### B8. Sidebar menu management: reorder, drop "My pages", visibility settings ★

Three related sidebar changes (relates to B5 collapsed-hover, B7 missing items):
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
- [~] Verify: trail correct per screen; clicking returns home (wasm green; browser spot-check pending).

### B11. "+ Add" opens a flip-panel of add actions ★

**Want:** the top-bar "+ Add" button should open a centered flip panel (the same lift-to-center +
`rotateY` animation as settings) offering the kinds of things you can add — new transaction, bills to
scan, docs to scan, custom workflows, etc. — instead of jumping straight to `/transactions`.
**Context / reuse:** the flip animation + centered panel already exist as `ui.FlipPanel`, driven by
the `uistate.UseSettings()` atom and rendered by `app.SettingsHost` (kinds: "global" / "widget"). The
cleanest path is to **reuse that mechanism** rather than build a parallel overlay.
**Fix:**
- [~] Back face: instead of a menu of cards, it goes straight to the **New transaction** flow inline
      (account / expense-income / amount / description / category / date → `PutTransaction`, toast).
      Still TODO if a menu is wanted: **Scan a bill** / **Scan a document** (Documents import) /
      **Custom workflow** cards.
- [~] Keyboard-accessible, labelled, light/dark — inherits FlipPanel's chrome and the focus-visible
      rings; a `role="dialog"`/`aria-modal`/focus-trap pass is tracked under the dialogs a11y item.
- _Decision to confirm:_ what "custom workflows" means here — map to the existing Customize screen
  (custom fields + formula builder), or a new "workflow" concept? Need scope before building that card.

### B15. App-wide accessibility — spike + program ★

**Goal:** make CashFlux usable with a keyboard and a screen reader, at high zoom, and without relying
on color — to WCAG 2.1 AA as the bar. This is large and cross-cutting, so it starts as a **spike**
(time-boxed audit → prioritized plan) before the implementation tasks it spawns. Supersedes the
one-line a11y item in §1.20.

**B15.0 — Spike (do first):**
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
- [~] **Custom controls → correct ARIA:** Segmented = `role="radiogroup"`/`role="radio"`/`aria-checked`;
      Toggle/ToggleRow = `role="switch"` + `aria-checked` + name; StepperPill ‹/› have `aria-label`s;
      SwatchPicker = labelled `role="radiogroup"` of `role="radio"` chips. The gear (`aria-label="Widget
      settings"`), accounts "⋯" overflow (`aria-label`), and the grip (`aria-hidden`, decorative) now have
      correct names; the AddMenu/menu/+Add carry text or titles. Still TODO: real keyboard operability for
      the div-based Toggle/Swatch (they have Space/Enter via OnKeyDown; verify with a screen reader).
- [~] **Touch targets:** small icon-only buttons (delete/toast-x/rstep/set-close) now meet the WCAG
      2.5.8 AA 24×24 minimum (centered glyph). 44×44 (AAA) left aspirational given the dense desktop UI.
      - **✅ clr-toggle fixed (2026-06-24).** The per-row cleared-status toggle (`.txn-table .clr-toggle`,
        the ○/✓ on every transaction) was MISSED by the pass above: it measured **26×17px on desktop**
        (the 44×44 sizing was scoped to `@media (max-width:640px)` only, so mouse users got the 17px box).
        Added a base `display:inline-flex; align-items:center; justify-content:center; min-height:1.5rem`
        so desktop now gets a **26×24** hit area (glyph unchanged, the 55px row absorbs it; the 640px touch
        rule still wins → 44×44 on mobile). MEASURED (`e2e/clrtoggle_targetsize_verify.mjs`, 3/3): desktop
        min-dim 24, mobile min-dim 44, desktop row height unchanged at 55px. Screenshot
        `e2e/screenshots/clrtoggle_desktop.png`. (Remaining sub-24px hits are inline text drill-links
        `.row-desc`/breadcrumb — covered by the 2.5.8 inline-text exception — and the intentionally-thin
        widget resize handles `.rz`.)
### B22. Bills & due-date tracker + calendar — SPEC (from C38, 2026-06-18)
**Want:** a real bills surface beyond the dashboard "upcoming bills" widget — a list with due dates,
amounts, paid/unpaid status, and a **month calendar** view.
- [~] **Pure `internal/bills`** (no `syscall/js`, tested): derive bills from liability accounts'
      due-day/min-payment **and** Planning recurring items; compute next-due, overdue, days-until,
      paid-this-cycle; month-grid layout helper (which bills fall on which day). Reuse `dateutil`,
      `freshness`, `domain.Recurring`.
      Liability bills, Planning recurring outflows, next-due/days-until, and month-grid dots are now tested
      and wired into Bills/dashboard/notifications. Remaining: paid-this-cycle derivation.
- [~] **UI:** Bills screen — upcoming/overdue list + a **month calendar** with bill dots; "mark paid" →
      logs the payment; ties **B19** (bill-due reminders) + the dashboard widget.
      Bills screen, calendar dots, reminder-to-task, dashboard, CSV, and bill-due notifications are live.
      Remaining: mark-paid creates/links a transaction.
### B24. Split / shared expenses & settle-up between members — SPEC (from C38, 2026-06-18)
**Want:** split a transaction across members ("50/50") and track **who owes whom** with a settle-up view.
- [~] **UI:** "Split…" on a transaction (equal / % / custom); a **Settle up** view of net balances +
      "record a settlement" (creates a transfer).
      Standalone Split calculator now supports even and weighted splits, shows who owes whom, and exports the
      settle-up plan as CSV. Remaining: transaction-row entry point and persisted settlement transfer.
### B26. Budget rollover / sinking funds — SPEC (from C38, 2026-06-18)
**Want:** envelope **rollover** (unspent carries over) + **sinking funds** (save toward periodic large
expenses).
- [~] **State/UI:** per-budget rollover toggle; "carried over $X"; a sinking-fund type. Ties the
      methodology selector (envelope/zero-based, D6).
      Per-budget rollover now persists on `Budget.Rollover`, has add/edit checkboxes, and shows previous-period
      carried amount in the Budgets list. Remaining: dedicated sinking-fund type/UI.
## C. Live UI/UX review findings — 2026-06-16 (sample data) ★

✅ FIX (2026-06-24) — account-row `⋯` overflow menu had no keyboard/outside-click dismissal + missing
`aria-expanded`. The hand-rolled `add-wrap`/`add-menu` menu on each account row (and the unused `OverflowMenu`
primitive) had the same gaps the +Add menu had: no Escape-to-close, no `aria-expanded` on the trigger, and it
relied on `.add-backdrop` for outside-clicks (which doesn't paint over page content — stacking). Extracted a
reusable **`uiw.DismissPopover(isOpen, wrapID, onClose)`** custom-hook in `internal/ui/dismiss.go` (Escape →
close + refocus trigger; document `pointerdown` outside the wrapper → close; stacking-immune), wired it into
the `OverflowMenu` primitive AND `accounts_row.go`, and added `aria-expanded` (via the existing `ariaBool`).
**Non-obvious bug found by verifying on the rendered menu:** `UseId()` returns ids containing colons (e.g.
`gwc:3:1`), which are invalid in a `#id` CSS selector — `querySelector("#"+id)` threw a SyntaxError and
panicked the wasm callback, silently breaking BOTH dismissal paths. Switched to `getElementById(id)` (never
throws). (The +Add menu was unaffected — it keys off the `.add-btn` class, not an id.) MEASURED on the live
`/accounts` page (`e2e/accounts_menu_verify.mjs`, 8/8): aria-expanded toggles false↔true, menu opens & stays
open, outside-click over content (a `SPAN`) closes, Escape closes + returns focus to the ⋯ trigger, menu item
still closes the menu. Build rc=0; `go test ./internal/ui` ok. (Note: Playwright stability waits stall on
interaction because WONDER hover transitions keep elements "unstable" — verified with raw mouse + querySelector
snapshots. **Correction (2026-06-24): this is NOT a re-render bug** — a MutationObserver shows **0 idle DOM
mutations** on /accounts, /, /transactions, /budgets over 3s with the pointer parked, so there's no runaway
re-render; the stall is purely animation-induced. Also verified: cold deep-link to 8 routes shows no dashboard
flicker.)

✅ ENHANCE (2026-06-24) — `ui.DismissPopover` now also does WAI-ARIA arrow-key roving focus. With Escape +
outside-click + aria-expanded already in place, the menus lacked keyboard item navigation. Added
ArrowDown/ArrowUp (cycle, with wraparound) + Home/End to the shared helper's keydown handler, gated on focus
being inside the popover so global arrow keys are never hijacked while a menu is merely open-but-unfocused.
Benefits every DismissPopover consumer (accounts ⋯, custom-page ⋯, the OverflowMenu primitive). MEASURED on
the accounts menu (`e2e/menu_arrowkeys_verify.mjs`, 8/8): ArrowDown from trigger → first item; Down/Up move &
wrap; End→last, Home→first; Escape still closes + refocuses (no regression). Prior dismissal guards re-run
green (accounts 8/8, custom-page 7/7). Build rc=0; `go test ./internal/ui` ok.

✅ ENHANCE (2026-06-24) — migrated the `+ Add` topbar menu onto `ui.DismissPopover` too. It was the last
dropdown still running its own ~50-line inline Escape/outside-click `UseEffect`, so it had no arrow-key nav and
duplicated the helper. Gave its `.add-wrap` a `UseId` and replaced the inline effect with one
`ui.DismissPopover(open, menuID, closeMenu)` call (keeping `addMenuShouldOpenLeft` for open-direction). Now
EVERY app dropdown (+Add, accounts ⋯, custom-page ⋯, OverflowMenu primitive) shares one helper with the full
menu-button pattern: aria-expanded, Escape+refocus, outside-click, Arrow/Home/End. MEASURED: +Add arrow-keys
(`e2e/addmenu_arrowkeys.mjs`, 6/6 — 9 items, Down/Up/Home/End + wrap, Escape+refocus); existing +Add guards
unchanged (widths 6/6, escape 5/5, outside-click 4/4). Build rc=0; `go test ./internal/app ./internal/ui` ok.
Net −~50 LOC from `addmenu.go`.
✅ FIX (2026-06-24) — `+ Add` menu opened OVER the left rail (items half-unclickable). The single "Add
something new" button sits at the top-left of the content area (x≈264), only ~24px right of the rail edge
(x=240). `.add-menu` used `position:absolute; right:0`, so its 210px panel extended **leftward** to x≈84 —
back over the sidebar. Measured consequence: the menu items' clickable centres (x≈189) fell inside the rail,
which intercepted the pointer (a real Playwright "rail subtree intercepts pointer events" failure when
clicking "New transaction"). Fix: open the menu **rightward** (`right:0` → `left:0`, `web/index.html`), so it
flows into the content column. MEASURED (`e2e/addmenu_verify.mjs`, 8/8, both themes): items now at minLeft=269
≥ railRight=240, menu fits viewport (maxRight=469), "New transaction" is clickable (no interception) **and
opens the add modal**. Pure-CSS; build rc=0. Screenshots `e2e/screenshots/addmenu_fixed_{dark,light}.png`.

⮑ FOLLOW-UP FIX (2026-06-24, same day) — the `left:0` above was verified only at 1280 and **regressed narrow
widths**. Measuring the button across 8 viewports showed it REFLOWS between the left of the content area
(gapLeft≈24-104px, near the rail) and the right edge (gapRight≈24-32px) with NO clean width→side mapping
(left at 1280/1200/1025/900; right at 1100/768/500). So a fixed `left:0` overflowed the viewport when the
button was on the right, while `right:0` overlapped the rail when on the left — and a breakpoint can't capture
it. Made it **decide direction live at open-time**: `internal/app/addmenu.go` measures the button's gap to the
right edge via `syscall/js` (`addMenuShouldOpenLeft`) and adds `.open-left` (→ `right:0`) when there's < ~224px
of room on the right; otherwise opens rightward (`left:0`, default). MEASURED (`e2e/addmenu_widths_verify.mjs`,
6/6 at 1280/1100/1025/1024/768/390): no viewport overflow AND no rail overlap at any width; click+modal still
8/8 (`addmenu_verify.mjs`, both themes). Build rc=0; `go test ./internal/app` ok.

✅ FIX (2026-06-24) — `+ Add` menu didn't close on ESCAPE (keyboard-a11y gap). Measured: opening the popover
then pressing Escape left `aria-expanded="true"` and the backdrop still active (it only closed via item-click
or a backdrop click). Per the WAI-ARIA menu-button pattern, Escape should dismiss it and return focus to the
trigger. Added a document `keydown` listener in `internal/app/addmenu.go` (registered only while open, torn
down on close/unmount — mirrors `dialoghost.go`) that closes the menu on Escape and refocuses `.add-btn`.
MEASURED (`e2e/escape_addmenu_verify.mjs`, 5/5): Escape closes (aria→false, menu hidden) + **focus returns to
the +Add button**; menu still reopens & positions correctly (no regression to the open-direction logic) and
still closes on backdrop click. Build rc=0; `go test ./internal/app` ok.
Captured by driving the running app (`http://127.0.0.1:8080`) in a real headless Chromium via the
now-installed Playwright driver and screenshotting all 14 routes (Dashboard, Accounts, Transactions,
Budgets, Goals, To-do, Planning, Allocate, Insights, Documents, Customize, Members, Categories,
Rules). Screenshots + rendered text are in `.review-screenshots/` (git-ignore this). Items are
ordered correctness-first, then cross-cutting chrome, then per-screen polish.

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
- [~] wasm wiring + UI: agent surface w/ step transcript + approval prompts; capability gating +
      plan-only fallback. Playwright story.
      _(2026-06-20: Insights screen rebuilt as a **chat interface** — conversation thread, Markdown assistant
      bubbles with per-message Save-as-task/Pin + cost, starter chips, composer; sends the whole history each
      turn. MVP uses the flat-prompt chat-completions path. STILL OPEN: bind `internal/agent` loop +
      `internal/aitools` gated read-tools via an `agent.Model` adapter + appstate `DataSource` (tool transcript,
      affordability, richer Q&A), token streaming, approval prompts for future write tools, and the Playwright
      story.)_
**Sequencing:** lands **after C81 Phase 1–3** (needs provider/dialect abstraction) and is much safer
**after C78** (undo). _Cross-links: **C81** (providers/dialects/caps), **C78** (undo = agent seatbelt),
**C76** (AI modal/approval surface), **C75** (notifications), `internal/workflow` (agent can author
workflows/rules), `internal/formula` (sandboxed compute tool)._

### C20. Collapsible side panel reads as "missing" — toggle is misplaced and collapse is broken ★
**Reported:** no collapsible left panel and no toggle button. **Reality (verified):** a menu-toggle
button *does* exist (28×28, with the `icon.Menu` glyph) and clicking it collapses the rail — but:
- [~] Verify: collapse → usable icon rail (C15 ✓) and persists (✓). An on-panel toggle is the open part.

### C22. Layout engine does not reflow on move or on resize ★ (= B2 / C14, with fresh evidence)
**Reported:** moving tiles doesn't reflow; scaling tiles up/down doesn't reflow. **Verified live:**
dragging `kpi-income` onto `kpi-liabilities` changed only those two tiles' `grid-area` (income→`2/4`,
liabilities→`2/3`) — **no other tile moved**, and the result even mis-placed a tile (not a clean swap).
Resize overlaps neighbors (C14). Root cause: absolute placement + pairwise `Swap`/`Resize`, no packing.
- [~] Verify: move/resize reflow is structural (✓, via Pack + the unit tests + the pixel-identical render
      check). The only open piece is the live drag-over **preview** (reflow currently lands on drop, not
      during the drag) — tracked as the remaining B2 UI-polish item, not a correctness gap.

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
- [~] Re-check at the new density: verified live on the dashboard + the quick-add form — body 14.5px,
      fields 34px with no text clipping, KPI figures still fit (0 clipped). The other screens are
      route-gated in the static oracle but use the same shared tokens, so the effect is uniform; nothing
      reduced below the existing **24px** B15 touch-target minimum (fields 34px, buttons ~30px).
### C26. Make text size configurable for low-vision users ★ (accessibility)
**Reported:** font size should be configurable for visually impaired folks. **Current state:** B6 added
a **Display scale** (70–130%) implemented as a whole-UI **`zoom`** on `#app`. That helps but isn't a
true text-resize control: it tops out at 130%, scales layout (not just text), and `zoom` can break the
non-responsive layout (**C10**) at large values.
- [~] Verify at 200%: confirmed no horizontal scroll / reflow on the dashboard (root). The other 13
      screens are route-gated in the static oracle, but they share the same responsive rules + zoom
      mechanism, so the reflow behavior is uniform.

## L. Loop user-story QA — story-driven gaps ★

Findings from the recurring user-story QA loop: invent a real household's flow, drive the app
end-to-end, screenshot it, and log mechanical + UI/UX gaps the dev agent should build/fix
bottom-up (model → tested logic → store → state → UI). Each story below names the persona and the
exact ritual, then the gaps that block it. Screenshots live in `e2e/loop*-*.png`; the driving
script is `e2e/loopstory_NN_*.mjs` (run via `node e2e/run-stories.mjs` or standalone against :8099).

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
**Dream-big gaps (extend a solid engine):**
- [~] **Richer match conditions** — substantially covered by the existing **workflow** engine
      (`internal/workflow` + `/workflows`): expression conditions like `txn_abs > 200` (amount range) and
      `contains(txn_payee, "coffee")` (keyword), tested. The pure `rules.Condition` type (AllKeywords/
      AnyKeywords/AccountID/Min-MaxAmount, tested) also exists but isn't yet wired into the simple `Rule`
      struct/form. (Remaining: surface `Condition` in the simple rules form, OR converge rules→workflows.)
- [~] **Actions beyond category + tags** — covered by the workflow engine's actions
      (`ActionSetCategory`, `ActionAddTag`, `ActionFlagReview`; seeded in sample.go). (Remaining:
      member/owner + budget actions; converge with the simple Rule.)
**Probe note:** the auto-categorize check **false-negatived** first (the script read the *account* select
"Auto Loan", not the *category* select); a focused re-measure confirmed the category = "Dining". Fix
`loopstory_15` to read the category select by position/label, then promote per the gate above.

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
**Probe note:** the first run's "achieved state" + "100% cap" checks **false-negatived** — the inline
**Contribute opens an amount form (not a JS `prompt`)**, so the `page.on("dialog")` handler never fired
and the goal stayed at $0 (and my row filter clicked the wrong goal's Contribute). A corrected re-drive
(create goal already over target) confirmed the **"Complete 🎉"** state + capped bar exist. Fix
`loopstory_20` to fill the inline contribute form for the *named* goal.

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
**Bonus note for the dev agent (cross-cuts L1/L3/L17/L18):** `Transaction.Splits []CategorySplit`
**already exists** in the model (`entities.go:77`). The category-split *data model* is in place — the
budgets-cover (L1), receipt-as-split (L3), allocate-apply (L17), and custom-field reporting work mostly
need **UI + apply logic over the existing Splits**, not a new schema. Verify the splits UI/round-trip.

**Probe note:** the first run's transfer was **vacuous** — my account picker selected the placeholder
"— To account —" as the destination, so submit failed validation and *no* transfer occurred (making the
invariant PASSes meaningless). The `_xfer` re-drive with a real Checking→Savings destination is the source
of truth. Fix `loopstory_24` to pick accounts by name and skip empty-value options.

# App running on http://127.0.0.1:8080 (gwc dev)
cd C:\Users\mreca\Desktop\CashFlux
$env:E2E_URL="http://127.0.0.1:8080"
node e2e/loopstory_76_following_the_thread.mjs
  → 7 PASS · 0 FAIL · 10 ABSENT    EXIT 0
```

**Screenshots produced (23):**
`L76_hop1_transactions.png` · `L76_hop1b_transactions_pivots.png` ·
`L76_hop2_accounts.png` · `L76_hop2b_accounts_ledger.png` · `L76_hop2c_accounts_pivots.png` ·
`L76_hop3_categories.png` · `L76_hop3b_categories_txn_drill.png` · `L76_hop3c_categories_pivots.png` ·
`L76_hop4_budgets.png` · `L76_hop4b_budgets_txn_drill.png` · `L76_hop4c_budgets_pivots.png` ·
`L76_hop5_rules.png` · `L76_hop5b_rules_pivots.png` ·
`L76_hop6_goals.png` · `L76_hop6b_goals_pivots.png` ·
`L76_hop7_bills.png` · `L76_hop7_planning.png` · `L76_hop7_dashboard.png` ·
`L76_final.png`
(plus `L76_hop1b_transactions_links.png` · `L76_hop5b_rules_txn_link.png` · `L76_hop7b_planning.png` · `L76_hop7c_dashboard.png` from first-pass run)

**Re-test status**
- **L74/L75 GAP-E** (drill-filter broken): **CLOSED** — all three drills (account/category/budget → /transactions) correctly filter the result set. The L75 probe was looking for `a[href]` not `button`; the filter itself was never broken.
- **L74 GAP-G** (/goals linked-account): **STILL OPEN** — "· linked to X" button navigates to /transactions, not /accounts. The goal→account specific pivot is absent; the button is a transaction drill with a confusing label.
- **L74 GAP-F** (bills widget → /accounts instead of /bills): not re-tested in L76 (out of scope).
- **L75 SA-10** (/reports → /transactions link): not re-tested in L76 (reports not in scope).

---

### L79. Story — "The Money Move" (Renu) — 2026-06-24 ★

**The ritual.** Renu does the most common household money action: move money between her own
accounts (Everyday Checking → Emergency Savings). Theme = **transfer integrity + cross-screen
consistency**. Drive script `e2e/loopstory_79_money_move.mjs` (run `E2E_URL=http://127.0.0.1:8099
node e2e/loopstory_79_money_move.mjs`). Result **7 PASS · 0 FAIL · 2 ABSENT** (the 2 ABSENT are the
finding below). Screenshots `L79_01..05`.

**🔴 CRITICAL (found + FIXED this pass) — the app did not boot at all.** Before any ritual could
run, both `serve.go` (:8099) and `gwc dev` (:8080) rendered a blank page: the wasm panicked on
startup with `panic: GoUseAtom called outside component context`. Root cause: the admin-console
gating added `uistate.UseAdminConsoleAvailable()` (a framework **hook**) into `app.navGroup`
(`shell.go:246`); but `navGroup` is also called at **boot** by `wireKeyboardShortcuts()`
(`shortcuts.go:32` ← `app.Run` ← `main.main`) to enumerate primary-nav paths for digit shortcuts.
Hooks may only run during a component render, so the boot-time call aborted `main()` and the whole
app failed to start. **Fix:** added a hook-free `primaryNavStatic()` in `shell.go` (enumerates the
primary group straight from `screens.All()`, excludes `AdminOnly`, calls no hook) and pointed
`wireKeyboardShortcuts` at it. Build rc=0 (PARENT-VERIFIED); `go test ./internal/app` ok. MEASURED:
the app now boots — `navLinks=27`, `#app` body 79 KB, **zero pageerrors/console errors** (was
`navLinks=0`, panic). This is a committed-HEAD regression, not local churn (git diff of shell.go/
shortcuts.go was empty). **A boot smoke test belongs in CI** — nothing caught "the app won't start".

**⚠ FINDING (dev ticket) — a transfer gives NO visible balance feedback on /accounts.** The transfer
itself is *correct*: the form submits, a labelled "Transfer" ledger entry is created (the $250/$500
legs show up in /transactions), and **net worth is conserved** across 1 + 3 stress transfers
(`$60,386.00` throughout — the double-entry integrity invariant holds, T-2/T-3/T-5 all PASS, zero JS
errors). BUT on /accounts the **displayed balances do not move**: source `Everyday Checking` stayed
`$6,473.50` and destination `Emergency Savings (HYSA)` stayed `$12,200.00` after a $500 transfer
(measured delta = 0 for both; T-1a/T-1b ABSENT). Cause: the row shows the **cleared** balance
("… cleared $6,473.50") and a newly-created transfer leg is **uncleared**, so the cleared figure is
unchanged. From the user's POV: *"I just moved $500 between my accounts and both balances look exactly
the same"* — confusing, looks broken, and undermines trust on the single most common household action.
- [ ] **L79-T1 — give transfers immediate, visible balance feedback on /accounts.** Options (pick per
  spec): (a) treat an own-account transfer as **cleared on creation** (the money has demonstrably
  moved between the user's own accounts — there's no pending settlement), so cleared balances update
  at once; and/or (b) show the **current/available balance** as the primary figure with cleared as a
  secondary line; and/or (c) surface a success toast ("Moved $500 → Emergency Savings · new balance
  $11,700") so the action is acknowledged even if the headline figure is the cleared one. Today there
  is no toast and no balance change — the only confirmation is hunting for the leg in /transactions.
  e2e: extend `loopstory_79` to assert source−amount / destination+amount on /accounts after a transfer.
- Note (not a bug): the transfer destination picker correctly offers liabilities (credit card, student
  loan) and investment accounts as targets — paying down a card via "Transfer" is a reasonable flow.

**What works well (regression anchors)** ✓
- ✓ Net-worth conservation across transfers (double-entry integrity) — exact to the cent over 4 transfers.
- ✓ Transfer creates a labelled "Transfer" ledger entry; not silently merged or mis-typed as spend/income.
- ✓ Reports spending total unaffected by transfers (transfers aren't counted as expenses).
- ✓ Zero JS errors / no crash through a 4-transfer mini-stress.

- **✅ Ledger integrity regression guard landed (`e2e/integrity_ledger.mjs`, 2026-06-25, 7/0/0).** A
  permanent e2e for the core finance invariant: a transaction moves the affected account's balance AND
  household net worth by EXACTLY its amount, in the right direction. Adds a $50 **expense** to an
  asset account (Roth IRA) and asserts balance ↓50 + net worth ↓50; then a $50 **income** and asserts
  both ↑50; then asserts the pair round-trips to baseline (no drift); 0 JS errors. Runs with
  `reducedMotion:'reduce'` so the W-15 count-up flourish (countup.js tweens `.fig` from 0) doesn't
  poison figure reads — without it, post-render reads catch the tween mid-flight at $0 (a test-harness
  trap, NOT a product bug; verified the figure settles to the correct value). Also found+fixed a
  selector trap in the test: a bare `^income$` button match hits the dashboard "Income" KPI widget
  header behind the modal, not the modal's `.seg-btn` toggle — scoped to `.seg-btn`. MEASURED:
  expense $11954.04→$11904.04 (Roth $8100→$8050), income back to $11954.04/$8100, exact to the cent.
  The app's most critical invariant now has a regression test.

**Process note — concurrent cloud-sync churn broke the build (again).** This pass also had to repair
a red tree before it could build: `internal/screens/homehero.go` (untracked, a half-applied
"HomeHero" homescreen feature) called `.String()` on `css.Rule` (no such method) and was wired into
`dashboard.go:277`; `internal/screens/widget_builder.go` was re-broken (missing `vbSeriesMax`/
`vbChartColors`, same as the prior two passes). To get a verifiable green build I `git checkout`-ed
`dashboard.go` + `widget_builder.go` to HEAD and renamed `homehero.go` → `homehero.go.churn-disabled`
(preserved, excluded from the build) — no other churn files touched. The HomeHero feature is
incomplete/uncompilable and needs a real finish-or-revert by a dev.
  - **RESOLVED 2026-06-25 (keep-tidy dead-code removal).** A compiling-but-unused `homehero.go` had
    reappeared in the tree (225 lines: `HomeHero`, `homeHeroFull`, `homeHeroEmpty`, `heroStatBlock`,
    `homeHeroFullProps`). It was never wired in — the live dashboard hero is `dashboard_hero.go`
    (`dashboard.go:281` renders `dashboardHero`), and a grep confirmed **0 external references** to any
    homehero symbol (and no test refs). Deleted the whole file to kill the duplicate-hero footgun that
    has broken the build across multiple passes. MEASURED: build rc=0 after deletion; dashboard renders
    the live hero ("Good morning." greeting + net-worth $11,954.04 + stat strip), 0 JS/console errors
    (`e2e/screenshots/dashboard_after_deadcode.png`). (C2's stale `homehero.go` reference now points at
    `dashboard_hero.go`, the live hero — the sample-data persist race itself is unchanged here.)

---

### L80. Story — "Paying the Bills" (Tomas) — 2026-06-24 ★

**The ritual.** Tomas clears this week's bills on payday — "Mark paid" is one of the most common
household actions, so it must be rock-solid. Theme = **bill-payment lifecycle + cross-screen
propagation**. Drive script `e2e/loopstory_80_paying_bills.mjs` (run `E2E_URL=http://127.0.0.1:8099
node e2e/loopstory_80_paying_bills.mjs`). Result **10 PASS · 0 FAIL · 0 ABSENT**. Screenshots
`L80_01_bills_before.png`, `L80_02_after_markpaid.png`.

**What works well (regression anchors)** ✓ — the lifecycle is genuinely solid:
- ✓ **B-1** Mark paid creates **exactly one** payment transaction (`RecordBillPayment` → `PutTransaction`),
  findable in /transactions.
- ✓ **B-2** For a recurring bill, **NextDue advances** after payment (measured 2026-07-01 → 2026-07-03 via
  `r.Advance()`) — it won't re-dun the same due date.
- ✓ **B-4** Clear success confirmation toast ("Logged a payment for Rent." in `.toast-msg`).
- ✓ **B-5** Spending (Reports) reflects the payments ($5,518.00 after the run).
- ✓ **B-6 STRESS** 3 back-to-back payments created **exactly 3** transactions — no double-post, no
  dropped post, no crash.
- ✓ Zero JS errors across the whole ritual.

**⚠ FINDING (dev ticket) — "Mark paid" has no double-charge guard (money-risk).** B-7: double-tapping
"Mark paid" on the same bill records **2 payments** (delta = 2 transactions; measured "Student Loan"
616→618). Each click posts a payment with no debounce, no optimistic "Paid ✓" state, and no confirm —
so an accidental double-tap (easy on touch / a laggy wasm frame) silently double-records a real money
movement, and the bill stays in the list looking unpaid. The lifecycle is otherwise correct; this is
purely a guard gap.
- [ ] **L80-T1 — guard "Mark paid" against accidental double payment.** On click, immediately disable
  the button (and/or swap it to a non-interactive "Paid ✓ — undo" affordance) until the row re-renders
  with the advanced NextDue, so a second tap can't post a duplicate. Optionally a lightweight confirm
  for non-recurring (liability) payments where there's no NextDue to move the row out of range. e2e:
  extend `loopstory_80` to assert a rapid double-tap yields delta = 1 transaction, not 2.
- Note: this mirrors the broader "destructive/irreversible action needs a guard" theme (cf. the bulk-
  delete-confirmation gap L50) — money-posting actions should be at least as protected.

**Probe note.** The transient confirmation renders as `<span class="toast-msg">`, distinct from the
persistent sample-data `.toast` banner — drive scripts must target `.toast-msg` (or filter by text) to
read action confirmations, else they grab the banner (initial L80 run mis-flagged "no toast" before the
selector fix).

---

### L82. Story — "Paying Yourself First" (Aaliyah) — 2026-06-24 ★

**The ritual.** Aaliyah moves money into her savings goals on payday via "Contribute". Theme =
**goal-contribution lifecycle + money-effect honesty**. Drive script
`e2e/loopstory_82_goal_contribution.mjs` (run `E2E_URL=http://127.0.0.1:8099 node
e2e/loopstory_82_goal_contribution.mjs`). Result **8 PASS · 0 FAIL · 1 ABSENT** (the ABSENT is the
HIGH finding below). Screenshots `L82_01..02`.

**What works well (regression anchors)** ✓
- ✓ **G-1** Contributing raises the goal's saved amount exactly (+$200 no-ledger, +$300 ledger; 3×$50
  accumulates to exactly $150 — no drift/double-count).
- ✓ **G-2** A no-ledger contribution creates **no** transaction (goal-only); a "Also debit …"
  contribution creates **exactly one** transaction.
- ✓ **G-4** $0 contribution is rejected (goal unchanged) — L41 holds.
- ✓ Zero JS errors across the ritual.

**🔴 FINDING (dev ticket, HIGH) — a goal contribution is counted as SPENDING (saving lowers your
savings rate).** With "Also debit …" checked, `ContributeToGoal` posts a category-less debit against
the linked account. MEASURED: Reports SPENDING total rose by **exactly the contribution amount**
($8,222.67 → $8,522.67 after a $300 contribution). So moving money into savings inflates "spending"
and — because savings-rate = (income − spending)/income — **literally lowers the user's savings rate
when they save more.** The source comment (`goal_ops.go`) intends the no-`CategoryID` to avoid
distorting per-*budget* rollups (it does), but the *total* spending figure sums all expense-signed
txns regardless of category, so the contribution still counts. Transfers are correctly excluded from
spending (verified L79) — a goal contribution is conceptually a transfer-to-savings and should be
treated the same.
- [ ] **L82-T1 — exclude goal-contribution ledger entries from the spending/savings-rate totals**
  (treat them like transfers): e.g. mark the posted txn as a transfer/`IsTransfer`-equivalent or add a
  "savings" exclusion so it doesn't inflate Reports/Dashboard spending or depress the savings-rate.
  e2e: extend `loopstory_82` to assert Reports spending is UNCHANGED after a ledger contribution.

**⚠ FINDING (dev ticket, LOW) — milestone toast shows a doubled percent ("25%%").** The progress
milestone toasts `goals.milestone25` = `"25%% of the way there — keep going!"` and `goals.milestone75`
= `"75%% funded — almost there!"` (`internal/i18n/en.go`) contain a literal `%%`, but they're rendered
via `uistate.T(key)` with **no `Sprintf`** (goals.go:155), so the `%%` is NOT collapsed and the UI
shows "25%%"/"75%%". (The `%d%%` strings like `goals.progressFmt` are fine — those ARE `Sprintf`'d.)
MEASURED: contribute toast rendered `"25%% of the way there — keep going!"` in `.toast-msg`.
**Semantic note (for design):** the "post ledger" checkbox reads *"Also debit <linked account> (move
money from this account)"*, yet goals display as *"linked to <account>"* (the savings destination).
Debiting the very account the goal tracks is conceptually backwards for a savings-destination link —
worth clarifying whether the linked account is the source or the destination (cross-ref L82-T1).

---

## G. GLAMOR — per-page UX/visual structure review (world-class, enterprise, glanceable) ★

### GD. DESIGN/FLAIR PASS — page-by-page beautification (2026-06-24, ongoing) ★
Goal: lift the visual design/flair (not UX) across every page + modal, per the frontend-design skill
(refined/luxury direction — depth, layering, premium surfaces). CSS-only in `web/index.html` where possible.
- **Audited (no change needed):** empty states are consistent across screens (icon + message + glossy CTA via
  `EmptyStateCTA`); all recent GD flair (GD-6 focus glow, GD-7 bar gloss, GD-10 seg pill, GD-11 calendar
  today) verified rendering correctly in BOTH themes — color-mix is theme-adaptive, no regressions.
- **Audited (no change needed):** alpha-composited contrast scan across ~15 screens in BOTH themes found
  **zero real low-contrast text** — recent fixes (Free badge, logo) hold. The only flags were false
  positives: `.btn-primary` uses a gradient (background-*image*) the probe can't read, so it reported
  white-on-"white" in light; confirmed the buttons render white-on-green correctly (`btn_light_check.png`).
- **GLAMOR audit (no defects this fire):** Appearance page renders at full opacity after settle (the dimmed
  look in a prior screenshot was the page-enter fade caught mid-animation; `#cf-page-view` opacity=1, no
  `.page-enter`). The **Motion control wires correctly** to the WONDER system — MEASURED Off→`data-wonder=off`
  /`--wonder-on:0`, Subtle→`subtle`/`.55`, Full→default/`1` (the user gateway to all WONDER + the chart-anim
  gating works). Accent swatches already have a clear selected state (`.swatch.sel` white ring + aria-checked).
  Confirmed the GD-18 avatar fix was the ONLY color-backed-text instance (dashboard member-chip is a neutral
  dark chip; category swatch is a textless dot; no owner-color badges).
- **Audited (no defect):** the destructive confirm dialog is well-built — backdrop blur, red `.btn-danger`
  Confirm (was already correctly danger-styled, not affirmative green), Cancel focused as the safe default.
- **Investigated (no code change — both candidates correctly no-go):** (a) the L97 CSV "no-dedup" was
  reclassified as INTENTIONAL (test-encoded in `appstate_more_test.go:232` — re-importing the same row is
  expected to import again; a dedup would break it) — corrected the ticket, left `ImportTransactionsCSV`
  untouched (import tests still green). (b) Mobile rail+tabbar both render at 390px (rail 56px + tabbar) — the
  known B31 redundancy needing a phone drawer-rail, dev-sized; not newly fixable.
- [ ] GD-24+ optional polish (lower priority): per-page accent micro-touches.

### G11. Bills — "What's Due This Week" (Tomas) — 2026-06-23 ★

**✅ RESOLVED (2026-06-23).** Most of this page already worked (mark-paid, urgency tones, soonest-due
sort, visible names — all confirmed by the audit). Remaining fixes shipped:
- **Card titles in light mode** (CRITICAL §3) — fixed by the G9 definitive `[data-theme="light"]`
  contrast fix (this was the 8th screen to flag it; now closed series-wide).
- **✅ Calendar weekday headers in light mode (2026-06-24).** The "SUN…SAT" headers (`.cal-head`)
  read `var(--text-faint)`, which the theme engine resolves to a too-pale `#969698` in light
  (measured WCAG **1.6:1** on the white calendar card — fails AA, nearly invisible). Pinned
  `[data-theme="light"] .cal-head { color:#686870 }` (the palette's intended faint tone, CSS-only in
  web/index.html). MEASURED (`e2e/calhead_contrast_verify.mjs` vs serve.go, 2/2): light **5.52:1**
  (AA normal), dark **3.58:1** unchanged (pin is light-scoped — no regression). Screenshot
  `e2e/screenshots/calhead_light.png`. Scoped to `.cal-head` only — deliberately does NOT touch the
  systemic `--text-faint` token, whose too-pale light derivation is separate GX14/Go work.
  - **✅ GX14 RESOLVED at the source (2026-06-25, keep-tidy GLAMOR re-check).** Two root-cause Go fixes
    for light-mode dim/faint text:
    1. **Theme-mode toggle didn't re-apply the engine's inline CSS vars.** `appearance.go`'s `savePrefs`
       called `ApplyPrefs`+`PersistPrefs` but **not** `ApplyTheme`, so toggling to Light only flipped
       `data-theme` while boot's dark `--text-dim:#ababb3` stayed inline on `:root` and beat the
       `[data-theme="light"]` stylesheet — every `var(--text-dim)` consumer rendered ~**2.28:1** on white
       (WCAG-AA fail). Fix: `savePrefs` now also calls `uistate.ApplyTheme(uistate.LoadTheme())`, exactly
       mirroring boot, so the inline vars track the new mode. → `--text-dim` now `#56565c` = **7.29:1**.
    2. **Derived `--text-faint` washed out on light backgrounds.** `theme/derived.go` derived it as
       `mixHex(TextDim, BgBase, 0.40)`, which on a white bg gave `#969698` (~**2.85:1**). Added a
       light-only derivation (`IsLight()` branch, dark unchanged) mixing just `0.15` toward bg →
       `#6e6e73` = **5.07:1**. MEASURED on Planning in true light mode (toggle via Appearance + SPA nav):
       `--text-dim` 7.29, `--text-faint` 5.07, **0 pale (>130) text elements remain** (was the
       `.t-caption`/`<p>` insight copy at 2.28), 0 JS errors; `go test ./internal/theme ./internal/prefs`
       ok; build rc=0; screenshot `e2e/screenshots/planning_light_contrast_fixed.png`. The `.cal-head` /
       "Custom range" light pins above are now redundant safety nets (left in place; harmless).
    3. **`tw.TextFaint` was the last text token NOT following the theme + dark faint was too pale
       (2026-06-25, follow-up sweep).** An app-wide light grey-text sweep (14 screens) found one
       remaining systemic fail: the rail "N members · USD base" line at **4.08:1** on white. Cause:
       `tw.TextFaint` (`internal/ui/tw/tw.go`) hardcoded `cFaint` (#7d7d85) instead of
       `var(--text-faint, …)` — the one text token the "follow-the-theme" migration (TextFg/TextDim)
       missed. Switched it to `var(--text-faint, #7d7d85)`. That alone would regress DARK (engine's dark
       `--text-faint` was `#6c6c71` ≈ **3.66:1** on near-black, fainter than the old hardcode), so also
       bumped the dark derivation `mixHex(TextDim, BgBase, 0.40 → 0.28)` in `theme/derived.go` (light
       branch unchanged at 0.15). MEASURED on the "members" line: **dark #7f7f85 = 4.85:1** (was 3.66 for
       var consumers), **light #6e6e73 = 5.07:1** (was 4.08); app-wide re-sweep → **0 failing grey-text
       elements** on all 14 screens (only white-on-accent button labels remain, a pre-existing brand-button
       concern in both themes, out of scope); `go test ./internal/theme ./internal/prefs` ok; build rc=0;
       0 JS errors. tw.TextFaint now tracks the live theme like TextFg/TextDim — GX14 fully closed.
- **✅ "Custom range" period-bar control contrast (2026-06-24).** The "Custom range" toggle in the
  resolution bar (`shell.go`) used `tw.TextFaint` (#7d7d85 → **1.87:1** on the light page bg — fails
  AA for a clickable control), while its immediate sibling "This period" button already used the
  darker `tw.TextDim`. Changed the one token `TextFaint → TextDim` so the two adjacent secondary
  controls match. Build rc=0; `go test ./internal/app` ok. MEASURED
  (`e2e/customrange_contrast_verify.mjs` vs serve.go, 2/2): light **6.74:1** (was 1.87), dark
  **8.46:1** — both clear AA, no regression. Screenshot `e2e/screenshots/customrange_light.png`.
  (Light-mode contrast sweep is now down to 2 entangled items left: a done/strikethrough To-do row —
  intentional dimming — and the semantic-red negative-change stat — both Go-token concerns, left alone.)
- **✅ Negative/positive amounts legible in light mode (2026-06-24).** The semantic up/down TEXT tokens
  `tw.TextDown`/`tw.TextUp` hardcoded the **dark-mode** hex (`#d8716f`/`#54b884`), so amounts using them
  rendered ~**1.8:1** on a white card — a negative "−$1,718.00 this month" under Net Worth was barely
  readable (a finance app must make negatives obvious). Made both theme-aware:
  `css.Color("var(--down, #d8716f)")` / `var(--up, #54b884)`, mirroring the earlier `TextFg`/`TextDim`
  fix. The theme engine emits readable light values (`--down #b3322f`, `--up #1f8a52`) and the **dark**
  vars equal the literals exactly (`--down #d8716f`, `--up #54b884` — measured), so dark mode is
  byte-identical. `BgDown`/`BgUp` keep the literal hex (intentional fills). Build rc=0;
  `go test ./internal/ui/tw` ok (golden `TextDown` expectation updated to `color:var(--down,#d8716f)`).
  MEASURED (`e2e/negamount_contrast_verify.mjs`, 2/2): light **6.15:1** (was 1.82), dark **5.80:1**
  unchanged. Full light audit now **1** finding total (only the intentional done-task dim remains).
  Screenshot `e2e/screenshots/negamount_light.png`.
  - **✅ FOLLOW-UP: the LITERAL `.text-up`/`.text-down`/`.text-warn` CSS classes (2026-06-25).** The
    above fixed the `tw.TextUp`/`TextDown` *inline* helpers, but `tw.ColorClass("text-up"/"text-down")`
    emits the **marker classes** (used by `dashboard.go` net-worth deltas, `bills_screen.go` urgency,
    etc.), which hit the hardcoded `.text-up{color:#54b884}` / `.text-down{color:#d8716f}` / `.text-warn
    {color:#cfa14e}` rule (web/index.html:1500) with **no light override** — so "Up $396.25" measured
    **2.23:1** on white. Fixes: (a) up/down literals now `var(--up,#54b884)` / `var(--down,#d8716f)` so
    they follow the engine in both modes (dark byte-identical); (b) added `[data-theme="light"] .text-warn
    { color:#8a6a16 }` (the bright `--warn` amber is ~2.2:1 on white — readable amber-brown for TEXT,
    bright amber kept for `bg-warn` fills). MEASURED both themes: LIGHT up **4.04** (was 2.23; = the
    theme's chosen brand green `#1f8a52`, AA-large; consistent with all other green text), down **6.15**
    (was ~2.2), warn **5.06** (was ~2.2); DARK up 7.87 / down 5.98 / warn 7.90 (unchanged). build rc=0;
    0 JS errors; screenshot `e2e/screenshots/dashboard_light_semantic.png`.
- **✅ Sample-data banner "Start fresh"/"Dismiss" legible in DARK mode (2026-06-24).** First DARK-mode
  contrast sweep (`e2e/dark_contrast_audit.mjs`) flagged ONE issue on **every screen**: the sample-data
  banner's CTA `.sample-banner-btn` used `color: var(--accent)` (#2e8b57 green) on the banner's
  `background: var(--accent-dim)` which in dark is **#205337** (dark green) → **1.55:1**, illegible
  green-on-green. Fix (CSS-only, web/index.html): default `.sample-banner-btn` to `var(--text)` (reads
  on the banner in dark, ~8:1) and keep the green CTA in **light** via `[data-theme="light"]
  .sample-banner-btn { color: var(--accent) }` (light was already AA-large and is the friendlier look —
  unchanged). Hover affordance is now a theme-agnostic underline-thickness bump (no contrast loss).
  MEASURED (`e2e/samplebanner_contrast_verify.mjs`, 2/2): dark **8.12:1** (was 1.55), light **3.93:1**
  (unchanged). Dark-mode audit now **0** findings (was 7). Screenshot `e2e/screenshots/samplebanner_dark.png`.
- **✅ DARK contrast re-check round 2 (2026-06-24, keep-tidy, CSS-only) — 3 more AA misses fixed.** A
  fresh WCAG sweep (alpha-compositing probe, gradient-skip) across 7 screens in dark caught three real
  sub-4.5 labels the earlier passes missed:
  1. **Sample-banner "Dismiss" 3.91:1** — the CTA was fixed last round but `.sample-banner-dismiss` kept
     its own `color: var(--text-dim)` override on the dark `--accent-dim` banner. Removed the override so
     Dismiss inherits `.sample-banner-btn` (dark `--text`, light `--accent`). → **8.12:1**.
  2. **`.hero-stat-label` 3.58:1** (Reports/dashboard/home hero eyebrows) — was `var(--text-faint)`
     (#888890); bumped to `var(--text-dim)` (#ababb3). → **8.2:1**.
  3. **`.section-divider` 3.69:1** (uppercase section eyebrows app-wide) — same `--text-faint`→`--text-dim`
     bump (light keeps its #686870 override). → **8.46:1**.
  MEASURED via alpha-compositing probe; all three now ≥4.5 (8.1–8.5:1). build rc=0; sw cache v260→v261.
  **Probe-methodology note (important for future contrast sweeps):** toggling `data-theme` via JS does
  NOT switch the palette — Go applies `theme.CSSVars()` as INLINE STYLE on documentElement (wins over
  the `[data-theme="light"]` stylesheet block at L756), so the LIGHT pass must switch via the Appearance
  "Light" seg-btn (and even that needs the Go re-emit to fire). A JS attribute flip yields a Frankenstein
  dark-tokens+light-element-rules state with false 1.05:1 readings (brand-name, font "Default"). Trust
  only the DARK pass unless the light palette is switched through the app.
  **Still open (deferred — palette-wide, not done here):** positive/income amounts (`.amount-income`,
  `--up` #54b884) and Reports category drill-links (`.row-desc.btn-link`, `--accent`) measure **4.41:1**
  in dark — just under AA 4.5. Fixing means nudging the semantic green/accent brighter, which touches the
  whole palette + light theme; left for a dedicated palette-tuning pass to avoid an aesthetic regression
  in a keep-tidy fire.
- **✅ Goals "Final stretch" pace badge legible in BOTH themes (2026-06-24).** A badge-contrast sweep
  found `.pace-final` used `background: var(--accent-dim); color: var(--accent)` — accent-green text on
  the accent-dim green pill (#205337 dark / #88bb9d light) → **2.1:1 dark / 1.95:1 light**, washed-out
  green-on-green (same family as the G14 rank-badge bug). Fix (CSS-only, web/index.html): give it the
  neutral `background: var(--bg-elev)` of its sibling `.pace-ontrack` while keeping the celebratory
  accent **text** (so "Final stretch" = green text vs "On track" = gray text, both on the same neutral
  pill — readable, distinct, and accent-aware for custom themes). MEASURED
  (`e2e/pacefinal_contrast_verify.mjs`, 2/2): dark **3.83:1** (was 2.1), light **3.76:1** (was 1.95) —
  AA-large, consistent with the tint-based `.pace-overdue`/`.pace-soon` badge family. Screenshot
  `e2e/screenshots/pacefinal_dark.png`. (Other pace/status badges measured ≥3.0 in both themes — clean.)
- **✅ Native form controls themed in dark mode — date-picker icon was invisible (2026-06-24).**
  `color-scheme` was `normal` everywhere, so in dark mode every native control rendered light-themed —
  most visibly the `<input type="date">` calendar indicator was **black on the #202022 field**, nearly
  invisible (transaction/transfer/goal/bill dates, custom range, etc.). Also affected native select
  dropdown chevrons and scrollbars. Fix (CSS-only, web/index.html): `:root { color-scheme: dark }` +
  `[data-theme="light"] { color-scheme: light }` so the browser themes native controls to match. MEASURED
  (`e2e/colorscheme_verify.mjs`, 2/2): date input resolves `color-scheme:dark` in dark / `light` in light;
  the calendar icon is now light/visible on the dark field (screenshot `e2e/screenshots/dateinput2_dark.png`,
  vs the black-icon before). Build rc=0 (CSS-only; no Go). Applies app-wide to all native inputs/scrollbars.
- **✅ Long unbroken names no longer overflow list rows (2026-06-24).** The generic `.row-desc` (account/
  goal/budget list rows) used `white-space: normal` with **no `overflow-wrap`**, so a long *unbroken*
  token (email, URL, ID, or a no-space string) had no break point and overflowed the card — MEASURED a
  166-char no-space goal name pushing the cell to right=1423px (parent ends 1249, viewport 1280;
  `overflowsParent: true`). Fix (CSS-only, web/index.html): `.row-desc { overflow-wrap: anywhere }` so
  long tokens break and wrap inside the card. MEASURED after (`e2e/rowdesc_overflow_verify.mjs`, 2/2):
  list-row cell right 1423→**812** (within parent, `overflowsParent:false`, docOverflow 0); the
  txn-table `.row-desc` truncation (nowrap+ellipsis+max-width:280px, a more specific rule) is
  **unaffected** — no regression. Screenshots `rowdesc_nospace_{before,after}.png`. Harmless for normal
  text; pure robustness against arbitrary user-entered names. (Verified clean this pass — no change
  needed: reduced-motion compliance [0 running anims], long-description truncation in the txn table,
  add-transaction form contrast, and `.attention-text` ellipsis.)
- **✅ On-brand text selection highlight (2026-06-24).** No `::selection` rule existed, so selecting
  text (e.g. copying an amount) used the off-brand OS-default highlight (a blue-grey that clashes with
  the green accent and varies by OS/theme). Added a global `::selection`/`::-moz-selection`:
  `background: color-mix(in srgb, var(--accent) 28%, transparent); color: var(--text)` — an accent-
  tinted highlight that keeps the text color readable, is theme- and custom-accent-aware, and works in
  light + dark. CSS-only (web/index.html); build rc=0 (no Go). MEASURED (`e2e/selection_verify.mjs`,
  4/4): rule present + color-mix resolves to the accent at 0.28 alpha in both themes; visual
  `e2e/screenshots/selection_greeting_dark.png` shows a subtle green highlight with the heading still
  legible. (Light-mode contrast sweep is now complete app-wide — Customize/Documents/Insights also
  verified clean in both themes this pass.)
- **✅ Print / save-to-PDF stylesheet (2026-06-24).** There were **zero `@media print` rules**, so
  printing a statement/report/ledger (a routine finance-app action) output the **dark UI + nav rail +
  topbar + banners + scrollbars** — ink-heavy and often unreadable (near-white text on the printed
  dark surfaces). Added a print stylesheet (CSS-only, web/index.html): forces an ink-friendly light
  palette regardless of the active theme by overriding the theme engine's INLINE `--*` vars with
  `!important` on `<html>` (`--bg/--bg-card/--text/…`) AND forcing the layout containers white
  (`.cf-shell`/`main`/`#cf-page-view`/`.bento` bake a hardcoded dark `tw.BgBase`, not `var(--bg)`, so
  they needed explicit overrides — else gaps between cards printed black); hides app chrome (rail,
  topbar, mobile tabbar, banners, toasts, the period `.reso-control`, hero action buttons); flows the
  scroll container across pages; keeps cards from splitting mid-page (`break-inside: avoid`); `@page
  { margin: 1.5cm }`. Semantic income-green/expense-red and chart/donut colors are intentionally kept
  (readable on white). MEASURED under `page.emulateMedia({media:'print'})` **from the DARK theme**
  (`e2e/print_styles_verify.mjs`, 5/5): body bg white (lum 255), `--text` forced `#111` over the inline
  dark var, nav rail + topbar `display:none`, cards `break-inside:avoid`; containers all
  `rgb(255,255,255)`. Visual `e2e/screenshots/print_reports2.png` — a clean B&W report with white
  surfaces, dark text, preserved semantic/chart colors, full-width content, no chrome. (CSS-only, no Go.)
  - **✅ EXTENDED to the transactions ledger/statement (2026-06-24).** Printing the txn table is a common
    finance-app case. Added print rules so a ledger prints like a statement: `.txn-table tr
    { break-inside: avoid }` (each row stays whole across page breaks — was `auto`), `thead
    { display: table-header-group }` (column headers repeat on every page), and hide the interactive-only
    columns (`.td-actions`, `.td-select` checkboxes) which are noise on paper. MEASURED under
    `emulateMedia({media:'print'})` (`e2e/print_styles_verify.mjs` for the page chrome; a table probe
    confirmed): rows `break-inside:avoid`, Actions+Select `display:none` in print but `table-cell` on
    SCREEN (no screen regression), thead `table-header-group`. Visual `e2e/screenshots/print_transactions2.png`
    — Date/Amount/Description/Category/Account/Tags/✓ columns, semantic red/green amounts, no row-action
    clutter. NOTE: the search/Filters/Export toolbar above the table still prints (minor; left to avoid
    over-broad selectors).
  - **✅ FIXED — the DASHBOARD printed dark (2026-06-24).** A print sweep across screens found the bento
    widgets still printed on a dark `#121214` background (hardcoded `tw.BgTile`, not `var(--bg-card)`),
    so the whole dashboard printed dark-on-dark and unreadable while Accounts/Budgets/Goals/Reports/
    Transactions were already clean. The print rule forced the layout *containers* white but not the
    widgets. Fix: added `.w, .bento .w` to the white-bg print override. MEASURED across Dashboard/
    Accounts/Budgets/Goals under `emulateMedia({media:'print'})` (`e2e/print_screens_verify.mjs`): **0
    dark blocks** on every screen (was 1 deduped class = all `.w` widgets on Dashboard). Screenshot
    `e2e/screenshots/printscan_Dashboard.png` — white widget cards, dark text, semantic colors, chart on
    white. Print is now robust app-wide. CSS-only; build rc=0.
  - **✅ POLISH — hide interactive form controls in print (2026-06-24).** A printed statement showed the
    full-width search box and filter selects (interactive-only noise). Added `input, select, textarea
    { display:none }` to the print block — the ledger data is plain `<td>` text (not inputs), so no
    statement data is dropped. MEASURED (`e2e/print_screens_verify.mjs` + a table probe): in print the
    search box is hidden but all 50 ledger rows still render with full data (date/amount/description/
    category/account); on SCREEN the search box is unaffected. Cross-screen print scan still 0 dark
    blocks; page-chrome verify still 5/5. (The small Filters/Clear/Export `.btn`s still print — left
    intentionally, since `.btn` is shared with content actions and has no safe print-only selector here.)
    Screenshot `e2e/screenshots/print_txn_clean.png`.
- **✅ BUG — raw CSS-rule text leaked onto the Appearance screen (2026-06-24).** Between "Accent" and
  "THEME" the divider rendered as literal text: `{[{border-top-width 1px}] { []} []}{[{border-top-style
  solid}]…}{[{border-color #232325}]…}`. Cause (`internal/screens/appearance.go:91`): the `<hr>` divider
  passed the `tw.BorderT` + `tw.BorderLine` `css.Rule` values **directly as Hr children** instead of
  wrapping them in `css.Class(...)` — so the rule slices were stringified into a text node (the 3
  fragments = BorderT's 2 rules + BorderLine's 1 rule, exact match). Fix: `Hr(css.Class(tw.BorderT,
  tw.BorderLine), Style(…))` (the standard pattern used everywhere else in the file). appearance.go was
  clean (not churned). Build rc=0; `go test ./internal/screens` is N/A (the pkg is `//go:build js &&
  wasm`, native test excludes it — the wasm build IS the compile check). MEASURED
  (`e2e/appearance_no_css_leak_verify.mjs`, 2/2): no `border-*`/`{[{border` text on /appearance, and the
  `<hr>` now renders its top border as STYLE (`1px solid`), not text. Screenshot
  `e2e/screenshots/appearance_fixed.png` — a clean divider line. (Found via a fresh GLAMOR re-scan of the
  previously-unaudited Appearance screen.)
- Mobile note (logged, not fixed — needs a UX decision): at 390px the top period bar
  (`.reso-control` inside `overflow-x:auto .topbar`) overflows to ~1052px, so Quarter/Year/Jump-to/
  Custom-range sit off-screen reachable only by horizontal swipe with no scroll affordance. `.reso-control`
  has `flex-wrap:wrap` (intent to wrap) but the scrollable parent defeats it. Scroll-vs-wrap-vs-hide is a
  responsive design call (cross-ref C19); left for a dev rather than changed unilaterally.
- **✅ BUG FIXED — fixed bottom tab bar obscured the last content on mobile (2026-06-24).** The phone
  bottom tab bar (`.mobile-tabbar`, `position:fixed`, `56px + safe-area`, shown `@media (max-width:640px)`)
  floats over the bottom of the scroll area, but `main.cf-scroll` had `padding-bottom: 0` — so the last
  content was hidden behind it and untappable. MEASURED on /transactions at 390px scrolled to bottom: the
  "Rows per page" 25/50/100/All selector + pagination sat behind the bar (page-size bottom > tab-bar top).
  Found via a mobile GLAMOR scan (no horizontal overflow on any screen; table correctly reflows to cards —
  those are clean). Fix (CSS-only, web/index.html): in the `@media (max-width:640px)` block, pad the
  scroller `main.cf-scroll { padding-bottom: calc(56px + env(safe-area-inset-bottom,0px) + 12px) }`.
  MEASURED (`e2e/mobile_tabbar_clearance_verify.mjs`, 3/3): mobile clearance 68px, the page-size selector
  (bottom 727) now clears the tab bar (top 784); **desktop scroller unaffected** (padding-bottom 0). Build
  rc=0 (no Go). Screenshot `e2e/screenshots/mobile_tabbar_fixed.png` — controls fully visible above the bar.
  (Aside, not changed: the collapsed icon rail co-exists with the bottom bar on phone — intentional, since
  the 4-item bottom bar doesn't cover Goals/Reports/Planning/etc.; the rail is the full nav.)
- **✅ A11Y FIXED — danger button text failed AA in dark (2026-06-24).** `.btn-danger` (the destructive
  confirm button: Delete/Wipe in `confirmModal`) used `background: var(--down)` + white text — but in dark
  `--down` is a deliberately SOFT red (`#d8716f`, tuned for amount/text legibility), so white-on-it
  measured **3.23:1** (fails WCAG AA for normal text). Light was fine (6.15). Fix (CSS-only,
  web/index.html): give `.btn-danger` a dedicated constant danger red `#c0392b` (danger shouldn't vary by
  theme; `--down` stays soft for amounts). MEASURED (`e2e/btn_danger_contrast_verify.mjs`, 2/2): white on
  `#c0392b` = **5.44:1** in BOTH themes (AA). Visual `e2e/screenshots/btn_danger.png` — vivid red "Delete"
  legible and clearly distinct from neutral "Cancel". Build rc=0.
  - **Confirm-dialog system verified solid (no change):** destructive confirms use `role="alertdialog"`,
    the danger button, focus defaults to **Cancel** (WCAG 3.2.4, so Enter can't trigger the danger
    action), a focus trap, and Enter-confirm/Esc-cancel — all correct (`internal/app/dialoghost.go`).
  - **⚠ Observation (logged, churned Go — not changed):** the sample-data banner's **"Start fresh" wipes
    all financial local state and reloads with NO confirmation dialog** (`samplebanner.go` calls
    `wipeFinancialLocalState` directly). Acceptable for pure demo data, but if a user has added real
    entries on top of the sample they're wiped with one click — consider a confirm (cross-ref the L50
    bulk-delete-no-confirm and L80 mark-paid-no-guard "destructive action needs a guard" theme).
- **✅ VERIFIED — first-run / empty (welcome) state is solid (2026-06-24).** Inspected the no-data state
  (via "Start fresh" in an *ephemeral* playwright context, so no real/persisted data touched). The
  welcome hero ("Your money, beautifully organized." + "Load sample data" / "Add your first account"),
  the "All clear — nothing urgent right now." attention widget, clean $0.00 KPI tiles, and every bento
  widget's friendly empty message + "Add a budget/goal/to-do/account" CTA all render correctly with
  **zero JS errors**. Good first impression. Screenshot `e2e/screenshots/empty_dashboard.png`.
- **✓ RESOLVED (option a landed 2026-06-24) — `.btn-primary` dark-mode text unified to white.** The app's
  most-used button (Save, Add transaction, Load sample, every empty-state CTA, …) is `background:
  var(--accent)` (#2e8b57) with **theme-specific text**: was `#052e13` (dark green) in dark, `#fff` in light.
  MEASURED before: dark **3.52:1**, light **4.25:1**. The dark `#052e13` was the *weaker* of the two, so I
  unified dark to white (`web/index.html` `.btn-primary { color:#fff }` for both themes; removed the now-
  redundant light override). MEASURED after (e2e `btn_primary_consistency_verify.mjs`, both themes): **4.25:1
  white-on-accent, consistent** — dark improved 3.52 → 4.25; screenshot `e2e/screenshots/btn_primary_dark.png`
  shows white "Save" on bright green. Both clear AA-large/UI (3:1) on the 600-weight label; brand accent kept.
  **Remaining for a dev (full AA-normal 4.5):** (b) darken the default `--accent` slightly so white clears 4.5,
  or (c) formally accept ~4.25 as AA-large for the bold label — a brand/design call, not done unilaterally.
  - **✅ RESOLVED 2026-06-25 (full AA-normal, via the button only — brand accent untouched).** Took a
    cleaner path than (b): rather than darken the global `--accent` (which would shift the whole app),
    darkened just the **`.btn-primary` gradient's top stop** in `web/index.html` —
    `linear-gradient(180deg, color-mix(--accent 90%, #000 10%), color-mix(--accent 78%, #000 22%))`
    (was raw `--accent` → `--accent 85%`). White text now clears AA-normal across the entire gradient:
    **top 5.07:1, bottom 6.32:1** (was 4.25 at the raw-accent top), MEASURED by resolving the `color-mix`
    stops to sRGB in-browser. This is one shared rule, so every primary CTA app-wide is fixed —
    confirmed `Add transaction` (dashboard), `Mark paid` (`bills_screen.go:306`), `Choose image`
    (`documents_image_import.go`) all use `btn btn-primary`. Brand green + 180° gloss preserved
    (screenshots `e2e/screenshots/btn_primary.png`, `dashboard_btn_aa.png`); build rc=0; 0 JS errors.
    (Supersedes the deferred white-on-accent note at the GX14 sweep above.)
- **✓ RESOLVED (2026-06-24, CSS-only) — `.rank-badge` dark-mode text unified to white (sibling of the
  btn-primary fix).** The Allocate ranked-suggestion ordinals (#1..#N) used the SAME accent chip with
  `color:#052e13` (dark green) in dark mode = MEASURED **3.52:1** on the accent — the weaker outlier vs
  light's white 4.25:1. Unified dark → white (`web/index.html`; removed the redundant
  `[data-theme="light"] .rank-badge` override). MEASURED after (`e2e/rankbadge_contrast_verify.mjs`, both
  themes): **4.25:1 white-on-accent, consistent** (dark 3.52 → 4.25). Screenshot
  `e2e/screenshots/rankbadge_dark.png` shows white "#1" on the green chip. Same remaining dev option as
  btn-primary for full AA-normal 4.5 (a brand `--accent` call).
- **a11y — every Settings control now has an accessible name (2026-06-25, keep-tidy, WCAG 4.1.2).** A
  sweep for controls with no accessible name (no text/aria-label/title/label-for/wrapping-label) found
  the top-level screens + add-transaction modal already clean (0), but **Settings had 4 unnamed control
  types** a screen reader would announce with no context: the FX rate inputs (`fxRateRow`), the
  freshness-threshold day inputs (`freshnessRow`), the widget-config number+select (`widgetCfgField`),
  and the workspace-switcher startup select (`wsswitcher.go`). Added `aria-label`s mirroring each visible
  label (new i18n keys `settings.fxRateAria`, `settings.freshnessAria`; widget/ws reuse existing labels).
  MEASURED: Settings now has **0 unnamed controls** (29/29 named); FX inputs read "Exchange rate: 1 AUD
  in USD" etc. build rc=0, i18n test ok. (FlipPanel already had role=dialog/aria-modal/aria-label.)
- **Desktop GLAMOR re-check (2026-06-25, keep-tidy) — all main screens clean, no defects, 0 console
  errors.** Swept 10 screens at 1440px (Dashboard, Transactions, Accounts, Budgets, Reports, Goals,
  Planning, Insights, Subscriptions, Bills): each has a polished hero/stat strip, well-structured rows, and
  proper actions; screenshots `e2e/screenshots/glamor_{goals,planning,insights,subscriptions,bills}.png`.
  Verified two "looks off" suspicions were actually correct: (a) Goals' "by 2026-12-01" sub-line uses
  `pr.FormatDate` (respects the date-style pref, not hardcoded); (b) individual-budget owner tags render as
  designed. **No change warranted — did not manufacture one** (the app is in good shape after the recent
  fires). Worktree note: the prune from the prior fire held (only `main` remains; the lingering
  `LoadSmartSettings undefined` LSP error is stale gopls cache — the symbol exists, build rc=0).
  - [ ] **(OPTIONAL, product call — NOT a bug) date-style default is `DateISO` (2006-01-02).** Every date
    across the app renders ISO by default (deliberate — `prefs.Default()` sets `DateStyle: DateISO`, and it's
    user-changeable to US `01/02/2006` or Long `Jan 2, 2026`). For an everyday US-household audience, a
    friendlier default (Long/US) might read better, but ISO is a defensible unambiguous/sortable choice — so
    flagging for Cam's decision rather than changing a deliberate default unilaterally.
  6 screens at 390px: **zero horizontal page overflow** anywhere. Re-measured the two known mobile-nav items
  (already logged under B31, lines ~1236-1241) and confirmed them unchanged: (a) the topbar period controls at
  ≤480px are a *deliberate* horizontally-scrollable strip (GX7-F2, scrollWidth 969 > 334) — the prev/next
  stepper + Quarter/Year sit off-screen-right but are reachable via scroll; working as designed, not a bug;
  (b) at phone width both the 56px icon rail AND the bottom tabbar render — the tabbar covers only 5 of ~27
  destinations, so the rail can't be hidden without a "More"/drawer affordance (the intended B31 phone
  drawer-rail phase). No code change made — reversing (a) would override a documented tradeoff and (b) is a
  dev-sized shell feature; both correctly remain B31 work.
  - **L50-T1 / L80-T1 / "Start fresh" all share the "destructive action needs a guard" theme** — worth
    a single pass adding confirms to unguarded destructive/money actions.
- **Dollar amounts too muted in light** (§3) — `.budget-amount` moved to the strong (`#1c1c1e`)
  light-mode group so the figures Tomas compares are full-contrast, not secondary grey.
- **"Next due" date hyphenating at 768** (§2) — `.stat-value { white-space: nowrap }` keeps the ISO
  date ("2026-07-01") on one line instead of breaking to "2026-07-" / "01".
- **Horizon filter + Show-all toggle** (§1, G11 follow-up, 2026-06-23) — bills default to 90-day
  window; a "Show all (N)" / "Show next 90 days" toggle exposes the full list on demand.
- **Two-column layout at ≥1024 px** (§1, G11 follow-up, 2026-06-23) — `.bills-layout` flex
  container puts the bill list left and the calendar right at wide viewports so both are visible
  without scrolling; stacks on narrower screens.
- **Fixed trailing action-button group** (§2, G11 follow-up, 2026-06-23) — `.bill-sub-actions`
  wraps "Mark paid" + "Remind me" in a `flex:none` trailing group so the bill name and amount have
  horizontal priority, mirroring the G10 `.sub-actions` pattern.

**The story**
Tomas opens Bills on a Monday morning to know exactly what he owes and when. His goal in
under ten seconds: see the total he needs to cover this cycle, spot any bill due today or
overdue, identify what's coming up in the next 7 days, and mark paid the ones he's already
settled. The calendar gives him a monthly at-a-glance map of when money will leave his
account. The page must surface urgency (overdue = red, due soon = amber), total due, and
the soonest-due bill immediately — without scrolling. Mark-paid must be one tap away per row.

**Drive script**
`e2e/glamor_11_bills.mjs` — widths 1280/1440/768, dark + light themes (light-theme
recipe: set `cashflux:prefs` in localStorage, reload, wait for `data-theme="light"`). Navigates
from `/` via in-app click ("Bills" nav link) to avoid the wasm deep-link 404 (B1).
Captures 8 screenshots plus a DOM audit JSON and a light-mode contrast spot-check. Run:
`node e2e/glamor_11_bills.mjs` against `:8099`.
Screenshots in `e2e/screenshots/glamor_11_bills_*.png`.

**Build/run evidence**
- `node e2e/glamor_11_bills.mjs` → EXIT 0
- Screenshots captured:
  `glamor_11_bills_1280_dark.png`, `glamor_11_bills_1280_dark_full.png`,
  `glamor_11_bills_1440_dark.png`, `glamor_11_bills_768_dark.png`,
  `glamor_11_bills_1280_light.png`, `glamor_11_bills_1280_light_full.png`,
  `glamor_11_bills_1440_light.png`, `glamor_11_bills_768_light.png`
- DOM audit: `glamor_11_bills_dom.json` — 2 cards ("Bills", "June 2026 calendar"),
  4 stat items (Total due soon $2,285.00 / Per year $23,550.00 / Upcoming bills 7 /
  Next due 2026-07-01), 7 rows, 7 mark-paid buttons (all present), 7 remind buttons,
  2 cal-dots, 7 cal-head cells, today-cell present, 0 overflow cards, 0 page errors.
- `statAboveFold: true` confirmed at 1280px dark.
- `hasMarkPaid: true`, `markPaidCount: 7` — mark-paid confirmed on every row (C57 fix verified).
- `hasUrgency: false` — no overdue or within-3-day bills in the sample data snapshot
  (all bills 8–206 days out); urgency code exists in source and is correct.
- `dataTheme: "dark"` confirmed on dark captures; `"light"` on light captures.
- theme after hard-reload: `"dark"` (persistence confirmed correct).
- Light contrast spot-check: `cardTitleColor: rgb(244, 244, 245)` on white background
  (card title near-invisible — same systemic `--fg` token failure as G4–G10).
  `rowDescColor: rgb(28, 28, 30)` — bill names ARE legible in light mode (improved vs.
  G10 where row names were invisible). `rowMetaColor: rgb(86, 86, 92)`, `budgetAmtColor:
  rgb(86, 86, 92)` — muted grey on white for due-date labels and amounts. `statLabelColor:
  rgb(86, 86, 92)` — stat labels faint. `urgencyColor: N/A` (no urgency elements in data).

**What already works well (keep — regression anchors)** ✓
- **Stat grid is the very first element and is above the fold at all widths.** Total due soon
  ($2,285.00 in red), Per year ($23,550.00), Upcoming bills (7), and Next due (2026-07-01)
  are all visible without scrolling at 1280 and 1440. `statAboveFold: true` DOM-confirmed. ✓
- **Mark-paid button present on every row (C57 fix confirmed).** `markPaidCount: 7` — all 7
  bill rows carry a green "Mark paid" `.btn-primary` button. C57 noted "no mark-paid" as the
  top deficiency; it is now fully implemented. ✓
- **Remind me button present on every row.** `remindCount: 7` — all 7 rows carry a "Remind me"
  button that creates a to-do dated to the bill's due date. ✓
- **Bill names are visible and lead the row at all widths.** `rowDescColor: rgb(28,28,30)` in
  light mode (full-weight foreground token) — bill names (Rent, Gym membership, Streaming &
  apps, Student Loan, Rewards Credit Card, Car insurance, Domain & hosting) are clearly
  readable in both dark and light screenshots. This is markedly better than G10 Subscriptions
  where names were invisible at 1280/1440. ✓
- **Urgency code is wired correctly.** Source: `billUrgencyTone()` applies `text-down` for
  overdue/today and `text-warn` for within 3 days. No urgency elements appear in the sample
  data (all bills 8–206 days out), which is correct behavior. ✓
- **Ordering is soonest-due first.** DOM audit `rowMetas` confirms ascending date order:
  2026-07-01 → 2026-07-03 → 2026-07-05 (×2) → 2026-07-22 → 2026-09-01 → 2027-01-15.
  Tomas's most pressing payment is always at the top. ✓
- **Calendar renders with today-cell and dot indicators.** `hasCalendar: true`, `hasTodayCell:
  true`, `calDots: 2`, `calHead: 7` — the June 2026 calendar grid is present with today
  highlighted and 2 dot indicators on bill-due days. ✓
- **No horizontal overflow at any width.** `overflowCards: 0` at 1280px dark. ✓
- **Zero JavaScript page errors.** Both dark and light sessions clean. ✓
- **Download CSV is present.** `hasCsvBtn: true` — "Download CSV" in the Bills card footer. ✓

**Structure fixes (bottom-up)**

*1. Layout — calendar is below the fold, disconnected from the urgency story*
*2. Spacing — row density and button visual weight at 768px*
*3. Theming — systemic light-mode token failure (G4–G10 pattern recurring)*
*4. Styling — urgency visualization absent from current data snapshot*
*5. Positioning — calendar placement vs. urgency hierarchy*
*6. Ordering — bills list includes 206-day-out bill with no horizon indicator*
*7. General UX / Glanceability — "What's Due This Week" use case assessment*
**UI/UX defects (screenshot-confirmed)**

| # | File | Symptom | Fix |
|---|------|---------|-----|
| D1 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png`, `glamor_11_bills_768_light.png` | Card titles "Bills" and "June 2026 calendar" near-invisible in light mode — computed `rgb(244,244,245)` on white; WCAG AA fail (≈1.02:1). Eighth consecutive page with this systemic `--fg` token failure | `h2.card-title` must use a strong foreground token in light mode; global CSS token fix |
| D2 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Dollar amounts (`.budget-amount`) render as muted grey `rgb(86,86,92)` in light mode — the key payment figures Tomas needs to read are styled as secondary text | `.budget-amount` should use `--fg` (strong) in light mode, not a muted token |
| D3 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Due-date + days-until metadata (`rgb(86,86,92)`) low-contrast on white in light mode — the operationally critical "due in 8 days" label is muted when not urgency-colored | `.row-meta` should use a higher-contrast token in light mode for non-urgency state |
| D4 | `glamor_11_bills_768_dark.png`, `glamor_11_bills_768_light.png` | "Next due" stat card wraps the ISO date "2026-07-01" across two lines as "2026-07-" / "01" at 768px — the hyphenated break reads as a formatting error | Use a shorter date format at narrow widths (e.g. "Jul 1") or use a 2×2 stat grid at 768px instead of 3+1 |
| D5 | `glamor_11_bills_1280_dark.png`, `glamor_11_bills_1440_dark.png`, `glamor_11_bills_1280_light.png` | Calendar is below the fold at all widths — the month map is a scroll destination, not a contextual aid visible alongside the list | Move calendar to a side-by-side layout at ≥1024px (list left, calendar right) so both are visible without scrolling |
| D6 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Stat grid remains visually dark (dark card backgrounds) even when `data-theme="light"` is active — creates a two-tone page (dark header stats + white content area) | Stat cards must adopt the light-mode background token when `data-theme="light"` |

**Re-screenshot close-out requirement:** After D1 (card title contrast), D2/D3 (amount and
meta-label contrast fixes), D4 (768px date hyphenation fix), and D5 (calendar above-fold fix),
re-run `node e2e/glamor_11_bills.mjs` and confirm: (a) card titles readable in all light
screenshots, (b) amounts and due-date labels readable in light mode, (c) date no longer
hyphenates at 768px, (d) calendar visible within the fold at 1280/1440, (e) all 8 screenshots
captured cleanly.

- [x] **D1–D5 RE-VERIFIED RESOLVED (2026-06-25, keep-tidy GLAMOR re-check).** Drove Bills in TRUE light
  mode (toggled via the Appearance theme control + in-app SPA nav, so the SQLite-backed pref actually
  applied — a hard `goto` reload reverts to dark, which masked this in earlier runs). MEASURED
  `getComputedStyle`: **D1** card-title `rgb(28,28,30)` on white (~15:1, AA pass); **D2** `.budget-amount`
  `rgb(28,28,30)` (strong); **D3** `.row-meta` `rgb(60,60,67)` (~8.6:1, AA pass) — all fixed by the
  systemic light-mode `--fg` token work. **D4** "Next due" ISO date `2026-07-01` renders on one line at
  768px (no hyphen break) — does not reproduce. **D5** calendar is side-by-side with the list at ≥1024px
  via the `bills-layout` two-column rule (`bills_screen.go:174`). D6 has a light-mode `.stat` background
  rule (`web/index.html:793`). 0 JS/console errors. The Bills GLAMOR review is clean.

**Probe hardening**
- Drive script uses in-app navigation (click "Bills" nav link from `/`) rather than direct
  deep-link to `/bills` — required because `gwc dev` returns 404 for non-root paths (B1).
- Wait condition is `.stat-grid, .card` — stat-grid is conditional on data being present;
  fallback to `.card` accommodates the empty-state (no accounts with due-day → empty bill
  list renders just the empty-state card without a stat-grid).
- "View as member" reset: removes `viewAsMember` from `cashflux:prefs` before navigation.
- Light theme set via the full localStorage recipe (set + reload + waitForFunction on
  `data-theme="light"`) rather than a nav click.
- Hard-reload probe: script reloads after dark screenshots and re-checks `data-theme` to
  confirm the dark preference persists (confirmed: "dark" after reload).
- Urgency-tone probe gap: no bills in the sample data fall within the 3-day warning or
  overdue windows, so `text-down` / `text-warn` styling cannot be screenshot-confirmed.
  A future fixture seeding an overdue bill and a 2-day-out bill is needed to close this gap.

**Cross-references**
- C57: "Bills clean calendar, but no mark-paid, no urgency tone, + a suspect annual figure"
  (marked DONE 2026-06-21). Mark-paid: confirmed present (7 buttons, `markPaidCount: 7` ✓).
  Urgency tone: code is wired, sample data has no urgency-triggering bills (evidence gap, not
  regression). Annual figure ($23,550.00): plausible for the sample data mix of monthly + yearly
  obligations; not confirmed suspect from this snapshot.
- L54: Bills page verified with mark-paid working (loop story screenshots `loop54-04-bills-page.png`,
  `loop54-11-bills-after-recurring.png`). Current audit confirms this remains correct.
- G4/G5/G6/G7/G8/G9/G10: Same systemic `--fg` light-mode token failure — D1 is the eighth
  consecutive page; a global CSS token fix (not per-page patch) is the only sustainable resolution.

---

# Probe scripts written and run 2026-06-23:
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_main.mjs    # boot + CSS var validation
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_full.mjs    # multi-page audit (exit 0)
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_deep.mjs    # deep Reports chart audit
```

**Exit codes:** `gx16_main.mjs` exit 0 · `gx16_full.mjs` exit 0 · `gx16_deep.mjs` exit 0

**Server:** `gwc dev` on `http://localhost:8080` (multiple gwc processes confirmed running). SPA root serves correctly; routes navigated via `history.pushState` + `PopStateEvent`. Build state: GI0 (wasm build broken) — stale wasm binary boots from existing `static/bin/main.wasm`; wasm did mount successfully (confirmed `.topbar` rendered).

**Theme injection method:** `addInitScript` overrides `localStorage.getItem('cashflux:prefs')` to return `{theme:'light'}` before page load; the inline `<head>` script reads this synchronously and sets `data-theme="light"` on `<html>` before first paint. Confirmed: `data-theme: light` on `document.documentElement` after load.

---

## W. WONDER — configurable animated flourishes (theme-engine driven) ★★
<!-- Batch 2 landed 2026-06-23: W-11 list stagger, W-12 bento entrance, W-13 modal backdrop blur, W-14 toast spring, W-16 progress ease (verified), W-19 skeleton shimmer, W-20 focus ring ease. W-21 deferred (needs IntersectionObserver JS). -->

### W1. WONDER — animated-flourish system: architecture + token layer (theme-engine driven) — 2026-06-23 ★★

**The vision.** Make CashFlux feel *alive* — clean, fast, beautiful micro-animation everywhere: page
transitions, hover effects, click/press feedback, focus, list reveals, value changes. It must be
**configurable by the theming engine** and **adjustable** (a single intensity dial), **extensive** but
**tasteful** (calm + fast, never gaudy or janky), and fully **reduced-motion safe**.

**Architecture — a token layer driven by a `data-wonder` attribute (mirrors `data-theme`/`data-density`).**
The whole system reads a small set of `--wonder-*` design tokens. The theme engine sets
`[data-wonder="off|subtle|full"]` on `<html>` (the same mechanism as `data-theme`), and — once GI0 is
fixed — emits `--wonder-*` via `theme.CSSVars()` so the theme editor gets an intensity slider. Every
flourish multiplies its transform by `--wonder-on` (0..1), so ONE dial scales the entire app smoothly.
`prefers-reduced-motion: reduce` forces everything off. **This means flourishes land in pure CSS now and
become theme-configurable the moment the Go side emits the config — no rework.**

**Token layer (LANDED in `web/index.html`):**
`--wonder-on` (0..1 master multiplier) · `--wonder-dur-fast/dur/dur-slow` · `--wonder-ease` /
`--wonder-ease-out` · `--wonder-lift` (hover rise) · `--wonder-press` (click scale) · `--wonder-shadow`.
Levels: `[data-wonder="off"]` (zeroes all), `[data-wonder="subtle"]` (~55%), default/`full` (100%).

**What's LANDED now (CSS, foundation — [CSS-ONLY], live):**
**EXTENSIVE catalog — the flourishes to build (grouped; tasteful + fast). [CSS-ONLY] unless noted:**
*Interaction feedback*
> Batch 1 (W-3..W-8) fully landed 2026-06-23 — all CSS, token-driven, reduced-motion safe.

*Entrance / reveal*
- [~] W-10 — Route cross-fade (View Transitions API) — PARTIAL (2026-06-24): CSS scaffold + view-transition-name + ::view-transition-* keyframes landed; startViewTransition wraps the W-9 class-toggle for progressive enhancement. True old→new cross-fade blocked by GWC framework constraint: UseEffect fires post-render, so the outgoing page snapshot is already replaced when triggerPageEnter runs. Scaffold is ready for a pre-render hook if GWC exposes one.
  - **✅ BUG FIXED (2026-06-24, keep-tidy) — "call to released function" on EVERY route change.** A
    console-error health sweep across 9 screens caught one error firing once per navigation. Root cause in
    `internal/app/pageenter.go`: the W-10 path built a `js.FuncOf` cb and `defer cb.Release()`'d it, on the
    (wrong) assumption that `crossFade` "invokes cb synchronously in both paths." But `crossFade` →
    `document.startViewTransition(cb)` runs its update callback **asynchronously** (later microtask), so cb
    was released BEFORE the browser called it → `call to released function`. In Chromium (View Transitions
    supported + motion on) this was the default path, so it fired on every route change. Fix: cb now
    **self-releases after it runs** (correct for both the async view-transition path and the sync
    direct-applyFn fallback). Also hardened the fallback double-rAF path, which previously leaked 2 js.Funcs
    per route change (callbacks never released) — both rAF callbacks now self-release too. MEASURED: released-
    function sweep **9 hits → 0**; page-enter class still applies on **4/4** navigations (W-9 animation intact);
    console errors **0** across all screens. build rc=0; `go test ./internal/app` ok. No JS/CSS change — Go only.
  - **✅ BUG FIXED (2026-06-24, keep-tidy) — "AbortError: Transition was skipped" under rapid navigation.**
    A stress sweep (rapid-fire route switching ×3 + menu open/close) surfaced 11 unhandled-rejection console
    errors. Cause: `document.startViewTransition` (used by `crossFade` in `web/wonder.js`) returns a
    ViewTransition whose `.ready`/`.finished` promises REJECT with an AbortError when a subsequent navigation
    starts a new transition before the current one settles — expected during fast switching, but the
    rejections bubble up as unhandled-promise console errors. Fix (JS-only, wonder.js): capture the returned
    transition and attach no-op `.catch()` handlers to its `.ready`/`.finished`/`.updateCallbackDone` promises
    (the DOM swap in applyFn has already run, so the visual transition being skipped is harmless). MEASURED:
    stress-sweep console errors **11 → 0**; page-enter still fires **4/4** navigations. build rc=0; sw cache
    v261→v262. Together with the released-function fix above, the route-change path is now console-clean under
    both normal and rapid navigation.
*Value / state changes*
*Polish*
**Theme-engine integration (GO-STRUCTURAL, build-gated GI0):**
**LANDED 2026-06-23 — Motion pref (full/subtle/off) wired end-to-end.**

**Principles (enforce in every flourish):**
- Fast (≤ ~200ms for feedback, ≤ ~320ms for entrances) + a single shared easing family.
- Transform/opacity only (GPU-friendly) — never animate layout properties; no `transition: all`.
- Everything reads `--wonder-*` + scales by `--wonder-on`; nothing hardcodes a duration/transform.
- Reduced-motion + `[data-wonder="off"]` must yield a completely static app.
- Tasteful restraint — flourishes guide attention, never distract; no infinite loops outside loaders.

**Probe hardening / acceptance.** A WONDER e2e should: (a) hover a `.card` and assert a non-identity
`transform` (lift) in default/full, (b) set `[data-wonder="off"]` and assert identity transform, (c)
`emulateMedia({reducedMotion:'reduce'})` and assert identity transform, (d) measure flourish durations
trace to `--wonder-*`. `e2e/w1_verify.mjs` covers the landed foundation.

**Cross-refs:** GX8 (motion inventory + reduced-motion coverage — WONDER builds on it), 6.16 (interaction
polish), B20 (theme engine — the config home), GI0 (build blocker — gates the theme-engine integration),
GM (modal flip), GX5 (toast), G9.1a (chart draw-in).


## 0. Foundation & tooling (Phase 0)

> ⚠️ OPS NOTE (2026-06-24) — if the app shows only the boot splash (blank `#app`, console
> "Refused to execute wasm_exec.js (MIME text/plain)" + "WebAssembly compile: status not ok"),
> the git-ignored build artifacts `web/wasm_exec.js` and/or `web/bin/main.wasm` are missing
> (a concurrent `git pull --rebase`/clean wiped them this session). Restore:
> `cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js` and rebuild
> `GOOS=js GOARCH=wasm go build -o web/bin/main.wasm .`. Done + verified 2026-06-24 (app boots
> ~3s, dashboard renders 17 cards, 0 console errors). NB: the wasm is ~60MB (≈2-3× normal) due to
> the in-flight Cloud-sync changeset — worth a size audit once that work lands.

- [~] **README.md** — what CashFlux is, the stack (Go→wasm on GoWebComponents), local dev (`gwc dev`),
      build/test commands, the local-first + BYO-AI-key model, badges, a **Live demo** link to the
      GitHub Pages build, a License section, and pointers to SPEC/DEVLOG/TODOS — all present.
- [~] **MIT licensing.** Set the project up under the MIT license.
- [ ] Fix framework `gwc dev -html` resolution (commit in GoWebComponents, rebuild + recopy `gwc`)
- [ ] Install Claude Code design skills (`frontend-design`, `playground`) — user action
---

## 1. Phase 1 — Local household core

### 1.2 Money & currency — ★

- [~] Money formatting per currency: `FormatMinor` (plain decimal) done; symbol/grouping/locale = UI layer
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
App shell & navigation:
Time-resolution control (top bar):
Bento grid system:
- [~] Persist per-user layout — order + spans saved to `localStorage`; hidden/per-page + store persistence later

Per-widget settings (gear → flip):
- [~] Settings fields: editable Title + behavior toggles done; accent swatches/default size/refresh/Remove + persistence later

Widget catalog (each backed by tested logic; see mockup):
- [~] Reusable SVG chart helpers — area/sparkline (`chart` + `ui.AreaChart`) done; bars are div-based; donut later
- [x] **Reports chart palette cohesion (G9.1a polish, 2026-06-25):** the "Spending by category" card showed THREE palettes — the ranked bar chart (uniform blue `#4f8ef7`), the donut (Tableau10 by slice), and the ranked share-bar list (rainbow HSL by rank) — so a category was a different color in each view. Unified them: added an optional per-point `Color` to `chartspec.Point`, taught `web/chart.js` bars to honor `p.color` (empty → series color, backward compatible), and colored both the category bars and the row share-bars with a new `tableau10(i)` helper that mirrors `d3.schemeTableau10` (the donut's own palette) by rank. Now Mortgage is blue, Auto loans orange, Dining red, etc. across the bar chart, donut, and list — the card reads as one picture. MEASURED: bar fills + row-bar fills = `#4e79a7,#f28e2c,#e15759,#76b7b2,#59a14f,#edc949,#af7aa1,#ff9da7`, exactly matching the donut's first-5 slice colors; build rc=0, `go test ./internal/chartspec` ok, 0 JS errors (screenshots `e2e/screenshots/reports_category_card_cohesive.png`). Pure color, no motion (reduced-motion unaffected).
- [x] **Reports hero period-over-period Net delta (G9.1 polish, 2026-06-25):** the hero showed the current-period Net figure with no context. Added a small signed delta chip directly beneath it — "▲/▼ $X vs last period" — comparing Net against the previous comparable window (`prevFlow`, computed once and shared with the existing spend-trend line so no extra query). Up in Net = `pos`/accent, down = `neg`/danger, reusing the established hero tone tokens; hidden when the delta is zero or the prior period can't be computed. New CSS `.hero-net-delta{.pos,.neg}` in `web/index.html`; i18n `reports.vsPrev`/`reports.vsPrevPeriod`. Static element (no animation → fully WONDER-compliant by default). MEASURED live on /reports: chip renders "▼ $1,401.10 vs last period", computed `color rgb(216,113,111)` (danger), 12.8px, `neg` tone; page still renders fully (hero + 87 chart SVGs + 11 cards); build rc=0, `go test ./internal/i18n` ok, 0 JS errors (screenshot `e2e/screenshots/reports_hero_delta.png`).

Global settings (household card → large flip panel):
Shared control components (from mockup):
### 1.8 Members / Household

- [~] Member switcher / filter — per-member "Transactions" drill-down filters the ledger by member;
      global cross-screen member scope deferred (ambiguous semantics)
### 1.9 Accounts (assets + liabilities) ★

- [~] Per-account ledger view — account row "Transactions" button filters the ledger to that account
      and navigates; dedicated running-balance view optional later
- [~] Credit utilization indicator done (on liability rows); due-date reminder via Upcoming bills widget
### 1.10 Categories

- [~] Default scheme + reset; methodology-aware presets (envelope/zero-based) — pure
      `internal/catscheme.Default()` (starter income/expense set + sub-categories), table-tested; the
      reset action (apply via appstate) + methodology presets remain
- [~] Tests: tree building, reassignment — reassignment tested; category tree building N/A (flat list)

### 1.13 Goals

- [~] Contribute-to-goal action done (prompt); auto-progress from linked account later
### 1.14 To-do (budgeting tasks)

- [~] Sort (open first, then due, then title) + hide-done filter done; more filters later
- [~] Tests: ordering (pure `internal/tasksort` — Order/Visible, table-tested); status transitions still UI

### 1.15 Freshness & friendly nudges

- [~] Dashboard nudge widget ("N balances could use a refresh") done; dismissible + one-tap update later
### 1.17 Dashboard

- [~] This-month income/expense (done); balance trend snapshot (later)
### 1.18 Settings

- [~] Preferences: theme/density, week-start, fiscal-month start, number/date formats
      — theme (dark/light/system) + accent + density + week-start + date format all complete &
        reload-persistent (engine + atom + Settings UI + `ApplyPrefs` + light/dark skins);
        only fiscal-month start remains
## 2. Phase 2 — Intelligence & power tools (OpenAI, client-side)

### 2.1 OpenAI client — `internal/ai`

- [~] Vision input support (images/PDF pages) for document parsing — `ai.BuildVisionRequest` (pure) done
### 2.2 Documents — AI import

- [~] Upload UI (CSV paste + image picker) done; PDF + drag-drop later
- [~] Tests: CSV parsing (store) + extraction parsing/dedupe (`extract`) done; extraction→txn mapping is UI

### 2.3 Insights & NL query

- [~] Natural-language query over data → answer (Insights "Ask about your money"); richer data context later
### 2.5 Formula builder + sandboxed engine — `internal/formula`

- [~] Variable resolution: live figures (net worth/income/expense/counts) done via `Env`; custom fields + filtered aggregates later
- [~] Typed results (number/bool/text) done; money/percent typing + formatting later
- [~] Builder UI: live preview + error messages + example chips done (Customize); guided insert later
### 2.6 Planning + Forecast

- [~] ★ Forecast engine (pure): `internal/forecast.Project` over horizon from start + recurring + one-time items done; actuals-derived recurring later
- [~] What-if scenarios: extra debt payment + trim-spending forecast done; add-recurring/rate-change later
- [~] Forecast visualization (net-worth curve) done on Planning; scenario comparison later
## 3. Phase 3 — Sync & PWA

> **§3.1–3.2 are superseded by [§7. Backend server](#7-backend-server--sync--ai-proxy-grpc-bridge-hybrid-)**
> (gRPC-bridge hybrid: LWW sync + AI proxy over gRPC; OAuth + blobs over HTTP). Stubs kept for history.

### 3.3 PWA / offline

- [~] Web manifest done (`manifest.webmanifest` + theme-color/apple meta); icons later
- [~] Installability prompt done (beforeinstallprompt button); offline read works (sw); update flow later
---

## 5. Future / nice-to-have (post-core)

Lower-priority items to pick up **only after the core product (Phases 0–3) is complete**. These are
enhancements, not part of the core spec; sequence them after the Phase 3 / sync work.

### 5.1 Standalone desktop app via Electron

Wrap the existing WASM/PWA build as a native, installable desktop app (Windows/macOS/Linux) so
CashFlux can be distributed and launched outside the browser while reusing the exact same Go→wasm
bundle and `web/` shell. Local-first; no behavior change — just a native window + installer.

- [ ] Verify: app installs and launches natively, loads offline, and matches the PWA behavior

---

## 6. UX / UI polish pass (2026-06-18 audit — static review of shell, screens, controls, CSS)

Findings from a full static UX/UI sweep (typography, shapes/sizing/weights, fonts, legibility/contrast,
shortcuts, click-to-item speed). Grouped by theme; `[H]/[M]/[L]` = severity. File refs are starting
points — verify exact lines before editing.

### 6.16 UI interaction & motion polish (2026-06-18 pass 7 — animations, hover, micro-interactions)

The motion **foundation is good**: FLIP-animated bento reorder/resize (`web/flip.js`), the settings flip-panel
(`transform .55s cubic-bezier`), boot loader + `#app` settle-in, toast enter, collapsed-rail flyout, switch
toggle, and a thorough `prefers-reduced-motion` block. The gap is the **micro-interaction layer** — the small
feedbacks that make a UI feel responsive and alive. Mostly enhancement-grade ([M]/[L]), ordered by bang-for-buck.
All additions must be wrapped in `@media (prefers-reduced-motion: no-preference)` (or no-op'd in the existing
reduced-motion block) to stay consistent with the app's a11y stance.

**Press / tactile feedback**
**Hover affordances**
**Data-viz & progress animation**
**Enter / exit transitions**
**Stateful micro-interactions**
- [~] **[L]** Active nav pill (`.nav-link.active` / `.nv`) jumps between items on route change. Consider animating
      a shared active indicator that slides to the selected item.
      **PARTIAL — grow-in landed (2026-06-24, keep-tidy, CSS-only).** The GD-15 "you are here" bar was an
      inset box-shadow (`aside.rail .nv.active`) that snapped between items. Converted it to an absolutely-
      positioned `aside.rail .nv.active::before` accent bar (3px, `var(--accent)`, theme-agnostic, no layout
      shift) that **grows in** via `@keyframes wonder-nav-bar-in` (scaleY + opacity, `--wonder-dur`/
      `--wonder-ease-out`) each time an item becomes active — so the new item's indicator eases in instead of
      hard-snapping. Removed the now-redundant light-mode box-shadow bar (index.html ~847) so both themes use
      the one animated `::before`. WONDER-gated: `[data-wonder="off"]` and `prefers-reduced-motion` → `animation:
      none` (and the keyframe's `from` collapses to `to` when `--wonder-on=0`), bar stays statically visible.
      MEASURED (6/6): full → bar present (3px, rgb(46,139,87)) + `animation-name: wonder-nav-bar-in`; off →
      bar present, `animation: none`; reduced-motion → bar present, `animation: none`. Bar renders accent in
      BOTH themes. Screenshots `e2e/screenshots/nav_pill_{dark,light}.png`. sw cache v258→v259. build rc=0.
      **Still open (the true "slide"):** a single shared indicator that physically slides between items needs
      JS measurement/FLIP — the framework's `Style()` drops CSS custom props, so a var-driven CSS slide isn't
      feasible; deferred as a larger JS change. The grow-in removes the worst of the "jump" for now.
> **Note:** animations/hover are hard to verify from still screenshots; this pass is a CSS/JS interaction audit.
> A future check could record short Playwright videos (`recordVideo`) of hover/drag/toast flows to confirm feel.

## 7. Backend server — sync + AI proxy (gRPC bridge hybrid) ★

> Supersedes the stubs in §3.1–3.2. Design: [`docs/BACKEND_PLAN.md`](./docs/BACKEND_PLAN.md).
> **Locked decisions:** last-write-wins sync (newest-by-timestamp) · per-user **BYO** OpenAI key
> stored **encrypted at rest** · auth via **OAuth (Google/GitHub)** · artifacts in a
> **content-addressed blob store** (refs only in the synced snapshot) · **gRPC over the GWC
> `GoGRPCBridge`** (WebSocket) for the app's data/AI RPCs · **plain HTTP** for OAuth + blobs.
> Thin server: it stores and forwards, never interprets the dataset. App stays local-first; the
> backend is an optional sync/proxy tier. Build bottom-up (proto/contract → storage → services →
> transport → client), one feature per commit, tests with each layer.

### 7.7 Client integration (wasm app) ★
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
- [~] Settings: backend URL, sign in/out, sync status; conflict/LWW UX ("a newer version was on the server - pulled it").
      Done: backend URL/token, test connection, key upload, Cloud/self-host mode, Sync now, and sync status are
      in Settings. OAuth sign-in buttons and local sign-out are wired. Remaining: richer conflict action sheet.

### 7.8 Security & privacy ★
- [~] TLS everywhere; OAuth `state`/PKCE; never log secrets; threat-model pass; `govulncheck` + `gosec` in CI.
      TLS/wss, OAuth PKCE/state, log redaction, Gitleaks, govulncheck, and gosec are covered; remaining:
      formal periodic threat-model and pre-launch pen-test pass.

### 7.9 Deploy & ops
- [~] CI: build server, run server tests, proto-drift check, lint + vuln scan.
      Done: Go tests, explicit server build, wasm build, vet, govulncheck, gosec, and gitleaks. Remaining:
      proto-drift check once codegen is pinned.

### 7.10 Testing & phased rollout
- [~] Integration: in-proc `grpc.Server` behind the bridge over a real WS; client<->server round-trips
      now cover AI `SetKey`/`Chat`/`ChatStream` and SyncService workspace `Put`/`List`/`Get`/`Delete` unary
      calls plus watch streams, with HTTP blob PUT/HEAD/GET verified against a workspace created through the
      bridge. Remaining: browser autosave push/pull e2e.
- [~] e2e: two-device sync (LWW + tombstone), offline->reconnect flush, OAuth login, artifact blob
      round-trip, AI proxy streaming with a real key.
      Done: in-proc bridge e2e covers two-device stale LWW rejection plus tombstone propagation; AI proxy
      streaming has bridge/client transport coverage. Remaining: offline->reconnect flush, OAuth login,
      artifact blob round-trip, and real-key AI proxy smoke.
### 7.11 Monetization — billing + Cloud UX (paid tier) ★

> CashFlux Cloud is the paid tier: sync + backup + AI proxy. App stays free/local-first.
> Design: [`docs/CLOUD_UX.md`](./docs/CLOUD_UX.md) + [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md).
> **Locked:** app free; Cloud paid (annual-first subscription); AI proxy bundled into Cloud; personal
> plan now, household later. Recommended pricing ~$34.99/yr / $3.99/mo, 14-day trial (validate).

#### Server (billing + entitlements)
#### Client (Cloud UX)
#### Launch gating
- [ ] Monetize at the **sync milestone** (auth + snapshot sync + Stripe + trial); AI proxy + blobs land
      as later Cloud upgrades (no price change). Household plan is a later phase.
- [ ] Analytics: trial starts, trial→paid, MRR/ARR, churn, ARPU, storage/user, gross margin (privacy-respecting).

### 7.13 Turnkey self-host deploy + DO referral ★

> One-click(ish) self-host on DigitalOcean, and turn the free self-host path into DO referral credit
> that offsets Cloud infra cost. Design: [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md) §14.
> Keep an unconditional plain self-host path (any host, no referral). Disclose referral plainly.

#### Packaging
- [ ] **DO Marketplace 1-Click**: Packer build of a droplet image; submit for vendor approval (later,
      after the script path is proven).

#### Referral
- [ ] Add the **DO referral link** to the "Deploy your own server" button + install docs + Marketplace
      listing, with a clear disclosure line.
- [ ] Verify current DO referral terms before relying on it; track referral credit as reduced COGS.
#### In-app hook
#### Ops/docs
### 7.14 Security hardening ★

> Defense-in-depth for a server that holds user financial data + encrypted AI keys. Pairs with §7.8.
> Run `gosec` + `govulncheck` in CI from day one; treat every finding as blocking.

#### AuthN / AuthZ
#### Transport / browser
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
#### Abuse / DoS
#### Supply chain / process
- [~] Reproducible builds; SBOM (e.g. `cyclonedx`); sign release artifacts/images (cosign).
      `deploy/release-server.example.sh` now builds the server with deterministic Go flags, writes
      checksums, generates a CycloneDX SBOM, and signs binary/SBOM blobs with cosign. Remaining: CI release
      automation and signed container images.
- [~] Periodic threat-model review; pre-launch pen-test pass; secrets scanning (gitleaks) in CI.
      Gitleaks now runs in CI; remaining: periodic threat-model review and pre-launch pen-test pass.
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
### 7.18 Performance, scale & limits
- [~] Load + soak tests (sync push/pull, blob up/down, AI streaming, WatchWorkspaces fan-out); publish
      a baseline like the bridge's benchmark snapshots; perf regression gate in CI.
      `TestServerLoadSmokeSyncBlobAndWatch` now covers concurrent sync pushes, workspace-watch fan-out, list,
      and blob upload/download through the in-process HTTP/gRPC bridge. Remaining: AI streaming, longer soak
      runs against production-like disk/proxy/network, and a published perf-regression gate.
### 7.20 Anti-abuse & fraud
- [~] Signup/login abuse controls (rate limit, optional CAPTCHA on bursts, email/OAuth verification).
      OAuth/session routes now have a dedicated per-IP `CASHFLUX_SERVER_AUTH_RATE_LIMIT_PER_MINUTE` cap with
      JSON `RATE_LIMITED` errors. Google ID-token verification now rejects missing/expired expiry claims and
      future issued-at claims before userinfo fetch or session issuance. OAuth userinfo rejects explicit
      unverified email claims. Remaining: optional CAPTCHA-on-burst policy only.
      policy and broader email/OAuth verification review.
---

### UI business-logic leak sweep (R-LEAK) — extract computation out of the wasm UI layer (started 2026-06-25)

**Goal:** enforce hard-rule #2 ("logic is platform-independent; never put computation in view code")
across the three UI packages — `internal/ui` (widget library), `internal/screens` (pages), and
`internal/uistate` (UI state). For each file: identify business logic (money/FX math, percentages,
domain aggregation, date-boundary logic, sorting/scoring rules) computed inline in the view, and move
it into the appropriate pure (no `syscall/js`), table-tested package; verify native tests + `GOOS=js
GOARCH=wasm` build + `screenlint`; commit one isolated change at a time (explicit paths only — the
tree is shared with concurrent sessions, never `git add -A`).

**Method per file:** grep for computation signals (`time.*`/`sort.*`/`math.*`/`*100`/`/100`/`.Amount`
arithmetic/aggregation loops) → deep-read suspects → confirm whether a domain helper exists (reuse) or
must be added → extract + test → rebuild/retest → commit + push. Display-only sorting, intrinsic widget
geometry (meter/progress %), and bar-chart max-normalization are **legitimately** left in the view.

**DONE (committed + pushed to origin/main):**
- [x] **internal/ui (all 24 files) — audited clean.** Purely presentational; delegates all computation
      to pure packages (`chart`, `chartspec`, `currency` edge-formatting, `pagination`, `dashlayout`).
      No changes needed. Only in-file math (meter/progress percent-fill) is intrinsic widget geometry.
- [x] **goal funding % -> `goals.RawPercent`** (commit 17d51993). `goals_row.go` + `chat_agent.go` computed
      un-clamped `current*100/target` inline (duplicating `goalsvc.Percent` minus its `[0,100]` clamp).
      Added pure `goals.RawPercent` + `TestRawPercent`; both call sites routed through it.
- [x] **major->minor money conversion -> `currency.MinorFromMajor`** (commit 17d51993). `chat_agent.go`
      re-derived float-major->int-minor in 11 places, **5 hardcoding `*100`** (a real bug for non-2-decimal
      currencies, e.g. JPY). Added pure helper (rounds via `Decimals`) + `TestMinorFromMajor`; routed all
      sites; deleted the file-local `majorToMinor`. **Fixed the JPY bug.**
- [x] **FX-aware goal totals -> `goals.Totals`** (commit 1d20640a). `goals.go` inlined a per-goal
      FX-convert-and-sum loop (raw-amount fallback) for the saved/target headline. Extracted to pure
      `goals.Totals(goals, rates, base, includeArchived)` + `TestTotals`.
- [x] **minor->major float conversion -> `currency.MajorFromMinor`** (commit 5e3e852b). `chat_agent.go`
      calculator var-map (6 vars, `/100`) and **two** `pow10` divisor loops (`divf`) in `planning.go`
      chart scaling. Added symmetric helper + `TestMajorFromMinor` (round-trip); removed both `divf` loops.

**TODO — remaining screens (next):**
- [ ] **allocate.go** — `allocate.go:303` reinvents `goalsvc.Remaining` (`max(0, target-current)`); swap
      to `goalsvc.Remaining(g).Amount` (clean, helper already exists + tested). *Identified, not yet applied.*
- [ ] **insights.go** — `topCat`/`topAmt` argmax over per-category FX-converted spend (L~85-95) is an
      analytical "top spending category" computation; consider moving to `spendsummary`/`insights`.
      Confirm whether an existing helper covers it before adding one. Large file — scan for other localized calcs.
- [ ] **budgets.go** — `totalSpent/totalLimit` summary loop over `budgeting.EvaluateRollup` statuses
      (L~244), incl. `limit = spent + remaining` re-derivation. LOW value (core rollup already in domain);
      optional `budgeting.SummarizeRollup` if a status `Limit` field is warranted instead of re-deriving.
- [ ] **accounts.go** — `monthStart` reinvents `dateutil.MonthStart` (L~161); trivial swap. Net-worth delta
      already via `ledger.NetWorthSeries`; `convBal` sort delegates to `ledger.Balance`/`Convert` (fine).
- [ ] Scan not-yet-deep-read screens for genuine computation (vs. display sums/sorts): **transactions.go,
      members.go, rules.go, categories.go, documents*.go, dashboard.go, smart*.go, activity.go,
      reports_screen.go**. NOTE: `reports_screen.go`, `dashboard.go`, `chartspec.go`, `health.go`/`healthscore`
      were under active edit by other sessions — re-check ownership before touching.

**TODO — internal/uistate (#95-128, not started):** sweep all 34 files. Expected low leak risk (mostly
atom/KV state plumbing), but check for any persisted-value computation or domain rules that belong in a
pure package.

**Residuals / notes:**
- `chat_agent.go:196` savings-rate `%` (`netFlow/income*100`) and `subscriptions_screen.go` FX-naive annual
  sum (`s.AnnualAmount()` summed without FX conversion) are lower-severity; log if revisited.
- Tracking task list (this session): tasks #1-128 mirror the file list; #1-24, #44, #64, #65, #71 are done.
- **Concurrency hazard:** an earlier in-flight rebase by another session once wiped an uncommitted edit;
  always verify `git status` is clean of a rebase/merge before editing, commit only own files by explicit
  path, and never revert/clobber another session's dirty files (e.g. TODOS.md/DEVLOG.md/CHANGELOG.md are
  frequently mid-edit).


<!-- ===== GRANULAR DECOMPOSITION (batches 9-15, folded 2026-06-25) ===== -->

# Granular todo decomposition — batch 9 (research, 2026-06-25)

> Produced by read-only research agents. To be folded into `TODOS.md` (before the
> `<!-- END-REVIEW-FINDINGS -->` anchor) once the in-progress `origin/main` merge is resolved
> and commits are unblocked. Research-lane output only — no code was written.

## F5 Quick-Add (#466 → atomic)

ALREADY SHIPPED by implementer agents (verify, then close — do NOT redo):
- **C40** Save & add another — DONE: `quickAddAnotherBtn` in `internal/app/quickadd.go` (data-testid `txn-add-another`) + `saveAndAnother`→`saveCore()`+`reset()`; i18n `quickAdd.saveAndAnother`.
- **C43** Amount autofocus — DONE: `Attr("autofocus","")` on amount Input; `flippanel.go` focuses the `[autofocus]` el on mount.
- **C44** One-click Quick-Add — DONE: addmenu/shortcuts/shell all call `quickAdd.Set(true)` directly (no intermediate menu).
- **C45** Account type cues — DONE: `quickAddTypeCue` appends " · Checking/Savings/…" to each option.

Remaining atomic todos:
- [ ] **[C41][MAJOR]** Replace inline default-account logic in `QuickAddHost` with `accountselect.DefaultID(accounts, app.Transactions(), activeMember)` — `internal/app/quickadd.go` (~l65-91) — adds the missing frequency-in-90d + checking-first tiers. Pure helper, safe to import.
- [ ] **[C41][MINOR]** Filter archived accounts out of the dropdown — `quickadd.go` `acctOpts` loop (~l171-181) — add `if a.Archived { continue }`.
- [ ] **[C42][MINOR]** Tab trapped in the native date picker — `quickadd.go` (~l263) — change `Type("date")`→`Type("text")` + `pattern \d{4}-\d{2}-\d{2}` + placeholder; value already ISO via `dateutil.FormatDate`; FlipPanel keydown then receives Tab cleanly.
- [ ] **[C39/C46][MAJOR]** Add a Payee field with recent-payee autocomplete — `quickadd.go` — `payee` UseState (BEFORE the open guard) + `FormField "Payee"` with `Input list="qa-payees"` + a `Datalist` populated from `quickpayee.RecentPayees(app.Transactions(),50)`; wire `Payee` into the `domain.Transaction` literal; add i18n `quickAdd.payee`. Pure helper, safe to import.
- [ ] **[C46][DESIGN]** `reset()` must also clear the new payee state (one-liner follow-on to C39/C46).
- [ ] **[C47][DESIGN]** Move the "reviewed — don't flag" checkbox below the Save button + mute it (`var --color-text-secondary`) — `quickadd.go` (~l265-271). Render-order + style only.
- Gotchas: new `UseState/UseEvent` hooks must precede the `if !open.Get()` guard; never use `On*` inside the `acctOpts` loop; confirm a `Datalist` shorthand exists else `El("datalist",…)`; `accountselect`/`quickpayee` are pure (no build constraint) so safe to import from the js/wasm `app` pkg.

## F33 Reports (#468 → atomic)

ALREADY SHIPPED:
- **C241** "Covering" ISO dates — DONE: `internal/screens/reports_screen.go` already routes cs/ce/ps/pe (and row dates) through `pr.FormatDate`.

Remaining atomic todos:
- [ ] **[C236][MAJOR]** Add "Print / Save as PDF" to the consolidated Export `<details>` — `reports_screen.go` (~l742-769) — an `opt` that calls `js.Global().Call("print")` (needs `syscall/js`); browser print = PDF, no library.
- [ ] **[C237][MAJOR]** YoY toggle — `reports_screen.go` (~l186-194) — `useYoY` UseState + `onToggleYoY`; prior window = `useYoY ? reports.YoYPrior(w).Range() : w.Shift(-1).Range()`; render toggle near the hero period label. Helper: `reports.YoYPrior` (already committed).
- [ ] **[C238][MAJOR]** Prior-zero delta badge — `reports_screen.go` `reportsCatRow` (~l1073-1086) — compute `ledger.Delta(amount,prior)`; show `d.Label()` for New/Gone/Pct, suppress only `DeltaZero`; root cause is `SpendingByCategory`/`ledger.PercentChange` returning `ok=false` when prior==0. Helpers: `ledger.Delta` + `.Label()` (already committed).
- [ ] **[C239][MINOR]** Bar chart `height="NaN"` on a zero-width domain — `web/chart.js` (~l107) — `var yMax=d3.max(ys); if(yMax===yMin) yMax=yMin+1;` before `scaleLinear().domain([yMin,yMax])`, so all-zero data → `[0,1]` not `[0,0]`.
- [ ] **[C240][MINOR]** Remove the 6 redundant per-card inline CSV buttons (category/payees/largest/income/members) — `reports_screen.go` (~l794-896) — keep only the consolidated Export panel; confirm every export stays reachable there.
- [ ] **[C242a][DESIGN]** Show Advanced/deductible even with no custom fields — `reports_screen.go` (~l932) — gate on `len(cfDefs)>0 || hasDeductibleCategories` (or always render; `deductibleSection` already returns `Fragment()` when empty).
- [ ] **[C242b][DESIGN]** Add custom-field + deductible exports to the consolidated Export panel — `reports_screen.go` (~l742-769) — hoist `cfRows`/`summary` compute so the top-level opts can call `downloadBytes`.
- [ ] **[C243][DESIGN]** Report-type selector — `reports_screen.go` (~l688) — `selectedReport` UseState ("overview"); segmented/`<select>` Overview/Spending/Income/Trends/Advanced; wrap each section group in `If(selectedReport==…)`; `OnChange` via a stable hook position.
# Granular todo decomposition — batch 10 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` once the in-progress `origin/main` merge
> resolves and commits unblock. No code written.

## R10 no-key receipt import (#441 → atomic)

ALREADY SHIPPED:
- **C94** Camera capture — DONE: `pickImageDataURL` sets `input.capture="environment"` (`internal/screens/documents.go:1086`); browser opens rear camera, no separate button needed.
- Pure helpers ready: `ai.EstimateCostUSD` + `ai.FormatCostUSD` (`internal/ai/ai.go:192,203`); `ai.Usage` already returned by vision callbacks (only the *display* wiring for C99 is missing).

Remaining atomic todos:
- [ ] **[C93][BLOCKER]** No-key manual fallback — `internal/screens/documents_image_import.go:32-85` — when `NeedsKey` && image chosen, add an "Enter manually" CTA → `uistate.RoutePath("/transactions")` (Tesseract.js local OCR is ~20MB, out of scope for one commit). New `OnManual ui.Handler` prop declared with `ui.UseEvent` at top of `Documents()` (unconditional), passed down.
- [ ] **[C95][MAJOR]** Swap key-check/image-check order in `readAI` — `documents.go:393-400` — image-empty guard must fire BEFORE the no-key guard so "Choose an image first" shows instead of the misleading needs-key notice.
- [ ] **[C96][MAJOR]** Unreadable-image error path — `documents.go:405-417` — distinguish 0-rows-parsed from API error; add i18n `documents.unreadableImage` ("couldn't read any transactions — try a clearer photo").
- [ ] **[C97][MAJOR]** Image size/format validation in `pickImageDataURL` onLoad — `documents.go:1094-1108` — reject >20MB + non jpeg/png/webp/gif via a new `onErr func(string)` param threaded into `chooseImage` (~l377); add 2 i18n keys.
- [ ] **[C98][MINOR]** Persist chosen image across the Settings round-trip — `documents.go:95` — `imageURL` is component-local `ui.UseState` (lost on nav); move to a `state.UseAtom("doc:pendingImageURL")` (or browserstore), clear on successful import (~l574/594).
- [ ] **[C99][MINOR]** Show token count + est. cost after vision call — `documents.go:404-417` — capture the `ai.Usage` (currently `_`), call `ai.EstimateCostUSD(aiModel,u)`+`FormatCostUSD`, set new `aiCostMsg` state, render muted line in `ImageImportCard` (`documents_image_import.go:53-85`); pattern at `insights.go:1077-1078`.
- [ ] **[C100][DESIGN]** Inline OpenAI-key explainer in `ImageImportCard` NeedsKey block — `documents_image_import.go:73-84` — what/where (platform.openai.com)/cost (~$0.002/receipt)/privacy (image goes browser→OpenAI, never to CashFlux); new i18n `documents.keyExplainer`.
- Gotchas: declare all new hooks at top of `Documents()` unconditionally; cost-estimation logic in the `onResult` closure (not the card); `onErr` is a pure-Go param (no alert()); use `state.UseAtom`/browserstore not `ui.UseState` for cross-nav persistence.

## F44 data ownership / backup (#469 → atomic)

ALREADY SHIPPED:
- **C298 (nav part)** Settings→Data jump-nav — DONE: `"settings.data"` in `settingsNavKeys` (`internal/app/settingssectionnav.go:34`).
- Pure helpers ready: `ExportJSONWithBlobs`/`ExportJSONRedactedWithBlobs`/`ImportJSONWithBlobs` (`internal/appstate/artifact_ops.go:110/122/136`); `recordBackupNow`/`loadLastBackup` (`internal/app/notifyrun.go:243/249`).

Remaining atomic todos:
- [ ] **[C294a][MAJOR]** `exportJSON()` → `app.ExportJSONWithBlobs()` — `internal/app/settings.go:1303` (callback, IDB-safe).
- [ ] **[C294b][MAJOR]** `activeDataset()` → `app.ExportJSONRedactedWithBlobs()` — `internal/app/backupall.go:55`.
- [ ] **[C294c][MAJOR]** `importJSON()` → `app.ImportJSONWithBlobs()` — `internal/app/settings.go:1360`.
- [ ] **[C295a][MAJOR]** Wrap `importJSON()` body in `confirmModal(...)` gated on ack — `settings.go:1354` — mirror `wipeData()` at l1386.
- [ ] **[C295b][MAJOR]** i18n `settings.importConfirm` ("Replace all current data with this file? This can't be undone.") — `internal/i18n/en.go` ~l1095.
- [ ] **[C296a][MINOR]** Add partial-CSV hint under the CSV export button — `internal/app/settings_section.go` ~l272 — muted `P` (pure helper, no hooks).
- [ ] **[C296b][MINOR]** i18n `settings.exportCSVHint` ("Exports your transactions only — not accounts, budgets, or attachments.") — en.go ~l1060.
- [ ] **[C297a-d][MINOR]** Surface "Back up everything"/"Restore" in Settings→Data — add `OnBackupEverything`/`OnRestoreBackup` to `settingsRightProps` (`settings_section.go` ~l124), 2 `dataBtn` calls (~l268-283), wire in `globalSettingsForm()` (`settings.go` ~l958-963), + 2 i18n keys.
- [ ] **[C298a-b][MINOR]** Destructive wipe-confirm label — `settings.go:1386` — add `ConfirmLabel` to the confirm-dialog request (check `internal/uistate` + `dialoghost.go:140`), pass `settings.wipeConfirmLabel` ("Erase data").
- [ ] **[C299a-d][MAJOR]** "Last backed up" timestamp — call `recordBackupNow()` at end of `backupEverything()` (`backupall.go:85`); add `LastBackupAt time.Time` to `settingsRightProps`; render muted `pr.FormatDate(p.LastBackupAt)` line (`settings_section.go` after l283); wire `LastBackupAt: loadLastBackup()` in `globalSettingsForm()` (`settings.go` ~l900).
- Gotchas: `*WithBlobs` variants block on IDB — only call from event/callback handlers (all listed sites are); `settingsRightColumn` is hook-free, derive values in `globalSettingsForm()` and pass via props.
# Granular todo decomposition — batch 11 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` at a checkpoint. No code written.

## F9 account types + net-worth clarity (#467 → atomic)

ALREADY SHIPPED:
- **C71 / C223** add-account persist — DONE: `accountaddform.go:182-186` calls `app.PutAccount` + `uistate.BumpDataRevision()` + `props.OnDone()`.
- `humanizeType` (`format.go:55-61`) title-cases any type label generically — new types render without code changes (except `retirement_ira` → "Retirement ira"; see C75).

Remaining atomic todos:
- [ ] **[C73/C75][MAJOR]** Add `TypeBrokerage`/`TypeRetirement401k`/`TypeRetirementIRA`/`TypeCrypto` consts + append to `AllAccountTypes` — `internal/domain/enums.go:34-52` — all default to `ClassAsset` (no `Class()` switch edit; `Valid()` iterates `AllAccountTypes`).
- [ ] **[C224][MAJOR]** Add `TypeProperty`/`TypeVehicle` consts likewise (same cluster) — `enums.go:34-52`.
- [ ] **[C73][MAJOR]** Update `domain_test.go:63,75` count + asset-class assertions for the new types.
- [ ] **[C73/C75][MAJOR]** `accountTypeIcon` switch — `internal/screens/accounts.go:429-441` — add icon cases for the new types.
- [ ] **[C73/C75][MAJOR]** Exclude new non-spending types from Quick-Add defaults — `internal/accountselect/accountselect.go:25` (`isSpendAccount`) + `internal/app/quickadd.go:82` — extend the `TypeInvestment` exclusion to the new investment/illiquid types.
- [ ] **[C73/C75][DESIGN]** `freshness.DefaultWindows` (`internal/freshness/freshness.go:31-43`) + `app/settings.go:448-458` `freshnessTypes` — add longer windows for the illiquid types (crypto ~14d, retirement ~90d, property/vehicle ~180d).
- [ ] **[C74][MINOR]** Promote lock-until out of Advanced for long-term asset types — `internal/screens/accountaddform.go:234-243` — add `isLongTermAsset(t)` helper; render lock-until when `!isLiab && (isLongTerm || advOpen)`.
- [ ] **[C72/C212][MAJOR]** Add `"kpi-assets"` bento renderer (uses already-computed `assets`, `dashboard.go:98`) — `internal/screens/dashboard.go:203-253` — + register in the default layout slice (uistate); add `assets.Amount` to `kpiSig` (C214).
- [ ] **[C75][DESIGN]** Group/label types in the add-form selector — `accountaddform.go:189-193` — add a `typeLabel(t)` lookup map (fixes "Retirement ira").
- [ ] **[C73][MINOR]** Update sample data to use the new types — `internal/store/sample.go:419-424` (401k/IRA/brokerage).
- Verify: `internal/ledger/liquid.go`, `runway/suggest.go`, `smartengine/accounts.go` liquid sets correctly EXCLUDE new types via default branch — confirm, do NOT add them.
- Gotchas: new hooks unconditional at top of form; strong-typed enum (add consts, don't loosen); `domain_test.go` count assertion is the build-time guard.

## F8 transfers (#472 → atomic)

ALREADY SHIPPED:
- `app.CreateTransferPair(TransferParams{...})` two-leg creation — `internal/appstate/transfer_ops.go:51`.
- Delete removes both legs — `appstate.go:1616` `DeleteTransactionWithTransferPair` + `isReciprocalTransferLeg`.
- "To account" selector exists in the row transfer form — `accounts_row.go:406-431`; `t.IsTransfer()` predicate available.

Remaining atomic todos:
- [ ] **[C67][MAJOR]** "New Transfer" primary action on `/transactions` toolbar opening a standalone `TransferFormModal` (new component, e.g. `internal/screens/transfer_form.go`) wired to `CreateTransferPair`; declare all hooks unconditionally.
- [ ] **[C68][MAJOR]** Guard `ActionFlagReview` against transfer legs — `internal/appstate/appstate.go` `case workflow.ActionFlagReview:` (~l1226) — add `if t.IsTransfer() { return }` (audit other applyEffect cases for the same).
- [ ] **[C69][MAJOR]** "From account" `<select>` in the new modal — exclude archived + the selected "To" account (mirror `accounts_row.go:406-431`); block submit if `fromID == toID`.
- [ ] **[C70][MAJOR]** Branch delete-confirm on `t.IsTransfer()` — `internal/screens/transactions_row.go:~64` — new i18n key `transactions.deleteTransferConfirm` ("Both sides of this transfer will be removed…").
- Gotchas: `CreateTransferPair` is non-atomic (documented) — surface partial-failure errors, don't swallow; logic stays out of view code; `ConfirmModal(msg, dangerous=true, cb)`.
# Granular todo decomposition — batch 12 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` at a checkpoint. No code written.
> Big theme this batch: many clusters are largely ALREADY SHIPPED by the implementer agents.

## R16 recurring & bills (#432 → atomic)
ALREADY SHIPPED: C155 — `bills_screen.go:163` uses `pr.FormatDate(upcoming[0].DueDate)`.
- [ ] [C147][MAJOR] Surface SMART-P1 detection card on bills screen + per-sub "Add to recurring" CTA — `bills_screen.go:100` (collect smart.PagePlanning) + `smartengine/planning.go:204` (action → "/recurring"); thread detected subs as structured payload (`planning.go:174`).
- [ ] [C148][MAJOR] Month prev/next nav — `bills_screen.go` add `calMonth` state + prev/next `UseEvent` (unconditional, ~after l51); pass to `bills.MonthCalendar()` (l215); header chevrons + `pr.FormatMonthYear(calMonth)` (helper at `prefs.go:227`).
- [ ] [C149][MAJOR] Next-due date field — `planning.go` add `rNextDue` state (~after l89) + `<input type=date>` (l584-590); parse into `NextDue` (replace `time.Now()` at l111); i18n `recurring.nextDuePlaceholder`.
- [ ] [C150][MAJOR] Enrich + click-through calendar dots — `bills_screen.go:243-249` — urgency tone + amounts in tooltip; extract `CalDotButton` component (hook inside it, NOT in the MapKeyed loop).
- [ ] [C151][MINOR] Exclude liability payments from subs — `subscriptions_screen.go:81-89` — filter via `subscriptions.IsLiabilityPayment(s, app.Transactions(), app.Accounts())` (`classify.go:51`).
- [ ] [C152][MINOR] Biweekly + semi-monthly cadence — `domain/entities.go:213-233` add consts + `Next()` cases + `MonthlyEquivalent()` (biweekly a*26/12, semi a*2); `planning.go:547-552` options + `cadenceLabel()` (l938-949); tests `domain_test.go`.
- [ ] [C153][MINOR] Inline edit recurring — `planning.go` add `editID` state + Edit btn on `RecurringRow` (hooks inside the component); submit via `PutRecurring` with same ID.
- [ ] [C154][MINOR] Persistent paid/autopay — new `recurring_occurrences` store table + `appstate.MarkOccurrencePaid` (reuse domain `IsPaid`/`MarkPaid` in `occurrence.go`); `Autopay bool` on `domain.Recurring`; paid indicator in `BillRow` (`bills_screen.go:279`).
- [ ] [C156][DESIGN] `/recurring` route — extract `Recurring()` into `internal/screens/recurring_screen.go`; register in `screens.All()` (`screens.go:74`) + shell nav (`shell.go:236-240`); replace planning card with a summary tile.

## R31 plans/pricing (#463 → atomic)
ALREADY SHIPPED: "Manage subscription"→Stripe portal (`settings_section.go:262`); trial note (`settings.cloudTrialNote`); annual/monthly price disclosure (Settings); UpgradeSheet trust line; SubscriptionBanner trial countdown; server-side trial-already-used guard (`billing_http.go:75`).
- [ ] [C301][CRITICAL] Decouple `ShowUpgradeSheet()` from CloudMention (only call site is `cloudmention.go:39`; once dismissed it's unreachable) — add a permanent "Try Cloud →"/Upgrade entry in sidebar (`shell.go`) + queue pending-open if called pre-mount (`upgradesheet.go:19-30`).
- [ ] [C300][MAJOR] Add `/plans` page (new `internal/app/plans.go`) reusing the Settings billing block + `startCheckout`/`openPortal`; "View plans & pricing" link in sidebar/Help; show both annual+monthly in UpgradeSheet (lift interval toggle from `settings.go:681`).
- [ ] [C302][MAJOR] Surface Manage/Cancel from `SubscriptionBanner` directly (deep-link to billing section; canceled-banner → checkout) — `subscriptionbanner.go:110-120`; add "canceling returns you to free local mode" copy.
- [ ] [C303][MAJOR] Plain-English free-vs-paid + trial in UpgradeSheet — add trial line + "Always free vs Cloud" comparison (`upgradesheet.go:37-74`); hint cost in CloudMention body.
- [ ] [C304][DESIGN] Split "Cloud & server" into "Connection" vs "Plan & billing" sub-sections w/ headings — `settings_section.go:194-265`; hint to switch to Cloud to see pricing when self-hosted.
- Gotcha: checkout/portal handlers close over endpoint/token (stale-snapshot risk) — pass as args or read fresh; `fetchBillingStatus` goroutine→UseState setter (confirm goroutine-safe).

## R28 alerts (#450 + #451 → atomic)
ALREADY SHIPPED (close as done): C263 (per-type settings UI `settings.go:95-160`), C264 (thresholds l208-260), C265 (paycheck `notify.go:44`+`notifyrun.go:331`), C266 (low-balance `notify.go:43`+`notifyrun.go:300`), C267 (severity pills `notifications.go:28`), C268 (read/dismiss/snooze `uistate/notifyfeed.go:101-156`), C269 (jump-nav `settingssectionnav.go:29`), C270. All have e2e tests.
- [ ] [#451][MAJOR] Add shared `OnTxnMutated func(*domain.Transaction)` seam on `App` — `appstate.go:69`; call at end of `PutTransaction` (l1554) guarded by `!triggersSuspended`; also fire on delete. (SHARED with #427 R13-reactivity — one field, two consumers.)
- [ ] [#451][MAJOR] New wasm-only `internal/app/livenotify.go` — `wireLiveNotify(app)` sets the hook; `runLiveNotifyFor(t)` runs only large/low-balance/paycheck/budget generators (skip time-based), config-gated via `notify.EnabledRules`, persists delivered log, prepends feed; recover() guard.
- [ ] [C272][MINOR] `runNotifyCatchUp` recover() → also `PostNotice(notify.catchUpError)` (`notifyrun.go:40-45`).
- [ ] [C271][MAJOR] "While you were away" digest grouping — `notifications.go:209-227` split `newSince` vs older into two `role=list` groups w/ headers (data already split at l159).
- [ ] [C268/snooze][MINOR] `pruneSnoozedFeed(now)` in `uistate/notifyfeed.go`; call from livenotify + NotificationCenter effect (l164).

## R29 roles (#462 → atomic)
ALREADY SHIPPED (close as done): C275 (role field in add `memberaddform.go:101-108` + edit `members.go:415-422`), C276 cosmetic badge (`members.go:432-441`), C274 disclosure note (`members.go:231`); full `internal/memberrole` pkg + tests; `domain.Member.Role` (`entities.go:50`); store round-trip; active-member switcher (`memberswitcher.go`).
- [ ] [C273][MAJOR] New `uistate.ActiveMemberRole()` helper (js&wasm) — resolves active member→role, `RoleOwner` when "Everyone".
- [ ] [C273][MAJOR] Gate Add/Delete/Make-default in Members on `CanManageMembers(role)` — `members.go:76-111,446-452` (derive once, pass `canManage` prop; no hook in loop).
- [ ] [C273][MAJOR] Gate write CTAs (Quick-Add/Add-menu/inline edit/delete) when Viewer (`CanViewOnly`) — add `uistate.IsViewerMode()`; wire in quickadd/addmenu/transactions/accounts/budgets/goals (one bool down).
- [ ] [C276][MINOR] Show role label in member switcher + txn member-filter options (`memberswitcher.go:52`, `transactions.go:745`).
- [ ] [C276][DESIGN] "Viewing as Viewer — read-only" banner in shell when CanViewOnly (overlaps C281).
- [ ] [cleanup] Remove orphaned i18n `members.roleMember`/`members.roleDefault`; seed `Role: RoleOwner` explicitly for default member (`sample.go`).
- Gotcha: local-first single-device → enforcement is SOFT UI only (no server auth).

## R33 a11y (#458 → atomic)
ALREADY SHIPPED (close as done): C318 radiogroup/role=radio/roving-tabindex (`ui/controls.go:131-190`) + server-mode/billing Segmented labels; most C315 aria-labels (rail-collapse, mobile +Add, NotifyBell, HelpButton, Muzak, offline, skip link, nav, breadcrumb, chart role=img); C317 `toggleTheme()` palette-wired + `/appearance` screen; C319 `DashboardLayoutControls` exists in Settings.
- [ ] [C315][MAJOR] aria-label on TopBar menu button (`shell.go:734`) + `aria-hidden` on brand "C" span (l502) + aria-label on HouseholdCard settings btn (l688-699).
- [ ] [C315][MINOR] i18n the chart default label `"Trend chart"` → `a11y.trendChart` (`ui/chart.go:56`).
- [ ] [C316][MAJOR] Sample-banner + subscription-banner text contrast — add `tw.TextFg` token to the text Span (`samplebanner.go:61`, `subscriptionbanner.go`).
- [ ] [C317][MAJOR] Visible theme-toggle button in TopBar controls (`shell.go:749-757`) calling `toggleTheme()` w/ Sun/Moon icon + aria-label.
- [ ] [C318][MINOR] Add `Label:` to remaining unlabeled Segmenteds: ResolutionControl (`shell.go:928`), week-start (`settings_section.go:162`), quickadd (`quickadd.go:246`).
- [ ] [C319][DESIGN] aria-label on layout-mode Select (`dashboard.go:1201`) + surface a layout/customize entry on the dashboard itself (not only Settings).

## R26 recommendations (#453 → atomic)
ALREADY SHIPPED (close as done): C256 executable actions (`smart_card.go:70-203`), C258a/b (SU1 same-page scroll, SU9 toast), C259b "enable free only" (`smart.go:277`), C259c per-rule cap (`smart/cap.go`). Settings KV persists across wipe by design.
- [ ] [C254][MAJOR] Verify `Settings{}.IsEnabled(free) == true` (add test); first-run auto-enable free via KV sentinel `cashflux:smart-first-run` in `SmartHub()` (~l39).
- [ ] [C255][MAJOR] Pre-init KV race — gate SmartHub/digest on `appstate.Default != nil` (already l28-29); add native tests for `LoadSmartSettings()` nil-app fallback + browser-store→SQLite migration on next get.
- [ ] [C257][MAJOR] Make /smart a ranked hub: relabel Insights tab "Recommendations" + subtitle; ensure `smart-digest` widget is in the DEFAULT bento layout (`dashboard.go:252` registered — add to default order in widgetcfg); `data-testid` on digest (l1378).
- [ ] [C259][DESIGN] Total cap (~25) before pagination (`smart.go:209`) + "Sorted by urgency" label.
- Gotcha: bulk-enable must bump `DataRevision` (SetSettingKV doesn't); digest widget hardcoded `GridRow 10` won't show unless in default layout list.
# Granular todo decomposition — batch 13 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` at a checkpoint. No code written.

## F49 sync reliability (#477) — ALL SHIPPED ✅ (close C320–C324)
- C320 backend gate: `sync_client.go:508-511` (`!BackendActive()`→empty) + `syncchip.go:61-63` (Fragment when not ok).
- C321 `data-testid="sync-chip"`: `syncchip.go:91`.
- C322 backoff: `sync_client.go:220-226` `backoff.Delay(attempt,2s,120s)`+`Jitter` (pkg `internal/backoff` tested).
- C323 offline handler: `sync_client.go:93-99` registers `"offline"` listener.
- C324 reactive: `syncchip.go:55-59` `state.UseAtom("sync:rev")` + `sync_client.go:489-495` bump in `setSyncStatus`.
→ No remaining todos. (Note: composes safely w/ R32 #464 conflict state — same `"conflict"` literal.)

## F41 per-member (#474 → atomic)
ALREADY SHIPPED: dashboard KPI member-filter (`dashboard.go:79-93` + `usePeriodTotals` memberSig); active-member infra (`uistate/activemember.go`, `memberswitcher.go`); pure `reports.SpendingByMember` (`internal/reports/members.go:26`, tested) + already called on reports screen (`reports_screen.go:496`); `ledger.NetByOwner` (binary owner) on Members.
- [ ] [C280][MINOR] Wire `reports.SpendingByMember` "spending this period" card onto /members — `members.go ~238` (use the period range; pure helper exists).
- [ ] [C277][MAJOR] Show member scope on /transactions count + extend KPI scope cues — `transactions.go:93` already layers `TxFilter.Member`; add a visible "showing <member>" count.
- [ ] [C278][MAJOR] Scope accounts/budgets/goals/allocate by active member (none call `UseActiveMember` today — `accounts.go`/`budgets.go`/`goals.go`/`allocate.go` confirmed absent) — add the member filter where meaningful (or document why net-worth stays household).
- [ ] [C279][MAJOR] Fractional ownership (pure first): `domain.Account.AllocationShares []MemberShare{MemberID,Weight}` (`entities.go`); `ledger.NetByOwner` (l240) distributes via `split.ByWeights` when shares set; new `allocate/membersplit.go SplitPeriodIncome` (compose `PeriodIncome`+`split.ByWeights`); then add-form shares sub-form.
- [ ] [C281][DESIGN] "Viewing as <member>" banner — new shell component reading `UseActiveMember` (OVERLAPS R29 C276 role banner + MIA scope banner — build ONE shared banner).

## F43 privacy/trust (#475 → atomic)
ALREADY SHIPPED: C289 trust footer (`shell.go:704`, `trust.localFooter`).
- [ ] [C291][CRITICAL] Fix inaccurate "end-to-end encrypted" copy — `i18n/en.go:966` `cloud.upgradeTrust` says E2E but sync sends raw JSON; change to "encrypted in transit" to match accurate `settings.cloudTrustLine` (en.go:1011). Consumer `upgradesheet.go:64`.
- [ ] [C291][MAJOR] "What syncs" disclosure under backend toggle (names categories + HTTPS), visible whenever backend on (not gated on CloudSelected) — `settings_section.go:194-216`; i18n `settings.syncDisclosure`.
- [ ] [C292][MAJOR] Persistent AI-key privacy note (remove empty-key gate `settings_section.go:182`) + show key-storage disclosure regardless of CloudSelected; extract shared `KeyExplainerNote()`.
- [ ] [C290][MAJOR] `/about` route + `internal/screens/about.go` (version, local-first statement, MIT, links) + footer link in HouseholdCard (`shell.go:697-706`) + jump-nav.
- [ ] [C293][MEDIUM] Expand the settings `about` div (`settings.go:1024-1028`): privacy line + MIT + /help link; later collapse to "More about CashFlux →".

## R25 anomaly hub (#454 → atomic)
ALREADY SHIPPED: `insights.Detect` + `detectSpendingAnomalies` (`insights.go:1323`) shared by /insights + dashboard; SMART A1/T6/T7/T2 engines exist+tested; reports anomaly card.
- [ ] [C252][CRITICAL] Audit: make A1/T6/T7 engine fns callable directly (export or add `smartengine.RunAnomaly(in) []smart.Insight`) — `engine.go:101-128` has no allowlist.
- [ ] [C252][CRITICAL] NEW `internal/screens/anomaly_helpers.go` `detectAllAnomalies(app,txns,cats,rates)` — union category-anomalies + A1/T6/T7 (converted), category-dedup, mid-month-zero guard, sort by |Δ|, cap 5. Verify no import cycle (smartengine must not import screens) via native `go build` first.
- [ ] [C252][MAJOR] `smartInsightToAnomaly` converter (read `smart.Insight` fields first).
- [ ] [C252][MAJOR] Wire `detectAllAnomalies` into `spendingHighlights` (`insights.go:1297`), `topHighlightWidget` (`dashboard.go:585`), `attentionWidget` (`dashboard.go:1250`) — pass `app`.
- [ ] [C253][MAJOR] Rename card "Spending Highlights"→"Anomalies" (`insights.highlightsTitle`). COORD F32 #471 (same card) + R24 #455 (same file) + mid-month guard shared w/ F32-C232.

## R20 sinking funds UI (#436 → atomic)
ALREADY SHIPPED (pure math): `goals.DrawDownFund`/`FundSetAsideMinor` (`goals/sinkingfund.go`), `budgeting.SinkingFund*` (`rollover.go:40-88`), SMART-BL9 detector (`smartengine/bills.go:578`).
- [ ] [C189][CRITICAL] `domain.Goal.IsSinkingFund bool` + `CategoryID string` (omitempty) — `entities.go:391` (no migration); persist through `saveGoal` (`goals.go:103-146`).
- [ ] [C189/C192][HIGH] IsSinkingFund toggle + (conditional) category selector in add form (`goaladdform.go:85-168`) + inline edit (`goals_row.go:138-165`).
- [ ] [C190][CRITICAL] Wire `FundSetAsideMinor` onto goal rows ("Set aside $X/mo") + aggregate stat card (`goals.go:207-213`).
- [ ] [C191][HIGH] Auto-accrual: appstate side-effect on txn save where `CategoryID` matches a fund → `DrawDownFund`+`PutGoal` (one top-level effect, iterate inside); monthly set-aside credit w/ once-per-month guard (`LastAccruedMonth`).
- [ ] [C193][HIGH] BL9 action → `ActionCreateGoal` prefilled IsSinkingFund (`bills.go:578`) + "Suggested sinking funds" strip on /goals.
- [ ] [C194][HIGH] 3-way goals partition (funds/active/achieved) + dedicated "Sinking Funds" section + Funds filter tab (`goals.go:184-293`).

## R4 FX UX (#447 → atomic)
ALREADY SHIPPED: C85 symbols `CA$`/`A$`/`MX$` (`currency.go:39-44`, all sites via `Symbol()`); C81 inverse hint after rate entered (`settings.go:1103`).
- [ ] [C78][MAJOR] Remove `singleCurrency` gate; always show currency picker (defaults to base) — `accountaddform.go:54-64,214`.
- [ ] [C78b/C79][MAJOR] Inline "set rate" affordance + add-time rate-missing notice when non-base currency w/ no FX rate — `accountaddform.go:113,221`.
- [ ] [C80][MINOR] Render `FXUpdatedAt[code]` date beside staleness badge — `settings.go:1083-1120` (map already persisted).
- [ ] [C81][MINOR] Static convention explainer above FX list (before any rate entered).
- [ ] [C82][MINOR] Net-worth conversion disclosure line when rates applied — `accounts.go:316` (may need `ConvertedCurrencies` on `NetWorthExplained`).
- [ ] [C85][DESIGN] Fix `currency.Symbol()=="$"` branch checks (`custompage.go:534`, `dashboard.go:717`, `planning.go:298,771`) — CAD/AUD/MXN miss the prefixed chart format; add `currency.IsDollarVariant`.
- [ ] [C84][DESIGN] Fix 3 dead `Navigate("/settings")` calls (`allocate.go:169`, `insights.go:136`, `documents_image_import.go:77`) → `settings.Set(uistate.Global())`; + clickable "Settings" link in accounts exclusion notice.
- [ ] [C83] TRIAGE — investigated, NO fix required (skip-link `.skip-link` vs add-menu `.add-item` are distinct classes; no collision confirmed). Close as not-a-bug.
# Granular todo decomposition — batch 14 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` at a checkpoint. No code written.

## R17 planning surfacing (#430 → atomic)
Reuse committed: `runway.ProjectLiquid`/`NextPaydayHorizon`, `cashflow.DipDate`/`PaydayBalance`, `ledger.LiquidBalance` (none called from screen yet).
- [ ] [C171][MAJOR] Seed runway from `ledger.LiquidBalance` not `assets.Amount` — `planning.go:469,478,524` (use `runway.ProjectLiquid`). (do first)
- [ ] [C168][MAJOR] Lead /planning with the liquid projection card; demote 12-mo net-worth chart — `planning.go:366-401`.
- [ ] [C172][MAJOR] Visualize `proj.Daily` as balance-over-time chart (template: forecastCard `toPoints` l290-296) — `planning.go:522-530`.
- [ ] [C169][MAJOR] Payday anchor tile via `runway.NextPaydayHorizon` — needs `Settings.PayCycleDay int` added (additive, ahead of R14) — `planning.go:465-542`.
- [ ] [C170][MAJOR] Dip warning + projected-on-payday balance via `cashflow.DipDate`/`PaydayBalance` — `planning.go:476-531`.
- [ ] [C173][MINOR] Low-balance date → stat tile (not muted footnote) — `planning.go:528`.
- [ ] [C174][MINOR] Runway empty-state → `EmptyStateCTA` to add recurring — `planning.go:476-477`.
- [ ] [C175][DESIGN] Add data-basis disclosure notes to afford + runway cards — `planning.go:385/407/465`.

## R12 budgets UI (#426 → atomic)
ALREADY DONE: `/budgets` route exists (404 is dev-server only = C115); `IncomeForBudgets`/`Generate5030`/`Classify` ready; `EmptyStateCTA` on empty budgets.
- [ ] [C118][HIGH] Add `Budget.Methodology string` to `entities.go:365-375` (BLOCKING prereq — R12-foundation #425) + methodology select in add form (`budgetaddform.go`) + edit (`budgets_row.go`) + thread `budgetRowProps` (`budgets.go:369`).
- [ ] [C114][HIGH] "Use 50/30/20 template" button → `Generate5030(IncomeForBudgets,...)` fan-out to CreateBudget — `budgets.go:275-295`.
- [ ] [C113][HIGH] Implement envelope mode (assign banner action + cover/top-up reach a store write + "available to assign" total) — `budgets.go`/`budgets_row.go`.
- [ ] [C112][HIGH] Zero-based empty-state CTA + always-visible Add button — `budgets.go:275-320`.
- [ ] [C119][HIGH] Income context bar (income/budgeted/remaining via `IncomeForBudgets`) + "remaining to budget" hint in add form — `budgets.go`.
- [ ] [C117][MED] Wrap rollover checkbox+label in flex `Label` (detaches at 1280px) — `budgetaddform.go:169`, `budgets_row.go:155`.
- [ ] [C115][MED] Dev-server SPA history fallback (mirror `e2e/serve.go:72`) — find dev server entry.
- [ ] [C116][MED] Audit `periodOptions()` for shared backing array / missing i18n — `budgets.go:428-434`.

## R21 loan amortization UI (#418 → atomic)
ALREADY DONE: `payoff.Amortize*` engine committed+tested; `domain.Account` has APR/MinPayment/DueDay/Lender/CreditLimit; installment vs revolving distinguished at type level (`enums.go:41-74`).
- [ ] [C204][MAJOR] Add `TermMonths int` + `OriginationDate time.Time` to `domain.Account` (`entities.go:91`) + `IsInstallment()` helper + `payoff.RemainingMonths()` helper. (BLOCKING prereq)
- [ ] [C206][MAJOR] Persist new fields (store JSON round-trip + test) + fix sample loans (`sample.go:428-430`: set TermMonths/OriginationDate; mortgage 360).
- [ ] [C204][MAJOR] Term fields in add form (`accountaddform.go`, gated `isLiab && IsInstallment`) + inline edit (`accounts_row.go`).
- [ ] [C204/C205][MAJOR] NEW `internal/screens/loan_amort_panel.go` `LoanAmortPanel` — `AmortizeFixed`/`AmortSummary` schedule table (Map, no On* in loop) + extra-payment simulator (`AmortizeWithExtra`) callout; wire into AccountRow read-only branch for installment liabilities. (negate signed ledger balance before AmortizeFixed)
- [ ] [C207][DESIGN] "Installment"/"Revolving" badge in account meta (`accounts.go:446`) + fix `TypeLineOfCredit` icon→CreditCard (`accounts.go:429`).

## R23 portfolio UI (#420 → atomic) — BLOCKED on foundation
BLOCKER: R23-foundation (#419) NOT landed — `domain.Holding` type, `holdings` table, store CRUD, dataset round-trip, appstate accessors all MISSING. portfolio calc pkg (PortfolioSummary/Allocation*) IS committed+tested.
- [ ] [C219][CRITICAL prereq] domain.Holding (`entities.go`) + `holdings` table (`sqlitestore.go:55`) + store CRUD (`crud.go`) + Dataset wiring (`dataset.go:85`) + appstate accessors + sample holdings (2+ asset classes).
- [ ] [C219][CRITICAL] NEW `internal/screens/investment_holdings.go` `InvestmentHoldingsPanel`+`HoldingRow` (own component, hooks unconditional) — table + add form; wire into AccountRow for TypeInvestment.
- [ ] [C220][NORMAL] Performance summary via `portfolio.PortfolioSummary`; override displayed balance for investment accts (display-only `PortfolioValueMinor` prop).
- [ ] [C221][NORMAL] Asset-class breakdown bars via `AllocationByAssetClass` + by-holding toggle. (Note: `/allocate` is NOT mislabeled — it's capital allocation, a different feature; no rename.)
- [ ] [C222][NORMAL] Suppress STALE nudge for investment accts with holdings (`accounts_row.go:527` add `!HasHoldings`). Freshness window already 60d.

## R5 setup wizard (#449 → atomic)
ALREADY DONE: C24 date-format (`prefs.DateStyle`+`settings_section.go:168`+`FormatDate`) — close; C29 budget empty-state real (`budgets.go:299`) — dev-server issue not code; `internal/setup` pure logic fully landed.
- [ ] [C21/C23][MAJOR] Add `WizardShownOnce`/`WizardDismissed`/`SetupCurrencyConfirmed bool` to `store.Settings` (`dataset.go:44`) — BLOCKING (R5-foundation #448 referenced but unread).
- [ ] [C30][MINOR] Owner default = sole member when 1 member (else group) — `accountaddform.go:70` (compute before UseState).
- [ ] [C26][MAJOR] Demote "Load sample" to outline; promote "Add first account" primary — `accounts.go:293-299`.
- [ ] [C27][MINOR] Opening-balance sign-convention hint + i18n `accounts.openingBalanceHint` — `accountaddform.go:~175`.
- [ ] [C21][MAJOR] NEW `internal/app/wizardhost.go` `WizardHost` (dialog overlay, ESC=skip, Back/Next/Skip/Done, sets WizardShownOnce) + uistate UseWizardOpen/Step atoms; render unconditionally in shell.
- [ ] [C23/C22/C21/C28][MAJOR] Wizard steps: currency+week-start (extract shared controls to avoid R4/R14 conflict), income (skip-gate until R12 income field), account (embed AccountAddForm), members (embed MemberAddForm, "skip — only me").
- [ ] [C21][MAJOR] First-run trigger in shell (post-hydrate `setup.IsFirstRun`; do NOT fire if sample auto-seeded).
- [ ] [C31][DESIGN] Wire `dashboard_onboard.go:51` checklist to `setup.Compute`/`NextIncompleteStep` + "Continue setup" → WizardOpen.

## F26 debt planner (#470 → atomic)
Reuse committed: `payoff.AggregateDebts` (FX-correct), `payoff.Compare` — NEITHER called from planning today (manual native-currency loop at `planning.go:654-672` = the C195 bug). C202 partial (explain text + Try button exist).
- [ ] [C195][MAJOR] Replace manual debt loop with `payoff.AggregateDebts(accounts,txns,base,rates)` + surface missingRates warning — `planning.go:654`.
- [ ] [C196][MAJOR] Per-debt table (Name/Balance-FX/APR/MinPayment) — `planning.go:724` after toggles.
- [ ] [C197][MAJOR] Call `payoff.Compare(snow,aval)` → "avalanche saves N months · $X interest" — `planning.go:733`.
- [ ] [C199][MINOR] Snowball overlay series in burn chart + legend — `planning.go:760`.
- [ ] [C203][DESIGN] Calendar date labels on burn-down points via `payoff.DebtFreeMonth` (mirror forecast l307) — `planning.go:765`.
- [ ] [C201][MINOR] Editable APR/MinPayment per debt row (own `DebtRow` component, PutAccount on change) — `planning.go`.
- [ ] [C200][MINOR] `/debt` route extracting the debt card (new `screens.Debt()`) + nav anchor — `screens.go:68`, `shell.go:236`.
- [ ] [C202][DESIGN] Reorder tie-state: show explain+Try before/instead of tied stat-grid — `planning.go:724`.
- [ ] [C198][MAJOR] After C195, recompute baseline from FX-correct debts + "reset & re-snapshot" nudge + verify `PayoffProgress` currency passthrough — `planning.go:677-696`.
# Granular todo decomposition — batch 15 (research, 2026-06-25)

## R15 safe-to-spend surfacing (#422 → atomic)
ALREADY DONE: `safespend.Compute`/`ComputeCategory` committed+tested (R15-foundation #421 shipped); C124 plain-English "$X over" (`budgets.go:409-414`).
- [ ] [C139][MAJOR] "Safe to spend" KPI tile on dashboard bento (`dashboard.go:203-253`) via `safespend.Compute(liquid, billsLeft, goalNeeds, committedBudgets, base)` (liquid from `ledger.LiquidBalance`); register tile in dashlayout.
- [ ] [C140][MAJOR] Render that tile UNCONDITIONALLY (no Smart gate); SMART-B8 stays advisory only.
- [ ] [C141][MAJOR] Planning "Free to spend" → use `ledger.LiquidBalance` not NetWorth + `safespend.Compute` formula — `planning.go:412,415,424`.
- [ ] [C142][MAJOR] Normalize terminology to "Safe to spend" (`i18n en.go:638`) + align SMART-B8 3-bucket formula to `safespend.Compute` 4-bucket (`smartengine/budgets.go:101`).
- [ ] [C143][MAJOR] Per-budget prorated pace sub-line via `safespend.ComputeCategory(remaining, daysLeft, daysInPeriod)` — `budgets_row.go`.
- [ ] [C144][MAJOR] Negative "Left" tile context sub-line (largest offender) — `budgets.go:329-337`.
- [ ] [C146][MINOR] $1 floor must not gate the dashboard tile (compute directly, no floor); shares formula-align fix w/ C142.

## R13 budget reactivity + polish (#427/#428 → atomic)
ALREADY DONE: C124 plain-English over-text (`budgets.go:409-414`); C125 static over-budget banner (`budgets.go:353-358`).
- [ ] [C120][HIGH] Budgets must subscribe to global `uistate.UseDataRevision()` (currently only `rev:budgets`) — add `_ = uistate.UseDataRevision().Get()` at `budgets.go:39`. (one line)
- [ ] [C122][HIGH] Add shared `App.OnTxnMutated func()` seam (`appstate.go:~63`), fire in PutTransaction (~l1555, both add+edit) + DeleteTransaction — SHARED with R28 #451 (build once, fan-out if 2 consumers).
- [ ] [C122][HIGH] NEW pure `internal/app/budgetdiff.go` `NewlyOverBudget(before,after []budgeting.Status) []string` (+ native tests).
- [ ] [C122][HIGH] NEW `internal/app/livenotify.go` (js&wasm) `wireOnTxnMutated` → snapshot before/after, toast newly-crossed, seed dedupe from delivered log; call from `app.go:~187`.
- [ ] [C123][MED] Quick-Add dialog clip — `quickadd.go:274` Height 420→520px + `.set-foot{flex-shrink:0}` (`web/index.html:2021`).
- [ ] [C125][MED] Navigate-in over-budget toast via `ui.UseEffect` keyed `"over:N"` — `budgets.go:~266`.

## R19 savings automations (#434 → atomic)
ALREADY DONE: C186 `workflow.ActionTransfer` (+ValidateTransferAction, executed via CreateTransferPair); C185 `CreateWorkflowFromGoal` (pay-yourself-first two-leg); C187 SMART-G17 executable (`smart_card.go:191-202`). Reuse `savings.RoundUpDelta`/`SurplusMinor`/`IsScheduleDue`/`PeriodKey`.
- [ ] [C183][MAJOR] Round-up: txnContext vars (`txn_is_transfer`/`txn_amount_minor_local`) + `RoundUpAccrual` store + `AccumulateRoundUp` on TxnAdded (base-ccy guard) + `CreateWorkflowFromRoundUp` template (DedupeKey `roundup:` resolves live accrual, resets to 0).
- [ ] [C184][MAJOR] Surplus sweep: `surplus_minor` in engineVars (`savings.SurplusMinor`) + `TransferAmountExpr` field on Action/Effect (formula-eval at apply) + `CreateWorkflowFromSweep` (cap to max).
- [ ] [C188][DESIGN] NEW `internal/screens/automations.go` `Automations()` (group transfer workflows by DedupeKey prefix, enable toggles, "transferred this period") + `/automations` route (`screens.go`) + i18n.
- Dep order: C183.1→.2→.3→.4; C184.1→.2→.3; then C188.

## R7 self-learning categorization (#437/#438 → atomic)
ALREADY DONE: **C32 fixed** (ruleaddform reads `UseRuleDraft` via UseEffect `rule-draft-consume`, `ruleaddform.go:62-70`) — close #437. Reuse `learntally` pkg.
- [ ] [C33][MAJOR] Self-learning: `App.tally learntally.Tally` field, warm from history on boot + `LoadTally`/`SaveTally` (KV `app:learntally`); `Increment` on PutTransaction (categorized, non-transfer); wire `ShouldSuggest` into `AutoCategorizeTransaction` as 2nd signal.
- [ ] [C34][MAJOR] Live Quick-Add suggestion: extend `catAssist` (`quickadd.go:214-226`) — after rules lookup, consult `app.TallySuggest(desc)` then `statement.DefaultCategorizer` (3-tier); pure render-time, no hooks.
- [ ] [C35][MAJOR] Threshold: replace literal `3` (`rules.go:167`) with `app.RuleSuggestMinCount()` (new persisted setting, default `rulesuggest.DefaultMinCount`) + numeric input in settings.
- [ ] [C36][MINOR] Wire keyword categorizer into `AutoCategorizeTransaction` (name→ID match) — `appstate.go:754` (covers Quick-Add + imports).
- [ ] [C37][MINOR] Visible label on create-rule funnel button + i18n `transactions.createRuleLabel` "Always categorize like this" — `transactions_row.go:251`.
- [ ] [C38][DESIGN] Move suggestions above the Mermaid order card + empty-state "keep categorizing…" — `rules.go:199-241`.

## R8 duplicate review/merge UI (#440 → atomic)
ALREADY DONE: C90 dedupe count filter-scoped (`transactions.go:70-73,490-494`); C91 selection-count toast (`transactions.go:356-362`). Reuse `fingerprint.TxFingerprint`/`GroupDuplicates`/`MergeResolve`.
- [ ] [C86][BLOCKER] Upgrade CSV-import dedup key from `dedupe.Signature` → `fingerprint.TxFingerprint` (account-scoped + POS-noise) — `appstate.go:208-223` + doc-import path ~l800; regression test (`# STARBUCKS` vs `STARBUCKS` re-import = 0).
- [ ] [C87][MAJOR] `App.MergeTransactions(keepID, discardIDs)` via `fingerprint.MergeResolve` under triggersSuspended — `appstate.go:~243` (+ test). Hard dep of C88/C89.
- [ ] [C88][MAJOR] Pre-import dup-warning stage: `PartitionCSV(...)→(fresh,candidates,skipped)` + inline warning card w/ per-row skip/import — `appstate.go:~171`, `documents.go:144-193`.
- [ ] [C89][MAJOR] `/duplicates` screen (new `duplicates.go` + route): `GroupDuplicates` → side-by-side cards + Keep-first/newest/most-detail → `MergeTransactions`; collect IDs then call once (no On* in loop).

## R30 applock security (#460 → atomic)
ALREADY DONE: `PasscodeStrength`/`isTrivialPasscode`/strength enum (`applock.go:116-222`); MinPasscodeLength=4 + StrengthTooShort reject; ValidHint guard.
- [ ] [C284][MAJOR] Replace SHA-256 gate hash with argon2id (`golang.org/x/crypto/argon2`, IDKey 3/64MB/4) + `argon2id$params$salt$hash` format + HashVersion + lazy re-hash migration on SHA-256 verify — `applock.go:58-104`; run in goroutine/promise (CPU-heavy) so unlock doesn't freeze.
- [ ] [C285][MAJOR] Add `"applock.section"` to `settingsNavKeys` (`settingssectionnav.go:22-36`).
- [ ] [C286][MINOR] Dark-mode gate card: bg `var(--surface,#fff)` undefined in dark → change to `var(--bg-card,#121214)` + explicit text color — `applockgate.go:125`.
- [ ] [C287][MINOR] Reject `StrengthWeak` (e.g. "000000") in setup `submit()` (`applockgate.go:419`) + i18n `applock.tooWeak` + live strength meter.
- [ ] [C288][DESIGN] Rename "App lock" heading → "Security" (`i18n en.go:357`) (+ optional `/security` route).

<!-- ===== GRANULAR DECOMPOSITION (batch 16, appended 2026-06-25) ===== -->

# Granular todo decomposition — batch 16 (research, 2026-06-25)

## R14 pay-cycle periods (#423/#424 -> atomic)
Reuse: dateutil.FiscalMonthRange (anchor math exists), runway.NextPaydayHorizon. Coordinate Settings.PayCycleDay with R17 #430.
- [ ] [C126][MAJOR] PeriodBiweekly const (enums.go:130-136) + 14-day bucket range in budgeting.PeriodRange (UTC-midnight, DST-safe) + tests.
- [ ] [C127][MAJOR] PeriodSemiMonthly const + 1st/15th range (dateutil.MonthStart/AddMonths) + tests.
- [ ] [C129][MAJOR] PeriodYearly const + year range; UI auto-wires via AllPeriods (budgets.go:427).
- [ ] [C128][MAJOR] PayCycleDay int on store.Settings + appstate accessor + PeriodPayCycle const + pure PayCycleRange(ref,day) + NEW PeriodRangeAnchored(p,ref,weekStart,payCycleDay) wrapper (keep 3-arg PeriodRange); thread payCycleDay at call sites (appstate.go:1418/1437, notifyrun.go:381, smartengine/budgets.go:133, budgets.go, health.go:102, envelope.go, rollover.go); pay-cycle settings card (settings_section.go) + handler; guard PeriodPayCycle option off when day==0.
- [ ] [C131][MINOR] Add 5 missing weekday consts + Normalize + WeekStartWeekday (prefs.go:18-21,138,235); thread week-start through hardcoded time.Monday (appstate.go:1418); settings 2-option Segmented -> 7-option SelectInput.
- [ ] [C130][MINOR] Helper text under period select clarifying "tracking period is not the dashboard view window" (budgetaddform.go:159).

## R32 sync/PWA (#464/#465 -> atomic)
ALREADY DONE: C306 PWA icons+iOS meta (manifest.webmanifest:12-16, index.html:66-74); C307 install button (index.html:2843-2872); C309 conflict backup/restore/discard cycle (sync_client.go:186-204,383-434 + settings UI); F49 C320-324.
- [ ] [C307][MINOR] iOS "Add to Home Screen" hint banner (no beforeinstallprompt on iOS) - index.html after install IIFE; gate on iOS+!standalone; localStorage dismiss.
- [ ] [C309][MAJOR] Force-push on restore: add Force bool to queuedSyncMutation, set in restoreConflictBackup (sync_client.go:416), pass as PutWorkspaceRequest{Force} (l168) - server already accepts force (proto field 4); without it the re-stamped item re-loses LWW.
- [ ] [C309][MAJOR] Store server UpdatedAt/Version from conflict response (already returned, ignored at sync_client.go:186) + show "server copy is X newer" in restore card (settings_section.go:233).
- [ ] [C309][MAJOR] Conflict chip -> open Settings on Cloud section directly (syncchip.go, conflict state) instead of generic global.
- [ ] [C310][MAJOR] Connected-devices list: DevicesList component (endpoint GET /v1/auth/sessions exists) - verify if stubbed at settings_section.go:245; row=own component (revoke button); DELETE /v1/auth/sessions/{family}.
- [ ] [C310][DESIGN] "Pair new device" flow: pairing_tokens table + POST /v1/auth/pair + redeem + new-device first-run prompt (sequence after devices list).
- [ ] [C308][DESIGN] Native app OUT OF SCOPE - doc note only (PWA is the path; Capacitor+60MB wasm is months).

## F32 trends/insights (#471 -> atomic)
Reuse: reports.PayeeTrends, reports.CategoryTrends (DeltaPct/HasDelta), ui.AreaChart, insights.Anomaly.Delta (computed, unrendered). COORD R25 #454 + R24 #455 (same file/shared detectSpendingAnomalies/highlightText).
- [ ] [C230][HIGH] categoryTrendChart() via reports.CategoryTrends (top 3) multi-line sparklines + delta badge - insights.go; place first.
- [ ] [C232][HIGH] Mid-month-zero guard: Options.MinDaysElapsed (default 7) in insights.Detect - skip current bucket when Current==0 and <7 days elapsed (or prorate). Shared w/ R25-C232.
- [ ] [C228][MED] Drill-through: add CategoryID to insights.Anomaly (l44) + populate in Detect; anomaly row = own component w/ OnClick -> set txFilter + nav /transactions (mirror reports_screen.go:145). (no On* in loop)
- [ ] [C229][MED] merchantTrendsCard() via reports.PayeeTrends(txns,bounds,rates,5) sparklines - insights.go.
- [ ] [C233][LOW-MED] Render dollar delta: pass a.Delta+rates to highlightText (l1352) + i18n with (+/-$X).
- [ ] [C231][MED] Starter chips: change guard to len(turns.Get())==0 (auto-resume makes convo non-empty) - insights.go:804; reset turns on New Chat.
- [ ] [C234][MED] "Ask AI" above the fold: anchor button or reorder composer above thread + placeholder.
- [ ] [C235][LOW] Source string on domain.SavedInsight (set "AI" in pinText l178) + render "via AI" in PinnedInsightRow (l1245).

## F6 ledger filters (#473 -> atomic) - MOSTLY SHIPPED
ALREADY DONE: C48 tags in inline edit (transactions_row.go:101,176); C49 tag filter end-to-end (txnfilter.go:281,391); C51 conditional Clear (transactions.go:817); C53 amount min/max filter (txnfilter.go:86,372); C54 tags-empty over full shown set (transactions.go:563-570).
- [ ] [C52][MED] Filter panel occludes table: compose a non-backdrop inline/drawer panel at filtertoolbar.go:93-99 (add Inline prop; do NOT alter shared FlipPanel internals).
- [ ] [C56][MED] Keyboard shortcut (Alt+F) to open filter panel via UseEffect keydown (filtertoolbar.go:55-101) + aria-keyshortcuts.
- [ ] [C57][LOW] SR count: add tw.SrOnly span ("N active filters") sibling to the aria-hidden badge (filtertoolbar.go:60-65).

## C58/F7 split transactions + bulk (#415/#416 -> atomic)
ALREADY DONE: C62 range/shift-select (transactions.go:304-344); C63 bulk export uses selection (transactions.go:175-196); C64 mark-uncleared bulk (transactions.go:394-426); C58 split ATTRIBUTION logic in budgets+reports (budgeting.go:90-99, reports.go:46-55) - logic layer done, UI missing.
- [ ] [C58][BLOCKER] Split editor UI: "Split (N)" badge (transactions_row.go:193); OnOpenSplitEditor prop; NEW transactions_split_editor.go with SplitEditorRow (own component - hooks in loop), "Add split", running total vs txn total, domain.SplitsReconcile guard; editTxnSplits->PutTransaction; splitEditorTxn state in screen.
- [ ] [C60][MAJOR] Payee field in inline edit: payeeS+onPayee hooks, seed in startEdit, Input after Description, extend OnSave signature + editTxn to apply orig.Payee (transactions_row.go + transactions.go:254,581) + i18n.
- [ ] [C61][MAJOR] Escape cancels inline edit: escEdit UseEvent (key==Escape -> editing.Set(false)) on the edit Form OnKeyDown (transactions_row.go:162).
- [ ] [C65][MINOR] aria-labels on inline desc/amount/payee inputs (transactions_row.go:163-167).
- [ ] [C66][DESIGN] Rename settle-up nav string nav.split value -> "Settle up" (i18n en.go:140) + subtitle; keep route/key.

## R24 chat UX (#456 -> atomic)
ALREADY DONE: per-bubble token+cost ("Used N tokens ~$X", insights.go:1074-1081); input has aria-label (placeholder-based); privacy line in key hint (en.go:1434); sample conversations seeded; auto-persistence (insights.go:476).
- [ ] [C247][HIGH] Key gate: "where to get key" link (aiprovider.KeyURL) + ballpark cost line + elevated privacy badge (insights.go:133-137) -> extract shared KeyExplainer(provider,showCost) (reuse F43 #475 + R10 #441).
- [ ] [C248][HIGH] Example canned Q&A for no-key + no-convos + no-sample state (insights.go:858) + 2 i18n example exchanges; gate noAI && len(convs)==0.
- [ ] [C250][HIGH] Active model badge near composer (resolved model incl silent default fix l49) + running session-total cost (sum Usage, ai.EstimateCostUSD, compute outside MapKeyed).
- [ ] [C251][HIGH] Gate "Edit prompt" on !noAI (insights.go:824) + "Conversations saved automatically" cue (l476) + de-emphasize vs New Chat.
- [ ] [C249][MED] aria-hidden on Sparkles send icon (insights.go:789) + distinct input aria-label key (not placeholder).

<!-- ===== GRANULAR DECOMPOSITION (batch 17 — final clusters, appended 2026-06-25) ===== -->

# Granular todo decomposition — batch 17 (research, 2026-06-25) — FINAL

## MIA multi-institution analytics (#443/#444/#445 -> atomic) [USER REQUEST]
ALREADY DONE: `internal/scope/scope.go` (ReportScope/IsAll/ResolveScope w/ institutionOf accessor/ApplyScopeToTxns/ApplyScopeToAccounts) committed+tested.
- [ ] [443][BLOCKER] `Account.Institution string` on domain (entities.go after Custom map) — additive JSON; unlocks everything. Update scope_test stub to real field; add `domain.DistinctInstitutions(accounts)` + `domain.InstitutionOf` accessor.
- [ ] [443] `UseActiveScope` uistate atom (new activescope.go, mirror activemember.go; localStorage "cashflux:active-scope"; default IsAll); `domain.SavedView` type + Dataset/SQLite persistence + CRUD; seed sample accounts with Institution (2+ distinct).
- [ ] [444][BLOCKER] Wire scope into reports: `reports_screen.go` after accounts() — ResolveScope + ApplyScopeToTxns/Accounts before all reports.* calls; short-circuit when IsAll.
- [ ] [444] `ScopeBanner` (new scopebanner.go, mirror samplebanner; "Viewing: <label>" + Clear; render when !IsAll) mounted in shell banner stack — ONE shared banner w/ R29-C276 + F41-C281.
- [ ] [444] `ScopeSelector` UI (institutions/owners/types multiselect pills — each pill own component, no On* in loop) on /reports + SavedView save/load.
- [ ] [445] Extend scope to dashboard (AND with member filter), insights, net-worth; `Account.Institution` field + datalist in add/edit account form (445-D); institution column in accounts list.
- Gotchas: no On* in loops; reuse scope pkg; ONE shared scope/member/role banner; i18n keys; js&wasm build tags on UI/uistate only.

## F1 sample/empty-states (-> atomic)
ALREADY DONE: C5 sync-chip backend-gated invisible (sync_client.go:498-524); C6 meta description present (index.html:49).
- [ ] [C2][CRIT] Data-loss race: gate initial save with `browserstore.SetThen(...)` before ready (persist.go ~280) — mirror wipeFinancialLocalState.
- [ ] [C1][HIGH] `SetSampleActive(true)` after LoadSample in accounts.go (~91) + settings.go (~1383) load paths (only hero path sets it).
- [ ] [C4][MED] Sample banner prominence: bg contrast + real button styling for "Start fresh" + icon + font-size (web/index.html .sample-banner ~2203; samplebanner.go:27).
- [ ] [C3][MED] First-run "viewing sample data" hint (firstRun prop from hydrateDataset → samplebanner.go).
- [ ] [C7][MED] First-run add-account framing via `SetAddContext("first-run")` (dashboard_hero.go:209) + welcome header in modal; clear on close.
- [ ] [C8][MED] Empty dashboard: gate kpi tiles to empty-state placeholders when len(accounts)==0 (dashboard.go) + de-emphasize bento below onboard card; ensure onboard card first in tab order.

## F2 CSV/import UX (-> atomic)
ALREADY DONE: C15 wizard pre-pop from detected columns (documents.go:239-244,1157); C17 per-row "Already imported" badge (partial, documents.go:963 — within-batch dup = R8 overlap); C18 cadence feedback fires (misplaced — see C18-a).
- [ ] [C10/C11][MAJOR] Root fix: `recordDocument` called with accountID="" + rows=nil for CSV (documents.go:157,185) — pass `importAcct.Get()` + real rows/RowCount; then summary "Imported N into <acct>" (i18n documents.importedCsvInto); add `RowCount int` to domain.Document.
- [ ] [C12][MAJOR] Draft-review: move footer (account select + Import button) above the rows list, or duplicate a condensed selector on top (documents_draft_review.go:168-192).
- [ ] [C13][MAJOR] Reorder Documents(): CSV/statement (no-AI) first, AI image import last + section separator (documents.go:705-852).
- [ ] [C14][MAJOR] Import entry from empty states: "Import" link (Href /documents) on transactions.go:528 + accounts.go:328 empty CTAs.
- [ ] [C16][MINOR] "Skipped N rows" detail: render skipped[].Line/.Reason (top 3 + "N more") in an err P (documents.go:160,188; store.CSVRowError).
- [ ] [C18][MINOR] Cadence reminder confirmation node placed next to the button (separate cadenceMsg state) (documents.go:736-741).
- [ ] [C9/C19][DESIGN] "Why no bank sync" + "How to export CSV from your bank" help text in CsvImportCard + link to /help (documents_csv_import.go:31).
- [ ] [C20][DESIGN] Richer no-key image explainer (cost/link/privacy) + "try manual entry" escape (overlaps R10 #441).

## F13 rules engine (-> atomic)
ALREADY DONE: C107 dup id fixed in Go (ruleaddform.go:121, data-testid="rule-add-form") — BUT 7 e2e files still query #rule-add (migrate selectors). NOTE: `internal/rules/conditions.go` Condition (AllKeywords/AnyKeywords/AccountID/Min/MaxAmount) fully built+tested but UNWIRED (dead code).
- [ ] [C105][MAJOR] Wire `rules.Condition` into Rule struct + matches()/FirstMatch() + pass AccountID/Amount at call sites (appstate.go:739,758,1495) + SQLite round-trip + advanced-conditions form panel. (the big one; gates C111)
- [ ] [C102][MAJOR] `SetPayee` rule action: Rule field + apply in AutoCategorize/ApplyRules + form/edit inputs + tests.
- [ ] [C103][MAJOR] `ApplyRulesResult{Total, ByRuleID}` per-rule counts + UI breakdown (appstate.go:1485; rules.go:82).
- [ ] [C104][MAJOR] Fix tag-skip: merge tags instead of no-op when txn already has any tag (appstate.go:1500).
- [ ] [C108][MAJOR] `ApplyRulesForce` (re-categorize already-categorized) + "Re-apply" button + correction auto-propagate hook.
- [ ] [C110][MED] Rule delete confirm/undo (pendingDelete inline or timed-undo notice) (rules.go:307-390).
- [ ] [C109][P3] Wrap match input in uiw.FormField w/ visible label + order (ruleaddform.go:125).
- [ ] [C111][P3] Rule OwnerID + filter /rules by active member (depends C105).

## F29 net worth (-> atomic)
- [ ] [C212][MAJOR] kpi-assets bento tile (nw.Assets computed ~dashboard.go:98) + default layout + kpiSig — OVERLAPS F9 #467.
- [ ] [C216][BUG] Reports NW AreaChart plots raw cents (reports_screen.go:245) — divide by nwDiv like dashboard (dashboard.go:708); keep raw for hover labels.
- [ ] [C213][MAJOR] Interactive hover tooltips on both NW charts (dashboard.go:744; reports_screen.go:920) — add Tooltip to ChartProps → chart.js.
- [ ] [C217][DESIGN] Decouple Reports NW trend from cash-flow period selector — separate always-monthly nwBounds (reports_screen.go:222-244).
- [ ] [C218][DESIGN] `/net-worth` route + NetWorthScreen (new networth.go) — extract shared NW render from reports.
- [ ] [C214][MINOR] Remove duplicate data-countup (hero vs kpi tile) so one figure animates (dashboard_hero.go:151).
- [ ] [C215][MINOR] Drop unlabeled partial current-month point from dashboard NW trend (dashboard.go:673 `i-(months-1)`).

## F50 help/support + F37 health (-> atomic)
ALREADY DONE: F50 — C326 whatsnew toast+card (whatsnew.go, help.go:60); C327 palette help + "?" button (shortcuts.go:288, shell.go:765); C328 /help route + 7 topic cards + help.faq.Items/Filter; C325 copy-bug-report in settings; C329 onboard card. F37 — C260/C262 healthscore.Evaluate (6-factor incl NW-trend) + /health screen + dashboard widget + UseHealthTrend all SHIPPED; C261 SMART-A10 correctly separate.
- [ ] [C328][HIGH] Wire help.Items()+Filter() into HelpScreen as searchable FAQ accordion (help.go after :91; query-state own component).
- [ ] [C260][HIGH] Wire NWTrendPct/HasNWTrend into `buildHealthInputs` (health.go ~147) via `ledger.NetWorthSeries(accounts,txns,[now-3mo,now],rates)` — factor loop already renders it. (R27 #452 — the only remaining health wiring)
- [ ] [C325][MED] "Report a bug / request feature" GitHub-issues link inside HelpScreen (i18n help.reportBug).
- [ ] [C329][MED] Per-screen feature-discovery tip line on accounts/budgets/goals empty CTAs (emptystate.go optional Tip field).
- [ ] [C326][LOW] Parse CHANGELOG.md (//go:embed via new internal/changelog) for whatsNewCard instead of static bullets.
- [ ] [C327][LOW] "Press ? for shortcuts" hint on HelpButton (shell.go:765).
- [ ] [C262/C261][LOW] data-testid on health-widget/health-screen + annotate SMART-A10 catalog (per-account vs free household).

<!-- ===== GRANULAR DECOMPOSITION (batch 18 — stragglers, decomposition COMPLETE 2026-06-25) ===== -->

# Granular todo decomposition — batch 18 (research, 2026-06-25) — stragglers / FINAL

## R18 date-display sweep (#442 -> atomic)
ALREADY DONE: C179 goal date (goals_row.go:184 pr.FormatDate); C241 reports Covering (reports_screen.go:692).
Remaining raw-date sites to route through `pr.FormatDate` / `pr.FormatMonthYear` (add `pr := uistate.UsePrefs().Get()` at each component top):
- [ ] [C155][MINOR] RecurringRow next-due — planning.go:865 (`r.NextDue.Format`).
- [ ] [MINOR] Reconcile txn row — accounts_row.go:568.
- [ ] [MINOR] Dashboard goals widget "by <date>" — dashboard.go:1043; recent-txns date col — dashboard.go:1158.
- [ ] [MINOR] Planning low-balance/breach/since dates — planning.go:486,489,694.
- [ ] [MINOR] Artifacts upload date — artifacts.go:302; DocHistoryRow upload — documents.go:990; documents cadence toast — documents.go:373; pinned insight date — insights.go:1245.
- [ ] [MINOR] FormatMonthYear: planning "debt free by" 263 + burn-down x-axis 311 + snow/aval month labels 753/754/758.
- [ ] [LOW] AI-context date strings (chat_agent.go:331,438,456,472; smartai.go:156,174) — consistency only.
- NOT bugs (machine ISO): <input type=date> seeds, dedupe/fingerprint/notify keys, extract.Row.Date storage.

## F23 goals remainder (-> atomic)
- [ ] [C180][MAJOR] Contribute/edit replace the WHOLE row (hiding name+actions) — goals_row.go:114-165 — render form as inline panel AFTER budget-head (like txn inline edit), not an early-return full replacement.
- [ ] [C181][MINOR] Delete button unreachable on touch (hover-only `.btn-del-hover` opacity:0+pointer-events:none, web/index.html:1407) — drop `btn-del-hover` on goal rows OR `@media (pointer:coarse)` always-show; add `:focus-visible` fallback.
- [ ] [C182][DESIGN] "Overall Progress" tooltip is Smart-gated (invisible when Smart off) — goals.go:272 — add plain `Attr("title", uistate.T("smart.tipGoalProgress"))` (key exists) unconditionally.

## F31 other-asset (-> atomic)
- [ ] [C224][MAJOR] `TypeProperty`/`TypeVehicle` consts + AllAccountTypes/Valid/Class (enums.go:49-67) + icons (accounts.go:430) + freshnessTypes (settings.go:448) — coordinate w/ F9 #467 (Retirement/Crypto) in ONE domain commit + sample property/vehicle.
- [ ] [C225][MAJOR] `ValuationEntry{date,value,note}` (domain) + separate SQLite table + Put/List + round-trip test; wire setBalance (accounts.go:200) + Mark-updated (accounts.go:82) to append; valuation-history panel (ValuationRow component) on account detail.
- [ ] [C226][MINOR] Type-aware stale copy ("Estimate due"/"Update estimated value") for property/vehicle (i18n accounts.staleIlliquid) + freshness window 365 for property/vehicle, raise investment 60->90 (freshness.go:30-43).
- [ ] [C227][DESIGN] Local-first disclosure note in the property/vehicle value form ("enter from Zillow/KBB; we don't fetch live") — accounts_row.go ~300; i18n accounts.valuationLocalNote.

## F20 bills + F21 subscriptions remainder (-> atomic)
ALREADY DONE: C157 autopay flag+badge (entities.go:264, bills.go:79, bills_screen.go:303, planning.go:597); C158 horizon 7->14 (notify/defaults.go:9); C161 IsLiabilityPayment (classify.go:51, subscriptions_screen.go:91); C162 renewing-soon dedup (subscriptions_screen.go:305); C163 cancel-guidance link (subscriptions_screen.go:662); C164 sample rename; C165 Netflix group-by-name (subscriptions.go:84).
- [ ] [C160][DESIGN] Autopay badge for liability-account-derived bills (not just Recurring): add `Account.Autopay bool` + toggle in liability sub-form + set in bills.Upcoming (bills.go:45-56).
- [ ] [C166][DESIGN] Detection preferences: `DetectOpts{ExcludedCategoryIDs, ExcludedAccountTypes}` into subscriptions.Detect (subscriptions.go:74) + prefs card in subscriptions_screen.
- [ ] [C167][DESIGN] Collapse Cancel + How-to-cancel into an overflow `…` menu (keep Remind primary) OR show Cancel only when row checkbox checked — subscriptions_screen.go:648-679.

## R30 webauthn (#461) + flip-backdrop (#414) (-> atomic)
- [ ] [#414][MINOR] `.flip-backdrop{pointer-events:none}` base + `.flip-backdrop.show{pointer-events:auto}` (web/index.html:1992-1993) + extend e2e probe_hardening.mjs assertion.
- [ ] [C283][CLOSE] MFA — deferred (no server = theater); leave a one-line comment in settings_section.go cloud block. Close.
- [ ] [C282][MAJOR, optional] Passkey unlock: new `internal/webauthn/webauthn.go` (js&wasm: IsAvailable w/ HTTPS+PRF feature-detect, Register, Authenticate→32B PRF) + `DecryptDatasetWithPRF` in datasetcrypto.go (dual-envelope: PRF-wrapped key alongside passcode-wrapped) + persist credentialID (browserstore) + "Unlock with passkey" button (applockgate.go:91) + enroll in setup (applockgate.go:196) + remove-passkey in settings. Caveats: PRF needs Chrome116+/Safari17+/FF119+; false on file://.

## Misc loose C-items (-> atomic)
ALREADY DONE: C77 JPY rate fixed (sample.go:853 = 0.0066); C92 txn_payee/txn_abs vars registered (appstate.go:1129-1156 + triggers_test); C158 horizon 14 (notify/defaults.go:9); C314 wasm gzip/brotli at serve (e2e/serve.go:24 + deploy workflow).
- [ ] [C72][MAJOR] Dashboard net-worth ambiguity: net-worth tile sub-label -> formula disclosure "assets X · debts Y" (dashboard.go:172, i18n) + aria-description on liabilities tile + verify kpiSig stable (dashboard.go:109-118). Overlaps F9/F29 kpi-assets tile.
- [ ] [C145][MINOR] "Needs attention" safe-to-spend anchor: `KindSafeToSpend` + Inputs.SafeToSpend/HasSafeToSpend + Config toggle + Rank() branch (attention.go:46,80,91,104) + widget schema (widgetcfg/builtins.go:76) + dashboard wires it — GATED on R15-foundation (safespend) wiring.

<!-- DECOMPOSITION COMPLETE: all C1-C329 now have atomic sub-todos or an ALREADY-SHIPPED flag with evidence. -->

<!-- ===== VERIFICATION PASS (audit batch-19, 2026-06-25) ===== -->
## Verified SHIPPED & marked completed (file:line + go test evidence)
26 C-items confirmed done by read-only audit and closed in the task list:
- PWA: C306 (manifest icons + apple-touch + meta), C307 (#installBtn beforeinstallprompt lifecycle).
- Sync F49: C320 (loadSyncStatus backend-gate), C321 (data-testid=sync-chip), C322 (backoff.Delay+Jitter; `go test ./internal/backoff` PASS), C323 (offline listener), C324 (sync:rev reactive atom).
- Alerts F38: C263 (notifySettings/alertRow), C264 (thresholds via notify.RuleConfigKey), C265 (EventPaycheckLanded), C266 (EventLowBalance), C267 (notifySeverityPill), C268 (read/dismiss/snooze), C269 (settings jump-nav); `go test ./internal/notify` PASS.
- Roles F40: C275 (role field add+edit forms), C276 (role badge); `go test ./internal/memberrole` PASS (9 tests).
- Bills/subs F20/F21: C157 (Autopay flag+badge), C158 (14-day horizon), C161 (IsLiabilityPayment), C162 (renewing-soon dedup), C163 (cancel-guidance link), C164 (sample rename), C165 (Netflix group-by-name); `go test ./internal/subscriptions ./internal/bills` PASS.
- Misc: C77 (JPY 0.0066), C92 (txn_payee/txn_abs vars + test), C314 (wasm gzip/brotli serve).
Pure-package health check: fingerprint/credithealth/payoff/scope/savings/learntally/setup/budgeting/safespend/localqa/ledger/reports/currency — all `go test` GREEN.

## New gap found by audit (filed as todo)
- [ ] [C265/C266 e2e][MINOR] Alert logic for paycheck-landed + low-balance is shipped + unit-tested, but `e2e/c265_*.mjs` / `e2e/c266_*.mjs` are MISSING — add e2e coverage to match the other alert stories (c263/c264/c267/c268/c269 exist).

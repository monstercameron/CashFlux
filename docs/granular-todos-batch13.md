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

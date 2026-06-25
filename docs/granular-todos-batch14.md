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

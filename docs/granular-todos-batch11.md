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

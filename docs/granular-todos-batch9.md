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

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// AccountEditFormProps drives the account editor form rendered inside the shell-root
// flip modal (see internal/app AccountEditHost). Mode selects which editor to show.
type AccountEditFormProps struct {
	AccountID string
	Mode      string // one of uistate.AcctEditMode*
	OnDone    func() // clears the atom (closes the modal); called after a save/cancel
}

// AccountUpdateActionLabel returns the localized "Update value" / "Update balance"
// label for the account with the given id — "value" for estimated-asset types
// (property, investments, …), "balance" for reconciled cash accounts — so the editor
// modal's title matches the wording on the row's button. Falls back to "Update balance".
func AccountUpdateActionLabel(accountID string) string {
	if app := appstate.Default; app != nil {
		for _, ac := range app.Accounts() {
			if ac.ID == accountID {
				return uistate.T(updateActionKey(ac.Type))
			}
		}
	}
	return uistate.T("accounts.updateBalance")
}

// reconSeed is the reconcile form's starting statement balance and date. It is read once, when the
// modal mounts.
type reconSeed struct {
	balance  string
	dateISO  string
	hasDraft bool
}

// loadReconSeed reads the saved reconcile draft for the account. Without a draft the statement date
// starts at today.
func loadReconSeed(accountID, todayISO string) *reconSeed {
	bal, date, has := uistate.LoadReconcileDraft(accountID)
	if !has || date == "" {
		date = todayISO
	}
	return &reconSeed{balance: bal, dateISO: date, hasDraft: has}
}

// reconciliationEvent builds the history event for a statement balance. A forced event records the
// difference that was left standing.
func reconciliationEvent(stmtMinor int64, cur, statementDate string, diffMinor int64, forced bool) domain.Reconciliation {
	ev := domain.Reconciliation{
		At: time.Now(), StatementBalance: money.New(stmtMinor, cur), DifferenceMinor: diffMinor, Forced: forced,
	}
	if d, err := dateutil.ParseDate(strings.TrimSpace(statementDate)); err == nil {
		ev.StatementDate = d
	}
	return ev
}

// editBalances is what the editor shows of the account's balances: the current and cleared figures,
// the sign convention reconciliation runs in, and the balance strip. They cost a pass over every
// transaction, so the editor computes them once per data revision, not on every keystroke.
type editBalances struct {
	Current, Cleared money.Money
	// OK is false when the balances could not be calculated.
	OK bool
	// Conv is the sign convention of the account's stored balance, and ClearedStated the cleared
	// balance in the stated convention (positive is money you have).
	Conv          reconcile.Convention
	ClearedStated money.Money
	// Strip is the balance strip, built only for the modes that show it.
	Strip ui.Node
}

// computeEditBalances calculates the editor's balances for the account. A failure is logged and
// leaves OK false, so the editors say the figures are unavailable instead of showing a guess.
func computeEditBalances(app *appstate.App, a domain.Account, found bool, mode string) editBalances {
	var b editBalances
	if !found {
		return b
	}
	txns := app.Transactions()
	var bErr, cErr error
	b.Current, bErr = ledger.Balance(a, txns)
	b.Cleared, cErr = ledger.ClearedBalance(a, txns)
	balErr := errors.Join(bErr, cErr)
	if balErr != nil {
		slog.Error("account editor: calculate balances", "account_id", a.ID, "error", balErr)
	}
	b.OK = balErr == nil
	// Reconciliation runs entirely in the stated convention, where positive is money you have and
	// negative is money you owe. That is what the account card shows and what a statement prints. The
	// stored figure cannot be used directly: a debt is stored positive by the "amount you owe" form
	// and negative by sample data and some importers, so the same debt would reconcile against
	// opposite statement figures depending on how it happens to be stored.
	b.Conv, _ = reconcile.ConventionOf(a, b.Current.Amount)
	b.ClearedStated = money.New(reconcile.Stated(b.Cleared.Amount, b.Conv), a.Currency)
	if mode != uistate.AcctEditModeReconcile && mode != uistate.AcctEditModeTransfer {
		b.Strip = accountBalancesStrip(app, a, b.Current, b.ClearedStated, b.OK)
	}
	return b
}

// AccountEditForm renders one of the account editors (edit / update-balance /
// reconcile / transfer) as the body of the flip modal. It owns all its form state
// and its own Save/Cancel buttons; the host's FlipPanel is CloseOnly. Because the
// host only renders this when the atom is set, the component mounts fresh on each
// open, so useState initializers seed correctly from the account (no explicit seeding
// step). It lives at the shell root, outside the transformed bento/tile ancestors, so
// the modal centers on the viewport.
func AccountEditForm(props AccountEditFormProps) ui.Node {
	// Re-render on data mutations (e.g. reconcile marking a txn cleared re-derives the
	// live cleared balance / difference below).
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var a domain.Account
	found := false
	if app != nil {
		for _, ac := range app.Accounts() {
			if ac.ID == props.AccountID {
				a = ac
				found = true
				break
			}
		}
	}
	dec := currency.Decimals(a.Currency)
	cbs := buildAcctRowCallbacks(app)
	todayISO := dateutil.FormatDate(time.Now())

	// Current balances for the setbal delta preview, the balance strip and the reconcile
	// difference. When either cannot be calculated the editors say so and the reconcile
	// form stays closed, because a difference built on a guess is worse than none.
	balances := ui.UseMemo(func() editBalances { return computeEditBalances(app, a, found, props.Mode) },
		props.AccountID, props.Mode, revKey(app))
	curBal, balancesOK := balances.Current, balances.OK
	reconConv, curClearedStated := balances.Conv, balances.ClearedStated

	// ---- setbal state ----
	setBalAmtS := ui.UseState("")
	setBalCatS := ui.UseState("")
	// The date the balance was true as of, which is not always today: a
	// figure read on Friday and entered on Monday belongs to Friday, or every
	// period total files it in the wrong month. It defaults to today, the common case.
	// Nobody should have to fill in a date just to update a balance.
	setBalAsOfS := ui.UseState(todayISO)
	onSetBalAmt := ui.UseEvent(func(v string) { setBalAmtS.Set(v) })
	onSetBalAsOf := ui.UseEvent(func(v string) { setBalAsOfS.Set(v) })

	// ---- transfer state ----
	xferFromS := ui.UseState(a.ID)
	xferToS := ui.UseState("")
	xferAmtS := ui.UseState("")
	xferDateS := ui.UseState(todayISO)
	xferDescS := ui.UseState("")
	onXferAmt := ui.UseEvent(func(v string) { xferAmtS.Set(v) })
	onXferDate := ui.UseEvent(func(v string) { xferDateS.Set(v) })
	onXferDesc := ui.UseEvent(func(v string) { xferDescS.Set(v) })

	// ---- reconcile state ----
	// reconUndo journals everything this modal writes, so Cancel can put it back. It is
	// a ref, not state: it is recorded during handlers and read only when the modal
	// closes, so mutating it must never trigger a re-render mid-write. It is created on
	// the first render only.
	reconUndoRef := ui.UseRef[*reconcile.Session](nil)
	if reconUndoRef.Get() == nil {
		reconUndoRef.Set(reconcile.NewSession())
	}
	reconUndo := reconUndoRef.Get()
	// reconAdjustKey identifies one adjustment action. It is created when the dialog opens and kept
	// across failed attempts, so a retry returns the row the first attempt posted instead of posting
	// a second one. A posted adjustment ends the action, and the next adjustment gets a new key.
	reconAdjustKey := ui.UseRef("")
	if reconAdjustKey.Get() == "" {
		reconAdjustKey.Set(id.New())
	}
	// An interrupted reconciliation resumes: the form seeds from the saved per-account
	// draft ("Save & finish later") when one exists. The draft is read on the first
	// render only.
	reconSeedRef := ui.UseRef[*reconSeed](nil)
	if reconSeedRef.Get() == nil {
		reconSeedRef.Set(loadReconSeed(props.AccountID, todayISO))
	}
	seed := reconSeedRef.Get()
	stmtBalS := ui.UseState(seed.balance)
	onStmtBal := ui.UseEvent(func(v string) { stmtBalS.Set(v) })
	// Statement closing date — the "reconciled through" point recorded with the event.
	stmtDateS := ui.UseState(seed.dateISO)
	onStmtDate := ui.UseEvent(func(v string) { stmtDateS.Set(v) })
	// recordReconciliation saves a completed reconciliation to the account's capped
	// history and confirms the balance (BalanceAsOf). Both the normal and the forced
	// finish go through it; it reports whether the event was saved.
	recordReconciliation := func(ev domain.Reconciliation) (appstate.RecordOutcome, bool) {
		outcome, err := app.RecordReconciliation(a.ID, ev)
		if err != nil {
			postAcctWriteError("record reconciliation", err)
			return outcome, false
		}
		uistate.ClearReconcileDraft(a.ID)
		reconUndo.Forget() // a recorded reconciliation keeps what it wrote
		uistate.BumpDataRevision()
		return outcome, true
	}
	// Recording a reconciliation closes the modal. It is only offered once the
	// difference is zero, so the recorded event is always a true match.
	doRecordRecon := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		stmtMinor, perr := money.ParseMinor(strings.TrimSpace(stmtBalS.Get()), dec)
		if perr != nil {
			uistate.PostNotice(uistate.T("accounts.invalidBalance"), true)
			return
		}
		ev := reconciliationEvent(stmtMinor, a.Currency, stmtDateS.Get(), 0, false)
		outcome, ok := recordReconciliation(ev)
		if !ok {
			return
		}
		msg := uistate.T("accounts.reconRecorded", uistate.CurrentPrefs().FormatDate(ev.Through()))
		if outcome.Retired > 0 {
			msg += " " + uistate.T("accounts.reconCheckpointsRetired", outcome.Retired)
		}
		uistate.PostNotice(msg, false)
		done()
	}))
	// Discrepancy-resolution paths.
	// Post an adjustment transaction for the exact difference so the cleared
	// balance meets the statement; cleared+reviewed, clearly labeled, undoable.
	doReconAdjust := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		_, diff, ok := unresolvedDifference(app, a, reconConv, stmtBalS.Get(), dec)
		if !ok {
			return
		}
		desc := uistate.T("accounts.reconAdjustDesc", strings.TrimSpace(stmtDateS.Get()))
		posted, err := app.PostReconcileAdjustmentOnce(a.ID, reconAdjustKey.Get(), diff.DifferenceMinor, reconConv, desc)
		if err != nil {
			postAcctWriteError("post reconcile adjustment", err)
			return
		}
		reconAdjustKey.Set(id.New())
		reconUndo.NoteCreated(posted.ID)
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("accounts.reconAdjustPosted", fmtMoney(money.New(diff.DifferenceMinor, a.Currency))))
	}))
	// Force-complete despite a discrepancy: an explicit confirm, then the event
	// records WITH the unresolved difference (Forced) so history stays honest.
	doReconForce := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		stmtMinor, diff, ok := unresolvedDifference(app, a, reconConv, stmtBalS.Get(), dec)
		if !ok {
			return
		}
		diffStr := fmtMoney(money.New(diff.DifferenceMinor, a.Currency))
		uistate.ConfirmModal(uistate.T("accounts.reconForceConfirm", diffStr), true, func(ok bool) {
			if !ok {
				return
			}
			ev := reconciliationEvent(stmtMinor, a.Currency, stmtDateS.Get(), diff.DifferenceMinor, true)
			if _, saved := recordReconciliation(ev); !saved {
				return
			}
			uistate.PostNotice(uistate.T("accounts.reconForceRecorded", diffStr), false)
			done()
		})
	}))
	// Save the half-done reconciliation and close; the draft re-seeds next open.
	doReconSaveDraft := ui.UseEvent(Prevent(func() {
		reconUndo.Forget() // finishing later KEEPS the rows already ticked
		uistate.SaveReconcileDraft(a.ID, strings.TrimSpace(stmtBalS.Get()), strings.TrimSpace(stmtDateS.Get()))
		uistate.PostNotice(uistate.T("accounts.reconDraftSaved"), false)
		done()
	}))
	// Reopen (undo) the most recent recorded reconciliation: pop it from history
	// and re-seed the form with its figures so the user is back inside it.
	doReconReopen := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		hist, removed, ok := reconcile.Undo(a.Reconciliations)
		if !ok {
			return
		}
		cp := a
		cp.Reconciliations = hist
		if err := app.PutAccount(cp); err != nil {
			postAcctWriteError("reopen reconciliation", err)
			return
		}
		stmtBalS.Set(money.FormatMinor(removed.StatementBalance.Amount, dec))
		if !removed.StatementDate.IsZero() {
			stmtDateS.Set(dateutil.FormatDate(removed.StatementDate))
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("accounts.reconReopened", uistate.CurrentPrefs().FormatDate(removed.Through())), false)
	}))
	// Investigate: jump to this account's cleared transactions in the ledger so a
	// stubborn discrepancy can be hunted where the rows live.
	reconNav := router.UseNavigate()
	reconTxFilter := uistate.UseTxFilter()
	doReconInvestigate := ui.UseEvent(Prevent(func() {
		reconUndo.Forget() // leaving to investigate KEEPS the rows already ticked
		f := uistate.TxFilter{Account: a.ID, Cleared: "yes"}.Normalize()
		reconTxFilter.Set(f)
		uistate.PersistTxFilter(f)
		uistate.SaveReconcileDraft(a.ID, strings.TrimSpace(stmtBalS.Get()), strings.TrimSpace(stmtDateS.Get()))
		done()
		reconNav.Navigate(uistate.RoutePath("/transactions"))
	}))
	// Bulk "Mark all cleared": flip every uncleared transaction on the account
	// to cleared in one action, so a statement whose lines are all present is
	// reconciled without clicking each row. Undoable via the shared undo story
	// (Ctrl+Z restores the pre-batch snapshot). The uncleared set is collected
	// first, then mutated, so PutTransaction never reshapes the slice mid-loop. A row that
	// cannot be saved is counted and reported; the other rows still clear.
	doReconMarkAll := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		pending := reconcile.Uncleared(a.ID, app.Transactions())
		if len(pending) == 0 {
			return
		}
		now := time.Now()
		failed := 0
		for _, t := range pending {
			reconUndo.NoteBefore(t)
			if err := app.PutTransaction(t.MarkCleared(true, now)); err != nil {
				slog.Error("reconcile: mark transaction cleared", "transaction_id", t.ID, "error", err)
				failed++
			}
		}
		uistate.BumpDataRevision()
		if failed > 0 {
			uistate.PostNotice(uistate.T("accounts.reconMarkAllPartial", failed), true)
			return
		}
		postUndoStory(uistate.T("accounts.reconMarkAllDone", len(pending)))
	}))

	// ---- edit state ----
	lockISO := ""
	if !a.LockUntil.IsZero() {
		lockISO = dateutil.FormatDate(a.LockUntil)
	}
	nameS := ui.UseState(a.Name)
	typeS := ui.UseState(string(a.Type))
	varField := useEntityVarField(accountVarKind, nameS, a.VarName)
	balS := ui.UseState(money.FormatMinor(a.OpeningBalance.Amount, dec))
	climS := ui.UseState(moneyMajorOrEmpty(a.CreditLimit, dec))
	aprS := ui.UseState(rateFieldValue(a))
	minpS := ui.UseState(moneyMajorOrEmpty(a.MinPayment, dec))
	dueS := ui.UseState(intOrEmpty(a.DueDayOfMonth))
	stmtDayS := ui.UseState(intOrEmpty(a.StatementDay))
	exclNWS := ui.UseState(a.ExcludeFromNetWorth)
	lenderS := ui.UseState(a.Lender)
	institutionS := ui.UseState(a.Institution)
	// The structured institution-directory picker, separate from the free-text
	// Institution/Lender field above (which stays as a fallback label).
	instIDS := ui.UseState(a.InstitutionID)
	// A beneficiary / transfer-on-death note, shown (never a password) in
	// the estate emergency pack.
	beneficiaryNoteS := ui.UseState(a.BeneficiaryNote)
	// The per-account revaluation-cadence override, in days (0 = the type default).
	revalueDaysS := ui.UseState(intOrEmpty(a.RevalueDays))
	// Stale-balance controls: persistent exemption + "ignore until" snooze date.
	freshSnoozeISO := ""
	if !a.FreshnessSnoozeUntil.IsZero() {
		freshSnoozeISO = dateutil.FormatDate(a.FreshnessSnoozeUntil)
	}
	freshExemptS := ui.UseState(a.FreshnessExempt)
	freshSnoozeS := ui.UseState(freshSnoozeISO)
	retS := ui.UseState(floatOrEmpty(a.ExpectedReturnAPR))
	apyS := ui.UseState(floatOrEmpty(a.APY))
	liqS := ui.UseState(intOrEmpty(a.LiquidityScore))
	stabS := ui.UseState(intOrEmpty(a.StabilityScore))
	lockS := ui.UseState(lockISO)
	ownerS := ui.UseState(a.OwnerID)
	editAdvOpen := ui.UseState(false)
	// asLiabS: for an "Other"-type account (the catch-all type), an explicit override to
	// count it as a liability (debt) in the formulas — e.g. an HOA obligation. Seeded
	// from the account's stored class. Ignored for typed accounts (their class follows
	// the type).
	asLiabS := ui.UseState(accountedit.CountsAsDebt(a.Type, a.Class))
	splitOwnS := ui.UseState(len(a.OwnershipShares) > 0)
	sharesMapS := ui.UseState(cloneSharesMap(a.OwnershipShares))
	customEditVals := ui.UseState(customMapToStrings(a.Custom))
	// attemptedS is set by the first refused save. From then on the editor shows what is wrong with
	// the fields as they stand, so a message goes away as soon as its field is fixed.
	attemptedS := ui.UseState(false)
	// foldForcedS keeps the details open after a refused save whose problem sits in them, when the
	// editor opened with them folded away.
	foldForcedS := ui.UseState(false)
	errID := "acct-edit-save-err-" + a.ID
	moveToProblem := useProblemFocus(errID)
	notesS := ui.UseState(a.Notes)
	onNotes := func(v string) { notesS.Set(v) }
	onBal := ui.UseEvent(func(v string) { balS.Set(v) })
	onClim := ui.UseEvent(func(v string) { climS.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprS.Set(v) })
	onMinp := ui.UseEvent(func(v string) { minpS.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onStmtDay := ui.UseEvent(func(v string) { stmtDayS.Set(v) })
	onToggleExclNW := ui.UseEvent(func() { exclNWS.Set(!exclNWS.Get()) })
	onToggleFreshExempt := ui.UseEvent(func() { freshExemptS.Set(!freshExemptS.Get()) })
	onFreshSnooze := ui.UseEvent(func(v string) { freshSnoozeS.Set(v) })
	onLender := ui.UseEvent(func(v string) { lenderS.Set(v) })
	onInstitution := func(v string) { institutionS.Set(v) }
	// One institution concept. Picking a directory entry also fills the free-text
	// label from it (so the two storage fields agree and only one control shows);
	// clearing the pick re-reveals the free-text fallback.
	onInstID := func(v string) {
		instIDS.Set(v)
		if v != "" && app != nil {
			for _, in := range sortedInstitutions(app) {
				if in.ID == v {
					institutionS.Set(in.CleanName())
					break
				}
			}
		}
	}
	onBeneficiaryNote := func(v string) { beneficiaryNoteS.Set(v) }
	onRevalueDays := ui.UseEvent(func(v string) { revalueDaysS.Set(v) })
	onRet := ui.UseEvent(func(v string) { retS.Set(v) })
	onApy := ui.UseEvent(func(v string) { apyS.Set(v) })
	onLiq := ui.UseEvent(func(v string) { liqS.Set(v) })
	onStab := ui.UseEvent(func(v string) { stabS.Set(v) })
	onLock := ui.UseEvent(func(v string) { lockS.Set(v) })
	onToggleEditAdv := ui.UseEvent(func() { editAdvOpen.Set(!editAdvOpen.Get()) })
	onToggleAsLiab := ui.UseEvent(func() { asLiabS.Set(!asLiabS.Get()) })
	onToggleSplitOwn := ui.UseEvent(func() { splitOwnS.Set(!splitOwnS.Get()) })
	onCustomEdit := func(key, value string) {
		m := customEditVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customEditVals.Set(nm)
	}

	// ---- save / cancel handlers (own the modal's commit actions) ----
	cancel := ui.UseEvent(Prevent(func() { done() }))
	// In the reconcile modal, Cancel means what it says: the ledger
	// goes back to how it was found. Ticking rows writes them immediately (every
	// figure in the dialog recomputes from live data), so Cancel rolls back what the
	// modal wrote; otherwise a person who opened the wrong statement and ticked twenty
	// rows would keep all twenty, with no record of which they were.
	//
	// It asks first, because the rows ticked so far are real work and Cancel is
	// adjacent to the action that keeps them. Nothing to take back closes at once
	// rather than asking a pointless question.
	reconCancel := ui.UseEvent(Prevent(func() {
		if !reconUndo.Touched() {
			done()
			return
		}
		n := reconUndo.Count()
		uistate.ConfirmModal(uistate.T("accounts.reconCancelConfirm", n), true, func(ok bool) {
			if !ok {
				return
			}
			var failed int
			app.BulkMutate(func() { failed = reconUndo.Rollback(app) })
			if failed > 0 {
				uistate.PostNotice(uistate.T("accounts.reconCancelPartial", failed), true)
			} else {
				uistate.PostNotice(uistate.T("accounts.reconCancelDone", n), false)
			}
			uistate.BumpDataRevision()
			done()
		})
	}))
	// "Manage institutions" jumps from the edit form straight to the
	// institution-directory modal. Declared unconditionally here (not inside
	// editForm) so the hook count stays stable across every switch branch below.
	institutionsMgrAtom := uistate.UseInstitutionsManager()
	openInstitutionsFromEditor := ui.UseEvent(Prevent(func() { institutionsMgrAtom.Set(true) }))
	// A transfer is checked here, before it is posted, so a figure or date that cannot be read
	// leaves the form open with its message instead of closing on a transfer that never happened.
	doTransfer := ui.UseEvent(Prevent(func() {
		if app == nil || xferFromS.Get() == xferToS.Get() || xferToS.Get() == "" {
			return
		}
		if _, err := accountselect.ParseTransfer(app.Accounts(), accountselect.TransferInput{
			FromID: xferFromS.Get(), ToID: xferToS.Get(), Amount: xferAmtS.Get(), Date: xferDateS.Get(),
			Desc: xferDescS.Get(), DefaultDesc: uistate.T("accounts.transferDefaultDesc"),
		}); err != nil {
			uistate.PostNotice(transferFormError(err), true)
			return
		}
		if cbs.OnTransfer(xferFromS.Get(), xferToS.Get(), xferAmtS.Get(), xferDateS.Get(), xferDescS.Get()) {
			done()
		}
	}))
	// checkEdit checks the editor's fields the way a save does: what the account builder refuses, a
	// variable name another account uses, a custom value that does not validate, and a new value or
	// as-of date that cannot be read. It reads the live lists, so it is right inside a handler as well
	// as in the render. The account it returns is only fit to save when the problem is empty.
	checkEdit := func() (domain.Account, formProblem) {
		members := app.Members()
		// Split ownership needs two members to split between; with fewer the editor hides it, so the
		// save does not hold the account to a total the person cannot fix, and the shares the account
		// already has are kept as they are.
		splitAvailable := accountedit.SplitAvailable(len(members))
		split := splitOwnS.Get() && splitAvailable
		// An account stored without a currency is saved in the household's base currency.
		cur := a.Currency
		if strings.TrimSpace(cur) == "" {
			cur = accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)
		}
		cp, problem := checkAccountForm(a, accountedit.Fields{
			Name: nameS.Get(), Type: typeS.Get(), Owner: ownerS.Get(), Currency: cur,
			Opening: balS.Get(), AsLiability: asLiabS.Get(), CreditLimit: climS.Get(), APR: aprS.Get(), APY: apyS.Get(),
			MinPayment: minpS.Get(), DueDay: dueS.Get(), StatementDay: stmtDayS.Get(), Lender: lenderS.Get(),
			ExpectedReturn: retS.Get(), Liquidity: liqS.Get(), Stability: stabS.Get(), LockUntil: lockS.Get(),
			Institution: institutionS.Get(), InstitutionID: instIDS.Get(), Notes: notesS.Get(),
			BeneficiaryNote: beneficiaryNoteS.Get(), RevalueDays: revalueDaysS.Get(), FreshnessExempt: freshExemptS.Get(),
			SnoozeUntil: freshSnoozeS.Get(), ExcludeNetWorth: exclNWS.Get(), VarName: varField.VarName.Get(),
			Shares: sharesOrNil(sharesMapS.Get(), split), MemberIDs: memberIDsOf(members),
		}, accountCheck{
			SelfID: a.ID, Accounts: app.Accounts(), Defs: app.CustomFieldDefsFor("account"), Custom: customEditVals.Get(),
			IDs: accountInputIDs("acct-edit-"+a.ID, "acct-edit-"+a.ID), VarInputID: "acct-edit-varname-" + a.ID,
		})
		if !splitAvailable {
			cp.OwnershipShares = cloneSharesMap(a.OwnershipShares)
		}
		return cp, valueUpdateProblem(problem, setBalAmtS.Get(), setBalAsOfS.Get(), dec, a.ID)
	}
	// saveEdit applies the form through the shared account builder. A field the builder
	// rejects (an unreadable opening balance, lock-until or snooze date, a share total that
	// is not 100), a new value or date that cannot be read, or a variable name that is taken
	// shows its message under the form, moves focus to the input, and nothing is saved.
	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		attemptedS.Set(true)
		cp, problem := checkEdit()
		if problem.Any() {
			if problem.NeedsAdvanced {
				editAdvOpen.Set(true)
			}
			if problem.inDetails(a.ID) {
				foldForcedS.Set(true)
			}
			moveToProblem(problem.First())
			return
		}
		// Merged update-value + edit: when the user typed a new current value, OnSetBalance
		// persists cp (all edits) AND posts the balance adjustment + freshens BalanceAsOf in
		// one write — so we must NOT also call OnSave (that would double-write the account and
		// clobber the adjustment's freshness). When the value field is blank, it's a plain edit.
		var saved bool
		if strings.TrimSpace(setBalAmtS.Get()) != "" {
			// The value and the date were checked above, so a refusal here comes after the account
			// edits were stored (the balance step reads the stored account). When that happened the
			// person is told which part was saved, and the form stays open to try the balance again.
			before, hadBefore := storedAccount(app, a.ID)
			saved = cbs.OnSetBalance(cp, setBalAmtS.Get(), setBalCatS.Get(), strings.TrimSpace(setBalAsOfS.Get()))
			if !saved {
				if after, ok := storedAccount(app, a.ID); ok && hadBefore && !reflect.DeepEqual(before, after) {
					uistate.PostNotice(uistate.T("accounts.editSavedBalanceNot", after.Name), true)
				}
			}
		} else {
			saved = cbs.OnSave(cp)
		}
		if saved {
			done()
		}
	}))

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	var problem formProblem
	if attemptedS.Get() {
		_, problem = checkEdit()
	}

	switch props.Mode {
	case uistate.AcctEditModeReconcile:
		return reconcileForm(reconcileProps{
			Account: a, Cleared: curClearedStated, Conv: reconConv, Session: reconUndo, Decimals: dec, App: app,
			Unavailable: !balancesOK,
			StmtBal:     stmtBalS, StmtDate: stmtDateS, OnStmtBal: onStmtBal, OnStmtDate: onStmtDate,
			Cancel: reconCancel, OnRecord: doRecordRecon, OnRefresh: cbs.OnRerender,
			Actions: reconcileActions{Adjust: doReconAdjust, Force: doReconForce, Investigate: doReconInvestigate,
				SaveDraft: doReconSaveDraft, Reopen: doReconReopen, MarkAll: doReconMarkAll, HasDraft: seed.hasDraft},
		})
	case uistate.AcctEditModeTransfer:
		return transferForm(transferProps{
			Account: a, App: app, Accounts: app.Accounts(),
			From: xferFromS, To: xferToS, Amount: xferAmtS, Date: xferDateS, Desc: xferDescS,
			OnAmount: onXferAmt, OnDate: onXferDate, OnDesc: onXferDesc, OnSubmit: doTransfer, Cancel: cancel,
		})
	default:
		// The edit and "update value / balance" actions share one merged editor: it can
		// update the current value AND edit the account's details in a single save. When
		// opened from "Update value" (setbal mode) the value field is auto-focused and the
		// details fold; from "Edit" the name field is focused and the details are open. Both
		// persist through the same saveEdit handler.
		return editForm(editFormState{
			Account: a, Decimals: dec, CurBal: curBal, BalancesOK: balancesOK,
			Balances:   balances.Strip,
			Members:    app.Members(),
			Accounts:   app.Accounts(),
			Categories: app.Categories(), CustomDefs: app.CustomFieldDefsFor("account"),
			ValueFirst: props.Mode == uistate.AcctEditModeSetBal,

			Name: nameS, Type: typeS, VarName: varField.VarName, Owner: ownerS, Balance: balS, CreditLimit: climS, APR: aprS,
			MinPayment: minpS, DueDay: dueS, Lender: lenderS, Institution: institutionS, ExpectedReturn: retS, APY: apyS,
			Liquidity: liqS, Stability: stabS, LockUntil: lockS, Notes: notesS, SetBalAmt: setBalAmtS, SetBalCat: setBalCatS,
			SetBalAsOf: setBalAsOfS, StatementDay: stmtDayS, InstitutionID: instIDS, BeneficiaryNote: beneficiaryNoteS,
			RevalueDays: revalueDaysS, FreshSnooze: freshSnoozeS, Problem: problem, ErrID: errID, FoldForced: foldForcedS.Get(),
			AdvOpen: editAdvOpen, AsLiability: asLiabS, SplitOwn: splitOwnS, ExcludeNetWorth: exclNWS, FreshExempt: freshExemptS,
			Shares: sharesMapS, CustomVals: customEditVals,

			OnName: varField.OnName, OnVarName: varField.OnVarName, OnBalance: onBal, OnCreditLimit: onClim, OnAPR: onApr,
			OnMinPayment: onMinp, OnDueDay: onDue, OnLender: onLender, OnExpectedReturn: onRet, OnAPY: onApy,
			OnLiquidity: onLiq, OnStability: onStab, OnLockUntil: onLock, OnSetBalAmt: onSetBalAmt, OnSetBalAsOf: onSetBalAsOf,
			OnStatementDay: onStmtDay, OnRevalueDays: onRevalueDays, OnFreshSnooze: onFreshSnooze,
			OnToggleAdv: onToggleEditAdv, OnToggleAsLiability: onToggleAsLiab, OnToggleSplit: onToggleSplitOwn,
			OnToggleExcludeNW: onToggleExclNW, OnToggleFreshExempt: onToggleFreshExempt, OpenInstitutions: openInstitutionsFromEditor,
			OnSave: saveEdit, OnCancel: cancel,
			OnInstitution: onInstitution, OnInstitutionID: onInstID, OnNotes: onNotes, OnBeneficiaryNote: onBeneficiaryNote,
			OnCustomEdit: onCustomEdit,
		})
	}
}

// accountBalancesStrip is the 4-way balance strip and provenance chip shown above the
// value-update section: current / cleared / statement (last reconciliation) / 30-day
// projected, and how the balance most recently moved. Adjustments are identified by the
// update-balance description (they're marked at the UI layer, not structurally). The current and
// cleared figures arrive in the stated convention. When the balances could not be calculated
// those two show as unavailable. The editor builds it only in its edit and update-value modes.
func accountBalancesStrip(app *appstate.App, a domain.Account, curBal, curClearedStated money.Money, balancesOK bool) ui.Node {
	provKind, provAt := ledger.BalanceProvenance(a.ID, app.Transactions(), isBalanceAdjustment)
	provKey := map[ledger.ProvenanceKind]string{
		ledger.ProvenanceOpening:  "accounts.provOpening",
		ledger.ProvenanceAdjusted: "accounts.provAdjusted",
		ledger.ProvenanceImported: "accounts.provImported",
		ledger.ProvenanceDerived:  "accounts.provDerived",
	}[provKind]
	provText := uistate.T(provKey)
	if !provAt.IsZero() {
		provText += uistate.T("accounts.provAsOf", uistate.CurrentPrefs().FormatDate(provAt))
	}
	stmtStr := uistate.T("accounts.balNone")
	if n := len(a.Reconciliations); n > 0 {
		stmtStr = fmtMoney(a.Reconciliations[n-1].StatementBalance)
	}
	projStr := uistate.T("accounts.balNone")
	if a.ShowsCashCurve() {
		proj, err := app.ProjectAccount(a.ID, time.Now(), 30)
		if err != nil {
			projStr = uistate.T("calculation.unavailable")
		} else if proj.Known() {
			projStr = fmtMoney(money.New(proj.End, a.Currency))
		}
	}
	currentStr, clearedStr := uistate.T("calculation.unavailable"), uistate.T("calculation.unavailable")
	if balancesOK {
		currentStr, clearedStr = fmtMoney(statedBalance(a, curBal)), fmtMoney(curClearedStated)
	}
	stat := func(key, val, testid string) ui.Node {
		return Div(css.Class("acct-bal-stat"),
			Span(css.Class("t-caption", tw.TextDim), uistate.T(key)),
			Span(Attr("data-testid", testid), val))
	}
	return Div(css.Class("acct-bal-strip"), Attr("data-testid", "acct-bal-4way"),
		stat("accounts.balCurrent", currentStr, "acct-bal-current"),
		stat("accounts.balCleared", clearedStr, "acct-bal-cleared"),
		stat("accounts.balStatement", stmtStr, "acct-bal-statement"),
		stat("accounts.balProjected", projStr, "acct-bal-projected"),
		Div(css.Class("acct-bal-prov"),
			Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "acct-bal-provenance"), provText)),
	)
}

// statedBalance converts a stored balance into the stated convention the app shows and
// the user types: positive is money you have, negative is money you owe. The update-value
// section and the balance strip both display through it.
func statedBalance(a domain.Account, stored money.Money) money.Money {
	conv, _ := reconcile.ConventionOf(a, stored.Amount)
	return money.New(reconcile.Stated(stored.Amount, conv), a.Currency)
}

// unresolvedDifference reads the typed statement balance and compares it with the cleared
// balance, which is stated in the account's convention. ok is false when no valid balance
// has been typed, the cleared balance cannot be calculated, or the account already
// reconciles, so there is nothing to resolve.
func unresolvedDifference(app *appstate.App, a domain.Account, conv reconcile.Convention, stmtText string, dec int) (int64, reconcile.Result, bool) {
	stmtMinor, err := money.ParseMinor(strings.TrimSpace(stmtText), dec)
	if err != nil {
		return 0, reconcile.Result{}, false
	}
	cleared, err := ledger.ClearedBalance(a, app.Transactions())
	if err != nil {
		slog.Error("reconcile: calculate cleared balance", "account_id", a.ID, "error", err)
		return 0, reconcile.Result{}, false
	}
	diff, err := reconcile.DiffChecked(reconcile.Stated(cleared.Amount, conv), stmtMinor)
	if err != nil {
		slog.Error("reconcile: calculate difference", "account_id", a.ID, "error", err)
		return 0, reconcile.Result{}, false
	}
	return stmtMinor, diff, !diff.Reconciled
}

// signedDiffLabel formats a difference with an explicit plus sign when it is positive,
// for example "+$40.50" or "−$26,711.50".
func signedDiffLabel(minor int64, sym string, dec int) string {
	label := fmtSignedMoney(minor, sym, dec)
	if minor > 0 {
		label = "+" + label
	}
	return label
}

// setBalanceNote says, in one line, what a typed figure does. For a debt it says whether
// the debt goes down or up; anything else shows the signed change. A debt typed without
// its minus sign also gets a short warning under the line. Both figures are in the
// stated convention.
func setBalanceNote(a domain.Account, curStated, targetStated int64) ui.Node {
	preview := reconcile.PreviewDelta(curStated, targetStated)
	delta := preview.AdjustmentMinor
	isDebt := accountedit.ClassOf(a.Type, accountedit.CountsAsDebt(a.Type, a.Class)) == domain.ClassLiability
	var text string
	switch shape := reconcile.ClassifyChange(curStated, targetStated); {
	case !preview.NeedsAdjustment:
		text = uistate.T("accounts.setBalanceNoAdjNeeded")
	case isDebt && shape == reconcile.ShapeDebtFalls:
		text = uistate.T("accounts.setBalanceDebtFalls", fmtMoney(money.New(delta, a.Currency)))
	case isDebt && shape == reconcile.ShapeDebtRises:
		text = uistate.T("accounts.setBalanceDebtRises", fmtMoney(money.New(-delta, a.Currency)))
	default:
		text = uistate.T("accounts.setBalanceDeltaPreview", signedDiffLabel(delta, currency.Symbol(a.Currency), currency.Decimals(a.Currency)))
	}
	line := P(css.Class("t-caption acct-value-delta"), Attr("data-testid", "setbal-delta-preview"), text)
	warning := If(isDebt && reconcile.LooksLikeUnsignedDebt(curStated, targetStated), P(css.Class("t-caption"), Attr("data-testid", "setbal-unsigned-warning"),
		uistate.T("accounts.setBalanceUnsignedDebt", fmtMoney(money.New(-targetStated, a.Currency)))))
	return Div(line, warning)
}

// setBalanceInputID is the id of the new-value input of the account's editor.
func setBalanceInputID(accountID string) string { return "acct-setbal-" + accountID }

// setBalanceAsOfInputID is the id of the as-of date input that appears beside the new value.
func setBalanceAsOfInputID(accountID string) string { return "acct-setbal-asof-" + accountID }

// valueUpdateProblem adds to the problem anything wrong with a new value and the date it was true
// as of. A blank value leaves the balance as it is, so nothing is checked then, and a blank date
// means today.
func valueUpdateProblem(p formProblem, amount, asOf string, decimals int, accountID string) formProblem {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return p
	}
	if _, err := money.ParseMinor(amount, decimals); err != nil {
		p = p.And(uistate.T("accounts.invalidBalance"), setBalanceInputID(accountID), false)
	}
	if date := strings.TrimSpace(asOf); date != "" {
		if _, err := dateutil.ParseDate(date); err != nil {
			p = p.And(uistate.T("accounts.setBalanceAsOfInvalid"), setBalanceAsOfInputID(accountID), false)
		}
	}
	return p
}

// storedAccount is the account with the id as the household has it stored right now.
func storedAccount(app *appstate.App, accountID string) (domain.Account, bool) {
	for _, ac := range app.Accounts() {
		if ac.ID == accountID {
			return ac, true
		}
	}
	return domain.Account{}, false
}

// inDetails reports whether the problem concerns the account's details, which an editor opened on
// "Update balance" folds away, and not only the new value at the top. A problem that names no input
// (a share total, a custom value) is about the details too.
func (p formProblem) inDetails(accountID string) bool {
	if len(p.Inputs) == 0 || p.Unplaced {
		return true
	}
	for _, id := range p.Inputs {
		if id != setBalanceInputID(accountID) && id != setBalanceAsOfInputID(accountID) {
			return true
		}
	}
	return false
}

// acctValueUpdateSection is the "update value / balance" block at the top of the merged
// editor. Entering a value records an adjustment so the balance matches it; leaving it
// blank keeps the balance as-is. The block shows the current balance, a live delta
// preview, and, once a value is entered, the adjustment's category and date. curStated
// is the current balance in the stated convention; balanceKnown is false when it could not be
// calculated, and the block then shows no current figure and no preview.
func acctValueUpdateSection(a domain.Account, curStated money.Money, balanceKnown bool, dec int, setBalAmtS, setBalCatS, setBalAsOfS ui.State[string], onSetBalAmt, onSetBalAsOf ui.Handler, categories []domain.Category, valueFirst bool, problem formProblem, errID string) ui.Node {
	rawAmt := strings.TrimSpace(setBalAmtS.Get())
	deltaNode := ui.If(rawAmt != "" && balanceKnown, func() ui.Node {
		targetMinor, parseErr := money.ParseMinor(rawAmt, dec)
		if parseErr != nil {
			return Fragment()
		}
		return setBalanceNote(a, curStated.Amount, targetMinor)
	})
	catOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.setBalanceNoCategory")}}
	for _, c := range categories {
		catOpts = append(catOpts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	nowText := uistate.T("accounts.currentValueNow", fmtMoney(curStated))
	if !balanceKnown {
		nowText = uistate.T("calculation.unavailable")
	}
	valID, asOfID := setBalanceInputID(a.ID), setBalanceAsOfInputID(a.ID)
	valInput := []any{css.Class("field", fieldInvalidName(problem, valID)), Attr("id", valID), Attr("data-testid", "acct-setbal-input"),
		Type("number"), Placeholder(uistate.T("accounts.setBalanceAmount")),
		Step(accountedit.MoneyStep(a.Currency)), OnInput(onSetBalAmt)}
	valInput = append(valInput, fieldInvalidAttrs(problem, valID, errID)...)
	if valueFirst {
		valInput = append(valInput, Attr("autofocus", ""))
	}
	return Div(css.Class("acct-value-section"), Attr("data-testid", "acct-value-section"),
		labeledField(uistate.T(updateActionKey(a.Type)), Input(append(valInput, uiw.FieldValue(setBalAmtS.Get()))...)),
		Span(css.Class("t-caption acct-value-now", tw.TextDim), Attr("data-testid", "acct-value-now"), nowText),
		deltaNode,
		// Shown only once a value is typed, so updating a balance stays one
		// field for the common case. It answers "when was this true", which is what
		// makes the resulting checkpoint land in the right month and lets the right
		// statement retire it later.
		If(rawAmt != "", labeledField(uistate.T("accounts.setBalanceAsOfLabel"),
			Input(append([]any{css.Class("field", fieldInvalidName(problem, asOfID)), Type("date"), Attr("id", asOfID), Attr("data-testid", "acct-setbal-asof"),
				AriaLabel(uistate.T("accounts.setBalanceAsOfLabel")),
				Title(uistate.T("accounts.setBalanceAsOfHint")),
				OnInput(onSetBalAsOf)}, append(fieldInvalidAttrs(problem, asOfID, errID), uiw.FieldValue(setBalAsOfS.Get()))...)...))),
		If(rawAmt != "", labeledField(uistate.T("accounts.setBalanceCategoryLabel"),
			uiw.SelectInput(uiw.SelectInputProps{Options: catOpts, Selected: setBalCatS.Get(),
				OnChange: func(v string) { setBalCatS.Set(v) }, AriaLabel: uistate.T("accounts.setBalanceCategoryLabel"), TestID: "setbal-cat-select"}))),
		If(isValuationType(a.Type), P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "valuation-manual-note"),
			uistate.T("accounts.valuationManualNote"))),
	)
}

// reconcileActions bundles the discrepancy-resolution handlers: post an adjustment for the
// difference, force-complete with the gap recorded, jump to the cleared ledger
// to investigate, save the half-done session as a draft, reopen (undo) the
// most recent recorded reconciliation, and mark every uncleared row cleared.
type reconcileActions struct {
	Adjust, Force, Investigate, SaveDraft, Reopen, MarkAll ui.Handler
	HasDraft                                               bool
}

// reconcileProps is everything the reconcile-to-statement editor reads.
type reconcileProps struct {
	Account domain.Account
	// Cleared is the cleared balance, already in the stated convention (positive is money you
	// have), matching what the account card shows and what the user types off the statement.
	Cleared  money.Money
	Conv     reconcile.Convention
	Session  *reconcile.Session
	Decimals int
	App      *appstate.App
	// Unavailable is set when the balances could not be calculated; the form then says so
	// and offers no way to record a reconciliation.
	Unavailable bool

	StmtBal, StmtDate ui.State[string]
	OnStmtBal         ui.Handler
	OnStmtDate        ui.Handler
	Cancel            ui.Handler
	OnRecord          ui.Handler
	OnRefresh         func(domain.Account)
	Actions           reconcileActions
}

// reconcileForm is the reconcile-to-statement editor. Once the difference hits
// zero, "Record reconciliation" (OnRecord) appends the event to the account's
// history; the form also shows the "reconciled through" status and the recent
// history so past reconciliations are inspectable in place. While a typed
// statement balance still disagrees with the cleared balance, the resolution
// row (Actions) offers the legitimate ways out of the dead end.
func reconcileForm(p reconcileProps) ui.Node {
	if p.Unavailable {
		return ui.CreateElement(reconcileUnavailable, p)
	}
	return ui.CreateElement(reconcileView, p)
}

// reconcileUnavailable is the reconcile editor when the balances could not be calculated: it says
// so and offers only Cancel, because a difference built on a guess is worse than none. It is its
// own component so the handler hooks of the full editor never change position when the
// balances become available.
func reconcileUnavailable(p reconcileProps) ui.Node {
	return Div(css.Class("acct-edit-form"), Attr("data-testid", "reconcile-statement-mode"),
		Div(css.Class("modal-scroll"),
			P(css.Class("err"), Role("alert"), Attr("data-testid", "reconcile-unavailable"), uistate.T("calculation.unavailable"))),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(p.Cancel), uistate.T("action.cancel"))),
	)
}

// reconcileView is the full reconcile editor, mounted as a component by reconcileForm.
func reconcileView(p reconcileProps) ui.Node {
	a, app, acts := p.Account, p.App, p.Actions
	stmtText := strings.TrimSpace(p.StmtBal.Get())
	stmtTyped := stmtText != ""
	stmtMinor, stmtErr := money.ParseMinor(stmtText, p.Decimals)
	// A figure that cannot be read is not a statement balance of zero: the difference, the resolution
	// actions and the finish button wait until it is a number.
	stmtInvalid := stmtTyped && stmtErr != nil
	stmtOK := stmtTyped && stmtErr == nil
	// The same checked subtraction the handlers use, so this view and the Finish gate cannot
	// disagree on a figure large enough to overflow. An overflow is shown as unavailable and
	// never counts as reconciled.
	result, diffErr := reconcile.DiffChecked(p.Cleared.Amount, stmtMinor)
	if diffErr != nil {
		slog.Error("reconcile: calculate difference", "account_id", a.ID, "error", diffErr)
		result = reconcile.Result{}
	}
	diffOverflow := stmtOK && diffErr != nil
	reconciled := stmtOK && diffErr == nil && result.Reconciled
	sym := currency.Symbol(a.Currency)
	// Signed and thousands-grouped, like the cleared-balance figure it
	// sits beside ("+$40.50" / "−$26,711.50"), not a bare "-26711.50".
	diffLabel := signedDiffLabel(result.DifferenceMinor, sym, p.Decimals)
	if stmtInvalid || diffOverflow {
		diffLabel = uistate.T("calculation.unavailable")
	}
	stmtErrID := "reconcile-statement-err-" + a.ID
	stmtErrMsg := ""
	if stmtInvalid {
		stmtErrMsg = uistate.T("accounts.invalidBalance")
	}
	unclearedTxns := reconcile.Uncleared(a.ID, app.Transactions())
	onToggleClear := func(t domain.Transaction) {
		p.Session.NoteBefore(t) // so Cancel can put this row back
		if err := app.PutTransaction(t.MarkCleared(!t.Cleared, time.Now())); err != nil {
			postAcctWriteError("mark transaction cleared", err)
			return
		}
		p.OnRefresh(a) // bumps the data revision → this modal re-derives the difference
	}
	keyOfTxn := func(t domain.Transaction) any { return t.ID }
	renderTxnRow := func(t domain.Transaction) ui.Node {
		return ui.CreateElement(ReconcileTxnRow, reconcileTxnRowProps{Txn: t, OnToggle: onToggleClear})
	}
	// "Reconciled through" status + the recent history (newest first, capped at
	// six rows here — the full capped history stays on the account).
	through, hasThrough := reconcile.Through(a.Reconciliations)
	var histRows []ui.Node
	for i := len(a.Reconciliations) - 1; i >= 0 && len(histRows) < 6; i-- {
		r := a.Reconciliations[i]
		histRows = append(histRows, Div(css.Class("row", "acct-recon-hist-row"), Attr("data-testid", "reconcile-history-row"),
			Span(uistate.CurrentPrefs().FormatDate(r.Through())),
			If(r.Forced, Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "reconcile-history-forced"),
				uistate.T("accounts.reconForcedTag", money.FormatMinor(r.DifferenceMinor, p.Decimals)))),
			Span(fmtMoney(r.StatementBalance)),
		))
	}

	// Bulk "Mark all cleared" preview: what the remaining difference becomes if
	// every uncleared row is cleared at once — so the button can promise an exact
	// match or warn about the gap that would still stand.
	unclearedAmounts := make([]int64, 0, len(unclearedTxns))
	for _, t := range unclearedTxns {
		// Stated, like Cleared. Stated is linear, so a sum of stated legs is the
		// stated sum. Mixing a stored leg into a stated total would reintroduce the sign
		// error.
		unclearedAmounts = append(unclearedAmounts, reconcile.Stated(t.Amount.Amount, p.Conv))
	}
	bulk, bulkErr := reconcile.PreviewBulkClearChecked(p.Cleared.Amount, stmtMinor, unclearedAmounts)
	if bulkErr != nil {
		slog.Error("reconcile: preview marking every row cleared", "account_id", a.ID, "error", bulkErr)
		bulk = reconcile.BulkClear{Count: len(unclearedAmounts)}
	}
	bulkDiffLabel := signedDiffLabel(bulk.Result.DifferenceMinor, sym, p.Decimals)
	remainingCls := "reconcile-remaining"
	if reconciled {
		remainingCls += " is-zero"
	}
	bulkPreview := uistate.T("accounts.reconMarkAllPreviewGap", bulk.Count, bulkDiffLabel)
	switch {
	case bulkErr != nil:
		bulkPreview = uistate.T("calculation.unavailable")
	case bulk.Result.Reconciled:
		bulkPreview = uistate.T("accounts.reconMarkAllPreviewMatch", bulk.Count)
	}

	return Div(css.Class("acct-edit-form"), Attr("data-testid", "reconcile-statement-mode"),
		Div(css.Class("modal-scroll"),
			// Sticky math header: the statement inputs and the live remaining-
			// difference readout stay anchored to the top while the transaction
			// list scrolls, so the reconciliation math is always in view.
			Div(css.Class("reconcile-header"), Attr("data-testid", "reconcile-header"),
				If(hasThrough, P(css.Class("budget-sub", "acct-gap-below"), Attr("data-testid", "reconcile-through"),
					uistate.T("accounts.reconThrough", uistate.CurrentPrefs().FormatDate(through)))),
				If(acts.HasDraft, P(css.Class("t-caption", tw.TextDim, "acct-gap-below"), Attr("data-testid", "reconcile-draft-note"),
					uistate.T("accounts.reconDraftNote"))),
				labeledField(uistate.T("accounts.statementBalance"),
					Input(append([]any{css.Class("field", condName(stmtInvalid, "field-invalid")), Attr("id", "acct-reconcile-stmt-"+a.ID), Attr("autofocus", ""),
						Attr("data-testid", "reconcile-statement-input"), Type("number"), Step(accountedit.MoneyStep(a.Currency)),
						Placeholder(uistate.T("accounts.statementBalancePh")), OnInput(p.OnStmtBal)},
						append(errAttrs(stmtErrID, stmtErrMsg), uiw.FieldValue(p.StmtBal.Get()))...)...)),
				If(stmtInvalid, P(css.Class("err"), Role("alert"), Attr("id", stmtErrID), Attr("data-testid", "reconcile-statement-invalid"), stmtErrMsg)),
				If(diffOverflow, P(css.Class("err"), Role("alert"), Attr("data-testid", "reconcile-difference-overflow"), uistate.T("calculation.unavailable"))),
				labeledField(uistate.T("accounts.reconStmtDate"),
					Input(css.Class("field"), Type("date"), Attr("data-testid", "reconcile-statement-date"),
						AriaLabel(uistate.T("accounts.reconStmtDate")), Title(uistate.T("accounts.reconStmtDateHint")), OnInput(p.OnStmtDate), uiw.FieldValue(p.StmtDate.Get()))),
				P(css.Class("t-caption", tw.TextDim, "acct-gap-below"), Attr("data-testid", "reconcile-sign-hint"),
					uistate.T(reconSignHintKey(a))),
				Div(css.Class("reconcile-summary"),
					Span(css.Class(tw.TextDim), uistate.T("accounts.clearedBalanceLabel"), fmtMoney(p.Cleared)),
					Span(ClassStr(remainingCls), Attr("data-testid", "reconcile-difference"),
						uistate.T("accounts.reconRemainingLabel"), diffLabel),
					If(reconciled, Span(css.Class(tw.TextDim), Attr("data-testid", "reconcile-confirmed"),
						uistate.T("accounts.reconciledCheck"))),
				),
			),
			// The arithmetic behind the difference. The header states a number;
			// this states which term it is in, which is the difference between a
			// lead and a dead end.
			reconcileWorksheet(app, a, strings.TrimSpace(p.StmtDate.Get()), stmtMinor, stmtOK),
			// Standing copy: what finishing a reconciliation requires. Shown while
			// the account isn't yet balanced, so the finish path is never a mystery.
			If(!reconciled, P(css.Class("t-caption", tw.TextDim, "reconcile-explain"),
				Attr("data-testid", "reconcile-explain"), uistate.T("accounts.reconExplain"))),
			// Bulk action: clear every outstanding row at once, with a preview of the
			// outcome. Undoable (Ctrl+Z) via the shared undo story.
			If(len(unclearedTxns) > 0, Div(css.Class("reconcile-markall"), Attr("data-testid", "reconcile-markall"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-markall-btn"),
					Title(uistate.T("accounts.reconMarkAllTitle")), OnClick(acts.MarkAll), uistate.T("accounts.reconMarkAll")),
				If(stmtOK, Span(css.Class("reconcile-markall-preview"), Attr("data-testid", "reconcile-markall-preview"),
					bulkPreview)))),
			If(len(unclearedTxns) > 0, Div(css.Class("acct-block-above"),
				P(css.Class("t-caption"), uistate.T("accounts.unclearedHeading")),
				Div(css.Class("rows"), MapKeyed(unclearedTxns, keyOfTxn, renderTxnRow)))),
			If(len(unclearedTxns) == 0 && !reconciled,
				P(css.Class("muted"), uistate.T("accounts.noUncleared"))),
			// The way out of a stuck difference: adjustment /
			// investigate / force-complete, offered once a balance is typed.
			If(stmtOK && !reconciled, Div(css.Class("acct-block-above"),
				Attr("data-testid", "reconcile-resolve"),
				P(css.Class("t-caption", tw.TextDim), uistate.T("accounts.reconResolveHint")),
				Div(css.Class("acct-recon-actions"),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-adjust"),
						OnClick(acts.Adjust), uistate.T("accounts.reconAdjustAction")),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-investigate"),
						OnClick(acts.Investigate), uistate.T("accounts.reconInvestigate")),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-force"),
						OnClick(acts.Force), uistate.T("accounts.reconForceAction"))))),
			If(len(histRows) > 0, Div(css.Class("acct-block-above"),
				Attr("data-testid", "reconcile-history"),
				Div(css.Class("acct-recon-hist-head"),
					P(css.Class("t-caption"), uistate.T("accounts.reconHistoryHeading")),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-reopen"),
						OnClick(acts.Reopen), uistate.T("accounts.reconReopenAction"))),
				Div(css.Class("rows"), histRows))),
		),
		Div(css.Class("modal-foot"),
			If(stmtTyped && !reconciled,
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "reconcile-save-draft"),
					OnClick(acts.SaveDraft), uistate.T("accounts.reconSaveDraft"))),
			Button(css.Class("btn"), Type("button"), OnClick(p.Cancel), uistate.T("action.cancel")),
			// Finish is the footer's primary action once the difference is zero, as Save is on Update value.
			If(reconciled, Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "reconcile-done"),
				OnClick(p.OnRecord), uistate.T("accounts.reconFinishAction"))),
		),
	)
}

// transferProps is everything the transfer-between-accounts editor reads.
type transferProps struct {
	Account  domain.Account
	App      *appstate.App
	Accounts []domain.Account

	From, To, Amount, Date, Desc ui.State[string]
	OnAmount, OnDate, OnDesc     ui.Handler
	OnSubmit, Cancel             ui.Handler
}

// transferForm is the transfer-between-accounts editor. It shares the eligibility
// rule and the cross-currency note with the page-level form via
// acctTransferOptions / acctTransferFXNote, so both entry points behave identically.
func transferForm(p transferProps) ui.Node {
	a := p.Account
	fromID, toID := p.From.Get(), p.To.Get()
	fromOpts, toOpts := acctTransferOptions(p.Accounts, fromID, toID)
	sameAcct := fromID != "" && toID != "" && fromID == toID
	// Same enable rule as the page form: no posting without a valid positive amount.
	submitDisabled := sameAcct || fromID == "" || toID == "" || !acctTransferAmountOK(p.App, fromID, p.Amount.Get())
	// The amount is in the source account's currency, so its box takes that currency's decimals.
	amountStep := accountedit.MoneyStep(a.Currency)
	for _, acc := range p.Accounts {
		if acc.ID == fromID {
			amountStep = accountedit.MoneyStep(acc.Currency)
		}
	}
	return Form(css.Class("acct-edit-form"), Attr("id", "acct-transfer-form-"+a.ID),
		AriaLabel(uistate.T("accounts.transferFormLabel")), OnSubmit(p.OnSubmit),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.transferFromLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: fromID, OnChange: func(v string) { p.From.Set(v) },
					AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "acct-xfer-from-select"})),
			labeledField(uistate.T("accounts.transferToLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: toID, OnChange: func(v string) { p.To.Set(v) },
					AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "acct-xfer-to-select"})),
			If(sameAcct, P(css.Class("err"), Role("alert"), uistate.T("accounts.transferSameAccountErr"))),
			labeledField(uistate.T("accounts.transferAmount"),
				Input(css.Class("field"), Attr("id", "acct-xfer-amt-"+a.ID), Attr("autofocus", ""),
					Type("number"), Placeholder(uistate.T("accounts.transferAmount")),
					Step(amountStep), Attr("min", amountStep), OnInput(p.OnAmount), uiw.FieldValue(p.Amount.Get()))),
			// Cross-currency note: the denomination, a converted preview, or a no-rate warning.
			acctTransferFXNote(p.App, fromID, toID, p.Amount.Get()),
			// Before/after balances for both sides (shared with the page form).
			acctTransferBalancePreview(p.App, fromID, toID, p.Amount.Get(), "", ""),
			labeledField(uistate.T("accounts.transferDateLabel"),
				Input(css.Class("field"), Type("date"), AriaLabel(uistate.T("accounts.transferDateLabel")), OnInput(p.OnDate), uiw.FieldValue(p.Date.Get()))),
			labeledField(uistate.T("accounts.transferDescLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.transferDefaultDesc")), OnInput(p.OnDesc), uiw.FieldValue(p.Desc.Get()))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(p.Cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// editFormState is everything the merged account editor reads: the account and its lists, one
// state handle per field with its change handler, and the save and cancel actions. The
// parent builds it so every hook sits in one stable place and the editor itself only renders.
type editFormState struct {
	Account domain.Account
	// Decimals is the account currency's decimal places.
	Decimals int
	// CurBal is the current balance as stored; BalancesOK is false when it could not be calculated.
	CurBal     money.Money
	BalancesOK bool
	// Balances is the pre-built 4-way balance and provenance strip.
	Balances   ui.Node
	Members    []domain.Member
	Accounts   []domain.Account
	Categories []domain.Category
	CustomDefs []customfields.Def
	// ValueFirst opens the editor on the value field with the details folded away, as
	// "Update balance" does; otherwise the name is focused and the details are open.
	ValueFirst bool

	Name, Type, VarName, Owner, Balance, CreditLimit, APR, MinPayment, DueDay, Lender, Institution ui.State[string]
	ExpectedReturn, APY, Liquidity, Stability, LockUntil, Notes, SetBalAmt, SetBalCat, SetBalAsOf  ui.State[string]
	StatementDay, InstitutionID, BeneficiaryNote, RevalueDays, FreshSnooze                         ui.State[string]
	// Problem is what is wrong with the fields after a refused save, ErrID the id of the error line,
	// and FoldForced keeps the details open once a refused save has found its problem in them.
	Problem                                                      formProblem
	ErrID                                                        string
	FoldForced                                                   bool
	AdvOpen, AsLiability, SplitOwn, ExcludeNetWorth, FreshExempt ui.State[bool]
	Shares                                                       ui.State[map[string]int]
	CustomVals                                                   ui.State[map[string]string]

	OnName, OnVarName, OnBalance, OnCreditLimit, OnAPR, OnMinPayment, OnDueDay, OnLender ui.Handler
	OnExpectedReturn, OnAPY, OnLiquidity, OnStability, OnLockUntil                       ui.Handler
	OnSetBalAmt, OnSetBalAsOf, OnStatementDay, OnRevalueDays, OnFreshSnooze              ui.Handler
	OnToggleAdv, OnToggleAsLiability, OnToggleSplit, OnToggleExcludeNW                   ui.Handler
	OnToggleFreshExempt, OpenInstitutions, OnSave, OnCancel                              ui.Handler
	// The free-text fields go through uiw's self-owning field components, which own the
	// event hook themselves and so take a plain func.
	OnInstitution, OnInstitutionID, OnNotes, OnBeneficiaryNote func(string)
	OnCustomEdit                                               func(key, value string)
}

// editForm is the full inline-edit editor: name, owner, balances, type-specific
// attributes, institution and custom fields.
func editForm(st editFormState) ui.Node {
	a, dec, members, accounts := st.Account, st.Decimals, st.Members, st.Accounts
	saveErrID := st.ErrID
	idPrefix := "acct-edit-" + a.ID
	moneyStep := accountedit.MoneyStep(a.Currency)
	// box is one input of the editor: its id, the danger marking when the last save refused it, and
	// the change handler. Every box registers no hook of its own, only the handler it is given.
	box := func(inputType string, field accountedit.Field, handler ui.Handler, value string, opts ...any) ui.Node {
		id := accountFieldInputID(idPrefix, field)
		args := append([]any{css.Class("field", fieldInvalidName(st.Problem, id)), Type(inputType), Attr("id", id)}, opts...)
		args = append(args, fieldInvalidAttrs(st.Problem, id, saveErrID)...)
		return Input(append(args, OnInput(handler), uiw.FieldValue(value))...)
	}
	// The type is editable; the shown attribute fields follow the SELECTED type's
	// class (not the account's stored class), so switching e.g. a line of credit to a
	// credit card, or a liability to an asset, reveals the right fields live.
	selType := domain.AccountType(st.Type.Get())
	isOther := selType == domain.TypeOther
	// "Other" accounts can be flagged as a liability (debt) explicitly; that reveals the
	// liability fields (min payment, due day, …) the debt formulas use.
	isLiab := accountedit.ClassOf(selType, st.AsLiability.Get()) == domain.ClassLiability
	typeOptions := uiw.OptionsFrom(domain.AllAccountTypes,
		func(t domain.AccountType) string { return string(t) },
		func(t domain.AccountType) string { return humanizeType(string(t)) },
		st.Type.Get())
	// The name field is auto-focused for the "Edit" entry; when opened from "Update
	// value" the value section takes focus instead, so the fast path is type-and-save.
	nameInput := []any{css.Class("field", fieldInvalidName(st.Problem, idPrefix)), Attr("id", idPrefix), Type("text"),
		Placeholder(uistate.T("common.name")), OnInput(st.OnName)}
	nameInput = append(nameInput, fieldInvalidAttrs(st.Problem, idPrefix, saveErrID)...)
	if !st.ValueFirst {
		nameInput = append(nameInput, Attr("autofocus", ""))
	}
	// Asset-only fields (returns, liquidity and similar) show when the account is not a
	// liability and the advanced section is open. Revalue days also need a revaluable type.
	showAssetAdvanced := !isLiab && st.AdvOpen.Get()
	showRevalue := showAssetAdvanced && isRevaluableType(selType)
	return Form(css.Class("acct-edit-form"), Attr("novalidate", true), OnSubmit(st.OnSave),
		// Fields scroll inside .modal-scroll; the Cancel/Save bar is pinned below in
		// .modal-foot (FlushBody on the host), so it never scrolls off a tall form.
		Div(css.Class("modal-scroll"),
			// Merged "update value / balance" section, up top so the marquee account action
			// (record a new value) is the fast path; leaving it blank keeps the balance as-is.
			st.Balances,
			acctValueUpdateSection(a, statedBalance(a, st.CurBal), st.BalancesOK, dec, st.SetBalAmt, st.SetBalCat, st.SetBalAsOf, st.OnSetBalAmt, st.OnSetBalAsOf, st.Categories, st.ValueFirst, st.Problem, saveErrID),
			// Opened from the balance figure ("Update balance"), the account
			// METADATA folds behind a native details disclosure so the dialog matches
			// its label — value on top, everything else opt-in. The Edit entry keeps
			// the full form expanded.
			foldAccountDetails(st.ValueFirst, st.FoldForced, Fragment(
				labeledField(uistate.T("common.name"), Input(append(nameInput, uiw.FieldValue(st.Name.Get()))...)),
				labeledField(uistate.T("accounts.typeLabel"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   typeOptions,
						Selected:  st.Type.Get(),
						OnChange:  func(v string) { st.Type.Set(v) },
						AriaLabel: uistate.T("accounts.typeLabel"),
						TestID:    "acct-edit-type-select",
					})),
				// "Other" is the catch-all type with no natural asset/liability class, so let the
				// user say whether it's a debt — that's what the net-worth and debt formulas read.
				If(isOther, Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-edit-as-liability"), CheckedIf(st.AsLiability.Get()), OnChange(st.OnToggleAsLiability)),
					Div(css.Class("row-main"),
						Span(uistate.T("accounts.countAsLiability")),
						Span(css.Class("row-meta", tw.TextDim), uistate.T("accounts.countAsLiabilityHint"))))),
				labeledField(uistate.T("common.owner"),
					uiw.SelectInput(uiw.SelectInputProps{Options: ownerSelectOptions(members, st.Owner.Get()), Selected: st.Owner.Get(),
						OnChange: func(v string) { st.Owner.Set(v) }, AriaLabel: uistate.T("common.owner")})),
				// Ownership preview: a changed owner moves this account's worth
				// between people's net-worth views; say so before the save, not after.
				If(st.Owner.Get() != a.OwnerID,
					P(css.Class("muted"), Attr("data-testid", "acct-owner-preview"),
						uistate.T("accounts.ownerMovePreview", fmtMoney(statedBalance(a, st.CurBal)),
							ownerDisplayName(members, a.OwnerID), ownerDisplayName(members, st.Owner.Get())))),
				If(accountedit.SplitAvailable(len(members)), func() ui.Node {
					memberIDs := memberIDsOf(members)
					shares := st.Shares.Get()
					// A figure that is not a whole percentage leaves the member's share as it was;
					// the new map still re-renders the row, which puts the previous figure back.
					onShareChange := func(memberID string, valStr string) {
						st.Shares.Set(accountedit.WithShare(st.Shares.Get(), memberID, valStr))
					}
					return Div(
						Button(css.Class("btn cf-adv-toggle"), Type("button"), AriaExpanded(st.SplitOwn.Get()), OnClick(st.OnToggleSplit),
							IfElse(st.SplitOwn.Get(), Text(uistate.T("account.splitOwnership")+" ▴"), Text(uistate.T("account.splitOwnership")+" ▾"))),
						If(st.SplitOwn.Get(), Div(
							P(css.Class("t-caption", tw.TextDim), uistate.T("account.splitOwnershipHint")),
							MapKeyed(members, func(m domain.Member) any { return m.ID }, func(m domain.Member) ui.Node {
								return ui.CreateElement(OwnerShareRow, ownerShareRowProps{Member: m, Share: shares[m.ID], OnChange: onShareChange})
							}),
							If(!accountedit.SharesValid(shares, memberIDs), P(css.Class("err"), uistate.T("account.shareSumError", accountedit.ShareTotal(shares, memberIDs)))),
						)),
					)
				}()),
				labeledField(uistate.T("accounts.openingBalance"),
					box("number", accountedit.FieldOpening, st.OnBalance, st.Balance.Get(), Placeholder(uistate.T("accounts.openingBalance")), Step(moneyStep))),
				If(isLiab, labeledField(uistate.T("accounts.creditLimit"),
					box("number", accountedit.FieldCreditLimit, st.OnCreditLimit, st.CreditLimit.Get(), Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Step(moneyStep)))),
				If(isLiab, labeledField(uistate.T("accounts.apr"),
					box("number", accountedit.FieldAPR, st.OnAPR, st.APR.Get(), Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Step(rateStep)))),
				If(isLiab, labeledField(uistate.T("accounts.minPayment"),
					box("number", accountedit.FieldMinPayment, st.OnMinPayment, st.MinPayment.Get(), Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Step(moneyStep)))),
				If(isLiab, labeledField(uistate.T("accounts.dueDay"),
					box("number", accountedit.FieldDueDay, st.OnDueDay, st.DueDay.Get(), Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay"))))),
				// Statement-close day: the day the billing cycle closes, distinct from the
				// payment due day above. Powers real due dates in the bill calendar and a tighter
				// on-time payment window.
				If(isLiab, labeledField(uistate.T("accountsstmt.statementDay"),
					box("number", accountedit.FieldStatementDay, st.OnStatementDay, st.StatementDay.Get(), Attr("min", "1"), Attr("max", "31"), Step("1"), Attr("data-testid", "acct-edit-statement-day"), Placeholder(uistate.T("accountsstmt.statementDay"))))),
				If(isLiab, labeledField(uistate.T("accounts.lender"),
					Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), OnInput(st.OnLender), uiw.FieldValue(st.Lender.Get())))),
				// One institution section. The directory picker leads (it colors the row
				// and powers multi-institution analytics); picking an entry also sets the
				// free-text label, which only shows as a fallback input while nothing is
				// picked — so the modal never presents two competing "Institution" fields.
				labeledField(uistate.T("accounts.institution"),
					Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
						uiw.SelectInput(uiw.SelectInputProps{Options: institutionPickerOptions(appstate.Default), Selected: st.InstitutionID.Get(),
							OnChange: st.OnInstitutionID, AriaLabel: uistate.T("accounts.institution"), TestID: "acct-edit-institution-select"}),
						Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-edit-manage-institutions"),
							OnClick(st.OpenInstitutions), uistate.T("accounts.manageInstitutionsLink")))),
				If(st.InstitutionID.Get() == "", labeledField(uistate.T("accounts.institutionFreeLabel"),
					uiw.Combobox(uiw.SuggestProps{Value: st.Institution.Get(), Placeholder: uistate.T("accounts.institutionHint"),
						AriaLabel: uistate.T("accounts.institutionFreeLabel"), OnInput: st.OnInstitution, Options: domain.UniqueInstitutions(accounts), ListID: "inst-list-edit-" + a.ID}))),
				// Free-text notes. Plain text (rides the dataset export/sync) — logins/secrets
				// go in the encrypted credential vault, never here.
				labeledField(uistate.T("accounts.notes"),
					uiw.TextAreaInput(uiw.TextFieldProps{Value: st.Notes.Get(), Placeholder: uistate.T("accounts.notesPlaceholder"),
						AriaLabel: uistate.T("accounts.notes"), OnInput: st.OnNotes})),
				// The form is grouped: identity and money essentials stay visible;
				// plumbing (variable name), estate planning and tuning sit behind one
				// disclosure, available for every class (a mortgage has a beneficiary too).
				Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "acct-edit-more"), AriaExpanded(st.AdvOpen.Get()), OnClick(st.OnToggleAdv),
					IfElse(st.AdvOpen.Get(), Text(uistate.T("accounts.hideAdvanced")), Text(uistate.T("accounts.showAdvanced")))),
				// Optional explicit variable name for formulas/widgets (engine plumbing).
				If(st.AdvOpen.Get(), labeledField(uistate.T("accounts.varNameLabel"),
					entityVarField(accountVarKind, accountVarEntities(accounts), a.ID, "acct-edit-varname-"+a.ID, "account-varname-warn", st.VarName.Get(), st.Name.Get(), st.OnVarName))),
				// Who inherits this account: a plain beneficiary / transfer-on-death
				// note surfaced (compassionately, never with a password) in the estate
				// emergency pack.
				If(st.AdvOpen.Get(), Fragment(
					labeledField(uistate.T("accounts.beneficiaryNoteLabel"),
						uiw.TextAreaInput(uiw.TextFieldProps{Value: st.BeneficiaryNote.Get(), Placeholder: uistate.T("accounts.beneficiaryNotePh"),
							AriaLabel: uistate.T("accounts.beneficiaryNoteLabel"), OnInput: st.OnBeneficiaryNote})),
					P(css.Class("t-caption", tw.TextDim, "acct-hint-tuck"), uistate.T("accounts.beneficiaryNoteHint")),
					// Keep this account visible in its class views but out of net worth.
					Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
						Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-edit-exclude-networth"), CheckedIf(st.ExcludeNetWorth.Get()), OnChange(st.OnToggleExcludeNW)),
						Div(css.Class("row-main"),
							Span(uistate.T("accountsstmt.excludeNetWorth")),
							Span(css.Class("row-meta", tw.TextDim), uistate.T("accountsstmt.excludeNetWorthHint")))),
					// Stale-balance reminders: opt out for good, or snooze until a date.
					Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
						Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-edit-fresh-exempt"), CheckedIf(st.FreshExempt.Get()), OnChange(st.OnToggleFreshExempt)),
						Div(css.Class("row-main"),
							Span(uistate.T("accounts.freshExempt")),
							Span(css.Class("row-meta", tw.TextDim), uistate.T("accounts.freshExemptHint")))),
					If(!st.FreshExempt.Get(), labeledField(uistate.T("accounts.freshSnoozeLabel"),
						box("date", accountedit.FieldSnoozeUntil, st.OnFreshSnooze, st.FreshSnooze.Get(), Attr("data-testid", "acct-edit-fresh-snooze"),
							AriaLabel(uistate.T("accounts.freshSnoozeLabel")), Title(uistate.T("accounts.freshSnoozeHint"))))),
				)),
				If(showAssetAdvanced, labeledField(uistate.T("accounts.expReturn"),
					box("number", accountedit.FieldExpectedRet, st.OnExpectedReturn, st.ExpectedReturn.Get(), Title(uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Step(rateStep)))),
				If(showAssetAdvanced, labeledField(uistate.T("accounts.apyLabel"),
					box("number", accountedit.FieldAPY, st.OnAPY, st.APY.Get(), Title(uistate.T("accounts.apyHint")), Attr("data-testid", "account-apy"), Placeholder(uistate.T("accounts.apyLabel")), Step(rateStep)))),
				// Scores run from 0 to 100, the scale the domain, validation and allocation use.
				// The inputs enforce that range natively, so Save never fails on a seeded score.
				// Keep these inputs within that range.
				If(showAssetAdvanced, labeledField(uistate.T("accounts.liquidity"),
					box("number", accountedit.FieldLiquidity, st.OnLiquidity, st.Liquidity.Get(), Attr("min", "0"), Attr("max", "100"), Step("1"), Title(uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity"))))),
				If(showAssetAdvanced, labeledField(uistate.T("accounts.stability"),
					box("number", accountedit.FieldStability, st.OnStability, st.Stability.Get(), Attr("min", "0"), Attr("max", "100"), Step("1"), Title(uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability"))))),
				If(showAssetAdvanced, labeledField(uistate.T("accounts.lockUntilEdit"),
					box("date", accountedit.FieldLockUntil, st.OnLockUntil, st.LockUntil.Get(), AriaLabel(uistate.T("accounts.lockUntilEdit")), Title(uistate.T("accounts.lockUntilEdit"))))),
				// How often to refresh a periodically estimated asset (property, vehicle,
				// crypto, or an "Other" you're treating as one) — 0/blank keeps the type's
				// default cadence (internal/revalue). The freshness machinery reads
				// Account.RevalueDays; this input sets it.
				If(showRevalue, labeledField(uistate.T("accounts.revalueDaysLabel"),
					box("number", accountedit.FieldRevalueDays, st.OnRevalueDays, st.RevalueDays.Get(), Attr("min", "1"), Step("1"), Attr("data-testid", "acct-edit-revalue-days"),
						Placeholder(uistate.T("accounts.revalueDaysPh"))))),
				If(showRevalue, P(css.Class("t-caption", tw.TextDim, "acct-hint-tuck"), uistate.T("accounts.revalueDaysHint"))),
				If(len(st.CustomDefs) > 0, Fragment(
					P(css.Class("t-caption", tw.TextDim, "acct-hint-label"), uistate.T("accounts.customFieldsLabel")),
					MapKeyed(st.CustomDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
						return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: st.CustomVals.Get()[d.Key], OnChange: st.OnCustomEdit}))
					}),
				)),
			)),
			// The refusal message sits outside the fold, so it is visible whichever way the
			// editor opened.
			errText(saveErrID, st.Problem.Message),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(st.OnCancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

// foldAccountDetails wraps the account-metadata fields. When the value comes first
// ("Update balance") they fold behind a native <details> disclosure so the dialog is as
// narrow as its label; from Edit they render expanded. A native disclosure keeps this
// stateless — no extra hooks, and the field nodes are built identically in both modes so
// hook order never shifts. forceOpen opens the fold for a problem inside it, and the person can
// still close it by hand afterwards.
func foldAccountDetails(valueFirst, forceOpen bool, fields ui.Node) ui.Node {
	if !valueFirst {
		return fields
	}
	return Details(css.Class("csv-help acct-edit-fold"), Attr("data-testid", "acct-edit-details-fold"), AttrIf(forceOpen, "open", ""),
		Summary(uistate.T("accounts.editDetailsFold")),
		fields,
	)
}

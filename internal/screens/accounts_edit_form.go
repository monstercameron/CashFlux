// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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

	// Current balances for the setbal delta preview + adjustment.
	var curBal, curCleared money.Money
	if app != nil {
		curBal, _ = ledger.Balance(a, app.Transactions())
		curCleared, _ = ledger.ClearedBalance(a, app.Transactions())
	}

	// ---- setbal state ----
	setBalAmtS := ui.UseState("")
	setBalCatS := ui.UseState("")
	onSetBalAmt := ui.UseEvent(func(v string) { setBalAmtS.Set(v) })

	// ---- transfer state ----
	xferFromS := ui.UseState(a.ID)
	xferToS := ui.UseState("")
	xferAmtS := ui.UseState("")
	xferDateS := ui.UseState(time.Now().Format("2006-01-02"))
	xferDescS := ui.UseState("")
	onXferAmt := ui.UseEvent(func(v string) { xferAmtS.Set(v) })
	onXferDate := ui.UseEvent(func(v string) { xferDateS.Set(v) })
	onXferDesc := ui.UseEvent(func(v string) { xferDescS.Set(v) })

	// ---- reconcile state ----
	stmtBalS := ui.UseState("")
	onStmtBal := ui.UseEvent(func(v string) { stmtBalS.Set(v) })

	// ---- edit state ----
	instInit := a.Institution
	if instInit == "" {
		instInit = a.Lender
	}
	lockISO := ""
	if !a.LockUntil.IsZero() {
		lockISO = dateutil.FormatDate(a.LockUntil)
	}
	nameS := ui.UseState(a.Name)
	typeS := ui.UseState(string(a.Type))
	ev := useEntityVarField(accountVarKind, nameS, a.VarName)
	balS := ui.UseState(money.FormatMinor(a.OpeningBalance.Amount, dec))
	climS := ui.UseState(moneyMajorOrEmpty(a.CreditLimit, dec))
	aprS := ui.UseState(floatOrEmpty(a.InterestRateAPR))
	minpS := ui.UseState(moneyMajorOrEmpty(a.MinPayment, dec))
	dueS := ui.UseState(intOrEmpty(a.DueDayOfMonth))
	stmtDayS := ui.UseState(intOrEmpty(a.StatementDay))
	exclNWS := ui.UseState(a.ExcludeFromNetWorth)
	lenderS := ui.UseState(a.Lender)
	institutionS := ui.UseState(instInit)
	// AC10: the structured institution-directory picker, separate from the legacy
	// free-text Institution/Lender field above (which stays as a fallback label).
	instIDS := ui.UseState(a.InstitutionID)
	// AC16: a beneficiary / transfer-on-death note, surfaced (never a password) in
	// the estate emergency pack.
	beneficiaryNoteS := ui.UseState(a.BeneficiaryNote)
	// AC5: the per-account revaluation-cadence override (0 = the type default).
	revalueDaysS := ui.UseState(intOrEmpty(a.RevalueDays))
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
	asLiabS := ui.UseState(a.Type == domain.TypeOther && a.Class == domain.ClassLiability)
	splitOwnS := ui.UseState(len(a.OwnershipShares) > 0)
	sharesMapS := ui.UseState(cloneSharesMap(a.OwnershipShares))
	customEditVals := ui.UseState(customMapToStrings(a.Custom))
	notesS := ui.UseState(a.Notes)
	onNotes := ui.UseEvent(func(v string) { notesS.Set(v) })
	onBal := ui.UseEvent(func(v string) { balS.Set(v) })
	onClim := ui.UseEvent(func(v string) { climS.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprS.Set(v) })
	onMinp := ui.UseEvent(func(v string) { minpS.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onStmtDay := ui.UseEvent(func(v string) { stmtDayS.Set(v) })
	onToggleExclNW := ui.UseEvent(func() { exclNWS.Set(!exclNWS.Get()) })
	onLender := ui.UseEvent(func(v string) { lenderS.Set(v) })
	onInstitution := ui.UseEvent(func(v string) { institutionS.Set(v) })
	onInstID := func(v string) { instIDS.Set(v) }
	onBeneficiaryNote := ui.UseEvent(func(v string) { beneficiaryNoteS.Set(v) })
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
	// AC10: "Manage institutions" jumps from the edit form straight to the
	// institution-directory modal. Declared unconditionally here (not inside
	// editForm) so the hook count stays stable across every switch branch below.
	institutionsMgrAtom := uistate.UseInstitutionsManager()
	openInstitutionsFromEditor := ui.UseEvent(Prevent(func() { institutionsMgrAtom.Set(true) }))
	doTransfer := ui.UseEvent(Prevent(func() {
		if xferFromS.Get() == xferToS.Get() || xferToS.Get() == "" {
			return
		}
		cbs.OnTransfer(xferFromS.Get(), xferToS.Get(), xferAmtS.Get(), xferDateS.Get(), xferDescS.Get())
		done()
	}))
	saveEdit := ui.UseEvent(Prevent(func() {
		// Block the save on a variable-name collision; the field already shows the warning
		// inline (there's no separate error line in this editor).
		if entityVarCollision(accountVarKind, accountVarEntities(app.Accounts()), a.ID, ev.VarName.Get(), nameS.Get()) != "" {
			return
		}
		cp := a
		cp.Name = strings.TrimSpace(nameS.Get())
		cp.VarName = strings.TrimSpace(ev.VarName.Get())
		cp.OwnerID = ownerS.Get()
		if splitOwnS.Get() {
			cp.OwnershipShares = cloneSharesMap(sharesMapS.Get())
		} else {
			cp.OwnershipShares = nil
		}
		if ownerS.Get() == domain.GroupOwnerID {
			cp.Scope = domain.ScopeShared
		} else {
			cp.Scope = domain.ScopeIndividual
		}
		if amt, err := money.ParseMinor(strings.TrimSpace(balS.Get()), dec); err == nil {
			cp.OpeningBalance = money.New(amt, a.Currency)
		}
		// Account type is editable (e.g. a line of credit → credit card, or an asset
		// reclassified as a liability). The class follows the type, and the fields
		// that don't belong to the new class are cleared so a reclassified account
		// doesn't carry stale credit-limit/lock-until data.
		selType := domain.AccountType(typeS.Get())
		cp.Type = selType
		cp.Class = selType.Class()
		// "Other" accounts can be explicitly counted as a liability (debt) via the toggle,
		// overriding the type's natural class so the liability formulas include them.
		if selType == domain.TypeOther && asLiabS.Get() {
			cp.Class = domain.ClassLiability
		}
		if cp.Class == domain.ClassLiability {
			cp.CreditLimit = parseMoneyOrZero(climS.Get(), dec, a.Currency)
			cp.InterestRateAPR = textutil.ParseFloat(aprS.Get())
			cp.MinPayment = parseMoneyOrZero(minpS.Get(), dec, a.Currency)
			cp.DueDayOfMonth = textutil.ParseInt(dueS.Get())
			cp.StatementDay = textutil.ParseInt(stmtDayS.Get())
			cp.Lender = strings.TrimSpace(lenderS.Get())
			cp.ExpectedReturnAPR = 0
			cp.APY = 0
			cp.LiquidityScore = 0
			cp.StabilityScore = 0
			cp.LockUntil = time.Time{}
		} else {
			cp.ExpectedReturnAPR = textutil.ParseFloat(retS.Get())
			cp.APY = textutil.ParseFloat(apyS.Get())
			cp.LiquidityScore = textutil.ParseInt(liqS.Get())
			cp.StabilityScore = textutil.ParseInt(stabS.Get())
			if lu := strings.TrimSpace(lockS.Get()); lu != "" {
				if d, err := dateutil.ParseDate(lu); err == nil {
					cp.LockUntil = d
				}
			} else {
				cp.LockUntil = time.Time{}
			}
			cp.CreditLimit = money.Money{}
			cp.InterestRateAPR = 0
			cp.MinPayment = money.Money{}
			cp.DueDayOfMonth = 0
			cp.StatementDay = 0
			cp.Lender = ""
		}
		// AC11: exclude-from-net-worth applies to any account class.
		cp.ExcludeFromNetWorth = exclNWS.Get()
		cp.Institution = titleCaseWords(strings.TrimSpace(institutionS.Get()))
		cp.InstitutionID = instIDS.Get()
		cp.Notes = strings.TrimSpace(notesS.Get())
		// AC16: beneficiary / transfer-on-death note — plain text, never a password.
		cp.BeneficiaryNote = strings.TrimSpace(beneficiaryNoteS.Get())
		// AC5: revaluation-cadence override (0 = the account type's default cadence).
		cp.RevalueDays = textutil.ParseInt(revalueDaysS.Get())
		if defs := app.CustomFieldDefsFor("account"); len(defs) > 0 {
			cp.Custom = customValuesToMap(defs, customEditVals.Get())
		}
		// Merged update-value + edit: when the user typed a new current value, OnSetBalance
		// persists cp (all edits) AND posts the balance adjustment + freshens BalanceAsOf in
		// one write — so we must NOT also call OnSave (that would double-write the account and
		// clobber the adjustment's freshness). When the value field is blank, it's a plain edit.
		if strings.TrimSpace(setBalAmtS.Get()) != "" {
			cbs.OnSetBalance(cp, curBal, setBalAmtS.Get(), setBalCatS.Get())
		} else {
			cbs.OnSave(cp)
		}
		done()
	}))

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	switch props.Mode {
	case uistate.AcctEditModeReconcile:
		return reconcileForm(a, curCleared, dec, stmtBalS, onStmtBal, cancel, cbs.OnRefresh, app)
	case uistate.AcctEditModeTransfer:
		return transferForm(a, app.Accounts(), xferFromS, xferToS, xferAmtS, xferDateS, xferDescS, onXferAmt, onXferDate, onXferDesc, doTransfer, cancel)
	default:
		// The edit and "update value / balance" actions share one merged editor: it can
		// update the current value AND edit the account's details in a single save. When
		// opened from "Update value" (setbal mode) the value field is auto-focused; from
		// "Edit" the name field is. Both persist through the same saveEdit handler.
		focusValue := props.Mode == uistate.AcctEditModeSetBal
		return editForm(a, dec, curBal, app.Members(), app.Accounts(), app.Categories(), app.CustomFieldDefsFor("account"),
			nameS, typeS, ev.VarName, ownerS, balS, climS, aprS, minpS, dueS, lenderS, institutionS, retS, apyS, liqS, stabS, lockS, notesS,
			setBalAmtS, setBalCatS,
			editAdvOpen, asLiabS, splitOwnS, sharesMapS, customEditVals,
			ev.OnName, ev.OnVarName, onBal, onClim, onApr, onMinp, onDue, onLender, onInstitution, onRet, onApy, onLiq, onStab, onLock,
			onToggleEditAdv, onToggleAsLiab, onToggleSplitOwn, onNotes, onSetBalAmt, onCustomEdit, saveEdit, cancel, focusValue,
			acctEditExtra{
				stmtDayS: stmtDayS, exclNWS: exclNWS, onStmtDay: onStmtDay, onToggleExclNW: onToggleExclNW,
				instIDS: instIDS, onInstID: onInstID, openInstitutions: openInstitutionsFromEditor,
				beneficiaryNoteS: beneficiaryNoteS, onBeneficiaryNote: onBeneficiaryNote,
				revalueDaysS: revalueDaysS, onRevalueDays: onRevalueDays,
			})
	}
}

// acctValueUpdateSection is the "update value / balance" block folded into the top of
// the merged editor (it replaced the standalone update-balance modal). Typing a new
// value records an adjustment so the computed balance matches; leaving it blank keeps
// the balance as-is (a plain edit). It shows the current balance for context, a live
// delta preview, and — once a value is entered — the category for the adjustment.
func acctValueUpdateSection(a domain.Account, curBal money.Money, dec int, setBalAmtS, setBalCatS ui.State[string], onSetBalAmt ui.Handler, categories []domain.Category, focusValue bool) ui.Node {
	rawAmt := strings.TrimSpace(setBalAmtS.Get())
	var deltaNode ui.Node = Fragment()
	if rawAmt != "" {
		if targetMinor, parseErr := money.ParseMinor(rawAmt, dec); parseErr == nil {
			dp := reconcile.PreviewDelta(curBal.Amount, targetMinor)
			sign := ""
			if dp.AdjustmentMinor > 0 {
				sign = "+"
			}
			adjLabel := sign + money.FormatMinor(dp.AdjustmentMinor, dec)
			if dp.NeedsAdjustment {
				deltaNode = P(css.Class("t-caption acct-value-delta"), Attr("data-testid", "setbal-delta-preview"),
					uistate.T("accounts.setBalanceDeltaPreview",
						fmtMoney(money.New(curBal.Amount, a.Currency)),
						fmtMoney(money.New(targetMinor, a.Currency)), adjLabel))
			} else {
				deltaNode = P(css.Class("t-caption acct-value-delta"), Attr("data-testid", "setbal-delta-preview"),
					uistate.T("accounts.setBalanceNoAdjNeeded"))
			}
		}
	}
	catOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.setBalanceNoCategory")}}
	for _, c := range categories {
		catOpts = append(catOpts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	valInput := []any{css.Class("field"), Attr("id", "acct-setbal-"+a.ID), Attr("data-testid", "acct-setbal-input"),
		Type("number"), Placeholder(uistate.T("accounts.setBalanceAmount")),
		Value(setBalAmtS.Get()), Step("0.01"), OnInput(onSetBalAmt)}
	if focusValue {
		valInput = append(valInput, Attr("autofocus", ""))
	}
	return Div(css.Class("acct-value-section"), Attr("data-testid", "acct-value-section"),
		labeledField(uistate.T(updateActionKey(a.Type)), Input(valInput...)),
		Span(css.Class("t-caption acct-value-now", tw.TextDim), Attr("data-testid", "acct-value-now"),
			uistate.T("accounts.currentValueNow", fmtMoney(curBal))),
		deltaNode,
		If(rawAmt != "", labeledField(uistate.T("accounts.setBalanceCategoryLabel"),
			uiw.SelectInput(uiw.SelectInputProps{Options: catOpts, Selected: setBalCatS.Get(),
				OnChange: func(v string) { setBalCatS.Set(v) }, AriaLabel: uistate.T("accounts.setBalanceCategoryLabel"), TestID: "setbal-cat-select"}))),
		If(isValuationType(a.Type), P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "valuation-manual-note"),
			uistate.T("accounts.valuationManualNote"))),
	)
}

// reconcileForm is the reconcile-to-statement editor.
func reconcileForm(a domain.Account, curCleared money.Money, dec int, stmtBalS ui.State[string], onStmtBal, cancel ui.Handler, onRefresh func(domain.Account), app *appstate.App) ui.Node {
	var allTxns []domain.Transaction
	if app != nil {
		allTxns = app.Transactions()
	}
	stmtMinor, _ := money.ParseMinor(strings.TrimSpace(stmtBalS.Get()), dec)
	result := reconcile.Diff(curCleared.Amount, stmtMinor)
	diffLabel := money.FormatMinor(result.DifferenceMinor, dec)
	if result.DifferenceMinor > 0 {
		diffLabel = "+" + diffLabel
	}
	var unclearedTxns []domain.Transaction
	for _, t := range allTxns {
		if t.AccountID == a.ID && !t.Cleared {
			unclearedTxns = append(unclearedTxns, t)
		}
	}
	onToggleClear := func(t domain.Transaction) {
		if app == nil {
			return
		}
		t.Cleared = !t.Cleared
		_ = app.PutTransaction(t)
		onRefresh(a) // bumps the data revision → this modal re-derives the difference
	}
	keyOfTxn := func(t domain.Transaction) any { return t.ID }
	renderTxnRow := func(t domain.Transaction) ui.Node {
		return ui.CreateElement(ReconcileTxnRow, reconcileTxnRowProps{Txn: t, Currency: a.Currency, OnToggle: onToggleClear})
	}
	return Div(css.Class("acct-edit-form"), Attr("data-testid", "reconcile-statement-mode"),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.statementBalance"),
				Input(css.Class("field"), Attr("id", "acct-reconcile-stmt-"+a.ID), Attr("autofocus", ""),
					Attr("data-testid", "reconcile-statement-input"), Type("number"), Step("0.01"),
					Placeholder(uistate.T("accounts.statementBalancePh")), Value(stmtBalS.Get()), OnInput(onStmtBal))),
			Div(Style(map[string]string{"margin": "0.5rem 0"}),
				Span(Style(map[string]string{"margin-right": "1rem"}), uistate.T("accounts.clearedBalanceLabel"), fmtMoney(curCleared)),
				Span(Attr("data-testid", "reconcile-difference"), uistate.T("accounts.differenceLabel"), diffLabel),
				If(result.Reconciled, Span(Style(map[string]string{"margin-left": "1rem", "color": "var(--cf-pos)", "font-weight": "bold"}),
					Attr("data-testid", "reconcile-confirmed"), uistate.T("accounts.reconciledCheck"))),
			),
			If(result.Reconciled, Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "reconcile-done"),
				OnClick(cancel), uistate.T("action.done"))),
			If(len(unclearedTxns) > 0, Div(Style(map[string]string{"margin-top": "0.75rem"}),
				P(css.Class("t-caption"), uistate.T("accounts.unclearedHeading")),
				Div(css.Class("rows"), MapKeyed(unclearedTxns, keyOfTxn, renderTxnRow)))),
			If(len(unclearedTxns) == 0 && !result.Reconciled,
				P(css.Class("muted"), uistate.T("accounts.noUncleared"))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
		),
	)
}

// transferForm is the transfer-between-accounts editor.
func transferForm(a domain.Account, accounts []domain.Account, xferFromS, xferToS, xferAmtS, xferDateS, xferDescS ui.State[string], onXferAmt, onXferDate, onXferDesc, doTransfer, cancel ui.Handler) ui.Node {
	fromID, toID := xferFromS.Get(), xferToS.Get()
	fromOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferFromPlaceholder")}}
	toOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.transferToPlaceholder")}}
	for _, ac := range accounts {
		if ac.Archived {
			continue
		}
		label := ac.Name + " (" + ac.Currency + ")"
		if ac.ID != toID {
			fromOpts = append(fromOpts, uiw.SelectOption{Value: ac.ID, Label: label})
		}
		if ac.ID != fromID {
			toOpts = append(toOpts, uiw.SelectOption{Value: ac.ID, Label: label})
		}
	}
	sameAcct := fromID != "" && toID != "" && fromID == toID
	submitDisabled := sameAcct || fromID == "" || toID == ""
	return Form(css.Class("acct-edit-form"), Attr("id", "acct-transfer-form-"+a.ID),
		Attr("aria-label", uistate.T("accounts.transferFormLabel")), OnSubmit(doTransfer),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("accounts.transferFromLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: fromOpts, Selected: fromID, OnChange: func(v string) { xferFromS.Set(v) },
					AriaLabel: uistate.T("accounts.transferFromLabel"), TestID: "acct-xfer-from-select"})),
			labeledField(uistate.T("accounts.transferToLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: toOpts, Selected: toID, OnChange: func(v string) { xferToS.Set(v) },
					AriaLabel: uistate.T("accounts.transferToLabel"), TestID: "acct-xfer-to-select"})),
			If(sameAcct, P(css.Class("err"), Attr("role", "alert"), uistate.T("accounts.transferSameAccountErr"))),
			labeledField(uistate.T("accounts.transferAmount"),
				Input(css.Class("field"), Attr("id", "acct-xfer-amt-"+a.ID), Attr("autofocus", ""),
					Type("number"), Placeholder(uistate.T("accounts.transferAmount")), Value(xferAmtS.Get()),
					Step("0.01"), Attr("min", "0.01"), OnInput(onXferAmt))),
			labeledField(uistate.T("accounts.transferDateLabel"),
				Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.transferDateLabel")),
					Value(xferDateS.Get()), OnInput(onXferDate))),
			labeledField(uistate.T("accounts.transferDescLabel"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.transferDefaultDesc")),
					Value(xferDescS.Get()), OnInput(onXferDesc))),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// editForm is the full inline-edit editor (name, owner, balances, type-specific
// attributes, institution, and custom fields).
// acctEditExtra carries the account-editor fields added after the original
// positional signature (AC3 statement day, AC11 exclude-from-net-worth), bundled
// into one struct so the editForm signature stays additive.
type acctEditExtra struct {
	stmtDayS       ui.State[string]
	exclNWS        ui.State[bool]
	onStmtDay      ui.Handler
	onToggleExclNW ui.Handler
	// instIDS/onInstID: AC10's structured institution-directory picker.
	instIDS          ui.State[string]
	onInstID         func(string)
	openInstitutions ui.Handler
	// beneficiaryNoteS/onBeneficiaryNote: AC16's beneficiary / TOD note.
	beneficiaryNoteS  ui.State[string]
	onBeneficiaryNote ui.Handler
	// revalueDaysS/onRevalueDays: AC5's per-account revaluation-cadence override.
	revalueDaysS  ui.State[string]
	onRevalueDays ui.Handler
}

func editForm(a domain.Account, dec int, curBal money.Money, members []domain.Member, accounts []domain.Account, categories []domain.Category, accDefs []customfields.Def,
	nameS, typeS, varNameS, ownerS, balS, climS, aprS, minpS, dueS, lenderS, institutionS, retS, apyS, liqS, stabS, lockS, notesS ui.State[string],
	setBalAmtS, setBalCatS ui.State[string],
	editAdvOpen, asLiabS, splitOwnS ui.State[bool], sharesMapS ui.State[map[string]int], customEditVals ui.State[map[string]string],
	onName, onVarName, onBal, onClim, onApr, onMinp, onDue, onLender, onInstitution, onRet, onApy, onLiq, onStab, onLock, onToggleEditAdv, onToggleAsLiab, onToggleSplitOwn, onNotes, onSetBalAmt ui.Handler,
	onCustomEdit func(key, value string), saveEdit, cancel ui.Handler, focusValue bool, x acctEditExtra) ui.Node {
	// The type is editable; the shown attribute fields follow the SELECTED type's
	// class (not the account's stored class), so switching e.g. a line of credit to a
	// credit card, or a liability to an asset, reveals the right fields live.
	selType := domain.AccountType(typeS.Get())
	isOther := selType == domain.TypeOther
	// "Other" accounts can be flagged as a liability (debt) explicitly; that reveals the
	// liability fields (min payment, due day, …) the debt formulas use.
	isLiab := selType.Class() == domain.ClassLiability || (isOther && asLiabS.Get())
	typeOptions := uiw.OptionsFrom(domain.AllAccountTypes,
		func(t domain.AccountType) string { return string(t) },
		func(t domain.AccountType) string { return humanizeType(string(t)) },
		typeS.Get())
	// The name field is auto-focused for the "Edit" entry; when opened from "Update
	// value" the value section takes focus instead, so the fast path is type-and-save.
	nameInput := []any{css.Class("field"), Attr("id", "acct-edit-"+a.ID), Type("text"),
		Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)}
	if !focusValue {
		nameInput = append(nameInput, Attr("autofocus", ""))
	}
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		// Fields scroll inside .modal-scroll; the Cancel/Save bar is pinned below in
		// .modal-foot (FlushBody on the host), so it never scrolls off a tall form.
		Div(css.Class("modal-scroll"),
			// Merged "update value / balance" section, up top so the marquee account action
			// (record a new value) is the fast path; leaving it blank keeps the balance as-is.
			acctValueUpdateSection(a, curBal, dec, setBalAmtS, setBalCatS, onSetBalAmt, categories, focusValue),
			labeledField(uistate.T("common.name"), Input(nameInput...)),
			// Optional explicit variable name for formulas/widgets, with a live chip + collision warning.
			labeledField(uistate.T("accounts.varNameLabel"),
				entityVarField(accountVarKind, accountVarEntities(accounts), a.ID, "acct-edit-varname-"+a.ID, "account-varname-warn", varNameS.Get(), nameS.Get(), onVarName)),
			labeledField(uistate.T("accounts.typeLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   typeOptions,
					Selected:  typeS.Get(),
					OnChange:  func(v string) { typeS.Set(v) },
					AriaLabel: uistate.T("accounts.typeLabel"),
					TestID:    "acct-edit-type-select",
				})),
			// "Other" is the catch-all type with no natural asset/liability class, so let the
			// user say whether it's a debt — that's what the net-worth and debt formulas read.
			If(isOther, Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-edit-as-liability"), OnChange(onToggleAsLiab)}, checkedAttr(asLiabS.Get())...)...),
				Div(css.Class("row-main"),
					Span(uistate.T("accounts.countAsLiability")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("accounts.countAsLiabilityHint"))))),
			labeledField(uistate.T("common.owner"),
				uiw.SelectInput(uiw.SelectInputProps{Options: ownerSelectOptions(members, ownerS.Get()), Selected: ownerS.Get(),
					OnChange: func(v string) { ownerS.Set(v) }, AriaLabel: uistate.T("common.owner")})),
			If(len(members) >= 2, func() ui.Node {
				shareSum := 0
				for _, v := range sharesMapS.Get() {
					shareSum += v
				}
				onShareChange := func(memberID string, valStr string) {
					n, _ := strconv.Atoi(valStr)
					m := sharesMapS.Get()
					nm := make(map[string]int, len(m)+1)
					for k, v := range m {
						nm[k] = v
					}
					nm[memberID] = n
					sharesMapS.Set(nm)
				}
				return Div(
					Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(splitOwnS.Get())), OnClick(onToggleSplitOwn),
						IfElse(splitOwnS.Get(), Text(uistate.T("account.splitOwnership")+" ▴"), Text(uistate.T("account.splitOwnership")+" ▾"))),
					If(splitOwnS.Get(), Div(
						P(css.Class("t-caption", tw.TextDim), uistate.T("account.splitOwnershipHint")),
						MapKeyed(members, func(m domain.Member) any { return m.ID }, func(m domain.Member) ui.Node {
							return ui.CreateElement(OwnerShareRow, ownerShareRowProps{Member: m, Share: sharesMapS.Get()[m.ID], OnChange: onShareChange})
						}),
						If(shareSum != 100, P(css.Class("err"), Attr("role", "alert"), uistate.T("account.shareSumError", shareSum))),
					)),
				)
			}()),
			labeledField(uistate.T("accounts.openingBalance"),
				Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(balS.Get()), Step("0.01"), OnInput(onBal))),
			If(isLiab, labeledField(uistate.T("accounts.creditLimit"),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Value(climS.Get()), Step("0.01"), OnInput(onClim)))),
			If(isLiab, labeledField(uistate.T("accounts.apr"),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Value(aprS.Get()), Step("0.01"), OnInput(onApr)))),
			If(isLiab, labeledField(uistate.T("accounts.minPayment"),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Value(minpS.Get()), Step("0.01"), OnInput(onMinp)))),
			If(isLiab, labeledField(uistate.T("accounts.dueDay"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay")), Value(dueS.Get()), OnInput(onDue)))),
			// AC3: statement-close day — the day the billing cycle closes, distinct from the
			// payment due day above. Powers real due dates in the bill calendar and a tighter
			// on-time payment window.
			If(isLiab, labeledField(uistate.T("accountsstmt.statementDay"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "31"), Step("1"), Attr("data-testid", "acct-edit-statement-day"), Placeholder(uistate.T("accountsstmt.statementDay")), Value(x.stmtDayS.Get()), OnInput(x.onStmtDay)))),
			If(isLiab, labeledField(uistate.T("accounts.lender"),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lenderS.Get()), OnInput(onLender)))),
			labeledField(uistate.T("accounts.institution"),
				uiw.Combobox(uiw.SuggestProps{Value: institutionS.Get(), Placeholder: uistate.T("accounts.institutionHint"),
					AriaLabel: uistate.T("accounts.institution"), OnInput: onInstitution, Options: domain.UniqueInstitutions(accounts), ListID: "inst-list-edit-" + a.ID})),
			// AC10: the structured institution directory — separate from the free-text
			// field above — colors this account's row and grounds the ★★
			// Multi-Institution Analytics feature with a real entity. "Manage
			// institutions" opens the shell-root directory modal to add a new one.
			labeledField(uistate.T("accounts.institutionDirectoryLabel"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
					uiw.SelectInput(uiw.SelectInputProps{Options: institutionPickerOptions(appstate.Default), Selected: x.instIDS.Get(),
						OnChange: x.onInstID, AriaLabel: uistate.T("accounts.institutionDirectoryLabel"), TestID: "acct-edit-institution-select"}),
					Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-edit-manage-institutions"),
						OnClick(x.openInstitutions), uistate.T("accounts.manageInstitutionsLink")))),
			// Free-text notes. Plain text (rides the dataset export/sync) — logins/secrets
			// go in the encrypted credential vault, never here.
			labeledField(uistate.T("accounts.notes"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("accounts.notesPlaceholder"),
					AriaLabel: uistate.T("accounts.notes"), OnInput: onNotes})),
			// AC16: who inherits this account — a plain beneficiary / transfer-on-death
			// note surfaced (compassionately, never with a password) in the estate
			// emergency pack.
			labeledField(uistate.T("accounts.beneficiaryNoteLabel"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: x.beneficiaryNoteS.Get(), Placeholder: uistate.T("accounts.beneficiaryNotePh"),
					AriaLabel: uistate.T("accounts.beneficiaryNoteLabel"), OnInput: x.onBeneficiaryNote})),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "-0.35rem 0 0"}), uistate.T("accounts.beneficiaryNoteHint")),
			// AC11: keep this account visible in its class views but out of net worth.
			Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-edit-exclude-networth"), OnChange(x.onToggleExclNW)}, checkedAttr(x.exclNWS.Get())...)...),
				Div(css.Class("row-main"),
					Span(uistate.T("accountsstmt.excludeNetWorth")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("accountsstmt.excludeNetWorthHint")))),
			If(!isLiab, Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(editAdvOpen.Get())), OnClick(onToggleEditAdv),
				IfElse(editAdvOpen.Get(), Text(uistate.T("accounts.hideAdvanced")), Text(uistate.T("accounts.showAdvanced"))))),
			If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.expReturn"),
				Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(retS.Get()), Step("0.01"), OnInput(onRet)))),
			If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.apyLabel"),
				Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.apyHint")), Attr("data-testid", "account-apy"), Placeholder(uistate.T("accounts.apyLabel")), Value(apyS.Get()), Step("0.01"), OnInput(onApy)))),
			If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.liquidity"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liqS.Get()), OnInput(onLiq)))),
			If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.stability"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stabS.Get()), OnInput(onStab)))),
			If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.lockUntilEdit"),
				Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntilEdit")), Title(uistate.T("accounts.lockUntilEdit")), Value(lockS.Get()), OnInput(onLock)))),
			// AC5: how often to refresh a periodically-ESTIMATED asset (property, vehicle,
			// crypto, or an "Other" you're treating as one) — 0/blank keeps the type's
			// default cadence (internal/revalue). The freshness machinery already reads
			// Account.RevalueDays; this is only the missing input.
			If(!isLiab && editAdvOpen.Get() && isRevaluableType(selType), labeledField(uistate.T("accounts.revalueDaysLabel"),
				Input(css.Class("field"), Type("number"), Attr("min", "1"), Step("1"), Attr("data-testid", "acct-edit-revalue-days"),
					Placeholder(uistate.T("accounts.revalueDaysPh")), Value(x.revalueDaysS.Get()), OnInput(x.onRevalueDays)))),
			If(!isLiab && editAdvOpen.Get() && isRevaluableType(selType), P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "-0.35rem 0 0"}), uistate.T("accounts.revalueDaysHint"))),
			If(len(accDefs) > 0, Fragment(
				P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0.5rem 0 0"}), uistate.T("accounts.customFieldsLabel")),
				MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customEditVals.Get()[d.Key], OnChange: onCustomEdit}))
				}),
			)),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

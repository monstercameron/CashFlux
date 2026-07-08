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
	lenderS := ui.UseState(a.Lender)
	institutionS := ui.UseState(instInit)
	retS := ui.UseState(floatOrEmpty(a.ExpectedReturnAPR))
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
	onLender := ui.UseEvent(func(v string) { lenderS.Set(v) })
	onInstitution := ui.UseEvent(func(v string) { institutionS.Set(v) })
	onRet := ui.UseEvent(func(v string) { retS.Set(v) })
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
	doSetBal := ui.UseEvent(Prevent(func() {
		if v := strings.TrimSpace(setBalAmtS.Get()); v != "" {
			cbs.OnSetBalance(a, curBal, v, setBalCatS.Get())
		}
		done()
	}))
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
			cp.Lender = strings.TrimSpace(lenderS.Get())
			cp.ExpectedReturnAPR = 0
			cp.LiquidityScore = 0
			cp.StabilityScore = 0
			cp.LockUntil = time.Time{}
		} else {
			cp.ExpectedReturnAPR = textutil.ParseFloat(retS.Get())
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
			cp.Lender = ""
		}
		cp.Institution = titleCaseWords(strings.TrimSpace(institutionS.Get()))
		cp.Notes = strings.TrimSpace(notesS.Get())
		if defs := app.CustomFieldDefsFor("account"); len(defs) > 0 {
			cp.Custom = customValuesToMap(defs, customEditVals.Get())
		}
		cbs.OnSave(cp)
		done()
	}))

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	switch props.Mode {
	case uistate.AcctEditModeSetBal:
		return setBalForm(a, curBal, dec, setBalAmtS, setBalCatS, onSetBalAmt, doSetBal, cancel, app.Categories())
	case uistate.AcctEditModeReconcile:
		return reconcileForm(a, curCleared, dec, stmtBalS, onStmtBal, cancel, cbs.OnRefresh, app)
	case uistate.AcctEditModeTransfer:
		return transferForm(a, app.Accounts(), xferFromS, xferToS, xferAmtS, xferDateS, xferDescS, onXferAmt, onXferDate, onXferDesc, doTransfer, cancel)
	default:
		return editForm(a, dec, app.Members(), app.Accounts(), app.CustomFieldDefsFor("account"),
			nameS, typeS, ev.VarName, ownerS, balS, climS, aprS, minpS, dueS, lenderS, institutionS, retS, liqS, stabS, lockS, notesS,
			editAdvOpen, asLiabS, splitOwnS, sharesMapS, customEditVals,
			ev.OnName, ev.OnVarName, onBal, onClim, onApr, onMinp, onDue, onLender, onInstitution, onRet, onLiq, onStab, onLock,
			onToggleEditAdv, onToggleAsLiab, onToggleSplitOwn, onNotes, onCustomEdit, saveEdit, cancel)
	}
}

// setBalForm is the update-balance / update-value editor.
func setBalForm(a domain.Account, curBal money.Money, dec int, setBalAmtS, setBalCatS ui.State[string], onSetBalAmt, doSetBal, cancel ui.Handler, categories []domain.Category) ui.Node {
	var deltaNode ui.Node = Fragment()
	if rawAmt := strings.TrimSpace(setBalAmtS.Get()); rawAmt != "" {
		if targetMinor, parseErr := money.ParseMinor(rawAmt, dec); parseErr == nil {
			dp := reconcile.PreviewDelta(curBal.Amount, targetMinor)
			sign := ""
			if dp.AdjustmentMinor > 0 {
				sign = "+"
			}
			adjLabel := sign + money.FormatMinor(dp.AdjustmentMinor, dec)
			if dp.NeedsAdjustment {
				deltaNode = P(css.Class("t-caption"), Attr("data-testid", "setbal-delta-preview"),
					uistate.T("accounts.setBalanceDeltaPreview",
						fmtMoney(money.New(curBal.Amount, a.Currency)),
						fmtMoney(money.New(targetMinor, a.Currency)), adjLabel))
			} else {
				deltaNode = P(css.Class("t-caption"), Attr("data-testid", "setbal-delta-preview"),
					uistate.T("accounts.setBalanceNoAdjNeeded"))
			}
		}
	}
	catOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.setBalanceNoCategory")}}
	for _, c := range categories {
		catOpts = append(catOpts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	return Form(css.Class("acct-edit-form"), Attr("id", "acct-setbal-form-"+a.ID),
		Attr("aria-label", uistate.T("accounts.setBalanceFormLabel", a.Name)), OnSubmit(doSetBal),
		labeledField(uistate.T("accounts.setBalanceAmount"),
			Input(css.Class("field"), Attr("id", "acct-setbal-"+a.ID), Attr("autofocus", ""),
				Type("number"), Placeholder(uistate.T("accounts.setBalanceAmount")),
				Value(setBalAmtS.Get()), Step("0.01"), OnInput(onSetBalAmt))),
		deltaNode,
		labeledField(uistate.T("accounts.setBalanceCategoryLabel"),
			uiw.SelectInput(uiw.SelectInputProps{Options: catOpts, Selected: setBalCatS.Get(),
				OnChange: func(v string) { setBalCatS.Set(v) }, AriaLabel: uistate.T("accounts.setBalanceCategoryLabel"), TestID: "setbal-cat-select"})),
		If(isValuationType(a.Type), P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "valuation-manual-note"),
			uistate.T("accounts.valuationManualNote"))),
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
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
		Div(
			labeledField(uistate.T("accounts.statementBalance"),
				Input(css.Class("field"), Attr("id", "acct-reconcile-stmt-"+a.ID), Attr("autofocus", ""),
					Attr("data-testid", "reconcile-statement-input"), Type("number"), Step("0.01"),
					Placeholder(uistate.T("accounts.statementBalancePh")), Value(stmtBalS.Get()), OnInput(onStmtBal))),
		),
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
		Div(css.Class("acct-edit-actions"),
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
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			IfElse(submitDisabled,
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("disabled", "disabled"), uistate.T("accounts.transferSubmit")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.transferSubmit"))),
		),
	)
}

// editForm is the full inline-edit editor (name, owner, balances, type-specific
// attributes, institution, and custom fields).
func editForm(a domain.Account, dec int, members []domain.Member, accounts []domain.Account, accDefs []customfields.Def,
	nameS, typeS, varNameS, ownerS, balS, climS, aprS, minpS, dueS, lenderS, institutionS, retS, liqS, stabS, lockS, notesS ui.State[string],
	editAdvOpen, asLiabS, splitOwnS ui.State[bool], sharesMapS ui.State[map[string]int], customEditVals ui.State[map[string]string],
	onName, onVarName, onBal, onClim, onApr, onMinp, onDue, onLender, onInstitution, onRet, onLiq, onStab, onLock, onToggleEditAdv, onToggleAsLiab, onToggleSplitOwn, onNotes ui.Handler,
	onCustomEdit func(key, value string), saveEdit, cancel ui.Handler) ui.Node {
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
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "acct-edit-"+a.ID), Attr("autofocus", ""), Type("text"),
				Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
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
		If(isLiab, labeledField(uistate.T("accounts.lender"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lenderS.Get()), OnInput(onLender)))),
		labeledField(uistate.T("accounts.institution"),
			uiw.Combobox(uiw.SuggestProps{Value: institutionS.Get(), Placeholder: uistate.T("accounts.institutionHint"),
				AriaLabel: uistate.T("accounts.institution"), OnInput: onInstitution, Options: domain.UniqueInstitutions(accounts), ListID: "inst-list-edit-" + a.ID})),
		// Free-text notes. Plain text (rides the dataset export/sync) — logins/secrets
		// go in the encrypted credential vault, never here.
		labeledField(uistate.T("accounts.notes"),
			uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("accounts.notesPlaceholder"),
				AriaLabel: uistate.T("accounts.notes"), OnInput: onNotes})),
		If(!isLiab, Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(editAdvOpen.Get())), OnClick(onToggleEditAdv),
			IfElse(editAdvOpen.Get(), Text(uistate.T("accounts.hideAdvanced")), Text(uistate.T("accounts.showAdvanced"))))),
		If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.expReturn"),
			Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(retS.Get()), Step("0.01"), OnInput(onRet)))),
		If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.liquidity"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liqS.Get()), OnInput(onLiq)))),
		If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.stability"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stabS.Get()), OnInput(onStab)))),
		If(!isLiab && editAdvOpen.Get(), labeledField(uistate.T("accounts.lockUntilEdit"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntilEdit")), Title(uistate.T("accounts.lockUntilEdit")), Value(lockS.Get()), OnInput(onLock)))),
		If(len(accDefs) > 0, Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0.5rem 0 0"}), uistate.T("accounts.customFieldsLabel")),
			MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customEditVals.Get()[d.Key], OnChange: onCustomEdit}))
			}),
		)),
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

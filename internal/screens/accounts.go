//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Accounts lists assets and liabilities with live balances, a net-worth summary,
// an add form, and per-row delete.
func Accounts() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	// Revision atom: bumping it after a mutation re-renders this screen.
	rev := state.UseAtom("rev:accounts", 0)

	name := ui.UseState("")
	curr := ui.UseState("USD")
	amount := ui.UseState("0")
	accType := ui.UseState(string(domain.TypeChecking))
	owner := ui.UseState(domain.GroupOwnerID)
	creditLimit := ui.UseState("")
	apr := ui.UseState("")
	minPayment := ui.UseState("")
	dueDay := ui.UseState("")
	lender := ui.UseState("")
	expReturn := ui.UseState("")
	liquidity := ui.UseState("")
	stability := ui.UseState("")
	lockUntil := ui.UseState("")
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	noticeAtom := uistate.UseNotice()
	notifyErr := func(text string) { noticeAtom.Set(noticeAtom.Get().With(text, true)) }

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onCurr := ui.UseEvent(func(v string) { curr.Set(strings.ToUpper(v)) })
	onAmount := ui.UseEvent(func(v string) { amount.Set(v) })
	onType := ui.UseEvent(func(e ui.Event) { accType.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onCreditLimit := ui.UseEvent(func(v string) { creditLimit.Set(v) })
	onApr := ui.UseEvent(func(v string) { apr.Set(v) })
	onMinPayment := ui.UseEvent(func(v string) { minPayment.Set(v) })
	onDueDay := ui.UseEvent(func(v string) { dueDay.Set(v) })
	onLender := ui.UseEvent(func(v string) { lender.Set(v) })
	onExpReturn := ui.UseEvent(func(v string) { expReturn.Set(v) })
	onLiquidity := ui.UseEvent(func(v string) { liquidity.Set(v) })
	onStability := ui.UseEvent(func(v string) { stability.Set(v) })
	onLockUntil := ui.UseEvent(func(v string) { lockUntil.Set(v) })

	bump := func() { rev.Set(rev.Get() + 1) }

	accDefs := app.CustomFieldDefsFor("account")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

	add := ui.UseEvent(Prevent(func() {
		c := strings.ToUpper(strings.TrimSpace(curr.Get()))
		amt, err := money.ParseMinor(strings.TrimSpace(amount.Get()), currency.Decimals(c))
		if err != nil {
			errMsg.Set(uistate.T("accounts.invalidOpening"))
			return
		}
		typ := domain.AccountType(accType.Get())
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		acc := domain.Account{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), OwnerID: owner.Get(), Scope: scope,
			Class: typ.Class(), Type: typ, Currency: c,
			OpeningBalance: money.New(amt, c), BalanceAsOf: time.Now(),
		}
		if typ.Class() == domain.ClassLiability {
			if cl, e := money.ParseMinor(strings.TrimSpace(creditLimit.Get()), currency.Decimals(c)); e == nil && cl > 0 {
				acc.CreditLimit = money.New(cl, c)
			}
			if a, e := strconv.ParseFloat(strings.TrimSpace(apr.Get()), 64); e == nil {
				acc.InterestRateAPR = a
			}
			if mp, e := money.ParseMinor(strings.TrimSpace(minPayment.Get()), currency.Decimals(c)); e == nil && mp > 0 {
				acc.MinPayment = money.New(mp, c)
			}
			if dd, e := strconv.Atoi(strings.TrimSpace(dueDay.Get())); e == nil {
				acc.DueDayOfMonth = dd
			}
			acc.Lender = strings.TrimSpace(lender.Get())
		} else {
			if r, e := strconv.ParseFloat(strings.TrimSpace(expReturn.Get()), 64); e == nil {
				acc.ExpectedReturnAPR = r
			}
			if l, e := strconv.Atoi(strings.TrimSpace(liquidity.Get())); e == nil {
				acc.LiquidityScore = l
			}
			if s, e := strconv.Atoi(strings.TrimSpace(stability.Get())); e == nil {
				acc.StabilityScore = s
			}
			if lu := strings.TrimSpace(lockUntil.Get()); lu != "" {
				if d, e := dateutil.ParseDate(lu); e == nil {
					acc.LockUntil = d
				}
			}
		}
		acc.Custom = customValuesToMap(accDefs, customVals.Get())
		if err := app.PutAccount(acc); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		amount.Set("0")
		creditLimit.Set("")
		apr.Set("")
		minPayment.Set("")
		dueDay.Set("")
		lender.Set("")
		expReturn.Set("")
		liquidity.Set("")
		lockUntil.Set("")
		stability.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		bump()
	}))

	deleteAccount := func(accountID string) {
		if err := app.DeleteAccount(accountID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	archiveAccount := func(ac domain.Account) {
		ac.Archived = !ac.Archived
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	refreshAccount := func(ac domain.Account) {
		ac.BalanceAsOf = time.Now()
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	loadSample := ui.UseEvent(Prevent(func() {
		if err := app.LoadSample(); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}))
	markAllUpdated := ui.UseEvent(Prevent(func() {
		w := app.FreshnessWindows()
		now := time.Now()
		n := 0
		for _, ac := range app.Accounts() {
			if ac.Archived || !freshness.IsStale(ac, w, now) {
				continue
			}
			ac.BalanceAsOf = now
			if err := app.PutAccount(ac); err != nil {
				notifyErr(uistate.T("accounts.markErr", err.Error()))
				continue
			}
			n++
		}
		errMsg.Set("")
		if n > 0 {
			noticeAtom.Set(noticeAtom.Get().With(uistate.T("accounts.markedUpdated", plural(n, "balance")), false))
		}
		bump()
	}))

	typeOptions := make([]ui.Node, 0, len(domain.AllAccountTypes))
	for _, t := range domain.AllAccountTypes {
		typeOptions = append(typeOptions, Option(Value(string(t)), SelectedIf(accType.Get() == string(t)), humanizeType(string(t))))
	}
	ownerOptions := []ui.Node{
		Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), uistate.T("owner.group")),
	}
	for _, m := range app.Members() {
		ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
	}

	isLiab := domain.AccountType(accType.Get()).Class() == domain.ClassLiability
	form := Section(Class("card"),
		H2(Class("card-title"), uistate.T("accounts.addTitle")),
		Form(Class("form-grid"), OnSubmit(add),
			Input(append([]any{Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("acct-err", errMsg.Get())...)...),
			Select(Class("field"), OnChange(onType), typeOptions),
			Select(Class("field"), OnChange(onOwner), ownerOptions),
			Input(Class("field"), Type("text"), Placeholder(uistate.T("accounts.currency")), Value(curr.Get()), OnInput(onCurr)),
			Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(amount.Get()), Step("0.01"), OnInput(onAmount)),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.creditLimit")), Value(creditLimit.Get()), Step("0.01"), OnInput(onCreditLimit))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.apr")), Value(apr.Get()), Step("0.01"), OnInput(onApr))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.minPayment")), Value(minPayment.Get()), Step("0.01"), OnInput(onMinPayment))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.dueDay")), Value(dueDay.Get()), OnInput(onDueDay))),
			If(isLiab, Input(Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lender.Get()), OnInput(onLender))),
			If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(expReturn.Get()), Step("0.01"), OnInput(onExpReturn))),
			If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liquidity.Get()), OnInput(onLiquidity))),
			If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stability.Get()), OnInput(onStability))),
			If(!isLiab, Input(Class("field"), Type("date"), Title(uistate.T("accounts.lockUntil")), Value(lockUntil.Get()), OnInput(onLockUntil))),
			MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
			Button(Class("btn btn-primary"), Type("submit"), uistate.T("accounts.addTitle")),
		),
		errText("acct-err", errMsg.Get()),
	)

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	net, assets, liabilities, _ := ledger.NetWorth(accounts, txns, rates)

	var assetList, liabList, archivedList []domain.Account
	for _, ac := range accounts {
		switch {
		case ac.Archived:
			archivedList = append(archivedList, ac)
		case ac.Class == domain.ClassLiability:
			liabList = append(liabList, ac)
		default:
			assetList = append(assetList, ac)
		}
	}

	saveAccount := func(ac domain.Account) {
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	setBalance := func(ac domain.Account, currentBal money.Money, newStr string) {
		dec := currency.Decimals(ac.Currency)
		target, err := money.ParseMinor(strings.TrimSpace(newStr), dec)
		if err != nil {
			errMsg.Set(uistate.T("accounts.invalidBalance"))
			return
		}
		// Post an adjustment transaction for the difference, so the computed
		// balance equals the figure entered (e.g. matching a statement).
		if amount, ok := ledger.AdjustmentToTarget(currentBal, target); ok {
			adj := domain.Transaction{
				ID: id.New(), AccountID: ac.ID, Date: time.Now(), Desc: uistate.T("accounts.balanceAdjustment"),
				Amount: amount, Cleared: true,
			}
			if err := app.PutTransaction(adj); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		ac.BalanceAsOf = time.Now()
		if err := app.PutAccount(ac); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
		// Confirm the update through the toast live region (a polite notice), so the
		// new balance is announced to screen readers and visibly acknowledged — the
		// reconcile flow was previously silent on success.
		noticeAtom.Set(noticeAtom.Get().With(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(target, ac.Currency))), false))
	}

	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTransactions := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate("/transactions")
	}

	windows := app.FreshnessWindows()
	now := time.Now()
	staleCount := 0
	for _, ac := range accounts {
		if freshness.IsStale(ac, windows, now) {
			staleCount++
		}
	}
	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		cleared, _ := ledger.ClearedBalance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Cleared: cleared, Stale: freshness.IsStale(ac, windows, now), Members: app.Members(),
			OnDelete: deleteAccount, OnArchive: archiveAccount, OnRefresh: refreshAccount, OnSave: saveAccount, OnView: viewTransactions, OnSetBalance: setBalance,
		})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	return Div(
		If(len(accounts) == 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("accounts.welcomeTitle")),
			P(Class("muted"), uistate.T("accounts.welcomeDesc")),
			Button(Class("btn btn-primary"), Type("button"), OnClick(loadSample), uistate.T("accounts.loadSample")),
		)),
		Div(Class("stat-grid"),
			stat(uistate.T("dashboard.netWorth"), fmtMoney(net), accentFor(net)),
			stat(uistate.T("accounts.assets"), fmtMoney(assets), "pos"),
			stat(uistate.T("dashboard.liabilities"), fmtMoney(liabilities), "neg"),
		),
		If(staleCount > 0, Div(Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(Class("btn"), Type("button"), Title(uistate.T("accounts.markAllTitle")), OnClick(markAllUpdated),
				Text(uistate.T("accounts.markAll", plural(staleCount, "account")))),
		)),
		form,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("accounts.assets")),
			IfElse(len(assetList) == 0, P(Class("empty"), uistate.T("accounts.noAssets")), Div(Class("rows"), MapKeyed(assetList, keyOf, renderRow))),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("dashboard.liabilities")),
			IfElse(len(liabList) == 0, P(Class("empty"), uistate.T("accounts.noLiabilities")), Div(Class("rows"), MapKeyed(liabList, keyOf, renderRow))),
		),
		If(len(archivedList) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("accounts.archived")),
			Div(Class("rows"), MapKeyed(archivedList, keyOf, renderRow)),
		)),
	)
}

// accountMeta builds an account row's subtitle: type · currency, plus credit
// utilization for liability accounts that have a credit limit.
func accountMeta(a domain.Account, bal money.Money) string {
	meta := humanizeType(string(a.Type)) + " · " + a.Currency
	if a.Class == domain.ClassLiability {
		if pct, ok := ledger.Utilization(bal.Amount, a.CreditLimit.Amount); ok {
			meta += fmt.Sprintf(" · %d%% of limit used", pct)
		}
	}
	return meta
}

type accountRowProps struct {
	Account      domain.Account
	Balance      money.Money
	Cleared      money.Money
	Stale        bool
	Members      []domain.Member
	OnDelete     func(string)
	OnArchive    func(domain.Account)
	OnRefresh    func(domain.Account)
	OnSave       func(domain.Account)
	OnView       func(string)
	OnSetBalance func(domain.Account, money.Money, string)
}

// moneyMajorOrEmpty renders a money value as a major-unit string, or "" when zero.
func moneyMajorOrEmpty(m money.Money, dec int) string {
	if m.Amount == 0 {
		return ""
	}
	return money.FormatMinor(m.Amount, dec)
}

// floatOrEmpty renders a float as a plain string, or "" when zero.
func floatOrEmpty(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// intOrEmpty renders an int, or "" when zero.
func intOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// AccountRow is a per-account row component. It can be edited inline (name,
// opening balance, and the type-specific asset/liability attributes); it owns all
// its hooks so the list and the edit toggle never disturb hook ordering.
func AccountRow(props accountRowProps) ui.Node {
	a := props.Account
	dec := currency.Decimals(a.Currency)

	// Secondary actions live in a "⋯" overflow menu so each row stays uncluttered
	// (primary: Transactions / Edit / ✕); selecting one closes the menu (C9).
	menuOpen := ui.UseState(false)
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(a.ID) }))
	arch := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnArchive(a) }))
	refresh := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnRefresh(a) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(a.ID) }))
	settingBal := ui.UseState(false)
	setBalAmtS := ui.UseState("")
	setBal := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		setBalAmtS.Set("")
		settingBal.Set(true)
	}))
	onSetBalAmt := ui.UseEvent(func(v string) { setBalAmtS.Set(v) })
	doSetBal := ui.UseEvent(Prevent(func() {
		if v := strings.TrimSpace(setBalAmtS.Get()); v != "" {
			props.OnSetBalance(a, props.Balance, v)
		}
		settingBal.Set(false)
	}))
	cancelSetBal := ui.UseEvent(Prevent(func() { settingBal.Set(false) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(a.Name)
	balS := ui.UseState(money.FormatMinor(a.OpeningBalance.Amount, dec))
	climS := ui.UseState(moneyMajorOrEmpty(a.CreditLimit, dec))
	aprS := ui.UseState(floatOrEmpty(a.InterestRateAPR))
	minpS := ui.UseState(moneyMajorOrEmpty(a.MinPayment, dec))
	dueS := ui.UseState(intOrEmpty(a.DueDayOfMonth))
	lenderS := ui.UseState(a.Lender)
	retS := ui.UseState(floatOrEmpty(a.ExpectedReturnAPR))
	liqS := ui.UseState(intOrEmpty(a.LiquidityScore))
	stabS := ui.UseState(intOrEmpty(a.StabilityScore))
	lockISO := ""
	if !a.LockUntil.IsZero() {
		lockISO = dateutil.FormatDate(a.LockUntil)
	}
	lockS := ui.UseState(lockISO)
	ownerS := ui.UseState(a.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onBal := ui.UseEvent(func(v string) { balS.Set(v) })
	onClim := ui.UseEvent(func(v string) { climS.Set(v) })
	onApr := ui.UseEvent(func(v string) { aprS.Set(v) })
	onMinp := ui.UseEvent(func(v string) { minpS.Set(v) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onLender := ui.UseEvent(func(v string) { lenderS.Set(v) })
	onRet := ui.UseEvent(func(v string) { retS.Set(v) })
	onLiq := ui.UseEvent(func(v string) { liqS.Set(v) })
	onStab := ui.UseEvent(func(v string) { stabS.Set(v) })
	onLock := ui.UseEvent(func(v string) { lockS.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(a.Name)
		balS.Set(money.FormatMinor(a.OpeningBalance.Amount, dec))
		climS.Set(moneyMajorOrEmpty(a.CreditLimit, dec))
		aprS.Set(floatOrEmpty(a.InterestRateAPR))
		minpS.Set(moneyMajorOrEmpty(a.MinPayment, dec))
		dueS.Set(intOrEmpty(a.DueDayOfMonth))
		lenderS.Set(a.Lender)
		retS.Set(floatOrEmpty(a.ExpectedReturnAPR))
		liqS.Set(intOrEmpty(a.LiquidityScore))
		stabS.Set(intOrEmpty(a.StabilityScore))
		lockS.Set(lockISO)
		ownerS.Set(a.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		cp := a
		cp.Name = strings.TrimSpace(nameS.Get())
		cp.OwnerID = ownerS.Get()
		if ownerS.Get() == domain.GroupOwnerID {
			cp.Scope = domain.ScopeShared
		} else {
			cp.Scope = domain.ScopeIndividual
		}
		if amt, err := money.ParseMinor(strings.TrimSpace(balS.Get()), dec); err == nil {
			cp.OpeningBalance = money.New(amt, a.Currency)
		}
		if a.Class == domain.ClassLiability {
			cp.CreditLimit = parseMoneyOrZero(climS.Get(), dec, a.Currency)
			cp.InterestRateAPR = textutil.ParseFloat(aprS.Get())
			cp.MinPayment = parseMoneyOrZero(minpS.Get(), dec, a.Currency)
			cp.DueDayOfMonth = textutil.ParseInt(dueS.Get())
			cp.Lender = strings.TrimSpace(lenderS.Get())
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
		}
		props.OnSave(cp)
		editing.Set(false)
	}))

	// Land the cursor in the first field when an inline editor opens (§6.7).
	ui.UseEffect(func() func() {
		switch {
		case settingBal.Get():
			focusByID("acct-setbal-" + a.ID)
		case editing.Get():
			focusByID("acct-edit-" + a.ID)
		}
		return nil
	}, fmt.Sprintf("%t-%t", editing.Get(), settingBal.Get()))

	if settingBal.Get() {
		return Div(Class("row-edit"),
			Form(Class("form-grid"), OnSubmit(doSetBal),
				Input(Class("field"), Attr("id", "acct-setbal-"+a.ID), Type("number"), Placeholder(uistate.T("accounts.setBalanceAmount")), Value(setBalAmtS.Get()), Step("0.01"), OnInput(onSetBalAmt)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelSetBal), uistate.T("action.cancel")),
			),
		)
	}
	if editing.Get() {
		isLiab := a.Class == domain.ClassLiability
		return Div(Class("row-edit"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", "acct-edit-"+a.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)),
				Select(Class("field"), Title(uistate.T("common.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get())),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(balS.Get()), Step("0.01"), OnInput(onBal)),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.creditLimit")), Value(climS.Get()), Step("0.01"), OnInput(onClim))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.apr")), Value(aprS.Get()), Step("0.01"), OnInput(onApr))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.minPayment")), Value(minpS.Get()), Step("0.01"), OnInput(onMinp))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder(uistate.T("accounts.dueDay")), Value(dueS.Get()), OnInput(onDue))),
				If(isLiab, Input(Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lenderS.Get()), OnInput(onLender))),
				If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(retS.Get()), Step("0.01"), OnInput(onRet))),
				If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liqS.Get()), OnInput(onLiq))),
				If(!isLiab, Input(Class("field"), Type("number"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stabS.Get()), OnInput(onStab))),
				If(!isLiab, Input(Class("field"), Type("date"), Title(uistate.T("accounts.lockUntilEdit")), Value(lockS.Get()), OnInput(onLock))),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	archLabel, archTitle := uistate.T("accounts.archive"), uistate.T("accounts.archiveTitle")
	if a.Archived {
		archLabel, archTitle = uistate.T("accounts.restore"), uistate.T("accounts.restoreTitle")
	}
	meta := accountMeta(a, props.Balance)
	if props.Cleared.Amount != props.Balance.Amount {
		meta += uistate.T("accounts.clearedSuffix", fmtMoney(props.Cleared))
	}
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), a.Name,
				If(props.Stale, Span(Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), uistate.T("accounts.stale"))),
			),
			Span(Class("row-meta"), meta),
		),
		Span(Class(amountClass(props.Balance)), fmtMoney(props.Balance)),
		// Primary actions inline; everything else in the ⋯ menu.
		Button(Class("btn"), Type("button"), Title(uistate.T("accounts.viewTitle")), OnClick(view), uistate.T("nav.transactions")),
		Button(Class("btn"), Type("button"), Title(uistate.T("accounts.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
		Div(Class("add-wrap"),
			Button(Class("btn"), Type("button"), Attr("title", uistate.T("accounts.moreActions")), Attr("aria-label", uistate.T("accounts.moreActions")), Attr("aria-haspopup", "menu"), OnClick(toggleMenu), Span(Attr("aria-hidden", "true"), "⋯")),
			Div(Class("add-backdrop"+menuHidden), OnClick(closeMenu)),
			Div(Class("add-menu"+menuHidden), Attr("role", "menu"),
				If(!a.Archived, Button(Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(setBal), uistate.T("accounts.updateBalance"))),
				If(!a.Archived, Button(Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(refresh), uistate.T("accounts.markUpdated"))),
				Button(Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("title", archTitle), OnClick(arch), archLabel),
			),
		),
		Button(Class("btn-del"), Type("button"), Title(uistate.T("accounts.deleteTitle")), OnClick(del), "✕"),
	)
}

// parseMoneyOrZero parses a major-unit amount to money, returning zero on error.
func parseMoneyOrZero(s string, dec int, cur string) money.Money {
	if amt, err := money.ParseMinor(strings.TrimSpace(s), dec); err == nil {
		return money.New(amt, cur)
	}
	return money.Money{Currency: cur}
}

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
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
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
			errMsg.Set("Enter a valid opening balance.")
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
		for _, ac := range app.Accounts() {
			if ac.Archived || !freshness.IsStale(ac, w, now) {
				continue
			}
			ac.BalanceAsOf = now
			_ = app.PutAccount(ac)
		}
		errMsg.Set("")
		bump()
	}))

	typeOptions := make([]ui.Node, 0, len(domain.AllAccountTypes))
	for _, t := range domain.AllAccountTypes {
		typeOptions = append(typeOptions, Option(Value(string(t)), SelectedIf(accType.Get() == string(t)), humanizeType(string(t))))
	}
	ownerOptions := []ui.Node{
		Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), "Group (shared)"),
	}
	for _, m := range app.Members() {
		ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
	}

	isLiab := domain.AccountType(accType.Get()).Class() == domain.ClassLiability
	form := Section(Class("card"),
		H2(Class("card-title"), "Add account"),
		Form(Class("form-grid"), OnSubmit(add),
			Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
			Select(Class("field"), OnChange(onType), typeOptions),
			Select(Class("field"), OnChange(onOwner), ownerOptions),
			Input(Class("field"), Type("text"), Placeholder("Currency"), Value(curr.Get()), OnInput(onCurr)),
			Input(Class("field"), Type("number"), Placeholder("Opening balance"), Value(amount.Get()), Step("0.01"), OnInput(onAmount)),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder("Credit limit"), Value(creditLimit.Get()), Step("0.01"), OnInput(onCreditLimit))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder("Interest APR %"), Value(apr.Get()), Step("0.01"), OnInput(onApr))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder("Minimum payment"), Value(minPayment.Get()), Step("0.01"), OnInput(onMinPayment))),
			If(isLiab, Input(Class("field"), Type("number"), Placeholder("Due day (1–28)"), Value(dueDay.Get()), OnInput(onDueDay))),
			If(isLiab, Input(Class("field"), Type("text"), Placeholder("Lender"), Value(lender.Get()), OnInput(onLender))),
			If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Expected return APR %"), Value(expReturn.Get()), Step("0.01"), OnInput(onExpReturn))),
			If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Liquidity 0–100"), Value(liquidity.Get()), OnInput(onLiquidity))),
			If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Stability 0–100"), Value(stability.Get()), OnInput(onStability))),
			If(!isLiab, Input(Class("field"), Type("date"), Title("Locked until (no new money before this date)"), Value(lockUntil.Get()), OnInput(onLockUntil))),
			MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
			Button(Class("btn btn-primary"), Type("submit"), "Add account"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
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
			errMsg.Set("Enter a valid balance amount.")
			return
		}
		// Post an adjustment transaction for the difference, so the computed
		// balance equals the figure entered (e.g. matching a statement).
		if delta := target - currentBal.Amount; delta != 0 {
			adj := domain.Transaction{
				ID: id.New(), AccountID: ac.ID, Date: time.Now(), Desc: "Balance adjustment",
				Amount: money.New(delta, ac.Currency), Cleared: true,
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
			H2(Class("card-title"), "Welcome to CashFlux"),
			P(Class("muted"), "No accounts yet. Add one below, or load some sample data to explore."),
			Button(Class("btn btn-primary"), Type("button"), OnClick(loadSample), "Load sample data"),
		)),
		Div(Class("stat-grid"),
			stat("Net worth", fmtMoney(net), accentFor(net)),
			stat("Assets", fmtMoney(assets), "pos"),
			stat("Liabilities", fmtMoney(liabilities), "neg"),
		),
		If(staleCount > 0, Div(Style(map[string]string{"margin-bottom": "0.6rem"}),
			Button(Class("btn"), Type("button"), Title("Mark every stale balance as checked today"), OnClick(markAllUpdated),
				Textf("Mark all updated (%s stale)", plural(staleCount, "account"))),
		)),
		form,
		Section(Class("card"),
			H2(Class("card-title"), "Assets"),
			IfElse(len(assetList) == 0, P(Class("empty"), "No asset accounts yet."), Div(Class("rows"), MapKeyed(assetList, keyOf, renderRow))),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Liabilities"),
			IfElse(len(liabList) == 0, P(Class("empty"), "No liabilities — nice."), Div(Class("rows"), MapKeyed(liabList, keyOf, renderRow))),
		),
		If(len(archivedList) > 0, Section(Class("card"),
			H2(Class("card-title"), "Archived"),
			Div(Class("rows"), MapKeyed(archivedList, keyOf, renderRow)),
		)),
	)
}

// accountMeta builds an account row's subtitle: type · currency, plus credit
// utilization for liability accounts that have a credit limit.
func accountMeta(a domain.Account, bal money.Money) string {
	meta := humanizeType(string(a.Type)) + " · " + a.Currency
	if a.Class == domain.ClassLiability && a.CreditLimit.Amount > 0 {
		owed := bal.Amount
		if owed < 0 {
			owed = -owed
		}
		meta += fmt.Sprintf(" · %d%% of limit used", owed*100/a.CreditLimit.Amount)
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

	del := ui.UseEvent(Prevent(func() { props.OnDelete(a.ID) }))
	arch := ui.UseEvent(Prevent(func() { props.OnArchive(a) }))
	refresh := ui.UseEvent(Prevent(func() { props.OnRefresh(a) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(a.ID) }))
	setBal := ui.UseEvent(Prevent(func() {
		if v := promptText("Actual balance of " + a.Name + " (" + a.Currency + ")?"); v != "" {
			props.OnSetBalance(a, props.Balance, v)
		}
	}))
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
			cp.InterestRateAPR = parseFloatOrZero(aprS.Get())
			cp.MinPayment = parseMoneyOrZero(minpS.Get(), dec, a.Currency)
			cp.DueDayOfMonth = parseIntOrZero(dueS.Get())
			cp.Lender = strings.TrimSpace(lenderS.Get())
		} else {
			cp.ExpectedReturnAPR = parseFloatOrZero(retS.Get())
			cp.LiquidityScore = parseIntOrZero(liqS.Get())
			cp.StabilityScore = parseIntOrZero(stabS.Get())
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

	if editing.Get() {
		isLiab := a.Class == domain.ClassLiability
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(nameS.Get()), OnInput(onName)),
				Select(Class("field"), Title("Owner"), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get())),
				Input(Class("field"), Type("number"), Placeholder("Opening balance"), Value(balS.Get()), Step("0.01"), OnInput(onBal)),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder("Credit limit"), Value(climS.Get()), Step("0.01"), OnInput(onClim))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder("Interest APR %"), Value(aprS.Get()), Step("0.01"), OnInput(onApr))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder("Minimum payment"), Value(minpS.Get()), Step("0.01"), OnInput(onMinp))),
				If(isLiab, Input(Class("field"), Type("number"), Placeholder("Due day (1–28)"), Value(dueS.Get()), OnInput(onDue))),
				If(isLiab, Input(Class("field"), Type("text"), Placeholder("Lender"), Value(lenderS.Get()), OnInput(onLender))),
				If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Expected return APR %"), Value(retS.Get()), Step("0.01"), OnInput(onRet))),
				If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Liquidity 0–100"), Value(liqS.Get()), OnInput(onLiq))),
				If(!isLiab, Input(Class("field"), Type("number"), Placeholder("Stability 0–100"), Value(stabS.Get()), OnInput(onStab))),
				If(!isLiab, Input(Class("field"), Type("date"), Title("Locked until"), Value(lockS.Get()), OnInput(onLock))),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

	archLabel := "Archive"
	if a.Archived {
		archLabel = "Restore"
	}
	meta := accountMeta(a, props.Balance)
	if props.Cleared.Amount != props.Balance.Amount {
		meta += " · cleared " + fmtMoney(props.Cleared)
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), a.Name,
				If(props.Stale, Span(Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), "Stale")),
			),
			Span(Class("row-meta"), meta),
		),
		Span(Class(amountClass(props.Balance)), fmtMoney(props.Balance)),
		Button(Class("btn"), Type("button"), Title("View this account's transactions"), OnClick(view), "Transactions"),
		If(!a.Archived, Button(Class("btn"), Type("button"), Title("Set the real balance; posts an adjustment"), OnClick(setBal), "Update balance")),
		If(!a.Archived, Button(Class("btn"), Type("button"), Title("Mark balance as checked today"), OnClick(refresh), "Mark updated")),
		Button(Class("btn"), Type("button"), Title("Edit account"), OnClick(startEdit), "Edit"),
		Button(Class("btn"), Type("button"), Title(archLabel+" account"), OnClick(arch), archLabel),
		Button(Class("btn-del"), Type("button"), Title("Delete account"), OnClick(del), "✕"),
	)
}

// parseMoneyOrZero parses a major-unit amount to money, returning zero on error.
func parseMoneyOrZero(s string, dec int, cur string) money.Money {
	if amt, err := money.ParseMinor(strings.TrimSpace(s), dec); err == nil {
		return money.New(amt, cur)
	}
	return money.Money{Currency: cur}
}

// parseFloatOrZero parses a float, returning 0 on error.
func parseFloatOrZero(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// parseIntOrZero parses an int, returning 0 on error.
func parseIntOrZero(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

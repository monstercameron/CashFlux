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
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
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

	windows := freshness.DefaultWindows()
	now := time.Now()
	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{
			Account: ac, Balance: bal, Stale: freshness.IsStale(ac, windows, now),
			OnDelete: deleteAccount, OnArchive: archiveAccount, OnRefresh: refreshAccount,
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
	Account   domain.Account
	Balance   money.Money
	Stale     bool
	OnDelete  func(string)
	OnArchive func(domain.Account)
	OnRefresh func(domain.Account)
}

// AccountRow is a per-account row component; it owns its action-handler hooks so
// the list can change without breaking hook ordering.
func AccountRow(props accountRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Account.ID) }))
	arch := ui.UseEvent(Prevent(func() { props.OnArchive(props.Account) }))
	refresh := ui.UseEvent(Prevent(func() { props.OnRefresh(props.Account) }))
	archLabel := "Archive"
	if props.Account.Archived {
		archLabel = "Restore"
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Account.Name,
				If(props.Stale, Span(Class("badge badge-prio prio-med"), Style(map[string]string{"margin-left": "0.5rem"}), "Stale")),
			),
			Span(Class("row-meta"), accountMeta(props.Account, props.Balance)),
		),
		Span(Class(amountClass(props.Balance)), fmtMoney(props.Balance)),
		If(!props.Account.Archived, Button(Class("btn"), Type("button"), Title("Mark balance as checked today"), OnClick(refresh), "Mark updated")),
		Button(Class("btn"), Type("button"), Title(archLabel+" account"), OnClick(arch), archLabel),
		Button(Class("btn-del"), Type("button"), Title("Delete account"), OnClick(del), "✕"),
	)
}

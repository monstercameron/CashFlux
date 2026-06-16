//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onCurr := ui.UseEvent(func(v string) { curr.Set(strings.ToUpper(v)) })
	onAmount := ui.UseEvent(func(v string) { amount.Set(v) })
	onType := ui.UseEvent(func(e ui.Event) { accType.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })

	bump := func() { rev.Set(rev.Get() + 1) }

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
		if err := app.PutAccount(acc); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		amount.Set("0")
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

	form := Section(Class("card"),
		H2(Class("card-title"), "Add account"),
		Form(Class("form-grid"), OnSubmit(add),
			Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
			Select(Class("field"), OnChange(onType), typeOptions),
			Select(Class("field"), OnChange(onOwner), ownerOptions),
			Input(Class("field"), Type("text"), Placeholder("Currency"), Value(curr.Get()), OnInput(onCurr)),
			Input(Class("field"), Type("number"), Placeholder("Opening balance"), Value(amount.Get()), Step("0.01"), OnInput(onAmount)),
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

	renderRow := func(ac domain.Account) ui.Node {
		bal, _ := ledger.Balance(ac, txns)
		return ui.CreateElement(AccountRow, accountRowProps{Account: ac, Balance: bal, OnDelete: deleteAccount, OnArchive: archiveAccount})
	}
	keyOf := func(ac domain.Account) any { return ac.ID }

	return Div(
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

type accountRowProps struct {
	Account   domain.Account
	Balance   money.Money
	OnDelete  func(string)
	OnArchive func(domain.Account)
}

// AccountRow is a per-account row component; it owns its action-handler hooks so
// the list can change without breaking hook ordering.
func AccountRow(props accountRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Account.ID) }))
	arch := ui.UseEvent(Prevent(func() { props.OnArchive(props.Account) }))
	archLabel := "Archive"
	if props.Account.Archived {
		archLabel = "Restore"
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), props.Account.Name),
			Span(Class("row-meta"), humanizeType(string(props.Account.Type))+" · "+props.Account.Currency),
		),
		Span(Class(amountClass(props.Balance)), fmtMoney(props.Balance)),
		Button(Class("btn"), Type("button"), Title(archLabel+" account"), OnClick(arch), archLabel),
		Button(Class("btn-del"), Type("button"), Title("Delete account"), OnClick(del), "✕"),
	)
}

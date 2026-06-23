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
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// AccountAddFormProps configures the AccountAddForm component.
type AccountAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
}

// AccountAddForm is the standalone add-an-account form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Accounts() for use in the AddHost modal.
func AccountAddForm(props AccountAddFormProps) ui.Node {
	return ui.CreateElement(accountAddForm, props)
}

func accountAddForm(props AccountAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	baseCur := app.Settings().BaseCurrency
	if baseCur == "" {
		baseCur = "USD"
	}

	name := ui.UseState("")
	curr := ui.UseState(baseCur)
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
	advOpen := ui.UseState(false)
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onCurr := ui.UseEvent(func(e ui.Event) { curr.Set(strings.ToUpper(e.GetValue())) })
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
	onToggleAdv := ui.UseEvent(func() { advOpen.Set(!advOpen.Get()) })

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
		// Reset fields.
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
		if props.OnDone != nil {
			props.OnDone()
		}
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

	return Form(css.Class("form-grid"), Attr("data-testid", "account-add-form"), OnSubmit(add),
		labeledField(uistate.T("common.name"),
			Input(append([]any{css.Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("acct-err", errMsg.Get())...)...)),
		labeledField(uistate.T("accounts.typeLabel"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("accounts.typeLabel")), OnChange(onType), typeOptions)),
		labeledField(uistate.T("common.owner"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("common.owner")), OnChange(onOwner), ownerOptions)),
		labeledField(uistate.T("accounts.currency"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("accounts.currency")), OnChange(onCurr), currencyOptions(app, curr.Get()))),
		labeledField(uistate.T("accounts.openingBalance"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(amount.Get()), Step("0.01"), OnInput(onAmount))),
		If(isLiab, labeledField(uistate.T("accounts.creditLimit"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Value(creditLimit.Get()), Step("0.01"), OnInput(onCreditLimit)))),
		If(isLiab, labeledField(uistate.T("accounts.apr"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Value(apr.Get()), Step("0.01"), OnInput(onApr)))),
		If(isLiab, labeledField(uistate.T("accounts.minPayment"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Value(minPayment.Get()), Step("0.01"), OnInput(onMinPayment)))),
		If(isLiab, labeledField(uistate.T("accounts.dueDay"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay")), Value(dueDay.Get()), OnInput(onDueDay)))),
		If(isLiab, labeledField(uistate.T("accounts.lender"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), Value(lender.Get()), OnInput(onLender)))),
		If(!isLiab, Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(advOpen.Get())), OnClick(onToggleAdv),
			IfElse(advOpen.Get(), Text("Hide advanced fields"), Text("Show advanced fields")))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.expReturn"),
			Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(expReturn.Get()), Step("0.01"), OnInput(onExpReturn)))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.liquidity"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liquidity.Get()), OnInput(onLiquidity)))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.stability"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stability.Get()), OnInput(onStability)))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.lockUntil"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntil")), Title(uistate.T("accounts.lockUntil")), Value(lockUntil.Get()), OnInput(onLockUntil)))),
		MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
		}),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.addTitle")),
		errText("acct-err", errMsg.Get()),
	)
}

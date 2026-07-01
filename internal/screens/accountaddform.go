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
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
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
	// A "single-currency household" has no FX rates configured and no accounts that
	// use a currency other than the base. In that case the currency picker adds noise
	// without value — hide it and default silently to the base currency (L37).
	singleCurrency := func() bool {
		if len(app.Settings().FXRates) > 0 {
			return false
		}
		for _, a := range app.Accounts() {
			if a.Currency != "" && a.Currency != baseCur {
				return false
			}
		}
		return true
	}()

	name := ui.UseState("")
	ev := useEntityVarField(accountVarKind, name, "")
	curr := ui.UseState(baseCur)
	amount := ui.UseState("0")
	accType := ui.UseState(string(domain.TypeChecking))
	owner := ui.UseState(domain.GroupOwnerID)
	creditLimit := ui.UseState("")
	apr := ui.UseState("")
	minPayment := ui.UseState("")
	dueDay := ui.UseState("")
	lender := ui.UseState("")
	institution := ui.UseState("")
	expReturn := ui.UseState("")
	liquidity := ui.UseState("")
	stability := ui.UseState("")
	lockUntil := ui.UseState("")
	advOpen := ui.UseState(false)
	splitOwn := ui.UseState(false)
	ownerShares := ui.UseState(map[string]int{})
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	// C78: single-currency households hide the currency picker (L37), which otherwise
	// makes the first foreign account impossible (chicken-egg: no rate → no picker →
	// no foreign account → no reason to add a rate). A "Use a different currency" link
	// reveals the full picker on demand so going multi-currency is always possible.
	revealCurr := ui.UseState(false)
	onRevealCurr := ui.UseEvent(Prevent(func() { revealCurr.Set(true) }))

	// onCurr/onType/onOwner hooks kept for stable hook ordering; SelectInput owns the
	// change event internally, so these event-handler hooks are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { curr.Set(strings.ToUpper(e.GetValue())) })
	onAmount := ui.UseEvent(func(v string) { amount.Set(v) })
	ui.UseEvent(func(e ui.Event) { accType.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onCreditLimit := ui.UseEvent(func(v string) { creditLimit.Set(v) })
	onApr := ui.UseEvent(func(v string) { apr.Set(v) })
	onMinPayment := ui.UseEvent(func(v string) { minPayment.Set(v) })
	onDueDay := ui.UseEvent(func(v string) { dueDay.Set(v) })
	onLender := ui.UseEvent(func(v string) { lender.Set(v) })
	onInstitution := ui.UseEvent(func(v string) { institution.Set(v) })
	onExpReturn := ui.UseEvent(func(v string) { expReturn.Set(v) })
	onLiquidity := ui.UseEvent(func(v string) { liquidity.Set(v) })
	onStability := ui.UseEvent(func(v string) { stability.Set(v) })
	onLockUntil := ui.UseEvent(func(v string) { lockUntil.Set(v) })
	onToggleAdv := ui.UseEvent(func() { advOpen.Set(!advOpen.Get()) })
	onToggleSplitOwn := ui.UseEvent(func() { splitOwn.Set(!splitOwn.Get()) })

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
		if warn := entityVarCollision(accountVarKind, accountVarEntities(app.Accounts()), "", ev.VarName.Get(), name.Get()); warn != "" {
			errMsg.Set(warn)
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
			VarName: strings.TrimSpace(ev.VarName.Get()),
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
		// MIA-extend (#445-10): normalise institution; pre-fill from lender when blank.
		inst := titleCaseWords(strings.TrimSpace(institution.Get()))
		if inst == "" && acc.Lender != "" {
			inst = titleCaseWords(acc.Lender)
		}
		acc.Institution = inst
		acc.Custom = customValuesToMap(accDefs, customVals.Get())
		if splitOwn.Get() {
			shares := cloneSharesMap(ownerShares.Get())
			sum := 0
			for _, v := range shares {
				sum += v
			}
			if sum != 100 {
				errMsg.Set(uistate.T("account.shareSumError", sum))
				return
			}
			acc.OwnershipShares = shares
		}
		if err := app.PutAccount(acc); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// Reset fields.
		name.Set("")
		ev.Reset()
		amount.Set("0")
		creditLimit.Set("")
		apr.Set("")
		minPayment.Set("")
		dueDay.Set("")
		lender.Set("")
		institution.Set("")
		expReturn.Set("")
		liquidity.Set("")
		lockUntil.Set("")
		stability.Set("")
		customVals.Set(map[string]string{})
		splitOwn.Set(false)
		ownerShares.Set(map[string]int{})
		errMsg.Set("")
		// The add modal lives in AddHost (a sibling of the Accounts screen), so
		// closing it only re-renders AddHost. Bump the shared data revision so the
		// Accounts list (which subscribes via UseDataRevision) shows the new account
		// immediately instead of only after a reload (C223/C71/R2).
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	typeOptions := uiw.OptionsFrom(domain.AllAccountTypes,
		func(t domain.AccountType) string { return string(t) },
		func(t domain.AccountType) string { return humanizeType(string(t)) },
		accType.Get())
	ownerOptions := ownerSelectOptions(app.Members(), owner.Get())

	isLiab := domain.AccountType(accType.Get()).Class() == domain.ClassLiability
	// C74: lock-until is meaningful for illiquid asset types (savings, investment,
	// retirement, crypto, other) where a maturity / lock date matters at creation time.
	// Everyday liquid accounts (checking, debit, cash) can still reach it via Advanced.
	at := domain.AccountType(accType.Get())
	isLockableAsset := at == domain.TypeSavings || at == domain.TypeInvestment ||
		at == domain.TypeRetirement || at == domain.TypeCrypto ||
		at == domain.TypeProperty || at == domain.TypeVehicle || at == domain.TypeOther

	return Form(css.Class("form-grid"), Attr("data-testid", "account-add-form"), OnSubmit(add),
		// C7: first-run framing — when this is the household's very first account,
		// explain in one friendly line what an account is and that nothing leaves the
		// device. Disappears once any account exists, so it never nags returning users.
		If(len(app.Accounts()) == 0,
			P(css.Class("notice", tw.Text12), Attr("data-testid", "account-firstrun-hint"),
				uistate.T("accounts.firstRunHint"))),
		// Name + Variable name stack full-width at the top (the account's identity), so the
		// var-name field reads directly under the name rather than in the grid's 2nd column.
		Div(Attr("style", "grid-column:1 / -1"),
			labeledField(uistate.T("common.name"),
				Input(append([]any{css.Class("field"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(ev.OnName)}, errAttrs("acct-err", errMsg.Get())...)...))),
		// Optional explicit variable name for formulas/widgets (autosuggested from the
		// name), with a live chip showing the generated handle + a collision warning.
		Div(Attr("style", "grid-column:1 / -1"),
			labeledField(uistate.T("accounts.varNameLabel"),
				entityVarField(accountVarKind, accountVarEntities(app.Accounts()), "", "account-add-varname", "account-add-varname-warn", ev.VarName.Get(), name.Get(), ev.OnVarName))),
		labeledField(uistate.T("accounts.typeLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   typeOptions,
				Selected:  accType.Get(),
				OnChange:  func(v string) { accType.Set(v) },
				AriaLabel: uistate.T("accounts.typeLabel"),
			})),
		// C30: the owner picker only offers "Everyone/Group" until members exist, so in
		// a 0-member household it's noise that defaults to a meaningless group. Hide it
		// (owner stays GroupOwnerID = shared) until at least one member is added.
		If(len(app.Members()) > 0, labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   ownerOptions,
				Selected:  owner.Get(),
				OnChange:  func(v string) { owner.Set(v) },
				AriaLabel: uistate.T("common.owner"),
			}))),
		// Split-ownership disclosure: available when there are 2+ members.
		// Per-member input rows use OwnerShareRow (a standalone component)
		// so no On* handler is called inside the MapKeyed loop (CLAUDE.md §gotchas).
		If(len(app.Members()) >= 2, func() ui.Node {
			shareSum := 0
			for _, v := range ownerShares.Get() {
				shareSum += v
			}
			// onShareChange is a plain func — not an On* hook — so it is safe
			// to capture as a prop inside MapKeyed.
			onShareChange := func(memberID string, valStr string) {
				n, _ := strconv.Atoi(valStr)
				m := ownerShares.Get()
				nm := make(map[string]int, len(m)+1)
				for k, v := range m {
					nm[k] = v
				}
				nm[memberID] = n
				ownerShares.Set(nm)
			}
			return Div(
				Button(css.Class("btn cf-adv-toggle"), Type("button"),
					Attr("aria-expanded", ariaBool(splitOwn.Get())),
					OnClick(onToggleSplitOwn),
					IfElse(splitOwn.Get(),
						Text(uistate.T("account.splitOwnership")+" ▴"),
						Text(uistate.T("account.splitOwnership")+" ▾"))),
				If(splitOwn.Get(), Div(
					P(css.Class("t-caption", tw.TextDim), uistate.T("account.splitOwnershipHint")),
					MapKeyed(app.Members(),
						func(m domain.Member) any { return m.ID },
						func(m domain.Member) ui.Node {
							return ui.CreateElement(OwnerShareRow, ownerShareRowProps{
								Member:   m,
								Share:    ownerShares.Get()[m.ID],
								OnChange: onShareChange,
							})
						}),
					If(shareSum != 100 && splitOwn.Get(),
						P(css.Class("err"), Attr("role", "alert"),
							uistate.T("account.shareSumError", shareSum))),
				)),
			)
		}()),
		If(!singleCurrency || revealCurr.Get(), labeledField(uistate.T("accounts.currency"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   currencyOptions(app, curr.Get()),
				Selected:  curr.Get(),
				OnChange:  func(v string) { curr.Set(strings.ToUpper(v)) },
				AriaLabel: uistate.T("accounts.currency"),
				TestID:    "account-currency-select",
			}))),
		// C78: reveal the currency picker on demand for single-currency households.
		If(singleCurrency && !revealCurr.Get(),
			Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "account-use-other-currency"),
				OnClick(onRevealCurr), uistate.T("accounts.useOtherCurrency"))),
		labeledField(uistate.T("accounts.openingBalance"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Value(amount.Get()), Step("0.01"), OnInput(onAmount))),
		// C27: explain what the opening balance is — and that for a card/loan you enter
		// the amount currently owed (it's tracked as a liability).
		P(css.Class(tw.TextFaint, tw.Text12), func() string {
			if isLiab {
				return uistate.T("accounts.openingBalanceHintLiab")
			}
			return uistate.T("accounts.openingBalanceHint")
		}()),
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
		// MIA-extend (#445-10): institution field shown for all account types.
		labeledField(uistate.T("accounts.institution"),
			uiw.Combobox(uiw.SuggestProps{
				Value:       institution.Get(),
				Placeholder: uistate.T("accounts.institutionHint"),
				AriaLabel:   uistate.T("accounts.institution"),
				OnInput:     onInstitution,
				Options:     domain.UniqueInstitutions(app.Accounts()),
				ListID:      "inst-list-add",
			})),
		If(!isLiab, Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(advOpen.Get())), OnClick(onToggleAdv),
			IfElse(advOpen.Get(), Text("Hide advanced fields"), Text("Show advanced fields")))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.expReturn"),
			Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Value(expReturn.Get()), Step("0.01"), OnInput(onExpReturn)))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.liquidity"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), Value(liquidity.Get()), OnInput(onLiquidity)))),
		If(!isLiab && advOpen.Get(), labeledField(uistate.T("accounts.stability"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "5"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), Value(stability.Get()), OnInput(onStability)))),
		// C74: lock-until is surfaced directly for lockable asset types (savings /
		// investment / retirement / crypto / other) — no Advanced toggle required.
		// Liquid everyday accounts (checking / debit / cash) can still reach it via
		// Advanced when it's genuinely useful (e.g. a locked flex-savings account
		// classified loosely as "checking"). Liabilities never show this field.
		If(isLockableAsset || (!isLiab && advOpen.Get()), labeledField(uistate.T("accounts.lockUntil"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntil")), Title(uistate.T("accounts.lockUntil")), Value(lockUntil.Get()), OnInput(onLockUntil)))),
		MapKeyed(accDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
		}),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("accounts.addTitle")),
		errText("acct-err", errMsg.Get()),
	)
}

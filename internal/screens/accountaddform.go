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
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// AccountAddFormProps configures the AccountAddForm component.
type AccountAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
	// PrefillLiability seeds the type as a credit card (a liability) instead of a
	// checking account, so opening this from the Debt page's "Add debt" lands on a
	// debt with the APR / limit / minimum / due-day fields already showing — not a
	// checking account someone adds as an asset by accident.
	PrefillLiability bool
}

// AccountAddForm is the standalone add-an-account form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Accounts() for use in the AddHost modal.
func AccountAddForm(props AccountAddFormProps) ui.Node {
	return ui.CreateElement(accountAddForm, props)
}

// acctCopyOptions builds the "Copy an existing account…" starter options (G5): a
// placeholder action label first, then every unarchived account.
func acctCopyOptions(accounts []domain.Account) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.copyExisting")}}
	for _, ac := range accounts {
		if !ac.Archived {
			opts = append(opts, uiw.SelectOption{Value: ac.ID, Label: ac.Name})
		}
	}
	return opts
}

// accountDraft is everything the add form collects. It is one typed value, so a copy or a reset is
// a single update and a field is named by the compiler, not by a string.
type accountDraft struct {
	Name           string
	Currency       string
	Amount         string
	Type           string
	Owner          string
	AsLiability    bool // an "Other" account counted as a debt
	CreditLimit    string
	APR            string
	MinPayment     string
	DueDay         string
	Lender         string
	Institution    string
	ExpectedReturn string
	Liquidity      string
	Stability      string
	LockUntil      string
	OwnerShares    map[string]int
	Custom         map[string]string
}

// newAccountDraft is the empty draft: the base currency, an opening balance of zero, and the type
// and owner the form starts from.
func newAccountDraft(baseCur, typ string) accountDraft {
	return accountDraft{
		Currency: baseCur, Amount: "0", Type: typ, Owner: domain.GroupOwnerID,
		OwnerShares: map[string]int{}, Custom: map[string]string{},
	}
}

// clearedAfterAdd keeps the currency, type, owner and the debt override for the next account, and
// clears everything the person typed for this one.
func clearedAfterAdd(d accountDraft) accountDraft {
	d.Name = ""
	d.Amount = "0"
	d.CreditLimit, d.APR, d.MinPayment, d.DueDay, d.Lender, d.Institution = "", "", "", "", "", ""
	d.ExpectedReturn, d.Liquidity, d.Stability, d.LockUntil = "", "", "", ""
	d.OwnerShares = map[string]int{}
	d.Custom = map[string]string{}
	return d
}

// copiedDraft prefills the draft from an existing account (the "copy an existing account" starter):
// the name gets a copy suffix, and the type, owner, institution and class-appropriate figures carry
// over. A foreign currency carries over too.
func copiedDraft(d accountDraft, ac domain.Account, baseCur string) accountDraft {
	d.Name = uistate.T("budgets.copySuffix", ac.Name)
	d.Type = string(ac.Type)
	d.Owner = ac.OwnerID
	d.Institution = ac.Institution
	if ac.Currency != "" && ac.Currency != baseCur {
		d.Currency = ac.Currency
	}
	d.AsLiability = ac.Type == domain.TypeOther && ac.Class == domain.ClassLiability
	if ac.Class == domain.ClassLiability {
		d.CreditLimit = moneyMajorOrEmpty(ac.CreditLimit, currency.Decimals(ac.Currency))
		d.APR = rateFieldValue(ac)
		d.MinPayment = moneyMajorOrEmpty(ac.MinPayment, currency.Decimals(ac.Currency))
		d.DueDay = intOrEmpty(ac.DueDayOfMonth)
		d.Lender = ac.Lender
	} else {
		d.ExpectedReturn = floatOrEmpty(ac.ExpectedReturnAPR)
		d.Liquidity = intOrEmpty(ac.LiquidityScore)
		d.Stability = intOrEmpty(ac.StabilityScore)
	}
	return d
}

// accountFromDraft turns a draft into the account to save. It returns the message to show, or "" when
// the draft is good. The message is a translated string, so the caller can show it as it is.
func accountFromDraft(d accountDraft, accDefs []customfields.Def, varName string, now time.Time) (domain.Account, string) {
	c := strings.ToUpper(strings.TrimSpace(d.Currency))
	amt, err := money.ParseMinor(strings.TrimSpace(d.Amount), currency.Decimals(c))
	if err != nil {
		return domain.Account{}, uistate.T("accounts.invalidOpening")
	}
	typ := domain.AccountType(d.Type)
	class := typ.Class()
	if typ == domain.TypeOther && d.AsLiability {
		class = domain.ClassLiability
	}
	scope := domain.ScopeIndividual
	if d.Owner == domain.GroupOwnerID {
		scope = domain.ScopeShared
	}
	acc := domain.Account{
		ID: id.New(), Name: strings.TrimSpace(d.Name), OwnerID: d.Owner, Scope: scope,
		Class: class, Type: typ, Currency: c,
		OpeningBalance: money.New(amt, c), BalanceAsOf: now,
		VarName: strings.TrimSpace(varName),
	}
	if class == domain.ClassLiability {
		if cl, e := money.ParseMinor(strings.TrimSpace(d.CreditLimit), currency.Decimals(c)); e == nil && cl > 0 {
			acc.CreditLimit = money.New(cl, c)
		}
		acc = applyRateAPR(acc, d.APR)
		if mp, e := money.ParseMinor(strings.TrimSpace(d.MinPayment), currency.Decimals(c)); e == nil && mp > 0 {
			acc.MinPayment = money.New(mp, c)
		}
		if dd, e := strconv.Atoi(strings.TrimSpace(d.DueDay)); e == nil {
			acc.DueDayOfMonth = dd
		}
		acc.Lender = strings.TrimSpace(d.Lender)
	} else {
		if r, e := strconv.ParseFloat(strings.TrimSpace(d.ExpectedReturn), 64); e == nil {
			acc.ExpectedReturnAPR = r
		}
		if l, e := strconv.Atoi(strings.TrimSpace(d.Liquidity)); e == nil {
			acc.LiquidityScore = l
		}
		if s, e := strconv.Atoi(strings.TrimSpace(d.Stability)); e == nil {
			acc.StabilityScore = s
		}
		if lu := strings.TrimSpace(d.LockUntil); lu != "" {
			if date, e := dateutil.ParseDate(lu); e == nil {
				acc.LockUntil = date
			}
		}
	}
	// MIA-extend (#445-10): normalise institution; pre-fill from lender when blank.
	inst := titleCaseWords(strings.TrimSpace(d.Institution))
	if inst == "" && acc.Lender != "" {
		inst = titleCaseWords(acc.Lender)
	}
	acc.Institution = inst
	custom, customIssues := customValuesToMap(accDefs, d.Custom)
	if len(customIssues) > 0 {
		return domain.Account{}, strings.Join(customIssues, " ")
	}
	acc.Custom = custom
	if len(d.OwnerShares) > 0 {
		shares := cloneSharesMap(d.OwnerShares)
		sum := 0
		for _, v := range shares {
			sum += v
		}
		if sum != 100 {
			return domain.Account{}, uistate.T("account.shareSumError", sum)
		}
		acc.OwnershipShares = shares
	}
	return acc, ""
}

// draftTextOf binds one text field of the draft. It satisfies the get and set shape the shared
// field helpers take, so a field is read and written the same way wherever it is used. The field is
// named by a selector, so the compiler checks the name.
func draftTextOf(f ui.Form[accountDraft], field func(*accountDraft) *string) draftText {
	return draftText{
		get: func() string {
			d := f.Get()
			return *field(&d)
		},
		set: func(v string) {
			f.Update(func(d accountDraft) accountDraft {
				*field(&d) = v
				return d
			})
		},
	}
}

// draftText is one text field of the draft, read and written through closures.
type draftText struct {
	get func() string
	set func(string)
}

func (t draftText) Get() string  { return t.get() }
func (t draftText) Set(v string) { t.set(v) }

// accountAddForm is the add-an-account form. Its state is one typed draft; the presentation toggles
// (advanced, split ownership, the currency reveal) stay as separate state because they are not
// account data.
//
// Hidden sections are not built. Each one is a native Go condition, so its nodes and props exist
// only while it is shown, and the components inside keep their hooks in stable positions.
func accountAddForm(props AccountAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	// One snapshot of the accounts and members for this render. Handlers read the live lists instead,
	// so a submit never works from a stale copy.
	accounts := app.Accounts()
	members := app.Members()

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
		for _, a := range accounts {
			if a.Currency != "" && a.Currency != baseCur {
				return false
			}
		}
		return true
	}()

	defaultType := string(domain.TypeChecking)
	if props.PrefillLiability {
		defaultType = string(domain.TypeCreditCard)
	}
	form := ui.UseForm(newAccountDraft(baseCur, defaultType))
	d := form.Get()
	focus := ui.UseFocusManager()
	announce := ui.UseAnnouncer()
	nameID := strings.ReplaceAll(ui.UseId(), ":", "-") + "-name"
	errID := strings.ReplaceAll(ui.UseId(), ":", "-") + "-err"

	name := draftTextOf(form, func(d *accountDraft) *string { return &d.Name })
	ev := useEntityVarField(accountVarKind, name, "")

	advOpen := ui.UseState(false)
	splitOwn := ui.UseState(false)
	// C78: single-currency households hide the currency picker (L37), which otherwise
	// makes the first foreign account impossible (chicken-egg: no rate → no picker →
	// no foreign account → no reason to add a rate). A "Use a different currency" link
	// reveals the full picker on demand so going multi-currency is always possible.
	revealCurr := ui.UseState(false)
	onRevealCurr := ui.UseEvent(Prevent(func() { revealCurr.Set(true) }))
	onToggleAdv := ui.UseEvent(func() { advOpen.Set(!advOpen.Get()) })
	onToggleSplitOwn := ui.UseEvent(func() { splitOwn.Set(!splitOwn.Get()) })
	onToggleAsLiab := ui.UseEvent(func() {
		form.Update(func(d accountDraft) accountDraft {
			d.AsLiability = !d.AsLiability
			return d
		})
	})

	// The field setters the child controls call. Each is memoized so a control's props keep their
	// identity from one keystroke to the next. The setters capture only the form, whose state is
	// stable, so an empty dependency list is correct.
	onTypeChange := ui.UseCallback(func(v string) {
		draftTextOf(form, func(d *accountDraft) *string { return &d.Type }).Set(v)
	})
	onCurrencyChange := ui.UseCallback(func(v string) {
		draftTextOf(form, func(d *accountDraft) *string { return &d.Currency }).Set(strings.ToUpper(v))
	})
	onInstitutionChange := ui.UseCallback(func(v string) {
		draftTextOf(form, func(d *accountDraft) *string { return &d.Institution }).Set(v)
	})
	onCustom := ui.UseCallback(func(key, value string) {
		form.Update(func(d accountDraft) accountDraft {
			nm := make(map[string]string, len(d.Custom)+1)
			for k, v := range d.Custom {
				nm[k] = v
			}
			nm[key] = value
			d.Custom = nm
			return d
		})
	})

	// The option lists are derived from the accounts, members and currencies. They are rebuilt only
	// when the data revision moves, not on every keystroke. The revision is the change token: the
	// lists hold slices and maps, which compare by identity, so their own values would never match.
	rev := uistate.UseDataRevision().Get()
	copyOptions := ui.UseMemo(func() []uiw.SelectOption { return acctCopyOptions(app.Accounts()) }, rev)
	institutionNames := ui.UseMemo(func() []string { return domain.UniqueInstitutions(app.Accounts()) }, rev)
	typeOptions := ui.UseMemo(func() []uiw.SelectOption {
		return uiw.OptionsFrom(domain.AllAccountTypes,
			func(t domain.AccountType) string { return string(t) },
			func(t domain.AccountType) string { return humanizeType(string(t)) },
			d.Type)
	}, d.Type)
	ownerOptions := ui.UseMemo(func() []uiw.SelectOption { return ownerSelectOptions(app.Members(), d.Owner) }, rev, d.Owner)
	currencyOpts := ui.UseMemo(func() []uiw.SelectOption { return currencyOptions(app, d.Currency) }, rev, d.Currency)

	accDefs := app.CustomFieldDefsFor("account")

	// G5: copy an existing account — prefills the whole draft in one update, so the copy and its
	// currency reveal land together.
	copyFrom := ui.UseCallback(func(aid string) {
		for _, ac := range app.Accounts() {
			if ac.ID != aid {
				continue
			}
			form.Update(func(d accountDraft) accountDraft { return copiedDraft(d, ac, baseCur) })
			if ac.Currency != "" && ac.Currency != baseCur {
				revealCurr.Set(true)
			}
			return
		}
	}, baseCur)

	fieldIDs := map[string]string{"Name": nameID}
	add := ui.UseEvent(Prevent(func() {
		d := form.Get()
		// G6: product copy, not a leaked validator string.
		if strings.TrimSpace(d.Name) == "" {
			form.SetErrors(ui.FieldErrors{"Name": uistate.T("accounts.nameRequired")})
			focus.FocusFirstError(form.Errors(), fieldIDs, "Name")
			return
		}
		form.SetErrors(ui.FieldErrors{})
		if warn := entityVarCollision(accountVarKind, accountVarEntities(app.Accounts()), "", ev.VarName.Get(), d.Name); warn != "" {
			form.SetFormError(warn)
			return
		}
		acc, msg := accountFromDraft(d, accDefs, ev.VarName.Get(), time.Now())
		if msg != "" {
			form.SetFormError(msg)
			return
		}
		if err := app.PutAccount(acc); err != nil {
			form.SetFormError(err.Error())
			return
		}
		form.Update(clearedAfterAdd)
		form.SetFormError("")
		ev.Reset()
		splitOwn.Set(false)
		// The add modal lives in AddHost (a sibling of the Accounts screen), so
		// closing it only re-renders AddHost. Bump the shared data revision so the
		// Accounts list (which subscribes via UseDataRevision) shows the new account
		// immediately instead of only after a reload (C223/C71/R2).
		uistate.BumpDataRevision()
		announce.Polite(uistate.T("accounts.addedAnnounce", acc.Name))
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	isOther := domain.AccountType(d.Type) == domain.TypeOther
	isLiab := domain.AccountType(d.Type).Class() == domain.ClassLiability || (isOther && d.AsLiability)
	// C74: lock-until is meaningful for illiquid asset types (savings, investment,
	// retirement, crypto, other) where a maturity / lock date matters at creation time.
	// Everyday liquid accounts (checking, debit, cash) can still reach it via Advanced.
	at := domain.AccountType(d.Type)
	isLockableAsset := at == domain.TypeSavings || at == domain.TypeInvestment ||
		at == domain.TypeRetirement || at == domain.TypeCrypto ||
		at == domain.TypeProperty || at == domain.TypeVehicle || at == domain.TypeOther

	message := form.FormError()
	if message == "" {
		message = form.FieldMessage("Name")
	}

	// Each optional section is built only when it is shown.
	var firstRun, copyStarter, otherToggle, ownership, currency, useOtherCurrency ui.Node
	var liability, advanced, assetFields, lockUntil ui.Node
	if len(accounts) == 0 {
		// C7: first-run framing — when this is the household's very first account,
		// explain in one friendly line what an account is and that nothing leaves the
		// device. Disappears once any account exists, so it never nags returning users.
		firstRun = P(css.Class("notice", tw.Text12), Attr("data-testid", "account-firstrun-hint"),
			uistate.T("accounts.firstRunHint"))
	}
	if len(accounts) > 0 {
		// G5: the copy-existing starter (only once accounts exist to copy), demoted
		// below the name field.
		copyStarter = Div(css.Class("acct-add-wide"),
			labeledField(uistate.T("accounts.copyExisting"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: copyOptions, Selected: "", TestID: "account-copy-existing",
					OnChange: copyFrom, AriaLabel: uistate.T("accounts.copyExisting"),
				})))
	}
	if isOther {
		// "Other" has no natural class — let the user mark it a liability (debt) so the
		// net-worth/debt formulas count it.
		otherToggle = Div(css.Class("acct-add-wide"),
			Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-add-as-liability"), CheckedIf(d.AsLiability), OnChange(onToggleAsLiab)),
				Div(css.Class("row-main"),
					Span(uistate.T("accounts.countAsLiability")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("accounts.countAsLiabilityHint")))))
	}
	ownership = ui.CreateElement(accountOwnershipFields, accountOwnershipProps{
		Form: form, Members: members, Owner: ownerOptions, Split: splitOwn, OnToggleSplit: onToggleSplitOwn,
	})
	if !singleCurrency || revealCurr.Get() {
		currency = labeledField(uistate.T("accounts.currency"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   currencyOpts,
				Selected:  d.Currency,
				OnChange:  onCurrencyChange,
				AriaLabel: uistate.T("accounts.currency"),
				TestID:    "account-currency-select",
			}))
	}
	// C78: reveal the currency picker on demand for single-currency households.
	if singleCurrency && !revealCurr.Get() {
		useOtherCurrency = Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "account-use-other-currency"),
			OnClick(onRevealCurr), uistate.T("accounts.useOtherCurrency"))
	}
	if isLiab {
		liability = ui.CreateElement(accountLiabilityFields, accountLiabilityProps{Form: form})
	}
	if advOpen.Get() {
		advanced = ui.CreateElement(accountAdvancedFields, accountAdvancedProps{Form: form, EV: ev, Accounts: accounts, Name: d.Name})
	}
	if !isLiab && advOpen.Get() {
		assetFields = ui.CreateElement(accountAssetFields, accountAssetProps{Form: form})
	}
	// C74: lock-until is surfaced directly for lockable asset types (savings /
	// investment / retirement / crypto / other) — no Advanced toggle required.
	// Liquid everyday accounts (checking / debit / cash) can still reach it via
	// Advanced when it's genuinely useful (e.g. a locked flex-savings account
	// classified loosely as "checking"). Liabilities never show this field.
	if isLockableAsset || (!isLiab && advOpen.Get()) {
		lockUntil = labeledField(uistate.T("accounts.lockUntil"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("accounts.lockUntil")), Title(uistate.T("accounts.lockUntil")),
				BindFunc(draftTextOf(form, func(d *accountDraft) *string { return &d.LockUntil }).Get, draftTextOf(form, func(d *accountDraft) *string { return &d.LockUntil }).Set)))
	}

	return Form(css.Class("form-grid"), Attr("id", "account-add-form"), Attr("data-testid", "account-add-form"), OnSubmit(add),
		firstRun,
		// Name leads the form — the main path. (The "Copy an existing account…"
		// shortcut is a secondary convenience, so it sits BELOW the name, not above
		// it: an edge case shouldn't be the first field a new user meets.)
		Div(css.Class("acct-add-wide"),
			labeledField(uistate.T("common.name"),
				Input(append([]any{css.Class("field"), Type("text"), Attr("id", nameID), Attr("data-testid", "account-add-name"),
					Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), OnInput(ev.OnName), ClassIf(message != "", "field-invalid"), AttrIf(message != "", "aria-invalid", "true"), AttrIf(message != "", "aria-describedby", errID)},
					uiw.FieldValue(name.Get()))...))),
		copyStarter,
		labeledField(uistate.T("accounts.typeLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   typeOptions,
				Selected:  d.Type,
				OnChange:  onTypeChange,
				AriaLabel: uistate.T("accounts.typeLabel"),
				TestID:    "account-type-select",
			})),
		otherToggle,
		ownership,
		currency,
		useOtherCurrency,
		labeledField(uistate.T("accounts.openingBalance"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("accounts.openingBalance")), Step("0.01"),
				BindFunc(draftTextOf(form, func(d *accountDraft) *string { return &d.Amount }).Get, draftTextOf(form, func(d *accountDraft) *string { return &d.Amount }).Set))),
		// C27: explain what the opening balance is — and that for a card/loan you enter
		// the amount currently owed (it's tracked as a liability).
		P(css.Class(tw.TextFaint, tw.Text12), func() string {
			if isLiab {
				return uistate.T("accounts.openingBalanceHintLiab")
			}
			return uistate.T("accounts.openingBalanceHint")
		}()),
		liability,
		// MIA-extend (#445-10): institution field shown for all account types.
		labeledField(uistate.T("accounts.institution"),
			uiw.Combobox(uiw.SuggestProps{
				Value:       d.Institution,
				Placeholder: uistate.T("accounts.institutionHint"),
				AriaLabel:   uistate.T("accounts.institution"),
				OnInput:     onInstitutionChange,
				Options:     institutionNames,
				ListID:      "inst-list-add",
			})),
		Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "account-add-advanced"), Attr("aria-expanded", ariaBool(advOpen.Get())), OnClick(onToggleAdv),
			IfElse(advOpen.Get(), Text(uistate.T("accounts.hideAdvanced")), Text(uistate.T("accounts.showAdvanced")))),
		advanced,
		assetFields,
		lockUntil,
		MapKeyed(accDefs, func(def customfields.Def) any { return def.ID }, func(def customfields.Def) ui.Node {
			return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: def, Value: d.Custom[def.Key], OnChange: onCustom})
		}),
		errText(errID, message),
		announce.Region(),
	)
}

// accountOwnershipProps configures the owner picker and the split-ownership editor.
type accountOwnershipProps struct {
	Form          ui.Form[accountDraft]
	Members       []domain.Member
	Owner         []uiw.SelectOption
	Split         ui.State[bool]
	OnToggleSplit ui.Handler
}

// accountOwnershipFields is the owner picker and, with two or more members, the split-ownership
// disclosure. The disclosure is built only when there are members to split between.
func accountOwnershipFields(props accountOwnershipProps) ui.Node {
	owner := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.Owner })
	// C30: the owner picker only offers "Everyone/Group" until members exist, so in
	// a 0-member household it's noise that defaults to a meaningless group. Hide it
	// (owner stays GroupOwnerID = shared) until at least one member is added.
	var ownerPicker, split ui.Node
	if len(props.Members) > 0 {
		ownerPicker = labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.Owner,
				Selected:  props.Form.Get().Owner,
				OnChange:  func(v string) { owner.Set(v) },
				AriaLabel: uistate.T("common.owner"),
			}))
	}
	// Split-ownership disclosure: available when there are 2+ members.
	if len(props.Members) >= 2 {
		split = ui.CreateElement(accountSplitFields, accountSplitProps{
			Form: props.Form, Members: props.Members, Split: props.Split, OnToggleSplit: props.OnToggleSplit,
		})
	}
	return Fragment(ownerPicker, split)
}

// accountSplitProps configures the split-ownership editor.
type accountSplitProps struct {
	Form          ui.Form[accountDraft]
	Members       []domain.Member
	Split         ui.State[bool]
	OnToggleSplit ui.Handler
}

// accountSplitFields is the split-ownership disclosure. Once it is open, it builds one share row per
// member; while it is closed, the rows are not built at all.
func accountSplitFields(props accountSplitProps) ui.Node {
	open := props.Split.Get()
	// onShareChange is a plain func, not an On* hook, so it is safe to pass as a prop inside MapKeyed.
	// It is memoized so the share rows keep their identity between keystrokes.
	onShareChange := ui.UseCallback(func(memberID string, valStr string) {
		n, _ := strconv.Atoi(valStr)
		props.Form.Update(func(d accountDraft) accountDraft {
			nm := make(map[string]int, len(d.OwnerShares)+1)
			for k, v := range d.OwnerShares {
				nm[k] = v
			}
			nm[memberID] = n
			d.OwnerShares = nm
			return d
		})
	})
	var editor ui.Node
	if open {
		shares := props.Form.Get().OwnerShares
		shareSum := 0
		for _, v := range shares {
			shareSum += v
		}
		editor = Div(
			P(css.Class("t-caption", tw.TextDim), uistate.T("account.splitOwnershipHint")),
			MapKeyed(props.Members,
				func(m domain.Member) any { return m.ID },
				func(m domain.Member) ui.Node {
					return ui.CreateElement(OwnerShareRow, ownerShareRowProps{
						Member:   m,
						Share:    shares[m.ID],
						OnChange: onShareChange,
					})
				}),
			If(shareSum != 100,
				P(css.Class("err"), Attr("role", "alert"),
					uistate.T("account.shareSumError", shareSum))),
		)
	}
	return Div(
		Button(css.Class("btn cf-adv-toggle"), Type("button"),
			Attr("aria-expanded", ariaBool(open)),
			OnClick(props.OnToggleSplit),
			IfElse(open,
				Text(uistate.T("account.splitOwnership")+" ▴"),
				Text(uistate.T("account.splitOwnership")+" ▾"))),
		editor,
	)
}

// accountLiabilityProps configures the debt-only fields.
type accountLiabilityProps struct {
	Form ui.Form[accountDraft]
}

// accountLiabilityFields is the credit limit, APR, minimum payment, due day and lender of a debt.
func accountLiabilityFields(props accountLiabilityProps) ui.Node {
	field := func(sel func(*accountDraft) *string) draftText { return draftTextOf(props.Form, sel) }
	limit := field(func(d *accountDraft) *string { return &d.CreditLimit })
	apr := field(func(d *accountDraft) *string { return &d.APR })
	minPay := field(func(d *accountDraft) *string { return &d.MinPayment })
	due := field(func(d *accountDraft) *string { return &d.DueDay })
	lender := field(func(d *accountDraft) *string { return &d.Lender })
	return Fragment(
		labeledField(uistate.T("accounts.creditLimit"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Step("0.01"), BindFunc(limit.Get, limit.Set))),
		labeledField(uistate.T("accounts.apr"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Step("0.01"), BindFunc(apr.Get, apr.Set))),
		labeledField(uistate.T("accounts.minPayment"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Step("0.01"), BindFunc(minPay.Get, minPay.Set))),
		labeledField(uistate.T("accounts.dueDay"),
			Input(css.Class("field"), Type("number"), Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay")), BindFunc(due.Get, due.Set))),
		labeledField(uistate.T("accounts.lender"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), BindFunc(lender.Get, lender.Set))),
	)
}

// accountAssetProps configures the asset-only figures.
type accountAssetProps struct {
	Form ui.Form[accountDraft]
}

// accountAssetFields is the expected return, liquidity and stability of an asset, shown in Advanced.
func accountAssetFields(props accountAssetProps) ui.Node {
	ret := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.ExpectedReturn })
	liq := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.Liquidity })
	stab := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.Stability })
	return Fragment(
		labeledField(uistate.T("accounts.expReturn"),
			Input(css.Class("field"), Type("number"), Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Step("0.01"), BindFunc(ret.Get, ret.Set))),
		labeledField(uistate.T("accounts.liquidity"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Attr("max", "100"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")), BindFunc(liq.Get, liq.Set))),
		labeledField(uistate.T("accounts.stability"),
			Input(css.Class("field"), Type("number"), Attr("min", "0"), Attr("max", "100"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")), BindFunc(stab.Get, stab.Set))),
	)
}

// accountAdvancedProps configures the Advanced disclosure's contents.
type accountAdvancedProps struct {
	Form     ui.Form[accountDraft]
	EV       entityVarState
	Accounts []domain.Account
	Name     string
}

// accountAdvancedFields is the optional variable name. It is engine plumbing, so it sits behind the
// disclosure, the same as the budgets and goals add forms.
func accountAdvancedFields(props accountAdvancedProps) ui.Node {
	return Div(css.Class("acct-add-wide"),
		labeledField(uistate.T("accounts.varNameLabel"),
			entityVarField(accountVarKind, accountVarEntities(props.Accounts), "", "account-add-varname", "account-add-varname-warn", props.EV.VarName.Get(), props.Name, props.EV.OnVarName)))
}

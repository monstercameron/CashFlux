// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
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
	// debt with the APR / limit / minimum / due-day fields already showing, not a
	// checking account someone adds as an asset by accident.
	PrefillLiability bool
}

// AccountAddForm is the standalone add-an-account form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. The AddHost modal renders it.
func AccountAddForm(props AccountAddFormProps) ui.Node {
	return ui.CreateElement(accountAddForm, props)
}

// acctCopyOptions builds the "Copy an existing account…" starter options: a
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
	InstitutionID  string // the institution-directory entry the institution was copied from, if any
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
	d.CreditLimit, d.APR, d.MinPayment, d.DueDay, d.Lender, d.Institution, d.InstitutionID = "", "", "", "", "", "", ""
	d.ExpectedReturn, d.Liquidity, d.Stability, d.LockUntil = "", "", "", ""
	d.OwnerShares = map[string]int{}
	d.Custom = map[string]string{}
	return d
}

// copiedDraft prefills the draft from an existing account (the "copy an existing account" starter):
// the name is copyName, and the type, owner, institution, currency and class-appropriate figures
// carry over; the figures of the other class are cleared, and so are the ownership shares and the
// custom values. The currency is always the source account's,
// or baseCur when the source has none, so copying a foreign account and then a base-currency one does
// not leave the foreign currency behind, and the money figures are written with that currency's
// decimals.
func copiedDraft(d accountDraft, ac domain.Account, baseCur, copyName string) accountDraft {
	d.Name = copyName
	d.Type = string(ac.Type)
	d.Owner = ac.OwnerID
	d.Institution = ac.Institution
	d.InstitutionID = ac.InstitutionID
	d.Currency = ac.Currency
	if d.Currency == "" {
		d.Currency = baseCur
	}
	d.AsLiability = accountedit.CountsAsDebt(ac.Type, ac.Class)
	d.CreditLimit, d.APR, d.MinPayment, d.DueDay, d.Lender = "", "", "", "", ""
	d.ExpectedReturn, d.Liquidity, d.Stability, d.LockUntil = "", "", "", ""
	d.OwnerShares = map[string]int{}
	d.Custom = map[string]string{}
	decimals := currency.Decimals(d.Currency)
	if accountedit.ClassOf(ac.Type, d.AsLiability) == domain.ClassLiability {
		d.CreditLimit = moneyMajorOrEmpty(ac.CreditLimit, decimals)
		d.APR = rateFieldValue(ac)
		d.MinPayment = moneyMajorOrEmpty(ac.MinPayment, decimals)
		d.DueDay = intOrEmpty(ac.DueDayOfMonth)
		d.Lender = ac.Lender
	} else {
		d.ExpectedReturn = floatOrEmpty(ac.ExpectedReturnAPR)
		d.Liquidity = intOrEmpty(ac.LiquidityScore)
		d.Stability = intOrEmpty(ac.StabilityScore)
	}
	return d
}

// draftFields is the add form's draft as the shared account builder reads it. memberIDs lists the
// members that exist.
func draftFields(d accountDraft, split bool, varName string, memberIDs []string) accountedit.Fields {
	return accountedit.Fields{
		Name: d.Name, Type: d.Type, Owner: d.Owner, Currency: d.Currency, Opening: d.Amount,
		AsLiability: d.AsLiability, CreditLimit: d.CreditLimit, APR: d.APR, MinPayment: d.MinPayment,
		DueDay: d.DueDay, Lender: d.Lender, Institution: d.Institution, InstitutionID: d.InstitutionID,
		ExpectedReturn: d.ExpectedReturn, Liquidity: d.Liquidity, Stability: d.Stability, LockUntil: d.LockUntil,
		VarName: varName, Shares: sharesOrNil(d.OwnerShares, split), MemberIDs: memberIDs,
	}
}

// sharesOrNil returns the split-ownership map while the split is on, even when no share is entered yet,
// so the builder reports a total that is not 100 and the save is refused. It returns nil while the
// split is off. The add and edit forms both read it, so what the view shows and what the save accepts
// agree.
func sharesOrNil(shares map[string]int, split bool) map[string]int {
	if !split {
		return nil
	}
	if shares == nil {
		return map[string]int{}
	}
	return shares
}

// updateDraft applies an edit the person made to the draft. It first drops a save failure the form
// is still showing, because that message was about the draft as it stood before this edit.
func updateDraft(f ui.Form[accountDraft], edit func(accountDraft) accountDraft) {
	if f.FormError() != "" {
		f.SetFormError("")
	}
	f.Update(edit)
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
			updateDraft(f, func(d accountDraft) accountDraft {
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
// Every hook runs before the form checks that the app is ready, so the hook count does not depend
// on whether the app has loaded. Hidden sections are not built. Each one is a native Go condition,
// and an input that registers a handler lives in a component of its own, so the components keep
// their hooks in stable positions whichever sections are shown.
//
// Once a save has been refused, the form checks the draft on every render and shows what is still
// wrong, so a message goes away as soon as its field is fixed.
func accountAddForm(props AccountAddFormProps) ui.Node {
	app := appstate.Default

	// One snapshot of the accounts and members for this render. Handlers read the live lists instead,
	// so a submit never works from a stale copy.
	var accounts []domain.Account
	var members []domain.Member
	baseCur := accountedit.DefaultBaseCurrency
	fxConfigured := false
	if app != nil {
		accounts = app.Accounts()
		members = app.Members()
		settings := app.Settings()
		baseCur = accountedit.BaseCurrencyOrDefault(settings.BaseCurrency)
		fxConfigured = len(settings.FXRates) > 0
	}
	// A single-currency household has no FX rates configured and no accounts that use a currency
	// other than the base. The currency picker adds noise there, so it is hidden and the currency
	// defaults silently to the base.
	singleCurrency := !fxConfigured
	for _, a := range accounts {
		if a.Currency != "" && a.Currency != baseCur {
			singleCurrency = false
		}
	}

	defaultType := string(domain.TypeChecking)
	if props.PrefillLiability {
		defaultType = string(domain.TypeCreditCard)
	}
	form := ui.UseForm(newAccountDraft(baseCur, defaultType))
	d := form.Get()
	idBase := strings.ReplaceAll(ui.UseId(), ":", "-")
	nameID, errID := idBase+"-name", idBase+"-err"
	inputIDs := accountInputIDs(idBase, nameID)
	moveToProblem := useProblemFocus(errID)

	// Each draft field is bound once and shared by the render, the setters and the sub-forms.
	name := draftTextOf(form, func(d *accountDraft) *string { return &d.Name })
	typeField := draftTextOf(form, func(d *accountDraft) *string { return &d.Type })
	currencyField := draftTextOf(form, func(d *accountDraft) *string { return &d.Currency })
	ownerField := draftTextOf(form, func(d *accountDraft) *string { return &d.Owner })
	amountField := draftTextOf(form, func(d *accountDraft) *string { return &d.Amount })
	ev := useEntityVarField(accountVarKind, name, "")

	advOpen := ui.UseState(false)
	splitOwn := ui.UseState(false)
	// attempted is set by the first refused save. From then on the form shows what is wrong with the
	// draft as it stands.
	attempted := ui.UseState(false)
	// Single-currency households hide the currency picker, which would otherwise make the first
	// foreign account impossible (no rate, so no picker, so no foreign account, so no reason to add
	// a rate). A "Use a different currency" link reveals the full picker on demand.
	revealCurr := ui.UseState(false)
	onRevealCurr := ui.UseEvent(Prevent(func() { revealCurr.Set(true) }))
	onToggleAdv := ui.UseEvent(func() { advOpen.Set(!advOpen.Get()) })
	onToggleSplitOwn := ui.UseEvent(func() { splitOwn.Set(!splitOwn.Get()) })
	onToggleAsLiab := ui.UseEvent(func() {
		updateDraft(form, func(d accountDraft) accountDraft {
			d.AsLiability = !d.AsLiability
			return d
		})
	})

	// The field setters the child controls call. Each is memoized so a control's props keep their
	// identity from one keystroke to the next. The setters capture only the form, whose state is
	// stable, so an empty dependency list is correct.
	onTypeChange := ui.UseCallback(func(v string) { typeField.Set(v) })
	onCurrencyChange := ui.UseCallback(func(v string) { currencyField.Set(strings.ToUpper(v)) })
	// Typing in the institution box makes it free text again, so the directory entry it was copied
	// from no longer describes it.
	onInstitutionChange := ui.UseCallback(func(v string) {
		updateDraft(form, func(d accountDraft) accountDraft {
			d.Institution, d.InstitutionID = v, ""
			return d
		})
	})
	onOwnerChange := ui.UseCallback(func(v string) { ownerField.Set(v) })
	onCustom := ui.UseCallback(func(key, value string) {
		updateDraft(form, func(d accountDraft) accountDraft {
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
	copyOptions := ui.UseMemo(func() []uiw.SelectOption { return acctCopyOptions(accounts) }, rev)
	institutionNames := ui.UseMemo(func() []string { return domain.UniqueInstitutions(accounts) }, rev)
	typeOptions := ui.UseMemo(func() []uiw.SelectOption {
		return uiw.OptionsFrom(domain.AllAccountTypes,
			func(t domain.AccountType) string { return string(t) },
			func(t domain.AccountType) string { return humanizeType(string(t)) },
			d.Type)
	}, d.Type)
	ownerOptions := ui.UseMemo(func() []uiw.SelectOption { return ownerSelectOptions(members, d.Owner) }, rev, d.Owner)
	currencyOpts := ui.UseMemo(func() []uiw.SelectOption {
		if app == nil {
			return nil
		}
		return currencyOptions(app, d.Currency)
	}, rev, d.Currency)

	// Copy an existing account: prefills the whole draft in one update, so the copy and its
	// currency reveal land together.
	copyFrom := ui.UseCallback(func(aid string) {
		if app == nil {
			return
		}
		for _, ac := range app.Accounts() {
			if ac.ID != aid {
				continue
			}
			copyName := uistate.T("budgets.copySuffix", ac.Name)
			updateDraft(form, func(d accountDraft) accountDraft { return copiedDraft(d, ac, baseCur, copyName) })
			ev.SuggestFor(copyName)
			if ac.Currency != "" && ac.Currency != baseCur {
				revealCurr.Set(true)
			}
			return
		}
	}, baseCur)

	// checkDraft checks the draft the way a save does. It reads the live lists, so it is right inside
	// a handler as well as in the render.
	checkDraft := func(d accountDraft) (domain.Account, formProblem) {
		return checkAccountForm(domain.Account{BalanceAsOf: time.Now()},
			draftFields(d, splitOwn.Get() && accountedit.SplitAvailable(len(app.Members())), ev.VarName.Get(), memberIDsOf(app.Members())),
			accountCheck{
				Accounts: app.Accounts(), Defs: app.CustomFieldDefsFor("account"), Custom: d.Custom,
				IDs: inputIDs, VarInputID: accountVarInputID,
			})
	}
	add := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		form.SetFormError("")
		attempted.Set(true)
		acc, problem := checkDraft(form.Get())
		if problem.Any() {
			if problem.NeedsAdvanced {
				advOpen.Set(true)
			}
			moveToProblem(problem.First())
			return
		}
		acc.ID = id.New()
		if err := app.PutAccount(acc); err != nil {
			slog.Error("add account", "account", acc.Name, "error", err)
			form.SetFormError(acctWriteErrorText(err))
			moveToProblem("")
			return
		}
		attempted.Set(false)
		form.Update(clearedAfterAdd)
		ev.Reset()
		splitOwn.Set(false)
		// The add modal lives in AddHost (a sibling of the Accounts screen), so closing it only
		// re-renders AddHost. Bump the shared data revision so the Accounts list, which subscribes
		// through UseDataRevision, shows the new account immediately. The confirmation is a notice,
		// which outlives this form; a live region inside the form would be unmounted with the modal
		// before it was read.
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("accounts.addedAnnounce", acc.Name), false)
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	var accDefs []customfields.Def
	if app != nil {
		accDefs = app.CustomFieldDefsFor("account")
	}

	class := accountedit.ClassOf(domain.AccountType(d.Type), d.AsLiability)
	isOther := domain.AccountType(d.Type) == domain.TypeOther
	isLiab := class == domain.ClassLiability
	// Lock-until is offered at creation for the lockable asset types (see accountedit.IsLockable).
	// The liquid everyday types (checking, debit, cash) can still reach it through Advanced.
	// Liabilities never show it.
	isLockableAsset := !isLiab && accountedit.IsLockable(domain.AccountType(d.Type))

	var problem formProblem
	if attempted.Get() && app != nil {
		_, problem = checkDraft(d)
	}
	message := problem.Message
	if message == "" {
		message = form.FormError()
	}
	moneyStep := accountedit.MoneyStep(d.Currency)

	// Each optional section is built only when it is shown.
	var firstRun, copyStarter, otherToggle, ownership, currencyPicker, useOtherCurrency ui.Node
	var liability, advanced, assetFields, lockUntil ui.Node
	if len(accounts) == 0 {
		// First-run framing: when this is the household's very first account, explain in one
		// friendly line what an account is and that nothing leaves the device. It disappears once any
		// account exists, so it never nags returning users.
		firstRun = P(css.Class("notice", tw.Text12), Attr("data-testid", "account-firstrun-hint"),
			uistate.T("accounts.firstRunHint"))
	}
	if len(accounts) > 0 {
		// The copy-existing starter, offered once accounts exist to copy and placed below the name
		// field.
		copyStarter = Div(css.Class("acct-add-wide"),
			labeledField(uistate.T("accounts.copyExisting"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: copyOptions, Selected: "", TestID: "account-copy-existing",
					OnChange: copyFrom, AriaLabel: uistate.T("accounts.copyExisting"),
				})))
	}
	if isOther {
		// "Other" has no natural class, so the person can mark it a liability (debt) and the
		// net-worth and debt formulas count it.
		otherToggle = Div(css.Class("acct-add-wide"),
			Label(css.Class("acct-liab-toggle", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "acct-add-as-liability"), CheckedIf(d.AsLiability), OnChange(onToggleAsLiab)),
				Div(css.Class("row-main"),
					Span(uistate.T("accounts.countAsLiability")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("accounts.countAsLiabilityHint")))))
	}
	ownership = ui.CreateElement(accountOwnershipFields, accountOwnershipProps{
		Form: form, Members: members, Owner: ownerOptions, OnOwner: onOwnerChange, Split: splitOwn, OnToggleSplit: onToggleSplitOwn,
	})
	if !singleCurrency || revealCurr.Get() {
		currencyPicker = labeledField(uistate.T("accounts.currency"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   currencyOpts,
				Selected:  d.Currency,
				OnChange:  onCurrencyChange,
				AriaLabel: uistate.T("accounts.currency"),
				TestID:    "account-currency-select",
			}))
	}
	// Reveal the currency picker on demand for single-currency households.
	if singleCurrency && !revealCurr.Get() {
		useOtherCurrency = Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "account-use-other-currency"),
			OnClick(onRevealCurr), uistate.T("accounts.useOtherCurrency"))
	}
	fieldProps := accountFieldsProps{Form: form, Prefix: idBase, Problem: problem, ErrID: errID, MoneyStep: moneyStep}
	if isLiab {
		liability = ui.CreateElement(accountLiabilityFields, fieldProps)
	}
	if advOpen.Get() {
		advanced = ui.CreateElement(accountAdvancedFields, accountAdvancedProps{Form: form, EV: ev, Accounts: accounts, Name: d.Name})
	}
	if !isLiab && advOpen.Get() {
		assetFields = ui.CreateElement(accountAssetFields, fieldProps)
	}
	if isLockableAsset || (!isLiab && advOpen.Get()) {
		lockUntil = ui.CreateElement(accountLockUntilField, fieldProps)
	}
	openingID := accountFieldInputID(idBase, accountedit.FieldOpening)

	// The form is built before the check that the app is ready, so its handlers register on every
	// render.
	node := Form(css.Class("form-grid"), Attr("id", "account-add-form"), Attr("data-testid", "account-add-form"), Attr("novalidate", true), OnSubmit(add),
		firstRun,
		// Name leads the form, the main path. The "Copy an existing account…" shortcut is a
		// secondary convenience, so it sits below the name.
		Div(css.Class("acct-add-wide"),
			labeledField(uistate.T("common.name"),
				Input(append([]any{css.Class("field", fieldInvalidName(problem, nameID)), Type("text"), Attr("id", nameID), Attr("data-testid", "account-add-name"),
					Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), OnInput(ev.OnName)},
					append(fieldInvalidAttrs(problem, nameID, errID), uiw.FieldValue(name.Get()))...)...))),
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
		currencyPicker,
		useOtherCurrency,
		labeledField(uistate.T("accounts.openingBalance"),
			Input(append([]any{css.Class("field", fieldInvalidName(problem, openingID)), Type("number"), Attr("id", openingID), Placeholder(uistate.T("accounts.openingBalance")), Step(moneyStep),
				OnInput(amountField.Set), uiw.FieldValue(amountField.Get())}, fieldInvalidAttrs(problem, openingID, errID)...)...)),
		// Explains what the opening balance is, and that for a card or loan the person enters the
		// amount currently owed (it is tracked as a liability).
		P(css.Class(tw.TextFaint, tw.Text12), func() string {
			if isLiab {
				return uistate.T("accounts.openingBalanceHintLiab")
			}
			return uistate.T("accounts.openingBalanceHint")
		}()),
		liability,
		// The institution field is shown for every account type.
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
	)
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return node
}

// accountVarInputID is the id of the variable-name input in the add form's Advanced section.
const accountVarInputID = "account-add-varname"

// accountOwnershipProps configures the owner picker and the split-ownership editor.
type accountOwnershipProps struct {
	Form    ui.Form[accountDraft]
	Members []domain.Member
	Owner   []uiw.SelectOption
	// OnOwner sets the owner. The parent memoizes it, so the picker's props keep their identity.
	OnOwner       func(string)
	Split         ui.State[bool]
	OnToggleSplit ui.Handler
}

// accountOwnershipFields is the owner picker and, with two or more members, the split-ownership
// disclosure. The disclosure is built only when there are members to split between.
func accountOwnershipFields(props accountOwnershipProps) ui.Node {
	// The owner picker only offers "Everyone/Group" until members exist, so with no members it is
	// noise that defaults to a meaningless group. It is hidden (the owner stays GroupOwnerID, which
	// is shared) until at least one member is added.
	var ownerPicker, split ui.Node
	if len(props.Members) > 0 {
		ownerPicker = labeledField(uistate.T("common.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   props.Owner,
				Selected:  props.Form.Get().Owner,
				OnChange:  props.OnOwner,
				AriaLabel: uistate.T("common.owner"),
			}))
	}
	// Split-ownership disclosure: available when there are 2+ members.
	if accountedit.SplitAvailable(len(props.Members)) {
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
// member; while it is closed, the rows are not built at all. While it is open the shares must total
// 100, which the alert shows and the save enforces.
func accountSplitFields(props accountSplitProps) ui.Node {
	open := props.Split.Get()
	// onShareChange is a plain func, not an On* hook, so it is safe to pass as a prop inside MapKeyed.
	// It is memoized so the share rows keep their identity between keystrokes. A figure that is not
	// a whole percentage leaves the member's share as it was; the new map still re-renders the row,
	// which puts the previous figure back.
	onShareChange := ui.UseCallback(func(memberID string, valStr string) {
		updateDraft(props.Form, func(d accountDraft) accountDraft {
			d.OwnerShares = accountedit.WithShare(d.OwnerShares, memberID, valStr)
			return d
		})
	})
	var editor ui.Node
	if open {
		shares := props.Form.Get().OwnerShares
		memberIDs := memberIDsOf(props.Members)
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
			If(!accountedit.SharesValid(shares, memberIDs),
				P(css.Class("err"), Attr("data-testid", "account-share-sum-error"),
					uistate.T("account.shareSumError", accountedit.ShareTotal(shares, memberIDs)))),
		)
	}
	return Div(
		Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "account-split-toggle"),
			Attr("aria-expanded", ariaBool(open)),
			OnClick(props.OnToggleSplit),
			IfElse(open,
				Text(uistate.T("account.splitOwnership")+" ▴"),
				Text(uistate.T("account.splitOwnership")+" ▾"))),
		editor,
	)
}

// accountFieldsProps configures the debt-only fields, the asset-only figures and the lock-until
// date: the draft, the prefix of the inputs' ids, what is wrong after a refused save, and the step
// of the money boxes.
type accountFieldsProps struct {
	Form      ui.Form[accountDraft]
	Prefix    string
	Problem   formProblem
	ErrID     string
	MoneyStep string
}

// numberInput is a number box for one field of the form: its id, the danger marking when the save
// refused it, and the draft text bound both ways. The extra options are the box's own (min, max,
// step, placeholder, title). It registers the box's handler hook, so a section that builds it is a
// component of its own.
func (p accountFieldsProps) numberInput(field accountedit.Field, text draftText, opts ...any) ui.Node {
	id := accountFieldInputID(p.Prefix, field)
	args := append([]any{css.Class("field", fieldInvalidName(p.Problem, id)), Type("number"), Attr("id", id)}, opts...)
	args = append(args, fieldInvalidAttrs(p.Problem, id, p.ErrID)...)
	return Input(append(args, OnInput(text.Set), uiw.FieldValue(text.Get()))...)
}

// accountLiabilityFields is the credit limit, APR, minimum payment, due day and lender of a debt.
func accountLiabilityFields(props accountFieldsProps) ui.Node {
	field := func(sel func(*accountDraft) *string) draftText { return draftTextOf(props.Form, sel) }
	limit := field(func(d *accountDraft) *string { return &d.CreditLimit })
	apr := field(func(d *accountDraft) *string { return &d.APR })
	minPay := field(func(d *accountDraft) *string { return &d.MinPayment })
	due := field(func(d *accountDraft) *string { return &d.DueDay })
	lender := field(func(d *accountDraft) *string { return &d.Lender })
	return Fragment(
		labeledField(uistate.T("accounts.creditLimit"),
			props.numberInput(accountedit.FieldCreditLimit, limit, Attr("min", "0"), Placeholder(uistate.T("accounts.creditLimit")), Step(props.MoneyStep))),
		labeledField(uistate.T("accounts.apr"),
			props.numberInput(accountedit.FieldAPR, apr, Attr("min", "0"), Placeholder(uistate.T("accounts.apr")), Step(rateStep))),
		labeledField(uistate.T("accounts.minPayment"),
			props.numberInput(accountedit.FieldMinPayment, minPay, Attr("min", "0"), Placeholder(uistate.T("accounts.minPayment")), Step(props.MoneyStep))),
		labeledField(uistate.T("accounts.dueDay"),
			props.numberInput(accountedit.FieldDueDay, due, Attr("min", "1"), Attr("max", "28"), Step("1"), Placeholder(uistate.T("accounts.dueDay")))),
		labeledField(uistate.T("accounts.lender"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("accounts.lender")), OnInput(lender.Set), uiw.FieldValue(lender.Get()))),
	)
}

// accountAssetFields is the expected return, liquidity and stability of an asset, shown in Advanced.
func accountAssetFields(props accountFieldsProps) ui.Node {
	ret := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.ExpectedReturn })
	liq := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.Liquidity })
	stab := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.Stability })
	return Fragment(
		labeledField(uistate.T("accounts.expReturn"),
			props.numberInput(accountedit.FieldExpectedRet, ret, Attr("title", uistate.T("accounts.expReturnTitle")), Placeholder(uistate.T("accounts.expReturn")), Step(rateStep))),
		labeledField(uistate.T("accounts.liquidity"),
			props.numberInput(accountedit.FieldLiquidity, liq, Attr("min", "0"), Attr("max", "100"), Step("1"), Attr("title", uistate.T("accounts.liquidityTitle")), Placeholder(uistate.T("accounts.liquidity")))),
		labeledField(uistate.T("accounts.stability"),
			props.numberInput(accountedit.FieldStability, stab, Attr("min", "0"), Attr("max", "100"), Step("1"), Attr("title", uistate.T("accounts.stabilityTitle")), Placeholder(uistate.T("accounts.stability")))),
	)
}

// accountLockUntilField is the lock-until date. It is a component of its own because its input
// registers a handler hook and the field comes and goes with the account type and the Advanced
// section; built inline, it would change the form's hook count each time it appeared.
func accountLockUntilField(props accountFieldsProps) ui.Node {
	lock := draftTextOf(props.Form, func(d *accountDraft) *string { return &d.LockUntil })
	id := accountFieldInputID(props.Prefix, accountedit.FieldLockUntil)
	args := []any{css.Class("field", fieldInvalidName(props.Problem, id)), Type("date"), Attr("id", id), Attr("aria-label", uistate.T("accounts.lockUntil")), Title(uistate.T("accounts.lockUntil"))}
	args = append(args, fieldInvalidAttrs(props.Problem, id, props.ErrID)...)
	return labeledField(uistate.T("accounts.lockUntil"), Input(append(args, OnInput(lock.Set), uiw.FieldValue(lock.Get()))...))
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
			entityVarField(accountVarKind, accountVarEntities(props.Accounts), "", accountVarInputID, "account-add-varname-warn", props.EV.VarName.Get(), props.Name, props.EV.OnVarName)))
}

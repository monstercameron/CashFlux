// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/smarttext"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// QuickAddHost mounts at the shell root and renders a quick "add a transaction"
// flip panel when the quick-add atom is open, so a transaction can be logged
// from anywhere without leaving the current screen. It renders nothing when
// closed. All hooks run unconditionally (before the open/closed guard) so the
// hook order stays stable across opens; the result is reported via the toast.
func QuickAddHost() uic.Node {
	open := uistate.UseQuickAdd()
	notice := uistate.UseNotice()
	dataRev := uistate.UseDataRevision()
	app := appstate.Default

	acctID := uic.UseState("")
	kind := uic.UseState("Expense")
	amount := uic.UseState("")
	desc := uic.UseState("")
	catID := uic.UseState("")
	date := uic.UseState("")
	reviewed := uic.UseState(false) // L43: mark a confident entry as already reviewed

	onReviewed := uic.UseEvent(func(e uic.Event) { reviewed.Set(e.IsChecked()) })
	onAcct := uic.UseEvent(func(e uic.Event) { acctID.Set(e.GetValue()) })
	onAmount := uic.UseEvent(func(v string) { amount.Set(v) })
	onDesc := uic.UseEvent(func(v string) { desc.Set(v) })
	onCat := uic.UseEvent(func(e uic.Event) { catID.Set(e.GetValue()) })
	onDate := uic.UseEvent(func(v string) { date.Set(v) })

	if !open.Get() || app == nil {
		return Fragment()
	}

	accounts := app.Accounts()
	// Active member drives the per-member default-account preselect (§1.19); read at
	// a stable hook position so the conditional fallback below never reorders hooks.
	activeMember := uistate.UseActiveMember().Get()
	today := dateutil.FormatDate(time.Now())
	// Effective values: fall back to the first account and today when the user
	// hasn't touched those fields, so the form works without a pre-render Set.
	effAcct := acctID.Get()
	if effAcct == "" {
		// Prefer the active member's per-member default account (§1.19) when they've
		// set one and it still exists, otherwise the first account.
		if am := activeMember; am != "" {
			for _, mb := range app.Members() {
				if mb.ID == am && mb.Prefs.DefaultAccountID != "" {
					if _, ok := accountByID(accounts, mb.Prefs.DefaultAccountID); ok {
						effAcct = mb.Prefs.DefaultAccountID
					}
					break
				}
			}
		}
		if effAcct == "" {
			// Default to the first non-investment asset (a real spending account), not
			// e.g. a 401(k)/Brokerage, which shouldn't seed a transaction (L78-T3).
			for _, a := range accounts {
				if a.Class == domain.ClassAsset && a.Type != domain.TypeInvestment && !a.Archived {
					effAcct = a.ID
					break
				}
			}
			if effAcct == "" && len(accounts) > 0 {
				effAcct = accounts[0].ID
			}
		}
	}
	effDate := date.Get()
	if effDate == "" {
		effDate = today
	}

	post := func(text string, isErr bool) { notice.Set(notice.Get().With(text, isErr)) }
	reset := func() {
		acctID.Set("")
		kind.Set("Expense")
		amount.Set("")
		desc.Set("")
		catID.Set("")
		date.Set("")
		reviewed.Set(false)
	}
	closePanel := func() {
		reset()
		open.Set(false)
	}
	// saveCore validates + persists the transaction, returning true on success. It is
	// shared by the panel's Save (which then closes) and "Save & add another" (C40),
	// which keeps the panel open and resets the form for the next entry.
	saveCore := func() bool {
		acc, ok := accountByID(accounts, effAcct)
		if !ok {
			post(uistate.T("quickAdd.needAccount"), true)
			return false
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amount.Get()), currency.Decimals(acc.Currency))
		if err != nil || amt == 0 {
			post(uistate.T("quickAdd.needAmount"), true)
			return false
		}
		if strings.TrimSpace(desc.Get()) == "" {
			// Plain-English, not the generic validator's "desc is required" (L78-T1c).
			post(uistate.T("quickAdd.needDesc"), true)
			return false
		}
		if kind.Get() == "Expense" {
			amt = -amt
		}
		d, derr := dateutil.ParseDate(strings.TrimSpace(effDate))
		if derr != nil {
			d = time.Now()
		}
		member := ""
		if acc.Scope == domain.ScopeIndividual {
			member = acc.OwnerID
		}
		t := domain.Transaction{
			ID: id.New(), AccountID: acc.ID, Date: d, Desc: strings.TrimSpace(desc.Get()),
			CategoryID: catID.Get(), Amount: money.New(amt, acc.Currency), MemberID: member,
			Reviewed: reviewed.Get(),
		}
		// Apply auto-categorization rules on save (it won't overwrite a manual
		// category). Quick-add is now the sole manual-add path after the inline
		// transaction form was removed (C73/C79), so without this a rule-matching
		// payee would no longer be auto-filed the way the inline form did (L15).
		t = app.AutoCategorizeTransaction(t)
		if err := app.PutTransaction(t); err != nil {
			post(err.Error(), true)
			return false
		}
		// PutTransaction now fires the "transaction added" workflow trigger itself
		// (for every add path), so no explicit RunTriggered call here.
		dataRev.Update(func(v int) int { return v + 1 })
		post(uistate.T("quickAdd.added"), false)
		return true
	}
	save := func() { saveCore() } // panel's Save: persist then the FlipPanel closes
	// C40: "Save & add another" — persist, then keep the panel open and clear the
	// inputs for rapid back-to-back entry (the amount field is re-focused). The
	// account/kind are also reset by reset(); a power user can re-pick once.
	saveAndAnother := func() {
		if saveCore() {
			reset()
		}
	}

	acctOpts := make([]uic.Node, 0, len(accounts))
	for _, a := range accounts {
		// C45: append a type cue ("Everyday · Checking") so two similarly-named
		// accounts (e.g. business vs personal checking) are distinguishable in the
		// dropdown instead of being truncated to identical names.
		label := a.Name
		if cue := quickAddTypeCue(a.Type); cue != "" {
			label = a.Name + " · " + cue
		}
		acctOpts = append(acctOpts, Option(Value(a.ID), SelectedIf(effAcct == a.ID), label))
	}
	catOpts := []uic.Node{Option(Value(""), SelectedIf(catID.Get() == ""), uistate.T("quickAdd.noCategory"))}
	for _, c := range app.Categories() {
		catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
	}

	// "Mark as reviewed" checkbox: a confident entry skips the auto review-tag (L43).
	reviewedArgs := []any{Type("checkbox"), OnChange(onReviewed)}
	if reviewed.Get() {
		reviewedArgs = append(reviewedArgs, Attr("checked", ""))
	}

	// SMART field assists (Wave 3 / Free):
	//  (a) Clean-merchant: suggests a normalised merchant name when the raw
	//      description looks like a bank POS string (prefix, store numbers, etc.).
	//  (b) Auto-category: when a user rule matches the description, shows the
	//      rule's category so it can be applied in one click — giving visibility
	//      into the auto-categorization that already runs on save.
	qaSmartSettings := uistate.LoadSmartSettings()

	// (a) Clean-merchant assist.
	rawDesc := desc.Get()
	cleanedDesc := smarttext.CleanMerchant(rawDesc)
	var descSuggestion string
	if cleanedDesc != strings.TrimSpace(rawDesc) && cleanedDesc != "" {
		descSuggestion = cleanedDesc
	}
	descAssist := screens.SmartFieldAssist(qaSmartSettings, "qa-desc", descSuggestion, func() {
		desc.Set(cleanedDesc)
	})

	// (b) Auto-category assist: resolve the matched category ID to a human name
	//     so the chip reads "Use 'Groceries'" rather than an opaque ID.
	suggestedCatID := rules.Category(app.Rules(), "", strings.TrimSpace(rawDesc))
	var catSuggestion string
	if suggestedCatID != "" && catID.Get() == "" {
		for _, c := range app.Categories() {
			if c.ID == suggestedCatID {
				catSuggestion = c.Name
				break
			}
		}
	}
	catAssist := screens.SmartFieldAssist(qaSmartSettings, "qa-cat", catSuggestion, func() {
		catID.Set(suggestedCatID)
	})

	// Form validity (L78-T1): Save is disabled until Description and a non-zero
	// Amount are present, so an invalid submit can't close the panel or lose input.
	// Computed before the body so "Save & add another" (C40) shares the same gate.
	formValid := strings.TrimSpace(desc.Get()) != ""
	if acc, ok := accountByID(accounts, effAcct); ok {
		if v, perr := money.ParseMinor(strings.TrimSpace(amount.Get()), currency.Decimals(acc.Currency)); perr != nil || v == 0 {
			formValid = false
		}
	} else {
		formValid = false
	}

	// GM2-3: 5 of 6 QuickAdd inputs were placeholder/title-only (no visible label).
	// Wrap each in ui.FormField so they render a visible caption above the control,
	// matching the .labeled-field pattern used by all entity add modals.
	body := Div(css.Class("form-grid"),
		ui.FormField(uistate.T("quickAdd.account"),
			Select(css.Class("field"), Attr("data-testid", "txn-add-account"), Attr("aria-label", uistate.T("quickAdd.account")), OnChange(onAcct), acctOpts)),
		ui.Segmented(ui.SegmentedProps{
			Label: uistate.T("quickAdd.kind"),
			Options: []ui.SegOption{
				{Value: "Expense", Label: uistate.T("quickAdd.expense")},
				{Value: "Income", Label: uistate.T("quickAdd.income")},
			},
			Selected: kind.Get(),
			OnSelect: func(v string) { kind.Set(v) },
		}),
		ui.FormField(uistate.T("quickAdd.amount"),
			Input(css.Class("field"), Type("number"), Attr("data-testid", "txn-add-amount"), Attr("autofocus", ""), Attr("aria-label", uistate.T("quickAdd.amount")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.amount")), Value(amount.Get()), Step("0.01"), OnInput(onAmount))),
		ui.FormField(uistate.T("quickAdd.description"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-desc"), Attr("aria-label", uistate.T("quickAdd.description")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.descPlaceholder")), Value(desc.Get()), OnInput(onDesc))),
		descAssist,
		ui.FormField(uistate.T("quickAdd.category"),
			Select(css.Class("field"), Attr("data-testid", "txn-add-category"), Attr("aria-label", uistate.T("quickAdd.category")), OnChange(onCat), catOpts)),
		catAssist,
		ui.FormField(uistate.T("quickAdd.date"),
			Input(css.Class("field"), Type("date"), Attr("data-testid", "txn-add-date"), Attr("aria-label", uistate.T("quickAdd.date")), Value(effDate), OnInput(onDate))),
		Label(css.Class("quickadd-reviewed"), Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.4rem", "font-size": "0.8rem"}),
			Input(reviewedArgs...),
			uistate.T("quickAdd.reviewed")),
		// C40: keep-open rapid entry. Disabled with the same validity gate as Save so
		// it can't persist an invalid row. Lives in the body (the panel's footer Save
		// closes the panel; this one deliberately keeps it open).
		quickAddAnotherBtn(formValid, saveAndAnother),
	)

	return ui.FlipPanel(ui.FlipPanelProps{
		Title: uistate.T("quickAdd.title"),
		Back:  body,
		// Shorter than the default so the compact add-a-transaction form doesn't
		// float in a tall, mostly-empty panel (C13). The body scrolls if it ever
		// overflows.
		Height:       "420px",
		OnSave:       save,
		SaveDisabled: !formValid,
		OnClose:      closePanel,
	})
}

// accountByID finds an account by ID in a slice.
func accountByID(accounts []domain.Account, id string) (domain.Account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Account{}, false
}

// quickAddTypeCue returns a short human label for an account type to disambiguate
// the quick-add account dropdown (C45), e.g. "checking" → "Checking". Empty for an
// unset type (no cue rather than a stray separator).
func quickAddTypeCue(t domain.AccountType) string {
	s := strings.ReplaceAll(string(t), "_", " ")
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// quickAddAnotherBtn renders the "Save & add another" button (C40), disabled with
// the same validity gate as the panel's Save so it can never persist an invalid row.
// Its own helper so the conditional disabled attr stays a stable render position.
func quickAddAnotherBtn(valid bool, onClick func()) uic.Node {
	args := []any{
		css.Class("btn"), Type("button"), Attr("data-testid", "txn-add-another"),
		OnClick(onClick), uistate.T("quickAdd.saveAndAnother"),
	}
	if !valid {
		args = append(args, Attr("disabled", ""))
	}
	return Button(args...)
}

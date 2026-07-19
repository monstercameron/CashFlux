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
	"github.com/monstercameron/CashFlux/internal/payees"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/smarttext"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// QuickAddHost mounts at the shell root and renders a quick "add a transaction"
// flip panel when the quick-add atom is open, so a transaction can be logged
// from anywhere without leaving the current screen. It renders nothing when
// closed. All hooks run unconditionally (before the open/closed guard) so the
// hook order stays stable across opens; the result is reported via the toast.
func QuickAddHost() uic.Node {
	open := uistate.UseQuickAdd()
	transferOpen := uistate.UseAcctTransferOpen()
	notice := uistate.UseNotice()
	dataRev := uistate.UseDataRevision()
	app := appstate.Default

	acctID := uic.UseState("")
	kind := uic.UseState("Expense")
	amount := uic.UseState("")
	payee := uic.UseState("")
	desc := uic.UseState("")
	catID := uic.UseState("")
	date := uic.UseState("")
	reviewed := uic.UseState(false) // L43: mark a confident entry as already reviewed
	// QA M7: the metadata that used to require save-then-reopen (tags, member,
	// note, cleared, report exclusion) is available at creation, folded behind a
	// collapsed "More details" disclosure so the fast path stays five fields.
	tagsS := uic.UseState("")
	memberS := uic.UseState("")
	noteS := uic.UseState("")
	clearedS := uic.UseState(false)
	excludeS := uic.UseState(false)

	onReviewed := uic.UseEvent(func(e uic.Event) { reviewed.Set(e.IsChecked()) })
	onTags := uic.UseEvent(func(v string) { tagsS.Set(v) })
	onMember := uic.UseEvent(func(e uic.Event) { memberS.Set(e.GetValue()) })
	onNote := uic.UseEvent(func(v string) { noteS.Set(v) })
	onCleared := uic.UseEvent(func(e uic.Event) { clearedS.Set(e.IsChecked()) })
	onExclude := uic.UseEvent(func(e uic.Event) { excludeS.Set(e.IsChecked()) })
	onAcct := uic.UseEvent(func(e uic.Event) { acctID.Set(e.GetValue()) })
	onAmount := uic.UseEvent(func(v string) { amount.Set(v) })
	// TX16: on blur/Enter, evaluate an arithmetic entry ("45.99*3", "(12+8)*2")
	// and replace it with the result; a plain number or parse failure is left as-is.
	onAmountBlur := uic.UseEvent(func(e uic.Event) {
		if s, ok := screens.EvalAmountField(e.GetValue()); ok {
			amount.Set(s)
		}
	})
	onPayee := uic.UseEvent(func(v string) { payee.Set(v) })
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
			// Default to the MOST-USED everyday spending account (by transaction count),
			// not merely the first in store order — so the modal opens on the account the
			// household actually records against (e.g. Joint Checking) instead of an
			// incidental first-listed one (e.g. a rarely-touched business checking).
			// Investment/retirement/crypto accounts are never everyday-spend accounts, so
			// they're excluded (C73/L78-T3).
			txnCount := map[string]int{}
			for _, t := range app.Transactions() {
				txnCount[t.AccountID]++
			}
			bestN := -1
			for _, a := range accounts {
				if a.Class != domain.ClassAsset || a.Archived ||
					a.Type == domain.TypeInvestment ||
					a.Type == domain.TypeRetirement ||
					a.Type == domain.TypeCrypto {
					continue
				}
				if n := txnCount[a.ID]; n > bestN {
					effAcct, bestN = a.ID, n
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
		payee.Set("")
		desc.Set("")
		catID.Set("")
		date.Set("")
		reviewed.Set(false)
		tagsS.Set("")
		memberS.Set("")
		noteS.Set("")
		clearedS.Set(false)
		excludeS.Set(false)
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
		// QA M7: an explicit member pick from "More details" overrides the
		// account-derived default.
		if m := memberS.Get(); m != "" {
			member = m
		}
		t := domain.Transaction{
			ID: id.New(), AccountID: acc.ID, Date: d, Payee: strings.TrimSpace(payee.Get()),
			Desc: strings.TrimSpace(desc.Get()), CategoryID: catID.Get(),
			Amount: money.New(amt, acc.Currency), MemberID: member, Reviewed: reviewed.Get(),
			Source:             domain.TxnSourceManual,
			Tags:               textutil.CommaFields(tagsS.Get()),
			Note:               strings.TrimSpace(noteS.Get()),
			Cleared:            clearedS.Get(),
			ExcludeFromReports: excludeS.Get(),
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
		// C33: record payee→category in the self-learning tally whenever a category
		// was explicitly provided by the user. The payee field (or description as
		// fallback) is the key; the tally is persisted across reloads via the
		// preserved KV store so corrections accumulate session-to-session.
		if t.CategoryID != "" {
			learnPayee := strings.TrimSpace(t.Payee)
			if learnPayee == "" {
				learnPayee = strings.TrimSpace(t.Desc)
			}
			uistate.IncrementLearnTally(learnPayee, t.CategoryID)
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

	// TXT: transaction quick-templates ("favourites"). onPick pre-fills the add
	// form's existing state atoms from a chosen template (account, kind, amount,
	// payee, description, category) so a frequent manual entry lands in one click.
	// The amount atom holds a plain positive string, so we format the template's
	// magnitude with its currency's decimals and carry the sign via the kind toggle.
	onPick := func(t domain.TxnTemplate) {
		if t.AccountID != "" {
			acctID.Set(t.AccountID)
		}
		if t.IsExpense() {
			kind.Set("Expense")
		} else {
			kind.Set("Income")
		}
		if t.AmountMinor != 0 {
			mag := t.AmountMinor
			if mag < 0 {
				mag = -mag
			}
			amount.Set(money.FormatMinor(mag, currency.Decimals(t.Currency)))
		}
		payee.Set(t.Payee)
		if t.Note != "" {
			desc.Set(t.Note)
		}
		catID.Set(t.CategoryID)
	}

	// Build the draft field set the "Save as template" action snapshots from the
	// live form. AmountMinor is the parsed magnitude against the effective account's
	// currency; the sign is carried by Direction (from the Expense/Income toggle).
	var draftCurrency string
	var draftAmountMinor int64
	if acc, ok := accountByID(accounts, effAcct); ok {
		draftCurrency = acc.Currency
		if v, perr := money.ParseMinor(strings.TrimSpace(amount.Get()), currency.Decimals(acc.Currency)); perr == nil {
			draftAmountMinor = v
		}
	}
	draftDir := domain.DirectionExpense
	if kind.Get() == "Income" {
		draftDir = domain.DirectionIncome
	}
	saveTemplateBtn := screens.SaveAsTemplateButton(screens.TxnTemplateDraft{
		Payee:       strings.TrimSpace(payee.Get()),
		CategoryID:  catID.Get(),
		AccountID:   effAcct,
		AmountMinor: draftAmountMinor,
		Currency:    draftCurrency,
		Direction:   draftDir,
		Note:        strings.TrimSpace(desc.Get()),
	}, nil)

	acctOpts := make([]uic.Node, 0, len(accounts))
	for _, a := range accounts {
		// C41: don't offer archived accounts as a destination for a NEW transaction —
		// they're retired. Keep one only if it's somehow the current selection, so we
		// never silently drop the effective account out from under the user.
		if a.Archived && a.ID != effAcct {
			continue
		}
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

	// C47: "Mark as reviewed" checkbox: a confident entry skips the auto review-tag (L43).
	// The original label key ("quickAdd.reviewed") was too terse — users couldn't tell
	// what "reviewed" meant or what leaving it unchecked would do. Replaced with a clear
	// primary label + a muted helper caption so the field is self-explanatory.
	reviewedArgs := []any{Type("checkbox"), OnChange(onReviewed), Checked(reviewed.Get())}

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

	// (c) C34: self-learning category assist — when no rule matched and the user
	//     hasn't picked a category yet, consult the persisted learntally. If the
	//     top payee→category correction count has crossed the user-tunable threshold,
	//     surface a chip. We key on payee first (more specific), falling back to the
	//     description, to match what saveCore records.
	learnThreshold := uistate.LoadLearnThreshold()
	learnTally := uistate.LoadLearnTally()
	learnPayeeKey := strings.TrimSpace(payee.Get())
	if learnPayeeKey == "" {
		learnPayeeKey = strings.TrimSpace(rawDesc)
	}
	var learnedCatAssist uic.Node = Fragment()
	if suggestedCatID == "" && catID.Get() == "" {
		if learnedCatID, ok := learnTally.ShouldSuggest(learnPayeeKey, learnThreshold); ok {
			var learnedCatName string
			for _, c := range app.Categories() {
				if c.ID == learnedCatID {
					learnedCatName = c.Name
					break
				}
			}
			if learnedCatName != "" {
				// Reuse SmartFieldAssist chip pattern: chip text will read
				// `✦ Use "Groceries"` which is the correct affordance.
				learnedCatAssist = screens.SmartFieldAssist(qaSmartSettings, "qa-learned-cat", learnedCatName, func() {
					catID.Set(learnedCatID)
				})
			}
		}
	}

	// TX17: entry-time budget impact. When a category and a non-zero amount are both
	// present, show a live caption ("Leaves $142 in Dining this month · safe to spend
	// $890"). Computed on render (the engine surface is memoized); the helper returns
	// an empty fragment until both inputs exist.
	var impactMinor int64
	if acc, ok := accountByID(accounts, effAcct); ok {
		if v, perr := money.ParseMinor(strings.TrimSpace(amount.Get()), currency.Decimals(acc.Currency)); perr == nil {
			impactMinor = v
		}
	}
	budgetImpact := screens.QuickAddBudgetImpact(app, catID.Get(), impactMinor)

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

	// C39/C46: build a datalist of recent payees (distinct, newest-first, ≤50) so the
	// Payee input offers autocomplete suggestions without constraining the user to them.
	recentPayees := payees.RecentPayees(app.Transactions(), 50)
	payeeDatalistArgs := make([]any, 0, len(recentPayees)+1)
	payeeDatalistArgs = append(payeeDatalistArgs, Attr("id", "qa-payees"))
	for _, p := range recentPayees {
		payeeDatalistArgs = append(payeeDatalistArgs, Option(Value(p)))
	}
	payeeDatalist := Datalist(payeeDatalistArgs...)

	// GM2-3: 5 of 6 QuickAdd inputs were placeholder/title-only (no visible label).
	// Wrap each in ui.FormField so they render a visible caption above the control,
	// matching the .labeled-field pattern used by all entity add modals.
	body := Div(css.Class("form-grid", "qa-grid"),
		payeeDatalist,
		// TXT: the templates zone — chips one-click pre-fill the form below, and "Save
		// as template" (disabled until an amount is present) snapshots the current form.
		// Kept together at the top so the feature it teaches is visible without scrolling.
		ui.FormField(uistate.T("quickAdd.account"),
			Select(css.Class("field"), Attr("data-testid", "txn-add-account"), Attr("aria-label", uistate.T("quickAdd.account")), OnChange(onAcct), acctOpts)),
		// .qa-pair: opt back into a single grid cell (pairs with Account on one row)
		// instead of the .qa-grid default full-width span for non-labeled-fields.
		Div(css.Class("qa-pair"), ui.Segmented(ui.SegmentedProps{
			Label: uistate.T("quickAdd.kind"),
			Options: []ui.SegOption{
				{Value: "Expense", Label: uistate.T("quickAdd.expense")},
				{Value: "Income", Label: uistate.T("quickAdd.income")},
				// Not a third form mode: picking Transfer hands off to the real
				// transfer workflow (two legs, FX, previews). It lives here because
				// the ledger's add flow is where users look for it (2026-07-18
				// assessment: "Transfer exists only on Accounts").
				{Value: "Transfer", Label: uistate.T("quickAdd.transfer")},
			},
			Selected: kind.Get(),
			OnSelect: func(v string) {
				if v == "Transfer" {
					closePanel()
					transferOpen.Set(true)
					return
				}
				kind.Set(v)
			},
		})),
		ui.FormField(uistate.T("quickAdd.amount"),
			Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"), Attr("data-testid", "txn-add-amount"), Attr("autofocus", ""), Attr("aria-label", uistate.T("quickAdd.amount")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.amount")), Value(amount.Get()), OnInput(onAmount), OnBlur(onAmountBlur))),
		ui.FormField(uistate.T("quickAdd.payee"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-payee"), Attr("list", "qa-payees"), Attr("aria-label", uistate.T("quickAdd.payee")), Placeholder(uistate.T("quickAdd.payeePlaceholder")), Value(payee.Get()), OnInput(onPayee))),
		ui.FormField(uistate.T("quickAdd.description"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-desc"), Attr("aria-label", uistate.T("quickAdd.description")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.descPlaceholder")), Value(desc.Get()), OnInput(onDesc))),
		descAssist,
		ui.FormField(uistate.T("quickAdd.category"),
			Select(css.Class("field"), Attr("data-testid", "txn-add-category"), Attr("aria-label", uistate.T("quickAdd.category")), OnChange(onCat), catOpts)),
		catAssist,
		learnedCatAssist,
		budgetImpact,
		ui.FormField(uistate.T("quickAdd.date"),
			Input(css.Class("field"), Type("date"), Attr("data-testid", "txn-add-date"), Attr("aria-label", uistate.T("quickAdd.date")), Value(effDate), OnInput(onDate))),
		// C47: wrap the checkbox in a block container so the helper caption sits
		// below the label text, not inline with it. The outer Div is not a label
		// so screen-readers read the Input's associated Label normally.
		Div(css.Class("qa-pair"), Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.15rem"}),
			Label(css.Class("quickadd-reviewed"), Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.4rem", "font-size": "0.8rem"}),
				Input(reviewedArgs...),
				uistate.T("quickAdd.reviewedClear")),
			Span(Style(map[string]string{"font-size": "0.72rem", "color": "var(--text-dim)", "padding-left": "1.4rem"}),
				uistate.T("quickAdd.reviewedHelp"))),
		// C42 TRIAGE — NO CODE BUG: native <input type="date"> has internal sub-field
		// segments (mm / dd / yyyy) that capture Arrow/Tab key presses to cycle between
		// them. On some browsers (Chrome/Edge) pressing Tab inside a date segment moves
		// to the next segment rather than the next focusable element. This is pure
		// browser-native behavior; there is no JavaScript key-trap in this file. The
		// field order is: date → reviewed checkbox → "Save & add another" button →
		// FlipPanel footer Save/Close, so once the user exits the date input (Tab past
		// the last segment, or Shift+Tab back) focus does proceed to those controls
		// correctly. No code change warranted — the tab order is already correct.
		// QA M7: the metadata that used to require save-then-find-then-edit — tags,
		// member, note, cleared, report exclusion — folded behind a collapsed
		// native disclosure so the everyday five-field path stays fast.
		Details(css.Class("csv-help"), Attr("data-testid", "txn-add-more"),
			Summary(uistate.T("quickAdd.moreDetails")),
			Div(css.Class("form-grid"), Style(map[string]string{"margin-top": "0.5rem"}),
				ui.FormField(uistate.T("quickAdd.tags"),
					Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-tags"), Attr("aria-label", uistate.T("quickAdd.tags")),
						Placeholder(uistate.T("quickAdd.tagsPh")), Value(tagsS.Get()), OnInput(onTags))),
				If(len(app.Members()) > 0, ui.FormField(uistate.T("quickAdd.member"),
					Select(css.Class("field"), Attr("data-testid", "txn-add-member"), Attr("aria-label", uistate.T("quickAdd.member")), OnChange(onMember),
						func() []uic.Node {
							opts := []uic.Node{Option(Value(""), SelectedIf(memberS.Get() == ""), uistate.T("quickAdd.memberAuto"))}
							for _, m := range app.Members() {
								opts = append(opts, Option(Value(m.ID), SelectedIf(memberS.Get() == m.ID), m.Name))
							}
							return opts
						}()))),
				ui.FormField(uistate.T("quickAdd.note"),
					Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-note"), Attr("aria-label", uistate.T("quickAdd.note")),
						Placeholder(uistate.T("quickAdd.notePh")), Value(noteS.Get()), OnInput(onNote))),
				Label(Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.4rem", "font-size": "0.8rem"}),
					Input(Type("checkbox"), Attr("data-testid", "txn-add-cleared"), OnChange(onCleared), Checked(clearedS.Get())),
					uistate.T("quickAdd.cleared")),
				Label(Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.4rem", "font-size": "0.8rem"}),
					Input(Type("checkbox"), Attr("data-testid", "txn-add-exclude"), OnChange(onExclude), Checked(excludeS.Get())),
					uistate.T("quickAdd.exclude")),
			),
		),
		// C40: keep-open rapid entry. Disabled with the same validity gate as Save so
		// it can't persist an invalid row. Lives in the body (the panel's footer Save
		// closes the panel; this one deliberately keeps it open).
		quickAddAnotherBtn(formValid, saveAndAnother),
		// TXT: the templates zone — one-click chips pre-fill the form, and "Save as
		// template" snapshots it. Demoted to the BOTTOM (from the prime top slot) so a
		// "No templates yet" placeholder no longer occupies the most valuable real estate;
		// the Account/Amount fields the user needs first now lead the form.
		Div(css.Class("txt-zone"),
			screens.TxnTemplatePicker(onPick),
			saveTemplateBtn,
		),
	)

	return ui.FlipPanel(ui.FlipPanelProps{
		Title: uistate.T("quickAdd.title"),
		Back:  body,
		// Large standard size (Cam 2026-07-19): the auto-fit .form-grid flows the
		// fields into multiple columns at this width and the full body — assists,
		// More-details disclosure, templates zone — fits without scrolling.
		// (Supersedes C13's compact 420px, which predated the templates zone and
		// assist rows that made the body scroll.)
		Width:        ui.FlipLargeW,
		Height:       ui.FlipLargeH,
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

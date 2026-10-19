// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/amountmath"
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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// QuickAddHost mounts at the shell root and renders a quick "add a transaction"
// modal when the quick-add atom is open, so a transaction can be logged
// from anywhere without leaving the current screen. It renders nothing when
// closed. The controlled Overlay stays mounted through close so GWC can return
// focus to the opener. Form hooks remain unconditional across opens.
func QuickAddHost() uic.Node {
	return uic.CreateElement(quickAddHost, appstate.Default)
}

func quickAddHost(app *appstate.App) uic.Node {
	open := uistate.UseQuickAdd()
	if app == nil {
		return Fragment()
	}
	return uic.CreateElement(quickAddForm, quickAddFormProps{App: app, Open: open.Get()})
}

type quickAddFormProps struct {
	App  *appstate.App
	Open bool
}

func quickAddForm(props quickAddFormProps) uic.Node {
	app := props.App
	open := uistate.UseQuickAdd()
	// C573: the direction a menu item promised ("Income…"), consumed as the panel
	// opens. Read unconditionally here so the atom is captured on every render, not
	// only the renders where the panel happens to be showing.
	seedKind := uistate.UseQuickAddKind()
	transferOpen := uistate.UseAcctTransferOpen()
	notice := uistate.UseNotice()
	dataRev := uistate.UseDataRevision()
	_ = dataRev.Get()

	acctID := uic.UseState("")
	kind := uic.UseState("Expense")
	amount := uic.UseState("")
	payee := uic.UseState("")
	desc := uic.UseState("")
	catID := uic.UseState("")
	date := uic.UseState("")
	needsReview := uic.UseState(false)
	saveAttempted := uic.UseState(false)
	focusField := uic.UseState("")
	// QA M7: the metadata that used to require save-then-reopen (tags, member,
	// note, cleared, report exclusion) is available at creation, folded behind a
	// collapsed "More details" disclosure so the fast path stays five fields.
	tagsS := uic.UseState("")
	activeMember := uistate.UseActiveMember().Get()
	ownersS := uic.UseState(quickAddOwnerDefaults(activeMember))
	ownersTouched := uic.UseState(false)
	noteS := uic.UseState("")
	clearedS := uic.UseState(false)
	excludeS := uic.UseState(false)
	categoryPickerOpen := uic.UseState(false)

	onReviewed := uic.UseEvent(func(e uic.Event) { needsReview.Set(e.IsChecked()) })
	onTags := uic.UseEvent(func(v string) { tagsS.Set(v) })
	onNote := uic.UseEvent(func(v string) { noteS.Set(v) })
	onCleared := uic.UseEvent(func(e uic.Event) { clearedS.Set(e.IsChecked()) })
	onExclude := uic.UseEvent(func(e uic.Event) { excludeS.Set(e.IsChecked()) })
	onAcct := func(e uic.Event) { acctID.Set(e.GetValue()) }
	onAmount := func(v string) {
		amount.Set(v)
		// C624: a typed minus means money OUT (C546), and the toggle must say so
		// while the user can still see it. Resolving the sign silently at save time
		// let "-1" be entered with Income selected and stored as an expense — the
		// direction/amount mismatch this ticket is about. Reflecting it here is the
		// "show the resulting sign before commit" option the ticket allows, and it
		// is safe in an INPUT handler (setting state mid-save is what loses writes).
		if strings.HasPrefix(strings.TrimSpace(v), "-") && kind.Get() != "Expense" {
			kind.Set("Expense")
		}
	}
	// TX16: on blur/Enter, evaluate an arithmetic entry ("45.99*3", "(12+8)*2")
	// and replace it with the result; a plain number or parse failure is left as-is.
	onAmountBlur := func(e uic.Event) {
		v := e.GetValue()
		if s, ok := screens.EvalAmountField(v); ok {
			v = s
			amount.Set(s)
		}
		// C624: once the user leaves the field, the minus has done its job — the kind
		// toggle carries the direction now. Leaving it in the box asks the reader to
		// resolve "Expense" over "-1", a double negative, and that ambiguity is the
		// whole complaint: nothing on the form said which way it would be stored. The
		// stored value is unchanged (ParseSigned reads the magnitude plus the toggle);
		// only the display stops contradicting itself.
		if t := strings.TrimSpace(v); strings.HasPrefix(t, "-") {
			amount.Set(strings.TrimSpace(strings.TrimPrefix(t, "-")))
		}
	}
	onPayee := func(v string) { payee.Set(v) }
	onDesc := func(v string) { desc.Set(v) }
	onCat := uic.UseEvent(func(e uic.Event) { catID.Set(e.GetValue()) })
	toggleNewCategory := uic.UseEvent(Prevent(func() { categoryPickerOpen.Set(true) }))
	onDate := func(v string) { date.Set(v) }
	uic.UseEffect(func() func() {
		if id := focusField.Get(); id != "" {
			focusQuickAddField(id)
		}
		return nil
	}, focusField.Get())

	accounts := app.Accounts()
	// The single-member view supplies the member-specific default. A broader view
	// has no member default and falls back to the workspace setting.
	memberLens := uistate.UseMemberLens()
	defaultMemberID := ""
	if len(memberLens) == 1 {
		defaultMemberID = memberLens[0]
	}
	today := dateutil.FormatDate(time.Now())
	// account selection remains explicit once the user or a template chooses it;
	// clearing state after Save & add another runs this same resolver again.
	effAcct := resolveQuickAddAccountID(accountselect.ResolveInput{
		Accounts: accounts, Transactions: app.Transactions(), ExplicitID: acctID.Get(),
		ActiveMemberID: defaultMemberID, Members: app.Members(),
		WorkspaceDefaultID: app.DefaultAccountID(),
	})
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
		categoryPickerOpen.Set(false)
		needsReview.Set(false)
		saveAttempted.Set(false)
		focusField.Set("")
		tagsS.Set("")
		ownersS.Set(quickAddOwnerDefaults(activeMember))
		ownersTouched.Set(false)
		noteS.Set("")
		clearedS.Set(false)
		excludeS.Set(false)
	}
	closePanel := func() {
		reset()
		open.Set(false)
	}
	// C573: apply the direction the entry point promised. Done in an effect rather
	// than during render — writing state while rendering re-enters the render — and
	// keyed on both the open flag and the seed, so it runs when the panel opens with
	// a seed waiting and again if a second menu choice arrives while it is open.
	// TakeQuickAddKind clears the seed, so the next plain "+ Add transaction" does
	// not silently inherit the last menu choice.
	uic.UseEffect(func() func() {
		if !open.Get() {
			return nil
		}
		if k := uistate.TakeQuickAddKind(); k != "" {
			kind.Set(k)
		}
		return nil
	}, "quickadd-seed:"+strconv.FormatBool(open.Get())+":"+seedKind.Get())
	// saveCore validates + persists the transaction, returning true on success. It is
	// shared by the panel's Save (which then closes) and "Save & add another" (C40),
	// which keeps the panel open and resets the form for the next entry.
	saveCore := func() bool {
		if field, _ := quickAddInvalidFieldR4F(accounts, effAcct, amount.Get(), kind.Get(), payee.Get(), desc.Get()); field != "" {
			saveAttempted.Set(true)
			focusField.Set(field)
			return false
		}
		acc, ok := accountByID(accounts, effAcct)
		if !ok {
			return false
		}
		// C546: the amount field is a magnitude and the kind toggle carries the
		// sign — but this used to guard on `amt == 0`, so a typed "-50" parsed to
		// −5000, survived the guard, and was then negated by the Expense branch
		// into +50 INCOME. The opposite of what was typed, saved silently.
		// amountmath.ParseSigned owns that reconciliation now: a typed minus is
		// read as intent and wins, and it reports the direction back so the
		// toggle follows instead of displaying a lie.
		want := amountmath.Out
		if kind.Get() == "Income" {
			want = amountmath.In
		}
		amt, resolved, err := amountmath.ParseSigned(amount.Get(), currency.Decimals(acc.Currency), want)
		if err != nil {
			return false
		}
		// The resolved direction is already carried by amt's sign. The kind toggle
		// is deliberately NOT set here: a state setter mid-handler re-renders the
		// panel while this closure is still running and the write can be lost, and
		// on this form the point is moot anyway — a successful save closes the
		// panel and reset() returns the toggle to Expense.
		_ = resolved
		d, derr := dateutil.ParseDate(strings.TrimSpace(effDate))
		if derr != nil {
			d = time.Now()
		}
		t := domain.Transaction{
			ID: id.New(), AccountID: acc.ID, Date: d, Payee: strings.TrimSpace(payee.Get()),
			Desc: quickAddDescriptionR4F(payee.Get(), desc.Get()), CategoryID: catID.Get(),
			Amount: money.New(amt, acc.Currency), MemberIDs: quickAddOwnerSelection(ownersS.Get(), activeMember, acc, ownersTouched.Get()), Reviewed: quickAddReviewedValue(needsReview.Get()),
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
	save := func() {
		if saveCore() {
			closePanel()
		}
	}
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
	// C570: the same qualified, naturally-sorted category list Review, Edit and the
	// split editor use — "Housing > Mortgage", not a bare "Mortgage". Quick-add is
	// where most transactions get their category, so it was the worst place to make
	// two same-named leaves indistinguishable.
	catOpts := []uic.Node{Option(Value(""), SelectedIf(catID.Get() == ""), uistate.T("quickAdd.noCategory"))}
	for _, o := range screens.CategoryPickOptions(app.Categories()) {
		catOpts = append(catOpts, Option(Value(o.Value), SelectedIf(catID.Get() == o.Value), o.Label))
	}

	// C47: "Mark as reviewed" checkbox: a confident entry skips the auto review-tag (L43).
	// The original label key ("quickAdd.reviewed") was too terse — users couldn't tell
	// what "reviewed" meant or what leaving it unchecked would do. Replaced with a clear
	// primary label + a muted helper caption so the field is self-explanatory.
	needsReviewArgs := []any{Type("checkbox"), Attr("data-testid", "txn-add-needs-review"), OnChange(onReviewed), Checked(needsReview.Get())}

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

	// C624: state the direction the typed amount resolves to, before commit.
	//
	// A typed minus means "money out" (C546/C547) and the kind toggle follows — but
	// only when the toggle disagreed. With Expense already selected, which is the
	// default, typing "-1" moved nothing and said nothing: the form sat reading
	// "Expense" over "-1" with Save enabled, and the reader had no way to tell
	// whether that would store a dollar out, a dollar in, or be rejected. The money
	// was always stored correctly; the form simply never admitted what it was about
	// to do. The ticket allows "normalize direction and show the resulting sign
	// before commit" — this is the showing half, and it is a plain sentence rather
	// than a badge because the question it answers is a sentence.
	var amountNote uic.Node = Fragment()
	if trimmed := strings.TrimSpace(amount.Get()); strings.HasPrefix(trimmed, "-") {
		if acc, ok := accountByID(accounts, effAcct); ok {
			dec := currency.Decimals(acc.Currency)
			if v, perr := money.ParseMinor(trimmed, dec); perr == nil && v != 0 {
				mag := v
				if mag < 0 {
					mag = -mag
				}
				amountNote = Div(css.Class("qa-amount-note"),
					Attr("data-testid", "txn-add-amount-note"),
					Attr("role", "status"), Attr("aria-live", "polite"),
					uistate.T("quickAdd.minusMeansOut", money.FormatMinor(mag, dec)))
			}
		}
	}

	accountError := saveAttempted.Get() && !quickAddHasAccount(accounts, effAcct)
	amountError := saveAttempted.Get() && !quickAddAmountValid(accounts, effAcct, amount.Get(), kind.Get())
	payeeDescError := saveAttempted.Get() && strings.TrimSpace(payee.Get()) == "" && strings.TrimSpace(desc.Get()) == ""
	accountAttrs := []any{css.Class("field"), Attr("id", "txn-add-account"), Attr("data-testid", "txn-add-account"), Attr("aria-label", uistate.T("quickAdd.account")), Attr("aria-required", "true")}
	amountAttrs := []any{css.Class("field"), Type("text"), Attr("inputmode", "decimal"), Attr("id", "txn-add-amount"), Attr("data-testid", "txn-add-amount"), Attr("autofocus", ""), Attr("aria-label", uistate.T("quickAdd.amount")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.amount")), ui.FieldValue(amount.Get())}
	descAttrs := []any{css.Class("field"), Type("text"), Attr("id", "txn-add-desc"), Attr("data-testid", "txn-add-desc"), Attr("aria-label", uistate.T("quickAdd.description")), Attr("aria-required", "false"), Placeholder(uistate.T("quickAdd.descPlaceholder")), ui.FieldValue(desc.Get())}
	payeeAttrs := []any{css.Class("field"), Type("text"), Attr("id", "txn-add-payee"), Attr("data-testid", "txn-add-payee"), Attr("list", "qa-payees"), Attr("aria-label", uistate.T("quickAdd.payee")), Placeholder(uistate.T("quickAdd.payeePlaceholder")), ui.FieldValue(payee.Get())}
	for _, f := range []struct {
		show bool
		id   string
		args *[]any
	}{{accountError, "txn-add-account-error", &accountAttrs}, {amountError, "txn-add-amount-error", &amountAttrs}, {payeeDescError, "txn-add-payee-error", &payeeAttrs}} {
		if f.show {
			*f.args = append(*f.args, Attr("aria-invalid", "true"), Attr("aria-describedby", f.id))
		}
	}
	fieldMessage := func(show bool, key string) string {
		if show {
			return uistate.T(key)
		}
		return ""
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
	// SM-15: type the whole thing in words and fill the fields below in one click.
	// It sits ABOVE the fields it fills — the point of the feature is to avoid
	// touching them, and a shortcut placed after the long way round is decoration.
	// Its own component: it owns its input state and the Smart+ request hooks.
	draftHint := uic.CreateElement(screens.TxnDraftHint, screens.TxnDraftHintProps{
		OnApply: func(f screens.TxnDraftFill) {
			// Each field is filled only when the parse actually found it, so a
			// partial read never wipes something already typed.
			if f.AmountMajor != "" {
				amount.Set(f.AmountMajor)
			}
			if f.Payee != "" {
				payee.Set(f.Payee)
				if strings.TrimSpace(desc.Get()) == "" {
					desc.Set(f.Payee)
				}
			}
			if !f.Date.IsZero() {
				date.Set(dateutil.FormatDate(f.Date))
			}
			if f.CategoryID != "" {
				catID.Set(f.CategoryID)
			}
			if f.Income {
				kind.Set("Income")
			} else if f.AmountMajor != "" {
				kind.Set("Expense")
			}
			focusField.Set(quickAddNextFocusR4F(accounts, effAcct, amount.Get(), payee.Get(), desc.Get(), f))
		},
	})
	draftHint = Div(css.Class("qa-sentence"),
		Label(Attr("for", "sm15-input"), uistate.T("txu3.sentenceLabel")),
		draftHint,
	)

	categoryField := Div(css.Class("labeled-field", "txn-edit-category-field"),
		Div(css.Class("txn-edit-category-label"), Span(css.Class("t-caption", tw.TextDim), uistate.T("quickAdd.category"))),
		Div(css.Class("txn-cat-picker"),
			Div(css.Class("txn-cat-row"), Select(css.Class("field"), Attr("data-testid", "txn-add-category"), Attr("aria-label", uistate.T("quickAdd.category")), OnChange(onCat), catOpts),
				screens.TxnNewCategoryAction("txn-add-newcat-toggle", categoryPickerOpen.Get(), toggleNewCategory)),
			catAssist, learnedCatAssist, budgetImpact,
			If(categoryPickerOpen.Get(), uic.CreateElement(screens.CategoryPicker, screens.CategoryPickerProps{
				Kind: func() domain.CategoryKind {
					if kind.Get() == "Income" {
						return domain.KindIncome
					}
					return domain.KindExpense
				}(),
				Selected: catID.Get(),
				OnPick:   func(categoryID string) { catID.Set(categoryID); categoryPickerOpen.Set(false) },
				OnClose:  func() { categoryPickerOpen.Set(false) },
			})),
		),
	)
	amountField := Div(
		ui.Field(ui.FieldProps{ID: "txn-add-amount", Label: uistate.T("quickAdd.amount"),
			Hint: uistate.T("sol1.amountHint"), Error: fieldMessage(amountError, "txu3.needAmount"), Required: true,
			Control: func(a ui.FieldA11y) uic.Node {
				return Input(append(amountAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onAmount), OnBlur(onAmountBlur))...)
			}}), amountNote)
	directionField := screens.TxnDirectionToggle(screens.TxnDirectionToggleProps{
		Label: uistate.T("quickAdd.kind"), Value: kind.Get(),
		Options: []screens.TxnDirectionOption{
			{Value: "Expense", Label: uistate.T("quickAdd.expense"), TestID: "txn-add-direction-expense"},
			{Value: "Income", Label: uistate.T("quickAdd.income"), TestID: "txn-add-direction-income"},
			{Value: "Transfer", Label: uistate.T("quickAdd.transfer"), TestID: "txn-add-direction-transfer"},
		},
		OnSelect: func(v string) {
			if v == "Transfer" {
				closePanel()
				transferOpen.Set(true)
				return
			}
			kind.Set(v)
		},
	})
	payeeField := ui.Field(ui.FieldProps{ID: "txn-add-payee", Label: uistate.T("quickAdd.payee"),
		Error: fieldMessage(payeeDescError, "r4f.addPayeeOrDescription"),
		Control: func(a ui.FieldA11y) uic.Node {
			return Input(append(payeeAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onPayee))...)
		}})
	dateField := ui.Field(ui.FieldProps{ID: "txn-add-date", Label: uistate.T("quickAdd.date"),
		Control: func(a ui.FieldA11y) uic.Node {
			return Input(css.Class("field"), ID(a.ID), Type("date"), Attr("data-testid", "txn-add-date"),
				Attr("aria-label", a.Label), OnInput(onDate), ui.FieldValue(effDate))
		}})
	accountControl := ui.Field(ui.FieldProps{ID: "txn-add-account", Label: uistate.T("quickAdd.account"),
		Error: fieldMessage(accountError, "txu3.needAccount"), Required: true,
		Control: func(a ui.FieldA11y) uic.Node {
			return Select(append(append(accountAttrs, Attr("aria-describedby", a.DescribedBy), OnChange(onAcct)), acctOpts)...)
		}})
	moreFields := []uic.Node{
		ui.Field(ui.FieldProps{ID: "txn-add-desc", Label: uistate.T("quickAdd.description"), Hint: uistate.T("sol1.bankDescription"),
			Control: func(a ui.FieldA11y) uic.Node {
				return Input(append(descAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onDesc))...)
			}}),
		descAssist,
		// C47: wrap the checkbox in a block container so the helper caption sits
		// below the label text, not inline with it. The outer Div is not a label
		// so screen-readers read the Input's associated Label normally.
		Div(css.Class("txn-form-check-field"),
			Label(css.Class("txn-form-check"),
				Input(needsReviewArgs...),
				uistate.T("txu3.needsReview")),
			Span(css.Class("txn-form-help"),
				uistate.T("txu3.needsReviewHelp"))),
		// Native date inputs have browser-specific sub-fields; the rapid-entry and
		// Save actions remain in normal tab order after the form controls.
		// QA M7: the metadata that used to require save-then-find-then-edit — tags,
		// member, note, cleared, report exclusion — folded behind a collapsed
		// native disclosure so the everyday five-field path stays fast.
		Div(css.Class("txn-form-detail-group"),
			ui.FormField(uistate.T("quickAdd.tags"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-tags"), Attr("aria-label", uistate.T("quickAdd.tags")),
					Placeholder(uistate.T("quickAdd.tagsPh")), OnInput(onTags), ui.FieldValue(tagsS.Get()))),
			If(len(app.Members()) > 0, screens.TxnWhoField(screens.TxnWhoFieldProps{
				ID: "txn-add-who", Owners: quickAddOwnerSelection(ownersS.Get(), activeMember, accountForQuickAdd(accounts, effAcct), ownersTouched.Get()), Colors: quickAddMemberColors(app.Members()), Options: func() []ui.PickerOption {
					options := make([]ui.PickerOption, 0, len(app.Members()))
					for _, member := range app.Members() {
						options = append(options, ui.PickerOption{Value: member.ID, Label: member.Name})
					}
					return options
				}(), OnToggle: func(owner string) {
					ownersS.Set(quickAddToggleOwner(ownersS.Get(), owner))
					ownersTouched.Set(true)
				},
			})),
			ui.FormField(uistate.T("quickAdd.note"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-note"), Attr("aria-label", uistate.T("quickAdd.note")),
					Placeholder(uistate.T("quickAdd.notePh")), OnInput(onNote), ui.FieldValue(noteS.Get()))),
			Div(css.Class("txn-form-check-field"),
				Label(css.Class("txn-form-check"), Input(Type("checkbox"), Attr("data-testid", "txn-add-cleared"),
					Attr("aria-describedby", "txn-add-cleared-hint"), OnChange(onCleared), Checked(clearedS.Get())), uistate.T("quickAdd.cleared")),
				Span(ID("txn-add-cleared-hint"), css.Class("txn-form-help"), uistate.T("sol1.clearedHint"))),
			Div(css.Class("txn-form-check-field"),
				Label(css.Class("txn-form-check"), Input(Type("checkbox"), Attr("data-testid", "txn-add-exclude"),
					Attr("aria-describedby", "txn-add-exclude-hint"), OnChange(onExclude), Checked(excludeS.Get())), uistate.T("quickAdd.exclude")),
				Span(ID("txn-add-exclude-hint"), css.Class("txn-form-help"), uistate.T("transactions.excludeHint"))),
		),
	}
	templatesSlot := Div(css.Class("txt-zone"),
		screens.TxnTemplatePicker(onPick),
		// DP-F5c: the save action sits beside the PICKER, so it is genuinely
		// ambiguous whether it saves the template you just picked or the form you
		// just filled. The zone's move to the bottom (TXT) fixed the ordering
		// complaint; this fixes the remaining one, which is that the button's own
		// label cannot say what it snapshots. One line does.
		Div(css.Class("txt-save-row"),
			saveTemplateBtn,
			Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("txnTemplates.saveHint")),
		),
	)
	body := Div(
		screens.TxnSharedForm(screens.TxnFormModel{Mode: screens.TxnFormAdd, Amount: amountField, Direction: directionField,
			Payee: payeeField, Category: categoryField, Date: dateField, Account: accountControl,
			More: moreFields, AddQuickFill: Div(draftHint, payeeDatalist), AddTemplates: templatesSlot, MoreID: "txn-add-more"}),
	)
	footer := Div(css.Class("txn-form-footer"),
		Div(css.Class("txn-form-footer-actions"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-add-cancel"), OnClick(closePanel), uistate.T("action.cancel")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-add-another"), OnClick(saveAndAnother), uistate.T("quickAdd.saveAndAnother")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("id", "txn-add-save"), Attr("data-testid", "flip-save"), OnClick(save), uistate.T("action.save")),
		),
	)
	return ui.Overlay(ui.OverlayProps{
		Open: props.Open, Kind: ui.OverlayModal, Title: uistate.T("quickAdd.title"),
		TestID: "txn-add-dialog", Body: body, Footer: footer, OnClose: closePanel,
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

func quickAddHasAccount(accounts []domain.Account, accountID string) bool {
	_, ok := accountByID(accounts, accountID)
	return ok
}

func resolveQuickAddAccountID(input accountselect.ResolveInput) string {
	id, _ := accountselect.Resolve(input)
	return id
}

func quickAddAmountValid(accounts []domain.Account, accountID, raw, kind string) bool {
	account, ok := accountByID(accounts, accountID)
	if !ok {
		return false
	}
	want := amountmath.Out
	if kind == "Income" {
		want = amountmath.In
	}
	_, _, err := amountmath.ParseSigned(raw, currency.Decimals(account.Currency), want)
	return err == nil
}

func quickAddInvalidFieldR4F(accounts []domain.Account, accountID, rawAmount, kind, payee, description string) (string, string) {
	if !quickAddHasAccount(accounts, accountID) {
		return "txn-add-account", "txu3.needAccount"
	}
	if !quickAddAmountValid(accounts, accountID, rawAmount, kind) {
		return "txn-add-amount", "txu3.needAmount"
	}
	if strings.TrimSpace(payee) == "" && strings.TrimSpace(description) == "" {
		return "txn-add-payee", "r4f.addPayeeOrDescription"
	}
	return "", ""
}

func quickAddDescriptionR4F(payee, description string) string {
	description = strings.TrimSpace(description)
	if description != "" {
		return description
	}
	return strings.TrimSpace(payee)
}

func quickAddNextFocusR4F(accounts []domain.Account, accountID, amount, payee, description string, fill screens.TxnDraftFill) string {
	if strings.TrimSpace(amount) == "" && fill.AmountMajor == "" {
		return "txn-add-amount"
	}
	if !quickAddHasAccount(accounts, accountID) {
		return "txn-add-account"
	}
	if strings.TrimSpace(payee) == "" && strings.TrimSpace(description) == "" && fill.Payee == "" {
		return "txn-add-payee"
	}
	return "txn-add-save"
}

func quickAddMemberValue(activeMember, selected string, explicitlySelected bool) string {
	if explicitlySelected {
		return selected
	}
	return activeMember
}

func quickAddOwnerDefaults(activeMember string) []string {
	if activeMember == "" {
		return nil
	}
	return []string{activeMember}
}

func quickAddOwnerSelection(owners []string, activeMember string, account domain.Account, touched bool) []string {
	if touched || len(owners) > 0 {
		return append([]string(nil), owners...)
	}
	if activeMember != "" {
		return []string{activeMember}
	}
	if account.Scope == domain.ScopeIndividual && account.OwnerID != "" {
		return []string{account.OwnerID}
	}
	return nil
}

func accountForQuickAdd(accounts []domain.Account, accountID string) domain.Account {
	account, _ := accountByID(accounts, accountID)
	return account
}

func quickAddMemberColors(members []domain.Member) map[string]string {
	colors := make(map[string]string, len(members))
	for _, member := range members {
		colors[member.ID] = member.Color
	}
	return colors
}

func quickAddToggleOwner(owners []string, owner string) []string {
	for i, current := range owners {
		if current == owner {
			return append(append([]string(nil), owners[:i]...), owners[i+1:]...)
		}
	}
	return append(append([]string(nil), owners...), owner)
}

func quickAddReviewedValue(needsReview bool) bool {
	return !needsReview
}

func focusQuickAddField(id string) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	field := doc.Call("getElementById", id)
	if !field.IsNull() && !field.IsUndefined() {
		field.Call("focus")
	}
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

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetNewCatSentinel is the category-picker value that means "create a new
// category" (named after the budget) instead of selecting an existing one.
const budgetNewCatSentinel = "__new_category__"

// parseTrackedTags splits a comma-separated tag input into a trimmed, "#"-stripped,
// case-insensitively-deduped list — the cross-category tags a budget also tracks. The
// dedupe means selecting the same tag twice never counts a charge twice.
func parseTrackedTags(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(s, ",") {
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// BudgetAddFormProps configures the BudgetAddForm component.
type BudgetAddFormProps struct {
	// OnDone is called after a successful add so the caller (e.g. AddHost) can
	// close the modal. On a validation error the form stays open and OnDone is
	// not called.
	OnDone func()
	// Seed pre-fills the form when the modal was opened from another surface
	// ("Budget this" on an unbudgeted category). Zero value = a blank form.
	Seed uistate.BudgetAddSeed
}

// BudgetAddForm is the standalone add-a-budget form. It owns all its state
// and handlers. On success it calls props.OnDone; on error it shows an inline
// message and stays open. Extracted from Budgets() for use in the AddHost modal.
func BudgetAddForm(props BudgetAddFormProps) ui.Node {
	return ui.CreateElement(budgetAddForm, props)
}

// matchExpenseCategory returns the existing expense category whose name equals
// name (case-insensitive, trimmed), or a zero Category when there is none. It
// is the guard that keeps the "create a new category" default from silently
// minting a duplicate of a category the household already has.
func matchExpenseCategory(categories []domain.Category, name string) (domain.Category, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return domain.Category{}, false
	}
	for _, c := range categories {
		if c.Kind == domain.KindExpense && strings.ToLower(strings.TrimSpace(c.Name)) == n {
			return c, true
		}
	}
	return domain.Category{}, false
}

func budgetAddForm(props BudgetAddFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	categories := app.Categories()
	var expenseCats []domain.Category
	for _, c := range categories {
		if c.Kind == domain.KindExpense {
			expenseCats = append(expenseCats, c)
		}
	}

	// A budget watches a category. By default we create a NEW category named after the
	// budget, so a transaction can be assigned to it immediately (closing the loop) —
	// unless the typed name matches an existing category, which is then reused (no
	// silent duplicates). The Advanced section still lets the user pick explicitly.
	defaultCat := budgetNewCatSentinel
	if props.Seed.CategoryID != "" {
		defaultCat = props.Seed.CategoryID
	}
	defaultPeriod := string(domain.PeriodMonthly)
	if props.Seed.Period != "" {
		defaultPeriod = props.Seed.Period
	}

	name := ui.UseState(props.Seed.Name)
	ev := useEntityVarField(budgetVarKind, name, "")
	limit := ui.UseState(props.Seed.LimitMajor)
	catID := ui.UseState(defaultCat)
	newCatName := ui.UseState("")
	owner := ui.UseState(domain.GroupOwnerID)
	period := ui.UseState(defaultPeriod)
	rollover := ui.UseState(false)
	methodology := ui.UseState("") // empty = inherit global method
	customVals := ui.UseState(map[string]string{})
	// trackTags: comma-separated tags this budget also counts, across categories (a
	// "#vacation" cap spanning travel + dining + shopping). Parsed + deduped on submit.
	trackTags := ui.UseState("")
	onTrackTags := ui.UseEvent(func(v string) { trackTags.Set(v) })
	// alsoTrack: extra existing categories this budget should also count (multi-category).
	// The primary category above is always tracked; these add to it.
	alsoTrack := ui.UseState(map[string]bool{})
	toggleAlso := func(id string) {
		m := alsoTrack.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[id] = !nm[id]
		alsoTrack.Set(nm)
	}
	errMsg := ui.UseState("")
	// The essentials-first layout: everything beyond Name/Limit/Period lives behind
	// this disclosure, so the common case reads as a two-field form. A seeded category
	// opens it so the pre-fill is visible rather than silently applied.
	advOpen := ui.UseState(props.Seed.CategoryID != "")
	toggleAdv := ui.UseEvent(Prevent(func() { advOpen.Set(!advOpen.Get()) }))
	// 50/30/20 review mode: the template button switches the modal body to a per-line
	// review (checkboxes + amounts) instead of a count-only confirm, so the user sees
	// and prunes exactly what will be created.
	tmplOpen := ui.UseState(false)
	openTmpl := ui.UseEvent(Prevent(func() { tmplOpen.Set(true) }))
	closeTmpl := ui.UseEvent(Prevent(func() { tmplOpen.Set(false) }))
	tmplExcluded := ui.UseState(map[string]bool{})
	toggleTmpl := func(id string) {
		m := tmplExcluded.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[id] = !nm[id]
		tmplExcluded.Set(nm)
	}

	onLimit := ui.UseEvent(func(v string) { limit.Set(v) })
	onNewCatName := ui.UseEvent(func(v string) { newCatName.Set(v) })
	onRollover := ui.UseEvent(func() { rollover.Set(!rollover.Get()) })
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	budgetDefs := app.CustomFieldDefsFor("budget")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

	// Copy an existing budget: choosing one pre-fills the form from it (name gets a
	// "copy" suffix; the category/period/method/rollover carry over) and opens the
	// Advanced section so every carried-over value is visible, not silently applied.
	copyFrom := func(bid string) {
		for _, b := range app.Budgets() {
			if b.ID != bid {
				continue
			}
			name.Set(uistate.T("budgets.copySuffix", b.Name))
			limit.Set(money.FormatMinor(b.Limit.Amount, currency.Decimals(b.Limit.Currency)))
			period.Set(string(b.Period))
			owner.Set(b.OwnerID)
			methodology.Set(b.Methodology)
			rollover.Set(b.Rollover)
			if b.CategoryID != "" {
				catID.Set(b.CategoryID)
			}
			also := map[string]bool{}
			for i, cid := range b.TrackedCategoryIDs() {
				if i > 0 {
					also[cid] = true
				}
			}
			alsoTrack.Set(also)
			trackTags.Set(strings.Join(b.TrackedTags, ", "))
			advOpen.Set(true)
			return
		}
	}

	add := ui.UseEvent(Prevent(func() {
		// Name first: it's the budget's identity and (by default) its category's name.
		if strings.TrimSpace(name.Get()) == "" {
			errMsg.Set(uistate.T("budgets.nameRequired"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(limit.Get()), currency.Decimals(base))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("budgets.limitRequired"))
			return
		}
		// Reject a variable name that collides with an existing budget's handle.
		if warn := entityVarCollision(budgetVarKind, budgetVarEntities(app.Budgets()), "", ev.VarName.Get(), name.Get()); warn != "" {
			errMsg.Set(warn)
			return
		}
		// Resolve the category. "New category" first tries to REUSE an existing expense
		// category with the same name (no silent duplicates); only a genuinely new name
		// creates one. Either way the duplicate-budget guard below applies to the
		// resolved category, so the one-budget-per-(category, period, owner) rule can't
		// be bypassed through the create-new path.
		finalCatID := catID.Get()
		createdCatName := ""
		if finalCatID == budgetNewCatSentinel {
			catName := strings.TrimSpace(newCatName.Get())
			if catName == "" {
				catName = strings.TrimSpace(name.Get())
			}
			if existing, ok := matchExpenseCategory(categories, catName); ok {
				finalCatID = existing.ID
			} else {
				nc := domain.Category{ID: id.New(), Name: catName, Kind: domain.KindExpense}
				if err := app.PutCategory(nc); err != nil {
					errMsg.Set(err.Error())
					return
				}
				finalCatID = nc.ID
				createdCatName = catName
			}
		}
		// One budget per (category, period, owner) — reject duplicates (L40). Runs for
		// every path now that the category is resolved.
		if createdCatName == "" && budgeting.IsDuplicateBudget(app.Budgets(), finalCatID, period.Get(), owner.Get(), "") {
			errMsg.Set(uistate.T("budgets.duplicateBudget"))
			return
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		// Resolve per-budget methodology override: empty = inherit global.
		methodVal := methodology.Get()
		if m := budgeting.Methodology(methodVal); methodVal != "" && !m.Valid() {
			methodVal = ""
		}
		// Fold in any "also track" extras (existing expense categories), keeping the
		// primary first and de-duped. Only set CategoryIDs when tracking more than one.
		catIDs := []string{finalCatID}
		seen := map[string]bool{finalCatID: true}
		selAlso := alsoTrack.Get()
		for _, c := range app.Categories() {
			if c.Kind == domain.KindExpense && selAlso[c.ID] && !seen[c.ID] {
				seen[c.ID] = true
				catIDs = append(catIDs, c.ID)
			}
		}
		b := domain.Budget{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			CategoryID: finalCatID, Period: domain.Period(period.Get()), Limit: money.New(amt, base),
			Rollover: rollover.Get(), Methodology: methodVal, Custom: customValuesToMap(budgetDefs, customVals.Get()),
			VarName: strings.TrimSpace(ev.VarName.Get()),
		}
		if len(catIDs) > 1 {
			b.CategoryIDs = catIDs
		}
		if tags := parseTrackedTags(trackTags.Get()); len(tags) > 0 {
			b.TrackedTags = tags
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.BumpDataRevision() // surface the new budget (and category) immediately
		// Reset fields.
		name.Set("")
		ev.Reset()
		limit.Set("")
		rollover.Set(false)
		methodology.Set("")
		catID.Set(budgetNewCatSentinel)
		newCatName.Set("")
		alsoTrack.Set(map[string]bool{})
		trackTags.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		advOpen.Set(false)
		if createdCatName != "" {
			uistate.PostNotice(uistate.T("budgets.addedWithCatToast", createdCatName), false)
		} else {
			uistate.PostNotice(uistate.T("budgets.addedToast"), false)
		}
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	// 50/30/20 proposals for the review list (recomputed per render while open — pure
	// and deterministic over the current data). Zero income disables the template.
	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	now := time.Now()
	curStart := dateutil.MonthStart(now)
	prevStart := dateutil.AddMonths(curStart, -1)
	tmplIncome := budgeting.IncomeForBudgets(uistate.CurrentPrefs().MonthlyIncomeMinor, txns, prevStart, curStart, base, rates)
	var tmplProposals []budgeting.BudgetProposal
	if tmplIncome > 0 {
		res := budgeting.Generate5030(tmplIncome, categories, txns, now)
		existing := map[string]bool{}
		for _, b := range app.Budgets() {
			existing[b.CategoryID] = true
		}
		for _, prop := range res.Proposals {
			if prop.LimitMinor > 0 && !existing[prop.Category.ID] {
				tmplProposals = append(tmplProposals, prop)
			}
		}
	}

	// Apply the reviewed template: create only the checked proposals.
	applyTmpl := ui.UseEvent(Prevent(func() {
		excluded := tmplExcluded.Get()
		n := 0
		for _, prop := range tmplProposals {
			if excluded[prop.Category.ID] {
				continue
			}
			nb := domain.Budget{
				ID: id.New(), Name: prop.Category.Name, CategoryID: prop.Category.ID,
				Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
				Period: domain.PeriodMonthly, Limit: money.New(prop.LimitMinor, base),
			}
			if err := app.PutBudget(nb); err == nil {
				n++
			}
		}
		if n == 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNothingToAdd"), false)
			return
		}
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("budgets.tmplApplied", plural(n, "budget")))
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	// The picker leads with "➕ Create a new category" (the default), then every existing
	// expense category, so the common case (a budget for something new) is one step.
	catOptions := []uiw.SelectOption{{Value: budgetNewCatSentinel, Label: uistate.T("budgets.newCategoryOption")}}
	catOptions = append(catOptions, uiw.OptionsFrom(expenseCats,
		func(c domain.Category) string { return c.ID },
		func(c domain.Category) string { return c.Name },
		catID.Get())...)
	ownerOptions := ownerSelectOptions(app.Members(), owner.Get())

	// Copy-an-existing-budget options (creation-time duplicate, G4). Leads with a
	// placeholder so it reads as an action, not a value.
	copyOptions := []uiw.SelectOption{{Value: "", Label: uistate.T("budgets.copyExisting")}}
	for _, b := range app.Budgets() {
		copyOptions = append(copyOptions, uiw.SelectOption{Value: b.ID, Label: b.Name})
	}

	// Resolve where the category WILL land (mirroring the add handler) so the form can
	// say so up front: reuse an existing same-named category, or create a new one. The
	// same resolution feeds the limit suggestion, so it fires on the default path too
	// (typing "Groceries" suggests from the Groceries category's history).
	sugCatID := catID.Get()
	var catFateHint string
	if catID.Get() == budgetNewCatSentinel {
		catName := strings.TrimSpace(newCatName.Get())
		if catName == "" {
			catName = strings.TrimSpace(name.Get())
		}
		if catName != "" {
			if existing, ok := matchExpenseCategory(categories, catName); ok {
				sugCatID = existing.ID
				catFateHint = uistate.T("budgets.catWillReuse", existing.Name)
			} else {
				catFateHint = uistate.T("budgets.catWillCreate", catName)
			}
		}
	}
	var catFateNode ui.Node = Fragment()
	if catFateHint != "" {
		catFateNode = Span(css.Class("budget-cat-fate", tw.TextFaint), Attr("data-testid", "budget-cat-fate"), catFateHint)
	}

	// Suggest a limit from the resolved category's recent monthly spend (D6).
	suggestion, _ := budgeting.SuggestLimit(sugCatID, txns, now, 6, rates)

	// Owner-scope consequence: picking a member silently made the budget individual;
	// say what the choice means right under the picker.
	ownerHint := uistate.T("budgets.ownerSharedHint")
	if owner.Get() != domain.GroupOwnerID {
		ownerName := ""
		for _, m := range app.Members() {
			if m.ID == owner.Get() {
				ownerName = m.Name
				break
			}
		}
		ownerHint = uistate.T("budgets.ownerIndividualHint", ownerName)
	}

	advLabel := uistate.T("budgets.advancedShow")
	if advOpen.Get() {
		advLabel = uistate.T("budgets.advancedHide")
	}

	// ---- 50/30/20 review mode -------------------------------------------------------
	if tmplOpen.Get() {
		excluded := tmplExcluded.Get()
		var total int64
		checked := 0
		rows := MapKeyed(tmplProposals, func(p budgeting.BudgetProposal) any { return p.Category.ID }, func(p budgeting.BudgetProposal) ui.Node {
			return ui.CreateElement(tmplReviewRow, tmplReviewRowProps{
				ID: p.Category.ID, Label: p.Category.Name, AmountStr: fmtMoney(money.New(p.LimitMinor, base)),
				Checked: !excluded[p.Category.ID], OnToggle: toggleTmpl,
			})
		})
		for _, p := range tmplProposals {
			if !excluded[p.Category.ID] {
				checked++
				total += p.LimitMinor
			}
		}
		var emptyNote ui.Node = Fragment()
		if len(tmplProposals) == 0 {
			emptyNote = P(css.Class("empty"), uistate.T("budgets.tmplNothingToAdd"))
		}
		return Form(css.Class("budget-add-shell"), Attr("data-testid", "budget-tmpl-review"), OnSubmit(applyTmpl),
			Div(css.Class("modal-scroll"),
				P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}), uistate.T("budgets.tmplReviewHint")),
				Div(css.Class("budget-tmpl-rows"), rows),
				emptyNote,
				If(checked > 0, P(css.Class("budget-tmpl-total"), Attr("data-testid", "budget-tmpl-total"),
					uistate.T("budgets.tmplReviewTotal", fmtMoney(money.New(total, base)), plural(checked, "budget")))),
			),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-tmpl-back"), OnClick(closeTmpl), uistate.T("budgets.tmplBack")),
				Button(css.Class("btn btn-primary", "ba-submit"), Type("submit"), Attr("data-testid", "budget-tmpl-apply"),
					attrIf(checked == 0, "disabled", "disabled"),
					uistate.T("budgets.tmplCreateN", plural(checked, "budget"))),
			),
		)
	}

	// ---- the add form (essentials first, the rest behind Advanced) -------------------
	return Form(css.Class("budget-add-shell"), Attr("data-testid", "budget-add-form"), OnSubmit(add),
		Div(css.Class("modal-scroll"),
			// Start-from shortcuts: the 50/30/20 review, or copy an existing budget.
			Div(css.Class("budget-add-tmpl"), Attr("data-testid", "budget-add-tmpl"),
				Div(css.Class("row-main"),
					Span(css.Class("budget-add-tmpl-title"), uistate.T("budgets.tmplBannerTitle")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.tmplBannerHint")),
				),
				Div(css.Class("budget-add-tmpl-actions"),
					Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "budgets-template-503020"),
						Title(uistate.T("budgets.tmplTitle")), OnClick(openTmpl),
						uiw.Icon(icon.Split, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.tmpl503020"))),
					If(len(copyOptions) > 1, uiw.SelectInput(uiw.SelectInputProps{
						Options:   copyOptions,
						Selected:  "",
						OnChange:  func(v string) { copyFrom(v) },
						AriaLabel: uistate.T("budgets.copyExisting"),
						TestID:    "budget-copy-existing",
					})),
				),
			),
			Div(css.Class("budget-add-or"), Span(uistate.T("budgets.tmplOr"))),
			Div(css.Class("form-grid", "budget-add-grid"),
				// The essentials: Name, then Limit + Period. Everything else is Advanced.
				Div(css.Class("ba-full"),
					labeledField(uistate.T("common.name"),
						Input(append([]any{css.Class("field"), Attr("id", "budget-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(ev.OnName)}, errAttrs("budget-err", errMsg.Get())...)...))),
				// Where the category will land (reuse vs create) — the side effect, said out loud.
				If(catFateHint != "", Div(css.Class("ba-full"), catFateNode)),
				labeledField(uistate.T("budgets.limitLabel"),
					Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("budgets.limitPlaceholder", base)), Value(limit.Get()), Step("0.01"), OnInput(onLimit))),
				labeledField(uistate.T("budgets.period"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   periodOptions(period.Get()),
						Selected:  period.Get(),
						OnChange:  func(v string) { period.Set(v) },
						AriaLabel: uistate.T("budgets.period"),
					})),
				If(suggestion > 0, Div(css.Class("ba-full", "suggest-row"),
					Span(css.Class("muted"), uistate.T("budgets.suggest", fmtMoney(money.New(suggestion, base)))),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-use-suggest"), OnClick(func() { limit.Set(money.FormatMinor(suggestion, currency.Decimals(base))) }), uistate.T("budgets.useSuggest")),
				)),
				// Advanced: identity + tracking + ownership details most adds never touch.
				Div(css.Class("ba-full"),
					Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("data-testid", "budget-add-advanced"),
						Attr("aria-expanded", ariaBool(advOpen.Get())), OnClick(toggleAdv), Text(advLabel))),
				If(advOpen.Get(), Fragment(
					Div(css.Class("ba-full"),
						labeledField(uistate.T("budgets.varNameLabel"),
							entityVarField(budgetVarKind, budgetVarEntities(app.Budgets()), "", "budget-add-varname", "budget-add-varname-warn", ev.VarName.Get(), name.Get(), ev.OnVarName))),
					// Category is full-width so its long "Create a new category" option isn't
					// truncated, and the new-category name field sits directly beneath it.
					Div(css.Class("ba-full"),
						labeledField(uistate.T("budgets.categoryLabel"),
							uiw.SelectInput(uiw.SelectInputProps{
								Options:   catOptions,
								Selected:  catID.Get(),
								OnChange:  func(v string) { catID.Set(v) },
								AriaLabel: uistate.T("budgets.categoryLabel"),
							}))),
					If(catID.Get() == budgetNewCatSentinel, Div(css.Class("ba-full"),
						labeledField(uistate.T("budgets.newCategoryName"),
							Input(css.Class("field"), Type("text"), Attr("data-testid", "budget-new-cat-name"),
								Placeholder(uistate.T("budgets.newCategoryPlaceholder")), Value(newCatName.Get()), OnInput(onNewCatName))))),
					// Optional multi-category: track more existing categories in this one budget.
					If(len(expenseCats) > 0, Div(css.Class("ba-full"),
						labeledField(uistate.T("budgets.catsAlsoTrack"),
							ui.CreateElement(budgetCategoryPicker, budgetCategoryPickerProps{Picked: alsoTrack.Get(), OnToggle: toggleAlso})))),
					// Optional cross-category tag tracking: count any charge with these tags,
					// whatever its category. Comma-separated; parsed + deduped on save.
					Div(css.Class("ba-full"),
						labeledField(uistate.T("budgets.tagsFieldLabel"),
							Fragment(
								Input(css.Class("field"), Type("text"), Attr("data-testid", "budget-add-tags"),
									Placeholder(uistate.T("budgets.tagsPlaceholder")),
									Value(trackTags.Get()), OnInput(onTrackTags)),
								Span(css.Class("budget-owner-hint", tw.TextFaint), uistate.T("budgets.tagsFieldHint"))))),
					// Owner (hidden until members exist) with its scope consequence spelled out.
					If(len(app.Members()) > 0, Fragment(
						labeledField(uistate.T("common.owner"),
							uiw.SelectInput(uiw.SelectInputProps{
								Options:   ownerOptions,
								Selected:  owner.Get(),
								OnChange:  func(v string) { owner.Set(v) },
								AriaLabel: uistate.T("common.owner"),
							})),
						Div(css.Class("ba-full"),
							Span(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "budget-owner-hint"), ownerHint)))),
					labeledField(uistate.T("budgets.methodLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   budgetMethodOptions(methodology.Get()),
							Selected:  methodology.Get(),
							OnChange:  func(v string) { methodology.Set(v) },
							AriaLabel: uistate.T("budgets.methodLabel"),
						})),
					// Rollover gets its own full-width row so the label never wraps.
					Label(css.Class("ba-full", "ba-check"),
						Input(append([]any{Type("checkbox"), Attr("style", "flex-shrink:0"), OnChange(onRollover)}, checkedAttr(rollover.Get())...)...),
						Span(Title(uistate.T("budgets.rolloverTitle")), uistate.T("budgets.rollover")),
					),
					MapKeyed(budgetDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
						return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
					}),
				)),
			),
			errText("budget-err", errMsg.Get()),
		),
		// Action bar pinned to the bottom of the modal: a quiet Cancel and the primary,
		// full-width-feeling "Add budget".
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary", "ba-submit"), Type("submit"), uistate.T("budgets.add")),
		),
	)
}

// attrIf returns the attribute option when cond holds, else a no-op fragment arg.
func attrIf(cond bool, k, v string) any {
	if cond {
		return Attr(k, v)
	}
	return Fragment()
}

// tmplReviewRowProps drives one proposal line in the 50/30/20 review list.
type tmplReviewRowProps struct {
	ID, Label, AmountStr string
	Checked              bool
	OnToggle             func(string) // plain func — never an On* hook (no-On*-in-loop rule)
}

// tmplReviewRow is one checkbox row of the template review: include/exclude the
// proposed budget. Its own component so the change hook sits at a stable call-site.
func tmplReviewRow(props tmplReviewRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	return Label(css.Class("budget-tmpl-row"),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "tmpl-pick-"+props.ID), OnChange(toggle)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("budget-tmpl-name"), props.Label),
		Span(css.Class("budget-tmpl-amt"), props.AmountStr),
	)
}

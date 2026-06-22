//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Budgets shows spend against each budget for the current month, with an add
// form and per-row delete.
func Budgets() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:budgets", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	// Drill from a budget to its spending: open Transactions filtered to the
	// budget's category (mirrors Accounts→Transactions and the dashboard
	// tile-click, C30/C50) — the natural "why am I over?" affordance.
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTransactions := func(categoryID string) {
		f := uistate.TxFilter{Category: categoryID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	categories := app.Categories()
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	var expenseCats []domain.Category
	for _, c := range categories {
		if c.Kind == domain.KindExpense {
			expenseCats = append(expenseCats, c)
		}
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	name := ui.UseState("")
	limit := ui.UseState("")
	defaultCat := ""
	if len(expenseCats) > 0 {
		defaultCat = expenseCats[0].ID
	}
	catID := ui.UseState(defaultCat)
	owner := ui.UseState(domain.GroupOwnerID)
	period := ui.UseState(string(domain.PeriodMonthly))
	rollover := ui.UseState(false)
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	// The viewed period comes from the shared top-bar resolution control (C7) —
	// the Budgets card no longer has its own competing month stepper.
	periodWin := uistate.UsePeriod()
	weekStart := uistate.UsePrefs().Get().WeekStartWeekday()

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limit.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onPeriod := ui.UseEvent(func(e ui.Event) { period.Set(e.GetValue()) })
	onRollover := ui.UseEvent(func() { rollover.Set(!rollover.Get()) })

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
	add := ui.UseEvent(Prevent(func() {
		amt, err := money.ParseMinor(strings.TrimSpace(limit.Get()), currency.Decimals(base))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("budgets.limitRequired"))
			return
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		b := domain.Budget{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			CategoryID: catID.Get(), Period: domain.Period(period.Get()), Limit: money.New(amt, base),
			Rollover: rollover.Get(), Custom: customValuesToMap(budgetDefs, customVals.Get()),
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		limit.Set("")
		rollover.Set(false)
		if len(expenseCats) > 0 {
			catID.Set(expenseCats[0].ID) // reset category to the default after add (L40)
		}
		customVals.Set(map[string]string{})
		errMsg.Set("")
		bump()
		uistate.PostNotice(uistate.T("budgets.addedToast"), false) // L40 success confirmation
	}))

	deleteBudget := func(budgetID string) {
		if err := app.DeleteBudget(budgetID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	saveBudget := func(id, newName, limitStr, periodStr, ownerID string, rollover bool) {
		for _, b := range app.Budgets() {
			if b.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				b.Name = n
			}
			amt, err := money.ParseMinor(strings.TrimSpace(limitStr), currency.Decimals(base))
			if err != nil || amt <= 0 {
				errMsg.Set(uistate.T("budgets.limitRequired"))
				return
			}
			b.Limit = money.New(amt, base)
			if p := domain.Period(periodStr); p.Valid() {
				b.Period = p
			}
			b.OwnerID = ownerID
			if ownerID == domain.GroupOwnerID {
				b.Scope = domain.ScopeShared
			} else {
				b.Scope = domain.ScopeIndividual
			}
			b.Rollover = rollover
			if err := app.PutBudget(b); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}

	// coverBudget (L1) moves money from another budget's limit into this one to
	// clear an overspend. It returns any error so the row can show it inline.
	coverBudget := func(toID, fromID, amountStr string) error {
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(base))
		if err != nil || amt <= 0 {
			return fmt.Errorf("enter an amount greater than zero")
		}
		if err := app.CoverBudget(fromID, toID, money.New(amt, base)); err != nil {
			return err
		}
		bump()
		return nil
	}

	var formCard ui.Node
	if len(expenseCats) == 0 {
		formCard = Section(css.Class("card"), P(css.Class("empty"), uistate.T("budgets.needCategory")))
	} else {
		catOptions := make([]ui.Node, 0, len(expenseCats))
		for _, c := range expenseCats {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		ownerOptions := ownerSelectOptions(app.Members(), owner.Get())
		// Suggest a limit from the selected category's recent monthly spend, with a
		// one-tap "use this" that fills the limit field (D6/budget hygiene).
		suggestRates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		suggestion, _ := budgeting.SuggestLimit(catID.Get(), app.Transactions(), time.Now(), 6, suggestRates)
		formCard = Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("budgets.add")),
			Form(css.Class("form-grid"), OnSubmit(add),
				labeledField(uistate.T("common.name"),
					Input(append([]any{css.Class("field"), Attr("id", "budget-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("budget-err", errMsg.Get())...)...)),
				labeledField(uistate.T("budgets.categoryLabel"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("budgets.categoryLabel")), OnChange(onCat), catOptions)),
				labeledField(uistate.T("common.owner"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("common.owner")), OnChange(onOwner), ownerOptions)),
				labeledField(uistate.T("budgets.period"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("budgets.period")), Title(uistate.T("budgets.period")), OnChange(onPeriod), periodOptions(period.Get()))),
				labeledField(uistate.T("budgets.limitLabel"),
					Input(css.Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("budgets.limitPlaceholder", base)), Value(limit.Get()), Step("0.01"), OnInput(onLimit))),
				Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(append([]any{Type("checkbox"), OnChange(onRollover)}, checkedAttr(rollover.Get())...)...),
					Span(uistate.T("budgets.rollover")),
				),
				MapKeyed(budgetDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
			),
			If(suggestion > 0, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Span(css.Class("muted"), uistate.T("budgets.suggest", fmtMoney(money.New(suggestion, base)))),
				Button(css.Class("btn"), Type("button"), OnClick(func() { limit.Set(money.FormatMinor(suggestion, currency.Decimals(base))) }), uistate.T("budgets.useSuggest")),
			)),
			errText("budget-err", errMsg.Get()),
		)
	}

	budgets := app.Budgets()
	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	now := time.Now()
	// Budgets track their own cadence. Anchor each budget's period to today when the
	// viewed window includes today, otherwise to the window's start. Fixes C40: under
	// a Quarter view the old code anchored to the quarter's START, so a Monthly budget
	// showed the quarter's FIRST month, making "Quarter" spend appear less than
	// "Month". Anchoring to today shows the current period under any containing view,
	// while navigating to a past window still shows that window's period.
	vw := periodWin.Get()
	viewFrom, viewTo := vw.Range()
	anchor := viewFrom
	if !now.Before(viewFrom) && now.Before(viewTo) {
		anchor = now
	}
	cats := app.Categories()
	// Each budget rolls up its sub-categories' spend (D5).
	statuses := make([]budgeting.Status, 0, len(budgets))
	paceOver := map[string]string{}  // budgetID → formatted projected overspend (in-progress only)
	rollCarry := map[string]string{} // budgetID → formatted previous-period carry
	rollNeg := map[string]bool{}     // budgetID → whether the previous-period carry is negative
	for _, b := range budgets {
		bs, be := budgeting.PeriodRange(b.Period, anchor, weekStart)
		st, err := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
		if err != nil {
			continue
		}
		statuses = append(statuses, st)
		// Pace projection (D2): warn only while the period is genuinely in progress
		// and the budget isn't already over — "you're spending too fast" — so a
		// finished period or an already-over budget doesn't double up the message.
		if p := budgeting.ProjectPace(st, bs, be, now); !p.OnTrack && p.Elapsed > 0 && p.Elapsed < 1 && st.State != budgeting.StateOver {
			paceOver[b.ID] = fmtMoney(p.OverBy)
		}
		if b.Rollover {
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, err := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); err == nil {
				rollCarry[b.ID] = fmtMoney(prev.Remaining)
				rollNeg[b.ID] = prev.Remaining.IsNegative()
			}
		}
	}

	overCount, nearCount := 0, 0
	var totalSpent, totalLimit int64
	for _, s := range statuses {
		switch s.State {
		case budgeting.StateOver:
			overCount++
		case budgeting.StateNear:
			nearCount++
		}
		totalSpent += s.Spent.Amount
		totalLimit += s.Spent.Amount + s.Remaining.Amount // limit = spent + remaining
	}

	// The methodology shapes the Budgets view (D6): zero-based surfaces unassigned
	// income; envelope shows each budget's carried-forward balance.
	method := budgeting.ParseMethodology(app.Settings().BudgetMethodology)
	envAvail := map[string]string{} // budgetID → formatted envelope balance (envelope mode)
	envNeg := map[string]bool{}     // budgetID → whether the envelope is overdrawn
	var assignBanner ui.Node = Fragment()
	switch method {
	case budgeting.MethodZeroBased:
		ms, me := budgeting.PeriodRange(domain.PeriodMonthly, anchor, weekStart)
		income, _, _ := ledger.PeriodTotals(txns, ms, me, rates)
		toAssign := budgeting.ToAssign(income.Amount, totalLimit)
		switch {
		case toAssign > 0:
			assignBanner = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.toAssign", fmtMoney(money.New(toAssign, base))))
		case toAssign == 0:
			assignBanner = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.allAssigned"))
		default:
			assignBanner = P(css.Class("budget-sub", tw.FontDisplay, tw.TextDown), uistate.T("budgets.overAssigned", fmtMoney(money.New(-toAssign, base))))
		}
	case budgeting.MethodEnvelope:
		assignBanner = P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.envelopeNote"))
		for _, b := range budgets {
			if av, err := budgeting.EnvelopeAvailable(b, txns, anchor, weekStart, rates, categorytree.Descendants(cats, b.CategoryID)); err == nil {
				envAvail[b.ID] = fmtMoney(av)
				envNeg[b.ID] = av.IsNegative()
			}
		}
	}

	var listBody ui.Node
	if len(statuses) == 0 {
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), FocusID: "budget-add"})
	} else {
		// Source budgets a "Cover…" action can pull from: every budget, labelled with
		// its remaining room (the row drops itself when building its picker).
		coverSources := make([]coverSource, 0, len(statuses))
		for _, s := range statuses {
			coverSources = append(coverSources, coverSource{
				ID:    s.Budget.ID,
				Label: budgetTitle(s.Budget.Name, catName[s.Budget.CategoryID]) + " · " + fmtMoney(s.Remaining) + " left",
			})
		}
		rows := MapKeyed(statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				shortfall := budgeting.CoverAmount(s)
				coverDefault := ""
				if shortfall.IsPositive() {
					coverDefault = money.FormatMinor(shortfall.Amount, currency.Decimals(shortfall.Currency))
				}
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], Members: app.Members(), Envelope: envAvail[s.Budget.ID], EnvelopeNeg: envNeg[s.Budget.ID], PaceOver: paceOver[s.Budget.ID], RolloverCarry: rollCarry[s.Budget.ID], RolloverNeg: rollNeg[s.Budget.ID], CoverSources: coverSources, CoverShortfall: fmtMoney(shortfall), CoverDefault: coverDefault, OnDelete: deleteBudget, OnSave: saveBudget, OnCover: coverBudget, OnDrill: viewTransactions})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		formCard,
		If(len(statuses) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("budgets.spent"), fmtMoney(money.New(totalSpent, base)), "neg"),
			stat(uistate.T("budgets.budgeted"), fmtMoney(money.New(totalLimit, base)), ""),
			stat(uistate.T("budgets.left"), fmtMoney(money.New(totalLimit-totalSpent, base)), accentFor(money.New(totalLimit-totalSpent, base))),
		)),
		Section(css.Class("card"),
			Div(css.Class("budget-head"),
				H2(css.Class("card-title"), uistate.T("nav.budgets")),
			),
			assignBanner,
			If(overCount > 0 || nearCount > 0, P(css.Class("budget-sub"), uistate.T("budgets.overNear", overCount, nearCount))),
			listBody,
		),
	)
}

type budgetRowProps struct {
	Status         budgeting.Status
	Category       string
	Members        []domain.Member
	Envelope       string        // formatted envelope balance (envelope methodology); "" hides the line
	EnvelopeNeg    bool          // envelope is overdrawn → danger tone
	PaceOver       string        // formatted projected overspend (pace, in-progress only); "" hides the line
	RolloverCarry  string        // formatted previous-period carry for per-budget rollover; "" hides the line
	RolloverNeg    bool          // previous-period carry is negative → danger tone
	CoverSources   []coverSource // budgets that can fund a "Cover…" (the row drops itself)
	CoverShortfall string        // formatted overspend, for the "covers the $X over" hint
	CoverDefault   string        // major-units default amount to prefill the cover field
	OnDelete       func(string)
	OnSave         func(id, name, limit, period, owner string, rollover bool)
	OnCover        func(toID, fromID, amount string) error
	OnDrill        func(categoryID string) // open Transactions filtered to this budget's category
}

// coverSource is one budget offered as a funding source in a row's "Cover…" picker.
type coverSource struct {
	ID    string
	Label string
}

// budgetTitle renders a budget's display title: its name, or its category when
// unnamed, or "name · category" when both add information (never "Food · Food").
func budgetTitle(name, category string) string {
	switch {
	case name == "":
		return category
	case category != "" && !strings.EqualFold(category, name):
		return name + " · " + category
	default:
		return name
	}
}

// periodOptions builds the budget-period <option>s with selected marked.
func periodOptions(selected string) []ui.Node {
	opts := make([]ui.Node, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, Option(Value(string(p)), SelectedIf(selected == string(p)), p.Label()))
	}
	return opts
}

// ownerSelectOptions builds owner <option>s (the shared group plus each member)
// with selected marked — used wherever an entity's owner can be chosen.
func ownerSelectOptions(members []domain.Member, selected string) []ui.Node {
	opts := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(selected == domain.GroupOwnerID), uistate.T("owner.group"))}
	for _, m := range members {
		opts = append(opts, Option(Value(m.ID), SelectedIf(selected == m.ID), m.Name))
	}
	return opts
}

func checkedAttr(checked bool) []any {
	if !checked {
		return nil
	}
	return []any{Attr("checked", "checked")}
}

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status
	limitMajor := money.FormatMinor(s.Budget.Limit.Amount, currency.Decimals(s.Budget.Limit.Currency))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(s.Budget.ID) }))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Budget.CategoryID)
		}
	}))
	editing := ui.UseState(false)
	nameS := ui.UseState(s.Budget.Name)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(s.Budget.Period))
	ownerS := ui.UseState(s.Budget.OwnerID)
	rolloverS := ui.UseState(s.Budget.Rollover)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onPeriod := ui.UseEvent(func(e ui.Event) { periodS.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(s.Budget.Name)
		limitS.Set(limitMajor)
		periodS.Set(string(s.Budget.Period))
		ownerS.Set(s.Budget.OwnerID)
		rolloverS.Set(s.Budget.Rollover)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(s.Budget.ID, nameS.Get(), limitS.Get(), periodS.Get(), ownerS.Get(), rolloverS.Get())
		editing.Set(false)
	}))

	// "Cover…" inline form (L1): move money from another budget to clear an overspend.
	covering := ui.UseState(false)
	coverFrom := ui.UseState("")
	coverAmt := ui.UseState("")
	coverErr := ui.UseState("")
	onCoverFrom := ui.UseEvent(func(e ui.Event) { coverFrom.Set(e.GetValue()) })
	onCoverAmt := ui.UseEvent(func(v string) { coverAmt.Set(v) })
	firstSource := func() string {
		for _, src := range props.CoverSources {
			if src.ID != s.Budget.ID {
				return src.ID
			}
		}
		return ""
	}
	startCover := ui.UseEvent(Prevent(func() {
		coverFrom.Set(firstSource())
		coverAmt.Set(props.CoverDefault)
		coverErr.Set("")
		covering.Set(true)
	}))
	cancelCover := ui.UseEvent(Prevent(func() { covering.Set(false) }))
	fullCover := ui.UseEvent(Prevent(func() { coverAmt.Set(props.CoverDefault) }))
	submitCover := ui.UseEvent(Prevent(func() {
		from := coverFrom.Get()
		if from == "" {
			from = firstSource()
		}
		if err := props.OnCover(s.Budget.ID, from, coverAmt.Get()); err != nil {
			coverErr.Set(err.Error())
			return
		}
		coverErr.Set("")
		covering.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("budget-edit-" + s.Budget.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(css.Class("budget"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("common.name"),
					Input(css.Class("field"), Attr("id", "budget-edit-"+s.Budget.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("budgets.limitLabel"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("budgets.limitLabel")), Value(limitS.Get()), Step("0.01"), OnInput(onLimit))),
				labeledField(uistate.T("budgets.period"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("budgets.period")), Title(uistate.T("budgets.period")), OnChange(onPeriod), periodOptions(periodS.Get()))),
				labeledField(uistate.T("common.owner"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("common.owner")), Title(uistate.T("common.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get()))),
				Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(append([]any{Type("checkbox"), OnChange(onRollover)}, checkedAttr(rolloverS.Get())...)...),
					Span(uistate.T("budgets.rollover")),
				),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	limit, _ := s.Spent.Add(s.Remaining) // limit in base currency

	width := s.Percent
	if width > 100 {
		width = 100
	}
	fillClass := "bar-fill"
	label := uistate.T("budgets.onTrack")
	switch s.State {
	case budgeting.StateNear:
		fillClass = "bar-fill near"
		label = uistate.T("budgets.nearLimit")
	case budgeting.StateOver:
		fillClass = "bar-fill over"
		label = uistate.T("budgets.overBudget")
	}

	// Show "name · category" only when they add information (see budgetTitle).
	title := budgetTitle(s.Budget.Name, props.Category)

	// Envelope methodology: show the carried-forward balance under the period row.
	var envLine ui.Node = Fragment()
	if props.Envelope != "" {
		cls := "budget-sub " + tw.Fold(tw.FontDisplay)
		if props.EnvelopeNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		envLine = Span(ClassStr(cls), uistate.T("budgets.envelopeRow", props.Envelope))
	}

	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if props.PaceOver != "" {
		paceLine = Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.paceOver", props.PaceOver))
	}

	var rolloverLine ui.Node = Fragment()
	if props.RolloverCarry != "" {
		cls := "budget-sub " + tw.Fold(tw.FontDisplay)
		if props.RolloverNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		rolloverLine = Span(ClassStr(cls), uistate.T("budgets.rolloverCarry", props.RolloverCarry))
	}

	// "Cover…" is offered only on an over-budget row that has another budget to
	// pull from. The inline form picks a source and an amount (prefilled to the
	// exact overspend), so Maya can clear the overspend without leaving the screen.
	isOver := s.State == budgeting.StateOver
	hasSource := firstSource() != ""
	var coverBtn ui.Node = Fragment()
	if isOver && hasSource && !covering.Get() {
		coverBtn = Button(css.Class("btn"), Type("button"), Title("Move money from another budget to cover this overspend"), OnClick(startCover), "Cover…")
	}
	var coverForm ui.Node = Fragment()
	if covering.Get() {
		srcOpts := make([]ui.Node, 0, len(props.CoverSources))
		for _, src := range props.CoverSources {
			if src.ID == s.Budget.ID {
				continue
			}
			srcOpts = append(srcOpts, Option(Value(src.ID), SelectedIf(coverFrom.Get() == src.ID), src.Label))
		}
		var coverErrLine ui.Node = Fragment()
		if coverErr.Get() != "" {
			coverErrLine = P(css.Class("budget-sub", tw.TextDown), coverErr.Get())
		}
		coverForm = Div(css.Class("cover-form"),
			Span(css.Class("budget-sub"), "Cover the "+props.CoverShortfall+" over by moving money from another budget:"),
			Form(css.Class("form-grid"), OnSubmit(submitCover),
				Select(css.Class("field"), Attr("aria-label", "Cover from budget"), OnChange(onCoverFrom), srcOpts),
				Input(css.Class("field"), Type("number"), Attr("aria-label", "Amount to move"), Placeholder("Amount"), Value(coverAmt.Get()), Step("0.01"), OnInput(onCoverAmt)),
				Button(css.Class("btn"), Type("button"), Title("Use the full overspend amount"), OnClick(fullCover), "Full "+props.CoverShortfall),
				Button(css.Class("btn btn-primary"), Type("submit"), "Cover"),
				Button(css.Class("btn"), Type("button"), OnClick(cancelCover), uistate.T("action.cancel")),
			),
			coverErrLine,
		)
	}

	return Div(css.Class("budget"),
		Div(css.Class("budget-head"),
			IfElse(s.Budget.CategoryID != "",
				Button(css.Class("row-desc budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
					Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "text-align": "left", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
					title),
				Span(css.Class("row-desc"), title)),
			Span(css.Class("budget-amount"), fmtMoney(s.Spent)+" / "+fmtMoney(limit)),
			coverBtn,
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("budgets.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")), Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		Span(css.Class("budget-sub"), uistate.T("budgets.rowSub", s.Budget.Period.Label(), label, s.Percent, fmtMoney(s.Remaining))),
		paceLine,
		rolloverLine,
		envLine,
		coverForm,
	)
}

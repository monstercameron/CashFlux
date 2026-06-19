//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Budgets shows spend against each budget for the current month, with an add
// form and per-row delete.
func Budgets() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:budgets", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

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
			Custom: customValuesToMap(budgetDefs, customVals.Get()),
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		limit.Set("")
		customVals.Set(map[string]string{})
		errMsg.Set("")
		bump()
	}))

	deleteBudget := func(budgetID string) {
		if err := app.DeleteBudget(budgetID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	saveBudget := func(id, newName, limitStr, periodStr, ownerID string) {
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
			if err := app.PutBudget(b); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}

	var formCard ui.Node
	if len(expenseCats) == 0 {
		formCard = Section(Class("card"), P(Class("empty"), uistate.T("budgets.needCategory")))
	} else {
		catOptions := make([]ui.Node, 0, len(expenseCats))
		for _, c := range expenseCats {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		ownerOptions := ownerSelectOptions(app.Members(), owner.Get())
		formCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("budgets.add")),
			Form(Class("form-grid"), OnSubmit(add),
				Input(append([]any{Class("field"), Attr("id", "budget-add"), Type("text"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("budget-err", errMsg.Get())...)...),
				Select(Class("field"), Attr("aria-label", uistate.T("budgets.categoryLabel")), OnChange(onCat), catOptions),
				Select(Class("field"), Attr("aria-label", uistate.T("common.owner")), OnChange(onOwner), ownerOptions),
				Select(Class("field"), Attr("aria-label", uistate.T("budgets.period")), Title(uistate.T("budgets.period")), OnChange(onPeriod), periodOptions(period.Get())),
				Input(Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("budgets.limitPlaceholder", base)), Value(limit.Get()), Step("0.01"), OnInput(onLimit)),
				MapKeyed(budgetDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
			),
			errText("budget-err", errMsg.Get()),
		)
	}

	budgets := app.Budgets()
	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	viewMonth := periodWin.Get().From
	cats := app.Categories()
	// Each budget is evaluated over its own period window around the viewed date,
	// and a parent-category budget rolls up its sub-categories' spend (D5).
	now := time.Now()
	statuses := make([]budgeting.Status, 0, len(budgets))
	paceOver := map[string]string{} // budgetID → formatted projected overspend (in-progress only)
	for _, b := range budgets {
		bs, be := budgeting.PeriodRange(b.Period, viewMonth, weekStart)
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
		ms, me := budgeting.PeriodRange(domain.PeriodMonthly, viewMonth, weekStart)
		income, _, _ := ledger.PeriodTotals(txns, ms, me, rates)
		toAssign := budgeting.ToAssign(income.Amount, totalLimit)
		switch {
		case toAssign > 0:
			assignBanner = P(Class("budget-sub font-display"), uistate.T("budgets.toAssign", fmtMoney(money.New(toAssign, base))))
		case toAssign == 0:
			assignBanner = P(Class("budget-sub font-display"), uistate.T("budgets.allAssigned"))
		default:
			assignBanner = P(Class("budget-sub font-display text-down"), uistate.T("budgets.overAssigned", fmtMoney(money.New(-toAssign, base))))
		}
	case budgeting.MethodEnvelope:
		assignBanner = P(Class("budget-sub font-display"), uistate.T("budgets.envelopeNote"))
		for _, b := range budgets {
			if av, err := budgeting.EnvelopeAvailable(b, txns, viewMonth, weekStart, rates, categorytree.Descendants(cats, b.CategoryID)); err == nil {
				envAvail[b.ID] = fmtMoney(av)
				envNeg[b.ID] = av.IsNegative()
			}
		}
	}

	var listBody ui.Node
	if len(statuses) == 0 {
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), FocusID: "budget-add"})
	} else {
		rows := MapKeyed(statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], Members: app.Members(), Envelope: envAvail[s.Budget.ID], EnvelopeNeg: envNeg[s.Budget.ID], PaceOver: paceOver[s.Budget.ID], OnDelete: deleteBudget, OnSave: saveBudget})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		formCard,
		If(len(statuses) > 0, Div(Class("stat-grid"),
			stat(uistate.T("budgets.spent"), fmtMoney(money.New(totalSpent, base)), "neg"),
			stat(uistate.T("budgets.budgeted"), fmtMoney(money.New(totalLimit, base)), ""),
			stat(uistate.T("budgets.left"), fmtMoney(money.New(totalLimit-totalSpent, base)), accentFor(money.New(totalLimit-totalSpent, base))),
		)),
		Section(Class("card"),
			Div(Class("budget-head"),
				H2(Class("card-title"), uistate.T("nav.budgets")),
			),
			assignBanner,
			If(overCount > 0 || nearCount > 0, P(Class("budget-sub"), uistate.T("budgets.overNear", overCount, nearCount))),
			listBody,
		),
	)
}

type budgetRowProps struct {
	Status      budgeting.Status
	Category    string
	Members     []domain.Member
	Envelope    string // formatted envelope balance (envelope methodology); "" hides the line
	EnvelopeNeg bool   // envelope is overdrawn → danger tone
	PaceOver    string // formatted projected overspend (pace, in-progress only); "" hides the line
	OnDelete    func(string)
	OnSave      func(id, name, limit, period, owner string)
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

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status
	limitMajor := money.FormatMinor(s.Budget.Limit.Amount, currency.Decimals(s.Budget.Limit.Currency))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(s.Budget.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(s.Budget.Name)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(s.Budget.Period))
	ownerS := ui.UseState(s.Budget.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onPeriod := ui.UseEvent(func(e ui.Event) { periodS.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(s.Budget.Name)
		limitS.Set(limitMajor)
		periodS.Set(string(s.Budget.Period))
		ownerS.Set(s.Budget.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(s.Budget.ID, nameS.Get(), limitS.Get(), periodS.Get(), ownerS.Get())
		editing.Set(false)
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
		return Div(Class("budget"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", "budget-edit-"+s.Budget.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("budgets.limitLabel")), Value(limitS.Get()), Step("0.01"), OnInput(onLimit)),
				Select(Class("field"), Attr("aria-label", uistate.T("budgets.period")), Title(uistate.T("budgets.period")), OnChange(onPeriod), periodOptions(periodS.Get())),
				Select(Class("field"), Attr("aria-label", uistate.T("common.owner")), Title(uistate.T("common.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get())),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
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

	// Show "name · category" only when they add information: an unnamed budget
	// shows just its category, and a budget named after its category ("Food" for
	// the Food category) shows one label, not a redundant "Food · Food".
	title := s.Budget.Name
	switch {
	case title == "":
		title = props.Category
	case props.Category != "" && !strings.EqualFold(props.Category, title):
		title += " · " + props.Category
	}

	// Envelope methodology: show the carried-forward balance under the period row.
	var envLine ui.Node = Fragment()
	if props.Envelope != "" {
		cls := "budget-sub font-display"
		if props.EnvelopeNeg {
			cls += " text-down"
		}
		envLine = Span(Class(cls), uistate.T("budgets.envelopeRow", props.Envelope))
	}

	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if props.PaceOver != "" {
		paceLine = Span(Class("budget-sub text-down"), uistate.T("budgets.paceOver", props.PaceOver))
	}
	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), title),
			Span(Class("budget-amount"), fmtMoney(s.Spent)+" / "+fmtMoney(limit)),
			Button(Class("btn"), Type("button"), Title(uistate.T("budgets.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
			Button(Class("btn-del"), Type("button"), Title(uistate.T("budgets.deleteTitle")), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		Span(Class("budget-sub"), uistate.T("budgets.rowSub", s.Budget.Period.Label(), label, s.Percent, fmtMoney(s.Remaining))),
		paceLine,
		envLine,
	)
}

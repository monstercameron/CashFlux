//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
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

	errMsg := ui.UseState("")
	// Open the add-budget modal from the card header (G4: discoverable add).
	addBudget := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("budget") }))
	// The viewed period comes from the shared top-bar resolution control (C7) —
	// the Budgets card no longer has its own competing month stepper.
	periodWin := uistate.UsePeriod()
	weekStart := uistate.UsePrefs().Get().WeekStartWeekday()

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

	// topupBudget raises a budget's limit by the entered amount so the user can
	// proactively add capacity before overspending (L43).
	topupBudget := func(toID, amountStr string) error {
		amt, err := money.ParseMinor(strings.TrimSpace(amountStr), currency.Decimals(base))
		if err != nil || amt <= 0 {
			return fmt.Errorf("enter an amount greater than zero")
		}
		for _, b := range app.Budgets() {
			if b.ID != toID {
				continue
			}
			b.Limit = money.New(b.Limit.Amount+amt, base)
			if err := app.PutBudget(b); err != nil {
				return err
			}
			bump()
			uistate.PostNotice(uistate.T("budgets.toppedUpToast", fmtMoney(money.New(amt, base))), false)
			return nil
		}
		return fmt.Errorf("budget not found")
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

	// Health-first ordering (G4): problems rise to the top so Renu scans the
	// budgets that need action before the healthy ones — Over → Near/At-risk →
	// On track, then by percent used descending within each tier. "At risk" (the
	// pace projection flags an overspend though the budget isn't Near yet) shares
	// the middle tier so a trending-over budget isn't buried among on-track ones.
	healthRank := func(s budgeting.Status) int {
		switch s.State {
		case budgeting.StateOver:
			return 0
		case budgeting.StateNear:
			return 1
		}
		if paceOver[s.Budget.ID] != "" {
			return 1
		}
		return 2
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		ri, rj := healthRank(statuses[i]), healthRank(statuses[j])
		if ri != rj {
			return ri < rj
		}
		return statuses[i].Percent > statuses[j].Percent
	})

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
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), AddTarget: "budget", Icon: icon.Budgets})
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
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], Members: app.Members(), Envelope: envAvail[s.Budget.ID], EnvelopeNeg: envNeg[s.Budget.ID], PaceOver: paceOver[s.Budget.ID], RolloverCarry: rollCarry[s.Budget.ID], RolloverNeg: rollNeg[s.Budget.ID], CoverSources: coverSources, CoverShortfall: fmtMoney(shortfall), CoverDefault: coverDefault, OnDelete: deleteBudget, OnSave: saveBudget, OnCover: coverBudget, OnTopUp: topupBudget, OnDrill: viewTransactions})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		If(len(statuses) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("budgets.spent"), fmtMoney(money.New(totalSpent, base)), "neg"),
			stat(uistate.T("budgets.budgeted"), fmtMoney(money.New(totalLimit, base)), ""),
			stat(uistate.T("budgets.left"), fmtMoney(money.New(totalLimit-totalSpent, base)), accentFor(money.New(totalLimit-totalSpent, base))),
		)),
		Section(css.Class("card"),
			Div(css.Class("card-head"),
				H2(css.Class("card-title"), uistate.T("nav.budgets")),
				If(len(statuses) > 0, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(addBudget),
					uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("budgets.addBudget")))),
			),
			assignBanner,
			If(overCount > 0 || nearCount > 0, P(css.Class("budget-sub", tw.Flex, tw.ItemsCenter, tw.Gap2),
				If(overCount > 0, Span(css.Class("pill", tw.TextDown), uistate.T("budgets.overBadge", overCount))),
				If(nearCount > 0, Span(css.Class("pill", tw.TextWarn), uistate.T("budgets.nearBadge", nearCount))),
			)),
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
	OnTopUp        func(id, amount string) error // increase this budget's limit by the entered amount
	OnDrill        func(categoryID string)       // open Transactions filtered to this budget's category
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

// periodOptions builds the budget-period SelectOptions.
func periodOptions(selected string) []uiw.SelectOption {
	opts := make([]uiw.SelectOption, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, uiw.SelectOption{Value: string(p), Label: p.Label()})
	}
	return opts
}

// ownerSelectOptions builds owner SelectOptions (the shared group plus each member)
// — used wherever an entity's owner can be chosen.
func ownerSelectOptions(members []domain.Member, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: domain.GroupOwnerID, Label: uistate.T("owner.group")}}
	for _, m := range members {
		opts = append(opts, uiw.SelectOption{Value: m.ID, Label: m.Name})
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
	// onPeriod/onOwner hooks kept for stable hook ordering; SelectInput owns the
	// change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { periodS.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
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
	// onCoverFrom hook kept for stable hook ordering; SelectInput owns the change event.
	ui.UseEvent(func(e ui.Event) { coverFrom.Set(e.GetValue()) })
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

	// "Top up…" inline form (L43): increase this budget's limit by a chosen amount,
	// available on budgets that are not already over (proactive capacity add).
	toppingUp := ui.UseState(false)
	topupAmt := ui.UseState("")
	topupErr := ui.UseState("")
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	startTopup := ui.UseEvent(Prevent(func() {
		topupAmt.Set("")
		topupErr.Set("")
		toppingUp.Set(true)
	}))
	cancelTopup := ui.UseEvent(Prevent(func() { toppingUp.Set(false) }))
	submitTopup := ui.UseEvent(Prevent(func() {
		if err := props.OnTopUp(s.Budget.ID, topupAmt.Get()); err != nil {
			topupErr.Set(err.Error())
			return
		}
		topupErr.Set("")
		toppingUp.Set(false)
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
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   periodOptions(periodS.Get()),
						Selected:  periodS.Get(),
						OnChange:  func(v string) { periodS.Set(v) },
						AriaLabel: uistate.T("budgets.period"),
					})),
				labeledField(uistate.T("common.owner"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   ownerSelectOptions(props.Members, ownerS.Get()),
						Selected:  ownerS.Get(),
						OnChange:  func(v string) { ownerS.Set(v) },
						AriaLabel: uistate.T("common.owner"),
					})),
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
	default:
		// Not over/near yet, but the pace projection says this budget is trending
		// to overspend — don't claim "On track" while also warning of an overspend
		// (the L35 contradiction). Call it "At risk" instead.
		if props.PaceOver != "" {
			fillClass = "bar-fill near"
			label = uistate.T("budgets.atRisk")
		}
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

	// "Top up…" is offered on budgets that are not over, letting the user raise
	// the limit proactively before they hit the ceiling (L43).
	var topupBtn ui.Node = Fragment()
	if !isOver && props.OnTopUp != nil && !toppingUp.Get() && !covering.Get() {
		topupBtn = Button(css.Class("btn"), Type("button"), Title("Increase this budget's limit for the current period"), OnClick(startTopup), "Top up…")
	}
	var topupForm ui.Node = Fragment()
	if toppingUp.Get() {
		var topupErrLine ui.Node = Fragment()
		if topupErr.Get() != "" {
			topupErrLine = P(css.Class("budget-sub", tw.TextDown), topupErr.Get())
		}
		topupForm = Div(css.Class("cover-form"),
			Span(css.Class("budget-sub"), "Increase this budget's limit by:"),
			Form(css.Class("form-grid"), OnSubmit(submitTopup),
				Input(css.Class("field"), Type("number"), Attr("aria-label", "Amount to add"), Placeholder("Amount"), Value(topupAmt.Get()), Step("0.01"), OnInput(onTopupAmt)),
				Button(css.Class("btn btn-primary"), Type("submit"), "Add funds"),
				Button(css.Class("btn"), Type("button"), OnClick(cancelTopup), uistate.T("action.cancel")),
			),
			topupErrLine,
		)
	}
	var coverForm ui.Node = Fragment()
	if covering.Get() {
		srcOpts := make([]uiw.SelectOption, 0, len(props.CoverSources))
		for _, src := range props.CoverSources {
			if src.ID == s.Budget.ID {
				continue
			}
			srcOpts = append(srcOpts, uiw.SelectOption{Value: src.ID, Label: src.Label})
		}
		var coverErrLine ui.Node = Fragment()
		if coverErr.Get() != "" {
			coverErrLine = P(css.Class("budget-sub", tw.TextDown), coverErr.Get())
		}
		coverForm = Div(css.Class("cover-form"),
			Span(css.Class("budget-sub"), "Cover the "+props.CoverShortfall+" over by moving money from another budget:"),
			Form(css.Class("form-grid"), OnSubmit(submitCover),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   srcOpts,
					Selected:  coverFrom.Get(),
					OnChange:  func(v string) { coverFrom.Set(v) },
					AriaLabel: "Cover from budget",
				}),
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
			topupBtn,
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("budgets.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")), Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		// Sub-line split (G4 §8): the primary line carries the at-a-glance signal —
		// health status + money left; the period and percent drop to a dimmer
		// secondary line so they read as low-signal context, not equal weight.
		Span(css.Class("budget-sub"), uistate.T("budgets.rowPrimary", label, fmtMoney(s.Remaining))),
		Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.rowSecondary", s.Budget.Period.Label(), width)),
		paceLine,
		rolloverLine,
		envLine,
		coverForm,
		topupForm,
	)
}

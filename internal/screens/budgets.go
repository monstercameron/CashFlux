// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
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
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:budgets", 0)
	bump := func() { rev.Set(rev.Get() + 1) }
	// C120: also re-render when the global dataset changes (a transaction added via
	// Quick-Add anywhere bumps this), so the budget bars/spent figures update live
	// instead of only on budget CRUD or a reload.
	_ = uistate.UseDataRevision().Get()

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
	// C112: switch the budgeting methodology (standard / zero-based / envelope) right
	// from /budgets — previously only reachable buried in global Settings, so the
	// zero-based + envelope views were effectively inaccessible to most users.
	onMethod := ui.UseEvent(func(e ui.Event) {
		s := app.Settings()
		s.BudgetMethodology = e.GetValue()
		_ = app.PutSettings(s)
		bump()
	})
	// C114: one-click 50/30/20 starter template. Uses last full calendar month's
	// income as the base, generates a per-category proposal via the tested
	// budgeting.Generate5030, and creates a monthly budget for each proposed category
	// that doesn't already have one (never clobbers an existing budget).
	apply503020 := ui.UseEvent(Prevent(func() {
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		now := time.Now()
		curStart := dateutil.MonthStart(now)
		prevStart := dateutil.AddMonths(curStart, -1)
		income := budgeting.IncomeForBudgets(0, txns, prevStart, curStart, base, rates)
		if income <= 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNoIncome"), true)
			return
		}
		res := budgeting.Generate5030(income, app.Categories(), txns, now)
		existing := map[string]bool{}
		for _, b := range app.Budgets() {
			existing[b.CategoryID] = true
		}
		n := 0
		for _, prop := range res.Proposals {
			if prop.LimitMinor <= 0 || existing[prop.Category.ID] {
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
		bump()
		uistate.PostNotice(uistate.T("budgets.tmplApplied", plural(n, "budget")), false)
	}))
	// The viewed period comes from the shared top-bar resolution control (C7) —
	// the Budgets card no longer has its own competing month stepper.
	periodWin := uistate.UsePeriod()
	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()
	// C128: parse the pay-cycle anchor from prefs. When set, biweekly budget
	// periods snap to the user's actual payday instead of the internal epoch.
	var payCycleAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, err := time.Parse("2006-01-02", pr.PayCycleAnchor); err == nil {
			payCycleAnchor = t
		}
	}

	deleteBudget := func(budgetID string) {
		// Guard the destructive delete with a confirm (matches the transactions delete
		// pattern). Previously the "×" deleted a budget instantly with no confirm or undo —
		// a single misclick was unrecoverable.
		name := uistate.T("budgets.thisBudget")
		for _, b := range app.Budgets() {
			if b.ID == budgetID {
				if n := catName[b.CategoryID]; n != "" {
					name = n
				}
				break
			}
		}
		uistate.ConfirmModal(uistate.T("budgets.deleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteBudget(budgetID); err != nil {
				errMsg.Set(err.Error())
				return
			}
			bump()
		})
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
	paceOver := map[string]string{}   // budgetID → formatted projected overspend (in-progress only)
	rollCarry := map[string]string{}  // budgetID → formatted previous-period carry
	rollNeg := map[string]bool{}      // budgetID → whether the previous-period carry is negative
	rollEffCap := map[string]string{} // budgetID → formatted effective cap (C136, rollover budgets only)
	for _, b := range budgets {
		// C128: route biweekly budgets through PeriodRangeAnchored so the grid
		// snaps to the user's actual pay cycle when PayCycleAnchor is set; all
		// other periods fall back to PeriodRange unchanged.
		bs, be := budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payCycleAnchor)
		// Rollover (C132): carry the previous period's remaining (negative when it
		// was overspent) into this period's effective limit so Remaining/Percent/
		// State/bar reflect the carry. Carryover() was never applied before, leaving
		// rollover purely decorative. The badge shows the carried amount, which is
		// exactly effectiveLimit − limit = prev.Remaining.
		eval := b
		if b.Rollover {
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, perr := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); perr == nil {
				if eff, cerr := budgeting.Carryover(prev.Remaining, b.Limit); cerr == nil {
					eval.Limit = eff
					// C136: the effective cap is the carry-in limit (base + previous carry).
					// Show it only when it differs from the base limit (i.e. there was a
					// non-zero carry), so the note appears as soon as rollover has an effect.
					if eff.Amount != b.Limit.Amount {
						rollEffCap[b.ID] = fmtMoney(eff)
					}
				}
				rollCarry[b.ID] = budgetRemainPhrase(prev.Remaining) // C124: "$90.00 over" not "($90.00)"
				rollNeg[b.ID] = prev.Remaining.IsNegative()
			}
		}
		st, err := budgeting.EvaluateRollup(eval, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
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
	var totalSpent, totalLimit, totalOver int64
	for _, s := range statuses {
		switch s.State {
		case budgeting.StateOver:
			overCount++
			if s.Remaining.IsNegative() { // C125: accumulate the overspend across all over budgets
				totalOver += -s.Remaining.Amount
			}
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
	case budgeting.MethodSimple:
		// C119: surface income context in simple mode so the user can see how their
		// budgets relate to what they actually earn. Use the same period-income helper
		// as zero-based (ledger.PeriodTotals over the current month) — the simple mode
		// just doesn't enforce "every dollar assigned"; it still helps to see the gap.
		sms, sme := budgeting.PeriodRange(domain.PeriodMonthly, anchor, weekStart)
		simpleIncome, _, _ := ledger.PeriodTotals(txns, sms, sme, rates)
		simpleUnbudgeted := simpleIncome.Amount - totalLimit
		var simpleDiffNode ui.Node
		if simpleUnbudgeted > 0 {
			simpleDiffNode = Span(css.Class(tw.TextUp), uistate.T("budgets.simpleUnbudgeted", fmtMoney(money.New(simpleUnbudgeted, base))))
		} else if simpleUnbudgeted == 0 {
			simpleDiffNode = Span(uistate.T("budgets.simpleFullyAllocated"))
		} else {
			simpleDiffNode = Span(css.Class(tw.TextDown), uistate.T("budgets.simpleOverAllocated", fmtMoney(money.New(-simpleUnbudgeted, base))))
		}
		assignBanner = P(css.Class("budget-sub", tw.FontDisplay),
			uistate.T("budgets.simpleIncome", fmtMoney(money.New(simpleIncome.Amount, base))),
			" · ",
			uistate.T("budgets.simpleBudgeted", fmtMoney(money.New(totalLimit, base))),
			" · ",
			simpleDiffNode,
		)
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
				// C124/C137 family: an overdrawn envelope reads "$X overdrawn", not the
				// ambiguous accounting parens "($X)".
				if av.IsNegative() {
					envAvail[b.ID] = fmtMoney(av.Abs()) + " " + uistate.T("budgets.overdrawnWord")
				} else {
					envAvail[b.ID] = fmtMoney(av)
				}
				envNeg[b.ID] = av.IsNegative()
			}
		}
	}

	// C190: sum the monthly set-aside across all active sinking-fund goals.
	// FundSetAsideMinor is the canonical per-goal figure; summing over goals
	// where IsSinkingFund && !Archived gives the household's total monthly
	// commitment to funds. Surfaced only when the total is non-zero.
	var totalFundSetAside int64
	for _, g := range app.Goals() {
		if g.IsSinkingFund && !g.Archived {
			totalFundSetAside += goalsvc.FundSetAsideMinor(g, now)
		}
	}
	fundSetAsideNode := Fragment()
	if totalFundSetAside > 0 {
		fundSetAsideNode = P(css.Class("budget-sub", tw.FontDisplay),
			Attr("data-testid", "budgets-fund-setaside"),
			uistate.T("budgets.fundSetAside", fmtMoney(money.New(totalFundSetAside, base))),
		)
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
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], Members: app.Members(), Envelope: envAvail[s.Budget.ID], EnvelopeNeg: envNeg[s.Budget.ID], PaceOver: paceOver[s.Budget.ID], RolloverCarry: rollCarry[s.Budget.ID], RolloverNeg: rollNeg[s.Budget.ID], EffectiveCap: rollEffCap[s.Budget.ID], CoverSources: coverSources, CoverShortfall: fmtMoney(shortfall), CoverDefault: coverDefault, OnDelete: deleteBudget, OnSave: saveBudget, OnCover: coverBudget, OnTopUp: topupBudget, OnDrill: viewTransactions})
			},
		)
		listBody = Div(rows)
	}

	smartSettings := uistate.LoadSmartSettings()
	return Div(
		If(len(statuses) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("budgets.spent"), fmtMoney(money.New(totalSpent, base)), "neg"),
			stat(uistate.T("budgets.budgeted"), fmtMoney(money.New(totalLimit, base)), ""),
			// "Left" (safe-to-spend) is the key budget figure — annotated with a smart
			// explainer tooltip so users understand what it means at a glance.
			Div(css.Class("stat"),
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("budgets.left"),
					smartTooltipFor(smartSettings, "budget-safe", uistate.T("budgets.left"), uistate.T("smart.tipBudgetSafe")),
				),
				Div(ClassStr("stat-value is-hero "+accentFor(money.New(totalLimit-totalSpent, base))), budgetLeftValue(money.New(totalLimit-totalSpent, base))),
			),
		)),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.budgets"),
			HeaderAction: Fragment(
				smartSectionAction(smartSettings),
				// C112: in-context budget-method picker (standard / zero-based / envelope).
				Select(css.Class("field", "set-input"), Attr("data-testid", "budgets-method"),
					Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(onMethod),
					Option(Value(string(budgeting.MethodSimple)), SelectedIf(method == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
					Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(method == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
					Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(method == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
				),
				// C114: one-click 50/30/20 starter template.
				Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "budgets-template-503020"),
					Title(uistate.T("budgets.tmplTitle")), OnClick(apply503020), uistate.T("budgets.tmpl503020")),
				If(len(statuses) > 0, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(addBudget),
					uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("budgets.addBudget")))),
			),
			Body: Fragment(
				// C130: when the top-bar date range is in custom-range mode (not a single
				// budget period), clarify that it changes the view window only — it doesn't
				// redefine each budget's own period.
				If(!periodWin.Get().IsSinglePeriod(),
					P(css.Class("muted"), Attr("data-testid", "budgets-custom-range-hint"),
						uistate.T("budgets.customRangeHint"))),
				assignBanner,
				// C190: sinking-fund monthly set-aside summary — shown when at least one
				// active fund has a non-zero monthly contribution. Placed after the income
				// / methodology banner so the reader sees income context first, then the
				// committed fund savings as a committed slice of that income.
				fundSetAsideNode,
				// C125: when budgets are over, lead with a salient alert banner stating the
				// total overspend up front — not just a small count pill — so the problem
				// is impossible to miss. The count/near pills stay below as detail.
				If(overCount > 0, Div(css.Class("card-alert", "budget-over-banner", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Attr("role", "status"), Attr("data-testid", "budgets-over-banner"),
					Span(css.Class("budget-over-icon"), Attr("aria-hidden", "true"), "⚠"),
					Span(css.Class("budget-over-text"),
						uistate.T("budgets.overBanner", overCount, fmtMoney(money.New(totalOver, base)))),
				)),
				If(overCount > 0 || nearCount > 0, P(css.Class("budget-sub", tw.Flex, tw.ItemsCenter, tw.Gap2),
					If(overCount > 0, Span(css.Class("pill is-danger"), uistate.T("budgets.overBadge", overCount))),
					If(nearCount > 0, Span(css.Class("pill is-warn"), uistate.T("budgets.nearBadge", nearCount))),
				)),
				listBody,
			),
		}),
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
	EffectiveCap   string        // C136: formatted effective cap for this period on rollover budgets; "" = not rollover
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
// budgetLeftValue formats a budget's remaining amount for the summary "Left" stat
// (C124): a positive remaining shows plainly ("$50.00"), but an overspend reads as
// "$50.00 over" instead of the ambiguous accounting parens "($50.00)" — clearer in
// a budgeting context where a minus is an overspend, not an accounting credit.
func budgetLeftValue(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m)
}

// budgetRemainPhrase is budgetLeftValue plus the trailing "left"/"over" word, for
// the per-row summary line ("$50.00 left" / "$50.00 over"). (C124)
func budgetRemainPhrase(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m) + " " + uistate.T("budgets.leftWord")
}

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
// periodLabel localizes a budget period for the UI (the domain layer's
// Period.Label() is hardcoded English by design — domain stays i18n-free). C116.
func periodLabel(p domain.Period) string {
	switch p {
	case domain.PeriodWeekly:
		return uistate.T("budgets.periodWeekly")
	case domain.PeriodBiweekly:
		return uistate.T("budgets.periodBiweekly")
	case domain.PeriodSemimonthly:
		return uistate.T("budgets.periodSemimonthly")
	case domain.PeriodQuarterly:
		return uistate.T("budgets.periodQuarterly")
	case domain.PeriodYearly:
		return uistate.T("budgets.periodYearly")
	default:
		return uistate.T("budgets.periodMonthly")
	}
}

func periodOptions(selected string) []uiw.SelectOption {
	opts := make([]uiw.SelectOption, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, uiw.SelectOption{Value: string(p), Label: periodLabel(p)})
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

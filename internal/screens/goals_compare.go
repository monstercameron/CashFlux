// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalcompare"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goaltrajectory"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// GoalCompareProps drives the goal-vs-goal compare form rendered inside the
// shell-root flip modal (GoalCompareHost). OnDone closes it.
type GoalCompareProps struct {
	App    *appstate.App
	OnDone func()
}

// goalCompareStats is one goal's comparable figures, all derived from the same
// services the goal card itself uses — so the comparison can never disagree
// with the cards.
type goalCompareStats struct {
	target, covered, toGo money.Money
	monthly               money.Money
	hasMonthly            bool
	projected             time.Time
	reachable             bool
	deadline              time.Time
	priority              int
}

func computeGoalCompareStats(g domain.Goal, now time.Time) goalCompareStats {
	cur := g.TargetAmount.Currency
	covered := goalsvc.CoverageMinor(g)
	st := goalCompareStats{
		target:   g.TargetAmount,
		covered:  money.New(covered, cur),
		toGo:     money.New(max(g.TargetAmount.Amount-covered, 0), cur),
		deadline: g.TargetDate,
		priority: g.Priority,
	}
	if m, ok, _ := goalsvc.MonthlyAssignment(g, now); ok && m.Amount > 0 {
		st.monthly, st.hasMonthly = m, true
		res := goaltrajectory.Project(goaltrajectory.Input{
			CurrentMinor: covered, TargetMinor: g.TargetAmount.Amount,
			MonthlyMinor: m.Amount, Start: now, TargetDate: g.TargetDate,
		})
		st.projected, st.reachable = res.ProjectedDate, res.Reachable
	}
	return st
}

// GoalCompareForm is the goal-vs-goal comparison: pick two financial goals and
// read their figures side by side — target, saved + set aside, to go, monthly
// plan, projected landing, deadline, and priority.
func GoalCompareForm(props GoalCompareProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	aS := ui.UseState("")
	bS := ui.UseState("")

	// C398: the picker used to apply this rule inline and say nothing about it, so
	// a reader who expected four goals and saw two had no way to learn why. The
	// rule is now named in goalcompare, and every exclusion carries a reason the
	// note below the picker can state.
	var goals []domain.Goal
	var excluded []goalcompare.Excluded
	if app != nil {
		goals, excluded = goalcompare.Partition(app.Goals())
	}
	byID := func(id string) (domain.Goal, bool) {
		for _, g := range goals {
			if g.ID == id {
				return g, true
			}
		}
		return domain.Goal{}, false
	}
	opts := func(excludeID string) []uiw.SelectOption {
		out := []uiw.SelectOption{{Value: "", Label: uistate.T("goalcompare.pick")}}
		for _, g := range goals {
			if g.ID != excludeID {
				out = append(out, uiw.SelectOption{Value: g.ID, Label: g.Name})
			}
		}
		return out
	}

	now := time.Now()
	ga, okA := byID(aS.Get())
	gb, okB := byID(bS.Get())

	var table ui.Node = P(css.Class("muted"), Attr("data-testid", "goal-compare-empty"),
		uistate.T("goalcompare.empty"))
	var verdict ui.Node = Fragment()
	if okA && okB {
		sa, sb := computeGoalCompareStats(ga, now), computeGoalCompareStats(gb, now)
		// One honest verdict sentence above the table, derived only from the figures the
		// table shows (projected landings + monthly plans). The monthly gap is compared
		// only when both goals share a currency (a cross-currency gap is meaningless).
		cin := goaltrajectory.CompareInput{
			AProjected: sa.projected, AReachable: sa.reachable,
			BProjected: sb.projected, BReachable: sb.reachable,
		}
		if sa.hasMonthly && sb.hasMonthly && sa.monthly.Currency == sb.monthly.Currency {
			cin.AMonthlyMinor, cin.BMonthlyMinor, cin.MonthlyKnown = sa.monthly.Amount, sb.monthly.Amount, true
		}
		if cmp := goaltrajectory.Compare(cin); cmp.Meaningful() {
			verdict = P(css.Class("goal-compare-verdict"), Attr("data-testid", "goal-compare-verdict"),
				goalCompareVerdict(cmp, ga.Name, gb.Name, sa.monthly.Currency))
		}
		dash := uistate.T("goalcompare.none")
		fmtDate := func(t time.Time, ok bool) string {
			if !ok || t.IsZero() {
				return dash
			}
			return t.Format("Jan 2006")
		}
		fmtPriority := func(p int) string {
			if p <= 0 {
				return dash
			}
			return uistate.T(goalPriorityKey(p))
		}
		fmtMonthly := func(s goalCompareStats) string {
			if !s.hasMonthly {
				return dash
			}
			return fmtMoney(s.monthly)
		}
		row := func(labelKey, va, vb string) ui.Node {
			return Tr(Td(css.Class("t-caption"), uistate.T(labelKey)), Td(va), Td(vb))
		}
		table = Table(css.Class("goal-compare-table"), Attr("data-testid", "goal-compare-table"),
			Style(map[string]string{"width": "100%", "border-collapse": "collapse"}),
			Thead(Tr(Th(""), Th(ga.Name), Th(gb.Name))),
			Tbody(
				row("goalcompare.target", fmtMoney(sa.target), fmtMoney(sb.target)),
				row("goalcompare.covered", fmtMoney(sa.covered), fmtMoney(sb.covered)),
				row("goalcompare.toGo", fmtMoney(sa.toGo), fmtMoney(sb.toGo)),
				row("goalcompare.monthly", fmtMonthly(sa), fmtMonthly(sb)),
				row("goalcompare.projected", fmtDate(sa.projected, sa.reachable), fmtDate(sb.projected, sb.reachable)),
				row("goalcompare.deadline", fmtDate(sa.deadline, true), fmtDate(sb.deadline, true)),
				row("goalcompare.priority", fmtPriority(sa.priority), fmtPriority(sb.priority)),
			),
		)
	}

	// C398: the figures show what each goal costs; they do not show the trade. With
	// one pot and two goals, choosing who goes first moves BOTH dates, and that is
	// the decision this surface exists to support. The pot is the money already
	// committed to these two — no new number to explain, and the question becomes
	// "same money, different order", which is the one the user can actually act on.
	var orderPanel ui.Node = Fragment()
	if okA && okB {
		sa, sb := computeGoalCompareStats(ga, now), computeGoalCompareStats(gb, now)
		switch {
		case !sa.hasMonthly || !sb.hasMonthly:
			orderPanel = P(css.Class("muted"), Attr("data-testid", "goal-compare-order-none"),
				uistate.T("goalcompare.orderNeedsPlans"))
		case sa.monthly.Currency != sb.monthly.Currency:
			// Pooling two currencies would produce a number that is not money.
			orderPanel = P(css.Class("muted"), Attr("data-testid", "goal-compare-order-none"),
				uistate.T("goalcompare.orderNeedsOneCurrency"))
		default:
			pool := sa.monthly.Amount + sb.monthly.Amount
			swap := goalcompare.Compare(pool,
				goalcompare.Fund{RemainingMinor: sa.toGo.Amount, MonthlyCapMinor: sa.monthly.Amount},
				goalcompare.Fund{RemainingMinor: sb.toGo.Amount, MonthlyCapMinor: sb.monthly.Amount})
			if !swap.Matters() {
				// Both plans fit inside the pot, so order changes nothing. Saying so
				// is the useful answer; a table of identical numbers is not.
				orderPanel = P(css.Class("muted"), Attr("data-testid", "goal-compare-order-moot"),
					uistate.T("goalcompare.orderMoot"))
			} else {
				orderPanel = Div(css.Class("goal-order-panel"), Attr("data-testid", "goal-compare-order"),
					H4(css.Class("t-caption"), uistate.T("goalcompare.orderTitle")),
					P(css.Class("muted"), uistate.T("goalcompare.orderLede",
						fmtMoney(money.New(pool, sa.monthly.Currency)))),
					Table(css.Class("goal-compare-table"),
						Style(map[string]string{"width": "100%", "border-collapse": "collapse"}),
						Thead(Tr(Th(""), Th(ga.Name), Th(gb.Name))),
						Tbody(
							Tr(Td(css.Class("t-caption"), uistate.T("goalcompare.orderAFirst", ga.Name)),
								Td(Attr("data-testid", "goal-order-a-first-a"), landingLabel(swap.AFirst.First)),
								Td(Attr("data-testid", "goal-order-a-first-b"), landingLabel(swap.AFirst.Second))),
							Tr(Td(css.Class("t-caption"), uistate.T("goalcompare.orderBFirst", gb.Name)),
								Td(Attr("data-testid", "goal-order-b-first-a"), landingLabel(swap.BFirst.Second)),
								Td(Attr("data-testid", "goal-order-b-first-b"), landingLabel(swap.BFirst.First))),
						),
					),
				)
			}
		}
	}

	// The eligibility note. It states the rule unconditionally — a reader seeing a
	// short list needs the rule whether or not anything happened to be excluded —
	// and adds the count of what was left out when there is any.
	eligNote := P(css.Class("muted"), Attr("data-testid", "goal-compare-eligibility"),
		uistate.T("goalcompare.eligibleRule"))
	if len(excluded) > 0 {
		eligNote = P(css.Class("muted"), Attr("data-testid", "goal-compare-eligibility"),
			uistate.T("goalcompare.eligibleRule")+" "+goalExcludedNote(excluded))
	}

	return Div(css.Class("acct-edit-form"), Attr("data-testid", "goal-compare-form"),
		Div(css.Class("modal-scroll"),
			Div(Style(map[string]string{"display": "flex", "gap": "0.75rem", "flex-wrap": "wrap"}),
				Div(Style(map[string]string{"flex": "1 1 12rem"}),
					labeledField(uistate.T("goalcompare.goalA"),
						uiw.SelectInput(uiw.SelectInputProps{Options: opts(bS.Get()), Selected: aS.Get(),
							TestID: "goal-compare-a", OnChange: func(v string) { aS.Set(v) },
							AriaLabel: uistate.T("goalcompare.goalA")}))),
				Div(Style(map[string]string{"flex": "1 1 12rem"}),
					labeledField(uistate.T("goalcompare.goalB"),
						uiw.SelectInput(uiw.SelectInputProps{Options: opts(aS.Get()), Selected: bS.Get(),
							TestID: "goal-compare-b", OnChange: func(v string) { bS.Set(v) },
							AriaLabel: uistate.T("goalcompare.goalB")}))),
			),
			eligNote,
			verdict,
			table,
			orderPanel,
		),
	)
}

// landingLabel renders one funding-order outcome. An unreached goal is NOT
// rendered as its month cap: the simulation stopped, it did not finish, and
// printing "600 months" would be a fabricated date rather than an honest
// "this order never gets there".
func landingLabel(l goalcompare.Landing) string {
	switch {
	case !l.Reached:
		return uistate.T("goalcompare.orderNever")
	case l.Months == 0:
		return uistate.T("goalcompare.orderAlready")
	case l.Months == 1:
		return uistate.T("goalcompare.orderOneMonth")
	}
	return uistate.T("goalcompare.orderMonths", l.Months)
}

// goalExcludedNote summarizes what the picker left out, grouped by reason, so
// the count and the cause arrive together.
func goalExcludedNote(ex []goalcompare.Excluded) string {
	var archived, notFinancial, noTarget int
	for _, e := range ex {
		switch e.Reason {
		case goalcompare.ReasonArchived:
			archived++
		case goalcompare.ReasonNotFinancial:
			notFinancial++
		case goalcompare.ReasonNoTarget:
			noTarget++
		}
	}
	var parts []string
	if notFinancial > 0 {
		parts = append(parts, uistate.T("goalcompare.exclNotFinancial", notFinancial))
	}
	if noTarget > 0 {
		parts = append(parts, uistate.T("goalcompare.exclNoTarget", noTarget))
	}
	if archived > 0 {
		parts = append(parts, uistate.T("goalcompare.exclArchived", archived))
	}
	if len(parts) == 0 {
		return ""
	}
	return uistate.T("goalcompare.exclLead", strings.Join(parts, ", "))
}

// goalCompareVerdict renders the one-sentence Compare verdict from the computed
// Comparison. Every branch draws only on figures already shown in the table, so the
// sentence can never disagree with the rows. gapCurrency formats the monthly gap (both
// goals share it whenever a monthly gap exists).
func goalCompareVerdict(c goaltrajectory.Comparison, aName, bName, gapCurrency string) string {
	nameOf := func(s goaltrajectory.CompareSide) string {
		if s == goaltrajectory.SideA {
			return aName
		}
		return bName
	}
	otherOf := func(s goaltrajectory.CompareSide) string {
		if s == goaltrajectory.SideA {
			return bName
		}
		return aName
	}
	dur := func(n int) string {
		if n == 1 {
			return uistate.T("goalcompare.vMonthsOne")
		}
		return uistate.T("goalcompare.vMonths", n)
	}
	gap := ""
	if c.Costlier != goaltrajectory.SideNone {
		gap = fmtMoney(money.New(c.MonthlyGapMinor, gapCurrency))
	}

	switch {
	case c.Sooner != goaltrajectory.SideNone:
		sooner, other, d := nameOf(c.Sooner), otherOf(c.Sooner), dur(c.MonthsApart)
		switch {
		case c.Costlier == c.Sooner:
			// The faster goal is also the more expensive — a trade-off.
			return uistate.T("goalcompare.vLeadSoonerCostMore", sooner, d, other, gap)
		case c.Costlier != goaltrajectory.SideNone:
			// The faster goal is the cheaper one — a clean win.
			return uistate.T("goalcompare.vLeadSoonerCostLess", sooner, d, other, gap)
		default:
			return uistate.T("goalcompare.vLeadSoonerOnly", sooner, d, other)
		}
	case c.SameTiming:
		if c.Costlier != goaltrajectory.SideNone {
			return uistate.T("goalcompare.vLeadSameCost", aName, bName, nameOf(c.Costlier), gap)
		}
		return uistate.T("goalcompare.vLeadSameOnly", aName, bName)
	default: // only the monthly plans are comparable
		return uistate.T("goalcompare.vLeadCostOnly", nameOf(c.Costlier), gap, otherOf(c.Costlier))
	}
}

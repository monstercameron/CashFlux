// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/emergencyfund"
	"github.com/monstercameron/CashFlux/internal/goalinterest"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// --- GL4: contribution slider with live ETA --------------------------------------

type goalSliderProps struct {
	App  *appstate.App
	Goal domain.Goal
	Base string
}

// GoalContribSlider is the per-goal contribution slider (GL4): drag the monthly
// amount and the projected finish date updates live; "Use this plan" persists the
// contribution. It is its own component so its state and event hooks sit at stable
// render positions (never inside the goal-list loop). It renders nothing for goals
// with no remaining balance or no sensible range to explore.
func GoalContribSlider(props goalSliderProps) ui.Node {
	return ui.CreateElement(goalContribSlider, props)
}

func goalContribSlider(props goalSliderProps) ui.Node {
	g := props.Goal
	if g.EffectiveKind() != domain.GoalKindFinancial || g.Archived {
		return Fragment()
	}
	now := time.Now()
	min, max, step, ok := goalsvc.SliderRange(g, now)
	if !ok {
		return Fragment()
	}

	// Seed the slider from the goal's explicit monthly, else its needed pace, else
	// the range midpoint — a realistic starting position the user can drag from.
	seed := g.MonthlyContribution.Amount
	if seed <= 0 {
		if needed, has, err := goalsvc.MonthlyNeeded(g, now); err == nil && has {
			seed = needed.Amount
		}
	}
	if seed < min.Amount {
		seed = min.Amount
	}
	if seed > max.Amount {
		seed = max.Amount
	}
	cur := ui.UseState(seed)
	dec := currency.Decimals(g.TargetAmount.Currency)
	// QA #51: the slider must never be the only precise input — a numeric $/mo
	// field sits beside it. draft holds the field's raw text so typing isn't
	// fought by reformatting; the slider keeps it in sync when dragged.
	draft := ui.UseState(money.FormatMinor(seed, dec))

	clampRange := func(v int64) int64 {
		if v < min.Amount {
			return min.Amount
		}
		if v > max.Amount {
			return max.Amount
		}
		return v
	}
	onSlide := ui.UseEvent(func(e ui.Event) {
		if v, err := strconv.ParseInt(e.GetValue(), 10, 64); err == nil {
			cur.Set(v)
			draft.Set(money.FormatMinor(v, dec))
		}
	})
	// QA #51: explicit keyboard support — the framework's value re-set could
	// swallow native range stepping, and Home/End/Page keys were dead. Handled
	// here with PreventDefault so a step never double-applies.
	onSlideKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		st := step.Amount
		var next int64
		switch e.GetKey() {
		case "ArrowLeft", "ArrowDown":
			next = clampRange(cur.Get() - st)
		case "ArrowRight", "ArrowUp":
			next = clampRange(cur.Get() + st)
		case "PageDown":
			next = clampRange(cur.Get() - 5*st)
		case "PageUp":
			next = clampRange(cur.Get() + 5*st)
		case "Home":
			next = min.Amount
		case "End":
			next = max.Amount
		default:
			return
		}
		e.PreventDefault()
		cur.Set(next)
		draft.Set(money.FormatMinor(next, dec))
	})
	// Direct numeric entry: parse as it's typed (partial text keeps the last
	// valid amount), clamp into the slider's range on commit.
	onDraft := ui.UseEvent(func(v string) {
		draft.Set(v)
		if minor, err := money.ParseMinor(strings.TrimSpace(v), dec); err == nil && minor > 0 {
			cur.Set(clampRange(minor))
		}
	})
	onDraftBlur := ui.UseEvent(func(e ui.Event) {
		draft.Set(money.FormatMinor(cur.Get(), dec))
	})
	nav := router.UseNavigate()
	useThis := ui.UseEvent(Prevent(func() {
		ng := g
		ng.MonthlyContribution = money.New(cur.Get(), g.TargetAmount.Currency)
		if err := props.App.PutGoal(ng); err != nil {
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("goals.planSavedToast", fmtMoney(ng.MonthlyContribution)), false)
	}))
	whereFrom := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/budgets")) }))

	// Coordinate with GL2: when the goal draws on an APY-bearing account, project
	// the finish with monthly compounding (goalinterest); else fall back to the
	// linear slider point. On-track is judged against the goal's target date.
	finish, hasFinish := planFinish(props.App, g, cur.Get(), now)
	onTrack := hasFinish && !g.TargetDate.IsZero() && !finish.After(g.TargetDate)
	var readout ui.Node
	switch {
	case !hasFinish:
		readout = Span(css.Class(tw.TextDim), uistate.T("goals.planNoFinish"))
	case onTrack:
		readout = Span(uistate.T("goals.planFinish", finish.Format("Jan 2006")),
			Span(css.Class("pace-badge pace-ontrack"), uistate.T("goals.planOnTrack")))
	default:
		readout = Span(uistate.T("goals.planFinish", finish.Format("Jan 2006")))
	}

	// #65: side-by-side plan comparison — the current plan bracketed by saving 25%
	// less and 25% more (clamped to the slider's honest range), each with its own
	// landing date, so "what if I stretch / ease off" is answered at a glance
	// instead of by dragging the slider back and forth.
	type comparePlan struct {
		key     string
		minor   int64
		current bool
	}
	plans := []comparePlan{
		{"goals.compareEasier", clampRange(cur.Get() * 3 / 4), false},
		{"goals.compareYours", cur.Get(), true},
		{"goals.compareHarder", clampRange(cur.Get() * 5 / 4), false},
	}
	var compareRows []ui.Node
	for _, p := range plans {
		fin, has := planFinish(props.App, g, p.minor, now)
		dateTxt := uistate.T("goals.compareNoLanding")
		if has {
			dateTxt = fin.Format("Jan 2006")
		}
		cls := "goal-plan-compare-row"
		if p.current {
			cls += " is-current"
		}
		compareRows = append(compareRows, Div(ClassStr(cls),
			Span(css.Class("goal-plan-compare-name"), uistate.T(p.key)),
			Span(css.Class("wf-line-amt"), uistate.T("goals.planPerMo", fmtMoney(money.New(p.minor, g.TargetAmount.Currency)))),
			Span(css.Class("wf-line-amt"), dateTxt),
		))
	}
	compare := Div(css.Class("goal-plan-compare"), Attr("data-testid", "goal-plan-compare-"+g.ID), compareRows)

	return Div(css.Class("goal-plan"), Attr("data-testid", "goal-plan-"+g.ID),
		Div(css.Class("goal-plan-head"),
			Span(css.Class("goal-plan-title"), uistate.T("goals.planTitle")),
			Span(css.Class("goal-plan-amt", tw.FontDisplay), uistate.T("goals.planPerMo", fmtMoney(money.New(cur.Get(), g.TargetAmount.Currency)))),
		),
		Div(css.Class("goal-plan-controls", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Input(Type("range"), css.Class("set-range goal-plan-slider"),
				Attr("min", strconv.FormatInt(min.Amount, 10)),
				Attr("max", strconv.FormatInt(max.Amount, 10)),
				Attr("step", strconv.FormatInt(step.Amount, 10)),
				Attr("data-testid", "goal-plan-slider-"+g.ID),
				Attr("aria-label", uistate.T("goals.planSliderLabel")),
				// QA #51: assistive tech announced the raw minor-units number
				// ("24059"); valuetext carries the formatted money-per-month.
				Attr("aria-valuetext", uistate.T("goals.planPerMo", fmtMoney(money.New(cur.Get(), g.TargetAmount.Currency)))),
				Value(strconv.FormatInt(cur.Get(), 10)), OnInput(onSlide), OnKeyDown(onSlideKey)),
			// QA #51: precise entry without dragging — a compact $/mo field kept in
			// two-way sync with the slider.
			Input(Type("text"), css.Class("field goal-plan-amount-input"),
				Style(map[string]string{"width": "7.5rem", "flex": "none"}),
				Attr("inputmode", "decimal"),
				Attr("data-testid", "goal-plan-amount-"+g.ID),
				Attr("aria-label", uistate.T("goals.planAmountLabel")),
				Value(draft.Get()), OnInput(onDraft), OnBlur(onDraftBlur)),
		),
		// aria-live so the projected finish is ANNOUNCED as the plan changes.
		Div(css.Class("goal-plan-readout"), Attr("role", "status"), Attr("aria-live", "polite"), Attr("data-testid", "goal-plan-readout-"+g.ID), readout),
		compare,
		Div(css.Class("goal-plan-actions", tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "goal-plan-use-"+g.ID), OnClick(useThis), uistate.T("goals.planUseThis")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-plan-wherefrom-"+g.ID), OnClick(whereFrom), uistate.T("goals.planWhereFrom")),
		),
	)
}

// planFinish projects a goal's finish date at a monthly contribution. When the
// goal draws on an APY-bearing account (GL2), it uses the compounding projector
// (goalinterest) so interest pulls the date in; otherwise it uses the linear
// slider math. It returns hasFinish=false when no finish can be projected.
func planFinish(app *appstate.App, g domain.Goal, monthlyMinor int64, now time.Time) (time.Time, bool) {
	apy := goalLinkedAPY(app, g)
	if apy > 0 {
		p := goalinterest.Project(g.CurrentAmount.Amount, monthlyMinor, g.TargetAmount.Amount, apy)
		if !p.Reached {
			return time.Time{}, false
		}
		return dateutil.AddMonths(now, p.Months), true
	}
	pt := goalsvc.SliderPointAt(g, monthlyMinor, now)
	return pt.Finish, pt.HasFinish
}

// goalLinkedAPY returns the highest APY among the goal's linked accounts (0 when
// none is APY-bearing), so the finish projection compounds when it should.
func goalLinkedAPY(app *appstate.App, g domain.Goal) float64 {
	if app == nil {
		return 0
	}
	linked := map[string]bool{}
	for _, id := range g.LinkedAccountIDs() {
		linked[id] = true
	}
	if len(linked) == 0 {
		return 0
	}
	var best float64
	for _, a := range app.Accounts() {
		if linked[a.ID] && a.APY > best {
			best = a.APY
		}
	}
	return best
}

// --- GL5: shared-goal pledge split-bar -------------------------------------------

type goalPledgeProps struct {
	App     *appstate.App
	Goal    domain.Goal
	Members []domain.Member
}

// GoalPledgeBar renders the shared-goal fairness readout (GL5): a BG13-style split
// bar of each member's actual contribution, with a quiet, blame-free standing line
// per member ("you're ahead of pledge · Priya on pace"). Read-only — pledges are
// edited in the goal editor. Renders nothing unless the goal carries pledges.
func GoalPledgeBar(props goalPledgeProps) ui.Node {
	return ui.CreateElement(goalPledgeBar, props)
}

func goalPledgeBar(props goalPledgeProps) ui.Node {
	g := props.Goal
	if !goalsvc.IsShared(g) {
		return Fragment()
	}
	now := time.Now()
	start := goalsvc.PledgeStartFrom(g, pledgeCreatedAnchor(g, now))
	readout := goalsvc.BuildPledgeReadout(g, "", goalsvc.MonthsElapsed(start, now))
	if len(readout.Standings) == 0 {
		return Fragment()
	}

	nameOf := func(id string) string {
		if id == "" {
			return uistate.T("goals.pledgeUnassigned")
		}
		for _, m := range props.Members {
			if m.ID == id {
				return m.Name
			}
		}
		return uistate.T("goals.pledgeUnassigned")
	}

	total := readout.TotalActual.Amount
	var segs []ui.Node
	var lines []ui.Node
	for i, s := range readout.Standings {
		name := nameOf(s.MemberID)
		// Split-bar segment sized by this member's actual share of the total.
		w := 0
		if total > 0 && s.Actual.Amount > 0 {
			w = int(s.Actual.Amount * 100 / total)
		}
		segs = append(segs, Div(ClassStr(fmt.Sprintf("goal-pledge-seg goal-pledge-seg-%d", i%6)),
			Attr("style", fmt.Sprintf("width:%d%%", w)),
			Attr("title", uistate.T("goals.pledgeRow", name, fmtMoney(s.Pledged))),
			Attr("aria-label", uistate.T("goals.pledgeRow", name, fmtMoney(s.Pledged)))))

		lines = append(lines, Div(css.Class("goal-pledge-line"),
			Span(ClassStr(fmt.Sprintf("goal-pledge-dot goal-pledge-seg-%d", i%6))),
			Span(css.Class("goal-pledge-name"), name),
			Span(css.Class("goal-pledge-standing", tw.TextDim), pledgeStandingText(s, name))))
	}

	return Div(css.Class("goal-pledge"), Attr("data-testid", "goal-pledge-"+g.ID),
		Div(css.Class("goal-pledge-head"), Span(css.Class("goal-pledge-title"), uistate.T("goals.pledgeTitle"))),
		Div(css.Class("goal-pledge-bar"), Attr("role", "img"), Attr("aria-label", uistate.T("goals.pledgeSplitLabel")), segs),
		Div(css.Class("goal-pledge-legend"), lines),
	)
}

// pledgeStandingText renders one member's blame-free standing sentence.
func pledgeStandingText(s goalsvc.PledgeStanding, name string) string {
	switch s.Pace() {
	case goalsvc.PledgePaceAhead:
		if s.AheadMonths >= 2 {
			return uistate.T("goals.pledgeAheadMonths", name, s.AheadMonths)
		}
		return uistate.T("goals.pledgeAhead", name)
	case goalsvc.PledgePaceBehind:
		return uistate.T("goals.pledgeBehind", name)
	default:
		return uistate.T("goals.pledgeOnPace", name)
	}
}

// pledgeCreatedAnchor is the best available "pledges started" date for a goal when
// it has no contributions yet: its last-reviewed timestamp, else now (so a brand-
// new shared goal reads as zero months elapsed rather than a huge gap).
func pledgeCreatedAnchor(g domain.Goal, now time.Time) time.Time {
	if !g.LastReviewedAt.IsZero() {
		return g.LastReviewedAt
	}
	return now
}

// isEmergencyName reports whether a goal's name marks it as an emergency fund —
// the same heuristic the SMART goal engines use, kept local to the screen layer.
func isEmergencyName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "emergency") || strings.Contains(n, "rainy")
}

// --- GL3: emergency-fund auto-sizing ---------------------------------------------

type goalEmergencyProps struct {
	App  *appstate.App
	Goal domain.Goal
	Base string
}

// GoalEmergencySizer offers one-tap emergency-fund sizing (GL3): it shows the
// derived essential month and sets the goal's target to a 3- or 6-month fund,
// stamping the basis so the SMART-G21 re-suggest can later notice drift. Renders
// only for a financial emergency-named goal. Low-pressure: a suggestion, not a nag.
func GoalEmergencySizer(props goalEmergencyProps) ui.Node {
	return ui.CreateElement(goalEmergencySizer, props)
}

func goalEmergencySizer(props goalEmergencyProps) ui.Node {
	g := props.Goal
	if g.EffectiveKind() != domain.GoalKindFinancial || g.Archived || !isEmergencyName(g.Name) {
		return Fragment()
	}
	pr := uistate.UsePrefs().Get()
	in := buildSmartInput(props.App, pr.WeekStartWeekday())
	basis := in.EssentialBasis()
	sizing := emergencyfund.Size(basis)
	em := basis.EssentialMonthlyMinor()
	if em <= 0 {
		return Fragment() // not enough data to size honestly
	}

	set := func(level emergencyfund.Level) func() {
		return func() {
			ng := g
			ng.TargetAmount = money.New(sizing.TargetMinor(level), in.Base)
			ng.EssentialBasisMinor = em
			if err := props.App.PutGoal(ng); err != nil {
				return
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("goals.essentialSetToast"), false)
		}
	}
	set3 := ui.UseEvent(Prevent(set(emergencyfund.LevelThree)))
	set6 := ui.UseEvent(Prevent(set(emergencyfund.LevelSix)))

	return Div(css.Class("goal-essential"), Attr("data-testid", "goal-essential-"+g.ID),
		Div(css.Class("goal-essential-head"),
			uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("goal-essential-title"), uistate.T("goals.essentialTitle"))),
		P(css.Class("goal-essential-body"), uistate.T("goals.essentialMonth", fmtMoney(money.New(em, in.Base)))),
		Div(css.Class("goal-essential-actions", tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-essential-3-"+g.ID), OnClick(set3),
				uistate.T("goals.essentialSet3"), Span(css.Class(tw.TextDim), " · "+fmtMoney(sizing.ThreeMonth))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-essential-6-"+g.ID), OnClick(set6),
				uistate.T("goals.essentialSet6"), Span(css.Class(tw.TextDim), " · "+fmtMoney(sizing.SixMonth))),
		),
		P(css.Class("goal-essential-hint", tw.TextDim), uistate.T("goals.essentialHint")),
	)
}

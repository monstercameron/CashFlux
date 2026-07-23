// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/finplan"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// planFrameworkKV persists which playbook the household chose (default FOO); the
// questionnaire answers persist alongside it so the two "confirm" steps stay
// resolved across reloads.
const (
	planFrameworkKV  = "plan.framework"
	planMatchKV      = "plan.q.match"
	planDeductibleKV = "plan.q.deductible"
)

// livePlanInputs derives the finplan signals from the same shared surfaces the
// health page reads (liquid cash + emergency-fund months + savings rate) plus a
// pass over the accounts to classify debt. Kept in the view layer because it
// glues the pure engines together; no assessment logic lives here.
func livePlanInputs(app *appstate.App, now time.Time) finplan.Inputs {
	h := liveHealthInputs(app, now)
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	liquid := majorMoney(liveEngineVars(app)["liquid_cash"], base).Amount

	accts := app.Accounts()
	hasHighInt, hasNonMort := false, false
	for _, a := range accts {
		if a.Archived || !a.IsLiability() || a.Type == domain.TypeMortgage {
			continue
		}
		hasNonMort = true
		if a.InterestRateAPR >= finplan.HighInterestAPR() {
			hasHighInt = true
		}
	}
	return finplan.Inputs{
		HasLiquidData:       h.HasLiquidData,
		LiquidCashMinor:     liquid,
		EmergencyMonths:     h.EmergencyMonths,
		HasIncome:           h.HasIncome,
		SavingsRatePct:      h.SavingsRatePct,
		KnowsLiabilities:    len(accts) > 0, // we can see the accounts, so we can judge the debt steps
		HasHighInterestDebt: hasHighInt,
		HasNonMortgageDebt:  hasNonMort,
	}
}

// planStepKey builds the i18n key for a step's title/detail from the framework
// and 1-based step number, so all copy stays in en_plan.go.
func planStepKey(fw finplan.Framework, num int, suffix string) string {
	base := "plan.foo."
	if fw == finplan.Ramsey {
		base = "plan.ramsey."
	}
	return base + strconv.Itoa(num) + "." + suffix
}

// planStatusMeta maps a step's assessed status (and whether it's the current
// step) to its pill label and tone class.
func planStatusMeta(s finplan.Status, isCurrent bool) (label, cls string) {
	if isCurrent {
		return uistate.T("plan.status.now"), "is-now"
	}
	switch s {
	case finplan.Done:
		return uistate.T("plan.status.done"), "is-done"
	case finplan.NotAssessable:
		return uistate.T("plan.status.ask"), "is-ask"
	default:
		return uistate.T("plan.status.todo"), "is-todo"
	}
}

// planStepRoute deep-links the current step to the screen where the user acts on
// it (debt payoff, cash accounts, investing), so the roadmap gives a next click.
func planStepRoute(fw finplan.Framework, num int) string {
	if fw == finplan.Ramsey {
		switch num {
		case 1, 3:
			return "accounts"
		case 2:
			return "debt"
		case 4, 7:
			return "investments"
		}
		return ""
	}
	switch num { // FOO
	case 3:
		return "debt"
	case 4:
		return "accounts"
	case 5, 6, 7:
		return "investments"
	}
	return ""
}

// answeredBool converts a tri-state questionnaire answer ("", "yes", "no",
// "unsure") into finplan's Answered/value pair: only a definite yes/no counts as
// answered.
func answeredBool(v string) (answered, yes bool) {
	switch v {
	case "yes":
		return true, true
	case "no":
		return true, false
	}
	return false, false
}

// PlanScreen is the /plan route: an opinionated, data-driven financial roadmap.
// It assesses the household against a chosen order-of-operations framework (the
// Financial Order of Operations by default, or Ramsey's Baby Steps) and names
// the single next move, backs it with the full ladder, a two-question
// on-boarding check, and free credit-score links.
func PlanScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return Div(css.Class("bento bento-sys"),
			rptTile("plan-empty", "1 / span 4", P(css.Class("empty"), uistate.T("common.notReady"))))
	}
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()

	// Persisted choices, hydrated once from the KV store.
	fw := ui.UseState(func() finplan.Framework {
		if uistate.KVGet(planFrameworkKV) == "ramsey" {
			return finplan.Ramsey
		}
		return finplan.FOO
	}())
	matchAns := ui.UseState(uistate.KVGet(planMatchKV))
	dedAns := ui.UseState(uistate.KVGet(planDeductibleKV))

	pickFOO := ui.UseEvent(func() {
		uistate.KVSet(planFrameworkKV, "foo")
		uistate.RequestPersist()
		fw.Set(finplan.FOO)
	})
	pickRamsey := ui.UseEvent(func() {
		uistate.KVSet(planFrameworkKV, "ramsey")
		uistate.RequestPersist()
		fw.Set(finplan.Ramsey)
	})

	framework := fw.Get()

	now := time.Now()
	in := livePlanInputs(app, now)
	matchDone, matchYes := answeredBool(matchAns.Get())
	in.AnsweredMatch, in.GetsFullMatch = matchDone, matchYes
	dedDone, dedYes := answeredBool(dedAns.Get())
	in.AnsweredDeductible, in.DeductiblesCovered = dedDone, dedYes

	plan := finplan.Assess(framework, in)
	cur, hasCur := plan.Current()

	done := 0
	for _, s := range plan.Steps {
		if s.Status == finplan.Done {
			done++
		}
	}

	// ── Hero: the single next move. ─────────────────────────────────────────────
	methodKey := "plan.method.foo"
	if framework == finplan.Ramsey {
		methodKey = "plan.method.ramsey"
	}
	fwName := uistate.T("plan.fw.foo")
	if framework == finplan.Ramsey {
		fwName = uistate.T("plan.fw.ramsey")
	}

	heroLabel := uistate.T("plan.hero.next")
	heroValue := uistate.T("plan.hero.allDone")
	heroDetail := uistate.T("plan.hero.allDoneDetail")
	var heroCTA ui.Node
	if hasCur {
		if cur.Status == finplan.NotAssessable {
			// Don't present something we can't judge as confident advice — send the
			// beginner to the two questions instead of headlining a jargon step.
			heroLabel = uistate.T("plan.hero.confirmLabel")
			heroValue = uistate.T("plan.hero.confirmValue")
			heroDetail = uistate.T("plan.hero.confirmDetail")
			heroCTA = Button(css.Class("btn"), Type("button"), Attr("data-testid", "plan-cta"),
				OnClick(func() { smoothScrollToSection("sec-plan-q") }),
				uistate.T("plan.hero.confirmCTA"))
		} else {
			// A real, assessed next move: lead with the plain-English one-liner.
			heroValue = uistate.T(planStepKey(framework, cur.Num, "title"))
			heroDetail = uistate.T(planStepKey(framework, cur.Num, "plain"))
			if route := planStepRoute(framework, cur.Num); route != "" {
				heroCTA = Button(css.Class("btn"), Type("button"), Attr("data-testid", "plan-cta"),
					OnClick(func() { nav.Navigate(uistate.RoutePath(route)) }),
					uistate.T("plan.fw.pick"))
			}
		}
	}

	hero := Div(css.Class("rpt-hero"), Attr("id", "sec-plan-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("plan.hero.eyebrow")),
		P(css.Class("plan-intro", tw.TextDim), uistate.T("plan.hero.intro")),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), heroLabel),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "plan-current"), heroValue),
			),
		),
		Div(css.Class("debt-chips"),
			rptChip(uistate.T("plan.chip.framework"), fwName, ""),
			rptChip(uistate.T("plan.chip.progress"), uistate.T("plan.chip.progressVal", done, len(plan.Steps)), rptToneCls("pos")),
			rptChip(uistate.T("plan.chip.method"), uistate.T(methodKey), ""),
		),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "plan-takeaway"), heroDetail),
		If(heroCTA != nil, Div(css.Class(tw.Mt3), heroCTA)),
	)

	// ── Playbook switch. ────────────────────────────────────────────────────────
	fooBtn := Button(ClassStr(planSegCls(framework == finplan.FOO)), Type("button"),
		Attr("data-testid", "plan-fw-foo"), Attr("aria-pressed", ariaBool(framework == finplan.FOO)),
		OnClick(pickFOO), uistate.T("plan.fw.foo"))
	ramseyBtn := Button(ClassStr(planSegCls(framework == finplan.Ramsey)), Type("button"),
		Attr("data-testid", "plan-fw-ramsey"), Attr("aria-pressed", ariaBool(framework == finplan.Ramsey)),
		OnClick(pickRamsey), uistate.T("plan.fw.ramsey"))
	fwDescKey := "plan.fw.fooDesc"
	if framework == finplan.Ramsey {
		fwDescKey = "plan.fw.ramseyDesc"
	}
	playbook := rptSection("sec-plan-fw", uistate.T("plan.fw.title"), nil, Div(
		Div(css.Class("plan-seg"), fooBtn, ramseyBtn),
		P(css.Class("t-body", tw.TextDim, tw.Mt3), Attr("data-testid", "plan-fw-desc"), uistate.T(fwDescKey)),
		P(css.Class("t-caption", tw.TextFaint, tw.Mt2), uistate.T("plan.fw.hint")),
	))

	// ── Questionnaire. ──────────────────────────────────────────────────────────
	setMatch := func(v string) {
		uistate.KVSet(planMatchKV, v)
		uistate.RequestPersist()
		matchAns.Set(v)
	}
	setDed := func(v string) {
		uistate.KVSet(planDeductibleKV, v)
		uistate.RequestPersist()
		dedAns.Set(v)
	}
	questions := rptSection("sec-plan-q", uistate.T("plan.q.title"), nil, Div(
		P(css.Class("t-caption", tw.TextFaint, tw.Mb3), uistate.T("plan.q.note")),
		ui.CreateElement(planQuestion, planQuestionProps{
			Q: uistate.T("plan.q.match"), Help: uistate.T("plan.q.matchHelp"),
			Value: matchAns.Get(), OnPick: setMatch, TestID: "plan-q-match"}),
		ui.CreateElement(planQuestion, planQuestionProps{
			Q: uistate.T("plan.q.deductible"), Help: uistate.T("plan.q.deductibleHelp"),
			Value: dedAns.Get(), OnPick: setDed, TestID: "plan-q-deductible"}),
	))

	// ── The full ladder. ────────────────────────────────────────────────────────
	rows := make([]ui.Node, 0, len(plan.Steps))
	for _, s := range plan.Steps {
		isCur := hasCur && s.Num == cur.Num
		label, tone := planStatusMeta(s.Status, isCur)
		rowCls := "plan-step"
		if isCur {
			rowCls += " is-current"
		}
		rows = append(rows, Div(ClassStr(rowCls), Attr("data-testid", "plan-step"),
			Div(css.Class("plan-step-num"), strconv.Itoa(s.Num)),
			Div(css.Class("plan-step-body"),
				Div(css.Class("plan-step-head"),
					Span(css.Class("plan-step-title", tw.Fold(tw.FontDisplay)), uistate.T(planStepKey(framework, s.Num, "title"))),
					Span(ClassStr("plan-pill "+tone), label)),
				P(css.Class("plan-step-plain"), uistate.T(planStepKey(framework, s.Num, "plain"))),
				P(css.Class("t-caption", tw.TextFaint), uistate.T(planStepKey(framework, s.Num, "detail"))),
			),
		))
	}
	ladder := rptSection("sec-plan-steps", uistate.T("plan.steps.title"), nil, Div(
		P(css.Class("t-caption", tw.TextFaint, tw.Mb3), uistate.T("plan.steps.note")),
		Div(css.Class("plan-ladder"), rows),
	))

	// ── Free credit-score links. ────────────────────────────────────────────────
	credit := rptSection("sec-plan-credit", uistate.T("plan.credit.title"), nil, Div(
		P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("plan.credit.note")),
		Div(css.Class("plan-credit-grid"),
			planCreditCard("plan.credit.annualHref", "plan.credit.annualName", "plan.credit.annualDesc"),
			planCreditCard("plan.credit.karmaHref", "plan.credit.karmaName", "plan.credit.karmaDesc"),
			planCreditCard("plan.credit.experianHref", "plan.credit.experianName", "plan.credit.experianDesc"),
			planCreditCard("plan.credit.chaseHref", "plan.credit.chaseName", "plan.credit.chaseDesc"),
		),
		P(css.Class("t-caption", tw.TextFaint, tw.Mt3), uistate.T("plan.credit.disclaimer")),
	))

	return Div(css.Class("bento bento-sys"), Attr("id", "plan-page"),
		rptTile("plan-hero", "1 / span 4", rptSection("", "", nil, hero)),
		// Questions first (they inform the hero above), then the playbook choice.
		rptTile("plan-questions", "span 2", questions),
		rptTile("plan-playbook", "span 2", playbook),
		rptTile("plan-ladder", "1 / span 4", ladder),
		rptTile("plan-credit", "1 / span 4", credit),
	)
}

// planSegCls is the class for a segmented-control button, on or off.
func planSegCls(on bool) string {
	if on {
		return "plan-seg-btn is-on"
	}
	return "plan-seg-btn"
}

// planCreditCard renders one free-credit-score resource as an external link card.
func planCreditCard(hrefKey, nameKey, descKey string) ui.Node {
	return A(css.Class("plan-credit-card"), Attr("href", uistate.T(hrefKey)),
		Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
		Attr("data-testid", "plan-credit-link"),
		Div(css.Class("plan-credit-name", tw.Fold(tw.FontDisplay)), uistate.T(nameKey)),
		P(css.Class("t-caption", tw.TextDim), uistate.T(descKey)),
	)
}

// planQuestionProps configures one yes/no/not-sure onboarding question.
type planQuestionProps struct {
	Q, Help string
	Value   string // "", "yes", "no", "unsure"
	OnPick  func(v string)
	TestID  string
}

// planQuestion is its own component so each option button's handler hook lives at
// a stable render position (never inside the parent's loop).
func planQuestion(props planQuestionProps) ui.Node {
	opt := func(val, labelKey string) ui.Node {
		cls := "plan-choice-btn"
		if props.Value == val {
			cls += " is-on"
		}
		return Button(ClassStr(cls), Type("button"),
			Attr("aria-pressed", ariaBool(props.Value == val)),
			OnClick(func() { props.OnPick(val) }),
			uistate.T(labelKey))
	}
	return Div(css.Class("plan-q"), Attr("data-testid", props.TestID),
		Div(css.Class("plan-q-label"), props.Q),
		P(css.Class("t-caption", tw.TextFaint), props.Help),
		Div(css.Class("plan-choices"),
			opt("yes", "plan.q.yes"),
			opt("no", "plan.q.no"),
			opt("unsure", "plan.q.unsure"),
		),
	)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

// Why one budget went over (SM-4). The card already shows the overage and, in the
// "what's driving this" disclosure, the merchants behind it. What it never said is
// which KIND of thing happened — one purchase, more trips, pricier trips, or
// nothing in particular — and those have different fixes, so leaving the reader to
// infer it from three merchant names is leaving them to guess.
//
// The judgment is internal/whyover; every figure it judges comes from
// internal/budgeting through the same period, rate, and category-cover path the
// card's own numbers take, so the sentence can never contradict the meter above it.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/whyover"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The free and paid halves of SM-4. The sentence is deterministic and free; the
// model only rephrases it and names a next step.
const (
	whyOverFreeCode = "SMART-B15F"
	whyOverAICode   = "SMART-B15"
)

// whyOverFacts is one budget's finding plus the pieces the copy needs.
type whyOverFacts struct {
	Result whyover.Result
	// TopLabel / TopAmount describe the biggest contributing merchant, formatted.
	TopLabel  string
	TopAmount string
	// Over / ProjectedOver / Avg / PriorAvg are the formatted figures the
	// reason sentences interpolate.
	Over          string
	ProjectedOver string
	Avg           string
	PriorAvg      string
}

// budgetWhyOver computes the finding for one budget over the VIEWED period.
//
// Every input is derived the same way the card's own figures are: the same
// PeriodRange, the same descendant category cover, the same rate table. A second
// derivation here is how a sentence ends up disagreeing with the meter it sits
// under, which is worse than having no sentence.
func budgetWhyOver(b domain.Budget, anchor time.Time) (whyOverFacts, bool) {
	app := appstate.Default
	if app == nil {
		return whyOverFacts{}, false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	if anchor.IsZero() {
		anchor = time.Now()
	}
	weekStart := uistate.LoadPrefs().WeekStartWeekday()
	start, end := budgeting.PeriodRange(b.Period, anchor, weekStart)
	// The comparable previous period: anchor one day before this one started, and
	// let PeriodRange find that period's own bounds. Subtracting a fixed number of
	// days would drift on months, which is the period most budgets use.
	prevStart, prevEnd := budgeting.PeriodRange(b.Period, start.AddDate(0, 0, -1), weekStart)

	txns := app.Transactions()
	descendants := categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs())
	covers := func(id string) bool { return b.TracksCategory(id) || descendants[id] }

	status, err := budgeting.EvaluateRollup(b, txns, start, end, rates, budgeting.DefaultNearThreshold, descendants)
	if err != nil {
		return whyOverFacts{}, false
	}
	prior, err := budgeting.EvaluateRollup(b, txns, prevStart, prevEnd, rates, budgeting.DefaultNearThreshold, descendants)
	if err != nil {
		return whyOverFacts{}, false
	}
	limit, err := status.Spent.Add(status.Remaining)
	if err != nil {
		return whyOverFacts{}, false
	}

	var topLabel string
	var topMinor int64
	if drivers, dbase := budgetTopDriversFor(b, 1, anchor); len(drivers) > 0 {
		topLabel, topMinor = drivers[0].Label, drivers[0].Amount
		_ = dbase
	}

	pace := budgeting.ProjectPace(status, start, end, time.Now())
	res, ok := whyover.Explain(whyover.Input{
		LimitMinor:      limit.Amount,
		SpentMinor:      status.Spent.Amount,
		TopDriverMinor:  topMinor,
		Count:           budgeting.ContributingCount(b, txns, start, end, covers),
		PriorSpentMinor: prior.Spent.Amount,
		PriorCount:      budgeting.ContributingCount(b, txns, prevStart, prevEnd, covers),
		ElapsedBP:       int(pace.Elapsed * 10000),
		ProjectedMinor:  pace.Projected.Amount,
	})
	if !ok {
		return whyOverFacts{}, false
	}
	cur := status.Spent.Currency
	return whyOverFacts{
		Result:        res,
		TopLabel:      topLabel,
		TopAmount:     fmtMoney(money.New(topMinor, cur)),
		Over:          fmtMoney(money.New(res.OverMinor, cur)),
		ProjectedOver: fmtMoney(money.New(res.ProjectedOverMinor, cur)),
		Avg:           fmtMoney(money.New(res.AvgMinor, cur)),
		PriorAvg:      fmtMoney(money.New(res.PriorAvgMinor, cur)),
	}, true
}

// whyOverSentence renders the finding as one plain-English sentence. Returns ""
// when the reason cannot be stated honestly — a one-charge finding with no named
// merchant, or a comparison reason without a comparison — so the panel shows
// nothing rather than a sentence with a hole in it.
func whyOverSentence(f whyOverFacts) string {
	switch f.Result.Reason {
	case whyover.ReasonOneCharge:
		if f.TopLabel == "" {
			return ""
		}
		return uistate.T("sm4.reasonOneCharge", f.TopLabel, f.TopAmount)
	case whyover.ReasonMoreOften:
		if !f.Result.HasComparison {
			return ""
		}
		return uistate.T("sm4.reasonMoreOften", f.Result.CountDelta)
	case whyover.ReasonPricier:
		if !f.Result.HasComparison {
			return ""
		}
		return uistate.T("sm4.reasonPricier", f.Avg, f.PriorAvg)
	case whyover.ReasonEarlyPace:
		return uistate.T("sm4.reasonEarlyPace", f.ProjectedOver)
	case whyover.ReasonSteady:
		return uistate.T("sm4.reasonSteady")
	}
	return ""
}

// budgetWhyOverProps carries the budget and the viewed period's anchor.
type budgetWhyOverProps struct {
	Budget domain.Budget
	Anchor time.Time
}

// budgetWhyOverHint renders the reason sentence above the drivers list, plus the
// optional Smart+ rephrasing. Its own component: it owns the request hooks.
func budgetWhyOverHint(props budgetWhyOverProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default

	aiText := ui.UseState("")
	loading := ui.UseState(false)
	errText := ui.UseState("")

	settings := uistate.LoadSmartSettings()
	facts, ok := budgetWhyOver(props.Budget, props.Anchor)
	sentence := ""
	if ok {
		sentence = whyOverSentence(facts)
	}

	backendAI := pr.Normalize().BackendActive()
	aiEnabled := app != nil && aiProviderConfigured(app, backendAI) &&
		settings.IsEnabled(whyOverAICode) && !settings.IsMuted(whyOverAICode)
	aiConn := smartAIConn{}
	if app != nil {
		aiConn = resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	}

	ask := ui.UseEvent(Prevent(func() {
		if loading.Get() || !ok {
			return
		}
		loading.Set(true)
		errText.Set("")
		// The model is handed the finding the app already made, plus the merchants,
		// and asked for phrasing and a next step. It is explicitly told not to
		// recompute, because a second opinion on arithmetic the app owns is not an
		// improvement — it is a way for two numbers to disagree on one card.
		ctx := whyOverContext(props.Budget, facts, props.Anchor)
		runSmartAI(aiConn, smartai.WhyOver(ctx),
			func(text string) { loading.Set(false); aiText.Set(text) },
			func(e string) { loading.Set(false); errText.Set(e) })
	}))

	if !ok || sentence == "" {
		return Fragment()
	}
	if !settings.IsEnabled(whyOverFreeCode) || settings.IsMuted(whyOverFreeCode) {
		return Fragment()
	}
	if !settings.DensityOrDefault().Shows(smart.AffordanceTooltip) {
		return Fragment()
	}

	var aiNode ui.Node = Fragment()
	if aiEnabled {
		label := uistate.T("sm4.aiAction")
		if loading.Get() {
			label = uistate.T("catSuggest.asking")
		}
		aiNode = Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "sm4-ai-"+props.Budget.ID), Attr("aria-disabled", ariaBool(loading.Get())),
			OnClick(ask), smartGlyph(true, tw.Fold(tw.W3, tw.H3)), Span(label))
	}

	return Div(css.Class("budget-whyover"), Attr("data-testid", "budget-whyover-"+props.Budget.ID),
		Div(css.Class("budget-whyover-row"),
			smartGlyph(false, tw.Fold(tw.W35, tw.H35, tw.ShrinkO)),
			Span(css.Class("budget-whyover-text"), sentence),
		),
		If(aiText.Get() != "", P(css.Class("budget-whyover-ai", tw.Text13),
			Attr("data-testid", "sm4-ai-answer-"+props.Budget.ID), aiText.Get())),
		If(errText.Get() != "", P(css.Class("err", tw.Text12), Attr("role", "alert"), errText.Get())),
		aiNode,
	)
}

// whyOverContext formats the finding for the model: the budget, the figures the
// app computed, and the merchants behind them. Nothing here asks the model to
// derive anything — it is given the answer and asked to say it well.
func whyOverContext(b domain.Budget, f whyOverFacts, anchor time.Time) string {
	drivers, base := budgetTopDriversFor(b, 3, anchor)
	s := b.Name + "\n"
	s += "Over by: " + f.Over + "\n"
	s += "Diagnosis (already computed): " + f.Result.Reason.String() + "\n"
	if f.Result.HasComparison {
		s += "Average charge: " + f.Avg + " (previous period " + f.PriorAvg + ")\n"
	}
	if len(drivers) > 0 {
		s += "Biggest contributors:\n"
		for _, d := range drivers {
			s += "- " + d.Label + ": " + fmtMoney(money.New(d.Amount, base)) + "\n"
		}
	}
	return s
}

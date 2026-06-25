// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// allocAlgoSummary returns a plain-English one-liner explaining the top-ranked
// candidate under the current profile — shown in the "Why this order?" card
// without requiring an AI key (G8 §7).
func allocAlgoSummary(ranked []allocate.Ranked, profileKey string) string {
	if len(ranked) == 0 {
		return ""
	}
	top := ranked[0]
	reason := ""
	bd := top.Breakdown
	switch {
	case top.Candidate.DebtReduction && bd.DebtReduction >= bd.Returns && bd.DebtReduction >= bd.Stability:
		reason = "paying it down is a guaranteed effective return"
	case bd.Returns >= bd.Stability && bd.Returns >= bd.Liquidity:
		reason = "it has the highest expected return"
	case bd.Stability >= bd.Returns && bd.Stability >= bd.Liquidity:
		reason = "it scores highest on stability"
	default:
		reason = "it best matches your current weights"
	}
	return fmt.Sprintf("%s ranks first: %s under the %s profile.", top.Candidate.Name, reason, profileKey)
}

// applyRowProps holds the props for one confirmation row in the apply-confirm panel.
type applyRowProps struct {
	Label string
}

// ApplyConfirmRow renders one confirmation row inside the apply-confirm panel.
func ApplyConfirmRow(props applyRowProps) ui.Node {
	return P(css.Class("muted"), props.Label)
}

type allocRowProps struct {
	R         allocate.Ranked
	Rank      int    // 1-based priority position, shown as "#1" (G8 glanceability)
	Amount    string // suggested dollar amount (empty when no split amount entered)
	OnExclude func(string)
}

// AllocRow renders one ranked suggestion with its score, breakdown bar, an
// optional suggested amount, and an Exclude action. Its own component so the
// action hook stays at a stable position.
func AllocRow(props allocRowProps) ui.Node {
	excl := ui.UseEvent(Prevent(func() { props.OnExclude(props.R.Candidate.ID) }))
	r := props.R
	// The breakdown's trailing note carries any non-numeric criteria: that this
	// pays debt, and how complete a funded goal is.
	note := ""
	if r.Candidate.DebtReduction {
		note += uistate.T("allocate.paysDebt")
	}
	if r.Breakdown.GoalProgress > 0 {
		note += uistate.T("allocate.goalNote", r.Breakdown.GoalProgress*100)
	}
	headRight := fmt.Sprintf("%.0f%%", r.Score*100)
	if props.Amount != "" {
		headRight = props.Amount + " · " + headRight
	}
	scorePct := int(r.Score*100 + 0.5)
	if scorePct < 0 {
		scorePct = 0
	}
	if scorePct > 100 {
		scorePct = 100
	}
	scoreLabel := uistate.T("allocate.scoreLabel", float64(scorePct))
	return Div(css.Class("budget"),
		Div(css.Class("budget-head"),
			If(props.Rank > 0, Span(css.Class("rank-badge"), Attr("aria-hidden", "true"), fmt.Sprintf("#%d", props.Rank))),
			Span(css.Class("row-desc"), r.Candidate.Name),
			Span(css.Class("budget-amount fig"), headRight),
			Button(css.Class("btn"), Type("button"), Title(uistate.T("allocate.excludeTitle")), OnClick(excl), uistate.T("allocate.exclude")),
		),
		// The score is shown once — in the head (headRight) and as this labelled
		// progress bar (C54). The breakdown is its own sub-line below, so no manual
		// separator span is needed.
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-label", scoreLabel),
			Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-valuenow", strconv.Itoa(scorePct)),
			Div(css.Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", scorePct))),
		),
		Span(css.Class("budget-sub"), uistate.T("allocate.breakdown",
			r.Breakdown.Returns*100, r.Breakdown.Stability*100, r.Breakdown.Liquidity*100, note)),
	)
}

type excludedChipProps struct {
	ID, Name  string
	OnRestore func(string)
}

// ExcludedChip is one excluded destination with a Restore action.
func ExcludedChip(props excludedChipProps) ui.Node {
	restore := ui.UseEvent(Prevent(func() { props.OnRestore(props.ID) }))
	return Div(css.Class("row"),
		Span(css.Class("row-desc"), props.Name),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("allocate.restoreTitle")), OnClick(restore), uistate.T("allocate.restore")),
	)
}

// parseWeight reads a weight input, treating blank/invalid/negative as 0.
func parseWeight(s string) float64 {
	if f := textutil.ParseFloat(s); f > 0 {
		return f
	}
	return 0
}

// trimWeight renders a weight for an input field without trailing zeros.
func trimWeight(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// allocProfiles maps a profile key to its criterion weights.
func allocProfiles() map[string]allocate.Weights {
	return map[string]allocate.Weights{
		"balanced": {Returns: 1, Stability: 1, Liquidity: 1, DebtReduction: 1, GoalProgress: 1},
		"returns":  {Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 1},
		"safety":   {Stability: 3, Liquidity: 2, Returns: 1, DebtReduction: 1},
		"debt":     {DebtReduction: 4, Returns: 1, Stability: 1, Liquidity: 1},
		"goals":    {GoalProgress: 4, Returns: 1, Stability: 1, Liquidity: 1, DebtReduction: 1},
	}
}

// Allocate ranks where to put new capital: it builds candidates from the user's
// asset accounts (by expected return / stability / liquidity) and high-interest
// liabilities (paying them down is a guaranteed return), scores them by the
// chosen profile (internal/allocate), and shows ranked, explainable suggestions.
func Allocate() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// nav is used to route the user to Settings when AI credentials are missing (C54).
	nav := router.UseNavigate()
	goToSettings := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/settings")) }))

	// amountStr is declared early so the income pre-fill handler (below) can reference
	// it; its OnInput handler (onAmount) is wired after the other form-state hooks.
	amountStr := ui.UseState("")

	// incomeNudgeDismissed tracks whether the user dismissed the income pre-fill banner
	// for this session. The hook is at a stable top-level position (not in a loop).
	incomeNudgeDismissed := ui.UseState(false)
	dismissIncomeNudge := ui.UseEvent(Prevent(func() { incomeNudgeDismissed.Set(true) }))

	// Compute this month's income once (pure read — no hooks).
	settings0 := app.Settings()
	base0 := settings0.BaseCurrency
	if base0 == "" {
		base0 = "USD"
	}
	rates0 := currency.Rates{Base: base0, Rates: settings0.FXRates}
	mStart0, mEnd0 := dateutil.MonthRange(time.Now())
	monthIncome, _, _ := ledger.PeriodTotals(app.Transactions(), mStart0, mEnd0, rates0)

	// prefillIncomeAmount copies the period income into the amount input field.
	// The event hook is at a stable render position — not in a loop.
	prefillIncomeAmount := ui.UseEvent(Prevent(func() {
		dec0 := currency.Decimals(base0)
		formatted := money.FormatMinor(monthIncome.Amount, dec0)
		amountStr.Set(formatted)
		incomeNudgeDismissed.Set(true)
	}))

	// allocationMode toggles between score-weighted and fill-to-target (envelope) allocation.
	allocationMode := ui.UseState("weighted")
	onMode := ui.UseEvent(func(e ui.Event) { allocationMode.Set(e.GetValue()) })

	// Weight-tuning is a power-user override (G8 §1/§6): collapse it behind an
	// "Advanced" disclosure so the typical path — pick profile, enter amount, see
	// list — isn't gated behind a wall of 5 weight inputs + a save-profile form.
	weightsOpen := ui.UseState(false)
	toggleWeights := ui.UseEvent(Prevent(func() { weightsOpen.Set(!weightsOpen.Get()) }))

	profile := ui.UseState("balanced")
	// Editable criterion weights drive the ranking; the profile select loads a
	// preset or saved profile into them, and they can be saved as a new profile.
	wReturns := ui.UseState("1")
	wStability := ui.UseState("1")
	wLiquidity := ui.UseState("1")
	wDebt := ui.UseState("1")
	wGoal := ui.UseState("1")
	profName := ui.UseState("")
	profMsg := ui.UseState("")
	setWeights := func(w allocate.Weights) {
		wReturns.Set(trimWeight(w.Returns))
		wStability.Set(trimWeight(w.Stability))
		wLiquidity.Set(trimWeight(w.Liquidity))
		wDebt.Set(trimWeight(w.DebtReduction))
		wGoal.Set(trimWeight(w.GoalProgress))
	}
	resolveWeights := func(sel string) allocate.Weights {
		if strings.HasPrefix(sel, "saved:") {
			for _, p := range app.AllocProfiles() {
				if "saved:"+p.ID == sel {
					return allocate.Weights{Returns: p.Returns, Stability: p.Stability, Liquidity: p.Liquidity, DebtReduction: p.DebtReduction, GoalProgress: p.GoalProgress}
				}
			}
		}
		if w, ok := allocProfiles()[sel]; ok {
			return w
		}
		return allocProfiles()["balanced"]
	}
	onProfile := ui.UseEvent(func(e ui.Event) {
		sel := e.GetValue()
		profile.Set(sel)
		setWeights(resolveWeights(sel))
		profMsg.Set("")
	})
	onWReturns := ui.UseEvent(func(v string) { wReturns.Set(v) })
	onWStability := ui.UseEvent(func(v string) { wStability.Set(v) })
	onWLiquidity := ui.UseEvent(func(v string) { wLiquidity.Set(v) })
	onWDebt := ui.UseEvent(func(v string) { wDebt.Set(v) })
	onWGoal := ui.UseEvent(func(v string) { wGoal.Set(v) })
	onProfName := ui.UseEvent(func(v string) { profName.Set(v) })
	reserveStr := ui.UseState("")
	maxPerStr := ui.UseState("")
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })
	onReserve := ui.UseEvent(func(v string) { reserveStr.Set(v) })
	onMaxPer := ui.UseEvent(func(v string) { maxPerStr.Set(v) })
	excluded := ui.UseState(map[string]bool{})
	toggleExclude := func(id string) {
		m := excluded.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if nm[id] {
			delete(nm, id)
		} else {
			nm[id] = true
		}
		excluded.Set(nm)
	}

	var cands []allocate.Candidate
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		if a.Class == domain.ClassLiability {
			if a.InterestRateAPR > 0 {
				cands = append(cands, allocate.Candidate{
					ID: a.ID, Name: uistate.T("allocate.payDown", a.Name), ExpectedReturnAPR: a.InterestRateAPR,
					StabilityScore: 100, LiquidityScore: 0, DebtReduction: true,
				})
			}
			continue
		}
		// A locked account (e.g. a CD) can't take new money until its lock lifts.
		if !a.LockUntil.IsZero() && a.LockUntil.After(time.Now()) {
			continue
		}
		cands = append(cands, allocate.Candidate{
			ID: a.ID, Name: a.Name, ExpectedReturnAPR: a.ExpectedReturnAPR,
			StabilityScore: a.StabilityScore, LiquidityScore: a.LiquidityScore,
		})
	}
	// Unfinished goals are candidates too — funding them is a place to put money.
	for _, g := range app.Goals() {
		if done, _ := goalsvc.IsComplete(g); done {
			continue
		}
		var remaining int64
		if allocationMode.Get() == "fill" {
			r := g.TargetAmount.Amount - g.CurrentAmount.Amount
			if r > 0 {
				remaining = r
			}
		}
		cands = append(cands, allocate.Candidate{
			ID: "goal:" + g.ID, Name: uistate.T("allocate.goalPrefix", g.Name),
			StabilityScore: 80, LiquidityScore: 60,
			GoalProgress:      float64(goalsvc.Percent(g)) / 100,
			RemainingToTarget: remaining,
		})
	}

	weights := allocate.Weights{
		Returns:       parseWeight(wReturns.Get()),
		Stability:     parseWeight(wStability.Get()),
		Liquidity:     parseWeight(wLiquidity.Get()),
		DebtReduction: parseWeight(wDebt.Get()),
		GoalProgress:  parseWeight(wGoal.Get()),
	}
	ranked := allocate.RankWith(cands, weights, allocate.Constraints{Exclude: excluded.Get()})
	// Drop candidates with no ranking signal (every criterion zero under the
	// current weights, e.g. an account with no expected-return/stability/liquidity
	// set) — they'd render as "0% · returns 0 · stability 0 …" noise. Remember if
	// any were hidden so we can nudge the user to fill in account attributes.
	scored := make([]allocate.Ranked, 0, len(ranked))
	for _, r := range ranked {
		if r.Score > 0 {
			scored = append(scored, r)
		}
	}
	hiddenZero := len(scored) < len(ranked)
	ranked = scored

	saveProfile := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(profName.Get())
		if name == "" {
			profMsg.Set(uistate.T("allocate.profileNameRequired"))
			return
		}
		p := domain.AllocationProfile{
			ID: id.New(), Name: name, Returns: weights.Returns, Stability: weights.Stability,
			Liquidity: weights.Liquidity, DebtReduction: weights.DebtReduction, GoalProgress: weights.GoalProgress,
		}
		if err := app.PutAllocProfile(p); err != nil {
			profMsg.Set(err.Error())
			return
		}
		profName.Set("")
		profile.Set("saved:" + p.ID)
		profMsg.Set(uistate.T("allocate.profileSaved"))
	}))
	deleteProfile := ui.UseEvent(Prevent(func() {
		sel := profile.Get()
		if !strings.HasPrefix(sel, "saved:") {
			return
		}
		_ = app.DeleteAllocProfile(strings.TrimPrefix(sel, "saved:"))
		profile.Set("balanced")
		setWeights(allocProfiles()["balanced"])
		profMsg.Set("")
	}))

	// Excluded candidates (shown in a restore list below).
	var excludedRows []ui.Node
	for _, c := range cands {
		if excluded.Get()[c.ID] {
			excludedRows = append(excludedRows, ui.CreateElement(ExcludedChip, excludedChipProps{
				ID: c.ID, Name: c.Name, OnRestore: toggleExclude,
			}))
		}
	}

	// Optional AI narrative explaining the ranking (bring-your-own-key).
	settings := app.Settings()
	aiKey := settings.OpenAIKey
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := pr.BackendActive()
	aiModel := settings.OpenAIModel
	if aiModel == "" {
		aiModel = "gpt-5.4-mini"
	}
	aiResult := ui.UseState("")
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	explain := ui.UseEvent(func() {
		if aiKey == "" && !useBackendAI {
			aiErr.Set(uistate.T("allocate.needKey"))
			return
		}
		if len(ranked) == 0 {
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		aiResult.Set("")
		var b strings.Builder
		for i, r := range ranked {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s (score %.0f%%)\n", r.Candidate.Name, r.Score*100)
		}
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant. In 2-3 sentences, explain why this ranking suits the chosen profile. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: "Profile: " + profile.Get() + ". Ranked places to put new money:\n" + b.String()},
		}
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, aiModel, messages, 0.5,
				func(c string, _ ai.Usage) { aiLoading.Set(false); aiResult.Set(c) },
				func(e string) { aiLoading.Set(false); aiErr.Set(e) },
			)
		} else {
			ai.SendChat(aiKey, ai.DefaultBaseURL, aiModel, messages, 0.5,
				func(c string, _ ai.Usage) { aiLoading.Set(false); aiResult.Set(c) },
				func(e string) { aiLoading.Set(false); aiErr.Set(e) },
			)
		}
	})

	// Optional amount split: when the user enters an amount, distribute it across
	// the ranked destinations (holding back any emergency-buffer reserve).
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	totalMinor, _ := money.ParseMinor(strings.TrimSpace(amountStr.Get()), dec)
	reserveMinor, _ := money.ParseMinor(strings.TrimSpace(reserveStr.Get()), dec)
	maxPerMinor, _ := money.ParseMinor(strings.TrimSpace(maxPerStr.Get()), dec)
	planByID := map[string]int64{}
	var remainder int64
	if totalMinor > 0 {
		var plans []allocate.Plan
		splitOpts := allocate.SplitOptions{Reserve: reserveMinor, MaxPer: maxPerMinor}
		if allocationMode.Get() == "fill" {
			plans, remainder = allocate.DistributeFillToTarget(ranked, totalMinor, splitOpts)
		} else {
			plans, remainder = allocate.Distribute(ranked, totalMinor, splitOpts)
		}
		for _, p := range plans {
			planByID[p.Candidate.ID] = p.Amount
		}
	}
	amountFor := func(id string) string {
		if totalMinor <= 0 {
			return ""
		}
		return fmtMoney(money.New(planByID[id], base))
	}

	// --- apply allocation state ---
	applyConfirming := ui.UseState(false)
	applyMsg := ui.UseState("")
	applyErr := ui.UseState("")
	applyDidApply := ui.UseState(false)

	// isLiabilityID returns true when id belongs to a liability account. Built
	// from the accounts list already in scope; no extra store call needed.
	liabilityIDs := map[string]bool{}
	for _, a := range app.Accounts() {
		if a.Class == domain.ClassLiability {
			liabilityIDs[a.ID] = true
		}
	}

	// planActions derives the full Action list from the current plans.
	planActions := func() []allocate.Action {
		if totalMinor <= 0 {
			return nil
		}
		plans := make([]allocate.Plan, 0, len(ranked))
		for _, r := range ranked {
			plans = append(plans, allocate.Plan{Candidate: r.Candidate, Amount: planByID[r.Candidate.ID]})
		}
		return allocate.PlanActions(plans, func(id string) bool { return liabilityIDs[id] })
	}

	openConfirm := ui.UseEvent(Prevent(func() {
		if totalMinor <= 0 {
			applyErr.Set(uistate.T("allocate.applyNoPlans"))
			return
		}
		applyErr.Set("")
		applyMsg.Set("")
		applyConfirming.Set(true)
	}))
	cancelConfirm := ui.UseEvent(Prevent(func() {
		applyConfirming.Set(false)
	}))
	doApply := ui.UseEvent(Prevent(func() {
		actions := planActions()
		if len(actions) == 0 {
			applyErr.Set(uistate.T("allocate.applyNoPlans"))
			applyConfirming.Set(false)
			return
		}
		result, err := app.ApplyAllocation(actions)
		if err != nil {
			applyErr.Set(uistate.T("allocate.applyErr", err.Error()))
			applyConfirming.Set(false)
			return
		}
		applyConfirming.Set(false)
		applyDidApply.Set(true)
		applyErr.Set("")

		// Build a plain-English result summary.
		goalAmt := fmtMoney(money.New(result.GoalDollars, base))
		earmarkAmt := fmtMoney(money.New(result.EarmarkDollars, base))
		var msg string
		switch {
		case result.GoalsFunded > 0 && result.EarmarksMade > 0:
			msg = uistate.T("allocate.applySuccess", result.GoalsFunded, goalAmt, earmarkAmt)
		case result.GoalsFunded > 0:
			msg = uistate.T("allocate.applySuccessNoEarmark", result.GoalsFunded, goalAmt)
		default:
			msg = uistate.T("allocate.applySuccessNoGoal", earmarkAmt, result.EarmarksMade)
		}
		if result.Overflow > 0 {
			msg += " " + uistate.T("allocate.applyOverflow", fmtMoney(money.New(result.Overflow, base)))
		}
		applyMsg.Set(msg)
	}))
	doUndo := ui.UseEvent(Prevent(func() {
		if err := app.UndoLastAllocation(); err != nil {
			applyErr.Set(uistate.T("allocate.undoErr"))
			return
		}
		applyMsg.Set(uistate.T("allocate.undoDone"))
		applyDidApply.Set(false)
		applyErr.Set("")
	}))

	// Build confirm rows — one per action. Wrapped in own components to keep hook
	// positions stable (no On* calls inside variable-length loops here; the
	// ApplyConfirmRow component has no interactive hooks).
	var confirmRows []ui.Node
	for _, act := range planActions() {
		amt := fmtMoney(money.New(act.Amount, base))
		var label string
		switch act.Kind {
		case allocate.GoalContribution:
			label = uistate.T("allocate.applyConfirmGoal", act.DestinationName, amt)
		case allocate.DebtPaydownEarmark:
			label = uistate.T("allocate.applyConfirmDebt", act.DestinationName, amt)
		default:
			label = uistate.T("allocate.applyConfirmEarmark", act.DestinationName, amt)
		}
		confirmRows = append(confirmRows, ui.CreateElement(ApplyConfirmRow, applyRowProps{Label: label}))
	}

	savedOpts := make([]ui.Node, 0)
	for _, p := range app.AllocProfiles() {
		key := "saved:" + p.ID
		savedOpts = append(savedOpts, Option(Value(key), SelectedIf(profile.Get() == key), p.Name))
	}

	// incomeNudge is the income pre-fill banner: shown whenever there is positive
	// income this month AND the user hasn't dismissed or used it yet. A dedicated
	// Section (not inline in the form card) so it can be targeted by tests and kept
	// as a clear visual affordance separate from the manual inputs.
	showIncomeNudge := monthIncome.Amount > 0 && !incomeNudgeDismissed.Get()
	incomeNudge := Fragment()
	if showIncomeNudge {
		incomeNudge = uiw.Card(uiw.CardProps{
			TestID: "income-nudge",
			Attrs:  []any{Attr("aria-label", uistate.T("allocate.incomeNudgeLabel"))},
			Body: Fragment(
				P(css.Class("muted"), uistate.T("allocate.incomeNudgeDesc",
					fmtMoney(monthIncome))),
				Div(css.Class(tw.Flex, tw.Gap2),
					Button(css.Class("btn btn-primary"), Type("button"),
						Attr("data-testid", "income-nudge-apply"),
						OnClick(prefillIncomeAmount),
						uistate.T("allocate.incomeNudgeApply", fmtMoney(monthIncome)),
					),
					Button(css.Class("btn"), Type("button"),
						OnClick(dismissIncomeNudge),
						uistate.T("allocate.incomeNudgeDismiss"),
					),
				),
			),
		})
	}

	return Div(
		incomeNudge,
		ProfileConfig(profileConfigProps{
			ProfileValue:    profile.Get(),
			ModeValue:       allocationMode.Get(),
			AmountStr:       amountStr.Get(),
			ReserveStr:      reserveStr.Get(),
			MaxPerStr:       maxPerStr.Get(),
			Base:            base,
			WReturns:        wReturns.Get(),
			WStability:      wStability.Get(),
			WLiquidity:      wLiquidity.Get(),
			WDebt:           wDebt.Get(),
			WGoal:           wGoal.Get(),
			ProfName:        profName.Get(),
			ProfMsg:         profMsg.Get(),
			WeightsOpen:     weightsOpen.Get(),
			TotalMinor:      totalMinor,
			Remainder:       remainder,
			SavedOpts:       savedOpts,
			OnMode:          onMode,
			OnProfile:       onProfile,
			OnAmount:        onAmount,
			OnReserve:       onReserve,
			OnMaxPer:        onMaxPer,
			OnWReturns:      onWReturns,
			OnWStability:    onWStability,
			OnWLiquidity:    onWLiquidity,
			OnWDebt:         onWDebt,
			OnWGoal:         onWGoal,
			OnProfName:      onProfName,
			OnSaveProfile:   saveProfile,
			OnDeleteProfile: deleteProfile,
			OnToggleWeights: toggleWeights,
		}),
		SuggestionList(suggestionListProps{
			Ranked:       ranked,
			ExcludedRows: excludedRows,
			HiddenZero:   hiddenZero,
			AmountFor:    amountFor,
			OnExclude:    toggleExclude,
		}),
		AiExplainCard(aiExplainCardProps{
			HasRanked:      len(ranked) > 0,
			AiResult:       aiResult.Get(),
			AiLoading:      aiLoading.Get(),
			AiErr:          aiErr.Get(),
			NeedKeyMsg:     uistate.T("allocate.needKey"),
			AlgoSummary:    allocAlgoSummary(ranked, profile.Get()),
			OnExplain:      explain,
			OnGoToSettings: goToSettings,
		}),
		// G8: when no amount has been entered yet, show a quiet hint so Marcus
		// knows the apply flow exists and is unlocked by entering an amount above.
		If(totalMinor == 0, P(css.Class("muted alloc-apply-hint"),
			uistate.T("allocate.applyHint"),
		)),
		If(totalMinor > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("allocate.applyTitle"),
			Attrs: []any{Attr("aria-label", uistate.T("allocate.applyTitle"))},
			Body: Fragment(
				P(css.Class("muted"), uistate.T("allocate.applyDesc")),
				If(applyErr.Get() != "", P(css.Class("err"), Attr("role", "alert"), applyErr.Get())),
				If(applyMsg.Get() != "", Div(css.Class(tw.Flex, tw.Gap1),
					P(css.Class("muted"), applyMsg.Get()),
					If(applyDidApply.Get(), Button(css.Class("btn"), Type("button"),
						Attr("aria-label", uistate.T("allocate.undoTitle")),
						OnClick(doUndo), uistate.T("allocate.undoButton"),
					)),
				)),
				IfElse(applyConfirming.Get(),
					Div(
						H3(css.Class("set-label"), uistate.T("allocate.applyConfirmTitle")),
						P(css.Class("muted"), uistate.T("allocate.applyConfirmDesc")),
						Div(css.Class("rows"), confirmRows),
						Div(css.Class(tw.Flex, tw.Gap1),
							Button(css.Class("btn btn-primary"), Type("button"),
								Attr("aria-label", uistate.T("allocate.applyConfirmTitle")),
								OnClick(doApply), uistate.T("allocate.applyConfirm"),
							),
							Button(css.Class("btn"), Type("button"), OnClick(cancelConfirm), uistate.T("allocate.applyCancel")),
						),
					),
					If(!applyDidApply.Get(),
						Button(css.Class("btn btn-primary"), Type("button"),
							Attr("aria-label", uistate.T("allocate.applyTitle")),
							Attr("data-testid", "allocate-apply-btn"),
							OnClick(openConfirm), uistate.T("allocate.applyButton"),
						),
					),
				),
			),
		})),
	)
}

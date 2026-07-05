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
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// allocAlgoSummary returns a plain-English one-liner explaining the top-ranked candidate under
// the current profile — shown in the "Why this order?" tile without requiring an AI key.
func allocAlgoSummary(ranked []allocate.Ranked, profileKey string) string {
	if len(ranked) == 0 {
		return ""
	}
	top := ranked[0]
	reason := ""
	bd := top.Breakdown
	switch {
	case top.Candidate.DebtReduction && bd.DebtReduction >= bd.Returns && bd.DebtReduction >= bd.Stability:
		reason = uistate.T("allocate.reasonDebt")
	case bd.Returns >= bd.Stability && bd.Returns >= bd.Liquidity:
		reason = uistate.T("allocate.reasonReturns")
	case bd.Stability >= bd.Returns && bd.Stability >= bd.Liquidity:
		reason = uistate.T("allocate.reasonStability")
	default:
		reason = uistate.T("allocate.reasonWeights")
	}
	return uistate.T("allocate.algoSummary", top.Candidate.Name, reason, allocProfileLabel(profileKey))
}

// allocProfileLabel maps a profile key to its display label (falls back to the key).
func allocProfileLabel(key string) string {
	switch key {
	case "balanced":
		return uistate.T("allocate.balanced")
	case "returns":
		return uistate.T("allocate.maxReturns")
	case "safety":
		return uistate.T("allocate.safety")
	case "debt":
		return uistate.T("allocate.debt")
	case "goals":
		return uistate.T("allocate.goals")
	}
	return key
}

// parseWeight reads a weight input, treating blank/invalid/negative as 0.
func parseWeight(s string) float64 {
	if f := textutil.ParseFloat(s); f > 0 {
		return f
	}
	return 0
}

// trimWeight renders a weight for an input field without trailing zeros.
func trimWeight(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

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

// allocResolveWeights returns the criterion weights for a profile key: a saved profile's stored
// weights, a built-in preset, or the balanced default.
func allocResolveWeights(app *appstate.App, sel string) allocate.Weights {
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

// allocView is the derived render model every allocate tile shares: the ranked destinations,
// the split plan for the entered amount, and the headline figures. Built once per render.
type allocView struct {
	Base         string
	Dec          int
	Ranked       []allocate.Ranked
	Candidates   []allocate.Candidate
	HiddenZero   bool // some candidates were dropped for having no ranking signal
	PlanByID     map[string]int64
	TotalMinor   int64 // amount to allocate (after parsing the input)
	ReserveMinor int64
	MaxPerMinor  int64
	Remainder    int64 // unallocated leftover (buffer + caps/rounding)
	MonthIncome  money.Money
}

// Allocate is the widgetized /allocate surface — a bento host of native tiles that plan where
// new money should go. A single controller owns the interconnected state (amount → ranking →
// split → apply) and lays the tiles out; each tile is its own component fed a computed view
// model plus callbacks. The plan is persisted (uistate.AllocConfig) so it survives a reload and
// feeds the alloc_* engine variables. Tiles:
//
//   - hero      : the amount to put to work + the split figures (allocatable / reserve / left over)
//   - controls  : profile + split mode, advanced (buffer / cap / weight tuning), metrics toggle
//   - plan      : the ranked, explainable destination cards with suggested amounts
//   - explain   : "why this order?" + an optional AI narrative
//   - apply     : the confirm / apply / undo flow (only once an amount is entered)
//   - formula   : an opt-in FormulaBuilder over the alloc_* variables
func Allocate() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	rates := currency.Rates{Base: base, Rates: settings.FXRates}

	// Seed the editable plan from the persisted config so it survives a reload.
	cfg := uistate.AllocConfigGet()
	seedAmount := ""
	if cfg.AmountMinor > 0 {
		seedAmount = money.FormatMinor(cfg.AmountMinor, dec)
	}
	seedReserve := ""
	if cfg.ReserveMinor > 0 {
		seedReserve = money.FormatMinor(cfg.ReserveMinor, dec)
	}
	seedMaxPer := ""
	if cfg.MaxPerMinor > 0 {
		seedMaxPer = money.FormatMinor(cfg.MaxPerMinor, dec)
	}

	nav := router.UseNavigate()
	goToSettings := ui.UseEvent(Prevent(func() { uistate.OpenGlobalSettingsAt("ai") }))

	// --- amount (local to the hero; persisted via AllocConfig) ---
	amountStr := ui.UseState(seedAmount)
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })

	// Income pre-fill nudge.
	incomeNudgeDismissed := ui.UseState(false)
	dismissIncomeNudge := ui.UseEvent(Prevent(func() { incomeNudgeDismissed.Set(true) }))
	mStart, mEnd := dateutil.MonthRange(time.Now())
	monthIncome, _, _ := ledger.PeriodTotals(app.Transactions(), mStart, mEnd, rates)
	prefillIncome := ui.UseEvent(Prevent(func() {
		amountStr.Set(money.FormatMinor(monthIncome.Amount, dec))
		incomeNudgeDismissed.Set(true)
	}))

	activeMemberID := uistate.UseActiveMember().Get()

	// --- strategy (shared atoms: edited in the AllocProfileHost flip modal) ---
	profile := uistate.UseAllocProfileSel()
	mode := uistate.UseAllocModeSel()
	reserveAtom := uistate.UseAllocReserveStr()
	maxPerAtom := uistate.UseAllocMaxPerStr()
	wReturns := uistate.UseAllocWReturns()
	wStability := uistate.UseAllocWStability()
	wLiquidity := uistate.UseAllocWLiquidity()
	wDebt := uistate.UseAllocWDebt()
	wGoal := uistate.UseAllocWGoal()
	// Seed the buffer/cap/weights once from the persisted plan + active profile (the screen has
	// the base-currency precision the atoms can't compute themselves).
	seeded := uistate.UseAllocSeeded()
	if !seeded.Get() {
		reserveAtom.Set(seedReserve)
		maxPerAtom.Set(seedMaxPer)
		w := allocResolveWeights(app, profile.Get())
		wReturns.Set(trimWeight(w.Returns))
		wStability.Set(trimWeight(w.Stability))
		wLiquidity.Set(trimWeight(w.Liquidity))
		wDebt.Set(trimWeight(w.DebtReduction))
		wGoal.Set(trimWeight(w.GoalProgress))
		seeded.Set(true)
	}

	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))
	profileOpen := uistate.UseAllocProfileOpen()
	openStrategy := ui.UseEvent(Prevent(func() { profileOpen.Set(true) }))

	excluded := ui.UseState(map[string]bool{})
	toggleExclude := func(cid string) {
		m := excluded.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			if v {
				nm[k] = v
			}
		}
		if nm[cid] {
			delete(nm, cid)
		} else {
			nm[cid] = true
		}
		excluded.Set(nm)
	}

	weights := allocate.Weights{
		Returns: parseWeight(wReturns.Get()), Stability: parseWeight(wStability.Get()),
		Liquidity: parseWeight(wLiquidity.Get()), DebtReduction: parseWeight(wDebt.Get()),
		GoalProgress: parseWeight(wGoal.Get()),
	}

	// --- build the view model (candidates → ranked → split) ---
	v := computeAllocView(app, computeAllocInput{
		Base: base, Dec: dec, Rates: rates, ActiveMember: activeMemberID,
		Mode: mode.Get(), Excluded: excluded.Get(), MonthIncome: monthIncome, Weights: weights,
		AmountStr: amountStr.Get(), ReserveStr: reserveAtom.Get(), MaxPerStr: maxPerAtom.Get(),
	})

	// Persist the plan whenever a plan input changes, so alloc_* variables stay live. Silent
	// (no data-revision bump) — the FormulaBuilder tile reads the config on its own render.
	planKey := fmt.Sprintf("%d|%d|%d|%s|%s", v.TotalMinor, v.ReserveMinor, v.MaxPerMinor, profile.Get(), mode.Get())
	ui.UseEffect(func() func() {
		uistate.SetAllocConfig(uistate.AllocConfig{
			AmountMinor: v.TotalMinor, ReserveMinor: v.ReserveMinor, MaxPerMinor: v.MaxPerMinor,
			Profile: profile.Get(), Mode: mode.Get(),
		})
		return nil
	}, planKey)

	amountFor := func(cid string) string {
		if v.TotalMinor <= 0 {
			return ""
		}
		return fmtMoney(money.New(v.PlanByID[cid], base))
	}

	// --- AI narrative ---
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
		if len(v.Ranked) == 0 {
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		aiResult.Set("")
		var b strings.Builder
		for i, r := range v.Ranked {
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

	// --- apply flow ---
	applyConfirming := ui.UseState(false)
	applyMsg := ui.UseState("")
	applyErr := ui.UseState("")
	applyDidApply := ui.UseState(false)
	liabilityIDs := map[string]bool{}
	for _, a := range app.Accounts() {
		if a.Class == domain.ClassLiability {
			liabilityIDs[a.ID] = true
		}
	}
	planActions := func() []allocate.Action {
		if v.TotalMinor <= 0 {
			return nil
		}
		plans := make([]allocate.Plan, 0, len(v.Ranked))
		for _, r := range v.Ranked {
			plans = append(plans, allocate.Plan{Candidate: r.Candidate, Amount: v.PlanByID[r.Candidate.ID]})
		}
		return allocate.PlanActions(plans, func(cid string) bool { return liabilityIDs[cid] })
	}
	openConfirm := ui.UseEvent(Prevent(func() {
		if v.TotalMinor <= 0 {
			applyErr.Set(uistate.T("allocate.applyNoPlans"))
			return
		}
		applyErr.Set("")
		applyMsg.Set("")
		applyConfirming.Set(true)
	}))
	cancelConfirm := ui.UseEvent(Prevent(func() { applyConfirming.Set(false) }))
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
	confirmLabels := make([]string, 0)
	for _, act := range planActions() {
		amt := fmtMoney(money.New(act.Amount, base))
		switch act.Kind {
		case allocate.GoalContribution:
			confirmLabels = append(confirmLabels, uistate.T("allocate.applyConfirmGoal", act.DestinationName, amt))
		case allocate.DebtPaydownEarmark:
			confirmLabels = append(confirmLabels, uistate.T("allocate.applyConfirmDebt", act.DestinationName, amt))
		default:
			confirmLabels = append(confirmLabels, uistate.T("allocate.applyConfirmEarmark", act.DestinationName, amt))
		}
	}

	// Excluded → restore chips.
	var excludedRows []ui.Node
	for _, c := range v.Candidates {
		if excluded.Get()[c.ID] {
			excludedRows = append(excludedRows, ui.CreateElement(excludedChip, excludedChipProps{ID: c.ID, Name: c.Name, OnRestore: toggleExclude}))
		}
	}

	showIncomeNudge := monthIncome.Amount > 0 && !incomeNudgeDismissed.Get() && v.TotalMinor == 0

	tiles := []ui.Node{
		ui.CreateElement(allocHeroTile, allocHeroProps{
			View: v, AmountStr: amountStr.Get(), OnAmount: onAmount,
			ShowIncomeNudge: showIncomeNudge, MonthIncome: monthIncome,
			OnPrefillIncome: prefillIncome, OnDismissIncome: dismissIncomeNudge,
		}),
		ui.CreateElement(allocStrategyTile, allocStrategyProps{
			View: v, Profile: profile.Get(), Mode: mode.Get(),
			ReserveMinor: v.ReserveMinor, MaxPerMinor: v.MaxPerMinor,
			ShowFormulas:   showFormulas.Get(),
			OnEditStrategy: openStrategy, OnToggleFormulas: toggleFormulas,
		}),
		ui.CreateElement(allocPlanTile, allocPlanProps{
			View: v, AmountFor: amountFor, OnExclude: toggleExclude, ExcludedRows: excludedRows,
			OnViewSource: func(route string) { nav.Navigate(uistate.RoutePath(route)) },
		}),
		ui.CreateElement(allocExplainTile, allocExplainProps{
			HasRanked: len(v.Ranked) > 0, AiResult: aiResult.Get(), AiLoading: aiLoading.Get(),
			AiErr: aiErr.Get(), AlgoSummary: allocAlgoSummary(v.Ranked, profile.Get()),
			OnExplain: explain, OnGoToSettings: goToSettings,
		}),
		ui.CreateElement(allocApplyTile, allocApplyProps{
			HasAmount: v.TotalMinor > 0, Confirming: applyConfirming.Get(),
			Msg: applyMsg.Get(), Err: applyErr.Get(), DidApply: applyDidApply.Get(),
			ConfirmLabels: confirmLabels,
			OnOpenConfirm: openConfirm, OnConfirm: doApply, OnCancel: cancelConfirm, OnUndo: doUndo,
		}),
	}
	if showFormulas.Get() {
		tiles = append(tiles, ui.CreateElement(allocFormulaTile))
	}

	return Div(css.Class("bento bento-allocate"), tiles)
}

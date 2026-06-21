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
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

type allocRowProps struct {
	R         allocate.Ranked
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
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

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
	amountStr := ui.UseState("")
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
		cands = append(cands, allocate.Candidate{
			ID: "goal:" + g.ID, Name: uistate.T("allocate.goalPrefix", g.Name),
			StabilityScore: 80, LiquidityScore: 60,
			GoalProgress: float64(goalsvc.Percent(g)) / 100,
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
		aiModel = "gpt-4o-mini"
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
		plans, remainder = allocate.Distribute(ranked, totalMinor, allocate.SplitOptions{Reserve: reserveMinor, MaxPer: maxPerMinor})
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

	var listBody ui.Node
	switch {
	case len(ranked) == 0 && hiddenZero:
		listBody = P(css.Class("empty"), uistate.T("allocate.setAttributes"))
	case len(ranked) == 0 && len(excludedRows) == 0:
		listBody = P(css.Class("empty"), uistate.T("allocate.emptyNoCandidates"))
	case len(ranked) == 0:
		listBody = P(css.Class("empty"), uistate.T("allocate.allExcluded"))
	default:
		listBody = Div(MapKeyed(ranked,
			func(r allocate.Ranked) any { return r.Candidate.ID },
			func(r allocate.Ranked) ui.Node {
				return ui.CreateElement(AllocRow, allocRowProps{R: r, Amount: amountFor(r.Candidate.ID), OnExclude: toggleExclude})
			},
		))
	}

	savedOpts := make([]ui.Node, 0)
	for _, p := range app.AllocProfiles() {
		key := "saved:" + p.ID
		savedOpts = append(savedOpts, Option(Value(key), SelectedIf(profile.Get() == key), p.Name))
	}

	return Div(
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("allocate.profileTitle")),
			P(css.Class("muted"), uistate.T("allocate.profileDesc")),
			Form(css.Class("form-grid"),
				Select(css.Class("field"), OnChange(onProfile),
					Option(Value("balanced"), SelectedIf(profile.Get() == "balanced"), uistate.T("allocate.balanced")),
					Option(Value("returns"), SelectedIf(profile.Get() == "returns"), uistate.T("allocate.maxReturns")),
					Option(Value("safety"), SelectedIf(profile.Get() == "safety"), uistate.T("allocate.safety")),
					Option(Value("debt"), SelectedIf(profile.Get() == "debt"), uistate.T("allocate.debt")),
					Option(Value("goals"), SelectedIf(profile.Get() == "goals"), uistate.T("allocate.goals")),
					savedOpts,
				),
				Input(css.Class("field"), Type("number"), Placeholder(uistate.T("allocate.amountPlaceholder", base)), Value(amountStr.Get()), Step("0.01"), OnInput(onAmount)),
				Input(css.Class("field"), Type("number"), Placeholder(uistate.T("allocate.reservePlaceholder")), Value(reserveStr.Get()), Step("0.01"), OnInput(onReserve)),
				Input(css.Class("field"), Type("number"), Title(uistate.T("allocate.maxPerTitle")), Placeholder(uistate.T("allocate.maxPerPlaceholder", base)), Value(maxPerStr.Get()), Step("0.01"), OnInput(onMaxPer)),
			),
			P(css.Class("set-label"), uistate.T("allocate.weightsTitle")),
			Form(css.Class("form-grid"),
				Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wReturns")),
					Input(css.Class("field"), Type("number"), Value(wReturns.Get()), Step("0.5"), OnInput(onWReturns))),
				Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wStability")),
					Input(css.Class("field"), Type("number"), Value(wStability.Get()), Step("0.5"), OnInput(onWStability))),
				Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wLiquidity")),
					Input(css.Class("field"), Type("number"), Value(wLiquidity.Get()), Step("0.5"), OnInput(onWLiquidity))),
				Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wDebt")),
					Input(css.Class("field"), Type("number"), Value(wDebt.Get()), Step("0.5"), OnInput(onWDebt))),
				Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wGoal")),
					Input(css.Class("field"), Type("number"), Value(wGoal.Get()), Step("0.5"), OnInput(onWGoal))),
			),
			Form(css.Class("form-grid"), OnSubmit(saveProfile),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("allocate.profileNamePlaceholder")), Value(profName.Get()), OnInput(onProfName)),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("allocate.saveProfile")),
				If(strings.HasPrefix(profile.Get(), "saved:"), Button(css.Class("btn"), Type("button"), OnClick(deleteProfile), uistate.T("allocate.deleteProfile"))),
			),
			If(profMsg.Get() != "", P(css.Class("muted"), profMsg.Get())),
			If(totalMinor > 0 && remainder > 0, P(css.Class("muted"), uistate.T("allocate.keptBack", fmtMoney(money.New(remainder, base))))),
		),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("allocate.suggestionsTitle")),
			listBody,
		),
		If(len(excludedRows) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("allocate.excludedTitle")),
			P(css.Class("muted"), uistate.T("allocate.excludedDesc")),
			Div(css.Class("rows"), excludedRows),
		)),
		If(len(ranked) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("allocate.whyTitle")),
			Button(css.Class("btn"), Type("button"), OnClick(explain), IfElse(aiLoading.Get(), Text(uistate.T("allocate.thinking")), Text(uistate.T("allocate.explainAI")))),
			If(aiErr.Get() != "", P(css.Class("err"), Attr("role", "alert"), aiErr.Get())),
			If(aiResult.Get() != "", P(css.Class("muted"), aiResult.Get())),
		)),
	)
}

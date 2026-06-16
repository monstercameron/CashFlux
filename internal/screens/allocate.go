//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// allocProfiles maps a profile key to its criterion weights.
func allocProfiles() map[string]allocate.Weights {
	return map[string]allocate.Weights{
		"balanced": {Returns: 1, Stability: 1, Liquidity: 1, DebtReduction: 1},
		"returns":  {Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 1},
		"safety":   {Stability: 3, Liquidity: 2, Returns: 1, DebtReduction: 1},
		"debt":     {DebtReduction: 4, Returns: 1, Stability: 1, Liquidity: 1},
	}
}

// Allocate ranks where to put new capital: it builds candidates from the user's
// asset accounts (by expected return / stability / liquidity) and high-interest
// liabilities (paying them down is a guaranteed return), scores them by the
// chosen profile (internal/allocate), and shows ranked, explainable suggestions.
func Allocate() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	profile := ui.UseState("balanced")
	onProfile := ui.UseEvent(func(e ui.Event) { profile.Set(e.GetValue()) })

	var cands []allocate.Candidate
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		if a.Class == domain.ClassLiability {
			if a.InterestRateAPR > 0 {
				cands = append(cands, allocate.Candidate{
					ID: a.ID, Name: "Pay down " + a.Name, ExpectedReturnAPR: a.InterestRateAPR,
					StabilityScore: 100, LiquidityScore: 0, DebtReduction: true,
				})
			}
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
			ID: "goal:" + g.ID, Name: "Goal · " + g.Name,
			StabilityScore: 80, LiquidityScore: 60,
		})
	}

	weights := allocProfiles()[profile.Get()]
	ranked := allocate.Rank(cands, weights)

	// Optional AI narrative explaining the ranking (bring-your-own-key).
	settings := app.Settings()
	aiKey := settings.OpenAIKey
	aiModel := settings.OpenAIModel
	if aiModel == "" {
		aiModel = "gpt-4o-mini"
	}
	aiResult := ui.UseState("")
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	explain := ui.UseEvent(func() {
		if aiKey == "" {
			aiErr.Set("Add your OpenAI key in Settings to get an explanation.")
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
		ai.SendChat(aiKey, ai.DefaultBaseURL, aiModel, messages, 0.5,
			func(c string) { aiLoading.Set(false); aiResult.Set(c) },
			func(e string) { aiLoading.Set(false); aiErr.Set(e) },
		)
	})

	var listBody ui.Node
	if len(ranked) == 0 {
		listBody = P(Class("empty"), "Add asset accounts (with expected return, stability, and liquidity) or high-interest debts to get suggestions.")
	} else {
		rows := make([]ui.Node, 0, len(ranked))
		for _, r := range ranked {
			debtNote := ""
			if r.Candidate.DebtReduction {
				debtNote = " · pays debt"
			}
			rows = append(rows, Div(Class("budget"),
				Div(Class("budget-head"),
					Span(Class("row-desc"), r.Candidate.Name),
					Span(Class("budget-amount fig"), fmt.Sprintf("%.0f%%", r.Score*100)),
				),
				Div(Class("bar"), Div(Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", int(r.Score*100))))),
				Span(Class("budget-sub"), fmt.Sprintf("returns %.0f · stability %.0f · liquidity %.0f%s",
					r.Breakdown.Returns*100, r.Breakdown.Stability*100, r.Breakdown.Liquidity*100, debtNote)),
			))
		}
		listBody = Div(rows)
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Allocation profile"),
			P(Class("muted"), "Pick what matters most; suggestions are ranked and show why."),
			Form(Class("form-grid"),
				Select(Class("field"), OnChange(onProfile),
					Option(Value("balanced"), SelectedIf(profile.Get() == "balanced"), "Balanced"),
					Option(Value("returns"), SelectedIf(profile.Get() == "returns"), "Maximize returns"),
					Option(Value("safety"), SelectedIf(profile.Get() == "safety"), "Safety & access"),
					Option(Value("debt"), SelectedIf(profile.Get() == "debt"), "Pay down debt"),
				),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Where to put your money next"),
			listBody,
		),
		If(len(ranked) > 0, Section(Class("card"),
			H2(Class("card-title"), "Why this order?"),
			Button(Class("btn"), Type("button"), OnClick(explain), IfElse(aiLoading.Get(), Text("Thinking…"), Text("Explain with AI"))),
			If(aiErr.Get() != "", P(Class("err"), aiErr.Get())),
			If(aiResult.Get() != "", P(Class("muted"), aiResult.Get())),
		)),
	)
}

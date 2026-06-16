//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
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
	debtNote := ""
	if r.Candidate.DebtReduction {
		debtNote = " · pays debt"
	}
	headRight := fmt.Sprintf("%.0f%%", r.Score*100)
	if props.Amount != "" {
		headRight = props.Amount + " · " + headRight
	}
	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), r.Candidate.Name),
			Span(Class("budget-amount fig"), headRight),
			Button(Class("btn"), Type("button"), Title("Leave this out of the suggestions"), OnClick(excl), "Exclude"),
		),
		Div(Class("bar"), Div(Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", int(r.Score*100))))),
		Span(Class("budget-sub"), fmt.Sprintf("returns %.0f · stability %.0f · liquidity %.0f%s",
			r.Breakdown.Returns*100, r.Breakdown.Stability*100, r.Breakdown.Liquidity*100, debtNote)),
	)
}

type excludedChipProps struct {
	ID, Name  string
	OnRestore func(string)
}

// ExcludedChip is one excluded destination with a Restore action.
func ExcludedChip(props excludedChipProps) ui.Node {
	restore := ui.UseEvent(Prevent(func() { props.OnRestore(props.ID) }))
	return Div(Class("row"),
		Span(Class("row-desc"), props.Name),
		Button(Class("btn"), Type("button"), Title("Bring this back into the suggestions"), OnClick(restore), "Restore"),
	)
}

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
	amountStr := ui.UseState("")
	reserveStr := ui.UseState("")
	onAmount := ui.UseEvent(func(v string) { amountStr.Set(v) })
	onReserve := ui.UseEvent(func(v string) { reserveStr.Set(v) })
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
					ID: a.ID, Name: "Pay down " + a.Name, ExpectedReturnAPR: a.InterestRateAPR,
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
			ID: "goal:" + g.ID, Name: "Goal · " + g.Name,
			StabilityScore: 80, LiquidityScore: 60,
		})
	}

	weights := allocProfiles()[profile.Get()]
	ranked := allocate.RankWith(cands, weights, allocate.Constraints{Exclude: excluded.Get()})

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

	// Optional amount split: when the user enters an amount, distribute it across
	// the ranked destinations (holding back any emergency-buffer reserve).
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	totalMinor, _ := money.ParseMinor(strings.TrimSpace(amountStr.Get()), dec)
	reserveMinor, _ := money.ParseMinor(strings.TrimSpace(reserveStr.Get()), dec)
	planByID := map[string]int64{}
	var remainder int64
	if totalMinor > 0 {
		var plans []allocate.Plan
		plans, remainder = allocate.Distribute(ranked, totalMinor, allocate.SplitOptions{Reserve: reserveMinor})
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
	case len(ranked) == 0 && len(excludedRows) == 0:
		listBody = P(Class("empty"), "Add asset accounts (with expected return, stability, and liquidity) or high-interest debts to get suggestions.")
	case len(ranked) == 0:
		listBody = P(Class("empty"), "Every destination is excluded. Restore one below to see suggestions.")
	default:
		listBody = Div(MapKeyed(ranked,
			func(r allocate.Ranked) any { return r.Candidate.ID },
			func(r allocate.Ranked) ui.Node {
				return ui.CreateElement(AllocRow, allocRowProps{R: r, Amount: amountFor(r.Candidate.ID), OnExclude: toggleExclude})
			},
		))
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
				Input(Class("field"), Type("number"), Placeholder("Amount to allocate ("+base+")"), Value(amountStr.Get()), Step("0.01"), OnInput(onAmount)),
				Input(Class("field"), Type("number"), Placeholder("Keep back (emergency buffer)"), Value(reserveStr.Get()), Step("0.01"), OnInput(onReserve)),
			),
			If(totalMinor > 0 && remainder > 0, P(Class("muted"), "Kept back: "+fmtMoney(money.New(remainder, base))+" (buffer plus anything caps or rounding left over).")),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Where to put your money next"),
			listBody,
		),
		If(len(excludedRows) > 0, Section(Class("card"),
			H2(Class("card-title"), "Excluded"),
			P(Class("muted"), "These are left out of the suggestions. Restore any to bring it back."),
			Div(Class("rows"), excludedRows),
		)),
		If(len(ranked) > 0, Section(Class("card"),
			H2(Class("card-title"), "Why this order?"),
			Button(Class("btn"), Type("button"), OnClick(explain), IfElse(aiLoading.Get(), Text("Thinking…"), Text("Explain with AI"))),
			If(aiErr.Get() != "", P(Class("err"), aiErr.Get())),
			If(aiResult.Get() != "", P(Class("muted"), aiResult.Get())),
		)),
	)
}

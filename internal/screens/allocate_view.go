// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/allocateview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/marginal"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// computeAllocInput carries everything computeAllocView needs to build the model — the scoring
// weights, the split mode/controls, and the current member scope.
type computeAllocInput struct {
	Base          string
	Dec           int
	Rates         currency.Rates
	ActiveMember  string
	Mode          string
	Weights       allocate.Weights
	Excluded      map[string]bool
	MonthIncome   money.Money
	AmountStr     string
	ReserveStr    string
	MaxPerStr     string
	Now           time.Time
	StandingFloor int64
}

// computeAllocView builds the shared allocate model over the live store: candidate destinations
// (asset accounts to grow, interest-bearing debts to pay down, unfinished goals to fund), the
// ranking under the chosen weights, and — when an amount is entered — the split plan. Pure (no
// hooks), so every tile renders from one consistent model.
func computeAllocView(app *appstate.App, in computeAllocInput) allocView {
	v := allocView{Base: in.Base, Dec: in.Dec, MonthIncome: in.MonthIncome, PlanByID: map[string]int64{}}
	accounts, accountErr := app.AccountsChecked()
	transactions, transactionErr := app.TransactionsChecked()
	goals, goalErr := app.GoalsChecked()
	if err := errors.Join(accountErr, transactionErr, goalErr); err != nil {
		v.InputInvalid, v.InputUnavailable = true, true
		return v
	}
	prepared, err := allocateview.Prepare(allocateview.Input{
		Accounts: accounts, Transactions: transactions, Goals: goals,
		Now: in.Now, ActiveMember: in.ActiveMember, Mode: in.Mode, Weights: in.Weights,
		Excluded: in.Excluded, Decimals: in.Dec, Amount: in.AmountStr, Reserve: in.ReserveStr,
		MaxPer: in.MaxPerStr, StandingFloor: in.StandingFloor,
	})
	if err != nil {
		return v
	} // failed snapshots never masquerade as a successfully prepared empty result
	v.Candidates = prepared.Candidates
	candidateByID := make(map[string]allocate.Candidate, len(v.Candidates))
	liabilityByID := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		liabilityByID[a.ID] = a.Class == domain.ClassLiability
	}
	for i := range v.Candidates {
		if strings.HasPrefix(v.Candidates[i].ID, "goal:") {
			v.Candidates[i].Name = uistate.T("allocate.goalPrefix", v.Candidates[i].Name)
		}
		if liabilityByID[v.Candidates[i].ID] {
			v.Candidates[i].Name = uistate.T("allocate.payDown", v.Candidates[i].Name)
		}
		candidateByID[v.Candidates[i].ID] = v.Candidates[i]
	}
	v.Ranked, v.HiddenZero = prepared.Ranked, prepared.HiddenZero
	for i := range v.Ranked {
		v.Ranked[i].Candidate = candidateByID[v.Ranked[i].Candidate.ID]
	}
	v.PlanByID, v.TotalMinor, v.ReserveMinor, v.AllocatableMinor, v.MaxPerMinor = prepared.PlanByID, prepared.TotalMinor, prepared.ReserveMinor, prepared.AllocatableMinor, prepared.MaxPerMinor
	v.ReserveFromStanding, v.Remainder, v.Marginal = prepared.ReserveFromStanding, prepared.Remainder, prepared.Marginal
	v.InputInvalid = prepared.TotalErr != nil || prepared.ReserveErr != nil || prepared.MaxPerErr != nil
	for i := range v.Marginal {
		if strings.HasPrefix(v.Marginal[i].ID, "goal:") {
			v.Marginal[i].Name = uistate.T("allocate.goalPrefix", v.Marginal[i].Name)
		}
		if liabilityByID[v.Marginal[i].ID] {
			v.Marginal[i].Name = uistate.T("allocate.payDown", v.Marginal[i].Name)
		}
	}
	return v
}

// allocMarginalDestinations describes each destination for the money-per-year
// comparison (WF10).
//
// Debt carries the APR it avoids and the balance it can absorb; an account
// carries its expected return. Everything else is KindOther, which reports no
// figure rather than a zero — see internal/marginal for why that distinction is
// the whole point.

// --- WF10: what another dollar actually earns, per destination ------------------

// allocMarginalProps carries the destinations and the amount being compared.
type allocMarginalProps struct {
	Dests       []marginal.Destination
	AmountMinor int64
	Base        string
}

// allocMarginalTile prices every destination in money per year.
//
// The ranking tile says which order; this says by how much, which is the part
// somebody can argue with. Destinations with no rate are LISTED WITHOUT A FIGURE
// rather than dropped — removing the emergency fund from a list of places to put
// money would be a recommendation dressed as a filter.
func allocMarginalTile(props allocMarginalProps) ui.Node {
	if len(props.Dests) == 0 {
		return Fragment()
	}
	if props.AmountMinor <= 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "alloc-marginal", Title: "", GridColumn: "1 / span 4", Preview: true,
			Body: allocSection("sec-alloc-marginal", uistate.T("allocMarginal.title"), Fragment(),
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "alloc-marginal-empty"),
					uistate.T("allocMarginal.needAmount"))),
		})
	}
	bs := marginal.Compare(props.Dests, props.AmountMinor)

	// Whether the choice is even worth making. A $3-a-year gap between the top two
	// deserves less of somebody's attention than the ranking implies.
	var spreadNode ui.Node = Fragment()
	if gap, ok := marginal.SpreadMinor(bs); ok {
		key := "allocMarginal.spread"
		if gap < smallSpreadMinor {
			key = "allocMarginal.spreadSmall"
		}
		spreadNode = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "alloc-marginal-spread"),
			uistate.T(key, fmtMoney(money.New(gap, props.Base))))
	}

	rows := make([]ui.Node, 0, len(bs))
	for _, b := range bs {
		val := uistate.T("allocMarginal.noFigure")
		cls := "t-caption " + tw.Fold(tw.TextFaint)
		if b.Known {
			val = uistate.T("allocMarginal.perYear", fmtMoney(money.New(b.AnnualMinor, props.Base)))
			cls = "t-body"
		}
		rows = append(rows, Div(css.Class("alloc-marginal-row"),
			Attr("data-testid", "alloc-marginal-"+b.Destination.ID),
			Span(css.Class("alloc-marginal-name"), b.Destination.Name),
			Span(ClassStr(cls), val),
			// Why a figure is smaller than the amount offered, when it is.
			If(b.Capped, Span(css.Class("t-caption", tw.TextFaint),
				uistate.T("allocMarginal.capped",
					fmtMoney(money.New(b.EffectiveMinor(), props.Base))))),
		))
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "alloc-marginal", Title: "", GridColumn: "1 / span 4", Preview: true,
		Body: allocSection("sec-alloc-marginal", uistate.T("allocMarginal.title"), Fragment(),
			Fragment(
				P(css.Class("t-caption", tw.TextDim), uistate.T("allocMarginal.sub",
					fmtMoney(money.New(props.AmountMinor, props.Base)))),
				spreadNode,
				Div(css.Class("alloc-marginal-list"), rows),
				P(css.Class("t-caption", tw.TextFaint), uistate.T("allocMarginal.note")),
			)),
	})
}

// smallSpreadMinor is the gap below which choosing between the top two
// destinations is not worth deliberating over ($10 a year).
const smallSpreadMinor = 1_000

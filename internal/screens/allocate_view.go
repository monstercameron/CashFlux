// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
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
	Base         string
	Dec          int
	Rates        currency.Rates
	ActiveMember string
	Mode         string
	Weights      allocate.Weights
	Excluded     map[string]bool
	MonthIncome  money.Money
	AmountStr    string
	ReserveStr   string
	MaxPerStr    string
}

// computeAllocView builds the shared allocate model over the live store: candidate destinations
// (asset accounts to grow, interest-bearing debts to pay down, unfinished goals to fund), the
// ranking under the chosen weights, and — when an amount is entered — the split plan. Pure (no
// hooks), so every tile renders from one consistent model.
func computeAllocView(app *appstate.App, in computeAllocInput) allocView {
	v := allocView{Base: in.Base, Dec: in.Dec, MonthIncome: in.MonthIncome, PlanByID: map[string]int64{}}

	var cands []allocate.Candidate
	for _, a := range app.Accounts() {
		if a.Archived || !ownerVisibleTo(a.OwnerID, in.ActiveMember) {
			continue
		}
		if a.Class == domain.ClassLiability {
			if r, ok := a.RateAPR(); ok && r > 0 {
				cands = append(cands, allocate.Candidate{
					ID: a.ID, Name: uistate.T("allocate.payDown", a.Name), ExpectedReturnAPR: r,
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
	for _, g := range app.Goals() {
		if done, _ := goalsvc.IsComplete(g); done {
			continue
		}
		if !ownerVisibleTo(g.OwnerID, in.ActiveMember) {
			continue
		}
		var remaining int64
		if in.Mode == "fill" {
			// R-LEAK: this re-derived max(0, target-current) inline, which is
			// goalsvc.Remaining minus its currency check. The helper also refuses to
			// subtract across currencies rather than silently comparing raw minor
			// units of two different ones — a goal in EUR against a current amount
			// in USD produced a number here that meant nothing.
			if rem, err := goalsvc.Remaining(g); err == nil {
				remaining = rem.Amount
			}
		}
		cands = append(cands, allocate.Candidate{
			ID: "goal:" + g.ID, Name: uistate.T("allocate.goalPrefix", g.Name),
			StabilityScore: 80, LiquidityScore: 60,
			GoalProgress:      float64(goalsvc.Percent(g)) / 100,
			RemainingToTarget: remaining,
		})
	}
	v.Candidates = cands

	// WF10: the same destinations on ONE scale a person can check. A rank says
	// "this first"; "$220 a year against $40" says why, and is a claim somebody
	// can disagree with. Only destinations with a RATE get a figure — a goal or an
	// emergency reserve has real value that is not interest, and attaching a
	// dollar number to it would be inventing one to fill a column.
	v.Marginal = allocMarginalDestinations(app, in.ActiveMember)

	ranked := allocate.RankWith(cands, in.Weights, allocate.Constraints{Exclude: in.Excluded})
	scored := make([]allocate.Ranked, 0, len(ranked))
	for _, r := range ranked {
		if r.Score > 0 {
			scored = append(scored, r)
		}
	}
	v.HiddenZero = len(scored) < len(ranked)
	v.Ranked = scored

	v.TotalMinor, _ = money.ParseMinor(strings.TrimSpace(in.AmountStr), in.Dec)
	v.ReserveMinor, _ = money.ParseMinor(strings.TrimSpace(in.ReserveStr), in.Dec)
	v.MaxPerMinor, _ = money.ParseMinor(strings.TrimSpace(in.MaxPerStr), in.Dec)
	if v.TotalMinor > 0 {
		var plans []allocate.Plan
		opts := allocate.SplitOptions{Reserve: v.ReserveMinor, MaxPer: v.MaxPerMinor}
		if in.Mode == "fill" {
			plans, v.Remainder = allocate.DistributeFillToTarget(v.Ranked, v.TotalMinor, opts)
		} else {
			plans, v.Remainder = allocate.Distribute(v.Ranked, v.TotalMinor, opts)
		}
		for _, p := range plans {
			v.PlanByID[p.Candidate.ID] = p.Amount
		}
	}
	return v
}

// Allocatable is the amount that actually gets split (total minus the reserve, floored at 0).
func (v allocView) Allocatable() int64 {
	if a := v.TotalMinor - v.ReserveMinor; a > 0 {
		return a
	}
	return 0
}

// allocMarginalDestinations describes each destination for the money-per-year
// comparison (WF10).
//
// Debt carries the APR it avoids and the balance it can absorb; an account
// carries its expected return. Everything else is KindOther, which reports no
// figure rather than a zero — see internal/marginal for why that distinction is
// the whole point.
func allocMarginalDestinations(app *appstate.App, activeMember string) []marginal.Destination {
	var out []marginal.Destination
	txns := app.Transactions()
	for _, a := range app.Accounts() {
		if a.Archived || !ownerVisibleTo(a.OwnerID, activeMember) {
			continue
		}
		if a.Class == domain.ClassLiability {
			owed := int64(0)
			if bal, err := ledger.Balance(a, txns); err == nil {
				owed = bal.Amount
				if owed < 0 {
					owed = -owed
				}
			}
			out = append(out, marginal.Destination{
				ID: a.ID, Name: uistate.T("allocate.payDown", a.Name),
				Kind: marginal.KindDebt, AnnualRatePct: a.RateAPROrZero(),
				CapacityMinor: owed,
			})
			continue
		}
		if !a.LockUntil.IsZero() && a.LockUntil.After(time.Now()) {
			continue
		}
		out = append(out, marginal.Destination{
			ID: a.ID, Name: a.Name, Kind: marginal.KindSavings,
			AnnualRatePct: a.ExpectedReturnAPR,
		})
	}
	for _, g := range app.Goals() {
		if done, _ := goalsvc.IsComplete(g); done || !ownerVisibleTo(g.OwnerID, activeMember) {
			continue
		}
		// A goal's value is hitting it by a date, which is not a rate. Listed so it
		// is not silently dropped from the places money could go, with no figure.
		out = append(out, marginal.Destination{
			ID: "goal:" + g.ID, Name: uistate.T("allocate.goalPrefix", g.Name),
			Kind: marginal.KindOther,
		})
	}
	return out
}

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

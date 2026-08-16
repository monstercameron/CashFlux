// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/allocate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type allocDestRowProps struct {
	R            allocate.Ranked
	Rank         int    // 1-based priority position
	Amount       string // suggested amount (empty when no amount entered)
	OnExclude    func(string)
	OnViewSource func(route string) // navigate to where this destination's value lives
}

// allocDestRow renders one ranked destination as a card: a priority medallion, the name, a
// suggested amount + score, an accent score meter, the criterion breakdown chips, and a ⋯
// overflow menu holding the Exclude action. Its own component so the per-row action hook stays
// stable in the list. The #1 destination gets an accent focus treatment so the order reads at
// a glance.
func allocDestRow(props allocDestRowProps) ui.Node {
	r := props.R
	excl := ui.UseEvent(Prevent(func() {
		if props.OnExclude != nil {
			props.OnExclude(r.Candidate.ID)
		}
	}))
	// Where this destination's value lives: a goal → /goals, a debt (interest-bearing
	// liability) → the debt page, any other account → /accounts. The ⋯ menu links there so the
	// user can jump from "put $X here" to the record the figure comes from.
	sourceRoute, sourceLabel := "/accounts", uistate.T("allocate.viewAccount")
	switch {
	case strings.HasPrefix(r.Candidate.ID, "goal:"):
		sourceRoute, sourceLabel = "/goals", uistate.T("allocate.viewGoal")
	case r.Candidate.DebtReduction:
		sourceRoute, sourceLabel = "/debt", uistate.T("allocate.viewDebt")
	}
	viewSource := ui.UseEvent(Prevent(func() {
		if props.OnViewSource != nil {
			props.OnViewSource(sourceRoute)
		}
	}))
	scorePct := int(r.Score*100 + 0.5)
	scorePct = max(0, min(scorePct, 100))

	cardCls := "alloc-dest"
	if props.Rank == 1 {
		cardCls += " is-first"
	}

	// Right-side figure: the suggested amount (when an amount is entered) over the score.
	var amountNode ui.Node = Fragment()
	if props.Amount != "" {
		amountNode = Span(css.Class("alloc-dest-amount", tw.FontDisplay), props.Amount)
	}

	// Breakdown chips — the criteria that earned the score, plus any qualitative note.
	chips := []any{css.Class("alloc-dest-breakdown")}
	// C353: these are NORMALIZED SCORES on abstract axes, not rates. Printing
	// "RETURNS 27%" beside a mortgage at 4.1% APR reads as a claim about the
	// mortgage — and "RETURNS 100%" on the card reads as an absurd one. They are
	// scored out of 100 now, with no percent sign, and the Returns chip carries
	// the REAL rate beside its score so the number it was being mistaken for is
	// actually available.
	chips = append(chips,
		allocBreakdownChip(uistate.T("allocate.critReturns"), r.Breakdown.Returns,
			allocRealRate(r.Candidate)),
		allocBreakdownChip(uistate.T("allocate.critStability"), r.Breakdown.Stability, ""),
		allocBreakdownChip(uistate.T("allocate.critLiquidity"), r.Breakdown.Liquidity, ""),
	)
	if r.Candidate.DebtReduction {
		chips = append(chips, Span(css.Class("alloc-dest-tag"), uistate.T("allocate.paysDebtTag")))
	}
	if r.Breakdown.GoalProgress > 0 {
		// Goal progress IS a real percentage of a real target, so it keeps its
		// percent sign — the axes that lost theirs are the abstract ones (C353).
		chips = append(chips, Span(css.Class("alloc-dest-tag"),
			fmt.Sprintf("%s %.0f%%", uistate.T("allocate.goalTag"), r.Breakdown.GoalProgress*100)))
	}

	return Div(css.Class(cardCls), Attr("data-testid", "alloc-dest-"+r.Candidate.ID), Attr("role", "listitem"),
		Div(css.Class("alloc-dest-rank", tw.FontDisplay), Attr("aria-hidden", "true"), fmt.Sprintf("%d", props.Rank)),
		Div(css.Class("alloc-dest-body"),
			Div(css.Class("alloc-dest-head"),
				Span(css.Class("alloc-dest-name"), r.Candidate.Name),
				Div(css.Class("alloc-dest-figs"),
					amountNode,
					Span(css.Class("alloc-dest-score", tw.TextDim),
						Attr("title", uistate.T("allocate.scoreHint")),
						uistate.T("allocate.scoreOutOf", scorePct)),
				),
			),
			uiw.MeterBar(uiw.MeterBarProps{
				Value: float64(scorePct), Tone: "bg-accent",
				Label: uistate.T("allocate.scoreMeterAria", scorePct),
			}),
			Div(chips...),
		),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "alloc-menu-" + r.Candidate.ID,
			AriaLabel:    uistate.T("allocate.moreActions"),
			ToggleTestID: "alloc-menu-" + r.Candidate.ID,
			WrapClass:    "alloc-dest-menu",
			Items: []ui.Node{
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "alloc-source-"+r.Candidate.ID), OnClick(viewSource), sourceLabel),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "alloc-exclude-"+r.Candidate.ID), Title(uistate.T("allocate.excludeTitle")),
					OnClick(excl), uistate.T("allocate.exclude")),
			},
		}),
	)
}

// allocBreakdownChip is one criterion's contribution as a compact labelled chip.
//
// The value is a SCORE out of 100 on an abstract axis, and it is rendered
// without a percent sign for exactly that reason (C353): "RETURNS 27%" beside a
// mortgage at 4.1% APR reads as a statement about the mortgage's rate, and
// "RETURNS 100%" reads as an impossible one. `real` is the underlying finance
// figure when the axis has one ("4.1% APR"), shown beside the score so the
// number the reader was reaching for is actually there.
func allocBreakdownChip(label string, frac float64, real string) ui.Node {
	pct := max(0, min(int(frac*100+0.5), 100))
	return Span(css.Class("alloc-dest-chip"),
		Span(css.Class("alloc-dest-chip-label", tw.TextDim), label),
		Span(css.Class("alloc-dest-chip-val"), uistate.T("allocate.scoreOutOf", pct)),
		If(real != "", Span(css.Class("alloc-dest-chip-real", tw.TextFaint), real)),
	)
}

// allocRealRate renders the actual finance number behind the Returns axis: the
// expected return for an asset, or the interest a debt payment stops accruing.
// Empty when the candidate has no rate, so the chip shows a bare score rather
// than a fabricated "0.0%".
func allocRealRate(c allocate.Candidate) string {
	if c.ExpectedReturnAPR == 0 {
		return ""
	}
	if c.DebtReduction {
		return uistate.T("allocate.realAPRDebt", c.ExpectedReturnAPR)
	}
	return uistate.T("allocate.realAPRAsset", c.ExpectedReturnAPR)
}

type excludedChipProps struct {
	ID, Name  string
	OnRestore func(string)
}

// excludedChip is one excluded destination with a Restore action.
func excludedChip(props excludedChipProps) ui.Node {
	restore := ui.UseEvent(Prevent(func() { props.OnRestore(props.ID) }))
	return Div(css.Class("alloc-excluded-chip"),
		Span(css.Class("alloc-excluded-name"), props.Name),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "alloc-restore-"+props.ID),
			Title(uistate.T("allocate.restoreTitle")), OnClick(restore), uistate.T("allocate.restore")),
	)
}

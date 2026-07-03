// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/allocate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

type allocDestRowProps struct {
	R         allocate.Ranked
	Rank      int    // 1-based priority position
	Amount    string // suggested amount (empty when no amount entered)
	OnExclude func(string)
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
	chips = append(chips,
		allocBreakdownChip(uistate.T("allocate.critReturns"), r.Breakdown.Returns),
		allocBreakdownChip(uistate.T("allocate.critStability"), r.Breakdown.Stability),
		allocBreakdownChip(uistate.T("allocate.critLiquidity"), r.Breakdown.Liquidity),
	)
	if r.Candidate.DebtReduction {
		chips = append(chips, Span(css.Class("alloc-dest-tag"), uistate.T("allocate.paysDebtTag")))
	}
	if r.Breakdown.GoalProgress > 0 {
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
					Span(css.Class("alloc-dest-score", tw.TextDim), fmt.Sprintf("%d%%", scorePct)),
				),
			),
			uiw.MeterBar(uiw.MeterBarProps{
				Value: float64(scorePct), Tone: "bg-accent",
				Label: uistate.T("allocate.scoreLabel", float64(scorePct)),
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
					Attr("data-testid", "alloc-exclude-"+r.Candidate.ID), Title(uistate.T("allocate.excludeTitle")),
					OnClick(excl), uistate.T("allocate.exclude")),
			},
		}),
	)
}

// allocBreakdownChip is one criterion contribution as a compact labelled chip (0–100%).
func allocBreakdownChip(label string, frac float64) ui.Node {
	pct := max(0, min(int(frac*100+0.5), 100))
	return Span(css.Class("alloc-dest-chip"),
		Span(css.Class("alloc-dest-chip-label", tw.TextDim), label),
		Span(css.Class("alloc-dest-chip-val"), fmt.Sprintf("%d%%", pct)),
	)
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

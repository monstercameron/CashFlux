//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/allocate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// suggestionListProps carries the ranked candidates and callbacks for the
// suggestion list and excluded-candidates sections.
type suggestionListProps struct {
	Ranked       []allocate.Ranked
	ExcludedRows []ui.Node
	HiddenZero   bool
	AmountFor    func(id string) string
	OnExclude    func(string)
}

// SuggestionList renders the ranked allocation suggestions card and, when
// present, the excluded-candidates restore card. It is a pure rendering
// function — all hooks live in Allocate() or in the AllocRow sub-components.
func SuggestionList(p suggestionListProps) ui.Node {
	ranked := p.Ranked
	var listBody ui.Node
	switch {
	case len(ranked) == 0 && p.HiddenZero:
		listBody = P(css.Class("empty"), uistate.T("allocate.setAttributes"))
	case len(ranked) == 0 && len(p.ExcludedRows) == 0:
		listBody = P(css.Class("empty"), uistate.T("allocate.emptyNoCandidates"))
	case len(ranked) == 0:
		listBody = P(css.Class("empty"), uistate.T("allocate.allExcluded"))
	default:
		rankByID := make(map[string]int, len(ranked))
		for i, r := range ranked {
			rankByID[r.Candidate.ID] = i + 1
		}
		listBody = Div(
			// G8: "Priority" column header so the score % is immediately legible.
			Div(css.Class("alloc-list-header"),
				Span(css.Class("muted"), uistate.T("allocate.priorityHeader")),
			),
			MapKeyed(ranked,
				func(r allocate.Ranked) any { return r.Candidate.ID },
				func(r allocate.Ranked) ui.Node {
					return ui.CreateElement(AllocRow, allocRowProps{R: r, Rank: rankByID[r.Candidate.ID], Amount: p.AmountFor(r.Candidate.ID), OnExclude: p.OnExclude})
				},
			))
	}

	return Fragment(
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("allocate.suggestionsTitle"),
			Body:  listBody,
		}),
		If(len(p.ExcludedRows) > 0, uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("allocate.excludedTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("allocate.excludedDesc")),
				Div(css.Class("rows"), p.ExcludedRows),
			),
		})),
	)
}

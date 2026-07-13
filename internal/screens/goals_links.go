// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// reviewCadenceOptions builds the "Review reminder" SelectOptions for the goal editor —
// an Off default plus the daily…yearly cadences the staleness check understands.
func reviewCadenceOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("goals.reviewNone")},
		{Value: string(domain.CadenceDaily), Label: uistate.T("goals.reviewDaily")},
		{Value: string(domain.CadenceWeekly), Label: uistate.T("goals.reviewWeekly")},
		{Value: string(domain.CadenceMonthly), Label: uistate.T("goals.reviewMonthly")},
		{Value: string(domain.CadenceQuarterly), Label: uistate.T("goals.reviewQuarterly")},
		{Value: string(domain.CadenceYearly), Label: uistate.T("goals.reviewYearly")},
	}
}

// goalLinkRowProps drives one checkbox row in a goal's account/budget link checklist.
type goalLinkRowProps struct {
	ID, Label, Meta string
	Selected        bool
	TestPrefix      string       // e.g. "goal-link-acct" / "goal-link-budget"
	OnToggle        func(string) // plain func — never an On* hook (no-On*-in-loop rule)
}

// goalLinkRow is one checkbox row (account or budget) in the goal editor's multi-link
// checklist. Its own component so the change hook stays at a stable call-site.
func goalLinkRow(props goalLinkRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.ID) })
	return Label(css.Class("goal-link-row"),
		Input(append([]any{Type("checkbox"), Attr("data-testid", props.TestPrefix+"-"+props.ID), OnChange(toggle)}, checkedAttr(props.Selected)...)...),
		Div(css.Class("row-main"),
			Span(props.Label),
			If(props.Meta != "", Span(css.Class("row-meta", tw.TextDim), props.Meta)),
		),
	)
}

// seedLinkSet builds a string-set from a list of ids (empty ids skipped) — used to seed a
// multi-link checklist's UseState map from a goal's stored links.
func seedLinkSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			m[id] = true
		}
	}
	return m
}

// toggleInSet returns a copy of set with id flipped — the immutable update a UseState map
// needs (mutating the stored map in place would not trigger a re-render).
func toggleInSet(set map[string]bool, id string) map[string]bool {
	next := make(map[string]bool, len(set)+1)
	for k, v := range set {
		if v {
			next[k] = true
		}
	}
	if next[id] {
		delete(next, id)
	} else {
		next[id] = true
	}
	return next
}

// sortedSetKeys returns the true keys of a string-set in stable (sorted) order.
func sortedSetKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k, v := range set {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

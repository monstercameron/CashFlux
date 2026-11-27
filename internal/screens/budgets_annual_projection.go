// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/GoWebComponents/v8/ui"
)

// useBudgetAnnualProjection keeps recurring and goal occurrence projection out
// of scenario and visible-column updates. A closed or failed grid computes none.
func useBudgetAnnualProjection(props budgetAnnualGridProps, build *budgetview.AnnualBuild, open bool, revision int) *budgetview.AnnualProjection {
	deps := []any{build, open, revision, props.App, props.Now.Format("2006-01-02"), props.Rates.Base}
	if props.App != nil {
		deps = append(deps, props.App.Rev())
	} else {
		deps = append(deps, props.Recurrings, len(props.Recurrings), props.Goals, len(props.Goals), props.Rates)
	}
	return ui.UseMemo(func() *budgetview.AnnualProjection {
		if !open || build.Err != nil {
			return &budgetview.AnnualProjection{}
		}
		recurrings := props.Recurrings
		goals := props.Goals
		if props.App != nil {
			if len(recurrings) == 0 {
				recurrings = props.App.Recurring()
			}
			if len(goals) == 0 {
				goals = props.App.Goals()
			}
		}
		value := budgetview.ProjectAnnual(build.Grid, build.Covers, recurrings, goals, props.Now, props.Rates)
		return &value
	}, deps...)
}

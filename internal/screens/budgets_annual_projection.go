// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// useBudgetAnnualProjection keeps recurring and goal occurrence projection out
// of scenario and visible-column updates. A closed or failed grid computes none.
func useBudgetAnnualProjection(props budgetAnnualGridProps, build *budgetview.AnnualBuild, open bool, revision int) *budgetview.AnnualProjection {
	return ui.UseMemo(func() *budgetview.AnnualProjection {
		if !open || build.Err != nil {
			return &budgetview.AnnualProjection{}
		}
		value := budgetview.ProjectAnnual(build.Grid, build.Covers, props.Recurrings, props.Goals, props.Now, props.Rates)
		return &value
	}, build, open, revision, props.Recurrings, len(props.Recurrings), props.Goals, len(props.Goals),
		props.Now.Format("2006-01-02"), props.Rates)
}

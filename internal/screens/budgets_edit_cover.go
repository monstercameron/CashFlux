// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetEditCoverProps identifies the row in the shared reviewed cover form.
type budgetEditCoverProps struct{ b domain.Budget }

// budgetEditCover opens the same reviewed form as the overages strip. Recurring
// arrangement creation is deliberately retired: existing imported arrangements
// still run and can be removed, but the former invisible editor is not mounted.
func budgetEditCover(p budgetEditCoverProps) ui.Node {
	return ui.CreateElement(coverAllForm, coverAllFormProps{OnDone: uistate.CloseBudgetEdit, TargetID: p.b.ID})
}

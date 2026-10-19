// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditCoverProps preserves the editor host inputs while routing its row action
// into the same cover form used by the overages strip.
type budgetEditCoverProps struct {
	coverSrcs         []budgetCoverSource
	coverIncomplete   bool
	app               *appstate.App
	pr                prefs.Prefs
	coverPayAnchor    time.Time
	amtFxS            ui.State[bool]
	amtFormulaS       ui.State[string]
	b                 domain.Budget
	cur               string
	coverAmtS         ui.State[string]
	dec               int
	coverWtS          ui.State[map[string]int]
	wtFxS             ui.State[bool]
	coverSelS         ui.State[map[string]bool]
	wtFormulaS        ui.State[string]
	coverMaxS         ui.State[map[string]bool]
	onToggleSrc       func(string)
	onWeightSrc       func(string, int)
	onToggleMax       func(string)
	submitCover       ui.Handler
	coverShortfallStr string
	onAmtFormula      ui.Handler
	onCoverAmt        ui.Handler
	coverDefaultStr   string
	fullCover         ui.Handler
	toggleAmtFx       ui.Handler
	toggleWtFx        ui.Handler
	onWtFormula       ui.Handler
	onToggleRecurring ui.Handler
	recurringS        ui.State[bool]
	coverCustomVals   ui.State[map[string]string]
	onCoverCustom     func(string, string)
	errLine           ui.Node
	cancel            ui.Handler
}

// budgetEditCover opens the shared reviewed cover flow focused on this row's budget.
func budgetEditCover(p budgetEditCoverProps) ui.Node {
	return ui.CreateElement(coverAllForm, coverAllFormProps{
		OnDone:   uistate.CloseBudgetEdit,
		TargetID: p.b.ID,
	})
}

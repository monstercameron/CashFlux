// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// runDueScheduledWorkflowsOnBoot runs every enabled scheduled workflow whose
// NextRun has passed as of boot time, advances each schedule, and forces an
// immediate re-save so the advanced NextRun survives a reload.
func runDueScheduledWorkflowsOnBoot() {
	app := appstate.Default
	if app == nil {
		return
	}
	now := time.Now()
	n, err := app.RunDueScheduledWorkflows(now)
	if err != nil {
		app.Log().Error("boot run of due scheduled workflows failed", "err", err)
		return
	}
	if n > 0 {
		app.Log().Info("ran due scheduled workflows on boot", "count", n)
		if resaveDataset != nil {
			resaveDataset()
		}
	}

	nf, err := app.RunDueFundAccruals(now)
	if err != nil {
		app.Log().Error("boot run of sinking-fund accruals failed", "err", err)
		return
	}
	if nf > 0 {
		app.Log().Info("auto-accrued sinking funds on boot", "count", nf)
		if resaveDataset != nil {
			resaveDataset()
		}
	}

	// C184: monthly surplus sweep — move leftover balance above the configured
	// buffer from the source account to the savings destination, once per month.
	p := uistate.LoadPrefs()
	cfg := appstate.SweepConfigFromPrefs(p)
	ns, updatedPrefs, err := app.RunDueSweeps(now, cfg, p)
	if err != nil {
		app.Log().Error("boot run of surplus sweep failed", "err", err)
		return
	}
	if ns > 0 {
		app.Log().Info("surplus sweep executed on boot", "count", ns)
		// Persist the updated SweepLastPeriod guard so the sweep doesn't repeat
		// in this calendar month even after a reload.
		uistate.PersistPrefs(updatedPrefs)
		if resaveDataset != nil {
			resaveDataset()
		}
	}

	// C183: monthly round-up batch — sum each expense's round-up delta over
	// the calendar month and move the total to savings in one transfer, once
	// per month.
	nr, updatedPrefs2, err := app.RunDueRoundUps(now, updatedPrefs)
	if err != nil {
		app.Log().Error("boot run of round-up batch failed", "err", err)
		return
	}
	if nr > 0 {
		app.Log().Info("round-up batch executed on boot", "count", nr)
		// Persist the updated RoundUpLastPeriod guard.
		uistate.PersistPrefs(updatedPrefs2)
		if resaveDataset != nil {
			resaveDataset()
		}
	}
}

// fireBillDueTriggerOnBoot fires bill-due workflows once on startup if any
// recurring item is currently on or past its due date.
func fireBillDueTriggerOnBoot() {
	app := appstate.Default
	if app == nil {
		return
	}
	app.FireBillDueTrigger(time.Now())
}

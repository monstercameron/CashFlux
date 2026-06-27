// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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

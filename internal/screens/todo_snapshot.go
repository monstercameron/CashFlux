// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/taskview"
)

// The full task surface shares one immutable snapshot. Retain only the current
// app/revision/day so a large task or entity index cannot accumulate per filter.
var taskSnapshotCache = newViewCache[taskview.Snapshot](1)

func taskSnapshot(app *appstate.App, today string) taskview.Snapshot {
	return memoByRev(taskSnapshotCache, revKey(app)+"|"+today, func() taskview.Snapshot {
		if app == nil {
			return taskview.BuildSnapshot(taskview.SnapshotInput{TodayISO: today})
		}
		return taskview.BuildSnapshot(taskview.SnapshotInput{
			Tasks: app.Tasks(), Accounts: app.Accounts(), Budgets: app.Budgets(), Goals: app.Goals(),
			Transactions: app.Transactions(), Members: app.Members(), TodayISO: today,
		})
	})
}

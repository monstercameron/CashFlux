//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

// postDueRecurringOnBoot posts every autopost recurring whose NextDue has passed,
// catching up missed periods, then forces an immediate dataset re-save so the
// advanced schedule and the newly posted transactions survive a reload. It is
// idempotent across reopens because PostDueRecurring advances each schedule's
// NextDue past today before persisting. Called once during boot, after autosave
// is armed.
func postDueRecurringOnBoot() {
	app := appstate.Default
	if app == nil {
		return
	}
	n, err := app.PostDueRecurring(time.Now())
	if err != nil {
		app.Log().Error("boot auto-post of due recurring failed", "err", err)
		return
	}
	if n > 0 {
		app.Log().Info("auto-posted due recurring on boot", "count", n)
		if resaveDataset != nil {
			resaveDataset()
		}
	}
}

package appstate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/store"
)

// StartPayoffTracking snapshots the current total debt owed as the payoff baseline,
// so progress can be measured against it from here on.
func (a *App) StartPayoffTracking(totalOwed int64, currency string) error {
	s := a.Settings()
	s.PayoffBaseline = &store.PayoffBaseline{TotalOwed: totalOwed, Currency: currency, StartedAt: time.Now()}
	return a.PutSettings(s)
}

// ClearPayoffTracking removes the payoff baseline (stop tracking / start over).
func (a *App) ClearPayoffTracking() error {
	s := a.Settings()
	s.PayoffBaseline = nil
	return a.PutSettings(s)
}

// PayoffProgress returns progress against the stored baseline given the current
// total owed, plus the date tracking started and whether tracking is active.
func (a *App) PayoffProgress(currentOwed int64) (prog payoff.Progress, since time.Time, tracking bool) {
	b := a.Settings().PayoffBaseline
	if b == nil {
		return payoff.Progress{}, time.Time{}, false
	}
	return payoff.TrackProgress(b.TotalOwed, currentOwed), b.StartedAt, true
}

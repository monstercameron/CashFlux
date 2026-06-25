// SPDX-License-Identifier: MIT

package runway

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// ProjectLiquid is a semantically-named wrapper over Project that makes the
// liquid-cash contract explicit: startBal is the user's current liquid balance
// (cash + checking — not total net worth or assets), and horizon is the number
// of days to project (typically to the next payday, from NextPaydayHorizon).
//
// The buffer is a safety floor below which the balance is considered at-risk
// (e.g. buffer=0 flags an overdraft; buffer=50000 flags dropping below $500).
//
// Returns (cashflow.Projection, error) consistent with Project; callers must
// check the error before using the projection.
func ProjectLiquid(liquidStart int64, recs []domain.Recurring, from time.Time, horizon int, buffer int64, rates currency.Rates) (cashflow.Projection, error) {
	return Project(liquidStart, recs, from, horizon, buffer, rates)
}

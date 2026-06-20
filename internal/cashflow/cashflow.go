// Package cashflow projects an account's running balance forward day by day from
// known upcoming events (bills due, paychecks arriving) and flags the first day it
// would dip below a buffer — the "you'll overdraft on Jul 2" safety net for living
// close to the edge. Deterministic and explainable: every figure comes from the
// supplied events.
//
// Amounts are integer minor units (inflows positive, outflows negative). Pure Go,
// no platform dependencies; unit-tested on native Go.
package cashflow

// Event is a dated cash movement on a day offset from the projection start
// (0 = today, 1 = tomorrow, …). Positive Amount is an inflow (income); negative is
// an outflow (a bill).
type Event struct {
	Day    int
	Amount int64
	Label  string
}

// DailyBalance is the projected end-of-day balance for one day.
type DailyBalance struct {
	Day     int
	Balance int64
}

// Projection is the forward daily cash-flow result for one account.
type Projection struct {
	Daily           []DailyBalance // end-of-day balance for each day [0, days)
	MinBalance      int64          // lowest projected balance over the horizon
	MinDay          int            // first day MinBalance occurs
	BreachDay       int            // first day the balance falls below buffer; -1 if never
	BreachShortfall int64          // buffer minus the balance at BreachDay (>0); 0 when no breach
}

// WillBreach reports whether the balance dips below the buffer within the horizon.
func (p Projection) WillBreach() bool { return p.BreachDay >= 0 }

// DailyBalances projects the running balance over `days` days from startBal,
// applying each event on its day (end-of-day), and flags the first day the balance
// falls below buffer (e.g. buffer 0 = overdraft). Events outside [0, days) are
// ignored. A non-positive horizon yields an empty projection with BreachDay -1.
func DailyBalances(startBal int64, events []Event, days int, buffer int64) Projection {
	p := Projection{BreachDay: -1}
	if days <= 0 {
		p.MinBalance = startBal
		return p
	}
	p.Daily = make([]DailyBalance, days)
	bal := startBal
	p.MinBalance = startBal
	p.MinDay = 0
	for d := 0; d < days; d++ {
		for _, e := range events {
			if e.Day == d {
				bal += e.Amount
			}
		}
		p.Daily[d] = DailyBalance{Day: d, Balance: bal}
		if bal < p.MinBalance {
			p.MinBalance = bal
			p.MinDay = d
		}
		if p.BreachDay < 0 && bal < buffer {
			p.BreachDay = d
			p.BreachShortfall = buffer - bal
		}
	}
	return p
}

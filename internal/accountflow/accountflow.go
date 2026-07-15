// SPDX-License-Identifier: MIT

// Package accountflow derives per-account, list-level read models from an
// account and its transactions: the money-in / money-out / net flow for a period
// (AC9) and a date-bounded running-balance series for a row sparkline (AC2).
//
// Both are cheap folds over the account's own transactions, sharing the balance
// conventions in internal/ledger (opening balance + booked transactions, in the
// account's currency). Transfers are NEVER counted as income or spending — a
// move between your own accounts is not cash flow — so the flow figures answer
// "which account is bleeding?" without a transfer masquerading as a paycheck.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package accountflow

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Flow is an account's money-in / money-out / net for a period, in the account's
// own currency (minor units). In and Out are non-negative magnitudes; Net = In −
// Out (can be negative when the account bled). Transfers are excluded from all
// three — they are counted separately by the caller, never as income or spend.
type Flow struct {
	In       money.Money
	Out      money.Money
	Net      money.Money
	Transfer money.Money // signed net of transfers touching this account (in − out), for reference
}

// PeriodFlow folds the account's non-transfer transactions dated in the half-open
// range [start, end) into money-in (positive amounts), money-out (magnitude of
// negative amounts), and net. Only transactions booked on the account
// (AccountID == account.ID) participate. Transfers are tallied into Transfer
// (signed, in − out) but kept out of In/Out/Net so a transfer never reads as
// income or spending. All amounts are in the account's currency.
func PeriodFlow(account domain.Account, all []domain.Transaction, start, end time.Time) Flow {
	cur := account.Currency
	in := money.Zero(cur)
	out := money.Zero(cur)
	xfer := money.Zero(cur)
	for _, t := range all {
		if t.AccountID != account.ID || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if t.IsTransfer() {
			xfer.Amount += t.Amount.Amount
			continue
		}
		if t.Amount.Amount > 0 {
			in.Amount += t.Amount.Amount
		} else {
			out.Amount += -t.Amount.Amount
		}
	}
	return Flow{In: in, Out: out, Net: money.New(in.Amount-out.Amount, cur), Transfer: xfer}
}

// BalanceSeries returns the account's end-of-day balance for each of the last
// `days` calendar days ending on asOf (inclusive), oldest first — a date-bounded
// running-balance fold suitable for a row sparkline. Each point is the opening
// balance plus every transaction booked on the account on or before that day, in
// the account's currency (minor units). The series always has exactly `days`
// points (days ≤ 0 yields nil); a flat run of points is itself the signal that
// nothing has posted since the last update.
//
// Only the account's own transactions participate (matching ledger.Balance); a
// transaction in a different currency is added by its minor amount as-is, so
// callers should reserve the sparkline for single-currency accounts.
func BalanceSeries(account domain.Account, all []domain.Transaction, asOf time.Time, days int) []int64 {
	if days <= 0 {
		return nil
	}
	// Day boundaries: the series covers [firstDay .. lastDay], one point per day.
	lastDay := dateutil.DayStart(asOf)
	firstDay := lastDay.AddDate(0, 0, -(days - 1))

	// Account's own transactions, oldest first (ties by ID for determinism).
	mine := make([]domain.Transaction, 0, len(all))
	for _, t := range all {
		if t.AccountID == account.ID {
			mine = append(mine, t)
		}
	}
	sort.SliceStable(mine, func(i, j int) bool {
		if mine[i].Date.Equal(mine[j].Date) {
			return mine[i].ID < mine[j].ID
		}
		return mine[i].Date.Before(mine[j].Date)
	})

	running := account.OpeningBalance.Amount
	series := make([]int64, days)
	ti := 0
	for d := 0; d < days; d++ {
		// The point's day (inclusive end-of-day cutoff).
		dayEnd := firstDay.AddDate(0, 0, d+1) // start of the next day = exclusive upper bound
		for ti < len(mine) && mine[ti].Date.Before(dayEnd) {
			running += mine[ti].Amount.Amount
			ti++
		}
		series[d] = running
	}
	return series
}

// Polyline maps a numeric series to an SVG polyline "points" string fitted into a
// box of the given width and height (a small top/bottom padding keeps the stroke
// off the edges). The series is scaled to its own min/max, so a flat series draws
// as a centered horizontal line rather than clipping. Fewer than two points yields
// an empty string (nothing to draw). Pure geometry — no rendering, so it is
// unit-tested without a DOM.
func Polyline(series []int64, width, height, pad float64) string {
	n := len(series)
	if n < 2 || width <= 0 || height <= 0 {
		return ""
	}
	min, max := series[0], series[0]
	for _, v := range series {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := float64(max - min)
	usableH := height - 2*pad
	if usableH < 0 {
		usableH = 0
	}
	stepX := width / float64(n-1)

	var b []byte
	for i, v := range series {
		x := float64(i) * stepX
		// y grows downward in SVG, so a larger value must sit higher (smaller y).
		var y float64
		if span == 0 {
			y = pad + usableH/2 // flat line, centered
		} else {
			frac := float64(v-min) / span
			y = pad + (1-frac)*usableH
		}
		if i > 0 {
			b = append(b, ' ')
		}
		b = appendFixed(b, x)
		b = append(b, ',')
		b = appendFixed(b, y)
	}
	return string(b)
}

// appendFixed appends f to b with two decimal places (enough for pixel geometry)
// without pulling in fmt, keeping Polyline allocation-light.
func appendFixed(b []byte, f float64) []byte {
	// Round to hundredths.
	scaled := int64(f*100 + 0.5)
	if f < 0 {
		scaled = int64(f*100 - 0.5)
		b = append(b, '-')
		scaled = -scaled
	}
	whole := scaled / 100
	frac := scaled % 100
	b = appendInt(b, whole)
	b = append(b, '.')
	b = append(b, byte('0'+frac/10), byte('0'+frac%10))
	return b
}

func appendInt(b []byte, n int64) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for n > 0 {
		i--
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	return append(b, tmp[i:]...)
}

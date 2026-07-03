// SPDX-License-Identifier: MIT

// Package billsched aligns WHEN bills get paid with the pay cycle. Two honest,
// distinct levers:
//
//  1. PAY-AHEAD (the smart schedule): a due date is a deadline, not an
//     instruction — you can always pay EARLIER. Paying earlier can never raise
//     the minimum of the balance curve (the cash just leaves sooner), so the
//     pay-ahead objective is NOT the low point: it is evening the load each
//     paycheck carries, so every check covers its own bills and no check is
//     "free" (the one people overspend). Constraint: the plan never pushes the
//     projected low below the raw schedule's low (or below the configured keep
//     floor), and never schedules after the due date.
//
//  2. BILLER-SIDE SUGGESTIONS: moving a due date LATER — just past a payday —
//     is what genuinely lifts the low point, and only the biller can do it.
//     Suggest returns the shifts worth asking for, each with its measured gain.
//     Autopay bills (the biller pulls on the due date) can only be helped here.
//
// Deterministic and explainable: greedy placement over candidate paydays,
// scored by simulating the daily balance curve — no black boxes. Pure Go, no
// syscall/js.
package billsched

import (
	"sort"
	"time"
)

// Item is one schedulable obligation inside the horizon.
type Item struct {
	ID      string // stable identity (e.g. account|due|name)
	Name    string
	Amount  int64     // amount owed, positive minor units
	Due     time.Time // the raw deadline (never pay after this)
	Movable bool      // false when the biller controls the charge date (autopay)
}

// Move pairs an item with its recommended pay-on date (strictly before Due —
// unmoved items are not reported as moves).
type Move struct {
	Item  Item
	PayOn time.Time
}

// PeriodLoad is the billed total assigned to one pay period (the span from a
// payday up to the next).
type PeriodLoad struct {
	Payday time.Time
	Total  int64
}

// Metrics summarizes one schedule's simulated balance curve.
type Metrics struct {
	Low     int64 // minimum projected balance over the horizon
	LowDate time.Time
	Loads   []PeriodLoad
}

// Suggestion is one biller-side due-date shift worth asking for: moving the due
// date to just after a payday, with the measured low-point improvement.
type Suggestion struct {
	Item         Item
	NewDue       time.Time
	LowGainMinor int64 // how much the projected low rises with this one shift
}

// Result is the optimizer's output: the raw-schedule metrics, the pay-ahead
// smart-schedule metrics, the recommended moves, the heaviest-paycheck
// improvement, and any biller-side suggestions.
type Result struct {
	Raw   Metrics
	Smart Metrics
	Moves []Move
	// EvenGainMinor is how much lighter the heaviest pay period got under the
	// smart plan (maxLoad(raw) − maxLoad(smart), ≥ 0) — the pay-ahead headline.
	EvenGainMinor int64
	// Suggestions are biller-side due-date shifts (the lever that can actually
	// lift the low point), best gain first.
	Suggestions []Suggestion
	// PayOnByID maps every item ID to its scheduled date under the smart plan
	// (= Due for unmoved items), so callers can render the full schedule.
	PayOnByID map[string]time.Time
}

// Paydays generates the paydays covering [from, from+horizonDays] from a known
// anchor payday and a frequency: "weekly", "biweekly", "semimonthly" (the
// anchor's day and that day +15, clamped into the month), or "monthly" (the
// anchor's day each month). The anchor may be in the past or future; the cycle
// is stepped onto the window. An unknown frequency defaults to biweekly. A zero
// anchor yields no paydays (the caller should treat that as "not configured").
func Paydays(anchor time.Time, freq string, from time.Time, horizonDays int) []time.Time {
	if anchor.IsZero() || horizonDays <= 0 {
		return nil
	}
	from = midnight(from)
	end := from.AddDate(0, 0, horizonDays)
	anchor = midnight(anchor)

	var out []time.Time
	add := func(t time.Time) {
		if !t.Before(from) && !t.After(end) {
			out = append(out, t)
		}
	}
	switch freq {
	case "monthly":
		day := anchor.Day()
		for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()); !m.After(end); m = m.AddDate(0, 1, 0) {
			add(clampDay(m.Year(), m.Month(), day, from.Location()))
		}
	case "semimonthly":
		d1 := anchor.Day()
		d2 := d1 + 15
		if d2 > 28 {
			d2 -= 28 // keep both dates inside every month (e.g. anchor 20th → 20th + 7th)
		}
		for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()); !m.After(end); m = m.AddDate(0, 1, 0) {
			add(clampDay(m.Year(), m.Month(), d1, from.Location()))
			add(clampDay(m.Year(), m.Month(), d2, from.Location()))
		}
	case "weekly":
		out = stepDays(anchor, 7, from, end)
	default: // biweekly
		out = stepDays(anchor, 14, from, end)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return dedupe(out)
}

// stepDays walks a fixed-interval cycle from the anchor onto [from, end].
func stepDays(anchor time.Time, every int, from, end time.Time) []time.Time {
	t := anchor
	for t.After(from) {
		t = t.AddDate(0, 0, -every)
	}
	for t.Before(from) {
		t = t.AddDate(0, 0, every)
	}
	var out []time.Time
	for !t.After(end) {
		out = append(out, t)
		t = t.AddDate(0, 0, every)
	}
	return out
}

// clampDay returns year/month at the given day, clamped to the month's length.
func clampDay(y int, m time.Month, day int, loc *time.Location) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	if day > last {
		day = last
	}
	return time.Date(y, m, day, 0, 0, 0, 0, loc)
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dedupe(ts []time.Time) []time.Time {
	out := ts[:0]
	for i, t := range ts {
		if i == 0 || !t.Equal(ts[i-1]) {
			out = append(out, t)
		}
	}
	return out
}

// Optimize builds the pay-ahead smart schedule. startLiquid is today's liquid
// cash (minor units); incomePerPayday is the expected net deposit landing on
// each payday; minKeep is a floor the plan must not dip below (it also never
// goes below the raw schedule's own low — paying ahead must not create a crunch
// that didn't exist). The simulation covers [from, from+horizonDays], crediting
// income on paydays before debiting same-day bills. Items are never scheduled
// after their due date or before `from`. With no paydays the raw schedule is
// returned unchanged (nothing to align to).
func Optimize(startLiquid int64, items []Item, paydays []time.Time, incomePerPayday int64, from time.Time, horizonDays int, minKeep int64) Result {
	from = midnight(from)
	assign := map[string]time.Time{}
	for _, it := range items {
		assign[it.ID] = clampToWindow(it.Due, from, horizonDays)
	}
	raw := simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)

	res := Result{Raw: raw, Smart: raw, PayOnByID: assign}
	if len(paydays) == 0 || len(items) == 0 {
		res.Suggestions = Suggest(startLiquid, items, paydays, incomePerPayday, from, horizonDays)
		return res
	}

	// The plan may never make the low point worse than the raw schedule already
	// is, nor dip below the keep floor when the raw schedule respects it.
	floor := raw.Low
	if minKeep > 0 && minKeep < floor {
		floor = minKeep
	}

	// Largest bills first — they dominate the loads, so placing them first gives
	// the greedy pass the most leverage. Ties break by ID for determinism.
	movable := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Movable {
			movable = append(movable, it)
		}
	}
	sort.SliceStable(movable, func(i, j int) bool {
		if movable[i].Amount != movable[j].Amount {
			return movable[i].Amount > movable[j].Amount
		}
		return movable[i].ID < movable[j].ID
	})

	// Two greedy passes: the second lets early placements re-settle. Each item
	// takes the candidate payday (≤ due; paying ON a payday is safe because
	// income credits before same-day bills) that most evens the heaviest
	// pay-period load — staying as late as possible on ties — subject to the
	// low-point floor.
	for pass := 0; pass < 2; pass++ {
		for _, it := range movable {
			due := clampToWindow(it.Due, from, horizonDays)
			candidates := []time.Time{due}
			for _, p := range paydays {
				if !p.After(due) && !p.Before(from) {
					candidates = append(candidates, p)
				}
			}
			best := assign[it.ID]
			bestM := simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)
			for _, c := range candidates {
				if c.Equal(best) {
					continue
				}
				prev := assign[it.ID]
				assign[it.ID] = c
				m := simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)
				if m.Low >= floor && evener(m, bestM, c, best) {
					best, bestM = c, m
				} else {
					assign[it.ID] = prev
				}
			}
		}
	}

	res.Smart = simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)
	res.PayOnByID = assign
	// Keep the plan only when it genuinely evens the paycheck loads — compared
	// as the whole sorted load vector, not just the single global max: over a
	// multi-month horizon two months can BOTH have a heavy paycheck, and evening
	// one of them is real progress even when the other's stack can't move (e.g.
	// it's all autopay). Otherwise report no moves — the honest "you're already
	// even" answer.
	if !lessLoads(res.Smart.Loads, raw.Loads) {
		for _, it := range items {
			res.PayOnByID[it.ID] = clampToWindow(it.Due, from, horizonDays)
		}
		res.Smart = raw
	} else {
		res.EvenGainMinor = maxLoad(raw.Loads) - maxLoad(res.Smart.Loads)
		for _, it := range items {
			if p := res.PayOnByID[it.ID]; it.Movable && p.Before(clampToWindow(it.Due, from, horizonDays)) {
				res.Moves = append(res.Moves, Move{Item: it, PayOn: p})
			}
		}
		sort.SliceStable(res.Moves, func(i, j int) bool { return res.Moves[i].PayOn.Before(res.Moves[j].PayOn) })
	}
	res.Suggestions = Suggest(startLiquid, items, paydays, incomePerPayday, from, horizonDays)
	return res
}

// evener reports whether metrics a (with candidate date da) beat metrics b
// (holding date db) for the pay-ahead objective: a more even load spread
// (lexicographically smaller sorted-descending load vector — lighter heaviest
// period, then lighter second-heaviest, …), then a higher low point, then the
// later date (keep money longer). Comparing the whole vector rather than the
// single max is what lets the greedy pass even out month B while month A still
// holds the global maximum — otherwise a two-heavy-month horizon deadlocks
// (no single move improves the max, so no move is ever accepted).
func evener(a, b Metrics, da, db time.Time) bool {
	if av, bv := loadVector(a.Loads), loadVector(b.Loads); !vectorsEqual(av, bv) {
		return lessVector(av, bv)
	}
	if a.Low != b.Low {
		return a.Low > b.Low
	}
	return da.After(db)
}

// loadVector returns the period totals sorted descending — the canonical shape
// for "how even is this schedule".
func loadVector(loads []PeriodLoad) []int64 {
	v := make([]int64, len(loads))
	for i, l := range loads {
		v[i] = l.Total
	}
	sort.Slice(v, func(i, j int) bool { return v[i] > v[j] })
	return v
}

// lessLoads reports whether load set a is strictly more even than b.
func lessLoads(a, b []PeriodLoad) bool {
	return lessVector(loadVector(a), loadVector(b))
}

// lessVector is the lexicographic order on sorted-descending load vectors: a
// beats b when its first differing (heaviest-first) entry is smaller. A missing
// trailing entry counts as zero, so fewer loaded periods beats more.
func lessVector(a, b []int64) bool {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	at := func(v []int64, i int) int64 {
		if i < len(v) {
			return v[i]
		}
		return 0
	}
	for i := 0; i < n; i++ {
		if av, bv := at(a, i), at(b, i); av != bv {
			return av < bv
		}
	}
	return false
}

func vectorsEqual(a, b []int64) bool {
	return !lessVector(a, b) && !lessVector(b, a)
}

// Suggest finds biller-side due-date shifts that lift the projected low point:
// for each bill due in the seven days BEFORE a payday, it measures moving the
// due date to the day after that payday. Only strictly-improving shifts are
// returned, best gain first (capped at five so the list stays actionable).
// This is the only lever that helps autopay bills.
func Suggest(startLiquid int64, items []Item, paydays []time.Time, incomePerPayday int64, from time.Time, horizonDays int) []Suggestion {
	if len(paydays) == 0 || len(items) == 0 {
		return nil
	}
	from = midnight(from)
	assign := map[string]time.Time{}
	for _, it := range items {
		assign[it.ID] = clampToWindow(it.Due, from, horizonDays)
	}
	raw := simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)

	var out []Suggestion
	for _, it := range items {
		due := clampToWindow(it.Due, from, horizonDays)
		var best *Suggestion
		for _, p := range paydays {
			gap := int(p.Sub(due).Hours() / 24)
			if gap <= 0 || gap > 7 {
				continue // only bills due shortly BEFORE a payday benefit
			}
			shifted := p.AddDate(0, 0, 1)
			if shifted.After(from.AddDate(0, 0, horizonDays)) {
				continue
			}
			prev := assign[it.ID]
			assign[it.ID] = shifted
			m := simulate(startLiquid, items, assign, paydays, incomePerPayday, from, horizonDays)
			assign[it.ID] = prev
			if gain := m.Low - raw.Low; gain > 0 && (best == nil || gain > best.LowGainMinor) {
				best = &Suggestion{Item: it, NewDue: shifted, LowGainMinor: gain}
			}
		}
		if best != nil {
			out = append(out, *best)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LowGainMinor != out[j].LowGainMinor {
			return out[i].LowGainMinor > out[j].LowGainMinor
		}
		return out[i].Item.ID < out[j].Item.ID
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func maxLoad(loads []PeriodLoad) int64 {
	var m int64
	for _, l := range loads {
		if l.Total > m {
			m = l.Total
		}
	}
	return m
}

// clampToWindow keeps a date inside [from, from+horizonDays] (bills already due
// before `from` are treated as payable today).
func clampToWindow(t time.Time, from time.Time, horizonDays int) time.Time {
	t = midnight(t)
	if t.Before(from) {
		return from
	}
	if end := from.AddDate(0, 0, horizonDays); t.After(end) {
		return end
	}
	return t
}

// simulate walks the daily balance over the horizon: income lands on paydays
// first, then that day's scheduled bills debit. Returns the curve's minimum (and
// its date) plus the billed load per pay period.
func simulate(start int64, items []Item, assign map[string]time.Time, paydays []time.Time, incomePerPayday int64, from time.Time, horizonDays int) Metrics {
	end := from.AddDate(0, 0, horizonDays)
	dayKey := func(t time.Time) string { return t.Format("2006-01-02") }

	income := map[string]int64{}
	for _, p := range paydays {
		if !p.Before(from) && !p.After(end) {
			income[dayKey(p)] += incomePerPayday
		}
	}
	debits := map[string]int64{}
	for _, it := range items {
		debits[dayKey(clampToWindow(assign[it.ID], from, horizonDays))] += it.Amount
	}

	m := Metrics{Low: start, LowDate: from}
	bal := start
	for d := 0; d <= horizonDays; d++ {
		day := from.AddDate(0, 0, d)
		k := dayKey(day)
		bal += income[k]
		bal -= debits[k]
		if bal < m.Low {
			m.Low, m.LowDate = bal, day
		}
	}

	// Load per pay period: each bill belongs to the latest payday on-or-before its
	// pay date (bills before the first payday belong to a leading pseudo-period
	// anchored at `from`).
	sorted := append([]time.Time(nil), paydays...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	loads := []PeriodLoad{}
	periodFor := func(t time.Time) time.Time {
		anchor := from
		for _, p := range sorted {
			if !p.After(t) {
				anchor = p
			}
		}
		return anchor
	}
	byPeriod := map[string]*PeriodLoad{}
	for _, it := range items {
		p := periodFor(clampToWindow(assign[it.ID], from, horizonDays))
		k := dayKey(p)
		if byPeriod[k] == nil {
			byPeriod[k] = &PeriodLoad{Payday: p}
		}
		byPeriod[k].Total += it.Amount
	}
	for _, l := range byPeriod {
		loads = append(loads, *l)
	}
	sort.Slice(loads, func(i, j int) bool { return loads[i].Payday.Before(loads[j].Payday) })
	m.Loads = loads
	return m
}

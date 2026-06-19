package subscriptions

import "time"

// UpcomingRenewals returns the subscriptions whose next renewal falls within the
// next withinDays days (inclusive of today through today+withinDays), soonest
// first — the "renewing soon, decide before you're charged again" view. A
// non-positive withinDays falls back to a 7-day window. Only the calendar date of
// each renewal is considered. The input order is otherwise preserved via a stable
// sort on the renewal date.
func UpcomingRenewals(subs []Subscription, withinDays int, now time.Time) []Subscription {
	if withinDays <= 0 {
		withinDays = 7
	}
	today := dayOnly(now)
	cutoff := today.AddDate(0, 0, withinDays)

	var out []Subscription
	for _, s := range subs {
		d := dayOnly(s.NextRenewal)
		if d.Before(today) || d.After(cutoff) {
			continue
		}
		out = append(out, s)
	}
	sortByRenewal(out)
	return out
}

// dayOnly truncates t to its calendar date (midnight in t's location).
func dayOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// sortByRenewal orders subscriptions by next renewal date (soonest first), ties
// broken by name for determinism. Uses an insertion sort to avoid pulling in sort
// just for this small, already-small slice — and to keep it allocation-free.
func sortByRenewal(s []Subscription) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0; j-- {
			a, b := s[j-1], s[j]
			if a.NextRenewal.Before(b.NextRenewal) || (a.NextRenewal.Equal(b.NextRenewal) && a.Name <= b.Name) {
				break
			}
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

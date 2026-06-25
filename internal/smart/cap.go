// SPDX-License-Identifier: MIT

package smart

// CapPerRule limits each distinct insight Code (Feature field) to at most n
// entries, preserving severity order (highest first). Input must already be
// severity-sorted (highest Severity value first). Insights from each rule are
// capped to the first n seen in iteration order — because input is
// severity-sorted, that keeps the highest-severity findings. A non-positive n
// is treated as 0 (all capped away). Input is never mutated.
func CapPerRule(insights []Insight, n int) []Insight {
	if n <= 0 {
		return nil
	}
	seen := make(map[string]int, len(insights))
	out := make([]Insight, 0, len(insights))
	for _, ins := range insights {
		if seen[ins.Feature] < n {
			out = append(out, ins)
			seen[ins.Feature]++
		}
	}
	return out
}

// EnableFreeOnly enables all Free-tier features and leaves AI-tier features
// untouched (off by tier default, unless the user has already explicitly
// enabled them). Any feature already in ExplicitOff with TierFree is cleared
// so the free tier-default ("on") re-applies. Features that were already
// explicitly on stay on.
func EnableFreeOnly(s Settings) Settings {
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	for _, f := range catalog {
		if f.Tier != TierFree {
			continue
		}
		// Turn on: record in Enabled, clear any explicit-off record.
		s.Enabled[f.Code] = true
		delete(s.ExplicitOff, f.Code)
	}
	return s
}

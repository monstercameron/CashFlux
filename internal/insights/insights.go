// Package insights derives spending trend and anomaly highlights from per-category
// spend histories: it compares a category's current-period spend against the
// trailing average of its prior periods and flags material deviations. The result
// is the data behind the "what changed this month" highlights — pure and
// explainable (every anomaly carries its baseline, delta, and percent change).
//
// Amounts are integer minor units (e.g. cents); spend is expressed as a positive
// magnitude. Pure Go, no platform dependencies; unit-tested on native Go.
package insights

import "sort"

// CategorySeries is one category's spend across consecutive periods, oldest first.
// The final element is the current period; the earlier elements form the baseline.
type CategorySeries struct {
	Category string
	// Spend per period in minor units, oldest → newest, as positive magnitudes.
	Spend []int64
}

// Direction says whether the current period rose above or fell below the baseline.
type Direction int

const (
	// Up means the current period spent more than the trailing baseline.
	Up Direction = iota
	// Down means the current period spent less than the trailing baseline.
	Down
)

// String renders the direction for display/debugging.
func (d Direction) String() string {
	if d == Down {
		return "down"
	}
	return "up"
}

// Anomaly flags a category whose current-period spend deviates materially from its
// trailing average. The baseline excludes the current period, so it answers "how
// does this period compare to how this category normally behaves".
type Anomaly struct {
	Category  string
	Current   int64     // current-period spend (minor units)
	Baseline  int64     // trailing average over the prior periods (minor units)
	Delta     int64     // Current - Baseline (signed)
	PctChange int64     // whole-percent change vs the baseline magnitude
	Direction Direction // Up when Delta > 0, Down when Delta < 0
}

// Options tunes anomaly detection. The zero value is usable; DefaultOptions
// supplies sensible non-zero defaults.
type Options struct {
	// MinPeriods is how many prior (baseline) periods a category must have before
	// it can be evaluated. Values below 1 are treated as 1.
	MinPeriods int
	// MinBaseline is a noise floor: categories whose baseline magnitude is below
	// this are ignored, so a jump from $0.50 to $1.00 doesn't read as "+100%".
	MinBaseline int64
	// ThresholdPct is the minimum absolute percent change (vs the baseline) that
	// counts as an anomaly. Values below 1 are treated as 1.
	ThresholdPct int64
}

// DefaultOptions flags categories that moved at least 50% versus a baseline of at
// least two prior periods, ignoring baselines under 10.00 (1000 minor units).
func DefaultOptions() Options {
	return Options{MinPeriods: 2, MinBaseline: 1000, ThresholdPct: 50}
}

// normalize clamps the options to safe minimums.
func (o Options) normalize() Options {
	if o.MinPeriods < 1 {
		o.MinPeriods = 1
	}
	if o.ThresholdPct < 1 {
		o.ThresholdPct = 1
	}
	if o.MinBaseline < 0 {
		o.MinBaseline = 0
	}
	return o
}

// Detect returns the categories whose current-period spend deviates from their
// trailing average by at least the threshold, most significant first (largest
// absolute delta, ties broken by category name). A category is skipped when it has
// fewer than MinPeriods baseline periods or its baseline magnitude is below the
// noise floor (so the percent change is always meaningful).
func Detect(series []CategorySeries, opts Options) []Anomaly {
	opts = opts.normalize()
	var out []Anomaly
	for _, s := range series {
		if len(s.Spend) < opts.MinPeriods+1 {
			continue
		}
		prior := s.Spend[:len(s.Spend)-1]
		current := s.Spend[len(s.Spend)-1]
		baseline := mean(prior)
		mag := baseline
		if mag < 0 {
			mag = -mag
		}
		if mag < opts.MinBaseline || mag == 0 {
			continue
		}
		delta := current - baseline
		pct := delta * 100 / mag
		if pct < 0 {
			if -pct < opts.ThresholdPct {
				continue
			}
		} else if pct < opts.ThresholdPct {
			continue
		}
		dir := Up
		if delta < 0 {
			dir = Down
		}
		out = append(out, Anomaly{
			Category:  s.Category,
			Current:   current,
			Baseline:  baseline,
			Delta:     delta,
			PctChange: pct,
			Direction: dir,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := abs64(out[i].Delta), abs64(out[j].Delta)
		if ai != aj {
			return ai > aj
		}
		return out[i].Category < out[j].Category
	})
	return out
}

// mean returns the integer average of xs (truncated toward zero), or 0 when empty.
func mean(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	var sum int64
	for _, x := range xs {
		sum += x
	}
	return sum / int64(len(xs))
}

// abs64 returns the absolute value of x.
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

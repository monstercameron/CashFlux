// SPDX-License-Identifier: MIT

// Package vitals holds small pure analyses over a financial-health time series:
// classifying which way the score is moving and for how long, so the health page
// can narrate "up three months running" instead of only "up 4 since the start".
//
// Pure Go, no platform dependencies.
package vitals

// Direction is the overall drift of a series.
type Direction int

const (
	Flat Direction = iota
	Rising
	Falling
)

// Trend summarizes a score series (oldest → newest).
type Trend struct {
	Direction   Direction // net drift from first to last reading
	Delta       int       // last − first
	StreakDir   Direction // direction of the current unbroken run at the end
	StreakLen   int       // how many consecutive step-to-step moves share StreakDir (≥1 once there are ≥2 points)
	Best        int       // highest reading in the series
	Worst       int       // lowest reading in the series
	Latest      int       // the newest reading
	InflectedUp bool      // the series turned from falling to rising at the end (a recovery)
}

// Classify analyses a series of scores (oldest first). A series shorter than two
// points has no trend (Flat, zero streak). The streak counts consecutive moves in
// one direction ending at the latest reading — flat steps break a streak.
func Classify(scores []int) Trend {
	n := len(scores)
	t := Trend{}
	if n == 0 {
		return t
	}
	t.Latest = scores[n-1]
	t.Best, t.Worst = scores[0], scores[0]
	for _, s := range scores {
		if s > t.Best {
			t.Best = s
		}
		if s < t.Worst {
			t.Worst = s
		}
	}
	if n < 2 {
		return t
	}
	t.Delta = scores[n-1] - scores[0]
	switch {
	case t.Delta > 0:
		t.Direction = Rising
	case t.Delta < 0:
		t.Direction = Falling
	}

	// Current streak: walk backwards while each step keeps the same sign.
	last := stepDir(scores[n-2], scores[n-1])
	t.StreakDir = last
	if last != Flat {
		t.StreakLen = 1
		for i := n - 2; i > 0; i-- {
			if stepDir(scores[i-1], scores[i]) == last {
				t.StreakLen++
			} else {
				break
			}
		}
	}

	// Recovery: the run before the current rising streak was falling.
	if last == Rising && t.StreakLen >= 1 {
		prevIdx := n - 1 - t.StreakLen
		if prevIdx >= 1 && stepDir(scores[prevIdx-1], scores[prevIdx]) == Falling {
			t.InflectedUp = true
		}
	}
	return t
}

// stepDir is the direction of a single step.
func stepDir(a, b int) Direction {
	switch {
	case b > a:
		return Rising
	case b < a:
		return Falling
	default:
		return Flat
	}
}

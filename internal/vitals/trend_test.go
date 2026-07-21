// SPDX-License-Identifier: MIT

package vitals

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name      string
		scores    []int
		dir       Direction
		delta     int
		streakDir Direction
		streakLen int
		inflected bool
	}{
		{"empty", nil, Flat, 0, Flat, 0, false},
		{"single", []int{60}, Flat, 0, Flat, 0, false},
		{"rising run", []int{50, 55, 60, 68}, Rising, 18, Rising, 3, false},
		{"falling run", []int{80, 72, 70}, Falling, -10, Falling, 2, false},
		{"net up but latest dip", []int{50, 65, 62}, Rising, 12, Falling, 1, false},
		{"flat breaks streak", []int{60, 66, 66}, Rising, 6, Flat, 0, false},
		{"recovery after dip", []int{70, 60, 55, 62, 68}, Falling, -2, Rising, 2, true},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.scores)
			if got.Direction != c.dir || got.Delta != c.delta || got.StreakDir != c.streakDir ||
				got.StreakLen != c.streakLen || got.InflectedUp != c.inflected {
				t.Errorf("Classify(%v) = dir %v delta %d streak %v/%d inflect %v; want %v %d %v/%d %v",
					c.scores, got.Direction, got.Delta, got.StreakDir, got.StreakLen, got.InflectedUp,
					c.dir, c.delta, c.streakDir, c.streakLen, c.inflected)
			}
		})
	}
}

func TestClassifyBestWorstLatest(t *testing.T) {
	got := Classify([]int{55, 40, 72, 61})
	if got.Best != 72 || got.Worst != 40 || got.Latest != 61 {
		t.Errorf("best/worst/latest = %d/%d/%d, want 72/40/61", got.Best, got.Worst, got.Latest)
	}
}

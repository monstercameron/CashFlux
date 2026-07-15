// SPDX-License-Identifier: MIT

package emergencyfund

import "testing"

func TestEssentialMonthlyMinor(t *testing.T) {
	cases := []struct {
		name  string
		basis Basis
		want  int64
	}{
		{"both", Basis{FixedMonthlyMinor: 190000, EssentialSpendMonthlyMinor: 100000, Currency: "USD"}, 290000},
		{"fixed only", Basis{FixedMonthlyMinor: 150000, Currency: "USD"}, 150000},
		{"negative clamped", Basis{FixedMonthlyMinor: -5000, EssentialSpendMonthlyMinor: 100000, Currency: "USD"}, 100000},
		{"zero", Basis{Currency: "USD"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.basis.EssentialMonthlyMinor(); got != c.want {
				t.Fatalf("EssentialMonthlyMinor = %d, want %d", got, c.want)
			}
		})
	}
}

func TestSize(t *testing.T) {
	s := Size(Basis{FixedMonthlyMinor: 190000, EssentialSpendMonthlyMinor: 100000, Currency: "USD"})
	if s.EssentialMonthly.Amount != 290000 || s.EssentialMonthly.Currency != "USD" {
		t.Fatalf("essential = %+v", s.EssentialMonthly)
	}
	if s.ThreeMonth.Amount != 870000 {
		t.Fatalf("3mo = %d, want 870000", s.ThreeMonth.Amount)
	}
	if s.SixMonth.Amount != 1740000 {
		t.Fatalf("6mo = %d, want 1740000", s.SixMonth.Amount)
	}
}

func TestTargetFor(t *testing.T) {
	s := Size(Basis{FixedMonthlyMinor: 100000, Currency: "USD"})
	if got := s.TargetFor(LevelThree).Amount; got != 300000 {
		t.Fatalf("LevelThree = %d, want 300000", got)
	}
	if got := s.TargetFor(LevelSix).Amount; got != 600000 {
		t.Fatalf("LevelSix = %d, want 600000", got)
	}
	// Unknown level falls back to six-month.
	if got := s.TargetFor(Level(4)).Amount; got != 600000 {
		t.Fatalf("unknown level = %d, want 600000 fallback", got)
	}
	if got := s.TargetMinor(LevelThree); got != 300000 {
		t.Fatalf("TargetMinor = %d", got)
	}
}

func TestLevelValid(t *testing.T) {
	if !LevelThree.Valid() || !LevelSix.Valid() {
		t.Fatal("3 and 6 should be valid")
	}
	if Level(4).Valid() || Level(0).Valid() {
		t.Fatal("4 and 0 should be invalid")
	}
	if LevelThree.Months() != 3 || LevelSix.Months() != 6 {
		t.Fatal("Months mismatch")
	}
}

func TestTrailingAverageMinor(t *testing.T) {
	cases := []struct {
		name    string
		monthly []int64
		want    int64
	}{
		{"empty", nil, 0},
		{"single", []int64{100000}, 100000},
		{"average", []int64{90000, 100000, 110000}, 100000},
		{"truncates", []int64{100, 100, 101}, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TrailingAverageMinor(c.monthly); got != c.want {
				t.Fatalf("TrailingAverageMinor = %d, want %d", got, c.want)
			}
		})
	}
}

func TestDriftExceeds(t *testing.T) {
	cases := []struct {
		name           string
		prior, derived int64
		pct            int
		want           bool
	}{
		{"no prior basis", 0, 100000, 10, false},
		{"within threshold", 100000, 105000, 10, false},
		{"exactly at threshold", 100000, 110000, 10, false}, // strictly greater required
		{"over threshold up", 100000, 120000, 10, true},
		{"over threshold down", 100000, 85000, 10, true},
		{"negative pct", 100000, 200000, -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DriftExceeds(c.prior, c.derived, c.pct); got != c.want {
				t.Fatalf("DriftExceeds(%d,%d,%d) = %v, want %v", c.prior, c.derived, c.pct, got, c.want)
			}
		})
	}
}

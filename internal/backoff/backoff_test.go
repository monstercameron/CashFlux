// SPDX-License-Identifier: MIT

package backoff

import (
	"testing"
	"time"
)

func TestDelay(t *testing.T) {
	base, cap := 2*time.Second, 120*time.Second
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{-1, base},
		{0, base},
		{1, 4 * time.Second},
		{2, 8 * time.Second},
		{3, 16 * time.Second},
		{4, 32 * time.Second},
		{5, 64 * time.Second},
		{6, cap},  // 128 -> capped at 120
		{99, cap}, // far out -> cap, no overflow
	}
	for _, c := range cases {
		if got := Delay(c.attempt, base, cap); got != c.want {
			t.Errorf("Delay(%d)=%v want %v", c.attempt, got, c.want)
		}
	}
}

func TestDelayEdge(t *testing.T) {
	if got := Delay(3, 0, time.Second); got != 0 {
		t.Errorf("zero base => %v want 0", got)
	}
	// cap below base is clamped up to base.
	if got := Delay(5, 10*time.Second, time.Second); got != 10*time.Second {
		t.Errorf("cap<base => %v want base", got)
	}
}

func TestJitter(t *testing.T) {
	d := 100 * time.Second
	// rnd=0.5 is the midpoint => unchanged.
	if got := Jitter(d, 0.5, 0.5); got != d {
		t.Errorf("midpoint jitter=%v want %v", got, d)
	}
	// rnd=0 => lower bound (1-frac); rnd→1 => upper bound (1+frac).
	if got := Jitter(d, 0.2, 0); got != 80*time.Second {
		t.Errorf("low jitter=%v want 80s", got)
	}
	if got := Jitter(d, 0.2, 1); got != 120*time.Second {
		t.Errorf("high jitter=%v want 120s", got)
	}
	// frac<=0 is a no-op.
	if got := Jitter(d, 0, 0.9); got != d {
		t.Errorf("no-frac jitter=%v want %v", got, d)
	}
	// Stays within bounds across the range, never negative.
	for i := 0; i <= 10; i++ {
		rnd := float64(i) / 10
		got := Jitter(d, 0.5, rnd)
		if got < 50*time.Second || got > 150*time.Second {
			t.Errorf("rnd=%v out of band: %v", rnd, got)
		}
	}
}

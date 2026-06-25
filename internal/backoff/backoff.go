// SPDX-License-Identifier: MIT

// Package backoff computes exponential reconnect delays with a cap. It is pure
// (no syscall/js, no clock, no randomness), so reconnect policy is unit-tested on
// native Go; the caller owns sleeping and jitter.
package backoff

import "time"

// Delay returns the exponential backoff for a zero-based attempt number:
// base * 2^attempt, clamped to cap. attempt<=0 returns base. A non-positive base
// returns 0. cap below base is treated as base (no negative window).
//
// The result is the deterministic backoff; apply jitter at the call site via
// Jitter so the pure schedule stays testable.
func Delay(attempt int, base, cap time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if cap < base {
		cap = base
	}
	if attempt <= 0 {
		return base
	}
	d := base
	for range attempt {
		d *= 2
		if d >= cap || d <= 0 { // d<=0 guards against int64 overflow on a long outage
			return cap
		}
	}
	if d > cap {
		return cap
	}
	return d
}

// Jitter spreads d by +/- frac (frac in [0,1)) using rnd, a caller-supplied random
// fraction in [0,1). This decorrelates many clients reconnecting at once (the
// thundering-herd problem). frac<=0 returns d unchanged. The result is clamped to
// be non-negative.
func Jitter(d time.Duration, frac, rnd float64) time.Duration {
	if frac <= 0 || d <= 0 {
		return d
	}
	if frac > 1 {
		frac = 1
	}
	// Map rnd in [0,1) to a multiplier in [1-frac, 1+frac).
	mult := 1 + frac*(2*rnd-1)
	out := time.Duration(float64(d) * mult)
	if out < 0 {
		return 0
	}
	return out
}

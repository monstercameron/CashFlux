// SPDX-License-Identifier: MIT

// Package goalinterest projects a savings goal's completion with monthly
// compounding interest — the savings mirror of internal/payoff's debt
// amortization. Given a goal's current balance, a fixed monthly contribution,
// its target, and the linked account's APY, it counts the whole months to reach
// the target and splits the growth into the part that came from contributions
// versus the part the interest added.
//
// The projection is deterministic and fully explainable (every result carries a
// contributions-vs-interest breakdown), and an APY of zero degrades exactly to
// the linear pace math (ceil(remaining / monthly) months, zero interest).
//
// Pure Go, no platform dependencies; unit-tested on native Go. All money is in
// integer minor units.
package goalinterest

// maxMonths caps the projection horizon so a contribution that never reaches the
// target (or grows impossibly slowly) terminates instead of looping forever. It
// is 100 years of months — far beyond any realistic savings goal.
const maxMonths = 1200

// Projection is the interest-aware forecast for a goal, all money in minor units.
type Projection struct {
	// Reached reports whether the target is met within the horizon. When false the
	// other fields describe the position at the horizon cap and Months is 0.
	Reached bool
	// Months is the whole number of monthly contributions needed to reach (or
	// exceed) the target. Zero when the goal is already complete or unreachable.
	Months int
	// FinalMinor is the projected balance at Months — at or just past the target.
	FinalMinor int64
	// ContributedMinor is the total the saver puts in over the projection: the
	// starting balance plus monthly*Months. It excludes interest.
	ContributedMinor int64
	// InterestMinor is how much the yield added: FinalMinor - ContributedMinor.
	// Zero when the APY is zero (the linear degradation).
	InterestMinor int64
}

// Project counts the whole months of a fixed monthly contribution needed to grow
// currentMinor to targetMinor at the given annual percentage yield (apyPercent,
// e.g. 4.4 for 4.4%), compounded monthly. Interest posts first each month, then
// the contribution lands.
//
//	balance₀ = current
//	balanceₙ = balanceₙ₋₁ × (1 + apy/1200) + monthly
//
// A goal already at or above target returns Reached with zero months. When
// apyPercent is zero the loop reduces to balance += monthly each month, so Months
// is the linear ceil(remaining / monthly) and InterestMinor is zero. When the
// contribution (and interest) never reach the target within maxMonths, Reached is
// false.
func Project(currentMinor, monthlyMinor, targetMinor int64, apyPercent float64) Projection {
	if targetMinor <= 0 || currentMinor >= targetMinor {
		// Already complete (or a non-positive target): nothing to project.
		return Projection{
			Reached:          true,
			Months:           0,
			FinalMinor:       currentMinor,
			ContributedMinor: currentMinor,
			InterestMinor:    0,
		}
	}

	monthlyRate := apyPercent / 1200.0 // percent → fraction, annual → monthly

	// A non-positive contribution with no yield can never reach the target.
	if monthlyMinor <= 0 && monthlyRate <= 0 {
		return Projection{Reached: false}
	}

	balance := float64(currentMinor)
	prevBalance := balance
	for m := 1; m <= maxMonths; m++ {
		balance = balance*(1+monthlyRate) + float64(monthlyMinor)
		if balance >= float64(targetMinor) {
			// Guard against a stalled projection: if interest can't lift the balance
			// and there's no contribution, bail (belt-and-braces; the pre-check above
			// already handles the pure cases).
			finalMinor := roundMinor(balance)
			contributed := currentMinor + monthlyMinor*int64(m)
			return Projection{
				Reached:          true,
				Months:           m,
				FinalMinor:       finalMinor,
				ContributedMinor: contributed,
				InterestMinor:    finalMinor - contributed,
			}
		}
		if balance <= prevBalance {
			// Not advancing — unreachable.
			return Projection{Reached: false}
		}
		prevBalance = balance
	}
	return Projection{Reached: false}
}

// roundMinor rounds a float minor-unit balance to the nearest whole minor unit.
func roundMinor(f float64) int64 {
	if f < 0 {
		return int64(f - 0.5)
	}
	return int64(f + 0.5)
}

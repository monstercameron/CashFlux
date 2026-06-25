// SPDX-License-Identifier: MIT

package payoff

// Comparison holds the headline differences between a snowball plan and an
// avalanche plan built from the same debts and extra payment. Both plans must
// have been produced by BuildPlan with the same inputs (same debts, same extra)
// so that the only variable is the payoff strategy.
type Comparison struct {
	// MonthsSaved is the number of months by which avalanche finishes before
	// snowball (snowball.Months - avalanche.Months). A positive value means
	// avalanche is faster; a negative value (avalanche slower) means snowball
	// wins on time, which is unusual but possible when minimum payments dominate.
	MonthsSaved int

	// InterestSavedMinor is the extra interest saved by choosing avalanche over
	// snowball (snowball.TotalInterest - avalanche.TotalInterest), in base-
	// currency minor units. A positive value means avalanche costs less interest.
	InterestSavedMinor int64

	// Faster reports which strategy completes first.
	// Values: "avalanche", "snowball", or "tie".
	// A tie occurs when both MonthsSaved and InterestSavedMinor are zero.
	// When only one delta is non-zero, the sign of MonthsSaved is the primary
	// tiebreaker (fewer months is better), with InterestSavedMinor as the
	// secondary criterion.
	Faster string
}

// Compare returns the headline differences between a snowball plan and an
// avalanche plan. It does not validate that the plans were produced from the
// same inputs — the caller is responsible for that consistency.
func Compare(snowball, avalanche Plan) Comparison {
	monthsSaved := snowball.Months - avalanche.Months
	interestSaved := snowball.TotalInterest - avalanche.TotalInterest

	var faster string
	switch {
	case monthsSaved == 0 && interestSaved == 0:
		faster = "tie"
	case monthsSaved > 0:
		// Avalanche finishes in fewer months.
		faster = "avalanche"
	case monthsSaved < 0:
		// Snowball finishes in fewer months (unusual but valid).
		faster = "snowball"
	case interestSaved > 0:
		// Same months, avalanche saves more interest.
		faster = "avalanche"
	default:
		// Same months, snowball saves more interest (or they're equal).
		faster = "snowball"
	}

	return Comparison{
		MonthsSaved:        monthsSaved,
		InterestSavedMinor: interestSaved,
		Faster:             faster,
	}
}

// SPDX-License-Identifier: MIT

package goaltrajectory

// Scenario pace factors, in quarters: conservative saves 25% less than the
// plan, best saves 25% more. Chosen as honest, plainly explainable what-ifs
// ("if you save a quarter less/more each month") rather than a statistical
// claim the local data can't back.
const (
	conservativeNum = 3 // ×3/4
	bestNum         = 5 // ×5/4
	scenarioDen     = 4
)

// Scenarios holds the three completion projections for one goal: the planned
// pace (Expected) bracketed by saving 25% less (Conservative) and 25% more
// (Best). Each is a full Result, so callers can read dates, months, and
// reachability per scenario.
type Scenarios struct {
	Conservative Result
	Expected     Result
	Best         Result
}

// ProjectScenarios projects a goal three times — at 75%, 100%, and 125% of the
// planned monthly contribution — so a card can show a best/expected/conservative
// landing date range instead of a single point estimate. The scaled paces are
// floored at one minor unit so a tiny plan still moves. Input semantics match
// Project.
func ProjectScenarios(in Input) Scenarios {
	scaled := func(num int64) Input {
		out := in
		if in.MonthlyMinor > 0 {
			m := in.MonthlyMinor * num / scenarioDen
			if m < 1 {
				m = 1
			}
			out.MonthlyMinor = m
		}
		return out
	}
	return Scenarios{
		Conservative: Project(scaled(conservativeNum)),
		Expected:     Project(in),
		Best:         Project(scaled(bestNum)),
	}
}

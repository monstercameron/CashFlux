// SPDX-License-Identifier: MIT

// Package montecarlo answers "will the money last" as a probability rather than
// a single line (FP-T3d).
//
// A projection at a fixed 7% return is a story about one future that will not
// happen. Real returns arrive in an order, and the ORDER matters enormously
// during drawdown: the same average return with the bad years at the start can
// exhaust a portfolio that the same years at the end would have left intact.
// That is sequence-of-returns risk, and no single-rate projection can show it.
//
// # Determinism
//
// This app does not ship black boxes, and a simulation that reports a different
// answer each time it is opened is the definition of one. So:
//
//   - The generator is IN THIS PACKAGE, not math/rand. A standard library
//     generator is free to change its stream between Go versions, which would
//     silently move every household's success rate on an unrelated upgrade.
//   - The seed is an input and is REPORTED with the result, so a figure can be
//     reproduced exactly and disputed.
//   - Same inputs give the same answer, always. A test asserts it.
//
// The method is stated rather than implied: returns are drawn from a normal
// distribution, which is a convention that understates the frequency of extreme
// years. Saying so is the difference between a model and a claim.
package montecarlo

import (
	"math"
	"slices"
)

// DefaultIterations is how many futures are simulated when a caller does not say.
//
// Two thousand. Enough that the success rate is stable to about a percentage
// point, few enough to run inside a render without the page stalling — and the
// figure is reported to the nearest whole percent for exactly that reason.
const DefaultIterations = 2000

// DefaultSeed is the seed used when a caller does not supply one.
//
// A constant, not a clock reading. Two people with the same plan should see the
// same number, and the same person should see the same number tomorrow.
const DefaultSeed uint64 = 0x5CA1AB1E

// DefaultStdDevPct is the assumed annual standard deviation of returns.
//
// Fifteen percent, roughly the long-run figure for a broad equity portfolio. A
// stated convention, not a measurement of this household's holdings.
const DefaultStdDevPct = 15.0

// MaxIterations bounds the work a single call can do.
const MaxIterations = 20000

// MaxYears bounds the horizon.
const MaxYears = 100

// Config is the simulation's assumptions.
type Config struct {
	// Iterations is how many futures to run; zero means DefaultIterations.
	Iterations int
	// Seed makes the run reproducible; zero means DefaultSeed.
	Seed uint64
	// MeanReturnPct and StdDevPct describe the annual return distribution.
	// StdDevPct of zero means "use DefaultStdDevPct" — see NoVolatility for the
	// genuine zero.
	MeanReturnPct, StdDevPct float64
	// NoVolatility runs every future identically, at exactly MeanReturnPct.
	//
	// It exists because zero is a REAL standard deviation and "unset" is not, and
	// the two cannot both be spelled `StdDevPct: 0`. Without this, asking for a
	// deterministic run silently got a 15% one — a simulation quietly answering a
	// different question than it was asked. It is a separate flag rather than a
	// sentinel so a caller cannot reach it by forgetting a field.
	NoVolatility bool
	// InflationPct raises spending each year, so the horizon is measured in what
	// the money BUYS rather than in dollars. A drawdown that ignores inflation
	// overstates how long a portfolio lasts by more, the longer the horizon.
	InflationPct float64
}

// normalized fills in defaults and reports whether the config is usable.
func (c Config) normalized() (Config, bool) {
	if c.Iterations == 0 {
		c.Iterations = DefaultIterations
	}
	if c.Seed == 0 {
		c.Seed = DefaultSeed
	}
	switch {
	case c.NoVolatility:
		c.StdDevPct = 0
	case c.StdDevPct == 0:
		c.StdDevPct = DefaultStdDevPct
	}
	if c.Iterations < 1 || c.Iterations > MaxIterations {
		return c, false
	}
	if c.StdDevPct < 0 || c.MeanReturnPct <= -100 || c.InflationPct <= -100 {
		return c, false
	}
	return c, true
}

// Result is what the simulation found.
type Result struct {
	// SuccessRatePct is the share of futures in which the money lasted the whole
	// horizon.
	SuccessRatePct float64
	// MedianEndingMinor, P10EndingMinor and P90EndingMinor describe the spread of
	// outcomes.
	//
	// The median and not the mean: a handful of enormous outcomes drag a mean
	// upward and describe nobody's life, while the median is a future half of
	// these runs beat.
	MedianEndingMinor, P10EndingMinor, P90EndingMinor int64
	// WorstDepletionYear is the earliest year any run ran out, and Depleted says
	// whether any did. Reported because "92% success" and "the failures happen in
	// year 4" are different situations from "92%" and "they happen in year 28".
	WorstDepletionYear int
	Depleted           bool
	// Iterations and Seed are echoed so the figure can be reproduced exactly.
	Iterations int
	Seed       uint64
}

// Run simulates a drawdown and reports how often the money lasted.
//
// Reports ok=false rather than a zero result for an unusable configuration, a
// non-positive starting balance, or a horizon outside the bounds. A "0% chance
// of success" for a plan nobody described is the most alarming possible way to
// say "we could not compute this".
func Run(startMinor, annualSpendMinor int64, years int, cfg Config) (Result, bool) {
	c, ok := cfg.normalized()
	if !ok || startMinor <= 0 || years < 1 || years > MaxYears {
		return Result{}, false
	}
	if annualSpendMinor < 0 {
		return Result{}, false
	}

	rng := newPCG(c.Seed)
	endings := make([]int64, 0, c.Iterations)
	successes := 0
	worstYear := 0
	depleted := false

	mean := c.MeanReturnPct / 100
	sd := c.StdDevPct / 100
	infl := c.InflationPct / 100

	for i := 0; i < c.Iterations; i++ {
		balance := float64(startMinor)
		spend := float64(annualSpendMinor)
		survived := true
		for y := 1; y <= years; y++ {
			// Spending comes out at the START of the year, before growth: a year's
			// spending cannot compound through the year it is spent. Same rule as
			// the deterministic drawdown in internal/retirement, and the two must
			// agree or the same plan gets two different answers.
			balance -= spend
			if balance <= 0 {
				survived = false
				if worstYear == 0 || y < worstYear {
					worstYear = y
				}
				depleted = true
				balance = 0
				break
			}
			balance *= 1 + rng.normal(mean, sd)
			if balance < 0 {
				balance = 0
			}
			spend *= 1 + infl
		}
		if survived {
			successes++
		}
		endings = append(endings, int64(math.Round(balance)))
	}

	slices.Sort(endings)
	return Result{
		SuccessRatePct:     float64(successes) / float64(c.Iterations) * 100,
		MedianEndingMinor:  percentile(endings, 50),
		P10EndingMinor:     percentile(endings, 10),
		P90EndingMinor:     percentile(endings, 90),
		WorstDepletionYear: worstYear,
		Depleted:           depleted,
		Iterations:         c.Iterations,
		Seed:               c.Seed,
	}, true
}

// percentile reads a value out of a sorted slice by nearest rank.
//
// Nearest rank rather than interpolation: interpolating between two simulated
// futures invents a third that was never run, and the extra precision is not
// precision about anything.
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * len(sorted) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

// pcg is a small permuted-congruential generator.
//
// Written out here rather than taken from math/rand so the stream is fixed by
// THIS code. A standard library generator may change its output between Go
// versions, which would move every household's success rate on an upgrade that
// had nothing to do with them.
type pcg struct {
	state uint64
	inc   uint64
	// spare holds the second value from a Box-Muller pair, which produces two
	// normal draws at a time. Keeping it costs one field and halves the work.
	spare    float64
	hasSpare bool
}

func newPCG(seed uint64) *pcg {
	p := &pcg{state: 0, inc: (seed << 1) | 1}
	p.next()
	p.state += seed
	p.next()
	return p
}

// next returns the following 32 bits of the stream.
func (p *pcg) next() uint32 {
	old := p.state
	p.state = old*6364136223846793005 + p.inc
	// #nosec G115 -- the narrowing IS the algorithm. PCG XSH-RR produces a 32-bit
	// output from 64-bit state by permuting the high bits and discarding the rest;
	// taking the low 32 of the xorshifted value is the "XSH" step, not an accident
	// of type width, and widening it would produce a different (and wrong) stream.
	// `rot` cannot overflow at all: shifting a uint64 right by 59 leaves 5 bits.
	xorshifted := uint32(((old >> 18) ^ old) >> 27)
	rot := uint32(old >> 59)
	return (xorshifted >> rot) | (xorshifted << ((-rot) & 31))
}

// float64 returns a value in [0,1).
func (p *pcg) float64() float64 {
	return float64(p.next()) / (1 << 32)
}

// normal draws from a normal distribution by Box-Muller.
//
// The normal distribution is a CONVENTION, and it understates how often extreme
// years happen — real markets have fatter tails than this. The surface says so;
// silently modelling a kinder world than the one outside is how a simulation
// becomes reassurance.
func (p *pcg) normal(mean, sd float64) float64 {
	if sd == 0 {
		return mean
	}
	if p.hasSpare {
		p.hasSpare = false
		return mean + sd*p.spare
	}
	// u1 must be non-zero: Log(0) is -Inf and would poison the draw.
	u1 := p.float64()
	for u1 <= 1e-12 {
		u1 = p.float64()
	}
	u2 := p.float64()
	r := math.Sqrt(-2 * math.Log(u1))
	theta := 2 * math.Pi * u2
	p.spare = r * math.Sin(theta)
	p.hasSpare = true
	return mean + sd*r*math.Cos(theta)
}

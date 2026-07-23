// SPDX-License-Identifier: MIT

// Package finplan is a pure, opinionated financial-plan engine: it places the user on a step of a
// well-known "order of operations" framework from their own data, so CashFlux can tell them the ONE
// next thing to do instead of a generic dashboard.
//
// Two frameworks are modelled, deliberately kept distinct rather than blended:
//
//   - FOO — the Financial Order of Operations (The Money Guy Show): 9 steps, math-optimized
//     (employer match and tax-advantaged investing come before paying off low-interest debt).
//   - Ramsey — Dave Ramsey's 7 Baby Steps: debt-first and behavioral (kill all non-mortgage debt
//     before investing).
//
// The engine assesses each step from the signals CashFlux can actually derive (emergency-fund months,
// whether high-interest debt exists, whether there's non-mortgage debt, a savings-rate proxy for
// investing). Steps it cannot judge from data (employer match %, insurance deductibles, a will) are
// marked NotAssessable — the UI asks the user rather than guessing. The current step is the first one
// that isn't Done, so unknowns correctly surface as "confirm this" work.
package finplan

// Framework selects which order-of-operations model to assess against.
type Framework int

const (
	// FOO is the Financial Order of Operations (default — the more math-efficient path).
	FOO Framework = iota
	// Ramsey is the 7 Baby Steps (debt-first, behavioral).
	Ramsey
)

// highInterestAPR is the threshold (annual %, whole number) at or above which debt is treated as
// "high interest" and prioritized ahead of investing — the ~6% rule of thumb both frameworks use.
const highInterestAPR = 6.0

// safeEmergencyMonths is the lower bound of the 3–6 month emergency-fund target.
const safeEmergencyMonths = 3.0

// starterFundMinor is Ramsey Baby Step 1's $1,000 starter fund, in minor units (cents).
const starterFundMinor = 1000 * 100

// Status is how a step was judged.
type Status int

const (
	// NotAssessable: the engine has no data signal for this step — the UI must ask the user.
	NotAssessable Status = iota
	// Done: assessed complete from data.
	Done
	// Todo: assessed incomplete from data.
	Todo
)

// Step is one rung of a framework, with its assessed status.
type Step struct {
	Num    int    // 1-based step number within the framework
	Title  string // short imperative title
	Detail string // what to actually do
	Status Status
}

// Inputs are the derived financial signals the engine assesses against. Fields carry a Has* guard
// where "no data" must be distinguished from a real value (so an empty account set doesn't read as
// "you have zero emergency fund — do step 1"). Questionnaire answers (employer match, deductibles)
// are optional and left zero/false when unknown.
type Inputs struct {
	HasLiquidData    bool    // whether liquid-cash + spending are known (gates emergency-fund + starter checks)
	LiquidCashMinor  int64   // spendable buffer, minor units
	EmergencyMonths  float64 // liquid cash ÷ average monthly spending
	HasIncome        bool    // whether income is known (gates match/investing framing)
	SavingsRatePct   int     // trailing savings rate, a proxy for "are you investing" (may be negative)
	KnowsLiabilities bool    // whether liability data is available (gates the debt checks)
	HasHighInterestDebt bool // any non-mortgage liability at/above highInterestAPR
	HasNonMortgageDebt  bool // any liability that isn't a mortgage

	// Optional questionnaire answers — only meaningful when the matching Answered flag is set.
	AnsweredMatch      bool
	GetsFullMatch      bool // contributes enough to capture the full employer match
	AnsweredDeductible bool
	DeductiblesCovered bool // has cash set aside for insurance deductibles
}

// Plan is an assessed framework: its steps and the index of the current (first not-Done) step.
type Plan struct {
	Framework Framework
	Steps     []Step
	// CurrentIndex is the 0-based index into Steps of the first step that isn't Done (i.e. the one to
	// work on now). It is len(Steps) when every step reads Done.
	CurrentIndex int
}

// Assess builds the plan for the chosen framework from the inputs.
func Assess(fw Framework, in Inputs) Plan {
	var steps []Step
	if fw == Ramsey {
		steps = ramseySteps(in)
	} else {
		steps = fooSteps(in)
	}
	return Plan{Framework: fw, Steps: steps, CurrentIndex: firstNotDone(steps)}
}

// Current returns the current step and true, or a zero step and false when everything reads Done.
func (p Plan) Current() (Step, bool) {
	if p.CurrentIndex < 0 || p.CurrentIndex >= len(p.Steps) {
		return Step{}, false
	}
	return p.Steps[p.CurrentIndex], true
}

// firstNotDone returns the index of the first step whose status isn't Done (Todo or NotAssessable),
// or len(steps) when all are Done.
func firstNotDone(steps []Step) int {
	for i, s := range steps {
		if s.Status != Done {
			return i
		}
	}
	return len(steps)
}

// doneIf maps a boolean assessment to Done/Todo.
func doneIf(ok bool) Status {
	if ok {
		return Done
	}
	return Todo
}

// emergencyStatus assesses a "3–6 months of expenses" step: NotAssessable without liquid data.
func emergencyStatus(in Inputs) Status {
	if !in.HasLiquidData {
		return NotAssessable
	}
	return doneIf(in.EmergencyMonths >= safeEmergencyMonths)
}

// highInterestDebtStatus assesses a "pay off high-interest debt" step.
func highInterestDebtStatus(in Inputs) Status {
	if !in.KnowsLiabilities {
		return NotAssessable
	}
	return doneIf(!in.HasHighInterestDebt)
}

// investingStatus assesses an "are you investing enough" step from the savings-rate proxy at the given
// target percent — NotAssessable without income data.
func investingStatus(in Inputs, targetPct int) Status {
	if !in.HasIncome {
		return NotAssessable
	}
	return doneIf(in.SavingsRatePct >= targetPct)
}

func fooSteps(in Inputs) []Step {
	deductible := NotAssessable
	if in.AnsweredDeductible {
		deductible = doneIf(in.DeductiblesCovered)
	}
	match := NotAssessable
	if in.AnsweredMatch {
		match = doneIf(in.GetsFullMatch)
	}
	return []Step{
		{1, "Cover your deductibles", "Keep enough cash on hand to pay your insurance deductibles if something breaks.", deductible},
		{2, "Get the full employer match", "Contribute at least enough to your workplace plan to capture the entire employer match — it's free money.", match},
		{3, "Pay off high-interest debt", "Attack any debt at ~6% APR or higher; it beats almost any investment return.", highInterestDebtStatus(in)},
		{4, "Build 3–6 months of reserves", "Save 3–6 months of expenses in cash so a job loss or big surprise doesn't derail you.", emergencyStatus(in)},
		{5, "Fund a Roth IRA & HSA", "Max these tax-advantaged accounts — tax-free growth (HSA is triple-tax-advantaged).", investingStatus(in, 10)},
		{6, "Max out retirement accounts", "Fill your 401(k)/IRA up to the annual limits.", investingStatus(in, 15)},
		{7, "Hyper-accumulate (25% of income)", "Push total retirement saving toward 25% of your gross income.", investingStatus(in, 25)},
		{8, "Prepay future expenses", "Fund known upcoming costs — a child's college (529), a planned purchase.", NotAssessable},
		{9, "Pay off low-interest debt", "With everything else handled, retire remaining low-interest debt like your mortgage.", doneIf(in.KnowsLiabilities && !in.HasNonMortgageDebt)},
	}
}

func ramseySteps(in Inputs) []Step {
	starter := NotAssessable
	if in.HasLiquidData {
		starter = doneIf(in.LiquidCashMinor >= starterFundMinor)
	}
	return []Step{
		{1, "$1,000 starter emergency fund", "Save a $1,000 starter fund before anything else.", starter},
		{2, "Pay off all debt (except the house)", "Clear every non-mortgage debt using the snowball — smallest balance first for momentum.", doneIf(in.KnowsLiabilities && !in.HasNonMortgageDebt)},
		{3, "3–6 months of expenses saved", "Build a full emergency fund of 3–6 months of expenses.", emergencyStatus(in)},
		{4, "Invest 15% for retirement", "Put 15% of household income into retirement.", investingStatus(in, 15)},
		{5, "Save for kids' college", "Start funding your children's education.", NotAssessable},
		{6, "Pay off your home early", "Throw everything extra at the mortgage.", NotAssessable},
		{7, "Build wealth and give", "Keep investing, and give generously.", NotAssessable},
	}
}

// HighInterestAPR exposes the threshold so callers can label debt consistently.
func HighInterestAPR() float64 { return highInterestAPR }

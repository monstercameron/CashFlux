// SPDX-License-Identifier: MIT

// Package vitals derives the household's POSITION metrics — the balance-sheet
// vital signs the Annual Review's "Where you stand" section reads out: cash-flow
// capacity (income, spending, surplus, savings rate, discretionary), the liquid
// cushion (essential month, coverage, fund target and gap, runway after debt
// service), and the obligation picture (debt totals, required payments, payment
// share of income, weighted rate, interest drag, payoff horizon, utilization).
//
// It is pure (no syscall/js, no I/O) so it unit-tests on native Go and runs the
// same in the wasm build. The UI layer assembles Inputs from already-derived
// figures (trailing monthly averages, the essential-month basis, liquid cash,
// the debt list); this package owns only the arithmetic and the judgment bands,
// so every figure is reproducible and explainable — no black boxes.
//
// All amounts are integer minor units of ONE currency (the caller converts to
// base first). Percentages are whole percents unless noted; month figures are
// tenths of a month so "5.3 mo" survives integer math.
package vitals

import (
	"math"

	"github.com/monstercameron/CashFlux/internal/payoff"
)

// Tone is a judgment band matching the report's tone vocabulary. The empty tone
// means "no judgment applies" (a context figure, neither good nor bad).
type Tone string

const (
	ToneNone Tone = ""
	ToneUp   Tone = "up"
	ToneWarn Tone = "warn"
	ToneDown Tone = "down"
)

// Debt is one liability's position inputs. Balance is the positive amount owed.
type Debt struct {
	Name            string
	BalanceMinor    int64
	AprPercent      float64
	MinPaymentMinor int64
	// IsMortgage marks a mortgage balance, split out of the ex-mortgage total.
	IsMortgage bool
	// InPayoff marks debts the payoff horizon simulates (the app's default is
	// every liability except a mortgage, honouring an explicit override).
	InPayoff bool
}

// Cards is the aggregate revolving-credit position.
type Cards struct {
	// HasCards is true when at least one active credit card exists — utilization
	// is inapplicable (not zero) without one.
	HasCards     bool
	BalanceMinor int64 // total revolving balance (positive)
	LimitMinor   int64 // total credit limit
}

// Inputs are the pre-derived signals the metrics are computed from.
type Inputs struct {
	// IncomeMonthlyMinor / ExpenseMonthlyMinor are trailing monthly averages of
	// non-transfer income and spending (both positive). MonthsAveraged records
	// how many active months fed the average, for the UI's honesty line.
	IncomeMonthlyMinor  int64
	ExpenseMonthlyMinor int64
	MonthsAveraged      int

	// EssentialMonthlyMinor is the cost of one essential month (fixed recurring
	// commitments + trailing essential spend — the emergencyfund basis).
	EssentialMonthlyMinor int64

	// LiquidMinor is the household's liquid cash (cash-type account balances).
	LiquidMinor int64

	Debts []Debt
	Cards Cards

	// FundMonths is the emergency-fund horizon the target is sized to; values
	// outside 1–24 fall back to 6 (the app's standard fuller fund).
	FundMonths int
}

// Result is the full, explainable output. Every judged figure carries its Tone;
// Has* flags mark applicability so the UI never renders a fake zero.
type Result struct {
	// ── Cash flow capacity ──
	HasIncome bool // any income in the averaged window
	// IncomeMonthlyMinor / ExpenseMonthlyMinor / LiquidMinor echo the inputs so a
	// renderer works from one struct and the figures reconcile by construction.
	IncomeMonthlyMinor  int64
	ExpenseMonthlyMinor int64
	LiquidMinor         int64
	SurplusMonthlyMinor int64 // income − spending (signed)
	SurplusAnnualMinor  int64 // surplus × 12 — the year-end picture at this pace
	SurplusTone         Tone
	SavingsRatePct      int // surplus as % of income, clamped to ±100
	SavingsTone         Tone
	// DiscretionaryMinor is what's left after spending AND required debt
	// minimums: surplus − minimum payments (signed).
	DiscretionaryMinor int64
	DiscretionaryTone  Tone

	// ── The cushion ──
	HasCushion            bool  // essential month is known and positive
	EssentialMonthlyMinor int64 // echoed from inputs (the basis of every cushion figure)
	CoverageMonthsTenths  int64 // liquid ÷ essential month, in tenths of a month
	CoverageTone          Tone
	FundMonths            int   // the horizon the target below is sized to
	FundTargetMinor       int64 // FundMonths × essential month
	FundGapMinor          int64 // target − liquid; positive = short, negative = past it
	// RunwayAfterDebtTenths is liquid ÷ (average spending + required debt
	// minimums), in tenths of a month — how long the cash lasts if income
	// stopped but the bills and minimums kept coming. -1 when that burn is zero.
	RunwayAfterDebtTenths int64
	RunwayTone            Tone

	// ── Debt & credit ──
	HasDebts               bool
	TotalDebtMinor         int64 // Σ balances (positive)
	ExMortgageMinor        int64 // Σ balances excluding mortgages
	HasMortgage            bool
	MinPaymentsMinor       int64 // Σ required monthly minimums
	AnnualDebtServiceMinor int64 // minimums × 12
	// PaymentShareOfIncomePct is required minimums as % of monthly income (the
	// minimums-only DTI the health score also uses). Applicable iff HasIncome.
	PaymentShareOfIncomePct int
	PaymentShareTone        Tone
	// WeightedAprPercent is the balance-weighted average APR across all debts
	// (zero-rate balances count — they genuinely dilute the blended cost).
	WeightedAprPercent float64
	WeightedAprTone    Tone
	// InterestDragMonthlyMinor is Σ balance × APR ÷ 12 — the monthly cost of
	// standing still.
	InterestDragMonthlyMinor int64
	// Payoff horizon at minimums only (no extra), avalanche order, over the
	// payoff-included debts. PayoffNeverClears is the honest "minimums alone
	// never clear this" verdict when the simulation can't make progress.
	PayoffApplicable  bool
	PayoffMonths      int
	PayoffNeverClears bool
	PayoffTone        Tone

	// ── Revolving credit ──
	HasCards           bool
	CardBalanceMinor   int64
	CardLimitMinor     int64
	CardAvailableMinor int64 // limit − balance, floored at 0
	UtilizationPct     int   // balance as % of limit, clamped 0–100; 0 when no limit
	HasUtilization     bool  // a positive limit exists to measure against
	UtilizationTone    Tone
}

// Judgment thresholds. Each is the published target its metric's meter ticks at.
const (
	// SavingsTargetPct is the savings-rate target (20%+, the healthscore target).
	SavingsTargetPct = 20
	// PaymentShareTargetPct is the DTI comfort ceiling (36%, common underwriting).
	PaymentShareTargetPct = 36
	// paymentShareCriticalPct is where the burden turns critical (43%).
	paymentShareCriticalPct = 43
	// UtilizationTargetPct is the utilization ceiling (30%; under 10% is cleanest).
	UtilizationTargetPct = 30
	// utilizationHighPct is where utilization turns critical.
	utilizationHighPct = 60
	// CoverageTargetTenths is the full-cushion coverage target (6.0 months).
	CoverageTargetTenths = 60
	// coverageFloorTenths is the minimum acceptable coverage (3.0 months).
	coverageFloorTenths = 30
	// aprLowPercent / aprHighPercent band the blended borrowing cost.
	aprLowPercent  = 5.0
	aprHighPercent = 10.0
	// defaultFundMonths is the fund horizon used when Inputs.FundMonths is unset.
	defaultFundMonths = 6
	// discretionaryWarnPctOfIncome: a positive-but-thin buffer (under this % of
	// income) reads as a tight month, not a healthy one.
	discretionaryWarnPctOfIncome = 10
)

// Evaluate derives every position metric from the inputs. Deterministic.
func Evaluate(in Inputs) Result {
	r := Result{
		HasIncome:             in.IncomeMonthlyMinor > 0,
		IncomeMonthlyMinor:    in.IncomeMonthlyMinor,
		ExpenseMonthlyMinor:   in.ExpenseMonthlyMinor,
		LiquidMinor:           in.LiquidMinor,
		EssentialMonthlyMinor: in.EssentialMonthlyMinor,
	}

	// ── Cash flow ──
	r.SurplusMonthlyMinor = in.IncomeMonthlyMinor - in.ExpenseMonthlyMinor
	r.SurplusAnnualMinor = r.SurplusMonthlyMinor * 12
	r.SurplusTone = signTone(r.SurplusMonthlyMinor)
	if r.HasIncome {
		r.SavingsRatePct = clampInt(int(r.SurplusMonthlyMinor*100/in.IncomeMonthlyMinor), -100, 100)
		switch {
		case r.SavingsRatePct >= SavingsTargetPct:
			r.SavingsTone = ToneUp
		case r.SavingsRatePct > 0:
			r.SavingsTone = ToneWarn
		default:
			r.SavingsTone = ToneDown
		}
	}

	// ── Debts ──
	var minPayments, total, exMortgage, drag int64
	var weighted float64
	var payoffDebts []payoff.Debt
	for _, d := range in.Debts {
		bal := max(d.BalanceMinor, 0)
		total += bal
		if d.IsMortgage {
			r.HasMortgage = true
		} else {
			exMortgage += bal
		}
		if d.MinPaymentMinor > 0 {
			minPayments += d.MinPaymentMinor
		}
		if d.AprPercent > 0 && bal > 0 {
			weighted += float64(bal) * d.AprPercent
			drag += int64(math.Round(float64(bal) * d.AprPercent / 1200.0))
		}
		if d.InPayoff && bal > 0 {
			payoffDebts = append(payoffDebts, payoff.Debt{
				Name: d.Name, Balance: bal, AprPercent: d.AprPercent, MinPayment: d.MinPaymentMinor,
			})
		}
	}
	r.HasDebts = total > 0
	r.TotalDebtMinor = total
	r.ExMortgageMinor = exMortgage
	r.MinPaymentsMinor = minPayments
	r.AnnualDebtServiceMinor = minPayments * 12
	r.InterestDragMonthlyMinor = drag
	if total > 0 {
		r.WeightedAprPercent = weighted / float64(total)
		switch {
		case r.WeightedAprPercent < aprLowPercent:
			r.WeightedAprTone = ToneUp
		case r.WeightedAprPercent < aprHighPercent:
			r.WeightedAprTone = ToneWarn
		default:
			r.WeightedAprTone = ToneDown
		}
	}
	if r.HasIncome && r.HasDebts {
		r.PaymentShareOfIncomePct = int(minPayments * 100 / in.IncomeMonthlyMinor)
		switch {
		case r.PaymentShareOfIncomePct < PaymentShareTargetPct:
			r.PaymentShareTone = ToneUp
		case r.PaymentShareOfIncomePct < paymentShareCriticalPct:
			r.PaymentShareTone = ToneWarn
		default:
			r.PaymentShareTone = ToneDown
		}
	}
	if len(payoffDebts) > 0 {
		plan, ok := payoff.BuildPlan(payoffDebts, 0, payoff.Avalanche)
		r.PayoffApplicable = true
		if !ok {
			r.PayoffNeverClears = true
			r.PayoffTone = ToneDown
		} else {
			r.PayoffMonths = plan.Months
			switch {
			case plan.Months <= 24:
				r.PayoffTone = ToneUp
			case plan.Months <= 60:
				r.PayoffTone = ToneWarn
			default:
				r.PayoffTone = ToneDown
			}
		}
	}

	// Discretionary: what the month leaves after spending AND required minimums,
	// so it reconciles with the surplus row (discretionary = surplus − minimums).
	// Toned against income, not just sign — a technically-positive sliver is a
	// tight month, not a healthy one.
	r.DiscretionaryMinor = r.SurplusMonthlyMinor - minPayments
	switch {
	case r.DiscretionaryMinor < 0:
		r.DiscretionaryTone = ToneDown
	case r.HasIncome && r.DiscretionaryMinor*100 < in.IncomeMonthlyMinor*int64(discretionaryWarnPctOfIncome):
		r.DiscretionaryTone = ToneWarn
	default:
		r.DiscretionaryTone = ToneUp
	}

	// ── Cushion ──
	r.FundMonths = in.FundMonths
	if r.FundMonths < 1 || r.FundMonths > 24 {
		r.FundMonths = defaultFundMonths
	}
	if in.EssentialMonthlyMinor > 0 {
		r.HasCushion = true
		r.CoverageMonthsTenths = tenths(in.LiquidMinor, in.EssentialMonthlyMinor)
		switch {
		case r.CoverageMonthsTenths >= CoverageTargetTenths:
			r.CoverageTone = ToneUp
		case r.CoverageMonthsTenths >= coverageFloorTenths:
			r.CoverageTone = ToneWarn
		default:
			r.CoverageTone = ToneDown
		}
		r.FundTargetMinor = in.EssentialMonthlyMinor * int64(r.FundMonths)
		r.FundGapMinor = r.FundTargetMinor - in.LiquidMinor
	}
	if burn := in.ExpenseMonthlyMinor + minPayments; burn > 0 {
		r.RunwayAfterDebtTenths = tenths(in.LiquidMinor, burn)
		switch {
		case r.RunwayAfterDebtTenths >= CoverageTargetTenths:
			r.RunwayTone = ToneUp
		case r.RunwayAfterDebtTenths >= coverageFloorTenths:
			r.RunwayTone = ToneWarn
		default:
			r.RunwayTone = ToneDown
		}
	} else {
		r.RunwayAfterDebtTenths = -1
	}

	// ── Revolving credit ──
	r.HasCards = in.Cards.HasCards
	if in.Cards.HasCards {
		r.CardBalanceMinor = in.Cards.BalanceMinor
		r.CardLimitMinor = in.Cards.LimitMinor
		if avail := in.Cards.LimitMinor - in.Cards.BalanceMinor; avail > 0 {
			r.CardAvailableMinor = avail
		}
		if in.Cards.LimitMinor > 0 {
			r.HasUtilization = true
			r.UtilizationPct = clampInt(int(in.Cards.BalanceMinor*100/in.Cards.LimitMinor), 0, 100)
			switch {
			case r.UtilizationPct <= UtilizationTargetPct:
				r.UtilizationTone = ToneUp
			case r.UtilizationPct <= utilizationHighPct:
				r.UtilizationTone = ToneWarn
			default:
				r.UtilizationTone = ToneDown
			}
		}
	}

	return r
}

// signTone maps a signed amount to up/down (zero reads as neutral-positive up:
// breaking even is not a problem state).
func signTone(v int64) Tone {
	if v < 0 {
		return ToneDown
	}
	return ToneUp
}

// tenths returns numerator ÷ denominator in tenths (5.3 → 53), 0-floored.
func tenths(num, den int64) int64 {
	if den <= 0 || num <= 0 {
		return 0
	}
	return num * 10 / den
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

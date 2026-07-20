// SPDX-License-Identifier: MIT

// Package debtcoach turns a snapshot of someone's debts into a short, ranked list
// of plain-English watch-outs — the "is this getting out of control?" read. It is
// pure Go (no platform or i18n dependencies): it decides WHICH concerns apply and
// how serious each is, and hands back structured Alerts. The UI owns the wording
// and the links; this package owns the judgement.
//
// Every threshold is either passed in (the utilization bands come from the user's
// DebtConfig) or a named constant here, so the rules are explicit and testable.
// Amounts are integer minor units (e.g. cents); rates are annual percents.
package debtcoach

import (
	"math"
	"sort"
)

// Severity ranks how loudly an alert should read. Higher is more urgent.
type Severity int

const (
	// Info is a nudge — nothing is wrong, but there's an easy win.
	Info Severity = iota
	// Watch is a real concern that's costing money or trending the wrong way.
	Watch
	// Critical is money actively spiralling: a balance that never clears, or a
	// card past its limit.
	Critical
)

// String returns the lowercase severity name (used as a CSS/i18n discriminator).
func (s Severity) String() string {
	switch s {
	case Critical:
		return "critical"
	case Watch:
		return "watch"
	default:
		return "info"
	}
}

// Alert is one watch-out. Kind is a stable identifier the UI maps to copy; the
// numeric fields carry everything the copy needs to format, so the wording lives
// entirely in the view layer. Subject names the specific debt when the alert is
// about one account (otherwise "").
type Alert struct {
	Kind     string   // stable key, e.g. "over-limit"
	Severity Severity //
	Subject  string   // the debt this is about, or "" for portfolio-wide
	Pct      float64  // a percentage the copy needs (utilization, APR, share…)
	Amount   int64    // a money figure in base minor units the copy needs
	Months   int      // a duration the copy needs
	Count    int      // how many debts match, when more than the named one
}

// DebtLine is one liability in its own currency. The per-debt rules (over its
// limit, a minimum that can't outrun the interest, a punishing APR) compare
// figures within a single account, so they stay correct without FX conversion.
type DebtLine struct {
	Name       string
	Balance    int64   // amount owed, positive, minor units of this debt's currency
	AprPercent float64 // annual interest rate, e.g. 19.99
	MinPayment int64   // required monthly minimum, minor units
	Limit      int64   // credit limit, minor units; 0 when not a line of credit
	Revolving  bool    // true for credit cards / lines of credit
}

// Input is the full debt snapshot. The aggregate figures are already FX-converted
// to the base currency by the caller (the engine surface is the source of truth
// for those); the per-account Debts stay native.
type Input struct {
	Debts []DebtLine

	// Base-currency aggregates (minor units unless noted).
	Assets               int64
	Liabilities          int64
	MinPaymentsTotal     int64   // required minimums per month
	MonthlyInterestTotal int64   // interest accruing per month at current balances
	CreditUtilPct        float64 // aggregate credit-card utilization, 0–100

	// Minimums-only projection for the whole plan (from payoff.BuildPlan at $0 extra).
	MinOnlyMonths int  // months to clear paying only minimums
	MinOnlyOK     bool // whether minimums-only ever clears every debt

	// Utilization bands from the user's DebtConfig (percent).
	WarnUtilPct int
	HighUtilPct int
}

// Rule thresholds. Named so the judgement is legible and adjustable in one place.
const (
	// highAPRPct: at or above this, a debt is "high-interest" — the kind that
	// compounds faster than most people pay it down.
	highAPRPct = 25.0
	// interestHeavyFrac: when this share of the monthly minimums is pure interest,
	// the payments are mostly treading water rather than shrinking the balance.
	interestHeavyFrac = 0.50
	// slowMonths: a minimums-only payoff this long (10 years) is a slog worth
	// flagging even when it does eventually clear.
	slowMonths = 120
)

// Evaluate applies every rule to the snapshot and returns the alerts that fire,
// most urgent first (ties broken by a stable rule order). It emits at most one
// alert per concern so the list stays glanceable — a portfolio with one flaw
// shows one line, not a wall of red.
func Evaluate(in Input) []Alert {
	var out []Alert

	// --- Per-debt rules: report the single worst offender, count the rest. ---

	// Over the credit limit — the balance has passed the card's ceiling, which
	// typically means fees and an immediate credit-score hit.
	if worst, n := worstOverLimit(in.Debts); worst != nil {
		util := 0.0
		if worst.Limit > 0 {
			util = float64(worst.Balance) / float64(worst.Limit) * 100
		}
		out = append(out, Alert{Kind: "over-limit", Severity: Critical, Subject: worst.Name, Pct: util, Count: n})
	}

	// A minimum payment that can't outrun the interest — the balance grows every
	// month no matter how long you pay. This is the definition of out of control.
	// Suppressed when the same debt is already flagged over its limit (that's the
	// louder, more immediate concern — one critical per debt keeps the list clean).
	if worst, n := worstUnderwater(in.Debts); worst != nil && !hasSubject(out, worst.Name) {
		out = append(out, Alert{Kind: "min-underwater", Severity: Critical, Subject: worst.Name, Pct: worst.AprPercent, Count: n})
	}

	// --- Portfolio utilization: emit only the highest band that applies. ---
	switch {
	case in.HighUtilPct > 0 && in.CreditUtilPct >= float64(in.HighUtilPct):
		out = append(out, Alert{Kind: "utilization-high", Severity: Critical, Pct: in.CreditUtilPct})
	case in.WarnUtilPct > 0 && in.CreditUtilPct >= float64(in.WarnUtilPct):
		out = append(out, Alert{Kind: "utilization-warn", Severity: Watch, Pct: in.CreditUtilPct})
	}

	// Owing more than you own — negative net worth on the debt side.
	if in.Assets > 0 && in.Liabilities > in.Assets {
		pct := float64(in.Liabilities) / float64(in.Assets) * 100
		out = append(out, Alert{Kind: "debt-over-assets", Severity: Watch, Pct: pct, Amount: in.Liabilities})
	}

	// A high-interest debt that isn't underwater but is still the expensive one to
	// carry — suppressed when it's already the underwater subject (same debt).
	if worst, n := worstHighAPR(in.Debts); worst != nil && !hasSubject(out, worst.Name) {
		out = append(out, Alert{Kind: "high-apr", Severity: Watch, Subject: worst.Name, Pct: worst.AprPercent, Count: n})
	}

	// Most of the monthly minimums are interest, not principal — the balance is
	// barely moving even though payments are being made.
	if in.MinPaymentsTotal > 0 && in.MonthlyInterestTotal > 0 {
		frac := float64(in.MonthlyInterestTotal) / float64(in.MinPaymentsTotal)
		if frac >= interestHeavyFrac {
			out = append(out, Alert{Kind: "interest-heavy", Severity: Watch, Pct: frac * 100, Amount: in.MonthlyInterestTotal})
		}
	}

	// Minimums clear the debt, but it takes a decade-plus — a nudge that a little
	// extra changes the timeline a lot. Skipped when something more urgent (an
	// underwater debt) already says the same thing louder.
	if in.MinOnlyOK && in.MinOnlyMonths >= slowMonths && !hasKind(out, "min-underwater") {
		out = append(out, Alert{Kind: "slow-payoff", Severity: Info, Months: in.MinOnlyMonths})
	}

	sortAlerts(out)
	return out
}

// worstOverLimit returns the revolving debt furthest over its limit (by balance)
// and how many debts are over their limit in total, or nil when none are.
func worstOverLimit(debts []DebtLine) (*DebtLine, int) {
	var worst *DebtLine
	count := 0
	for i := range debts {
		d := &debts[i]
		if !d.Revolving || d.Limit <= 0 || d.Balance <= d.Limit {
			continue
		}
		count++
		if worst == nil || d.Balance > worst.Balance {
			worst = d
		}
	}
	return worst, count
}

// worstUnderwater returns the debt whose minimum can't cover its monthly interest
// (so the balance never falls), preferring the largest such balance, plus the
// count of underwater debts.
func worstUnderwater(debts []DebtLine) (*DebtLine, int) {
	var worst *DebtLine
	count := 0
	for i := range debts {
		d := &debts[i]
		if d.Balance <= 0 || d.AprPercent <= 0 {
			continue
		}
		monthlyInterest := int64(math.Round(float64(d.Balance) * d.AprPercent / 1200.0))
		if d.MinPayment > monthlyInterest {
			continue
		}
		count++
		if worst == nil || d.Balance > worst.Balance {
			worst = d
		}
	}
	return worst, count
}

// worstHighAPR returns the debt with the highest APR at or above the high-interest
// threshold, and how many debts clear that bar.
func worstHighAPR(debts []DebtLine) (*DebtLine, int) {
	var worst *DebtLine
	count := 0
	for i := range debts {
		d := &debts[i]
		if d.Balance <= 0 || d.AprPercent < highAPRPct {
			continue
		}
		count++
		if worst == nil || d.AprPercent > worst.AprPercent {
			worst = d
		}
	}
	return worst, count
}

// hasSubject reports whether any alert already targets the named debt.
func hasSubject(alerts []Alert, name string) bool {
	for _, a := range alerts {
		if a.Subject == name {
			return true
		}
	}
	return false
}

// hasKind reports whether an alert of the given kind was already emitted.
func hasKind(alerts []Alert, kind string) bool {
	for _, a := range alerts {
		if a.Kind == kind {
			return true
		}
	}
	return false
}

// ruleOrder gives each kind a stable rank so equal-severity alerts sort
// deterministically (and read in a sensible order).
var ruleOrder = map[string]int{
	"over-limit":       0,
	"min-underwater":   1,
	"utilization-high": 2,
	"utilization-warn": 3,
	"debt-over-assets": 4,
	"high-apr":         5,
	"interest-heavy":   6,
	"slow-payoff":      7,
}

// sortAlerts orders alerts by descending severity, then by the stable rule order.
func sortAlerts(alerts []Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity > alerts[j].Severity
		}
		return ruleOrder[alerts[i].Kind] < ruleOrder[alerts[j].Kind]
	})
}

// SPDX-License-Identifier: MIT

// Package unusual detects explainable unusual spending using deterministic,
// local-only rules and integer minor-unit amounts.
package unusual

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Signal identifies the kind of unusual activity found.
type Signal string

const (
	// Spike is a material purchase above its robust merchant or category baseline.
	Spike Signal = "Spike"
	// PriceIncrease is a recurring merchant charge that rose for two cycles.
	PriceIncrease Signal = "PriceIncrease"
	// PossibleDoubleCharge is a same-merchant, same-amount charge within three days.
	PossibleDoubleCharge Signal = "PossibleDoubleCharge"
	// FirstTimeMerchantLarge is a material charge from a merchant with no prior history.
	FirstTimeMerchantLarge Signal = "FirstTimeMerchantLarge"
)

// Confidence is an explainability grade, not a probability.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// BaselineSource identifies the transaction group used as a baseline.
type BaselineSource string

const (
	BaselineMerchant BaselineSource = "merchant"
	BaselineCategory BaselineSource = "category"
)

// Evidence contains plain, reproducible inputs behind a finding.
type Evidence struct {
	AmountMinor         int64          `json:"amount_minor"`
	CurrencyCode        string         `json:"currency_code,omitempty"`
	TypicalMinor        int64          `json:"typical_minor,omitempty"`
	DeltaMinor          int64          `json:"delta_minor,omitempty"`
	MADMinor            int64          `json:"mad_minor,omitempty"`
	Method              string         `json:"method"`
	BaselineSource      BaselineSource `json:"baseline_source,omitempty"`
	HistoryCount        int            `json:"history_count,omitempty"`
	Window              string         `json:"window,omitempty"`
	RelatedTxnIDs       []string       `json:"related_txn_ids,omitempty"`
	ExpectationEvidence []string       `json:"expectation_evidence,omitempty"`
	Confirmation        string         `json:"confirmation,omitempty"`
}

// Finding is a detector result for one transaction.
type Finding struct {
	TxnID      string     `json:"txn_id"`
	Signal     Signal     `json:"signal"`
	Confidence Confidence `json:"confidence"`
	Evidence   Evidence   `json:"evidence"`
	Suppressed bool       `json:"suppressed,omitempty"`
}

// Settings contains configurable detector thresholds.
type Settings struct {
	FirstTimeMerchantFloorMinor int64
}

// Input is one immutable ledger snapshot and the context needed for detection.
type Input struct {
	Transactions []domain.Transaction
	Categories   []domain.Category
	Accounts     []domain.Account
	Recurring    []domain.Recurring
	Goals        []domain.Goal
	Budgets      []domain.Budget
	Settings     Settings
	Now          time.Time
	Rates        currency.Rates
	// ExpectedTxnIDs carries explicit "expected" or purpose-made envelope links
	// supplied by a caller when the domain has no durable field for that marker.
	ExpectedTxnIDs map[string][]string
}

const (
	merchantMinimum = 4
	categoryMinimum = 8
	baselineMonths  = 24
	spikeFloor      = 5000
	spikeDeltaFloor = 2500
)

type charge struct {
	txn   domain.Transaction
	minor int64
	key   string
	date  time.Time
}

// Detect returns findings in date then transaction-ID order. No wall-clock
// value is consulted; candidates after Input.Now are ignored.
func Detect(in Input) []Finding {
	job := NewJob(in)
	for !job.Step(256) {
	}
	return job.Findings()
}

func minHistory(source BaselineSource) int {
	if source == BaselineMerchant {
		return merchantMinimum
	}
	return categoryMinimum
}
func withinWindow(date, end time.Time, months int) bool {
	if end.IsZero() {
		return true
	}
	return !date.Before(end.AddDate(0, -months, 0)) && !date.After(end)
}
func baseMinor(amount money.Money, in Input) (int64, bool) {
	abs, err := amount.AbsChecked()
	if err != nil {
		return 0, false
	}
	if in.Rates.Base == "" || strings.EqualFold(amount.Currency, in.Rates.Base) {
		return abs.Amount, true
	}
	converted, err := in.Rates.ToBase(abs)
	if err != nil {
		return 0, false
	}
	return converted.Amount, true
}

func baseCurrency(amount money.Money, in Input) string {
	if in.Rates.Base != "" {
		return strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	}
	return strings.ToUpper(strings.TrimSpace(amount.Currency))
}

func sameComparisonCurrency(a, b string, in Input) bool {
	return in.Rates.Base != "" || strings.EqualFold(a, b)
}
func median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]int64(nil), values...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	mid := len(v) / 2
	if len(v)%2 == 1 {
		return v[mid]
	}
	return v[mid-1] + (v[mid]-v[mid-1])/2
}
func mad(values []int64, center int64) int64 {
	diffs := make([]int64, len(values))
	for i, v := range values {
		if v >= center {
			diffs[i] = v - center
		} else {
			diffs[i] = center - v
		}
	}
	return median(diffs)
}
func mulRatio(n, a, b int64) int64 {
	if b == 0 {
		return 0
	}
	if n > math.MaxInt64/a {
		return math.MaxInt64
	}
	return n * a / b
}
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func latestIDs(history []charge, limit int) []string {
	sort.Slice(history, func(i, j int) bool {
		if !history[i].date.Equal(history[j].date) {
			return history[i].date.After(history[j].date)
		}
		return history[i].txn.ID < history[j].txn.ID
	})
	if len(history) > limit {
		history = history[:limit]
	}
	out := make([]string, len(history))
	for i, h := range history {
		out[i] = h.txn.ID
	}
	sort.Strings(out)
	return out
}
func expectationEvidence(c charge, in Input, goals map[string]domain.Goal, seasonalYears map[string]map[time.Month]map[int]bool) []string {
	t := c.txn
	var evidence []string
	if values := in.ExpectedTxnIDs[t.ID]; len(values) > 0 {
		evidence = append(evidence, values...)
	}
	if t.GoalID != "" {
		if _, ok := goals[t.GoalID]; ok {
			evidence = append(evidence, "goal:"+t.GoalID)
		}
	}
	for _, goal := range in.Goals {
		if goal.IsSinkingFund && t.CategoryID != "" && goal.CategoryID == t.CategoryID {
			evidence = append(evidence, "sinking_fund:"+goal.ID)
		}
	}
	for _, r := range in.Recurring {
		if !r.Paused && recurrenceMatches(t, r) && recurringAmountExpected(c, r, in) {
			evidence = append(evidence, "recurring:"+r.ID)
		}
	}
	for _, budget := range in.Budgets {
		if budget.Notes != "" && strings.Contains(strings.ToLower(budget.Notes), "made for "+strings.ToLower(t.ID)) {
			evidence = append(evidence, "envelope:"+budget.ID)
		}
	}
	priorSeasons := 0
	for year := range seasonalYears[c.key][t.Date.Month()] {
		if year < t.Date.Year() {
			priorSeasons++
		}
	}
	if priorSeasons >= 2 {
		evidence = append(evidence, "seasonal_cycles:2")
	}
	sort.Strings(evidence)
	return compact(evidence)
}

func recurrenceMatches(t domain.Transaction, r domain.Recurring) bool {
	name := strings.ToLower(strings.TrimSpace(t.SubscriptionName))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(t.Payee))
	}
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(t.Desc))
	}
	label := strings.ToLower(strings.TrimSpace(r.Label))
	if label == "" || (name != label && !strings.Contains(name, label) && !strings.Contains(label, name)) {
		return false
	}
	return true
}

func currencyThreshold(minorUSD int64, code string) int64 {
	decimals := currency.Decimals(code)
	if decimals == 2 {
		return minorUSD
	}
	if decimals < 2 {
		for range 2 - decimals {
			minorUSD /= 10
		}
		return minorUSD
	}
	for range decimals - 2 {
		if minorUSD > math.MaxInt64/10 {
			return math.MaxInt64
		}
		minorUSD *= 10
	}
	return minorUSD
}

func comparisonCurrency(amount money.Money, in Input) string {
	if strings.TrimSpace(in.Rates.Base) != "" {
		return in.Rates.Base
	}
	return amount.Currency
}

func increaseThresholdFor(expected int64, amount money.Money, in Input) int64 {
	return maxInt64(mulRatio(expected, 10, 100), currencyThreshold(200, comparisonCurrency(amount, in)))
}

func stableAmounts(values []int64, typical int64, amount money.Money, in Input) bool {
	tolerance := increaseThresholdFor(typical, amount, in)
	for _, value := range values {
		if value < typical-tolerance || value > typical+tolerance {
			return false
		}
	}
	return true
}

func amountNear(a, b, tolerance int64) bool {
	if a >= b {
		return a-b <= tolerance
	}
	return b-a <= tolerance
}

func cadenceGapMatched(a, b, c time.Time) bool {
	first := int64(b.Sub(a) / (24 * time.Hour))
	second := int64(c.Sub(b) / (24 * time.Hour))
	if first < 1 || first > 400 || second < 1 || second > 400 {
		return false
	}
	center := (first + second) / 2
	tolerance := maxInt64(3, mulRatio(center, 15, 100))
	return first >= center-tolerance && first <= center+tolerance && second >= center-tolerance && second <= center+tolerance
}

func recurringAmountExpected(candidate charge, recurring domain.Recurring, in Input) bool {
	expected, ok := baseMinor(recurring.Amount, in)
	if !ok || expected == 0 {
		return true
	}
	delta := candidate.minor - expected
	if delta < 0 {
		delta = -delta
	}
	return delta <= maxInt64(mulRatio(expected, 10, 100), 200)
}

func compact(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, v := range values[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

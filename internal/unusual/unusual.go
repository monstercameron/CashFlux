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
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeebase"
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
	if in.Settings.FirstTimeMerchantFloorMinor <= 0 {
		in.Settings.FirstTimeMerchantFloorMinor = 25000
	}
	transactions := append([]domain.Transaction(nil), in.Transactions...)
	sort.Slice(transactions, func(i, j int) bool {
		if !transactions[i].Date.Equal(transactions[j].Date) {
			return transactions[i].Date.Before(transactions[j].Date)
		}
		return transactions[i].ID < transactions[j].ID
	})
	goalByID := make(map[string]domain.Goal, len(in.Goals))
	for _, g := range in.Goals {
		goalByID[g.ID] = g
	}
	charges := make([]charge, 0, len(transactions))
	byID := make(map[string]charge, len(transactions))
	merchantGroups := make(map[string][]charge)
	categoryGroups := make(map[string][]charge)
	seasonalYears := make(map[string]map[time.Month]map[int]bool)
	for _, txn := range transactions {
		if !txn.IsExpense() || txn.ID == "" || txn.Date.IsZero() || (in.Now.IsZero() == false && txn.Date.After(in.Now)) || txn.ExcludeFromReports {
			continue
		}
		if len(txn.Splits) > 0 {
			continue
		} // split parents are not whole-category purchases
		minor, ok := baseMinor(txn.Amount, in)
		if !ok || minor <= 0 {
			continue
		}
		c := charge{txn: txn, minor: minor, key: payeebase.Key(txn), date: txn.Date}
		charges = append(charges, c)
		byID[txn.ID] = c
		if c.key != "" {
			merchantGroups[c.key] = append(merchantGroups[c.key], c)
			if seasonalYears[c.key] == nil {
				seasonalYears[c.key] = make(map[time.Month]map[int]bool)
			}
			if seasonalYears[c.key][txn.Date.Month()] == nil {
				seasonalYears[c.key][txn.Date.Month()] = make(map[int]bool)
			}
			seasonalYears[c.key][txn.Date.Month()][txn.Date.Year()] = true
		}
		if txn.CategoryID != "" {
			categoryGroups[txn.CategoryID] = append(categoryGroups[txn.CategoryID], c)
		}
	}
	duplicateIDs := duplicateSuspects(charges)
	var findings []Finding
	for _, candidate := range charges {
		if !withinWindow(candidate.date, in.Now, 24) || duplicateIDs[candidate.txn.ID] {
			continue
		}
		expectations := expectationEvidence(candidate, in, goalByID, seasonalYears)
		if len(expectations) > 0 {
			continue
		}
		merchantHistory := make([]charge, 0)
		categoryHistory := make([]charge, 0)
		for _, other := range merchantGroups[candidate.key] {
			if other.txn.ID == candidate.txn.ID || !sameComparisonCurrency(other.txn.Amount.Currency, candidate.txn.Amount.Currency, in) || !other.date.Before(candidate.date) || !withinWindow(other.date, candidate.date, baselineMonths) || duplicateIDs[other.txn.ID] {
				continue
			}
			merchantHistory = append(merchantHistory, other)
		}
		for _, other := range categoryGroups[candidate.txn.CategoryID] {
			if other.txn.ID == candidate.txn.ID || !sameComparisonCurrency(other.txn.Amount.Currency, candidate.txn.Amount.Currency, in) || !other.date.Before(candidate.date) || !withinWindow(other.date, candidate.date, baselineMonths) || duplicateIDs[other.txn.ID] {
				continue
			}
			categoryHistory = append(categoryHistory, other)
		}
		baseline, source := merchantHistory, BaselineMerchant
		if len(baseline) < merchantMinimum {
			baseline, source = categoryHistory, BaselineCategory
		}
		if len(merchantHistory) == 0 && candidate.key != "" && candidate.minor >= in.Settings.FirstTimeMerchantFloorMinor {
			findings = append(findings, Finding{TxnID: candidate.txn.ID, Signal: FirstTimeMerchantLarge, Confidence: ConfidenceLow,
				Evidence: Evidence{AmountMinor: candidate.minor, CurrencyCode: baseCurrency(candidate.txn.Amount, in), Method: "first_merchant_charge", Window: "24 months"}})
		}
		if len(baseline) >= minHistory(source) {
			vals := make([]int64, len(baseline))
			for i, item := range baseline {
				vals[i] = item.minor
			}
			median := median(vals)
			mad := mad(vals, median)
			delta := candidate.minor - median
			tolerance := maxInt64(mulRatio(median, 20, 100), 500)
			if mad > 0 {
				tolerance = maxInt64(mulRatio(mad, 3, 1), 500)
			}
			if candidate.minor >= spikeFloor && delta >= spikeDeltaFloor && candidate.minor >= mulRatio(median, 3, 1) && delta > tolerance {
				confidence := ConfidenceMedium
				if len(baseline) >= 8 && candidate.minor >= mulRatio(median, 5, 1) {
					confidence = ConfidenceHigh
				}
				findings = append(findings, Finding{TxnID: candidate.txn.ID, Signal: Spike, Confidence: confidence, Evidence: Evidence{
					AmountMinor: candidate.minor, CurrencyCode: baseCurrency(candidate.txn.Amount, in), TypicalMinor: median, DeltaMinor: delta, MADMinor: mad, Method: "median_mad", BaselineSource: source,
					HistoryCount: len(baseline), Window: "24 months"}})
			}
		}
		if len(merchantHistory) == 0 {
			continue
		}
		if increase, ok := recurringPriceIncrease(candidate, merchantHistory, merchantGroups[candidate.key], in); ok {
			findings = append(findings, increase)
		}
	}
	findings = append(findings, doubleChargeFindings(charges, duplicateIDs, in, goalByID, seasonalYears)...)
	sort.Slice(findings, func(i, j int) bool {
		a, b := byID[findings[i].TxnID], byID[findings[j].TxnID]
		if !a.date.Equal(b.date) {
			return a.date.Before(b.date)
		}
		if findings[i].TxnID != findings[j].TxnID {
			return findings[i].TxnID < findings[j].TxnID
		}
		return findings[i].Signal < findings[j].Signal
	})
	return findings
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
	if in.Rates.Base == "" || strings.EqualFold(amount.Currency, in.Rates.Base) {
		return amount.Abs().Amount, true
	}
	converted, err := in.Rates.ToBase(amount.Abs())
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
func duplicateSuspects(charges []charge) map[string]bool {
	marked := map[string]bool{}
	// Canonical same-day import duplicates are always excluded from baseline math.
	txns := make([]domain.Transaction, len(charges))
	for i, c := range charges {
		txns[i] = c.txn
	}
	for _, group := range dedupe.FindDuplicates(txns) {
		for _, id := range group.IDs {
			marked[id] = true
		}
	}
	return marked
}

func doubleChargeFindings(charges []charge, excluded map[string]bool, in Input, goals map[string]domain.Goal, seasonalYears map[string]map[time.Month]map[int]bool) []Finding {
	var out []Finding
	for i, a := range charges {
		if a.key == "" || len(expectationEvidence(a, in, goals, seasonalYears)) > 0 {
			continue
		}
		var related []string
		for j := i + 1; j < len(charges); j++ {
			b := charges[j]
			if b.date.Sub(a.date) > 72*time.Hour {
				break
			}
			if excluded[a.txn.ID] || excluded[b.txn.ID] || !sameComparisonCurrency(a.txn.Amount.Currency, b.txn.Amount.Currency, in) || b.key != a.key || b.minor != a.minor || b.txn.ID == a.txn.ID {
				continue
			}
			if len(expectationEvidence(b, in, goals, seasonalYears)) > 0 {
				continue
			}
			related = append(related, b.txn.ID)
		}
		if len(related) > 0 {
			sort.Strings(related)
			out = append(out, Finding{TxnID: a.txn.ID, Signal: PossibleDoubleCharge, Confidence: ConfidenceHigh, Evidence: Evidence{AmountMinor: a.minor, CurrencyCode: baseCurrency(a.txn.Amount, in), Method: "same_merchant_amount_within_3_days", HistoryCount: 1, Window: "1–3 days", RelatedTxnIDs: related}})
			for _, id := range related {
				out = append(out, Finding{TxnID: id, Signal: PossibleDoubleCharge, Confidence: ConfidenceHigh, Evidence: Evidence{AmountMinor: a.minor, CurrencyCode: baseCurrency(a.txn.Amount, in), Method: "same_merchant_amount_within_3_days", HistoryCount: 1, Window: "1–3 days", RelatedTxnIDs: []string{a.txn.ID}}})
			}
		}
	}
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

func recurringPriceIncrease(candidate charge, history, series []charge, in Input) (Finding, bool) {
	if len(history) < 3 {
		return Finding{}, false
	}
	ordered := append([]charge(nil), history...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].date.Before(ordered[j].date) })
	series = append([]charge(nil), series...)
	sort.Slice(series, func(i, j int) bool { return series[i].date.Before(series[j].date) })

	// Any stable, cadence-matched three-charge block can establish the prior
	// amount. Prefer the most recent such block that is below this candidate.
	var baseline int64
	var baselineStart int = -1
	for i := len(ordered) - 3; i >= 0; i-- {
		block := ordered[i : i+3]
		values := []int64{block[0].minor, block[1].minor, block[2].minor}
		typical := median(values)
		if !stableAmounts(values, typical) || candidate.minor-typical < increaseThreshold(typical) {
			continue
		}
		points := make([]time.Time, 0, len(ordered)-i+1)
		for _, item := range ordered[i:] {
			points = append(points, item.date)
		}
		points = append(points, candidate.date)
		if !cadenceMatched(points) {
			continue
		}
		baseline, baselineStart = typical, i
		break
	}
	if baselineStart < 0 {
		return Finding{}, false
	}

	confidence := ConfidenceMedium
	var related []string
	if baselineStart+3 < len(ordered) {
		newAmount := ordered[baselineStart+3].minor
		confirmed := amountNear(candidate.minor, newAmount, increaseThreshold(baseline))
		for _, item := range ordered[baselineStart+3:] {
			if item.minor-baseline < increaseThreshold(baseline) || !amountNear(item.minor, newAmount, increaseThreshold(baseline)) {
				confirmed = false
				break
			}
			related = append(related, item.txn.ID)
		}
		if confirmed && candidate.minor-baseline >= increaseThreshold(baseline) {
			confidence = ConfidenceHigh
		}
	}
	if confidence == ConfidenceMedium {
		related = append(related, ordered[len(ordered)-1].txn.ID)
	}
	confirmation := ""
	if confidence == ConfidenceMedium && confirmedByNextCharge(candidate, series, in) {
		confirmation = "confirmed by the next charge"
	}
	return Finding{TxnID: candidate.txn.ID, Signal: PriceIncrease, Confidence: confidence, Evidence: Evidence{
		AmountMinor: candidate.minor, TypicalMinor: baseline, DeltaMinor: candidate.minor - baseline, Method: "established_recurring_series",
		CurrencyCode: baseCurrency(candidate.txn.Amount, in), BaselineSource: BaselineMerchant, HistoryCount: len(ordered),
		Window: "prior cadence-matched charges", RelatedTxnIDs: related, Confirmation: confirmation}}, true
}

func increaseThreshold(expected int64) int64 {
	return maxInt64(mulRatio(expected, 10, 100), 200)
}

func stableAmounts(values []int64, typical int64) bool {
	tolerance := increaseThreshold(typical)
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

func cadenceMatched(points []time.Time) bool {
	if len(points) < 4 {
		return false
	}
	gaps := make([]int64, len(points)-1)
	for i := 1; i < len(points); i++ {
		gap := int64(points[i].Sub(points[i-1]) / (24 * time.Hour))
		if gap < 1 || gap > 400 {
			return false
		}
		gaps[i-1] = gap
	}
	center := median(gaps)
	tolerance := maxInt64(3, mulRatio(center, 15, 100))
	for _, gap := range gaps {
		if gap < center-tolerance || gap > center+tolerance {
			return false
		}
	}
	return true
}

func confirmedByNextCharge(candidate charge, series []charge, in Input) bool {
	var previous time.Time
	for _, prior := range series {
		if prior.date.Before(candidate.date) && prior.date.After(previous) {
			previous = prior.date
		}
	}
	if previous.IsZero() {
		return false
	}
	for _, next := range series {
		if !next.date.After(candidate.date) || !sameComparisonCurrency(candidate.txn.Amount.Currency, next.txn.Amount.Currency, in) {
			continue
		}
		if !cadenceGapMatched(previous, candidate.date, next.date) {
			return false
		}
		if amountNear(next.minor, candidate.minor, increaseThreshold(candidate.minor)) {
			return true
		}
		// The first later merchant charge determines whether this was a
		// one-off; do not skip a return to the old amount for a later match.
		return false
	}
	return false
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

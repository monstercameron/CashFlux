// SPDX-License-Identifier: MIT

package unusual

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeebase"
)

// Job incrementally evaluates an immutable ledger without truncating history.
// Call Step between browser tasks and publish Findings only when it returns true.
// The input and nested slices/maps must remain immutable until completion.
type Job struct {
	price                            *priceJob
	in                               Input
	phase, position                  int
	charges                          []charge
	byID                             map[string]charge
	goals                            map[string]domain.Goal
	merchants, categories            map[string][]charge
	seasons                          map[string]map[time.Month]map[int]bool
	duplicates                       map[string]bool
	refunds                          map[string]map[string]int64
	firstDuplicate                   map[string]string
	expected                         map[string]bool
	doubles                          map[doubleKey][]charge
	sortCharges                      *mergeJob[charge]
	sortFindings                     *mergeJob[Finding]
	findings                         []Finding
	candidate                        charge
	merchantHistory, categoryHistory []charge
	history                          []charge
	historyPosition, historyKind     int
	doubleSeries                     []charge
	doublePosition                   int
	related                          []string
}

type doubleKey struct {
	merchant, currency string
	minor              int64
}

// NewJob creates an empty detector job. No ledger scan or sort runs here.
func NewJob(in Input) *Job {
	if in.Settings.FirstTimeMerchantFloorMinor <= 0 {
		in.Settings.FirstTimeMerchantFloorMinor = 25000
	}
	return &Job{in: in, byID: map[string]charge{}, goals: map[string]domain.Goal{},
		merchants: map[string][]charge{}, categories: map[string][]charge{}, seasons: map[string]map[time.Month]map[int]bool{},
		duplicates: map[string]bool{}, refunds: map[string]map[string]int64{}, firstDuplicate: map[string]string{}, expected: map[string]bool{}, doubles: map[doubleKey][]charge{}}
}

// Step performs at most work index, history, pair or merge operations. A budget
// of 32 is suitable for interactive callers; elapsed time depends on host speed.
// Zero/negative budgets do no work. Completed jobs are stable and idempotent.
func (j *Job) Step(work int) bool {
	for ; work > 0 && j.phase < 8; work-- {
		switch j.phase {
		case 0:
			if j.position < len(j.in.Goals) {
				goal := j.in.Goals[j.position]
				j.goals[goal.ID] = goal
				j.position++
				continue
			}
			j.phase, j.position = 1, 0
		case 1:
			if j.position == len(j.in.Transactions) {
				j.sortCharges = newMergeJob(j.charges, chargeLess)
				j.phase, j.position = 2, 0
				continue
			}
			txn := j.in.Transactions[j.position]
			j.position++
			if txn.Amount.IsPositive() && txn.ID != "" && !txn.Date.IsZero() && !txn.IsTransfer() {
				minor, ok := baseMinor(txn.Amount, j.in)
				key := payeebase.Key(txn)
				if ok && key != "" {
					if j.refunds[key] == nil {
						j.refunds[key] = map[string]int64{}
					}
					day := txn.Date.Format("2006-01-02")
					amount, err := money.Add(j.refunds[key][day], minor)
					if err != nil {
						amount = math.MaxInt64
					}
					j.refunds[key][day] = amount
				}
			}
			if !txn.IsExpense() || txn.ID == "" || txn.Date.IsZero() || (!j.in.Now.IsZero() && txn.Date.After(j.in.Now)) || txn.ExcludeFromReports || len(txn.Splits) > 0 {
				continue
			}
			minor, ok := baseMinor(txn.Amount, j.in)
			if !ok || minor <= 0 {
				continue
			}
			c := charge{txn: txn, minor: minor, key: payeebase.Key(txn), date: txn.Date}
			j.charges = append(j.charges, c)
			key := dedupe.Key(txn)
			if first := j.firstDuplicate[key]; first != "" {
				j.duplicates[first], j.duplicates[txn.ID] = true, true
			} else {
				j.firstDuplicate[key] = txn.ID
			}
		case 2:
			if !j.sortCharges.step() {
				continue
			}
			j.charges = j.sortCharges.values
			j.sortCharges = nil
			j.phase = 3
		case 3:
			if j.position == len(j.charges) {
				j.phase, j.position = 4, 0
				continue
			}
			c := j.charges[j.position]
			j.position++
			j.byID[c.txn.ID] = c
			if c.key != "" {
				j.merchants[c.key] = append(j.merchants[c.key], c)
				if j.seasons[c.key] == nil {
					j.seasons[c.key] = map[time.Month]map[int]bool{}
				}
				if j.seasons[c.key][c.date.Month()] == nil {
					j.seasons[c.key][c.date.Month()] = map[int]bool{}
				}
				j.seasons[c.key][c.date.Month()][c.date.Year()] = true
			}
			if c.txn.CategoryID != "" {
				j.categories[c.txn.CategoryID] = append(j.categories[c.txn.CategoryID], c)
			}
			j.doubles[j.doubleKey(c)] = append(j.doubles[j.doubleKey(c)], c)
		case 4:
			if j.position == len(j.charges) {
				j.phase, j.position = 5, 0
				continue
			}
			c := j.charges[j.position]
			j.position++
			j.expected[c.txn.ID] = len(expectationEvidence(c, j.in, j.goals, j.seasons)) > 0
		case 5:
			if j.price != nil {
				if j.price.step() {
					if j.price.found {
						j.findings = append(j.findings, j.price.finding)
					}
					j.price = nil
				}
				continue
			}
			if j.historyKind != 0 {
				j.stepHistory()
				continue
			}
			if j.position == len(j.charges) {
				j.phase, j.position = 6, 0
				continue
			}
			c := j.charges[j.position]
			j.position++
			if !withinWindow(c.date, j.in.Now, 24) || j.duplicates[c.txn.ID] || j.expected[c.txn.ID] {
				continue
			}
			if refund := refundAfter(c, j.refunds[c.key]); refund >= c.minor {
				continue
			}
			j.candidate = c
			j.merchantHistory, j.categoryHistory = nil, nil
			j.historyKind = 1
			j.setHistory(j.merchants[c.key])
		case 6:
			if j.doubleSeries != nil {
				j.stepDouble()
				continue
			}
			if j.position == len(j.charges) {
				j.sortFindings = newMergeJob(j.findings, func(a, b Finding) bool {
					ac, bc := j.byID[a.TxnID], j.byID[b.TxnID]
					if !ac.date.Equal(bc.date) {
						return ac.date.Before(bc.date)
					}
					if a.TxnID != b.TxnID {
						return a.TxnID < b.TxnID
					}
					return a.Signal < b.Signal
				})
				j.phase = 7
				continue
			}
			c := j.charges[j.position]
			j.position++
			if c.key == "" || j.expected[c.txn.ID] {
				continue
			}
			series := j.doubles[j.doubleKey(c)]
			start := sort.Search(len(series), func(i int) bool { return chargeLess(c, series[i]) })
			j.candidate, j.doubleSeries, j.doublePosition, j.related = c, series, start, nil
		case 7:
			if !j.sortFindings.step() {
				continue
			}
			j.findings, j.sortFindings, j.phase = j.sortFindings.values, nil, 8
		}
	}
	return j.phase == 8
}

// Findings returns completed findings in deterministic date/ID/signal order.
// It returns nil while the job is incomplete. Treat the result as immutable.
func (j *Job) Findings() []Finding {
	if j.phase != 8 {
		return nil
	}
	return j.findings
}

func chargeLess(a, b charge) bool {
	if !a.date.Equal(b.date) {
		return a.date.Before(b.date)
	}
	return a.txn.ID < b.txn.ID
}

func (j *Job) doubleKey(c charge) doubleKey {
	currency := ""
	if j.in.Rates.Base == "" {
		currency = strings.ToUpper(c.txn.Amount.Currency)
	}
	return doubleKey{merchant: c.key, currency: currency, minor: c.minor}
}

func (j *Job) setHistory(series []charge) {
	start := sort.Search(len(series), func(i int) bool { return !series[i].date.Before(j.candidate.date.AddDate(0, -baselineMonths, 0)) })
	end := sort.Search(len(series), func(i int) bool { return !series[i].date.Before(j.candidate.date) })
	j.history, j.historyPosition = series[start:end], 0
}

func (j *Job) stepHistory() {
	if j.historyPosition < len(j.history) {
		other := j.history[j.historyPosition]
		j.historyPosition++
		if other.txn.ID == j.candidate.txn.ID || j.duplicates[other.txn.ID] || !sameComparisonCurrency(other.txn.Amount.Currency, j.candidate.txn.Amount.Currency, j.in) {
			return
		}
		if j.historyKind == 1 {
			j.merchantHistory = append(j.merchantHistory, other)
		} else {
			j.categoryHistory = append(j.categoryHistory, other)
		}
		return
	}
	if j.historyKind == 1 && len(j.merchantHistory) < merchantMinimum {
		j.historyKind = 2
		j.setHistory(j.categories[j.candidate.txn.CategoryID])
		return
	}
	candidate := j.candidate
	if refund := refundAfter(candidate, j.refunds[candidate.key]); refund > 0 {
		candidate.minor -= refund
	}
	j.findings = append(j.findings, evaluateCandidate(candidate, j.merchantHistory, j.categoryHistory, j.in)...)
	if len(j.merchantHistory) >= 3 {
		j.price = newPriceJob(candidate, j.merchantHistory, j.merchants[j.candidate.key], j.in)
	}
	j.history, j.merchantHistory, j.categoryHistory, j.historyKind = nil, nil, nil, 0
}

func (j *Job) stepDouble() {
	a := j.candidate
	if j.doublePosition < len(j.doubleSeries) {
		b := j.doubleSeries[j.doublePosition]
		j.doublePosition++
		if b.date.Sub(a.date) <= 72*time.Hour {
			if !j.expected[b.txn.ID] && b.txn.ID != a.txn.ID {
				j.related = append(j.related, b.txn.ID)
			}
			return
		}
	}
	if len(j.related) > 0 {
		sort.Strings(j.related)
		j.findings = append(j.findings, doubleFinding(a, j.related, j.in))
		for _, id := range j.related {
			c := a
			c.txn.ID = id
			j.findings = append(j.findings, doubleFinding(c, []string{a.txn.ID}, j.in))
		}
	}
	j.doubleSeries, j.related = nil, nil
}

func refundAfter(candidate charge, refunds map[string]int64) int64 {
	var total int64
	for offset := range 31 {
		day := candidate.date.AddDate(0, 0, offset).Format("2006-01-02")
		next, err := money.Add(total, refunds[day])
		if err != nil {
			return candidate.minor
		}
		total = next
	}
	if total > candidate.minor {
		return candidate.minor
	}
	return total
}

func doubleFinding(c charge, related []string, in Input) Finding {
	return Finding{TxnID: c.txn.ID, Signal: PossibleDoubleCharge, Confidence: ConfidenceHigh, Evidence: Evidence{AmountMinor: c.minor, CurrencyCode: baseCurrency(c.txn.Amount, in), Method: "same_merchant_amount_within_3_days", HistoryCount: 1, Window: "1–3 days", RelatedTxnIDs: related}}
}

// mergeJob does a stable bottom-up merge sort one comparison/copy at a time.
type mergeJob[T any] struct {
	values, scratch                  []T
	less                             func(T, T) bool
	width, left, i, k, end, mid, out int
	merging                          bool
}

func newMergeJob[T any](values []T, less func(T, T) bool) *mergeJob[T] {
	return &mergeJob[T]{values: values, scratch: make([]T, len(values)), less: less, width: 1}
}

func (m *mergeJob[T]) step() bool {
	if m.width >= len(m.values) {
		return true
	}
	if !m.merging {
		if m.left >= len(m.values) {
			m.values, m.scratch = m.scratch, m.values
			m.width *= 2
			m.left = 0
			return m.width >= len(m.values)
		}
		m.i, m.mid, m.end, m.out = m.left, min(m.left+m.width, len(m.values)), min(m.left+2*m.width, len(m.values)), m.left
		m.k = m.mid
		m.merging = true
	}
	if m.out == m.end {
		m.left = m.end
		m.merging = false
		return false
	}
	if m.i < m.mid && (m.k == m.end || !m.less(m.values[m.k], m.values[m.i])) {
		m.scratch[m.out] = m.values[m.i]
		m.i++
	} else {
		m.scratch[m.out] = m.values[m.k]
		m.k++
	}
	m.out++
	return false
}

func evaluateCandidate(candidate charge, merchantHistory, categoryHistory []charge, in Input) []Finding {
	var findings []Finding

	baseline, source := merchantHistory, BaselineMerchant
	if len(baseline) < merchantMinimum {
		baseline, source = categoryHistory, BaselineCategory
	}
	floor := in.Settings.FirstTimeMerchantFloorMinor
	if floor == 25000 {
		floor = currencyThreshold(25000, comparisonCurrency(candidate.txn.Amount, in))
	}
	if len(merchantHistory) == 0 && candidate.key != "" && candidate.minor >= floor {
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
		delta, err := money.Sub(candidate.minor, median)
		if err != nil {
			delta = math.MaxInt64
		}
		comparisonCode := comparisonCurrency(candidate.txn.Amount, in)
		tolerance := maxInt64(mulRatio(median, 20, 100), currencyThreshold(500, comparisonCode))
		if mad > 0 {
			tolerance = maxInt64(mulRatio(mad, 3, 1), currencyThreshold(500, comparisonCode))
		}
		if candidate.minor >= currencyThreshold(spikeFloor, comparisonCode) && delta >= currencyThreshold(spikeDeltaFloor, comparisonCode) && candidate.minor >= mulRatio(median, 3, 1) && delta > tolerance {
			confidence := ConfidenceMedium
			if len(baseline) >= 8 && candidate.minor >= mulRatio(median, 5, 1) {
				confidence = ConfidenceHigh
			}
			findings = append(findings, Finding{TxnID: candidate.txn.ID, Signal: Spike, Confidence: confidence, Evidence: Evidence{
				AmountMinor: candidate.minor, CurrencyCode: baseCurrency(candidate.txn.Amount, in), TypicalMinor: median, DeltaMinor: delta, MADMinor: mad, Method: "median_mad", BaselineSource: source,
				HistoryCount: len(baseline), Window: "24 months"}})
		}
	}
	return findings
}

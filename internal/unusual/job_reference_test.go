// SPDX-License-Identifier: MIT
package unusual

import (
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeebase"
)

// referenceDetect preserves the pre-job algorithm as an equivalence oracle.
func referenceDetect(in Input) []Finding {
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
	refunds := make(map[string]map[string]int64)
	for _, txn := range transactions {
		if txn.Amount.IsPositive() && txn.ID != "" && !txn.Date.IsZero() && !txn.IsTransfer() {
			minor, ok := baseMinor(txn.Amount, in)
			key := payeebase.Key(txn)
			if ok && key != "" {
				if refunds[key] == nil {
					refunds[key] = make(map[string]int64)
				}
				amount, err := money.Add(refunds[key][txn.Date.Format("2006-01-02")], minor)
				if err != nil {
					amount = math.MaxInt64
				}
				refunds[key][txn.Date.Format("2006-01-02")] = amount
			}
		}
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
		refund := refundAfter(candidate, refunds[candidate.key])
		if refund >= candidate.minor {
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
		floor := in.Settings.FirstTimeMerchantFloorMinor
		floorAvailable := true
		if floor == 25000 {
			floor, floorAvailable = currencyThreshold(25000, comparisonCurrency(candidate.txn.Amount, in), in.Rates)
		}
		candidate.minor -= refund
		if floorAvailable && len(merchantHistory) == 0 && candidate.key != "" && candidate.minor >= floor {
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
			comparisonCode := comparisonCurrency(candidate.txn.Amount, in)
			floor, floorAvailable := currencyThreshold(500, comparisonCode, in.Rates)
			tolerance := mulRatio(median, 20, 100)
			if floorAvailable {
				tolerance = maxInt64(tolerance, floor)
			}
			if mad > 0 {
				tolerance = mulRatio(mad, 3, 1)
				if floorAvailable {
					tolerance = maxInt64(tolerance, floor)
				}
			}
			amountFloor, amountFloorAvailable := currencyThreshold(spikeFloor, comparisonCode, in.Rates)
			deltaFloor, deltaFloorAvailable := currencyThreshold(spikeDeltaFloor, comparisonCode, in.Rates)
			if amountFloorAvailable && deltaFloorAvailable && candidate.minor >= amountFloor && delta >= deltaFloor && candidate.minor >= mulRatio(median, 3, 1) && delta > tolerance {
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
		if a.key == "" || !withinWindow(a.date, in.Now, baselineMonths) || len(expectationEvidence(a, in, goals, seasonalYears)) > 0 {
			continue
		}
		var related []string
		for j := range charges {
			if j == i {
				continue
			}
			b := charges[j]
			if delta := b.date.Sub(a.date); delta < -72*time.Hour || delta > 72*time.Hour {
				continue
			}
			if !sameComparisonCurrency(a.txn.Amount.Currency, b.txn.Amount.Currency, in) || b.key != a.key || b.minor != a.minor || b.txn.ID == a.txn.ID {
				continue
			}
			if len(expectationEvidence(b, in, goals, seasonalYears)) > 0 {
				continue
			}
			related = append(related, b.txn.ID)
		}
		if len(related) > 0 {
			sort.Strings(related)
			out = append(out, Finding{TxnID: a.txn.ID, Signal: PossibleDoubleCharge, Confidence: ConfidenceHigh, Evidence: Evidence{AmountMinor: a.minor, CurrencyCode: baseCurrency(a.txn.Amount, in), Method: "same_merchant_amount_within_3_days", HistoryCount: 1, Window: "1â€“3 days", RelatedTxnIDs: related}})
		}
	}
	return out
}

func recurringPriceIncrease(candidate charge, history, series []charge, in Input) (Finding, bool) {
	if len(history) < 3 {
		return Finding{}, false
	}
	ordered := append([]charge(nil), history...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].date.Before(ordered[j].date) })
	series = append([]charge(nil), series...)
	sort.Slice(series, func(i, j int) bool { return series[i].date.Before(series[j].date) })

	return recurringPriceIncreaseOrdered(candidate, ordered, series, in)
}

func recurringPriceIncreaseOrdered(candidate charge, ordered, series []charge, in Input) (Finding, bool) {
	if len(ordered) < 3 {
		return Finding{}, false
	}

	// Any stable, cadence-matched three-charge block can establish the prior
	// amount. Prefer the most recent such block that is below this candidate.
	var baseline int64
	var baselineStart int = -1
	for i := len(ordered) - 3; i >= 0; i-- {
		block := ordered[i : i+3]
		values := []int64{block[0].minor, block[1].minor, block[2].minor}
		typical := median(values)
		if !stableAmounts(values, typical, candidate.txn.Amount, in) || candidate.minor-typical < increaseThresholdFor(typical, candidate.txn.Amount, in) {
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
		confirmed := amountNear(candidate.minor, newAmount, increaseThresholdFor(baseline, candidate.txn.Amount, in))
		for _, item := range ordered[baselineStart+3:] {
			if item.minor-baseline < increaseThresholdFor(baseline, candidate.txn.Amount, in) || !amountNear(item.minor, newAmount, increaseThresholdFor(baseline, candidate.txn.Amount, in)) {
				confirmed = false
				break
			}
			related = append(related, item.txn.ID)
		}
		if confirmed && candidate.minor-baseline >= increaseThresholdFor(baseline, candidate.txn.Amount, in) {
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
		if amountNear(next.minor, candidate.minor, increaseThresholdFor(candidate.minor, candidate.txn.Amount, in)) {
			return true
		}
		// The first later merchant charge determines whether this was a
		// one-off; do not skip a return to the old amount for a later match.
		return false
	}
	return false
}

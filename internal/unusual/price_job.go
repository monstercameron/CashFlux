// SPDX-License-Identifier: MIT

package unusual

import (
	"sort"
	"time"
)

// priceJob bounds the previously quadratic search for a cadence-matched prior
// block. Building, sorting and checking each block's gaps are separate steps.
type priceJob struct {
	candidate                             charge
	history, series                       []charge
	in                                    Input
	phase, block, position, baselineStart int
	baseline                              int64
	gaps                                  []int64
	sorter                                *mergeJob[int64]
	center, tolerance                     int64
	confidence                            Confidence
	confirmed                             bool
	newAmount                             int64
	related                               []string
	previous                              time.Time
	confirmation                          string
	finding                               Finding
	found                                 bool
}

func newPriceJob(candidate charge, history, series []charge, in Input) *priceJob {
	return &priceJob{candidate: candidate, history: history, series: series, in: in, block: len(history) - 3, baselineStart: -1}
}

func (p *priceJob) nextBlock() { p.block--; p.phase = 0; p.gaps = nil; p.sorter = nil }

func (p *priceJob) step() bool {
	switch p.phase {
	case 0:
		if p.block < 0 {
			return true
		}
		block := p.history[p.block : p.block+3]
		values := []int64{block[0].minor, block[1].minor, block[2].minor}
		typical := median(values)
		if !stableAmounts(values, typical, p.candidate.txn.Amount, p.in) || p.candidate.minor-typical < increaseThresholdFor(typical, p.candidate.txn.Amount, p.in) {
			p.block--
			return false
		}
		p.baseline = typical
		p.position = p.block + 1
		p.phase = 1
	case 1:
		previous := p.history[p.position-1].date
		next := p.candidate.date
		if p.position < len(p.history) {
			next = p.history[p.position].date
		}
		gap := int64(next.Sub(previous) / (24 * time.Hour))
		if gap < 1 || gap > 400 {
			p.nextBlock()
			return false
		}
		p.gaps = append(p.gaps, gap)
		p.position++
		if p.position > len(p.history) {
			p.sorter = newMergeJob(p.gaps, func(a, b int64) bool { return a < b })
			p.phase = 2
		}
	case 2:
		if !p.sorter.step() {
			return false
		}
		p.gaps = p.sorter.values
		mid := len(p.gaps) / 2
		p.center = p.gaps[mid]
		if len(p.gaps)%2 == 0 {
			low := p.gaps[mid-1]
			p.center = low + (p.center-low)/2
		}
		p.tolerance = maxInt64(3, mulRatio(p.center, 15, 100))
		p.position = 0
		p.phase = 3
	case 3:
		if p.position < len(p.gaps) {
			gap := p.gaps[p.position]
			p.position++
			if gap < p.center-p.tolerance || gap > p.center+p.tolerance {
				p.nextBlock()
			}
			return false
		}
		p.baselineStart = p.block
		p.confidence = ConfidenceMedium
		p.position = p.block + 3
		if p.position < len(p.history) {
			p.newAmount = p.history[p.position].minor
			p.confirmed = amountNear(p.candidate.minor, p.newAmount, increaseThresholdFor(p.baseline, p.candidate.txn.Amount, p.in))
		}
		p.phase = 4
	case 4:
		if p.position < len(p.history) {
			item := p.history[p.position]
			p.position++
			if item.minor-p.baseline < increaseThresholdFor(p.baseline, p.candidate.txn.Amount, p.in) || !amountNear(item.minor, p.newAmount, increaseThresholdFor(p.baseline, p.candidate.txn.Amount, p.in)) {
				p.confirmed = false
				p.position = len(p.history)
				return false
			}
			p.related = append(p.related, item.txn.ID)
			return false
		}
		if p.confirmed && p.candidate.minor-p.baseline >= increaseThresholdFor(p.baseline, p.candidate.txn.Amount, p.in) {
			p.confidence = ConfidenceHigh
		}
		if p.confidence == ConfidenceMedium {
			p.related = append(p.related, p.history[len(p.history)-1].txn.ID)
			before := sort.Search(len(p.series), func(i int) bool { return !p.series[i].date.Before(p.candidate.date) })
			if before > 0 && p.series[before-1].date.After(time.Time{}) {
				p.previous = p.series[before-1].date
			}
			p.position = sort.Search(len(p.series), func(i int) bool { return p.series[i].date.After(p.candidate.date) })
			p.phase = 5
		} else {
			p.phase = 6
		}
	case 5:
		if p.previous.IsZero() || p.position == len(p.series) {
			p.phase = 6
			return false
		}
		next := p.series[p.position]
		p.position++
		if !sameComparisonCurrency(p.candidate.txn.Amount.Currency, next.txn.Amount.Currency, p.in) {
			return false
		}
		if cadenceGapMatched(p.previous, p.candidate.date, next.date) && amountNear(next.minor, p.candidate.minor, increaseThresholdFor(p.candidate.minor, p.candidate.txn.Amount, p.in)) {
			p.confirmation = "confirmed by the next charge"
		}
		p.phase = 6
	case 6:
		p.finding = Finding{TxnID: p.candidate.txn.ID, Signal: PriceIncrease, Confidence: p.confidence, Evidence: Evidence{
			AmountMinor: p.candidate.minor, TypicalMinor: p.baseline, DeltaMinor: p.candidate.minor - p.baseline, Method: "established_recurring_series",
			CurrencyCode: baseCurrency(p.candidate.txn.Amount, p.in), BaselineSource: BaselineMerchant, HistoryCount: len(p.history),
			Window: "prior cadence-matched charges", RelatedTxnIDs: p.related, Confirmation: p.confirmation}}
		p.found = true
		return true
	}
	return false
}

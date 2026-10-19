// SPDX-License-Identifier: MIT

package unusual

import (
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeebase"
)

// MerchantFinding is the legacy notification rule's leave-one-out median result.
// Unlike the chronological robust detector, its baseline includes all other
// expenses at the merchant. Keeping this separate preserves notification rules.
type MerchantFinding struct {
	Transaction               domain.Transaction
	AmountMinor, TypicalMinor int64
}

type merchantValue struct {
	minor int64
	index int
}
type merchantHistory struct{ values []merchantValue }

// MerchantJob bounds the notification detector's indexing, sorting and evaluation
// work. Each merchant is sorted once; excluding a candidate's rank is O(1).
type MerchantJob struct {
	transactions                  []domain.Transaction
	floor                         int64
	since                         time.Time
	rates                         currency.Rates
	phase, position, rankPosition int
	items                         []MerchantFinding
	keys                          []string
	itemKeys                      []string
	ranks                         []int
	groups                        map[string]*merchantHistory
	sorter                        *mergeJob[merchantValue]
	findings                      []MerchantFinding
	err                           error
}

// NewMerchantJob creates the legacy notification job without scanning input.
func NewMerchantJob(transactions []domain.Transaction, floor int64, since time.Time, rates currency.Rates) *MerchantJob {
	return &MerchantJob{transactions: transactions, floor: floor, since: since, rates: rates, groups: map[string]*merchantHistory{}}
}

// Step performs at most work input, merge, rank or candidate operations.
func (j *MerchantJob) Step(work int) bool {
	if j.floor <= 0 {
		j.phase = 3
		return true
	}
	for ; work > 0 && j.phase < 3; work-- {
		switch j.phase {
		case 0:
			if j.position == len(j.transactions) {
				sort.Strings(j.keys)
				j.phase, j.position = 1, 0
				continue
			}
			t := j.transactions[j.position]
			j.position++
			key := payeebase.Key(t)
			if !t.IsExpense() || key == "" {
				continue
			}
			amount, err := j.rates.Convert(t.Amount.Abs(), j.rates.Base)
			if err != nil {
				j.err = err
				j.phase = 3
				return true
			}
			group := j.groups[key]
			if group == nil {
				group = &merchantHistory{}
				j.groups[key] = group
				j.keys = append(j.keys, key)
			}
			group.values = append(group.values, merchantValue{minor: amount.Amount, index: len(j.items)})
			j.items = append(j.items, MerchantFinding{Transaction: t, AmountMinor: amount.Amount})
			j.itemKeys = append(j.itemKeys, key)
			j.ranks = append(j.ranks, 0)
		case 1:
			if j.position == len(j.keys) {
				j.phase, j.position = 2, 0
				continue
			}
			group := j.groups[j.keys[j.position]]
			if j.sorter == nil {
				j.sorter = newMergeJob(group.values, func(a, b merchantValue) bool { return a.minor < b.minor })
			}
			if !j.sorter.step() {
				continue
			}
			group.values = j.sorter.values
			if j.rankPosition < len(group.values) {
				value := group.values[j.rankPosition]
				j.ranks[value.index] = j.rankPosition
				j.rankPosition++
				continue
			}
			j.sorter = nil
			j.rankPosition = 0
			j.position++
		case 2:
			if j.position == len(j.items) {
				j.phase = 3
				continue
			}
			i := j.position
			j.position++
			item := j.items[i]
			group := j.groups[j.itemKeys[i]].values
			if item.Transaction.Date.Before(j.since) || item.AmountMinor < j.floor || len(group)-1 < payeebase.MinHistory {
				continue
			}
			n := len(group) - 1
			at := func(rank int) int64 {
				if rank >= j.ranks[i] {
					rank++
				}
				return group[rank].minor
			}
			typical := at(n / 2)
			if n%2 == 0 {
				low := at(n/2 - 1)
				typical = low + (typical-low)/2
			}
			if typical <= 0 || typical > math.MaxInt64/3 || item.AmountMinor < 3*typical {
				continue
			}
			item.TypicalMinor = typical
			j.findings = append(j.findings, item)
		}
	}
	return j.phase == 3
}

// Findings returns complete results only after Step completes successfully.
func (j *MerchantJob) Findings() []MerchantFinding {
	if j.phase != 3 || j.err != nil {
		return nil
	}
	return j.findings
}

// Err reports a currency-conversion failure; failed jobs publish no findings.
func (j *MerchantJob) Err() error { return j.err }

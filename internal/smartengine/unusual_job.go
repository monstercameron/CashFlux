// SPDX-License-Identifier: MIT

package smartengine

import (
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/debtpay"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/unusual"
)

// UnusualJob cooperatively performs the complete Smart Unusual adapter,
// including debt-payment exclusion and evidence formatting. It consumes an
// immutable Input and returns the same raw insights as the registered engine.
type UnusualJob struct {
	in              Input
	phase, position int
	debt            debtpay.Dataset
	counterparts    map[debtCounterpartKey]map[int64][]debtpay.Transaction
	transactions    []domain.Transaction
	byID            map[string]domain.Transaction
	detector        *unusual.Job
	findings        []unusual.Finding
	insights        []smart.Insight
	expectedTxnIDs  map[string][]string
}

type debtCounterpartKey struct {
	currency string
	amount   int64
}

// NewUnusualJob prepares a job without scanning the ledger.
func NewUnusualJob(in Input) *UnusualJob {
	return &UnusualJob{in: in, byID: map[string]domain.Transaction{}}
}

// Step performs at most work preparation/adapter operations or detector steps.
// Callers should yield to the browser between calls and cancel stale revisions.
func (j *UnusualJob) Step(work int) bool {
	for ; work > 0 && j.phase < 5; work-- {
		switch j.phase {
		case 0:
			if j.position == len(j.in.Accounts) {
				j.phase, j.position = 1, 0
				continue
			}
			a := j.in.Accounts[j.position]
			j.position++
			j.debt.Accounts = append(j.debt.Accounts, debtpay.Account{ID: a.ID, Name: a.Name, Lender: a.Institution, Currency: a.Currency, Liability: a.Class == domain.ClassLiability})
		case 1:
			if j.position == len(j.in.Transactions) {
				j.phase, j.position = 2, 0
				continue
			}
			t := j.in.Transactions[j.position]
			j.position++
			debtTxn := debtTransaction(t)
			j.debt.Transactions = append(j.debt.Transactions, debtTxn)
			j.indexDebtCounterpart(debtTxn)
			j.byID[t.ID] = t
		case 2:
			if j.position == len(j.in.Transactions) {
				j.detector = unusual.NewJob(unusual.Input{Transactions: j.transactions, Categories: j.in.Categories, Accounts: j.in.Accounts, Recurring: j.in.Recurring, Goals: j.in.Goals, Budgets: j.in.Budgets, Now: j.in.Now, Rates: j.in.Rates, ExpectedTxnIDs: j.expectedTxnIDs})
				j.phase, j.position = 3, 0
				continue
			}
			t := j.in.Transactions[j.position]
			j.position++
			if t.Amount.IsNegative() {
				candidate := debtTransaction(t)
				if t.Amount.Amount != math.MinInt64 {
					candidateData := debtpay.Dataset{Accounts: j.debt.Accounts, Transactions: j.counterpartCandidates(candidate)}
					if debtpay.Classify(candidate, candidateData).Confidence != debtpay.ConfidenceNone {
						continue
					}
				}
			}
			j.transactions = append(j.transactions, t)
		case 3:
			if j.detector.Step(work + 1) {
				j.findings = j.detector.Findings()
				j.phase = 4
			}
			return false
		case 4:
			if j.position == len(j.findings) {
				j.phase = 5
				continue
			}
			f := j.findings[j.position]
			j.position++
			if insight, ok := adaptUnusualFinding(f, j.byID, j.in); ok {
				j.insights = append(j.insights, insight)
			}
		}
	}
	return j.phase == 5
}

func (j *UnusualJob) indexDebtCounterpart(txn debtpay.Transaction) {
	if txn.AmountMinor <= 0 || txn.Date.IsZero() {
		return
	}
	if j.counterparts == nil {
		j.counterparts = make(map[debtCounterpartKey]map[int64][]debtpay.Transaction)
	}
	key := debtCounterpartKey{currency: txn.Currency, amount: txn.AmountMinor}
	if j.counterparts[key] == nil {
		j.counterparts[key] = make(map[int64][]debtpay.Transaction)
	}
	day := dateutil.WallClockUTC(txn.Date).Unix() / int64((24 * time.Hour).Seconds())
	j.counterparts[key][day] = append(j.counterparts[key][day], txn)
}

func (j *UnusualJob) counterpartCandidates(txn debtpay.Transaction) []debtpay.Transaction {
	if txn.AmountMinor >= 0 || txn.AmountMinor == math.MinInt64 || txn.Date.IsZero() {
		return nil
	}
	day := dateutil.WallClockUTC(txn.Date).Unix() / int64((24 * time.Hour).Seconds())
	rows := j.counterparts[debtCounterpartKey{currency: txn.Currency, amount: -txn.AmountMinor}]
	var out []debtpay.Transaction
	for offset := int64(-7); offset <= 7; offset++ {
		out = append(out, rows[day+offset]...)
	}
	return out
}

// Insights returns completed raw insights; nil while work remains. Callers still
// apply the household's normal mute, dismissal and page filters before display.
func (j *UnusualJob) Insights() []smart.Insight {
	if j.phase != 5 {
		return nil
	}
	return j.insights
}

// ActiveInsights applies the same dismissal, mute, ordering and translation
// policy as RunPage. It lets cooperative UI callers preserve the engine's exact
// publication contract without duplicating presentation or evidence logic.
func (j *UnusualJob) ActiveInsights(settings smart.Settings) []smart.Insight {
	if j.phase != 5 {
		return nil
	}
	out := settings.Active(j.insights)
	smart.SortInsights(out)
	return localize(out)
}

// SPDX-License-Identifier: MIT

// Package importsafe holds the pure pre-import dependability checks (#57):
// the balance-impact preview an import shows before it commits, the
// implausible-jump warning, the "why matched" explanation for duplicate rows,
// and detection of incoming rows that look like the second leg of a transfer
// already in the ledger. All functions are deterministic and side-effect free
// so the previews can never disagree with what the import then does.
package importsafe

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Impact sums the incoming rows destined for accountID (rows with a blank
// AccountID fall back to it, mirroring the importer) and returns the net
// signed change plus the resulting balance.
func Impact(currentMinor int64, incoming []domain.Transaction, accountID, accountCurrency string) (netMinor, afterMinor int64, err error) {
	for _, t := range incoming {
		acct := t.AccountID
		if acct == "" {
			acct = accountID
		}
		if acct != accountID {
			continue
		}
		if !strings.EqualFold(t.Amount.Currency, accountCurrency) {
			return 0, 0, fmt.Errorf("importsafe: row currency %s does not match account currency %s", t.Amount.Currency, accountCurrency)
		}
		netMinor, err = money.Add(netMinor, t.Amount.Amount)
		if err != nil {
			return 0, 0, fmt.Errorf("importsafe: sum import impact: %w", err)
		}
	}
	afterMinor, err = money.Add(currentMinor, netMinor)
	if err != nil {
		return 0, 0, fmt.Errorf("importsafe: add import impact to balance: %w", err)
	}
	return netMinor, afterMinor, nil
}

// jumpFloorMinor is the absolute net change below which an import is never
// flagged: $10,000 in minor units. Small imports are routine whatever their
// relative size (a first import into an empty account must not warn).
const jumpFloorMinor = 1_000_000

// JumpWarning reports whether an import's net change looks implausible for
// the account: at least the absolute floor AND large relative to the current
// balance (more than 3× its magnitude). Both conditions keep the warning
// rare and explainable — a $12k statement into a $40k account is normal; a
// $12k swing on a $900 account deserves a second look before committing.
func JumpWarning(currentMinor, netMinor int64) bool {
	abs := magnitude(netMinor)
	if abs < jumpFloorMinor {
		return false
	}
	cur := magnitude(currentMinor)
	return cur == 0 || abs/3 > cur || (abs/3 == cur && abs%3 > 0)
}

func magnitude(value int64) uint64 {
	if value >= 0 {
		return uint64(value)
	}
	// #nosec G115 -- for value < 0, ^value is in [0, MaxInt64], even at MinInt64.
	return uint64(^value) + 1
}

// WhyDup explains one incoming row that would be skipped as a duplicate: the
// fields it collides on (the dedupe signature — same calendar day, same signed
// amount and currency, same normalized description, same account).
type WhyDup struct {
	Date        string // YYYY-MM-DD
	Desc        string
	AmountMinor int64
	Currency    string
	// InBatch is retained for compatibility. The importer counts existing
	// occurrences only, so additional same-file repetitions are new and this is false.
	InBatch bool
}

// Duplicates returns the why-matched detail for every incoming row that the
// importer would skip, in input order. It mirrors dedupe.CountIncomingDuplicates
// exactly (per-account signature vs the ledger, then vs earlier batch rows), so
// len(Duplicates(...)) always equals the preview count.
func Duplicates(incoming, existing []domain.Transaction, accountID string) []WhyDup {
	remaining := make(map[string]int, len(existing))
	for _, t := range existing {
		remaining[dedupe.Key(t)]++
	}
	var out []WhyDup
	for _, t := range incoming {
		acct := t.AccountID
		if acct == "" {
			acct = accountID
		}
		// Same key the importer and the duplicates screen use, with the row's
		// account resolved through the caller's fallback first (C688).
		scoped := t
		scoped.AccountID = acct
		sig := dedupe.Key(scoped)
		if dedupe.ConsumeExisting(remaining, sig) {
			out = append(out, WhyDup{Date: t.Date.Format("2006-01-02"), Desc: strings.TrimSpace(t.Desc),
				AmountMinor: t.Amount.Amount, Currency: t.Amount.Currency})
		}
	}
	return out
}

// Pair is one incoming row that mirrors an existing transaction in a DIFFERENT
// account — the classic missed transfer: the card statement's "PAYMENT THANK
// YOU" arriving after the checking export already recorded the outflow.
type Pair struct {
	IncomingDesc string
	IncomingDate string // YYYY-MM-DD
	AmountMinor  int64
	OtherAccount string // the existing transaction's account ID
	OtherDesc    string
}

// TransferPairs finds incoming rows whose signed amount is exactly opposite an
// existing non-transfer transaction in another account within windowDays. Each
// existing transaction is claimed at most once; rows and matches already
// marked as transfers are skipped. Results are in incoming order.
func TransferPairs(incoming, existing []domain.Transaction, accountID string, windowDays int) []Pair {
	byOpposite := make(map[string]map[string][]*domain.Transaction, len(existing))
	for i := range existing {
		ex := &existing[i]
		if ex.IsTransfer() || ex.Amount.Amount == 0 {
			continue
		}
		opposite, err := money.Neg(ex.Amount.Amount)
		if err != nil {
			continue
		}
		key := pairAmountKey(opposite, ex.Amount.Currency)
		if byOpposite[key] == nil {
			byOpposite[key] = make(map[string][]*domain.Transaction)
		}
		byOpposite[key][ex.AccountID] = append(byOpposite[key][ex.AccountID], ex)
	}
	sets := make(map[string]map[string]*candidateSet, len(byOpposite))
	accountsByKey := make(map[string][]string, len(byOpposite))
	for key, accounts := range byOpposite {
		sets[key] = make(map[string]*candidateSet, len(accounts))
		for acct, candidates := range accounts {
			sort.SliceStable(candidates, func(i, j int) bool {
				if candidates[i].Date.Equal(candidates[j].Date) {
					return candidates[i].ID < candidates[j].ID
				}
				return candidates[i].Date.Before(candidates[j].Date)
			})
			sets[key][acct] = newCandidateSet(candidates)
			accountsByKey[key] = append(accountsByKey[key], acct)
		}
		sort.Strings(accountsByKey[key])
	}
	if windowDays < 0 {
		return nil
	}
	window := time.Duration(windowDays) * 24 * time.Hour
	var out []Pair
	for _, in := range incoming {
		if in.IsTransfer() || in.Amount.Amount == 0 {
			continue
		}
		acct := in.AccountID
		if acct == "" {
			acct = accountID
		}
		key := pairAmountKey(in.Amount.Amount, in.Amount.Currency)
		lower, upper := in.Date.Add(-window), in.Date.Add(window)
		var match *domain.Transaction
		for _, otherAccount := range accountsByKey[key] {
			if otherAccount == acct {
				continue
			}
			set := sets[key][otherAccount]
			idx := sort.Search(len(set.transactions), func(i int) bool {
				return !set.transactions[i].Date.Before(lower)
			})
			idx = set.find(idx)
			if idx < len(set.transactions) && !set.transactions[idx].Date.After(upper) {
				match = set.transactions[idx]
				set.claim(idx)
				break
			}
		}
		if match != nil {
			out = append(out, Pair{
				IncomingDesc: strings.TrimSpace(in.Desc), IncomingDate: in.Date.Format("2006-01-02"),
				AmountMinor: in.Amount.Amount, OtherAccount: match.AccountID, OtherDesc: strings.TrimSpace(match.Desc),
			})
		}
	}
	return out
}

type candidateSet struct {
	transactions []*domain.Transaction
	parent       []int
}

func newCandidateSet(transactions []*domain.Transaction) *candidateSet {
	set := &candidateSet{transactions: transactions, parent: make([]int, len(transactions)+1)}
	for i := range set.parent {
		set.parent[i] = i
	}
	return set
}

func (s *candidateSet) find(index int) int {
	if index >= len(s.transactions) {
		return len(s.transactions)
	}
	if s.parent[index] != index {
		s.parent[index] = s.find(s.parent[index])
	}
	return s.parent[index]
}

func (s *candidateSet) claim(index int) {
	s.parent[index] = s.find(index + 1)
}

func pairAmountKey(amount int64, currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency)) + "|" + fmt.Sprint(amount)
}

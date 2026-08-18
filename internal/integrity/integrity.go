// SPDX-License-Identifier: MIT

// Package integrity is the local data-health checker (#53): pure,
// deterministic cross-checks over the dataset that surface quiet
// inconsistencies before they corrupt totals — an orphaned transfer leg, a
// split that no longer sums to its transaction, a transaction denominated in
// a currency its account doesn't use, a liability that flipped sign, cleared
// history drifting from a recorded reconciliation, and impossible budget /
// goal arithmetic. Everything is computed from the same domain values the
// reports read, so a clean bill of health means the numbers agree with
// themselves.
package integrity

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/transferpair"
)

// Check identifies which cross-check produced a finding.
type Check string

const (
	// CheckTransferOrphan: a transfer leg with no counterpart in the other account.
	CheckTransferOrphan Check = "transfer-orphan"
	// CheckTransferLegsDisagree: a transfer whose two legs exist but do not cancel,
	// so the pair moved money into existence or out of it. Distinct from an orphan
	// on purpose — the far side IS there, and telling someone to go looking for a
	// missing transaction that is sitting in front of them wastes the trip.
	CheckTransferLegsDisagree Check = "transfer-legs-disagree"
	// CheckSplitSum: a split transaction whose lines don't sum to its amount.
	CheckSplitSum Check = "split-sum"
	// CheckCurrencyMismatch: a transaction denominated differently from its account.
	CheckCurrencyMismatch Check = "currency-mismatch"
	// CheckOrphanAccount: a transaction pointing at an account that doesn't exist.
	CheckOrphanAccount Check = "orphan-account"
	// CheckLiabilitySign: a liability account whose balance is positive (credit).
	CheckLiabilitySign Check = "liability-sign"
	// CheckReconcileDrift: cleared history no longer matches the recorded
	// statement balance of the newest (unforced) reconciliation.
	CheckReconcileDrift Check = "reconcile-drift"
	// CheckBudgetLimit: a budget with a zero or negative limit.
	CheckBudgetLimit Check = "budget-limit"
	// CheckGoalArithmetic: a goal with a non-positive target or negative progress.
	CheckGoalArithmetic Check = "goal-arithmetic"
	// CheckGoalOverfunded: a goal whose progress exceeds its target.
	CheckGoalOverfunded Check = "goal-overfunded"
)

// Severity ranks a finding. Warnings distort money math; infos are worth a
// look but may be intentional.
type Severity string

const (
	SevWarning Severity = "warning"
	SevInfo    Severity = "info"
)

// Finding is one detected inconsistency, with enough typed context for the UI
// to compose a plain-English line and a drill-through. ID is stable for a
// given dataset state (check + entity), so lists diff cleanly across runs.
type Finding struct {
	ID         string
	Check      Check
	Severity   Severity
	EntityType string // "transaction" | "account" | "budget" | "goal"
	EntityID   string
	Name       string // display name: txn description, account/budget/goal name
	Currency   string
	// AmountMinor / OtherMinor are check-specific figures (e.g. the split sum
	// vs the transaction amount; the expected vs recorded statement balance).
	AmountMinor int64
	OtherMinor  int64
}

// Input carries the dataset slices the checks read.
type Input struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Budgets      []domain.Budget
	Goals        []domain.Goal
}

// Run executes every check and returns findings in a deterministic order
// (check, then entity id). A healthy dataset returns an empty slice.
func Run(in Input) []Finding {
	var out []Finding
	acctByID := make(map[string]domain.Account, len(in.Accounts))
	for _, a := range in.Accounts {
		acctByID[a.ID] = a
	}

	out = append(out, checkTransfers(in.Transactions, in.Accounts)...)
	out = append(out, checkTransactions(in.Transactions, acctByID)...)
	out = append(out, checkAccounts(in.Accounts, in.Transactions)...)
	out = append(out, checkBudgets(in.Budgets)...)
	out = append(out, checkGoals(in.Goals)...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Check != out[j].Check {
			return out[i].Check < out[j].Check
		}
		return out[i].EntityID < out[j].EntityID
	})
	return out
}

// checkTransfers pairs transfer legs: a leg in account A pointing at account B
// must have a counterpart in B pointing back at A with the opposite amount.
// Unpaired legs are orphans — the classic half-deleted transfer that makes one
// account lie by exactly the moved amount.
func checkTransfers(txns []domain.Transaction, accounts []domain.Account) []Finding {
	// C686: this used to greedily match legs on an exactly opposite amount with no
	// date check, while internal/contradict matched on the same calendar day with
	// no amount check. The two disagreed about the same rows on two different
	// screens, and this one had a second problem of its own: it required the same
	// CURRENCY and an exactly opposite AMOUNT, so a correct cross-currency transfer
	// and a correct payment into a positive-owed card — whose legs are both
	// negative — were each reported as two orphans. The check meant to catch the
	// orphan bug fired hardest on the healthy version of it.
	//
	// Both screens now ask internal/transferpair, which honours a declared pair
	// before it matches anything.
	var out []Finding
	for _, t := range txns {
		if !t.IsTransfer() {
			continue
		}
		if partner, ok := transferpair.Declared(t, txns); ok {
			// Report each declared pair once, from the leg with the lower id, so a
			// disagreement is one finding rather than two saying the same thing.
			if net, balanced, known := transferpair.NetStated(t, partner, accounts); known && !balanced && t.ID < partner.ID {
				out = append(out, Finding{
					ID: fmt.Sprintf("%s:%s", CheckTransferLegsDisagree, t.ID), Check: CheckTransferLegsDisagree,
					Severity: SevWarning, EntityType: "transaction", EntityID: t.ID,
					Name: t.Desc, Currency: t.Amount.Currency, AmountMinor: net,
				})
			}
			continue
		}
		if transferpair.For(t, txns, accounts).Confidence != transferpair.Unmatched {
			continue
		}
		out = append(out, Finding{
			ID: fmt.Sprintf("%s:%s", CheckTransferOrphan, t.ID), Check: CheckTransferOrphan,
			Severity: SevWarning, EntityType: "transaction", EntityID: t.ID,
			Name: t.Desc, Currency: t.Amount.Currency, AmountMinor: t.Amount.Amount,
		})
	}
	return out
}

// checkTransactions validates per-row arithmetic and referential integrity.
func checkTransactions(txns []domain.Transaction, acctByID map[string]domain.Account) []Finding {
	var out []Finding
	for _, t := range txns {
		if t.HasSplits() && !t.SplitsReconcile() {
			var sum int64
			for _, s := range t.Splits {
				sum += s.Amount.Amount
			}
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckSplitSum, t.ID), Check: CheckSplitSum,
				Severity: SevWarning, EntityType: "transaction", EntityID: t.ID,
				Name: t.Desc, Currency: t.Amount.Currency,
				AmountMinor: sum, OtherMinor: t.Amount.Amount,
			})
		}
		acc, known := acctByID[t.AccountID]
		if !known {
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckOrphanAccount, t.ID), Check: CheckOrphanAccount,
				Severity: SevWarning, EntityType: "transaction", EntityID: t.ID,
				Name: t.Desc, Currency: t.Amount.Currency, AmountMinor: t.Amount.Amount,
			})
			continue
		}
		if acc.Currency != "" && t.Amount.Currency != "" && t.Amount.Currency != acc.Currency {
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckCurrencyMismatch, t.ID), Check: CheckCurrencyMismatch,
				Severity: SevWarning, EntityType: "transaction", EntityID: t.ID,
				Name: t.Desc, Currency: t.Amount.Currency, AmountMinor: t.Amount.Amount,
			})
		}
	}
	return out
}

// checkAccounts validates liability sign conventions and reconciliation drift.
func checkAccounts(accounts []domain.Account, txns []domain.Transaction) []Finding {
	var out []Finding
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err == nil && a.Class == domain.ClassLiability && bal.Amount > 0 {
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckLiabilitySign, a.ID), Check: CheckLiabilitySign,
				Severity: SevInfo, EntityType: "account", EntityID: a.ID,
				Name: a.Name, Currency: a.Currency, AmountMinor: bal.Amount,
			})
		}
		// Reconciliation drift: the newest UNFORCED reconciliation vouched that
		// cleared history through its date summed to the statement balance. If
		// cleared rows at or before that date were later edited, the recorded
		// statement no longer holds — the strongest balances-vs-transactions
		// cross-check available locally. Forced events recorded a known gap, so
		// they are exempt.
		if n := len(a.Reconciliations); n > 0 {
			r := a.Reconciliations[n-1]
			if !r.Forced {
				through := dayKey(r.Through())
				var cleared int64
				for _, t := range txns {
					if t.AccountID == a.ID && t.Cleared && dayKey(t.Date) <= through {
						cleared += t.Amount.Amount
					}
				}
				expect := a.OpeningBalance.Amount + cleared
				if expect != r.StatementBalance.Amount {
					out = append(out, Finding{
						ID: fmt.Sprintf("%s:%s", CheckReconcileDrift, a.ID), Check: CheckReconcileDrift,
						Severity: SevWarning, EntityType: "account", EntityID: a.ID,
						Name: a.Name, Currency: a.Currency,
						AmountMinor: expect, OtherMinor: r.StatementBalance.Amount,
					})
				}
			}
		}
	}
	return out
}

// dayKey collapses a timestamp to its calendar day (in its own location) so a
// reconciliation's date-only "through" boundary never excludes a same-day
// transaction that happens to carry a time of day.
func dayKey(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// checkBudgets flags limits that make the arithmetic meaningless.
func checkBudgets(budgets []domain.Budget) []Finding {
	var out []Finding
	for _, b := range budgets {
		if b.Limit.Amount <= 0 {
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckBudgetLimit, b.ID), Check: CheckBudgetLimit,
				Severity: SevWarning, EntityType: "budget", EntityID: b.ID,
				Name: b.Name, Currency: b.Limit.Currency, AmountMinor: b.Limit.Amount,
			})
		}
	}
	return out
}

// checkGoals flags impossible or surprising goal arithmetic. Only financial
// goals carry money targets — checklist/milestone/habit kinds measure progress
// by tasks or check-ins and legitimately have zero amounts.
func checkGoals(goals []domain.Goal) []Finding {
	var out []Finding
	for _, g := range goals {
		if g.Kind != "" && g.Kind != domain.GoalKindFinancial {
			continue
		}
		switch {
		case g.TargetAmount.Amount <= 0 || g.CurrentAmount.Amount < 0:
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckGoalArithmetic, g.ID), Check: CheckGoalArithmetic,
				Severity: SevWarning, EntityType: "goal", EntityID: g.ID,
				Name: g.Name, Currency: g.TargetAmount.Currency,
				AmountMinor: g.CurrentAmount.Amount, OtherMinor: g.TargetAmount.Amount,
			})
		case g.CurrentAmount.Amount > g.TargetAmount.Amount:
			out = append(out, Finding{
				ID: fmt.Sprintf("%s:%s", CheckGoalOverfunded, g.ID), Check: CheckGoalOverfunded,
				Severity: SevInfo, EntityType: "goal", EntityID: g.ID,
				Name: g.Name, Currency: g.TargetAmount.Currency,
				AmountMinor: g.CurrentAmount.Amount, OtherMinor: g.TargetAmount.Amount,
			})
		}
	}
	return out
}

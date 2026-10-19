package debtpay

import (
	"sort"
	"strings"
	"time"
)

const (
	strongScore       = 80
	mediumScore       = 50
	counterpartWindow = 7 * 24 * time.Hour
)

type scoredCandidate struct {
	candidate Candidate
	score     int
}

// Classify returns the unique best debt-payment candidate. Weak or tied
// evidence returns a no-confidence candidate, never an arbitrary winner.
func Classify(txn Transaction, data Dataset) Candidate {
	noCandidate := Candidate{TxnID: txn.ID, Confidence: ConfidenceNone, MemberID: txn.MemberID}
	if txn.ID == "" || txn.AmountMinor >= 0 || txn.Currency == "" || txn.TransferGroupID != "" || txn.TransferAccountID != "" {
		return noCandidate
	}

	var source Account
	sourceFound := false
	for _, account := range data.Accounts {
		if account.ID == txn.AccountID && account.ScopeID == txn.ScopeID {
			source = account
			sourceFound = true
			break
		}
	}
	if !sourceFound || source.Liability {
		return noCandidate
	}
	var candidates []scoredCandidate
	for _, account := range data.Accounts {
		if !account.Liability || account.ID == "" || account.ScopeID != txn.ScopeID || account.ID == txn.AccountID || account.Currency == "" || account.Currency != txn.Currency {
			continue
		}
		var evidence []Evidence
		counterpartID := findCounterpart(txn, account, data.Transactions)
		if counterpartID != "" {
			evidence = append(evidence, Evidence{Kind: EvidenceLiabilityCounterpart, Description: "Matching credit on the debt account", Score: 100})
		}
		if txn.BillAccountID == account.ID {
			evidence = append(evidence, Evidence{Kind: EvidenceBillAccount, Description: "Transaction already names this bill account", Score: 90})
		}
		if matchingPayeeRule(txn, account, data.PayeeRules) {
			evidence = append(evidence, Evidence{Kind: EvidencePayeeRule, Description: "Previously confirmed payee rule", Score: 90})
		}
		if descriptorMatches(txn, account) {
			if recurringMatches(txn, account, data.Recurring) {
				evidence = append(evidence, Evidence{Kind: EvidenceDescriptorPattern, Description: "Payee or description names the lender", Score: 25})
				evidence = append(evidence, Evidence{Kind: EvidenceRecurringPattern, Description: "Amount and date match the debt payment schedule", Score: 35})
			}
		}
		if len(evidence) == 0 {
			continue
		}
		score := 0
		for _, item := range evidence {
			score += item.Score
		}
		sort.Slice(evidence, func(i, j int) bool { return evidence[i].Kind < evidence[j].Kind })
		confidence := ConfidenceMedium
		if score >= strongScore {
			confidence = ConfidenceStrong
		} else if score < mediumScore {
			continue
		}
		candidates = append(candidates, scoredCandidate{
			candidate: Candidate{TxnID: txn.ID, TargetDebtAccountID: account.ID, Evidence: evidence, Confidence: confidence, ExistingCounterpartID: counterpartID, MemberID: txn.MemberID},
			score:     score,
		})
	}
	if len(candidates) == 0 {
		return noCandidate
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].candidate.TargetDebtAccountID < candidates[j].candidate.TargetDebtAccountID
	})
	if len(candidates) > 1 && candidates[0].score == candidates[1].score {
		return noCandidate
	}
	return candidates[0].candidate
}

func findCounterpart(txn Transaction, debt Account, transactions []Transaction) string {
	found := ""
	for _, other := range transactions {
		if other.ID == "" || other.ID == txn.ID || other.AccountID != debt.ID || other.ScopeID != txn.ScopeID || other.AmountMinor <= 0 || other.Currency != txn.Currency {
			continue
		}
		if other.TransferGroupID != "" || other.TransferAccountID != "" || other.BillAccountID != "" {
			continue
		}
		if other.AmountMinor != -txn.AmountMinor || !withinDays(txn.Date, other.Date, counterpartWindow) {
			continue
		}
		// Multiple plausible credits are ambiguous and provide no counterpart signal.
		if found != "" {
			return ""
		}
		found = other.ID
	}
	return found
}

func withinDays(a, b time.Time, window time.Duration) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	delta := a.Sub(b)
	if delta < 0 {
		delta = -delta
	}
	return delta <= window
}

func matchingPayeeRule(txn Transaction, account Account, rules []PayeeRule) bool {
	payee := normalize(txn.Payee)
	if payee == "" {
		return false
	}
	for _, rule := range rules {
		if rule.ScopeID == txn.ScopeID && rule.DebtAccountID == account.ID && normalize(rule.Payee) == payee {
			return true
		}
	}
	return false
}

func descriptorMatches(txn Transaction, account Account) bool {
	descriptor := normalize(txn.Payee + " " + txn.Description)
	for _, hint := range []string{account.Name, account.Lender} {
		hint = normalize(hint)
		if hint != "" && strings.Contains(descriptor, hint) {
			return true
		}
	}
	return false
}

func recurringMatches(txn Transaction, account Account, recurrings []RecurringPayment) bool {
	for _, recurring := range recurrings {
		if recurring.ScopeID != txn.ScopeID || recurring.DebtAccountID != account.ID || recurring.Currency != txn.Currency || recurring.AmountMinor != -txn.AmountMinor {
			continue
		}
		if !withinDays(txn.Date, recurring.NextDue, counterpartWindow) {
			continue
		}
		return true
	}
	return false
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

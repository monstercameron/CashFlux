// SPDX-License-Identifier: MIT

package subscriptions

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// lenderPhrases is the case-insensitive substring list used by the label signal.
// A transaction whose payee or description contains any of these terms is treated
// as a liability payment rather than a real subscription.
var lenderPhrases = []string{
	"payment",
	"min payment",
	"autopay",
	"loan",
	"mortgage",
	"card payment",
	"chase",
	"wells fargo",
	"bank of america",
	"citibank",
	"discover",
	"synchrony",
	"capital one",
	"american express",
	"amex",
	"navy federal",
	"usaa",
}

// IsLiabilityPayment reports whether a detected subscription is actually a
// loan- or credit-card payment masquerading as a recurring charge.
//
// Two independent signals are checked; either alone is sufficient (OR logic):
//
//  1. Account-type signal — if any matched transaction debits from a liability
//     account (credit card, loan, line of credit, mortgage, personal loan), the
//     charge is classified as a liability payment. Account lookup is by
//     Transaction.AccountID against the provided accounts slice.
//
//  2. Label signal — if the subscription's Name, or the Payee or Desc of any
//     matched transaction, contains a lender- or payment-related phrase
//     (case-insensitive), it is classified as a liability payment.
//
// "Matched transactions" are those whose Desc (trimmed, case-insensitive) equals
// the subscription's Name — the same join Detect uses when building the
// subscription.
func IsLiabilityPayment(sub Subscription, txns []domain.Transaction, accounts []domain.Account) bool {
	// Fast path: label signal on the subscription name itself.
	if containsLenderPhrase(sub.Name) {
		return true
	}

	for _, t := range txns {
		if !isMatchedTxn(sub.Name, t) {
			continue
		}

		// Signal 1 — account type.
		if acc, ok := accountByID(accounts, t.AccountID); ok {
			if acc.Type.IsLiability() || acc.Class == domain.ClassLiability {
				return true
			}
		}

		// Signal 2 — label on the individual transaction's Payee or Desc.
		if containsLenderPhrase(t.Payee) || containsLenderPhrase(t.Desc) {
			return true
		}
	}

	return false
}

// accountByID returns the Account with the given id from accounts, and reports
// whether it was found.
func accountByID(accounts []domain.Account, id string) (domain.Account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Account{}, false
}

// isMatchedTxn reports whether t is one of the transactions that Detect would
// have grouped into sub — same join key: trimmed Desc equals sub.Name
// (case-insensitive).
func isMatchedTxn(subName string, t domain.Transaction) bool {
	return strings.EqualFold(strings.TrimSpace(t.Desc), strings.TrimSpace(subName))
}

// containsLenderPhrase reports whether s contains any of the lenderPhrases as a
// case-insensitive substring.
func containsLenderPhrase(s string) bool {
	lower := strings.ToLower(s)
	for _, phrase := range lenderPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

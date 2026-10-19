// SPDX-License-Identifier: MIT

package txnclassify

import (
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PaymentSuggestion identifies an existing expense that appears to be a
// recurring payment to a liability account. It is advice only; callers must not
// change the transaction without the user's confirmation.
type PaymentSuggestion struct {
	TransactionID string
	AccountID     string
	AccountName   string
	Message       string
}

// RecurringLiabilityDestination resolves a recurring payment to a liability
// only when amount, currency and the label identify exactly one liability.
// Account names and payment labels are compared by meaningful words, so a
// generic label such as "Payment" cannot pick a destination by itself.
func RecurringLiabilityDestination(r domain.Recurring, accounts []domain.Account) (domain.Account, bool) {
	if r.ID == "" || !r.Amount.IsNegative() {
		return domain.Account{}, false
	}
	labelWords := meaningfulWords(r.Label)
	var match domain.Account
	for _, account := range accounts {
		if account.Class != domain.ClassLiability || account.Archived || account.MinPayment.Amount == 0 || account.Currency != r.Amount.Currency || account.MinPayment.Abs().Amount != r.Amount.Abs().Amount {
			continue
		}
		if !overlap(labelWords, meaningfulWords(account.Name)) {
			continue
		}
		if match.ID != "" {
			return domain.Account{}, false
		}
		match = account
	}
	return match, match.ID != ""
}

// FindRecurringPaymentSuggestions flags expense rows whose payee matches a
// recurring liability payment. It never rewrites transactions.
func FindRecurringPaymentSuggestions(transactions []domain.Transaction, accounts []domain.Account, recurring []domain.Recurring) []PaymentSuggestion {
	var suggestions []PaymentSuggestion
	for _, txn := range transactions {
		if txn.ID == "" || txn.IsTransfer() || !txn.Amount.IsNegative() {
			continue
		}
		for _, rule := range recurring {
			destination, ok := RecurringLiabilityDestination(rule, accounts)
			if !ok || txn.Amount.Currency != rule.Amount.Currency || txn.Amount.Abs().Amount != rule.Amount.Abs().Amount {
				continue
			}
			if !overlap(meaningfulWords(txn.Payee), meaningfulWords(rule.Label)) {
				continue
			}
			suggestions = append(suggestions, PaymentSuggestion{
				TransactionID: txn.ID,
				AccountID:     destination.ID,
				AccountName:   destination.Name,
				Message:       "Looks like a payment to " + destination.Name + " — record as a transfer?",
			})
			break
		}
	}
	return suggestions
}

func meaningfulWords(value string) map[string]struct{} {
	words := make(map[string]struct{})
	var b strings.Builder
	flush := func() {
		word := strings.ToLower(b.String())
		b.Reset()
		switch word {
		case "", "payment", "pay", "loan", "card", "credit", "debit", "account", "the", "to", "for":
			return
		}
		words[word] = struct{}{}
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return words
}

func overlap(a, b map[string]struct{}) bool {
	for word := range a {
		if _, ok := b[word]; ok {
			return true
		}
	}
	return false
}

// SPDX-License-Identifier: MIT

package txnclassify

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// PaymentSuggestion identifies an existing expense that appears to be a
// recurring payment to a liability account. It is advice only; callers must not
// change the transaction without the user's confirmation.
type PaymentSuggestion struct {
	TransactionID string
	AccountID     string
	AccountName   string
}

// RecurringLiabilityDestination resolves a recurring payment to a liability
// only when amount, currency and the label identify exactly one liability.
// Account names and payment labels are compared by meaningful words, so a
// generic label such as "Payment" cannot pick a destination by itself.
func RecurringLiabilityDestination(r domain.Recurring, accounts []domain.Account) (domain.Account, bool) {
	if r.ID == "" || !r.Amount.IsNegative() {
		return domain.Account{}, false
	}
	paymentAmount, err := money.Abs(r.Amount.Amount)
	if err != nil {
		return domain.Account{}, false
	}
	labelWords := meaningfulWords(r.Label)
	var match domain.Account
	for _, account := range accounts {
		if account.ID == r.AccountID || account.Class != domain.ClassLiability || account.Archived || account.MinPayment.Amount == 0 || account.Currency != r.Amount.Currency {
			continue
		}
		minimum, err := money.Abs(account.MinPayment.Amount)
		if err != nil || minimum != paymentAmount {
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
	transfers := recurringTransferIndex(transactions)
	rules := make([]recurringPaymentRule, 0, len(recurring))
	for _, rule := range recurring {
		destination, ok := RecurringLiabilityDestination(rule, accounts)
		if !ok {
			continue
		}
		amount, err := money.Abs(rule.Amount.Amount)
		if err != nil {
			continue
		}
		rules = append(rules, recurringPaymentRule{
			labelWords:  meaningfulWords(rule.Label),
			destination: destination,
			amount:      amount,
			currency:    rule.Amount.Currency,
		})
	}

	var suggestions []PaymentSuggestion
	for _, txn := range transactions {
		if txn.ID == "" || txn.IsTransfer() || !txn.Amount.IsNegative() {
			continue
		}
		amount, err := money.Abs(txn.Amount.Amount)
		if err != nil {
			continue
		}
		payeeWords := meaningfulWords(txn.Payee)
		for _, rule := range rules {
			if txn.Amount.Currency != rule.currency || amount != rule.amount {
				continue
			}
			if !overlap(payeeWords, rule.labelWords) {
				continue
			}
			if hasNearbyRecurringTransfer(transfers, rule.destination.ID, txn.Amount.Currency, amount, txn.Date) {
				continue
			}
			suggestions = append(suggestions, PaymentSuggestion{
				TransactionID: txn.ID,
				AccountID:     rule.destination.ID,
				AccountName:   rule.destination.Name,
			})
			break
		}
	}
	return suggestions
}

type recurringPaymentRule struct {
	labelWords  map[string]struct{}
	destination domain.Account
	amount      int64
	currency    string
}

type recurringTransferKey struct {
	destination string
	currency    string
	amount      int64
}

func recurringTransferIndex(transactions []domain.Transaction) map[recurringTransferKey][]time.Time {
	index := make(map[recurringTransferKey][]time.Time)
	for _, txn := range transactions {
		if !txn.IsTransfer() || txn.TransferAccountID == "" {
			continue
		}
		amount, err := money.Abs(txn.Amount.Amount)
		if err != nil {
			continue
		}
		key := recurringTransferKey{destination: txn.TransferAccountID, currency: txn.Amount.Currency, amount: amount}
		index[key] = append(index[key], txn.Date)
	}
	for key := range index {
		sort.Slice(index[key], func(i, j int) bool { return index[key][i].Before(index[key][j]) })
	}
	return index
}

func hasNearbyRecurringTransfer(index map[recurringTransferKey][]time.Time, destination, currency string, amount int64, date time.Time) bool {
	dates := index[recurringTransferKey{destination: destination, currency: currency, amount: amount}]
	if len(dates) == 0 {
		return false
	}
	start := date.Add(-PairWindow)
	position := sort.Search(len(dates), func(i int) bool { return !dates[i].Before(start) })
	return position < len(dates) && !dates[position].After(date.Add(PairWindow))
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

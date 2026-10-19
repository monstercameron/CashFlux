// SPDX-License-Identifier: MIT

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TxnFormOption is a stable value and label for an option shared by transaction
// Add and Edit forms.
type TxnFormOption struct {
	Value string
	Label string
}

// TxnFormAccountOptions returns active accounts, plus a selected archived
// account when necessary, sorted by their visible names. The account type is
// deliberately omitted so Add and Edit show the same concise option text.
func TxnFormAccountOptions(accounts []domain.Account, selectedID string) []TxnFormOption {
	options := make([]TxnFormOption, 0, len(accounts))
	for _, account := range accounts {
		if account.Archived && account.ID != selectedID {
			continue
		}
		options = append(options, TxnFormOption{Value: account.ID, Label: account.Name})
	}
	sort.SliceStable(options, func(i, j int) bool {
		left, right := strings.ToLower(strings.TrimSpace(options[i].Label)), strings.ToLower(strings.TrimSpace(options[j].Label))
		if left != right {
			return left < right
		}
		return options[i].Value < options[j].Value
	})
	return options
}

// TxnEditHeading returns the row description and, when it adds information, its
// payee subtitle for the Edit dialog header.
func TxnEditHeading(txn domain.Transaction) (title, subtitle string) {
	title = strings.TrimSpace(txn.Desc)
	payee := strings.TrimSpace(txn.Payee)
	if title == "" {
		title = payee
	}
	if payee != "" && payee != title {
		subtitle = payee
	}
	return title, subtitle
}

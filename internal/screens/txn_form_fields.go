// SPDX-License-Identifier: MIT

package screens

import (
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TxnFormOption is a stable value and label for an option shared by transaction
// Add and Edit forms.
type TxnFormOption struct {
	Value string
	Label string
}

// TxnFormAccountOptions returns active accounts, plus a selected archived
// account when necessary, in the account picker order. When same-named accounts
// have different types, their labels include the type so users can distinguish
// them.
func TxnFormAccountOptions(accounts []domain.Account, selectedID string, retainedArchivedIDs ...string) []TxnFormOption {
	ordered := acctsort.ForPicker(accounts)
	visible := make([]domain.Account, 0, len(ordered))
	nameTypes := make(map[string]map[domain.AccountType]struct{}, len(ordered))
	for _, account := range ordered {
		retainArchived := account.ID == selectedID
		for _, id := range retainedArchivedIDs {
			retainArchived = retainArchived || account.ID == id
		}
		if account.Archived && !retainArchived {
			continue
		}
		visible = append(visible, account)
		name := strings.ToLower(strings.TrimSpace(account.Name))
		if nameTypes[name] == nil {
			nameTypes[name] = make(map[domain.AccountType]struct{})
		}
		nameTypes[name][account.Type] = struct{}{}
	}

	options := make([]TxnFormOption, 0, len(visible))
	for _, account := range visible {
		label := account.Name
		name := strings.ToLower(strings.TrimSpace(account.Name))
		if len(nameTypes[name]) > 1 {
			label += " · " + txnAccountTypeCue(account.Type)
		}
		options = append(options, TxnFormOption{Value: account.ID, Label: label})
	}
	return options
}

func txnAccountTypeCue(accountType domain.AccountType) string {
	label := strings.ReplaceAll(string(accountType), "_", " ")
	if label == "" {
		return "Other"
	}
	runes := []rune(label)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
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

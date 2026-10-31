// SPDX-License-Identifier: MIT

package accountselect

import "github.com/monstercameron/CashFlux/internal/domain"

// CanPostTransfer reports whether the transfer form's current text would post: two different
// accounts picked, a valid positive amount, and clean optional received and fee fields. The date is
// left out, because the form states a bad date in its own message when it is submitted. Both
// transfer forms gate their submit button on it.
func CanPostTransfer(accounts []domain.Account, in TransferInput) bool {
	if in.FromID == "" || in.ToID == "" || in.FromID == in.ToID {
		return false
	}
	in.Date = ""
	_, err := ParseTransfer(accounts, in)
	return err == nil
}

// IsCrossCurrency reports whether both accounts are picked and are held in different currencies.
func IsCrossCurrency(accounts []domain.Account, fromID, toID string) bool {
	from, to := CurrencyOf(accounts, fromID), CurrencyOf(accounts, toID)
	return from != "" && to != "" && from != to
}

// HasTransferPair reports whether any transfer can be posted at all: some account can be a source
// and a different account can be the destination (see TransferCandidates for the rules). Two
// property accounts, for example, are both active but neither can send or receive, so a page with
// only those offers no transfer.
func HasTransferPair(accounts []domain.Account) bool {
	from, to := TransferCandidates(accounts, "", "")
	if len(from) == 0 || len(to) == 0 {
		return false
	}
	return len(from) > 1 || len(to) > 1 || from[0].ID != to[0].ID
}

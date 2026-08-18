// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "github.com/monstercameron/CashFlux/internal/domain"

// reconSignHintKey names the sentence that tells the user which sign to type.
//
// The convention is one sentence for every account, but WHICH sentence depends on
// what the account is: "money you owe is negative" is the useful half on a card
// and noise on a checking account, and a dialog that says both every time trains
// people to read neither.
func reconSignHintKey(a domain.Account) string {
	if a.Class == domain.ClassLiability {
		return "accounts.reconSignHintDebt"
	}
	return "accounts.reconSignHintAsset"
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// isBalanceAdjustment reports whether a row is a balance the USER stated rather
// than money that moved — what the provenance chip, the net-worth bridge and the
// balance-quality assessment all mean by "adjusted".
//
// It asks the row structurally first. A provisional balance checkpoint (C684) IS
// such a row and says so in a field, which is the marker
// ledger.BalanceProvenance's doc comment notes did not exist when it was written
// ("adjustments are marked at the UI layer (description text), not
// structurally"). Now it does.
//
// The description match stays as a fallback for rows written before checkpoints
// existed. Three call sites each rolled their own copy of that string comparison,
// so when the description changed, the provenance chip silently stopped
// appearing — the balance strip lost a line, and a regression test caught it only
// because the modal's footer moved 48 pixels. A structural marker cannot fail
// that way.
func isBalanceAdjustment(t domain.Transaction) bool {
	if t.IsBalanceCheckpoint() {
		return true
	}
	return t.Source == domain.TxnSourceManual && t.Desc == uistate.T("accounts.balanceAdjustment")
}

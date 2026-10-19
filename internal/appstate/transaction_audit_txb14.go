// SPDX-License-Identifier: MIT

package appstate

import "github.com/monstercameron/CashFlux/internal/auditlog"

// TransactionAudit returns the complete audit history for one transaction,
// newest first. Older events outside the recent activity window remain visible.
func (a *App) TransactionAudit(transactionID string) []auditlog.Entry {
	if transactionID == "" {
		return nil
	}
	entries, err := a.store.ListAuditEntriesByEntity("transaction", transactionID, 0)
	if err != nil {
		a.log.Error("audit: list transaction history", "transaction_id", transactionID, "err", err)
		return nil
	}
	return entries
}

// SPDX-License-Identifier: MIT

package workflow

var transactionVariableNames = [...]string{
	"txn_amount", "txn_abs", "txn_payee", "txn_desc", "txn_category",
	"txn_account", "txn_tags", "txn_payee_typical", "txn_vs_typical",
}

// TransactionVariableNames lists the reserved variables supplied by a triggering
// transaction. Monetary values use that transaction's currency in major units;
// txn_vs_typical is a ratio. The returned slice is safe for callers to modify.
func TransactionVariableNames() []string {
	return append([]string(nil), transactionVariableNames[:]...)
}

// IsTransactionVariable reports whether a name requires a transaction context.
// These reserved names cannot be used by a non-transaction trigger, even if a
// user-defined molecule happens to have the same name.
func IsTransactionVariable(name string) bool {
	for _, candidate := range transactionVariableNames {
		if name == candidate {
			return true
		}
	}
	return false
}

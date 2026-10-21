// SPDX-License-Identifier: MIT

package workflow

import "testing"

func TestTransactionVariableNamesAreReservedAndIsolated(t *testing.T) {
	names := TransactionVariableNames()
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] || !IsTransactionVariable(name) {
			t.Fatalf("invalid/duplicate declaration %q", name)
		}
		seen[name] = true
	}
	for _, name := range []string{"txn_abs", "txn_amount", "txn_payee", "txn_desc", "txn_tags", "txn_account", "txn_category", "txn_payee_typical", "txn_vs_typical"} {
		if !seen[name] {
			t.Errorf("missing runtime declaration %s", name)
		}
	}
	for _, name := range []string{"income", "cf_txn_tip", "txn_undefined"} {
		if IsTransactionVariable(name) {
			t.Errorf("%s incorrectly reserved as a transaction context field", name)
		}
	}
	names[0] = "modified"
	if TransactionVariableNames()[0] == "modified" {
		t.Fatal("caller changed the canonical declaration")
	}
}

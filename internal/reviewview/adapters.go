// SPDX-License-Identifier: MIT

package reviewview

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// Memo caches a derived value by explicit owner identity and revision. It never
// compares slice base pointers, so replacement or append at the same base is safe.
type Memo[K comparable, V any] struct {
	owner       K
	revision    uint64
	initialized bool
	value       V
}

// Get loads once for each owner/revision pair, including an empty first result.
func (m *Memo[K, V]) Get(owner K, revision uint64, load func() V) V {
	if !m.initialized || m.owner != owner || m.revision != revision {
		m.owner, m.revision = owner, revision
		m.value = load()
		m.initialized = true
	}
	return m.value
}

// DecodeDismissals preserves the existing empty-map fallback for unreadable KV.
func DecodeDismissals(raw string) map[string]bool {
	var dismissed map[string]bool
	if err := json.Unmarshal([]byte(raw), &dismissed); err != nil || dismissed == nil {
		return map[string]bool{}
	}
	return dismissed
}

// EncodeDismissals encodes a dismissal snapshot using the existing JSON format.
func EncodeDismissals(dismissed map[string]bool) string {
	// A string-to-bool map is always JSON encodable.
	raw, _ := json.Marshal(dismissed)
	return string(raw)
}

// LoanPayment validates that the offer is still current, then builds the
// in-place transfer conversion. The caller owns the actual durable write.
func LoanPayment(suggestion txnclassify.PaymentSuggestion, current []txnclassify.PaymentSuggestion, txn domain.Transaction, found bool, accounts []domain.Account) (domain.Transaction, error) {
	active := false
	for _, candidate := range current {
		if candidate.TransactionID == suggestion.TransactionID && candidate.AccountID == suggestion.AccountID {
			active = true
			break
		}
	}
	if !active {
		return domain.Transaction{}, fmt.Errorf("review: loan payment suggestion is no longer available")
	}
	if !found {
		return domain.Transaction{}, fmt.Errorf("review: transaction %q was not found", suggestion.TransactionID)
	}
	classified, err := txnclassify.Apply(txn, suggestion.AccountID, true, accounts)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("review: classify loan payment: %w", err)
	}
	return classified, nil
}

// PlainLeftClick mirrors the router's native-click navigation boundary. A
// missing button property is treated as a left click, as in the original UI.
func PlainLeftClick(button int, numeric, meta, ctrl, shift, alt bool) bool {
	return (!numeric || button == 0) && !meta && !ctrl && !shift && !alt
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "strings"

// Reconcile drafts (QA R3 CF-02, "save incomplete reconciliation"): a per-
// account statement balance + closing date the user typed but did not finish
// reconciling. Stored in the SQLite-backed KV (rides the dataset export), one
// key per account, so an interrupted reconciliation can resume later — and the
// "continue where I left off" surface can list it.

// reconcileDraftKey builds the per-account KV key.
func reconcileDraftKey(accountID string) string { return "cashflux:reconcile:draft:" + accountID }

// SaveReconcileDraft persists an in-progress reconciliation's statement balance
// (the raw typed string) and ISO closing date for the account.
func SaveReconcileDraft(accountID, balance, dateISO string) {
	KVSet(reconcileDraftKey(accountID), balance+"|"+dateISO)
}

// LoadReconcileDraft returns the saved draft for the account and whether one
// exists.
func LoadReconcileDraft(accountID string) (balance, dateISO string, ok bool) {
	raw := KVGet(reconcileDraftKey(accountID))
	if raw == "" {
		return "", "", false
	}
	balance, dateISO, _ = strings.Cut(raw, "|")
	return balance, dateISO, balance != "" || dateISO != ""
}

// ClearReconcileDraft removes the account's saved draft (called when a
// reconciliation is recorded, force-completed, or explicitly discarded).
func ClearReconcileDraft(accountID string) { KVSet(reconcileDraftKey(accountID), "") }

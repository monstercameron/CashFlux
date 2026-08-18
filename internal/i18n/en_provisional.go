// SPDX-License-Identifier: MIT

package i18n

// provisionalKeys holds the strings for provisional balance checkpoints (C684).
// Merged via init so this file never touches en.go.
var provisionalKeys = Catalog{
	// The row's own description. It says what the row IS, because someone meeting
	// it in the ledger months later needs to know it is a placeholder rather than
	// a payment — "Balance adjustment" told them neither.
	"accounts.balanceCheckpointDesc": "Balance checkpoint (awaiting statement)",
	// Said when an earlier guess is replaced, so a number quietly disappearing
	// from the ledger is explained rather than noticed later.
	"accounts.balanceCheckpointReplaced": "Replaced %d earlier balance checkpoint(s) — only the latest one stands.",
	// Said when the statement finally confirms the balance and the guess retires.
	"accounts.reconCheckpointsRetired": "%d balance checkpoint(s) removed — the statement covers that period now.",
}

func init() {
	for k, v := range provisionalKeys {
		english[k] = v
	}
}

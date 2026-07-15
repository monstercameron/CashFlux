// SPDX-License-Identifier: MIT

package i18n

// splitOwnerKeys holds English strings for the per-line owner picker in the
// split editor (XC10 — a split line can be attributed to a household member
// other than the transaction's payer). Registered via the new-file init-merge
// pattern so en.go (which may have concurrent WIP) is never touched.
var splitOwnerKeys = Catalog{
	// Accessible label for the per-line owner select in the split editor.
	"splitEditor.owner": "Who this line is for",
	// Default owner option: the line follows the transaction's own member.
	"splitEditor.ownerSameAsTxn": "Same as transaction",
	// Prefix shown before a member's name on a read-only split line that carries
	// its own owner, e.g. "for Alex".
	"splitEditor.ownerFor": "for %s",
}

func init() {
	for k, v := range splitOwnerKeys {
		english[k] = v
	}
}

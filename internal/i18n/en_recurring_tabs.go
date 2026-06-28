// SPDX-License-Identifier: MIT

package i18n

// recurringTabKeys holds the English strings for the three-tab /recurring hub
// (FEATURE_MAP §5.3). Merged via init so this file does not touch en.go;
// mirrors the en_syncconflict.go / en_webauthn.go init-merge pattern.
var recurringTabKeys = Catalog{
	"recurring.tabScheduled":     "Scheduled",
	"recurring.tabBills":         "Bills",
	"recurring.tabSubscriptions": "Subscriptions",
}

func init() {
	for k, v := range recurringTabKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

// payeeAliasKeys holds English strings for the TX1 merchant-name cleanup
// (payee aliases) and the TX7 "apply to similar" recategorize offer. Kept in a
// separate file (not en.go) so this change does not touch the concurrent-WIP
// file. Registered into the catalog at init time.
var payeeAliasKeys = Catalog{
	// TX1 — learning flow: quiet confirm shown after a payee rename.
	// %s = the raw payee just changed, %s = the new display name.
	"payeealias.learnPrompt": "Always show “%s” as “%s”?",
	"payeealias.learned":     "Saved. “%s” now shows as “%s” everywhere.",

	// TX1 — management UI (a section on the Rules screen).
	"payeealias.sectionTitle":       "Merchant names",
	"payeealias.sectionHint":        "Clean up noisy payee names once and they show that way everywhere — in the ledger, filters, and reports. The original name stays on each transaction.",
	"payeealias.rawLabel":           "Original name",
	"payeealias.displayLabel":       "Show as",
	"payeealias.rawPlaceholder":     "AMZN Mktp US*2K4RT0",
	"payeealias.displayPlaceholder": "Amazon",
	"payeealias.addBtn":             "Add name",
	"payeealias.empty":              "No custom merchant names yet. Add one, or rename a payee while editing a transaction.",
	"payeealias.deleteConfirm":      "Remove the merchant name “%s”? Transactions keep their original names.",
	"payeealias.needBoth":           "Enter both the original name and the name to show.",

	// TX7 — apply-to-similar recategorize offer, shown after a category change.
	// %d = number of similar transactions found.
	"similartxns.offer":       "%d more look like this — recategorize them too?",
	"similartxns.offerOne":    "1 more looks like this — recategorize it too?",
	"similartxns.apply":       "Recategorize them",
	"similartxns.alwaysDo":    "Always do this",
	"similartxns.dismiss":     "No thanks",
	"similartxns.moreCount":   "and %d more",
	"similartxns.hasCategory": "already categorized",
	"similartxns.applied":     "Recategorized %d transaction(s).",
}

func init() {
	for k, v := range payeeAliasKeys {
		english[k] = v
	}
}

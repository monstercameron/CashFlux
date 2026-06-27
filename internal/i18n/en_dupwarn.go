// SPDX-License-Identifier: MIT

package i18n

// dupWarnKeys holds English strings for the C88 pre-import duplicate-warning
// feature. Kept in a separate file (not en.go) so this change does not touch
// the concurrent-WIP file. Registered into the catalog at init time.
var dupWarnKeys = Catalog{
	// Pre-import warning shown in the CSV paste card when duplicates are detected.
	// %d = duplicate count, %d = total parseable rows.
	"documents.dupWarnBanner": "%d of %d rows look like duplicates of transactions you already have — they'll be skipped.",

	// Warning shown when ALL rows are duplicates.
	"documents.dupWarnAllDups": "All %d rows look like duplicates of transactions you already have — nothing would be imported.",

	// Confirm button label shown alongside the warning.
	"documents.dupWarnConfirm": "Import anyway",

	// Label for the preview step that reveals the duplicate count.
	"documents.dupWarnPreview": "Preview",
}

func init() {
	for k, v := range dupWarnKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

// impcKeys contains English copy for safer transaction import previews and limits.
var impcKeys = Catalog{
	"impc.aiDisclosure":         "When you click “Read with AI,” this selected image is sent to OpenAI for analysis.",
	"impc.fileTooLarge":         "This %s is too large. Choose a file no larger than %s.",
	"impc.backupTooLarge":       "This %s is too large to restore. Choose a backup no larger than %s.",
	"impc.tooManyRows":          "This file has more than %d rows. Split it into smaller files and try again.",
	"impc.noValidRows":          "No rows could be imported. %d rows need correction.",
	"impc.importSummary":        "Imported: %d. Skipped as duplicates: %d. Failed rows: %d. Possible duplicates to review: %d.",
	"impc.rowCounts":            "Before import: %d ambiguous dates, %d ambiguous numbers, %d invalid rows.",
	"impc.pageRange":            "Rows %d–%d of %d",
	"impc.previousPage":         "Previous",
	"impc.nextPage":             "Next",
	"impc.duplicateCount":       "Duplicate rows across this import: %d of %d. They will be skipped when you import.",
	"impc.clearAllRows":         "Clear all rows",
	"impc.importAllRows":        "Import all %d rows",
	"impc.suggestCategoriesAll": "Suggest categories for all rows",
	"impc.formatTitle":          "Check the statement format",
	"impc.dateOrder":            "Date order",
	"impc.monthDay":             "Month / day / year",
	"impc.dayMonth":             "Day / month / year",
	"impc.numberFormat":         "Number format",
	"impc.detectAutomatically":  "Detect automatically",
	"impc.dotDecimal":           "1,234.56 (period decimal)",
	"impc.commaDecimal":         "1.234,56 (comma decimal)",
	"impc.applyFormats":         "Apply formats and refresh preview",
}

func init() {
	for key, value := range impcKeys {
		english[key] = value
	}
}

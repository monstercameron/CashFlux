// SPDX-License-Identifier: MIT

package i18n

// impcKeys contains English copy for safer transaction import previews and limits.
var impcKeys = Catalog{
	"impc.aiDisclosure":         "The image is sent to OpenAI with your key only after you choose Read with AI. OpenAI charges for the request.",
	"impc.aiNeedsKey":           "Add an OpenAI key in Settings before using this AI feature.",
	"impc.fileTooLarge":         "This %s is too large. Choose a file no larger than %s.",
	"impc.backupTooLarge":       "This %s is too large to restore. Choose a backup no larger than %s.",
	"impc.tooManyRows":          "This file has more than %d rows. Split it into smaller files and try again.",
	"impc.noValidRows":          "Rows needing attention: %d. Correct or remove them before importing.",
	"impc.importSummary":        "Import result: %d added, %d already in your ledger, %d could not be read, and %d possible duplicates to review.",
	"impc.rowCounts":            "Before import: %d ambiguous dates, %d ambiguous numbers, %d invalid rows.",
	"impc.pageRange":            "Rows %d–%d of %d",
	"impc.previousPage":         "Previous",
	"impc.nextPage":             "Next",
	"impc.duplicateCount":       "%d of %d rows repeat within this import. Review them before importing.",
	"impc.clearAllRows":         "Clear all rows",
	"impc.importAllRows":        "Import rows (%d)",
	"impc.importOneToAccount":   "Import 1 transaction to %s",
	"impc.importManyToAccount":  "Import %d transactions to %s",
	"impc.close":                "Close",
	"impc.suggestCategoriesAll": "Suggest categories with AI",
	"impc.pdfDisclosure":        "The PDF is sent to OpenAI with your key only after you choose Read statement. OpenAI charges for the request.",
	"impc.extractDisclosure":    "The statement text is sent to OpenAI with your key only after you choose Extract with AI. OpenAI charges for the request.",
	"impc.suggestDisclosure":    "Transaction descriptions and category names are sent to OpenAI with your key only after you choose Suggest categories with AI. OpenAI charges for the request.",
	"impc.categoriesSuggested":  "Categories suggested.",
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

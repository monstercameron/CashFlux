// SPDX-License-Identifier: MIT

package i18n

// categoriesSurfaceKeys holds the English strings for the redesigned
// /categories taxonomy-ledger surface: the hero (filed spending + figure chips
// + the plain-English takeaway) and the per-category figure rows. Merged via
// init so this file does not touch en.go.
var categoriesSurfaceKeys = Catalog{
	"categories.heroTitle":    "Your categories",
	"categories.heroLabel":    "Spent this period",
	"categories.chipExpense":  "Expense",
	"categories.chipIncome":   "Income",
	"categories.chipDeduct":   "Deductible",
	"categories.chipUnfiled":  "Not filed yet",
	"categories.countWord":    "%d categories",
	"cats.leadTake":           "%s leads this period's spending.",
	"cats.quietTake":          "Nothing spent this period yet.",
	"cats.filedClause":        "Everything you spent is filed under a category.",
	"cats.unfiledClause":      "%s of it isn't filed under any category yet.",
	"categories.mapTake":      "Every category at a glance — parents with their sub-categories.",
	"categories.deductTag":    "Deductible",
	"categories.menuAria":     "Category actions",
	"categories.earnedSub":    "earned this period",
	"categories.spentSub":     "spent this period",
}

func init() {
	for k, v := range categoriesSurfaceKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"txp3.bulkChoosePerson":   "Choose a person",
		"txp3.bulkCategorise":     "Categorize",
		"txp3.bulkAssign":         "Assign",
		"txp3.bulkStatus":         "Status",
		"txp3.bulkMore":           "More",
		"txp3.bulkApplyCategory":  "Apply category",
		"txp3.bulkClearCategory":  "Clear category",
		"txp3.bulkAssignPerson":   "Assign person",
		"txp3.bulkMarkCleared":    "Mark as cleared",
		"txp3.bulkMarkUncleared":  "Mark as not cleared",
		"txp3.bulkExcludeReports": "Exclude from budgets and reports",
		"txp3.bulkIncludeReports": "Include in budgets and reports",
		"txp3.bulkGroupPurchase":  "Group as one purchase",
		"txp3.bulkExportSelected": "Export selected",
		"txp3.bulkDeleteSelected": "Delete selected",
	} {
		english[key] = value
	}
}

// SPDX-License-Identifier: MIT

package i18n

// txnWidgetKeys holds the English strings added for the widgetized transactions
// page (KPI tiles + the edit modal). Defined in their own file and merged via
// init so this does not touch the user-WIP en.go; mirrors the en_quickaddfix.go
// pattern.
var txnWidgetKeys = Catalog{
	// KPI tile titles + sub-lines for the fixed transactions bento.
	"txnwidget.countTitle":     "Transactions",
	"txnwidget.countSub":       "matching your filters",
	"txnwidget.netTitle":       "Net",
	"txnwidget.netSub":         "across the shown set",
	"txnwidget.unclearedTitle": "Uncleared",
	"txnwidget.unclearedSub":   "not yet reconciled",
	"txnwidget.tableTitle":     "All transactions",

	// Edit modal.
	"txnwidget.notFound":     "That transaction could not be found.",
	"txnwidget.clearedLabel": "Cleared (reconciled)",

	// Filters panel — the Cleared-status select's neutral resting label (review #19):
	// empty value = no cleared-status filter, so it must not read as an active pick.
	"transactions.clearedAny": "— Any —",
}

func init() {
	for k, v := range txnWidgetKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

// txnReviewKeys holds the English copy for the transfer review panel (C676):
// resolving a statement's worth of mis-filed transfers as a handful of decisions.
//
// The register is deliberate throughout. These rows are not errors the user made;
// they are what a bank export looks like, and copy that scolds ("fix these") would
// be blaming someone for their statement. It says what the rows are doing to the
// figures, what the app believes, and how sure it is — and leaves the decision
// where it belongs. Kept in its own file and merged via init so it never touches
// the shared en.go.
var txnReviewKeys = Catalog{
	// Entry point.
	"txnreview.reviewBtn":   "Link transfers",
	"txnreview.reviewBadge": "Link transfers (%s)",

	// The panel.
	"txnreview.intro":     "These transactions look like money moved between your accounts. Choose the other account so they stop counting as income or spending.",
	"txnreview.empty":     "No unlinked transfers found. Transactions that move money between your accounts are already left out of income and spending.",
	"txnreview.unrouted":  "Choose the other account",
	"txnreview.batchMeta": "%s · %s counting right now",

	// Why a group was grouped. A user confirming a grouping is entitled to know
	// how much of it is evidence and how much is a reading.
	"txnreview.basisReciprocal": "The other side of these transfers is in that account on a matching date.",
	"txnreview.basisName":       "These transactions name that account in their text.",
	"txnreview.basisCategory":   "Assigned to a payment category, and this is the only account of that kind.",
	"txnreview.basisNone":       "Nothing in these rows says where the money went, so they are yours to place.",

	// Controls.
	"txnreview.counterparty":    "Other account",
	"txnreview.pickAccount":     "Choose an account",
	"txnreview.countTowardDebt": "Count these as payments toward what you owe",

	// The consequence, stated before the button rather than after it.
	"txnreview.summary": "%s will stop counting as income or spending — %s in all.",
	// Several currencies in one selection: this layer has no rate table, so the
	// row count is stated and the amounts stay on the per-batch lines, which are
	// each a single currency. A summed figure with one symbol on it would look
	// precise and mean nothing.
	"txnreview.summaryRows":  "%s will stop counting as income or spending.",
	"txnreview.summaryMixed": "%s will stop counting as income or spending across %s currencies — amounts are listed by account above.",
	// An uncertain pairing is not a wrong classification, and says so.
	"txnreview.ambiguousLeg": "More than one transaction in that account could match this one",
	"txnreview.debtNote":     "%s will also count as paying down that balance.",
	"txnreview.failures":     "%s can't take this and will be left alone; each one says why above.",
	"txnreview.apply":        "Link transfers",

	"txnreview.appliedToast": "Linked %s.",

	// C672: the leak notice. It changes no total — it states the size of an error
	// beside the figures it affects. A report wrong by a known amount that says so
	// is honest; the same report saying nothing is not.
	"txnleak.title":        "%s in this %s look like transfers that were never linked, so they are counted here.",
	"txnleak.income":       "%s of it is counted as income you didn't earn.",
	"txnleak.spending":     "%s of it is counted as spending you didn't do.",
	"txnleak.review":       "Review them",
	"txnleak.reviewTitle":  "Open the transfer review, where you can link them in a few clicks",
	"txnleak.whereReport":  "report",
	"txnleak.whereBudgets": "period",
}

func init() {
	for k, v := range txnReviewKeys {
		english[k] = v
	}
}

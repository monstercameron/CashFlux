// SPDX-License-Identifier: MIT

package i18n

// importAuditKeys holds the strings the import history needs to say what an
// import actually did, and what has become of it since (C687). Merged via init
// so this file never touches en.go.
//
// The history used to say "N imported · M skipped". Both halves were unhelpful:
// "skipped" lumped together rows the ledger already had and rows that could not
// be read at all, and "imported" was a claim about the past with nothing to check
// it against — which is how a history claiming 688 rows sat beside a ledger
// holding 565 with no way to reconcile the two.
var importAuditKeys = Catalog{
	// Duplicates are the safeguard working. Re-importing an overlapping statement
	// is a normal thing to do, so this must not read as a fault.
	"documents.historyDuplicates": "· %d already in your ledger",
	// A parse failure IS a fault: that money is not in the ledger and nothing
	// else will mention it.
	"documents.historyFailed": "· %d could not be read",
	// Said only when rows have gone since. Both numbers, because "3 removed" on
	// its own invites the question this line exists to answer.
	"documents.historyRemoved": "· %d still here, %d removed since",
	// For imports made before rows recorded which run created them. Saying "0
	// still here" would be a confident lie in the direction that alarms people.
	"documents.historyUntraceable": "· from before imports were tracked, so how many remain is not recorded",
}

func init() {
	for k, v := range importAuditKeys {
		english[k] = v
	}
}

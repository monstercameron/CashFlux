// SPDX-License-Identifier: MIT

package i18n

// stagingKeys holds the strings for the staged-import preview (C689). Merged via
// init so this file never touches en.go.
//
// Both outcomes below used to be either invisible or counted as "skipped"
// alongside ordinary duplicates, and they mean opposite things. One is the file
// being redundant, which costs nothing. The other is money that will not arrive
// in the ledger and that nothing else will ever mention.
var stagingKeys = Catalog{
	"documents.preflightRepeated": "%d row(s) appear more than once in this file — only the first of each will be imported.",
	"documents.preflightUnusable": "%d row(s) could not be read and will not be imported. Their money will be missing from this account.",
}

func init() {
	for k, v := range stagingKeys {
		english[k] = v
	}
}

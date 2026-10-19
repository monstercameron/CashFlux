// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR2ATransactionColumnsFitTheirContent(t *testing.T) {
	resetSheet()
	registerUxbatch6()
	out := Build()
	for _, want := range []struct{ selector, width string }{
		{selector: ".txn-table th.td-date", width: "width:112px;"},
		{selector: ".txn-table th.td-cat", width: "width:170px;"},
		{selector: ".txn-table th.td-acct", width: "width:150px;"},
		{selector: ".txn-table th.td-user", width: "width:56px;"},
	} {
		if !strings.Contains(out, want.selector) || !strings.Contains(out, want.width) {
			t.Errorf("transaction column rule missing %q with %q", want.selector, want.width)
		}
	}
}

func TestR2ACountLineFiltersAndUpcomingDisclosureHaveRules(t *testing.T) {
	resetSheet()
	registerGenerated()
	out := Build()
	for _, want := range []string{
		".txn-count-filter{",
		".txn-count-filter:focus-visible{",
		".txn-countline .txn-count-upcoming-rows{",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transaction count-line rule missing %q", want)
		}
	}
}

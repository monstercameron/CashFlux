// SPDX-License-Identifier: MIT

package cmdmatch

import (
	"strings"
	"testing"
)

func ids(cmds []Command) string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.ID
	}
	return strings.Join(out, ",")
}

var sample = []Command{
	{ID: "txns", Title: "Transactions"},
	{ID: "budgets", Title: "Budgets"},
	{ID: "accounts", Title: "Accounts"},
	{ID: "newtxn", Title: "New transaction", Keywords: []string{"add", "create", "new"}},
	{ID: "export", Title: "Export JSON", Keywords: []string{"backup", "save"}},
}

func TestMatchEmptyReturnsAllInOrder(t *testing.T) {
	got := Match("   ", sample)
	if ids(got) != "txns,budgets,accounts,newtxn,export" {
		t.Errorf("empty query order = %s", ids(got))
	}
}

func TestMatchSubsequenceTitle(t *testing.T) {
	got := Match("budg", sample)
	if len(got) == 0 || got[0].ID != "budgets" {
		t.Errorf("'budg' should rank Budgets first, got %s", ids(got))
	}
}

func TestMatchKeywordAlias(t *testing.T) {
	// "add" matches the noun-labeled "New transaction" only via its keyword.
	got := Match("add", sample)
	found := false
	for _, c := range got {
		if c.ID == "newtxn" {
			found = true
		}
	}
	if !found {
		t.Errorf("'add' should surface New transaction via its alias, got %s", ids(got))
	}
}

func TestMatchTitleBeatsKeyword(t *testing.T) {
	cmds := []Command{
		{ID: "kwonly", Title: "Backup everything", Keywords: []string{"save"}},
		{ID: "titlematch", Title: "Save filter", Keywords: nil},
	}
	got := Match("save", cmds)
	if len(got) != 2 || got[0].ID != "titlematch" {
		t.Errorf("a title match should outrank a keyword-only match, got %s", ids(got))
	}
}

func TestMatchPrefixRanksAboveScattered(t *testing.T) {
	// "trans" is a prefix of Transactions and scattered in "New transaction".
	got := Match("trans", sample)
	if len(got) < 2 || got[0].ID != "txns" {
		t.Errorf("prefix match Transactions should rank first, got %s", ids(got))
	}
}

func TestMatchNoMatchEmpty(t *testing.T) {
	if got := Match("zzzz", sample); len(got) != 0 {
		t.Errorf("no-match query should be empty, got %s", ids(got))
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	if got := Match("BUDG", sample); len(got) == 0 || got[0].ID != "budgets" {
		t.Errorf("match should be case-insensitive, got %s", ids(got))
	}
}

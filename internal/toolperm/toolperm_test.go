// SPDX-License-Identifier: MIT

package toolperm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestForDescribesWhatAWriteToolChangesAndItsScope(t *testing.T) {
	p := For("categorize_transactions", json.RawMessage(`{"match":"Trader Joe's","category":"Groceries"}`))
	if !p.Known {
		t.Fatal("categorize_transactions is not in the table")
	}
	if p.ReadOnly() {
		t.Fatal("a categorize call reported as read-only")
	}
	line := p.Writes[0].Line()
	if !strings.Contains(line, "Trader Joe's") {
		t.Fatalf("write line does not state the scope: %q", line)
	}
	if !strings.HasPrefix(line, "Changes") {
		t.Fatalf("write line does not lead with the verb: %q", line)
	}
	if len(p.Reads) == 0 {
		t.Fatal("a tool that scans every transaction reported no reads")
	}
}

func TestForCountsWhatItCanAndSaysNothingWhenItCannot(t *testing.T) {
	one := For("add_transaction", json.RawMessage(`{"account":"Checking","amount":12}`))
	if got := one.Writes[0].Line(); !strings.Contains(got, "1 transaction") {
		t.Fatalf("a single add did not say one: %q", got)
	}
	if got := one.Writes[0].Line(); !strings.Contains(got, `in "Checking"`) {
		t.Fatalf("the account scope is missing: %q", got)
	}

	// The number of matches depends on the data, not the request, so the card must
	// not invent a count — least of all "0".
	many := For("categorize_transactions", json.RawMessage(`{"match":"coffee","category":"Dining"}`))
	if got := many.Writes[0].Line(); strings.Contains(got, "0 ") {
		t.Fatalf("an unknown count printed as zero: %q", got)
	}
}

func TestMergeCountsTheRowsItRemovesNotTheOnesItReads(t *testing.T) {
	p := For("merge_duplicate_transactions", json.RawMessage(`{"ids":["a","b","c"]}`))
	got := p.Writes[0].Line()
	if !strings.Contains(got, "2 duplicate transactions") {
		t.Fatalf("merging 3 rows should remove 2: %q", got)
	}
	if p.Reversible {
		t.Fatal("a merge that removes rows is reported as reversible")
	}
}

func TestMissingArgumentsDoNotProduceAbsurdCounts(t *testing.T) {
	p := For("merge_duplicate_transactions", json.RawMessage(`{}`))
	if got := p.Writes[0].Count; got != 0 {
		t.Fatalf("count = %d, want 0 rather than a negative number", got)
	}
	if got := p.Writes[0].Line(); strings.Contains(got, "-") {
		t.Fatalf("line shows a negative count: %q", got)
	}
}

func TestMalformedArgumentsStillProduceACard(t *testing.T) {
	p := For("add_task", json.RawMessage(`{"title": `))
	if !p.Known || len(p.Writes) == 0 {
		t.Fatalf("malformed arguments broke the permission: %+v", p)
	}
	if got := p.Writes[0].Line(); !strings.Contains(got, "1 to-do") {
		t.Fatalf("write line = %q", got)
	}
}

func TestUnknownToolIsDescribedConservatively(t *testing.T) {
	p := For("wire_money_to_a_stranger", json.RawMessage(`{}`))
	if p.Known {
		t.Fatal("an unlisted tool claimed to be known")
	}
	if p.ReadOnly() {
		t.Fatal("an unlisted tool was described as harmless")
	}
	if p.Reversible {
		t.Fatal("an unlisted tool claimed its change could be undone")
	}
}

func TestEveryMutatingToolIsDescribed(t *testing.T) {
	// The assistant's write tools, from internal/screens/chat_agent*.go. A tool
	// added there without an entry here would show an unknown-change card, which
	// is safe but unhelpful — this test is the reminder to describe it.
	for _, tool := range []string{
		"add_task", "complete_task", "add_transaction", "create_category",
		"categorize_transactions", "add_goal_contribution", "add_account",
		"add_transfer", "update_account_balance", "merge_duplicate_transactions",
		"delete_transaction", "dismiss_flagged_activity", "remember_fact",
	} {
		p := For(tool, json.RawMessage(`{}`))
		if !p.Known {
			t.Errorf("%s has no entry in the permission table", tool)
			continue
		}
		if len(p.Writes) == 0 {
			t.Errorf("%s is a write tool but declares no writes", tool)
		}
		for _, w := range p.Writes {
			if strings.TrimSpace(w.Entity) == "" {
				t.Errorf("%s declares a write with no entity", tool)
			}
		}
	}
}

func TestPluralHandlesMultiWordEntities(t *testing.T) {
	for _, tc := range []struct {
		n      int
		entity string
		want   string
	}{
		{1, "transaction", "1 transaction"},
		{2, "transaction", "2 transactions"},
		{2, "account balance", "2 account balances"},
		{3, "to-do", "3 to-dos"},
	} {
		if got := plural(tc.n, tc.entity); got != tc.want {
			t.Errorf("plural(%d, %q) = %q, want %q", tc.n, tc.entity, got, tc.want)
		}
	}
}

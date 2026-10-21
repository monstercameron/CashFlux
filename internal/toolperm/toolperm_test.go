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

func TestMergeDescribesItsActualMatchArgumentsWithoutInventingCount(t *testing.T) {
	p := For("merge_duplicate_transactions", json.RawMessage(`{"target":"coffee","transaction_id":"txn-42","amount":"12.5000000000000001","date":"2026-08-01"}`))
	w := p.Writes[0]
	got := w.Line()
	if w.Count != 0 || !strings.Contains(got, "coffee") || !strings.Contains(got, "txn-42") || !strings.Contains(got, "12.5000000000000001") || !strings.Contains(got, "2026-08-01") {
		t.Fatalf("merge permission should describe the actual selector without claiming a count: %+v; line %q", w, got)
	}
	if p.Reversible {
		t.Fatal("a merge that removes rows is reported as reversible")
	}
}

func TestStructuredSelectorsPreferStableIDsAndKeepLegacyTextFallbacks(t *testing.T) {
	complete := For("complete_task", json.RawMessage(`{"task_id":"task-17","title":"Bills"}`))
	if got := complete.Writes[0].Line(); !strings.Contains(got, `task-17`) || strings.Contains(got, `Bills`) {
		t.Fatalf("completion should scope by stable task ID when present: %q", got)
	}
	legacyComplete := For("complete_task", json.RawMessage(`{"title":"Bills"}`))
	if got := legacyComplete.Writes[0].Line(); !strings.Contains(got, `matching title "Bills"`) {
		t.Fatalf("legacy title selector missing: %q", got)
	}

	goal := For("add_goal_contribution", json.RawMessage(`{"goal_id":"goal-9","goal":"Trip","amount":"12345678901234567890.1234","currency":"EUR"}`))
	if got := goal.Writes[0].Line(); !strings.Contains(got, `goal-9`) || strings.Contains(got, `Trip`) || !strings.Contains(got, `12345678901234567890.1234`) || !strings.Contains(got, `EUR`) {
		t.Fatalf("goal contribution scope lost stable ID or exact amount: %q", got)
	}
	legacyGoal := For("add_goal_contribution", json.RawMessage(`{"goal":"Trip","amount":12.50}`))
	if got := legacyGoal.Writes[0].Line(); !strings.Contains(got, `named "Trip"`) || !strings.Contains(got, `amount 12.50`) {
		t.Fatalf("legacy goal selector or exact JSON number missing: %q", got)
	}
}

func TestTransactionWritePermissionsNameExactIDs(t *testing.T) {
	deleted := For("delete_transaction", json.RawMessage(`{"target":"Cafe","transaction_id":"txn-delete","amount":"38.49","date":"2026-08-01"}`))
	if got := deleted.Writes[0].Line(); !strings.Contains(got, `txn-delete`) || !strings.Contains(got, `38.49`) {
		t.Fatalf("delete scope missing stable selector: %q", got)
	}
	updated := For("update_transaction", json.RawMessage(`{"id":"txn-edit","amount":12345678901234567890.01,"payee":"Cafe"}`))
	if got := updated.Writes[0].Line(); !strings.Contains(got, `txn-edit`) || !strings.Contains(got, `amount`) || !strings.Contains(got, `payee`) {
		t.Fatalf("edit scope missing ID or changed fields: %q", got)
	}
}

func TestMissingArgumentsDoNotProduceAbsurdCounts(t *testing.T) {
	p := For("merge_duplicate_transactions", json.RawMessage(`{}`))
	if got := p.Writes[0].Count; got != 0 {
		t.Fatalf("count = %d, want unknown count", got)
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
		"create_rule", "create_workflow",
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

func TestDismissFlagsDoesNotClaimSingleFlagOrReversibility(t *testing.T) {
	p := For("dismiss_flagged_activity", json.RawMessage(`{}`))
	if p.Reversible {
		t.Fatal("dismissed local flags are not reversible through Activity")
	}
	if len(p.Writes) != 1 || p.Writes[0].Count != 0 || strings.Contains(p.Writes[0].Line(), "1 flag") {
		t.Fatalf("clearing all flags must have an unknown count: %+v", p.Writes)
	}
	filtered := For("dismiss_flagged_activity", json.RawMessage(`{"match":"balance"}`))
	if !strings.Contains(filtered.Writes[0].Line(), `matching "balance"`) {
		t.Fatalf("filtered dismissal scope missing: %q", filtered.Writes[0].Line())
	}
}

func TestAuthoringPermissionsDescribeCreatedItems(t *testing.T) {
	rule := For("create_rule", json.RawMessage(`{"match":"coffee","category":"Dining"}`))
	if !rule.Known || rule.ReadOnly() || !strings.Contains(rule.Writes[0].Line(), `matching "coffee"`) {
		t.Fatalf("rule permission = %+v", rule)
	}
	workflow := For("create_workflow", json.RawMessage(`{"name":"Bill reminder"}`))
	if !workflow.Known || workflow.ReadOnly() || !strings.Contains(workflow.Writes[0].Line(), `named "Bill reminder"`) {
		t.Fatalf("workflow permission = %+v", workflow)
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

func TestCapabilitiesListEveryWriteToolExactlyOnce(t *testing.T) {
	// A hand-maintained "what the AI can do" page is a promise that quietly stops
	// being true. This one is generated from the same table the approval cards
	// use, so adding a tool changes the page automatically.
	caps := Capabilities()
	if len(caps) != len(rules) {
		t.Fatalf("capabilities = %d, table has %d entries", len(caps), len(rules))
	}
	seen := map[string]bool{}
	for _, c := range caps {
		if seen[c.Tool] {
			t.Errorf("%s listed twice", c.Tool)
		}
		seen[c.Tool] = true
		if len(c.Writes) == 0 {
			t.Errorf("%s appears on the capability sheet with nothing it can change", c.Tool)
		}
	}
}

func TestCapabilitiesAreOrderedSoTheSheetDoesNotReshuffle(t *testing.T) {
	first := Capabilities()
	for i := 0; i < 5; i++ {
		again := Capabilities()
		for j := range first {
			if again[j].Tool != first[j].Tool {
				t.Fatalf("order changed between calls at %d: %q then %q", j, first[j].Tool, again[j].Tool)
			}
		}
	}
}

func TestThePermanentListIsShortEnoughToActuallyRead(t *testing.T) {
	// Burying the irreversible actions in a list of thirteen is the same as not
	// disclosing them.
	perm := PermanentCapabilities()
	if len(perm) == 0 {
		t.Fatal("nothing is listed as permanent, but merges and deletes are")
	}
	if len(perm) >= len(Capabilities()) {
		t.Fatalf("%d of %d capabilities are permanent — the list is not a short one", len(perm), len(Capabilities()))
	}
	for _, c := range perm {
		if c.Reversible {
			t.Errorf("%s is on the permanent list but reports as reversible", c.Tool)
		}
	}
}

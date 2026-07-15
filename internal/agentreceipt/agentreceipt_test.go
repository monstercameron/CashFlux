// SPDX-License-Identifier: MIT

package agentreceipt

import (
	"strings"
	"testing"
)

func TestActionPhrasesOrderAndPlural(t *testing.T) {
	tally := NewTally()
	tally.AddKinds([]string{"categorize_transactions", "categorize_transactions", "categorize_transactions"})
	tally.AddKinds([]string{"create_category"})

	got := tally.ActionPhrases()
	want := []string{"3 transactions categorized", "1 category created"}
	if len(got) != len(want) {
		t.Fatalf("phrases = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("phrase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if tally.TotalActions() != 4 {
		t.Fatalf("TotalActions = %d, want 4", tally.TotalActions())
	}
}

func TestUnknownKindFallback(t *testing.T) {
	tally := NewTally()
	tally.AddKinds([]string{"some_new_op"})
	got := tally.ActionPhrases()
	if len(got) != 1 || got[0] != "1 change applied" {
		t.Fatalf("fallback phrase = %v, want [1 change applied]", got)
	}
}

func TestCostPhraseAndSummary(t *testing.T) {
	tally := NewTally()
	tally.AddKinds([]string{"add_transaction", "add_transaction"})
	tally.AddCost(800, 0.021, true)
	tally.AddCost(440, 0.019, true)

	if cp := tally.CostPhrase(); cp != "~$0.04, 1,240 tokens" {
		t.Fatalf("CostPhrase = %q, want %q", cp, "~$0.04, 1,240 tokens")
	}
	sum := tally.Summary()
	if !strings.HasPrefix(sum, "This chat: 2 transactions recorded") {
		t.Fatalf("Summary = %q", sum)
	}
	if !strings.Contains(sum, "· ~$0.04, 1,240 tokens") {
		t.Fatalf("Summary missing cost: %q", sum)
	}
}

func TestEmptyTally(t *testing.T) {
	tally := NewTally()
	if tally.Summary() != "" {
		t.Fatalf("empty Summary = %q, want \"\"", tally.Summary())
	}
	if tally.CostPhrase() != "" {
		t.Fatalf("empty CostPhrase = %q, want \"\"", tally.CostPhrase())
	}
}

func TestTokensOnlyNoCost(t *testing.T) {
	tally := NewTally()
	tally.AddCost(1500000, 0, false)
	if cp := tally.CostPhrase(); cp != "1,500,000 tokens" {
		t.Fatalf("CostPhrase = %q, want %q", cp, "1,500,000 tokens")
	}
}

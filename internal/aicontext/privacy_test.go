// SPDX-License-Identifier: MIT

package aicontext

import (
	"strings"
	"testing"
)

func privacyInputs() Inputs {
	return Inputs{
		PeriodLabel: "July 2026",
		NetWorth:    "$100,000",
		Income:      "$5,000",
		Expense:     "$3,000",
		Accounts:    []Account{{Name: "Checking", Type: "checking", Balance: "$2,000"}},
		Formulas:    []Formula{{Name: "savings_rate", Value: "40%"}},
		TopCategories: []Bucket{
			{Label: "Groceries", Amount: "$400"},
			{Label: "Rent", Amount: "$1,500"},
		},
		TopPayees: []Bucket{
			{Label: "Whole Foods", Amount: "$400"},
			{Label: "Landlord LLC", Amount: "$1,500"},
		},
		RecentTxns: []Txn{
			{Date: "Jul 3", Desc: "Whole Foods Market", Amount: "-$88.20", Category: "Groceries"},
		},
	}
}

func TestParseConversationTier(t *testing.T) {
	cases := map[string]ConversationTier{
		"aggregates-only": TierAggregatesOnly,
		"AGGREGATES-ONLY": TierAggregatesOnly,
		" aggregates-only ": TierAggregatesOnly,
		"full":            TierFull,
		"":                TierFull,
		"nonsense":        TierFull,
	}
	for in, want := range cases {
		if got := ParseConversationTier(in); got != want {
			t.Errorf("ParseConversationTier(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactFullIsPassthrough(t *testing.T) {
	in := privacyInputs()
	out := Redact(in, TierFull)
	if len(out.RecentTxns) != 1 || len(out.TopPayees) != 2 {
		t.Fatalf("TierFull must pass inputs through unchanged, got txns=%d payees=%d", len(out.RecentTxns), len(out.TopPayees))
	}
}

// TestRedactAggregatesOnlyExcludesTxnAndPayee is the AG17 guarantee: after
// redaction, the serialized prompt contains no transaction- or payee-level string.
func TestRedactAggregatesOnlyExcludesTxnAndPayee(t *testing.T) {
	in := privacyInputs()
	out := Redact(in, TierAggregatesOnly)

	if len(out.RecentTxns) != 0 {
		t.Errorf("aggregates-only must drop RecentTxns, got %d", len(out.RecentTxns))
	}
	if len(out.TopPayees) != 0 {
		t.Errorf("aggregates-only must drop TopPayees, got %d", len(out.TopPayees))
	}

	// Build the richest context and render it; no payee/txn string may survive.
	prompt := Build(out, Opts{Tier: TierTransactions}).Prompt()
	forbidden := []string{"Whole Foods", "Landlord LLC", "Whole Foods Market", "-$88.20"}
	for _, s := range forbidden {
		if strings.Contains(prompt, s) {
			t.Errorf("aggregates-only prompt leaked %q:\n%s", s, prompt)
		}
	}

	// The aggregate signal must survive: KPIs and category totals remain.
	for _, s := range []string{"savings_rate", "Groceries", "$100,000"} {
		if !strings.Contains(prompt, s) {
			t.Errorf("aggregates-only prompt dropped aggregate %q:\n%s", s, prompt)
		}
	}
}

func TestToolAllowed(t *testing.T) {
	// Full tier allows everything.
	for name := range DetailToolNames {
		if !ToolAllowed(name, TierFull) {
			t.Errorf("TierFull should allow %q", name)
		}
	}
	// Aggregates-only withholds detail tools, allows aggregate/action tools.
	if ToolAllowed("list_transactions", TierAggregatesOnly) {
		t.Error("aggregates-only must withhold list_transactions")
	}
	if !ToolAllowed("financial_summary", TierAggregatesOnly) {
		t.Error("aggregates-only must still allow financial_summary")
	}
	if !ToolAllowed("spending_by_category", TierAggregatesOnly) {
		t.Error("aggregates-only must still allow spending_by_category (category totals)")
	}
}

func TestConversationTierLabel(t *testing.T) {
	if TierFull.Label() == "" || TierAggregatesOnly.Label() == "" {
		t.Fatal("tiers must have non-empty labels")
	}
	if TierFull.Label() == TierAggregatesOnly.Label() {
		t.Error("tier labels must be distinct")
	}
}

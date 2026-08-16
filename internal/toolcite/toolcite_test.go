// SPDX-License-Identifier: MIT

package toolcite

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestForNamesTheToolAndItsScope(t *testing.T) {
	s := For("spending_by_category", json.RawMessage(`{"month":"2026-08"}`))
	if s.Label != "Spending by category" {
		t.Fatalf("label = %q", s.Label)
	}
	if s.Scope != "2026-08" {
		t.Fatalf("scope = %q", s.Scope)
	}
	if got := s.Title(); got != "Spending by category · 2026-08" {
		t.Fatalf("title = %q", got)
	}
}

func TestScopeJoinsSuppliedArgumentsInReadingOrder(t *testing.T) {
	s := For("list_transactions", json.RawMessage(`{"category":"Groceries","match":"trader","limit":20}`))
	if s.Scope != "trader · Groceries · 20" {
		t.Fatalf("scope = %q, want match then category then limit", s.Scope)
	}
}

func TestScopeSkipsArgumentsTheCallDidNotSupply(t *testing.T) {
	s := For("list_transactions", json.RawMessage(`{"match":"coffee"}`))
	if s.Scope != "coffee" {
		t.Fatalf("scope = %q, want just the one supplied argument", s.Scope)
	}
	if strings.Contains(s.Title(), "··") {
		t.Fatalf("empty arguments left a gap in the title: %q", s.Title())
	}
}

func TestAToolWithNoScopeArgumentsTitlesCleanly(t *testing.T) {
	s := For("account_balances", json.RawMessage(`{}`))
	if s.Scope != "" {
		t.Fatalf("scope = %q, want empty", s.Scope)
	}
	if got := s.Title(); got != "Account balances" {
		t.Fatalf("title = %q", got)
	}
}

func TestAnUntabledToolStillCites(t *testing.T) {
	s := For("some_new_tool", json.RawMessage(`{}`))
	if s.Label == "" {
		t.Fatal("an untabled tool produced no label, so it would vanish from the citation list")
	}
	if s.Label != "Some new tool" {
		t.Fatalf("label = %q", s.Label)
	}
}

func TestMalformedArgumentsLoseTheScopeNotTheCitation(t *testing.T) {
	s := For("spending_by_category", json.RawMessage(`{"month":`))
	if s.Label != "Spending by category" {
		t.Fatalf("label = %q", s.Label)
	}
	if s.Scope != "" {
		t.Fatalf("scope = %q, want empty when the arguments could not be read", s.Scope)
	}
}

func TestNumbersInScopePrintWithoutFloatNoise(t *testing.T) {
	s := For("check_affordability", json.RawMessage(`{"amount":1200,"item":"a laptop"}`))
	if !strings.Contains(s.Scope, "1200") || strings.Contains(s.Scope, ".00") {
		t.Fatalf("scope = %q", s.Scope)
	}
	frac := For("check_affordability", json.RawMessage(`{"amount":12.5}`))
	if frac.Scope != "12.5" {
		t.Fatalf("fractional scope = %q", frac.Scope)
	}
}

func TestNumericSpotsAClaimWorthChecking(t *testing.T) {
	for _, tc := range []struct {
		reply string
		want  bool
	}{
		{"You spent $312 on groceries.", true},
		{"Your budgets all look fine — nothing over.", false},
		{"", false},
		{"Try setting one up under Budgets.", false},
	} {
		if got := Numeric(tc.reply); got != tc.want {
			t.Errorf("Numeric(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}

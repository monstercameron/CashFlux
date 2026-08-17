// SPDX-License-Identifier: MIT

package entitysearch

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var day = time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)

func txn(id, payee, desc string) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, Desc: desc, Date: day,
		Amount: money.New(-2500, "USD")}
}

func kinds(hits []Hit) []Kind {
	out := make([]Kind, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Kind)
	}
	return out
}

// Places and plans before events. There are a handful of accounts and thousands
// of transactions, so ranking by score alone buries every account under a wall
// of matching ledger rows.
func TestResultsAreOrderedByKindNotByScore(t *testing.T) {
	in := Input{
		Accounts:     []domain.Account{{ID: "a1", Name: "Roof fund", Type: domain.TypeSavings}},
		Budgets:      []domain.Budget{{ID: "b1", Name: "Roof repairs"}},
		Goals:        []domain.Goal{{ID: "g1", Name: "New roof"}},
		Tasks:        []domain.Task{{ID: "t1", Title: "Call the roofer", Status: domain.StatusOpen}},
		Transactions: []domain.Transaction{txn("x1", "Roofing Co", ""), txn("x2", "", "roof tiles")},
	}
	got := kinds(Search("roof", in))
	want := []Kind{KindAccount, KindBudget, KindGoal, KindTask, KindTransaction, KindTransaction}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// A name that STARTS with what you typed is almost always the one you meant.
func TestEarlierMatchesRankHigherWithinAKind(t *testing.T) {
	in := Input{Accounts: []domain.Account{
		{ID: "a1", Name: "Emergency roof fund"},
		{ID: "a2", Name: "Roof fund"},
	}}
	got := Search("roof", in)
	if len(got) != 2 {
		t.Fatalf("got %d hits", len(got))
	}
	if got[0].ID != "a2" {
		t.Errorf("first hit = %q, want the one starting with the query", got[0].ID)
	}
}

// Two characters over a full ledger matches almost everything, which is the same
// experience as matching nothing except slower.
func TestVeryShortQueriesReturnNothing(t *testing.T) {
	in := Input{Accounts: []domain.Account{{ID: "a", Name: "Checking"}}}
	if got := Search("c", in); got != nil {
		t.Errorf("a one-character query returned %d hits", len(got))
	}
	if got := Search("  ", in); got != nil {
		t.Errorf("a whitespace query returned %d hits", len(got))
	}
	// MinQuery characters is enough.
	if got := Search("ch", in); len(got) != 1 {
		t.Errorf("a two-character query returned %d hits, want 1", len(got))
	}
}

// A result that navigates without applying a filter has moved the reader to a
// haystack and called it an answer.
func TestTransactionHitsCarryTheFilterText(t *testing.T) {
	in := Input{Transactions: []domain.Transaction{txn("x1", "Greenfield Market", "")}}
	got := Search("greenfield", in)
	if len(got) != 1 {
		t.Fatalf("got %d hits", len(got))
	}
	if got[0].Route != "/transactions" {
		t.Errorf("Route = %q", got[0].Route)
	}
	if got[0].Query != "greenfield" {
		t.Errorf("Query = %q, want the search text so the ledger lands filtered", got[0].Query)
	}
	// Non-transaction hits do not carry a query — they navigate to a page that
	// shows the entity directly.
	acct := Search("check", Input{Accounts: []domain.Account{{ID: "a", Name: "Checking"}}})
	if acct[0].Query != "" {
		t.Errorf("an account hit carried a filter query: %q", acct[0].Query)
	}
}

// A description match on the same row must not produce a second hit for one
// transaction.
func TestOneTransactionProducesAtMostOneHit(t *testing.T) {
	in := Input{Transactions: []domain.Transaction{txn("x1", "Roof Co", "roof repair")}}
	if got := Search("roof", in); len(got) != 1 {
		t.Errorf("got %d hits for one transaction", len(got))
	}
}

// A transaction with no payee still matches on its description, and titles
// itself with what it has.
func TestDescriptionOnlyTransactions(t *testing.T) {
	in := Input{Transactions: []domain.Transaction{txn("x1", "", "annual roof inspection")}}
	got := Search("roof", in)
	if len(got) != 1 {
		t.Fatalf("got %d hits", len(got))
	}
	if got[0].Title != "annual roof inspection" {
		t.Errorf("Title = %q, want the description as a fallback", got[0].Title)
	}
}

// Archived and completed things are not places the reader can act, and
// surfacing them makes every result need a liveness check.
func TestArchivedAndDoneAreExcluded(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{{ID: "a", Name: "Old roof fund", Archived: true}},
		Goals:    []domain.Goal{{ID: "g", Name: "Roof", Archived: true}},
		Tasks: []domain.Task{
			{ID: "t1", Title: "Roof done", Status: domain.StatusDone},
			{ID: "t2", Title: "Roof pending", Status: domain.StatusOpen},
		},
	}
	got := Search("roof", in)
	if len(got) != 1 || got[0].ID != "t2" {
		t.Errorf("got %v, want only the open task", got)
	}
}

func TestCaseAndWhitespaceAreIgnored(t *testing.T) {
	in := Input{Accounts: []domain.Account{{ID: "a", Name: "  Checking Account  "}}}
	if got := Search("  CHECKING ", in); len(got) != 1 {
		t.Errorf("case/whitespace-insensitive match failed: %d hits", len(got))
	}
}

// A palette shows what fits on a screen; beyond that the answer is "narrow it".
func TestResultsAreLimited(t *testing.T) {
	var txns []domain.Transaction
	for i := range 100 {
		txns = append(txns, txn("x"+string(rune('a'+i%26))+string(rune('a'+i/26)), "Roof Co", ""))
	}
	if got := Search("roof", Input{Transactions: txns}); len(got) != DefaultLimit {
		t.Errorf("got %d hits, want the default limit %d", len(got), DefaultLimit)
	}
	if got := Search("roof", Input{Transactions: txns, Limit: 5}); len(got) != 5 {
		t.Errorf("got %d hits with Limit 5", len(got))
	}
}

// The package must not decide how money is displayed.
func TestAmountFormattingIsTheCallersJob(t *testing.T) {
	in := Input{Transactions: []domain.Transaction{txn("x1", "Roof Co", "")}}
	plain := Search("roof", in)
	if plain[0].Subtitle != "Aug 16, 2026" {
		t.Errorf("Subtitle without a formatter = %q, want the date alone", plain[0].Subtitle)
	}
	in.FormatAmount = func(domain.Transaction) string { return "-$25.00" }
	withAmt := Search("roof", in)
	if withAmt[0].Subtitle != "Aug 16, 2026 · -$25.00" {
		t.Errorf("Subtitle with a formatter = %q", withAmt[0].Subtitle)
	}
}

func TestCountByKind(t *testing.T) {
	in := Input{
		Accounts:     []domain.Account{{ID: "a", Name: "Roof fund"}},
		Transactions: []domain.Transaction{txn("x1", "Roof Co", ""), txn("x2", "Roofers", "")},
	}
	got := CountByKind(Search("roof", in))
	if got[KindAccount] != 1 || got[KindTransaction] != 2 {
		t.Errorf("CountByKind = %v", got)
	}
	if got[KindGoal] != 0 {
		t.Errorf("an absent kind counted %d", got[KindGoal])
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Search("anything", Input{}); got != nil {
		t.Errorf("empty input returned %d hits", len(got))
	}
}

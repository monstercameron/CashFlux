// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/store"
)

func clearInput(now time.Time, accounts []domain.Account, txns []domain.Transaction) Input {
	return Input{
		Now: now, Base: "USD", Rates: currency.Rates{Base: "USD"},
		Accounts: accounts, Transactions: txns,
	}
}

// A cleared history of n charges taking `days` each, so the account has a normal.
func clearedHistory(acct string, now time.Time, n, days int) []domain.Transaction {
	out := make([]domain.Transaction, 0, n)
	for i := range n {
		d := now.AddDate(0, 0, -(i + 20))
		out = append(out, domain.Transaction{
			ID: acct + "-h" + string(rune('a'+i)), AccountID: acct, Date: d,
			Payee: "Shop", Amount: domain.Transaction{}.Amount,
		}.MarkCleared(true, d.AddDate(0, 0, days)))
	}
	return out
}

func TestT23FlagsACharePastThisAccountsOwnNormal(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	accts := []domain.Account{{ID: "fast", Name: "Everyday", Class: domain.ClassAsset}}
	txns := clearedHistory("fast", now, 10, 1)
	stuck := domain.Transaction{ID: "stuck", AccountID: "fast", Date: now.AddDate(0, 0, -9), Payee: "Corner Shop"}
	txns = append(txns, stuck)

	got := t23StaleUncleared(clearInput(now, accts, txns))
	if len(got) != 1 {
		t.Fatalf("insights = %d, want 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Detail, "Corner Shop") {
		t.Errorf("detail did not name the charge: %q", got[0].Detail)
	}
	// The evidence has to be in the sentence, or "unusual for you" is unarguable.
	if !strings.Contains(got[0].Detail, "usually clear in 1 day") {
		t.Errorf("detail did not state the account's own window: %q", got[0].Detail)
	}
	if !strings.Contains(got[0].Detail, "last 10 charges") {
		t.Errorf("detail did not say how much history it rests on: %q", got[0].Detail)
	}
}

// A single "over 5 days" rule would nag about every credit-card charge. The
// window is the account's own.
func TestT23StaysQuietWhenTheAccountIsJustSlow(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	accts := []domain.Account{{ID: "slow", Name: "Rewards Card", Class: domain.ClassLiability}}
	txns := clearedHistory("slow", now, 12, 6)
	txns = append(txns, domain.Transaction{
		ID: "recent", AccountID: "slow", Date: now.AddDate(0, 0, -7), Payee: "Target"})
	if got := t23StaleUncleared(clearInput(now, accts, txns)); len(got) != 0 {
		t.Errorf("a 7-day-old charge on a 6-day account was flagged: %+v", got)
	}
}

// An account somebody started using last week has no normal to be late against.
func TestT23NeedsAHistoryFirst(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	accts := []domain.Account{{ID: "new", Name: "New Account", Class: domain.ClassAsset}}
	txns := append(clearedHistory("new", now, 3, 1), domain.Transaction{
		ID: "old", AccountID: "new", Date: now.AddDate(0, 0, -60), Payee: "Whoever"})
	if got := t23StaleUncleared(clearInput(now, accts, txns)); len(got) != 0 {
		t.Errorf("an account with three cleared charges produced a verdict: %+v", got)
	}
}

// Old rows carry the flag and no stamp. Treating those as same-day clearings
// would teach every account a window of nothing and flag everything.
func TestT23IgnoresClearedRowsWithNoRecordedMoment(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	accts := []domain.Account{{ID: "legacy", Name: "Old Account", Class: domain.ClassAsset}}
	var txns []domain.Transaction
	for i := range 20 {
		txns = append(txns, domain.Transaction{
			ID: "legacy-" + string(rune('a'+i)), AccountID: "legacy",
			Date: now.AddDate(0, 0, -(i + 10)), Cleared: true, // no ClearedAt
		})
	}
	txns = append(txns, domain.Transaction{
		ID: "pending", AccountID: "legacy", Date: now.AddDate(0, 0, -40), Payee: "Someone"})
	if got := t23StaleUncleared(clearInput(now, accts, txns)); len(got) != 0 {
		t.Errorf("legacy rows with no stamp were treated as evidence: %+v", got)
	}
}

// Eight clearing warnings would push everything else off a strip that shows
// three, at which point the reader stops reading the strip.
func TestT23ReportsOneFindingAndCountsTheRest(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	var accts []domain.Account
	var txns []domain.Transaction
	for _, id := range []string{"a", "b", "c"} {
		accts = append(accts, domain.Account{ID: id, Name: "Account " + id, Class: domain.ClassAsset})
		txns = append(txns, clearedHistory(id, now, 10, 1)...)
		txns = append(txns, domain.Transaction{
			ID: id + "-stuck", AccountID: id, Date: now.AddDate(0, 0, -9), Payee: "Shop " + id})
	}
	got := t23StaleUncleared(clearInput(now, accts, txns))
	if len(got) != 1 {
		t.Fatalf("insights = %d, want exactly 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Detail, "2 other accounts also have one") {
		t.Errorf("the others were not counted: %q", got[0].Detail)
	}
}

// The demo data has to exercise this, or the feature reads as unbuilt.
func TestT23FiresOnTheSampleDataset(t *testing.T) {
	ds := store.SampleDataset()
	got := t23StaleUncleared(Input{
		Now: time.Now(), Base: "USD", Rates: currency.Rates{Base: "USD"},
		Accounts: ds.Accounts, Transactions: ds.Transactions,
	})
	if len(got) != 1 {
		t.Fatalf("sample produced %d findings, want exactly 1: %+v", len(got), got)
	}
	if got[0].Feature != "SMART-T23" || got[0].Page != smart.PageTransactions {
		t.Errorf("unexpected finding: %+v", got[0])
	}
	if !strings.Contains(got[0].Detail, "usually clear in") {
		t.Errorf("the sample finding states no learned window: %q", got[0].Detail)
	}
}

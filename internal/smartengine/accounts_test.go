// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// ref is the fixed reference clock for engine tests.
var ref = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func usd(minor int64) money.Money { return money.New(minor, "USD") }

// baseInput returns an Input with USD base and an identity rate table.
func baseInput() Input {
	return Input{Now: ref, Base: "USD", Rates: currency.Rates{Base: "USD"}}
}

// enableAll returns Settings opting into the given feature codes.
func enable(codes ...string) smart.Settings {
	var s smart.Settings
	for _, c := range codes {
		s = s.SetEnabled(c, true)
	}
	return s
}

func acct(id, name string, typ domain.AccountType, opening int64, asOf time.Time) domain.Account {
	return domain.Account{
		ID: id, Name: name, Type: typ, Class: typ.Class(), Currency: "USD",
		OpeningBalance: usd(opening), BalanceAsOf: asOf,
	}
}

func txn(id, account string, when time.Time, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: account, Date: when, Amount: usd(amount), Desc: "x"}
}

func findInsight(ins []smart.Insight, feature string) (smart.Insight, bool) {
	for _, i := range ins {
		if i.Feature == feature {
			return i, true
		}
	}
	return smart.Insight{}, false
}

func TestA2DormantAccount(t *testing.T) {
	in := baseInput()
	old := ref.AddDate(0, -10, 0)
	in.Accounts = []domain.Account{
		acct("a1", "Old Savings", domain.TypeSavings, 200000, old), // dormant, $2000
		acct("a2", "Active Checking", domain.TypeChecking, 50000, old),
	}
	in.Transactions = []domain.Transaction{
		txn("t1", "a2", ref.AddDate(0, 0, -3), -2000), // a2 has recent activity
	}
	got := a2DormantAccount(in)
	if len(got) != 1 {
		t.Fatalf("want 1 dormant insight, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-A2:a1" {
		t.Errorf("wrong account flagged: %s", got[0].Key)
	}
	if !got[0].HasAmount || got[0].Amount.Amount != 200000 {
		t.Errorf("expected balance amount, got %+v", got[0].Amount)
	}
	if got[0].Action == nil || got[0].Action.RelatedID != "a1" {
		t.Errorf("expected navigate action to a1")
	}
}

func TestA2SkipsRecentAndEmpty(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{
		acct("a1", "Recent", domain.TypeSavings, 200000, ref.AddDate(0, 0, -5)), // recent asOf
		acct("a2", "Empty", domain.TypeSavings, 100, ref.AddDate(0, -10, 0)),    // below min balance
	}
	if got := a2DormantAccount(in); len(got) != 0 {
		t.Errorf("want 0, got %d: %+v", len(got), got)
	}
}

func TestA4CashPositioning(t *testing.T) {
	in := baseInput()
	low := acct("low", "Big Checking", domain.TypeChecking, 500000, ref) // $5000 @ 0%
	high := acct("high", "HY Savings", domain.TypeSavings, 100000, ref)
	high.ExpectedReturnAPR = 4.5
	in.Accounts = []domain.Account{low, high}
	got := a4CashPositioning(in)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d: %+v", len(got), got)
	}
	if got[0].Key != "SMART-A4:low" {
		t.Errorf("flagged wrong account: %s", got[0].Key)
	}
	// Gain = $5000 * 4.5% = $225/yr.
	if got[0].Amount.Amount != 22500 {
		t.Errorf("gain = %d, want 22500", got[0].Amount.Amount)
	}
}

func TestA4NoYieldAnywhere(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{
		acct("a", "Checking", domain.TypeChecking, 500000, ref),
		acct("b", "Savings", domain.TypeSavings, 500000, ref),
	}
	if got := a4CashPositioning(in); len(got) != 0 {
		t.Errorf("no APR set anywhere — want 0, got %d", len(got))
	}
}

func TestA1BalanceAnomaly(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 1000000, ref)}
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	in.Transactions = []domain.Transaction{
		// trailing months ~$200 each
		txn("p1", "a", monthStart.AddDate(0, -1, 9), -20000),
		txn("p2", "a", monthStart.AddDate(0, -2, 9), -20000),
		txn("p3", "a", monthStart.AddDate(0, -3, 9), -20000),
		// current month $700 — 3.5× the baseline
		txn("c1", "a", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), -70000),
	}
	got := a1BalanceAnomaly(in)
	if len(got) != 1 {
		t.Fatalf("want 1 anomaly, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityWarn {
		t.Errorf("anomaly should warn, got %v", got[0].Severity)
	}
}

func TestA1NoAnomalyWhenSteady(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 1000000, ref)}
	monthStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	in.Transactions = []domain.Transaction{
		txn("p1", "a", monthStart.AddDate(0, -1, 9), -20000),
		txn("p2", "a", monthStart.AddDate(0, -2, 9), -20000),
		txn("c1", "a", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), -22000), // normal
	}
	if got := a1BalanceAnomaly(in); len(got) != 0 {
		t.Errorf("steady spend — want 0, got %d: %+v", len(got), got)
	}
}

func TestA8OverdraftForecast(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 10000, ref)} // $100
	in.Recurring = []domain.Recurring{{
		ID: "r1", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly,
		NextDue: ref.AddDate(0, 0, 10), AccountID: "a",
	}}
	got := a8OverdraftForecast(in)
	if len(got) != 1 {
		t.Fatalf("want 1 overdraft warning, got %d: %+v", len(got), got)
	}
	if got[0].Severity != smart.SeverityAlert {
		t.Errorf("overdraft should alert, got %v", got[0].Severity)
	}
	if !got[0].HasAmount || got[0].Amount.Amount <= 0 {
		t.Errorf("expected positive shortfall amount, got %+v", got[0].Amount)
	}
}

func TestA8NoBreachNoInsight(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 1000000, ref)}
	in.Recurring = []domain.Recurring{{
		ID: "r1", Label: "Rent", Amount: usd(-50000), Cadence: domain.CadenceMonthly,
		NextDue: ref.AddDate(0, 0, 10), AccountID: "a",
	}}
	if got := a8OverdraftForecast(in); len(got) != 0 {
		t.Errorf("healthy balance — want 0, got %d", len(got))
	}
}

func TestA7RecurringCharges(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("a", "Checking", domain.TypeChecking, 1000000, ref)}
	var txns []domain.Transaction
	// Two monthly recurring charges, 3 occurrences each.
	for i := range 3 {
		d1 := time.Date(2026, time.Month(4+i), 5, 0, 0, 0, 0, time.UTC)
		d2 := time.Date(2026, time.Month(4+i), 12, 0, 0, 0, 0, time.UTC)
		txns = append(txns,
			domain.Transaction{ID: "n" + itoa64(int64(i)), AccountID: "a", Date: d1, Amount: usd(-1599), Desc: "Netflix"},
			domain.Transaction{ID: "g" + itoa64(int64(i)), AccountID: "a", Date: d2, Amount: usd(-3000), Desc: "Gym"},
		)
	}
	in.Transactions = txns
	got := a7RecurringCharges(in)
	if len(got) != 1 {
		t.Fatalf("want 1 summary insight, got %d: %+v", len(got), got)
	}
	if !got[0].HasAmount || got[0].Amount.Amount <= 0 {
		t.Errorf("expected monthly total amount, got %+v", got[0].Amount)
	}
}

func TestRunDispatchAndGating(t *testing.T) {
	in := baseInput()
	old := ref.AddDate(0, -10, 0)
	in.Accounts = []domain.Account{acct("a1", "Old Savings", domain.TypeSavings, 200000, old)}

	// Nothing enabled → no insights, even though the data would trigger A2.
	if got := Run(in, smart.Settings{}); len(got) != 0 {
		t.Errorf("opt-out default should yield nothing, got %d", len(got))
	}
	// Enable A2 → the dormant insight surfaces.
	got := Run(in, enable("SMART-A2"))
	if _, ok := findInsight(got, "SMART-A2"); !ok {
		t.Errorf("A2 enabled but not surfaced: %+v", got)
	}
	// Dismissing it hides it on the next run.
	s := enable("SMART-A2").Dismiss("SMART-A2:a1")
	if got := Run(in, s); len(got) != 0 {
		t.Errorf("dismissed insight still shown: %+v", got)
	}
}

func TestRunPageScoping(t *testing.T) {
	in := baseInput()
	old := ref.AddDate(0, -10, 0)
	in.Accounts = []domain.Account{acct("a1", "Old Savings", domain.TypeSavings, 200000, old)}
	s := enable("SMART-A2")
	if got := RunPage(in, s, smart.PageBills); len(got) != 0 {
		t.Errorf("A2 is an Accounts feature; Bills page should be empty, got %d", len(got))
	}
	if got := RunPage(in, s, smart.PageAccounts); len(got) == 0 {
		t.Errorf("Accounts page should surface the A2 insight")
	}
}

func TestImplementedCodesRegistered(t *testing.T) {
	for _, c := range []string{"SMART-A1", "SMART-A2", "SMART-A4", "SMART-A7", "SMART-A8"} {
		if !HasEngine(c) {
			t.Errorf("engine not registered: %s", c)
		}
	}
	// Implemented codes must all be Free features in the catalog.
	for _, c := range ImplementedCodes() {
		f, ok := smart.ByCode(c)
		if !ok || f.Tier != smart.TierFree {
			t.Errorf("implemented code %s is not a Free catalog feature", c)
		}
	}
}

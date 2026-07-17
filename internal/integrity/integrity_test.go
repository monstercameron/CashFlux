// SPDX-License-Identifier: MIT

package integrity

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

// cleanInput builds a dataset in which every check passes: a paired transfer,
// a reconciling split, a liability holding debt, an exact reconciliation, a
// sane budget, and a part-funded financial goal.
func cleanInput() Input {
	chk := domain.Account{ID: "chk", Name: "Checking", Currency: "USD",
		Type: domain.TypeChecking, Class: domain.ClassAsset, OpeningBalance: money.New(10000, "USD")}
	card := domain.Account{ID: "card", Name: "Card", Currency: "USD",
		Type: domain.TypeCreditCard, Class: domain.ClassLiability, OpeningBalance: money.New(-5000, "USD")}
	chk.Reconciliations = []domain.Reconciliation{{
		At: d("2026-07-10"), StatementDate: d("2026-07-10"), StatementBalance: money.New(10000 - 2500, "USD"),
	}}
	return Input{
		Accounts: []domain.Account{chk, card},
		Transactions: []domain.Transaction{
			{ID: "t1", AccountID: "chk", Date: d("2026-07-01"), Desc: "groceries",
				Amount: money.New(-2500, "USD"), Cleared: true},
			{ID: "x-out", AccountID: "chk", Date: d("2026-07-05"), Desc: "to card",
				Amount: money.New(-1000, "USD"), TransferAccountID: "card"},
			{ID: "x-in", AccountID: "card", Date: d("2026-07-05"), Desc: "from checking",
				Amount: money.New(1000, "USD"), TransferAccountID: "chk"},
			{ID: "t2", AccountID: "chk", Date: d("2026-07-12"), Desc: "split shop",
				Amount: money.New(-3000, "USD"),
				Splits: []domain.CategorySplit{
					{CategoryID: "a", Amount: money.New(-2000, "USD")},
					{CategoryID: "b", Amount: money.New(-1000, "USD")},
				}},
		},
		Budgets: []domain.Budget{{ID: "b1", Name: "Food", Limit: money.New(40000, "USD")}},
		Goals: []domain.Goal{{ID: "g1", Name: "Trip", Kind: domain.GoalKindFinancial,
			TargetAmount: money.New(100000, "USD"), CurrentAmount: money.New(40000, "USD")}},
	}
}

func findingChecks(fs []Finding) map[Check]int {
	out := map[Check]int{}
	for _, f := range fs {
		out[f.Check]++
	}
	return out
}

func TestCleanDatasetHasNoFindings(t *testing.T) {
	if fs := Run(cleanInput()); len(fs) != 0 {
		t.Fatalf("clean dataset produced findings: %+v", fs)
	}
}

func TestSeededCorruptions(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(*Input)
		want    Check
		entity  string
	}{
		{"orphaned transfer leg", func(in *Input) {
			in.Transactions = in.Transactions[:2] // drop x-in (index 2) and t2; keep t1 + x-out
		}, CheckTransferOrphan, "x-out"},
		{"sign-inconsistent transfer pair", func(in *Input) {
			in.Transactions[2].Amount = money.New(-1000, "USD") // both legs negative
		}, CheckTransferOrphan, ""},
		{"split no longer sums", func(in *Input) {
			in.Transactions[3].Splits[0].Amount = money.New(-1500, "USD")
		}, CheckSplitSum, "t2"},
		{"transaction in a foreign currency", func(in *Input) {
			in.Transactions[0].Amount = money.New(-2500, "EUR")
		}, CheckCurrencyMismatch, "t1"},
		{"transaction on a deleted account", func(in *Input) {
			in.Transactions[0].AccountID = "gone"
		}, CheckOrphanAccount, "t1"},
		{"liability holding a positive balance", func(in *Input) {
			in.Accounts[1].OpeningBalance = money.New(9000, "USD")
		}, CheckLiabilitySign, "card"},
		{"cleared history drifted from the reconciliation", func(in *Input) {
			in.Transactions[0].Amount = money.New(-9900, "USD") // edited after reconciling
		}, CheckReconcileDrift, "chk"},
		{"budget with a zero limit", func(in *Input) {
			in.Budgets[0].Limit = money.New(0, "USD")
		}, CheckBudgetLimit, "b1"},
		{"goal with a negative balance", func(in *Input) {
			in.Goals[0].CurrentAmount = money.New(-1, "USD")
		}, CheckGoalArithmetic, "g1"},
		{"goal funded past its target", func(in *Input) {
			in.Goals[0].CurrentAmount = money.New(150000, "USD")
		}, CheckGoalOverfunded, "g1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := cleanInput()
			tc.corrupt(&in)
			fs := Run(in)
			if got := findingChecks(fs)[tc.want]; got == 0 {
				t.Fatalf("corruption not detected; findings = %+v", fs)
			}
			if tc.entity != "" {
				found := false
				for _, f := range fs {
					if f.Check == tc.want && f.EntityID == tc.entity {
						found = true
					}
				}
				if !found {
					t.Errorf("finding for %q missing entity %q: %+v", tc.want, tc.entity, fs)
				}
			}
		})
	}
}

func TestReconcileDriftDayBoundary(t *testing.T) {
	// A cleared transaction dated the SAME calendar day as the statement — but
	// with a time of day — must count toward the reconciled sum (statement
	// dates are date-only; transaction dates can carry timestamps).
	in := cleanInput()
	in.Transactions[0].Date = time.Date(2026, 7, 10, 16, 30, 0, 0, time.UTC)
	if fs := Run(in); len(fs) != 0 {
		t.Fatalf("same-day timestamped clearing produced findings: %+v", fs)
	}
}

func TestExemptions(t *testing.T) {
	in := cleanInput()
	// A checklist goal with zero amounts is fine.
	in.Goals = append(in.Goals, domain.Goal{ID: "g2", Name: "Read more", Kind: domain.GoalKindChecklist})
	// A FORCED reconciliation recorded a known gap — no drift finding.
	in.Accounts[0].Reconciliations = []domain.Reconciliation{{
		At: d("2026-07-10"), StatementDate: d("2026-07-10"),
		StatementBalance: money.New(999, "USD"), Forced: true, DifferenceMinor: 123,
	}}
	// An archived liability with a positive balance stays quiet.
	in.Accounts[1].OpeningBalance = money.New(9000, "USD")
	in.Accounts[1].Archived = true
	if fs := Run(in); len(fs) != 0 {
		t.Fatalf("exempt cases produced findings: %+v", fs)
	}
}

func TestDeterministicOrder(t *testing.T) {
	in := cleanInput()
	in.Transactions[0].Amount = money.New(-2500, "EUR")
	in.Budgets[0].Limit = money.New(0, "USD")
	a := Run(in)
	b := Run(in)
	if len(a) != len(b) || len(a) != 2 {
		t.Fatalf("want 2 findings both runs, got %d/%d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("order not deterministic: %v vs %v", a[i].ID, b[i].ID)
		}
	}
	if a[0].Check != CheckBudgetLimit {
		t.Errorf("findings should sort by check slug; got %v first", a[0].Check)
	}
}

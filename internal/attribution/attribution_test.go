// SPDX-License-Identifier: MIT

package attribution

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	since = time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	until = time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func rates() currency.Rates { return currency.Rates{Base: "USD"} }

func asset(id, name string) domain.Account {
	return domain.Account{ID: id, Name: name, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: usd(0)}
}

func txn(id, acct string, day int, amountMinor int64, payee, cat string) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Date: time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC),
		Payee: payee, CategoryID: cat, Amount: usd(amountMinor),
	}
}

func isAdjDesc(t domain.Transaction) bool { return t.Desc == "Balance adjustment" }

func find(t *testing.T, items []Item, k Kind) Item {
	t.Helper()
	for _, it := range items {
		if it.Kind == k {
			return it
		}
	}
	t.Fatalf("no %s item in %+v", k, items)
	return Item{}
}

func TestComputeFlowAndRanking(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking")},
		Txns: []domain.Transaction{
			txn("t0", "a1", 1, -4000, "Kroger", "groceries"), // pre-window: payee known
			txn("t1", "a1", 12, 150000, "ACME Payroll", "salary"),
			txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
			txn("t3", "a1", 14, -5000, "Chevron", "gas"),
		},
		Rates: rates(), Since: since, Until: until, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if rep.NetDeltaMinor != 125000 || rep.IncomeMinor != 150000 || rep.SpendingMinor != 25000 {
		t.Fatalf("totals: %+v", rep)
	}
	if rep.OtherMinor != 0 || rep.AdjustmentsMinor != 0 {
		t.Fatalf("residual should be zero: %+v", rep)
	}
	if rep.TxnCount != 3 {
		t.Fatalf("TxnCount = %d, want 3", rep.TxnCount)
	}
	if rep.Items[0].Kind != KindNetWorth {
		t.Fatalf("headline not pinned: %v", rep.Items[0].Kind)
	}
	if got := rep.Items[0].Parts[0]; got.Kind != PartFlow || got.AmountMinor != 125000 {
		t.Fatalf("flow part: %+v", got)
	}
	// Income ($1500) outranks the account move ($1250)? No: account contribution
	// is the full +1250.00; income is 1500.00 — income ranks above account.
	if rep.Items[1].Kind != KindIncome || rep.Items[2].Kind != KindAccount {
		t.Fatalf("ranking: %v then %v", rep.Items[1].Kind, rep.Items[2].Kind)
	}
	cat := find(t, rep.Items, KindCategory)
	if cat.CategoryID != "groceries" || cat.AmountMinor != -20000 || cat.TxnIDs[0] != "t2" {
		t.Fatalf("category: %+v", cat)
	}
	// t2 qualifies as a large expense (80% ≥ 30% of spending) but cites the
	// exact evidence the category finding already told — the evidence dedupe
	// keeps only the higher-ranked category row (one issue, one finding).
	for _, it := range rep.Items {
		if it.Kind == KindLargeTxn {
			t.Fatalf("large txn should be deduped into the category finding: %+v", it)
		}
	}
	// Kroger seen pre-window; ACME is income-side (not a new-payee signal);
	// Chevron is the one genuinely new merchant paid.
	np := find(t, rep.Items, KindNewPayee)
	if np.Payee != "Chevron" || np.Count != 1 {
		t.Fatalf("new payee: %+v", np)
	}
}

func TestComputeAdjustmentSplit(t *testing.T) {
	adj := txn("adj1", "a1", 15, 50000, "", "")
	adj.Desc = "Balance adjustment"
	in := Input{
		Accounts: []domain.Account{asset("a1", "Brokerage")},
		Txns:     []domain.Transaction{adj, txn("t1", "a1", 16, -10000, "Rent Co", "rent")},
		Rates:    rates(), Since: since, Until: until, IsAdjustment: isAdjDesc, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if rep.NetDeltaMinor != 40000 || rep.AdjustmentsMinor != 50000 || rep.SpendingMinor != 10000 {
		t.Fatalf("totals: %+v", rep)
	}
	if rep.OtherMinor != 0 {
		t.Fatalf("other = %d, want 0", rep.OtherMinor)
	}
	head := rep.Items[0]
	if len(head.Parts) != 2 || head.Parts[1].Kind != PartAdjustments || head.Parts[1].AmountMinor != 50000 {
		t.Fatalf("headline parts: %+v", head.Parts)
	}
	acct := find(t, rep.Items, KindAccount)
	if acct.AccountName != "Brokerage" || acct.AmountMinor != 40000 {
		t.Fatalf("account item: %+v", acct)
	}
}

func TestComputeTransferNetsToZero(t *testing.T) {
	liab := domain.Account{ID: "cc", Name: "Card", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-100000)}
	out := txn("t1", "a1", 12, -20000, "", "")
	out.TransferAccountID = "cc"
	in2 := txn("t2", "cc", 12, 20000, "", "")
	in2.TransferAccountID = "a1"
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking"), liab},
		Txns:     []domain.Transaction{out, in2},
		Rates:    rates(), Since: since, Until: until, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	// Paying $200 of debt from checking: checking −200, debt magnitude −200 →
	// net worth unchanged, no income/spending, no residual.
	if rep.NetDeltaMinor != 0 || rep.IncomeMinor != 0 || rep.SpendingMinor != 0 || rep.OtherMinor != 0 {
		t.Fatalf("totals: %+v", rep)
	}
	if rep.TxnCount != 2 {
		t.Fatalf("TxnCount = %d", rep.TxnCount)
	}
	// Both accounts moved ±200; the account finding decomposes its own flow.
	acct := find(t, rep.Items, KindAccount)
	if abs64(acct.AmountMinor) != 20000 || len(acct.Parts) != 1 || acct.Parts[0].Kind != PartFlow {
		t.Fatalf("account item: %+v", acct)
	}
}

func TestComputeExcludedLandsInOther(t *testing.T) {
	ex := txn("t1", "a1", 12, -30000, "Insurance Payout Reversal", "")
	ex.ExcludeFromReports = true
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking")},
		Txns:     []domain.Transaction{ex},
		Rates:    rates(), Since: since, Until: until, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if rep.NetDeltaMinor != -30000 || rep.SpendingMinor != 0 || rep.OtherMinor != -30000 {
		t.Fatalf("totals: %+v", rep)
	}
	head := rep.Items[0]
	last := head.Parts[len(head.Parts)-1]
	if last.Kind != PartOther || last.AmountMinor != -30000 {
		t.Fatalf("headline parts: %+v", head.Parts)
	}
}

func TestComputeLargeTxnThreshold(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking")},
		Txns: []domain.Transaction{
			txn("t1", "a1", 12, -2000, "A", "misc"),
			txn("t2", "a1", 13, -2000, "B", "misc"),
			txn("t3", "a1", 14, -2000, "C", "misc"),
			txn("t4", "a1", 15, -2000, "D", "misc"),
			txn("t5", "a1", 16, -2000, "E", "misc"), // each 20% < 30% share
		},
		Rates: rates(), Since: since, Until: until, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range rep.Items {
		if it.Kind == KindLargeTxn {
			t.Fatalf("unexpected large-txn item: %+v", it)
		}
	}
}

func TestComputeEmptyWindow(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking")},
		Txns:     []domain.Transaction{txn("t0", "a1", 1, -4000, "Kroger", "groceries")},
		Rates:    rates(), Since: since, Until: until,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Items) != 0 || rep.TxnCount != 0 || rep.NetDeltaMinor != 0 {
		t.Fatalf("expected quiet report: %+v", rep)
	}
}

func TestComputePreservesExactHalfOpenCutoffAcrossInterleavedAccounts(t *testing.T) {
	cutoff := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	before := txn("before", "a1", 19, -100, "Shop", "food")
	before.Date = cutoff.Add(-time.Nanosecond)
	atCutoff := txn("at-cutoff", "a2", 19, -900, "Shop", "food")
	atCutoff.Date = cutoff
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking"), asset("a2", "Savings")},
		Txns:     []domain.Transaction{before, atCutoff},
		Rates:    rates(), Since: since, Until: cutoff,
	}
	got, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetDeltaMinor != -100 || got.SpendingMinor != 100 || got.TxnCount != 1 {
		t.Fatalf("exact half-open cutoff report = %+v", got)
	}
	account := find(t, got.Items, KindAccount)
	if account.Count != 1 || len(account.TxnIDs) != 1 || account.TxnIDs[0] != "before" {
		t.Fatalf("cutoff evidence includes excluded transaction: %+v", account)
	}
}

func BenchmarkComputeManyAccounts(b *testing.B) {
	const accountCount, txnsPerAccount = 50, 100
	accounts := make([]domain.Account, 0, accountCount)
	txns := make([]domain.Transaction, 0, accountCount*txnsPerAccount)
	for a := range accountCount {
		id := fmt.Sprintf("account-%02d", a)
		accounts = append(accounts, asset(id, id))
		for n := range txnsPerAccount {
			t := txn(fmt.Sprintf("txn-%02d-%03d", a, n), id, 12+n%7, -100, "Shop", "food")
			txns = append(txns, t)
		}
	}
	in := Input{Accounts: accounts, Txns: txns, Rates: rates(), Since: since, Until: until}
	for _, tc := range []struct {
		name    string
		compute func(Input) (Report, error)
	}{
		{name: "legacy_full_scan", compute: computeLegacy},
		{name: "indexed_positions", compute: Compute},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := tc.compute(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	if b.N == 0 {
		b.Fatal("benchmark did not execute")
	}
}

func TestComputeMatchesFrozenLegacyScan(t *testing.T) {
	cutoff := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	before := txn("before", "a1", 19, math.MaxInt64, "Shop", "food")
	before.Date = cutoff.Add(-time.Nanosecond)
	atCutoff := txn("at-cutoff", "a2", 19, 1, "Shop", "food")
	atCutoff.Date = cutoff
	adj := txn("adj", "a1", 15, -math.MaxInt64, "", "")
	adj.Desc = "Balance adjustment"
	liability := domain.Account{ID: "a2", Name: "Card", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: usd(-100)}
	missingRate := Input{
		Accounts: []domain.Account{{ID: "eur", Name: "Euro", Class: domain.ClassAsset, Currency: "EUR"}},
		Txns:     []domain.Transaction{txn("bad-rate", "eur", 14, -1, "Shop", "food")},
		Rates:    rates(), Since: since, Until: cutoff,
	}
	cases := []struct {
		name string
		in   Input
	}{
		{
			name: "alternating cancellation preserves source order and account order",
			in: Input{
				Accounts: []domain.Account{asset("a2", "Savings"), asset("a1", "Checking")},
				Txns: []domain.Transaction{
					func() domain.Transaction {
						t := txn("a1-max", "a1", 12, math.MaxInt64, "", "")
						t.ExcludeFromReports = true
						return t
					}(),
					func() domain.Transaction {
						t := txn("a2-out", "a2", 13, -300, "", "")
						t.ExcludeFromReports = true
						return t
					}(),
					func() domain.Transaction {
						t := txn("a1-minus", "a1", 14, -math.MaxInt64, "", "")
						t.ExcludeFromReports = true
						return t
					}(),
					func() domain.Transaction {
						t := txn("a1-one", "a1", 15, 1, "", "")
						t.ExcludeFromReports = true
						return t
					}(),
				},
				Rates: rates(), Since: since, Until: cutoff, IsAdjustment: isAdjDesc,
			},
		},
		{
			name: "liability adjustments and exact cutoff",
			in: Input{
				Accounts: []domain.Account{asset("a1", "Checking"), liability},
				Txns:     []domain.Transaction{before, adj, atCutoff},
				Rates:    rates(), Since: since, Until: cutoff, IsAdjustment: isAdjDesc,
			},
		},
		{name: "conversion failure order and error", in: missingRate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, wantErr := computeLegacy(tc.in)
			got, gotErr := Compute(tc.in)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("indexed report differs from frozen scan\n got: %+v\nwant: %+v", got, want)
			}
			if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
				t.Fatalf("indexed error = %v; legacy error = %v", gotErr, wantErr)
			}
		})
	}
}

func TestComputeRejectsMonetaryOverflowWithoutPartialReport(t *testing.T) {
	adjustment := func(id, account string, day int, amount int64) domain.Transaction {
		t := txn(id, account, day, amount, "", "")
		t.Desc = "Balance adjustment"
		return t
	}
	tooMuchSince := txn("since-overflow", "a", 1, 1, "", "")
	flowMax := txn("flow-max", "a", 12, math.MaxInt64, "", "")
	flowOne := txn("flow-one", "a", 14, 1, "", "")
	flowOffset := adjustment("flow-offset", "a", 13, -1)
	contribMax := txn("contrib-max", "a", 12, math.MaxInt64, "", "")
	contribOne := adjustment("contrib-one", "a", 13, 1)
	adjMax := adjustment("adj-max", "a1", 12, math.MaxInt64)
	ordinaryMax := txn("ordinary-max", "a1", 13, -math.MaxInt64, "", "")
	adjOne := adjustment("adj-one", "a2", 12, 1)
	ordinaryOne := txn("ordinary-one", "a2", 13, -1, "", "")

	assetWith := func(id string, opening int64) domain.Account {
		a := asset(id, id)
		a.OpeningBalance = usd(opening)
		return a
	}
	liabilityWith := func(id string, opening int64) domain.Account {
		a := domain.Account{ID: id, Name: id, Class: domain.ClassLiability, Currency: "USD"}
		a.OpeningBalance = usd(opening)
		return a
	}
	cases := []struct {
		name    string
		in      Input
		context string
	}{
		{
			name:    "balance since",
			in:      Input{Accounts: []domain.Account{assetWith("a", math.MaxInt64)}, Txns: []domain.Transaction{tooMuchSince}, Rates: rates(), Since: since, Until: until},
			context: "balance at since",
		},
		{
			name:    "balance until",
			in:      Input{Accounts: []domain.Account{asset("a", "a")}, Txns: []domain.Transaction{flowMax, flowOne}, Rates: rates(), Since: since, Until: until},
			context: "balance at until",
		},
		{
			name:    "ordinary flow despite offsetting adjustment",
			in:      Input{Accounts: []domain.Account{asset("a", "a")}, Txns: []domain.Transaction{flowMax, flowOffset, flowOne}, Rates: rates(), Since: since, Until: until, IsAdjustment: isAdjDesc},
			context: "ordinary flow",
		},
		{
			name:    "asset endpoint subtraction",
			in:      Input{Accounts: []domain.Account{assetWith("a", math.MinInt64)}, Txns: []domain.Transaction{contribMax, contribOne}, Rates: rates(), Since: since, Until: until, IsAdjustment: isAdjDesc},
			context: "net-worth delta",
		},
		{
			name:    "liability minimum magnitude",
			in:      Input{Accounts: []domain.Account{liabilityWith("a", math.MinInt64)}, Rates: rates(), Since: since, Until: until},
			context: "since liability magnitude",
		},
		{
			name:    "transaction minimum magnitude",
			in:      Input{Accounts: []domain.Account{asset("a", "a")}, Txns: []domain.Transaction{txn("minimum", "a", 12, math.MinInt64, "", "")}, Rates: rates(), Since: since, Until: until},
			context: "transaction minimum magnitude",
		},
		{
			name:    "net-worth total",
			in:      Input{Accounts: []domain.Account{asset("a1", "a1"), asset("a2", "a2")}, Txns: []domain.Transaction{txn("max", "a1", 12, math.MaxInt64, "", ""), txn("one", "a2", 12, 1, "", "")}, Rates: rates(), Since: since, Until: until},
			context: "net-worth movement total",
		},
		{
			name:    "adjustment total",
			in:      Input{Accounts: []domain.Account{asset("a1", "a1"), asset("a2", "a2")}, Txns: []domain.Transaction{adjMax, ordinaryMax, adjOne, ordinaryOne}, Rates: rates(), Since: since, Until: until, IsAdjustment: isAdjDesc},
			context: "balance adjustment total",
		},
		{
			name:    "spending total after representable net-worth movement",
			in:      Input{Accounts: []domain.Account{liabilityWith("a1", math.MaxInt64), liabilityWith("a2", 0)}, Txns: []domain.Transaction{txn("expense-max", "a1", 12, -math.MaxInt64, "", "food"), txn("expense-one", "a2", 12, -1, "", "food")}, Rates: rates(), Since: since, Until: until},
			context: "spending total",
		},
		{
			name:    "income total after representable net-worth movement",
			in:      Input{Accounts: []domain.Account{liabilityWith("a1", 0), liabilityWith("a2", -1)}, Txns: []domain.Transaction{txn("income-max", "a1", 12, math.MaxInt64, "", ""), txn("income-one", "a2", 12, 1, "", "")}, Rates: rates(), Since: since, Until: until},
			context: "income total",
		},
		{
			name: "residual subtraction",
			in: Input{Accounts: []domain.Account{asset("a1", "a1"), liabilityWith("a2", math.MaxInt64)}, Txns: []domain.Transaction{func() domain.Transaction {
				t := txn("excluded-max", "a1", 12, math.MaxInt64-1, "", "")
				t.ExcludeFromReports = true
				return t
			}(), txn("expense-one", "a2", 12, -1, "", "food")}, Rates: rates(), Since: since, Until: until},
			context: "residual after cash flow",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compute(tc.in)
			if !errors.Is(err, money.ErrOverflow) || !strings.Contains(err.Error(), tc.context) {
				t.Fatalf("Compute error = %v; want overflow in %q", err, tc.context)
			}
			if !reflect.DeepEqual(got, Report{}) {
				t.Fatalf("overflow returned partial report: %+v", got)
			}
		})
	}
}

func TestComputeEvidenceDedupe(t *testing.T) {
	// One account whose only window activity is two car payments in one
	// category: the account and category findings cite the same evidence set,
	// so only the higher-ranked account finding survives.
	big := txn("t1", "a1", 15, -62000, "", "auto") // payee-less manual entry
	big.Desc = "Car payment"
	in := Input{
		Accounts: []domain.Account{asset("a1", "Joint Checking")},
		Txns: []domain.Transaction{
			big,
			txn("t2", "a1", 17, -48000, "Car payment", "auto"),
		},
		Rates: rates(), Since: since, Until: until, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	var haveAccount, haveCategory, haveLarge bool
	for _, it := range rep.Items {
		switch it.Kind {
		case KindAccount:
			haveAccount = true
		case KindCategory:
			haveCategory = true
		case KindLargeTxn:
			haveLarge = true // distinct evidence ([t1] vs [t1 t2]) — survives
			if it.Payee != "Car payment" {
				t.Fatalf("large-txn label should fall back to Desc: %+v", it)
			}
		}
	}
	if !haveAccount || haveCategory || !haveLarge {
		t.Fatalf("dedupe: account=%v category=%v large=%v items=%+v",
			haveAccount, haveCategory, haveLarge, rep.Items)
	}
}

func TestComputeTopNCap(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{asset("a1", "Checking")},
		Txns: []domain.Transaction{
			txn("t1", "a1", 12, 150000, "ACME Payroll", "salary"),
			txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
		},
		Rates: rates(), Since: since, Until: until, // TopN defaults to 3
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Items) != 3 {
		t.Fatalf("len(items) = %d, want DefaultTopN", len(rep.Items))
	}
	if rep.Items[0].Kind != KindNetWorth {
		t.Fatalf("headline not pinned")
	}
}

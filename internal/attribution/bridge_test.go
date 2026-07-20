// SPDX-License-Identifier: MIT

package attribution

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// liabilityAcct builds a liability account with an explicit opening balance, so
// a test can pick the "stored negative" or "stored positive" debt convention.
func liabilityAcct(id, name string, t domain.AccountType, openingMinor int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, Class: domain.ClassLiability, Type: t,
		Currency: "USD", OpeningBalance: usd(openingMinor),
	}
}

func assetOfType(id, name string, t domain.AccountType, openingMinor int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, Class: domain.ClassAsset, Type: t,
		Currency: "USD", OpeningBalance: usd(openingMinor),
	}
}

// adj marks a transaction as a balance-update adjustment row, matching the
// isAdjDesc predicate the suite uses.
func adj(t domain.Transaction) domain.Transaction {
	t.Desc = "Balance adjustment"
	return t
}

func TestBuildBridgeLegs(t *testing.T) {
	tests := []struct {
		name     string
		accounts []domain.Account
		txns     []domain.Transaction
		want     map[LegKind]int64
		start    int64
		end      int64
	}{
		{
			name:     "cash flow through an asset account is money kept",
			accounts: []domain.Account{assetOfType("a1", "Checking", domain.TypeChecking, 100000)},
			txns: []domain.Transaction{
				txn("t1", "a1", 12, 150000, "Payroll", "salary"),
				txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
			},
			want:  map[LegKind]int64{LegMoneyKept: 130000},
			start: 100000, end: 230000,
		},
		{
			name:     "an adjustment on a brokerage is market movement, not saving",
			accounts: []domain.Account{assetOfType("a2", "Brokerage", domain.TypeInvestment, 5000000)},
			txns: []domain.Transaction{
				adj(txn("t1", "a2", 12, 320000, "", "")),
			},
			want:  map[LegKind]int64{LegMarketMovement: 320000},
			start: 5000000, end: 5320000,
		},
		{
			name:     "an adjustment on a property is a revaluation",
			accounts: []domain.Account{assetOfType("a3", "Condo", domain.TypeProperty, 30400000)},
			txns: []domain.Transaction{
				adj(txn("t1", "a3", 12, 900000, "", "")),
			},
			want:  map[LegKind]int64{LegRevaluation: 900000},
			start: 30400000, end: 31300000,
		},
		{
			name:     "negative-stored debt: a payment pays down, a charge is new debt",
			accounts: []domain.Account{liabilityAcct("l1", "Visa", domain.TypeCreditCard, -200000)},
			txns: []domain.Transaction{
				txn("t1", "l1", 12, 50000, "Payment", ""),
				txn("t2", "l1", 13, -12000, "Charge", ""),
			},
			// |bal| goes 2000.00 → 1620.00, so net worth rises 380.00.
			want:  map[LegKind]int64{LegDebtPaidDown: 50000, LegNewDebt: -12000},
			start: -200000, end: -162000,
		},
		{
			name:     "positive-stored debt: the sign convention flips",
			accounts: []domain.Account{liabilityAcct("l2", "Loan", domain.TypeLoan, 500000)},
			txns: []domain.Transaction{
				txn("t1", "l2", 12, -80000, "Payment", ""),
				txn("t2", "l2", 13, 3000, "Interest", ""),
			},
			want:  map[LegKind]int64{LegDebtPaidDown: 80000, LegNewDebt: -3000},
			start: -500000, end: -423000,
		},
		{
			name: "a mixed household splits into every leg at once",
			accounts: []domain.Account{
				assetOfType("a1", "Checking", domain.TypeChecking, 100000),
				assetOfType("a2", "Brokerage", domain.TypeInvestment, 5000000),
				assetOfType("a3", "Condo", domain.TypeProperty, 30400000),
				liabilityAcct("l1", "Mortgage", domain.TypeMortgage, -25000000),
			},
			txns: []domain.Transaction{
				txn("t1", "a1", 12, 150000, "Payroll", "salary"),
				txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
				adj(txn("t3", "a2", 14, 320000, "", "")),
				adj(txn("t4", "a3", 15, -100000, "", "")),
				txn("t5", "l1", 16, 90000, "Mortgage payment", ""),
			},
			want: map[LegKind]int64{
				LegMoneyKept: 130000, LegMarketMovement: 320000,
				LegRevaluation: -100000, LegDebtPaidDown: 90000,
			},
			start: 100000 + 5000000 + 30400000 - 25000000,
			end:   230000 + 5320000 + 30300000 - 24910000,
		},
		{
			name:     "an empty window still balances with every leg at zero",
			accounts: []domain.Account{assetOfType("a1", "Checking", domain.TypeChecking, 100000)},
			txns:     nil,
			want:     map[LegKind]int64{},
			start:    100000, end: 100000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := BuildBridge(Input{
				Accounts: tc.accounts, Txns: tc.txns, Rates: rates(),
				Since: since, Until: until, IsAdjustment: isAdjDesc,
			})
			if err != nil {
				t.Fatal(err)
			}
			if b.StartMinor != tc.start || b.EndMinor != tc.end {
				t.Fatalf("start/end = %d/%d, want %d/%d", b.StartMinor, b.EndMinor, tc.start, tc.end)
			}
			for _, k := range BridgeLegOrder {
				if k == LegResidual {
					continue
				}
				if got := b.Leg(k); got != tc.want[k] {
					t.Errorf("leg %s = %d, want %d", k, got, tc.want[k])
				}
			}
			if got := b.Leg(LegResidual); got != 0 {
				t.Errorf("residual = %d, want 0 for a fully-explained window", got)
			}
		})
	}
}

// TestBuildBridgeAlwaysBalances is the engine's core contract: whatever the
// input, start + every leg lands exactly on end. A waterfall that does not sum
// is worse than no waterfall.
func TestBuildBridgeAlwaysBalances(t *testing.T) {
	tests := []struct {
		name     string
		accounts []domain.Account
		txns     []domain.Transaction
	}{
		{
			name:     "report-excluded activity lands in the residual",
			accounts: []domain.Account{assetOfType("a1", "Checking", domain.TypeChecking, 0)},
			txns: []domain.Transaction{
				txn("t1", "a1", 12, 150000, "Payroll", "salary"),
				txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
			},
		},
		{
			name:     "a liability crossing zero mid-window still balances",
			accounts: []domain.Account{liabilityAcct("l1", "Card", domain.TypeCreditCard, -10000)},
			txns: []domain.Transaction{
				txn("t1", "l1", 12, 25000, "Overpayment", ""),
				txn("t2", "l1", 14, -5000, "Charge", ""),
			},
		},
		{
			name: "archived accounts are excluded from both cutoffs",
			accounts: []domain.Account{
				assetOfType("a1", "Checking", domain.TypeChecking, 100000),
				func() domain.Account {
					a := assetOfType("a9", "Old", domain.TypeSavings, 999999)
					a.Archived = true
					return a
				}(),
			},
			txns: []domain.Transaction{txn("t1", "a9", 12, 5000, "x", "")},
		},
		{
			name: "a foreign-currency account converts without breaking the identity",
			accounts: []domain.Account{
				func() domain.Account {
					a := assetOfType("a1", "Euro savings", domain.TypeSavings, 123457)
					a.Currency = "EUR"
					return a
				}(),
			},
			txns: []domain.Transaction{
				func() domain.Transaction {
					tx := txn("t1", "a1", 12, 33333, "Interest", "")
					tx.Amount = money.New(33333, "EUR")
					return tx
				}(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := BuildBridge(Input{
				Accounts: tc.accounts, Txns: tc.txns,
				Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}},
				Since: since, Until: until, IsAdjustment: isAdjDesc,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := b.StartMinor+b.LegsSumMinor(), b.EndMinor; got != want {
				t.Fatalf("bridge does not sum: start %d + legs %d = %d, want end %d",
					b.StartMinor, b.LegsSumMinor(), got, want)
			}
			if b.DeltaMinor() != b.LegsSumMinor() {
				t.Fatalf("delta %d != legs %d", b.DeltaMinor(), b.LegsSumMinor())
			}
			if len(b.Legs) != len(BridgeLegOrder) {
				t.Fatalf("Legs has %d entries, want the full canonical shape %d", len(b.Legs), len(BridgeLegOrder))
			}
			for i, k := range BridgeLegOrder {
				if b.Legs[i].Kind != k {
					t.Fatalf("Legs[%d] = %s, want %s", i, b.Legs[i].Kind, k)
				}
			}
		})
	}
}

// TestBuildBridgeAgreesWithCompute pins the bridge to the E1 headline: both
// engines must report the SAME net-worth movement for the same window, or the
// page and the dashboard card would contradict each other.
func TestBuildBridgeAgreesWithCompute(t *testing.T) {
	in := Input{
		Accounts: []domain.Account{
			assetOfType("a1", "Checking", domain.TypeChecking, 100000),
			liabilityAcct("l1", "Visa", domain.TypeCreditCard, -200000),
		},
		Txns: []domain.Transaction{
			txn("t1", "a1", 12, 150000, "Payroll", "salary"),
			txn("t2", "a1", 13, -20000, "Kroger", "groceries"),
			txn("t3", "l1", 14, 50000, "Card payment", ""),
			adj(txn("t4", "a1", 15, 700, "", "")),
		},
		Rates: rates(), Since: since, Until: until, IsAdjustment: isAdjDesc, TopN: 10,
	}
	rep, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildBridge(in)
	if err != nil {
		t.Fatal(err)
	}
	if b.DeltaMinor() != rep.NetDeltaMinor {
		t.Fatalf("bridge delta %d != attribution NetDeltaMinor %d", b.DeltaMinor(), rep.NetDeltaMinor)
	}
}

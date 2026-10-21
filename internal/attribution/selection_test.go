// SPDX-License-Identifier: MIT

package attribution

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

func TestChosenExclusionsMatchNetWorthWithoutChangingCashFlowScope(t *testing.T) {
	accounts := []domain.Account{assetOfType("own", "Own", domain.TypeChecking, 10000), assetOfType("custodial", "Custodial", domain.TypeInvestment, 900000), liabilityAcct("business", "Business debt", domain.TypeLoan, -70000)}
	accounts[1].ExcludeFromNetWorth, accounts[2].ExcludeFromNetWorth = true, true
	in := Input{Accounts: accounts, Rates: rates(), Since: since, Until: until, TopN: 20, IsAdjustment: isAdjDesc,
		Txns: []domain.Transaction{txn("pay", "own", 12, 200, "Salary", "salary"), txn("spend", "custodial", 13, -100, "Food", "food"), adj(txn("market", "custodial", 14, 500000, "", "")), txn("debt", "business", 15, 60000, "Payment", "")}}
	in.Txns[3].TransferAccountID = "own"
	canonical, err := ledger.NetWorthSeries(accounts, in.Txns, []time.Time{since, until}, in.Rates)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := BuildBridge(in)
	if err != nil {
		t.Fatal(err)
	}
	if canonical[0].Amount != 10000 || canonical[1].Amount != 10200 || report.NetDeltaMinor != 200 || bridge.StartMinor != canonical[0].Amount || bridge.EndMinor != canonical[1].Amount || bridge.LegsSumMinor() != 200 {
		t.Fatalf("canonical=%v report=%+v bridge=%+v", canonical, report, bridge)
	}
	// The exclusion applies to net worth, not ordinary reports of cash flow.
	if report.IncomeMinor != 200 || report.SpendingMinor != 100 || report.TxnCount != 4 || report.OtherMinor != 100 {
		t.Fatalf("cash-flow scope changed: %+v", report)
	}
	if bridge.Leg(LegMarketMovement) != 0 || bridge.Leg(LegDebtPaidDown) != 0 || bridge.Leg(LegMoneyKept) != 200 {
		t.Fatalf("chosen activity leaked into net-worth bridge: %+v", bridge)
	}
	for _, leg := range bridge.Legs {
		for _, c := range leg.Contributors {
			if c.AccountID != "own" {
				t.Fatalf("chosen bridge contributor=%+v", c)
			}
		}
	}
	for _, item := range report.Items {
		if item.Kind == KindAccount && item.AccountID != "own" {
			t.Fatalf("chosen net-worth account move=%+v", item)
		}
	}
}

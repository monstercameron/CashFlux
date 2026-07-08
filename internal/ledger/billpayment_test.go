// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func billTxn(id, billAcct string, minor int64, day int) domain.Transaction {
	return domain.Transaction{ID: id, BillAccountID: billAcct, Amount: money.New(minor, "USD"),
		Date: time.Date(2026, time.July, day, 0, 0, 0, 0, time.UTC)}
}

func TestBillPaymentForAccount(t *testing.T) {
	txns := []domain.Transaction{
		billTxn("t1", "mortgage", -148000, 1),
		billTxn("t2", "mortgage", -150000, 15), // most recent
		billTxn("t3", "car", -62000, 10),
		billTxn("t4", "", -999, 20), // unlinked
	}

	m := BillPaymentForAccount("mortgage", txns)
	if !m.HasAny || m.Count != 2 {
		t.Fatalf("mortgage: hasAny=%v count=%d, want true/2", m.HasAny, m.Count)
	}
	if m.LatestTxn != "t2" {
		t.Errorf("latest txn = %q, want t2 (most recent date)", m.LatestTxn)
	}
	if m.Latest.Amount != 150000 { // magnitude of the most recent
		t.Errorf("latest amount = %d, want 150000 (Abs)", m.Latest.Amount)
	}

	c := BillPaymentForAccount("car", txns)
	if !c.HasAny || c.Count != 1 || c.Latest.Amount != 62000 {
		t.Errorf("car = %+v, want 1 payment / 62000", c)
	}

	if none := BillPaymentForAccount("nope", txns); none.HasAny || none.Count != 0 {
		t.Errorf("unknown account = %+v, want empty", none)
	}
	if empty := BillPaymentForAccount("", txns); empty.HasAny {
		t.Error("empty accountID should yield no result")
	}
}

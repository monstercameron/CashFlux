// SPDX-License-Identifier: MIT

package receiptmatch

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func txn(id, desc string, minor int64, date string) domain.Transaction {
	d, _ := time.Parse("2006-01-02", date)
	return domain.Transaction{ID: id, AccountID: "a1", Desc: desc, Date: d, Amount: money.New(minor, "USD")}
}

func day(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }

func TestMatchAmountIsTheEntryBar(t *testing.T) {
	txns := []domain.Transaction{
		txn("t1", "COSTCO WHOLESALE #123", -8742, "2026-07-10"),
		txn("t2", "Costco gas", -5000, "2026-07-10"),   // wrong amount
		txn("t3", "income", 8742, "2026-07-10"),        // income, not an expense
		txn("t4", "far away", -8742, "2026-06-01"),     // outside the window
	}
	got := Match(8742, "Costco", day("2026-07-11"), txns, 5)
	if len(got) != 1 || got[0].Txn.ID != "t1" {
		t.Fatalf("Match = %+v, want just t1", got)
	}
	if !got[0].MerchantHit || got[0].DaysApart != 1 {
		t.Errorf("candidate detail wrong: %+v", got[0])
	}
}

func TestMatchOrdersByDateProximityAndMerchant(t *testing.T) {
	txns := []domain.Transaction{
		txn("far", "Trader Joes", -2500, "2026-07-06"),
		txn("near", "TRADER JOE'S #55", -2500, "2026-07-09"),
		txn("noname", "CARD PURCHASE", -2500, "2026-07-09"),
	}
	got := Match(2500, "Trader Joes", day("2026-07-09"), txns, 5)
	if len(got) != 3 {
		t.Fatalf("want 3 candidates, got %d", len(got))
	}
	if got[0].Txn.ID != "near" {
		t.Errorf("best candidate = %s, want near (same day + merchant)", got[0].Txn.ID)
	}
	if got[1].Txn.ID != "far" {
		t.Errorf("second = %s, want far (a merchant hit at 3 days outranks an anonymous same-day row)", got[1].Txn.ID)
	}
}

func TestMatchSkipsTransfersSplitsAndZero(t *testing.T) {
	tr := txn("tr", "transfer", -1000, "2026-07-10")
	tr.TransferAccountID = "a2"
	sp := txn("sp", "already split", -1000, "2026-07-10")
	sp.Splits = []domain.CategorySplit{{CategoryID: "c1", Amount: money.New(-1000, "USD")}}
	txns := []domain.Transaction{tr, sp}
	if got := Match(1000, "shop", day("2026-07-10"), txns, 5); len(got) != 0 {
		t.Errorf("transfers/split rows must not match: %+v", got)
	}
	if got := Match(0, "shop", day("2026-07-10"), txns, 5); got != nil {
		t.Errorf("zero total must return nil, got %+v", got)
	}
}

func TestMatchCapsAtThree(t *testing.T) {
	var txns []domain.Transaction
	for i := 0; i < 6; i++ {
		txns = append(txns, txn(string(rune('a'+i)), "Shop", -999, "2026-07-10"))
	}
	if got := Match(999, "Shop", day("2026-07-10"), txns, 5); len(got) != 3 {
		t.Errorf("want cap of 3, got %d", len(got))
	}
}

func TestTokensIgnoreShortFragments(t *testing.T) {
	got := tokens("SQ *BLUE BOTTLE CO 47")
	want := map[string]bool{"blue": true, "bottle": true}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("unexpected token %q (short fragments must drop)", tok)
		}
		delete(want, tok)
	}
	if len(want) != 0 {
		t.Errorf("missing tokens: %v", want)
	}
}

// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func subTxn(id, subName string, minor int64, day int) domain.Transaction {
	return domain.Transaction{ID: id, SubscriptionName: subName, Amount: money.New(minor, "USD"),
		Date: time.Date(2026, time.July, day, 0, 0, 0, 0, time.UTC)}
}

func TestSubscriptionPaymentForName(t *testing.T) {
	txns := []domain.Transaction{
		subTxn("t1", "Netflix", -1599, 1),
		subTxn("t2", "Netflix", -1699, 15), // most recent
		subTxn("t3", "Spotify", -1199, 10),
		subTxn("t4", "", -999, 20), // unlinked
	}

	n := SubscriptionPaymentForName("Netflix", txns)
	if !n.HasAny || n.Count != 2 {
		t.Fatalf("Netflix: hasAny=%v count=%d, want true/2", n.HasAny, n.Count)
	}
	if n.LatestTxn != "t2" {
		t.Errorf("latest txn = %q, want t2 (most recent date)", n.LatestTxn)
	}
	if n.Latest.Amount != 1699 { // magnitude of the most recent
		t.Errorf("latest amount = %d, want 1699 (Abs)", n.Latest.Amount)
	}

	s := SubscriptionPaymentForName("Spotify", txns)
	if !s.HasAny || s.Count != 1 || s.Latest.Amount != 1199 {
		t.Errorf("Spotify = %+v, want 1 payment / 1199", s)
	}

	if none := SubscriptionPaymentForName("Hulu", txns); none.HasAny || none.Count != 0 {
		t.Errorf("unknown subscription = %+v, want empty", none)
	}
	if empty := SubscriptionPaymentForName("", txns); empty.HasAny {
		t.Error("empty name should yield no result")
	}
}

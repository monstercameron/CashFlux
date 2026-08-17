// SPDX-License-Identifier: MIT

package payeebase

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func exp(id, payee string, minor int64) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, Amount: money.New(-minor, "USD")}
}

func TestKeyFallsBackToDescription(t *testing.T) {
	if got := Key(domain.Transaction{Payee: "  Netflix "}); got != "netflix" {
		t.Errorf("Key = %q, want netflix", got)
	}
	if got := Key(domain.Transaction{Desc: "NETFLIX.COM"}); got != "netflix.com" {
		t.Errorf("Key from desc = %q", got)
	}
	// Neither set is ungroupable — an empty key must never become a bucket that
	// lumps every anonymous charge together into a nonsense baseline.
	if got := Key(domain.Transaction{}); got != "" {
		t.Errorf("Key of an anonymous transaction = %q, want empty", got)
	}
}

func TestMedianIsRobustToOneOutlier(t *testing.T) {
	// A yearly renewal among monthly charges is exactly why this is not a mean.
	monthly := []int64{1599, 1599, 1599, 1599, 19900}
	if got := Median(monthly); got != 1599 {
		t.Errorf("Median = %d, want 1599", got)
	}
	if got := Median([]int64{100, 200}); got != 150 {
		t.Errorf("even-length Median = %d, want 150", got)
	}
	if got := Median(nil); got != 0 {
		t.Errorf("Median(nil) = %d, want 0", got)
	}
	// The input must survive: callers pass slices they still need.
	src := []int64{3, 1, 2}
	Median(src)
	if src[0] != 3 {
		t.Errorf("Median sorted its input in place: %v", src)
	}
}

func TestTypicalExcludesTheTransactionBeingJudged(t *testing.T) {
	// Four at 15.99 plus one at 19.99. Judging the 19.99 must see 15.99 as
	// typical — a charge that is its own baseline can never look unusual.
	txns := []domain.Transaction{
		exp("a", "Netflix", 1599), exp("b", "Netflix", 1599),
		exp("c", "Netflix", 1599), exp("d", "Netflix", 1599),
		exp("e", "Netflix", 1999),
	}
	got, ok := Typical(txns, txns[4])
	if !ok || got != 1599 {
		t.Fatalf("Typical = %d,%v want 1599,true", got, ok)
	}
	r, ok := Ratio(txns, txns[4])
	if !ok || math.Abs(r-1999.0/1599.0) > 1e-9 {
		t.Errorf("Ratio = %v,%v", r, ok)
	}
}

func TestTypicalDeclinesWithoutEnoughHistory(t *testing.T) {
	txns := []domain.Transaction{
		exp("a", "Rare Shop", 500), exp("b", "Rare Shop", 700), exp("c", "Rare Shop", 900),
	}
	// Judging "c" leaves only two others — below MinHistory.
	if _, ok := Typical(txns, txns[2]); ok {
		t.Error("a two-charge history produced a baseline")
	}
	// And "no opinion" must not be reported as a ratio of zero — an unknown
	// baseline is not evidence that a price dropped to nothing.
	if r, ok := Ratio(txns, txns[2]); ok || r != 0 {
		t.Errorf("Ratio = %v,%v want 0,false", r, ok)
	}
}

func TestTypicalIgnoresOtherMerchantsAndIncome(t *testing.T) {
	txns := []domain.Transaction{
		exp("a", "Netflix", 1599), exp("b", "Netflix", 1599),
		exp("c", "Netflix", 1599), exp("d", "Netflix", 1599),
		exp("x", "Hulu", 9900), exp("y", "Hulu", 9900), exp("z", "Hulu", 9900),
		{ID: "in", Payee: "Netflix", Amount: money.New(1599, "USD")}, // a refund, not a charge
		exp("t", "Netflix", 1999),
	}
	got, ok := Typical(txns, txns[len(txns)-1])
	if !ok || got != 1599 {
		t.Errorf("Typical = %d,%v want 1599,true — Hulu and the refund must not count", got, ok)
	}
}

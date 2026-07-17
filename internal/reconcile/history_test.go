// SPDX-License-Identifier: MIT

package reconcile

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func rec(day int, stmtDay int) domain.Reconciliation {
	r := domain.Reconciliation{
		At:               time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC),
		StatementBalance: money.New(1000, "USD"),
	}
	if stmtDay > 0 {
		r.StatementDate = time.Date(2026, 6, stmtDay, 0, 0, 0, 0, time.UTC)
	}
	return r
}

func TestRecordAppendsWithoutMutating(t *testing.T) {
	base := []domain.Reconciliation{rec(1, 0)}
	out := Record(base, rec(2, 0))
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if len(base) != 1 {
		t.Error("input slice was mutated")
	}
	if !out[1].At.Equal(rec(2, 0).At) {
		t.Error("event not appended last (oldest-first order)")
	}
}

func TestRecordCapsAtMaxHistory(t *testing.T) {
	var h []domain.Reconciliation
	for i := 0; i < MaxHistory+5; i++ {
		h = Record(h, domain.Reconciliation{At: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)})
	}
	if len(h) != MaxHistory {
		t.Fatalf("len = %d, want cap %d", len(h), MaxHistory)
	}
	// The oldest entries were dropped: the first survivor is month 5.
	want := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	if !h[0].At.Equal(want) {
		t.Errorf("first surviving entry = %v, want %v", h[0].At, want)
	}
}

func TestThrough(t *testing.T) {
	if _, ok := Through(nil); ok {
		t.Error("empty history should report not-reconciled")
	}
	// Statement date wins over recording time; newest wins overall.
	h := []domain.Reconciliation{rec(10, 5), rec(20, 15)}
	got, ok := Through(h)
	if !ok {
		t.Fatal("expected a through date")
	}
	want := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("through = %v, want %v", got, want)
	}
	// An entry with no statement date falls back to its recording time.
	h = append(h, rec(25, 0))
	got, _ = Through(h)
	if !got.Equal(rec(25, 0).At) {
		t.Errorf("through = %v, want the newest recording time", got)
	}
}

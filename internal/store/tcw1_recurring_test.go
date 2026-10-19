package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTCW1OccurrenceRollbackAndRetry(t *testing.T) {
	for _, failure := range []string{"first leg", "second leg", "receipt", "schedule"} {
		t.Run(failure, func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			prior := domain.Recurring{ID: "r", Label: "Rent", NextDue: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Cadence: domain.CadenceMonthly, Amount: money.New(-100, "USD")}
			if err := s.PutRecurring(prior); err != nil {
				t.Fatal(err)
			}
			rows := []domain.Transaction{{ID: "out", Amount: money.New(-100, "USD")}, {ID: "in", Amount: money.New(100, "USD")}}
			var trigger string
			switch failure {
			case "first leg":
				trigger = `CREATE TRIGGER fail_occurrence BEFORE INSERT ON transactions WHEN NEW.id='out' BEGIN SELECT RAISE(FAIL,'injected'); END`
			case "second leg":
				trigger = `CREATE TRIGGER fail_occurrence BEFORE INSERT ON transactions WHEN NEW.id='in' BEGIN SELECT RAISE(FAIL,'injected'); END`
			case "receipt":
				trigger = `CREATE TRIGGER fail_occurrence BEFORE INSERT ON appkv BEGIN SELECT RAISE(FAIL,'injected'); END`
			case "schedule":
				trigger = `CREATE TRIGGER fail_occurrence BEFORE UPDATE ON recurring BEGIN SELECT RAISE(FAIL,'injected'); END`
			}
			if _, err := s.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			cursor, err := s.ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			if posted, err := s.PutRecurringOccurrenceAtomic("op", prior, prior.Advance(), rows); posted || err == nil {
				t.Fatalf("posted=%t err=%v", posted, err)
			}
			if txns, err := s.ListTransactions(); err != nil || len(txns) != 0 {
				t.Fatalf("partial rows=%+v %v", txns, err)
			}
			current, _, _ := s.GetRecurring(prior.ID)
			if !current.NextDue.Equal(prior.NextDue) {
				t.Fatal("failed occurrence advanced")
			}
			if _, found, err := s.GetKV("recurring-occurrence:op"); found || err != nil {
				t.Fatal("failed occurrence reserved receipt")
			}
			if after, _ := s.ChangeCursor(); after != cursor {
				t.Fatal("rolled back writes leaked into durable change capture")
			}
			if _, err := s.db.Exec("DROP TRIGGER fail_occurrence"); err != nil {
				t.Fatal(err)
			}
			if posted, err := s.PutRecurringOccurrenceAtomic("op", prior, prior.Advance(), rows); !posted || err != nil {
				t.Fatalf("retry posted=%t err=%v", posted, err)
			}
			if posted, err := s.PutRecurringOccurrenceAtomic("op", prior, prior.Advance(), rows); posted || err != nil {
				t.Fatalf("idempotent retry posted=%t err=%v", posted, err)
			}
		})
	}
}

func TestTCW1OccurrenceRejectsScheduleAndTransactionConflicts(t *testing.T) {
	for _, conflict := range []string{"schedule", "transaction"} {
		t.Run(conflict, func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := domain.Recurring{ID: "r", NextDue: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Cadence: domain.CadenceMonthly}
			if err := s.PutRecurring(r); err != nil {
				t.Fatal(err)
			}
			rows := []domain.Transaction{{ID: "collision"}}
			if conflict == "schedule" {
				changed := r
				changed.Label = "Changed"
				if err := s.PutRecurring(changed); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := s.PutTransaction(rows[0]); err != nil {
					t.Fatal(err)
				}
			}
			if posted, err := s.PutRecurringOccurrenceAtomic("op", r, r.Advance(), rows); posted || err == nil {
				t.Fatalf("posted=%t err=%v", posted, err)
			}
			current, _, _ := s.GetRecurring(r.ID)
			if !current.NextDue.Equal(r.NextDue) {
				t.Fatal("conflict advanced schedule")
			}
		})
	}
}

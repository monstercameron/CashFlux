// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func contributionRecord(t *testing.T, table, id string, before, after any) RecordChange {
	t.Helper()
	encode := func(value any) json.RawMessage {
		if value == nil {
			return nil
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	return RecordChange{Table: table, ID: id, Before: encode(before), After: encode(after)}
}

func TestGoalContributionBatchRollsBackEveryRecordAndCapture(t *testing.T) {
	for _, failure := range []string{"goal save", "debit save", "debit undo", "relationship undo"} {
		t.Run(failure, func(t *testing.T) {
			s := newStore(t)
			before := domain.Goal{ID: "goal", Name: "Trip", CurrentAmount: money.New(100, "USD")}
			after := before
			after.CurrentAmount = money.New(300, "USD")
			after = after.RecordContribution(domain.GoalContribution{Amount: money.New(200, "USD"), TxnID: "debit"})
			debit := domain.Transaction{ID: "debit", AccountID: "bank", Amount: money.New(-200, "USD")}
			goalChange := contributionRecord(t, "goals", before.ID, before, after)
			debitChange := contributionRecord(t, "transactions", debit.ID, nil, debit)
			var changes []RecordChange
			var trigger string
			switch failure {
			case "goal save", "debit save":
				if err := s.PutGoal(before); err != nil {
					t.Fatal(err)
				}
				changes = []RecordChange{debitChange, goalChange}
				if failure == "goal save" {
					trigger = "CREATE TRIGGER reject_contribution BEFORE UPDATE ON goals BEGIN SELECT RAISE(ABORT, 'injected goal failure'); END"
				} else {
					trigger = "CREATE TRIGGER reject_contribution BEFORE INSERT ON transactions BEGIN SELECT RAISE(ABORT, 'injected debit failure'); END"
				}
			default:
				if err := s.PutGoal(after); err != nil {
					t.Fatal(err)
				}
				if err := s.PutTransaction(debit); err != nil {
					t.Fatal(err)
				}
				link := domain.TxnLink{ID: "relationship", TxnIDs: []string{debit.ID, "other"}}
				if err := s.PutTxnLink(link); err != nil {
					t.Fatal(err)
				}
				changes = []RecordChange{contributionRecord(t, "goals", before.ID, after, before),
					contributionRecord(t, "transactions", debit.ID, debit, nil),
					contributionRecord(t, "txnlinks", link.ID, link, nil)}
				if failure == "debit undo" {
					trigger = "CREATE TRIGGER reject_contribution BEFORE DELETE ON transactions BEGIN SELECT RAISE(ABORT, 'injected debit delete failure'); END"
				} else {
					trigger = "CREATE TRIGGER reject_contribution BEFORE DELETE ON txnlinks BEGIN SELECT RAISE(ABORT, 'injected relationship failure'); END"
				}
			}
			baseline, err := s.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			rev, from := s.Rev(), cursor(t, s)
			if err := s.ReplayChangesExpected(changes); err == nil {
				t.Fatal("injected failure was accepted")
			}
			got, err := s.Snapshot()
			if err != nil || !reflect.DeepEqual(got, baseline) || s.Rev() != rev || cursor(t, s) != from {
				t.Fatalf("failed batch changed records, revision or capture: %v", err)
			}
			if _, err := s.db.Exec("DROP TRIGGER reject_contribution"); err != nil {
				t.Fatal(err)
			}
			if err := s.ReplayChangesExpected(changes); err != nil {
				t.Fatalf("unchanged failed batch could not be retried: %v", err)
			}
			if captured := captureAll(t, s, from, cursor(t, s), 8); len(captured) != len(changes) || s.Rev() != rev+1 {
				t.Fatalf("retry did not commit exactly one complete capture: %d records", len(captured))
			}
		})
	}
}

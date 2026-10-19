package store

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestIMV4ImportBatchEffectsAtomic(t *testing.T) {
	for _, test := range []string{"success", "row conflict", "history conflict", "effect conflict", "invalid effect"} {
		t.Run(test, func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			row := domain.Transaction{ID: "new"}
			doc := domain.Document{ID: "import"}
			goal := domain.Goal{ID: "goal", Name: "Existing fund"}
			if err := s.PutGoal(goal); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(goal)
			goal.Name = "Updated fund"
			after, _ := json.Marshal(goal)
			change := RecordChange{Table: "goals", ID: goal.ID, Before: before, After: after}
			switch test {
			case "row conflict":
				if err := s.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			case "history conflict":
				if err := s.PutDocument(doc); err != nil {
					t.Fatal(err)
				}
			case "effect conflict":
				change.Before = json.RawMessage(`{"id":"goal"}`)
			case "invalid effect":
				change.Table = "settings"
			}
			cursor, _ := s.ChangeCursor()
			revision := s.Rev()
			err = s.PutImportBatchWithChanges([]domain.Transaction{row}, doc, []RecordChange{change})
			if test == "success" {
				if err != nil {
					t.Fatal(err)
				}
				end, _ := s.ChangeCursor()
				page, err := s.CaptureChanges(cursor, end, 20)
				if err != nil {
					t.Fatal(err)
				}
				if len(page.Changes) != 3 || s.Rev() != revision+1 {
					t.Fatalf("changes=%d revision=%d/%d", len(page.Changes), s.Rev(), revision)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid batch succeeded")
			}
			end, _ := s.ChangeCursor()
			if end != cursor || s.Rev() != revision {
				t.Fatal("rolled back import advanced capture or revision")
			}
			rows, _ := s.ListTransactions()
			docs, _ := s.ListDocuments()
			funds, _ := s.ListGoals()
			wantRows, wantDocs := 0, 0
			if test == "row conflict" {
				wantRows = 1
			}
			if test == "history conflict" {
				wantDocs = 1
			}
			if len(rows) != wantRows || len(docs) != wantDocs || funds[0].Name != "Existing fund" {
				t.Fatalf("partial import: rows=%d docs=%d goals=%+v", len(rows), len(docs), funds)
			}
		})
	}
}

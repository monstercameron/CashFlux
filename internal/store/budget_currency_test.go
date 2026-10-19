// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetCurrencyBackfill(t *testing.T) {
	for _, base := range []string{"USD", "EUR", ""} {
		t.Run(base, func(t *testing.T) {
			want := base
			if want == "" {
				want = "USD"
			}
			raw := []byte(`{"schemaVersion":1,"settings":{"baseCurrency":"` + base + `"},"budgets":[{"id":"bud-splurges","period":"yearly","limit":{"amount":600000,"currency":""}}]}`)
			ds, err := Import(raw)
			if err != nil {
				t.Fatal(err)
			}
			if ds.Budgets[0].Limit.Currency != want {
				t.Fatalf("import currency=%q", ds.Budgets[0].Limit.Currency)
			}
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			legacy := Dataset{Settings: Settings{BaseCurrency: base}, Budgets: []domain.Budget{{ID: "bud-splurges", Limit: money.New(600000, "")}, {ID: "explicit", Limit: money.New(100, "JPY")}}}
			if err := s.Load(legacy); err != nil {
				t.Fatal(err)
			}
			snap, err := s.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range snap.Budgets {
				if b.ID == "bud-splurges" && b.Limit.Currency != want {
					t.Fatalf("load currency=%q", b.Limit.Currency)
				}
				if b.ID == "explicit" && b.Limit.Currency != "JPY" {
					t.Fatal("explicit currency changed")
				}
			}
			if legacy.Budgets[0].Limit.Currency != "" {
				t.Fatal("load mutated input")
			}
			encoded, err := Export(snap)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Import(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if again.Budgets[0].Limit.Currency != snap.Budgets[0].Limit.Currency {
				t.Fatal("backfill not idempotent")
			}
		})
	}
}

func TestBudgetCurrencyWriteAndSample(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutSettings(Settings{BaseCurrency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutBudget(domain.Budget{ID: "implicit", Limit: money.New(100, "")}); err != nil {
		t.Fatal(err)
	}
	var data string
	if err := s.db.QueryRow("SELECT data FROM budgets WHERE id='implicit'").Scan(&data); err != nil {
		t.Fatal(err)
	}
	var b domain.Budget
	if err := json.Unmarshal([]byte(data), &b); err != nil {
		t.Fatal(err)
	}
	if b.Limit.Currency != "EUR" {
		t.Fatal("write did not persist explicit currency")
	}
	for _, b := range SampleDatasetAt(sampleAuthoredNow).Budgets {
		if b.Limit.Currency != "USD" {
			t.Fatalf("sample budget %s currency=%q", b.ID, b.Limit.Currency)
		}
		if b.Rollover && b.RolloverStart.IsZero() {
			t.Fatalf("sample budget %s lacks first rollover period", b.ID)
		}
	}
}

func TestBudgetCarryConfigurationRoundTrip(t *testing.T) {
	b := domain.Budget{ID: "groceries", Limit: money.New(45000, "USD"), Rollover: true, RolloverStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), LimitHistory: map[string]int64{"2026-09-01": 45000, "2026-10-01": 63564}, PeriodBoosts: map[string]int64{"2026-10-01": 100}}
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	data, err := Export(Dataset{Budgets: []domain.Budget{b}})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Load(ds); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.GetBudget(b.ID)
	if err != nil || !found {
		t.Fatalf("read: found=%v err=%v", found, err)
	}
	if !got.RolloverStart.Equal(b.RolloverStart) || got.LimitHistory["2026-10-01"] != 63564 || got.PeriodBoosts["2026-10-01"] != 100 {
		t.Fatalf("round trip=%+v", got)
	}
}

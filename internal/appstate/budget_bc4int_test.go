// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestBudgetBC4INTMixedBatchIsAtomic(t *testing.T) {
	for _, mode := range []string{"success", "invalid create", "second row storage failure", "duplicate ID"} {
		t.Run(mode, func(t *testing.T) {
			a := newApp(t, false)
			start, ref := moneyDate("2026-09-01"), moneyDate("2026-10-19")
			if err := a.PutBudgetAt(sharedBudget("existing", "Existing", "cat", 10000), start); err != nil {
				t.Fatal(err)
			}
			before := a.Budgets()[0]
			updated := before
			updated.Limit.Amount = 15000
			updated.LimitHistory = map[string]int64{"2026-09-01": 99999}
			created := sharedBudget("new", "New", "new-cat", 5000)
			switch mode {
			case "invalid create":
				created.Name = ""
			case "second row storage failure":
				// Validation permits unknown custom fields; JSON encoding fails
				// inside the transaction after the first row has been written.
				created.Custom = map[string]any{"cannotEncode": func() {}}
			case "duplicate ID":
				created.ID = updated.ID
			}
			rev := a.Rev()
			err := a.PutBudgetsAtomicAt([]domain.Budget{updated, created}, ref)
			if mode != "success" {
				if err == nil || a.Rev() != rev || len(a.Budgets()) != 1 || !reflect.DeepEqual(a.Budgets()[0], before) {
					t.Fatalf("failed batch changed data: err=%v budgets=%+v", err, a.Budgets())
				}
				return
			}
			if err != nil || len(a.Budgets()) != 2 {
				t.Fatalf("mixed batch: %v budgets=%+v", err, a.Budgets())
			}
			for _, b := range a.Budgets() {
				if b.ID == "existing" && (b.LimitHistory["2026-09-01"] != 10000 || b.LimitHistory["2026-10-01"] != 15000) {
					t.Fatalf("stored history was bypassed: %+v", b)
				}
				if b.ID == "new" && (b.LimitHistory["2026-10-01"] != 5000 || !b.RolloverStart.Equal(moneyDate("2026-10-01"))) {
					t.Fatalf("new budget missing initial funding: %+v", b)
				}
			}
		})
	}
}

func TestBudgetBC4INTMigrationValidationIsAtomic(t *testing.T) {
	for _, mode := range []string{"success", "invalid history", "off-grid boost", "off-grid note", "off-grid start", "plan change", "new row", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			a := newApp(t, false)
			var before []domain.Budget
			for _, id := range []string{"first", "second"} {
				b := sharedBudget(id, id, "cat", 10000)
				b.Period = domain.PeriodWeekly
				b.LimitHistory = map[string]int64{"2026-10-12": 10000}
				b.PeriodBoosts = map[string]int64{"2026-10-12": 500}
				b.PeriodNotes = map[string]string{"2026-10-12": "Guests"}
				b.RolloverStart = moneyDate("2026-10-12")
				if err := a.Store().PutBudget(b); err != nil {
					t.Fatal(err)
				}
			}
			before = a.Budgets()
			writes := append([]domain.Budget(nil), before...)
			for i := range writes {
				writes[i].LimitHistory = map[string]int64{"2026-10-11": 10000}
				writes[i].PeriodBoosts = map[string]int64{"2026-10-11": 500}
				writes[i].PeriodNotes = map[string]string{"2026-10-11": "Guests"}
				writes[i].RolloverStart = moneyDate("2026-10-11")
			}
			switch mode {
			case "invalid history":
				writes[1].LimitHistory = map[string]int64{"bad": 10000}
			case "off-grid boost":
				writes[1].PeriodBoosts = map[string]int64{"2026-10-12": 500}
			case "off-grid note":
				writes[1].PeriodNotes = map[string]string{"2026-10-12": "Guests"}
			case "off-grid start":
				writes[1].RolloverStart = moneyDate("2026-10-12")
			case "plan change":
				writes[1].Limit.Amount++
			case "new row":
				writes[1].ID = "missing"
			case "duplicate":
				writes[1] = writes[0]
			}
			rev := a.Rev()
			err := a.MigrateBudgetPeriodsAtomic(writes, time.Sunday, time.Time{})
			if mode != "success" {
				if err == nil || a.Rev() != rev || !reflect.DeepEqual(a.Budgets(), before) {
					t.Fatalf("failed migration changed budgets: %v %+v", err, a.Budgets())
				}
				return
			}
			if err != nil || !reflect.DeepEqual(a.Budgets(), writes) || before[0].LimitHistory["2026-10-12"] != 10000 {
				t.Fatalf("migration=%v budgets=%+v", err, a.Budgets())
			}
		})
	}
}

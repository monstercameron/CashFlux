// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

func TestSettingsRekeyBC4INTKeepsCurrentCardCap(t *testing.T) {
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Store().Close()
	old, next := prefs.Prefs{WeekStart: prefs.WeekMonday}, prefs.Prefs{WeekStart: prefs.WeekSunday}
	if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"monday"}`); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	ref := start.AddDate(0, 0, 2)
	b := domain.Budget{ID: "weekly", Name: "Food", CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Period: domain.PeriodWeekly, Limit: money.New(20000, "USD"), Rollover: true,
		PeriodBoosts: map[string]int64{"2026-10-12": 500}, PeriodNotes: map[string]string{"2026-10-12": "Guests"}}
	if err := a.PutBudgetAt(b, start); err != nil {
		t.Fatal(err)
	}
	input, err := a.BudgetViewInput(ref)
	if err != nil {
		t.Fatal(err)
	}
	input.Period.WeekStart = time.Monday
	before := budgeting.EvaluateViewBudget(a.Budgets()[0], input)
	if err := rekeyBudgetPeriodPreferences(a, old, next, ref); err != nil {
		t.Fatal(err)
	}
	got := a.Budgets()[0]
	input.Period.WeekStart = time.Sunday
	after := budgeting.EvaluateViewBudget(got, input)
	if !before.Valid || !after.Valid || before.Status.Limit.Amount != 20500 || after.Status.Limit != before.Status.Limit {
		t.Fatalf("card cap before=%+v after=%+v", before, after)
	}
	if got.LimitHistory["2026-10-11"] != 20000 || got.PeriodBoosts["2026-10-11"] != 500 || got.PeriodNotes["2026-10-11"] != "Guests" || got.RolloverStart.Format(time.DateOnly) != "2026-10-11" {
		t.Fatalf("period data did not migrate: %+v", got)
	}
	if _, exists := got.LimitHistory["2026-10-12"]; exists {
		t.Fatal("old history key survived")
	}
}

func TestFinancialWipeBC4INTOwnerCommitsWithDataset(t *testing.T) {
	for _, failure := range []bool{true, false} {
		t.Run(map[bool]string{true: "failed wipe", false: "successful wipe"}[failure], func(t *testing.T) {
			fixture := solp22Journal(t)
			solp23PreserveStorage(t)
			const owner = `{"serverId":"bc4int-server","userId":"bc4int-owner"}`
			solp22Batch(t, map[string]string{datasetOwnerKey: owner}, nil)
			checkpoint, head, generation := browserstore.GetString(datasetStoreKey), browserstore.GetString(datasetHeadKey), readDatasetGen()
			oldLocation := js.Global().Get("location")
			reloaded := false
			reloadDone := make(chan struct{})
			reload := js.FuncOf(func(js.Value, []js.Value) any {
				reloaded = true
				close(reloadDone)
				return nil
			})
			location := js.Global().Get("Object").New()
			location.Set("reload", reload)
			js.Global().Set("location", location)
			t.Cleanup(func() { js.Global().Set("location", oldLocation); reload.Release() })
			var interrupted *bool
			if failure {
				interrupted = solp23FailPut(t, datasetOwnerKey)
			}
			if err := appstate.Default.Wipe(); err != nil {
				t.Fatal(err)
			}
			suspendAutosave = true
			wipeFinancialLocalState(reloadPage, wipeEraseEverything)
			if failure {
				browserstore.Barrier()
				if !*interrupted {
					t.Fatal("owner tombstone failure was not injected")
				}
				if reloaded || browserstore.GetString(datasetOwnerKey) != owner || browserstore.GetString(datasetStoreKey) != checkpoint || browserstore.GetString(datasetHeadKey) != head || readDatasetGen() != generation {
					t.Fatal("failed wipe released owner, changed saved data, or reloaded")
				}
				solp22Reload(t, fixture.row)
			} else {
				select {
				case <-reloadDone:
				case <-time.After(2 * time.Minute):
					t.Fatal("successful wipe did not reload after its durable writes")
				}
				browserstore.Barrier()
				if !reloaded || browserstore.GetString(datasetOwnerKey) != "" || readDatasetGen() == generation {
					t.Fatal("successful wipe did not tombstone owner with dataset and reload")
				}
			}
		})
	}
}

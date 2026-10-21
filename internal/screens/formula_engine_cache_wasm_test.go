// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestLiveEngineVarsCacheTracksPersistedInputsWithoutRevisionBump(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	for _, txn := range []domain.Transaction{
		{ID: "oct", AccountID: "cash", Desc: "October expense", Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-10000, "USD")},
		{ID: "nov", AccountID: "cash", Desc: "November expense", Date: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-25000, "USD")},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}

	base := liveEngineVarInputs{
		Now:       time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		WeekStart: time.Sunday, PayCycleAnchor: "2026-10-02", IdleBenchmark: 4.5,
		Alloc:       engineenv.AllocData{AmountMinor: 250000, ReserveMinor: 50000, MaxPerMinor: 75000},
		Planning:    engineenv.PlanningData{RunwayBufferMinor: 10000, RunwayDays: 45, ForecastMonths: 6},
		BillsConfig: uistate.BillsSmartConfig{PayFrequency: "biweekly", MinKeepMinor: 20000},
	}
	storeRev, uiRev := app.Rev(), uistate.CurrentDataRevision()
	first := liveEngineVarsWithInputs(app, base)
	if got := first["alloc_amount"]; got != 2500 {
		t.Fatalf("alloc_amount = %v, want 2500", got)
	}
	if got := first["runway_buffer"]; got != 100 {
		t.Fatalf("runway_buffer = %v, want 100", got)
	}
	if got := first["forecast_horizon"]; got != 6 {
		t.Fatalf("forecast_horizon = %v, want 6", got)
	}
	if got := first["expense"]; got != 100 {
		t.Fatalf("October expense = %v, want 100", got)
	}
	if same := liveEngineVarsWithInputs(app, base); reflect.ValueOf(first).Pointer() != reflect.ValueOf(same).Pointer() {
		t.Fatal("identical inputs should reuse the cached immutable map")
	}

	changedAlloc := base
	changedAlloc.Alloc.AmountMinor = 500000
	second := liveEngineVarsWithInputs(app, changedAlloc)
	if second["alloc_amount"] != 5000 || reflect.ValueOf(first).Pointer() == reflect.ValueOf(second).Pointer() {
		t.Fatalf("alloc config change reused stale metric: %v", second["alloc_amount"])
	}
	changedPlanning := changedAlloc
	changedPlanning.Planning.RunwayBufferMinor = 30000
	changedPlanning.Planning.ForecastMonths = 18
	third := liveEngineVarsWithInputs(app, changedPlanning)
	if third["runway_buffer"] != 300 || third["forecast_horizon"] != 18 {
		t.Fatalf("planning config change did not refresh metrics: buffer=%v horizon=%v", third["runway_buffer"], third["forecast_horizon"])
	}
	if app.Rev() != storeRev || uistate.CurrentDataRevision() != uiRev {
		t.Fatal("fixture unexpectedly changed a revision while varying cache inputs")
	}

	november := changedPlanning
	november.Now = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	novemberVars := liveEngineVarsWithInputs(app, november)
	if novemberVars["expense"] != 250 || reflect.ValueOf(third).Pointer() == reflect.ValueOf(novemberVars).Pointer() {
		t.Fatalf("month boundary retained October calculation: expense=%v", novemberVars["expense"])
	}
	nextDay := november
	nextDay.Now = nextDay.Now.AddDate(0, 0, 1)
	novemberKey, _ := liveEngineVarsKey(app, november)
	nextDayKey, _ := liveEngineVarsKey(app, nextDay)
	if novemberKey == nextDayKey {
		t.Fatal("date-sensitive inputs must have distinct cache identities across days")
	}

	beforeIntradayBoundary := changedPlanning
	beforeIntradayBoundary.Now = time.Date(2026, 10, 31, 19, 59, 0, 0, time.FixedZone("EDT", -4*60*60))
	afterIntradayBoundary := beforeIntradayBoundary
	afterIntradayBoundary.Now = time.Date(2026, 10, 31, 20, 1, 0, 0, time.FixedZone("EDT", -4*60*60))
	beforeRaw := liveEngineVarsRaw(app, beforeIntradayBoundary)
	afterRaw := liveEngineVarsRaw(app, afterIntradayBoundary)
	if beforeRaw["days_left"] != 1 || afterRaw["days_left"] != 0 {
		t.Fatalf("fixture missed the exact month-end boundary: raw before=%v after=%v", beforeRaw["days_left"], afterRaw["days_left"])
	}
	beforeVars := liveEngineVarsWithInputs(app, beforeIntradayBoundary)
	afterVars := liveEngineVarsWithInputs(app, afterIntradayBoundary)
	if beforeVars["days_left"] != beforeRaw["days_left"] || afterVars["days_left"] != afterRaw["days_left"] {
		t.Fatalf("cache differs from exact-time engine calculation: before=%v/%v after=%v/%v", beforeVars["days_left"], beforeRaw["days_left"], afterVars["days_left"], afterRaw["days_left"])
	}
}

func TestLiveEngineVarsCacheKeyIncludesEveryNonRevisionInput(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	base := liveEngineVarInputs{Now: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), BillsConfig: uistate.BillsSmartConfig{PayFrequency: "biweekly"}}
	mutations := []func(*liveEngineVarInputs){
		func(v *liveEngineVarInputs) { v.Now = v.Now.AddDate(0, 0, 1) },
		func(v *liveEngineVarInputs) { v.Now = v.Now.In(time.FixedZone("other", 3600)) },
		func(v *liveEngineVarInputs) { v.WeekStart = time.Monday },
		func(v *liveEngineVarInputs) { v.PayCycleAnchor = "2026-10-03" },
		func(v *liveEngineVarInputs) { v.IdleBenchmark = 1 },
		func(v *liveEngineVarInputs) { v.Alloc.AmountMinor = 1 },
		func(v *liveEngineVarInputs) { v.Planning.RunwayDays = 1 },
		func(v *liveEngineVarInputs) { v.BillsConfig.MinKeepMinor = 1 },
		func(v *liveEngineVarInputs) { v.Pools = []engineenv.PoolDef{{Name: "retirement"}} },
		func(v *liveEngineVarInputs) { v.Smart.FreeOn = 1 },
	}
	baseKey, ok := liveEngineVarsKey(app, base)
	if !ok {
		t.Fatal("normal input snapshot should have a valid JSON key")
	}
	for i, mutate := range mutations {
		changed := base
		mutate(&changed)
		if key, ok := liveEngineVarsKey(app, changed); !ok || key == baseKey {
			t.Errorf("mutation %d did not change the cache identity", i)
		}
	}
}

func TestLiveEngineVarsUncacheableInputsBypassCache(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	in := liveEngineVarInputs{Now: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), IdleBenchmark: math.NaN()}
	if _, ok := liveEngineVarsKey(app, in); ok {
		t.Fatal("NaN input unexpectedly produced a cache identity")
	}
	first := liveEngineVarsWithInputs(app, in)
	second := liveEngineVarsWithInputs(app, in)
	if reflect.ValueOf(first).Pointer() == reflect.ValueOf(second).Pointer() {
		t.Fatal("uncacheable inputs should be recomputed without retaining a colliding value")
	}
}

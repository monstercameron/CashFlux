// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func newPlanningRetirementHostApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

type planningRetirementHostProps struct {
	Base string
	Tick int
}

// planningRetirementHost wraps the production tile in a component with an
// explicit test tick. GWC v7 can bail out when the same component is rendered
// with unchanged props, so changing Tick makes this mounted view execute again
// without touching the app's UI data revision.
func planningRetirementHost(props planningRetirementHostProps) ui.Node {
	_ = props.Tick
	return retirementTile(props.Base)
}

func renderPlanningRetirementHost(f *render.Fixture, base string, tick int) {
	f.Rerender(ui.CreateElement(planningRetirementHost, planningRetirementHostProps{Base: base, Tick: tick}))
	f.Stabilize()
}

func mountPlanningRetirementHost(t *testing.T, app *appstate.App, plan uistate.RetirementPlan) *render.Fixture {
	t.Helper()
	previousApp := appstate.Default
	previousPlan := uistate.SettingKVGet("cashflux:retirement-plan")
	previousInflation := uistate.SettingKVGet("cashflux:inflation-pct")
	appstate.Default = app
	uistate.SaveRetirementPlan(plan)
	retirementFactsCache.Clear()

	f := render.New(t)
	t.Cleanup(func() {
		// Tear down every mounted hook before restoring global state or closing the
		// fixture store, so no render can read the prior app or a closed test app.
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
		retirementFactsCache.Clear()
		uistate.FlushPendingSettingsPersist()
		appstate.Default = previousApp
		restorePlanningRetirementSetting("cashflux:retirement-plan", previousPlan)
		restorePlanningRetirementSetting("cashflux:inflation-pct", previousInflation)
		uistate.FlushPendingSettingsPersist()
	})
	return f
}

func restorePlanningRetirementSetting(key, value string) {
	if value == "" {
		uistate.SettingKVDelete(key)
		return
	}
	uistate.SettingKVSet(key, value)
}

func planningRetirementHostNode(f *render.Fixture, testID string) *render.QueryNode {
	for _, tag := range []string{"p", "div", "span"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == testID {
				return node
			}
		}
	}
	return nil
}

func putPlanningRetirementHostSettings(t *testing.T, app *appstate.App, rates map[string]float64) {
	t.Helper()
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = rates
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
}

func putPlanningRetirementHostAccount(t *testing.T, app *appstate.App, id, code string, openingMinor int64) {
	t.Helper()
	if err := app.PutAccount(domain.Account{
		ID: id, Name: id, OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeRetirement,
		Currency: code, OpeningBalance: money.New(openingMinor, code),
	}); err != nil {
		t.Fatalf("PutAccount %s: %v", id, err)
	}
}

func TestPlanningRetirementHostShowsUnavailableForMissingFXWithoutPartialPot(t *testing.T) {
	app := newPlanningRetirementHostApp(t)
	putPlanningRetirementHostSettings(t, app, nil)
	putPlanningRetirementHostAccount(t, app, "retirement-usd", "USD", 50_000)
	putPlanningRetirementHostAccount(t, app, "retirement-eur", "EUR", 100_000)
	f := mountPlanningRetirementHost(t, app, uistate.RetirementPlan{
		YearsToRetirement: 20, ReturnPct: 7, InflationPct: 3,
	})
	renderPlanningRetirementHost(f, "USD", 1)

	if planningRetirementHostNode(f, "plan-retire-unavailable") == nil {
		t.Fatalf("missing FX did not disclose unavailable retirement balances: %s", f.Text())
	}
	if planningRetirementHostNode(f, "plan-retire-basis") != nil || planningRetirementHostNode(f, "plan-retire-pot") != nil {
		t.Fatalf("missing FX left a partial or zero retirement pot visible: %s", f.Text())
	}
}

func TestPlanningRetirementHostDoesNotReuseWarmedFactsAfterStoreCloses(t *testing.T) {
	app := newPlanningRetirementHostApp(t)
	putPlanningRetirementHostSettings(t, app, nil)
	putPlanningRetirementHostAccount(t, app, "retirement-usd", "USD", 100_000)
	f := mountPlanningRetirementHost(t, app, uistate.RetirementPlan{
		YearsToRetirement: 20, ReturnPct: 7, InflationPct: 3,
	})
	renderPlanningRetirementHost(f, "USD", 1)
	if planningRetirementHostNode(f, "plan-retire-pot") == nil {
		t.Fatalf("initial retirement facts did not warm: %s", f.Text())
	}

	if err := app.Store().Close(); err != nil {
		t.Fatalf("close warmed retirement source store: %v", err)
	}
	renderPlanningRetirementHost(f, "USD", 2)
	if planningRetirementHostNode(f, "plan-retire-unavailable") == nil || planningRetirementHostNode(f, "plan-retire-pot") != nil {
		t.Fatalf("closed store retained warmed retirement facts: %s", f.Text())
	}
}

func TestPlanningRetirementHostRefreshesMountedPotAfterFXOnlySettingsChange(t *testing.T) {
	app := newPlanningRetirementHostApp(t)
	putPlanningRetirementHostSettings(t, app, map[string]float64{"EUR": 2})
	putPlanningRetirementHostAccount(t, app, "retirement-eur", "EUR", 100_000)
	f := mountPlanningRetirementHost(t, app, uistate.RetirementPlan{
		YearsToRetirement: 1, ReturnPct: 7, InflationPct: 0,
	})
	renderPlanningRetirementHost(f, "USD", 1)
	assertPlanningRetirementHostAmount(t, f, "plan-retire-basis", "$2,000.00")
	assertPlanningRetirementHostAmount(t, f, "plan-retire-pot", "$2,140.00")
	uiRevision := uistate.CurrentDataRevision()
	revision := app.Rev()

	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings with changed EUR rate: %v", err)
	}
	if app.Rev() <= revision {
		t.Fatal("FX-only settings write did not advance the app revision")
	}
	if got := uistate.CurrentDataRevision(); got != uiRevision {
		t.Fatalf("FX-only settings write unexpectedly changed the UI revision: before=%d after=%d", uiRevision, got)
	}

	// Force the mounted host through a render at the unchanged UI revision. The
	// prepared cache must see the newer app/store revision and exact FX table.
	renderPlanningRetirementHost(f, "USD", 2)
	if got := uistate.CurrentDataRevision(); got != uiRevision {
		t.Fatalf("retirement rerender changed the UI revision: before=%d after=%d", uiRevision, got)
	}
	assertPlanningRetirementHostAmount(t, f, "plan-retire-basis", "$3,000.00")
	assertPlanningRetirementHostAmount(t, f, "plan-retire-pot", "$3,210.00")
}

func assertPlanningRetirementHostAmount(t *testing.T, f *render.Fixture, testID, amount string) {
	t.Helper()
	node := planningRetirementHostNode(f, testID)
	if node == nil || !strings.Contains(node.Text(), amount) {
		t.Fatalf("%s did not contain %q: %s", testID, amount, f.Text())
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type householdTestControls struct {
	Period     state.Atom[period.Window]
	Edit       state.Atom[uistate.MemberEdit]
	Standalone func(bool)
}

func householdTestHost(controls *householdTestControls) ui.Node {
	controls.Period = uistate.UsePeriod()
	controls.Edit = uistate.UseMemberEdit()
	standalone := ui.UseState(false)
	controls.Standalone = standalone.Set
	if standalone.Get() {
		return ui.CreateElement(Members)
	}
	return ui.CreateElement(HouseholdHub, householdHubProps{})
}

func householdTestFixture(t *testing.T) (*appstate.App, *render.Fixture, *householdTestControls, *int) {
	t.Helper()
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(previousCSS) })
	app := primaryTodoNotificationsApp(t)
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, member := range []domain.Member{{ID: "alice", Name: "Alice"}, {ID: "bob", Name: "Bob", IsDefault: true}} {
		if err := app.Store().PutMember(member); err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range []domain.Account{
		{ID: "alice-bank", Name: "Alice bank", OwnerID: "alice", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, Scope: domain.ScopeShared, OpeningBalance: money.New(1000, "USD")},
		{ID: "bob-bank", Name: "Bob bank", OwnerID: "bob", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, Scope: domain.ScopeShared, OpeningBalance: money.New(2000, "USD")},
		{ID: "shared-bank", Name: "Shared bank", OwnerID: domain.GroupOwnerID, Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, Scope: domain.ScopeShared, OpeningBalance: money.New(500, "USD")},
	} {
		if err := app.Store().PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	date := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	for _, txn := range []domain.Transaction{{ID: "income", AccountID: "alice-bank", Date: date, Amount: money.New(101, "USD")}, {ID: "expense", AccountID: "alice-bank", Date: date, Amount: money.New(-101, "USD"), MemberIDs: []string{"alice", "bob"}}} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	// render.New resets the runtime atom registry. Seed the actual shared
	// period only after that reset, before the first production host render.
	f := render.New(t)
	window := state.NewGlobalAtom("dashboard:period", period.NewWindow(period.Month, date, time.Monday))
	oldWindow := window.Get()
	window.Set(period.NewWindow(period.Month, date, time.Monday))
	t.Cleanup(func() { window.Set(oldWindow) })
	previousObserver := householdPrepareObserver
	builds := new(int)
	householdPrepareObserver = func() {
		*builds++
		t.Logf("household preparation %d: app=%d storeRev=%d notificationRev=%d", *builds, app.InstanceID(), app.Rev(), uistate.CurrentDataRevision())
	}
	householdPreparedCache.Clear()
	t.Cleanup(func() { householdPrepareObserver = previousObserver; householdPreparedCache.Clear() })
	controls := &householdTestControls{}
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	f.Render(ui.CreateElement(householdTestHost, controls))
	f.Stabilize()
	start, end := controls.Period.Get().Range()
	if !start.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("production host did not observe the seeded June period: %s .. %s", start, end)
	}
	return app, f, controls, builds
}

func householdAssertText(t *testing.T, f *render.Fixture, tag, id, want string) {
	t.Helper()
	node := byTestID(f, tag, id)
	if node == nil || !strings.Contains(node.Text(), want) {
		t.Fatalf("%s expected %q: %s", id, want, f.Text())
	}
}

func TestHouseholdPreparedOrdinaryParitySharedAcrossViewsAndPeriod(t *testing.T) {
	_, f, controls, builds := householdTestFixture(t)
	householdAssertText(t, f, "div", "hh-worth-total", fmtMoney(money.New(3500, "USD")))
	householdAssertText(t, f, "span", "member-worth-alice", fmtMoney(money.New(1000, "USD")))
	householdAssertText(t, f, "span", "member-spent-alice", fmtMoney(money.New(51, "USD")))
	householdAssertText(t, f, "span", "member-share-alice", "50%")
	if *builds != 1 {
		t.Fatalf("hero/roster prepared %d models, want one", *builds)
	}
	primaryClick(t, f, "hh-tab-byperson")
	for _, value := range []struct {
		id     string
		amount int64
	}{{"worth/member/alice", 1000}, {"spend/member/alice", 51}, {"spend/member/bob", 50}, {"income/member/alice", 51}, {"income/member/bob", 50}} {
		householdAssertText(t, f, "span", "hh-amount-"+value.id, fmtMoney(money.New(value.amount, "USD")))
	}
	primaryClick(t, f, "hh-tab-split")
	primaryClick(t, f, "hh-tab-members")
	primaryClick(t, f, "hh-tab-byperson")
	if *builds != 1 {
		t.Fatalf("tab changes repeated native preparation: %d", *builds)
	}
	controls.Period.Set(period.NewWindow(period.Month, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), time.Monday))
	f.Stabilize()
	if *builds != 2 || byTestID(f, "button", "hh-tab-byperson").Attr("aria-checked") != "true" {
		t.Fatalf("period refresh lost tab or failed to invalidate: builds%d", *builds)
	}
	householdAssertText(t, f, "div", "hh-worth-total", fmtMoney(money.New(3500, "USD")))
	householdAssertText(t, f, "span", "hh-amount-spend/member/alice", fmtMoney(money.Zero("USD")))
	controls.Standalone(true)
	f.Stabilize()
	householdAssertText(t, f, "span", "member-worth-alice", fmtMoney(money.New(1000, "USD")))
	householdAssertText(t, f, "span", "hh-amount-spend/member/alice", fmtMoney(money.Zero("USD")))
	if *builds != 2 {
		t.Fatalf("standalone Members recomputed the identical snapshot: %d", *builds)
	}
}

func TestHouseholdPreparedMissingFXRestoresIndependently(t *testing.T) {
	app, f, _, builds := householdTestFixture(t)
	foreign := domain.Account{ID: "foreign", Name: "Foreign", OwnerID: "alice", Currency: "EUR", Class: domain.ClassAsset, Type: domain.TypeChecking, Scope: domain.ScopeShared, OpeningBalance: money.New(100, "EUR")}
	if err := app.Store().PutAccount(foreign); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	householdAssertText(t, f, "div", "hh-worth-total", uistate.T("calculation.unavailable"))
	householdAssertText(t, f, "span", "member-worth-alice", uistate.T("calculation.unavailable"))
	householdAssertText(t, f, "span", "member-spent-alice", fmtMoney(money.New(51, "USD")))
	if byTestID(f, "p", "hh-worth-unavailable") == nil || byTestID(f, "p", "hh-spend-unavailable") != nil || byTestID(f, "p", "hh-income-unavailable") != nil {
		t.Fatal("worth FX error hid independent spending/income or its source explanation")
	}
	primaryClick(t, f, "hh-tab-byperson")
	householdAssertText(t, f, "span", "hh-amount-worth/member/alice", uistate.T("calculation.unavailable"))
	householdAssertText(t, f, "span", "hh-amount-income/member/alice", fmtMoney(money.New(51, "USD")))
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	householdAssertText(t, f, "div", "hh-worth-total", fmtMoney(money.New(3700, "USD")))
	householdAssertText(t, f, "span", "hh-amount-worth/member/alice", fmtMoney(money.New(1200, "USD")))
	if byTestID(f, "p", "hh-worth-unavailable") != nil || *builds != 3 {
		t.Fatalf("restored rates retained error/rebuilt unnecessarily: %d", *builds)
	}
}

func TestHouseholdPreparedOverflowRetainsKnownIndividualRows(t *testing.T) {
	app, f, _, _ := householdTestFixture(t)
	for _, id := range []string{"income", "expense"} {
		if _, err := app.Store().DeleteTransaction(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"alice-bank", "bob-bank", "shared-bank"} {
		account, _, err := app.Store().GetAccount(id)
		if err != nil {
			t.Fatal(err)
		}
		switch id {
		case "alice-bank":
			account.OpeningBalance = money.New(math.MaxInt64, "USD")
		case "bob-bank":
			account.OpeningBalance = money.New(1, "USD")
		default:
			account.Archived = true
		}
		if err := app.Store().PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	for _, txn := range []domain.Transaction{{ID: "big", Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), MemberID: "alice", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "one", Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), MemberID: "bob", Amount: money.New(-1, "USD")}} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	householdAssertText(t, f, "div", "hh-worth-total", uistate.T("calculation.unavailable"))
	householdAssertText(t, f, "span", "member-worth-alice", fmtMoney(money.New(math.MaxInt64, "USD")))
	householdAssertText(t, f, "span", "member-spent-alice", fmtMoney(money.New(math.MaxInt64, "USD")))
	if byTestID(f, "p", "hh-worth-unavailable") == nil || byTestID(f, "p", "hh-spend-unavailable") == nil {
		t.Fatal("overflow total was not disclosed")
	}
	primaryClick(t, f, "hh-tab-byperson")
	householdAssertText(t, f, "span", "hh-amount-worth/member/bob", fmtMoney(money.New(1, "USD")))
	householdAssertText(t, f, "span", "hh-amount-spend/member/bob", fmtMoney(money.New(1, "USD")))
}

func TestHouseholdPreparedClosedSourceShowsErrorInsteadOfEmptyRoster(t *testing.T) {
	app, f, controls, _ := householdTestFixture(t)
	start, end := controls.Period.Get().Range()
	warm := householdPreparedAt(app, start, end)
	if warm.SourceErr != nil {
		t.Fatal(warm.SourceErr)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if failed := householdPreparedAt(app, start, end); failed.SourceErr == nil || failed.Model.RosterErr == nil || failed.Model.Worth.Total.Err == nil {
		t.Fatal("warm model hid a fresh source failure")
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	householdAssertText(t, f, "div", "hh-worth-total", uistate.T("calculation.unavailable"))
	if byTestID(f, "p", "members-roster-unavailable") == nil || strings.Contains(f.Text(), uistate.T("members.empty")) {
		t.Fatalf("read error was rendered as an empty roster: %s", f.Text())
	}
	primaryClick(t, f, "hh-tab-byperson")
	if byTestID(f, "p", "hh-by-person-roster-unavailable") == nil || strings.Contains(f.Text(), uistate.T("household.byPersonEmpty")) {
		t.Fatalf("read error became by-person empty CTA: %s", f.Text())
	}
}

func TestHouseholdPreparedRevisionRefreshKeepsKeyedMenuAndTargets(t *testing.T) {
	app, f, controls, builds := householdTestFixture(t)
	assertBuilds := func(stage string, want int) {
		t.Helper()
		if *builds != want {
			t.Fatalf("%s: preparations=%d, want%d; storeRev=%d notificationRev=%d", stage, *builds, want, app.Rev(), uistate.CurrentDataRevision())
		}
	}
	assertBuilds("initial hero and roster", 1)
	primaryClick(t, f, "member-menu-btn-alice")
	assertBuilds("menu-only state", 1)
	uistate.BumpDataRevision()
	f.Stabilize()
	assertBuilds("notification without source change", 1)
	toggle := byTestID(f, "button", "member-menu-btn-alice")
	identity := toggle.NodeID()
	if toggle.Attr("aria-expanded") != "true" {
		t.Fatal("member menu did not open")
	}
	alice, _, err := app.Store().GetMember("alice")
	if err != nil {
		t.Fatal(err)
	}
	alice.Name = "Renamed Alice"
	if err := app.Store().PutMember(alice); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertBuilds("committed rename", 2)
	toggle = byTestID(f, "button", "member-menu-btn-alice")
	if toggle.NodeID() != identity || toggle.Attr("aria-expanded") != "true" || !strings.Contains(byTestID(f, "div", "member-row-alice").Text(), "Renamed Alice") {
		t.Fatal("revision/rename moved row menu identity or retained stale roster")
	}
	primaryClick(t, f, "member-make-default-alice")
	assertBuilds("default member command", 3)
	if app.DefaultMemberID() != "alice" || byTestID(f, "span", "member-default-chip-alice") == nil || byTestID(f, "button", "member-make-default-alice") != nil {
		t.Fatal("default action failed to refresh the shared prepared roster")
	}
	if byTestID(f, "button", "member-menu-btn-alice").NodeID() != identity {
		t.Fatal("conditional action change remounted the member menu")
	}
	if err := app.Store().PutMember(domain.Member{ID: "0-zero", Name: "Inserted before Alice"}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertBuilds("preceding member insertion", 4)
	if byTestID(f, "button", "member-menu-btn-alice").NodeID() != identity {
		t.Fatal("new preceding member moved another row's state")
	}
	primaryClick(t, f, "member-set-pin-alice")
	if controls.Edit.Get().ID != "alice" || controls.Edit.Get().Mode != uistate.MemberEditModePIN {
		t.Fatal("conditional menu hooks changed the PIN target")
	}
	assertBuilds("PIN editor selection without financial mutation", 4)
}

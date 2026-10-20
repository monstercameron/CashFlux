// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestDashboardHeroHandlesUnavailableEmptyPopulatedTransitions(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	previous := appstate.Default
	appstate.Default = nil
	defer func() { appstate.Default = previous }()

	var scopeAtom state.Atom[scope.ReportScope]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		scopeAtom = uistate.UseActiveScope()
		return ui.CreateElement(dashboardHero)
	}))
	previousScope := scopeAtom.Get()
	defer scopeAtom.Set(previousScope)
	scopeAtom.Set(scope.ReportScope{})
	if f.Text() != "" {
		t.Fatalf("unavailable app rendered a hero: %s", f.Text())
	}

	appstate.Default = app
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "button", "hero-load-sample") == nil {
		t.Fatalf("empty app did not render the welcome hero: %s", f.Text())
	}
	account := domain.Account{ID: "cash", Name: "Cash", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(12345, "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "button", "hero-load-sample") != nil || byTestID(f, "button", "hero-add-txn") == nil || !strings.Contains(f.Text(), "123.45") {
		t.Fatalf("populated app did not render a fresh financial summary: %s", f.Text())
	}
	if err := app.DeleteAccount(account.ID); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "button", "hero-load-sample") == nil || byTestID(f, "button", "hero-add-txn") != nil {
		t.Fatalf("wiped app retained the populated hero: %s", f.Text())
	}
	appstate.Default = nil
	uistate.BumpDataRevision()
	f.Stabilize()
	if f.Text() != "" {
		t.Fatalf("unavailable app retained hero content: %s", f.Text())
	}
}

func TestDashboardHeroInvalidatesTotalsForAmbiguousInstitutionNames(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	previous := appstate.Default
	appstate.Default = app
	defer func() { appstate.Default = previous }()
	now := time.Now()
	for i, institution := range []string{"A B", "A", "B"} {
		id := institution
		if err := app.PutAccount(domain.Account{ID: id, Name: id, Institution: institution, Scope: domain.ScopeShared,
			OwnerID: domain.GroupOwnerID, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutTransaction(domain.Transaction{ID: "pay-" + id, AccountID: id, Desc: "Paycheck", Date: now, Amount: money.New(int64(i+1)*10000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	var scopeAtom state.Atom[scope.ReportScope]
	var periodAtom state.Atom[period.Window]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		scopeAtom = uistate.UseActiveScope()
		periodAtom = uistate.UsePeriod()
		return ui.CreateElement(dashboardHero)
	}))
	previousScope := scopeAtom.Get()
	defer scopeAtom.Set(previousScope)
	previousPeriod := periodAtom.Get()
	defer periodAtom.Set(previousPeriod)
	periodAtom.Set(period.NewWindow(period.Month, now, time.Monday))
	// fmt's %v rendered both lists as [A B], retaining the old period totals.
	scopeAtom.Set(scope.ReportScope{Institutions: []string{"A B"}})
	f.Stabilize()
	first := f.Text()
	if !strings.Contains(first, "100.00") || strings.Contains(first, "500.00") {
		t.Fatalf("first institution scope = %s", first)
	}
	scopeAtom.Set(scope.ReportScope{Institutions: []string{"A", "B"}})
	f.Stabilize()
	second := f.Text()
	if !strings.Contains(second, "500.00") || strings.Contains(second, "100.00") {
		t.Fatalf("different institution scope retained old totals: %s", second)
	}
}

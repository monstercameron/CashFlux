// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func newWhatChangedHostApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

func mountWhatChangedHost(t *testing.T, app *appstate.App) *render.Fixture {
	t.Helper()
	previousApp := appstate.Default
	appstate.Default = app
	// App writes normalize transaction dates to midnight UTC. Keep the visit
	// baseline one second before today's normalized date so this fixture stays
	// inside the report window regardless of the time of day the wasm lane runs.
	now := time.Now()
	baseline := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(-time.Second)
	uistate.MarkVisitCaughtUp(baseline.Unix())

	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
		appstate.Default = previousApp
	})
	return f
}

func putWhatChangedHostLedger(t *testing.T, app *appstate.App, accountCurrency string, openingMinor, transactionMinor int64, fxRates map[string]float64) {
	t.Helper()
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = fxRates
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	account := domain.Account{
		ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: accountCurrency, OpeningBalance: money.New(openingMinor, accountCurrency),
	}
	if err := app.PutAccount(account); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "recent-income", AccountID: account.ID, Date: time.Now(),
		Desc: "Paycheck", Payee: "Employer", Amount: money.New(transactionMinor, accountCurrency),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
}

func whatChangedHostNode(f *render.Fixture, testID string) *render.QueryNode {
	for _, tag := range []string{"div", "p", "span"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == testID {
				return node
			}
		}
	}
	return nil
}

func waitForWhatChangedHost(t *testing.T, f *render.Fixture, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	f.Stabilize()
	if !ready() {
		t.Fatalf("timed out waiting for %s: %s", description, f.Text())
	}
}

func waitForWhatChangedSettle(t *testing.T, f *render.Fixture, mountedAt time.Time) {
	t.Helper()
	// The real card defers its data read for 300ms. Poll with a deadline so the
	// test yields to that browser timer rather than assuming Stabilize runs it.
	deadline := mountedAt.Add(3 * time.Second)
	settledAfter := mountedAt.Add(350 * time.Millisecond)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if !time.Now().Before(settledAfter) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("What Changed host did not reach its settle window")
}

func assertWhatChangedNetWorthAmount(t *testing.T, f *render.Fixture, want string) {
	t.Helper()
	row := whatChangedHostNode(f, "wc-row-0-networth")
	if row == nil || !strings.Contains(row.Text(), want) {
		t.Fatalf("net-worth row did not contain %q: %s", want, f.Text())
	}
}

type whatChangedHostTickProps struct {
	Tick int
}

func whatChangedHostWithTick(p whatChangedHostTickProps) ui.Node {
	_ = p.Tick
	// Invoke the production card body during this component render. Changing the
	// wrapper's prop forces a real render pass while leaving the UI data revision
	// untouched; a same-props Rerender is intentionally allowed to bail out in v7.
	return dashWhatChangedCard()
}

func TestWhatChangedHostReplacesWarmedCardWithUnavailableAfterStoreCloses(t *testing.T) {
	app := newWhatChangedHostApp(t)
	putWhatChangedHostLedger(t, app, "USD", 100_000, 1_000, nil)
	f := mountWhatChangedHost(t, app)
	f.Render(ui.CreateElement(dashWhatChangedCard))
	waitForWhatChangedHost(t, f, "the warmed What Changed card", func() bool {
		return whatChangedHostNode(f, "dash-whatchanged-card") != nil && whatChangedHostNode(f, "wc-row-0-networth") != nil
	})

	if err := app.Store().Close(); err != nil {
		t.Fatalf("close warmed source store: %v", err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if whatChangedHostNode(f, "dash-whatchanged-unavailable") == nil || whatChangedHostNode(f, "dash-whatchanged-card") != nil {
		t.Fatalf("closed sources retained a stale What Changed card: %s", f.Text())
	}
}

func TestWhatChangedHostReadsReplacementAppAtSameUIRevision(t *testing.T) {
	appA, appB := newWhatChangedHostApp(t), newWhatChangedHostApp(t)
	// The event date and payee are the same. Different activity amounts produce
	// different net-worth movement; opening balances alone would not.
	putWhatChangedHostLedger(t, appA, "USD", 100_000, 1_000, nil)
	putWhatChangedHostLedger(t, appB, "USD", 200_000, 2_500, nil)
	if appA.Rev() != appB.Rev() {
		t.Fatalf("fixture revisions differ before mount: app A=%d app B=%d", appA.Rev(), appB.Rev())
	}
	f := mountWhatChangedHost(t, appA)
	f.Render(ui.CreateElement(whatChangedHostWithTick, whatChangedHostTickProps{}))
	waitForWhatChangedHost(t, f, "app A's What Changed card", func() bool {
		return whatChangedHostNode(f, "dash-whatchanged-card") != nil && whatChangedHostNode(f, "wc-row-0-networth") != nil
	})
	assertWhatChangedNetWorthAmount(t, f, "$10.00")

	uiRevision := uistate.CurrentDataRevision()
	appstate.Default = appB
	// Change only the test wrapper prop to force the mounted card body to execute
	// against app B. App replacement itself leaves the UI data revision unchanged.
	f.Rerender(ui.CreateElement(whatChangedHostWithTick, whatChangedHostTickProps{Tick: 1}))
	f.Stabilize()
	if got := uistate.CurrentDataRevision(); got != uiRevision {
		t.Fatalf("replacement changed the UI data revision: before=%d after=%d", uiRevision, got)
	}
	if whatChangedHostNode(f, "dash-whatchanged-card") == nil {
		t.Fatalf("replacement app became unavailable: %s", f.Text())
	}
	assertWhatChangedNetWorthAmount(t, f, "$25.00")
}

func TestWhatChangedHostRecomputesMountedAmountAfterFXOnlySettingsChange(t *testing.T) {
	app := newWhatChangedHostApp(t)
	putWhatChangedHostLedger(t, app, "EUR", 100_000, 10_000, map[string]float64{"EUR": 2})
	f := mountWhatChangedHost(t, app)
	f.Render(ui.CreateElement(dashWhatChangedCard))
	waitForWhatChangedHost(t, f, "the initial FX-converted What Changed card", func() bool {
		return whatChangedHostNode(f, "dash-whatchanged-card") != nil && whatChangedHostNode(f, "wc-row-0-networth") != nil
	})
	assertWhatChangedNetWorthAmount(t, f, "$200.00")

	accountsBefore, err := app.AccountsChecked()
	if err != nil {
		t.Fatalf("AccountsChecked before rate change: %v", err)
	}
	transactionsBefore, err := app.TransactionsChecked()
	if err != nil {
		t.Fatalf("TransactionsChecked before rate change: %v", err)
	}
	revisionBefore := app.Rev()
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings with the new EUR rate: %v", err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if app.Rev() <= revisionBefore {
		t.Fatal("settings write did not advance the app revision")
	}
	accountsAfter, err := app.AccountsChecked()
	if err != nil {
		t.Fatalf("AccountsChecked after rate change: %v", err)
	}
	transactionsAfter, err := app.TransactionsChecked()
	if err != nil {
		t.Fatalf("TransactionsChecked after rate change: %v", err)
	}
	if !reflect.DeepEqual(accountsBefore, accountsAfter) || !reflect.DeepEqual(transactionsBefore, transactionsAfter) {
		t.Fatal("FX-only settings change altered account or transaction sources")
	}
	assertWhatChangedNetWorthAmount(t, f, "$300.00")
}

func TestWhatChangedHostTracksExactTimestampAcrossUntilCutoff(t *testing.T) {
	app := newWhatChangedHostApp(t)
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	account := domain.Account{
		ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: money.New(100_000, "USD"),
	}
	if err := app.PutAccount(account); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	f := mountWhatChangedHost(t, app)
	// PutTransaction intentionally normalizes UI-entered dates to midnight. Use
	// the raw store here to represent an imported exact instant. The component
	// samples time.Now internally, so this checks the instant on either side of
	// the live Until cutoff without adding a production clock seam.
	futureCutoff := time.Now().Add(2 * time.Second)
	if err := app.Store().PutTransaction(domain.Transaction{
		ID: "future-income", AccountID: account.ID, Date: futureCutoff,
		Desc: "Future deposit", Payee: "Employer", Amount: money.New(4_200, "USD"),
	}); err != nil {
		t.Fatalf("Store.PutTransaction with exact timestamp: %v", err)
	}
	mountedAt := time.Now()
	f.Render(ui.CreateElement(dashWhatChangedCard))
	waitForWhatChangedSettle(t, f, mountedAt)
	f.Stabilize()
	if whatChangedHostNode(f, "dash-whatchanged-card") != nil || whatChangedHostNode(f, "dash-whatchanged-slot") == nil {
		t.Fatalf("a transaction after the exact Until cutoff appeared early: %s", f.Text())
	}

	deadline := futureCutoff.Add(time.Second)
	for time.Now().Before(futureCutoff) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if time.Now().Before(futureCutoff) {
		t.Fatal("test did not reach the exact transaction timestamp before its deadline")
	}
	uistate.BumpDataRevision()
	waitForWhatChangedHost(t, f, "the transaction after its timestamp enters the window", func() bool {
		return whatChangedHostNode(f, "dash-whatchanged-card") != nil && whatChangedHostNode(f, "wc-row-0-networth") != nil
	})
	assertWhatChangedNetWorthAmount(t, f, "$42.00")
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cardgraph"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func widgetBuilderHostApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	previousApp := appstate.Default
	appstate.Default = app
	vbDataCache.Lock()
	previousKey, previousPrepared, previousValid := vbDataCache.key, vbDataCache.prepared, vbDataCache.valid
	vbDataCache.valid = false
	vbDataCache.Unlock()
	t.Cleanup(func() {
		appstate.Default = previousApp
		vbDataCache.Lock()
		vbDataCache.key, vbDataCache.prepared, vbDataCache.valid = previousKey, previousPrepared, previousValid
		vbDataCache.Unlock()
		_ = app.Store().Close()
	})
	return app
}

func TestWidgetBuilderPreparedHostKeepsLiteralAndRejectsUnavailableNetWorthSource(t *testing.T) {
	app := widgetBuilderHostApp(t)
	if err := app.Store().PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if err := app.Store().PutAccount(domain.Account{
		ID: "eur", Name: "Euro account", Type: domain.TypeChecking, Currency: "EUR",
		OpeningBalance: money.New(100_00, "EUR"),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.Store().PutTransaction(domain.Transaction{
		ID: "eur-expense", AccountID: "eur", Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-12_34, "EUR"), Payee: "Market",
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	literal := cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "value", Kind: cardgraph.KindLiteralNumber, Props: map[string]string{"value": "7"}},
			{ID: "kpi", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Value", "format": "number"}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "value", Port: cardgraph.OutPort}, To: cardgraph.PortRef{Node: "kpi", Port: "value"}}},
		Root:  "kpi",
	}
	result, prepared, err := vbEvalGraph(literal, app, now)
	if err != nil {
		t.Fatalf("literal graph with missing EUR rate: %v", err)
	}
	if result.Render == nil {
		t.Fatalf("literal graph was blocked by unrelated net-worth availability: %#v", result.Issues)
	}
	if len(prepared.Datasets["transactions"].Rows) != 1 {
		t.Fatalf("transaction source rows = %d, want 1", len(prepared.Datasets["transactions"].Rows))
	}
	if prepared.NetWorthSeriesErr == nil {
		t.Fatal("missing EUR rate did not report unavailable net-worth series")
	}

	seriesGraph := cardgraph.Graph{Nodes: []cardgraph.Node{{
		ID: "series", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "net_worth_series"},
	}}, Root: "series"}
	result, _, err = vbEvalGraph(seriesGraph, app, now)
	if err != nil {
		t.Fatalf("net-worth graph should return a preview issue, not a snapshot read error: %v", err)
	}
	if result.Render != nil || len(result.Issues) == 0 || !strings.Contains(result.Issues[0].Message, "net worth") {
		t.Fatalf("unavailable net-worth source result = %#v, want explicit preview issue", result)
	}
}

func TestWidgetBuilderPreparedHostInvalidatesByRevisionAndDoesNotHideReadErrors(t *testing.T) {
	app := widgetBuilderHostApp(t)
	if err := app.Store().PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if err := app.Store().PutAccount(domain.Account{
		ID: "checking", Name: "Checking", Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100_00, "USD"),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	first, err := vbPreparedData(app, now)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if got := len(first.Datasets["transactions"].Rows); got != 0 {
		t.Fatalf("initial transactions = %d, want 0", got)
	}

	txn := domain.Transaction{ID: "expense", AccountID: "checking", Date: now, Amount: money.New(-1_00, "USD")}
	if err := app.Store().PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	second, err := vbPreparedData(app, now)
	if err != nil {
		t.Fatalf("snapshot after revision: %v", err)
	}
	if got := len(second.Datasets["transactions"].Rows); got != 1 {
		t.Fatalf("transactions after revision = %d, want 1", got)
	}
	if got := second.Datasets["transactions"].Rows[0]["amount"].Num; got != 1 {
		t.Fatalf("cached transaction amount = %v, want 1", got)
	}

	if err := app.Store().Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if _, err := vbPreparedData(app, now); err == nil {
		t.Fatal("closed store returned a cached snapshot instead of reporting the settings read failure")
	}
}

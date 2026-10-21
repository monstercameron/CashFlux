// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func assistantDataTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsData(app) {
		if tool.spec.Function.Name == name {
			if tool.mutates || tool.preview != nil {
				t.Fatal("read tool requests mutation", name)
			}
			return tool
		}
	}
	t.Fatal("missing data tool", name)
	return chatTool{}
}

func TestAssistantDataCatalogReadsEveryRetainedCollectionAndWhitelistsSettings(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	// Seed every typed persisted entity, including collection types not present
	// in the short chat prompt. Load is the store's real hydration seam.
	ds := store.Dataset{Settings: store.Settings{BaseCurrency: "USD", OpenAIKey: "DO_NOT_LEAK_API", OpenAIBaseURL: "https://DO_NOT_LEAK_ENDPOINT", FXRates: map[string]float64{"EUR": 1.25}}}
	v := reflect.ValueOf(&ds).Elem()
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Slice || typ.Field(i).Name == "TransferOperations" {
			continue
		}
		row := reflect.New(f.Type().Elem()).Elem()
		if row.Kind() != reflect.Struct {
			continue
		}
		if id := row.FieldByName("ID"); id.IsValid() && id.Kind() == reflect.String {
			id.SetString("coverage-" + typ.Field(i).Name)
		}
		if name := row.FieldByName("Name"); name.IsValid() && name.Kind() == reflect.String {
			name.SetString("Coverage " + typ.Field(i).Name)
		}
		if key := row.FieldByName("Key"); key.IsValid() && key.Kind() == reflect.String {
			key.SetString("financial_custom_key")
		}
		f.Set(reflect.Append(f, row))
	}
	ds.Accounts = []domain.Account{{ID: "account", Name: "Archived euro", Currency: "EUR", Class: domain.ClassAsset, Type: domain.TypeChecking, OpeningBalance: money.New(100, "EUR"), Archived: true}}
	ds.Artifacts = []domain.Artifact{{ID: "artifact", Name: "Financial rows", Kind: "csv", Bytes: []byte("DO_NOT_LEAK_BINARY"), Columns: []string{"date", "amount"}, Rows: [][]string{{"2026-10-20", "12.34"}}}}
	ds.Documents = []domain.Document{{ID: "document", Filename: "statement.csv", Extracted: []domain.DocumentRow{{Date: "2026-10-20", Description: "Financial row DO_NOT_LEAK_API", Amount: "12.34"}}}}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().SetSettingKV("cashflux:saved-scopes", `{"view":"{\"id\":\"view\",\"name\":\"Saved scope\"}"}`); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().SetSettingKV("cashflux:saved-txn-views", `{"txview":"{\"id\":\"txview\",\"name\":\"Saved ledger\"}"}`); err != nil {
		t.Fatal(err)
	}
	manifest := assistantDataTool(t, app, "data_manifest")
	out := manifest.run(json.RawMessage(`{}`))
	var m struct {
		Collections []struct {
			Collection string `json:"collection"`
			Total      *int   `json:"total"`
			Error      string `json:"error"`
		} `json:"collections"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(out, err)
	}
	if len(m.Collections) != 42 {
		t.Fatalf("catalog count %d: %s", len(m.Collections), out)
	}
	query := assistantDataTool(t, app, "query_data")
	for _, c := range m.Collections {
		if c.Error != "" || c.Total == nil || *c.Total < 1 {
			t.Errorf("collection unavailable: %+v", c)
			continue
		}
		raw := query.run(json.RawMessage(fmt.Sprintf(`{"collection":%q}`, c.Collection)))
		if strings.Contains(raw, "DO_NOT_LEAK") {
			t.Errorf("private content leaked in %s: %s", c.Collection, raw)
		}
		var p assistantdata.Page
		if err := json.Unmarshal([]byte(raw), &p); err != nil || len(p.Records) < 1 {
			t.Errorf("collection %s not readable: %s", c.Collection, raw)
		}
	}
	for _, name := range []string{"accounts", "artifacts", "documents", "financial_settings", "custom_field_definitions", "audit_entries"} {
		raw := query.run(json.RawMessage(fmt.Sprintf(`{"collection":%q}`, name)))
		switch name {
		case "accounts":
			if !strings.Contains(raw, `"archived":true`) {
				t.Error("archived source omitted", raw)
			}
		case "artifacts":
			if !strings.Contains(raw, "12.34") || strings.Contains(raw, `"bytes"`) {
				t.Error(raw)
			}
		case "documents":
			if !strings.Contains(raw, "Financial row") {
				t.Error(raw)
			}
		case "financial_settings":
			if !strings.Contains(raw, `"EUR":1.25`) || strings.Contains(strings.ToLower(raw), "openaikey") {
				t.Error(raw)
			}
		case "custom_field_definitions":
			if !strings.Contains(raw, "financial_custom_key") {
				t.Error(raw)
			}
		case "audit_entries":
			if !strings.Contains(raw, `"ID":"coverage-AuditEntries"`) {
				t.Error(raw)
			}
		}
	}
	stored, err := app.Store().GetSettings()
	if err != nil || stored.OpenAIKey != "DO_NOT_LEAK_API" {
		t.Fatal("read mutated settings", stored, err)
	}
}

func TestAssistantDataToolsUseFreshStateAndRejectStalePages(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.Store().PutAccount(domain.Account{ID: "eur", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 63; i++ {
		if err := app.Store().PutTransaction(domain.Transaction{ID: fmt.Sprintf("row%03d", i), AccountID: "eur", Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Desc: "Beyond short prompt", Amount: money.New(math.MaxInt64, "EUR")}); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantDataTool(t, app, "query_data")
	raw := tool.run(json.RawMessage(`{"collection":"transactions","limit":50,"equals":{"amount.Currency":"EUR"},"from":"2026-10-20","through":"2026-10-20","dateField":"date"}`))
	var first assistantdata.Page
	if err := json.Unmarshal([]byte(raw), &first); err != nil || first.FilteredTotal != 63 || len(first.Records) != 50 || first.NextOffset == nil {
		t.Fatal(raw, err)
	}
	args := fmt.Sprintf(`{"collection":"transactions","offset":50,"snapshot":%q}`, first.Snapshot)
	var second assistantdata.Page
	raw = tool.run(json.RawMessage(args))
	if err := json.Unmarshal([]byte(raw), &second); err != nil || len(second.Records) != 13 || second.NextOffset != nil {
		t.Fatal(raw, err)
	}
	if !strings.Contains(string(second.Records[12].Data), `"Amount":9223372036854775807`) {
		t.Fatal("precision lost", string(second.Records[12].Data))
	}
	// Reuse the exact same tool closure after an external store write, without
	// rendering or app.Rev subscription. It must see fresh data and reject cursors.
	if err := app.Store().PutTransaction(domain.Transaction{ID: "new-row", AccountID: "eur", Date: time.Now(), Amount: money.New(1, "EUR")}); err != nil {
		t.Fatal(err)
	}
	if got := tool.run(json.RawMessage(args)); !strings.Contains(got, "snapshot changed") {
		t.Fatal("stale page accepted", got)
	}
	var fresh assistantdata.Page
	raw = tool.run(json.RawMessage(`{"collection":"transactions","ids":["new-row"]}`))
	if err := json.Unmarshal([]byte(raw), &fresh); err != nil || fresh.Total != 64 || fresh.FilteredTotal != 1 {
		t.Fatal("fresh data unreachable", raw, err)
	}
	for _, raw := range []string{`{"collection":"transactions","limit":51}`, `{"collection":"transactions","offset":1}`, `{"collection":"transactions","sql":"SELECT *"}`, `{"collection":"settings"}`, `{"collection":"transactions"} {}`} {
		if got := tool.run(json.RawMessage(raw)); !strings.Contains(got, `"error"`) {
			t.Error("unsafe/invalid query accepted", got)
		}
	}
}

func TestAssistantDataRecordChunksExposeFinancialContentAndRejectSecrets(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	text := strings.Repeat("financial 🪙 detail\n", 1900) + " Bearer DO_NOT_LEAK_CREDENTIAL"
	if err := app.Store().PutSavedInsight(domain.SavedInsight{ID: "long", Text: text}); err != nil {
		t.Fatal(err)
	}
	query := assistantDataTool(t, app, "query_data")
	var page assistantdata.Page
	out := query.run(json.RawMessage(`{"collection":"saved_insights"}`))
	if err := json.Unmarshal([]byte(out), &page); err != nil || len(page.Records) != 1 || !page.Records[0].DetailRequired {
		t.Fatal(out, err)
	}
	read := assistantDataTool(t, app, "read_record")
	q := assistantdata.DetailRequest{Collection: "saved_insights", ID: "long", Field: "text", Limit: 2000}
	var joined strings.Builder
	for {
		args, _ := json.Marshal(q)
		out := read.run(args)
		if strings.Contains(out, "DO_NOT_LEAK") {
			t.Fatal("credential leaked", out)
		}
		var d assistantdata.Detail
		if err := json.Unmarshal([]byte(out), &d); err != nil || d.Collection == "" {
			t.Fatal(out, err)
		}
		joined.WriteString(d.JSONText)
		if d.NextOffset == nil {
			break
		}
		q.Offset = *d.NextOffset
		q.Snapshot = d.Snapshot
	}
	var decoded string
	if err := json.Unmarshal([]byte(joined.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(decoded, strings.Repeat("financial 🪙 detail\n", 1900)) || !strings.Contains(decoded, "[REDACTED CREDENTIAL]") {
		t.Fatal("record incompletely read")
	}
	if out := read.run(json.RawMessage(`{"collection":"saved_insights","id":"missing"}`)); !strings.Contains(out, "not found") {
		t.Fatal(out)
	}
	if err := app.Store().SetSettingKV("cashflux:saved-scopes", "{broken"); err != nil {
		t.Fatal(err)
	}
	if out := query.run(json.RawMessage(`{"collection":"saved_views"}`)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) {
		t.Fatal("corrupt data became healthy zero", out)
	}
}

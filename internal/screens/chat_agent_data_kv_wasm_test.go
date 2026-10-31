// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/notifyhistory"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func assistantSetJSONKV(t *testing.T, app *appstate.App, key string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().SetKV(key, string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestAssistantDataKVRetainsAllNotificationHistoryAndPoolMembershipWithoutWrites(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	p := prefs.Default()
	p.ServerToken = "copied-auth-opaque-123"
	b, _ := json.Marshal(p)
	if err := app.Store().SetSettingKV("cashflux:prefs", string(b)); err != nil {
		t.Fatal(err)
	}
	feed := make([]uistate.FeedItem, 55)
	for i := range feed {
		feed[i] = uistate.FeedItem{ID: fmt.Sprintf("feed-%03d", i), Title: "Retained event", Body: "Stored evidence " + p.ServerToken, At: int64(1000 + i), Read: i%2 == 0, SnoozedUntil: 9999999999, MemberID: "member"}
	}
	history := notifyhistory.Archive{Items: make([]notifyhistory.Record, 503)}
	for i := range history.Items {
		history.Items[i] = notifyhistory.Record{ID: fmt.Sprintf("history-%03d", i), Message: "Archived/dismissed historical alert " + p.ServerToken, At: int64(i), Read: true}
	}
	assistantSetJSONKV(t, app, "cashflux:notify:feed", feed)
	assistantSetJSONKV(t, app, "cashflux:notifhistory", history)
	assistantSetJSONKV(t, app, "cashflux:invest:pools", []uistate.InvestPool{{ID: "pool", Name: "User pool " + p.ServerToken, AccountIDs: []string{"archived-account", "removed-account"}}})
	query := assistantDataTool(t, app, "query_data")
	for _, tc := range []struct {
		name  string
		count int
	}{{"notification_feed", 55}, {"notification_history", 503}, {"investment_pools", 1}} {
		q := assistantdata.Query{Collection: tc.name, Limit: 50}
		seen := map[string]bool{}
		for {
			args, _ := json.Marshal(q)
			before := app.Rev()
			out := query.run(args)
			if app.Rev() != before {
				t.Fatal("read wrote/migrated/recapped data", tc.name)
			}
			if strings.Contains(out, p.ServerToken) {
				t.Fatal("copied credential leaked", out)
			}
			var page assistantdata.Page
			if err := json.Unmarshal([]byte(out), &page); err != nil || page.Total != tc.count || page.FilteredTotal != tc.count {
				t.Fatal(out, err)
			}
			for _, r := range page.Records {
				if seen[r.ID] {
					t.Fatal("duplicate continuation", r.ID)
				}
				seen[r.ID] = true
			}
			if page.NextOffset == nil {
				break
			}
			q.Offset = *page.NextOffset
			q.Snapshot = page.Snapshot
		}
		if len(seen) != tc.count {
			t.Fatalf("%s retained %d/%d", tc.name, len(seen), tc.count)
		}
	}
	out := query.run(json.RawMessage(`{"collection":"investment_pools"}`))
	for _, id := range []string{"archived-account", "removed-account"} {
		if !strings.Contains(out, id) {
			t.Fatal("retained membership filtered", out)
		}
	}
	for key, want := range map[string]int{"cashflux:notify:feed": 55, "cashflux:notifhistory": 503} {
		raw, _, err := app.Store().GetKV(key)
		if err != nil {
			t.Fatal(err)
		}
		if key == "cashflux:notify:feed" {
			var got []uistate.FeedItem
			_ = json.Unmarshal([]byte(raw), &got)
			if len(got) != want || got[0].Body != feed[0].Body {
				t.Fatal("feed changed on read")
			}
		} else {
			var got notifyhistory.Archive
			_ = json.Unmarshal([]byte(raw), &got)
			if len(got.Items) != want || got.Items[0].Message != history.Items[0].Message {
				t.Fatal("archive changed on read")
			}
		}
	}
}

func TestAssistantDataKVUsesExplicitStoreFreshConfigsAndPropagatesInvalidSources(t *testing.T) {
	first := primaryAccountsGoalsApp(t)
	keys := map[string]string{"allocation_config": "cashflux:allocate:config", "planning_config": "cashflux:planning:config", "bills_smart_config": "cashflux:bills:smartconfig", "notification_feed": "cashflux:notify:feed", "notification_history": "cashflux:notifhistory", "investment_pools": "cashflux:invest:pools"}
	values := map[string]string{"allocation_config": `{"amountMinor":12345,"profile":"saved:custom","mode":"fill"}`, "planning_config": `{"runwayDays":120,"runwayBufferMinor":900}`, "bills_smart_config": `{"enabled":true,"payFrequency":"weekly","minKeepMinor":500}`, "notification_feed": `[{"id":"first","title":"First workspace"}]`, "notification_history": `{"items":[{"id":"archive-first","message":"First archive"}]}`, "investment_pools": `[{"id":"pool-first","name":"First pool"}]`}
	for name, key := range keys {
		if err := first.Store().SetKV(key, values[name]); err != nil {
			t.Fatal(err)
		}
	}
	firstQuery := assistantDataTool(t, first, "query_data")
	second := primaryAccountsGoalsApp(t)
	if err := second.Store().SetKV(keys["planning_config"], `{"runwayDays":30}`); err != nil {
		t.Fatal(err)
	}
	// A different active Default must not redirect an existing tool's explicit app.
	out := firstQuery.run(json.RawMessage(`{"collection":"planning_config"}`))
	if !strings.Contains(out, `"runwayDays":120`) || !strings.Contains(out, `"stored":true`) {
		t.Fatal("global workspace leaked", out)
	}
	secondQuery := assistantDataTool(t, second, "query_data")
	out = secondQuery.run(json.RawMessage(`{"collection":"planning_config"}`))
	if !strings.Contains(out, `"runwayDays":30`) {
		t.Fatal("second workspace unavailable", out)
	}
	if err := first.Store().SetKV(keys["planning_config"], `{"runwayDays":180,"forecastMonths":24}`); err != nil {
		t.Fatal(err)
	}
	out = firstQuery.run(json.RawMessage(`{"collection":"planning_config"}`))
	if !strings.Contains(out, `"runwayDays":180`) || !strings.Contains(out, `"forecastMonths":24`) {
		t.Fatal("config snapshot stale", out)
	}
	values["planning_config"] = `{"runwayDays":180,"forecastMonths":24}`
	for name, key := range keys {
		if err := first.Store().SetKV(key, "{broken"); err != nil {
			t.Fatal(err)
		}
		before := first.Rev()
		args := json.RawMessage(fmt.Sprintf(`{"collection":%q}`, name))
		out = firstQuery.run(args)
		if first.Rev() != before {
			t.Fatal("bad read wrote fallback", name)
		}
		if !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) {
			t.Fatalf("%s corruption became a healthy default: %s", name, out)
		}
		if err := first.Store().SetKV(key, values[name]); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"allocation_config", "planning_config", "bills_smart_config"} {
		if err := first.Store().DeleteKV(keys[name]); err != nil {
			t.Fatal(err)
		}
		out = firstQuery.run(json.RawMessage(fmt.Sprintf(`{"collection":%q}`, name)))
		if !strings.Contains(out, `"stored":false`) {
			t.Fatal("missing defaults not labeled", out)
		}
		switch name {
		case "allocation_config":
			if !strings.Contains(out, `"profile":"balanced"`) || !strings.Contains(out, `"mode":"weighted"`) {
				t.Fatal(out)
			}
		case "planning_config":
			if !strings.Contains(out, `"runwayDays":60`) || !strings.Contains(out, `"forecastMonths":12`) {
				t.Fatal(out)
			}
		case "bills_smart_config":
			if !strings.Contains(out, `"payFrequency":"biweekly"`) {
				t.Fatal(out)
			}
		}
	}
	if err := first.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if out := firstQuery.run(json.RawMessage(`{"collection":"notification_feed"}`)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) {
		t.Fatal("store failure became an empty feed", out)
	}
}

func TestAssistantDataConfigGettersPreserveLegacyFallbackAndNormalization(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	for key, value := range map[string]string{"cashflux:allocate:config": `{"amountMinor":-1,"mode":"fill","profile":"saved:custom"}`, "cashflux:planning:config": `{"runwayDays":120,"runwayBufferMinor":-1}`, "cashflux:bills:smartconfig": `{"enabled":true,"payFrequency":"weekly","minKeepMinor":-1}`} {
		if err := app.Store().SetKV(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if c := uistate.AllocConfigGet(); c.AmountMinor != 0 || c.Mode != "fill" || c.Profile != "saved:custom" {
		t.Fatal(c)
	}
	if c := uistate.PlanningConfigGet(); c.RunwayDays != 120 || c.RunwayBufferMinor != 0 || c.ForecastMonths != 12 {
		t.Fatal(c)
	}
	if c := uistate.BillsSmartConfigGet(); !c.Enabled || c.PayFrequency != "weekly" || c.MinKeepMinor != 0 {
		t.Fatal(c)
	}
	for _, key := range []string{"cashflux:allocate:config", "cashflux:planning:config", "cashflux:bills:smartconfig"} {
		if err := app.Store().SetKV(key, "{broken"); err != nil {
			t.Fatal(err)
		}
	}
	if c := uistate.AllocConfigGet(); c.Mode != "weighted" || c.Profile != "balanced" {
		t.Fatal(c)
	}
	if c := uistate.PlanningConfigGet(); c.RunwayDays != 60 || c.ForecastMonths != 12 {
		t.Fatal(c)
	}
	if c := uistate.BillsSmartConfigGet(); c.PayFrequency != "biweekly" || c.Enabled {
		t.Fatal(c)
	}
}

func TestAssistantDataKVNotificationDateFiltersUseDeclaredUTCTimestamps(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC).Unix()
	assistantSetJSONKV(t, app, "cashflux:notify:feed", []uistate.FeedItem{{ID: "before", Title: "Earlier UTC day", At: start - 1}, {ID: "first", Title: "Start UTC", At: start, DueAt: start + 86399}, {ID: "last", Title: "End UTC", At: start + 86399, SnoozedUntil: start}, {ID: "after", Title: "Following UTC day", At: start + 86400}, {ID: "missing", Title: "Legacy no timestamp"}})
	assistantSetJSONKV(t, app, "cashflux:notifhistory", notifyhistory.Archive{Items: []notifyhistory.Record{{ID: "past", Message: "Past", At: start - 1}, {ID: "recorded", Message: "Recorded at UTC end", At: start + 86399}}})
	query := assistantDataTool(t, app, "query_data")
	for _, tc := range []struct {
		collection, field string
		want              int
	}{{"notification_feed", "at", 2}, {"notification_feed", "dueAt", 1}, {"notification_feed", "snoozedUntil", 1}, {"notification_history", "at", 1}} {
		args, _ := json.Marshal(assistantdata.Query{Collection: tc.collection, DateField: tc.field, From: "2026-10-20", Through: "2026-10-20"})
		out := query.run(args)
		var p assistantdata.Page
		if err := json.Unmarshal([]byte(out), &p); err != nil || p.FilteredTotal != tc.want || !strings.Contains(p.Notes, "calendar bounds in UTC") {
			t.Fatal(out, err)
		}
	}
	manifest := assistantDataTool(t, app, "data_manifest")
	out := manifest.run(json.RawMessage(`{"collection":"notification_feed"}`))
	var m assistantdata.Manifest
	if err := json.Unmarshal([]byte(out), &m); err != nil || m.UnixSecondsTimezone != "UTC" || len(m.UnixSecondsDateFields) != 3 || !m.UnixSecondsDateFields["dueAt"].ZeroIsMissing {
		t.Fatal("timestamp schema unavailable", out, err)
	}
	out = query.run(json.RawMessage(`{"collection":"allocation_config","dateField":"config.amountMinor","from":"1970-01-01","through":"1970-01-01"}`))
	if !strings.Contains(out, "not declared as Unix seconds") {
		t.Fatal("money became timestamp/healthy zero", out)
	}
	assistantSetJSONKV(t, app, "cashflux:notify:feed", []uistate.FeedItem{{ID: "bad-date", Title: "Invalid date", At: math.MaxInt64}})
	out = query.run(json.RawMessage(`{"collection":"notification_feed","dateField":"at","from":"2026-10-20","through":"2026-10-20"}`))
	if !strings.Contains(out, "outside the supported calendar range") || strings.Contains(out, `"filteredTotal":0`) {
		t.Fatal("unrepresentable timestamp became healthy zero", out)
	}
}

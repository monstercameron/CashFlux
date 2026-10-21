// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func assistantSnapshotTasks(t *testing.T, app *appstate.App) {
	t.Helper()
	for i := range 4 {
		if err := app.Store().PutTask(domain.Task{ID: fmt.Sprintf("snapshot-%d", i), Title: fmt.Sprintf("Task %d", i), Status: domain.StatusOpen}); err != nil {
			t.Fatal(err)
		}
	}
}

func assistantSnapshotUsage(t *testing.T, app *appstate.App) {
	t.Helper()
	before := app.Rev()
	app.RecordAISpend("assistant", "gpt-4.1", ai.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500})
	if app.Rev() == before || len(app.Settings().AISpend) == 0 {
		t.Fatal("fixture did not persist real provider usage and advance the app revision")
	}
}

func assistantSnapshotDataPage(t *testing.T, app *appstate.App, args assistantdata.Query) assistantdata.Page {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	// Rebuild the real tool just as a later provider response does.
	out := assistantDataTool(t, app, "query_data").run(raw)
	var page assistantdata.Page
	if err := json.Unmarshal([]byte(out), &page); err != nil || page.Snapshot == "" {
		t.Fatalf("query failed: %s (%v)", out, err)
	}
	return page
}

func TestAssistantDataSnapshotSurvivesProviderUsageAndUnrelatedWrites(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSnapshotTasks(t, app)
	first := assistantSnapshotDataPage(t, app, assistantdata.Query{Collection: "tasks", Limit: 2})
	if first.NextOffset == nil || *first.NextOffset != 2 {
		t.Fatalf("missing continuation: %+v", first)
	}
	assistantSnapshotUsage(t, app)
	if err := app.SetAISpendCap(12.50); err != nil {
		t.Fatal(err)
	}
	args := assistantdata.Query{Collection: "tasks", Offset: *first.NextOffset, Limit: 2, Snapshot: first.Snapshot}
	second := assistantSnapshotDataPage(t, app, args)
	if second.Snapshot != first.Snapshot || second.Total != 4 || len(second.Records) != 2 || second.Records[0].ID != "snapshot-2" || second.NextOffset != nil {
		t.Fatalf("unchanged tasks could not continue after provider accounting: %+v", second)
	}
	// A row outside the continuation page still belongs to the full snapshot.
	if err := app.Store().PutTask(domain.Task{ID: "snapshot-0", Title: "Changed while reading", Status: domain.StatusOpen}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(args)
	if out := assistantDataTool(t, app, "query_data").run(raw); !strings.Contains(out, "snapshot changed") {
		t.Fatal("changed collection accepted stale continuation", out)
	}
}

func TestAssistantListsSnapshotSurvivesProviderUsageAndUnrelatedWrites(t *testing.T) {
	app := assistantListsFixture(t)
	assistantSnapshotTasks(t, app)
	first := assistantListsPage(t, assistantListsTool(t, app, "list_tasks"), assistantListArgs{Limit: 2})
	if first.NextOffset == nil || *first.NextOffset != 2 {
		t.Fatalf("missing continuation: %+v", first)
	}
	assistantSnapshotUsage(t, app)
	if err := app.SetAISpendCap(25); err != nil {
		t.Fatal(err)
	}
	args := assistantListArgs{Offset: *first.NextOffset, Limit: 2, Snapshot: first.Snapshot}
	second := assistantListsPage(t, assistantListsTool(t, app, "list_tasks"), args)
	if second.Snapshot != first.Snapshot || second.Total != 4 || len(second.Records) != 2 || second.Records[0].ID != "snapshot-2" || second.NextOffset != nil {
		t.Fatalf("unchanged list could not continue after provider accounting: %+v", second)
	}
	if err := app.Store().PutTask(domain.Task{ID: "snapshot-0", Title: "Changed while reading", Status: domain.StatusOpen}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(args)
	if err := assistantListsError(t, assistantListsTool(t, app, "list_tasks"), string(raw)); !strings.Contains(err, "snapshot changed") {
		t.Fatal("changed list accepted stale continuation", err)
	}
}

func TestAssistantDataSnapshotTracksFinancialSettingsContent(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	first := assistantSnapshotDataPage(t, app, assistantdata.Query{Collection: "financial_settings"})
	assistantSnapshotUsage(t, app)
	raw, _ := json.Marshal(assistantdata.DetailRequest{Collection: "financial_settings", ID: "household", Snapshot: first.Snapshot})
	if out := assistantDataTool(t, app, "read_record").run(raw); !strings.Contains(out, "snapshot changed") {
		t.Fatal("provider accounting changed this collection but stale detail was accepted", out)
	}
	fresh := assistantSnapshotDataPage(t, app, assistantdata.Query{Collection: "financial_settings"})
	if fresh.Snapshot == first.Snapshot || !strings.Contains(string(fresh.Records[0].Data), `"aiSpend"`) {
		t.Fatal("financial settings content change was not fingerprinted", fresh)
	}
}

func TestAssistantReadSnapshotsRejectOtherBoundStoreWithIdenticalRows(t *testing.T) {
	firstApp := primaryAccountsGoalsApp(t)
	assistantSnapshotTasks(t, firstApp)
	data := assistantSnapshotDataPage(t, firstApp, assistantdata.Query{Collection: "tasks", Limit: 2})
	list := assistantListsPage(t, assistantListsTool(t, firstApp, "list_tasks"), assistantListArgs{Limit: 2})
	otherApp := primaryAccountsGoalsApp(t)
	assistantSnapshotTasks(t, otherApp)
	if firstApp.Store() == otherApp.Store() {
		t.Fatal("fixture reused the same store")
	}
	raw, _ := json.Marshal(assistantdata.Query{Collection: "tasks", Offset: 2, Limit: 2, Snapshot: data.Snapshot})
	if out := assistantDataTool(t, otherApp, "query_data").run(raw); !strings.Contains(out, "snapshot changed") {
		t.Fatal("identical rows from another bound store accepted data token", out)
	}
	raw, _ = json.Marshal(assistantListArgs{Offset: 2, Limit: 2, Snapshot: list.Snapshot})
	if err := assistantListsError(t, assistantListsTool(t, otherApp, "list_tasks"), string(raw)); !strings.Contains(err, "snapshot changed") {
		t.Fatal("identical rows from another bound store accepted list token", err)
	}
}

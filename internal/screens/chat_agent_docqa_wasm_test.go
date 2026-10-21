// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSearchDocumentsFindsLateArtifactRowsAndReturnsBoundedEvidence(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	rows := make([][]string, 240)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("ordinary entry %d", i+1)}
	}
	rows[231] = []string{"policy renewal", "November 12"}
	for i := 0; i < 4; i++ {
		if err := app.PutArtifact(domain.Artifact{ID: fmt.Sprintf("docqa-%d", i), Name: "Insurance policy", Kind: "csv", Columns: []string{"event", "date"}, Rows: rows}); err != nil {
			t.Fatal(err)
		}
	}
	var search chatTool
	for _, tool := range agToolsDocQA(app, "USD", currency.Rates{}) {
		if tool.spec.Function.Name == "search_documents" {
			search = tool
			break
		}
	}
	if search.run == nil {
		t.Fatal("search_documents tool missing")
	}
	out := search.run(json.RawMessage(`{"question":"when is policy renewal"}`))
	if !strings.Contains(out, "matching sources; showing 3") || !strings.Contains(out, "row 232") || !strings.Contains(out, "policy renewal") || !strings.Contains(out, "More matching sources were omitted") {
		t.Fatalf("late-row evidence or honest page metadata missing: %s", out)
	}
	if strings.Contains(out, "ordinary entry 1\n") {
		t.Fatalf("search returned the start of the artifact instead of the matching row: %s", out)
	}
}

func TestSearchDocumentsReportsReadFailureAsUnavailable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	var search chatTool
	for _, tool := range agToolsDocQA(app, "USD", currency.Rates{}) {
		if tool.spec.Function.Name == "search_documents" {
			search = tool
			break
		}
	}
	out := search.run(json.RawMessage(`{"question":"insurance"}`))
	if !strings.Contains(out, "unavailable") || strings.Contains(out, "don't see a document") {
		t.Fatalf("read failure was treated as a successful empty search: %s", out)
	}
}

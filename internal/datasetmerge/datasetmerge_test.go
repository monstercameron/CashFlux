// SPDX-License-Identifier: MIT

package datasetmerge

import (
	"encoding/json"
	"testing"
)

// The thing that must never happen here is a lost or duplicated transaction, so
// most of these check the union rather than the happy path.

func collections(t *testing.T, merged []byte, key string) []map[string]any {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("merged snapshot is not an object: %v", err)
	}
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("collection %s: %v", key, err)
	}
	return out
}

func changeFor(r Report, name string) CollectionChange {
	for _, c := range r.Collections {
		if c.Name == name {
			return c
		}
	}
	return CollectionChange{}
}

func TestMergeUnionsRecordsByID(t *testing.T) {
	target := []byte(`{"transactions":[{"id":"a","amount":1},{"id":"b","amount":2}]}`)
	source := []byte(`{"transactions":[{"id":"b","amount":2},{"id":"c","amount":3}]}`)

	merged, report, err := Merge(target, source, PreferTarget)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	rows := collections(t, merged, "transactions")
	if len(rows) != 3 {
		t.Fatalf("merged %d transactions, want 3 (a, b, c): %s", len(rows), merged)
	}
	seen := map[string]int{}
	for _, r := range rows {
		seen[r["id"].(string)]++
	}
	for _, id := range []string{"a", "b", "c"} {
		if seen[id] != 1 {
			t.Errorf("id %q appears %d times, want exactly 1 — a merge must not duplicate records", id, seen[id])
		}
	}
	ch := changeFor(report, "transactions")
	if ch.Added != 1 || ch.Identical != 1 || ch.TargetOnly != 1 || ch.Total != 3 {
		t.Errorf("change = %+v, want 1 added / 1 identical / 1 target-only / 3 total", ch)
	}
	if report.Conflicts != 0 {
		t.Errorf("identical records must not count as conflicts: %+v", report)
	}
}

func TestMergeConflictPolicyDecidesAndIsCounted(t *testing.T) {
	target := []byte(`{"accounts":[{"id":"a","name":"Checking"}]}`)
	source := []byte(`{"accounts":[{"id":"a","name":"Everyday"}]}`)

	kept, report, err := Merge(target, source, PreferTarget)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := collections(t, kept, "accounts")[0]["name"]; got != "Checking" {
		t.Errorf("prefer-target kept %q, want the target's own record", got)
	}
	if report.Conflicts != 1 {
		t.Errorf("a differing record on both sides is a conflict: %+v", report)
	}

	taken, report, err := Merge(target, source, PreferSource)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := collections(t, taken, "accounts")[0]["name"]; got != "Everyday" {
		t.Errorf("prefer-source kept %q, want the incoming record", got)
	}
	// The count is the same either way: the policy decides, it does not hide.
	if report.Conflicts != 1 {
		t.Errorf("conflicts = %d, want 1 regardless of policy", report.Conflicts)
	}
}

func TestMergeIgnoresKeyOrderAndWhitespace(t *testing.T) {
	target := []byte(`{"accounts":[{"id":"a","name":"Checking","type":"checking"}]}`)
	source := []byte(`{"accounts":[{ "type":"checking",  "name":"Checking", "id":"a" }]}`)
	_, report, err := Merge(target, source, PreferTarget)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if report.Conflicts != 0 {
		t.Fatalf("the same record written in a different key order is not a conflict: %+v", report)
	}
	if changeFor(report, "accounts").Identical != 1 {
		t.Fatalf("change = %+v, want it recognised as identical", changeFor(report, "accounts"))
	}
}

func TestMergeTakesCollectionsTheTargetHasNeverSeen(t *testing.T) {
	target := []byte(`{"accounts":[{"id":"a"}]}`)
	source := []byte(`{"accounts":[{"id":"a"}],"holdings":[{"id":"h1"},{"id":"h2"}]}`)
	merged, report, err := Merge(target, source, PreferTarget)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if len(collections(t, merged, "holdings")) != 2 {
		t.Fatalf("a collection only the source has must come across: %s", merged)
	}
	if changeFor(report, "holdings").Added != 2 {
		t.Errorf("change = %+v, want 2 added", changeFor(report, "holdings"))
	}
}

func TestMergeKeepsTheTargetsSettingsAndSaysSo(t *testing.T) {
	// Settings are not records with identities. "Merging" them means picking one
	// arbitrarily, so the target keeps its own and the key is named — a refusal
	// stated rather than an omission.
	target := []byte(`{"settings":{"theme":"dark"},"schemaVersion":9,"accounts":[]}`)
	source := []byte(`{"settings":{"theme":"light"},"schemaVersion":9,"accounts":[]}`)
	merged, report, err := Merge(target, source, PreferSource)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	var doc struct {
		Settings struct {
			Theme string `json:"theme"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("merged: %v", err)
	}
	if doc.Settings.Theme != "dark" {
		t.Errorf("theme = %q, want the target's own setting even under prefer-source", doc.Settings.Theme)
	}
	if len(report.KeptFromTarget) != 1 || report.KeptFromTarget[0] != "settings" {
		t.Errorf("KeptFromTarget = %v, want [settings] so the user is not surprised", report.KeptFromTarget)
	}
	// schemaVersion is identical on both sides, so it is not worth reporting.
	for _, k := range report.KeptFromTarget {
		if k == "schemaVersion" {
			t.Error("an identical non-record key should not be reported as kept-from-target")
		}
	}
}

func TestMergeRefusesToMatchRecordsWithoutIDs(t *testing.T) {
	// Matching these positionally or by content would be a guess, and a wrong
	// guess either duplicates somebody's records or drops them.
	target := []byte(`{"mystery":[{"name":"one"},{"name":"two"}]}`)
	source := []byte(`{"mystery":[{"name":"three"}]}`)
	merged, report, err := Merge(target, source, PreferSource)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if len(collections(t, merged, "mystery")) != 2 {
		t.Fatalf("the target's copy must be kept whole: %s", merged)
	}
	if len(report.UnmergeableCollections) != 1 || report.UnmergeableCollections[0] != "mystery" {
		t.Fatalf("UnmergeableCollections = %v, want [mystery] reported rather than silently skipped", report.UnmergeableCollections)
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	// Merging twice must not grow the dataset — the shape of a duplicate-records
	// bug that only shows up after somebody retries.
	target := []byte(`{"transactions":[{"id":"a"},{"id":"b"}]}`)
	source := []byte(`{"transactions":[{"id":"b"},{"id":"c"}]}`)
	once, _, err := Merge(target, source, PreferSource)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	twice, report, err := Merge(once, source, PreferSource)
	if err != nil {
		t.Fatalf("second Merge: %v", err)
	}
	if len(collections(t, twice, "transactions")) != 3 {
		t.Fatalf("a repeated merge grew the dataset: %s", twice)
	}
	if report.TotalAdded != 0 {
		t.Errorf("second merge added %d records, want 0", report.TotalAdded)
	}
}

func TestMergeRejectsSnapshotsItCannotRead(t *testing.T) {
	if _, _, err := Merge([]byte(`not json`), []byte(`{}`), PreferTarget); err == nil {
		t.Error("an unreadable target must be an error, not an empty merge")
	}
	if _, _, err := Merge([]byte(`{}`), []byte(`[1,2,3]`), PreferTarget); err == nil {
		t.Error("a source that is not an object must be an error")
	}
}

func TestMergeDefaultsToTheConservativePolicy(t *testing.T) {
	target := []byte(`{"accounts":[{"id":"a","name":"Checking"}]}`)
	source := []byte(`{"accounts":[{"id":"a","name":"Everyday"}]}`)
	merged, report, err := Merge(target, source, Policy("nonsense"))
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if report.Policy != PreferTarget {
		t.Errorf("policy = %q, want the conservative default", report.Policy)
	}
	if got := collections(t, merged, "accounts")[0]["name"]; got != "Checking" {
		t.Errorf("name = %q, want the target's record kept under the default", got)
	}
}

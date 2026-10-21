// SPDX-License-Identifier: MIT

package undosnap_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

// normalize unmarshals JSON into an interface{} so byte-level differences
// (key order, whitespace) don't cause spurious failures.
func normalize(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return v
}

func TestToSnapshot_Arrays(t *testing.T) {
	input := []byte(`{
		"accounts": [
			{"id":"a1","name":"Checking","balance":1000},
			{"id":"a2","name":"Savings","balance":5000}
		],
		"transactions": [
			{"id":"t1","amount":-50,"account":"a1"}
		]
	}`)

	snap, err := undosnap.ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot: %v", err)
	}
	if len(snap["accounts"]) != 2 {
		t.Errorf("accounts: want 2 rows, got %d", len(snap["accounts"]))
	}
	if _, ok := snap["accounts"]["a1"]; !ok {
		t.Error("accounts[a1] missing")
	}
	if _, ok := snap["accounts"]["a2"]; !ok {
		t.Error("accounts[a2] missing")
	}
	if len(snap["transactions"]) != 1 {
		t.Errorf("transactions: want 1 row, got %d", len(snap["transactions"]))
	}
}

func TestToSnapshot_Scalars(t *testing.T) {
	input := []byte(`{
		"schemaVersion": 3,
		"settings": {"currency":"USD","openAIKey":""},
		"accounts": []
	}`)

	snap, err := undosnap.ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot: %v", err)
	}
	// Scalars land in _meta:* collections.
	if _, ok := snap["_meta:schemaVersion"]["schemaVersion"]; !ok {
		t.Error("_meta:schemaVersion missing")
	}
	if _, ok := snap["_meta:settings"]["settings"]; !ok {
		t.Error("_meta:settings missing")
	}
	// Empty array produces an empty map (not nil) collection.
	if snap["accounts"] == nil {
		t.Error("accounts collection should be non-nil (empty map)")
	}
	if len(snap["accounts"]) != 0 {
		t.Errorf("accounts: want 0 rows, got %d", len(snap["accounts"]))
	}
}

func TestToSnapshot_ReservedKeyError(t *testing.T) {
	input := []byte(`{"_meta:evil": "bad"}`)
	_, err := undosnap.ToSnapshot(input)
	if err == nil {
		t.Fatal("expected error for reserved _meta: key in export JSON")
	}
}

func TestToSnapshot_MissingIDError(t *testing.T) {
	input := []byte(`{"accounts": [{"name":"no id here"}]}`)
	_, err := undosnap.ToSnapshot(input)
	if err == nil {
		t.Fatal("expected error for element missing id")
	}
}

func TestToSnapshot_StoreKeyedCollections(t *testing.T) {
	input := []byte(`{
		"placements": [
			{"id":"hero","surface":"dashboard","layout":{"x":0}},
			{"id":"hero","surface":"page:accounts","layout":{"x":1}}
		],
		"molecules": [
			{"name":"monthlySpend","formula":"sum(expenses)"},
			{"name":"netWorth","formula":"assets-liabilities"}
		]
	}`)

	snap, err := undosnap.ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot: %v", err)
	}
	if len(snap["placements"]) != 2 {
		t.Fatalf("placements: want 2 rows, got %d", len(snap["placements"]))
	}
	for _, key := range []string{"dashboard/hero", "page:accounts/hero"} {
		if _, ok := snap["placements"][key]; !ok {
			t.Errorf("placements[%q] missing", key)
		}
	}
	if len(snap["molecules"]) != 2 {
		t.Fatalf("molecules: want 2 rows, got %d", len(snap["molecules"]))
	}
	for _, key := range []string{"netWorth", "monthlySpend"} {
		if _, ok := snap["molecules"][key]; !ok {
			t.Errorf("molecules[%q] missing", key)
		}
	}

	out, err := undosnap.ToJSON(snap)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if got := normalize(t, out); !reflect.DeepEqual(normalize(t, input), got) {
		t.Errorf("round-trip mismatch\ninput: %s\noutput: %s", input, out)
	}
}

func TestToSnapshot_StoreKeyedCollectionsRequireKeys(t *testing.T) {
	for _, input := range []string{
		`{"molecules":[{"formula":"x"}]}`,
		`{"placements":[{"id":"p"}]}`,
		`{"placements":[{"surface":"dashboard"}]}`,
		`{"molecules":[{"name":"same"},{"name":"same"}]}`,
		`{"placements":[{"id":"p","surface":"dashboard"},{"id":"p","surface":"dashboard"}]}`,
	} {
		if _, err := undosnap.ToSnapshot([]byte(input)); err == nil {
			t.Errorf("ToSnapshot(%s): expected invalid/missing/duplicate collection key error", input)
		}
	}
}

func TestToSnapshotDatasetMatchesExportSnapshotForEveryDatasetField(t *testing.T) {
	ds := store.Dataset{Settings: store.Settings{
		BaseCurrency: "USD",
		OpenAIKey:    "device-only-secret",
		FXRates:      map[string]float64{"EUR": 0.91},
	}}
	populateEveryDatasetSlice(t, &ds)
	ds.Transactions = []domain.Transaction{{ID: "txn-parity"}}
	ds.Placements[0].Spec.Graph = json.RawMessage(` { "nodes": [ ] } `)
	ds.TransferOperations = []store.TransferOperationReceipt{
		{ID: "live", Transactions: []domain.Transaction{{ID: "txn-parity"}}},
		{ID: "orphan", Transactions: []domain.Transaction{{ID: "missing"}}},
		{ID: "bad-leg", Transactions: []domain.Transaction{{ID: ""}}},
		{ID: "empty"},
		{Transactions: []domain.Transaction{{ID: "txn-parity"}}},
	}
	ds.KV = map[string]string{"layout": `{"widgets":[]}`}
	ds.SettingsKV = map[string]string{}

	wantDataset := ds
	wantDataset.Settings.OpenAIKey = ""
	wantJSON, err := store.Export(wantDataset)
	if err != nil {
		t.Fatalf("store.Export: %v", err)
	}
	want, err := undosnap.ToSnapshot(wantJSON)
	if err != nil {
		t.Fatalf("ToSnapshot(export): %v", err)
	}
	got, err := undosnap.ToSnapshotDataset(ds)
	if err != nil {
		t.Fatalf("ToSnapshotDataset: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		for collection := range want {
			if !reflect.DeepEqual(got[collection], want[collection]) {
				t.Errorf("collection %q differs\n got: %#v\nwant: %#v", collection, got[collection], want[collection])
			}
		}
		for collection := range got {
			if _, exists := want[collection]; !exists {
				t.Errorf("unexpected collection %q: %#v", collection, got[collection])
			}
		}
	}
	if got := string(got["_meta:settings"]["settings"]); containsCredential(got) {
		t.Fatalf("redacted settings snapshot retained credential: %s", got)
	}
	if ds.Settings.OpenAIKey != "device-only-secret" {
		t.Fatal("ToSnapshotDataset mutated the source dataset credential")
	}
}

func TestToSnapshotDatasetNilAndEmptyCollectionsMatchExport(t *testing.T) {
	for _, emptySlices := range []bool{false, true} {
		name := "nil"
		if emptySlices {
			name = "empty-non-nil"
		}
		t.Run(name, func(t *testing.T) {
			ds := store.Dataset{}
			if emptySlices {
				value := reflect.ValueOf(&ds).Elem()
				for i := 0; i < value.NumField(); i++ {
					field := value.Field(i)
					if field.Kind() == reflect.Slice {
						field.Set(reflect.MakeSlice(field.Type(), 0, 0))
					}
				}
			}
			wantDataset := ds
			wantDataset.Settings.OpenAIKey = ""
			data, err := store.Export(wantDataset)
			if err != nil {
				t.Fatalf("store.Export: %v", err)
			}
			want, err := undosnap.ToSnapshot(data)
			if err != nil {
				t.Fatalf("ToSnapshot(export): %v", err)
			}
			got, err := undosnap.ToSnapshotDataset(ds)
			if err != nil {
				t.Fatalf("ToSnapshotDataset: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("snapshot mismatch\n got: %#v\nwant: %#v", got, want)
			}
		})
	}
}

func TestToSnapshotDatasetKeyValidationRemainsStrict(t *testing.T) {
	tests := []struct {
		name string
		ds   store.Dataset
	}{
		{name: "missing id", ds: store.Dataset{Accounts: []domain.Account{{Name: "no id"}}}},
		{name: "duplicate id", ds: store.Dataset{Accounts: []domain.Account{{ID: "same"}, {ID: "same"}}}},
		{name: "missing molecule name", ds: store.Dataset{Molecules: []domain.Molecule{{Formula: "x"}}}},
		{name: "missing placement surface", ds: store.Dataset{Placements: []domain.Placement{{ID: "same"}}}},
		{name: "missing placement id", ds: store.Dataset{Placements: []domain.Placement{{Surface: "dashboard"}}}},
		{name: "duplicate composite placement key", ds: store.Dataset{Placements: []domain.Placement{{ID: "same", Surface: "dashboard"}, {ID: "same", Surface: "dashboard"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := undosnap.ToSnapshotDataset(tc.ds); err == nil {
				t.Fatal("ToSnapshotDataset succeeded; want key validation error")
			}
		})
	}
}

func TestToSnapshotDatasetTransactionCustomMarshalerPreservesKeyAndRawFields(t *testing.T) {
	transaction := domain.Transaction{
		ID:        "transaction-key",
		MemberIDs: []string{"owner-z", "", "owner-a", "owner-z"},
		MemberID:  "legacy-owner",
		Custom: map[string]any{
			"graph": json.RawMessage(` { "nodes": [ { "id": "n1" } ] } `),
		},
	}
	ds := store.Dataset{Transactions: []domain.Transaction{transaction}}
	before := ds.Transactions[0]
	redacted := ds
	redacted.Settings.OpenAIKey = ""
	exported, err := store.Export(redacted)
	if err != nil {
		t.Fatalf("store.Export: %v", err)
	}
	want, err := undosnap.ToSnapshot(exported)
	if err != nil {
		t.Fatalf("ToSnapshot(export): %v", err)
	}
	got, err := undosnap.ToSnapshotDataset(ds)
	if err != nil {
		t.Fatalf("ToSnapshotDataset: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("transaction snapshot mismatch\n got: %#v\nwant: %#v", got["transactions"], want["transactions"])
	}
	if _, ok := got["transactions"]["transaction-key"]; !ok {
		t.Fatalf("custom marshaler changed direct row key: %#v", got["transactions"])
	}
	if !reflect.DeepEqual(ds.Transactions[0], before) {
		t.Fatal("typed key extraction mutated transaction owners or custom payload")
	}
}

func populateEveryDatasetSlice(t *testing.T, dataset *store.Dataset) {
	t.Helper()
	value := reflect.ValueOf(dataset).Elem()
	typeOf := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.Slice || typeOf.Field(i).Name == "TransferOperations" {
			continue
		}
		row := reflect.New(field.Type().Elem()).Elem()
		setStringTag(t, row, "id", "fixture-"+typeOf.Field(i).Name)
		setStringTag(t, row, "name", "fixture-"+typeOf.Field(i).Name)
		setStringTag(t, row, "surface", "dashboard")
		field.Set(reflect.MakeSlice(field.Type(), 1, 1))
		field.Index(0).Set(row)
	}
}

func setStringTag(t *testing.T, value reflect.Value, wantedTag, text string) {
	t.Helper()
	if value.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := value.Type().Field(i)
		jsonName := strings.Split(structField.Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			jsonName = structField.Name
		}
		if strings.EqualFold(jsonName, wantedTag) && field.Kind() == reflect.String && field.CanSet() {
			field.SetString(text)
		}
	}
}

func containsCredential(raw string) bool {
	var settings map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return true
	}
	return string(settings["openAiKey"]) != ""
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{
			name: "typical export",
			input: `{
				"schemaVersion": 2,
				"settings": {"currency":"USD","theme":"dark"},
				"accounts": [
					{"id":"acc1","name":"Checking","type":"checking"},
					{"id":"acc2","name":"Savings","type":"savings"}
				],
				"transactions": [
					{"id":"tx1","amount":-120,"account":"acc1","date":"2026-01-15"},
					{"id":"tx2","amount":5000,"account":"acc2","date":"2026-01-01"}
				],
				"categories": []
			}`,
		},
		{
			name:  "empty collections",
			input: `{"accounts":[],"transactions":[],"categories":[]}`,
		},
		{
			name:  "only scalars",
			input: `{"schemaVersion":1,"settings":{"currency":"EUR"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := undosnap.ToSnapshot([]byte(tc.input))
			if err != nil {
				t.Fatalf("ToSnapshot: %v", err)
			}
			out, err := undosnap.ToJSON(snap)
			if err != nil {
				t.Fatalf("ToJSON: %v", err)
			}
			if !reflect.DeepEqual(normalize(t, []byte(tc.input)), normalize(t, out)) {
				t.Errorf("round-trip mismatch\ninput: %s\noutput: %s", tc.input, out)
			}
		})
	}
}

func TestDiffAndApplyIntegration(t *testing.T) {
	// Confirm that Diff / Invert / Apply work correctly when combined with
	// undosnap: forward diff moves forward, inverted diff moves back.
	before := []byte(`{"accounts":[{"id":"a1","name":"Checking","balance":1000}]}`)
	after := []byte(`{"accounts":[{"id":"a1","name":"Checking","balance":1500}]}`)

	snapBefore, err := undosnap.ToSnapshot(before)
	if err != nil {
		t.Fatal(err)
	}
	snapAfter, err := undosnap.ToSnapshot(after)
	if err != nil {
		t.Fatal(err)
	}

	cs := history.Diff(snapBefore, snapAfter)
	if cs.IsEmpty() {
		t.Fatal("Diff should detect the balance change")
	}

	// Apply forward: snapBefore + cs == snapAfter
	applied := cs.Apply(snapBefore)
	gotAfterJSON, err := undosnap.ToJSON(applied)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(t, gotAfterJSON), normalize(t, after)) {
		t.Errorf("forward apply wrong: got %s", gotAfterJSON)
	}

	// Apply inverse: snapAfter + cs.Invert() == snapBefore
	reverted := cs.Invert().Apply(snapAfter)
	gotBeforeJSON, err := undosnap.ToJSON(reverted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalize(t, gotBeforeJSON), normalize(t, before)) {
		t.Errorf("inverse apply wrong: got %s", gotBeforeJSON)
	}
}

func TestStackUndoRedo(t *testing.T) {
	// Confirm Stack.Undo returns the inverse change set and Stack.Redo returns
	// the forward change set, so our undo controller uses them correctly.
	snap0 := history.Snapshot{
		"items": {"i1": json.RawMessage(`{"id":"i1","v":1}`)},
	}
	snap1 := history.Snapshot{
		"items": {"i1": json.RawMessage(`{"id":"i1","v":2}`)},
	}

	cs := history.Diff(snap0, snap1)
	if cs.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}

	stack := history.NewStack(0)
	stack.Push(cs)

	// Undo returns the INVERSE change set (moves backward).
	undoCS, ok := stack.Undo()
	if !ok {
		t.Fatal("Undo should succeed")
	}
	revertedSnap := undoCS.Apply(snap1)
	if string(revertedSnap["items"]["i1"]) != `{"id":"i1","v":1}` {
		t.Errorf("undo did not revert: got %s", revertedSnap["items"]["i1"])
	}

	// Redo returns the FORWARD change set (re-applies the mutation).
	redoCS, ok := stack.Redo()
	if !ok {
		t.Fatal("Redo should succeed")
	}
	reappliedSnap := redoCS.Apply(snap0)
	if string(reappliedSnap["items"]["i1"]) != `{"id":"i1","v":2}` {
		t.Errorf("redo did not re-apply: got %s", reappliedSnap["items"]["i1"])
	}
}

// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestLoadPreparedMatchesImportThenLoad(t *testing.T) {
	base := SampleDatasetAt(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC))
	baseRaw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	var current, legacy Dataset
	if err := json.Unmarshal(baseRaw, &current); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(baseRaw, &legacy); err != nil {
		t.Fatal(err)
	}
	current.SchemaVersion = SchemaVersion
	legacy.SchemaVersion = 1
	for i := range legacy.Accounts {
		if legacy.Accounts[i].Class == domain.ClassLiability {
			legacy.Accounts[i].LiabilitySign = ""
		}
	}
	for i := range legacy.Transactions {
		legacy.Transactions[i].TransferGroupID = ""
	}
	for i := range legacy.Budgets {
		legacy.Budgets[i].Limit.Currency = ""
	}
	for _, tc := range []struct {
		name string
		ds   Dataset
	}{{"current schema", current}, {"legacy schema", legacy}} {
		t.Run(tc.name, func(t *testing.T) {
			dataset := tc.ds
			dataset.Settings.OpenAIKey = "incoming-key-must-not-be-adopted"
			raw, err := json.Marshal(dataset)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := Import(raw)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareDataset(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(prepared.dataset, imported) {
				t.Fatal("prepared dataset differs from Import result")
			}
			const deviceKey = "device-local-key"
			imported.Settings.OpenAIKey = deviceKey

			ordinary, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			preparedStore, err := NewMemory()
			if err != nil {
				_ = ordinary.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = ordinary.Close()
				_ = preparedStore.Close()
			})
			if err := ordinary.Load(imported); err != nil {
				t.Fatal(err)
			}
			if err := preparedStore.LoadPrepared(prepared, deviceKey); err != nil {
				t.Fatal(err)
			}

			want, err := ordinary.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			got, err := preparedStore.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("prepared load differs from import plus ordinary load")
			}
			if got.Settings.OpenAIKey != deviceKey {
				t.Fatal("prepared load did not preserve the device-local credential")
			}

			wantChanges := captureAll(t, ordinary, 0, cursor(t, ordinary), 100)
			gotChanges := captureAll(t, preparedStore, 0, cursor(t, preparedStore), 100)
			if len(wantChanges) == 0 || len(gotChanges) == 0 {
				t.Fatal("dataset load did not produce captured row changes")
			}
			if !reflect.DeepEqual(gotChanges, wantChanges) {
				t.Fatal("prepared load produced different journal row changes")
			}
		})
	}
}

func TestLoadPreparedReuseIsImmutableAcrossDeviceKeys(t *testing.T) {
	dataset := EmptyDataset()
	dataset.Settings.OpenAIKey = "incoming-key"
	raw, err := json.Marshal(dataset)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareDataset(raw)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, key := range []string{"device-one", "device-two"} {
		if err := s.LoadPrepared(prepared, key); err != nil {
			t.Fatal(err)
		}
		ds, err := s.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if ds.Settings.OpenAIKey != key {
			t.Fatalf("stored key = %q, want %q", ds.Settings.OpenAIKey, key)
		}
		if prepared.dataset.Settings.OpenAIKey != "incoming-key" {
			t.Fatal("prepared token was mutated by load")
		}
	}
}

func TestLoadPreparedRejectsUnpreparedHandles(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, prepared := range []*PreparedDataset{nil, {}} {
		if err := s.LoadPrepared(prepared, ""); err == nil {
			t.Fatal("load accepted an unprepared dataset handle")
		}
	}
	if _, err := PrepareDataset([]byte(`{"schemaVersion":999999}`)); err == nil {
		t.Fatal("preparation accepted unsupported schema")
	}
}

func TestLoadPreparedFailureLeavesStoreAndRevisionUnchanged(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Load(SampleDatasetAt(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	before, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	beforeCursor, beforeRev := cursor(t, s), s.Rev()
	if err := s.LoadPrepared(&PreparedDataset{}, "new-device-key"); err == nil {
		t.Fatal("unprepared load unexpectedly succeeded")
	}
	after, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("failed prepared load changed stored rows")
	}
	if cursor(t, s) != beforeCursor || s.Rev() != beforeRev {
		t.Fatal("failed prepared load advanced the store revision")
	}
}

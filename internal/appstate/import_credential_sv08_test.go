// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/store"
)

func TestDatasetImportsAlwaysUseDeviceOpenAIKey(t *testing.T) {
	remote, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	remoteData := store.EmptyDataset()
	remoteData.Settings.OpenAIKey = "sk-imported-attacker-key"
	if err := remote.Store().Load(remoteData); err != nil {
		t.Fatal(err)
	}
	sqliteData, err := remote.ExportSQLite()
	if err != nil {
		t.Fatalf("export SQLite fixture: %v", err)
	}
	jsonData, err := store.Export(remoteData)
	if err != nil {
		t.Fatalf("export JSON fixture: %v", err)
	}
	importers := map[string]func(*App, []byte) error{
		"JSON":            (*App).ImportJSON,
		"JSON with blobs": (*App).ImportJSONWithBlobs,
	}

	for name, importDataset := range importers {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				deviceKey  string
				wantDevice string
			}{
				{name: "empty device key remains empty"},
				{name: "device key wins", deviceKey: "sk-device-key", wantDevice: "sk-device-key"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					target, err := New(nil, false)
					if err != nil {
						t.Fatal(err)
					}
					current := store.EmptyDataset()
					current.Settings.OpenAIKey = tc.deviceKey
					if err := target.Store().Load(current); err != nil {
						t.Fatal(err)
					}
					if err := importDataset(target, jsonData); err != nil {
						t.Fatalf("import: %v", err)
					}
					if got := target.Settings().OpenAIKey; got != tc.wantDevice {
						t.Fatalf("OpenAI key after import = %q, want %q", got, tc.wantDevice)
					}
				})
			}
		})
	}

	t.Run("SQLite", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			deviceKey  string
			wantDevice string
		}{
			{name: "empty device key remains empty"},
			{name: "device key wins", deviceKey: "sk-device-key", wantDevice: "sk-device-key"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				target, err := New(nil, false)
				if err != nil {
					t.Fatal(err)
				}
				current := store.EmptyDataset()
				current.Settings.OpenAIKey = tc.deviceKey
				if err := target.Store().Load(current); err != nil {
					t.Fatal(err)
				}
				if err := target.ImportSQLite(sqliteData); err != nil {
					t.Fatalf("import SQLite: %v", err)
				}
				if got := target.Settings().OpenAIKey; got != tc.wantDevice {
					t.Fatalf("OpenAI key after SQLite import = %q, want %q", got, tc.wantDevice)
				}
			})
		}
	})
}

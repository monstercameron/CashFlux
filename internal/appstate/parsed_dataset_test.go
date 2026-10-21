// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/store"
)

func TestParsedDatasetImportMatchesJSONIngress(t *testing.T) {
	for _, tc := range []struct{ name, deviceKey string }{
		{"keyless device", ""},
		{"credentialed device", "device-only-credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deviceKey := tc.deviceKey
			jsonApp, parsedApp := newApp(t, false), newApp(t, false)
			t.Cleanup(func() { jsonApp.Store().Close(); parsedApp.Store().Close() })
			for _, app := range []*App{jsonApp, parsedApp} {
				ds := store.EmptyDataset()
				ds.Settings.OpenAIKey = deviceKey
				if err := app.Store().Load(ds); err != nil {
					t.Fatal(err)
				}
			}
			// Marshal directly so the parser, rather than Export, performs legacy
			// migration and removes transfer receipts without live transactions.
			incoming := store.EmptyDataset()
			incoming.SchemaVersion = 0
			incoming.Settings.OpenAIKey = "incoming-credential-must-not-be-adopted"
			incoming.KV = map[string]string{"fixture": "retained"}
			incoming.TransferOperations = []store.TransferOperationReceipt{{ID: "orphan"}}
			raw, err := json.Marshal(incoming)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseDataset(raw)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.dataset.SchemaVersion != store.SchemaVersion || len(parsed.dataset.TransferOperations) != 0 {
				t.Fatal("parse did not migrate and normalize dataset")
			}
			if err := jsonApp.ImportJSON(raw); err != nil {
				t.Fatal(err)
			}
			if err := parsedApp.ImportParsedDataset(parsed); err != nil {
				t.Fatal(err)
			}
			want, err := jsonApp.Store().Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			got, err := parsedApp.Store().Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("parsed ingress differs from JSON ingress")
			}
			if got.Settings.OpenAIKey != deviceKey {
				t.Fatal("import adopted an incoming credential")
			}
		})
	}
}

func TestParsedDatasetRejectsInvalidIngress(t *testing.T) {
	app := newApp(t, false)
	t.Cleanup(func() { app.Store().Close() })
	for _, parsed := range []*ParsedDataset{nil, {}} {
		if err := app.ImportParsedDataset(parsed); err == nil {
			t.Fatal("invalid parsed handle accepted")
		}
	}
	for _, raw := range []string{`{`, `{"schemaVersion":999999}`} {
		if _, err := ParseDataset([]byte(raw)); err == nil {
			t.Fatal("invalid dataset accepted")
		}
	}
}

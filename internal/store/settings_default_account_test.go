// SPDX-License-Identifier: MIT

package store

import (
	"strings"
	"testing"
)

func TestDefaultAccountSettingsJSONAndSQLiteRoundTrip(t *testing.T) {
	original := EmptyDataset()
	original.Settings.DefaultAccountID = "checking-1"

	data, err := Export(original)
	if err != nil {
		t.Fatalf("Export JSON: %v", err)
	}
	fromJSON, err := Import(data)
	if err != nil {
		t.Fatalf("Import JSON: %v", err)
	}
	if got := fromJSON.Settings.DefaultAccountID; got != "checking-1" {
		t.Fatalf("JSON default account = %q, want checking-1", got)
	}

	s := loadedStore(t, original)
	sqliteBytes, err := s.SerializeSQLite()
	if err != nil {
		t.Fatalf("SerializeSQLite: %v", err)
	}
	fromSQLite, err := ImportSQLiteFile(sqliteBytes)
	if err != nil {
		t.Fatalf("ImportSQLiteFile: %v", err)
	}
	if got := fromSQLite.Settings.DefaultAccountID; got != "checking-1" {
		t.Fatalf("SQLite default account = %q, want checking-1", got)
	}

	checkpoint, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot checkpoint: %v", err)
	}
	if got := checkpoint.Settings.DefaultAccountID; got != "checking-1" {
		t.Fatalf("checkpoint default account = %q, want checking-1", got)
	}
}

func TestLegacyDatasetWithoutDefaultAccountLoadsUnchanged(t *testing.T) {
	legacy := `{"schemaVersion":1,"settings":{"baseCurrency":"USD"}}`
	ds, err := Import([]byte(legacy))
	if err != nil {
		t.Fatalf("Import legacy dataset: %v", err)
	}
	if ds.Settings.BaseCurrency != "USD" || ds.Settings.DefaultAccountID != "" {
		t.Fatalf("legacy settings = %+v, want USD with empty default account", ds.Settings)
	}
	s := loadedStore(t, ds)
	loaded, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot legacy dataset: %v", err)
	}
	if loaded.Settings.BaseCurrency != "USD" || loaded.Settings.DefaultAccountID != "" {
		t.Fatalf("legacy stored settings = %+v", loaded.Settings)
	}
	data, err := Export(loaded)
	if err != nil {
		t.Fatalf("Export legacy dataset: %v", err)
	}
	if strings.Contains(string(data), `"defaultAccountId"`) {
		t.Fatalf("empty default account should remain omitted from JSON: %s", data)
	}
}

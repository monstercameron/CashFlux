// SPDX-License-Identifier: MIT

package server

import "testing"

func TestRefreshRotationBindingMigration(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	version, err := store.SchemaVersion()
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentServerSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentServerSchemaVersion)
	}
	hasBinding, err := store.columnExists("refresh_tokens", "rotation_binding")
	if err != nil {
		t.Fatalf("inspect refresh token binding: %v", err)
	}
	if !hasBinding {
		t.Fatal("refresh_tokens.rotation_binding is missing")
	}
	if err := store.migrateTo20(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}

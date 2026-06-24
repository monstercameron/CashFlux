// SPDX-License-Identifier: MIT

package store

import "testing"

// TestKVExportImportRoundTrip proves the appkv (wiped-on-wipe) and settingskv
// (preserved) maps survive the Export→Import JSON round-trip — i.e. the exact blob
// browserstore persists to IndexedDB carries both back intact. A regression here
// would silently drop the dashboard layout, filters, theme, prefs, etc. on reload.
func TestKVExportImportRoundTrip(t *testing.T) {
	s := newStore(t)
	if err := s.SetKV("cashflux:layout", `["w-net","w-spend"]`); err != nil {
		t.Fatalf("SetKV: %v", err)
	}
	if err := s.SetSettingKV("cashflux:theme", "midnight"); err != nil {
		t.Fatalf("SetSettingKV: %v", err)
	}
	ds, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	back, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if back.KV["cashflux:layout"] != `["w-net","w-spend"]` {
		t.Errorf("appkv lost in JSON round-trip: %v", back.KV)
	}
	if back.SettingsKV["cashflux:theme"] != "midnight" {
		t.Errorf("settingskv lost in JSON round-trip: %v", back.SettingsKV)
	}
	// And loading the round-tripped dataset into a fresh store restores both.
	s2 := newStore(t)
	if err := s2.Load(back); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok, _ := s2.GetKV("cashflux:layout"); !ok || v != `["w-net","w-spend"]` {
		t.Errorf("appkv not restored after Load: %q ok=%v", v, ok)
	}
	if v, ok, _ := s2.GetSettingKV("cashflux:theme"); !ok || v != "midnight" {
		t.Errorf("settingskv not restored after Load: %q ok=%v", v, ok)
	}
}

// TestEmptyKVOmitted confirms empty KV maps don't bloat the JSON (omitempty) and
// round-trip cleanly as nil/empty.
func TestEmptyKVOmitted(t *testing.T) {
	ds := EmptyDataset()
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if back, err := Import(blob); err != nil {
		t.Fatalf("Import: %v", err)
	} else if len(back.KV) != 0 || len(back.SettingsKV) != 0 {
		t.Errorf("empty dataset gained KV entries: kv=%v settingsKV=%v", back.KV, back.SettingsKV)
	}
}

func TestKVRoundTripAndWipe(t *testing.T) {
	s := newStore(t)
	if err := s.SetKV("cashflux:layout", `["a","b"]`); err != nil {
		t.Fatalf("SetKV: %v", err)
	}
	if err := s.SetKV("cashflux:notify:feed", `[{"id":"n1"}]`); err != nil {
		t.Fatalf("SetKV: %v", err)
	}
	if v, ok, _ := s.GetKV("cashflux:layout"); !ok || v != `["a","b"]` {
		t.Fatalf("GetKV layout = %q ok=%v", v, ok)
	}
	// Round-trips through Snapshot/Load (the dataset blob).
	ds, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if ds.KV["cashflux:notify:feed"] != `[{"id":"n1"}]` {
		t.Fatalf("KV not in snapshot: %v", ds.KV)
	}
	s2 := newStore(t)
	if err := s2.Load(ds); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok, _ := s2.GetKV("cashflux:layout"); !ok || v != `["a","b"]` {
		t.Fatalf("KV lost across Load: %q ok=%v", v, ok)
	}
	// Settings KV round-trips and is PRESERVED by a wipe (it's config).
	if err := s2.SetSettingKV("cashflux:theme", "dark"); err != nil {
		t.Fatalf("SetSettingKV: %v", err)
	}
	ds2, _ := s2.Snapshot()
	if ds2.SettingsKV["cashflux:theme"] != "dark" {
		t.Fatalf("settingsKV not in snapshot: %v", ds2.SettingsKV)
	}
	// A wipe clears KV (financial/derived) but keeps settingskv (config).
	if err := s2.Wipe(); err != nil {
		t.Fatalf("Wipe: %v", err)
	}
	if _, ok, _ := s2.GetKV("cashflux:layout"); ok {
		t.Fatal("KV survived wipe")
	}
	if v, ok, _ := s2.GetSettingKV("cashflux:theme"); !ok || v != "dark" {
		t.Fatalf("settingsKV must survive wipe, got %q ok=%v", v, ok)
	}
	// DeleteKV.
	_ = s.SetKV("k", "v")
	if err := s.DeleteKV("k"); err != nil {
		t.Fatalf("DeleteKV: %v", err)
	}
	if _, ok, _ := s.GetKV("k"); ok {
		t.Fatal("DeleteKV did not remove")
	}
}

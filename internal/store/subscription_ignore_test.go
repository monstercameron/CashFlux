package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSubscriptionIgnoreRoundTrip(t *testing.T) {
	ignoredOn := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	ds := Dataset{
		SubscriptionIgnores: []domain.SubscriptionIgnore{
			{ID: "si1", SubName: "Morning Coffee", IgnoredOn: ignoredOn},
			{ID: "si2", SubName: "Gym App", IgnoredOn: ignoredOn.AddDate(0, -1, 0)},
		},
	}

	exported, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(exported)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	reexported, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(exported, reexported) {
		t.Errorf("round-trip not lossless:\nfirst:\n%s\nsecond:\n%s", exported, reexported)
	}
	if len(imported.SubscriptionIgnores) != 2 {
		t.Fatalf("count = %d, want 2", len(imported.SubscriptionIgnores))
	}
	si := imported.SubscriptionIgnores[0]
	if si.ID != "si1" {
		t.Errorf("ID = %q, want si1", si.ID)
	}
	if si.SubName != "Morning Coffee" {
		t.Errorf("SubName = %q, want Morning Coffee", si.SubName)
	}
	if !si.IgnoredOn.Equal(ignoredOn) {
		t.Errorf("IgnoredOn = %v, want %v", si.IgnoredOn, ignoredOn)
	}

	// Verify the SQLite load/snapshot path also round-trips.
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()
	if err := st.Load(imported); err != nil {
		t.Fatalf("load: %v", err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.SubscriptionIgnores) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snap.SubscriptionIgnores))
	}
	if snap.SubscriptionIgnores[1].SubName != "Gym App" {
		t.Errorf("si2 SubName = %q, want Gym App", snap.SubscriptionIgnores[1].SubName)
	}
}

func TestSubscriptionIgnoreCRUD(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	ig := domain.SubscriptionIgnore{
		ID:        "x1",
		SubName:   "Morning Coffee",
		IgnoredOn: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := st.PutSubscriptionIgnore(ig); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := st.GetSubscriptionIgnore("x1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.SubName != "Morning Coffee" {
		t.Errorf("SubName = %q, want Morning Coffee", got.SubName)
	}
	list, err := st.ListSubscriptionIgnores()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	deleted, err := st.DeleteSubscriptionIgnore("x1")
	if err != nil || !deleted {
		t.Fatalf("delete: ok=%v err=%v", deleted, err)
	}
	list2, _ := st.ListSubscriptionIgnores()
	if len(list2) != 0 {
		t.Errorf("after delete: len=%d, want 0", len(list2))
	}
}

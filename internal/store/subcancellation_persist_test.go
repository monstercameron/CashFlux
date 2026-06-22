package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSubscriptionCancellationRoundTrip(t *testing.T) {
	cancelledOn := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	ds := Dataset{
		SubscriptionCancellations: []domain.SubscriptionCancellation{
			{ID: "sc1", SubName: "Netflix", CancelledOn: cancelledOn},
			{ID: "sc2", SubName: "Spotify", CancelledOn: cancelledOn.AddDate(0, -1, 0)},
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
	if len(imported.SubscriptionCancellations) != 2 {
		t.Fatalf("count = %d, want 2", len(imported.SubscriptionCancellations))
	}
	sc := imported.SubscriptionCancellations[0]
	if sc.ID != "sc1" {
		t.Errorf("ID = %q, want sc1", sc.ID)
	}
	if sc.SubName != "Netflix" {
		t.Errorf("SubName = %q, want Netflix", sc.SubName)
	}
	if !sc.CancelledOn.Equal(cancelledOn) {
		t.Errorf("CancelledOn = %v, want %v", sc.CancelledOn, cancelledOn)
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
	if len(snap.SubscriptionCancellations) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snap.SubscriptionCancellations))
	}
	if snap.SubscriptionCancellations[1].SubName != "Spotify" {
		t.Errorf("sc2 SubName = %q, want Spotify", snap.SubscriptionCancellations[1].SubName)
	}
}

func TestSubscriptionCancellationCRUD(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	sc := domain.SubscriptionCancellation{
		ID:          "x1",
		SubName:     "Netflix",
		CancelledOn: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
	}
	if err := st.PutSubscriptionCancellation(sc); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := st.GetSubscriptionCancellation("x1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.SubName != "Netflix" {
		t.Errorf("SubName = %q, want Netflix", got.SubName)
	}
	list, err := st.ListSubscriptionCancellations()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	deleted, err := st.DeleteSubscriptionCancellation("x1")
	if err != nil || !deleted {
		t.Fatalf("delete: ok=%v err=%v", deleted, err)
	}
	list2, _ := st.ListSubscriptionCancellations()
	if len(list2) != 0 {
		t.Errorf("after delete: len=%d, want 0", len(list2))
	}
}

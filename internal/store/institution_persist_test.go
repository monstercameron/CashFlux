// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestInstitutionRoundTrip(t *testing.T) {
	ds := Dataset{
		Institutions: []domain.Institution{
			{ID: "chase", Name: "Chase", Color: "#1a73e8", SupportPhone: "1-800-935-9935", SupportURL: "https://chase.com"},
			{ID: "fid", Name: "Fidelity", Note: "Retirement accounts"},
		},
		Accounts: []domain.Account{
			{ID: "a1", Name: "Checking", InstitutionID: "chase", BeneficiaryNote: "TOD to Jane",
				DocRefs: []domain.AccountDocRef{{ArtifactID: "doc1", Label: "Statement", AttachedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)}}},
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
	reexported, _ := Export(imported)
	if !bytes.Equal(exported, reexported) {
		t.Errorf("round-trip not lossless")
	}
	if len(imported.Institutions) != 2 {
		t.Fatalf("institutions = %d, want 2", len(imported.Institutions))
	}
	ac := imported.Accounts[0]
	if ac.InstitutionID != "chase" || ac.BeneficiaryNote != "TOD to Jane" {
		t.Errorf("account fields not preserved: %+v", ac)
	}
	if len(ac.DocRefs) != 1 || ac.DocRefs[0].Label != "Statement" || ac.DocRefs[0].ExpiresAt.IsZero() {
		t.Errorf("doc refs not preserved: %+v", ac.DocRefs)
	}

	// SQLite load/snapshot path.
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
	if len(snap.Institutions) != 2 {
		t.Fatalf("snapshot institutions = %d, want 2", len(snap.Institutions))
	}
}

func TestInstitutionCRUD(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	if err := st.PutInstitution(domain.Institution{ID: "i1", Name: "Chase"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := st.GetInstitution("i1")
	if err != nil || !ok || got.Name != "Chase" {
		t.Fatalf("get: ok=%v err=%v name=%q", ok, err, got.Name)
	}
	list, err := st.ListInstitutions()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	deleted, err := st.DeleteInstitution("i1")
	if err != nil || !deleted {
		t.Fatalf("delete: ok=%v err=%v", deleted, err)
	}
	list2, _ := st.ListInstitutions()
	if len(list2) != 0 {
		t.Errorf("after delete: len=%d, want 0", len(list2))
	}
}

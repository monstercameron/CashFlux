package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/customfields"
)

func TestCustomFieldDefCRUD(t *testing.T) {
	s := newStore(t)

	def := customfields.Def{
		ID: "cf1", EntityType: "account", Key: "tier", Label: "Tier",
		Type: customfields.TypeSelect, Options: []string{"gold", "silver"},
	}
	if err := s.PutCustomFieldDef(def); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetCustomFieldDef("cf1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Label != "Tier" || got.Type != customfields.TypeSelect || len(got.Options) != 2 {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// A second entity type, to test the per-entity query.
	if err := s.PutCustomFieldDef(customfields.Def{ID: "cf2", EntityType: "transaction", Key: "ref", Label: "Reference", Type: customfields.TypeText}); err != nil {
		t.Fatalf("Put 2: %v", err)
	}
	acctDefs, err := s.CustomFieldDefsByEntity("account")
	if err != nil {
		t.Fatalf("ByEntity: %v", err)
	}
	if len(acctDefs) != 1 || acctDefs[0].ID != "cf1" {
		t.Errorf("expected only cf1 for account, got %+v", acctDefs)
	}

	all, err := s.ListCustomFieldDefs()
	if err != nil || len(all) != 2 {
		t.Fatalf("List: n=%d err=%v", len(all), err)
	}

	ok, err = s.DeleteCustomFieldDef("cf1")
	if err != nil || !ok {
		t.Fatalf("Delete: ok=%v err=%v", ok, err)
	}
	if _, ok, _ := s.GetCustomFieldDef("cf1"); ok {
		t.Error("cf1 should be gone")
	}
}

func TestCustomFieldDefsRoundTripDataset(t *testing.T) {
	s := newStore(t)
	in := Dataset{
		CustomFields: []customfields.Def{
			{ID: "cf1", EntityType: "account", Key: "tier", Label: "Tier", Type: customfields.TypeText},
		},
	}
	if err := s.Load(in); err != nil {
		t.Fatalf("Load: %v", err)
	}
	out, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(out.CustomFields) != 1 || out.CustomFields[0].ID != "cf1" {
		t.Errorf("custom field defs lost on round-trip: %+v", out.CustomFields)
	}

	// Export → Import must preserve the defs too.
	raw, err := Export(out)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	back, err := Import(raw)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(back.CustomFields) != 1 || back.CustomFields[0].Key != "tier" {
		t.Errorf("custom field defs lost on export/import: %+v", back.CustomFields)
	}
}

func TestWipeClearsCustomFieldDefs(t *testing.T) {
	s := newStore(t)
	if err := s.PutCustomFieldDef(customfields.Def{ID: "cf1", EntityType: "account", Key: "k", Label: "L", Type: customfields.TypeText}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Wipe(); err != nil {
		t.Fatalf("Wipe: %v", err)
	}
	all, err := s.ListCustomFieldDefs()
	if err != nil || len(all) != 0 {
		t.Fatalf("expected no defs after wipe, n=%d err=%v", len(all), err)
	}
}

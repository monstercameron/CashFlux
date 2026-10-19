// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTransactionOwnersAreStableAndDeduplicated(t *testing.T) {
	txn := Transaction{MemberIDs: []string{"m2", "m1", "m2", ""}}
	want := []string{"m2", "m1"}
	if got := txn.Owners(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Owners() = %#v, want %#v", got, want)
	}
	if !txn.HasOwner("m1") || txn.HasOwner("m3") || txn.HasOwner("") {
		t.Fatal("HasOwner did not identify membership exactly")
	}
	if txn.IsUnassigned() {
		t.Fatal("owned transaction reported unassigned")
	}
}

func TestLegacyMemberIDMigratesAndSaveWritesOnlyMemberIDs(t *testing.T) {
	var txn Transaction
	if err := json.Unmarshal([]byte(`{"id":"t1","memberId":"m1"}`), &txn); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(txn.MemberIDs, []string{"m1"}) || txn.MemberID != "" {
		t.Fatalf("legacy migration = %#v", txn)
	}
	encoded, err := json.Marshal(Transaction{ID: "t1", MemberID: "old", MemberIDs: []string{"m2", "m2"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"memberId":`) || !strings.Contains(string(encoded), `"memberIds":["m2"]`) {
		t.Fatalf("noncanonical ownership JSON: %s", encoded)
	}
}

func TestUnassignedAndSetOwners(t *testing.T) {
	var txn Transaction
	if !txn.IsUnassigned() {
		t.Fatal("zero transaction should be unassigned")
	}
	txn.SetOwners([]string{"m1", "m1", "m2", ""})
	if !reflect.DeepEqual(txn.MemberIDs, []string{"m1", "m2"}) || txn.MemberID != "" {
		t.Fatalf("SetOwners did not canonicalize: %#v", txn)
	}
}

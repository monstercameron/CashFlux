// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"testing"
)

func TestV5BulkOwnerChoicesApplyOnceWhenPickerCloses(t *testing.T) {
	var got []string
	calls := 0
	selected := txnBulkOwnerChoice(nil, "member-1")
	selected = txnBulkOwnerChoice(selected, "member-2")
	if calls != 0 {
		t.Fatal("owner choice applied before the picker closed")
	}
	commitTxnBulkOwnerPick(selected, func(owners []string) {
		calls++
		got = append([]string(nil), owners...)
	})
	if calls != 1 {
		t.Fatalf("owner picker close applied %d times, want once", calls)
	}
	if !reflect.DeepEqual(got, []string{"member-1", "member-2"}) {
		t.Fatalf("applied owners = %v, want both selected owners", got)
	}
	if !reflect.DeepEqual(selected, got) {
		t.Fatalf("staged owner selection = %v, applied owners = %v", selected, got)
	}
}

func TestV5BulkUnassignedChoiceAppliesEmptyOwnerSet(t *testing.T) {
	var got []string
	commitTxnBulkOwnerPick(txnBulkOwnerChoice([]string{"member-1"}, txnBulkUnassignedOwner), func(owners []string) {
		got = owners
	})
	if len(got) != 0 {
		t.Fatalf("applied unassigned owners = %v, want empty", got)
	}
}

// SPDX-License-Identifier: MIT

package extract

import (
	"testing"
)

func TestIMAExtractPreservesExactJSONMoney(t *testing.T) {
	rows, err := ParseRows(`[{"description":"Large","amount":90071992547409.93}]`)
	if err != nil || len(rows) != 1 || rows[0].Amount != "90071992547409.93" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if _, err := ParseRows(`[{"description":"Large","amount":1}] {}`); err == nil {
		t.Fatal("accepted trailing JSON")
	}
	a := Row{Amount: "90071992547409.93", Currency: "USD"}
	b := Row{Amount: "90071992547409.94", Currency: "USD"}
	if a.Signature() == b.Signature() {
		t.Fatal("large amounts lost a cent in signature")
	}
	jpyA := Row{Amount: "123", Currency: "JPY"}
	jpyB := Row{Amount: "+123", Currency: "JPY"}
	if jpyA.Signature() != jpyB.Signature() {
		t.Fatal("JPY integer amounts differ")
	}
}

func TestIMADraftRowIDsSurviveEditsAndRemoval(t *testing.T) {
	rows := EnsureRowIDs([]Row{{Description: "Metro", Amount: "-2.50"}, {Description: "Metro", Amount: "-2.50"}})
	if rows[0].ID == "" || rows[0].ID == rows[1].ID {
		t.Fatal("equal purchases need different identities")
	}
	remaining := EnsureRowIDs(rows[1:])
	remaining[0].Description = "Edited"
	if remaining[0].ID != rows[1].ID || rows[1].Description != "Metro" {
		t.Fatal("row identity changed or input was mutated")
	}
}

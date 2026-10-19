// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestDebtPaymentTypesRoundTrip(t *testing.T) {
	created := time.Date(2026, 10, 19, 12, 30, 0, 0, time.UTC)
	want := DebtPaymentLink{
		ID: "dp1", TransactionIDs: []string{"checking-txn", "loan-txn"}, DebtAccountID: "loan",
		Currency: "USD", PrincipalMinor: 8000, InterestMinor: 1500, FeesMinor: 200,
		EscrowMinor: 300, Estimated: true, Method: DebtPaymentAmortized,
		Source: DebtPaymentConfirmed, CreatedRevision: 42, CreatedAt: created,
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal link: %v", err)
	}
	var got DebtPaymentLink
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal link: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip = %#v, want %#v", got, want)
	}
}

func TestDebtPayeeRuleRoundTripAndOptionalSource(t *testing.T) {
	want := DebtPayeeRule{ID: "rule1", WorkspaceID: "workspace1", Payee: "CHASE AUTOPAY", DebtAccountID: "card", CreatedAt: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal rule: %v", err)
	}
	var got DebtPayeeRule
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal rule: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip = %#v, want %#v", got, want)
	}
	if got.SourceAccountID != "" || got.WorkspaceID == "" {
		t.Fatalf("unexpected rule scope: %#v", got)
	}
}

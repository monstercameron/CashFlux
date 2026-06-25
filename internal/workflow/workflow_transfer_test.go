// SPDX-License-Identifier: MIT

package workflow

import "testing"

func TestValidateTransferAction(t *testing.T) {
	valid := Action{
		Kind:                  ActionTransfer,
		TransferFromAccountID: "acc-checking",
		TransferToAccountID:   "acc-savings",
		TransferAmount:        5000,
	}

	tests := []struct {
		name        string
		action      Action
		trigger     TriggerKind
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid transfer on scheduled trigger",
			action:  valid,
			trigger: TriggerScheduled,
			wantErr: false,
		},
		{
			name:    "valid transfer on manual trigger",
			action:  valid,
			trigger: TriggerManual,
			wantErr: false,
		},
		{
			name:        "missing source account ID",
			action:      Action{Kind: ActionTransfer, TransferToAccountID: "acc-savings", TransferAmount: 5000},
			trigger:     TriggerScheduled,
			wantErr:     true,
			errContains: "source account",
		},
		{
			name:        "missing destination account ID",
			action:      Action{Kind: ActionTransfer, TransferFromAccountID: "acc-checking", TransferAmount: 5000},
			trigger:     TriggerScheduled,
			wantErr:     true,
			errContains: "destination account",
		},
		{
			name:        "zero amount",
			action:      Action{Kind: ActionTransfer, TransferFromAccountID: "acc-checking", TransferToAccountID: "acc-savings", TransferAmount: 0},
			trigger:     TriggerScheduled,
			wantErr:     true,
			errContains: "positive",
		},
		{
			name:        "negative amount",
			action:      Action{Kind: ActionTransfer, TransferFromAccountID: "acc-checking", TransferToAccountID: "acc-savings", TransferAmount: -100},
			trigger:     TriggerScheduled,
			wantErr:     true,
			errContains: "positive",
		},
		{
			name:        "txn-added trigger is rejected (loop guard)",
			action:      valid,
			trigger:     TriggerTxnAdded,
			wantErr:     true,
			errContains: "loop",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTransferAction(tc.action, tc.trigger)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && tc.errContains != "" {
				if got := err.Error(); !contains(got, tc.errContains) {
					t.Errorf("error %q does not contain %q", got, tc.errContains)
				}
			}
		})
	}
}

func TestPlanActionTransfer(t *testing.T) {
	a := Action{
		Kind:                  ActionTransfer,
		TransferFromAccountID: "acc-checking",
		TransferToAccountID:   "acc-savings",
		TransferAmount:        10000,
		DedupeKey:             "pyf:wf-1:2026-06",
	}
	e := planAction(a, Context{})
	if e.Kind != ActionTransfer {
		t.Errorf("kind: got %q, want %q", e.Kind, ActionTransfer)
	}
	if e.TransferFromAccountID != a.TransferFromAccountID {
		t.Errorf("from: got %q, want %q", e.TransferFromAccountID, a.TransferFromAccountID)
	}
	if e.TransferToAccountID != a.TransferToAccountID {
		t.Errorf("to: got %q, want %q", e.TransferToAccountID, a.TransferToAccountID)
	}
	if e.TransferAmount != a.TransferAmount {
		t.Errorf("amount: got %d, want %d", e.TransferAmount, a.TransferAmount)
	}
	if e.DedupeKey != a.DedupeKey {
		t.Errorf("dedupeKey: got %q, want %q", e.DedupeKey, a.DedupeKey)
	}
	if e.Summary == "" {
		t.Error("summary should be non-empty")
	}
}

// contains is a substring helper to avoid importing strings in the test.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

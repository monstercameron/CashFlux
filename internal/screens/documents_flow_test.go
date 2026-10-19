// SPDX-License-Identifier: MIT

package screens

import "testing"

func TestImportFlowScreenFor(t *testing.T) {
	tests := []struct {
		name  string
		state importFlowState
		want  importFlowScreen
	}{
		{"picker", importFlowState{Stage: importFlowInput}, importScreenStart},
		{"csv paste", importFlowState{Stage: importFlowInput, Source: importSourceCSV}, importScreenCSV},
		{"statement csv", importFlowState{Stage: importFlowInput, Source: importSourceStmt}, importScreenStmt},
		{"statement pdf", importFlowState{Stage: importFlowInput, Source: importSourcePDF}, importScreenPDF},
		{"receipt", importFlowState{Stage: importFlowInput, Source: importSourceReceipt}, importScreenReceipt},
		{"order", importFlowState{Stage: importFlowInput, Source: importSourceAmazon}, importScreenOrder},
		{"review wins over source", importFlowState{Stage: importFlowReview, Source: importSourceCSV}, importScreenReview},
		{"unknown source returns to picker", importFlowState{Stage: importFlowInput, Source: "unknown"}, importScreenStart},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := importFlowScreenFor(tt.state); got != tt.want {
				t.Fatalf("importFlowScreenFor(%+v) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestTransitionImportFlow(t *testing.T) {
	tests := []struct {
		name   string
		state  importFlowState
		action importFlowAction
		want   importFlowState
	}{
		{"next keeps source", importFlowState{Stage: importFlowInput, Source: importSourceStmt}, importActionNext, importFlowState{Stage: importFlowReview, Source: importSourceStmt}},
		{"back from review returns to picker", importFlowState{Stage: importFlowReview, Source: importSourceReceipt}, importActionBack, importFlowState{Stage: importFlowInput, Source: importSourcePicker}},
		{"back from source returns to picker", importFlowState{Stage: importFlowInput, Source: importSourcePDF}, importActionBack, importFlowState{Stage: importFlowInput, Source: importSourcePicker}},
		{"add more returns to picker", importFlowState{Stage: importFlowReview, Source: importSourceCSV}, importActionAddMore, importFlowState{Stage: importFlowInput, Source: importSourcePicker}},
		{"unknown action is inert", importFlowState{Stage: importFlowInput, Source: importSourceAmazon}, "unknown", importFlowState{Stage: importFlowInput, Source: importSourceAmazon}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transitionImportFlow(tt.state, tt.action); got != tt.want {
				t.Fatalf("transitionImportFlow(%+v, %q) = %+v, want %+v", tt.state, tt.action, got, tt.want)
			}
		})
	}
}

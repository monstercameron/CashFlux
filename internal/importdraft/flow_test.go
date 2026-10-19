// SPDX-License-Identifier: MIT

package importdraft

import "testing"

func TestImportFlowScreenFor(t *testing.T) {
	tests := []struct {
		name  string
		state FlowState
		want  FlowScreen
	}{
		{"picker", FlowState{Stage: FlowInput}, ScreenStart},
		{"csv paste", FlowState{Stage: FlowInput, Source: SourceCSV}, ScreenCSV},
		{"statement csv", FlowState{Stage: FlowInput, Source: SourceStmt}, ScreenStmt},
		{"statement pdf", FlowState{Stage: FlowInput, Source: SourcePDF}, ScreenPDF},
		{"receipt", FlowState{Stage: FlowInput, Source: SourceReceipt}, ScreenReceipt},
		{"order", FlowState{Stage: FlowInput, Source: SourceAmazon}, ScreenOrder},
		{"review wins over source", FlowState{Stage: FlowReview, Source: SourceCSV}, ScreenReview},
		{"unknown source returns to picker", FlowState{Stage: FlowInput, Source: "unknown"}, ScreenStart},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlowScreenFor(tt.state); got != tt.want {
				t.Fatalf("FlowScreenFor(%+v) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestTransitionImportFlow(t *testing.T) {
	tests := []struct {
		name   string
		state  FlowState
		action FlowAction
		want   FlowState
	}{
		{"next keeps source", FlowState{Stage: FlowInput, Source: SourceStmt}, ActionNext, FlowState{Stage: FlowReview, Source: SourceStmt}},
		{"back from review returns to picker", FlowState{Stage: FlowReview, Source: SourceReceipt}, ActionBack, FlowState{Stage: FlowInput, Source: SourcePicker}},
		{"back from source returns to picker", FlowState{Stage: FlowInput, Source: SourcePDF}, ActionBack, FlowState{Stage: FlowInput, Source: SourcePicker}},
		{"add more returns to picker", FlowState{Stage: FlowReview, Source: SourceCSV}, ActionAddMore, FlowState{Stage: FlowInput, Source: SourcePicker}},
		{"unknown action is inert", FlowState{Stage: FlowInput, Source: SourceAmazon}, "unknown", FlowState{Stage: FlowInput, Source: SourceAmazon}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TransitionFlow(tt.state, tt.action); got != tt.want {
				t.Fatalf("TransitionFlow(%+v, %q) = %+v, want %+v", tt.state, tt.action, got, tt.want)
			}
		})
	}
}

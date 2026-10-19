// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

func TestTC4N9JournalCheckpointThreshold(t *testing.T) {
	tests := []struct {
		name        string
		initialized bool
		forced      bool
		records     int
		want        bool
	}{
		{name: "first write establishes checkpoint", want: true},
		{name: "ordinary short journal appends", initialized: true, records: journalCheckpointRecordLimit - 1},
		{name: "threshold compacts", initialized: true, records: journalCheckpointRecordLimit, want: true},
		{name: "forced checkpoint", initialized: true, forced: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCheckpointJournal(tt.initialized, tt.forced, tt.records); got != tt.want {
				t.Fatalf("shouldCheckpointJournal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTC4N9JournalKeysWalkAndRejectCycles(t *testing.T) {
	prefix := recordWorkspacePrefix("test-workspace")
	tail, parent, row1, row2 := prefix+"tail", prefix+"parent", prefix+"row-1", prefix+"row-2"
	values := map[string]string{
		tail:   `{"parent":"` + parent + `","keys":["` + row2 + `"]}`,
		parent: `{"keys":["` + row1 + `"]}`,
	}
	read := func(key string) (string, bool, error) { value, ok := values[key]; return value, ok, nil }
	keys, err := journalKeysFromHead(datasetRecordHead{Tail: tail}, "test-workspace", read)
	if err != nil || len(keys) != 4 || keys[0] != row2 || keys[1] != tail || keys[2] != row1 || keys[3] != parent {
		t.Fatalf("journalKeysFromHead() = %#v, %v", keys, err)
	}
	values[parent] = `{"parent":"` + tail + `","keys":[]}`
	if _, err := journalKeysFromHead(datasetRecordHead{Tail: tail}, "test-workspace", read); err == nil {
		t.Fatal("cyclic journal chain was accepted")
	}
}

func TestTC4N9StaleGenerationUsesNonRetryableError(t *testing.T) {
	err := datasetSaveGenerationError(true, false)
	if !errors.Is(err, appstate.ErrStaleDataset) {
		t.Fatalf("stale generation error = %v, want ErrStaleDataset", err)
	}
	if retryable := datasetSaveGenerationError(false, false); errors.Is(retryable, appstate.ErrStaleDataset) {
		t.Fatalf("workspace switch was misclassified as stale generation: %v", retryable)
	}
}

func TestTC4N9ToastParsesCurrentSavedBalanceCopy(t *testing.T) {
	main, detail := toastV5RowCopy("Saved. Checking is now $2,140.12.")
	if main != "Transaction saved" || detail != "Checking balance $2,140.12" {
		t.Fatalf("toast copy = %q / %q", main, detail)
	}
}

func TestTC4N9ToastDoesNotRetryStaleDurabilityFailures(t *testing.T) {
	if toastDurabilityFailureIsRetryable(fmt.Errorf("save: %w", appstate.ErrStaleDataset)) {
		t.Fatal("stale generation conflict was offered as a toast retry")
	}
	if !toastDurabilityFailureIsRetryable(errors.New("storage unavailable")) {
		t.Fatal("ordinary storage failure lost its retry")
	}
}

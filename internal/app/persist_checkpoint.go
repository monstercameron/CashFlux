// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/store"
)

// Record checkpoints deliberately carry a future dataset schema at the outer
// level. Older app bundles must quarantine them instead of opening the older
// base beneath a journal they do not understand and overwriting saved edits.
func encodeRecordCheckpoint(dataset []byte) string {
	return encodeRecordCheckpointGeneration(dataset, "")
}

func encodeRecordCheckpointGeneration(dataset []byte, generation string) string {
	metadata := ""
	if generation != "" {
		raw, _ := json.Marshal(generation)
		metadata = `,"generation":` + string(raw)
	}
	return `{"schemaVersion":` + strconv.Itoa(store.SchemaVersion+1) + `,"cashfluxRecordCheckpoint":1` + metadata + `,"dataset":` + string(dataset) + `}`
}

func checkpointGeneration(raw string) string {
	var metadata struct {
		Generation string `json:"generation"`
	}
	_ = json.Unmarshal([]byte(raw), &metadata)
	return metadata.Generation
}

func checkpointDatasetJSON(raw []byte) ([]byte, bool, error) {
	var header struct {
		Version int             `json:"cashfluxRecordCheckpoint"`
		Schema  int             `json:"schemaVersion"`
		Dataset json.RawMessage `json:"dataset"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, false, fmt.Errorf("read dataset checkpoint: %w", err)
	}
	if header.Version == 0 {
		return raw, false, nil
	}
	// Schema is a compatibility ceiling, not an exact version check. Checkpoints
	// written before an additive dataset schema bump carry the then-current
	// ceiling and remain safe to unwrap: the inner Dataset is independently
	// migrated by store.Import. A larger ceiling still means this build may be
	// too old to understand the journal and must fail closed.
	if header.Version != 1 || header.Schema <= 0 || header.Schema > store.SchemaVersion+1 || len(header.Dataset) == 0 {
		return nil, true, fmt.Errorf("unsupported record checkpoint")
	}
	return header.Dataset, true, nil
}

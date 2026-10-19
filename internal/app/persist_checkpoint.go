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
	return `{"schemaVersion":` + strconv.Itoa(store.SchemaVersion+1) + `,"cashfluxRecordCheckpoint":1,"dataset":` + string(dataset) + `}`
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
	if header.Version != 1 || header.Schema != store.SchemaVersion+1 || len(header.Dataset) == 0 {
		return nil, true, fmt.Errorf("unsupported record checkpoint")
	}
	return header.Dataset, true, nil
}

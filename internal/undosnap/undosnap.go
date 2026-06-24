// SPDX-License-Identifier: MIT

// Package undosnap converts between CashFlux's export-JSON wire format and the
// history.Snapshot type used by the diff/undo engine.
//
// CashFlux's ExportJSON produces a top-level JSON object whose values are either
// JSON arrays of entity rows (each row has an "id" field) or scalar values such
// as settings objects and a schemaVersion string.  A history.Snapshot is
// map[collection]map[rowID]rowJSON, so arrays need to be exploded into per-id
// maps and scalars need a synthetic home.
//
// Scalar fields are stored under collection names prefixed with "_meta:" (e.g.
// "_meta:settings", "_meta:schemaVersion"), keyed by that same field name so
// the roundtrip is lossless.  Collection names that start with "_meta:" are
// therefore reserved.
package undosnap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/history"
)

// ToSnapshot converts a raw ExportJSON payload into a history.Snapshot.
//
// For every top-level key whose value is a JSON array the array is expanded:
// each element is parsed for its "id" field and stored as
// snapshot[key][id] = elementJSON.  An element without an "id" field causes an
// error so no data is silently dropped.
//
// For every top-level key whose value is NOT an array (object, string, number,
// bool, null) the value is stored as snapshot["_meta:"+key][key] = valueJSON,
// giving it a stable home that survives round-trips.
func ToSnapshot(exportJSON []byte) (history.Snapshot, error) {
	// Unmarshal the top-level object into a key→raw-value map preserving JSON
	// exactly as-is (no float64 coercion on ids etc.).
	var top map[string]json.RawMessage
	if err := json.Unmarshal(exportJSON, &top); err != nil {
		return nil, fmt.Errorf("undosnap: top-level unmarshal: %w", err)
	}

	snap := make(history.Snapshot, len(top))

	for key, raw := range top {
		if strings.HasPrefix(key, "_meta:") {
			return nil, fmt.Errorf("undosnap: reserved key %q in export JSON", key)
		}
		// Detect arrays by peeking at the first non-whitespace byte.
		trimmed := strings.TrimSpace(string(raw))
		if len(trimmed) > 0 && trimmed[0] == '[' {
			// Array of entity rows.
			var rows []json.RawMessage
			if err := json.Unmarshal(raw, &rows); err != nil {
				return nil, fmt.Errorf("undosnap: unmarshal array %q: %w", key, err)
			}
			coll := make(map[string]json.RawMessage, len(rows))
			for i, elem := range rows {
				id, err := extractID(elem)
				if err != nil {
					return nil, fmt.Errorf("undosnap: element %d of %q: %w", i, key, err)
				}
				coll[id] = elem
			}
			snap[key] = coll
		} else {
			// Scalar / object / null — store in a synthetic _meta collection.
			metaKey := "_meta:" + key
			snap[metaKey] = map[string]json.RawMessage{key: raw}
		}
	}

	return snap, nil
}

// ToJSON reassembles a history.Snapshot produced by ToSnapshot back into a
// CashFlux export-JSON payload.  Array collections are sorted by id for
// determinism; _meta:* scalars are lifted back to top-level fields.
func ToJSON(s history.Snapshot) ([]byte, error) {
	top := make(map[string]json.RawMessage, len(s))

	for coll, rows := range s {
		if strings.HasPrefix(coll, "_meta:") {
			// Each _meta collection contains exactly one entry keyed by the original
			// field name.
			for fieldName, raw := range rows {
				top[fieldName] = raw
			}
			continue
		}
		// Rebuild a sorted array.
		ids := make([]string, 0, len(rows))
		for id := range rows {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		arr := make([]json.RawMessage, 0, len(ids))
		for _, id := range ids {
			arr = append(arr, rows[id])
		}
		b, err := json.Marshal(arr)
		if err != nil {
			return nil, fmt.Errorf("undosnap: marshal array %q: %w", coll, err)
		}
		top[coll] = b
	}

	out, err := json.Marshal(top)
	if err != nil {
		return nil, fmt.Errorf("undosnap: marshal top-level: %w", err)
	}
	return out, nil
}

// extractID parses the "id" string field from a raw JSON object.
func extractID(raw json.RawMessage) (string, error) {
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("unmarshal for id: %w", err)
	}
	if obj.ID == "" {
		return "", fmt.Errorf("element has no non-empty \"id\" field")
	}
	return obj.ID, nil
}

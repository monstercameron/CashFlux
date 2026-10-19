// SPDX-License-Identifier: MIT

package changes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// DiffDocuments computes the top-level field diff between two JSON object
// documents, as stored one-document-per-record by internal/store.
//
//   - A nil (or empty) before means "create": every field of after is a write.
//   - A nil (or empty) after means "delete": the op is a tombstone.
//   - Fields present only in before are listed in Op.Removed.
//   - Fields whose canonical JSON differs are writes; equal fields are
//     omitted, so unchanged documents yield ("", false).
//
// Nested objects are compared and replaced whole: if "address.city" changes,
// the diff writes the entire "address" value. This keeps every field an
// atomic merge unit with one stamp, matching how the store rewrites whole
// documents; finer-grained merging of nested objects would need a schema for
// each nested shape. Key order and whitespace inside values never count as
// changes because comparison uses the canonical encoding.
//
// DiffDocuments has no error return: an unreadable or non-object document
// yields false. Callers that must distinguish "unchanged" from "unreadable"
// (for example to quarantine corrupt data instead of silently skipping it)
// should use DiffDocumentsErr.
func DiffDocuments(table, id string, before, after []byte) (Op, bool) {
	op, ok, err := DiffDocumentsErr(table, id, before, after)
	if err != nil {
		return Op{}, false
	}
	return op, ok
}

// DiffDocumentsErr is DiffDocuments with the parse error surfaced.
func DiffDocumentsErr(table, id string, before, after []byte) (Op, bool, error) {
	if err := checkName("table", table); err != nil {
		return Op{}, false, err
	}
	if err := checkName("recordId", id); err != nil {
		return Op{}, false, err
	}
	beforeAbsent := len(bytes.TrimSpace(before)) == 0
	afterAbsent := len(bytes.TrimSpace(after)) == 0
	switch {
	case beforeAbsent && afterAbsent:
		return Op{}, false, nil
	case afterAbsent:
		// The prior document need not parse: deleting a record whose stored
		// bytes are corrupt must still be possible.
		return Op{Table: table, RecordID: id, Deleted: true}, true, nil
	case beforeAbsent:
		fields, err := parseObject(after)
		if err != nil {
			return Op{}, false, fmt.Errorf("changes: diff %s/%s: after: %w", table, id, err)
		}
		return Op{Table: table, RecordID: id, Fields: fields}, true, nil
	}
	beforeFields, err := parseObject(before)
	if err != nil {
		return Op{}, false, fmt.Errorf("changes: diff %s/%s: before: %w", table, id, err)
	}
	afterFields, err := parseObject(after)
	if err != nil {
		return Op{}, false, fmt.Errorf("changes: diff %s/%s: after: %w", table, id, err)
	}
	op := Op{Table: table, RecordID: id}
	for name, afterRaw := range afterFields {
		beforeRaw, existed := beforeFields[name]
		if !existed || !bytes.Equal(beforeRaw, afterRaw) {
			if op.Fields == nil {
				op.Fields = make(map[string]json.RawMessage)
			}
			op.Fields[name] = afterRaw
		}
	}
	for name := range beforeFields {
		if _, kept := afterFields[name]; !kept {
			op.Removed = append(op.Removed, name)
		}
	}
	sort.Strings(op.Removed)
	if len(op.Fields) == 0 && len(op.Removed) == 0 {
		return Op{}, false, nil
	}
	return op, true, nil
}

// parseObject decodes a JSON object document into canonical raw field
// values. Non-object documents are rejected with ErrInvalidDocument.
func parseObject(data []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDocument, preview(trimmed))
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	out := make(map[string]json.RawMessage, len(rawFields))
	for name, raw := range rawFields {
		canon, err := canonicalJSONValue(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: field %q: %v", ErrInvalidDocument, name, err)
		}
		out[name] = canon
	}
	return out, nil
}

// preview shortens a document for an error message.
func preview(data []byte) string {
	const maxPreview = 32
	if len(data) <= maxPreview {
		return string(data)
	}
	return string(data[:maxPreview]) + "..."
}

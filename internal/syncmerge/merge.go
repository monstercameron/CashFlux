// SPDX-License-Identifier: MIT

// Package syncmerge provides pure, platform-independent helpers for merging
// workspace sync records. All functions are safe to call from native Go tests
// and from wasm — no syscall/js imports.
//
// # Design
//
// Workspaces are treated as flat key→(value, updatedAt) maps called Records.
// Each field carries its own last-modified timestamp so that two devices that
// changed different fields can be reconciled without either write being lost.
//
// # Conflict policy
//
// A conflict occurs when both local and remote changed the same field relative
// to the common base and they chose different values. In that case the
// higher-revision (later UpdatedAt) wins, but the losing value is never
// silently discarded — it is recorded in a ConflictEntry so the caller can
// surface it to the user.
package syncmerge

import "time"

// FieldValue pairs a field's value with the timestamp of the last write to
// that field. The zero UpdatedAt means the field has never been written.
type FieldValue struct {
	Value     string
	UpdatedAt time.Time
}

// Record is a workspace snapshot modelled as a named set of field values.
// Each key maps to the field's current value and the time it was last written.
type Record map[string]FieldValue

// ConflictEntry records a field where local and remote disagreed. The Chosen
// value was selected by last-write-wins; the Other value is preserved here so
// the caller can display or log it.
type ConflictEntry struct {
	// Field is the name of the conflicting field.
	Field string
	// LocalValue is the value that existed on the local device.
	LocalValue string
	// RemoteValue is the value received from the remote.
	RemoteValue string
	// ChosenValue is the value that was selected (the later write wins).
	ChosenValue string
	// ChosenSide is "local" or "remote".
	ChosenSide string
}

// MergeRecord performs a deterministic field-level last-writer-wins merge of
// local and remote onto a merged Record. A field is updated whenever one side
// has a strictly later UpdatedAt than the other side's record for that field.
//
// When both sides changed the same field and their timestamps are equal, the
// remote value wins as a tie-break (this is deterministic and prevents
// oscillation). When they differ, the later timestamp wins.
//
// Every field where local and remote hold different values — regardless of
// which side wins — is captured in the returned conflicts slice. Callers
// MUST NOT silently drop this slice; surface it to users or write it to a log.
//
// The base Record is used only by ThreeWayMerge; pass nil here for a pure
// two-way field-level LWW merge.
func MergeRecord(local, remote Record) (merged Record, conflicts []ConflictEntry) {
	merged = make(Record)
	conflicts = []ConflictEntry{}

	// Collect all field names across both sides.
	keys := make(map[string]struct{}, len(local)+len(remote))
	for k := range local {
		keys[k] = struct{}{}
	}
	for k := range remote {
		keys[k] = struct{}{}
	}

	for field := range keys {
		lv, lOk := local[field]
		rv, rOk := remote[field]

		switch {
		case !lOk:
			// Only remote has this field — take it unconditionally.
			merged[field] = rv
		case !rOk:
			// Only local has this field — keep it unconditionally.
			merged[field] = lv
		default:
			// Both sides have the field; compare values.
			if lv.Value == rv.Value {
				// Same value: keep the later timestamp for future merges.
				if rv.UpdatedAt.After(lv.UpdatedAt) {
					merged[field] = rv
				} else {
					merged[field] = lv
				}
				// No conflict — values match.
			} else {
				// Values differ — LWW by timestamp; remote wins on tie.
				var chosen, other FieldValue
				var side, otherSide string
				if lv.UpdatedAt.After(rv.UpdatedAt) {
					chosen, other = lv, rv
					side, otherSide = "local", "remote"
				} else {
					chosen, other = rv, lv
					side, otherSide = "remote", "local"
				}
				merged[field] = chosen
				_ = otherSide
				conflicts = append(conflicts, ConflictEntry{
					Field:       field,
					LocalValue:  lv.Value,
					RemoteValue: rv.Value,
					ChosenValue: chosen.Value,
					ChosenSide:  side,
					// other.Value is the losing side — preserved in Local/RemoteValue above.
				})
				_ = other
			}
		}
	}
	return merged, conflicts
}

// SPDX-License-Identifier: MIT

package syncmerge

// ThreeWayMerge performs a field-level 3-way merge of local and remote against
// a known common base. This is the right algorithm for structured fields where
// a pure LWW merge would lose an edit made on one side that the other side did
// not touch.
//
// Rules per field:
//   - Only local changed it (relative to base) → take local.
//   - Only remote changed it (relative to base) → take remote.
//   - Neither changed it → keep base value (or whichever side has it).
//   - Both changed it to the same value → no conflict, take that value.
//   - Both changed it to different values → conflict; fall back to LWW
//     (later UpdatedAt wins, remote wins on tie).
//
// The base may be nil or empty, in which case this degenerates to a two-way
// LWW merge identical to MergeRecord.
func ThreeWayMerge(base, local, remote Record) (merged Record, conflicts []ConflictEntry) {
	merged = make(Record)
	conflicts = []ConflictEntry{}

	// Collect all field names.
	keys := make(map[string]struct{}, len(base)+len(local)+len(remote))
	for k := range base {
		keys[k] = struct{}{}
	}
	for k := range local {
		keys[k] = struct{}{}
	}
	for k := range remote {
		keys[k] = struct{}{}
	}

	for field := range keys {
		bv, bOk := base[field]
		lv, lOk := local[field]
		rv, rOk := remote[field]

		// A DELETION is a change. This read `lOk && ...`, so a field the local
		// side had removed scored localChanged=false — the delete was invisible,
		// the field fell through to the "only remote has it" arm, and it came back
		// from the other side with no conflict recorded. Deletes propagated in
		// neither direction; every device resurrected what any other had removed.
		localDeleted := bOk && !lOk
		remoteDeleted := bOk && !rOk
		localChanged := localDeleted || (lOk && (!bOk || lv.Value != bv.Value))
		remoteChanged := remoteDeleted || (rOk && (!bOk || rv.Value != bv.Value))

		// Both sides removed it: agreement, and nothing to write.
		if localDeleted && remoteDeleted {
			continue
		}
		// One side deleted while the other edited. That is a real conflict and the
		// caller has to hear about it — a delete beaten silently is the same data
		// loss as an edit beaten silently. The surviving EDIT wins: resurrecting a
		// record is recoverable by deleting it again, whereas honouring the delete
		// discards an edit that no longer exists anywhere.
		if localDeleted && remoteChanged {
			merged[field] = rv
			conflicts = append(conflicts, ConflictEntry{
				Field: field, LocalValue: "", RemoteValue: rv.Value,
				ChosenValue: rv.Value, ChosenSide: "remote",
			})
			continue
		}
		if remoteDeleted && localChanged {
			merged[field] = lv
			conflicts = append(conflicts, ConflictEntry{
				Field: field, LocalValue: lv.Value, RemoteValue: "",
				ChosenValue: lv.Value, ChosenSide: "local",
			})
			continue
		}
		// A delete on one side that the other never touched: honour it by writing
		// nothing.
		if localDeleted || remoteDeleted {
			continue
		}

		switch {
		case localChanged && remoteChanged:
			// Both sides changed the field.
			if lv.Value == rv.Value {
				// Both converged on the same value — no conflict.
				if rv.UpdatedAt.After(lv.UpdatedAt) {
					merged[field] = rv
				} else {
					merged[field] = lv
				}
			} else {
				// True conflict — fall back to LWW, record it.
				var chosen FieldValue
				var side string
				if lv.UpdatedAt.After(rv.UpdatedAt) {
					chosen, side = lv, "local"
				} else {
					chosen, side = rv, "remote"
				}
				merged[field] = chosen
				conflicts = append(conflicts, ConflictEntry{
					Field:       field,
					LocalValue:  lv.Value,
					RemoteValue: rv.Value,
					ChosenValue: chosen.Value,
					ChosenSide:  side,
				})
			}
		case localChanged:
			// Only local touched this field — safe to take local.
			merged[field] = lv
		case remoteChanged:
			// Only remote touched this field — safe to take remote.
			merged[field] = rv
		case lOk:
			// Neither side changed relative to base; keep local (equivalent to base).
			merged[field] = lv
		case rOk:
			merged[field] = rv
		case bOk:
			merged[field] = bv
		}
	}
	return merged, conflicts
}

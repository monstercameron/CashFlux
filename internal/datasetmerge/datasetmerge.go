// SPDX-License-Identifier: MIT

// Package datasetmerge combines two CashFlux dataset snapshots into one
// (TODOS.md C695).
//
// It exists because the two operations people actually need when a household's
// data has ended up split across two accounts are "replace theirs with mine"
// and "put the two together", and only the first is expressible as a copy. The
// second has to reconcile records, and doing that by hand — export both, open
// them in an editor, re-import — is how a year of transactions goes missing.
//
// # Why this works on JSON rather than on the typed Dataset
//
// The server stores a snapshot as opaque bytes and has no business importing
// the client's entity types; a merge that required them would drag the whole
// domain model across the boundary and would need editing every time an entity
// is added. Working structurally instead means an unknown collection merges
// correctly the day it is introduced, and a schema this package has never heard
// of is handled rather than dropped.
//
// # What it will and will not decide
//
// Records are matched by "id". A record only one side has is kept — that is the
// case that makes merging worth doing. A record BOTH sides have, with differing
// content, is a genuine conflict: the data itself says nothing about which is
// right, because CashFlux entities carry no per-record modification time. So the
// caller states a policy up front and every conflict is counted and reported.
// Nothing is resolved by guessing, and nothing is silently dropped.
package datasetmerge

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Policy says which side wins when both hold a different version of one record.
type Policy string

const (
	// PreferTarget keeps the record already in the target snapshot. This is the
	// conservative choice: the account being merged INTO keeps what it has, and
	// the merge only ever adds.
	PreferTarget Policy = "prefer-target"
	// PreferSource takes the incoming record. The right choice when the source
	// is known to be the more current copy — the usual case when a second device
	// account accumulated the real history.
	PreferSource Policy = "prefer-source"
)

// CollectionChange is what happened to one named collection.
type CollectionChange struct {
	// Name is the JSON key, e.g. "transactions".
	Name string `json:"name"`
	// Added counts records present only in the source.
	Added int `json:"added"`
	// Conflicts counts ids present on both sides holding DIFFERENT content. The
	// policy decided each one; the number is reported so an operator can see how
	// much the policy actually moved.
	Conflicts int `json:"conflicts"`
	// Identical counts ids present on both sides with the same content — the
	// records the two copies already agree on.
	Identical int `json:"identical"`
	// TargetOnly counts records only the target has. They are always kept.
	TargetOnly int `json:"targetOnly"`
	// Total is the record count after merging.
	Total int `json:"total"`
}

// Report is the whole outcome, and is what a preview shows before anybody
// commits to it.
type Report struct {
	Policy      Policy             `json:"policy"`
	Collections []CollectionChange `json:"collections"`
	TotalAdded  int                `json:"totalAdded"`
	Conflicts   int                `json:"conflicts"`
	// KeptFromTarget lists top-level keys that are NOT record collections and
	// differed between the two snapshots — settings, UI state, preferences. The
	// target's values are kept, and the keys are named so nobody is surprised
	// that a merge did not carry their theme across.
	//
	// This is a deliberate refusal rather than an omission: settings are not
	// records with identities, so "merging" them means picking one arbitrarily,
	// and picking arbitrarily is what this package exists to avoid.
	KeptFromTarget []string `json:"keptFromTarget,omitempty"`
	// UnmergeableCollections names collections whose records lack usable ids, so
	// they could not be matched and the target's copy was kept whole.
	UnmergeableCollections []string `json:"unmergeableCollections,omitempty"`
}

// Merge combines source into target and returns the merged snapshot.
//
// The result is built on the TARGET document, so any key this package does not
// understand survives exactly as the target had it. That is the safe direction:
// an unrecognised key is far more likely to be the target's own state than
// something the merge should import.
func Merge(target, source []byte, policy Policy) ([]byte, Report, error) {
	if policy != PreferSource {
		policy = PreferTarget
	}
	report := Report{Policy: policy}

	var targetDoc, sourceDoc map[string]json.RawMessage
	if err := json.Unmarshal(target, &targetDoc); err != nil {
		return nil, report, fmt.Errorf("datasetmerge: read target snapshot: %w", err)
	}
	if err := json.Unmarshal(source, &sourceDoc); err != nil {
		return nil, report, fmt.Errorf("datasetmerge: read source snapshot: %w", err)
	}

	merged := make(map[string]json.RawMessage, len(targetDoc)+len(sourceDoc))
	for k, v := range targetDoc {
		merged[k] = v
	}

	keys := make([]string, 0, len(sourceDoc))
	for k := range sourceDoc {
		keys = append(keys, k)
	}
	// Deterministic order so a preview and its commit report identically, and so
	// two runs over the same inputs produce byte-identical output.
	sort.Strings(keys)

	for _, key := range keys {
		sourceRaw := sourceDoc[key]
		targetRaw, inTarget := targetDoc[key]
		if !inTarget {
			// A collection only the source has: take it wholesale, and count it
			// so the report explains where the records came from.
			merged[key] = sourceRaw
			if recs, ok := decodeRecords(sourceRaw); ok {
				report.Collections = append(report.Collections, CollectionChange{
					Name: key, Added: len(recs), Total: len(recs),
				})
				report.TotalAdded += len(recs)
			}
			continue
		}
		targetRecs, targetOK := decodeRecords(targetRaw)
		sourceRecs, sourceOK := decodeRecords(sourceRaw)
		if !targetOK || !sourceOK {
			// Not a record collection (settings, appState, schemaVersion, a
			// scalar). Keep the target's, and say so when the two differ.
			if !jsonEqual(targetRaw, sourceRaw) {
				report.KeptFromTarget = append(report.KeptFromTarget, key)
			}
			continue
		}
		out, change, ok := mergeCollection(key, targetRecs, sourceRecs, policy)
		if !ok {
			report.UnmergeableCollections = append(report.UnmergeableCollections, key)
			continue
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			return nil, report, fmt.Errorf("datasetmerge: encode %s: %w", key, err)
		}
		merged[key] = encoded
		report.Collections = append(report.Collections, change)
		report.TotalAdded += change.Added
		report.Conflicts += change.Conflicts
	}

	sort.Slice(report.Collections, func(i, j int) bool { return report.Collections[i].Name < report.Collections[j].Name })
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, report, fmt.Errorf("datasetmerge: encode merged snapshot: %w", err)
	}
	return out, report, nil
}

// record is one entity as it appears in a snapshot: its raw JSON plus the id it
// is matched on.
type record struct {
	id  string
	raw json.RawMessage
}

// decodeRecords reads a JSON array of objects. ok is false for anything that is
// not an array — the caller then treats the key as non-mergeable rather than
// mangling it.
func decodeRecords(raw json.RawMessage) ([]record, bool) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	out := make([]record, 0, len(items))
	for _, item := range items {
		var probe struct {
			ID string `json:"id"`
		}
		// An element that is not an object (or has no id) leaves probe.ID empty;
		// mergeCollection decides what that means for the collection as a whole.
		_ = json.Unmarshal(item, &probe)
		out = append(out, record{id: probe.ID, raw: item})
	}
	return out, true
}

// mergeCollection unions two record lists by id.
//
// ok is false when the records cannot be matched — some have no id — because a
// positional or content-based match would be a guess, and a wrong guess here
// either duplicates somebody's transactions or drops them.
func mergeCollection(name string, target, source []record, policy Policy) ([]json.RawMessage, CollectionChange, bool) {
	change := CollectionChange{Name: name}
	if !allIdentified(target) || !allIdentified(source) {
		// An empty collection on one side is not an obstacle: nothing to match.
		if len(target) > 0 && len(source) > 0 {
			return nil, change, false
		}
	}

	index := make(map[string]int, len(target))
	out := make([]json.RawMessage, 0, len(target)+len(source))
	for _, r := range target {
		if r.id != "" {
			index[r.id] = len(out)
		}
		out = append(out, r.raw)
	}
	change.TargetOnly = len(target)

	for _, r := range source {
		if r.id == "" {
			// Only reachable when the target side was empty, so there is nothing
			// to collide with and appending is safe.
			out = append(out, r.raw)
			change.Added++
			continue
		}
		at, exists := index[r.id]
		if !exists {
			index[r.id] = len(out)
			out = append(out, r.raw)
			change.Added++
			// Not a decrement: this id was never in the target, so it cannot
			// come out of the target-only tally.
			continue
		}
		// The id WAS in the target and is also in the source, so it stops being
		// target-only whichever way the policy decides.
		change.TargetOnly--
		if jsonEqual(out[at], r.raw) {
			change.Identical++
			continue
		}
		change.Conflicts++
		if policy == PreferSource {
			out[at] = r.raw
		}
	}
	if change.TargetOnly < 0 {
		change.TargetOnly = 0
	}
	change.Total = len(out)
	return out, change, true
}

// allIdentified reports whether every record carries an id.
func allIdentified(recs []record) bool {
	for _, r := range recs {
		if r.id == "" {
			return false
		}
	}
	return true
}

// jsonEqual compares two raw JSON values structurally, so key order and
// whitespace do not make two identical records look like a conflict.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, err := json.Marshal(av)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(bv)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

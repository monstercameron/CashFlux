package conflictcheckpoint

import (
	"errors"
	"fmt"
	"sort"
)

// Side names one snapshot in a comparison.
type Side string

const (
	SideLocal  Side = "local"
	SideServer Side = "server"
)

// Record is a decrypted dataset record. Values contains the fields that
// define equality for this comparison; Label is a safe, human-readable hint
// such as a transaction merchant, date, and amount.
type Record struct {
	Entity string
	ID     string
	Label  string
	Values map[string]string
}

// Snapshot is a dataset view. Available must be false when the source could
// not be decrypted; its records are then ignored rather than guessed at.
type Snapshot struct {
	Available bool
	Records   []Record
}

// ChangeKind describes how a record differs between snapshots.
type ChangeKind string

const (
	ChangeAdded   ChangeKind = "added"
	ChangeChanged ChangeKind = "changed"
	ChangeDeleted ChangeKind = "deleted"
)

// RecordChange identifies one added, changed, or deleted record by entity and
// ID. Labels are copied from the relevant side for display in a review screen.
type RecordChange struct {
	Kind        ChangeKind
	Entity      string
	ID          string
	BeforeLabel string
	AfterLabel  string
}

// Comparison summarizes entity counts and record changes between snapshots.
// When Available is false, UnavailableSides identifies which side could not
// be read; changes are nil and no conclusions are drawn about that side.
type Comparison struct {
	Available        bool
	UnavailableSides []Side
	Counts           map[Side]map[string]int
	Changes          []RecordChange
}

// Compare returns a deterministic count and record diff for local and server
// snapshots. Duplicate (entity, ID) pairs and incomplete record identities
// are rejected because they make a reliable diff impossible.
func Compare(local, server Snapshot) (Comparison, error) {
	result := Comparison{Counts: map[Side]map[string]int{
		SideLocal:  {},
		SideServer: {},
	}}
	var left, right map[string]Record
	if local.Available {
		var counts map[string]int
		var err error
		left, counts, err = indexSnapshot(local)
		if err != nil {
			return Comparison{}, fmt.Errorf("local snapshot: %w", err)
		}
		result.Counts[SideLocal] = counts
	}
	if server.Available {
		var counts map[string]int
		var err error
		right, counts, err = indexSnapshot(server)
		if err != nil {
			return Comparison{}, fmt.Errorf("server snapshot: %w", err)
		}
		result.Counts[SideServer] = counts
	}
	if !local.Available {
		result.UnavailableSides = append(result.UnavailableSides, SideLocal)
	}
	if !server.Available {
		result.UnavailableSides = append(result.UnavailableSides, SideServer)
	}
	if len(result.UnavailableSides) > 0 {
		return result, nil
	}

	result.Available = true
	for key, before := range left {
		after, exists := right[key]
		if !exists {
			result.Changes = append(result.Changes, RecordChange{Kind: ChangeDeleted, Entity: before.Entity, ID: before.ID, BeforeLabel: before.Label})
			continue
		}
		if !sameValues(before.Values, after.Values) {
			result.Changes = append(result.Changes, RecordChange{Kind: ChangeChanged, Entity: before.Entity, ID: before.ID, BeforeLabel: before.Label, AfterLabel: after.Label})
		}
	}
	for key, after := range right {
		if _, exists := left[key]; !exists {
			result.Changes = append(result.Changes, RecordChange{Kind: ChangeAdded, Entity: after.Entity, ID: after.ID, AfterLabel: after.Label})
		}
	}
	sort.Slice(result.Changes, func(i, j int) bool {
		a, b := result.Changes[i], result.Changes[j]
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Kind < b.Kind
	})
	return result, nil
}

func indexSnapshot(snapshot Snapshot) (map[string]Record, map[string]int, error) {
	indexed := make(map[string]Record, len(snapshot.Records))
	counts := make(map[string]int)
	for _, record := range snapshot.Records {
		if record.Entity == "" || record.ID == "" {
			return nil, nil, errors.New("each record needs an entity type and ID")
		}
		key := record.Entity + "\x00" + record.ID
		if _, exists := indexed[key]; exists {
			return nil, nil, fmt.Errorf("duplicate record %s/%s", record.Entity, record.ID)
		}
		indexed[key] = record
		counts[record.Entity]++
	}
	return indexed, counts, nil
}

func sameValues(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, exists := b[key]; !exists || other != value {
			return false
		}
	}
	return true
}

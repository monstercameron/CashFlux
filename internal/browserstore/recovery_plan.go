package browserstore

import "sort"

// RecoveryLimit is the normal number of distinct copies retained in each scope.
const RecoveryLimit = 3

// RecoveryDescription identifies a recovery payload without keeping its bytes.
// Group includes its class, workspace and owner. Versions are exact record
// fingerprints; an unknown stale payload has nil Versions and is protected.
type RecoveryDescription struct {
	Group    string   `json:"group"`
	Hash     string   `json:"hash"`
	Versions []string `json:"versions,omitempty"`
	Protect  bool     `json:"protect,omitempty"`
}

// RecoveryEntry is a small authenticated index entry. Order is a logical append
// sequence, never a wall clock used to choose between conflicting edits.
type RecoveryEntry struct {
	RecoveryDescription
	Key         string `json:"key"`
	Order       uint64 `json:"order"`
	Bytes       int    `json:"bytes"`
	ReasonBytes int    `json:"reasonBytes,omitempty"`
	EncodedHash string `json:"encodedHash"`
	Empty       bool   `json:"empty,omitempty"`
}

// PlanRecoveryRetention keeps the newest distinct copies in each scope. A stale
// copy outside the cap is removed only when every record version it holds is
// represented by the live dataset or a newer retained copy in the SAME scope.
// Unknown payloads fail closed. The cap may be exceeded to preserve unique edits.
func PlanRecoveryRetention(entries []RecoveryEntry, live map[string][]string) (keep, remove []RecoveryEntry) {
	ordered := append([]RecoveryEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Order == ordered[j].Order {
			return ordered[i].Key > ordered[j].Key
		}
		return ordered[i].Order > ordered[j].Order
	})
	seen := map[string]bool{}
	counts := map[string]int{}
	represented := map[string]map[string]bool{}
	for group, versions := range live {
		represented[group] = map[string]bool{}
		for _, v := range versions {
			represented[group][v] = true
		}
	}
	for _, e := range ordered {
		if e.Empty {
			remove = append(remove, e)
			continue
		}
		fingerprint := e.Group + "\x00" + e.Hash
		duplicate := e.Hash != "" && seen[fingerprint]
		covered := !e.Protect
		if e.Protect && len(e.Versions) > 0 {
			covered = true
			for _, v := range e.Versions {
				if !represented[e.Group][v] {
					covered = false
					break
				}
			}
		}
		if duplicate || (counts[e.Group] >= RecoveryLimit && covered) {
			remove = append(remove, e)
			continue
		}
		keep = append(keep, e)
		seen[fingerprint] = true
		counts[e.Group]++
		if represented[e.Group] == nil {
			represented[e.Group] = map[string]bool{}
		}
		for _, v := range e.Versions {
			represented[e.Group][v] = true
		}
	}
	return
}

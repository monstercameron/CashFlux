package browserstore

import "testing"

func TestBOOTRecoveryRetentionPlan(t *testing.T) {
	tests := []struct {
		name                 string
		entries              []RecoveryEntry
		live                 map[string][]string
		wantKeep, wantRemove int
	}{
		{name: "cap distinct quarantine", entries: []RecoveryEntry{
			{Key: "1", Order: 1, RecoveryDescription: RecoveryDescription{Group: "q", Hash: "1"}},
			{Key: "2", Order: 2, RecoveryDescription: RecoveryDescription{Group: "q", Hash: "2"}},
			{Key: "3", Order: 3, RecoveryDescription: RecoveryDescription{Group: "q", Hash: "3"}},
			{Key: "4", Order: 4, RecoveryDescription: RecoveryDescription{Group: "q", Hash: "4"}},
		}, wantKeep: 3, wantRemove: 1},
		{name: "duplicates retain newest", entries: []RecoveryEntry{
			{Key: "1", Order: 1, RecoveryDescription: RecoveryDescription{Group: "s", Hash: "same", Protect: true}},
			{Key: "2", Order: 2, RecoveryDescription: RecoveryDescription{Group: "s", Hash: "same", Protect: true}},
		}, wantKeep: 1, wantRemove: 1},
		{name: "do not mix scopes", entries: []RecoveryEntry{
			{Key: "1", Order: 1, RecoveryDescription: RecoveryDescription{Group: "owner-a", Hash: "same", Protect: true}},
			{Key: "2", Order: 2, RecoveryDescription: RecoveryDescription{Group: "owner-b", Hash: "same", Protect: true}},
		}, wantKeep: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep, remove := PlanRecoveryRetention(tt.entries, tt.live)
			if len(keep) != tt.wantKeep || len(remove) != tt.wantRemove {
				t.Fatalf("keep %v remove %v", keep, remove)
			}
			if len(keep) == 1 && keep[0].Key != "2" {
				t.Fatal("old duplicate retained")
			}
		})
	}
}

func TestBOOTUniqueStaleEditSurvivesCap(t *testing.T) {
	var entries []RecoveryEntry
	for i := 1; i <= 4; i++ {
		hash := string(rune('0' + i))
		entries = append(entries, RecoveryEntry{Key: hash, Order: uint64(i), RecoveryDescription: RecoveryDescription{Group: "scope", Hash: hash, Protect: true, Versions: []string{hash}}})
	}
	keep, remove := PlanRecoveryRetention(entries, nil)
	if len(keep) != 4 || len(remove) != 0 {
		t.Fatal("unique unrepresented edit was pruned")
	}
	keep, remove = PlanRecoveryRetention(entries, map[string][]string{"scope": {"1"}})
	if len(keep) != 3 || len(remove) != 1 || remove[0].Key != "1" {
		t.Fatal("represented oldest copy was not pruned")
	}
	keep, remove = PlanRecoveryRetention(entries, map[string][]string{"other-owner": {"1"}})
	if len(keep) != 4 || len(remove) != 0 {
		t.Fatal("other scope authorized pruning")
	}
}

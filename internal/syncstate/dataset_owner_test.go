// SPDX-License-Identifier: MIT

package syncstate

import "testing"

func TestDatasetOwnerForeignIsScopedToServerAndUser(t *testing.T) {
	tests := []struct {
		name    string
		owner   DatasetOwner
		current DatasetOwner
		want    bool
	}{
		{"same server and user", DatasetOwner{"server-a", "user-1"}, DatasetOwner{"server-a", "user-1"}, false},
		{"same user on another server", DatasetOwner{"server-a", "user-1"}, DatasetOwner{"server-b", "user-1"}, true},
		{"different user", DatasetOwner{"server-a", "user-1"}, DatasetOwner{"server-a", "user-2"}, true},
		{"unowned dataset", DatasetOwner{}, DatasetOwner{"server-a", "user-1"}, false},
		{"unknown current user", DatasetOwner{"server-a", "user-1"}, DatasetOwner{"server-b", ""}, false},
		{"incomplete owner fails closed", DatasetOwner{"", "user-1"}, DatasetOwner{"server-a", "user-1"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DatasetOwnerForeign(tc.owner, tc.current); got != tc.want {
				t.Fatalf("DatasetOwnerForeign(%+v, %+v) = %v, want %v", tc.owner, tc.current, got, tc.want)
			}
		})
	}
}

func TestParseDatasetOwnerMigratesLegacyUserToCurrentServer(t *testing.T) {
	owner, legacy := ParseDatasetOwner("user-1", "server-now")
	if !legacy || owner != (DatasetOwner{ServerID: "server-now", UserID: "user-1"}) {
		t.Fatalf("legacy parse = %+v, %v", owner, legacy)
	}

	owner, legacy = ParseDatasetOwner(`{"serverId":"server-a","userId":"user-1"}`, "server-b")
	if legacy || owner != (DatasetOwner{ServerID: "server-a", UserID: "user-1"}) {
		t.Fatalf("versioned parse = %+v, %v", owner, legacy)
	}
}

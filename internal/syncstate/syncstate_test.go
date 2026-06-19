package syncstate

import (
	"testing"
	"time"
)

func TestShouldApplyRemote(t *testing.T) {
	base := time.Date(2026, time.June, 18, 20, 30, 0, 0, time.UTC)
	tests := []struct {
		name          string
		local         time.Time
		hasLocalMeta  bool
		hasLocalData  bool
		remote        time.Time
		hasRemoteData bool
		want          bool
	}{
		{"newer remote", base, true, true, base.Add(time.Minute), true, true},
		{"older remote", base, true, true, base.Add(-time.Minute), true, false},
		{"equal remote", base, true, true, base, true, false},
		{"missing local metadata with local data", base, false, true, base.Add(time.Minute), true, false},
		{"fresh browser accepts remote", base, false, false, base.Add(time.Minute), true, true},
		{"missing remote data", base, true, true, base.Add(time.Minute), false, false},
		{"zero remote time", base, true, true, time.Time{}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldApplyRemote(tt.local, tt.hasLocalMeta, tt.hasLocalData, tt.remote, tt.hasRemoteData); got != tt.want {
				t.Fatalf("ShouldApplyRemote = %v, want %v", got, tt.want)
			}
		})
	}
}

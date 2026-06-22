package artifactstore_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/artifactstore"
)

func TestOverQuota(t *testing.T) {
	tests := []struct {
		name      string
		used      int64
		quota     int64
		wantOver  bool
	}{
		{name: "well under quota", used: 10 << 20, quota: 100 << 20, wantOver: false},
		{name: "exactly at threshold", used: int64(float64(100<<20) * artifactstore.WarnThreshold), quota: 100 << 20, wantOver: true},
		{name: "over threshold", used: 95 << 20, quota: 100 << 20, wantOver: true},
		{name: "zero quota (unknown)", used: 50 << 20, quota: 0, wantOver: false},
		{name: "zero used", used: 0, quota: 50 << 20, wantOver: false},
		{name: "used equals quota", used: 50 << 20, quota: 50 << 20, wantOver: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := artifactstore.OverQuota(tc.used, tc.quota)
			if got != tc.wantOver {
				t.Errorf("OverQuota(%d, %d) = %v, want %v", tc.used, tc.quota, got, tc.wantOver)
			}
		})
	}
}

func TestNearLimit(t *testing.T) {
	tests := []struct {
		name string
		used int64
		want bool
	}{
		{name: "zero", used: 0, want: false},
		{name: "well under 50MB limit", used: 10 << 20, want: false},
		{name: "at 90pct of 50MB", used: int64(float64(artifactstore.RecommendedQuota) * artifactstore.WarnThreshold), want: true},
		{name: "over 50MB", used: 60 << 20, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := artifactstore.NearLimit(tc.used)
			if got != tc.want {
				t.Errorf("NearLimit(%d) = %v, want %v", tc.used, got, tc.want)
			}
		})
	}
}

func TestRecommendedQuotaIs50MB(t *testing.T) {
	const want = 50 << 20
	if artifactstore.RecommendedQuota != want {
		t.Errorf("RecommendedQuota = %d, want %d (50 MiB)", artifactstore.RecommendedQuota, want)
	}
}

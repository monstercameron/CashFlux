// SPDX-License-Identifier: MIT

package buildinfo

import "testing"

func TestFormatBuildID(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		unix int64
		want string
	}{
		{"normal", "abc1234", 1700000000000000000, "abc1234-1700000000000000000"},
		{"empty sha", "", 1700000000000000000, "nogit-1700000000000000000"},
		{"blank sha", "  ", 1, "nogit-1"},
		{"dev never stamped", "dev", 1, "nogit-1"},
		{"unsafe chars sanitized", "ab/c d", 2, "ab-c-d-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBuildID(tt.sha, tt.unix); got != tt.want {
				t.Errorf("FormatBuildID(%q, %d) = %q, want %q", tt.sha, tt.unix, got, tt.want)
			}
		})
	}
}

func TestFormatBuildIDNanosecondsDistinguishBuildsOfSameCommit(t *testing.T) {
	first := FormatBuildID("abc1234", 1700000000000000000)
	second := FormatBuildID("abc1234", 1700000000000000001)
	if first == second {
		t.Fatalf("build IDs for distinct release timestamps collided: %q", first)
	}
}

func TestCacheName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abc1234-1700000000", "cashflux-abc1234-1700000000"},
		{"dev", "cashflux-dev"},
		{"", "cashflux-dev"},
		{"a/b c?d", "cashflux-a-b-c-d"},
		{"v1.2_beta", "cashflux-v1.2_beta"},
	}
	for _, tt := range tests {
		if got := CacheName(tt.in); got != tt.want {
			t.Errorf("CacheName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseBuildJSON(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Info
		wantErr bool
	}{
		{"stamped", `{"buildId":"abc-1","schema":3}`, Info{BuildID: "abc-1", Schema: 3}, false},
		{"unknown fields ignored", `{"buildId":"abc-1","schema":3,"future":true}`, Info{BuildID: "abc-1", Schema: 3}, false},
		{"schema optional", `{"buildId":"abc-1"}`, Info{BuildID: "abc-1"}, false},
		{"missing id", `{"schema":3}`, Info{}, true},
		{"blank id", `{"buildId":"  "}`, Info{}, true},
		{"negative schema", `{"buildId":"a","schema":-1}`, Info{}, true},
		{"not json", `<!doctype html>`, Info{}, true},
		{"empty", ``, Info{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBuildJSON([]byte(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseBuildJSON(%s) = %+v, want error", tt.raw, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("ParseBuildJSON(%s) = %+v, %v; want %+v", tt.raw, got, err, tt.want)
			}
		})
	}
}

func TestMarshalParseRoundTrip(t *testing.T) {
	want := Info{BuildID: "deadbee-1700000000", Schema: 1}
	raw, err := MarshalBuildJSON(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseBuildJSON(raw)
	if err != nil || got != want {
		t.Errorf("round trip = %+v, %v; want %+v (raw %s)", got, err, want, raw)
	}
	if _, err := MarshalBuildJSON(Info{}); err == nil {
		t.Error("MarshalBuildJSON with empty buildId should fail")
	}
}

func TestRunningAndIsDev(t *testing.T) {
	old := BuildID
	t.Cleanup(func() { BuildID = old })

	BuildID = ""
	if Running() != DevBuild || !IsDev() {
		t.Errorf("empty stamp: Running()=%q IsDev()=%v, want dev/true", Running(), IsDev())
	}
	BuildID = "abc-1"
	if Running() != "abc-1" || IsDev() {
		t.Errorf("stamped: Running()=%q IsDev()=%v, want abc-1/false", Running(), IsDev())
	}
}

func TestIsStale(t *testing.T) {
	oldID := BuildID
	mu.Lock()
	oldActive, oldKnown := active, activeKnown
	active, activeKnown = Info{}, false
	mu.Unlock()
	t.Cleanup(func() {
		BuildID = oldID
		mu.Lock()
		active, activeKnown = oldActive, oldKnown
		mu.Unlock()
	})

	BuildID = "build-a"
	if IsStale() {
		t.Error("unknown active build (offline) must not be stale")
	}
	SetActive(Info{BuildID: "build-a", Schema: 1})
	if IsStale() {
		t.Error("running the active build must not be stale")
	}
	SetActive(Info{BuildID: "build-b", Schema: 1})
	if !IsStale() {
		t.Error("running a replaced build must be stale")
	}
	SetActive(Info{BuildID: ""}) // ignored: a garbled fetch changes nothing
	if got, _ := Active(); got.BuildID != "build-b" {
		t.Errorf("empty SetActive overwrote active build: %+v", got)
	}
	BuildID = DevBuild
	if IsStale() {
		t.Error("dev builds are never stale")
	}
	BuildID = "build-a"
	SetActive(Info{BuildID: DevBuild})
	if IsStale() {
		t.Error("a dev build.json (unstamped server) must not mark tabs stale")
	}
}

func TestDecideBanner(t *testing.T) {
	activeB := Info{BuildID: "build-b", Schema: 1}
	tests := []struct {
		name string
		o    Observations
		want Banner
	}{
		{
			"dev build ignores everything",
			Observations{Running: DevBuild, Active: activeB, ActiveKnown: true, UpdateReady: true},
			BannerNone,
		},
		{
			"unstamped runnning id is dev",
			Observations{Running: "", Active: activeB, ActiveKnown: true, UpdateReady: true},
			BannerNone,
		},
		{
			"current build, no update",
			Observations{Running: "build-b", Active: activeB, ActiveKnown: true},
			BannerNone,
		},
		{
			"waiting worker on current build offers reload",
			Observations{Running: "build-b", Active: activeB, ActiveKnown: true, UpdateReady: true},
			BannerUpdateReady,
		},
		{
			"replaced build is stale even without a waiting worker",
			Observations{Running: "build-a", Active: activeB, ActiveKnown: true},
			BannerStale,
		},
		{
			"stale banner cannot be dismissed",
			Observations{Running: "build-a", Active: activeB, ActiveKnown: true, UpdateReady: true, DismissedBuild: "build-b"},
			BannerStale,
		},
		{
			"newer schema on the server is stale",
			Observations{Running: "build-b", RunningSchema: 1, Active: Info{BuildID: "build-b", Schema: 2}, ActiveKnown: true},
			BannerStale,
		},
		{
			"same schema is not stale",
			Observations{Running: "build-b", RunningSchema: 1, Active: activeB, ActiveKnown: true},
			BannerNone,
		},
		{
			"offline tab with no news shows nothing",
			Observations{Running: "build-a"},
			BannerNone,
		},
		{
			"announced update without build.json offers reload",
			Observations{Running: "build-a", UpdateReady: true},
			BannerUpdateReady,
		},
		{
			"announced update dismissed anonymously",
			Observations{Running: "build-a", UpdateReady: true, DismissedBuild: "*"},
			BannerNone,
		},
		{
			"named update dismissed by build",
			Observations{Running: "build-b", Active: activeB, ActiveKnown: true, UpdateReady: true, DismissedBuild: "build-b"},
			BannerNone,
		},
		{
			"a still newer build re-offers after dismissal",
			Observations{Running: "build-b", Active: Info{BuildID: "build-c", Schema: 1}, ActiveKnown: true, UpdateReady: true, DismissedBuild: "build-b"},
			BannerStale, // running build-b, active is build-c: stale wins over gentle
		},
		{
			"dev build.json never makes a stamped tab stale",
			Observations{Running: "build-a", Active: Info{BuildID: DevBuild}, ActiveKnown: true},
			BannerNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecideBanner(tt.o); got != tt.want {
				t.Errorf("DecideBanner(%+v) = %v, want %v", tt.o, got, tt.want)
			}
		})
	}
}

func TestDismissalKeyMatchesDecision(t *testing.T) {
	// Whatever DismissalKey records, feeding it back must silence the
	// gentle banner for the same observations.
	cases := []Observations{
		{Running: "build-b", Active: Info{BuildID: "build-b", Schema: 1}, ActiveKnown: true, UpdateReady: true},
		{Running: "build-a", UpdateReady: true},
	}
	for _, o := range cases {
		o.DismissedBuild = DismissalKey(o)
		if got := DecideBanner(o); got != BannerNone {
			t.Errorf("after dismissing %+v, DecideBanner = %v, want BannerNone", o, got)
		}
	}
}

func TestLDFlags(t *testing.T) {
	want := "-X github.com/monstercameron/CashFlux/internal/buildinfo.BuildID=abc-1"
	if got := LDFlags("abc-1"); got != want {
		t.Errorf("LDFlags = %q, want %q", got, want)
	}
}

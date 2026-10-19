// SPDX-License-Identifier: MIT

// Package buildinfo identifies the exact build of CashFlux that is running
// and the build the server currently serves, so one tab can never quietly
// keep writing data with code another tab has already replaced (SV7a).
//
// The running build is stamped at release time: tools/stampbuild writes
// web/build.json and prints the matching -ldflags -X flag that sets
// [BuildID] in the wasm binary. A build that was never stamped — `gwc dev`,
// a plain `go build`, the e2e harness — reports [DevBuild], and every
// update/stale decision in this package turns off for it, so live reload is
// never blocked by an "update ready" banner.
//
// Build IDs are opaque. They are only ever compared for equality against
// the server's build.json, never ordered: no wall-clock or sequence
// comparison decides which tab wins (SV principle 6). The tab whose ID
// differs from the active one is the stale one, whichever opened first.
package buildinfo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// DevBuild is the build ID of an unstamped (development) build. Every
// staleness and banner decision short-circuits to "do nothing" for it.
const DevBuild = "dev"

// BuildID is the stamped identity of this binary. Release builds set it
// with:
//
//	-ldflags "-X github.com/monstercameron/CashFlux/internal/buildinfo.BuildID=<id>"
//
// using the same ID tools/stampbuild wrote into web/build.json.
var BuildID = DevBuild

// Info is one build.json document: the identity of a build and the dataset
// schema version that build writes.
type Info struct {
	BuildID string `json:"buildId"`
	Schema  int    `json:"schema"`
}

// Running returns this binary's build ID, normalising an empty stamp (a
// -X flag with an empty value) to [DevBuild].
func Running() string {
	if strings.TrimSpace(BuildID) == "" {
		return DevBuild
	}
	return BuildID
}

// IsDev reports whether this is an unstamped development build.
func IsDev() bool { return Running() == DevBuild }

// FormatBuildID renders the canonical "<git short sha and nonce>-<unix
// nanoseconds>" build ID the stamp step emits. Nanosecond precision helps
// distinguish repeated release builds; stampbuild also supplies a random
// nonce. An unknown sha becomes "nogit" so the ID is never empty and never
// accidentally equals [DevBuild].
func FormatBuildID(gitShortSHA string, unixNanoseconds int64) string {
	sha := Sanitize(strings.TrimSpace(gitShortSHA))
	if sha == "" || sha == DevBuild {
		sha = "nogit"
	}
	return sha + "-" + strconv.FormatInt(unixNanoseconds, 10)
}

// Sanitize reduces an arbitrary string to the characters allowed in a build
// ID and cache name: letters, digits, '.', '_' and '-'. It mirrors the
// sanitizer in web/sw.js so both sides derive the same cache name.
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, s)
}

// CachePrefix prefixes every service-worker cache CashFlux owns.
const CachePrefix = "cashflux-"

// CacheName returns the service-worker cache name for a build. web/sw.js
// derives the identical name; TestCacheName pins the shared shape.
func CacheName(buildID string) string {
	id := Sanitize(strings.TrimSpace(buildID))
	if id == "" {
		id = DevBuild
	}
	return CachePrefix + id
}

// ParseBuildJSON decodes a build.json document. Unknown fields are ignored
// so a newer stamp step can add fields without breaking older tabs.
func ParseBuildJSON(raw []byte) (Info, error) {
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return Info{}, fmt.Errorf("buildinfo: parse build.json: %w", err)
	}
	info.BuildID = strings.TrimSpace(info.BuildID)
	if info.BuildID == "" {
		return Info{}, fmt.Errorf("buildinfo: build.json has no buildId")
	}
	if info.Schema < 0 {
		return Info{}, fmt.Errorf("buildinfo: build.json schema %d is negative", info.Schema)
	}
	return info, nil
}

// MarshalBuildJSON renders the build.json document the stamp step writes.
func MarshalBuildJSON(info Info) ([]byte, error) {
	if strings.TrimSpace(info.BuildID) == "" {
		return nil, fmt.Errorf("buildinfo: cannot stamp an empty buildId")
	}
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("buildinfo: render build.json: %w", err)
	}
	return append(raw, '\n'), nil
}

// LDFlags returns the -ldflags fragment that stamps id into a wasm build,
// ready to splice into a `go build` command line.
func LDFlags(id string) string {
	return "-X github.com/monstercameron/CashFlux/internal/buildinfo.BuildID=" + Sanitize(id)
}

var (
	mu          sync.RWMutex
	active      Info
	activeKnown bool
)

// SetActive records the build the server currently serves (fetched from
// build.json with cache disabled). Invalid or empty IDs are ignored so a
// garbled response can never mark a healthy tab stale.
func SetActive(info Info) {
	if strings.TrimSpace(info.BuildID) == "" {
		return
	}
	mu.Lock()
	active, activeKnown = info, true
	mu.Unlock()
}

// Active returns the last known server build, if one was ever fetched.
func Active() (Info, bool) {
	mu.RLock()
	defer mu.RUnlock()
	return active, activeKnown
}

// IsStale reports whether the running build differs from the build the
// server now serves. Development builds and tabs that have not yet learned
// the active build (offline) are never stale: being offline on the only
// copy you have is not a mixed build. Other lanes consult this to refuse
// writes from a tab another tab has already replaced.
func IsStale() bool {
	info, known := Active()
	return stale(Running(), info, known)
}

// stale is the pure core of [IsStale].
func stale(running string, active Info, activeKnown bool) bool {
	if running == DevBuild || !activeKnown {
		return false
	}
	return active.BuildID != DevBuild && active.BuildID != running
}

// Banner is what the update banner should do for one set of observations.
type Banner int

const (
	// BannerNone: show nothing.
	BannerNone Banner = iota
	// BannerUpdateReady: a newer build is installed and waiting; offer a
	// reload. The user may dismiss it for this build.
	BannerUpdateReady
	// BannerStale: this tab is not the active build (or writes an older
	// schema than the active build). Offer a reload and keep offering:
	// dismissing never silences it, because this tab must not quietly keep
	// writing old-format data.
	BannerStale
)

// Observations are the inputs to [DecideBanner]: everything the wasm side
// can learn about its own build versus the server's.
type Observations struct {
	// Running is this tab's build ID.
	Running string
	// RunningSchema is the dataset schema this build writes.
	RunningSchema int
	// Active is the server's build.json, when ActiveKnown.
	Active      Info
	ActiveKnown bool
	// UpdateReady reports a waiting service worker (a newer build is
	// installed) or an equivalent notice from another tab.
	UpdateReady bool
	// DismissedBuild is the active build ID the user already dismissed the
	// gentle banner for, if any.
	DismissedBuild string
}

// DecideBanner turns observations into a banner decision. Development
// builds never show a banner. A tab that is merely told an update exists
// gets the gentle, dismissible banner; a tab known to differ from the
// active build — or to write an older schema than it — gets the
// persistent one.
func DecideBanner(o Observations) Banner {
	running := o.Running
	if strings.TrimSpace(running) == "" {
		running = DevBuild
	}
	if running == DevBuild {
		return BannerNone
	}
	if stale(running, o.Active, o.ActiveKnown) {
		return BannerStale
	}
	if o.ActiveKnown && o.RunningSchema > 0 && o.Active.Schema > o.RunningSchema {
		// Same stamped ID should mean same schema, but a schema ahead of
		// what this code writes is stale by definition: never let it be
		// dismissed away.
		return BannerStale
	}
	if !o.UpdateReady {
		return BannerNone
	}
	if o.DismissedBuild == DismissalKey(o) {
		return BannerNone
	}
	return BannerUpdateReady
}

// DismissalKey is the value a "later" click should record so [DecideBanner]
// stops offering the same update: the active build being offered, or "*"
// when the update was announced (a waiting worker, another tab) before
// build.json could be fetched to name it.
func DismissalKey(o Observations) string {
	if o.ActiveKnown && o.Active.BuildID != "" {
		return o.Active.BuildID
	}
	return "*"
}

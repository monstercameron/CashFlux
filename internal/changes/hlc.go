// SPDX-License-Identifier: MIT

package changes

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// HLC is a hybrid logical clock timestamp (Kulkarni et al., 2014).
//
// WallMillis is the largest physical wall-clock time (Unix milliseconds) the
// issuing clock has observed. Logical is a counter that orders events which
// share the same WallMillis. An HLC is ordered first by WallMillis, then by
// Logical. Two HLCs can still tie across devices; merge comparisons therefore
// use a Stamp, which adds the DeviceID as the final tie-breaker. No merge
// decision in this package is ever made by comparing wall-clock times alone.
type HLC struct {
	WallMillis int64  `json:"wallMillis"`
	Logical    uint32 `json:"logical"`
}

// Compare returns -1, 0 or +1 as h orders before, equal to, or after o.
func (h HLC) Compare(o HLC) int {
	switch {
	case h.WallMillis != o.WallMillis:
		if h.WallMillis < o.WallMillis {
			return -1
		}
		return 1
	case h.Logical != o.Logical:
		if h.Logical < o.Logical {
			return -1
		}
		return 1
	default:
		return 0
	}
}

// Before reports whether h is strictly before o.
func (h HLC) Before(o HLC) bool { return h.Compare(o) < 0 }

// After reports whether h is strictly after o.
func (h HLC) After(o HLC) bool { return h.Compare(o) > 0 }

// Equal reports whether h and o are the same HLC.
func (h HLC) Equal(o HLC) bool { return h == o }

// IsZero reports whether h is the zero HLC (no time observed yet).
func (h HLC) IsZero() bool { return h == HLC{} }

// String renders h as "<wallMillis>.<logical>".
func (h HLC) String() string { return fmt.Sprintf("%d.%d", h.WallMillis, h.Logical) }

// Stamp is the total ordering key used for every merge decision: an HLC plus
// the DeviceID that issued it. Stamps order by (WallMillis, Logical, DeviceID),
// so two stamps compare equal only when they come from the same device at the
// same HLC.
type Stamp struct {
	HLC      HLC    `json:"hlc"`
	DeviceID string `json:"deviceId"`
}

// NewStamp builds the Stamp for a batch issued by deviceID at hlc.
func NewStamp(hlc HLC, deviceID string) Stamp { return Stamp{HLC: hlc, DeviceID: deviceID} }

// Compare returns -1, 0 or +1 as s orders before, equal to, or after o.
func (s Stamp) Compare(o Stamp) int {
	if c := s.HLC.Compare(o.HLC); c != 0 {
		return c
	}
	return strings.Compare(s.DeviceID, o.DeviceID)
}

// Before reports whether s is strictly before o.
func (s Stamp) Before(o Stamp) bool { return s.Compare(o) < 0 }

// After reports whether s is strictly after o.
func (s Stamp) After(o Stamp) bool { return s.Compare(o) > 0 }

// Equal reports whether s and o are the same stamp.
func (s Stamp) Equal(o Stamp) bool { return s == o }

// IsZero reports whether s is the zero stamp.
func (s Stamp) IsZero() bool { return s.HLC.IsZero() && s.DeviceID == "" }

// String renders s as "<hlc>@<deviceId>".
func (s Stamp) String() string { return s.HLC.String() + "@" + s.DeviceID }

// PhysicalClock returns the physical wall time in Unix milliseconds. It is
// injectable so tests can drive the clock backwards and forwards.
type PhysicalClock func() int64

// SystemPhysicalClock is the production PhysicalClock, backed by time.Now.
func SystemPhysicalClock() int64 { return time.Now().UnixMilli() }

// DefaultMaxSkew is the default bound on how far ahead of the local physical
// clock a remote HLC may be before its wall component is clamped (see
// Clock.Observe). It guards against a peer with a badly wrong clock dragging
// every device's timestamps far into the future.
const DefaultMaxSkew = 24 * time.Hour

// ClockConfig configures a Clock. The zero value uses the system clock and
// DefaultMaxSkew.
type ClockConfig struct {
	// Physical supplies wall time in Unix milliseconds. Nil means
	// SystemPhysicalClock.
	Physical PhysicalClock
	// MaxSkew bounds accepted remote wall times. Zero or negative means
	// DefaultMaxSkew.
	MaxSkew time.Duration
	// Initial seeds the clock, e.g. the largest HLC persisted with a
	// snapshot, so a restarted device never re-issues an older stamp.
	Initial HLC
}

// Clock is a concurrency-safe hybrid logical clock.
//
// Now is monotonic even if the physical clock moves backwards: the wall
// component never decreases and the logical component absorbs the difference.
type Clock struct {
	mu         sync.Mutex
	physical   PhysicalClock
	maxSkewMs  int64
	last       HLC
	skewEvents uint64
}

// NewClock returns a Clock configured by cfg.
func NewClock(cfg ClockConfig) *Clock {
	physical := cfg.Physical
	if physical == nil {
		physical = SystemPhysicalClock
	}
	skew := cfg.MaxSkew
	if skew <= 0 {
		skew = DefaultMaxSkew
	}
	return &Clock{physical: physical, maxSkewMs: skew.Milliseconds(), last: cfg.Initial}
}

// NewSystemClock returns a Clock using the system wall clock and the default
// maximum skew.
func NewSystemClock() *Clock { return NewClock(ClockConfig{}) }

// Now returns a fresh HLC strictly greater than every HLC this clock has
// previously returned or observed.
func (c *Clock) Now() HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	pt := c.physical()
	switch {
	case pt > c.last.WallMillis:
		c.last = HLC{WallMillis: pt, Logical: 0}
	default:
		// Physical time has not advanced (or went backwards): keep the
		// wall component and count logically so stamps stay monotonic.
		c.last = nextLogical(c.last)
	}
	return c.last
}

// Observe merges a remote HLC into this clock following the HLC receive rule,
// so the next Now is strictly greater than remote. It returns the clock's new
// value.
//
// Bounded drift guard: if remote.WallMillis is more than the configured
// maximum skew ahead of the local physical clock, the wall component is
// clamped to physical+maxSkew before merging and the returned error wraps
// ErrClockSkew. The clamped value is still merged (the error reports the
// clamp, it does not roll it back), so causally-later local writes still
// order after the accepted part of the remote time — but a peer can never
// push this clock beyond the bound. Callers should log the error and may
// surface a "check the time on the other device" hint.
func (c *Clock) Observe(remote HLC) (HLC, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pt := c.physical()
	effective := remote
	var err error
	if limit := pt + c.maxSkewMs; remote.WallMillis > limit {
		effective.WallMillis = limit
		c.skewEvents++
		err = fmt.Errorf("changes: remote wall time %d is more than %d ms ahead of local physical time %d, clamped to %d: %w",
			remote.WallMillis, c.maxSkewMs, pt, limit, ErrClockSkew)
	}
	prev := c.last
	wall := max64(prev.WallMillis, effective.WallMillis, pt)
	switch {
	case wall == prev.WallMillis && wall == effective.WallMillis:
		logical := max32(prev.Logical, effective.Logical)
		c.last = nextAfter(wall, logical)
	case wall == prev.WallMillis:
		c.last = nextLogical(prev)
	case wall == effective.WallMillis:
		c.last = nextAfter(wall, effective.Logical)
	default: // wall == pt, strictly greater than both
		c.last = HLC{WallMillis: wall, Logical: 0}
	}
	return c.last, err
}

// Current returns the clock's present value without advancing it.
func (c *Clock) Current() HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

// SkewEvents returns how many remote observations have been clamped by the
// drift guard since the clock was created.
func (c *Clock) SkewEvents() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.skewEvents
}

// nextLogical advances h by one logical step, spilling into the wall
// component when the logical counter would overflow.
func nextLogical(h HLC) HLC { return nextAfter(h.WallMillis, h.Logical) }

// nextAfter returns the smallest HLC strictly greater than (wall, logical).
func nextAfter(wall int64, logical uint32) HLC {
	if logical == math.MaxUint32 {
		return HLC{WallMillis: wall + 1, Logical: 0}
	}
	return HLC{WallMillis: wall, Logical: logical + 1}
}

func max64(v int64, rest ...int64) int64 {
	for _, o := range rest {
		if o > v {
			v = o
		}
	}
	return v
}

func max32(a, b uint32) uint32 {
	if b > a {
		return b
	}
	return a
}

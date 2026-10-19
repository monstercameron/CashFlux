// SPDX-License-Identifier: MIT

package changes_test

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
)

func TestHLCCompareOrder(t *testing.T) {
	cases := []struct {
		a, b changes.HLC
		want int
	}{
		{changes.HLC{100, 0}, changes.HLC{100, 0}, 0},
		{changes.HLC{100, 0}, changes.HLC{100, 1}, -1},
		{changes.HLC{100, 1}, changes.HLC{100, 0}, 1},
		{changes.HLC{99, 99}, changes.HLC{100, 0}, -1},
		{changes.HLC{101, 0}, changes.HLC{100, 99}, 1},
	}
	for _, tc := range cases {
		if got := tc.a.Compare(tc.b); got != tc.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestStampTieBreakByDevice(t *testing.T) {
	h := changes.HLC{WallMillis: 100, Logical: 1}
	a := changes.NewStamp(h, "device-a")
	b := changes.NewStamp(h, "device-b")
	if !a.Before(b) || !b.After(a) || a.Equal(b) {
		t.Fatalf("stamp tie-break wrong: %v vs %v", a, b)
	}
	if !a.Equal(changes.NewStamp(h, "device-a")) {
		t.Fatal("identical stamps must be equal")
	}
}

func TestClockNowMonotonicWhenPhysicalGoesBackwards(t *testing.T) {
	phys := &fakePhysical{readings: []int64{1000, 1000, 900, 500, 999, 1000, 1000, 1001}}
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next})
	prev := clock.Now()
	if prev != (changes.HLC{WallMillis: 1000, Logical: 0}) {
		t.Fatalf("first Now = %v, want 1000.0", prev)
	}
	for i := 0; i < 7; i++ {
		next := clock.Now()
		if !next.After(prev) {
			t.Fatalf("Now #%d = %v, not strictly after %v (physical clock went backwards)", i+2, next, prev)
		}
		if next.WallMillis < prev.WallMillis {
			t.Fatalf("wall moved backwards: %v after %v", next, prev)
		}
		prev = next
	}
	if prev.WallMillis != 1001 {
		t.Fatalf("final Now = %v, want wall 1001 once physical time caught up", prev)
	}
}

func TestClockNowIncrementsLogicalWithinSameMillisecond(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(42)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next})
	for want := uint32(0); want < 5; want++ {
		got := clock.Now()
		if got != (changes.HLC{WallMillis: 42, Logical: want}) {
			t.Fatalf("Now = %v, want 42.%d", got, want)
		}
	}
}

func TestClockObserveMergesRemote(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(100)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next})
	_ = clock.Now() // 100.0
	got, err := clock.Observe(changes.HLC{WallMillis: 150, Logical: 3})
	if err != nil {
		t.Fatalf("Observe: unexpected error: %v", err)
	}
	if got != (changes.HLC{WallMillis: 150, Logical: 4}) {
		t.Fatalf("Observe(remote 150.3) = %v, want 150.4", got)
	}
	if next := clock.Now(); next != (changes.HLC{WallMillis: 150, Logical: 5}) {
		t.Fatalf("Now after Observe = %v, want 150.5 (physical still 100)", next)
	}
}

func TestClockObserveOlderRemoteStillAdvances(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(100)
	clock := changes.NewClock(changes.ClockConfig{
		Physical: phys.Next,
		Initial:  changes.HLC{WallMillis: 150, Logical: 4},
	})
	got, err := clock.Observe(changes.HLC{WallMillis: 120, Logical: 9})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if got != (changes.HLC{WallMillis: 150, Logical: 5}) {
		t.Fatalf("Observe(older remote) = %v, want 150.5", got)
	}
}

func TestClockSkewClamp(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(1_000)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next, MaxSkew: time.Hour})
	tenDaysMs := int64(10 * 24 * time.Hour / time.Millisecond)
	remote := changes.HLC{WallMillis: 1_000 + tenDaysMs, Logical: 7}
	got, err := clock.Observe(remote)
	if !errors.Is(err, changes.ErrClockSkew) {
		t.Fatalf("Observe(far-future remote) error = %v, want ErrClockSkew", err)
	}
	limit := int64(1_000) + int64(time.Hour/time.Millisecond)
	if got.WallMillis != limit {
		t.Fatalf("clamped wall = %d, want %d (physical + max skew), remote was %d", got.WallMillis, limit, remote.WallMillis)
	}
	if got.Logical != 8 {
		t.Fatalf("clamped logical = %d, want 8 (one past remote logical)", got.Logical)
	}
	if clock.SkewEvents() != 1 {
		t.Fatalf("SkewEvents = %d, want 1", clock.SkewEvents())
	}
	// The clock must keep working monotonically from the clamped value and
	// must never jump to the unclamped remote wall time.
	next := clock.Now()
	if !next.After(got) || next.WallMillis != limit {
		t.Fatalf("Now after clamp = %v, want > %v with wall %d", next, got, limit)
	}
}

func TestClockSkewBoundaryIsNotClamped(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(5_000)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next, MaxSkew: time.Hour})
	limit := int64(5_000) + int64(time.Hour/time.Millisecond)
	if _, err := clock.Observe(changes.HLC{WallMillis: limit, Logical: 0}); err != nil {
		t.Fatalf("Observe(exactly at limit) error = %v, want nil", err)
	}
	if _, err := clock.Observe(changes.HLC{WallMillis: limit + 1, Logical: 0}); !errors.Is(err, changes.ErrClockSkew) {
		t.Fatalf("Observe(1ms past limit) error = %v, want ErrClockSkew", err)
	}
}

func TestClockDefaultSkewIs24Hours(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(0)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next})
	within := changes.HLC{WallMillis: int64(23 * time.Hour / time.Millisecond)}
	if _, err := clock.Observe(within); err != nil {
		t.Fatalf("Observe(23h ahead) error = %v, want nil under default 24h skew", err)
	}
	phys.Set(0) // physical back to 0; 25h ahead must now clamp
	beyond := changes.HLC{WallMillis: int64(25 * time.Hour / time.Millisecond)}
	if _, err := clock.Observe(beyond); !errors.Is(err, changes.ErrClockSkew) {
		t.Fatalf("Observe(25h ahead) error = %v, want ErrClockSkew under default 24h skew", err)
	}
}

func TestClockLogicalOverflowSpillsIntoWall(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(50)
	clock := changes.NewClock(changes.ClockConfig{
		Physical: phys.Next,
		Initial:  changes.HLC{WallMillis: 100, Logical: math.MaxUint32},
	})
	if got := clock.Now(); got != (changes.HLC{WallMillis: 101, Logical: 0}) {
		t.Fatalf("Now after logical overflow = %v, want 101.0", got)
	}
}

func TestClockConcurrentNowIsUnique(t *testing.T) {
	phys := &fakePhysical{}
	phys.Set(777)
	clock := changes.NewClock(changes.ClockConfig{Physical: phys.Next})
	const goroutines, per = 8, 500
	seen := make(map[changes.HLC]struct{}, goroutines*per)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				h := clock.Now()
				mu.Lock()
				seen[h] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != goroutines*per {
		t.Fatalf("unique HLCs = %d, want %d", len(seen), goroutines*per)
	}
}

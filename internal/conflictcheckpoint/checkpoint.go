// Package conflictcheckpoint models durable, scope-isolated copies kept after
// a sync conflict and compares decrypted snapshots without platform services.
package conflictcheckpoint

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Retention is the period for which an unexpired checkpoint is retained.
const Retention = 90 * 24 * time.Hour

// Scope identifies one exact server, account, and workspace boundary.
// All three values are required; partial scopes are not valid checkpoint keys.
type Scope struct {
	ServerID    string
	AccountID   string
	WorkspaceID string
}

// Validate reports whether all scope identifiers are present.
func (s Scope) Validate() error {
	if s.ServerID == "" || s.AccountID == "" || s.WorkspaceID == "" {
		return errors.New("server, account, and workspace scope IDs are required")
	}
	return nil
}

// Same reports whether two checkpoints belong to exactly the same scope.
func (s Scope) Same(other Scope) bool { return s == other }

// RevisionOrder identifies a dataset position without relying on wall time.
// DeviceCounter breaks ties for changes made at the same dataset revision.
type RevisionOrder struct {
	DatasetRevision uint64
	DeviceCounter   uint64
}

// Compare reports the ordering of o and other using dataset revision first,
// then the device's monotonic counter. Wall time is deliberately excluded.
func (o RevisionOrder) Compare(other RevisionOrder) int {
	if o.DatasetRevision < other.DatasetRevision {
		return -1
	}
	if o.DatasetRevision > other.DatasetRevision {
		return 1
	}
	if o.DeviceCounter < other.DeviceCounter {
		return -1
	}
	if o.DeviceCounter > other.DeviceCounter {
		return 1
	}
	return 0
}

// Origin describes why a checkpoint was created.
type Origin string

const (
	OriginRejectedPush    Origin = "rejected_push"
	OriginDisplacedServer Origin = "displaced_server_copy"
	OriginManual          Origin = "manual"
)

// State is the lifecycle state of a checkpoint.
type State string

const (
	StateKept      State = "kept"
	StateRestored  State = "restored"
	StateDiscarded State = "discarded"
	StateExpired   State = "expired"
)

// Checkpoint describes one preserved snapshot. CreatedAt orders snapshots by
// dataset revision and device counter; CreatedWallTime is only for display and
// retention and must never be used to choose a winning dataset.
type Checkpoint struct {
	ID              string
	Scope           Scope
	CreatedAt       RevisionOrder
	CreatedWallTime time.Time
	Origin          Origin
	BaseVersion     uint64
	DeviceLabel     string
	RecordCounts    map[string]int
	State           State
}

// Validate checks the required metadata and record counts.
func (c Checkpoint) Validate() error {
	if c.ID == "" {
		return errors.New("checkpoint ID is required")
	}
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("checkpoint scope: %w", err)
	}
	if c.CreatedWallTime.IsZero() {
		return errors.New("checkpoint creation time is required")
	}
	switch c.Origin {
	case OriginRejectedPush, OriginDisplacedServer, OriginManual:
	default:
		return fmt.Errorf("unknown checkpoint origin %q", c.Origin)
	}
	switch c.State {
	case StateKept, StateRestored, StateDiscarded, StateExpired:
	default:
		return fmt.Errorf("unknown checkpoint state %q", c.State)
	}
	for entity, count := range c.RecordCounts {
		if entity == "" || count < 0 {
			return errors.New("record counts need non-empty entity names and non-negative values")
		}
	}
	return nil
}

// IsExpired reports whether a kept checkpoint has reached its retention
// boundary. The supplied time is injectable so callers can test and schedule
// expiry without embedding a wall clock in this package.
func (c Checkpoint) IsExpired(now time.Time) bool {
	return c.State == StateKept && !c.CreatedWallTime.IsZero() && !now.Before(c.CreatedWallTime.Add(Retention))
}

// Expire marks an eligible checkpoint expired and reports whether it changed.
func (c *Checkpoint) Expire(now time.Time) bool {
	if c == nil || !c.IsExpired(now) {
		return false
	}
	c.State = StateExpired
	return true
}

// ListForScope returns unexpired, kept checkpoints from only the exact
// requested scope, ordered by dataset revision and device counter. The
// supplied time is injectable; wall time is used only to filter retention.
// It returns a new slice and does not alter the supplied checkpoint values.
func ListForScope(checkpoints []Checkpoint, scope Scope, now time.Time) []Checkpoint {
	if scope.Validate() != nil {
		return nil
	}
	result := make([]Checkpoint, 0)
	for _, checkpoint := range checkpoints {
		if checkpoint.Scope.Same(scope) && checkpoint.State == StateKept && !checkpoint.IsExpired(now) {
			result = append(result, checkpoint)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if order := result[i].CreatedAt.Compare(result[j].CreatedAt); order != 0 {
			return order < 0
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// CanRestore reports whether a kept checkpoint can be restored into the exact
// requested scope and has not expired. A checkpoint is never restorable across
// scope boundaries. The supplied time is used only for the retention check.
func (c Checkpoint) CanRestore(scope Scope, now time.Time) bool {
	return scope.Validate() == nil && c.State == StateKept && c.Scope.Same(scope) && !c.IsExpired(now)
}

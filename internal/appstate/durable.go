// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"sync"
)

// ErrDurabilityUnavailable means no durable storage boundary is installed.
var ErrDurabilityUnavailable = errors.New("saving is not ready; please keep this page open and try again")

// DurableResult describes completion of a requested store revision. A nil Err
// means that revision, including all its records, committed to durable storage.
type DurableResult struct {
	Revision uint64
	Err      error
}

// SetDurableWriter installs the page's persistence boundary. The writer must
// complete each request exactly once, after commit or with an explicit failure.
func (a *App) SetDurableWriter(write func(uint64, func(DurableResult))) {
	a.durableWriter = write
}

// RequestDurable asks persistence to commit at least the current revision. It
// never treats the request itself, a timer, or an in-memory SQL write as success.
func (a *App) RequestDurable(done func(DurableResult)) {
	if done == nil {
		return
	}
	revision := a.Rev()
	if a.durableWriter == nil {
		done(DurableResult{Revision: revision, Err: ErrDurabilityUnavailable})
		return
	}
	var once sync.Once
	a.durableWriter(revision, func(result DurableResult) {
		once.Do(func() {
			if result.Err == nil && result.Revision < revision {
				result.Err = ErrDurabilityUnavailable
			}
			done(result)
		})
	})
}

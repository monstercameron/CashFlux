// SPDX-License-Identifier: MIT

package dashboardview

import (
	"encoding/json"
	"slices"
	"sync/atomic"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/viewcache"
	"github.com/monstercameron/CashFlux/internal/widgetengine"
)

// FrameIdentity names the immutable data and clock snapshot behind all frames.
// Rates identifies the complete FX map; Scope identifies the full scope, rather
// than a slice length. Preferences also affect formula-series inputs.
type FrameIdentity struct {
	Instance, Revision uint64
	Preferences        int
	Scope, Base, Rates string
	Start, End         time.Time
	ClockMinute        int64
	VariablesAt        time.Time
}

// FrameContexts bounds retained financial snapshots independently of layout.
// Changing settings/pipelines changes the per-snapshot request key, while moving
// or hiding tiles can reuse the same financial context.
type FrameContexts struct {
	cache *viewcache.Cache[FrameIdentity, *PreparedFrames]
}

// NewFrameContexts creates a bounded cache of financial data snapshots.
func NewFrameContexts(capacity int) *FrameContexts {
	return &FrameContexts{cache: viewcache.New[FrameIdentity, *PreparedFrames](capacity)}
}

// Prepare reuses a matching snapshot or builds its immutable source context.
func (c *FrameContexts) Prepare(key FrameIdentity, build func() widgetengine.DataCtx) *PreparedFrames {
	return c.cache.GetOrCompute(key, func() *PreparedFrames {
		return &PreparedFrames{data: build(), frames: viewcache.New[string, preparedFrame](32)}
	})
}

// Len returns the number of retained financial contexts.
func (c *FrameContexts) Len() int { return c.cache.Len() }

// PreparedFrames owns one immutable hydrator context. Source slices and nested
// values follow the app snapshot contract: shared and read-only. Request keys
// serialize the current pipeline; returned frame columns are copied so changing
// a displayed cell cannot poison cached results.
type PreparedFrames struct {
	data       widgetengine.DataCtx
	frames     *viewcache.Cache[string, preparedFrame]
	hydrations atomic.Uint64
}
type preparedFrame struct {
	frame domain.Frame
	err   error
}

// Hydrate evaluates a pipeline once per snapshot/request key and returns a copy.
func (p *PreparedFrames) Hydrate(pipe *domain.Pipeline) (domain.Frame, error) {
	if pipe == nil {
		return widgetengine.HydrateFrame(nil, p.data)
	}
	key, err := json.Marshal(pipe)
	if err != nil {
		return domain.Frame{}, err
	}
	result := p.frames.GetOrCompute(string(key), func() preparedFrame {
		p.hydrations.Add(1)
		frame, err := widgetengine.HydrateFrame(pipe, p.data)
		return preparedFrame{frame: frame, err: err}
	})
	return cloneFrame(result.frame), result.err
}

// Hydrations counts actual source/transform evaluations, for focused host tests.
func (p *PreparedFrames) Hydrations() uint64 { return p.hydrations.Load() }

// Len returns the number of retained pipeline results in this snapshot.
func (p *PreparedFrames) Len() int { return p.frames.Len() }
func cloneFrame(frame domain.Frame) domain.Frame {
	fields := slices.Clone(frame.Fields)
	for i := range fields {
		fields[i].Values = slices.Clone(fields[i].Values)
	}
	frame.Fields = fields
	return frame
}

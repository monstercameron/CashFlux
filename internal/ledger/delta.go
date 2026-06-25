// SPDX-License-Identifier: MIT

package ledger

import "fmt"

// DeltaKind classifies the result of a period-over-period comparison.
type DeltaKind int

const (
	// DeltaPctKind means a normal percentage change was computable.
	DeltaPctKind DeltaKind = iota
	// DeltaNew means the value appeared from nothing (prev==0, curr!=0).
	// Reporting an infinite or undefined percentage would be meaningless, so
	// callers should surface this as "New" in the UI.
	DeltaNew
	// DeltaGone means the value vanished (curr==0, prev!=0).
	// Reporting -100% would be technically correct but less legible than "Gone".
	DeltaGone
	// DeltaZero means both values were zero; there is no change to report.
	DeltaZero
)

// String returns the name of the DeltaKind constant.
func (k DeltaKind) String() string {
	switch k {
	case DeltaPctKind:
		return "DeltaPctKind"
	case DeltaNew:
		return "DeltaNew"
	case DeltaGone:
		return "DeltaGone"
	case DeltaZero:
		return "DeltaZero"
	default:
		return fmt.Sprintf("DeltaKind(%d)", int(k))
	}
}

// DeltaResult holds the outcome of a Delta computation.
// Pct is the signed whole-number percentage change; it is only meaningful when
// Kind == DeltaPctKind.
type DeltaResult struct {
	Pct  int64
	Kind DeltaKind
}

// Delta computes the period-over-period change between curr and prev, both in
// the same integer minor units (e.g. cents).
//
// Prior-zero semantics — when prev is zero the percentage denominator is
// undefined, so Delta returns a descriptive kind instead of a raw number:
//   - prev==0 && curr!=0 → DeltaNew  (something appeared where there was nothing)
//   - curr==0 && prev!=0 → DeltaGone (something vanished)
//   - both==0            → DeltaZero (nothing to report)
//
// When both values are non-zero, Kind is DeltaPctKind and Pct is computed as:
//
//	(curr - prev) * 100 / |prev|
//
// Dividing by the magnitude of prev (not its signed value) means that a move
// from -100 to -50 is correctly reported as +50% (an improvement), not -50%.
// The result truncates toward zero, matching Go's integer division.
func Delta(curr, prev int64) DeltaResult {
	switch {
	case prev == 0 && curr == 0:
		return DeltaResult{Kind: DeltaZero}
	case prev == 0:
		return DeltaResult{Kind: DeltaNew}
	case curr == 0:
		return DeltaResult{Kind: DeltaGone}
	default:
		mag := prev
		if mag < 0 {
			mag = -mag
		}
		return DeltaResult{
			Pct:  (curr - prev) * 100 / mag,
			Kind: DeltaPctKind,
		}
	}
}

// Label returns a short human-readable string suitable for display in a UI cell.
//
//   - DeltaNew      → "New"
//   - DeltaGone     → "Gone"
//   - DeltaZero     → "—"
//   - DeltaPctKind  → e.g. "+12%" or "-5%"
func (d DeltaResult) Label() string {
	switch d.Kind {
	case DeltaNew:
		return "New"
	case DeltaGone:
		return "Gone"
	case DeltaZero:
		return "—"
	default:
		return fmt.Sprintf("%+d%%", d.Pct)
	}
}

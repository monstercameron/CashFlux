// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PutPlacementsIfChanged persists only placements whose stored JSON differs.
// Dashboard calls this after mounting; identical writes would otherwise advance
// the store revision and invalidate financial snapshots on every visit.
// Placements absent from ps are left alone, matching PutPlacements semantics.
// The returned bool reports whether any row was written, including before an
// error on a later row; the next call reads the store and retries remaining rows.
func (a *App) PutPlacementsIfChanged(surface string, ps []domain.Placement) (bool, error) {
	if surface == "" {
		return false, fmt.Errorf("appstate: placement surface is required")
	}
	if len(ps) == 0 {
		return false, nil
	}
	stored, err := a.store.PlacementsForSurface(surface)
	if err != nil {
		return false, fmt.Errorf("appstate: read %s placements: %w", surface, err)
	}
	byID := make(map[string][]byte, len(stored))
	for _, p := range stored {
		encoded, err := json.Marshal(p)
		if err != nil {
			return false, fmt.Errorf("appstate: encode stored placement %s/%s: %w", surface, p.ID, err)
		}
		byID[p.ID] = encoded
	}
	pending := make([]domain.Placement, 0, len(ps))
	for _, p := range ps {
		if p.Surface != surface {
			return false, fmt.Errorf("appstate: placement %q belongs to %q, want %q", p.ID, p.Surface, surface)
		}
		encoded, err := json.Marshal(p)
		if err != nil {
			return false, fmt.Errorf("appstate: encode placement %s/%s: %w", surface, p.ID, err)
		}
		if bytes.Equal(encoded, byID[p.ID]) {
			continue
		}
		pending = append(pending, p)
		byID[p.ID] = encoded
	}
	changed := false
	for _, p := range pending {
		if err := a.store.PutPlacement(p); err != nil {
			return changed, fmt.Errorf("appstate: save placement %s/%s: %w", surface, p.ID, err)
		}
		changed = true
	}
	return changed, nil
}

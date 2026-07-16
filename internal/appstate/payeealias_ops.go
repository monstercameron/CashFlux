// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/payeealias"
)

// PayeeAliases returns every persisted payee alias (the learned merchant-name
// cleanup table — TX1).
func (a *App) PayeeAliases() []domain.PayeeAlias {
	v, err := a.store.ListPayeeAliases()
	a.logErr("payee aliases", err)
	return v
}

// PayeeResolver builds a resolver over the current alias table. Callers use it to
// map a raw payee to its clean display name (learned alias → rule pack → raw).
// It is cheap to construct; build one per render pass rather than caching stale.
func (a *App) PayeeResolver() *payeealias.Resolver {
	return payeealias.NewResolver(a.PayeeAliases())
}

// ResolvePayee returns the clean display name for a raw payee using the current
// alias table and the built-in normalizer rule pack. Convenience wrapper for
// one-off lookups; when resolving many payees, build a PayeeResolver once.
func (a *App) ResolvePayee(raw string) string {
	return a.PayeeResolver().Resolve(raw)
}

// PutPayeeAlias validates and persists a payee alias, creating an id and stamping
// CreatedAt when the alias is new. When an alias already exists for the same raw
// payee (case-insensitively) or the same id its display name is updated in place
// rather than creating a duplicate row, so re-learning a name is idempotent — and
// the PRIOR display is appended to the rename History so the cleanup UI can show the
// lineage.
//
// Validation:
//   - RawPayee and Display are non-empty after trimming.
func (a *App) PutPayeeAlias(p domain.PayeeAlias) error {
	p.RawPayee = strings.TrimSpace(p.RawPayee)
	p.Display = strings.TrimSpace(p.Display)
	if p.RawPayee == "" {
		return fmt.Errorf("appstate: a payee alias needs a raw payee")
	}
	if p.Display == "" {
		return fmt.Errorf("appstate: a payee alias needs a display name")
	}

	// Find any existing alias for this mapping — by id (a /rules inline edit) or by raw
	// payee (a fresh cleanup with no id) — so the row is updated in place, CreatedAt and
	// History are preserved, and a genuine rename is recorded in the lineage.
	all := a.PayeeAliases()
	var existing *domain.PayeeAlias
	if pid := strings.TrimSpace(p.ID); pid != "" {
		for i := range all {
			if all[i].ID == pid {
				existing = &all[i]
				break
			}
		}
	}
	if existing == nil {
		key := strings.ToLower(p.RawPayee)
		for i := range all {
			if strings.ToLower(strings.TrimSpace(all[i].RawPayee)) == key {
				existing = &all[i]
				break
			}
		}
	}
	if existing != nil {
		p.ID = existing.ID
		p.CreatedAt = existing.CreatedAt
		p.History = existing.History
		// Record the previous display only when it actually changed, so re-saving the
		// same name doesn't pad the history with duplicates.
		if prev := strings.TrimSpace(existing.Display); prev != "" && !strings.EqualFold(prev, p.Display) {
			p.History = append(p.History, domain.PayeeAliasRename{Display: prev, At: a.clock()})
		}
	}
	if strings.TrimSpace(p.ID) == "" {
		p.ID = id.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = a.clock()
	}
	if err := a.store.PutPayeeAlias(p); err != nil {
		return fmt.Errorf("appstate: save payee alias: %w", err)
	}
	a.log.Info("payee alias saved", "id", p.ID, "raw", p.RawPayee, "display", p.Display, "renames", len(p.History))
	return nil
}

// DeletePayeeAlias removes a payee alias by id. Deleting an alias only removes the
// view-layer mapping — the raw payee stays on every transaction it named.
func (a *App) DeletePayeeAlias(aliasID string) error {
	if strings.TrimSpace(aliasID) == "" {
		return fmt.Errorf("appstate: payee-alias id is required")
	}
	return a.del("payee alias", aliasID, a.store.DeletePayeeAlias)
}

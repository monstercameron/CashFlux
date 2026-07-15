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
// payee (case-insensitively) its display name is updated in place rather than
// creating a duplicate row, so re-learning a name is idempotent.
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

	// Merge onto any existing alias for the same raw payee (case-insensitive).
	if strings.TrimSpace(p.ID) == "" {
		key := strings.ToLower(p.RawPayee)
		for _, ex := range a.PayeeAliases() {
			if strings.ToLower(strings.TrimSpace(ex.RawPayee)) == key {
				p.ID = ex.ID
				p.CreatedAt = ex.CreatedAt
				break
			}
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
	a.log.Info("payee alias saved", "id", p.ID, "raw", p.RawPayee, "display", p.Display)
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

// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/sweep"
)

// SweepRules returns every persisted surplus-sweep rule (AC7), sorted by source
// account id then id for a stable order.
func (a *App) SweepRules() []domain.SweepRule {
	v, err := a.store.ListSweepRules()
	a.logErr("sweep rules", err)
	sort.SliceStable(v, func(i, j int) bool {
		if v[i].SourceAccountID != v[j].SourceAccountID {
			return v[i].SourceAccountID < v[j].SourceAccountID
		}
		return v[i].ID < v[j].ID
	})
	return v
}

// PutSweepRule validates and persists a sweep rule (AC7), assigning an id when the
// rule is new and defaulting its cadence to monthly. Source and destination must be
// two distinct existing accounts and the keep floor may not be negative.
func (a *App) PutSweepRule(r domain.SweepRule) (domain.SweepRule, error) {
	r.SourceAccountID = strings.TrimSpace(r.SourceAccountID)
	r.DestAccountID = strings.TrimSpace(r.DestAccountID)
	if r.SourceAccountID == "" || r.DestAccountID == "" {
		return domain.SweepRule{}, fmt.Errorf("appstate: a sweep rule needs a source and a destination account")
	}
	if r.SourceAccountID == r.DestAccountID {
		return domain.SweepRule{}, fmt.Errorf("appstate: a sweep rule's source and destination must differ")
	}
	if _, ok := a.findAccount(r.SourceAccountID); !ok {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep source account not found")
	}
	if _, ok := a.findAccount(r.DestAccountID); !ok {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep destination account not found")
	}
	if r.KeepMinor < 0 || r.MinSweepMinor < 0 {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep amounts may not be negative")
	}
	if !r.Cadence.Valid() {
		r.Cadence = domain.SweepMonthly
	}
	if strings.TrimSpace(r.ID) == "" {
		r.ID = id.New()
	}
	if err := a.store.PutSweepRule(r); err != nil {
		return domain.SweepRule{}, fmt.Errorf("appstate: save sweep rule: %w", err)
	}
	a.log.Info("sweep rule saved", "id", r.ID)
	return r, nil
}

// DeleteSweepRule removes a sweep rule by id.
func (a *App) DeleteSweepRule(ruleID string) error {
	if _, err := a.store.DeleteSweepRule(ruleID); err != nil {
		return fmt.Errorf("appstate: delete sweep rule: %w", err)
	}
	return nil
}

// MarkSweepProposed records that a rule generated a proposal as of now, resetting
// its cadence clock. The UI calls this when it surfaces (or the user acts on) a
// proposal so the same sweep is not re-proposed until the next cadence window.
func (a *App) MarkSweepProposed(ruleID string, now time.Time) error {
	r, ok, err := a.store.GetSweepRule(ruleID)
	if err != nil {
		return fmt.Errorf("appstate: load sweep rule: %w", err)
	}
	if !ok {
		return fmt.Errorf("appstate: sweep rule not found")
	}
	r.LastProposed = now.Format(time.RFC3339)
	if err := a.store.PutSweepRule(r); err != nil {
		return fmt.Errorf("appstate: update sweep rule: %w", err)
	}
	return nil
}

// SweepProposals evaluates every enabled sweep rule against the current balances
// and returns the transfers it proposes as of now (AC7). Nothing is executed — the
// UI shows each Proposal on the payday-waterfall approval surface (GL1) and only
// the transfer flow mutates the ledger on approval. The sweepable excess respects
// earmark integrity (XC7): goal-reserved money is never swept.
func (a *App) SweepProposals(now time.Time) []sweep.Proposal {
	txns := a.Transactions()
	gs := a.Goals()
	var out []sweep.Proposal
	for _, r := range a.SweepRules() {
		src, ok := a.findAccount(r.SourceAccountID)
		if !ok {
			continue
		}
		bal, err := ledger.Balance(src, txns)
		if err != nil {
			a.logErr("sweep balance", err)
			continue
		}
		earmarked := goals.AccountEarmarkedMinor(gs, src.ID, "")
		in := sweep.Inputs{BalanceMinor: bal.Amount, EarmarkedMinor: earmarked, Currency: src.Currency}
		if p, ok := sweep.Propose(r, in, now); ok {
			out = append(out, p)
		}
	}
	return out
}

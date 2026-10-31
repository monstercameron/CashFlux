// SPDX-License-Identifier: MIT

package appstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/sweep"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// ErrSweepPostedUnmarked is returned by ApproveSweepProposal when the transfer posted but the
// proposal could not be marked handled. The money has moved, so the proposal must not be approved
// again.
var ErrSweepPostedUnmarked = errors.New("appstate: sweep transfer posted but the proposal was not marked handled")

// ErrSweepAlreadyPosted is returned by ApproveSweepProposal when the rule already swept in this
// cadence window, or swept within its cadence before a replay that landed in the next window. Nothing
// new was moved and the rule's stamp already names that sweep.
var ErrSweepAlreadyPosted = errors.New("appstate: the sweep rule already swept in this cadence window")

// ErrSweepStampRepaired is returned by ApproveSweepProposal when the sweep for this window posted
// earlier and only the rule's cadence stamp was restored. Nothing moved now, and the replayed
// proposal's amount is not the amount that moved earlier.
var ErrSweepStampRepaired = errors.New("appstate: the sweep posted earlier and only the cadence stamp was restored")

// ErrSweepProposalStale is returned by ApproveSweepProposal when the proposal no longer matches the
// rule or the balances it was built from, so nothing was posted.
var ErrSweepProposalStale = errors.New("appstate: sweep proposal is out of date")

// SweepRules returns every persisted surplus-sweep rule (AC7), sorted by source
// account id then id for a stable order.
func (a *App) SweepRules() []domain.SweepRule {
	if a.ready() != nil {
		return nil
	}
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

// SweepPairError reports why a source and destination cannot carry a sweep, or nil when they can:
// the two are the same account, either one is archived, the source is a liability (sweeping out of
// debt is a cash advance, never a surplus), or the two hold different currencies. PutSweepRule,
// the approval of a proposal and the sweep rules form's pickers all apply this one rule.
func SweepPairError(src, dest domain.Account) error {
	switch {
	case src.ID == dest.ID:
		return fmt.Errorf("a sweep rule's source and destination must differ")
	case src.Archived || dest.Archived:
		return fmt.Errorf("archived accounts cannot take part in a sweep")
	case src.IsLiability():
		return fmt.Errorf("a liability cannot be the source of a sweep")
	case src.Currency != dest.Currency:
		return fmt.Errorf("a sweep needs both accounts in the same currency")
	}
	return nil
}

// PutSweepRule validates and persists a sweep rule (AC7), assigning an id when the
// rule is new and defaulting its cadence to monthly. Source and destination must be
// two distinct, active accounts in one currency, the source must not be a liability,
// and the keep floor may not be negative. An existing rule keeps its stored LastProposed
// whatever the caller passes.
func (a *App) PutSweepRule(r domain.SweepRule) (domain.SweepRule, error) {
	if err := a.roleGuard(); err != nil {
		return domain.SweepRule{}, err
	}
	r.SourceAccountID = strings.TrimSpace(r.SourceAccountID)
	r.DestAccountID = strings.TrimSpace(r.DestAccountID)
	if r.SourceAccountID == "" || r.DestAccountID == "" {
		return domain.SweepRule{}, fmt.Errorf("appstate: a sweep rule needs a source and a destination account")
	}
	if r.SourceAccountID == r.DestAccountID {
		return domain.SweepRule{}, fmt.Errorf("appstate: a sweep rule's source and destination must differ")
	}
	src, ok := a.findAccount(r.SourceAccountID)
	if !ok {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep source account not found")
	}
	dest, ok := a.findAccount(r.DestAccountID)
	if !ok {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep destination account not found")
	}
	if err := SweepPairError(src, dest); err != nil {
		return domain.SweepRule{}, fmt.Errorf("appstate: %w", err)
	}
	if r.KeepMinor < 0 || r.MinSweepMinor < 0 {
		return domain.SweepRule{}, fmt.Errorf("appstate: sweep amounts may not be negative")
	}
	if !r.Cadence.Valid() {
		r.Cadence = domain.SweepMonthly
	}
	if strings.TrimSpace(r.ID) == "" {
		r.ID = id.New()
	} else if stored, found, err := a.store.GetSweepRule(r.ID); err != nil {
		return domain.SweepRule{}, fmt.Errorf("appstate: load sweep rule: %w", err)
	} else if found {
		// The proposal stamp belongs to MarkSweepProposed and ApproveSweepProposal; an edit never
		// rewinds the cadence clock, so a rule saved from a fresh struct is not re-proposed at once.
		r.LastProposed = stored.LastProposed
	}
	if err := a.store.PutSweepRule(r); err != nil {
		return domain.SweepRule{}, fmt.Errorf("appstate: save sweep rule: %w", err)
	}
	a.log.Info("sweep rule saved", "id", r.ID)
	return r, nil
}

// DeleteSweepRule removes a sweep rule by id.
func (a *App) DeleteSweepRule(ruleID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if _, err := a.store.DeleteSweepRule(ruleID); err != nil {
		return fmt.Errorf("appstate: delete sweep rule: %w", err)
	}
	return nil
}

// MarkSweepProposed records that a rule generated a proposal as of now, resetting
// its cadence clock. The UI calls this when it surfaces (or the user acts on) a
// proposal so the same sweep is not re-proposed until the next cadence window.
func (a *App) MarkSweepProposed(ruleID string, now time.Time) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if now.IsZero() {
		now = a.clock()
	}
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

// sweepStampTime parses a rule's LastProposed stamp, an RFC3339 time or a plain date. It reports
// false when the stamp is empty or not a date.
func sweepStampTime(r domain.SweepRule) (time.Time, bool) {
	if r.LastProposed == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, r.LastProposed); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", r.LastProposed); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// sweepOperationID names the one sweep a rule may post within its current cadence window. The
// amount is not part of the id, so approving the same rule twice in a window, even for different
// amounts, resolves to the same transfer and posts at most once. The window is counted in the same
// calendar days the stamp and the transfer date use (normalizeTransactionDate, the local day as
// midnight UTC) and that sweep.IsDue compares, so a rule that is due always lands in a window after
// the one that posted, in any time zone.
func sweepOperationID(r domain.SweepRule, now time.Time) string {
	return sweepOperationIDForCadence(r.ID, r.Cadence, now)
}

// sweepOperationIDForCadence is sweepOperationID for an explicit cadence, so the window a sweep
// posted under can be rebuilt after the rule's cadence changed.
func sweepOperationIDForCadence(ruleID string, cadence domain.SweepCadence, now time.Time) string {
	window := normalizeTransactionDate(now).Unix() / 86400 / int64(cadence.CadenceDays())
	sum := sha256.Sum256([]byte(fmt.Sprintf("sweep|%s|%d", ruleID, window)))
	return "sweep-" + hex.EncodeToString(sum[:16])
}

// sweepCadences lists every cadence a sweep can have posted under.
var sweepCadences = []domain.SweepCadence{domain.SweepWeekly, domain.SweepBiweekly, domain.SweepMonthly, domain.SweepQuarterly}

// sweepPostedForStamp reports whether the sweep a rule's LastProposed stamp names has its outgoing
// transfer leg in the ledger. The stamp's calendar day picks the window that sweep posted in, so a
// stamp written by a sweep always finds its transfer and a stamp set any other way finds none. The
// rule's cadence may have been edited since the sweep posted, so the window is rebuilt under each
// cadence, the rule's current one first.
func (a *App) sweepPostedForStamp(rule domain.SweepRule) (bool, error) {
	stamp, ok := sweepStampTime(rule)
	if !ok {
		return false, nil
	}
	cadences := append([]domain.SweepCadence{rule.Cadence}, sweepCadences...)
	for _, cadence := range cadences {
		_, found, err := a.store.GetTransaction(transferID(sweepOperationIDForCadence(rule.ID, cadence, stamp), "out"))
		if err != nil {
			return false, fmt.Errorf("sweep transfer: check earlier approval: %w", err)
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

// sweepInputs reads the source account's booked balance and the money reserved against it: goal
// allocations plus earmarks pointing at the account in its own currency.
func (a *App) sweepInputs(src domain.Account, txns []domain.Transaction, gs []domain.Goal, earmarks []domain.Earmark) (sweep.Inputs, error) {
	bal, err := ledger.Balance(src, txns)
	if err != nil {
		return sweep.Inputs{}, err
	}
	reserved := goals.AccountEarmarkedMinor(gs, src.ID, "")
	for _, e := range earmarks {
		cur := e.Currency
		if cur == "" {
			cur = e.Amount.Currency
		}
		if e.DestinationID == src.ID && cur == src.Currency {
			reserved += e.Amount.Amount
		}
	}
	return sweep.Inputs{BalanceMinor: bal.Amount, EarmarkedMinor: reserved, Currency: src.Currency}, nil
}

// ApproveSweepProposal moves the money a sweep proposal describes and marks the proposal handled in
// one commit: the transfer rows, their operation receipt and the rule's LastProposed stamp are
// written together, so a failure leaves nothing half-done. Before anything posts it re-reads the
// rule, both accounts, goals, earmarks and the source balance, and refuses a proposal that no longer
// matches them (ErrSweepProposalStale), whose rule is disabled, or whose amount is no longer
// sweepable surplus. A rule that is not due again (sweep.IsDue) or an amount under the rule's
// MinSweepMinor is refused the same way, except that a rule not due again whose last stamp names a
// sweep that posted returns ErrSweepAlreadyPosted, since that money already moved. The transfer is dated now and carries an operation id
// derived from the rule and its cadence window only, so approving the rule again in that window
// posts nothing. A replay when the rule's LastProposed already marks the posted transfer returns
// ErrSweepAlreadyPosted whatever the amount, because nothing new moved. Only a replay that finds
// the stamp missing or on an earlier calendar day than the posted transfer repairs it, to the transfer's own date rather
// than the replay's time. It returns nil when the replay's amount is the amount that moved, and
// ErrSweepStampRepaired when it is not, since nothing moved now; if the repair fails the error wraps
// ErrSweepPostedUnmarked. When the rule's cadence was edited after the sweep posted, the window is
// rebuilt under each cadence to find the posted transfer (sweepPostedForStamp).
func (a *App) ApproveSweepProposal(p sweep.Proposal, desc string, now time.Time) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if now.IsZero() {
		now = a.clock()
	}
	rule, ok, err := a.store.GetSweepRule(p.RuleID)
	if err != nil {
		return fmt.Errorf("sweep transfer: load sweep rule: %w", err)
	}
	if !ok {
		return fmt.Errorf("sweep transfer: %w: the rule no longer exists", ErrSweepProposalStale)
	}
	if !rule.Enabled {
		return fmt.Errorf("sweep transfer: %w: the rule is turned off", ErrSweepProposalStale)
	}
	if p.AmountMinor <= 0 || rule.SourceAccountID != p.SourceAccountID || rule.DestAccountID != p.DestAccountID {
		return fmt.Errorf("sweep transfer: %w: the rule changed", ErrSweepProposalStale)
	}
	src, ok := a.findAccount(rule.SourceAccountID)
	if !ok {
		return fmt.Errorf("sweep transfer: source account not found")
	}
	dest, ok := a.findAccount(rule.DestAccountID)
	if !ok {
		return fmt.Errorf("sweep transfer: destination account not found")
	}
	if err := SweepPairError(src, dest); err != nil {
		return fmt.Errorf("sweep transfer: %w", err)
	}
	opID := sweepOperationID(rule, now)
	posted, done, err := a.store.GetTransaction(transferID(opID, "out"))
	if err != nil {
		return fmt.Errorf("sweep transfer: check earlier approval: %w", err)
	}
	if done {
		// The sweep for this window already posted, so no money moves. When the rule already carries a
		// stamp on or after the posted transfer's calendar day there is nothing to repair and nothing
		// new happened. Both are reduced to the calendar day they show in their own location, the way
		// sweep.IsDue reads the stamp, so a stamp written in a zone ahead of UTC is not mistaken for an
		// earlier day than its transfer's UTC-midnight date.
		if last, stamped := sweepStampTime(rule); stamped && !normalizeTransactionDate(last).Before(normalizeTransactionDate(posted.Date)) {
			return fmt.Errorf("sweep transfer: %w", ErrSweepAlreadyPosted)
		}
		// The stamp is missing or on an earlier day than the transfer: restore it to the transfer's own date, never
		// to the replay's time, so a replay cannot push the next due date later.
		if err := a.MarkSweepProposed(p.RuleID, posted.Date); err != nil {
			return fmt.Errorf("%w: %v", ErrSweepPostedUnmarked, err)
		}
		// Only a replay of the amount that moved reports success; any other amount did not move now.
		if moved := posted.Amount.Amount; p.AmountMinor != moved && p.AmountMinor != -moved {
			return fmt.Errorf("sweep transfer: %w", ErrSweepStampRepaired)
		}
		return nil
	}
	if !sweep.IsDue(rule, now) {
		// A replay can land in the window after the one that posted, for example just past midnight
		// when the sweep posted late the evening before. If the rule's last stamp is a sweep that really
		// posted, the money already moved and the caller must not be told the proposal is out of date.
		stampPosted, err := a.sweepPostedForStamp(rule)
		if err != nil {
			return err
		}
		if stampPosted {
			return fmt.Errorf("sweep transfer: %w", ErrSweepAlreadyPosted)
		}
		return fmt.Errorf("sweep transfer: %w: the rule is not due again yet", ErrSweepProposalStale)
	}
	minSweep := rule.MinSweepMinor
	if minSweep < 1 {
		minSweep = 1
	}
	if p.AmountMinor < minSweep {
		return fmt.Errorf("sweep transfer: %w: the amount is below the rule's minimum sweep", ErrSweepProposalStale)
	}
	gs, err := a.GoalsChecked()
	if err != nil {
		return fmt.Errorf("sweep transfer: read goals: %w", err)
	}
	earmarks, err := a.store.ListEarmarks()
	if err != nil {
		return fmt.Errorf("sweep transfer: read earmarks: %w", err)
	}
	in, err := a.sweepInputs(src, a.Transactions(), gs, earmarks)
	if err != nil {
		return fmt.Errorf("sweep transfer: read source balance: %w", err)
	}
	if p.AmountMinor > sweep.ExcessMinor(rule, in) {
		return fmt.Errorf("sweep transfer: %w: the surplus is now smaller than the proposal", ErrSweepProposalStale)
	}
	return a.commitSweepTransfer(rule, src, dest, TransferParams{
		OperationID:   opID,
		FromAccountID: p.SourceAccountID,
		ToAccountID:   p.DestAccountID,
		AmountMinor:   p.AmountMinor,
		Date:          now,
		Desc:          desc,
	}, now)
}

// commitSweepTransfer writes a sweep's transfer rows, its operation receipt and the rule's
// LastProposed stamp in one expected-state commit, so it fails if the rule changed since it was read
// or if any of the rows already exists.
func (a *App) commitSweepTransfer(rule domain.SweepRule, src, dest domain.Account, p TransferParams, now time.Time) error {
	legs, _, _, err := buildTransferRows(p, src, dest, a.transactionPairRates())
	if err != nil {
		return fmt.Errorf("sweep transfer: %w", err)
	}
	accounts := a.Accounts()
	for _, leg := range legs {
		if issues := validate.ValidateTransaction(leg, accounts...); !issues.OK() {
			return fmt.Errorf("sweep transfer: %w", issues)
		}
		if err := a.validateCustom("transaction", leg.Custom); err != nil {
			return fmt.Errorf("sweep transfer: %w", err)
		}
	}
	changes := make([]store.RecordChange, 0, len(legs)+2)
	for _, leg := range legs {
		data, err := json.Marshal(leg)
		if err != nil {
			return fmt.Errorf("sweep transfer: encode leg: %w", err)
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: leg.ID, After: data})
	}
	receipt, err := json.Marshal(legs)
	if err != nil {
		return fmt.Errorf("sweep transfer: encode operation receipt: %w", err)
	}
	changes = append(changes, store.RecordChange{Table: "transfer_operations", ID: p.OperationID, After: receipt})
	marked := rule
	marked.LastProposed = normalizeTransactionDate(now).Format(time.RFC3339)
	before, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("sweep transfer: encode rule: %w", err)
	}
	after, err := json.Marshal(marked)
	if err != nil {
		return fmt.Errorf("sweep transfer: encode updated rule: %w", err)
	}
	changes = append(changes, store.RecordChange{Table: "sweeprules", ID: rule.ID, Before: before, After: after})
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("sweep transfer: commit: %w", err)
	}
	a.log.Info("sweep committed", "rule", rule.ID, "operation_id", p.OperationID)
	a.fireTxnMutated()
	return nil
}

// SweepProposals evaluates every enabled sweep rule against the current balances
// and returns the transfers it proposes as of now (AC7). Nothing is executed — the
// UI shows each Proposal on the payday-waterfall approval surface (GL1) and only
// the transfer flow mutates the ledger on approval. The sweepable excess respects
// earmark integrity (XC7): goal allocations and earmarks reserved against the source
// are never swept. Rules sharing a source draw on one running balance, so their
// proposals never add up to more than the source holds; a proposal's BalanceMinor is
// the balance left after the proposals before it. Rules whose accounts are missing,
// archived, in different currencies or sourced from a liability propose nothing, and
// when goals or earmarks cannot be read nothing is proposed, because the reserved
// money is unknown.
func (a *App) SweepProposals(now time.Time) []sweep.Proposal {
	if a.ready() != nil {
		return nil
	}
	if now.IsZero() {
		now = a.clock()
	}
	txns := a.Transactions()
	gs, err := a.GoalsChecked()
	if err != nil {
		a.logErr("sweep goals", err)
		return nil
	}
	earmarks, err := a.store.ListEarmarks()
	if err != nil {
		a.logErr("sweep earmarks", err)
		return nil
	}
	remaining := make(map[string]int64)
	var out []sweep.Proposal
	for _, r := range a.SweepRules() {
		src, ok := a.findAccount(r.SourceAccountID)
		if !ok {
			continue
		}
		dest, ok := a.findAccount(r.DestAccountID)
		if !ok || SweepPairError(src, dest) != nil {
			continue
		}
		in, err := a.sweepInputs(src, txns, gs, earmarks)
		if err != nil {
			a.logErr("sweep balance", err)
			continue
		}
		if left, seen := remaining[src.ID]; seen {
			in.BalanceMinor = left
		}
		if p, ok := sweep.Propose(r, in, now); ok {
			out = append(out, p)
			remaining[src.ID] = in.BalanceMinor - p.AmountMinor
		}
	}
	return out
}

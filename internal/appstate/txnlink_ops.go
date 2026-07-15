// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// TxnLinks returns every persisted transaction-link (order groups and refund
// pairs — the XC0b link primitive).
func (a *App) TxnLinks() []domain.TxnLink {
	v, err := a.store.ListTxnLinks()
	a.logErr("txn links", err)
	return v
}

// PutTxnLink validates and persists a transaction link (creating an id when the
// link is new). Grouping and pairing are read-model overlays, so this never
// touches the linked transactions themselves.
//
// Validation (the XC0b invariants):
//   - Kind is a known TxnLinkKind.
//   - An order-group has at least 2 members; a refund-pair has exactly 2.
//   - Member ids are non-empty and distinct.
//   - A transaction belongs to at most one order-group (a member already in a
//     different group is rejected, naming the conflict).
func (a *App) PutTxnLink(l domain.TxnLink) error {
	if !domain.KnownTxnLinkKind(l.Kind) {
		return fmt.Errorf("appstate: unknown transaction-link kind %q", l.Kind)
	}
	seen := map[string]bool{}
	for _, tid := range l.TxnIDs {
		if strings.TrimSpace(tid) == "" {
			return fmt.Errorf("appstate: transaction link has an empty member id")
		}
		if seen[tid] {
			return fmt.Errorf("appstate: transaction %q appears twice in the same link", tid)
		}
		seen[tid] = true
	}
	switch l.Kind {
	case domain.TxnLinkOrderGroup:
		if len(l.TxnIDs) < 2 {
			return fmt.Errorf("appstate: an order group needs at least 2 transactions")
		}
	case domain.TxnLinkRefundPair:
		if len(l.TxnIDs) != 2 {
			return fmt.Errorf("appstate: a %s links exactly 2 transactions", l.Kind)
		}
	case domain.TxnLinkBillMatch:
		// A bill-match ties ONE transaction to ONE recurring occurrence (TX9): the
		// single member is the paying transaction; the occurrence lives on the
		// RecurringID/OccurrenceDate fields, not as a second transaction member.
		if len(l.TxnIDs) != 1 {
			return fmt.Errorf("appstate: a bill-match links exactly 1 transaction")
		}
		if strings.TrimSpace(l.RecurringID) == "" || l.OccurrenceDate.IsZero() {
			return fmt.Errorf("appstate: a bill-match needs a recurring id and occurrence date")
		}
	case domain.TxnLinkEventTxn:
		// An event link maps ONE transaction to ONE Event (TX10): the single
		// member is the transaction; the event lives on the EventID field.
		if len(l.TxnIDs) != 1 {
			return fmt.Errorf("appstate: an event link maps exactly 1 transaction")
		}
		if strings.TrimSpace(l.EventID) == "" {
			return fmt.Errorf("appstate: an event link needs an event id")
		}
		if _, ok, _ := a.store.GetEvent(l.EventID); !ok {
			return fmt.Errorf("appstate: event %q does not exist", l.EventID)
		}
		// One event-link per transaction per event.
		for _, other := range a.TxnLinks() {
			if other.ID == l.ID || other.Kind != domain.TxnLinkEventTxn || other.EventID != l.EventID {
				continue
			}
			if other.HasTxn(l.TxnIDs[0]) {
				return fmt.Errorf("appstate: transaction %q is already in this event", l.TxnIDs[0])
			}
		}
	}

	// One order-group per transaction: reject a member already in a different group.
	if l.Kind == domain.TxnLinkOrderGroup {
		existing := a.TxnLinks()
		for _, other := range existing {
			if other.ID == l.ID || other.Kind != domain.TxnLinkOrderGroup {
				continue
			}
			for _, tid := range l.TxnIDs {
				if other.HasTxn(tid) {
					return fmt.Errorf("appstate: transaction %q is already in another group", tid)
				}
			}
		}
	}

	if strings.TrimSpace(l.ID) == "" {
		l.ID = id.New()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = a.clock()
	}
	if err := a.store.PutTxnLink(l); err != nil {
		return fmt.Errorf("appstate: save transaction link: %w", err)
	}
	a.log.Info("transaction link saved", "id", l.ID, "kind", l.Kind, "members", len(l.TxnIDs))
	a.SyncTxnLinkNetting()
	return nil
}

// SyncTxnLinkNetting installs the current refund-pair links into the budgeting
// and reports read-model netting overlays (XC2), so a paired refund nets in its
// original purchase's period across budget bars and reports. It is idempotent
// and cheap; it runs after every link change and once after each dataset
// load/import/wipe (wired in New, ImportJSON, ImportJSONWithBlobs, Wipe).
func (a *App) SyncTxnLinkNetting() {
	links := a.TxnLinks()
	budgeting.SetRefundLinks(links)
	reports.SetRefundLinks(links)
}

// DeleteTxnLink removes a transaction link by id, releasing its members. The
// transactions are never deleted — only the relation between them.
func (a *App) DeleteTxnLink(linkID string) error {
	if strings.TrimSpace(linkID) == "" {
		return fmt.Errorf("appstate: transaction-link id is required")
	}
	if err := a.del("txn link", linkID, a.store.DeleteTxnLink); err != nil {
		return err
	}
	a.SyncTxnLinkNetting()
	return nil
}

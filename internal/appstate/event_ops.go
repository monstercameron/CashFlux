// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/events"
	"github.com/monstercameron/CashFlux/internal/id"
)

// Events returns every persisted event (the first-class spending events — TX10).
func (a *App) Events() []domain.Event {
	v, err := a.store.ListEvents()
	a.logErr("events", err)
	return v
}

// GetEvent returns the event with the given id and whether it exists.
func (a *App) GetEvent(eventID string) (domain.Event, bool) {
	e, ok, err := a.store.GetEvent(eventID)
	a.logErr("event", err)
	return e, ok
}

// PutEvent validates and persists an event, assigning an id and creation stamp
// when the event is new. An event needs a non-empty name; when both dates are set
// the end must not precede the start (a zero end is open-ended and always valid).
func (a *App) PutEvent(e domain.Event) (domain.Event, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return domain.Event{}, fmt.Errorf("appstate: an event needs a name")
	}
	if e.Start.IsZero() {
		return domain.Event{}, fmt.Errorf("appstate: an event needs a start date")
	}
	if !e.End.IsZero() && e.End.Before(e.Start) {
		return domain.Event{}, fmt.Errorf("appstate: an event's end cannot be before its start")
	}
	if strings.TrimSpace(e.ID) == "" {
		e.ID = id.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = a.clock()
	}
	if err := a.store.PutEvent(e); err != nil {
		return domain.Event{}, fmt.Errorf("appstate: save event: %w", err)
	}
	a.log.Info("event saved", "id", e.ID, "name", e.Name)
	return e, nil
}

// DeleteEvent removes an event and unmaps its transactions by deleting every
// event-member link that referenced it. The transactions themselves are never
// touched (reassign-on-delete = unmap, per the house rule).
func (a *App) DeleteEvent(eventID string) error {
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("appstate: event id is required")
	}
	for _, l := range a.TxnLinks() {
		if l.Kind == domain.TxnLinkEventTxn && l.EventID == eventID {
			if _, err := a.store.DeleteTxnLink(l.ID); err != nil {
				return fmt.Errorf("appstate: unmap event transaction: %w", err)
			}
		}
	}
	if err := a.del("event", eventID, a.store.DeleteEvent); err != nil {
		return err
	}
	a.SyncTxnLinkNetting()
	return nil
}

// EventMembers returns the set of transaction ids mapped to the given event.
func (a *App) EventMembers(eventID string) map[string]bool {
	return events.Members(a.TxnLinks(), eventID)
}

// MapTxnToEvent creates an event-member link mapping one transaction to an event.
// It is a no-op (returns nil) when the transaction is already in the event.
func (a *App) MapTxnToEvent(txnID, eventID string) error {
	if a.EventMembers(eventID)[txnID] {
		return nil
	}
	return a.PutTxnLink(domain.TxnLink{
		Kind:    domain.TxnLinkEventTxn,
		TxnIDs:  []string{txnID},
		EventID: eventID,
	})
}

// UnmapTxnFromEvent removes the event-member link that maps the given transaction
// to the given event, if any. The transaction is never deleted.
func (a *App) UnmapTxnFromEvent(txnID, eventID string) error {
	for _, l := range a.TxnLinks() {
		if l.Kind == domain.TxnLinkEventTxn && l.EventID == eventID && l.HasTxn(txnID) {
			return a.del("event link", l.ID, a.store.DeleteTxnLink)
		}
	}
	return nil
}

// AutoAssociateEvent maps every non-transfer transaction whose date falls inside
// the event's range to the event (TX10 create-time association), skipping any
// already mapped, and returns how many links it created. The user opts a
// transaction out afterward by unmapping it.
func (a *App) AutoAssociateEvent(eventID string) (int, error) {
	ev, ok := a.GetEvent(eventID)
	if !ok {
		return 0, fmt.Errorf("appstate: event %q does not exist", eventID)
	}
	already := a.EventMembers(eventID)
	tagged := 0
	for _, tid := range events.AutoAssociate(ev, a.Transactions()) {
		if already[tid] {
			continue
		}
		if err := a.PutTxnLink(domain.TxnLink{
			Kind:    domain.TxnLinkEventTxn,
			TxnIDs:  []string{tid},
			EventID: eventID,
		}); err != nil {
			return tagged, err
		}
		tagged++
	}
	return tagged, nil
}

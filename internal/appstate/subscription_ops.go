// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

// IgnoredSubscriptions returns every persisted subscription-ignore record.
func (a *App) IgnoredSubscriptions() []domain.SubscriptionIgnore {
	v, err := a.store.ListSubscriptionIgnores()
	a.logErr("subscription ignores", err)
	return v
}

// IgnoreSubscription records that the subscription identified by subName should
// be suppressed from the detected list ("not a subscription"). If an ignore
// record already exists for this name it is updated in place (dedupe by SubName,
// case-insensitive). subName must not be empty.
func (a *App) IgnoreSubscription(subName string) error {
	subName = strings.TrimSpace(subName)
	if subName == "" {
		return fmt.Errorf("appstate: subscription name is required")
	}
	// Dedupe: look for an existing record with the same SubName.
	existing, err := a.store.ListSubscriptionIgnores()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, ig := range existing {
		if strings.EqualFold(strings.TrimSpace(ig.SubName), subName) {
			ig.IgnoredOn = now
			if err := a.store.PutSubscriptionIgnore(ig); err != nil {
				return err
			}
			a.log.Info("subscription ignore updated", "subName", subName)
			return nil
		}
	}
	ig := domain.SubscriptionIgnore{
		ID:        id.New(),
		SubName:   subName,
		IgnoredOn: now,
	}
	if err := a.store.PutSubscriptionIgnore(ig); err != nil {
		return err
	}
	a.log.Info("subscription marked ignored", "subName", subName)
	return nil
}

// UnignoreSubscription removes the ignore record for subName so the subscription
// reappears in the detected list. It is a no-op (and returns nil) if no record
// exists for that name.
func (a *App) UnignoreSubscription(subName string) error {
	subName = strings.TrimSpace(subName)
	existing, err := a.store.ListSubscriptionIgnores()
	if err != nil {
		return err
	}
	for _, ig := range existing {
		if strings.EqualFold(strings.TrimSpace(ig.SubName), subName) {
			if _, err := a.store.DeleteSubscriptionIgnore(ig.ID); err != nil {
				return err
			}
			a.log.Info("subscription ignore removed", "subName", subName)
			return nil
		}
	}
	return nil
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
)

const lastSeenVersionKey = "cashflux:last-seen-version"

// whatsNewToastOnBoot (C326) surfaces a calm one-line "what's new" notice the first
// time the app boots on a newer version than the user last saw, pointing them at the
// Help center's What's-new card for details. It is deliberately quiet:
//   - first ever run (no stored version) only records the version — a brand-new user
//     has no "what's new" to catch up on, so no toast.
//   - same version → nothing.
//   - newer version → one dismissible notice, then the seen-version is advanced so it
//     won't repeat on the next reload (idempotent).
func whatsNewToastOnBoot() {
	current := version.Version
	if current == "" {
		return
	}
	seen := lsGet(lastSeenVersionKey)
	// Record and return on first run (or if the store couldn't be read): no toast.
	if seen == "" {
		lsSet(lastSeenVersionKey, current)
		return
	}
	if seen == current {
		return
	}
	// Genuine upgrade: greet once, then remember this version. The post is deferred
	// because the toast surface captures its atom on first render — at the synchronous
	// boot point PostNotice would be a no-op. A short delay lets the first paint land
	// (and keeps the greeting from racing the boot notify-catch-up toasts).
	lsSet(lastSeenVersionKey, current)
	go func() {
		time.Sleep(1200 * time.Millisecond)
		uistate.PostNotice(uistate.T("whatsnew.updated", version.Label()), false)
	}()
}

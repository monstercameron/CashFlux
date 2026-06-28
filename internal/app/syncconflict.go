// SPDX-License-Identifier: MIT

//go:build js && wasm

// syncconflict.go — C309 sync-conflict resolve modal (#464).
//
// When a backend push is rejected (LWW: the server snapshot is newer), the local
// edit is stashed to syncConflictPrefix and the SyncChip turns amber. Clicking
// the chip opens this modal so the user can choose:
//
//   - Keep my changes  — re-push the stashed local dataset with force=true so
//     the server accepts it, overwriting the server copy. Calls
//     resolveConflictKeepLocal (sync_client.go).
//
//   - Use server version — pull the current server snapshot, apply it locally,
//     and discard the stash only after a successful import. Calls
//     resolveConflictUseServer (sync_client.go).
//
// Mount pattern mirrors ProfileSwitchHost: a singleton component, all hooks
// declared unconditionally before any early return, opened via a captured atom
// (syncConflictHandle). Add uic.CreateElement(SyncConflictHost) to shell.go.
package app

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// syncConflictHandle is captured by SyncConflictHost on each mount so
// openSyncConflict can open the modal from outside the component (e.g. from
// SyncChip's onClick handler). Nil before the first mount — no-op in that
// window, mirroring the psHandle pattern in profileswitch.go.
var syncConflictHandle interface {
	Get() bool
	Set(bool)
}

// openSyncConflict opens the sync-conflict resolve modal.
// Safe to call before the host is mounted (no-op in that case).
func openSyncConflict() {
	if syncConflictHandle != nil {
		syncConflictHandle.Set(true)
	}
}

// SyncConflictHost is the singleton modal for resolving a sync conflict.
// Mount once in Shell alongside SettingsHost / ProfileSwitchHost so its hook
// depth is always constant regardless of which screen is active.
//
// All hooks are declared unconditionally before any early return so the hook
// call order never changes between open and closed renders (GWC rule).
func SyncConflictHost() uic.Node {
	// ── hooks — always called, always in this order ──────────────────────────

	// hook 1: is the modal open?
	open := state.UseAtom("sync:conflict:open", false)
	syncConflictHandle = open // capture for openSyncConflict()

	// hook 2: keep-local — force-push the stashed dataset.
	onKeepLocal := uic.UseEvent(func() {
		open.Set(false)
		resolveConflictKeepLocal()
	})

	// hook 3: use-server — pull server snapshot, discard stash after success.
	onUseServer := uic.UseEvent(func() {
		open.Set(false)
		resolveConflictUseServer()
	})

	// hook 4: close / cancel without resolving.
	onClose := uic.UseEvent(func() {
		open.Set(false)
	})

	// ── stable anchor when closed ────────────────────────────────────────────
	if !open.Get() {
		return Div(css.Class("cf-sc-root"))
	}

	// ── modal ─────────────────────────────────────────────────────────────────
	return Div(css.Class("cf-sc-root"),
		Div(css.Class("cf-dialog-backdrop"),
			Attr("role", "dialog"),
			Attr("aria-modal", "true"),
			Attr("aria-label", uistate.T("syncConflict.title")),
			Div(css.Class("cf-dialog-scrim"), OnClick(onClose)),
			Div(css.Class("cf-ps-panel"),
				H3(css.Class("cf-ps-title"), uistate.T("syncConflict.title")),
				P(css.Class("muted"), uistate.T("syncConflict.description")),

				// Option A: Keep local changes (force push)
				Div(css.Class("sync-conflict-option"),
					B(uistate.T("syncConflict.keepLocalTitle")),
					P(css.Class("muted"), uistate.T("syncConflict.keepLocalDesc")),
					Button(css.Class("btn btn-primary"), Type("button"),
						Attr("data-testid", "sync-conflict-keep-local"),
						OnClick(onKeepLocal),
						uistate.T("syncConflict.keepLocalBtn"),
					),
				),

				// Option B: Use server version (pull + discard stash)
				Div(css.Class("sync-conflict-option"),
					B(uistate.T("syncConflict.useServerTitle")),
					P(css.Class("muted"), uistate.T("syncConflict.useServerDesc")),
					Button(css.Class("btn"), Type("button"),
						Attr("data-testid", "sync-conflict-use-server"),
						OnClick(onUseServer),
						uistate.T("syncConflict.useServerBtn"),
					),
				),

				Div(css.Class("cf-ps-actions"),
					Button(css.Class("btn"), Type("button"), OnClick(onClose),
						uistate.T("syncConflict.close")),
				),
			),
		),
	)
}

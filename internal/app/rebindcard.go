// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/datasetmerge"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// RebindCard is what the Cloud pane shows when the signed-in account does not
// own the workspace this device is holding (TODOS.md C694, C697).
//
// What it replaced was one line: "1 change(s) waiting to upload — Reason:
// workspace not found". Everything a person needs in order to act was missing
// from it — which account they are signed in as, which server, which workspace,
// when this last worked, and what they are allowed to do about it. So the only
// available response was to wait, and waiting is the one thing that cannot
// work: the server is refusing the workspace because it belongs to somebody
// else, and it will go on refusing it for ever.
//
// The card states the facts and offers the four real options: point this data
// at a workspace the current account owns, sign in as the account that owns it,
// keep this device local-only, or take a backup first. Nothing is uploaded
// until a mapping is confirmed, and the local copy is exported before it is.

// rebindProps carries the live connection facts in from the Cloud pane, which
// already holds them, so this component does no prefs reading of its own.
type rebindProps struct {
	ServerURL string
	Status    syncStatus
	// OnSignOut lets the user go and sign in as the other account, reusing the
	// pane's existing session teardown rather than a second implementation.
	OnSignOut func()
	// OnDisconnect turns sync off for this device (keep local only).
	OnDisconnect func()
	// OnRetry re-runs the flush, for the case where the operator has fixed the
	// mapping on the server side and the client just needs to try again.
	OnRetry func()
}

// remoteWorkspaceChoice is one workspace the signed-in account can actually
// address — the candidates a rebind may target.
type remoteWorkspaceChoice struct {
	ID        string
	Name      string
	UpdatedAt string
}

// RebindCard renders the recovery surface. It renders nothing at all unless the
// client is in the blocked state, so the ordinary Cloud pane is unchanged.
func RebindCard(p rebindProps) uic.Node {
	choices := uic.UseState[[]remoteWorkspaceChoice](nil)
	loading := uic.UseState(false)
	picking := uic.UseState(false)
	selected := uic.UseState("")
	message := uic.UseState("")
	busy := uic.UseState(false)

	blocked := p.Status.State == syncStateRebind
	// The workspace the DECISION is about, which is not always the one on
	// screen: the stranded entry can belong to another workspace entirely.
	// Rebinding the active one instead would repoint a workspace that was never
	// broken and leave the stuck entry exactly where it was.
	localWS := blockedWorkspaceID()
	if localWS == "" {
		localWS = activeWorkspaceID()
	}
	userID := signedInUserID()

	loadChoices := uic.UseEvent(func() {
		picking.Set(true)
		loading.Set(true)
		message.Set("")
		go func() {
			list, err := listRemoteWorkspaces()
			loading.Set(false)
			if err != nil {
				message.Set(uistate.T("sync.rebindFailed", err.Error()))
				return
			}
			choices.Set(list)
		}()
	})
	cancelPick := uic.UseEvent(func() {
		picking.Set(false)
		selected.Set("")
	})
	onPick := uic.UseEvent(func(v string) { selected.Set(v) })
	exportFirst := uic.UseEvent(func() {
		exportWorkspace(activeWorkspaceID())
		message.Set(uistate.T("sync.rebindBackupSaved"))
	})
	// C695: once a target is chosen and BOTH copies hold records, the user still
	// has to say what happens to the server's copy. The choice is only raised
	// when there is genuinely something on the other side — inventing the
	// dilemma when the target is empty would be noise.
	compare := uic.UseState[*remoteComparison](nil)
	comparing := uic.UseState(false)
	winner := uic.UseState("local")
	onWinner := uic.UseEvent(func(v string) { winner.Set(v) })
	policyOf := func() datasetmerge.Policy {
		if winner.Get() == "remote" {
			return datasetmerge.PreferTarget // the server's copy is the merge target
		}
		return datasetmerge.PreferSource
	}
	doReplace := uic.UseEvent(func() {
		c := compare.Get()
		if c == nil || busy.Get() {
			return
		}
		busy.Set(true)
		message.Set("")
		go func() {
			defer busy.Set(false)
			exportWorkspace(activeWorkspaceID())
			if err := replaceRemoteWithLocal(c.WorkspaceID); err != nil {
				message.Set(uistate.T("sync.mergeFailed", err.Error()))
				return
			}
			compare.Set(nil)
			message.Set(uistate.T("sync.mergeReplaced"))
		}()
	})
	doMerge := uic.UseEvent(func() {
		c := compare.Get()
		if c == nil || busy.Get() {
			return
		}
		busy.Set(true)
		message.Set("")
		go func() {
			defer busy.Set(false)
			exportWorkspace(activeWorkspaceID())
			report, err := mergeWithRemote(c.WorkspaceID, policyOf())
			if err != nil {
				message.Set(uistate.T("sync.mergeFailed", err.Error()))
				return
			}
			compare.Set(nil)
			message.Set(uistate.T("sync.mergeDone", report.TotalAdded, report.Conflicts))
		}()
	})
	onSignOutClick := uic.UseEvent(p.OnSignOut)
	onDisconnectClick := uic.UseEvent(p.OnDisconnect)
	onRetryClick := uic.UseEvent(p.OnRetry)
	confirm := uic.UseEvent(func() {
		target := strings.TrimSpace(selected.Get())
		if target == "" || busy.Get() {
			return
		}
		busy.Set(true)
		message.Set("")
		go func() {
			defer busy.Set(false)
			// The local copy is exported BEFORE anything is repointed. A rebind
			// is the moment a person is least sure which copy is which, and an
			// export costs one file.
			exportWorkspace(localWS)
			if err := rebindLocalWorkspace(localWS, target, userID); err != nil {
				message.Set(uistate.T("sync.rebindFailed", err.Error()))
				return
			}
			name := target
			for _, c := range choices.Get() {
				if c.ID == target {
					name = c.Name
					break
				}
			}
			message.Set(uistate.T("sync.rebindDone", name))
			picking.Set(false)
			// Look at what is already on the other side before pushing. If that
			// workspace also holds records, the user gets the replace/merge
			// choice; if it is empty, the ordinary flush is all that is needed.
			comparing.Set(true)
			cmp, cmpErr := compareWithRemote(target, policyOf())
			comparing.Set(false)
			if cmpErr == nil && cmp.RemoteFound {
				compare.Set(&cmp)
				return
			}
			p.OnRetry()
		}()
	})

	// Every hook above this point runs unconditionally. The framework matches
	// hooks positionally, so an early return placed before a UseEvent would
	// shift the whole list on the render where the card is hidden and corrupt
	// the ones that remain.
	if !blocked {
		return Fragment()
	}

	who := strings.TrimSpace(userID)
	if who == "" {
		who = uistate.T("sync.rebindUnknownUser")
	}
	last := strings.TrimSpace(p.Status.LastSyncedAt)
	if last == "" {
		last = uistate.T("sync.rebindNeverSynced")
	} else if t, err := time.Parse(time.RFC3339Nano, last); err == nil {
		last = t.Local().Format("2006-01-02 15:04")
	}

	var picker uic.Node = Fragment()
	if picking.Get() {
		var options []uic.Node
		options = append(options, Option(Value(""), uistate.T("sync.rebindAction")))
		for _, c := range choices.Get() {
			label := c.Name
			if strings.TrimSpace(label) == "" {
				label = c.ID
			}
			if c.UpdatedAt != "" {
				label += " · " + c.UpdatedAt
			}
			options = append(options, Option(Value(c.ID), SelectedIf(c.ID == selected.Get()), label))
		}
		var body uic.Node
		switch {
		case loading.Get():
			body = P(css.Class("muted"), uistate.T("sync.syncingNow"))
		case len(choices.Get()) == 0:
			body = P(css.Class("muted"), Attr("data-testid", "rebind-remote-empty"), uistate.T("sync.rebindRemoteEmpty"))
		default:
			body = Fragment(
				P(css.Class("muted"), uistate.T("sync.rebindPickPrompt")),
				Select(css.Class("field"), Attr("data-testid", "rebind-target"),
					Attr("aria-label", uistate.T("sync.rebindRemoteTitle")), OnChange(onPick), options),
			)
		}
		picker = Div(css.Class("rebind-picker"), Attr("data-testid", "rebind-picker"),
			H4(uistate.T("sync.rebindRemoteTitle")),
			body,
			Div(css.Class("row-actions"),
				Button(css.Class("btn", "btn-primary"), Type("button"),
					Attr("data-testid", "rebind-confirm"),
					Disabled(busy.Get() || strings.TrimSpace(selected.Get()) == ""),
					OnClick(confirm), uistate.T("sync.rebindConfirm")),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-cancel"),
					OnClick(cancelPick), uistate.T("sync.rebindCancel")),
			),
		)
	}

	return Div(css.Class("rebind-card"), Attr("data-testid", "sync-rebind-card"),
		Attr("role", "region"), Attr("aria-label", uistate.T("sync.rebindTitle")),
		H3(uistate.T("sync.rebindTitle")),
		P(uistate.T("sync.rebindReason")),
		P(css.Class("muted"), uistate.T("sync.rebindExplain")),
		// The facts. Each line is here because its absence made the old message
		// unactionable: you cannot decide which account should own this data
		// without knowing which account you are.
		Div(css.Class("rebind-facts"),
			rebindFact("sync.rebindSignedInAs", who, "rebind-identity"),
			rebindFact("sync.rebindServer", p.ServerURL, "rebind-server"),
			rebindFact("sync.rebindWorkspace", localWS, "rebind-workspace"),
			rebindFact("sync.rebindLastSuccess", last, "rebind-lastsync"),
		),
		If(p.Status.Pending > 0, P(css.Class("muted"), Attr("data-testid", "rebind-pending"),
			uistate.T("sync.pendingCount", p.Status.Pending))),
		If(strings.TrimSpace(p.Status.Message) != "", P(css.Class("muted"),
			uistate.T("sync.statusDetail", p.Status.Message))),
		Div(css.Class("row-actions"),
			Button(css.Class("btn", "btn-primary"), Type("button"), Attr("data-testid", "rebind-start"),
				OnClick(loadChoices), uistate.T("sync.rebindAction")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-export"),
				OnClick(exportFirst), uistate.T("sync.rebindExport")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-signin"),
				OnClick(onSignOutClick), uistate.T("sync.rebindSignIn")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-keeplocal"),
				Title(uistate.T("sync.rebindKeepLocalHint")),
				OnClick(onDisconnectClick), uistate.T("sync.rebindKeepLocal")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-retry"),
				OnClick(onRetryClick), uistate.T("sync.rebindRetry")),
		),
		picker,
		mergeChoicePanel(mergeChoiceProps{
			comparing: comparing.Get(),
			compare:   compare.Get(),
			winner:    winner.Get(),
			busy:      busy.Get(),
			onWinner:  onWinner,
			onReplace: doReplace,
			onMerge:   doMerge,
		}),
		If(strings.TrimSpace(message.Get()) != "", P(css.Class("muted"), Attr("role", "status"),
			Attr("data-testid", "rebind-message"), message.Get())),
	)
}

// rebindFact renders one labelled fact line.
func rebindFact(key, value, testID string) uic.Node {
	if strings.TrimSpace(value) == "" {
		value = "—"
	}
	return Div(css.Class("rebind-fact"),
		Span(css.Class("rebind-fact-label"), uistate.T(key)),
		Span(css.Class("rebind-fact-value"), Attr("data-testid", testID), value),
	)
}

// listRemoteWorkspaces asks the server which workspaces the signed-in account
// can actually address. This is the list a rebind chooses from, and it comes
// from the server rather than from local state precisely because local state is
// the thing currently believed to be wrong.
func listRemoteWorkspaces() ([]remoteWorkspaceChoice, error) {
	pr := uistate.LoadPrefs().Normalize()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var resp backendrpc.ListWorkspacesResponse
	if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &resp); err != nil {
		return nil, err
	}
	out := make([]remoteWorkspaceChoice, 0, len(resp.Workspaces))
	for _, w := range resp.Workspaces {
		if w.Deleted {
			continue
		}
		updated := ""
		if t, err := time.Parse(time.RFC3339Nano, w.UpdatedAt); err == nil {
			updated = t.Local().Format("2006-01-02")
		}
		out = append(out, remoteWorkspaceChoice{ID: w.ID, Name: w.Name, UpdatedAt: updated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

// rebindLocalWorkspace repoints this device's local workspace at a workspace the
// signed-in account owns.
//
// The local workspace keeps its data and its place in the registry; what changes
// is the ID the sync layer addresses it by, plus the ownership stamped on
// anything queued against the old id. Sync metadata for the old id is dropped
// rather than carried over: it describes a conversation with the server about a
// workspace this account was never party to, and keeping it would let a stale
// version number decide the first exchange with the new one.
func rebindLocalWorkspace(fromWorkspaceID, toWorkspaceID, userID string) error {
	from := strings.TrimSpace(fromWorkspaceID)
	to := strings.TrimSpace(toWorkspaceID)
	if from == "" || to == "" {
		return errRebindIncomplete
	}
	if from == to {
		return nil
	}
	if err := renameWorkspaceID(from, to); err != nil {
		return err
	}
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	byKey := map[string]queuedSyncMutation{}
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{UserID: q.UserID, WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
		byKey[q.UserID+"\x00"+q.WorkspaceID+"\x00"+q.Hash] = q
	}
	for _, owner := range []string{userID, ""} {
		pending = syncstate.Rebind(pending,
			syncstate.Binding{UserID: owner, WorkspaceID: from},
			syncstate.Binding{UserID: userID, WorkspaceID: to})
	}
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		q, ok := byKey[p.UserID+"\x00"+from+"\x00"+p.Hash]
		if !ok {
			q, ok = byKey[""+"\x00"+from+"\x00"+p.Hash]
		}
		if !ok {
			q, ok = byKey[p.UserID+"\x00"+p.WorkspaceID+"\x00"+p.Hash]
		}
		if !ok {
			continue
		}
		q.UserID = p.UserID
		q.WorkspaceID = p.WorkspaceID
		next = append(next, q)
	}
	saveSyncQueue(next)
	carryConflictBackup(from, to)
	clearSyncMetaFor(from)
	setSyncStatus(syncStatus{State: "syncing", Pending: len(next)})
	return nil
}

// mergeChoiceProps drives the replace-or-merge decision panel.
type mergeChoiceProps struct {
	comparing bool
	compare   *remoteComparison
	winner    string
	busy      bool
	onWinner  uic.Handler
	onReplace uic.Handler
	onMerge   uic.Handler
}

// mergeChoicePanel asks what should happen to the server's copy, and states the
// consequences in RECORDS. Bytes are shown as a footnote: "1.2 MB replacing 900
// KB" tells nobody whether they are about to lose a month of transactions.
func mergeChoicePanel(p mergeChoiceProps) uic.Node {
	if p.comparing {
		return P(css.Class("muted"), Attr("data-testid", "rebind-comparing"), uistate.T("sync.mergeChecking"))
	}
	c := p.compare
	if c == nil {
		return Fragment()
	}
	var overlap uic.Node = P(css.Class("muted"), uistate.T("sync.mergeNoOverlap"))
	if c.Merge.TotalAdded > 0 || c.Merge.Conflicts > 0 {
		overlap = Fragment(
			P(css.Class("muted"), Attr("data-testid", "rebind-merge-adds"),
				uistate.T("sync.mergeWouldAdd", c.Merge.TotalAdded)),
			If(c.Merge.Conflicts > 0, P(css.Class("muted"), Attr("data-testid", "rebind-merge-conflicts"),
				uistate.T("sync.mergeWouldConflict", c.Merge.Conflicts))),
		)
	}
	return Div(css.Class("rebind-picker"), Attr("data-testid", "rebind-merge-choice"),
		H4(uistate.T("sync.mergeChoiceTitle")),
		P(uistate.T("sync.mergeChoiceIntro")),
		overlap,
		P(css.Class("muted"), uistate.T("sync.mergeCompare",
			formatByteSize(c.LocalBytes), formatByteSize(c.RemoteBytes))),
		If(c.Merge.Conflicts > 0, Div(css.Class("rebind-facts"),
			Span(css.Class("rebind-fact-label"), uistate.T("sync.mergeConflictWins")),
			Select(css.Class("field"), Attr("data-testid", "rebind-merge-winner"),
				Attr("aria-label", uistate.T("sync.mergeConflictWins")), OnChange(p.onWinner),
				Option(Value("local"), SelectedIf(p.winner == "local"), uistate.T("sync.mergeWinsLocal")),
				Option(Value("remote"), SelectedIf(p.winner == "remote"), uistate.T("sync.mergeWinsRemote")),
			),
		)),
		Div(css.Class("row-actions"),
			Button(css.Class("btn", "btn-primary"), Type("button"), Attr("data-testid", "rebind-merge-keep-both"),
				Disabled(p.busy), OnClick(p.onMerge), uistate.T("sync.mergeKeepBoth")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "rebind-merge-replace"),
				Title(uistate.T("sync.mergeReplaceHint")),
				Disabled(p.busy), OnClick(p.onReplace), uistate.T("sync.mergeReplace")),
		),
		P(css.Class("muted"), uistate.T("sync.mergeReplaceHint")),
	)
}

// formatByteSize renders a byte count for a human, at one decimal place.
func formatByteSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

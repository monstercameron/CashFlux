// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — changeset_card.go is the reusable UI for AG1 (changeset
// review) and AG20 (session receipt). It renders an agent's multi-step proposal
// as ONE reviewable card — per-item toggles + "Apply all (N)" — and, after apply,
// a receipt with one-tap "Undo all" (reusing the session undo stack via
// auditview.UndoFunc). It also renders the per-conversation cumulative receipt
// ("this chat: 3 transactions categorized …").
//
// The card is data-driven and self-contained: other AG features raise a proposal
// with uistate.SetPendingChangeset and mount PendingChangesetHost — no coupling
// to the chat tool loop. All apply/undo/aggregation logic lives in the tested
// internal/changeset, internal/appstate, and internal/agentreceipt packages.
package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/changeset"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PendingChangesetHost renders the current agent proposal (if any) as a review
// card, or Fragment() when nothing is pending. Mount it once in the assistant
// host (the coordinator does this); it reads the shared pending-changeset atom.
func PendingChangesetHost() ui.Node {
	pending := uistate.UsePendingChangeset().Get()
	if pending.Set == nil || pending.Set.IsEmpty() {
		return Fragment()
	}
	// Key the card by the proposal Nonce so a new proposal remounts it with fresh
	// local state (toggles, receipt).
	return Fragment(MapKeyed([]uistate.PendingChangeset{pending},
		func(p uistate.PendingChangeset) any { return p.Nonce },
		func(p uistate.PendingChangeset) ui.Node {
			return ui.CreateElement(changesetReviewCard, changesetReviewProps{
				ConversationID: p.ConversationID,
				Set:            p.Set,
			})
		},
	))
}

type changesetReviewProps struct {
	ConversationID string
	Set            *changeset.Changeset
}

// changesetReviewCard is the AG1 review card: a titled list of proposed steps,
// each with an include toggle, an "Apply all (N)" button, and — after apply — a
// receipt with "Undo all". Its own component so its hooks sit at stable
// positions.
func changesetReviewCard(props changesetReviewProps) ui.Node {
	cs := props.Set
	rev := ui.UseState(0)                           // bumped on a toggle to re-render
	receipt := ui.UseState[*changeset.Receipt](nil) // set once applied
	dismissed := ui.UseState(false)
	_ = rev.Get()

	if dismissed.Get() {
		return Fragment()
	}

	// After apply: show the receipt view.
	if r := receipt.Get(); r != nil {
		return changesetReceiptView(cs, *r, dismissed)
	}

	onToggle := func(i int, on bool) {
		cs.SetEnabled(i, on)
		rev.Set(rev.Get() + 1)
	}
	onApply := ui.UseEvent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		rec := app.ApplyChangeset(*cs)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.AddAgentActions(props.ConversationID, rec.Kinds())
		receipt.Set(&rec)
		if rec.OK() {
			// C364: the completion toast tells the reversal story at the moment of
			// risk — Ctrl+Z reverses the last step, and the persistent receipt below
			// carries "Undo all" + "View in Activity" (the audit timeline) for the
			// full picture.
			uistate.PostNotice(uistate.T("changeset.appliedUndo", rec.AppliedCount()), false)
		} else {
			uistate.PostNotice(uistate.T("changeset.failed", rec.Failed.Line, rec.Failed.Err), true)
		}
	})
	onDismiss := ui.UseEvent(func() {
		dismissed.Set(true)
		uistate.ClearPendingChangeset()
	})

	enabled := cs.EnabledCount()
	rows := make([]ui.Node, 0, cs.Len())
	for i, op := range cs.Ops {
		rows = append(rows, ui.CreateElement(changesetOpRow, changesetOpRowProps{
			Index: i, Line: op.Line, Enabled: op.Enabled, OnToggle: onToggle,
		}))
	}

	applyLabel := uistate.T("changeset.applyAll", enabled)
	if enabled == 0 {
		applyLabel = uistate.T("changeset.applyNone")
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "group"),
		Attr("data-testid", "changeset-card"),
		Attr("aria-label", uistate.T("changeset.title")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol, tw.Gap2),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "📝"),
				Div(css.Class("catchup-card-text"),
					Strong(ifStr(cs.Title != "", cs.Title, uistate.T("changeset.title"))),
					P(css.Class("t-caption", tw.TextDim), uistate.T("changeset.subtitle")),
				),
			),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), rows),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				buttonWithDisabled(enabled == 0,
					[]any{css.Class("btn btn-primary btn-sm"), Type("button"),
						Attr("data-testid", "changeset-apply"), OnClick(onApply)},
					applyLabel),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "changeset-dismiss"),
					Attr("aria-label", uistate.T("changeset.dismissAria")),
					OnClick(onDismiss), uistate.T("changeset.dismiss")),
			),
		),
	)
}

type changesetOpRowProps struct {
	Index    int
	Line     string
	Enabled  bool
	OnToggle func(int, bool)
}

// changesetOpRow renders one proposed step with an include checkbox. Its own
// component so the checkbox's change hook is at a stable position (never inside a
// variable-length loop).
func changesetOpRow(props changesetOpRowProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { props.OnToggle(props.Index, e.IsChecked()) })
	return Label(css.Class("checkbox-label", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", fmt.Sprintf("changeset-op-%d", props.Index)),
		Input(Type("checkbox"),
			Attr("aria-label", uistate.T("changeset.itemAria")),
			CheckedIf(props.Enabled), OnChange(onChange)),
		changesetOpLabel(props.Enabled, props.Line),
	)
}

// changesetOpLabel renders a step's description, dimmed when the step is
// disabled (excluded from apply).
func changesetOpLabel(enabled bool, line string) ui.Node {
	if enabled {
		return Span(line)
	}
	return Span(css.Class(tw.TextDim), line)
}

// changesetReceiptView renders the post-apply receipt: what applied, the first
// failure (if any), and one-tap "Undo all" over the session undo stack.
func changesetReceiptView(cs *changeset.Changeset, r changeset.Receipt, dismissed ui.State[bool]) ui.Node {
	nav := router.UseNavigate()
	// C364: "View in Activity" jumps to the audit timeline where every applied step
	// is recorded and individually reversible — the durable companion to Ctrl+Z.
	viewActivity := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/activity")) }))
	onUndoAll := ui.UseEvent(func() {
		// The apply captured one undo point per applied op (auditview.CaptureNow
		// inside ApplyChangeset), so undo that many times to reverse them all.
		for i := 0; i < r.AppliedCount(); i++ {
			if !auditview.UndoFunc() {
				break
			}
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("changeset.undone"), false)
		dismissed.Set(true)
		uistate.ClearPendingChangeset()
	})
	onClose := ui.UseEvent(func() {
		dismissed.Set(true)
		uistate.ClearPendingChangeset()
	})

	title := uistate.T("changeset.receiptTitle", r.AppliedCount(), cs.Len())

	lines := make([]ui.Node, 0, len(r.Applied)+1)
	for _, ap := range r.Applied {
		lines = append(lines, Div(css.Class("t-caption", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span("✓"), Span(ap.Result)))
	}
	if r.Failed != nil {
		lines = append(lines, P(css.Class("t-caption", tw.TextDim), Attr("role", "alert"),
			uistate.T("changeset.failed", r.Failed.Line, r.Failed.Err)))
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "status"),
		Attr("data-testid", "changeset-receipt"),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol, tw.Gap2),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), ifStr(r.OK(), "✅", "⚠️")),
				Strong(title),
			),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), lines),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				If(r.AppliedCount() > 0,
					Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
						Attr("data-testid", "changeset-undo-all"),
						Attr("aria-label", uistate.T("changeset.undoAllAria")),
						OnClick(onUndoAll), uistate.T("changeset.undoAll")),
				),
				If(r.AppliedCount() > 0,
					Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
						Attr("data-testid", "changeset-view-activity"),
						Attr("aria-label", uistate.T("changeset.viewActivity")),
						OnClick(viewActivity), uistate.T("changeset.viewActivity")),
				),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "changeset-receipt-close"),
					OnClick(onClose), uistate.T("action.close")),
			),
		),
	)
}

// AgentSessionReceipt renders the AG20 cumulative "this chat: …" receipt for a
// conversation, or Fragment() when the agent has made no changes. Mount it in the
// assistant host below the thread (the coordinator does this).
func AgentSessionReceipt(conversationID string) ui.Node {
	_ = uistate.UseAgentTallyRevision().Get()
	if uistate.AgentReceiptActionCount(conversationID) == 0 {
		return Fragment()
	}
	summary := uistate.AgentReceiptSummary(conversationID)
	if summary == "" {
		return Fragment()
	}
	return Div(css.Class("t-caption", tw.TextDim, tw.Flex, tw.ItemsCenter, tw.Gap1),
		Attr("role", "status"),
		Attr("data-testid", "agent-session-receipt"),
		Attr("aria-label", uistate.T("changeset.sessionAria")),
		Span("🧾"), Span(summary),
	)
}

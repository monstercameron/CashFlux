// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// pendingPhase tracks pendingDeviceWatcher's state machine.
type pendingPhase string

const (
	pendingPhaseConnecting  pendingPhase = "connecting"
	pendingPhaseWaiting     pendingPhase = "waiting"
	pendingPhaseApproved    pendingPhase = "approved"
	pendingPhaseSettingPass pendingPhase = "settingPassword"
	pendingPhaseRecovery    pendingPhase = "recovery"
	pendingPhaseDone        pendingPhase = "done"
	pendingPhaseRejected    pendingPhase = "rejected"
	pendingPhaseCanceled    pendingPhase = "canceled"
	pendingPhaseExpired     pendingPhase = "expired"
	pendingPhaseError       pendingPhase = "error"
)

// PendingDeviceCard offers the admin-approved first-account bootstrap
// (TODOS.md C454/C473) when Register is disabled. It can be the primary card or
// a secondary disclosure, but in either case the actual request fires only
// after the user clicks Request access. Merely visiting Settings never creates
// a server-side request.
type PendingDeviceCardProps struct {
	Primary bool
}

func PendingDeviceCard(props PendingDeviceCardProps) uic.Node {
	expanded := uic.UseState(false)
	onToggleExpand := uic.UseEvent(func() { expanded.Set(!expanded.Get()) })

	if !expanded.Get() && props.Primary {
		return Div(css.Class("card", "pending-device-card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2),
			Attr("data-testid", "pending-device-request-card"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				ui.Icon(icon.Clock, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
				Span(css.Class(tw.Text15, tw.FontSemibold), uistate.T("authCards.pendingTitle")),
			),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.pendingIntro")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "pending-device-request"),
				OnClick(onToggleExpand), uistate.T("authCards.pendingRequest")),
		)
	}
	if !expanded.Get() {
		return Div(Attr("data-testid", "pending-device-collapsed"),
			Button(css.Class("btn-link", tw.Text12, tw.TextDim), Type("button"), Attr("data-testid", "pending-device-expand"),
				OnClick(onToggleExpand), uistate.T("authCards.waitForApproval")),
		)
	}
	return uic.CreateElement(pendingDeviceWatcher)
}

// pendingDeviceWatcher owns the whole request→watch→accept/reject→set-
// password flow. Mounting it (i.e. PendingDeviceCard being expanded) is what
// triggers RequestDevicePairing — one shot per mount, no retry loop: if it
// errors, expires, or the user cancels, collapsing and re-expanding
// PendingDeviceCard (which remounts this component fresh) is how to try
// again, matching the "one-shot per app load" design decision.
func pendingDeviceWatcher() uic.Node {
	prefsAtom := uistate.UsePrefs()

	phase := uic.UseState(string(pendingPhaseConnecting))
	deviceID := uic.UseState("")
	pairingCode := uic.UseState("")
	errMsg := uic.UseState("")

	setPwUsername := uic.UseState("")
	setPwPassword := uic.UseState("")
	setPwSubmitting := uic.UseState(false)
	setPwErr := uic.UseState("")
	setPwRecovery := uic.UseState("")
	pendingPair := uic.UseState(backendrpc.TokenPairResponse{})

	uic.UseEffect(func() func() {
		pr := prefsAtom.Get().Normalize()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// RequestDevicePairing/WatchPairingStatus are skip-listed
			// server-side (no bearer token required) — a brand-new device
			// with no session yet has no real token to present, so the
			// same placeholder trick PasswordAuthCard/DeviceLinkCard use
			// satisfies syncbridge's client-side "a token is required"
			// guard without being checked server-side.
			var reqOut backendrpc.RequestDevicePairingResponse
			reqErr := invokeWorkerRPC(ctx, pr.ServerURL, "refresh", backendrpc.MethodAuthRequestDevicePairing,
				backendrpc.RequestDevicePairingRequest{DeviceLabel: customSyncDeviceLabel()},
				&reqOut)
			if reqErr != nil {
				phase.Set(string(pendingPhaseError))
				errMsg.Set(customSyncErrorMessage(reqErr, uistate.T("authCards.pendingRequestFailed")))
				return
			}
			deviceID.Set(reqOut.DeviceID)
			phase.Set(string(pendingPhaseWaiting))

			stream, err := openWorkerRPCStream(ctx, pr.ServerURL, "refresh", backendrpc.MethodAuthWatchPairingStatus,
				backendrpc.WatchPairingStatusRequest{DeviceID: reqOut.DeviceID})
			if err != nil {
				phase.Set(string(pendingPhaseError))
				errMsg.Set(customSyncErrorMessage(err, uistate.T("authCards.pendingWatchFailed")))
				return
			}
			var ev backendrpc.PairingStatusEvent
			if err := stream.Recv(&ev); err != nil {
				if ctx.Err() != nil {
					// The component unmounted (cancel ran) — not a real
					// failure worth surfacing, just the watch ending because
					// nobody's looking at it anymore.
					return
				}
				phase.Set(string(pendingPhaseError))
				errMsg.Set(customSyncErrorMessage(err, uistate.T("authCards.pendingWatchFailed")))
				return
			}
			switch ev.Status {
			case "approved", "redeemed":
				// "redeemed" is this same request, already used. It reaches a
				// watcher that reconnected after redeeming, and it means
				// approved — not expired, which is what the old default branch
				// told the user.
				pairingCode.Set(ev.PairingCode)
				phase.Set(string(pendingPhaseApproved))
			case "rejected":
				phase.Set(string(pendingPhaseRejected))
			case "canceled":
				// The device's own withdrawal. onCancel already set this phase
				// optimistically and did NOT cancel this watch, so the server's
				// echo used to arrive here and overwrite the correct "canceled"
				// state with a danger-styled "expired" — telling the user their
				// request timed out when they had just cancelled it themselves.
				// Found by adversarial review, 2026-08-17.
				phase.Set(string(pendingPhaseCanceled))
			default: // "expired", or anything unrecognized
				phase.Set(string(pendingPhaseExpired))
			}
		}()
		return cancel
	}, "pending-device-watch-mount")

	notifyAtom := uistate.UseNotice()
	notify := func(text string, isErr bool) { notifyAtom.Set(notifyAtom.Get().With(text, isErr)) }

	onCancel := uic.UseEvent(func() {
		id := deviceID.Get()
		phase.Set(string(pendingPhaseCanceled))
		if id == "" {
			return
		}
		go func() {
			ctx := context.Background()
			var out backendrpc.CancelDevicePairingResponse
			_ = invokeWorkerRPC(ctx, prefsAtom.Get().Normalize().ServerURL, "refresh",
				backendrpc.MethodAuthCancelDevicePairing, backendrpc.CancelDevicePairingRequest{DeviceID: id}, &out)
		}()
	})

	onAccept := uic.UseEvent(func() {
		pr := prefsAtom.Get().Normalize()
		code := pairingCode.Get()
		phase.Set(string(pendingPhaseSettingPass)) // optimistic; reverted to approved on failure below
		go func() {
			ctx := context.Background()
			var out backendrpc.TokenPairResponse
			err := invokeWorkerRPC(ctx, pr.ServerURL, "refresh", backendrpc.MethodAuthRedeemPairingCode, backendrpc.RedeemPairingCodeRequest{
				PairingCode:    code,
				DeviceLabel:    customSyncDeviceLabel(),
				IdempotencyKey: newIdempotencyKey(),
			}, &out)
			if err != nil {
				phase.Set(string(pendingPhaseApproved))
				notify(customSyncErrorMessage(err, uistate.T("authCards.pendingRedeemFailed")), true)
				return
			}
			// Keep the provisional session in memory until credentials have
			// been created and the requester confirms the one-time recovery
			// code is saved. Persisting it here would allow a reload (and
			// therefore sync) before account recovery is configured.
			pendingPair.Set(out)
			phase.Set(string(pendingPhaseSettingPass))
		}()
	})

	onReject := uic.UseEvent(func() {
		id := deviceID.Get()
		phase.Set(string(pendingPhaseRejected))
		go func() {
			ctx := context.Background()
			var out backendrpc.CancelDevicePairingResponse
			_ = invokeWorkerRPC(ctx, prefsAtom.Get().Normalize().ServerURL, "refresh",
				backendrpc.MethodAuthCancelDevicePairing, backendrpc.CancelDevicePairingRequest{DeviceID: id}, &out)
		}()
	})

	onSetPwUsername := uic.UseEvent(func(v string) { setPwUsername.Set(v); setPwErr.Set("") })
	onSetPwPassword := uic.UseEvent(func(v string) { setPwPassword.Set(v); setPwErr.Set("") })
	onSubmitSetPassword := uic.UseEvent(func() {
		u := normalizeUsername(setPwUsername.Get())
		pw := setPwPassword.Get()
		if err := validateRegisterCredentials(u, pw); err != nil {
			switch err {
			case ErrUsernameRequired:
				setPwErr.Set(uistate.T("authCards.usernameRequired"))
			case ErrPasswordRequired:
				setPwErr.Set(uistate.T("authCards.passwordRequired"))
			case ErrPasswordTooShort:
				setPwErr.Set(uistate.T("authCards.passwordTooShort", authMinPasswordLength))
			}
			return
		}
		setPwSubmitting.Set(true)
		go func() {
			ctx := context.Background()
			pr := prefsAtom.Get().Normalize()
			pair := pendingPair.Get()
			var out backendrpc.SetPasswordResponse
			err := invokeWorkerRPC(ctx, pr.ServerURL, pair.AccessToken, backendrpc.MethodAuthSetPassword,
				backendrpc.SetPasswordRequest{Username: u, Password: pw}, &out)
			setPwSubmitting.Set(false)
			if err != nil {
				setPwErr.Set(customSyncErrorMessage(err, uistate.T("authCards.pendingSetPasswordFailed")))
				return
			}
			setPwPassword.Set("")
			// Setting the password revoked every session on the account, the
			// redemption pair this card is holding included. Swap in the
			// replacement the server issued, or the pair persisted when the user
			// dismisses the recovery code below would already be dead.
			if replacement, ok := sessionFromSetPassword(out); ok {
				pendingPair.Set(replacement)
			}
			setPwRecovery.Set(strings.TrimSpace(out.RecoveryCode))
			phase.Set(string(pendingPhaseRecovery))
		}()
	})
	onDismissRecovery := uic.UseEvent(func() {
		pair := pendingPair.Get()
		pr := prefsAtom.Get().Normalize()
		setPwRecovery.Set("")
		pendingPair.Set(backendrpc.TokenPairResponse{})
		persistAuthSession(prefsAtom, pr.ServerURL, pair, true)
		notify(uistate.T("authCards.pendingSetPasswordSuccess"), false)
		phase.Set(string(pendingPhaseDone))
	})

	p := pendingPhase(phase.Get())

	return Div(css.Class("card", "pending-device-card", tw.Mt1, tw.Flex, tw.FlexCol, tw.Gap2), Attr("data-testid", "pending-device-card"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			ui.Icon(icon.Clock, css.Class(tw.W5, tw.H5, tw.ShrinkO, tw.TextDim)),
			Span(css.Class(tw.Text15, tw.FontSemibold), uistate.T("authCards.pendingTitle")),
		),

		If(p == pendingPhaseConnecting, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "pending-device-connecting"), uistate.T("authCards.pendingConnecting"))),

		If(p == pendingPhaseWaiting, Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "pending-device-waiting"), uistate.T("authCards.pendingWaiting")),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.pendingWaitingHint")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "pending-device-cancel"), OnClick(onCancel), uistate.T("authCards.pendingCancel")),
		)),

		If(p == pendingPhaseApproved, Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "pending-device-approved"), uistate.T("authCards.pendingApprovedTitle")),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.pendingApprovedHint")),
			Div(css.Class("set-input", tw.Text15), Attr("data-testid", "pending-device-code"), Text(pairingCode.Get())),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt1),
				Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "pending-device-accept"), OnClick(onAccept), uistate.T("authCards.pendingAccept")),
				Button(css.Class("btn btn-sm btn-del"), Type("button"), Attr("data-testid", "pending-device-reject"), OnClick(onReject), uistate.T("authCards.pendingReject")),
			),
		)),

		If(p == pendingPhaseSettingPass, Fragment(
			P(css.Class(tw.Text12), Attr("data-testid", "pending-device-setpassword"), uistate.T("authCards.pendingSetPasswordTitle")),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.pendingSetPasswordHint")),
			If(setPwErr.Get() != "", P(css.Class(tw.Text12, tw.TextDanger), setPwErr.Get())),
			Input(css.Class("set-input"), Type("text"), Attr("autocomplete", "username"),
				Attr("aria-label", uistate.T("authCards.usernameLabel")), Attr("data-testid", "pending-device-username"),
				Placeholder(uistate.T("authCards.usernamePlaceholder")), OnInput(onSetPwUsername), ui.FieldValue(setPwUsername.Get())),
			Input(css.Class("set-input"), Type("password"), Attr("autocomplete", "new-password"),
				Attr("aria-label", uistate.T("authCards.passwordLabel")), Attr("data-testid", "pending-device-password"),
				Placeholder(uistate.T("authCards.passwordPlaceholderRegister")), OnInput(onSetPwPassword), ui.FieldValue(setPwPassword.Get())),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt1),
				Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "pending-device-setpassword-submit"),
					DisabledIf(setPwSubmitting.Get()), OnClick(onSubmitSetPassword),
					IfElse(setPwSubmitting.Get(), Text(uistate.T("authCards.pendingSetPasswordSaving")), Text(uistate.T("authCards.pendingSetPasswordSubmit")))),
			),
		)),

		If(p == pendingPhaseRecovery, Fragment(
			Span(css.Class(tw.Text13, tw.FontSemibold), uistate.T("authCards.recoveryTitle")),
			P(css.Class(tw.TextFaint, tw.Text12), uistate.T("authCards.recoveryIntro")),
			Div(css.Class("set-input", tw.Text15), Attr("data-testid", "pending-device-recovery-code"), Text(setPwRecovery.Get())),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "pending-device-recovery-dismiss"),
				OnClick(onDismissRecovery), uistate.T("authCards.recoveryDismiss")),
		)),

		If(p == pendingPhaseDone, P(css.Class(tw.Text12), Attr("data-testid", "pending-device-done"), uistate.T("authCards.pendingDone"))),
		If(p == pendingPhaseRejected, P(css.Class(tw.Text12, tw.TextDanger), Attr("data-testid", "pending-device-rejected"), uistate.T("authCards.pendingRejectedByAdmin"))),
		If(p == pendingPhaseCanceled, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "pending-device-canceled"), uistate.T("authCards.pendingCanceled"))),
		If(p == pendingPhaseExpired, P(css.Class(tw.Text12, tw.TextDanger), Attr("data-testid", "pending-device-expired"), uistate.T("authCards.pendingExpired"))),
		If(p == pendingPhaseError, P(css.Class(tw.Text12, tw.TextDanger), Attr("data-testid", "pending-device-error"), errMsg.Get())),
	)
}

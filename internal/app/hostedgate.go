// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/rpcprotocol"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

const hostedAppMetaSelector = `meta[name="cashflux-hosted-app"][content="true"]`

type hostedGatePhase string

const (
	hostedGateChecking hostedGatePhase = "checking"
	hostedGateSignIn   hostedGatePhase = "signIn"
	hostedGateAllowed  hostedGatePhase = "allowed"
	hostedGateError    hostedGatePhase = "error"
)

type HostedAuthGateProps struct {
	Content uic.Node
}

var hostedValidatedRefreshToken string

func hostedDocument() bool {
	doc := jsDocument()
	if !doc.Truthy() {
		return false
	}
	return doc.Call("querySelector", hostedAppMetaSelector).Truthy()
}

func jsDocument() js.Value {
	return js.Global().Get("document")
}

// configureHostedPrefs runs after browserstore initialization but before the
// router mounts. A hosted client always talks to its own origin and always
// enables that connection; without a user session BackendActive still remains
// false because there is no bearer token.
func configureHostedPrefs() {
	if !hostedDocument() {
		return
	}
	p := uistate.LoadPrefs()
	p.ServerURL = appOrigin()
	p.ServerMode = prefs.ServerSelfHosted
	p.ConnectionSegment = prefs.ConnectionLocal
	p.BackendDisabled = false
	uistate.PersistPrefs(p.Normalize())
}

func initialHostedGatePhase() hostedGatePhase {
	refresh := strings.TrimSpace(lsGet(authRefreshTokenKey))
	if refresh == "" {
		return hostedGateSignIn
	}
	if refresh == hostedValidatedRefreshToken {
		return hostedGateAllowed
	}
	return hostedGateChecking
}

func clearHostedUserSession(prefsAtom state.Atom[prefs.Prefs]) {
	clearAuthSession()
	p := prefsAtom.Get()
	p.ServerToken = ""
	p.ServerCSRF = ""
	p.BackendDisabled = false
	p.ServerURL = appOrigin()
	p.ServerMode = prefs.ServerSelfHosted
	p.ConnectionSegment = prefs.ConnectionLocal
	p = p.Normalize()
	prefsAtom.Set(p)
	uistate.PersistPrefs(p)
}

// HostedAuthGate prevents every routed financial screen from mounting until a
// rotating account session is accepted by the same-origin server. Static,
// GitHub Pages, desktop, and offline bundles have no server-injected marker and
// return their content immediately.
func HostedAuthGate(props HostedAuthGateProps) uic.Node {
	if !hostedDocument() {
		return props.Content
	}
	return uic.CreateElement(hostedAuthGateActive, props)
}

func hostedAuthGateActive(props HostedAuthGateProps) uic.Node {
	prefsAtom := uistate.UsePrefs()
	phase := uic.UseState(string(initialHostedGatePhase()))
	errMsg := uic.UseState("")
	retry := uic.UseState(0)

	pr := prefsAtom.Get().Normalize()
	refresh := strings.TrimSpace(lsGet(authRefreshTokenKey))
	sessionKey := pr.ServerURL + "\x00" + pr.ServerToken + "\x00" + refresh + "\x00" + itoa(retry.Get())

	uic.UseEffect(func() func() {
		if refresh == "" {
			hostedValidatedRefreshToken = ""
			phase.Set(string(hostedGateSignIn))
			errMsg.Set("")
			return nil
		}
		if refresh == hostedValidatedRefreshToken {
			phase.Set(string(hostedGateAllowed))
			return nil
		}

		phase.Set(string(hostedGateChecking))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		go func() {
			var out backendrpc.ListDevicesResponse
			err := invokeAuthed(ctx, pr, backendrpc.MethodAuthListDevices, backendrpc.ListDevicesRequest{}, &out)
			if err == nil {
				hostedValidatedRefreshToken = strings.TrimSpace(lsGet(authRefreshTokenKey))
				errMsg.Set("")
				phase.Set(string(hostedGateAllowed))
				return
			}
			if isAuthError(err) || rpcprotocol.IsCode(err, "PermissionDenied") {
				hostedValidatedRefreshToken = ""
				clearHostedUserSession(prefsAtom)
				errMsg.Set(customSyncErrorMessage(err, uistate.T("hostedAuth.sessionEnded")))
				phase.Set(string(hostedGateSignIn))
				return
			}
			errMsg.Set(customSyncErrorMessage(err, uistate.T("hostedAuth.checkFailed")))
			phase.Set(string(hostedGateError))
		}()
		return cancel
	}, sessionKey)
	onRetry := uic.UseEvent(func() { retry.Set(retry.Get() + 1) })

	p := hostedGatePhase(phase.Get())
	if p == hostedGateAllowed && refresh != "" {
		return props.Content
	}

	content := Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.JustifyCenter),
			Span(css.Class(tw.Grid, tw.PlaceItemsCenter, tw.W9, tw.H9, tw.Rounded, tw.BgAccent, tw.TextFg, tw.FontDisplay, tw.FontSemibold), "C"),
			Span(css.Class(tw.Text18, tw.FontSemibold), uistate.T("app.name")),
		),
		H1(css.Class(tw.Text28, tw.FontDisplay, tw.TextCenter), uistate.T("hostedAuth.title")),
		P(css.Class(tw.Text14, tw.TextDim, tw.TextCenter), uistate.T("hostedAuth.intro")),
	)

	switch p {
	case hostedGateChecking:
		content = Fragment(content,
			Div(css.Class("card", tw.TextCenter), Attr("role", "status"), Attr("data-testid", "hosted-auth-checking"),
				uiw.Icon(icon.Cloud, css.Class(tw.W5, tw.H5, tw.TextDim)),
				P(css.Class(tw.Text13, tw.TextDim, tw.Mt2), uistate.T("hostedAuth.checking"))),
		)
	case hostedGateError:
		content = Fragment(content,
			Div(css.Class("card", tw.Flex, tw.FlexCol, tw.Gap2), Attr("role", "alert"), Attr("data-testid", "hosted-auth-error"),
				P(css.Class(tw.Text13, tw.TextDanger), errMsg.Get()),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(onRetry), uistate.T("hostedAuth.retry"))),
		)
	default:
		content = Fragment(content,
			uic.CreateElement(PasswordAuthCard, PasswordAuthCardProps{
				AllowRegistration: false,
				InitiallyExpanded: true,
			}),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class(tw.Flex1, tw.BorderT, tw.BorderLine)),
				Span(css.Class(tw.Text11, tw.Uppercase, tw.Tracking008, tw.TextFaint), uistate.T("hostedAuth.newAccount")),
				Span(css.Class(tw.Flex1, tw.BorderT, tw.BorderLine)),
			),
			uic.CreateElement(PendingDeviceCard, PendingDeviceCardProps{Primary: true}),
			A(css.Class("btn-link", tw.Text12, tw.TextDim, tw.TextCenter), Attr("href", "/console/"), uistate.T("hostedAuth.adminConsole")),
		)
	}

	return Div(css.Class(tw.HScreen, tw.BgBase, tw.TextFg, tw.FontSans, tw.Flex, tw.JustifyCenter, tw.Px5, tw.Py2),
		Attr("style", "overflow:auto;"),
		Div(css.Class(tw.WFull, tw.MxAuto, tw.Flex, tw.FlexCol, tw.Gap4),
			Attr("style", "max-width:32rem;padding-top:clamp(2rem,8vh,6rem);padding-bottom:3rem;"),
			Attr("data-testid", "hosted-auth-gate"),
			content,
		),
		uic.CreateElement(Toast),
	)
}

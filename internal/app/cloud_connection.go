// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

type cloudConnection struct {
	Segment, URL, Token       string
	Discovery                 backendauth.Discovery
	Phase                     discoveryPhase
	OnSegment, OnURL, OnToken func(string)
	Test, SaveToken           func()
}

// useCloudServer owns drafts. Typing never switches server/account scope. Only
// the explicit connection check commits the address, after a successful probe.
func useCloudServer(atom state.Atom[prefs.Prefs], step cloudStep) cloudConnection {
	p := atom.Get().Normalize()
	segment := uic.UseState(segmentFor(p))
	url := uic.UseState(p.ServerURL)
	token := uic.UseState(p.ServerToken)
	discovery := uic.UseState(backendauth.Discovery{})
	phase := uic.UseState(discoveryIdle)
	generation := uic.UseRef(0)
	probe := func(address string, save bool) {
		g := generation.Get() + 1
		generation.Set(g)
		phase.Set(discoveryChecking)
		testBackendConnection(address, "", func(d backendauth.Discovery) {
			if generation.Get() != g {
				return
			}
			complete := func() { discovery.Set(d); phase.Set(discoveryOK) }
			if !save {
				complete()
				return
			}
			next := atom.Get()
			next.ServerURL = address
			if segment.Get() == "commercial" {
				next.ServerMode = prefs.ServerCloud
			} else {
				next.ServerMode = prefs.ServerSelfHosted
				next.ConnectionSegment = prefs.ConnectionSegment(segment.Get())
			}
			hostChanged := normalizedBackendEndpoint(next.ServerURL) != normalizedBackendEndpoint(atom.Get().ServerURL)
			hadSession := uistate.Session(atom.Get().ServerToken).Present()
			if hostChanged {
				next.ServerToken = ""
				next.ServerCSRF = ""
			}
			outgoing := outgoingRefreshScope()
			commit := func() {
				cloudCommitPreferences(atom, next, func(err error) {
					if err != nil {
						logSyncError("save cloud connection failed", err)
						if generation.Get() == g {
							phase.Set(discoveryError)
						}
						return
					}
					if hostChanged && (outgoing != nil || hadSession) {
						reloadAfterIdentityChange("cloud server changed")
						return
					}
					if generation.Get() == g {
						complete()
					}
				})
			}
			if hostChanged {
				if outgoing != nil {
					beginSessionTransition()
					resetScopeForServerChangeThen(*outgoing, func(_ bool, err error) {
						if err != nil {
							logSyncError("remove outgoing server session failed", err)
							phase.Set(discoveryError)
							return
						}
						commit()
					})
				} else if !backendSessionReady() {
					phase.Set(discoveryError)
				} else {
					resetScopeForServerChange()
					commit()
				}
			} else {
				commit()
			}
		}, func(string) {
			if generation.Get() == g {
				phase.Set(discoveryError)
			}
		})
	}
	uic.UseEffect(func() func() {
		if step != cloudOff && step != cloudWhere {
			probe(p.ServerURL, false)
		}
		return func() { generation.Set(generation.Get() + 1) }
	}, p.ServerURL, string(step))
	test := func() {
		address := strings.TrimSpace(url.Get())
		if segment.Get() == "local" {
			address = currentPageOrigin()
			url.Set(address)
		}
		if address == "" {
			phase.Set(discoveryError)
			return
		}
		if backendHost(address) != backendHost(atom.Get().ServerURL) && uistate.Session(atom.Get().ServerToken).Present() {
			uistate.ConfirmModalLabeled(uistate.T("cloud1.changeServerConfirm", backendHost(address)), uistate.T("cloud1.changeConnection"), true,
				func(ok bool) {
					if ok {
						probe(address, true)
					}
				})
			return
		}
		probe(address, true)
	}
	saveToken := func() {
		if phase.Get() != discoveryOK || !atom.Get().BackendDisabled {
			return
		}
		next := atom.Get()
		next.ServerToken = strings.TrimSpace(token.Get())
		commit := func() {
			cloudCommitPreferences(atom, next, func(err error) {
				if err != nil {
					logSyncError("save cloud access code failed", err)
					phase.Set(discoveryError)
					return
				}
				reloadAfterIdentityChange("cloud access code sign-in")
			})
		}
		if outgoing := outgoingRefreshScope(); outgoing != nil {
			beginSessionTransition()
			resetScopeForServerChangeThen(*outgoing, func(_ bool, err error) {
				if err != nil {
					logSyncError("remove outgoing session failed", err)
					phase.Set(discoveryError)
					return
				}
				commit()
			})
		} else if backendSessionReady() {
			resetScopeForServerChange()
			commit()
		} else {
			phase.Set(discoveryError)
		}
	}
	return cloudConnection{Segment: segment.Get(), URL: url.Get(), Token: token.Get(), Discovery: discovery.Get(), Phase: phase.Get(),
		OnSegment: func(v string) { segment.Set(v); phase.Set(discoveryIdle); generation.Set(generation.Get() + 1) },
		OnURL:     func(v string) { url.Set(v); phase.Set(discoveryIdle); generation.Set(generation.Get() + 1) },
		OnToken:   func(v string) { token.Set(v) }, Test: test, SaveToken: saveToken}
}

type cloudWhereProps struct {
	Connection cloudConnection
	OnContinue func()
	Busy       bool
}

func cloudWhereView(p cloudWhereProps) uic.Node {
	c := p.Connection
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), Attr("data-testid", "cloud-setup-where"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.step.where")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.whereHint")),
		uiw.Segmented(uiw.SegmentedProps{Label: uistate.T("cloud1.destination"), Selected: c.Segment, OnSelect: c.OnSegment,
			Options: []uiw.SegOption{{Value: "commercial", Label: uistate.T("sync.segmentCommercial")}, {Value: "local", Label: uistate.T("cloud1.sameServer")}, {Value: "remote", Label: uistate.T("cloud1.ownServer")}}}),
		If(c.Segment == "local", P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.sameServerHint"))),
		If(c.Segment != "local", uic.CreateElement(cloudDisclosure, cloudDisclosureProps{Title: uistate.T("cloud1.connectionAddress"), TestID: "cloud-address-advanced", Body: uic.CreateElement(cloudAddressField, c)})),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.test"), TestID: "sync-test", OnClick: c.Test, Disabled: p.Busy || c.Phase == discoveryChecking}),
		uic.CreateElement(cloudConnectionFeedback, c.Phase),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.continue"), TestID: "cloud-where-continue", Kind: cloudActionPrimary, OnClick: p.OnContinue, Disabled: p.Busy || c.Phase != discoveryOK}),
		A(css.Class("btn-link"), Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/docs/SELF_HOSTING.md"), Attr("target", "_blank"), Attr("rel", "noreferrer"), uistate.T("settings.deploySelfHost")),
	)
}

func cloudAddressField(c cloudConnection) uic.Node {
	return uiw.Field(uiw.FieldProps{Label: uistate.T("cloud1.serverAddress"), Hint: uistate.T("cloud1.addressHint"), Control: func(a uiw.FieldA11y) uic.Node {
		return Input(css.Class("set-input"), Type("url"), ID(a.ID), Attr("aria-describedby", a.DescribedBy), Attr("data-testid", "sync-server-url"),
			Placeholder(prefs.DefaultServerURL), OnInput(c.OnURL), uiw.FieldValue(c.URL))
	}})
}

func cloudConnectionFeedback(p discoveryPhase) uic.Node {
	key, id := "cloud1.testHint", "cloud-connection-idle"
	switch p {
	case discoveryChecking:
		key, id = "cloud1.testing", "sync-discovery-checking"
	case discoveryOK:
		key, id = "cloud1.testOK", "sync-discovery-ok"
	case discoveryError:
		key, id = "cloud1.testFailed", "sync-discovery-error"
	}
	return P(css.Class(tw.Text14, tw.TextDim), Attr("role", "status"), Attr("aria-live", "polite"), Attr("data-testid", id), uistate.T(key))
}

type cloudFirstProps struct {
	Facts   cloudFacts
	Busy    bool
	OnStart func()
	OnRetry func()
}

func cloudFirstView(p cloudFirstProps) uic.Node {
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), Attr("data-testid", "cloud-setup-first"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.step.first")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.firstHint")),
		If(!p.Facts.Disabled, P(Attr("role", "status"), Attr("aria-live", "polite"), uistate.T("cloud1.syncingHint"))),
		If(!p.Facts.Disabled && p.Facts.State != "syncing", uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.retry"), TestID: "cloud-first-retry", OnClick: p.OnRetry, Disabled: p.Busy || !cloudCanSync(p.Facts)})),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.enable"), TestID: "cloud-start-encrypted-sync", Kind: cloudActionPrimary, OnClick: p.OnStart,
			Disabled: p.Busy || !p.Facts.Disabled || !p.Facts.Protected || !p.Facts.SignedIn}),
	)
}

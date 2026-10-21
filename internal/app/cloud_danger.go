// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type cloudDangerProps struct {
	Household, Server string
	OnPause           func(func(error))
	OnDelete          func(func(error)) // injected by the container; never a transport call in the view
}

type cloudDangerRequest struct {
	Mode, Household, Server string
	Run                     func(func(error))
}

func cloudDangerView(p cloudDangerProps) uic.Node {
	request := uic.UseState(cloudDangerRequest{})
	open := func(mode string, run func(func(error))) {
		request.Set(cloudDangerRequest{Mode: mode, Household: p.Household, Server: p.Server, Run: run})
	}
	r := request.Get()
	return Section(css.Class(cloudSectionRules()), Attr("data-testid", "cloud-danger"),
		H3(css.Class(tw.Text15, tw.FontSemibold, tw.TextDanger), uistate.T("cloud1.danger")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.dangerHint")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.pause"), TestID: "cloud-pause", Kind: cloudActionDanger, OnClick: func() { open("pause", p.OnPause) }}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.delete"), TestID: "cloud-delete-server", Kind: cloudActionDanger, OnClick: func() { open("delete", p.OnDelete) }, Disabled: p.OnDelete == nil}),
		),
		P(css.Class(tw.Text12, tw.TextDim), uistate.T("cloud1.eraseHint")),
		If(r.Mode != "", uic.CreateElement(cloudDangerConfirm, cloudDangerConfirmProps{Mode: r.Mode, Household: r.Household, Server: r.Server,
			OnClose: func() { request.Set(cloudDangerRequest{}) }, OnPause: r.Run, OnDelete: r.Run})),
	)
}

type cloudDangerConfirmProps struct {
	Mode, Household, Server string
	OnClose                 func()
	OnPause, OnDelete       func(func(error))
}

func cloudDangerConfirm(p cloudDangerConfirmProps) uic.Node {
	name := uic.UseState("")
	busy := uic.UseState(false)
	errText := uic.UseState("")
	action, explanation := "cloud1.pause", "cloud1.pauseConfirm"
	if p.Mode == "delete" {
		action, explanation = "cloud1.delete", "cloud1.deleteConfirm"
	}
	close := func() {
		if !busy.Get() {
			p.OnClose()
		}
	}
	confirm := func() {
		if busy.Get() || strings.TrimSpace(name.Get()) != p.Household {
			return
		}
		busy.Set(true)
		errText.Set("")
		done := func(err error) {
			busy.Set(false)
			if err != nil {
				errText.Set(uistate.T("cloud1.dangerFailed"))
				return
			}
			p.OnClose()
		}
		if p.Mode == "delete" {
			if p.OnDelete == nil {
				done(errors.New("server deletion unavailable"))
				return
			}
			p.OnDelete(done)
		} else {
			p.OnPause(done)
		}
	}
	return uiw.Overlay(uiw.OverlayProps{Open: true, Kind: uiw.OverlayModal, Title: uistate.T(action), TestID: "cloud-danger-confirm", OnClose: close,
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3),
			P(uistate.T(explanation, p.Household)), P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.serverNamed", p.Server)),
			If(p.Mode == "delete", P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.deleteCaution"))),
			uiw.Field(uiw.FieldProps{Label: uistate.T("cloud1.typeHousehold", p.Household), Error: errText.Get(), Control: func(a uiw.FieldA11y) uic.Node {
				return Input(css.Class("set-input"), ID(a.ID), Type("text"), Attr("aria-describedby", a.DescribedBy), Attr("data-testid", "cloud-confirm-name"),
					DisabledIf(busy.Get()), OnInput(func(v string) { name.Set(v) }), uiw.FieldValue(name.Get()))
			}})),
		Footer: Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.cancel"), TestID: "cloud-danger-cancel", OnClick: close, Disabled: busy.Get()}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T(action), TestID: "cloud-danger-submit", Kind: cloudActionDanger, OnClick: confirm, Disabled: busy.Get() || strings.TrimSpace(name.Get()) != p.Household})),
	})
}

// cloudDeleteCopy stops this device durably before issuing the existing deletion
// RPC. A mutex waits out its in-flight snapshot push; the scope is captured here
// and never read from a different workspace after the asynchronous pause.
func cloudDeleteCopy(pr prefs.Prefs, workspaceID string, pause func(func(error)), done func(error)) {
	owner := signedInUserID()
	if activeWorkspaceID() != workspaceID {
		done(errors.New("the active household changed"))
		return
	}
	pause(func(err error) {
		if err != nil {
			done(err)
			return
		}
		go func() {
			syncPushMu.Lock()
			defer syncPushMu.Unlock()
			if activeWorkspaceID() != workspaceID || signedInUserID() != owner {
				done(errors.New("the active sync scope changed"))
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			var out backendrpc.DeleteWorkspaceResponse
			err := invokeAuthed(ctx, pr, backendrpc.MethodSyncDeleteWorkspace, backendrpc.DeleteWorkspaceRequest{ID: workspaceID, DeviceID: syncDeviceID()}, &out)
			if err != nil {
				logSyncError("delete cloud workspace failed", err)
			} else {
				uistate.PostNotice(uistate.T("cloud1.deleted"), false)
			}
			done(err)
		}()
	})
}

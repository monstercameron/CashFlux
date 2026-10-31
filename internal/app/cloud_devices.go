// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
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

type cloudDevicesPhase string

const (
	cloudDevicesLoading cloudDevicesPhase = "loading"
	cloudDevicesReady   cloudDevicesPhase = "ready"
	cloudDevicesFailed  cloudDevicesPhase = "failed"
)

type cloudDevicesProps struct {
	Phase      cloudDevicesPhase
	Devices    []backendrpc.DeviceSession
	OtherCount int
	OnRetry    func()
	OnRevoke   func(backendrpc.DeviceSession)
}

// useCloudDevices shares the same rotating-session RPC path as authentication,
// rather than treating an empty static-token preference as signed out.
func useCloudDevices(pr prefs.Prefs, signedIn bool) cloudDevicesProps {
	devices := uic.UseState([]backendrpc.DeviceSession{})
	phase := uic.UseState(cloudDevicesLoading)
	rev := uic.UseState(0)
	uic.UseEffect(func() func() {
		if !signedIn {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		phase.Set(cloudDevicesLoading)
		go func() {
			var out backendrpc.ListDevicesResponse
			err := invokeAuthed(ctx, pr, backendrpc.MethodAuthListDevices, backendrpc.ListDevicesRequest{}, &out)
			if ctx.Err() == context.Canceled {
				return
			}
			if err != nil {
				logSyncError("list cloud devices failed", err)
				phase.Set(cloudDevicesFailed)
				return
			}
			devices.Set(out.Devices)
			phase.Set(cloudDevicesReady)
		}()
		return cancel
	}, pr.ServerURL, effectiveServerToken(pr), signedIn, rev.Get())
	retry := func() { rev.Update(func(n int) int { return n + 1 }) }
	revoke := func(d backendrpc.DeviceSession) {
		if d.Current {
			return
		}
		name := d.DeviceLabel
		if name == "" {
			name = d.FamilyID
		}
		uistate.ConfirmModalLabeled(uistate.T("cloud1.revokeConfirm", name), uistate.T("devices.revoke"), true, func(ok bool) {
			if !ok {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				var out backendrpc.RevokeDeviceResponse
				if err := invokeAuthed(ctx, pr, backendrpc.MethodAuthRevokeDevice, backendrpc.RevokeDeviceRequest{FamilyID: d.FamilyID}, &out); err != nil {
					logSyncError("revoke cloud device failed", err)
					phase.Set(cloudDevicesFailed)
					return
				}
				retry()
			}()
		})
	}
	others := 0
	for _, d := range devices.Get() {
		if !d.Current {
			others++
		}
	}
	return cloudDevicesProps{Phase: phase.Get(), Devices: devices.Get(), OtherCount: others, OnRetry: retry, OnRevoke: revoke}
}

func cloudDevicesView(p cloudDevicesProps) uic.Node {
	return Section(css.Class(cloudSectionRules()), Attr("data-testid", "cloud-devices"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("devices.title")),
		If(p.Phase == cloudDevicesLoading, P(Attr("role", "status"), uistate.T("devices.loading"))),
		If(p.Phase == cloudDevicesFailed, Fragment(
			P(Attr("role", "alert"), uistate.T("cloud1.devicesFailed")),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.retry"), TestID: "cloud-devices-retry", OnClick: p.OnRetry}))),
		If(p.Phase == cloudDevicesReady && len(p.Devices) == 0, P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.noDevices"))),
		Ul(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), MapKeyed(p.Devices, func(d backendrpc.DeviceSession) any { return d.FamilyID }, func(d backendrpc.DeviceSession) uic.Node {
			return uic.CreateElement(cloudDeviceView, cloudDeviceProps{Device: d, OnRevoke: p.OnRevoke})
		})),
	)
}

type cloudDeviceProps struct {
	Device   backendrpc.DeviceSession
	OnRevoke func(backendrpc.DeviceSession)
}

func cloudDeviceView(p cloudDeviceProps) uic.Node {
	d := p.Device
	name := d.DeviceLabel
	if name == "" {
		name = uistate.T("cloud1.unnamedDevice")
	}
	return Li(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		Span(css.Class(tw.Text14), name),
		If(d.Current, uiw.Pill(uiw.PillProps{Label: uistate.T("devices.current"), Tone: uiw.PillAccent})),
		If(!d.Current, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.revokeDevice", name), TestID: "cloud-revoke-" + d.FamilyID, OnClick: func() { p.OnRevoke(d) }})),
	)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package main

// The console's answer to "which browser is this, and what can it reach?"
// (TODOS.md C698, C700).
//
// The Users table showed account, provider, plan, status and date — none of
// which identifies a device or names a workspace. So when somebody reported
// that their browser could not sync, there was nothing in the console to check
// the report against: no workspace ids to compare, no device rows to match, no
// way to see that an account had been created by an approval and never used.
// This adds that view, and the two recovery actions that do not require minting
// yet another account.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// adminWorkspaceRow mirrors server.AdminWorkspaceResponse.
type adminWorkspaceRow struct {
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`
	Deleted     bool   `json:"deleted"`
	Version     int64  `json:"version"`
	UpdatedAt   string `json:"updatedAt"`
	DeviceID    string `json:"deviceId"`
}

// adminDeviceRow mirrors server.AdminDeviceResponse.
type adminDeviceRow struct {
	DeviceID       string `json:"deviceId"`
	Label          string `json:"label"`
	Status         string `json:"status"`
	UserID         string `json:"userId"`
	RequestedAt    string `json:"requestedAt"`
	ExpiresAt      string `json:"expiresAt"`
	ResolvedAt     string `json:"resolvedAt"`
	ResolvedBy     string `json:"resolvedBy"`
	RedeemedAt     string `json:"redeemedAt"`
	HasPairingCode bool   `json:"hasPairingCode"`
	Reissuable     bool   `json:"reissuable"`
}

// adminAccessView mirrors server.AdminAccessView.
type adminAccessView struct {
	UserID     string              `json:"userId"`
	Username   string              `json:"username"`
	Email      string              `json:"email"`
	Provider   string              `json:"provider"`
	Role       string              `json:"role"`
	Suspended  bool                `json:"suspended"`
	CreatedAt  string              `json:"createdAt"`
	Workspaces []adminWorkspaceRow `json:"workspaces"`
	Devices    []adminDeviceRow    `json:"devices"`
	LastSyncAt string              `json:"lastSyncAt"`
}

func fetchUserAccess(token, id string) (*adminAccessView, error) {
	code, body, err := adminDo(token, "GET", "/v1/admin/users/"+id+"/access", "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("access: HTTP %d", code)
	}
	var v adminAccessView
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func postAttachDevice(token, deviceID, userID string) (string, error) {
	body, _ := json.Marshal(map[string]string{"userId": userID})
	code, out, err := adminDo(token, "POST", "/v1/admin/pending-devices/"+deviceID+"/attach", string(body))
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("attach: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	var d adminPendingDecision
	if err := json.Unmarshal(out, &d); err != nil {
		return "", err
	}
	return d.PairingCode, nil
}

// deleteProvisionalAccount removes a device account an approval created that
// never became anything. It goes through its OWN endpoint rather than the
// general account delete: that one will remove any account, and this is used in
// exactly the moment somebody is confused about which account is which.
func deleteProvisionalAccount(token, userID string) error {
	code, out, err := adminDo(token, "DELETE", "/v1/admin/users/"+userID+"/provisional", "")
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("remove: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	return nil
}

func postReissuePairing(token, deviceID string) (string, error) {
	code, out, err := adminDo(token, "POST", "/v1/admin/pending-devices/"+deviceID+"/reissue", "")
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("reissue: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	var d adminPendingDecision
	if err := json.Unmarshal(out, &d); err != nil {
		return "", err
	}
	return d.PairingCode, nil
}

type accessProps struct {
	token  string
	userID string
}

// accessPanel is the read-only "what this account can reach" view, plus the
// reissue action for its devices.
//
// C698 requires this to be READ-ONLY and shown BEFORE any transfer action. That
// ordering is the point: a migration decided from memory about which workspace
// belongs to whom is how data lands on the wrong account. The only action
// offered here is reissuing a pairing code, which grants nothing new — it
// re-delivers access to the account the request was already approved onto.
func accessPanel(p accessProps) ui.Node {
	view := ui.UseState[*adminAccessView](nil)
	status := ui.UseState("")
	loadErr := ui.UseState("")
	reload := ui.UseState(0)
	busy := ui.UseState("")

	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	token, id := p.token, p.userID
	ui.UseEffect(func() func() {
		go func() {
			loadErr.Set("")
			v, err := fetchUserAccess(token, id)
			if err != nil {
				loadErr.Set(err.Error())
				return
			}
			view.Set(v)
		}()
		return nil
	}, id, reload.Get())

	retry := ui.UseEvent(func() { reload.Set(reload.Get() + 1) })
	confirmRemove := ui.UseState(false)
	toggleRemove := ui.UseEvent(func() { confirmRemove.Set(!confirmRemove.Get()) })
	removeProvisional := ui.UseEvent(func() {
		if busy.Get() != "" {
			return
		}
		busy.Set(id)
		status.Set("")
		go func() {
			defer busy.Set("")
			if err := deleteProvisionalAccount(token, id); err != nil {
				status.Set("Could not remove this account: " + err.Error())
				return
			}
			confirmRemove.Set(false)
			status.Set("Provisional account removed.")
			reload.Set(reload.Get() + 1)
		}()
	})
	reissue := func(deviceID string) {
		if busy.Get() != "" {
			return
		}
		busy.Set(deviceID)
		status.Set("")
		go func() {
			defer busy.Set("")
			code, err := postReissuePairing(token, deviceID)
			if err != nil {
				status.Set("Could not reissue: " + err.Error())
				return
			}
			// Shown once, here, and never served back from a list endpoint.
			status.Set("New verification code: " + code)
			reload.Set(reload.Get() + 1)
		}()
	}

	v := view.Get()
	switch {
	case loadErr.Get() != "":
		return Div(css.Class("panel-card"), Attr("data-testid", "admin-access-error"),
			Div(css.Class("action-desc"), Text("Could not load what this account can reach: "+loadErr.Get())),
			Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-access-retry"), OnClick(retry), Text("Try again")),
		)
	case v == nil:
		return Div(css.Class("panel-card"), Attr("data-testid", "admin-access-loading"), Text("Loading…"))
	}

	var wsRows []ui.Node
	for _, w := range v.Workspaces {
		name := strings.TrimSpace(w.Name)
		if name == "" {
			name = "(unnamed)"
		}
		// A deleted workspace is shown, not hidden: it still owns its id, and a
		// device pinned to that id still gets `workspace not found`. Hiding the
		// row would hide the explanation.
		state := "live"
		if w.Deleted {
			state = "deleted"
		}
		wsRows = append(wsRows, Div(css.Class("detail-row"), Attr("data-testid", "admin-access-workspace"),
			Div(css.Class("detail-label"), Text(name+" · "+state)),
			Div(css.Class("detail-value"), Text(w.WorkspaceID+" · v"+fmt.Sprintf("%d", w.Version)+" · "+trimDate(w.UpdatedAt))),
		))
	}
	if len(wsRows) == 0 {
		wsRows = append(wsRows, Div(css.Class("usage-empty"), Attr("data-testid", "admin-access-no-workspaces"),
			Text("This account owns no workspaces. A device signed in as it cannot read anything.")))
	}

	var devRows []ui.Node
	for _, d := range v.Devices {
		devRows = append(devRows, ui.CreateElement(accessDeviceRow, accessDeviceRowProps{
			device: d, busy: busy.Get() != "", onReissue: reissue,
		}))
	}
	if len(devRows) == 0 {
		devRows = append(devRows, Div(css.Class("usage-empty"), Attr("data-testid", "admin-access-no-devices"),
			Text("No device request produced this account — it was created some other way.")))
	}

	// Offered only where it can succeed: a device: account with no live
	// workspace data. The server refuses anything else, and a button that is
	// always there but usually fails teaches operators to ignore the refusal.
	liveWorkspaces := 0
	for _, w := range v.Workspaces {
		if !w.Deleted {
			liveWorkspaces++
		}
	}
	removable := strings.HasPrefix(v.UserID, "device:") && v.Role != "owner" && liveWorkspaces == 0

	lastSync := strings.TrimSpace(v.LastSyncAt)
	if lastSync == "" {
		lastSync = "never"
	} else {
		lastSync = trimDate(lastSync)
	}

	return Div(Attr("data-testid", "admin-access-panel"),
		Div(css.Class("section-title"), Text("What this account can reach")),
		Div(css.Class("panel-card"),
			detailRow("Account", v.UserID),
			detailRow("Last write to any workspace", lastSync),
			detailRow("Suspended", map[bool]string{true: "yes", false: "no"}[v.Suspended]),
		),
		Div(css.Class("section-title"), Text("Workspaces")),
		Div(css.Class("panel-card"), wsRows),
		Div(css.Class("section-title"), Text("Devices")),
		If(status.Get() != "", Div(css.Class("status-banner"), Attr("role", "status"),
			Attr("data-testid", "admin-access-status"), Text(status.Get()))),
		Div(css.Class("approval-list"), devRows),
		If(removable, Div(css.Class("action-card action-danger"), Attr("data-testid", "admin-provisional-remove-card"),
			Div(css.Class("action-desc"),
				Text("This is a provisional device account with no live workspace data — an approval that never became anything. Removing it keeps the console readable.")),
			If(!confirmRemove.Get(), Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-provisional-remove"), OnClick(toggleRemove),
				Text("Remove this provisional account"))),
			If(confirmRemove.Get(), Div(css.Class("confirm-delete"),
				Span(Text("This cannot be undone. The account and its pairing history go away.")),
				Div(css.Class("approval-actions"),
					Button(Type("button"), css.Class("btn btn-danger"),
						Attr("data-testid", "admin-provisional-remove-confirm"),
						Disabled(busy.Get() != ""), OnClick(removeProvisional), Text("Yes, remove it")),
					Button(Type("button"), css.Class("btn btn-secondary"),
						Attr("data-testid", "admin-provisional-remove-cancel"),
						OnClick(toggleRemove), Text("Cancel")),
				),
			)),
		)),
	)
}

type accessDeviceRowProps struct {
	device    adminDeviceRow
	busy      bool
	onReissue func(string)
}

// accessDeviceRow renders one device's lifecycle. Its own component so the
// reissue handler sits at a stable hook position (the framework forbids
// registering On* handlers inside a variable-length loop).
func accessDeviceRow(p accessDeviceRowProps) ui.Node {
	reissue := ui.UseEvent(func() { p.onReissue(p.device.DeviceID) })
	label := strings.TrimSpace(p.device.Label)
	if label == "" {
		label = "Unnamed device"
	}
	// The lifecycle in one line, in the order it happened. "approved" and
	// "redeemed" look identical without this — one is a working browser, the
	// other is an approval nobody ever used.
	story := "requested " + trimDate(p.device.RequestedAt)
	if p.device.ResolvedAt != "" {
		story += " · " + p.device.Status + " " + trimDate(p.device.ResolvedAt)
		if p.device.ResolvedBy != "" {
			story += " by " + p.device.ResolvedBy
		}
	} else {
		story += " · " + p.device.Status
	}
	if p.device.RedeemedAt != "" {
		story += " · code used " + trimDate(p.device.RedeemedAt)
	} else if p.device.Status == "approved" {
		story += " · code never used"
	}
	return Div(css.Class("approval-row"), Attr("data-testid", "admin-access-device"),
		Attr("data-device-status", p.device.Status),
		Div(css.Class("approval-meta"),
			Span(css.Class("approval-label"), Text(label)),
			Span(css.Class("approval-time"), Text(story)),
			Span(css.Class("approval-time"), Text(p.device.DeviceID)),
		),
		Div(css.Class("approval-actions"),
			If(p.device.Reissuable, Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-device-reissue"),
				Attr("aria-label", "Reissue pairing code for "+label),
				Disabled(p.busy), OnClick(reissue), Text("Reissue pairing"))),
		),
	)
}

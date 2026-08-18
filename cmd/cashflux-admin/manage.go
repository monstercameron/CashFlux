// SPDX-License-Identifier: MIT

//go:build js && wasm

package main

// This file adds the user-management layer of the operator console: a clickable
// users table, a per-user detail view with usage analytics, and the account actions
// (override plan, revoke sessions, delete) that call the admin management API
// (internal/server/admin_manage.go). It lives in its own file so the management UI is
// self-contained; main.go only adds a small navigation hook.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// manageCSS styles the user-management UI. It is injected from Go (once, guarded by
// element id) rather than added to web/admin/index.html so the management layer is
// fully self-contained in this file and doesn't conflict with the console shell.
const manageCSS = `
.user-row{cursor:pointer;transition:background .12s ease}
.user-row:hover{background:rgba(255,255,255,0.06)}
.user-row:focus-visible{outline:2px solid #6366f1;outline-offset:-2px}
.row-action{text-align:right;white-space:nowrap}
.btn-row{padding:.25rem .7rem;font-size:12px}
.table-hint{color:#9aa0aa;font-size:13px;margin:.1rem 0 .6rem}
.status-banner{background:rgba(99,102,241,0.14);border:1px solid rgba(99,102,241,0.4);color:#c7d2fe;padding:.6rem .9rem;border-radius:10px;margin:.6rem 0}
.manage-grid{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(0,1fr);gap:1.25rem;align-items:start}
@media (max-width:860px){.manage-grid{grid-template-columns:1fr}}
.manage-col{min-width:0;display:flex;flex-direction:column;gap:.5rem}
.section-title{font-size:13px;text-transform:uppercase;letter-spacing:.06em;color:#9aa0aa;margin:.8rem 0 .2rem}
.detail-card{background:rgba(255,255,255,0.03);border:1px solid rgba(255,255,255,0.08);border-radius:12px;padding:.4rem .9rem}
.detail-row{display:flex;justify-content:space-between;gap:1rem;padding:.45rem 0;border-bottom:1px solid rgba(255,255,255,0.06)}
.detail-row:last-child{border-bottom:none}
.detail-label{color:#9aa0aa;font-size:13px}
.detail-value{color:#e8eaed;font-weight:600;text-align:right;word-break:break-all}
.action-card{background:rgba(255,255,255,0.03);border:1px solid rgba(255,255,255,0.08);border-radius:12px;padding:.9rem;display:flex;flex-direction:column;gap:.6rem}
.action-card.action-danger{border-color:rgba(239,68,68,0.35)}
.action-desc{color:#9aa0aa;font-size:13px}
.field-row{display:flex;flex-direction:column;gap:.25rem}
.field-row label{font-size:12px;color:#9aa0aa}
.field-row input,.field-row select{background:#0f1115;border:1px solid rgba(255,255,255,0.14);border-radius:8px;color:#e8eaed;padding:.5rem .6rem;font-size:14px}
.recovery-code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:18px;letter-spacing:.08em;background:#0f1115;border:1px solid rgba(99,102,241,.45);border-radius:8px;padding:.75rem;word-break:break-all}
.confirm-delete{display:flex;flex-direction:column;gap:.5rem}
.confirm-delete span{color:#fca5a5;font-size:13px}
.usage-list{display:flex;flex-direction:column;gap:.4rem}
.usage-bar-row{display:grid;grid-template-columns:84px 1fr auto;gap:.6rem;align-items:center;font-size:12px;color:#c7ccd3}
.usage-day{color:#9aa0aa}
.usage-track{background:rgba(255,255,255,0.06);border-radius:999px;height:10px;overflow:hidden}
.usage-fill{background:linear-gradient(90deg,#6366f1,#8b5cf6);height:100%}
.usage-num{white-space:nowrap;color:#9aa0aa}
.usage-empty{color:#9aa0aa;font-size:13px}
.approval-list{display:flex;flex-direction:column;gap:.55rem;margin:.35rem 0 1rem}
.approval-row{display:flex;align-items:center;justify-content:space-between;gap:1rem;background:rgba(255,255,255,.03);border:1px solid rgba(255,255,255,.08);border-radius:10px;padding:.75rem .9rem}
.approval-meta{display:flex;flex-direction:column;gap:.15rem;min-width:0}
.approval-label{font-weight:600;color:#e8eaed;overflow-wrap:anywhere}
.approval-time{font-size:12px;color:#9aa0aa}
.approval-actions{display:flex;gap:.45rem;flex-wrap:wrap}
@media (max-width:620px){.approval-row{align-items:flex-start;flex-direction:column}}
`

// ensureManageCSS injects manageCSS into <head> once.
func ensureManageCSS() {
	doc := js.Global().Get("document")
	if doc.Call("getElementById", "cf-admin-manage-style").Truthy() {
		return
	}
	st := doc.Call("createElement", "style")
	st.Set("id", "cf-admin-manage-style")
	st.Set("textContent", manageCSS)
	doc.Get("head").Call("appendChild", st)
}

// ---------------------------------------------------------------------------
// Domain types (mirror internal/server admin_manage.go response shapes)
// ---------------------------------------------------------------------------

type adminUserDetail struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Email              string `json:"email"`
	CreatedAt          string `json:"createdAt"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	SubscriptionPlan   string `json:"subscriptionPlan"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	CurrentPeriodEnd   string `json:"currentPeriodEnd"`
	TrialEnd           string `json:"trialEnd"`
	WorkspaceCount     int    `json:"workspaceCount"`
	BlobBytes          int64  `json:"blobBytes"`
	UsageTodayRequests int64  `json:"usageTodayRequests"`
	UsageTodayTokens   int64  `json:"usageTodayTokens"`
	Suspended          bool   `json:"suspended"`
}

type adminUsageRow struct {
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

type adminUsageResp struct {
	UserID string          `json:"userId"`
	Days   int             `json:"days"`
	Usage  []adminUsageRow `json:"usage"`
}

type adminCreateUserResp struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	RecoveryCode string `json:"recoveryCode"`
}

type adminPendingDevice struct {
	DeviceID    string `json:"deviceId"`
	Label       string `json:"label"`
	RequestedAt string `json:"requestedAt"`
	ExpiresAt   string `json:"expiresAt"`
}

type adminPendingDecision struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action"`
	DeviceID    string `json:"deviceId"`
	PairingCode string `json:"pairingCode"`
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// adminDo performs an operator request and returns the status and body. A
// non-empty token is the explicit break-glass bearer path. Otherwise the
// browser sends the HttpOnly owner cookies; a 401 gets exactly one rotating
// refresh attempt before the request is retried.
func adminDo(token, method, url, body string) (int, []byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, url, rdr)
		if err != nil {
			return 0, nil, err
		}
		mutation := method != http.MethodGet && method != http.MethodHead
		applyAdminRequestAuth(req, token, mutation)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, nil, err
		}
		captureAdminCSRF(resp)
		b, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, readErr
		}
		if resp.StatusCode == http.StatusUnauthorized && strings.TrimSpace(token) == "" && attempt == 0 && refreshAdminSession() {
			continue
		}
		return resp.StatusCode, b, nil
	}
	return http.StatusUnauthorized, nil, nil
}

func fetchUserDetail(token, id string) (*adminUserDetail, error) {
	code, body, err := adminDo(token, "GET", "/v1/admin/users/"+id, "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("detail: HTTP %d", code)
	}
	var d adminUserDetail
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func fetchUserUsage(token, id string, days int) ([]adminUsageRow, error) {
	code, body, err := adminDo(token, "GET", fmt.Sprintf("/v1/admin/users/%s/usage?days=%d", id, days), "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("usage: HTTP %d", code)
	}
	var r adminUsageResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return r.Usage, nil
}

// actionResult reports the outcome of a management action for the status line.
func postSetPlan(token, id, plan, status string) error {
	body, _ := json.Marshal(map[string]string{"plan": plan, "status": status})
	code, _, err := adminDo(token, "POST", "/v1/admin/users/"+id+"/plan", string(body))
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("set plan: HTTP %d", code)
	}
	return nil
}

func postRevokeSessions(token, id string) error {
	code, _, err := adminDo(token, "POST", "/v1/admin/users/"+id+"/revoke-sessions", "")
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("revoke: HTTP %d", code)
	}
	return nil
}

func deleteUser(token, id string) error {
	code, _, err := adminDo(token, "DELETE", "/v1/admin/users/"+id, "")
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("delete: HTTP %d", code)
	}
	return nil
}

func postSuspend(token, id string, suspended bool) error {
	body, _ := json.Marshal(map[string]bool{"suspended": suspended})
	code, _, err := adminDo(token, "POST", "/v1/admin/users/"+id+"/suspend", string(body))
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("suspend: HTTP %d", code)
	}
	return nil
}

func createAdminUser(token, username, password, role string) (*adminCreateUserResp, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password, "role": role})
	code, responseBody, err := adminDo(token, "POST", "/v1/admin/users", string(body))
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("create user: HTTP %d", code)
	}
	var created adminCreateUserResp
	if err := json.Unmarshal(responseBody, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

func patchUserIdentity(token, id, username, role string) error {
	body, _ := json.Marshal(map[string]string{"username": username, "role": role})
	code, _, err := adminDo(token, "PATCH", "/v1/admin/users/"+id, string(body))
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("update user: HTTP %d", code)
	}
	return nil
}

func fetchPendingDevices(token string) ([]adminPendingDevice, error) {
	code, body, err := adminDo(token, http.MethodGet, "/v1/admin/pending-devices", "")
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("pending requests: HTTP %d", code)
	}
	var pending []adminPendingDevice
	if err := json.Unmarshal(body, &pending); err != nil {
		return nil, err
	}
	return pending, nil
}

func decidePendingDevice(token, deviceID, action string) (*adminPendingDecision, error) {
	code, body, err := adminDo(token, http.MethodPost, "/v1/admin/pending-devices/"+deviceID+"/"+action, "")
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("%s request: HTTP %d", action, code)
	}
	var decision adminPendingDecision
	if err := json.Unmarshal(body, &decision); err != nil {
		return nil, err
	}
	return &decision, nil
}

// ---------------------------------------------------------------------------
// Clickable users table (replaces the static table in readyView)
// ---------------------------------------------------------------------------

type pendingApprovalRowProps struct {
	device   adminPendingDevice
	disabled bool
	onDecide func(string, string)
	// onAttach approves a request onto an account that already exists (C700),
	// rather than minting a new one.
	onAttach func(deviceID, userID string)
}

func pendingApprovalRow(p pendingApprovalRowProps) ui.Node {
	approve := ui.UseEvent(func() { p.onDecide(p.device.DeviceID, "approve") })
	reject := ui.UseEvent(func() { p.onDecide(p.device.DeviceID, "reject") })
	// C700: approving used to be the only decision available, and it always
	// created a NEW account. An operator whose answer was "this is my other
	// browser, it belongs to the account I already have" had to pick the wrong
	// action — producing a second account that owns none of the first one's
	// workspaces, which the server then refuses to sync for ever.
	attaching := ui.UseState(false)
	target := ui.UseState("")
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	toggleAttach := ui.UseEvent(func() { attaching.Set(!attaching.Get()) })
	confirmAttach := ui.UseEvent(func() {
		if strings.TrimSpace(target.Get()) == "" {
			return
		}
		p.onAttach(p.device.DeviceID, strings.TrimSpace(target.Get()))
		attaching.Set(false)
	})
	label := strings.TrimSpace(p.device.Label)
	if label == "" {
		label = "Unnamed device"
	}
	var attachForm ui.Node = Fragment()
	if attaching.Get() {
		attachForm = Div(css.Class("field-row"), Attr("data-testid", "admin-pending-attach-form"),
			Label(Attr("for", "attach-"+p.device.DeviceID), Text("Account id to attach this device to")),
			Input(Attr("id", "attach-"+p.device.DeviceID),
				Attr("data-testid", "admin-pending-attach-target"),
				Attr("placeholder", "device:… or the account id from the Users table"),
				Value(target.Get()), OnInput(onTarget)),
			Div(css.Class("approval-actions"),
				Button(Type("button"), css.Class("btn btn-primary"),
					Attr("data-testid", "admin-pending-attach-confirm"),
					Disabled(p.disabled || strings.TrimSpace(target.Get()) == ""),
					OnClick(confirmAttach), Text("Attach to this account")),
			),
		)
	}
	return Div(css.Class("approval-row"), Attr("data-testid", "admin-pending-device"),
		Div(css.Class("approval-meta"),
			Span(css.Class("approval-label"), Text(label)),
			Span(css.Class("approval-time"), Text("Requested "+trimDate(p.device.RequestedAt)+" · expires "+trimDate(p.device.ExpiresAt))),
			attachForm,
		),
		Div(css.Class("approval-actions"),
			Button(Type("button"), css.Class("btn btn-primary"), Attr("data-testid", "admin-pending-approve"),
				Attr("aria-label", "Approve access for "+label), Disabled(p.disabled), OnClick(approve), Text("Approve as new account")),
			Button(Type("button"), css.Class("btn btn-secondary"), Attr("data-testid", "admin-pending-attach"),
				Attr("aria-label", "Attach "+label+" to an existing account"),
				Disabled(p.disabled), OnClick(toggleAttach), Text("Attach to existing…")),
			Button(Type("button"), css.Class("btn btn-danger"), Attr("data-testid", "admin-pending-reject"),
				Attr("aria-label", "Reject access for "+label), Disabled(p.disabled), OnClick(reject), Text("Reject")),
		),
	)
}

type pendingApprovalsProps struct {
	token string
}

func pendingApprovals(p pendingApprovalsProps) ui.Node {
	devices := ui.UseState[[]adminPendingDevice](nil)
	status := ui.UseState("")
	busyID := ui.UseState("")
	reload := ui.UseState(0)

	ui.UseEffect(func() func() {
		go func() {
			out, err := fetchPendingDevices(p.token)
			if err != nil {
				status.Set("Could not load access requests: " + err.Error())
				return
			}
			devices.Set(out)
			if status.Get() == "Refreshing access requests…" {
				status.Set("")
			}
		}()
		return nil
	}, p.token, reload.Get())

	refresh := ui.UseEvent(func() {
		status.Set("Refreshing access requests…")
		reload.Set(reload.Get() + 1)
	})
	decide := func(deviceID, action string) {
		busyID.Set(deviceID)
		status.Set("")
		go func() {
			out, err := decidePendingDevice(p.token, deviceID, action)
			busyID.Set("")
			if err != nil {
				status.Set("Could not " + action + " access: " + err.Error())
				return
			}
			if action == "approve" {
				status.Set("Access approved. Verification code: " + out.PairingCode)
			} else {
				status.Set("Access request rejected.")
			}
			reload.Set(reload.Get() + 1)
		}()
	}

	// Attaching to an existing account is the recovery this whole ticket set
	// exists for: it grants the device the account the operator names, instead
	// of creating a second account that owns none of the first one's data.
	attach := func(deviceID, userID string) {
		busyID.Set(deviceID)
		status.Set("")
		go func() {
			code, err := postAttachDevice(p.token, deviceID, userID)
			busyID.Set("")
			if err != nil {
				status.Set("Could not attach this device: " + err.Error())
				return
			}
			status.Set("Device attached to " + userID + ". Verification code: " + code)
			reload.Set(reload.Get() + 1)
		}()
	}

	list := Div(css.Class("usage-empty"), Attr("data-testid", "admin-pending-empty"), Text("No pending access requests."))
	if len(devices.Get()) > 0 {
		list = Div(css.Class("approval-list"),
			Map(devices.Get(), func(device adminPendingDevice) ui.Node {
				return ui.CreateElement(pendingApprovalRow, pendingApprovalRowProps{
					device: device, disabled: busyID.Get() != "", onDecide: decide, onAttach: attach,
				})
			}),
		)
	}

	return Div(Attr("data-testid", "admin-pending-approvals"),
		Div(css.Class("users-toolbar"),
			Div(
				H2(css.Class("table-title"), Text("Access requests")),
				Div(css.Class("table-hint"), Text("New clients cannot create an account or sync until you approve them.")),
			),
			Button(Type("button"), css.Class("btn btn-secondary"), Attr("data-testid", "admin-pending-refresh"),
				OnClick(refresh), Text("Refresh requests")),
		),
		If(status.Get() != "", Div(css.Class("status-banner"), Attr("role", "status"), Attr("data-testid", "admin-pending-status"), Text(status.Get()))),
		list,
	)
}

type userRowProps struct {
	user   adminUserRow
	onOpen func(string)
}

// userRow is its own component so it can own an OnClick hook safely — the framework
// rule forbids registering On* handlers inside a variable-length Map loop.
func userRow(p userRowProps) ui.Node {
	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	open := ui.UseEvent(func() { p.onOpen(p.user.ID) })
	identity := strings.TrimSpace(p.user.Username)
	if identity == "" {
		identity = strings.TrimSpace(p.user.Email)
	}
	if identity == "" {
		identity = p.user.ID
	}
	created := p.user.CreatedAt
	if len(created) >= 10 {
		created = created[:10]
	}
	plan := p.user.SubscriptionPlan
	if plan == "" {
		plan = "—"
	}
	status := p.user.SubscriptionStatus
	if status == "" {
		status = "—"
	}
	// C701: the row used to BE the button — tr[role=button][tabindex=0] with a
	// click handler. Two things were wrong with that. A table row that claims to
	// be a button stops assistive technology presenting its cells as a row, so
	// the columns an operator navigates by disappear; and role=button on a
	// non-button element does not bring keyboard activation with it, so Enter
	// and Space did nothing and the only way in was a mouse click that landed
	// nowhere visible. The affordance is now a real <button> in its own cell:
	// natively focusable, natively activated by both keys, and visible — an
	// operator no longer has to discover that rows are clickable.
	//
	// The whole-row click is kept as a mouse convenience. It is no longer the
	// only way in, so it carries no accessibility weight.
	return Tr(
		css.Class("user-row"),
		Attr("data-testid", "admin-user-row"),
		Attr("data-user-id", p.user.ID),
		OnClick(open),
		Td(Text(identity)),
		Td(Text(p.user.Provider)),
		Td(Text(plan)),
		Td(Text(status)),
		Td(Text(created)),
		Td(css.Class("row-action"),
			Button(Type("button"), css.Class("btn btn-secondary btn-row"),
				Attr("data-testid", "admin-user-manage"),
				Attr("data-user-id", p.user.ID),
				// The accessible name carries the account, so a column of
				// "Manage" buttons reads as N distinct actions rather than one
				// repeated N times.
				Attr("aria-label", "Manage "+identity),
				OnClick(open),
				Text("Manage")),
		),
	)
}

// usersTable renders the users list with clickable rows that open the detail view.
func usersTable(users []adminUserRow, onOpen func(string)) ui.Node {
	return Div(
		css.Class("table-section"),
		H2(css.Class("table-title"), Text("Users")),
		Div(css.Class("table-hint"), Text("Select a user to view usage and manage their account.")),
		Table(
			css.Class("users-table"),
			Thead(
				Tr(
					Th(Text("Account")),
					Th(Text("Provider")),
					Th(Text("Plan")),
					Th(Text("Status")),
					Th(Text("Created")),
					Th(Text("Actions")),
				),
			),
			Tbody(
				Map(users, func(u adminUserRow) ui.Node {
					return ui.CreateElement(userRow, userRowProps{user: u, onOpen: onOpen})
				}),
			),
		),
	)
}

type createUserProps struct {
	token   string
	onClose func()
}

// createUserView creates a normal username/password account and holds the
// one-time recovery code on screen until the operator explicitly confirms it
// was saved.
func createUserView(p createUserProps) ui.Node {
	username := ui.UseState("")
	password := ui.UseState("")
	role := ui.UseState("member")
	status := ui.UseState("")
	submitting := ui.UseState(false)
	created := ui.UseState[*adminCreateUserResp](nil)

	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	closeHandler := ui.UseEvent(p.onClose)
	onUsername := ui.UseEvent(func(v string) { username.Set(v) })
	onPassword := ui.UseEvent(func(v string) { password.Set(v) })
	onRole := ui.UseEvent(func(v string) { role.Set(v) })
	onSubmit := ui.UseEvent(func() {
		u, pw, r := strings.TrimSpace(username.Get()), password.Get(), strings.TrimSpace(role.Get())
		if u == "" || pw == "" {
			status.Set("Username and password are required.")
			return
		}
		submitting.Set(true)
		status.Set("Creating account…")
		go func() {
			out, err := createAdminUser(p.token, u, pw, r)
			submitting.Set(false)
			if err != nil {
				status.Set("Create failed: " + err.Error())
				return
			}
			password.Set("")
			created.Set(out)
			status.Set("")
		}()
	})

	var content ui.Node
	if out := created.Get(); out != nil {
		content = Div(css.Class("manage-col"),
			Div(css.Class("action-card"),
				H2(css.Class("section-title"), Text("Account created")),
				Div(css.Class("action-desc"), Text("Give this recovery code to the user now. It is shown only once.")),
				Div(css.Class("recovery-code"), Attr("aria-label", "One-time recovery code"), Text(out.RecoveryCode)),
				detailRow("Username", out.Username),
				detailRow("Role", out.Role),
				Button(Type("button"), css.Class("btn btn-primary"), Attr("aria-label", "I saved the recovery code"), OnClick(closeHandler), Text("I saved the code")),
			),
		)
	} else {
		content = Div(css.Class("manage-col"),
			Div(css.Class("action-card"),
				Div(css.Class("action-desc"), Text("Create a username/password account that can sign into CashFlux immediately.")),
				Div(css.Class("field-row"),
					Label(Attr("for", "create-username"), Text("Username")),
					Input(Attr("id", "create-username"), Type("text"), Value(username.Get()), Attr("autocomplete", "off"), OnInput(onUsername)),
				),
				Div(css.Class("field-row"),
					Label(Attr("for", "create-password"), Text("Temporary password")),
					Input(Attr("id", "create-password"), Type("password"), Value(password.Get()), Attr("autocomplete", "new-password"), OnInput(onPassword)),
				),
				Div(css.Class("field-row"),
					Label(Attr("for", "create-role"), Text("Role")),
					Select(Attr("id", "create-role"), OnChange(onRole),
						Option(Value("owner"), Selected(role.Get() == "owner"), Text("Owner")),
						Option(Value("member"), Selected(role.Get() == "member"), Text("Member")),
						Option(Value("viewer"), Selected(role.Get() == "viewer"), Text("Viewer")),
					),
				),
				Button(Type("button"), css.Class("btn btn-primary"), Disabled(submitting.Get()), OnClick(onSubmit),
					IfElse(submitting.Get(), Text("Creating…"), Text("Create account"))),
			),
		)
	}

	return Div(css.Class("console-page"),
		Div(css.Class("console-header"),
			H1(css.Class("console-title"), Div(css.Class("brand-mark"), Text("C")), Text("Create user")),
			If(created.Get() == nil, Div(css.Class("header-actions"),
				Button(Type("button"), css.Class("btn btn-secondary"), Attr("aria-label", "Back to console"), OnClick(closeHandler), Text("← Back")),
			)),
		),
		If(status.Get() != "", Div(css.Class("status-banner"), Attr("role", "status"), Text(status.Get()))),
		content,
	)
}

// ---------------------------------------------------------------------------
// User detail + management view
// ---------------------------------------------------------------------------

type manageProps struct {
	token   string
	userID  string
	onClose func() // plain callback so it can be invoked after a delete and wrapped for buttons
}

// detailRow renders one label/value line in the detail card.
func detailRow(label, value string) ui.Node {
	if value == "" {
		value = "—"
	}
	return Div(css.Class("detail-row"),
		Span(css.Class("detail-label"), Text(label)),
		Span(css.Class("detail-value"), Text(value)),
	)
}

// manageView fetches a user's detail + usage on mount and renders the management
// surface: an account summary, recent daily usage, and the operator actions.
func manageView(p manageProps) ui.Node {
	detail := ui.UseState[*adminUserDetail](nil)
	usage := ui.UseState[[]adminUsageRow](nil)
	status := ui.UseState("")
	usernameInput := ui.UseState("")
	roleInput := ui.UseState("member")
	planInput := ui.UseState("")
	statusInput := ui.UseState("")
	confirmDelete := ui.UseState(false)
	reload := ui.UseState(0)
	// C701: a failed detail fetch used to leave the card reading "Loading..."
	// for ever, with the real reason in a status banner further up the page. An
	// operator reasonably reads that as a slow request and waits. loadErr makes
	// the failure land where the data was supposed to be, with a way to retry.
	loadErr := ui.UseState("")

	token, id := p.token, p.userID
	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	closeHandler := ui.UseEvent(p.onClose)

	// Fetch detail + usage whenever the target user or a reload tick changes.
	ui.UseEffect(func() func() {
		go func() {
			loadErr.Set("")
			d, err := fetchUserDetail(token, id)
			if err != nil {
				loadErr.Set(err.Error())
				status.Set("Could not load user: " + err.Error())
				return
			}
			detail.Set(d)
			usernameInput.Set(d.Username)
			if strings.TrimSpace(d.Role) == "" {
				roleInput.Set("member")
			} else {
				roleInput.Set(d.Role)
			}
			planInput.Set(d.SubscriptionPlan)
			// Default the status picker to a real option so it reflects what will be
			// sent — an empty status (no subscription yet) would otherwise show the
			// first <option> while the state held "".
			if strings.TrimSpace(d.SubscriptionStatus) == "" {
				statusInput.Set("active")
			} else {
				statusInput.Set(d.SubscriptionStatus)
			}
			// A failed usage fetch used to be swallowed entirely, leaving an
			// empty chart that reads as "this account has no activity" - a
			// wrong answer presented as a fact.
			if u, err := fetchUserUsage(token, id, 14); err == nil {
				usage.Set(u)
			} else {
				status.Set("Usage history could not be loaded: " + err.Error())
			}
		}()
		return nil
	}, id, reload.Get())

	retryLoad := ui.UseEvent(func() { reload.Set(reload.Get() + 1) })

	onPlanInput := ui.UseEvent(func(v string) { planInput.Set(v) })
	onStatusInput := ui.UseEvent(func(v string) { statusInput.Set(v) })
	onUsernameInput := ui.UseEvent(func(v string) { usernameInput.Set(v) })
	onRoleInput := ui.UseEvent(func(v string) { roleInput.Set(v) })

	saveIdentity := ui.UseEvent(func() {
		status.Set("Saving account…")
		go func() {
			if err := patchUserIdentity(token, id, strings.TrimSpace(usernameInput.Get()), strings.TrimSpace(roleInput.Get())); err != nil {
				status.Set("Account update failed: " + err.Error())
				return
			}
			status.Set("Account updated.")
			reload.Set(reload.Get() + 1)
		}()
	})

	savePlan := ui.UseEvent(func() {
		status.Set("Saving plan…")
		go func() {
			if err := postSetPlan(token, id, strings.TrimSpace(planInput.Get()), strings.TrimSpace(statusInput.Get())); err != nil {
				status.Set("Set plan failed: " + err.Error())
				return
			}
			status.Set("Plan updated.")
			reload.Set(reload.Get() + 1)
		}()
	})
	revoke := ui.UseEvent(func() {
		status.Set("Revoking sessions…")
		go func() {
			if err := postRevokeSessions(token, id); err != nil {
				status.Set("Revoke failed: " + err.Error())
				return
			}
			status.Set("All sessions revoked — the user must sign in again.")
		}()
	})
	toggleSuspend := ui.UseEvent(func() {
		d := detail.Get()
		if d == nil {
			return
		}
		want := !d.Suspended
		if want {
			status.Set("Suspending account…")
		} else {
			status.Set("Reinstating account…")
		}
		go func() {
			if err := postSuspend(token, id, want); err != nil {
				status.Set("Suspend failed: " + err.Error())
				return
			}
			if want {
				status.Set("Account suspended — logged out everywhere and blocked from signing back in.")
			} else {
				status.Set("Account reinstated.")
			}
			reload.Set(reload.Get() + 1)
		}()
	})
	askDelete := ui.UseEvent(func() { confirmDelete.Set(true) })
	cancelDelete := ui.UseEvent(func() { confirmDelete.Set(false) })
	doDelete := ui.UseEvent(func() {
		status.Set("Deleting account…")
		go func() {
			if err := deleteUser(token, id); err != nil {
				status.Set("Delete failed: " + err.Error())
				confirmDelete.Set(false)
				return
			}
			p.onClose()
		}()
	})

	d := detail.Get()
	var summary ui.Node
	switch {
	case d == nil && loadErr.Get() != "":
		summary = Div(css.Class("detail-card"), Attr("data-testid", "admin-user-detail-error"),
			Div(css.Class("action-desc"), Text("This account could not be loaded: "+loadErr.Get())),
			Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-user-detail-retry"),
				OnClick(retryLoad), Text("Try again")),
		)
	case d == nil:
		summary = Div(css.Class("detail-card"), Attr("data-testid", "admin-user-detail-loading"), Text("Loading…"))
	default:
		summary = Div(css.Class("detail-card"),
			detailRow("User ID", d.ID),
			detailRow("Username", d.Username),
			detailRow("Role", d.Role),
			detailRow("Email", d.Email),
			detailRow("Provider", d.Provider),
			detailRow("Joined", trimDate(d.CreatedAt)),
			detailRow("Plan", d.SubscriptionPlan),
			detailRow("Status", d.SubscriptionStatus),
			detailRow("Renews", trimDate(d.CurrentPeriodEnd)),
			detailRow("Workspaces", fmt.Sprintf("%d", d.WorkspaceCount)),
			detailRow("Storage", formatBytes(d.BlobBytes)),
			detailRow("Today's usage", fmt.Sprintf("%d requests · %d tokens", d.UsageTodayRequests, d.UsageTodayTokens)),
		)
	}

	// Usage history as a compact bar list (newest first).
	usageRows := usage.Get()
	var maxTokens int64 = 1
	for _, u := range usageRows {
		if u.Tokens > maxTokens {
			maxTokens = u.Tokens
		}
	}
	usageList := Div(css.Class("usage-list"),
		Map(usageRows, func(u adminUsageRow) ui.Node {
			w := fmt.Sprintf("%d%%", (u.Tokens*100)/maxTokens)
			return Div(css.Class("usage-bar-row"),
				Span(css.Class("usage-day"), Text(u.Day)),
				Div(css.Class("usage-track"), Div(css.Class("usage-fill"), Style(map[string]string{"width": w}))),
				Span(css.Class("usage-num"), Text(fmt.Sprintf("%d req · %d tok", u.Requests, u.Tokens))),
			)
		}),
	)
	if len(usageRows) == 0 {
		usageList = Div(css.Class("usage-empty"), Text("No usage recorded in the last 14 days."))
	}

	// A user with no subscription row shows the create/comp affordance; the same
	// endpoint updates an existing one. Status is a closed set the entitlement seam
	// understands (matches the server's validSubscriptionStatus) — never free text.
	hasSub := d != nil && strings.TrimSpace(d.SubscriptionStatus) != ""
	planBtnLabel := "Save plan"
	planCardDesc := "Change the user's plan or subscription status."
	if !hasSub {
		planBtnLabel = "Create subscription"
		planCardDesc = "This user has no subscription — set a plan and status to comp or create one."
	}
	statusOptions := []string{"active", "trialing", "past_due", "canceled", "none"}

	// Suspend/reinstate affordance reflects the account's current state.
	isSuspended := d != nil && d.Suspended
	suspendBtnLabel := "Suspend account"
	suspendBtnClass := "btn btn-danger"
	suspendCardDesc := "Block this account: log it out everywhere and stop it signing back in. Reversible; deletes no data."
	if isSuspended {
		suspendBtnLabel = "Reinstate account"
		suspendBtnClass = "btn btn-secondary"
		suspendCardDesc = "This account is suspended — blocked from signing in and denied cloud features. Reinstate to restore access."
	}

	deleteBlock := Button(Type("button"), css.Class("btn btn-danger"), Attr("aria-label", "Delete this account"), OnClick(askDelete), Text("Delete account"))
	if confirmDelete.Get() {
		deleteBlock = Div(css.Class("confirm-delete"),
			Span(Text("Permanently delete this account and all its data?")),
			Button(Type("button"), css.Class("btn btn-danger"), OnClick(doDelete), Text("Yes, delete")),
			Button(Type("button"), css.Class("btn btn-secondary"), OnClick(cancelDelete), Text("Cancel")),
		)
	}

	return Div(css.Class("console-page"),
		Div(css.Class("console-header"),
			H1(css.Class("console-title"),
				Div(css.Class("brand-mark"), Text("C")),
				Text("Manage user"),
			),
			Div(css.Class("header-actions"),
				Button(Type("button"), css.Class("btn btn-secondary"), Attr("aria-label", "Back to console"), OnClick(closeHandler), Text("← Back")),
			),
		),
		If(status.Get() != "", Div(css.Class("status-banner"), Text(status.Get()))),
		Div(css.Class("manage-grid"),
			Div(css.Class("manage-col"),
				H2(css.Class("section-title"), Text("Account")),
				summary,
				// C698: the read-only "what this device can access" view, shown
				// BEFORE any action that moves data. That ordering is the point —
				// a transfer decided from memory about which workspace belongs to
				// whom is how data lands on the wrong account.
				ui.CreateElement(accessPanel, accessProps{token: token, userID: id}),
				H2(css.Class("section-title"), Text("Actions")),
				Div(css.Class("action-card"),
					Div(css.Class("action-desc"), Text("Change the user's sign-in name or access role.")),
					Div(css.Class("field-row"),
						Label(Attr("for", "username-input"), Text("Username")),
						Input(Attr("id", "username-input"), Type("text"), Value(usernameInput.Get()), OnInput(onUsernameInput)),
					),
					Div(css.Class("field-row"),
						Label(Attr("for", "role-input"), Text("Role")),
						Select(Attr("id", "role-input"), OnChange(onRoleInput),
							Option(Value("owner"), Selected(roleInput.Get() == "owner"), Text("Owner")),
							Option(Value("member"), Selected(roleInput.Get() == "member"), Text("Member")),
							Option(Value("viewer"), Selected(roleInput.Get() == "viewer"), Text("Viewer")),
						),
					),
					Button(Type("button"), css.Class("btn btn-primary"), OnClick(saveIdentity), Text("Save account")),
				),
				Div(css.Class("action-card"),
					Div(css.Class("action-desc"), Text(planCardDesc)),
					Div(css.Class("field-row"),
						Label(Attr("for", "plan-input"), Text("Plan")),
						Input(Attr("id", "plan-input"), Type("text"), Value(planInput.Get()), Attr("placeholder", "monthly / annual / comp"), OnInput(onPlanInput)),
					),
					Div(css.Class("field-row"),
						Label(Attr("for", "status-input"), Text("Status")),
						Select(Attr("id", "status-input"), OnChange(onStatusInput),
							Map(statusOptions, func(s string) ui.Node {
								return Option(Value(s), Selected(statusInput.Get() == s), Text(s))
							}),
						),
					),
					Button(Type("button"), css.Class("btn btn-primary"), OnClick(savePlan), Text(planBtnLabel)),
				),
				Div(css.Class("action-card"),
					Div(css.Class("action-desc"), Text("Force the user to sign in again on every device.")),
					Button(Type("button"), css.Class("btn btn-secondary"), OnClick(revoke), Text("Revoke sessions")),
				),
				Div(css.Class("action-card"),
					Div(css.Class("action-desc"), Text(suspendCardDesc)),
					Button(Type("button"), css.Class(suspendBtnClass), OnClick(toggleSuspend), Text(suspendBtnLabel)),
				),
				Div(css.Class("action-card action-danger"),
					Div(css.Class("action-desc"), Text("Irreversibly remove this account and all server-side data.")),
					deleteBlock,
				),
			),
			Div(css.Class("manage-col"),
				H2(css.Class("section-title"), Text("Usage — last 14 days")),
				usageList,
			),
		),
	)
}

// trimDate shortens an RFC3339 timestamp to its date portion.
func trimDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

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

	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// manageCSS styles the user-management UI. It is injected from Go (once, guarded by
// element id) rather than added to web/admin/index.html so the management layer is
// fully self-contained in this file and doesn't conflict with the console shell.
const manageCSS = `
.user-row{cursor:pointer;transition:background .12s ease}
.user-row:hover{background:rgba(255,255,255,0.06)}
.user-row:focus-visible{outline:2px solid #6366f1;outline-offset:-2px}
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
.field-row input{background:#0f1115;border:1px solid rgba(255,255,255,0.14);border-radius:8px;color:#e8eaed;padding:.5rem .6rem;font-size:14px}
.confirm-delete{display:flex;flex-direction:column;gap:.5rem}
.confirm-delete span{color:#fca5a5;font-size:13px}
.usage-list{display:flex;flex-direction:column;gap:.4rem}
.usage-bar-row{display:grid;grid-template-columns:84px 1fr auto;gap:.6rem;align-items:center;font-size:12px;color:#c7ccd3}
.usage-day{color:#9aa0aa}
.usage-track{background:rgba(255,255,255,0.06);border-radius:999px;height:10px;overflow:hidden}
.usage-fill{background:linear-gradient(90deg,#6366f1,#8b5cf6);height:100%}
.usage-num{white-space:nowrap;color:#9aa0aa}
.usage-empty{color:#9aa0aa;font-size:13px}
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
	SubscriptionPlan   string `json:"subscriptionPlan"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	CurrentPeriodEnd   string `json:"currentPeriodEnd"`
	TrialEnd           string `json:"trialEnd"`
	WorkspaceCount     int    `json:"workspaceCount"`
	BlobBytes          int64  `json:"blobBytes"`
	UsageTodayRequests int64  `json:"usageTodayRequests"`
	UsageTodayTokens   int64  `json:"usageTodayTokens"`
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

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// adminDo performs a bearer-authenticated request and returns the status and body.
func adminDo(token, method, url, body string) (int, []byte, error) {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
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

// ---------------------------------------------------------------------------
// Clickable users table (replaces the static table in readyView)
// ---------------------------------------------------------------------------

type userRowProps struct {
	user   adminUserRow
	onOpen func(string)
}

// userRow is its own component so it can own an OnClick hook safely — the framework
// rule forbids registering On* handlers inside a variable-length Map loop.
func userRow(p userRowProps) ui.Node {
	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	open := ui.UseEvent(func() { p.onOpen(p.user.ID) })
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
	return Tr(
		css.Class("user-row"),
		Attr("role", "button"),
		Attr("tabindex", "0"),
		Attr("aria-label", "Manage "+p.user.Email),
		OnClick(open),
		Td(Text(p.user.Email)),
		Td(Text(p.user.Provider)),
		Td(Text(plan)),
		Td(Text(status)),
		Td(Text(created)),
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
					Th(Text("Email")),
					Th(Text("Provider")),
					Th(Text("Plan")),
					Th(Text("Status")),
					Th(Text("Created")),
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
	planInput := ui.UseState("")
	statusInput := ui.UseState("")
	confirmDelete := ui.UseState(false)
	reload := ui.UseState(0)

	token, id := p.token, p.userID
	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")
	closeHandler := ui.UseEvent(p.onClose)

	// Fetch detail + usage whenever the target user or a reload tick changes.
	ui.UseEffect(func() func() {
		go func() {
			d, err := fetchUserDetail(token, id)
			if err != nil {
				status.Set("Could not load user: " + err.Error())
				return
			}
			detail.Set(d)
			planInput.Set(d.SubscriptionPlan)
			// Default the status picker to a real option so it reflects what will be
			// sent — an empty status (no subscription yet) would otherwise show the
			// first <option> while the state held "".
			if strings.TrimSpace(d.SubscriptionStatus) == "" {
				statusInput.Set("active")
			} else {
				statusInput.Set(d.SubscriptionStatus)
			}
			if u, err := fetchUserUsage(token, id, 14); err == nil {
				usage.Set(u)
			}
		}()
		return nil
	}, id, reload.Get())

	onPlanInput := ui.UseEvent(func(v string) { planInput.Set(v) })
	onStatusInput := ui.UseEvent(func(v string) { statusInput.Set(v) })

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
	if d == nil {
		summary = Div(css.Class("detail-card"), Text("Loading…"))
	} else {
		summary = Div(css.Class("detail-card"),
			detailRow("User ID", d.ID),
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
				H2(css.Class("section-title"), Text("Actions")),
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

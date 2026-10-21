// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// sessionFamily mirrors one entry of the server's GET /v1/auth/refresh/sessions response.
type sessionFamily struct {
	FamilyID  string `json:"familyId"`
	ExpiresAt string `json:"expiresAt"`
	Current   bool   `json:"current,omitempty"`
}

type sessionsResponse struct {
	Sessions []sessionFamily `json:"sessions"`
}

func fetchSessions(endpoint, token string, onDone func([]sessionFamily), onError func(error)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		return
	}
	go func() {
		headers := js.Global().Get("Headers").New()
		headers.Call("set", "Authorization", "Bearer "+token)
		opts := js.Global().Get("Object").New()
		opts.Set("method", "GET")
		opts.Set("credentials", "include")
		opts.Set("headers", headers)
		status, body, err := fetchResponseBody(endpoint+"/v1/auth/refresh/sessions", opts, "load devices")
		if err != nil {
			onError(err)
			return
		}
		if status < 200 || status >= 300 {
			onError(fmt.Errorf("load devices failed: HTTP %d", status))
			return
		}
		var sr sessionsResponse
		if err := json.Unmarshal([]byte(body), &sr); err != nil {
			onError(fmt.Errorf("decode device list: %w", err))
			return
		}
		onDone(sr.Sessions)
	}()
}

func revokeSession(endpoint, token, csrf, family string, onDone func(error)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	csrf = strings.TrimSpace(csrf)
	if endpoint == "" || token == "" || strings.TrimSpace(family) == "" {
		onDone(fmt.Errorf("revoke device requires a server, access token and device ID"))
		return
	}
	if csrf == "" {
		onDone(fmt.Errorf("revoke device requires the CSRF security cookie"))
		return
	}
	go func() {
		headers := js.Global().Get("Headers").New()
		headers.Call("set", "Authorization", "Bearer "+token)
		headers.Call("set", "X-CashFlux-CSRF", csrf)
		opts := js.Global().Get("Object").New()
		opts.Set("method", "DELETE")
		opts.Set("credentials", "include")
		opts.Set("headers", headers)
		status, _, err := fetchResponseBody(endpoint+"/v1/auth/refresh/sessions/"+url.PathEscape(family), opts, "revoke device")
		if err == nil && (status < 200 || status >= 300) {
			err = fmt.Errorf("revoke device failed: HTTP %d", status)
		}
		onDone(err)
	}()
}

func fetchResponseBody(endpoint string, opts js.Value, operation string) (int, string, error) {
	if err := validateBackendTransport(endpoint); err != nil {
		return 0, "", fmt.Errorf("%s: %w", operation, err)
	}
	type result struct {
		status int
		body   string
		err    error
	}
	done := make(chan result, 1)
	var received, decoded, failed js.Func
	var status int
	received = js.FuncOf(func(_ js.Value, args []js.Value) any {
		response := args[0]
		status = response.Get("status").Int()
		response.Call("text").Call("then", decoded).Call("catch", failed)
		return nil
	})
	decoded = js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{status: status, body: args[0].String()}
		return nil
	})
	failed = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		done <- result{err: fmt.Errorf("%s request could not reach the server", operation)}
		return nil
	})
	defer received.Release()
	defer decoded.Release()
	defer failed.Release()
	js.Global().Call("fetch", endpoint, opts).Call("then", received).Call("catch", failed)
	res := <-done
	return res.status, res.body, res.err
}

type deviceRowProps struct {
	Session  sessionFamily
	OnRevoke func(string)
}

// deviceRow renders one session/device with a revoke button. Its own component so
// the revoke OnClick hook stays at a stable position inside the list loop.
func deviceRow(p deviceRowProps) uic.Node {
	s := p.Session
	short := s.FamilyID
	if len(short) > 10 {
		short = short[:10] + "…"
	}
	meta := uistate.T("devices.expires", s.ExpiresAt)
	onRevoke := uic.UseEvent(func() { p.OnRevoke(s.FamilyID) })

	row := []any{css.Class("row")}
	row = append(row, Div(css.Class("row-main"),
		Span(css.Class("row-desc"), short),
		Span(css.Class("row-meta"), meta),
	))
	if s.Current {
		row = append(row, Span(css.Class("badge", tw.TextDim), uistate.T("devices.current")))
	} else {
		row = append(row, Button(css.Class("btn", "btn-sm", "btn-del"), Type("button"),
			Attr("aria-label", uistate.T("devices.revoke")), Title(uistate.T("devices.revoke")),
			OnClick(onRevoke), uistate.T("devices.revoke")))
	}
	return Div(row...)
}

// DevicesList shows the signed-in user's active sessions/devices with per-device
// revoke (§7.11), backed by GET /v1/auth/refresh/sessions and DELETE
// /v1/auth/refresh/sessions/{family}. Self-hides when not signed in or
// when there are no sessions. Intended for the Cloud settings surface.
func DevicesList() uic.Node {
	prefsAtom := uistate.UsePrefs()
	notice := uistate.UseNotice()
	notify := func(message string, bad bool) { notice.Set(notice.Get().With(message, bad)) }
	sessions := uic.UseState([]sessionFamily{})
	loaded := uic.UseState(false)
	rev := uic.UseState(0)

	pr := prefsAtom.Get().Normalize()
	endpoint, token := pr.ServerURL, pr.ServerToken
	signedIn := strings.TrimSpace(endpoint) != "" && strings.TrimSpace(token) != ""

	uic.UseEffect(func() func() {
		if signedIn {
			fetchSessions(endpoint, token, func(list []sessionFamily) { sessions.Set(list); loaded.Set(true) }, func(error) {
				notify("Could not load devices. Please try again.", true)
			})
		}
		return nil
	}, endpoint+"\x00"+token+"\x00"+itoa(rev.Get()))

	// Loading state (§7.11 Cloud a11y): while the first fetch is in flight, show a
	// polite skeleton instead of nothing, so the list doesn't pop in silently.
	if signedIn && !loaded.Get() {
		return Div(css.Class(tw.Mt3),
			H4(css.Class("set-label"), uistate.T("devices.title")),
			uiw.Skeleton(uiw.SkeletonProps{Lines: 2, AriaLabel: uistate.T("devices.loading")}),
		)
	}

	list := sessions.Get()
	if len(list) == 0 {
		return Fragment()
	}

	onRevoke := func(family string) {
		revokeSession(endpoint, token, cookieSessionCSRF(prefs.Prefs{ServerURL: endpoint}), family, func(err error) {
			if err != nil {
				notify("Could not sign out that device. Please try again.", true)
				return
			}
			rev.Set(rev.Get() + 1)
		})
	}
	rows := make([]any, 0, len(list)+1)
	rows = append(rows, css.Class("rows"))
	for _, s := range list {
		rows = append(rows, uic.CreateElement(deviceRow, deviceRowProps{Session: s, OnRevoke: onRevoke}))
	}
	return Div(css.Class(tw.Mt3),
		H4(css.Class("set-label"), uistate.T("devices.title")),
		Div(rows...),
	)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// sessionFamily mirrors one entry of the server's GET /v1/auth/sessions response.
type sessionFamily struct {
	FamilyID  string `json:"familyId"`
	ExpiresAt string `json:"expiresAt"`
	Current   bool   `json:"current,omitempty"`
}

type sessionsResponse struct {
	Sessions []sessionFamily `json:"sessions"`
}

func fetchSessions(endpoint, token string, onDone func([]sessionFamily)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/auth/sessions", nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return
		}
		var sr sessionsResponse
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			return
		}
		onDone(sr.Sessions)
	}()
}

func revokeSession(endpoint, token, family string, onDone func()) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" || family == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodDelete, endpoint+"/v1/auth/sessions/"+url.PathEscape(family), nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
		onDone()
	}()
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
// revoke (§7.11), backed by the existing GET /v1/auth/sessions + DELETE
// /v1/auth/sessions/{family} server endpoints. Self-hides when not signed in or
// when there are no sessions. Intended for the Cloud settings surface.
func DevicesList() uic.Node {
	prefsAtom := uistate.UsePrefs()
	sessions := uic.UseState([]sessionFamily{})
	loaded := uic.UseState(false)
	rev := uic.UseState(0)

	pr := prefsAtom.Get().Normalize()
	endpoint, token := pr.ServerURL, pr.ServerToken
	signedIn := strings.TrimSpace(endpoint) != "" && strings.TrimSpace(token) != ""

	uic.UseEffect(func() func() {
		if signedIn {
			fetchSessions(endpoint, token, func(list []sessionFamily) { sessions.Set(list); loaded.Set(true) })
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
		revokeSession(endpoint, token, family, func() { rev.Set(rev.Get() + 1) })
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

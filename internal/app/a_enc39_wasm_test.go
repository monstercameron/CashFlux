// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

type enc39Transport struct{ account string }

func (s enc39Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body string
	switch req.URL.Path {
	case "/v1/version":
		body = `{"serverId":"enc39-server"}`
	case "/v1/me":
		value, _ := json.Marshal(map[string]string{"userId": s.account})
		body = string(value)
	default:
		return nil, errors.New("unexpected ENC3.9 HTTP request: " + req.URL.Path)
	}
	return enc35Response(body), nil
}

func enc39WaitUntil(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func TestENC39ActivationHandoffSurvivesReloadWithoutPrefsToken(t *testing.T) {
	if !browserstore.Durable() {
		t.Fatal("ENC31 IndexedDB/Web Crypto fixture is unavailable")
	}
	oldTransport, oldLocation, oldHistory := http.DefaultTransport, js.Global().Get("location"), js.Global().Get("history")
	http.DefaultTransport = enc39Transport{account: "enc39-handoff-account"}
	js.Global().Set("location", map[string]any{"origin": "https://app.test", "href": "https://app.test/?activate=123456", "search": "?activate=123456"})
	js.Global().Set("history", map[string]any{"replaceState": js.Global().Get("Function").New("return function(){}")})
	oldDoc := js.Global().Get("document")
	enc31FakeDocument(t)
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		js.Global().Set("location", oldLocation)
		js.Global().Set("history", oldHistory)
		js.Global().Set("document", oldDoc)
		enc35ResetPage()
	})
	if code := handoffCodeFromURL(); code != "123456" {
		t.Fatalf("activation code = %q", code)
	}
	pr := prefs.Default()
	pr.ServerURL = "https://sync.test"
	pr.BackendDisabled = true
	uistate.PersistPrefs(pr)
	// Use a token pair from the activation redemption response and pass it
	// through the same durable sign-in boundary used by the handoff caller.
	pair := backendrpc.TokenPairResponse{AccessToken: "enc39-handoff-access", RefreshToken: "enc39-handoff-refresh", ExpiresInSeconds: 900}
	var atom state.Atom[prefs.Prefs]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node { atom = uistate.UsePrefs(); return Div() }, nil))
	f.Stabilize()
	atom.Set(pr)
	if err := enc33Result(t, func(finish func(error)) {
		persistAuthSessionWithCommitThen(pr, pr.ServerURL, pair, false, func(next prefs.Prefs, committed func(error)) {
			raw, err := json.Marshal(next.Normalize())
			if err != nil {
				committed(err)
				return
			}
			browserstore.SetSecure("cashflux:prefs", string(raw), func(err error) {
				if err == nil {
					atom.Set(next.Normalize())
					uistate.PersistPrefs(next.Normalize())
				}
				committed(err)
			})
		}, finish)
	}); err != nil {
		t.Fatal(err)
	}
	if got := uistate.LoadPrefs().ServerToken; got != "" {
		t.Fatalf("rotating access token leaked into preferences: %q", got)
	}
	scope := enc33Scope(t, "enc39-server", "enc39-handoff-account")
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || !have {
		t.Fatalf("activation credential was not durable: %v", err)
	}
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !backendSessionReady() || lsGet(authAccessTokenKey) == "" || uistate.LoadPrefs().ServerToken != "" {
		t.Fatal("reload lost the signed-in session or exposed a token in preferences")
	}
}

func TestENC39EraseRevokeIsBestEffortAndLocalCredentialDeleteIsRequired(t *testing.T) {
	if !browserstore.Durable() {
		t.Fatal("ENC31 IndexedDB/Web Crypto fixture is unavailable")
	}
	oldTransport, oldDocument, oldLocation := http.DefaultTransport, js.Global().Get("document"), js.Global().Get("location")
	http.DefaultTransport = enc39Transport{account: "enc39-wipe-account"}
	enc31FakeDocument(t)
	document := js.Global().Get("document")
	document.Set("baseURI", "https://app.test/")
	document.Set("querySelector", js.Global().Get("Function").New("return {}"))
	reloads := make(chan struct{}, 2)
	reload := js.FuncOf(func(js.Value, []js.Value) any { reloads <- struct{}{}; return nil })
	js.Global().Set("location", map[string]any{"origin": "https://app.test", "reload": reload})
	t.Cleanup(func() {
		reload.Release()
		http.DefaultTransport = oldTransport
		js.Global().Set("document", oldDocument)
		js.Global().Set("location", oldLocation)
		enc35ResetPage()
	})
	scope := enc33Scope(t, "enc39-wipe-server", "enc39-wipe-account")
	pair := backendrpc.TokenPairResponse{AccessToken: "enc39-wipe-access", RefreshToken: "enc39-wipe-refresh", ExpiresInSeconds: 900}
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "enc39-keep", Name: "Household data", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	oldApp := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = oldApp })
	pr := prefs.Default()
	pr.ServerURL = scope.Endpoint
	var atom state.Atom[prefs.Prefs]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node { atom = uistate.UsePrefs(); return Div() }, nil))
	f.Stabilize()
	atom.Set(pr)
	uistate.PersistPrefs(pr)
	captureAuthPrefsAtom(atom)
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, pair, done) }); err != nil {
		t.Fatal(err)
	}
	pageRefreshState.Lock()
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = &scope, true, false
	pageRefreshState.Unlock()
	if cfg := js.Global().Get("__enc35RPC"); cfg.Truthy() {
		cfg.Set("error", "offline")
	}
	var notices []string
	notify := func(message string, _ bool) { notices = append(notices, message) }
	startRPC := js.Global().Get("__enc35RPC").Get("calls").Length()
	eraseDataThen(notify)
	select {
	case <-reloads:
	case <-time.After(15 * time.Second):
		t.Fatal("erase did not continue after the best-effort server revoke failed")
	}
	if len(notices) != 1 || notices[0] != uistate.T("enc39.revokeFailed") {
		t.Fatalf("server-revoke notice = %q", notices)
	}
	logoutAttempted := false
	calls := js.Global().Get("__enc35RPC").Get("calls")
	for i := startRPC; i < calls.Length(); i++ {
		if strings.HasSuffix(calls.Index(i).Get("method").String(), "/Logout") {
			logoutAttempted = true
			break
		}
	}
	if !logoutAttempted {
		t.Fatal("erase did not attempt server revoke before its reload boundary")
	}
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || have {
		t.Fatalf("server failure retained local credential: %v", err)
	}
	if len(app.Accounts()) != 0 {
		t.Fatal("erase did not wipe household data after local credential deletion")
	}

	if err := app.PutAccount(domain.Account{ID: "enc39-keep", Name: "Household data", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, pair, done) }); err != nil {
		t.Fatal(err)
	}
	pageRefreshState.Lock()
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = &scope, true, false
	pageRefreshState.Unlock()
	browserstore.Barrier()
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if cfg := js.Global().Get("__enc35RPC"); cfg.Truthy() {
		cfg.Set("error", "")
	}
	failureNotice := make(chan string, 1)
	eraseDataThen(func(message string, isErr bool) {
		if isErr {
			failureNotice <- message
		}
	})
	select {
	case message := <-failureNotice:
		if !strings.Contains(message, "Couldn't erase your data") {
			t.Fatalf("local-delete failure notice = %q", message)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("local credential deletion failure was not reported")
	}
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || !have {
		t.Fatalf("failed local delete lost credential: %v", err)
	}
	if len(app.Accounts()) != 1 {
		t.Fatal("failed local credential delete did not preserve household data")
	}
	select {
	case <-reloads:
		t.Fatal("failed local credential deletion reloaded after a partial erase")
	default:
	}
}

func TestENC39RecoveryPersistenceFailureRetainsPairAndCode(t *testing.T) {
	if !browserstore.Durable() {
		t.Fatal("ENC31 IndexedDB/Web Crypto fixture is unavailable")
	}
	pr := prefs.Default()
	pr.ServerURL = "https://sync.test"
	oldTransport := http.DefaultTransport
	oldLocation := js.Global().Get("location")
	http.DefaultTransport = enc39Transport{account: "enc39-recovery-account"}
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		js.Global().Set("location", oldLocation)
	})
	pair := backendrpc.TokenPairResponse{AccessToken: "enc39-recovery-access", RefreshToken: "enc39-recovery-refresh", ExpiresInSeconds: 900}
	code := "enc39-one-time-recovery-code"
	oldPrefs := uistate.LoadPrefs()
	oldSuspend := suspendAutosave
	suspendAutosave = true
	t.Cleanup(func() {
		uistate.PersistPrefs(oldPrefs)
		suspendAutosave = oldSuspend
	})
	browserstore.Barrier()
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	completed := make(chan error, 1)
	persistAuthSessionWithCommitThen(pr, pr.ServerURL, pair, false, func(next prefs.Prefs, done func(error)) {
		done(nil)
	}, func(err error) { completed <- err })
	persistErr := enc39Receive(t, completed)
	if persistErr == nil {
		t.Fatal("injected durable credential failure was acknowledged")
	}
	if pair.AccessToken == "" || code == "" {
		t.Fatal("failed persistence dropped pending recovery material")
	}
	if strings.Contains(uistate.LoadPrefs().ServerToken, pair.AccessToken) {
		t.Fatal("failed recovery persistence exposed a token in preferences")
	}
	// The component callback is the owner of the one-time code and provisional
	// pair. Guard its error return before either value is cleared, alongside the
	// durable-write failure exercised above.
	source, err := os.ReadFile("pendingdevicecard.go")
	if err != nil {
		t.Fatal(err)
	}
	callback := string(source)
	start := strings.Index(callback, "onDismissRecovery :=")
	if start < 0 {
		t.Fatal("pending-device recovery dismissal callback is missing")
	}
	callback = callback[start:]
	failure := strings.Index(callback, "if err != nil")
	returned := strings.Index(callback, "return")
	clearCode := strings.Index(callback, `setPwRecovery.Set("")`)
	clearPair := strings.Index(callback, "pendingPair.Set(backendrpc.TokenPairResponse{})")
	if failure < 0 || returned < failure || clearCode < returned || clearPair < returned {
		t.Fatal("recovery code or pending pair can be cleared before persistence succeeds")
	}
}

func enc39Receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case value := <-c:
		return value
	case <-time.After(15 * time.Second):
		var zero T
		t.Fatal("ENC3.9 lifecycle callback did not settle")
		return zero
	}
}

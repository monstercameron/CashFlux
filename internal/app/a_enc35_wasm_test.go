// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/rpcworker"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type enc35Transport struct {
	failVersion bool
	account     string
}

func (s *enc35Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/v1/version") {
		if s.failVersion {
			return nil, errors.New("offline")
		}
		return enc35Response(`{"serverId":"enc35-server"}`), nil
	}
	if strings.HasSuffix(req.URL.Path, "/v1/me") {
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer enc35-") {
			return nil, errors.New("new access token was not used")
		}
		raw, _ := json.Marshal(map[string]string{"userId": s.account})
		return enc35Response(string(raw)), nil
	}
	return nil, errors.New("unexpected HTTP request")
}
func enc35Response(raw string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(raw))}
}

func enc35ResetPage() {
	stopProactiveRefreshTimer()
	stopBackendWatch()
	pageRefreshState.Lock()
	pageRefreshState.scope, pageRefreshState.ready, pageRefreshState.blocked = nil, false, false
	pageRefreshState.Unlock()
	clearRefreshMemory()
}

// enc35Storage gives each test that persists scoped credentials its own
// initialized Node IndexedDB fixture and clears any one-shot fault left by a
// prior test in the shared wasm process.
func enc35Storage(t *testing.T) {
	t.Helper()
	solp22Storage(t)
	browserstore.Barrier()
	control := js.Global().Get("__fakeIDB")
	resetFaults := func() {
		for _, field := range []string{"_failNext", "_failNextWrite", "_failNextOpen", "_failNextTransaction"} {
			control.Set(field, js.Null())
		}
	}
	resetFaults()
	// Credential persistence lazily creates a device ID. Complete that write
	// before tests install their own one-shot IndexedDB failures.
	syncDeviceID()
	browserstore.Barrier()
	t.Cleanup(func() {
		browserstore.Barrier()
		resetFaults()
	})
}

// Uses the page-wide ENC31 storage fixture, rather than installing a second KV
// store. All requests are local mocks; no server or paid API is contacted.
func TestENC35LiveSessionBoundaries(t *testing.T) {
	enc35Storage(t)
	if !browserstore.Durable() {
		t.Fatal("page storage fixture is unavailable")
	}
	oldApp, oldTransport := appstate.Default, http.DefaultTransport
	oldLocation, oldDocument, oldFetch, oldWorker := js.Global().Get("location"), js.Global().Get("document"), js.Global().Get("fetch"), js.Global().Get("Worker")
	oldNavigator := js.Global().Get("navigator")
	appstate.Default = nil
	transport := &enc35Transport{account: "enc35-account-a"}
	http.DefaultTransport = transport
	enc33SetGlobal("navigator", js.Global().Get("Object").New())
	enc31FakeDocument(t)
	document := js.Global().Get("document")
	document.Set("baseURI", "https://app.test/")
	document.Set("cookie", "cashflux_csrf=enc35-csrf")
	document.Set("querySelector", js.Global().Get("Function").New("return {}"))
	js.Global().Set("location", map[string]any{"origin": "https://app.test", "href": "https://app.test/settings/cloud"})
	js.Global().Call("eval", `globalThis.__enc35RPC={calls:[],error:"",next:1};
 globalThis.Worker=class {
 constructor(){setTimeout(()=>this.onmessage({data:{version:2,kind:"ready"}}),0)}
 postMessage(m){
  if(m.kind!=="unary")return;
  const cfg=globalThis.__enc35RPC;cfg.calls.push(m);
  let reply;
  if(cfg.error)reply={version:2,kind:"failure",id:m.id,error:{code:cfg.error,message:"mock failure",mayHaveApplied:false}};
  else if(m.method.endsWith("/RefreshToken")) {let n=cfg.next++;reply={version:2,kind:"reply",id:m.id,payload:JSON.stringify({accessToken:"enc35-access-"+n,refreshToken:"enc35-refresh-"+n,expiresInSeconds:900})}}
  else reply={version:2,kind:"reply",id:m.id,payload:"{}"};
  setTimeout(()=>this.onmessage({data:reply}),0);
 }
 };`)
	if _, err := rpcworker.StartDefault(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		browserstore.Barrier()
		enc35ResetPage()
		appstate.Default, http.DefaultTransport = oldApp, oldTransport
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
		js.Global().Set("fetch", oldFetch)
		js.Global().Set("Worker", oldWorker)
		enc33SetGlobal("navigator", oldNavigator)
	})
	var atom state.Atom[prefs.Prefs]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node { atom = uistate.UsePrefs(); return Div() }, nil))
	f.Stabilize()
	base := prefs.Default()
	base.ServerURL = "https://remote.test"
	base.ServerMode = prefs.ServerSelfHosted
	base.BackendDisabled = true
	atom.Set(base)
	uistate.PersistPrefs(base)
	first := backendrpc.TokenPairResponse{AccessToken: "enc35-login-access", RefreshToken: "enc35-login-refresh", ExpiresInSeconds: 900}
	if err := enc33Result(t, func(done func(error)) { persistAuthSessionThen(atom, base.ServerURL, first, false, done) }); err != nil {
		t.Fatal(err)
	}
	if atom.Get().ServerToken != "" || atom.Get().ServerCSRF != "" {
		t.Fatal("rotating credential reached prefs")
	}
	scope := enc33Scope(t, "enc35-server", "enc35-account-a")
	scope.Endpoint = base.ServerURL
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || !have {
		t.Fatalf("password credential not durable: %v", err)
	}
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !backendSessionReady() || lsGet(authAccessTokenKey) != "enc35-access-1" {
		t.Fatal("reload did not refresh before enabling session")
	}
	transport.account = "enc35-foreign-account"
	if adoptIdentityForSync(scope.Endpoint, lsGet(authAccessTokenKey)) || signedInUserID() != scope.AccountID {
		t.Fatal("identity adoption crossed the fixed account boundary")
	}
	transport.account = scope.AccountID
	foreign := atom.Get()
	foreign.ServerURL = "https://foreign.test"
	foreign.ServerToken = ""
	beforeForeign := js.Global().Get("__enc35RPC").Get("calls").Length()
	if effectiveServerToken(foreign) != "" || invokeAuthed(context.Background(), foreign, backendrpc.MethodAuthListDevices, backendrpc.ListDevicesRequest{}, &backendrpc.ListDevicesResponse{}) == nil {
		t.Fatal("rotating access escaped the fixed server endpoint")
	}
	if js.Global().Get("__enc35RPC").Get("calls").Length() != beforeForeign {
		t.Fatal("foreign endpoint received an authenticated RPC")
	}

	pr := atom.Get()
	pr.BackendDisabled = true
	if !refreshAccessToken(context.Background(), pr) || lsGet(authAccessTokenKey) != "enc35-access-2" {
		t.Fatal("runtime refresh failed")
	}
	proactiveRefreshMu.Lock()
	armed := proactiveRefreshTimer != nil
	proactiveRefreshMu.Unlock()
	if !armed {
		t.Fatal("encrypted refresh did not rearm timer")
	}
	// Transient failure preserves the durable credential and page identity.
	js.Global().Get("__enc35RPC").Set("error", "Unavailable")
	if refreshAccessToken(context.Background(), pr) {
		t.Fatal("offline refresh succeeded")
	}
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || !have {
		t.Fatal("offline refresh deleted credential")
	}
	js.Global().Get("__enc35RPC").Set("error", "")
	other := scope
	other.AccountID = "enc35-account-b"
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, other, first, done) }); err != nil {
		t.Fatal(err)
	}
	if err := enc33Result(t, func(done func(error)) { endPageSessionThen(pr, done) }); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || have {
		t.Fatal("sign-out retained outgoing credential")
	}
	if _, _, have, err := (scopedRefreshSession{other}).load(); err != nil || !have {
		t.Fatal("sign-out deleted another account")
	}
	// Missing credential must never enable sync or a watch.
	enc35ResetPage()
	if err := enc33Result(t, func(done func(error)) { saveSessionRoutingThen(scope, done) }); err != nil {
		t.Fatal(err)
	}
	if err := bootstrapSelectedSession(context.Background()); err == nil || backendSessionReady() {
		t.Fatal("missing credential enabled sync")
	}
	before := js.Global().Get("__enc35RPC").Get("calls").Length()
	startBackendSync()
	startBackendWatch()
	flushBackendSyncQueue()
	if js.Global().Get("__enc35RPC").Get("calls").Length() != before {
		t.Fatal("failed boot started data RPCs")
	}
	// An unavailable stable identity cannot fall back to the endpoint string.
	enc35ResetPage()
	transport.failVersion = true
	if err := bootstrapSelectedSession(context.Background()); err == nil || backendSessionReady() {
		t.Fatal("offline version invented a server identity")
	}
	transport.failVersion = false
	transport.account = scope.AccountID
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, first, done) }); err != nil {
		t.Fatal(err)
	}
	if err := enc33Result(t, func(done func(error)) { saveSessionRoutingThen(scope, done) }); err != nil {
		t.Fatal(err)
	}
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	transport.account = other.AccountID
	if err := enc33Result(t, func(done func(error)) { persistAuthSessionThen(atom, base.ServerURL, first, false, done) }); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := (scopedRefreshSession{scope}).load(); err != nil || have {
		t.Fatal("account switch retained outgoing credential")
	}
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if selected := outgoingRefreshScope(); selected == nil || selected.AccountID != other.AccountID {
		t.Fatal("reload used previous account")
	}
	transport.account = "enc35-foreign-account"
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err == nil || backendSessionReady() || lsGet(authAccessTokenKey) != "" {
		t.Fatal("refreshed access for another account enabled sync")
	}
	transport.account = other.AccountID
	enc35ResetPage()
	if err := bootstrapSelectedSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded := make(chan struct{}, 1)
	reload := js.FuncOf(func(js.Value, []js.Value) any { reloaded <- struct{}{}; return nil })
	defer reload.Release()
	js.Global().Get("location").Set("reload", reload)
	js.Global().Get("__enc35RPC").Set("error", "Unauthenticated")
	if refreshAccessToken(context.Background(), pr) {
		t.Fatal("revoked refresh succeeded")
	}
	select {
	case <-reloaded:
	case <-time.After(10 * time.Second):
		t.Fatal("revocation never reached its awaited reload boundary")
	}
	js.Global().Get("__enc35RPC").Set("error", "")
	if _, _, have, err := (scopedRefreshSession{other}).load(); err != nil || have {
		t.Fatal("revocation reloaded before deleting scoped credential")
	}
	if lsGet(authAccessTokenKey) != "" {
		t.Fatal("revocation retained access in memory")
	}

	enc35Scan(t, []refreshScope{scope, other}, first.AccessToken, first.RefreshToken, "enc35-access-1", "enc35-access-2", "enc35-refresh-1", "enc35-refresh-2", "enc35-csrf")
}

func TestENC35CookieExchangeAndOAuthCode(t *testing.T) {
	enc35Storage(t)
	oldFetch, oldLocation, oldDocument := js.Global().Get("fetch"), js.Global().Get("location"), js.Global().Get("document")
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	js.Global().Set("document", map[string]any{"cookie": "cashflux_csrf=enc35-csrf"})
	t.Cleanup(func() {
		js.Global().Set("fetch", oldFetch)
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
		clearRefreshMemory()
		stopProactiveRefreshTimer()
	})
	mock := js.Global().Get("Function").New(`return {calls:[],account:"enc35-account", fetch(url,opts){
 this.calls.push({url,body:opts.body,credentials:opts.credentials,csrf:opts.headers.get("X-CashFlux-CSRF")});
 if(url.endsWith("/csrf"))return Promise.resolve({status:204,text:()=>Promise.resolve("")});
 let body=url.endsWith("/code")?{accessToken:"enc35-code-access",refreshToken:"enc35-code-refresh",expiresIn:900}:{accessToken:"enc35-cookie-access",userId:this.account,expiresIn:900};
 return Promise.resolve({status:200,text:()=>Promise.resolve(JSON.stringify(body))});
 }};`).Invoke()
	js.Global().Set("fetch", mock.Get("fetch").Call("bind", mock))
	scope, err := newRefreshScope("enc35-cookie-server", "enc35-account", "https://app.test", "https://app.test")
	if err != nil {
		t.Fatal(err)
	}
	pair := backendrpc.TokenPairResponse{AccessToken: "enc35-password-access", RefreshToken: "enc35-password-refresh", ExpiresInSeconds: 900}
	cookie, err := installSessionCookie(context.Background(), scope, pair)
	if err != nil || cookie.RefreshToken != "" || cookie.AccessToken != "enc35-cookie-access" {
		t.Fatalf("cookie exchange: %v", err)
	}
	calls := mock.Get("calls")
	if calls.Length() != 2 || calls.Index(1).Get("csrf").String() != "enc35-csrf" || calls.Index(1).Get("credentials").String() != "include" {
		t.Fatal("cookie exchange omitted bootstrap or CSRF")
	}
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, cookie, done) }); err != nil {
		t.Fatal(err)
	}
	mock.Set("account", "foreign")
	if _, err := installSessionCookie(context.Background(), scope, pair); err == nil {
		t.Fatal("cookie exchange accepted another account")
	}
	code, err := exchangeOAuthCode(context.Background(), "https://remote.test", "one-time-code", "memory-only-verifier", "https://app.test/settings/cloud")
	if err != nil || code.RefreshToken != "enc35-code-refresh" {
		t.Fatalf("code exchange: %v", err)
	}
	last := calls.Index(calls.Length() - 1)
	if last.Get("credentials").String() != "omit" || !strings.Contains(last.Get("body").String(), `"codeVerifier":"memory-only-verifier"`) {
		t.Fatal("code exchange did not use private verifier without cookies")
	}
	enc35Scan(t, []refreshScope{scope}, pair.AccessToken, pair.RefreshToken, cookie.AccessToken, code.AccessToken, code.RefreshToken, "enc35-csrf", "memory-only-verifier")
}

func TestENC35CanceledSessionExchangeSettles(t *testing.T) {
	old := js.Global().Get("fetch")
	t.Cleanup(func() { js.Global().Set("fetch", old) })
	mock := js.Global().Get("Function").New(`let settled=false;return {get settled(){return settled},fetch(url,opts){return new Promise((_,reject)=>opts.signal.addEventListener("abort",()=>setTimeout(()=>{settled=true;reject(Error("aborted"))},10)))}}`).Invoke()
	js.Global().Set("fetch", mock.Get("fetch"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sessionHTTP(ctx, "https://app.test", "/session", "", "", true, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if !mock.Get("settled").Bool() {
		t.Fatal("returned before fetch settled")
	}
}

func TestENC35OAuthPopupModesAndSourceBinding(t *testing.T) {
	oldWindow, oldLocation, oldFetch := js.Global().Get("window"), js.Global().Get("location"), js.Global().Get("fetch")
	t.Cleanup(func() {
		js.Global().Set("window", oldWindow)
		js.Global().Set("location", oldLocation)
		js.Global().Set("fetch", oldFetch)
	})
	js.Global().Set("location", map[string]any{"origin": "https://app.test", "href": "https://app.test/settings/cloud"})
	for _, endpoint := range []string{"https://app.test", "https://app.test:443", "https://remote.test"} {
		t.Run(endpoint, func(t *testing.T) {
			window := js.Global().Get("Function").New(`return {listeners:[],popup:{},loginURL:"",addEventListener(k,f){this.listeners.push(f)},removeEventListener(k,f){this.listeners=this.listeners.filter(x=>x!==f)},open(u){this.loginURL=u;return this.popup},emit(e){for(const f of [...this.listeners])f(e)}};`).Invoke()
			js.Global().Set("window", window)
			mock := js.Global().Get("Function").New(`return {body:null,fetch(url,opts){this.body=JSON.parse(opts.body);return Promise.resolve({status:200,text:()=>Promise.resolve(JSON.stringify({accessToken:"enc35-popup-access",refreshToken:"enc35-popup-refresh",expiresIn:900}))})}}`).Invoke()
			js.Global().Set("fetch", mock.Get("fetch").Call("bind", mock))
			done := make(chan backendrpc.TokenPairResponse, 2)
			failed := make(chan string, 2)
			startScopedOAuthLogin(endpoint, "google", func(pair backendrpc.TokenPairResponse, _ string) { done <- pair }, func(s string) { failed <- s })
			login, err := url.Parse(window.Get("loginURL").String())
			if err != nil {
				t.Fatal(err)
			}
			origin := js.Global().Get("URL").New(endpoint).Get("origin").String()
			data := map[string]any{"type": "cashflux.oauth", "accessToken": "enc35-popup-access", "csrf": "enc35-popup-csrf", "expiresIn": 900, "code": "one-use-popup-code"}
			window.Call("emit", map[string]any{"origin": origin, "source": js.Global().Get("Object").New(), "data": data})
			window.Call("emit", map[string]any{"origin": "https://attacker.test", "source": window.Get("popup"), "data": data})
			if len(done) != 0 || len(failed) != 0 || window.Get("listeners").Length() != 1 {
				t.Fatal("foreign popup source or origin was accepted")
			}
			window.Call("emit", map[string]any{"origin": origin, "source": window.Get("popup"), "data": data})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			select {
			case pair := <-done:
				if origin == "https://app.test" {
					if pair.RefreshToken != "" || login.Query().Get("code_challenge") != "" {
						t.Fatal("cookie OAuth returned a raw refresh credential")
					}
				} else {
					body := mock.Get("body")
					verifier := body.Get("codeVerifier").String()
					digest := sha256.Sum256([]byte(verifier))
					if pair.RefreshToken == "" || login.Query().Get("code_challenge_method") != "S256" || login.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(digest[:]) || body.Get("returnTo").String() != "https://app.test/settings/cloud" {
						t.Fatal("PKCE exchange was not bound to popup attempt and return URL")
					}
				}
			case message := <-failed:
				t.Fatal(message)
			case <-ctx.Done():
				t.Fatal("popup completion did not settle")
			}
			if window.Get("listeners").Length() != 0 {
				t.Fatal("popup listener survived completion")
			}
			window.Call("emit", map[string]any{"origin": origin, "source": window.Get("popup"), "data": data})
			if len(done) != 0 {
				t.Fatal("popup completed twice")
			}
		})
	}
}

func TestENC35ServerChangeAndLockedBoot(t *testing.T) {
	enc35Storage(t)
	// Keep the production page KV store, using unique scoped auth databases.
	a := enc33Scope(t, "enc35-change-server", "enc35-change-account")
	b := a
	b.ServerID = "enc35-unrelated-server"
	pair := backendrpc.TokenPairResponse{AccessToken: "enc35-change-access", RefreshToken: "enc35-change-refresh", ExpiresInSeconds: 900}
	for _, scope := range []refreshScope{a, b} {
		if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, pair, done) }); err != nil {
			t.Fatal(err)
		}
	}
	pageRefreshState.Lock()
	pageRefreshState.scope = &a
	pageRefreshState.ready = true
	pageRefreshState.blocked = false
	pageRefreshState.Unlock()
	t.Cleanup(enc35ResetPage)
	browserstore.Barrier()
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if err := enc33Result(t, func(done func(error)) { resetScopeForServerChangeThen(a, func(_ bool, err error) { done(err) }) }); err == nil {
		t.Fatal("failed outgoing delete acknowledged server change")
	}
	if _, _, have, err := (scopedRefreshSession{a}).load(); err != nil || !have {
		t.Fatal("failed server change lost credential")
	}
	if err := enc33Result(t, func(done func(error)) { resetScopeForServerChangeThen(a, func(_ bool, err error) { done(err) }) }); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := (scopedRefreshSession{a}).load(); err != nil || have {
		t.Fatal("server change retained outgoing scope")
	}
	if _, _, have, err := (scopedRefreshSession{b}).load(); err != nil || !have {
		t.Fatal("server change removed another scope")
	}
	// Lock the auth-only key independently of household App Lock. Reopening must
	// report recovery and never fabricate a replacement key or plaintext record.
	keyScope := keyring.Scope{ServerID: b.ServerID, AccountID: b.AccountID}
	keys, err := atrest.NewBrowserKeyStore(keyScope, refreshKeyWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	service, err := atrest.OpenKeyService(keys, keyScope, refreshKeyWorkspace, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnableAppLock("enc35-auth-lock"); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	keys.Close()
	clearRefreshMemory()
	if err := bootstrapScopedSession(context.Background(), b); !errors.Is(err, atrest.ErrLocked) {
		t.Fatalf("locked credential: %v", err)
	}
	if lsGet(authAccessTokenKey) != "" || lsGet(authRefreshTokenKey) != "" {
		t.Fatal("locked credential published session")
	}
	records, err := openRefreshRecords(b)
	if err != nil {
		t.Fatal(err)
	}
	raw, have, err := records.Read(b.key())
	records.db.Call("close")
	if err != nil || !have || !atrest.IsSealed(raw) {
		t.Fatal("locked credential was replaced or deleted")
	}
	enc35Scan(t, []refreshScope{a, b}, pair.AccessToken, pair.RefreshToken, "enc35-auth-lock")
}

// Scan every durable store touched by these lifecycle paths, including the
// household KV store used for routing/preferences, not only auth databases.
func enc35Scan(t *testing.T, scopes []refreshScope, secrets ...string) {
	t.Helper()
	browserstore.Barrier()
	enc33Scan(t, scopes, secrets...)
	local := keyring.LocalScope("")
	identity, _ := json.Marshal([]string{local.ServerID, local.AccountID, "legacy-browser-profile"})
	for _, name := range []string{"cashflux-kv", "cashflux-atrest-keys-v1:" + base64.RawURLEncoding.EncodeToString(identity)} {
		dump := js.Global().Get("__fakeIDB").Call("dump", name).String()
		for _, secret := range secrets {
			if strings.Contains(dump, secret) {
				t.Fatalf("plaintext sentinel in %s", name)
			}
		}
	}
}

func TestENC35CookieRuntimeAndSignOutScope(t *testing.T) {
	oldFetch, oldLocation, oldDocument := js.Global().Get("fetch"), js.Global().Get("location"), js.Global().Get("document")
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	js.Global().Set("document", map[string]any{"cookie": "cashflux_csrf=enc35-runtime-csrf"})
	t.Cleanup(func() {
		enc35ResetPage()
		js.Global().Set("fetch", oldFetch)
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
	})
	a, err := newRefreshScope("enc35-runtime-cookie-server", "enc35-cookie-a", "https://app.test", "https://app.test")
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.AccountID = "enc35-cookie-b"
	for _, scope := range []refreshScope{a, b} {
		if err := enc33Result(t, func(done func(error)) {
			saveScopedSessionThen(nil, scope, backendrpc.TokenPairResponse{AccessToken: "enc35-cookie-login", ExpiresInSeconds: 900}, done)
		}); err != nil {
			t.Fatal(err)
		}
	}
	if have, err := (scopedRefreshSession{a}).restore(); err != nil || !have {
		t.Fatal(err)
	}
	pageRefreshState.Lock()
	pageRefreshState.scope = &a
	pageRefreshState.ready = true
	pageRefreshState.blocked = false
	pageRefreshState.Unlock()
	mock := js.Global().Get("Function").New(`return {account:"enc35-cookie-a",logout:0,fetch(url,opts){
 if(url.endsWith("/logout")){this.logout++;return Promise.resolve({status:204,text:()=>Promise.resolve("")})}
 const body={accessToken:"enc35-cookie-runtime-access",userId:this.account,expiresIn:900};
 return Promise.resolve({status:200,headers:{get(){return "enc35-runtime-csrf"}},json:()=>Promise.resolve(body)});
 }};`).Invoke()
	js.Global().Set("fetch", mock.Get("fetch").Call("bind", mock))
	pr := prefs.Prefs{ServerURL: a.Endpoint, BackendDisabled: true}
	if !refreshAccessToken(context.Background(), pr) || lsGet(authAccessTokenKey) != "enc35-cookie-runtime-access" {
		t.Fatal("cookie runtime refresh failed")
	}
	proactiveRefreshMu.Lock()
	armed := proactiveRefreshTimer != nil
	proactiveRefreshMu.Unlock()
	if !armed {
		t.Fatal("cookie runtime refresh did not rearm timer")
	}
	mock.Set("account", b.AccountID)
	if refreshAccessToken(context.Background(), pr) {
		t.Fatal("runtime cookie accepted another account")
	}
	if err := enc33Result(t, func(done func(error)) { endPageSessionThen(pr, done) }); err != nil {
		t.Fatal(err)
	}
	if mock.Get("logout").Int() != 0 {
		t.Fatal("sign-out revoked another account's browser cookie")
	}
	if _, _, have, err := (scopedRefreshSession{a}).load(); err != nil || have {
		t.Fatal("sign-out retained outgoing cookie marker")
	}
	if _, _, have, err := (scopedRefreshSession{b}).load(); err != nil || !have {
		t.Fatal("sign-out removed other account's marker")
	}
	// A matching browser cookie is revoked through its narrow path before delete.
	if err := enc33Result(t, func(done func(error)) {
		saveScopedSessionThen(nil, a, backendrpc.TokenPairResponse{AccessToken: "enc35-cookie-login", ExpiresInSeconds: 900}, done)
	}); err != nil {
		t.Fatal(err)
	}
	mock.Set("account", a.AccountID)
	if err := enc33Result(t, func(done func(error)) { endPageSessionThen(pr, done) }); err != nil {
		t.Fatal(err)
	}
	if mock.Get("logout").Int() != 1 {
		t.Fatal("matching cookie session was not revoked")
	}
	enc35Scan(t, []refreshScope{a, b}, "enc35-cookie-login", "enc35-cookie-runtime-access", "enc35-runtime-csrf")
}

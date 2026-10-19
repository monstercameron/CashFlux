// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

// Node exposes navigator through a getter without a setter. Define the mock
// explicitly so these tests exercise Web Locks rather than the fallback path.
func enc33SetGlobal(name string, value any) {
	js.Global().Get("Object").Call("defineProperty", js.Global(), name, map[string]any{"value": value, "writable": true, "configurable": true})
}

func enc33Fixture(t *testing.T) {
	t.Helper()
	oldIDB, oldCrypto := js.Global().Get("indexedDB"), js.Global().Get("crypto")
	if !oldCrypto.Truthy() || !oldCrypto.Get("subtle").Truthy() {
		js.Global().Set("crypto", js.Global().Get("require").Invoke("node:crypto").Get("webcrypto"))
	}
	script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
	if err != nil {
		t.Fatal(err)
	}
	js.Global().Call("eval", string(script))
	oldNavigator := js.Global().Get("navigator")
	enc33SetGlobal("navigator", js.Global().Get("Object").New())
	t.Cleanup(func() {
		clearRefreshMemory()
		js.Global().Set("indexedDB", oldIDB)
		js.Global().Set("crypto", oldCrypto)
		enc33SetGlobal("navigator", oldNavigator)
	})
}

func enc33Result(t *testing.T, issue func(func(error))) error {
	t.Helper()
	done := make(chan error, 1)
	issue(func(err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("credential callback did not settle")
		return nil
	}
}

func enc33Scope(t *testing.T, server, account string) refreshScope {
	t.Helper()
	s, err := newRefreshScope(server, account, "https://sync.test", "https://app.test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func enc33Scan(t *testing.T, scopes []refreshScope, sentinels ...string) {
	t.Helper()
	for _, scope := range scopes {
		identity, _ := json.Marshal([]string{scope.ServerID, scope.AccountID, refreshKeyWorkspace})
		for _, dbName := range []string{refreshDatabaseName(scope), "cashflux-atrest-keys-v1:" + base64.RawURLEncoding.EncodeToString(identity)} {
			dump := js.Global().Get("__fakeIDB").Call("dump", dbName).String()
			for _, secret := range sentinels {
				if strings.Contains(dump, secret) {
					t.Fatalf("plaintext credential in %s", dbName)
				}
			}
		}
		records, err := openRefreshRecords(scope)
		if err != nil {
			t.Fatal(err)
		}
		keys, err := records.Keys()
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			raw, have, err := records.Read(key)
			if err != nil || !have || !atrest.IsSealed(raw) || key != scope.key() {
				t.Fatalf("unprotected record %s: %v", key, err)
			}
		}
		records.db.Call("close")
	}
	for _, name := range []string{"localStorage", "sessionStorage"} {
		storage := js.Global().Get(name)
		if !storage.Truthy() {
			continue
		}
		for i := 0; i < storage.Get("length").Int(); i++ {
			raw := storage.Call("getItem", storage.Call("key", i)).String()
			for _, secret := range sentinels {
				if strings.Contains(raw, secret) {
					t.Fatalf("plaintext credential in %s", name)
				}
			}
		}
	}
}

func TestENC33ScopedCredentialReloadRotationAndSignOut(t *testing.T) {
	enc33Fixture(t)
	scope := enc33Scope(t, "server-a", "account-a")
	s := scopedRefreshSession{scope: scope}
	pair := backendrpc.TokenPairResponse{AccessToken: "enc33-access-first", RefreshToken: "enc33-refresh-first", ExpiresInSeconds: 900}
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, pair, done) }); err != nil {
		t.Fatal(err)
	}
	enc33Scan(t, []refreshScope{scope}, pair.AccessToken, pair.RefreshToken)
	// Every operation closes its key service and connection. This drops page
	// memory too, then restore reopens the durable device wrap from IndexedDB.
	clearRefreshMemory()
	if have, err := s.restore(); err != nil || !have {
		t.Fatalf("reload: have=%v err=%v", have, err)
	}
	if lsGet(authAccessTokenKey) != "" || lsGet(authCSRFKey) != "" || lsGet(authRefreshTokenKey) != pair.RefreshToken {
		t.Fatal("boot restored more than the scoped refresh credential")
	}
	rotated := backendrpc.TokenPairResponse{AccessToken: "enc33-access-second", RefreshToken: "enc33-refresh-second", ExpiresInSeconds: 900}
	if err := s.refreshFirst(context.Background(), func(_ context.Context, refresh string) (backendrpc.TokenPairResponse, error) {
		if refresh != pair.RefreshToken || lsGet(authAccessTokenKey) != "" {
			t.Error("boot did not refresh first")
		}
		return rotated, nil
	}); err != nil {
		t.Fatal(err)
	}
	if lsGet(authAccessTokenKey) != rotated.AccessToken {
		t.Fatal("new access token missing from memory")
	}
	clearRefreshMemory()
	if have, err := s.restore(); err != nil || !have || lsGet(authRefreshTokenKey) != rotated.RefreshToken {
		t.Fatalf("rotation not durable across reload: %v", err)
	}
	enc33Scan(t, []refreshScope{scope}, pair.AccessToken, pair.RefreshToken, rotated.AccessToken, rotated.RefreshToken)
	if err := enc33Result(t, s.clearThen); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := s.load(); err != nil || have {
		t.Fatalf("sign-out retained credential: %v", err)
	}
	if lsGet(authRefreshTokenKey) != "" || lsGet(authAccessTokenKey) != "" {
		t.Fatal("sign-out retained memory tokens")
	}
}

func TestENC33ScopesNeverShareCredentialsAndSwitchDeletesOldScope(t *testing.T) {
	enc33Fixture(t)
	a := enc33Scope(t, "server-a", "account-a")
	b := enc33Scope(t, "server-a", "account-b")
	c := enc33Scope(t, "server-b", "account-a")
	for _, scope := range []refreshScope{a, b, c} {
		pair := backendrpc.TokenPairResponse{AccessToken: "enc33-access-" + scope.AccountID, RefreshToken: "enc33-refresh-" + scope.ServerID + scope.AccountID, ExpiresInSeconds: 900}
		if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(nil, scope, pair, done) }); err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range []refreshScope{a, b, c} {
		if have, err := (scopedRefreshSession{scope}).restore(); err != nil || !have || lsGet(authRefreshTokenKey) != "enc33-refresh-"+scope.ServerID+scope.AccountID {
			t.Fatalf("scope restored another credential: %v", err)
		}
	}
	missing := enc33Scope(t, "server-missing", "account-a")
	if have, err := (scopedRefreshSession{missing}).restore(); err != nil || have || lsGet(authRefreshTokenKey) != "" {
		t.Fatal("missing scope reused previous page memory")
	}
	pair := backendrpc.TokenPairResponse{AccessToken: "enc33-new-access", RefreshToken: "enc33-new-refresh", ExpiresInSeconds: 900}
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(&a, b, pair, done) }); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := (scopedRefreshSession{a}).load(); err != nil || have {
		t.Fatal("account change retained old credential")
	}
	if _, _, have, err := (scopedRefreshSession{c}).load(); err != nil || !have {
		t.Fatal("account change removed another server's credential")
	}
	if err := enc33Result(t, func(done func(error)) { saveScopedSessionThen(&b, c, pair, done) }); err != nil {
		t.Fatal(err)
	}
	if _, _, have, err := (scopedRefreshSession{b}).load(); err != nil || have {
		t.Fatal("server change retained old credential")
	}
	enc33Scan(t, []refreshScope{a, b, c}, "enc33-new-access", "enc33-new-refresh")
}

func TestENC33FailedCommitAndDeleteNeverAcknowledgeSuccess(t *testing.T) {
	enc33Fixture(t)
	s := scopedRefreshSession{enc33Scope(t, "server-a", "account-a")}
	if err := enc33Result(t, func(done func(error)) { s.saveThen("enc33-initial-refresh", "", done) }); err != nil {
		t.Fatal(err)
	}
	_, raw, _, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if err := enc33Result(t, func(done func(error)) { s.saveThen("enc33-failed-refresh", raw, done) }); err == nil {
		t.Fatal("failed save acknowledged")
	}
	c, _, _, err := s.load()
	if err != nil || c.Token != "enc33-initial-refresh" {
		t.Fatal("failed write changed durable credential")
	}
	if err := enc33Result(t, func(done func(error)) { s.saveThen("stale", "", done) }); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("stale write: %v", err)
	}
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if err := enc33Result(t, s.clearThen); err == nil {
		t.Fatal("failed delete acknowledged")
	}
	if _, _, have, err := s.load(); err != nil || !have {
		t.Fatal("failed delete lost credential")
	}
	clearRefreshMemory()
	err = s.refreshFirst(context.Background(), func(context.Context, string) (backendrpc.TokenPairResponse, error) {
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		return backendrpc.TokenPairResponse{AccessToken: "enc33-uncommitted-access", RefreshToken: "enc33-uncommitted-refresh", ExpiresInSeconds: 900}, nil
	})
	if err == nil || lsGet(authAccessTokenKey) != "" {
		t.Fatal("failed rotation published a new access token")
	}
	enc33Scan(t, []refreshScope{s.scope}, "enc33-initial-refresh", "enc33-failed-refresh", "enc33-uncommitted-access", "enc33-uncommitted-refresh")
}

func TestENC33CookieCSRFStaysInMemoryAndUsesLatestCookie(t *testing.T) {
	oldLocation, oldDocument := js.Global().Get("location"), js.Global().Get("document")
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	js.Global().Set("document", map[string]any{"cookie": "other=value; cashflux_csrf=enc33-csrf-current"})
	t.Cleanup(func() {
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
		clearRefreshMemory()
	})
	pr := prefs.Prefs{ServerURL: "https://app.test"}
	browserstore.SetMemory(authCSRFKey, "old-csrf")
	if got := cookieSessionCSRF(pr); got != "enc33-csrf-current" {
		t.Fatal("cookie did not supersede stale memory")
	}
	pr.ServerURL = "https://other.test"
	if got := cookieSessionCSRF(pr); got != "" {
		t.Fatal("CSRF sent to another origin")
	}
}

func TestENC33LockStartsWorkAfterGrantAndHoldsThroughCommit(t *testing.T) {
	oldNavigator := js.Global().Get("navigator")
	lock := js.Global().Get("Function").New(`
let grant;
let tail = new Promise(resolve => {grant = resolve});
let held = false;
return {grant(){grant()},get held(){return held},request(name,callback){
  const result = tail.then(async () => {
    held = true;
    try {return await callback({name})} finally {held = false}
  });
  tail = result.catch(() => {});
  return result;
}};
`).Invoke()
	enc33SetGlobal("navigator", map[string]any{"locks": lock})
	t.Cleanup(func() { enc33SetGlobal("navigator", oldNavigator) })
	started, finishCommit, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		withTokenRefreshLock(func() {
			if !lock.Get("held").Bool() {
				t.Error("refresh started outside lock callback")
			}
			close(started)
			<-finishCommit
		})
		close(finished)
	}()
	select {
	case <-started:
		t.Fatal("refresh started before grant")
	case <-time.After(10 * time.Millisecond):
	}
	lock.Call("grant")
	<-started
	secondStarted, secondFinished := make(chan struct{}), make(chan struct{})
	go func() { withTokenRefreshLock(func() { close(secondStarted) }); close(secondFinished) }()
	select {
	case <-secondStarted:
		t.Fatal("second refresh ran during pending commit")
	case <-time.After(10 * time.Millisecond):
	}
	if !lock.Get("held").Bool() {
		t.Fatal("lock released before commit")
	}
	close(finishCommit)
	<-finished
	<-secondStarted
	<-secondFinished
}

func TestENC33TwoTabsReadLatestDurableCredentialInsideLock(t *testing.T) {
	enc33Fixture(t)
	lock := js.Global().Get("Function").New(`
let tail = Promise.resolve();
return {request(name, callback) {
 const result = tail.then(() => callback({name}));
 tail = result.catch(() => {});
 return result;
}};
`).Invoke()
	enc33SetGlobal("navigator", map[string]any{"locks": lock})
	s := scopedRefreshSession{enc33Scope(t, "server-a", "account-a")}
	if err := enc33Result(t, func(done func(error)) { s.saveThen("enc33-refresh-0", "", done) }); err != nil {
		t.Fatal(err)
	}
	firstStarted, finishFirst := make(chan struct{}), make(chan struct{})
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() {
		firstDone <- s.refreshFirst(context.Background(), func(_ context.Context, token string) (backendrpc.TokenPairResponse, error) {
			if token != "enc33-refresh-0" {
				t.Error("first tab read wrong credential")
			}
			close(firstStarted)
			<-finishFirst
			return backendrpc.TokenPairResponse{AccessToken: "enc33-access-1", RefreshToken: "enc33-refresh-1", ExpiresInSeconds: 900}, nil
		})
	}()
	<-firstStarted
	go func() {
		secondDone <- s.refreshFirst(context.Background(), func(_ context.Context, token string) (backendrpc.TokenPairResponse, error) {
			if token != "enc33-refresh-1" {
				t.Error("waiting tab replayed the consumed credential")
			}
			return backendrpc.TokenPairResponse{AccessToken: "enc33-access-2", RefreshToken: "enc33-refresh-2", ExpiresInSeconds: 900}, nil
		})
	}()
	close(finishFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	c, _, _, err := s.load()
	if err != nil || c.Token != "enc33-refresh-2" {
		t.Fatalf("latest rotation not durable: %v", err)
	}
	enc33Scan(t, []refreshScope{s.scope}, "enc33-access-1", "enc33-access-2", "enc33-refresh-0", "enc33-refresh-1", "enc33-refresh-2")
}

func TestENC33RejectedLockDoesNotRefreshUnlocked(t *testing.T) {
	oldNavigator := js.Global().Get("navigator")
	locks := js.Global().Get("Function").New(`return {request(){return Promise.reject(new Error("denied"))}};`).Invoke()
	enc33SetGlobal("navigator", map[string]any{"locks": locks})
	t.Cleanup(func() { enc33SetGlobal("navigator", oldNavigator) })
	ran := false
	withTokenRefreshLock(func() { ran = true })
	if ran {
		t.Fatal("rejected lock ran refresh unlocked")
	}
}

func TestENC33ForeignCiphertextIsPreservedAndRejected(t *testing.T) {
	enc33Fixture(t)
	a := scopedRefreshSession{enc33Scope(t, "server-a", "account-a")}
	b := scopedRefreshSession{enc33Scope(t, "server-a", "account-b")}
	for _, s := range []scopedRefreshSession{a, b} {
		if err := enc33Result(t, func(done func(error)) { s.saveThen("enc33-private-refresh", "", done) }); err != nil {
			t.Fatal(err)
		}
	}
	ra, err := openRefreshRecords(a.scope)
	if err != nil {
		t.Fatal(err)
	}
	defer ra.db.Call("close")
	rb, err := openRefreshRecords(b.scope)
	if err != nil {
		t.Fatal(err)
	}
	defer rb.db.Call("close")
	foreign, _, err := ra.Read(a.scope.key())
	if err != nil {
		t.Fatal(err)
	}
	prior, _, err := rb.Read(b.scope.key())
	if err != nil {
		t.Fatal(err)
	}
	if err := rb.Replace(b.scope.key(), foreign, &prior); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.load(); err == nil {
		t.Fatal("foreign ciphertext authenticated")
	}
	if err := enc33Result(t, func(done func(error)) { b.saveThen("replacement", "", done) }); err == nil {
		t.Fatal("unreadable credential silently replaced")
	}
	after, _, err := rb.Read(b.scope.key())
	if err != nil || after != foreign {
		t.Fatal("unreadable credential was changed")
	}
}

func TestENC33CookieBootstrapRefreshesAndRejectsAnotherAccount(t *testing.T) {
	enc33Fixture(t)
	oldLocation, oldDocument, oldFetch := js.Global().Get("location"), js.Global().Get("document"), js.Global().Get("fetch")
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	js.Global().Set("document", map[string]any{"cookie": "cashflux_csrf=enc33-csrf-cookie"})
	t.Cleanup(func() {
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
		js.Global().Set("fetch", oldFetch)
	})
	scope, err := newRefreshScope("server-a", "account-a", "https://app.test", "https://app.test")
	if err != nil {
		t.Fatal(err)
	}
	setResponse := func(account string) {
		js.Global().Set("fetch", js.Global().Get("Function").New("account", `
return function(url, opts) {
 if(url !== "https://app.test/v1/auth/refresh" || opts.credentials !== "include" || opts.headers.get("X-CashFlux-CSRF") !== "enc33-csrf-cookie") throw new Error("wrong cookie request");
 return Promise.resolve({status:200,headers:{get(){return "enc33-csrf-new"}},json(){return Promise.resolve({accessToken:"enc33-cookie-access",userId:account,expiresIn:900})}});
};`).Invoke(account))
	}
	setResponse("account-a")
	if err := enc33Result(t, func(done func(error)) {
		saveScopedSessionThen(nil, scope, backendrpc.TokenPairResponse{AccessToken: "enc33-oauth-access", ExpiresInSeconds: 900}, done)
	}); err != nil {
		t.Fatal(err)
	}
	clearRefreshMemory()
	if err := bootstrapScopedSession(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if lsGet(authAccessTokenKey) != "enc33-cookie-access" || lsGet(authRefreshTokenKey) != "" {
		t.Fatal("cookie bootstrap did not publish a memory-only access token")
	}
	enc33Scan(t, []refreshScope{scope}, "enc33-oauth-access", "enc33-cookie-access", "enc33-csrf-cookie", "enc33-csrf-new")
	setResponse("account-b")
	clearRefreshMemory()
	if err := bootstrapScopedSession(context.Background(), scope); err == nil || lsGet(authAccessTokenKey) != "" {
		t.Fatal("cookie for another account was accepted")
	}
}

func TestENC33CanceledCookieFetchSettlesBeforeLockCanRelease(t *testing.T) {
	oldFetch := js.Global().Get("fetch")
	mock := js.Global().Get("Function").New(`
let aborted=false, settled=false;
return {get aborted(){return aborted},get settled(){return settled},fetch(url,opts){return new Promise((resolve,reject)=>{
 opts.signal.addEventListener("abort",()=>{aborted=true;setTimeout(()=>{settled=true;reject(new Error("aborted"))},10)},{once:true});
})}};
`).Invoke()
	js.Global().Set("fetch", mock.Get("fetch"))
	t.Cleanup(func() { js.Global().Set("fetch", oldFetch) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, completed := fetchCookieRefresh(ctx, "https://app.test", "csrf"); completed {
		t.Fatal("canceled exchange reported completion")
	}
	if !mock.Get("aborted").Bool() || !mock.Get("settled").Bool() {
		t.Fatal("canceled exchange returned while fetch was still in flight")
	}
}

func TestENC33StrictDurabilityCannotDowngrade(t *testing.T) {
	enc33Fixture(t)
	scope := enc33Scope(t, "server-a", "account-a")
	records, err := openRefreshRecords(scope)
	if err != nil {
		t.Fatal(err)
	}
	defer records.db.Call("close")
	db := records.db
	records.db = js.Global().Get("Function").New("db", `return {transaction(...args){if(args[1]==="readwrite")throw new TypeError("strict unsupported");return db.transaction(...args)}};`).Invoke(db)
	if err := records.Replace(scope.key(), "ciphertext", nil); err == nil || !strings.Contains(err.Error(), "strict unsupported") {
		t.Fatalf("strict durability downgraded: %v", err)
	}
	if _, have, err := records.Read(scope.key()); err != nil || have {
		t.Fatal("failed strict write created a credential")
	}
}

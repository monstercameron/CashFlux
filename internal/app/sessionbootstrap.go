// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// scopedRefreshSession is constructed from boot's selected identity. It never
// reads a mutable preference, dataset owner or generic token to choose a scope.
// Its device-wrapped credential store is separate from household App Lock.
type scopedRefreshSession struct{ scope refreshScope }

func (s scopedRefreshSession) load() (refreshCredential, string, bool, error) {
	if err := s.scope.validate(); err != nil {
		return refreshCredential{}, "", false, err
	}
	store, closeStore, err := openRefreshStorage(s.scope, false)
	if err != nil {
		return refreshCredential{}, "", false, err
	}
	defer closeStore()
	raw, have, err := store.Get(s.scope.key())
	if err != nil || !have {
		return refreshCredential{}, raw, have, err
	}
	c, err := decodeRefreshCredential(s.scope, raw)
	return c, raw, true, err
}

// saveThen acknowledges only the encrypted strict-durability commit. expected
// is the exact authenticated record previously read (empty means absent), so
// a stale tab cannot replace a rotated credential silently. Safe in JS callbacks.
func (s scopedRefreshSession) saveThen(token, expected string, done func(error)) {
	_, err := encodeRefreshCredential(s.scope, token)
	if err != nil {
		done(err)
		return
	}
	go func() { done(saveRefreshCredential(s.scope, token, expected)) }()
}

// clearThen is the sign-out/server-change/account-change boundary. Callers stop
// refresh work first, and reload or publish the next identity only on done(nil).
// Only this scope's record is removed; no key-prefix sweep is used.
func (s scopedRefreshSession) clearThen(done func(error)) {
	if err := s.scope.validate(); err != nil {
		done(err)
		return
	}
	go func() {
		ran := false
		var result error
		withTokenRefreshLock(func() {
			ran = true
			result = removeRefreshCredential(s.scope)
			if result == nil {
				clearRefreshMemory()
			}
		})
		if !ran {
			result = errors.New("session refresh: could not acquire the refresh lock")
		}
		done(result)
	}()
}

func clearRefreshMemory() {
	for _, key := range []string{authAccessTokenKey, authRefreshTokenKey, authCSRFKey, authRefreshModeKey, authExpiresInKey} {
		browserstore.SetMemory(key, "")
	}
}

// saveScopedSessionThen is the sign-in hook for the reload-based v1 shell.
// Resolve the account with the newly issued access token first, then pass the
// outgoing fixed scope and the target explicitly. Call reload only on nil.
// A cookie target requires an already-installed server cookie and no raw
// refresh token. It never converts a token-pair login into a cookie login.
func saveScopedSessionThen(previous *refreshScope, target refreshScope, pair backendrpc.TokenPairResponse, done func(error)) {
	token := pair.RefreshToken
	if _, err := encodeRefreshCredential(target, token); err != nil {
		done(err)
		return
	}
	if strings.TrimSpace(pair.AccessToken) == "" {
		done(errors.New("session refresh: an access token is required to complete sign-in"))
		return
	}
	var old refreshScope
	if previous != nil {
		old = *previous // do not retain a caller-owned pointer across async work
		if err := old.validate(); err != nil {
			done(err)
			return
		}
	}
	go func() {
		var result error
		ran := false
		withTokenRefreshLock(func() {
			ran = true
			s := scopedRefreshSession{scope: target}
			_, raw, _, err := s.load()
			if err != nil {
				result = err
				return
			}
			if previous != nil && old != target {
				if result = removeRefreshCredential(old); result != nil {
					return
				}
			}
			settled := make(chan error, 1)
			s.saveThen(token, raw, func(err error) { settled <- err })
			result = <-settled
		})
		if !ran {
			result = errors.New("session refresh: could not acquire the refresh lock")
		}
		done(result)
	}()
}

// restore reads only the fixed scope's record into transient page memory. An
// access token is intentionally absent until refreshFirst completes. BLOCKING:
// boot must call this from its goroutine after the protected-storage gate.
func (s scopedRefreshSession) restore() (bool, error) {
	clearRefreshMemory()
	c, _, have, err := s.load()
	if err != nil || !have {
		return have, err
	}
	browserstore.SetMemory(authAccessTokenKey, "")
	browserstore.SetMemory(authCSRFKey, "")
	browserstore.SetMemory(authRefreshTokenKey, c.Token)
	browserstore.SetMemory(authRefreshModeKey, s.scope.Mode)
	return true, nil
}

type refreshExchange func(context.Context, string) (backendrpc.TokenPairResponse, error)

// refreshFirst reloads the encrypted credential INSIDE the refresh lock, asks
// the worker for an access token, and durably saves the rotated refresh token
// before exposing either token. A reload must never start identity resolution,
// sync or a watch with yesterday's access token or the placeholder "refresh".
func (s scopedRefreshSession) refreshFirst(ctx context.Context, exchange refreshExchange) error {
	if s.scope.Mode != authRefreshModeEncrypted {
		return errors.New("session refresh: encrypted-record mode is required")
	}
	var result error
	ran := false
	withTokenRefreshLock(func() {
		ran = true
		credential, raw, have, err := s.load()
		if err != nil {
			result = err
			return
		}
		if !have {
			result = errors.New("session refresh: no credential for this account")
			return
		}
		pair, err := exchange(ctx, credential.Token)
		if err != nil {
			result = fmt.Errorf("refresh scoped session: %w", err)
			return
		}
		if strings.TrimSpace(pair.AccessToken) == "" || strings.TrimSpace(pair.RefreshToken) == "" || pair.ExpiresInSeconds <= 0 {
			result = errors.New("session refresh: server returned an incomplete token pair")
			return
		}
		done := make(chan error, 1)
		s.saveThen(pair.RefreshToken, raw, func(err error) { done <- err })
		// Do not abandon a rotation after the server consumed the old token.
		// Keep the lock until the storage callback settles, even on cancellation.
		if err := <-done; err != nil {
			result = fmt.Errorf("save rotated refresh credential: %w", err)
			return
		}
		browserstore.SetMemory(authRefreshTokenKey, pair.RefreshToken)
		browserstore.SetMemory(authAccessTokenKey, pair.AccessToken)
		browserstore.SetMemory(authExpiresInKey, strconv.FormatInt(pair.ExpiresInSeconds, 10))
		browserstore.SetMemory(authRefreshModeKey, s.scope.Mode)
		uistate.BumpAuthRevision()
	})
	if !ran {
		return errors.New("session refresh: could not acquire the refresh lock")
	}
	return result
}

// bootstrapScopedSession is the pre-sync boot hook. Identity selection must
// supply the stable server ID and account explicitly; callers must not search
// other scopes for a credential when one is missing or unreadable.
func bootstrapScopedSession(ctx context.Context, scope refreshScope) error {
	s := scopedRefreshSession{scope: scope}
	have, err := s.restore()
	if err != nil || !have {
		if err != nil {
			return err
		}
		return errors.New("session refresh: no saved session for this account")
	}
	if scope.Mode == authRefreshModeCookie {
		// CSRF is read from the same-origin cookie, never persisted in prefs.
		pr := uistate.LoadPrefs().Normalize()
		pr.ServerURL, pr.ServerToken, pr.ServerCSRF = scope.Endpoint, "", ""
		ok := false
		withTokenRefreshLock(func() { ok = doCookieRefreshForAccount(ctx, pr, scope.AccountID) })
		if !ok {
			return errors.New("session refresh: cookie refresh failed")
		}
		return nil
	}
	return s.refreshFirst(ctx, func(ctx context.Context, token string) (backendrpc.TokenPairResponse, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		var pair backendrpc.TokenPairResponse
		err := invokeWorkerRPC(ctx, scope.Endpoint, "refresh", backendrpc.MethodAuthRefreshToken,
			backendrpc.RefreshTokenRequest{RefreshToken: token}, &pair)
		return pair, err
	})
}

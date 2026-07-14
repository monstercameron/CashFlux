// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// lockQuoteInFlight guards refreshDailyLockQuote against firing overlapping API
// calls when boot, unlock, and key-save trigger it near-simultaneously. Single-
// threaded wasm makes a plain bool sufficient (no data race).
var lockQuoteInFlight bool

// refreshDailyLockQuote generates and caches the Smart+ "quote of the day" so the
// lock screen (and the dashboard) can show a fresh AI quote instead of the static
// fallback. It is intentionally decoupled from the dashboard's manual opt-in: the
// user only has to configure an AI provider (an OpenAI key or a backend). We honor
// an explicit opt-out (ExplicitOff), never spend when a fresh same-day quote is
// already cached, and no-op when no provider is reachable.
//
// Why here, and why proactive: while the app is LOCKED the encrypted dataset — and
// with it the OpenAI key — is unavailable, so the lock screen can only display a
// quote produced earlier. Generating just on the dashboard (the old behavior) meant
// "added a key" alone never populated the cache. So we call this on boot, on unlock,
// and right after the key is saved, writing the result into the SMART Results store
// (cashflux:smart-settings) which is a plain browserstore key readable while locked.
//
// The key is sourced from the live session settings when unlocked, or the separately
// stored "remember on this device" key (cashflux:openai-key) which is readable while
// locked — so a remembered key even lets the lock screen refresh its own quote.
func refreshDailyLockQuote() {
	app := appstate.Default
	if app == nil {
		return
	}
	// Guard against duplicate dispatch when boot, unlock, and key-save fire close
	// together (each is a legitimate trigger, but only one call should spend).
	if lockQuoteInFlight {
		return
	}

	s := uistate.LoadSmartSettings()
	// Respect an explicit "off" — but a never-touched feature still generates, since
	// the lock-screen quote is gated on "is AI configured", not the dashboard opt-in.
	if s.ExplicitOff[smartQuoteCode] {
		return
	}

	now := time.Now()
	if cached := strings.TrimSpace(s.ResultFor(smartQuoteCode)); cached != "" {
		if last := s.LastRunAt(smartQuoteCode); last.Year() == now.Year() && last.YearDay() == now.YearDay() {
			return // already have today's quote — don't re-spend
		}
	}

	// Provider: the session key from the loaded dataset (the single source of truth), or
	// a configured backend proxy. While an encrypted dataset is still locked the key
	// isn't available, so no fresh quote is generated then — the last cached quote shows
	// and a new one is generated right after unlock (see hydrateFromPasscode).
	prefs := uistate.LoadPrefs().Normalize()
	useBackend := prefs.BackendActive()
	aiKey := strings.TrimSpace(app.Settings().OpenAIKey)
	if aiKey == "" && !useBackend {
		return
	}

	model := "gpt-5.4-mini"
	if m := strings.TrimSpace(app.Settings().OpenAIModel); m != "" {
		model = m
	}

	// Stamp the run before dispatching so overlapping callers (boot + unlock firing
	// close together) don't both spend on the same day.
	uistate.MarkSmartRun(smartQuoteCode, now)
	lockQuoteInFlight = true

	req := smartai.QuoteOfDay(s.QuoteThemeOr(), "")
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: req.System},
		{Role: ai.RoleUser, Content: req.User},
	}
	onOK := func(c string, _ ai.Usage) {
		lockQuoteInFlight = false
		q := cleanLockQuote(c)
		if q == "" {
			return
		}
		uistate.SetSmartResult(smartQuoteCode, q, time.Now())
		updateLiveLockQuote(q)
	}
	onErr := func(string) { lockQuoteInFlight = false }
	if useBackend {
		ai.SendProxyChat(prefs.ServerURL, prefs.ServerToken, model, messages, 0.85, onOK, onErr)
	} else {
		ai.SendChat(aiKey, ai.DefaultBaseURL, model, messages, 0.85, onOK, onErr)
	}
}

// cleanLockQuote trims whitespace and any wrapping quotation marks the model may add,
// so the stored line is just the quote (and its attribution).
func cleanLockQuote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'“”‘’")
	return strings.TrimSpace(s)
}

// updateLiveLockQuote swaps the freshly generated quote into the lock-screen element
// in place, so a quote that finishes generating while the gate is already showing
// appears without needing a reload. No-op when the gate isn't mounted/visible.
func updateLiveLockQuote(q string) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", "cf-lock-quote")
	if el.IsNull() || el.IsUndefined() {
		return
	}
	if cfg := loadAppLock(); cfg.HideQuotes {
		return
	}
	el.Set("textContent", q)
}

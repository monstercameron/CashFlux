// SPDX-License-Identifier: MIT

//go:build js && wasm

// The browserstore boundary always encrypts local records. The legacy inner
// passcode envelope remains for v1 migration and SEC6 transport encryption.
package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// errSyncDatasetLocked is returned by the backend hydrate path when a pulled
// snapshot is an encryption envelope but the session passcode is not known (the
// app is locked). The caller must NOT apply or drop it — the server keeps the
// snapshot and a fresh pull runs after the next secure unlock.
var errSyncDatasetLocked = errors.New("encrypted sync: dataset locked (passcode unknown)")

// errSyncEncryptEmpty guards the push path: if encryption yields empty or non-envelope
// bytes, the push fails (and retries) rather than clobbering the server snapshot.
var errSyncEncryptEmpty = errors.New("encrypted sync: refusing to push empty/plaintext snapshot")

// encryptDatasetSync is a synchronous wrapper over the async, SubtleCrypto-backed
// encryptDataset, for use from the sync goroutines. It blocks on the crypto
// callback, so it must never run on the main JS callback stack — only inside the
// `go func()` sync workers, which is where it is called.
func encryptDatasetSync(plaintext []byte, passcode string) ([]byte, error) {
	type result struct {
		env []byte
		err error
	}
	ch := make(chan result, 1)
	encryptDataset(plaintext, passcode, func(env []byte, err error) { ch <- result{env, err} })
	r := <-ch
	return r.env, r.err
}

// decryptEnvelopeSync parses and decrypts a cryptobox envelope synchronously (see
// encryptDatasetSync's goroutine caveat). The envelope carries its own salt, so any
// device with the same passcode can derive the key and decrypt — no salt channel.
func decryptEnvelopeSync(raw []byte, passcode string) ([]byte, error) {
	env, ok := cryptobox.Parse(raw)
	if !ok {
		return nil, errors.New("encrypted sync: envelope unreadable")
	}
	type result struct {
		plain []byte
		err   error
	}
	ch := make(chan result, 1)
	decryptDataset(env, passcode, func(plain []byte, err error) { ch <- result{plain, err} })
	r := <-ch
	return r.plain, r.err
}

// activePasscode holds the session passcode once the user has unlocked (or just
// set) the lock, so the autosave can derive the encryption key. It lives only in
// wasm memory — the same exposure as the in-memory dataset itself — and is never
// persisted. Cleared when the lock is removed.
var activePasscode string

// pendingEnvelopeRaw holds a not-yet-decrypted dataset envelope read at boot. While
// it is non-empty the dataset is locked: the store is empty and the autosave is
// suppressed so it can't clobber the ciphertext. It is cleared once the envelope is
// decrypted and imported (or the user wipes).
var pendingEnvelopeRaw string

// encSaving guards against overlapping async encrypt-and-write operations: a new
// autosave tick is skipped while a prior encryption is still in flight.
var encSaving bool

// resaveDataset, set by startDatasetAutosave, forces an immediate dataset rewrite
// regardless of whether the bytes changed. It is nil until the autosave starts.
var resaveDataset func()

// migrateDatasetAtRest triggers an immediate re-save so the at-rest copy switches
// between plaintext and envelope the moment the passcode (encryption mode) changes.
func migrateDatasetAtRest() {
	if resaveDataset != nil {
		resaveDataset()
	}
	// The encryption mode just changed, so the server's snapshot must be re-pushed in
	// the new form (plaintext↔envelope). The plaintext content is unchanged, so force
	// past the push dedup guard.
	forceBackendResyncActiveWorkspace()
}

// datasetEncryptionActive is SEC6's unchanged v1 transport/inner-envelope gate:
// the lock is active (enabled and not suspended) and the session passcode is known.
// The outer browserstore boundary always encrypts local records in both modes.
func datasetEncryptionActive() bool {
	return loadAppLock().Active() && activePasscode != ""
}

// unlockProtectedSession opens the WDK and authenticated cache before any
// legacy passcode envelope is hydrated or the caller dismisses the gate.
func unlockProtectedSession(secret string, recovery bool, done func(error)) {
	unlocked := func(err error) {
		if err != nil {
			done(err)
			return
		}
		go func() {
			passcode := secret
			if recovery {
				passcode = ""
			}
			restored, changed, err := recoverSecureLockTransition()
			if err != nil {
				done(err)
				return
			}
			if changed {
				passcode = restored
			}
			activePasscode = passcode
			if pendingEnvelopeRaw != "" {
				hydrateFromPasscodeThen(passcode, done)
				return
			}
			if appstate.Default != nil {
				hydrateDeviceAIKey()
				if backendConnectionActive(uistate.LoadPrefs().Normalize()) {
					pullActiveWorkspaceFromBackend(true)
				}
			}
			done(nil)
		}()
	}
	if recovery {
		browserstore.UnlockRecoverySecure(secret, unlocked)
	} else {
		browserstore.UnlockSecureThen(secret, unlocked)
	}
}

// unlockLegacyProfile upgrades a v1 UI-only lock to a passcode-wrapped WDK.
// Verify the retained legacy credential before changing any key metadata.
func unlockLegacyProfile(passcode string, done func(error)) {
	if !loadAppLock().Verify(passcode) {
		done(errLegacyDatasetLocked)
		return
	}
	opened := func(err error) {
		if err != nil {
			done(err)
			return
		}
		activePasscode = passcode
		if pendingEnvelopeRaw != "" {
			hydrateFromPasscodeThen(passcode, done)
		} else {
			done(nil)
		}
	}
	if state := browserstore.SecureKeyState(); state != nil && !state.AppLock {
		browserstore.EnableAppLockSecure(passcode, opened)
	} else {
		opened(nil)
	}
}

// hydrateFromPasscode parses the pending envelope, decrypts it with the session
// passcode, and imports the recovered dataset. A failed stored-passcode check
// leaves the ciphertext pending; after the passcode is verified, unreadable or
// unimportable data is quarantined before autosave is released.
func hydrateFromPasscode(passcode string) {
	hydrateFromPasscodeThen(passcode, func(error) {})
}

// hydrateFromPasscodeThen acknowledges only an imported dataset. Failure keeps
// the gate closed, even when the outer WDK was unlocked successfully.
func hydrateFromPasscodeThen(passcode string, done func(error)) {
	app := appstate.Default
	if app == nil {
		done(errors.New("dataset hydration is unavailable"))
		return
	}
	if !loadAppLock().Verify(passcode) {
		app.Log().Error("encrypted dataset: passcode verification failed; keeping ciphertext")
		done(errLegacyDatasetLocked)
		return
	}
	env, ok := cryptobox.Parse([]byte(pendingEnvelopeRaw))
	if !ok {
		app.Log().Error("encrypted dataset: envelope unreadable after verified passcode; quarantining ciphertext")
		quarantinePendingEnvelope("The encrypted dataset envelope is unreadable after the passcode was verified.")
		done(errors.New("legacy dataset envelope is unreadable"))
		return
	}
	decryptDataset(env, passcode, func(plain []byte, err error) {
		// WebCrypto resolves this callback on the JavaScript event stack. Journal
		// replay opens IndexedDB transactions and waits for browser events; it
		// must run after this callback returns so those events can be delivered.
		go func() {
			if err != nil {
				app.Log().Error("encrypted dataset: decrypt failed after verified passcode; quarantining ciphertext", "err", err)
				quarantinePendingEnvelope("The encrypted dataset could not be decrypted after the passcode was verified.")
				done(err)
				return
			}
			if err := app.ImportJSON(plain); err != nil {
				app.Log().Error("encrypted dataset: import after decrypt failed; quarantining ciphertext", "err", err)
				quarantinePendingEnvelope("The decrypted dataset could not be imported.")
				done(err)
				return
			}
			// Grant autosave authority only after import succeeds. Set both flags before
			// clearing the pending envelope, which otherwise releases the autosave guard.
			activePasscode = passcode
			hydrationSucceeded = true
			if err := replayDurableDatasetRecords(app, pendingEnvelopeRaw); err != nil {
				quarantinePendingEnvelope("Saved changes could not be recovered: " + err.Error())
				done(err)
				return
			}
			hadLocalDataset = true
			pendingEnvelopeRaw = "" // releases the autosave and unblocks normal operation
			// Re-seed the PREFS ATOM from the just-imported settings KV. The lock screen
			// rendered before this import, so the first UsePrefs seeded the atom from an
			// EMPTY store — prefs.Default(). Without this, every passcode boot shows
			// default preferences (the budgets income basis reset to "all income", Cam
			// 2026-07-17) even though the saved values sit right here in the store — and
			// the next PersistPrefs (any theme toggle) would write those defaults back
			// over the real ones. SetPrefs also re-applies theme/accent to the document.
			uistate.SetPrefs(uistate.LoadPrefs())
			uistate.ApplyTheme(uistate.LoadTheme())
			// The dataset is loaded now, so it's safe to fold any legacy standalone OpenAI
			// key + language selection into it (encrypted users couldn't migrate at boot
			// while still locked).
			migrateStandaloneAIKey()
			uistate.MigrateLegacyLanguage()
			seedMusicFromDataset()
			initUndo()
			uistate.BumpDataRevision()
			// The OpenAI key is now decrypted and available — refresh the lock-screen
			// quote-of-the-day cache so the next lock shows a fresh AI quote (C: lock quote).
			refreshDailyLockQuote()
			done(nil)
		}()
	})
}

// quarantinePendingEnvelope preserves the exact encrypted bytes for recovery and
// releases the pending-envelope autosave guard only after quarantine has taken over.
func quarantinePendingEnvelope(reason string) {
	raw := pendingEnvelopeRaw
	pendingEnvelopeRaw = ""
	quarantineDataset(raw, reason)
}

// secureLockTransition bridges the durable keyring commit and the atomic record
// commit. The journal itself is always encrypted by the WDK. If the page closes
// between them, either surviving wrap opens this journal and completes the same
// prepared migration before any protected preference/dataset is consumed.
// Passcode is never logged, exported or stored outside this encrypted record.
type secureLockTransition struct {
	Passcode string            `json:"passcode"`
	Values   map[string]string `json:"values"`
	Expected map[string]string `json:"expected"`
}

const secureLockTransitionKey = "cashflux:applock-transition"

// awaitSecureAction is goroutine-only; it must never park a JS event callback
// or the browserstore queue worker waiting for a later job on that same queue.
func awaitSecureAction(start func(func(error))) error {
	done := make(chan error, 1)
	start(func(err error) { done <- err })
	return <-done
}

func prepareSecureLockTransition(passcode string, config applock.Config, discardVault bool) (*secureLockTransition, error) {
	t := &secureLockTransition{Passcode: passcode, Values: map[string]string{}, Expected: map[string]string{}}
	configRaw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("prepare App Lock settings: %w", err)
	}
	t.Values[appLockKey] = string(configRaw)
	for _, key := range []string{appLockKey, datasetStoreKey, credVaultStoreKey, aiCredentialStoreKey, webauthnCredKey, webauthnSaltKey, webauthnVaultKey} {
		raw, _, err := browserstore.GetSecure(key)
		if err != nil {
			return nil, err
		}
		t.Expected[key] = raw
		if key != appLockKey {
			t.Values[key] = raw
		}
	}
	dataset, preconditions, err := materializeLocalDataset()
	if err != nil {
		return nil, fmt.Errorf("prepare complete dataset for App Lock: %w", err)
	}
	for key, raw := range preconditions {
		t.Expected[key] = raw
		if key == datasetOwnerKey || key == workspacesKey {
			t.Values[key] = raw
		}
	}
	t.Values[datasetStoreKey] = dataset
	// The legacy vault and dataset still have an inner passcode envelope.
	// Prepare replacements in memory; never expose a half-rekeyed vault.
	if raw := t.Values[credVaultStoreKey]; raw != "" && !discardVault {
		encoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid base64", errCredVaultUnreadable)
		}
		plain, err := decryptEnvelopeSync(encoded, activePasscode)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errCredVaultUnreadable, err)
		}
		var vault credVault
		if err := json.Unmarshal(plain, &vault); err != nil {
			return nil, fmt.Errorf("%w: invalid contents", errCredVaultUnreadable)
		}
		next, err := encryptDatasetSync(plain, passcode)
		if err != nil {
			return nil, err
		}
		t.Values[credVaultStoreKey] = base64.StdEncoding.EncodeToString(next)
	}
	if discardVault {
		t.Values[credVaultStoreKey] = ""
	}
	if cryptobox.IsEnvelope([]byte(t.Expected[datasetStoreKey])) && passcode != "" {
		next, err := encryptDatasetSync([]byte(dataset), passcode)
		if err != nil {
			return nil, err
		}
		t.Values[datasetStoreKey] = string(next)
	}
	if raw := t.Values[aiCredentialStoreKey]; raw != "" {
		record, err := aicred.Parse([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("prepare AI credential: %w", err)
		}
		key := record.Key
		if record.Envelope != "" {
			plain, err := decryptEnvelopeSync([]byte(record.Envelope), activePasscode)
			if err != nil {
				return nil, fmt.Errorf("prepare AI credential: %w", err)
			}
			key = string(plain)
		}
		envelope := ""
		if passcode != "" {
			encrypted, err := encryptDatasetSync([]byte(key), passcode)
			if err != nil {
				return nil, err
			}
			envelope, key = string(encrypted), ""
		}
		next, err := aicred.New(key, envelope, record.Remember)
		if err != nil {
			return nil, err
		}
		encoded, err := aicred.Marshal(next)
		if err != nil {
			return nil, err
		}
		t.Values[aiCredentialStoreKey] = string(encoded)
	}
	// Invalidate stale PRF passcode envelopes in the same record commit.
	for _, key := range []string{webauthnCredKey, webauthnSaltKey, webauthnVaultKey} {
		t.Values[key] = ""
	}
	// The re-keyed checkpoint supersedes its journal atomically. A journal-only
	// edit in another tab must reject this transition just like a changed vault.
	t.Values, _ = prepareWorkspaceCommit(t.Values, preconditions[datasetGenKey])
	return t, nil
}

func commitSecureLockTransition(t *secureLockTransition, journal string) error {
	state := browserstore.SecureKeyState()
	if state == nil {
		return atrest.ErrRecoveryRequired
	}
	if t.Passcode != "" {
		if state.AppLock {
			if err := awaitSecureAction(func(done func(error)) { browserstore.ChangePasscodeSecure(t.Passcode, done) }); err != nil {
				return err
			}
		} else if err := awaitSecureAction(func(done func(error)) { browserstore.EnableAppLockSecure(t.Passcode, done) }); err != nil {
			return err
		}
	} else if state.AppLock {
		if err := awaitSecureAction(browserstore.DisableAppLockSecure); err != nil {
			return err
		}
	}
	t.Values[secureLockTransitionKey] = ""
	t.Expected[secureLockTransitionKey] = journal
	if err := awaitSecureAction(func(done func(error)) { browserstore.SetSecureBatch(t.Values, t.Expected, done) }); err != nil {
		return err
	}
	datasetMyGen = t.Values[datasetGenKey]
	activePasscode = t.Passcode
	lockedPasskey = [3]string{}
	clearDatasetKeyCache()
	clearArtifactKeyCache()
	return nil
}

func runSecureLockTransition(passcode string, config applock.Config, discardVault bool) error {
	browserstore.Barrier()
	if existing, _, err := browserstore.GetSecure(secureLockTransitionKey); err != nil {
		return err
	} else if existing != "" {
		return errors.New("an App Lock change needs to finish before another can start")
	}
	t, err := prepareSecureLockTransition(passcode, config, discardVault)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("prepare App Lock transition: %w", err)
	}
	journal := string(raw)
	if err := awaitSecureAction(func(done func(error)) {
		browserstore.SetSecureBatch(map[string]string{secureLockTransitionKey: journal}, map[string]string{secureLockTransitionKey: ""}, done)
	}); err != nil {
		return err
	}
	return commitSecureLockTransition(t, journal)
}

func recoverSecureLockTransition() (string, bool, error) {
	raw, _, err := browserstore.GetSecure(secureLockTransitionKey)
	if err != nil || raw == "" {
		return "", false, err
	}
	var t secureLockTransition
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return "", false, fmt.Errorf("read pending App Lock change: %w", err)
	}
	var config applock.Config
	if err := json.Unmarshal([]byte(t.Values[appLockKey]), &config); err != nil || config.Enabled != (t.Passcode != "") {
		return "", false, errors.New("pending App Lock settings are inconsistent")
	}
	if config.Enabled && !config.Verify(t.Passcode) {
		return "", false, errors.New("pending App Lock passcode does not match its settings")
	}
	// Refuse a damaged journal rather than acknowledging a key-only transition.
	for _, key := range []string{appLockKey, datasetStoreKey, credVaultStoreKey, aiCredentialStoreKey, webauthnCredKey, webauthnSaltKey, webauthnVaultKey} {
		if _, ok := t.Values[key]; !ok {
			return "", false, errors.New("pending App Lock data is incomplete")
		}
		if _, ok := t.Expected[key]; !ok {
			return "", false, errors.New("pending App Lock preconditions are incomplete")
		}
	}
	if len(t.Values) == 7 && len(t.Expected) == 7 {
		// A transition prepared by a pre-journal bundle is safe only without a
		// journal. Never let its checkpoint discard newer record-only writes.
		head, _, err := browserstore.GetSecure(datasetHeadKey)
		if err != nil || head != "" {
			return "", false, errors.New("the older App Lock change needs dataset recovery")
		}
		t.Values, _ = prepareWorkspaceCommit(t.Values, readDatasetGen())
		t.Expected[datasetGenKey], t.Expected[datasetHeadKey] = readDatasetGen(), ""
	} else {
		for _, key := range []string{datasetHeadKey, datasetGenKey} {
			if _, ok := t.Values[key]; !ok {
				return "", false, errors.New("pending App Lock checkpoint is incomplete")
			}
		}
		for _, key := range []string{datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey} {
			if _, ok := t.Expected[key]; !ok {
				return "", false, errors.New("pending App Lock checkpoint preconditions are incomplete")
			}
		}
		if len(t.Values) != 11 || len(t.Expected) != 11 || t.Values[datasetHeadKey] != "" || t.Values[datasetGenKey] == "" {
			return "", false, errors.New("pending App Lock data has unexpected records")
		}
	}
	if err := commitSecureLockTransition(&t, raw); err != nil {
		return "", false, err
	}
	return t.Passcode, true, nil
}

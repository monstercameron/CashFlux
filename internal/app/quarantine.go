// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

const quarantinePrefix = "cashflux:quarantine:"

var (
	datasetQuarantined     bool
	quarantinePending      bool
	quarantineRaw          string
	quarantineAvailable    bool
	quarantineBannerHidden bool
	hydrationSucceeded     bool
)

// autosaveBlocked reports whether hydration has left the in-memory dataset
// without authority to replace the saved bytes.
func autosaveBlocked() bool {
	// In hosted mode the route gate holds autosave until the authoritative pull
	// resolves. Once it does, the downloaded workspace may safely replace the
	// unreadable local cache even though its recovery copy remains available.
	return (datasetQuarantined && !hostedDocument()) || quarantinePending || (!hydrationSucceeded && !hadLocalDataset && pendingEnvelopeRaw == "")
}

// quarantineDataset saves the exact unreadable bytes before enabling recovery
// UI, reusing an existing record when its content hash matches.
func quarantineDataset(raw, reason string) {
	workspace, owner := recoveryWorkspaceName(), browserstore.GetString(datasetOwnerKey)
	var supplied recordRecovery
	suppliedCopy := json.Unmarshal([]byte(raw), &supplied) == nil && supplied.Format == "v1"
	raw = datasetRecoveryCopy(raw, workspace, browserstore.Keys(), browserstore.GetSecure)
	var copy recordRecovery
	if json.Unmarshal([]byte(raw), &copy) == nil && copy.Format == "v1" && copy.Workspace != "" {
		workspace = copy.Workspace
		if suppliedCopy {
			owner = copy.Owner
		} else {
			copy.Owner = owner
			encoded, err := json.Marshal(copy)
			if err != nil {
				return
			}
			raw = string(encoded)
		}
	}
	quarantinePending = true
	// Scope is part of the durable name, including for opaque bytes which have
	// no parseable owner metadata. Random suffixes prevent simultaneous tabs
	// from claiming the same timestamp-based recovery key.
	key := quarantineKey(time.Now().Unix()) + ":" + checkpointHash(workspace) + ":" + checkpointHash(owner) + ":" + id.New()
	quarantineRaw = raw
	quarantineAvailable = true
	browserstore.RetainRecovery(key, raw, reason, describeRecoveryPayload, browserstore.RecoveryLive{}, func(key string, err error) {
		if err != nil {
			if app := appstate.Default; app != nil {
				app.Log().Error("recovery copy could not be committed", "err", err)
			}
			return
		}
		quarantinePending = false
		datasetQuarantined = true
		if app := appstate.Default; app != nil {
			app.Log().Error("unreadable dataset quarantined", "reason", reason, "key", key)
		}
		showQuarantineBanner()
	})
}

// loadQuarantineRecord restores the newest saved recovery copy on every boot.
func loadQuarantineRecord() bool {
	records := quarantineRecords()
	newest, ok := store.NewestQuarantineRecord(records)
	if !ok {
		return false
	}
	quarantineRaw = string(newest.Raw)
	quarantineAvailable = true
	showQuarantineBanner()
	return true
}

func findQuarantineRecord(raw []byte) (string, bool) {
	return store.FindQuarantineRecord(raw, quarantineRecords())
}

func quarantineRecords() []store.QuarantineRecord {
	browserstore.Barrier()
	keys := browserstore.Keys()
	records := make([]store.QuarantineRecord, 0, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(key, quarantinePrefix) || strings.HasSuffix(key, ":reason") {
			continue
		}
		createdAt, sequence, ok := parseQuarantineKey(key)
		if !ok {
			continue
		}
		raw, present := browserstore.Get(key)
		var err error
		if browserstore.Durable() {
			raw, present, err = browserstore.GetSecure(key)
		}
		if err != nil || !present || raw == "" {
			continue
		}
		records = append(records, store.QuarantineRecord{
			Key: key, Raw: []byte(raw), CreatedAt: createdAt, Sequence: sequence,
		})
	}
	return records
}

func parseQuarantineKey(key string) (int64, int64, bool) {
	base := strings.SplitN(strings.TrimPrefix(key, quarantinePrefix), ":", 2)[0]
	parts := strings.SplitN(base, "-", 2)
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	if len(parts) == 1 {
		return createdAt, 0, true
	}
	sequence, err := strconv.ParseInt(parts[1], 10, 64)
	return createdAt, sequence, err == nil
}

func quarantineKey(unixSeconds int64) string {
	key := fmt.Sprintf("%s%d", quarantinePrefix, unixSeconds)
	for suffix := int64(1); ; suffix++ {
		exists := false
		for _, saved := range browserstore.Keys() {
			if saved == key {
				exists = true
				break
			}
		}
		if !exists {
			return key
		}
		key = fmt.Sprintf("%s%d-%d", quarantinePrefix, unixSeconds, suffix)
	}
}

func showQuarantineBanner() {
	doc := js.Global().Get("document")
	if quarantineBannerHidden || !doc.Truthy() || !doc.Get("body").Truthy() {
		return
	}
	if old := doc.Call("getElementById", "cashflux-quarantine-banner"); old.Truthy() {
		return
	}
	banner := doc.Call("createElement", "section")
	banner.Set("id", "cashflux-quarantine-banner")
	banner.Call("setAttribute", "role", "alert")
	banner.Call("setAttribute", "aria-live", "assertive")
	banner.Set("style", "padding:16px 20px;background:color-mix(in srgb,var(--warn) 16%,var(--bg-elev));color:var(--text);border-bottom:1px solid var(--warn);box-shadow:0 2px 12px rgba(0,0,0,.12);font:500 15px/1.5 system-ui,sans-serif")
	p := doc.Call("createElement", "p")
	p.Set("textContent", uistate.T("datasetQuarantine.message"))
	p.Set("style", "margin:0 0 10px")
	banner.Call("appendChild", p)
	buttons := doc.Call("createElement", "div")
	buttons.Set("style", "display:flex;gap:10px;flex-wrap:wrap")
	download := quarantineButton(doc, uistate.T("datasetQuarantine.download"), downloadQuarantinedDataset)
	startFresh := quarantineButton(doc, uistate.T("datasetQuarantine.startFresh"), startFreshAfterQuarantine)
	hide := quarantineButton(doc, uistate.T("datasetQuarantine.hideForNow"), hideQuarantineBanner)
	buttons.Call("appendChild", download)
	buttons.Call("appendChild", startFresh)
	buttons.Call("appendChild", hide)
	banner.Call("appendChild", buttons)
	body := doc.Get("body")
	if first := body.Get("firstChild"); first.Truthy() {
		body.Call("insertBefore", banner, first)
	} else {
		body.Call("appendChild", banner)
	}
}

func quarantineButton(doc js.Value, label string, action func()) js.Value {
	button := doc.Call("createElement", "button")
	button.Set("type", "button")
	button.Set("textContent", label)
	button.Set("style", "padding:7px 11px;border:1px solid var(--border);border-radius:6px;background:var(--bg-elev);color:var(--text);font:inherit;cursor:pointer")
	button.Set("onclick", js.FuncOf(func(js.Value, []js.Value) any { action(); return nil }))
	button.Set("onfocus", js.FuncOf(func(js.Value, []js.Value) any {
		button.Get("style").Set("outline", "2px solid var(--warn)")
		button.Get("style").Set("outlineOffset", "2px")
		return nil
	}))
	button.Set("onblur", js.FuncOf(func(js.Value, []js.Value) any {
		button.Get("style").Set("outline", "")
		return nil
	}))
	return button
}

func hideQuarantineBanner() {
	quarantineBannerHidden = true
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	if banner := doc.Call("getElementById", "cashflux-quarantine-banner"); banner.Truthy() {
		banner.Call("remove")
	}
}

func downloadQuarantinedDataset() { go downloadQuarantinedDatasetAsync() }

func downloadQuarantinedDatasetAsync() {
	if !quarantineAvailable {
		return
	}
	raw := quarantinedDownloadBytes()
	doc := js.Global().Get("document")
	blobParts := js.Global().Get("Array").New()
	blobParts.Call("push", string(raw))
	blob := js.Global().Get("Blob").New(blobParts, map[string]any{"type": "application/octet-stream"})
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", "cashflux-unreadable-dataset.bin")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	a.Call("remove")
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

func quarantinedDownloadBytes() []byte {
	if !quarantineAvailable {
		return nil
	}
	if quarantineRaw != "" {
		return []byte(quarantineRaw)
	}
	return lazyRecoveryDownload()
}

func clearQuarantineState() {
	datasetQuarantined = false
	quarantinePending = false
	quarantineRaw = ""
	quarantineAvailable = false
	quarantineBannerHidden = true
	if doc := js.Global().Get("document"); doc.Truthy() {
		if banner := doc.Call("getElementById", "cashflux-quarantine-banner"); banner.Truthy() {
			banner.Call("remove")
		}
	}
}

func clearEmptyQuarantineState() {
	if quarantinePending || quarantineRaw != "" {
		return
	}
	for _, key := range browserstore.Keys() {
		if (!strings.HasPrefix(key, quarantinePrefix) && !strings.HasPrefix(key, journalRecoveryPrefix)) || strings.HasSuffix(key, ":reason") {
			continue
		}
		// A missing cache value is an opaque deferred record, not evidence
		// that the recovery copy is empty. Maintenance resolves it after paint.
		raw, present := browserstore.Get(key)
		if !present || raw != "" {
			return
		}
	}
	clearQuarantineState()
}

func startFreshAfterQuarantine() {
	uistate.ConfirmModalLabeled(uistate.T("datasetQuarantine.confirm"), uistate.T("datasetQuarantine.confirmButton"), true, func(ok bool) {
		if ok {
			startFreshConfirmed()
		}
	})
}

func startFreshConfirmed() {
	app := appstate.Default
	if app == nil || app.Wipe() != nil {
		return
	}
	datasetQuarantined = false
	quarantinePending = false
	quarantineAvailable = false
	hydrationSucceeded = true
	hadLocalDataset = false
	pendingEnvelopeRaw = ""
	uistate.SetSampleActive(false)
	browserstore.Set(seededFlagKey, "1")
	if banner := js.Global().Get("document").Call("getElementById", "cashflux-quarantine-banner"); banner.Truthy() {
		banner.Call("remove")
	}
	if resaveDataset != nil {
		resaveDataset()
	}
	uistate.PostNotice(uistate.T("datasetQuarantine.started"), false)
}

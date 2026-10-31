// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

// Workspaces let one user keep several independent contexts — each with its own
// dataset and UI state — and switch between them (e.g. real money vs. an
// experimental sandbox). A swap changes *everything* except the user-global
// OpenAI key, which stays available across workspaces.
//
// Mechanism: the canonical "cashflux:*" localStorage keys always hold the ACTIVE
// workspace's live state (exactly as the single-workspace app already worked). An
// inactive workspace's state is bundled into "cashflux:ws-data:<id>". Switching
// bundles the current keys out, restores the target's bundle in, then reloads the
// page so boot re-hydrates everything from the swapped-in keys — no per-atom
// re-seeding, and the 12 uistate stores are untouched.
const (
	workspacesKey   = "cashflux:workspaces"
	wsBlobPrefix    = "cashflux:ws-data:"
	defaultWSID     = "default"
	defaultWSName   = "Default"
	newWSNamePrefix = "Workspace"
)

// perWorkspaceKeys are the localStorage keys that belong to a single workspace
// and are bundled/restored on a switch. Everything else (the OpenAI key, the
// workspace registry itself) is user-global and left untouched.
var perWorkspaceKeys = []string{
	"cashflux:dataset",
	"cashflux:prefs",
	"cashflux:layout",
	"cashflux:layout-mode",
	"cashflux:nav-order",
	"cashflux:period-res",
	"cashflux:widget-config",
	"cashflux:hidden-modules",
	"cashflux:tx-filter",
	"cashflux:rail-collapsed",
	"cashflux:languages",
	"cashflux:active-lang",
}

func lsGet(key string) string { return browserstore.GetString(key) }
func lsSet(key, val string)   { browserstore.Set(key, val) }
func lsSetThen(key, val string, done func()) {
	if key == datasetStoreKey {
		commitWorkspaceValuesThen(map[string]string{key: val}, done)
		return
	}
	browserstore.SetThen(key, val, done)
}
func lsRemove(key string) { browserstore.Remove(key) }
func reloadPage()         { js.Global().Get("location").Call("reload") }

type workspaceKeyValue struct{ key, value string }

func writeKeyValuesThen(values []workspaceKeyValue, done func()) {
	if len(values) == 0 {
		if done != nil {
			done()
		}
		return
	}
	updates := make(map[string]string, len(values))
	for _, value := range values {
		updates[value.key] = value.value
	}
	commitWorkspaceValuesThen(updates, done)
}

func saveBlobThen(wsID string, b map[string]string, done func()) {
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	lsSetThen(wsBlobKey(wsID), string(data), done)
}

func saveRegistryThen(r workspace.Registry, done func()) {
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	lsSetThen(workspacesKey, string(data), done)
}

func applyBundleThen(b map[string]string, done func()) {
	values := make(map[string]string, len(perWorkspaceKeys)+1)
	values[datasetHeadKey] = ""
	for _, key := range perWorkspaceKeys {
		values[key] = b[key]
	}
	commitWorkspaceValuesThen(values, done)
}

func commitWorkspaceValuesThen(values map[string]string, done func()) {
	values, expected := prepareWorkspaceCommit(values, readDatasetGen())
	browserstore.SetSecureBatch(values, expected, func(err error) {
		if err != nil {
			if app := appstate.Default; app != nil {
				app.Log().Error("workspace could not be committed", "err", err)
			}
			return
		}
		if generation, ok := values[datasetGenKey]; ok {
			datasetMyGen = generation
		}
		if done != nil {
			done()
		}
	})
}

func prepareWorkspaceCommit(values map[string]string, generation string) (map[string]string, map[string]string) {
	updates := make(map[string]string, len(values)+2)
	for key, value := range values {
		updates[key] = value
	}
	var expected map[string]string
	if _, replaces := values[datasetStoreKey]; replaces {
		updates[datasetHeadKey] = ""
		if updates[datasetGenKey] == "" {
			updates[datasetGenKey] = id.New()
		}
		expected = map[string]string{datasetGenKey: generation}
	}
	return updates, expected
}

func reloadWorkspaceAfterWrites() {
	generation := id.New()
	browserstore.SetThen(datasetGenKey, generation, func() {
		datasetMyGen = generation
		reloadPage()
	})
}

func bumpAndReloadWorkspace(bump func() string, adopt func(string), reload func()) {
	adopt(bump())
	reload()
}

func reloadAfterDurableWrites(write func(func()), adopt func(), reload func()) {
	write(func() {
		adopt()
		reload()
	})
}

// loadRegistry reads the workspace registry from localStorage (empty when absent).
func loadRegistry() workspace.Registry {
	var r workspace.Registry
	if raw := lsGet(workspacesKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &r)
	}
	return r
}

func saveRegistry(r workspace.Registry) {
	if data, err := json.Marshal(r); err == nil {
		lsSet(workspacesKey, string(data))
	}
}

// ensureWorkspaceRegistry initializes the registry on first run with the new
// build: existing single-workspace data becomes the "Default" workspace (its
// canonical keys are already in place, so no migration of the data itself is
// needed). Idempotent — a no-op once a registry exists. Returns the registry.
func ensureWorkspaceRegistry() workspace.Registry {
	r := loadRegistry()
	if len(r.Workspaces) == 0 {
		r = workspace.Registry{}.Add(defaultWSID, defaultWSName).SetColor(defaultWSID, paletteColor(0))
		saveRegistry(r)
	}
	return r
}

// applyStartupWorkspace runs once at boot (after ensureWorkspaceRegistry, before
// hydrateDataset) to honor the user's startup preference. When a workspace is
// pinned and it isn't the one whose data currently sits in the canonical keys, it
// swaps the pinned workspace's bundle in — bundling the last-active one out first,
// so nothing is lost. No reload is needed: nothing has mounted or read the keys
// yet, so hydrateDataset simply loads the swapped-in context.
func applyStartupWorkspace() {
	// Boot runs on a Go goroutine before hydration. Await the strict transaction
	// here so no reader can observe a partly adopted workspace.
	generation := readDatasetGen()
	registryRaw := lsGet(workspacesKey)
	r := loadRegistry()
	target := r.StartupTarget()
	if target == "" || target == r.ActiveID {
		return
	}
	current := bundleCurrent()
	if current == nil {
		return
	}
	currentRaw, err := json.Marshal(current)
	if err != nil {
		return
	}
	nextRaw, err := json.Marshal(r.SetActive(target))
	if err != nil {
		return
	}
	targetKey := wsBlobKey(target)
	targetRaw := lsGet(targetKey)
	bundle := map[string]string{}
	if targetRaw != "" {
		if err := json.Unmarshal([]byte(targetRaw), &bundle); err != nil || bundle == nil {
			if app := appstate.Default; app != nil {
				app.Log().Error("startup workspace could not be read; current workspace was kept", "err", err)
			}
			return
		}
	}
	values := map[string]string{
		wsBlobKey(r.ActiveID): string(currentRaw),
		workspacesKey:         string(nextRaw),
		targetKey:             targetRaw,
	}
	expected := map[string]string{
		datasetGenKey:         generation,
		workspacesKey:         registryRaw,
		targetKey:             targetRaw,
		wsBlobKey(r.ActiveID): lsGet(wsBlobKey(r.ActiveID)),
	}
	if err := applyBundle(bundle, values, expected); err != nil {
		if app := appstate.Default; app != nil {
			app.Log().Error("startup workspace could not be committed; current workspace was kept", "err", err)
		}
	}
}

// setStartupWorkspace records which workspace the app opens on launch ("" =
// resume the last-active one). No reload — it only takes effect next boot.
func setStartupWorkspace(wsID string) {
	saveRegistry(loadRegistry().SetStartup(wsID))
}

// setWorkspaceColor records a workspace's accent color ("" clears it). No reload —
// the switcher reads the registry on its next render.
func setWorkspaceColor(wsID, color string) {
	saveRegistry(loadRegistry().SetColor(wsID, color))
}

// moveWorkspace repositions a workspace in the list to toIndex. No reload — the
// switcher and management list read the registry on their next render.
func moveWorkspace(wsID string, toIndex int) {
	saveRegistry(loadRegistry().Move(wsID, toIndex))
}

// wsExport is the portable envelope for a single workspace: its name, color, and
// the snapshot of its per-workspace keys (dataset + UI state).
//
// The OpenAI key is user-global and lives outside perWorkspaceKeys, so it never
// appears here. The server connection fields DO live in perWorkspaceKeys (prefs
// is bundled per workspace) and are stripped by stripConnectionPrefs on the way
// out and on the way back in — this comment used to claim the envelope "carries
// no secrets" while it was writing a live access token into a downloaded file.
type wsExport struct {
	Version int               `json:"version"`
	Name    string            `json:"name"`
	Color   string            `json:"color,omitempty"`
	Bundle  map[string]string `json:"bundle"`
}

const wsExportVersion = 1

// exportWorkspace downloads a workspace with its file bytes. IndexedDB reads
// must run off the browser event callback stack.
func exportWorkspace(wsID string) {
	go func() {
		if err := exportWorkspaceFile(wsID); err != nil {
			uistate.PostNotice(uistate.T("sync.mergeFailed", err.Error()), true)
		}
	}()
}

// exportWorkspaceFile completes the file before a recovery action may proceed.
// It returns an error rather than silently producing an incomplete backup.
func exportWorkspaceFile(wsID string) error {
	r := loadRegistry()
	w, ok := r.Get(wsID)
	if !ok {
		return fmt.Errorf("workspace %s is unavailable", wsID)
	}
	bundle := loadBlob(wsID)
	if wsID == r.ActiveID {
		bundle = bundleCurrent()
		if bundle == nil {
			return fmt.Errorf("current workspace could not be read")
		}
		data, err := activeDataset()
		if err != nil {
			return fmt.Errorf("complete current workspace backup: %w", err)
		}
		bundle[datasetStoreKey] = data
	} else {
		if bundle[datasetStoreKey] == "" {
			return fmt.Errorf("workspace %s has no saved dataset", wsID)
		}
		app := appstate.Default
		if app == nil {
			return fmt.Errorf("active dataset is unavailable")
		}
		complete, err := app.CompleteArtifactSnapshot([]byte(bundle[datasetStoreKey]))
		if err != nil {
			return fmt.Errorf("complete workspace %s backup: %w", wsID, err)
		}
		bundle[datasetStoreKey] = string(complete)
	}
	env := wsExport{Version: wsExportVersion, Name: w.Name, Color: w.Color, Bundle: stripConnectionPrefs(bundle)}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace backup: %w", err)
	}
	downloadBytes("workspace-"+slugify(w.Name)+".json", "application/json", data)
	return nil
}

// importWorkspace adds a workspace from an exported JSON file and switches to it
// (the current workspace is bundled out first, so nothing is lost). Returns false
// on a malformed file so the caller can surface an error.
func importWorkspace(data []byte) bool {
	var env wsExport
	if err := json.Unmarshal(data, &env); err != nil || env.Bundle == nil {
		return false
	}
	name := strings.TrimSpace(env.Name)
	if name == "" {
		name = uistate.T("ws.importedDefault")
	}
	r := loadRegistry()
	suspendAutosave = true
	current := bundleCurrent()
	if current == nil {
		return false
	}
	newID := id.NewWithPrefix("ws")
	color := env.Color
	if color == "" {
		color = paletteColor(len(r.Workspaces))
	}
	// An imported file is untrusted input: it may be a colleague's export, or a
	// file handed over by somebody who wants this browser talking to their
	// server. Stripping the connection fields on the way IN — not just on the way
	// out — means an older export written before the export-side strip existed,
	// or one hand-edited to add them back, still cannot inject an endpoint or a
	// bearer token into this device's prefs.
	imported := stripConnectionPrefs(env.Bundle)
	next := r.Add(newID, name).SetActive(newID).SetColor(newID, color)
	saveBlobThen(r.ActiveID, current, func() {
		saveBlobThen(newID, imported, func() {
			saveRegistryThen(next, func() {
				applyBundleThen(imported, reloadWorkspaceAfterWrites)
			})
		})
	})
	return true
}

// slugify makes a filename-safe slug from a workspace name (lowercase, spaces and
// other non-alphanumerics collapsed to hyphens), falling back to "workspace".
func slugify(name string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "workspace"
	}
	return s
}

func wsBlobKey(wsID string) string { return wsBlobPrefix + wsID }

// bundleCurrent snapshots the active workspace's per-workspace keys.
func bundleCurrent() map[string]string {
	out := make(map[string]string, len(perWorkspaceKeys))
	for _, k := range perWorkspaceKeys {
		if v := lsGet(k); v != "" {
			out[k] = v
		}
	}
	if raw := out[datasetStoreKey]; raw != "" {
		data, err := portableDatasetForBundle(raw)
		if err != nil {
			uistate.PostNotice(uistate.T("bv6d1.workspaceCopyFailed"), true)
			return nil
		}
		out[datasetStoreKey] = data
	}
	return out
}

// applyBundle atomically replaces the canonical keys together with the supplied
// workspace metadata. Missing bundle keys become empty tombstones (defaults).
// It blocks until commit and adopts the new generation only after success; call
// it from the boot goroutine, never from a JavaScript event callback.
func applyBundle(b, metadata, expected map[string]string) error {
	values := make(map[string]string, len(metadata)+len(perWorkspaceKeys))
	for key, value := range metadata {
		values[key] = value
	}
	for _, key := range perWorkspaceKeys {
		values[key] = b[key]
	}
	values, condition := prepareWorkspaceCommit(values, expected[datasetGenKey])
	for key, value := range expected {
		condition[key] = value
	}
	done := make(chan error, 1)
	browserstore.SetSecureBatch(values, condition, func(err error) { done <- err })
	if err := <-done; err != nil {
		return fmt.Errorf("replace startup workspace: %w", err)
	}
	datasetMyGen = values[datasetGenKey]
	return nil
}

func saveBlob(wsID string, b map[string]string) {
	if data, err := json.Marshal(b); err == nil {
		lsSet(wsBlobKey(wsID), string(data))
	}
}

func loadBlob(wsID string) map[string]string {
	b := map[string]string{}
	if raw := lsGet(wsBlobKey(wsID)); raw != "" {
		_ = json.Unmarshal([]byte(raw), &b)
	}
	return b
}

// switchWorkspace bundles out the current workspace, restores the target's
// bundle, marks it active, and reloads so boot rehydrates the new context.
func switchWorkspace(targetID string) {
	r := loadRegistry()
	if !r.Has(targetID) || r.ActiveID == targetID {
		return
	}
	suspendAutosave = true
	current := bundleCurrent()
	if current == nil {
		suspendAutosave = false
		return
	}
	target := loadBlob(targetID)
	next := r.SetActive(targetID)
	saveBlobThen(r.ActiveID, current, func() {
		applyBundleThen(target, func() {
			saveRegistryThen(next, reloadWorkspaceAfterWrites)
		})
	})
}

// createWorkspace adds a fresh, EMPTY workspace and switches to it: it clears the
// per-workspace UI keys (back to defaults) and seeds an explicitly empty dataset
// so the new workspace is a clean slate — not a copy of the current one, and not
// the demo sample. (Clearing the dataset key alone would make boot re-seed the
// sample, which looks like a clone of the current sample-based workspace.)
func createWorkspace(name string) {
	r := loadRegistry()
	current := bundleCurrent()
	if current == nil {
		return
	}
	newID := id.NewWithPrefix("ws")
	next := r.Add(newID, name).SetActive(newID).SetColor(newID, paletteColor(len(r.Workspaces)))
	data, err := store.Export(store.EmptyDataset())
	if err != nil {
		return
	}
	suspendAutosave = true
	dataset := string(data) // explicit empty dataset, not the sample
	saveBlobThen(r.ActiveID, current, func() {
		saveRegistryThen(next, func() {
			applyBundleThen(map[string]string{}, func() {
				lsSetThen(datasetStoreKey, dataset, reloadWorkspaceAfterWrites)
			})
		})
	})
}

// duplicateWorkspace clones the active workspace's data into a new one and
// switches to it.
func duplicateWorkspace(name string) {
	r := loadRegistry()
	suspendAutosave = true
	cur := bundleCurrent()
	if cur == nil {
		suspendAutosave = false
		return
	}
	newID := id.NewWithPrefix("ws")
	next := r.Add(newID, name).SetActive(newID).SetColor(newID, paletteColor(len(r.Workspaces)))
	saveBlobThen(r.ActiveID, cur, func() {
		saveBlobThen(newID, cur, func() {
			saveRegistryThen(next, func() {
				applyBundleThen(cur, reloadWorkspaceAfterWrites)
			})
		})
	})
}

// renameWorkspace updates a workspace's name in place (no reload — no context
// change). Returns the updated registry for the caller to refresh its view.
func renameWorkspace(wsID, name string) workspace.Registry {
	r := loadRegistry().Rename(wsID, name)
	saveRegistry(r)
	return r
}

// deleteWorkspace removes a workspace and its bundle. Deleting the active one
// switches to a survivor (with a reload); deleting an inactive one returns the
// updated registry so the caller can refresh. The last workspace can't be removed.
func deleteWorkspace(wsID string) workspace.Registry {
	r := loadRegistry()
	if len(r.Workspaces) <= 1 || !r.Has(wsID) {
		return r
	}
	wasActive := r.ActiveID == wsID
	r = r.Remove(wsID)
	if wasActive {
		suspendAutosave = true
		target := loadBlob(r.ActiveID)
		saveRegistryThen(r, func() {
			lsSetThen(wsBlobKey(wsID), "", func() {
				applyBundleThen(target, reloadWorkspaceAfterWrites)
			})
		})
	} else {
		lsSet(wsBlobKey(wsID), "")
		saveRegistry(r)
	}
	return r
}

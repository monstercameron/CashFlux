//go:build js && wasm

package app

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
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

func lsGet(key string) string {
	v := js.Global().Get("localStorage").Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}

func lsSet(key, val string) { js.Global().Get("localStorage").Call("setItem", key, val) }
func lsRemove(key string)   { js.Global().Get("localStorage").Call("removeItem", key) }
func reloadPage()           { js.Global().Get("location").Call("reload") }

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
	r := loadRegistry()
	target := r.StartupTarget()
	if target == "" || target == r.ActiveID {
		return
	}
	saveBlob(r.ActiveID, bundleCurrent())
	applyBundle(loadBlob(target))
	saveRegistry(r.SetActive(target))
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

func wsBlobKey(wsID string) string { return wsBlobPrefix + wsID }

// bundleCurrent snapshots the active workspace's per-workspace keys.
func bundleCurrent() map[string]string {
	out := make(map[string]string, len(perWorkspaceKeys))
	for _, k := range perWorkspaceKeys {
		if v := lsGet(k); v != "" {
			out[k] = v
		}
	}
	return out
}

// applyBundle writes a bundle back into the canonical keys, removing any key the
// bundle doesn't carry (so a workspace with no saved layout boots to defaults).
func applyBundle(b map[string]string) {
	for _, k := range perWorkspaceKeys {
		if v, ok := b[k]; ok {
			lsSet(k, v)
		} else {
			lsRemove(k)
		}
	}
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
	saveBlob(r.ActiveID, bundleCurrent())
	applyBundle(loadBlob(targetID))
	saveRegistry(r.SetActive(targetID))
	reloadPage()
}

// createWorkspace adds a fresh, EMPTY workspace and switches to it: it clears the
// per-workspace UI keys (back to defaults) and seeds an explicitly empty dataset
// so the new workspace is a clean slate — not a copy of the current one, and not
// the demo sample. (Clearing the dataset key alone would make boot re-seed the
// sample, which looks like a clone of the current sample-based workspace.)
func createWorkspace(name string) {
	r := loadRegistry()
	suspendAutosave = true
	saveBlob(r.ActiveID, bundleCurrent())
	newID := id.NewWithPrefix("ws")
	saveRegistry(r.Add(newID, name).SetActive(newID).SetColor(newID, paletteColor(len(r.Workspaces))))
	applyBundle(map[string]string{}) // clear UI keys → defaults
	if data, err := store.Export(store.EmptyDataset()); err == nil {
		lsSet(datasetStoreKey, string(data)) // explicit empty dataset, not the sample
	}
	reloadPage()
}

// duplicateWorkspace clones the active workspace's data into a new one and
// switches to it.
func duplicateWorkspace(name string) {
	r := loadRegistry()
	suspendAutosave = true
	cur := bundleCurrent()
	saveBlob(r.ActiveID, cur)
	newID := id.NewWithPrefix("ws")
	saveBlob(newID, cur)
	saveRegistry(r.Add(newID, name).SetActive(newID).SetColor(newID, paletteColor(len(r.Workspaces))))
	applyBundle(cur) // already current, but explicit
	reloadPage()
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
	lsRemove(wsBlobKey(wsID))
	r = r.Remove(wsID)
	saveRegistry(r)
	if wasActive {
		suspendAutosave = true
		applyBundle(loadBlob(r.ActiveID))
		reloadPage()
	}
	return r
}

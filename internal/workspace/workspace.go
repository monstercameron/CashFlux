// Package workspace models CashFlux's named, independent contexts. A workspace is
// a self-contained household — its own dataset and UI state — that the user can
// create, switch between, duplicate, and delete (e.g. "real money" vs. an
// "experimental" sandbox). Only the registry (which workspaces exist and which is
// active) lives here; the per-workspace data and its persistence live in the store
// and uistate layers, namespaced by the active workspace's ID.
//
// Pure Go, no platform dependencies; unit-tested on native Go. ID generation is
// the caller's job (so this stays deterministic and testable) — Add takes an id.
package workspace

// Workspace is one named context. Color is an optional accent (a CSS color
// string) used to tell workspaces apart at a glance in the switcher; empty means
// "no color set" and the UI falls back to a neutral.
type Workspace struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Registry is the ordered set of workspaces plus the active one's ID. The zero
// value is a valid empty registry.
//
// StartupID controls which workspace the app opens on launch: when empty (the
// default) it resumes the last-active workspace; when set to a workspace's id it
// always opens that one. A StartupID pointing at a removed workspace is cleared,
// so launch never targets a workspace that no longer exists.
type Registry struct {
	Workspaces []Workspace `json:"workspaces"`
	ActiveID   string      `json:"activeId"`
	StartupID  string      `json:"startupId,omitempty"`
}

// Has reports whether a workspace with the given id exists.
func (r Registry) Has(id string) bool {
	for _, w := range r.Workspaces {
		if w.ID == id {
			return true
		}
	}
	return false
}

// Get returns the workspace with the given id.
func (r Registry) Get(id string) (Workspace, bool) {
	for _, w := range r.Workspaces {
		if w.ID == id {
			return w, true
		}
	}
	return Workspace{}, false
}

// Active returns the active workspace. When ActiveID is unset or dangling it
// falls back to the first workspace, so callers always get a sane current
// context as long as at least one workspace exists.
func (r Registry) Active() (Workspace, bool) {
	if w, ok := r.Get(r.ActiveID); ok {
		return w, true
	}
	if len(r.Workspaces) > 0 {
		return r.Workspaces[0], true
	}
	return Workspace{}, false
}

// Add appends a new workspace (id assumed fresh; a duplicate id is ignored) and,
// when the registry was empty, makes it active. It does not switch to the new
// workspace otherwise — the caller decides whether the add is followed by a swap.
func (r Registry) Add(id, name string) Registry {
	if id == "" || r.Has(id) {
		return r
	}
	out := r.clone()
	out.Workspaces = append(out.Workspaces, Workspace{ID: id, Name: name})
	if out.ActiveID == "" {
		out.ActiveID = id
	}
	return out
}

// Rename changes a workspace's name; unknown ids are left unchanged.
func (r Registry) Rename(id, name string) Registry {
	if !r.Has(id) {
		return r
	}
	out := r.clone()
	for i := range out.Workspaces {
		if out.Workspaces[i].ID == id {
			out.Workspaces[i].Name = name
		}
	}
	return out
}

// SetColor sets a workspace's accent color (a CSS color string; "" clears it).
// Unknown ids are left unchanged.
func (r Registry) SetColor(id, color string) Registry {
	if !r.Has(id) {
		return r
	}
	out := r.clone()
	for i := range out.Workspaces {
		if out.Workspaces[i].ID == id {
			out.Workspaces[i].Color = color
		}
	}
	return out
}

// SetActive switches the active workspace; switching to an unknown id is a no-op.
func (r Registry) SetActive(id string) Registry {
	if !r.Has(id) {
		return r
	}
	out := r.clone()
	out.ActiveID = id
	return out
}

// Move repositions a workspace to toIndex (clamped into range), keeping the order
// of the others. A no-op for an unknown id, a list shorter than two, or a move
// that doesn't change position. The active and startup selections are unaffected
// (they're tracked by id, not position).
func (r Registry) Move(id string, toIndex int) Registry {
	if !r.Has(id) || len(r.Workspaces) < 2 {
		return r
	}
	from := 0
	for i, w := range r.Workspaces {
		if w.ID == id {
			from = i
			break
		}
	}
	if toIndex < 0 {
		toIndex = 0
	}
	if max := len(r.Workspaces) - 1; toIndex > max {
		toIndex = max
	}
	if from == toIndex {
		return r
	}
	out := r.clone()
	w := out.Workspaces[from]
	rest := append(out.Workspaces[:from:from], out.Workspaces[from+1:]...)
	final := make([]Workspace, 0, len(r.Workspaces))
	final = append(final, rest[:toIndex]...)
	final = append(final, w)
	final = append(final, rest[toIndex:]...)
	out.Workspaces = final
	return out
}

// SetStartup sets the workspace the app opens on launch. An empty id means
// "resume the last-active workspace" (the default); a non-empty id must name an
// existing workspace, otherwise the call is a no-op.
func (r Registry) SetStartup(id string) Registry {
	if id != "" && !r.Has(id) {
		return r
	}
	out := r.clone()
	out.StartupID = id
	return out
}

// StartupTarget resolves which workspace the app should open on launch: the
// pinned StartupID when it still names an existing workspace, otherwise the
// active workspace. Returns "" only for an empty registry.
func (r Registry) StartupTarget() string {
	if r.StartupID != "" && r.Has(r.StartupID) {
		return r.StartupID
	}
	if w, ok := r.Active(); ok {
		return w.ID
	}
	return ""
}

// Remove deletes a workspace. The last remaining workspace cannot be removed
// (there must always be one), and an unknown id is a no-op. When the active
// workspace is removed, the first survivor becomes active.
func (r Registry) Remove(id string) Registry {
	if !r.Has(id) || len(r.Workspaces) <= 1 {
		return r
	}
	out := Registry{ActiveID: r.ActiveID, StartupID: r.StartupID}
	for _, w := range r.Workspaces {
		if w.ID != id {
			out.Workspaces = append(out.Workspaces, w)
		}
	}
	if out.ActiveID == id {
		out.ActiveID = out.Workspaces[0].ID
	}
	if out.StartupID == id {
		out.StartupID = "" // never pin launch to a removed workspace
	}
	return out
}

func (r Registry) clone() Registry {
	cp := Registry{ActiveID: r.ActiveID, StartupID: r.StartupID}
	cp.Workspaces = append([]Workspace(nil), r.Workspaces...)
	return cp
}

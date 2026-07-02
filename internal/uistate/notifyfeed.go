// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	notifyFeedAtomID  = "app:notify-feed"
	notifyFeedStoreID = "cashflux:notify:feed"
	notifyFeedCap     = 50
	notifyBrowserKey  = "cashflux:notify:browser" // "1" when browser notifications are enabled
)

// capturedNotifyFeed holds the live feed atom captured during render, so boot
// catch-up and event-handler mutators can push updates WITHOUT calling the
// UseNotifyFeed hook outside a component (which panics). Mirrors notice.go.
var (
	capturedNotifyFeed state.Atom[[]FeedItem]
	notifyFeedCaptured bool
)

// UseNotifyFilter is the shared severity filter for the Notification Center surface
// ("" = all, or "critical"/"warning"/"info"). Read by the toolbar + list tiles.
func UseNotifyFilter() state.Atom[string] { return state.UseAtom("notify:filter", "") }

// UseNotifyFeed returns the shared, persisted Notification Center feed (newest
// first). The catch-up runner appends to it; the center screen renders it. It also
// captures the atom so out-of-render code can update it via setNotifyFeed.
func UseNotifyFeed() state.Atom[[]FeedItem] {
	a := state.UseAtom(notifyFeedAtomID, loadNotifyFeed())
	capturedNotifyFeed = a
	notifyFeedCaptured = true
	return a
}

// setNotifyFeed pushes a new feed into the live atom from non-render code (the
// boot-time catch-up runner, event-handler mutators) via the captured reference.
// Calling the UseNotifyFeed hook there panics ("GoUseAtom called outside component
// context"); before the first render the captured atom is nil and the KV write
// (done by callers) is the durable source, so a no-op here is safe.
func setNotifyFeed(items []FeedItem) {
	if notifyFeedCaptured {
		capturedNotifyFeed.Set(items)
	}
}

// PersistNotifyFeed saves the feed (capped at notifyFeedCap, newest kept).
func PersistNotifyFeed(items []FeedItem) {
	if len(items) > notifyFeedCap {
		items = items[:notifyFeedCap]
	}
	if data, err := json.Marshal(items); err == nil {
		kvSet(notifyFeedStoreID, string(data)) // SQLite-backed app KV (not localStorage)
	}
}

// PrependNotifyFeed adds new items to the front of the feed (newest first),
// persists them, and immediately pushes the new feed into the live atom so
// every subscriber (Notification Center screen, rail badge) sees the update
// regardless of component mount order.
//
// C270 / closes C121 C158 C159: without the atom push, runNotifyCatchUp fires
// before the Notification Center screen mounts. UseAtom only uses its default
// value the first time the atom is created, so any atom that was already
// created (e.g. for the rail unread badge) holds a stale empty feed — the KV
// write from PersistNotifyFeed is invisible to it. The fix mirrors the
// existing pattern in runNotifyCatchUp where UseNotice().Set(...) is called
// from non-render boot code; setNotifyFeed(...) is equally safe there.
func PrependNotifyFeed(items []FeedItem) {
	if len(items) == 0 {
		return
	}
	cur := loadNotifyFeed()
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		seen[it.ID] = true
	}
	out := make([]FeedItem, 0, len(items)+len(cur))
	out = append(out, items...)
	for _, it := range cur {
		if !seen[it.ID] {
			out = append(out, it)
		}
	}
	// Cap before persisting so KV and atom hold exactly the same slice.
	if len(out) > notifyFeedCap {
		out = out[:notifyFeedCap]
	}
	PersistNotifyFeed(out)
	// Push into the live atom so all current subscribers update immediately,
	// even if they were created before this call (mount-order hazard, C270).
	setNotifyFeed(out)
}

// MarkFeedItemRead sets the Read flag on the item with the given id, persists
// the updated feed, and pushes the new slice into the live atom (C268).
// If the id is not found, the call is a no-op.
func MarkFeedItemRead(id string, read bool) {
	cur := loadNotifyFeed()
	changed := false
	for i, it := range cur {
		if it.ID == id && it.Read != read {
			cur[i].Read = read
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	PersistNotifyFeed(cur)
	setNotifyFeed(cur)
}

// MarkAllNotifyRead marks every unread item in the CURRENT persisted feed read (used to
// clear the rail badge when the center opens). It reads the live store — not a render
// snapshot — so it never clobbers a snooze/dismiss the user just performed. No-op when
// everything is already read.
func MarkAllNotifyRead() {
	cur := loadNotifyFeed()
	changed := false
	for i := range cur {
		if !cur[i].Read {
			cur[i].Read = true
			changed = true
		}
	}
	if !changed {
		return
	}
	PersistNotifyFeed(cur)
	setNotifyFeed(cur)
}

// DismissFeedItem removes the item with the given id from the feed, persists
// the result, and pushes the new slice into the live atom (C268).
func DismissFeedItem(id string) {
	cur := loadNotifyFeed()
	out := make([]FeedItem, 0, len(cur))
	found := false
	for _, it := range cur {
		if it.ID == id {
			found = true
			continue
		}
		out = append(out, it)
	}
	if !found {
		return
	}
	PersistNotifyFeed(out)
	setNotifyFeed(out)
}

// SnoozeFeedItem sets SnoozedUntil on the item with the given id to the
// provided unix-second timestamp, persists, and pushes the live atom (C268).
// Pass until=0 to clear a snooze.
func SnoozeFeedItem(id string, until int64) {
	cur := loadNotifyFeed()
	changed := false
	for i, it := range cur {
		if it.ID == id && it.SnoozedUntil != until {
			cur[i].SnoozedUntil = until
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	PersistNotifyFeed(cur)
	setNotifyFeed(cur)
}

func loadNotifyFeed() []FeedItem {
	raw := kvGet(notifyFeedStoreID)
	if raw == "" {
		return nil
	}
	var items []FeedItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

// UnreadNotifyCount returns how many feed items are unread (for a rail badge).
func UnreadNotifyCount(items []FeedItem) int {
	n := 0
	for _, it := range items {
		if !it.Read {
			n++
		}
	}
	return n
}

// BrowserNotifyEnabled reports whether the user has opted into OS/browser
// notifications (defaults off until explicitly enabled).
func BrowserNotifyEnabled() bool {
	return browserstore.GetString(notifyBrowserKey) == "1"
}

// SetBrowserNotifyEnabled persists the browser-notification opt-in.
func SetBrowserNotifyEnabled(on bool) {
	val := "0"
	if on {
		val = "1"
	}
	browserstore.Set(notifyBrowserKey, val)
}

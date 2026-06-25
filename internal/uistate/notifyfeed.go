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

// FeedItem is one entry in the Notification Center (C75): the title/body of an
// emitted notification, when it fired, and whether the user has read it.
type FeedItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	At    int64  `json:"at"` // unix seconds
	Read  bool   `json:"read,omitempty"`
}

// UseNotifyFeed returns the shared, persisted Notification Center feed (newest
// first). The catch-up runner appends to it; the center screen renders it.
func UseNotifyFeed() state.Atom[[]FeedItem] {
	return state.UseAtom(notifyFeedAtomID, loadNotifyFeed())
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
// from non-render boot code; UseNotifyFeed().Set(...) is equally safe there.
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
	UseNotifyFeed().Set(out)
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

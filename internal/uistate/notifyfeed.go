//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

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
		js.Global().Get("localStorage").Call("setItem", notifyFeedStoreID, string(data))
	}
}

// PrependNotifyFeed adds new items to the front of the feed (newest first) and
// persists, dropping ids already present so re-runs don't duplicate.
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
	PersistNotifyFeed(out)
}

func loadNotifyFeed() []FeedItem {
	v := js.Global().Get("localStorage").Call("getItem", notifyFeedStoreID)
	if v.IsNull() || v.IsUndefined() {
		return nil
	}
	var items []FeedItem
	if err := json.Unmarshal([]byte(v.String()), &items); err != nil {
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
	v := js.Global().Get("localStorage").Call("getItem", notifyBrowserKey)
	return !v.IsNull() && !v.IsUndefined() && v.String() == "1"
}

// SetBrowserNotifyEnabled persists the browser-notification opt-in.
func SetBrowserNotifyEnabled(on bool) {
	val := "0"
	if on {
		val = "1"
	}
	js.Global().Get("localStorage").Call("setItem", notifyBrowserKey, val)
}

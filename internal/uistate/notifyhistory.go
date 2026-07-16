// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/notifyhistory"
)

// notifHistoryKVKey is the single SQLite-backed KV key under which the whole
// notification archive is stored as one JSON blob (mirrors the occurrences
// paid-map pattern — no SQL schema/migration).
const notifHistoryKVKey = "cashflux:notifhistory"

// The archive is loaded lazily from KV on first use and cached in-process; each
// mutation updates the cache and writes the whole blob back. This is the ONLY
// place uistate.FeedItem and notifyhistory.Record meet.
var (
	histArchive notifyhistory.Archive
	histLoaded  bool
)

// loadHistory hydrates the cached archive from KV once (tolerant of an
// absent/corrupt blob — that yields an empty archive).
func loadHistory() {
	if histLoaded {
		return
	}
	histArchive, _ = notifyhistory.Unmarshal(kvGet(notifHistoryKVKey))
	histLoaded = true
}

// saveHistory persists the cached archive as one JSON blob.
func saveHistory() {
	if s, err := notifyhistory.Marshal(histArchive); err == nil {
		kvSet(notifHistoryKVKey, s)
	}
}

// feedMessage flattens a feed item to a single searchable message line.
func feedMessage(it FeedItem) string {
	if it.Title != "" {
		return it.Title
	}
	return it.Body
}

// ArchiveItems returns the archived records matching an optional case-insensitive
// message query and optional severity ("" = any). Newest first.
func ArchiveItems(query, severity string) []notifyhistory.Record {
	loadHistory()
	return histArchive.Filter(query, severity)
}

// ArchiveUnreadCount reports how many archived records are unread.
func ArchiveUnreadCount() int {
	loadHistory()
	return histArchive.UnreadCount()
}

// RecordNotification archives one notification (dedupe by a derived ID, so
// re-recording the same alert is idempotent) and persists. The ID is derived
// from severity+time+message; callers with a stable feed ID should prefer
// SyncFeedToArchive, which keys on that ID.
func RecordNotification(sev, msg, route string, at int64) {
	loadHistory()
	histArchive.Add(notifyhistory.Record{
		ID:       fmt.Sprintf("%s|%d|%s", sev, at, msg),
		Severity: sev,
		Message:  msg,
		Route:    route,
		At:       at,
	})
	saveHistory()
}

// SyncFeedToArchive folds every item currently in the live notification feed
// into the archive, keyed on each item's stable feed ID so past alerts persist
// even after they are dismissed from the live feed. Dedupe-by-ID makes it safe
// to call on every open. Persists once.
func SyncFeedToArchive() {
	loadHistory()
	feed := loadNotifyFeed()
	if len(feed) == 0 {
		return
	}
	for _, it := range feed {
		histArchive.Add(notifyhistory.Record{
			ID:       it.ID,
			Severity: it.Severity,
			Message:  feedMessage(it),
			Route:    RouteForNotifyID(it.ID),
			At:       it.At,
			Read:     it.Read,
		})
	}
	saveHistory()
}

// ClearNotificationHistory empties the archive and persists the empty blob.
func ClearNotificationHistory() {
	histArchive = notifyhistory.Archive{}
	histLoaded = true
	saveHistory()
}

// MarkNotificationHistoryRead flags every archived record read and persists.
func MarkNotificationHistoryRead() {
	loadHistory()
	histArchive.MarkAllRead()
	saveHistory()
}

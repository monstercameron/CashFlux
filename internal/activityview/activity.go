// SPDX-License-Identifier: MIT

// Package activityview prepares the bounded Activity timeline for rendering.
package activityview

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditundo"
	"github.com/monstercameron/CashFlux/internal/checkpoint"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Source identifies the data source selected for the prepared timeline.
type Source string

const (
	// SourceAuditFeed means the in-session audit feed supplied the entries.
	SourceAuditFeed Source = "auditFeed"
	// SourceEntitySnapshot means transactions and tasks supplied fallback entries.
	SourceEntitySnapshot Source = "entitySnapshot"
)

// Input contains independent availability for the audit feed and entity snapshot.
// AuditEntries should be the already-bounded feed snapshot returned by Feed.Recent.
type Input struct {
	AuditEntries      []auditlog.Entry
	AuditAvailable    bool
	Transactions      []domain.Transaction
	Tasks             []domain.Task
	SnapshotAvailable bool
	Limit             int
	Filter            string
	Checkpoints       []checkpoint.Checkpoint
}

// Row pairs a displayed entry with its undo assessment, prepared once for this view.
type Row struct {
	Entry auditlog.Entry
	Undo  auditundo.Assessment
}

// Day groups contiguous visible rows by their local date key. Undated entries use
// an empty key and remain at the end in the order prepared by Build.
type Day struct {
	Key  string
	Rows []Row
}

// Hero contains facts for the unfiltered bounded feed.
type Hero struct {
	Count      int
	YouCount   int
	OtherCount int
	Latest     auditlog.Entry
	HasLatest  bool
}

// Model is an immutable, bounded view of the activity feed.
type Model struct {
	Source            Source
	AuditAvailable    bool
	SnapshotAvailable bool
	Available         bool
	Entries           []auditlog.Entry
	Rows              []Row
	Days              []Day
	Hero              Hero
}

// Build selects, orders, redacts and caps a source feed, then prepares the filter,
// hero facts, day groups and per-row undo assessments. It never mutates inputs.
func Build(in Input) Model {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	model := Model{AuditAvailable: in.AuditAvailable, SnapshotAvailable: in.SnapshotAvailable}
	if len(in.AuditEntries) > 0 {
		model.Source = SourceAuditFeed
		model.Available = in.AuditAvailable
		model.Entries = append([]auditlog.Entry(nil), in.AuditEntries...)
		sort.SliceStable(model.Entries, func(i, j int) bool {
			zi, zj := model.Entries[i].At.IsZero(), model.Entries[j].At.IsZero()
			if zi != zj {
				return zj
			}
			return model.Entries[i].At.After(model.Entries[j].At)
		})
		for i := range model.Entries {
			model.Entries[i].EntityType = EntityLabel(model.Entries[i].EntityType)
			model.Entries[i].Summary = HumanizeSummary(model.Entries[i].Summary)
		}
	} else {
		model.Source = SourceEntitySnapshot
		model.Available = in.SnapshotAvailable
		model.Entries = fallbackEntries(in.Transactions, in.Tasks)
		if len(model.Entries) > limit {
			model.Entries = model.Entries[:limit]
		}
	}
	model.Hero = heroFor(model.Entries)
	filtered := model.Entries
	if in.Filter != "" {
		filtered = make([]auditlog.Entry, 0, len(model.Entries))
		for _, e := range model.Entries {
			if e.EntityType == in.Filter {
				filtered = append(filtered, e)
			}
		}
	}
	model.Rows = make([]Row, len(filtered))
	for i, e := range filtered {
		model.Rows[i] = Row{Entry: e, Undo: auditundo.Assess(e, model.Entries, in.Checkpoints)}
	}
	model.Days = groupDays(model.Rows)
	return model
}

func fallbackEntries(transactions []domain.Transaction, tasks []domain.Task) []auditlog.Entry {
	entries := make([]auditlog.Entry, 0, len(transactions)+len(tasks))
	for _, t := range transactions {
		desc := t.Desc
		if desc == "" {
			desc = t.Payee
		}
		entries = append(entries, auditlog.Entry{ID: "txn-" + t.ID, At: t.Date, Actor: "user", Action: "added",
			EntityType: "transaction", EntityID: t.ID,
			Summary: auditlog.Redact(fmt.Sprintf("Transaction: %s", truncate(desc, 60)))})
	}
	for _, t := range tasks {
		entries = append(entries, auditlog.Entry{ID: "task-" + t.ID, At: t.Due, Actor: "user", Action: "added",
			EntityType: "task", EntityID: t.ID,
			Summary: auditlog.Redact(fmt.Sprintf("Task: %s", truncate(t.Title, 60)))})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].At.After(entries[j].At) })
	return entries
}

func heroFor(entries []auditlog.Entry) Hero {
	h := Hero{Count: len(entries)}
	if len(entries) > 0 {
		h.Latest, h.HasLatest = entries[0], true
	}
	for _, e := range entries {
		if e.Actor == "user" || e.Actor == "" {
			h.YouCount++
		} else {
			h.OtherCount++
		}
	}
	return h
}

func groupDays(rows []Row) []Day {
	var days []Day
	for _, row := range rows {
		key := ""
		if !row.Entry.At.IsZero() {
			key = row.Entry.At.Format("2006-01-02")
		}
		if len(days) == 0 || days[len(days)-1].Key != key {
			days = append(days, Day{Key: key})
		}
		last := len(days) - 1
		days[last].Rows = append(days[last].Rows, row)
	}
	return days
}

// EntityLabel maps persisted legacy collection names to user-facing entity words.
func EntityLabel(t string) string {
	switch t {
	case "auditEntries", "auditEntry":
		return "history"
	case "placements", "placement":
		return "dashboard layout"
	case "recurring":
		return "recurring item"
	case "settingsState", "settings":
		return "settings"
	case "customPages":
		return "page"
	case "sharedExpenses":
		return "shared expense"
	default:
		return t
	}
}

// HumanizeSummary repairs machine-speak in legacy persisted summaries.
func HumanizeSummary(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 4 && fields[3] == "records" {
		if label := EntityLabel(fields[2]); label != fields[2] {
			return fields[0] + " " + fields[1] + " " + label + " records"
		}
		return s
	}
	if len(fields) == 3 && fields[1] == "a" && fields[2] == "settings" {
		return fields[0] + " settings"
	}
	if len(fields) == 3 {
		if label := EntityLabel(fields[1]); label != fields[1] {
			return fields[0] + " " + label
		}
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

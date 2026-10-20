// SPDX-License-Identifier: MIT

package taskview

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasklink"
	"github.com/monstercameron/CashFlux/internal/tasktree"
)

// Link identifies one task's related entity without concatenating its fields.
type Link struct {
	Type domain.RelatedType
	ID   string
}

// SnapshotInput supplies the full task set and the entities named by task rows.
// TodayISO is a calendar date in YYYY-MM-DD form, independent of view filters.
type SnapshotInput struct {
	Tasks        []domain.Task
	Accounts     []domain.Account
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Transactions []domain.Transaction
	Members      []domain.Member
	TodayISO     string
}

// Snapshot shares counts and lookup indexes across the task surface. Maps are
// owned by this result and must be treated as read-only by consumers. EntityNames
// retains only entities referenced by tasks, bounding its size by task links.
type Snapshot struct {
	Counts      Counts
	ByID        map[string]domain.Task
	Children    map[string]tasktree.ChildStat
	OpenLinks   map[Link]bool
	EntityNames map[Link]string
	MemberNames map[string]string
}

// BuildSnapshot prepares one revision's task data. Task ID lookup keeps the last
// duplicate, matching task mutations; entity/member display lookup keeps the
// first duplicate, matching tasklink.EntityName and the prior row search.
// Input slices and nested values are never modified.
func BuildSnapshot(in SnapshotInput) Snapshot {
	out := Snapshot{
		Counts: Count(in.Tasks, in.TodayISO), ByID: make(map[string]domain.Task, len(in.Tasks)),
		Children: tasktree.ChildStats(in.Tasks), OpenLinks: make(map[Link]bool),
		EntityNames: make(map[Link]string), MemberNames: make(map[string]string),
	}
	wanted := make(map[Link]bool)
	remaining := make(map[domain.RelatedType]int)
	wantedMembers := make(map[string]bool)
	for _, task := range in.Tasks {
		out.ByID[task.ID] = task
		if task.RelatedID != "" {
			link := Link{task.RelatedType, task.RelatedID}
			if !wanted[link] {
				remaining[link.Type]++
			}
			wanted[link] = true
			if task.Status == domain.StatusOpen {
				out.OpenLinks[link] = true
			}
		}
		if task.MemberID != "" {
			wantedMembers[task.MemberID] = true
		}
	}
	put := func(kind domain.RelatedType, id, name string) {
		link := Link{kind, id}
		if _, exists := out.EntityNames[link]; wanted[link] && !exists {
			out.EntityNames[link] = name
			remaining[kind]--
		}
	}
	for _, account := range in.Accounts {
		if remaining[domain.RelatedAccount] == 0 {
			break
		}
		put(domain.RelatedAccount, account.ID, account.Name)
	}
	for _, budget := range in.Budgets {
		if remaining[domain.RelatedBudget] == 0 {
			break
		}
		put(domain.RelatedBudget, budget.ID, budget.Name)
	}
	for _, goal := range in.Goals {
		if remaining[domain.RelatedGoal] == 0 {
			break
		}
		put(domain.RelatedGoal, goal.ID, goal.Name)
	}
	for _, txn := range in.Transactions {
		if remaining[domain.RelatedTransaction] == 0 {
			break
		}
		name := txn.Payee
		if name == "" {
			name = txn.Desc
		}
		put(domain.RelatedTransaction, txn.ID, name)
	}
	for _, member := range in.Members {
		if _, exists := out.MemberNames[member.ID]; wantedMembers[member.ID] && !exists {
			out.MemberNames[member.ID] = member.Name
		}
	}
	return out
}

// EntityName resolves a task link from the prepared index. Review-inbox links
// remain resolvable without a stored entity, exactly like tasklink.EntityName.
func (s Snapshot) EntityName(kind domain.RelatedType, id string) (string, bool) {
	if id == "" || kind == "" || kind == domain.RelatedNone {
		return "", false
	}
	if kind == domain.RelatedReviewQueue {
		return tasklink.EntityName(kind, id, nil, nil, nil, nil)
	}
	name, ok := s.EntityNames[Link{kind, id}]
	return name, ok
}

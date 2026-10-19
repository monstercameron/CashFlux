// SPDX-License-Identifier: MIT

package reviewview

import "github.com/monstercameron/CashFlux/internal/reviewscope"

// Session holds the review task's scope, mode and merchant identity. Its shape
// matches the persisted atom so the browser adapter can convert without logic.
type Session struct {
	ScopeChosen bool
	Scope       reviewscope.Scope
	Commit      reviewscope.CommitScope
	CommitKey   string
	Mode        string
	Cursor      int
	CursorKey   string
}

// Normalized gives persisted unknown values the existing safe defaults.
func (s Session) Normalized() Session {
	s.Scope = reviewscope.Normalize(s.Scope)
	s.Commit = reviewscope.NormalizeCommit(s.Commit)
	if s.Mode != "single" && s.Mode != "bulk" {
		s.Mode = "bulk"
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	return s
}

// WithScope chooses a population and resets both parts of the previous position.
func (s Session) WithScope(scope string) Session {
	s = s.Normalized()
	s.ScopeChosen, s.Scope = true, reviewscope.Normalize(reviewscope.Scope(scope))
	s.Cursor, s.CursorKey = 0, ""
	return s
}

// WithMode changes presentation without losing the current card or scope.
func (s Session) WithMode(mode string) Session {
	s.Mode = mode
	return s.Normalized()
}

// WithCursor records the merchant and slot together, preserving commit identity.
func (s Session) WithCursor(position int, key string) Session {
	s.Cursor, s.CursorKey = position, key
	return s.Normalized()
}

// WithCommit binds a commit scope to the merchant for which it was chosen.
func (s Session) WithCommit(scope, merchant string) Session {
	s.Commit, s.CommitKey = reviewscope.NormalizeCommit(reviewscope.CommitScope(scope)), merchant
	return s.Normalized()
}

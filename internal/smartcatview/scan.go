// SPDX-License-Identifier: MIT

// Package smartcatview prepares category and capture views using platform-independent data.
package smartcatview

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/txnreview"
)

// The scan modes preserve the Smart+ modal's persisted identifiers.
const (
	// Suggest proposes new categories.
	Suggest = "suggest"
	// Auto categorizes unfiled transactions.
	Auto = "auto"
	// Recat reviews already categorized transactions.
	Recat = "recat"
	// ScanCap bounds model context and the review list.
	ScanCap = 40
)

// Assignment is one reviewable transaction/category pair.
type Assignment struct {
	TxnID, Label, Amount, Current, CatID, CatName string
}

// Catalog builds qualified paths with the same one-parent policy as Smart+ prompts.
func Catalog(cats []domain.Category) smartai.Catalog {
	names := categoryNames(cats)
	out := make([]smartai.Cat, 0, len(cats))
	for _, c := range cats {
		path := c.Name
		if parent, ok := names[c.ParentID]; ok && c.ParentID != c.ID {
			path = parent + smartai.PathSep + c.Name
		}
		out = append(out, smartai.Cat{ID: c.ID, Path: path, Income: c.Kind == domain.KindIncome})
	}
	return smartai.NewCatalog(out)
}

func categoryNames(cats []domain.Category) map[string]string {
	out := make(map[string]string, len(cats))
	for _, c := range cats {
		out[c.ID] = c.Name
	}
	return out
}

func formatMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}

func scanLabel(t domain.Transaction) string { return strings.TrimSpace(t.Payee + " \u2014 " + t.Desc) }

// Scan is a bounded, ordered sample and its model request.
type Scan struct {
	Sample  []domain.Transaction
	Request smartai.Request
}

// BuildScan samples matching transactions without sorting, combining currencies or using FX.
func BuildScan(mode string, txns []domain.Transaction, cats []domain.Category) Scan {
	var out Scan
	var lines strings.Builder
	names := categoryNames(cats)
	for _, t := range txns {
		if t.IsTransfer() || (t.CategoryID != "") != (mode == Recat) {
			continue
		}
		out.Sample = append(out.Sample, t)
		line := scanLabel(t) + " | " + formatMoney(t.Amount)
		if mode == Suggest {
			line = "- " + line
		} else {
			line = strconv.Itoa(len(out.Sample)) + " | " + line
		}
		if mode == Recat {
			cur := names[t.CategoryID]
			if cur == "" {
				cur = "(uncategorized)"
			}
			line += " | currently: " + cur
		}
		lines.WriteString(line + "\n")
		if len(out.Sample) >= ScanCap {
			break
		}
	}
	prompt := Catalog(cats).Prompt()
	switch mode {
	case Suggest:
		out.Request = smartai.SuggestCategories(lines.String(), prompt)
	case Recat:
		out.Request = smartai.Recategorize(lines.String(), prompt)
	default:
		out.Request = smartai.AutoCategorize(lines.String(), prompt)
	}
	return out
}

// ParseAssignments rejects invented categories, sign mismatches and no-op assignments.
func ParseAssignments(text string, sample []domain.Transaction, cats []domain.Category) []Assignment {
	income := make(map[int]bool, len(sample))
	for i, t := range sample {
		income[i+1] = t.Amount.IsPositive()
	}
	parsed := smartai.RejectSignMismatches(smartai.ParseCategoryAssignments(text, len(sample), Catalog(cats)), income)
	names := categoryNames(cats)
	var out []Assignment
	for _, a := range parsed {
		t := sample[a.Ref-1]
		if t.CategoryID == a.CategoryID {
			continue
		}
		out = append(out, Assignment{TxnID: t.ID, Label: scanLabel(t), Amount: formatMoney(t.Amount), Current: names[t.CategoryID], CatID: a.CategoryID, CatName: a.CategoryName})
	}
	return out
}

// ParseSuggestions filters the model's category proposals against existing names.
func ParseSuggestions(text string, cats []domain.Category) []smartai.SuggestedCategory {
	existing := make(map[string]bool, len(cats))
	for _, c := range cats {
		existing[strings.ToLower(c.Name)] = true
	}
	return smartai.ParseCategorySuggestions(text, existing)
}

// NewCategory builds a proposed category with its first matching existing parent.
func NewCategory(s smartai.SuggestedCategory, cats []domain.Category, id string) domain.Category {
	c := domain.Category{ID: id, Name: s.Name, Kind: domain.CategoryKind(s.Kind)}
	if s.ParentName != "" {
		for _, p := range cats {
			if strings.EqualFold(p.Name, s.ParentName) {
				c.ParentID = p.ID
				break
			}
		}
	}
	return c
}

// State holds the review modal's transient state.
type State struct {
	Mode             string
	Loading, Scanned bool
	Error            string
	Suggestions      []smartai.SuggestedCategory
	Assignments      []Assignment
	Picked           map[int]bool
}

// Reset starts a mode with no previous results.
func Reset(mode string) State { return State{Mode: mode, Picked: map[int]bool{}} }

// Start begins a scan, retaining previous choices until a response arrives.
func (s State) Start() State { s.Loading = true; s.Error = ""; s.Scanned = false; return s }

// Finish installs a successful response and selects each returned row.
func (s State) Finish(mode string, suggestions []smartai.SuggestedCategory, assignments []Assignment) State {
	s.Loading = false
	s.Scanned = true
	n := len(assignments)
	if mode == Suggest {
		s.Suggestions = suggestions
		n = len(suggestions)
	} else {
		s.Assignments = assignments
	}
	s.Picked = make(map[int]bool, n)
	for i := 0; i < n; i++ {
		s.Picked[i] = true
	}
	return s
}

// Fail completes a failed scan without discarding the previous choices.
func (s State) Fail(message string) State {
	s.Error = message
	s.Scanned = true
	s.Loading = false
	return s
}

// Toggle returns independent selection storage, preserving even out-of-range entries.
func (s State) Toggle(i int) State {
	picked := make(map[int]bool, len(s.Picked)+1)
	for k, v := range s.Picked {
		picked[k] = v
	}
	picked[i] = !picked[i]
	s.Picked = picked
	return s
}

// ReviewRow is a prepared checklist row.
type ReviewRow struct {
	Key, Label, Sub, KindKey, TestID string
	Index                            int
	Checked                          bool
}

// ReviewView holds label keys and mutually exclusive result visibility.
type ReviewView struct {
	HintKey, ScanKey, ApplyKey, Result, EmptyKey, ApplySuffix string
	Selected                                                  int
	ShowApply, ApplyDisabled                                  bool
	Rows                                                      []ReviewRow
}

// PresentReview prepares rows and label decisions with the original precedence.
func PresentReview(s State, provider bool) ReviewView {
	v := ReviewView{HintKey: "smartcat.hintSuggest", ScanKey: "smartcat.scan", ApplyKey: "smartcat.createSelected"}
	switch s.Mode {
	case Auto:
		v.HintKey = "smartcat.hintAuto"
	case Recat:
		v.HintKey = "smartcat.hintRecat"
	}
	if s.Mode != Suggest {
		v.ApplyKey = "smartcat.applySelected"
	}
	if s.Scanned {
		v.ScanKey = "smartcat.rescan"
	}
	for _, p := range s.Picked {
		if p {
			v.Selected++
		}
	}
	n := len(s.Assignments)
	if s.Mode == Suggest {
		n = len(s.Suggestions)
	}
	v.ShowApply = s.Scanned && n > 0
	v.EmptyKey = "smartcat.noneAssign"
	if s.Mode == Suggest {
		v.EmptyKey = "smartcat.noneSuggest"
	}
	v.ApplySuffix = " (" + strconv.Itoa(v.Selected) + ")"
	v.ApplyDisabled = v.Selected == 0
	switch {
	case !provider:
		v.Result = "provider"
	case s.Loading:
		v.Result = "loading"
	case s.Error != "":
		v.Result = "error"
	case s.Scanned && n == 0:
		v.Result = "empty"
	case s.Scanned:
		v.Result = "rows"
	}
	if s.Mode == Suggest {
		indices := map[string]int{}
		for i, r := range s.Suggestions {
			indices[r.Name] = i
		}
		for _, r := range s.Suggestions {
			i := indices[r.Name]
			key := "smartcat.kindExpense"
			if r.Kind == "income" {
				key = "smartcat.kindIncome"
			}
			v.Rows = append(v.Rows, ReviewRow{Key: r.Name, Index: i, Label: r.Name, KindKey: key, Checked: s.Picked[i], TestID: "smartcat-sug-" + strconv.Itoa(i)})
		}
	} else {
		indices := map[string]int{}
		for i, r := range s.Assignments {
			indices[r.TxnID] = i
		}
		for _, r := range s.Assignments {
			i := indices[r.TxnID]
			sub := "\u2192 " + r.CatName + " \u00b7 " + r.Amount
			if s.Mode == Recat && r.Current != "" {
				sub = r.Current + " \u2192 " + r.CatName + " \u00b7 " + r.Amount
			}
			v.Rows = append(v.Rows, ReviewRow{Key: r.TxnID, Index: i, Label: r.Label, Sub: sub, Checked: s.Picked[i], TestID: "smartcat-row-" + strconv.Itoa(i)})
		}
	}
	return v
}

// AssignmentWrites confirms selected assignments against live transactions.
func AssignmentWrites(rows []Assignment, picked map[int]bool, lookup func(string) (domain.Transaction, bool)) ([]domain.Transaction, []string) {
	var writes []domain.Transaction
	var ids []string
	for i, r := range rows {
		if !picked[i] {
			continue
		}
		t, ok := lookup(r.TxnID)
		if !ok || t.IsTransfer() {
			continue
		}
		writes = append(writes, txnreview.Confirm(t, r.CatID))
		ids = append(ids, t.ID)
	}
	return writes, ids
}

// NoSample completes an empty scan while retaining its existing selection map.
func (s State) NoSample(mode string) State {
	s.Loading = false
	s.Scanned = true
	if mode == Suggest {
		s.Suggestions = nil
	} else {
		s.Assignments = nil
	}
	return s
}

// CategoryDrafts prepares selected proposals in input order, leaving IDs to the caller.
func CategoryDrafts(suggestions []smartai.SuggestedCategory, picked map[int]bool, cats []domain.Category) []domain.Category {
	var out []domain.Category
	for i, s := range suggestions {
		if picked[i] {
			out = append(out, NewCategory(s, cats, ""))
		}
	}
	return out
}

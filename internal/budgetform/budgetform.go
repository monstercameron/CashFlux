// SPDX-License-Identifier: MIT

// Package budgetform prepares budget drafts and their previews without browser dependencies.
package budgetform

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
)

// NewCategory is the legacy picker value requesting a new category.
const NewCategory = "__new_category__"

// Draft is the editable, unparsed form state. Maps are treated as immutable.
type Draft struct {
	Name, Limit, Currency, CategoryID, NewCategoryName       string
	Direction, Owner, Period, Methodology, Tags, VarName     string
	CreateCategory, Rollover, Advanced, Template, VarTouched bool
	Also, Excluded                                           map[string]bool
	Buckets                                                  map[string]budgeting.Bucket
	Custom                                                   map[string]string
}

// Initial supplies the defaults for an optionally seeded form.
func Initial(name, limit, category, period, base string) Draft {
	if base == "" {
		base = "USD"
	}
	if period == "" {
		period = string(domain.PeriodMonthly)
	}
	return Draft{Name: name, Limit: limit, Currency: base, CategoryID: category, Period: period,
		Direction: string(domain.DirectionSpend), Owner: domain.GroupOwnerID, Advanced: category != ""}
}

// ChangeName updates the automatic formula handle until the user edits it.
func ChangeName(d Draft, name string) Draft {
	d.Name = name
	if !d.VarTouched {
		d.VarName = engineenv.BudgetVarSlug(name)
	}
	return d
}

// Toggle returns an independent selection map with one entry toggled.
func Toggle(values map[string]bool, key string) map[string]bool {
	next := maps.Clone(values)
	if next == nil {
		next = map[string]bool{}
	}
	next[key] = !next[key]
	return next
}

// SetCustom returns an independent string map with the supplied field updated.
func SetCustom(values map[string]string, key, value string) map[string]string {
	next := maps.Clone(values)
	if next == nil {
		next = map[string]string{}
	}
	next[key] = value
	return next
}

// SetBucket changes a valid template bucket, leaving invalid input untouched.
func SetBucket(values map[string]budgeting.Bucket, key, raw string) map[string]budgeting.Bucket {
	n, err := strconv.Atoi(raw)
	if err != nil || n < int(budgeting.BucketNeeds) || n > int(budgeting.BucketSavings) {
		return values
	}
	next := maps.Clone(values)
	if next == nil {
		next = map[string]budgeting.Bucket{}
	}
	next[key] = budgeting.Bucket(n)
	return next
}

// CountKey selects the catalog's singular or plural key.
func CountKey(prefix string, count int) string {
	if count == 1 {
		return prefix + ".one"
	}
	return prefix + ".many"
}

// ParseTags trims and case-insensitively deduplicates comma-separated tracked tags.
func ParseTags(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(s, ",") {
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
		key := strings.ToLower(t)
		if t != "" && !seen[key] {
			seen[key] = true
			out = append(out, t)
		}
	}
	return out
}

// MatchCategory prefers a top-level sibling, then a unique match in the whole tree.
func MatchCategory(categories []domain.Category, name string) (domain.Category, bool) {
	if c, ok := catname.FindSibling(categories, "", name); ok {
		return c, true
	}
	if all := catname.FindByName(categories, name); len(all) == 1 {
		return all[0], true
	}
	return domain.Category{}, false
}

// NameForCategory preserves a user-customized budget name.
func NameForCategory(current, previous, next string) string {
	if strings.TrimSpace(current) == "" || strings.TrimSpace(current) == previous {
		return next
	}
	return current
}

// CategoryOption describes a category without coupling the model to a UI picker.
type CategoryOption struct {
	ID, Name string
	Disabled bool
}

// CategoryOptions preserves used categories and explains their disabled state.
func CategoryOptions(categories []domain.Category, budgets []domain.Budget, period, owner string) []CategoryOption {
	out := make([]CategoryOption, 0, len(categories))
	for _, c := range categories {
		out = append(out, CategoryOption{c.ID, c.Name, budgeting.IsDuplicateBudget(budgets, c.ID, period, owner, "")})
	}
	return out
}

// Preview contains the category and validation decisions used by the form.
type Preview struct {
	ExpenseCount                                                        int
	Categories                                                          []domain.Category
	MustCreate, WantNew, Saving, Blocked, NameAutofocus, LimitAutofocus bool
	TypedCategory, SelectedName, SuggestCategory, FateKey, FatePath     string
	Ambiguous                                                           int
	LimitProblem                                                        budgeting.LimitProblem
	LimitFieldError                                                     bool
	BlockerKey, OwnerKey, OwnerName, AdvancedKey                        string
}

// Inspect derives the display decisions from a draft and current records.
func Inspect(d Draft, cats []domain.Category, budgets []domain.Budget, members []domain.Member, seedName, seedLimit string) Preview {
	v := Preview{Saving: d.Direction == string(domain.DirectionSave), NameAutofocus: strings.TrimSpace(seedName) == "", LimitAutofocus: strings.TrimSpace(seedLimit) == ""}
	for _, c := range cats {
		if c.Kind == domain.KindExpense {
			v.ExpenseCount++
			v.Categories = append(v.Categories, c)
		}
	}
	if v.Saving {
		v.Categories = cats
	}
	v.Categories = catname.Sorted(v.Categories)
	v.MustCreate = v.ExpenseCount == 0
	v.WantNew = d.CreateCategory || v.MustCreate
	v.TypedCategory = strings.TrimSpace(d.NewCategoryName)
	if v.TypedCategory == "" {
		v.TypedCategory = strings.TrimSpace(d.Name)
	}
	v.SuggestCategory = d.CategoryID
	if v.SuggestCategory == NewCategory || v.WantNew {
		v.SuggestCategory = ""
	}
	if v.WantNew && v.TypedCategory != "" {
		if c, ok := MatchCategory(cats, v.TypedCategory); ok {
			v.SuggestCategory = c.ID
			v.FateKey = "budgets.catWillReuse"
			v.FatePath = catname.Path(cats, c)
		} else if all := catname.FindByName(cats, v.TypedCategory); len(all) > 1 {
			v.FateKey = "budgets.catAmbiguous"
			v.Ambiguous = len(all)
		} else {
			v.FateKey = "budgets.catWillCreate"
		}
	}
	for _, c := range v.Categories {
		if c.ID == d.CategoryID {
			v.SelectedName = c.Name
			break
		}
	}
	_, v.LimitProblem = budgeting.ParseLimitMajor(d.Limit, currency.Decimals(d.Currency))
	v.LimitFieldError = !v.LimitProblem.OK() && v.LimitProblem != budgeting.LimitBlank
	nameMissing := strings.TrimSpace(d.Name) == ""
	categoryMissing := !v.WantNew && strings.TrimSpace(d.CategoryID) == ""
	duplicate := !v.WantNew && d.CategoryID != "" && budgeting.IsDuplicateBudget(budgets, d.CategoryID, d.Period, d.Owner, "")
	v.Blocked = nameMissing || categoryMissing || duplicate || !v.LimitProblem.OK()
	switch {
	case nameMissing && v.LimitProblem == budgeting.LimitBlank:
		v.BlockerKey = "budgets.addNeedsBoth"
	case nameMissing:
		v.BlockerKey = "budgets.addNeedsName"
	case categoryMissing:
		v.BlockerKey = "budgets.categoryRequired"
	case duplicate:
		v.BlockerKey = "budgets.categoryAlreadyBudgeted"
	case v.LimitProblem == budgeting.LimitBlank:
		v.BlockerKey = "budgets.addNeedsLimit"
	}
	v.OwnerKey = "budgets.ownerSharedHint"
	if d.Owner != domain.GroupOwnerID {
		v.OwnerKey = "budgets.ownerIndividualHint"
		for _, m := range members {
			if m.ID == d.Owner {
				v.OwnerName = m.Name
				break
			}
		}
	}
	v.AdvancedKey = "budgets.advancedShow"
	if d.Advanced {
		v.AdvancedKey = "budgets.advancedHide"
	}
	return v
}

// ChooseCategory changes the selection and its automatic name together.
func ChooseCategory(d Draft, cats []domain.Category, id string) Draft {
	previous := ""
	for _, c := range cats {
		if c.ID == d.CategoryID {
			previous = c.Name
			break
		}
	}
	d.CategoryID = id
	for _, c := range cats {
		if c.ID == id {
			d.Name = NameForCategory(d.Name, previous, c.Name)
			break
		}
	}
	return d
}

// Copy pre-fills a draft using the same creation defaults as a copied budget.
func Copy(d Draft, budgets []domain.Budget, id string) (Draft, string, bool) {
	for _, b := range budgets {
		if b.ID != id {
			continue
		}
		defaults := budgeting.CopyBudgetDefaults(b)
		d.Limit = FormatLimit(defaults.Limit)
		d.Currency = defaults.Limit.Currency
		d.Period = string(defaults.Period)
		d.Direction = string(defaults.Direction)
		d.Owner = b.OwnerID
		d.Methodology = b.Methodology
		d.Rollover = b.Rollover
		if b.CategoryID != "" {
			d.CategoryID = b.CategoryID
		}
		d.Also = map[string]bool{}
		for i, cid := range b.TrackedCategoryIDs() {
			if i > 0 {
				d.Also[cid] = true
			}
		}
		d.Tags = strings.Join(b.TrackedTags, ", ")
		d.Advanced = true
		return d, b.Name, true
	}
	return d, "", false
}

// FormatLimit formats an integer amount for a major-unit form input.
func FormatLimit(value money.Money) string {
	return money.FormatMinor(value.Amount, currency.Decimals(value.Currency))
}

// Prepared is an atomic category-and-budget write ready for persistence.
type Prepared struct {
	Budget                    domain.Budget
	Category                  *domain.Category
	CreatedName               string
	ProblemKey, CollisionName string
	CustomIssues              []string
	LimitProblem              budgeting.LimitProblem
}

// Prepare validates and builds a draft against fresh records. IDs are supplied by the caller.
func Prepare(d Draft, cats []domain.Category, budgets []domain.Budget, defs []customfields.Def, budgetID, categoryID string) Prepared {
	p := Prepared{}
	if strings.TrimSpace(d.Name) == "" {
		p.ProblemKey = "budgets.nameRequired"
		return p
	}
	custom, customIssues := customfields.ParseFormValues(defs, d.Custom)
	if len(customIssues) > 0 {
		p.CustomIssues = customIssues
		return p
	}
	amount, problem := budgeting.ParseLimitMajor(d.Limit, currency.Decimals(d.Currency))
	p.LimitProblem = problem
	if !problem.OK() {
		return p
	}
	src := d.VarName
	if src == "" {
		src = d.Name
	}
	slug := engineenv.BudgetVarSlug(src)
	for _, b := range budgets {
		other := b.VarName
		if other == "" {
			other = b.Name
		}
		if b.ID != "" && slug != "" && engineenv.BudgetVarSlug(other) == slug {
			p.ProblemKey = "budgets.varNameTaken"
			p.CollisionName = b.Name
			return p
		}
	}
	expense := 0
	for _, c := range cats {
		if c.Kind == domain.KindExpense {
			expense++
		}
	}
	final := d.CategoryID
	if d.CreateCategory || expense == 0 || final == NewCategory {
		name := strings.TrimSpace(d.NewCategoryName)
		if name == "" {
			name = strings.TrimSpace(d.Name)
		}
		if c, ok := MatchCategory(cats, name); ok {
			final = c.ID
		} else {
			p.Category = &domain.Category{ID: categoryID, Name: name, Kind: domain.KindExpense}
			final = categoryID
			p.CreatedName = name
		}
	}
	if final == "" {
		p.ProblemKey = "budgets.categoryRequired"
		return p
	}
	if p.CreatedName == "" && budgeting.IsDuplicateBudget(budgets, final, d.Period, d.Owner, "") {
		p.ProblemKey = "budgets.duplicateBudget"
		return p
	}
	scope := domain.ScopeIndividual
	if d.Owner == domain.GroupOwnerID {
		scope = domain.ScopeShared
	}
	method := d.Methodology
	if method != "" && !budgeting.Methodology(method).Valid() {
		method = ""
	}
	b := domain.Budget{ID: budgetID, Name: strings.TrimSpace(d.Name), Scope: scope, OwnerID: d.Owner, CategoryID: final, Period: domain.Period(d.Period), Limit: money.New(amount, d.Currency), Rollover: d.Rollover, Methodology: method, Custom: custom, VarName: strings.TrimSpace(d.VarName)}
	direction := domain.BudgetDirection(d.Direction)
	if direction.Valid() {
		b.Direction = direction
	}
	ids := []string{final}
	seen := map[string]bool{final: true}
	for _, c := range cats {
		if (direction.IsSaving() || c.Kind == domain.KindExpense) && d.Also[c.ID] && !seen[c.ID] {
			seen[c.ID] = true
			ids = append(ids, c.ID)
		}
	}
	if len(ids) > 1 {
		b.CategoryIDs = ids
	}
	b.TrackedTags = ParseTags(d.Tags)
	p.Budget = b
	return p
}

// Template proposes positive limits only for categories not tracked by an existing budget.
func Template(incomeMinor int64, txns []domain.Transaction, ledger budgeting.NettedLedger, cats []domain.Category, budgets []domain.Budget, now time.Time, rates currency.Rates, overrides map[string]budgeting.Bucket) ([]budgeting.BudgetProposal, error) {
	start := dateutil.MonthStart(now)
	income, err := budgeting.IncomeForBudgetsChecked(incomeMinor, txns, dateutil.AddMonths(start, -1), start, rates.Base, rates)
	if err != nil {
		return nil, fmt.Errorf("template income: %w", err)
	}
	if income <= 0 {
		return nil, nil
	}
	result, err := budgeting.Generate5030WithRatesAndOverrides(income, cats, ledger, now, rates, overrides)
	if err != nil {
		return nil, fmt.Errorf("template proposals: %w", err)
	}
	existing := map[string]bool{}
	for _, b := range budgets {
		for _, id := range b.TrackedCategoryIDs() {
			existing[id] = true
		}
	}
	var out []budgeting.BudgetProposal
	for _, p := range result.Proposals {
		if p.LimitMinor > 0 && !existing[p.Category.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}

// Suggestion contains a period- and currency-adjusted estimate and its category scope.
type Suggestion struct {
	Amount    money.Money
	ScopePath string
	Err       error
}

// Suggest estimates from the selected category and all its descendants.
func Suggest(category string, cats []domain.Category, ledger budgeting.NettedLedger, now time.Time, period, code string, rates currency.Rates) Suggestion {
	covers := map[string]bool{}
	if category != "" {
		covers = categorytree.DescendantsOfAll(cats, []string{category})
		covers[category] = true
	}
	monthly, err := budgeting.SuggestLimitIn(covers, ledger, now, 6, rates)
	if err != nil {
		return Suggestion{Err: fmt.Errorf("suggest limit: %w", err)}
	}
	amount, err := budgeting.LimitForPeriod(money.New(monthly, rates.Base), domain.PeriodMonthly, domain.Period(period), code, rates)
	s := Suggestion{Amount: amount, Err: err}
	if err != nil {
		s.Amount.Amount = 0
	}
	if s.Amount.Amount > 0 && len(covers) > 1 {
		for _, c := range cats {
			if c.ID == category {
				s.ScopePath = catname.Path(cats, c)
				break
			}
		}
	}
	return s
}

// Review totals the selected proposals using checked integer arithmetic.
func Review(proposals []budgeting.BudgetProposal, excluded map[string]bool) (int, int64, error) {
	var total int64
	count := 0
	for _, p := range proposals {
		if excluded[p.Category.ID] {
			continue
		}
		var err error
		total, err = money.Add(total, p.LimitMinor)
		if err != nil {
			return 0, 0, fmt.Errorf("template review total: %w", err)
		}
		count++
	}
	return count, total, nil
}

// TemplateBudgets builds the checked template rows without writing any records.
func TemplateBudgets(proposals []budgeting.BudgetProposal, excluded map[string]bool, base string, newID func() string) []domain.Budget {
	var out []domain.Budget
	for _, p := range proposals {
		if excluded[p.Category.ID] {
			continue
		}
		b := domain.Budget{ID: newID(), Name: p.Category.Name, CategoryID: p.Category.ID, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(p.LimitMinor, base)}
		if p.Bucket == budgeting.BucketSavings {
			b.Direction = domain.DirectionSave
		}
		out = append(out, b)
	}
	return out
}

// Reset clears the fields historically reset after an add, preserving owner, period and direction.
func Reset(d Draft, base, category string) Draft {
	d.Name = ""
	d.VarName = ""
	d.VarTouched = false
	d.Limit = ""
	d.Currency = base
	d.Rollover = false
	d.Methodology = ""
	d.CategoryID = category
	d.NewCategoryName = ""
	d.CreateCategory = false
	d.Also = nil
	d.Tags = ""
	d.Custom = nil
	d.Advanced = false
	return d
}

// MonthIdentity identifies the calendar month used by all history previews.
func MonthIdentity(now time.Time) string { return now.Format("2006-01") }

// Action identifies an editable field or disclosure transition.
type Action uint8

// Form actions preserve the add form's existing state transitions.
const (
	EditLimit Action = iota
	EditNewCategory
	EditDirection
	EditOwner
	EditPeriod
	EditMethod
	EditTags
	EditVariable
	ToggleCreate
	ToggleRollover
	ToggleAdvanced
	OpenTemplate
	CloseTemplate
)

// Change applies one form transition without mutating the previous draft's maps.
func Change(d Draft, action Action, value string) Draft {
	switch action {
	case EditLimit:
		d.Limit = value
	case EditNewCategory:
		d.NewCategoryName = value
	case EditDirection:
		d.Direction = value
	case EditOwner:
		d.Owner = value
	case EditPeriod:
		d.Period = value
	case EditMethod:
		d.Methodology = value
	case EditTags:
		d.Tags = value
	case EditVariable:
		d.VarName = value
		d.VarTouched = true
	case ToggleCreate:
		d.CreateCategory = !d.CreateCategory
	case ToggleRollover:
		d.Rollover = !d.Rollover
	case ToggleAdvanced:
		d.Advanced = !d.Advanced
	case OpenTemplate:
		d.Template = true
	case CloseTemplate:
		d.Template = false
	}
	return d
}

// SPDX-License-Identifier: MIT

package budgetform

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func categories() []domain.Category {
	return []domain.Category{
		{ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "child", Name: "Lunch", ParentID: "food", Kind: domain.KindExpense},
		{ID: "pay", Name: "Pay", Kind: domain.KindIncome}, {ID: "other", Name: "Other", Kind: domain.KindExpense},
	}
}
func valid() Draft { d := Initial("Food", "100", "food", "", "USD"); return d }
func TestInitialAndTransitions(t *testing.T) {
	for _, tc := range []struct{ base, period, cat string }{{"", "", ""}, {"JPY", "weekly", "food"}} {
		t.Run(tc.base+tc.period, func(t *testing.T) {
			d := Initial("seed", "12", tc.cat, tc.period, tc.base)
			if d.Owner != domain.GroupOwnerID || d.Direction != string(domain.DirectionSpend) || d.Advanced != (tc.cat != "") || d.Currency == "" || d.Period == "" {
				t.Fatal(d)
			}
			d = ChangeName(d, "Fun money")
			if d.Name != "Fun money" || d.VarName != "fun_money" {
				t.Fatal(d)
			}
			d.VarTouched = true
			d = ChangeName(d, "Different")
			if d.VarName != "fun_money" {
				t.Fatal(d)
			}
		})
	}
	original := map[string]bool{"a": true}
	next := Toggle(original, "a")
	if !original["a"] || next["a"] || !Toggle(nil, "x")["x"] {
		t.Fatal(original, next)
	}
	originalText := map[string]string{"a": "old"}
	if SetCustom(originalText, "a", "new")["a"] != "new" || originalText["a"] != "old" || SetCustom(nil, "a", "v")["a"] != "v" {
		t.Fatal(originalText)
	}
	for _, raw := range []string{"bad", "-1", "99", "0", "1", "2", "3"} {
		out := SetBucket(nil, "a", raw)
		n, err := strconv.Atoi(raw)
		valid := err == nil && n >= int(budgeting.BucketNeeds) && n <= int(budgeting.BucketSavings)
		if (out != nil) != valid {
			t.Fatal(raw, out)
		}
	}
	buckets := map[string]budgeting.Bucket{"a": budgeting.BucketNeeds}
	_ = SetBucket(buckets, "a", strconv.Itoa(int(budgeting.BucketSavings)))
	if buckets["a"] != budgeting.BucketNeeds {
		t.Fatal(buckets)
	}
	for _, n := range []int{-1, 0, 1, 2} {
		suffix := ".many"
		if n == 1 {
			suffix = ".one"
		}
		if CountKey("x", n) != "x"+suffix {
			t.Fatal(n)
		}
	}
}
func TestTagsAndMatching(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{{"", nil}, {" , # , ", nil}, {" #Travel,travel, ##two, Food,food ", []string{"Travel", "#two", "Food"}}} {
		if got := ParseTags(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Fatal(got, tc.want)
		}
	}
	cats := categories()
	cats = append(cats, domain.Category{ID: "duplicate", Name: "Lunch", ParentID: "other"})
	for _, tc := range []struct {
		name, id string
		ok       bool
	}{{" food ", "food", true}, {"Pay", "pay", true}, {"Other", "other", true}, {"Lunch", "", false}, {"missing", "", false}} {
		got, ok := MatchCategory(cats, tc.name)
		if got.ID != tc.id || ok != tc.ok {
			t.Fatal(tc, got, ok)
		}
	}
	if got, ok := MatchCategory(categories(), "Lunch"); !ok || got.ID != "child" {
		t.Fatal(got, ok)
	}
	if _, ok := MatchCategory(nil, "x"); ok {
		t.Fatal("nil matched")
	}
	for _, tc := range []struct{ current, previous, next, want string }{{" ", "Food", "Pay", "Pay"}, {" Food ", "Food", "Pay", "Pay"}, {"Custom", "Food", "Pay", "Custom"}} {
		if NameForCategory(tc.current, tc.previous, tc.next) != tc.want {
			t.Fatal(tc)
		}
	}
}
func TestInspect(t *testing.T) {
	bs := []domain.Budget{{ID: "b", Name: "Existing", CategoryID: "food", Period: domain.PeriodMonthly, OwnerID: domain.GroupOwnerID}}
	cases := []struct {
		name    string
		change  func(*Draft)
		key     string
		blocked bool
	}{
		{"both", func(d *Draft) { d.Name = " "; d.Limit = "" }, "budgets.addNeedsBoth", true},
		{"name", func(d *Draft) { d.Name = "" }, "budgets.addNeedsName", true},
		{"category", func(d *Draft) { d.CategoryID = "" }, "budgets.categoryRequired", true},
		{"duplicate", func(d *Draft) {}, "budgets.categoryAlreadyBudgeted", true},
		{"limit", func(d *Draft) { d.CategoryID = "other"; d.Limit = "" }, "budgets.addNeedsLimit", true},
		{"malformed", func(d *Draft) { d.CategoryID = "other"; d.Limit = "xx" }, "", true},
		{"valid", func(d *Draft) { d.CategoryID = "other" }, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := valid()
			tc.change(&d)
			v := Inspect(d, categories(), bs, nil, "", "")
			if v.BlockerKey != tc.key || v.Blocked != tc.blocked || !v.NameAutofocus || !v.LimitAutofocus {
				t.Fatal(v)
			}
		})
	}
	for _, tc := range []struct {
		cats      []domain.Category
		name, key string
		ambiguous int
	}{{nil, "New", "budgets.catWillCreate", 0}, {categories(), "Lunch", "budgets.catWillReuse", 0}, {append(categories(), domain.Category{ID: "dup", Name: "Lunch", ParentID: "other"}), "Lunch", "budgets.catAmbiguous", 2}} {
		d := valid()
		d.CreateCategory = true
		d.NewCategoryName = tc.name
		v := Inspect(d, tc.cats, nil, nil, "seed", "1")
		if v.FateKey != tc.key || v.Ambiguous != tc.ambiguous {
			t.Fatal(v)
		}
	}
	d := valid()
	d.Direction = "save"
	d.Owner = "m"
	d.Advanced = true
	d.CategoryID = NewCategory
	v := Inspect(d, categories(), nil, []domain.Member{{ID: "m", Name: "Maya"}}, "", "")
	if !v.Saving || len(v.Categories) != 4 || v.OwnerName != "Maya" || v.OwnerKey != "budgets.ownerIndividualHint" || v.AdvancedKey != "budgets.advancedHide" || v.SuggestCategory != "" {
		t.Fatal(v)
	}
	d = ChooseCategory(valid(), categories(), "pay")
	if d.Name != "Pay" || d.CategoryID != "pay" {
		t.Fatal(d)
	}
	d.Name = "Custom"
	d = ChooseCategory(d, categories(), "missing")
	if d.Name != "Custom" {
		t.Fatal(d)
	}
	opts := CategoryOptions(categories(), bs, "monthly", domain.GroupOwnerID)
	if len(opts) != 4 || !opts[0].Disabled || opts[1].Disabled {
		t.Fatal(opts)
	}
	if len(CategoryOptions(nil, nil, "", "")) != 0 {
		t.Fatal("nil options")
	}
}
func TestPrepareValidation(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		problem budgeting.LimitProblem
	}{{"", budgeting.LimitBlank}, {"0", budgeting.LimitNotPositive}, {"-1", budgeting.LimitNotPositive}, {"wat", budgeting.LimitMalformed}, {"9223372036854775808", budgeting.LimitMalformed}, {"1.001", budgeting.LimitMalformed}} {
		d := valid()
		d.Limit = tc.raw
		p := Prepare(d, categories(), nil, nil, "b", "c")
		if p.LimitProblem != tc.problem || p.Budget.ID != "" {
			t.Fatal(tc, p)
		}
	}
	for _, tc := range []struct {
		name    string
		change  func(*Draft)
		budgets []domain.Budget
		key     string
	}{
		{"name", func(d *Draft) { d.Name = "" }, nil, "budgets.nameRequired"},
		{"category", func(d *Draft) { d.CategoryID = "" }, nil, "budgets.categoryRequired"},
		{"duplicate", func(d *Draft) { d.Name = "Different" }, []domain.Budget{{ID: "b0", Name: "Other", CategoryID: "food", Period: domain.PeriodMonthly, OwnerID: domain.GroupOwnerID}}, "budgets.duplicateBudget"},
		{"var collision", func(d *Draft) { d.VarName = "other" }, []domain.Budget{{ID: "b0", Name: "Other"}}, "budgets.varNameTaken"},
		{"explicit collision", func(d *Draft) { d.VarName = "shared" }, []domain.Budget{{ID: "b0", Name: "Other", VarName: "shared"}}, "budgets.varNameTaken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := valid()
			tc.change(&d)
			p := Prepare(d, categories(), tc.budgets, nil, "b", "c")
			if p.ProblemKey != tc.key {
				t.Fatal(p)
			}
		})
	}
}
func TestPrepareBuild(t *testing.T) {
	defs := []customfields.Def{{Key: "number", Type: customfields.TypeNumber}, {Key: "invalid", Type: customfields.TypeNumber}, {Key: "flag", Type: customfields.TypeBool}, {Key: "text", Type: customfields.TypeText}, {Key: "empty", Type: customfields.TypeText}}
	d := valid()
	d.CreateCategory = true
	d.NewCategoryName = " New "
	d.Name = " Name "
	d.Owner = "m"
	d.Methodology = "invalid"
	d.Direction = "save"
	d.Also = map[string]bool{"food": true, "pay": true, "child": true}
	d.Tags = "#A, a, B"
	d.VarName = " handle "
	d.Custom = map[string]string{"number": "2.5", "invalid": "bad", "flag": "true", "text": " text "}
	p := Prepare(d, categories(), nil, defs, "b", "new")
	if p.ProblemKey != "" || p.Category == nil || p.CreatedName != "New" || p.Budget.Name != "Name" || p.Budget.Scope != domain.ScopeIndividual || p.Budget.Methodology != "" || p.Budget.VarName != "handle" || p.Budget.Direction != domain.DirectionSave || len(p.Budget.CategoryIDs) != 4 || len(p.Budget.TrackedTags) != 2 || p.Budget.Custom["number"] != 2.5 || p.Budget.Custom["invalid"] != "bad" || p.Budget.Custom["flag"] != true || p.Budget.Custom["text"] != "text" {
		t.Fatal(p, p.Budget)
	}
	for _, tc := range []struct {
		cats     []domain.Category
		create   bool
		name, id string
	}{{categories(), true, "Food", "food"}, {nil, false, "New", "new"}, {categories(), false, "New", NewCategory}} {
		d := valid()
		d.CreateCategory = tc.create
		d.NewCategoryName = tc.name
		if tc.id == NewCategory {
			d.CategoryID = NewCategory
		}
		p := Prepare(d, tc.cats, nil, nil, "b", "new")
		if p.ProblemKey != "" || p.Budget.ID != "b" {
			t.Fatal(p)
		}
	}
	d = valid()
	d.Direction = "invalid"
	d.Also = map[string]bool{"pay": true, "child": true, "food": true}
	d.Custom = map[string]string{}
	p = Prepare(d, append(categories(), categories()...), nil, defs, "b", "c")
	if p.Budget.Direction != "" || len(p.Budget.CategoryIDs) != 2 || p.Budget.Custom != nil {
		t.Fatal(p)
	}
	d = valid()
	d.Methodology = string(budgeting.MethodZeroBased)
	p = Prepare(d, categories(), nil, nil, "b", "c")
	if p.Budget.Methodology != d.Methodology || p.Budget.CategoryIDs != nil || p.Budget.Scope != domain.ScopeShared {
		t.Fatal(p)
	}
}
func TestCopyAndFormat(t *testing.T) {
	for _, tc := range []money.Money{money.New(0, "USD"), money.New(-123, "USD"), money.New(math.MaxInt64, "JPY"), money.New(math.MinInt64, "USD")} {
		raw := FormatLimit(tc)
		n, err := money.ParseMinor(raw, currency.Decimals(tc.Currency))
		if err != nil || n != tc.Amount {
			t.Fatal(raw, n, err)
		}
	}
	d := valid()
	bs := []domain.Budget{{ID: "skip"}, {ID: "b", Name: "Old", CategoryID: "food", CategoryIDs: []string{"food", "child"}, TrackedTags: []string{"a", "b"}, Limit: money.New(150, "JPY"), OwnerID: "m", Rollover: true, Period: domain.PeriodWeekly, Methodology: "zero-based"}}
	got, name, ok := Copy(d, bs, "b")
	if !ok || name != "Old" || got.Limit != "150" || !got.Also["child"] || got.Tags != "a, b" || !got.Advanced || !got.Rollover || got.Owner != "m" {
		t.Fatal(got, name, ok)
	}
	if got, _, ok := Copy(d, bs, "missing"); ok || !reflect.DeepEqual(got, d) {
		t.Fatal(got, ok)
	}
	bs[1].CategoryID = ""
	if got, _, _ := Copy(d, bs, "b"); got.CategoryID != d.CategoryID {
		t.Fatal(got)
	}
}
func TestReviewAndTemplateBudgets(t *testing.T) {
	proposals := []budgeting.BudgetProposal{{Category: domain.Category{ID: "a", Name: "A"}, LimitMinor: 10}, {Category: domain.Category{ID: "b", Name: "B"}, LimitMinor: 20, Bucket: budgeting.BucketSavings}}
	for _, tc := range []struct {
		excluded map[string]bool
		n        int
		total    int64
	}{{nil, 2, 30}, {map[string]bool{"a": true}, 1, 20}, {map[string]bool{"a": true, "b": true}, 0, 0}} {
		n, total, err := Review(proposals, tc.excluded)
		if n != tc.n || total != tc.total || err != nil {
			t.Fatal(n, total, err)
		}
		i := 0
		writes := TemplateBudgets(proposals, tc.excluded, "USD", func() string { i++; return strconv.Itoa(i) })
		if len(writes) != n || i != n {
			t.Fatal(writes)
		}
		if n > 0 && writes[len(writes)-1].Direction != domain.DirectionSave {
			t.Fatal(writes)
		}
	}
	if n, total, err := Review(nil, nil); n != 0 || total != 0 || err != nil {
		t.Fatal(n, total, err)
	}
	for _, amount := range []int64{math.MaxInt64, math.MinInt64} {
		ps := []budgeting.BudgetProposal{{LimitMinor: amount}, {LimitMinor: amount}}
		if _, _, err := Review(ps, nil); !errors.Is(err, money.ErrOverflow) {
			t.Fatal(err)
		}
	}
}
func txn(id, cat, code string, amount int64, date time.Time) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: cat, Amount: money.New(amount, code), Date: date}
}
func TestTemplateAndSuggestion(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	cats := categories()
	for _, now := range []time.Time{time.Date(2024, 3, 31, 23, 59, 0, 0, time.FixedZone("west", -7*3600)), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} {
		t.Run(now.String(), func(t *testing.T) {
			previous := time.Date(now.Year(), now.Month()-1, 15, 12, 0, 0, 0, now.Location())
			txns := []domain.Transaction{txn("expense", "child", "USD", -12300, previous)}
			ledger := budgeting.NettedLedger(txns)
			s := Suggest("food", cats, ledger, now, "monthly", "USD", rates)
			if s.Err != nil || s.Amount.Amount != 12300 || s.ScopePath != "Food" {
				t.Fatal(s)
			}
			for _, code := range []string{"USD", "EUR"} {
				r := rates
				if code == "EUR" {
					r.Rates = map[string]float64{"EUR": 2}
				}
				s := Suggest("food", cats, ledger, now, "weekly", code, r)
				if s.Err != nil || s.Amount.Amount <= 0 {
					t.Fatal(s)
				}
			}
			if s := Suggest("", nil, nil, now, "monthly", "USD", rates); s.Err != nil || s.Amount.Amount != 0 {
				t.Fatal(s)
			}
			if s := Suggest("food", cats, ledger, now, "monthly", "EUR", rates); !errors.Is(s.Err, currency.ErrUnknownRate) || s.Amount.Amount != 0 {
				t.Fatal(s)
			}
			bad := budgeting.NettedLedger{txn("bad", "food", "EUR", -100, previous)}
			if s := Suggest("food", cats, bad, now, "monthly", "USD", rates); !errors.Is(s.Err, currency.ErrUnknownRate) {
				t.Fatal(s)
			}
			overflow := budgeting.NettedLedger{txn("1", "food", "USD", -math.MaxInt64, previous), txn("2", "food", "USD", -math.MaxInt64, previous)}
			if s := Suggest("food", cats, overflow, now, "monthly", "USD", rates); !errors.Is(s.Err, money.ErrOverflow) {
				t.Fatal(s)
			}
			ps, err := Template(100000, nil, ledger, cats, nil, now, rates, nil)
			if err != nil || len(ps) == 0 {
				t.Fatal(ps, err)
			}
			existing := []domain.Budget{{CategoryIDs: []string{"food", "child", "other", "pay"}}}
			if ps, err := Template(100000, nil, ledger, cats, existing, now, rates, nil); err != nil || len(ps) != 0 {
				t.Fatal(ps, err)
			}
			for _, income := range []int64{0, -1} {
				if ps, err := Template(income, nil, nil, cats, nil, now, rates, nil); err != nil || len(ps) != 0 {
					t.Fatal(ps, err)
				}
			}
			if _, err := Template(0, []domain.Transaction{txn("income", "pay", "EUR", 100, previous)}, nil, cats, nil, now, rates, nil); !errors.Is(err, currency.ErrUnknownRate) {
				t.Fatal(err)
			}
			if _, err := Template(100000, nil, bad, cats, nil, now, rates, nil); !errors.Is(err, currency.ErrUnknownRate) {
				t.Fatal(err)
			}
		})
	}
}
func TestHugeSelections(t *testing.T) {
	cats := make([]domain.Category, 10000)
	selected := map[string]bool{}
	for i := range cats {
		cats[i] = domain.Category{ID: strconv.Itoa(i), Name: strconv.Itoa(i), Kind: domain.KindExpense}
		selected[cats[i].ID] = true
	}
	d := valid()
	d.CategoryID = "0"
	d.Also = selected
	p := Prepare(d, cats, nil, nil, "b", "c")
	if len(p.Budget.CategoryIDs) != len(cats) {
		t.Fatal(len(p.Budget.CategoryIDs))
	}
	if len(Inspect(d, cats, nil, nil, "", "").Categories) != len(cats) {
		t.Fatal("lost categories")
	}
}
func FuzzTagsIdempotent(f *testing.F) {
	for _, s := range []string{"#Travel, travel, Food", "", strings.Repeat("a,", 1000)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		tags := ParseTags(s)
		again := ParseTags(strings.Join(tags, ",")) // A second leading # intentionally strips again; exclude that legacy input from the round-trip invariant.
		for _, tag := range tags {
			if strings.HasPrefix(tag, "#") {
				return
			}
		}
		if !reflect.DeepEqual(tags, again) {
			t.Fatal(tags, again)
		}
		seen := map[string]bool{}
		for _, tag := range tags {
			key := strings.ToLower(tag)
			if tag == "" || seen[key] {
				t.Fatal(tags)
			}
			seen[key] = true
		}
	})
}
func FuzzReviewReconciles(f *testing.F) {
	f.Add(int64(100), int64(-50), true)
	f.Add(int64(math.MaxInt64), int64(1), false)
	f.Fuzz(func(t *testing.T, a, b int64, exclude bool) {
		ps := []budgeting.BudgetProposal{{Category: domain.Category{ID: "a"}, LimitMinor: a}, {Category: domain.Category{ID: "b"}, LimitMinor: b}}
		n, total, err := Review(ps, map[string]bool{"b": exclude})
		if exclude {
			if err != nil || n != 1 || total != a {
				t.Fatal(n, total, err)
			}
			return
		}
		want, werr := money.Add(a, b)
		if (err != nil) != (werr != nil) || err == nil && (n != 2 || total != want) {
			t.Fatal(n, total, err, want, werr)
		}
	})
}

func TestResetAndMonthIdentity(t *testing.T) {
	d := valid()
	d.Owner = "m"
	d.Direction = "save"
	d.Period = "weekly"
	d.Excluded = map[string]bool{"x": true}
	d.Template = true
	d.Name = "old"
	d.VarName = "old"
	d.VarTouched = true
	d.CreateCategory = true
	d.Advanced = true
	d.Rollover = true
	d.Tags = "old"
	d.Also = map[string]bool{"old": true}
	d.Custom = map[string]string{"old": "v"}
	got := Reset(d, "JPY", "seed")
	if got.Name != "" || got.Limit != "" || got.VarName != "" || got.VarTouched || got.CreateCategory || got.Advanced || got.Rollover || got.Tags != "" || got.Also != nil || got.Custom != nil || got.Currency != "JPY" || got.CategoryID != "seed" || got.Owner != "m" || got.Direction != "save" || got.Period != "weekly" || !got.Template || !got.Excluded["x"] {
		t.Fatal(got)
	}
	if MonthIdentity(time.Date(2026, 1, 31, 23, 59, 0, 0, time.FixedZone("west", -7*3600))) != "2026-01" {
		t.Fatal("month identity moved to UTC")
	}
}

func TestChange(t *testing.T) {
	for _, tc := range []struct {
		action Action
		read   func(Draft) string
	}{
		{EditLimit, func(d Draft) string { return d.Limit }}, {EditNewCategory, func(d Draft) string { return d.NewCategoryName }},
		{EditDirection, func(d Draft) string { return d.Direction }}, {EditOwner, func(d Draft) string { return d.Owner }},
		{EditPeriod, func(d Draft) string { return d.Period }}, {EditMethod, func(d Draft) string { return d.Methodology }},
		{EditTags, func(d Draft) string { return d.Tags }}, {EditVariable, func(d Draft) string { return d.VarName }},
	} {
		t.Run(strconv.Itoa(int(tc.action)), func(t *testing.T) {
			d := valid()
			got := Change(d, tc.action, "value")
			if tc.read(got) != "value" || got.VarTouched != (tc.action == EditVariable) {
				t.Fatal(got)
			}
		})
	}
	for _, tc := range []struct {
		action Action
		read   func(Draft) bool
	}{{ToggleCreate, func(d Draft) bool { return d.CreateCategory }}, {ToggleRollover, func(d Draft) bool { return d.Rollover }}, {ToggleAdvanced, func(d Draft) bool { return d.Advanced }}} {
		d := Draft{}
		got := Change(d, tc.action, "")
		if !tc.read(got) || !reflect.DeepEqual(Change(got, tc.action, ""), d) {
			t.Fatal(got)
		}
	}
	d := Change(Draft{}, OpenTemplate, "")
	if !d.Template || Change(d, CloseTemplate, "").Template {
		t.Fatal(d)
	}
	if got := Change(valid(), Action(255), "ignored"); !reflect.DeepEqual(got, valid()) {
		t.Fatal(got)
	}
}

func TestPreparationUsesBudgetNameWhenNewCategoryNameIsBlank(t *testing.T) {
	d := valid()
	d.CreateCategory = true
	p := Prepare(d, categories(), nil, nil, "b", "new")
	if p.Category != nil || p.Budget.CategoryID != "food" || p.CreatedName != "" {
		t.Fatal(p)
	}
}

func TestCategoryOrderIsStableAndDoesNotMutateRecords(t *testing.T) {
	cats := []domain.Category{{ID: "z", Name: "Zulu", Kind: domain.KindExpense}, {ID: "a1", Name: "Alpha", Kind: domain.KindExpense}, {ID: "a2", Name: "alpha", Kind: domain.KindExpense}, {ID: "b", Name: "Bravo", Kind: domain.KindExpense}}
	before := append([]domain.Category(nil), cats...)
	v := Inspect(valid(), cats, nil, nil, "", "")
	var ids []string
	for _, c := range v.Categories {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []string{"a1", "a2", "b", "z"}) || !reflect.DeepEqual(cats, before) {
		t.Fatal(ids, cats)
	}
}
